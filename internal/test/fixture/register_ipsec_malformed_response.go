package fixture

// register_ipsec_malformed_response.go names the IKEv2 test peer that negotiates a real
// IKE SA with a Ze responder and then sends, under that SA's keys, one INFORMATIONAL
// response whose inner payload chain does not parse.
//
// Related: tunnel_fixture_ipsec_malformed_response.go -- the peer
// Related: test/ipsec/ike-peer-malformed-doctor.ci -- the run

func init() {
	Register("ipsec/malformed-response-peer", tunnelIPsecMalformedResponsePeer)
}
