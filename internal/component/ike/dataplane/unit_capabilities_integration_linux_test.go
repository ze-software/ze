// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- the ESP form fast path and the override guard under the systemd unit's capabilities
// Related: espform_fastpath_linux.go -- loadESPFormFastPathCollection, the fast path's load
// Related: espform_program_linux.go -- espFormFastPathSpec, the programs and maps it loads
// Related: override_guard_linux.go -- overrideGuard.attach, the guard's load and attach
// Related: ../../../core/privilege/capabilities.go -- DaemonCapabilities, the set the unit grants
//
// VALIDATES: the IPsec ESP form fast path's collection loads and both of its
// programs attach at the head of a link's TCX ingress, and the application
// override guard loads, attaches at the cgroup2 root with its link reaching the
// cgroup, and detaches, each from a process that is not root and holds exactly the
// capabilities the systemd unit Ze installs grants (privilege.DaemonCapabilities),
// through the dataplane's own loadESPFormFastPathCollection and
// xfrmBackend.AttachOverrideGuard.
// PREVENTS: a hand-assembled program the verifier accepts only from a privileged
// loader. Without CAP_PERFMON the verifier applies its unprivileged rules:
// bpf_check sets "env->allow_ptr_leaks = bpf_allow_ptr_leaks(env->prog->aux->token);",
// and bpf_allow_ptr_leaks, bpf_allow_uninit_stack, bpf_bypass_spec_v1 and
// bpf_bypass_spec_v4 each ask "bpf_token_capable(token, CAP_PERFMON)"
// (kernel/bpf/verifier.c, include/linux/bpf.h, v7.0). Every other test here loads
// as root, which hides those rules.
// METHOD: privileged QEMU (`//ze:guest-root`, `./le test qemu all`). Each root test
// re-runs this test binary as TestDataplaneUnitCapabilitiesHelperProcess under uid
// and gid 65534 with the unit's set as its ambient capabilities, which execve makes
// its effective set, in a network namespace the root parent's clone creates for it.
// The helper checks its own uid and CapEff, then loads. It is the loader, so the
// guard exempts it, and the guard governs only the helper's own namespace, so no
// other process in the guest meets it. The helper creates the fast path's dummy
// link in that namespace with the unit's CAP_NET_ADMIN, and the link goes with the
// namespace.

//go:build linux

package dataplane

import (
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/vishvananda/netlink"

	"github.com/ze-software/ze/internal/core/privilege"
)

// unitCapsModeEnv names the helper's mode, and marks the run as the helper's.
// The modes are the two loads the helper can run.
const (
	unitCapsModeEnv     = "ZE_TEST_DATAPLANE_UNIT_CAPS_MODE"
	unitCapsModeESPForm = "esp-form-fast-path"
	unitCapsModeGuard   = "override-guard"
)

// unitCapsUnprivilegedID is the uid and gid the helper runs as: nobody and
// nogroup, which hold no capability of their own.
const unitCapsUnprivilegedID = 65534

// unitCapsLink is the dummy link the fast path helper attaches to, in the
// helper's own network namespace.
const unitCapsLink = "zeucap0"

// unitCapsLoaded opens the line the helper logs, followed by its mode, once every
// load and attach succeeded. The parent requires it, so a helper that ran nothing
// (a renamed test, a -test.run that matches none) cannot pass.
const unitCapsLoaded = "unit-capabilities: loaded and attached:"

// TestESPFormFastPathLoadsUnderTheUnitCapabilities loads the ESP form fast path
// and attaches both programs at the TCX head, in a process holding exactly the
// systemd unit's capabilities, as a non-root uid. A verifier refusal fails it
// with the verifier's log.
//
//ze:guest-root
func TestESPFormFastPathLoadsUnderTheUnitCapabilities(t *testing.T) {
	runUnitCapsHelper(t, unitCapsModeESPForm)
}

// TestOverrideGuardLoadsUnderTheUnitCapabilities attaches the application
// override guard at the cgroup2 root and detaches it, in a process holding
// exactly the systemd unit's capabilities, as a non-root uid. A verifier refusal
// fails it with the verifier's log.
//
//ze:guest-root
func TestOverrideGuardLoadsUnderTheUnitCapabilities(t *testing.T) {
	runUnitCapsHelper(t, unitCapsModeGuard)
}

