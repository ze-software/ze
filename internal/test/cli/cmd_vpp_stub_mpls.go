// Design: docs/functional-tests.md -- the VPP stub's MPLS table 0 readback
// Related: cmd_vpp_stub.go -- the stub whose mpls_route_add_del and mpls_route_dump arms call this
//
// The VPP stub answers mpls_route_dump from the label routes the client
// installed with mpls_route_add_del, so a functional test can prove that
// `show mpls forwarding` on the vpp data plane reads back what fib-vpp wrote.
// Only MPLS table 0 is kept: it is the only label table fib-vpp writes.

package cli

import (
	"encoding/binary"
	"io"
	"slices"
	"sync"
)

// vppStubLabelKey names one VPP label route: an in-label at one S bit.
type vppStubLabelKey struct {
	label uint32
	eos   byte
}

// vppStubLabelTable holds the encoded mpls_route of every label route the
// client installed, in install order. Safe for concurrent use: GoVPP clients
// on several connections share one stub.
type vppStubLabelTable struct {
	mu     sync.Mutex
	keys   []vppStubLabelKey
	routes map[vppStubLabelKey][]byte
}

// record applies one mpls_route_add_del body: mr_is_add (u8), mr_is_multipath
// (u8), then the mpls_route, whose first fields are mr_table_id (u32),
// mr_label (u32) and mr_eos (u8). The route is the last field, so body[2:]
// is exactly the mpls_route a mpls_route_details reply carries. A body too
// short to hold the key is ignored, as decodeMPLSRoute ignores it.
func (t *vppStubLabelTable) record(body []byte) {
	if len(body) < 11 {
		return
	}
	key := vppStubLabelKey{label: binary.BigEndian.Uint32(body[6:10]), eos: body[10]}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.routes == nil {
		t.routes = make(map[vppStubLabelKey][]byte)
	}
	if body[0] == 0 {
		delete(t.routes, key)
		t.keys = slices.DeleteFunc(t.keys, func(k vppStubLabelKey) bool { return k == key })
		return
	}
	if _, present := t.routes[key]; !present {
		t.keys = append(t.keys, key)
	}
	t.routes[key] = slices.Clone(body[2:])
}

// snapshot copies the installed routes, in install order.
func (t *vppStubLabelTable) snapshot() [][]byte {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([][]byte, 0, len(t.keys))
	for _, key := range t.keys {
		out = append(out, slices.Clone(t.routes[key]))
	}
	return out
}

// serveMPLSRouteDump answers one mpls_route_dump with a mpls_route_details
// frame per installed route. The client follows the dump with a
// control_ping, whose reply the ordinary handler sends and which ends the
// dump on the client side. Every MPLS table dump is answered from table 0.
func (state *vppStubState) serveMPLSRouteDump(writer io.Writer, labels *vppStubLabelTable, contextID uint32) error {
	routes := labels.snapshot()
	if err := state.writeLog("mpls_route_dump", contextID, map[string]any{"routes": len(routes)}); err != nil {
		return err
	}
	for _, route := range routes {
		frame, err := state.reply("mpls_route_details", contextID, route)
		if err != nil {
			return err
		}
		if err := writeVPPFrame(writer, frame); err != nil {
			return err
		}
	}
	return nil
}
