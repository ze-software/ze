// Design: docs/contributing/patching-the-standard-library.md -- the TLS 1.2 ServerHello session_id
//
// VALIDATES: AC-2, a TLS 1.2 full handshake with ze's authenticator answers a
// ServerHello whose session_id is 32 octets the server chose, never the
// client's, and issues no session ticket: whether the client offers the
// ticket extension, offers none, or sends legacy_version 0x0304 with no
// supported_versions extension.
// PREVENTS: the empty ServerHello session_id unpatched crypto/tls sends, and
// a TLS 1.2 ticket issued to a client the ticket suppression misclassifies.

package eap

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/binary"
	"io"
	"net"
	"slices"
	"testing"
	"time"
)

// tlsExtSessionTicket is the SessionTicket extension (RFC 5077 Section 3.2).
const tlsExtSessionTicket uint16 = 35

// serverHelloSession answers the session_id of a ServerHello body and the
// extension types it carries (RFC 5246 Section 7.4.1.3).
func serverHelloSession(t *testing.T, body []byte) (sessionID []byte, extensions []uint16) {
	t.Helper()
	if len(body) < 35 || 35+int(body[34]) > len(body) {
		t.Fatalf("a ServerHello body of %d octets", len(body))
	}
	sessionID = body[35 : 35+int(body[34])]
	at := 35 + len(sessionID) + 3 // cipher_suite, compression_method
	if at+2 > len(body) {
		return sessionID, nil
	}
	end := at + 2 + int(binary.BigEndian.Uint16(body[at:]))
	for off := at + 2; off+4 <= end && off+4 <= len(body); {
		extensions = append(extensions, binary.BigEndian.Uint16(body[off:]))
		off += 4 + int(binary.BigEndian.Uint16(body[off+2:]))
	}
	return sessionID, extensions
}

// clientHelloSession answers the session_id of a ClientHello record.
func clientHelloSession(record []byte) []byte {
	at := tlsRecordHeaderLen + 4 + 2 + tlsClientHelloRandomOctets
	return record[at+1 : at+1+int(record[at])]
}

// firstServerFlight replays one ClientHello record at a fresh authenticator
// and answers the handshake messages of the flight it writes back, up to
// ServerHelloDone. The client cannot finish a handshake whose ClientHello
// was rewritten, so the flight is what the test reads.
func firstServerFlight(t *testing.T, pki *eapTLSPKI, record []byte) tlsFlight {
	t.Helper()
	sess, err := NewSession(TypeTLS, pki.serverConfig())
	if err != nil {
		t.Fatalf("create the authenticator session: %v", err)
	}
	t.Cleanup(sess.Close)
	method, ok := sess.method.(*tlsMethod)
	if !ok {
		t.Fatalf("authenticator method is %T, want *tlsMethod", sess.method)
	}
	clientSide, serverSide := net.Pipe()
	t.Cleanup(func() {
		clientSide.Close() //nolint:errcheck // torn down with the test
	})
	if err := clientSide.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		t.Fatal(err)
	}
	go func() {
		conn := tls.Server(serverSide, method.tlsConfig)
		conn.HandshakeContext(context.Background()) //nolint:errcheck // the assertion is the flight the authenticator wrote
		conn.Close()                                //nolint:errcheck // torn down with the test
	}()
	written := make(chan error, 1)
	go func() {
		_, err := clientSide.Write(record)
		written <- err
	}()

	var flight []byte
	for {
		header := make([]byte, tlsRecordHeaderLen)
		if _, err := io.ReadFull(clientSide, header); err != nil {
			t.Fatalf("reading the authenticator's flight: %v", err)
		}
		payload := make([]byte, binary.BigEndian.Uint16(header[3:5]))
		if _, err := io.ReadFull(clientSide, payload); err != nil {
			t.Fatalf("reading the authenticator's flight: %v", err)
		}
		flight = append(append(flight, header...), payload...)
		parsed := parseTLSFlight(t, flight)
		if header[0] != tlsContentHandshake {
			t.Fatalf("the authenticator answered content type %#02x, want its handshake flight", header[0])
		}
		if slices.Contains(parsed.types, tlsHSServerHelloDone) {
			if err := <-written; err != nil {
				t.Fatalf("writing the ClientHello: %v", err)
			}
			return parsed
		}
	}
}

// requireChosenSessionID fails unless the ServerHello carries a 32-octet
// session_id other than the client's and no SessionTicket extension, the
// extension a TLS 1.2 server answers with when it will send a ticket.
func requireChosenSessionID(t *testing.T, flight tlsFlight, client []byte) {
	t.Helper()
	sessionID, extensions := serverHelloSession(t, flight.bodies[tlsHSServerHello])
	if len(sessionID) != 32 {
		t.Errorf("the ServerHello session_id is %d octets, want the 32 the server chose", len(sessionID))
	}
	if len(client) > 0 && bytes.Equal(sessionID, client) {
		t.Error("the ServerHello echoes the client's session_id")
	}
	if slices.Contains(extensions, tlsExtSessionTicket) {
		t.Error("the ServerHello carries the SessionTicket extension, so a ticket follows")
	}
	if slices.Contains(flight.types, tlsHSNewSessionTicket) {
		t.Error("the server flight carries a NewSessionTicket")
	}
}

