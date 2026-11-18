// Design: docs/guide/l2tp.md -- original bound-listener selection

package l2tp

// Caller holds Subsystem.mu. Both loops are bounded by this subsystem's active
// and retained original roots; the map prevents reporting one root twice.
func (s *Subsystem) listenerSnapshotsLocked() []ListenerSnapshot {
	seen := make(map[*l2tpReactor]bool, len(s.reactors)+len(s.retiredReactors))
	var snapshots []ListenerSnapshot
	appendOriginal := func(reactor *l2tpReactor) {
		if reactor == nil {
			return
		}
		if seen[reactor] {
			return
		}
		seen[reactor] = true
		reactor.tunnelsMu.Lock()
		defer reactor.tunnelsMu.Unlock()
		if reactor.listener == nil {
			return
		}
		snapshots = append(snapshots, ListenerSnapshot{Name: reactor.params.ServerName, Addr: reactor.listener.Addr()})
	}
	for _, reactor := range s.reactors {
		appendOriginal(reactor)
	}
	for _, reactor := range s.retiredReactors {
		appendOriginal(reactor)
	}
	return snapshots
}
