package migration

import (
	"errors"
	"fmt"
	"strings"

	"github.com/exa-networks/zebgp/pkg/config"
)

// ErrEmptyProcesses is returned when api block has no processes or processes-match.
var ErrEmptyProcesses = errors.New("api block requires processes or processes-match")

// ErrDuplicateProcess is returned when the same process appears multiple times.
var ErrDuplicateProcess = errors.New("duplicate process in api block")

// ErrAPICollision is returned when migration would overwrite an existing named api block.
var ErrAPICollision = errors.New("api block collision: old syntax process conflicts with existing named block")

// MigrateAPIBlocks transforms old api syntax to new named syntax.
//
// For each peer/template block with an old-style api block:
//
//	api { processes [ foo bar ]; neighbor-changes; }
//	api { processes-match [ "^collector" ]; }
//
// Becomes:
//
//	api foo { receive { state; } }
//	api bar { receive { state; } }
//	api "^collector" { }
//
// If neighbor-changes is not set, receive block is omitted (inherit defaults).
//
// Returns error if:
//   - api block has neither processes nor processes-match
//   - duplicate process names found
//   - process name conflicts with existing named api block
//
// Returns a new tree; original is not modified.
// Returns ErrNilTree for nil input.
func MigrateAPIBlocks(tree *config.Tree) (*config.Tree, error) {
	if tree == nil {
		return nil, ErrNilTree
	}

	result := tree.Clone()

	// Process peer blocks
	for _, entry := range result.GetListOrdered("peer") {
		if err := migrateAPIFromPeer("peer "+entry.Key, entry.Value); err != nil {
			return nil, err // Error already contains location
		}
	}

	// Process template.group and template.match blocks
	if tmpl := result.GetContainer("template"); tmpl != nil {
		for _, entry := range tmpl.GetListOrdered("group") {
			if err := migrateAPIFromPeer("template.group "+entry.Key, entry.Value); err != nil {
				return nil, err
			}
		}
		for _, entry := range tmpl.GetListOrdered("match") {
			if err := migrateAPIFromPeer("template.match "+entry.Key, entry.Value); err != nil {
				return nil, err
			}
		}
	}

	return result, nil
}

// migrateAPIFromPeer converts old api syntax to new named syntax in a peer tree.
// Returns error if api block is invalid.
func migrateAPIFromPeer(location string, peer *config.Tree) error {
	if peer == nil {
		return nil
	}

	// Get the api list - old syntax uses _anonymous key
	apiList := peer.GetList("api")
	if len(apiList) == 0 {
		return nil
	}

	// Check for old anonymous syntax
	anonymousAPI := apiList["_anonymous"]
	if anonymousAPI == nil {
		// Already using new syntax or no api block
		return nil
	}

	// Extract process names from "processes [ foo bar ]" and "processes-match [ ... ]"
	processNames := extractProcessNames(anonymousAPI)
	matchPatterns := extractProcessesMatch(anonymousAPI)

	// Validate: must have at least one process or pattern
	if len(processNames) == 0 && len(matchPatterns) == 0 {
		return fmt.Errorf("%s: %w", location, ErrEmptyProcesses)
	}

	// Check for duplicates in processes
	seen := make(map[string]bool)
	for _, name := range processNames {
		if seen[name] {
			return fmt.Errorf("%s: %w: %s", location, ErrDuplicateProcess, name)
		}
		seen[name] = true
	}

	// Check for duplicates in processes-match
	for _, pattern := range matchPatterns {
		if seen[pattern] {
			return fmt.Errorf("%s: %w: %s", location, ErrDuplicateProcess, pattern)
		}
		seen[pattern] = true
	}

	// Check for collision with existing named api blocks
	for name := range seen {
		if _, exists := apiList[name]; exists {
			return fmt.Errorf("%s: %w: %s", location, ErrAPICollision, name)
		}
	}

	// Check for neighbor-changes flag
	hasNeighborChanges := false
	if _, ok := anonymousAPI.GetFlex("neighbor-changes"); ok {
		hasNeighborChanges = true
	}

	// Remove the old anonymous api block
	peer.RemoveListEntry("api", "_anonymous")

	// Create new named api blocks for each process
	for _, processName := range processNames {
		newAPI := config.NewTree()

		if hasNeighborChanges {
			// Add receive { state; } block
			receive := config.NewTree()
			receive.Set("state", "true")
			newAPI.SetContainer("receive", receive)
		}

		peer.AddListEntry("api", processName, newAPI)
	}

	// Create new named api blocks for each match pattern
	for _, pattern := range matchPatterns {
		newAPI := config.NewTree()

		if hasNeighborChanges {
			receive := config.NewTree()
			receive.Set("state", "true")
			newAPI.SetContainer("receive", receive)
		}

		peer.AddListEntry("api", pattern, newAPI)
	}

	return nil
}

// extractProcessNames parses "[ foo bar ]" or "foo bar" format from processes field.
func extractProcessNames(apiTree *config.Tree) []string {
	processesValue, ok := apiTree.Get("processes")
	if !ok {
		return nil
	}

	// Remove brackets and parse space-separated names
	processesValue = strings.Trim(processesValue, "[]")
	names := strings.Fields(processesValue)

	return names
}

// extractProcessesMatch parses "[ pattern1 pattern2 ]" format from processes-match field.
func extractProcessesMatch(apiTree *config.Tree) []string {
	matchValue, ok := apiTree.Get("processes-match")
	if !ok {
		return nil
	}

	// Remove brackets and parse space-separated patterns
	matchValue = strings.Trim(matchValue, "[]")
	patterns := strings.Fields(matchValue)

	return patterns
}
