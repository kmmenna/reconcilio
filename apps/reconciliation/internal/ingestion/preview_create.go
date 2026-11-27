package ingestion

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/catalog"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/ingestion/ingestiondb"
	"uuid"
)

func (s *Service) createPreview(ctx context.Context, userID pgtype.UUID, input previewRequest) (importView, error) {
	params, accountID, issues := importSelections(input)
	if len(issues) > 0 {
		return importView{}, importProblem("invalid_import_selection", "Check the import selections.", issues...)
	}
	if _, err := s.catalog.GetSource(ctx, params.SourceID); err != nil {
		if errors.Is(err, catalog.ErrSourceNotFound) {
			return importView{}, importProblem("invalid_import_selection", "Check the import selections.",
				importIssue{Field: "source_id", Reason: "unknown_source"})
		}
		return importView{}, err
	}
	var accounts []importAccount
	var registered []catalog.FinancialAccount
	var accountsByCode map[string]string
	if accountID.Valid {
		account, err := s.catalog.GetFinancialAccount(ctx, accountID)
		if errors.Is(err, catalog.ErrFinancialAccountNotFound) {
			return importView{}, importProblem("invalid_import_selection", "Check the import selections.",
				importIssue{Field: "financial_account_id", Reason: "unknown_financial_account"})
		}
		if err != nil {
			return importView{}, err
		}
		accounts = []importAccount{{id: accountID, entry: account}}
	} else {
		var err error
		registered, err = s.catalog.ListFinancialAccounts(ctx)
		if err != nil {
			return importView{}, err
		}
		accountsByCode = make(map[string]string, len(registered))
		for _, account := range registered {
			accountsByCode[account.Code] = account.ID
		}
	}

	period := coveragePeriod{start: params.PeriodStart, end: params.PeriodEnd}
	rows, issues := validatePreviewRows(input.file, period, accountsByCode)
	if len(issues) > 0 {
		return importView{}, importProblem("invalid_csv", "The CSV contains invalid data.", issues...)
	}
	if !accountID.Valid {
		var err error
		accounts, err = resolveImportAccounts(rows, registered)
		if err != nil {
			return importView{}, err
		}
	}
	if err := assignRowAccounts(rows, accounts); err != nil {
		return importView{}, err
	}
	period = assignCoverage(accounts, rows, period)
	params.PeriodStart, params.PeriodEnd = period.start, period.end
	params.FileFingerprint = input.file.fingerprint[:]
	params.CreatedBy = userID
	return s.savePreview(ctx, params, accounts, rows)
}

func importSelections(input previewRequest) (ingestiondb.CreatePendingImportParams, pgtype.UUID, []importIssue) {
	var issues []importIssue
	add := func(field, reason string) {
		issues = append(issues, importIssue{Field: field, Reason: reason})
	}

	source, sourceErr := uuid.Parse(input.sourceID)
	if sourceErr != nil {
		add("source_id", "invalid_source")
	}
	account, accountErr := uuid.Parse(input.financialAccountID)
	if input.financialAccountID != "" && accountErr != nil {
		add("financial_account_id", "invalid_financial_account")
	}
	var start, end pgtype.Date
	if input.periodStart != nil || input.periodEnd != nil {
		parseEndpoint := func(field string, input *string) pgtype.Date {
			if input != nil {
				if date, valid := parseDate(*input); valid {
					return pgtype.Date{Time: date, Valid: true}
				}
			}
			add(field, "invalid_date")
			return pgtype.Date{}
		}
		start = parseEndpoint("period_start", input.periodStart)
		end = parseEndpoint("period_end", input.periodEnd)
		if start.Valid && end.Valid && end.Time.Before(start.Time) {
			add("period_end", "invalid_period")
		}
	}
	if input.file == nil {
		add("file", "missing_file")
	}
	return ingestiondb.CreatePendingImportParams{
		SourceID:    pgtype.UUID{Bytes: source, Valid: sourceErr == nil},
		PeriodStart: start,
		PeriodEnd:   end,
	}, pgtype.UUID{Bytes: account, Valid: accountErr == nil}, issues
}

func (s *Service) savePreview(ctx context.Context, params ingestiondb.CreatePendingImportParams, accounts []importAccount, rows []stagedRow) (importView, error) {
	var view importView
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		ids := accountIDs(accounts)
		if err := catalog.LockFinancialAccounts(ctx, tx, ids); err != nil {
			return err
		}
		queries := s.queries.WithTx(tx)
		replay, err := findReplay(ctx, queries, params.SourceID, ids, params.FileFingerprint)
		if err != nil {
			return err
		}
		if replay.ID.Valid {
			view, err = readReceipt(replay.Receipt)
			view.replayed = true
			return err
		}
		source, err := catalog.LockSourceForImport(ctx, tx, params.SourceID)
		if err != nil {
			return err
		}
		view, err = stagePreview(ctx, queries, params, source, accounts, rows, false)
		return err
	})
	return view, err
}
