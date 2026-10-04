// Design: docs/architecture/mrt.md — real command errors and unsupported replay.
package analyze

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/mrt"
)

// framedUpdate is a test fixture, not a raw-byte parser compatibility API.
func framedUpdate(body []byte) mrt.BGPMessage {
	wire := bytes.Repeat([]byte{255}, 16)
	wire = binary.BigEndian.AppendUint16(wire, uint16(19+len(body)))
	wire = append(wire, 2)
	return mrt.BGPMessage{Bytes: append(wire, body...)}
}

func commandOpenPrelude(subtype uint16, mode4, mode6 byte) []byte {
	var records []byte
	for _, sent := range []bool{false, true} {
		asn := byte(0xe8)
		base := uint16(1)
		fields := []byte{0xfd, 0xe8, 0xfd, 0xe9}
		if mrt.IsAS4Subtype(subtype) {
			base = 4
			fields = []byte{0, 0, 0xfd, 0xe8, 0, 0, 0xfd, 0xe9}
		}
		if sent {
			asn = 0xe9
			if base == 4 {
				base = 7
			} else {
				base = 6
			}
		}
		fields = append(fields, 0, 0, 0, 1, 192, 0, 2, 1, 192, 0, 2, 2)
		caps := []byte{1, 4, 0, 1, 0, 1, 1, 4, 0, 2, 0, 1}
		var tuples []byte
		if mode4 != 0 {
			tuples = append(tuples, 0, 1, 1, mode4)
		}
		if mode6 != 0 {
			tuples = append(tuples, 0, 2, 1, mode6)
		}
		if len(tuples) != 0 {
			caps = append(caps, 69, byte(len(tuples)))
			caps = append(caps, tuples...)
		}
		body := []byte{4, 0xfd, asn, 0, 90, 192, 0, 2, asn, byte(len(caps) + 2), 2, byte(len(caps))}
		body = append(body, caps...)
		wire := framedUpdate(body).Bytes
		wire[18] = 1
		records = append(records, mrtRecord(16, base, append(fields, wire...))...)
	}
	return records
}

func mixedCommandRecord(classicAP bool) []byte {
	ann4, wd4 := []byte{24, 10, 0, 0}, []byte{24, 10, 1, 0}
	ann6, wd6 := []byte{32, 0x20, 1, 0x0d, 0xb8}, []byte{32, 0x20, 1, 0x0d, 0xb9}
	if classicAP {
		ann4 = append([]byte{0, 0, 0, 1}, ann4...)
		wd4 = append([]byte{0, 0, 0, 2}, wd4...)
	} else {
		ann6 = append([]byte{0, 0, 0, 1}, ann6...)
		wd6 = append([]byte{0, 0, 0, 2}, wd6...)
	}
	attrs := []byte{0x40, 1, 1, 0, 0x40, 2, 6, 2, 1, 0, 0, 0xfd, 0xe8, 0x40, 3, 4, 192, 0, 2, 1}
	for _, attr := range []mrt.PathAttribute{mpReachAttr(net.ParseIP("2001:db8::1").To16(), ann6), mpUnreachAttr(wd6)} {
		attrs = append(attrs, 0x80, attr.Code, byte(len(attr.Value)))
		attrs = append(attrs, attr.Value...)
	}
	fields := []byte{0, 0, 0xfd, 0xe8, 0, 0, 0xfd, 0xe9, 0, 0, 0, 1, 192, 0, 2, 1, 192, 0, 2, 2}
	return mrtRecord(16, 9, append(fields, framedUpdate(buildUpdate(wd4, attrs, ann4)).Bytes...))
}

