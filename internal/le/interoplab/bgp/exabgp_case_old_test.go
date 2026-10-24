// Tests for the parts of the exabgp-compat mock that only ExaBGP's 4.2 and 5.0
// fixtures use (test/exabgp-compat/old-4.2, old-5.0).
//
// VALIDATES: a 4.2 fixture's `<n>:announce:<prefix>` and `<n>:withdraw:<prefix>`
// steps are matched against the IPv4 routes the speaker's UPDATEs carry, the way
// the 4.2 mock (qa/old/4.2/qa/sbin/bgp, BGPHandler.routes) decodes them;
// `option=notification:send-notification` makes the mock answer each message
// with a Cease and pass only when the speaker closes; and
// `option=open:send-unknown-message` sends a message of type 255.
// PREVENTS: the mock refusing 13 old-4.2 fixtures as "unknown step kind" and
// two options as unread, which left those cases unrunnable.
// Related: exabgp_case.go -- readExaBGPRoute; exabgp_server.go -- matchExaBGPRoutes

//go:build ze_bgp

package bgp

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"io"
	"net"
	"strings"
	"testing"
	"time"
)

// caseLegacyUpdate builds an UPDATE whose withdrawn routes and NLRI are the
// given encoded prefixes, with no path attribute: the 4.2 mock reads only the
// two route fields, so the attributes are not what these tests pin.
func caseLegacyUpdate(withdrawn, announced []byte) []byte {
	body := make([]byte, 0, 4+len(withdrawn)+len(announced))
	body = binary.BigEndian.AppendUint16(body, uint16(len(withdrawn)))
	body = append(body, withdrawn...)
	body = binary.BigEndian.AppendUint16(body, 0)
	body = append(body, announced...)
	return speakerMessage(bgpUpdate, body)
}

// TestExaBGPCaseMatchesDecodedRoutes walks a 4.2 script whose routes arrive in
// one UPDATE that spans two groups, then an End-of-RIB the 4.2 mock skips, then
// the withdrawal of the third group.
func TestExaBGPCaseMatchesDecodedRoutes(t *testing.T) {
	spec, err := readExaBGPCase(writeCase(t,
		"1:announce:1.1.0.0/24\n2:announce:1.2.0.0/25\n3:withdraw:1.1.0.0/24\n"))
	if err != nil {
		t.Fatal(err)
	}
	// 1.1.0.0/24 is three prefix octets, 1.2.0.0/25 four (RFC 4271 Section 4.3).
	announce := caseLegacyUpdate(nil, []byte{24, 1, 1, 0, 25, 1, 2, 0, 0})
	eor := caseLegacyUpdate(nil, nil)
	withdraw := caseLegacyUpdate([]byte{24, 1, 1, 0}, nil)
	if err := serveCaseOverPipe(t, spec, [][]byte{announce, eor, withdraw}); err != nil {
		t.Fatalf("the run failed: %v", err)
	}
}

// TestExaBGPCaseRefusesAnUnexpectedRoute sends a route the script does not
// name, which the 4.2 mock answers with a failure.
func TestExaBGPCaseRefusesAnUnexpectedRoute(t *testing.T) {
	spec, err := readExaBGPCase(writeCase(t, "1:announce:1.1.0.0/24\n"))
	if err != nil {
		t.Fatal(err)
	}
	err = serveCaseOverPipe(t, spec, [][]byte{caseLegacyUpdate(nil, []byte{24, 9, 9, 9})})
	if err == nil || !strings.Contains(err.Error(), "unexpected route announce:9.9.9.0/24") {
		t.Fatalf("err = %v, want the unexpected route named", err)
	}
}

