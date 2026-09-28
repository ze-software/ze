// Design: docs/architecture/ospf/ospf-4-component-config.md -- the OSPFv3 manual IPsec SA an interface installs

package ospf

import (
	"errors"
	"testing"
)

// TestIPsecSPIZeroRefused proves the manual-SA validator refuses SPI 0 itself,
// not only the top of the reserved range.
//
// Goal: TestIPsecSPIBoundary refuses 255 and accepts 256, so a guard that
// admitted 0 alone passes it. RFC 4303 Section 2.1 forbids exactly 0 on the
// wire, and the SPI configured here is the SPI the installed SA sends. Method:
// an ESP interface with spi 0 MUST fail validateConfig with
// ErrIPsecSPIReserved, and the same interface with the largest SPI,
// 4294967295, MUST validate.
//
// RFC requirement: RFC4303-2.1-1 negative -- an OSPFv3 ESP interface configured with spi 0 fails validateConfig with ErrIPsecSPIReserved, so no SA with SPI 0 is installed.
// RFC requirement: RFC4303-2.1-1 positive -- the same interface with the non-zero spi 4294967295 validates.
func TestIPsecSPIZeroRefused(t *testing.T) {
	for _, c := range []struct {
		spi     string
		wantErr bool
	}{{"0", true}, {"4294967295", false}} {
		cfg, err := parseOSPFConfig(ospfSec(v6IPsecCfg(
			`"protocol":"esp","spi":`+c.spi+`,"algorithm":"sha256","key":"`+hexKey(32)+`"`, "")), nil)
		if err != nil {
			t.Fatalf("spi %s parse: %v", c.spi, err)
		}
		err = validateConfig(cfg)
		if c.wantErr && !errors.Is(err, ErrIPsecSPIReserved) {
			t.Errorf("spi %s: err = %v, want ErrIPsecSPIReserved", c.spi, err)
		}
		if !c.wantErr && err != nil {
			t.Errorf("spi %s: unexpected err %v", c.spi, err)
		}
	}
}
