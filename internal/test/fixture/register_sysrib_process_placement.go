// Design: docs/architecture/testing/interop.md -- actual system RIB process proof.

package fixture

func init() {
	Register("plugin/sysrib-process-placement", sysribProcessPlacementDriver)
}
