//go:build ze_l2tp

package pppoe

import (
	"encoding/binary"
	"strings"
	"testing"
)

// papSessionFrame builds one PPPoE session-stage Ethernet frame carrying a PAP
// packet with code and identifier, laid out as check_pap.go documents.
func papSessionFrame(protocol uint16, code, identifier byte) []byte {
	frame := make([]byte, papFrameOctetsMin+4)
	copy(frame[0:6], replayACMAC[:])
	copy(frame[6:12], replayClientMAC[:])
	binary.BigEndian.PutUint16(frame[etherTypeOffset:], etherTypePPPoESession)
	frame[14] = 0x11
	binary.BigEndian.PutUint16(frame[16:], 0x0007)
	binary.BigEndian.PutUint16(frame[18:], uint16(len(frame)-20)) //nolint:gosec // test frame is a few dozen octets
	binary.BigEndian.PutUint16(frame[pppProtocolOffset:], protocol)
	frame[papCodeOffset] = code
	frame[papIdentifierOffset] = identifier
	binary.BigEndian.PutUint16(frame[24:], uint16(len(frame)-papCodeOffset)) //nolint:gosec // test frame is a few dozen octets
	return frame
}

// VALIDATES: observePAPFrames counts PAP Requests, Acks and Naks among PPPoE
// session frames, keeps the first Request's bytes for replay, and skips LCP.
// PREVENTS: A reanswer check that counts an LCP echo as Ze's answer, or replays
// a frame other than the client's own Authenticate-Request.
func TestObservePAPFramesCountsOnlyPAP(t *testing.T) {
	request := papSessionFrame(pppProtocolPAP, papCodeAuthRequest, 9)
	capture := buildCapture(t,
		papSessionFrame(0xc021, 9, 1),
		request,
		papSessionFrame(pppProtocolPAP, papCodeAuthAck, 9),
		papSessionFrame(pppProtocolPAP, papCodeAuthRequest, 10),
		papSessionFrame(pppProtocolPAP, papCodeAuthNak, 10),
	)
	observed, err := observePAPFrames(capture)
	if err != nil {
		t.Fatalf("observe PAP frames: %v", err)
	}
	if observed.requests != 2 || observed.acks != 1 || observed.naks != 1 {
		t.Fatalf("requests/acks/naks = %d/%d/%d, want 2/1/1", observed.requests, observed.acks, observed.naks)
	}
	if observed.requestID != 9 || observed.ackID != 9 {
		t.Fatalf("request/ack Identifier = %d/%d, want 9/9", observed.requestID, observed.ackID)
	}
	if string(observed.request) != string(request) {
		t.Fatal("observed request is not the first Authenticate-Request's bytes")
	}

	empty, err := observePAPFrames(buildCapture(t, papSessionFrame(0xc021, 9, 1)))
	if err != nil {
		t.Fatalf("observe LCP-only capture: %v", err)
	}
	if empty.acks != 0 || empty.requests != 0 {
		t.Fatalf("LCP-only capture counted PAP: %+v", empty)
	}
}

// VALIDATES: A truncated PAP frame or an unknown PAP Code fails the capture.
// PREVENTS: A corrupt capture read as "Ze sent no Ack".
func TestObservePAPFramesRefusesCorruptFrames(t *testing.T) {
	short := papSessionFrame(pppProtocolPAP, papCodeAuthAck, 1)[:papFrameOctetsMin-1]
	if _, err := observePAPFrames(buildCapture(t, short)); err == nil {
		t.Fatal("truncated PAP frame was accepted")
	}
	unknown := papSessionFrame(pppProtocolPAP, 7, 1)
	if _, err := observePAPFrames(buildCapture(t, unknown)); err == nil {
		t.Fatal("unknown PAP Code was accepted")
	}
}

// VALIDATES: papAuthEvidence requires Ze's PAP demand, the client's named
// request and Ze's Ack, and refuses any CHAP in the trace; accelPAPEvidence
// does the same over accel-ppp's trace.
// PREVENTS: A PAP scenario passing on a CHAP session.
func TestPAPAuthEvidenceRequiresPAPOnly(t *testing.T) {
	good := strings.Join([]string{
		"rcvd [LCP ConfReq id=0x1 <mru 1492> <auth pap> <magic 0x1>]",
		`sent [PAP AuthReq id=0x1 user="alice" password=<hidden>]`,
		`rcvd [PAP AuthAck id=0x1 ""]`,
	}, "\n")
	if err := papAuthEvidence(good); err != nil {
		t.Fatalf("good PAP trace refused: %v", err)
	}
	for _, bad := range []string{
		strings.Replace(good, "<auth pap>", "<auth chap MD5>", 1),
		strings.Replace(good, "rcvd [PAP AuthAck", "rcvd [PAP AuthNak", 1),
		strings.Replace(good, `user="alice"`, `user="bob"`, 1),
		good + "\nrcvd [CHAP Challenge id=0x2]",
	} {
		if err := papAuthEvidence(bad); err == nil {
			t.Fatalf("bad PAP trace accepted:\n%s", bad)
		}
	}

	accel := "ppp0:alice: recv [PAP AuthReq id=1 <alice>]\nppp0:alice: send [PAP AuthAck id=1 \"Authentication succeeded\"]"
	if err := accelPAPEvidence(accel); err != nil {
		t.Fatalf("good accel-ppp trace refused: %v", err)
	}
	for _, bad := range []string{
		strings.Replace(accel, "send [PAP AuthAck", "send [PAP AuthNak", 1),
		strings.Replace(accel, "recv [PAP AuthReq", "recv [CHAP Response", 1),
		accel + "\nsend [CHAP Challenge id=2]",
	} {
		if err := accelPAPEvidence(bad); err == nil {
			t.Fatalf("bad accel-ppp trace accepted:\n%s", bad)
		}
	}
}
