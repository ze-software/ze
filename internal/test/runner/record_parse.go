// Design: docs/architecture/testing/ci-format.md — CI file parsing and test discovery
// Overview: record.go — Record type definitions and methods
// Related: record_collection.go — Tests container and querying

package runner

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/test/ci"
	"codeberg.org/thomas-mangin/ze/internal/test/tmpfs"
)

// EncodingTests manages encoding test discovery.
type EncodingTests struct {
	*Tests
	baseDir string
	port    int
}

// NewEncodingTests creates an encoding test manager.
func NewEncodingTests(baseDir string) *EncodingTests {
	return &EncodingTests{
		Tests:   NewTests(),
		baseDir: baseDir,
		port:    1790,
	}
}

// SetBasePort sets the starting port.
func (et *EncodingTests) SetBasePort(port int) {
	et.port = port
}

// Discover finds all .ci files in the directory.
func (et *EncodingTests) Discover(dir string) error {
	pattern := filepath.Join(dir, "*.ci")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}

	sort.Strings(files)

	for _, ciFile := range files {
		if err := et.parseAndAdd(ciFile); err != nil {
			return fmt.Errorf("%s: %w", filepath.Base(ciFile), err)
		}
	}

	return nil
}

// parseAndAdd parses a .ci file and adds it as a test.
// Uses new key=value format: action=type:key=value:key=value:...
// Supports Tmpfs blocks for embedded files.
func (et *EncodingTests) parseAndAdd(ciFile string) error {
	// First, try Tmpfs parsing to extract embedded files
	v, err := tmpfs.ReadFrom(ciFile)
	if err != nil {
		return fmt.Errorf("parse %s: %w", ciFile, err)
	}

	name := strings.TrimSuffix(filepath.Base(ciFile), ".ci")
	r := et.Add(name)
	r.Port = et.port
	et.port += 2 // Reserve 2 ports per test ($PORT and $PORT2)
	r.CIFile = ciFile
	r.Files = append(r.Files, ciFile)

	// Store Tmpfs files if any
	if len(v.Files) > 0 {
		r.TmpfsFiles = make(map[string][]byte)
		for _, f := range v.Files {
			r.TmpfsFiles[f.Path] = f.Content
		}
	}

	// Store stdin blocks if any
	if len(v.StdinBlocks) > 0 {
		r.StdinBlocks = v.StdinBlocks
		for name, content := range v.StdinBlocks {
			recordLogger().Debug("stdin block loaded", "name", name, "size", len(content), "preview", string(content[:min(100, len(content))]))
		}

		// Also parse "peer" stdin block for expectations (for reporting purposes).
		// The peer block content is passed to ze-peer which parses it, but the
		// test runner also needs to know about expectations for progress/failure reporting.
		if peerBlock, ok := v.StdinBlocks["peer"]; ok {
			lines := strings.SplitSeq(string(peerBlock), "\n")
			for line := range lines {
				trimmed := strings.TrimSpace(line)
				if trimmed == "" || strings.HasPrefix(trimmed, "#") {
					continue
				}
				// Parse expect= and action= lines for reporting purposes
				if strings.HasPrefix(trimmed, "expect=") || strings.HasPrefix(trimmed, "action=") {
					if err := et.parseLine(r, ciFile, trimmed); err != nil {
						// Log but don't fail - these are primarily for ze-peer
						recordLogger().Debug("parsing peer block line", "line", trimmed, "error", err)
					}
				}
			}
		}
	}

	// Parse the non-Tmpfs lines (option:, expect:, cmd:, run=, etc.)
	for lineNum, line := range v.OtherLines {
		if err := et.parseLine(r, ciFile, line); err != nil {
			return fmt.Errorf("line %d: %w", lineNum+1, err)
		}
	}

	// Verify config exists (for non-Tmpfs configs)
	if configPath, ok := r.Conf["config"].(string); ok {
		// Check if it's a Tmpfs file first
		if r.TmpfsFiles != nil {
			if _, isTmpfs := r.TmpfsFiles[filepath.Base(configPath)]; isTmpfs {
				// Config is in Tmpfs, will be written to temp dir at runtime
				goto generateDecoded
			}
		}
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			return fmt.Errorf("config not found: %s", configPath)
		}
	}

