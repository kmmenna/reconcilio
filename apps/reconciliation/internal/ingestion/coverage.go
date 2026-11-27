package ingestion

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/catalog"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/ingestion/ingestiondb"
)

type accountCoverage struct {
	FinancialAccountID string `json:"financial_account_id" format:"uuid"`
	PeriodStart        string `json:"period_start" format:"date"`
	PeriodEnd          string `json:"period_end" format:"date"`
	Basis              string `json:"basis" enum:"inferred,declared"`
}

type coveragePeriod struct {
	start pgtype.Date
	end   pgtype.Date
	basis string
}

func assignCoverage(accounts []importAccount, rows []stagedRow, declared coveragePeriod) coveragePeriod {
	if declared.start.Valid {
		declared.basis = "declared"
		for i := range accounts {
			accounts[i].coverage = declared
		}
		return declared
	}

	positions := make(map[string]int, len(accounts))
	for i, account := range accounts {
		positions[account.entry.ID] = i
		accounts[i].coverage.basis = "inferred"
	}
	var summary coveragePeriod
	for _, row := range rows {
		// Validation has already checked every date and account assignment.
		date, _ := parseDate(row.TransactionDate)
		summary.include(date)
		position := positions[row.FinancialAccountID]
		accounts[position].coverage.include(date)
	}
	return summary
}

func (p *coveragePeriod) include(date time.Time) {
	if !p.start.Valid || date.Before(p.start.Time) {
		p.start = pgtype.Date{Time: date, Valid: true}
	}
	if !p.end.Valid || date.After(p.end.Time) {
		p.end = pgtype.Date{Time: date, Valid: true}
	}
}

type coverageOverlap struct {
	ImportID                    string                   `json:"import_id" format:"uuid"`
	FinancialAccount            catalog.FinancialAccount `json:"financial_account"`
	PeriodStart                 string                   `json:"period_start" format:"date"`
	PeriodEnd                   string                   `json:"period_end" format:"date"`
	PreviousHasSourceReferences bool                     `json:"previous_has_source_references"`
	IncomingHasSourceReferences bool                     `json:"incoming_has_source_references"`
}

func checkCoverage(ctx context.Context, queries *ingestiondb.Queries, params ingestiondb.FindBlockedCoverageParams, account catalog.FinancialAccount) error {
	prior, err := queries.FindBlockedCoverage(ctx, params)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("check import coverage: %w", err)
	}
	return &coverageBlocked{overlap: coverageOverlap{
		ImportID: prior.ImportID.String(), FinancialAccount: account,
		PeriodStart: prior.PeriodStart.Time.Format(time.DateOnly), PeriodEnd: prior.PeriodEnd.Time.Format(time.DateOnly),
		PreviousHasSourceReferences: prior.AllHaveSourceReferences,
		IncomingHasSourceReferences: params.IncomingHasSourceReferences,
	}}
}
