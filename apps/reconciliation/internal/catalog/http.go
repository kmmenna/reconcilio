package catalog

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/identity"
)

type listInput struct {
	Authorization string `header:"Authorization"`
}

type sourcesOutput struct {
	CacheControl string `header:"Cache-Control"`
	Body         []Source
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
		accounts, err := s.queries.ListFinancialAccounts(ctx)
		if err != nil {
			slog.ErrorContext(ctx, "Could not list Financial Accounts", "error", err)
			return nil, huma.Error500InternalServerError("Financial Accounts are temporarily unavailable.")
		}
		out := &financialAccountsOutput{CacheControl: "no-store", Body: make([]FinancialAccount, 0, len(accounts))}
		for _, account := range accounts {
			out.Body = append(out.Body, accountFromDB(account))
		}
		return out, nil
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
		sources, err := s.queries.ListSources(ctx)
		if err != nil {
			slog.ErrorContext(ctx, "Could not list Sources", "error", err)
			return nil, huma.Error500InternalServerError("Sources are temporarily unavailable.")
		}
		out := &sourcesOutput{CacheControl: "no-store", Body: make([]Source, 0, len(sources))}
		for _, source := range sources {
			out.Body = append(out.Body, sourceFromDB(source))
		}
		return out, nil
	})
}
