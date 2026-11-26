package main

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"uuid"

	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/reconciliation"
	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/telemetry"
)

func TestHTTPConsoleCorrelation(t *testing.T) {
	for _, config := range []struct {
		name            string
		traces, metrics bool
	}{
		{"logs only", false, false},
		{"traces only", true, false},
		{"metrics only", false, true},
	} {
		t.Run(config.name, func(t *testing.T) {
			var output bytes.Buffer
			signals, err := telemetry.New(telemetry.Options{Traces: config.traces, Metrics: config.metrics, Writer: &output})
			if err != nil {
				t.Fatal(err)
			}
			mux := http.NewServeMux()
			mux.HandleFunc("GET /test/{id}", func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusNoContent)
			})
			handler := signals.HTTP(mux)
			requestID, workflowID := uuid.New().String(), uuid.New().String()
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test/1234", nil)
			req.Header.Set("X-Request-ID", requestID)
			req.Header.Set("X-Correlation-ID", workflowID)
			res := httptest.NewRecorder()
			handler.ServeHTTP(res, req)
			next := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/test/1234", nil)
			next.Header.Set("X-Request-ID", "unsafe input")
			next.Header.Set("X-Correlation-ID", workflowID)
			nextRes := httptest.NewRecorder()
			handler.ServeHTTP(nextRes, next)
			if err := signals.Shutdown(t.Context()); err != nil {
				t.Fatal(err)
			}
			if res.Header().Get("X-Request-ID") != requestID || res.Header().Get("X-Correlation-ID") != workflowID ||
				nextRes.Header().Get("X-Request-ID") == requestID || nextRes.Header().Get("X-Correlation-ID") != workflowID {
				t.Fatalf("unexpected response headers: %v %v", res.Header(), nextRes.Header())
			}
			if _, err := uuid.Parse(nextRes.Header().Get("X-Request-ID")); err != nil {
				t.Fatal(err)
			}
			console := output.String()
			for _, value := range []string{"HTTP request", requestID, workflowID, "/test/{id}"} {
				if !strings.Contains(console, value) {
					t.Errorf("console missing %q: %s", value, console)
				}
			}
			if strings.Contains(console, "unsafe input") ||
				strings.Contains(console, "\"SpanKind\"") != config.traces ||
				strings.Contains(console, "http.server.request.duration") != config.metrics {
				t.Errorf("unexpected console signals: %s", console)
			}
		})
	}
}

