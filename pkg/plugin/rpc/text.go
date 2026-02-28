// Design: docs/architecture/api/ipc_protocol.md — text handshake serialization
// Related: types.go — RPC type definitions used by text format/parse
// Related: text_conn.go — TextConn text-mode framing over net.Conn
// Related: text_mux.go — TextMuxConn concurrent text RPCs with #N routing

package rpc

import (
	"fmt"
	"strconv"
	"strings"
)

// heredocMarker is the fixed terminator for heredoc-style JSON embedding.
const heredocMarker = "END"

// textTrue is the string representation of boolean true in text protocol.
const textTrue = "true"

// Common text protocol keywords used across multiple stages.
const (
	kwEncoding = "encoding"
	kwPeers    = "peers"
)

// --- Stage 1: Registration ---

// FormatRegistrationText formats DeclareRegistrationInput as text lines for stage 1.
// Each declaration is one line. The message starts with "register" and ends with a blank line.
func FormatRegistrationText(input DeclareRegistrationInput) (string, error) {
	var b strings.Builder

	b.WriteString("register\n")

	for _, f := range input.Families {
		fmt.Fprintf(&b, "family %s mode %s\n", f.Name, f.Mode)
	}

	for _, c := range input.Commands {
		fmt.Fprintf(&b, "command %s", c.Name)
		if c.Description != "" {
			fmt.Fprintf(&b, " description %q", c.Description)
		}
		if len(c.Args) > 0 {
			fmt.Fprintf(&b, " args %s", strings.Join(c.Args, ","))
		}
		if c.Completable {
			b.WriteString(" completable true")
		}
		b.WriteByte('\n')
	}

	for _, d := range input.Dependencies {
		fmt.Fprintf(&b, "dependency %s\n", d)
	}

	for _, w := range input.WantsConfig {
		fmt.Fprintf(&b, "config-root %s\n", w)
	}

	if input.Schema != nil {
		s := input.Schema
		fmt.Fprintf(&b, "schema module %s", s.Module)
		if s.Namespace != "" {
			fmt.Fprintf(&b, " namespace %s", s.Namespace)
		}
		if len(s.Handlers) > 0 {
			fmt.Fprintf(&b, " handlers %s", strings.Join(s.Handlers, ","))
		}
		b.WriteByte('\n')
	}

	if input.WantsValidateOpen {
		b.WriteString("wants-validate-open true\n")
	}
	if input.CacheConsumer {
		b.WriteString("cache-consumer true\n")
	}
	if input.CacheConsumerUnordered {
		b.WriteString("cache-consumer-unordered true\n")
	}

	b.WriteByte('\n') // blank line terminator

	return b.String(), nil
}

// registrationKeywords lists all valid top-level keywords for stage 1.
var registrationKeywords = map[string]bool{
	"family": true, "command": true, "dependency": true, "config-root": true,
	"schema": true, "wants-validate-open": true, "cache-consumer": true,
	"cache-consumer-unordered": true,
}

// ParseRegistrationText parses text lines into DeclareRegistrationInput for stage 1.
func ParseRegistrationText(text string) (DeclareRegistrationInput, error) {
	var result DeclareRegistrationInput

	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	if len(lines) == 0 || lines[0] != "register" {
		return result, fmt.Errorf("text registration: expected 'register' verb, got %q", firstLine(lines))
	}

	for _, line := range lines[1:] {
		if line == "" {
			break
		}
		fields := tokenizeLine(line)
		if len(fields) < 2 {
			return result, fmt.Errorf("text registration: short line: %q", line)
		}

		keyword := fields[0]
		if !registrationKeywords[keyword] {
			return result, fmt.Errorf("text registration: unknown keyword %q", keyword)
		}

		var err error
		switch keyword {
		case "family":
			var f FamilyDecl
			f, err = parseFamilyDecl(fields[1:])
			if err == nil {
				result.Families = append(result.Families, f)
			}
		case "command":
			var c CommandDecl
			c, err = parseCommandDecl(fields[1:])
			if err == nil {
				result.Commands = append(result.Commands, c)
			}
		case "dependency":
			result.Dependencies = append(result.Dependencies, fields[1])
		case "config-root":
			result.WantsConfig = append(result.WantsConfig, fields[1])
		case "schema":
			var s SchemaDecl
			s, err = parseSchemaDecl(fields[1:])
			if err == nil {
				result.Schema = &s
			}
		case "wants-validate-open":
			result.WantsValidateOpen = fields[1] == textTrue
		case "cache-consumer":
			result.CacheConsumer = fields[1] == textTrue
		case "cache-consumer-unordered":
			result.CacheConsumerUnordered = fields[1] == textTrue
		}
		if err != nil {
			return result, err
		}
	}

	return result, nil
}

