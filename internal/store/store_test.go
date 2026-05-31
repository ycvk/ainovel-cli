package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFoundationMissingReturnsReadErrors(t *testing.T) {
	s := NewStore(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := os.WriteFile(filepath.Join(s.Dir(), "outline.json"), []byte("{"), 0o644); err != nil {
		t.Fatalf("corrupt outline: %v", err)
	}

	_, err := s.FoundationMissing()
	if err == nil {
		t.Fatal("FoundationMissing should fail on unreadable foundation facts")
	}
	if !strings.Contains(err.Error(), "outline") {
		t.Fatalf("error = %v, want outline context", err)
	}
}

func TestLoadLatestSnapshotsReturnsLayeredOutlineReadError(t *testing.T) {
	s := NewStore(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := os.WriteFile(filepath.Join(s.Dir(), "layered_outline.json"), []byte("{"), 0o644); err != nil {
		t.Fatalf("corrupt layered outline: %v", err)
	}

	_, err := s.Characters.LoadLatestSnapshots()
	if err == nil {
		t.Fatal("LoadLatestSnapshots should fail on unreadable layered outline")
	}
	if !strings.Contains(err.Error(), "layered_outline") {
		t.Fatalf("error = %v, want layered_outline context", err)
	}
}
