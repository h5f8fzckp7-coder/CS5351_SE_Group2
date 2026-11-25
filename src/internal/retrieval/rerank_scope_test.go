package retrieval

import (
	"context"
	"encoding/json"
	"testing"
)

// The Phase-7 NFR-02 measurement was taken with rerank off: weights
// 0.35/0.65/0 and no reranker. Enabling rerank changes the hybrid weights and
// the passage ordering before assembly, so it is a different configuration and
// must not inherit the rerank-off replay. These tests freeze that boundary.

const rerankScopeQuestion = "居民的權利如何受保障"

// TestDefaultSettingsAreTheFrozenRerankOffConfiguration: the shipped defaults
// are the configuration the frozen replay measured. Changing them silently
// would change what the NFR-02 result refers to.
func TestDefaultSettingsAreTheFrozenRerankOffConfiguration(t *testing.T) {
	settings := DefaultEmbeddingSettings()
	if settings.RerankProvider != "off" || settings.MaxRerankRequests != 0 {
		t.Fatalf("default rerank = %q/%d, want off/0", settings.RerankProvider, settings.MaxRerankRequests)
	}
	if settings.LexicalWeight != 0.35 || settings.EmbeddingWeight != 0.65 || settings.RerankWeight != 0 {
		t.Fatalf("default weights = %v/%v/%v, want 0.35/0.65/0",
			settings.LexicalWeight, settings.EmbeddingWeight, settings.RerankWeight)
	}
	weights := DefaultWeights()
	if weights.Lexical != 0.35 || weights.Embedding != 0.65 || weights.Rerank != 0 {
		t.Fatalf("DefaultWeights = %+v, want 0.35/0.65/0", weights)
	}
}

// TestRerankOffFrozenSelectionReplay: the rerank-off selection is replayed over
// a fixed fixture. The gold passage is selected, the order is frozen, no rerank
// work is reported, and two runs produce byte-identical selections. This is the
// configuration the existing NFR-02 result covers.
func TestRerankOffFrozenSelectionReplay(t *testing.T) {
	_, _, snap := cacheFixture(t)
	cfg := defaultSelectorConfig()
	cfg.Weights = DefaultWeights()

	run := func() Selection {
		t.Helper()
		fresh := newFakeEmbedder(BatchingShared)
		fresh.goldMarkers = []string{"居民的權利和自由受法律保障"}
		selection, err := NewSelector(&fakeSource{snap: snap}, fresh, cfg).
			Select(context.Background(), snap.ContentID, rerankScopeQuestion, 0)
		if err != nil {
			t.Fatal(err)
		}
		return selection
	}

	first, second := run(), run()
	if first.RerankUsed || first.RerankRequests != 0 || first.RerankReason != "" || first.RerankModel != "" {
		t.Fatalf("rerank-off replay reported rerank work: %+v", first)
	}

	// Recall on the frozen fixture: the gold paragraph is selected exactly.
	goldHits := 0
	for _, p := range first.Passages {
		if p.Text == snap.Blocks[0].Text {
			goldHits++
		}
	}
	if goldHits != 1 {
		t.Fatalf("gold paragraph selected %d times, want 1", goldHits)
	}

	// Frozen order for the rerank-off configuration.
	wantLocators := []string{
		ArticleLocator(0, 0, 2),
		ArticleLocator(1, 0, 2),
		ArticleLocator(2, 0, 2),
	}
	if len(first.Passages) != len(wantLocators) {
		t.Fatalf("selected %d passages, want %d", len(first.Passages), len(wantLocators))
	}
	for i, want := range wantLocators {
		if first.Passages[i].Locator != want {
			t.Fatalf("passage %d locator = %q, want %q", i, first.Passages[i].Locator, want)
		}
		if first.Passages[i].RerankScore != 0 {
			t.Fatalf("passage %d rerank score = %v in a rerank-off run", i, first.Passages[i].RerankScore)
		}
	}

	j1, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	j2, err := json.Marshal(second)
	if err != nil {
		t.Fatal(err)
	}
	if string(j1) != string(j2) {
		t.Fatal("two rerank-off replays produced different selections")
	}
}

// TestRerankEnabledIsASeparateConfiguration: enabling rerank changes the
// hybrid score and the passage order. That is why the rerank-off frozen replay
// cannot be extended to the enabled configuration without its own frozen
// rerank scores and replay.
func TestRerankEnabledIsASeparateConfiguration(t *testing.T) {
	src, emb, snap := cacheFixture(t)
	emb.goldMarkers = []string{"居民的權利和自由受法律保障"}

	offCfg := defaultSelectorConfig()
	offCfg.Weights = DefaultWeights()
	off, err := NewSelector(src, emb, offCfg).
		Select(context.Background(), snap.ContentID, rerankScopeQuestion, 0)
	if err != nil {
		t.Fatal(err)
	}

	onCfg := defaultSelectorConfig()
	onCfg.Weights = DefaultRerankWeights()
	onCfg.Reranker = &fakeReranker{model: "frozen-scope-rerank", score: func(text string) float64 {
		if text == snap.Blocks[2].Text {
			return 1
		}
		return 0
	}}
	on, err := NewSelector(src, emb, onCfg).
		Select(context.Background(), snap.ContentID, rerankScopeQuestion, 0)
	if err != nil {
		t.Fatal(err)
	}

	if !on.RerankUsed || on.RerankModel != "frozen-scope-rerank" || on.RerankRequests != 1 {
		t.Fatalf("rerank-enabled run did not report rerank: %+v", on)
	}
	// The reranked paragraph moves ahead of the untouched one: the enabled
	// configuration has a different ordering from the frozen rerank-off replay.
	if len(on.Passages) != 3 || on.Passages[1].Text != snap.Blocks[2].Text {
		t.Fatalf("rerank did not reorder the passages: %+v", on.Passages)
	}
	if on.Passages[1].RerankScore != 1 {
		t.Fatalf("reranked passage score = %v, want 1", on.Passages[1].RerankScore)
	}
	if diff := on.Passages[1].Score - 0.2; diff > 1e-9 || diff < -1e-9 {
		t.Fatalf("reranked passage hybrid score = %v, want 0.2", on.Passages[1].Score)
	}
	if off.Passages[1].Text == on.Passages[1].Text {
		t.Fatal("rerank-enabled ordering unexpectedly matches the rerank-off replay")
	}
	if off.Passages[0].Score == on.Passages[0].Score {
		t.Fatal("rerank-enabled score unexpectedly matches the rerank-off replay")
	}
}