package ai

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestComparisonReturnsNaturalAnswerWithoutCategory(t *testing.T) {
	service, _, provider, request := newComparisonFixture(t)
	const answer = "两个来源的退款期限不同，分别为14天和30天。"
	provider.answer = func(_ context.Context, evidence []comparisonEvidence) (string, error) {
		var draft comparisonDraft
		_ = json.Unmarshal([]byte(compareDraftFixture(evidence, "")), &draft)
		draft.Answer = answer
		raw, err := json.Marshal(draft)
		return string(raw), err
	}
	result, err := service.Compare(context.Background(), request)
	if err != nil || result.Answer != answer || result.Relation != "" {
		t.Fatalf("sentence was lost or forced into a category: %+v %v", result, err)
	}
	if result.Budget.Started != 1 {
		t.Fatalf("answer unexpectedly required extra generation: %+v", result.Budget)
	}
}

func TestComparisonRejectsMissingMultilineAndOversizedAnswer(t *testing.T) {
	for _, tc := range []struct{ name, answer string }{
		{"empty", ""}, {"multiline", "结论一。\n结论二。"}, {"oversized", strings.Repeat("字", 601)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service, _, provider, request := newComparisonFixture(t)
			provider.answer = func(_ context.Context, evidence []comparisonEvidence) (string, error) {
				var draft comparisonDraft
				_ = json.Unmarshal([]byte(compareDraftFixture(evidence, "")), &draft)
				draft.Answer = tc.answer
				raw, err := json.Marshal(draft)
				return string(raw), err
			}
			if _, err := service.Compare(context.Background(), request); !errors.Is(err, ErrFormat) {
				t.Fatalf("invalid answer accepted: %v", err)
			}
		})
	}
}

func TestComparisonPreservesAnswerWhenSourceVerificationFails(t *testing.T) {
	service, _, provider, request := newComparisonFixture(t)
	provider.answer = func(_ context.Context, evidence []comparisonEvidence) (string, error) {
		var draft comparisonDraft
		_ = json.Unmarshal([]byte(compareDraftFixture(evidence, "")), &draft)
		draft.Answer = "两篇文章完全一致。"
		draft.Sources[0].Citations[0].PassageID = evidence[1].Passages[0].ID
		raw, err := json.Marshal(draft)
		return string(raw), err
	}
	result, err := service.Compare(context.Background(), request)
	if err != nil || result.Answer != "两篇文章完全一致。" || result.Sources[0].Reason != "cross_source_reference" || result.Sources[1].State != "supported" {
		t.Fatalf("model answer hidden or verification diagnostics lost: %+v %v", result, err)
	}
}
