// Design: docs/architecture/testing/ci-format.md — test runner framework

package runner

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

var (
	errMissingTypeLine                  = errors.New("missing type line")
	errMissingHexLine                   = errors.New("missing hex line")
	errMissingJsonLine                  = errors.New("missing json line")
	errMissingHexPayloadUseStdinPayload = errors.New("missing hex payload (use stdin=payload:hex= or decode=)")
	errMissingExpectJsonLine            = errors.New("missing expect=json: line")
)

// Message type constants.
const (
	msgTypeUpdate       = "update"
	msgTypeOpen         = "open"
	msgTypeNLRI         = "nlri"
	msgTypeNotification = "notification"
	msgTypeKeepalive    = "keepalive"
)

// decodingTest holds a single decoding test case.
type decodingTest struct {
	BaseTest     // Embeds Name, Nick, Active, Error
	File         string
	Type         string   // "open", "update"
	Family       string   // e.g., "l2vpn/evpn"
	Plugins      []string // --plugin flags for capability/NLRI decode
	HexPayload   string
	ExpectedJSON string
	OutputJSON   bool // true if --json flag specified in test

	// ParseError marks a test file that could not be parsed at discovery time.
	// Discover records the file as a permanent failure and continues, so a
	// malformed file fails loudly instead of vanishing from the suite. The
	// runner short-circuits such tests without executing them.
	ParseError error

	// Results
	ActualJSON string
}

// DecodingTests manages decoding test discovery and execution.
type DecodingTests struct {
	*TestSet[*decodingTest]
	baseDir string
}

// NewDecodingTests creates a new decoding test manager.
func NewDecodingTests(baseDir string) *DecodingTests {
	return &DecodingTests{
		TestSet: NewTestSet[*decodingTest](),
		baseDir: baseDir,
	}
}

// Discover finds all .test and .ci files in the directory.
func (dt *DecodingTests) Discover(dir string) error {
	// Find both .test (legacy) and .ci (new format) files
	var files []string
	for _, ext := range []string{"*.test", "*.ci"} {
		pattern := filepath.Join(dir, ext)
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return err
		}
		files = append(files, matches...)
	}

	sort.Strings(files)
	ResetNickCounter()

	for _, testFile := range files {
		var test *decodingTest
		var err error

		if strings.HasSuffix(testFile, ".ci") {
			test, err = dt.parseCIFile(testFile)
		} else {
			test, err = dt.parseTestFile(testFile)
		}
		if err != nil {
			// A malformed file used to `continue` here: it disappeared from the
			// suite with no warning and no failure, so its coverage was silently
			// lost. Warn and record it as a permanent failure instead -- a guard
			// that neither denies nor speaks does not exist
			// (ai/rules/fail-closed-guards.md). Mirrors EncodingTests.Discover.
			recordLogger().Warn("unparseable test file recorded as failure; continuing discovery",
				"file", filepath.Base(testFile), "error", err)
			base := filepath.Base(testFile)
			name := strings.TrimSuffix(base, filepath.Ext(base))
			test = &decodingTest{
				BaseTest:   BaseTest{Name: name, Nick: GenerateNick(name)},
				File:       testFile,
				ParseError: fmt.Errorf("parse %s: %w", testFile, err),
			}
		}
		dt.Add(test)
	}

	return nil
}

// parseTestFile parses a 3-line .test file.
func (dt *DecodingTests) parseTestFile(filePath string) (*decodingTest, error) {
	f, err := os.Open(filePath) //nolint:gosec // Test files from known directory
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)

	// Line 1: type [fam]
	if !scanner.Scan() {
		return nil, errMissingTypeLine
	}
	typeLine := strings.TrimSpace(scanner.Text())

	// Line 2: hex payload
	if !scanner.Scan() {
		return nil, errMissingHexLine
	}
	hexPayload := strings.TrimSpace(scanner.Text())

	// Line 3: expected JSON
	if !scanner.Scan() {
		return nil, errMissingJsonLine
	}
	expectedJSON := strings.TrimSpace(scanner.Text())

	// Parse type line: "update l2vpn/evpn" or "open"
	msgType, fam := parseTypeLine(typeLine)

	name := strings.TrimSuffix(filepath.Base(filePath), ".test")
	nick := GenerateNick(name)

	return &decodingTest{
		BaseTest: BaseTest{
			Name: name,
			Nick: nick,
		},
		File:         filePath,
		Type:         msgType,
		Family:       fam,
		HexPayload:   hexPayload,
		ExpectedJSON: expectedJSON,
	}, nil
}

