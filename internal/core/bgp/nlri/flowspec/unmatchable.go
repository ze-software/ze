// Design: docs/architecture/wire/nlri-flowspec.md — FlowSpec component combinations
// RFC: rfc/short/rfc8955.md
// Overview: types.go — core FlowSpec types, constants, and interface
// Related: vocabulary.go — KeywordComponentType, the keyword each type is written with

package flowspec

import "errors"

// ErrFlowSpecUnmatchable is the refusal of a flow route no packet can match.
var ErrFlowSpecUnmatchable = errors.New("flowspec: a flow route matching an ICMP type or " +
	"code and a port matches no packet, because ICMP has no ports (RFC 8955 Section 4.2); " +
	"remove the port match or the ICMP match")

// unmatchableICMP and unmatchablePort are the two sides of the combination RFC
// 8955 Section 4.2 names: a component from each side in one route matches no
// packet, since the ICMP components match only protocol 1 (Sections 4.2.2.7 and
// 4.2.2.8) and the port components only TCP or UDP (Section 4.2.2.4).
var (
	unmatchableICMP = [...]FlowComponentType{FlowICMPType, FlowICMPCode}
	unmatchablePort = [...]FlowComponentType{FlowPort, FlowDestPort, FlowSourcePort}
)

// CheckMatchable answers ErrFlowSpecUnmatchable when the component types that
// has reports present cannot match any packet, and nil when a packet can.
// It is the one statement of the rule: the FlowSpec methods ask it about their
// components, and the ExaBGP migration asks it about the keywords of a route
// before any FlowSpec exists.
//
// RFC 8955 Section 4.2: "some combinations cannot match any packets (e.g.,
// "ICMP Type AND Port" will never match any packets) and thus SHOULD NOT be
// propagated by BGP."
func CheckMatchable(has func(FlowComponentType) bool) error {
	if !anyPresent(unmatchableICMP[:], has) {
		return nil
	}
	if !anyPresent(unmatchablePort[:], has) {
		return nil
	}
	return ErrFlowSpecUnmatchable
}

// anyPresent reports whether has answers true for one of types.
func anyPresent(types []FlowComponentType, has func(FlowComponentType) bool) bool {
	for _, t := range types {
		if has(t) {
			return true
		}
	}
	return false
}

// CheckMatchable answers ErrFlowSpecUnmatchable when no packet can match this
// FlowSpec, and nil when one can.
//
// RFC 8955 Section 4.2.
func (f *FlowSpec) CheckMatchable() error {
	return CheckMatchable(func(t FlowComponentType) bool { return f.componentOfType(t) != nil })
}

// CheckMatchable answers ErrFlowSpecUnmatchable when no packet can match this
// FlowSpec VPN route, and nil when one can.
//
// RFC 8955 Section 4.2.
func (f *FlowSpecVPN) CheckMatchable() error {
	return f.flowSpec.CheckMatchable()
}
