// Design: docs/architecture/core-design.md -- learned-only final retirement.
// Related: selected_cleanup.go -- engine-owned exact-owner removal authority.
package reactor

import (
	"context"
	"errors"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	"github.com/ze-software/ze/internal/component/plugin"
)

// retireOrdinaryLearned snapshots only recorded current output owners. The
// existing selected-stream and final-writer locks then join old accepted work
// and retire that recorded kind; a concurrently selected local replacement is
// never mistaken for the learned route that used to occupy its native key.
func (p *Peer) retireOrdinaryLearned(ctx context.Context, session *Session) error {
	if err := session.selectedMu.LockContext(ctx); err != nil {
		return err
	}
	p.adjOut.mu.Lock()
	if p.adjOut.session != session {
		p.adjOut.mu.Unlock()
		session.selectedMu.Unlock()
		return nil
	}
	var owners []plugin.SelectedOwner
	for _, entry := range p.adjOut.routes {
		if entry.selection.kind != filterapi.ExportOrdinaryLearned {
			continue
		}
		found := false
		for _, owner := range owners {
			if owner.SameIncarnation(entry.selection.owner) {
				found = true
				break
			}
		}
		if !found {
			owners = append(owners, entry.selection.owner)
		}
	}
	p.adjOut.mu.Unlock()
	session.selectedMu.Unlock()
	var failures []error
	for _, owner := range owners {
		if err := session.retireSelectedOwnerKind(ctx, p, owner, nil, filterapi.ExportOrdinaryLearned); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}
