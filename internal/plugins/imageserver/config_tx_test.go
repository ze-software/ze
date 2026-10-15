// Design: docs/architecture/config/transaction-protocol.md -- a plugin's verify, apply and rollback of one parsed config

package imageserver

import (
	"log/slog"
	"net/http"
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
	orig := imageStartTargets
	t.Cleanup(func() { imageStartTargets = orig })
	imageStartTargets = func(cfg imageConfig, _ string, _ []listenTarget, _ *slog.Logger) []*http.Server {
		return []*http.Server{{Addr: cfg.ImageDirectory}}
	}

	srv := &imageServer{log: slog.New(slog.DiscardHandler)}
	t.Cleanup(srv.stop)
	return sdk.NewConfigTx(imageConfig{}, parseImageSections, srv.install), srv
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
