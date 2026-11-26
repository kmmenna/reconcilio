package openairank

import (
	"encoding/json/v2"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/kmmenna/reconcilio/apps/reconciliation/internal/reconciliation"
)

func TestResponsesRequestAndBilingualResult(t *testing.T) {
	var calls atomic.Int32
	endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method != http.MethodPost || r.URL.Path != "/v1/responses" ||
			r.Header.Get("Authorization") != "Bearer test-key" || r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}
		var request struct {
			Model        string `json:"model"`
			Instructions string `json:"instructions"`
			Store        *bool  `json:"store"`
			Input        string `json:"input"`
			Text         struct {
				Format struct {
					Type   string         `json:"type"`
					Strict bool           `json:"strict"`
					Schema map[string]any `json:"schema"`
				} `json:"format"`
			} `json:"text"`
		}
		if err := json.Unmarshal(body, &request); err != nil {
			t.Error(err)
		}
		if request.Model != "example-model" || request.Store == nil || *request.Store ||
			!strings.Contains(request.Instructions, "untrusted") || !strings.Contains(request.Input, "temporary") ||
			request.Text.Format.Type != "json_schema" || !request.Text.Format.Strict || request.Text.Format.Schema["additionalProperties"] != false {
			t.Errorf("unexpected request body: %s", body)
		}
		io.WriteString(w, `{"status":"completed","model":"example-model-snapshot","usage":{"input_tokens":49,"output_tokens":60},"output":[{"type":"reasoning"},{"type":"message","role":"assistant","status":"completed","content":[{"type":"output_text","text":"{\"ranking\":[{\"candidate_id\":\"temporary\",\"score\":0,\"explanations\":{\"pt-BR\":\"Descrição diferente.\",\"en\":\"Different description.\"}}]}"}]}]}`)
	}))
	defer endpoint.Close()

	client, err := New("test-key", "example-model", endpoint.URL+"/v1/responses")
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.Rank(t.Context(), reconciliation.RankingInput{
		InternalDescription: "Canva monthly subscription",
		Candidates:          []reconciliation.RankingCandidate{{ID: "temporary", Description: "ADOBE CLOUD"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 1 || result.Model != "example-model-snapshot" || result.Usage == nil ||
		result.Usage.InputTokens != 49 || result.Usage.OutputTokens != 60 ||
		len(result.Ranking) != 1 || result.Ranking[0].Score == nil || *result.Ranking[0].Score != 0 ||
		result.Ranking[0].Explanations.Portuguese != "Descrição diferente." || result.Ranking[0].Explanations.English != "Different description." {
		t.Fatalf("unexpected response: %+v", result)
	}
}

func TestResponsesFallbackKeepsReportedUsage(t *testing.T) {
	for _, tc := range []struct {
		name, response string
		want           error
	}{
		{"refusal", `"status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"refusal","refusal":"cannot rank"}]}]`, reconciliation.ErrRankingRefused},
		{"incomplete", `"status":"incomplete"`, reconciliation.ErrRankingIncomplete},
		{"ambiguous output", `"status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"{}"},{"type":"output_text","text":"{}"}]}]`, reconciliation.ErrInvalidRanking},
		{"malformed ranking", `"status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"not json"}]}]`, reconciliation.ErrInvalidRanking},
	} {
		t.Run(tc.name, func(t *testing.T) {
			endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				io.WriteString(w, `{"model":"snapshot","usage":{"input_tokens":10,"output_tokens":20},`+tc.response+`}`)
			}))
			defer endpoint.Close()
			client, err := New("test-key", "example-model", endpoint.URL)
			if err != nil {
				t.Fatal(err)
			}
			result, err := client.Rank(t.Context(), reconciliation.RankingInput{})
			if !errors.Is(err, tc.want) || result.Model != "snapshot" || result.Usage == nil || result.Usage.OutputTokens != 20 {
				t.Fatalf("got result %+v and error %v, want %v with usage", result, err, tc.want)
			}
		})
	}
}

func TestResponsesDoesNotFollowRedirects(t *testing.T) {
	var calls atomic.Int32
	endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Location", "/another-path")
		w.WriteHeader(http.StatusTemporaryRedirect)
	}))
	defer endpoint.Close()
	client, err := New("test-key", "example-model", endpoint.URL)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Rank(t.Context(), reconciliation.RankingInput{}); err == nil || calls.Load() != 1 {
		t.Fatalf("redirect should fall back after one call: error %v, calls %d", err, calls.Load())
	}
}
