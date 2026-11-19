// Design: docs/architecture/l2tp/subscriber-session-model.md -- a silent
// accounting server cannot hold the L2TP control reactor.
// Related: tunnel_fixture_l2tp_slow_accounting.go -- the LAC peer and the
// RADIUS server that withholds the Accounting-Stop, both registered here.

package fixture

func init() {
	Register("l2tp/slow-accounting-control", tunnelL2TPSlowAccountingControl)
	Register("l2tp/slow-accounting-control/radius", tunnelSlowAccountingRadius)
}
