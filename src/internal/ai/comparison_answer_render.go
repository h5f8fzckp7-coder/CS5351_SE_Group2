package ai

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

var comparisonSourceReference = regexp.MustCompile(`\[\[(S[0-9]+)\]\]|\bS[0-9]+\b`)

// Render model-issued source references from the immutable input mapping. This
// changes labels only, never the model's judgment or answer-display policy.
func renderComparisonSourceReferences(answer string, evidence []comparisonEvidence) string {
	labels, compact := make(map[string]string), make(map[string]string)
	for i, source := range evidence {
		title := []rune(strings.Join(strings.Fields(source.Title), " "))
		if len(title) > 28 {
			title = append(title[:27], '…')
		}
		compact[source.ContentID] = string(rune('①' + i))
		labels[source.ContentID] = "「" + string(title) + "」"
		if len(title) == 0 {
			labels[source.ContentID] = compact[source.ContentID]
		}
	}
	render := func(names map[string]string) string {
		return comparisonSourceReference.ReplaceAllStringFunc(answer, func(token string) string {
			if label, ok := names[strings.Trim(token, "[]")]; ok {
				return label
			}
			return token
		})
	}
	rendered := render(labels)
	if utf8.RuneCountInString(rendered) > 600 {
		// Long/repeated titles must not expand a bounded model sentence beyond
		// the display contract. Compact labels preserve source order and facts.
		rendered = render(compact)
	}
	return rendered
}
