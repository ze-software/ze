// Design: docs/architecture/wire/nlri-bgpls.md -- native EPE label ownership

package ls

import (
	"context"
	"errors"
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/mplsfib"
)

// TestNativeEPEReassignmentWaitsForRemoval keeps a failed old-label removal
// pending while another live peer requests that label. Neither forwarding nor
// the collector may claim the new peer until the old assignment is removed.
func TestNativeEPEReassignmentWaitsForRemoval(t *testing.T) {
	exporter, capture := exportFixture(t)
	bus := &epeProofBus{exporter: exporter, labels: make(map[uint32]netip.Addr)}
	source := newEPESource(bus)
	oldPeer := netip.MustParseAddr("198.51.100.2")
	newPeer := netip.MustParseAddr("198.51.100.3")
	require.NoError(t, source.configure(epeConfig{base: 16000, size: 100, peers: map[netip.Addr]epePeerConfig{oldPeer: {index: 7}}}))
	require.NoError(t, source.state(epeProofEvent(t, "up")))
	require.NoError(t, exporter.reconcile(context.Background()))
	require.Len(t, capture.commands, 2)
	event := epeProofEvent(t, "up")
	event.Peer = []byte(strings.ReplaceAll(string(event.Peer), "198.51.100.2", "198.51.100.3"))
	require.NoError(t, source.state(event))
	bus.reject = errors.New("old label removal failed")
	bus.rejectAction = mplsfib.ActionRemove
	require.Error(t, source.configure(epeConfig{base: 16000, size: 100, peers: map[netip.Addr]epePeerConfig{newPeer: {index: 7}}}))
	require.Equal(t, map[uint32]netip.Addr{16007: oldPeer}, bus.labels)
	require.NoError(t, exporter.reconcile(context.Background()))
	require.Len(t, capture.commands, 4)
	require.Contains(t, capture.commands[2], " del ")
	require.Contains(t, capture.commands[3], " del ")
	bus.reject = nil
	require.NoError(t, source.replay())
	require.Equal(t, map[uint32]netip.Addr{16007: newPeer}, bus.labels)
	require.NoError(t, exporter.reconcile(context.Background()))
	require.Len(t, capture.commands, 6)
	link := exportCommandBytes(t, capture.commands[5], "nlri")
	require.Equal(t, [][]byte{{198, 51, 100, 3}}, exportTLVValues(t, link[13:], 260))
	require.NoError(t, source.stop())
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
	oldSource := newEPESource(bus)
	require.NoError(t, oldSource.configure(epeConfig{base: 16000, size: 100, peers: map[netip.Addr]epePeerConfig{oldPeer: {index: 7}}}))
	require.NoError(t, oldSource.state(epeProofEvent(t, "up")))
	require.NoError(t, exporter.reconcile(context.Background()))
	bus.reject, bus.rejectAction = errors.New("kernel removal failed"), mplsfib.ActionRemove
	require.Error(t, oldSource.stop())
	require.NoError(t, exporter.reconcile(context.Background()))
	require.Len(t, capture.commands, 4)
	require.Equal(t, map[uint32]netip.Addr{16007: oldPeer}, bus.labels)

	recreated := newEPESource(bus)
	require.Error(t, recreated.configure(epeConfig{base: 16000, size: 100, peers: map[netip.Addr]epePeerConfig{newPeer: {index: 7}}}))
	event := epeProofEvent(t, "up")
	event.Peer = []byte(strings.ReplaceAll(string(event.Peer), oldPeer.String(), newPeer.String()))
	require.Error(t, recreated.state(event))
	require.NoError(t, exporter.reconcile(context.Background()))
	require.Len(t, capture.commands, 4, "the recreated source must not advertise an uncleared assignment")
	require.Equal(t, map[uint32]netip.Addr{16007: oldPeer}, bus.labels)

	bus.reject = nil
	require.NoError(t, recreated.replay())
	require.Equal(t, map[uint32]netip.Addr{16007: newPeer}, bus.labels)
	require.NoError(t, exporter.reconcile(context.Background()))
	require.Len(t, capture.commands, 6)
	link := exportCommandBytes(t, capture.commands[5], "nlri")
	require.Equal(t, [][]byte{{198, 51, 100, 3}}, exportTLVValues(t, link[13:], 260))
	require.NoError(t, recreated.stop())
	require.Empty(t, bus.labels)
}

func TestNativeEPERetiredSourceCannotResetReplacement(t *testing.T) {
	exporter, capture := exportFixture(t)
	bus := &epeProofBus{exporter: exporter, labels: make(map[uint32]netip.Addr)}
	peer := netip.MustParseAddr("198.51.100.2")
	config := epeConfig{base: 16000, size: 100, peers: map[netip.Addr]epePeerConfig{peer: {index: 7}}}
	oldSource := newEPESource(bus)
	require.NoError(t, oldSource.configure(config))
	require.NoError(t, oldSource.state(epeProofEvent(t, "up")))
	require.NoError(t, oldSource.stop())
	replacement := newEPESource(bus)
	require.NoError(t, replacement.configure(config))
	require.NoError(t, replacement.state(epeProofEvent(t, "up")))
	require.NoError(t, exporter.reconcile(context.Background()))
	require.Len(t, capture.commands, 2)

	// SDK bye succeeds before Run returns and its deferred stop executes.
	require.NoError(t, oldSource.stop())
	require.NoError(t, oldSource.replay())
	require.Equal(t, map[uint32]netip.Addr{16007: peer}, bus.labels)
	require.NoError(t, exporter.reconcile(context.Background()))
	require.Len(t, capture.commands, 2, "retired callbacks must not withdraw the replacement's SID")
	require.NoError(t, replacement.stop())
}
