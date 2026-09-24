// Design: docs/architecture/wire/nlri-bgpls.md -- native EPE label ownership

package ls_export

import (
	"context"
	"errors"
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/plugins/epe"
	"github.com/ze-software/ze/internal/core/mplsfib"
)

// TestNativeEPEReassignmentWaitsForRemoval keeps a failed old-label removal
// pending while another live peer requests that label. Neither forwarding nor
// the collector may claim the new peer until the old assignment is removed.
func TestNativeEPEReassignmentWaitsForRemoval(t *testing.T) {
	exporter, capture := exportFixture(t)
	bus := &epeProofBus{exporter: exporter, labels: make(map[uint32]netip.Addr)}
	source := epe.NewSource(bus)
	oldPeer := netip.MustParseAddr("198.51.100.2")
	newPeer := netip.MustParseAddr("198.51.100.3")
	require.NoError(t, source.Configure(epe.Config{Base: 16000, Size: 100, Peers: map[netip.Addr]epe.PeerConfig{oldPeer: {Index: 7}}}))
	require.NoError(t, source.State(epeProofEvent(t, "up")))
	require.NoError(t, exporter.reconcile(context.Background()))
	require.Len(t, capture.commands, 2)
	event := epeProofEvent(t, "up")
	event.Peer = []byte(strings.ReplaceAll(string(event.Peer), "198.51.100.2", "198.51.100.3"))
	require.NoError(t, source.State(event))
	bus.reject = errors.New("old label removal failed")
	bus.rejectAction = mplsfib.ActionRemove
	require.Error(t, source.Configure(epe.Config{Base: 16000, Size: 100, Peers: map[netip.Addr]epe.PeerConfig{newPeer: {Index: 7}}}))
	require.Equal(t, map[uint32]netip.Addr{16007: oldPeer}, bus.labels)
	require.NoError(t, exporter.reconcile(context.Background()))
	require.Len(t, capture.commands, 4)
	require.Contains(t, capture.commands[2], " del ")
	require.Contains(t, capture.commands[3], " del ")
	bus.reject = nil
	require.NoError(t, source.Replay())
	require.Equal(t, map[uint32]netip.Addr{16007: newPeer}, bus.labels)
	require.NoError(t, exporter.reconcile(context.Background()))
	require.Len(t, capture.commands, 6)
	link := exportCommandBytes(t, capture.commands[5], "nlri")
	require.Equal(t, [][]byte{{198, 51, 100, 3}}, exportTLVValues(t, link[13:], 260))
	require.NoError(t, source.Stop())
	require.Empty(t, bus.labels)
	require.NoError(t, exporter.reconcile(context.Background()))
	require.Len(t, capture.commands, 8)
}

// A new producer has no copy of the old source's installed map. Cleanup must
// therefore consult the persistent native owner before claiming the same label.
func TestNativeEPERecreatedSourceRetiresFailedOwnership(t *testing.T) {
	exporter, capture := exportFixture(t)
	bus := &epeProofBus{exporter: exporter, labels: make(map[uint32]netip.Addr)}
	oldPeer := netip.MustParseAddr("198.51.100.2")
	newPeer := netip.MustParseAddr("198.51.100.3")
	oldSource := epe.NewSource(bus)
	require.NoError(t, oldSource.Configure(epe.Config{Base: 16000, Size: 100, Peers: map[netip.Addr]epe.PeerConfig{oldPeer: {Index: 7}}}))
	require.NoError(t, oldSource.State(epeProofEvent(t, "up")))
	require.NoError(t, exporter.reconcile(context.Background()))
	bus.reject, bus.rejectAction = errors.New("kernel removal failed"), mplsfib.ActionRemove
	require.Error(t, oldSource.Stop())
	require.NoError(t, exporter.reconcile(context.Background()))
	require.Len(t, capture.commands, 4)
	require.Equal(t, map[uint32]netip.Addr{16007: oldPeer}, bus.labels)

	recreated := epe.NewSource(bus)
	require.Error(t, recreated.Configure(epe.Config{Base: 16000, Size: 100, Peers: map[netip.Addr]epe.PeerConfig{newPeer: {Index: 7}}}))
	event := epeProofEvent(t, "up")
	event.Peer = []byte(strings.ReplaceAll(string(event.Peer), oldPeer.String(), newPeer.String()))
	require.Error(t, recreated.State(event))
	require.NoError(t, exporter.reconcile(context.Background()))
	require.Len(t, capture.commands, 4, "the recreated source must not advertise an uncleared assignment")
	require.Equal(t, map[uint32]netip.Addr{16007: oldPeer}, bus.labels)

	bus.reject = nil
	require.NoError(t, recreated.Replay())
	require.Equal(t, map[uint32]netip.Addr{16007: newPeer}, bus.labels)
	require.NoError(t, exporter.reconcile(context.Background()))
	require.Len(t, capture.commands, 6)
	link := exportCommandBytes(t, capture.commands[5], "nlri")
	require.Equal(t, [][]byte{{198, 51, 100, 3}}, exportTLVValues(t, link[13:], 260))
	require.NoError(t, recreated.Stop())
	require.Empty(t, bus.labels)
}

func TestNativeEPERetiredSourceCannotResetReplacement(t *testing.T) {
	exporter, capture := exportFixture(t)
	bus := &epeProofBus{exporter: exporter, labels: make(map[uint32]netip.Addr)}
	peer := netip.MustParseAddr("198.51.100.2")
	config := epe.Config{Base: 16000, Size: 100, Peers: map[netip.Addr]epe.PeerConfig{peer: {Index: 7}}}
	oldSource := epe.NewSource(bus)
	require.NoError(t, oldSource.Configure(config))
	require.NoError(t, oldSource.State(epeProofEvent(t, "up")))
	require.NoError(t, oldSource.Stop())
	replacement := epe.NewSource(bus)
	require.NoError(t, replacement.Configure(config))
	require.NoError(t, replacement.State(epeProofEvent(t, "up")))
	require.NoError(t, exporter.reconcile(context.Background()))
	require.Len(t, capture.commands, 2)

	// SDK bye succeeds before Run returns and its deferred stop executes.
	require.NoError(t, oldSource.Stop())
	require.NoError(t, oldSource.Replay())
	require.Equal(t, map[uint32]netip.Addr{16007: peer}, bus.labels)
	require.NoError(t, exporter.reconcile(context.Background()))
	require.Len(t, capture.commands, 2, "retired callbacks must not withdraw the replacement's SID")
	require.NoError(t, replacement.Stop())
}
