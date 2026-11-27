package reconciliation

import (
	"context"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/catalog"
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
