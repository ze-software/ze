// Design: docs/architecture/config/transaction-protocol.md -- a plugin's verify, apply and rollback of one parsed config

package tftpserver

import (
	"bytes"
	"encoding/binary"
	"errors"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/tftp"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// tftpSections returns a service section with tftp-server enabled on one
// interface with root and a dhcp-server sibling carrying description, so two
// calls that differ only in description differ only outside tftp-server's
// subtree.
func tftpSections(root, description string) []sdk.ConfigSection {
	return []sdk.ConfigSection{{Root: configRootService, Data: `{"service":{"tftp-server":{"enabled":"true",` +
		`"listen-interface":["eth0"],"root-directory":"` + root + `","max-transfers":"4"},` +
		`"dhcp-server":{"description":"` + description + `"}}}`}}
}

// newTestTFTPConfigTx returns a transaction whose listeners bind loopback
// sockets in place of the configured interface, and the buffer its log goes
// to.
func newTestTFTPConfigTx(t *testing.T) (*sdk.ConfigTx[tftpConfig], *tftpServer, *bytes.Buffer) {
	t.Helper()
	tx, srv, buf, _ := newTestTFTPConfigTxRefusing(t)
	return tx, srv, buf
}

// newTestTFTPConfigTxRefusing is newTestTFTPConfigTx, and also answers the
// set of device names whose bind the stub refuses, which a test fills to
// model an interface that does not exist.
func newTestTFTPConfigTxRefusing(t *testing.T) (*sdk.ConfigTx[tftpConfig], *tftpServer, *bytes.Buffer, map[string]bool) {
	t.Helper()
	orig := tftpListen
	t.Cleanup(func() { tftpListen = orig })
	refused := map[string]bool{}
	tftpListen = func(device string) (*net.UDPConn, error) {
		if refused[device] {
			return nil, errors.New("no such device")
		}
		return net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	}

	var buf bytes.Buffer
	srv := &tftpServer{log: slog.New(slog.NewTextHandler(&buf, nil))}
	t.Cleanup(srv.stop)
	return sdk.NewConfigTx(tftpConfig{}, parseTFTPSections, srv.install), srv, &buf, refused
}

// tftpSectionsOn is tftpSections serving root on iface.
func tftpSectionsOn(iface, root string) []sdk.ConfigSection {
	return []sdk.ConfigSection{{Root: configRootService, Data: `{"service":{"tftp-server":{"enabled":"true",` +
		`"listen-interface":["` + iface + `"],"root-directory":"` + root + `","max-transfers":"4"}}}`}}
}

// TestConfigTxAppliesReloadedRootDirectory proves AC-1 to AC-4 for tftpserver:
// verify binds nothing, apply rebinds the listener for the new root
// directory, and rollback rebinds it for the replaced one.
// VALIDATES: spec-plugin-reload-applies-or-refuses row 25.
// PREVENTS: the acked-and-dropped reload that kept serving the boot
// root-directory.
func TestConfigTxAppliesReloadedRootDirectory(t *testing.T) {
	tx, srv, buf := newTestTFTPConfigTx(t)
	if err := tx.Configure(tftpSections("/srv/a", "x")); err != nil {
		t.Fatalf("configure: %v", err)
	}
	booted := srv.listeners[0]
	if err := tx.Verify(tftpSections("/srv/b", "x")); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if srv.listeners[0] != booted {
		t.Fatal("verify rebound the listener")
	}
	if err := tx.Apply(); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if len(srv.listeners) != 1 || srv.listeners[0] == booted {
		t.Fatal("apply of a new root-directory kept the boot listener")
	}
	if !strings.Contains(buf.String(), "root-directory=/srv/b") {
		t.Fatalf("apply did not install root-directory /srv/b: %s", buf.String())
	}
	buf.Reset()
	if err := tx.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if !strings.Contains(buf.String(), "root-directory=/srv/a") {
		t.Fatalf("rollback did not reinstall root-directory /srv/a: %s", buf.String())
	}
}

// TestConfigTxRefusesEnabledWithoutRoot proves the reload verify is the static
// verifier: an enabled server with no root-directory is refused at verify and
// the running listener stays.
// VALIDATES: spec-plugin-reload-applies-or-refuses I2 for tftpserver.
// PREVENTS: a reload accepting a config the static verifier refuses.
func TestConfigTxRefusesEnabledWithoutRoot(t *testing.T) {
	tx, srv, _ := newTestTFTPConfigTx(t)
	if err := tx.Configure(tftpSections("/srv/a", "x")); err != nil {
		t.Fatalf("configure: %v", err)
	}
	booted := srv.listeners[0]
	if err := tx.Verify(tftpSections("", "x")); err == nil {
		t.Fatal("verify accepted an enabled server with no root-directory")
	}
	if err := tx.Apply(); err == nil {
		t.Fatal("apply after a refused verify was accepted")
	}
	if srv.listeners[0] != booted {
		t.Fatal("a refused reload rebound the listener")
	}
}

// TestConfigTxUnchangedSubtreeIsNoop proves AC-5 for tftpserver: a reload
// that edits service outside tftp-server rebinds no listener, so no request
// is lost, and its rollback rebinds nothing either.
// VALIDATES: spec-plugin-reload-applies-or-refuses AC-5 for tftpserver.
// PREVENTS: an unrelated service edit restarting the TFTP server (R-2).
// MUTATION: delete the reflect.DeepEqual short-circuit in sdk.ConfigTx.Apply
// and the listener check fails.
func TestConfigTxUnchangedSubtreeIsNoop(t *testing.T) {
	tx, srv, _ := newTestTFTPConfigTx(t)
	if err := tx.Configure(tftpSections("/srv/a", "first")); err != nil {
		t.Fatalf("configure: %v", err)
	}
	listener := srv.listeners[0]
	for _, step := range []struct {
		name string
		run  func() error
	}{
		{"verify", func() error { return tx.Verify(tftpSections("/srv/a", "second")) }},
		{"apply", tx.Apply},
		{"rollback", tx.Rollback},
	} {
		if err := step.run(); err != nil {
			t.Fatalf("%s: %v", step.name, err)
		}
		if len(srv.listeners) != 1 || srv.listeners[0] != listener {
			t.Fatalf("%s of an edit outside tftp-server rebound the listener", step.name)
		}
	}
}

// TestConfigTxUnbindableInterfaceFailsApply proves R-13 for tftpserver: an
// apply whose listen-interface cannot be bound returns an error naming the
// interface and the fix, so the transaction rolls back, and the server binds
// the config it ran again.
// VALIDATES: spec-plugin-reload-applies-or-refuses R-13 for tftpserver.
// PREVENTS: a commit that succeeds while the TFTP server serves nothing.
// MUTATION: log the bind failure and return nil, and the error check fails.
func TestConfigTxUnbindableInterfaceFailsApply(t *testing.T) {
	tx, srv, _, refused := newTestTFTPConfigTxRefusing(t)
	refused["eth9"] = true
	if err := tx.Configure(tftpSectionsOn("eth0", "/srv/a")); err != nil {
		t.Fatalf("configure: %v", err)
	}
	if err := tx.Verify(tftpSectionsOn("eth9", "/srv/a")); err != nil {
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
}

// TestConfigTxRollbackThatCannotRebindFails proves the CodeBroken path of
// R-13 is reachable for tftpserver: a rollback whose replaced config can no
// longer be bound returns an error, so the orchestrator restarts the plugin.
// VALIDATES: spec-plugin-reload-applies-or-refuses R-13, AC-4.
// PREVENTS: a rollback acked while the TFTP server serves nothing.
func TestConfigTxRollbackThatCannotRebindFails(t *testing.T) {
	tx, _, _, refused := newTestTFTPConfigTxRefusing(t)
	if err := tx.Configure(tftpSectionsOn("eth0", "/srv/a")); err != nil {
		t.Fatalf("configure: %v", err)
	}
	if err := tx.Verify(tftpSectionsOn("eth1", "/srv/a")); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if err := tx.Apply(); err != nil {
		t.Fatalf("apply: %v", err)
	}
	refused["eth0"] = true
	if err := tx.Rollback(); err == nil {
		t.Fatal("rollback onto an interface that cannot be bound succeeded")
	}
}

// TestConfigTxReloadCutsTransferInFlight proves spec row 25 for tftpserver: a
// transfer running when a changed config is applied is cut, so no file is
// served from a root directory the operator replaced.
// Method: a client reads block 1 of a three-block file, the root directory
// is changed and applied, then the client acknowledges block 1; with the
// transfer cut, block 2 never comes.
// VALIDATES: spec-plugin-reload-applies-or-refuses row 25 (cut on change).
// PREVENTS: a transfer finishing from the old root after a reload.
// MUTATION: drop the context.AfterFunc in handleRRQ and block 2 arrives.
func TestConfigTxReloadCutsTransferInFlight(t *testing.T) {
	tx, srv, _ := newTestTFTPConfigTx(t)
	rootA, rootB := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(rootA, "boot.bin"), make([]byte, 3*512-1), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	if err := tx.Configure(tftpSectionsOn("eth0", rootA)); err != nil {
		t.Fatalf("configure: %v", err)
	}
	server, ok := srv.listeners[0].conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatalf("listener address is %T", srv.listeners[0].conn.LocalAddr())
	}
	client, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	rrq := append([]byte{0, byte(tftp.OpRRQ)}, []byte("boot.bin\x00octet\x00")...)
	if _, err := client.WriteToUDP(rrq, server); err != nil {
		t.Fatalf("send RRQ: %v", err)
	}
	buf := make([]byte, 600)
	if err := client.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("deadline: %v", err)
	}
	n, transfer, err := client.ReadFromUDP(buf)
	if err != nil || n < 4 || binary.BigEndian.Uint16(buf[2:4]) != 1 {
		t.Fatalf("block 1 not received: n=%d err=%v", n, err)
	}

	if err := tx.Verify(tftpSectionsOn("eth0", rootB)); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if err := tx.Apply(); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if _, err := client.WriteToUDP([]byte{0, byte(tftp.OpACK), 0, 1}, transfer); err != nil {
		t.Fatalf("send ACK 1: %v", err)
	}
	if err := client.SetReadDeadline(time.Now().Add(500 * time.Millisecond)); err != nil {
		t.Fatalf("deadline: %v", err)
	}
	n, _, err = client.ReadFromUDP(buf)
	if err == nil && n >= 4 && binary.BigEndian.Uint16(buf[2:4]) == 2 {
		t.Fatal("the transfer from the replaced root directory went on after the reload")
	}
}

