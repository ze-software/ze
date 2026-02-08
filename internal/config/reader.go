package config

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

// SchemaInfo holds schema information for handler routing.
type SchemaInfo struct {
	Module    string   // YANG module name
	Namespace string   // YANG namespace URI (optional)
	Handlers  []string // Handler paths this schema provides
	Yang      string   // YANG content (optional)
}

// BlockEntry represents a parsed config block stored by the reader.
type BlockEntry struct {
	Handler string // Handler path (e.g., "bgp.peer")
	Key     string // List key (e.g., "192.0.2.1" for peer address)
	Path    string // Full path with key (e.g., "bgp.peer[key=192.0.2.1]")
	Data    string // JSON data
}

// BlockState stores the current configuration as handler → key → data.
type BlockState struct {
	blocks map[string]map[string]*BlockEntry // handler → key → block
}

// NewBlockState creates an empty block state.
func NewBlockState() *BlockState {
	return &BlockState{
		blocks: make(map[string]map[string]*BlockEntry),
	}
}

// Set adds or updates a block.
func (bs *BlockState) Set(block *BlockEntry) {
	if bs.blocks[block.Handler] == nil {
		bs.blocks[block.Handler] = make(map[string]*BlockEntry)
	}
	bs.blocks[block.Handler][block.Key] = block
}

// Get returns a block by handler and key.
func (bs *BlockState) Get(handler, key string) *BlockEntry {
	if bs.blocks[handler] == nil {
		return nil
	}
	return bs.blocks[handler][key]
}

// BlockChange represents a change between old and new config.
type BlockChange struct {
	Action  string // "create", "modify", "delete"
	Handler string // Handler path
	Path    string // Full path with key
	OldData string // Previous data (for modify/delete)
	NewData string // New data (for create/modify)
}

// ConfigValidator validates config data against a schema.
type ConfigValidator interface {
	ValidateContainer(path string, data map[string]any) error
}

// ConfigFrontend parses raw config content into a nested map.
// Both frontends (tokenizer and set-parser) produce the same structural form:
// a nested map[string]any with containers as sub-maps and lists as
// maps of key → sub-map.
type ConfigFrontend interface {
	ParseConfig(content string) (map[string]any, error)
}

// TokenizerFrontend parses Ze/Junos-style config into a nested map.
type TokenizerFrontend struct{}

// ParseConfig tokenizes the content and produces a nested map.
func (f *TokenizerFrontend) ParseConfig(content string) (map[string]any, error) {
	tokenizer := NewTokenizer(content)
	tokens := tokenizer.All()
	return tokensToNestedMap(tokens), nil
}

// SetParserFrontend parses set-style config into a nested map.
type SetParserFrontend struct {
	Schema *Schema
}

