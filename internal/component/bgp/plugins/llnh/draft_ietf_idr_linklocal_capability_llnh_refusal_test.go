package llnh

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	// The schema the configuration under test is parsed against: ze-bgp-conf,
	// the ze-hub-conf it imports, and this plugin's own ze-link-local-nexthop
	// (registered by the package under test, register.go).
	_ "github.com/ze-software/ze/internal/component/bgp/yang"
	_ "github.com/ze-software/ze/internal/component/hub/yang"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// Goal: capability 77 is never advertised for a peer that has no Link-Local
// address to send. Method: each configuration is parsed by the real config
// loader, lowered to the section the plugin receives at Stage 2 and at
// config-verify, and handed to the refusal both handlers run.

// deliverLLNHSection parses text with the real schema and returns the bgp
// section exactly as the engine delivers it to this plugin.
func deliverLLNHSection(t *testing.T, text string) string {
	t.Helper()
	result, err := zeconfig.LoadConfig(text, "test.conf", nil)
	require.NoError(t, err, "the configuration under test must parse")
	subtree := zeconfig.ExtractConfigSubtree(result.Tree.ToPluginMap(), configRootBGP)
	require.NotNil(t, subtree, "ExtractConfigSubtree returned nil, so the plugin would be handed {}")
	data, err := json.Marshal(subtree)
	require.NoError(t, err, "marshal the delivered subtree")
	return string(data)
}

// llnhPeerConfig builds one standalone peer. sessionExtra goes inside the
// session block, capabilityBody inside its capability block.
func llnhPeerConfig(sessionExtra, capabilityBody string) string {
	return "bgp {\n\tpeer peer1 {\n" +
		"\t\tconnection {\n\t\t\tremote { ip 2001:db8:1::2; }\n\t\t\tlocal { ip 2001:db8:1::1; }\n\t\t}\n" +
		"\t\tsession {\n\t\t\tasn { local 65001; remote 65001; }\n" +
		"\t\t\tfamily {\n\t\t\t\tipv6/unicast { }\n\t\t\t}\n" +
		sessionExtra +
		"\t\t\tcapability {\n" + capabilityBody + "\t\t\t}\n" +
		"\t\t}\n\t}\n}\n"
}

// llnhGroupConfig builds one group enabling the capability, with one peer.
// groupExtra and peerExtra go inside the group's and the peer's session block.
func llnhGroupConfig(groupExtra, peerExtra string) string {
	return "bgp {\n\tgroup fabric {\n" +
		"\t\tsession {\n\t\t\tfamily {\n\t\t\t\tipv6/unicast { }\n\t\t\t}\n" +
		groupExtra +
		"\t\t\tcapability {\n\t\t\t\tlink-local-nexthop\n\t\t\t}\n\t\t}\n" +
		"\t\tpeer peer1 {\n" +
		"\t\t\tconnection {\n\t\t\t\tremote { ip 2001:db8:1::2; }\n\t\t\t\tlocal { ip 2001:db8:1::1; }\n\t\t\t}\n" +
		"\t\t\tsession {\n\t\t\t\tasn { local 65001; remote 65001; }\n" + peerExtra + "\t\t\t}\n" +
		"\t\t}\n\t}\n}\n"
}

const (
	llnhCapabilityOn   = "\t\t\t\tlink-local-nexthop\n"
	llnhLinkLocalLeaf  = "\t\t\tlink-local fe80::1\n"
	llnhLinkLocalInner = "\t\t\t\tlink-local fe80::1\n"
)

// TestLinkLocalCapabilityRefusedWithoutLinkLocalAddress is the refusal. A peer
// that enables capability 77, on its own or through its group, with no
// `session > link-local` anywhere, is refused, and the error names the peer and
// the missing leaf. The section-level entry both SDK handlers call refuses too.
//
// RFC requirement: DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-3 negative -- the
// configuration under which a negotiated capability 77 could not carry Ze's own
// Link-Local (capability enabled, no link-local address) is refused at load, for
// a standalone peer and for a peer inheriting the capability from its group.
func TestLinkLocalCapabilityRefusedWithoutLinkLocalAddress(t *testing.T) {
	cases := map[string]string{
		"standalone peer":           llnhPeerConfig("", llnhCapabilityOn),
		"peer inheriting the group": llnhGroupConfig("", ""),
	}
	for name, text := range cases {
		t.Run(name, func(t *testing.T) {
			section := deliverLLNHSection(t, text)

			err := refuseLinkLocalCapabilityWithoutAddress(section)
			require.Error(t, err, "capability 77 without a link-local address must be refused")
			assert.Contains(t, err.Error(), "peer1")
			assert.Contains(t, err.Error(), "session link-local")

			sections := []sdk.ConfigSection{{Root: configRootBGP, Data: section}}
			require.Error(t, refuseLinkLocalCapabilityWithoutAddressSections(sections),
				"the handler entry must refuse the same section")
		})
	}
}

// TestLinkLocalCapabilityAcceptedWithLinkLocalAddress is the other polarity.
// The capability with the leaf on the peer, on the group, or on the group's
// member loads, and so does a peer with no capability.
//
// RFC requirement: DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-4-3 positive -- a peer enabling
// capability 77 with `session > link-local` configured (on the peer or on its
// group) is accepted, so the address the next hop must include is present.
func TestLinkLocalCapabilityAcceptedWithLinkLocalAddress(t *testing.T) {
	cases := map[string]string{
		"leaf on the peer":   llnhPeerConfig(llnhLinkLocalLeaf, llnhCapabilityOn),
		"leaf on the group":  llnhGroupConfig(llnhLinkLocalLeaf, ""),
		"leaf on the member": llnhGroupConfig("", llnhLinkLocalInner),
		"no capability":      llnhPeerConfig("", ""),
	}
	for name, text := range cases {
		t.Run(name, func(t *testing.T) {
			section := deliverLLNHSection(t, text)
			require.NoError(t, refuseLinkLocalCapabilityWithoutAddress(section))
			if strings.Contains(text, "link-local-nexthop") {
				assert.Len(t, extractLLNHCapabilities(section), 1, "the capability is still declared")
			}
		})
	}
}
