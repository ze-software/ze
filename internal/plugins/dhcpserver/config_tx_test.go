// Design: docs/architecture/config/transaction-protocol.md -- a plugin's verify, apply and rollback of one parsed config

package dhcpserver

import (
	"net"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// dhcpSections returns a service section with two subnets, 10.0.1.0/24 (range
// .100 to .200) and 10.0.2.0/24 (range .100 to stop2), and a tftp sibling
// carrying description, so two calls that differ only in description differ
// only outside dhcp-server's subtree.
func dhcpSections(stop2, description string) []sdk.ConfigSection {
	return []sdk.ConfigSection{{Root: configRootService, Data: `{"service":{"dhcp-server":{"enabled":"true",` +
		`"listen-interface":["eth1"],"shared-network":{"LAN":{"subnet":{` +
		`"10.0.1.0/24":{"range":{"start":"10.0.1.100","stop":"10.0.1.200"},"lease-time":"3600"},` +
		`"10.0.2.0/24":{"range":{"start":"10.0.2.100","stop":"` + stop2 + `"},"lease-time":"3600"}}}}},` +
		`"tftp":{"description":"` + description + `"}}}`}}
}

// newTestDHCPConfigTx returns a transaction whose listeners bind loopback
// sockets in place of the configured interfaces.
func newTestDHCPConfigTx(t *testing.T) (*sdk.ConfigTx[serverConfig], *dhcpServer) {
	t.Helper()
	orig := dhcpListen
	t.Cleanup(func() { dhcpListen = orig })
	dhcpListen = func(string) (*net.UDPConn, error) { return ephemeralUDP(t), nil }

	log, _ := captureLogger()
	srv := &dhcpServer{log: log}
	t.Cleanup(srv.stop)
	return sdk.NewConfigTx(serverConfig{}, parseDHCPSections, srv.install), srv
}

// handlerFor answers the running handler of prefix, failing the test when
// there is none.
func handlerFor(t *testing.T, srv *dhcpServer, prefix string) *dhcpHandler {
	t.Helper()
	want := netip.MustParsePrefix(prefix)
	for _, h := range srv.handlers {
		if h.subnet.Prefix == want {
			return h
		}
	}
	t.Fatalf("no running handler for %s", prefix)
	return nil
}

// TestConfigTxKeepsLeasesOfUnchangedSubnets proves Q3 = d1 and AC-1 to AC-4
// for dhcpserver: a reload that edits one subnet's range keeps the handler,
// and so the leases, of the subnet it did not edit, while the edited subnet
// gets a new handler at apply (not at verify) and its rollback a new one
// again. Method: a lease is added to each subnet's table, and the handler
// pointers and lease lookups are compared across verify, apply and rollback.
// VALIDATES: spec-plugin-reload-applies-or-refuses row 24, Q3 (d1), R-11.
// PREVENTS: every dhcp edit dropping every client's lease.
// MUTATION: make unchangedHandler return nil and the kept-lease check fails.
func TestConfigTxKeepsLeasesOfUnchangedSubnets(t *testing.T) {
	tx, srv := newTestDHCPConfigTx(t)
	if err := tx.Configure(dhcpSections("10.0.2.200", "x")); err != nil {
		t.Fatalf("configure: %v", err)
	}
	kept := handlerFor(t, srv, "10.0.1.0/24")
	edited := handlerFor(t, srv, "10.0.2.0/24")
	mac := net.HardwareAddr{0x02, 0, 0, 0, 0, 1}
	kept.leases.add(mac, netip.MustParseAddr("10.0.1.100"), 3600)
	edited.leases.add(mac, netip.MustParseAddr("10.0.2.100"), 3600)

	if err := tx.Verify(dhcpSections("10.0.2.150", "x")); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if handlerFor(t, srv, "10.0.2.0/24") != edited {
		t.Fatal("verify replaced the edited subnet's handler")
	}
	if err := tx.Apply(); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if handlerFor(t, srv, "10.0.1.0/24") != kept {
		t.Fatal("apply replaced the handler of the subnet the reload did not edit")
	}
	if kept.leases.lookup(mac) == nil {
		t.Fatal("apply dropped a lease of the subnet the reload did not edit")
	}
	applied := handlerFor(t, srv, "10.0.2.0/24")
	if applied == edited {
		t.Fatal("apply kept the old handler of the edited subnet")
	}
	if applied.subnet.Ranges[0].Stop != netip.MustParseAddr("10.0.2.150") {
		t.Fatalf("edited subnet range ends at %s, want 10.0.2.150", applied.subnet.Ranges[0].Stop)
	}
	if len(srv.listeners) != 1 {
		t.Fatalf("apply left %d listeners, want 1", len(srv.listeners))
	}

	if err := tx.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if handlerFor(t, srv, "10.0.1.0/24") != kept {
		t.Fatal("rollback replaced the handler of the subnet neither config edited")
	}
	restored := handlerFor(t, srv, "10.0.2.0/24")
	if restored.subnet.Ranges[0].Stop != netip.MustParseAddr("10.0.2.200") {
		t.Fatalf("rollback left the edited range ending at %s", restored.subnet.Ranges[0].Stop)
	}
}

// TestConfigTxUnchangedSubtreeIsNoop proves AC-5 for dhcpserver: a reload that
// edits service outside dhcp-server rebinds no listener and replaces no
// handler, so no lease is dropped, and its rollback changes nothing either.
// Method: the handler and listener pointers are compared across a verify,
// apply and rollback whose candidate differs only in a sibling's leaf.
// VALIDATES: spec-plugin-reload-applies-or-refuses AC-5 for dhcpserver.
// PREVENTS: an unrelated service edit restarting the DHCP server (R-2).
// MUTATION: delete the reflect.DeepEqual short-circuit in sdk.ConfigTx.Apply
// and the listener check fails.
func TestConfigTxUnchangedSubtreeIsNoop(t *testing.T) {
	tx, srv := newTestDHCPConfigTx(t)
	if err := tx.Configure(dhcpSections("10.0.2.200", "first")); err != nil {
		t.Fatalf("configure: %v", err)
	}
	handlers := append([]*dhcpHandler(nil), srv.handlers...)
	listener := srv.listeners[0]
	for _, step := range []struct {
		name string
		run  func() error
	}{
		{"verify", func() error { return tx.Verify(dhcpSections("10.0.2.200", "second")) }},
		{"apply", tx.Apply},
		{"rollback", tx.Rollback},
	} {
		if err := step.run(); err != nil {
			t.Fatalf("%s: %v", step.name, err)
		}
		if len(srv.listeners) != 1 || srv.listeners[0] != listener {
			t.Fatalf("%s of an edit outside dhcp-server rebound the listener", step.name)
		}
		for i, h := range handlers {
			if srv.handlers[i] != h {
				t.Fatalf("%s of an edit outside dhcp-server replaced a handler", step.name)
			}
		}
	}
}