// parseCIFile parses a .ci file with stdin=, cmd=, and expect= lines.
// New format:
//
//	stdin=payload:hex=<hex-payload>
//	cmd=foreground:seq=1:exec=ze-test decode --family <family> -:stdin=payload
//	expect=json:json=<expected-json>
//
// Legacy format (still supported):
//
//	decode=<type>:family=<family>:hex=<hex-payload>
//	expect=json:json=<expected-json>
func (dt *DecodingTests) parseCIFile(filePath string) (*decodingTest, error) {
	f, err := os.Open(filePath) //nolint:gosec // Test files from known directory
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	var msgType, family, hexPayload, expectedJSON string
	var cmdLine string

	scanner := bufio.NewScanner(f)
	stdinBlocks := make(map[string]string) // name -> hex content

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Parse stdin= line (single-line hex format)
		if after, ok := strings.CutPrefix(line, "stdin="); ok {
			rest := after
			parts := strings.Split(rest, ":")
			if len(parts) >= 2 {
				stdinName := parts[0]
				for _, part := range parts[1:] {
					if after, ok := strings.CutPrefix(part, "hex="); ok {
						stdinBlocks[stdinName] = after
					}
				}
			}
			continue
		}

		// Parse cmd= line (new format: cmd=foreground:seq=1:exec=...)
		if strings.HasPrefix(line, "cmd=") {
			cmdLine = line
			continue
		}

		// Parse legacy decode= line
		if after, ok := strings.CutPrefix(line, "decode="); ok {
			rest := after
			parts := strings.Split(rest, ":")
			if len(parts) == 0 {
				return nil, fmt.Errorf("invalid decode= line: %s", line)
			}
			msgType = strings.ToLower(parts[0])

			// Parse key=value pairs
			for _, part := range parts[1:] {
				if after, ok := strings.CutPrefix(part, "family="); ok {
					family = after
				} else if after, ok := strings.CutPrefix(part, "hex="); ok {
					hexPayload = after
				}
			}
			continue
		}

		// Parse expect=json: line
		if after, ok := strings.CutPrefix(line, "expect=json:"); ok {
			rest := after
			if after, ok := strings.CutPrefix(rest, "json="); ok {
				expectedJSON = after
			}
			continue
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// New format: extract from cmd: line
	var outputJSON bool
	var plugins []string
	if cmdLine != "" && hexPayload == "" {
		cmd := parseDecodeCmdLine(cmdLine, stdinBlocks)
		msgType = cmd.msgType
		family = cmd.family
		hexPayload = cmd.hexPayload
		outputJSON = cmd.outputJSON
		plugins = cmd.plugins
	}

	// Validate required fields
	if msgType == "" {
		msgType = msgTypeUpdate // Default to update
	}
	if hexPayload == "" {
		return nil, errMissingHexPayloadUseStdinPayload
	}
	if expectedJSON == "" {
		return nil, errMissingExpectJsonLine
	}

	name := strings.TrimSuffix(filepath.Base(filePath), ".ci")
	nick := GenerateNick(name)

	return &decodingTest{
		BaseTest: BaseTest{
			Name: name,
			Nick: nick,
		},
		File:         filePath,
		Type:         msgType,
		Family:       family,
		Plugins:      plugins,
		HexPayload:   hexPayload,
		ExpectedJSON: expectedJSON,
		OutputJSON:   outputJSON,
	}, nil
}

// decodeCmdResult holds parsed results from a cmd= line.
type decodeCmdResult struct {
	msgType    string
	family     string
	hexPayload string
	outputJSON bool
	plugins    []string
}

// parseDecodeCmdLine extracts type, family, hex payload, json flag, and plugins from a cmd= line.
// Format: cmd=foreground:seq=1:exec=ze-test decode [--json] [--plugin <name>] --open -:stdin=payload.
func parseDecodeCmdLine(cmdLine string, stdinBlocks map[string]string) decodeCmdResult {
	result := decodeCmdResult{msgType: msgTypeUpdate}

	// Find exec= part
	rest := strings.TrimPrefix(cmdLine, "cmd:")
	parts := strings.Split(rest, ":")

	var execPart string
	var stdinRef string
	for _, part := range parts {
		if after, ok := strings.CutPrefix(part, "exec="); ok {
			execPart = after
		}
		if after, ok := strings.CutPrefix(part, "stdin="); ok {
			stdinRef = after
		}
	}

	if execPart == "" {
		return result
	}

	// Parse exec command: ze-test decode [--json] [--plugin <name>] [--family <family>] [--open|--update] [--nlri <family>] -
	args := strings.Fields(execPart)
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--json", "-json":
			result.outputJSON = true
		case "--plugin":
			if i+1 < len(args) {
				result.plugins = append(result.plugins, args[i+1])
				i++
			}
		case "--family", "-f":
			if i+1 < len(args) {
				result.family = args[i+1]
				i++
			}
		case "--open":
			result.msgType = msgTypeOpen
		case "--update":
			result.msgType = msgTypeUpdate
		case "--notification":
			result.msgType = msgTypeNotification
		case "--keepalive":
			result.msgType = msgTypeKeepalive
		case "--nlri":
			result.msgType = msgTypeNLRI
			// --nlri takes family as its value
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				result.family = args[i+1]
				i++
			}
		}
	}

	// Get hex from stdin reference
	if stdinRef != "" {
		if hex, ok := stdinBlocks[stdinRef]; ok {
			result.hexPayload = hex
		}
	}

	return result
}

