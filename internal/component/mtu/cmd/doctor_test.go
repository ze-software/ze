package cmd

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/diagnostic"
)

// registeredMTULocalStateCheck finds the check through the registry, which
// is the entry point ze doctor runs it from.
func registeredMTULocalStateCheck(t *testing.T) diagnostic.DoctorCheck {
	t.Helper()
	for _, check := range diagnostic.DoctorChecksForPhase(diagnostic.DoctorPhasePreConfig) {
		if check.Name == doctorCheckName {
			return check
		}
	}
	t.Fatalf("doctor check %q is not registered in phase %s", doctorCheckName, diagnostic.DoctorPhasePreConfig)
	return diagnostic.DoctorCheck{}
}

func stubLocalStateReaders(t *testing.T, fragmentation, tcpMTUProbing error) {
	t.Helper()
	prevFrag, prevTCP := doctorFragmentation, doctorTCPMTUProbing
	doctorFragmentation = func() error { return fragmentation }
	doctorTCPMTUProbing = func() error { return tcpMTUProbing }
	t.Cleanup(func() { doctorFragmentation, doctorTCPMTUProbing = prevFrag, prevTCP })
}

// TestDoctorMTULocalStateReportsEachUnreadableSource: a failing procfs read
// and a failing sysctl read each yield one warning carrying the code, the
// source, the error and what show mtu reports without it.
func TestDoctorMTULocalStateReportsEachUnreadableSource(t *testing.T) {
	stubLocalStateReaders(t, errors.New("open /proc/net/snmp: no such file"), errors.New("sysctl backend absent"))
	check := registeredMTULocalStateCheck(t)
	if !slices.Contains(check.Codes, codeMTULocalState) {
		t.Errorf("check declares codes %v, missing %s", check.Codes, codeMTULocalState)
	}
	diags := check.Check(diagnostic.DoctorCheckContext{})
	if len(diags) != 2 {
		t.Fatalf("%d diagnostics, want 2: %+v", len(diags), diags)
	}
	wants := []string{"/proc/net/snmp", sysctlTCPMTUProbing}
	for i, d := range diags {
		if d.Code != codeMTULocalState {
			t.Errorf("diagnostic %d code %q, want %s", i, d.Code, codeMTULocalState)
		}
		if d.Severity != diagnostic.SeverityWarning {
			t.Errorf("diagnostic %d severity %q, want warning", i, d.Severity)
		}
		for _, want := range []string{wants[i], "still measures the path", "unreadable"} {
			if !strings.Contains(d.Message, want) {
				t.Errorf("diagnostic %d message %q does not name %s", i, d.Message, want)
			}
		}
	}
	// The code resolves once the binary entry point registers the built-in
	// table, which is what `ze explain` reads.
	diagnostic.RegisterBuiltinCodes()
	if diagnostic.Lookup(codeMTULocalState) == nil {
		t.Errorf("code %s is not in internal/core/diagnostic/codes.go", codeMTULocalState)
	}
}

// TestDoctorMTULocalStateSilentWhenBothRead: both readers answering yields
// no diagnostic, so a healthy host is not warned about.
func TestDoctorMTULocalStateSilentWhenBothRead(t *testing.T) {
	stubLocalStateReaders(t, nil, nil)
	diags := registeredMTULocalStateCheck(t).Check(diagnostic.DoctorCheckContext{})
	if len(diags) != 0 {
		t.Fatalf("%d diagnostics, want none: %+v", len(diags), diags)
	}
}
