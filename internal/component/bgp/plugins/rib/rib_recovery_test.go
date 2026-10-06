// Design: docs/architecture/plugin/rib-storage-design.md -- source-owned DOWN recovery.
package rib

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"net/netip"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
)

func recoveryTestCleanup(t *testing.T, r *RIBManager) {
	t.Helper()
	t.Cleanup(func() {
		for _, peer := range r.bgpPeers {
			peer.Release()
		}
		for _, families := range r.ribOut {
			for _, routes := range families {
				for _, entry := range routes {
					entry.release()
				}
			}
		}
	})
}

// RFC 4271 Section 6 recalculates the best routes and advertises
// "either withdraws for the routes marked as invalid, or the new best routes".
// RFC 8654 Section 5 requires fatal extended-message errors to follow RFC 4271.
// PREVENTS: a blind source-DOWN withdrawal replacing an available survivor, or
// replaying the failed source's attributes under the survivor's address.
func TestRecoverySelectsSurvivorWithItsOwnAttributes(t *testing.T) {
	r := newTestRIBManager(t)
	recoveryTestCleanup(t, r)
	failed := netip.MustParseAddr("192.0.2.10")
	backup := netip.MustParseAddr("192.0.2.11")
	dest := netip.MustParseAddr("192.0.2.20")
	raw := []byte{24, 198, 51, 100}
	var backupAttrs []byte
	for _, path := range []struct {
		peer netip.Addr
		pref byte
		id   uint64
	}{{failed, 200, 80}, {backup, 100, 81}} {
		attrs := appendAttr(nativeSentAttrs(), byte(attribute.AttrNextHop), 0x40, path.peer.AsSlice())
		attrs = appendAttr(attrs, byte(attribute.AttrLocalPref), 0x40, []byte{0, 0, 0, path.pref})
		peer := storage.NewPeerRIB(path.peer.String())
		peer.Insert(family.IPv4Unicast, attrs, raw)
		peer.ModifyFamilyEntry(family.IPv4Unicast, raw, func(entry *storage.RouteEntry) { entry.MsgID = path.id })
		r.bgpPeers[path.peer], r.peerUp[path.peer] = peer, true
		if path.peer == backup {
			backupAttrs = attrs
		}
	}
	event := nativeSentEvent(t, dest, family.IPv4Unicast, raw, backupAttrs, false, false)
	event.RouteMeta["source-message-id"] = float64(80)
	r.handleSent(event)
	request := ribevents.RecoveryRequest{Source: failed, Destination: dest, Family: family.IPv4Unicast, NLRIs: [][]byte{raw}, Cut: 90}
	// RFC 4271 Section 6: query the same whole-set selection used by live changes.
	routes, err := r.recoveryRoutes(request)
	require.NoError(t, err)
	require.Len(t, routes, 1)
	require.False(t, routes[0].Withdraw, "a survivor is a replacement, never a transient withdrawal")
	require.Equal(t, backup, routes[0].Source)
	require.Equal(t, uint64(81), routes[0].MessageID)
	require.Equal(t, raw, routes[0].NLRI)
	require.Equal(t, backup.AsSlice(), routes[0].NextHop)
	for _, wanted := range []attribute.AttributeCode{attribute.AttrASPath, attribute.AttrLocalPref, 250} {
		var before, after []byte
		for _, data := range []struct {
			raw    []byte
			target *[]byte
		}{{backupAttrs, &before}, {routes[0].Attributes, &after}} {
			it := attribute.NewAttrIterator(data.raw)
			for code, _, value, ok := it.Next(); ok; code, _, value, ok = it.Next() {
				if code == wanted {
					*data.target = bytes.Clone(value)
				}
			}
		}
		require.NotEmpty(t, before)
		require.Equal(t, before, after, "replacement attribute %d", wanted)
	}
	// Reading recovery is not a destructive second received-route lifecycle.
	require.Contains(t, r.bgpPeers, failed)
	// The external command must reach the exact same producer and framing.
	status, answer, err := r.recoveryCommand("", []string{failed.String(), dest.String(), family.IPv4Unicast.String(), hex.EncodeToString(raw), "false", "false", "false", "90"})
	require.NoError(t, err)
	require.Equal(t, statusDone, status)
	require.Equal(t, routes, answer)

	// Re-forwarding a backup before the fatal event does not remove the need
	// to advertise the surviving selection after the failed source is removed.
	event.RouteMeta["source-peer"] = backup.String()
	event.RouteMeta["source-message-id"] = float64(81)
	r.handleSent(event)
	// RFC 4271 Section 6: an already advertised survivor remains the replacement.
	routes, err = r.recoveryRoutes(request)
	require.NoError(t, err)
	require.Len(t, routes, 1)
	require.False(t, routes[0].Withdraw)

	for _, owner := range []netip.Addr{failed, backup} {
		event.RouteMeta["source-peer"] = owner.String()
		event.RouteMeta["source-message-id"] = float64(91)
		r.handleSent(event)
		// RFC 4271 Section 3.1: a later advertisement already replaced the old path.
		routes, err = r.recoveryRoutes(request)
		require.NoError(t, err)
		require.Empty(t, routes, "old DOWN must not erase or replace a newer received owner")
	}
	// With no survivor, only the failed advertisement is withdrawn.
	r.peerUp[backup] = false
	event.RouteMeta["source-peer"] = failed.String()
	event.RouteMeta["source-message-id"] = float64(80)
	r.handleSent(event)
	// RFC 4271 Section 6: no feasible replacement leaves a withdrawal.
	routes, err = r.recoveryRoutes(request)
	require.NoError(t, err)
	require.Len(t, routes, 1)
	require.True(t, routes[0].Withdraw)
	require.Equal(t, raw, routes[0].NLRI)
}

