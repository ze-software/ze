// Design: docs/architecture/wire/nlri.md -- SR-Policy NLRI decode for CLI
// RFC: rfc/short/rfc9830.md -- SR-Policy NLRI wire format
// Related: types.go -- Parse, SRPolicy struct
// Related: register.go -- plugin registration

package srpolicy

import (
	"encoding/hex"
	"fmt"

	"github.com/ze-software/ze/internal/core/family"
)

// DecodeNLRIHex decodes SR-Policy NLRI from hex bytes, returning a JSON-friendly map.
// Registered as InProcessNLRIDecoder in the plugin registry.
func DecodeNLRIHex(familyStr, hexStr string) (any, error) {
	afi, err := familyToAFI(familyStr)
	if err != nil {
		return nil, err
	}

	data, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, fmt.Errorf("invalid hex: %w", err)
	}

	sp, err := Parse(afi, data)
	if err != nil {
		return nil, fmt.Errorf("parse sr-policy: %w", err)
	}

	return map[string]any{
		"color":         sp.color,
		"distinguisher": sp.distinguisher,
		"endpoint":      sp.endpoint.String(),
	}, nil
}

func familyToAFI(familyStr string) (family.AFI, error) {
	switch familyStr {
	case "ipv4/sr-policy":
		return family.AFIIPv4, nil
	case "ipv6/sr-policy":
		return family.AFIIPv6, nil
	default:
		return 0, fmt.Errorf("unsupported sr-policy family: %s", familyStr)
	}
}
