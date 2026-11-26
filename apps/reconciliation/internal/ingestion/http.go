package ingestion

import (
	"context"
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
)

type importForm struct {
	SourceID           string        `form:"source_id" format:"uuid"`
	FinancialAccountID string        `form:"financial_account_id" format:"uuid"`
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

type previewOutput struct {
	CacheControl string `header:"Cache-Control"`
	Body         preview
}

func (s *Service) RegisterHTTP(api huma.API, identities *identity.Service) {
	bearer := []map[string][]string{{"bearer": {}}}
	huma.Register(api, huma.Operation{
		OperationID:        "preview-csv-import",
		Method:             http.MethodPost,
		Path:               "/api/imports",
		Summary:            "Stage a CSV import for review",
		DefaultStatus:      http.StatusCreated,
		Errors:             []int{http.StatusUnauthorized, http.StatusRequestEntityTooLarge, http.StatusUnprocessableEntity},
		Security:           bearer,
		MaxBodyBytes:       1024 * 1024,
		SkipValidateParams: true, // Report invalid selections with stable, field-specific reasons.
	}, func(ctx context.Context, in *createPreviewInput) (*previewOutput, error) {
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
		Summary:     "Read a staged import",
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
		staged, err := s.getPreview(ctx, pgtype.UUID{Bytes: id, Valid: true})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, previewNotFound()
		}
		if err != nil {
			return nil, previewUnavailable(ctx, err)
		}
		return &previewOutput{CacheControl: "no-store", Body: staged}, nil
	})
}

func (s *Service) createPreview(ctx context.Context, userID pgtype.UUID, form *importForm) (*previewOutput, error) {
	params, issues := importSelections(form)
	if len(issues) > 0 {
		return nil, importProblem("invalid_import_selection", "Check the import selections.", issues...)
	}
	source, account, err := s.catalog.GetSelection(ctx, params.SourceID, params.FinancialAccountID)
	switch {
	case errors.Is(err, catalog.ErrSourceNotFound):
		return nil, importProblem("invalid_import_selection", "Check the import selections.",
			httpapi.Issue{Field: "source_id", Reason: "unknown_source"})
	case errors.Is(err, catalog.ErrFinancialAccountNotFound):
		return nil, importProblem("invalid_import_selection", "Check the import selections.",
			httpapi.Issue{Field: "financial_account_id", Reason: "unknown_financial_account"})
	case err != nil:
		return nil, previewUnavailable(ctx, err)
	}
	data, err := io.ReadAll(form.File)
	if err != nil {
		return nil, previewUnavailable(ctx, err)
	}
	rows, issues := parseCSV(data, params.PeriodStart.Time, params.PeriodEnd.Time)
	if len(issues) > 0 {
		return nil, importProblem("invalid_csv", "The CSV contains invalid data.", issues...)
	}

	params.CreatedBy = userID
	params.SourceSide = source.Side
	id, err := s.savePreview(ctx, params, data, rows)
	if err != nil {
		return nil, previewUnavailable(ctx, err)
	}
	staged := presentPreview(id, "pending", source, account, form.PeriodStart, form.PeriodEnd, rows)
	return &previewOutput{CacheControl: "no-store", Body: staged}, nil
}

func importSelections(form *importForm) (ingestiondb.CreatePendingImportParams, []httpapi.Issue) {
	var issues []httpapi.Issue
	add := func(field, reason string) {
		issues = append(issues, httpapi.Issue{Field: field, Reason: reason})
	}

	source, sourceErr := uuid.Parse(form.SourceID)
	if sourceErr != nil {
		add("source_id", "invalid_source")
	}
	account, accountErr := uuid.Parse(form.FinancialAccountID)
	if accountErr != nil {
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
		SourceID:           pgtype.UUID{Bytes: source, Valid: sourceErr == nil},
		FinancialAccountID: pgtype.UUID{Bytes: account, Valid: accountErr == nil},
		PeriodStart:        pgtype.Date{Time: start, Valid: validStart},
		PeriodEnd:          pgtype.Date{Time: end, Valid: validEnd},
	}, issues
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