// RFC 7911 Section 3 identifies a route by prefix AND the
// advertised identifier. RFC 8277 Section 2.4 uses one Compatibility field on
// withdrawal, regardless of the former label stack. Zero is a valid identifier.
// PREVENTS: native source-DOWN recovery withdrawing another source's path or
// copying ingress identifiers over independently assigned egress identifiers.
func TestRecoveryNativeSentIdentityAndNewerOwner(t *testing.T) {
	vpn4 := []byte{112, 0, 6, 65, 0, 0, 0, 1, 0, 0, 0, 2, 198, 51, 100}
	for _, tc := range []struct {
		name string
		fam  family.Family
		raw  []byte
	}{
		{"ipv4", family.IPv4Unicast, []byte{24, 198, 51, 100}},
		{"ipv6", family.IPv6Unicast, []byte{32, 32, 1, 13, 184}},
		{"labeled", family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIMPLSLabel}, []byte{72, 0, 6, 64, 0, 12, 129, 198, 51, 100}},
		{"vpn4", family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIVPN}, vpn4},
		{"evpn", family.Family{AFI: family.AFIL2VPN, SAFI: family.SAFIEVPN}, nativeEVPNRoute()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := newTestRIBManager(t)
			recoveryTestCleanup(t, r)
			failed := netip.MustParseAddr("192.0.2.10")
			backup := netip.MustParseAddr("192.0.2.11")
			dest := netip.MustParseAddr("192.0.2.20")
			for _, path := range []struct {
				id      uint32
				source  netip.Addr
				message uint64
			}{
				{0, failed, 80}, {7, failed, 80}, {17, backup, 81}, {99, failed, 91},
			} {
				raw := make([]byte, 4+len(tc.raw))
				binary.BigEndian.PutUint32(raw, path.id)
				copy(raw[4:], tc.raw)
				event := nativeSentEvent(t, dest, tc.fam, raw, nativeSentAttrs(), true, false)
				event.RouteMeta["source-peer"] = path.source.String()
				event.RouteMeta["source-message-id"] = float64(path.message)
				r.handleSent(event)
			}
			request := ribevents.RecoveryRequest{Source: failed, Destination: dest, Family: tc.fam, NLRIs: [][]byte{tc.raw}, SentAddPath: true, Cut: 90}
			// RFC 7911 Section 3: only the failed source's advertised IDs depart.
			routes, err := r.recoveryRoutes(request)
			require.NoError(t, err)
			require.Len(t, routes, 2)
			ids := make(map[uint32]bool)
			for _, route := range routes {
				require.True(t, route.Withdraw)
				key, ok := ribOutRouteKeyForAction(tc.fam, route.NLRI, true, true)
				require.True(t, ok)
				ids[key.PathID] = true
				require.Contains(t, r.ribOut[dest][tc.fam], key)
				if tc.fam.SAFI == family.SAFIMPLSLabel || tc.fam.SAFI == family.SAFIVPN {
					require.Equal(t, []byte{0x80, 0, 0}, route.NLRI[5:8])
				}
				r.handleSent(nativeSentEvent(t, dest, tc.fam, route.NLRI, nil, true, true))
			}
			require.Equal(t, map[uint32]bool{0: true, 7: true}, ids)
			require.Len(t, r.ribOut[dest][tc.fam], 2, "survivor and reconnect advertisement remain")
			// RFC 7911 Section 3: repeating DOWN cannot withdraw a different path.
			routes, err = r.recoveryRoutes(request)
			require.NoError(t, err)
			require.Empty(t, routes)
		})
	}
}

