// Design: docs/architecture/config/transaction-protocol.md -- a plugin's verify, apply and rollback of one parsed config

package tftpserver

import (
	"bytes"
	"log/slog"
	"net"
	"strings"
	"testing"

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
	orig := tftpListen
	t.Cleanup(func() { tftpListen = orig })
	tftpListen = func(string) (*net.UDPConn, error) {
		return net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	}

	var buf bytes.Buffer
	srv := &tftpServer{log: slog.New(slog.NewTextHandler(&buf, nil))}
	t.Cleanup(srv.closeListeners)
	return sdk.NewConfigTx(tftpConfig{}, parseTFTPSections, srv.install), srv, &buf
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
