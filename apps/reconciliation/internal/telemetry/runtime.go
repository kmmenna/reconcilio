// Package telemetry emits structured operational signals and observes persisted business results.
package telemetry

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutlog"
	"go.opentelemetry.io/otel/exporters/stdout/stdoutmetric"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/metric"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.40.0"
)

const serviceName = "reconcilio"

type Options struct {
	Traces  bool
	Metrics bool
	Writer  io.Writer
}

type Runtime struct {
	Logger  *slog.Logger
	logs    *sdklog.LoggerProvider
	traces  *sdktrace.TracerProvider
	metrics *sdkmetric.MeterProvider

	importOperations operationMetrics
	jobAttempts      operationMetrics
	rankingRequests  metric.Int64Counter
	rankingDuration  metric.Float64Histogram
	rankingFallbacks metric.Int64Counter
	rankingTokens    metric.Int64Counter
}

func New(options Options) (*Runtime, error) {
	writer := options.Writer
	if writer == nil {
		writer = os.Stdout
	}
	console := &consoleWriter{writer: writer}
	res := resource.NewSchemaless(semconv.ServiceName(serviceName))
	logExporter, err := stdoutlog.New(stdoutlog.WithWriter(console))
	if err != nil {
		return nil, fmt.Errorf("configure JSON log exporter: %w", err)
	}

	traceOptions := []sdktrace.TracerProviderOption{sdktrace.WithResource(res)}
	if options.Traces {
		exporter, err := stdouttrace.New(stdouttrace.WithWriter(console))
		if err != nil {
			return nil, fmt.Errorf("configure stdout trace exporter: %w", err)
		}
		traceOptions = append(traceOptions, sdktrace.WithBatcher(exporter))
	}

	metricOptions := []sdkmetric.Option{sdkmetric.WithResource(res)}
	if options.Metrics {
		exporter, err := stdoutmetric.New(stdoutmetric.WithWriter(console))
		if err != nil {
			return nil, fmt.Errorf("configure stdout metric exporter: %w", err)
		}
		metricOptions = append(metricOptions, sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter, sdkmetric.WithInterval(10*time.Second))))
	}

	s := &Runtime{
		logs:    sdklog.NewLoggerProvider(sdklog.WithResource(res), sdklog.WithProcessor(sdklog.NewBatchProcessor(logExporter))),
		traces:  sdktrace.NewTracerProvider(traceOptions...),
		metrics: sdkmetric.NewMeterProvider(metricOptions...),
	}
	handler := otelslog.NewHandler(serviceName, otelslog.WithLoggerProvider(s.logs))
	s.Logger = slog.New(contextHandler{handler})
	if err := s.createInstruments(); err != nil {
		return nil, errors.Join(err, s.Shutdown(context.Background()))
	}
	return s, nil
}

func (s *Runtime) createInstruments() error {
	meter := s.metrics.Meter(serviceName)
	var errs []error
	var err error
	s.importOperations, err = newOperationMetrics(meter, "reconcilio.import", "operations")
	errs = append(errs, err)
	s.jobAttempts, err = newOperationMetrics(meter, "reconcilio.job", "attempts")
	errs = append(errs, err)
	s.rankingRequests, err = meter.Int64Counter("reconcilio.ai.requests")
	errs = append(errs, err)
	s.rankingDuration, err = durationHistogram(meter, "reconcilio.ai.duration")
	errs = append(errs, err)
	s.rankingFallbacks, err = meter.Int64Counter("reconcilio.ai.fallbacks")
	errs = append(errs, err)
	s.rankingTokens, err = meter.Int64Counter("reconcilio.ai.tokens", metric.WithUnit("{token}"))
	return errors.Join(append(errs, err)...)
}

func (s *Runtime) Shutdown(ctx context.Context) error {
	return errors.Join(s.metrics.Shutdown(ctx), s.traces.Shutdown(ctx), s.logs.Shutdown(ctx))
}

type consoleWriter struct {
	sync.Mutex
	writer io.Writer
}

func (w *consoleWriter) Write(p []byte) (int, error) {
	w.Lock()
	defer w.Unlock()
	return w.writer.Write(p)
}

type contextHandler struct{ slog.Handler }

func (h contextHandler) Handle(ctx context.Context, record slog.Record) error {
	// Raw database and transport errors can contain values from inputs or connection settings.
	safe := slog.NewRecord(record.Time, record.Level, record.Message, record.PC)
	record.Attrs(func(a slog.Attr) bool {
		switch {
		case a.Key == "panic_val" || a.Key == "stack_trace":
			return true
		case a.Key == "error" || a.Key == "err":
			// River can stringify errors before sending them to slog.
			err, _ := a.Value.Any().(error)
			if pgErr, ok := errors.AsType[*pgconn.PgError](err); ok {
				a = slog.String("error_type", "PostgreSQL "+pgErr.Code)
			} else if err != nil {
				a = slog.String("error_type", fmt.Sprintf("%T", err))
			} else {
				return true
			}
		}
		safe.AddAttrs(a)
		return true
	})
	if ids := identifiersFrom(ctx); ids.RequestID != "" {
		safe.AddAttrs(slog.String("request_id", ids.RequestID), slog.String("correlation_id", ids.CorrelationID))
	}
	if work, ok := ctx.Value(workKey{}).(Work); ok {
		safe.AddAttrs(slog.Int64("job_id", work.JobID), slog.String("financial_account_id", work.AccountID), slog.Int("attempt", work.Attempt))
	}
	if importID := importFrom(ctx); importID != "" {
		safe.AddAttrs(slog.String("import_id", importID))
	}
	return h.Handler.Handle(ctx, safe)
}

func (h contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return contextHandler{h.Handler.WithAttrs(attrs)}
}

func (h contextHandler) WithGroup(name string) slog.Handler {
	return contextHandler{h.Handler.WithGroup(name)}
}
