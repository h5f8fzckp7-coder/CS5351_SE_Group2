package ai

import (
	"strings"
	"unicode"
)

// comparisonSingleSentence checks surface sentence boundaries, not meaning.
// Decimal points, initials and common abbreviations are not boundaries.
// Closing quotes/brackets and repeated terminal marks may end one sentence.
func comparisonSingleSentence(answer string) bool {
	runes := []rune(answer)
	for i, r := range runes {
		if !strings.ContainsRune("。！？.!?", r) {
			continue
		}
		if r == '.' {
			if i > 0 && i+1 < len(runes) && unicode.IsDigit(runes[i-1]) && unicode.IsDigit(runes[i+1]) {
				continue
			}
			start := i
			for start > 0 && (unicode.IsLetter(runes[start-1]) || runes[start-1] == '.') {
				start--
			}
			word := strings.ToLower(string(runes[start:i]))
			switch word {
			case "mr", "mrs", "ms", "dr", "prof", "e.g", "i.e", "vs", "u.s", "u.k":
				continue
			}
			if i+1 < len(runes) && !unicode.IsSpace(runes[i+1]) && unicode.IsLetter(runes[i+1]) {
				continue // Domain names and dotted identifiers.
			}
			if i-start == 1 && unicode.IsLetter(runes[start]) {
				continue // Initials.
			}
		}
		for _, tail := range runes[i+1:] {
			if !unicode.IsSpace(tail) && !strings.ContainsRune("。！？.!?\"'”’」』）)]】", tail) {
				return false
			}
		}
		return true
	}
	return true // A concise sentence need not have terminal punctuation.
}
