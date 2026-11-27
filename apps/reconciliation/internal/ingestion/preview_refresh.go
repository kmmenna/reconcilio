package ingestion

import (
	"context"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/catalog"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/ingestion/ingestiondb"
)

func (s *Service) refreshPreview(ctx context.Context, id, analystID pgtype.UUID) (importView, error) {
	var view importView
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		queries := s.queries.WithTx(tx)
		record, accounts, err := lockImportAccounts(ctx, tx, queries, id)
		if err != nil {
			return err
		}
		if record.Status != "pending" {
			return errImportResolved
		}
		source, err := catalog.LockSourceForImport(ctx, tx, record.SourceID)
		if err != nil {
			return err
		}
		document, err := decodePreview(record.Preview, accounts)
		if err != nil {
			return err
		}
		if issues := coverageDateIssues(document.Rows, accounts); len(issues) > 0 {
			return importProblem("invalid_csv", "The CSV contains invalid data.", issues...)
		}
		replay, err := findReplay(ctx, queries, record.SourceID, accountIDs(accounts), record.FileFingerprint)
		if err != nil {
			return err
		}
		view, err = stagePreview(ctx, queries, ingestiondb.CreatePendingImportParams{
			SourceID: record.SourceID, CreatedBy: analystID, PeriodStart: record.PeriodStart,
			PeriodEnd: record.PeriodEnd, FileFingerprint: record.FileFingerprint,
		}, source, accounts, document.Rows, replay.ID.Valid)
		return err
	})
	return view, err
}
