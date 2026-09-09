// Design: docs/architecture/core-design.md -- selection, and the FIB write that follows it
// Related: fib_withhold_fixture.go -- the scenario the first driver below runs
// Related: fib_withhold_controller_fixture.go -- the scenario the second one runs

package fixture

func init() {
	Register("plugin/fib-withhold-one-protocol-keeps-the-other",
		observer02("fib-withhold-test", fibWithholdOneProtocol))
	Register("plugin/fib-withhold-controller-programs-no-route",
		observer02("fib-withhold-controller", fibWithholdController))
}
