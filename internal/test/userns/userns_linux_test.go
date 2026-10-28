//go:build linux

package userns

import (
	"errors"
	"flag"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
	"golang.org/x/sys/unix"
)

// TestEnterRunsTheTestAsRootWithLoopbackUp pins the contract every existing
// caller relies on: the body runs as uid 0 over a network namespace of its own,
// with lo up.
func TestEnterRunsTheTestAsRootWithLoopbackUp(t *testing.T) {
	if !Enter(t) {
		return
	}
	if uid := os.Geteuid(); uid != 0 {
		t.Fatalf("euid in the namespace = %d, want 0", uid)
	}
	lo, err := netlink.LinkByName("lo")
	if err != nil {
		t.Fatalf("lo lookup: %v", err)
	}
	if lo.Attrs().Flags&syscall.IFF_UP == 0 {
		t.Fatalf("lo flags = %v, want up", lo.Attrs().Flags)
	}
}

// TestEnterGivesAPrivateNetnsDirectory proves a named network namespace works in
// the child and stays there: /run/netns is a tmpfs on a device other than the
// one the parent saw for the host's /run/netns, and netns.NewNamed,
// netns.GetFromName and netns.DeleteNamed all succeed in it.
func TestEnterGivesAPrivateNetnsDirectory(t *testing.T) {
	if !Enter(t) {
		return
	}
	// NewNamed moves the calling thread into the new namespace. The thread stays
	// locked, so the runtime ends it with this goroutine rather than reusing it.
	runtime.LockOSThread()

	var fs unix.Statfs_t
	if err := unix.Statfs("/run/netns", &fs); err != nil {
		t.Fatalf("statfs /run/netns: %v", err)
	}
	if fs.Type != unix.TMPFS_MAGIC {
		t.Fatalf("/run/netns filesystem magic = %#x, want tmpfs %#x", fs.Type, unix.TMPFS_MAGIC)
	}
	hostDevice, err := strconv.ParseUint(os.Getenv(hostNetnsDeviceEnv), 10, 64)
	if err != nil {
		t.Fatalf("host /run/netns device from the parent: %v", err)
	}
	device, err := netnsDevice("/")
	if err != nil {
		t.Fatalf("stat /run/netns: %v", err)
	}
	if device == hostDevice {
		t.Fatalf("/run/netns device %d is the host's, so a named namespace would reach the host", device)
	}

	origin, err := netns.Get()
	if err != nil {
		t.Fatalf("current namespace: %v", err)
	}
	defer origin.Close() //nolint:errcheck // A handle closed at test end; nothing reads the result.
	const name = "ze-userns-probe"
	created, err := netns.NewNamed(name)
	if err != nil {
		t.Fatalf("netns.NewNamed(%q): %v", name, err)
	}
	defer created.Close() //nolint:errcheck // A handle closed at test end; nothing reads the result.
	if err := netns.Set(origin); err != nil {
		t.Fatalf("return to the test's namespace: %v", err)
	}
	found, err := netns.GetFromName(name)
	if err != nil {
		t.Fatalf("netns.GetFromName(%q): %v", name, err)
	}
	defer found.Close() //nolint:errcheck // A handle closed at test end; nothing reads the result.
	if !found.Equal(created) {
		t.Fatalf("netns.GetFromName(%q) = %v, want the namespace NewNamed created (%v)", name, found, created)
	}
	if err := netns.DeleteNamed(name); err != nil {
		t.Fatalf("netns.DeleteNamed(%q): %v", name, err)
	}
	if _, err := os.Stat("/run/netns/" + name); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("/run/netns/%s after DeleteNamed: %v, want it gone", name, err)
	}
}

