// Package ingestion stages CSV data for analyst review.
package ingestion

import (
	"context"
	"crypto/sha256"
	"encoding/json/v2"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/catalog"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/ingestion/ingestiondb"
)

type previewDocument struct {
	Rows []stagedRow `json:"rows"`
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
	ConfirmedAt      *time.Time               `json:"confirmed_at,omitempty"`
	ConfirmedBy      string                   `json:"confirmed_by,omitempty" format:"uuid"`
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

func (s *Service) savePreview(ctx context.Context, params ingestiondb.CreatePendingImportParams, data []byte, rows []stagedRow) (pgtype.UUID, error) {
	document, err := json.Marshal(previewDocument{Rows: rows})
	if err != nil {
		return pgtype.UUID{}, fmt.Errorf("encode preview rows: %w", err)
	}
	fingerprint := sha256.Sum256(data)
	params.FileFingerprint = fingerprint[:]
	params.Preview = document
	return s.queries.CreatePendingImport(ctx, params)
}

func (s *Service) getPreview(ctx context.Context, id pgtype.UUID) (importView, error) {
	record, err := s.queries.GetImport(ctx, id)
	if err != nil {
		return importView{}, err
	}
	if record.Status == "confirmed" {
		var receipt importView
		if err := json.Unmarshal(record.Receipt, &receipt); err != nil {
			return importView{}, fmt.Errorf("decode import receipt: %w", err)
		}
		return receipt, nil
	}
	var document previewDocument
	if err := json.Unmarshal(record.Preview, &document); err != nil {
		return importView{}, fmt.Errorf("decode preview rows: %w", err)
	}
	return presentPreview(
		record.ID,
		record.Status,
		catalog.Source{ID: record.SourceID.String(), Name: record.SourceName, Side: record.SourceSide},
		catalog.FinancialAccount{ID: record.FinancialAccountID.String(), Code: record.FinancialAccountCode, Name: record.FinancialAccountName},
		record.PeriodStart.Time.Format(time.DateOnly),
		record.PeriodEnd.Time.Format(time.DateOnly),
		document.Rows,
	), nil
}

func presentPreview(id pgtype.UUID, status string, source catalog.Source, account catalog.FinancialAccount, start, end string, staged []stagedRow) importView {
	rows := make([]previewRow, 0, len(staged))
	for _, row := range staged {
		rows = append(rows, previewRow{
			rowFields: row.rowFields,
			Amount:    row.decimalAmount(),
			Currency:  row.Currency,
		})
	}
	return importView{
		ID:               id.String(),
		Status:           status,
		Source:           source,
		FinancialAccount: account,
		PeriodStart:      start,
		PeriodEnd:        end,
		Rows:             rows,
	}
}
