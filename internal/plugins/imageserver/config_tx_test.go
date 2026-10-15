// Design: docs/architecture/config/transaction-protocol.md -- a plugin's verify, apply and rollback of one parsed config

package imageserver

import (
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"testing"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// imageSections returns a service section with image-server enabled on every
// address with imageDir and a tftp-server sibling carrying description, so two
// calls that differ only in description differ only outside image-server's
// subtree.
func imageSections(imageDir, description string) []sdk.ConfigSection {
	return []sdk.ConfigSection{{Root: configRootService, Data: `{"service":{"image-server":{"enabled":"true",` +
		`"image-directory":"` + imageDir + `"},"tftp-server":{"description":"` + description + `"}}}`}}
}

// newTestImageConfigTx returns a transaction whose bind is a stub: each
// install gets one unstarted server whose Addr is the installed
// image-directory, so a test reads which config is running and whether the
// server was replaced.
func newTestImageConfigTx(t *testing.T) (*sdk.ConfigTx[imageConfig], *imageServer) {
	t.Helper()
	tx, srv, _ := newTestImageConfigTxRefusing(t)
	return tx, srv
}

// imageSectionsOn returns a service section with image-server enabled on
// iface, serving imageDir.
func imageSectionsOn(iface, imageDir string) []sdk.ConfigSection {
	return []sdk.ConfigSection{{Root: configRootService, Data: `{"service":{"image-server":{"enabled":"true",` +
		`"listen-interface":["` + iface + `"],"image-directory":"` + imageDir + `"}}}`}}
}

// newTestImageConfigTxRefusing is newTestImageConfigTx, and also answers the
// set of interface names the stub can neither resolve nor bind, which a test
// fills to model an interface that does not exist, or a port taken.
func newTestImageConfigTxRefusing(t *testing.T) (*sdk.ConfigTx[imageConfig], *imageServer, map[string]bool) {
	t.Helper()
	origResolve, origServe := imageResolveIPv4, imageServeTarget
	t.Cleanup(func() { imageResolveIPv4, imageServeTarget = origResolve, origServe })
	refused := map[string]bool{}
	imageResolveIPv4 = func(name string) (string, error) {
		if refused[name] {
			return "", errors.New("no such interface")
		}
		return "127.0.0.1", nil
	}
	imageServeTarget = func(cfg imageConfig, _ string, target listenTarget) (*http.Server, error) {
		if refused[target.iface] {
			return nil, errors.New("address already in use")
		}
		return &http.Server{Addr: cfg.ImageDirectory}, nil
	}

	srv := &imageServer{log: slog.New(slog.DiscardHandler)}
	t.Cleanup(srv.stop)
	return sdk.NewConfigTx(imageConfig{}, parseImageSections, srv.install), srv, refused
}

// TestConfigTxAppliesReloadedImageDirectory proves AC-1 to AC-4 for
// imageserver: verify replaces no server, apply replaces it with one serving
// the new image directory, and rollback with one serving the replaced one.
// VALIDATES: spec-plugin-reload-applies-or-refuses row 26.
// PREVENTS: the acked-and-dropped reload that kept serving the boot
// image-directory.
func TestConfigTxAppliesReloadedImageDirectory(t *testing.T) {
	tx, srv := newTestImageConfigTx(t)
	if err := tx.Configure(imageSections("/srv/a", "x")); err != nil {
		t.Fatalf("configure: %v", err)
	}
	booted := srv.httpServers[0]
	if err := tx.Verify(imageSections("/srv/b", "x")); err != nil {
		t.Fatalf("verify: %v", err)
	}
	if srv.httpServers[0] != booted {
		t.Fatal("verify replaced the server")
	}
	if err := tx.Apply(); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got := srv.httpServers[0].Addr; got != "/srv/b" {
		t.Fatalf("apply left the server on %q, want /srv/b", got)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}
	if got := srv.httpServers[0].Addr; got != "/srv/a" {
		t.Fatalf("rollback left the server on %q, want /srv/a", got)
	}
}

// TestConfigTxRefusesEnabledWithoutDirectory proves the reload verify is the
// static verifier: an enabled server with neither directory is refused at
// verify and the running server stays.
// VALIDATES: spec-plugin-reload-applies-or-refuses I2 for imageserver.
// PREVENTS: a reload accepting a config the static verifier refuses.
func TestConfigTxRefusesEnabledWithoutDirectory(t *testing.T) {
	tx, srv := newTestImageConfigTx(t)
	if err := tx.Configure(imageSections("/srv/a", "x")); err != nil {
		t.Fatalf("configure: %v", err)
	}
	booted := srv.httpServers[0]
	if err := tx.Verify(imageSections("", "x")); err == nil {
		t.Fatal("verify accepted an enabled server with no directory")
	}
	if err := tx.Apply(); err == nil {
		t.Fatal("apply after a refused verify was accepted")
	}
	if srv.httpServers[0] != booted {
		t.Fatal("a refused reload replaced the server")
	}
}

// TestConfigTxUnchangedSubtreeIsNoop proves AC-5 for imageserver: a reload
// that edits service outside image-server replaces no HTTP server, so no
// download is cut, and its rollback replaces nothing either.
// VALIDATES: spec-plugin-reload-applies-or-refuses AC-5 for imageserver.
// PREVENTS: an unrelated service edit cutting every image download (R-2).
// MUTATION: delete the reflect.DeepEqual short-circuit in sdk.ConfigTx.Apply
// and the server check fails.
func TestConfigTxUnchangedSubtreeIsNoop(t *testing.T) {
	tx, srv := newTestImageConfigTx(t)
	if err := tx.Configure(imageSections("/srv/a", "first")); err != nil {
		t.Fatalf("configure: %v", err)
	}
	server := srv.httpServers[0]
	for _, step := range []struct {
		name string
		run  func() error
	}{
		{"verify", func() error { return tx.Verify(imageSections("/srv/a", "second")) }},
		{"apply", tx.Apply},
		{"rollback", tx.Rollback},
	} {
		if err := step.run(); err != nil {
			t.Fatalf("%s: %v", step.name, err)
		}
		if len(srv.httpServers) != 1 || srv.httpServers[0] != server {
			t.Fatalf("%s of an edit outside image-server replaced the server", step.name)
		}
	}
}

// TestConfigTxUnbindableInterfaceFailsApply proves R-13 for imageserver: an
// apply whose listen-interface cannot be bound returns an error naming the
// interface and the fix, so the transaction rolls back, and the server
// serves the config it ran again.
// VALIDATES: spec-plugin-reload-applies-or-refuses R-13 for imageserver.
// PREVENTS: a commit that succeeds while the image server serves nothing.
// MUTATION: log the bind failure and return nil, and the error check fails.
func TestConfigTxUnbindableInterfaceFailsApply(t *testing.T) {
	tx, srv, refused := newTestImageConfigTxRefusing(t)
	refused["eth9"] = true
	if err := tx.Configure(imageSectionsOn("eth0", "/srv/a")); err != nil {
		t.Fatalf("configure: %v", err)
	}
	if err := tx.Verify(imageSectionsOn("eth9", "/srv/b")); err != nil {
		t.Fatalf("verify: %v", err)
	}
	err := tx.Apply()
	if err == nil {
		t.Fatal("apply onto an interface that cannot be bound succeeded")
	}
	for _, want := range []string{"eth9", "listen-interface"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("apply error %q does not name %s", err, want)
		}
	}
	if len(srv.httpServers) != 1 || srv.httpServers[0].Addr != "/srv/a" {
		t.Fatalf("a failed apply does not serve the config it ran: %v", srv.httpServers)
	}
}