// tls12ClientHello answers a TLS 1.2 ClientHello record: ze's peer's first
// ClientHello with the three TLS 1.3 extensions removed (supported_versions,
// key_share, psk_key_exchange_modes). ze's peer always offers TLS 1.3, so
// without supported_versions the record is the one a TLS 1.2-only client
// sends: legacy_version 0x0303, the SessionTicket extension, an empty
// session_id. A fresh authenticator at its production MaxVersion answers it
// with TLS 1.2.
func tls12ClientHello(t *testing.T, pki *eapTLSPKI) []byte {
	t.Helper()
	peer := NewPeerSessionTLS("eap-tls-client", pki.resumptionPeerConfig(NewResumption(time.Now, true)))
	fl := driveEAPTLSFlight(t, pki.serverConfig(), peer, tls.VersionTLS12, eapTLS13Rounds)
	requireCompleted(t, fl, "a TLS 1.2 exchange")
	record := parseClientHello(t, fl).record
	for _, drop := range []uint16{tlsExtSupportedVersions, tlsExtKeyShare, tlsExtPSKKeyExchangeModes} {
		record = withoutExtension(t, record, drop)
	}
	extensions := clientHelloExtensions(t, record)
	_, ticket := extensions[tlsExtSessionTicket]
	_, versions := extensions[tlsExtSupportedVersions]
	if !ticket || versions {
		t.Fatalf("the TLS 1.2 ClientHello carries SessionTicket %v and supported_versions %v, want SessionTicket and no supported_versions", ticket, versions)
	}
	return record
}

// TestRFC5216TLS12FullHandshakeServerHelloSessionID reads the ServerHello of
// TLS 1.2 handshakes with ze's authenticator.
//
// RFC requirement: RFC5216-2.1.1-4 positive -- on a TLS 1.2 full handshake
// whose ClientHello carries a session_id the authenticator does not
// recognize, the authenticator's ServerHello carries a 32-octet session_id,
// with or without the client's SessionTicket extension and for a
// legacy_version 0x0304 ClientHello with no supported_versions extension, and
// the server sends no session ticket.
//
// RFC requirement: RFC5216-2.1.1-4 negative -- offered a non-empty session_id
// it does not recognize, the authenticator does not echo it: the ServerHello's
// session_id differs from the client's.
func TestRFC5216TLS12FullHandshakeServerHelloSessionID(t *testing.T) {
	pki := newEAPTLSPKI(t)

	t.Run("client-offers-an-unrecognized-session-id", func(t *testing.T) {
		record := tls12ClientHello(t, pki)
		client := clientHelloSession(record)
		if len(client) == 0 {
			t.Fatal("the ClientHello offers an empty session_id, so an echo cannot be told from a choice")
		}
		requireChosenSessionID(t, firstServerFlight(t, pki, record), client)
	})

	t.Run("client-offers-the-ticket-extension", func(t *testing.T) {
		_, _, server, peer := tls12Exchange(t, pki, NewResumption(time.Now, true), NewResumption(time.Now, true), nil)
		if len(server) < 1 || len(peer) < 1 {
			t.Fatalf("the exchange carried %d server and %d peer messages", len(server), len(peer))
		}
		hello := parseTLSFlight(t, peer[0])
		if len(clientHelloTicket(t, hello.bodies[tlsHSClientHello])) != 0 {
			t.Fatal("the first ClientHello offers a ticket, so the handshake is not the full one under test")
		}
		requireChosenSessionID(t, parseTLSFlight(t, server[0]), nil)
		for i, message := range server {
			if slices.Contains(parseTLSFlight(t, message).types, tlsHSNewSessionTicket) {
				t.Errorf("server message %d carries a NewSessionTicket", i)
			}
		}
	})

	t.Run("client-offers-no-ticket-extension", func(t *testing.T) {
		record := withoutExtension(t, tls12ClientHello(t, pki), tlsExtSessionTicket)
		requireChosenSessionID(t, firstServerFlight(t, pki, record), clientHelloSession(record))
	})

	t.Run("legacy-version-0x0304-without-supported-versions", func(t *testing.T) {
		record := tls12ClientHello(t, pki)
		record[tlsRecordHeaderLen+4], record[tlsRecordHeaderLen+5] = 0x03, 0x04
		flight := firstServerFlight(t, pki, record)
		if version := binary.BigEndian.Uint16(flight.bodies[tlsHSServerHello]); version != tls.VersionTLS12 {
			t.Fatalf("the ServerHello negotiates %#04x, want TLS 1.2", version)
		}
		requireChosenSessionID(t, flight, clientHelloSession(record))
	})
}
