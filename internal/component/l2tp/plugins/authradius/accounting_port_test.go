// Design: docs/guide/l2tp.md -- the accounting-port leaf of a RADIUS server entry
// Related: config.go -- parseConfigFromTree builds the accounting server list
// Related: register.go -- activateRadiusConfig gives accounting its own client

package l2tpauthradius

import (
	"net"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/l2tp"
	"github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/l2tp/plugins/authradius/yang"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/radius"
)

// parseRADIUSConfigText runs operator config text through the production
// pipeline: YANG tree, Tree.ToPluginMap, then the plugin's own parser.
func parseRADIUSConfigText(t *testing.T, server string) *radiusConfig {
	t.Helper()
	text := "l2tp {\n    enabled true\n    auth {\n        radius {\n" +
		"            timeout 1\n            retries 1\n" +
		"            server primary {\n" + server + "            }\n" +
		"        }\n    }\n}\n"
	tree, err := config.ParseTreeWithYANG(text, map[string]string{
		"l2tp-auth-radius": yang.ZeL2TPAuthRadiusConfYANG,
	})
	if err != nil {
		t.Fatalf("parse tree: %v", err)
	}
	cfg, err := parseConfigFromTree(tree.ToPluginMap())
	if err != nil {
		t.Fatalf("parseConfigFromTree: %v", err)
	}
	return cfg
}

// VALIDATES: a server entry that sets only `port` sends its accounting to 1813,
// the port RFC 2866 Section 3 names, and its Access-Request to `port`.
// PREVENTS: the one-port entry that sent Accounting-Request to the
// authentication port, which a standard server refuses ("Invalid packet code 4
// sent to authentication port", FreeRADIUS 3.2.7, 2026-10-20).
func TestAccountingPortDefaultsTo1813(t *testing.T) {
	cfg := parseRADIUSConfigText(t, "                address 192.0.2.1\n"+
		"                port 1645\n                shared-key testing123\n")
	if got := cfg.Servers[0].Address; got != "192.0.2.1:1645" {
		t.Errorf("Access-Request destination = %q, want 192.0.2.1:1645", got)
	}
	if len(cfg.AccountingServers) != 1 {
		t.Fatalf("accounting servers: got %d, want 1", len(cfg.AccountingServers))
	}
	if got := cfg.AccountingServers[0].Address; got != "192.0.2.1:1813" {
		t.Errorf("Accounting-Request destination = %q, want 192.0.2.1:1813", got)
	}
	if string(cfg.AccountingServers[0].SharedKey) != "testing123" {
		t.Errorf("accounting shared key = %q, want the entry's key", cfg.AccountingServers[0].SharedKey)
	}
}

