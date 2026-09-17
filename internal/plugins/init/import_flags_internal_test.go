// Design: docs/architecture/storage-backends.md -- ze init --from imports a blob as it is.

package init

import (
	"io"
	"os"
	"strings"
	"testing"
)

// VALIDATES: `ze init --from` refuses, by name, each flag that shapes a new
// store and that an import would otherwise silently ignore.
func TestInitFromRefusesNewStoreFlagsByName(t *testing.T) {
	for _, flag := range []string{"--managed", "--web-cert=0.0.0.0:8080", "--web-cert-name=router.example.com"} {
		t.Run(flag, func(t *testing.T) {
			name, _, _ := strings.Cut(flag, "=")
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			saved := os.Stderr
			os.Stderr = w
			code := Run([]string{"--from", "/nonexistent/database.zefs", flag})
			os.Stderr = saved
			w.Close() //nolint:errcheck // pipe writer
			out, _ := io.ReadAll(r)
			if code != 1 {
				t.Fatalf("Run(--from %s) = %d, want 1", flag, code)
			}
			if !strings.Contains(string(out), name+" shapes a new store and cannot be combined") {
				t.Fatalf("stderr did not refuse %s by name:\n%s", name, out)
			}
		})
	}
}
