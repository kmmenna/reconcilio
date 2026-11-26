// Package reconciliation evaluates Transactions and records authoritative groups.
package reconciliation

import (
	"context"
	"errors"
	"fmt"
	"time"
	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/catalog"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/reconciliation/reconciliationdb"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
)

const reconcileKind = "reconcile_account"
const accountDeferredMetadata = "reconcilio_account_deferred"

type ReconcileArgs struct {
	FinancialAccountID string `json:"financial_account_id"`
}

func (ReconcileArgs) Kind() string { return reconcileKind }

func (ReconcileArgs) InsertOpts() river.InsertOpts {
	return river.InsertOpts{MaxAttempts: 3}
}

type AccountProcessor interface {
	ProcessAccount(context.Context, *AccountAttempt) error
}

type accountWorker struct {
	river.WorkerDefaults[ReconcileArgs]
	processor AccountProcessor
	pool      *pgxpool.Pool
	queries   *reconciliationdb.Queries
}

func (w *accountWorker) Work(ctx context.Context, job *river.Job[ReconcileArgs]) error {
	if err := river.MetadataSet(ctx, accountDeferredMetadata, false); err != nil {
		return err
	}
	id, err := uuid.Parse(job.Args.FinancialAccountID)
	if err != nil {
		return river.JobCancel(err)
	}
	attempt := &AccountAttempt{
		pool: w.pool, accountID: pgtype.UUID{Bytes: id, Valid: true}, jobID: job.ID,
		token: pgtype.UUID{Bytes: uuid.New(), Valid: true},
	}
	acquireCtx, finish := context.WithTimeout(ctx, accountLeaseOperationTimeout)
	rows, err := w.queries.AcquireAccountLease(acquireCtx, reconciliationdb.AcquireAccountLeaseParams{
		FinancialAccountID: attempt.accountID, AttemptToken: attempt.token, LeaseDuration: leaseDuration(),
	})
	finish()
	if err != nil {
		return fmt.Errorf("acquire reconciliation account lease: %w", err)
	}
	if rows == 0 {
		if err := river.MetadataSet(ctx, accountDeferredMetadata, true); err != nil {
			return err
		}
		return river.JobSnooze(accountBusySnooze)
	}

	workCtx, cancel := context.WithCancelCause(ctx)
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		attempt.renew(workCtx, cancel, w.queries)
	}()
	defer func() {
		cancel(nil)
		<-stopped
		attempt.release(ctx, w.queries)
	}()
	err = w.processor.ProcessAccount(workCtx, attempt)
	return errors.Join(err, context.Cause(workCtx))
}

type Queue struct {
	stopped chan struct{}
	pool    *pgxpool.Pool
	client  *river.Client[pgx.Tx]
	queries *reconciliationdb.Queries
}

func NewQueue(pool *pgxpool.Pool, processor AccountProcessor, workerEnabled bool) (*Queue, error) {
	queries := reconciliationdb.New(pool)
	config := &river.Config{}
	if workerEnabled {
		config.Workers = river.NewWorkers()
		if err := river.AddWorkerSafely(config.Workers, &accountWorker{processor: processor, pool: pool, queries: queries}); err != nil {
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
	return &Queue{pool: pool, client: client, queries: queries}, nil
}

func (q *Queue) Start(ctx context.Context) error {
	if err := q.recoverSummaries(ctx); err != nil {
		return fmt.Errorf("recover reconciliation job summaries: %w", err)
	}
	if err := q.client.Start(ctx); err != nil {
		return err
	}
	q.stopped = make(chan struct{})
	go q.recoverSummariesPeriodically(ctx)
	return nil
}

func (q *Queue) Stopped() <-chan struct{} {
	return q.stopped
}

func (q *Queue) EnqueueTx(ctx context.Context, tx pgx.Tx, accountID pgtype.UUID) (int64, error) {
	result, err := q.client.InsertTx(ctx, tx, ReconcileArgs{FinancialAccountID: accountID.String()}, nil)
	if err != nil {
		return 0, fmt.Errorf("enqueue reconciliation: %w", err)
	}
	return result.Job.ID, nil
}

// Reprocess durably queues evaluation of a Financial Account's open Transactions.
func (q *Queue) Reprocess(ctx context.Context, accountID pgtype.UUID) (int64, error) {
	var id int64
	err := pgx.BeginFunc(ctx, q.pool, func(tx pgx.Tx) error {
		if _, err := q.queries.WithTx(tx).CheckReprocessingAccount(ctx, accountID); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return catalog.ErrFinancialAccountNotFound
			}
			return err
		}
		var err error
		id, err = q.EnqueueTx(ctx, tx, accountID)
		return err
	})
	return id, err
}
