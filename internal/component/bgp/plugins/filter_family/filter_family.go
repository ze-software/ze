// Design: docs/architecture/core-design.md -- address-family policy filter
// Related: config.go -- policy/family-filter config parsing + tear-down-in-export guard
// Related: match.go -- family extraction from the raw UPDATE body
// Related: handler.go -- per-update remove / tear-down decision
// RFC: rfc/short/rfc4760.md -- multiprotocol families (Section 6)
// RFC: rfc/short/rfc4271.md -- NOTIFICATION + session close (Section 6)
//
// Package filter_family implements the bgp-filter-family plugin.
//
// The plugin loads named family-filter definitions from
// policy { family-filter NAME { family ipv4/flow; action remove } }
// at Stage 2 and on every reload (config_tx.go). At runtime, peer filter chains reference an instance as
// bgp-filter-family:NAME. The engine calls the plugin's one evaluator
// (OnFilter: the typed bridge slot internal, the filter-update batch forked),
// which extracts the UPDATE's address family from the raw wire body (the
// declared raw read) and applies the instance's action:
//
//   - remove: if the family is all the UPDATE carries, reject it (whole-UPDATE
//     suppress); otherwise modify with the family-strip intent, which the
//     engine resolves by dropping the family's MP_REACH/MP_UNREACH NLRI
//     (RFC 4760 §6, D-5). The declared family read admits the intent.
//   - tear-down (import only): request a NOTIFICATION + session close (RFC 4271).
//
// The plugin declares one filter with the name pattern ".+" and the family and raw reads, so every
// referenced instance receives the wire payload; filter names come from config,
// not compile-time registration (same model as filter_remove_private_as).
package filter_family

import (
	"errors"
	"net"
	"sync/atomic"

	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/slogutil"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

var errFilterFamilyInvalidBgpConfigJSON = errors.New("filter-family: invalid bgp config JSON")

var logger = slogutil.LazyLogger("bgp.filter.family")

const configRootBGP = "bgp"

// fieldFamily is the route field the filter reads and the intent it answers
// names: a filter that reads family may answer a family remove (D-5).
var fieldFamily = policy.FieldFamily.String()

// instancesByName is the runtime-loaded set of family-filter instances, keyed by
// the YANG list name. Stored whole by installFamilyFilters (Stage 2 and each
// reload's apply and rollback) so the hot path reads without a lock.
var instancesByName atomic.Pointer[map[string]*familyFilter]

// runFilterFamily runs the family filter plugin using the SDK RPC protocol.
// This is the in-process entry point called via InternalPluginRunner.
func runFilterFamily(conn net.Conn) int {
	p := sdk.NewWithConn("bgp-filter-family", conn)
	defer p.Close() //nolint:errcheck // best-effort cleanup

	newFamilyConfigTx().Register(p)

	// One evaluator for both transports (D-12): the typed bridge slot when
	// the plugin runs internal, the filter-update batch when it runs forked.
	p.OnFilter(evaluateFamilyFilter)

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	if err := p.Run(ctx, &sdk.Registration{
		// The policy root is listed although the registry's ConfigReads names
		// it: a forked process has no registry row (registryRow refuses an
		// external program), so Stage 1 is the only declaration it carries,
		// and without it the definitions under policy never arrive.
		WantsConfig: []string{configRootBGP, policy.RootName},
		Filters: []sdk.FilterDecl{{
			NamePattern: ".+",
			Direction:   sdk.FilterBoth,
			Reads:       []string{fieldFamily, sdk.FilterFieldRaw},
			OnError:     sdk.OnErrorReject,
		}},
	}); err != nil {
		logger().Error("filter-family plugin failed", "error", err)
		return 1
	}
	return 0
}
