package engine

import (
	"bytes"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/transport"
	"github.com/ze-software/ze/internal/component/ike/wire"
	"github.com/ze-software/ze/internal/core/slogutil"
)

// ntfRequest builds a bare 28-byte IKE header for a datagram that names an SA the
// receiver does not hold. Nothing but the header is needed: the emitter under test
// never parses a payload chain.
func ntfRequest(ispi, rspi [8]byte, exchange uint8, msgID uint32, response bool) []byte {
	flags := uint8(0)
	if response {
		flags = wire.FlagResponse
	}
	hdr := wire.Header{
		InitiatorSPI: ispi,
		ResponderSPI: rspi,
		MajorVersion: 2,
		ExchangeType: exchange,
		Flags:        flags,
		MessageID:    msgID,
		Length:       wire.HeaderLen,
	}
	buf := make([]byte, wire.HeaderLen)
	hdr.WriteTo(buf, 0)
	return buf
}

// ntfSPI builds a distinguishable eight-octet SPI.
func ntfSPI(seed byte) [8]byte {
	var s [8]byte
	for i := range s {
		s[i] = seed + byte(i)
	}
	return s
}

// ntfEmit drives the out-of-SA emitter with a fresh limiter and returns the datagram
// the peer read, or nil when nothing was sent.
// The sentinel proves an absence, and it does not wait on a clock.
func ntfEmit(t *testing.T, req []byte, natT bool) []byte {
	t.Helper()
	log := slogutil.DiscardLogger()
	peerTr, myTr := rtxPeerLink(t)
	remote, ok := peerTr.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatal("peer transport local address is not *net.UDPAddr")
	}
	limiter := newOutboundNotifyLimiter(unprotectedNotifyRate, unprotectedNotifyBurst)
	answerOutOfSA(myTr, transport.Packet{Data: req, RemoteAddr: remote}, natT, limiter, log)

	// One socket keeps send order on loopback, so a sentinel that arrives first proves
	// the emitter wrote nothing.
	if err := myTr.Send(rtxSentinel, remote); err != nil {
		t.Fatalf("send sentinel: %v", err)
	}
	got := rtxRecv(t, peerTr)
	if got == nil {
		t.Fatal("the sentinel never arrived")
	}
	if bytes.Equal(got, rtxSentinel) {
		return nil
	}
	return got
}