// TestEnterServesIPNetns proves the iproute2 route that rsvpte and other
// callers take works in the child: `ip netns add` and `ip netns exec` both
// exit 0, and the link created through exec lands in the named namespace.
func TestEnterServesIPNetns(t *testing.T) {
	if !Enter(t) {
		return
	}
	if _, err := exec.LookPath("ip"); err != nil {
		t.Skipf("iproute2 `ip` is not installed: %v", err)
	}
	const name = "ze-userns-ip"
	steps := [][]string{
		{"netns", "add", name},
		{"netns", "exec", name, "ip", "link", "add", "zeprobe0", "type", "dummy"},
		{"-n", name, "link", "show", "zeprobe0"},
		{"netns", "delete", name},
	}
	for _, step := range steps {
		out, err := exec.CommandContext(t.Context(), "ip", step...).CombinedOutput()
		if err != nil {
			t.Fatalf("ip %s: %v\n%s", strings.Join(step, " "), err, out)
		}
	}
}

// TestEnterCreatesAMissingNetnsDirectory proves the path for a host with no
// /run/netns: mountNetnsDir, pointed at a temporary root whose run directory
// lacks netns, mounts a tmpfs on run, creates netns in it, and a network
// namespace file bind-mounted there opens as that namespace.
func TestEnterCreatesAMissingNetnsDirectory(t *testing.T) {
	if !Enter(t) {
		return
	}
	runtime.LockOSThread()
	root := t.TempDir()
	run := filepath.Join(root, "run")
	if err := os.Mkdir(run, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", run, err)
	}
	if err := mountNetnsDir(root); err != nil {
		t.Fatalf("mountNetnsDir(%s): %v", root, err)
	}
	// A lazy unmount detaches run and every mount below it, so TempDir's cleanup,
	// which runs after this one, removes plain directories.
	t.Cleanup(func() {
		if err := syscall.Unmount(run, syscall.MNT_DETACH); err != nil {
			t.Errorf("unmount %s: %v", run, err)
		}
	})

	netnsDir := filepath.Join(run, "netns")
	info, err := os.Stat(netnsDir)
	if err != nil {
		t.Fatalf("stat %s: %v", netnsDir, err)
	}
	if !info.IsDir() {
		t.Fatalf("%s mode = %v, want a directory", netnsDir, info.Mode())
	}
	var rootStat, netnsStat syscall.Stat_t
	if err := syscall.Stat(root, &rootStat); err != nil {
		t.Fatalf("stat %s: %v", root, err)
	}
	if err := syscall.Stat(netnsDir, &netnsStat); err != nil {
		t.Fatalf("stat %s: %v", netnsDir, err)
	}
	if netnsStat.Dev == rootStat.Dev {
		t.Fatalf("%s is on the temporary root's device %d, want a tmpfs of its own", netnsDir, netnsStat.Dev)
	}

	probe := filepath.Join(netnsDir, "probe")
	if err := os.WriteFile(probe, nil, 0o444); err != nil {
		t.Fatalf("create %s: %v", probe, err)
	}
	source := "/proc/self/task/" + strconv.Itoa(unix.Gettid()) + "/ns/net"
	if err := unix.Mount(source, probe, "bind", unix.MS_BIND, ""); err != nil {
		t.Fatalf("bind %s on %s: %v", source, probe, err)
	}
	current, err := netns.Get()
	if err != nil {
		t.Fatalf("current namespace: %v", err)
	}
	defer current.Close() //nolint:errcheck // A handle closed at test end; nothing reads the result.
	found, err := netns.GetFromPath(probe)
	if err != nil {
		t.Fatalf("netns.GetFromPath(%s): %v", probe, err)
	}
	defer found.Close() //nolint:errcheck // A handle closed at test end; nothing reads the result.
	if !found.Equal(current) {
		t.Fatalf("namespace at %s = %v, want the test's own (%v)", probe, found, current)
	}
}

