// Package ingestion stages CSV data for analyst review.
package ingestion

import (
	"cmp"
	"context"
	"crypto/sha256"
	"encoding/json/v2"
	"errors"
	"fmt"
	"maps"
	"slices"
	"time"
	"uuid"

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
	replayed          bool
	ID                string                     `json:"id" format:"uuid"`
	Status            string                     `json:"status"`
	Source            catalog.Source             `json:"source"`
	FinancialAccount  *catalog.FinancialAccount  `json:"financial_account,omitempty"`
	FinancialAccounts []catalog.FinancialAccount `json:"financial_accounts,omitempty"`
	AccountCoverage   []accountCoverage          `json:"account_coverage,omitempty"`
	PeriodStart       string                     `json:"period_start" format:"date"`
	PeriodEnd         string                     `json:"period_end" format:"date"`
	Rows              []previewRow               `json:"rows,omitempty"`
	Conflicts         []importConflict           `json:"conflicts,omitempty"`
	Decision          string                     `json:"decision,omitempty" enum:"keep_original,cancel"`
	ConfirmedAt       *time.Time                 `json:"confirmed_at,omitempty"`
	ConfirmedBy       string                     `json:"confirmed_by,omitempty" format:"uuid"`
	CanceledAt        *time.Time                 `json:"canceled_at,omitempty"`
	CanceledBy        string                     `json:"canceled_by,omitempty" format:"uuid"`
	TransactionCount  *int64                     `json:"transaction_count,omitempty"`
	Jobs              []scheduledJob             `json:"jobs,omitempty"`
}

type importAccount struct {
	id       pgtype.UUID
	entry    catalog.FinancialAccount
	coverage coveragePeriod
}

type assessedAccount struct {
	importAccount
	accountAssessment
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

type previewRequest struct {
	sourceID           string
	financialAccountID string
	periodStart        *string
	periodEnd          *string
	file               *decodedUpload
}

type decodedUpload struct {
	fingerprint [sha256.Size]byte
	rows        []decodedRow
	issues      []importIssue
}

func stagePreview(ctx context.Context, queries *ingestiondb.Queries, params ingestiondb.CreatePendingImportParams, source catalog.Source, accounts []importAccount, rows []stagedRow, exactReplay bool) (importView, error) {
	document := previewDocument{Rows: rows}
	if !exactReplay {
		_, conflicts, err := assessImport(ctx, queries, params.SourceID, accounts, rows)
		if err != nil {
			return importView{}, err
		}
		document.Conflicts = conflicts
	}
	params.SourceSide = source.Side
	data, err := json.Marshal(document)
	if err != nil {
		return importView{}, fmt.Errorf("encode preview rows: %w", err)
	}
	params.Preview = data
	id, err := queries.CreatePendingImport(ctx, params)
	if err != nil {
		return importView{}, err
	}
	assignments := make([]ingestiondb.AssignImportAccountsParams, 0, len(accounts))
	for _, account := range accounts {
		assignments = append(assignments, ingestiondb.AssignImportAccountsParams{
			ImportID: id, FinancialAccountID: account.id,
			PeriodStart: account.coverage.start, PeriodEnd: account.coverage.end, CoverageBasis: account.coverage.basis,
		})
	}
	if _, err := queries.AssignImportAccounts(ctx, assignments); err != nil {
		return importView{}, fmt.Errorf("assign Financial Accounts to import: %w", err)
	}
	return presentPreview(id, source, accounts, params.PeriodStart, params.PeriodEnd, document), nil
}

func assessImport(ctx context.Context, queries *ingestiondb.Queries, sourceID pgtype.UUID, accounts []importAccount, rows []stagedRow) ([]assessedAccount, []importConflict, error) {
	grouped := make(map[string][]stagedRow, len(accounts))
	for _, row := range rows {
		grouped[row.FinancialAccountID] = append(grouped[row.FinancialAccountID], row)
	}
	assessments := make([]assessedAccount, 0, len(accounts))
	var conflicts []importConflict
	var issues []importIssue
	for _, account := range accounts {
		assessment, err := assessAccount(ctx, queries, sourceID, account.id, grouped[account.entry.ID])
		if problem, ok := errors.AsType[*invalidImport](err); ok {
			issues = append(issues, problem.issues...)
			continue
		}
		if err != nil {
			return nil, nil, err
		}
		assessments = append(assessments, assessedAccount{importAccount: account, accountAssessment: assessment})
		conflicts = append(conflicts, assessment.Conflicts...)
	}
	if len(issues) > 0 {
		slices.SortStableFunc(issues, func(a, b importIssue) int { return cmp.Compare(a.Line, b.Line) })
		return nil, nil, importProblem("invalid_csv", "The CSV contains conflicting rows for a new Source Reference.", issues[:min(len(issues), maxRowIssues)]...)
	}
	for _, assessment := range assessments {
		if err := checkCoverage(ctx, queries, ingestiondb.FindBlockedCoverageParams{
			SourceID: sourceID, FinancialAccountID: assessment.id, IncomingStart: assessment.coverage.start, IncomingEnd: assessment.coverage.end,
			IncomingHasSourceReferences: assessment.AllHaveSourceReferences,
		}, assessment.entry); err != nil {
			return nil, nil, err
		}
	}
	slices.SortFunc(conflicts, func(a, b importConflict) int { return cmp.Compare(a.Line, b.Line) })
	return assessments, conflicts, nil
}

func (s *Service) getPreview(ctx context.Context, id pgtype.UUID) (importView, error) {
	record, err := s.queries.GetImport(ctx, id)
	if err != nil {
		return importView{}, err
	}
	if record.Status != "pending" {
		return readReceipt(record.Receipt)
	}
	accounts, err := listImportAccounts(ctx, s.queries, id)
	if err != nil {
		return importView{}, err
	}
	document, err := decodePreview(record.Preview, accounts)
	if err != nil {
		return importView{}, err
	}
	return presentPreview(
		record.ID,
		catalog.Source{ID: record.SourceID.String(), Name: record.SourceName, Side: record.SourceSide},
		accounts, record.PeriodStart, record.PeriodEnd, document,
	), nil
}

func lockImportAccounts(ctx context.Context, tx pgx.Tx, queries *ingestiondb.Queries, id pgtype.UUID) (ingestiondb.GetImportRow, []importAccount, error) {
	accounts, err := listImportAccounts(ctx, queries, id)
	if err != nil {
		return ingestiondb.GetImportRow{}, nil, err
	}
	if err := catalog.LockFinancialAccounts(ctx, tx, accountIDs(accounts)); err != nil {
		return ingestiondb.GetImportRow{}, nil, err
	}
	record, err := queries.GetImport(ctx, id)
	return record, accounts, err
}

func listImportAccounts(ctx context.Context, queries *ingestiondb.Queries, id pgtype.UUID) ([]importAccount, error) {
	entries, err := queries.ListImportAccounts(ctx, id)
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, pgx.ErrNoRows
	}
	accounts := make([]importAccount, 0, len(entries))
	for _, entry := range entries {
		accounts = append(accounts, importAccount{
			id:       entry.ID,
			entry:    catalog.FinancialAccount{ID: entry.ID.String(), Code: entry.Code, Name: entry.Name},
			coverage: coveragePeriod{start: entry.PeriodStart, end: entry.PeriodEnd, basis: entry.CoverageBasis},
		})
	}
	return accounts, nil
}

