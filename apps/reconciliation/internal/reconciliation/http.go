package reconciliation

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"uuid"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/httpapi"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/identity"
)

type listInput struct {
	Authorization      string `header:"Authorization"`
	FinancialAccountID string `query:"financial_account_id" format:"uuid"`
}

type listOutput struct {
	CacheControl string `header:"Cache-Control"`
	Body         []Summary
}

type detailInput struct {
	Authorization string `header:"Authorization"`
	ID            string `path:"id"`
}

type detailOutput struct {
	CacheControl string `header:"Cache-Control"`
	Body         Detail
}

func (s *Service) RegisterHTTP(api huma.API, identities *identity.Service) {
	bearer := []map[string][]string{{"bearer": {}}}
	s.registerReviewHTTP(api, identities, bearer)
	s.registerOpenTransactionsHTTP(api, identities, bearer)
	s.registerDecisionHTTP(api, identities, bearer)
	huma.Register(api, huma.Operation{
		OperationID: "list-reconciliations",
		Method:      http.MethodGet,
		Path:        "/api/reconciliations",
		Summary:     "List confirmed reconciliations",
		Errors:      []int{http.StatusUnauthorized, http.StatusUnprocessableEntity},
		Security:    bearer,
	}, func(ctx context.Context, in *listInput) (*listOutput, error) {
		if _, err := identities.RequireUser(ctx, in.Authorization); err != nil {
			return nil, err
		}
		var accountID pgtype.UUID
		if in.FinancialAccountID != "" {
			id, err := uuid.Parse(in.FinancialAccountID)
			if err != nil {
				return nil, httpapi.NewProblem(http.StatusUnprocessableEntity, "invalid_financial_account", "Choose a valid Financial Account.")
			}
			accountID = pgtype.UUID{Bytes: id, Valid: true}
		}
		rows, err := s.List(ctx, accountID)
		if err != nil {
			slog.ErrorContext(ctx, "Could not list reconciliations", "error", err)
			return nil, huma.Error500InternalServerError("Reconciliations are temporarily unavailable.")
		}
		return &listOutput{CacheControl: "no-store", Body: rows}, nil
	})
	huma.Register(api, huma.Operation{
		OperationID: "get-reconciliation",
		Method:      http.MethodGet,
		Path:        "/api/reconciliations/{id}",
		Summary:     "Read a reconciliation and its recorded evidence",
		Errors:      []int{http.StatusUnauthorized, http.StatusNotFound},
		Security:    bearer,
	}, func(ctx context.Context, in *detailInput) (*detailOutput, error) {
		if _, err := identities.RequireUser(ctx, in.Authorization); err != nil {
			return nil, err
		}
		id, err := uuid.Parse(in.ID)
		if err != nil {
			return nil, reconciliationNotFound()
		}
		detail, err := s.Get(ctx, pgtype.UUID{Bytes: id, Valid: true})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, reconciliationNotFound()
		}
		if err != nil {
			slog.ErrorContext(ctx, "Could not read reconciliation", "reconciliation_id", in.ID, "error", err)
			return nil, huma.Error500InternalServerError("The reconciliation is temporarily unavailable.")
		}
		return &detailOutput{CacheControl: "no-store", Body: detail}, nil
	})
}

func reconciliationNotFound() *httpapi.Problem {
	return httpapi.NewProblem(http.StatusNotFound, "reconciliation_not_found", "The reconciliation could not be found.")
}