func TestImportWorkerAndRankingConsoleTelemetry(t *testing.T) {
	pool, mux, token := jobTestAPI(t)
	var output consoleCapture
	signals, err := telemetry.New(telemetry.Options{Traces: true, Metrics: true, Writer: &output})
	if err != nil {
		t.Fatal(err)
	}
	shutdown := sync.OnceValue(func() error {
		return signals.Shutdown(context.WithoutCancel(t.Context()))
	})
	t.Cleanup(func() {
		if err := shutdown(); err != nil {
			t.Error(err)
		}
	})
	stopMetrics, err := signals.ObserveBusinessMetrics(pool)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := stopMetrics(); err != nil {
			t.Error(err)
		}
	})

	handler := signals.HTTP(mux)
	readResponse(t, jobRequest(t, handler, http.MethodPost, "/api/imports/"+uuid.New().String()+"/confirm", token, ""), http.StatusNotFound, new(any))
	account, _ := twoJobAccounts(t, handler, token)
	sources := rankingSources(t, handler, token)
	options := reconciliation.DefaultRankingOptions()
	options.Model = "configured-test-model"
	options.Observe = rankingObserver(signals)
	options.Ranker = rankingDouble(func(_ context.Context, input reconciliation.RankingInput) (reconciliation.RankingResult, error) {
		result := rankedInput(input)
		result.Ranking[0].Explanations.English = ""
		result.Model = "reported-test-model"
		result.Usage = &reconciliation.RankingUsage{InputTokens: 50, OutputTokens: 82}
		return result, nil
	})
	worker, err := reconciliation.NewQueue(pool, reconciliation.NewWithRanking(pool, options), true)
	if err != nil {
		t.Fatal(err)
	}
	ctx, stopWorker := context.WithCancel(signals.Context(t.Context()))
	if err := worker.Start(ctx); err != nil {
		stopWorker()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		stopWorker()
		<-worker.Stopped()
	})

	upload := func(side string) (id int64, confirm *http.Request, correlationID string) {
		t.Helper()
		data, err := os.ReadFile("../../../../examples/demo/" + side + ".csv")
		if err != nil {
			t.Fatal(err)
		}
		correlationID = uuid.New().String()
		uploadHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Header.Set("X-Request-ID", uuid.New().String())
			r.Header.Set("X-Correlation-ID", correlationID)
			if strings.HasSuffix(r.URL.Path, "/confirm") {
				confirm = r
			}
			handler.ServeHTTP(w, r)
		})
		return submitJobTestCSV(t, uploadHandler, token, account, sources[side], data), confirm, correlationID
	}

	first, _, _ := upload("internal")
	waitForJob(t, handler, token, first, "completed")
	jobID, confirm, workflow := upload("external")
	assertJobCounts(t, waitForJob(t, handler, token, jobID, "completed"), 1, 2)

	group := readRankingGroup(t, handler, token, rankingGroupPaths(t, handler, token)["ERP-002"])
	if group.RankingEvaluationID != "" || len(group.Proposals) != 2 {
		t.Fatalf("invalid bilingual assessment should fall back to the two engine proposals: %+v", group)
	}
	readResponse(t, jobRequest(t, handler, http.MethodPost, "/api/match-proposals/"+group.Proposals[1].ID+"/reject", token, ""),
		http.StatusOK, new(any))
	readResponse(t, jobRequest(t, handler, http.MethodPost, "/api/match-proposals/"+group.Proposals[0].ID+"/confirm", token, ""),
		http.StatusOK, new(any))
	if err := signals.FlushMetrics(t.Context()); err != nil {
		t.Fatal(err)
	}
	want := map[string]int64{
		"imported_transactions": 7, "confirmed_reconciliations:automatic": 1, "confirmed_reconciliations:manual": 1,
		"rejected_proposals": 1, "open_transactions:internal": 1, "open_transactions:external": 2,
	}
	if got := latestConsoleMetrics(t, output.String(), "reconcilio.business."); !reflect.DeepEqual(got, want) {
		t.Fatalf("business snapshot = %v, want %v", got, want)
	}

	var replay struct {
		Jobs []struct {
			ID int64 `json:"job_id"`
		} `json:"jobs"`
	}
	readResponse(t, jobRequest(t, handler, http.MethodPost, confirm.URL.Path, token, ""), http.StatusOK, &replay)
	if len(replay.Jobs) != 1 || replay.Jobs[0].ID != jobID {
		t.Fatalf("replayed confirmation should retain the original job: %+v", replay)
	}
	assertJobCounts(t, waitForJob(t, handler, token, enqueueReprocessing(t, handler, token, account), "completed"), 0, 0)
	if err := signals.FlushMetrics(t.Context()); err != nil {
		t.Fatal(err)
	}
	if got := latestConsoleMetrics(t, output.String(), "reconcilio.business."); !reflect.DeepEqual(got, want) {
		t.Fatalf("replayed business snapshot = %v, want %v", got, want)
	}
	if got := latestConsoleMetrics(t, output.String(), "reconcilio.import."); got["operations:confirm:confirmed"] != 2 ||
		got["operations:confirm:replayed"] != 1 || got["duration:confirm:rejected"] != 1 ||
		got["operations:preview:pending"] != 2 {
		t.Fatalf("import operations should record their outcomes and durations: %v", got)
	}
	if got := latestConsoleMetrics(t, output.String(), "reconcilio.job."); got["attempts:completed"] != 3 || got["duration:completed"] != 3 {
		t.Fatalf("job attempts should record their outcomes and durations: %v", got)
	}
	if got := latestConsoleMetrics(t, output.String(), "reconcilio.ai."); got["requests:reported-test-model:fallback:openai"] != 1 ||
		got["fallbacks:reported-test-model:openai:invalid_ranking"] != 1 ||
		got["tokens:input:reported-test-model:openai"] != 50 ||
		got["tokens:output:reported-test-model:openai"] != 82 ||
		got["duration:reported-test-model:fallback:openai"] != 1 {
		t.Fatalf("fallback should retain the attempt, duration and reported usage: %v", got)
	}

	stopWorker()
	<-worker.Stopped()
	if err := shutdown(); err != nil {
		t.Fatal(err)
	}
	console := output.String()
	importID := strings.Split(confirm.URL.Path, "/")[3]
	requestID := confirm.Header.Get("X-Request-ID")
	for _, observed := range []struct {
		message string
		values  []string
	}{
		{"HTTP request", []string{requestID, workflow, "/api/imports/{id}/confirm"}},
		{"Import operation", []string{requestID, workflow, importID, "confirmed"}},
		{"Reconciliation job", []string{requestID, workflow, importID, fmt.Sprint(jobID), "completed"}},
		{"AI ranking", []string{requestID, workflow, "invalid_ranking", "reported-test-model"}},
	} {
		var found bool
		for line := range strings.SplitSeq(console, "\n") {
			if !strings.Contains(line, observed.message) {
				continue
			}
			found = true
			for _, want := range observed.values {
				found = found && strings.Contains(line, want)
			}
			if found {
				break
			}
		}
		if !found {
			t.Errorf("missing %s with %q", observed.message, observed.values)
		}
	}
	for _, private := range []string{"Canva subscription for the design team", "test-job-analyst", token} {
		if strings.Contains(console, private) {
			t.Error("console exposed a sensitive input")
		}
	}
	traceIDs := make(map[string]string)
	for line := range strings.SplitSeq(console, "\n") {
		var span struct {
			Name        string
			SpanContext struct{ TraceID string }
		}
		if strings.Contains(line, requestID) && json.Unmarshal([]byte(line), &span) == nil &&
			span.Name != "" && span.SpanContext.TraceID != "" {
			traceIDs[span.Name] = span.SpanContext.TraceID
		}
	}
	if id := traceIDs["import.confirm"]; id == "" || id != traceIDs["reconciliation.job"] || id != traceIDs["ranking.evaluate"] {
		t.Fatalf("confirmation, River and AI spans should share one trace: %v", traceIDs)
	}
}

