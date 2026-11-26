package telemetry

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/reconciliation/reconciliationdb"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// ObserveBusinessMetrics reads all counts from one database snapshot per collection.
func (s *Runtime) ObserveBusinessMetrics(db reconciliationdb.DBTX) (func() error, error) {
	meter := s.metrics.Meter(serviceName)
	imported, importedErr := meter.Int64ObservableGauge("reconcilio.business.imported_transactions")
	confirmed, confirmedErr := meter.Int64ObservableGauge("reconcilio.business.confirmed_reconciliations")
	rejected, rejectedErr := meter.Int64ObservableGauge("reconcilio.business.rejected_proposals")
	open, openErr := meter.Int64ObservableGauge("reconcilio.business.open_transactions")
	if err := errors.Join(importedErr, confirmedErr, rejectedErr, openErr); err != nil {
		return nil, fmt.Errorf("configure business metrics: %w", err)
	}
	queries := reconciliationdb.New(db)
	registration, err := meter.RegisterCallback(func(ctx context.Context, observer metric.Observer) error {
		query, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		counts, err := queries.ObserveBusinessCounts(query)
		if err != nil {
			s.Logger.WarnContext(ctx, "Could not collect business metrics", "error", err)
			// Skip this snapshot if the database is unavailable.
			return nil
		}
		observer.ObserveInt64(imported, counts.Imported)
		observer.ObserveInt64(confirmed, counts.Automatic, metric.WithAttributes(attribute.String("method", "automatic")))
		observer.ObserveInt64(confirmed, counts.Manual, metric.WithAttributes(attribute.String("method", "manual")))
		observer.ObserveInt64(rejected, counts.Rejected)
		observer.ObserveInt64(open, counts.OpenInternal, metric.WithAttributes(attribute.String("side", "internal")))
		observer.ObserveInt64(open, counts.OpenExternal, metric.WithAttributes(attribute.String("side", "external")))
		return nil
	}, imported, confirmed, rejected, open)
	if err != nil {
		return nil, fmt.Errorf("observe business metrics: %w", err)
	}
	return registration.Unregister, nil
}

func (s *Runtime) FlushMetrics(ctx context.Context) error {
	return s.metrics.ForceFlush(ctx)
}
