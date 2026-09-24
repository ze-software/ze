// Design: docs/architecture/wire/nlri-bgpls.md -- native opaque attribute provenance
// RFC: rfc/short/rfc9552.md -- OSPF opaque attribute carriers

package ls

import (
	"context"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/internal/core/linkstateevents"
)

func provenanceSnapshot(protocol linkstateevents.Protocol, kind uint16, source linkstateevents.Provenance) *linkstateevents.Snapshot {
	node := linkstateevents.NodeID{ASN: 65000, Area: 0, HasArea: true, RouterID: []byte{192, 0, 2, 1}}
	opaque := []linkstateevents.Opaque{{Source: source, Value: []byte{0xfe, 1, 0, 1, 0xa5}}}
	snapshot := &linkstateevents.Snapshot{Domain: linkstateevents.Domain{Protocol: protocol}}
	switch kind {
	case 1:
		snapshot.Nodes = []linkstateevents.Node{{ID: node, Opaque: opaque}}
	case 2:
		snapshot.Links = []linkstateevents.Link{{Local: node, Remote: node, LocalID: 7, HasLinkIDs: true, Opaque: opaque}}
	case 3:
		snapshot.Prefixes = []linkstateevents.Prefix{{Node: node, Prefix: netip.MustParsePrefix("192.0.2.0/24"), RouteType: 1, Opaque: opaque}}
	}
	return snapshot
}

func proveNativeOpaqueCarrier(t *testing.T, protocol linkstateevents.Protocol, kind uint16, accepted, rejected linkstateevents.Provenance) {
	t.Helper()
	exporter, capture := exportFixture(t)
	snapshot := provenanceSnapshot(protocol, kind, accepted)
	require.NoError(t, exporter.replace("ospf", snapshot))
	require.NoError(t, exporter.reconcile(context.Background()))
	require.Len(t, capture.commands, 1)
	code := map[uint16]uint16{1: 1025, 2: 1097, 3: 1157}[kind]
	attributes := exportCommandBytes(t, capture.commands[0], "attr")
	require.Equal(t, [][]byte{{0xfe, 1, 0, 1, 0xa5}}, exportTLVValues(t, attributes[11:], code))
	bad := provenanceSnapshot(protocol, kind, rejected)
	bad.Generation = 1
	require.Error(t, exporter.replace("ospf", bad))
	require.NoError(t, exporter.reconcile(context.Background()))
	require.Len(t, capture.commands, 1, "refusing unrelated LSA provenance must not replace or withdraw the valid source state")
}

// VALIDATES: only RI-LSA native fields enter Opaque Node Attribute wire data.
// PREVENTS: a prefix/link LSA's unknown field being mislabeled as a node field.
func TestRFC9552NativeOpaqueNodeProvenance(t *testing.T) {
	// RFC requirement: RFC9552-5.3.1.5-1 positive -- RI-LSA native opaque node bytes reach the collector.
	// RFC requirement: RFC9552-5.3.1.5-1 negative -- Extended Link LSA bytes cannot replace the valid node advertisement.
	proveNativeOpaqueCarrier(t, linkstateevents.OSPFv2, 1, linkstateevents.OSPFRouterInformation, linkstateevents.OSPFv2ExtendedLink)
}

// VALIDATES: OSPFv2 native opaque link content comes from Extended Link LSAs.
// PREVENTS: RI-LSA bytes being carried as an opaque link extension.
func TestRFC9552NativeOpaqueOSPFv2LinkProvenance(t *testing.T) {
	// RFC requirement: RFC9552-5.3.2.6-1 positive -- Extended Link LSA opaque bytes reach the link attribute.
	// RFC requirement: RFC9552-5.3.2.6-1 negative -- RI-LSA data cannot originate an opaque link attribute.
	proveNativeOpaqueCarrier(t, linkstateevents.OSPFv2, 2, linkstateevents.OSPFv2ExtendedLink, linkstateevents.OSPFRouterInformation)
}

// VALIDATES: OSPFv3 native opaque link content comes from E-Router/E-Link LSAs.
// PREVENTS: an E-Inter-Area-Prefix LSA being relabeled as an opaque link.
func TestRFC9552NativeOpaqueOSPFv3LinkProvenance(t *testing.T) {
	// RFC requirement: RFC9552-5.3.2.6-2 positive -- E-Router LSA opaque bytes reach the link attribute.
	// RFC requirement: RFC9552-5.3.2.6-2 negative -- E-Inter-Area-Prefix data cannot originate an opaque link attribute.
	proveNativeOpaqueCarrier(t, linkstateevents.OSPFv3, 2, linkstateevents.OSPFv3ExtendedRouter, linkstateevents.OSPFv3ExtendedInterAreaPrefix)
}

// VALIDATES: OSPFv2 native opaque prefix content comes from Extended Prefix LSAs.
// PREVENTS: Extended Link LSA bytes being attached to a prefix advertisement.
func TestRFC9552NativeOpaqueOSPFv2PrefixProvenance(t *testing.T) {
	// RFC requirement: RFC9552-5.3.3.6-1 positive -- Extended Prefix LSA opaque bytes reach the prefix attribute.
	// RFC requirement: RFC9552-5.3.3.6-1 negative -- Extended Link LSA data cannot originate an opaque prefix attribute.
	proveNativeOpaqueCarrier(t, linkstateevents.OSPFv2, 3, linkstateevents.OSPFv2ExtendedPrefix, linkstateevents.OSPFv2ExtendedLink)
}

// VALIDATES: OSPFv3 native opaque prefix data retains its extended-prefix carrier.
// PREVENTS: E-Link LSA data being relabeled as an opaque prefix extension.
func TestRFC9552NativeOpaqueOSPFv3PrefixProvenance(t *testing.T) {
	// RFC requirement: RFC9552-5.3.3.6-2 positive -- E-NSSA LSA opaque bytes reach the prefix attribute.
	// RFC requirement: RFC9552-5.3.3.6-2 negative -- E-Link LSA data cannot originate an opaque prefix attribute.
	proveNativeOpaqueCarrier(t, linkstateevents.OSPFv3, 3, linkstateevents.OSPFv3ExtendedNSSA, linkstateevents.OSPFv3ExtendedLink)
}