// RFC 8277 Section 2.4 encodes a labeled withdrawal as one
// Compatibility field and the prefix, even when the advertisement had a stack.
// PREVENTS: the text withdrawal grammar losing the labeled route's sent identity.
func TestRecoveryLabeledTextPrefixMatchesNativeSentStack(t *testing.T) {
	r := newTestRIBManager(t)
	recoveryTestCleanup(t, r)
	fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIMPLSLabel}
	failed := netip.MustParseAddr("192.0.2.10")
	dest := netip.MustParseAddr("192.0.2.20")
	advertised := []byte{72, 0, 6, 64, 0, 12, 129, 198, 51, 100}
	r.handleSent(nativeSentEvent(t, dest, fam, advertised, nativeSentAttrs(), false, false))
	// RFC 8277 Section 2.4: the label-free text name is the same route identity.
	routes, err := r.recoveryRoutes(ribevents.RecoveryRequest{Source: failed, Destination: dest,
		Family: fam, NLRIs: [][]byte{{24, 198, 51, 100}}, PrefixOnly: true})
	require.NoError(t, err)
	require.Len(t, routes, 1)
	require.True(t, routes[0].Withdraw)
	require.Equal(t, []byte{48, 0x80, 0, 0, 198, 51, 100}, routes[0].NLRI)
}

// TestRecoveryBatchedIdentitiesShareOneElectionResult drives both lookup
// surfaces with many distinct prefixes and repeated ingress identities. The
// single result retains all failed egress IDs, skips new ownership and survives
// actual sent-withdrawal projection without a restart for each path.
func TestRecoveryBatchedIdentitiesShareOneElectionResult(t *testing.T) {
	r := newTestRIBManager(t)
	recoveryTestCleanup(t, r)
	failed := netip.MustParseAddr("192.0.2.10")
	backup := netip.MustParseAddr("192.0.2.11")
	destination := netip.MustParseAddr("192.0.2.20")
	request := ribevents.RecoveryRequest{Source: failed, Destination: destination,
		Family: family.IPv4Unicast, AddPath: true, SentAddPath: true, Cut: 90}
	var encoded []string
	for prefix := range 64 {
		raw := []byte{0, 0, 0, 42, 24, 198, 51, byte(prefix)}
		request.NLRIs = append(request.NLRIs, raw, raw)
		encoded = append(encoded, hex.EncodeToString(raw), hex.EncodeToString(raw))
		for _, path := range []struct {
			id      uint32
			source  netip.Addr
			message uint64
		}{{0, failed, 80}, {7, failed, 80}, {17, backup, 81}, {99, failed, 91}} {
			sent := bytes.Clone(raw)
			binary.BigEndian.PutUint32(sent, path.id)
			event := nativeSentEvent(t, destination, request.Family, sent, nativeSentAttrs(), true, false)
			event.RouteMeta["source-peer"] = path.source.String()
			event.RouteMeta["source-message-id"] = float64(path.message)
			r.handleSent(event)
		}
	}
	routes, err := r.recoveryRoutes(request)
	require.NoError(t, err)
	require.Len(t, routes, 128, "one result covers every prefix and every failed sent identifier exactly once")
	status, document, err := r.recoveryCommand("", []string{failed.String(), destination.String(),
		request.Family.String(), strings.Join(encoded, ","), "true", "false", "true", "90"})
	require.NoError(t, err)
	require.Equal(t, statusDone, status)
	external, ok := document.([]ribevents.RecoveryRoute)
	require.True(t, ok)
	require.ElementsMatch(t, routes, external, "external IPC must use the same batched producer")
	for _, route := range routes {
		require.True(t, route.Withdraw)
		id := binary.BigEndian.Uint32(route.NLRI)
		require.Contains(t, []uint32{0, 7}, id)
		r.handleSent(nativeSentEvent(t, destination, request.Family, route.NLRI, nil, true, true))
	}
	require.Len(t, r.ribOut[destination][request.Family], 128, "survivors and newer source generation remain")
	routes, err = r.recoveryRoutes(request)
	require.NoError(t, err)
	require.Empty(t, routes)
}