// VALIDATES: a datagram naming an unknown IKE SA draws an INVALID_IKE_SPI response
// whose header copies the request, at the address the request came from.
// RFC requirement: RFC7296-2.21.4-2 positive -- sendInvalidIKESPI copies both SPIs, the
// Exchange Type and the Message ID from the request, sets the Response flag, and sends
// to pkt.RemoteAddr. The fixture uses a NON-ZERO Message ID, because the neighboring
// sendSAInitNotify hardcodes MessageID 0 and a zero fixture would pass against it.
// RFC requirement: RFC7296-2.21.4-2 negative -- a second request carrying different
// SPIs and a different Message ID draws a response carrying THOSE values, so the fields
// are copied from each request rather than fixed.
// RFC requirement: RFC7296-2.21.4-4 positive -- the response holds exactly one Notify
// payload, of type INVALID_IKE_SPI, with an empty SPI field and no Notification Data.
// RFC requirement: RFC7296-2.21.4-4 negative -- a request the emitter refuses to answer
// yields no payload at all, so the Notify is present only when the emitter ran.
func TestNtfOutOfSAAnswersWithInvalidIKESPI(t *testing.T) {
	ispi, rspi := ntfSPI(0x11), ntfSPI(0x91)
	got := ntfEmit(t, ntfRequest(ispi, rspi, wire.ExchangeInformational, 7, false), false)
	if got == nil {
		t.Fatal("a request naming an unknown SA drew no answer")
	}
	var msg wire.Message
	if err := msg.ReadFrom(got); err != nil {
		t.Fatalf("the answer does not parse as an IKE message: %v", err)
	}
	if msg.Header.InitiatorSPI != ispi {
		t.Errorf("initiator SPI = %x, want the request's %x", msg.Header.InitiatorSPI, ispi)
	}
	if msg.Header.ResponderSPI != rspi {
		t.Errorf("responder SPI = %x, want the request's %x", msg.Header.ResponderSPI, rspi)
	}
	if msg.Header.MessageID != 7 {
		t.Errorf("message ID = %d, want the request's 7", msg.Header.MessageID)
	}
	if msg.Header.ExchangeType != wire.ExchangeInformational {
		t.Errorf("exchange type = %d, want the request's INFORMATIONAL", msg.Header.ExchangeType)
	}
	if msg.Header.Flags&wire.FlagResponse == 0 {
		t.Error("the answer is not marked as a response")
	}
	if msg.Header.Flags&wire.FlagInitiator != 0 {
		t.Error("the answer sets the Initiator bit, which it never originated")
	}
	if msg.Header.MajorVersion != 2 || msg.Header.MinorVersion != 0 {
		t.Errorf("version = %d.%d, want 2.0", msg.Header.MajorVersion, msg.Header.MinorVersion)
	}

	if len(msg.Payloads) != 1 {
		t.Fatalf("payload count = %d, want exactly one Notify", len(msg.Payloads))
	}
	notify, ok := msg.Payloads[0].Payload.(*wire.PayloadNotify)
	if !ok {
		t.Fatalf("payload type = %T, want *wire.PayloadNotify", msg.Payloads[0].Payload)
	}
	if notify.NotifyMsgType != wire.NotifyInvalidIKESPI {
		t.Errorf("notify type = %d, want INVALID_IKE_SPI", notify.NotifyMsgType)
	}
	if notify.SPISize != 0 || len(notify.SPI) != 0 {
		t.Errorf("notify carries an SPI (%d octets), want an empty SPI field", notify.SPISize)
	}
	if len(notify.NotificationData) != 0 {
		t.Errorf("notify carries %d octets of data, want none", len(notify.NotificationData))
	}

	// Negative half one. A different request draws a response carrying its own values.
	ispi2, rspi2 := ntfSPI(0x44), ntfSPI(0xC4)
	got2 := ntfEmit(t, ntfRequest(ispi2, rspi2, wire.ExchangeCreateChildSA, 99, false), false)
	if got2 == nil {
		t.Fatal("the second request drew no answer")
	}
	var msg2 wire.Message
	if err := msg2.ReadFrom(got2); err != nil {
		t.Fatalf("the second answer does not parse: %v", err)
	}
	if msg2.Header.InitiatorSPI != ispi2 || msg2.Header.ResponderSPI != rspi2 {
		t.Error("the second answer did not copy the second request's SPIs")
	}
	if msg2.Header.MessageID != 99 {
		t.Errorf("second message ID = %d, want 99", msg2.Header.MessageID)
	}
	if msg2.Header.ExchangeType != wire.ExchangeCreateChildSA {
		t.Errorf("second exchange type = %d, want CREATE_CHILD_SA", msg2.Header.ExchangeType)
	}

	// Negative half two. A request the emitter refuses yields no payload at all.
	if refused := ntfEmit(t, ntfRequest(ispi, rspi, wire.ExchangeIKESAInit, 0, false), false); refused != nil {
		t.Error("an IKE_SA_INIT drew an answer, so the payload is not conditional on the guards")
	}
}

// VALIDATES: a message marked as a response is audited and never answered.
// RFC requirement: RFC7296-2.21.4-1 positive -- sendInvalidIKESPI returns before it
// builds anything when the Response flag is set, so an out-of-SA response draws
// nothing. This is the guard that makes the emitter unable to start a message loop.
// RFC requirement: RFC7296-2.21.4-1 negative -- the identical datagram with the
// Response flag CLEAR does draw INVALID_IKE_SPI. Without this half the positive would
// also pass if the whole emitter were deleted.
func TestNtfOutOfSAIgnoresResponses(t *testing.T) {
	ispi, rspi := ntfSPI(0x21), ntfSPI(0xA1)
	if got := ntfEmit(t, ntfRequest(ispi, rspi, wire.ExchangeInformational, 3, true), false); got != nil {
		t.Errorf("a message marked as a response drew %d bytes back", len(got))
	}
	if got := ntfEmit(t, ntfRequest(ispi, rspi, wire.ExchangeInformational, 3, false), false); got == nil {
		t.Fatal("the same datagram marked as a request drew nothing")
	}
}

