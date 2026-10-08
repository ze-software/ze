// Design: docs/architecture/wire/messages.md -- per-message receive validation.
// RFC: rfc/short/rfc7606.md -- Sections 3(j), 5.3 and 6.
package reactor

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/netip"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// receiveBoundaryReply drains notifications until the fixture closes its session.
func receiveBoundaryReply(conn net.Conn, reply chan<- []byte) {
	data, _ := io.ReadAll(conn) // The fixture deliberately closes the pipe.
	reply <- data
}

// receiveBoundaryFrames retains real session negotiation and notification transport,
// but places the entire burst in the reader deterministically, without socket timing.
func receiveBoundaryFrames(t *testing.T, s *Session, coalesced bool, frames ...[]byte) error {
	t.Helper()
	stream := bytes.Join(frames, nil)
	reader := bufio.NewReaderSize(bytes.NewReader(stream), 65536)
	for range frames {
		var err error
		if coalesced {
			// RFC 7606 Sections 3(j), 5.3 and 6: preserve each original UPDATE boundary.
			err = s.readAndProcessCoalesced(s.Conn(), reader)
		} else {
			// RFC 7606 Sections 3(j), 5.3 and 6: ordinary reader control.
			err = s.readAndProcessMessage(s.Conn(), reader)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func receiveBoundaryAttrs(extended bool) []byte {
	attrs := bytes.Clone(fatalLengthAnnouncement().PathAttributes)
	if extended {
		// Optional transitive, already Partial, with a complete extended length.
		attrs = append(attrs, 0xf0, 99, 0x10, 0x00)
		attrs = append(attrs, make([]byte, 4096)...)
	}
	return attrs
}

// receiveBoundaryDiagnosticSession negotiates both tested IP families on the
// actual OPEN exchange, so a valid IPv6 diagnostic control reaches dispatch.
// The caller MUST run the returned cleanup to stop timers and close both endpoints.
func receiveBoundaryDiagnosticSession(t *testing.T, extended bool) (*Session, net.Conn, *dispatchCapture, func()) {
	t.Helper()
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
	settings.Capabilities = []capability.Capability{
		&capability.ASN4{ASN: 65001},
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
		&capability.Multiprotocol{AFI: capability.AFIIPv6, SAFI: capability.SAFIUnicast},
	}
	peerCaps := []capability.Capability{
		&capability.ASN4{ASN: 65002},
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
		&capability.Multiprotocol{AFI: capability.AFIIPv6, SAFI: capability.SAFIUnicast},
	}
	if extended {
		settings.Capabilities = append(settings.Capabilities, &capability.ExtendedMessage{})
		peerCaps = append(peerCaps, &capability.ExtendedMessage{})
	}
	s, client := firstASSession(t, settings, peerCaps, nil)
	s.nextHopScope.Store(&receiveNextHopScope{
		local: netip.MustParseAddr("192.0.2.254"), remote: settings.Address,
	})
	capture := &dispatchCapture{}
	s.SetMessageCallback(func(_ netip.Addr, kind msgtype.MessageType, _ []byte,
		wu *wireu.WireUpdate, _ bgpctx.ContextID, direction rpc.MessageDirection,
		_ BufHandle, _ map[string]any, _ string, _ uint64,
	) bool {
		if kind == msgtype.TypeUPDATE && direction == rpc.DirectionReceived {
			capture.mu.Lock()
			capture.payloads = append(capture.payloads, bytes.Clone(wu.Payload()))
			capture.mu.Unlock()
		}
		return false
	})
	conn := s.Conn()
	cleanup := func() {
		// Cleanup MUST stop both timer sets and close both connection endpoints.
		s.timers.StopAll()
		s.stopSendHoldTimer()
		_ = client.Close()
		_ = conn.Close()
	}
	return s, client, capture, cleanup
}

// TestRFC7606CoalescedMalformedNLRIBoundaries contrasts complete /24 plus /0
// with a truncated /24 followed by /0, including an earlier accepted batch.
// RFC requirement: RFC7606-5.3-2 positive -- both readers retain a complete legacy /24 and the adjacent default route, including extended messages and ADD-PATH framing.
// RFC requirement: RFC7606-5.3-2 negative -- a truncated original legacy NLRI field resets with exact NOTIFICATION 3/10 before an adjacent message can complete it, without inventing any dispatch.
func TestRFC7606CoalescedMalformedNLRIBoundaries(t *testing.T) {
	for _, coalesced := range []bool{false, true} {
		for _, extended := range []bool{false, true} {
			for _, addPath := range []bool{false, true} {
				for _, mode := range []string{"valid", "truncated-first", "truncated-after-valid"} {
					name := "coalesced=" + strconv.FormatBool(coalesced) + "/extended=" + strconv.FormatBool(extended) + "/addpath=" + strconv.FormatBool(addPath) + "/" + mode
					t.Run(name, func(t *testing.T) {
						s, client, capture, cleanup := setupCapturingSession(t, 65002, extended, extended)
						defer cleanup()
						if addPath {
							addPathCap := &capability.AddPath{Families: []capability.AddPathFamily{{
								AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast, Mode: capability.AddPathBoth,
							}}}
							local := append([]capability.Capability(nil), s.settings.Capabilities...)
							local = append(local, addPathCap)
							remote := []capability.Capability{
								&capability.ASN4{ASN: 65002},
								&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
								addPathCap,
							}
							if extended {
								remote = append(remote, &capability.ExtendedMessage{})
							}
							neg := capability.Negotiate(local, remote, capability.PeerIdentity{LocalASN: 65001, PeerASN: 65002})
							ctxID, err := bgpctx.Registry.Register(bgpctx.FromNegotiatedRecv(neg))
							require.NoError(t, err)
							s.setRecvCtxID(ctxID)
						}
						reply := make(chan []byte, 1)
						go receiveBoundaryReply(client, reply)
						attrs := receiveBoundaryAttrs(extended)
						prefix := []byte{24, 192, 0, 0}
						defaultRoute := []byte{0}
						if addPath {
							prefix = append([]byte{0x81, 0, 0, 1}, prefix...)
							defaultRoute = append([]byte{0, 0, 0, 2}, defaultRoute...)
						}
						valid := buildUpdateMsg(receivedUpdateBody(attrs, prefix))
						bad := buildUpdateMsg(receivedUpdateBody(attrs, prefix[:len(prefix)-1]))
						adjacent := buildUpdateMsg(receivedUpdateBody(attrs, defaultRoute))
						if extended {
							require.Greater(t, len(bad), message.MaxMsgLen)
							require.Greater(t, len(adjacent), message.MaxMsgLen)
						}
						frames := [][]byte{bad, adjacent}
						if mode == "valid" {
							frames[0] = valid
						}
						if mode == "truncated-after-valid" {
							frames = append([][]byte{valid}, frames...)
						}
						err := receiveBoundaryFrames(t, s, coalesced, frames...)
						if mode == "valid" {
							require.NoError(t, err)
							require.Equal(t, fsm.StateEstablished, s.State())
							got := capture.all()
							if coalesced {
								require.Equal(t, [][]byte{receivedUpdateBody(attrs, append(bytes.Clone(prefix), defaultRoute...))}, got)
							} else {
								require.Equal(t, [][]byte{valid[message.HeaderLen:], adjacent[message.HeaderLen:]}, got)
							}
							cleanup()
							require.Empty(t, <-reply)
							return
						}
						require.Error(t, err)
						require.Equal(t, fsm.StateIdle, s.State())
						if mode == "truncated-after-valid" {
							require.Equal(t, [][]byte{valid[message.HeaderLen:]}, capture.all())
						} else {
							require.Empty(t, capture.all())
						}
						assertNotification(t, <-reply, message.NotifyUpdateMessage, message.NotifyUpdateInvalidNetwork, []byte{})
					})
				}
			}
		}
	}
}

// TestRFC7606DiagnosticWireAndMPNLRI inspects structured diagnostic records from
// both actual readers, not substrings that a body-only or synthetic dump satisfies.
// RFC requirement: RFC7606-6-1 positive -- malformed original UPDATEs produce separate records with exact header-inclusive wire hex and explicit legacy and MP NLRI lists, including extended inputs.
// RFC requirement: RFC7606-6-1 negative -- conforming adjacent UPDATEs produce no diagnostic record, and disabling debug prevents diagnostic allocation.
func TestRFC7606DiagnosticWireAndMPNLRI(t *testing.T) {
	for _, coalesced := range []bool{false, true} {
		for _, extended := range []bool{false, true} {
			for _, mp := range []string{"legacy", "ipv4", "ipv6", "typed"} {
				name := "coalesced=" + strconv.FormatBool(coalesced) + "/extended=" + strconv.FormatBool(extended) + "/mp=" + mp
				t.Run(name, func(t *testing.T) {
					logbuf := &syncBuffer{}
					lg := slog.New(slog.NewJSONHandler(logbuf, &slog.HandlerOptions{Level: slog.LevelDebug}))
					t.Cleanup(swapSessionLogger(func() *slog.Logger { return lg }))
					s, client, capture, cleanup := receiveBoundaryDiagnosticSession(t, extended)
					defer cleanup()
					reply := make(chan []byte, 1)
					go receiveBoundaryReply(client, reply)
					attrs := receiveBoundaryAttrs(extended)
					var wantReach, wantUnreach []any
					switch mp {
					case "legacy":
					case "ipv4":
						// RFC 4760 Sections 3 and 4: independently listed reach and unreach.
						attrs = append(attrs, 0x80, 14, 13, 0, 1, 1, 4, 192, 0, 2, 254, 0, 24, 198, 51, 100)
						attrs = append(attrs, 0x80, 15, 7, 0, 1, 1, 24, 203, 0, 113)
						wantReach = []any{"198.51.100.0/24"}
						wantUnreach = []any{"203.0.113.0/24"}
					case "ipv6":
						attrs = append(attrs, 0x80, 14, 26, 0, 2, 1, 16,
							0x20, 1, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1,
							0, 32, 0x20, 1, 0x0d, 0xb8)
						attrs = append(attrs, 0x80, 15, 10, 0, 2, 1, 48, 0x20, 1, 0x0d, 0xb8, 0, 1)
						wantReach = []any{"2001:db8::/32"}
						wantUnreach = []any{"2001:db8:1::/48"}
					case "typed":
						registerTypeLenRecognizer(t, evpnFam, 1, 5)
						attrs = append(attrs, 0x80, 14, 12, 0, 25, 70, 4, 192, 0, 2, 254, 0, 99, 1, 0xaa)
						attrs = append(attrs, 0x80, 15, 6, 0, 25, 70, 200, 1, 0xbb)
						wantReach = []any{"afi=25/safi=70/nlri=6301aa"}
						wantUnreach = []any{"afi=25/safi=70/nlri=c801bb"}
					}
					var withdrawn []byte
					if mp != "legacy" {
						withdrawn = []byte{24, 10, 0, 0}
					}
					valid := buildUpdateMsg(makeUpdateBody(withdrawn, attrs, []byte{24, 192, 0, 2}))
					// RFC 7606 Section 7.1: change only ORIGIN's value, retaining all NLRI.
					attrs[3] = 3
					bad1 := buildUpdateMsg(makeUpdateBody(withdrawn, attrs, []byte{24, 192, 0, 2}))
					bad2 := buildUpdateMsg(makeUpdateBody(withdrawn, attrs, []byte{0}))
					if extended {
						require.Greater(t, len(bad2), message.MaxMsgLen)
					}
					require.NoError(t, receiveBoundaryFrames(t, s, coalesced, valid, bad1, bad2))
					require.Equal(t, fsm.StateEstablished, s.State())
					require.NotEmpty(t, capture.all())
					var records []map[string]any
					for _, line := range strings.Split(strings.TrimSpace(logbuf.String()), "\n") {
						var record map[string]any
						require.NoError(t, json.Unmarshal([]byte(line), &record))
						if record["msg"] == "RFC 7606 diagnostics" {
							records = append(records, record)
						}
					}
					require.Len(t, records, 2, "one record per malformed original; no error for the valid control")
					for i, frame := range [][]byte{bad1, bad2} {
						require.Equal(t, "treat-as-withdraw", records[i]["event"])
						require.Equal(t, hex.EncodeToString(frame), records[i]["update-wire-hex"])
						want := "192.0.2.0/24"
						if i == 1 {
							want = "0.0.0.0/0"
						}
						require.Equal(t, []any{want}, records[i]["nlri-prefixes"])
						if mp != "legacy" {
							require.Equal(t, []any{"10.0.0.0/24"}, records[i]["withdrawn-prefixes"])
							require.Equal(t, wantReach, records[i]["mp-reach-prefixes"])
							require.Equal(t, wantUnreach, records[i]["mp-unreach-prefixes"])
						}
					}
					cleanup()
					require.Empty(t, <-reply)
				})
			}
		}
	}
	for _, coalesced := range []bool{false, true} {
		for _, extended := range []bool{false, true} {
			t.Run("first-as/coalesced="+strconv.FormatBool(coalesced)+"/extended="+strconv.FormatBool(extended), func(t *testing.T) {
				logbuf := &syncBuffer{}
				lg := slog.New(slog.NewJSONHandler(logbuf, &slog.HandlerOptions{Level: slog.LevelDebug}))
				t.Cleanup(swapSessionLogger(func() *slog.Logger { return lg }))
				s, client, capture, cleanup := receiveBoundaryDiagnosticSession(t, extended)
				defer cleanup()
				reply := make(chan []byte, 1)
				go receiveBoundaryReply(client, reply)
				attrs := receiveBoundaryAttrs(extended)
				// An unrecognized transitive attribute must gain Partial only on
				// publication, never in the original-message diagnostic.
				attrs = append(attrs, 0xc0, 100, 1, 0xaa)
				valid := buildUpdateMsg(receivedUpdateBody(attrs, []byte{24, 192, 0, 2}))
				attrs[12] = 0xeb // First ASN 65003, but the sending peer is 65002.
				bad1 := buildUpdateMsg(receivedUpdateBody(attrs, []byte{24, 198, 51, 100}))
				bad2 := buildUpdateMsg(receivedUpdateBody(attrs, []byte{0}))
				// RFC 7606 Sections 7.2 and 6: the optional first-AS check uses
				// treat-as-withdraw and must diagnose each original message.
				require.NoError(t, receiveBoundaryFrames(t, s, coalesced, valid, bad1, bad2))
				require.Equal(t, fsm.StateEstablished, s.State())
				require.Len(t, capture.all(), 3)
				require.Equal(t, makeUpdateBody([]byte{24, 198, 51, 100}, nil, nil), capture.all()[1])
				require.Equal(t, makeUpdateBody([]byte{0}, nil, nil), capture.all()[2])
				var records []map[string]any
				for _, line := range strings.Split(strings.TrimSpace(logbuf.String()), "\n") {
					var record map[string]any
					require.NoError(t, json.Unmarshal([]byte(line), &record))
					if record["msg"] == "RFC 7606 diagnostics" {
						records = append(records, record)
					}
				}
				require.Len(t, records, 2)
				for i, frame := range [][]byte{bad1, bad2} {
					require.Equal(t, "treat-as-withdraw", records[i]["event"])
					require.Equal(t, float64(2), records[i]["attr"])
					require.Equal(t, hex.EncodeToString(frame), records[i]["update-wire-hex"])
				}
				cleanup()
				require.Empty(t, <-reply)
			})
		}
	}
	for _, coalesced := range []bool{false, true} {
		for _, mode := range []string{"reconstructed-mismatch", "reconstructed-match", "aggregator-gate", "aggregator-as-trans"} {
			t.Run("first-as/old/coalesced="+strconv.FormatBool(coalesced)+"/"+mode, func(t *testing.T) {
				logbuf := &syncBuffer{}
				lg := slog.New(slog.NewJSONHandler(logbuf, &slog.HandlerOptions{Level: slog.LevelDebug}))
				t.Cleanup(swapSessionLogger(func() *slog.Logger { return lg }))
				// Match setupCapturingSession's deterministic pool budget:
				// this control must exercise a batch, not exhaustion fallback.
				bufMuxGlobalMu.Lock()
				if budget := bufMuxStd.mux.budget; budget != nil {
					previous := budget.maxBytes.Load()
					updateBufMuxBudget(0)
					t.Cleanup(func() {
						bufMuxGlobalMu.Lock()
						updateBufMuxBudget(previous)
						bufMuxGlobalMu.Unlock()
					})
				}
				bufMuxGlobalMu.Unlock()
				settings := NewPeerSettings(netip.MustParseAddr("192.0.2.2"), 65001, 65002, 0x01020301)
				caps := []capability.Capability{&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast}}
				s, client := firstASSession(t, settings, caps, caps)
				s.nextHopScope.Store(&receiveNextHopScope{local: netip.MustParseAddr("192.0.2.1")})
				var captured []*wireu.WireUpdate
				s.SetMessageCallback(func(_ netip.Addr, kind msgtype.MessageType, _ []byte,
					wu *wireu.WireUpdate, _ bgpctx.ContextID, direction rpc.MessageDirection,
					_ BufHandle, _ map[string]any, _ string, _ uint64,
				) bool {
					if kind == msgtype.TypeUPDATE && direction == rpc.DirectionReceived {
						captured = append(captured, wu.Snapshot())
					}
					return false
				})
				reply := make(chan []byte, 1)
				go receiveBoundaryReply(client, reply)
				rawAS, as4AS := uint32(65002), uint32(65003)
				if mode == "reconstructed-match" {
					rawAS, as4AS = as4AS, rawAS
				}
				attrs := firstASAttrs(2, rawAS)
				attrs = append(attrs, collapseAttr(0xc0, byte(attribute.AttrAS4Path), collapseASPathValue(4, as4AS))...)
				if mode == "aggregator-gate" || mode == "aggregator-as-trans" {
					aggAS := uint32(65002)
					if mode == "aggregator-as-trans" {
						aggAS = collapseASTrans
					}
					attrs = append(attrs, collapseAttr(0xc0, byte(attribute.AttrAggregator), collapseAggregatorValue(2, aggAS))...)
					attrs = append(attrs, collapseAttr(0xc0, byte(attribute.AttrAS4Aggregator), collapseAggregatorValue(4, 65002))...)
				}
				attrs = append(attrs, 0xc0, 100, 1, 0xaa)
				one := buildUpdateMsg(receivedUpdateBody(attrs, []byte{24, 198, 51, 100}))
				two := buildUpdateMsg(receivedUpdateBody(attrs, []byte{0}))
				// RFC 6793 Section 4.2.3: check the reconstructed first AS,
				// including the paired AGGREGATOR gate, not raw AS_TRANS-era bytes.
				require.NoError(t, receiveBoundaryFrames(t, s, coalesced, one, two))
				require.Equal(t, fsm.StateEstablished, s.State())
				var records []map[string]any
				decoder := json.NewDecoder(strings.NewReader(logbuf.String()))
				for {
					var record map[string]any
					err := decoder.Decode(&record)
					if err == io.EOF {
						break
					}
					require.NoError(t, err)
					if record["msg"] == "RFC 7606 diagnostics" {
						records = append(records, record)
					}
				}
				if mode == "reconstructed-mismatch" || mode == "aggregator-as-trans" {
					require.Len(t, captured, 2)
					require.Equal(t, makeUpdateBody([]byte{24, 198, 51, 100}, nil, nil), captured[0].Payload())
					require.Equal(t, makeUpdateBody([]byte{0}, nil, nil), captured[1].Payload())
					require.Len(t, records, 2)
					require.Equal(t, hex.EncodeToString(one), records[0]["update-wire-hex"])
					require.Equal(t, hex.EncodeToString(two), records[1]["update-wire-hex"])
				} else {
					require.Empty(t, records)
					if coalesced {
						require.Len(t, captured, 1)
						nlri, err := captured[0].NLRI()
						require.NoError(t, err)
						require.Equal(t, []byte{24, 198, 51, 100, 0}, nlri)
					} else {
						require.Len(t, captured, 2)
					}
					base, err := captured[0].Attrs()
					require.NoError(t, err)
					path, err := base.GetRaw(attribute.AttrASPath)
					require.NoError(t, err)
					require.Equal(t, collapseASPathValue(4, 65002), path)
				}
				require.NoError(t, client.Close())
				require.Empty(t, <-reply)
			})
		}
	}
	for _, coalesced := range []bool{false, true} {
		t.Run("first-as/after-in-place-discard/coalesced="+strconv.FormatBool(coalesced), func(t *testing.T) {
			logbuf := &syncBuffer{}
			lg := slog.New(slog.NewJSONHandler(logbuf, &slog.HandlerOptions{Level: slog.LevelDebug}))
			t.Cleanup(swapSessionLogger(func() *slog.Logger { return lg }))
			s, client, capture, cleanup := setupCapturingSession(t, 65002, false, false)
			defer cleanup()
			reply := make(chan []byte, 1)
			go receiveBoundaryReply(client, reply)
			attrs := receiveBoundaryAttrs(false)
			attrs[12] = 0xeb
			// RFC 7606 Section 7.7 discards this seven-octet AGGREGATOR.
			// The single value is large enough for an in-place tombstone.
			attrs = append(attrs, 0xc0, 7, 7, 0, 0, 0xfd, 0xea, 192, 0, 2)
			attrs = append(attrs, 0xc0, 100, 1, 0xaa)
			frame := buildUpdateMsg(receivedUpdateBody(attrs, []byte{24, 198, 51, 100}))
			// RFC 7606 Sections 7.2 and 6: both errors describe the same
			// received message, not the first error's ATTR_TOMBSTONE rewrite.
			require.NoError(t, receiveBoundaryFrames(t, s, coalesced, frame))
			require.Equal(t, fsm.StateEstablished, s.State())
			require.Equal(t, [][]byte{makeUpdateBody([]byte{24, 198, 51, 100}, nil, nil)}, capture.all())
			var records []map[string]any
			for _, line := range strings.Split(strings.TrimSpace(logbuf.String()), "\n") {
				var record map[string]any
				require.NoError(t, json.Unmarshal([]byte(line), &record))
				if record["msg"] == "RFC 7606 diagnostics" {
					records = append(records, record)
				}
			}
			require.Len(t, records, 2)
			require.Equal(t, "attribute-discard", records[0]["event"])
			require.Equal(t, "treat-as-withdraw", records[1]["event"])
			for _, record := range records {
				require.Equal(t, hex.EncodeToString(frame), record["update-wire-hex"])
			}
			cleanup()
			require.Empty(t, <-reply)
		})
	}
	t.Run("next-hop-policy-labels-processed-representation", func(t *testing.T) {
		logbuf := &syncBuffer{}
		lg := slog.New(slog.NewJSONHandler(logbuf, &slog.HandlerOptions{Level: slog.LevelDebug}))
		t.Cleanup(swapSessionLogger(func() *slog.Logger { return lg }))
		s, client, capture, cleanup := setupCapturingSession(t, 65002, false, false)
		defer cleanup()
		reply := make(chan []byte, 1)
		go receiveBoundaryReply(client, reply)
		s.nextHopScope.Store(&receiveNextHopScope{local: netip.MustParseAddr("192.0.2.1")})
		attrs := receiveBoundaryAttrs(false)
		one := buildUpdateMsg(receivedUpdateBody(attrs, []byte{24, 192, 0, 2}))
		two := buildUpdateMsg(receivedUpdateBody(attrs, []byte{0}))
		// RFC 4271 Section 6.3 semantic scope is a route-ignore policy, not
		// RFC 7606 Section 7.3's malformed NEXT_HOP length.
		require.NoError(t, receiveBoundaryFrames(t, s, true, one, two))
		require.Equal(t, fsm.StateEstablished, s.State())
		require.Len(t, capture.all(), 1)
		var records []map[string]any
		for _, line := range strings.Split(strings.TrimSpace(logbuf.String()), "\n") {
			var record map[string]any
			require.NoError(t, json.Unmarshal([]byte(line), &record))
			if record["msg"] == "RFC 7606 diagnostics" {
				records = append(records, record)
			}
		}
		require.Len(t, records, 1)
		require.Equal(t, "invalid-next-hop", records[0]["event"])
		require.Equal(t, "processed", records[0]["representation"])
		require.NotContains(t, records[0], "update-wire-hex")
		require.Equal(t, hex.EncodeToString(buildUpdateMsg(receivedUpdateBody(attrs, []byte{24, 192, 0, 2, 0}))), records[0]["update-processed-hex"])
		cleanup()
		require.Empty(t, <-reply)
	})
	for _, coalesced := range []bool{false, true} {
		for _, extended := range []bool{false, true} {
			for _, reachable := range []bool{false, true} {
				for _, mismatch := range []bool{false, true} {
					name := "late-first-as/coalesced=" + strconv.FormatBool(coalesced) +
						"/extended=" + strconv.FormatBool(extended) + "/reachable=" +
						strconv.FormatBool(reachable) + "/mismatch=" + strconv.FormatBool(mismatch)
					t.Run(name, func(t *testing.T) {
						logbuf := &syncBuffer{}
						lg := slog.New(slog.NewJSONHandler(logbuf, &slog.HandlerOptions{Level: slog.LevelDebug}))
						t.Cleanup(swapSessionLogger(func() *slog.Logger { return lg }))
						s, client, capture, cleanup := receiveBoundaryDiagnosticSession(t, extended)
						defer cleanup()
						reply := make(chan []byte, 1)
						go receiveBoundaryReply(client, reply)
						attrs := receiveBoundaryAttrs(extended)
						if mismatch {
							attrs[12] = 0xeb
						}
						mp := []byte{0x80, 14, 9, 0, 1, 1, 4, 192, 0, 2, 254, 0}
						if reachable {
							mp[2] = 13
							mp = append(mp, 24, 198, 51, 100)
						}
						attrs = append(attrs, mp...)
						body := makeUpdateBody(nil, attrs, nil)
						frame := buildUpdateMsg(body)
						if extended {
							require.Greater(t, len(frame), message.MaxMsgLen)
						}
						err := receiveBoundaryFrames(t, s, coalesced, frame)
						if mismatch && !reachable {
							// RFC 7606 Section 5.2: an attribute error stronger than
							// discard, with no reachable NLRI, MUST reset, not emit EOR.
							require.Error(t, err)
							require.Equal(t, fsm.StateIdle, s.State())
							require.Nil(t, s.Conn())
							require.Empty(t, capture.all(), "no consumer UPDATE or false End-of-RIB")
							assertNotification(t, <-reply, 3, 1, []byte{})
						} else {
							require.NoError(t, err)
							require.Equal(t, fsm.StateEstablished, s.State())
							require.Len(t, capture.all(), 1)
							if mismatch {
								wantAttrs := []byte{0x80, 15, 7, 0, 1, 1, 24, 198, 51, 100}
								require.Equal(t, makeUpdateBody(nil, wantAttrs, nil), capture.all()[0])
							} else {
								require.Equal(t, body, capture.all()[0])
							}
							_, eor := wireu.NewWireUpdate(capture.all()[0], s.recvCtxID).IsEOR()
							require.False(t, eor)
							cleanup()
							require.Empty(t, <-reply)
						}
						var records []map[string]any
						decoder := json.NewDecoder(strings.NewReader(logbuf.String()))
						for {
							var record map[string]any
							err := decoder.Decode(&record)
							if err == io.EOF {
								break
							}
							require.NoError(t, err)
							if record["msg"] == "RFC 7606 diagnostics" {
								records = append(records, record)
							}
						}
						if mismatch {
							require.Len(t, records, 1)
							event := "session-reset"
							if reachable {
								event = "treat-as-withdraw"
							}
							require.Equal(t, event, records[0]["event"])
							require.Equal(t, hex.EncodeToString(frame), records[0]["update-wire-hex"])
						} else {
							require.Empty(t, records)
						}
					})
				}
			}
		}
	}
	for _, coalesced := range []bool{false, true} {
		for _, extended := range []bool{false, true} {
			for _, reachable := range []bool{false, true} {
				for _, endpointCase := range []string{"valid", "iana-invalid", "length-invalid"} {
					name := "late-tunnel/coalesced=" + strconv.FormatBool(coalesced) +
						"/extended=" + strconv.FormatBool(extended) + "/reachable=" +
						strconv.FormatBool(reachable) + "/endpoint=" + endpointCase
					t.Run(name, func(t *testing.T) {
						logbuf := &syncBuffer{}
						lg := slog.New(slog.NewJSONHandler(logbuf, &slog.HandlerOptions{Level: slog.LevelDebug}))
						t.Cleanup(swapSessionLogger(func() *slog.Logger { return lg }))
						s, client, capture, cleanup := receiveBoundaryDiagnosticSession(t, extended)
						defer cleanup()
						reply := make(chan []byte, 1)
						go receiveBoundaryReply(client, reply)
						attrs := receiveBoundaryAttrs(extended) // Matching neighbor AS.
						endpoint := teRegistryEndpoint("10.0.0.77")
						switch endpointCase {
						case "iana-invalid":
							// RFC 9012 Section 3.1: loopback is not forwardable.
							endpoint = teRegistryEndpoint("127.0.0.1")
						case "length-invalid":
							// Correct framing, but IPv4 requires ten value octets.
							endpoint = teSub(6, 0, 0, 0, 0, 0, 1, 10, 0, 0)
						}
						attrs = append(attrs, collapseAttr(0xc0, byte(attribute.AttrTunnelEncap), teTLV(2, endpoint))...)
						mp := []byte{0x80, 14, 9, 0, 1, 1, 4, 192, 0, 2, 254, 0}
						if reachable {
							mp[2] = 13
							mp = append(mp, 24, 198, 51, 100)
						}
						attrs = append(attrs, mp...)
						verdict := s.validateRFC7606Attrs(attrs, false)
						require.Equal(t, message.RFC7606ActionNone, verdict.Action, "isolate the late endpoint validator")
						require.Equal(t, reachable, verdict.HasReachableNLRI)
						body := makeUpdateBody(nil, attrs, nil)
						frame := buildUpdateMsg(body)
						if extended {
							require.Greater(t, len(frame), message.MaxMsgLen)
						}
						invalid := endpointCase != "valid"
						err := receiveBoundaryFrames(t, s, coalesced, frame)
						if invalid && !reachable {
							// RFC 7606 Section 5.2 also applies when the error is
							// found by RFC 9012 carrier/endpoint enforcement.
							require.Error(t, err)
							require.Equal(t, fsm.StateIdle, s.State())
							require.Nil(t, s.Conn())
							require.Empty(t, capture.all(), "no consumer UPDATE or false End-of-RIB")
							assertNotification(t, <-reply, 3, 1, []byte{})
						} else {
							require.NoError(t, err)
							require.Equal(t, fsm.StateEstablished, s.State())
							require.Len(t, capture.all(), 1)
							if invalid {
								wantAttrs := []byte{0x80, 15, 7, 0, 1, 1, 24, 198, 51, 100}
								require.Equal(t, makeUpdateBody(nil, wantAttrs, nil), capture.all()[0])
							} else {
								require.Equal(t, body, capture.all()[0])
							}
							_, eor := wireu.NewWireUpdate(capture.all()[0], s.recvCtxID).IsEOR()
							require.False(t, eor)
							cleanup()
							require.Empty(t, <-reply)
						}
						var records []map[string]any
						decoder := json.NewDecoder(strings.NewReader(logbuf.String()))
						for {
							var record map[string]any
							err := decoder.Decode(&record)
							if err == io.EOF {
								break
							}
							require.NoError(t, err)
							if record["msg"] == "RFC 7606 diagnostics" {
								records = append(records, record)
							}
						}
						if invalid {
							require.Len(t, records, 1)
							event := "session-reset"
							if reachable {
								event = "treat-as-withdraw"
							}
							require.Equal(t, event, records[0]["event"])
							require.Equal(t, hex.EncodeToString(frame), records[0]["update-wire-hex"])
						} else {
							require.Empty(t, records)
						}
					})
				}
			}
		}
	}
	t.Run("debug-disabled-no-allocation", func(t *testing.T) {
		captureSessionLog(t, slog.LevelWarn)
		s := rfc7606DiagSession()
		wu := wireu.NewWireUpdate(receivedUpdateBody(receiveBoundaryAttrs(true), []byte{0}), 0)
		// RFC 7606 Section 6: disabled facility returns before building wire or NLRI text.
		require.Zero(t, testing.AllocsPerRun(50, func() {
			s.rfc7606Diagnostics("treat-as-withdraw", wu, 1, "malformed ORIGIN")
		}))
	})
}

// TestReceiveAdmissionSurvivesIdleTransition holds the existing session mutex
// at the consumer boundary while a timer moves the FSM to Idle. No production
// hook is needed: the goroutine's blocked stack identifies the ordering.
func TestReceiveAdmissionSurvivesIdleTransition(t *testing.T) {
	for _, state := range []fsm.State{fsm.StateOpenSent, fsm.StateOpenConfirm} {
		t.Run(state.String(), func(t *testing.T) {
			settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65002, 0x01020301)
			settings.Connection = ConnectionPassive
			s := NewSession(settings)
			client, server := net.Pipe()
			t.Cleanup(func() {
				s.timers.StopAll()
				s.stopSendHoldTimer()
				_ = client.Close()
				_ = server.Close()
			})
			startSession(t, s)
			readOpen(t, s, server, client)
			s.timers.StopAll()
			s.fsm.SetCallback(nil)
			if state == fsm.StateOpenConfirm {
				require.NoError(t, s.fsm.Event(fsm.EventBGPOpen))
			}
			require.Equal(t, state, s.State())
			dispatches := 0
			s.SetMessageCallback(func(_ netip.Addr, typ msgtype.MessageType, _ []byte,
				_ *wireu.WireUpdate, _ bgpctx.ContextID, direction rpc.MessageDirection,
				_ BufHandle, _ map[string]any, _ string, _ uint64,
			) bool {
				if typ == msgtype.TypeUPDATE && direction == rpc.DirectionReceived {
					dispatches++
				}
				return false
			})
			reply := make(chan []byte, 1)
			go receiveBoundaryReply(client, reply)
			type outcome struct {
				err        error
				kept       bool
				panicValue any
			}
			done := make(chan outcome, 1)
			s.mu.Lock()
			locked := true
			defer func() {
				if locked {
					s.mu.Unlock()
				}
			}()
			go func() {
				var result outcome
				defer func() {
					result.panicValue = recover()
					done <- result
				}()
				// A valid empty UPDATE would be an EOR if wrongly admitted.
				hdr := message.Header{Length: message.HeaderLen + 4, Type: msgtype.TypeUPDATE}
				result.err, result.kept = s.processMessage(&hdr, []byte{0, 0, 0, 0}, BufHandle{})
			}()
			stacks := make([]byte, 1<<20)
			require.Eventually(t, func() bool {
				n := runtime.Stack(stacks, true)
				for _, stack := range strings.Split(string(stacks[:n]), "\n\n") {
					if strings.Contains(stack, "(*Session).processMessage(") &&
						strings.Contains(stack, "sync.(*RWMutex).RLock(") &&
						(strings.Contains(stack, "(*Session).processValidatedMessage(") ||
							strings.Contains(stack, "(*Session).fsmMessageEvent(")) {
						return true
					}
				}
				return false
			}, time.Second, time.Millisecond, "receive must reach the existing locked consumer boundary")
			// RFC 4271 Section 8.2.2: a timer can move an unexpected-message
			// state to Idle independently of the receive goroutine.
			require.NoError(t, s.fsm.Event(fsm.EventHoldTimerExpires))
			require.Equal(t, fsm.StateIdle, s.State())
			s.mu.Unlock()
			locked = false
			result := <-done
			require.Nil(t, result.panicValue, "an unexpected UPDATE must never reach unclassified enforcement")
			require.ErrorIs(t, result.err, fsm.ErrFSMError)
			require.False(t, result.kept)
			require.Zero(t, dispatches, "no UPDATE or false EOR reaches consumers")
			require.Nil(t, s.Conn())
			assertNotification(t, <-reply, 5, 0, []byte{})
		})
	}
}
