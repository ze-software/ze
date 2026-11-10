// Design: docs/architecture/testing/interop.md -- exact retained wire evidence.
// Related: check_typed_discard_wire.go -- the wire check these tests drive.
//
// Every test here expects requireTypedEVPNWire to accept the retained route,
// which only the ze_bgp build can decode, so this file carries the same gate.

//go:build ze_bgp

package bgp

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// This is the complete assigned NLRI in the tracked injector fixture, including
// its type/length octets: RD 172.30.0.9:1, zero ESI/tag, MAC 00:11:22:33:44:55,
// no IP, and label 2 (bottom-of-stack set).
const typedDiscardRetainedHex = "02210001ac1e0009000100000000000000000000000000003000112233445500000021"

func typedDiscardBoundaryBody(t *testing.T) []byte {
	t.Helper()
	nlri, err := hex.DecodeString(typedDiscardRetainedHex)
	if err != nil {
		t.Fatal(err)
	}
	value := append([]byte{0, 25, 70, 4, 172, 30, 0, 9, 0}, nlri...)
	attrs := append([]byte{0x80, mpReach, byte(len(value))}, value...)
	return buildSpeakerUpdate(attrs, nil)
}

func TestTypedDiscardRequiresExactCompleteWireWitness(t *testing.T) {
	body := typedDiscardBoundaryBody(t)
	report := func(body []byte, extra string) string {
		return "result: PASS\nplugin: no-unrecognized-evpn-type\nnote: established: yes\nnote: ze-open: yes\nnote: capture-complete: yes\nnote: route-bearing-updates: 1\nnote: evpn-nlri: 1\n" +
			fmt.Sprintf("note: update-hex: %x\n", body) + extra
	}
	good := report(body, "")
	if err := requireSpeakerEVPNDiscard(good); err != nil {
		t.Fatal(err)
	}
	wrongMAC := bytes.Clone(body)
	wrongMAC[len(wrongMAC)-5] ^= 1
	for _, test := range []struct{ name, report string }{
		{"wrong retained type2", report(wrongMAC, "")},
		{"missing wire evidence", strings.ReplaceAll(good, fmt.Sprintf("note: update-hex: %x\n", body), "")},
		{"missing OPEN", strings.ReplaceAll(good, "note: ze-open: yes\n", "")},
		{"unfinished capture", strings.ReplaceAll(good, "capture-complete: yes", "capture-complete: no")},
		{"truncated attribute", report(body[:len(body)-1], "")},
		{"trailing attribute byte", report(append(bytes.Clone(body), 0xff), "")},
		{"extra assigned route", report(body, fmt.Sprintf("note: update-hex: %x\n", body))},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := requireSpeakerEVPNDiscard(test.report); err == nil {
				t.Fatal("inexact or incomplete retained-route witness passed")
			}
		})
	}
}

func TestTypedNativeSpeakerRejectsMissingHandshakeAndEarlyEOF(t *testing.T) {
	body := typedDiscardBoundaryBody(t)
	open, err := speakerOpen(65001, 90, net.ParseIP("172.30.0.2"), []bgpFamily{{25, 70}}, false, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name       string
		frames     [][]byte
		closeEarly bool
	}{
		{"complete", [][]byte{open, speakerKeepalive(), speakerMessage(bgpUpdate, body)}, false},
		{"missing OPEN", [][]byte{speakerKeepalive(), speakerMessage(bgpUpdate, body)}, false},
		{"missing KEEPALIVE", [][]byte{open, speakerMessage(bgpUpdate, body)}, false},
		{"early EOF", [][]byte{open, speakerKeepalive(), speakerMessage(bgpUpdate, body)}, true},
		{"truncated attributes", [][]byte{open, speakerKeepalive(), speakerMessage(bgpUpdate, body[:len(body)-1])}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			done := make(chan error, 1)
			go func() {
				connection, err := listener.Accept()
				if err != nil {
					done <- err
					return
				}
				defer connection.Close()
				_ = connection.SetDeadline(time.Now().Add(3 * time.Second))
				var frame [4096]byte
				if _, err := readExtendedFrame(connection, frame[:]); err != nil {
					done <- err
					return
				}
				for _, frame := range test.frames {
					if _, err := connection.Write(frame); err != nil {
						done <- err
						return
					}
				}
				if !test.closeEarly {
					_, _ = io.Copy(io.Discard, connection)
				}
				done <- nil
			}()
			var output bytes.Buffer
			_ = runOracleSpeaker(speakerOptions{connect: listener.Addr().String(), asn: 65002, routerID: "172.30.0.8", test: speakerOracleNoUnrecognizedEVPNType, holdTime: 90, duration: 200 * time.Millisecond, families: []bgpFamily{{25, 70}}}, &output)
			if err := <-done; err != nil {
				t.Fatal(err)
			}
			err = requireSpeakerEVPNDiscard(output.String())
			if test.name == "complete" {
				if err != nil {
					t.Fatalf("complete capture rejected: %v\n%s", err, output.String())
				}
			} else if err == nil {
				t.Fatalf("incomplete native capture passed:\n%s", output.String())
			}
		})
	}
}