// VALIDATES: an IKE_SA_INIT request is a request to START an SA, so the out-of-SA
// answer never applies to it, and the limiter bounds what remains.
// RFC requirement: RFC7296-2.21.4-1 positive -- the exchange type is tested explicitly.
// It is not inferred from a refusal by tryResponderSAInit.
// That function also refuses an unconfigured source, and a non-zero responder SPI.
// Both of those are still IKE_SA_INIT requests, and neither can draw this answer.
// RFC requirement: RFC7296-2.21.4-1 negative -- the same header with an INFORMATIONAL
// exchange type IS answered, so the refusal comes from the exchange type.
func TestNtfOutOfSASkipsSAInitAndRateLimits(t *testing.T) {
	log := slogutil.DiscardLogger()
	ispi, rspi := ntfSPI(0x31), ntfSPI(0xB1)

	if got := ntfEmit(t, ntfRequest(ispi, rspi, wire.ExchangeIKESAInit, 0, false), false); got != nil {
		t.Errorf("an IKE_SA_INIT drew %d bytes back", len(got))
	}
	if got := ntfEmit(t, ntfRequest(ispi, rspi, wire.ExchangeInformational, 0, false), false); got == nil {
		t.Fatal("the same header with an INFORMATIONAL exchange drew nothing")
	}

	// RFC 7296 Section 2.21.4: "A node needs to limit the rate at which it will send
	// messages in response to unprotected messages." One limiter, more requests than
	// its burst, and the surplus is refused.
	t.Run("the limiter denies past its burst", func(t *testing.T) {
		peerTr, myTr := rtxPeerLink(t)
		remote, ok := peerTr.LocalAddr().(*net.UDPAddr)
		if !ok {
			t.Fatal("peer transport local address is not *net.UDPAddr")
		}
		limiter := newOutboundNotifyLimiter(unprotectedNotifyRate, unprotectedNotifyBurst)
		req := ntfRequest(ispi, rspi, wire.ExchangeInformational, 5, false)
		before := errorNotifySentCount(wire.NotifyInvalidIKESPI, false)
		for range unprotectedNotifyBurst + 20 {
			answerOutOfSA(myTr, transport.Packet{Data: req, RemoteAddr: remote}, false, limiter, log)
		}
		sent := errorNotifySentCount(wire.NotifyInvalidIKESPI, false) - before
		if sent > uint64(unprotectedNotifyBurst)+1 {
			t.Errorf("the limiter let %d messages out for a burst of %d", sent, unprotectedNotifyBurst)
		}
		if sent == 0 {
			t.Error("the limiter refused every message, so the emitter never runs")
		}
	})

	// A nil limiter denies. A caller that forgot to build one sends nothing rather
	// than sending without a bound (ai/rules/evidence.md).
	t.Run("a nil limiter denies", func(t *testing.T) {
		var nilLimiter *outboundNotifyLimiter
		if nilLimiter.allow() {
			t.Error("a nil limiter allowed a send")
		}
	})
}

// VALIDATES: the out-of-SA answer carries no cryptographic protection.
// RFC requirement: RFC7296-2.21.4-3 positive -- the emitted datagram parses with no
// Encrypted payload in it, and it holds one Notify in the clear.
// The real guarantee is structural rather than observed.
// sendInvalidIKESPI takes no *SA and no *PeerSession.
// No key material is reachable from any argument, and it can protect nothing.
// A test can confirm the output. The signature is what makes the property hold.
// RFC requirement: RFC7296-2.21.4-3 negative -- an error response for the SAME notify
// class built by buildErrorNotifyResponse on an established SA DOES carry an Encrypted
// payload. So the absence belongs to the unprotected emitter, not to notifies at large.
func TestNtfOutOfSAAnswerIsUnprotected(t *testing.T) {
	got := ntfEmit(t, ntfRequest(ntfSPI(0x51), ntfSPI(0xD1), wire.ExchangeInformational, 11, false), false)
	if got == nil {
		t.Fatal("the emitter wrote nothing")
	}
	var msg wire.Message
	if err := msg.ReadFrom(got); err != nil {
		t.Fatalf("the answer does not parse: %v", err)
	}
	if carriesSKPayload(&msg) {
		t.Error("the unprotected answer carries an Encrypted payload")
	}
	if len(msg.Payloads) != 1 {
		t.Fatalf("payload count = %d, want one Notify in the clear", len(msg.Payloads))
	}
	if _, ok := msg.Payloads[0].Payload.(*wire.PayloadNotify); !ok {
		t.Errorf("payload type = %T, want *wire.PayloadNotify", msg.Payloads[0].Payload)
	}

	// Negative. The protected sender puts the same notify class inside SK.
	_, resp, _ := establishPSK(t)
	protected, err := buildErrorNotifyResponse(resp, 4, wire.ExchangeInformational, wire.NotifyInvalidSyntax, nil)
	if err != nil {
		t.Fatalf("buildErrorNotifyResponse: %v", err)
	}
	var pmsg wire.Message
	if err := pmsg.ReadFrom(protected); err != nil {
		t.Fatalf("the protected answer does not parse: %v", err)
	}
	if !carriesSKPayload(&pmsg) {
		t.Error("the protected answer carries no Encrypted payload, so the absence above proves nothing")
	}

	// buildErrorNotifyResponse fails closed on a nil SA rather than emitting in clear.
	if _, err := buildErrorNotifyResponse(nil, 4, wire.ExchangeInformational, wire.NotifyInvalidSyntax, nil); err == nil {
		t.Error("the protected builder accepted a nil SA")
	}
}

