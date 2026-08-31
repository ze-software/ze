// VALIDATES: spec-ospf-ext-16 AC-1/AC-4/AC-5/AC-6 + key-length boundaries -- the
// RFC 4552 IPsec config surface parses under the IPv6 family and rejects every
// invalid combination (IPv4-family, AH+confidentiality, key length, 7166 clash).
// PREVENTS: a malformed IPsec block reaching the kernel installer, and IPsec being
// silently accepted on an OSPFv2 interface.

package ospf

import (
	"encoding/hex"
	"errors"
	"strings"
	"testing"
)

// v6IPsecCfg builds an `ospf { address-family ipv6 { interfaces { interface eth1 {
// area 0; ipsec {...} } } } }` config with the given ipsec leaves and optional extra
// interface leaves (e.g. an authentication key-chain).
func v6IPsecCfg(ipsecLeaves, extraIface string) string {
	return `{"ospf":{"router-id":"10.0.0.1","address-family":{"ipv6":{"areas":{"area":{"0":{"area-id":"0"}}},` +
		`"interfaces":{"interface":{"eth1":{"area":"0"` + extraIface + `,"ipsec":{` + ipsecLeaves + `}}}}}}}}`
}

func hexKey(nBytes int) string { return strings.Repeat("ab", nBytes) }

func TestParseOSPFIPsecConfig(t *testing.T) {
	key := hexKey(32) // sha256 = 32 bytes
	cfg, err := parseOSPFConfig(ospfSec(v6IPsecCfg(`"protocol":"esp","spi":256,"algorithm":"sha256","key":"`+key+`"`, "")), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig: %v", err)
	}
	if cfg.V6 == nil || len(cfg.V6.Interfaces) != 1 {
		t.Fatalf("v6 interface not parsed: %+v", cfg.V6)
	}
	ips := cfg.V6.Interfaces[0].IPsec
	if ips == nil {
		t.Fatal("interface IPsec block not parsed")
	}
	if ips.SPI != 256 || ips.Protocol != "esp" || ips.AuthAlgo != "sha256" {
		t.Errorf("parsed IPsec = %+v, want spi=256 esp sha256", ips)
	}
	// RFC requirement: RFC4552-12-1 positive -- a cryptographic/authentication key configured as a
	// hexadecimal string is accepted and decoded to bytes (authKeyBytes -> hex.DecodeString,
	// config_ipsec.go:68-70), so configuring keys in hexadecimal format is supported.
	if want, _ := hex.DecodeString(key); len(ips.authKeyBytes()) != len(want) {
		t.Errorf("authKeyBytes len = %d, want %d", len(ips.authKeyBytes()), len(want))
	}
	// RFC requirement: RFC4552-3-1 positive -- a well-formed ESP interface with an HMAC-SHA integrity
	// algorithm+key validates, so OSPFv3 authentication is supported (validateIPsecInterface always
	// requires an auth algorithm+key, config_ipsec.go:114-124).
	if err := validateConfig(cfg); err != nil {
		t.Fatalf("validateConfig(well-formed ESP): %v", err)
	}
}

func TestIPsecRejectedUnderV4(t *testing.T) {
	// An IPsec block on an IPv4-family interface must be rejected (RFC 4552 §5).
	data := `{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth0":{"area":"0","ipsec":{"protocol":"esp","spi":256,"algorithm":"sha256","key":"` + hexKey(32) + `"}}}}}}`
	cfg, err := parseOSPFConfig(ospfSec(data), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig: %v", err)
	}
	if err := validateConfig(cfg); !errors.Is(err, ErrIPsecIPv4Family) {
		t.Fatalf("validateConfig(v4 ipsec) = %v, want ErrIPsecIPv4Family", err)
	}
}

func TestIPsecAnd7166MutuallyExclusive(t *testing.T) {
	cfg, err := parseOSPFConfig(ospfSec(v6IPsecCfg(
		`"protocol":"esp","spi":256,"algorithm":"sha256","key":"`+hexKey(32)+`"`,
		`,"authentication":{"key-chain":"kc1"}`)), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig: %v", err)
	}
	if err := validateConfig(cfg); !errors.Is(err, ErrIPsecMutualExclusion) {
		t.Fatalf("validateConfig(ipsec+7166) = %v, want ErrIPsecMutualExclusion", err)
	}
}

func TestIPsecAHWithEncryptionRejected(t *testing.T) {
	// RFC 4552 §4: AH must not provide confidentiality.
	cfg, err := parseOSPFConfig(ospfSec(v6IPsecCfg(
		`"protocol":"ah","spi":256,"algorithm":"sha256","key":"`+hexKey(32)+`","encryption-algorithm":"aes256","encryption-key":"`+hexKey(32)+`"`, "")), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig: %v", err)
	}
	// RFC requirement: RFC4552-4-2 negative -- confidentiality on AH is rejected with
	// ErrIPsecAHConfidentiality (config_ipsec.go:125-129), enforcing that when confidentiality is
	// provided ESP MUST be used, never AH.
	if err := validateConfig(cfg); !errors.Is(err, ErrIPsecAHConfidentiality) {
		t.Fatalf("validateConfig(ah+enc) = %v, want ErrIPsecAHConfidentiality", err)
	}
}

