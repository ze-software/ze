// Design: docs/architecture/testing/interop.md -- one bespoke owner for the watchdog scenario

package bgp

const scenarioWatchdogWithdrawValueFRR = "watchdog-withdraw-value-frr"

func init() {
	specialCheckers[scenarioWatchdogWithdrawValueFRR] = checkWatchdogWithdrawValueFRR
}
