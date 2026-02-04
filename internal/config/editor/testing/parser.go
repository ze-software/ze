// Package testing provides a replay-based testing framework for the Ze configuration editor.
package testing

import (
	"bufio"
	"fmt"
	"strings"
)

// StepType identifies the type of a test step.
type StepType int

const (
	StepInput StepType = iota
	StepExpect
	StepWait
)

// Step represents a single ordered step in the test (input, expect, or wait).
type Step struct {
	Type        StepType
	InputIndex  int // Index into Inputs slice (if Type == StepInput)
	ExpectIndex int // Index into Expects slice (if Type == StepExpect)
	WaitIndex   int // Index into Waits slice (if Type == StepWait)
}

// TestCase represents a parsed .et (Editor Test) file.
type TestCase struct {
	Tmpfs   []TmpfsBlock  // Embedded files
	Options []Option      // Test options
	Inputs  []InputAction // User input actions (in order)
	Expects []Expectation // Expectations (in order)
	Waits   []WaitAction  // Wait actions
	Steps   []Step        // Ordered sequence of steps (preserves interleaving)
}

// TmpfsBlock represents an embedded file.
type TmpfsBlock struct {
	Path    string // Relative file path
	Mode    string // File mode (e.g., "755"), empty for default
	Content string // File content
}

// Option represents a test configuration option.
type Option struct {
	Type   string            // Option type: "file", "timeout", "width", "height"
	Values map[string]string // Key-value pairs
}

// InputAction represents a user input action.
type InputAction struct {
	Action string            // Action type: "type", "key", "ctrl"
	Values map[string]string // Key-value pairs (e.g., "text", "name", "key")
}

// Expectation represents an assertion to verify.
type Expectation struct {
	Type   string            // Type: "context", "completion", "content", "errors", "dirty", etc.
	Values map[string]string // Key-value pairs
}

// WaitAction represents a wait/timing action.
type WaitAction struct {
	Values map[string]string // Key-value pairs (e.g., "ms", "validation", "timer")
}

// shorthandKeys maps single-word input shortcuts to key names.
var shorthandKeys = map[string]bool{
	"tab":       true,
	"enter":     true,
	"up":        true,
	"down":      true,
	"left":      true,
	"right":     true,
	"esc":       true,
	"backspace": true,
	"delete":    true,
	"home":      true,
	"end":       true,
	"pgup":      true,
	"pgdn":      true,
	"space":     true,
}

// validActions lists the recognized action types for .et files.
var validActions = map[string]bool{
	"tmpfs":  true,
	"option": true,
	"input":  true,
	"expect": true,
	"wait":   true,
}

// ParseETFile parses an .et (Editor Test) file content.
func ParseETFile(content string) (*TestCase, error) {
	tc := &TestCase{
		Tmpfs:   make([]TmpfsBlock, 0),
		Options: make([]Option, 0),
		Inputs:  make([]InputAction, 0),
		Expects: make([]Expectation, 0),
		Waits:   make([]WaitAction, 0),
		Steps:   make([]Step, 0),
	}

	scanner := bufio.NewScanner(strings.NewReader(content))
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Skip blank lines and comments
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		// Parse line
		if err := tc.parseLine(trimmed, scanner, &lineNum); err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNum, err)
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading input: %w", err)
	}

	return tc, nil
}

// parseLine parses a single line, handling multi-line blocks like tmpfs.
func (tc *TestCase) parseLine(line string, scanner *bufio.Scanner, lineNum *int) error {
	// Split on first = to get action type
	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid line format, expected action=...: %s", line)
	}

	action := parts[0]
	rest := parts[1]

	// Validate action type - reject unknown actions
	if !validActions[action] {
		return fmt.Errorf("unknown action type: %s", action)
	}

	// Dispatch to handler
	if action == "tmpfs" {
		return tc.parseTmpfs(rest, scanner, lineNum)
	}
	if action == "option" {
		return tc.parseOption(rest)
	}
	if action == "input" {
		return tc.parseInput(rest)
	}
	if action == "expect" {
		return tc.parseExpect(rest)
	}
	if action == "wait" {
		return tc.parseWait(rest)
	}

	// Should never reach here due to validActions check above
	return fmt.Errorf("unhandled action type: %s", action)
}

// parseTmpfs parses a tmpfs block with multi-line content.
func (tc *TestCase) parseTmpfs(rest string, scanner *bufio.Scanner, lineNum *int) error {
	// Parse header: path[:mode=xxx]:terminator=TERM
	block := TmpfsBlock{}

	// First segment is the path
	segments := strings.Split(rest, ":")
	if len(segments) < 2 {
		return fmt.Errorf("tmpfs requires at least path and terminator")
	}

	block.Path = segments[0]
	var terminator string

	// Parse remaining segments
	for _, seg := range segments[1:] {
		kv := strings.SplitN(seg, "=", 2)
		if len(kv) == 2 {
			switch kv[0] {
			case "mode":
				block.Mode = kv[1]
			case "terminator":
				terminator = kv[1]
			}
		}
	}

	if terminator == "" {
		return fmt.Errorf("tmpfs requires terminator")
	}

	// Read content until terminator
	var content strings.Builder
	found := false

	for scanner.Scan() {
		*lineNum++
		line := scanner.Text()

		if line == terminator {
			found = true
			break
		}

		if content.Len() > 0 {
			content.WriteString("\n")
		}
		content.WriteString(line)
	}

	if !found {
		return fmt.Errorf("missing terminator %q", terminator)
	}

	block.Content = content.String()
	tc.Tmpfs = append(tc.Tmpfs, block)
	return nil
}

