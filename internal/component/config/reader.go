// Design: docs/architecture/config/syntax.md — config parsing and loading
// Related: parser.go — config parser core

package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"sort"
	"strconv"
	"strings"
)

var errNoConfigPathSpecified = errors.New("no config path specified")

// schemaInfo holds schema information for handler routing.
type schemaInfo struct {
	module   string   // YANG module name
	handlers []string // Handler paths this schema provides
}

// blockEntry represents a parsed config block stored by the reader.
type blockEntry struct {
	handler string // Handler path (e.g., "bgp/peer")
	key     string // List key (e.g., "192.0.2.1" for peer address)
	path    string // Full path with key (e.g., "bgp/peer[key=192.0.2.1]")
	data    string // JSON data
}

// blockState stores the current configuration as handler → key → data.
type blockState struct {
	blocks map[string]map[string]*blockEntry // handler → key → block
}

// newBlockState creates an empty block state.
func newBlockState() *blockState {
	return &blockState{
		blocks: make(map[string]map[string]*blockEntry),
	}
}

// set adds or updates a block.
func (bs *blockState) set(block *blockEntry) {
	if bs.blocks[block.handler] == nil {
		bs.blocks[block.handler] = make(map[string]*blockEntry)
	}
	bs.blocks[block.handler][block.key] = block
}

// get returns a block by handler and key.
func (bs *blockState) get(handler, key string) *blockEntry {
	if bs.blocks[handler] == nil {
		return nil
	}
	return bs.blocks[handler][key]
}

// blockChange represents a change between old and new config.
type blockChange struct {
	action  string // "create", "modify", "delete"
	handler string // Handler path
	path    string // Full path with key
	oldData string // Previous data (for modify/delete)
	newData string // New data (for create/modify)
}

// configValidator validates config data against a schema.
type configValidator interface {
	ValidateContainer(path string, data map[string]any) error
}

// configFrontend parses raw config content into a nested map.
// Both frontends (tokenizer and set-parser) produce the same structural form:
// a nested map[string]any with containers as sub-maps and lists as
// maps of key → sub-map.
type configFrontend interface {
	parseConfigContent(content string) (map[string]any, error)
}

// tokenizerFrontend parses Ze/Junos-style config into a nested map.
type tokenizerFrontend struct{}

// parseConfigContent tokenizes the content and produces a nested map.
func (f *tokenizerFrontend) parseConfigContent(content string) (map[string]any, error) {
	tokenizer := newTokenizer(content)
	tokens := tokenizer.all()
	return tokensToNestedMap(tokens), nil
}

// setParserFrontend parses set-style config into a nested map.
type setParserFrontend struct {
	schema *Schema
}

// parseConfigContent parses set commands into a Tree and converts to a map.
// String leaf values are converted to typed values (int64, float64, bool)
// using parseConfigValue so both frontends produce compatible maps.
func (f *setParserFrontend) parseConfigContent(content string) (map[string]any, error) {
	parser := NewSetParser(f.schema)
	tree, err := parser.Parse(content)
	if err != nil {
		return nil, err
	}
	result := tree.ToMap()
	convertStringValues(result)
	return result, nil
}

// convertStringValues recursively converts string leaf values to typed values.
// This ensures SetParser output matches tokenizer output types.
func convertStringValues(m map[string]any) {
	for k, v := range m {
		switch val := v.(type) {
		case string:
			m[k] = parseConfigValue(val)
		case map[string]any:
			convertStringValues(val)
		}
	}
}

// tokensToNestedMap converts tokens into a nested map.
// Handles flat key-value pairs, containers, and list entries.
func tokensToNestedMap(tokens []token) map[string]any {
	result := make(map[string]any)
	i := 0
	for i < len(tokens) {
		if tokens[i].kind != tokenWord {
			i++
			continue
		}

		key := tokens[i].value
		i++
		if i >= len(tokens) {
			break
		}

		//nolint:exhaustive // structural tokens (RBrace, brackets, parens, EOF) at value position are skipped
		switch tokens[i].kind {
		case tokenWord, tokenString:
			// Could be "key value ;" or "key listkey { ... }"
			value := tokens[i].value
			i++

			if i < len(tokens) && tokens[i].kind == tokenLBrace {
				// List entry: key listkey { ... }
				i++ // skip {
				innerTokens := extractBraceContent(tokens, &i)
				innerMap := tokensToNestedMap(innerTokens)

				listMap, _ := result[key].(map[string]any)
				if listMap == nil {
					listMap = make(map[string]any)
					result[key] = listMap
				}
				listMap[value] = innerMap
			} else {
				// Simple leaf: key value
				result[key] = parseConfigValue(value)
			}

		case tokenLBrace:
			// Container: key { ... }
			i++ // skip {
			innerTokens := extractBraceContent(tokens, &i)
			innerMap := tokensToNestedMap(innerTokens)

			// Merge if container already exists (repeated blocks).
			existing, _ := result[key].(map[string]any)
			if existing != nil {
				maps.Copy(existing, innerMap)
			} else {
				result[key] = innerMap
			}

		case tokenSemicolon:
			// Flag: key;
			result[key] = true
			i++
			continue // already consumed semicolon

		case tokenRBrace, tokenEOF, tokenLBracket, tokenRBracket, tokenLParen, tokenRParen:
			// Structural tokens at value position — not a key-value pair, skip.
			i++
		}

		// Skip trailing semicolon.
		if i < len(tokens) && tokens[i].kind == tokenSemicolon {
			i++
		}
	}
	return result
}

