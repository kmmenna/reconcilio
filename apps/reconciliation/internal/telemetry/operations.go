package telemetry

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

type operationMetrics struct {
	count    metric.Int64Counter
	duration metric.Float64Histogram
}

func newOperationMetrics(meter metric.Meter, prefix, countName string) (operationMetrics, error) {
	count, countErr := meter.Int64Counter(prefix + "." + countName)
	duration, durationErr := durationHistogram(meter, prefix+".duration")
	return operationMetrics{count, duration}, errors.Join(countErr, durationErr)
}

func durationHistogram(meter metric.Meter, name string) (metric.Float64Histogram, error) {
	return meter.Float64Histogram(name, metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60))
}

func (m operationMetrics) record(ctx context.Context, duration time.Duration, attrs ...attribute.KeyValue) {
	options := metric.WithAttributeSet(attribute.NewSet(attrs...))
	m.count.Add(ctx, 1, options)
	m.duration.Record(ctx, duration.Seconds(), options)
}

func (s *Runtime) start(ctx context.Context, name string, options ...trace.SpanStartOption) (context.Context, trace.Span) {
	options = append(options, trace.WithAttributes(identifiersFrom(ctx).attributes()...))
	if id := importFrom(ctx); id != "" {
		options = append(options, trace.WithAttributes(attribute.String("import_id", id)))
	}
	if work, ok := ctx.Value(workKey{}).(Work); ok {
		options = append(options, trace.WithAttributes(attribute.Int64("job_id", work.JobID),
			attribute.String("financial_account_id", work.AccountID), attribute.Int("attempt", work.Attempt)))
	}
	return s.traces.Tracer(serviceName).Start(ctx, name, options...)
}

type ImportResult struct {
	ID               string
	Outcome          string
	TransactionCount *int64
	JobIDs           []int64
}

func StartImport(ctx context.Context, operation string) (context.Context, func(ImportResult)) {
	s := from(ctx)
	if s == nil {
		return ctx, func(ImportResult) {}
	}
	started := time.Now()
	parentSpan := trace.SpanFromContext(ctx)
	ctx, span := s.start(ctx, "import."+operation)
	return ctx, func(result ImportResult) {
		defer span.End()
		duration := time.Since(started)
		outcome := attribute.String("outcome", result.Outcome)
		s.importOperations.record(ctx, duration, attribute.String("operation", operation), outcome)
		span.SetAttributes(outcome)
		if result.ID != "" {
			ctx = WithImport(ctx, result.ID)
			span.SetAttributes(attribute.String("import_id", result.ID))
			parentSpan.SetAttributes(attribute.String("import_id", result.ID))
		}
		level := slog.LevelInfo
		if result.Outcome == "failed" {
			span.SetStatus(codes.Error, "import operation failed")
			level = slog.LevelError
		}
		logAttrs := []slog.Attr{slog.String("operation", operation), slog.String("outcome", result.Outcome),
			slog.Float64("duration_seconds", duration.Seconds())}
		if len(result.JobIDs) > 0 {
			span.SetAttributes(attribute.Int64Slice("job_ids", result.JobIDs))
			logAttrs = append(logAttrs, slog.Any("job_ids", result.JobIDs))
		}
		if result.TransactionCount != nil {
			span.SetAttributes(attribute.Int64("transaction_count", *result.TransactionCount))
			logAttrs = append(logAttrs, slog.Int64("transaction_count", *result.TransactionCount))
		}
		s.Logger.LogAttrs(ctx, level, "Import operation", logAttrs...)
	}
}

type Work struct {
	JobID     int64
	AccountID string
	Attempt   int
}