// VALIDATES: feeding the emitter its own output back produces nothing, so two nodes
// running this code cannot exchange error notifications without end.
// RFC requirement: RFC7296-2.21.4-5 positive -- the exact bytes sendInvalidIKESPI
// produces, delivered back into the same out-of-SA branch, emit nothing.
// The output carries the Response flag, and the Response guard refuses it.
// The emitter is therefore a fixed point in one step.
// The argument needs no timing and no second daemon.
// RFC requirement: RFC7296-2.21.4-5 negative -- the same bytes with the Response flag
// cleared DO draw an answer, so the silence above comes from the guard rather than
// from the datagram being unparseable or otherwise inert.
func TestNtfEmitterIsAFixedPoint(t *testing.T) {
	first := ntfEmit(t, ntfRequest(ntfSPI(0x61), ntfSPI(0xE1), wire.ExchangeInformational, 13, false), false)
	if first == nil {
		t.Fatal("the emitter wrote nothing to feed back")
	}
	if again := ntfEmit(t, first, false); again != nil {
		t.Fatalf("the emitter answered its own output with %d bytes, so two nodes would loop", len(again))
	}

	// Negative. Clear the Response flag on those very bytes and the answer returns.
	cleared := append([]byte(nil), first...)
	cleared[19] &^= wire.FlagResponse
	if got := ntfEmit(t, cleared, false); got == nil {
		t.Fatal("the same bytes as a request drew nothing, so the fixed point is vacuous")
	}
}

// VALIDATES: the emitter answers on the NAT-T socket with the non-ESP marker, and on
// port 500 without it.
// RFC requirement: RFC7296-2.21.4-2 positive -- the answer goes back over the transport
// the request arrived on, in that socket's framing. RFC 3948 Section 2.2 prefixes IKE
// on port 4500 with four zero octets, so a peer on 4500 would read an unmarked answer
// as ESP and drop it.
// RFC requirement: RFC7296-2.21.4-2 negative -- the port 500 answer carries no marker,
// so the four octets are added by the NAT-T path rather than by the builder.
func TestNtfOutOfSAAnswerCarriesTheSocketFraming(t *testing.T) {
	req := ntfRequest(ntfSPI(0x71), ntfSPI(0xF1), wire.ExchangeInformational, 17, false)

	plain := ntfEmit(t, req, false)
	if plain == nil {
		t.Fatal("the port 500 path wrote nothing")
	}
	if _, marked := transport.StripNonESPMarker(plain); marked && len(plain) > wire.HeaderLen {
		if plain[0] == 0 && plain[1] == 0 && plain[2] == 0 && plain[3] == 0 {
			t.Error("the port 500 answer carries a non-ESP marker")
		}
	}

	natted := ntfEmit(t, req, true)
	if natted == nil {
		t.Fatal("the NAT-T path wrote nothing")
	}
	if len(natted) != len(plain)+4 {
		t.Errorf("NAT-T answer is %d bytes and the plain answer is %d, want four more", len(natted), len(plain))
	}
	stripped, isIKE := transport.StripNonESPMarker(natted)
	if !isIKE {
		t.Fatal("the NAT-T answer carries no non-ESP marker, so a peer reads it as ESP")
	}
	var msg wire.Message
	if err := msg.ReadFrom(stripped); err != nil {
		t.Fatalf("the NAT-T answer does not parse once stripped: %v", err)
	}
	if msg.Header.MessageID != 17 {
		t.Errorf("NAT-T answer message ID = %d, want 17", msg.Header.MessageID)
	}
}

// ntfNotifyConstant matches a reference to a wire notify constant in ze source.
var ntfNotifyConstant = regexp.MustCompile(`wire\.(Notify[A-Za-z0-9]+)\b`)