generateDecoded:
	// Generate decoded strings for messages with Raw
	for i := range r.Messages {
		if len(r.Messages[i].Raw) > 0 {
			if decoded, err := DecodeMessageBytes(r.Messages[i].Raw); err == nil {
				r.Messages[i].Decoded = decoded.String()
			}
		}
	}

	return nil
}

// parseLine parses a single .ci line in the action=type:key=value format.
func (et *EncodingTests) parseLine(r *Record, ciFile, line string) error {
	// Parse action=type:key=value:key=value:...
	// First segment is action=type, remaining segments are key=value pairs
	parts := strings.Split(line, ":")
	if len(parts) < 1 {
		return fmt.Errorf("invalid format %q, expected action=type:key=value", line)
	}

	// First segment is action=type
	actionType := strings.SplitN(parts[0], "=", 2)
	if len(actionType) != 2 {
		return fmt.Errorf("invalid format %q, expected action=type:key=value", line)
	}
	action := actionType[0]
	lineType := actionType[1]
	kvPairs := ci.ParseKVPairs(parts[1:])

	switch action {
	case "option":
		return et.parseOption(r, ciFile, lineType, kvPairs)
	case "expect":
		return et.parseExpect(r, lineType, kvPairs)
	case "reject":
		return et.parseReject(r, lineType, kvPairs)
	case "action":
		return et.parseAction(r, lineType, kvPairs)
	case "cmd":
		return et.parseCmd(r, lineType, kvPairs, line)
	case "http":
		return et.parseHTTP(r, lineType, line)
	default:
		return fmt.Errorf("unknown action %q in %q", action, line)
	}
}

// parseOption handles option=type:key=value lines.
func (et *EncodingTests) parseOption(r *Record, ciFile, optType string, kv map[string]string) error {
	switch optType {
	case "file":
		configName := kv["path"]
		if configName == "" {
			return fmt.Errorf("option:file missing path=")
		}
		configPath := filepath.Join(filepath.Dir(ciFile), configName)
		absConfig, err := filepath.Abs(configPath)
		if err != nil {
			return fmt.Errorf("invalid config path: %w", err)
		}
		absTestDir, err := filepath.Abs(filepath.Dir(ciFile))
		if err != nil {
			return fmt.Errorf("invalid test dir: %w", err)
		}
		if !strings.HasPrefix(absConfig, absTestDir+string(filepath.Separator)) && absConfig != absTestDir {
			return fmt.Errorf("config file outside test directory: %s", configName)
		}
		r.Conf["config"] = configPath
		r.ConfigFile = configPath
		r.Files = append(r.Files, configPath)

	case "asn":
		value := kv["value"]
		if value == "" {
			return fmt.Errorf("option:asn missing value=")
		}
		r.Extra["asn"] = value
		r.Options = append(r.Options, fmt.Sprintf("option=asn:value=%s", value))

	case "bind":
		value := kv["value"]
		if value == "" {
			return fmt.Errorf("option:bind missing value=")
		}
		r.Extra["bind"] = value
		r.Options = append(r.Options, fmt.Sprintf("option=bind:value=%s", value))

	case "timeout":
		value := kv["value"]
		if value == "" {
			return fmt.Errorf("option:timeout missing value=")
		}
		r.Extra["timeout"] = value

	case "tcp_connections":
		value := kv["value"]
		if value == "" {
			return fmt.Errorf("option:tcp_connections missing value=")
		}
		r.Options = append(r.Options, fmt.Sprintf("option=tcp_connections:value=%s", value))

	case "open":
		value := kv["value"]
		if value == "" {
			return fmt.Errorf("option:open missing value=")
		}
		r.Options = append(r.Options, fmt.Sprintf("option=open:value=%s", value))

	case "update":
		value := kv["value"]
		if value == "" {
			return fmt.Errorf("option:update missing value=")
		}
		r.Options = append(r.Options, fmt.Sprintf("option=update:value=%s", value))

	case "env":
		varName := kv["var"]
		value := kv["value"]
		if varName == "" {
			return fmt.Errorf("option:env missing var=")
		}
		// Store as KEY=VALUE for environment setting
		r.EnvVars = append(r.EnvVars, fmt.Sprintf("%s=%s", varName, value))

	default:
		return fmt.Errorf("unknown option type %q", optType)
	}
	return nil
}

