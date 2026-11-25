package ai

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"tangentry/internal/retrieval"
)

// Reproduce the screenshot's combination: a focused question has one verified
// answer while the other selected sources vary between evidence states.
func TestComparisonFocusedAnswerDoesNotDependOnUnusedEvidenceLabels(t *testing.T) {
	for _, state := range []string{"supported", "not_relevant", "insufficient_evidence", "read_failed"} {
		t.Run(state, func(t *testing.T) {
			service, source, provider, request := newComparisonFixture(t)
			third := "rss:" + strings.Repeat("c", 64)
			source.snapshots[third] = retrieval.Snapshot{ContentID: third, Kind: retrieval.SourceArticle, Version: "1", Blocks: []retrieval.Block{{Index: 0, Text: "游戏评测。"}}}
			request.ContentIDs = append(request.ContentIDs, third)
			request.Question = "影视那篇推荐了哪些类型的作品？"
			film := source.snapshots[request.ContentIDs[1]]
			film.Blocks[0].Text = "推荐电影、剧集、动画和纪录片。"
			source.snapshots[request.ContentIDs[1]] = film
			if state == "read_failed" {
				source.failures[request.ContentIDs[0]] = errors.New("fixture read failure")
			}
			const answer = "第二篇推荐了电影、剧集、动画和纪录片。"
			provider.answer = func(_ context.Context, evidence []comparisonEvidence) (string, error) {
				var draft comparisonDraft
				_ = json.Unmarshal([]byte(compareDraftFixture(evidence, "")), &draft)
				draft.Answer, draft.AnswerSources = answer, []string{evidence[1].ContentID}
				for _, i := range []int{0, 2} {
					if state != "supported" {
						rowState := state
						if state == "read_failed" {
							rowState = "insufficient_evidence"
						}
						draft.Sources[i] = comparisonDraftSource{ContentID: evidence[i].ContentID, State: rowState}
					}
				}
				raw, err := json.Marshal(draft)
				return string(raw), err
			}
			result, err := service.Compare(context.Background(), request)
			if err != nil || result.Answer != answer || len(result.AnswerSourceIDs) != 1 || result.AnswerSourceIDs[0] != request.ContentIDs[1] || result.Budget.Started != 1 {
				t.Fatalf("focused answer lost: %+v %v", result, err)
			}
			if state == "read_failed" && result.Sources[0].Reason != "read_failed" {
				t.Fatalf("unused source failure hidden: %+v", result.Sources[0])
			}
		})
	}
}

func TestComparisonDeclaredAnswerDependenciesDoNotGateDisplay(t *testing.T) {
	for _, mode := range []string{"bad_quote", "cross_source", "stale", "insufficient", "unrelated", "all_sources"} {
		t.Run(mode, func(t *testing.T) {
			service, source, provider, request := newComparisonFixture(t)
			provider.answer = func(_ context.Context, evidence []comparisonEvidence) (string, error) {
				var draft comparisonDraft
				_ = json.Unmarshal([]byte(compareDraftFixture(evidence, "")), &draft)
				draft.AnswerSources = []string{evidence[0].ContentID}
				switch mode {
				case "bad_quote":
					draft.Sources[0].Citations[0].Quote = "not in the source"
				case "cross_source":
					draft.Sources[0].Citations = draft.Sources[1].Citations
				case "stale":
					snapshot := source.snapshots[request.ContentIDs[0]]
					snapshot.Version = "changed"
					source.snapshots[request.ContentIDs[0]] = snapshot
				case "insufficient":
					draft.Sources[0] = comparisonDraftSource{ContentID: evidence[0].ContentID, State: "insufficient_evidence"}
				case "unrelated":
					draft.Sources[0] = comparisonDraftSource{ContentID: evidence[0].ContentID, State: "not_relevant"}
				case "all_sources":
					draft.AnswerSources = append(draft.AnswerSources, evidence[1].ContentID)
					draft.Sources[1].Citations[0].Quote = "not in the source"
				}
				raw, err := json.Marshal(draft)
				return string(raw), err
			}
			result, err := service.Compare(context.Background(), request)
			if err != nil || result.Answer == "" || len(result.AnswerSourceIDs) == 0 {
				t.Fatalf("source verification hid the answer or metadata: %+v %v", result, err)
			}
		})
	}
}

