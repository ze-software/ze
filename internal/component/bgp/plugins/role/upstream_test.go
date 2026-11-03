package role

import (
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// upstreamTestBGP is a bgp subtree as Tree.ToPluginMap delivers it: a peer
// whose local role is customer (its remote AS 64500 is an upstream), a group
// whose role is customer with the remote ASN on the group (64501), and a
// customer session (local role provider) naming an asn-set of providers this
// router does not peer with.
func upstreamTestBGP() map[string]any {
	return map[string]any{
		"peer": map[string]any{
			"transit-a": map[string]any{
				"role":    map[string]any{"import": "customer"},
				"session": map[string]any{"asn": map[string]any{"remote": "64500"}},
			},
			"cust": map[string]any{
				"role":    map[string]any{"import": "provider", "transit-asns": "far"},
				"session": map[string]any{"asn": map[string]any{"remote": "64600"}},
			},
		},
		"group": map[string]any{
			"uplinks": map[string]any{
				"role":    map[string]any{"import": "customer"},
				"session": map[string]any{"asn": map[string]any{"remote": "64501"}},
				"peer": map[string]any{
					"transit-b": map[string]any{},
				},
			},
		},
	}
}

// TestUpstreamSetDerivedFromRoles proves the set is the remote ASN of every
// peer whose role import is customer (RFC 9234 Section 3.1: the remote AS is
// then a transit provider), group inheritance included, unioned with the
// asn-set a transit-asns leaf names, and that a leaf naming no set refuses.
func TestUpstreamSetDerivedFromRoles(t *testing.T) {
	policyRoot := map[string]any{"asn-set": map[string]any{"far": map[string]any{"asn": []any{"64700", "64701"}}}}

	set, err := deriveUpstreamSet(upstreamTestBGP(), policyRoot)
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	want := upstreamSet{64500, 64501, 64700, 64701}
	if !slices.Equal(set, want) {
		t.Fatalf("set = %v, want %v", set, want)
	}

	_, err = deriveUpstreamSet(upstreamTestBGP(), nil)
	if err == nil {
		t.Fatal("a transit-asns naming no asn-set was accepted")
	}
	if !strings.Contains(err.Error(), `asn-set "far"`) || !strings.Contains(err.Error(), "fix:") {
		t.Fatalf("refusal %q names neither the set nor the fix", err)
	}
}

// TestUpstreamSetReadsEveryASNotation proves the set reads a remote ASN and
// an asn-set member in the RFC 5396 asdot form the config accepts, as
// asn.Parse reads it everywhere else: 1.10 is 65546, 0.64700 is 64700.
func TestUpstreamSetReadsEveryASNotation(t *testing.T) {
	bgp := map[string]any{
		"peer": map[string]any{
			"transit-a": map[string]any{
				"role":    map[string]any{"import": "customer"},
				"session": map[string]any{"asn": map[string]any{"remote": "1.10"}},
			},
			"cust": map[string]any{
				"role":    map[string]any{"import": "provider", "transit-asns": "far"},
				"session": map[string]any{"asn": map[string]any{"remote": "64600"}},
			},
		},
	}
	policyRoot := map[string]any{"asn-set": map[string]any{"far": map[string]any{"asn": []any{"0.64700"}}}}

	set, err := deriveUpstreamSet(bgp, policyRoot)
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	want := upstreamSet{64700, 65546}
	if !slices.Equal(set, want) {
		t.Fatalf("set = %v, want %v", set, want)
	}
}

// TestUpstreamSetEmptyWithoutProviders proves a config with no provider
// session and no transit-asns derives an empty set, which compares nothing.
func TestUpstreamSetEmptyWithoutProviders(t *testing.T) {
	bgp := map[string]any{"peer": map[string]any{"p": map[string]any{
		"role":    map[string]any{"import": "peer"},
		"session": map[string]any{"asn": map[string]any{"remote": "64900"}},
	}}}
	set, err := deriveUpstreamSet(bgp, nil)
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	if len(set) != 0 {
		t.Fatalf("set = %v, want empty", set)
	}
}

func upstreamRoute(peerAS uint32, fields string) *sdk.FilterRoute {
	route := rpc.InputFilterRoute(&rpc.FilterUpdateInput{Filter: upstreamFilterName, PeerAS: peerAS, Fields: []byte(fields)})
	return &route
}

// TestUpstreamCheckRejectsPathThroughProvider proves the evaluator rejects a
// route whose AS path carries an upstream ASN at any position, an AS_SET
// member included (RFC 7454 Section 9), accepts one that carries none, never
// compares the session's own remote ASN, and follows a swapped set at once,
// as a reload that removes the provider swaps it.
func TestUpstreamCheckRejectsPathThroughProvider(t *testing.T) {
	t.Cleanup(func() { upstreamASNs.Store(nil) })
	installUpstreamSet(upstreamSet{64500})

	cases := []struct {
		name   string
		peerAS uint32
		fields string
		want   rpc.FilterAction
	}{
		{"customer route through the provider", 64600, `{"as-path":[64600,64500,65000]}`, sdk.FilterReject},
		{"provider at the origin", 64600, `{"as-path":[64600,64500]}`, sdk.FilterReject},
		{"provider inside an AS_SET", 64600, `{"as-path":[64600,[65001,64500]]}`, sdk.FilterReject},
		{"customer route of its own", 64600, `{"as-path":[64600,65000]}`, sdk.FilterAccept},
		{"no AS path", 64600, `{}`, sdk.FilterAccept},
		{"the provider's own session", 64500, `{"as-path":[64500,65000]}`, sdk.FilterAccept},
	}
	for _, tc := range cases {
		out, err := evaluateUpstream(upstreamRoute(tc.peerAS, tc.fields))
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if out.Action != tc.want {
			t.Errorf("%s: action %q, want %q", tc.name, out.Action, tc.want)
		}
	}

	// The reload that removes the provider peer installs a set without it.
	installUpstreamSet(nil)
	out, err := evaluateUpstream(upstreamRoute(64600, `{"as-path":[64600,64500,65000]}`))
	if err != nil {
		t.Fatalf("after the swap: %v", err)
	}
	if out.Action != sdk.FilterAccept {
		t.Fatalf("after the provider left: action %q, want accept", out.Action)
	}
}

// TestUpstreamCheckFailsMalformedPath proves a path the evaluator cannot read
// fails the member, so the declared on-error rejects the route.
func TestUpstreamCheckFailsMalformedPath(t *testing.T) {
	t.Cleanup(func() { upstreamASNs.Store(nil) })
	installUpstreamSet(upstreamSet{64500})
	if _, err := evaluateUpstream(upstreamRoute(64600, `{"as-path":["x"]}`)); err == nil {
		t.Fatal("a malformed as-path was judged")
	}
}

// TestUpstreamConfigReloadSwapsSet proves the config transaction carries the
// set: a reload whose bgp root drops the provider peer installs a set
// without its ASN, and one that only edits the asn-set under policy is not
// treated as unchanged.
func TestUpstreamConfigReloadSwapsSet(t *testing.T) {
	t.Cleanup(func() { upstreamASNs.Store(nil) })
	sections := func(bgp, policyJSON string) []sdk.ConfigSection {
		return []sdk.ConfigSection{{Root: configRootBGP, Data: bgp}, {Root: "policy", Data: policyJSON}}
	}
	withProvider := `{"bgp":{"peer":{"t":{"role":{"import":"customer"},"session":{"asn":{"remote":"64500"}}},` +
		`"c":{"role":{"import":"provider","transit-asns":"far"},"session":{"asn":{"remote":"64600"}}}}}}`
	withoutProvider := `{"bgp":{"peer":{"c":{"role":{"import":"provider","transit-asns":"far"},"session":{"asn":{"remote":"64600"}}}}}}`
	tx := newRoleConfigTx()
	if err := tx.configure(sections(withProvider, `{"policy":{"asn-set":{"far":{"asn":["64700"]}}}}`)); err != nil {
		t.Fatalf("configure: %v", err)
	}
	if got := *upstreamASNs.Load(); !slices.Equal(got, upstreamSet{64500, 64700}) {
		t.Fatalf("boot set = %v", got)
	}
	steps := []struct {
		bgp, policy string
		want        upstreamSet
	}{
		{withoutProvider, `{"policy":{"asn-set":{"far":{"asn":["64700"]}}}}`, upstreamSet{64700}},
		{withoutProvider, `{"policy":{"asn-set":{"far":{"asn":["64701"]}}}}`, upstreamSet{64701}},
	}
	for _, step := range steps {
		if err := tx.tx.Verify(sections(step.bgp, step.policy)); err != nil {
			t.Fatalf("verify: %v", err)
		}
		if err := tx.tx.Apply(); err != nil {
			t.Fatalf("apply: %v", err)
		}
		if got := *upstreamASNs.Load(); !slices.Equal(got, step.want) {
			t.Fatalf("set = %v, want %v", got, step.want)
		}
	}
}