// parseExpect handles expect:type:... lines.
func (et *EncodingTests) parseExpect(r *Record, expType string, kv map[string]string) error {
	switch expType {
	case "bgp":
		conn, seq, err := parseConnSeq(kv)
		if err != nil {
			return fmt.Errorf("expect:bgp: %w", err)
		}
		hexData := kv["hex"]
		if hexData == "" {
			return fmt.Errorf("expect:bgp missing hex=")
		}
		idx := connSeqToIndex(conn, seq)
		msg := r.getOrCreateMessage(idx)
		msg.RawHex = strings.ReplaceAll(hexData, ":", "")
		if rawBytes, err := hex.DecodeString(msg.RawHex); err == nil {
			msg.Raw = rawBytes
		}
		// Add to Expects for testpeer (new format).
		r.Expects = append(r.Expects, fmt.Sprintf("expect=bgp:conn=%d:seq=%d:hex=%s", conn, seq, hexData))

	case "json":
		conn, seq, err := parseConnSeq(kv)
		if err != nil {
			return fmt.Errorf("expect:json: %w", err)
		}
		jsonData := kv["json"]
		if jsonData == "" {
			return fmt.Errorf("expect:json missing json=")
		}
		idx := connSeqToIndex(conn, seq)
		msg := r.getOrCreateMessage(idx)
		msg.JSON = jsonData

	case "exit":
		codeStr := kv["code"]
		if codeStr == "" {
			return fmt.Errorf("expect:exit missing code=")
		}
		code, err := strconv.Atoi(codeStr)
		if err != nil {
			return fmt.Errorf("expect:exit invalid code=%q: %w", codeStr, err)
		}
		r.ExpectExitCode = &code

	case "stderr":
		// Support both pattern= (regex) and contains= (substring)
		if pattern, ok := kv["pattern"]; ok {
			r.ExpectStderr = append(r.ExpectStderr, pattern)
		}
		if contains := kv["contains"]; contains != "" {
			r.ExpectStderrMatch = contains
		}

	case "stdout":
		// Support contains= (substring match)
		if contains := kv["contains"]; contains != "" {
			r.ExpectStdoutMatch = append(r.ExpectStdoutMatch, contains)
		}

	case "syslog":
		pattern := kv["pattern"]
		r.ExpectSyslog = append(r.ExpectSyslog, pattern)

	default:
		return fmt.Errorf("unknown expect type %q", expType)
	}
	return nil
}

// parseReject handles reject:type:... lines.
func (et *EncodingTests) parseReject(r *Record, rejType string, kv map[string]string) error {
	switch rejType {
	case "stderr":
		pattern := kv["pattern"]
		r.RejectStderr = append(r.RejectStderr, pattern)

	case "syslog":
		pattern := kv["pattern"]
		r.RejectSyslog = append(r.RejectSyslog, pattern)

	default:
		return fmt.Errorf("unknown reject type %q", rejType)
	}
	return nil
}

// parseAction handles action:type:... lines.
func (et *EncodingTests) parseAction(r *Record, actType string, kv map[string]string) error {
	switch actType {
	case "notification":
		conn, seq, err := parseConnSeq(kv)
		if err != nil {
			return fmt.Errorf("action:notification: %w", err)
		}
		text := kv["text"]
		// Add to Expects for testpeer (new format).
		r.Expects = append(r.Expects, fmt.Sprintf("action=notification:conn=%d:seq=%d:text=%s", conn, seq, text))

	case "send":
		conn, seq, err := parseConnSeq(kv)
		if err != nil {
			return fmt.Errorf("action:send: %w", err)
		}
		hexData := kv["hex"]
		if hexData == "" {
			return fmt.Errorf("action:send missing hex=")
		}
		// Add to Expects for testpeer (new format).
		r.Expects = append(r.Expects, fmt.Sprintf("action=send:conn=%d:seq=%d:hex=%s", conn, seq, hexData))

	case "rewrite":
		conn, seq, err := parseConnSeq(kv)
		if err != nil {
			return fmt.Errorf("action:rewrite: %w", err)
		}
		source := kv["source"]
		if source == "" {
			return fmt.Errorf("action:rewrite missing source=")
		}
		dest := kv["dest"]
		if dest == "" {
			return fmt.Errorf("action:rewrite missing dest=")
		}
		// Add to Expects for testpeer (new format).
		r.Expects = append(r.Expects, fmt.Sprintf("action=rewrite:conn=%d:seq=%d:source=%s:dest=%s", conn, seq, source, dest))

	case "sighup":
		conn, seq, err := parseConnSeq(kv)
		if err != nil {
			return fmt.Errorf("action:sighup: %w", err)
		}
		// Add to Expects for testpeer (new format).
		r.Expects = append(r.Expects, fmt.Sprintf("action=sighup:conn=%d:seq=%d", conn, seq))

	case "sigterm":
		conn, seq, err := parseConnSeq(kv)
		if err != nil {
			return fmt.Errorf("action:sigterm: %w", err)
		}
		// Add to Expects for testpeer (new format).
		r.Expects = append(r.Expects, fmt.Sprintf("action=sigterm:conn=%d:seq=%d", conn, seq))

	default:
		return fmt.Errorf("unknown action type %q", actType)
	}
	return nil
}

