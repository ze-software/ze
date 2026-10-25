package dockerhost

import (
	"errors"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func envHas(environ []string, want string) bool { return slices.Contains(environ, want) }

// homeMe answers a fixed home directory, as os.UserHomeDir would.
func homeMe() (string, error) { return "/home/me", nil }

// VALIDATES: AC-8 (a): on macOS with no DOCKER_HOST, the `ze` profile's socket is
// named, and Colima's `default` socket or /var/run/docker.sock never is, even
// when both answer.
// METHOD: a scripted filesystem on which every socket exists, and one on which
// only the `ze` socket exists.
// PREVENTS: the labs running on Colima's Ubuntu `default` VM or on Docker
// Desktop, which the owner's standing decisions rule out.
func TestEnvironmentSelectsTheZeProfileSocket(t *testing.T) {
	for name, socket := range map[string]func(string) bool{
		"every socket answers": func(string) bool { return true },
		"only ze answers":      func(path string) bool { return path == "/home/me/.colima/ze/docker.sock" },
	} {
		got, err := Environment("darwin", homeMe, []string{"PATH=/bin"}, socket)
		if err != nil {
			t.Fatalf("%s: refused: %v", name, err)
		}
		if !envHas(got, "DOCKER_HOST=unix:///home/me/.colima/ze/docker.sock") {
			t.Fatalf("%s: env=%q, want the ze profile's socket", name, got)
		}
		for _, entry := range got {
			if strings.Contains(entry, "default") || strings.Contains(entry, "/var/run/docker.sock") {
				t.Errorf("%s: another daemon was named: %q", name, entry)
			}
		}
	}
}

// VALIDATES: AC-8 (b): on macOS with no DOCKER_HOST and no `ze` socket, the
// selection refuses, naming the missing socket, that the labs run on the `ze`
// profile, and `./le setup docker host start` on a line of its own, while the
// `default` socket and /var/run/docker.sock are present.
// METHOD: a fake home under t.TempDir() through the real IsSocket, holding a
// listening `default` socket and no `ze` directory.
// PREVENTS: a silent fallback to `default`, and a refusal with no fix.
func TestEnvironmentRefusesWithoutTheZeSocket(t *testing.T) {
	home := t.TempDir()
	defaultDir := filepath.Join(home, ".colima", "default")
	if err := os.MkdirAll(defaultDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	// A unix socket path is bounded near 104 bytes on macOS, which a test's
	// temporary directory can exceed; listen from inside the directory.
	t.Chdir(defaultDir)
	listener, err := (&net.ListenConfig{}).Listen(t.Context(), "unix", "docker.sock")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { listener.Close() }) //nolint:errcheck // test cleanup of a listener nobody dialed
	if !IsSocket(filepath.Join(defaultDir, "docker.sock")) {
		t.Fatal("the fake default socket is not a socket")
	}

	got, err := Environment("darwin", func() (string, error) { return home, nil }, []string{"PATH=/bin"}, IsSocket)
	if err == nil {
		t.Fatalf("selected %q, want a refusal", got)
	}
	message := err.Error()
	for _, want := range []string{
		filepath.Join(home, ".colima", "ze", "docker.sock"),
		"On macOS the labs run on the `ze` profile",
		"\n  ./le setup docker host start",
	} {
		if !strings.Contains(message, want) {
			t.Errorf("refusal lacks %q:\n%s", want, message)
		}
	}
	if strings.Contains(message, "unix://") {
		t.Errorf("the refusal named an endpoint to use:\n%s", message)
	}
}

// VALIDATES: an explicit DOCKER_HOST is kept on macOS whether or not the `ze`
// socket exists, and nothing changes on Linux.
// PREVENTS: overwriting the operator's chosen daemon (the QEMU docker-lab's
// forwarded socket is one); the Docker kernel check judges that daemon.
func TestEnvironmentKeepsAnExplicitDockerHost(t *testing.T) {
	original := []string{"DOCKER_HOST=tcp://docker", "PATH=/bin"}
	for _, socket := range []func(string) bool{func(string) bool { return true }, func(string) bool { return false }} {
		changed, err := Environment("darwin", homeMe, original, socket)
		if err != nil {
			t.Fatalf("refused an explicit DOCKER_HOST: %v", err)
		}
		if !reflect.DeepEqual(changed, original) {
			t.Fatalf("existing changed=%q", changed)
		}
	}
	changed, err := Environment("linux", homeMe, []string{"PATH=/bin"}, func(string) bool { return false })
	if err != nil {
		t.Fatalf("linux refused: %v", err)
	}
	if len(changed) != 1 {
		t.Fatalf("linux changed=%q", changed)
	}
}

// VALIDATES: an empty DOCKER_HOST is treated as unset, so the socket is selected.
// PREVENTS: `DOCKER_HOST=` in the environment silently disabling the selection.
func TestEnvironmentEmptyValueIsUnset(t *testing.T) {
	socket := func(path string) bool { return path == "/home/me/.colima/ze/docker.sock" }
	got, err := Environment("darwin", homeMe, []string{"DOCKER_HOST=", "PATH=/bin"}, socket)
	if err != nil {
		t.Fatalf("refused: %v", err)
	}
	if !envHas(got, "DOCKER_HOST=unix:///home/me/.colima/ze/docker.sock") {
		t.Fatalf("env=%q", got)
	}
}

// VALIDATES: a duplicated DOCKER_HOST is read at its last occurrence, which is
// the one a child process resolves.
// PREVENTS: an environment ending in an empty DOCKER_HOST= keeping the selection
// off, so the child inherits a name that resolves to nothing.
func TestEnvironmentReadsTheLastDockerHost(t *testing.T) {
	socket := func(path string) bool { return path == "/home/me/.colima/ze/docker.sock" }
	got, err := Environment("darwin", homeMe, []string{"DOCKER_HOST=tcp://stale", "DOCKER_HOST=", "PATH=/bin"}, socket)
	if err != nil {
		t.Fatalf("refused: %v", err)
	}
	if !envHas(got, "DOCKER_HOST=unix:///home/me/.colima/ze/docker.sock") {
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

// VALIDATES: an unknown home directory on macOS is a refusal that says so, and
// Linux never asks for one.
// PREVENTS: a relative `.colima/ze/docker.sock` named from an empty home, and a
// Linux run stopped by a home lookup it does not need.
func TestEnvironmentRefusesAnUnknownHome(t *testing.T) {
	unknown := func() (string, error) { return "", errors.New("$HOME is not defined") }
	if _, err := Environment("darwin", unknown, []string{"PATH=/bin"}, func(string) bool { return true }); err == nil || !strings.Contains(err.Error(), "home directory is unknown") {
		t.Errorf("darwin with no home: %v, want the home refusal", err)
	}
	if _, err := Environment("linux", unknown, []string{"PATH=/bin"}, func(string) bool { return true }); err != nil {
		t.Errorf("linux asked for a home: %v", err)
	}
}
