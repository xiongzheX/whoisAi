package ai

import (
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"whoisai/internal/game"
)

func TestLoadConfigFromEnv(t *testing.T) {
	t.Setenv("AI_API_KEY", "test-key")
	t.Setenv("AI_BASE_URL", "https://example.com/v1")
	t.Setenv("AI_MODEL", "gpt-4o-mini")
	t.Setenv("AI_TIMEOUT_MS", "2500")

	config := LoadConfigFromEnv()
	if !config.Enabled() {
		t.Fatal("Enabled = false, want true")
	}
	if config.APIKey != "test-key" || config.BaseURL != "https://example.com/v1" || config.Model != "gpt-4o-mini" {
		t.Fatalf("config = %+v, want env values", config)
	}
	if got := config.Timeout.Milliseconds(); got != 2500 {
		t.Fatalf("timeout = %dms, want 2500ms", got)
	}
}

func TestClientRewriteCallsOpenAICompatibleEndpoint(t *testing.T) {
	t.Parallel()

	var authHeader string
	var path string
	var body string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		path = r.URL.Path
		rawBody, _ := io.ReadAll(r.Body)
		body = string(rawBody)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"外部模型改写"}}]}`))
	}))
	defer server.Close()

	client := NewClient(Config{
		APIKey:  "secret",
		BaseURL: server.URL,
		Model:   "demo-model",
	}, log.New(io.Discard, "", 0))

	rewritten, err := client.Rewrite("我觉得不行", game.StylePolite)
	if err != nil {
		t.Fatalf("Rewrite returned error: %v", err)
	}
	if rewritten != "外部模型改写" {
		t.Fatalf("Rewrite = %q, want external response", rewritten)
	}
	if path != "/chat/completions" {
		t.Fatalf("path = %q, want /chat/completions", path)
	}
	if authHeader != "Bearer secret" {
		t.Fatalf("Authorization = %q, want bearer token", authHeader)
	}
	if !strings.Contains(body, `"model":"demo-model"`) {
		t.Fatalf("request body = %s, want model field", body)
	}
	if !strings.Contains(body, "我觉得不行") {
		t.Fatalf("request body = %s, want user message", body)
	}
}

func TestClientRewriteReturnsErrorWhenDisabled(t *testing.T) {
	t.Parallel()

	client := NewClient(Config{}, log.New(io.Discard, "", 0))
	if _, err := client.Rewrite("我觉得不行", game.StyleNeutral); err == nil {
		t.Fatal("Rewrite returned nil error, want disabled error")
	}
}
