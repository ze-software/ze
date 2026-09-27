package plugin

import (
	"strings"
	"testing"
)

// VALIDATES: BGP reads the contents of a delivered bgp section, which the
// server wraps in its root key, through ONE parser: the config verify and the
// operation decomposer both call parseBGPSection.
// PREVENTS: the wrapped section reaching PeersFromTree, which then finds no
// `peer` key and answers no peers, so reactor.VerifyPeerBFDProfiles passed
// every candidate and a commit that deleted a profile a peer names was
// accepted. Also prevents a non-empty object that lacks the bgp key being
// read as an empty tree, which would reach the peer reconcile as "no peers"
// and remove every peer.
func TestParseBGPSection(t *testing.T) {
	tree, err := parseBGPSection(`{"bgp":{"peer":{"peer1":{}}}}`)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := tree["peer"]; !ok {
		t.Fatalf("unwrapped tree %v has no peer key", tree)
	}

	tree, err = parseBGPSection(`{}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(tree) != 0 {
		t.Fatalf("a deleted root answered %v, want an empty tree", tree)
	}

	refused := []struct {
		name string
		data string
		want string
	}{
		{"bgp value not an object", `{"bgp":"text"}`, "not an object"},
		{"object without the bgp key", `{"peer":{"peer1":{}}}`, "has no bgp key"},
		{"json null", `null`, "not an object"},
		{"empty string", ``, "unmarshal"},
	}
	for _, tc := range refused {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseBGPSection(tc.data)
			if err == nil {
				t.Fatalf("%q answered tree %v, want a refusal", tc.data, got)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("%q refused with %q, want it to contain %q", tc.data, err, tc.want)
			}
		})
	}
}
