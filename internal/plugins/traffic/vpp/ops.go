// Design: plan/spec-fw-7b-backend-hardening.md -- VPP-operation seam for unit tests

package trafficvpp

import (
	"go.fd.io/govpp/binapi/interface_types"
	"go.fd.io/govpp/binapi/policer"
)

// vppOps is the narrow VPP-call surface that trafficvpp's Apply path
// will depend on once spec-fw-7b-backend-hardening Phase 2 lands.
// Extracted as an interface so unit tests can substitute a scripted
// fake (`fakeOps`) without a running VPP daemon, and the production
// path will use a `govppOps{ch api.Channel}` adapter that forwards
// each call to the existing sendX helpers in backend_linux.go.
//
// Current state: this interface is declared but has no consumers in
// production code yet. `backend_linux.go` still calls `api.Channel`
// directly. Phase 2 of fw-7b does the refactor. The interface lives
// here as scaffolding so that spec has a concrete starting point.
//
// Only four operations live here because only four are used:
//   - dumpInterfaces: SwInterfaceDump -> name->swIfIndex map
//   - policerAddDel:  PolicerAddDel(IsAdd=true) upsert, returns assigned index
//   - policerDel:     PolicerDel(PolicerIndex)
//   - policerOutput:  PolicerOutput(Name, SwIfIndex, Apply=true|false)
//
// Extending the interface is cheap: add a method, implement on the
// production adapter, stub on fakeOps. Keeping it narrow makes
// regressions obvious.
//
//nolint:unused // scaffolding for spec-fw-7b-backend-hardening Phase 2.
type vppOps interface {
	dumpInterfaces() (map[string]interface_types.InterfaceIndex, error)
	policerAddDel(req *policer.PolicerAddDel) (uint32, error)
	policerDel(index uint32) error
	policerOutput(name string, swIfIndex interface_types.InterfaceIndex, apply bool) error
}
