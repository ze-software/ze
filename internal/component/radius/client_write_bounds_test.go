// Design: docs/research/l2tpv2-ze-integration.md -- bounded RADIUS request I/O
// Related: client_write.go, client_observed.go -- actual resolver and socket boundaries
package radius

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func boundaryRequest(t *testing.T, client *Client) []byte {
	t.Helper()
	var storage [MaxPacketLen]byte
	length, _, err := encodeRequest(&Packet{Code: CodeAccountingReq, Identifier: client.NextID()}, []byte("observed-write"), storage[:])
	require.NoError(t, err)
	return storage[:length]
}

// boundaryLiveWrite observes a real datagram from this same client, so a
// refusal test cannot pass merely because its socket or endpoint never worked.
func boundaryLiveWrite(t *testing.T, client *Client, address string, seen <-chan *Packet) {
	t.Helper()
	destination, err := net.ResolveUDPAddr("udp4", address)
	require.NoError(t, err)
	data := boundaryRequest(t, client)
	var receipt RequestWrite
	var observations int
	err = client.writeRequest(t.Context(), data, destination, func(write RequestWrite) {
		receipt = write
		observations++
	})
	require.NoError(t, err)
	require.Equal(t, 1, observations)
	require.Equal(t, address, receipt.Server())
	require.Equal(t, len(data), receipt.Bytes())
	require.NoError(t, receipt.Err())
	select {
	case packet := <-seen:
		require.Equal(t, data[1], packet.Identifier)
	case <-time.After(5 * time.Second):
		t.Fatal("live control did not reach the original UDP endpoint")
	}
}

// Missing API inputs refuse before writing; the same live endpoint accepts
// a complete observed request and returns its authenticated accounting reply.
func TestObservedRequestRequiresContextAndObserver(t *testing.T) {
	address, seen := observedWriteServer(t, true)
	client := observedWriteClient(t, address)
	var observations int
	response, err := client.SendToServersObserved(nil, &Packet{Code: CodeAccountingReq}, func(RequestWrite) { observations++ }) //nolint:staticcheck // SA1012: deliberately nil to prove refusal before any write.
	require.ErrorContains(t, err, "context")
	require.Nil(t, response)
	response, err = client.SendToServersObserved(t.Context(), &Packet{Code: CodeAccountingReq}, nil)
	require.ErrorContains(t, err, "observer")
	require.Nil(t, response)
	require.Zero(t, observations)
	response, err = client.SendToServersObserved(t.Context(), &Packet{Code: CodeAccountingReq}, func(RequestWrite) { observations++ })
	require.NoError(t, err)
	require.Equal(t, uint8(CodeAccountingResp), response.Code)
	require.Equal(t, 1, observations)
	select {
	case packet := <-seen:
		require.Equal(t, response.Identifier, packet.Identifier)
	case <-time.After(5 * time.Second):
		t.Fatal("valid observed control did not reach its UDP endpoint")
	}
	select {
	case packet := <-seen:
		t.Fatalf("missing-input request unexpectedly wrote: %+v", packet)
	default:
	}
}

// Malformed destinations and IPv6 cannot become a guessed UDP4 address.
// The zone literal reaches LookupIPAddr's real literal fast path, producing
// only IPv6 without a DNS query; the NUL name is locally invalid as a host.
func TestRequestAddressRefusesMalformedAndNonIPv4Destinations(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	for _, input := range []string{"127.0.0.1", "127.0.0.1:65536", "[::1]:1812", "[::1%radius-proof]:1812", "bad\x00host:1812"} {
		t.Run(input, func(t *testing.T) {
			address, err := resolveRequestAddress(ctx, input)
			require.Error(t, err)
			require.Nil(t, address)
		})
	}
	address, err := resolveRequestAddress(ctx, "127.0.0.1:1812")
	require.NoError(t, err)
	require.True(t, address.IP.Equal(net.IPv4(127, 0, 0, 1)))
	require.Equal(t, 1812, address.Port)
	require.NoError(t, ctx.Err())
}