// VALIDATES: the plain-map shape with neither port leaf gives 1812 and 1813.
// PREVENTS: the parser depending on the YANG default to reach 1813.
func TestAccountingPortDefaultWithoutSchemaDefaults(t *testing.T) {
	cfg, err := parseConfigFromTree(map[string]any{
		"l2tp": map[string]any{"auth": map[string]any{"radius": map[string]any{
			"server": []any{map[string]any{"name": "a", "address": "10.0.0.1", "shared-key": "k"}},
		}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.AccountingServers[0].Address; got != "10.0.0.1:1813" {
		t.Errorf("default accounting destination = %q, want 10.0.0.1:1813", got)
	}
}

// VALIDATES: accounting-port takes 1 and 65535 and refuses 0 and 65536 at the
// parser, the same bounds the YANG zt:port type declares.
// PREVENTS: a JSON-delivered config reaching the socket with an impossible port.
func TestAccountingPortBoundaries(t *testing.T) {
	for _, tc := range []struct {
		port     float64
		accepted bool
	}{
		{0, false}, {1, true}, {65535, true}, {65536, false},
	} {
		_, err := parseConfigFromTree(map[string]any{
			"l2tp": map[string]any{"auth": map[string]any{"radius": map[string]any{
				"server": []any{map[string]any{
					"name": "a", "address": "10.0.0.1", "shared-key": "k", "accounting-port": tc.port,
				}},
			}}},
		})
		if tc.accepted && err != nil {
			t.Errorf("accounting-port %v refused: %v", tc.port, err)
		}
		if !tc.accepted && err == nil {
			t.Errorf("accounting-port %v accepted, want refused", tc.port)
		}
	}
}

// awaitCode waits for one datagram on srv and returns its RADIUS Code.
func awaitCode(t *testing.T, srv *capturingRADIUS, what string) uint8 {
	t.Helper()
	select {
	case raw := <-srv.reqs:
		return raw[0]
	case <-time.After(5 * time.Second):
		t.Fatalf("no datagram reached the %s socket", what)
		return 0
	}
}

// expectSilence fails when srv received anything.
func expectSilence(t *testing.T, srv *capturingRADIUS, what string) {
	t.Helper()
	select {
	case raw := <-srv.reqs:
		t.Errorf("the %s socket received code %d, want nothing", what, raw[0])
	default:
	}
}

// VALIDATES: through the production wiring (parser, activateRadiusConfig, the
// auth handler and the accountant), an Access-Request reaches the entry's
// `port` and every Accounting-Request reaches its `accounting-port`. Two
// capture sockets stand for the two server ports, and each must see only its
// own code.
// PREVENTS: accounting sharing the authentication client, so that every
// Accounting-Request goes to the authentication port.
func TestAccountingRequestGoesToTheAccountingPort(t *testing.T) {
	key := []byte("testing123")
	authServer := startCapturingRADIUS(t, key, radius.CodeAccessReject)
	acctServer := startCapturingRADIUS(t, key, radius.CodeAccountingResp)
	_, authPort, err := net.SplitHostPort(authServer.addr)
	if err != nil {
		t.Fatal(err)
	}
	_, acctPort, err := net.SplitHostPort(acctServer.addr)
	if err != nil {
		t.Fatal(err)
	}

	cfg := parseRADIUSConfigText(t, "                address 127.0.0.1\n"+
		"                port "+authPort+"\n"+
		"                accounting-port "+acctPort+"\n"+
		"                shared-key testing123\n")

	previousHandler := l2tp.GetAuthHandler()
	if err := activateRadiusConfig(cfg); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if c := authInstance.swapClient(nil, "", "", nil, ""); c != nil {
			c.Close() //nolint:errcheck // test cleanup
		}
		if c := acctInstance.setClient(nil, "", 0, "", nil, ""); c != nil {
			c.Close() //nolint:errcheck // test cleanup
		}
		if previousHandler == nil {
			l2tp.UnregisterAuthHandler()
			return
		}
		l2tp.RegisterAuthHandler(previousHandler)
	})

	resp := newFakeResponder()
	authInstance.handle(ppp.EventAuthRequest{
		TunnelID: 1, SessionID: 2, Method: ppp.AuthMethodPAP,
		Username: "alice", Response: []byte("password123"),
	}, resp.respond)
	resp.waitOne(t)
	if code := awaitCode(t, authServer, "authentication"); code != radius.CodeAccessRequest {
		t.Errorf("authentication socket received code %d, want Access-Request (%d)", code, radius.CodeAccessRequest)
	}
	expectSilence(t, acctServer, "accounting")

	acctInstance.onSessionIPAssigned(&events.SessionIPAssignedPayload{
		TunnelID: 1, SessionID: 2, Username: "alice", PeerAddr: "198.51.100.23",
	})
	if code := awaitCode(t, acctServer, "accounting"); code != radius.CodeAccountingReq {
		t.Errorf("accounting socket received code %d, want Accounting-Request (%d)", code, radius.CodeAccountingReq)
	}
	acctInstance.onSessionDown(&events.SessionDownPayload{TunnelID: 1, SessionID: 2})
	if code := awaitCode(t, acctServer, "accounting"); code != radius.CodeAccountingReq {
		t.Errorf("accounting socket received code %d for the Stop, want Accounting-Request (%d)", code, radius.CodeAccountingReq)
	}
	expectSilence(t, authServer, "authentication")
}
