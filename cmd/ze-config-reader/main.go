// Package main provides the ze-config-reader binary.
// This is a specialized process (not a regular plugin) that:
// 1. Receives YANG schemas from Hub after Stage 1 completes
// 2. Parses configuration files
// 3. Sends namespace commands to Hub for each config block
// 4. Handles reload requests with proper diffing
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/config"
)

// SchemaInfo holds schema information received from Hub.
type SchemaInfo struct {
	Module    string   // YANG module name
	Namespace string   // YANG namespace URI (optional)
	Handlers  []string // Handler paths this schema provides
	Yang      string   // YANG content (optional for Phase 2)
}

// ConfigBlock represents a parsed config block.
type ConfigBlock struct {
	Handler string // Handler path (e.g., "bgp.peer")
	Key     string // List key (e.g., "192.0.2.1" for peer address)
	Path    string // Full path with key (e.g., "bgp.peer[address=192.0.2.1]")
	Data    string // JSON data
}

// ConfigState stores the current configuration as handler → key → data.
type ConfigState struct {
	blocks map[string]map[string]*ConfigBlock // handler → key → block
}

// NewConfigState creates an empty config state.
func NewConfigState() *ConfigState {
	return &ConfigState{
		blocks: make(map[string]map[string]*ConfigBlock),
	}
}

// Set adds or updates a block.
func (cs *ConfigState) Set(block *ConfigBlock) {
	if cs.blocks[block.Handler] == nil {
		cs.blocks[block.Handler] = make(map[string]*ConfigBlock)
	}
	cs.blocks[block.Handler][block.Key] = block
}

// Get returns a block by handler and key.
func (cs *ConfigState) Get(handler, key string) *ConfigBlock {
	if cs.blocks[handler] == nil {
		return nil
	}
	return cs.blocks[handler][key]
}

// ConfigChange represents a change between old and new config.
type ConfigChange struct {
	Action  string // "create", "modify", "delete"
	Handler string // Handler path
	Path    string // Full path with key
	OldData string // Previous data (for modify/delete)
	NewData string // New data (for create/modify)
}

// ConfigReader is the main config reader process state.
type ConfigReader struct {
	schemas    []SchemaInfo
	configPath string
	reader     *bufio.Reader
	writer     *bufio.Writer

	// Handler path to schema mapping for routing.
	handlerMap map[string]*SchemaInfo

	// Current applied configuration state.
	currentState *ConfigState

	// Serial counter for requests.
	serial int
}

// NewConfigReader creates a new config reader instance.
func NewConfigReader() *ConfigReader {
	return &ConfigReader{
		reader:       bufio.NewReader(os.Stdin),
		writer:       bufio.NewWriter(os.Stdout),
		handlerMap:   make(map[string]*SchemaInfo),
		currentState: NewConfigState(),
		serial:       1,
	}
}

// Run is the main entry point.
func (cr *ConfigReader) Run() error {
	// Phase 1: Receive initialization from Hub.
	if err := cr.receiveInit(); err != nil {
		return fmt.Errorf("init: %w", err)
	}

	// Phase 2: Parse config and send commands.
	if err := cr.loadInitialConfig(); err != nil {
		return fmt.Errorf("load: %w", err)
	}

	// Phase 3: Signal completion and wait for commands.
	if err := cr.sendComplete(); err != nil {
		return fmt.Errorf("complete: %w", err)
	}

	// Event loop: handle reload and shutdown.
	return cr.eventLoop()
}

