package bootstrap

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLogContextWindowChoiceUsesActualDefaultWindow(t *testing.T) {
	var buf bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	LogContextWindowChoice("writer", "unknown-model", DefaultContextWindow, CtxWindowDefault)

	got := buf.String()
	if strings.Contains(got, "128k") {
		t.Fatalf("log should not contain stale 128k copy: %s", got)
	}
	if !strings.Contains(got, "200k") {
		t.Fatalf("log should include actual default window: %s", got)
	}
}

func TestSaveExampleConfigReturnsConfigDirError(t *testing.T) {
	homeFile := filepath.Join(t.TempDir(), "home-as-file")
	if err := os.WriteFile(homeFile, []byte("not a directory"), 0o644); err != nil {
		t.Fatalf("write home file: %v", err)
	}
	t.Setenv("HOME", homeFile)

	if err := saveExampleConfig(); err == nil {
		t.Fatal("saveExampleConfig should fail when config dir cannot be created")
	}
}
