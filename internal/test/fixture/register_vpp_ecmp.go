// Design: docs/architecture/fib/fib-vpp-route-ownership.md -- VPP FIB ECMP functional fixture registration
// Related: vpp_fixture_ecmp.go -- driver and observer scenario for vpp/vpp-fib-ecmp
// Related: misc_fixture_vpp.go -- shared VPP stub, ze launcher and stub log helpers

package fixture

func init() {
	Register("vpp/vpp-fib-ecmp", vppECMPDriver)
	Register("vpp/vpp-fib-ecmp-observer", vppECMPObserverDriver, StopsDaemon)
}
