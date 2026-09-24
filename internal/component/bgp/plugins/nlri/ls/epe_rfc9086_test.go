// Design: docs/architecture/wire/nlri-bgpls.md -- native EPE lifecycle proof
// RFC: rfc/short/rfc9086.md -- PeerNode index and SRGB coupling

package ls

import (
	"context"
	"encoding/binary"
	"errors"
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/internal/component/bgp"
	"github.com/ze-software/ze/internal/core/linkstateevents"
	"github.com/ze-software/ze/internal/core/mplsfib"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

type epeProofBus struct {
	exporter     *topologyExporter
	labels       map[uint32]netip.Addr
	reject       error
	rejectAction mplsfib.Action
}

func (b *epeProofBus) Emit(namespace, _ string, payload any) (int, error) {
	if namespace == mplsfib.Namespace {
		batch := payload.(*mplsfib.EntryBatch)
		rejected := b.reject
		action := batch.Entries[0].Action
		if action == mplsfib.ActionRemoveLabelSource {
			action = mplsfib.ActionRemove
			if len(b.labels) == 0 {
				batch.Acknowledge(nil)
				return 0, nil
			}
		}
		if b.rejectAction != mplsfib.ActionUnspecified && action != b.rejectAction {
			rejected = nil
		}
		if rejected == nil {
			for _, entry := range batch.Entries {
				switch entry.Action {
				case mplsfib.ActionAdd:
					b.labels[entry.InLabel] = entry.NextHop
				case mplsfib.ActionRemoveLabelSource:
					clear(b.labels)
				case mplsfib.ActionRemove:
					delete(b.labels, entry.InLabel)
				default:
					panic("BUG: the exporter emitted an unspecified MPLS FIB action")
				}
			}
		}
		if batch.Acknowledge != nil {
			batch.Acknowledge(rejected)
		}
		return 0, nil
	}
	return 0, b.exporter.replace(namespace, payload.(*linkstateevents.Snapshot))
}

func (*epeProofBus) Subscribe(string, string, func(any)) func() { return func() {} }

func epeProofEvent(t *testing.T, state string) *bgp.Event {
	t.Helper()
	event, err := bgp.ParseEvent([]byte(`{"type":"state","state":"` + state + `","peer":{"router-id":"192.0.2.1","local":{"address":"198.51.100.1","as":65000},"remote":{"address":"198.51.100.2","as":65001,"router-id":"192.0.2.2"}}}`))
	require.NoError(t, err)
	return event
}

// VALIDATES: an acknowledged native PeerNode label is advertised as an index
// with its actual local SRGB, and a session loss withdraws both route and label.
// PREVENTS: an index with no resolvable SRGB or a SID surviving session teardown.
func TestRFC9086NativePeerIndexAdvertisesSRGB(t *testing.T) {
	// RFC requirement: RFC9086-5-10 positive -- real configured SRGB and live BGP state produce an installed PeerNode label plus matching advertised SRGB.
	// RFC requirement: RFC9086-3-1 positive -- an enabled live session produces exactly one installed PeerNode SID advertisement.
	// RFC requirement: RFC9086-3-2 positive -- the collector receives one PeerNode SID for the configured session, not one per SRGB label.
	// RFC requirement: RFC9086-5-1 positive -- the emitted EPE Link includes the actual assigned PeerNode SID.
	// RFC requirement: RFC9086-4.2-1 positive -- collector-visible local descriptors contain the live BGP identifier and AS.
	// RFC requirement: RFC9086-4.2-2 positive -- collector-visible remote descriptors contain the live BGP identifier and AS.
	exporter, capture := exportFixture(t)
	bus := &epeProofBus{exporter: exporter, labels: make(map[uint32]netip.Addr)}
	source := newEPESource(bus)
	peer := netip.MustParseAddr("198.51.100.2")
	require.NoError(t, source.configure(epeConfig{base: 16000, size: 100, peers: map[netip.Addr]epePeerConfig{peer: {index: 7, weight: 5}}}))
	require.NoError(t, source.state(epeProofEvent(t, "up")))
	require.Equal(t, map[uint32]netip.Addr{16007: peer}, bus.labels)
	require.NoError(t, exporter.reconcile(context.Background()))
	require.Len(t, capture.commands, 2)
	nodeAttrs := exportCommandBytes(t, capture.commands[0], "attr")
	linkAttrs := exportCommandBytes(t, capture.commands[1], "attr")
	blocks := exportTLVValues(t, nodeAttrs[11:], 1034)
	require.Equal(t, [][]byte{{0, 0, 0, 0, 100, 4, 137, 0, 3, 0, 62, 128}}, blocks)
	sids := exportTLVValues(t, linkAttrs[11:], 1101)
	require.Equal(t, [][]byte{{0x10, 5, 0, 0, 0, 0, 0, 7}}, sids)
	link := exportCommandBytes(t, capture.commands[1], "nlri")
	require.Equal(t, uint16(2), binary.BigEndian.Uint16(link))
	require.Equal(t, byte(7), link[4])
	local := exportTLVValues(t, link[13:], 256)
	remote := exportTLVValues(t, link[13:], 257)
	require.Equal(t, [][]byte{{192, 0, 2, 1}}, exportTLVValues(t, local[0], 516))
	require.Equal(t, [][]byte{{192, 0, 2, 2}}, exportTLVValues(t, remote[0], 516))
	require.Equal(t, [][]byte{{0, 0, 253, 232}}, exportTLVValues(t, local[0], 512))
	require.Equal(t, [][]byte{{0, 0, 253, 233}}, exportTLVValues(t, remote[0], 512))
	require.NoError(t, source.state(epeProofEvent(t, "down")))
	require.Empty(t, bus.labels)
	require.NoError(t, exporter.reconcile(context.Background()))
	require.Len(t, capture.commands, 4)
	require.Contains(t, capture.commands[2], " del ")
	require.Contains(t, capture.commands[3], " del ")
	require.Equal(t, uint16(2), binary.BigEndian.Uint16(exportCommandBytes(t, capture.commands[2], "nlri")))
	require.Equal(t, uint16(1), binary.BigEndian.Uint16(exportCommandBytes(t, capture.commands[3], "nlri")))
}

// VALIDATES: an index-only source without an advertised SRGB is refused before
// any BGP UPDATE is emitted.
// PREVENTS: an EPE PeerNode SID whose index the collector cannot resolve.
func TestRFC9086NativePeerIndexRequiresSRGB(t *testing.T) {
	// RFC requirement: RFC9086-5-10 negative -- an index PeerNode without local SRGB never reaches the emitter.
	exporter, capture := exportFixture(t)
	local := linkstateevents.NodeID{ASN: 65000, BGPRouterID: netip.MustParseAddr("192.0.2.1")}
	remote := linkstateevents.NodeID{ASN: 65001, BGPRouterID: netip.MustParseAddr("192.0.2.2")}
	snapshot := &linkstateevents.Snapshot{Domain: linkstateevents.Domain{Protocol: linkstateevents.BGP},
		Nodes: []linkstateevents.Node{{ID: local}}, Links: []linkstateevents.Link{{Local: local, Remote: remote,
			Attributes: []linkstateevents.TLV{{Type: 1101, Value: []byte{0x10, 0, 0, 0, 0, 0, 0, 7}}}}}}
	require.Error(t, exporter.replace(epeName, snapshot))
	require.NoError(t, exporter.reconcile(context.Background()))
	require.Empty(t, capture.commands)
}

// VALIDATES: a FIB rejection withdraws previous SID advertisements while failed
// replacement labels remain absent, including a successful old-label removal.
// PREVENTS: reporting a usable EPE SID after the forwarding owner refused it.
func TestNativeEPEInstallFailureDoesNotAdvertiseSID(t *testing.T) {
	exporter, capture := exportFixture(t)
	bus := &epeProofBus{exporter: exporter, labels: make(map[uint32]netip.Addr), reject: errors.New("MPLS install rejected")}
	source := newEPESource(bus)
	peer := netip.MustParseAddr("198.51.100.2")
	require.NoError(t, source.configure(epeConfig{base: 16000, size: 100, peers: map[netip.Addr]epePeerConfig{peer: {index: 7}}}))
	require.Error(t, source.state(epeProofEvent(t, "up")))
	require.NoError(t, exporter.reconcile(context.Background()))
	require.Empty(t, capture.commands)
	require.Empty(t, bus.labels)
	bus.reject = nil
	require.NoError(t, source.replay())
	require.NoError(t, exporter.reconcile(context.Background()))
	require.Len(t, capture.commands, 2)
	bus.reject = errors.New("replacement label rejected")
	require.Error(t, source.configure(epeConfig{base: 17000, size: 100, peers: map[netip.Addr]epePeerConfig{peer: {index: 7}}}))
	require.NoError(t, exporter.reconcile(context.Background()))
	require.Len(t, capture.commands, 4)
	require.Contains(t, capture.commands[2], " del ")
	require.Contains(t, capture.commands[3], " del ")
	require.Equal(t, map[uint32]netip.Addr{16007: peer}, bus.labels, "failed deletion remains tracked for retry")
	bus.rejectAction = mplsfib.ActionAdd
	require.Error(t, source.replay())
	require.Empty(t, bus.labels, "old removal succeeded but the replacement installation failed")
	require.NoError(t, exporter.reconcile(context.Background()))
	require.Len(t, capture.commands, 4, "no SID may be re-advertised without its forwarding label")
	bus.reject = nil
	require.NoError(t, source.replay())
	require.Equal(t, map[uint32]netip.Addr{17007: peer}, bus.labels)
	require.NoError(t, exporter.reconcile(context.Background()))
	require.Len(t, capture.commands, 6)
}

// VALIDATES: incomplete live BGP identities never create forwarding labels or
// ambiguous local/remote Node Descriptors in the native advertisement.
// PREVENTS: silently encoding AS zero or an absent remote BGP identifier.
func TestRFC9086NativePeerRequiresNodeIdentities(t *testing.T) {
	// RFC requirement: RFC9086-4.2-1 negative -- missing local ASN refuses native origination before installing a label.
	// RFC requirement: RFC9086-4.2-2 negative -- missing remote BGP identifier refuses native origination before installing a label.
	for _, identity := range []struct{ name, from, to string }{
		{"local ASN", `"as":65000`, `"as":0`},
		{"remote identifier", `"router-id":"192.0.2.2"`, `"router-id":"0.0.0.0"`},
	} {
		t.Run(identity.name, func(t *testing.T) {
			exporter, capture := exportFixture(t)
			bus := &epeProofBus{exporter: exporter, labels: make(map[uint32]netip.Addr)}
			source := newEPESource(bus)
			peer := netip.MustParseAddr("198.51.100.2")
			require.NoError(t, source.configure(epeConfig{base: 16000, size: 100, peers: map[netip.Addr]epePeerConfig{peer: {index: 7}}}))
			event := epeProofEvent(t, "up")
			event.Peer = []byte(strings.Replace(string(event.Peer), identity.from, identity.to, 1))
			require.Error(t, source.state(event))
			require.Empty(t, bus.labels)
			require.NoError(t, exporter.reconcile(context.Background()))
			require.Empty(t, capture.commands)
		})
	}
}

// VALIDATES: enabling native EPE from the actual config leaf representation
// installs and advertises a SID, and removing the root removes both forms.
// PREVENTS: a config-only enable flag with no effective disable/withdrawal path.
func TestRFC9086NativeConfigurationEnableDisable(t *testing.T) {
	// RFC requirement: RFC9086-7-1 positive -- operator configuration produces forwarding state and a real PeerNode advertisement.
	// RFC requirement: RFC9086-7-1 negative -- removing that configuration withdraws forwarding and collector state despite the session remaining up.
	config, err := parseEPEConfig([]rpc.ConfigSection{{Root: epeName, Data: `{"bgp-epe":{"srgb":{"lower-bound":"16000","upper-bound":"16099"},"peer":{"198.51.100.2":{"sid-index":"7","weight":"5"}}}}`}})
	require.NoError(t, err)
	exporter, capture := exportFixture(t)
	bus := &epeProofBus{exporter: exporter, labels: make(map[uint32]netip.Addr)}
	source := newEPESource(bus)
	require.NoError(t, source.state(epeProofEvent(t, "up")))
	require.NoError(t, source.configure(config))
	require.NoError(t, exporter.reconcile(context.Background()))
	require.Equal(t, map[uint32]netip.Addr{16007: netip.MustParseAddr("198.51.100.2")}, bus.labels)
	require.Len(t, capture.commands, 2)
	require.Equal(t, [][]byte{{0x10, 5, 0, 0, 0, 0, 0, 7}}, exportTLVValues(t, exportCommandBytes(t, capture.commands[1], "attr")[11:], 1101))
	disabled, err := parseEPEConfig(nil)
	require.NoError(t, err)
	require.NoError(t, source.configure(disabled))
	require.Empty(t, bus.labels)
	require.NoError(t, exporter.reconcile(context.Background()))
	require.Len(t, capture.commands, 4)
	require.Contains(t, capture.commands[2], " del ")
	require.Contains(t, capture.commands[3], " del ")
}
