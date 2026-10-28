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
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
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
// when the child failed or ran no test of that name, and skipped when the kernel
// refused the namespace or the child skipped the test itself. The child receives
// every flag the parent was given except those childFlagsWithheld names, so a
// test's own flags reach the body. The caller returns at once on false.
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
	out, err := childCommand(t, t.Name()).CombinedOutput()
	var exitErr *exec.ExitError
	if err != nil && !errors.As(err, &exitErr) {
		t.Skip(refusal(err))
	}
	if err != nil {
		t.Fatalf("test in its own namespace failed: %v\n%s", err, out)
	}
	switch childResult(t.Name(), out) {
	case childPassed:
		t.Logf("namespace output:\n%s", out)
	case childSkipped:
		t.Skipf("test skipped in its own namespace:\n%s", out)
	case childFailed:
		t.Fatalf("test in its own namespace reported a failure with exit status 0:\n%s", out)
	case childRanNothing:
		// A child that selects no test exits 0, so without this guard a name the
		// child does not match passes the parent on nothing.
		t.Fatalf("test in its own namespace never ran: the child output holds no "+
			"`--- PASS: %s` or `--- SKIP: %s` line, so -test.run selected nothing:\n%s",
			t.Name(), t.Name(), out)
	case childResultUnspecified:
		panic("BUG: childResult answered no result")
	}
	return false
}

// childCommand answers the command that re-executes the test binary in a new
// user, network and mount namespace, selecting the one top-level test called
// name. Run, an error that is not an *exec.ExitError means the child never
// started, which on Linux is the kernel refusing the namespaces.
func childCommand(t *testing.T, name string) *exec.Cmd {
	t.Helper()
	hostDevice, err := netnsDevice("/")
	if err != nil {
		t.Fatalf("userns: read the host's /run/netns before entering a namespace: %v", err)
	}
	cmd := exec.CommandContext(t.Context(), os.Args[0], childArgs(flag.CommandLine, name)...) //nolint:gosec // The test binary re-executes itself with its own flags.
	cmd.Env = append(os.Environ(),
		childEnv+"=1",
		hostNetnsDeviceEnv+"="+string(strconv.AppendUint(nil, hostDevice, 10)))
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags:  syscall.CLONE_NEWUSER | syscall.CLONE_NEWNET | syscall.CLONE_NEWNS,
		UidMappings: []syscall.SysProcIDMap{{ContainerID: 0, HostID: os.Getuid(), Size: 1}},
		GidMappings: []syscall.SysProcIDMap{{ContainerID: 0, HostID: os.Getgid(), Size: 1}},
	}
	return cmd
}

// childFlagsWithheld names the flags the parent was given that the child does
// not receive, each with the reason. Every other flag the parent was given
// reaches the child with its value: a test's own flag (`-flowexport-ze-path`,
// `-rsvp-producer-daemon`), and testing's own flags such as `-test.gocoverdir`,
// which lets the code the child runs appear in a coverage run's profile, the one
// `./le rfc record` reads.
var childFlagsWithheld = map[string]string{
	"test.run":          "Enter selects the one test itself",
	"test.v":            "Enter sets plain -test.v, whose result lines childResult reads; test2json framing would hide them",
	"test.count":        "Enter runs the test once; the parent already repeats it once per count",
	"test.cpu":          "the parent already repeats the test once per value, and the child's result line would carry a -N suffix",
	"test.bench":        "the child would run benchmarks as well as the test",
	"test.fuzz":         "the child would fuzz rather than run the test",
	"test.fuzzworker":   "a fuzzing worker is a process the fuzz engine starts, never a test run",
	"test.list":         "the child would list names and run nothing",
	"test.testlogfile":  "the parent writes it on exit, and go test's result cache reads it",
	"test.coverprofile": "the parent writes it on exit from the counter directory the child shares",
	"test.cpuprofile":   parentWritesOnExit,
	"test.memprofile":   parentWritesOnExit,
	"test.blockprofile": parentWritesOnExit,
	"test.mutexprofile": parentWritesOnExit,
	"test.trace":        parentWritesOnExit,
}

// parentWritesOnExit is the reason a profile or trace file is withheld: the
// child writing the same path would race with the parent and be overwritten.
const parentWritesOnExit = "the parent writes it on exit"

// childArgs answers the child's command line: the one top-level test called
// name, verbose so childResult can read its result line, once, followed by every
// flag set on fs (the flags the parent was given) that childFlagsWithheld does
// not hold. flag.Visit walks only flags that were set, in lexical order.
func childArgs(fs *flag.FlagSet, name string) []string {
	args := []string{"-test.run", "^" + regexp.QuoteMeta(name) + "$", "-test.v", "-test.count=1"}
	fs.Visit(func(f *flag.Flag) {
		if _, withheld := childFlagsWithheld[f.Name]; withheld {
			return
		}
		args = append(args, "-"+f.Name+"="+f.Value.String())
	})
	return args
}

// childOutcome is what the child's output says happened to the selected test.
type childOutcome uint8

const (
	childResultUnspecified childOutcome = iota
	// childPassed: the output holds `--- PASS: <name> (`.
	childPassed
	// childSkipped: the output holds `--- SKIP: <name> (`.
	childSkipped
	// childFailed: the output holds `--- FAIL: <name> (`.
	childFailed
	// childRanNothing: no result line names the test, so the child selected
	// nothing (a name mismatch) or never reached it.
	childRanNothing
)

// childResult reads the top-level result line `-test.v` writes for the test
// called name. Only a line at column 0 counts: a subtest's result line is
// indented, so a skipped subtest neither skips nor passes the test it belongs
// to, and a test whose name merely starts with name is a different test.
func childResult(name string, out []byte) childOutcome {
	line := regexp.MustCompile(`(?m)^--- (PASS|SKIP|FAIL): ` + regexp.QuoteMeta(name) + ` \(`)
	match := line.FindSubmatch(out)
	if match == nil {
		return childRanNothing
	}
	switch string(match[1]) {
	case "PASS":
		return childPassed
	case "SKIP":
		return childSkipped
	case "FAIL":
		return childFailed
	}
	panic("BUG: the result-line pattern admits only PASS, SKIP and FAIL")
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
