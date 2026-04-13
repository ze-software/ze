// Design: docs/architecture/core-design.md -- sysctl conflict detection
// Related: profiles.go -- profile definitions that conflict rules apply to

package sysctl

import (
	"strings"
	"sync"
)

// ConflictRule describes a pair of sysctl keys that are incompatible
// when both active on the same interface with specific values.
type ConflictRule struct {
	KeyA   string // Suffix match (e.g., "arp_ignore")
	ValueA string // Value that triggers the conflict
	KeyB   string // Suffix match (e.g., "proxy_arp")
	ValueB string // Value that triggers the conflict
	Reason string // Human-readable explanation
}

var (
	conflictMu    sync.RWMutex
	conflictRules []ConflictRule
)

// RegisterConflict adds a conflict rule to the table.
func RegisterConflict(r ConflictRule) {
	conflictMu.Lock()
	defer conflictMu.Unlock()

	conflictRules = append(conflictRules, r)
}

// AllConflicts returns a copy of all registered conflict rules.
func AllConflicts() []ConflictRule {
	conflictMu.RLock()
	defer conflictMu.RUnlock()

	result := make([]ConflictRule, len(conflictRules))
	copy(result, conflictRules)
	return result
}

// CheckConflicts checks a set of active sysctl key/value pairs against
// the conflict table. Returns all matching conflict rules.
// Keys are matched by suffix (e.g., rule KeyA "arp_ignore" matches
// "net.ipv4.conf.eth0.arp_ignore").
func CheckConflicts(active map[string]string) []ConflictRule {
	conflictMu.RLock()
	defer conflictMu.RUnlock()

	var matches []ConflictRule
	for _, rule := range conflictRules {
		aFound, bFound := false, false
		for key, val := range active {
			if strings.HasSuffix(key, "."+rule.KeyA) && val == rule.ValueA {
				aFound = true
			}
			if strings.HasSuffix(key, "."+rule.KeyB) && val == rule.ValueB {
				bFound = true
			}
		}
		if aFound && bFound {
			matches = append(matches, rule)
		}
	}
	return matches
}

// ResetConflicts clears all conflict rules. Only for use in tests.
func ResetConflicts() {
	conflictMu.Lock()
	defer conflictMu.Unlock()
	conflictRules = nil
}
