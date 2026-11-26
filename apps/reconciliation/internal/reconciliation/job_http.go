package reconciliation

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"uuid"

	"github.com/danielgtaylor/huma/v2"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/catalog"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/httpapi"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/identity"
)

type jobInput struct {
	Authorization string `header:"Authorization"`
	ID            int64  `path:"id" minimum:"1"`
}

type jobOutput struct {
	CacheControl string `header:"Cache-Control"`
	Body         JobStatus
}

type reprocessOutput struct {
	CacheControl string `header:"Cache-Control"`
	Location     string `header:"Location"`
	Body         struct {
		JobID int64 `json:"job_id"`
	}
}

func (q *Queue) RegisterHTTP(api huma.API, identities *identity.Service) {
	q.registerReprocessingHTTP(api, identities)
	huma.Register(api, huma.Operation{
		OperationID: "get-reconciliation-job",
		Method:      http.MethodGet,
		Path:        "/api/jobs/{id}",
		Summary:     "Read reconciliation job status",
		Errors:      []int{http.StatusUnauthorized, http.StatusNotFound},
		Security:    []map[string][]string{{"bearer": {}}},
	}, func(ctx context.Context, in *jobInput) (*jobOutput, error) {
		if _, err := identities.RequireUser(ctx, in.Authorization); err != nil {
			return nil, err
		}
		job, err := q.Status(ctx, in.ID)
		if errors.Is(err, ErrJobNotFound) {
			return nil, httpapi.NewProblem(http.StatusNotFound, "job_not_found", "The reconciliation job could not be found.")
		}
		if err != nil {
			slog.ErrorContext(ctx, "Could not read reconciliation job", "job_id", in.ID, "error", err)
			return nil, huma.Error500InternalServerError("The job status is temporarily unavailable.")
		}
		return &jobOutput{CacheControl: "no-store", Body: job}, nil
	})
}

func (q *Queue) registerReprocessingHTTP(api huma.API, identities *identity.Service) {
	huma.Register(api, huma.Operation{
		OperationID:   "reprocess-financial-account",
		Method:        http.MethodPost,
		Path:          "/api/financial-accounts/{id}/reprocess",
		Summary:       "Reprocess a Financial Account's open Transactions",
		DefaultStatus: http.StatusAccepted,
		Errors:        []int{http.StatusUnauthorized, http.StatusNotFound},
		Security:      []map[string][]string{{"bearer": {}}},
	}, func(ctx context.Context, in *detailInput) (*reprocessOutput, error) {
		if _, err := identities.RequireUser(ctx, in.Authorization); err != nil {
			return nil, err
		}
		notFound := httpapi.NewProblem(http.StatusNotFound, "financial_account_not_found", "The Financial Account could not be found.")
		accountID, err := uuid.Parse(in.ID)
		if err != nil {
			return nil, notFound
		}
		jobID, err := q.Reprocess(ctx, pgtype.UUID{Bytes: accountID, Valid: true})
		if errors.Is(err, catalog.ErrFinancialAccountNotFound) {
			return nil, notFound
		}
		if err != nil {
			slog.ErrorContext(ctx, "Could not queue account reprocessing", "financial_account_id", in.ID, "error", err)
			return nil, huma.Error500InternalServerError("Reprocessing is temporarily unavailable.")
		}
		out := &reprocessOutput{CacheControl: "no-store", Location: fmt.Sprintf("/api/jobs/%d", jobID)}
		out.Body.JobID = jobID
		return out, nil
	})
}