// parseTypeLine parses "update l2vpn/evpn" into type and family.
func parseTypeLine(line string) (msgType, family string) {
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return msgTypeUpdate, ""
	}

	msgType = strings.ToLower(parts[0])
	if len(parts) >= 2 {
		family = textbuf.Join(parts[1:], " ")
	}
	return msgType, family
}

// List prints available tests (overrides TestSet.List to show Type).
func (dt *DecodingTests) List() {
	writeTestListHeader("Available decoding tests")
	registered := dt.Registered()
	total := len(registered)
	for i, t := range registered {
		var suffix textbuf.Buffer
		writeTestListLine(i+1, total, t.Nick, t.Name, suffix.Str(" (").Str(t.Type).Byte(')').String())
	}
	writeTestListFooter()
}

// decodingRunner executes decoding tests.
type decodingRunner struct {
	tests   *DecodingTests
	baseDir string
	zePath  string
	colors  *Colors
}

// NewDecodingRunner creates a decoding test runner.
func NewDecodingRunner(tests *DecodingTests, baseDir, zePath string) *decodingRunner {
	return &decodingRunner{
		tests:   tests,
		baseDir: baseDir,
		zePath:  zePath,
		colors:  NewColors(),
	}
}

// Run executes selected tests in parallel with real-time progress display.
func (r *decodingRunner) Run(ctx context.Context, verbose, quiet bool) bool {
	selected := r.tests.Selected()
	if len(selected) == 0 {
		fmt.Fprintln(os.Stdout, "No tests selected") //nolint:errcheck // user output
		return true
	}

	// Create parallel runner with generic type for direct test access
	runner := NewParallelRunner[*decodingTest](r.colors)
	runner.SetQuiet(quiet)
	runner.SetVerbose(verbose)
	runner.SetLabel("decode")
	runner.setNoHeader(true) // header managed by caller
	runner.SetBaseDir(r.baseDir)

	// Add tests to runner
	for _, test := range selected {
		runner.AddTestWithNick(test.Name, test.Nick, test, func(runCtx context.Context, t *decodingTest) (bool, error) {
			success := r.runTest(runCtx, t)
			if !success {
				return false, t.Error
			}
			return true, nil
		})
	}

	// Set failure callback for verbose output
	runner.SetOnFail(func(test *decodingTest, _ error) {
		fmt.Fprintf(os.Stdout, "%s %s: %v\n", r.colors.Red("✗"), test.Name, test.Error) //nolint:errcheck // user output
		if test.ActualJSON != "" {
			r.printJSONDiff(test)
		}
	})

	return runner.Run(ctx)
}

