// Design: docs/architecture/plugin/rib-storage-design.md -- current selected output status.
// Related: rib_selected_failure.go -- bounded failures point to this full live projection.
package rib

import "net/netip"

// selectedOutputStatus is a neutral projection, not a health finding or another
// completion ledger. Its lifetime is one command answer; it retains no routes.
type selectedOutputStatus struct {
	Destination    netip.Addr `json:"destination"`
	Session        uint64     `json:"session"`
	Pending        int        `json:"pending"`
	InFlight       int        `json:"in-flight"`
	Ready          bool       `json:"ready"`
	Failure        string     `json:"failure,omitempty"`
	RefreshPending int        `json:"refresh-pending,omitempty"`
	RefreshActive  bool       `json:"refresh-active,omitempty"`
}

// unsettledStatus snapshots every currently owed destination under the same
// mutex and predicate as the barrier. Safe for concurrent use. The live admitted
// destination population bounds its loop; no ended-Session history is retained.
func (d *selectedDispatch) unsettledStatus() []selectedOutputStatus {
	d.mu.Lock()
	defer d.mu.Unlock()
	status := make([]selectedOutputStatus, 0, len(d.destinations))
	for _, destination := range d.destinations {
		if d.settledLocked(destination) {
			continue
		}
		entry := selectedOutputStatus{Destination: destination.peer, Session: destination.session,
			Pending: len(destination.pending), InFlight: len(destination.inFlight), Ready: destination.ready}
		entry.RefreshPending = len(destination.refreshOrder)
		entry.RefreshActive = destination.refreshActive != nil
		if destination.failure != nil {
			entry.Failure = destination.failure.Error()
		}
		status = append(status, entry)
	}
	return status
}
