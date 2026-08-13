// VALIDATES: the `blackhole` container resolves to one Rule per peer, keyed by
// remote IP, with both leaf-lists accumulating down bgp, group and peer; and the
// 4-octet scan that answers whether a route carries an agreed community.
// PREVENTS: three deciders reading the same container three ways -- the honoring
// path, the origin-validation exemption and the origination gate each need this
// answer, and a second copy of the walk is what would let them disagree.

package blackholecfg

import (
	"encoding/json"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/configjson"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// parseJSON runs Parse over a bgp subtree written as JSON, which is the shape
// the config framework delivers.
func parseJSON(t *testing.T, jsonStr string) map[configjson.PeerConfigKey]Rule {
	t.Helper()
	rules, err := Parse(bgpSubtree(t, jsonStr))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return rules
}

// bgpSubtree unwraps the {"bgp":{...}} envelope the config framework delivers.
func bgpSubtree(t *testing.T, jsonStr string) map[string]any {
	t.Helper()
	var tree map[string]any
	if err := json.Unmarshal([]byte(jsonStr), &tree); err != nil {
		t.Fatalf("test fixture is not JSON: %v", err)
	}
	bgpTree, ok := tree["bgp"].(map[string]any)
	if !ok {
		t.Fatalf("test fixture has no bgp section")
	}
	return bgpTree
}

// peerKey is the key a configured peer's rule is stored under.
func peerKey(addr string) configjson.PeerConfigKey {
	return configjson.PeerConfigKey{ID: addr}
}

func TestParsePeerLevel(t *testing.T) {
	rules := parseJSON(t, `{"bgp":{"peer":{"p1":{
		"connection":{"remote":{"ip":"192.0.2.1"}},
		"blackhole":{"communities":["blackhole"],"prefixes":["10.0.0.0/24"]}}}}}`)

	rule, ok := rules[peerKey("192.0.2.1")]
	if !ok {
		t.Fatalf("peer not keyed by its remote IP, got %v", rules)
	}
	if !rule.Agreed(attribute.CommunityBlackhole) {
		t.Errorf("Agreed(BLACKHOLE) = false: the shorthand keyword did not resolve to 65535:666")
	}
	if len(rule.Authorized) != 1 || rule.Authorized[0].String() != "10.0.0.0/24" {
		t.Errorf("Authorized = %v, want [10.0.0.0/24]", rule.Authorized)
	}
}

// The community is a value, not a spelling. RFC 7999 Section 4 suggests the
// keyword; 65535:666 is the same agreement.
func TestParseCommunitySpellingsAreOneValue(t *testing.T) {
	for _, spelling := range []string{"blackhole", "65535:666"} {
		t.Run(spelling, func(t *testing.T) {
			rules := parseJSON(t, `{"bgp":{"peer":{"p1":{
				"connection":{"remote":{"ip":"192.0.2.1"}},
				"blackhole":{"communities":["`+spelling+`"]}}}}}`)
			if !rules[peerKey("192.0.2.1")].Agreed(attribute.CommunityBlackhole) {
				t.Errorf("%q did not resolve to the well-known value", spelling)
			}
		})
	}
}

// An operator's own community is the common case, and it is NOT the well-known
// value. A session that agreed only to 65001:666 has not agreed to BLACKHOLE.
func TestParseOwnCommunityIsNotTheWellKnownOne(t *testing.T) {
	rules := parseJSON(t, `{"bgp":{"peer":{"p1":{
		"connection":{"remote":{"ip":"192.0.2.1"}},
		"blackhole":{"communities":["65001:666"]}}}}}`)

	rule := rules[peerKey("192.0.2.1")]
	if !rule.Agreed(attribute.Community(65001<<16 | 666)) {
		t.Error("Agreed(65001:666) = false on the session that named it")
	}
	if rule.Agreed(attribute.CommunityBlackhole) {
		t.Error("Agreed(BLACKHOLE) = true on a session that named only its own community")
	}
}

func TestParseAccumulatesDownTheLevels(t *testing.T) {
	rules := parseJSON(t, `{"bgp":{
		"blackhole":{"communities":["65000:666"],"prefixes":["10.0.0.0/8"]},
		"group":{"g1":{
			"blackhole":{"communities":["65001:666"],"prefixes":["10.1.0.0/16"]},
			"peer":{"p1":{
				"connection":{"remote":{"ip":"192.0.2.1"}},
				"blackhole":{"communities":["blackhole"],"prefixes":["10.1.1.0/24"]}}}}}}}`)

	rule := rules[peerKey("192.0.2.1")]
	if len(rule.Communities) != 3 {
		t.Errorf("Communities = %v, want the bgp, group and peer values", rule.Communities)
	}
	for _, want := range []attribute.Community{
		attribute.Community(65000<<16 | 666),
		attribute.Community(65001<<16 | 666),
		attribute.CommunityBlackhole,
	} {
		if !rule.Agreed(want) {
			t.Errorf("Agreed(%v) = false: a narrower level dropped a wider one's community", want)
		}
	}
	if len(rule.Authorized) != 3 {
		t.Errorf("Authorized = %v, want three accumulated prefixes", rule.Authorized)
	}
}

// The four combinations of the two leaf-lists, settled explicitly. A default
// that only works in the happy case is worse than none.
//
// Row 1 is the new behavior: stating the prefixes a neighbor may blackhole
// within IS the explicit configuration directive RFC 7999 Section 4 asks for, so
// the well-known value is assumed. Row 2 is the one that must not drift: a
// stated set is taken exactly, because an operator who names their own community
// and not 65535:666 means it.
func TestParseDefaultCommunity(t *testing.T) {
	own := attribute.Community(65001<<16 | 666)
	cases := []struct {
		name     string
		block    string
		present  bool
		wellKnwn bool
		ownValue bool
	}{
		{
			name:     "prefixes alone defaults to the well-known value",
			block:    `"prefixes":["10.0.0.0/24"]`,
			present:  true,
			wellKnwn: true,
		},
		{
			name:     "a stated set is taken exactly, the default does not union in",
			block:    `"communities":["65001:666"],"prefixes":["10.0.0.0/24"]`,
			present:  true,
			ownValue: true,
		},
		{
			name:    "communities alone stays as stated and covers nothing",
			block:   `"communities":["65001:666"]`,
			present: true,

			ownValue: true,
		},
		{
			name:    "neither leaf discards nothing, which is Section 4's default",
			block:   `"other":"x"`,
			present: false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rules := parseJSON(t, `{"bgp":{"peer":{"p1":{
				"connection":{"remote":{"ip":"192.0.2.1"}},
				"blackhole":{`+c.block+`}}}}}`)
			rule, ok := rules[peerKey("192.0.2.1")]
			if ok != c.present {
				t.Fatalf("peer present = %v, want %v", ok, c.present)
			}
			if !c.present {
				return
			}
			if got := rule.Agreed(attribute.CommunityBlackhole); got != c.wellKnwn {
				t.Errorf("Agreed(BLACKHOLE) = %v, want %v", got, c.wellKnwn)
			}
			if got := rule.Agreed(own); got != c.ownValue {
				t.Errorf("Agreed(65001:666) = %v, want %v", got, c.ownValue)
			}
		})
	}
}

