// Design: docs/research/vpp-deployment-reference.md -- VPP process lifecycle and production values
// Detail: config.go -- VPPSettings parsed from YANG config
// Detail: startupconf.go -- startup.conf generation
// Detail: dpdk.go -- DPDK NIC driver binding
// Detail: conn.go -- GoVPP connection management

package vpp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"go.fd.io/govpp/api"

	"github.com/ze-software/ze/internal/core/slogutil"
	vppevents "github.com/ze-software/ze/internal/core/vpp/events"
	"github.com/ze-software/ze/pkg/ze"
)

// loggerPtr is the package-level logger, disabled by default.
var loggerPtr atomic.Pointer[slog.Logger]

func init() {
	d := slogutil.DiscardLogger()
	loggerPtr.Store(d)
	setGovppLoggers(d)
}

func logger() *slog.Logger { return loggerPtr.Load() }

// setVPPLogger sets the package-level logger for the VPP component.
func setVPPLogger(l *slog.Logger) {
	if l != nil {
		loggerPtr.Store(l)
		setGovppLoggers(l)
	}
}

// eventBusMu guards eventBusRef.
var (
	eventBusMu  sync.Mutex
	eventBusRef ze.EventBus
)

// setVPPEventBus sets the package-level EventBus reference.
// MUST be called before VPPManager.Run starts.
func setVPPEventBus(eb ze.EventBus) {
	eventBusMu.Lock()
	defer eventBusMu.Unlock()
	eventBusRef = eb
}

func getEventBus() ze.EventBus {
	eventBusMu.Lock()
	defer eventBusMu.Unlock()
	return eventBusRef
}

// connectorMu guards the package-level connector reference.
var (
	connectorMu  sync.Mutex
	connectorRef *Connector
)

// GetActiveConnector returns the GoVPP connector from the running VPP Manager.
// Dependent plugins (fibvpp, ifacevpp) call this to get API channels.
// Returns nil if the VPP component is not running.
func GetActiveConnector() *Connector {
	connectorMu.Lock()
	defer connectorMu.Unlock()
	return connectorRef
}

func setActiveConnector(c *Connector) {
	connectorMu.Lock()
	defer connectorMu.Unlock()
	connectorRef = c
}

// activeLCP holds the LCP settings of the running VPP Manager so dependent
// plugins (ifacevpp) can read the configured netns and enabled flag when
// creating Linux Control Plane pairs. Guarded by connectorMu since it is set
// and cleared alongside the connector in Run.
var (
	activeLCP    LCPSettings
	activeLCPSet bool
)

// GetActiveLCPSettings returns the LCP settings of the running VPP Manager and
// whether they are available. Returns ok=false when the VPP component is not
// running, so callers treat LCP as unavailable rather than assuming defaults.
func GetActiveLCPSettings() (LCPSettings, bool) {
	connectorMu.Lock()
	defer connectorMu.Unlock()
	return activeLCP, activeLCPSet
}

func setActiveLCPSettings(lcp LCPSettings, ok bool) {
	connectorMu.Lock()
	defer connectorMu.Unlock()
	activeLCP = lcp
	activeLCPSet = ok
}

// IfaceStatsReader is the subset of the VPP stats provider that dependent
// plugins (ifacevpp) need to read per-interface counters.
type IfaceStatsReader interface {
	GetInterfaceStats(*api.InterfaceStats) error
}

var (
	statsProviderMu  sync.Mutex
	statsProviderRef IfaceStatsReader
)

// GetActiveStatsProvider returns the VPP stats reader from the running VPP
// Manager. Dependent plugins (ifacevpp) call this to read per-interface
// counters. Returns nil if the VPP component has no stats connection.
func GetActiveStatsProvider() IfaceStatsReader {
	statsProviderMu.Lock()
	defer statsProviderMu.Unlock()
	return statsProviderRef
}

func setActiveStatsProvider(sp IfaceStatsReader) {
	statsProviderMu.Lock()
	defer statsProviderMu.Unlock()
	statsProviderRef = sp
}

