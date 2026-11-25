package app

import (
	"context"
	"errors"
	"fmt"

	"tangentry/internal/adapters/sqlite"
	"tangentry/internal/articles"
	"tangentry/internal/content"
	"tangentry/internal/retrieval"
	"tangentry/internal/subtitles"
)

type comparisonSource struct{ store *sqlite.Store }

func (s comparisonSource) Get(ctx context.Context, id string) (content.Item, error) {
	return s.store.Get(ctx, id)
}

func (s comparisonSource) Snapshot(ctx context.Context, id string) (retrieval.Snapshot, error) {
	item, err := s.Get(ctx, id)
	if err != nil {
		return retrieval.Snapshot{}, err
	}
	snapshot := retrieval.Snapshot{ContentID: id}
	switch item.Kind {
	case "article":
		blocks := item.ArticleBlocks
		version := fmt.Sprintf("content-revision:%d", item.ContentRevision)
		if item.Platform == "rss" {
			doc, readErr := s.store.GetArticle(ctx, id)
			switch {
			case readErr == nil:
				blocks = doc.Blocks
				version = fmt.Sprintf("article-revision:%d", doc.Revision)
			case !errors.Is(readErr, articles.ErrNotFound):
				return retrieval.Snapshot{}, readErr
			}
		}
		snapshot.Kind = retrieval.SourceArticle
		for i, block := range blocks {
			snapshot.Blocks = append(snapshot.Blocks, retrieval.Block{Index: i, Kind: block.Kind, Level: block.Level, Text: block.Text})
		}
		snapshot.Version = version + ":" + retrieval.SnapshotHash(snapshot)
	case "video":
		tracks, readErr := s.store.ListSavedSubtitles(ctx, id)
		if readErr != nil {
			return retrieval.Snapshot{}, readErr
		}
		var originals []subtitles.SavedTrack
		var selected *subtitles.SavedTrack
		for _, saved := range tracks {
			if saved.Track.Translated || saved.Track.Mismatch {
				continue
			}
			originals = append(originals, saved)
			if saved.Selected {
				track := saved
				selected = &track
			}
		}
		if selected == nil {
			if len(originals) > 1 {
				return retrieval.Snapshot{}, retrieval.StatusError{Status: "track_required"}
			}
			if len(originals) == 1 {
				selected = &originals[0]
			}
		}
		snapshot.Kind = retrieval.SourceSubtitle
		if selected != nil {
			track := retrieval.SubtitleTrack{Language: selected.Track.Language, Revision: selected.Revision}
			for i, cue := range selected.Track.Cues {
				track.Cues = append(track.Cues, retrieval.Cue{
					ID: fmt.Sprintf("%s:%d", track.Language, i), Language: track.Language,
					StartMs: int64(cue.Start * 1000), EndMs: int64(cue.End * 1000), Text: cue.Text,
				})
			}
			snapshot.Tracks = []retrieval.SubtitleTrack{track}
		}
		snapshot.Version = retrieval.SnapshotHash(snapshot)
	default:
		return retrieval.Snapshot{}, retrieval.StatusError{Status: "unsupported_source"}
	}
	return snapshot, nil
}

func (s comparisonSource) ReadRange(ctx context.Context, id, locator string, start, end int) (string, string, error) {
	snapshot, err := s.Snapshot(ctx, id)
	if err != nil {
		return "", "", err
	}
	return retrieval.SnapshotRange(snapshot, locator, start, end)
}
