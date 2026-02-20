// Design: docs/architecture/hub-architecture.md — hub coordination
//
// Package hub provides the hub/orchestrator process for ze.
//
// Config parsing handles 3-section config format:
//   - env { } - global settings (handled by hub)
//   - plugin { } - process declarations (what to fork)
//   - remaining blocks - plugin configs (stored for routing)

package hub

import (
	"fmt"
	"io"
	"os"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/config"
)

// HubConfig holds hub orchestrator configuration.
// Note: This extends the existing HubConfig in hub.go with config-related fields.

// LoadHubConfig loads configuration from a file.
// If path is "-", reads from stdin.
func LoadHubConfig(path string) (*HubConfig, error) {
	var data []byte
	var err error

	if path == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(path) //nolint:gosec // Config file path from command line
	}
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}
	cfg, err := ParseHubConfig(string(data))
	if err != nil {
		return nil, err
	}
	cfg.ConfigPath = path
	return cfg, nil
}

// ParseHubConfig parses hub configuration from a string.
func ParseHubConfig(input string) (*HubConfig, error) {
	cfg := &HubConfig{
		Env:    make(map[string]string),
		Blocks: make(map[string]any),
	}

	tok := config.NewTokenizer(input)

	for {
		t := tok.Peek()
		if t.Type == config.TokenEOF {
			break
		}

		if t.Type != config.TokenWord {
			return nil, fmt.Errorf("line %d: expected keyword, got %s", t.Line, t.Type)
		}

		blockName := t.Value
		tok.Next() // consume block name

		switch blockName {
		case "env":
			if err := parseEnvBlock(tok, cfg); err != nil {
				return nil, err
			}
		case "plugin":
			if err := parsePluginBlock(tok, cfg); err != nil {
				return nil, err
			}
		default:
			// Store as config block for routing to plugins
			block, err := parseGenericBlock(tok)
			if err != nil {
				return nil, err
			}
			cfg.Blocks[blockName] = block
		}
	}

	return cfg, nil
}

// parseEnvBlock parses env block: env { key value; key value; }.
func parseEnvBlock(tok *config.Tokenizer, cfg *HubConfig) error {
	t := tok.Peek()
	if t.Type != config.TokenLBrace {
		return fmt.Errorf("line %d: expected '{' after env, got %s", t.Line, t.Type)
	}
	tok.Next() // consume {

	for {
		t = tok.Peek()
		if t.Type == config.TokenRBrace {
			tok.Next() // consume }
			break
		}
		if t.Type == config.TokenEOF {
			return fmt.Errorf("line %d: unexpected EOF in env block", t.Line)
		}
		if t.Type != config.TokenWord {
			return fmt.Errorf("line %d: expected keyword in env block, got %s", t.Line, t.Type)
		}

		key := t.Value
		tok.Next() // consume key

		// Get value
		t = tok.Peek()
		if t.Type != config.TokenWord && t.Type != config.TokenString {
			return fmt.Errorf("line %d: expected value for %s, got %s", t.Line, key, t.Type)
		}
		value := t.Value
		tok.Next() // consume value

		// Expect semicolon
		t = tok.Peek()
		if t.Type != config.TokenSemicolon {
			return fmt.Errorf("line %d: expected ';' after %s value, got %s", t.Line, key, t.Type)
		}
		tok.Next() // consume ;

		cfg.Env[key] = value
	}

	return nil
}