// VPPManager is the self-contained VPP lifecycle manager.
// It owns startup, health monitoring, crash recovery, and clean shutdown.
// Call Run(ctx) to start the lifecycle loop. Run blocks until ctx is canceled.
type VPPManager struct {
	settings   *VPPSettings
	dpdk       *DPDKBinder
	connector  *Connector
	confDir    string       // directory to write startup.conf
	vppBinary  string       // path to VPP binary
	hasRunOnce bool         // tracks first vs subsequent starts for connected/reconnected events
	ready      bool         // this manager currently owns one published readiness lifetime
	startup    chan<- error // configured before Run; one buffered startup receipt
}

// NewVPPManager creates a VPP lifecycle manager from parsed settings.
// confDir is the directory where startup.conf will be written.
// vppBinary is the path to the VPP executable.
func NewVPPManager(settings *VPPSettings, confDir, vppBinary string) *VPPManager {
	return &VPPManager{
		settings:  settings,
		dpdk:      newDPDKBinder(),
		connector: NewConnector(settings.APISocket),
		confDir:   confDir,
		vppBinary: vppBinary,
	}
}

// maxRestartBackoff caps the exponential backoff for VPP crash restarts.
const maxRestartBackoff = 30 * time.Second

// Run is the self-contained lifecycle loop. It blocks until ctx is canceled.
//
// Sequence: generate startup.conf, bind DPDK NICs, exec VPP, connect GoVPP,
// bind the original child, complete its configured startup CLI, activate
// statistics, emit ("vpp","connected"), monitor. On crash: emit ("vpp","disconnected"),
// backoff, restart, emit ("vpp","reconnected"). On ctx cancel: kill the
// original supervised child, join its wait, then restore the NIC drivers.
func (m *VPPManager) Run(ctx context.Context) (failure error) {
	return m.runWithOps(ctx, managerRunOps{bindAll: m.dpdk.BindAll, runOnce: m.runOnce, unbindAll: m.dpdk.UnbindAll})
}

// managerRunOps fixes the native run and restoration boundaries for one run.
// All callbacks MUST belong to the same manager and MUST be nonnil.
type managerRunOps struct {
	bindAll   func([]DPDKInterface) error
	runOnce   func(context.Context, string) error
	unbindAll func() error
}

// runWithOps runs the same lifecycle through its captured operation boundaries.
// The caller MUST supply complete operations before starting this serial run.
func (m *VPPManager) runWithOps(ctx context.Context, ops managerRunOps) (failure error) {
	defer func() { m.notifyStartup(failure) }()
	lg := logger()

	if !m.settings.Enabled {
		lg.Info("vpp: disabled, exiting")
		<-ctx.Done()
		return nil
	}

	if err := m.settings.Validate(); err != nil {
		return fmt.Errorf("vpp: config validation: %w", err)
	}

	confPath := filepath.Join(m.confDir, "startup.conf")
	if m.settings.External {
		lg.Info("vpp: external mode, skipping startup.conf + DPDK bind",
			"reason", "external supervisor owns VPP lifecycle")
	} else {
		if err := checkManagedStartupCLI(m.settings, openStartupCLIFile); err != nil {
			return err
		}
		if err := m.writeStartupConf(confPath); err != nil {
			return fmt.Errorf("vpp: write startup.conf: %w", err)
		}
		lg.Info("vpp: startup.conf written", "path", confPath)

		if err := ops.bindAll(m.settings.DPDK.Interfaces); err != nil {
			failure := fmt.Errorf("vpp: dpdk bind: %w", err)
			if len(m.dpdk.savedDrivers) != 0 {
				return m.retainNativeCleanup(failure, nil, ops.unbindAll)
			}
			return failure
		}
		lg.Info("vpp: DPDK NICs bound", "count", len(m.settings.DPDK.Interfaces))
	}

	// Register connector so dependent plugins can access it via GetActiveConnector().
	setActiveConnector(m.connector)
	defer setActiveConnector(nil)

	// Publish the LCP settings so ifacevpp can create Linux Control Plane pairs
	// in the configured netns when the iface backend is vpp.
	setActiveLCPSettings(m.settings.LCP, true)
	defer setActiveLCPSettings(LCPSettings{}, false)

	defer func() {
		var childFailure error
		if _, ok := errors.AsType[nativeCleanupError](failure); ok {
			childFailure = failure
		}
		if err := ops.unbindAll(); err != nil {
			lg.Error("vpp: dpdk unbind failed", "error", err)
			failure = m.retainNativeCleanup(errors.Join(failure, fmt.Errorf("vpp: dpdk unbind: %w", err)), childFailure, ops.unbindAll)
		} else if childFailure != nil {
			failure = m.retainNativeCleanup(failure, childFailure, ops.unbindAll)
		}
	}()

	backoff := time.Second
	for {
		err := ops.runOnce(ctx, confPath)
		m.notifyStartup(err)
		if ctx.Err() != nil {
			lg.Info("vpp: shutdown requested")
			m.connector.Close()
			if err == ctx.Err() { //nolint:errorlint // Only exact ctx.Err identity is a clean shutdown; a joined cleanup cause must survive.
				return nil
			}
			return err
		}
		if _, ok := errors.AsType[nativeCleanupError](err); ok {
			m.connector.Close()
			return err
		}

		lg.Error("vpp: process exited unexpectedly", "error", err, "backoff", backoff)

		select {
		case <-time.After(backoff):
			backoff = min(backoff*2, maxRestartBackoff)
		case <-ctx.Done():
			return nil
		}
	}
}

