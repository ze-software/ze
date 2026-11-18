// Design: docs/architecture/plugin/plugin-system.md -- authenticated replay fencing.
// Related: registry.go -- FencesLiveForwards reads the installed process seam.
package registry

// RegisteredReplayFence reads only immutable compiled metadata. The server
// MUST use it only for a process that runs a built-in implementation, never to
// lend a same-named external program that implementation's declaration.
func RegisteredReplayFence(name string) bool {
	mu.RLock()
	defer mu.RUnlock()
	reg := plugins[name]
	if reg == nil {
		return false
	}
	if !reg.SignalsSessionReady {
		return false
	}
	return reg.FencesLiveForwards
}

// replayFenceLookup is one owned predicate publication, not retained process
// declarations. Pointer identity prevents an old release removing its successor.
type replayFenceLookup struct {
	query func(process string) bool
}

// SetRuntimeReplayFence installs one server's process lookup. It MUST answer
// false for an ended process. The caller MUST run the returned release when
// that server stops; release MUST leave a newer server's lookup in place.
// Passing nil withdraws the current lookup. With none installed, static
// registry callers keep their compiled answer. Safe after Seal.
// It is a runtime seam, callable after Seal.
func SetRuntimeReplayFence(query func(process string) bool) func() {
	var publication *replayFenceLookup
	if query != nil {
		publication = &replayFenceLookup{query: query}
	}
	mu.Lock()
	runtimeReplayFence = publication
	mu.Unlock()
	return func() {
		mu.Lock()
		defer mu.Unlock()
		if runtimeReplayFence == publication {
			runtimeReplayFence = nil
		}
	}
}
