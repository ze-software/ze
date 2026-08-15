// Design: docs/architecture/config/yang-config-design.md — config search
// Overview: model.go — Model definition and update loop

package cli

import (
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// searchMaxResults caps the number of config search completions to avoid UI sluggishness.
const searchMaxResults = 50

// searchConfig searches the current config set-commands for lines matching the query.
// Each space-separated token in query is a word-prefix filter: "/r a" matches lines
// containing a word starting with "r" followed by a word starting with "a" (e.g., "remote as").
// Results use type "search" so applyCompletion can strip the last word (the value).
//
// A secret value never reaches this function. displaySetView masks the tree
// before it serializes, so the cached text already reads as the placeholder.
// Search used to mask here instead. It matched the leaf NAME against a union of
// SensitiveKeys and BcryptKeys. That was a third answer to the question
// config.LeafHoldsSecret answers. It was right only about the leaves whose
// names it collected.
func (m *Model) searchConfig(query string) []Completion {
	if m.editor == nil {
		return nil
	}

	// Cache the set-view to avoid re-serializing the entire config tree on every keystroke.
	// Invalidated when the tree is dirty (user edited config since last cache).
	if m.searchCache == "" || m.editor.Dirty() {
		m.searchCache = m.editor.displaySetView()
	}
	if m.searchCache == "" {
		return nil
	}

	tokens := strings.Fields(query)
	var results []Completion
	for line := range strings.SplitSeq(m.searchCache, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if matchesPrefixTokens(line, tokens) {
			words := strings.Fields(line)
			if len(words) < 2 {
				continue
			}
			results = append(results, Completion{
				Text:        line,
				Description: textbuf.Join(words[1:], " "),
				Type:        "search",
			})
			if len(results) >= searchMaxResults {
				break
			}
		}
	}
	return results
}

// matchesPrefixTokens returns true if the line contains words matching each token as a prefix,
// in order. Tokens match anywhere in the line's words but must appear in sequence.
func matchesPrefixTokens(line string, tokens []string) bool {
	if len(tokens) == 0 {
		return true
	}
	words := strings.Fields(strings.ToLower(line))
	ti := 0
	for _, w := range words {
		if strings.HasPrefix(w, strings.ToLower(tokens[ti])) {
			ti++
			if ti == len(tokens) {
				return true
			}
		}
	}
	return false
}
