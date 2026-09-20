// Related: rfc7606.go — validateAttributeFlags, the entry point's flags check
// Related: ../../../core/bgp/attribute/attribute.go — RegisterName, which takes the name
// and the flags specification together
//
// VALIDATES: RFC 7606 Section 3.c on the attributes a PLUGIN owns. Section 3.c binds every
// attribute whose own specification fixes an Optional or a Transitive value, and two of
// those live outside the core: BGP-LS (29) in plugins/nlri/ls and OTC (35) in plugins/role.
// PREVENTS: the state both were in until 2026-09-20, recognized by the names registry and
// missing from the hand-authored flags table beside it, so a conflicting flags octet on
// either one was accepted.
//
// The package is message_test rather than message because these cases need the plugin
// registrations, and a plugin imports the message package. The blank imports are what put
// the two attributes in the registry, exactly as a shipped ze does through the generated
// composition root (internal/component/plugin/all).
package message_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"

	_ "github.com/ze-software/ze/internal/component/bgp/plugins/nlri/ls"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/role"
)

// pluginAttrFlagsUpdate builds a valid IPv4 unicast UPDATE's path attributes with one
// plugin-owned attribute appended under the flags each case supplies.
//
// The three well-known mandatory attributes are present and well-formed, so Section 3.d
// cannot fire first, and each appended value is the shape its own RFC requires, so no
// per-attribute validator can fire either. The flags octet is the only defect.
func pluginAttrFlagsUpdate(flags, code byte, value []byte) []byte {
	attrs := []byte{
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH, empty
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01, // NEXT_HOP = 192.0.2.1
	}
	attrs = append(attrs, flags, code, byte(len(value)))
	return append(attrs, value...)
}

// otcValue is the four-octet AS number RFC 9234 Section 5 fixes as the OTC value.
var otcValue = []byte{0x00, 0x00, 0xfd, 0xe8}

// TestRFC7606FlagsPluginAttributeConflict drives an UPDATE carrying a plugin-owned
// attribute whose flags octet conflicts with its own RFC through the receive-path entry
// point, and asserts the UPDATE is treated as a withdrawal.
//
// Isolation: the paired positive below sends the same UPDATEs with the conforming flags
// octet and asserts RFC7606ActionNone, so each case here fails on the flags and not on a
// neighboring rule. AttrCode pins the verdict to the attribute under test.
//
// RFC requirement: RFC7606-3.c-1 negative — a PLUGIN-owned attribute whose Optional or
// Transitive bit conflicts with the value its own specification fixes (BGP-LS marked
// transitive or well-known, OTC marked non-transitive or well-known) is treat-as-withdraw.
func TestRFC7606FlagsPluginAttributeConflict(t *testing.T) {
	for _, one := range []struct {
		name  string
		flags byte
		code  byte
		value []byte
	}{
		{
			// RFC 9552 Section 5.3: "The BGP-LS Attribute (assigned value 29 by IANA) is
			// an optional, non-transitive BGP Attribute".
			name: "BGP-LS marked transitive", flags: 0xc0, code: 29,
		},
		{
			name: "BGP-LS marked well-known", flags: 0x40, code: 29,
		},
		{
			// RFC 9234 Section 5: "The OTC Attribute is an optional transitive Path
			// Attribute of the UPDATE message with Attribute Type Code 35".
			name: "OTC marked non-transitive", flags: 0x80, code: 35, value: otcValue,
		},
		{
			name: "OTC marked well-known", flags: 0x40, code: 35, value: otcValue,
		},
	} {
		t.Run(one.name, func(t *testing.T) {
			attrs := pluginAttrFlagsUpdate(one.flags, one.code, one.value)

			result := message.ValidateUpdateRFC7606(attrs, true /*hasNLRI*/, false /*isIBGP*/, true /*asn4*/)
			require.Equal(t, message.RFC7606ActionTreatAsWithdraw, result.Action,
				"RFC 7606 Section 3.c asks for treat-as-withdraw, and this attribute's own RFC asks for nothing else")
			require.Equal(t, one.code, result.AttrCode,
				"the flags conflict must be attributed to the attribute that carries it")
			require.Contains(t, result.Description, "3.c")
		})
	}
}

// TestRFC7606FlagsPluginAttributeAsSpecifiedAccepted is the conforming side of the same
// UPDATE: each plugin-owned attribute carries the flags its specification fixes.
//
// VALIDATES: the check rejects only a conflict, so a conformant BGP-LS or OTC attribute
// still reaches the RIB.
// PREVENTS: a declaration that fixes the wrong bit values, which would withdraw every
// UPDATE carrying the attribute and which the negative cases above cannot detect.
//
// RFC requirement: RFC7606-3.c-1 positive — a plugin-owned attribute carrying the Optional
// and Transitive values its own specification fixes (BGP-LS optional non-transitive, OTC
// optional transitive) raises no conflict and the UPDATE is accepted.
func TestRFC7606FlagsPluginAttributeAsSpecifiedAccepted(t *testing.T) {
	for _, one := range []struct {
		name  string
		flags byte
		code  byte
		value []byte
	}{
		{name: "BGP-LS optional non-transitive", flags: 0x80, code: 29},
		{name: "OTC optional transitive", flags: 0xc0, code: 35, value: otcValue},
	} {
		t.Run(one.name, func(t *testing.T) {
			attrs := pluginAttrFlagsUpdate(one.flags, one.code, one.value)

			result := message.ValidateUpdateRFC7606(attrs, true /*hasNLRI*/, false /*isIBGP*/, true /*asn4*/)
			require.Equal(t, message.RFC7606ActionNone, result.Action,
				"these are the flags the attribute's own RFC fixes: %s", result.Description)
		})
	}
}
