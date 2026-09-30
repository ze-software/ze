// Design: docs/architecture/ospf/ospf-ext-16-ipsec-auth.md -- OSPFv3 manual-key ESP SAs installed into XFRM.
// Related: config_ipsec.go -- validateIPsecInterface, the management-interface checks these tests drive.
// Related: ipsec_install.go -- buildIPsecSA and buildIPsecInterfaceSAs, the states these tests read.
//
// VALIDATES: RFC 4303 (ESP) on the manually keyed OSPFv3 SAs, at the boundary Ze owns: the
// configuration an operator writes, the refusal validateConfig answers, and the SA
// parameters the installer hands the kernel. Each test drives the operator's JSON through
// parseOSPFConfig and validateConfig, the path the plugin's config verify runs.
// PREVENTS: anti-replay enabled on an ESP SA with no integrity, integrity-only ESP being
// refused by the management interface, and an inbound ESP SA whose lookup key is not the
// one the configuration set.

package ospf

import (
	"errors"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	ospfv3transport "github.com/ze-software/ze/internal/plugins/ospf/v3/transport"
)

// rfc4303ValidatedInterface parses one ipsec block through the operator's configuration
// path and returns the validated interface, failing the test when either step refuses it.
func rfc4303ValidatedInterface(t *testing.T, ipsecLeaves string) interfaceConfig {
	t.Helper()
	cfg, err := parseOSPFConfig(ospfSec(v6IPsecCfg(ipsecLeaves, "")), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig(%s): %v", ipsecLeaves, err)
	}
	if err := validateConfig(cfg); err != nil {
		t.Fatalf("validateConfig(%s): %v", ipsecLeaves, err)
	}
	if cfg.V6 == nil || len(cfg.V6.Interfaces) != 1 || cfg.V6.Interfaces[0].IPsec == nil {
		t.Fatalf("config %s parsed no IPv6 interface with an ipsec block", ipsecLeaves)
	}
	return cfg.V6.Interfaces[0]
}

// rfc4303Refusal parses one ipsec block and returns what validateConfig answers for it.
func rfc4303Refusal(t *testing.T, ipsecLeaves string) error {
	t.Helper()
	cfg, err := parseOSPFConfig(ospfSec(v6IPsecCfg(ipsecLeaves, "")), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig(%s): %v", ipsecLeaves, err)
	}
	return validateConfig(cfg)
}

// rfc4303Installed installs one validated interface through the installer over the fake
// dataplane and returns the SAs it handed the kernel.
func rfc4303Installed(t *testing.T, ic interfaceConfig) []dataplane.SAParams {
	t.Helper()
	inst, fake := testInstaller(t, netip.MustParseAddr("fe80::1"))
	inst.setConfig([]interfaceConfig{ic})
	inst.onInterfaceUp(testIfIndex, ic.Name)
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if len(fake.sas) == 0 {
		t.Fatalf("interface %q installed no SA", ic.Name)
	}
	return append([]dataplane.SAParams(nil), fake.sas...)
}

// TestRFC4303ESPReplayWindowCarriesIntegrity checks that an ESP SA with anti-replay enabled
// carries the ESP integrity service. Method: configure replay-window 64 on esp interfaces
// with a null cipher and with aes128, install them, and read every SA the kernel receives.
//
// RFC requirement: RFC4303-3.4.3-2 positive -- RFC 4303 Section 3.4.3: "This service MUST
// NOT be enabled unless the ESP integrity service also is enabled for the SA". Every OSPFv3
// ESP SA installed with a non-zero replay window carries an integrity algorithm and its key.
func TestRFC4303ESPReplayWindowCarriesIntegrity(t *testing.T) {
	configs := []string{
		`"protocol":"esp","spi":256,"replay-window":64,"algorithm":"sha256","key":"` + hexKey(32) + `","encryption-algorithm":"null"`,
		`"protocol":"esp","spi":256,"replay-window":64,"algorithm":"sha256","key":"` + hexKey(32) +
			`","encryption-algorithm":"aes128","encryption-key":"` + hexKey(16) + `"`,
	}
	for _, leaves := range configs {
		for _, sa := range rfc4303Installed(t, rfc4303ValidatedInterface(t, leaves)) {
			if sa.ReplayWin != 64 {
				t.Errorf("config %s: SA to %v ReplayWin = %d, want 64", leaves, sa.Dst, sa.ReplayWin)
			}
			if sa.AuthAlgo != "sha256" {
				t.Errorf("config %s: anti-replay SA to %v carries integrity algorithm %q, want sha256", leaves, sa.Dst, sa.AuthAlgo)
			}
			if len(sa.AuthKey) != 32 {
				t.Errorf("config %s: anti-replay SA to %v carries a %d-octet integrity key, want 32", leaves, sa.Dst, len(sa.AuthKey))
			}
		}
	}
}

