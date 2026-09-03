// Design: docs/architecture/bgp/filter-path-asn.md -- the reject-asn filter plugin
// Detail: config.go -- the position vocabulary and the config parse
// Detail: match.go -- the AS_PATH scan and the position judgement
// Related: internal/component/bgp/filtertext -- the reader of the filter text format
//
// Package filter_path_asn implements the bgp-filter-path-asn plugin.
//
// The plugin loads named reject-asn definitions from
// bgp { policy { reject-asn NAME { indirect [ N N ]; regex [ "P" ]; } } }
// at OnConfigure (Stage 2). At runtime, a peer filter chain names a list by its
// bare name, or as reject-asn:NAME, or as bgp-filter-path-asn:NAME. The engine
// dispatches each match through CallFilterUpdate (the filter-update RPC) and
// the plugin answers it in OnFilterUpdate.
//
// A list is an unordered reject set with two arms. A route is rejected when a
// position leaf-list holds an ASN the AS_PATH carries at that position, or when
// a pattern of the regex leaf-list matches the whole path. There is no ordering
// between the arms and no first-match-wins inside either, which is what
// separates the type from as-path-list, whose entries are ordered and carry
// their own accept-or-reject action.
//
// RFC 7454 Section 9: "This loose policy could be combined with filters for
// specific 2-byte or 4-byte AS paths that must not be accepted if advertised by
// the customer, such as upstream transit providers or peer ASNs."
//
// The plugin declares ZERO filters at Stage 1, exactly as filter_aspath does:
// filter names come from config (Stage 2), never from compile-time
// registration. The registration declares the filter TYPE instead, and
// BuildFilterRegistry discovers each instance from the ze:filter marker on the
// YANG list.
package filter_path_asn

