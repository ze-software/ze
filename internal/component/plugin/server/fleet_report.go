// Design: docs/architecture/fleet-config.md -- hub-side managed report receivers and fleet health summary
// Related: managed_serve.go -- handleRequest dispatches registered report methods
// Related: managed_registry.go -- per-device inventory and health snapshot storage

package server

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/fleet"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const (
	managedInventoryMinInterval = 24 * time.Hour
	managedHealthMinInterval    = time.Minute
)

// ManagedReportReceiver declares one hub-side handler for a managed client
// report RPC method. Separate from pkg/fleet's producer registry so server code
// never imports host or managed-client collectors.
type ManagedReportReceiver struct {
	Name            string
	Method          string
	MinInterval     time.Duration
	BodyMax         int  // Optional payload byte bound; 0 defaults to fleet.ReportBodyMax.
	SelfRateLimited bool // When true, Handle enforces its own rate limit and MinInterval may be 0.
	OnRejected      func(s *ManagedServer)
	Handle          func(s *ManagedServer, device string, mc *rpc.MuxConn, payload []byte) error
}

// ManagedFleetHealthDevice describes one degraded or down declared device in
// the aggregated fleet health summary.
type ManagedFleetHealthDevice struct {
	Name       string                  `json:"name"`
	Status     fleet.HealthStatus      `json:"status"`
	Connected  bool                    `json:"connected"`
	ReceivedAt time.Time               `json:"received-at"`
	CheckedAt  string                  `json:"checked-at"`
	Components []fleet.HealthComponent `json:"components"`
}

// ManagedFleetHealthSummary aggregates the last reported health status across
// all declared managed devices. Disconnected devices keep their last reported
// status with Connected false; declared devices that have never reported are
// counted separately in Unreported.
type ManagedFleetHealthSummary struct {
	Total      int                        `json:"total"`
	Healthy    int                        `json:"healthy"`
	Degraded   int                        `json:"degraded"`
	Down       int                        `json:"down"`
	Unreported int                        `json:"unreported"`
	Devices    []ManagedFleetHealthDevice `json:"devices"`
}

var (
	managedReportReceiversMu sync.RWMutex
	managedReportReceivers   = map[string]ManagedReportReceiver{}
)

func registerManagedReportReceivers() {
	MustRegisterManagedReportReceiver(ManagedReportReceiver{
		Name:        "inventory",
		Method:      fleet.VerbInventoryReport,
		MinInterval: managedInventoryMinInterval,
		Handle:      handleInventoryReportPayload,
	})
	MustRegisterManagedReportReceiver(ManagedReportReceiver{
		Name:        "health",
		Method:      fleet.VerbHealthReport,
		MinInterval: managedHealthMinInterval,
		Handle:      handleHealthReportPayload,
	})
	MustRegisterManagedReportReceiver(ManagedReportReceiver{
		Name:            "sample",
		Method:          fleet.VerbSampleReport,
		BodyMax:         fleet.SampleReportBodyMax,
		SelfRateLimited: true,
		OnRejected:      recordSampleReportRejected,
		Handle:          handleSampleReportPayload,
	})
}

// RegisterManagedReportReceiver registers r in the hub's managed report
// receiver registry. It rejects empty Name, empty Method, non-positive
// MinInterval (unless SelfRateLimited is true), negative BodyMax, nil Handle,
// duplicate Name, and duplicate Method. Safe for concurrent use.
func RegisterManagedReportReceiver(r ManagedReportReceiver) error {
	if r.Name == "" {
		return errors.New("pluginserver: report receiver name is required")
	}
	if r.Method == "" {
		return fmt.Errorf("pluginserver: report receiver %q method is required", r.Name)
	}
	if !r.SelfRateLimited && r.MinInterval <= 0 {
		return fmt.Errorf("pluginserver: report receiver %q min interval must be positive, got %v", r.Name, r.MinInterval)
	}
	if r.BodyMax < 0 {
		return fmt.Errorf("pluginserver: report receiver %q body max cannot be negative, got %d", r.Name, r.BodyMax)
	}
	if r.Handle == nil {
		return fmt.Errorf("pluginserver: report receiver %q handler is required", r.Name)
	}
	managedReportReceiversMu.Lock()
	defer managedReportReceiversMu.Unlock()
	if _, dup := managedReportReceivers[r.Method]; dup {
		return fmt.Errorf("pluginserver: duplicate report receiver method %q", r.Method)
	}
	for _, existing := range managedReportReceivers {
		if existing.Name == r.Name {
			return fmt.Errorf("pluginserver: duplicate report receiver name %q", r.Name)
		}
	}
	managedReportReceivers[r.Method] = r
	return nil
}

