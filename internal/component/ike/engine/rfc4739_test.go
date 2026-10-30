package engine

import (
	"bytes"
	"encoding/binary"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/component/ike/wire"
)

// rfc4739Auth is a pre-shared-secret peer with a second round each way. The round-2
// credentials differ from round 1 so a test can tell which round a reader took.
func rfc4739Auth() ipsec.AuthConfig {
	return ipsec.AuthConfig{
		Mode: ipsec.AuthPreSharedSecret,
		PSK:  "round-one-secret",
		Additions: &ipsec.AuthAdditions{SecondRound: &ipsec.SecondRoundAuth{
			Local: &ipsec.AuthConfig{
				Mode: ipsec.AuthPreSharedSecret, PSK: "round-two-local", LocalID: "ze-round-two",
			},
			Remote: &ipsec.AuthConfig{
				Mode: ipsec.AuthPreSharedSecret, PSK: "round-two-remote", RemoteID: "peer-round-two",
			},
		}},
	}
}

// rfc4739PlainAuth is the same peer with no second round.
func rfc4739PlainAuth() ipsec.AuthConfig {
	return ipsec.AuthConfig{Mode: ipsec.AuthPreSharedSecret, PSK: "round-one-secret"}
}

// rfc4739Notifies returns the Notify payloads of one type among parsed payloads.
func rfc4739Notifies(payloads []wire.PayloadEntry, msgType uint16) []*wire.PayloadNotify {
	var found []*wire.PayloadNotify
	for i := range payloads {
		n, ok := payloads[i].Payload.(*wire.PayloadNotify)
		if ok && n.NotifyMsgType == msgType {
			found = append(found, n)
		}
	}
	return found
}

// rfc4739RawNotifies walks the cleartext payload chain of an unencrypted IKE message
// and returns the raw octets of every Notify payload of one type, generic header
// included. It reads the wire, not the parser, so the octets are what a peer receives.
func rfc4739RawNotifies(t *testing.T, raw []byte, msgType uint16) [][]byte {
	t.Helper()
	return rfc4739ChainNotifies(t, raw, raw[16], 28, msgType)
}

// rfc4739ChainNotifies walks a payload chain that starts at offset off of raw with
// payload type next, and returns the raw octets of every Notify payload of one type,
// generic header included.
func rfc4739ChainNotifies(t *testing.T, raw []byte, next uint8, off int, msgType uint16) [][]byte {
	t.Helper()
	var found [][]byte
	for next != 0 {
		if off+4 > len(raw) {
			t.Fatalf("payload header at offset %d runs past the %d-octet message", off, len(raw))
		}
		length := int(binary.BigEndian.Uint16(raw[off+2:]))
		if length < 4 || off+length > len(raw) {
			t.Fatalf("payload at offset %d claims %d octets in a %d-octet message", off, length, len(raw))
		}
		if next == wire.PayloadTypeNotify && length >= 8 && binary.BigEndian.Uint16(raw[off+6:]) == msgType {
			found = append(found, raw[off:off+length])
		}
		next = raw[off]
		off += length
	}
	return found
}

// rfc4739CheckEmptyNotify fails unless the parsed notification carries Protocol ID 0,
// SPI Size 0, no SPI octets and no Notification Data.
func rfc4739CheckEmptyNotify(t *testing.T, name string, n *wire.PayloadNotify) {
	t.Helper()
	if n.ProtocolID != 0 {
		t.Errorf("%s Protocol ID = %d, want 0", name, n.ProtocolID)
	}
	if n.SPISize != 0 {
		t.Errorf("%s SPI Size = %d, want 0", name, n.SPISize)
	}
	if len(n.SPI) != 0 {
		t.Errorf("%s carries %d SPI octets, want none", name, len(n.SPI))
	}
	if len(n.NotificationData) != 0 {
		t.Errorf("%s carries %d data octets, want none", name, len(n.NotificationData))
	}
}

