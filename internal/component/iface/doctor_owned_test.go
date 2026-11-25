// Design: docs/features/interfaces.md -- plugin-owned devices (bridges and links)
// Related: doctor_owned.go -- the check and the raise and clear helpers under test

package iface

import (
	"errors"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
)

// ownedFindings runs the owned-device check and answers its findings.
func ownedFindings() []diagnostic.Diagnostic {
	return checkOwnedDevices(diagnostic.DoctorCheckContext{})
}

// resetOwnedReport empties the report bus before and after a test, so a
// warning one test raised never answers for another.
func resetOwnedReport(t *testing.T) {
	t.Helper()
	report.ResetForTest()
	t.Cleanup(report.ResetForTest)
}

// runOwnedPass runs one owned pass alone against b, as a request does.
func runOwnedPass(b Backend) error {
	reconcileMu.Lock()
	defer reconcileMu.Unlock()
	return reconcileOwnedLocked(b, nil)
}

// TestOwnedDeviceDoctorCheckIsRegistered proves `show doctor` runs the
// owned-device check and that it declares both codes the owned-device log
// lines carry, which is what the doctor-coverage gate counts.
// Method: read the runtime-phase registry the daemon's doctor reads.
func TestOwnedDeviceDoctorCheckIsRegistered(t *testing.T) {
	for _, check := range diagnostic.DoctorChecksForPhase(diagnostic.DoctorPhaseRuntime) {
		if check.Name != "iface-owned-devices" {
			continue
		}
		for _, code := range []string{CodeDoctorIfaceOwnedDeviceCleanup, CodeDoctorIfaceOwnedDeviceReconcile} {
			if !slices.Contains(check.Codes, code) {
				t.Errorf("the iface-owned-devices check declares %v, want it to declare %s", check.Codes, code)
			}
		}
		return
	}
	t.Fatal("no runtime doctor check named iface-owned-devices is registered")
}

// TestWithdrawFailureIsReportedUntilACleanPass proves a withdrawal that could
// not delete an ended plugin's devices shows in the doctor check with the
// restart that redeclares the set, and that the next clean owned pass clears
// it. Method: the scripted backend refuses its listing during the withdrawal,
// then answers it for the pass that follows.
func TestWithdrawFailureIsReportedUntilACleanPass(t *testing.T) {
	resetOwnedReport(t)
	fb := &fakeBackend{}
	useOwnedFakeBackend(t, fb)
	deviceOwnerMu.Lock()
	ownedBridgeOwners[ownedTestOwner] = map[string]OwnedBridgeSpec{ownedTestPrefix + "-net": {}}
	deviceOwnerMu.Unlock()

	if len(ownedFindings()) != 0 {
		t.Fatalf("findings %v before any failure, want none", ownedFindings())
	}
	fb.listErr = errors.New("netlink dump interrupted")
	if err := WithdrawOwnedDevices(ownedTestOwner); err == nil {
		t.Fatal("the withdrawal answered no error while the listing failed")
	}
	findings := ownedFindings()
	if len(findings) != 1 {
		t.Fatalf("findings %v after a failed withdrawal, want exactly one", findings)
	}
	if findings[0].Code != CodeDoctorIfaceOwnedDeviceCleanup {
		t.Errorf("finding code %q, want %q", findings[0].Code, CodeDoctorIfaceOwnedDeviceCleanup)
	}
	if want := "ze request plugin restart " + ownedTestOwner; findings[0].Fix != want {
		t.Errorf("finding fix %q, want %q", findings[0].Fix, want)
	}

	fb.listErr = nil
	if err := runOwnedPass(fb); err != nil {
		t.Fatalf("clean owned pass: %v", err)
	}
	if remaining := ownedFindings(); len(remaining) != 0 {
		t.Errorf("findings %v after a clean pass, want none", remaining)
	}
}

// TestHalfMadeDeviceWarningClearsOnlyOnceTheDeviceIsGone proves a warning
// about one device by name survives a clean pass whose listing still holds
// that device, and clears on the first clean pass after it is gone.
// Method: raise the warning the kernel backend rollback raises, keep an unmarked
// device of that name in the scripted backend for one pass, then remove it.
func TestHalfMadeDeviceWarningClearsOnlyOnceTheDeviceIsGone(t *testing.T) {
	resetOwnedReport(t)
	fb := &fakeBackend{}
	useOwnedFakeBackend(t, fb)
	const halfMade = ownedTestPrefix + "-half"
	fb.ifaces[halfMade] = fakeIface{name: halfMade, linkType: TypeBridge, index: 50}
	report.RaiseWarningFix(OwnedDeviceReportSource, CodeDoctorIfaceOwnedDeviceReconcile,
		OwnedDeviceSubjectDevice+halfMade, "half-made owned bridge left", "ip link delete "+halfMade)

	if err := runOwnedPass(fb); err != nil {
		t.Fatalf("owned pass with the device present: %v", err)
	}
	findings := ownedFindings()
	if len(findings) != 1 || findings[0].Code != CodeDoctorIfaceOwnedDeviceReconcile {
		t.Fatalf("findings %v while the device is listed, want the one reconcile finding", findings)
	}

	delete(fb.ifaces, halfMade)
	if err := runOwnedPass(fb); err != nil {
		t.Fatalf("owned pass with the device gone: %v", err)
	}
	if remaining := ownedFindings(); len(remaining) != 0 {
		t.Errorf("findings %v after the device is gone, want none", remaining)
	}
}