// extractBraceContent returns tokens between matching braces.
// On entry, tokens[*pos] is the first token after the opening brace.
// On exit, *pos points past the closing brace.
func extractBraceContent(tokens []token, pos *int) []token {
	start := *pos
	depth := 1
	for *pos < len(tokens) && depth > 0 {
		//nolint:exhaustive // only counting braces
		switch tokens[*pos].kind {
		case tokenLBrace:
			depth++
		case tokenRBrace:
			depth--
		}
		(*pos)++
	}
	if *pos > start {
		return tokens[start : *pos-1]
	}
	return nil
}

// reader parses config files and maps blocks to handlers.
type reader struct {
	configPath string
	handlerMap map[string]*schemaInfo
	current    *blockState
	validator  configValidator
	frontend   configFrontend
}

// newReader creates a new config reader with the given schemas, config path,
// optional YANG validator, and optional frontend parser.
// If validator is nil, validation is skipped.
// If frontend is nil, tokenizerFrontend is used.
func newReader(schemas []schemaInfo, configPath string, validator configValidator, frontend configFrontend) *reader {
	if frontend == nil {
		frontend = &tokenizerFrontend{}
	}
	r := &reader{
		configPath: configPath,
		handlerMap: make(map[string]*schemaInfo),
		current:    newBlockState(),
		validator:  validator,
		frontend:   frontend,
	}
	for i := range schemas {
		for _, h := range schemas[i].handlers {
			r.handlerMap[h] = &schemas[i]
		}
	}
	return r
}

// load parses the config file and returns the initial state.
func (r *reader) load() (*blockState, error) {
	state, err := r.parseConfig()
	if err != nil {
		return nil, err
	}
	r.current = state
	return state, nil
}

// reload re-parses the config file, diffs against the current state,
// and returns the list of changes. Updates the internal state on success.
func (r *reader) reload() ([]blockChange, error) {
	newState, err := r.parseConfig()
	if err != nil {
		return nil, err
	}
	changes := diffBlocks(r.current, newState)
	r.current = newState
	return changes, nil
}

// parseConfig parses the config file into a blockState using the frontend.
func (r *reader) parseConfig() (*blockState, error) {
	if r.configPath == "" {
		return nil, errNoConfigPathSpecified
	}

	content, err := os.ReadFile(r.configPath) //nolint:gosec // Config file path from caller
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	data, err := r.frontend.parseConfigContent(string(content))
	if err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	state := newBlockState()
	if err := r.walkMap(data, "", state); err != nil {
		return nil, err
	}

	return state, nil
}

