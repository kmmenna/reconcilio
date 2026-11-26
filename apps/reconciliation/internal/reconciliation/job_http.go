package reconciliation

import (
	"context"
	"errors"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
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

func (q *Queue) RegisterHTTP(api huma.API, identities *identity.Service) {
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
