package cli

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
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
