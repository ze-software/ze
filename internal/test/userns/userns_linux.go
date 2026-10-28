// Design: docs/architecture/testing/qemu-integration.md -- Build Tags, a namespace of the test's own
// Related: userns_other.go -- Enter off Linux, which runs the test in place
//
// Package userns runs a Go test in a user, network and mount namespace of its
// own, with no privilege. A test that binds a fixed port (a protocol's
// well-known port, for example) collides on the host with a running daemon and
// with a parallel run of the same test. Inside a namespace of its own the port
// is always free, and nothing the test sends leaves the namespace. The mount
// namespace carries a private /run/netns, so a named network namespace
// (`netns.NewNamed`, `ip netns add`) works there and never reaches the host.

//go:build linux

package userns

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/vishvananda/netlink"
)

// childEnv marks the re-executed test binary. Its presence is the only thing that
// tells the child from the parent.
const childEnv = "ZE_TEST_USERNS_CHILD"

// hostNetnsDeviceEnv carries to the child the device number the parent saw at
// the host's /run/netns (or at /run, where the host has no /run/netns). The
// child refuses to run the test while its own /run/netns still sits on that
// device, because a named namespace created there would then be the host's.
const hostNetnsDeviceEnv = "ZE_TEST_USERNS_HOST_NETNS_DEV"

// Enter runs the calling test again in a new user, network and mount namespace
// and returns true only in that child, where the test body runs as root over
// its own namespaces, with loopback up and a private tmpfs on /run/netns.
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
		childSetup(t)
		return true
	}
	hostDevice, err := netnsDevice("/")
	if err != nil {
		t.Fatalf("userns: read the host's /run/netns before entering a namespace: %v", err)
	}
	args := []string{"-test.run", "^" + regexp.QuoteMeta(t.Name()) + "$", "-test.v", "-test.count=1"}
	// A coverage run hands the binary a counter directory, and the parent's profile
	// is built from every counter file in it. Passing it on lets the code the child
	// runs appear in that profile, which is what `./le rfc record`
	// reads.
	for _, arg := range os.Args[1:] {
		if strings.HasPrefix(arg, "-test.gocoverdir=") {
			args = append(args, arg)
		}
	}
	cmd := exec.CommandContext(t.Context(), os.Args[0], args...) //nolint:gosec // The test binary re-executes itself with its own flags.
	cmd.Env = append(os.Environ(),
		childEnv+"=1",
		hostNetnsDeviceEnv+"="+string(strconv.AppendUint(nil, hostDevice, 10)))
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags:  syscall.CLONE_NEWUSER | syscall.CLONE_NEWNET | syscall.CLONE_NEWNS,
		UidMappings: []syscall.SysProcIDMap{{ContainerID: 0, HostID: os.Getuid(), Size: 1}},
		GidMappings: []syscall.SysProcIDMap{{ContainerID: 0, HostID: os.Getgid(), Size: 1}},
	}
	out, err := cmd.CombinedOutput()
	var exitErr *exec.ExitError
	if err != nil && !errors.As(err, &exitErr) {
		t.Skip(refusal(err))
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

// refusal is the skip text for a kernel that would not create the namespaces.
// It names the setup step that lifts the restriction, because the usual cause
// on a developer host is a sysctl or AppArmor policy that `./le setup install`
// changes.
func refusal(err error) string {
	return "the kernel refused a user, network and mount namespace for the test: " + err.Error() +
		"; run `./le setup install` and see its `userns-unrestricted` check"
}

// childSetup prepares the child before the test body runs: a mount tree whose
// changes propagate nowhere, a private /run/netns on a device the host does not
// hold, and lo up. Any step that fails fails the test, never skips it, because
// the namespace grants every privilege these steps need.
func childSetup(t *testing.T) {
	t.Helper()
	// MS_REC|MS_PRIVATE on / stops a mount made below from propagating to a peer
	// group the child's mount namespace inherited from the host.
	if err := syscall.Mount("", "/", "", syscall.MS_REC|syscall.MS_PRIVATE, ""); err != nil {
		t.Fatalf("userns: make the mount tree private: %v", err)
	}
	if err := mountNetnsDir("/"); err != nil {
		t.Fatalf("userns: %v", err)
	}
	hostDevice, err := strconv.ParseUint(os.Getenv(hostNetnsDeviceEnv), 10, 64)
	if err != nil {
		t.Fatalf("userns: the parent passed no host /run/netns device in %s: %v", hostNetnsDeviceEnv, err)
	}
	device, err := netnsDevice("/")
	if err != nil {
		t.Fatalf("userns: read the private /run/netns: %v", err)
	}
	if device == hostDevice {
		t.Fatalf("userns: /run/netns in the namespace is still the host's (device %d)", device)
	}
	loopbackUp(t)
}

// mountNetnsDir mounts a tmpfs on <root>/run/netns. Where that directory is
// absent, and the caller cannot create it on the host's /run, the tmpfs goes on
// <root>/run instead and the directory is created in it. The error names the
// step that failed and wraps the errno.
func mountNetnsDir(root string) error {
	run := filepath.Join(root, "run")
	netnsDir := filepath.Join(run, "netns")
	_, err := os.Stat(netnsDir)
	if err == nil {
		return mountTmpfs(netnsDir)
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := mountTmpfs(run); err != nil {
		return err
	}
	// 0750 as the host's own /run/netns: the child's root, which is the caller's
	// uid on the host, is the only user that needs it.
	return os.Mkdir(netnsDir, 0o750)
}

// mountTmpfs mounts a fresh tmpfs on target, nosuid and nodev like the host's /run.
func mountTmpfs(target string) error {
	if err := syscall.Mount("tmpfs", target, "tmpfs", syscall.MS_NOSUID|syscall.MS_NODEV, "mode=0755"); err != nil {
		return &os.PathError{Op: "mount tmpfs on", Path: target, Err: err}
	}
	return nil
}

// netnsDevice answers the device number that holds <root>/run/netns, or <root>/run
// where the host has no /run/netns: the filesystem a named namespace would be
// created on.
func netnsDevice(root string) (uint64, error) {
	var st syscall.Stat_t
	err := syscall.Stat(filepath.Join(root, "run", "netns"), &st)
	if errors.Is(err, syscall.ENOENT) {
		err = syscall.Stat(filepath.Join(root, "run"), &st)
	}
	if err != nil {
		return 0, err
	}
	return st.Dev, nil
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
