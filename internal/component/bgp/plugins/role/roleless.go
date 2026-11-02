// Design: docs/guide/bgp-role.md -- peers that declare no role
// Overview: config.go -- extractPeerRoleConfigs, the reader of the role a peer declares
// Related: upstream.go -- parseASN, the ASN reader this file shares
// RFC: rfc/short/rfc9234.md -- the relationship a role states
// RFC: rfc/short/rfc7705.md -- the migration ASN that keeps a session internal

package role

import (
	"fmt"
	"log/slog"
	"net/netip"
	"slices"

	"github.com/ze-software/ze/internal/component/bgp/configjson"
	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// rolelessPeers names the peers and the dynamic groups of a bgp config subtree
// that declare no RFC 9234 role, sorted by name. They are accepted, and none of
// the checks a role inserts (the OTC procedures, the RFC 7454 Section 9 upstream
// check) runs on them. This is the ONE declaration of that set: the warning
// logged when a config is installed or verified (warnPeersWithoutRole) and the
// `ze doctor` check (doctor.go) both read it, so the two never enumerate
// different peers.
//
// A role is declared when the peer or its group carries a role container this
// plugin parses (parseRoleFromMap), the same reader extractPeerRoleConfigs uses,
// so a peer named here is exactly a peer this plugin holds no role for.
//
// An ASN that does not parse is an error rather than a guess: whether the
// session is internal decides whether the peer is named.
func rolelessPeers(bgpSubtree map[string]any) ([]string, error) {
	var names []string
	var walkErr error
	configjson.ForEachPeer(bgpSubtree, func(name string, peerMap, groupMap map[string]any, origin configjson.PeerOrigin) {
		if walkErr != nil {
			return
		}
		if parseRoleFromMap(peerMap) != nil {
			return
		}
		if parseRoleFromMap(groupMap) != nil {
			return
		}
		named, err := reportsRoleless(name, bgpSubtree, peerMap, groupMap, origin)
		if err != nil {
			walkErr = fmt.Errorf("bgp-role: peer %s: %w", name, err)
			return
		}
		if named {
			names = append(names, name)
		}
	})
	if walkErr != nil {
		return nil, walkErr
	}
	// ForEachPeer walks Go maps, so the order is sorted here: the warning's
	// sample and the doctor report name the same peers on every run.
	slices.Sort(names)
	return names, nil
}

// reportsRoleless reports whether a peer that declares no role is named. ONE
// kind of peer is left out, and the reason is what the report says: an RFC 9234
// role describes an eBGP relationship, so a session Ze KNOWS is iBGP owes no
// role, and naming one would be noise on every route-reflector deployment.
func reportsRoleless(name string, bgpSubtree, peerMap, groupMap map[string]any,
	origin configjson.PeerOrigin,
) (bool, error) {
	// A dynamic group states no remote AS. It arrives in the member's OPEN
	// (RFC 4271 Section 4.2), so there is nothing to compare: UNKNOWN here,
	// never iBGP. A listen range that declares no role is the IXP
	// route-server shape this report exists for.
	if origin.Template {
		return true, nil
	}
	// A static peer with no remote address, or no remote AS, never becomes a
	// session: the engine drops it with ErrIncompleteConfig
	// (reactor.parsePeerFromTree), so there is no session to report on. The
	// name stands in for the address only when it is one, as the engine reads
	// it (extractPeerRoleConfigs).
	if configjson.PeerRemoteIP(peerMap, groupMap) == "" && !isAddress(name) {
		return false, nil
	}
	layers := []map[string]any{peerMap, groupMap, bgpSubtree}
	remoteAS, configured, err := sessionASN("remote", layers)
	if err != nil {
		return false, err
	}
	if !configured {
		return false, nil
	}
	localAS, _, err := sessionASN("local", layers)
	if err != nil {
		return false, err
	}
	migrationAS, _, err := sessionASN("migration", layers)
	if err != nil {
		return false, err
	}
	// The one rule the reactor applies too. A session running under the RFC
	// 7705 Section 4.2 migration ASN is iBGP, so it owes no RFC 9234 role, and
	// naming it would put every migrating route reflector in this report.
	return !filterapi.InternalSession(remoteAS, localAS, migrationAS), nil
}

// isAddress reports whether a peer name is itself an IP address, which the
// engine then reads as the peer's remote address.
func isAddress(name string) bool {
	_, err := netip.ParseAddr(name)
	return err == nil
}

// sessionASN reads session asn <leaf> from the first layer that sets it: the
// peer, then its group, then the bgp block, the precedence the engine resolves
// a peer's config with (bgpconfig.ResolveBGPTree). configured is false when no
// layer sets it.
func sessionASN(leaf string, layers []map[string]any) (asn uint32, configured bool, err error) {
	for _, m := range layers {
		session, _ := m["session"].(map[string]any) // no session container: next layer
		asnMap, _ := session["asn"].(map[string]any)
		value, present := asnMap[leaf]
		if !present {
			continue
		}
		asn, err = parseASN(value)
		if err != nil {
			return 0, false, fmt.Errorf("session asn %s: %w", leaf, err)
		}
		return asn, true, nil
	}
	return 0, false, nil
}

// warnPeersWithoutRole logs ONE line for the whole config, naming how many
// peers and dynamic groups declare no role and the first few by name. One line
// per peer would bury the fact it exists to report on a router with a thousand
// sessions.
func warnPeersWithoutRole(log *slog.Logger, names []string) {
	if len(names) == 0 {
		return
	}
	shown := names
	if len(shown) > rolelessPeersNamed {
		shown = shown[:rolelessPeersNamed]
	}
	log.Warn("peers and dynamic groups declare no RFC 9234 role, so no check a role inserts runs on them",
		"peers", len(names),
		"first", textbuf.Join(shown, ", "),
		"action", "add `role { import <role> }` to each peer or group that should carry one, or leave it out deliberately")
}

// rolelessPeersNamed bounds how many peer names the aggregated warning prints.
// The count is always exact; the names are a sample that keeps the line short.
const rolelessPeersNamed = 5

// verifyLogger is the logger the in-process config verifier warns through.
// `ze config validate` runs that verifier without starting this plugin, so
// logger() is still the discard logger there (ConfigureEngineLogger never
// ran); this one prints under the same subsystem name the engine gives the
// plugin (plugin.CanonicalSubsystemName of rolePluginName).
var verifyLogger = slogutil.LazyLogger("bgp.role")
