// Design: docs/features/interfaces.md -- iface-ra plugin registration
// Related: sender_linux.go -- the sender this factory starts
// Related: ifacera.go -- the counters ConfigureMetrics binds

//go:build linux

package ifacera

import (
	"fmt"
	"log/slog"
	"net"
	"os"
	"sync/atomic"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// loggerPtr holds the engine logger. It starts as a discard logger so a send
// loop that runs before the engine configures logging still has somewhere to
// write.
var loggerPtr atomic.Pointer[slog.Logger]

func init() {
	loggerPtr.Store(slogutil.DiscardLogger())

	// Hand the interface component a way to start senders without importing
	// this package. Removing this plugin leaves the factory unset, and
	// reconcileRA becomes a no-op.
	iface.SetRASenderFactory(newSenderFromFactory)

	reg := registry.Registration{
		Name:         "iface-ra",
		Description:  "Router Advertisement sender: advertises IPv6 prefixes, flags, and resolvers on a LAN (RFC 4861)",
		Dependencies: []string{"interface"},
		RunEngine:    runRAPlugin,
	}
	reg.CLIHandler = func(_ []string) int { return 1 }
	reg.ConfigureEngineLogger = func(loggerName string) {
		if l := slogutil.Logger(loggerName); l != nil {
			loggerPtr.Store(l)
		}
	}
	reg.ConfigureMetrics = func(r metrics.Registry) { SetMetricsRegistry(r) }
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "iface-ra: registration failed: %v\n", err)
		os.Exit(1)
	}

	// The forwarding warning belongs to the package that owns the dependency,
	// so removing this plugin removes the check with it.
	if err := diagnostic.RegisterDoctorCheck(raForwardingDoctorCheck); err != nil {
		fmt.Fprintf(os.Stderr, "iface-ra: doctor check registration failed: %v\n", err)
		os.Exit(1)
	}
}

// newSenderFromFactory starts one sender, bridging the interface component's
// factory callback to this package's concrete type.
func newSenderFromFactory(spec iface.RASenderSpec) (iface.RAStopper, error) {
	return NewSender(spec, loggerPtr.Load())
}

// runRAPlugin is the engine-mode entry point.
func runRAPlugin(conn net.Conn) int {
	log := loggerPtr.Load()
	log.Debug("iface-ra plugin starting")

	p := sdk.NewWithConn("iface-ra", conn)
	defer func() { _ = p.Close() }()

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	if err := p.Run(ctx, sdk.Registration{}); err != nil {
		log.Error("iface-ra plugin failed", "error", err)
		return 1
	}
	return 0
}
