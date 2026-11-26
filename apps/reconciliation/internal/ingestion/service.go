// Package ingestion stages CSV data for analyst review.
package ingestion

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/catalog"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/ingestion/ingestiondb"
)

type previewDocument struct {
	Rows      []stagedRow      `json:"rows"`
	Conflicts []importConflict `json:"conflicts,omitempty"`
}

type previewRow struct {
	rowFields
	Amount   string `json:"amount"`
	Currency string `json:"currency"`
}

type importView struct {
	ID               string                   `json:"id" format:"uuid"`
	Status           string                   `json:"status"`
	Source           catalog.Source           `json:"source"`
	FinancialAccount catalog.FinancialAccount `json:"financial_account"`
	PeriodStart      string                   `json:"period_start" format:"date"`
	PeriodEnd        string                   `json:"period_end" format:"date"`
	Rows             []previewRow             `json:"rows,omitempty"`
	Conflicts        []importConflict         `json:"conflicts,omitempty"`
	Decision         string                   `json:"decision,omitempty" enum:"keep_original,cancel"`
	ConfirmedAt      *time.Time               `json:"confirmed_at,omitempty"`
	ConfirmedBy      string                   `json:"confirmed_by,omitempty" format:"uuid"`
	CanceledAt       *time.Time               `json:"canceled_at,omitempty"`
	CanceledBy       string                   `json:"canceled_by,omitempty" format:"uuid"`
	TransactionCount *int64                   `json:"transaction_count,omitempty"`
	Jobs             []scheduledJob           `json:"jobs,omitempty"`
}

type Service struct {
	pool    *pgxpool.Pool
	jobs    jobEnqueuer
	queries *ingestiondb.Queries
	catalog *catalog.Service
}

func New(pool *pgxpool.Pool, entries *catalog.Service, jobs jobEnqueuer) *Service {
	return &Service{pool: pool, queries: ingestiondb.New(pool), catalog: entries, jobs: jobs}
}

func (s *Service) savePreview(ctx context.Context, params ingestiondb.CreatePendingImportParams, source catalog.Source, account catalog.FinancialAccount, rows []stagedRow) (importView, error) {
	var view importView
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		if err := catalog.LockFinancialAccounts(ctx, tx, []pgtype.UUID{params.FinancialAccountID}); err != nil {
			return err
		}
		queries := s.queries.WithTx(tx)
		replay, err := findReplay(ctx, queries, params.SourceID, params.FinancialAccountID, params.FileFingerprint)
		if err != nil {
			return err
		}
		if replay.ID.Valid {
			view, err = readReceipt(replay.Receipt)
			return err
		}

		assessment, err := assessAccount(ctx, queries, params.SourceID, params.FinancialAccountID, rows)
		if err != nil {
			return err
		}
		if err := checkCoverage(ctx, queries, ingestiondb.FindBlockedCoverageParams{
			SourceID: params.SourceID, FinancialAccountID: params.FinancialAccountID,
			IncomingStart: params.PeriodStart, IncomingEnd: params.PeriodEnd,
			IncomingHasSourceReferences: assessment.AllHaveSourceReferences,
		}, account); err != nil {
			return err
		}
		document := previewDocument{Rows: rows, Conflicts: assessment.Conflicts}
		params.Preview, err = json.Marshal(document)
		if err != nil {
			return fmt.Errorf("encode preview rows: %w", err)
		}
		id, err := queries.CreatePendingImport(ctx, params)
		if err != nil {
			return err
		}
		view = presentPreview(id, source, account, params.PeriodStart, params.PeriodEnd, document)
		return nil
	})
	return view, err
}

func (s *Service) getPreview(ctx context.Context, id pgtype.UUID) (importView, error) {
	record, err := s.queries.GetImport(ctx, id)
	if err != nil {
		return importView{}, err
	}
	if record.Status != "pending" {
		return readReceipt(record.Receipt)
	}
	var document previewDocument
	if err := json.Unmarshal(record.Preview, &document); err != nil {
		return importView{}, fmt.Errorf("decode preview rows: %w", err)
	}
	return presentPreview(
		record.ID,
		catalog.Source{ID: record.SourceID.String(), Name: record.SourceName, Side: record.SourceSide},
		catalog.FinancialAccount{ID: record.FinancialAccountID.String(), Code: record.FinancialAccountCode, Name: record.FinancialAccountName},
		record.PeriodStart, record.PeriodEnd,
		document,
	), nil
}

func findReplay(ctx context.Context, queries *ingestiondb.Queries, sourceID, accountID pgtype.UUID, fingerprint []byte) (ingestiondb.FindConfirmedReplayRow, error) {
	replay, err := queries.FindConfirmedReplay(ctx, ingestiondb.FindConfirmedReplayParams{
		SourceID: sourceID, FinancialAccountID: accountID, FileFingerprint: fingerprint,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return ingestiondb.FindConfirmedReplayRow{}, nil
	}
	if err != nil {
		return ingestiondb.FindConfirmedReplayRow{}, fmt.Errorf("check file replay: %w", err)
	}
	return replay, nil
}

func readReceipt(data []byte) (importView, error) {
	var receipt importView
	if err := json.Unmarshal(data, &receipt); err != nil {
		return importView{}, fmt.Errorf("decode import receipt: %w", err)
	}
	return receipt, nil
}

func presentPreview(id pgtype.UUID, source catalog.Source, account catalog.FinancialAccount, start, end pgtype.Date, document previewDocument) importView {
	rows := make([]previewRow, 0, len(document.Rows))
	for _, row := range document.Rows {
		rows = append(rows, previewRow{
			rowFields: row.rowFields,
			Amount:    row.decimalAmount(),
			Currency:  row.Currency,
		})
	}
	return importView{
		ID:               id.String(),
		Status:           "pending",
		Source:           source,
		FinancialAccount: account,
		PeriodStart:      start.Time.Format(time.DateOnly),
		PeriodEnd:        end.Time.Format(time.DateOnly),
		Rows:             rows,
		Conflicts:        document.Conflicts,
	}
}