// ntfNotifyLiteral matches a bare numeric notify type assigned to a NotifyMsgType
// field. That is the one route by which a private-range value reaches the wire, and
// it names no constant.
var ntfNotifyLiteral = regexp.MustCompile(`NotifyMsgType:\s*(\d+)`)

// VALIDATES: every notify message type ze can put on the wire is one RFC 7296 defines.
// RFC requirement: RFC7296-2.21.2-3 positive -- the set of notify types ze transmits
// is DERIVED from a scan of the ike source for references.
// It never comes from a list written beside the assertion.
// Each member is in the wire package registry of RFC-defined types.
// Ze therefore uses no extension error notification at all.
// That is the strongest form the obligation can take.
// Ze cannot use one "unless the peer has been shown to understand them", and it
// defines none.
// RFC requirement: RFC7296-2.21.2-3 negative -- THIS ARGUMENT IS AN ABSENCE, and it is
// recorded as one deliberately.
// No negative rests on a property the code has, because the obligation governs a
// surface ze does not have.
//
// The test EXPIRES the moment an extension-notify surface arrives.
// At that point this assertion fails.
// The failure is the signal that the real obligation has become reachable and needs a
// real guard. That obligation is
// "MUST NOT use them unless the peer has been shown to understand them",
// such as by a Vendor ID payload.
//
// Do not relax this test to accommodate the new constant.
// Add the peer-understanding check the RFC requires.
func TestNtfNotifyVocabularyIsRFCDefined(t *testing.T) {
	// rfc-test-change-approved: 2026-07-31 owner standing approval for
	// docs/architecture/ike/rfcgate-1b-rfc7296-pilot.md, strengthening only.
	// The scan reads the working tree with os.ReadFile, which no build overlay reaches.
	// A mutation of the ike source therefore cannot reach the assertion, and the test
	// was unfalsifiable.
	// The sub-test below proves the DETECTOR on a fixture, which a mutation can reach.
	t.Run("the detector rejects a private-range notify", func(t *testing.T) {
		dir := t.TempDir()
		bad := filepath.Join(dir, "sender.go")
		if err := os.WriteFile(bad, []byte(
			"package x\n\nvar n = Notify{NotifyMsgType: 40961}\nvar m = wire.NotifyPrivateRange\n"), 0o600); err != nil {
			t.Fatalf("write fixture: %v", err)
		}
		names, literals := ntfScanNotifyUse(t, dir)
		if len(literals) != 1 || literals[0] != "40961" {
			t.Errorf("the scan found bare literals %v, want [40961]", literals)
		}
		if !names["NotifyPrivateRange"] {
			t.Errorf("the scan found names %v, want NotifyPrivateRange among them", names)
		}
		if _, ok := ntfConstantValue("NotifyPrivateRange"); ok {
			t.Error("an unregistered constant name resolved, so the registry check is vacuous")
		}
		// rfc-test-change-approved: 2026-07-31 owner standing approval for
		// docs/architecture/ike/rfcgate-1b-rfc7296-pilot.md, strengthening only.
		// The registry check below loops over names that are all registered today.
		// It stayed green when NotifyTypeRecognized was mutated to answer true for
		// everything. This asserts the registry can still say no.
		if wire.NotifyTypeRecognized(40961) {
			t.Error("the RFC registry vouched for the private-range type 40961, so the " +
				"check that ze transmits only RFC-defined types cannot fail")
		}
	})

	root := filepath.Join("..", "..", "..", "..", "internal", "component", "ike")
	seen, literals := ntfScanNotifyUse(t, root)
	for _, lit := range literals {
		t.Errorf("the ike source assigns the bare notify value %s to NotifyMsgType. "+
			"A notify type reaches the wire through a named wire.Notify constant, "+
			"so the registry can vouch for it", lit)
	}
	if len(seen) == 0 {
		t.Fatal("the scan found no notify constant at all, so it proves nothing")
	}

	// Every name the source uses must resolve to a type the wire registry knows, and
	// the registry holds only types RFC 7296 defines.
	for name := range seen {
		value, ok := ntfConstantValue(name)
		if !ok {
			t.Errorf("the source names wire.%s, which this test cannot resolve. "+
				"Add it to ntfConstantValue beside its declaration", name)
			continue
		}
		if !wire.NotifyTypeRecognized(value) {
			t.Errorf("ze can transmit notify type %d (wire.%s), which the RFC registry "+
				"does not hold. RFC 7296 Section 2.21.2 forbids using an extension error "+
				"notification unless the peer has been shown to understand it", value, name)
		}
	}
}

