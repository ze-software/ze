package server

import (
	"context"
	"fmt"
	"net"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func orderingTestEngine(name string, configure func() error) func(net.Conn) int {
	return func(conn net.Conn) int {
		p := sdk.NewWithConn(name, conn)
		defer p.Close() //nolint:errcheck // Test transport cleanup.
		p.OnConfigure(func([]sdk.ConfigSection) error { return configure() })
		if err := p.Run(context.Background(), sdk.Registration{}); err != nil {
			return 1
		}
		return 0
	}
}

// These are real SDK engines driven through the daemon's startup selection and
// lifecycle spawner. The consumer refuses Configure unless the selected
// provider has published; direct calls to TopologicalTiers cannot satisfy it.
func TestStartupOrdersSelectedExplicitProviderBeforeConfigConsumer(t *testing.T) {
	for _, selected := range []bool{false, true} {
		t.Run(fmt.Sprintf("provider-selected-%t", selected), func(t *testing.T) {
			saved := registry.Snapshot()
			registry.Reset()
			t.Cleanup(func() { registry.Restore(saved) })
			var providerReady, consumerReady, unrelatedReady atomic.Bool
			const provider = "selected-order-provider"
			const consumer = "selected-order-consumer"
			const unrelated = "selected-order-unrelated"
			for _, reg := range []registry.Registration{
				{
					Name: provider, Description: "publishes interface facts", ConfigRoots: []string{"provider-root"},
					CLIHandler: func([]string) int { return 0 },
					RunEngine:  orderingTestEngine(provider, func() error { providerReady.Store(true); return nil }),
				},
				{
					Name: consumer, Description: "consumes published facts", ConfigRoots: []string{"consumer-root"},
					StartAfter: []string{provider}, CLIHandler: func([]string) int { return 0 },
					RunEngine: orderingTestEngine(consumer, func() error {
						if selected && !providerReady.Load() {
							return fmt.Errorf("selected provider has not published")
						}
						if unrelatedReady.Load() {
							return fmt.Errorf("unrelated explicit plugin moved ahead of config infrastructure")
						}
						consumerReady.Store(true)
						return nil
					}),
				},
				{
					Name: unrelated, Description: "ordinary explicit plugin", CLIHandler: func([]string) int { return 0 },
					RunEngine: orderingTestEngine(unrelated, func() error {
						if !consumerReady.Load() {
							return fmt.Errorf("config infrastructure was not started first")
						}
						unrelatedReady.Store(true)
						return nil
					}),
				},
			} {
				require.NoError(t, registry.Register(reg))
			}
			s, spawner := newLifecycleStartupServer(t)
			s.config.ConfiguredPaths = []string{"consumer-root"}
			s.config.Plugins = []plugin.PluginConfig{{Name: unrelated, Internal: true, Encoder: "json"}}
			if selected {
				s.config.ConfiguredPaths = append(s.config.ConfiguredPaths, "provider-root")
				s.config.Plugins = append(s.config.Plugins, plugin.PluginConfig{Name: provider, Internal: true, Encoder: "json"})
			}
			s.wg.Add(1)
			s.runPluginStartup()
			require.NoError(t, s.startupErr)
			require.True(t, consumerReady.Load(), "consumer Configure must execute successfully")
			require.True(t, unrelatedReady.Load(), "remaining explicit phase must execute")
			require.Equal(t, selected, providerReady.Load())
			if !selected {
				require.Nil(t, spawner.pm.GetProcess(provider), "an order-only target must not be activated")
			}
		})
	}
}

func TestStartupSelectedCycleFailsBeforeSpawning(t *testing.T) {
	for _, aliases := range []int{0, 1, 2} {
		t.Run(fmt.Sprintf("aliases-%d", aliases), func(t *testing.T) {
			saved := registry.Snapshot()
			registry.Reset()
			t.Cleanup(func() { registry.Restore(saved) })
			for _, reg := range []registry.Registration{
				{Name: "cycle-auto", Description: "auto", ConfigRoots: []string{"cycle-root"}, StartAfter: []string{"cycle-explicit"}},
				{Name: "cycle-explicit", Description: "explicit", StartAfter: []string{"cycle-auto"}},
			} {
				reg.CLIHandler = func([]string) int { return 0 }
				reg.RunEngine = func(net.Conn) int { t.Error("cyclic startup must not spawn"); return 1 }
				require.NoError(t, registry.Register(reg))
			}
			s, spawner := newLifecycleStartupServer(t)
			s.config.ConfiguredPaths = []string{"cycle-root"}
			if aliases == 0 {
				s.config.Plugins = []plugin.PluginConfig{{Name: "cycle-explicit", Internal: true}}
			} else {
				for i := range aliases {
					s.config.Plugins = append(s.config.Plugins, plugin.PluginConfig{
						Name: fmt.Sprintf("cycle-provider-%d", i), Run: "cycle-explicit", Internal: true,
					})
				}
				if aliases > 1 {
					s.config.Plugins = append(s.config.Plugins, plugin.PluginConfig{
						Name: "cycle-consumer", Run: "cycle-auto", Internal: true,
					})
				}
			}
			s.wg.Add(1)
			s.runPluginStartup()
			require.ErrorIs(t, s.startupErr, registry.ErrCircularDependency)
			require.Nil(t, spawner.pm)
		})
	}
}

// The automatic front plugin promotes an explicit consumer, which must in
// turn promote every selected provider instance. Engines run through the real
// SDK Configure boundary, so correct phase selection alone cannot pass this.
func TestStartupOrdersAliasedImplementations(t *testing.T) {
	for _, tc := range []struct {
		name      string
		providers []string
		consumers []string
	}{
		{"provider-alias", []string{"links"}, []string{"order-consumer"}},
		{"consumer-alias", []string{"order-provider"}, []string{"sessions"}},
		{"both-aliases", []string{"links"}, []string{"sessions"}},
		{"multiple-aliases", []string{"links-a", "links-b"}, []string{"sessions-a", "sessions-b"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			saved := registry.Snapshot()
			registry.Reset()
			t.Cleanup(func() { registry.Restore(saved) })
			var providers, consumers atomic.Int32
			var frontReady, unrelatedReady atomic.Bool
			for _, reg := range []registry.Registration{
				{
					Name: "order-provider", Description: "selected provider",
					RunEngine: orderingTestEngine("order-provider", func() error { providers.Add(1); return nil }),
				},
				{
					Name: "order-consumer", Description: "selected consumer", StartAfter: []string{"order-provider"},
					RunEngine: orderingTestEngine("order-consumer", func() error {
						if got := providers.Load(); got != int32(len(tc.providers)) {
							return fmt.Errorf("only %d selected providers published", got)
						}
						consumers.Add(1)
						return nil
					}),
				},
				{
					Name: "order-front", Description: "automatic front", ConfigRoots: []string{"order-root"},
					StartAfter: []string{"order-consumer"},
					RunEngine: orderingTestEngine("order-front", func() error {
						if got := consumers.Load(); got != int32(len(tc.consumers)) {
							return fmt.Errorf("only %d selected consumers configured", got)
						}
						if unrelatedReady.Load() {
							return fmt.Errorf("unrelated explicit plugin ran before automatic phase")
						}
						frontReady.Store(true)
						return nil
					}),
				},
				{
					Name: "order-unrelated", Description: "remaining explicit",
					RunEngine: orderingTestEngine("order-unrelated", func() error {
						if !frontReady.Load() {
							return fmt.Errorf("automatic front has not configured")
						}
						unrelatedReady.Store(true)
						return nil
					}),
				},
			} {
				reg.CLIHandler = func([]string) int { return 0 }
				require.NoError(t, registry.Register(reg))
			}
			s, spawner := newLifecycleStartupServer(t)
			s.config.ConfiguredPaths = []string{"order-root"}
			s.config.Plugins = []plugin.PluginConfig{{Name: "order-unrelated", Internal: true, Encoder: "json"}}
			for _, name := range tc.consumers {
				s.config.Plugins = append(s.config.Plugins, plugin.PluginConfig{
					Name: name, Run: "order-consumer", Internal: true, Encoder: "json",
				})
			}
			for _, name := range tc.providers {
				s.config.Plugins = append(s.config.Plugins, plugin.PluginConfig{
					Name: name, Run: "order-provider", Internal: true, Encoder: "json",
				})
			}
			s.wg.Add(1)
			s.runPluginStartup()
			require.NoError(t, s.startupErr)
			require.True(t, frontReady.Load())
			require.True(t, unrelatedReady.Load())
			require.EqualValues(t, len(tc.providers), providers.Load())
			require.EqualValues(t, len(tc.consumers), consumers.Load())
			for _, cfg := range s.config.Plugins {
				proc := spawner.pm.GetProcess(cfg.Name)
				require.NotNil(t, proc, "dispatch must keep selected process label %q", cfg.Name)
				require.Equal(t, cfg.Run, proc.Config().Run, "ordering must not rewrite the selected implementation")
			}
		})
	}
}

// The selection boundary must not borrow a same-named compiled registration's
// edges for an explicit external program, nor replace its execution config.
func TestStartupPhaseSelectionPreservesExternalProgramIsolation(t *testing.T) {
	saved := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(saved) })
	for _, reg := range []registry.Registration{
		{Name: "auto", Description: "auto", StartAfter: []string{"external"}},
		{Name: "external", Description: "compiled namesake", StartAfter: []string{"unrelated"}},
		{Name: "unrelated", Description: "unrelated"},
	} {
		reg.CLIHandler = func([]string) int { return 0 }
		reg.RunEngine = func(net.Conn) int { return 0 }
		require.NoError(t, registry.Register(reg))
	}
	external := plugin.PluginConfig{Name: "external", Run: "/chosen/program"}
	unrelated := plugin.PluginConfig{Name: "unrelated", Internal: true}
	first, remaining, err := startupSelectedPhases(
		[]plugin.PluginConfig{{Name: "auto", Internal: true}},
		[]plugin.PluginConfig{external, unrelated},
	)
	require.NoError(t, err)
	require.Len(t, first, 2)
	require.Equal(t, external, first[1], "promote the selected program, not its compiled namesake")
	require.Equal(t, []plugin.PluginConfig{unrelated}, remaining, "external program inherits no compiled edges")
}
