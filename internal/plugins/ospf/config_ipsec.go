// Design: docs/architecture/ospf/ospf-ext-16-ipsec-auth.md -- RFC 4552 manual IPsec (IPv6 family).
// Related: config.go -- parseInterface calls parseIPsec; validateConfigAF calls the checks.
// RFC: rfc/short/rfc4552.md -- OSPFv3 IPsec AH/ESP; rfc/short/rfc4303.md -- ESP SPI/keys.
//
// The RFC 4552 IPsec config surface lives here (not in config.go) so it can be
// removed with the feature. It is parsed for every OSPF interface but is VALID
// only under the IPv6 (OSPFv3) address family; validateConfigAF rejects it on an
// IPv4-family interface (RFC 4552 §5) and alongside a RFC 7166 key chain (the two
// auth paths are mutually exclusive, spec-ospf-ext-16 AC-6).

package ospf

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
)

var (
	// ErrIPsecIPv4Family rejects an IPsec block on an IPv4-family (OSPFv2) interface.
	ErrIPsecIPv4Family = errors.New("ospf: ipsec is an IPv6-family (OSPFv3) feature only (RFC 4552 §5); it cannot be configured on an IPv4-family interface")
	// ErrIPsecSPIReserved rejects an SPI in the RFC 4303 §2.1 reserved range 0..255.
	ErrIPsecSPIReserved = errors.New("ospf: ipsec spi must be 256..4294967295 (RFC 4303 §2.1 reserves 0..255)")
	// ErrIPsecProtocol rejects a protocol other than ah/esp.
	ErrIPsecProtocol = errors.New("ospf: ipsec protocol must be ah or esp (RFC 4552 §3)")
	// ErrIPsecAuthAlgo rejects an integrity algorithm outside sha1/sha256/sha384/sha512.
	ErrIPsecAuthAlgo = errors.New("ospf: ipsec algorithm must be sha1, sha256, sha384, or sha512")
	// ErrIPsecKeyHex rejects a non-hex key.
	ErrIPsecKeyHex = errors.New("ospf: ipsec key must be a hex string")
	// ErrIPsecKeyLength rejects an integrity key whose length does not match the algorithm.
	ErrIPsecKeyLength = errors.New("ospf: ipsec key length does not match the algorithm")
	// ErrIPsecEncAlgo rejects an encryption algorithm outside null/aes128/aes256.
	ErrIPsecEncAlgo = errors.New("ospf: ipsec encryption-algorithm must be null, aes128, or aes256")
	// ErrIPsecEncKeyHex rejects a non-hex encryption key.
	ErrIPsecEncKeyHex = errors.New("ospf: ipsec encryption-key must be a hex string")
	// ErrIPsecEncKeyLength rejects an encryption key whose length does not match the algorithm.
	ErrIPsecEncKeyLength = errors.New("ospf: ipsec encryption-key length does not match the encryption-algorithm")
	// ErrIPsecMissingEncKey rejects ESP confidentiality with no encryption key.
	ErrIPsecMissingEncKey = errors.New("ospf: ipsec esp with an encryption-algorithm requires an encryption-key")
	// ErrIPsecStrayEncKey rejects an encryption key with no (non-null) encryption algorithm.
	ErrIPsecStrayEncKey = errors.New("ospf: ipsec encryption-key set without an encryption-algorithm")
	// ErrIPsecAHConfidentiality rejects AH + confidentiality (RFC 4552 §4: only ESP).
	ErrIPsecAHConfidentiality = errors.New("ospf: ipsec AH cannot provide confidentiality; use esp for encryption (RFC 4552 §4)")
	// ErrIPsecReplayWindow rejects an anti-replay window outside 0 (disabled) or 32..255.
	ErrIPsecReplayWindow = errors.New("ospf: ipsec replay-window must be 0 (anti-replay disabled) or 32..255 packets (RFC 4302 §3.4.3 requires a minimum window of 32)")
	// ErrIPsecMutualExclusion rejects IPsec and RFC 7166 on the same interface.
	ErrIPsecMutualExclusion = errors.New("ospf: an interface cannot configure both ipsec (RFC 4552) and a 7166 authentication key-chain; they are mutually exclusive auth paths")
)

