// Design: docs/research/vpp-deployment-reference.md -- VPP FIB route programming
// Detail: backend.go -- GoVPP backend and mock for testing
//
// fib-vpp subscribes to (system-rib, best-change) on the EventBus and programs
// VPP's FIB directly via GoVPP binary API. Mirrors the fib-p4/fib-kernel pattern
// with a GoVPP backend instead of noop/netlink.
package fibvpp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/core/bgp/routeaction"

	sysribevents "github.com/ze-software/ze/internal/component/sysrib/events"
	"github.com/ze-software/ze/internal/core/mplsfib"
	"github.com/ze-software/ze/internal/core/replay"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/ze"
)

var (
	errFibVppConfigSectionMissingFib  = errors.New("fib-vpp: config section missing 'fib' root")
	errFibVppConfigSectionMissingFib2 = errors.New("fib-vpp: config section missing 'fib/vpp' subtree")
	errFibVPPNotStarted               = errors.New("show fib vpp: the fib-vpp plugin has not finished " +
		"starting, so it has no FIB writer to read installed routes from; retry 'show fib vpp' " +
		"once startup completes")
	errFibVPPNoConnector = errors.New("fib-vpp: the vpp component is not running, so there is no " +
		"GoVPP connection to program routes through; configure 'vpp { }' and run 'ze doctor' " +
		"to see why VPP is not ready")
)

// loggerPtr is the package-level logger, disabled by default.
var loggerPtr atomic.Pointer[slog.Logger]

func init() {
	d := slogutil.DiscardLogger()
	loggerPtr.Store(d)
}

func logger() *slog.Logger { return loggerPtr.Load() }

func setFibVPPLogger(l *slog.Logger) {
	if l != nil {
		loggerPtr.Store(l)
	}
}

// eventBusPtr stores the EventBus instance.
var eventBusPtr atomic.Pointer[ze.EventBus]

func setFibVPPEventBus(eb ze.EventBus) {
	if eb != nil {
		eventBusPtr.Store(&eb)
	}
}

func getEventBus() ze.EventBus {
	p := eventBusPtr.Load()
	if p == nil {
		return nil
	}
	return *p
}

// fibVPPConfig holds parsed fib/vpp config values.
// batch-size and batch-interval-ms are accepted by the parser (YANG leaves
// exist) but not consumed: sysRIB already delivers per-family batches and
// VPP's ip_route_add_del_v2 is per-route, so cross-emission accumulation adds
// complexity for zero benefit today.
type fibVPPConfig struct {
	tableID uint32
}

// parseFibVPPConfigSection parses a wrapped fib-vpp config section delivered
// by the plugin-server ExtractConfigSubtree helper. For ConfigRoots "fib/vpp"
// the helper wraps the subtree as `{"fib":{"vpp":{...}}}`. This function
// unwraps both levels and delegates to parseFibVPPConfig.
//
// Use this from the plugin OnConfigure callback. Use parseFibVPPConfig
// directly from tests or callers that already hold the inner subtree.
func parseFibVPPConfigSection(data string) (*fibVPPConfig, error) {
	var outer map[string]json.RawMessage
	if err := json.Unmarshal([]byte(data), &outer); err != nil {
		return nil, fmt.Errorf("fib-vpp: parse wrapped config: %w", err)
	}
	fibRaw, ok := outer["fib"]
	if !ok {
		return nil, errFibVppConfigSectionMissingFib
	}
	var fib map[string]json.RawMessage
	if err := json.Unmarshal(fibRaw, &fib); err != nil {
		return nil, fmt.Errorf("fib-vpp: parse fib container: %w", err)
	}
	vppRaw, ok := fib["vpp"]
	if !ok {
		return nil, errFibVppConfigSectionMissingFib2
	}
	return parseFibVPPConfig(string(vppRaw))
}

