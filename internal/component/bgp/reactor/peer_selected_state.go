// Design: docs/architecture/api/process-protocol.md -- exact selected destinations.
// Related: dissemination_snapshot.go -- cached, fully resolved output owner.
package reactor

// selectedStateDestination reads the already published exact-session owner.
// No process-manager lookup or configuration resolution runs during delivery.
// An absent/invalid snapshot grants no selected output; the configuration gate
// refuses invalid declarations before a peer starts. Explicit roles are served
// by their own owner, rather than the ordinary advertiser.
func (p *Peer) selectedStateDestination() (session uint64, ordinaryOwner string) {
	current := p.currentSession()
	if current == nil {
		return 0, ""
	}
	session = current.initialReplay.Load()
	snapshot := p.selectedOwnerSnapshot()
	if snapshot == nil {
		return session, ""
	}
	if snapshot.err != nil {
		return session, ""
	}
	if snapshot.peer != p {
		return session, ""
	}
	if snapshot.session != current {
		return session, ""
	}
	if !snapshot.owner.IsSet() {
		return session, ""
	}
	if snapshot.owner.Explicit() {
		return session, ""
	}
	return session, snapshot.owner.Name()
}