// MustRegisterManagedReportReceiver registers r and panics on error. Intended
// for compile-time init() registration only.
func MustRegisterManagedReportReceiver(r ManagedReportReceiver) {
	if err := RegisterManagedReportReceiver(r); err != nil {
		panic("BUG: pluginserver: " + strings.TrimPrefix(err.Error(), "pluginserver: "))
	}
}

// SnapshotManagedReportReceivers captures the current report receiver registry
// and returns a restore function for test cleanup. Callers MUST invoke the
// returned function (for example via t.Cleanup) to restore the captured state.
// Safe for concurrent use.
func SnapshotManagedReportReceivers() func() {
	managedReportReceiversMu.RLock()
	saved := maps.Clone(managedReportReceivers)
	managedReportReceiversMu.RUnlock()
	return func() {
		managedReportReceiversMu.Lock()
		managedReportReceivers = maps.Clone(saved)
		managedReportReceiversMu.Unlock()
	}
}

func lookupManagedReportReceiver(method string) (ManagedReportReceiver, bool) {
	managedReportReceiversMu.RLock()
	recv, ok := managedReportReceivers[method]
	managedReportReceiversMu.RUnlock()
	return recv, ok
}

func handleInventoryReportPayload(s *ManagedServer, device string, mc *rpc.MuxConn, payload []byte) error {
	report, err := fleet.DecodeInventoryReport(payload)
	if err != nil {
		return err
	}
	return s.recordInventoryReport(device, mc, report)
}

func handleHealthReportPayload(s *ManagedServer, device string, mc *rpc.MuxConn, payload []byte) error {
	report, err := fleet.DecodeHealthReport(payload)
	if err != nil {
		return err
	}
	return s.recordHealthReport(device, mc, report)
}

func recordSampleReportRejected(s *ManagedServer) {
	if s != nil && s.history != nil {
		s.history.RecordRejected()
	}
}

func handleSampleReportPayload(s *ManagedServer, device string, mc *rpc.MuxConn, payload []byte) error {
	if s.history == nil {
		return errors.New("fleet metric history is not enabled on this hub")
	}
	report, err := fleet.DecodeAndValidateSampleReport(payload)
	if err != nil {
		s.history.RecordRejected()
		return err
	}
	return s.recordSampleReport(device, mc, report)
}

func (s *ManagedServer) responseContext() context.Context {
	if s.ctx != nil {
		return s.ctx
	}
	return context.Background()
}

func (s *ManagedServer) sendReportError(ctx context.Context, name, method string, mc *rpc.MuxConn, id uint64, msg string) {
	if err := mc.SendError(ctx, id, msg); err != nil && ctx.Err() == nil {
		managedLogger().Debug("send managed report error reply failed",
			"name", name, "method", method, "error", err)
	}
}

