// VALIDATES: the operator error type of spec-operator-errors-name-the-fix
// (TDD plan row TestOperatorErrorRendersThreeParts): every boundary renders the
// problem, the why and the fix; the constructors refuse an empty problem,
// why or fix; errors.Is and errors.As see through the type to its cause.
// PREVENTS: a refusal that names no fix, and a wrap that hides its cause.
package opererr

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"strings"
	"testing"
)

const (
	testProblem = "all TACACS+ servers failed for login"
	testWhy     = "ze cannot check the password, so the login is refused"
	testFix     = "ze show tacacs servers"
)

// TestOperatorErrorRendersThreeParts proves each boundary's form carries the
// three parts. Method: build one error with a cause and read it through
// Error() (Go chain, API, plugin RPC, CLI), through a slog text handler (the
// log line), and through the accessors (the web notice).
func TestOperatorErrorRendersThreeParts(t *testing.T) {
	cause := errors.New("dial tcp 192.0.2.1:49: connect: connection refused")
	e := Wrap(cause, testProblem, testWhy, testFix)

	t.Run("error text", func(t *testing.T) {
		want := testProblem + "\n" + testWhy + ": " + cause.Error() + "\n" + "fix: " + testFix
		if got := e.Error(); got != want {
			t.Fatalf("Error():\n%s\nwant:\n%s", got, want)
		}
	})

	t.Run("log line", func(t *testing.T) {
		var out bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&out, &slog.HandlerOptions{
			ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
				if a.Key == slog.TimeKey {
					return slog.Attr{}
				}
				return a
			},
		}))
		logger.LogAttrs(t.Context(), slog.LevelWarn, e.Problem(), e.LogAttrs(slog.String("code", "test-code"))...)
		line := out.String()
		if strings.Count(line, "\n") != 1 {
			t.Fatalf("log form is not one line:\n%s", line)
		}
		for _, want := range []string{
			`msg="` + testProblem + `"`,
			`why="` + testWhy + `"`,
			`fix="` + testFix + `"`,
			`error="` + cause.Error() + `"`,
			`code=test-code`,
		} {
			if !strings.Contains(line, want) {
				t.Errorf("log line lacks %s:\n%s", want, line)
			}
		}
	})

	t.Run("web notice accessors", func(t *testing.T) {
		if e.Problem() != testProblem {
			t.Errorf("Problem() = %q, want %q", e.Problem(), testProblem)
		}
		if e.Why() != testWhy {
			t.Errorf("Why() = %q, want %q", e.Why(), testWhy)
		}
		if e.Fix() != testFix {
			t.Errorf("Fix() = %q, want %q", e.Fix(), testFix)
		}
	})
}

// TestOperatorErrorFixOnItsOwnLine proves the fix is the last line, alone
// with its label, even when an outer layer prepends context to the first line.
// Method: wrap the operator error with fmt.Errorf and split the text by line.
func TestOperatorErrorFixOnItsOwnLine(t *testing.T) {
	e := New(testProblem, testWhy, testFix)
	wrapped := fmt.Errorf("reload: %w", e)
	for _, text := range []string{e.Error(), wrapped.Error()} {
		lines := strings.Split(text, "\n")
		if len(lines) != 3 {
			t.Fatalf("want 3 lines (problem, why, fix), got %d:\n%s", len(lines), text)
		}
		if lines[2] != "fix: "+testFix {
			t.Errorf("last line = %q, want the fix alone: %q", lines[2], "fix: "+testFix)
		}
		if strings.Contains(lines[0], testFix) {
			t.Errorf("first line carries the fix: %q", lines[0])
		}
	}
}

// TestOperatorErrorCauseFollowsWhy proves the cause, when present, follows the
// why on line 2, and that no cause (a nil one to Wrap included) leaves the why
// alone there, the fix line unchanged. Method: compare the rendered text for
// each case.
func TestOperatorErrorCauseFollowsWhy(t *testing.T) {
	cause := errors.New("connection refused")
	cases := []struct {
		name string
		err  *Error
		want string
	}{
		{"with cause", Wrap(cause, testProblem, testWhy, testFix), testProblem + "\n" + testWhy + ": connection refused\nfix: " + testFix},
		{"no cause", New(testProblem, testWhy, testFix), testProblem + "\n" + testWhy + "\nfix: " + testFix},
		{"nil cause", Wrap(nil, testProblem, testWhy, testFix), testProblem + "\n" + testWhy + "\nfix: " + testFix},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.err.Error(); got != tc.want {
				t.Fatalf("Error():\n%s\nwant:\n%s", got, tc.want)
			}
		})
	}
}

