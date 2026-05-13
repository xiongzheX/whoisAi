package envfile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadSetsEnvFromFileWithoutOverridingExisting(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	content := "# comment\nPORT=4010\nAI_MODEL=\"gpt-4o-mini\"\nAI_BASE_URL='https://example.com/v1'\nBROKEN_LINE\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile returned error: %v", err)
	}

	t.Setenv("AI_API_KEY", "pre-set-key")
	if err := Load(path); err != nil {
		t.Fatalf("Load returned error: %v", err)
	}

	if got := os.Getenv("PORT"); got != "4010" {
		t.Fatalf("PORT = %q, want 4010", got)
	}
	if got := os.Getenv("AI_MODEL"); got != "gpt-4o-mini" {
		t.Fatalf("AI_MODEL = %q, want gpt-4o-mini", got)
	}
	if got := os.Getenv("AI_BASE_URL"); got != "https://example.com/v1" {
		t.Fatalf("AI_BASE_URL = %q, want https://example.com/v1", got)
	}
	if got := os.Getenv("AI_API_KEY"); got != "pre-set-key" {
		t.Fatalf("AI_API_KEY = %q, want pre-set-key", got)
	}
}

func TestLoadMissingFileIsIgnored(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if err := Load(path); err != nil {
		t.Fatalf("Load returned error for missing file: %v", err)
	}
}
