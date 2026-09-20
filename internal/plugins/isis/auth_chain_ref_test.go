// Design: docs/architecture/isis/isis-10-auth.md -- IS-IS authentication key store.
// Related: config.go -- validateKeyChainRefs, the validator these drive
// Related: show.go -- the authenticated column of `show isis interface`
//
// VALIDATES: an auth-key-chain leaf naming no declared chain is refused at the
// config entry point an operator reaches (parse then validate), the error names
// the chain, a config naming no chain is still accepted, and a circuit whose
// chain did not resolve is never reported authenticated.
package isis

import (
	"errors"
	"strings"
	"testing"
)

// TestISISDanglingKeyChainIsRefused: every auth-key-chain leaf that names no
// key-chains entry is refused, and the error names the chain the operator typed.
// Before this check a dangling name and an unset one took the same branch in
// newKeyStore, so the circuit ran unauthenticated and nothing said so.
func TestISISDanglingKeyChainIsRefused(t *testing.T) {
	const chains = `"key-chains":{"area-key":{"name":"area-key","key":{"1":{"key-id":"1","algorithm":"hmac-sha-256","secret":"s3cr3t"}}}},`
	cases := []struct {
		name string
		data string
		want string
	}{
		{
			name: "interface level-1",
			data: `{"isis":{"net":"49.0001.0000.0000.0001.00",` + chains +
				`"interfaces":{"interface":{"eth0":{"name":"eth0","level-1":{"auth-key-chain":"aera-key"}}}}}}`,
			want: "aera-key",
		},
		{
			name: "interface level-2",
			data: `{"isis":{"net":"49.0001.0000.0000.0001.00",` + chains +
				`"interfaces":{"interface":{"eth0":{"name":"eth0","level-2":{"auth-key-chain":"domain-key"}}}}}}`,
			want: "domain-key",
		},
		{
			name: "instance level-1",
			data: `{"isis":{"net":"49.0001.0000.0000.0001.00",` + chains +
				`"level-1":{"auth-key-chain":"no-such-chain"}}}`,
			want: "no-such-chain",
		},
		{
			name: "instance level-2",
			data: `{"isis":{"net":"49.0001.0000.0000.0001.00",` + chains +
				`"level-2":{"auth-key-chain":"domain-key"}}}`,
			want: "domain-key",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			cfg, err := parseISISConfig(sec(c.data))
			if err != nil {
				t.Fatalf("parseISISConfig: %v", err)
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

// TestISISNoKeyChainIsAccepted: a config that names no chain asks for no
// authentication, which stays legitimate. A declared chain is accepted too, so
// the refusal above is about the dangling name and nothing else.
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
