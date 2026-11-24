package ai

import (
	"context"
	"encoding/json"
	"testing"

	"tangentry/internal/citation"
	"tangentry/internal/retrieval"
)

// Two independently supported descriptions can differ without contradicting
// one another. Citation verification must not force their relation to same.
func TestComparisonPreservesDifferentTopicsWithVerifiedSources(t *testing.T) {
	service, source, provider, request := newComparisonFixture(t)
	texts := []string{"年轻开发者编写解决日常问题的应用。", "本期推荐十部影视作品，并介绍最新预告片。"}
	for i, id := range request.ContentIDs {
		snapshot := source.snapshots[id]
		snapshot.Blocks = []retrieval.Block{{Index: 0, Kind: "paragraph", Text: texts[i]}}
		source.snapshots[id] = snapshot
	}
	request.Question = "两篇文章的主题是否相同？"
	provider.answer = func(_ context.Context, evidence []comparisonEvidence) (string, error) {
		return compareDraftFixture(evidence, "different"), nil
	}
	result, err := service.Compare(context.Background(), request)
	if err != nil || result.Relation != "different" {
		t.Fatalf("different topics lost: %+v %v", result, err)
	}
	for i, row := range result.Sources {
		if row.State != "supported" || row.Fact != texts[i] || len(row.Citations) != 1 || row.Citations[0].Status != citation.StatusVerified {
			t.Fatalf("source evidence lost: %+v", row)
		}
	}
	encoded, err := json.Marshal(comparisonResponseFormat([]comparisonEvidence{{ContentID: "S1"}, {ContentID: "S2"}}))
	if err != nil {
		t.Fatal(err)
	}
	var format map[string]any
	if err := json.Unmarshal(encoded, &format); err != nil {
		t.Fatal(err)
	}
	schema := format["json_schema"].(map[string]any)["schema"].(map[string]any)
	properties := schema["properties"].(map[string]any)
	if _, fixedCategory := properties["relation"]; fixedCategory {
		t.Fatal("model contract still forces a categorical verdict")
	}
	answer := properties["answer"].(map[string]any)
	if answer["type"] != "string" || answer["maxLength"] != float64(600) || answer["enum"] != nil {
		t.Fatal("answer is not a bounded free-form sentence")
	}
}

func TestComparisonRelationKeepsDifferenceConflictAndEvidenceSeparate(t *testing.T) {
	supported := ComparisonSourceResult{State: "supported"}
	absent := ComparisonSourceResult{State: "not_mentioned"}
	failed := ComparisonSourceResult{State: "insufficient_evidence"}
	for _, tc := range []struct {
		name, proposed, want string
		sources              []ComparisonSourceResult
	}{
		{"equivalent answers", "same", "same", []ComparisonSourceResult{supported, supported}},
		{"different topics", "different", "different", []ComparisonSourceResult{supported, supported}},
		{"incompatible facts", "conflict", "conflict", []ComparisonSourceResult{supported, supported}},
		{"partial failure", "different", "insufficient_evidence", []ComparisonSourceResult{supported, failed}},
		{"absent source", "different", "not_mentioned", []ComparisonSourceResult{supported, absent}},
		{"three sources", "different", "different", []ComparisonSourceResult{supported, supported, supported}},
		{"no evidence", "same", "insufficient_evidence", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := comparisonRelation(tc.proposed, tc.sources); got != tc.want {
				t.Fatalf("got %s, want %s", got, tc.want)
			}
		})
	}
}