// walkMap recursively walks a nested config map, routing blocks to handlers
// and applying YANG validation. For each handler match, flat fields (non-map
// values) are extracted, validated, and stored as a blockEntry.
func (r *reader) walkMap(data map[string]any, pathPrefix string, state *blockState) error {
	for blockName, blockValue := range data {
		subMap, ok := blockValue.(map[string]any)
		if !ok {
			continue // leaf values handled by parent
		}

		// Build handler path.
		handler := blockName
		if pathPrefix != "" {
			basePrefix, _, _ := strings.Cut(pathPrefix, "[")
			handler = AppendPath(basePrefix, blockName)
		}

		if r.findHandler(handler) != nil {
			// Separate flat fields from nested sub-maps.
			flatData := make(map[string]any)
			for k, v := range subMap {
				if _, isMap := v.(map[string]any); !isMap {
					flatData[k] = v
				}
			}

			// Validate flat data via YANG.
			if r.validator != nil && len(flatData) > 0 {
				if err := r.validator.ValidateContainer(handler, flatData); err != nil {
					return fmt.Errorf("validate %s: %w", handler, err)
				}
			}

			// Store as blockEntry.
			jsonData, _ := json.Marshal(flatData)
			state.set(&blockEntry{
				handler: handler,
				key:     "_default",
				path:    handler,
				data:    string(jsonData),
			})

			// Process nested sub-maps (lists and containers).
			for subName, subValue := range subMap {
				nestedMap, ok := subValue.(map[string]any)
				if !ok {
					continue
				}

				subHandler := AppendPath(handler, subName)

				if r.findHandler(subHandler) != nil {
					// Check if all values are maps (list entries).
					isList := len(nestedMap) > 0
					for _, v := range nestedMap {
						if _, isMap := v.(map[string]any); !isMap {
							isList = false
							break
						}
					}

					if isList {
						if err := r.walkListEntries(subHandler, nestedMap, state); err != nil {
							return err
						}
					} else {
						if err := r.walkMap(map[string]any{subName: nestedMap}, handler, state); err != nil {
							return err
						}
					}
				} else {
					// No handler — recurse looking for deeper handlers.
					if err := r.walkMap(nestedMap, subHandler, state); err != nil {
						return err
					}
				}
			}
		} else {
			// No handler for this block — recurse.
			if err := r.walkMap(subMap, handler, state); err != nil {
				return err
			}
		}
	}

	return nil
}

// walkListEntries processes list entries where each key maps to a sub-map.
func (r *reader) walkListEntries(handler string, entries map[string]any, state *blockState) error {
	for listKey, entryValue := range entries {
		entryMap, ok := entryValue.(map[string]any)
		if !ok {
			continue
		}

		entryPath := handler + "[key=" + listKey + "]"

		// Extract flat fields.
		flatEntry := make(map[string]any)
		for k, v := range entryMap {
			if _, isMap := v.(map[string]any); !isMap {
				flatEntry[k] = v
			}
		}

		// Validate.
		if r.validator != nil && len(flatEntry) > 0 {
			if err := r.validator.ValidateContainer(handler, flatEntry); err != nil {
				return fmt.Errorf("validate %s: %w", entryPath, err)
			}
		}

		// Store.
		jsonData, _ := json.Marshal(flatEntry)
		state.set(&blockEntry{
			handler: handler,
			key:     listKey,
			path:    entryPath,
			data:    string(jsonData),
		})

		// Recurse into list entry for deeper handlers.
		if err := r.walkMap(entryMap, entryPath, state); err != nil {
			return err
		}
	}

	return nil
}

// findHandler finds the handler for a given path using longest prefix match.
func (r *reader) findHandler(path string) *schemaInfo {
	// Try exact match.
	if schema, ok := r.handlerMap[path]; ok {
		return schema
	}

	// Extract base path (without list key).
	basePath, _, _ := strings.Cut(path, "[")

	if schema, ok := r.handlerMap[basePath]; ok {
		return schema
	}

	// Try progressively shorter prefixes.
	parts := SplitPath(basePath)
	for i := len(parts) - 1; i > 0; i-- {
		prefix := JoinPath(parts[:i]...)
		if schema, ok := r.handlerMap[prefix]; ok {
			return schema
		}
	}

	return nil
}

// parseConfigValue converts a string value to appropriate type.
// Returns int64 for integers, float64 for floats, bool for true/false, string otherwise.
func parseConfigValue(s string) any {
	if s == configTrue {
		return true
	}
	if s == configFalse {
		return false
	}

	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return i
	}

	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}

	return s
}

// diffBlocks compares old and new block states, returning changes.
// Changes are sorted deterministically by handler then key.
func diffBlocks(oldState, newState *blockState) []blockChange {
	var changes []blockChange

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
			oldBlocks = make(map[string]*blockEntry)
		}
		if newBlocks == nil {
			newBlocks = make(map[string]*blockEntry)
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
				changes = append(changes, blockChange{
					action:  "create",
					handler: handler,
					path:    newBlock.path,
					newData: newBlock.data,
				})
			case oldBlock != nil && newBlock == nil:
				changes = append(changes, blockChange{
					action:  "delete",
					handler: handler,
					path:    oldBlock.path,
					oldData: oldBlock.data,
				})
			case oldBlock != nil && newBlock != nil && oldBlock.data != newBlock.data:
				changes = append(changes, blockChange{
					action:  "modify",
					handler: handler,
					path:    newBlock.path,
					oldData: oldBlock.data,
					newData: newBlock.data,
				})
			}
		}
	}

	return changes
}