// rfc4739AuthRequest decrypts the initiator's first IKE_AUTH request the way the
// responder reads it.
func rfc4739AuthRequest(t *testing.T, ini, resp *SA) []wire.PayloadEntry {
	t.Helper()
	inner, err := decryptAndParse(resp, parseMsg(t, ini.LastSentMsg), ini.LastSentMsg)
	if err != nil {
		t.Fatalf("decrypt the IKE_AUTH request: %v", err)
	}
	return inner
}

// TestRFC4739ResponderAnnouncesMultipleAuth proves AC-1. Goal: a Ze responder for a
// peer with a second round puts one MULTIPLE_AUTH_SUPPORTED in its IKE_SA_INIT
// response, with every field the RFC fixes at zero. Method: run the real IKE_SA_INIT
// exchange (handleSAInitRequest builds the response through buildSAInitResponse),
// then read the response octets off the wire and through the parser.
func TestRFC4739ResponderAnnouncesMultipleAuth(t *testing.T) {
	_, resp, _ := autSAInitPair(t, rfc4739Auth())
	raw := resp.ResponderSAInitMsg

	// RFC requirement: RFC4739-3.1-1 positive -- the IKE_SA_INIT response built by
	// buildSAInitResponse carries exactly one MULTIPLE_AUTH_SUPPORTED notify (16404)
	// whose octets are the 8-octet generic header and notify header alone: Protocol ID 0
	// at octet 4, SPI Size 0 at octet 5, no SPI and no Notification Data.
	wireNotifies := rfc4739RawNotifies(t, raw, wire.NotifyMultipleAuthSupported)
	if len(wireNotifies) != 1 {
		t.Fatalf("IKE_SA_INIT response carries %d MULTIPLE_AUTH_SUPPORTED notifies, want 1", len(wireNotifies))
	}
	octets := wireNotifies[0]
	if len(octets) != 8 {
		t.Errorf("MULTIPLE_AUTH_SUPPORTED is %d octets on the wire, want 8 (no SPI, no data)", len(octets))
	}
	if octets[4] != 0 {
		t.Errorf("MULTIPLE_AUTH_SUPPORTED Protocol ID octet = %d, want 0", octets[4])
	}
	if octets[5] != 0 {
		t.Errorf("MULTIPLE_AUTH_SUPPORTED SPI Size octet = %d, want 0", octets[5])
	}

	parsed := rfc4739Notifies(parseMsg(t, raw).Payloads, wire.NotifyMultipleAuthSupported)
	if len(parsed) != 1 {
		t.Fatalf("parsed IKE_SA_INIT response carries %d MULTIPLE_AUTH_SUPPORTED notifies, want 1", len(parsed))
	}
	rfc4739CheckEmptyNotify(t, "MULTIPLE_AUTH_SUPPORTED", parsed[0])
}

// TestRFC4739InitiatorRequestCarriesBothNotifies proves AC-2 and the notify half of
// AC-4. Goal: a Ze initiator with a second local round, facing a responder that
// announced support, sends one MULTIPLE_AUTH_SUPPORTED and one ANOTHER_AUTH_FOLLOWS
// in its first IKE_AUTH request, beside its AUTH, each with the zero fields the RFC
// fixes. Method: the real IKE_SA_INIT exchange records the responder's announcement
// and builds the request, which the responder's keys then decrypt.
func TestRFC4739InitiatorRequestCarriesBothNotifies(t *testing.T) {
	ini, resp, _ := autSAInitPair(t, rfc4739Auth())
	if !ini.peerMultipleAuth {
		t.Fatal("the initiator did not record the responder's MULTIPLE_AUTH_SUPPORTED")
	}
	inner := rfc4739AuthRequest(t, ini, resp)

	if !rfc4555HasAuth(inner) {
		t.Fatal("the first IKE_AUTH request carries no AUTH payload")
	}

	// RFC requirement: RFC4739-3.1-1 positive -- the first IKE_AUTH request built by
	// buildAuthRequest carries exactly one MULTIPLE_AUTH_SUPPORTED notify whose parsed
	// Protocol ID and SPI Size are 0 and which holds no SPI and no Notification Data.
	supported := rfc4739Notifies(inner, wire.NotifyMultipleAuthSupported)
	if len(supported) != 1 {
		t.Fatalf("IKE_AUTH request carries %d MULTIPLE_AUTH_SUPPORTED notifies, want 1", len(supported))
	}
	rfc4739CheckEmptyNotify(t, "MULTIPLE_AUTH_SUPPORTED", supported[0])

	// RFC requirement: RFC4739-3.2-1 positive -- the same request, which carries an AUTH
	// payload, carries exactly one ANOTHER_AUTH_FOLLOWS notify whose parsed Protocol ID
	// and SPI Size are 0 and which holds no SPI and no Notification Data.
	follows := rfc4739Notifies(inner, wire.NotifyAnotherAuthFollows)
	if len(follows) != 1 {
		t.Fatalf("IKE_AUTH request carries %d ANOTHER_AUTH_FOLLOWS notifies, want 1", len(follows))
	}
	rfc4739CheckEmptyNotify(t, "ANOTHER_AUTH_FOLLOWS", follows[0])
}

