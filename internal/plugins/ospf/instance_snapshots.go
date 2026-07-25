// Design: plan/learned/967-ospf-13-cli-diag-interop.md -- engine `show ospf ...` snapshots.
// Related: instance.go -- the engine that owns the neighbor table, LSDB, and SPF computer.
// Related: register.go -- OnExecuteCommand renders these snapshots for the show commands.
package ospf

import "github.com/ze-software/ze/internal/plugins/ospf/types"

func (e *engine) neighborSnapshot() []any {
	if e.neighbors == nil {
		return nil
	}
	snap := e.neighbors.Snapshot()
	out := make([]any, 0, len(snap))
	for i := range snap {
		// Annotate the BFD session state (AC-15). RouterID is a lossless dotted-quad round-trip.
		if id, err := types.ParseRouterID(snap[i].RouterID); err == nil {
			if st, ok := e.bfdSessionState(bfdClientKey{iface: snap[i].Interface, router: id}); ok {
				snap[i].BFD = st
			}
		}
		out = append(out, snap[i])
	}
	return out
}

func (e *engine) interfaceSnapshot() []any {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := make([]any, 0, len(e.interfaces))
	for _, ifc := range e.interfaces {
		out = append(out, ifc.Snapshot())
	}
	return out
}

func (e *engine) databaseSnapshot() []any {
	if e.lsdb == nil {
		return nil
	}
	return []any{e.lsdb.Snapshot()}
}

func (e *engine) routeSnapshot() []any {
	if e.spf == nil {
		return nil
	}
	rows := e.spf.Snapshot()
	out := make([]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, row)
	}
	return out
}

// fastRerouteSnapshot renders `show ospf route fast-reroute`: each prefix's
// primary next-hops with their RFC 5286 / TI-LFA backups (spec-ospf-ext-6).
func (e *engine) fastRerouteSnapshot() []any {
	if e.spf == nil {
		return nil
	}
	rows := e.spf.FastRerouteSnapshot()
	out := make([]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, row)
	}
	return out
}

func (e *engine) borderRouterSnapshot() []any {
	if e.spf == nil {
		return nil
	}
	rows := e.spf.BorderRouterSnapshot()
	out := make([]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, row)
	}
	return out
}

func (e *engine) spfSnapshot() []any {
	if e.spf == nil {
		return nil
	}
	rows := e.spf.SPFSnapshot()
	out := make([]any, 0, len(rows))
	for _, row := range rows {
		out = append(out, row)
	}
	return out
}
