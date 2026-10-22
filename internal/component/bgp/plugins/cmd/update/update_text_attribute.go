// Design: docs/architecture/api/update-syntax.md -- the generic attribute keyword
// Overview: update_text.go -- parseCommonAttributeText reads the keyword, ParseUpdateText merges it
// Related: ../../../textparse/rawattr.go -- ParseRawAttribute, the reader the config file shares
// RFC: rfc/short/rfc4271.md -- the attribute triple, its flags, and a repeated attribute

package update

import (
	"errors"
	"fmt"

	"github.com/ze-software/ze/internal/component/bgp/route"
	"github.com/ze-software/ze/internal/component/bgp/textparse"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// errRawAttributeTwice refuses a generic attribute whose type code another
// statement of the same command already wrote, because an UPDATE carrying it
// would be malformed. RFC 4271 Section 6.3: "If any attribute appears more than
// once in the UPDATE message, then the Error Subcode MUST be set to Malformed
// Attribute List." Ze refuses to build that message.
var errRawAttributeTwice = errors.New("attribute given twice")

// parseRawAttributeText reads `[ 0x<code> 0x<flags> 0x<value> ]` from args,
// args[0] being the token after the keyword, and answers the attribute with the
// count of tokens it took. textparse.ParseRawAttribute reads the octets,
// refuses the flags Ze must not send, and decodes the value of a code one of
// the update text keywords writes with that attribute's wire decoder, refusing
// a malformed one. Such an attribute goes into the block as its typed form
// writes it, with the flags its specification fixes, exactly as the config
// file's reader stores it in the typed field (bgpconfig.parseRawAttributeInto).
func parseRawAttributeText(args []string) (registry.PluginRouteAttr, int, error) {
	value, consumed, err := route.ParseBracketedValue(args)
	if err != nil {
		return registry.PluginRouteAttr{}, 0, fmt.Errorf("%w: %w", textparse.ErrRawAttributeShape, err)
	}
	raw, err := textparse.ParseRawAttribute(value)
	if err != nil {
		return registry.PluginRouteAttr{}, 0, err
	}
	if raw.Typed != nil {
		return registry.PluginRouteAttr{Code: raw.Code, Flags: uint8(raw.Typed.Flags()), Value: raw.Value}, consumed, nil
	}
	return registry.PluginRouteAttr{Code: raw.Code, Flags: raw.Flags, Value: raw.Value}, consumed, nil
}

// withRawAttrs answers the attribute block with the command's generic
// attributes merged in by type code. It refuses a code given twice, between
// two generic attributes or beside a keyword that writes the same attribute
// (errRawAttributeTwice), so the merge that follows never sees a duplicate:
// a generic MED beside med is one MED too many, not a replacement.
func withRawAttrs(wire *attribute.AttributesWire, raw []registry.PluginRouteAttr) (*attribute.AttributesWire, error) {
	if len(raw) == 0 {
		return wire, nil
	}
	var base []byte
	if wire != nil {
		base = wire.Packed()
	}
	for i := range raw {
		code := attribute.AttributeCode(raw[i].Code)
		if _, _, _, found := attribute.AttrFind(base, code); found {
			return nil, fmt.Errorf("%w: attribute 0x%02x is also written by its own keyword; keep one of the two", errRawAttributeTwice, raw[i].Code)
		}
		for j := range i {
			if raw[j].Code == raw[i].Code {
				return nil, fmt.Errorf("%w: attribute 0x%02x is written twice; keep one", errRawAttributeTwice, raw[i].Code)
			}
		}
	}
	return withSectionAttrs(wire, raw)
}
