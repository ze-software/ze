// Design: plan/spec-fw-7b-backend-hardening.md -- VPP-operation seam for unit tests
// Related: backend_linux.go -- govppOps production adapter + Apply/applyWithOps consumers

//go:build linux

package trafficvpp

import (
	"go.fd.io/govpp/binapi/interface_types"
	"go.fd.io/govpp/binapi/policer"
)

// vppOps is the narrow VPP-call surface that trafficvpp's Apply path
// depends on. Extracted as an interface so unit tests can substitute a
// scripted fake (`fakeOps`) without a running VPP daemon. The production
// path uses the `govppOps` adapter in `backend_linux.go`.
//
// Only four operations live here because only four are used:
//   - dumpInterfaces: SwInterfaceDump -> name->swIfIndex map
//   - policerAddDel:  PolicerAddDel upsert, returns VPP-assigned index
//   - policerDel:     PolicerDel(PolicerIndex)
//   - policerOutput:  PolicerOutput(Name, SwIfIndex, Apply=true|false)
//
// Extending the interface is cheap: add a method, implement on the
// production adapter, stub on fakeOps. Keeping it narrow makes
// regressions obvious.
type vppOps interface {
	dumpInterfaces() (map[string]interface_types.InterfaceIndex, error)
	policerAddDel(req *policer.PolicerAddDel) (uint32, error)
	policerDel(index uint32) error
	policerOutput(name string, swIfIndex interface_types.InterfaceIndex, apply bool) error
}
