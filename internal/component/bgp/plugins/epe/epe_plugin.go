// Design: docs/architecture/wire/nlri-bgpls.md -- native EPE plugin lifecycle

package epe

import (
	"errors"
	"fmt"
	"net"
	"sync"

	"github.com/ze-software/ze/internal/component/bgp"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/linkstateevents"
	"github.com/ze-software/ze/internal/core/mplsfib"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func runEPEProducer(conn net.Conn) int {
	p := sdk.NewWithConn(Name, conn)
	defer func() { _ = p.Close() }()
	bus := registry.GetEventBus()
	if bus == nil {
		epeLogger.Error("native BGP EPE requires an internal plugin with the engine event bus")
		return 1
	}
	source := NewSource(bus)
	defer func() {
		if err := source.Stop(); err != nil {
			epeLogger.Error("BGP EPE teardown failed", "error", err)
		}
	}()
	p.OnBye(func(_ string) error {
		return source.Stop()
	})
	unsubscribe := linkstateevents.Request.Subscribe(bus, func() {
		if err := source.Replay(); err != nil {
			epeLogger.Error("BGP EPE snapshot failed", "error", err)
		}
	})
	defer unsubscribe()
	// A label writer that opens after EPE installed its labels (fib-vpp once
	// VPP connects, or after VPP restarted with an empty FIB) asks for them
	// again. The deferred unsubscribe runs before the deferred Stop, so a
	// writer that opens during teardown is not handed labels being withdrawn.
	unsubscribeLabels := mplsfib.ReplayRequest.Subscribe(bus, source.ReplayLabels)
	defer unsubscribeLabels()
	var configMu sync.Mutex
	var current, candidate, previous Config
	var staged, applied bool
	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		cfg, err := ParseConfig(sections)
		if err != nil {
			return err
		}
		if err := source.Configure(cfg); err != nil {
			return err
		}
		configMu.Lock()
		current = cfg
		configMu.Unlock()
		return nil
	})
	p.OnConfigVerify(func(sections []sdk.ConfigSection) error {
		cfg, err := ParseConfig(sections)
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
		return source.Configure(cfg)
	})
	p.OnConfigRollback(func(_ string) error {
		configMu.Lock()
		if applied {
			current, applied = previous, false
		}
		staged = false
		cfg := current
		configMu.Unlock()
		return source.Configure(cfg)
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
		return source.State(event)
	})
	p.OnAllPluginsReady(source.Replay)
	// The bgp-epe-label-entries check reads the refusals EPE's own process
	// holds, so it is answered here.
	p.OnDoctorCheck(func(name string) ([]rpc.DoctorCheckDiagnostic, error) {
		if name != labelEntriesCheckName {
			return nil, fmt.Errorf("unknown %s doctor check %s", Name, name)
		}
		return source.labelDoctorFindings(), nil
	})
	ctx, cancel := sdk.SignalContext()
	defer cancel()
	if err := p.Run(ctx, &sdk.Registration{WantsConfig: []string{Name},
		DoctorChecks: []rpc.DoctorCheckDecl{labelEntriesCheckDecl}}); err != nil {
		epeLogger.Error("BGP EPE producer stopped", "error", err)
		return 1
	}
	return 0
}
