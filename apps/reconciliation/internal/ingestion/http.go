package ingestion

import (
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"uuid"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/catalog"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/httpapi"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/identity"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/ingestion/ingestiondb"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/telemetry"
	"go.opentelemetry.io/otel/attribute"
)

type importForm struct {
	SourceID           string        `form:"source_id" format:"uuid"`
	FinancialAccountID string        `form:"financial_account_id" format:"uuid" required:"false" doc:"Omit when the CSV specifies a financial_account code on each row."`
	PeriodStart        string        `form:"period_start" format:"date"`
	PeriodEnd          string        `form:"period_end" format:"date"`
	File               huma.FormFile `form:"file" contentType:"text/plain"`
}

type createPreviewInput struct {
	Authorization string `header:"Authorization"`
	RawBody       huma.MultipartFormFiles[importForm]
}

type getPreviewInput struct {
	Authorization string `header:"Authorization"`
	ID            string `path:"id"`
}

type confirmImportInput struct {
	Authorization string `header:"Authorization"`
	ID            string `path:"id"`
	Body          *struct {
		KeepOriginalConflictIDs []string `json:"keep_original_conflict_ids,omitempty" maxItems:"10000"`
	}
}

type importProblemContext struct {
	Preview           *importView      `json:"preview,omitempty"`
	Coverage          *coverageOverlap `json:"coverage,omitempty"`
	PreviewSourceSide string           `json:"preview_source_side,omitempty"`
	CurrentSourceSide string           `json:"current_source_side,omitempty"`
}

type previewOutput struct {
	Status       int
	CacheControl string `header:"Cache-Control"`
	Body         importView
}

func (s *Service) RegisterHTTP(api huma.API, identities *identity.Service) {
	bearer := []map[string][]string{{"bearer": {}}}
	huma.Register(api, huma.Operation{
		OperationID:        "preview-csv-import",
		Method:             http.MethodPost,
		Path:               "/api/imports",
		Summary:            "Stage a CSV import for review",
		DefaultStatus:      http.StatusCreated,
		Errors:             []int{http.StatusUnauthorized, http.StatusConflict, http.StatusRequestEntityTooLarge, http.StatusUnprocessableEntity},
		Security:           bearer,
		MaxBodyBytes:       1024 * 1024,
		SkipValidateParams: true, // Report invalid selections with stable, field-specific reasons.
	}, func(ctx context.Context, in *createPreviewInput) (out *previewOutput, err error) {
		ctx, finish := startImport(ctx, "preview", "")
		defer func() { finish(out, err) }()
		form := in.RawBody.Data()
		if form.File.File != nil {
			defer form.File.Close()
		}
		userID, err := identities.RequireUser(ctx, in.Authorization)
		if err != nil {
			return nil, err
		}
		return s.createPreview(ctx, userID, form)
	})
	huma.Register(api, huma.Operation{
		OperationID: "get-import-preview",
		Method:      http.MethodGet,
		Path:        "/api/imports/{id}",
		Summary:     "Read an import preview or receipt",
		Errors:      []int{http.StatusUnauthorized, http.StatusNotFound},
		Security:    bearer,
	}, func(ctx context.Context, in *getPreviewInput) (*previewOutput, error) {
		if _, err := identities.RequireUser(ctx, in.Authorization); err != nil {
			return nil, err
		}
		id, err := uuid.Parse(in.ID)
		if err != nil {
			return nil, previewNotFound()
		}
		telemetry.SetSpanAttributes(ctx, attribute.String("import_id", id.String()))
		staged, err := s.getPreview(ctx, pgtype.UUID{Bytes: id, Valid: true})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, previewNotFound()
		}
		if err != nil {
			return nil, previewUnavailable(ctx, err)
		}
		return &previewOutput{Status: http.StatusOK, CacheControl: "no-store", Body: staged}, nil
	})
	huma.Register(api, huma.Operation{
		OperationID:   "refresh-import-preview",
		Method:        http.MethodPost,
		Path:          "/api/imports/{id}/refresh-preview",
		DefaultStatus: http.StatusCreated,
		Summary:       "Regenerate an import preview from retained CSV rows",
		Errors:        []int{http.StatusUnauthorized, http.StatusNotFound, http.StatusConflict, http.StatusUnprocessableEntity},
		Security:      bearer,
	}, func(ctx context.Context, in *getPreviewInput) (out *previewOutput, err error) {
		ctx, finish := startImport(ctx, "refresh_preview", in.ID)
		defer func() { finish(out, err) }()
		analystID, err := identities.RequireUser(ctx, in.Authorization)
		if err != nil {
			return nil, err
		}
		id, err := uuid.Parse(in.ID)
		if err != nil {
			return nil, previewNotFound()
		}
		view, err := s.refreshPreview(ctx, pgtype.UUID{Bytes: id, Valid: true}, analystID)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return nil, previewNotFound()
		case errors.Is(err, errImportResolved):
			return nil, importAlreadyResolved()
		case err != nil:
			if problem, ok := errors.AsType[*httpapi.Problem](err); ok {
				return nil, problem
			}
			return nil, previewUnavailable(ctx, err)
		}
		return &previewOutput{Status: http.StatusCreated, CacheControl: "no-store", Body: view}, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "confirm-import",
		Method:      http.MethodPost,
		Path:        "/api/imports/{id}/confirm",
		Summary:     "Confirm an import and schedule reconciliation",
		Errors:      []int{http.StatusUnauthorized, http.StatusNotFound, http.StatusConflict, http.StatusUnprocessableEntity},
		Security:    bearer,
	}, func(ctx context.Context, in *confirmImportInput) (*previewOutput, error) {
		var keepIDs []string
		if in.Body != nil {
			keepIDs = in.Body.KeepOriginalConflictIDs
		}
		return s.resolveImport(ctx, identities, in.ID, in.Authorization, false, keepIDs)
	})
	huma.Register(api, huma.Operation{
		OperationID: "cancel-import",
		Method:      http.MethodPost,
		Path:        "/api/imports/{id}/cancel",
		Summary:     "Cancel an import and save the decision",
		Errors:      []int{http.StatusUnauthorized, http.StatusNotFound, http.StatusConflict},
		Security:    bearer,
	}, func(ctx context.Context, in *getPreviewInput) (*previewOutput, error) {
		return s.resolveImport(ctx, identities, in.ID, in.Authorization, true, nil)
	})
}

