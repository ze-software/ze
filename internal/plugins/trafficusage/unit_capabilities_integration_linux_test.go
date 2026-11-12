// Design: docs/architecture/traffic/traffic-usage.md -- traffic-usage loads under the systemd unit's capabilities
// Related: attach_linux.go -- tcxAttacher, the load and attach under test
// Related: ../../core/privilege/capabilities.go -- DaemonCapabilities, the set the unit grants
//
// VALIDATES: traffic-usage's eBPF collection loads, and its two programs attach to
// a link's TCX ingress and egress, from a process that is not root and holds
// exactly the capabilities the systemd unit Ze installs grants
// (privilege.DaemonCapabilities), in both shapes an operator configures (track-ip
// off and on) at the default map size, through the plugin's own Available and
// Attach.
// PREVENTS: a program the verifier accepts only from a privileged loader. Without
// CAP_PERFMON the verifier applies its unprivileged rules: bpf_check sets
// "env->allow_ptr_leaks = bpf_allow_ptr_leaks(env->prog->aux->token);", and
// bpf_allow_ptr_leaks, bpf_allow_uninit_stack, bpf_bypass_spec_v1 and
// bpf_bypass_spec_v4 each ask "bpf_token_capable(token, CAP_PERFMON)"
// (kernel/bpf/verifier.c, include/linux/bpf.h, v7.0). Every other test loads as
// root, which hides those rules. Also a map type, such as the LRU hash, that
// map_create refuses without CAP_BPF.
// METHOD: privileged QEMU (`//ze:guest-root`, `./le test qemu all`). The root test
// creates a dummy link and re-runs this test binary as
// TestTrafficUsageUnitCapabilitiesHelperProcess under uid and gid 65534 with the
// unit's set as its ambient capabilities, which execve makes its effective set.
// The helper checks its own uid and CapEff before it loads anything.

//go:build linux

package trafficusage

import (
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/cilium/ebpf"
	"github.com/vishvananda/netlink"

	"github.com/ze-software/ze/internal/core/privilege"
)

// unitCapsIfindexEnv carries the dummy link's index to the helper, and marks the
// run as the helper's.
const unitCapsIfindexEnv = "ZE_TEST_TRAFFIC_USAGE_UNIT_CAPS_IFINDEX"

// unitCapsUnprivilegedID is the uid and gid the helper runs as: nobody and
// nogroup, which hold no capability of their own.
const unitCapsUnprivilegedID = 65534

// unitCapsLoaded is the line the helper logs once every load and attach
// succeeded. The parent requires it, so a helper that ran nothing (a renamed
// test, a -test.run that matches none) cannot pass.
const unitCapsLoaded = "unit-capabilities: traffic-usage loaded and attached"

// TestTrafficUsageLoadsUnderTheUnitCapabilities runs traffic-usage's load and
// attach in a process holding exactly the systemd unit's capabilities, as a
// non-root uid. A verifier refusal fails it with the verifier's log.
//
//ze:guest-root
func TestTrafficUsageLoadsUnderTheUnitCapabilities(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("requires privileged QEMU: the test creates a link and starts a helper under another uid with ambient capabilities, which needs root; run `./le test qemu all`")
	}

	name := "zetu" + strconv.Itoa(os.Getpid())
	attrs := netlink.NewLinkAttrs()
	attrs.Name = name
	if err := netlink.LinkAdd(&netlink.Dummy{LinkAttrs: attrs}); err != nil {
		t.Fatalf("create the dummy link %s: %v", name, err)
	}
	dummy, err := netlink.LinkByName(name)
	if err != nil {
		t.Fatalf("read back the dummy link %s: %v", name, err)
	}
	t.Cleanup(func() {
		if err := netlink.LinkDel(dummy); err != nil {
			t.Errorf("remove the dummy link %s: %v", name, err)
		}
	})
	if err := netlink.LinkSetUp(dummy); err != nil {
		t.Fatalf("bring the dummy link %s up: %v", name, err)
	}

	var ambient []uintptr
	for _, c := range privilege.DaemonCapabilities() {
		ambient = append(ambient, uintptr(c.Bit))
	}
	cmd := exec.CommandContext(t.Context(), os.Args[0],
		"-test.run=^TestTrafficUsageUnitCapabilitiesHelperProcess$", "-test.count=1", "-test.v")
	cmd.Env = append(os.Environ(), unitCapsIfindexEnv+"="+strconv.Itoa(dummy.Attrs().Index))
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential:  &syscall.Credential{Uid: unitCapsUnprivilegedID, Gid: unitCapsUnprivilegedID},
		AmbientCaps: ambient,
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("the helper, uid %d holding the unit's capabilities, failed: %v\n%s", unitCapsUnprivilegedID, err, out)
	}
	if !strings.Contains(string(out), unitCapsLoaded) {
		t.Fatalf("the helper exited 0 without reporting %q, so it loaded nothing:\n%s", unitCapsLoaded, out)
	}
	t.Logf("helper output:\n%s", out)
}

