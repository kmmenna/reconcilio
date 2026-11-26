package telemetry

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"net/http"
	"uuid"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

const requestHeader = "X-Request-ID"
const correlationHeader = "X-Correlation-ID"

type runtimeKey struct{}
type idsKey struct{}
type importKey struct{}
type workKey struct{}

type identifiers struct {
	RequestID     string `json:"initiating_request_id,omitempty"`
	CorrelationID string `json:"correlation_id,omitempty"`
}

func (ids identifiers) attributes() []attribute.KeyValue {
	if ids.RequestID == "" {
		return nil
	}
	return []attribute.KeyValue{
		attribute.String("request_id", ids.RequestID),
		attribute.String("correlation_id", ids.CorrelationID),
	}
}

func identifiersFrom(ctx context.Context) identifiers {
	ids, _ := ctx.Value(idsKey{}).(identifiers)
	return ids
}

func validOrNewID(value string) string {
	if id, err := uuid.Parse(value); err == nil && id != (uuid.UUID{}) {
		return id.String()
	}
	return uuid.New().String()
}

func (s *Runtime) Context(ctx context.Context) context.Context {
	return context.WithValue(ctx, runtimeKey{}, s)
}

func from(ctx context.Context) *Runtime {
	s, _ := ctx.Value(runtimeKey{}).(*Runtime)
	return s
}

// WithImport carries the originating import into queue metadata.
func WithImport(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, importKey{}, id)
}

func importFrom(ctx context.Context) string {
	id, _ := ctx.Value(importKey{}).(string)
	return id
}

type initiatingMetadata struct {
	identifiers
	ImportID string                 `json:"import_id,omitempty"`
	Trace    propagation.MapCarrier `json:"trace_context,omitempty"`
}

// JobMetadata uses River's application metadata so records remain compatible with older jobs.
func JobMetadata(ctx context.Context) ([]byte, error) {
	traceFields := make(propagation.MapCarrier)
	propagation.TraceContext{}.Inject(ctx, traceFields)
	data, err := json.Marshal(initiatingMetadata{identifiersFrom(ctx), importFrom(ctx), traceFields})
	if err != nil {
		return nil, fmt.Errorf("encode job telemetry metadata: %w", err)
	}
	return data, nil
}

func jobContext(ctx context.Context, metadata []byte) context.Context {
	var initiation initiatingMetadata
	if err := json.Unmarshal(metadata, &initiation); err != nil {
		return ctx
	}
	ctx = context.WithValue(ctx, idsKey{}, initiation.identifiers)
	ctx = WithImport(ctx, initiation.ImportID)
	return propagation.TraceContext{}.Extract(ctx, initiation.Trace)
}

func requestContext(r *http.Request) (context.Context, identifiers) {
	ids := identifiers{
		RequestID:     validOrNewID(r.Header.Get(requestHeader)),
		CorrelationID: validOrNewID(r.Header.Get(correlationHeader)),
	}
	return context.WithValue(r.Context(), idsKey{}, ids), ids
}

func SetSpanAttributes(ctx context.Context, attrs ...attribute.KeyValue) {
	trace.SpanFromContext(ctx).SetAttributes(attrs...)
}
