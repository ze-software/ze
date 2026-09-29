// Design: docs/architecture/wire/messages.md -- ROUTE-REFRESH receive handling
// Related: rfc2918_reserved_field_test.go -- the session setup and the Reserved-octet rules

package reactor

import (
	"net/netip"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/fsm"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestRFC2918RouteRefreshForAnUnadvertisedFamilyIsIgnored proves the receive path ignores
// a ROUTE-REFRESH for an <AFI, SAFI> this speaker did not advertise, and still delivers one
// for a family it did.
//
// RFC 2918 Section 4: "If a BGP speaker receives from its peer a ROUTE-REFRESH message with
// the <AFI, SAFI> that the speaker didn't advertise to the peer at the session establishment
// time via capability advertisement, the speaker shall ignore such a message."
//
// Method: an Established session whose OPEN advertised IPv4/unicast only (no IPv6, no
// capability 70) reads, through ReadAndProcess, a ROUTE-REFRESH for IPv6/unicast and then
// one for IPv4/unicast. "Ignore" is asserted as: no delivery to onMessageReceived (the
// consumer that re-advertises the Adj-RIB-Out), no NOTIFICATION on the wire, and the session
// still Established. The advertised family must reach the consumer byte for byte.
//
// VALIDATES: the family rule is applied before the message is delivered to any consumer.
// PREVENTS: a refresh request for a family the speaker never offered reaching a plugin
// that would act on it.
//
// RFC requirement: RFC2918-4-2 positive -- a ROUTE-REFRESH for IPv6/unicast, which the speaker did not advertise, reaches no onMessageReceived consumer, writes no NOTIFICATION and leaves the session Established.
// RFC requirement: RFC2918-4-2 negative -- a ROUTE-REFRESH for IPv4/unicast, which the speaker advertised, reaches onMessageReceived exactly once with its body unchanged.
func TestRFC2918RouteRefreshForAnUnadvertisedFamilyIsIgnored(t *testing.T) {
	session, client, cleanup := setupEstablishedSessionRFC2918RouteRefresh(t)
	defer cleanup()

	var delivered [][]byte
	session.onMessageReceived = func(_ netip.Addr, msgType msgtype.MessageType, raw []byte, _ *wireu.WireUpdate, _ bgpctx.ContextID, _ rpc.MessageDirection, _ BufHandle, _ map[string]any, _ string, _ uint64) bool {
		if msgType == msgtype.TypeROUTEREFRESH {
			delivered = append(delivered, append([]byte(nil), raw...))
		}
		return false
	}

	// AFI = 2 (IPv6), Reserved = 0, SAFI = 1 (Unicast): not advertised by this speaker.
	unadvertised := []byte{0x00, 0x02, 0x00, 0x01}
	go func() {
		client.Write(buildRouteRefreshMsg(unadvertised)) //nolint:errcheck // test goroutine
	}()
	require.NoError(t, session.ReadAndProcess())
	assert.Empty(t, delivered, "a refresh for an unadvertised family must not reach any consumer")
	assert.Equal(t, fsm.StateEstablished, session.State())

	// net.Pipe is unbuffered, so a NOTIFICATION written for the ignored message would be
	// read here instead of the deadline expiring.
	require.NoError(t, client.SetReadDeadline(time.Now().Add(200*time.Millisecond)))
	buf := make([]byte, 4096)
	n, readErr := client.Read(buf)
	require.ErrorIs(t, readErr, os.ErrDeadlineExceeded,
		"an ignored ROUTE-REFRESH must draw no reply. Got %d byte(s): %x", n, buf[:max(n, 0)])
	require.NoError(t, client.SetReadDeadline(time.Time{}))

	// AFI = 1 (IPv4), Reserved = 0, SAFI = 1 (Unicast): advertised by this speaker.
	advertised := []byte{0x00, 0x01, 0x00, 0x01}
	go func() {
		client.Write(buildRouteRefreshMsg(advertised)) //nolint:errcheck // test goroutine
	}()
	require.NoError(t, session.ReadAndProcess())
	require.Len(t, delivered, 1, "a refresh for an advertised family must reach the consumer")
	assert.Equal(t, advertised, delivered[0])
	assert.Equal(t, fsm.StateEstablished, session.State())
}