func TestIPsecESPRequiresIntegrity(t *testing.T) {
	// RFC 4301 §4.2: an ESP SA MUST NOT be instantiated with NULL encryption AND no
	// integrity. Ze's config validator always requires an integrity algorithm+key for ESP
	// (config_ipsec.go:114-124), so the forbidden NULL-enc + no-integrity combination is
	// rejected, and NULL encryption is allowed only when integrity is present.

	// RFC requirement: RFC4301-4.2-1 negative -- an esp interface with NULL encryption and NO
	// integrity algorithm is rejected with ErrIPsecAuthAlgo, so a null-cipher/no-auth ESP SA
	// can never be instantiated (validateIPsecInterface, config_ipsec.go:114-124).
	// RFC requirement: RFC4552-3-1 negative -- the same rejection proves OSPFv3 authentication cannot
	// be silently omitted: an interface with no valid integrity algorithm is refused with
	// ErrIPsecAuthAlgo, so authentication support is mandatory (config_ipsec.go:114-116).
	// RFC requirement: RFC4303-3.2-1 negative -- RFC 4303 Section 3.2: "although both
	// confidentiality and integrity are optional, at least one of these services MUST be
	// selected, hence both algorithms MUST NOT be simultaneously NULL". The refusal is the
	// guard: a null cipher with no integrity algorithm never reaches an installed SA.
	// RFC requirement: RFC4303-1-1 negative -- the integrity-only ESP service Ze offers is
	// integrity BEARING. A configuration that names the null cipher and omits integrity does
	// not select it; it is refused rather than installed as an unauthenticated ESP SA.
	cfg, err := parseOSPFConfig(ospfSec(v6IPsecCfg(
		`"protocol":"esp","spi":256,"encryption-algorithm":"null"`, "")), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig (null enc, no integrity): %v", err)
	}
	if err := validateConfig(cfg); !errors.Is(err, ErrIPsecAuthAlgo) {
		t.Fatalf("validateConfig(esp null-enc no-integrity) = %v, want ErrIPsecAuthAlgo", err)
	}

	// RFC requirement: RFC4301-4.2-1 positive -- an esp interface with NULL encryption but WITH
	// a valid integrity algorithm+key IS accepted: NULL encryption is legal precisely because
	// integrity is present, the boundary the guard enforces (config_ipsec.go:114-166).
	// RFC requirement: RFC4303-1-1 positive -- RFC 4303 Section 1: "Integrity-only ESP MUST be
	// offered as a service selection option ... and MUST be configurable via management
	// interfaces". This configuration IS that selection, made through Ze's management
	// interface, and it is accepted.
	// RFC requirement: RFC4303-3.2-1 positive -- the accepted combination has a NULL cipher and
	// a named integrity algorithm, so exactly one service is selected and the two algorithms
	// are not simultaneously NULL.
	cfg, err = parseOSPFConfig(ospfSec(v6IPsecCfg(
		`"protocol":"esp","spi":256,"algorithm":"sha256","key":"`+hexKey(32)+`","encryption-algorithm":"null"`, "")), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig (null enc, sha256): %v", err)
	}
	if err := validateConfig(cfg); err != nil {
		t.Fatalf("validateConfig(esp null-enc with integrity): %v", err)
	}
}

func TestIPsecKeyLengthValidation(t *testing.T) {
	// Auth key length must match the algorithm.
	cases := []struct {
		algo    string
		nBytes  int
		wantErr bool
	}{
		{"sha1", 20, false},
		{"sha256", 32, false},
		{"sha384", 48, false},
		{"sha512", 64, false},
		{"sha256", 16, true}, // too short
		{"sha256", 64, true}, // too long
	}
	for _, c := range cases {
		cfg, err := parseOSPFConfig(ospfSec(v6IPsecCfg(
			`"protocol":"esp","spi":256,"algorithm":"`+c.algo+`","key":"`+hexKey(c.nBytes)+`"`, "")), nil)
		if err != nil {
			t.Fatalf("%s/%dB parse: %v", c.algo, c.nBytes, err)
		}
		err = validateConfig(cfg)
		if c.wantErr && !errors.Is(err, ErrIPsecKeyLength) {
			t.Errorf("%s/%dB: err = %v, want ErrIPsecKeyLength", c.algo, c.nBytes, err)
		}
		if !c.wantErr && err != nil {
			t.Errorf("%s/%dB: unexpected err %v", c.algo, c.nBytes, err)
		}
	}
}

