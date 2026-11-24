// Design: docs/architecture/hub-architecture.md -- an external plugin's declaration changes only when ze restarts
// Related: main_reload.go -- diskConfigLoaders, the loader every reload reads the config through

package hub

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	zePlugin "github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/opererr"
)

// reloadAddsPlugin is reloadValidHost with an external plugin declared.
const reloadAddsPlugin = `system {
	host reload-valid-host;
}
bus {
	endpoint late-plugin {
		external {
			run "./late-plugin";
		}
	}
}
`

// TestReloadLoaderRefusesChangedPluginDeclarations: the loader every reload
// reads the config through compares the config's external plugins with the
// ones the daemon started with, before the full parse. Method: write a config
// and call the loader the daemon installs, once for a plugin the config adds
// and once for one it removes.
//
// VALIDATES: AC-12. Both loads are refused with the operator error that names
// the plugin and the restart of ze, and no tree is answered.
// PREVENTS: a reload that accepts a config whose added plugin is never
// started, or whose removed plugin keeps running, and reports success.
func TestReloadLoaderRefusesChangedPluginDeclarations(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "router.conf")
	store := newTestFileStore(t, configPath)

	require.NoError(t, os.WriteFile(configPath, []byte(reloadAddsPlugin), 0o600))
	_, loadBoth := diskConfigLoaders(store, configPath, nil, nil)
	treeMap, tree, err := loadBoth()
	require.Error(t, err, "a reload that adds an external plugin was loaded")
	assert.Nil(t, treeMap)
	assert.Nil(t, tree)
	refusal, operator := errors.AsType[*opererr.Error](err)
	require.True(t, operator, "the refusal is no operator error: %v", err)
	assert.Contains(t, refusal.Problem(), `it adds plugin "late-plugin"`)
	assert.Equal(t, "restart ze", refusal.Fix())

	require.NoError(t, os.WriteFile(configPath, []byte(reloadValidHost), 0o600))
	started := []zePlugin.PluginConfig{{Name: "acme-monitor", Run: "./acme-monitor"}}
	_, loadBoth = diskConfigLoaders(store, configPath, nil, started)
	_, _, err = loadBoth()
	require.Error(t, err, "a reload that removes an external plugin was loaded")
	refusal, operator = errors.AsType[*opererr.Error](err)
	require.True(t, operator, "the refusal is no operator error: %v", err)
	assert.Contains(t, refusal.Problem(), `it removes plugin "acme-monitor"`)
	assert.Equal(t, "restart ze", refusal.Fix())
}

// TestReloadLoaderLoadsWhenTheDeclarationsAreUnchanged: a reload that leaves
// the external plugins as they were is loaded, whatever else it changes.
// Method: a daemon that started with no external plugin loads a config that
// declares none.
//
// VALIDATES: AC-12's permitted survivor at the loader: equal declarations
// reach the full parse and answer the tree.
// PREVENTS: the comparison refusing every reload.
func TestReloadLoaderLoadsWhenTheDeclarationsAreUnchanged(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "router.conf")
	store := newTestFileStore(t, configPath)
	require.NoError(t, os.WriteFile(configPath, []byte(reloadValidHost), 0o600))

	_, loadBoth := diskConfigLoaders(store, configPath, nil, nil)
	treeMap, tree, err := loadBoth()
	require.NoError(t, err)
	require.NotNil(t, tree)
	assert.Contains(t, treeMap, "system")
}