// TestTrafficUsageUnitCapabilitiesHelperProcess is not a test: it is the loader
// TestTrafficUsageLoadsUnderTheUnitCapabilities starts, and it returns at once
// unless unitCapsIfindexEnv is set.
//
// test-asserts-nothing: subprocess entry point; TestTrafficUsageLoadsUnderTheUnitCapabilities
// asserts its exit status and output.
func TestTrafficUsageUnitCapabilitiesHelperProcess(t *testing.T) {
	index := os.Getenv(unitCapsIfindexEnv)
	if index == "" {
		return
	}
	ifindex, err := strconv.Atoi(index)
	if err != nil {
		t.Fatalf("%s=%q is not an interface index: %v", unitCapsIfindexEnv, index, err)
	}
	requireExactlyTheUnitCapabilities(t)

	// The plugin's own path: runEngine calls Available, the monitor calls Attach
	// with the interface's configured map size and track-ip.
	if err := (tcxAttacher{}).Available(); err != nil {
		t.Fatalf("Available under the unit's capabilities: %v", err)
	}
	for _, trackIP := range []bool{false, true} {
		att, err := tcxAttacher{}.Attach(ifindex, "", defaultMaxEntries, trackIP)
		if err != nil {
			if ve, ok := errors.AsType[*ebpf.VerifierError](err); ok {
				t.Fatalf("track-ip %v: the verifier refused a program under the unit's capabilities: %v\n%+v", trackIP, err, ve)
			}
			t.Fatalf("track-ip %v: load and attach under the unit's capabilities: %v", trackIP, err)
		}
		if err := att.Close(); err != nil {
			t.Fatalf("track-ip %v: detach: %v", trackIP, err)
		}
	}
	t.Log(unitCapsLoaded)
}

// requireExactlyTheUnitCapabilities fails unless this process is not root and
// its effective capability set is exactly the unit's: a helper that kept root's
// set, or lost one, proves nothing about the unit.
func requireExactlyTheUnitCapabilities(t *testing.T) {
	t.Helper()
	if os.Getuid() == 0 {
		t.Fatal("the helper runs as uid 0; it must run as a non-root uid to prove the unit's capabilities suffice")
	}
	if os.Geteuid() == 0 {
		t.Fatal("the helper runs with euid 0; it must run as a non-root uid to prove the unit's capabilities suffice")
	}
	var want uint64
	for _, c := range privilege.DaemonCapabilities() {
		want |= 1 << c.Bit
	}
	effective, err := unitCapsEffective()
	if err != nil {
		t.Fatalf("read the helper's effective capabilities: %v", err)
	}
	if effective != want {
		t.Fatalf("the helper's CapEff is %#016x; the unit grants exactly %#016x", effective, want)
	}
}

// unitCapsEffective reads the helper's effective capability set, one bit per
// privilege.Capability.Bit, from the CapEff line of /proc/self/status. The test
// reads the kernel's own record rather than a privilege export, so the check it
// feeds does not depend on the code it judges.
func unitCapsEffective() (uint64, error) {
	data, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return 0, err
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		if hex, ok := strings.CutPrefix(line, "CapEff:\t"); ok {
			return strconv.ParseUint(strings.TrimSpace(hex), 16, 64)
		}
	}
	return 0, errors.New("CapEff not found in /proc/self/status")
}
