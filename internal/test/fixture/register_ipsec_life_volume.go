// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- Child SA volume lifetimes on Linux XFRM.
// Related: tunnel_fixture_ipsec_life_volume.go -- the scenario body.
// Related: test/ipsec/ipsec-life-volume.ci -- the real-XFRM functional test.

package fixture

func init() {
	Register("ipsec/life-volume", tunnelIPsecLifeVolume)
}