// TestExaBGPCaseRefusesAMalformedRouteUpdate sends an UPDATE whose NLRI names
// a 33-bit IPv4 prefix, which no route decodes from.
func TestExaBGPCaseRefusesAMalformedRouteUpdate(t *testing.T) {
	spec, err := readExaBGPCase(writeCase(t, "1:announce:1.1.0.0/24\n"))
	if err != nil {
		t.Fatal(err)
	}
	err = serveCaseOverPipe(t, spec, [][]byte{caseLegacyUpdate(nil, []byte{33, 1, 1, 0, 0})})
	if err == nil || !strings.Contains(err.Error(), "33") {
		t.Fatalf("err = %v, want the 33-bit prefix refused", err)
	}
}

// TestExaBGPCaseRefusesARouteScriptMistake checks the reader refuses what the
// 4.2 mock could never match: a prefix that is no IPv4 prefix, and a script
// mixing decoded routes with raw frames, which the 4.2 mock reads in one mode
// or the other for a whole case.
func TestExaBGPCaseRefusesARouteScriptMistake(t *testing.T) {
	for _, tc := range []struct{ name, fixture, want string }{
		{"no prefix", "1:announce:1.1.0/24\n", "IPv4 prefix"},
		{"IPv6", "1:withdraw:2001:db8::/32\n", "IPv4 prefix"},
		{"mixed", "1:announce:1.1.0.0/24\n2:raw:" + caseFrameEOR + "\n", "raw frames and decoded routes"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := readExaBGPCase(writeCase(t, tc.fixture))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to say %q", err, tc.want)
			}
		})
	}
}

// TestExaBGPCaseNotifiesUntilTheSpeakerCloses plays a speaker that answers the
// mock's OPEN with a KEEPALIVE, reads the Cease the mock sends back, and closes.
// The case passes on that close, as the 4.2 mock's does, although the script's
// route never arrives: its fixture says the route must not be what the helper
// sends.
func TestExaBGPCaseNotifiesUntilTheSpeakerCloses(t *testing.T) {
	spec, err := readExaBGPCase(writeCase(t, "option=notification:send-notification\n1:announce:1.2.3.4/32\n"))
	if err != nil {
		t.Fatal(err)
	}
	mock, speaker := net.Pipe()
	defer func() { _ = mock.Close() }()
	notifications := make(chan []byte, 1)
	go playClosingSpeaker(speaker, notifications)
	_ = mock.SetDeadline(time.Now().Add(5 * time.Second))
	if err := serveExaBGPConnection(mock, spec, 1, nil, io.Discard); err != nil {
		t.Fatalf("the run failed: %v", err)
	}
	want := append([]byte{6, 0}, "closing session because we can"...)
	if got := <-notifications; !bytes.Equal(got, want) {
		t.Fatalf("NOTIFICATION body = %X, want %X", got, want)
	}
}

// TestExaBGPCaseNotificationNeedsTheClose checks a speaker that keeps the
// session after the Cease fails the case rather than passing it.
func TestExaBGPCaseNotificationNeedsTheClose(t *testing.T) {
	spec, err := readExaBGPCase(writeCase(t,
		"option=notification:send-notification\noption=timeout:1\n1:announce:1.2.3.4/32\n"))
	if err != nil {
		t.Fatal(err)
	}
	mock, speaker := net.Pipe()
	defer func() { _ = mock.Close() }()
	go playKeepingSpeaker(speaker)
	err = serveExaBGPConnection(mock, spec, 1, nil, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "kept the session") {
		t.Fatalf("err = %v, want the open session named", err)
	}
}

// playKeepingSpeaker sends an OPEN and a KEEPALIVE, then reads what the mock
// sends until the mock closes the pipe: it never closes the session itself.
func playKeepingSpeaker(speaker net.Conn) {
	writeDone := make(chan error, 1)
	go writeCaseFrames(speaker, [][]byte{speakerKeepalive()}, writeDone)
	drainCaseSpeaker(speaker)
}

