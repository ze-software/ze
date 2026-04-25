// Design: docs/architecture/testing/ci-format.md -- web browser test parser
// Related: runner.go -- .wb test execution and browser control
// Related: expect.go -- expectation checking
//
// Package webtesting provides a declarative test framework for the web interface.
// Tests are written as .wb files with action= and expect= directives, executed
// against a headless browser via agent-browser.
package webtesting

import (
	"fmt"
	"strings"
)

// WBStepType identifies the kind of step in a web browser test case.
type WBStepType int

const (
	// WBStepAction is a browser action (navigate, click, fill).
	WBStepAction WBStepType = iota
	// WBStepExpect is a browser state assertion.
	WBStepExpect
)

// WBStep is an ordered entry in the test execution sequence.
type WBStep struct {
	Type        WBStepType
	ActionIndex int
	ExpectIndex int
}

// WBAction represents a browser action (navigate, click, fill, press, wait).
type WBAction struct {
	Kind   string            // open, click, fill, press, hover, wait, screenshot
	Values map[string]string // key=value pairs (path, text, ref, value, file, ms, load)
	Line   int               // source line number for error reporting
}

// WBExpectation represents a browser state assertion.
type WBExpectation struct {
	Kind   string            // element, breadcrumb, url, title, count
	Values map[string]string // key=value pairs (text, not-text, contains, not-contains, min)
	Line   int
}

// WBTestCase holds a parsed .wb test file.
type WBTestCase struct {
	Actions    []WBAction
	Expects    []WBExpectation
	Steps      []WBStep
	Timeout    string // from option=timeout:value=
	SkipReason string // from option=skip:reason=...; non-empty means skip the test
	Comments   []string
}

// ParseWBFile parses a .wb file content into a WBTestCase.
func ParseWBFile(content string) (*WBTestCase, error) {
	tc := &WBTestCase{
		Timeout: "30s",
	}

	for lineNum, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			if strings.HasPrefix(line, "# ") {
				tc.Comments = append(tc.Comments, line)
			}
			continue
		}

		directive, rest, found := strings.Cut(line, "=")
		if !found {
			return nil, fmt.Errorf("line %d: missing '=' in directive: %s", lineNum+1, line)
		}

		var err error
		switch directive {
		case "option":
			err = parseWBOption(tc, rest, lineNum+1)
		case "action":
			err = parseWBAction(tc, rest, lineNum+1)
		case "expect":
			err = parseWBExpect(tc, rest, lineNum+1)
		default: // unknown directive -- fail immediately
			return nil, fmt.Errorf("line %d: unknown directive %q", lineNum+1, directive)
		}
		if err != nil {
			return nil, err
		}
	}

	return tc, nil
}

func parseWBOption(tc *WBTestCase, rest string, line int) error {
	kv := parseWBKV(rest)
	kind := extractWBKind(rest)
	switch kind {
	case "timeout":
		if v, ok := kv["value"]; ok {
			tc.Timeout = v
		}
		return nil
	case "skip":
		// option=skip:reason=<text> marks the test as skipped by the
		// runner. Used for .wb tests that require an out-of-band
		// environment (e.g. an env var) the runner does not set.
		// Reason is surfaced in the runner output so the operator can
		// invoke the test manually when prerequisites are met.
		if v, ok := kv["reason"]; ok {
			tc.SkipReason = v
		} else {
			tc.SkipReason = "skipped"
		}
		return nil
	}
	return fmt.Errorf("line %d: unknown option %q", line, rest)
}

func parseWBAction(tc *WBTestCase, rest string, line int) error {
	kind := extractWBKind(rest)
	if kind == "" {
		return fmt.Errorf("line %d: action missing kind", line)
	}

	a := WBAction{Kind: kind, Values: parseWBKV(rest), Line: line}
	tc.Actions = append(tc.Actions, a)
	tc.Steps = append(tc.Steps, WBStep{Type: WBStepAction, ActionIndex: len(tc.Actions) - 1})
	return nil
}

func parseWBExpect(tc *WBTestCase, rest string, line int) error {
	kind := extractWBKind(rest)
	if kind == "" {
		return fmt.Errorf("line %d: expect missing kind", line)
	}

	e := WBExpectation{Kind: kind, Values: parseWBKV(rest), Line: line}
	tc.Expects = append(tc.Expects, e)
	tc.Steps = append(tc.Steps, WBStep{Type: WBStepExpect, ExpectIndex: len(tc.Expects) - 1})
	return nil
}

// extractWBKind returns the first segment before ':' (e.g., "click" from "click:text=BGP").
func extractWBKind(s string) string {
	kind, _, found := strings.Cut(s, ":")
	if !found {
		return s
	}
	return kind
}

// parseWBKV splits "kind:key1=val1:key2=val2" into a map (excluding the kind).
func parseWBKV(s string) map[string]string {
	m := make(map[string]string)
	parts := strings.Split(s, ":")
	for _, p := range parts[1:] { // skip kind
		if k, v, ok := strings.Cut(p, "="); ok {
			m[k] = v
		}
	}
	return m
}
