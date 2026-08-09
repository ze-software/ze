// Design: docs/architecture/ospf/ospf-ext-14-debug-introspection.md -- `show ospf [ipv6] interface detail`.
// RFC: rfc/short/rfc2328.md (Section 9 ISM state + DR/BDR election), rfc/short/rfc5340.md
// (Section 3.4.3 OSPFv3 Interface ID; RFC 6549 Instance ID).
//
// The interface deep-dump reads each interface's Detail snapshot (ISM state, DR/BDR, all
// three timers, and for OSPFv3 the local Interface ID + Instance ID) and, for OSPFv2, adds
// the opaque-capable neighbor count from the neighbor table (RFC 5250). Read-only; additive
// over the summary `... interface` view.

package ospf

import (
	ospfiface "github.com/ze-software/ze/internal/plugins/ospf/iface"
	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// interfaceDetailView is one interface's full state plus derived counts / AF identity.
type interfaceDetailView struct {
	ospfiface.Detail
	OpaqueCapableNeighbors int    `json:"opaque-capable-neighbors"`
	AddressFamily          string `json:"address-family,omitempty"`
}

// interfaceDetailSnapshot renders `show ospf interface detail` (OSPFv2).
func (e *engine) interfaceDetailSnapshot() []any {
	return e.interfaceDetailRows(false)
}

// v3InterfaceDetailSnapshot renders `show ospf ipv6 interface detail` (OSPFv3: local
// Interface ID + Instance ID, AF-tagged).
func (e *engine) v3InterfaceDetailSnapshot() []any {
	return e.interfaceDetailRows(true)
}

func (e *engine) interfaceDetailRows(v6 bool) []any {
	e.mu.Lock()
	ifaces := make([]*ospfiface.Interface, 0, len(e.interfaces))
	for _, ic := range e.interfaces {
		ifaces = append(ifaces, ic)
	}
	e.mu.Unlock()

	capable := map[string]int{}
	if e.neighbors != nil {
		details := e.neighbors.DetailSnapshot()
		for i := range details {
			if types.Options(details[i].Options).Has(types.OptionO) {
				capable[details[i].Interface]++
			}
		}
	}
	out := make([]any, 0, len(ifaces))
	for _, ic := range ifaces {
		d := ic.DetailSnapshot()
		view := interfaceDetailView{Detail: d, OpaqueCapableNeighbors: capable[d.Name]}
		if v6 {
			view.AddressFamily = e.af.String()
		}
		out = append(out, view)
	}
	return out
}
