package catalog

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"uuid"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/httpapi"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/identity"
)

type listInput struct {
	Authorization string `header:"Authorization"`
}

type sourcesOutput struct {
	CacheControl string `header:"Cache-Control"`
	Body         []sourceEntry
}

type financialAccountsOutput struct {
	CacheControl string `header:"Cache-Control"`
	Body         []FinancialAccount
}

func (s *Service) RegisterHTTP(api huma.API, identities *identity.Service) {
	huma.Register(api, huma.Operation{
		OperationID: "list-financial-accounts",
		Method:      http.MethodGet,
		Path:        "/api/financial-accounts",
		Summary:     "List Financial Accounts",
		Errors:      []int{http.StatusUnauthorized},
		Security:    []map[string][]string{{"bearer": {}}},
	}, func(ctx context.Context, in *listInput) (*financialAccountsOutput, error) {
		if _, err := identities.RequireUser(ctx, in.Authorization); err != nil {
			return nil, err
		}
		accounts, err := s.ListFinancialAccounts(ctx)
		if err != nil {
			slog.ErrorContext(ctx, "Could not list Financial Accounts", "error", err)
			return nil, huma.Error500InternalServerError("Financial Accounts are temporarily unavailable.")
		}
		return &financialAccountsOutput{CacheControl: "no-store", Body: accounts}, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "list-sources",
		Method:      http.MethodGet,
		Path:        "/api/sources",
		Summary:     "List Sources",
		Errors:      []int{http.StatusUnauthorized},
		Security:    []map[string][]string{{"bearer": {}}},
	}, func(ctx context.Context, in *listInput) (*sourcesOutput, error) {
		if _, err := identities.RequireUser(ctx, in.Authorization); err != nil {
			return nil, err
		}
		sources, err := s.listSources(ctx)
		if err != nil {
			slog.ErrorContext(ctx, "Could not list Sources", "error", err)
			return nil, huma.Error500InternalServerError("Sources are temporarily unavailable.")
		}
		return &sourcesOutput{CacheControl: "no-store", Body: sources}, nil
	})
	s.registerCatalogWrites(api, identities)
}

type createFinancialAccountInput struct {
	Authorization string `header:"Authorization"`
	Body          struct {
		Name string `json:"name" minLength:"1"`
		Code string `json:"code" minLength:"1"`
	}
}

type editFinancialAccountInput struct {
	Authorization string `header:"Authorization"`
	ID            string `path:"id"`
	Body          struct {
		Name string `json:"name" minLength:"1"`
	}
}

type createSourceInput struct {
	Authorization string `header:"Authorization"`
	Body          struct {
		Name string `json:"name" minLength:"1"`
		Side string `json:"side" enum:"internal,external"`
	}
}

type editSourceInput struct {
	Authorization string `header:"Authorization"`
	ID            string `path:"id"`
	Body          struct {
		Name *string `json:"name,omitempty" minLength:"1"`
		Side *string `json:"side,omitempty" enum:"internal,external"`
	}
}

type financialAccountOutput struct {
	CacheControl string `header:"Cache-Control"`
	Body         FinancialAccount
}

type sourceOutput struct {
	CacheControl string `header:"Cache-Control"`
	Body         sourceEntry
}

