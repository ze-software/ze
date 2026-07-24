// Design: ai/rules/feature-gate-registration.md -- ze_vpp partition of the dataplane registry

//go:build ze_vpp

package dataplane

// The GoVPP-backed IPsec dataplane (vpp.go) registers only in ze_vpp builds;
// without the tag the backend name is unknown and selecting `dataplane vpp`
// fails closed at Load with "not registered". Register's only error is a
// duplicate name (dataplane.go Register) -- two init()s claiming "vpp" is a
// programmer error, so it panics like the registry convention rather than
// limping on with a half-registered backend.
func init() {
	if err := Register("vpp", newVPPBackend); err != nil {
		panic("BUG: dataplane vpp backend already registered")
	}
}
