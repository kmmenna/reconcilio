package ingestion

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/catalog"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/ingestion/ingestiondb"
)

var (
	errImportResolved    = errors.New("import has already been resolved")
	errSourceSideChanged = errors.New("source side has changed")
)

type jobEnqueuer interface {
	EnqueueTx(context.Context, pgx.Tx, pgtype.UUID) (int64, error)
}

type scheduledJob struct {
	FinancialAccount catalog.FinancialAccount `json:"financial_account"`
	JobID            int64                    `json:"job_id"`
}

func (s *Service) confirm(ctx context.Context, id, analystID pgtype.UUID) (importView, error) {
	var receipt importView
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		queries := s.queries.WithTx(tx)
		accountID, err := queries.GetImportAccountID(ctx, id)
		if err != nil {
			return err
		}
		if err := catalog.LockFinancialAccounts(ctx, tx, []pgtype.UUID{accountID}); err != nil {
			return err
		}
		record, err := queries.GetImportForConfirmation(ctx, id)
		if err != nil {
			return err
		}
		switch record.Status {
		case "confirmed":
			return json.Unmarshal(record.Receipt, &receipt)
		case "pending":
		default:
			return errImportResolved
		}
		if record.SourceSide != record.CurrentSourceSide {
			return errSourceSideChanged
		}

		var document previewDocument
		if err := json.Unmarshal(record.Preview, &document); err != nil {
			return fmt.Errorf("decode preview rows: %w", err)
		}
		allHaveReferences := true
		transactions := make([]ingestiondb.InsertTransactionsParams, 0, len(document.Rows))
		for _, row := range document.Rows {
			date, err := time.Parse(time.DateOnly, row.TransactionDate)
			if err != nil {
				return fmt.Errorf("decode staged transaction date: %w", err)
			}
			allHaveReferences = allHaveReferences && row.SourceReference != ""
			transactions = append(transactions, ingestiondb.InsertTransactionsParams{
				ImportID: id, FinancialAccountID: accountID, SourceID: record.SourceID, SourceSide: record.SourceSide,
				SourceReference:  pgtype.Text{String: row.SourceReference, Valid: row.SourceReference != ""},
				PaymentReference: pgtype.Text{String: row.PaymentReference, Valid: row.PaymentReference != ""},
				TransactionDate:  pgtype.Date{Time: date, Valid: true},
				AmountMinor:      row.MinorUnits, Currency: row.Currency, MinorDigits: int32(row.MinorDigits), Description: row.Description,
			})
		}
		count, err := queries.InsertTransactions(ctx, transactions)
		if err != nil {
			return fmt.Errorf("persist imported transactions: %w", err)
		}
		jobID, err := s.jobs.EnqueueTx(ctx, tx, accountID)
		if err != nil {
			return err
		}
		if err := queries.InsertImportCoverage(ctx, ingestiondb.InsertImportCoverageParams{
			ImportID: id, FinancialAccountID: accountID, SourceID: record.SourceID, PeriodStart: record.PeriodStart,
			PeriodEnd: record.PeriodEnd, AllHaveSourceReferences: allHaveReferences, JobID: jobID,
		}); err != nil {
			return fmt.Errorf("record import coverage: %w", err)
		}
		account := catalog.FinancialAccount{ID: accountID.String(), Code: record.FinancialAccountCode, Name: record.FinancialAccountName}
		confirmedAt := time.Now().UTC().Truncate(time.Microsecond)
		receipt = importView{
			ID: id.String(), Status: "confirmed",
			Source:           catalog.Source{ID: record.SourceID.String(), Name: record.SourceName, Side: record.SourceSide},
			FinancialAccount: account,
			PeriodStart:      record.PeriodStart.Time.Format(time.DateOnly), PeriodEnd: record.PeriodEnd.Time.Format(time.DateOnly),
			ConfirmedAt: &confirmedAt, ConfirmedBy: analystID.String(), TransactionCount: new(count),
			Jobs: []scheduledJob{{FinancialAccount: account, JobID: jobID}},
		}
		observation, err := json.Marshal(receipt)
		if err != nil {
			return fmt.Errorf("encode import receipt: %w", err)
		}
		return queries.ConfirmImport(ctx, ingestiondb.ConfirmImportParams{
			ID: id, ConfirmedAt: pgtype.Timestamptz{Time: confirmedAt, Valid: true}, ConfirmedBy: analystID, Receipt: observation,
		})
	})
	return receipt, err
}