// TestRFC4303ESPReplayWindowWithoutIntegrityIsRefused checks the refusal half. Method: an
// esp interface that enables replay-window 64 and names no integrity algorithm, with no
// cipher, the null cipher and aes128, is handed to validateConfig.
//
// RFC requirement: RFC4303-3.4.3-2 negative -- a configuration that would enable anti-replay
// on an ESP SA without the integrity service is refused with ErrIPsecAuthAlgo, so no such SA
// reaches the installer.
func TestRFC4303ESPReplayWindowWithoutIntegrityIsRefused(t *testing.T) {
	configs := []string{
		`"protocol":"esp","spi":256,"replay-window":64`,
		`"protocol":"esp","spi":256,"replay-window":64,"encryption-algorithm":"null"`,
		`"protocol":"esp","spi":256,"replay-window":64,"encryption-algorithm":"aes128","encryption-key":"` + hexKey(16) + `"`,
	}
	for _, leaves := range configs {
		if err := rfc4303Refusal(t, leaves); !errors.Is(err, ErrIPsecAuthAlgo) {
			t.Errorf("validateConfig(%s) = %v, want ErrIPsecAuthAlgo", leaves, err)
		}
	}
}

// TestRFC4303IntegrityOnlyESPIsConfigurable checks that the management interface accepts
// integrity-only ESP in every form an operator can write it. Method: for every integrity
// algorithm the manual-SA surface offers, configure esp with the encryption leaf omitted and
// with it set to null, then install it and read the SA.
//
// RFC requirement: RFC4303-1-1 negative -- RFC 4303 Section 1: integrity-only ESP "MUST be
// configurable via management interfaces". The inputs are pushed toward a refusal (no
// cipher at all, the explicit null cipher, each integrity algorithm in turn) and each is
// accepted and installed as ESP (protocol 50, not AH) with the null cipher and no cipher
// key, so a validator requiring a cipher for ESP, or accepting integrity-only ESP for one
// algorithm only, fails this test.
func TestRFC4303IntegrityOnlyESPIsConfigurable(t *testing.T) {
	for algo, keyLen := range ipsecAuthKeyLen {
		for _, cipher := range []string{"", `,"encryption-algorithm":"null"`} {
			leaves := `"protocol":"esp","spi":256,"algorithm":"` + algo + `","key":"` + hexKey(keyLen) + `"` + cipher
			for _, sa := range rfc4303Installed(t, rfc4303ValidatedInterface(t, leaves)) {
				if sa.Proto != dataplane.ProtoESP {
					t.Errorf("config %s: SA proto = %d, want ESP (50)", leaves, sa.Proto)
				}
				if sa.EncAlgo != ipsecEncNull || len(sa.EncKey) != 0 {
					t.Errorf("config %s: SA cipher = %q with a %d-octet key, want null and none", leaves, sa.EncAlgo, len(sa.EncKey))
				}
				if sa.AuthAlgo != algo || len(sa.AuthKey) != keyLen {
					t.Errorf("config %s: SA integrity = %q with a %d-octet key, want %s and %d", leaves, sa.AuthAlgo, len(sa.AuthKey), algo, keyLen)
				}
			}
		}
	}
}

// TestRFC4303InboundESPSAKeyedOnConfiguredDestination checks the SA identifier the manual
// configuration sets for the inbound lookup. Method: install an esp interface whose local
// address is fe80::1 and read the SPI, protocol, destination and source of each state.
//
// RFC requirement: RFC4303-2.1-2 positive -- RFC 4303 Section 2.1: "The indication of
// whether source and destination address matching is required to map inbound IPsec traffic
// to SAs MUST be set either as a side effect of manual SA configuration or via negotiation
// using an SA management protocol". The manual configuration sets it: every state carries
// the configured SPI, protocol 50 and a destination the configuration names (the router's
// own address and the two OSPFv3 groups), with the source unspecified, so the inbound lookup
// matches the destination and not the source.
func TestRFC4303InboundESPSAKeyedOnConfiguredDestination(t *testing.T) {
	ic := rfc4303ValidatedInterface(t, `"protocol":"esp","spi":4660,"algorithm":"sha256","key":"`+hexKey(32)+`"`)
	want := map[netip.Addr]bool{
		netip.MustParseAddr("fe80::1"): false,
		ospfv3transport.AllSPFRouters:  false,
		ospfv3transport.AllDRouters:    false,
	}
	for _, sa := range rfc4303Installed(t, ic) {
		if sa.SPI != 4660 || sa.Proto != dataplane.ProtoESP {
			t.Errorf("state to %v: SPI %d proto %d, want the configured SPI 4660 and ESP (50)", sa.Dst, sa.SPI, sa.Proto)
		}
		if !sa.Src.IsUnspecified() {
			t.Errorf("state to %v: source %v, want unspecified: source matching is not part of the configured indication", sa.Dst, sa.Src)
		}
		dst, ok := netip.AddrFromSlice(sa.Dst)
		if !ok {
			t.Fatalf("state destination %v is not an address", sa.Dst)
		}
		seen, named := want[dst.Unmap()]
		if !named {
			t.Errorf("state keyed on %v, a destination the configuration does not name", dst)
			continue
		}
		if seen {
			t.Errorf("two states keyed on %v with SPI %d", dst, sa.SPI)
		}
		want[dst.Unmap()] = true
	}
	for dst, seen := range want {
		if !seen {
			t.Errorf("no state keyed on the configured destination %v", dst)
		}
	}
}
