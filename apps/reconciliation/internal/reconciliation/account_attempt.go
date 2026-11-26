package reconciliation

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/catalog"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/reconciliation/reconciliationdb"
)

const (
	accountLeaseDuration         = 30 * time.Second
	accountLeaseHeartbeat        = 5 * time.Second
	accountLeaseOperationTimeout = 2 * time.Second
	accountBusySnooze            = 2 * time.Second
)

var errAccountLeaseLost = errors.New("reconciliation attempt lost its Financial Account lease")

// AccountAttempt grants one worker attempt permission to write results for an account.
type AccountAttempt struct {
	pool      *pgxpool.Pool
	accountID pgtype.UUID
	jobID     int64
	token     pgtype.UUID
}

func (a *AccountAttempt) write(ctx context.Context, effects func(pgx.Tx) error) error {
	return pgx.BeginFunc(ctx, a.pool, func(tx pgx.Tx) error {
		if err := catalog.LockFinancialAccounts(ctx, tx, []pgtype.UUID{a.accountID}); err != nil {
			return err
		}
		queries := reconciliationdb.New(tx)
		if err := a.verify(ctx, queries); err != nil {
			return err
		}
		if err := effects(tx); err != nil {
			return err
		}
		return a.verify(ctx, queries)
	})
}

func (a *AccountAttempt) verify(ctx context.Context, queries *reconciliationdb.Queries) error {
	_, err := queries.VerifyAccountLease(ctx, reconciliationdb.VerifyAccountLeaseParams{
		FinancialAccountID: a.accountID, AttemptToken: a.token,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return errAccountLeaseLost
	}
	if err != nil {
		return fmt.Errorf("verify reconciliation account lease: %w", err)
	}
	return nil
}

func (a *AccountAttempt) renew(ctx context.Context, cancel context.CancelCauseFunc, queries *reconciliationdb.Queries) {
	ticks := time.Tick(accountLeaseHeartbeat)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticks:
			// If a result transaction holds the row, it verifies expiration itself.
			renewCtx, finish := context.WithTimeout(ctx, accountLeaseOperationTimeout)
			owned, err := queries.RenewAccountLease(renewCtx, reconciliationdb.RenewAccountLeaseParams{
				FinancialAccountID: a.accountID, AttemptToken: a.token, LeaseDuration: leaseDuration(),
			})
			finish()
			if err != nil {
				cancel(fmt.Errorf("renew reconciliation account lease: %w", err))
				return
			}
			if !owned.Valid || !owned.Bool {
				cancel(errAccountLeaseLost)
				return
			}
		}
	}
}

func (a *AccountAttempt) release(ctx context.Context, queries *reconciliationdb.Queries) {
	releaseCtx, finish := context.WithTimeout(context.WithoutCancel(ctx), accountLeaseOperationTimeout)
	defer finish()
	if err := queries.ReleaseAccountLease(releaseCtx, reconciliationdb.ReleaseAccountLeaseParams{
		FinancialAccountID: a.accountID, AttemptToken: a.token,
	}); err != nil {
		slog.WarnContext(ctx, "Could not release reconciliation account lease", "job_id", a.jobID, "error", err)
	}
}

func leaseDuration() pgtype.Interval {
	return pgtype.Interval{Microseconds: accountLeaseDuration.Microseconds(), Valid: true}
}
