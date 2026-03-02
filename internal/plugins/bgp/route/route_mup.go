// Design: docs/architecture/route-types.md — MUP route parsing
// Overview: route.go — core route types and attribute parsing

//nolint:goconst // Many string literals are intentional for BGP protocol keywords
package route

import (
	"fmt"
	"strconv"
	"strings"

	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/attribute"
	"codeberg.org/thomas-mangin/ze/internal/plugins/bgp/context"
	bgptypes "codeberg.org/thomas-mangin/ze/internal/plugins/bgp/types"
)

// MUP route type constants.
const (
	MUPRouteTypeISD  = "mup-isd"  // Interwork Segment Discovery
	MUPRouteTypeDSD  = "mup-dsd"  // Direct Segment Discovery
	MUPRouteTypeT1ST = "mup-t1st" // Type 1 Session Transformed
	MUPRouteTypeT2ST = "mup-t2st" // Type 2 Session Transformed
)

// validMUPRouteTypes is the set of valid MUP route types.
var validMUPRouteTypes = map[string]bool{
	MUPRouteTypeISD:  true,
	MUPRouteTypeDSD:  true,
	MUPRouteTypeT1ST: true,
	MUPRouteTypeT2ST: true,
}

// ParseMUPArgs parses MUP route arguments.
// Format: <route-type> <prefix/addr> rd <RD> next-hop <NH> [extended-community [...]] [bgp-prefix-sid-srv6 (...)].
// Route types: mup-isd, mup-dsd, mup-t1st, mup-t2st.
func ParseMUPArgs(args []string, isIPv6 bool) (bgptypes.MUPRouteSpec, error) {
	spec := bgptypes.MUPRouteSpec{
		IsIPv6: isIPv6,
	}

	if len(args) < 1 {
		return spec, fmt.Errorf("missing MUP route type")
	}

	// First arg is route type
	routeType := strings.ToLower(args[0])
	if !validMUPRouteTypes[routeType] {
		return spec, fmt.Errorf("invalid MUP route type: %s (expected: mup-isd, mup-dsd, mup-t1st, mup-t2st)", args[0])
	}
	spec.RouteType = routeType

	if len(args) < 2 {
		return spec, fmt.Errorf("missing prefix/address for MUP route")
	}

	// Second arg is prefix or address depending on route type
	switch routeType {
	case MUPRouteTypeISD, MUPRouteTypeT1ST:
		spec.Prefix = args[1]
	case MUPRouteTypeDSD, MUPRouteTypeT2ST:
		spec.Address = args[1]
	}

	// Use wire-first Builder for attribute parsing
	builder := attribute.NewBuilder()

	// Parse remaining args as key-value pairs
	for i := 2; i < len(args); i++ {
		key := strings.ToLower(args[i])

		// Handle MUP-specific attributes BEFORE common attribute parsing.
		// These must be set in bgptypes.MUPRouteSpec fields, not just in the builder.
		switch key {
		case "rd":
			if i+1 >= len(args) {
				return spec, fmt.Errorf("missing rd value")
			}
			spec.RD = args[i+1]
			i++
			continue

		case "next-hop":
			if i+1 >= len(args) {
				return spec, fmt.Errorf("missing next-hop value")
			}
			spec.NextHop = args[i+1]
			i++
			continue

		case "teid":
			if i+1 >= len(args) {
				return spec, fmt.Errorf("missing teid value")
			}
			spec.TEID = args[i+1]
			i++
			continue

		case "qfi":
			if i+1 >= len(args) {
				return spec, fmt.Errorf("missing qfi value")
			}
			qfi, err := strconv.ParseUint(args[i+1], 10, 8)
			if err != nil {
				return spec, fmt.Errorf("invalid qfi value: %s", args[i+1])
			}
			spec.QFI = uint8(qfi)
			i++
			continue

		case "endpoint":
			if i+1 >= len(args) {
				return spec, fmt.Errorf("missing endpoint value")
			}
			spec.Endpoint = args[i+1]
			i++
			continue

		case "source":
			if i+1 >= len(args) {
				return spec, fmt.Errorf("missing source value")
			}
			spec.Source = args[i+1]
			i++
			continue

		case "extended-community":
			if i+1 >= len(args) {
				return spec, fmt.Errorf("missing extended-community value")
			}
			// Collect bracketed value - must set spec.ExtCommunity for MUP
			tokens, consumed := attribute.ParseBracketedList(args[i+1:])
			spec.ExtCommunity = "[" + strings.Join(tokens, " ") + "]"
			i += consumed
			continue

		case "bgp-prefix-sid-srv6":
			if i+1 >= len(args) {
				return spec, fmt.Errorf("missing bgp-prefix-sid-srv6 value")
			}
			// Parse parenthesized value
			value, consumed, err := parseParenthesizedValue(args[i+1:])
			if err != nil {
				return spec, fmt.Errorf("invalid bgp-prefix-sid-srv6: %w", err)
			}
			spec.PrefixSID = value
			i += consumed
			continue
		}

		// Try common attribute parsing with Builder (wire-first)
		// for attributes that are NOT MUP-specific (origin, local-preference, etc.)
		consumed, err := parseCommonAttributeBuilder(key, args, i, builder)
		if err != nil {
			return spec, err
		}
		if consumed > 0 {
			i += consumed
			continue
		}

		// Unknown keyword - could be future extension, skip silently
	}

	// Build wire-format attributes
	wireBytes := builder.Build()
	if len(wireBytes) > 0 {
		spec.Wire = attribute.NewAttributesWire(wireBytes, context.APIContextID)
	}

	return spec, nil
}
