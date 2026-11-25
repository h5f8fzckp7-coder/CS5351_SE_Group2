package ai

import (
	"context"

	"tangentry/internal/citation"
	"tangentry/internal/content"
	"tangentry/internal/retrieval"
)

// ComparisonSource reads only saved, accessible items and their current evidence.
type ComparisonSource interface {
	Get(context.Context, string) (content.Item, error)
	retrieval.PassageSource
}

// ComparisonGenerator optionally supports structured provider output. The
// provider must use the same invocation ledger for retries and fallbacks.
type ComparisonGenerator interface {
	CompareEvidence(context.Context, Config, ComparisonPrompt) (string, error)
}

type ComparisonPrompt struct {
	Source         string
	Question       string
	ResponseFormat map[string]any
}

type CompareRequest struct {
	ContentIDs       []string `json:"contentIds"`
	Question         string   `json:"question"`
	Revision         int64    `json:"revision"`
	Model            string   `json:"model,omitempty"`
	MaxProviderCalls *int     `json:"maxProviderCalls,omitempty"`
	MaxChars         *int     `json:"maxChars,omitempty"`
}

type ComparisonCoverage struct {
	Field         string `json:"field"`
	SourceVersion string `json:"sourceVersion"`
	SnapshotHash  string `json:"snapshotHash"`
	Blocks        int    `json:"blocks"`
}

type ComparisonSourceResult struct {
	ContentID     string              `json:"contentId"`
	Title         string              `json:"title"`
	State         string              `json:"state"`
	Reason        string              `json:"reason,omitempty"`
	Fact          string              `json:"fact,omitempty"`
	SourceVersion string              `json:"sourceVersion,omitempty"`
	Citations     []citation.Citation `json:"citations"`
	Coverage      *ComparisonCoverage `json:"coverage,omitempty"`
	Retrieval     *retrieval.Response `json:"retrieval,omitempty"`
}

type ComparisonResult struct {
	Status          string                   `json:"status"`
	Relation        string                   `json:"relation,omitempty"` // Legacy categorical metadata.
	Answer          string                   `json:"answer,omitempty"`
	AnswerSourceIDs []string                 `json:"answerSourceIds,omitempty"`
	Sources         []ComparisonSourceResult `json:"sources"`
	Model           string                   `json:"model,omitempty"`
	Budget          *BudgetReport            `json:"budget,omitempty"`
}

type comparisonEvidence struct {
	ContentID            string              `json:"contentId"`
	Title                string              `json:"title"`
	Passages             []comparisonPassage `json:"passages"`
	Complete             bool                `json:"complete"`
	Kind                 string              `json:"kind,omitempty"`
	VideoDurationSeconds float64             `json:"videoDurationSeconds,omitempty"`
	CoverageStartSeconds float64             `json:"coverageStartSeconds,omitempty"`
	CoverageEndSeconds   float64             `json:"coverageEndSeconds,omitempty"`
	Scope                string              `json:"scope,omitempty"`
	// Local verification state is never supplied by a client or a model.
	snapshot retrieval.Snapshot
}

type comparisonPassage struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

type comparisonDraft struct {
	Relation      string                 `json:"relation,omitempty"`
	Answer        string                 `json:"answer"`
	AnswerSources []string               `json:"answerSources,omitempty"`
	Sources       comparisonDraftSources `json:"sources"`
}

// Current providers return one fixed object member per selected source.
// Legacy arrays are still decoded conservatively for older providers.
type comparisonDraftSources []comparisonDraftSource

type comparisonDraftSource struct {
	ContentID  string                    `json:"contentId"`
	State      string                    `json:"state"`
	Fact       string                    `json:"fact"`
	Field      string                    `json:"field"`
	PassageIDs []string                  `json:"passageIds,omitempty"`
	Citations  []comparisonDraftCitation `json:"citations,omitempty"` // Legacy exact-quote output only.
}

type comparisonDraftCitation struct {
	PassageID string `json:"passageId"`
	Quote     string `json:"quote"`
}

func (s *Service) SetComparisonSource(source ComparisonSource) {
	s.mu.Lock()
	s.comparisonSource = source
	s.mu.Unlock()
}

type ComparisonEvidenceResult struct {
	Title        string   `json:"title"`
	ContentID    string   `json:"contentId"`
	Text         string   `json:"text"`
	StartRune    int      `json:"startRune"`
	EndRune      int      `json:"endRune"`
	Locator      string   `json:"locator"`
	StartSeconds *float64 `json:"startSeconds,omitempty"`
}

// ComparisonEvidence revalidates a citation when a reader opens it. The text
// and highlight range come from the current saved source, including after an
// intervening edit or deletion.
func (s *Service) ComparisonEvidence(ctx context.Context, ref citation.Citation) (ComparisonEvidenceResult, error) {
	s.mu.Lock()
	source := s.comparisonSource
	s.mu.Unlock()
	if source == nil {
		return ComparisonEvidenceResult{}, ErrNotReady
	}
	if !content.ValidID(ref.ContentID) || ref.Quote == "" || ref.EndRune <= ref.StartRune ||
		ref.StartRune < 0 || ref.Status != citation.StatusVerified {
		return ComparisonEvidenceResult{}, ErrInvalid
	}
	item, err := source.Get(ctx, ref.ContentID)
	if err != nil {
		return ComparisonEvidenceResult{}, ErrInvalid
	}
	text, version, err := source.ReadRange(ctx, ref.ContentID, ref.Locator, ref.StartRune, ref.EndRune)
	if err != nil || text != ref.Quote || version != ref.SourceVersion {
		return ComparisonEvidenceResult{}, ErrConflict
	}
	snapshot, err := source.Snapshot(ctx, ref.ContentID)
	if err != nil {
		return ComparisonEvidenceResult{}, ErrConflict
	}
	passages, _ := retrieval.SnapshotPassages(snapshot)
	for _, passage := range passages {
		if passage.ID != ref.PassageID || passage.Locator != ref.Locator ||
			ref.StartRune < passage.StartRune || ref.EndRune > passage.EndRune {
			continue
		}
		verified := citation.Validate(ref.ContentID, []citation.RawCitation{{PassageID: ref.PassageID, Quote: ref.Quote}},
			[]retrieval.Passage{passage})
		if len(verified) != 1 || verified[0].Status != citation.StatusVerified ||
			verified[0].StartRune != ref.StartRune || passage.SourceVersion != ref.SourceVersion {
			return ComparisonEvidenceResult{}, ErrConflict
		}
		result := ComparisonEvidenceResult{Title: item.Title, ContentID: item.ID, Text: passage.Text,
			StartRune: ref.StartRune - passage.StartRune, EndRune: ref.EndRune - passage.StartRune, Locator: passage.Locator}
		if passage.Source == retrieval.SourceSubtitle && len(passage.CueIDs) > 0 {
			for _, track := range snapshot.Tracks {
				for _, cue := range track.Cues {
					if cue.ID == passage.CueIDs[0] {
						start := float64(cue.StartMs) / 1000
						result.StartSeconds = &start
					}
				}
			}
		}
		return result, nil
	}
	return ComparisonEvidenceResult{}, ErrConflict
}
