// Design: docs/architecture/ospf/ospf-4-component-config.md -- OSPFv2 config resolution.
// Related: config.go -- validateConfig, the validator these drive
// Related: auth_keystore.go -- (*authStore).configure, the store the reference feeds
//
// VALIDATES: a config naming no key chain, or a declared one, is accepted at
// the config entry point an operator reaches (parse then validate). A dangling
// name is refused by the YANG leafref on the key-chain leaves at commit, which
// test/parse/reference-ospf-key-chain-missing.ci proves.
package ospf

import (
	"testing"
)

// TestOSPFNoKeyChainIsAccepted: an interface that names no chain asks for no
// authentication, which stays legitimate. A declared chain is accepted too.
func TestOSPFNoKeyChainIsAccepted(t *testing.T) {
	const chains = `"key-chains":{"area-key":{"name":"area-key","key":{"1":{"key-id":"1","algorithm":"hmac-sha-256","secret":"s3cr3t"}}}}`
	cases := map[string]string{
		"no authentication": `{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0":{"area-id":"0"}}},` +
			`"interfaces":{"interface":{"eth0":{"name":"eth0","area":"0"}}}}}`,
		"declared chain": `{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0":{"area-id":"0","authentication":{"key-chain":"area-key"}}}},` +
			`"interfaces":{"interface":{"eth0":{"name":"eth0","area":"0","authentication":{"mode":"md5","key-chain":"area-key"}}}},` + chains + `}}`,
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			cfg, err := parseOSPFConfig(ospfSec(data), nil)
			if err != nil {
				t.Fatalf("parseOSPFConfig: %v", err)
			}
			if err := validateConfig(cfg); err != nil {
				t.Fatalf("validateConfig = %v, want nil", err)
			}
		})
	}
}
