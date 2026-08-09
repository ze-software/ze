// Design: docs/architecture/ospf/ospf-13-cli-diag-interop.md -- `clear ospf ...` runtime resets.
// These reset runtime state without reconfiguring: neighbors re-form from the next Hello
// and SPF re-runs. The wire methods (ze-clear:ospf-*) are registered in cmd_show.go and
// the command tree in yang/ze-ospf-cmd.yang; OnExecuteCommand (register.go) dispatches here.

package ospf

// clearResult is the status payload a `clear ospf <action>` returns: the action and
// the number of objects reset (neighbors torn down; 0 for a counter reset).
type clearResult struct {
	Action  string `json:"action"`
	Cleared int    `json:"cleared"`
}

// clearNeighbors tears down every adjacency (clear ospf neighbor); each re-forms from
// the next Hello. Returns the number reset.
func (e *engine) clearNeighbors() int {
	if e.neighbors == nil {
		return 0
	}
	return e.neighbors.ResetAll()
}

// clearCounters resets the SPF run history shown by `show ospf spf` (clear ospf
// counters). Monotonic Prometheus series are not reset.
func (e *engine) clearCounters() {
	if e.spf != nil {
		e.spf.ClearSPFLog()
	}
}

// clearProcess is a full reset (clear ospf process): every adjacency is torn down and
// SPF is re-run across all areas. Returns the number of neighbors reset.
func (e *engine) clearProcess() int {
	n := e.clearNeighbors()
	if e.spf != nil {
		e.spf.Trigger()
	}
	return n
}
