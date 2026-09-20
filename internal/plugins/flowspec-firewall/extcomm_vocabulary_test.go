// Design: docs/guide/flowspec-protected-router.md -- the FlowSpec firewall bridge
// Related: internal/core/bgp/attribute/extcomm_decoded.go -- AppendDecoded, the renderer this reads
//
// The renderer and the bridge are one contract split over two packages, and
// nothing else in the tree holds both halves. AppendDecoded writes every
// extended community the daemon puts in an event, and parseExtendedCommunities
// reads them back. Every other test of the bridge hand-writes the string it
// feeds in, so a renderer that changes a spelling leaves them all green while
// the daemon stops acting on a peer's traffic filtering action.
//
// This file writes no spelling of its own. It builds the wire octets, renders
// them with the real producer, and asserts what the bridge then does.

package flowspecfirewall

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// extCommVocabularyCase pairs one wire extended community with the text
// AppendDecoded renders for it and the action the bridge takes on that text.
//
// want carries the performed action. unperformable is separate because the
// field holds the rendered text itself, which the case cannot spell without
// duplicating the renderer.
type extCommVocabularyCase struct {
	name          string
	comm          attribute.ExtendedCommunity
	rendered      string
	want          flowAction
	unperformable bool
}

// TestParseExtendedCommunitiesReadsWhatAppendDecodedWrites drives the real
// renderer into the real bridge for every extended community AppendDecoded
// names, and for the two hex forms it falls back to.
//
// The goal is the pairing rather than either half: a form the renderer names
// and the bridge does not read is a traffic filtering action a peer asked for
// and ze silently declined. RFC 8955 Section 7.4 is the case that made this
// worth a test of its own. It says the rt-redirect community "allows 3
// different encodings formats for the route-target (type 0x80, 0x81, 0x82)",
// so a four-byte-ASN peer sends type 0x82 where a two-byte-ASN peer sends type
// 0x80, and the bridge has to refuse both alike or the route installs a rate
// limit whose redirect was dropped.
func TestParseExtendedCommunitiesReadsWhatAppendDecodedWrites(t *testing.T) {
	cases := []extCommVocabularyCase{
		// RFC 4360 Section 4: "The value of the high-order octet of the Type
		// field for the Route Target Community can be 0x00, 0x01, or 0x02."
		// A Route Target is not a traffic filtering action, so all three are
		// ignored rather than refused.
		{
			name:     "route target two-octet AS is not an action",
			comm:     attribute.ExtendedCommunity{0x00, 0x02, 0xfd, 0xe8, 0x00, 0x00, 0x00, 0x01},
			rendered: "target:65000:1",
		},
		{
			name:     "route target IPv4 address is not an action",
			comm:     attribute.ExtendedCommunity{0x01, 0x02, 0xc0, 0x00, 0x02, 0x01, 0x00, 0x64},
			rendered: "target:192.0.2.1:100",
		},
		{
			name:     "route target four-octet AS is not an action",
			comm:     attribute.ExtendedCommunity{0x02, 0x02, 0x00, 0x01, 0x00, 0x00, 0x00, 0x64},
			rendered: "target:65536:100",
		},
		// RFC 4360 Section 5 gives the Route Origin the same three forms.
		{
			name:     "route origin two-octet AS is not an action",
			comm:     attribute.ExtendedCommunity{0x00, 0x03, 0x00, 0x64, 0x00, 0x00, 0x00, 0x02},
			rendered: "origin:100:0.0.0.2",
		},
		{
			name:     "route origin IPv4 address is not an action",
			comm:     attribute.ExtendedCommunity{0x01, 0x03, 0xc0, 0x00, 0x02, 0x02, 0x00, 0xc8},
			rendered: "origin:192.0.2.2:200",
		},
		{
			name:     "route origin four-octet AS is not an action",
			comm:     attribute.ExtendedCommunity{0x02, 0x03, 0x00, 0x01, 0x00, 0x01, 0x00, 0xc8},
			rendered: "origin:65537:200",
		},
		// RFC 8955 Section 7.4, all three encodings. Ze has no firewall action
		// that redirects into a VRF, so each is refused by name.
		{
			name:          "rt-redirect two-octet AS is refused",
			comm:          attribute.ExtendedCommunity{0x80, 0x08, 0xfd, 0xe8, 0x00, 0x00, 0x03, 0xe7},
			rendered:      "redirect:65000:999",
			unperformable: true,
		},
		{
			name:          "rt-redirect IPv4 address is refused",
			comm:          attribute.ExtendedCommunity{0x81, 0x08, 0xc0, 0x00, 0x02, 0x03, 0x03, 0xe7},
			rendered:      "redirect:192.0.2.3:999",
			unperformable: true,
		},
		{
			name:          "rt-redirect four-octet AS is refused",
			comm:          attribute.ExtendedCommunity{0x82, 0x08, 0x00, 0x01, 0x00, 0x02, 0x03, 0xe7},
			rendered:      "redirect:65538:999",
			unperformable: true,
		},
		// draft-ietf-idr-flowspec-redirect-ip, the other redirect ze cannot
		// perform. Its text is space-separated, so it exercises a second
		// spelling shape through the same refusal.
		{
			name:          "redirect to an IPv4 next hop is refused",
			comm:          attribute.ExtendedCommunity{0x01, 0x0c, 0x0a, 0x00, 0x00, 0x01, 0x00, 0x00},
			rendered:      "redirect-to-nexthop 10.0.0.1",
			unperformable: true,
		},
		{
			name:          "copy to an IPv4 next hop is refused",
			comm:          attribute.ExtendedCommunity{0x01, 0x0c, 0x0a, 0x00, 0x00, 0x01, 0x00, 0x01},
			rendered:      "copy-to-nexthop 10.0.0.1",
			unperformable: true,
		},
		// RFC 8955 Section 7.3. Ze's firewall has no sampling action.
		{
			name:          "traffic-action is refused",
			comm:          attribute.ExtendedCommunity{0x80, 0x07, 0x00, 0x00, 0x00, 0x00, 0x00, 0x03},
			rendered:      "traffic-action:sample-terminal",
			unperformable: true,
		},
		// RFC 8955 Sections 7.1 and 7.2. Both render "rate-limit:<n>" and only
		// the suffix carries the unit, so the pair is read together.
		{
			name:     "traffic-rate-bytes limits bytes",
			comm:     attribute.ExtendedCommunity{0x80, 0x06, 0x00, 0x00, 0x44, 0x7a, 0x00, 0x00},
			rendered: "rate-limit:1000",
			want:     flowAction{rateLimit: 1000},
		},
		{
			name:     "traffic-rate-packets limits packets",
			comm:     attribute.ExtendedCommunity{0x80, 0x0c, 0x00, 0x00, 0x44, 0x7a, 0x00, 0x00},
			rendered: "rate-limit:1000:packets",
			want:     flowAction{rateLimit: 1000, rateInPackets: true},
		},
		{
			name:     "a zero traffic-rate discards",
			comm:     attribute.ExtendedCommunity{0x80, 0x06, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
			rendered: "rate-limit:0",
			want:     flowAction{discard: true},
		},
		// RFC 8955 Section 7.5.
		{
			name:     "traffic-marking marks the DSCP",
			comm:     attribute.ExtendedCommunity{0x80, 0x09, 0x00, 0x00, 0x00, 0x00, 0x00, 0x2e},
			rendered: "mark:46",
			want:     flowAction{markDSCP: 46, hasMark: true},
		},
		// Communities the renderer names that belong to other features. None
		// is a traffic filtering action, so a FlowSpec route carrying one is
		// installed rather than refused.
		{
			name:     "layer2 info is not an action",
			comm:     attribute.ExtendedCommunity{0x80, 0x0a, 0x13, 0x00, 0x05, 0xdc, 0x00, 0x6f},
			rendered: "l2info:19:0:1500:111",
		},
		{
			name:     "an interface set is not an action",
			comm:     attribute.ExtendedCommunity{0x07, 0x02, 0x00, 0x00, 0xfd, 0xe8, 0x80, 0x0a},
			rendered: "interface-set:transitive:output:65000:10",
		},
		{
			name:     "a MUP segment identifier is not an action",
			comm:     attribute.ExtendedCommunity{0x0c, 0x00, 0x00, 0x0a, 0x00, 0x00, 0x00, 0x0a},
			rendered: "mup:10:10",
		},
		// The hex arm, both polarities. RFC 8955 Section 7 takes every traffic
		// filtering action it defines from types 0x80, 0x81 and 0x82, so an
		// undecoded type in that range is an action and is refused, while an
		// undecoded type outside it is not and is ignored.
		{
			name:          "an undecoded type in the FlowSpec action range is refused",
			comm:          attribute.ExtendedCommunity{0x80, 0x99, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06},
			rendered:      "0x8099:010203040506",
			unperformable: true,
		},
		{
			name:     "an undecoded type outside that range is ignored",
			comm:     attribute.ExtendedCommunity{0x00, 0xff, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06},
			rendered: "0x00ff:010203040506",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rendered := string(tc.comm.AppendDecoded(nil))
			require.Equal(t, tc.rendered, rendered, "the renderer writes the text the bridge is built to read")

			want := tc.want
			if tc.unperformable {
				want.unperformable = rendered
			}
			assert.Equal(t, want, parseExtendedCommunities([]string{rendered}))
		})
	}
}

