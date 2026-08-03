// Design: plan/learned/1049-anomaly-2-shape.md -- shadow-first autonomous responder
//
// Implements the pinned Responder State Machine: one mutex over {armed map,
// armedCount, killed, timers}; per-entity firewall rules re-registered as a whole
// owner set on every change; timed auto-revert independent of events; blast-radius
// cap, kill-switch, and allowlist. The firewall install/withdraw is mockable and
// the clock is injectable for deterministic tests.

package shape

import (
	"log/slog"
	"net/netip"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/core/anomalyevent"
)

const owner = "anomaly-shape"

var loggerPtr atomic.Pointer[slog.Logger]

func setLogger(l *slog.Logger) {
	if l != nil {
		loggerPtr.Store(l)
	}
}

func logger() *slog.Logger {
	if l := loggerPtr.Load(); l != nil {
		return l
	}
	return slog.Default()
}

// registerTables / applyAll are indirected through package vars so tests can mock
// the firewall backend (mirrors ddos/local).
var registerTables = firewall.RegisterTables
var applyAll = firewall.ApplyAll

// stopper is the injectable-timer contract; *time.Timer satisfies it.
type stopper interface{ Stop() bool }

type armedRecord struct {
	gen   int
	timer stopper
}

type responder struct {
	mu         sync.Mutex
	cfg        *Config
	armed      map[netip.Prefix]*armedRecord
	armedCount int
	killed     bool
	gen        int // responder-level monotonic timer generation (globally unique)
	afterFunc  func(time.Duration, func()) stopper
	m          *shapeMetrics
	// published mirrors the armed set and the kill-switch for readers that must
	// not wait on mu. mu is held across reinstall/revertAll, and those reconcile
	// the kernel: a reader taking mu waited out a netlink round trip. show
	// anomaly-shape is such a reader (show.go handleShowAnomalyShape ->
	// statusSnapshot()). Written under mu by publishStatus, read lock-free by
	// statusSnapshot. Design: plan/spec-fixit-firewall-concurrency-deadlock.md D-4.
	published atomic.Pointer[shapeStatus]
}

func newResponder(cfg *Config) *responder {
	r := &responder{
		cfg:       cfg,
		armed:     make(map[netip.Prefix]*armedRecord),
		afterFunc: func(d time.Duration, f func()) stopper { return time.AfterFunc(d, f) },
		m:         loadMetrics(),
	}
	// Publish the idle snapshot before the responder is reachable, so
	// statusSnapshot never has to interpret a nil pointer.
	r.publishStatus()
	return r
}

func (r *responder) ttl() time.Duration {
	return time.Duration(r.cfg.AutoRevertTTL) * time.Second
}

// onDetected is step 1 of the state machine.
func (r *responder) onDetected(e *anomalyevent.AnomalyDetected) {
	r.mu.Lock()
	defer r.mu.Unlock()

	entity := e.Entity
	if r.killed || r.cfg.Mode == ModeShadow {
		logger().Info("anomaly-shape: shadow, would act on source", "entity", entity, "action", r.cfg.Action)
		return
	}
	if allowlisted(entity, r.cfg.Allowlist) {
		logger().Info("anomaly-shape: source allowlisted, skipping", "entity", entity)
		return
	}
	if rec, ok := r.armed[entity]; ok {
		r.rearm(entity, rec) // refresh the TTL; no reinstall, no count change
		return
	}
	if r.armedCount >= r.cfg.BlastRadiusCap {
		if r.m != nil {
			r.m.armRefused.Inc()
		}
		logger().Warn("anomaly-shape: blast-radius cap reached, refusing to arm", "entity", entity, "cap", r.cfg.BlastRadiusCap)
		return
	}
	rec := &armedRecord{}
	r.armed[entity] = rec
	r.armedCount++
	r.reinstall()
	r.rearm(entity, rec)
	logger().Info("anomaly-shape: armed source", "entity", entity, "action", r.cfg.Action)
}

// onOngoing extends the auto-revert deadline for a still-anomalous armed entity.
func (r *responder) onOngoing(e *anomalyevent.AnomalyOngoing) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if rec, ok := r.armed[e.Entity]; ok {
		r.rearm(e.Entity, rec)
	}
}

// onCleared withdraws an armed entity's rule early (before its TTL).
func (r *responder) onCleared(e *anomalyevent.AnomalyCleared) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.armed[e.Entity]; ok {
		r.withdraw(e.Entity)
	}
}

// rearm bumps the generation and (re)starts the auto-revert timer. Caller holds mu.
func (r *responder) rearm(entity netip.Prefix, rec *armedRecord) {
	if rec.timer != nil {
		rec.timer.Stop()
	}
	// A responder-level monotonic generation makes every timer's id globally
	// unique, so a stale timer never collides with a re-armed record (whose
	// per-record counter would otherwise restart) or with a later extend.
	r.gen++
	rec.gen = r.gen
	gen := r.gen
	rec.timer = r.afterFunc(r.ttl(), func() { r.autoRevertFire(entity, gen) })
}

