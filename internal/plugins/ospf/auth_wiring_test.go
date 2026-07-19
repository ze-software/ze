// VALIDATES: spec-ospf-12 -- the engine sign hook rewrites a default-AuType-0 encoded
// packet to the configured scheme (AuType + checksum + auth field + digest) so it
// round-trips through verify; AuType 1 keeps a valid checksum, crypto keeps it zero.
// PREVENTS: regressions where the signer leaves the wrong AuType/checksum, or a signed
// packet fails its own verify.
package ospf

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"codeberg.org/thomas-mangin/ze/internal/core/metrics"
	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/packet"
	"codeberg.org/thomas-mangin/ze/internal/plugins/ospf/transport"
)

// authFailRegistry counts ze_ospf_auth_failures_total increments so a test can assert the
// drop path bumps the metric. Every other series falls through to the nop backend.
type authFailRegistry struct {
	metrics.NopRegistry
	authFailures int
}

func (r *authFailRegistry) CounterVec(name, help string, labels []string) metrics.CounterVec {
	if name == "ze_ospf_auth_failures_total" {
		return countingCounterVec{&r.authFailures}
	}
	return r.NopRegistry.CounterVec(name, help, labels)
}

type countingCounterVec struct{ n *int }

func (v countingCounterVec) With(...string) metrics.Counter { return countingCounter(v) }
func (v countingCounterVec) Delete(...string) bool          { return false }

type countingCounter struct{ n *int }

func (c countingCounter) Inc()        { *c.n++ }
func (c countingCounter) Add(float64) { *c.n++ }

func encodeHello(t *testing.T) []byte {
	t.Helper()
	p := packet.Packet{Header: packet.Header{Type: packet.PacketTypeHello}, Hello: &packet.Hello{NetworkMask: [4]byte{255, 255, 255, 0}, HelloInterval: 10, DeadInterval: 40}}
	buf := make([]byte, p.EncodedLen())
	n := p.WriteTo(buf, 0)
	return buf[:n]
}

func TestEngineSignPacketCrypto(t *testing.T) {
	eng, _ := newRedistEngine(t, `{"ospf":{"router-id":"10.0.0.1"}}`)
	eng.auth.configure(authCfg(keyConfig{KeyID: 1, Algorithm: "hmac-sha-256", Secret: "k3y"}))

	signed := eng.signPacket("eth0", encodeHello(t))
	// RFC requirement: RFC5709-3.1-1 positive -- the engine sign hook rewrites an HMAC-SHA-authenticated packet's AuType to 2 (Cryptographic Authentication) on the wire (signPacket -> Sign; header AuType stamp header.go:190), asserted from the on-wire low octet here.
	assert.Equal(t, packet.AuTypeCryptographic, packet.AuType(uint16(signed[14])<<8|uint16(signed[15])), "AuType rewritten to 2")
	assert.Zero(t, uint16(signed[12])<<8|uint16(signed[13]), "crypto keeps the checksum zero (trap #10)")

	// AuType 2 (HMAC-SHA) ignores the source address in the digest, so any src verifies.
	reason, ok := eng.auth.verify("eth0", ridOf("2.2.2.2"), [4]byte{}, signed)
	assert.True(t, ok, "the engine-signed packet round-trips through verify")
	assert.Empty(t, reason)
}

func TestEngineSignPacketSimple(t *testing.T) {
	eng, _ := newRedistEngine(t, `{"ospf":{"router-id":"10.0.0.1"}}`)
	eng.auth.configure(authCfg(keyConfig{KeyID: 1, Algorithm: "simple", Secret: "password"}))

	signed := eng.signPacket("eth0", encodeHello(t))
	assert.Equal(t, packet.AuTypeSimple, packet.AuType(uint16(signed[14])<<8|uint16(signed[15])))
	require.True(t, packet.VerifyPacketChecksum(signed), "AuType 1 keeps a valid checksum")
	reason, ok := eng.auth.verify("eth0", ridOf("2.2.2.2"), [4]byte{}, signed)
	assert.True(t, ok)
	assert.Empty(t, reason)
}

func TestEngineSignPacketNoAuth(t *testing.T) {
	eng, _ := newRedistEngine(t, `{"ospf":{"router-id":"10.0.0.1"}}`)
	// no auth configured -> the payload is returned unchanged.
	pkt := encodeHello(t)
	signed := eng.signPacket("eth0", pkt)
	assert.Equal(t, pkt, signed, "no auth chain -> packet sent byte-for-byte")
}

// TestEngineVerifyPacketDropsAndCounts exercises the RX auth chokepoint (the dispatcher
// authOK hook): on an authenticated interface an unauthenticated packet is dropped
// (verifyPacket returns false) AND ze_ospf_auth_failures_total{interface,reason} is bumped,
// while a correctly-signed packet passes and does not bump the counter. This is the engine
// glue (ifindex->interface resolution, src extraction, metric increment, drop) that the
// keystore-level verify tests do not cover; a failed interface lookup would fail-open
// (return true), so the drop assertion also proves the ifindex resolved.
func TestEngineVerifyPacketDropsAndCounts(t *testing.T) {
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth0":{"area":"0","network-type":"point-to-point"}}}}}`), nil)
	require.NoError(t, err)
	fb := &fakeBackend{}
	eng := newEngine(transport.New(fb))
	rec := &authFailRegistry{}
	eng.setMetrics(rec)
	eng.setConfig(cfg)
	require.NoError(t, eng.openInterfaces())
	defer eng.shutdown()
	// Require cryptographic auth on eth0 (inherits the backbone key chain).
	eng.auth.configure(authCfg(keyConfig{KeyID: 1, Algorithm: "hmac-sha-256", Secret: "k3y"}))

	fb.mu.Lock()
	handle := fb.handles["eth0"]
	fb.mu.Unlock()
	require.NotNil(t, handle, "eth0 transport handle present")
	h := Header{RouterID: ridOf("2.2.2.2"), AreaID: cfg.Areas[0].AreaID}

	// Unauthenticated packet on an authenticated interface -> dropped + counted.
	if eng.verifyPacket(transport.RawPacket{IfIndex: handle.ifindex, Payload: encodeHello(t)}, h) {
		t.Fatal("verifyPacket accepted an unauthenticated packet on an authenticated interface (fail-open)")
	}
	if rec.authFailures != 1 {
		t.Fatalf("ze_ospf_auth_failures_total increments = %d, want 1", rec.authFailures)
	}

	// A correctly-signed packet passes verification and does not bump the failure counter.
	if !eng.verifyPacket(transport.RawPacket{IfIndex: handle.ifindex, Payload: eng.signPacket("eth0", encodeHello(t))}, h) {
		t.Fatal("verifyPacket rejected a correctly-signed packet")
	}
	if rec.authFailures != 1 {
		t.Fatalf("a passing packet must not bump auth failures: got %d", rec.authFailures)
	}
}
