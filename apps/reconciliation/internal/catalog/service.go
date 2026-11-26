// Package catalog owns Financial Accounts and Sources.
package catalog

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/catalog/catalogdb"
)

var (
	ErrSourceNotFound           = errors.New("source not found")
	ErrFinancialAccountNotFound = errors.New("financial account not found")
	errSourceSideLocked         = errors.New("source side is locked")
)

type FinancialAccount struct {
	ID   string `json:"id" format:"uuid"`
	Code string `json:"code"`
	Name string `json:"name"`
}

type Source struct {
	ID   string `json:"id" format:"uuid"`
	Name string `json:"name"`
	Side string `json:"side" enum:"internal,external"`
}

type sourceEntry struct {
	Source
	SideEditable bool `json:"side_editable"`
}

type Service struct {
	pool    *pgxpool.Pool
	queries *catalogdb.Queries
}

func New(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool, queries: catalogdb.New(pool)}
}

func (s *Service) GetSource(ctx context.Context, id pgtype.UUID) (Source, error) {
	return readSource(s.queries.GetSource(ctx, id))
}

func (s *Service) GetFinancialAccount(ctx context.Context, id pgtype.UUID) (FinancialAccount, error) {
	account, err := s.queries.GetFinancialAccount(ctx, id)
	if errors.Is(err, pgx.ErrNoRows) {
		return FinancialAccount{}, ErrFinancialAccountNotFound
	}
	if err != nil {
		return FinancialAccount{}, fmt.Errorf("look up financial account: %w", err)
	}
	return accountFromDB(account), nil
}

func (s *Service) ListFinancialAccounts(ctx context.Context) ([]FinancialAccount, error) {
	accounts, err := s.queries.ListFinancialAccounts(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]FinancialAccount, 0, len(accounts))
	for _, account := range accounts {
		result = append(result, accountFromDB(account))
	}
	return result, nil
}

func (s *Service) editSource(ctx context.Context, id pgtype.UUID, name, side *string) (sourceEntry, error) {
	var entry sourceEntry
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		queries := s.queries.WithTx(tx)
		source, err := queries.LockSourceForEdit(ctx, id)
		if err != nil {
			return err
		}
		// Check committed imports after taking the Source lock. A waiting editor
		// must see a confirmation that committed while it waited.
		hasConfirmed, err := queries.SourceHasConfirmedImport(ctx, id)
		if err != nil {
			return err
		}
		if side != nil && *side != source.Side {
			if hasConfirmed {
				return errSourceSideLocked
			}
			source.Side = *side
		}
		if name != nil {
			source.Name = *name
		}
		source, err = queries.UpdateSource(ctx, catalogdb.UpdateSourceParams{
			ID: source.ID, Name: source.Name, Side: source.Side,
		})
		if err != nil {
			return err
		}
		entry = sourceEntry{Source: sourceFromDB(source), SideEditable: !hasConfirmed}
		return nil
	})
	return entry, err
}

func sourceFromDB(source catalogdb.Source) Source {
	return Source{ID: source.ID.String(), Name: source.Name, Side: source.Side}
}

func accountFromDB(account catalogdb.FinancialAccount) FinancialAccount {
	return FinancialAccount{ID: account.ID.String(), Code: account.Code, Name: account.Name}
}

func readSource(source catalogdb.Source, err error) (Source, error) {
	if errors.Is(err, pgx.ErrNoRows) {
		return Source{}, ErrSourceNotFound
	}
	if err != nil {
		return Source{}, fmt.Errorf("look up source: %w", err)
	}
	return sourceFromDB(source), nil
}

// LockSourceForImport holds the Source side stable while an import is confirmed.
func LockSourceForImport(ctx context.Context, tx pgx.Tx, id pgtype.UUID) (Source, error) {
	return readSource(catalogdb.New(tx).LockSourceForImport(ctx, id))
}

// LockFinancialAccounts serializes confirmation and reconciliation effects for the given accounts.
func LockFinancialAccounts(ctx context.Context, tx pgx.Tx, ids []pgtype.UUID) error {
	locked, err := catalogdb.New(tx).LockFinancialAccounts(ctx, ids)
	if err != nil {
		return fmt.Errorf("lock financial accounts: %w", err)
	}
	if len(locked) != len(ids) {
		return ErrFinancialAccountNotFound
	}
	return nil
}
