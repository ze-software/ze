// Package main provides the ze-config-reader binary.
// This is a specialized process (not a regular plugin) that:
// 1. Receives YANG schemas from Hub after Stage 1 completes
// 2. Parses configuration files
// 3. Sends verify/apply requests to Hub for each config block
// 4. Handles reload requests from Hub
package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
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

// ConfigReader is the main config reader process state.
type ConfigReader struct {
	schemas    []SchemaInfo
	configPath string
	reader     *bufio.Reader
	writer     *bufio.Writer

	// Handler path to schema mapping for routing
	handlerMap map[string]*SchemaInfo
}

// NewConfigReader creates a new config reader instance.
func NewConfigReader() *ConfigReader {
	return &ConfigReader{
		reader:     bufio.NewReader(os.Stdin),
		writer:     bufio.NewWriter(os.Stdout),
		handlerMap: make(map[string]*SchemaInfo),
	}
}

// Run is the main entry point.
func (cr *ConfigReader) Run() error {
	// Phase 1: Receive initialization from Hub
	if err := cr.receiveInit(); err != nil {
		return fmt.Errorf("init: %w", err)
	}

	// Phase 2: Parse config and send verify requests
	if err := cr.parseAndVerify(); err != nil {
		return fmt.Errorf("parse: %w", err)
	}

	// Phase 3: Signal completion and wait for commands
	if err := cr.sendComplete(); err != nil {
		return fmt.Errorf("complete: %w", err)
	}

	// Event loop: handle reload and shutdown
	return cr.eventLoop()
}