// extCommLayer2InfoType is the one sub-type inside the FlowSpec action type
// range that is not a traffic filtering action.
//
// RFC 4761 Section 3.2.4 took it for VPLS before RFC 8955 existed: "The
// extended community value is to be allocated by IANA (currently used value is
// 0x800A)." A VPLS route carries it, a FlowSpec route has no reason to, and
// nothing in either RFC makes it an instruction to a filtering engine. So the
// bridge ignores it, and the sweep below excludes it by name rather than by
// widening its rule.
const extCommLayer2InfoType = 0x800a

// TestEveryFlowSpecActionTypeIsAnsweredByTheBridge sweeps the whole 0x80-0x82
// type range that RFC 8955 Section 7 takes its traffic filtering actions from,
// and asserts the bridge answers for each sub-type: it either performs the
// action or records it as unperformable.
//
// The table above names the sub-types that exist today. This one covers the one
// that does not exist yet. The bridge refuses an unnamed sub-type on the "0x80"
// prefix of the hex fallback, so a sub-type the renderer LATER names escapes
// that refusal the moment it gains an arm: the text stops starting with "0x80"
// and no bridge arm reads the new word. A peer's instruction then reaches the
// firewall as silence, and silence here widens the rule rather than narrowing
// it. That is the pairing this file exists to hold, and this is the half no
// table of known values can cover.
func TestEveryFlowSpecActionTypeIsAnsweredByTheBridge(t *testing.T) {
	for typeHigh := 0x80; typeHigh <= 0x82; typeHigh++ {
		for subType := 0x00; subType <= 0xff; subType++ {
			if typeHigh<<8|subType == extCommLayer2InfoType {
				continue
			}

			comm := attribute.ExtendedCommunity{byte(typeHigh), byte(subType), 0x00, 0x00, 0x00, 0x00, 0x00, 0x01}
			rendered := string(comm.AppendDecoded(nil))
			act := parseExtendedCommunities([]string{rendered})

			answered := act.discard || act.rateLimit != 0 || act.hasMark || act.unperformable != ""
			assert.True(t, answered, "type 0x%02x%02x renders %q and the bridge neither performs nor refuses it", typeHigh, subType, rendered)
		}
	}
}