// ParseConfig parses set commands into a Tree and converts to a map.
// String leaf values are converted to typed values (int64, float64, bool)
// using parseConfigValue so both frontends produce compatible maps.
func (f *SetParserFrontend) ParseConfig(content string) (map[string]any, error) {
	parser := NewSetParser(f.Schema)
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
func tokensToNestedMap(tokens []Token) map[string]any {
	result := make(map[string]any)
	i := 0
	for i < len(tokens) {
		if tokens[i].Type != TokenWord {
			i++
			continue
		}

		key := tokens[i].Value
		i++
		if i >= len(tokens) {
			break
		}

		//nolint:exhaustive // structural tokens (RBrace, brackets, parens, EOF) at value position are skipped
		switch tokens[i].Type {
		case TokenWord, TokenString:
			// Could be "key value ;" or "key listkey { ... }"
			value := tokens[i].Value
			i++

			if i < len(tokens) && tokens[i].Type == TokenLBrace {
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

		case TokenLBrace:
			// Container: key { ... }
			i++ // skip {
			innerTokens := extractBraceContent(tokens, &i)
			innerMap := tokensToNestedMap(innerTokens)

			// Merge if container already exists (repeated blocks).
			existing, _ := result[key].(map[string]any)
			if existing != nil {
				for k, v := range innerMap {
					existing[k] = v
				}
			} else {
				result[key] = innerMap
			}

		case TokenSemicolon:
			// Flag: key;
			result[key] = true
			i++
			continue // already consumed semicolon

		case TokenRBrace, TokenEOF, TokenLBracket, TokenRBracket, TokenLParen, TokenRParen:
			// Structural tokens at value position — not a key-value pair, skip.
			i++
		}

		// Skip trailing semicolon.
		if i < len(tokens) && tokens[i].Type == TokenSemicolon {
			i++
		}
	}
	return result
}

// extractBraceContent returns tokens between matching braces.
// On entry, tokens[*pos] is the first token after the opening brace.
// On exit, *pos points past the closing brace.
func extractBraceContent(tokens []Token, pos *int) []Token {
	start := *pos
	depth := 1
	for *pos < len(tokens) && depth > 0 {
		//nolint:exhaustive // only counting braces
		switch tokens[*pos].Type {
		case TokenLBrace:
			depth++
		case TokenRBrace:
			depth--
		}
		(*pos)++
	}
	if *pos > start {
		return tokens[start : *pos-1]
	}
	return nil
}

// Reader parses config files and maps blocks to handlers.
type Reader struct {
	configPath string
	handlerMap map[string]*SchemaInfo
	current    *BlockState
	validator  ConfigValidator
	frontend   ConfigFrontend
}

// NewReader creates a new config reader with the given schemas, config path,
// optional YANG validator, and optional frontend parser.
// If validator is nil, validation is skipped.
// If frontend is nil, TokenizerFrontend is used.
func NewReader(schemas []SchemaInfo, configPath string, validator ConfigValidator, frontend ConfigFrontend) *Reader {
	if frontend == nil {
		frontend = &TokenizerFrontend{}
	}
	r := &Reader{
		configPath: configPath,
		handlerMap: make(map[string]*SchemaInfo),
		current:    NewBlockState(),
		validator:  validator,
		frontend:   frontend,
	}
	for i := range schemas {
		for _, h := range schemas[i].Handlers {
			r.handlerMap[h] = &schemas[i]
		}
	}
	return r
}

// Load parses the config file and returns the initial state.
func (r *Reader) Load() (*BlockState, error) {
	state, err := r.parseConfig()
	if err != nil {
		return nil, err
	}
	r.current = state
	return state, nil
}

// Reload re-parses the config file, diffs against the current state,
// and returns the list of changes. Updates the internal state on success.
func (r *Reader) Reload() ([]BlockChange, error) {
	newState, err := r.parseConfig()
	if err != nil {
		return nil, err
	}
	changes := DiffBlocks(r.current, newState)
	r.current = newState
	return changes, nil
}

// parseConfig parses the config file into a BlockState using the frontend.
func (r *Reader) parseConfig() (*BlockState, error) {
	if r.configPath == "" {
		return nil, fmt.Errorf("no config path specified")
	}

	content, err := os.ReadFile(r.configPath) //nolint:gosec // Config file path from caller
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	data, err := r.frontend.ParseConfig(string(content))
	if err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	state := NewBlockState()
	if err := r.walkMap(data, "", state); err != nil {
		return nil, err
	}

	return state, nil
}

// walkMap recursively walks a nested config map, routing blocks to handlers
// and applying YANG validation. For each handler match, flat fields (non-map
// values) are extracted, validated, and stored as a BlockEntry.
func (r *Reader) walkMap(data map[string]any, pathPrefix string, state *BlockState) error {
	for blockName, blockValue := range data {
		subMap, ok := blockValue.(map[string]any)
		if !ok {
			continue // leaf values handled by parent
		}

		// Build handler path.
		handler := blockName
		if pathPrefix != "" {
			basePrefix, _, _ := strings.Cut(pathPrefix, "[")
			handler = basePrefix + "." + blockName
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

			// Store as BlockEntry.
			jsonData, _ := json.Marshal(flatData)
			state.Set(&BlockEntry{
				Handler: handler,
				Key:     "_default",
				Path:    handler,
				Data:    string(jsonData),
			})

			// Process nested sub-maps (lists and containers).
			for subName, subValue := range subMap {
				nestedMap, ok := subValue.(map[string]any)
				if !ok {
					continue
				}

				subHandler := handler + "." + subName

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
func (r *Reader) walkListEntries(handler string, entries map[string]any, state *BlockState) error {
	for listKey, entryValue := range entries {
		entryMap, ok := entryValue.(map[string]any)
		if !ok {
			continue
		}

		entryPath := fmt.Sprintf("%s[key=%s]", handler, listKey)

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
		state.Set(&BlockEntry{
			Handler: handler,
			Key:     listKey,
			Path:    entryPath,
			Data:    string(jsonData),
		})

		// Recurse into list entry for deeper handlers.
		if err := r.walkMap(entryMap, entryPath, state); err != nil {
			return err
		}
	}

	return nil
}

// findHandler finds the handler for a given path using longest prefix match.
func (r *Reader) findHandler(path string) *SchemaInfo {
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
	parts := strings.Split(basePath, ".")
	for i := len(parts) - 1; i > 0; i-- {
		prefix := strings.Join(parts[:i], ".")
		if schema, ok := r.handlerMap[prefix]; ok {
			return schema
		}
	}

	return nil
}

// parseConfigValue converts a string value to appropriate type.
// Returns int64 for integers, float64 for floats, bool for true/false, string otherwise.
func parseConfigValue(s string) any {
	if s == "true" {
		return true
	}
	if s == "false" {
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

// DiffBlocks compares old and new block states, returning changes.
// Changes are sorted deterministically by handler then key.
func DiffBlocks(oldState, newState *BlockState) []BlockChange {
	var changes []BlockChange

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
			oldBlocks = make(map[string]*BlockEntry)
		}
		if newBlocks == nil {
			newBlocks = make(map[string]*BlockEntry)
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
				changes = append(changes, BlockChange{
					Action:  "create",
					Handler: handler,
					Path:    newBlock.Path,
					NewData: newBlock.Data,
				})
			case oldBlock != nil && newBlock == nil:
				changes = append(changes, BlockChange{
					Action:  "delete",
					Handler: handler,
					Path:    oldBlock.Path,
					OldData: oldBlock.Data,
				})
			case oldBlock != nil && newBlock != nil && oldBlock.Data != newBlock.Data:
				changes = append(changes, BlockChange{
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
