// Design: docs/architecture/plugin/rib-storage-design.md -- bounded selected batches.
// Related: ../../../../../pkg/plugin/rpc/selected.go -- one frame-size authority.
package rib

import (
	"errors"
	"fmt"
	"iter"
	"math"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// selectedItemBudget is a measured request and widest-answer array member.
// Its zero value is unmeasured; production values come from immutable capture
// construction before publication, never from a destination's hot loop.
type selectedItemBudget struct {
	request  int
	response int
}

func selectedItemSize(route rpc.SelectedRoute) (selectedItemBudget, error) {
	request, response, err := rpc.SelectRoutesItemSizes(route)
	return selectedItemBudget{request: request, response: response}, err
}

func newSelectedScopedRoute(route rpc.SelectedRoute) (*selectedScopedRoute, error) {
	value := &selectedScopedRoute{wire: route}
	// The published revision can advance on abort without changing payload.
	// Both request and response therefore reserve its widest representation.
	route.Revision = math.MaxUint64
	var err error
	value.steady, err = selectedItemSize(route)
	if err != nil {
		return nil, err
	}
	initial := *route.Route
	initial.InitialUpdate = true
	route.Route = &initial
	value.initial, err = selectedItemSize(route)
	if err != nil {
		return nil, err
	}
	value.absence, err = selectedItemSize(rpc.SelectedRoute{Kind: rpc.SelectedRouteAbsent,
		Family: route.Family, KeyNLRI: route.KeyNLRI, Revision: math.MaxUint64})
	if err != nil {
		return nil, err
	}
	return value, nil
}

func selectedEnvelopeSize(input rpc.SelectRoutesInput) (selectedItemBudget, error) {
	input.Routes = []rpc.SelectedRoute{}
	request, response, err := rpc.SelectRoutesExchangeSizes(input)
	return selectedItemBudget{request: request, response: response}, err
}

// selectedDispatchBatch streams only the keys it visits. Immutable production
// items carry cached sizes, so choosing a batch marshals no route under mu.
// The returned arrays own exactly their chosen population; an iterator and its
// unchosen payloads are never retained by a blocked final writer.
func selectedDispatchBatch(batch rpc.SelectRoutesInput, pending iter.Seq[selectedDispatchItem]) (rpc.SelectRoutesInput, []selectedDispatchItem, error) {
	base, err := selectedEnvelopeSize(batch)
	if err != nil {
		return rpc.SelectRoutesInput{}, nil, err
	}
	return selectedDispatchSizedBatch(batch, base, pending)
}

func selectedDispatchSizedBatch(batch rpc.SelectRoutesInput, size selectedItemBudget, pending iter.Seq[selectedDispatchItem]) (rpc.SelectRoutesInput, []selectedDispatchItem, error) {
	base := size
	var chosen []selectedDispatchItem
	var refused error
	for item := range pending {
		budget := item.budget
		if budget.request == 0 {
			// Direct packer callers provide raw values. The publication path
			// supplies measured immutable budgets before taking d.mu.
			var err error
			budget, err = selectedItemSize(item.route)
			if err != nil {
				refused = errors.Join(refused, err)
				continue
			}
			item.budget = budget
		}
		if max(base.request+budget.request, base.response+budget.response) > rpc.MaxMessageSize {
			refused = errors.Join(refused, fmt.Errorf("selected %s route exceeds the IPC frame limit; reduce the route's attributes before retrying", item.route.Family))
			continue
		}
		comma := 0
		if len(chosen) > 0 {
			comma = 1
		}
		request := size.request + budget.request + comma
		response := size.response + budget.response + comma
		if max(request, response) > rpc.MaxMessageSize {
			break
		}
		size = selectedItemBudget{request: request, response: response}
		chosen = append(chosen, item)
	}
	items := make([]selectedDispatchItem, len(chosen))
	copy(items, chosen)
	batch.Routes = make([]rpc.SelectedRoute, len(items))
	for i := range items {
		batch.Routes[i] = items[i].route
	}
	return batch, items, refused
}