// parseFibVPPConfig extracts fib-vpp settings from config section JSON.
func parseFibVPPConfig(data string) (*fibVPPConfig, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(data), &raw); err != nil {
		return nil, fmt.Errorf("fib-vpp config: %w", err)
	}
	known := map[string]bool{"enabled": true, "table-id": true}
	deferred := map[string]bool{"batch-size": true, "batch-interval-ms": true}
	for k := range raw {
		if deferred[k] {
			return nil, fmt.Errorf("fib-vpp config: %q is not yet implemented (deferred)", k)
		}
		if !known[k] {
			return nil, fmt.Errorf("fib-vpp config: unknown key %q", k)
		}
	}

	cfg := &fibVPPConfig{}
	if v, ok := raw["table-id"]; ok {
		s := strings.Trim(string(v), `"`)
		n, err := strconv.ParseUint(s, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("fib-vpp table-id: %w", err)
		}
		cfg.tableID = uint32(n)
	}
	return cfg, nil
}

// incomingBatch aliases the (system-rib, best-change) payload type.
type incomingBatch = sysribevents.BestChangeBatch

// incomingChange aliases a single entry in an incoming batch.
type incomingChange = sysribevents.BestChangeEntry

// installedRoute tracks a route installed in VPP for correct flush/delete.
type installedRoute struct {
	nextHop string
}

// srv6Refusal records why one SRv6 service route was not programmed into VPP.
type srv6Refusal struct {
	sid netip.Addr
	err error
}

// fibVPP manages VPP FIB route programming.
type fibVPP struct {
	installed      map[srv6RouteKey]installedRoute // (prefix, table) -> ordinary state
	mplsInstalled  map[string]bool                 // prefix -> true (MPLS labeled routes)
	srv6Installed  map[srv6RouteKey]bool           // (prefix, table) -> owned SRv6 steering
	srv6TableID    uint32
	srv6RestoreErr error
	srv6Refusals   map[srv6RouteKey]srv6Refusal
	// labelOwners maps each swap/pop in-label this writer installed to the
	// source that owns it; labelPushes each (mpls-fib, entry) push FEC to
	// whether it carries labels (false: the plain route of an Implicit NULL
	// push).
	labelOwners map[uint32]uint16
	labelPushes map[netip.Prefix]bool
	// labelTableReady records that MPLS table 0 exists on this connection.
	labelTableReady bool
	// labelStale holds the label routes MPLS table 0 carried when this
	// writer opened, before it asked for the replay: what a previous writer
	// left behind. sweepStaleLabels removes the ones no source re-claimed.
	// labelSweepFailures keeps each one VPP would not remove, and
	// labelDumpErr why table 0 could not be read, for the doctor check.
	labelStale         []labelPosition
	labelSweepFailures map[labelPosition]error
	labelDumpErr       error
	// labelReplayToken is the targeted (mpls-fib, replay-request) token this
	// writer minted when it opened; a replay batch carrying another token is
	// another writer's. Set before the writer is published, then read only.
	labelReplayToken uint64
	// ipTableID is fib.vpp.table-id, the IP table an egress pop looks the
	// popped packet up in.
	ipTableID     uint32
	pathMTUWarned bool
	retired       bool // prevents an old subscription racing reconnect reconciliation
	// routes is the VPP FIB source this writer's IP routes and labeled pushes
	// are installed under, and the record of what a previous writer left
	// under it (routesource.go). nil for a writer over a mock backend, which
	// holds no VPP to sweep.
	routes      *routeSource
	backend     vppBackend
	mplsBackend mplsBackend
	srv6Backend srv6Backend
	mu          sync.RWMutex
}

func newFibVPP(backend vppBackend) *fibVPP {
	return &fibVPP{
		installed:     make(map[srv6RouteKey]installedRoute),
		mplsInstalled: make(map[string]bool),
		srv6Installed: make(map[srv6RouteKey]bool),
		srv6Refusals:  make(map[srv6RouteKey]srv6Refusal),
		labelOwners:   make(map[uint32]uint16),
		labelPushes:   make(map[netip.Prefix]bool),
		backend:       backend,
	}
}

// recordSRv6RefusalLocked stores the refusal for (prefix, table) so
// fib-vpp-srv6 reports it until a later install, replace, or withdrawal clears it.
// Caller MUST hold f.mu.
func (f *fibVPP) recordSRv6RefusalLocked(key srv6RouteKey, sid netip.Addr, err error) {
	if f.srv6Refusals == nil {
		f.srv6Refusals = make(map[srv6RouteKey]srv6Refusal)
	}
	f.srv6Refusals[key] = srv6Refusal{sid: sid, err: err}
}