func TestIPsecSPIBoundary(t *testing.T) {
	// RFC 4303 §2.1 reserves 0..255; 256 is the last valid boundary.
	for _, c := range []struct {
		spi     int
		wantErr bool
	}{{255, true}, {256, false}} {
		cfg, err := parseOSPFConfig(ospfSec(v6IPsecCfg(
			`"protocol":"esp","spi":`+itoa(c.spi)+`,"algorithm":"sha256","key":"`+hexKey(32)+`"`, "")), nil)
		if err != nil {
			t.Fatalf("spi %d parse: %v", c.spi, err)
		}
		err = validateConfig(cfg)
		// RFC requirement: RFC4303-2.1-1 negative -- an SPI in the reserved 0..255 range (spi=255)
		// is rejected with ErrIPsecSPIReserved, so a reserved SPI can never be installed and thus
		// never placed on the wire (validateIPsecInterface, config_ipsec.go:104-106).
		if c.wantErr && !errors.Is(err, ErrIPsecSPIReserved) {
			t.Errorf("spi %d: err = %v, want ErrIPsecSPIReserved", c.spi, err)
		}
		// RFC requirement: RFC4303-2.1-1 positive -- the first non-reserved SPI (spi=256), the
		// boundary just above the reserved 0..255 range, is accepted, so a valid ESP SPI passes
		// validation and can be installed (config_ipsec.go:104-106).
		if !c.wantErr && err != nil {
			t.Errorf("spi %d: unexpected err %v", c.spi, err)
		}
	}
}

func TestIPsecESPConfidentialityValid(t *testing.T) {
	// AC-3: ESP with a valid encryption-algorithm + matching key parses and validates.
	// RFC requirement: RFC4552-4-2 positive -- confidentiality is provided via ESP: an esp interface
	// with a valid encryption-algorithm+key validates and installs an ESP SA (config_ipsec.go:130-166).
	// RFC requirement: RFC4552-6-6 positive -- the accepted encryption algorithm aes256 is a block
	// cipher; the encryption-algorithm enum admits only null/aes128/aes256 (config_ipsec.go:143-148),
	// so no stream cipher is selectable.
	cfg, err := parseOSPFConfig(ospfSec(v6IPsecCfg(
		`"protocol":"esp","spi":256,"algorithm":"sha256","key":"`+hexKey(32)+`","encryption-algorithm":"aes256","encryption-key":"`+hexKey(32)+`"`, "")), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig: %v", err)
	}
	if err := validateConfig(cfg); err != nil {
		t.Fatalf("validateConfig(esp+enc): %v", err)
	}
	ips := cfg.V6.Interfaces[0].IPsec
	if !ips.hasConfidentiality() || len(ips.encKeyBytes()) != 32 {
		t.Errorf("encKeyBytes len = %d, want 32", len(ips.encKeyBytes()))
	}
}

func TestIPsecStreamCipherRejected(t *testing.T) {
	// RFC 4552 §6: stream ciphers MUST NOT be selectable as the OSPFv3 encryption algorithm.
	// RFC requirement: RFC4552-6-6 negative -- an encryption-algorithm outside the block-cipher
	// enum (here the stream cipher "rc4") is rejected with ErrIPsecEncAlgo, so the user cannot
	// choose a stream cipher (validateESPConfidentiality, config_ipsec.go:143-148).
	cfg, err := parseOSPFConfig(ospfSec(v6IPsecCfg(
		`"protocol":"esp","spi":256,"algorithm":"sha256","key":"`+hexKey(32)+`","encryption-algorithm":"rc4","encryption-key":"`+hexKey(16)+`"`, "")), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig (rc4): %v", err)
	}
	if err := validateConfig(cfg); !errors.Is(err, ErrIPsecEncAlgo) {
		t.Fatalf("validateConfig(rc4 stream cipher) = %v, want ErrIPsecEncAlgo", err)
	}
}

func TestIPsecNonHexKeyRejected(t *testing.T) {
	// RFC requirement: RFC4552-12-1 negative -- a key that is not valid hexadecimal is rejected with
	// ErrIPsecKeyHex (hex.DecodeString failure, config_ipsec.go:118-120), so the hexadecimal key
	// format is enforced rather than silently mis-decoded.
	cfg, err := parseOSPFConfig(ospfSec(v6IPsecCfg(
		`"protocol":"esp","spi":256,"algorithm":"sha256","key":"`+strings.Repeat("zz", 32)+`"`, "")), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig (non-hex key): %v", err)
	}
	if err := validateConfig(cfg); !errors.Is(err, ErrIPsecKeyHex) {
		t.Fatalf("validateConfig(non-hex key) = %v, want ErrIPsecKeyHex", err)
	}
}