// parseCmd handles cmd:type:... lines.
func (et *EncodingTests) parseCmd(r *Record, cmdType string, kv map[string]string, rawLine string) error {
	switch cmdType {
	case "api":
		conn, seq, err := parseConnSeq(kv)
		if err != nil {
			return fmt.Errorf("cmd:api: %w", err)
		}
		text := kv["text"]
		idx := connSeqToIndex(conn, seq)
		msg := r.getOrCreateMessage(idx)
		msg.Cmd = text

	case "background", "foreground":
		// Use marker-based parsing because exec= values may contain colons
		// (e.g., --web :8000). The standard KV parser splits on colons and
		// would truncate the exec value.
		rc, err := parseCmdExec(cmdType, rawLine)
		if err != nil {
			return err
		}
		r.RunCommands = append(r.RunCommands, rc)

	default:
		return fmt.Errorf("unknown cmd type %q", cmdType)
	}
	return nil
}

// parseCmdExec extracts fields from a cmd=background/foreground line using
// marker-based parsing. This handles exec= values containing colons correctly.
//
// Format: cmd=background:seq=N:exec=COMMAND[:stdin=BLOCK][:timeout=DUR].
func parseCmdExec(mode, line string) (RunCommand, error) {
	seqMarker := ":seq="
	execMarker := ":exec="
	stdinMarker := ":stdin="
	timeoutMarker := ":timeout="

	seqIdx := strings.Index(line, seqMarker)
	execIdx := strings.Index(line, execMarker)

	if seqIdx < 0 {
		return RunCommand{}, fmt.Errorf("cmd:%s missing seq=", mode)
	}
	if execIdx < 0 {
		return RunCommand{}, fmt.Errorf("cmd:%s missing exec=", mode)
	}

	// Extract seq value: from after ":seq=" to the next known marker or end.
	seqStart := seqIdx + len(seqMarker)
	seqEnd := nextMarker(line, seqStart, execMarker, stdinMarker, timeoutMarker)
	seqStr := line[seqStart:seqEnd]
	seq, err := strconv.Atoi(seqStr)
	if err != nil || seq < 1 {
		return RunCommand{}, fmt.Errorf("cmd:%s invalid seq=%q", mode, seqStr)
	}

	// Extract exec value: from after ":exec=" to the next known marker or end.
	// This correctly preserves colons inside the exec value.
	execStart := execIdx + len(execMarker)
	execEnd := nextMarker(line, execStart, stdinMarker, timeoutMarker)
	execVal := line[execStart:execEnd]
	if execVal == "" {
		return RunCommand{}, fmt.Errorf("cmd:%s missing exec=", mode)
	}

	rc := RunCommand{
		Mode: mode,
		Seq:  seq,
		Exec: execVal,
	}

	// Extract optional stdin= and timeout= values.
	if idx := strings.Index(line, stdinMarker); idx >= 0 {
		start := idx + len(stdinMarker)
		end := nextMarker(line, start, timeoutMarker)
		rc.Stdin = line[start:end]
	}
	if idx := strings.Index(line, timeoutMarker); idx >= 0 {
		start := idx + len(timeoutMarker)
		end := nextMarker(line, start) // no more markers
		rc.Timeout = line[start:end]
	}

	return rc, nil
}

