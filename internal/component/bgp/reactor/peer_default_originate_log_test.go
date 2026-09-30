package reactor

import (
	"log/slog"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/test/sim"
)

// TestDefaultOriginateRefusalIsAWarnNamingPeerAndPrefix pins what the operator
// sees when the default route this peer is configured to receive cannot be
// built.
//
// VALIDATES: an IPv4 default-originate whose only next hop is an IPv6 local
// address, on a session without Extended Next Hop, is refused by BuildUnicast
// (RFC 4271 Section 5: an UPDATE carrying NLRI carries NEXT_HOP) and the
// refusal is one Warn line naming the peer, the prefix 0.0.0.0/0 and the
// family, with nothing written to the session. A second refusal inside the
// one-second window is swallowed and counted; the next line after the window
// reports it through suppressed-since-last.
// PREVENTS: the refusal staying at Debug, where a default-level daemon shows
// nothing and the operator sees a peer that silently lacks its default route;
// and the opposite failure, a flapping peer turning the line into a flood.
func TestDefaultOriginateRefusalIsAWarnNamingPeerAndPrefix(t *testing.T) {
	var sink syncBuffer
	defer swapRoutesLogger(slog.New(slog.NewTextHandler(&sink, &slog.HandlerOptions{Level: slog.LevelWarn})))()

	peer, conn := newInitialSyncPeer(t, true, family.IPv4Unicast)
	peer.settings.LocalAddress = netip.MustParseAddr("2001:db8::1")
	peer.settings.DefaultOriginate = map[string]bool{"ipv4/unicast": true}
	fc := sim.NewFakeClock(time.Now())
	peer.SetClock(fc)
	nc := peer.negotiated.Load()
	require.NotNil(t, nc)

	peer.sendDefaultOriginateRoutes(nc)

	assert.Empty(t, conn.written(), "a refused default route must not reach the wire")
	lines := warnLines(sink.String())
	require.Len(t, lines, 1, "the refusal is one Warn line: %q", sink.String())
	assert.Contains(t, lines[0], "level=WARN")
	assert.Contains(t, lines[0], "peer=10.0.0.2")
	assert.Contains(t, lines[0], "prefix=0.0.0.0/0")
	assert.Contains(t, lines[0], "family=ipv4/unicast")
	assert.Contains(t, lines[0], "next-hop=2001:db8::1")

	peer.sendDefaultOriginateRoutes(nc)
	assert.Len(t, warnLines(sink.String()), 1, "a second refusal inside the window is swallowed")

	fc.Add(time.Second)
	peer.sendDefaultOriginateRoutes(nc)
	lines = warnLines(sink.String())
	require.Len(t, lines, 2, "the window has passed, so the refusal is said again")
	assert.Contains(t, lines[1], "suppressed-since-last=1", "the swallowed refusal is reported")
	assert.Empty(t, conn.written())
}

// warnLines returns the non-empty lines of a text-handler sink.
func warnLines(out string) []string {
	var lines []string
	for line := range strings.SplitSeq(out, "\n") {
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}