func (s *Service) resolveImport(ctx context.Context, identities *identity.Service, idValue, authorization string, cancel bool, keepIDs []string) (out *previewOutput, err error) {
	operation := "confirm"
	if cancel {
		operation = "cancel"
	}
	ctx, finish := startImport(ctx, operation, idValue)
	defer func() { finish(out, err) }()
	analystID, err := identities.RequireUser(ctx, authorization)
	if err != nil {
		return nil, err
	}
	id, err := uuid.Parse(idValue)
	if err != nil {
		return nil, previewNotFound()
	}
	receipt, err := s.resolve(ctx, pgtype.UUID{Bytes: id, Valid: true}, analystID, cancel, keepIDs)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return nil, previewNotFound()
	case errors.Is(err, errImportResolved):
		return nil, importAlreadyResolved()
	case err != nil:
		if problem, ok := errors.AsType[*httpapi.Problem](err); ok {
			return nil, problem
		}
		slog.ErrorContext(ctx, "Could not resolve import", "error", err)
		return nil, huma.Error500InternalServerError("The import could not be resolved.")
	}
	return &previewOutput{Status: http.StatusOK, CacheControl: "no-store", Body: receipt}, nil
}

func (s *Service) createPreview(ctx context.Context, userID pgtype.UUID, form *importForm) (*previewOutput, error) {
	params, accountID, issues := importSelections(form)
	if len(issues) > 0 {
		return nil, importProblem("invalid_import_selection", "Check the import selections.", issues...)
	}
	if _, err := s.catalog.GetSource(ctx, params.SourceID); err != nil {
		if errors.Is(err, catalog.ErrSourceNotFound) {
			return nil, importProblem("invalid_import_selection", "Check the import selections.",
				httpapi.Issue{Field: "source_id", Reason: "unknown_source"})
		}
		return nil, previewUnavailable(ctx, err)
	}
	data, err := io.ReadAll(form.File)
	if err != nil {
		return nil, previewUnavailable(ctx, err)
	}
	fingerprint := sha256.Sum256(data)
	params.FileFingerprint = fingerprint[:]
	params.CreatedBy = userID

	var accounts []importAccount
	var registered []catalog.FinancialAccount
	var accountsByCode map[string]string
	if accountID.Valid {
		account, err := s.catalog.GetFinancialAccount(ctx, accountID)
		if errors.Is(err, catalog.ErrFinancialAccountNotFound) {
			return nil, importProblem("invalid_import_selection", "Check the import selections.",
				httpapi.Issue{Field: "financial_account_id", Reason: "unknown_financial_account"})
		}
		if err != nil {
			return nil, previewUnavailable(ctx, err)
		}
		accounts = []importAccount{{id: accountID, entry: account}}
		replay, err := findReplay(ctx, s.queries, params.SourceID, []pgtype.UUID{accountID}, params.FileFingerprint)
		if err != nil {
			return nil, previewUnavailable(ctx, err)
		}
		if replay.ID.Valid {
			receipt, err := readReceipt(replay.Receipt)
			if err != nil {
				return nil, previewUnavailable(ctx, err)
			}
			receipt.replayed = true
			return &previewOutput{Status: http.StatusOK, CacheControl: "no-store", Body: receipt}, nil
		}
	} else {
		registered, err = s.catalog.ListFinancialAccounts(ctx)
		if err != nil {
			return nil, previewUnavailable(ctx, err)
		}
		accountsByCode = make(map[string]string, len(registered))
		for _, account := range registered {
			accountsByCode[account.Code] = account.ID
		}
	}

	rows, issues := parseCSV(data, params.PeriodStart.Time, params.PeriodEnd.Time, accountsByCode)
	if len(issues) > 0 {
		return nil, importProblem("invalid_csv", "The CSV contains invalid data.", issues...)
	}
	if !accountID.Valid {
		accounts, err = resolveImportAccounts(rows, registered)
		if err != nil {
			return nil, previewUnavailable(ctx, err)
		}
	}
	staged, err := s.savePreview(ctx, params, accounts, rows)
	if problem, ok := errors.AsType[*httpapi.Problem](err); ok {
		return nil, problem
	}
	if err != nil {
		return nil, previewUnavailable(ctx, err)
	}
	status := http.StatusCreated
	if staged.Status == "confirmed" {
		status = http.StatusOK
	}
	return &previewOutput{Status: status, CacheControl: "no-store", Body: staged}, nil
}

