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
	"strconv"
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

// WBViewport is a browser viewport size from option=viewport:width=..:height=..
// The zero value means "use the harness default" (no resize).
type WBViewport struct {
	Width  int
	Height int
}

// WBAuthUser is a user the harness must seed and that a login action can use,
// from option=auth:user=..:password=..:role=.. (repeatable). A test that
// declares any auth user needs the server started with authentication enabled
// (not --insecure-web).
type WBAuthUser struct {
	Name     string
	Password string //nolint:gosec // G117: test-harness login password for a seeded .wb user, not a real secret
	Role     string // "", "admin", or "read-only"
}

// WBServer names the server a test drives. Ze serves three htmx interfaces from
// three different programs, and a browser proof of one of them has to start
// that one: the web UI is `ze start --web --web-only`, the looking glass is a
// listener of the `ze` daemon, and the chaos dashboard is `ze-chaos`.
type WBServer string

const (
	// WBServerWeb is the config and monitoring UI, and the default.
	WBServerWeb WBServer = "web"
	// WBServerLG is the looking glass. Its pages read `show bgp`, so
	// the harness gives it a daemon with a peer rather than a bare listener.
	WBServerLG WBServer = "lg"
	// WBServerChaos is the chaos dashboard, served by ze-chaos.
	WBServerChaos WBServer = "chaos"
	// WBServerLGNoEngine is the looking glass with an engine that always
	// fails, served by `ze-test lg`. A daemon cannot be asked for that state:
	// the looking glass dispatches in process, so a daemon with no BGP still
	// answers an empty peer list rather than an error.
	WBServerLGNoEngine WBServer = "lg-no-engine"
)

// WBEnvVar is one environment variable the harness sets on the server process,
// from option=env:var=..:value=.. (repeatable). Ze reads its settings through
// dotted names (`ze.web.ui`), so a test that needs a non-default mode names it
// here instead of being skipped for want of a way to ask.
type WBEnvVar struct {
	Var   string
	Value string
}

// WBTestCase holds a parsed .wb test file.
type WBTestCase struct {
	Actions    []WBAction
	Expects    []WBExpectation
	Steps      []WBStep
	Timeout    string       // from option=timeout:value=
	SkipReason string       // from option=skip:reason=...; non-empty means skip the test
	Viewport   WBViewport   // from option=viewport:width=..:height=..
	Locale     string       // from option=locale:lang=.. (sets Accept-Language)
	Auth       []WBAuthUser // from option=auth:.. (repeatable); non-empty => server needs auth
	Server     WBServer     // from option=server:kind=web|lg|lg-no-engine|chaos; empty means web
	Env        []WBEnvVar   // from option=env:var=..:value=.. (repeatable)
	Comments   []string
}

// RequiresAuth reports whether the test declared any auth user, meaning the
// harness must start the server with authentication enabled and seed the users.
func (tc *WBTestCase) RequiresAuth() bool { return len(tc.Auth) > 0 }

// ServerKind returns the server this test drives, defaulting to the web UI.
func (tc *WBTestCase) ServerKind() WBServer {
	if tc.Server == "" {
		return WBServerWeb
	}

	return tc.Server
}

// ParseWBFile parses a .wb file content into a WBTestCase.
func ParseWBFile(content string) (*WBTestCase, error) {
	tc := &WBTestCase{
		Timeout: "30s",
	}

	lineNum := 0
	for line := range strings.SplitSeq(content, "\n") {
		lineNum++
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			if strings.HasPrefix(line, "# ") {
				tc.Comments = append(tc.Comments, line)
			}
			continue
		}

		directive, rest, found := strings.Cut(line, "=")
		if !found {
			return nil, fmt.Errorf("line %d: missing '=' in directive: %s", lineNum, line)
		}

		var err error
		switch directive {
		case "option":
			err = parseWBOption(tc, rest, lineNum)
		case "action":
			err = parseWBAction(tc, rest, lineNum)
		case "expect":
			err = parseWBExpect(tc, rest, lineNum)
		default: // unknown directive -- fail immediately
			return nil, fmt.Errorf("line %d: unknown directive %q", lineNum, directive)
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
	case "viewport":
		// option=viewport:width=390:height=844 resizes the browser before the
		// first navigation so mobile-layout assertions run at that size.
		if v, ok := kv["width"]; ok {
			n, err := strconv.Atoi(v)
			if err != nil {
				return fmt.Errorf("line %d: viewport width %q: %w", line, v, err)
			}
			tc.Viewport.Width = n
		}
		if v, ok := kv["height"]; ok {
			n, err := strconv.Atoi(v)
			if err != nil {
				return fmt.Errorf("line %d: viewport height %q: %w", line, v, err)
			}
			tc.Viewport.Height = n
		}
		return nil
	case "locale":
		// option=locale:lang=fr sets the browser Accept-Language so the UI
		// renders under the proving locale.
		if v, ok := kv["lang"]; ok {
			tc.Locale = v
		}
		return nil
	case "auth":
		// option=auth:user=noc:password=secret:role=read-only (repeatable)
		// declares a user to seed; its presence makes the harness start the
		// server with authentication enabled instead of --insecure-web.
		u := WBAuthUser{Name: kv["user"], Password: kv["password"], Role: kv["role"]}
		if u.Name == "" {
			return fmt.Errorf("line %d: auth option requires user=", line)
		}
		tc.Auth = append(tc.Auth, u)
		return nil
	case "server":
		// option=server:kind=lg starts the looking glass instead of the web UI.
		// An unknown kind is a PARSE error: the harness would otherwise start
		// the default server and the test would assert against the wrong one,
		// which is a pass that proves nothing.
		kind := WBServer(kv["kind"])
		switch kind {
		case WBServerWeb, WBServerLG, WBServerLGNoEngine, WBServerChaos:
			tc.Server = kind

			return nil
		default:
			return fmt.Errorf("line %d: server kind %q is not one of web, lg, lg-no-engine, chaos", line, kv["kind"])
		}
	case "env":
		// option=env:var=ze.web.ui:value=finder (repeatable) reaches the
		// server process, never this one.
		if kv["var"] == "" {
			return fmt.Errorf("line %d: env option requires var=", line)
		}
		tc.Env = append(tc.Env, WBEnvVar{Var: kv["var"], Value: kv["value"]})
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