// ipsecInterfaceConfig is the RFC 4552 manual Security Association for one IPv6-family
// interface. Keys stay as hex strings (masked by ze:sensitive in YANG, never logged);
// the installer decodes them once. All fields are comparable so ipsecEqual can diff a
// reconcile with ==.
type ipsecInterfaceConfig struct {
	SPI      uint32 // RFC 4303 §2.1 SPI (256..4294967295)
	Protocol string // ah | esp
	AuthAlgo string // sha1 | sha256 | sha384 | sha512
	AuthKey  string //nolint:gosec // hex integrity key, masked via ze:sensitive, never logged
	EncAlgo  string // "" | null | aes128 | aes256 (ESP confidentiality; empty/null = auth-only)
	EncKey   string //nolint:gosec // hex encryption key, masked via ze:sensitive, never logged

	// ReplayWindow is the anti-replay window in packets: 0 disables the service and
	// 32..255 enables it (RFC 4302 §3.4.3, RFC 4303 §3.4.3). It is held at the parser's
	// own width, so validateIPsecInterface sees the value the operator wrote rather than
	// one already truncated into the uint8 the dataplane takes; buildIPsecSA narrows it
	// once the range is proven.
	ReplayWindow uint64
}

// hasConfidentiality reports whether ESP confidentiality is requested.
func (c ipsecInterfaceConfig) hasConfidentiality() bool {
	return c.EncAlgo != "" && c.EncAlgo != ipsecEncNull
}

// authKeyBytes decodes the (validated) integrity key.
func (c ipsecInterfaceConfig) authKeyBytes() []byte {
	b, _ := hex.DecodeString(c.AuthKey)
	return b
}

// encKeyBytes decodes the (validated) encryption key, or nil when auth-only.
func (c ipsecInterfaceConfig) encKeyBytes() []byte {
	if !c.hasConfidentiality() {
		return nil
	}
	b, _ := hex.DecodeString(c.EncKey)
	return b
}

// parseIPsec reads an `ipsec { ... }` container into an ipsecInterfaceConfig. It does
// not validate: validateIPsecInterface (called with address-family context) owns range,
// enum, hex-length, ESP-only-confidentiality, and RFC 7166 mutual-exclusion checks so a
// v4-family block reports the family error rather than a hex error.
func parseIPsec(m map[string]any) *ipsecInterfaceConfig {
	c := &ipsecInterfaceConfig{}
	if v, ok := configNumber(m["spi"]); ok && v <= 0xFFFFFFFF {
		c.SPI = uint32(v)
	}
	c.Protocol = strings.ToLower(configString(m["protocol"]))
	c.AuthAlgo = strings.ToLower(configString(m["algorithm"]))
	c.AuthKey = configString(m["key"])
	c.EncAlgo = strings.ToLower(configString(m["encryption-algorithm"]))
	c.EncKey = configString(m["encryption-key"])
	if v, ok := configNumber(m["replay-window"]); ok {
		c.ReplayWindow = v
	}
	return c
}

