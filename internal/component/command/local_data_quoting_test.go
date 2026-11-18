// Design: docs/architecture/api/commands.md -- shared operational value quoting.
// Related: invocation_execution_test.go -- unfinished quotes refuse before execution.

package command

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestServeLocalDecodesQuotedInvocationValues proves the local CLI route sends
// the same literal values as daemon execution, including an explicit empty
// required value. The real JSON pipe must deliver those values unchanged.
func TestServeLocalDecodesQuotedInvocationValues(t *testing.T) {
	const path = "show test local quoted content"
	var received string
	calls := 0
	t.Cleanup(AddLocalDataForTest(path, func(args ValidatedArgs) LocalAnswer {
		calls++
		value, found := args.Keyed("content")
		if !found {
			return LocalRefusal(1, "content was not bound")
		}
		received = value
		return LocalAnswer{Payload: map[string]string{"content": value}}
	}))
	withArgDefSource(t, func(string) ([]ArgDef, error) {
		return []ArgDef{mustArgDef(NewStringArg("content", []UintRange{{Min: 0, Max: 256}}, nil,
			ArgOptions{Mandatory: true, KeywordOnly: true}))}, nil
	})
	for _, test := range []struct {
		name, command, spelling, literal string
	}{
		{"unquoted base64", path, "c2V0IGJncCByb3V0ZXItaWQgMjAzLjAuMTEzLjEy", "c2V0IGJncCByb3V0ZXItaWQgMjAzLjAuMTEzLjEy"},
		{"quoted base64", path, `"c2V0IGJncCByb3V0ZXItaWQgMjAzLjAuMTEzLjEy"`, "c2V0IGJncCByb3V0ZXItaWQgMjAzLjAuMTEzLjEy"},
		{"explicit empty", path, `""`, ""},
		{"spaces retained", path, `" leading and trailing "`, " leading and trailing "},
		{"escaped literal bytes", path, `"quoted \"value\" \\ path\nline\tend\r"`, "quoted \"value\" \\ path\nline\tend\r"},
		{"quoted pipe is data", path, `"left | count"`, "left | count"},
		{"quoted registered path", `show "test" local quoted content`, `"literal value"`, "literal value"},
	} {
		t.Run(test.name, func(t *testing.T) {
			before := calls
			received = "not called"
			answer, code, served := ServeLocal(test.command+" content "+test.spelling+" | json compact", "")
			if !served {
				t.Fatal("registered local command was not served")
			}
			if code != 0 {
				t.Fatalf("valid invocation exited %d: %s", code, answer)
			}
			if calls != before+1 {
				t.Fatalf("producer calls=%d, want %d", calls, before+1)
			}
			if received != test.literal {
				t.Errorf("producer received %q, want literal %q", received, test.literal)
			}
			var delivered map[string]string
			if err := json.Unmarshal([]byte(answer), &delivered); err != nil {
				t.Fatalf("real pipe delivered invalid JSON: %v: %q", err, answer)
			}
			if len(delivered) != 1 {
				t.Fatalf("pipe delivered fields=%v, want only content", delivered)
			}
			if delivered["content"] != test.literal {
				t.Errorf("pipe delivered %q, want literal %q", delivered["content"], test.literal)
			}
		})
	}
}

// TestServeLocalRefusesMalformedEscapesBeforeProducer proves the local route
// uses execution's strict escape grammar and never echoes the private operand.
func TestServeLocalRefusesMalformedEscapesBeforeProducer(t *testing.T) {
	const path = "show test local malformed content"
	calls := 0
	t.Cleanup(AddLocalDataForTest(path, func(ValidatedArgs) LocalAnswer {
		calls++
		return LocalAnswer{Payload: map[string]bool{"called": true}}
	}))
	withArgDefSource(t, func(string) ([]ArgDef, error) { return nil, nil })
	for _, suffix := range []string{`"private-operand\q"`, `private-operand\value`, `"private-operand\`} {
		answer, code, served := ServeLocal(path+" content "+suffix+" | json compact", "")
		if !served {
			t.Fatal("registered command did not own its malformed invocation")
		}
		if code != 1 {
			t.Errorf("malformed escape exited %d, want 1", code)
		}
		if calls != 0 {
			t.Errorf("malformed escape reached producer %d times", calls)
		}
		if !IsPipeError(answer) {
			t.Errorf("malformed escape did not produce a pipe refusal: %q", answer)
		}
		if strings.Contains(answer, "private-operand") {
			t.Errorf("refusal echoed the private operand: %q", answer)
		}
	}
}
