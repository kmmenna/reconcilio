package reconciliation

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/catalog"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/httpapi"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/identity"
)

type OpenTransaction struct {
	Transaction
	FinancialAccount         catalog.FinancialAccount `json:"financial_account"`
	HasPendingMatchProposals bool                     `json:"has_pending_match_proposals"`
	AmountDiscrepancy        *AmountDiscrepancy       `json:"amount_discrepancy,omitempty"`
}

type AmountDiscrepancy struct {
	PaymentReference string `json:"payment_reference"`
	Amount           string `json:"amount"`
	Currency         string `json:"currency"`
}

func (s *Service) ListOpenTransactions(ctx context.Context) ([]OpenTransaction, error) {
	rows, err := s.queries.ListOpenTransactions(ctx)
	if err != nil {
		return nil, err
	}
	transactions := make([]OpenTransaction, 0, len(rows))
	for _, row := range rows {
		item := presentTransaction(row.Transaction, row.SourceName)
		result := OpenTransaction{
			Transaction: item, FinancialAccount: presentAccount(row.FinancialAccount),
			HasPendingMatchProposals: row.HasPendingMatchProposals,
		}
		if row.DiscrepancyAmount != "" {
			result.AmountDiscrepancy = &AmountDiscrepancy{
				PaymentReference: item.PaymentReference, Amount: row.DiscrepancyAmount, Currency: item.Currency,
			}
		}
		transactions = append(transactions, result)
	}
	return transactions, nil
}

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
