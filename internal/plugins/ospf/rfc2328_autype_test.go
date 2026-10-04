// Design: docs/architecture/ospf/ospf-12-auth.md -- OSPF packet authentication.
// Related: auth_keystore.go -- authStore.verify, the AuType check.
// Related: rfc2328_receive_test.go -- receiveEngine and the whole-packet receive tests.
//
// VALIDATES: RFC 2328 section 8.2 "The AuType specified in the packet must match the AuType
// specified for the associated area", isolated: the drop is attributed to the AuType
// mismatch itself (the reason label of ze_ospf_auth_failures_total), in both directions --
// a null-AuType packet into a simple-password area and a simple-password packet into a
// null-authentication area.
// PREVENTS: a receiver that skips the AuType comparison and fails later on the password
// only, and a null-authentication area that accepts an authenticated packet.
package ospf

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/plugins/ospf/packet"
	"github.com/ze-software/ze/internal/plugins/ospf/transport"
)

// authReasonRegistry records the reason label of every ze_ospf_auth_failures_total increment.
type authReasonRegistry struct {
	metrics.NopRegistry
	reasons []string
}

func (r *authReasonRegistry) CounterVec(name, help string, labels []string) metrics.CounterVec {
	if name == "ze_ospf_auth_failures_total" {
		return authReasonVec{r}
	}
	return r.NopRegistry.CounterVec(name, help, labels)
}

type authReasonVec struct{ r *authReasonRegistry }

// With records the last label value, the reason, and counts nothing else.
func (v authReasonVec) With(labelValues ...string) metrics.Counter {
	if len(labelValues) > 0 {
		v.r.reasons = append(v.r.reasons, labelValues[len(labelValues)-1])
	}
	return countingCounter{new(int)}
}

func (v authReasonVec) Delete(...string) bool { return false }

// RFC requirement: RFC2328-8.2-3 negative -- a Hello carrying AuType 0 received on a
// simple-password (AuType 1) area is dropped, and the one auth failure it records carries
// reason "autype-mismatch", not a password failure; no neighbor forms.
func TestRFC2328NullAuTypeIntoPasswordAreaDroppedForAuType(t *testing.T) {
	// Goal: the AuType comparison, not the password comparison, refuses the packet. Method:
	// dispatch an unsigned Hello into a simple-password engine and read the failure reason.
	eng, cfg, handle := receiveEngine(t)
	rec := &authReasonRegistry{}
	eng.setMetrics(rec)
	eng.auth.configure(authCfg(keyConfig{KeyID: 1, Algorithm: packet.AuthSimple, Secret: "pw"}))
	peer := ridOf("10.0.0.2")
	before := eng.dispatch.dropped()
	eng.dispatch.dispatch(transport.RawPacket{IfIndex: handle.ifindex, Src: netip.AddrFrom4([4]byte(peer)), Payload: helloFromPeer(peer, cfg.Areas[0].AreaID)})
	if got := eng.dispatch.dropped(); got != before+1 {
		t.Fatalf("dropped = %d, want %d", got, before+1)
	}
	if len(rec.reasons) != 1 || rec.reasons[0] != "autype-mismatch" {
		t.Fatalf("auth failure reasons = %v, want [autype-mismatch]", rec.reasons)
	}
	if rows := eng.neighborSnapshot(); len(rows) != 0 {
		t.Fatalf("neighbor rows = %d, want 0", len(rows))
	}
}

// RFC requirement: RFC2328-8.2-3 negative -- a Hello carrying AuType 1 (simple password)
// received on an area with null authentication is dropped: the packet's AuType does not
// match the area's; no neighbor forms.
func TestRFC2328PasswordAuTypeIntoNullAreaDropped(t *testing.T) {
	// Goal: the reverse mismatch. Method: a second engine configured with a simple password
	// signs the Hello, and the null-authentication engine receives it.
	eng, cfg, handle := receiveEngine(t)
	signer, _, _ := receiveEngine(t)
	signer.auth.configure(authCfg(keyConfig{KeyID: 1, Algorithm: packet.AuthSimple, Secret: "pw"}))
	peer := ridOf("10.0.0.2")
	signed := signer.signPacket("eth0", helloFromPeer(peer, cfg.Areas[0].AreaID))
	if got := packet.AuType(signed[15]); got != packet.AuTypeSimple {
		t.Fatalf("signed AuType = %d, want %d (simple password)", got, packet.AuTypeSimple)
	}
	before := eng.dispatch.dropped()
	eng.dispatch.dispatch(transport.RawPacket{IfIndex: handle.ifindex, Src: netip.AddrFrom4([4]byte(peer)), Payload: signed})
	if got := eng.dispatch.dropped(); got != before+1 {
		t.Fatalf("dropped = %d, want %d (AuType 1 into a null-authentication area)", got, before+1)
	}
	if rows := eng.neighborSnapshot(); len(rows) != 0 {
		t.Fatalf("neighbor rows = %d, want 0", len(rows))
	}
}
