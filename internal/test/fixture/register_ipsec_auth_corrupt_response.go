package fixture

// register_ipsec_auth_corrupt_response.go names the UDP relay that sits between two Ze
// IKE daemons and, before it forwards each first IKE_AUTH response to the initiator,
// sends three copies whose last octet it changed; in withhold mode it replaces every
// IKE_AUTH response with one damaged copy.
//
// Related: tunnel_fixture_ipsec_corrupt_response.go -- the relay
// Related: test/ipsec/ike-auth-corrupt-response.ci -- the run
// Related: test/ipsec/ike-auth-unauthenticated-doctor.ci -- the withhold run

func init() {
	Register("ipsec/auth-corrupt-response", tunnelIPsecAuthCorruptResponse)
}