// An explicit empty host preserves the wildcard, while localhost traverses
// the real system resolver and still returns an IPv4 loopback destination.
func TestRequestAddressPreservesWildcardAndResolvesLocalhost(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	wildcard, err := resolveRequestAddress(ctx, ":1812")
	require.NoError(t, err)
	require.Nil(t, wildcard.IP)
	require.Equal(t, 1812, wildcard.Port)
	local, err := resolveRequestAddress(ctx, "localhost:1812")
	require.NoError(t, err)
	require.NotNil(t, local.IP.To4())
	require.True(t, local.IP.IsLoopback())
	require.Equal(t, 1812, local.Port)
	require.NoError(t, ctx.Err())
}

// A canceled operation authorizes neither address selection nor a native
// write receipt. A later live write uses the same original socket and peer.
func TestCanceledRequestRefusesResolutionAndNativeWrite(t *testing.T) {
	address, seen := observedWriteServer(t, true)
	client := observedWriteClient(t, address)
	destination, err := net.ResolveUDPAddr("udp4", address)
	require.NoError(t, err)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	resolved, err := resolveRequestAddress(ctx, address)
	require.ErrorIs(t, err, context.Canceled)
	require.Nil(t, resolved)
	var observations int
	err = client.writeRequest(ctx, boundaryRequest(t, client), destination, func(RequestWrite) { observations++ })
	require.ErrorIs(t, err, context.Canceled)
	require.Zero(t, observations)
	boundaryLiveWrite(t, client, address, seen)
}

// Close joins the actual answer reader, and the closed-client guard refuses
// another write without inventing an outcome for an unattempted socket call.
func TestClosedClientRefusesNativeWriteWithoutReceipt(t *testing.T) {
	address, seen := observedWriteServer(t, true)
	client := observedWriteClient(t, address)
	boundaryLiveWrite(t, client, address, seen)
	require.NoError(t, client.Close())
	select {
	case <-client.done:
	default:
		t.Fatal("Close did not join the original answer reader")
	}
	destination, err := net.ResolveUDPAddr("udp4", address)
	require.NoError(t, err)
	var observations int
	err = client.writeRequest(t.Context(), boundaryRequest(t, client), destination, func(RequestWrite) { observations++ })
	require.ErrorContains(t, err, "client closed")
	require.Zero(t, observations)
}

// UDP destination port zero makes the real socket reject the write. Its one
// receipt must retain that native failure; the same socket then sends normally.
func TestNativeWriteFailureRetainsActualErrorReceipt(t *testing.T) {
	address, seen := observedWriteServer(t, true)
	client := observedWriteClient(t, address)
	destination := &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)}
	var receipt RequestWrite
	var observations int
	err := client.writeRequest(t.Context(), boundaryRequest(t, client), destination, func(write RequestWrite) {
		receipt = write
		observations++
	})
	require.Error(t, err)
	require.Equal(t, 1, observations)
	require.Equal(t, destination.String(), receipt.Server())
	require.Zero(t, receipt.Bytes())
	require.Error(t, receipt.Err())
	require.ErrorIs(t, err, receipt.Err())
	var operation *net.OpError
	require.ErrorAs(t, receipt.Err(), &operation)
	require.Equal(t, "write", operation.Op)
	boundaryLiveWrite(t, client, address, seen)
}

// Deliberately closing this test's own native socket forces SetWriteDeadline
// to fail before any write. Cleanup checks that error and joins the real reader.
func TestNativeDeadlineFailureHasNoWriteReceiptAndJoinsReader(t *testing.T) {
	address, seen := observedWriteServer(t, true)
	client, err := NewClient(ClientConfig{Timeout: time.Second, Retries: 1})
	require.NoError(t, err)
	nativeClosed := false
	t.Cleanup(func() {
		closeErr := client.Close()
		if nativeClosed {
			require.ErrorIs(t, closeErr, net.ErrClosed)
		} else {
			require.NoError(t, closeErr)
		}
		select {
		case <-client.done:
		default:
			t.Error("cleanup did not join the original answer reader")
		}
	})
	boundaryLiveWrite(t, client, address, seen)
	require.NoError(t, client.conn.Close())
	nativeClosed = true
	destination, err := net.ResolveUDPAddr("udp4", address)
	require.NoError(t, err)
	var observations int
	err = client.writeRequest(t.Context(), boundaryRequest(t, client), destination, func(RequestWrite) { observations++ })
	require.ErrorIs(t, err, net.ErrClosed)
	require.Zero(t, observations)
}
