// Design: docs/architecture/core-design.md -- remove-private-as policy action filter
// Detail: config.go -- policy/remove-private-as config parsing
// Detail: private_as.go -- Private Use ASN detection

package filter_remove_private_as

import (
	"errors"
	"net"
	"sync/atomic"

	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

var (
	errRemovePrivateASInvalidBgpConfigJSON = errors.New("filter-remove-private-as: invalid bgp config JSON")
	errFilterBeforeConfigure               = errors.New("filter-remove-private-as: a route arrived before the configuration")
	errUnknownRemovePrivateAS              = errors.New("filter-remove-private-as: no remove-private-as of this name is configured")
)

// fieldASPath is the route field the filter reads and the intent it answers
// names (policy.FieldASPath on the wire).
const fieldASPath = "as-path"

var logger = slogutil.LazyLogger("bgp.filter.remove-private-as")

var defsByName atomic.Pointer[map[string]*removePrivateASDef]

func runFilterRemovePrivateAS(conn net.Conn) int {
	p := sdk.NewWithConn("bgp-filter-remove-private-as", conn)
	defer p.Close() //nolint:errcheck // best-effort cleanup

	newRemovePrivateASConfigTx().Register(p)

	// One evaluator for both transports (D-12): the typed bridge slot when
	// the plugin runs internal, the filter-update batch when it runs forked.
	p.OnFilter(evaluateRemovePrivateAS)

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
			Reads:       []string{fieldASPath, sdk.FilterFieldRaw},
			Sets:        []string{fieldASPath},
			OnError:     sdk.OnErrorReject,
		}},
	}); err != nil {
		logger().Error("filter-remove-private-as plugin failed", "error", err)
		return 1
	}
	return 0
}

// evaluateRemovePrivateAS answers the remove-private intent for a route whose
// AS_PATH or AS4_PATH carries a Private Use ASN, and accepts the rest
// unchanged. The engine performs the rewrite of both attributes, in peer-as
// mode with the session's peer AS (policychain RemovePrivateAS). It reads
// only the configured definitions, swapped whole by the config transaction
// (defsByName), so it is safe for concurrent use and never blocks.
func evaluateRemovePrivateAS(route *sdk.FilterRoute) (*sdk.FilterUpdateOutput, error) {
	defsP := defsByName.Load()
	if defsP == nil {
		return nil, errFilterBeforeConfigure
	}
	def, ok := (*defsP)[route.Filter()]
	if !ok {
		return nil, errUnknownRemovePrivateAS
	}
	fields, err := route.Fields()
	if err != nil {
		return nil, err
	}
	if !asPathFieldHasPrivateASN(fields) && !hasPrivateAS4PathPayload(route.Raw()) {
		return &sdk.FilterUpdateOutput{Action: sdk.FilterAccept}, nil
	}
	logger().Info("remove-private-as modify", "filter", route.Filter(), "peer", route.Peer(), "mode", def.mode.String())
	return &sdk.FilterUpdateOutput{Action: sdk.FilterModify, Sets: []rpc.FilterSet{{
		Field: fieldASPath, Action: rpc.FilterSetActionRemove, Value: def.mode.intentValue(),
	}}}, nil
}
