// Design: docs/architecture/traffic/fw-7b-backend-hardening.md -- VPP-operation seam for unit tests
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
// Only six operations live here because only six are used:
//   - dumpInterfaces: SwInterfaceDump -> name->swIfIndex map
//   - dumpPolicers:   PolicerDump -> policer names
//   - policerAddDel:  PolicerAddDel upsert, returns VPP-assigned index
//   - policerDel:     PolicerDel(PolicerIndex)
//   - deleteByName:    PolicerAddDel(IsAdd=false, Name) for startup cleanup
//   - policerOutput:  PolicerOutput(Name, SwIfIndex, Apply=true|false)
//
// Extending the interface is cheap: add a method, implement on the
// production adapter, stub on fakeOps. Keeping it narrow makes
// regressions obvious.
//
// The classify* / policerClassify* methods program the VPP classify +
// policer-classify pipeline used by protocol filters (only matching traffic
// is policed). They mirror the proven firewall/vpp classify wrappers, with
// two traffic-specific parameters: skip_n_vectors (the L2-skip the
// policer-classify arc needs) and the session hit-next index (the policer
// index a matching packet is steered to).
type vppOps interface {
	dumpInterfaces() (map[string]interface_types.InterfaceIndex, error)
	dumpPolicers() ([]string, error)
	policerAddDel(req *policer.PolicerAddDel) (uint32, error)
	policerDel(index uint32) error
	policerDeleteByName(name string) error
	policerOutput(name string, swIfIndex interface_types.InterfaceIndex, apply bool) error

	// classifyAddDelTable creates (isAdd=true, tableIdx=^uint32(0)) or removes
	// a classify table with the given mask and skip_n_vectors. nextTableIdx is
	// the table a miss falls through to (^uint32(0) = end of chain); it lets
	// the multi-class steering path chain distinct-mask tables. Returns the
	// VPP-assigned table index on create.
	classifyAddDelTable(tableIdx uint32, mask []byte, skipNVectors, nextTableIdx uint32, isAdd bool) (uint32, error)
	// classifyAddDelSession adds/removes a session in tableIdx. hitNextIndex
	// is the policer index a matching packet is steered to on the
	// policer-classify arc.
	classifyAddDelSession(tableIdx, hitNextIndex uint32, match []byte, isAdd bool) error
	// policerClassifySetInterface binds (isAdd=true) or unbinds the given
	// ip4/ip6 classify tables to an interface's policer-classify feature.
	// A table index of ^uint32(0) means "no table for this family".
	policerClassifySetInterface(swIfIndex interface_types.InterfaceIndex, ip4TableIdx, ip6TableIdx uint32, isAdd bool) error
}
