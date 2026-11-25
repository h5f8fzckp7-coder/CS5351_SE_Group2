package ai

import (
	"context"

	"tangentry/internal/content"
	"tangentry/internal/retrieval"
)

type ComparisonSourceStatus struct {
	ContentID string `json:"contentId"`
	Kind      string `json:"kind,omitempty"`
	State     string `json:"state"`
	Passages  int    `json:"passages"`
}

type ComparisonSourcesResult struct {
	Sources []ComparisonSourceStatus `json:"sources"`
}

// InspectComparisonSources implements D-T01/D-T03 without model calls,
// downloads, platform fetches, or changes to the saved source selection.
// It is advisory: Compare re-reads and verifies sources at submission time.
func (s *Service) InspectComparisonSources(ctx context.Context, ids []string) (ComparisonSourcesResult, error) {
	result := ComparisonSourcesResult{Sources: []ComparisonSourceStatus{}}
	if len(ids) < 1 || len(ids) > 100 {
		return result, ErrInvalid
	}
	seen := make(map[string]bool, len(ids))
	for _, id := range ids {
		if !content.ValidID(id) || seen[id] {
			return result, ErrInvalid
		}
		seen[id] = true
	}
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return result, ErrStorage
	}
	source := s.comparisonSource
	if source == nil {
		s.mu.Unlock()
		return result, ErrNotReady
	}
	s.wait.Add(1)
	s.mu.Unlock()
	defer s.wait.Done()
	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		row := ComparisonSourceStatus{ContentID: id, State: "source_unavailable"}
		item, err := source.Get(ctx, id)
		if ctx.Err() != nil {
			return result, ctx.Err()
		}
		if err == nil && item.ID == id {
			row.Kind = item.Kind
			row.State = "unsupported_source"
			if item.Kind == "article" || item.Kind == "video" {
				snapshot, readErr := source.Snapshot(ctx, id)
				if ctx.Err() != nil {
					return result, ctx.Err()
				}
				switch {
				case readErr != nil:
					row.State = comparisonFailureReason(readErr)
				case snapshot.ContentID != id:
					row.State = "source_unverified"
				default:
					passages, dropped := retrieval.SnapshotPassages(snapshot)
					row.Passages = len(passages)
					row.State = "no_text"
					if len(passages) > 0 {
						row.State = "ready"
					} else if dropped > 0 {
						row.State = "oversize_dropped"
					}
				}
			}
		}
		result.Sources = append(result.Sources, row)
	}
	return result, nil
}
