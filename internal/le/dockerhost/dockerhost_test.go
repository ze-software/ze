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

// VALIDATES: the endpoint a Docker client uses names a Colima VM exactly when
// it is a docker.sock beside a profile's colima.yaml, under any Colima home
// (~/.colima, the XDG colima directory, or a custom COLIMA_HOME), and the
// profile is that directory's name; the docker.sock Colima also forwards to
// the home's root names the default profile, whose Lima instance is
// _lima/colima; the ssh prefix runs a command in that profile's VM.
// METHOD: a table of endpoints over a scripted filesystem holding a default
// and a work profile under ~/.colima, a default under ~/.config/colima and
// one under /srv/colima-home, beside Docker Desktop's socket.
// PREVENTS: loading an AppArmor profile through `colima ssh` into a VM that is
// not the daemon the labs use, and refusing a Colima VM whose home was moved.
func TestColimaProfile(t *testing.T) {
	present := map[string]bool{
		"/Users/me/.colima/default/colima.yaml":        true,
		"/Users/me/.colima/work/colima.yaml":           true,
		"/Users/me/.colima/_lima/colima/lima.yaml":     true,
		"/Users/me/.config/colima/default/colima.yaml": true,
		"/srv/colima-home/default/colima.yaml":         true,
	}
	exists := func(path string) bool { return present[path] }
	for endpoint, want := range map[string]string{
		"unix:///Users/me/.colima/default/docker.sock":        "default",
		"unix:///Users/me/.colima/work/docker.sock":           "work",
		"unix:///Users/me/.config/colima/default/docker.sock": "default",
		"unix:///srv/colima-home/default/docker.sock":         "default",
		"unix:///Users/me/.colima/default/docker.sock\n":      "default",
		"unix:///Users/me/.colima/docker.sock":                "default",
		"unix:///Users/me/.colima/gone/docker.sock":           "",
		"unix:///var/run/docker.sock":                         "",
		"unix:///Users/me/.docker/run/docker.sock":            "",
		"unix:///Users/me/.colima/default/containerd.sock":    "",
		"tcp://127.0.0.1:2375":                                "",
		"/Users/me/.colima/default/docker.sock":               "",
		"":                                                    "",
	} {
		got, colima := ColimaProfile(endpoint, exists)
		if colima != (want != "") {
			t.Errorf("%q: Colima %v, want %v", endpoint, colima, want != "")
			continue
		}
		if got != want {
			t.Errorf("%q: profile %q, want %q", endpoint, got, want)
		}
	}
	if got, want := ColimaSSH("work"), []string{"colima", "ssh", "--profile", "work", "--"}; !slices.Equal(got, want) {
		t.Errorf("ColimaSSH = %q, want %q", got, want)
	}
}
