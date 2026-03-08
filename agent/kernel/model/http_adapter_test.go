package model

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestOpenAIAdapterComplete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("missing auth header")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model": "gpt-4o-mini",
			"choices": []any{
				map[string]any{
					"message": map[string]any{"content": "hello from openai"},
				},
			},
		})
	}))
	defer srv.Close()

	adapter := NewHTTPAdapter(HTTPAdapterConfig{
		Provider:  "openai-main",
		APIStyle:  "openai",
		BaseURL:   srv.URL + "/v1",
		Model:     "gpt-4o-mini",
		MaxTokens: 256,
		HTTPClient: &http.Client{
			Timeout: 2 * time.Second,
		},
	})
	resp, err := adapter.Complete(context.Background(), Request{
		Model: "gpt-4o-mini",
		Input: map[string]any{"messages": []any{
			map[string]any{"role": "user", "content": "hello"},
		}},
	}, Credential{ID: "c1", Secret: "test-key"})
	if err != nil {
		t.Fatalf("openai adapter failed: %v", err)
	}
	text, _ := resp.Output["text"].(string)
	if text != "hello from openai" {
		t.Fatalf("unexpected output text: %q", text)
	}
}

func TestAnthropicAdapterComplete(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "test-key" {
			t.Fatalf("missing x-api-key")
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"model": "claude-test",
			"content": []any{
				map[string]any{"type": "text", "text": "hello from anthropic"},
			},
		})
	}))
	defer srv.Close()

	adapter := NewHTTPAdapter(HTTPAdapterConfig{
		Provider: "anthropic-main",
		APIStyle: "anthropic",
		BaseURL:  srv.URL + "/v1",
		Model:    "claude-test",
		HTTPClient: &http.Client{
			Timeout: 2 * time.Second,
		},
	})
	resp, err := adapter.Complete(context.Background(), Request{
		Input: map[string]any{"messages": []any{
			map[string]any{"role": "user", "content": "hello"},
		}},
	}, Credential{ID: "c1", Secret: "test-key"})
	if err != nil {
		t.Fatalf("anthropic adapter failed: %v", err)
	}
	text, _ := resp.Output["text"].(string)
	if text != "hello from anthropic" {
		t.Fatalf("unexpected output text: %q", text)
	}
}

func TestAdapterErrorClassified(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{"message": "unauthorized"},
		})
	}))
	defer srv.Close()

	adapter := NewHTTPAdapter(HTTPAdapterConfig{
		Provider: "openai-main",
		APIStyle: "openai",
		BaseURL:  srv.URL,
		Model:    "gpt",
	})
	_, err := adapter.Complete(context.Background(), Request{Input: map[string]any{"text": "hi"}}, Credential{ID: "c1", Secret: "bad"})
	if err == nil {
		t.Fatal("expected error")
	}
	pe, ok := err.(*ProviderError)
	if !ok || pe.Class != ErrAuthPermanent {
		t.Fatalf("expected auth_permanent, got %T %v", err, err)
	}
}

func TestOpenAIAdapterResponsesEndpoint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/responses" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("missing auth header")
		}
		if r.Header.Get("Accept") != "*/*" {
			t.Fatalf("unexpected accept header: %q", r.Header.Get("Accept"))
		}
		if !strings.HasPrefix(r.Header.Get("User-Agent"), "ActionAgent/") {
			t.Fatalf("unexpected user-agent: %q", r.Header.Get("User-Agent"))
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode req body failed: %v", err)
		}
		if body["model"] != "gpt-5.3-codex" {
			t.Fatalf("unexpected model in body: %v", body["model"])
		}
		input, ok := body["input"].([]any)
		if !ok || len(input) == 0 {
			t.Fatalf("expected input array, got: %#v", body["input"])
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"model":       "gpt-5.3-codex",
			"output_text": "pong from responses",
		})
	}))
	defer srv.Close()

	adapter := NewHTTPAdapter(HTTPAdapterConfig{
		Provider: "openai-main",
		APIStyle: "openai",
		BaseURL:  srv.URL + "/v1",
		Model:    "gpt-5.3-codex",
		HTTPClient: &http.Client{
			Timeout: 2 * time.Second,
		},
	})

	resp, err := adapter.Complete(context.Background(), Request{
		Model: "gpt-5.3-codex",
		Input: map[string]any{
			"input": []any{
				map[string]any{"role": "user", "content": "ping"},
			},
		},
	}, Credential{ID: "c1", Secret: "test-key"})
	if err != nil {
		t.Fatalf("openai responses adapter failed: %v", err)
	}
	text, _ := resp.Output["text"].(string)
	if text != "pong from responses" {
		t.Fatalf("unexpected output text: %q", text)
	}
}
