package ingestion

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/catalog"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/httpapi"
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

func (s *Service) resolve(ctx context.Context, id, analystID pgtype.UUID, cancel bool, keepIDs []string) (importView, error) {
	var receipt importView
	var reviewProblem *httpapi.Problem
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
		if record.Status != "pending" {
			if (cancel && record.Status == "canceled") || (!cancel && record.Status == "confirmed") {
				receipt, err = readReceipt(record.Receipt)
				return err
			}
			return errImportResolved
		}

		source := catalog.Source{ID: record.SourceID.String(), Name: record.SourceName, Side: record.SourceSide}
		account := catalog.FinancialAccount{ID: accountID.String(), Code: record.FinancialAccountCode, Name: record.FinancialAccountName}
		receipt = importView{
			ID: id.String(), Source: source, FinancialAccount: account,
			PeriodStart: record.PeriodStart.Time.Format(time.DateOnly), PeriodEnd: record.PeriodEnd.Time.Format(time.DateOnly),
		}
		var document previewDocument
		if err := json.Unmarshal(record.Preview, &document); err != nil {
			return fmt.Errorf("decode preview rows: %w", err)
		}
		if cancel {
			receipt.Status = "canceled"
			receipt.Decision = "cancel"
			receipt.Conflicts = document.Conflicts
			receipt.CanceledAt = new(time.Now().UTC().Truncate(time.Microsecond))
			receipt.CanceledBy = analystID.String()
			observation, err := json.Marshal(receipt)
			if err != nil {
				return fmt.Errorf("encode cancellation receipt: %w", err)
			}
			return queries.CancelImport(ctx, ingestiondb.CancelImportParams{ID: id, Receipt: observation})
		}
		if record.SourceSide != record.CurrentSourceSide {
			return errSourceSideChanged
		}
		replay, err := findReplay(ctx, queries, record.SourceID, accountID, record.FileFingerprint)
		if err != nil {
			return err
		}
		if replay.ID.Valid {
			receipt, err = readReceipt(replay.Receipt)
			if err != nil {
				return err
			}
			return queries.ResolveImportReplay(ctx, ingestiondb.ResolveImportReplayParams{ID: id, OriginalID: replay.ID})
		}

		assessment, err := assessAccount(ctx, queries, record.SourceID, accountID, document.Rows)
		if err != nil {
			return err
		}
		if err := checkCoverage(ctx, queries, ingestiondb.FindBlockedCoverageParams{
			SourceID: record.SourceID, FinancialAccountID: accountID,
			IncomingStart: record.PeriodStart, IncomingEnd: record.PeriodEnd,
			IncomingHasSourceReferences: assessment.AllHaveSourceReferences,
		}, account); err != nil {
			return err
		}
		if !slices.EqualFunc(document.Conflicts, assessment.Conflicts, func(a, b importConflict) bool { return a.ID == b.ID }) {
			document.Conflicts = assessment.Conflicts
			preview := presentPreview(id, source, account, record.PeriodStart, record.PeriodEnd, document)
			reviewProblem = conflictProblem("import_conflicts_changed", "The conflicts changed. Review the updated preview.", preview)
			// Commit the updated preview before returning 409 so the same ID can be reviewed again.
			data, err := json.Marshal(document)
			if err != nil {
				return fmt.Errorf("encode updated preview: %w", err)
			}
			return queries.UpdateImportPreview(ctx, ingestiondb.UpdateImportPreviewParams{ID: id, Preview: data})
		}
		if !acceptsConflicts(document.Conflicts, keepIDs) {
			preview := presentPreview(id, source, account, record.PeriodStart, record.PeriodEnd, document)
			return conflictProblem("import_conflicts_require_review", "Review the conflicts before confirming.", preview)
		}

		transactions := make([]ingestiondb.InsertTransactionsParams, 0, len(assessment.NewRows))
		for _, row := range assessment.NewRows {
			date, err := time.Parse(time.DateOnly, row.TransactionDate)
			if err != nil {
				return fmt.Errorf("decode staged transaction date: %w", err)
			}
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
			PeriodEnd: record.PeriodEnd, AllHaveSourceReferences: assessment.AllHaveSourceReferences, JobID: jobID,
		}); err != nil {
			return fmt.Errorf("record import coverage: %w", err)
		}
		confirmedAt := time.Now().UTC().Truncate(time.Microsecond)
		receipt.Status = "confirmed"
		receipt.ConfirmedAt = &confirmedAt
		receipt.ConfirmedBy = analystID.String()
		receipt.TransactionCount = new(count)
		receipt.Jobs = []scheduledJob{{FinancialAccount: account, JobID: jobID}}
		if len(document.Conflicts) > 0 {
			receipt.Decision = "keep_original"
			receipt.Conflicts = document.Conflicts
		}
		observation, err := json.Marshal(receipt)
		if err != nil {
			return fmt.Errorf("encode import receipt: %w", err)
		}
		return queries.ConfirmImport(ctx, ingestiondb.ConfirmImportParams{
			ID: id, ConfirmedAt: pgtype.Timestamptz{Time: confirmedAt, Valid: true}, ConfirmedBy: analystID, Receipt: observation,
		})
	})
	if err == nil && reviewProblem != nil {
		err = reviewProblem
	}
	return receipt, err
}

func acceptsConflicts(conflicts []importConflict, keepIDs []string) bool {
	if len(conflicts) != len(keepIDs) {
		return false
	}
	reviewed := make(map[string]bool, len(keepIDs))
	for _, id := range keepIDs {
		reviewed[id] = true
	}
	for _, conflict := range conflicts {
		if !reviewed[conflict.ID] {
			return false
		}
	}
	return true
}

func conflictProblem(code, detail string, preview importView) *httpapi.Problem {
	problem := httpapi.NewProblem(http.StatusConflict, code, detail)
	problem.Context = importProblemContext{Preview: &preview}
	return problem
}