// GetConnector returns the GoVPP connector for dependent plugins.
// Dependents call connector.NewChannel() to get API channels.
func (m *VPPManager) GetConnector() *Connector {
	return m.connector
}

// IsConnected returns whether the GoVPP connection is established.
func (m *VPPManager) IsConnected() bool {
	return m.connector.IsConnected()
}

// runOnce starts VPP (or skips exec when external=true), connects GoVPP, and
// blocks until the VPP process exits or ctx is canceled.
// Expected original-child shutdown returns the exact ctx.Err value. Native
// cleanup failures retain their separate cause and prevent another child run.
func (m *VPPManager) runOnce(ctx context.Context, confPath string) (failure error) {
	lg := logger()

	var cmd *exec.Cmd
	var stop *managedChildStop
	var child *processWitness
	if m.settings.External {
		lg.Info("vpp: external mode, not execing VPP binary", "socket", m.settings.APISocket)
	} else {
		if err := checkManagedStartupCLI(m.settings, openStartupCLIFile); err != nil {
			return err
		}
		cmd = exec.CommandContext(ctx, m.vppBinary, "-c", confPath) //nolint:gosec // vppBinary is set at registration, not user input
		stop = trackManagedChildStop(cmd)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		var err error
		child, err = startManagedProcess(cmd)
		if err != nil {
			if cmd.Process == nil {
				if err == ctx.Err() { //nolint:errorlint // Only exact ctx.Err identity with no started child proves clean pre-start cancellation; a wrapped or joined startup or cleanup cause must survive.
					return err
				}
			}
			return fmt.Errorf("start vpp: %w", err)
		}
		defer func() {
			if err := child.release(); err != nil {
				failure = errors.Join(failure, nativeCleanupError{cause: fmt.Errorf("vpp: release child witness: %w", err)})
			}
		}()
		lg.Info("vpp: process started", "pid", cmd.Process.Pid)
	}

	// Give VPP time to initialize and create the API socket.
	connectCtx, connectCancel := context.WithTimeout(ctx, 30*time.Second)
	defer connectCancel()

	if err := m.connector.Connect(connectCtx, 10, time.Second); err != nil {
		var cleanup error
		if cmd != nil {
			cleanup = stopVPPChildWithStop(ctx, cmd, stop)
			m.connector.Close()
		}
		if cleanup == nil {
			if err == ctx.Err() { //nolint:errorlint // Only exact ctx.Err identity is clean cancellation; a joined connect or cleanup cause must survive.
				return err
			}
		}
		return errors.Join(fmt.Errorf("vpp: connect: %w", err), cleanup)
	}
	lg.Info("vpp: GoVPP connected", "socket", m.settings.APISocket)
	var owned managedConnection
	if cmd != nil {
		var err error
		owned, err = m.connector.bindManagedProcess(ctx, child)
		if err != nil {
			cleanup := stopVPPChildWithStop(ctx, cmd, stop)
			m.connector.Close()
			if cleanup == nil {
				return err
			}
			return errors.Join(err, cleanup)
		}
	}

	// Counters serve runtime consumers as well as optional telemetry. Publish
	// their owned connection (or its failure) before any readiness observer.
	statsGeneration, statsGenerationErr := m.connector.ConnectionGeneration()
	stats, err := m.activateManagedRuntime(ctx, owned, func(path string) (runtimeStatsClient, error) {
		if statsGenerationErr != nil {
			return nil, statsGenerationErr
		}
		return connectStats(path, m.connector, statsGeneration)
	})
	if err != nil {
		if cmd != nil {
			cleanup := stopVPPChildWithStop(ctx, cmd, stop)
			// Join the exact child before closing its matched transport. Its
			// retained physical observer alone publishes process death.
			m.connector.retireManagedConnection(owned)
			if cleanup == nil {
				return err
			}
			return errors.Join(err, cleanup)
		}
		m.connector.Close()
		return err
	}
	defer m.retireReadiness()

	loss, err := m.connector.ConnectionLoss()
	if err == nil {
		err = waitVPPConnectionWithStop(ctx, cmd, loss, stop)
	} else if cmd != nil {
		err = errors.Join(err, stopVPPChildWithStop(ctx, cmd, stop))
	}
	if stats != nil {
		stats.Close()
	}
	if cmd != nil {
		// waitVPPConnection joins cmd.Wait on process exit, API loss and
		// shutdown. The physical observer independently proves process death.
		m.connector.retireManagedConnection(owned)
		if err != nil {
			if err == ctx.Err() { //nolint:errorlint // Only exact ctx.Err identity is clean cancellation; a joined exit or cleanup cause must survive.
				return err
			}
		}
		if err == nil {
			return errors.New("vpp process exited")
		}
		return fmt.Errorf("vpp process exited: %w", err)
	}
	m.connector.Close()
	return err
}

