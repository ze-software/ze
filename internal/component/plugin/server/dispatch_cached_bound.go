// Design: docs/architecture/api/process-protocol.md -- same authenticated cached-write evaluator.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"net/netip"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

func cachedWriteOwner(ctx context.Context, proc *process.Process, owner, session uint64, deadline int64) (plugin.Sender, plugin.SelectedOwner, error) {
	if err := rpc.ValidateCachedWrite(owner, session, deadline); err != nil {
		return plugin.Sender{}, plugin.SelectedOwner{}, err
	}
	if err := ctx.Err(); err != nil {
		return plugin.Sender{}, plugin.SelectedOwner{}, err
	}
	if proc == nil {
		return plugin.Sender{}, plugin.SelectedOwner{}, errors.New("bound cached write has no authenticated process")
	}
	captured, err := proc.SelectedOwner()
	if err != nil {
		return plugin.Sender{}, plugin.SelectedOwner{}, err
	}
	sender, err := captured.Sender().WithDestination(owner, session)
	return sender, captured, err
}
func (s *Server) forwardCachedRequest(ctx context.Context, proc *process.Process, input rpc.ForwardCachedInput) (*rpc.CachedWriteOutput, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}
	if !input.Bound() {
		return nil, s.forwardCached(proc, input.IDs, input.Destinations)
	}
	if proc == nil {
		return nil, errors.New("bound forward-cached has no authenticated process; use its original connection")
	}
	owner, err := proc.SelectedOwner()
	if err != nil {
		return nil, err
	}
	bound, ok := s.reactor.(plugin.ReactorBoundForwardCoordinator)
	if !ok {
		return nil, errors.New("forward-cached reactor lacks bound completion; rebuild the BGP reactor")
	}
	limit, stop := context.WithTimeout(ctx, rpc.CancellableRequestTimeout)
	defer stop()
	work, cancel := context.WithDeadline(limit, time.UnixMilli(input.DeadlineMS))
	defer cancel()
	result := bound.ForwardUpdatesBound(work, input.IDs, input.Targets, proc.Name(), owner.Sender(), owner)
	result = errors.Join(result, owner.Current(), work.Err())
	return rpc.CompletedCachedWrite(input.DeadlineMS, result), nil
}
func (s *Server) relayStoredRouteRequest(ctx context.Context, proc *process.Process, input rpc.RelayStoredRouteInput) (*rpc.CachedWriteOutput, error) {
	if err := input.Validate(); err != nil {
		return nil, err
	}
	if input.DestinationOwner == 0 {
		return nil, s.relayStoredRouteForProcess(proc, input.Destination, input.Routes)
	}
	sender, owner, err := cachedWriteOwner(ctx, proc, input.DestinationOwner, input.DestinationSession, input.DeadlineMS)
	if err != nil {
		return nil, err
	}
	bound, ok := s.reactor.(plugin.ReactorBoundForwardCoordinator)
	if !ok {
		return nil, errors.New("relay-stored-route reactor lacks bound completion; rebuild the BGP reactor")
	}
	address, err := netip.ParseAddr(input.Destination)
	if err != nil {
		return nil, err
	}
	limit, stop := context.WithTimeout(ctx, rpc.CancellableRequestTimeout)
	defer stop()
	work, cancel := context.WithDeadline(limit, time.UnixMilli(input.DeadlineMS))
	defer cancel()
	result := bound.RelayStoredRouteBound(work, address, input.Routes, sender, owner)
	result = errors.Join(result, owner.Current(), work.Err())
	return rpc.CompletedCachedWrite(input.DeadlineMS, result), nil
}
func cachedWriteCancellation(proc *process.Process, owner, session uint64, deadline int64, method string) (requestScope, error) {
	if err := rpc.ValidateCachedWrite(owner, session, deadline); err != nil {
		return requestScope{}, err
	}
	if owner == 0 {
		return requestScope{}, nil
	}
	if proc == nil {
		return requestScope{}, errors.New("bound cached cancellation requires its authenticated process")
	}
	if _, err := proc.SelectedOwner(); err != nil {
		return requestScope{}, err
	}
	return rpc.NewRequestCancellationScope(method, time.UnixMilli(deadline)), nil
}
func forwardCachedCancellationScope(_ *Server, proc *process.Process, raw json.RawMessage) (requestScope, error) {
	var input rpc.ForwardCachedInput
	if err := json.Unmarshal(raw, &input); err != nil {
		return requestScope{}, err
	}
	if err := input.Validate(); err != nil {
		return requestScope{}, err
	}
	if !input.Bound() {
		return requestScope{}, nil
	}
	if proc == nil {
		return requestScope{}, errors.New("bound cached cancellation requires its authenticated process")
	}
	if _, err := proc.SelectedOwner(); err != nil {
		return requestScope{}, err
	}
	return rpc.NewRequestCancellationScope(rpc.MethodForwardCached, time.UnixMilli(input.DeadlineMS)), nil
}
func relayStoredRouteCancellationScope(_ *Server, proc *process.Process, raw json.RawMessage) (requestScope, error) {
	var input rpc.RelayStoredRouteInput
	if err := json.Unmarshal(raw, &input); err != nil {
		return requestScope{}, err
	}
	if err := input.Validate(); err != nil {
		return requestScope{}, err
	}
	return cachedWriteCancellation(proc, input.DestinationOwner, input.DestinationSession, input.DeadlineMS, rpc.MethodRelayStoredRoute)
}