// TestRFC8050CommandsRequireMixedContext drives the actual stdin command rails.
// RFC requirement: RFC8050-x-4 positive -- actual directional OPENs let show and density count two distinct announcements and two withdrawals in both mixed classic/MP mode arrangements.
// RFC requirement: RFC8050-x-4 negative -- show, density and content filter report a nonzero error for the identical mixed UPDATE without both OPENs; filtering never reports undecodable bytes as a nonmatch.
func TestRFC8050CommandsRequireMixedContext(t *testing.T) {
	for _, classicAP := range []bool{false, true} {
		mode4, mode6 := byte(0), byte(3)
		if classicAP {
			mode4, mode6 = 3, 0
		}
		update := mixedCommandRecord(classicAP)
		wire := append(commandOpenPrelude(9, mode4, mode6), update...)
		code, out, stderr := runSubcommandCapturing(t, wire, func() int { return runShow([]string{"-"}) })
		if code != 0 || !strings.Contains(out, "W=2 A=2") {
			t.Fatalf("mixed show=%d %q %q", code, out, stderr)
		}
		code, out, stderr = runSubcommandCapturing(t, wire, func() int { return runDensity([]string{"-"}) })
		if code != 0 || !strings.Contains(out, "Total announced NLRIs: 2") || !strings.Contains(out, "Total withdrawn NLRIs: 2") {
			t.Fatalf("mixed density=%d %q %q", code, out, stderr)
		}
		outputPath := filepath.Join(t.TempDir(), "filtered.mrt")
		code, _, stderr = runSubcommandCapturing(t, wire, func() int { return runFilter([]string{"--as-path", "65000", "-", outputPath}) })
		if code != 0 {
			t.Fatalf("mixed filter=%d %q", code, stderr)
		}
		filtered, err := os.ReadFile(outputPath)
		if err != nil {
			t.Fatal(err)
		}
		code, out, stderr = runSubcommandCapturing(t, filtered, func() int { return runShow([]string{"-"}) })
		if code != 0 || !strings.Contains(out, "W=2 A=2") {
			t.Fatalf("filtered context lost=%d %q %q", code, out, stderr)
		}
		for _, command := range []func() int{
			func() int { return runShow([]string{"-"}) },
			func() int { return runDensity([]string{"-"}) },
			func() int { return runFilter([]string{"--as-path", "65000", "-", outputPath}) },
			func() int { return runASPath([]string{"-"}) },
			func() int { return runAttributes([]string{"-"}) },
			func() int { return runCommunities([]string{"-"}) },
		} {
			code, out, stderr = runSubcommandCapturing(t, update, command)
			if code == 0 || !strings.Contains(out+stderr, "context unavailable") {
				t.Fatalf("missing context=%d %q %q", code, out, stderr)
			}
		}
		err = mrt.ReadFrom(bytes.NewReader(update), &mrt.Handler{OnMessage: func(_ mrt.Header, _ uint32, rec *mrt.MessageRecord) error {
			_, err := matchMessageContent(rec, true, &filterOpts{})
			return err
		}})
		if !errors.Is(err, mrt.ErrContextUnavailable) {
			t.Fatalf("filter swallowed context error: %v", err)
		}
	}
}

// TestMRTServeRefusesAddPathBeforeWrite reaches the actual file-to-socket path.
// The ordinary UPDATE and empty EOR controls must still write complete packets.
func TestMRTServeRefusesAddPathBeforeWrite(t *testing.T) {
	for _, tc := range []struct {
		name   string
		body   []byte
		ap     bool
		refuse bool
	}{
		{"path-id", []byte{0, 0, 0, 0, 0, 0, 0, 1, 24, 10, 0, 0}, true, true},
		{"ordinary", []byte{0, 0, 0, 0, 24, 10, 0, 0}, false, false},
		{"empty-eor", []byte{0, 0, 0, 0}, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			record := makeBGP4MPRecord(4, 65000, 1, tc.body)
			subtype := uint16(4)
			if tc.ap {
				subtype = 9
			}
			path := filepath.Join(t.TempDir(), "replay.mrt")
			if err := os.WriteFile(path, mrtRecord(16, subtype, record), 0o600); err != nil {
				t.Fatal(err)
			}
			server, client := net.Pipe()
			defer closeMRTTestSocket(t, server)
			defer closeMRTTestSocket(t, client)
			if err := server.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
				t.Fatal(err)
			}
			if err := client.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
				t.Fatal(err)
			}
			type result struct {
				n   uint64
				err error
			}
			done := make(chan result, 1)
			go func() {
				n, err := serveFile(server, path, 0, false)
				closeMRTTestSocket(t, server)
				done <- result{n, err}
			}()
			var received bytes.Buffer
			_, readErr := received.ReadFrom(client)
			got := <-done
			if readErr != nil {
				t.Fatal(readErr)
			}
			if tc.refuse {
				if !errors.Is(got.err, errReplayAddPath) || got.n != 0 || received.Len() != 0 {
					t.Fatalf("refusal=%+v bytes=%x", got, received.Bytes())
				}
			} else if got.err != nil || readErr != nil || got.n != 1 || !bytes.Equal(received.Bytes(), framedUpdate(tc.body).Bytes) {
				t.Fatalf("ordinary/EOR=%+v read=%v bytes=%x", got, readErr, received.Bytes())
			}
		})
	}
}

