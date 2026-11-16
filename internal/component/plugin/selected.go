// Design: docs/architecture/api/process-protocol.md -- authenticated selected output.
// Related: sender.go -- process names alone do not identify a process lifetime.
package plugin

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"slices"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// SelectedOwner is an authenticated process lifetime, captured by the server.
// Its zero value is invalid. The private cancellation channel distinguishes two
// instances of the same process name and lets the final writer reject old work.
// Safe for concurrent use; a captured value MUST be rechecked before writing.
type SelectedOwner struct {
	sender Sender
	done   <-chan struct{}
	ended  <-chan struct{}
	currentSelection *selectedCurrentReader
}

// NewSelectedOwner captures a live process; an operator is not a selecting owner.
func NewSelectedOwner(sender Sender, done, ended <-chan struct{}) (SelectedOwner, error) {
	if _, named := sender.Process(); !named {
		return SelectedOwner{}, errors.New("select-routes: caller is not a named process")
	}
	if done == nil {
		return SelectedOwner{}, errors.New("select-routes: process lifetime is unavailable")
	}
	owner := SelectedOwner{sender: sender, done: done, ended: ended}
	if err := owner.Current(); err != nil {
		return SelectedOwner{}, err
	}
	return owner, nil
}

// Current checks lifetime revocation; it does not grant permission to a peer.
func (owner SelectedOwner) Current() error {
	if owner.done == nil {
		return errors.New("select-routes: process lifetime is unavailable")
	}
	select {
	case <-owner.done:
		return errors.New("select-routes: selecting process has stopped")
	case <-owner.ended:
		return errors.New("select-routes: selecting engine has ended")
	default:
		return nil
	}
}

// Sender answers the authenticated sender for the existing grant guard.
func (owner SelectedOwner) Sender() Sender { return owner.sender }

// Done is the captured process-incarnation cancellation, never a successor's.
func (owner SelectedOwner) Done() <-chan struct{} { return owner.done }

// Ended additionally fences an in-process engine that returns before Stop.
// It is nil for a fork, whose exit cancels Done.
func (owner SelectedOwner) Ended() <-chan struct{} { return owner.ended }

// SameIncarnation compares captured lifetimes, not only configured names.
func (owner SelectedOwner) SameIncarnation(other SelectedOwner) bool {
	return owner.done != nil && owner == other
}

// SelectedBatch is shape-validated selected input with owned snapshot storage.
// Its zero value and nil pointer are invalid. The constructor does not confer
// forwarding permission: receipt authenticity, effective role, source-policy
// epoch, native identity and destination session are the reactor's checks.
type SelectedBatch struct {
	destination netip.Addr
	input       rpc.SelectRoutesInput
}

// NewSelectedBatch validates external DTOs once and takes an owned snapshot.
// A caller MUST keep input unchanged until this constructor returns.
func NewSelectedBatch(input rpc.SelectRoutesInput) (*SelectedBatch, error) {
	destination, err := netip.ParseAddr(input.Destination)
	if err != nil {
		return nil, fmt.Errorf("select-routes: invalid destination: %w", err)
	}
	if input.DestinationSession == 0 {
		return nil, errors.New("select-routes: destination session is unrecorded")
	}
	size, err := rpc.SelectRoutesExchangeSize(input)
	if err != nil {
		return nil, err
	}
	if size > rpc.MaxMessageSize {
		return nil, fmt.Errorf("select-routes: encoded request or final receipt is %d bytes, maximum %d", size, rpc.MaxMessageSize)
	}
	for i := range input.Routes {
		if err := validateSelectedRoute(&input.Routes[i]); err != nil {
			return nil, fmt.Errorf("select-routes: route %d: %w", i, err)
		}
	}
	batch := &SelectedBatch{destination: destination, input: input}
	batch.input.Routes = slices.Clone(input.Routes)
	for i := range batch.input.Routes {
		route := &batch.input.Routes[i]
		route.KeyNLRI = slices.Clone(route.KeyNLRI)
		if route.Route != nil {
			stored := *route.Route
			route.Route = &stored
		}
	}
	return batch, nil
}

// Destination is the parsed address, so the reactor never reparses a string.
func (batch *SelectedBatch) Destination() netip.Addr { return batch.destination }

// Input borrows the validated snapshot until the selected operation completes.
// The consumer MUST NOT mutate it or retain its slices beyond that completion.
func (batch *SelectedBatch) Input() rpc.SelectRoutesInput { return batch.input }