// autoRevertFire withdraws the entity when its TTL elapses, REGARDLESS of any
// Cleared event. The generation guard makes a superseded timer (a re-arm, clear,
// kill-switch, or Stop happened) a no-op.
func (r *responder) autoRevertFire(entity netip.Prefix, gen int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rec, ok := r.armed[entity]
	if !ok || rec.gen != gen {
		return // stale timer
	}
	r.withdraw(entity)
}

// withdraw removes one entity's rule and re-registers the remaining owner set. It
// counts both the auto-revert (autoRevertFire) and early-clear (onCleared) paths;
// the bulk revertAll (kill-switch / Stop) does not go through here. Caller holds mu.
func (r *responder) withdraw(entity netip.Prefix) {
	if rec, ok := r.armed[entity]; ok && rec.timer != nil {
		rec.timer.Stop()
	}
	delete(r.armed, entity)
	r.armedCount--
	if r.m != nil {
		r.m.reverted.Inc()
	}
	r.reinstall()
}

// killSwitch reverts every armed rule and forces the responder to shadow.
func (r *responder) killSwitch() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.revertAll()
	r.killed = true
	r.publishStatus()
	if r.m != nil {
		r.m.killswitch.Inc()
	}
	logger().Warn("anomaly-shape: kill-switch engaged, all actions reverted, forced to shadow")
}

// Stop withdraws all owner tables at plugin shutdown / reconfigure. Caller must
// not hold mu.
func (r *responder) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.revertAll()
}

// revertAll clears the armed map, stops all timers, and unregisters the owner
// tables. Caller holds mu.
func (r *responder) revertAll() {
	for _, rec := range r.armed {
		if rec.timer != nil {
			rec.timer.Stop()
		}
	}
	r.armed = make(map[netip.Prefix]*armedRecord)
	r.armedCount = 0
	registerTables(owner, nil)
	if err := applyAll(); err != nil {
		logger().Error("anomaly-shape: failed to revert all", "error", err)
	}
	r.gauge()
}

// gauge publishes the current armed count to the metric and republishes the
// status snapshot. Caller holds mu. Both readers want the same fact, and every
// path that changes the armed set (reinstall, revertAll) already calls this, so
// keeping them together is what makes a missed republish impossible. killSwitch
// republishes again for the kill-switch flag, which it sets after revertAll.
func (r *responder) gauge() {
	if r.m != nil {
		r.m.armed.Set(float64(r.armedCount))
	}
	r.publishStatus()
}

// publishStatus republishes the lock-free snapshot statusSnapshot reads. Caller
// holds mu.
func (r *responder) publishStatus() {
	armed := make([]string, 0, len(r.armed))
	for p := range r.armed {
		armed = append(armed, p.String())
	}
	r.published.Store(&shapeStatus{
		Mode: r.cfg.Mode, Killed: r.killed, Action: r.cfg.Action, ArmedList: armed,
	})
}

// reinstall re-registers the owner's whole table set from the current armed map.
// Caller holds mu.
func (r *responder) reinstall() {
	prefixes := make([]netip.Prefix, 0, len(r.armed))
	for p := range r.armed {
		prefixes = append(prefixes, p)
	}
	registerTables(owner, buildTables(prefixes, r.cfg))
	if err := applyAll(); err != nil {
		logger().Error("anomaly-shape: failed to apply firewall tables", "error", err)
	}
	r.gauge()
}

// shapeStatus is one immutable snapshot of the responder's mode, kill-switch and
// armed set. Never mutated after Store; publishStatus builds a new value on every
// change. ArmedList makes that a two-sided property, unlike the pointer-only
// snapshots of ddos/local and ddos/flowspec: the slice header is copied by value
// but the backing array is not, so statusSnapshot hands out a copy of the list
// rather than let a reader reach it.
type shapeStatus struct {
	Mode      string
	Killed    bool
	Action    string
	ArmedList []string
}

// statusSnapshot returns the published view for the show handler. It takes NO
// lock on purpose -- mu is held across the firewall reconcile, so reading
// through it would make show anomaly-shape wait out a netlink round trip.
//
// The returned view owns its ArmedList: returning *s alone would share the
// backing array with the published snapshot and with every other concurrent
// reader, so one reader sorting or truncating in place would rewrite what all of
// them report. The copy is one allocation on a management-plane read.
func (r *responder) statusSnapshot() shapeStatus {
	if s := r.published.Load(); s != nil {
		out := *s
		out.ArmedList = slices.Clone(s.ArmedList)
		return out
	}
	// Unreachable: newResponder publishes the idle snapshot before the responder
	// is shared. An empty view claims no armed source, which is the fail-closed
	// reading for a responder that has installed nothing.
	return shapeStatus{ArmedList: []string{}}
}
