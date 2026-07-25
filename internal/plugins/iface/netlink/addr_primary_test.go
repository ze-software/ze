package ifacenetlink

import (
	"errors"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func mustPrefix(t *testing.T, s string) netip.Prefix {
	t.Helper()
	p, err := netip.ParsePrefix(s)
	if err != nil {
		t.Fatalf("parse prefix %q: %v", s, err)
	}
	return p
}

// TestFlushedByDeletePrimaryTakesSameSubnetSecondary is the reload bug in one
// assertion: a same-subnet renumber leaves the NEW address as a secondary of
// the OLD one, so deleting the old (primary) address makes the kernel delete
// the new one too and the interface ends up with no address at all.
//
// VALIDATES: deleting a primary IPv4 address reports the same-subnet secondaries the kernel would delete with it.
// PREVENTS: a make-before-break address swap silently emptying the interface.
func TestFlushedByDeletePrimaryTakesSameSubnetSecondary(t *testing.T) {
	target := deviceAddress{Prefix: mustPrefix(t, "10.77.0.1/24")}
	existing := []deviceAddress{
		target,
		{Prefix: mustPrefix(t, "10.77.0.2/24"), Secondary: true},
	}

	doomed := flushedByDelete(target, existing)
	if len(doomed) != 1 || doomed[0] != mustPrefix(t, "10.77.0.2/24") {
		t.Fatalf("flushedByDelete = %v, want [10.77.0.2/24]", doomed)
	}
}

// TestFlushedByDeleteSecondaryIsIsolated verifies that removing a secondary
// never cascades: the kernel only flushes when the PRIMARY goes away.
//
// VALIDATES: deleting a secondary address endangers nothing.
// PREVENTS: needlessly rewriting promote_secondaries on every address removal.
func TestFlushedByDeleteSecondaryIsIsolated(t *testing.T) {
	target := deviceAddress{Prefix: mustPrefix(t, "10.77.0.2/24"), Secondary: true}
	existing := []deviceAddress{
		{Prefix: mustPrefix(t, "10.77.0.1/24")},
		target,
	}

	if doomed := flushedByDelete(target, existing); len(doomed) != 0 {
		t.Fatalf("flushedByDelete = %v, want none", doomed)
	}
}

// TestFlushedByDeleteOtherSubnetsSurvive verifies the kernel's match is on
// network AND mask length, so addresses in other subnets (or the same network
// with a different prefix length) are not at risk.
//
// VALIDATES: only same-network, same-prefix-length secondaries are reported.
// PREVENTS: over-broad hazard detection rejecting safe removals.
func TestFlushedByDeleteOtherSubnetsSurvive(t *testing.T) {
	target := deviceAddress{Prefix: mustPrefix(t, "10.77.0.1/24")}
	existing := []deviceAddress{
		target,
		{Prefix: mustPrefix(t, "10.78.0.2/24"), Secondary: true},
		{Prefix: mustPrefix(t, "10.77.0.3/25"), Secondary: true},
	}

	if doomed := flushedByDelete(target, existing); len(doomed) != 0 {
		t.Fatalf("flushedByDelete = %v, want none", doomed)
	}
}

// TestFlushedByDeleteIPv6HasNoPrimarySecondary verifies IPv6 is exempt: the
// kernel has no primary/secondary distinction for it.
//
// VALIDATES: IPv6 removals report no endangered siblings.
// PREVENTS: touching an IPv4-only sysctl on an IPv6-only change.
func TestFlushedByDeleteIPv6HasNoPrimarySecondary(t *testing.T) {
	target := deviceAddress{Prefix: mustPrefix(t, "2001:db8::1/64")}
	existing := []deviceAddress{
		target,
		{Prefix: mustPrefix(t, "2001:db8::2/64"), Secondary: true},
	}

	if doomed := flushedByDelete(target, existing); len(doomed) != 0 {
		t.Fatalf("flushedByDelete = %v, want none", doomed)
	}
}

// fakeSysctl replaces the procfs seams for one test.
type fakeSysctl struct {
	values    map[string]string
	readErr   map[string]error
	writeErr  map[string]error
	writeLog  []string
	writeCall int
}

func (f *fakeSysctl) install(t *testing.T) {
	t.Helper()
	prevRead, prevWrite := addrSysctlRead, addrSysctlWrite
	addrSysctlRead = func(path string) (string, error) {
		if err := f.readErr[path]; err != nil {
			return "", err
		}
		value, ok := f.values[path]
		if !ok {
			return "", errors.New("no such file")
		}
		return value, nil
	}
	addrSysctlWrite = func(path, value string) error {
		f.writeCall++
		if err := f.writeErr[path]; err != nil {
			return err
		}
		f.writeLog = append(f.writeLog, path+"="+value)
		if f.values == nil {
			f.values = map[string]string{}
		}
		f.values[path] = value
		return nil
	}
	t.Cleanup(func() { addrSysctlRead, addrSysctlWrite = prevRead, prevWrite })
}

func withTempProcRoot(t *testing.T) string {
	t.Helper()
	prev := procNetRoot
	procNetRoot = t.TempDir()
	t.Cleanup(func() { procNetRoot = prev })
	return procNetRoot
}

// TestEnsureDeleteIsolatedEnablesPromoteSecondaries verifies the fix: before a
// hazardous delete, promote_secondaries is turned on so the kernel promotes a
// secondary instead of flushing the subnet.
//
// VALIDATES: a hazardous IPv4 primary delete enables net.ipv4.conf.<dev>.promote_secondaries.
// PREVENTS: an address swap on one subnet leaving the interface with no address.
func TestEnsureDeleteIsolatedEnablesPromoteSecondaries(t *testing.T) {
	root := withTempProcRoot(t)
	knob := filepath.Join(root, "ipv4", "conf", "zdiag0", "promote_secondaries")
	fake := &fakeSysctl{values: map[string]string{knob: "0"}}
	fake.install(t)

	target := deviceAddress{Prefix: mustPrefix(t, "10.77.0.1/24")}
	existing := []deviceAddress{target, {Prefix: mustPrefix(t, "10.77.0.2/24"), Secondary: true}}

	if err := ensureDeleteIsolated("zdiag0", target, existing); err != nil {
		t.Fatalf("ensureDeleteIsolated: %v", err)
	}
	want := knob + "=1"
	if len(fake.writeLog) != 1 || fake.writeLog[0] != want {
		t.Fatalf("writeLog = %v, want [%s]", fake.writeLog, want)
	}
}

// TestEnsureDeleteIsolatedSkipsWriteWhenAlreadyEnabled verifies the knob is not
// rewritten when the kernel already promotes. Driven through a VLAN
// sub-interface name, the shape that reproduces the same bug at the
// applyConfig layer (TestIntegrationApplyConfigVLANUnitAddressReconcile).
//
// VALIDATES: an already-enabled promote_secondaries triggers no procfs write.
// PREVENTS: a needless write per address removal on every reload.
func TestEnsureDeleteIsolatedSkipsWriteWhenAlreadyEnabled(t *testing.T) {
	root := withTempProcRoot(t)
	knob := filepath.Join(root, "ipv4", "conf", "parent0.200", "promote_secondaries")
	fake := &fakeSysctl{values: map[string]string{knob: "1"}}
	fake.install(t)

	target := deviceAddress{Prefix: mustPrefix(t, "10.60.200.1/24")}
	existing := []deviceAddress{target, {Prefix: mustPrefix(t, "10.60.200.2/24"), Secondary: true}}

	if err := ensureDeleteIsolated("parent0.200", target, existing); err != nil {
		t.Fatalf("ensureDeleteIsolated: %v", err)
	}
	if fake.writeCall != 0 {
		t.Fatalf("writeCall = %d, want 0", fake.writeCall)
	}
}

// TestEnsureDeleteIsolatedNoOpWithoutHazard verifies an ordinary removal does
// not touch kernel knobs at all.
//
// VALIDATES: a removal with no same-subnet secondary performs no sysctl write.
// PREVENTS: ze flipping operator-visible kernel state it does not need.
func TestEnsureDeleteIsolatedNoOpWithoutHazard(t *testing.T) {
	withTempProcRoot(t)
	fake := &fakeSysctl{}
	fake.install(t)

	target := deviceAddress{Prefix: mustPrefix(t, "10.77.0.1/24")}

	if err := ensureDeleteIsolated("zdiag0", target, []deviceAddress{target}); err != nil {
		t.Fatalf("ensureDeleteIsolated: %v", err)
	}
	if fake.writeCall != 0 {
		t.Fatalf("writeCall = %d, want 0", fake.writeCall)
	}
}

// TestEnsureDeleteIsolatedRejectsWhenKnobUnwritable verifies exact-or-reject:
// when ze cannot stop the kernel from cascading, the removal fails loudly and
// the error names the addresses that would have been destroyed.
//
// VALIDATES: an unwritable promote_secondaries makes a hazardous removal return an error naming the endangered address.
// PREVENTS: silently destroying addresses the operator's config still asks for.
func TestEnsureDeleteIsolatedRejectsWhenKnobUnwritable(t *testing.T) {
	root := withTempProcRoot(t)
	knob := filepath.Join(root, "ipv4", "conf", "zdiag0", "promote_secondaries")
	fake := &fakeSysctl{
		values:   map[string]string{knob: "0"},
		writeErr: map[string]error{knob: errors.New("read-only file system")},
	}
	fake.install(t)

	target := deviceAddress{Prefix: mustPrefix(t, "10.77.0.1/24")}
	existing := []deviceAddress{target, {Prefix: mustPrefix(t, "10.77.0.2/24"), Secondary: true}}

	err := ensureDeleteIsolated("zdiag0", target, existing)
	if err == nil {
		t.Fatal("ensureDeleteIsolated returned nil, want an error naming the endangered address")
	}
	if !strings.Contains(err.Error(), "10.77.0.2/24") {
		t.Fatalf("error %q does not name the endangered address 10.77.0.2/24", err)
	}
	if !strings.Contains(err.Error(), "10.77.0.1/24") {
		t.Fatalf("error %q does not name the address being removed", err)
	}
}

// TestPromoteSecondariesKnobUsesLiteralDeviceName verifies the procfs path uses
// the literal device name, so VLAN sub-interfaces (eth0.100) resolve.
//
// VALIDATES: the knob path is <root>/ipv4/conf/<device>/promote_secondaries.
// PREVENTS: dotted-sysctl-key escaping breaking VLAN sub-interface removals.
func TestPromoteSecondariesKnobUsesLiteralDeviceName(t *testing.T) {
	root := withTempProcRoot(t)
	got := promoteSecondariesKnob("parent0.200")
	want := filepath.Join(root, "ipv4", "conf", "parent0.200", "promote_secondaries")
	if got != want {
		t.Fatalf("promoteSecondariesKnob = %q, want %q", got, want)
	}
}

// TestSelectDeleteTargetAdoptsKernelSecondaryFlag verifies the target's
// primary/secondary state is taken from the device's live addresses, not from
// the caller-parsed CIDR (which carries no IFA_F_* flags at all).
//
// VALIDATES: selectDeleteTarget returns the target with the kernel's secondary flag.
// PREVENTS: treating a secondary as a primary and rewriting promote_secondaries needlessly.
func TestSelectDeleteTargetAdoptsKernelSecondaryFlag(t *testing.T) {
	want := mustPrefix(t, "10.77.0.2/24")
	existing := []deviceAddress{
		{Prefix: mustPrefix(t, "10.77.0.1/24")},
		{Prefix: want, Secondary: true},
	}

	got, ok := selectDeleteTarget(deviceAddress{Prefix: want}, existing)
	if !ok {
		t.Fatal("selectDeleteTarget reported the address as absent")
	}
	if !got.Secondary {
		t.Fatal("target did not adopt the kernel's IFA_F_SECONDARY classification")
	}
}

// TestSelectDeleteTargetReportsAbsentAddress verifies an address that is not on
// the device (and the empty-device boundary) is reported absent, so no kernel
// knob is touched for a removal that cannot happen.
//
// VALIDATES: selectDeleteTarget reports false for an address absent from the device, including an empty device.
// PREVENTS: enabling promote_secondaries on the error path of a removal that will fail anyway.
func TestSelectDeleteTargetReportsAbsentAddress(t *testing.T) {
	target := deviceAddress{Prefix: mustPrefix(t, "10.77.0.9/24")}

	if _, ok := selectDeleteTarget(target, nil); ok {
		t.Fatal("selectDeleteTarget on an address-less device reported the address present")
	}
	existing := []deviceAddress{{Prefix: mustPrefix(t, "10.77.0.1/24")}}
	if _, ok := selectDeleteTarget(target, existing); ok {
		t.Fatal("selectDeleteTarget reported an absent address as present")
	}
}

// TestFlushedByDeleteHostRoutesAreTheirOwnSubnet covers the /32 boundary: two
// host addresses are different networks under a /32 mask, so neither endangers
// the other.
//
// VALIDATES: /32 addresses are never same-subnet siblings of each other.
// PREVENTS: loopback/anycast /32 removals rejecting or flipping kernel knobs for no reason.
func TestFlushedByDeleteHostRoutesAreTheirOwnSubnet(t *testing.T) {
	target := deviceAddress{Prefix: mustPrefix(t, "10.77.0.1/32")}
	existing := []deviceAddress{
		target,
		{Prefix: mustPrefix(t, "10.77.0.2/32"), Secondary: true},
	}

	if doomed := flushedByDelete(target, existing); len(doomed) != 0 {
		t.Fatalf("flushedByDelete = %v, want none", doomed)
	}
}

// TestFlushedByDeleteReportsEveryEndangeredSecondary covers the above-two
// boundary: a subnet holding a primary and several secondaries loses ALL of the
// secondaries, so all of them must be reported.
//
// VALIDATES: every same-subnet secondary is reported, not just the first.
// PREVENTS: an error or log that under-reports how much a removal would destroy.
func TestFlushedByDeleteReportsEveryEndangeredSecondary(t *testing.T) {
	target := deviceAddress{Prefix: mustPrefix(t, "10.77.0.1/24")}
	existing := []deviceAddress{
		target,
		{Prefix: mustPrefix(t, "10.77.0.2/24"), Secondary: true},
		{Prefix: mustPrefix(t, "10.77.0.3/24"), Secondary: true},
		{Prefix: mustPrefix(t, "10.88.0.1/24"), Secondary: true},
	}

	doomed := flushedByDelete(target, existing)
	if len(doomed) != 2 {
		t.Fatalf("flushedByDelete = %v, want both 10.77.0.2/24 and 10.77.0.3/24", doomed)
	}
	for _, want := range []string{"10.77.0.2/24", "10.77.0.3/24"} {
		found := false
		for _, got := range doomed {
			if got.String() == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("flushedByDelete = %v, missing %s", doomed, want)
		}
	}
}

// TestFlushedByDeleteSoleAddressEndangersNothing covers the single-address
// boundary: with nothing else on the device there is nothing to cascade to.
//
// VALIDATES: removing the only address on a device endangers nothing.
// PREVENTS: a needless sysctl write on the most common removal of all.
func TestFlushedByDeleteSoleAddressEndangersNothing(t *testing.T) {
	target := deviceAddress{Prefix: mustPrefix(t, "10.77.0.1/24")}

	if doomed := flushedByDelete(target, []deviceAddress{target}); len(doomed) != 0 {
		t.Fatalf("flushedByDelete = %v, want none", doomed)
	}
}

// TestOsAddrSysctlReadTrimsProcfsNewline exercises the REAL procfs reader
// against a real file. procfs terminates scalar knobs with a newline, and the
// already-enabled check compares against "1" exactly: without trimming, ze
// would rewrite the knob on every single address removal.
//
// VALIDATES: osAddrSysctlRead strips the trailing newline procfs appends.
// PREVENTS: operator-visible sysctl churn on every address removal.
func TestOsAddrSysctlReadTrimsProcfsNewline(t *testing.T) {
	root := withTempProcRoot(t)
	dir := filepath.Join(root, "ipv4", "conf", "zdiag0")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	knob := filepath.Join(dir, "promote_secondaries")
	if err := os.WriteFile(knob, []byte("1\n"), 0o644); err != nil {
		t.Fatalf("write knob: %v", err)
	}

	got, err := osAddrSysctlRead(promoteSecondariesKnob("zdiag0"))
	if err != nil {
		t.Fatalf("osAddrSysctlRead: %v", err)
	}
	if got != "1" {
		t.Fatalf("osAddrSysctlRead = %q, want %q", got, "1")
	}
}

// TestOsAddrSysctlWriteRoundTrips exercises the real writer and reader
// together, so the seam's production implementation is not assumed correct.
//
// VALIDATES: osAddrSysctlWrite writes a value osAddrSysctlRead reads back.
// PREVENTS: a procfs seam that only ever works against the test fake.
func TestOsAddrSysctlWriteRoundTrips(t *testing.T) {
	root := withTempProcRoot(t)
	dir := filepath.Join(root, "ipv4", "conf", "zdiag0")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	knob := promoteSecondariesKnob("zdiag0")
	if err := os.WriteFile(knob, []byte("0\n"), 0o644); err != nil {
		t.Fatalf("seed knob: %v", err)
	}

	if err := osAddrSysctlWrite(knob, "1"); err != nil {
		t.Fatalf("osAddrSysctlWrite: %v", err)
	}
	got, err := osAddrSysctlRead(knob)
	if err != nil {
		t.Fatalf("osAddrSysctlRead: %v", err)
	}
	if got != "1" {
		t.Fatalf("read back %q, want %q", got, "1")
	}
}

// TestEnsureDeleteIsolatedWritesWhenKnobUnreadable verifies the fail-closed
// direction of the read: a knob that cannot be read is NOT taken for
// already-enabled, so the guard still writes rather than silently standing down.
//
// VALIDATES: an unreadable promote_secondaries still triggers the enabling write.
// PREVENTS: a read error being mistaken for "the kernel already promotes".
func TestEnsureDeleteIsolatedWritesWhenKnobUnreadable(t *testing.T) {
	root := withTempProcRoot(t)
	knob := filepath.Join(root, "ipv4", "conf", "zdiag0", "promote_secondaries")
	fake := &fakeSysctl{
		values:  map[string]string{knob: "1"},
		readErr: map[string]error{knob: errors.New("permission denied")},
	}
	fake.install(t)

	target := deviceAddress{Prefix: mustPrefix(t, "10.77.0.1/24")}
	existing := []deviceAddress{target, {Prefix: mustPrefix(t, "10.77.0.2/24"), Secondary: true}}

	if err := ensureDeleteIsolated("zdiag0", target, existing); err != nil {
		t.Fatalf("ensureDeleteIsolated: %v", err)
	}
	if fake.writeCall != 1 {
		t.Fatalf("writeCall = %d, want 1 when the knob could not be read", fake.writeCall)
	}
}

// TestEnsureDeleteIsolatedErrorNamesEveryEndangeredAddress verifies the
// rejection message renders the full list, so an operator sees everything the
// removal would have destroyed rather than just the first casualty.
//
// VALIDATES: the rejection error names every endangered address.
// PREVENTS: an under-reporting error message hiding the true blast radius.
func TestEnsureDeleteIsolatedErrorNamesEveryEndangeredAddress(t *testing.T) {
	root := withTempProcRoot(t)
	knob := filepath.Join(root, "ipv4", "conf", "zdiag0", "promote_secondaries")
	fake := &fakeSysctl{
		values:   map[string]string{knob: "0"},
		writeErr: map[string]error{knob: errors.New("read-only file system")},
	}
	fake.install(t)

	target := deviceAddress{Prefix: mustPrefix(t, "10.77.0.1/24")}
	existing := []deviceAddress{
		target,
		{Prefix: mustPrefix(t, "10.77.0.2/24"), Secondary: true},
		{Prefix: mustPrefix(t, "10.77.0.3/24"), Secondary: true},
	}

	err := ensureDeleteIsolated("zdiag0", target, existing)
	if err == nil {
		t.Fatal("ensureDeleteIsolated returned nil, want an error")
	}
	for _, want := range []string{"10.77.0.2/24", "10.77.0.3/24"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error %q does not name endangered address %s", err, want)
		}
	}
	if !strings.Contains(err.Error(), ", ") {
		t.Fatalf("error %q does not render the endangered list with a separator", err)
	}
}
