package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/config"
)

// TestShowConfigDumpMasksASecretInTheProducer proves `show config dump` masks a
// ze:sensitive value inside the handler, so every route that runs it answers
// the masked form and never the cleartext.
// PREVENTS: a secret masked only by the text printer, readable through
// `| json` or a remote surface.
// Method: dump a config carrying an md5 password, in the default and the
// --strip-private mode, and search the JSON encoding of each whole payload.
func TestShowConfigDumpMasksASecretInTheProducer(t *testing.T) {
	file := writeTestConfig(t, testConfigSecretBase)

	for _, mode := range []struct {
		name        string
		args        []string
		placeholder bool
	}{
		{name: "default", args: []string{file}},
		{name: "strip-private", args: []string{"--strip-private", file}, placeholder: true},
	} {
		t.Run(mode.name, func(t *testing.T) {
			answer := command.AnswerLocal(dataDump, commandtest.Args(mode.args...))
			if answer.Code != 0 || answer.Payload == nil {
				t.Fatalf("show config dump = code %d, diagnostic %q", answer.Code, answer.Diagnostic)
			}
			encoded, err := json.Marshal(answer.Payload)
			if err != nil {
				t.Fatalf("encode: %v", err)
			}
			text := string(encoded)
			if !strings.Contains(text, "password") {
				t.Fatalf("the dump carries no password leaf, so this case proves nothing: %s", text)
			}
			if strings.Contains(text, diffStoredPassword) {
				t.Errorf("the dump answers the cleartext password: %s", text)
			}
			if mode.placeholder && !strings.Contains(text, config.SecretDataPlaceholder) {
				t.Errorf("--strip-private answers no placeholder: %s", text)
			}
		})
	}
}

// TestConfigDataRefusalsAreReturned proves each refusal of the config data
// handlers comes back in the answer, with the exact lines the operator reads,
// rather than being written by the handler.
// PREVENTS: a refusal printed on the daemon's stderr when a daemon route runs
// the handler, which leaves the caller an empty failure.
// Method: one call per refusal, comparing payload, code and Diagnostic whole.
func TestConfigDataRefusalsAreReturned(t *testing.T) {
	tests := []struct {
		name    string
		handler command.LocalDataHandler
		args    []string
		code    int
		reason  string
	}{
		{
			name: "dump with no file", handler: dataDump, code: 1,
			reason: "error: show config dump needs a config file to read (use - for stdin).\n" +
				"To see the configuration the running daemon applied, run:\n" +
				"  show config running\n",
		},
		{
			name: "list with an argument", handler: dataList, args: []string{"extra"}, code: exitError,
			reason: "usage: show config list\n",
		},
		{
			name: "history with no file", handler: dataHistory, code: exitError,
			reason: "error: requires exactly one config file\n",
		},
		{
			name: "history of stdin", handler: dataHistory, args: []string{"-"}, code: exitError,
			reason: "error: history needs on-disk revision history; a config read from stdin (\"-\") has none\n",
		},
		{
			name: "validation with no file", handler: dataValidate, code: exitError,
			reason: "error: missing config file (use - for stdin)\n",
		},
		{
			name: "validation with two files", handler: dataValidate, args: []string{"a.conf", "b.conf"}, code: exitError,
			reason: "error: show config validation takes one config file, got 2\n",
		},
		{
			name: "diff with one file", handler: dataDiff, args: []string{"a.conf"}, code: exitError,
			reason: "error: requires exactly two config files, or revision number and config file\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			answer := tt.handler(commandtest.Args(tt.args...))
			if answer.Payload != nil {
				t.Errorf("a refusal answered a payload: %#v", answer.Payload)
			}
			if answer.Code != tt.code {
				t.Errorf("exit code = %d, want %d", answer.Code, tt.code)
			}
			if answer.Diagnostic != tt.reason {
				t.Errorf("diagnostic:\n got %q\nwant %q", answer.Diagnostic, tt.reason)
			}
		})
	}
}
