package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// unionPeerTree is the tree TestValidateTree_UnionViolation validates, with
// the peer's remote ip, a union of ipv4-address, ipv6-address and the enum
// "dynamic", set to remoteIP.
func unionPeerTree(remoteIP string) map[string]any {
	return map[string]any{
		"router-id": "192.0.2.1",
		"session":   map[string]any{"asn": map[string]any{"local": uint32(65001)}},
		"peer": map[string]any{
			"peer1": map[string]any{
				"connection": map[string]any{
					"remote": map[string]any{"ip": remoteIP},
					"local":  map[string]any{"ip": "192.0.2.1"},
				},
				"session": map[string]any{"asn": map[string]any{"remote": uint32(65002)}},
			},
		},
	}
}

// TestRFC7950UnionAcceptsEveryMemberType validates the union leaf with a
// value that matches only its second member, and one that matches only its
// third, so a union check that tries the first member alone goes red.
//
// RFC requirement: RFC7950-9.12-1 positive — a remote ip of "2001:db8::1", which only the ipv6-address member matches, and of "dynamic", which only the enum member matches, each validate with no error.
func TestRFC7950UnionAcceptsEveryMemberType(t *testing.T) {
	v := newTestValidator(t)
	for _, remoteIP := range []string{"2001:db8::1", "dynamic"} {
		assert.Empty(t, v.ValidateTree("bgp", unionPeerTree(remoteIP)), "%q matches a member type of the union", remoteIP)
	}
}
