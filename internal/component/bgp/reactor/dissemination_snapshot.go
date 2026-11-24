// Design: docs/architecture/api/architecture.md -- final selected owner fence.
// Related: peer_forward_facts.go -- immutable session-bound publication.
package reactor

import (
	"errors"
	"fmt"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	bgpevents "github.com/ze-software/ze/internal/core/bgp/events"
)

// SelectedDisseminationRole is the opaque declaration contract read by the
// selected-output API and its configuration validator, not an implementation.
const SelectedDisseminationRole = "bgp-selected-output"

// disseminationSnapshot belongs to one exact peer and session publication.
// A final writer compares its pointer with the current forward-facts pointer's
// snapshot; a reload or reconnect cannot authorize work admitted beforehand.
// Its fields and the configured owner are immutable after construction.
type disseminationSnapshot struct {
	peer    *Peer
	session *Session
	owner   pluginserver.DisseminationOwner
	err     error
}

// reloadSettingsSnapshot compares runtime candidates with the same live owner
// authority, rather than the unresolved owner the pure builder started with.
// An actual owner or owner-class change remains a restart-scoped difference.
func (p *Peer) reloadSettingsSnapshot() *PeerSettings {
	settings := p.settingsSnapshot()
	if owner := p.selectedOwnerSnapshot(); owner != nil {
		settings.DisseminationOwner = owner.owner
	}
	return settings
}

func (p *Peer) selectedOwnerSnapshot() *disseminationSnapshot {
	facts := p.forwardFacts()
	if facts == nil {
		return nil
	}
	return facts.dissemination
}

// ExportKind checks authenticated lifetime and this exact publication's owner.
// It allocates no authority, takes no locks and never consults process manager.
func (snapshot *disseminationSnapshot) ExportKind(owner plugin.SelectedOwner, received bool) (filterapi.ExportKind, error) {
	if snapshot == nil {
		return filterapi.ExportUnknown, errors.New("select-routes: destination has no live dissemination snapshot")
	}
	if snapshot.err != nil {
		return filterapi.ExportUnknown, snapshot.err
	}
	if err := owner.Current(); err != nil {
		return filterapi.ExportUnknown, err
	}
	name, named := owner.Sender().Process()
	if !named {
		return filterapi.ExportUnknown, errors.New("select-routes: caller is not a configured process")
	}
	if !snapshot.owner.IsSet() {
		return filterapi.ExportUnknown, errors.New("select-routes: destination has no eligible configured dissemination owner; complete its bus filter grants")
	}
	if snapshot.owner.Name() != name {
		return filterapi.ExportUnknown, fmt.Errorf("select-routes: destination belongs to process %q, not %q; reconcile through its configured owner", snapshot.owner.Name(), name)
	}
	if snapshot.owner.Explicit() || !received {
		return filterapi.ExportExplicit, nil
	}
	return filterapi.ExportOrdinaryLearned, nil
}

// buildDisseminationSnapshot runs only at forwarding lifecycle boundaries,
// outside peer/reactor locks. The exact destination settings snapshot prevents
// a same-address sibling from supplying send or lifecycle authority.
func (p *Peer) buildDisseminationSnapshot(session *Session, settings *PeerSettings) *disseminationSnapshot {
	snapshot := &disseminationSnapshot{peer: p, session: session}
	if p.reactor == nil {
		snapshot.err = errors.New("select-routes: reactor is unavailable")
		return snapshot
	}
	if p.reactor.api == nil {
		snapshot.err = errors.New("select-routes: plugin server is unavailable")
		return snapshot
	}
	peer := deliveryPeerFromSettings(settings)
	snapshot.owner, snapshot.err = p.reactor.api.ResolveDisseminationOwnerForPeer(peer, SelectedDisseminationRole)
	if snapshot.err != nil {
		return snapshot
	}
	previous := p.selectedOwnerSnapshot()
	if previous == nil {
		return snapshot
	}
	if previous.err != nil {
		return snapshot
	}
	if previous.peer != p {
		return snapshot
	}
	if previous.session != session {
		return snapshot
	}
	if previous.owner != snapshot.owner {
		return snapshot
	}
	// Successful resolution has rechecked every required grant. Unrelated
	// next-hop or interface facts do not change this authority publication.
	return previous
}

// resolveDisseminationSettings runs before any reload mutation, so a changed
// authenticated declaration reaches the existing settings restart decision.
func (r *Reactor) resolveDisseminationSettings(peers []*PeerSettings, groups dynamicGroupsLoad) error {
	if r.api == nil {
		return nil
	}
	settings := append([]*PeerSettings(nil), peers...)
	groupSettings := groups.groups
	if !groups.read {
		// A section-only apply leaves these templates installed. Read them
		// under r.mu, then resolve declarations after releasing it: the cold
		// resolver takes process-manager locks.
		r.mu.RLock()
		groupSettings = append([]*DynamicGroupConfig(nil), r.dynamicGroups...)
		r.mu.RUnlock()
	}
	delivery := DisseminationPeersFromSettings(settings)
	for _, group := range groupSettings {
		candidate := *group.Settings
		settings = append(settings, &candidate)
		node := deliveryPeersFromSettings([]*PeerSettings{&candidate})[0]
		// Templates have no remote endpoint yet. Their group name gives each
		// original node a separate identity rather than merging invalid IPs.
		node.Addr = "dynamic-group:" + group.GroupName
		delivery = append(delivery, node)
	}
	owners, err := r.api.ResolveDisseminationOwners(bgpevents.Namespace, SelectedDisseminationRole, delivery)
	if err != nil {
		return err
	}
	for i, peer := range settings {
		peer.DisseminationOwner = owners[delivery[i].Addr]
	}
	if groups.read {
		for i, group := range groupSettings {
			group.Settings.DisseminationOwner = settings[len(peers)+i].DisseminationOwner
		}
	}
	return nil
}
