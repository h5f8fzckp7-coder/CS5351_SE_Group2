package sqlite

import (
	"context"
	"errors"

	"tangentry/internal/content"
	"tangentry/internal/freshness"
)

// CurrentContentVersion returns the live saved_content revision and hash for
// Owner E freshness checks. A missing row is Exists=false with a nil error.
func (s *Store) CurrentContentVersion(ctx context.Context, id string) (freshness.Current, error) {
	item, err := s.Get(ctx, id)
	if errors.Is(err, content.ErrNotFound) {
		return freshness.Current{Exists: false}, nil
	}
	if err != nil {
		return freshness.Current{}, err
	}
	return freshness.Current{Exists: true, Revision: item.ContentRevision, Hash: item.ContentHash}, nil
}