// TestRFC4739NoSecondRoundNoNotifies proves AC-3. Goal: a peer with no second round
// sees neither RFC 4739 notification from Ze, as responder or initiator. Method: the
// real IKE_SA_INIT exchange, then the response octets and the decrypted IKE_AUTH
// request are searched for both types.
func TestRFC4739NoSecondRoundNoNotifies(t *testing.T) {
	ini, resp, _ := autSAInitPair(t, rfc4739PlainAuth())

	for _, msgType := range []uint16{wire.NotifyMultipleAuthSupported, wire.NotifyAnotherAuthFollows} {
		if got := rfc4739RawNotifies(t, resp.ResponderSAInitMsg, msgType); len(got) != 0 {
			t.Errorf("IKE_SA_INIT response carries %d notifies of type %d, want none", len(got), msgType)
		}
	}
	if ini.peerMultipleAuth {
		t.Error("the initiator recorded MULTIPLE_AUTH_SUPPORTED that no responder sent")
	}

	inner := rfc4739AuthRequest(t, ini, resp)
	for _, msgType := range []uint16{wire.NotifyMultipleAuthSupported, wire.NotifyAnotherAuthFollows} {
		if got := rfc4739Notifies(inner, msgType); len(got) != 0 {
			t.Errorf("IKE_AUTH request carries %d notifies of type %d, want none", len(got), msgType)
		}
	}
}