// tftpSectionsOnAll is tftpSectionsOn serving root on every interface of
// ifaces.
func tftpSectionsOnAll(root string, ifaces ...string) []sdk.ConfigSection {
	return []sdk.ConfigSection{{Root: configRootService, Data: `{"service":{"tftp-server":{"enabled":"true",` +
		`"listen-interface":["` + strings.Join(ifaces, `","`) + `"],"root-directory":"` + root + `","max-transfers":"4"}}}`}}
}

// TestConfigTxReloadThatLosesARunningInterfaceNamesIt proves ISSUE-5 for
// tftpserver, partial loss: a refused reload whose running interfaces cannot
// all be bound again keeps the ones that bind serving and fails with
// sdk.ErrRunningConfigLost naming the one it lost.
// Method: the server runs on eth0 and eth1; the reload names eth9, which is
// refused, and eth1 is refused too, as a port taken in the rebind window.
// VALIDATES: spec-plugin-reload-applies-or-refuses round 2 ISSUE-5.
// PREVENTS: one lost interface closing every other one, and an error that
// does not say the running config is no longer what runs.
func TestConfigTxReloadThatLosesARunningInterfaceNamesIt(t *testing.T) {
	tx, srv, _, refused := newTestTFTPConfigTxRefusing(t)
	if err := tx.Configure(tftpSectionsOnAll("/srv/a", "eth0", "eth1")); err != nil {
		t.Fatalf("configure: %v", err)
	}
	if err := tx.Verify(tftpSectionsOnAll("/srv/a", "eth9")); err != nil {
		t.Fatalf("verify: %v", err)
	}
	refused["eth9"] = true
	refused["eth1"] = true
	err := tx.Apply()
	if !errors.Is(err, sdk.ErrRunningConfigLost) {
		t.Fatalf("reload that lost eth1: %v, want sdk.ErrRunningConfigLost", err)
	}
	for _, want := range []string{`"eth9"`, `"eth1"`, "1 of the 2", "ze rolls the commit back",
		"Fix: stop the program that holds the address and port", "systemctl restart ze"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("apply error %q does not say %s", err, want)
		}
	}
	if strings.Contains(err.Error(), `"eth0"`) {
		t.Errorf("apply error %q names eth0, which serves again", err)
	}
	if len(srv.listeners) != 1 {
		t.Fatalf("after losing eth1, %d listeners serve, want eth0's 1", len(srv.listeners))
	}
}

