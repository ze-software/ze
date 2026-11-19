// Design: docs/guide/graceful-restart.md -- exact original-physical RIB lifecycle commands.
package rib

import (
	"context"
	"encoding/hex"
	"net/netip"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// One real SDK source lease feeds the real registered RIB command. The old
// physical route changes while a same-address alias and a fresh physical route
// remain useful. Neither an address nor the newest metadata supplies authority.
// MUTATION: ignore SourceConnectionID in ribPhysicalSource.matchesReceived;
// the same-owner successor is then marked and removed by the old command.
func TestGRSourceCommandsPreserveOtherPhysicalRoutes(t *testing.T) {
	for _, native := range []bool{false, true} {
		for _, retire := range []string{"purge-stale", "release-routes"} {
			t.Run(strconv.FormatBool(native)+"/"+retire, func(t *testing.T) {
				r, calls := sourceLeaseTestRIB(t, native)
				address := netip.MustParseAddr("192.0.2.241")
				grSourceCommandRoute(t, r, address, 701, 41, 31, 10, nil)
				grSourceCommandRoute(t, r, address, 702, 51, 32, 12, nil)
				grSourceCommandRun(t, r, "retain-routes", 701, 41, "on-down", "families", "ipv4/unicast")
				require.Equal(t, 1, calls.count(rpc.SourceLeaseRetain, 31))
				require.Zero(t, calls.count(rpc.SourceLeaseRetain, 32))
				grSourceCommandRun(t, r, "mark-stale", 701, 41, "restart-time", "0")
				grSourceCommandRoute(t, r, address, 701, 42, 33, 11, nil)
				r.retainSourceOnDown(address, 701)
				require.Zero(t, calls.count(rpc.SourceLeaseRetain, 33), "the old retained cycle cannot retain its successor on DOWN")
				grSourceCommandRun(t, r, "mark-stale", 701, 41, "restart-time", "0", "stale-level", "2")
				grSourceCommandRun(t, r, "attach-community", 701, 41, "family", "ipv4/unicast", "community", "ffff0006")
				old := grSourceCommandRead(t, r, 701, 10)
				require.Equal(t, uint8(2), old.StaleLevel)
				encoded, err := old.ToWireBytes()
				require.NoError(t, err)
				require.Contains(t, hex.EncodeToString(encoded), "ffff0006")
				require.Zero(t, grSourceCommandRead(t, r, 701, 11).StaleLevel)
				require.Zero(t, grSourceCommandRead(t, r, 702, 12).StaleLevel)
				grSourceCommandRun(t, r, retire, 701, 41)
				_, found := r.bgpPeers[701].Lookup(family.IPv4Unicast, []byte{8, 10})
				require.False(t, found, "the original physical route survived its retirement")
				successor := grSourceCommandRead(t, r, 701, 11)
				require.Equal(t, uint64(42), successor.SourceConnectionID())
				sameAddress := grSourceCommandRead(t, r, 702, 12)
				require.Equal(t, uint64(51), sameAddress.SourceConnectionID())
				require.Equal(t, 1, calls.count(rpc.SourceLeaseRelease, 31))
				require.Zero(t, calls.count(rpc.SourceLeaseRelease, 32))
				require.Zero(t, calls.count(rpc.SourceLeaseRelease, 33))
			})
		}
	}
}

// Deleting NO_LLGR on one old connection must leave its useful sibling and a
// same-owner replacement with the same community. A later scoped decoration
// must not decorate that replacement or another configured owner.
func TestGRSourceCommunityCleanupKeepsPhysicalSiblings(t *testing.T) {
	r, _ := sourceLeaseTestRIB(t, true)
	address := netip.MustParseAddr("192.0.2.242")
	grSourceCommandRoute(t, r, address, 711, 61, 41, 20, []byte{0xff, 0xff, 0, 7})
	grSourceCommandRoute(t, r, address, 711, 61, 41, 21, nil)
	grSourceCommandRun(t, r, "retain-routes", 711, 61, "on-down", "families", "ipv4/unicast")
	grSourceCommandRun(t, r, "mark-stale", 711, 61, "restart-time", "0")
	grSourceCommandRoute(t, r, address, 711, 62, 42, 22, []byte{0xff, 0xff, 0, 7})
	grSourceCommandRoute(t, r, address, 712, 71, 43, 23, []byte{0xff, 0xff, 0, 7})
	grSourceCommandRun(t, r, "delete-with-community", 711, 61, "family", "ipv4/unicast", "community", "ffff0007")
	_, found := r.bgpPeers[711].Lookup(family.IPv4Unicast, []byte{8, 20})
	require.False(t, found)
	oldSibling := grSourceCommandRead(t, r, 711, 21)
	require.Equal(t, uint64(61), oldSibling.SourceConnectionID())
	successor := grSourceCommandRead(t, r, 711, 22)
	require.Equal(t, uint64(62), successor.SourceConnectionID())
	sameAddress := grSourceCommandRead(t, r, 712, 23)
	require.Equal(t, uint64(71), sameAddress.SourceConnectionID())
	grSourceCommandRun(t, r, "attach-community", 711, 61, "family", "ipv4/unicast", "community", "ffff0006")
	require.Equal(t, uint8(2), grSourceCommandRead(t, r, 711, 21).StaleLevel)
	require.Zero(t, grSourceCommandRead(t, r, 711, 22).StaleLevel)
	require.Zero(t, grSourceCommandRead(t, r, 712, 23).StaleLevel)
}

// The real LLGR sequence continues after NO_LLGR removes the last route, and
// family expiry calls purge then release. Its existing active cycle must still
// accept those empty follow-ups, without recording completed source history.
func TestGRSourceEmptyLLGRContinuation(t *testing.T) {
	r, calls := sourceLeaseTestRIB(t, true)
	address := netip.MustParseAddr("192.0.2.245")
	grSourceCommandRoute(t, r, address, 741, 101, 71, 50, []byte{0xff, 0xff, 0, 7})
	grSourceCommandRun(t, r, "retain-routes", 741, 101, "on-down", "families", "ipv4/unicast")
	grSourceCommandRun(t, r, "mark-stale", 741, 101, "restart-time", "0")
	grSourceCommandRun(t, r, "delete-with-community", 741, 101, "family", "ipv4/unicast", "community", "ffff0007")
	require.NotNil(t, r.grState[741])
	grSourceCommandRun(t, r, "attach-community", 741, 101, "family", "ipv4/unicast", "community", "ffff0006")
	grSourceCommandRun(t, r, "mark-stale", 741, 101, "restart-time", "0", "stale-level", "2", "family", "ipv4/unicast")
	grSourceCommandRun(t, r, "purge-stale", 741, 101, "family", "ipv4/unicast")
	grSourceCommandRun(t, r, "release-routes", 741, 101)
	require.Nil(t, r.grState[741])
	require.False(t, r.retainedPeers[741])
	require.Equal(t, 1, calls.count(rpc.SourceLeaseRelease, 71))
}

// Received and sent inventories have separate provenance. The real command
// must decorate and withdraw only the sent copy from its original connection.
// The update hook records the existing final-writer metadata contract; socket
// joining is exercised by the retained-validation functional fixture.
func TestGRSourceSentProjectionKeepsSuccessor(t *testing.T) {
	r, _ := sourceLeaseTestRIB(t, true)
	address := netip.MustParseAddr("192.0.2.246")
	grSourceCommandRoute(t, r, address, 751, 111, 81, 60, nil)
	grSourceCommandRun(t, r, "retain-routes", 751, 111, "on-down", "families", "ipv4/unicast")
	grSourceCommandRoute(t, r, address, 751, 112, 82, 61, nil)
	r.peerMeta[761] = &peerMetadata{Address: netip.MustParseAddr("192.0.2.247"), ConnectionID: 121, Session: 91}
	sent := testRibOutMap(map[string]*Route{
		"60.0.0.0/8": {MsgID: 201, NextHop: "192.0.2.1"},
		"61.0.0.0/8": {MsgID: 202, NextHop: "192.0.2.1"},
	})
	for key, entry := range sent {
		entry.DestinationSession = 91
		entry.SourceOwner = 751
		entry.SourceMessageID = 1081
		entry.SourceConnectionID = 111
		if key.Prefix.Addr().As4()[0] == 61 {
			entry.SourceConnectionID = 112
			entry.SourceMessageID = 1082
		}
		sent[key] = entry
	}
	r.ribOut[761] = map[family.Family]map[ribOutKey]ribOutEntry{family.IPv4Unicast: sent}
	defer func() {
		for _, entry := range sent {
			entry.release()
		}
	}()
	var writes []map[string]any
	r.updateHook = func(_ string, meta map[string]any) { writes = append(writes, meta) }
	grSourceCommandRun(t, r, "mark-stale", 751, 111, "restart-time", "0")
	grSourceCommandRun(t, r, "attach-community", 751, 111, "family", "ipv4/unicast", "community", "ffff0006")
	oldKey := ribOutKey{Prefix: netip.MustParsePrefix("60.0.0.0/8")}
	newKey := ribOutKey{Prefix: netip.MustParsePrefix("61.0.0.0/8")}
	require.Equal(t, uint8(2), sent[oldKey].StaleLevel)
	require.Zero(t, sent[newKey].StaleLevel)
	grSourceCommandRun(t, r, "release-routes", 751, 111)
	require.NotContains(t, sent, oldKey)
	require.Contains(t, sent, newKey)
	require.Len(t, writes, 1)
	require.Equal(t, "761", writes[0]["destination-owner"])
	require.Equal(t, "201", writes[0]["sent-owner-message"])
}

// The actual safety callback keeps its original physical scope after metadata
// changes. A repeated old callback cannot purge a successor's own stale route.
// MUTATION: take the owner-wide expiry branch for a source-bound GR state;
// the successor's deliberately stale control is then removed.
func TestGRSourceSafetyExpiryKeepsSuccessor(t *testing.T) {
	r, _ := sourceLeaseTestRIB(t, true)
	address := netip.MustParseAddr("192.0.2.243")
	grSourceCommandRoute(t, r, address, 721, 81, 51, 30, nil)
	grSourceCommandRun(t, r, "retain-routes", 721, 81, "on-down", "families", "ipv4/unicast")
	grSourceCommandRun(t, r, "mark-stale", 721, 81, "restart-time", "120")
	r.peerMu.Lock()
	original := r.grState[721]
	if original != nil {
		r.stopGRExpiryLocked(original)
	}
	r.peerMu.Unlock()
	require.NotNil(t, original)
	grSourceCommandRoute(t, r, address, 721, 82, 52, 31, nil)
	require.True(t, r.bgpPeers[721].ModifyFamilyEntry(family.IPv4Unicast, []byte{8, 31}, func(entry *storage.RouteEntry) { entry.StaleLevel = 1 }))
	r.autoExpireStale(721, original)
	_, found := r.bgpPeers[721].Lookup(family.IPv4Unicast, []byte{8, 30})
	require.False(t, found)
	require.Equal(t, uint8(1), grSourceCommandRead(t, r, 721, 31).StaleLevel)
	r.autoExpireStale(721, original)
	surviving := grSourceCommandRead(t, r, 721, 31)
	require.Equal(t, uint64(82), surviving.SourceConnectionID())
}

// All six source declarations reach handlers, while malformed or unknown
// original identities are refused before any route or retention mark changes.
func TestGRSourceCommandRegistrationAndRefusal(t *testing.T) {
	r, _ := sourceLeaseTestRIB(t, true)
	address := netip.MustParseAddr("192.0.2.244")
	grSourceCommandRoute(t, r, address, 731, 91, 61, 40, nil)
	names := []string{"retain-routes", "release-routes", "mark-stale", "purge-stale", "attach-community", "delete-with-community"}
	declared := make(map[string]bool)
	for _, decl := range commandDecls() {
		declared[decl.Name] = true
	}
	for _, name := range names {
		command := "request bgp rib source " + name
		require.True(t, declared[command], command)
		require.NotNil(t, registeredCommands[command], command)
		for _, args := range [][]string{{}, {"owner", "731", "connection", "0"}, {"owner", "0", "connection", "91"}, {"owner", "999", "connection", "91"}, {"owner", "731", "connection", "unknown"}} {
			status, _, err := r.handleCommand(command, "*", args)
			require.Error(t, err, command)
			require.Equal(t, statusError, status)
		}
	}
	require.Zero(t, grSourceCommandRead(t, r, 731, 40).StaleLevel)
	require.False(t, r.retainedPeers[731])
}

// grSourceCommandRoute inserts only after the actual SDK lease acquire. Its
// returned storage ownership MUST be released by the owning fixture manager;
// each local borrowed reference MUST be released before this helper returns.
func grSourceCommandRoute(t *testing.T, r *RIBManager, address netip.Addr, owner, connection, receipt uint64, prefix byte, community []byte) {
	t.Helper()
	reference, err := r.sourceLeases.acquire(context.Background(), address, owner, receipt, connection)
	require.NoError(t, err)
	defer reference.Release()
	attrs := []byte{0x40, 1, 1, 0, 0x40, 2, 6, 2, 1, 0, 0, 0xfd, 0xe9, 0x40, 3, 4, 192, 0, 2, 1}
	if len(community) != 0 {
		attrs = append(attrs, 0xc0, 8, 4)
		attrs = append(attrs, community...)
	}
	entry, err := storage.ParseAttributes(attrs)
	require.NoError(t, err)
	defer entry.Release()
	entry.MsgID = receipt + 1000
	entry.SourceReceipt = receipt
	entry.SourcePolicyEpoch = 5
	entry.SetSourceReference(reference)
	r.peerMu.Lock()
	routes := r.bgpPeers[owner]
	if routes == nil {
		routes, err = storage.NewAddressPeerRIB(address)
		if err != nil {
			r.peerMu.Unlock()
			require.NoError(t, err)
			return
		}
		r.bgpPeers[owner] = routes
	}
	r.peerMeta[owner] = &peerMetadata{Address: address, ConnectionID: connection, Session: receipt, PeerASN: 65001, LocalASN: 65000}
	r.peerMu.Unlock()
	routes.InsertEntry(family.IPv4Unicast, entry, 0, uint32(len(attrs)), []byte{8, prefix})
}

func grSourceCommandRun(t *testing.T, r *RIBManager, action string, owner, connection uint64, rest ...string) {
	t.Helper()
	args := []string{"owner", strconv.FormatUint(owner, 10), "connection", strconv.FormatUint(connection, 10)}
	args = append(args, rest...)
	status, _, err := r.handleCommand("request bgp rib source "+action, "*", args)
	require.NoError(t, err)
	require.Equal(t, statusDone, status)
}

func grSourceCommandRead(t *testing.T, r *RIBManager, owner uint64, prefix byte) storage.RouteEntry {
	t.Helper()
	routes := r.bgpPeers[owner]
	require.NotNil(t, routes)
	entry, found := routes.Lookup(family.IPv4Unicast, []byte{8, prefix})
	require.True(t, found)
	return entry
}
