// Design: docs/architecture/ospf/ospf-4-component-config.md -- OSPFv2 config resolution.
// Related: config.go -- validateKeyChainRefs, the validator these drive
// Related: auth_keystore.go -- (*authStore).configure, the store the reference feeds
//
// VALIDATES: an authentication key-chain reference naming no declared chain is
// refused at the config entry point an operator reaches (parse then validate),
// the error names the chain, and a config naming no chain is still accepted.
package ospf

import (
	"errors"
	"strings"
	"testing"
)

// TestOSPFDanglingKeyChainIsRefused: an area or interface reference that names no
// key-chains entry is refused, and the error names the chain the operator typed.
// Before this check the dangling name took the same branch in configure as an
// unset one, so the interface accepted every packet unsigned and nothing said so.
func TestOSPFDanglingKeyChainIsRefused(t *testing.T) {
	const chains = `"key-chains":{"area-key":{"name":"area-key","key":{"1":{"key-id":"1","algorithm":"hmac-sha-256","secret":"s3cr3t"}}}}`
	cases := []struct {
		name string
		data string
		want string
	}{
		{
			name: "interface",
			data: `{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0":{"area-id":"0"}}},` +
				`"interfaces":{"interface":{"eth0":{"name":"eth0","area":"0","authentication":{"mode":"md5","key-chain":"aera-key"}}}},` + chains + `}}`,
			want: "aera-key",
		},
		{
			name: "area",
			data: `{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0":{"area-id":"0","authentication":{"key-chain":"no-such-chain"}}}},` +
				`"interfaces":{"interface":{"eth0":{"name":"eth0","area":"0"}}},` + chains + `}}`,
			want: "no-such-chain",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg, err := parseOSPFConfig(ospfSec(c.data), nil)
			if err != nil {
				t.Fatalf("parseOSPFConfig: %v", err)
			}
			err = validateConfig(cfg)
			if !errors.Is(err, ErrUnknownKeyChain) {
				t.Fatalf("validateConfig = %v, want ErrUnknownKeyChain", err)
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("error %q does not name the chain %q", err, c.want)
			}
		})
	}
}

// TestOSPFNoKeyChainIsAccepted: an interface that names no chain asks for no
// authentication, which stays legitimate. A declared chain is accepted too, so
// the refusal above is about the dangling name and nothing else.
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
