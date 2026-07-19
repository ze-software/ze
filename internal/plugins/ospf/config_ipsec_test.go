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
	if want, _ := hex.DecodeString(key); len(ips.authKeyBytes()) != len(want) {
		t.Errorf("authKeyBytes len = %d, want %d", len(ips.authKeyBytes()), len(want))
	}
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
		if c.wantErr && !errors.Is(err, ErrIPsecSPIReserved) {
			t.Errorf("spi %d: err = %v, want ErrIPsecSPIReserved", c.spi, err)
		}
		if !c.wantErr && err != nil {
			t.Errorf("spi %d: unexpected err %v", c.spi, err)
		}
	}
}

func TestIPsecESPConfidentialityValid(t *testing.T) {
	// AC-3: ESP with a valid encryption-algorithm + matching key parses and validates.
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
