package gr

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// VALIDATES: the "graceful-restart family" container, over the configuration
// chain the daemon really runs: config text, the tree lowering a plugin
// receives, the Stage-2 refusal, and the octets of the code-64 capability.
// PREVENTS: the container silently doing nothing, the empty container reading
// as the default, and a family the session does not carry reaching the wire as
// a promise Ze cannot keep.
//
// The default state, no container at all, is asserted by
// TestRFC4724GRCapabilityListsTheFamiliesOfTheSession (gr_capability_test.go),
// which reads the same octets for a peer that writes no container.
// deliverBGPSection, grConfig and grPayloadForConfig live in that file too.

// twoFamilies is the session both narrowing tests start from: the peer carries
// ipv4/unicast and ipv6/unicast, so each test's container is a real choice
// between them rather than the only family available.
var twoFamilies = []string{"ipv4/unicast", "ipv6/unicast"}

// TestGRCapabilityFamilyContainerNarrowsTheCapability is the option's purpose.
// The container names one of the two families the session carries, and the
// capability names that one and no other.
//
// RFC 4724 Section 4: "The presence and the setting of the "Forwarding State"
// bit for an address family depend upon the actual forwarding state and
// configuration." The container is that configuration.
func TestGRCapabilityFamilyContainerNarrowsTheCapability(t *testing.T) {
	payload := grPayloadForConfig(t, grConfig(twoFamilies,
		"\t\t\t\t\tfamily {\n\t\t\t\t\t\tname ipv6/unicast\n\t\t\t\t\t}\n"))

	// 0078      Restart Flags 0, Restart Time 120 seconds (the YANG default)
	// 0002 01 00  AFI 2 (IPv6), SAFI 1 (unicast), F bit clear
	assert.Equal(t, "007800020100", payload,
		"the capability names the one family the container asked for")
}

// TestGRCapabilityEmptyFamilyContainerNamesNoFamily is the answer a bare
// leaf-list could not give. The operator wrote the container and named nothing
// in it, which is not the same as writing nothing at all.
//
// RFC 4724 Section 3: "When a sender of this capability does not include any
// <AFI, SAFI> in the capability, it means that the sender is not capable of
// preserving its forwarding state during BGP restart, but supports procedures
// for the Receiving Speaker (as defined in Section 4.2 of this document)."
// That signal stays reachable, and it stays off the default.
func TestGRCapabilityEmptyFamilyContainerNamesNoFamily(t *testing.T) {
	payload := grPayloadForConfig(t, grConfig(twoFamilies, "\t\t\t\t\tfamily {\n\t\t\t\t\t}\n"))

	// 0078 alone: Restart Flags 0, Restart Time 120 seconds, and no tuple.
	assert.Equal(t, "0078", payload,
		"the container is present and empty, so the capability names no address family")
}

// TestGRCapabilityRefusesAFamilyTheSessionDoesNotCarry is the refusal. The
// peer carries ipv4/unicast, the container names ipv6/unicast, and the
// configuration is rejected with the offending family in the message rather
// than dropped quietly.
//
// RFC 4724 Section 3 scopes a tuple to routes "advertised with the same AFI
// and SAFI", so the tuple would promise the peer that Ze retains routes the
// session cannot carry.
//
// The error leaves OnConfigure (RunGRPlugin, gr.go) and fails the Stage 2
// configure RPC. deliverConfigRPC (internal/component/plugin/server/startup.go)
// then calls proc.Stop, and the gr registration's FatalOnConfigError stops the
// daemon rather than running it without Graceful Restart.
func TestGRCapabilityRefusesAFamilyTheSessionDoesNotCarry(t *testing.T) {
	section := deliverBGPSection(t, grConfig([]string{"ipv4/unicast"},
		"\t\t\t\t\tfamily {\n\t\t\t\t\t\tname ipv6/unicast\n\t\t\t\t\t}\n"))

	err := refuseUncarriedGRFamilies(section)

	require.Error(t, err, "a family the peer does not carry is refused, never dropped")
	assert.Contains(t, err.Error(), "ipv6/unicast", "the message names the offending family")
	assert.Contains(t, err.Error(), "ipv4/unicast", "the message names what the peer does carry")
}
