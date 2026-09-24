// Design: docs/architecture/wire/nlri-bgpls.md -- native EPE plugin lifecycle

package ls

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"sync"

	"github.com/ze-software/ze/internal/component/bgp"
	lsyang "github.com/ze-software/ze/internal/component/bgp/plugins/nlri/ls/yang"
	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/linkstateevents"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

var epeLogger = slog.Default()

func init() {
	reg := registry.Registration{Name: epeName, Description: "Native BGP Egress Peer Engineering segments",
		RFCs: []string{"9086"}, Features: "yang", YANG: lsyang.ZeBGPEpeConfYANG, ConfigRoots: []string{epeName},
		NeedsDataPlane: true, RunEngine: runEPEProducer,
		ConfigureEngineLogger:   func(name string) { epeLogger = slogutil.Logger(name) },
		InProcessConfigVerifier: func(sections []rpc.ConfigSection) error { _, err := parseEPEConfig(sections); return err }}
	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		cfg.GetYANG = func() string { return lsyang.ZeBGPEpeConfYANG }
		cfg.ConfigLogger = func(level string) { epeLogger = slogutil.PluginLogger(reg.Name, level) }
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		panic(fmt.Sprintf("BUG: register BGP EPE producer: %v", err))
	}
}

func runEPEProducer(conn net.Conn) int {
	p := sdk.NewWithConn(epeName, conn)
	defer func() { _ = p.Close() }()
	bus := registry.GetEventBus()
	if bus == nil {
		epeLogger.Error("native BGP EPE requires an internal plugin with the engine event bus")
		return 1
	}
	source := newEPESource(bus)
	defer func() {
		if err := source.stop(); err != nil {
			epeLogger.Error("BGP EPE teardown failed", "error", err)
		}
	}()
	p.OnBye(func(_ string) error {
		return source.stop()
	})
	unsubscribe := linkstateevents.Request.Subscribe(bus, func() {
		if err := source.replay(); err != nil {
			epeLogger.Error("BGP EPE snapshot failed", "error", err)
		}
	})
	defer unsubscribe()
	var configMu sync.Mutex
	var current, candidate, previous epeConfig
	var staged, applied bool
	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		cfg, err := parseEPEConfig(sections)
		if err != nil {
			return err
		}
		if err := source.configure(cfg); err != nil {
			return err
		}
		configMu.Lock()
		current = cfg
		configMu.Unlock()
		return nil
	})
	p.OnConfigVerify(func(sections []sdk.ConfigSection) error {
		cfg, err := parseEPEConfig(sections)
		if err != nil {
			return err
		}
		configMu.Lock()
		candidate, staged, applied = cfg, true, false
		configMu.Unlock()
		return nil
	})
	p.OnConfigApply(func(_ []sdk.ConfigDiffSection) error {
		configMu.Lock()
		if !staged {
			configMu.Unlock()
			return errors.New("BGP EPE apply without verified configuration")
		}
		previous, current, staged, applied = current, candidate, false, true
		cfg := current
		configMu.Unlock()
		return source.configure(cfg)
	})
	p.OnConfigRollback(func(_ string) error {
		configMu.Lock()
		if applied {
			current, applied = previous, false
		}
		staged = false
		cfg := current
		configMu.Unlock()
		return source.configure(cfg)
	})
	p.SetStartupSubscriptions([]string{"state"}, []string{"*"}, "json")
	p.OnEvent(func(payload string) error {
		event, err := bgp.ParseEvent([]byte(payload))
		if err != nil {
			return err
		}
		if event.GetEventType() != rpc.EventKindState {
			return nil
		}
		return source.state(event)
	})
	p.OnAllPluginsReady(source.replay)
	ctx, cancel := sdk.SignalContext()
	defer cancel()
	if err := p.Run(ctx, sdk.Registration{WantsConfig: []string{epeName}}); err != nil {
		epeLogger.Error("BGP EPE producer stopped", "error", err)
		return 1
	}
	return 0
}