func validateSelectedRoute(route *rpc.SelectedRoute) error {
	if route.Family == "" {
		return errors.New("family is missing")
	}
	if len(route.KeyNLRI) == 0 {
		return errors.New("native key NLRI is missing")
	}
	if route.Revision == 0 {
		return errors.New("selection revision is unrecorded")
	}
	// The DTO is open plugin input; unknown variants are errors, never panics.
	switch route.Kind {
	case rpc.SelectedRouteAbsent:
		return validateSelectedAbsence(route)
	case rpc.SelectedRouteReceived:
		if route.SourceReceipt == 0 {
			return errors.New("received selection has no source receipt")
		}
		if route.OriginReceipt != 0 {
			return errors.New("received selection carries an origin receipt")
		}
	case rpc.SelectedRouteOrigin:
		if route.FlowSpecRedirectRefused {
			return errors.New("origin selection carries received FlowSpec authorization")
		}
		if route.OriginReceipt == 0 {
			return errors.New("origin selection has no origin receipt")
		}
		if route.SourceReceipt != 0 {
			return errors.New("origin selection carries a source receipt")
		}
		if route.Retained {
			return errors.New("origin selection carries received retention authority")
		}
		if route.SourceAddPath {
			return errors.New("origin selection carries received ADD-PATH authority")
		}
	case rpc.SelectedRouteUnspecified:
		return errors.New("selection kind is unspecified")
	default:
		return fmt.Errorf("selection kind %d is unknown", route.Kind)
	}
	if route.Route == nil {
		return errors.New("selected announcement has no stored route")
	}
	if route.Route.Withdraw {
		return errors.New("selected announcement carries a legacy source withdrawal")
	}
	if route.Route.Family != route.Family {
		return errors.New("stored route family differs from selected family")
	}
	if route.Route.NLRIHex == "" {
		return errors.New("selected announcement has no native NLRI")
	}
	for _, value := range []string{route.Route.AttrHex, route.Route.NextHopHex, route.Route.NLRIHex} {
		if err := validateSelectedHex(value); err != nil {
			return err
		}
	}
	// NLRI framing arrives from outside; every known value is handled here.
	switch route.Route.NLRIFraming {
	case rpc.NLRIFramingPrefixOnly, rpc.NLRIFramingSourceWire:
	case rpc.NLRIFramingUnrecorded:
		return errors.New("selected announcement has unrecorded NLRI framing")
	default:
		return errors.New("selected announcement has unknown NLRI framing")
	}
	// Stale level is external input, so unknown values fail closed.
	switch route.StaleLevel {
	case rpc.SelectedFresh, rpc.SelectedStale, rpc.SelectedLLGR:
	case rpc.SelectedStaleUnspecified:
		return errors.New("selected announcement has unspecified stale treatment")
	default:
		return errors.New("selected announcement has unknown stale treatment")
	}
	if route.Kind == rpc.SelectedRouteReceived {
		if _, err := netip.ParseAddr(route.Route.SourcePeer); err != nil {
			return errors.New("received selection has an invalid original source")
		}
		if route.Route.MsgID == 0 {
			return errors.New("received selection has an unrecorded message generation")
		}
		if route.Route.SourcePolicyEpoch == 0 {
			return errors.New("received selection has an unrecorded source-policy epoch")
		}
		return nil
	}
	if route.Route.SourcePeer != "" {
		return errors.New("origin selection carries an original received source")
	}
	if route.Route.SourcePolicyEpoch != 0 {
		return errors.New("origin selection carries received policy admission")
	}
	if route.StaleLevel != rpc.SelectedFresh {
		return errors.New("origin selection carries received stale treatment")
	}
	return nil
}

func validateSelectedHex(value string) error {
	if len(value)%2 != 0 {
		return errors.New("selected stored route has odd-length hex")
	}
	// Input length is bounded by the encoded IPC ceiling before this scan.
	for i := range len(value) {
		switch {
		case value[i] >= '0' && value[i] <= '9':
		case value[i] >= 'a' && value[i] <= 'f':
		case value[i] >= 'A' && value[i] <= 'F':
		default:
			return errors.New("selected stored route has invalid hex")
		}
	}
	return nil
}

func validateSelectedAbsence(route *rpc.SelectedRoute) error {
	if route.FlowSpecRedirectRefused {
		return errors.New("selected absence carries received FlowSpec authorization")
	}
	if route.Route != nil {
		return errors.New("selected absence carries an announcement")
	}
	if route.SourceReceipt != 0 {
		return errors.New("selected absence carries a source receipt")
	}
	if route.OriginReceipt != 0 {
		return errors.New("selected absence carries an origin receipt")
	}
	if route.Retained {
		return errors.New("selected absence carries retention authority")
	}
	if route.SourceAddPath {
		return errors.New("selected absence carries a source ADD-PATH marker")
	}
	if route.StaleLevel != rpc.SelectedStaleUnspecified {
		return errors.New("selected absence carries stale treatment")
	}
	return nil
}

// ReactorSelectedCoordinator is optional until a reactor implements selected
// authority. It MUST validate issued receipts and join final writer completion.
// Neither this method nor a wrapper may answer success for unsupported output.
type ReactorSelectedCoordinator interface {
	SelectRoutes(context.Context, *SelectedBatch, SelectedOwner) (*rpc.SelectRoutesOutput, error)
}
