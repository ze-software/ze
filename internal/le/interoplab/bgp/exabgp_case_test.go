// Tests for the exabgp-compat mock's case reader and its script walk.
//
// VALIDATES: a `.ci` case passes only when every frame it names arrives, in
// the order its sequence numbers give, and a case that names no frame is
// refused before any session opens.
// PREVENTS: the vacuous greens found on 2026-10-17, where a frame written
// without colons was skipped, a bare sequence number was read as a session the
// mock never served, and a case with no frame passed on the OPEN alone.
// Related: exabgp_server.go -- readExaBGPCase, orderExaBGPCase, serveExaBGPConnection

//go:build ze_bgp

package bgp

import (
	"bytes"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The frames these tests name: two End-of-RIB markers and a withdrawal, each a
// whole BGP message.
const (
	caseFrameEOR       = "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF00170200000000"
	caseFrameWithdraw  = "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001B02000418CB00710000"
	caseFrameEORIPv6   = "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001E0200000007900F0003000201"
	caseFramePunctured = "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF:001B:02:000418CB00710000"
	// caseFrameOpen is the OPEN playCaseSpeaker sends: version 4, AS 65001,
	// hold time 90, router id 1.1.1.1, no parameters.
	caseFrameOpen = "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001D0104FDE9005A0101010100"
	// caseFrameEORIPv6Short is caseFrameEORIPv6 with a one-octet attribute
	// length, the other form RFC 4724 Section 2 admits.
	caseFrameEORIPv6Short = "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF001D0200000006800F03000201"
)

func writeCase(t *testing.T, fixture string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "case.ci")
	if err := os.WriteFile(path, []byte(fixture), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestExaBGPCaseReadsAFrameWithoutColons reads one frame written as one hex
// run and one written with colons, and checks both become the same frame step.
// The first shape was skipped until 2026-10-17, which is what let a probe of
// api-sr-policy pass with ze refusing the announce.
func TestExaBGPCaseReadsAFrameWithoutColons(t *testing.T) {
	spec, err := readExaBGPCase(writeCase(t, "1:raw:"+caseFrameWithdraw+"\n2:raw:"+caseFramePunctured+"\n"))
	if err != nil {
		t.Fatal(err)
	}
	steps := spec.steps[1]
	if len(steps) != 2 {
		t.Fatalf("session 1 holds %d steps, want 2: %+v", len(steps), steps)
	}
	for index, step := range steps {
		if step.kind != exabgpStepFrame || !strings.EqualFold(hex.EncodeToString(step.frame), caseFrameWithdraw) {
			t.Fatalf("step %d = kind %v frame %X, want the withdrawal", index, step.kind, step.frame)
		}
	}
}

// TestExaBGPCaseNumberIsASequence checks a bare number is a sequence of the
// first session, ordered numerically, and a letter opens a session of its own.
// The number was read as a session, so api-rib (numbered from 10) asserted
// nothing.
func TestExaBGPCaseNumberIsASequence(t *testing.T) {
	spec, err := readExaBGPCase(writeCase(t, "20:raw:"+caseFrameWithdraw+"\n3:raw:"+caseFrameEOR+"\n10:raw:"+caseFrameEORIPv6+"\n"))
	if err != nil {
		t.Fatal(err)
	}
	if spec.connections != 1 {
		t.Fatalf("the script names %d sessions, want 1", spec.connections)
	}
	want := []string{caseFrameEOR, caseFrameEORIPv6, caseFrameWithdraw}
	steps := spec.steps[1]
	if len(steps) != len(want) {
		t.Fatalf("session 1 holds %d steps, want %d", len(steps), len(want))
	}
	for index, frame := range want {
		if !strings.EqualFold(hex.EncodeToString(steps[index].frame), frame) {
			t.Fatalf("step %d = %X, want %s: the steps are not in sequence order", index, steps[index].frame, frame)
		}
	}

	lettered, err := readExaBGPCase(writeCase(t, "A1:raw:"+caseFrameEOR+"\nB1:raw:"+caseFrameEOR+"\n"))
	if err != nil {
		t.Fatal(err)
	}
	if lettered.connections != 2 {
		t.Fatalf("the lettered script names %d sessions, want 2", lettered.connections)
	}
}

// TestExaBGPCaseRefusesAScriptThatAssertsNothing lists the scripts that would
// pass whatever the speaker sent, and checks each is refused with an error that
// names the cause, while the explicit marker admits an action-only script.
func TestExaBGPCaseRefusesAScriptThatAssertsNothing(t *testing.T) {
	for _, tc := range []struct {
		name    string
		fixture string
		want    string
	}{
		{"no step", "option=file:x.conf\n1:cmd:announce route 10.0.0.0/24 next-hop 1.2.3.4\n", "has no step"},
		{"action only", "A1:notification:bye\n", "expects no frame"},
		{"marker beside frames", "option=frames:none\n1:raw:" + caseFrameEOR + "\n", "also expects frames"},
		{"missing session", "A1:raw:" + caseFrameEOR + "\nC1:raw:" + caseFrameEOR + "\n", "the script has steps for sessions A and C but none for B: rename C to B, or add B's steps"},
		{"unknown kind", "1:rwa:" + caseFrameEOR + "\n", "unknown step kind"},
		{"not a step", "1:raw\n", "is not a <position>:<kind>:<content> step"},
		{"no sequence", "A:raw:" + caseFrameEOR + "\n", "carries no sequence number"},
		{"short frame", "1:raw:FFFF\n", "shorter than a BGP header"},
		{"action beside frames", "1:raw:" + caseFrameEOR + "\n1:signal:SIGUSR1\n", "holds an action beside other steps"},
		{"long communication", "A1:notification:" + strings.Repeat("x", 256) + "\noption=frames:none\n", "the limit is 255"},
		{"invalid utf-8", "option=frames:none\nA1:notification:bye \xff\n", "is not valid UTF-8"},
		{"unknown option", "option=open:send-unknown-message\n1:raw:" + caseFrameEOR + "\n", "read by neither this mock nor the runner"},
		{"timeout zero", "option=timeout:0\n1:raw:" + caseFrameEOR + "\n", "not a positive number"},
		{"timeout negative", "option=timeout:-1\n1:raw:" + caseFrameEOR + "\n", "not a positive number"},
		{"timeout past the session", "option=timeout:60.5\n1:raw:" + caseFrameEOR + "\n", "longer than the 1m0s"},
		{"timeout not a number", "option=timeout:soon\n1:raw:" + caseFrameEOR + "\n", "not a number of seconds"},
		{"open without inspect", "1:raw:" + caseFrameOpen + "\n2:raw:" + caseFrameEOR + "\n", "add the option"},
		{"open after a frame", "option=open:inspect-open-message\n1:raw:" + caseFrameEOR + "\n2:raw:" + caseFrameOpen + "\n", "after other steps"},
		{"inspect without open", "option=open:inspect-open-message\n1:raw:" + caseFrameEOR + "\n", "does not open with an OPEN"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := readExaBGPCase(writeCase(t, tc.fixture))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want one containing %q", err, tc.want)
			}
		})
	}

	spec, err := readExaBGPCase(writeCase(t, "option=frames:none\nA1:notification:closing session because we can\n"))
	if err != nil {
		t.Fatalf("a stated frame-less case is refused: %v", err)
	}
	if steps := spec.steps[1]; len(steps) != 1 || steps[0].kind != exabgpStepNotification {
		t.Fatalf("session 1 = %+v, want one notification step", steps)
	}
}

