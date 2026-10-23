// Design: docs/functional-tests.md -- explicit final-policy consumer proofs.
package fixture

func init() {
	Register("plugin/policy-final-disposition", policyFinalDriver)
	Register("reload/bgp-policy-final-reload", policyFinalReloadDriver)
}
