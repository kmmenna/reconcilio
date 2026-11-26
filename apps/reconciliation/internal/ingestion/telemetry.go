package ingestion

import (
	"context"
	"errors"
	"net/http"
	"uuid"

	"github.com/danielgtaylor/huma/v2"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/telemetry"
)

func startImport(ctx context.Context, operation, id string) (context.Context, func(*previewOutput, error)) {
	var result telemetry.ImportResult
	if parsed, err := uuid.Parse(id); err == nil {
		result.ID = parsed.String()
		ctx = telemetry.WithImport(ctx, result.ID)
	}
	ctx, finish := telemetry.StartImport(ctx, operation)
	return ctx, func(out *previewOutput, err error) {
		if out != nil {
			result.ID = out.Body.ID
			result.Outcome = out.Body.Status
			result.TransactionCount = out.Body.TransactionCount
			for _, job := range out.Body.Jobs {
				result.JobIDs = append(result.JobIDs, job.JobID)
			}
			if out.Body.replayed {
				result.Outcome = "replayed"
				result.TransactionCount = nil
			}
		} else if problem, ok := errors.AsType[huma.StatusError](err); ok && problem.GetStatus() < http.StatusInternalServerError {
			result.Outcome = "rejected"
		} else {
			result.Outcome = "failed"
		}
		finish(result)
	}
}