// validateIPsecInterface enforces the RFC 4552 manual-SA constraints for one IPv6-family
// interface: SPI range (RFC 4303 §2.1), protocol/algorithm enums, hex key length vs
// algorithm, anti-replay window range (RFC 4302 §3.4.3), ESP-only confidentiality (§4),
// and RFC 7166 mutual exclusion (AC-6).
func validateIPsecInterface(ic interfaceConfig) error {
	c := ic.IPsec
	if c.SPI < ipsecSPIMin {
		return fmt.Errorf("%w: interface %q spi %d", ErrIPsecSPIReserved, ic.Name, c.SPI)
	}
	switch c.Protocol {
	case ipsecProtoAH, ipsecProtoESP:
	default:
		return fmt.Errorf("%w: interface %q protocol %q", ErrIPsecProtocol, ic.Name, c.Protocol)
	}
	// RFC 4552 §3: ESP authentication MUST be supported and AH MAY be; both carry an
	// HMAC-SHA integrity transform, so an auth algorithm + key is always required.
	authLen, ok := ipsecAuthKeyLen[c.AuthAlgo]
	if !ok {
		return fmt.Errorf("%w: interface %q algorithm %q", ErrIPsecAuthAlgo, ic.Name, c.AuthAlgo)
	}
	authKey, err := hex.DecodeString(c.AuthKey)
	if err != nil {
		return fmt.Errorf("%w: interface %q", ErrIPsecKeyHex, ic.Name)
	}
	if len(authKey) != authLen {
		return fmt.Errorf("%w: interface %q algorithm %s wants %d bytes, got %d", ErrIPsecKeyLength, ic.Name, c.AuthAlgo, authLen, len(authKey))
	}
	if err := validateIPsecReplayWindow(ic); err != nil {
		return err
	}
	if c.Protocol == ipsecProtoAH {
		// RFC 4552 §4: confidentiality MUST use ESP, never AH.
		if c.hasConfidentiality() || c.EncKey != "" {
			return fmt.Errorf("%w: interface %q", ErrIPsecAHConfidentiality, ic.Name)
		}
	} else if err := validateESPConfidentiality(ic); err != nil {
		return err
	}
	// AC-6: RFC 4552 IPsec and a RFC 7166 (Authentication) key chain are mutually exclusive.
	if ic.Authentication.KeyChain != "" {
		return fmt.Errorf("%w: interface %q", ErrIPsecMutualExclusion, ic.Name)
	}
	return nil
}

// validateIPsecReplayWindow enforces the anti-replay window range. RFC 4302 §3.4.3:
// "All AH implementations MUST support the anti-replay service, though its use may be
// enabled or disabled by the receiver on a per-SA basis." RFC 4303 §3.4.3 says the same
// for ESP, so one leaf serves both protocols. Zero is the disabled case the RFC allows
// the receiver to choose; a window that IS enabled starts at the 32-packet minimum the
// same section makes mandatory to support.
func validateIPsecReplayWindow(ic interfaceConfig) error {
	w := ic.IPsec.ReplayWindow
	if w == ipsecReplayWindowOff {
		return nil
	}
	if w >= ipsecReplayWindowMin && w <= ipsecReplayWindowMax {
		return nil
	}
	return fmt.Errorf("%w: interface %q replay-window %d", ErrIPsecReplayWindow, ic.Name, w)
}

// validateESPConfidentiality validates the optional ESP encryption fields (RFC 4552 §4).
func validateESPConfidentiality(ic interfaceConfig) error {
	c := ic.IPsec
	if c.EncAlgo != "" {
		switch c.EncAlgo {
		case ipsecEncNull, ipsecEncAES128, ipsecEncAES256:
		default:
			return fmt.Errorf("%w: interface %q encryption-algorithm %q", ErrIPsecEncAlgo, ic.Name, c.EncAlgo)
		}
	}
	if !c.hasConfidentiality() {
		if c.EncKey != "" {
			return fmt.Errorf("%w: interface %q", ErrIPsecStrayEncKey, ic.Name)
		}
		return nil
	}
	if c.EncKey == "" {
		return fmt.Errorf("%w: interface %q", ErrIPsecMissingEncKey, ic.Name)
	}
	encKey, err := hex.DecodeString(c.EncKey)
	if err != nil {
		return fmt.Errorf("%w: interface %q", ErrIPsecEncKeyHex, ic.Name)
	}
	if want := ipsecEncKeyLen[c.EncAlgo]; len(encKey) != want {
		return fmt.Errorf("%w: interface %q algorithm %s wants %d bytes, got %d", ErrIPsecEncKeyLength, ic.Name, c.EncAlgo, want, len(encKey))
	}
	return nil
}

// ipsecEqual reports whether two IPsec blocks are identical, so a reconcile installs a
// changed SPI/algorithm/key and leaves an unchanged one alone (spec-ospf-ext-16 AC-11).
func ipsecEqual(a, b *ipsecInterfaceConfig) bool {
	if (a == nil) != (b == nil) {
		return false
	}
	if a == nil {
		return true
	}
	return *a == *b
}
