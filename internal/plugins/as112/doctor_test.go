package as112

import (
	"testing"
	"time"
)

// VALIDATES: AC-3 -- no cert diagnostic when neither DoT nor DoH is enabled.
func TestAS112TLSDiagnostic_DisabledSecure(t *testing.T) {
	if d := as112TLSDiagnostic(false, false, "/x.pem", "/x.key", time.Now()); len(d) != 0 {
		t.Fatalf("diagnostics = %v, want none (no secure listener)", d)
	}
}

// VALIDATES: AC-3 -- with DoT enabled and a missing cert file, the shared
// cert check surfaces doctor-tls-missing.
func TestAS112TLSDiagnostic_MissingCert(t *testing.T) {
	d := as112TLSDiagnostic(true, false, "/does/not/exist.pem", "/does/not/exist.key", time.Now())
	if len(d) != 1 || d[0].Code != "doctor-tls-missing" {
		t.Fatalf("diagnostics = %v, want one doctor-tls-missing", d)
	}
}

// VALIDATES: AC-3 -- self-signed fallback (no cert files) with DoH enabled is
// not a diagnostic.
func TestAS112TLSDiagnostic_SelfSigned(t *testing.T) {
	if d := as112TLSDiagnostic(false, true, "", "", time.Now()); len(d) != 0 {
		t.Fatalf("diagnostics = %v, want none (self-signed fallback)", d)
	}
}

// VALIDATES: AC-7 -- as112 enabled on a privileged port the process cannot
// bind produces the doctor-as112-port-unavailable diagnostic.
func TestAS112ListenDiagnostic_UnbindablePrivilegedPort(t *testing.T) {
	diags := as112ListenDiagnostic(true, addressFamilyBoth, func(string, int) bool { return false })
	if len(diags) != 1 || diags[0].Code != "doctor-as112-port-unavailable" {
		t.Fatalf("diagnostics = %v, want exactly one doctor-as112-port-unavailable", diags)
	}
}

// VALIDATES: no diagnostic when the port IS bindable.
func TestAS112ListenDiagnostic_Bindable(t *testing.T) {
	diags := as112ListenDiagnostic(true, addressFamilyBoth, func(string, int) bool { return true })
	if len(diags) != 0 {
		t.Fatalf("diagnostics = %v, want none (port bindable)", diags)
	}
}

// VALIDATES: no diagnostic when as112 is disabled, regardless of bindability.
func TestAS112ListenDiagnostic_Disabled(t *testing.T) {
	diags := as112ListenDiagnostic(false, addressFamilyBoth, func(string, int) bool { return false })
	if len(diags) != 0 {
		t.Fatalf("diagnostics = %v, want none (as112 disabled)", diags)
	}
}

// VALIDATES: an ipv6-only node probes the IPv6 wildcard, never IPv4 -- a v4
// bind failure (or success) must not influence a v6-only node's diagnostic.
func TestAS112ListenDiagnostic_IPv6OnlyProbesIPv6Only(t *testing.T) {
	var probed []string
	as112ListenDiagnostic(true, addressFamilyIPv6Only, func(host string, _ int) bool {
		probed = append(probed, host)
		return true
	})
	if len(probed) != 1 || probed[0] != "::" {
		t.Fatalf("probed hosts = %v, want exactly [\"::\"]", probed)
	}
}

// VALIDATES: an ipv4-only node probes the IPv4 wildcard, never IPv6.
func TestAS112ListenDiagnostic_IPv4OnlyProbesIPv4Only(t *testing.T) {
	var probed []string
	as112ListenDiagnostic(true, addressFamilyIPv4Only, func(host string, _ int) bool {
		probed = append(probed, host)
		return true
	})
	if len(probed) != 1 || probed[0] != "0.0.0.0" {
		t.Fatalf("probed hosts = %v, want exactly [\"0.0.0.0\"]", probed)
	}
}

// VALIDATES: "both" probes both wildcards, and an IPv6-only bind failure
// (IPv4 bindable) still produces the diagnostic -- neither family alone
// gives false confidence for a dual-stack node.
func TestAS112ListenDiagnostic_BothProbesBothFamilies(t *testing.T) {
	var probed []string
	diags := as112ListenDiagnostic(true, addressFamilyBoth, func(host string, _ int) bool {
		probed = append(probed, host)
		return host != "::" // IPv6 unbindable, IPv4 bindable.
	})
	if len(probed) != 2 || probed[0] != "0.0.0.0" || probed[1] != "::" {
		t.Fatalf("probed hosts = %v, want [\"0.0.0.0\" \"::\"]", probed)
	}
	if len(diags) != 1 || diags[0].Code != "doctor-as112-port-unavailable" {
		t.Fatalf("diagnostics = %v, want exactly one doctor-as112-port-unavailable (IPv6 unbindable)", diags)
	}
}