func importSelections(form *importForm) (ingestiondb.CreatePendingImportParams, pgtype.UUID, []httpapi.Issue) {
	var issues []httpapi.Issue
	add := func(field, reason string) {
		issues = append(issues, httpapi.Issue{Field: field, Reason: reason})
	}

	source, sourceErr := uuid.Parse(form.SourceID)
	if sourceErr != nil {
		add("source_id", "invalid_source")
	}
	account, accountErr := uuid.Parse(form.FinancialAccountID)
	if form.FinancialAccountID != "" && accountErr != nil {
		add("financial_account_id", "invalid_financial_account")
	}
	start, validStart := parseDate(form.PeriodStart)
	if !validStart {
		add("period_start", "invalid_date")
	}
	end, validEnd := parseDate(form.PeriodEnd)
	if !validEnd {
		add("period_end", "invalid_date")
	}
	if validStart && validEnd && end.Before(start) {
		add("period_end", "invalid_period")
	}
	if !form.File.IsSet {
		add("file", "missing_file")
	}
	return ingestiondb.CreatePendingImportParams{
		SourceID:    pgtype.UUID{Bytes: source, Valid: sourceErr == nil},
		PeriodStart: pgtype.Date{Time: start, Valid: validStart},
		PeriodEnd:   pgtype.Date{Time: end, Valid: validEnd},
	}, pgtype.UUID{Bytes: account, Valid: accountErr == nil}, issues
}

func importProblem(code, detail string, issues ...httpapi.Issue) *httpapi.Problem {
	problem := httpapi.NewProblem(http.StatusUnprocessableEntity, code, detail)
	problem.Issues = issues
	return problem
}

func previewNotFound() *httpapi.Problem {
	return httpapi.NewProblem(http.StatusNotFound, "preview_not_found", "The import preview could not be found.")
}

func previewUnavailable(ctx context.Context, err error) error {
	slog.ErrorContext(ctx, "Could not process import preview", "error", err)
	return huma.Error500InternalServerError("The import preview is temporarily unavailable.")
}

func importAlreadyResolved() *httpapi.Problem {
	return httpapi.NewProblem(http.StatusConflict, "import_already_resolved", "The import has already been resolved.")
}
