// Design: docs/architecture/wire/nlri-bgpls.md -- native SRv6 origination
// RFC: rfc/short/rfc9514.md -- capability, locator and SID advertisements

package ls

import (
	"context"
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/linkstateevents"
)

func nativeSRv6Snapshot(reserved byte) *linkstateevents.Snapshot {
	return &linkstateevents.Snapshot{Domain: linkstateevents.Domain{Protocol: linkstateevents.ISISLevel1},
		Generation: 1,
		Nodes:      []linkstateevents.Node{{ID: nativeTestNode(), Attributes: []linkstateevents.TLV{{Type: 1038, Value: []byte{0x40, 0, reserved, reserved}}}}},
		Prefixes: []linkstateevents.Prefix{{Node: nativeTestNode(), Prefix: netip.MustParsePrefix("2001:db8::/64"), Topology: 7,
			Attributes: []linkstateevents.TLV{{Type: 1162, Value: []byte{0x80, 128, reserved, reserved, 0, 0, 0, 7, 0xc3, 0x50, 0, 2, 0xaa, 0xbb}}}}},
		SIDs: []linkstateevents.SID{{Node: nativeTestNode(), SID: netip.MustParseAddr("2001:db8::1"), Topology: 7,
			Attributes: []linkstateevents.TLV{{Type: 1250, Value: []byte{0, 1, 0, 128}}, {Type: 1252, Value: []byte{32, 32, 16, 48}}}}}}
}

func nativeSRv6Commands(t *testing.T, reserved byte) []string {
	t.Helper()
	e, capture := exportFixture(t)
	require.NoError(t, e.replace("isis", nativeSRv6Snapshot(reserved)))
	require.NoError(t, e.reconcile(context.Background()))
	require.Len(t, capture.commands, 3)
	return capture.commands
}

// VALIDATES: standard capability and locator fields survive origination, and
// each emitted SID has its one SID descriptor and mandatory Endpoint Behavior.
// PREVENTS: clearing defined fields with reserved bytes or omitting SID meaning.
func TestRFC9514NativeSRv6Advertisements(t *testing.T) {
	// RFC requirement: RFC9514-3.1-2 positive -- a native capability preserves its flags while emitting a zero reserved word.
	// RFC requirement: RFC9514-5.1-1 positive -- a locator preserves metric, algorithm and unknown sub-TLVs with its reserved word zero.
	// RFC requirement: RFC9514-6-1 positive -- a native SID emits exactly one SID Information descriptor containing that SID.
	// RFC requirement: RFC9514-7.1-1 positive -- the collector-visible SID attribute contains Endpoint Behavior.
	commands := nativeSRv6Commands(t, 0)
	require.Equal(t, [][]byte{{0x40, 0, 0, 0}}, exportTLVValues(t, exportCommandBytes(t, commands[0], "attr")[11:], 1038))
	require.Equal(t, [][]byte{{0x80, 128, 0, 0, 0, 0, 0, 7, 0xc3, 0x50, 0, 2, 0xaa, 0xbb}}, exportTLVValues(t, exportCommandBytes(t, commands[1], "attr")[11:], 1162))
	wire := exportCommandBytes(t, commands[2], "nlri")
	require.Equal(t, uint16(6), binary.BigEndian.Uint16(wire))
	sid := netip.MustParseAddr("2001:db8::1").As16()
	require.Equal(t, [][]byte{sid[:]}, exportTLVValues(t, wire[13:], 518))
	require.Equal(t, [][]byte{{0, 1, 0, 128}}, exportTLVValues(t, exportCommandBytes(t, commands[2], "attr")[11:], 1250))
	require.Equal(t, [][]byte{{32, 32, 16, 48}}, exportTLVValues(t, exportCommandBytes(t, commands[2], "attr")[11:], 1252))
}

// VALIDATES: nonzero native reserved words do not leak into either advertisement.
// PREVENTS: copying received IGP bytes directly into differently framed LS TLVs.
func TestRFC9514NativeSRv6ReservedCleared(t *testing.T) {
	// RFC requirement: RFC9514-3.1-2 negative -- poisoned source capability reserved bytes are absent from originated wire.
	// RFC requirement: RFC9514-5.1-1 negative -- poisoned source locator reserved bytes are absent without damaging following fields.
	commands := nativeSRv6Commands(t, 0xff)
	require.Equal(t, [][]byte{{0x40, 0, 0, 0}}, exportTLVValues(t, exportCommandBytes(t, commands[0], "attr")[11:], 1038))
	require.Equal(t, [][]byte{{0x80, 128, 0, 0, 0, 0, 0, 7, 0xc3, 0x50, 0, 2, 0xaa, 0xbb}}, exportTLVValues(t, exportCommandBytes(t, commands[1], "attr")[11:], 1162))
}

// VALIDATES: an incomplete native SID cannot replace a usable advertised SID.
// PREVENTS: advertising an SRv6 address with no endpoint behavior for consumers.
func TestRFC9514NativeSIDRequiresEndpointBehavior(t *testing.T) {
	// RFC requirement: RFC9514-7.1-1 negative -- a SID replacement without Endpoint Behavior is refused before emitting an UPDATE.
	e, capture := exportFixture(t)
	snapshot := nativeSRv6Snapshot(0)
	require.NoError(t, e.replace("isis", snapshot))
	require.NoError(t, e.reconcile(context.Background()))
	require.Len(t, capture.commands, 3)
	snapshot.Generation++
	snapshot.SIDs[0].Attributes = snapshot.SIDs[0].Attributes[1:]
	require.Error(t, e.replace("isis", snapshot))
	require.NoError(t, e.reconcile(context.Background()))
	require.Len(t, capture.commands, 3)
}

// VALIDATES: an exact 128-bit SID structure is usable, but its overflowing
// replacement is refused without withdrawing the accepted database.
// PREVENTS: accepting a SID structure whose fields exceed the IPv6 address.
func TestRFC9514NativeSIDStructureBoundary(t *testing.T) {
	// RFC requirement: RFC9514-8-1 positive -- a 128-bit native structure reaches the collector unchanged.
	// RFC requirement: RFC9514-8-1 negative -- an overflowing replacement is refused without altering accepted advertisements.
	e, capture := exportFixture(t)
	snapshot := nativeSRv6Snapshot(0)
	require.NoError(t, e.replace("isis", snapshot))
	require.NoError(t, e.reconcile(context.Background()))
	require.Len(t, capture.commands, 3)
	require.Equal(t, [][]byte{{32, 32, 16, 48}}, exportTLVValues(t, exportCommandBytes(t, capture.commands[2], "attr")[11:], 1252))
	snapshot.Generation++
	snapshot.SIDs[0].Attributes[1].Value[3] = 49
	require.Error(t, e.replace("isis", snapshot))
	require.NoError(t, e.reconcile(context.Background()))
	require.Len(t, capture.commands, 3)
}