// receiveInit receives schemas and config path from Hub.
func (cr *ConfigReader) receiveInit() error {
	var heredocDelimiter string
	var currentYang strings.Builder

	for {
		line, err := cr.reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("read: %w", err)
		}
		line = strings.TrimSuffix(line, "\n")

		// Handle heredoc continuation.
		if heredocDelimiter != "" {
			if strings.TrimSpace(line) == heredocDelimiter {
				if len(cr.schemas) > 0 {
					cr.schemas[len(cr.schemas)-1].Yang = currentYang.String()
				}
				heredocDelimiter = ""
				currentYang.Reset()
				continue
			}
			if currentYang.Len() > 0 {
				currentYang.WriteString("\n")
			}
			currentYang.WriteString(line)
			continue
		}

		if line == "config done" {
			break
		}

		if strings.HasPrefix(line, "config schema ") {
			schema, delim, err := parseSchemaLine(line)
			if err != nil {
				return fmt.Errorf("parse schema: %w", err)
			}
			cr.schemas = append(cr.schemas, *schema)

			for _, h := range schema.Handlers {
				cr.handlerMap[h] = &cr.schemas[len(cr.schemas)-1]
			}

			if delim != "" {
				heredocDelimiter = delim
			}
			continue
		}

		if strings.HasPrefix(line, "config path ") {
			cr.configPath = strings.TrimPrefix(line, "config path ")
			continue
		}
	}

	return nil
}

// parseSchemaLine parses "config schema <module> handlers <h1,h2,...> [yang <<EOF]".
func parseSchemaLine(line string) (*SchemaInfo, string, error) {
	rest := strings.TrimPrefix(line, "config schema ")
	parts := strings.Fields(rest)
	if len(parts) < 3 {
		return nil, "", fmt.Errorf("expected 'config schema <module> handlers <list>'")
	}

	schema := &SchemaInfo{Module: parts[0]}

	handlersIdx := -1
	for i, p := range parts {
		if p == "handlers" {
			handlersIdx = i
			break
		}
	}
	if handlersIdx < 0 || handlersIdx+1 >= len(parts) {
		return nil, "", fmt.Errorf("expected 'handlers <list>'")
	}

	schema.Handlers = strings.Split(parts[handlersIdx+1], ",")

	var heredocDelim string
	for i, p := range parts {
		if p == "yang" && i+1 < len(parts) && strings.HasPrefix(parts[i+1], "<<") {
			heredocDelim = strings.TrimPrefix(parts[i+1], "<<")
			break
		}
	}

	return schema, heredocDelim, nil
}

// loadInitialConfig parses config and applies all blocks.
func (cr *ConfigReader) loadInitialConfig() error {
	newState, err := cr.parseConfig()
	if err != nil {
		return err
	}

	// Initial load: all blocks are "create".
	changes := cr.diffConfig(NewConfigState(), newState)

	// Send all commands, then commit each namespace.
	if err := cr.applyChanges(changes); err != nil {
		return err
	}

	cr.currentState = newState
	return nil
}

// parseConfig parses the config file into a ConfigState.
func (cr *ConfigReader) parseConfig() (*ConfigState, error) {
	if cr.configPath == "" {
		return nil, fmt.Errorf("no config path specified")
	}

	content, err := os.ReadFile(cr.configPath)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	tokenizer := config.NewTokenizer(string(content))
	tokens := tokenizer.All()

	state := NewConfigState()
	if err := cr.parseBlocks(tokens, "", state); err != nil {
		return nil, err
	}

	return state, nil
}

