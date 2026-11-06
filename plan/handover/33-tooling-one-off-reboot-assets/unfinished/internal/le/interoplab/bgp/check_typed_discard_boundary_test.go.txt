// Design: docs/architecture/testing/interop.md -- exact retained wire evidence.
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

func TestSpeakerFramingRejectsInvalidOuterHeader(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func([]byte)
	}{
		{"marker", func(frame []byte) { frame[0] = 0 }},
		{"short length", func(frame []byte) { frame[16], frame[17] = 0, 18 }},
		{"KEEPALIVE body", func(frame []byte) { frame[18] = bgpKeepalive }},
	} {
		t.Run(test.name, func(t *testing.T) {
			frame := speakerMessage(bgpUpdate, []byte{0, 0, 0, 0})
			test.mutate(frame)
			conn := &scriptedSpeakerConn{reads: []scriptedSpeakerRead{{data: frame[:19]}, {data: frame[19:]}}}
			if _, _, _, err := readSpeakerMessage(conn, time.Now().Add(time.Second)); err == nil {
				t.Fatal("invalid BGP outer header passed")
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
