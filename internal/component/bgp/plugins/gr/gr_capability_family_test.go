package gr

import (
	"fmt"
	"strings"
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

// grGroupConfig builds one group holding one peer, with a session family list
// and a graceful-restart body at each level. An empty body writes no container
// at that level, which is how a test asks for the other level to govern.
func grGroupConfig(groupFamilies, peerFamilies []string, groupGR, peerGR string) string {
	var text strings.Builder
	text.WriteString("bgp {\n\tgroup group1 {\n\t\tsession {\n")
	text.WriteString(grSessionBody(groupFamilies, groupGR, "\t\t\t"))
	text.WriteString("\t\t}\n\t\tpeer peer1 {\n\t\t\tsession {\n")
	text.WriteString(grSessionBody(peerFamilies, peerGR, "\t\t\t\t"))
	text.WriteString("\t\t\t}\n\t\t}\n\t}\n}\n")
	return text.String()
}

// grSessionBody writes the family list and the graceful-restart container of
// one session block, indented under indent.
func grSessionBody(families []string, gracefulRestartBody, indent string) string {
	var text strings.Builder
	if len(families) > 0 {
		fmt.Fprintf(&text, "%sfamily {\n", indent)
		for _, name := range families {
			fmt.Fprintf(&text, "%s\t%s { }\n", indent, name)
		}
		fmt.Fprintf(&text, "%s}\n", indent)
	}
	if gracefulRestartBody != "" {
		fmt.Fprintf(&text, "%scapability {\n%s\tgraceful-restart {\n", indent, indent)
		text.WriteString(gracefulRestartBody)
		fmt.Fprintf(&text, "%s\t}\n%s}\n", indent, indent)
	}
	return text.String()
}

// TestGRCapabilityCarriesTheFamiliesInheritedFromTheGroup is the merge the
// reactor performs and this plugin must match. deepMergeAt
// (internal/component/bgp/config/resolve.go) merges a group's session family
// list into its peer's key by key, so the session carries BOTH lists and
// parseFamiliesFromTree negotiates both.
//
// Reading the peer's list alone named one of the two families in the
// capability, which is the defect 15e8b877a fixed for the peer level and left
// standing for the group level.
func TestGRCapabilityCarriesTheFamiliesInheritedFromTheGroup(t *testing.T) {
	payload := grPayloadForConfig(t, grGroupConfig(
		[]string{"ipv6/unicast"}, []string{"ipv4/unicast"},
		"", "\t\t\t\t\trestart-time 120\n"))

	// 0078      Restart Flags 0, Restart Time 120 seconds
	// 0001 01 00  AFI 1 (IPv4), SAFI 1 (unicast), from the peer
	// 0002 01 00  AFI 2 (IPv6), SAFI 1 (unicast), from the group
	assert.Equal(t, "0078 00010100 00020100", spaced(payload),
		"the capability names the group's family beside the peer's own")
}

// TestGRCapabilityAcceptsAFamilyTheGroupContributes is the refusal's other
// half. Naming an inherited family is correct configuration, and refusing it
// stops ze, because the gr registration sets FatalOnConfigError.
func TestGRCapabilityAcceptsAFamilyTheGroupContributes(t *testing.T) {
	section := deliverBGPSection(t, grGroupConfig(
		[]string{"ipv6/unicast"}, []string{"ipv4/unicast"},
		"", "\t\t\t\t\tfamily {\n\t\t\t\t\t\tname ipv6/unicast\n\t\t\t\t\t}\n"))

	require.NoError(t, refuseUncarriedGRFamilies(section),
		"the peer carries ipv6/unicast through its group, so naming it is correct")
}

// TestGRCapabilityOmitsADisabledFamily reads the mode leaf of a session family
// entry. ze-bgp-conf.yang: "disable does not advertise it", and
// parseFamiliesFromTree skips such an entry, so the session never negotiates
// that family and a tuple for it would promise routes that cannot exist.
func TestGRCapabilityOmitsADisabledFamily(t *testing.T) {
	text := "bgp {\n\tpeer peer1 {\n\t\tsession {\n\t\t\tfamily {\n" +
		"\t\t\t\tipv4/unicast { }\n" +
		"\t\t\t\tipv6/unicast { mode disable; }\n" +
		"\t\t\t}\n\t\t\tcapability {\n\t\t\t\tgraceful-restart {\n" +
		"\t\t\t\t}\n\t\t\t}\n\t\t}\n\t}\n}\n"

	payload := grPayloadForConfig(t, text)

	assert.Equal(t, "007800010100", payload,
		"the disabled family gets no tuple, because the session does not carry it")
}

// TestGRCapabilityRefusesADisabledFamily is the same fact at the guard. The
// operator named a family their own configuration switched off, and the two
// halves cannot both be what they meant.
func TestGRCapabilityRefusesADisabledFamily(t *testing.T) {
	text := "bgp {\n\tpeer peer1 {\n\t\tsession {\n\t\t\tfamily {\n" +
		"\t\t\t\tipv4/unicast { }\n" +
		"\t\t\t\tipv6/unicast { mode disable; }\n" +
		"\t\t\t}\n\t\t\tcapability {\n\t\t\t\tgraceful-restart {\n" +
		"\t\t\t\t\tfamily {\n\t\t\t\t\t\tname ipv6/unicast\n\t\t\t\t\t}\n" +
		"\t\t\t\t}\n\t\t\t}\n\t\t}\n\t}\n}\n"

	err := refuseUncarriedGRFamilies(deliverBGPSection(t, text))

	require.Error(t, err, "a family the peer disabled is not one it carries")
	assert.Contains(t, err.Error(), "ipv6/unicast", "the message names the offending family")
}

// TestGRCapabilityGroupContainerIsNotRefusedForAnOverridingPeer pins the
// precedence the guard and the builder now share. The peer writes its own
// graceful-restart container, so the group's never reaches this peer's OPEN
// (grCapabilityFor, gr_capability.go). Refusing the group's list on that
// peer's behalf stopped ze over a capability it would not have sent.
func TestGRCapabilityGroupContainerIsNotRefusedForAnOverridingPeer(t *testing.T) {
	text := grGroupConfig(
		nil, []string{"ipv4/unicast"},
		"\t\t\t\tfamily {\n\t\t\t\t\tname ipv6/unicast\n\t\t\t\t}\n",
		"\t\t\t\t\t\trestart-time 300\n")

	section := deliverBGPSection(t, text)

	require.NoError(t, refuseUncarriedGRFamilies(section),
		"the peer's own container governs, so the group's family list is not this peer's")
	assert.Equal(t, "012c00010100", grPayloadForConfig(t, text),
		"and the peer advertises its own container")
}

// spaced groups a hex payload into the fields of RFC 4724 Section 3: the
// Restart Flags and Restart Time pair, then one tuple per address family.
func spaced(payload string) string {
	if len(payload) <= 4 {
		return payload
	}
	var text strings.Builder
	text.WriteString(payload[:4])
	for i := 4; i < len(payload); i += 8 {
		text.WriteString(" ")
		text.WriteString(payload[i:min(i+8, len(payload))])
	}
	return text.String()
}
