// Design: docs/architecture/api/commands.md -- complete operational invocations
// Related: invocation_scan.go -- partial input remains valid while completing

package command

import (
	"strings"
	"testing"
)

// TestExecutionPipelinesRefuseUnfinishedQuote exercises every public pipeline
// preparation route that can dispatch a command or open a monitor without the
// daemon's Dispatcher. They must share execution's complete-input requirement,
// while ScanInvocation continues serving completion's partial input.
// VALIDATES: spec-web-cli-config-one-off-defects AC-7.
// PREVENTS: protecting daemon dispatch while local and streaming paths run it.
func TestExecutionPipelinesRefuseUnfinishedQuote(t *testing.T) {
	const input = `show quote "unfinished`
	for _, route := range []struct {
		name    string
		prepare func(string) string
	}{
		{"remote one-shot", func(input string) string {
			_, _, message := ProcessPipesDefaultFormatChecked(input, "")
			return message
		}},
		{"local one-shot", func(input string) string {
			_, _, message := ProcessPipesDefaultFormatLocal(input, "")
			return message
		}},
		{"one-shot result", func(input string) string {
			_, _, message := ProcessPipesDefaultFormatResult(input, "")
			return message
		}},
		{"local stream", func(input string) string {
			_, _, _, saves, message := ProcessStreamPipes(input, "")
			if saves != nil {
				if err := saves.Abort(); err != nil {
					return err.Error()
				}
			}
			return message
		}},
		{"remote stream", func(input string) string {
			_, _, _, message := ProcessRemoteStreamPipes(input, "")
			return message
		}},
		{"stream default renderer", func(input string) string {
			_, _, saves, message := ProcessStreamPipesDefaultFunc(input, func(answer string) string { return answer })
			if saves != nil {
				if err := saves.Abort(); err != nil {
					return err.Error()
				}
			}
			return message
		}},
	} {
		t.Run(route.name, func(t *testing.T) {
			message := route.prepare(input)
			if !strings.Contains(message, "column 12") {
				t.Errorf("unfinished quote refusal = %q, want its opening column", message)
			}
		})
	}
}

// TestServeLocalRefusesUnfinishedQuoteBeforeProducer verifies the CLI's local
// execution route itself, beyond a preparer's return value. A handler accepting
// all arguments must still remain untouched by an unfinished quoted value.
func TestServeLocalRefusesUnfinishedQuoteBeforeProducer(t *testing.T) {
	const path = "show quote"
	calls := 0
	t.Cleanup(AddLocalDataForTest(path, func(ValidatedArgs) LocalAnswer {
		calls++
		return LocalAnswer{Payload: map[string]any{"answered": true}}
	}))
	withArgDefSource(t, func(string) ([]ArgDef, error) { return nil, nil })
	answer, code, served := ServeLocal(path+` "unfinished`, "")
	if !served {
		t.Fatal("the registered local command must handle its malformed invocation")
	}
	if code != 1 {
		t.Errorf("unfinished quote exit code = %d, want 1", code)
	}
	if calls != 0 {
		t.Errorf("unfinished quote reached the local producer %d times", calls)
	}
	if !strings.Contains(answer, "column 12") {
		t.Errorf("unfinished quote refusal = %q, want its opening column", answer)
	}
}
