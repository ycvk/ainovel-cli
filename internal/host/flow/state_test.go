package flow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/voocel/ainovel-cli/internal/domain"
	storepkg "github.com/voocel/ainovel-cli/internal/store"
)

func TestLoadStateReturnsProgressReadError(t *testing.T) {
	st := storepkg.NewStore(t.TempDir())
	if err := st.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := os.WriteFile(filepath.Join(st.Dir(), "meta", "progress.json"), []byte("{"), 0o644); err != nil {
		t.Fatalf("corrupt progress: %v", err)
	}

	_, err := LoadState(st)
	if err == nil {
		t.Fatal("LoadState should fail when progress cannot be read")
	}
	if !strings.Contains(err.Error(), "progress") {
		t.Fatalf("error = %v, want progress context", err)
	}
}

func TestLoadStateReturnsArcSummaryReadError(t *testing.T) {
	st := storepkg.NewStore(t.TempDir())
	if err := st.Init(); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := st.Outline.SaveLayeredOutline([]domain.VolumeOutline{{
		Index: 1,
		Title: "第一卷",
		Arcs: []domain.ArcOutline{{
			Index: 1,
			Title: "第一弧",
			Chapters: []domain.OutlineEntry{{
				Chapter:   1,
				Title:     "第一章",
				CoreEvent: "开局",
			}},
		}},
	}}); err != nil {
		t.Fatalf("save layered outline: %v", err)
	}
	if err := st.Progress.Save(&domain.Progress{
		Phase:             domain.PhaseWriting,
		Flow:              domain.FlowWriting,
		Layered:           true,
		CompletedChapters: []int{1},
	}); err != nil {
		t.Fatalf("save progress: %v", err)
	}
	if err := os.WriteFile(filepath.Join(st.Dir(), "summaries", "arc-v01a01.json"), []byte("{"), 0o644); err != nil {
		t.Fatalf("corrupt arc summary: %v", err)
	}

	_, err := LoadState(st)
	if err == nil {
		t.Fatal("LoadState should fail when arc summary cannot be read")
	}
	if !strings.Contains(err.Error(), "arc summary") {
		t.Fatalf("error = %v, want arc summary context", err)
	}
}