// TestConfigTxRollbackThatCannotRebindFails proves the CodeBroken path of
// R-13 is reachable for imageserver: a rollback whose replaced config can no
// longer be bound returns an error, so the orchestrator restarts the plugin.
// VALIDATES: spec-plugin-reload-applies-or-refuses R-13, AC-4.
// PREVENTS: a rollback acked while the image server serves nothing.
func TestConfigTxRollbackThatCannotRebindFails(t *testing.T) {
	tx, _, refused := newTestImageConfigTxRefusing(t)
	if err := tx.Configure(imageSectionsOn("eth0", "/srv/a")); err != nil {
		t.Fatalf("configure: %v", err)
	}
	if err := tx.Verify(imageSectionsOn("eth1", "/srv/a")); err != nil {
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

// imageSectionsOnAll is imageSectionsOn serving imageDir on every interface
// of ifaces.
func imageSectionsOnAll(imageDir string, ifaces ...string) []sdk.ConfigSection {
	return []sdk.ConfigSection{{Root: configRootService, Data: `{"service":{"image-server":{"enabled":"true",` +
		`"listen-interface":["` + strings.Join(ifaces, `","`) + `"],"image-directory":"` + imageDir + `"}}}`}}
}

// TestConfigTxReloadThatLosesARunningInterfaceNamesIt proves ISSUE-5 for
// imageserver, partial loss: a refused reload whose running servers cannot
// all be bound again keeps the ones that bind serving and fails with
// sdk.ErrRunningConfigLost naming the one it lost.
// Method: the server runs on eth1 and eth2; the reload names eth9, which is
// refused, and eth2 is refused too, as a port taken in the rebind window.
// VALIDATES: spec-plugin-reload-applies-or-refuses round 2 ISSUE-5.
// PREVENTS: one lost interface closing every other one, and an error that
// does not say the running config is no longer what runs.
func TestConfigTxReloadThatLosesARunningInterfaceNamesIt(t *testing.T) {
	tx, srv, refused := newTestImageConfigTxRefusing(t)
	if err := tx.Configure(imageSectionsOnAll("/srv/a", "eth1", "eth2")); err != nil {
		t.Fatalf("configure: %v", err)
	}
	if err := tx.Verify(imageSectionsOnAll("/srv/a", "eth9")); err != nil {
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
	if len(srv.httpServers) != 1 {
		t.Fatalf("after losing eth2, %d servers serve, want eth1's 1", len(srv.httpServers))
	}
}

// TestConfigTxReloadThatLosesEveryRunningInterfaceRepairs proves ISSUE-5 for
// imageserver, total loss, through sdk.ConfigTx: the failed apply wraps
// sdk.ErrRunningConfigLost, the rollback while eth1 is still refused answers
// an error (the bridge acks it CodeBroken and the engine restarts the
// plugin), and once eth1 binds, committing the running config again binds it
// rather than being skipped as unchanged.
// VALIDATES: spec-plugin-reload-applies-or-refuses round 2 ISSUE-5.
// PREVENTS: an image server dark after a refused reload, with every later
// commit of the running config answered OK as a no-op.
func TestConfigTxReloadThatLosesEveryRunningInterfaceRepairs(t *testing.T) {
	tx, srv, refused := newTestImageConfigTxRefusing(t)
	if err := tx.Configure(imageSectionsOn("eth1", "/srv/a")); err != nil {
		t.Fatalf("configure: %v", err)
	}
	if err := tx.Verify(imageSectionsOn("eth9", "/srv/a")); err != nil {
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
	if err := tx.Verify(imageSectionsOn("eth1", "/srv/a")); err != nil {
		t.Fatalf("verify the running config: %v", err)
	}
	if err := tx.Apply(); err != nil {
		t.Fatalf("commit the running config again: %v", err)
	}
	if len(srv.httpServers) != 1 {
		t.Fatalf("after the re-commit %d servers serve, want 1", len(srv.httpServers))
	}
}
