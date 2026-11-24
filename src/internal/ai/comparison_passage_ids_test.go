package ai

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"tangentry/internal/citation"
	"tangentry/internal/retrieval"
)

// Models choose evidence IDs; actual quotes are extracted from the exact
// supplied segment and must still pass the existing C source/range checks.
func passageSelectionDraft(evidence []comparisonEvidence) comparisonDraft {
	draft := comparisonDraft{Answer: "两份资料分别给出了退款期限。"}
	for _, source := range evidence {
		draft.AnswerSources = append(draft.AnswerSources, source.ContentID)
		row := comparisonDraftSource{ContentID: source.ContentID, State: "supported", Fact: source.Passages[0].Text,
			PassageIDs: []string{source.Passages[0].ID}}
		draft.Sources = append(draft.Sources, row)
	}
	return draft
}

func TestComparisonExtractsExactSavedTextFromSelectedPassages(t *testing.T) {
	for _, original := range []string{
		"原标题：《侠盗猎车手6》游戏评测：10分｜篝火。作为RockstarGames时隔13年推出的《侠盗猎车手》系列新 ...",
		"《作品》：中文标点、空 格、emoji😀和Ｘ全角字。",
		"14 天内退款，14 天内退款（重复文字仍属于同一完整片段）。",
	} {
		service, source, provider, request := newComparisonFixture(t)
		snapshot := source.snapshots[request.ContentIDs[0]]
		snapshot.Blocks[0].Text = original
		source.snapshots[request.ContentIDs[0]] = snapshot
		var supplied string
		provider.answer = func(_ context.Context, evidence []comparisonEvidence) (string, error) {
			supplied = evidence[0].Passages[0].Text
			draft := passageSelectionDraft(evidence)
			raw, err := json.Marshal(draft)
			return string(raw), err
		}
		result, err := service.Compare(context.Background(), request)
		if err != nil || result.Answer == "" || result.Budget.Started != 1 {
			t.Fatalf("server-extracted evidence failed: %+v %v", result, err)
		}
		ref := result.Sources[0].Citations[0]
		if supplied == "" || !strings.HasPrefix(original, supplied) || ref.Quote != supplied || ref.Status != citation.StatusVerified || ref.ContentID != request.ContentIDs[0] || ref.StartRune != 0 || ref.EndRune != len([]rune(supplied)) {
			t.Fatalf("source text or provenance changed: %+v", ref)
		}
		reopened, err := service.ComparisonEvidence(context.Background(), ref)
		if err != nil || string([]rune(reopened.Text)[reopened.StartRune:reopened.EndRune]) != supplied {
			t.Fatalf("server-extracted quote failed exact reopen: %+v %v", reopened, err)
		}
	}
}

type comparisonRangeFailureAfterGeneration struct {
	*compareSourceFixture
	fail bool
}

func (s *comparisonRangeFailureAfterGeneration) ReadRange(ctx context.Context, id, locator string, start, end int) (string, string, error) {
	if s.fail {
		return "", "", errors.New("original range became unreadable")
	}
	return s.compareSourceFixture.ReadRange(ctx, id, locator, start, end)
}

func TestComparisonExtractedQuotesStillRequireFreshRangeVerification(t *testing.T) {
	service, source, provider, request := newComparisonFixture(t)
	ranges := &comparisonRangeFailureAfterGeneration{compareSourceFixture: source}
	service.SetComparisonSource(ranges)
	provider.answer = func(_ context.Context, evidence []comparisonEvidence) (string, error) {
		draft := passageSelectionDraft(evidence)
		ranges.fail = true
		raw, err := json.Marshal(draft)
		return string(raw), err
	}
	result, err := service.Compare(context.Background(), request)
	if err != nil || result.Answer == "" {
		t.Fatalf("range failure hid the model answer: %+v %v", result, err)
	}
	for _, row := range result.Sources {
		if row.State != "insufficient_evidence" {
			t.Fatalf("source range was not revalidated: %+v", row)
		}
	}
}