// parseBlocks recursively parses config blocks into state.
func (cr *ConfigReader) parseBlocks(tokens []config.Token, pathPrefix string, state *ConfigState) error {
	i := 0
	for i < len(tokens) {
		tok := tokens[i]

		if tok.Type != config.TokenWord {
			i++
			continue
		}

		blockName := tok.Value
		i++

		// Check for list key (identifier after block name).
		var listKey string
		if i < len(tokens) && tokens[i].Type == config.TokenWord {
			listKey = tokens[i].Value
			i++
		}

		// Build handler path (without key).
		var handler string
		if pathPrefix != "" {
			// Extract handler from prefix (strip any existing key).
			basePrefix := pathPrefix
			if idx := strings.Index(basePrefix, "["); idx >= 0 {
				basePrefix = basePrefix[:idx]
			}
			handler = basePrefix + "." + blockName
		} else {
			handler = blockName
		}

		// Build full path with key.
		var path string
		if listKey != "" {
			path = fmt.Sprintf("%s[key=%s]", handler, listKey)
		} else {
			path = handler
			listKey = "_default" // Use default key for non-list items.
		}

		// Find block content.
		if i < len(tokens) && tokens[i].Type == config.TokenLBrace {
			i++ // Skip {
			braceCount := 1
			blockStart := i

			for i < len(tokens) && braceCount > 0 {
				//nolint:exhaustive // only counting braces
				switch tokens[i].Type {
				case config.TokenLBrace:
					braceCount++
				case config.TokenRBrace:
					braceCount--
				}
				i++
			}
			blockEnd := i - 1
			blockTokens := tokens[blockStart:blockEnd]

			// Check if we have a handler for this path.
			if cr.findHandler(handler) != nil {
				data := tokensToJSON(blockTokens)
				state.Set(&ConfigBlock{
					Handler: handler,
					Key:     listKey,
					Path:    path,
					Data:    data,
				})
			}

			// Recurse into nested blocks.
			if err := cr.parseBlocks(blockTokens, path, state); err != nil {
				return err
			}
		} else if i < len(tokens) && tokens[i].Type == config.TokenSemicolon {
			i++
		}
	}

	return nil
}

// diffConfig compares old and new config states, returning changes.
func (cr *ConfigReader) diffConfig(oldState, newState *ConfigState) []ConfigChange {
	var changes []ConfigChange

	// Collect all handlers from both states.
	handlers := make(map[string]bool)
	for h := range oldState.blocks {
		handlers[h] = true
	}
	for h := range newState.blocks {
		handlers[h] = true
	}

	// Sort handlers for deterministic order.
	sortedHandlers := make([]string, 0, len(handlers))
	for h := range handlers {
		sortedHandlers = append(sortedHandlers, h)
	}
	sort.Strings(sortedHandlers)

	for _, handler := range sortedHandlers {
		oldBlocks := oldState.blocks[handler]
		newBlocks := newState.blocks[handler]

		if oldBlocks == nil {
			oldBlocks = make(map[string]*ConfigBlock)
		}
		if newBlocks == nil {
			newBlocks = make(map[string]*ConfigBlock)
		}

		// Collect all keys.
		keys := make(map[string]bool)
		for k := range oldBlocks {
			keys[k] = true
		}
		for k := range newBlocks {
			keys[k] = true
		}

		// Sort keys for deterministic order.
		sortedKeys := make([]string, 0, len(keys))
		for k := range keys {
			sortedKeys = append(sortedKeys, k)
		}
		sort.Strings(sortedKeys)

		for _, key := range sortedKeys {
			oldBlock := oldBlocks[key]
			newBlock := newBlocks[key]

			switch {
			case oldBlock == nil && newBlock != nil:
				// Create.
				changes = append(changes, ConfigChange{
					Action:  "create",
					Handler: handler,
					Path:    newBlock.Path,
					NewData: newBlock.Data,
				})
			case oldBlock != nil && newBlock == nil:
				// Delete.
				changes = append(changes, ConfigChange{
					Action:  "delete",
					Handler: handler,
					Path:    oldBlock.Path,
					OldData: oldBlock.Data,
				})
			case oldBlock != nil && newBlock != nil && oldBlock.Data != newBlock.Data:
				// Modify.
				changes = append(changes, ConfigChange{
					Action:  "modify",
					Handler: handler,
					Path:    newBlock.Path,
					OldData: oldBlock.Data,
					NewData: newBlock.Data,
				})
			}
		}
	}

	return changes
}

