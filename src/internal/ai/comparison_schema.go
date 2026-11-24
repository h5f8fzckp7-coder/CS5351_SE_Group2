package ai

// Ordered properties let structured local decoders ground the source facts
// before selecting evidence states and the overall comparison relationship.
type comparisonSourceProperties struct {
	ContentID  any `json:"contentId"`
	Fact       any `json:"fact"`
	State      any `json:"state"`
	Field      any `json:"field"`
	PassageIDs any `json:"passageIds"`
}

type comparisonResultProperties struct {
	Sources       any `json:"sources"`
	AnswerSources any `json:"answerSources"`
	Answer        any `json:"answer"`
}

// comparisonResponseFormat is the output contract for this invocation, not a
// source of evidence. Only server-issued source/passage handles are allowed.
// Re-reading the real source and C's quotation verifier remain authoritative.
func comparisonResponseFormat(evidence []comparisonEvidence) map[string]any {
	text := map[string]any{"type": "string"}
	fact := map[string]any{"type": "string", "description": "A substantive finding for this source only, in the user's language; never a status word."}
	makeSource := func(source comparisonEvidence, states []string, minimumQuotes int) map[string]any {
		passages := make([]string, 0, len(source.Passages))
		for _, passage := range source.Passages {
			passages = append(passages, passage.ID)
		}
		passageID := map[string]any{"type": "string"}
		if len(passages) > 0 {
			passageID["enum"] = passages
		}
		maximumQuotes := 0
		if minimumQuotes > 0 {
			maximumQuotes = 8
		}
		return map[string]any{"type": "object", "additionalProperties": false,
			"required": []string{"contentId", "state", "fact", "field", "passageIds"},
			"properties": comparisonSourceProperties{
				ContentID: map[string]any{"type": "string", "enum": []string{source.ContentID}}, Fact: fact, Field: text,
				State:      map[string]any{"type": "string", "enum": states},
				PassageIDs: map[string]any{"type": "array", "minItems": minimumQuotes, "maxItems": maximumQuotes, "items": passageID},
			}}
	}
	// Bind each source to only its own passages, including for local decoders.
	// A global union would allow S1 to cite an otherwise valid passage from S2.
	sourceProperties := make(map[string]any, len(evidence))
	for _, source := range evidence {
		variants := make([]any, 0, 2)
		if len(source.Passages) > 0 {
			variants = append(variants, makeSource(source, []string{"supported"}, 1))
		}
		states := []string{"insufficient_evidence"}
		if source.Complete && source.Kind != "subtitle" {
			states = append(states, "not_mentioned")
		}
		if source.Complete && len(source.Passages) > 0 {
			states = append(states, "not_relevant")
		}
		variants = append(variants, makeSource(source, states, 0))
		sourceProperties[source.ContentID] = map[string]any{"anyOf": variants}
	}
	ids := make([]string, len(evidence))
	for i, source := range evidence {
		ids[i] = source.ContentID
	}
	schema := map[string]any{"type": "object", "additionalProperties": false,
		"required": []string{"sources", "answerSources", "answer"}, "properties": comparisonResultProperties{
			Sources:       map[string]any{"type": "object", "additionalProperties": false, "required": ids, "properties": sourceProperties},
			AnswerSources: map[string]any{"type": "array", "minItems": 1, "maxItems": len(evidence), "items": map[string]any{"type": "string", "enum": ids}},
			Answer:        map[string]any{"type": "string", "minLength": 1, "maxLength": 600, "description": "One sentence preserving source judgments. Use [[S1]], [[S2]], [[S3]] for source references; never renumber sources."},
		}}
	return map[string]any{"type": "json_schema", "json_schema": map[string]any{"name": "content_comparison", "strict": true, "schema": schema}}
}