// TestRFC7606TypedNLRIDiscardCheckerBranches drives both polarities of every
// reading checkRFC7606TypedNLRIDiscard makes of the speaker's log, and then the
// checker body itself, with no container.
func TestRFC7606TypedNLRIDiscardCheckerBranches(t *testing.T) {
	report := func(lines ...string) string {
		return strings.Join(append(lines, "note: ze-open: yes", "note: capture-complete: yes",
			fmt.Sprintf("note: update-hex: %x", typedDiscardBoundaryBody(t))), "\n") + "\n"
	}
	const (
		pass        = "result: PASS"
		oracle      = "plugin: no-unrecognized-evpn-type"
		established = "note: established: yes"
		updates     = "note: route-bearing-updates: 1"
		evpn        = "note: evpn-nlri: 1"
	)
	relayed := report(pass, oracle, updates, established, evpn)
	if !speakerReportPrinted(relayed) {
		t.Fatal("a finished verdict was read as an unfinished run")
	}
	if speakerReportPrinted(report("waiting for the relayed EVPN routes")) {
		t.Fatal("a log without a verdict line was read as a finished run")
	}
	if err := requireSpeakerEVPNDiscard(relayed); err != nil {
		t.Fatalf("a verdict naming only the assigned EVPN route type was rejected: %v", err)
	}
	unassigned := report(
		"result: FAIL",
		oracle,
		"fail: RFC 7606 Section 5.4: received EVPN route type 99, which is not assigned; it must have been discarded, not relayed",
		updates, established, evpn,
	)
	if requireSpeakerEVPNDiscard(unassigned) == nil {
		t.Fatal("an unassigned EVPN route type that reached the speaker passed")
	}
	if requireSpeakerEVPNDiscard(report(pass, oracle, updates, "note: established: no", evpn)) == nil {
		t.Fatal("a session that never came up passed as proof of a discard")
	}
	if requireSpeakerEVPNDiscard(report(pass, oracle, updates, "note: not-established: yes", evpn)) == nil {
		t.Fatal("a note naming not-established answered for established")
	}
	if requireSpeakerEVPNDiscard(report(pass, oracle, updates, established, "note: evpn-nlri: 0")) == nil {
		t.Fatal("a relay that delivered no EVPN NLRI passed as a discard")
	}
	if requireSpeakerEVPNDiscard(report(pass, oracle, "note: route-bearing-updates: 0", established, evpn)) == nil {
		t.Fatal("a session that carried no route-bearing UPDATE passed as a discard")
	}
	unmeasured := requireSpeakerEVPNDiscard(report(pass, oracle, updates, established))
	measuredZero := requireSpeakerEVPNDiscard(report(pass, oracle, updates, established, "note: evpn-nlri: 0"))
	if unmeasured == nil || measuredZero == nil {
		t.Fatalf("an unmeasured EVPN count passed: unmeasured=%v zero=%v", unmeasured, measuredZero)
	}
	if unmeasured.Error() == measuredZero.Error() {
		t.Fatalf("a count the speaker never took reported the same failure as a measured zero: %v", unmeasured)
	}
	if requireSpeakerEVPNDiscard(report(pass, oracle, updates, established, "note: evpn-nlri: many")) == nil {
		t.Fatal("an EVPN count that is not a number passed as a count")
	}
	if requireSpeakerEVPNDiscard(report(pass, "plugin: no-duplicate-attribute", updates, established, evpn)) == nil {
		t.Fatal("another oracle's verdict answered for this requirement")
	}
	if requireSpeakerEVPNDiscard(report(oracle, updates, established, evpn)) == nil {
		t.Fatal("notes with no verdict line passed as a PASS")
	}
	if requireSpeakerEVPNDiscard("") == nil {
		t.Fatal("an unread speaker log passed as a discard")
	}
	// The body itself, driven with a lab that answers the speaker's log and
	// nothing else. It reaches the verdict with no container, so both polarities
	// of the whole checker run here rather than only the predicate's.
	if err := checkRFC7606TypedNLRIDiscard(t.Context(), &interoplab.CheckContext{Lab: &recordingLab{logs: relayed}}); err != nil {
		t.Fatalf("the checker rejected a verdict naming only the assigned EVPN route type: %v", err)
	}
	if err := checkRFC7606TypedNLRIDiscard(t.Context(), &interoplab.CheckContext{Lab: &recordingLab{logs: unassigned}}); err == nil {
		t.Fatal("the checker passed a verdict reporting an unassigned EVPN route type")
	}
}