// findHandler finds the handler for a given path using longest prefix match.
func (cr *ConfigReader) findHandler(path string) *SchemaInfo {
	// Try exact match.
	if schema, ok := cr.handlerMap[path]; ok {
		return schema
	}

	// Extract base path (without list key).
	basePath := path
	if idx := strings.Index(path, "["); idx >= 0 {
		basePath = path[:idx]
	}

	if schema, ok := cr.handlerMap[basePath]; ok {
		return schema
	}

	// Try progressively shorter prefixes.
	parts := strings.Split(basePath, ".")
	for i := len(parts) - 1; i > 0; i-- {
		prefix := strings.Join(parts[:i], ".")
		if schema, ok := cr.handlerMap[prefix]; ok {
			return schema
		}
	}

	return nil
}

// tokensToJSON converts block tokens to JSON data.
// Preserves numeric types where possible.
func tokensToJSON(tokens []config.Token) string {
	data := make(map[string]any)

	i := 0
	for i < len(tokens) {
		if tokens[i].Type == config.TokenWord {
			key := tokens[i].Value
			i++

			if i < len(tokens) {
				//nolint:exhaustive // default case handles remaining types
				switch tokens[i].Type {
				case config.TokenWord, config.TokenString:
					data[key] = parseValue(tokens[i].Value)
					i++
				case config.TokenSemicolon:
					data[key] = true
					i++
				case config.TokenLBrace:
					// Skip nested block.
					i++
					braceCount := 1
					for i < len(tokens) && braceCount > 0 {
						switch tokens[i].Type {
						case config.TokenLBrace:
							braceCount++
						case config.TokenRBrace:
							braceCount--
						}
						i++
					}
				default:
					i++
				}
			}
		} else {
			i++
		}

		if i < len(tokens) && tokens[i].Type == config.TokenSemicolon {
			i++
		}
	}

	result, _ := json.Marshal(data)
	return string(result)
}

// parseValue converts a string value to appropriate type.
// Returns int64 for integers, float64 for floats, bool for true/false, string otherwise.
func parseValue(s string) any {
	// Check for boolean.
	if s == "true" {
		return true
	}
	if s == "false" {
		return false
	}

	// Check for integer.
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return i
	}

	// Check for float.
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}

	// Return as string.
	return s
}

// applyChanges sends all config commands and commits each namespace.
func (cr *ConfigReader) applyChanges(changes []ConfigChange) error {
	if len(changes) == 0 {
		return nil
	}

	// Track namespaces that need commit.
	namespaceSet := make(map[string]bool)

	// Send all config commands.
	for _, change := range changes {
		if err := cr.sendCommand(change); err != nil {
			return fmt.Errorf("command %s: %w", change.Path, err)
		}
		namespace := extractNamespace(change.Handler)
		namespaceSet[namespace] = true
	}

	// Sort namespaces for deterministic commit order.
	namespaces := make([]string, 0, len(namespaceSet))
	for ns := range namespaceSet {
		namespaces = append(namespaces, ns)
	}
	sort.Strings(namespaces)

	// Commit each namespace in sorted order.
	for _, namespace := range namespaces {
		if err := cr.sendCommit(namespace); err != nil {
			return fmt.Errorf("commit %s: %w", namespace, err)
		}
	}

	return nil
}

// extractNamespace extracts the namespace from a handler path.
// "bgp.peer" → "bgp".
func extractNamespace(handler string) string {
	if idx := strings.Index(handler, "."); idx >= 0 {
		return handler[:idx]
	}
	return handler
}

// extractPath extracts the path from a handler.
// "bgp.peer" → "peer".
func extractPath(handler string) string {
	if idx := strings.Index(handler, "."); idx >= 0 {
		return handler[idx+1:]
	}
	return ""
}

