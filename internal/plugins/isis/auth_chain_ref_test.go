// Design: docs/architecture/isis/isis-10-auth.md -- IS-IS authentication key store.
// Related: config.go -- validateConfig, the validator these drive
// Related: show.go -- the authenticated column of `show isis interface`
//
// VALIDATES: a config naming no chain, or a declared one, is accepted, and a
// circuit whose chain did not resolve is never reported authenticated. A
// dangling auth-key-chain is refused by the YANG leafref before this plugin
// parses the section: test/parse/reference-isis-key-chain-missing.ci.
package isis

import (
	"testing"
)

// TestISISNoKeyChainIsAccepted: a config that names no chain asks for no
// authentication, which stays legitimate. A declared chain is accepted too, so
// the leafref refusal of a dangling name is about that name and nothing else.
func TestISISNoKeyChainIsAccepted(t *testing.T) {
	cases := map[string]string{
		"no authentication": `{"isis":{"net":"49.0001.0000.0000.0001.00","interfaces":{"interface":{"eth0":{"name":"eth0"}}}}}`,
		"declared chain": `{"isis":{"net":"49.0001.0000.0000.0001.00",` +
			`"key-chains":{"area-key":{"name":"area-key","key":{"1":{"key-id":"1","algorithm":"hmac-sha-256","secret":"s3cr3t"}}}},` +
			`"level-1":{"auth-key-chain":"area-key"},` +
			`"interfaces":{"interface":{"eth0":{"name":"eth0","level-1":{"auth-key-chain":"area-key"}}}}}}`,
	}
	for name, data := range cases {
		t.Run(name, func(t *testing.T) {
			cfg, err := parseISISConfig(sec(data))
			if err != nil {
				t.Fatalf("parseISISConfig: %v", err)
			}
			if err := validateConfig(cfg); err != nil {
				t.Fatalf("validateConfig = %v, want nil", err)
			}
		})
	}
}

// TestISISUnresolvedChainIsNotReportedAuthenticated: `show isis interface` reads
// the resolved key store, so a circuit whose chain name resolved to nothing is
// reported unauthenticated, which is what it is on the wire. Reporting the
// configured NAME told the operator the opposite and hid the typo.
func TestISISUnresolvedChainIsNotReportedAuthenticated(t *testing.T) {
	const chains = `"key-chains":{"area-key":{"name":"area-key","key":{"1":{"key-id":"1","algorithm":"hmac-sha-256","secret":"s3cr3t"}}}},`
	cases := []struct {
		name string
		data string
		want bool
	}{
		{
			name: "dangling chain",
			data: `{"isis":{"net":"49.0001.0000.0000.0001.00",` + chains +
				`"interfaces":{"interface":{"eth0":{"name":"eth0","circuit-type":"point-to-point","level-1":{"auth-key-chain":"aera-key"}}}}}}`,
			want: false,
		},
		{
			name: "resolved chain",
			data: `{"isis":{"net":"49.0001.0000.0000.0001.00",` + chains +
				`"interfaces":{"interface":{"eth0":{"name":"eth0","circuit-type":"point-to-point","level-1":{"auth-key-chain":"area-key"}}}}}}`,
			want: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			eng := startedEngine(t, c.data)
			defer eng.shutdown()
			rows := eng.interfaceSnapshot()
			if len(rows) != 1 {
				t.Fatalf("interface rows = %d, want 1: %+v", len(rows), rows)
			}
			row, ok := rows[0].(interfaceRow)
			if !ok {
				t.Fatalf("interface row is %T, want interfaceRow", rows[0])
			}
			if row.Authenticated != c.want {
				t.Errorf("eth0 authenticated = %v, want %v", row.Authenticated, c.want)
			}
		})
	}
}
