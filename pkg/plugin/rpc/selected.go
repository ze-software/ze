// Design: docs/architecture/api/process-protocol.md -- selected-route output.
// Related: types.go -- StoredRoute remains the legacy retained-wire carrier.
package rpc

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
)

// MethodSelectRoutes submits one bounded destination batch and answers only
// after its final writer settles. Queue admission alone MUST NOT answer success.
const MethodSelectRoutes = "ze-plugin-engine:select-routes"

// SelectedRouteKind names the payload variant. Zero is never an admission.
// The numeric JSON values are part of the public plugin contract.
type SelectedRouteKind uint8

const (
	SelectedRouteUnspecified SelectedRouteKind = iota
	SelectedRouteReceived
	SelectedRouteOrigin
	SelectedRouteAbsent
)

// SelectedStaleLevel is the deciding owner's explicit stale treatment, whose
// authority is validated against the engine-issued received-source receipt.
type SelectedStaleLevel uint8

const (
	SelectedStaleUnspecified SelectedStaleLevel = iota
	SelectedFresh
	SelectedStale
	SelectedLLGR
)

// SelectedRoute is raw input, not forwarding authority. KeyNLRI is native
// advertisement framing with the source ADD-PATH identifier removed. The engine
// derives its registered semantic key; it MUST NOT project every family to IP.
// A received path's identifier, including zero, remains separate in Route.
// Route is absent only for SelectedRouteAbsent. Source and origin receipts are
// engine-issued handles; clients cannot name an owner or destination incarnation.
type SelectedRoute struct {
	Kind          SelectedRouteKind  `json:"kind"`
	Family        string             `json:"family"`
	KeyNLRI       []byte             `json:"key-nlri"`
	Revision      uint64             `json:"revision"`
	Route         *StoredRoute       `json:"route,omitempty"`
	SourceReceipt uint64             `json:"source-receipt,omitempty"`
	OriginReceipt uint64             `json:"origin-receipt,omitempty"`
	SourceAddPath bool               `json:"source-add-path,omitempty"`
	Retained      bool               `json:"retained,omitempty"`
	StaleLevel    SelectedStaleLevel `json:"stale-level,omitempty"`
	// FlowSpecRedirectRefused is the selecting owner's current authorization
	// projection. The engine accepts it only on a received FlowSpec selection
	// and strips the redirect action through its existing egress treatment.
	FlowSpecRedirectRefused bool `json:"flowspec-redirect-refused,omitempty"`
}

// SelectRoutesInput borrows its fields until SelectRoutes returns. The caller
// MUST leave them unchanged for that interval. DestinationSession is the exact
// InitialReplay token of the destination's UP, for live output as well as replay.
// Revisions increase per native key and admitted scope in one owner lifetime.
type SelectRoutesInput struct {
	Destination        string `json:"destination"`
	DestinationSession uint64 `json:"destination-session"`
	// DeadlineMS is the absolute Unix-millisecond work bound. SDK calls derive
	// it from their context; a socket request MUST carry a positive deadline.
	DeadlineMS int64           `json:"deadline-ms,omitempty"`
	Routes     []SelectedRoute `json:"routes"`
}

// SelectedRouteOutcome is a final receipt. Zero and unknown values are refused.
type SelectedRouteOutcome uint8

const (
	SelectedOutcomeUnspecified SelectedRouteOutcome = iota
	SelectedWritten
	SelectedSuppressed
	SelectedAbsent
	SelectedStaleRevision
)

// SelectedRouteResult echoes one requested identity and revision. CurrentRevision
// is the final writer's stream frontier, not a sent-event or source message ID.
// Suppressed and Absent confirm that the selected output has been retired.
type SelectedRouteResult struct {
	Family          string               `json:"family"`
	KeyNLRI         []byte               `json:"key-nlri"`
	Revision        uint64               `json:"revision"`
	CurrentRevision uint64               `json:"current-revision"`
	Outcome         SelectedRouteOutcome `json:"outcome"`
}

// SelectRoutesOutput contains exactly one ordered final receipt per request.
type SelectRoutesOutput struct {
	Results []SelectedRouteResult `json:"results"`
}

// Validate checks result completeness on both transports. An empty/missing
// result never means that a requested write or retirement completed.
func (out *SelectRoutesOutput) Validate(input SelectRoutesInput) error {
	if out == nil {
		return errors.New("select-routes: final result is missing")
	}
	if len(out.Results) != len(input.Routes) {
		return fmt.Errorf("select-routes: got %d final results for %d routes", len(out.Results), len(input.Routes))
	}
	for i := range input.Routes {
		want, got := &input.Routes[i], &out.Results[i]
		if got.Family != want.Family {
			return fmt.Errorf("select-routes: result %d has a different family", i)
		}
		if !bytes.Equal(got.KeyNLRI, want.KeyNLRI) {
			return fmt.Errorf("select-routes: result %d has a different native key", i)
		}
		if got.Revision != want.Revision {
			return fmt.Errorf("select-routes: result %d has a different requested revision", i)
		}
		if err := selectedResultRevision(got, want.Kind); err != nil {
			return fmt.Errorf("select-routes: result %d: %w", i, err)
		}
	}
	return nil
}

func selectedResultRevision(result *SelectedRouteResult, kind SelectedRouteKind) error {
	// The result is open plugin/engine wire input; unknown values fail closed.
	switch result.Outcome {
	case SelectedWritten:
		if kind == SelectedRouteAbsent {
			return errors.New("absence cannot complete as an announcement")
		}
	case SelectedSuppressed, SelectedAbsent:
	case SelectedStaleRevision:
		if result.CurrentRevision < result.Revision {
			return errors.New("stale result has an older current revision")
		}
		return nil
	case SelectedOutcomeUnspecified:
		return errors.New("final outcome is unspecified")
	default:
		return fmt.Errorf("final outcome %d is unknown", result.Outcome)
	}
	if result.CurrentRevision != result.Revision {
		return errors.New("completed result has a different current revision")
	}
	return nil
}

// SelectRoutesFrameSize measures the actual JSON envelope with the widest RPC
// correlation ID. Batch producers MUST keep this at or below MaxMessageSize;
// route count alone cannot bound hex/base64 payloads or escaped family names.
func SelectRoutesFrameSize(input SelectRoutesInput) (int, error) {
	encoded, err := json.Marshal(input)
	if err != nil {
		return 0, err
	}
	return idPrefixMax + len(MethodSelectRoutes) + 1 + len(encoded), nil
}

// SelectRoutesExchangeSize bounds both frames before a batch can be applied.
// A selected absence's result is larger than its request. The answer reserves
// the widest current revision, even when a stale request has a smaller one;
// the request reserves the absolute deadline the SDK adds before sending.
func SelectRoutesExchangeSize(input SelectRoutesInput) (int, error) {
	input.DeadlineMS = math.MaxInt64
	request, err := SelectRoutesFrameSize(input)
	if err != nil {
		return 0, err
	}
	output := SelectRoutesOutput{Results: make([]SelectedRouteResult, len(input.Routes))}
	for i, route := range input.Routes {
		output.Results[i] = SelectedRouteResult{Family: route.Family, KeyNLRI: route.KeyNLRI,
			Revision: route.Revision, CurrentRevision: math.MaxUint64, Outcome: SelectedStaleRevision}
	}
	encoded, err := json.Marshal(output)
	if err != nil {
		return 0, err
	}
	response := idPrefixMax + len("ok ") + len(encoded)
	if response > request {
		return response, nil
	}
	return request, nil
}
