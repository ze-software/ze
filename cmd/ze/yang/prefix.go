// Design: docs/architecture/config/yang-config-design.md -- YANG analysis tool
// Related: tree.go -- unified analysis tree builder
// Related: format.go -- output formatting for collisions and trees
//
// Package yang provides the ze yang CLI subcommand for YANG tree analysis.
package yang

import "sort"

// Source domain constants for analysis nodes.
const (
	SourceConfig  = "config"
	SourceCommand = "command"
	SourceBoth    = "both"
)

// Filter constants for tree output.
const (
	FilterCommands = "commands"
)

// SiblingInfo describes a node at a given tree level for prefix analysis.
type SiblingInfo struct {
	Name        string
	Source      string // SourceConfig, SourceCommand, or SourceBoth
	Type        string // YANG type (config nodes) or empty (command nodes)
	Description string
}

// CollisionGroup describes a set of siblings that share a prefix.
type CollisionGroup struct {
	Path     []string      // tree path to the parent node
	Prefix   string        // shared first character(s)
	MinChars int           // minimum chars needed to disambiguate any pair
	MaxChars int           // maximum chars needed to disambiguate worst pair
	Siblings []SiblingInfo // the colliding siblings
}

// FindCollisions groups siblings by shared first character and returns
// collision groups where the minimum disambiguation depth meets or exceeds
// minPrefix. minPrefix=1 means any shared first character is reported.
func FindCollisions(siblings []SiblingInfo, minPrefix int) []CollisionGroup {
	if len(siblings) < 2 {
		return nil
	}

	// Group by first character.
	byFirst := make(map[byte][]SiblingInfo)
	for _, s := range siblings {
		if s.Name == "" {
			continue
		}
		first := s.Name[0]
		byFirst[first] = append(byFirst[first], s)
	}

	var groups []CollisionGroup
	for _, members := range byFirst {
		if len(members) < 2 {
			continue
		}

		minD, maxD := disambiguationDepth(members)
		if maxD < minPrefix {
			continue
		}

		sort.Slice(members, func(i, j int) bool {
			return members[i].Name < members[j].Name
		})

		groups = append(groups, CollisionGroup{
			Prefix:   string(members[0].Name[0]),
			MinChars: minD,
			MaxChars: maxD,
			Siblings: members,
		})
	}

	sort.Slice(groups, func(i, j int) bool {
		return groups[i].Prefix < groups[j].Prefix
	})

	return groups
}

// disambiguationDepth computes the minimum and maximum number of characters
// needed to uniquely identify each member in the group. For each member,
// find the longest common prefix with any other member -- that plus one
// is the disambiguation depth for that member.
func disambiguationDepth(members []SiblingInfo) (minDepth, maxDepth int) {
	if len(members) < 2 {
		return 0, 0
	}

	// Sort names for efficient pairwise comparison.
	names := make([]string, len(members))
	for i, m := range members {
		names[i] = m.Name
	}
	sort.Strings(names)

	// For each name, the longest common prefix is with its sorted neighbor.
	maxDepth = 0
	minDepth = len(names[0]) // start high

	for i := range len(names) - 1 {
		lcp := longestCommonPrefix(names[i], names[i+1])
		depth := lcp + 1
		if depth > maxDepth {
			maxDepth = depth
		}
	}

	// MinDepth: for each name, find its max LCP with any neighbor.
	for i := range names {
		d := 1 // at minimum 1 char
		if i > 0 {
			d = max(d, longestCommonPrefix(names[i], names[i-1])+1)
		}
		if i < len(names)-1 {
			d = max(d, longestCommonPrefix(names[i], names[i+1])+1)
		}
		minDepth = min(minDepth, d)
	}

	return minDepth, maxDepth
}

// longestCommonPrefix returns the length of the longest common prefix of a and b.
func longestCommonPrefix(a, b string) int {
	n := min(len(a), len(b))
	for i := range n {
		if a[i] != b[i] {
			return i
		}
	}
	return n
}