// TestOperatorErrorConstructorRefusesEmptyParts proves New and Wrap refuse an
// empty problem, an empty why and an empty fix as a Ze defect. Method: recover the panic and
// read its BUG text.
func TestOperatorErrorConstructorRefusesEmptyParts(t *testing.T) {
	cases := []struct {
		name  string
		build func()
		want  string
	}{
		{"New empty problem", func() { _ = New("", testWhy, testFix) }, "BUG: opererr.New called with an empty problem"},
		{"New empty why", func() { _ = New(testProblem, "", testFix) }, "BUG: opererr.New called with an empty why for: " + testProblem},
		{"New empty fix", func() { _ = New(testProblem, testWhy, "") }, "BUG: opererr.New called with an empty fix for: " + testProblem},
		{"Wrap empty problem", func() { _ = Wrap(errors.New("x"), "", testWhy, testFix) }, "BUG: opererr.New called with an empty problem"},
		{"Wrap empty why", func() { _ = Wrap(errors.New("x"), testProblem, "", testFix) }, "BUG: opererr.New called with an empty why for: " + testProblem},
		{"Wrap empty fix", func() { _ = Wrap(errors.New("x"), testProblem, testWhy, "") }, "BUG: opererr.New called with an empty fix for: " + testProblem},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				got, ok := recover().(string)
				if !ok {
					t.Fatalf("no string panic, want %q", tc.want)
				}
				if got != tc.want {
					t.Fatalf("panic %q, want %q", got, tc.want)
				}
			}()
			tc.build()
		})
	}
}

// pathFault is a typed cause, so errors.As has a target type to find.
type pathFault struct{ path string }

func (p *pathFault) Error() string { return "no file at " + p.path }

// TestOperatorErrorUnwrapReachesCause proves errors.Is and errors.As see
// through the operator error to its cause, and find the operator error inside
// an outer wrap. Method: wrap a sentinel and a typed cause, then wrap the
// operator error again with fmt.Errorf.
func TestOperatorErrorUnwrapReachesCause(t *testing.T) {
	sentinel := Wrap(fmt.Errorf("open ze.conf: %w", fs.ErrNotExist), "the config file is missing", "ze cannot load a config it cannot read", "ze config edit")
	outer := fmt.Errorf("reload: %w", sentinel)
	if !errors.Is(outer, fs.ErrNotExist) {
		t.Errorf("errors.Is does not reach fs.ErrNotExist through %q", outer)
	}

	var target *Error
	if !errors.As(outer, &target) {
		t.Fatalf("errors.As finds no *Error in %q", outer)
	}
	if target.Fix() != "ze config edit" {
		t.Errorf("errors.As found Fix() = %q", target.Fix())
	}

	typed := Wrap(&pathFault{path: "/etc/ze/ze.conf"}, "the config file is missing", "ze cannot load a config it cannot read", "ze config edit")
	var fault *pathFault
	if !errors.As(typed, &fault) {
		t.Fatalf("errors.As finds no *pathFault in %q", typed)
	}
	if fault.path != "/etc/ze/ze.conf" {
		t.Errorf("errors.As found path %q", fault.path)
	}

	if New(testProblem, testWhy, testFix).Unwrap() != nil {
		t.Errorf("New built an error with a cause")
	}
}

// TestOperatorErrorZeroValueNamesTheDefect proves a zero Error, which only a
// composite literal builds, never renders an empty refusal. Method: render a
// zero value through Error and LogAttrs.
func TestOperatorErrorZeroValueNamesTheDefect(t *testing.T) {
	var e Error
	if got := e.Error(); got != zeroText {
		t.Errorf("zero Error() = %q, want %q", got, zeroText)
	}
	attrs := e.LogAttrs()
	if len(attrs) != 1 || attrs[0].Key != "error" || attrs[0].Value.String() != zeroText {
		t.Errorf("zero LogAttrs() = %v, want one error attribute %q", attrs, zeroText)
	}
}

// TestOperatorErrorLogAttrsInventsNoCode proves LogAttrs adds a "code"
// attribute only when the caller passes one, and passes it through unchanged.
// Method: read the attribute keys with and without a caller attribute.
func TestOperatorErrorLogAttrsInventsNoCode(t *testing.T) {
	e := New(testProblem, testWhy, testFix)
	for _, a := range e.LogAttrs() {
		if a.Key == "code" {
			t.Fatalf("LogAttrs() invented a code: %v", a)
		}
	}
	attrs := e.LogAttrs(slog.String("code", "tacacs-unreachable"))
	last := attrs[len(attrs)-1]
	if last.Key != "code" || last.Value.String() != "tacacs-unreachable" {
		t.Fatalf("caller code not passed through: %v", attrs)
	}
	if e.Fix() != testFix {
		t.Errorf("Fix() = %q, want the command alone %q", e.Fix(), testFix)
	}
}
