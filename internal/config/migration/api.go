package migration

import (
	"errors"
	"fmt"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/config"
)

// ErrEmptyProcesses is returned when process block has no processes or processes-match.
var ErrEmptyProcesses = errors.New("process block requires processes or processes-match")

// ErrDuplicateProcess is returned when the same process appears multiple times.
var ErrDuplicateProcess = errors.New("duplicate process in process block")

// ErrAPICollision is returned when migration would overwrite an existing named process block.
var ErrAPICollision = errors.New("process block collision: old syntax process conflicts with existing named block")

// MigrateAPIBlocks transforms old api syntax to new named syntax.
//
// Handles two cases:
//
// 1. Anonymous process blocks:
//
//	api { processes [ foo ]; neighbor-changes; }
//
// Becomes:
//
//	api foo { receive { state; } }
//
// 2. Named process blocks with processes inside:
//
//	api speaking {
//	    processes [ foo ];
//	    receive { parsed; update; }
//	}
//
// Becomes:
//
//	api foo {
//	    content { format parsed; }
//	    receive { update; }
//	}
//
// Format mapping:
//   - parsed only (no packets) → format parsed
//   - packets only (no parsed) → format raw
//   - parsed + packets OR consolidate → format full
//
// State flag (neighbor-changes) → receive { state; }
//
// Note: Format flags in send block (parsed, packets, consolidate) are dropped
// since ZeBGP uses a single format for both directions.
//
// Returns a new tree; original is not modified.
func MigrateAPIBlocks(tree *config.Tree) (*config.Tree, error) {
	if tree == nil {
		return nil, ErrNilTree
	}

	result := tree.Clone()

	// Process peer blocks
	for _, entry := range result.GetListOrdered("peer") {
		if err := migrateAPIFromPeer("peer "+entry.Key, entry.Value); err != nil {
			return nil, err
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
func migrateAPIFromPeer(location string, peer *config.Tree) error {
	if peer == nil {
		return nil
	}

	apiList := peer.GetListOrdered("process")
	if len(apiList) == 0 {
		return nil
	}

	// Collect all blocks that need migration (to avoid modifying while iterating)
	type migrationTask struct {
		key     string
		apiTree *config.Tree
	}
	var tasks []migrationTask

	for _, entry := range apiList {
		if needsMigration(entry.Value) {
			tasks = append(tasks, migrationTask{key: entry.Key, apiTree: entry.Value})
		}
	}

	if len(tasks) == 0 {
		return nil
	}

	// Process each block that needs migration
	for _, task := range tasks {
		if err := migrateAPIBlock(location, peer, task.key, task.apiTree); err != nil {
			return err
		}
	}

	return nil
}

// needsMigration returns true if the process block uses old syntax.
// Delegates to isOldStyleAPIBlock in detect.go to avoid duplication.
func needsMigration(apiTree *config.Tree) bool {
	return isOldStyleAPIBlock(apiTree)
}

// migrateAPIBlock migrates a single process block (anonymous or named).
func migrateAPIBlock(location string, peer *config.Tree, key string, apiTree *config.Tree) error {
	// Extract process names
	processNames := extractProcessNames(apiTree)
	matchPatterns := extractProcessesMatch(apiTree)

	// For named blocks without processes field, the key IS the process name
	if len(processNames) == 0 && len(matchPatterns) == 0 {
		if key == config.KeyDefault {
			return fmt.Errorf("%s: %w", location, ErrEmptyProcesses)
		}
		// Named block without processes - just transform in place
		processNames = []string{key}
	}

	// Check for duplicates
	seen := make(map[string]bool)
	for _, name := range processNames {
		if seen[name] {
			return fmt.Errorf("%s: %w: %s", location, ErrDuplicateProcess, name)
		}
		seen[name] = true
	}
	for _, pattern := range matchPatterns {
		if seen[pattern] {
			return fmt.Errorf("%s: %w: %s", location, ErrDuplicateProcess, pattern)
		}
		seen[pattern] = true
	}

	// Check for collision with existing named process blocks
	apiList := peer.GetList("process")
	for name := range seen {
		if name == key {
			continue // Same block, will be replaced
		}
		if _, exists := apiList[name]; exists {
			return fmt.Errorf("%s: %w: %s", location, ErrAPICollision, name)
		}
	}

	// Extract configuration from old block
	cfg := extractAPIConfig(apiTree)

	// Remove the old block
	peer.RemoveListEntry("process", key)

	// Create new named blocks for each process
	for _, procName := range processNames {
		newAPI := buildNewAPIBlock(cfg)
		peer.AddListEntry("process", procName, newAPI)
	}
	for _, procName := range matchPatterns {
		newAPI := buildNewAPIBlock(cfg)
		peer.AddListEntry("process", procName, newAPI)
	}

	return nil
}

// apiConfig holds extracted configuration from old process block.
type apiConfig struct {
	// Format from receive block (parsed/packets/consolidate)
	format string

	// Message types from receive block
	receiveUpdate       bool
	receiveOpen         bool
	receiveNotification bool
	receiveKeepalive    bool
	receiveRefresh      bool
	receiveOperational  bool

	// State from api-level flags
	receiveState bool

	// Message types from send block
	sendUpdate      bool
	sendRefresh     bool
	sendKeepalive   bool
	sendOperational bool
}

// extractAPIConfig extracts configuration from old-style process block.
func extractAPIConfig(apiTree *config.Tree) apiConfig {
	cfg := apiConfig{}

	// Extract neighbor-changes flag from api level (maps to receive { state; })
	if _, ok := apiTree.GetFlex("neighbor-changes"); ok {
		cfg.receiveState = true
	}

	// Extract from receive block
	if recv := apiTree.GetContainer("receive"); recv != nil {
		cfg.format = extractFormat(recv)
		cfg.receiveUpdate = hasFlag(recv, "update")
		cfg.receiveOpen = hasFlag(recv, "open")
		cfg.receiveNotification = hasFlag(recv, "notification")
		cfg.receiveKeepalive = hasFlag(recv, "keepalive")
		cfg.receiveRefresh = hasFlag(recv, "refresh")
		cfg.receiveOperational = hasFlag(recv, "operational")
	}

	// Extract from send block
	if send := apiTree.GetContainer("send"); send != nil {
		cfg.sendUpdate = hasFlag(send, "update")
		cfg.sendRefresh = hasFlag(send, "refresh")
		cfg.sendKeepalive = hasFlag(send, "keepalive")
		cfg.sendOperational = hasFlag(send, "operational")
	}

	return cfg
}

// extractFormat determines format from parsed/packets/consolidate flags.
func extractFormat(block *config.Tree) string {
	hasParsed := hasFlag(block, "parsed")
	hasPackets := hasFlag(block, "packets")
	hasConsolidate := hasFlag(block, "consolidate")

	// consolidate implies full format
	if hasConsolidate {
		return "full"
	}

	// Both parsed and packets = full
	if hasParsed && hasPackets {
		return "full"
	}

	// Only packets = raw
	if hasPackets && !hasParsed {
		return "raw"
	}

	// Only parsed or nothing specified = parsed (default)
	if hasParsed {
		return "parsed"
	}

	return "" // No format flags, inherit default
}

// hasFlag checks if a flag is set in a tree (handles both Get and GetFlex).
func hasFlag(tree *config.Tree, flag string) bool {
	if _, ok := tree.Get(flag); ok {
		return true
	}
	if _, ok := tree.GetFlex(flag); ok {
		return true
	}
	return false
}

// buildNewAPIBlock creates a new process block from extracted config.
func buildNewAPIBlock(cfg apiConfig) *config.Tree {
	newAPI := config.NewTree()

	// Add content block if format is specified
	if cfg.format != "" {
		content := config.NewTree()
		content.Set("format", cfg.format)
		newAPI.SetContainer("content", content)
	}

	// Add receive block if any receive flags are set
	if cfg.receiveState || cfg.receiveUpdate || cfg.receiveOpen ||
		cfg.receiveNotification || cfg.receiveKeepalive || cfg.receiveRefresh ||
		cfg.receiveOperational {
		receive := config.NewTree()
		if cfg.receiveState {
			receive.Set("state", "true")
		}
		if cfg.receiveUpdate {
			receive.Set("update", "true")
		}
		if cfg.receiveOpen {
			receive.Set("open", "true")
		}
		if cfg.receiveNotification {
			receive.Set("notification", "true")
		}
		if cfg.receiveKeepalive {
			receive.Set("keepalive", "true")
		}
		if cfg.receiveRefresh {
			receive.Set("refresh", "true")
		}
		if cfg.receiveOperational {
			receive.Set("operational", "true")
		}
		newAPI.SetContainer("receive", receive)
	}

	// Add send block if any send flags are set
	if cfg.sendUpdate || cfg.sendRefresh || cfg.sendKeepalive || cfg.sendOperational {
		send := config.NewTree()
		if cfg.sendUpdate {
			send.Set("update", "true")
		}
		if cfg.sendRefresh {
			send.Set("refresh", "true")
		}
		if cfg.sendKeepalive {
			send.Set("keepalive", "true")
		}
		if cfg.sendOperational {
			send.Set("operational", "true")
		}
		newAPI.SetContainer("send", send)
	}

	return newAPI
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