// TestEnterMountFailureNamesTheStep proves a failed mount step reports which
// step and which errno: a tmpfs cannot go on a regular file, so the error names
// the mount and its target and wraps ENOTDIR. childSetup fails the test with
// this error rather than skipping.
func TestEnterMountFailureNamesTheStep(t *testing.T) {
	if !Enter(t) {
		return
	}
	root := t.TempDir()
	run := filepath.Join(root, "run")
	if err := os.Mkdir(run, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", run, err)
	}
	target := filepath.Join(run, "netns")
	if err := os.WriteFile(target, nil, 0o644); err != nil {
		t.Fatalf("create %s: %v", target, err)
	}
	err := mountNetnsDir(root)
	if !errors.Is(err, syscall.ENOTDIR) {
		t.Fatalf("mountNetnsDir over a regular file: %v, want ENOTDIR", err)
	}
	if want := "mount tmpfs on " + target; !strings.Contains(err.Error(), want) {
		t.Fatalf("mountNetnsDir error %q does not name the step %q", err, want)
	}
}

// forwardProbe is a test's own flag, the shape of `-flowexport-ze-path`. Only
// TestEnterForwardsTheParentsFlags sets it, and only in the parent.
var forwardProbe = flag.String("userns-forward-probe", "", "Set by TestEnterForwardsTheParentsFlags in the parent only")

// TestEnterForwardsTheParentsFlags proves a flag the parent was given reaches
// the body in the child. The parent sets the flag on flag.CommandLine, as a
// command-line argument would, before Enter; the child never sets it, so the
// value it reads can only have arrived on its own command line.
func TestEnterForwardsTheParentsFlags(t *testing.T) {
	const want = "forwarded-by-the-parent"
	if os.Getenv(childEnv) == "" {
		if err := flag.Set("userns-forward-probe", want); err != nil {
			t.Fatalf("set the probe flag in the parent: %v", err)
		}
	}
	if !Enter(t) {
		return
	}
	if *forwardProbe != want {
		t.Fatalf("-userns-forward-probe in the child = %q, want %q: the parent's flag did not reach the child", *forwardProbe, want)
	}
}

// TestChildArgsForwardsSetFlagsAndWithholdsEnters proves the child's command line
// from a flag set: Enter's own selection first, then every flag that was set, in
// lexical order with its value, minus each flag childFlagsWithheld names. A flag
// that was registered but never set is not passed.
func TestChildArgsForwardsSetFlagsAndWithholdsEnters(t *testing.T) {
	fs := flag.NewFlagSet("probe", flag.ContinueOnError)
	fs.String("flowexport-ze-path", "", "")
	fs.Bool("rsvp-verbose", false, "")
	fs.String("never-set", "default", "")
	fs.String("test.gocoverdir", "", "")
	fs.Duration("test.timeout", 0, "")
	for name := range childFlagsWithheld {
		fs.String(name, "", "")
	}
	args := []string{
		"-flowexport-ze-path=/tmp/ze", "-rsvp-verbose", "-test.gocoverdir=/tmp/cover", "-test.timeout=5m",
	}
	for name := range childFlagsWithheld {
		args = append(args, "-"+name+"=parent-value")
	}
	if err := fs.Parse(args); err != nil {
		t.Fatalf("parse the probe flags: %v", err)
	}
	got := childArgs(fs, "TestProbe")
	want := []string{
		"-test.run", "^TestProbe$", "-test.v", "-test.count=1",
		"-flowexport-ze-path=/tmp/ze", "-rsvp-verbose=true", "-test.gocoverdir=/tmp/cover", "-test.timeout=5m0s",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("childArgs =\n%q\nwant\n%q", got, want)
	}
}

