// Design: docs/architecture/ike/ipsec-11-interop-eap.md -- EAP-TLS peer
// RFC: rfc/short/rfc9190.md -- EAP-TLS 1.3, protected success result indication (Section 2.5)
//
// The PEER half of RFC 9190 Section 2.5. The authenticator half is
// tlsMethod.indicateSuccess (eap_tls.go), which writes the encrypted record;
// this file reads it and decides whether the exchange may conclude.
//
// ZE IS STRICTER HERE THAN THE PUBLISHED RFC. Section 2.5 addresses its
// procedure to the server, and no sentence in RFC 9190 obliges a peer to refuse
// an authenticator that skipped it. Errata 7577 proposes exactly that
// obligation and is in state Reported rather than Verified, so it is a proposal
// and not an approved correction. What the published text does say is that the
// keying material "can be made available to lower layers and the authenticator
// after the authenticated success result indication has been sent or received",
// and ze takes that at its word: without the indication there is no protected
// statement that the authenticator succeeded, so the MSK the IKEv2 AUTH payload
// is computed from stays unavailable.
//
// Being stricter costs interoperability where an authenticator is
// non-conformant, so the two shipped EAP-TLS 1.3 scenarios are what keep this
// honest: strongSwan's charon sends the indication and refuses an exchange that
// carries none in EITHER direction (its eap_tls.c get_msk returns FAILED and
// logs "missing protected success indication for EAP-TLS with TLS 1.3").
package eap

import (
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
)

// eapTLSIndicationKept bounds the application data one EAP-TLS peer session
// keeps from an authenticator.
//
// RFC 9190 Section 2.5 gives an EAP-TLS authenticator exactly one octet of
// application data to send, so anything past this bound is already a refusal
// and nothing reads it. The bound exists because the authenticator chooses how
// much it sends: an unbounded accumulator would let it grow this session's
// memory for as long as the exchange lasts.
const eapTLSIndicationKept = 8

// consumePostHandshakeRecords reads the records the authenticator sends after
// its handshake is complete, so crypto/tls processes them.
//
// IT IS WHAT STORES THE RFC 9190 SECTION 2.1.2 TICKET. A NewSessionTicket is a
// post-handshake handshake message, and Go handles one only from inside Read:
// Conn.Read drives readRecord and then handlePostHandshakeMessage
// (crypto/tls/conn.go), which is the call that reaches the ClientSessionCache.
// A peer that stopped reading when HandshakeContext returned received every
// ticket and stored none, so no exchange it ever had could resume.
//
// It also decrypts the Section 2.5 protected success result indication, which
// requireSuccessIndication then judges. The two outcomes are kept apart:
// indication holds what arrived, indicationErr holds a read that failed, and a
// clean end of stream writes neither, because an authenticator that sent
// nothing and a record this peer could not decrypt are different answers
// (ai/rules/principles.md).
//
// It returns when the transport closes, which Close does: eapTLSTransport.Read
// answers io.EOF once closed, and that is the expected end rather than a
// failure.
func (ps *PeerSession) consumePostHandshakeRecords() {
	buf := make([]byte, eapTLSIndicationKept)
	for {
		n, err := ps.tlsConn.Read(buf)
		if n > 0 {
			ps.recordIndication(buf[:n])
		}
		if err == nil {
			continue
		}
		if !errors.Is(err, io.EOF) {
			ps.indicationErr.Store(&err)
		}
		return
	}
}

// recordIndication adds the application data one post-handshake read produced
// to what this session has already seen, and publishes the total.
//
// It accumulates rather than replacing, because the judgement below is about
// the WHOLE of the application data the authenticator sent: two records of one
// octet each, and one record of two octets, are the same violation, and a field
// that held only the last read would report the second as conformant. The
// published value is a fresh slice on every call, because the session's own
// goroutine reads the pointer while this runs on the TLS reader goroutine.
//
// Called only from consumePostHandshakeRecords, which is the single writer.
func (ps *PeerSession) recordIndication(data []byte) {
	var seen []byte
	if held := ps.indication.Load(); held != nil {
		seen = *held
	}
	room := eapTLSIndicationKept - len(seen)
	if room < 1 {
		return
	}
	if len(data) > room {
		data = data[:room]
	}
	total := make([]byte, 0, len(seen)+len(data))
	total = append(total, seen...)
	total = append(total, data...)
	ps.indication.Store(&total)
}

// requireSuccessIndication answers whether this exchange may conclude, reading
// the RFC 9190 Section 2.5 protected success result indication the
// authenticator was meant to send.
//
// THE VERSION TEST READS THE NEGOTIATED VERSION, NOT THE CONFIGURED ONE, for
// the same reason tlsMethod.indicateSuccess does on the other role: Section 2.5
// is new against RFC 5216 and "only applies to TLS 1.3". A TLS 1.2 exchange is
// governed by RFC 5216, which defines no indication, so requiring one there
// would refuse every conformant TLS 1.2 authenticator. tlsCfg carries no
// version at all and MinVersion is TLS 1.2 on every session including the TLS
// 1.3 ones, so neither can answer this.
//
// It answers nil for a method other than EAP-TLS. RFC 9190 governs EAP-TLS, and
// a password method's Success is judged by that method's own rules.
func (ps *PeerSession) requireSuccessIndication() error {
	if ps.method != TypeTLS {
		return nil
	}
	if ps.tlsConn == nil {
		return errors.New("eap-tls: no TLS connection to read the RFC 9190 Section 2.5 protected success result indication from")
	}
	cs := ps.tlsConn.ConnectionState()
	if cs.Version < tls.VersionTLS13 {
		return nil
	}

	// A read that FAILED is reported as itself. It is not an authenticator that
	// sent nothing: the record arrived and this peer could not read it, which
	// names a different repair.
	if err := ps.indicationErr.Load(); err != nil {
		return fmt.Errorf(
			"eap-tls: cannot read what the authenticator %s sent after its %s handshake, so the RFC 9190 Section 2.5 protected success result indication cannot be checked: %w",
			eapTLSPeerName(cs), tls.VersionName(cs.Version), *err)
	}

	seen := ps.indication.Load()
	if seen == nil {
		return fmt.Errorf(
			"eap-tls: the authenticator %s ended an %s exchange without the RFC 9190 Section 2.5 protected success result indication. "+
				"ze requires the encrypted TLS record carrying application data 0x00 before it accepts the EAP-Success. "+
				"Upgrade the authenticator to an implementation that follows RFC 9190 Section 2.5, or move the peering to TLS 1.2, where RFC 5216 governs and no indication is sent",
			eapTLSPeerName(cs), tls.VersionName(cs.Version))
	}
	if !bytes.Equal(*seen, eapTLSSuccessIndication) {
		// "starting" rather than "of", because eapTLSIndicationKept caps what
		// this session kept: a longer payload is reported by its first octets.
		return fmt.Errorf(
			"eap-tls: the authenticator %s ended an %s exchange with application data starting % x, "+
				"want the single octet 00 that RFC 9190 Section 2.5 defines as the protected success result indication",
			eapTLSPeerName(cs), tls.VersionName(cs.Version), *seen)
	}
	return nil
}
