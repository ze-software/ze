package gr

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	// The schema the configuration under test is parsed against: ze-bgp-conf,
	// the ze-hub-conf it imports, and this plugin's own ze-graceful-restart
	// (registered by the package under test, register.go).
	_ "github.com/ze-software/ze/internal/component/bgp/yang"
	_ "github.com/ze-software/ze/internal/component/hub/yang"

	zeconfig "github.com/ze-software/ze/internal/component/config"
)

// VALIDATES: the octets of the RFC 4724 code-64 capability Ze declares for a
// peer, read out of the production config-extraction path.
// PREVENTS: Ze advertising Graceful Restart with no <AFI, SAFI> tuple, which
// tells the peer it preserves forwarding state for nothing and leaves it no
// reason to retain Ze's routes across a restart. It also prevents the opposite
// error, a tuple for a family the session does not carry or a Forwarding State
// bit Ze cannot support.

// deliverBGPSection builds the config section the GR plugin actually receives,
// by driving the real producer chain rather than typing the JSON:
//
//	LoadConfig              (internal/component/config)
//	  -> (*Tree).ToPluginMap  (cmd/ze/hub/main_reload.go, loadTreeForReload)
//	  -> ExtractConfigSubtree (internal/component/config/plugin_verify.go)
//	  -> json.Marshal         (deliverConfigRPC, plugin/server/startup.go)
//
// The SHAPE is the point, and a hand-typed section cannot state it. A fixture
// and the reader it feeds are written by one author on one day, so they agree
// with each other while both disagree with the model, and no assertion in
// between can tell. This node hid three such disagreements at once: a peer key
// is a NAME and the config refuses an address as one, a YANG list arrives as a
// MAP keyed by the list key rather than an array of names, and "family" sits
// under "session" beside "capability" rather than at the top of the peer.
func deliverBGPSection(t *testing.T, text string) string {
	t.Helper()
	result, err := zeconfig.LoadConfig(text, "test.conf", nil)
	require.NoError(t, err, "the configuration under test must parse")
	subtree := zeconfig.ExtractConfigSubtree(result.Tree.ToPluginMap(), configRootBGP)
	require.NotNil(t, subtree, "ExtractConfigSubtree returned nil, so the plugin would be handed {}")
	data, err := json.Marshal(subtree)
	require.NoError(t, err, "marshal the delivered subtree")
	t.Logf("delivered section for %s: %s", configRootBGP, data)
	return string(data)
}

// grConfig builds the configuration of one peer: the address families its
// session carries, and the body of its graceful-restart container.
func grConfig(sessionFamilies []string, gracefulRestartBody string) string {
	var text strings.Builder
	text.WriteString("bgp {\n\tpeer peer1 {\n\t\tsession {\n\t\t\tfamily {\n")
	for _, name := range sessionFamilies {
		text.WriteString("\t\t\t\t" + name + " { }\n")
	}
	text.WriteString("\t\t\t}\n\t\t\tcapability {\n\t\t\t\tgraceful-restart {\n")
	text.WriteString(gracefulRestartBody)
	text.WriteString("\t\t\t\t}\n\t\t\t}\n\t\t}\n\t}\n}\n")
	return text.String()
}

// grPayloadForConfig returns the hex payload of the single code-64
// declaration the plugin makes for the single peer in text. The test reads the
// octets Ze puts in the OPEN, because the reactor takes this payload verbatim:
// Peer.getPluginCapabilities hands it to capability.NewPlugin
// (internal/component/bgp/reactor/peer.go).
func grPayloadForConfig(t *testing.T, text string) string {
	t.Helper()
	section := deliverBGPSection(t, text)
	require.NoError(t, refuseUncarriedGRFamilies(section), "the configuration under test is accepted")
	caps := extractGRCapabilities(section)
	require.Len(t, caps, 1, "one peer configures graceful-restart, so one declaration is owed")
	require.Equal(t, uint8(grCapCode), caps[0].Code, "the declaration is the Graceful Restart capability")
	return caps[0].Payload
}

// TestRFC4724GRCapabilityListsTheFamiliesOfTheSession reads the whole code-64
// payload Ze declares for a peer that carries two address families, and
// compares it octet for octet with the RFC 4724 Section 3 encoding.
//
// The method is the production path, from the configuration text: the tree
// lowering a plugin receives, then extractGRCapabilities. Not a typed section,
// and not parseGRCapValue on its own.
//
// RFC 4724 Section 3: "The AFI and SAFI, taken in combination, indicate that
// Graceful Restart is supported for routes that are advertised with the same
// AFI and SAFI."
//
// RFC requirement: RFC4724-4-4 positive -- parseGRCapValue
// (internal/component/bgp/plugins/gr/gr_capability.go) appends one <AFI, SAFI,
// Flags> tuple for each address family the peer configures, after the Restart
// Flags and Restart Time pair. The test asserts the exact payload
// "012c" + "00010100" + "00020100" for a peer with restart-time 300 and the
// families ipv4/unicast and ipv6/unicast.
func TestRFC4724GRCapabilityListsTheFamiliesOfTheSession(t *testing.T) {
	payload := grPayloadForConfig(t,
		grConfig([]string{"ipv4/unicast", "ipv6/unicast"}, "\t\t\t\t\trestart-time 300\n"))

	// 012c      Restart Flags 0, Restart Time 300 seconds
	// 0001 01 00  AFI 1 (IPv4), SAFI 1 (unicast), F bit clear
	// 0002 01 00  AFI 2 (IPv6), SAFI 1 (unicast), F bit clear
	assert.Equal(t, "012c0001010000020100", payload,
		"the capability lists both families of the session")
}

// TestRFC4724GRCapabilityClaimsNoFamilyItDoesNotCarry reads the same payload
// for a peer configured with one family, and asserts the two things Ze must
// not say: a tuple for a family the session does not carry, and a Forwarding
// State bit for a restart that preserved nothing.
//
// RFC 4724 Section 4.1: "Unless allowed via configuration, the "Forwarding
// State" bit for an address family in the capability can be set only if the
// forwarding state has indeed been preserved for that address family during
// the restart."
//
// RFC requirement: RFC4724-4-4 negative -- with only ipv6/unicast configured,
// parseGRCapValue (internal/component/bgp/plugins/gr/gr_capability.go) emits
// the ipv6 tuple alone, and the Flags octet of that tuple is 0x00 rather than
// 0x80. The test asserts the exact payload "0078" + "00020100", so an ipv4
// tuple Ze does not carry and a Forwarding State bit Ze cannot support both
// fail it.
func TestRFC4724GRCapabilityClaimsNoFamilyItDoesNotCarry(t *testing.T) {
	payload := grPayloadForConfig(t, grConfig([]string{"ipv6/unicast"}, ""))

	// 0078      Restart Flags 0, Restart Time 120 seconds (the YANG default)
	// 0002 01 00  AFI 2 (IPv6), SAFI 1 (unicast), F bit clear
	assert.Equal(t, "007800020100", payload,
		"one configured family, one tuple, and no claim of preserved forwarding state")
}
