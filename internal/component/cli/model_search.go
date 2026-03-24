// Design: docs/architecture/config/yang-config-design.md — config search
// Overview: model.go — Model definition and update loop

package cli

import (
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/component/config"
)

// searchMaxResults caps the number of config search completions to avoid UI sluggishness.
const searchMaxResults = 50

// searchConfig searches the current config set-commands for lines matching the query.
// Each space-separated token in query is a word-prefix filter: "/r a" matches lines
// containing a word starting with "r" followed by a word starting with "a" (e.g., "remote as").
// Results use type "search" so applyCompletion can strip the last word (the value).
// Sensitive values (ze:sensitive leaves like md5-password) are masked in results.
func (m *Model) searchConfig(query string) []Completion {
	if m.editor == nil {
		return nil
	}

	// Cache the set-view to avoid re-serializing the entire config tree on every keystroke.
	// Invalidated when the tree is dirty (user edited config since last cache).
	if m.searchCache == "" || m.editor.Dirty() {
		m.searchCache = m.editor.SetView()
	}
	if m.searchCache == "" {
		return nil
	}

	sensitiveKeys := m.editor.SensitiveKeys()
	tokens := strings.Fields(query)
	lines := strings.Split(m.searchCache, "\n")
	var results []Completion
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if matchesPrefixTokens(line, tokens) {
			words := strings.Fields(line)
			if len(words) < 2 {
				continue
			}
			line = maskSensitiveLine(words, sensitiveKeys)
			results = append(results, Completion{
				Text:        line,
				Description: strings.Join(words[1:], " "),
				Type:        "search",
			})
			if len(results) >= searchMaxResults {
				break
			}
		}
	}
	return results
}

// maskSensitiveLine replaces the value (last word) with a placeholder when the
// leaf name (second-to-last word) is a sensitive key. The words slice is modified
// in place so both Text and Description reflect the mask.
// Example: "set peer X md5-password secret" becomes "set peer X md5-password /* SECRET-DATA */".
func maskSensitiveLine(words []string, sensitiveKeys map[string]bool) string {
	if len(words) >= 3 && sensitiveKeys[words[len(words)-2]] {
		words[len(words)-1] = config.SecretDataPlaceholder
	}
	return strings.Join(words, " ")
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
