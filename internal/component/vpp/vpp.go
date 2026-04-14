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

	"codeberg.org/thomas-mangin/ze/internal/core/events"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

// loggerPtr is the package-level logger, disabled by default.
var loggerPtr atomic.Pointer[slog.Logger]

func init() {
	d := slogutil.DiscardLogger()
	loggerPtr.Store(d)
}

func logger() *slog.Logger { return loggerPtr.Load() }

// SetVPPLogger sets the package-level logger for the VPP component.
func SetVPPLogger(l *slog.Logger) {
	if l != nil {
		loggerPtr.Store(l)
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
	if err := m.writeStartupConf(confPath); err != nil {
		return fmt.Errorf("vpp: write startup.conf: %w", err)
	}
	lg.Info("vpp: startup.conf written", "path", confPath)

	if err := m.dpdk.BindAll(m.settings.DPDK.Interfaces); err != nil {
		return fmt.Errorf("vpp: dpdk bind: %w", err)
	}
	lg.Info("vpp: DPDK NICs bound", "count", len(m.settings.DPDK.Interfaces))
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
		m.emitEvent(events.EventVPPDisconnected)

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

// runOnce starts VPP, connects GoVPP, and waits for process exit.
// Returns when VPP exits or ctx is canceled.
func (m *VPPManager) runOnce(ctx context.Context, confPath string) error {
	lg := logger()

	cmd := exec.CommandContext(ctx, m.vppBinary, "-c", confPath) //nolint:gosec // vppBinary is set at registration, not user input
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start vpp: %w", err)
	}
	lg.Info("vpp: process started", "pid", cmd.Process.Pid)

	// Give VPP time to initialize and create the API socket.
	connectCtx, connectCancel := context.WithTimeout(ctx, 30*time.Second)
	defer connectCancel()

	if err := m.connector.Connect(connectCtx, 10, time.Second); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return fmt.Errorf("govpp connect: %w", err)
	}
	lg.Info("vpp: GoVPP connected", "socket", m.settings.APISocket)

	if m.hasRunOnce {
		m.emitEvent(events.EventVPPReconnected)
	} else {
		m.emitEvent(events.EventVPPConnected)
		m.hasRunOnce = true
	}

	// Wait for VPP process to exit.
	err := cmd.Wait()
	m.connector.Close()
	return fmt.Errorf("vpp process exited: %w", err)
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
	if _, err := eb.Emit(events.NamespaceVPP, eventType, ""); err != nil {
		logger().Error("vpp: emit event failed", "event", eventType, "error", err)
	}
}