// --- Stage 2: Config (heredoc) ---

// FormatConfigureText formats ConfigureInput as text with heredoc-delimited JSON for stage 2.
func FormatConfigureText(input ConfigureInput) (string, error) {
	var b strings.Builder

	b.WriteString("configure\n")

	for _, s := range input.Sections {
		fmt.Fprintf(&b, "root %s json << %s\n", s.Root, heredocMarker)
		b.WriteString(s.Data)
		b.WriteByte('\n')
		b.WriteString(heredocMarker)
		b.WriteByte('\n')
	}

	b.WriteByte('\n') // blank line terminator

	return b.String(), nil
}

// ParseConfigureText parses text with heredoc-delimited JSON into ConfigureInput for stage 2.
func ParseConfigureText(text string) (ConfigureInput, error) {
	var result ConfigureInput

	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	if len(lines) == 0 || lines[0] != "configure" {
		return result, fmt.Errorf("text configure: expected 'configure' verb, got %q", firstLine(lines))
	}

	i := 1
	for i < len(lines) {
		line := lines[i]
		if line == "" {
			break
		}

		// Expect: root <name> json << END
		fields := strings.Fields(line)
		if len(fields) < 5 || fields[0] != "root" || fields[2] != "json" || fields[3] != "<<" || fields[4] != heredocMarker {
			return result, fmt.Errorf("text configure: expected 'root <name> json << %s', got %q", heredocMarker, line)
		}

		root := fields[1]
		i++

		// Read lines until heredoc marker
		var jsonLines []string
		for i < len(lines) && lines[i] != heredocMarker {
			jsonLines = append(jsonLines, lines[i])
			i++
		}
		if i >= len(lines) {
			return result, fmt.Errorf("text configure: missing heredoc terminator %q for root %q", heredocMarker, root)
		}
		i++ // skip the END marker

		result.Sections = append(result.Sections, ConfigSection{
			Root: root,
			Data: strings.Join(jsonLines, "\n"),
		})
	}

	return result, nil
}

// --- Stage 3: Capabilities ---

// FormatCapabilitiesText formats DeclareCapabilitiesInput as text lines for stage 3.
func FormatCapabilitiesText(input DeclareCapabilitiesInput) (string, error) {
	var b strings.Builder

	b.WriteString("capabilities\n")

	for _, c := range input.Capabilities {
		fmt.Fprintf(&b, "code %d", c.Code)
		if c.Encoding != "" {
			fmt.Fprintf(&b, " encoding %s", c.Encoding)
		}
		if c.Payload != "" {
			fmt.Fprintf(&b, " payload %s", c.Payload)
		}
		if len(c.Peers) > 0 {
			fmt.Fprintf(&b, " peers %s", strings.Join(c.Peers, ","))
		}
		b.WriteByte('\n')
	}

	b.WriteByte('\n')

	return b.String(), nil
}

