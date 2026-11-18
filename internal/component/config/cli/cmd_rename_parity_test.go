// Design: docs/architecture/api/commands.md -- stored configuration publication.

package cli

import (
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/core/env"
)

// TestConfigRenameRootMovesPointerOnlyHistory proves the standalone adapter
// recognizes a configuration without a mirror and preserves its active bytes.
func TestConfigRenameRootMovesPointerOnlyHistory(t *testing.T) {
	store := newImportBlobStore(t)
	const oldName, newName = "router.conf", "renamed.conf"
	const content = "system { host renamed; }\n"
	if _, _, err := storage.EnsureActiveVersion(store, oldName, []byte(content), time.Now()); err != nil {
		t.Fatal(err)
	}
	if store.Exists(oldName) {
		t.Fatal("positive control needs a pointer without a mirror")
	}
	code, _, stderr := captureBlobStdio(t, func() int {
		return cmdRenameWithStorage(store, []string{oldName, newName})
	})
	if code != exitOK {
		t.Fatalf("pointer-only configuration refused: %d: %s", code, stderr)
	}
	active, err := storage.ReadActiveConfig(store, newName)
	if err != nil {
		t.Fatal(err)
	}
	if string(active) != content {
		t.Fatalf("renamed active bytes = %q, want %q", active, content)
	}
	if _, err := storage.ReadActiveConfig(store, oldName); err == nil {
		t.Fatal("the old name still resolves after rename")
	}
}

// TestConfigRenameRootRoutesBusyStoreAndChecksIdentity proves only a matching
// daemon acknowledgment can report success for a rename in an owned store.
func TestConfigRenameRootRoutesBusyStoreAndChecksIdentity(t *testing.T) {
	_, folder := busyStore(t)
	previous := env.Get("ze.config.dir")
	t.Cleanup(func() { _ = env.Set("ze.config.dir", previous) })
	if err := env.Set("ze.config.dir", folder); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name string
		body string
		code int
	}{
		{name: "matching", body: `{"old":"router.conf","new":"renamed.conf"}`, code: exitOK},
		{name: "wrong identity", body: `{"old":"router.conf","new":"different.conf"}`, code: exitError},
	} {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			ask := func(gotFolder, line string) ([]byte, error) {
				calls++
				if gotFolder != folder || line != "request data config rename old router.conf new renamed.conf" {
					t.Fatalf("wrong daemon target: %q: %s", gotFolder, line)
				}
				return []byte(test.body), nil
			}
			code, stdout, stderr := captureBlobStdio(t, func() int {
				return cmdRenameThrough(nil, []string{"router.conf", "renamed.conf"}, ask)
			})
			if calls != 1 || code != test.code {
				t.Fatalf("calls=%d code=%d, want one call and code=%d: %s", calls, code, test.code, stderr)
			}
			if test.code == exitOK {
				if !strings.Contains(stdout, "renamed router.conf -> renamed.conf") {
					t.Fatalf("matching answer lost success: %s", stdout)
				}
			} else if stdout != "" {
				t.Fatalf("mismatched answer reported success: %s", stdout)
			}
		})
	}
}
