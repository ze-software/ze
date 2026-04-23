// Design: docs/research/vpp-deployment-reference.md -- VPP process lifecycle and production values
// Detail: config.go -- VPPSettings parsed from YANG config
// Detail: startupconf.go -- startup.conf generation
// Detail: dpdk.go -- DPDK NIC driver binding
// Detail: conn.go -- GoVPP connection management

package vpp

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	vppevents "codeberg.org/thomas-mangin/ze/internal/component/vpp/events"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// loggerPtr is the package-level logger, disabled by default.
var loggerPtr atomic.Pointer[slog.Logger]

func init() {
	d := slogutil.DiscardLogger()
	loggerPtr.Store(d)
	setGovppLoggers(d)
}

func logger() *slog.Logger { return loggerPtr.Load() }

// SetVPPLogger sets the package-level logger for the VPP component.
func SetVPPLogger(l *slog.Logger) {
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

// SetVPPEventBus sets the package-level EventBus reference.
// MUST be called before VPPManager.Run starts.
func SetVPPEventBus(eb ze.EventBus) {
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

// VPPManager is the self-contained VPP lifecycle manager.
// It owns startup, health monitoring, crash recovery, and clean shutdown.
// Call Run(ctx) to start the lifecycle loop. Run blocks until ctx is canceled.
type VPPManager struct {
	settings   *VPPSettings
	dpdk       *DPDKBinder
	connector  *Connector
	confDir    string // directory to write startup.conf
	vppBinary  string // path to VPP binary
	hasRunOnce bool   // tracks first vs subsequent starts for connected/reconnected events
}

// NewVPPManager creates a VPP lifecycle manager from parsed settings.
// confDir is the directory where startup.conf will be written.
// vppBinary is the path to the VPP executable.
func NewVPPManager(settings *VPPSettings, confDir, vppBinary string) *VPPManager {
	return &VPPManager{
		settings:  settings,
		dpdk:      NewDPDKBinder(),
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
// emit ("vpp","connected"), monitor. On crash: emit ("vpp","disconnected"),
// backoff, restart, emit ("vpp","reconnected"). On ctx cancel: SIGTERM VPP,
// wait, unbind NICs.
func (m *VPPManager) Run(ctx context.Context) error {
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
	if !m.settings.External {
		if err := m.writeStartupConf(confPath); err != nil {
			return fmt.Errorf("vpp: write startup.conf: %w", err)
		}
		lg.Info("vpp: startup.conf written", "path", confPath)

		if err := m.dpdk.BindAll(m.settings.DPDK.Interfaces); err != nil {
			return fmt.Errorf("vpp: dpdk bind: %w", err)
		}
		lg.Info("vpp: DPDK NICs bound", "count", len(m.settings.DPDK.Interfaces))
	} else {
		lg.Info("vpp: external mode, skipping startup.conf + DPDK bind",
			"reason", "external supervisor owns VPP lifecycle")
	}

	// Register connector so dependent plugins can access it via GetActiveConnector().
	setActiveConnector(m.connector)
	defer setActiveConnector(nil)

	defer func() {
		if err := m.dpdk.UnbindAll(); err != nil {
			lg.Error("vpp: dpdk unbind failed", "error", err)
		}
	}()

	backoff := time.Second
	for {
		err := m.runOnce(ctx, confPath)
		if ctx.Err() != nil {
			lg.Info("vpp: shutdown requested")
			m.connector.Close()
			return nil
		}

		lg.Error("vpp: process exited unexpectedly", "error", err, "backoff", backoff)
		m.emitEvent(vppevents.EventDisconnected)

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
// Returns a non-nil error when VPP exited unexpectedly; returns ctx.Err when
// ctx was canceled (the outer loop converts ctx.Err to a clean shutdown).
func (m *VPPManager) runOnce(ctx context.Context, confPath string) error {
	lg := logger()

	var cmd *exec.Cmd
	if !m.settings.External {
		cmd = exec.CommandContext(ctx, m.vppBinary, "-c", confPath) //nolint:gosec // vppBinary is set at registration, not user input
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Start(); err != nil {
			return fmt.Errorf("start vpp: %w", err)
		}
		lg.Info("vpp: process started", "pid", cmd.Process.Pid)
	} else {
		lg.Info("vpp: external mode, not execing VPP binary", "socket", m.settings.APISocket)
	}

	// Give VPP time to initialize and create the API socket.
	connectCtx, connectCancel := context.WithTimeout(ctx, 30*time.Second)
	defer connectCancel()

	if err := m.connector.Connect(connectCtx, 10, time.Second); err != nil {
		if cmd != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
		return fmt.Errorf("govpp connect: %w", err)
	}
	lg.Info("vpp: GoVPP connected", "socket", m.settings.APISocket)

	if m.hasRunOnce {
		m.emitEvent(vppevents.EventReconnected)
	} else {
		m.emitEvent(vppevents.EventConnected)
		m.hasRunOnce = true
	}

	// Start stats telemetry poller if metrics registry and stats connection are available.
	var pollerCancel context.CancelFunc
	var statsConn statsDisconnector
	if reg := getMetricsRegistry(); reg != nil {
		sc, statsErr := connectStats(m.settings.Stats.SocketPath)
		if statsErr != nil {
			lg.Warn("vpp: stats connection failed, telemetry disabled", "error", statsErr)
		} else {
			pollerCtx, cancel := context.WithCancel(ctx)
			pollerCancel = cancel
			statsConn = sc
			interval := time.Duration(m.settings.Stats.PollInterval) * time.Second
			vppM := newVPPMetrics(reg)
			poller := newStatsPoller(sc, vppM, interval)
			go poller.run(pollerCtx)
			lg.Info("vpp: stats telemetry started", "interval", interval)
		}
	}

	var err error
	if cmd != nil {
		// Wait for VPP process to exit.
		err = cmd.Wait()
	} else {
		// External mode: block on ctx.Done; the external supervisor owns VPP lifecycle.
		<-ctx.Done()
		err = ctx.Err()
	}
	if pollerCancel != nil {
		pollerCancel()
	}
	if statsConn != nil {
		statsConn.Disconnect()
	}
	m.connector.Close()
	if cmd != nil {
		return fmt.Errorf("vpp process exited: %w", err)
	}
	return err
}

// writeStartupConf generates and writes the VPP startup.conf file.
func (m *VPPManager) writeStartupConf(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	f, err := os.Create(path) //nolint:gosec // path is confDir + "startup.conf", not user input
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	return GenerateStartupConf(f, m.settings)
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
