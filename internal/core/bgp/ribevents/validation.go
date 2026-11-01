// Design: docs/architecture/plugin/rib-storage-design.md -- validation eligibility between receive stores
// RFC: rfc/short/draft-ietf-sidrops-aspa-verification.md -- Section 5.7 retained ineligible routes
package ribevents

import (
	"encoding/hex"
	"encoding/json"
	"net/netip"
	"sync/atomic"

	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// ValidationRoute identifies a received path without its attribute bytes.
type ValidationRoute struct {
	Peer   netip.Addr
	Family family.Family
	Prefix netip.Prefix
	PathID uint32
	// NLRI is the full native wire encoding for a non-CIDR route, without
	// ADD-PATH. FlowSpec uses FlowSpecKey's shortest length encoding.
	// A string keeps the key comparable and owns its bytes.
	NLRI string
}

// validationRouteJSON is ValidationRoute as a plugin-process subscriber of
// ValidationChange receives it: kebab-case keys, and the NLRI as hex, because
// JSON replaces bytes that are not valid UTF-8 and a FlowSpec key is binary.
type validationRouteJSON struct {
	Peer   netip.Addr    `json:"peer-address"`
	Family family.Family `json:"family"`
	Prefix netip.Prefix  `json:"prefix"`
	PathID uint32        `json:"path-id"`
	NLRI   string        `json:"nlri,omitempty"`
}

// MarshalJSON writes the route in the form validationRouteJSON names.
func (r ValidationRoute) MarshalJSON() ([]byte, error) {
	return json.Marshal(validationRouteJSON{
		Peer:   r.Peer,
		Family: r.Family,
		Prefix: r.Prefix,
		PathID: r.PathID,
		NLRI:   hex.EncodeToString([]byte(r.NLRI)),
	})
}

// UnmarshalJSON reads the form MarshalJSON writes. An nlri that is not hex
// refuses the route, so a decoder never invents a key; r changes only on
// success.
func (r *ValidationRoute) UnmarshalJSON(data []byte) error {
	var wire validationRouteJSON
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	nlri, err := hex.DecodeString(wire.NLRI)
	if err != nil {
		return err
	}
	*r = ValidationRoute{Peer: wire.Peer, Family: wire.Family, Prefix: wire.Prefix, PathID: wire.PathID, NLRI: string(nlri)}
	return nil
}

// ValidationLookup reads the authoritative Adj-RIB-In eligibility. A zero msgID
// asks about the current route; a nonzero msgID MUST match its received UPDATE.
// Implementations MUST be safe for concurrent calls and MUST NOT emit events.
type ValidationLookup func(route ValidationRoute, msgID uint64) bool

type validationProvider struct {
	eligible ValidationLookup
	present  func(ValidationRoute) bool
}

var validationLookup atomic.Pointer[validationProvider]

// RegisterValidationLookup installs the receive gate before peer events start.
// The owner MUST unregister with nil after stopping event delivery.
func RegisterValidationLookup(fn ValidationLookup, present func(ValidationRoute) bool) {
	if fn == nil {
		validationLookup.Store(nil)
		return
	}
	validationLookup.Store(&validationProvider{eligible: fn, present: present})
}

// gateLogger reports a receive gate this process cannot reach.
var gateLogger = slogutil.LazyLogger("bgp.ribevents")

// unservedValidation is the provider a process holds when the receive gate's
// owner enabled validation in another process: the lookup function lives
// there, so nothing here can answer it. It refuses every route (fail closed),
// because answering eligible would forward and select routes RPKI and ASPA
// never judged, with no error anywhere. The first refusal is logged, so an
// operator learns why nothing is advertised.
var unservedValidation = &validationProvider{
	eligible: refuseUnservedRoute,
	present:  func(ValidationRoute) bool { return false },
}

// unservedRefusalLogged makes refuseUnservedRoute log once per unserved period.
var unservedRefusalLogged atomic.Bool

func refuseUnservedRoute(route ValidationRoute, _ uint64) bool {
	if unservedRefusalLogged.CompareAndSwap(false, true) {
		gateLogger().Error("route refused: the receive validation gate is enabled in another process and cannot be read from this one",
			"peer", route.Peer, "family", route.Family.String(), "prefix", route.Prefix,
			"fix", "run bgp-adj-rib-in internal (in the daemon), where its eligibility lookup is reachable")
	}
	return false
}

// EventValidationGate announces the receive gate's state. The gate's owner
// emits it whenever validation is enabled or disabled, on both transports, so
// every process that reads RouteEligible learns that a gate exists even when
// the lookup function itself lives in another process.
const EventValidationGate = "validation-gate"

// ValidationGateState is the EventValidationGate payload.
type ValidationGateState struct {
	Enabled bool `json:"enabled"`
}

// ValidationGate is the typed handle of EventValidationGate.
var ValidationGate = events.Register[ValidationGateState](Namespace, EventValidationGate)

// NoteValidationGate applies an EventValidationGate announcement to this
// process. A process that holds the owner's lookup (RegisterValidationLookup
// ran here) keeps it: the announcement changes nothing. A process that holds
// none fails closed while the gate is enabled, and returns to "no validation"
// when it is disabled. It never replaces or removes a registered lookup.
// Safe for concurrent use.
func NoteValidationGate(state ValidationGateState) {
	if !state.Enabled {
		validationLookup.CompareAndSwap(unservedValidation, nil)
		return
	}
	if validationLookup.CompareAndSwap(nil, unservedValidation) {
		unservedRefusalLogged.Store(false)
		gateLogger().Error("receive validation gate unreachable: every route is refused until it is",
			"reason", "validation was enabled in another process, and its eligibility lookup does not cross the process boundary",
			"fix", "run bgp-adj-rib-in internal (in the daemon)")
	}
}

// DecodeValidationGate reads an enveloped delivered EventValidationGate, as
// DecodeValidationChange reads its event: matched reports the event, and err
// refuses a payload that does not decode.
func DecodeValidationGate(delivered string) (state ValidationGateState, matched bool, err error) {
	payload, matched := envelopedPayload(delivered, EventValidationGate)
	if !matched {
		return ValidationGateState{}, false, nil
	}
	if err := json.Unmarshal(payload, &state); err != nil {
		return ValidationGateState{}, true, err
	}
	return state, true, nil
}

// ValidationEnabled reports whether a receive validator has enabled its gate.
// The owner registers its lookup when enabling validation and unregisters at
// stop. A gate enabled in another process (NoteValidationGate) also counts:
// the forward path and selection must then consult RouteEligible, which
// refuses.
func ValidationEnabled() bool {
	return validationLookup.Load() != nil
}

// RouteEligible combines independent validators. FlowSpec fails closed until
// the selecting RIB has authorized the exact received generation.
func RouteEligible(route ValidationRoute, msgID uint64) bool {
	if IsFlowSpec(route.Family) {
		provider := flowSpecLookup.Load()
		if provider == nil || !provider.eligible(route, msgID) {
			return false
		}
	}
	fn := validationLookup.Load()
	if fn == nil {
		return true
	}
	return fn.eligible(route, msgID)
}

// RoutePresent reports whether the receive store still owns a pending or
// installed path, including an ineligible one. Retained paths must keep their
// regenerated ADD-PATH identity across temporary validation withdrawals.
// Without a receive gate there is no retained validation ownership.
func RoutePresent(route ValidationRoute) bool {
	if IsFlowSpec(route.Family) {
		provider := flowSpecLookup.Load()
		return provider != nil && provider.present != nil && provider.present(route)
	}
	provider := validationLookup.Load()
	return provider != nil && provider.present != nil && provider.present(route)
}

// EventValidationChange requests selection after the receive gate changes.
const EventValidationChange = "validation-change"

// ValidationChange names paths whose eligibility may have changed. Producers
// MUST emit after releasing the receive-store lock. Consumers MUST re-read the
// gate rather than trusting event order; independent plugin delivery can race.
var ValidationChange = events.Register[[]ValidationRoute](Namespace, EventValidationChange)

// DecodeValidationChange reads an enveloped delivered event: matched reports
// whether it is the ValidationChange event, and err refuses a batch that does
// not decode, so a consumer never acts on half a batch. A plugin process
// subscribed to ValidationChange through the SDK (AddStartupSubscriptionsIn
// with SetEnvelope) receives it this way on both transports, where the
// typed Subscribe reaches only a subscriber holding the daemon's own bus.
func DecodeValidationChange(delivered string) (routes []ValidationRoute, matched bool, err error) {
	payload, matched := envelopedPayload(delivered, EventValidationChange)
	if !matched {
		return nil, false, nil
	}
	if err := json.Unmarshal(payload, &routes); err != nil {
		return nil, true, err
	}
	return routes, true, nil
}

// envelopedPayload returns the payload of delivered when it is the enveloped
// event eventType of this namespace. A text event, another namespace or
// another event is not matched; nothing is refused.
func envelopedPayload(delivered, eventType string) (json.RawMessage, bool) {
	if delivered == "" {
		return nil, false
	}
	if delivered[0] != '{' {
		return nil, false
	}
	envelope, err := rpc.ParseEventEnvelope(delivered)
	if err != nil {
		return nil, false //nolint:nilerr // a text event is not an envelope: not this event, nothing refused
	}
	if envelope.Namespace != Namespace {
		return nil, false
	}
	if envelope.Event != eventType {
		return nil, false
	}
	return envelope.Payload, true
}