// TestMRTReplayAndInjectRefuseBeforeUpdate drives both CLI socket entry points,
// rather than accepting a parser/helper-only refusal.
func TestMRTReplayAndInjectRefuseBeforeUpdate(t *testing.T) {
	for _, command := range []struct {
		name string
		run  func([]string) int
	}{
		{"inject", runInject}, {"replay", runReplay},
	} {
		for _, tc := range []struct {
			name    string
			record  []byte
			refused bool
		}{
			{"path-id", mrtRecord(16, 9, makeBGP4MPRecord(4, 65000, 1, []byte{0, 0, 0, 0, 0, 0, 0, 1, 24, 10, 0, 0})), true},
			{"missing-context", mixedCommandRecord(false), true},
			{"ordinary", mrtRecord(16, 4, makeBGP4MPRecord(4, 65000, 1, []byte{0, 0, 0, 0, 24, 10, 0, 0})), false},
			{"empty-eor", mrtRecord(16, 9, makeBGP4MPRecord(4, 65000, 1, []byte{0, 0, 0, 0})), false},
		} {
			t.Run(command.name+"/"+tc.name, func(t *testing.T) {
				listener, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
				if err != nil {
					t.Fatal(err)
				}
				defer closeMRTTestSocket(t, listener)
				type observation struct {
					updates int
					err     error
				}
				done := make(chan observation, 1)
				go func() {
					var seen observation
					defer func() { done <- seen }()
					conn, err := listener.Accept()
					if err != nil {
						seen.err = err
						return
					}
					defer closeMRTTestSocket(t, conn)
					if err := conn.SetDeadline(time.Now().Add(5 * time.Second)); err != nil {
						seen.err = err
						return
					}
					if _, err = serveBGPOpen(conn, 65001, net.ParseIP("192.0.2.2"), 90); err != nil {
						seen.err = err
						return
					}
					for {
						kind, _, err := bgpReadMsg(conn)
						if err != nil {
							if !errors.Is(err, io.EOF) {
								seen.err = err
							}
							return
						}
						if kind == 2 {
							seen.updates++
						}
					}
				}()
				code, _, stderr := runSubcommandCapturing(t, tc.record, func() int { return command.run([]string{"-", listener.Addr().String()}) })
				closeMRTTestSocket(t, listener)
				seen := <-done
				if seen.err != nil {
					t.Fatal(seen.err)
				}
				if tc.refused {
					if code == 0 || seen.updates != 0 || (!strings.Contains(stderr, "ADD-PATH") && !strings.Contains(stderr, "context unavailable")) {
						t.Fatalf("refusal code=%d updates=%d stderr=%q", code, seen.updates, stderr)
					}
				} else if code != 0 || seen.updates != 1 {
					t.Fatalf("ordinary/EOR code=%d updates=%d stderr=%q", code, seen.updates, stderr)
				}
			})
		}
	}
}
