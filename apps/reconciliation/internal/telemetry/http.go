package telemetry

import (
	"cmp"
	"log/slog"
	"net/http"
	"strings"

	"github.com/felixge/httpsnoop"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/propagation"
)

func (s *Runtime) HTTP(mux *http.ServeMux) http.Handler {
	handler := otelhttp.NewHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		SetSpanAttributes(ctx, identifiersFrom(ctx).attributes()...)
		result := httpsnoop.CaptureMetrics(mux, w, r)
		_, pattern, _ := strings.Cut(r.Pattern, " ")
		level := slog.LevelInfo
		switch {
		case result.Code >= http.StatusInternalServerError:
			level = slog.LevelError
		case result.Code >= http.StatusBadRequest:
			level = slog.LevelWarn
		}
		s.Logger.LogAttrs(ctx, level, "HTTP request",
			slog.String("method", r.Method),
			slog.String("route", cmp.Or(pattern, "unmatched")),
			slog.Int("status", result.Code),
			slog.Float64("duration_seconds", result.Duration.Seconds()))
	}), "http.server", otelhttp.WithTracerProvider(s.traces), otelhttp.WithMeterProvider(s.metrics),
		otelhttp.WithPropagators(propagation.TraceContext{}))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, ids := requestContext(r)
		w.Header().Set(requestHeader, ids.RequestID)
		w.Header().Set(correlationHeader, ids.CorrelationID)
		handler.ServeHTTP(w, r.WithContext(s.Context(ctx)))
	})
}