func TestComparisonScreenshotTitleIsNotAcceptedAsLegacyBodyQuote(t *testing.T) {
	service, source, provider, request := newComparisonFixture(t)
	snapshot := source.snapshots[request.ContentIDs[0]]
	snapshot.Blocks[0].Text = "原标题：《侠盗猎车手6》游戏评测：10分｜篝火。"
	source.snapshots[request.ContentIDs[0]] = snapshot
	provider.answer = func(_ context.Context, evidence []comparisonEvidence) (string, error) {
		var draft comparisonDraft
		_ = json.Unmarshal([]byte(compareDraftFixture(evidence, "")), &draft)
		draft.Sources[0].Citations[0].Quote = "《GTA 6》游玩体验"
		raw, err := json.Marshal(draft)
		return string(raw), err
	}
	result, err := service.Compare(context.Background(), request)
	if err != nil || result.Answer == "" || result.Sources[0].State != "insufficient_evidence" || result.Sources[0].Citations[0].Status == citation.StatusVerified {
		t.Fatalf("legacy title quote was silently replaced or accepted: %+v %v", result, err)
	}
}

func TestComparisonPassageSelectionsCannotBypassProvenanceOrFreshness(t *testing.T) {
	for _, mode := range []string{"foreign", "foreign_identical", "unknown", "source_handle", "native_id", "stale", "same_version_edit", "read_failure"} {
		t.Run(mode, func(t *testing.T) {
			service, source, provider, request := newComparisonFixture(t)
			if mode == "foreign_identical" {
				snapshot := source.snapshots[request.ContentIDs[1]]
				snapshot.Blocks = append([]retrieval.Block(nil), source.snapshots[request.ContentIDs[0]].Blocks...)
				source.snapshots[request.ContentIDs[1]] = snapshot
			}
			provider.answer = func(_ context.Context, evidence []comparisonEvidence) (string, error) {
				draft := passageSelectionDraft(evidence)
				switch mode {
				case "foreign", "foreign_identical":
					draft.Sources[0].PassageIDs[0] = evidence[1].Passages[0].ID
				case "unknown":
					draft.Sources[0].PassageIDs[0] = "S1-P9999"
				case "source_handle":
					draft.Sources[0].PassageIDs[0] = "S1"
				case "native_id":
					passages, _ := retrieval.SnapshotPassages(source.snapshots[request.ContentIDs[0]])
					draft.Sources[0].PassageIDs[0] = passages[0].ID
				case "stale", "same_version_edit":
					snapshot := source.snapshots[request.ContentIDs[0]]
					if mode == "stale" {
						snapshot.Version = "new-version"
					} else {
						snapshot.Blocks = []retrieval.Block{{Index: 0, Text: "changed text with same version"}}
					}
					source.snapshots[request.ContentIDs[0]] = snapshot
				case "read_failure":
					source.failures[request.ContentIDs[0]] = errors.New("became unreadable")
				}
				raw, err := json.Marshal(draft)
				return string(raw), err
			}
			result, err := service.Compare(context.Background(), request)
			if err != nil || result.Answer == "" || result.Sources[0].State != "insufficient_evidence" || result.Sources[1].State != "supported" || result.Budget.Started != 1 {
				t.Fatalf("bad passage selection accepted or other source lost: %+v %v", result, err)
			}
			if strings.HasPrefix(mode, "foreign") && result.Sources[0].Reason != "cross_source_reference" {
				t.Fatalf("foreign passage reason lost: %+v", result.Sources[0])
			}
		})
	}
}

func TestComparisonRejectsMixedDuplicateOrOversizedPassageSelections(t *testing.T) {
	for _, mode := range []string{"mixed", "duplicate", "oversized", "empty_id"} {
		service, _, provider, request := newComparisonFixture(t)
		provider.answer = func(_ context.Context, evidence []comparisonEvidence) (string, error) {
			draft := passageSelectionDraft(evidence)
			switch mode {
			case "mixed":
				draft.Sources[0].Citations = []comparisonDraftCitation{{PassageID: evidence[0].Passages[0].ID, Quote: "invented quote"}}
			case "duplicate":
				draft.Sources[0].PassageIDs = append(draft.Sources[0].PassageIDs, draft.Sources[0].PassageIDs[0])
			case "oversized":
				draft.Sources[0].PassageIDs = make([]string, 9)
			case "empty_id":
				draft.Sources[0].PassageIDs = []string{""}
			}
			raw, err := json.Marshal(draft)
			return string(raw), err
		}
		if _, err := service.Compare(context.Background(), request); !errors.Is(err, ErrFormat) {
			t.Fatalf("invalid passage selection %s accepted: %v", mode, err)
		}
	}
}
