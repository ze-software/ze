// Design: docs/architecture/config/yang-config-design.md -- Validation
// Related: main_reload.go -- diskConfigLoaders and doReload, the reload these tests drive

package hub

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

// reloadValidHost is a config every YANG type and every ze:validate rule accepts.
const reloadValidHost = `system {
	host reload-valid-host;
}
`

// reloadRefusedPlugin passes the parse window, since plugin/internal/use is a
// string of 1..64 characters, and only the ze:validate rule that names the
// internal plugins refuses it.
const reloadRefusedPlugin = `system {
	host reload-refused-host;
}
plugin {
	internal p1 {
		use no-such-plugin-here
	}
}
`

// reloadThroughDisk writes config to the daemon's config file and runs the
// reload the daemon runs on SIGHUP: diskConfigLoaders is the loader runYANGConfig
// installs, and doReload is the driver every reload trigger reaches.
func reloadThroughDisk(t *testing.T, server *pluginserver.Server, store storage.Storage, configPath, config string) error {
	t.Helper()
	require.NoError(t, os.WriteFile(configPath, []byte(config), 0o600))
	_, loadBoth := diskConfigLoaders(store, configPath, nil)
	return doReload(server, nil, store, configPath, loadBoth, nil)
}

// reloadInstalledHost reads system/host out of the tree the reload installed.
func reloadInstalledHost(t *testing.T, reactor *reloadTestReactor) string {
	t.Helper()
	system, ok := reactor.setTree["system"].(map[string]any)
	require.True(t, ok, "the installed tree carries no system container: %v", reactor.setTree)
	host, ok := system["host"].(string)
	require.True(t, ok, "the installed system container carries no host: %v", system)
	return host
}

// newReloadValidationServer builds a plugin server over a recording reactor.
func newReloadValidationServer(t *testing.T) (*pluginserver.Server, *reloadTestReactor) {
	t.Helper()
	reactor := &reloadTestReactor{tree: map[string]any{}}
	server, err := pluginserver.NewServer(&pluginserver.ServerConfig{}, reactor)
	require.NoError(t, err)
	t.Cleanup(func() { server.Stop() })
	return server, reactor
}

// VALIDATES: a reload of a config every constraint accepts installs that config
// as the running tree. The method writes the file and drives the daemon's own
// loader and reload driver, then reads the tree the reactor was given.
// PREVENTS: a reload test that passes because nothing is ever installed.
//
// RFC 7950 Section 8.1: "The running configuration datastore MUST always be valid."
//
// RFC requirement: RFC7950-8.1-2 positive -- a SIGHUP reload of a valid config through diskConfigLoaders and doReload returns no error and the reactor receives the tree, its system/host "reload-valid-host".
func TestRFC7950ReloadInstallsAValidConfig(t *testing.T) {
	server, reactor := newReloadValidationServer(t)
	configPath := filepath.Join(t.TempDir(), "router.conf")
	store := newTestFileStore(t, configPath)

	require.NoError(t, reloadThroughDisk(t, server, store, configPath, reloadValidHost))
	if got := reloadInstalledHost(t, reactor); got != "reload-valid-host" {
		t.Errorf("installed host = %q, want reload-valid-host", got)
	}
}

// VALIDATES: a reload of a config that a ze:validate rule refuses is itself
// refused, and the running tree stays the last valid one. The method installs a
// valid config first, then reloads the refused one through the same path.
// PREVENTS: a reload route that installs a tree without the ze:validate pass that
// LoadConfig runs, which would make the running datastore invalid.
//
// RFC requirement: RFC7950-8.1-2 negative -- after a valid reload, a SIGHUP reload of a config the internal-plugin-name ze:validate rule refuses returns ErrCustomValidation naming the value, and the reactor still holds the valid tree, its system/host "reload-valid-host".
func TestRFC7950ReloadRefusesAConstraintViolation(t *testing.T) {
	server, reactor := newReloadValidationServer(t)
	configPath := filepath.Join(t.TempDir(), "router.conf")
	store := newTestFileStore(t, configPath)
	require.NoError(t, reloadThroughDisk(t, server, store, configPath, reloadValidHost))

	err := reloadThroughDisk(t, server, store, configPath, reloadRefusedPlugin)
	if err == nil {
		t.Fatal("the reload installed a config the ze:validate rule refuses")
	}
	if !errors.Is(err, zeconfig.ErrCustomValidation) {
		t.Errorf("refusal is not ErrCustomValidation: %v", err)
	}
	if !strings.Contains(err.Error(), "no-such-plugin-here") {
		t.Errorf("refusal does not name the refused value: %v", err)
	}
	if got := reloadInstalledHost(t, reactor); got != "reload-valid-host" {
		t.Errorf("installed host = %q after the refused reload, want reload-valid-host", got)
	}
}
