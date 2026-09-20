// Design: docs/architecture/wire/nlri.md -- SR-Policy NLRI decode for CLI
// RFC: rfc/short/rfc9830.md -- SR-Policy NLRI wire format
// Related: types.go -- Parse, SRPolicy struct
// Related: register.go -- plugin registration

package srpolicy

import (
	"encoding/hex"
	"fmt"

	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
)

// DecodeNLRIHex decodes SR-Policy NLRI from hex bytes, returning a JSON-friendly map.
// Registered as InProcessNLRIDecoder in the plugin registry.
//
// addPath states whether the NLRI carries a 4-octet Path Identifier ahead of it
// (RFC 7911 Section 3). The hex alone cannot say, so the flag travels with it.
func DecodeNLRIHex(familyStr, hexStr string, addPath bool) (any, error) {
	afi, err := familyToAFI(familyStr)
	if err != nil {
		return nil, err
	}

	data, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, fmt.Errorf("invalid hex: %w", err)
	}

	// RFC 7911 Section 3: "the NLRI encoding MUST be extended by prepending the
	// Path Identifier field, which is of four octets."
	pathID, data, err := nlri.SplitPathID(data, addPath)
	if err != nil {
		return nil, err
	}

	sp, err := Parse(afi, data)
	if err != nil {
		return nil, fmt.Errorf("parse sr-policy: %w", err)
	}

	result := map[string]any{
		fieldColor:         sp.color,
		fieldDistinguisher: sp.distinguisher,
		fieldEndpoint:      sp.endpoint.String(),
	}
	if addPath {
		result["path-id"] = pathID
	}
	return result, nil
}

// familyToAFI resolves a family name to the AFI of the SR-Policy family it
// names. The name is looked up in the registry, which composed it from the AFI
// and SAFI parts register.go passed to family.MustRegister, so an unregistered
// name and a family that is not SR-Policy are both refused here.
func familyToAFI(familyStr string) (family.AFI, error) {
	fam, ok := family.LookupFamily(familyStr)
	if !ok {
		return 0, fmt.Errorf("unsupported sr-policy family: %s", familyStr)
	}
	if fam != IPv4SRPolicy && fam != IPv6SRPolicy {
		return 0, fmt.Errorf("unsupported sr-policy family: %s", familyStr)
	}
	return fam.AFI, nil
}
