// Design: docs/architecture/diagnostics/active-probes.md -- the doctor check, driven through the registry
//
// VALIDATES: AC-8 and the wiring row "Daemon start with no CAP_NET_RAW ->
// the registered doctor check": the check is discovered through the
// diagnostic registry, and it reports each socket outcome by its own code.
// PREVENTS: a check that exists but registers nowhere, and a missing
// dependency that surfaces only as a socket error at the moment of use.

package probe

import (
	"context"
	"net"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/ze-software/ze/internal/core/diagnostic"
)

// registeredICMPProbeCheck finds the check through the registry, which is
// the entry point ze doctor runs it from.
func registeredICMPProbeCheck(t *testing.T) diagnostic.DoctorCheck {
	t.Helper()
	for _, check := range diagnostic.DoctorChecksForPhase(diagnostic.DoctorPhasePreConfig) {
		if check.Name == doctorCheckName {
			return check
		}
	}
	t.Fatalf("doctor check %q is not registered in phase %s", doctorCheckName, diagnostic.DoctorPhasePreConfig)
	return diagnostic.DoctorCheck{}
}

// TestDoctorICMPProbeCheckReportsMissingCapability: both socket kinds
// refused for privilege yields one warning with the dependency code,
// naming CAP_NET_RAW, ping_group_range and the group the daemon runs as.
func TestDoctorICMPProbeCheckReportsMissingCapability(t *testing.T) {
	stubOpeners(t, syscall.EPERM, syscall.EACCES)
	check := registeredICMPProbeCheck(t)
	if !slices.Contains(check.Codes, codeICMPProbeUnavailable) {
		t.Errorf("check declares codes %v, missing %s", check.Codes, codeICMPProbeUnavailable)
	}
	diags := check.Check(diagnostic.DoctorCheckContext{})
	if len(diags) != 1 {
		t.Fatalf("%d diagnostics, want 1: %+v", len(diags), diags)
	}
	d := diags[0]
	if d.Code != codeICMPProbeUnavailable {
		t.Errorf("code %q, want %s", d.Code, codeICMPProbeUnavailable)
	}
	if d.Severity != diagnostic.SeverityWarning {
		t.Errorf("severity %q, want warning", d.Severity)
	}
	for _, want := range []string{"CAP_NET_RAW", "ping_group_range", strconv.Itoa(processGroupID())} {
		if !strings.Contains(d.Message, want) {
			t.Errorf("message %q does not name %s", d.Message, want)
		}
	}
	diagnostic.RegisterBuiltinCodes()
	if diagnostic.Lookup(codeICMPProbeUnavailable) == nil {
		t.Errorf("%s has no metadata in codes.go, so ze explain cannot answer for it", codeICMPProbeUnavailable)
	}
}

// TestDoctorICMPProbeCheckReportsTheFallback: the raw socket refused for
// privilege and the datagram socket open is its own warning, so an
// operator learns the probes run degraded before traceroute refuses.
func TestDoctorICMPProbeCheckReportsTheFallback(t *testing.T) {
	stubOpeners(t, syscall.EPERM, nil)
	check := registeredICMPProbeCheck(t)
	diags := check.Check(diagnostic.DoctorCheckContext{})
	if len(diags) != 1 {
		t.Fatalf("%d diagnostics, want 1: %+v", len(diags), diags)
	}
	if diags[0].Code != codeICMPProbeUnprivileged {
		t.Errorf("code %q, want %s", diags[0].Code, codeICMPProbeUnprivileged)
	}
	diagnostic.RegisterBuiltinCodes()
	if diagnostic.Lookup(codeICMPProbeUnprivileged) == nil {
		t.Errorf("%s has no metadata in codes.go", codeICMPProbeUnprivileged)
	}
}

// TestDoctorICMPProbeCheckSilentWithRawSocket: a raw socket that opens is
// the healthy state and yields nothing.
func TestDoctorICMPProbeCheckSilentWithRawSocket(t *testing.T) {
	stubOpeners(t, nil, nil)
	openRawICMP = func(ctx context.Context, _, _ string, _ func(string, string, syscall.RawConn) error) (net.PacketConn, error) {
		var lc net.ListenConfig
		return lc.ListenPacket(ctx, "udp4", "127.0.0.1:0")
	}
	check := registeredICMPProbeCheck(t)
	if diags := check.Check(diagnostic.DoctorCheckContext{}); len(diags) != 0 {
		t.Errorf("raw socket opens, yet %d diagnostics: %+v", len(diags), diags)
	}
}

// TestDoctorICMPProbeCheckReportsANonPrivilegeRefusal: a raw refusal that
// is not privilege is reported as itself, with the fallback untried, which
// is the guard the check shares with OpenICMP.
func TestDoctorICMPProbeCheckReportsANonPrivilegeRefusal(t *testing.T) {
	fallbackTried := stubOpeners(t, syscall.EAFNOSUPPORT, nil)
	check := registeredICMPProbeCheck(t)
	diags := check.Check(diagnostic.DoctorCheckContext{})
	if len(diags) != 1 {
		t.Fatalf("%d diagnostics, want 1: %+v", len(diags), diags)
	}
	if diags[0].Code != codeICMPProbeUnavailable {
		t.Errorf("code %q, want %s", diags[0].Code, codeICMPProbeUnavailable)
	}
	if !strings.Contains(diags[0].Message, syscall.EAFNOSUPPORT.Error()) {
		t.Errorf("message %q does not carry the raw refusal", diags[0].Message)
	}
	if *fallbackTried {
		t.Error("the datagram opener was reached for a refusal that is not privilege")
	}
}
