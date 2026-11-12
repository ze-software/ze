package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/cliio"
	coreenv "github.com/ze-software/ze/internal/core/env"
)

// TestCmdEngineStepsValidatesFileAndStdin drives the real CLI handler through
// both input carriers. Unknown kinds must fail parsing, not a later TLS dial;
// valid kinds must pass parsing and reach the deliberately unconfigured dial.
func TestCmdEngineStepsValidatesFileAndStdin(t *testing.T) {
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if strings.ReplaceAll(strings.ToLower(key), "_", ".") == "ze.plugin.hub.token" {
			t.Setenv(key, "")
		}
	}
	t.Setenv("ZE_PLUGIN_HUB_TOKEN", "")
	coreenv.ResetCache()
	t.Cleanup(coreenv.ResetCache)
	originalLog := slog.Default()
	t.Cleanup(func() { slog.SetDefault(originalLog) })
	for _, one := range []struct {
		name string
		data string
		want string
	}{
		{"missing", `[{}]`, "unknown kind 0"},
		{"zero", `[{"kind":0}]`, "unknown kind 0"},
		{"unknown", `[{"kind":255}]`, "unknown kind 255"},
		{"valid-command", `[{"kind":1,"text":"show version"}]`, "TLS connect-back failed"},
		{"valid-stream", `[{"kind":2,"text":"monitor events"}]`, "TLS connect-back failed"},
	} {
		for _, carrier := range []string{"file", "stdin"} {
			t.Run(one.name+"/"+carrier, func(t *testing.T) {
				var output bytes.Buffer
				slog.SetDefault(slog.New(slog.NewTextHandler(&output, nil)))
				path := "-"
				if carrier == "file" {
					path = filepath.Join(t.TempDir(), "steps.json")
					if err := os.WriteFile(path, []byte(one.data), 0o600); err != nil {
						t.Fatal(err)
					}
				} else {
					t.Cleanup(cliio.SwapStreams(strings.NewReader(one.data), &output))
				}
				if code := CmdEngineSteps([]string{path}); code != 1 {
					t.Fatalf("exit = %d, want 1: %s", code, output.String())
				}
				if !strings.Contains(output.String(), one.want) {
					t.Fatalf("output = %q, want %q", output.String(), one.want)
				}
				if strings.HasPrefix(one.want, "unknown kind") {
					if !strings.Contains(output.String(), "parse steps file") {
						t.Fatalf("unknown kind did not fail parsing: %s", output.String())
					}
					if strings.Contains(output.String(), "TLS connect-back") {
						t.Fatalf("unknown kind reached TLS dial: %s", output.String())
					}
				}
			})
		}
	}
}

// TestPipedEngineDispatchSplitsTheChain proves an engine step's pipe chain is
// split off before the daemon sees the line, and applied to the answer.
//
// VALIDATES: `show doctor config <file> | json` reaches the daemon as `show
// doctor config <file>`, the doctor's daemon form, which refuses any token
// after the file name (readDoctorRequest), and the step reads JSON holding the
// daemon's answer. A line with no pipe reaches the daemon unchanged and its raw
// answer is returned byte for byte, so json= predicates keep walking it.
// PREVENTS: the whole line, "|" and "json" included, reaching the dispatcher
// as arguments, where a handler that reads them refuses the step and one that
// ignores them answers raw JSON whatever the chain asked for.
func TestPipedEngineDispatchSplitsTheChain(t *testing.T) {
	// Two top-level keys, so applyJSON's one-key-array unwrapping does not
	// apply and the rendered document must equal the answer exactly.
	const answer = `{"diagnostics":[{"code":"doctor-ipsec-unauthenticated-input","severity":"warning"}],"runtime-checked":true}`
	var sent []string
	dispatch := pipedEngineDispatch(func(_ context.Context, line string) (string, []byte, error) {
		sent = append(sent, line)
		return "done", []byte(answer), nil
	})

	status, output, err := dispatch(context.Background(), "show doctor config daemon-2/ze-config.conf | json")
	if err != nil {
		t.Fatalf("piped dispatch failed: %v", err)
	}
	if len(sent) != 1 || sent[0] != "show doctor config daemon-2/ze-config.conf" {
		t.Fatalf("daemon received %q, want the command alone", sent)
	}
	if status != "done" {
		t.Fatalf("status = %q, want the daemon's done", status)
	}
	// The raw answer is one compact line; | json indents it, so a newline is
	// the mark that the chain was applied rather than the answer passed through.
	if output == answer || !strings.Contains(output, "\n") {
		t.Fatalf("| json was not applied, output is the raw answer: %s", output)
	}
	var got, want any
	if err := json.Unmarshal([]byte(output), &got); err != nil {
		t.Fatalf("| json output is not JSON: %v\n%s", err, output)
	}
	if err := json.Unmarshal([]byte(answer), &want); err != nil {
		t.Fatalf("fixture answer: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("| json output holds %v, want the daemon's answer %v", got, want)
	}

	sent = nil
	status, output, err = dispatch(context.Background(), "show doctor config daemon-2/ze-config.conf")
	if err != nil {
		t.Fatalf("plain dispatch failed: %v", err)
	}
	if len(sent) != 1 || sent[0] != "show doctor config daemon-2/ze-config.conf" {
		t.Fatalf("daemon received %q, want the line unchanged", sent)
	}
	if status != "done" || output != answer {
		t.Fatalf("plain dispatch answered %q %q, want the raw answer unchanged", status, output)
	}
}

// TestPipedEngineDispatchRefusesABadChain proves a chain the pipe layer refuses
// fails the step before the daemon is asked, as `ze cli -c` exits 1 for it.
//
// VALIDATES: an unknown operator is an error naming the pipe fault, and nothing
// is dispatched.
// PREVENTS: a bad chain silently dropped, leaving the step to read an answer
// the operator never asked for.
func TestPipedEngineDispatchRefusesABadChain(t *testing.T) {
	called := false
	dispatch := pipedEngineDispatch(func(context.Context, string) (string, []byte, error) {
		called = true
		return "done", []byte(`{}`), nil
	})
	_, _, err := dispatch(context.Background(), "show doctor | bogus")
	if err == nil {
		t.Fatal("an unknown pipe operator was accepted")
	}
	if !strings.Contains(err.Error(), "bogus") {
		t.Fatalf("refusal %q does not name the operator", err)
	}
	if called {
		t.Fatal("the daemon was asked before the chain was judged")
	}
}
