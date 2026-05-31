package store

import (
	"strings"
	"testing"

	"github.com/voocel/ainovel-cli/internal/domain"
)

func completeBookStore(t *testing.T, total int, completed ...int) *Store {
	t.Helper()
	s := NewStore(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := s.Progress.Init("test", total); err != nil {
		t.Fatalf("InitProgress: %v", err)
	}
	for _, ch := range completed {
		if err := s.Drafts.SaveFinalChapter(ch, "第%d章终稿。"); err != nil {
			t.Fatalf("SaveFinalChapter(%d): %v", ch, err)
		}
		if err := s.Progress.MarkChapterComplete(ch, 6, "", ""); err != nil {
			t.Fatalf("MarkChapterComplete(%d): %v", ch, err)
		}
	}
	return s
}

func TestCompleteBookMarksPhaseComplete(t *testing.T) {
	s := completeBookStore(t, 2, 1, 2)

	if err := s.CompleteBook(); err != nil {
		t.Fatalf("CompleteBook: %v", err)
	}
	progress, err := s.Progress.Load()
	if err != nil {
		t.Fatalf("Load progress: %v", err)
	}
	if progress.Phase != domain.PhaseComplete {
		t.Fatalf("expected phase complete, got %s", progress.Phase)
	}
}

func TestCompleteBookRejectsMissingCompletedChapter(t *testing.T) {
	s := completeBookStore(t, 2, 1)

	err := s.CompleteBook()
	if err == nil {
		t.Fatal("expected error before all chapters are completed")
	}
	if !strings.Contains(err.Error(), "第 2/2 章") {
		t.Fatalf("error = %v, want missing chapter context", err)
	}
}

func TestCompleteBookRejectsMissingFinalArtifact(t *testing.T) {
	s := NewStore(t.TempDir())
	if err := s.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := s.Progress.Save(&domain.Progress{
		NovelName:         "test",
		Phase:             domain.PhaseWriting,
		TotalChapters:     1,
		CompletedChapters: []int{1},
	}); err != nil {
		t.Fatalf("Save progress: %v", err)
	}

	err := s.CompleteBook()
	if err == nil {
		t.Fatal("expected error when final chapter artifact is missing")
	}
	if !strings.Contains(err.Error(), "终稿") {
		t.Fatalf("error = %v, want final artifact context", err)
	}
}

func TestCompleteBookRejectsInProgressChapter(t *testing.T) {
	s := completeBookStore(t, 1, 1)
	if err := s.Progress.StartChapter(1); err != nil {
		t.Fatalf("StartChapter: %v", err)
	}

	err := s.CompleteBook()
	if err == nil {
		t.Fatal("expected error with in-progress chapter")
	}
	if !strings.Contains(err.Error(), "仍处于写作中") {
		t.Fatalf("error = %v, want in-progress context", err)
	}
}