// parseOption parses an option line.
func (tc *TestCase) parseOption(rest string) error {
	opt := Option{
		Values: make(map[string]string),
	}

	// First segment is the type
	segments := strings.Split(rest, ":")
	if len(segments) < 1 {
		return fmt.Errorf("option requires type")
	}

	opt.Type = segments[0]

	// Parse key=value pairs
	for _, seg := range segments[1:] {
		kv := strings.SplitN(seg, "=", 2)
		if len(kv) == 2 {
			opt.Values[kv[0]] = kv[1]
		} else {
			// Flag without value
			opt.Values[kv[0]] = ""
		}
	}

	tc.Options = append(tc.Options, opt)
	return nil
}

// parseInput parses an input action line.
func (tc *TestCase) parseInput(rest string) error {
	inp := InputAction{
		Values: make(map[string]string),
	}

	// Check for shorthand keys (e.g., input=tab, input=enter)
	if shorthandKeys[rest] {
		inp.Action = "key"
		inp.Values["name"] = rest
		tc.Inputs = append(tc.Inputs, inp)
		tc.Steps = append(tc.Steps, Step{Type: StepInput, InputIndex: len(tc.Inputs) - 1})
		return nil
	}

	// Parse type:key=value:key=value...
	segments := strings.Split(rest, ":")
	if len(segments) < 1 {
		return fmt.Errorf("input requires action type")
	}

	inp.Action = segments[0]

	// Parse key=value pairs
	// For "type" action, the text may contain colons, so handle specially
	if inp.Action == "type" && len(segments) >= 2 {
		// Rejoin everything after type: as the text value might contain colons
		textPart := strings.Join(segments[1:], ":")
		kv := strings.SplitN(textPart, "=", 2)
		if len(kv) == 2 && kv[0] == "text" {
			inp.Values["text"] = kv[1]
		}
	} else {
		for _, seg := range segments[1:] {
			kv := strings.SplitN(seg, "=", 2)
			if len(kv) == 2 {
				inp.Values[kv[0]] = kv[1]
			} else {
				// Flag without value
				inp.Values[kv[0]] = ""
			}
		}
	}

	tc.Inputs = append(tc.Inputs, inp)
	tc.Steps = append(tc.Steps, Step{Type: StepInput, InputIndex: len(tc.Inputs) - 1})
	return nil
}

// parseExpect parses an expectation line.
func (tc *TestCase) parseExpect(rest string) error {
	exp := Expectation{
		Values: make(map[string]string),
	}

	// Parse type:key=value:key=value...
	segments := strings.Split(rest, ":")
	if len(segments) < 1 {
		return fmt.Errorf("expect requires type")
	}

	exp.Type = segments[0]

	// Parse key=value pairs
	// For some types, the value may contain colons, so handle specially
	if len(segments) >= 2 {
		// Check if this is a type that needs special handling (contains= might have colons in value)
		restPart := strings.Join(segments[1:], ":")

		// Try to parse as key=value first
		kv := strings.SplitN(restPart, "=", 2)
		if len(kv) == 2 {
			exp.Values[kv[0]] = kv[1]
		} else {
			// Flag without value (e.g., "root", "empty", "none", "true", "false", "active", "inactive", "visible", "hidden")
			exp.Values[kv[0]] = ""
		}
	}

	tc.Expects = append(tc.Expects, exp)
	tc.Steps = append(tc.Steps, Step{Type: StepExpect, ExpectIndex: len(tc.Expects) - 1})
	return nil
}

// parseWait parses a wait action line.
func (tc *TestCase) parseWait(rest string) error {
	w := WaitAction{
		Values: make(map[string]string),
	}

	// Parse type:value or just type
	segments := strings.Split(rest, ":")
	if len(segments) < 1 {
		return fmt.Errorf("wait requires type")
	}

	// First segment might be key=value or just key
	kv := strings.SplitN(segments[0], "=", 2)
	if len(kv) == 2 {
		w.Values[kv[0]] = kv[1]
	} else {
		// Check if there's a value in next segment
		if len(segments) >= 2 {
			w.Values[segments[0]] = segments[1]
		} else {
			// Flag without value (e.g., "validation")
			w.Values[segments[0]] = ""
		}
	}

	tc.Waits = append(tc.Waits, w)
	tc.Steps = append(tc.Steps, Step{Type: StepWait, WaitIndex: len(tc.Waits) - 1})
	return nil
}
