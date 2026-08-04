// RFC: rfc/short/rfc7606.md -- Section 6 debugging facilities, Section 5.4 typed NLRI
// Overview: session_validation.go -- enforceRFC7606, whose Section 5.4 branch returns early

package reactor

import (
	"encoding/hex"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
)

// VALIDATES: RFC 7606 Section 6 -- "such facilities must include logging an error listing
// the NLRI involved and containing the entire malformed UPDATE message" -- still holds for
// an UPDATE that is malformed AND emptied by Section 5.4 at the same time.
// PREVENTS: the Section 5.4 early return swallowing the Section 6 record. That return jumps
// the action switch, which is where every other malformed UPDATE is logged, so the one case
// with two things wrong with it was the one case an operator could not trace.
//
// It also pins WHICH bytes are logged: the ones the peer sent, not the rewritten body. The
// Section 5.4 rewrite runs before this point, and a log of ze's own output would answer a
// different question from the one Section 6 asks.
func TestRFC7606Section54EmptiedUpdateStillLogsSection6(t *testing.T) {
	registerTypeLenRecognizer(t, evpnFam, 1, 5)
	buf := captureSessionLog(t, slog.LevelDebug)
	s := nlriTypeTestSession()

	// Every route unrecognized, so the UPDATE empties, AND a two-octet ORIGIN, which
	// RFC 7606 Section 7.1 makes malformed and Section 2 turns into treat-as-withdraw.
	// mpReachAttrs prepends a valid ORIGIN of its own, so the section also carries a
	// duplicate code and drives the Section 3.g keep-first strip. That is deliberate: the
	// received and rewritten bodies then differ twice over, so an assertion on the received
	// hex cannot pass by matching the rewritten one.
	nlri := append(evpnWireNLRI(99, 0xaa), evpnWireNLRI(200, 0xbb)...)
	attrs := append([]byte{0x40, 0x01, 0x02, 0x00, 0x00}, mpReachAttrs(evpnFam, nlri)...)

	body := makeUpdateBody(nil, attrs, nil)
	_, action, err := s.enforceRFC7606(wireu.NewWireUpdate(body, 0))
	require.NoError(t, err)
	require.Equal(t, message.RFC7606ActionTreatAsWithdraw, action,
		"precondition: the UPDATE is both malformed and emptied")

	logged := buf.String()
	assert.Contains(t, logged, "RFC 7606 diagnostics",
		"Section 6 requires the malformed UPDATE to be logged, emptied or not")
	assert.Contains(t, logged, "update-body-hex",
		"Section 6 asks for the entire malformed UPDATE message")
	assert.Contains(t, logged, hex.EncodeToString(body),
		"the body logged must be the one the PEER sent, not the rewritten one")
}