// TestExaBGPCaseRefusesAFrameAGroupEarly drives serveExaBGPConnection over a
// pipe with a speaker that sends group 2's frame before group 1's, then with
// one that closes before its last frame, and checks neither run succeeds. A
// third speaker sends both in order and must pass, so the refusals are not a
// mock that refuses everything.
func TestExaBGPCaseRefusesAFrameAGroupEarly(t *testing.T) {
	spec, err := readExaBGPCase(writeCase(t, "1:raw:"+caseFrameEOR+"\n2:raw:"+caseFrameWithdraw+"\n"))
	if err != nil {
		t.Fatal(err)
	}
	first, _ := hex.DecodeString(caseFrameEOR)       //nolint:errcheck // a constant of this file
	second, _ := hex.DecodeString(caseFrameWithdraw) //nolint:errcheck // a constant of this file
	for _, tc := range []struct {
		name   string
		frames [][]byte
		passes bool
	}{
		{"out of order", [][]byte{second, first}, false},
		{"closes early", [][]byte{first}, false},
		{"in order", [][]byte{first, second}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := serveCaseOverPipe(t, spec, tc.frames)
			if tc.passes && err != nil {
				t.Fatalf("an in-order speaker failed: %v", err)
			}
			if !tc.passes && err == nil {
				t.Fatal("the run succeeded although the speaker did not send the script in order")
			}
		})
	}
}

