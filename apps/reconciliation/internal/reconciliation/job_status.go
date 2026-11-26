package reconciliation

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"log/slog"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/reconciliation/reconciliationdb"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/rivertype"
)

const jobSummaryRecoveryInterval = 10 * time.Second

var ErrJobNotFound = errors.New("reconciliation job not found")

type JobStatus struct {
	ID                 int64      `json:"id"`
	FinancialAccountID string     `json:"financial_account_id" format:"uuid"`
	Status             string     `json:"status" enum:"queued,deferred,running,retrying,completed,failed,cancelled"`
	Attempt            int        `json:"attempt"`
	MaxAttempts        int        `json:"max_attempts"`
	CompletedAt        *time.Time `json:"completed_at,omitempty"`
	AutomaticCount     *int64     `json:"automatic_count,omitempty"`
	ProposalCount      *int64     `json:"proposal_count,omitempty"`
}

func (q *Queue) Status(ctx context.Context, id int64) (JobStatus, error) {
	saved, err := q.queries.GetJobSummary(ctx, id)
	if err == nil {
		return presentJobSummary(saved), nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return JobStatus{}, err
	}

	row, err := q.client.JobGet(ctx, id)
	if errors.Is(err, rivertype.ErrNotFound) {
		// Recovery may have inserted the summary before River removed the row.
		saved, err := q.queries.GetJobSummary(ctx, id)
		if errors.Is(err, pgx.ErrNoRows) {
			return JobStatus{}, ErrJobNotFound
		}
		if err != nil {
			return JobStatus{}, err
		}
		return presentJobSummary(saved), nil
	}
	if err != nil {
		return JobStatus{}, err
	}
	if row.Kind != reconcileKind {
		return JobStatus{}, ErrJobNotFound
	}

	accountID, err := jobAccountID(row)
	if err != nil {
		return JobStatus{}, err
	}
	status := presentRiverState(row)
	if row.FinalizedAt != nil {
		if err := q.saveJobSummary(ctx, row, accountID, status); err != nil {
			return JobStatus{}, err
		}
		saved, err := q.queries.GetJobSummary(ctx, id)
		if err != nil {
			return JobStatus{}, err
		}
		return presentJobSummary(saved), nil
	}
	return JobStatus{
		ID: row.ID, FinancialAccountID: accountID.String(), Status: status,
		Attempt: row.Attempt, MaxAttempts: row.MaxAttempts,
	}, nil
}

func jobAccountID(row *rivertype.JobRow) (pgtype.UUID, error) {
	var args ReconcileArgs
	if err := json.Unmarshal(row.EncodedArgs, &args); err != nil {
		return pgtype.UUID{}, fmt.Errorf("decode reconciliation job arguments: %w", err)
	}
	id, err := uuid.Parse(args.FinancialAccountID)
	if err != nil {
		return pgtype.UUID{}, fmt.Errorf("decode reconciliation job account: %w", err)
	}
	return pgtype.UUID{Bytes: id, Valid: true}, nil
}

func presentRiverState(row *rivertype.JobRow) string {
	switch row.State {
	case rivertype.JobStateAvailable, rivertype.JobStateScheduled:
		// River uses available for both short retries and short snoozes.
		var metadata struct {
			Deferred bool `json:"reconcilio_account_deferred"`
		}
		if err := json.Unmarshal(row.Metadata, &metadata); err == nil && metadata.Deferred {
			return "deferred"
		}
		if row.Attempt > 0 && len(row.Errors) > 0 {
			return "retrying"
		}
	case rivertype.JobStateRunning:
		return "running"
	case rivertype.JobStateRetryable:
		return "retrying"
	case rivertype.JobStateCompleted:
		return "completed"
	case rivertype.JobStateDiscarded:
		return "failed"
	case rivertype.JobStateCancelled:
		return "cancelled"
	}
	return "queued"
}

func presentJobSummary(row reconciliationdb.ReconciliationJobSummary) JobStatus {
	return JobStatus{
		ID: row.JobID, FinancialAccountID: row.FinancialAccountID.String(), Status: row.Status,
		Attempt: int(row.Attempt), MaxAttempts: int(row.MaxAttempts), CompletedAt: new(row.CompletedAt.Time),
		AutomaticCount: new(row.AutomaticCount), ProposalCount: new(row.ProposalCount),
	}
}

func (q *Queue) saveJobSummary(ctx context.Context, row *rivertype.JobRow, accountID pgtype.UUID, status string) error {
	if err := q.queries.SaveJobSummary(ctx, reconciliationdb.SaveJobSummaryParams{
		JobID: row.ID, FinancialAccountID: accountID, Status: status,
		CompletedAt: pgtype.Timestamptz{Time: *row.FinalizedAt, Valid: true},
		Attempt:     int32(row.Attempt), MaxAttempts: int32(row.MaxAttempts),
	}); err != nil {
		return fmt.Errorf("save reconciliation job %d summary: %w", row.ID, err)
	}
	return nil
}

func (q *Queue) recoverSummaries(ctx context.Context) error {
	const batchSize = 100
	params := river.NewJobListParams().
		Kinds(reconcileKind).
		States(rivertype.JobStateCompleted, rivertype.JobStateCancelled, rivertype.JobStateDiscarded).
		Where("NOT EXISTS (SELECT 1 FROM reconciliation_job_summaries WHERE job_id = river_job.id)").
		OrderBy(river.JobListOrderByID, river.SortOrderAsc).
		First(batchSize)
	for {
		result, err := q.client.JobList(ctx, params)
		if err != nil {
			return fmt.Errorf("list finalized reconciliation jobs: %w", err)
		}
		for _, row := range result.Jobs {
			id, err := jobAccountID(row)
			if err != nil {
				slog.ErrorContext(ctx, "Could not read terminal reconciliation job", "job_id", row.ID, "error", err)
				continue
			}
			if err := q.saveJobSummary(ctx, row, id, presentRiverState(row)); err != nil {
				return err
			}
		}
		if len(result.Jobs) < batchSize {
			return nil
		}
		params = params.After(result.LastCursor)
	}
}

func (q *Queue) recoverSummariesPeriodically(ctx context.Context) {
	defer close(q.stopped)
	defer func() { <-q.client.Stopped() }()
	ticks := time.Tick(jobSummaryRecoveryInterval)
	for {
		select {
		case <-ctx.Done():
			return
		case <-q.client.Stopped():
			return
		case <-ticks:
			if err := q.recoverSummaries(ctx); err != nil && ctx.Err() == nil {
				slog.ErrorContext(ctx, "Could not recover reconciliation job summaries", "error", err)
			}
		}
	}
}
