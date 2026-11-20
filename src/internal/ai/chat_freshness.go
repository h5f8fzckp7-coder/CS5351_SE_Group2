package ai

import (
	"context"

	"tangentry/internal/citation"
	"tangentry/internal/freshness"
)

// ContentVersionReader looks up the live saved_content revision and hash.
// A missing item must return a Current with Exists=false and a nil error.
// Any error fails closed: callers never display evidence as currently valid.
type ContentVersionReader interface {
	CurrentContentVersion(ctx context.Context, contentID string) (freshness.Current, error)
}

// SetContentVersionReader connects the optional content-version lookup at the
// composition root. Without it, stored answers cannot be proven current and
// are assessed as if the lookup failed (fail closed).
func (s *Service) SetContentVersionReader(reader ContentVersionReader) {
	s.mu.Lock()
	s.contentVersions = reader
	s.mu.Unlock()
}

func (s *Service) contentVersionReader() ContentVersionReader {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.contentVersions
}

func lookupCurrent(ctx context.Context, reader ContentVersionReader, contentID string) (freshness.Current, error) {
	if reader == nil {
		return freshness.Current{}, errVersionLookupUnavailable
	}
	return reader.CurrentContentVersion(ctx, contentID)
}

// errVersionLookupUnavailable is a sentinel used when the service was not
// wired with a content version reader. Assess treats any lookup error as
// unavailable (fail closed), so this must be a non-nil error.
var errVersionLookupUnavailable = errSentinel("content version lookup is not configured")

type errSentinel string

func (e errSentinel) Error() string { return string(e) }

// stampFreshness records the live content snapshot on a newly stored assistant
// message and its citations, and sets their freshness to the assessed value.
func stampFreshness(message *ChatMessage, contentID string, current freshness.Current, lookupErr error) {
	if message == nil {
		return
	}
	if lookupErr == nil && current.Exists {
		message.SourceRevision = current.Revision
		message.SourceHash = current.Hash
	}
	captured := messageSnapshot(*message, contentID)
	message.VersionStatus = freshness.Assess(captured, current, lookupErr)
	for i := range message.Citations {
		if lookupErr == nil && current.Exists {
			if message.Citations[i].ContentRevision == 0 {
				message.Citations[i].ContentRevision = current.Revision
			}
			if message.Citations[i].ContentHash == "" {
				message.Citations[i].ContentHash = current.Hash
			}
		}
		message.Citations[i].Freshness = freshness.Assess(citationSnapshot(message.Citations[i], contentID), current, lookupErr)
	}
}

// applyFreshness recomputes version status for stored messages against the
// live content. It does not rewrite C's generation-time Status field.
func applyFreshness(messages []ChatMessage, contentID string, current freshness.Current, lookupErr error) []ChatMessage {
	out := append([]ChatMessage(nil), messages...)
	for i := range out {
		if out[i].Role != "assistant" {
			out[i].VersionStatus = ""
			continue
		}
		out[i].VersionStatus = freshness.Assess(messageSnapshot(out[i], contentID), current, lookupErr)
		if len(out[i].Citations) == 0 {
			continue
		}
		citations := append([]citation.Citation(nil), out[i].Citations...)
		var worst freshness.Status
		for j := range citations {
			citations[j].Freshness = freshness.Assess(citationSnapshot(citations[j], contentID), current, lookupErr)
			worst = freshness.Worst(worst, citations[j].Freshness)
		}
		out[i].Citations = citations
		out[i].VersionStatus = freshness.Worst(out[i].VersionStatus, worst)
	}
	return out
}

func messageSnapshot(message ChatMessage, contentID string) freshness.Snapshot {
	return freshness.Snapshot{
		ContentID:       contentID,
		ContentRevision: message.SourceRevision,
		ContentHash:     message.SourceHash,
	}
}

func citationSnapshot(c citation.Citation, fallbackContentID string) freshness.Snapshot {
	contentID := c.ContentID
	if contentID == "" {
		contentID = fallbackContentID
	}
	return freshness.Snapshot{
		ContentID:       contentID,
		ContentRevision: c.ContentRevision,
		ContentHash:     c.ContentHash,
		SourceVersion:   c.SourceVersion,
	}
}

// stripUntrustedEvidence clears client-supplied citations and freshness so
// only server-assessed values are stored or returned.
func stripUntrustedEvidence(messages []ChatMessage) {
	for i := range messages {
		messages[i].Citations = nil
		messages[i].VersionStatus = ""
		messages[i].SourceRevision = 0
		messages[i].SourceHash = ""
	}
}

func (s *Service) assessHistory(ctx context.Context, contentID string, messages []ChatMessage) []ChatMessage {
	current, err := lookupCurrent(ctx, s.contentVersionReader(), contentID)
	return applyFreshness(messages, contentID, current, err)
}
