// Design: docs/architecture/core-design.md — Plugin filters join the chain
// Related: external.go — judgeExternal, which hands this view to the caller

package policychain

import (
	"slices"

	"github.com/ze-software/ze/internal/component/bgp/policyview"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// memberView is the rpc.FilterView an external member's call carries: the
// chain's view of the route, read in place by a plugin running internal
// (D-12), or encoded once into the run's fields buffer when the filter reads
// the JSON object or the route travels in a filter-update batch. It lives in
// the RunSets, so a call allocates no view, and it is valid only during the
// member's call: Engine's view and the slices Fields and Raw answer borrow
// the received UPDATE.
type memberView struct {
	run     *RunSets
	view    *policyview.View
	reads   []string
	encoded bool
	fields  []byte
	err     error
}

var _ rpc.FilterView = (*memberView)(nil)

// begin points the view at one member call.
func (v *memberView) begin(run *RunSets, view *policyview.View, reads []string) {
	*v = memberView{run: run, view: view, reads: reads}
}

// end drops the call's borrowed view.
func (v *memberView) end() {
	*v = memberView{}
}

// Fields encodes the declared fields on its first call of the member's call
// and answers the same bytes after; nil when the filter declared none.
func (v *memberView) Fields() ([]byte, error) {
	if v.encoded {
		return v.fields, v.err
	}
	v.encoded = true
	fields, err := policyview.AppendFields(v.run.fields[:0], v.view, v.reads)
	if err != nil {
		v.err = err
		return nil, err
	}
	v.run.fields = fields[:0]
	if len(fields) > 0 {
		v.fields = fields
	}
	return v.fields, nil
}

// Raw answers the raw UPDATE body when the filter declared the raw read.
func (v *memberView) Raw() []byte {
	if slices.Contains(v.reads, rpc.FilterFieldRaw) {
		return v.run.route.Raw
	}
	return nil
}

// Engine answers the chain's *policyview.View, overlays applied, so a filter
// compiled with the engine reads each field from the wire bytes on first read.
func (v *memberView) Engine() any { return v.view }
