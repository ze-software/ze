// Design: docs/architecture/testing/runner-architecture.md -- native resource admission.

package editorcase

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/test/runner"
)

// TestEditorSuiteNativeAdmission drives the registered suite with malformed and
// unavailable resources. A missing authored config is a tripwire for execution;
// no request.json may be allocated anywhere in the owned scratch root.
func TestEditorSuiteNativeAdmission(t *testing.T) {
	executable := admissionExecutable(t)
	var capability runner.Record
	if _, err := runner.ParseResourceOption(&capability, "needs-linux:caps=sys-time"); err != nil {
		t.Fatal(err)
	}
	if capability.SkipReason == "" {
		t.Skip("negative admission proof requires a host without sys-time")
	}
	for _, tc := range []struct {
		name, declaration, want string
		fail                    bool
	}{
		{"unavailable", "option=needs-linux:caps=sys-time", "sys-time", false},
		{"malformed", "option=needs-linux:caps=sys-time\noption=exclusive:wrong=a", "unknown", true},
		{"overflow", "option=needs-linux:caps=sys-time\noption=timeout:value=9223372036854775807ns", "duration limit", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module admission-proof\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			content := tc.declaration + "\noption=file:path=must-not-open.conf\n"
			if err := os.WriteFile(filepath.Join(root, "admission.et"), []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, executable, "test", "editor", "--dir", root, "--all")
			cmd.Dir = root
			cmd.Env = append(os.Environ(), "TMPDIR="+root, "ze_le_store_trim=off")
			plugin.KillGroupOnCancel(cmd)
			log, err := os.Create(filepath.Join(root, "native.log"))
			if err != nil {
				t.Fatal(err)
			}
			defer log.Close() //nolint:errcheck // Test reads the completed output below.
			cmd.Stdout, cmd.Stderr = log, log
			if err := plugin.StartChild(cmd); err != nil {
				t.Fatal(err)
			}
			waitErr := cmd.Wait()
			if ctx.Err() != nil {
				t.Fatal("native admission needed watchdog")
			}
			if (waitErr != nil) != tc.fail {
				t.Fatalf("wrong exit: %v", waitErr)
			}
			body, err := os.ReadFile(log.Name())
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(body), tc.want) {
				t.Fatalf("missing %q:\n%s", tc.want, body)
			}
			if strings.Contains(string(body), "must-not-open") {
				t.Fatalf("editor ran before admission:\n%s", body)
			}
			if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if entry.Name() == "request.json" {
					t.Errorf("child resources allocated before admission: %s", path)
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
		})
	}
}
