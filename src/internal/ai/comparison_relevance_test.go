package ai

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"tangentry/internal/retrieval"
)

func relevanceDraft(evidence []comparisonEvidence, excluded int) string {
	var draft comparisonDraft
	_ = json.Unmarshal([]byte(compareDraftFixture(evidence, "")), &draft)
	draft.Answer = "根据最后一份材料，这篇文章主要介绍游戏体验与作品评价。"
	for i := 0; i < excluded; i++ {
		draft.Sources[i] = comparisonDraftSource{ContentID: evidence[i].ContentID, State: "not_relevant"}
	}
	raw, _ := json.Marshal(draft)
	return string(raw)
}

func TestComparisonAnswersFocusedQuestionWithOtherSourcesUnrelated(t *testing.T) {
	service, source, provider, request := newComparisonFixture(t)
	third := "rss:" + strings.Repeat("c", 64)
	source.snapshots[third] = retrieval.Snapshot{ContentID: third, Kind: retrieval.SourceArticle, Version: "1", Blocks: []retrieval.Block{{Index: 0, Text: "本文主要介绍游戏体验与作品评价。"}}}
	request.ContentIDs = append(request.ContentIDs, third)
	request.Question = "只解释第三份材料的主题。"
	provider.answer = func(_ context.Context, evidence []comparisonEvidence) (string, error) {
		return relevanceDraft(evidence, 2), nil
	}
	result, err := service.Compare(context.Background(), request)
	if err != nil || result.Answer == "" || result.Sources[2].State != "supported" || result.Budget.Started != 1 {
		t.Fatalf("focused answer blocked: %+v %v", result, err)
	}
	for _, row := range result.Sources[:2] {
		if row.State != "not_relevant" || row.Reason != "question_unrelated" || row.Fact != "" || len(row.Citations) != 0 {
			t.Fatalf("unrelated source presented as evidence: %+v", row)
		}
	}
}

func TestComparisonRelevanceCannotHideReadFailureOrMissingText(t *testing.T) {
	for _, mode := range []string{"read_failure", "missing_text", "stale"} {
		t.Run(mode, func(t *testing.T) {
			service, source, provider, request := newComparisonFixture(t)
			if mode == "read_failure" {
				source.failures[request.ContentIDs[0]] = errors.New("fixture read failure")
			} else if mode == "missing_text" {
				snapshot := source.snapshots[request.ContentIDs[0]]
				snapshot.Blocks = nil
				source.snapshots[request.ContentIDs[0]] = snapshot
			}
			provider.answer = func(_ context.Context, evidence []comparisonEvidence) (string, error) {
				if mode == "stale" {
					snapshot := source.snapshots[request.ContentIDs[0]]
					snapshot.Version = "new-version"
					source.snapshots[request.ContentIDs[0]] = snapshot
				}
				return relevanceDraft(evidence, 1), nil
			}
			result, err := service.Compare(context.Background(), request)
			if err != nil || result.Answer == "" || result.Sources[0].State != "insufficient_evidence" || result.Sources[1].State != "supported" {
				t.Fatalf("failed source hidden as unrelated: %+v %v", result, err)
			}
		})
	}
}

func TestComparisonAllSourcesUnrelatedStillDisplaysModelAnswer(t *testing.T) {
	service, _, provider, request := newComparisonFixture(t)
	provider.answer = func(_ context.Context, evidence []comparisonEvidence) (string, error) {
		return relevanceDraft(evidence, len(evidence)), nil
	}
	result, err := service.Compare(context.Background(), request)
	if err != nil || result.Answer == "" {
		t.Fatalf("model answer hidden by unrelated source states: %+v %v", result, err)
	}
}

func TestComparisonRelevanceRequiresCompleteEvidenceAndNoClaims(t *testing.T) {
	for _, mode := range []string{"partial", "fact", "field", "citation"} {
		t.Run(mode, func(t *testing.T) {
			_, source, _, request := newComparisonFixture(t)
			id := request.ContentIDs[0]
			snapshot := source.snapshots[id]
			passages, _ := retrieval.SnapshotPassages(snapshot)
			evidence := comparisonEvidence{Complete: mode != "partial", snapshot: snapshot}
			row := comparisonDraftSource{ContentID: "S1", State: "not_relevant"}
			switch mode {
			case "fact":
				row.Fact = "Unsupported claim"
			case "field":
				row.Field = "Refund deadline"
			case "citation":
				var draft comparisonDraft
				_ = json.Unmarshal([]byte(compareDraftFixture([]comparisonEvidence{{ContentID: "S1", Passages: []comparisonPassage{{ID: "S1-P1", Text: "claim"}}}}, "")), &draft)
				row.Citations = draft.Sources[0].Citations
			}
			result := ComparisonSourceResult{ContentID: id, State: "insufficient_evidence"}
			validateComparisonFinding(context.Background(), source, request.Question, row, evidence, passages, &result)
			if result.State != "insufficient_evidence" || result.Reason != "relevance_unconfirmed" {
				t.Fatalf("invalid exclusion accepted: %+v", result)
			}
		})
	}
}

type relevanceRangeFailure struct{ *compareSourceFixture }

func (s relevanceRangeFailure) ReadRange(context.Context, string, string, int, int) (string, string, error) {
	return "", "", errors.New("saved original became unreadable")
}

func TestComparisonRelevanceRevalidatesSavedRanges(t *testing.T) {
	_, source, _, request := newComparisonFixture(t)
	id := request.ContentIDs[0]
	snapshot := source.snapshots[id]
	passages, _ := retrieval.SnapshotPassages(snapshot)
	result := ComparisonSourceResult{ContentID: id, State: "insufficient_evidence"}
	validateComparisonFinding(context.Background(), relevanceRangeFailure{source}, request.Question,
		comparisonDraftSource{State: "not_relevant"}, comparisonEvidence{Complete: true, snapshot: snapshot}, passages, &result)
	if result.State != "insufficient_evidence" || result.Reason != "relevance_unconfirmed" {
		t.Fatalf("unreadable original excluded as unrelated: %+v", result)
	}
}
