package reactor

import (
	"io"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// TestRFC7606AttributeEnvelopeSession drives RFC 7606 Sections 3(c), 5.2 and
// 7.16 through both Session readers. Exact dispatched bodies distinguish
// acceptance, withdrawal, attribute discard and a fabricated End-of-RIB.
// RFC requirement: RFC7606-5.2-1 negative -- empty MP_REACH plus malformed ORIGIN resets with NOTIFICATION 3/1 and no dispatch through both readers, unlike reachable-NLRI and discard-only controls.
// RFC requirement: RFC7606-3.c-1 negative -- TE and ATTR_SET Optional/Transitive flag conflicts withdraw actual Session routes while correct flags preserve the UPDATE.
// RFC requirement: RFC7606-7.16-1 negative -- malformed inner AGGREGATOR width and ORIGIN flags withdraw actual Session routes while valid counterparts survive.
func TestRFC7606AttributeEnvelopeSession(t *testing.T) {
	encode := func(flags, code byte, value []byte) []byte {
		return append([]byte{flags, code, byte(len(value))}, value...)
	}
	attrSet := func(inner []byte) []byte {
		return encode(0xc0, 128, append([]byte{0, 0, 0xfd, 0xe8}, inner...))
	}
	emptyMP := []byte{0x80, 14, 9, 0, 1, 1, 4, 192, 0, 2, 1, 0}
	reachableMP := []byte{0x80, 14, 13, 0, 1, 1, 4, 192, 0, 2, 1, 0, 24, 203, 0, 113}
	legacyNLRI := []byte{24, 198, 51, 100}
	for _, tc := range []struct {
		name      string
		extra     []byte
		nlri      []byte
		badOrigin bool
		want      message.RFC7606Action
	}{
		{"empty_mp_bad_origin", emptyMP, nil, true, message.RFC7606ActionSessionReset},
		{"reachable_mp_bad_origin", reachableMP, nil, true, message.RFC7606ActionTreatAsWithdraw},
		{"empty_mp_valid", emptyMP, nil, false, message.RFC7606ActionNone},
		{"empty_mp_discard_only", append(append([]byte(nil), emptyMP...), 0xc0, 7, 1, 0), nil, false, message.RFC7606ActionAttributeDiscard},
		{"te_correct", encode(0x80, 24, make([]byte, 36)), legacyNLRI, false, message.RFC7606ActionNone},
		{"te_optional_conflict", encode(0x00, 24, make([]byte, 36)), legacyNLRI, false, message.RFC7606ActionTreatAsWithdraw},
		{"te_transitive_conflict", encode(0xc0, 24, make([]byte, 36)), legacyNLRI, false, message.RFC7606ActionTreatAsWithdraw},
		{"attr_set_correct", encode(0xc0, 128, []byte{0, 0, 0xfd, 0xe8}), legacyNLRI, false, message.RFC7606ActionNone},
		{"attr_set_optional_conflict", encode(0x40, 128, []byte{0, 0, 0xfd, 0xe8}), legacyNLRI, false, message.RFC7606ActionTreatAsWithdraw},
		{"attr_set_transitive_conflict", encode(0x80, 128, []byte{0, 0, 0xfd, 0xe8}), legacyNLRI, false, message.RFC7606ActionTreatAsWithdraw},
		{"inner_aggregator_asn2", attrSet(encode(0xc0, 7, []byte{0xfd, 0xe8, 192, 0, 2, 1})), legacyNLRI, false, message.RFC7606ActionTreatAsWithdraw},
		{"inner_aggregator_asn4", attrSet(encode(0xc0, 7, []byte{0, 0, 0xfd, 0xe8, 192, 0, 2, 1})), legacyNLRI, false, message.RFC7606ActionNone},
		{"inner_origin_conflict", attrSet(encode(0xc0, 1, []byte{0})), legacyNLRI, false, message.RFC7606ActionTreatAsWithdraw},
		{"inner_origin_correct", attrSet(encode(0x40, 1, []byte{0})), legacyNLRI, false, message.RFC7606ActionNone},
	} {
		for _, coalesced := range []bool{false, true} {
			t.Run(tc.name+"/coalesced="+strconv.FormatBool(coalesced), func(t *testing.T) {
				session, client, capture, cleanup := setupCapturingSession(t, 65002, true, false)
				defer cleanup()
				attrs := firstASAttrs(4, 65002)
				if tc.badOrigin {
					attrs[3] = 3
				}
				attrs = append(attrs, tc.extra...)
				// RFC 4271 Section 5: opaque optional-transitive data; Partial
				// is already set so valid-control bytes survive normalization.
				attrs = append(attrs, 0xf0, 99, 0x10, 0x00)
				attrs = append(attrs, make([]byte, 4096)...)
				body := receivedUpdateBody(attrs, tc.nlri)
				frame := buildUpdateMsg(body)
				require.Greater(t, len(frame), message.MaxMsgLen)
				answers := make(chan []byte, 1)
				go func() {
					data, _ := io.ReadAll(client)
					answers <- data
				}()
				written := make(chan error, 1)
				go func() {
					_, err := client.Write(frame)
					written <- err
				}()
				// RFC 7606 Sections 3(c), 5.2 and 7.16: real receive enforcement.
				var err error
				if coalesced {
					err = session.readAndProcessCoalesced(session.Conn(), session.bufReader)
				} else {
					err = session.ReadAndProcess()
				}
				require.NoError(t, <-written)
				got := capture.all()
				if tc.want == message.RFC7606ActionSessionReset {
					require.Error(t, err)
					require.Equal(t, fsm.StateIdle, session.State())
					require.Empty(t, got, "no consumer may receive a forged End-of-RIB")
					cleanup()
					assertNotification(t, <-answers, message.NotifyUpdateMessage, message.NotifyUpdateMalformedAttr, []byte{})
					return
				}
				require.NoError(t, err)
				require.Equal(t, fsm.StateEstablished, session.State())
				require.Len(t, got, 1)
				_, isEOR := wireu.NewWireUpdate(got[0], 0).IsEOR()
				require.False(t, isEOR, "the peer sent no End-of-RIB")
				switch tc.want {
				case message.RFC7606ActionNone:
					require.Equal(t, body, got[0], "valid attributes must survive intact")
				case message.RFC7606ActionTreatAsWithdraw:
					if tc.nlri != nil {
						require.Equal(t, makeUpdateBody(tc.nlri, nil, nil), got[0])
					} else {
						wantMP := []byte{0x80, 15, 7, 0, 1, 1, 24, 203, 0, 113}
						require.Equal(t, makeUpdateBody(nil, wantMP, nil), got[0])
					}
				case message.RFC7606ActionAttributeDiscard:
					withdrawn, kept, nlri := payloadSections(t, got[0])
					require.Empty(t, withdrawn)
					require.Empty(t, nlri)
					_, _, _, found := attribute.AttrFind(kept, attribute.AttrAggregator)
					require.False(t, found)
					_, _, mp, found := attribute.AttrFind(kept, attribute.AttrMPReachNLRI)
					require.True(t, found)
					require.Equal(t, emptyMP[3:], mp)
				case message.RFC7606ActionSessionReset:
					t.Fatal("session-reset handled above")
				}
				cleanup()
				require.Empty(t, <-answers, "surviving session must emit no NOTIFICATION")
			})
		}
	}
}
