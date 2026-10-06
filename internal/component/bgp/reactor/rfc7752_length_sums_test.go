package reactor

import (
	"bufio"
	"bytes"
	"log/slog"
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// RFC 7752 Section 6.2.2: "An implementation of BGP-LS MUST perform the following
// syntactic checks for determining if a message is malformed."
// Its successor, RFC 9552 Section 8.2.2, distinguishes skipable descriptor
// errors (NLRI discard) from unparseable outer framing (session reset).
// Keep the original exact-sum payloads and vary only their length octets.
// RFC requirement: RFC7752-6.2.2-2 positive -- exact-sum type 1, 2 and 3 NLRIs retain their bytes through MP_REACH and MP_UNREACH validation and dispatch without notification.
// RFC requirement: RFC7752-6.2.2-2 negative -- outer NLRI overruns reset with the length-sum diagnostic and NOTIFICATION 3/1; descriptor over/underruns log malformed-NLRI discard and dispatch nothing without notification.
func TestRFC7752LinkStateLengthSumsOnReceive(t *testing.T) {
	for _, kind := range []byte{1, 2, 3} {
		for _, withdrawn := range []bool{false, true} {
			for _, fault := range []string{"none", "nlri-overrun", "descriptor-overrun", "descriptor-underrun"} {
				wire := lsNodeNLRI(65001)
				wire[1] = kind
				switch fault {
				case "nlri-overrun":
					wire[3]++
				case "descriptor-overrun":
					wire[16]++
				case "descriptor-underrun":
					wire[16]--
				}
				attrs := mpReachAttrs(lsFam, wire)
				if withdrawn {
					attrs = mpUnreachAttrs(lsFam, wire)
				}
				body := makeUpdateBody(nil, attrs, nil)
				original := bytes.Clone(body)
				diagnostics := captureSessionLog(t, slog.LevelDebug)
				// RFC 7752 Section 6.2.2; RFC 9552 Section 8.2.2.
				result, action, err := nlriTypeTestSession().enforceRFC7606(wireu.NewWireUpdate(body, 0))
				wantAction := message.RFC7606ActionTreatAsWithdraw
				wantError := ""
				switch fault {
				case "none":
					wantAction = message.RFC7606ActionNone
				case "nlri-overrun":
					wantAction = message.RFC7606ActionSessionReset
					wantError = "RFC 7606 session reset: RFC 9552 Section 8.2.2: Link-State NLRI lengths do not sum to the MP attribute length"
				}
				if action != wantAction {
					t.Fatalf("kind=%d withdrawn=%v fault=%s: action=%v, want %v", kind, withdrawn, fault, action, wantAction)
				}
				assertLinkStateLengthError(t, err, wantError)
				if result == nil {
					t.Fatalf("kind=%d withdrawn=%v fault=%s: missing validation result", kind, withdrawn, fault)
				}
				if fault == "none" || fault == "nlri-overrun" {
					if !bytes.Equal(result.Payload(), original) {
						t.Fatalf("kind=%d withdrawn=%v fault=%s: validation changed payload", kind, withdrawn, fault)
					}
				} else {
					remainingAttrs := attrs[:7] // Only ORIGIN and the empty AS_PATH survive MP_REACH.
					if withdrawn {
						remainingAttrs = nil
					}
					if !bytes.Equal(result.Payload(), makeUpdateBody(nil, remainingAttrs, nil)) {
						t.Fatalf("kind=%d withdrawn=%v fault=%s: discard left unexpected payload %x", kind, withdrawn, fault, result.Payload())
					}
					if !strings.Contains(diagnostics.String(), `msg="RFC 9552 Section 8.2.2: discarded malformed Link-State NLRIs"`) {
						t.Fatalf("kind=%d withdrawn=%v fault=%s: missing malformed-NLRI diagnostic: %s", kind, withdrawn, fault, diagnostics.String())
					}
					if !strings.Contains(diagnostics.String(), "discarded=1") {
						t.Fatalf("expected exactly one malformed NLRI discarded: %s", diagnostics.String())
					}
				}
				assertLinkStateLengthDispatch(t, original, fault, wantError)
			}
		}
	}
}

// TestRFC7752AttributeLengthSumsOnReceive observes code 29 at the Established
// session's consumer boundary, rather than through the offline TLV decoder.
// RFC 7752 Section 6.2.2 requires exact attribute sums; its successor RFC 9552
// Section 8.2.2 requires whole-attribute discard without withdrawing the NLRI.
// The propagator checks framing, not TLV semantic layouts or value contents.
// RFC requirement: RFC7752-6.2.2-2 positive -- exact-sum code29 TLVs and a valid continuation survive Established processing byte-for-byte, including an unknown TLV.
// RFC requirement: RFC7752-6.2.2-2 negative -- isolated truncated TLV headers and value overruns remove the whole code29 attribute at dispatch while preserving all other attributes and the Link-State NLRI.
func TestRFC7752AttributeLengthSumsOnReceive(t *testing.T) {
	for _, tc := range []struct {
		name    string
		value   []byte
		discard bool
	}{
		{
			name:  "exact-sum",
			value: []byte{0x04, 0x00, 0, 1, 0, 0xff, 0xfe, 0, 2, 0xab, 0xcd},
		},
		{
			name:    "truncated-header",
			value:   []byte{0x04, 0x00, 0, 1, 0, 0xff, 0xfe, 0},
			discard: true,
		},
		{
			name:    "value-overrun",
			value:   []byte{0x04, 0x00, 0, 1, 0, 0xff, 0xfe, 0, 3, 0xab, 0xcd},
			discard: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65001, 0x01020301)
			session := NewSession(settings)
			t.Cleanup(session.timers.StopAll)
			t.Cleanup(session.stopSendHoldTimer)
			for _, event := range []fsm.Event{fsm.EventManualStart, fsm.EventTCPConnectionConfirmed, fsm.EventBGPOpen, fsm.EventKeepaliveMsg} {
				if err := session.fsm.Event(event); err != nil {
					t.Fatalf("establish attribute length-sum fixture: %v", err)
				}
			}
			conn := &recordingConn{}
			session.conn = conn
			session.bufWriter = bufio.NewWriterSize(conn, 4096)
			t.Cleanup(session.closeConn)
			var dispatched [][]byte
			session.onMessageReceived = func(_ netip.Addr, _ msgtype.MessageType, _ []byte,
				wu *wireu.WireUpdate, _ bgpctx.ContextID, direction rpc.MessageDirection,
				_ BufHandle, _ map[string]any, _ string, _ uint64) bool {
				if direction == rpc.DirectionReceived && wu != nil {
					dispatched = append(dispatched, bytes.Clone(wu.Payload()))
				}
				return false
			}

			// Keep the outer attribute lengths correct. Only the code29 TLV
			// framing differs, after a complete valid Node Flag Bits TLV.
			// Place MP_REACH after code29 to prove the rest is still processed.
			baseAttrs := mpReachAttrs(lsFam, lsNodeNLRI(65001))
			attrs := append(bytes.Clone(baseAttrs[:7]), 0x80, 29, byte(len(tc.value)))
			attrs = append(attrs, tc.value...)
			attrs = append(attrs, baseAttrs[7:]...)
			body := makeUpdateBody(nil, attrs, nil)
			wantAttrs := bytes.Clone(attrs)
			if tc.discard {
				// Ze's existing ATTR_TOMBSTONE representation keeps the
				// original size, names code29/reason 2, and zeroes its tail.
				// Construct the oracle independently of ApplyAttrDiscard.
				wantAttrs[8] = byte(attribute.AttrTombstone)
				wantAttrs[10] = 29
				wantAttrs[11] = wireu.TombstoneInvalidLength
				clear(wantAttrs[12 : 10+len(tc.value)])
			}
			want := makeUpdateBody(nil, wantAttrs, nil)
			continuationAttrs := mpReachAttrs(lsFam, lsNodeNLRI(65002))
			continuationAttrs = append(continuationAttrs,
				0x80, 29, 5, 0x04, 0x00, 0, 1, 0)
			continuation := makeUpdateBody(nil, continuationAttrs, nil)
			for i, input := range [][]byte{body, continuation} {
				header := message.Header{Type: msgtype.TypeUPDATE, Length: uint16(message.HeaderLen + len(input))} //nolint:gosec // Fixed fixtures fit in a BGP message.
				err, kept := session.processMessage(&header, bytes.Clone(input), BufHandle{ID: noPoolBufID})
				assertLinkStateLengthError(t, err, "")
				if kept {
					t.Fatal("non-owning callback retained the UPDATE buffer")
				}
				if len(dispatched) != i+1 {
					t.Fatalf("message %d: dispatches=%d, want %d", i, len(dispatched), i+1)
				}
				if i == 1 {
					want = continuation
				}
				if !bytes.Equal(dispatched[i], want) {
					t.Fatalf("message %d: dispatched UPDATE=%x, want %x", i, dispatched[i], want)
				}
				if session.State() != fsm.StateEstablished {
					t.Fatalf("message %d: state=%v, want Established", i, session.State())
				}
				if len(conn.written()) != 0 {
					t.Fatalf("message %d: unexpected notification bytes %x", i, conn.written())
				}
			}
		})
	}
}

