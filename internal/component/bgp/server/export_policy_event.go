// Design: docs/architecture/api/process-protocol.md -- selected policy settlement.
// Related: selected_state.go -- per-recipient captured destination authority.
package server

import (
	"context"
	"fmt"

	"github.com/ze-software/ze/internal/component/bgp/format"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// OnPeerExportPolicy delivers only to the actual selecting process, retaining
// its process identity through event application and deferred-output settlement.
// It does not broadcast a synthetic connection or arm peer-up readiness.
func (d *EventDispatcher) OnPeerExportPolicy(ctx context.Context, peer *plugin.PeerInfo, owner plugin.SelectedOwner) error {
	proc, err := d.policyEventProcess(peer, owner)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	name, _ := owner.Sender().Process() // policyEventProcess validated it.
	projected := selectedStatePeer(peer, name, rpc.SessionStateUp, rpc.ReasonExportPolicy)
	if !projected.SelectedDestination {
		return opererr.New("the policy event has no captured ordinary destination", "Policy reconciliation requires the current ready process and its explicit peer attachment", "ze request reload")
	}
	// This is an already admitted live destination, never an initial replay.
	projected.InitialReplay = 0
	results := make(chan process.EventResult, 1)
	delivery := process.EventDelivery{Result: results}
	if proc.HasStructuredHandler() {
		delivery.Event = getStructuredStateEvent(&projected, rpc.SessionStateUp, rpc.ReasonExportPolicy, nil)
	} else {
		var scratch [512]byte
		delivery.Output = string(format.AppendStateChange(scratch[:0], &projected, rpc.SessionStateUp,
			rpc.ReasonExportPolicy, nil, proc.Encoding()))
	}
	if !proc.DeliverContext(ctx, delivery) {
		if delivery.Event != nil {
			rpc.PutStructuredEvent(delivery.Event.(*rpc.StructuredEvent))
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		return opererr.New("the selecting process refused policy event delivery", "Policy reconciliation requires the current ready process and its explicit peer attachment", "ze request reload")
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case result := <-results:
		if result.Err != nil {
			return fmt.Errorf("selecting process policy event: %w", result.Err)
		}
	}
	if err := proc.SettleEventsApplied(ctx); err != nil {
		return err
	}
	current, err := d.policyEventProcess(peer, owner)
	if err != nil {
		return err
	}
	if current != proc {
		return opererr.New("selecting policy event process changed before settlement completed", "Policy reconciliation requires the current ready process and its explicit peer attachment", "ze request reload")
	}
	return nil
}

// policyEventProcess checks the exact live Process, not only a configured
// name. The caller MUST retain the returned pointer through delivery and
// settlement, then call this check again before acknowledging success.
func (d *EventDispatcher) policyEventProcess(peer *plugin.PeerInfo, owner plugin.SelectedOwner) (*process.Process, error) {
	if err := owner.Current(); err != nil {
		return nil, err
	}
	name, named := owner.Sender().Process()
	if !named {
		return nil, opererr.New("selecting policy event owner is not an authenticated process", "Policy reconciliation requires the current ready process and its explicit peer attachment", "ze request reload")
	}
	if peer.SelectedOwner != name {
		return nil, opererr.New("selecting policy event belongs to another configured owner", "Policy reconciliation requires the current ready process and its explicit peer attachment", "ze request reload")
	}
	if peer.DestinationSession == 0 {
		return nil, opererr.New("selecting policy event has no captured destination session", "Policy reconciliation requires the current ready process and its explicit peer attachment", "ze request reload")
	}
	manager := d.server.ProcessManager()
	if manager == nil {
		return nil, opererr.New("selecting policy event process manager is unavailable", "Policy reconciliation requires the current ready process and its explicit peer attachment", "ze request reload")
	}
	proc := manager.GetProcess(name)
	if proc == nil {
		return nil, opererr.New("selecting policy event process has ended", "Policy reconciliation requires the current ready process and its explicit peer attachment", "ze request reload")
	}
	if !proc.Running() {
		return nil, opererr.New("selecting policy event process is not running", "Policy reconciliation requires the current ready process and its explicit peer attachment", "ze request reload")
	}
	if proc.Stage() < plugin.StageRunning {
		return nil, opererr.New("selecting policy event process has not completed startup", "Policy reconciliation requires the current ready process and its explicit peer attachment", "ze request reload")
	}
	if err := proc.RequireRuntimeRestoration(); err != nil {
		return nil, err
	}
	current, err := proc.SelectedOwner()
	if err != nil {
		return nil, err
	}
	if !current.SameIncarnation(owner) {
		return nil, opererr.New("selecting policy event process incarnation changed", "Policy reconciliation requires the current ready process and its explicit peer attachment", "ze request reload")
	}
	for _, subscribed := range d.server.PeerScopedProcs(bgpNS(), events.LookupEventTypeID(bgpevents.EventState),
		events.DirUnspecified, peer.AddrStr(), peer.Name) {
		if subscribed == proc {
			return proc, nil
		}
	}
	return nil, opererr.New("selecting policy event process has no granted destination state subscription", "Policy reconciliation requires the current ready process and its explicit peer attachment", "ze request reload")
}
