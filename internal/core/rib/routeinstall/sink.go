// Design: plan/learned/1070-forked-route-install.md -- forked route install via Loc-RIB RPC
//
// Package routeinstall provides the RouteSink a FORKED route-installing plugin
// (OSPF, IS-IS) uses in place of a direct Loc-RIB write. In-process, an installer
// writes locrib.Default() directly; in a forked subprocess locrib.Default() is nil
// (default.go, guarded on ze.plugin.hub.token), so the installer holds one of these
// sinks instead and each insert/remove is shipped to the engine over the
// ze-plugin-engine:route-install / :route-remove RPC. The engine re-resolves the
// protocol NAME to its own ProtocolID (redistevents IDs are per-process) and
// applies the op to its real Loc-RIB, where sysrib programs the kernel.
package routeinstall

import (
	"context"
	"net/netip"
	"sync"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	"github.com/ze-software/ze/internal/core/rib/locrib"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

var logger = slogutil.LazyLogger("routeinstall")

// Client is the subset of the plugin SDK the sink needs. *sdk.Plugin satisfies it;
// tests supply a capture/mock. Kept narrow so this package does not depend on the
// whole SDK surface.
type Client interface {
	RouteInstall(ctx context.Context, routes []rpc.RouteInstallEntry) (uint32, error)
	RouteRemove(ctx context.Context, routes []rpc.RouteRemoveEntry) (uint32, error)
}

// Sink ships Loc-RIB install/remove operations to the engine over RPC. It matches
// the method set the OSPF and IS-IS SPF installers expect from their local
// RouteSink interface, so a forked installer can swap a *locrib.RIB for a *Sink.
//
// InsertForward/Remove BUFFER their operations; the installer calls Flush once per
// SPF Apply so a whole route delta travels in one route-install + one route-remove
// RPC instead of one round-trip per next-hop (R-1). The buffers are mutex-guarded:
// in production a single SPF goroutine drives one Sink (and the two IS-IS address
// families share it sequentially), but the lock keeps a future off-thread caller
// race-free.
type Sink struct {
	ctx    context.Context
	client Client

	mu       sync.Mutex
	installs []rpc.RouteInstallEntry
	removes  []rpc.RouteRemoveEntry
}

// maxFlushAttempts bounds the per-batch RPC retries in Flush (transient tolerance).
const maxFlushAttempts = 3

// New builds a Sink that dispatches over client using ctx for every RPC. The
// forked OSPF/IS-IS wiring passes context.Background(): flush is best-effort and
// bounded by the RPC layer's own per-call handling and by the mux closing on
// plugin shutdown, not by ctx cancellation.
func New(ctx context.Context, client Client) *Sink {
	return &Sink{ctx: ctx, client: client}
}

// InsertForward buffers one route for the next Flush. The protocol NAME is carried
// (redistevents.ProtocolName(p.Source)); the engine re-resolves it to its own
// ProtocolID.
func (s *Sink) InsertForward(fam family.Family, prefix netip.Prefix, p locrib.Path) {
	entry := rpc.RouteInstallEntry{
		Protocol:           redistevents.ProtocolName(p.Source),
		AFI:                uint16(fam.AFI),
		SAFI:               uint8(fam.SAFI),
		Prefix:             prefix.String(),
		Instance:           p.Instance,
		NextHop:            addrString(p.NextHop),
		AdminDistance:      p.AdminDistance,
		Metric:             p.Metric,
		Labels:             p.Labels,
		IsEBGP:             p.IsEBGP,
		BackupNextHop:      addrString(p.BackupNextHop),
		BackupRepairLabels: p.BackupRepairLabels,
	}
	s.mu.Lock()
	s.installs = append(s.installs, entry)
	s.mu.Unlock()
}

// Remove buffers one (Source, Instance) withdrawal for the next Flush.
func (s *Sink) Remove(fam family.Family, prefix netip.Prefix, source redistevents.ProtocolID, instance uint32) {
	entry := rpc.RouteRemoveEntry{
		Protocol: redistevents.ProtocolName(source),
		AFI:      uint16(fam.AFI),
		SAFI:     uint8(fam.SAFI),
		Prefix:   prefix.String(),
		Instance: instance,
	}
	s.mu.Lock()
	s.removes = append(s.removes, entry)
	s.mu.Unlock()
}

// Flush ships all buffered installs and removes as one route-install and one
// route-remove RPC, then clears the buffers. Called by the installer at the end of
// each SPF Apply (and RemoveAll). Installs are sent before removes so a
// within-delta prefix churn settles on the new paths; because a single SPF
// goroutine drives one Sink, that order holds (the mutex only guards the slice
// swap against a future off-thread caller, not ordering across concurrent flushes).
//
// A transient RPC error is retried up to maxFlushAttempts (engine busy / timeout on
// the local mux). A PERSISTENT error is logged and dropped, not returned: it means
// the connection is dead, so the plugin's read loop exits, the engine withdraws
// this plugin's routes on disconnect, and the respawned plugin re-installs from a
// clean slate -- the batch is deferred to respawn, not silently lost. The ops are
// idempotent (InsertForward upserts, Remove is (Source,Instance)-keyed), so a
// re-sent batch is safe.
func (s *Sink) Flush() {
	s.mu.Lock()
	installs := s.installs
	removes := s.removes
	s.installs = nil
	s.removes = nil
	s.mu.Unlock()

	if len(installs) > 0 {
		s.send("route-install", len(installs), func() error {
			_, err := s.client.RouteInstall(s.ctx, installs)
			return err
		})
	}
	if len(removes) > 0 {
		s.send("route-remove", len(removes), func() error {
			_, err := s.client.RouteRemove(s.ctx, removes)
			return err
		})
	}
}

// send runs fn up to maxFlushAttempts times, tolerating a transient error. See
// Flush for the persistent-failure recovery contract.
func (s *Sink) send(op string, n int, fn func() error) {
	var err error
	for attempt := 1; attempt <= maxFlushAttempts; attempt++ {
		if err = fn(); err == nil {
			return
		}
	}
	logger().Warn("forked route flush failed after retries", "op", op, "routes", n, "attempts", maxFlushAttempts, "error", err)
}

// addrString renders a next-hop for the wire; an invalid Addr (zero value, "no
// next-hop"/directly-connected) becomes the empty string, which the entry omits.
func addrString(a netip.Addr) string {
	if !a.IsValid() {
		return ""
	}
	return a.String()
}
