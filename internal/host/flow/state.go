package flow

import (
	"fmt"

	storepkg "github.com/voocel/ainovel-cli/internal/store"
)

// LoadState 从 Store 读取 Route 所需的全部事实。
// 这是路由的"IO 边界"：所有读取集中在这里，Route 保持纯。
// 读取失败直接返回错误，避免 Router 基于损坏事实继续派发。
func LoadState(store *storepkg.Store) (State, error) {
	missing, err := store.FoundationMissing()
	if err != nil {
		return State{}, fmt.Errorf("load foundation status: %w", err)
	}
	s := State{
		FoundationMissing: missing,
	}
	progress, err := store.Progress.Load()
	if err != nil {
		return State{}, fmt.Errorf("load progress: %w", err)
	}
	if progress == nil {
		return s, nil
	}
	s.Progress = progress

	if n := len(progress.CompletedChapters); n > 0 {
		s.LastCompleted = progress.CompletedChapters[n-1]
	}

	// 弧边界仅在分层模式且有已完成章节时才计算
	if progress.Layered && s.LastCompleted > 0 {
		boundary, err := store.Outline.CheckArcBoundary(s.LastCompleted)
		if err != nil {
			return State{}, fmt.Errorf("load arc boundary for chapter %d: %w", s.LastCompleted, err)
		}
		if boundary != nil {
			s.ArcBoundary = boundary
			if boundary.IsArcEnd {
				review, err := store.World.LoadReview(s.LastCompleted)
				if err != nil {
					return State{}, fmt.Errorf("load arc review for chapter %d: %w", s.LastCompleted, err)
				}
				s.HasArcReview = review != nil && review.Scope == "arc"
				arcSummary, err := store.Summaries.LoadArcSummary(boundary.Volume, boundary.Arc)
				if err != nil {
					return State{}, fmt.Errorf("load arc summary v%d a%d: %w", boundary.Volume, boundary.Arc, err)
				}
				s.HasArcSummary = arcSummary != nil
				if boundary.IsVolumeEnd {
					volSummary, err := store.Summaries.LoadVolumeSummary(boundary.Volume)
					if err != nil {
						return State{}, fmt.Errorf("load volume summary v%d: %w", boundary.Volume, err)
					}
					s.HasVolumeSummary = volSummary != nil
				}
			}
		}
	}

	return s, nil
}