// runUnitCapsHelper starts the helper in mode, under uid and gid
// unitCapsUnprivilegedID with the unit's set as its ambient capabilities, in a
// network namespace of its own, and fails unless it exits 0 and reports the load.
func runUnitCapsHelper(t *testing.T, mode string) {
	t.Helper()
	if os.Geteuid() != 0 {
		t.Skip("requires privileged QEMU: the test starts a helper in a network namespace of its own, under another uid with ambient capabilities, which needs root; run `./le test qemu all`")
	}
	capabilities := privilege.DaemonCapabilities()
	ambient := make([]uintptr, 0, len(capabilities))
	for _, c := range capabilities {
		ambient = append(ambient, uintptr(c.Bit))
	}
	cmd := exec.CommandContext(t.Context(), os.Args[0],
		"-test.run=^TestDataplaneUnitCapabilitiesHelperProcess$", "-test.count=1", "-test.v")
	cmd.Env = append(os.Environ(), unitCapsModeEnv+"="+mode)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags:  syscall.CLONE_NEWNET,
		Credential:  &syscall.Credential{Uid: unitCapsUnprivilegedID, Gid: unitCapsUnprivilegedID},
		AmbientCaps: ambient,
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("the %s helper, uid %d holding the unit's capabilities, failed: %v\n%s", mode, unitCapsUnprivilegedID, err, out)
	}
	if !strings.Contains(string(out), unitCapsLoaded+" "+mode) {
		t.Fatalf("the %s helper exited 0 without reporting %q, so it loaded nothing:\n%s", mode, unitCapsLoaded+" "+mode, out)
	}
	t.Logf("helper output:\n%s", out)
}

// TestDataplaneUnitCapabilitiesHelperProcess is not a test: it is the loader the
// unit capability tests start, and it returns at once unless unitCapsModeEnv
// names a mode.
//
// test-asserts-nothing: subprocess entry point; the unit capability tests in this
// file assert its exit status and output.
func TestDataplaneUnitCapabilitiesHelperProcess(t *testing.T) {
	mode := os.Getenv(unitCapsModeEnv)
	if mode == "" {
		return
	}
	requireExactlyTheUnitCapabilities(t)
	switch mode {
	case unitCapsModeESPForm:
		loadESPFormFastPathUnderUnitCaps(t)
	case unitCapsModeGuard:
		attachOverrideGuardUnderUnitCaps(t)
	default:
		t.Fatalf("unknown helper mode %q", mode)
	}
	t.Log(unitCapsLoaded + " " + mode)
}

// loadESPFormFastPathUnderUnitCaps creates and raises unitCapsLink, loads the
// fast path's collection through loadESPFormFastPathCollection, and attaches both
// of its programs at the head of the link's TCX ingress, as the fast path does.
func loadESPFormFastPathUnderUnitCaps(t *testing.T) {
	t.Helper()
	attrs := netlink.NewLinkAttrs()
	attrs.Name = unitCapsLink
	if err := netlink.LinkAdd(&netlink.Dummy{LinkAttrs: attrs}); err != nil {
		t.Fatalf("create the dummy link %s with the unit's CAP_NET_ADMIN: %v", unitCapsLink, err)
	}
	dummy, err := netlink.LinkByName(unitCapsLink)
	if err != nil {
		t.Fatalf("read back the dummy link %s: %v", unitCapsLink, err)
	}
	if err := netlink.LinkSetUp(dummy); err != nil {
		t.Fatalf("bring the dummy link %s up: %v", unitCapsLink, err)
	}

	coll, err := loadESPFormFastPathCollection()
	if err != nil {
		failUnderUnitCaps(t, "load the ESP form fast path", err)
	}
	defer coll.Close()

	var attached []link.Link
	defer func() {
		for _, l := range attached {
			if err := l.Close(); err != nil {
				t.Errorf("release a TCX link of %s: %v", unitCapsLink, err)
			}
		}
	}()
	for _, name := range []string{espFormFastPathEthernetProg, espFormFastPathL3Prog} {
		tcx, err := link.AttachTCX(link.TCXOptions{
			Interface: dummy.Attrs().Index,
			Program:   coll.Programs[name],
			Attach:    ebpf.AttachTCXIngress,
			Anchor:    link.Head(),
		})
		if err != nil {
			t.Fatalf("attach %s at the head of %s's TCX ingress under the unit's capabilities: %v", name, unitCapsLink, err)
		}
		attached = append(attached, tcx)
	}
}

// attachOverrideGuardUnderUnitCaps attaches the override guard through the
// backend's AttachOverrideGuard, requires the state to read attached, and
// detaches it.
func attachOverrideGuardUnderUnitCaps(t *testing.T) {
	t.Helper()
	b := &xfrmBackend{}
	if err := b.AttachOverrideGuard(); err != nil {
		failUnderUnitCaps(t, "attach the override guard", err)
	}
	state, err := b.OverrideGuardState()
	if err != nil {
		t.Fatalf("read the override guard's state under the unit's capabilities: %v", err)
	}
	if state.Status != OverrideGuardAttached {
		t.Fatalf("the override guard's state is %v after the attach, want %v", state.Status, OverrideGuardAttached)
	}
	if err := b.DetachOverrideGuard(); err != nil {
		t.Fatalf("detach the override guard under the unit's capabilities: %v", err)
	}
}

// failUnderUnitCaps fails the helper for err, with the verifier's whole log when
// the verifier refused a program, which is the refusal these tests exist to catch.
func failUnderUnitCaps(t *testing.T, what string, err error) {
	t.Helper()
	if ve, ok := errors.AsType[*ebpf.VerifierError](err); ok {
		t.Fatalf("%s: the verifier refused the program under the unit's capabilities: %v\n%+v", what, err, ve)
	}
	t.Fatalf("%s under the unit's capabilities: %v", what, err)
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
