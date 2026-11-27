// Design: docs/architecture/doctor-and-health-checks.md -- post-config readiness checks
// RFC: rfc/short/rfc5216.md -- Section 5.3, peer and server identity authorization
//
// VALIDATES: checkIPsecEAPTLSIdentity reports doctor-ipsec-eap-tls-identity
// with a concrete set fix for unconstrained site-to-site eap-tls peers and
// answers healthy when every eap-tls peer carries a valid identity constraint.
// PREVENTS: a daemon started from an unvalidated legacy config file failing
// closed silently without ze doctor identifying the missing identity policy.

package engine

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

func eapTLSDoctorPeerTree(peerName, connType, remoteAddr, serverID string, allowed []string) *config.Tree {
	auth := config.NewTree()
	auth.Set("mode", "eap-tls")
	auth.Set("certificate", "node-cert")
	auth.Set("ca-certificate", "corp-ca")
	if serverID != "" {
		auth.Set("server-identity", serverID)
	}
	if len(allowed) > 0 {
		auth.SetSlice("allowed-identities", allowed)
	}

	peer := config.NewTree()
	peer.Set("connection-type", connType)
	if remoteAddr != "" {
		peer.Set("remote-address", remoteAddr)
	}
	peer.SetContainer("authentication", auth)

	sts := config.NewTree()
	sts.AddListEntry("peer", peerName, peer)

	ipsecRoot := config.NewTree()
	ipsecRoot.SetContainer("site-to-site", sts)

	vpnRoot := config.NewTree()
	vpnRoot.SetContainer("ipsec", ipsecRoot)

	root := config.NewTree()
	root.SetContainer("vpn", vpnRoot)
	return root
}

// TestDoctorIPsecEAPTLSIdentityCheck verifies that checkIPsecEAPTLSIdentity
// flags unconstrained eap-tls peers with doctor-ipsec-eap-tls-identity and a
// concrete set fix command, and returns no findings for constrained peers.
func TestDoctorIPsecEAPTLSIdentityCheck(t *testing.T) {
	t.Run("unconstrained respond peer is reported", func(t *testing.T) {
		tree := eapTLSDoctorPeerTree("branch", "respond", "192.0.2.2", "", nil)
		diags := checkIPsecEAPTLSIdentity(registry.DoctorCheckContext{Tree: tree})
		if len(diags) != 1 {
			t.Fatalf("got %d diagnostics, want 1", len(diags))
		}
		d := diags[0]
		if d.Code != codeDoctorIPsecEAPTLSIdentity {
			t.Errorf("code = %q, want %q", d.Code, codeDoctorIPsecEAPTLSIdentity)
		}
		if d.Severity != severityError {
			t.Errorf("severity = %q, want %q", d.Severity, severityError)
		}
		if !strings.Contains(d.Fix, "set vpn ipsec site-to-site peer branch authentication allowed-identities") {
			t.Errorf("fix %q does not give the allowed-identities set command", d.Fix)
		}
	})

	t.Run("unconstrained initiate peer targeting IP is reported", func(t *testing.T) {
		tree := eapTLSDoctorPeerTree("hq", "initiate", "192.0.2.1", "", nil)
		diags := checkIPsecEAPTLSIdentity(registry.DoctorCheckContext{Tree: tree})
		if len(diags) != 1 {
			t.Fatalf("got %d diagnostics, want 1", len(diags))
		}
		d := diags[0]
		if d.Code != codeDoctorIPsecEAPTLSIdentity {
			t.Errorf("code = %q, want %q", d.Code, codeDoctorIPsecEAPTLSIdentity)
		}
		if !strings.Contains(d.Fix, "set vpn ipsec site-to-site peer hq authentication server-identity") {
			t.Errorf("fix %q does not give the server-identity set command", d.Fix)
		}
	})

	t.Run("constrained peers produce no findings", func(t *testing.T) {
		respTree := eapTLSDoctorPeerTree("branch", "respond", "192.0.2.2", "", []string{"client.example.com"})
		if got := checkIPsecEAPTLSIdentity(registry.DoctorCheckContext{Tree: respTree}); len(got) != 0 {
			t.Errorf("constrained respond peer produced %d diagnostics: %+v", len(got), got)
		}
		initTree := eapTLSDoctorPeerTree("hq", "initiate", "192.0.2.1", "vpn.example.com", nil)
		if got := checkIPsecEAPTLSIdentity(registry.DoctorCheckContext{Tree: initTree}); len(got) != 0 {
			t.Errorf("constrained initiate peer produced %d diagnostics: %+v", len(got), got)
		}
		fqdnTree := eapTLSDoctorPeerTree("hq-fqdn", "initiate", "vpn.example.com", "", nil)
		if got := checkIPsecEAPTLSIdentity(registry.DoctorCheckContext{Tree: fqdnTree}); len(got) != 0 {
			t.Errorf("FQDN-constrained initiate peer produced %d diagnostics: %+v", len(got), got)
		}
	})

	t.Run("malformed allowed-identities entry is reported", func(t *testing.T) {
		tree := eapTLSDoctorPeerTree("branch", "respond", "192.0.2.2", "", []string{"*.com"})
		diags := checkIPsecEAPTLSIdentity(registry.DoctorCheckContext{Tree: tree})
		if len(diags) != 1 {
			t.Fatalf("got %d diagnostics, want 1", len(diags))
		}
		if diags[0].Code != codeDoctorIPsecEAPTLSIdentity {
			t.Errorf("code = %q, want %q", diags[0].Code, codeDoctorIPsecEAPTLSIdentity)
		}
		if diags[0].Severity != severityError {
			t.Errorf("severity = %q, want %q", diags[0].Severity, severityError)
		}
		if diags[0].Fix == "" {
			t.Error("expected non-empty Fix for malformed allowed-identities")
		}
	})

	t.Run("missing parsed tree warns rather than claiming healthy", func(t *testing.T) {
		diags := checkIPsecEAPTLSIdentity(registry.DoctorCheckContext{Tree: nil})
		if len(diags) != 1 {
			t.Fatalf("got %d diagnostics, want 1 warning", len(diags))
		}
		if diags[0].Code != codeDoctorIPsecEAPTLSIdentity {
			t.Errorf("code = %q, want %q", diags[0].Code, codeDoctorIPsecEAPTLSIdentity)
		}
		if diags[0].Severity != severityWarning {
			t.Errorf("severity = %q, want %q", diags[0].Severity, severityWarning)
		}
		if diags[0].Fix == "" {
			t.Error("expected non-empty Fix for missing parsed tree warning")
		}
	})
}
