// Design: docs/architecture/api/process-protocol.md -- source grant removal barrier.
// Related: peer_settings_apply.go -- publish changed grants before joining writes.
package reactor

import (
	"context"
	"errors"

	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// revokeRemovedSourceGrants joins only the owners whose complete source grant
// this exact peer lost. Callers MUST publish the removed grant first and hold
// no peer or reactor lock. Unrelated owners' retained leases remain usable.
func (p *Peer) revokeRemovedSourceGrants(r *Reactor, before, after []ProcessBinding) error {
	if r == nil {
		return nil
	}
	originErr := p.revokeRemovedOriginGrants(r, before, after)
	if r.api == nil {
		return originErr
	}
	manager := r.api.ProcessManager()
	if manager == nil {
		return originErr
	}
	old := deliveryPeersFromSettings([]*PeerSettings{{Address: p.settings.Address, Name: p.settings.Name, ProcessBindings: before}})[0]
	next := deliveryPeersFromSettings([]*PeerSettings{{Address: p.settings.Address, Name: p.settings.Name, ProcessBindings: after}})[0]
	ctx, cancel := context.WithTimeout(context.Background(), rpc.CancellableRequestTimeout+rpc.RequestCancelTimeout)
	defer cancel()
	for _, binding := range before {
		process := manager.GetProcess(binding.PluginName)
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
