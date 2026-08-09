// Design: docs/architecture/ospf/ospf-ext-14-debug-introspection.md -- engine-arm helpers for the v6
// database views and the shared debug enablement, keeping register.go's switch thin.

package ospf

import "errors"

// errNoV6Engine is returned when an OSPFv3 command runs with no v6 address family configured.
var errNoV6Engine = errors.New("no OSPFv3 address family is configured")

// debugEnableResult is the typed JSON payload of the shared enable/disable toggle.
type debugEnableResult struct {
	Action  string `json:"action"`
	Enabled bool   `json:"enabled"`
}

// v6DatabaseDetail renders the OSPFv3 database detail for the default IPv6-unicast engine,
// optionally filtered by LS-type name and/or flooding scope. An idle v6 engine yields an
// empty (but AF-tagged) database rather than an error.
func v6DatabaseDetail(set *v6EngineSet, typeFilter, scope string) []any {
	v6eng, ok := set.engineFor(afIPv6Unicast)
	if !ok {
		return []any{v3DetailDatabase{AddressFamily: afIPv6Unicast.String(), LSAs: []v3DetailLSA{}}}
	}
	out, err := v6eng.v3DatabaseDetailSnapshot(typeFilter, scope)
	if err != nil {
		return []any{v3DetailDatabase{AddressFamily: afIPv6Unicast.String(), LSAs: []v3DetailLSA{}}}
	}
	return out
}

// v6DatabaseExtended renders the RFC 8362 extended OSPFv3 LSAs (E-Router / E-Network / ...)
// decoded into named TLVs (AC-7).
func v6DatabaseExtended(set *v6EngineSet) []any {
	return v6DatabaseDetail(set, "extended", "")
}
