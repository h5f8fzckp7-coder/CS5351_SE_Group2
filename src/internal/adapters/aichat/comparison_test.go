package aichat

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"tangentry/internal/ai"
)

func TestComparisonStructuredOutputAndBudgetedCompatibilityFallback(t *testing.T) {
	for _, tc := range []struct {
		name                   string
		limit, rejected, calls int
		want                   error
	}{
		{"schema mode", 1, 0, 1, nil},
		{"JSON fallback blocked", 1, 1, 1, ai.ErrBudgetExhausted},
		{"JSON fallback reserved", 2, 1, 2, nil},
		{"plain fallback blocked", 2, 2, 2, ai.ErrBudgetExhausted},
		{"plain fallback reserved", 3, 2, 3, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				call := int(calls.Add(1))
				var body struct {
					Model          string              `json:"model"`
					Stream         bool                `json:"stream"`
					ResponseFormat map[string]any      `json:"response_format"`
					Messages       []map[string]string `json:"messages"`
				}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				if body.Model != "fixture" || body.Stream || len(body.Messages) != 2 || body.Messages[1]["content"] != "question" {
					t.Errorf("bad comparison request: %+v", body)
				}
				mode := body.ResponseFormat["type"]
				if call == 1 && mode != "json_schema" || call == 2 && mode != "json_object" || call == 3 && body.ResponseFormat != nil {
					t.Errorf("unexpected format for call %d: %+v", call, body.ResponseFormat)
				}
				if call <= tc.rejected {
					w.WriteHeader(400)
					_, _ = w.Write([]byte(`{"error":{"param":"response_format","code":"unsupported_parameter"}}`))
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"choices": []map[string]any{{"finish_reason": "stop", "message": map[string]string{"content": `{"relation":"same","sources":[]}`}}}, "usage": map[string]int{"prompt_tokens": 3, "completion_tokens": 4, "total_tokens": 7}})
			}))
			defer server.Close()
			ctx, _ := ai.NewBudgetContext(context.Background(), tc.limit)
			client := New()
			client.HTTP = server.Client()
			output, err := client.CompareEvidence(ctx, ai.Config{BaseURL: server.URL, Model: "fixture"}, ai.ComparisonPrompt{Source: "source evidence", Question: "question", ResponseFormat: map[string]any{"type": "json_schema", "json_schema": map[string]any{"name": "fixture_comparison", "strict": true, "schema": map[string]any{"type": "object"}}}})
			if !errors.Is(err, tc.want) || int(calls.Load()) != tc.calls {
				t.Fatalf("calls=%d output=%s error=%v want=%v", calls.Load(), output, err, tc.want)
			}
			if err == nil && output == "" {
				t.Fatal("missing structured output")
			}
		})
	}
}

// D-T01: malformed generation contracts must never start transport.
func TestComparisonRejectsMissingOutputContract(t *testing.T) {
	_, err := New().CompareEvidence(context.Background(), ai.Config{BaseURL: "http://127.0.0.1:1", Model: "fixture"}, ai.ComparisonPrompt{Source: "evidence", Question: "question"})
	if !errors.Is(err, ai.ErrInvalid) {
		t.Fatalf("missing contract accepted: %v", err)
	}
}
