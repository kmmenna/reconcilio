package reconciliation

import (
	"context"
	"github.com/danielgtaylor/huma/v2"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/httpapi"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/identity"
	"log/slog"
	"net/http"
)

type openTransactionsInput struct {
	Authorization string `header:"Authorization"`
	Status        string `query:"status" enum:"open" default:"open"`
}

type openTransactionsOutput struct {
	CacheControl string `header:"Cache-Control"`
	Body         []OpenTransaction
}

func (s *Service) registerOpenTransactionsHTTP(api huma.API, identities *identity.Service, bearer []map[string][]string) {
	huma.Register(api, huma.Operation{
		OperationID: "list-open-transactions",
		Method:      http.MethodGet,
		Path:        "/api/transactions",
		Summary:     "List transactions without an active reconciliation",
		Errors:      []int{http.StatusUnauthorized, http.StatusUnprocessableEntity},
		Security:    bearer,
	}, func(ctx context.Context, in *openTransactionsInput) (*openTransactionsOutput, error) {
		if _, err := identities.RequireUser(ctx, in.Authorization); err != nil {
			return nil, err
		}
		result, err := s.ListOpenTransactions(ctx)
		if err != nil {
			slog.ErrorContext(ctx, "Could not list open transactions", "error", err)
			return nil, httpapi.NewProblem(http.StatusInternalServerError, "open_transactions_unavailable", "Open transactions are temporarily unavailable.")
		}
		return &openTransactionsOutput{CacheControl: "no-store", Body: result}, nil
	})
}