// sendCommand sends a namespace command to Hub.
// Format: #N <namespace> <path> <action> {json}.
func (cr *ConfigReader) sendCommand(change ConfigChange) error {
	namespace := extractNamespace(change.Handler)
	path := extractPath(change.Handler)

	data := change.NewData
	if change.Action == "delete" {
		data = change.OldData // Send old data for delete so handler knows what to delete.
		if data == "" {
			data = "{}"
		}
	}

	var msg string
	if path == "" {
		// Handler is just the namespace (e.g., "bgp").
		msg = fmt.Sprintf("#%d %s %s %s\n", cr.serial, namespace, change.Action, data)
	} else {
		msg = fmt.Sprintf("#%d %s %s %s %s\n", cr.serial, namespace, path, change.Action, data)
	}

	if _, err := cr.writer.WriteString(msg); err != nil {
		return err
	}
	if err := cr.writer.Flush(); err != nil {
		return err
	}

	// Wait for response.
	resp, err := cr.waitResponse()
	if err != nil {
		return err
	}
	if resp != "ok" {
		return fmt.Errorf("command rejected: %s", resp)
	}

	return nil
}

// sendCommit sends a commit command to Hub.
// Format: #N <namespace> commit.
func (cr *ConfigReader) sendCommit(namespace string) error {
	msg := fmt.Sprintf("#%d %s commit\n", cr.serial, namespace)

	if _, err := cr.writer.WriteString(msg); err != nil {
		return err
	}
	if err := cr.writer.Flush(); err != nil {
		return err
	}

	// Wait for response.
	resp, err := cr.waitResponse()
	if err != nil {
		return err
	}
	if resp != "ok" {
		return fmt.Errorf("commit rejected: %s", resp)
	}

	return nil
}

// waitResponse waits for a response from Hub.
func (cr *ConfigReader) waitResponse() (string, error) {
	line, err := cr.reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	line = strings.TrimSuffix(line, "\n")

	expectedPrefix := fmt.Sprintf("@%d ", cr.serial)
	if !strings.HasPrefix(line, expectedPrefix) {
		return "", fmt.Errorf("unexpected response: %s", line)
	}

	cr.serial++
	rest := strings.TrimPrefix(line, expectedPrefix)

	if strings.HasPrefix(rest, "error ") {
		return strings.TrimPrefix(rest, "error "), nil
	}

	return "ok", nil
}

// sendComplete signals config parsing is complete.
func (cr *ConfigReader) sendComplete() error {
	_, err := cr.writer.WriteString("#0 config complete\n")
	if err != nil {
		return err
	}
	return cr.writer.Flush()
}

// eventLoop handles reload and shutdown requests.
func (cr *ConfigReader) eventLoop() error {
	for {
		line, err := cr.reader.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		line = strings.TrimSuffix(line, "\n")

		if !strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line[1:], " ", 2)
		if len(parts) < 2 {
			continue
		}
		serial := parts[0]
		command := parts[1]

		switch {
		case command == "config reload":
			if err := cr.handleReload(); err != nil {
				cr.sendError(serial, err.Error())
			} else {
				cr.sendDone(serial)
			}
		case strings.HasPrefix(command, "shutdown"):
			return nil
		default:
			cr.sendError(serial, "unknown command")
		}
	}
}

// handleReload handles a config reload request with proper diffing.
func (cr *ConfigReader) handleReload() error {
	// Parse new config.
	newState, err := cr.parseConfig()
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}

	// Compute diff.
	changes := cr.diffConfig(cr.currentState, newState)

	if len(changes) == 0 {
		return nil // No changes.
	}

	// Apply all changes.
	if err := cr.applyChanges(changes); err != nil {
		return err
	}

	// Update state on success.
	cr.currentState = newState
	return nil
}

// sendDone sends a done response.
func (cr *ConfigReader) sendDone(serial string) {
	_, _ = fmt.Fprintf(cr.writer, "@%s done\n", serial)
	_ = cr.writer.Flush()
}

// sendError sends an error response.
func (cr *ConfigReader) sendError(serial, msg string) {
	_, _ = fmt.Fprintf(cr.writer, "@%s error %s\n", serial, msg)
	_ = cr.writer.Flush()
}

func main() {
	flag.Parse()

	cr := NewConfigReader()
	if err := cr.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
