// Design: docs/architecture/bgp/structural-forwarding.md -- route-server transparency
// Related: register.go -- the compiled registration that carries the declarations
// Related: server.go -- routeServerRegistration, the Stage 1 declaration that carries them

package rs

import (
	"errors"
	"fmt"

	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	// rsPluginName is the registration name a member reference starts with.
	rsPluginName = "bgp-rs"
	// transparencyFilterName is the export step every route-server client's
	// list carries.
	transparencyFilterName = "transparency"
	// transparencyPriority places the step after the protocol procedures
	// other plugins insert (bgp-role's at 100, bgp-gr's egress at 150): it
	// changes no attribute, so its place decides nothing but the order a
	// reader of the list sees.
	transparencyPriority = 200
	// sessionFactTrue is the text form of a set flag fact.
	sessionFactTrue = "true"
)

// transparencySets is the step's one answer: the treatment set. It is shared
// by every answer and MUST NOT be modified.
var transparencySets = []rpc.FilterSet{{
	Field:  policyview.FieldPreserveOpaqueAttributes.String(),
	Action: rpc.FilterSetActionSet,
	Value:  []byte(sessionFactTrue),
}}

// rsFilterDecls declares the transparency step. It has two readers and MUST
// stay one function: register.go puts it on registry.Registration.Filters and
// routeServerRegistration sends it at Stage 1. The insertion's session
// condition already chose the destinations it runs for, so the step needs no
// fact; it declares rs-client, the one it is placed by, because a filter that
// declares no read is sent the whole text update of every route. A failure
// rejects, as every procedure a plugin inserts does: a route this
// plugin cannot judge is never sent on.
func rsFilterDecls() []sdk.FilterDecl {
	return []sdk.FilterDecl{{
		Name:      transparencyFilterName,
		Direction: sdk.FilterExport,
		Reads:     []string{policyview.FieldRSClient.String()},
		Sets:      []string{policyview.FieldPreserveOpaqueAttributes.String()},
		OnError:   sdk.OnErrorReject,
	}}
}

// rsInsertions places the transparency step on the export list of every
// peer whose session is a route-server client (`session rs-client`). It is
// not required: RFC 7947 Section 2.2 makes transparency a SHOULD NOT update
// "unless enforced by local IXP operator configuration", so an operator may
// name it `inactive:` to run a client's list without it. Like rsFilterDecls
// it has two readers and MUST stay one function.
func rsInsertions() []rpc.FilterInsertion {
	return []rpc.FilterInsertion{{
		Filter: rsPluginName + ":" + transparencyFilterName, Direction: rpc.FilterExport,
		Priority: transparencyPriority,
		Session:  []rpc.FilterSessionFact{{Fact: policyview.FieldRSClient.String(), Values: []string{sessionFactTrue}}},
	}}
}

// errNoSuchRSFilter fails a call for a filter this plugin does not serve.
var errNoSuchRSFilter = errors.New("bgp-rs: the engine called a filter this plugin does not declare")

// evaluateRSFilter is the plugin's one filter evaluator (sdk.Plugin.OnFilter),
// on both transports: the typed slot internal, the batched filter-update
// forked.
//
// RFC 7947 Section 2.2: "Optional recognized and unrecognized BGP attributes,
// whether transitive or non-transitive, SHOULD NOT be updated by the route server
// (unless enforced by local IXP operator configuration) and SHOULD be passed on
// to other route server clients."
// The step answers every route toward a client with the treatment
// preserve-opaque-attributes set, which the forward path applies in place of
// RFC 4271 Section 5's ordinary handling of the attributes it does not
// recognize.
func evaluateRSFilter(route *sdk.FilterRoute) (*sdk.FilterUpdateOutput, error) {
	if route.Filter() != transparencyFilterName {
		return nil, fmt.Errorf("%w: %q", errNoSuchRSFilter, route.Filter())
	}
	return &sdk.FilterUpdateOutput{Action: sdk.FilterModify, Sets: transparencySets}, nil
}