// waitVPPConnection owns one process-wait bridge for the supervised process
// lifetime. It MUST join that bridge before returning. External mode has no
// process to reap and MUST NOT manufacture process-exit evidence on API loss.
func waitVPPConnection(ctx context.Context, cmd *exec.Cmd, loss <-chan error) error {
	return waitVPPConnectionWithStop(ctx, cmd, loss, nil)
}

func waitVPPConnectionWithStop(ctx context.Context, cmd *exec.Cmd, loss <-chan error, stop *managedChildStop) error {
	if cmd == nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-loss:
			return err
		}
	}
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	var failure error
	select {
	case err := <-exited:
		if stop.confirmed() && expectedVPPKill(cmd, err) {
			return ctx.Err()
		}
		return managedVPPWaitResult(ctx, cmd, err)
	case <-ctx.Done():
		failure = ctx.Err()
	case failure = <-loss:
	}
	// The API may be lost while its VPP process is still alive. Reap that
	// exact owned child before restarting; a changed PID or socket is no proof.
	killErr := cmd.Process.Kill()
	waitErr := <-exited
	cleanup := managedChildCleanup(ctx, cmd, killErr, waitErr, stop.confirmed())
	if cleanup == nil {
		return failure
	}
	return errors.Join(failure, cleanup)
}

// writeStartupConf generates and writes the VPP startup.conf file. The host CPU
// inventory is read here, so the worker core list names the CPUs this machine
// isolated rather than the ones a config author guessed at.
func (m *VPPManager) writeStartupConf(path string) error {
	inv, err := hostCPUInventory()
	if err != nil {
		return fmt.Errorf("vpp: read host CPU inventory: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	f, err := os.Create(path) //nolint:gosec // path is confDir + "startup.conf", not user input
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	return GenerateStartupConf(f, m.settings, inv)
}

// emitEvent emits a VPP lifecycle event on the EventBus.
func (m *VPPManager) emitEvent(eventType string) {
	eb := getEventBus()
	if eb == nil {
		return
	}
	if _, err := eb.Emit(vppevents.Namespace, eventType, ""); err != nil {
		logger().Error("vpp: emit event failed", "event", eventType, "error", err)
	}
}
