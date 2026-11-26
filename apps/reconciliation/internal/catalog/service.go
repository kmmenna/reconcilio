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

type Service struct {
	queries *catalogdb.Queries
}

func New(pool *pgxpool.Pool) *Service {
	return &Service{queries: catalogdb.New(pool)}
}

func (s *Service) GetSelection(ctx context.Context, sourceID, accountID pgtype.UUID) (Source, FinancialAccount, error) {
	source, err := s.queries.GetSource(ctx, sourceID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Source{}, FinancialAccount{}, ErrSourceNotFound
	}
	if err != nil {
		return Source{}, FinancialAccount{}, fmt.Errorf("look up source: %w", err)
	}
	account, err := s.queries.GetFinancialAccount(ctx, accountID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Source{}, FinancialAccount{}, ErrFinancialAccountNotFound
	}
	if err != nil {
		return Source{}, FinancialAccount{}, fmt.Errorf("look up financial account: %w", err)
	}
	return sourceFromDB(source), accountFromDB(account), nil
}

func sourceFromDB(source catalogdb.Source) Source {
	return Source{ID: source.ID.String(), Name: source.Name, Side: source.Side}
}

func accountFromDB(account catalogdb.FinancialAccount) FinancialAccount {
	return FinancialAccount{ID: account.ID.String(), Code: account.Code, Name: account.Name}
}