// ParseCapabilitiesText parses text lines into DeclareCapabilitiesInput for stage 3.
func ParseCapabilitiesText(text string) (DeclareCapabilitiesInput, error) {
	var result DeclareCapabilitiesInput

	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	if len(lines) == 0 || lines[0] != "capabilities" {
		return result, fmt.Errorf("text capabilities: expected 'capabilities' verb, got %q", firstLine(lines))
	}

	for _, line := range lines[1:] {
		if line == "" {
			break
		}
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] != "code" {
			return result, fmt.Errorf("text capabilities: expected 'code <N> ...', got %q", line)
		}

		c, err := parseCapabilityDecl(fields)
		if err != nil {
			return result, err
		}
		result.Capabilities = append(result.Capabilities, c)
	}

	return result, nil
}

// --- Stage 4: Registry ---

// FormatRegistryText formats ShareRegistryInput as text lines for stage 4.
func FormatRegistryText(input ShareRegistryInput) (string, error) {
	var b strings.Builder

	b.WriteString("registry\n")

	for _, c := range input.Commands {
		fmt.Fprintf(&b, "command %s plugin %s", c.Name, c.Plugin)
		if c.Encoding != "" {
			fmt.Fprintf(&b, " encoding %s", c.Encoding)
		}
		b.WriteByte('\n')
	}

	b.WriteByte('\n')

	return b.String(), nil
}

// ParseRegistryText parses text lines into ShareRegistryInput for stage 4.
func ParseRegistryText(text string) (ShareRegistryInput, error) {
	var result ShareRegistryInput

	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	if len(lines) == 0 || lines[0] != "registry" {
		return result, fmt.Errorf("text registry: expected 'registry' verb, got %q", firstLine(lines))
	}

	for _, line := range lines[1:] {
		if line == "" {
			break
		}
		fields := strings.Fields(line)
		if len(fields) < 4 || fields[0] != "command" || fields[2] != "plugin" {
			return result, fmt.Errorf("text registry: expected 'command <name> plugin <plugin> ...', got %q", line)
		}

		cmd := RegistryCommand{
			Name:   fields[1],
			Plugin: fields[3],
		}
		for i := 4; i+1 < len(fields); i += 2 {
			if fields[i] == kwEncoding {
				cmd.Encoding = fields[i+1]
			} else {
				return result, fmt.Errorf("text registry: unknown keyword %q in command %q", fields[i], cmd.Name)
			}
		}
		result.Commands = append(result.Commands, cmd)
	}

	return result, nil
}

// --- Stage 5: Ready ---

// FormatReadyText formats ReadyInput as text for stage 5.
func FormatReadyText(input ReadyInput) (string, error) {
	var b strings.Builder

	b.WriteString("ready\n")

	if input.Subscribe != nil {
		s := input.Subscribe
		b.WriteString("subscribe")
		if len(s.Events) > 0 {
			fmt.Fprintf(&b, " events %s", strings.Join(s.Events, ","))
		}
		if s.Encoding != "" {
			fmt.Fprintf(&b, " encoding %s", s.Encoding)
		}
		if s.Format != "" {
			fmt.Fprintf(&b, " format %s", s.Format)
		}
		if len(s.Peers) > 0 {
			fmt.Fprintf(&b, " peers %s", strings.Join(s.Peers, ","))
		}
		b.WriteByte('\n')
	}

	b.WriteByte('\n')

	return b.String(), nil
}

// subscribeKeywords lists valid keywords inside a subscribe line.
var subscribeKeywords = map[string]bool{
	"events": true, kwEncoding: true, "format": true, "peers": true,
}

