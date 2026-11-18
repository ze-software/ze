// Design: docs/architecture/storage-backends.md -- registered storage patterns.
package cli

import (
	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/zefs"
)

// registeredPattern holds only the public description, never a stored value.
type registeredPattern struct {
	Pattern     string `json:"pattern"`
	Description string `json:"description"`
}

type registeredPatterns struct {
	Patterns []registeredPattern `json:"patterns"`
}

// registeredAnswer is the one lookup for standalone and daemon forms. The
// registry bounds the walk; an unknown exact pattern is never an empty success.
func registeredAnswer(pattern string) command.LocalAnswer {
	entries := zefs.Entries()
	answer := registeredPatterns{Patterns: make([]registeredPattern, 0, len(entries))}
	for _, entry := range entries {
		if pattern != "" {
			if entry.Pattern != pattern {
				continue
			}
		}
		answer.Patterns = append(answer.Patterns, registeredPattern{
			Pattern: entry.Pattern, Description: entry.Description,
		})
	}
	if pattern != "" {
		if len(answer.Patterns) == 0 {
			refusal := opererr.New("unknown storage key pattern "+pattern,
				"this binary has no registered description for that exact pattern", "show data registered")
			return command.LocalRefusal(1, command.LocalErrorLine(refusal))
		}
	}
	return command.LocalAnswer{Payload: answer}
}