// playClosingSpeaker sends an OPEN and a KEEPALIVE, hands back the body of the
// first NOTIFICATION the mock writes, and closes.
func playClosingSpeaker(speaker net.Conn, notifications chan<- []byte) {
	defer func() { _ = speaker.Close() }()
	// The pipe is synchronous and the mock writes its OPEN while the speaker
	// writes, so the writes run beside the reads.
	writeDone := make(chan error, 1)
	go writeCaseFrames(speaker, [][]byte{speakerKeepalive()}, writeDone)
	for {
		messageType, body, err := readBGPWireMessage(speaker)
		if err != nil {
			notifications <- nil
			return
		}
		if messageType == bgpNotification {
			notifications <- body
			return
		}
	}
}

// TestExaBGPCaseSendsTheUnknownMessage checks the mock follows its OPEN and
// KEEPALIVE with a header-only message of type 255, as the 5.0 mock does for
// `option=open:send-unknown-message`, and does not without the option.
func TestExaBGPCaseSendsTheUnknownMessage(t *testing.T) {
	for _, tc := range []struct {
		name    string
		fixture string
		want    bool
	}{
		{"stated", "option=open:send-unknown-message\n1:raw:" + caseFrameEOR + "\n", true},
		{"not stated", "1:raw:" + caseFrameEOR + "\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			spec, err := readExaBGPCase(writeCase(t, tc.fixture))
			if err != nil {
				t.Fatal(err)
			}
			mock, speaker := net.Pipe()
			defer func() { _ = mock.Close() }()
			types := make(chan []byte, 1)
			go playTypeRecordingSpeaker(speaker, types)
			_ = mock.SetDeadline(time.Now().Add(5 * time.Second))
			if err := serveExaBGPConnection(mock, spec, 1, nil, io.Discard); err != nil {
				t.Fatalf("the run failed: %v", err)
			}
			_ = mock.Close()
			got := <-types
			if bytes.Contains(got, []byte{255}) != tc.want {
				t.Fatalf("message types the mock sent = %v, want type 255 present = %v", got, tc.want)
			}
		})
	}
}

// playTypeRecordingSpeaker sends an OPEN and an End-of-RIB, then hands back the
// type of every message the mock writes until the pipe closes.
func playTypeRecordingSpeaker(speaker net.Conn, types chan<- []byte) {
	defer func() { _ = speaker.Close() }()
	eor := caseLegacyUpdate(nil, nil)
	writeDone := make(chan error, 1)
	go writeCaseFrames(speaker, [][]byte{eor}, writeDone)
	var seen []byte
	for {
		messageType, _, err := readBGPWireMessage(speaker)
		if err != nil {
			types <- seen
			return
		}
		seen = append(seen, messageType)
	}
}

// TestExaBGPCaseUnknownMessageSkipsAnEarlyEndOfRIB plays a speaker that sends
// its initial End-of-RIB before its answer to the type 255 message, the order
// ze produces under load, and checks the mock still holds the case to the
// NOTIFICATION it expects. Without the option the same End-of-RIB stays an
// unexpected message, as upstream's mock refuses any frame its group does not
// name.
func TestExaBGPCaseUnknownMessageSkipsAnEarlyEndOfRIB(t *testing.T) {
	// RFC 4271 Section 6.1: Message Header Error (1), Bad Message Type (3),
	// the erroneous Type field 0xFF as its Data.
	const notification = "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF" + "0016" + "03" + "01" + "03" + "FF"
	eor, _ := hex.DecodeString(caseFrameEOR)    //nolint:errcheck // a constant of this file
	answer, _ := hex.DecodeString(notification) //nolint:errcheck // a constant of this test
	for _, tc := range []struct {
		name    string
		option  string
		wantErr string
	}{
		{"stated", "option=open:send-unknown-message\n", ""},
		{"not stated", "", "unexpected message = " + caseFrameEOR},
	} {
		t.Run(tc.name, func(t *testing.T) {
			spec, err := readExaBGPCase(writeCase(t, tc.option+"1:raw:"+notification+"\n"))
			if err != nil {
				t.Fatal(err)
			}
			err = serveCaseOverPipe(t, spec, [][]byte{eor, answer})
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("the run failed: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("the run answered %v, want an error naming %q", err, tc.wantErr)
			}
		})
	}
}
