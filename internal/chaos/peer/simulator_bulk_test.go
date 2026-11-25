package peer

import (
	"context"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/chaos/scenario"
	"github.com/ze-software/ze/internal/component/bgp/message"
)

// VALIDATES: Bulk initial advertisements pack every seeded prefix into
// bounded UPDATEs and report each prefix, with one capture per wire message.
// PREVENTS: Large seeded workloads sending one UPDATE per prefix, missing
// prefixes, or writing a second capture record for the same UPDATE.
func TestSimulatorPacksBulkSeededRoutes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ln, err := (&net.ListenConfig{}).Listen(ctx, "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = ln.Close() })

	const seed = 42
	profile := SimProfile{
		ASN:        65002,
		RouterID:   netip.MustParseAddr("10.255.0.2"),
		HoldTime:   90,
		RouteCount: 4400,
		TotalPeers: 64,
		Families:   []string{familyIPv4Unicast, familyIPv6Unicast, familyIPv4Multicast, familyIPv6Multicast},
	}
	zeResult := make(chan int, 1)
	go fakeZePeer(t, ln, zeResult, profile.Families...)
	count := profile.RouteCount
	peers := profile.TotalPeers
	want := map[string][]netip.Prefix{
		familyIPv4Unicast:   scenario.GenerateIPv4Routes(seed, 0, count, peers),
		familyIPv6Unicast:   scenario.GenerateIPv6Routes(seed, 0, count, peers),
		familyIPv4Multicast: scenario.GenerateIPv4MulticastRoutes(seed, 0, count/4, peers),
		familyIPv6Multicast: scenario.GenerateIPv6MulticastRoutes(seed, 0, count/4, peers),
	}
	events := make(chan Event, 256)
	go func() {
		RunSimulator(ctx, SimulatorConfig{Profile: profile, Seed: seed, Addr: ln.Addr().String(), Events: events, Quiet: true})
		close(events)
	}()

	wire := newEventBuffer()
	packets := make(map[string]int)
	sent := make(map[string][]netip.Prefix)
	eorCount := -1
	for event := range events {
		if event.Type == EventError {
			t.Fatalf("simulator error: %v", event.Err)
		}
		if event.Type == EventEORSent {
			eorCount = event.Count
			cancel()
		}
		if event.Type != EventRouteSent {
			continue
		}
		sent[event.Family] = append(sent[event.Family], event.Prefix)
		require.Equal(t, int64(len(event.BGPMessage)), event.BytesSent)
		if len(event.BGPMessage) == 0 {
			continue
		}
		packets[event.Family]++
		require.LessOrEqual(t, len(event.BGPMessage), message.MaxMsgLen)
		parseUpdatePrefixes(event.BGPMessage[message.HeaderLen:], 0, wire)
	}

	encoded := make(map[string][]netip.Prefix)
	for _, event := range wire.items {
		encoded[event.Family] = append(encoded[event.Family], event.Prefix)
	}
	assert.Equal(t, want, sent)
	assert.Equal(t, want, encoded)
	assert.Equal(t, count*2+count/2, eorCount)
	totalPackets := 0
	for _, name := range profile.Families {
		assert.Greater(t, packets[name], 1, name)
		assert.Less(t, packets[name], 20, name)
		totalPackets += packets[name]
	}
	assert.Equal(t, totalPackets+len(profile.Families), <-zeResult)
}