// TestExaBGPCaseReplaysOnRouteRefresh checks the mock answers a matched
// ROUTE-REFRESH by sending back the UPDATE the speaker sent before it, as
// upstream's mock does. api-rr cannot complete without it.
func TestExaBGPCaseReplaysOnRouteRefresh(t *testing.T) {
	const refresh = "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF00170500010001"
	spec, err := readExaBGPCase(writeCase(t, "1:raw:"+caseFrameWithdraw+"\n2:raw:"+refresh+"\n"))
	if err != nil {
		t.Fatal(err)
	}
	update, _ := hex.DecodeString(caseFrameWithdraw) //nolint:errcheck // a constant of this file
	request, _ := hex.DecodeString(refresh)          //nolint:errcheck // a constant of this file
	mock, speaker := net.Pipe()
	defer func() { _ = mock.Close() }()
	received := make(chan []byte, 1)
	go readCaseReplay(speaker, [][]byte{update, request}, received)
	if err := serveExaBGPConnection(mock, spec, 1, nil, io.Discard); err != nil {
		t.Fatalf("the run failed: %v", err)
	}
	select {
	case got := <-received:
		if !bytes.Equal(got, update) {
			t.Fatalf("replayed %X, want the speaker's UPDATE %X", got, update)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the mock sent no UPDATE back after the ROUTE-REFRESH")
	}
}

// readCaseReplay plays the speaker, then reads what the mock writes after the
// OPEN and KEEPALIVE and hands back the first UPDATE.
func readCaseReplay(speaker net.Conn, frames [][]byte, received chan<- []byte) {
	defer func() { _ = speaker.Close() }()
	writeErr := make(chan error, 1)
	go writeCaseFrames(speaker, frames, writeErr)
	for {
		messageType, body, err := readBGPWireMessage(speaker)
		if err != nil {
			return
		}
		if messageType == bgpUpdate {
			received <- speakerMessage(messageType, body)
			// The mock still writes its KEEPALIVE, so keep reading until it
			// closes rather than closing under its write.
			drainCaseSpeaker(speaker)
			return
		}
	}
}

func writeCaseFrames(speaker net.Conn, frames [][]byte, done chan<- error) {
	open := speakerMessage(bgpOpen, []byte{4, 0xfd, 0xe9, 0, 90, 1, 1, 1, 1, 0})
	for _, frame := range append([][]byte{open}, frames...) {
		if _, err := speaker.Write(frame); err != nil {
			done <- err
			return
		}
	}
	done <- nil
}

// serveCaseOverPipe plays a speaker on one end of a pipe: it sends an OPEN,
// drains what the mock writes, sends the frames, and closes.
func serveCaseOverPipe(t *testing.T, spec exabgpCase, frames [][]byte) error {
	t.Helper()
	mock, speaker := net.Pipe()
	defer func() { _ = mock.Close() }()
	done := make(chan struct{})
	go playCaseSpeaker(speaker, frames, done)
	_ = mock.SetDeadline(time.Now().Add(5 * time.Second))
	err := serveExaBGPConnection(mock, spec, 1, nil, io.Discard)
	_ = mock.Close()
	<-done
	return err
}

func playCaseSpeaker(speaker net.Conn, frames [][]byte, done chan<- struct{}) {
	defer close(done)
	defer func() { _ = speaker.Close() }()
	// Version 4, AS 65001, hold time 90, router id 1.1.1.1, no parameters.
	open := speakerMessage(bgpOpen, []byte{4, 0xfd, 0xe9, 0, 90, 1, 1, 1, 1, 0})
	go drainCaseSpeaker(speaker)
	if _, err := speaker.Write(open); err != nil {
		return
	}
	for _, frame := range frames {
		if _, err := speaker.Write(bytes.Clone(frame)); err != nil {
			return
		}
	}
}

func drainCaseSpeaker(speaker net.Conn) {
	_, _ = io.Copy(io.Discard, speaker) //nolint:errcheck // the pipe closing ends the drain
}

// TestExaBGPShutdownNotificationBytes pins the Cease NOTIFICATION a
// notification step sends, octet by octet, against RFC 9003 Section 2: the
// header, type 3, code 6, subcode 2, the length, then the text.
func TestExaBGPShutdownNotificationBytes(t *testing.T) {
	want, _ := hex.DecodeString("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF" + "0019" + "03" + "06" + "02" + "03" + "627965") //nolint:errcheck // a constant of this test
	if got := exabgpShutdownNotification("bye"); !bytes.Equal(got, want) {
		t.Fatalf("notification = %X, want %X", got, want)
	}
}

// TestExaBGPCaseNotificationTextIsUpstreams checks a notification step sends
// what upstream's mock sends for the same line: the text after the last colon,
// lowercased.
func TestExaBGPCaseNotificationTextIsUpstreams(t *testing.T) {
	spec, err := readExaBGPCase(writeCase(t, "option=frames:none\nA1:notification:reason:Closing Now\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got := spec.steps[1][0].notification; got != "closing now" {
		t.Fatalf("notification text = %q, want %q", got, "closing now")
	}
}

// TestExaBGPEndOfRIBForms checks both RFC 4724 Section 2 forms of an
// End-of-RIB, and the IPv6 one in both attribute length encodings, are taken
// for no route, so neither a json line nor a ROUTE-REFRESH replay attaches to
// one, while a withdrawal is still a route.
func TestExaBGPEndOfRIBForms(t *testing.T) {
	for _, tc := range []struct {
		name  string
		frame string
		route bool
	}{
		{"ipv4 end-of-rib", caseFrameEOR, false},
		{"mp end-of-rib, extended length", caseFrameEORIPv6, false},
		{"mp end-of-rib, one-octet length", caseFrameEORIPv6Short, false},
		{"withdrawal", caseFrameWithdraw, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wire, _ := hex.DecodeString(tc.frame) //nolint:errcheck // a constant of this file
			if got := documentableFrame(wire); got != tc.route {
				t.Fatalf("documentableFrame(%s) = %v, want %v", tc.frame, got, tc.route)
			}
		})
	}
}

// TestExaBGPCaseSendsTheUnknownCapability runs a session under
// `option=open:send-unknown-capability` and reads the mock's OPEN: it must end
// with a Capabilities parameter carrying code 66, as upstream's _add_capa66
// writes it. The same case without the option must carry no parameter, so the
// check is not one any OPEN passes.
func TestExaBGPCaseSendsTheUnknownCapability(t *testing.T) {
	parameter, _ := hex.DecodeString("020C420A" + hex.EncodeToString([]byte("loremipsum"))) //nolint:errcheck // a constant of this test
	for _, tc := range []struct {
		name    string
		fixture string
		want    []byte
	}{
		{"stated", "option=open:send-unknown-capability\n1:raw:" + caseFrameEOR + "\n", parameter},
		{"not stated", "1:raw:" + caseFrameEOR + "\n", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			spec, err := readExaBGPCase(writeCase(t, tc.fixture))
			if err != nil {
				t.Fatal(err)
			}
			eor, _ := hex.DecodeString(caseFrameEOR) //nolint:errcheck // a constant of this file
			mock, speaker := net.Pipe()
			defer func() { _ = mock.Close() }()
			opens := make(chan []byte, 1)
			go playCapabilitySpeaker(speaker, eor, opens)
			if err := serveExaBGPConnection(mock, spec, 1, nil, io.Discard); err != nil {
				t.Fatalf("the run failed: %v", err)
			}
			body := <-opens
			if len(body) < 10 || int(body[9]) != len(tc.want) || !bytes.Equal(body[10:], tc.want) {
				t.Fatalf("mock OPEN body = %X, want optional parameters %X", body, tc.want)
			}
		})
	}
}

