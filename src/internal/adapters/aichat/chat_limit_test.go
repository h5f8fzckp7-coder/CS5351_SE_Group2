package aichat

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"tangentry/internal/ai"
	"testing"
)

// Reasoning models spend part of the completion budget before they emit any
// visible content. The chat path must therefore send an explicit limit; the
// parameter name differs between the two supported protocols.
func TestChatRequestsSetTokenLimit(t *testing.T) {
	for _, tc := range []struct {
		name     string
		protocol string
		want     string
	}{
		{name: "openai", protocol: "openai", want: "max_completion_tokens"},
		{name: "compatible", protocol: "compatible", want: "max_tokens"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got map[string]any
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				raw, _ := io.ReadAll(r.Body)
				if err := json.Unmarshal(raw, &got); err != nil {
					t.Errorf("decode request: %v", err)
				}
				w.Header().Set("Content-Type", "application/json")
				io.WriteString(w, `{"choices":[{"message":{"content":"ok"},"finish_reason":"stop"}]}`)
			}))
			defer server.Close()

			client := New()
			text, err := client.Chat(context.Background(), ai.Config{BaseURL: server.URL, Model: "fixture", Protocol: tc.protocol}, "", []ai.ChatMessage{{Role: "user", Content: "hi"}})
			if err != nil || text != "ok" {
				t.Fatalf("chat = %q, err = %v", text, err)
			}
			assertChatLimit(t, got, tc.want)
		})
	}
}

func TestChatStreamRequestsSetTokenLimit(t *testing.T) {
	for _, tc := range []struct {
		name     string
		protocol string
		want     string
	}{
		{name: "openai", protocol: "openai", want: "max_completion_tokens"},
		{name: "compatible", protocol: "compatible", want: "max_tokens"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got map[string]any
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				raw, _ := io.ReadAll(r.Body)
				if err := json.Unmarshal(raw, &got); err != nil {
					t.Errorf("decode request: %v", err)
				}
				w.Header().Set("Content-Type", "text/event-stream")
				io.WriteString(w, `data: {"choices":[{"delta":{"content":"ok"}}]}`+"\n\n"+
					`data: {"choices":[],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`+"\n\n"+
					"data: [DONE]\n\n")
			}))
			defer server.Close()

			client := New()
			var answer string
			err := client.ChatStream(context.Background(), ai.Config{BaseURL: server.URL, Model: "fixture", Protocol: tc.protocol}, "", []ai.ChatMessage{{Role: "user", Content: "hi"}}, func(delta string) error {
				answer += delta
				return nil
			})
			if err != nil || answer != "ok" {
				t.Fatalf("stream = %q, err = %v", answer, err)
			}
			assertChatLimit(t, got, tc.want)
		})
	}
}

func assertChatLimit(t *testing.T, body map[string]any, want string) {
	t.Helper()
	if body[want] != float64(chatCompletionTokenLimit) {
		t.Fatalf("%s = %v, want %d", want, body[want], chatCompletionTokenLimit)
	}
	other := "max_tokens"
	if want == "max_tokens" {
		other = "max_completion_tokens"
	}
	if _, ok := body[other]; ok {
		t.Fatalf("unexpected %s in request", other)
	}
}
