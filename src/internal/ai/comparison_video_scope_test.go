package ai

import (
	"context"
	"encoding/json"
	"strings"
	"tangentry/internal/content"
	"tangentry/internal/retrieval"
	"testing"
)

type timedComparisonFixture struct{ *compareSourceFixture }

func (s timedComparisonFixture) Get(ctx context.Context, id string) (content.Item, error) {
	item, err := s.compareSourceFixture.Get(ctx, id)
	if item.Kind == "video" {
		item.DurationSeconds = 455
	}
	return item, err
}
func TestComparisonVideoExcerptScopeReachesProvider(t *testing.T) {
	service, source, provider, request := newComparisonFixture(t)
	id := request.ContentIDs[1]
	source.snapshots[id] = retrieval.Snapshot{ContentID: id, Kind: retrieval.SourceSubtitle, Version: "selected-track", Tracks: []retrieval.SubtitleTrack{{Language: "zh", Revision: 2, Cues: []retrieval.Cue{{ID: "zh:0", Language: "zh", StartMs: 160, EndMs: 90140, Text: "从中午12.1直打到晚上快十点"}}}}}
	service.SetComparisonSource(timedComparisonFixture{source})
	inspected := false
	service.provider = &comparisonStructuredFixture{compareProviderFixture: provider, inspect: func(input ComparisonPrompt) {
		inspected = true
		_, raw, _ := strings.Cut(input.Source, "\nSOURCES\n")
		var wire []comparisonEvidence
		if err := json.Unmarshal([]byte(raw), &wire); err != nil {
			t.Fatal(err)
		}
		v := wire[1]
		if !v.Complete || v.Kind != "subtitle" || v.VideoDurationSeconds != 455 || v.CoverageStartSeconds != 0.16 || v.CoverageEndSeconds != 90.14 || v.Scope == "" {
			t.Fatalf("lost partial-video bounds: %+v", v)
		}
		if !strings.Contains(raw, "12.1直打到") || strings.Contains(raw, "12:01") {
			t.Fatal("ASR was rewritten")
		}
		encoded, _ := json.Marshal(input.ResponseFormat)
		var format map[string]any
		_ = json.Unmarshal(encoded, &format)
		props := format["json_schema"].(map[string]any)["schema"].(map[string]any)["properties"].(map[string]any)
		video := props["sources"].(map[string]any)["properties"].(map[string]any)["S2"]
		videoJSON, _ := json.Marshal(video)
		if strings.Contains(string(videoJSON), "not_mentioned") || !strings.Contains(string(videoJSON), "not_relevant") {
			t.Fatal("video schema offers unsupported absence or loses exclusion")
		}
	}}
	result, err := service.Compare(context.Background(), request)
	if err != nil || !inspected || result.Answer == "" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}
