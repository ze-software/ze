// Design: docs/architecture/core-design.md -- route-kind final reconciliation.
// Related: policy_reload.go -- local restoration and ordinary selected reoffer.
package reactor

import (
	"context"
	"errors"
	"fmt"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/opererr"
)

// exportFinalKinds records dispositions, or changed kinds, using the same
// route authority and absent-policy decision as the final output writer.
type exportFinalKinds struct {
	local   bool
	learned bool
}

func exportFinalDisposition(policy filterapi.Policy, external, ordinary bool) exportFinalKinds {
	kind := filterapi.ExportExplicit
	if ordinary {
		kind = filterapi.ExportOrdinaryLearned
	}
	return exportFinalKinds{local: policy.AcceptsExportAtEnd(external, filterapi.ExportExplicit),
		learned: policy.AcceptsExportAtEnd(external, kind)}
}

func (before exportFinalKinds) restores(after exportFinalKinds) bool {
	return !before.local && after.local || !before.learned && after.learned
}

func (before exportFinalKinds) changed(after exportFinalKinds) exportFinalKinds {
	return exportFinalKinds{local: before.local != after.local, learned: before.learned != after.learned}
}

// ordinarySelectedExport reads the already authenticated, exact destination
// authority. Plugin implementation names and optional replay stores decide none.
func (p *Peer) ordinarySelectedExport() bool {
	snapshot := p.selectedOwnerSnapshot()
	return snapshot != nil && snapshot.err == nil && snapshot.owner.IsSet() && !snapshot.owner.Explicit()
}

type peerExportPolicyEvents interface {
	OnPeerExportPolicy(context.Context, *plugin.PeerInfo, plugin.SelectedOwner, bool) error
}

// reconcileSelectedExport asks the existing owner to reoffer its current selections.
// An absent-policy change that keeps local output eligible MUST reoffer learned
// routes only. Settlement joins final output and earlier admitted work.
func (p *Peer) reconcileSelectedExport(ctx context.Context, session *Session, includeLocal bool) error {
	snapshot := p.selectedOwnerSnapshot()
	if snapshot == nil || snapshot.err != nil || !snapshot.owner.IsSet() || snapshot.owner.Explicit() {
		return errors.New("export final reconciliation lost its ordinary selecting owner")
	}
	if snapshot.session != session || p.currentSession() != session {
		return errRecoveryDestinationClosed
	}
	dispatcher, ok := p.reactor.bindEvents.(peerExportPolicyEvents)
	if !ok {
		return errors.New("export final reconciliation needs the selecting owner's policy event dispatcher")
	}
	peer := establishedPeerInfo(p)
	if peer.DestinationSession == 0 || peer.SelectedOwner != snapshot.owner.Name() {
		return errors.New("export final reconciliation lost its captured destination authority")
	}
	manager := p.reactor.api.ProcessManager()
	if manager == nil {
		return opererr.New("export final reconciliation has no selecting process manager", "The policy reoffer requires its exact current selecting process", "ze request reload")
	}
	proc := manager.GetProcess(snapshot.owner.Name())
	if proc == nil {
		return opererr.New("export final reconciliation selecting process has ended", "The policy reoffer requires its exact current selecting process", "ze request reload")
	}
	owner, err := proc.SelectedOwner()
	if err != nil {
		return err
	}
	kind, err := snapshot.ExportKind(owner, true)
	if err != nil {
		return err
	}
	if kind != filterapi.ExportOrdinaryLearned {
		return opererr.New("export final reconciliation lost its ordinary learned output authority", "The policy reoffer requires its exact current selecting process", "ze request reload")
	}
	if p.currentSession() != session || p.selectedOwnerSnapshot() != snapshot {
		return errRecoveryDestinationClosed
	}
	if err := dispatcher.OnPeerExportPolicy(ctx, &peer, owner, includeLocal); err != nil {
		return fmt.Errorf("export final selected reconciliation: %w", err)
	}
	if p.currentSession() != session || p.selectedOwnerSnapshot() != snapshot {
		return errRecoveryDestinationClosed
	}
	return nil
}