// clearSRv6RefusalOnWithdrawLocked clears a recorded refusal for key, and on a
// zero-table withdrawal also clears a uniquely matching refused prefix in
// another table. Caller MUST hold f.mu.
func (f *fibVPP) clearSRv6RefusalOnWithdrawLocked(requestedTable uint32, key srv6RouteKey) {
	delete(f.srv6Refusals, key)
	if requestedTable != 0 {
		return
	}
	var matched srv6RouteKey
	matches := 0
	for k := range f.srv6Refusals {
		if k.prefix == key.prefix {
			matched = k
			matches++
		}
	}
	if matches == 1 {
		delete(f.srv6Refusals, matched)
	}
}

// refuseSRv6WithoutStoreLocked handles an SRv6 change when no persistent daemon
// store is configured, before processSRv6Change can touch b.store. It returns
// true when the change is fully handled and the caller should continue to the
// next entry in the batch. Caller MUST hold f.mu.
func (f *fibVPP) refuseSRv6WithoutStoreLocked(c *incomingChange, storeErr error) bool {
	key, _ := f.srv6ChangeKey(c.Prefix, c.TableID, false) //nolint:errcheck // recoverTable=false cannot fail
	switch c.Action.Verb() {
	case routeaction.VerbInstall, routeaction.VerbReplace:
		f.recordSRv6RefusalLocked(key, c.SRv6SID, storeErr)
		logger().Error("fib-vpp: SRv6 service route refused because durable daemon state is unavailable; run 'ze init', then restart ze",
			"prefix", key.prefix, "table", key.table, "sid", c.SRv6SID, "error", storeErr, "code", codeSRv6Refused)
		return true
	case routeaction.VerbRemove:
		f.clearSRv6RefusalOnWithdrawLocked(c.TableID, key)
		if _, installed := f.installed[key]; installed {
			return false
		}
		if key.table == f.srv6TableID && f.mplsInstalled[key.prefix.String()] {
			return false
		}
		return true
	case routeaction.VerbSkip:
		// Unspecified and unknown actions do not change SRv6 forwarding.
		return true
	default:
		panic("BUG: invalid normalized route verb")
	}
}

// hasRichFields reports whether a change carries attributes beyond prefix+next-hop.
func hasRichFields(c *incomingChange) bool {
	// The outgoing device counts: the plain backend takes an address alone and
	// has nowhere to put a device, so a route named by a device would reach VPP
	// with no path at all.
	return c.RouteType != 0 || c.Metric != 0 || c.TableID != 0 ||
		c.Interface != "" || c.Weight != 0 || len(c.ECMPPaths) > 0
}

func changeToRichRoute(c *incomingChange) vppRichRoute {
	return vppRichRoute{
		Prefix:    c.Prefix,
		NextHop:   c.NextHop,
		Interface: c.Interface,
		Weight:    c.Weight,
		Labels:    c.Labels,
		RouteType: c.RouteType,
		Metric:    c.Metric,
		TableID:   c.TableID,
		ECMPPaths: c.ECMPPaths,
	}
}

// isSinglePathMPLS reports whether c belongs to the single-path MPLS push
// handler rather than the rich multipath IP route path. An entry carrying
// ECMPPaths always takes the rich path so every member keeps its own next hop
// and label stack.
func (f *fibVPP) isSinglePathMPLS(c *incomingChange) bool {
	if len(c.ECMPPaths) > 0 {
		return false
	}
	if len(c.Labels) > 0 {
		return true
	}
	return f.mplsInstalled[c.Prefix.String()]
}

// addVPPRoute dispatches to rich, multi-path, or single-path based on fields present.
func (f *fibVPP) addVPPRoute(c *incomingChange) error {
	if hasRichFields(c) {
		return f.backend.addRichRoute(changeToRichRoute(c))
	}
	return f.backend.addRoute(c.Prefix, c.NextHop)
}

