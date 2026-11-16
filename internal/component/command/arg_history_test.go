// Design: docs/architecture/api/commands.md -- validate masked recall before a handler runs.
package command

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command/registry"
	"github.com/ze-software/ze/internal/core/redact"
)

// TestMaskedHistoryArgumentNeverReachesHandler proves shared parsed argument
// validation refuses credential/content placeholders, with usable real values
// and an ordinary field whose literal text happens to be the marker.
func TestMaskedHistoryArgumentNeverReachesHandler(t *testing.T) {
	for _, name := range []string{"password", "plaintext-password", "content", "description"} {
		t.Run(name, func(t *testing.T) {
			withLocalArgDefs(t)
			withArgDefSource(t, func(path string) ([]ArgDef, error) {
				if path != localDataArgsPath {
					return nil, nil
				}
				return []ArgDef{mustArgDef(NewStringArg(name, nil, nil, ArgOptions{Mandatory: true}))}, nil
			})
			calls := 0
			if err := RegisterLocalData(localDataArgsPath, func(ValidatedArgs) LocalAnswer {
				calls++
				return LocalAnswer{Payload: map[string]any{"ok": true}}
			}, registry.Meta{}, func(string, any) int { return 0 }); err != nil {
				t.Fatal(err)
			}
			var answer string
			var code int
			var served bool
			stdout, stderr := captureStdio(t, func() {
				answer, code, served = ServeLocal(localDataArgsPath+" "+name+" "+redact.Placeholder, "")
			})
			if stdout != "" {
				t.Errorf("local route wrote stdout %q; want empty", stdout)
			}
			if !served {
				t.Fatal("registered command was not served")
			}
			if name == "description" {
				if code != 0 || calls != 1 {
					t.Fatalf("ordinary marker: code %d, calls %d; want 0, 1", code, calls)
				}
			} else {
				if code != 1 || calls != 0 {
					t.Fatalf("masked credential: code %d, calls %d; want 1, 0", code, calls)
				}
				if answer != "" {
					t.Errorf("refusal answer %q; want empty", answer)
				}
				if !strings.Contains(stderr, "masked") || !strings.Contains(stderr, "real value") {
					t.Errorf("refusal %q must explain replacement with the real value", stderr)
				}
			}
			before := calls
			_, code, served = ServeLocal(localDataArgsPath+" "+name+" actual-value", "")
			if !served || code != 0 || calls != before+1 {
				t.Errorf("real value: served %v, code %d, calls %d; want true, 0, %d", served, code, calls, before+1)
			}
		})
	}
}
