// Design: docs/guide/radius.md -- an accounting exchange requires Accounting-Response.
// Related: client.go -- Client.exchange processes authenticated replies for a pending request.

package radius

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestAccountingExchangeWaitsForAccountingResponse proves an authenticated
// wrong-kind reply leaves the same request live for the proper answer.
// MUTATION: let Client.exchange return the first decoded reply to Accounting-Request.
func TestAccountingExchangeWaitsForAccountingResponse(t *testing.T) {
	secret := []byte("accounting-kind-fixture")
	server := newScriptedServer(t, func(request []byte) [][]byte {
		return [][]byte{unsignedAnswer(CodeAccessAccept, request, secret), acctResponse(request, secret)}
	})
	client, _ := blastClient(t, nil, 3*time.Second, 1)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	request := &Packet{
		Code:       CodeAccountingReq,
		Identifier: client.NextID(),
		Attrs: []Attr{
			{Type: AttrAcctStatusType, Value: AttrUint32(AcctStatusStop)},
			{Type: AttrAcctSessionID, Value: AttrString("accounting-kind-session")},
		},
	}
	response, err := client.Exchange(ctx, request, Server{Address: server.addr, SharedKey: secret})
	require.NoError(t, err)
	require.Equal(t, uint8(CodeAccountingResp), response.Code, "only Accounting-Response can acknowledge the pending Stop")
	require.Len(t, server.received(), 1, "the correct reply completed the original datagram's wait")
}
