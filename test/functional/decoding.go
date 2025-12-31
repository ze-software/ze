package functional

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// DecodingTest holds a single decoding test case.
type DecodingTest struct {
	Name         string
	Nick         string
	File         string
	Type         string // "open", "update"
	Family       string // e.g., "l2vpn evpn"
	HexPayload   string
	ExpectedJSON string

	// Results
	Active     bool
	State      State
	ActualJSON string
	Error      error
	Duration   time.Duration
}

// DecodingTests manages decoding test discovery and execution.
type DecodingTests struct {
	tests   []*DecodingTest
	byNick  map[string]*DecodingTest
	baseDir string
}

// NewDecodingTests creates a new decoding test manager.
func NewDecodingTests(baseDir string) *DecodingTests {
	return &DecodingTests{
		byNick:  make(map[string]*DecodingTest),
		baseDir: baseDir,
	}
}

// Discover finds all .test files in the directory.
func (dt *DecodingTests) Discover(dir string) error {
	pattern := filepath.Join(dir, "*.test")
	files, err := filepath.Glob(pattern)
	if err != nil {
		return err
	}

	sort.Strings(files)
	ResetNickCounter()

	for _, testFile := range files {
		test, err := dt.parseTestFile(testFile)
		if err != nil {
			// Skip malformed test files
			continue
		}
		dt.tests = append(dt.tests, test)
		dt.byNick[test.Nick] = test
	}

	return nil
}

// parseTestFile parses a 3-line .test file.
func (dt *DecodingTests) parseTestFile(filePath string) (*DecodingTest, error) {
	f, err := os.Open(filePath) //nolint:gosec // Test files from known directory
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)

	// Line 1: type [family]
	if !scanner.Scan() {
		return nil, fmt.Errorf("missing type line")
	}
	typeLine := strings.TrimSpace(scanner.Text())

	// Line 2: hex payload
	if !scanner.Scan() {
		return nil, fmt.Errorf("missing hex line")
	}
	hexPayload := strings.TrimSpace(scanner.Text())

	// Line 3: expected JSON
	if !scanner.Scan() {
		return nil, fmt.Errorf("missing json line")
	}
	expectedJSON := strings.TrimSpace(scanner.Text())

	// Parse type line: "update l2vpn evpn" or "open"
	msgType, family := parseTypeLine(typeLine)

	name := strings.TrimSuffix(filepath.Base(filePath), ".test")
	nick := generateNick(name)

	return &DecodingTest{
		Name:         name,
		Nick:         nick,
		File:         filePath,
		Type:         msgType,
		Family:       family,
		HexPayload:   hexPayload,
		ExpectedJSON: expectedJSON,
	}, nil
}

// parseTypeLine parses "update l2vpn evpn" into type and family.
func parseTypeLine(line string) (msgType, family string) {
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return "update", ""
	}

	msgType = strings.ToLower(parts[0])
	if len(parts) >= 3 {
		family = strings.Join(parts[1:], " ")
	}
	return msgType, family
}

// Registered returns all tests in order.
func (dt *DecodingTests) Registered() []*DecodingTest {
	return dt.tests
}

// Selected returns active tests.
func (dt *DecodingTests) Selected() []*DecodingTest {
	var result []*DecodingTest
	for _, t := range dt.tests {
		if t.Active {
			result = append(result, t)
		}
	}
	return result
}

// Count returns the number of tests.
func (dt *DecodingTests) Count() int {
	return len(dt.tests)
}

// EnableAll activates all tests.
func (dt *DecodingTests) EnableAll() {
	for _, t := range dt.tests {
		t.Active = true
	}
}

// EnableByNick activates a test by nick.
func (dt *DecodingTests) EnableByNick(nick string) bool {
	if t, ok := dt.byNick[nick]; ok {
		t.Active = true
		return true
	}
	return false
}

// List prints available tests.
func (dt *DecodingTests) List() {
	fmt.Println("\nAvailable decoding tests:")
	fmt.Println()
	for _, t := range dt.tests {
		fmt.Printf("  %s  %s (%s)\n", t.Nick, t.Name, t.Type)
	}
	fmt.Println()
}

// DecodingRunner executes decoding tests.
type DecodingRunner struct {
	tests     *DecodingTests
	baseDir   string
	zebgpPath string
	colors    *Colors
}

// NewDecodingRunner creates a decoding test runner.
func NewDecodingRunner(tests *DecodingTests, baseDir, zebgpPath string) *DecodingRunner {
	return &DecodingRunner{
		tests:     tests,
		baseDir:   baseDir,
		zebgpPath: zebgpPath,
		colors:    NewColors(),
	}
}

// Run executes selected tests.
func (r *DecodingRunner) Run(ctx context.Context, verbose, quiet bool) bool {
	selected := r.tests.Selected()
	if len(selected) == 0 {
		fmt.Println("No tests selected")
		return true
	}

	passed, failed := 0, 0

	for _, test := range selected {
		test.State = StateRunning
		start := time.Now()

		success := r.runTest(ctx, test)
		test.Duration = time.Since(start)

		if success {
			test.State = StateSuccess
			passed++
			if !quiet {
				fmt.Printf("%s %s (%s)\n", r.colors.Green("✓"), test.Name, test.Duration.Truncate(time.Millisecond))
			}
		} else {
			test.State = StateFail
			failed++
			if !quiet {
				fmt.Printf("%s %s: %v\n", r.colors.Red("✗"), test.Name, test.Error)
				if verbose && test.ActualJSON != "" {
					r.printJSONDiff(test)
				}
			}
		}
	}

	// Summary
	if !quiet {
		fmt.Printf("\nDecoding tests: %d passed, %d failed\n", passed, failed)
	}

	return failed == 0
}

// runTest executes a single decoding test.
func (r *DecodingRunner) runTest(ctx context.Context, test *DecodingTest) bool {
	// Build command args
	args := []string{"decode"}

	switch test.Type {
	case "open":
		args = append(args, "--open")
	case "nlri":
		args = append(args, "--nlri")
	case "update":
		args = append(args, "--update")
	default:
		args = append(args, "--update") // Default to update
	}

	if test.Family != "" {
		args = append(args, "-f", test.Family)
	}

	args = append(args, test.HexPayload)

	// Run command
	cmd := exec.CommandContext(ctx, r.zebgpPath, args...) //nolint:gosec // Test runner, paths from temp dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		test.Error = fmt.Errorf("command failed: %w: %s", err, string(output))
		return false
	}

	test.ActualJSON = strings.TrimSpace(string(output))

	// Compare JSON (ignoring volatile fields)
	return r.compareJSON(test)
}

// compareJSON compares actual vs expected JSON, ignoring volatile fields.
func (r *DecodingRunner) compareJSON(test *DecodingTest) bool {
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
	volatileFields := []string{"exabgp", "zebgp", "time", "host", "pid", "ppid", "counter"}
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

	if string(actualBytes) != string(expectedBytes) {
		test.Error = fmt.Errorf("JSON mismatch")
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
func (r *DecodingRunner) printJSONDiff(test *DecodingTest) {
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

// Summary returns counts by state.
func (dt *DecodingTests) Summary() (passed, failed int) {
	for _, t := range dt.tests {
		switch t.State { //nolint:exhaustive // only count terminal states
		case StateSuccess:
			passed++
		case StateFail:
			failed++
		}
	}
	return
}
