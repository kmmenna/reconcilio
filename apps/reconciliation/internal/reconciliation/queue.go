// Package reconciliation evaluates Transactions and records authoritative groups.
package reconciliation

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/reconciliation/reconciliationdb"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivertype"
)

const reconcileKind = "reconcile_account"

type ReconcileArgs struct {
	FinancialAccountID string `json:"financial_account_id"`
}

func (ReconcileArgs) Kind() string { return reconcileKind }

func (ReconcileArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: 3}
}

type AccountProcessor interface {
	ProcessAccount(context.Context, pgtype.UUID, int64) error
}

type accountWorker struct {
	river.WorkerDefaults[ReconcileArgs]
	processor AccountProcessor
}

func (w *accountWorker) Work(ctx context.Context, job *river.Job[ReconcileArgs]) error {
	id, err := uuid.Parse(job.Args.FinancialAccountID)
	if err != nil {
		return river.JobCancel(err)
	}
	return w.processor.ProcessAccount(ctx, pgtype.UUID{Bytes: id, Valid: true}, job.ID)
}

type Queue struct {
	client  *river.Client[pgx.Tx]
	queries *reconciliationdb.Queries
}

func NewQueue(pool *pgxpool.Pool, processor AccountProcessor, workerEnabled bool) (*Queue, error) {
	config := &river.Config{}
	if workerEnabled {
		config.Workers = river.NewWorkers()
		if err := river.AddWorkerSafely(config.Workers, &accountWorker{processor: processor}); err != nil {
			return nil, err
		}
		config.Queues = map[string]river.QueueConfig{river.QueueDefault: {MaxWorkers: 4}}
		config.SoftStopTimeout = 10 * time.Second
		config.CompletedJobRetentionPeriod = 30 * 24 * time.Hour
		config.CancelledJobRetentionPeriod = 30 * 24 * time.Hour
		config.DiscardedJobRetentionPeriod = 30 * 24 * time.Hour
	}
	client, err := river.NewClient(riverpgxv5.New(pool), config)
	if err != nil {
		return nil, fmt.Errorf("configure River queue: %w", err)
	}
	return &Queue{client: client, queries: reconciliationdb.New(pool)}, nil
}

func (q *Queue) Start(ctx context.Context) error {
	return q.client.Start(ctx)
}

func (q *Queue) Stopped() <-chan struct{} {
	return q.client.Stopped()
}

func (q *Queue) EnqueueTx(ctx context.Context, tx pgx.Tx, accountID pgtype.UUID) (int64, error) {
	result, err := q.client.InsertTx(ctx, tx, ReconcileArgs{FinancialAccountID: accountID.String()}, nil)
	if err != nil {
		return 0, fmt.Errorf("enqueue reconciliation: %w", err)
	}
	return result.Job.ID, nil
}

var ErrJobNotFound = errors.New("reconciliation job not found")

type JobStatus struct {
	ID                 int64      `json:"id"`
	FinancialAccountID string     `json:"financial_account_id" format:"uuid"`
	Status             string     `json:"status" enum:"queued,running,retrying,completed,failed,cancelled"`
	Attempt            int        `json:"attempt"`
	MaxAttempts        int        `json:"max_attempts"`
	CompletedAt        *time.Time `json:"completed_at,omitempty"`
	AutomaticCount     *int64     `json:"automatic_count,omitempty"`
}

func (q *Queue) Status(ctx context.Context, id int64) (JobStatus, error) {
	row, err := q.client.JobGet(ctx, id)
	if errors.Is(err, rivertype.ErrNotFound) {
		return JobStatus{}, ErrJobNotFound
	}
	if err != nil {
		return JobStatus{}, err
	}
	if row.Kind != reconcileKind {
		return JobStatus{}, ErrJobNotFound
	}
	var args ReconcileArgs
	if err := json.Unmarshal(row.EncodedArgs, &args); err != nil {
		return JobStatus{}, fmt.Errorf("decode reconciliation job arguments: %w", err)
	}
	status := "queued"
	switch row.State {
	case rivertype.JobStateRunning:
		status = "running"
	case rivertype.JobStateRetryable:
		status = "retrying"
	case rivertype.JobStateCompleted:
		status = "completed"
	case rivertype.JobStateDiscarded:
		status = "failed"
	case rivertype.JobStateCancelled:
		status = "cancelled"
	}
	job := JobStatus{
		ID: row.ID, FinancialAccountID: args.FinancialAccountID, Status: status, Attempt: row.Attempt,
		MaxAttempts: row.MaxAttempts, CompletedAt: row.FinalizedAt,
	}
	if row.FinalizedAt != nil {
		count, err := q.queries.CountAutomaticReconciliations(ctx, pgtype.Int8{Int64: row.ID, Valid: true})
		if err != nil {
			return JobStatus{}, fmt.Errorf("count automatic reconciliations: %w", err)
		}
		job.AutomaticCount = &count
	}
	return job, nil
}
