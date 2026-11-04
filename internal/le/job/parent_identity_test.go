// Design: docs/contributing/running-commands.md -- inherited admission identity.
// Related: job.go -- admit must refuse an identity it cannot read.

package job

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
)

// TestUnreadableParentIdentityRefusesWork drives RunContext with a directory
// instead of a readable entry, and with a readable entry lacking its label.
// Neither failure may start work, acquire a slot, or alter the named resource.
func TestUnreadableParentIdentityRefusesWork(t *testing.T) {
	for _, malformed := range []bool{false, true} {
		t.Run(map[bool]string{false: "unreadable", true: "missing-label"}[malformed], func(t *testing.T) {
			detach(t)
			root := fixtureRepo(t)
			parent := filepath.Join(t.TempDir(), "parent")
			if malformed {
				if err := os.WriteFile(parent, []byte("PID=1\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			} else if err := os.Mkdir(parent, 0o700); err != nil {
				t.Fatal(err)
			}
			t.Setenv(ParentKey, parent)
			env.ResetCache()
			t.Cleanup(env.ResetCache)
			ran := filepath.Join(root, "ran")
			var notes bytes.Buffer
			adm := admission(t, root)
			adm.Err = &notes
			report, code := adm.RunContext(t.Context(), "probe", []string{"sh", "-c", "touch \"$1\"", "sh", ran}, root, nil)
			if code != 2 || report.Admission != KindUnspecified.String() {
				t.Errorf("unreadable identity admitted work: code=%d report=%+v", code, report)
			}
			if !strings.Contains(notes.String(), parent) {
				t.Errorf("identity refusal omitted the named entry: %s", notes.String())
			}
			if _, err := os.Stat(ran); !os.IsNotExist(err) {
				t.Errorf("work ran with an unreadable identity: %v", err)
			}
			if _, err := os.Stat(parent); err != nil {
				t.Errorf("identity resource was removed: %v", err)
			}
		})
	}
}