func StartJob(ctx context.Context, work Work, metadata []byte) (context.Context, func(string)) {
	ctx = context.WithValue(jobContext(ctx, metadata), workKey{}, work)
	s := from(ctx)
	if s == nil {
		return ctx, func(string) {}
	}
	started := time.Now()
	ctx, span := s.start(ctx, "reconciliation.job", trace.WithSpanKind(trace.SpanKindConsumer))
	return ctx, func(outcome string) {
		defer span.End()
		duration := time.Since(started)
		outcomeAttribute := attribute.String("outcome", outcome)
		s.jobAttempts.record(ctx, duration, outcomeAttribute)
		span.SetAttributes(outcomeAttribute)
		level := slog.LevelInfo
		switch outcome {
		case "failed", "retrying", "panicked":
			span.SetStatus(codes.Error, outcome)
			level = slog.LevelError
		case "cancelled":
			level = slog.LevelWarn
		}
		s.Logger.LogAttrs(ctx, level, "Reconciliation job",
			slog.String("outcome", outcome), slog.Float64("duration_seconds", duration.Seconds()))
	}
}

type RankingObservation struct {
	Provider       string
	Model          string
	Attempted      bool
	Duration       time.Duration
	StartedAt      time.Time
	FallbackReason string
	InputTokens    *int64
	OutputTokens   *int64
}

func (s *Runtime) ObserveRanking(ctx context.Context, event RankingObservation) {
	outcome := "ranked"
	if event.FallbackReason != "" {
		outcome = "fallback"
	}
	provider, model := attribute.String("provider", event.Provider), attribute.String("model", event.Model)
	dimensions := metric.WithAttributeSet(attribute.NewSet(provider, model, attribute.String("outcome", outcome)))
	if event.Attempted {
		s.rankingRequests.Add(ctx, 1, dimensions)
		s.rankingDuration.Record(ctx, event.Duration.Seconds(), dimensions)
	}
	if event.FallbackReason != "" {
		s.rankingFallbacks.Add(ctx, 1, metric.WithAttributes(provider, model, attribute.String("reason", event.FallbackReason)))
	}

	spanOpts := []trace.SpanStartOption{trace.WithAttributes(provider, model, attribute.String("outcome", outcome),
		attribute.Bool("attempted", event.Attempted))}
	if event.Attempted {
		spanOpts = append(spanOpts, trace.WithTimestamp(event.StartedAt), trace.WithSpanKind(trace.SpanKindClient))
	}
	ctx, span := s.start(ctx, "ranking.evaluate", spanOpts...)
	if event.Attempted {
		defer span.End(trace.WithTimestamp(event.StartedAt.Add(event.Duration)))
	} else {
		defer span.End()
	}
	logAttrs := []slog.Attr{slog.String("provider", event.Provider), slog.String("model", event.Model),
		slog.String("outcome", outcome), slog.Bool("attempted", event.Attempted),
		slog.Float64("duration_seconds", event.Duration.Seconds())}
	if event.FallbackReason != "" {
		span.SetAttributes(attribute.String("fallback_reason", event.FallbackReason))
		if event.Attempted {
			span.SetStatus(codes.Error, event.FallbackReason)
		}
		logAttrs = append(logAttrs, slog.String("fallback_reason", event.FallbackReason))
	}
	if event.InputTokens != nil {
		s.rankingTokens.Add(ctx, *event.InputTokens, metric.WithAttributes(provider, model, attribute.String("direction", "input")))
		span.SetAttributes(attribute.Int64("input_tokens", *event.InputTokens))
		logAttrs = append(logAttrs, slog.Int64("input_tokens", *event.InputTokens))
	}
	if event.OutputTokens != nil {
		s.rankingTokens.Add(ctx, *event.OutputTokens, metric.WithAttributes(provider, model, attribute.String("direction", "output")))
		span.SetAttributes(attribute.Int64("output_tokens", *event.OutputTokens))
		logAttrs = append(logAttrs, slog.Int64("output_tokens", *event.OutputTokens))
	}
	s.Logger.LogAttrs(ctx, slog.LevelInfo, "AI ranking", logAttrs...)
}
