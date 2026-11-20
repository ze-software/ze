// Design: docs/guide/l2tp.md -- a RADIUS Framed-MTU lowers the IP MTU Ze
// installs on the subscriber's pppN.
// Related: tunnel_fixture_l2tp_framed_mtu.go -- tunnelL2TPFramedMTU, the LAC
// peer this registers, and radiusAttrFramedMTU.
// Related: tunnel_fixture_l2tp_radius.go -- tunnelRadiusDriver, the RADIUS mock
// registered here three times with three Access-Accept bodies.

package fixture

func init() {
	Register("l2tp/radius-framed-mtu", tunnelL2TPFramedMTU)
	// One mock function, three Access-Accept bodies: a Framed-MTU below the
	// LCP limit, none, and one above it.
	Register("l2tp/radius-framed-mtu/radius",
		tunnelRadiusDriver(0, 2, tunnelRadiusUint32Attr(radiusAttrFramedMTU, 1400)))
	Register("l2tp/radius-framed-mtu-absent/radius", tunnelRadiusDriver(0, 2, nil))
	Register("l2tp/radius-framed-mtu-exceeds/radius",
		tunnelRadiusDriver(0, 2, tunnelRadiusUint32Attr(radiusAttrFramedMTU, 1600)))
}