// TestConfigTxReloadThatLosesEveryRunningInterfaceRepairs proves ISSUE-5 for
// tftpserver, total loss, through sdk.ConfigTx: the failed apply wraps
// sdk.ErrRunningConfigLost, the rollback while eth0 is still refused answers
// an error (the bridge acks it CodeBroken and the engine restarts the
// plugin), and once eth0 binds, a later apply of the running config binds it
// rather than being skipped as unchanged.
// VALIDATES: spec-plugin-reload-applies-or-refuses round 2 ISSUE-5.
// PREVENTS: a TFTP server dark after a refused reload, with every later
// commit of the running config answered OK as a no-op.
func TestConfigTxReloadThatLosesEveryRunningInterfaceRepairs(t *testing.T) {
	tx, srv, _, refused := newTestTFTPConfigTxRefusing(t)
	if err := tx.Configure(tftpSectionsOn("eth0", "/srv/a")); err != nil {
		t.Fatalf("configure: %v", err)
	}
	if err := tx.Verify(tftpSectionsOn("eth9", "/srv/a")); err != nil {
		t.Fatalf("verify: %v", err)
	}
	refused["eth9"] = true
	refused["eth0"] = true
	if err := tx.Apply(); !errors.Is(err, sdk.ErrRunningConfigLost) {
		t.Fatalf("reload that lost eth0: %v, want sdk.ErrRunningConfigLost", err)
	}
	if err := tx.Rollback(); err == nil {
		t.Fatal("rollback while eth0 is still refused answered OK")
	}

	refused["eth0"] = false
	if err := tx.Verify(tftpSectionsOn("eth0", "/srv/a")); err != nil {
		t.Fatalf("verify the running config: %v", err)
	}
	if err := tx.Apply(); err != nil {
		t.Fatalf("a later apply of the running config: %v", err)
	}
	if len(srv.listeners) != 1 {
		t.Fatalf("after the re-commit %d listeners serve, want 1", len(srv.listeners))
	}
}

