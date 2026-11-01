package env

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/core/env"
)

// TestShowEnvMasksASecretInTheProducer proves `show env get` and
// `show env list` mask a secret variable inside the handler, so every route
// that runs it (the local CLI and any daemon route) answers the mask and never
// the value.
// PREVENTS: a secret masked only by one renderer, readable through `| json` or
// a remote surface.
// Method: set a value on a variable registered Secret, then read the row
// both handlers answer and the JSON encoding of each whole payload.
func TestShowEnvMasksASecretInTheProducer(t *testing.T) {
	const key = "ze.test.environ.token"
	const value = "hunter2-must-not-leak"
	if err := env.Set(key, value); err != nil {
		t.Fatalf("set %s: %v", key, err)
	}

	answer := dataGet(commandtest.Args(key))
	if answer.Code != 0 || answer.Diagnostic != "" {
		t.Fatalf("show env get %s = code %d, diagnostic %q", key, answer.Code, answer.Diagnostic)
	}
	payload, ok := answer.Payload.(map[string]any)
	if !ok {
		t.Fatalf("show env get answers %T, want a map", answer.Payload)
	}
	rows, ok := payload[keyVariables].([]envRow)
	if !ok || len(rows) != 1 {
		t.Fatalf("show env get rows = %#v, want one row", payload[keyVariables])
	}
	if rows[0].Current != "****" {
		t.Errorf("secret current value = %q, want ****", rows[0].Current)
	}

	for name, handler := range map[string]command.LocalDataHandler{"get": dataGet, "list": dataList} {
		encoded, err := json.Marshal(handler(commandtest.Args(key)).Payload)
		if err != nil {
			t.Fatalf("show env %s: encode: %v", name, err)
		}
		if strings.Contains(string(encoded), value) {
			t.Errorf("show env %s carries the secret value: %s", name, encoded)
		}
	}
}

// TestShowEnvGetRefusalsAreReturned proves each refusal of `show env get`
// comes back in the answer, with the exact line the operator reads, rather
// than being written by the handler.
// Method: the two refusals, compared whole.
func TestShowEnvGetRefusalsAreReturned(t *testing.T) {
	tests := []struct {
		name   string
		args   []string
		reason string
	}{
		{name: "no key", reason: "error: show env get requires a key\n"},
		{name: "unknown key", args: []string{"ze.test.environ.absent"}, reason: "error: no environment variable named ze.test.environ.absent\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			answer := dataGet(commandtest.Args(tt.args...))
			if answer.Payload != nil || answer.Code != 1 || answer.Diagnostic != tt.reason {
				t.Errorf("answer = %+v, want nil payload, code 1, %q", answer, tt.reason)
			}
		})
	}
}