// The default is a resolved-rule answer, so a community stated at ANY level
// suppresses it for the peer under that level.
func TestParseDefaultCommunitySuppressedByAWiderLevel(t *testing.T) {
	rules := parseJSON(t, `{"bgp":{
		"blackhole":{"communities":["65001:666"]},
		"peer":{"p1":{
			"connection":{"remote":{"ip":"192.0.2.1"}},
			"blackhole":{"prefixes":["10.0.0.0/24"]}}}}}`)

	rule := rules[peerKey("192.0.2.1")]
	if rule.Agreed(attribute.CommunityBlackhole) {
		t.Error("the default fired although a wider level stated a community: the operator cannot turn 65535:666 off")
	}
}

// Row 1, on the wire. A defaulted session honors the well-known value and
// nothing else.
func TestParseDefaultCommunityMatchesOnlyTheWellKnownValue(t *testing.T) {
	rules := parseJSON(t, `{"bgp":{"peer":{"p1":{
		"connection":{"remote":{"ip":"192.0.2.1"}},
		"blackhole":{"prefixes":["10.0.0.0/24"]}}}}}`)

	want := rules[peerKey("192.0.2.1")].Communities
	if !Carries([]byte{0xFF, 0xFF, 0x02, 0x9A}, want) {
		t.Error("a route carrying 65535:666 is not honored on a session configured with prefixes alone")
	}
	if Carries([]byte{0xFD, 0xE9, 0x02, 0x9A}, want) {
		t.Error("a route carrying only 65001:666 is honored by the default: the default is not the well-known value alone")
	}
}