type consoleCapture struct {
	sync.Mutex
	data bytes.Buffer
}

func (c *consoleCapture) Write(data []byte) (int, error) {
	c.Lock()
	defer c.Unlock()
	return c.data.Write(data)
}

func (c *consoleCapture) String() string {
	c.Lock()
	defer c.Unlock()
	return c.data.String()
}

func latestConsoleMetrics(t *testing.T, console, prefix string) map[string]int64 {
	t.Helper()
	var points map[string]int64
	for line := range strings.SplitSeq(console, "\n") {
		if !strings.Contains(line, "\"ScopeMetrics\"") {
			continue
		}
		var data struct {
			ScopeMetrics []struct {
				Metrics []struct {
					Name string
					Data struct {
						DataPoints []struct {
							Attributes []struct {
								Key   string
								Value struct{ Value any }
							}
							Value float64
							Count uint64
						}
					}
				}
			}
		}
		if err := json.Unmarshal([]byte(line), &data); err != nil {
			t.Fatalf("decode stdout metrics: %v", err)
		}
		points = make(map[string]int64)
		for _, scope := range data.ScopeMetrics {
			for _, series := range scope.Metrics {
				name, ok := strings.CutPrefix(series.Name, prefix)
				if !ok {
					continue
				}
				for _, point := range series.Data.DataPoints {
					key := name
					for _, a := range point.Attributes {
						key += ":" + fmt.Sprint(a.Value.Value)
					}
					points[key] = int64(point.Value) + int64(point.Count)
				}
			}
		}
	}
	return points
}