// replaceVPPRoute dispatches to rich, multi-path, or single-path replace.
func (f *fibVPP) replaceVPPRoute(c *incomingChange) error {
	if hasRichFields(c) {
		return f.backend.replaceRichRoute(changeToRichRoute(c))
	}
	return f.backend.replaceRoute(c.Prefix, c.NextHop)
}

// delVPPRoute recovers an omitted table only from unambiguous owned identity.
func (f *fibVPP) delVPPRoute(c *incomingChange) error {
	key, err := f.srv6ChangeKey(c.Prefix, c.TableID, true)
	if err != nil {
		return err
	}
	if key.table == 0 {
		err = f.backend.delRoute(key.prefix)
	} else {
		err = f.backend.delRichRoute(key.prefix, key.table)
	}
	if err == nil {
		delete(f.installed, key)
		f.clearSRv6RefusalOnWithdrawLocked(c.TableID, key)
	}
	return err
}

// processEvent handles a single (system-rib, best-change) payload received
// via the typed BestChange handle.
func (f *fibVPP) processEvent(batch *incomingBatch) {
	if batch == nil {
		return
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if f.retired {
		return
	}

	var restoreErr error
	storeUnavailable := false
	for i := range batch.Changes {
		c := &batch.Changes[i]
		if !c.Prefix.IsValid() {
			logger().Warn("fib-vpp: skipping change with empty prefix")
			continue
		}
		if !storeUnavailable {
			restoreErr = f.restoreSRv6()
			storeUnavailable = isSRv6StoreUnavailable(restoreErr)
			if restoreErr != nil {
				if !storeUnavailable {
					f.srv6RestoreErr = restoreErr
					logger().Error("fib-vpp: SRv6 ownership reconciliation failed; preserving forwarding; run 'ze show doctor' for the fix",
						"error", restoreErr, "code", codeSRv6Refused)
					return
				}
			}
			f.srv6RestoreErr = nil
		}
		if storeUnavailable {
			if c.SRv6SID.IsValid() {
				if f.refuseSRv6WithoutStoreLocked(c, restoreErr) {
					continue
				}
			}
		} else {
			if c.SRv6SID.IsValid() {
				f.processSRv6Change(c)
				continue
			}
			if f.hasSRv6Route(c.Prefix, c.TableID) {
				f.processSRv6Change(c)
				continue
			}
		}
		if f.isSinglePathMPLS(c) {
			f.processMPLSChange(c)
			continue
		}
		switch c.Action.Verb() {
		case routeaction.VerbInstall:
			if err := f.addVPPRoute(c); err != nil {
				logger().Error("fib-vpp: add route failed", "prefix", c.Prefix, "error", err)
				continue
			}
			key, _ := f.srv6ChangeKey(c.Prefix, c.TableID, false)
			f.installed[key] = installedRoute{nextHop: c.NextHop.String()}
			delete(f.mplsInstalled, c.Prefix.String())
			delete(f.srv6Refusals, key)
			if m := fibVPPMetricsPtr.Load(); m != nil {
				m.routeInstalls.Inc()
				m.routesInstalled.Set(float64(len(f.installed)))
			}
		case routeaction.VerbReplace:
			if err := f.replaceVPPRoute(c); err != nil {
				logger().Error("fib-vpp: replace route failed", "prefix", c.Prefix, "error", err)
				continue
			}
			key, _ := f.srv6ChangeKey(c.Prefix, c.TableID, false)
			f.installed[key] = installedRoute{nextHop: c.NextHop.String()}
			delete(f.mplsInstalled, c.Prefix.String())
			delete(f.srv6Refusals, key)
			if m := fibVPPMetricsPtr.Load(); m != nil {
				m.routeUpdates.Inc()
			}
		case routeaction.VerbRemove:
			if err := f.delVPPRoute(c); err != nil {
				logger().Error("fib-vpp: del route failed", "prefix", c.Prefix, "error", err)
				continue
			}
			if m := fibVPPMetricsPtr.Load(); m != nil {
				m.routeRemovals.Inc()
				m.routesInstalled.Set(float64(len(f.installed)))
			}
		case routeaction.VerbSkip:
			logger().Warn("fib-vpp: skipping change with unspecified action", "prefix", c.Prefix)
		default:
			panic("BUG: invalid normalized route verb")
		}
	}
}

// processMPLSChange handles a single best-change entry with MPLS labels.
// Caller must hold f.mu.
func (f *fibVPP) processMPLSChange(c *incomingChange) {
	if f.mplsBackend == nil {
		logger().Warn("fib-vpp: MPLS change but no MPLS backend configured", "prefix", c.Prefix)
		return
	}
	pfxStr := c.Prefix.String()
	switch c.Action.Verb() {
	case routeaction.VerbInstall, routeaction.VerbReplace:
		if err := f.mplsBackend.addMPLSRoute(c.Prefix, c.NextHop, c.Labels); err != nil {
			logger().Error("fib-vpp: MPLS add failed", "prefix", c.Prefix, "error", err)
			return
		}
		f.mplsInstalled[pfxStr] = true
		key, _ := f.srv6ChangeKey(c.Prefix, c.TableID, false) //nolint:errcheck // recoverTable=false cannot fail
		delete(f.installed, key)
		delete(f.srv6Refusals, key)
		if m := fibVPPMetricsPtr.Load(); m != nil {
			m.routeInstalls.Inc()
		}
	case routeaction.VerbRemove:
		if err := f.mplsBackend.delMPLSRoute(c.Prefix, c.Labels); err != nil {
			logger().Error("fib-vpp: MPLS del failed", "prefix", c.Prefix, "error", err)
			return
		}
		delete(f.mplsInstalled, pfxStr)
		key, _ := f.srv6ChangeKey(c.Prefix, c.TableID, false) //nolint:errcheck // recoverTable=false cannot fail
		f.clearSRv6RefusalOnWithdrawLocked(c.TableID, key)
		if m := fibVPPMetricsPtr.Load(); m != nil {
			m.routeRemovals.Inc()
		}
	case routeaction.VerbSkip:
		// Unspecified and unknown actions do not change MPLS forwarding.
	default:
		panic("BUG: invalid normalized route verb")
	}
}

// flushRoutes removes all installed entries from VPP FIB.
func (f *fibVPP) flushRoutes() {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.restoreSRv6(); err != nil {
		if !isSRv6StoreUnavailable(err) {
			f.srv6RestoreErr = err
			logger().Warn("fib-vpp: flush ownership reconciliation failed; preserving forwarding; run 'ze show doctor' for the fix",
				"error", err, "code", codeSRv6Refused)
			return
		}
	}
	f.srv6RestoreErr = nil
	for key := range f.installed {
		if backend, ok := f.srv6Backend.(*govppSRv6Backend); ok && backend.fallbacks[key] != nil {
			if err := f.removeSRv6Fallback(key); err != nil {
				logger().Warn("fib-vpp: flush fallback failed", "prefix", key.prefix, "table", key.table, "error", err)
			}
			continue
		}
		var err error
		if key.table == 0 {
			err = f.backend.delRoute(key.prefix)
		} else {
			err = f.backend.delRichRoute(key.prefix, key.table)
		}
		if err != nil {
			logger().Warn("fib-vpp: flush del failed", "prefix", key.prefix, "table", key.table, "error", err)
			continue
		}
		delete(f.installed, key)
	}

	if f.mplsBackend != nil {
		for prefixStr := range f.mplsInstalled {
			prefix, err := netip.ParsePrefix(prefixStr)
			if err != nil {
				continue
			}
			key := srv6RouteKey{prefix: prefix, table: f.srv6TableID}
			if err := f.removeSRv6Fallback(key); err != nil {
				logger().Warn("fib-vpp: flush MPLS del failed", "prefix", prefixStr, "error", err)
			}
		}
	}

	f.flushLabelRoutesLocked()

	if f.srv6Backend != nil {
		for key := range f.srv6Installed {
			if _, remains := f.installed[key]; remains {
				continue
			}
			if key.table == f.srv6TableID && f.mplsInstalled[key.prefix.String()] {
				continue
			}
			if err := f.srv6Backend.delSRv6Steer(key.prefix, key.table); err != nil {
				logger().Warn("fib-vpp: flush SRv6 del failed", "prefix", key.prefix, "table", key.table, "error", err)
				continue
			}
			delete(f.srv6Installed, key)
		}
	}

	if m := fibVPPMetricsPtr.Load(); m != nil {
		m.routesInstalled.Set(float64(len(f.installed)))
	}
}

// showInstalled returns the currently installed routes as JSON.
func (f *fibVPP) showInstalled() any {
	f.mu.RLock()
	defer f.mu.RUnlock()

	type entry struct {
		Prefix  string `json:"prefix"`
		NextHop string `json:"next-hop,omitempty"`
		MPLS    bool   `json:"mpls,omitempty"`
		TableID uint32 `json:"table-id,omitempty"`
	}

	entries := make([]entry, 0, len(f.installed)+len(f.mplsInstalled))
	for key, ir := range f.installed {
		entries = append(entries, entry{Prefix: key.prefix.String(), NextHop: ir.nextHop, TableID: key.table})
	}
	for prefix := range f.mplsInstalled {
		entries = append(entries, entry{Prefix: prefix, MPLS: true})
	}

	return entries
}

// requestLabelReplay creates MPLS table 0, records the label routes a previous
// writer left there (markStaleLabelsLocked), and then asks every label producer
// to re-apply the entries it wants installed, tagged with this writer's token:
// a writer that opens after the producers started, or after VPP restarted and
// lost its FIB, holds none of them. The answer arrives on the (mpls-fib,
// entry) subscription, which the lifecycle lock orders after this writer is
// published.
func (f *fibVPP) requestLabelReplay(eb ze.EventBus) {
	f.mu.Lock()
	var err error
	if f.mplsBackend != nil {
		err = f.ensureMPLSTableLocked()
	}
	if err == nil && f.mplsBackend != nil {
		// Read what table 0 holds before any source answers the replay, so
		// a route a source re-claims is told apart from a leftover.
		f.markStaleLabelsLocked()
	}
	f.mu.Unlock()
	if err != nil {
		logger().Warn("fib-vpp: MPLS table 0 could not be created, so no label entry is programmed "+
			"until it is; run 'ze show doctor' to see which label entries are missing", "error", err,
			"code", mplsfib.CodeDoctorNoWriter)
		return
	}
	if _, err := mplsfib.ReplayRequest.Emit(eb, &replay.Request{ReplayID: f.labelReplayToken}); err != nil {
		logger().Warn("fib-vpp: mpls-fib replay-request emit failed", "error", err,
			"code", mplsfib.CodeDoctorNoWriter)
	}
}

// run requests replay after the plugin has subscribed and restored ownership,
// then, labelSweepDelay later unless ctx ends first, removes the leftover IP
// routes, labeled pushes and label routes no source re-claimed. The caller
// MUST cancel ctx when retiring this worker.
func (f *fibVPP) run(ctx context.Context, flushOnStop bool) {
	eb := getEventBus()
	if eb == nil {
		logger().Warn("fib-vpp: no event bus configured")
		return
	}

	// Request full-table replay from sysrib. Broadcast hop: the token addresses
	// every consumer.
	if _, err := sysribevents.ReplayRequest.Emit(eb, &replay.Request{ReplayID: replay.Broadcast}); err != nil {
		logger().Warn("fib-vpp: replay-request emit failed", "error", err)
	}

	f.requestLabelReplay(eb)

	logger().Info("fib-vpp: running")
	// sysrib and the label sources answer the replay asynchronously; once the
	// window has passed, a leftover IP route or label route nobody re-claimed
	// is removed, as fib-kernel's sweepStale removes an IP route sysrib did
	// not refresh.
	sweep := time.NewTimer(labelSweepDelay)
	select {
	case <-ctx.Done():
	case <-sweep.C:
		f.sweepStaleRoutes()
		f.sweepStaleLabels()
		<-ctx.Done()
	}
	sweep.Stop()

	if flushOnStop {
		f.flushRoutes()
	}
	logger().Info("fib-vpp: stopped")
}
