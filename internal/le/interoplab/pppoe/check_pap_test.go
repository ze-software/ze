//go:build ze_l2tp

package pppoe

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/interoplab"
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

// papFrameOnSession is papSessionFrame on PPPoE session sid.
func papFrameOnSession(sid uint16, code, identifier byte) []byte {
	frame := papSessionFrame(pppProtocolPAP, code, identifier)
	binary.BigEndian.PutUint16(frame[pppoeSessionIDOffset:], sid)
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
	if observed.requestSession != 7 || observed.ackSession != 7 {
		t.Fatalf("request/ack SESSION_ID = %d/%d, want 7/7", observed.requestSession, observed.ackSession)
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

// papReplayLab is a fake lab that answers the capture, REST and link probes
// checkPAPReanswer makes. The capture it hands back is whatever reply built
// from the frame the fake sender was given, so each case scripts Ze's answer.
type papReplayLab struct {
	fakeLab
	sent    [][]byte
	running bool
	capture []byte
	restSID int
}

func newPAPReplayLab(t *testing.T, restSID int, reply func(sent []byte) [][]byte) (*papReplayLab, clientFrameSender) {
	t.Helper()
	lab := &papReplayLab{restSID: restSID}
	lab.detachedFn = func(_ string, argv []string) error {
		if !strings.Contains(strings.Join(argv, " "), "tcpdump -i eth0 -w "+sessionCaptureFile) {
			return errors.New("unexpected detached command")
		}
		lab.running = true
		return nil
	}
	lab.execFn = func(_ string, argv []string) (interoplab.CommandResult, error) {
		switch strings.Join(argv, " ") {
		case "pgrep -x tcpdump":
			if lab.running {
				return interoplab.CommandResult{}, nil
			}
			return interoplab.CommandResult{ExitCode: 1}, errors.New("exit status 1")
		case "pkill -INT -x tcpdump":
			lab.running = false
			return interoplab.CommandResult{}, nil
		case "ip -o link show type ppp":
			return interoplab.CommandResult{Stdout: "12: ppp0: <POINTOPOINT> mtu 1492\n"}, nil
		default:
			return interoplab.CommandResult{}, errors.New("unexpected exec: " + strings.Join(argv, " "))
		}
	}
	lab.queryFn = func(_ string, argv []string) (string, error) {
		command := strings.Join(argv, " ")
		if command == "sh -c base64 "+sessionCaptureFile {
			return base64.StdEncoding.EncodeToString(lab.capture), nil
		}
		if argv[0] == "curl" {
			var body strings.Builder
			body.WriteString(`{"status":"done","data":[{"sid":`)
			body.WriteString(strconv.Itoa(lab.restSID))
			body.WriteString(`,"service-name":"internet","interface":"eth0"}]}`)
			return body.String(), nil
		}
		return "", errors.New("unexpected query: " + command)
	}
	send := func(_ context.Context, _ interoplab.CheckerLab, frame []byte) error {
		lab.sent = append(lab.sent, append([]byte(nil), frame...))
		lab.capture = buildCapture(t, reply(frame)...)
		return nil
	}
	return lab, send
}

// VALIDATES: checkPAPReanswer sends exactly one replay, with the original
// request's bytes except an Identifier advanced by one, and accepts only one
// Ack carrying that new Identifier on Ze's session, with no Nak, Ze's session
// unchanged and one client PPP link.
// PREVENTS: A reanswer check that passes on a cached Ack resent with the old
// Identifier, on a Nak, on silence, on an Ack for another session, or after the
// replay replaced Ze's session.
// METHOD: A fake sender scripts Ze's reply from the frame it was handed; a fake
// lab answers the capture, REST and link probes. Each case waits out the real
// replay bound, so the cases run in parallel.
func TestCheckPAPReanswerJudgesTheReply(t *testing.T) {
	const sid = 7
	request := papFrameOnSession(sid, papCodeAuthRequest, 9)
	first := papObservation{requests: 1, acks: 1, request: request, requestID: 9, requestSession: sid, ackID: 9, ackSession: sid}

	echo := func(sent []byte) []byte { return sent }
	ackWith := func(id byte) []byte { return papFrameOnSession(sid, papCodeAuthAck, id) }
	cases := []struct {
		name    string
		restSID int
		reply   func(sent []byte) [][]byte
		wantErr string
	}{
		{"ack with new identifier", sid, func(sent []byte) [][]byte {
			return [][]byte{echo(sent), ackWith(sent[papIdentifierOffset])}
		}, ""},
		{"ack with stale identifier", sid, func(sent []byte) [][]byte {
			return [][]byte{echo(sent), ackWith(9)}
		}, "Section 2.2.2"},
		{"nak", sid, func(sent []byte) [][]byte {
			return [][]byte{echo(sent), papFrameOnSession(sid, papCodeAuthNak, sent[papIdentifierOffset])}
		}, "0 Acks and 1 Naks"},
		{"ack and nak", sid, func(sent []byte) [][]byte {
			id := sent[papIdentifierOffset]
			return [][]byte{echo(sent), ackWith(id), papFrameOnSession(sid, papCodeAuthNak, id)}
		}, "1 Naks"},
		{"zero acks", sid, func(sent []byte) [][]byte {
			return [][]byte{echo(sent)}
		}, "0 Acks"},
		{"two acks", sid, func(sent []byte) [][]byte {
			id := sent[papIdentifierOffset]
			return [][]byte{echo(sent), ackWith(id), ackWith(id)}
		}, "2 Acks"},
		{"ack on another session", sid, func(sent []byte) [][]byte {
			return [][]byte{echo(sent), papFrameOnSession(sid+1, papCodeAuthAck, sent[papIdentifierOffset])}
		}, "Ack travelled on PPPoE session 8"},
		{"replay not on the wire", sid, func(sent []byte) [][]byte {
			return [][]byte{ackWith(sent[papIdentifierOffset])}
		}, "0 PAP Authenticate-Requests"},
		{"session replaced", sid + 1, func(sent []byte) [][]byte {
			return [][]byte{echo(sent), ackWith(sent[papIdentifierOffset])}
		}, "replaced Ze's session 7 with session 8"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			lab, send := newPAPReplayLab(t, tc.restSID, tc.reply)
			err := checkPAPReanswer(t.Context(), lab, first, sid, send)
			if len(lab.sent) != 1 {
				t.Fatalf("sender called %d times, want 1", len(lab.sent))
			}
			want := append([]byte(nil), request...)
			want[papIdentifierOffset] = 10
			if !bytes.Equal(lab.sent[0], want) {
				t.Fatalf("replayed frame % x, want the request with Identifier 10: % x", lab.sent[0], want)
			}
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("conformant reply refused: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("reply accepted, want error containing %q", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error %q, want it to contain %q", err, tc.wantErr)
			}
		})
	}
}