// ntfScanNotifyUse walks a directory of Go source.
// It reports the wire notify constant names the source references.
// It also reports any bare numeric value assigned to a NotifyMsgType field.
func ntfScanNotifyUse(t *testing.T, root string) (map[string]bool, []string) {
	t.Helper()
	seen := map[string]bool{}
	var literals []string

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		body, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := string(body)
		for _, m := range ntfNotifyConstant.FindAllStringSubmatch(text, -1) {
			seen[m[1]] = true
		}
		for _, m := range ntfNotifyLiteral.FindAllStringSubmatch(text, -1) {
			literals = append(literals, m[1])
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}
	return seen, literals
}

// ntfConstantValue resolves a wire notify constant name to its value.
// It is the one place a name and a value are paired for this test.
// An unresolvable name is a loud failure, not a silent skip
// (ai/rules/evidence.md).
func ntfConstantValue(name string) (uint16, bool) {
	known := map[string]uint16{
		"NotifyUnsupportedCriticalPayload": wire.NotifyUnsupportedCriticalPayload,
		"NotifyInvalidIKESPI":              wire.NotifyInvalidIKESPI,
		"NotifyInvalidMajorVersion":        wire.NotifyInvalidMajorVersion,
		"NotifyInvalidSyntax":              wire.NotifyInvalidSyntax,
		"NotifyInvalidMessageID":           wire.NotifyInvalidMessageID,
		"NotifyInvalidSPI":                 wire.NotifyInvalidSPI,
		"NotifyNoProposalChosen":           wire.NotifyNoProposalChosen,
		"NotifyInvalidKEPayload":           wire.NotifyInvalidKEPayload,
		"NotifyAuthenticationFailed":       wire.NotifyAuthenticationFailed,
		"NotifySinglePairRequired":         wire.NotifySinglePairRequired,
		"NotifyNoAdditionalSAs":            wire.NotifyNoAdditionalSAs,
		"NotifyInternalAddressFailure":     wire.NotifyInternalAddressFailure,
		"NotifyFailedCPRequired":           wire.NotifyFailedCPRequired,
		"NotifyTSUnacceptable":             wire.NotifyTSUnacceptable,
		"NotifyInvalidSelectors":           wire.NotifyInvalidSelectors,
		"NotifyTemporaryFailure":           wire.NotifyTemporaryFailure,
		"NotifyChildSANotFound":            wire.NotifyChildSANotFound,
		"NotifyInitialContact":             wire.NotifyInitialContact,
		"NotifySetWindowSize":              wire.NotifySetWindowSize,
		"NotifyAdditionalTSPossible":       wire.NotifyAdditionalTSPossible,
		"NotifyIPCompSupported":            wire.NotifyIPCompSupported,
		"NotifyNATDetectionSourceIP":       wire.NotifyNATDetectionSourceIP,
		"NotifyNATDetectionDestIP":         wire.NotifyNATDetectionDestIP,
		"NotifyCookie":                     wire.NotifyCookie,
		"NotifyUseTransportMode":           wire.NotifyUseTransportMode,
		"NotifyHTTPCertLookupSupported":    wire.NotifyHTTPCertLookupSupported,
		"NotifyRekeySA":                    wire.NotifyRekeySA,
		"NotifyESPTFCPaddingNotSupported":  wire.NotifyESPTFCPaddingNotSupported,
		"NotifyNonFirstFragmentsAlso":      wire.NotifyNonFirstFragmentsAlso,
		"NotifyFragmentationSupported":     wire.NotifyFragmentationSupported,
		"NotifySignatureHashAlgorithms":    wire.NotifySignatureHashAlgorithms,
		"NotifyStatusFloor":                wire.NotifyStatusFloor,
		"NotifyIsError":                    0,
		"NotifyTypeRecognized":             0,
		"NotifyTypeName":                   0,
	}
	v, ok := known[name]
	if !ok {
		return 0, false
	}
	// The three helper names resolve to a sentinel that the registry holds, because
	// they are function references rather than transmittable types.
	if v == 0 && strings.HasPrefix(name, "NotifyType") || name == "NotifyIsError" {
		return wire.NotifyUnsupportedCriticalPayload, true
	}
	return v, true
}
