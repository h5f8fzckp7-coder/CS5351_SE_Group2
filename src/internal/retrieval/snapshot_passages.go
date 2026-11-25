package retrieval

// SnapshotPassages uses the same canonical segmentation and locators as
// retrieval. A short comparison can include all units without a model call.
// dropped reports oversized units, so callers cannot claim complete coverage.
func SnapshotPassages(snapshot Snapshot) (passages []Passage, dropped int) {
	s := &selector{}
	candidates, dropped := s.segments(snapshot, DefaultMaxSegmentRunes)
	for i := range candidates {
		passages = append(passages, candidates[i].passage(snapshot.ContentID, DefaultWeights(), 1))
	}
	return passages, dropped
}

// ForSource snapshots the engine configuration for a scoped source reader.
// Dividing the saved request allowance bounds embedding work across the
// selected sources of one comparison. Existing single-content callers keep
// their original configuration.
func (e *Engine) ForSource(source PassageSource, sourceCount int) *Engine {
	e.mu.Lock()
	defer e.mu.Unlock()
	settings := DefaultEmbeddingSettings()
	if e.Settings != nil {
		settings = *e.Settings
	}
	sourceCount = max(1, sourceCount)
	settings.MaxEmbeddingRequests /= sourceCount
	settings.MaxEmbeddingChars /= sourceCount
	if settings.MaxEmbeddingChars < 1 {
		settings.MaxEmbeddingRequests = 0
		settings.MaxEmbeddingChars = 1
	}
	return &Engine{Source: source, Embedder: e.Embedder, Settings: &settings, Assembler: e.Assembler}
}