// runTest executes a single decoding test.
func (r *decodingRunner) runTest(ctx context.Context, test *decodingTest) bool {
	// A test file that failed to parse at discovery has no payload to decode.
	// Report the parse error as a hard failure without attempting execution
	// (see DecodingTests.Discover).
	if test.ParseError != nil {
		test.Error = test.ParseError
		return false
	}

	// Build command args
	args := []string{"bgp", "decode"}

	// Add --json flag if test specifies it
	if test.OutputJSON {
		args = append(args, "--json")
	}

	// Add --plugin flags for capability/NLRI decode
	for _, p := range test.Plugins {
		args = append(args, "--plugin", p)
	}

	switch test.Type {
	case "open":
		args = append(args, "--open")
	case "nlri":
		// --nlri takes the family as its value
		if test.Family != "" {
			args = append(args, "--nlri", test.Family)
		} else {
			args = append(args, "--nlri", "unknown/unknown")
		}
	case msgTypeUpdate:
		args = append(args, "--update")
		if test.Family != "" {
			args = append(args, "-f", test.Family)
		}
	case msgTypeNotification:
		args = append(args, "--notification")
	case msgTypeKeepalive:
		args = append(args, "--keepalive")
	default:
		args = append(args, "--update") // Default to update.
		if test.Family != "" {
			args = append(args, "-f", test.Family)
		}
	}

	args = append(args, test.HexPayload)

	// Run command
	cmd := exec.CommandContext(ctx, r.zePath, args...) //nolint:gosec // Test runner, paths from temp dir
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	err := cmd.Run()
	if err != nil {
		test.Error = fmt.Errorf("command failed: %w: %s", err, stderrBuf.String())
		return false
	}

	test.ActualJSON = strings.TrimSpace(stdoutBuf.String())

	// Compare JSON (ignoring volatile fields)
	return r.compareJSON(test)
}

// compareJSON compares actual vs expected JSON, ignoring volatile fields.
func (r *decodingRunner) compareJSON(test *decodingTest) bool {
	// Parse both JSONs
	var actual, expected map[string]any
	if err := json.Unmarshal([]byte(test.ActualJSON), &actual); err != nil {
		test.Error = fmt.Errorf("invalid actual JSON: %w", err)
		return false
	}
	if err := json.Unmarshal([]byte(test.ExpectedJSON), &expected); err != nil {
		test.Error = fmt.Errorf("invalid expected JSON: %w", err)
		return false
	}

	// Remove volatile fields
	volatileFields := []string{"exabgp", "ze-bgp", "time", "host", "pid", "ppid", "counter"}
	for _, field := range volatileFields {
		delete(actual, field)
		delete(expected, field)
	}

	// Normalize neighbor section (ExaBGP uses "neighbor", we might use "peer")
	normalizeNeighborSection(actual)
	normalizeNeighborSection(expected)

	// Compare
	actualBytes, _ := json.Marshal(actual)
	expectedBytes, _ := json.Marshal(expected)

	if !bytes.Equal(actualBytes, expectedBytes) {
		diff := ColoredCharDiff(string(expectedBytes), string(actualBytes))
		test.Error = fmt.Errorf("JSON mismatch\n%s", diff)
		return false
	}

	return true
}

// normalizeNeighborSection handles "neighbor" vs "peer" differences.
func normalizeNeighborSection(m map[string]any) {
	// If "peer" exists but "neighbor" doesn't, rename it
	if peer, ok := m["peer"]; ok {
		if _, hasNeighbor := m["neighbor"]; !hasNeighbor {
			m["neighbor"] = peer
			delete(m, "peer")
		}
	}

	// Also normalize within neighbor section
	if neighbor, ok := m["neighbor"].(map[string]any); ok {
		// Remove direction field (not always present in expected)
		delete(neighbor, "direction")

		// Normalize address section
		if addr, ok := neighbor["address"].(map[string]any); ok {
			// ExaBGP test files may have different local/peer addresses
			// For now, we skip address comparison
			_ = addr
		}
	}
}

// printJSONDiff prints a diff between actual and expected JSON.
func (r *decodingRunner) printJSONDiff(test *decodingTest) {
	fmt.Println("  Expected:")
	var expected map[string]any
	if err := json.Unmarshal([]byte(test.ExpectedJSON), &expected); err == nil {
		prettyExpected, _ := json.MarshalIndent(expected, "    ", "  ")
		fmt.Printf("    %s\n", prettyExpected)
	}

	fmt.Println("  Actual:")
	var actual map[string]any
	if err := json.Unmarshal([]byte(test.ActualJSON), &actual); err == nil {
		prettyActual, _ := json.MarshalIndent(actual, "    ", "  ")
		fmt.Printf("    %s\n", prettyActual)
	}
}
