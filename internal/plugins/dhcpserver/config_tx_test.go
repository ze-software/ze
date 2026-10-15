// Design: docs/architecture/config/transaction-protocol.md -- a plugin's verify, apply and rollback of one parsed config

package dhcpserver

import (
	"errors"
	"net"
	"net/netip"
	"strings"
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

// dhcpSectionsOn returns a service section serving 10.0.1.0/24 (range .100
// to .200) on iface, with router as its default router and, when bootfile is
// not empty, PXE answering with bootfile as the BIOS boot file.
func dhcpSectionsOn(iface, router, bootfile string) []sdk.ConfigSection {
	pxe := ""
	if bootfile != "" {
		pxe = `,"pxe":{"enabled":"true","tftp-server":"10.0.1.1","bootfile-bios":"` + bootfile + `","bootfile-uefi":"ipxe.efi"}`
	}
	return []sdk.ConfigSection{{Root: configRootService, Data: `{"service":{"dhcp-server":{"enabled":"true",` +
		`"listen-interface":["` + iface + `"],"shared-network":{"LAN":{"subnet":{` +
		`"10.0.1.0/24":{"range":{"start":"10.0.1.100","stop":"10.0.1.200"},"lease-time":"3600",` +
		`"default-router":"` + router + `"}}}}` + pxe + `}}}`}}
}

// newTestDHCPConfigTx returns a transaction whose listeners bind loopback
// sockets in place of the configured interfaces, and the set of interface
// names whose bind the stub refuses, which a test fills to model an interface
// that does not exist.
func newTestDHCPConfigTx(t *testing.T) (*sdk.ConfigTx[serverConfig], *dhcpServer, map[string]bool) {
	t.Helper()
	orig := dhcpListen
	t.Cleanup(func() { dhcpListen = orig })
	refused := map[string]bool{}
	dhcpListen = func(iface string) (*net.UDPConn, error) {
		if refused[iface] {
			return nil, errors.New("no such device")
		}
		return ephemeralUDP(t), nil
	}

	log, _ := captureLogger()
	srv := &dhcpServer{log: log}
	t.Cleanup(srv.stop)
	return sdk.NewConfigTx(serverConfig{}, parseDHCPSections, srv.install), srv, refused
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
// MUTATION: make sameAddressPlan return false and the kept-lease check fails.
func TestConfigTxKeepsLeasesOfUnchangedSubnets(t *testing.T) {
	tx, srv, _ := newTestDHCPConfigTx(t)
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
	if handlerFor(t, srv, "10.0.1.0/24").leases != kept.leases {
		t.Fatal("apply replaced the lease table of the subnet the reload did not edit")
	}
	if kept.leases.lookup(mac) == nil {
		t.Fatal("apply dropped a lease of the subnet the reload did not edit")
	}
	applied := handlerFor(t, srv, "10.0.2.0/24")
	if applied.leases == edited.leases {
		t.Fatal("apply kept the old lease table of the subnet whose range it edited")
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
	if handlerFor(t, srv, "10.0.1.0/24").leases != kept.leases {
		t.Fatal("rollback replaced the lease table of the subnet neither config edited")
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
	tx, srv, _ := newTestDHCPConfigTx(t)
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

// TestConfigTxOptionsOrPXEEditKeepsLeases proves Q3 = d1 by the letter: a
// reload that edits only a subnet's options, or only the PXE settings, keeps
// the subnet's pool and lease table, because its prefix, ranges and static
// mappings are unchanged, while the handler answers with the new options.
// Method: a lease is added, the edit is applied, and the running handler's
// lease table, the lease, and the edited value are read back.
// VALIDATES: spec-plugin-reload-applies-or-refuses Q3 (d1), R-11.
// PREVENTS: a default-router or boot file edit dropping every client's lease.
// MUTATION: compare the whole subnet and the PXE settings in sameAddressPlan
// and both cases fail.
func TestConfigTxOptionsOrPXEEditKeepsLeases(t *testing.T) {
	for _, tc := range []struct {
		name  string
		edit  []sdk.ConfigSection
		check func(h *dhcpHandler) bool
	}{
		{"default-router", dhcpSectionsOn("eth1", "10.0.1.254", ""), func(h *dhcpHandler) bool {
			return h.subnet.DefaultRouter == netip.MustParseAddr("10.0.1.254")
		}},
		{"pxe", dhcpSectionsOn("eth1", "10.0.1.1", "ipxe.pxe"), func(h *dhcpHandler) bool {
			return h.pxe.BootfileBIOS == "ipxe.pxe"
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, srv, _ := newTestDHCPConfigTx(t)
			if err := tx.Configure(dhcpSectionsOn("eth1", "10.0.1.1", "")); err != nil {
				t.Fatalf("configure: %v", err)
			}
			booted := handlerFor(t, srv, "10.0.1.0/24")
			mac := net.HardwareAddr{0x02, 0, 0, 0, 0, 1}
			booted.leases.add(mac, netip.MustParseAddr("10.0.1.100"), 3600)

			if err := tx.Verify(tc.edit); err != nil {
				t.Fatalf("verify: %v", err)
			}
			if err := tx.Apply(); err != nil {
				t.Fatalf("apply: %v", err)
			}
			applied := handlerFor(t, srv, "10.0.1.0/24")
			if applied.leases != booted.leases {
				t.Fatalf("a %s edit replaced the lease table of a subnet whose address plan it did not edit", tc.name)
			}
			if applied.leases.lookup(mac) == nil {
				t.Fatalf("a %s edit dropped a lease", tc.name)
			}
			if !tc.check(applied) {
				t.Fatalf("the running handler does not answer with the %s edit", tc.name)
			}
		})
	}
}

// TestConfigTxUnbindableInterfaceFailsApply proves R-13 for dhcpserver: an
// apply whose listen-interface cannot be bound returns an error that names
// the interface and the fix, so the transaction rolls back, and the server
// keeps serving the config it ran, with its listener and its leases.
// Method: the listener stub refuses eth9, a reload moves the server there,
// and the apply's error, the listener count and the lease are read back.
// VALIDATES: spec-plugin-reload-applies-or-refuses R-13 for dhcpserver.
// PREVENTS: a commit that succeeds while the DHCP server serves nothing.
// MUTATION: log the bind failure and return nil, and the error check fails.
func TestConfigTxUnbindableInterfaceFailsApply(t *testing.T) {
	tx, srv, refused := newTestDHCPConfigTx(t)
	refused["eth9"] = true
	if err := tx.Configure(dhcpSectionsOn("eth1", "10.0.1.1", "")); err != nil {
		t.Fatalf("configure: %v", err)
	}
	booted := handlerFor(t, srv, "10.0.1.0/24")
	mac := net.HardwareAddr{0x02, 0, 0, 0, 0, 1}
	booted.leases.add(mac, netip.MustParseAddr("10.0.1.100"), 3600)

	if err := tx.Verify(dhcpSectionsOn("eth9", "10.0.1.1", "")); err != nil {
		t.Fatalf("verify: %v", err)
	}
	err := tx.Apply()
	if err == nil {
		t.Fatal("apply onto an interface that cannot be bound succeeded")
	}
	for _, want := range []string{`"eth9"`, "listen-interface"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("apply error %q does not name %s", err, want)
		}
	}
	if len(srv.listeners) != 1 {
		t.Fatalf("a failed apply left %d listeners, want the 1 it ran", len(srv.listeners))
	}
	if handlerFor(t, srv, "10.0.1.0/24").leases != booted.leases || booted.leases.lookup(mac) == nil {
		t.Fatal("a failed apply dropped the running leases")
	}
}

// TestConfigTxRollbackThatCannotRebindFails proves the CodeBroken path of
// R-13 is reachable for dhcpserver: a rollback whose replaced config can no
// longer be bound returns an error, so the orchestrator restarts the plugin
// instead of believing the old config runs.
// Method: apply moves the server from eth1 to eth2, eth1 is then refused, and
// the rollback's answer is read.
// VALIDATES: spec-plugin-reload-applies-or-refuses R-13, AC-4.
// PREVENTS: a rollback acked while the DHCP server serves nothing.
func TestConfigTxRollbackThatCannotRebindFails(t *testing.T) {
	tx, _, refused := newTestDHCPConfigTx(t)
	if err := tx.Configure(dhcpSectionsOn("eth1", "10.0.1.1", "")); err != nil {
		t.Fatalf("configure: %v", err)
	}
	if err := tx.Verify(dhcpSectionsOn("eth2", "10.0.1.1", "")); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if err := tx.Apply(); err != nil {
		t.Fatalf("apply: %v", err)
	}
	refused["eth1"] = true
	if err := tx.Rollback(); err == nil {
		t.Fatal("rollback onto an interface that cannot be bound succeeded")
	}
}

// dhcpSectionsOnAll is dhcpSectionsOn serving 10.0.1.0/24 on every interface
// of ifaces, with no PXE.
func dhcpSectionsOnAll(ifaces ...string) []sdk.ConfigSection {
	return []sdk.ConfigSection{{Root: configRootService, Data: `{"service":{"dhcp-server":{"enabled":"true",` +
		`"listen-interface":["` + strings.Join(ifaces, `","`) + `"],"shared-network":{"LAN":{"subnet":{` +
		`"10.0.1.0/24":{"range":{"start":"10.0.1.100","stop":"10.0.1.200"},"lease-time":"3600"}}}}}}}`}}
}

// TestConfigTxReloadThatLosesARunningInterfaceNamesIt proves ISSUE-5 for
// dhcpserver, partial loss: a refused reload whose running interfaces cannot
// all be bound again keeps the ones that bind serving and fails with
// sdk.ErrRunningConfigLost naming the one it lost.
// Method: the server runs on eth1 and eth2; the reload names eth9, which is
// refused, and eth2 is refused too, as a port taken in the rebind window.
// VALIDATES: spec-plugin-reload-applies-or-refuses round 2 ISSUE-5.
// PREVENTS: one lost interface closing every other one, and an error that
// does not say the running config is no longer what runs.
func TestConfigTxReloadThatLosesARunningInterfaceNamesIt(t *testing.T) {
	tx, srv, refused := newTestDHCPConfigTx(t)
	if err := tx.Configure(dhcpSectionsOnAll("eth1", "eth2")); err != nil {
		t.Fatalf("configure: %v", err)
	}
	if err := tx.Verify(dhcpSectionsOnAll("eth9")); err != nil {
		t.Fatalf("verify: %v", err)
	}
	refused["eth9"] = true
	refused["eth2"] = true
	err := tx.Apply()
	if !errors.Is(err, sdk.ErrRunningConfigLost) {
		t.Fatalf("reload that lost eth2: %v, want sdk.ErrRunningConfigLost", err)
	}
	for _, want := range []string{`"eth9"`, `"eth2"`, "1 of the 2", "commit the running config again"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("apply error %q does not say %s", err, want)
		}
	}
	if strings.Contains(err.Error(), `"eth1"`) {
		t.Errorf("apply error %q names eth1, which serves again", err)
	}
	if len(srv.listeners) != 1 {
		t.Fatalf("after losing eth2, %d listeners serve, want eth1's 1", len(srv.listeners))
	}
}

// TestConfigTxReloadThatLosesEveryRunningInterfaceRepairs proves ISSUE-5 for
// dhcpserver, total loss, through sdk.ConfigTx: the failed apply wraps
// sdk.ErrRunningConfigLost, the rollback while eth1 is still refused answers
// an error (the bridge acks it CodeBroken and the engine restarts the
// plugin), and once eth1 binds, committing the running config again binds it
// rather than being skipped as unchanged.
// VALIDATES: spec-plugin-reload-applies-or-refuses round 2 ISSUE-5.
// PREVENTS: a DHCP server dark after a refused reload, with every later
// commit of the running config answered OK as a no-op.
func TestConfigTxReloadThatLosesEveryRunningInterfaceRepairs(t *testing.T) {
	tx, srv, refused := newTestDHCPConfigTx(t)
	if err := tx.Configure(dhcpSectionsOnAll("eth1")); err != nil {
		t.Fatalf("configure: %v", err)
	}
	if err := tx.Verify(dhcpSectionsOnAll("eth9")); err != nil {
		t.Fatalf("verify: %v", err)
	}
	refused["eth9"] = true
	refused["eth1"] = true
	if err := tx.Apply(); !errors.Is(err, sdk.ErrRunningConfigLost) {
		t.Fatalf("reload that lost eth1: %v, want sdk.ErrRunningConfigLost", err)
	}
	if err := tx.Rollback(); err == nil {
		t.Fatal("rollback while eth1 is still refused answered OK")
	}

	refused["eth1"] = false
	if err := tx.Verify(dhcpSectionsOnAll("eth1")); err != nil {
		t.Fatalf("verify the running config: %v", err)
	}
	if err := tx.Apply(); err != nil {
		t.Fatalf("commit the running config again: %v", err)
	}
	if len(srv.listeners) != 1 {
		t.Fatalf("after the re-commit %d listeners serve, want 1", len(srv.listeners))
	}
}