// nextMarker returns the index of the earliest occurrence of any marker
// in line starting from offset, or len(line) if none found.
func nextMarker(line string, offset int, markers ...string) int {
	best := len(line)
	for _, m := range markers {
		if idx := strings.Index(line[offset:], m); idx >= 0 {
			if offset+idx < best {
				best = offset + idx
			}
		}
	}
	return best
}

// parseHTTP handles http=method:seq=N:url=URL:status=CODE[:contains=TEXT] lines.
// Uses marker-based parsing (nextMarker) because URLs contain colons that would
// confuse simple colon-splitting. Each marker's value extends to the next known
// marker or end-of-line, so marker order in the input does not matter.
func (et *EncodingTests) parseHTTP(r *Record, method, line string) error {
	if method != "get" && method != "post" {
		return fmt.Errorf("unsupported HTTP method %q (use get or post)", method)
	}

	seqMarker := ":seq="
	urlMarker := ":url="
	statusMarker := ":status="
	containsMarker := ":contains="

	seqIdx := strings.Index(line, seqMarker)
	urlIdx := strings.Index(line, urlMarker)
	statusIdx := strings.Index(line, statusMarker)
	containsIdx := strings.Index(line, containsMarker)

	if seqIdx < 0 {
		return fmt.Errorf("http= missing seq=")
	}
	if urlIdx < 0 {
		return fmt.Errorf("http= missing url=")
	}
	if statusIdx < 0 {
		return fmt.Errorf("http= missing status=")
	}

	// Extract seq value: from after ":seq=" to next known marker or end.
	seqStart := seqIdx + len(seqMarker)
	seqEnd := nextMarker(line, seqStart, urlMarker, statusMarker, containsMarker)
	seqStr := line[seqStart:seqEnd]
	seq, err := strconv.Atoi(seqStr)
	if err != nil || seq < 1 {
		return fmt.Errorf("http= invalid seq=%q", seqStr)
	}

	// Extract url value: from after ":url=" to next known marker or end.
	urlStart := urlIdx + len(urlMarker)
	urlEnd := nextMarker(line, urlStart, seqMarker, statusMarker, containsMarker)
	url := line[urlStart:urlEnd]

	// Extract status value: from after ":status=" to next known marker or end.
	statusStart := statusIdx + len(statusMarker)
	statusEnd := nextMarker(line, statusStart, seqMarker, urlMarker, containsMarker)
	statusStr := line[statusStart:statusEnd]
	status, err := strconv.Atoi(statusStr)
	if err != nil {
		return fmt.Errorf("http= invalid status=%q", statusStr)
	}

	// Extract optional contains value.
	var contains string
	if containsIdx >= 0 {
		containsStart := containsIdx + len(containsMarker)
		containsEnd := nextMarker(line, containsStart, seqMarker, urlMarker, statusMarker)
		contains = line[containsStart:containsEnd]
	}

	r.HTTPChecks = append(r.HTTPChecks, HTTPCheck{
		Seq:      seq,
		Method:   method,
		URL:      url,
		Status:   status,
		Contains: contains,
	})
	return nil
}

// parseConnSeq extracts conn and seq from key-value pairs.
// Validates: conn must be 1-4, seq must be >= 1.
func parseConnSeq(kv map[string]string) (conn, seq int, err error) {
	connStr := kv["conn"]
	seqStr := kv["seq"]

	if connStr == "" {
		return 0, 0, fmt.Errorf("missing conn=")
	}
	if seqStr == "" {
		return 0, 0, fmt.Errorf("missing seq=")
	}

	conn, err = strconv.Atoi(connStr)
	if err != nil || conn < 1 || conn > 4 {
		return 0, 0, fmt.Errorf("invalid conn=%q (must be 1-4)", connStr)
	}
	seq, err = strconv.Atoi(seqStr)
	if err != nil || seq < 1 {
		return 0, 0, fmt.Errorf("invalid seq=%q (must be >= 1)", seqStr)
	}

	return conn, seq, nil
}

// connSeqToIndex converts conn+seq to a unique message index.
// conn=1:seq=1 -> 101, conn=1:seq=2 -> 102, conn=2:seq=1 -> 201, etc.
func connSeqToIndex(conn, seq int) int {
	return conn*100 + seq
}
