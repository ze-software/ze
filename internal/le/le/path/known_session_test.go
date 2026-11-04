// Design: docs/contributing/running-commands.md -- refusal context lookup must not write.
package lepath

import (
	"encoding/base64"
	"maps"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/le/human/humantest"
)

// TestKnownSessionIdentityNeverWrites covers the resolver's precedence, absent
// and unusable state using isolated process/cache fixtures. The entire tree and
// an existing cache's timestamp must remain unchanged, unlike normal resolution.
func TestKnownSessionIdentityNeverWrites(t *testing.T) {
	for _, source := range []string{"environment", "process", "jwt", "cache", "absent", "unsafe-environment", "unreadable-cache"} {
		t.Run(source, func(t *testing.T) {
			root := t.TempDir()
			process := fixtureSessionProcess{pid: 30, parents: map[int]int{30: 20, 20: 1}}
			t.Setenv("CLAUDE_CODE_SESSION_ID", "")
			t.Setenv("CLAUDE_CODE_SESSION_ACCESS_TOKEN", "")
			want := ""
			wantError := false
			switch source {
			case "environment":
				t.Setenv("CLAUDE_CODE_SESSION_ID", "from-environment")
				process.argv = map[int][]string{20: {"/bin/claude", "--session-id", "from-process"}}
				want = "from-environment"
			case "process":
				process.argv = map[int][]string{20: {"/bin/claude", "--session-id", "from-process"}}
				payload := base64.RawURLEncoding.EncodeToString([]byte(`{"session_id":"from-jwt"}`))
				t.Setenv("CLAUDE_CODE_SESSION_ACCESS_TOKEN", "x."+payload+".x")
				want = "from-process"
			case "jwt":
				payload := base64.RawURLEncoding.EncodeToString([]byte(`{"session_id":"from-jwt"}`))
				t.Setenv("CLAUDE_CODE_SESSION_ACCESS_TOKEN", "x."+payload+".x")
				want = "from-jwt"
			case "cache":
				want = "from-cache"
			case "absent":
			case "unsafe-environment":
				t.Setenv("CLAUDE_CODE_SESSION_ID", "../escape")
				wantError = true
			case "unreadable-cache":
				if err := os.WriteFile(filepath.Join(root, "tmp"), []byte("not a directory"), 0o600); err != nil {
					t.Fatal(err)
				}
				wantError = true
			}
			cache := sessionIdentityCache(root, process)
			if source == "cache" {
				if err := os.MkdirAll(filepath.Dir(cache), 0o750); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(cache, []byte(want+"\n"), 0o600); err != nil {
					t.Fatal(err)
				}
				old := time.Unix(123456789, 0)
				if err := os.Chtimes(cache, old, old); err != nil {
					t.Fatal(err)
				}
			}
			before := humantest.Snapshot(t, root)
			id, err := knownSessionID(root, process)
			if (err != nil) != wantError || id != want {
				t.Errorf("known identity = %q, %v; want %q, error %t", id, err, want, wantError)
			}
			if !maps.Equal(before, humantest.Snapshot(t, root)) {
				t.Error("read-only identity lookup wrote files or directories")
			}
			if source == "cache" {
				info, err := os.Stat(cache)
				if err != nil {
					t.Fatal(err)
				}
				if !info.ModTime().Equal(time.Unix(123456789, 0)) {
					t.Error("read-only lookup refreshed the cache timestamp")
				}
			}
		})
	}
}