// receiveInit receives schemas and config path from Hub.
// Format:
//
//	config schema <module> handlers <h1,h2,...> [yang <<EOF ... EOF]
//	config path <path>
//	config done
func (cr *ConfigReader) receiveInit() error {
	var heredocDelimiter string
	var currentYang strings.Builder

	for {
		line, err := cr.reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("read: %w", err)
		}
		line = strings.TrimSuffix(line, "\n")

		// Handle heredoc continuation
		if heredocDelimiter != "" {
			if strings.TrimSpace(line) == heredocDelimiter {
				// End of heredoc - assign to last schema
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

		// Parse init commands
		if line == "config done" {
			break
		}

		if strings.HasPrefix(line, "config schema ") {
			schema, delim, err := parseSchemaLine(line)
			if err != nil {
				return fmt.Errorf("parse schema: %w", err)
			}
			cr.schemas = append(cr.schemas, *schema)

			// Register handlers
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

		// Unknown command - ignore for forward compatibility
	}

	return nil
}

// parseSchemaLine parses "config schema <module> handlers <h1,h2,...> [yang <<EOF]".
// Returns the schema info and heredoc delimiter if present.
func parseSchemaLine(line string) (*SchemaInfo, string, error) {
	// Remove prefix
	rest := strings.TrimPrefix(line, "config schema ")

	// Parse module name (first word)
	parts := strings.Fields(rest)
	if len(parts) < 3 {
		return nil, "", fmt.Errorf("expected 'config schema <module> handlers <list>'")
	}

	schema := &SchemaInfo{
		Module: parts[0],
	}

	// Find "handlers" keyword
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

	// Parse handlers (comma-separated)
	handlerList := parts[handlersIdx+1]
	schema.Handlers = strings.Split(handlerList, ",")

	// Check for yang heredoc
	var heredocDelim string
	for i, p := range parts {
		if p == "yang" && i+1 < len(parts) && strings.HasPrefix(parts[i+1], "<<") {
			heredocDelim = strings.TrimPrefix(parts[i+1], "<<")
			break
		}
	}

	return schema, heredocDelim, nil
}

// parseAndVerify parses the config file and sends verify requests.
func (cr *ConfigReader) parseAndVerify() error {
	if cr.configPath == "" {
		return fmt.Errorf("no config path specified")
	}

	// Read config file
	content, err := os.ReadFile(cr.configPath)
	if err != nil {
		return fmt.Errorf("read config: %w", err)
	}

	// Tokenize
	tokenizer := config.NewTokenizer(string(content))
	tokens := tokenizer.All()

	// Parse into blocks and send verify requests
	serial := 1
	return cr.processBlocks(tokens, "", &serial)
}

// processBlocks processes config blocks and sends verify requests.
func (cr *ConfigReader) processBlocks(tokens []config.Token, pathPrefix string, serial *int) error {
	i := 0
	for i < len(tokens) {
		tok := tokens[i]

		// Skip non-keywords
		if tok.Type != config.TokenWord {
			i++
			continue
		}

		blockName := tok.Value
		i++

		// Check for list key (identifier after block name)
		var listKey string
		if i < len(tokens) && tokens[i].Type == config.TokenWord {
			listKey = tokens[i].Value
			i++
		}

		// Build handler path
		var handlerPath string
		if pathPrefix != "" {
			handlerPath = pathPrefix + "." + blockName
		} else {
			handlerPath = blockName
		}

		// Add list key if present
		if listKey != "" {
			handlerPath = fmt.Sprintf("%s[key=%s]", handlerPath, listKey)
		}

		// Find block content
		if i < len(tokens) && tokens[i].Type == config.TokenLBrace {
			i++ // Skip {
			braceCount := 1
			blockStart := i

			// Find matching }
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

			// Extract block tokens
			blockTokens := tokens[blockStart:blockEnd]

			// Find matching handler
			handler := cr.findHandler(handlerPath)
			if handler == nil {
				// Check if this is a known top-level block
				if pathPrefix == "" {
					return fmt.Errorf("unknown block: %s", blockName)
				}
				// For nested blocks without specific handler, continue
			}

			// Send verify request
			data := cr.tokensToJSON(blockTokens)
			if err := cr.sendVerify(*serial, handlerPath, data); err != nil {
				return err
			}
			*serial++

			// Process nested blocks
			if err := cr.processBlocks(blockTokens, handlerPath, serial); err != nil {
				return err
			}
		} else if i < len(tokens) && tokens[i].Type == config.TokenSemicolon {
			// Simple statement - skip
			i++
		}
	}

	return nil
}

// findHandler finds the handler for a given path using longest prefix match.
func (cr *ConfigReader) findHandler(path string) *SchemaInfo {
	// Try exact match
	if schema, ok := cr.handlerMap[path]; ok {
		return schema
	}

	// Extract base path (without list key)
	basePath := path
	if idx := strings.Index(path, "["); idx >= 0 {
		basePath = path[:idx]
	}

	if schema, ok := cr.handlerMap[basePath]; ok {
		return schema
	}

	// Try progressively shorter prefixes
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
func (cr *ConfigReader) tokensToJSON(tokens []config.Token) string {
	data := make(map[string]any)

	i := 0
	for i < len(tokens) {
		if tokens[i].Type == config.TokenWord {
			key := tokens[i].Value
			i++

			// Check for value
			if i < len(tokens) {
				//nolint:exhaustive // default case handles remaining types
				switch tokens[i].Type {
				case config.TokenWord, config.TokenString:
					data[key] = tokens[i].Value
					i++
				case config.TokenSemicolon:
					data[key] = true // Flag-style
					i++
				case config.TokenLBrace:
					// Nested block - skip for this level
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

		// Skip semicolons
		if i < len(tokens) && tokens[i].Type == config.TokenSemicolon {
			i++
		}
	}

	result, _ := json.Marshal(data)
	return string(result)
}

// sendVerify sends a verify request to Hub.
func (cr *ConfigReader) sendVerify(serial int, handler, data string) error {
	msg := fmt.Sprintf("#%d config verify handler %q data '%s'\n", serial, handler, data)
	_, err := cr.writer.WriteString(msg)
	if err != nil {
		return err
	}
	if err := cr.writer.Flush(); err != nil {
		return err
	}

	// Wait for response
	line, err := cr.reader.ReadString('\n')
	if err != nil {
		return err
	}
	line = strings.TrimSuffix(line, "\n")

	// Parse response: @serial status [message]
	if !strings.HasPrefix(line, fmt.Sprintf("@%d ", serial)) {
		return fmt.Errorf("unexpected response: %s", line)
	}

	rest := strings.TrimPrefix(line, fmt.Sprintf("@%d ", serial))
	if strings.HasPrefix(rest, "error ") {
		return fmt.Errorf("verify failed: %s", strings.TrimPrefix(rest, "error "))
	}

	return nil
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
				return nil // EOF = clean shutdown
			}
			return err
		}
		line = strings.TrimSuffix(line, "\n")

		// Parse command: #serial command
		if !strings.HasPrefix(line, "#") {
			continue
		}

		// Extract serial and command
		parts := strings.SplitN(line[1:], " ", 2)
		if len(parts) < 2 {
			continue
		}
		serial := parts[0]
		command := parts[1]

		switch {
		case command == "config reload":
			if err := cr.handleReload(serial); err != nil {
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

// handleReload handles a config reload request.
func (cr *ConfigReader) handleReload(_ string) error {
	// Re-parse config file
	return cr.parseAndVerify()
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