// parsePluginBlock parses plugin block: plugin { external NAME { run "..."; } }.
func parsePluginBlock(tok *config.Tokenizer, cfg *HubConfig) error {
	t := tok.Peek()
	if t.Type != config.TokenLBrace {
		return fmt.Errorf("line %d: expected '{' after plugin, got %s", t.Line, t.Type)
	}
	tok.Next() // consume {

	for {
		t = tok.Peek()
		if t.Type == config.TokenRBrace {
			tok.Next() // consume }
			break
		}
		if t.Type == config.TokenEOF {
			return fmt.Errorf("line %d: unexpected EOF in plugin block", t.Line)
		}
		if t.Type != config.TokenWord {
			return fmt.Errorf("line %d: expected 'external' in plugin block, got %s", t.Line, t.Type)
		}

		if t.Value != "external" {
			return fmt.Errorf("line %d: expected 'external', got %s", t.Line, t.Value)
		}
		tok.Next() // consume 'external'

		// Get plugin name
		t = tok.Peek()
		if t.Type != config.TokenWord {
			return fmt.Errorf("line %d: expected plugin name after 'external', got %s", t.Line, t.Type)
		}
		pluginName := t.Value
		tok.Next() // consume name

		// Expect opening brace
		t = tok.Peek()
		if t.Type != config.TokenLBrace {
			return fmt.Errorf("line %d: expected '{' after plugin name, got %s", t.Line, t.Type)
		}
		tok.Next() // consume {

		plugin := PluginDef{Name: pluginName}

		// Parse plugin properties
		for {
			t = tok.Peek()
			if t.Type == config.TokenRBrace {
				tok.Next() // consume }
				break
			}
			if t.Type == config.TokenEOF {
				return fmt.Errorf("line %d: unexpected EOF in external block", t.Line)
			}
			if t.Type != config.TokenWord {
				return fmt.Errorf("line %d: expected property name, got %s", t.Line, t.Type)
			}

			propName := t.Value
			tok.Next() // consume property name

			// Get property value
			t = tok.Peek()
			if t.Type != config.TokenWord && t.Type != config.TokenString {
				return fmt.Errorf("line %d: expected value for %s, got %s", t.Line, propName, t.Type)
			}
			propValue := t.Value
			tok.Next() // consume value

			// Expect semicolon
			t = tok.Peek()
			if t.Type != config.TokenSemicolon {
				return fmt.Errorf("line %d: expected ';' after %s value, got %s", t.Line, propName, t.Type)
			}
			tok.Next() // consume ;

			if propName == "run" {
				plugin.Run = propValue
			}
		}

		cfg.Plugins = append(cfg.Plugins, plugin)
	}

	return nil
}

// parseGenericBlock parses any block and returns its content as nested maps.
// Format: name { key value; nested { ... } }.
func parseGenericBlock(tok *config.Tokenizer) (map[string]any, error) {
	t := tok.Peek()
	if t.Type != config.TokenLBrace {
		return nil, fmt.Errorf("line %d: expected '{', got %s", t.Line, t.Type)
	}
	tok.Next() // consume {

	result := make(map[string]any)

	for {
		t = tok.Peek()
		if t.Type == config.TokenRBrace {
			tok.Next() // consume }
			break
		}
		if t.Type == config.TokenEOF {
			return nil, fmt.Errorf("line %d: unexpected EOF in block", t.Line)
		}
		if t.Type != config.TokenWord {
			return nil, fmt.Errorf("line %d: expected keyword, got %s", t.Line, t.Type)
		}

		key := t.Value
		tok.Next() // consume key

		t = tok.Peek()

		// Check if nested block or value
		switch t.Type { //nolint:exhaustive // Other token types handled by default case
		case config.TokenLBrace:
			// Nested block
			nested, err := parseGenericBlock(tok)
			if err != nil {
				return nil, err
			}
			result[key] = nested

		case config.TokenWord, config.TokenString:
			// Value - collect all values until semicolon
			var values []string
			for t.Type == config.TokenWord || t.Type == config.TokenString {
				values = append(values, t.Value)
				tok.Next()
				t = tok.Peek()

				// Check for nested block after key (e.g., "peer 192.168.1.1 { ... }")
				if t.Type == config.TokenLBrace {
					nested, err := parseGenericBlock(tok)
					if err != nil {
						return nil, err
					}
					// Store as "key value" -> nested
					var fullKey strings.Builder
					fullKey.WriteString(key)
					for _, v := range values {
						fullKey.WriteString(" " + v)
					}
					result[fullKey.String()] = nested
					values = nil // Reset - handled as nested block
					break
				}
			}

			if len(values) > 0 {
				// Expect semicolon
				if t.Type != config.TokenSemicolon {
					return nil, fmt.Errorf("line %d: expected ';' after value, got %s", t.Line, t.Type)
				}
				tok.Next() // consume ;

				// Join values
				var value strings.Builder
				for i, v := range values {
					if i > 0 {
						value.WriteString(" ")
					}
					value.WriteString(v)
				}
				result[key] = value.String()
			}

		case config.TokenSemicolon:
			// Flag (key with no value)
			tok.Next() // consume ;
			result[key] = "true"

		default:
			return nil, fmt.Errorf("line %d: expected value or '{' after %s, got %s", t.Line, key, t.Type)
		}
	}

	return result, nil
}