func TestComparisonRejectsInvalidAnswerDependencies(t *testing.T) {
	for _, dependencies := range [][]string{{}, {"S9"}, {"S1", "S1"}, {"S1-P1"}} {
		service, _, provider, request := newComparisonFixture(t)
		provider.answer = func(_ context.Context, evidence []comparisonEvidence) (string, error) {
			var payload map[string]any
			_ = json.Unmarshal([]byte(compareDraftFixture(evidence, "")), &payload)
			payload["answerSources"] = dependencies
			raw, err := json.Marshal(payload)
			return string(raw), err
		}
		if _, err := service.Compare(context.Background(), request); !errors.Is(err, ErrFormat) {
			t.Fatalf("invalid dependencies %v accepted: %v", dependencies, err)
		}
	}
}

func TestComparisonAnswerDependenciesUseIdentityNotRowOrder(t *testing.T) {
	service, _, provider, request := newComparisonFixture(t)
	provider.answer = func(_ context.Context, evidence []comparisonEvidence) (string, error) {
		var draft comparisonDraft
		_ = json.Unmarshal([]byte(compareDraftFixture(evidence, "")), &draft)
		draft.AnswerSources = []string{evidence[1].ContentID}
		draft.Sources[0] = comparisonDraftSource{ContentID: evidence[0].ContentID, State: "insufficient_evidence"}
		draft.Sources[0], draft.Sources[1] = draft.Sources[1], draft.Sources[0]
		raw, err := json.Marshal(draft)
		return string(raw), err
	}
	result, err := service.Compare(context.Background(), request)
	if err != nil || result.Answer == "" || len(result.AnswerSourceIDs) != 1 || result.AnswerSourceIDs[0] != request.ContentIDs[1] {
		t.Fatalf("answer bound to wrong source: %+v %v", result, err)
	}
}

func TestComparisonKeyedSourcesRejectDuplicateMissingOrMismatchedIdentity(t *testing.T) {
	ids := []string{"S1", "S2", "S3"}
	var draft comparisonDraft
	_ = json.Unmarshal([]byte(compareDraftFixture([]comparisonEvidence{
		{ContentID: "S1"}, {ContentID: "S2"}, {ContentID: "S3"},
	}, "")), &draft)
	rows := make([]string, len(draft.Sources))
	for i, row := range draft.Sources {
		raw, _ := json.Marshal(row)
		rows[i] = string(raw)
	}
	wrap := func(sources string) string {
		return `{"answer":"第二篇介绍影视作品。","answerSources":["S2"],"sources":` + sources + `}`
	}
	valid := wrap(`{"S3":` + rows[2] + `,"S2":` + rows[1] + `,"S1":` + rows[0] + `}`)
	if got, err := decodeComparison(valid, ids); err != nil || got.Sources[0].ContentID != "S1" || got.Sources[2].ContentID != "S3" {
		t.Fatalf("fixed source slots lost selection order: %+v %v", got, err)
	}
	for _, invalid := range []string{
		// Real gateway regression: two rows for S2 and no row for S3.
		wrap(`[` + rows[1] + `,` + rows[1] + `,` + rows[0] + `]`),
		wrap(`{"S2":` + rows[1] + `,"S2":` + rows[1] + `,"S1":` + rows[0] + `}`),
		wrap(`{"S1":` + rows[0] + `,"S2":` + rows[1] + `}`),
		wrap(`{"S1":` + rows[0] + `,"S2":` + rows[1] + `,"S3":` + rows[1] + `}`),
		wrap(`{"S1":` + rows[0] + `,"S2":` + rows[1] + `,"S9":` + rows[2] + `}`),
	} {
		if _, err := decodeComparison(invalid, ids); !errors.Is(err, ErrFormat) {
			t.Fatalf("invalid source membership accepted: %v", err)
		}
	}
}
