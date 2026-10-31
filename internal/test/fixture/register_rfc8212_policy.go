// Design: docs/functional-tests.md -- RFC8212 Phase 1 real-entry fixtures.
package fixture

func init() {
	Register("plugin/rfc8212-ebgp-no-import-policy", rfc8212PolicyDriver(rfc8212DirectionImport))
	Register("plugin/rfc8212-ebgp-no-export-policy", rfc8212PolicyDriver(directionExport))
	Register("plugin/rfc8212-ibgp-no-policy-unchanged", rfc8212PolicyDriver("ibgp"))
	Register("plugin/rfc8212-policy-status", rfc8212PolicyStatus)
	Register("plugin/rfc8212-policy-bridge", rfc8212PolicyBridge)
}
