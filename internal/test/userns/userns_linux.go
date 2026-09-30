// Design: docs/architecture/testing/qemu-integration.md -- Build Tags, a namespace of the test's own
//
// Package userns runs a Go test in a user and network namespace of its own, with
// no privilege. A test that binds a fixed port (a protocol's well-known port, for
// example) collides on the host with a running daemon and with a parallel run of
// the same test. Inside a namespace of its own the port is always free, and
// nothing the test sends leaves the namespace.

//go:build linux

package userns

import (
	"errors"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"syscall"
	"testing"

	"github.com/vishvananda/netlink"
)

// childEnv marks the re-executed test binary. Its presence is the only thing that
// tells the child from the parent.
const childEnv = "ZE_TEST_USERNS_CHILD"

// Enter runs the calling test again in a new user and network namespace and
// returns true only in that child, where the test body runs with loopback up.
// The parent returns false once the child has finished: the test has then failed
// when the child failed, and skipped when the kernel refused the namespace or the
// child skipped. The caller returns at once on false.
//
// Enter MUST be the first statement of a top-level test: the child is selected by
// the test's name, and a subtest name would select its parent as well.
// Not safe for concurrent use with t.Parallel, because the child re-runs the one
// named test in a fresh process.
func Enter(t *testing.T) bool {
	t.Helper()
	if os.Getenv(childEnv) == "1" {
		loopbackUp(t)
		return true
	}
	args := []string{"-test.run", "^" + regexp.QuoteMeta(t.Name()) + "$", "-test.v", "-test.count=1"}
	// A coverage run hands the binary a counter directory, and the parent's profile
	// is built from every counter file in it. Passing it on lets the code the child
	// runs appear in that profile, which is what `./le rfc discriminate-record`
	// reads.
	for _, arg := range os.Args[1:] {
		if strings.HasPrefix(arg, "-test.gocoverdir=") {
			args = append(args, arg)
		}
	}
	cmd := exec.CommandContext(t.Context(), os.Args[0], args...) //nolint:gosec // The test binary re-executes itself with its own flags.
	cmd.Env = append(os.Environ(), childEnv+"=1")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags:  syscall.CLONE_NEWUSER | syscall.CLONE_NEWNET,
		UidMappings: []syscall.SysProcIDMap{{ContainerID: 0, HostID: os.Getuid(), Size: 1}},
		GidMappings: []syscall.SysProcIDMap{{ContainerID: 0, HostID: os.Getgid(), Size: 1}},
	}
	out, err := cmd.CombinedOutput()
	var exitErr *exec.ExitError
	if err != nil && !errors.As(err, &exitErr) {
		t.Skipf("the kernel refused a user and network namespace for the test: %v", err)
	}
	if err != nil {
		t.Fatalf("test in its own namespace failed: %v\n%s", err, out)
	}
	if strings.Contains(string(out), "--- SKIP") {
		t.Skipf("test skipped in its own namespace:\n%s", out)
	}
	t.Logf("namespace output:\n%s", out)
	return false
}

// loopbackUp brings up lo, which a new network namespace creates down. Up, it
// answers 127.0.0.0/8 and ::1.
func loopbackUp(t *testing.T) {
	t.Helper()
	lo, err := netlink.LinkByName("lo")
	if err != nil {
		t.Fatalf("lo lookup: %v", err)
	}
	if err := netlink.LinkSetUp(lo); err != nil {
		t.Fatalf("lo up: %v", err)
	}
}
