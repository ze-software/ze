// Design: docs/architecture/ike/ipsec-14-responder.md -- the authenticator's ticket key
// Detail: docs/architecture/ike/ipsec-11-interop-eap.md -- the peer's ticket cache
// Related: resumption.go -- resumptionFor and recordEAPTLSAuthentication
// RFC: rfc/short/rfc9190.md -- Sections 2.1.2 and 2.1.3
//
// VALIDATES: the peering's EAP-TLS resumption store reaches BOTH roles, one
// store serves every SA of one peer, an authentication edit replaces it, and a
// completed EAP-TLS authentication is counted under the outcome it had.
// PREVENTS: the store being built per SA, which would issue a ticket no later
// authentication could redeem; and a store surviving a change to the
// certificate, the CA or the revocation lists, which would let a resumed session
// be authorized on material the operator has replaced.

package engine

import (
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/core/eap"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// resumptionWiringAuth is an EAP-TLS authentication config naming the PKI entries
// the CRL wiring fixture installs.
func resumptionWiringAuth() ipsec.AuthConfig {
	return ipsec.AuthConfig{
		Mode:              ipsec.AuthEAPTLS,
		Certificate:       "crl-cert",
		CACertificate:     "crl-ca",
		SessionResumption: true,
	}
}

// TestResumptionStoreIsOnePerPeerAndOutlivesTheSA asserts one peer name with one
// authentication config always answers the same store.
//
// It is the property the whole feature rests on. Ze mints a session ticket under
// a key the store holds, and a store rebuilt per SA, per reconnect or per
// operator `clear` would hand the next authentication a key that cannot decrypt
// the ticket the peer is offering.
func TestResumptionStoreIsOnePerPeerAndOutlivesTheSA(t *testing.T) {
	const peerName = "resumption-wiring-peer"
	t.Cleanup(func() { forgetResumption(peerName) })

	first := resumptionFor(peerName, resumptionWiringAuth())
	if first == nil {
		t.Fatal("resumptionFor answered no store")
	}
	if again := resumptionFor(peerName, resumptionWiringAuth()); again != first {
		t.Fatal("a second lookup with the same authentication config built a new store, " +
			"so no ticket ze issues could be redeemed after a reconnect or a clear")
	}
	if !first.Enabled() {
		t.Fatal("the store does not carry the operator's session-resumption setting")
	}
}

// TestResumptionStoreIsReplacedWhenTheAuthenticationChanges asserts an edit to
// the certificate material discards the tickets minted under it.
//
// A resumed handshake presents no certificate: crypto/tls carries the chain in
// the ticket. So an operator who replaces the certificate, the CA or the
// revocation lists expects the next authentication to be decided by the new
// material, and a surviving ticket would carry the old decision past the edit.
func TestResumptionStoreIsReplacedWhenTheAuthenticationChanges(t *testing.T) {
	const peerName = "resumption-rotation-peer"
	t.Cleanup(func() { forgetResumption(peerName) })

	before := resumptionFor(peerName, resumptionWiringAuth())

	edited := resumptionWiringAuth()
	edited.CACertificate = "another-ca"
	after := resumptionFor(peerName, edited)
	if after == before {
		t.Fatal("changing the ca-certificate kept the store, so a ticket issued under the old " +
			"trust anchor could still resume")
	}

	turnedOff := resumptionWiringAuth()
	turnedOff.SessionResumption = false
	off := resumptionFor(peerName, turnedOff)
	if off == after {
		t.Fatal("turning session-resumption off kept the store")
	}
	if off.Enabled() {
		t.Fatal("the rebuilt store still accepts resumption after the leaf was turned off")
	}
}

// TestResumptionStoreIsDroppedForAPeerTheConfigNoLongerNames keeps the map
// bounded by the configuration.
func TestResumptionStoreIsDroppedForAPeerTheConfigNoLongerNames(t *testing.T) {
	const peerName = "resumption-departing-peer"
	held := resumptionFor(peerName, resumptionWiringAuth())
	forgetResumption(peerName)
	if again := resumptionFor(peerName, resumptionWiringAuth()); again == held {
		t.Fatal("forgetResumption left the store in place, so a peer removed from the config " +
			"keeps its ticket keys for the life of the daemon")
	}
	forgetResumption(peerName)
}

// TestEAPTLSConfigsCarryTheResumptionStore is the wiring test: what
// startPeerSession looked up reaches both EAP-TLS roles.
func TestEAPTLSConfigsCarryTheResumptionStore(t *testing.T) {
	ca, caDER, caKey := crlWiringCA(t, "resumption-wiring-ca")
	leaf, leafDER, leafKey := crlWiringLeaf(t, "resumption-wiring-node", 3, ca, caKey)
	loadCRLWiringStore(t, ca, caDER, leaf, leafDER, leafKey, [][]byte{crlWiringList(t, ca, caKey, 4242)})

	store := eap.NewResumption(time.Now, true)
	sa := &SA{
		PeerName:   "branch",
		PeerCfg:    ipsec.SiteToSitePeer{Auth: resumptionWiringAuth()},
		Resumption: store,
	}

	serverCfg, err := eapTLSServerConfig(sa)
	if err != nil {
		t.Fatalf("eapTLSServerConfig: %v", err)
	}
	if serverCfg.Resumption != store {
		t.Fatal("the authenticator config does not carry the peering's resumption store, " +
			"so the RFC 9190 Section 2.1.2 ticket would be minted under a key only this SA can read")
	}

	peerCfg := buildPeerTLSConfig(sa, slogutil.DiscardLogger())
	if peerCfg == nil {
		t.Fatal("buildPeerTLSConfig returned nil for a peer with a certificate and a CA")
	}
	if peerCfg.Resumption != store {
		t.Fatal("the peer config does not carry the peering's resumption store, so no ticket is ever offered")
	}
}

// TestEAPTLSConfigsRefuseAnSAWithNoResumptionStore keeps the wiring failure
// loud. An SA that reached either builder without a store is a programming
// defect, and both roles must say so rather than fall back to a per-exchange key
// or to a silent full handshake for ever.
func TestEAPTLSConfigsRefuseAnSAWithNoResumptionStore(t *testing.T) {
	ca, caDER, caKey := crlWiringCA(t, "resumption-missing-ca")
	leaf, leafDER, leafKey := crlWiringLeaf(t, "resumption-missing-node", 2, ca, caKey)
	loadCRLWiringStore(t, ca, caDER, leaf, leafDER, leafKey, [][]byte{crlWiringList(t, ca, caKey, 4242)})

	sa := &SA{PeerName: "branch", PeerCfg: ipsec.SiteToSitePeer{Auth: resumptionWiringAuth()}}

	if _, err := eapTLSServerConfig(sa); err == nil {
		t.Fatal("eapTLSServerConfig accepted an SA with no resumption store")
	}
	if cfg := buildPeerTLSConfig(sa, slogutil.DiscardLogger()); cfg != nil {
		t.Fatal("buildPeerTLSConfig accepted an SA with no resumption store")
	}
}

// TestEAPTLSAuthenticationIsCountedByOutcome asserts the operator's only view of
// whether resumption is working moves.
//
// The counters are cumulative and shared by every test in this package, so each
// assertion is a RISE rather than an absolute (ai/rules/evidence.md).
func TestEAPTLSAuthenticationIsCountedByOutcome(t *testing.T) {
	const peerName = "resumption-counter-peer"
	sa := &SA{PeerName: peerName, PeerCfg: ipsec.SiteToSitePeer{Auth: resumptionWiringAuth()}}
	log := slogutil.DiscardLogger()

	hits, misses := eapTLSResumptionCounts(peerName)
	recordEAPTLSAuthentication(sa, false, log)
	afterMiss, afterMissM := eapTLSResumptionCounts(peerName)
	if afterMiss != hits || afterMissM != misses+1 {
		t.Fatalf("a full handshake moved the counters to (%d, %d), want (%d, %d)", afterMiss, afterMissM, hits, misses+1)
	}

	recordEAPTLSAuthentication(sa, true, log)
	afterHit, afterHitM := eapTLSResumptionCounts(peerName)
	if afterHit != hits+1 || afterHitM != misses+1 {
		t.Fatalf("a resumption moved the counters to (%d, %d), want (%d, %d)", afterHit, afterHitM, hits+1, misses+1)
	}

	// A peer running another EAP method has no resumption to report, and a
	// resumed=false row for it would read as a resumption that failed.
	other := &SA{PeerName: peerName, PeerCfg: ipsec.SiteToSitePeer{Auth: ipsec.AuthConfig{Mode: ipsec.AuthEAPMSCHAPv2}}}
	recordEAPTLSAuthentication(other, false, log)
	finalH, finalM := eapTLSResumptionCounts(peerName)
	if finalH != hits+1 || finalM != misses+1 {
		t.Fatalf("an EAP-MSCHAPv2 authentication moved the EAP-TLS counters to (%d, %d)", finalH, finalM)
	}

	// The counters are PER PEER, so a peer that authenticated nothing reads two
	// zeros rather than the totals above.
	if otherH, otherM := eapTLSResumptionCounts("resumption-silent-peer"); otherH != 0 || otherM != 0 {
		t.Fatalf("a peer that authenticated nothing reads (%d, %d), want (0, 0)", otherH, otherM)
	}
}
