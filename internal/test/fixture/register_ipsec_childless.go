package fixture

// register_ipsec_childless.go names the scenario that proves an esp-group reload
// lets a childless IKE SA create its Child SA without a reconnect.
//
// Related: tunnel_fixture_ipsec_childless.go -- the scenario body
// Related: test/ipsec/ipsec-childless-ike-sa-recovers.ci -- the run

func init() {
	Register("ipsec/childless-ike-sa-recovers", tunnelIPsecChildlessRecovers)
}
