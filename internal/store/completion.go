package store

import (
	"fmt"
	"strings"

	"github.com/voocel/ainovel-cli/internal/domain"
	"github.com/voocel/ainovel-cli/internal/errs"
)

func (s *Store) ValidateBookCompletable() error {
	progress, err := s.Progress.Load()
	if err != nil {
		return fmt.Errorf("load progress: %w: %w", errs.ErrStoreRead, err)
	}
	return s.validateBookCompletable(progress)
}

func (s *Store) CompleteBook() error {
	s.crossMu.Lock()
	defer s.crossMu.Unlock()

	progress, err := s.Progress.Load()
	if err != nil {
		return fmt.Errorf("load progress: %w: %w", errs.ErrStoreRead, err)
	}
	if err := s.validateBookCompletable(progress); err != nil {
		return err
	}
	if err := s.Progress.UpdatePhase(domain.PhaseComplete); err != nil {
		return fmt.Errorf("mark complete: %w: %w", errs.ErrStoreWrite, err)
	}
	return nil
}

func (s *Store) validateBookCompletable(progress *domain.Progress) error {
	if progress == nil {
		return fmt.Errorf("progress 未初始化: %w", errs.ErrToolPrecondition)
	}
	if progress.Phase != domain.PhaseWriting {
		return fmt.Errorf("complete_book 仅在 writing 阶段可调用（当前 phase=%s）: %w", progress.Phase, errs.ErrToolPrecondition)
	}
	if len(progress.PendingRewrites) > 0 {
		return fmt.Errorf("还有 %d 章在返工队列中，处理完再完结全书: %w", len(progress.PendingRewrites), errs.ErrToolPrecondition)
	}
	if progress.InProgressChapter > 0 {
		return fmt.Errorf("第 %d 章仍处于写作中，不能完结全书: %w", progress.InProgressChapter, errs.ErrToolPrecondition)
	}
	if progress.TotalChapters <= 0 {
		return fmt.Errorf("complete_book 需要 progress.total_chapters > 0，当前为 %d: %w", progress.TotalChapters, errs.ErrToolPrecondition)
	}

	completed := make(map[int]bool, len(progress.CompletedChapters))
	for _, ch := range progress.CompletedChapters {
		if ch > 0 {
			completed[ch] = true
		}
	}
	if len(completed) == 0 {
		return fmt.Errorf("complete_book 需要至少 1 个已完成章节，当前为 0: %w", errs.ErrToolPrecondition)
	}

	for ch := 1; ch <= progress.TotalChapters; ch++ {
		if !completed[ch] {
			return fmt.Errorf("complete_book 前置条件未满足：第 %d/%d 章尚未完成: %w", ch, progress.TotalChapters, errs.ErrToolPrecondition)
		}
		text, err := s.Drafts.LoadChapterText(ch)
		if err != nil {
			return fmt.Errorf("load final chapter %d: %w: %w", ch, errs.ErrStoreRead, err)
		}
		if strings.TrimSpace(text) == "" {
			return fmt.Errorf("complete_book 前置条件未满足：第 %d 章终稿不存在或为空: %w", ch, errs.ErrToolPrecondition)
		}
	}
	return nil
}