// assertLinkStateLengthError excludes unrelated errors from the length-sum oracle.
func assertLinkStateLengthError(t *testing.T, err error, want string) {
	t.Helper()
	if want == "" {
		if err != nil {
			t.Fatalf("unexpected receive error: %v", err)
		}
		return
	}
	if err == nil {
		t.Fatalf("missing receive error %q", want)
	}
	if err.Error() != want {
		t.Fatalf("receive error=%q, want %q", err.Error(), want)
	}
}

// assertLinkStateLengthDispatch observes the consumer and notification boundaries.
// The empty AS_PATH control is legal on iBGP; keep its original bytes rather than
// changing the payload to satisfy the unrelated eBGP first-AS admission rule.
func assertLinkStateLengthDispatch(t *testing.T, body []byte, fault, wantError string) {
	t.Helper()
	settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, 65001, 0x01020301)
	session := NewSession(settings)
	t.Cleanup(session.timers.StopAll)
	t.Cleanup(session.stopSendHoldTimer)
	// RFC 4271 Section 8.2.2: establish the fixture before delivering UPDATEs.
	for _, event := range []fsm.Event{fsm.EventManualStart, fsm.EventTCPConnectionConfirmed, fsm.EventBGPOpen, fsm.EventKeepaliveMsg} {
		if err := session.fsm.Event(event); err != nil {
			t.Fatalf("establish length-sum fixture: %v", err)
		}
	}
	conn := &recordingConn{}
	session.conn = conn
	session.bufWriter = bufio.NewWriterSize(conn, 4096)
	t.Cleanup(session.closeConn)
	var dispatched [][]byte
	session.onMessageReceived = func(_ netip.Addr, _ msgtype.MessageType, _ []byte,
		wu *wireu.WireUpdate, _ bgpctx.ContextID, direction rpc.MessageDirection,
		_ BufHandle, _ map[string]any, _ string, _ uint64) bool {
		if direction == rpc.DirectionReceived && wu != nil {
			dispatched = append(dispatched, bytes.Clone(wu.Payload()))
		}
		return false
	}
	header := message.Header{Type: msgtype.TypeUPDATE, Length: uint16(message.HeaderLen + len(body))} //nolint:gosec // Fixed fixtures fit in a BGP message.
	// RFC 7752 Section 6.2.2; RFC 9552 Section 8.2.2; RFC 7606 Section 3(a).
	err, kept := session.processMessage(&header, bytes.Clone(body), BufHandle{ID: noPoolBufID})
	assertLinkStateLengthError(t, err, wantError)
	if kept {
		t.Fatal("non-owning callback retained the UPDATE buffer")
	}
	if fault == "none" {
		if len(dispatched) != 1 {
			t.Fatalf("valid control dispatches=%d, want 1", len(dispatched))
		}
		if !bytes.Equal(dispatched[0], body) {
			t.Fatalf("valid control changed on dispatch: got %x, want %x", dispatched[0], body)
		}
	} else if len(dispatched) != 0 {
		t.Fatalf("fault=%s dispatched %d malformed UPDATEs", fault, len(dispatched))
	}
	if fault == "nlri-overrun" {
		// RFC 7606 Section 3(a): "An error detected while processing the UPDATE
		// message for which a session reset is specified MUST be indicated by
		// sending the NOTIFICATION message with the Error Code UPDATE Message Error."
		want := message.PackTo(&message.Notification{
			ErrorCode: message.NotifyUpdateMessage, ErrorSubcode: message.NotifyUpdateMalformedAttr,
		}, nil)
		if !bytes.Equal(conn.written(), want) {
			t.Fatalf("notification=%x, want %x", conn.written(), want)
		}
		if session.State() != fsm.StateIdle {
			t.Fatalf("outer overrun state=%v, want Idle", session.State())
		}
		return
	}
	if len(conn.written()) != 0 {
		t.Fatalf("fault=%s unexpectedly wrote %x", fault, conn.written())
	}
	if session.State() != fsm.StateEstablished {
		t.Fatalf("fault=%s state=%v, want Established", fault, session.State())
	}
}
