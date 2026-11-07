package fixture

// register_ipsec_follows_wan.go names the scenario that proves an IPsec peer bound to
// `vpn ipsec interface` follows the interface's address: it starts when an address
// appears, rehomes when it changes, and stops when it leaves, while a peer with its own
// literal local-address keeps its Child SA throughout.
//
// Related: tunnel_fixture_ipsec_follows_wan.go -- the scenario body
// Related: test/ipsec/ipsec-follows-wan-address.ci -- the run

func init() {
	Register("ipsec/follows-wan-address", tunnelIPsecFollowsWAN)
}