// A peer that stated nothing is absent, so the honoring path costs one map miss.
func TestParseSkipsPeersWithNoRule(t *testing.T) {
	rules := parseJSON(t, `{"bgp":{"peer":{"p1":{
		"connection":{"remote":{"ip":"192.0.2.1"}}}}}}`)
	if len(rules) != 0 {
		t.Errorf("rules = %v, want empty for a peer with no blackhole block", rules)
	}
}

// A listen-range group is the canonical IXP route-server shape, and its members
// exist nowhere in the config document -- the reactor builds them from the
// template when a connection arrives. The block an operator writes on the group
// is therefore the ONLY place the agreement can be stated, so it must load, and
// it must land under a key a member can be resolved through.
//
// Both earlier answers were wrong. The block used to load and do nothing,
// because the traversal never visited a dynamic group at all. Visiting it then
// made Parse REFUSE the whole configuration, because the group's remote IP is
// the "dynamic" placeholder and no address parses out of it.
func TestParseKeysADynamicGroupsBlockUnderTheGroup(t *testing.T) {
	rules := parseJSON(t, `{"bgp":{"group":{"ix":{
		"connection":{"remote":{"ip":"dynamic","range":["192.0.2.0/24"]}},
		"blackhole":{"communities":["65001:666"],"prefixes":["10.0.0.0/24"]}}}}}`)

	rule, ok := rules[configjson.GroupKey("ix")]
	if !ok {
		t.Fatalf("a dynamic group's blackhole block is not keyed under the group, got %v", rules)
	}
	if !rule.Agreed(attribute.Community(65001<<16 | 666)) {
		t.Error("the group's stated community did not reach the rule")
	}
	if len(rule.Authorized) != 1 || rule.Authorized[0].String() != "10.0.0.0/24" {
		t.Errorf("Authorized = %v, want the group's stated prefix", rule.Authorized)
	}

	// The placeholder is not an identity. Keyed under it, the entry would be
	// unreachable: every consumer produces an address or a group name.
	if _, bad := rules[peerKey(configjson.DynamicPeerIP)]; bad {
		t.Error("the template was stored under the literal \"dynamic\" placeholder, which no consumer can produce")
	}
	if len(rules) != 1 {
		t.Errorf("rules = %v, want exactly the group's entry", rules)
	}
}

// A dynamic group that also names static peers states two configurations, and
// both are in force. The named peer keeps the address key it has today and
// accumulates the group's leaves; the template keeps its own entry for the
// members the reactor will build.
func TestParseKeepsBothAStaticPeerAndTheTemplateOfItsDynamicGroup(t *testing.T) {
	rules := parseJSON(t, `{"bgp":{"group":{"ix":{
		"connection":{"remote":{"ip":"dynamic","range":["192.0.2.0/24"]}},
		"blackhole":{"communities":["65001:666"]},
		"peer":{"p1":{
			"connection":{"remote":{"ip":"198.51.100.1"}},
			"blackhole":{"prefixes":["10.0.0.0/24"]}}}}}}}`)

	peer, ok := rules[peerKey("198.51.100.1")]
	if !ok {
		t.Fatalf("the named peer lost its address key, got %v", rules)
	}
	if !peer.Agreed(attribute.Community(65001<<16 | 666)) {
		t.Error("the named peer did not accumulate its group's community")
	}
	if len(peer.Authorized) != 1 {
		t.Errorf("Authorized = %v, want the peer's own prefix", peer.Authorized)
	}

	tmpl, ok := rules[configjson.GroupKey("ix")]
	if !ok {
		t.Fatalf("the template entry is missing, got %v", rules)
	}
	if len(tmpl.Authorized) != 0 {
		t.Errorf("the template took the named peer's prefix: Authorized = %v", tmpl.Authorized)
	}
}

// The refusal is preserved where it is still right. A NAMED peer is never built
// from a template, so a member address the consumer could produce does not
// exist for it, and the agreement would read as in force while doing nothing.
func TestParseStillRefusesANamedPeerWithNoUsableRemoteIP(t *testing.T) {
	_, err := Parse(bgpSubtree(t, `{"bgp":{"group":{"ix":{
		"connection":{"remote":{"ip":"dynamic","range":["192.0.2.0/24"]}},
		"peer":{"p1":{"blackhole":{"communities":["blackhole"]}}}}}}}`))
	if err == nil {
		t.Error("Parse accepted a named peer whose only remote IP is its group's placeholder")
	}
}