// TestReloadOfTransferValuesKeepsTransfers proves AC-24: a reload that changes
// only timeout and max-windowsize cuts no transfer and rebinds no listener. The
// transfer in flight keeps the window it started with, and the next request
// takes the new one.
// Method: a client starts a transfer under max-windowsize 4 and reads its
// first window; max-windowsize 2 and timeout 1 are applied; the client
// acknowledges the window and gets a full window of 4 again, while a second
// request is granted 2.
// VALIDATES: the limits-only path of tftpServer.install.
// PREVENTS: a timeout change cutting every PXE boot in flight, and a running
// transfer whose window changes under it.
func TestReloadOfTransferValuesKeepsTransfers(t *testing.T) {
	tx, srv, _ := newTestTFTPConfigTx(t)
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "boot.bin"), make([]byte, 12*512), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}
	sections := func(window, timeout string) []sdk.ConfigSection {
		return []sdk.ConfigSection{{Root: configRootService, Data: `{"service":{"tftp-server":{"enabled":"true",` +
			`"listen-interface":["eth0"],"root-directory":"` + root + `","max-transfers":"4",` +
			`"max-windowsize":"` + window + `","timeout":"` + timeout + `"}}}`}}
	}
	if err := tx.Configure(sections("4", "5")); err != nil {
		t.Fatalf("configure: %v", err)
	}
	listener := srv.listeners[0].conn
	server, ok := listener.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatalf("listener address is %T", listener.LocalAddr())
	}
	running := newTransferClientFor(t, server)
	running.sendRRQ("boot.bin", "windowsize", "8")
	opts, _ := running.expectOACK()
	if opts["windowsize"] != "4" {
		t.Fatalf("OACK windowsize = %q, want the configured 4", opts["windowsize"])
	}
	running.ack(0)
	running.expectWindow(1, 4)

	if err := tx.Verify(sections("2", "1")); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if err := tx.Apply(); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if srv.listeners[0].conn != listener {
		t.Fatal("a reload of timeout and max-windowsize bound the listener again")
	}

	running.ack(4)
	running.expectWindow(5, 4)

	next := newTransferClientFor(t, server)
	next.sendRRQ("boot.bin", "windowsize", "8")
	opts, _ = next.expectOACK()
	if opts["windowsize"] != "2" {
		t.Fatalf("OACK windowsize after the reload = %q, want the new 2", opts["windowsize"])
	}
}
