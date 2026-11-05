//go:build integration

// Design: docs/architecture/testing/qemu-integration.md -- own-system guest commands.
// Related: dockerlab.go -- the registered command branch this proof executes.

package testqemu

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/job"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// TestDockerLabCommandRunsInZeSystem reaches the registered action, boots the
// supplied kernel on Ze's image, and has its unprivileged guest user read and
// write this checkout's private scratch. The command checks the real PID 1 and
// root filesystem before publishing its result, rather than accepting a host
// command or a kernel-only Alpine boot as guest execution. No prerequisite skip
// can satisfy this proof. The parent MUST stage the canonical native runtime
// kernel at tmp/kernel/build/vmlinuz before selecting this test.
func TestDockerLabCommandRunsInZeSystem(t *testing.T) {
	if os.Getuid() == 0 {
		t.Fatal("this proof must run as the ordinary checkout owner, never host root")
	}
	root, err := lepath.Root()
	if err != nil {
		t.Fatal(err)
	}
	paths, err := lepath.ResolveSession(root, true)
	if err != nil {
		t.Fatal(err)
	}
	scratch := filepath.Join(root, paths.Scratch)
	directory, err := os.MkdirTemp(scratch, "own-system-command-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(directory); err != nil {
			t.Error(err)
		}
	})
	input := filepath.Join(directory, "input with spaces")
	output := filepath.Join(directory, "output with spaces")
	const payload = "the mounted main checkout supplied these bytes\n"
	if err := os.WriteFile(input, []byte(payload), 0o600); err != nil {
		t.Fatal(err)
	}
	version, err := os.ReadFile(filepath.Join(root, "internal", "appliance", "kernel.version"))
	if err != nil {
		t.Fatal(err)
	}
	kernel := filepath.Join(root, "tmp", "kernel", "build", "vmlinuz")
	command := strings.Join([]string{
		"set -eu",
		"test \"$(id -u)\" = " + shellQuote(strconv.Itoa(os.Getuid())),
		"test \"$(sudo -n readlink /proc/1/exe)\" = /gokrazy/init",
		"test \"$(stat -f -c %T /)\" = squashfs",
		"test ! -S /var/run/docker.sock",
		"test -r \"$" + job.ParentVariable + "\"",
		"cd " + shellQuote(root),
		"test -r go.mod",
		"actual=$(uname -r)",
		"case \"$actual\" in " + strings.TrimSpace(string(version)) + "|" + strings.TrimSpace(string(version)) + ".*|" + strings.TrimSpace(string(version)) + "-*) ;; *) exit 91 ;; esac",
		"cat " + shellQuote(input) + " > " + shellQuote(output),
	}, " && ")
	answer, code := Answer([]string{dockerLabVerb, keywordKernel, kernel, keywordCommand, command, keywordTimeout, "120s"})
	if code != 0 {
		t.Fatalf("own-system guest command exited %d: %+v", code, answer)
	}
	observed, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("guest never published its checked result through the main mount: %v", err)
	}
	if string(observed) != payload {
		t.Fatalf("guest read or published different main-checkout bytes: %q", observed)
	}
}