func (s *Service) registerCatalogWrites(api huma.API, identities *identity.Service) {
	bearer := []map[string][]string{{"bearer": {}}}
	huma.Register(api, huma.Operation{
		OperationID:   "create-financial-account",
		Method:        http.MethodPost,
		Path:          "/api/financial-accounts",
		DefaultStatus: http.StatusCreated,
		Summary:       "Register a Financial Account",
		Errors:        []int{http.StatusUnauthorized, http.StatusConflict, http.StatusUnprocessableEntity},
		Security:      bearer,
	}, func(ctx context.Context, in *createFinancialAccountInput) (*financialAccountOutput, error) {
		if _, err := identities.RequireUser(ctx, in.Authorization); err != nil {
			return nil, err
		}
		name, code := strings.TrimSpace(in.Body.Name), strings.TrimSpace(in.Body.Code)
		if name == "" || code == "" {
			return nil, invalidCatalogEntry()
		}
		account, err := s.createFinancialAccount(ctx, name, code)
		if errors.Is(err, errAccountCodeInUse) {
			return nil, httpapi.NewProblem(http.StatusConflict, "financial_account_code_in_use", "This Financial Account code is already in use.")
		}
		if err != nil {
			return nil, catalogUnavailable(ctx, err)
		}
		return &financialAccountOutput{CacheControl: "no-store", Body: account}, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "edit-financial-account",
		Method:      http.MethodPatch,
		Path:        "/api/financial-accounts/{id}",
		Summary:     "Rename a Financial Account",
		Errors:      []int{http.StatusUnauthorized, http.StatusNotFound, http.StatusUnprocessableEntity},
		Security:    bearer,
	}, func(ctx context.Context, in *editFinancialAccountInput) (*financialAccountOutput, error) {
		if _, err := identities.RequireUser(ctx, in.Authorization); err != nil {
			return nil, err
		}
		id, err := uuid.Parse(in.ID)
		if err != nil {
			return nil, catalogNotFound("financial_account_not_found", "Financial Account")
		}
		name := strings.TrimSpace(in.Body.Name)
		if name == "" {
			return nil, invalidCatalogEntry()
		}
		account, err := s.renameFinancialAccount(ctx, pgtype.UUID{Bytes: id, Valid: true}, name)
		if errors.Is(err, ErrFinancialAccountNotFound) {
			return nil, catalogNotFound("financial_account_not_found", "Financial Account")
		}
		if err != nil {
			return nil, catalogUnavailable(ctx, err)
		}
		return &financialAccountOutput{CacheControl: "no-store", Body: account}, nil
	})
	huma.Register(api, huma.Operation{
		OperationID:   "create-source",
		Method:        http.MethodPost,
		Path:          "/api/sources",
		DefaultStatus: http.StatusCreated,
		Summary:       "Register a Source",
		Errors:        []int{http.StatusUnauthorized, http.StatusUnprocessableEntity},
		Security:      bearer,
	}, func(ctx context.Context, in *createSourceInput) (*sourceOutput, error) {
		if _, err := identities.RequireUser(ctx, in.Authorization); err != nil {
			return nil, err
		}
		name := strings.TrimSpace(in.Body.Name)
		if name == "" {
			return nil, invalidCatalogEntry()
		}
		source, err := s.createSource(ctx, name, in.Body.Side)
		if err != nil {
			return nil, catalogUnavailable(ctx, err)
		}
		return &sourceOutput{CacheControl: "no-store", Body: source}, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "edit-source",
		Method:      http.MethodPatch,
		Path:        "/api/sources/{id}",
		Summary:     "Edit a Source",
		Errors:      []int{http.StatusUnauthorized, http.StatusNotFound, http.StatusConflict, http.StatusUnprocessableEntity},
		Security:    bearer,
	}, func(ctx context.Context, in *editSourceInput) (*sourceOutput, error) {
		if _, err := identities.RequireUser(ctx, in.Authorization); err != nil {
			return nil, err
		}
		id, err := uuid.Parse(in.ID)
		if err != nil {
			return nil, catalogNotFound("source_not_found", "Source")
		}
		if in.Body.Name == nil && in.Body.Side == nil {
			return nil, invalidCatalogEntry()
		}
		if in.Body.Name != nil {
			name := strings.TrimSpace(*in.Body.Name)
			if name == "" {
				return nil, invalidCatalogEntry()
			}
			in.Body.Name = new(name)
		}
		entry, err := s.editSource(ctx, pgtype.UUID{Bytes: id, Valid: true}, in.Body.Name, in.Body.Side)
		switch {
		case errors.Is(err, ErrSourceNotFound):
			return nil, catalogNotFound("source_not_found", "Source")
		case errors.Is(err, errSourceSideLocked):
			return nil, httpapi.NewProblem(http.StatusConflict, "source_side_locked", "The Source side is locked by a confirmed import.")
		case err != nil:
			return nil, catalogUnavailable(ctx, err)
		}
		return &sourceOutput{CacheControl: "no-store", Body: entry}, nil
	})
}

func invalidCatalogEntry() *httpapi.Problem {
	return httpapi.NewProblem(http.StatusUnprocessableEntity, "invalid_catalog_entry", "Check the catalog fields.")
}

func catalogNotFound(code, entity string) *httpapi.Problem {
	return httpapi.NewProblem(http.StatusNotFound, code, "The "+entity+" could not be found.")
}

func catalogUnavailable(ctx context.Context, err error) error {
	slog.ErrorContext(ctx, "Could not update catalog", "error", err)
	return huma.Error500InternalServerError("The catalog is temporarily unavailable.")
}
