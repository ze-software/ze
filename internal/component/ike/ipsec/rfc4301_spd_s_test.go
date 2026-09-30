// Design: docs/architecture/ike/ipsec-3-data-model.md -- IPsec data model
// Related: config.go -- parseSiteToSitePeer and parseESPGroup, validate.go -- ValidateGroupRefs, the producers
// RFC: rfc/short/rfc4301.md -- SPD management (Section 4.4.1), the SPD-S entry
package ipsec

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/ike/dataplane"
)

// rfc4301SPDSTree builds one SPD-S entry as the administrator writes it: an esp-group
// "strong" (proposal 1, aes256 with sha256) and a site-to-site peer "branch" that names
// it, with the peer leaves given and one traffic selector local -> remote. An empty
// local or remote prefix leaves that side unset.
func rfc4301SPDSTree(peerLeaves map[string]string, local, remote string) *config.Tree {
	tree := config.NewTree()
	ipsec := tree.GetOrCreateContainer("vpn").GetOrCreateContainer("ipsec")

	proposal := config.NewTree()
	proposal.Set("encryption", "aes256")
	proposal.Set("hash", "sha256")
	group := config.NewTree()
	group.AddListEntry("proposal", "1", proposal)
	ipsec.AddListEntry("esp-group", "strong", group)

	peer := config.NewTree()
	for leaf, value := range peerLeaves {
		peer.Set(leaf, value)
	}
	ts := config.NewTree()
	ts.GetOrCreateContainer("local").Set("prefix", local)
	ts.GetOrCreateContainer("remote").Set("prefix", remote)
	peer.AddListEntry("traffic-selector", "1", ts)
	ipsec.GetOrCreateContainer("site-to-site").AddListEntry("peer", "branch", peer)
	return tree
}

// rfc4301SPDSPeerLeaves is a complete SPD-S entry's peer half.
func rfc4301SPDSPeerLeaves() map[string]string {
	return map[string]string{
		"remote-address":  "198.51.100.1",
		"connection-type": "respond",
		"mode":            "transport",
		"esp-group":       "strong",
	}
}

// TestRFC4301SPDSEntryCarriesSelectorsSAControlsAndProtection proves the administrator
// can write the SPD-S entry RFC 4301 Section 4.4.1 describes: the selectors of the traffic
// to protect, the controls on how SAs are created for it, and the parameters that effect
// the protection. In Ze that entry is a site-to-site peer. Method: parse one peer through
// ParseIPsecConfig and the engine's cross-reference check (ValidateGroupRefs), then read
// back each of the three parts as written.
func TestRFC4301SPDSEntryCarriesSelectorsSAControlsAndProtection(t *testing.T) {
	// RFC requirement: RFC4301-4.4.1-6 positive -- an SPD-S entry (a site-to-site peer) is written with its selectors (local and remote prefix of the traffic selector), its SA creation controls (connection-type respond, remote address, the esp-group it negotiates from) and its protection parameters (mode transport, proposal aes256 with sha256), and each reaches the parsed model as written.
	cfg, err := ParseIPsecConfig(rfc4301SPDSTree(rfc4301SPDSPeerLeaves(), "10.0.0.0/24", "10.1.0.0/24"))
	if err != nil {
		t.Fatalf("ParseIPsecConfig: %v", err)
	}
	if err := cfg.ValidateGroupRefs(); err != nil {
		t.Fatalf("ValidateGroupRefs: %v", err)
	}
	peer, ok := cfg.Peers["branch"]
	if !ok {
		t.Fatalf("no peer named branch was parsed; got %v", cfg.Peers)
	}

	if len(peer.TrafficSelectors) != 1 {
		t.Fatalf("selectors = %d, want the 1 written", len(peer.TrafficSelectors))
	}
	selector := peer.TrafficSelectors[0]
	if selector.LocalPrefix == nil || selector.LocalPrefix.String() != "10.0.0.0/24" {
		t.Errorf("local selector = %v, want 10.0.0.0/24", selector.LocalPrefix)
	}
	if selector.RemotePrefix == nil || selector.RemotePrefix.String() != "10.1.0.0/24" {
		t.Errorf("remote selector = %v, want 10.1.0.0/24", selector.RemotePrefix)
	}

	if peer.ConnectionType != ConnectionRespond {
		t.Errorf("connection type = %v, want respond", peer.ConnectionType)
	}
	if peer.RemoteAddress != "198.51.100.1" {
		t.Errorf("remote address = %q, want 198.51.100.1", peer.RemoteAddress)
	}

	if peer.Mode != dataplane.ModeTransport {
		t.Errorf("mode = %d, want transport (%d)", peer.Mode, dataplane.ModeTransport)
	}
	group, ok := cfg.ESPGroups[peer.ESPGroup]
	if !ok {
		t.Fatalf("the peer's esp-group %q resolves to no group", peer.ESPGroup)
	}
	if len(group.Proposals) != 1 {
		t.Fatalf("proposals = %d, want the 1 written", len(group.Proposals))
	}
	if group.Proposals[0].Encryption != EncryptionAES256 || group.Proposals[0].Hash != HashSHA256 {
		t.Errorf("proposal = %v/%v, want aes256/sha256", group.Proposals[0].Encryption, group.Proposals[0].Hash)
	}
}

// TestRFC4301SPDSEntryRefusesAMissingOrUnusablePart proves an SPD-S entry whose
// protection parameters, SA controls or selectors cannot be honored is refused rather
// than installed with a part missing or replaced. Method: start from the complete entry
// and break one part per case, then require the parse or the cross-reference check to
// refuse it naming the part.
func TestRFC4301SPDSEntryRefusesAMissingOrUnusablePart(t *testing.T) {
	// RFC requirement: RFC4301-4.4.1-6 negative -- an SPD-S entry naming an esp-group that is not defined, a mode Ze cannot install, an unknown connection type, or a malformed selector prefix is refused, naming the part.
	for _, tc := range []struct {
		name   string
		leaf   string
		value  string
		local  string
		remote string
		want   string
	}{
		{"undefined esp-group", "esp-group", "absent", "10.0.0.0/24", "10.1.0.0/24", `esp-group "absent" not defined`},
		{"unknown mode", "mode", "beet", "10.0.0.0/24", "10.1.0.0/24", "mode: unsupported value"},
		{"unknown connection type", "connection-type", "sometimes", "10.0.0.0/24", "10.1.0.0/24", "connection-type: unsupported value"},
		{"malformed selector", "", "", "10.0.0.0/33", "10.1.0.0/24", "10.0.0.0/33"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			leaves := rfc4301SPDSPeerLeaves()
			if tc.leaf != "" {
				leaves[tc.leaf] = tc.value
			}
			cfg, err := ParseIPsecConfig(rfc4301SPDSTree(leaves, tc.local, tc.remote))
			if err == nil {
				err = cfg.ValidateGroupRefs()
			}
			if err == nil {
				t.Fatalf("the entry was accepted, want a refusal containing %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("refusal = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}
