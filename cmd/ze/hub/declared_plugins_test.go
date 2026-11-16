package hub

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/yang"
	zePlugin "github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// untouchedSpawner fails the test when boot asks it for anything: a boot with
// no declared external plugin must never reach the plugin manager early.
type untouchedSpawner struct{ t *testing.T }

func (u untouchedSpawner) StartDeclared(context.Context, []zePlugin.PluginConfig) (map[string]error, error) {
	u.t.Fatal("a boot with no declared external plugin started plugins before the full parse")
	return map[string]error{}, nil
}

func (u untouchedSpawner) FailDeclared(context.Context, string, error) {
	u.t.Fatal("a boot with no declared external plugin failed a plugin before the full parse")
}

func (u untouchedSpawner) GetProcessManager() any {
	u.t.Fatal("a boot with no declared external plugin asked for the process manager before the full parse")
	return nil
}

// refusingSpawner forks nothing and answers one refusal for every plugin, as
// a manager with no listener does.
type refusingSpawner struct{ cause error }

func (r refusingSpawner) StartDeclared(context.Context, []zePlugin.PluginConfig) (map[string]error, error) {
	return nil, r.cause
}

func (refusingSpawner) FailDeclared(context.Context, string, error) {}

func (refusingSpawner) GetProcessManager() any { return nil }

// TestNoExternalPluginKeepsBootOrder proves AC-9: when neither the config nor
// the command line declares an external plugin, the steps between the first
// pass and the full parse do nothing. No certificate authority is loaded
// early, no plugin is started, nothing is waited for, and the process's module
// set is not published, so the boot order is the one it was.
func TestNoExternalPluginKeepsBootOrder(t *testing.T) {
	restore := yang.ResetPublishedModulesForTest()
	defer restore()

	firstPass, err := zeconfig.FirstPass("set environment log level info\n", "", nil)
	require.NoError(t, err)
	require.Empty(t, firstPass.External, "the config declares no external plugin")

	declarations, err := joinDeclaredPlugins(context.Background(), firstPass, untouchedSpawner{t: t}, func() error {
		t.Fatal("a boot with no declared external plugin prepared the plugin hub before the full parse")
		return nil
	})

	require.NoError(t, err)
	assert.Nil(t, declarations)
	_, published := yang.PublishedExternalModules()
	assert.False(t, published, "nothing is published when no external plugin is declared")
}

// TestDeclaredPluginThatCannotRunIsKeptAsAnOutcome proves a declared plugin
// that gave no declaration does not refuse the boot by itself: the step
// answers its reason for the load to report, and publishes a module set that
// holds no external module.
func TestDeclaredPluginThatCannotRunIsKeptAsAnOutcome(t *testing.T) {
	restore := yang.ResetPublishedModulesForTest()
	defer restore()

	cause := errors.New("plugin TLS acceptor: no listener")
	firstPass := &zeconfig.FirstPassResult{
		External: []zePlugin.PluginConfig{{Name: "absent-plugin", Run: "./no-such-plugin-program"}},
	}
	prepared := 0

	declarations, err := joinDeclaredPlugins(context.Background(), firstPass, refusingSpawner{cause: cause}, func() error {
		prepared++
		return nil
	})

	require.NoError(t, err, "a plugin that did not declare refuses nothing by itself")
	assert.Equal(t, 1, prepared)
	require.Len(t, declarations, 1)
	require.ErrorIs(t, declarations[0].Err, cause)
	modules, published := yang.PublishedExternalModules()
	assert.True(t, published)
	assert.Empty(t, modules)

	lines := undeclaredPluginLines(declarations)
	assert.Contains(t, lines, `only plugin "absent-plugin" defines is an unknown keyword`)
	assert.Contains(t, lines, `fix: ze show plugin conformance run "./no-such-plugin-program"`)
}

// TestPrepareFailureRefusesTheBoot proves a boot that cannot give the plugins
// a hub to connect back to is refused before anything is started or published.
func TestPrepareFailureRefusesTheBoot(t *testing.T) {
	restore := yang.ResetPublishedModulesForTest()
	defer restore()

	cause := errors.New("local certificate authority: unreadable")
	firstPass := &zeconfig.FirstPassResult{
		External: []zePlugin.PluginConfig{{Name: "acme-monitor", Run: "acme-monitor-plugin"}},
	}

	declarations, err := joinDeclaredPlugins(context.Background(), firstPass, untouchedSpawner{t: t}, func() error { return cause })

	require.ErrorIs(t, err, cause)
	assert.Nil(t, declarations)
	_, published := yang.PublishedExternalModules()
	assert.False(t, published)
}

// TestDeclaredModulesTakesOnlyDeclaredModuleTexts proves the join is fed by
// the declarations that carry a module text and by nothing else.
func TestDeclaredModulesTakesOnlyDeclaredModuleTexts(t *testing.T) {
	declarations := []pluginserver.EarlyDeclaration{
		{Plugin: "acme-monitor", Schema: &rpc.SchemaDecl{Module: "acme-monitor-conf", Namespace: "urn:acme:monitor", YANGText: "module acme-monitor-conf {}"}},
		{Plugin: "no-schema"},
		{Plugin: "handlers-only", Schema: &rpc.SchemaDecl{Module: "handlers-only-conf"}},
		{Plugin: "silent", Err: errors.New("sent no declaration")},
	}

	modules := declaredModules(declarations)

	assert.Equal(t, []yang.ExternalModule{{
		Plugin:    "acme-monitor",
		Module:    "acme-monitor-conf",
		Namespace: "urn:acme:monitor",
		Text:      "module acme-monitor-conf {}",
	}}, modules)
	assert.Empty(t, undeclaredPluginLines(declarations[:3]), "plugins that declared give the load nothing to add")
}

// TestPluginServerConfigsCarriesTheWorkDirectory proves the declared plugins
// and the full plugin list are built alike, so the config a plugin was forked
// with before the full parse is the one its startup phase names.
func TestPluginServerConfigsCarriesTheWorkDirectory(t *testing.T) {
	plugins := []zePlugin.PluginConfig{{
		Name:          "acme-monitor",
		Run:           "acme-monitor-plugin",
		Encoder:       "json",
		Respawn:       zePlugin.RespawnDeclined,
		ReceiveUpdate: true,
		StageTimeout:  7 * time.Second,
	}}

	configs := pluginServerConfigs(plugins, "/etc/ze")

	want := plugins[0]
	want.WorkDir = "/etc/ze"
	assert.Equal(t, []zePlugin.PluginConfig{want}, configs)
}
