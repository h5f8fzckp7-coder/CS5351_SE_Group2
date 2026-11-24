package ai

import (
	"context"
	"errors"
	"strings"
	"testing"

	"tangentry/internal/retrieval"
)

// D-T01 / D-T03: readiness is local-only and never starts a provider request.
func TestComparisonSourceInspectionBeforeGeneration(t *testing.T) {
	service, source, provider, request := newComparisonFixture(t)
	missing := "rss:" + strings.Repeat("c", 64)
	snapshot := source.snapshots[request.ContentIDs[1]]
	snapshot.Blocks = nil
	source.snapshots[request.ContentIDs[1]] = snapshot
	ids := append(append([]string{}, request.ContentIDs...), missing)
	result, err := service.InspectComparisonSources(context.Background(), ids)
	if err != nil || len(result.Sources) != 3 {
		t.Fatalf("inspection=%+v error=%v", result, err)
	}
	for i, want := range []string{"ready", "no_text", "source_unavailable"} {
		if result.Sources[i].ContentID != ids[i] || result.Sources[i].State != want {
			t.Fatalf("source %d: %+v want %s", i, result.Sources[i], want)
		}
	}
	if provider.calls.Load() != 0 {
		t.Fatal("inspection contacted model")
	}
	source.failures[ids[0]] = retrieval.StatusError{Status: "track_required"}
	result, err = service.InspectComparisonSources(context.Background(), ids[:1])
	if err != nil || result.Sources[0].State != "track_required" {
		t.Fatalf("track selection not disclosed: %+v %v", result, err)
	}
}

func TestComparisonSourceInspectionRejectsInvalidInput(t *testing.T) {
	service, _, provider, request := newComparisonFixture(t)
	for _, ids := range [][]string{nil, {"invalid"}, {request.ContentIDs[0], request.ContentIDs[0]}, make([]string, 101)} {
		if _, err := service.InspectComparisonSources(context.Background(), ids); !errors.Is(err, ErrInvalid) {
			t.Fatalf("invalid inspection accepted: %v %v", ids, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := service.InspectComparisonSources(ctx, request.ContentIDs); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
	if provider.calls.Load() != 0 {
		t.Fatal("invalid inspection contacted provider")
	}
}

// D-T03/D-T05: stored-but-oversized text is different from missing text.
func TestComparisonOversizedSourceIsExplicitAndStartsNoGeneration(t *testing.T) {
	service, source, provider, request := newComparisonFixture(t)
	snapshot := source.snapshots[request.ContentIDs[0]]
	snapshot.Blocks[0].Text = strings.Repeat("长", retrieval.DefaultMaxSegmentRunes+1)
	source.snapshots[request.ContentIDs[0]] = snapshot
	other := source.snapshots[request.ContentIDs[1]]
	other.Blocks = nil
	source.snapshots[request.ContentIDs[1]] = other
	inspection, err := service.InspectComparisonSources(context.Background(), request.ContentIDs)
	if err != nil || inspection.Sources[0].State != "oversize_dropped" {
		t.Fatalf("oversized source mislabeled: %+v %v", inspection, err)
	}
	result, err := service.Compare(context.Background(), request)
	if err != nil || result.Relation != "insufficient_evidence" || result.Sources[0].Reason != "oversize_dropped" ||
		result.Sources[1].Reason != "no_text" || provider.calls.Load() != 0 || result.Budget.Started != 0 {
		t.Fatalf("unusable source generated an answer: %+v %v", result, err)
	}
}