// ParseReadyText parses text into ReadyInput for stage 5.
func ParseReadyText(text string) (ReadyInput, error) {
	var result ReadyInput

	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	if len(lines) == 0 || lines[0] != "ready" {
		return result, fmt.Errorf("text ready: expected 'ready' verb, got %q", firstLine(lines))
	}

	for _, line := range lines[1:] {
		if line == "" {
			break
		}
		fields := strings.Fields(line)
		if len(fields) < 1 {
			continue
		}
		if fields[0] != "subscribe" {
			return result, fmt.Errorf("text ready: unknown keyword %q", fields[0])
		}

		sub := &SubscribeEventsInput{}
		for i := 1; i+1 < len(fields); i += 2 {
			key := fields[i]
			if !subscribeKeywords[key] {
				return result, fmt.Errorf("text ready: unknown subscribe keyword %q", key)
			}
			val := fields[i+1]
			switch key {
			case "events":
				sub.Events = strings.Split(val, ",")
			case kwEncoding:
				sub.Encoding = val
			case "format":
				sub.Format = val
			case "peers":
				sub.Peers = strings.Split(val, ",")
			}
		}
		result.Subscribe = sub
	}

	return result, nil
}

// --- Helpers ---

// tokenizeLine splits a line respecting double-quoted strings.
func tokenizeLine(line string) []string {
	var tokens []string
	var current strings.Builder
	inQuote := false

	for i := range len(line) {
		ch := line[i]
		switch {
		case ch == '"':
			inQuote = !inQuote
		case ch == ' ' && !inQuote:
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		case ch != ' ' || inQuote:
			current.WriteByte(ch)
		}
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens
}

func firstLine(lines []string) string {
	if len(lines) > 0 {
		return lines[0]
	}
	return ""
}

func parseFamilyDecl(fields []string) (FamilyDecl, error) {
	var f FamilyDecl
	if len(fields) < 3 || fields[1] != "mode" {
		return f, fmt.Errorf("text registration: expected '<family> mode <mode>', got %v", fields)
	}
	f.Name = fields[0]
	f.Mode = fields[2]
	return f, nil
}

func parseCommandDecl(fields []string) (CommandDecl, error) {
	if len(fields) < 1 {
		return CommandDecl{}, fmt.Errorf("text registration: empty command declaration")
	}

	c := CommandDecl{Name: fields[0]}
	commandKeywords := map[string]bool{"description": true, "args": true, "completable": true}

	for i := 1; i+1 < len(fields); i += 2 {
		key := fields[i]
		if !commandKeywords[key] {
			return CommandDecl{}, fmt.Errorf("text registration: unknown command keyword %q", key)
		}
		val := fields[i+1]
		switch key {
		case "description":
			c.Description = val
		case "args":
			c.Args = strings.Split(val, ",")
		case "completable":
			c.Completable = val == textTrue
		}
	}
	return c, nil
}

func parseSchemaDecl(fields []string) (SchemaDecl, error) {
	var s SchemaDecl
	schemaKeywords := map[string]bool{"module": true, "namespace": true, "handlers": true}

	for i := 0; i+1 < len(fields); i += 2 {
		key := fields[i]
		if !schemaKeywords[key] {
			return SchemaDecl{}, fmt.Errorf("text registration: unknown schema keyword %q", key)
		}
		val := fields[i+1]
		switch key {
		case "module":
			s.Module = val
		case "namespace":
			s.Namespace = val
		case "handlers":
			s.Handlers = strings.Split(val, ",")
		}
	}

	if s.Module == "" {
		return s, fmt.Errorf("text registration: schema missing module")
	}
	return s, nil
}

func parseCapabilityDecl(fields []string) (CapabilityDecl, error) {
	var c CapabilityDecl
	capKeywords := map[string]bool{"code": true, kwEncoding: true, "payload": true, "peers": true}

	for i := 0; i+1 < len(fields); i += 2 {
		key := fields[i]
		if !capKeywords[key] {
			return CapabilityDecl{}, fmt.Errorf("text capabilities: unknown keyword %q", key)
		}
		val := fields[i+1]
		switch key {
		case "code":
			n, err := strconv.ParseUint(val, 10, 8)
			if err != nil {
				return c, fmt.Errorf("text capabilities: invalid code %q: %w", val, err)
			}
			c.Code = uint8(n)
		case kwEncoding:
			c.Encoding = val
		case "payload":
			c.Payload = val
		case "peers":
			c.Peers = strings.Split(val, ",")
		}
	}
	return c, nil
}
