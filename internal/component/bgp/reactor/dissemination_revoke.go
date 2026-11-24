// Design: docs/architecture/api/process-protocol.md -- source grant removal barrier.
// Related: peer_settings_apply.go -- publish changed grants before joining writes.
package reactor

import (
	"context"
	"errors"

	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// revokeRemovedSourceGrants joins only the owners whose complete source grant
// this exact peer lost. Callers MUST publish the removed grant first and hold
// no peer or reactor lock. Unrelated owners' retained leases remain usable.
func (p *Peer) revokeRemovedSourceGrants(r *Reactor, before, after *PeerSettings, beforeGraph, afterGraph *pluginserver.DeliveryGraph) error {
	if r == nil {
		return nil
	}
	originErr := p.revokeRemovedOriginGrants(r, before, after, beforeGraph, afterGraph)
	if r.api == nil {
		return originErr
	}
	manager := r.api.ProcessManager()
	if manager == nil {
		return originErr
	}
	old := deliveryPeerFromSettings(before)
	next := deliveryPeerFromSettings(after)
	ctx, cancel := context.WithTimeout(context.Background(), rpc.CancellableRequestTimeout+rpc.RequestCancelTimeout)
	defer cancel()
	candidates := beforeGraph.ProcessListForPeer(p.addrString)
	if len(candidates) == 0 {
		for _, proc := range manager.AllProcesses() {
			candidates = append(candidates, proc.Name())
		}
	}
	for _, procName := range candidates {
		process := manager.GetProcess(procName)
		if process == nil {
			continue
		}
		owner, cached := process.SelectedOwnerForCleanup()
		if !cached {
			continue
		}
		if err := r.api.CheckDisseminationSource(process, old, bgpevents.Namespace, SelectedDisseminationRole); err != nil {
			continue
		}
		if err := r.api.CheckDisseminationSource(process, next, bgpevents.Namespace, SelectedDisseminationRole); err == nil {
			continue
		}
		if err := sourceReceipts.revokePeerOwner(ctx, p, owner); err != nil {
			return errors.Join(originErr, err)
		}
	}
	return originErr
}
