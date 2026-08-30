package dockerhost

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"
)

func envHas(environ []string, want string) bool { return slices.Contains(environ, want) }

// VALIDATES: Darwin Colima selection changes only DOCKER_HOST data and never invokes a shell.
// PREVENTS: Linux /usr/local assumptions or an existing Docker context being overwritten.
func TestEnvironmentSelection(t *testing.T) {
	socket := func(path string) bool { return path == "/home/me/.colima/default/docker.sock" }
	got := Environment("darwin", "/home/me", []string{"PATH=/bin"}, socket)
	if !envHas(got, "DOCKER_HOST=unix:///home/me/.colima/default/docker.sock") {
		t.Fatalf("env=%q", got)
	}
	original := []string{"DOCKER_HOST=tcp://docker", "PATH=/bin"}
	if changed := Environment("darwin", "/home/me", original, socket); !reflect.DeepEqual(changed, original) {
		t.Fatalf("existing changed=%q", changed)
	}
	if changed := Environment("linux", "/home/me", []string{"PATH=/bin"}, socket); len(changed) != 1 {
		t.Fatalf("linux changed=%q", changed)
	}
	absent := func(string) bool { return false }
	if changed := Environment("darwin", "/home/me", []string{"PATH=/bin"}, absent); len(changed) != 1 {
		t.Fatalf("absent socket changed=%q", changed)
	}
}

// VALIDATES: a client that can already reach its default endpoint is left alone.
// PREVENTS: DOCKER_HOST, which outranks a Docker context, moving an operator's
// working daemon onto a Colima socket that merely exists beside it.
func TestEnvironmentDefersToAReachableDefaultSocket(t *testing.T) {
	both := func(string) bool { return true }
	got := Environment("darwin", "/home/me", []string{"PATH=/bin"}, both)
	if len(got) != 1 {
		t.Fatalf("env=%q, want no selection while /var/run/docker.sock answers", got)
	}
}

// VALIDATES: an empty DOCKER_HOST is treated as unset, so the socket is selected.
// PREVENTS: `DOCKER_HOST=` in the environment silently disabling the selection.
func TestEnvironmentEmptyValueIsUnset(t *testing.T) {
	socket := func(path string) bool { return path == "/home/me/.colima/default/docker.sock" }
	got := Environment("darwin", "/home/me", []string{"DOCKER_HOST=", "PATH=/bin"}, socket)
	if !envHas(got, "DOCKER_HOST=unix:///home/me/.colima/default/docker.sock") {
		t.Fatalf("env=%q", got)
	}
}

// VALIDATES: a duplicated DOCKER_HOST is read at its last occurrence, which is
// the one a child process resolves.
// PREVENTS: an environment ending in an empty DOCKER_HOST= keeping the selection
// off, so the child inherits a name that resolves to nothing.
func TestEnvironmentReadsTheLastDockerHost(t *testing.T) {
	socket := func(path string) bool { return path == "/home/me/.colima/default/docker.sock" }
	got := Environment("darwin", "/home/me", []string{"DOCKER_HOST=tcp://stale", "DOCKER_HOST=", "PATH=/bin"}, socket)
	if !envHas(got, "DOCKER_HOST=unix:///home/me/.colima/default/docker.sock") {
		t.Fatalf("env=%q, want the socket over an environment whose last value is empty", got)
	}
	for _, entry := range got {
		if entry == "DOCKER_HOST=tcp://stale" {
			t.Errorf("a shadowed value survived: %q", got)
		}
	}
}

// VALIDATES: IsSocket accepts a socket and refuses a regular file or a missing path.
// PREVENTS: a leftover regular file at the socket path being named as the daemon.
func TestIsSocket(t *testing.T) {
	dir := t.TempDir()
	regular := filepath.Join(dir, "regular")
	if err := os.WriteFile(regular, []byte("x"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if IsSocket(regular) {
		t.Error("a regular file was named as a socket")
	}
	if IsSocket(filepath.Join(dir, "absent")) {
		t.Error("a missing path was named as a socket")
	}
}
