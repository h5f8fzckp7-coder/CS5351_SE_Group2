package ai

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestComparisonAnswerReferencesUseIdentityInsteadOfMentionOrder(t *testing.T) {
	evidence := []comparisonEvidence{{ContentID: "S1", Title: "数学视频"}, {ContentID: "S2", Title: "游戏视频"}}
	if got := renderComparisonSourceReferences("[[S2]]介绍连招，[[S1]]讨论数学。", evidence); got != "「游戏视频」介绍连招，「数学视频」讨论数学。" {
		t.Fatal(got)
	}
	if got := renderComparisonSourceReferences("S2介绍连招，S1讨论数学。", evidence); got != "「游戏视频」介绍连招，「数学视频」讨论数学。" {
		t.Fatal(got)
	}
	// Inserted titles are data, not another round of source references.
	evidence[0].Title = "literal [[S2]] title"
	if got := renderComparisonSourceReferences("[[S1]]与[[S2]]不同。", evidence); got != "「literal [[S2]] title」与「游戏视频」不同。" {
		t.Fatal(got)
	}
}

func TestComparisonAnswerRenderingPreservesLegacyAndLengthBound(t *testing.T) {
	evidence := []comparisonEvidence{{ContentID: "S1", Title: strings.Repeat("长标题", 100)}}
	const legacy = "来源的观点不同，但不一定冲突。"
	if got := renderComparisonSourceReferences(legacy, evidence); got != legacy {
		t.Fatal(got)
	}
	if got := renderComparisonSourceReferences("[[S9]]未知。", evidence); got != "[[S9]]未知。" {
		t.Fatal("renderer guessed a foreign identity", got)
	}
	if got := renderComparisonSourceReferences(strings.Repeat("[[S1]]", 100), evidence); utf8.RuneCountInString(got) > 600 || got != strings.Repeat("①", 100) {
		t.Fatal("expanded answer exceeded display bound", got)
	}
}

func TestComparisonRendersReferencesWithoutAnotherModelCallOrGate(t *testing.T) {
	service, _, provider, request := newComparisonFixture(t)
	provider.answer = func(_ context.Context, evidence []comparisonEvidence) (string, error) {
		var draft comparisonDraft
		_ = json.Unmarshal([]byte(compareDraftFixture(evidence, "")), &draft)
		draft.Answer, draft.AnswerSources = "[[S2]]为30天，[[S1]]为14天。", []string{"S2", "S1"}
		draft.Sources[0].Citations[0].Quote = "invalid citation text"
		raw, err := json.Marshal(draft)
		return string(raw), err
	}
	result, err := service.Compare(context.Background(), request)
	if err != nil || result.Answer != "「Source b」为30天，「Source a」为14天。" || result.Budget.Started != 1 || result.Sources[0].State != "insufficient_evidence" || result.AnswerSourceIDs[0] != request.ContentIDs[1] {
		t.Fatal("rendering changed identity, budget or display policy", result, err)
	}
}