// TestRFC4739LocalRoundSkippedWithoutPeerSupport proves the request-builder half of
// AC-9. Goal: a Ze initiator with a second local round, facing a responder that did
// not announce support, still announces its own support but asks for no second round,
// and says so once in the log with the fix. Method: the IKE_SA_INIT exchange runs
// against a responder with no second round, then the initiator's peer gains a local
// second round and the first IKE_AUTH request is built again under a captured logger.
func TestRFC4739LocalRoundSkippedWithoutPeerSupport(t *testing.T) {
	ini, resp, _ := autSAInitPair(t, rfc4739PlainAuth())
	if ini.peerMultipleAuth {
		t.Fatal("fixture: the responder must not announce MULTIPLE_AUTH_SUPPORTED")
	}
	ini.PeerCfg.Auth = rfc4739Auth()

	logged := &bytes.Buffer{}
	previous := getLogger()
	setLogger(slog.New(slog.NewTextHandler(logged, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { setLogger(previous) })

	raw, err := buildAuthRequest(ini)
	if err != nil {
		t.Fatalf("buildAuthRequest: %v", err)
	}
	ini.LastSentMsg = raw
	inner := rfc4739AuthRequest(t, ini, resp)

	if got := rfc4739Notifies(inner, wire.NotifyMultipleAuthSupported); len(got) != 1 {
		t.Errorf("IKE_AUTH request carries %d MULTIPLE_AUTH_SUPPORTED notifies, want 1", len(got))
	}
	if got := rfc4739Notifies(inner, wire.NotifyAnotherAuthFollows); len(got) != 0 {
		t.Errorf("IKE_AUTH request carries %d ANOTHER_AUTH_FOLLOWS notifies to a peer without support, want none", len(got))
	}

	text := logged.String()
	const warning = "the peer does not support RFC 4739 multiple authentication"
	if n := strings.Count(text, warning); n != 1 {
		t.Fatalf("log carries the skipped-round warning %d times, want 1:\n%s", n, text)
	}
	for _, want := range []string{"level=WARN", "peer=ze", "remove authentication second-round local"} {
		if !strings.Contains(text, want) {
			t.Errorf("the skipped-round warning lacks %q:\n%s", want, text)
		}
	}
}

// TestRFC4739RoundAccessorsSelectTheRoundCredential proves the credential accessors.
// Goal: round 1 reads the peer's own authentication block and round 2 the
// second-round credential of the same direction, and a move to a round the peer does
// not configure is refused with the round unchanged. Method: an SA over each config,
// read through localAuth, remoteAuth and one reader that uses them (buildIDPayload).
func TestRFC4739RoundAccessorsSelectTheRoundCredential(t *testing.T) {
	iniPeer, _ := autPeers(rfc4739Auth())
	sa := &SA{PeerName: "ze", PeerCfg: iniPeer}

	if sa.localAuth() != &sa.PeerCfg.Auth {
		t.Error("round 1 localAuth is not the peer's own authentication block")
	}
	if sa.remoteAuth() != &sa.PeerCfg.Auth {
		t.Error("round 1 remoteAuth is not the peer's own authentication block")
	}
	if got := string(buildIDPayload(sa, true).IDData); got != "ze" {
		t.Errorf("round 1 IDi = %q, want the peer name %q", got, "ze")
	}

	if err := sa.beginLocalSecondRound(); err != nil {
		t.Fatalf("beginLocalSecondRound: %v", err)
	}
	rounds := sa.PeerCfg.Auth.SecondRound()
	if sa.localAuth() != rounds.Local {
		t.Error("round 2 localAuth is not the second-round local credential")
	}
	if got := sa.localAuth().PSK; got != "round-two-local" {
		t.Errorf("round 2 local PSK = %q, want round-two-local", got)
	}
	if got := string(buildIDPayload(sa, true).IDData); got != "ze-round-two" {
		t.Errorf("round 2 IDi = %q, want ze-round-two", got)
	}
	if sa.remoteAuth() != &sa.PeerCfg.Auth {
		t.Error("moving the local round moved the remote round too")
	}

	if err := sa.beginRemoteSecondRound(); err != nil {
		t.Fatalf("beginRemoteSecondRound: %v", err)
	}
	if sa.remoteAuth() != rounds.Remote {
		t.Error("round 2 remoteAuth is not the second-round remote credential")
	}
	if got := sa.remoteAuth().RemoteID; got != "peer-round-two" {
		t.Errorf("round 2 remote id = %q, want peer-round-two", got)
	}

	plainPeer, _ := autPeers(rfc4739PlainAuth())
	plain := &SA{PeerName: "ze", PeerCfg: plainPeer}
	if err := plain.beginLocalSecondRound(); !errors.Is(err, errNoSecondRound) {
		t.Errorf("beginLocalSecondRound on a peer with no second round = %v, want errNoSecondRound", err)
	}
	if err := plain.beginRemoteSecondRound(); !errors.Is(err, errNoSecondRound) {
		t.Errorf("beginRemoteSecondRound on a peer with no second round = %v, want errNoSecondRound", err)
	}
	if plain.localAuth() != &plain.PeerCfg.Auth {
		t.Error("a refused local move left the round changed")
	}
	if plain.remoteAuth() != &plain.PeerCfg.Auth {
		t.Error("a refused remote move left the round changed")
	}

	localOnly := rfc4739Auth()
	localOnly.Additions.SecondRound = &ipsec.SecondRoundAuth{Local: rfc4739Auth().SecondRound().Local}
	localPeer, _ := autPeers(localOnly)
	half := &SA{PeerName: "ze", PeerCfg: localPeer}
	if err := half.beginRemoteSecondRound(); !errors.Is(err, errNoSecondRound) {
		t.Errorf("beginRemoteSecondRound on a peer with only a local second round = %v, want errNoSecondRound", err)
	}
}