// handleReport enforces per-connection rate limiting (unless SelfRateLimited)
// and the body size bound before invoking recv.Handle. A refused report never
// updates session state (so rejected attempts never postpone the next allowed
// report) and never mutates the device registry or history store.
func (s *ManagedServer) handleReport(name string, mc *rpc.MuxConn, req *rpc.Request, session *managedSession, recv ManagedReportReceiver) {
	ctx := s.responseContext()
	now := s.currentTime()
	if !recv.SelfRateLimited && session != nil && session.lastReport != nil {
		if prev, seen := session.lastReport[recv.Method]; seen && now.Sub(prev) < recv.MinInterval {
			if recv.OnRejected != nil {
				recv.OnRejected(s)
			}
			var tb textbuf.Buffer
			tb.Str(recv.Method).Str(" rate limited: minimum interval is ").Str(recv.MinInterval.String())
			s.sendReportError(ctx, name, recv.Method, mc, req.ID, tb.String())
			return
		}
	}
	bodyMax := recv.BodyMax
	if bodyMax == 0 {
		bodyMax = fleet.ReportBodyMax
	}
	if len(req.Params) > bodyMax {
		if recv.OnRejected != nil {
			recv.OnRejected(s)
		}
		var tb textbuf.Buffer
		tb.Str(recv.Method).Str(" payload size ").Int(int64(len(req.Params))).
			Str(" exceeds max ").Int(int64(bodyMax))
		s.sendReportError(ctx, name, recv.Method, mc, req.ID, tb.String())
		return
	}
	if err := recv.Handle(s, name, mc, req.Params); err != nil {
		var tb textbuf.Buffer
		tb.Str(recv.Method).Str(": ").Err(err)
		s.sendReportError(ctx, name, recv.Method, mc, req.ID, tb.String())
		return
	}
	if !recv.SelfRateLimited && session != nil {
		if session.lastReport == nil {
			session.lastReport = make(map[string]time.Time)
		}
		session.lastReport[recv.Method] = now
	}
	if err := mc.SendResult(ctx, req.ID, struct{}{}); err != nil && ctx.Err() == nil {
		managedLogger().Debug("send managed report result reply failed",
			"name", name, "method", recv.Method, "error", err)
	}
}

// ManagedFleetHealth returns the aggregated fleet health summary from the
// running managed server, or ErrManagedNotServing when no managed server is
// published. Safe for concurrent use.
func ManagedFleetHealth() (ManagedFleetHealthSummary, error) {
	managedRegistryReaders.RLock()
	defer managedRegistryReaders.RUnlock()
	s := managedDoctorSource.Load()
	if s == nil {
		return ManagedFleetHealthSummary{}, ErrManagedNotServing
	}
	return s.FleetHealthSummary()
}

// FleetHealthSummary classifies every declared managed device once by its last
// reported health status. Disconnection does not alter a device's health
// status; its Connected field is reported alongside the last status. Returned
// component slices are deep-copied so callers cannot mutate live state. Safe
// for concurrent use.
func (s *ManagedServer) FleetHealthSummary() (ManagedFleetHealthSummary, error) {
	summary := ManagedFleetHealthSummary{
		Devices: []ManagedFleetHealthDevice{},
	}
	s.mu.Lock()
	for name := range s.devices {
		device := s.visibleDeviceLocked(name)
		if device == nil {
			continue
		}
		if !device.declared {
			continue
		}
		summary.Total++
		if device.Health == nil {
			summary.Unreported++
			continue
		}
		switch device.Health.Report.Status {
		case fleet.HealthStatusHealthy:
			summary.Healthy++
		case fleet.HealthStatusDegraded:
			summary.Degraded++
			summary.Devices = append(summary.Devices, fleetHealthDeviceFromEntry(device))
		case fleet.HealthStatusDown:
			summary.Down++
			summary.Devices = append(summary.Devices, fleetHealthDeviceFromEntry(device))
		case fleet.HealthStatusUnspecified:
			summary.Unreported++
		default:
			// Open enum from pkg/fleet: treat an unrecognized status as unreported.
			summary.Unreported++
		}
	}
	s.mu.Unlock()
	slices.SortFunc(summary.Devices, func(a, b ManagedFleetHealthDevice) int {
		return cmp.Compare(a.Name, b.Name)
	})
	return summary, nil
}

func fleetHealthDeviceFromEntry(entry *managedDevice) ManagedFleetHealthDevice {
	report := entry.Health.Report.Clone()
	components := report.Components
	if components == nil {
		components = []fleet.HealthComponent{}
	}
	return ManagedFleetHealthDevice{
		Name:       entry.Name,
		Status:     report.Status,
		Connected:  entry.conn != nil,
		ReceivedAt: entry.Health.ReceivedAt,
		CheckedAt:  report.CheckedAt,
		Components: components,
	}
}