// TestChildResultReadsOnlyTheTestsOwnLine proves the guard against a child that
// ran nothing: only a column-0 result line naming exactly the test counts, so
// an output with no tests, a subtest's line or another test's line answers
// childRanNothing or the top-level verdict, never a pass or skip by accident.
func TestChildResultReadsOnlyTheTestsOwnLine(t *testing.T) {
	cases := []struct {
		name string
		out  string
		want childOutcome
	}{
		{"pass", "=== RUN   TestX\n--- PASS: TestX (0.01s)\nPASS\n", childPassed},
		{"skip", "=== RUN   TestX\n    x_test.go:1: no kernel\n--- SKIP: TestX (0.00s)\nPASS\n", childSkipped},
		{"fail", "=== RUN   TestX\n--- FAIL: TestX (0.00s)\nFAIL\n", childFailed},
		{"no tests ran", "testing: warning: no tests to run\nPASS\n", childRanNothing},
		{"empty output", "", childRanNothing},
		{"another test whose name extends it", "=== RUN   TestXY\n--- PASS: TestXY (0.00s)\nPASS\n", childRanNothing},
		{"subtest skip only", "=== RUN   TestX/sub\n    --- SKIP: TestX/sub (0.00s)\nPASS\n", childRanNothing},
		{"subtest skip under a pass", "    --- SKIP: TestX/sub (0.00s)\n--- PASS: TestX (0.00s)\nPASS\n", childPassed},
	}
	for _, c := range cases {
		if got := childResult("TestX", []byte(c.out)); got != c.want {
			t.Errorf("%s: childResult = %d, want %d", c.name, got, c.want)
		}
	}
}

// TestEnterRefusesAChildThatRanNoTest proves the guard end to end over the real
// re-executed binary: the command Enter builds, asked for a test name the binary
// does not hold, exits 0 with no result line, and childResult answers
// childRanNothing, the outcome on which Enter fails the parent. The namespace
// clone flags are cleared, because a child that selects no test never reaches
// childSetup, so the run needs no namespace and cannot be refused one.
func TestEnterRefusesAChildThatRanNoTest(t *testing.T) {
	const name = "TestNoSuchTestInThisBinary"
	cmd := childCommand(t, name)
	cmd.SysProcAttr = nil
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("child selecting no test exited %v, want 0:\n%s", err, out)
	}
	if got := childResult(name, out); got != childRanNothing {
		t.Fatalf("childResult over a child that ran nothing = %d, want childRanNothing (%d):\n%s", got, childRanNothing, out)
	}
}

// TestEnterRejectsAnInvalidExecutableName proves that a startup error fails
// rather than skips the calling test. The subprocess gives Enter a binary name
// containing NUL, which Go rejects before clone, even on a host denying userns.
// Only the subprocess changes os.Args, so other tests keep their executable.
func TestEnterRejectsAnInvalidExecutableName(t *testing.T) {
	const probeEnv = "ZE_TEST_USERNS_INVALID_EXECUTABLE"
	if os.Getenv(probeEnv) == "1" {
		os.Args[0] += "\x00"
		if Enter(t) {
			t.Fatal("Enter ran the test body with an invalid executable name")
		}
		t.Fatal("Enter returned without failing an invalid executable name")
	}

	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run", "^"+t.Name()+"$", "-test.v")
	cmd.Env = append(os.Environ(), childEnv+"=", probeEnv+"=1")
	out, err := cmd.CombinedOutput()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("Enter with an invalid executable name exited %v, want a failed test:\n%s", err, out)
	}
	if exitErr.ExitCode() != 1 {
		t.Fatalf("Enter with an invalid executable name exited %d, want 1:\n%s", exitErr.ExitCode(), out)
	}
	if got := childResult(t.Name(), out); got != childFailed {
		t.Fatalf("Enter with an invalid executable name reported %d, want childFailed:\n%s", got, out)
	}
	if !strings.Contains(string(out), syscall.EINVAL.Error()) {
		t.Fatalf("Enter lost the invalid executable error %q:\n%s", syscall.EINVAL, out)
	}
}

// TestEnterRefusalNamesTheFix proves the failure diagnostic retains the kernel
// error and names the setup step to use when namespace policy caused it.
func TestEnterRefusalNamesTheFix(t *testing.T) {
	text := childFailure(syscall.EPERM)
	for _, want := range []string{syscall.EPERM.Error(), "./le setup install", "userns-unrestricted"} {
		if !strings.Contains(text, want) {
			t.Errorf("failure text %q does not contain %q", text, want)
		}
	}
}
