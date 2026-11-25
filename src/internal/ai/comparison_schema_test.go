package ai

import (
	"context"
	"encoding/json"
	"testing"

	"tangentry/internal/citation"
)

// Independent global enums accepted a real film quote attributed to an app
// article. Source IDs and passage IDs must be constrained together.
func TestComparisonSchemaBindsEachSourceToItsOwnPassages(t *testing.T) {
	evidence := []comparisonEvidence{
		{ContentID: "S1", Complete: true, Passages: []comparisonPassage{{ID: "S1-P1"}, {ID: "S1-P2"}}},
		{ContentID: "S2", Passages: []comparisonPassage{{ID: "S2-P1"}}},
		{ContentID: "S3", Complete: true}, // Empty sources cannot be excluded or supported.
	}
	encoded, err := json.Marshal(comparisonResponseFormat(evidence))
	if err != nil {
		t.Fatal(err)
	}
	var format map[string]any
	if err := json.Unmarshal(encoded, &format); err != nil {
		t.Fatal(err)
	}
	schema := format["json_schema"].(map[string]any)["schema"].(map[string]any)
	dependencies := schema["properties"].(map[string]any)["answerSources"].(map[string]any)
	dependencyIDs := dependencies["items"].(map[string]any)["enum"].([]any)
	if dependencies["minItems"] != float64(1) || dependencies["maxItems"] != float64(3) || len(dependencyIDs) != 3 || dependencyIDs[0] != "S1" || dependencyIDs[2] != "S3" {
		t.Fatalf("answer dependencies are not bound to selected sources: %v", dependencies)
	}
	sources := schema["properties"].(map[string]any)["sources"].(map[string]any)
	if sources["type"] != "object" || sources["additionalProperties"] != false || len(sources["required"].([]any)) != 3 {
		t.Fatalf("schema permits repeated or missing source identities: %v", sources)
	}
	var branches []any
	for _, variants := range sources["properties"].(map[string]any) {
		branches = append(branches, variants.(map[string]any)["anyOf"].([]any)...)
	}
	allowed := map[string]map[string]bool{
		"S1": {"S1-P1": true, "S1-P2": true},
		"S2": {"S2-P1": true},
		"S3": {},
	}
	supported := map[string]bool{}
	for _, branch := range branches {
		properties := branch.(map[string]any)["properties"].(map[string]any)
		ids := properties["contentId"].(map[string]any)["enum"].([]any)
		if len(ids) != 1 {
			t.Fatal("a schema alternative allows mixing source identities")
		}
		id := ids[0].(string)
		states := properties["state"].(map[string]any)["enum"].([]any)
		quotes := properties["passageIds"].(map[string]any)
		if _, exists := properties["citations"]; exists {
			t.Fatal("model is still asked to copy quote text")
		}
		if states[0] != "supported" {
			canExclude := false
			for _, state := range states {
				canExclude = canExclude || state == "not_relevant"
			}
			if canExclude != (id == "S1") {
				t.Fatalf("incomplete or empty source may be excluded: source=%s states=%v", id, states)
			}
			if quotes["maxItems"] != float64(0) {
				t.Fatalf("unsupported source %s may still invent citations", id)
			}
			continue
		}
		supported[id] = true
		passageIDs := quotes["items"].(map[string]any)["enum"].([]any)
		if len(passageIDs) == 0 || len(passageIDs) != len(allowed[id]) {
			t.Fatalf("source %s has wrong passage scope: %v", id, passageIDs)
		}
		for _, passageID := range passageIDs {
			if !allowed[id][passageID.(string)] {
				t.Fatalf("source %s can cite another source's passage: %v", id, passageID)
			}
		}
	}
	if !supported["S1"] || !supported["S2"] || supported["S3"] {
		t.Fatalf("supported source variants: %v", supported)
	}
}

// Providers that ignore JSON schema must not launder another source's passage,
// even when identical wording appears in both saved sources.
func TestComparisonCrossSourceCitationPreservesOtherFinding(t *testing.T) {
	service, source, provider, request := newComparisonFixture(t)
	first := source.snapshots[request.ContentIDs[0]]
	second := source.snapshots[request.ContentIDs[1]]
	second.Blocks = first.Blocks
	source.snapshots[request.ContentIDs[1]] = second
	provider.answer = func(_ context.Context, evidence []comparisonEvidence) (string, error) {
		if evidence[0].Title == "" || evidence[1].Title == "" {
			t.Fatal("source titles missing from the generation context")
		}
		var draft comparisonDraft
		if err := json.Unmarshal([]byte(compareDraftFixture(evidence, "same")), &draft); err != nil {
			return "", err
		}
		draft.Sources[0].Citations = draft.Sources[1].Citations
		raw, err := json.Marshal(draft)
		return string(raw), err
	}
	result, err := service.Compare(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	bad, good := result.Sources[0], result.Sources[1]
	if result.Relation != "insufficient_evidence" || bad.Reason != "cross_source_reference" || bad.State != "insufficient_evidence" || bad.Fact != "" {
		t.Fatalf("cross-source finding passed: %+v", result)
	}
	if len(bad.Citations) != 1 || bad.Citations[0].Status == citation.StatusVerified {
		t.Fatalf("foreign citation promoted: %+v", bad)
	}
	if good.State != "supported" || len(good.Citations) != 1 || good.Citations[0].Status != citation.StatusVerified {
		t.Fatalf("unaffected source lost its verified result: %+v", good)
	}
}
