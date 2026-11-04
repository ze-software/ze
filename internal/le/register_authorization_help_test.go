// Design: docs/architecture/core-design.md -- direct area help completes without admitted execution.
package le

import (
	"bytes"
	"maps"
	"os"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/le/commit"
	leaction "github.com/ze-software/ze/internal/le/le/action"
	"github.com/ze-software/ze/internal/le/le/human/humantest"
	"github.com/ze-software/ze/internal/le/rfc"
	"github.com/ze-software/ze/internal/le/spec"
)

// TestPreparedAuthorizationAreaHelpCompletesWithoutExecution calls each real
// prepared action through its direct area API, without root help interception.
// Help must print the declared page, return nil and zero, and leave the cold
// root unchanged. Panics are recorded per case so all four routes are judged.
func TestPreparedAuthorizationAreaHelpCompletesWithoutExecution(t *testing.T) {
	program := leaction.Program()
	leaction.SetProgram("le")
	t.Cleanup(func() { leaction.SetProgram(program) })
	cases := []struct {
		name, verb string
		answer     func([]string) (any, int)
		actions    func() leaction.List
	}{
		{"rfc-approve", "approve", rfc.Answer, rfc.Actions},
		{"commit-create", "create", commit.Answer, commit.Actions},
		{"commit-push", "push", commit.Answer, commit.Actions},
		{"spec-review-record", "record", spec.Review, func() leaction.List { return spec.ReviewArea().Actions() }},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			root := authorizationColdRoot(t)
			t.Setenv("ZE_REPO_ROOT", root)
			env.ResetCache()
			t.Cleanup(env.ResetCache)
			before := humantest.Snapshot(t, root)
			output, err := os.CreateTemp(t.TempDir(), "help-output-")
			if err != nil {
				t.Fatal(err)
			}
			diagnostic, err := os.CreateTemp(t.TempDir(), "help-diagnostic-")
			if err != nil {
				if closeErr := output.Close(); closeErr != nil {
					t.Error(closeErr)
				}
				t.Fatal(err)
			}
			stdout, stderr := os.Stdout, os.Stderr
			os.Stdout, os.Stderr = output, diagnostic
			t.Cleanup(func() {
				os.Stdout, os.Stderr = stdout, stderr
				for _, file := range []*os.File{output, diagnostic} {
					if err := file.Close(); err != nil {
						t.Error(err)
					}
				}
			})
			page, declared := test.actions().ActionPage("le", test.verb)
			if !declared {
				t.Fatal("prepared action has no declared help page")
			}
			page.Width = leaction.PageWidth(output)
			var expected bytes.Buffer
			page.WriteTo(&expected, slogutil.UseColor(output))
			payload, code, failure := authorizationAreaHelpAnswer(test.answer, []string{test.verb, "--help"})
			if failure != nil {
				t.Errorf("direct area help panicked instead of completing: %v", failure)
			}
			if payload != nil || code != 0 {
				t.Errorf("direct area help answered (%#v, %d), want (nil, 0)", payload, code)
			}
			actual, err := os.ReadFile(output.Name())
			if err != nil {
				t.Fatal(err)
			}
			if len(actual) == 0 || !bytes.Equal(actual, expected.Bytes()) {
				t.Errorf("direct area help output = %q, want declared page %q", actual, expected.Bytes())
			}
			messages, err := os.ReadFile(diagnostic.Name())
			if err != nil {
				t.Fatal(err)
			}
			if len(messages) != 0 {
				t.Errorf("successful help wrote diagnostic: %s", messages)
			}
			if !maps.Equal(before, humantest.Snapshot(t, root)) {
				t.Error("direct area help changed the cold root")
			}
		})
	}
}

// authorizationAreaHelpAnswer preserves a panic as a failed observation rather
// than stopping the table before the other real prepared routes are exercised.
func authorizationAreaHelpAnswer(answer func([]string) (any, int), words []string) (payload any, code int, failure any) {
	code = -1
	defer func() { failure = recover() }()
	payload, code = answer(words)
	return payload, code, nil
}