func resolveImportAccounts(rows []stagedRow, registered []catalog.FinancialAccount) ([]importAccount, error) {
	byID := make(map[string]catalog.FinancialAccount, len(registered))
	for _, account := range registered {
		byID[account.ID] = account
	}
	assigned := make(map[string]catalog.FinancialAccount)
	for _, row := range rows {
		assigned[row.FinancialAccountID] = byID[row.FinancialAccountID]
	}
	accounts := make([]importAccount, 0, len(assigned))
	for _, key := range slices.Sorted(maps.Keys(assigned)) {
		id, err := uuid.Parse(key)
		if err != nil {
			return nil, fmt.Errorf("decode registered Financial Account ID: %w", err)
		}
		accounts = append(accounts, importAccount{id: pgtype.UUID{Bytes: id, Valid: true}, entry: assigned[key]})
	}
	return accounts, nil
}

func accountIDs(accounts []importAccount) []pgtype.UUID {
	ids := make([]pgtype.UUID, 0, len(accounts))
	for _, account := range accounts {
		ids = append(ids, account.id)
	}
	return ids
}

func findReplay(ctx context.Context, queries *ingestiondb.Queries, sourceID pgtype.UUID, accountIDs []pgtype.UUID, fingerprint []byte) (ingestiondb.FindConfirmedReplayRow, error) {
	replay, err := queries.FindConfirmedReplay(ctx, ingestiondb.FindConfirmedReplayParams{
		SourceID: sourceID, AccountIds: accountIDs, FileFingerprint: fingerprint,
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

func decodePreview(data []byte, accounts []importAccount) (previewDocument, error) {
	var document previewDocument
	if err := json.Unmarshal(data, &document); err != nil {
		return previewDocument{}, fmt.Errorf("decode preview rows: %w", err)
	}
	if err := assignRowAccounts(document.Rows, accounts); err != nil {
		return previewDocument{}, err
	}
	return document, nil
}

func newImportView(id pgtype.UUID, source catalog.Source, accounts []importAccount, start, end pgtype.Date) importView {
	view := importView{
		ID: id.String(), Source: source, FinancialAccounts: make([]catalog.FinancialAccount, 0, len(accounts)),
		PeriodStart: start.Time.Format(time.DateOnly), PeriodEnd: end.Time.Format(time.DateOnly),
	}
	for _, account := range accounts {
		view.FinancialAccounts = append(view.FinancialAccounts, account.entry)
		view.AccountCoverage = append(view.AccountCoverage, accountCoverage{
			FinancialAccountID: account.entry.ID,
			PeriodStart:        account.coverage.start.Time.Format(time.DateOnly),
			PeriodEnd:          account.coverage.end.Time.Format(time.DateOnly),
			Basis:              account.coverage.basis,
		})
	}
	if len(accounts) == 1 {
		view.FinancialAccount = new(accounts[0].entry)
	}
	return view
}

func presentPreview(id pgtype.UUID, source catalog.Source, accounts []importAccount, start, end pgtype.Date, document previewDocument) importView {
	view := newImportView(id, source, accounts, start, end)
	view.Status = "pending"
	view.Conflicts = document.Conflicts
	view.Rows = make([]previewRow, 0, len(document.Rows))
	for _, row := range document.Rows {
		view.Rows = append(view.Rows, previewRow{
			rowFields: row.rowFields,
			Amount:    row.decimalAmount(),
			Currency:  row.Currency,
		})
	}
	return view
}
