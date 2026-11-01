// Design: docs/architecture/api/process-protocol.md — the typed filter-update slot (Runtime hot path)
// Related: bridge.go — DirectBridge, whose delivery admission the slot shares
// Related: types.go — FilterUpdateInput and FilterUpdateOutput, the forked form of one route

package rpc

import (
	"errors"
	"fmt"
)

// FilterView is the engine's lazy view of one route, handed to the filter of
// a plugin running internal with nothing encoded. Every answer is borrowed
// from the engine's run: it is valid only until the filter call returns, and
// the filter MUST NOT keep it, or any slice it answered, past that return.
type FilterView interface {
	// Fields answers the fields the filter declared it reads, as the JSON
	// object filter-update's fields member carries, encoded on this first
	// call only. It is nil when the filter declared no route field.
	Fields() (json []byte, err error)
	// Raw answers the raw UPDATE body when the filter declared the
	// FilterFieldRaw read, and nil otherwise.
	Raw() []byte
	// Engine answers the engine's own typed view, for a filter compiled with
	// the engine that reads fields without any encoding. Its type is the
	// engine's to state (for BGP, the policy view), as StructuredEvent's
	// RawMessage is.
	Engine() any
}

// FilterRoute is one route a filter judges, whichever transport carried it.
// A route of a forked plugin holds the decoded filter-update input; a route
// of a plugin running internal holds the input's header (filter, direction,
// peer, peer AS) and the engine's view, so its fields are read in place.
// A FilterRoute is borrowed for one call, like its view.
type FilterRoute struct {
	input *FilterUpdateInput
	view  FilterView
}

// InputFilterRoute answers the route input carries, its fields already
// encoded: a route that arrived over filter-update, or one the text chain
// built.
func InputFilterRoute(input *FilterUpdateInput) FilterRoute {
	return FilterRoute{input: input}
}

// ViewFilterRoute answers the route of a fast-path call: header names the
// filter, direction, peer and peer AS, and view answers the fields. header's
// Update, Fields and Raw are not read.
func ViewFilterRoute(header *FilterUpdateInput, view FilterView) FilterRoute {
	return FilterRoute{input: header, view: view}
}

// Filter answers the name of the filter the route is sent to.
func (r *FilterRoute) Filter() string { return r.input.Filter }

// Direction answers "import" or "export".
func (r *FilterRoute) Direction() string { return r.input.Direction }

// Peer answers the address of the peer the route came from or goes to.
func (r *FilterRoute) Peer() string { return r.input.Peer }

// PeerAS answers that peer's AS number.
func (r *FilterRoute) PeerAS() uint32 { return r.input.PeerAS }

// Fields answers the declared fields as one JSON object, nil when the filter
// declared none. On the fast path the view encodes them on this call.
func (r *FilterRoute) Fields() ([]byte, error) {
	if r.view != nil {
		return r.view.Fields()
	}
	return r.input.Fields, nil
}

// Raw answers the raw UPDATE body when the filter declared the raw read.
func (r *FilterRoute) Raw() []byte {
	if r.view != nil {
		return r.view.Raw()
	}
	return r.input.Raw
}

// Engine answers the engine's typed view on the fast path, and nil for a
// route that arrived encoded: a filter that asserts it reads the fields from
// Fields when it is nil.
func (r *FilterRoute) Engine() any {
	if r.view != nil {
		return r.view.Engine()
	}
	return nil
}

// Input answers the route as one filter-update input, encoding the fields of
// a fast-path route. The batched transport sends it; a filter written against
// the input type reads it. Its slices are borrowed for the call.
func (r *FilterRoute) Input() (FilterUpdateInput, error) {
	if r.view == nil {
		return *r.input, nil
	}
	input := FilterUpdateInput{
		Filter:    r.input.Filter,
		Direction: r.input.Direction,
		Peer:      r.input.Peer,
		PeerAS:    r.input.PeerAS,
		Raw:       r.view.Raw(),
	}
	fields, err := r.view.Fields()
	if err != nil {
		return FilterUpdateInput{}, err
	}
	if len(fields) > 0 {
		input.Fields = fields
	}
	return input, nil
}

// FilterSlotHandler is the plugin side of the typed filter slot: one route
// in, its verdict and sets out. It is called on the engine's goroutines,
// concurrently, so it MUST honor the fast-path contract
// (docs/plugin-development/handlers.md, "Route filters"): non-blocking,
// bounded, safe for concurrent calls, keeping nothing of route past its
// return, reading its state from a snapshot its own goroutines swap.
type FilterSlotHandler func(route *FilterRoute) (*FilterUpdateOutput, error)

// errFilterSlotUnset answers a call on a bridge whose plugin installed no
// filter slot; the engine checks HasFilterSlot first, so only a defect
// reaches it.
var errFilterSlotUnset = errors.New("BUG: filter slot called on a bridge that holds none")

// SetFilterSlot installs the plugin's filter evaluator. The SDK calls it once,
// after Stage 5 and before SetReady, as it calls SetDeliverStructured. The
// hasFilterSlot store publishes fn to every HasFilterSlot reader.
func (b *DirectBridge) SetFilterSlot(fn FilterSlotHandler) {
	b.filterSlot = fn
	b.hasFilterSlot.Store(fn != nil)
}

// HasFilterSlot reports whether the plugin installed a filter evaluator. It
// stays true after the bridge closes, so the engine never falls back to the
// queued callback once the slot existed: a closed slot answers
// ErrBridgeClosed, which is "unavailable". Safe for concurrent use.
func (b *DirectBridge) HasFilterSlot() bool {
	return b.hasFilterSlot.Load()
}

// CallFilterSlot calls the plugin's evaluator for route, on the caller's
// goroutine, with nothing encoded and nothing queued on the callback channel.
// Before Stage 5 (the bridge not ready), after CloseCallbacks, and after the
// callback loop failed it answers ErrBridgeClosed and calls nothing: the
// plugin is unavailable, and the caller applies the filter's on-error. A
// panic in the evaluator answers an error for this route alone, so a plugin
// defect never ends the engine goroutine that called it. Safe for concurrent
// use.
func (b *DirectBridge) CallFilterSlot(route *FilterRoute) (out *FilterUpdateOutput, err error) {
	if !b.beginDelivery() {
		return nil, ErrBridgeClosed
	}
	defer b.deliveryWG.Done()
	if !b.ready.Load() {
		return nil, ErrBridgeClosed
	}
	if !b.hasFilterSlot.Load() {
		return nil, errFilterSlotUnset
	}
	defer func() {
		if r := recover(); r != nil {
			out, err = nil, fmt.Errorf("filter %s panicked: %v", route.Filter(), r)
		}
	}()
	return b.filterSlot(route)
}
