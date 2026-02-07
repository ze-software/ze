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

// Reader parses config files and maps blocks to handlers.
type Reader struct {
	configPath string
	handlerMap map[string]*SchemaInfo
	current    *BlockState
	validator  ConfigValidator
}

// NewReader creates a new config reader with the given schemas, config path,
// and optional YANG validator. If validator is nil, validation is skipped.
func NewReader(schemas []SchemaInfo, configPath string, validator ConfigValidator) *Reader {
	r := &Reader{
		configPath: configPath,
		handlerMap: make(map[string]*SchemaInfo),
		current:    NewBlockState(),
		validator:  validator,
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

// parseConfig parses the config file into a BlockState.
func (r *Reader) parseConfig() (*BlockState, error) {
	if r.configPath == "" {
		return nil, fmt.Errorf("no config path specified")
	}

	content, err := os.ReadFile(r.configPath) //nolint:gosec // Config file path from caller
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	tokenizer := NewTokenizer(string(content))
	tokens := tokenizer.All()

	state := NewBlockState()
	if err := r.parseBlocks(tokens, "", state); err != nil {
		return nil, err
	}

	return state, nil
}

// parseBlocks recursively parses config blocks into state.
func (r *Reader) parseBlocks(tokens []Token, pathPrefix string, state *BlockState) error {
	i := 0
	for i < len(tokens) {
		tok := tokens[i]

		if tok.Type != TokenWord {
			i++
			continue
		}

		blockName := tok.Value
		i++

		// Check for list key (identifier after block name).
		var listKey string
		if i < len(tokens) && tokens[i].Type == TokenWord {
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
		if i < len(tokens) && tokens[i].Type == TokenLBrace {
			i++ // Skip {
			braceCount := 1
			blockStart := i

			for i < len(tokens) && braceCount > 0 {
				//nolint:exhaustive // only counting braces
				switch tokens[i].Type {
				case TokenLBrace:
					braceCount++
				case TokenRBrace:
					braceCount--
				}
				i++
			}
			blockEnd := i - 1
			blockTokens := tokens[blockStart:blockEnd]

			// Check if we have a handler for this path.
			if r.findHandler(handler) != nil {
				data := TokensToJSON(blockTokens)

				// Validate against YANG schema if validator is provided.
				if r.validator != nil {
					var dataMap map[string]any
					if err := json.Unmarshal([]byte(data), &dataMap); err != nil {
						return fmt.Errorf("unmarshal config for %s: %w", handler, err)
					}
					if err := r.validator.ValidateContainer(handler, dataMap); err != nil {
						return fmt.Errorf("validate %s: %w", handler, err)
					}
				}

				state.Set(&BlockEntry{
					Handler: handler,
					Key:     listKey,
					Path:    path,
					Data:    data,
				})
			}

			// Recurse into nested blocks only if sub-blocks exist.
			for _, bt := range blockTokens {
				if bt.Type == TokenLBrace {
					if err := r.parseBlocks(blockTokens, path, state); err != nil {
						return err
					}
					break
				}
			}
		} else if i < len(tokens) && tokens[i].Type == TokenSemicolon {
			i++
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
	basePath := path
	if idx := strings.Index(path, "["); idx >= 0 {
		basePath = path[:idx]
	}

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

// TokensToJSON converts block tokens to a JSON string of key-value pairs.
// Preserves numeric types where possible.
// Tokens represent flat key-value pairs at the current block level;
// nested blocks are skipped (handled by recursive parseBlocks).
func TokensToJSON(tokens []Token) string {
	data := make(map[string]any)

	i := 0
	for i < len(tokens) {
		if tokens[i].Type == TokenWord {
			key := tokens[i].Value
			i++

			if i < len(tokens) {
				//nolint:exhaustive // TokenLBrace/TokenRBrace/EOF handled explicitly or skipped
				switch tokens[i].Type {
				case TokenWord, TokenString:
					data[key] = parseConfigValue(tokens[i].Value)
					i++
				case TokenSemicolon:
					data[key] = true
					i++
				case TokenLBrace:
					// Skip nested block — handled by recursive parseBlocks.
					i++
					braceCount := 1
					for i < len(tokens) && braceCount > 0 {
						switch tokens[i].Type {
						case TokenLBrace:
							braceCount++
						case TokenRBrace:
							braceCount--
						}
						i++
					}
				case TokenRBrace, TokenEOF, TokenLBracket, TokenRBracket, TokenLParen, TokenRParen:
					// Structural tokens at value position — skip, not a key-value pair.
					i++
				}
			}
		} else {
			i++
		}

		if i < len(tokens) && tokens[i].Type == TokenSemicolon {
			i++
		}
	}

	result, _ := json.Marshal(data)
	return string(result)
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