// TestExaBGPUnknownCapabilityRefusesAWrappingLength checks capability 66 is
// added only while the one-octet Optional Parameters Length stays below 255,
// the value RFC 9072 Section 2 reserves for the extended form: 240 octets plus
// the 14-octet parameter is 254 and accepted, 241 reaches 255 and is refused.
func TestExaBGPUnknownCapabilityRefusesAWrappingLength(t *testing.T) {
	for _, tc := range []struct {
		existing int
		refused  bool
	}{
		{0, false},
		{240, false},
		{241, true},
		{255, true},
	} {
		body := append([]byte{4, 0xfd, 0xe9, 0, 90, 1, 1, 1, 1, byte(tc.existing)}, make([]byte, tc.existing)...)
		rewritten, err := addExaBGPUnknownCapability(body)
		if tc.refused {
			if err == nil || !strings.Contains(err.Error(), "past the one-octet length") {
				t.Errorf("existing %d: err = %v, want the one-octet length refusal", tc.existing, err)
			}
			continue
		}
		if err != nil {
			t.Errorf("existing %d: %v", tc.existing, err)
			continue
		}
		if got := int(rewritten[9]); got != tc.existing+14 || len(rewritten) != 10+got {
			t.Errorf("existing %d: length octet %d over %d parameter octets, want %d", tc.existing, got, len(rewritten)-10, tc.existing+14)
		}
	}
}

