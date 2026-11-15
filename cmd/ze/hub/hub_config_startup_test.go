// Design: docs/guide/plugins.md -- validate explicit hub settings before startup
// Related: main.go -- Run and the early hub configuration refusal

package hub

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// TestRunHubConfigurationOutcomes drives the real daemon entry point. The NUL
// sentinel followed by EOF gives accepted configurations their normal ordered
// stop, so a missing refusal returns the wrong result instead of hanging.
// VALIDATES: invalid explicit transport is refused and logged, while valid
// explicit and omitted hub settings both reach successful startup and stop.
// PREVENTS: continuing startup after explicit hub settings were rejected.
func TestRunHubConfigurationOutcomes(t *testing.T) {
	const invalidSecret = "bmp-bind-reload-test-0123456789"
	cases := []struct {
		name    string
		content string
		want    int
	}{
		{
			name: "invalid explicit hub",
			content: `plugin {
    hub {
        server invalid-hub {
            ip 127.0.0.1
            port 0
            secret bmp-bind-reload-test-0123456789
        }
    }
}`,
			want: 1,
		},
		{
			name: "valid explicit hub",
			content: `plugin {
    hub {
        server valid-hub {
            ip 127.0.0.1
            port 0
            secret bmp-bind-reload-test-01234567890
        }
    }
}`,
		},
		{name: "omitted hub", content: "plugin {}\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			previousServer := registry.GetPluginServer()
			isolateRegistry(t)
			t.Cleanup(func() { registry.SetPluginServer(previousServer) })
			pinLogEnv(t)
			pinConfigDir(t)
			store := storage.BindConfigSource(newTestStore(t), "-", storage.ConfigSourceStdin)
			path := filepath.Join(t.TempDir(), "stdin.conf")
			if err := os.WriteFile(path, []byte(tc.content+"\x00"), 0o600); err != nil {
				t.Fatal(err)
			}
			input, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			original := os.Stdin
			os.Stdin = input
			t.Cleanup(func() {
				os.Stdin = original
				if err := input.Close(); err != nil {
					t.Errorf("close test stdin: %v", err)
				}
			})
			started := time.Now()
			code, written := stderrDuringRun(t, func() int {
				return Run(store, "-", nil, 0, -1, false, "", false, "", "")
			})
			if code != tc.want {
				t.Errorf("Run returned %d, want %d: %s", code, tc.want, written)
			}
			if strings.Contains(written, invalidSecret) {
				t.Errorf("startup diagnostic exposed a hub secret: %s", written)
			}
			if tc.want == 0 {
				if strings.Contains(written, "startup failed") {
					t.Errorf("accepted hub settings reported startup failure: %s", written)
				}
				return
			}
			for _, want := range []string{
				`hub server "invalid-hub"`,
				"secret too short: minimum 32 characters, got 31",
				`stage="plugin hub config"`,
				"code=" + diagnostic.CodeConfigHubInvalid,
				"delete plugin hub",
			} {
				if !strings.Contains(written, want) {
					t.Errorf("startup refusal omitted %q: %s", want, written)
				}
			}
			entries, err := slogutil.GlobalLogRing().Snapshot(0, "", "hub")
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range entries {
				if entry.Timestamp.Before(started) {
					continue
				}
				if entry.Level == "error" {
					if entry.Message == "plugin hub configuration is invalid" {
						return
					}
				}
			}
			t.Error("the hub log ring has no startup failure from this Run")
		})
	}
}