// A group that is not dynamic yields no template entry, so a plain peer-group is
// untouched by the change (AC-3 at this consumer).
func TestParseYieldsNoTemplateForAPlainGroup(t *testing.T) {
	rules := parseJSON(t, `{"bgp":{"group":{"ix":{
		"blackhole":{"communities":["65001:666"]},
		"peer":{"p1":{"connection":{"remote":{"ip":"198.51.100.1"}}}}}}}}`)

	if _, ok := rules[configjson.GroupKey("ix")]; ok {
		t.Errorf("a plain group produced a template entry, got %v", rules)
	}
	if _, ok := rules[peerKey("198.51.100.1")]; !ok {
		t.Errorf("the grouped peer lost its inherited rule, got %v", rules)
	}
}

func TestParseRefusesBadValues(t *testing.T) {
	cases := []struct {
		name string
		json string
	}{
		{"community", `{"bgp":{"peer":{"p1":{
			"connection":{"remote":{"ip":"192.0.2.1"}},
			"blackhole":{"communities":["not-a-community"]}}}}}`},
		{"prefix", `{"bgp":{"peer":{"p1":{
			"connection":{"remote":{"ip":"192.0.2.1"}},
			"blackhole":{"prefixes":["10.0.0.1"]}}}}}`},
		{"no remote IP", `{"bgp":{"peer":{"p1":{
			"blackhole":{"communities":["blackhole"]}}}}}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var tree map[string]any
			if err := json.Unmarshal([]byte(c.json), &tree); err != nil {
				t.Fatalf("fixture: %v", err)
			}
			bgpTree, _ := tree["bgp"].(map[string]any)
			if _, err := Parse(bgpTree); err == nil {
				t.Error("Parse accepted a value it cannot act on: the operator would read an agreement that does nothing")
			}
		})
	}
}

// The wire scan that answers "does this route carry a community this session
// agreed to". The COMMUNITIES attribute is a set of 4-octet values, so the scan
// must find a value at any position and must not match a value that merely
// shares two octets with one.
func TestCarries(t *testing.T) {
	blackhole := []byte{0xFF, 0xFF, 0x02, 0x9A}
	noExport := []byte{0xFF, 0xFF, 0xFF, 0x01}
	other := []byte{0x00, 0x64, 0x00, 0x01}

	wellKnown := []attribute.Community{attribute.CommunityBlackhole}
	// 65001:666, an operator's own RTBH community. Operators use one of these
	// far more often than the well-known value, which is why the scan takes the
	// set from configuration instead of holding one constant.
	ownValue := []byte{0xFD, 0xE9, 0x02, 0x9A}
	own := []attribute.Community{attribute.Community(65001<<16 | 666)}

	cases := []struct {
		name string
		data []byte
		want []attribute.Community
		ok   bool
	}{
		{"only value", blackhole, wellKnown, true},
		{"first of three", concat(blackhole, noExport, other), wellKnown, true},
		{"last of three", concat(other, noExport, blackhole), wellKnown, true},
		{"middle of three", concat(other, blackhole, noExport), wellKnown, true},
		{"absent", concat(other, noExport), wellKnown, false},
		{"empty", nil, wellKnown, false},
		{"straddles a boundary, must not match", concat([]byte{0x00, 0xFF, 0xFF, 0x02}, []byte{0x9A, 0x00, 0x00, 0x00}), wellKnown, false},
		{"truncated attribute", []byte{0xFF, 0xFF, 0x02}, wellKnown, false},
		{"no community agreed matches nothing", blackhole, nil, false},

		// The case a hardcoded value cannot express at all.
		{"an operator's own community", concat(other, ownValue), own, true},
		{"the well-known value does not match an operator's own agreement", blackhole, own, false},
		{"an operator's own value does not match the well-known agreement", ownValue, wellKnown, false},
		{"either of two agreed values matches", concat(other, ownValue), append(append([]attribute.Community{}, wellKnown...), own...), true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Carries(c.data, c.want); got != c.ok {
				t.Errorf("Carries = %v, want %v", got, c.ok)
			}
		})
	}
}

func concat(parts ...[]byte) []byte {
	var out []byte
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}