// playCapabilitySpeaker sends an OPEN, hands back the body of the OPEN the mock
// answers with, then sends one frame and closes.
func playCapabilitySpeaker(speaker net.Conn, frame []byte, opens chan<- []byte) {
	defer func() { _ = speaker.Close() }()
	if _, err := speaker.Write(speakerMessage(bgpOpen, []byte{4, 0xfd, 0xe9, 0, 90, 1, 1, 1, 1, 0})); err != nil {
		opens <- nil
		return
	}
	_, body, err := readBGPWireMessage(speaker)
	opens <- body
	if err != nil {
		return
	}
	go drainCaseSpeaker(speaker)
	_, _ = speaker.Write(frame) //nolint:errcheck // the mock's verdict is what the test reads
}

// TestExaBGPCaseNamesTheStalledFrame checks a session that ends before a group
// completes fails with the sequence and the frame it still owed, not only the
// socket's own error.
func TestExaBGPCaseNamesTheStalledFrame(t *testing.T) {
	spec, err := readExaBGPCase(writeCase(t, "1:raw:"+caseFrameEOR+"\n2:raw:"+caseFrameWithdraw+"\n"))
	if err != nil {
		t.Fatal(err)
	}
	first, _ := hex.DecodeString(caseFrameEOR) //nolint:errcheck // a constant of this file
	err = serveCaseOverPipe(t, spec, [][]byte{first})
	want := "sequence 2: 1 expected frame(s) never arrived (first: " + caseFrameWithdraw + ")"
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("error = %v, want one containing %q", err, want)
	}
}

// TestExaBGPCaseMessageTimeout checks `option=timeout:` fails a session whose
// speaker stays connected and sends nothing, on the stated limit rather than
// the session's minute.
func TestExaBGPCaseMessageTimeout(t *testing.T) {
	spec, err := readExaBGPCase(writeCase(t, "option=timeout:0.2\n1:raw:"+caseFrameEOR+"\n"))
	if err != nil {
		t.Fatal(err)
	}
	mock, speaker := net.Pipe()
	defer func() { _ = speaker.Close() }()
	go playSilentSpeaker(speaker)
	start := time.Now()
	err = serveExaBGPConnection(mock, spec, 1, nil, io.Discard)
	_ = mock.Close()
	if !errors.Is(err, os.ErrDeadlineExceeded) {
		t.Fatalf("error = %v, want the per-message deadline", err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("the session failed after %s, want about the 200ms the case states", elapsed)
	}
}

// playSilentSpeaker sends an OPEN and then nothing, reading what the mock sends
// until the pipe closes.
func playSilentSpeaker(speaker net.Conn) {
	go drainCaseSpeaker(speaker)
	_, _ = speaker.Write(speakerMessage(bgpOpen, []byte{4, 0xfd, 0xe9, 0, 90, 1, 1, 1, 1, 0})) //nolint:errcheck // the mock's verdict is what the test reads
}

// TestExaBGPCaseReadsEveryFixture reads every compatibility case on disk,
// untracked ones included because the glob reads the directory, so a
// fixture the reader would refuse fails here rather than as a mock that exits
// before listening, and checks conf-unknowncap reaches the mock as the option
// that makes its OPEN carry capability 66.
func TestExaBGPCaseReadsEveryFixture(t *testing.T) {
	root := filepath.Join("..", "..", "..", "..", "test", "exabgp-compat")
	paths, err := filepath.Glob(filepath.Join(root, "*", "*.ci"))
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) < 80 {
		t.Fatalf("found %d fixtures under %s: the glob did not reach the case directories", len(paths), root)
	}
	for _, path := range paths {
		spec, err := readExaBGPCase(path)
		if err != nil {
			t.Errorf("%s: %v", path, err)
			continue
		}
		if filepath.Base(path) == "conf-unknowncap.ci" && !spec.sendUnknownCapability {
			t.Errorf("%s states option=open:send-unknown-capability and the case does not carry it", path)
		}
	}
}
