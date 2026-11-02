// Design: docs/architecture/api/commands.md — path arguments and stdin on the daemon route

package command

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withCallerPathDirs registers source for one test and restores the previous
// registration when the test ends. Only this package's tests swap it.
func withCallerPathDirs(t *testing.T, source func() []string) {
	t.Helper()
	callerPathDirs.mu.Lock()
	previous := callerPathDirs.source
	callerPathDirs.source = source
	callerPathDirs.mu.Unlock()
	t.Cleanup(func() {
		callerPathDirs.mu.Lock()
		callerPathDirs.source = previous
		callerPathDirs.mu.Unlock()
	})
}

// TestLocalCommandPathConfined proves the one rule every daemon-served
// path-taking command reads a file through.
//
// Goal: on the daemon route a caller-named file is read only inside the
// registered configuration directories, whatever spelling reaches outside
// them, and `-` is refused by name with the local command to run; on the
// operator's own route every path and `-` pass unchanged.
// Method: a configuration directory holding a file, a symbolic link out and a
// sibling directory outside it; each case goes through CallerPath on the
// remote, the unmarked and the local route.
func TestLocalCommandPathConfined(t *testing.T) {
	root := t.TempDir()
	configDir := filepath.Join(root, "ze")
	outsideDir := filepath.Join(root, "elsewhere")
	for _, dir := range []string{configDir, outsideDir} {
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	inside := filepath.Join(configDir, "router.conf")
	outside := filepath.Join(outsideDir, "secret.conf")
	for _, file := range []string{inside, outside} {
		if err := os.WriteFile(file, []byte("system {}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	linkOut := filepath.Join(configDir, "link.conf")
	if err := os.Symlink(outside, linkOut); err != nil {
		t.Fatal(err)
	}
	withCallerPathDirs(t, func() []string { return []string{configDir} })

	wantInside, err := filepath.EvalSymlinks(inside)
	if err != nil {
		t.Fatal(err)
	}

	refused := []struct {
		name string
		path string
		want string
	}{
		{"system file", "/etc/passwd", "the daemon reads only a file inside the configuration directories"},
		{"dot-dot escape", filepath.Join(configDir, "..", "elsewhere", "secret.conf"), "the daemon reads only a file inside"},
		{"symlink pointing out", linkOut, "the daemon reads only a file inside"},
		{"missing file inside", filepath.Join(configDir, "absent.conf"), "the daemon reads only a file inside"},
		{"directory itself", configDir, "the daemon reads only a file inside"},
		{"stdin", "-", "reads standard input, which reaches only the ze process the operator runs"},
	}
	for _, route := range []Route{RouteRemote, RouteUnspecified} {
		for _, tc := range refused {
			t.Run(tc.name, func(t *testing.T) {
				tokens := []string{tc.path}
				got, err := CallerPath(route, "show config validation", tokens, tc.path)
				if err == nil {
					t.Fatalf("route %d: %q answered %q, want a refusal", route, tc.path, got)
				}
				message := err.Error()
				if !strings.Contains(message, tc.want) {
					t.Errorf("route %d: refusal %q does not say %q", route, message, tc.want)
				}
				local := "run 'ze show config validation " + tc.path + "' on the host that holds the file"
				if !strings.Contains(message, local) {
					t.Errorf("route %d: refusal %q does not name the local command %q", route, message, local)
				}
			})
		}

		got, err := CallerPath(route, "show config validation", []string{inside}, inside)
		if err != nil {
			t.Fatalf("route %d: a file inside the configuration directory was refused: %v", route, err)
		}
		if got != wantInside {
			t.Errorf("route %d: inside file answered %q, want %q", route, got, wantInside)
		}
	}

	for _, path := range []string{"/etc/passwd", "-", linkOut} {
		got, err := CallerPath(RouteLocal, "show config validation", []string{path}, path)
		if err != nil {
			t.Fatalf("local route refused %q: %v", path, err)
		}
		if got != path {
			t.Errorf("local route answered %q for %q, want it unchanged", got, path)
		}
	}
}

// TestCallerPathRootConfinesNothing proves a registered directory that is the
// filesystem root is not a confinement: a daemon started from `/` has it as
// its working directory, and admitting it would admit every file.
func TestCallerPathRootConfinesNothing(t *testing.T) {
	withCallerPathDirs(t, func() []string { return []string{"/"} })
	if got, err := CallerPath(RouteRemote, "show config dump", []string{"/etc/passwd"}, "/etc/passwd"); err == nil {
		t.Fatalf("/ as a configuration directory admitted /etc/passwd as %q", got)
	}
}

// TestCallerPathNoRegistrationRefuses proves the miss is a refusal: with no
// registered directory the daemon route reads no file at all.
func TestCallerPathNoRegistrationRefuses(t *testing.T) {
	file := filepath.Join(t.TempDir(), "x.conf")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	withCallerPathDirs(t, nil)
	_, err := CallerPath(RouteRemote, "show config dump", []string{file}, file)
	if err == nil {
		t.Fatal("no registration admitted a file")
	}
	if !strings.Contains(err.Error(), "none of which exists on this host") {
		t.Errorf("refusal %q does not say no directory exists", err)
	}
}
