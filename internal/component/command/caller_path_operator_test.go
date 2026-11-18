// Design: docs/architecture/api/commands.md -- actionable caller-path refusals.
package command

import (
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/opererr"
)

// TestCallerPathRefusalCarriesACompleteFix requires the shared path boundary
// to retain its local recovery command as a typed refusal, without echoing
// private command content. Readable local and admitted remote paths still work.
func TestCallerPathRefusalCarriesACompleteFix(t *testing.T) {
	insideDir := t.TempDir()
	inside := filepath.Join(insideDir, "router.conf")
	if err := os.WriteFile(inside, []byte("system {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	withCallerPathDirs(t, func() []string { return []string{insideDir} })
	outside := filepath.Join(t.TempDir(), "source with spaces.conf")
	private := base64.StdEncoding.EncodeToString([]byte("private caller edit"))
	for _, test := range []struct {
		name   string
		path   string
		verb   string
		tokens []string
	}{
		{name: "stdin", path: "-", verb: "show config validation", tokens: []string{"-"}},
		{name: "outside file", path: outside, verb: "show config source", tokens: []string{"file", outside}},
		{name: "private edit", path: outside, verb: "show config set", tokens: []string{"file", outside, "content", private}},
	} {
		t.Run(test.name, func(t *testing.T) {
			selected, err := CallerPath(RouteRemote, test.verb, test.tokens, test.path)
			if err == nil {
				t.Fatalf("remote path was admitted as %q", selected)
			}
			if selected != "" {
				t.Fatalf("refused path retained an admitted value: %q", selected)
			}
			refusal, typed := errors.AsType[*opererr.Error](err)
			if !typed {
				t.Fatalf("path refusal has no operator error: %v", err)
			}
			if refusal.Problem() == "" {
				t.Fatalf("path refusal lacks its problem: %v", err)
			}
			if refusal.Why() == "" {
				t.Fatalf("path refusal lacks its explanation: %v", err)
			}
			if !strings.HasPrefix(refusal.Fix(), "ze "+test.verb+" ") {
				t.Fatalf("fix does not name the caller's local command: %v", err)
			}
			if !strings.HasSuffix(err.Error(), "\nfix: "+refusal.Fix()) {
				t.Fatalf("the recovery command is not the final line: %v", err)
			}
			if strings.Contains(err.Error(), private) {
				t.Fatalf("private editing content was copied into the refusal: %v", err)
			}
		})
	}
	for _, route := range []Route{RouteLocal, RouteRemote} {
		selected, err := CallerPath(route, "show config source", []string{inside}, inside)
		if err != nil {
			t.Fatalf("readable path was refused on route %d: %v", route, err)
		}
		if route == RouteRemote {
			resolved, err := filepath.EvalSymlinks(inside)
			if err != nil {
				t.Fatal(err)
			}
			if selected != resolved {
				t.Fatalf("remote path = %q, want %q", selected, resolved)
			}
		} else if selected != inside {
			t.Fatalf("local path = %q, want %q", selected, inside)
		}
	}
}