import (
	"errors"
	"fmt"
	"net"
	"sync/atomic"

	"github.com/ze-software/ze/internal/component/bgp/configjson"
	"github.com/ze-software/ze/internal/component/bgp/filtertext"
	"github.com/ze-software/ze/internal/core/slogutil"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

var errInvalidBGPConfigJSON = errors.New("filter-path-asn: invalid bgp config JSON")

var logger = slogutil.LazyLogger("bgp.filter.path.asn")

const (
	configRootBGP = "bgp"

	// directionImport is the direction string the ingress chain supplies
	// ((*Reactor).runIngressPolicyChain, internal/component/bgp/reactor/filter_ordered.go).
	// It is the one direction whose PeerAS names the peer that SENT the route.
	directionImport = "import"
)

// listsByName holds the reject-asn lists the last configure delivery carried,
// keyed by the list name an operator wrote. It is replaced whole on every
// delivery so the hot path reads it without a lock.
//
// A nil pointer means no delivery has arrived yet, which handleFilterUpdate
// answers by rejecting: it is a state, not an empty list.
//
// Safe for concurrent use. The map it points at MUST NOT be written after the
// Store, because every filter-update reads it without a lock.
var listsByName atomic.Pointer[map[string]*rejectList]

// runFilterPathASN runs the reject-asn filter plugin over the SDK RPC protocol.
// This is the in-process entry point called through InternalPluginRunner.
func runFilterPathASN(conn net.Conn) int {
	p := sdk.NewWithConn("bgp-filter-path-asn", conn)
	defer p.Close() //nolint:errcheck // best-effort cleanup

	p.OnConfigure(configure)

	p.OnFilterUpdate(func(in *sdk.FilterUpdateInput) (*sdk.FilterUpdateOutput, error) {
		return handleFilterUpdate(in), nil
	})

	p.OnExecuteCommand(func(_, command string, args []string, _ string) (string, any, error) {
		return handleCommand(command, args)
	})

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	if err := p.Run(ctx, sdk.Registration{
		Commands: []sdk.CommandDecl{
			{Name: cmdShowRejectASN, Description: "Show every reject-asn list with its ASNs, positions and attached peers"},
			{Name: cmdShowRejectASNName, Description: "Show one reject-asn list by name", Args: []string{"<name>"}},
			// The description says "well-known" rather than naming the table
			// this command reads: TestCuratedTableDecidesNothing asserts that
			// no file deciding anything mentions it, and this file decides.
			{Name: cmdShowRejectASNTransitFree, Description: "Print the well-known transit-free ASNs as a config block"},
		},
		WantsConfig: []string{configRootBGP},
	}); err != nil {
		logger().Error("filter-path-asn plugin failed", "error", err)
		return 1
	}
	return 0
}

// configure reads one Stage 2 config delivery and replaces the lists whole.
//
// A delivery that does not parse stops the load, so a config an operator
// believes is filtering never reaches a session half-applied.
func configure(sections []sdk.ConfigSection) error {
	for _, section := range sections {
		if section.Root != configRootBGP {
			continue
		}
		bgpCfg, ok := configjson.ParseBGPSubtree(section.Data)
		if !ok {
			return errInvalidBGPConfigJSON
		}
		lists, err := parseRejectASNLists(bgpCfg)
		if err != nil {
			return fmt.Errorf("filter-path-asn: %w", err)
		}
		listsByName.Store(&lists)

		// The attachment counts are stored after the lists and only when the
		// parse succeeded, so `show bgp reject-asn` can never report a peer
		// count against a set of lists the plugin refused.
		counts := countAttachments(bgpCfg)
		attachmentsByList.Store(&counts)

		logger().Debug("configured", "reject-asn-lists", len(lists))
	}
	return nil
}

// senderOf reads the sending peer's ASN off the filter input.
//
// FilterUpdateInput.PeerAS means the peer that SENT the route on import, and the
// DESTINATION peer on export ((*Peer).buildForwardFacts,
// internal/component/bgp/reactor/reactor_api_forward.go). So the `direct`
// position has an input on import and none on export, where `indirect`
// therefore covers the whole path. This is the one place the direction is read.
//
// A direction that is neither is treated as export. An unknown sender puts more
// of the path under transit and origin, so the unrecognized case rejects more
// rather than less.
func senderOf(in *sdk.FilterUpdateInput) senderASN {
	if in.Direction != directionImport {
		return senderASN{}
	}
	return senderASN{asn: in.PeerAS, known: true}
}

// handleFilterUpdate answers one filter-update RPC.
//
//   - the list holds an ASN the path carries at a position it rejects: reject,
//     naming the ASN and the position
//   - a pattern of the list matches the whole path: reject, naming the pattern
//   - neither: accept. That is also the answer for an UPDATE with no AS_PATH,
//     because an empty path carries no token for a position to match (AC-19)
//   - the named list is not held: reject, fail-closed (AC-22)
//   - no configure delivery has arrived: reject, fail-closed (AC-23)
//
// Every reject increments ze_filter_path_asn_rejects_total under the direction
// and the slot that decided it (recordReject, metrics.go). An accept is counted
// nowhere: the engine already counts the routes that pass.
//
// in.Filter is the BARE list name. PolicyFilterChain
// (internal/component/bgp/reactor/filter_chain.go) cuts the plugin prefix off
// the chain reference before the RPC, so a chain naming
// bgp-filter-path-asn:NO-TRANSIT reaches here as NO-TRANSIT.
func handleFilterUpdate(in *sdk.FilterUpdateInput) *sdk.FilterUpdateOutput {
	held := listsByName.Load()
	if held == nil {
		recordReject(in.Direction, slotUnconfigured)
		logger().Warn("reject-asn filter-update before configure -- fail-closed",
			"filter", in.Filter, "direction", in.Direction, "peer", in.Peer)
		return &sdk.FilterUpdateOutput{Action: sdk.FilterReject}
	}
	list, ok := (*held)[in.Filter]
	if !ok {
		recordReject(in.Direction, slotUnknownList)
		logger().Warn("unknown reject-asn list -- fail-closed",
			"filter", in.Filter, "direction", in.Direction, "peer", in.Peer)
		return &sdk.FilterUpdateOutput{Action: sdk.FilterReject}
	}

	asPath := filtertext.ASPath(in.Update)

	if found, ok := matchPositions(asPath, senderOf(in), list.positions, list.nth); ok {
		recordReject(in.Direction, rejectSlot(found.at))
		// An nth reject names the collapsed position it matched at, because
		// "nth" alone does not tell the operator which of their rules fired.
		if found.at == positionNth {
			logger().Info("reject-asn reject",
				"filter", list.name, "direction", in.Direction, "peer", in.Peer,
				"asn", found.asn, "position", found.at.String(), "index", found.index,
				"as-path", asPath)
			return &sdk.FilterUpdateOutput{Action: sdk.FilterReject}
		}
		logger().Info("reject-asn reject",
			"filter", list.name, "direction", in.Direction, "peer", in.Peer,
			"asn", found.asn, "position", found.at.String(), "as-path", asPath)
		return &sdk.FilterUpdateOutput{Action: sdk.FilterReject}
	}
	if pattern, ok := matchPattern(asPath, list.patterns); ok {
		recordReject(in.Direction, slotPattern)
		logger().Info("reject-asn reject",
			"filter", list.name, "direction", in.Direction, "peer", in.Peer,
			"pattern", pattern.String(), "as-path", asPath)
		return &sdk.FilterUpdateOutput{Action: sdk.FilterReject}
	}

	logger().Debug("reject-asn accept",
		"filter", list.name, "direction", in.Direction, "peer", in.Peer, "as-path", asPath)
	return &sdk.FilterUpdateOutput{Action: sdk.FilterAccept}
}
