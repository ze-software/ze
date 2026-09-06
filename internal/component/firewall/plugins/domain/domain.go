// Design: docs/architecture/firewall/firewall-domain-group.md -- firewall domain-group plugin entry point
// Related: schedule.go -- the per-name, per-family TTL schedule this file drives
// Related: cache.go -- the zefs last-good addresses a restart programs from
// Related: changelog.go -- the JSON-lines record of what a name pointed at, and when it moved
//
// Package domain populates nftables sets from what DNS names resolve to.
//
// An operator writes a domain group holding one or more names, and a rule
// matches its addresses through source-domain-group or
// destination-domain-group. Ze asks for each name's A and AAAA records, puts
// the answers into the group's two sets, and asks again when THAT answer
// expires rather than on a shared interval, because a TTL is a property of one
// answer.
//
// Three decisions carry the correctness of this package.
//
// A failed refresh keeps the last good addresses, and only NXDOMAIN empties
// them. Without the resolver's response code a SERVFAIL and a deleted name are
// the same signal, so a firewall would either enforce a stale permit list
// forever or empty a live set because a server was down.
//
// The set is reprogrammed only when the addresses actually changed. A name
// with a 60-second TTL is asked for 1440 times a day, and reprogramming on
// each answer would rewrite the kernel's sets for nothing and fill the change
// log with a record of nothing happening.
//
// A group Ze has never resolved is refused at COMMIT, not tolerated at apply.
// The registry's policy for a rule naming a set no owner supplied is to hold
// the whole table back and warn, which is right when the alternative is a
// blackhole, and wrong here: the operator would learn from a log line after
// traffic already passed unfiltered.

package domain

import (
	"context"
	"errors"
	"fmt"
	"net"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/component/resolve/dns"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

var logger = slogutil.LazyLogger("firewall.domain")

// resolveTimeout bounds one name lookup. The engine answers from its own
// resolver, which carries its own query timeout; this is the outer bound on
// the RPC itself, so a hung engine cannot stop the refresh worker forever.
const resolveTimeout = 30 * time.Second

// tableOwner is the name this plugin registers its firewall tables under.
const tableOwner = "firewall-domain"

// tableNamePrefix is the ownership prefix RegisterTables requires on every
// table name (internal/component/firewall/registry.go).
const tableNamePrefix = "ze_"

// refreshOutcome values label the refresh counter. Four of them are the
// outcomes AC-2 through AC-5 define, so a dashboard shows the same distinction
// the code branches on. The fifth, panic, is the one no acceptance criterion
// names: a refresh that recovered rather than resolving must be countable, or
// a name that fails on every answer looks idle.
const (
	outcomeUnchanged = "unchanged"
	outcomeChanged   = "changed"
	outcomeNXDomain  = "nxdomain"
	outcomeError     = "error"
	outcomePanic     = "panic"
)

type domainMetrics struct {
	addressesCached metrics.Gauge
	refreshOutcomes metrics.CounterVec
	lastRefresh     metrics.Gauge
	dataAge         metrics.Gauge
}

var domainMetricsPtr atomic.Pointer[domainMetrics]

func setMetricsRegistry(reg metrics.Registry) {
	m := &domainMetrics{
		addressesCached: reg.Gauge("ze_firewall_domain_group_addresses_cached", "Total DNS-resolved addresses cached for firewall domain groups."),
		refreshOutcomes: reg.CounterVec("ze_firewall_domain_group_refresh_outcomes_total", "Firewall domain-group refresh outcomes.", []string{"result"}),
		lastRefresh:     reg.Gauge("ze_firewall_domain_group_last_refresh_timestamp", "Unix timestamp of the last firewall domain-group refresh that changed addresses."),
		dataAge:         reg.Gauge("ze_firewall_domain_group_data_age_seconds", "Age in seconds of the oldest resolved addresses the firewall is enforcing."),
	}
	domainMetricsPtr.Store(m)
}

// resolveFunc is how the plugin asks for a name. It exists so a test drives
// the whole refresh path without an engine, and so this file names no
// transport: production passes the SDK's ResolveDNS.
type resolveFunc func(ctx context.Context, name string, qtype uint16) (records []string, ttl uint32, status string, err error)

type domainPlugin struct {
	plugin  *sdk.Plugin
	resolve resolveFunc

	cache     *store
	changeLog *changeLog

	mu            sync.RWMutex
	config        *domainConfig
	workerStarted bool

	// sched carries its own lock, because a command handler rearms a unit it
	// resolved while the worker is asleep on the next one. wake is how any
	// goroutine asks the worker to look at the schedule again.
	sched *schedule
	wake  chan struct{}

	stopCh   chan struct{}
	stopOnce sync.Once
	done     chan struct{}

	lastRefresh atomic.Int64 // unix seconds; 0 means no refresh has changed anything
}

func runFirewallDomain(conn net.Conn) int {
	p := sdk.NewWithConn("firewall-domain", conn)
	defer func() { _ = p.Close() }()

	plug := newDomainPlugin(p, func(ctx context.Context, name string, qtype uint16) ([]string, uint32, string, error) {
		return p.ResolveDNS(ctx, name, qtype)
	})

	// pendingCfg carries the config OnConfigVerify approved into
	// OnConfigApply, which receives only a diff. Without it a reload would
	// apply the config the daemon started with: the groups, their names and
	// their floors all live here. firewall-irr carries the same field for the
	// same reason.
	var pendingCfg *domainConfig

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		return plug.configure(parseDomainConfig(sections))
	})

	p.OnConfigVerify(func(sections []sdk.ConfigSection) error {
		cfg := parseDomainConfig(sections)
		if err := plug.verify(cfg); err != nil {
			return err
		}
		pendingCfg = cfg
		return nil
	})

	p.OnConfigApply(func(_ []sdk.ConfigDiffSection) error {
		cfg := pendingCfg
		pendingCfg = nil
		if cfg == nil {
			// No verify ran for this transaction, so nothing new was approved:
			// reconcile what is already configured.
			return plug.applyTables()
		}
		return plug.configure(cfg)
	})

	p.OnExecuteCommand(func(_, command string, args []string, _ string) (string, any, error) {
		return plug.handleCommand(command, args)
	})

	p.OnEnrichShow(plug.enrichShow)

	// The refresh worker starts here and not in configure. Its first act is a
	// resolve, which is an engine call, and OnStarted is where the SDK says an
	// engine call is safe: the five-stage handshake has completed, so the
	// startup coordinator is no longer reading the connection for this plugin's
	// `ready` (Plugin.OnStarted, pkg/plugin/sdk/sdk_callbacks.go).
	p.OnStarted(func(context.Context) error {
		plug.startRefreshWorker()
		return nil
	})

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	defer plug.stop()

	if err := p.Run(ctx, sdk.Registration{
		Commands: []sdk.CommandDecl{
			{Name: cmdShowDomainGroup, Description: "Show what each configured domain group's DNS names resolve to", Args: []string{"[<name>]"}},
			{Name: cmdUpdateDomainGroup, Description: "Resolve a domain group's DNS names now and program its set", Args: []string{"<name>"}},
			{Name: cmdClearDomainGroup, Description: "Remove the addresses cached for a domain group", Args: []string{"<name>"}},
		},
		Enrichers:   []sdk.EnricherDecl{{Command: enrichCommand, Key: enrichKey}},
		WantsConfig: []string{configRoot},
		// The resolved addresses survive a restart in the zefs store, so a
		// crashed process comes back and programs its sets again from the cache.
		// Without the restart the registry holds back every table naming a
		// domain-group set for the life of the daemon.
		FailurePolicy: sdk.FailureRestart,
	}); err != nil {
		logger().Error("firewall-domain plugin failed", "error", err)
		return 1
	}
	return 0
}

// newDomainPlugin builds the plugin over a resolve function. The caller MUST
// call stop when done, and MUST start the refresh worker from OnStarted rather
// than here: the worker resolves as its first act, and a resolve is an engine
// call the five-stage handshake has not finished making room for
// (startRefreshWorker).
func newDomainPlugin(p *sdk.Plugin, resolve resolveFunc) *domainPlugin {
	return &domainPlugin{
		plugin:    p,
		resolve:   resolve,
		cache:     newStore(cacheStorePath()),
		changeLog: newChangeLog(changeLogPath()),
		sched:     newSchedule(),
		wake:      make(chan struct{}, 1),
		stopCh:    make(chan struct{}),
		done:      make(chan struct{}),
	}
}

// verify refuses a config whose referenced domain groups have never resolved.
//
// It performs NO network I/O. The verify callback runs while the commit is
// held open, so resolving here would put a DNS round trip, and every timeout
// it can hit, in the path of an operator pressing return.
//
// The refusal names the group and the command that fetches it, because the
// alternative outcome is the one the registry produces on its own: the table
// is held back with a warning, and the operator learns their filter is not in
// the kernel from a log line, after the traffic it was written for has passed.
func (plug *domainPlugin) verify(cfg *domainConfig) error {
	referenced := cfg.referencedGroups()
	if len(referenced) == 0 {
		return nil
	}

	// Verify runs before configure on a fresh start, so the cache may not be
	// open yet. Reading it here is what makes the refusal true rather than
	// unconditional.
	if err := plug.cache.open(cfg.groups); err != nil {
		return err
	}

	for _, name := range referenced {
		g, ok := cfg.groupByName(name)
		if !ok {
			var tb textbuf.Buffer
			tb.Str("firewall domain-group: a rule names group ").Str(name)
			tb.Str(", which no domain-group defines")
			return errors.New(tb.String())
		}
		if len(g.Names) == 0 {
			var tb textbuf.Buffer
			tb.Str("firewall domain-group: group ").Str(name).Str(" holds no domain-names, so it filters nothing")
			return errors.New(tb.String())
		}
		if plug.cache.hasAddresses(g) {
			continue
		}
		return errors.New(uncachedGroupMessage(name))
	}
	return nil
}

// uncachedGroupMessage names the group with no addresses and the command that
// resolves it.
func uncachedGroupMessage(groupName string) string {
	var tb textbuf.Buffer
	tb.Str("firewall domain-group: no resolved addresses for ").Str(groupName)
	tb.Str("; run 'update firewall domain-group ").Str(groupName).Str("' first")
	return tb.String()
}

// configure installs cfg as the running configuration: it reads the cache,
// programs the tables from it, and rearms the schedule.
//
// The cache is read and the tables programmed BEFORE any name is resolved,
// which is AC-7: a box rebooting with its upstream DNS down resumes filtering
// from what it learned last time rather than waiting for a query that will not
// answer.
func (plug *domainPlugin) configure(cfg *domainConfig) error {
	if err := plug.cache.open(cfg.groups); err != nil {
		logger().Warn("firewall-domain: address cache open failed, groups will filter nothing until their names resolve",
			"error", err)
	}
	if err := plug.changeLog.open(); err != nil {
		logger().Warn("firewall-domain: change log open failed, a DNS change will not be recorded", "error", err)
	}

	plug.mu.Lock()
	plug.config = cfg
	plug.mu.Unlock()

	// Report at startup what a commit would have refused. A daemon start reads
	// its config through OnConfigure alone, so no verify runs, and a group with
	// no addresses would otherwise be a table silently held back.
	plug.warnUncachedGroups(cfg)

	if err := plug.applyTables(); err != nil {
		logger().Warn("firewall-domain: apply failed", "error", err)
	}

	// The schedule is armed here and the worker is NOT started here. A name
	// with no cached answer is due immediately, so a worker started at this
	// point resolves at once, and resolving is an engine call: OnConfigure runs
	// while the engine is waiting for its response, and the startup coordinator
	// reads that request where it expects the plugin's `ready` (Plugin.OnStarted,
	// pkg/plugin/sdk/sdk_callbacks.go). The daemon then refuses to start with
	// "stage 5: expected ready, got ze-plugin-engine:resolve-dns". The worker is
	// started from OnStarted instead (runFirewallDomain), and a nudge arriving
	// before it exists is not lost: reset leaves every unit due, and the worker
	// reads the schedule as its first act.
	plug.rearmSchedule(cfg)

	logger().Debug("configured", "groups", len(cfg.groups), "references", len(cfg.refs))
	return nil
}

// warnUncachedGroups logs the refusal message verify would have produced for
// every referenced group with no addresses. It reports all of them, where
// verify stops at the first: a commit needs one reason to refuse, and an
// operator reading a startup log needs the whole list.
func (plug *domainPlugin) warnUncachedGroups(cfg *domainConfig) {
	for _, name := range cfg.referencedGroups() {
		g, ok := cfg.groupByName(name)
		if ok && plug.cache.hasAddresses(g) {
			continue
		}
		logger().Warn(uncachedGroupMessage(name),
			"effect", "the rules naming it filter nothing until its names resolve")
	}
}

// applyTables builds the sets from the cache and hands them to the firewall
// registry.
//
// No lock is held across ApplyAll: it reconciles the whole kernel ruleset for
// every owner, and holding this plugin's lock across it would block a command
// or a config apply for the length of a netlink round trip.
func (plug *domainPlugin) applyTables() error {
	plug.mu.RLock()
	cfg := plug.config
	plug.mu.RUnlock()

	if cfg == nil || len(cfg.refs) == 0 {
		// A withdraw registers no name, so it cannot be refused.
		_ = firewall.RegisterTables(tableOwner, nil)
		return nil
	}

	tables := buildTables(cfg, func(g group, fam family) []string {
		return plug.cache.groupAddresses(g.Name, g.Names, fam)
	})
	if err := firewall.RegisterTables(tableOwner, tables); err != nil {
		return fmt.Errorf("firewall-domain: register: %w", err)
	}
	if err := firewall.ApplyAll(); err != nil {
		return fmt.Errorf("firewall-domain: apply: %w", err)
	}
	plug.updateMetricsGauges(cfg)
	return nil
}

// startRefreshWorker starts the single refresh goroutine, once. The worker
// runs until stop closes stopCh, and stop then waits for done.
//
// The caller MUST NOT call it before this plugin's five-stage startup has
// completed. The worker's first act is a resolve, which is an engine call, and
// an engine call made while the startup coordinator is waiting for the plugin's
// `ready` is read as that message. OnStarted is the callback the SDK names as
// the safe place for an engine call, so that is where the process form starts
// it.
func (plug *domainPlugin) startRefreshWorker() {
	plug.mu.Lock()
	started := plug.workerStarted
	plug.workerStarted = true
	plug.mu.Unlock()
	if started {
		return
	}
	go plug.refreshWorker()
}

// stop ends the refresh worker and releases the cache. It MUST be called
// before the process exits, and the caller MUST NOT use the plugin after it.
func (plug *domainPlugin) stop() {
	plug.stopOnce.Do(func() {
		close(plug.stopCh)
		plug.mu.RLock()
		started := plug.workerStarted
		plug.mu.RUnlock()
		if started {
			<-plug.done
		}
		plug.cache.close()
	})
}

// rearmSchedule replaces the schedule for the given config and wakes the
// worker, which may be asleep until a due time the new config moved.
func (plug *domainPlugin) rearmSchedule(cfg *domainConfig) {
	plug.sched.reset(cfg.groups, time.Now())
	plug.nudge()
}

// nudge asks the worker to look at the schedule now. It never blocks: the
// channel holds one slot, and a nudge that finds it full has already been
// delivered by whoever filled it.
func (plug *domainPlugin) nudge() {
	select {
	case plug.wake <- struct{}{}:
	default:
	}
}

// refreshWorker is the plugin's one long-lived goroutine. It sleeps until the
// earliest-due unit and refreshes that one, so N names and 2 families cost one
// goroutine and one timer.
//
// The recover covers one unit, not the worker. The records it parses came from
// an upstream DNS server, which is untrusted input reaching a parser, and
// ze-go-style.md states a peer MUST NOT be able to panic the daemon. Unguarded,
// such a panic takes the whole firewall-domain process; the manager respawns
// it, but an answer that panics every time crash-loops into the respawn limit,
// and the registry then holds back every table naming a domain-group set. One
// lost cycle costs nothing by comparison: the cache is what the firewall
// enforces, and it is still there on the next one.
func (plug *domainPlugin) refreshWorker() {
	defer close(plug.done)

	timer := time.NewTimer(time.Hour)
	if !timer.Stop() {
		<-timer.C
	}
	defer timer.Stop()

	for {
		key, due, found := plug.sched.nextDue()
		if !found {
			// Nothing to refresh. Wait for a config or a command rather than
			// polling: an idle plugin costs no wake-ups.
			select {
			case <-plug.wake:
			case <-plug.stopCh:
				return
			}
			continue
		}

		timer.Reset(max(time.Until(due), 0))

		select {
		case <-timer.C:
			plug.refreshOne(key)
		case <-plug.wake:
			// A configure replaced the schedule, or a command rearmed a unit.
			// Go round again and read what is due now.
			stopTimer(timer)
		case <-plug.stopCh:
			stopTimer(timer)
			return
		}
	}
}

// stopTimer drains a timer that may or may not have fired, so the next Reset
// starts from an empty channel.
func stopTimer(t *time.Timer) {
	if t.Stop() {
		return
	}
	select {
	case <-t.C:
	default:
	}
}

// refreshOne resolves a single name and family, records what changed, and
// reprograms only when it did.
func (plug *domainPlugin) refreshOne(key nameKey) {
	defer func() {
		if r := recover(); r != nil {
			incRefreshOutcome(outcomePanic)
			logger().Error("firewall-domain: panic during refresh, keeping the cached addresses",
				"group", key.group, "name", key.name, "family", key.family.label,
				"panic", r, "stack", string(debug.Stack()))
			// Rearm anyway. A unit left unscheduled would never be asked for
			// again, so one bad answer would freeze that name for the life of
			// the process.
			plug.sched.arm(key, 0, plug.floorFor(key.group), time.Now())
		}
	}()

	changed, err := plug.resolveAndRecord(key)
	if err != nil {
		logger().Warn("firewall-domain: refresh failed, keeping the cached addresses",
			"group", key.group, "name", key.name, "family", key.family.label, "error", err)
	}
	if !changed {
		return
	}
	if applyErr := plug.applyTables(); applyErr != nil {
		logger().Warn("firewall-domain: apply after refresh failed",
			"group", key.group, "name", key.name, "error", applyErr)
	}
}

// resolveAndRecord asks for one name and family and decides what the answer
// means. It returns whether the programmed addresses changed.
//
// The four outcomes are AC-2 through AC-5, and the classification is the whole
// point of surfacing the resolver's response code:
//
//   - a transport error keeps the last good addresses (AC-4);
//   - SERVFAIL, REFUSED and any other non-authoritative code keep them too,
//     because they describe the SERVER and say nothing about the name;
//   - NXDOMAIN empties this name's contribution, because it is the server
//     saying authoritatively that the name is gone (AC-5);
//   - NOERROR is authoritative for the name, so its addresses are what the
//     group holds, and an empty NOERROR for AAAA on an IPv4-only name is a
//     correct empty rather than a failure.
//
// The schedule is rearmed in every path, including the failing ones. A unit
// that failed and was left unscheduled would never be asked for again.
func (plug *domainPlugin) resolveAndRecord(key nameKey) (changed bool, err error) {
	floor := plug.floorFor(key.group)

	ctx, cancel := context.WithTimeout(context.Background(), resolveTimeout)
	defer cancel()

	records, ttl, statusText, resolveErr := plug.resolve(ctx, key.name, key.family.qtype)
	now := time.Now()

	previous, seen := plug.cache.get(key)

	if resolveErr != nil {
		incRefreshOutcome(outcomeError)
		plug.sched.arm(key, 0, floor, now)
		plug.markFailing(key, previous, seen, dns.StatusUnspecified, now)
		return false, resolveErr
	}

	status := dns.ParseStatus(statusText)
	if !status.Authoritative() {
		// The server could not answer. Keeping what we have is the only safe
		// reading: emptying the set here would enforce a server outage.
		incRefreshOutcome(outcomeError)
		plug.sched.arm(key, ttl, floor, now)
		plug.markFailing(key, previous, seen, status, now)
		return false, fmt.Errorf("firewall-domain: %s %s answered %s", key.name, key.family.label, status)
	}

	var addresses []string
	if status == dns.StatusSuccess {
		var truncated bool
		addresses, truncated = addressesFromRecords(records, key.family, maxAddressesPerName)
		if truncated {
			logger().Warn("firewall-domain: answer exceeds the per-name address cap, dropping the rest",
				"group", key.group, "name", key.name, "family", key.family.label,
				"answered", len(records), "cap", maxAddressesPerName,
				"effect", "the group permits or denies only the addresses Ze kept")
		}
	}

	plug.sched.arm(key, ttl, floor, now)

	if sameAddresses(previous.Addresses, addresses) {
		// AC-2: no set is reprogrammed, no cache write happens, and no record
		// is logged. This is the common case for a name that does not move,
		// and it is what keeps a 60-second TTL from rewriting the kernel 1440
		// times a day.
		incRefreshOutcome(outcomeUnchanged)
		plug.recordSteadyAnswer(key, previous, seen, addresses, status, now)
		return false, nil
	}

	if status == dns.StatusNameError {
		incRefreshOutcome(outcomeNXDomain)
	} else {
		incRefreshOutcome(outcomeChanged)
	}

	writeErr := plug.cache.put(key, resolvedName{
		Addresses:  addresses,
		ResolvedAt: now,
		Status:     status.String(),
	})
	if writeErr != nil {
		// The addresses are in memory and will be programmed, but the next
		// boot will not have them. Report it rather than swallow it: a cache
		// that stopped persisting is exactly what makes AC-7 fail silently.
		logger().Warn("firewall-domain: address cache write failed, the next restart will not have these addresses",
			"group", key.group, "name", key.name, "error", writeErr)
	}

	markRefreshChanged()
	plug.lastRefresh.Store(now.Unix())

	if logErr := plug.changeLog.append(changeRecord{
		Time:   now,
		Group:  key.group,
		Name:   key.name,
		Family: key.family.label,
		Status: status.String(),
		Old:    previous.Addresses,
		New:    addresses,
	}); logErr != nil {
		logger().Warn("firewall-domain: change log append failed", "group", key.group, "name", key.name, "error", logErr)
	}

	logger().Info("firewall-domain: addresses changed",
		"group", key.group, "name", key.name, "family", key.family.label,
		"status", status.String(), "was", len(previous.Addresses), "now", len(addresses))
	return true, nil
}

// recordSteadyAnswer persists an authoritative answer that did NOT change the
// addresses, but only when it says something the store does not already hold.
//
// Two cases earn a write and no others, because AC-2 requires the steady state
// to write nothing:
//
//   - The FIRST answer for this name and family, even when it carries no
//     address. "Never asked" and "asked, and it holds nothing" are different
//     facts, and only a written entry can tell them apart: an IPv4-only name
//     answers NOERROR-empty for AAAA forever, and without this its IPv6 family
//     would read as unqueried for the life of the box.
//   - The answer that ENDS an outage. The failing marker was written when the
//     outage started, so leaving it would report a healthy name as failing.
//
// A name that keeps answering the same addresses matches neither, so a
// 60-second TTL writes to the store exactly once.
func (plug *domainPlugin) recordSteadyAnswer(key nameKey, previous resolvedName, seen bool, addresses []string, status dns.Status, now time.Time) {
	if seen && !previous.failing() {
		return
	}
	if err := plug.cache.put(key, resolvedName{
		Addresses:  addresses,
		ResolvedAt: now,
		Status:     status.String(),
	}); err != nil {
		logger().Warn("firewall-domain: address cache write failed",
			"group", key.group, "name", key.name, "family", key.family.label, "error", err)
	}
}

// markFailing records that this name and family has stopped resolving, once,
// at the start of the outage.
//
// The addresses stay exactly as they were: that is the whole point of keeping
// the last good answer. What changes is that an operator, and `ze doctor`, can
// now see that the firewall is enforcing addresses whose name has not answered
// since a stated time. Without it a week-old outage and a healthy name look
// identical, because both hold the same addresses.
//
// A name that is already marked writes nothing, so an outage costs one write
// however long it lasts. A name with no entry at all writes nothing either:
// there are no addresses to describe, and the group is already reported as
// having none.
func (plug *domainPlugin) markFailing(key nameKey, previous resolvedName, seen bool, status dns.Status, now time.Time) {
	if !seen || previous.failing() {
		return
	}
	previous.Status = status.String()
	previous.FailingSince = now
	if err := plug.cache.put(key, previous); err != nil {
		logger().Warn("firewall-domain: address cache write failed",
			"group", key.group, "name", key.name, "family", key.family.label, "error", err)
	}
}

// floorFor is the ttl-floor of the group, or the default when the group is
// gone from the config between a schedule entry and its refresh.
func (plug *domainPlugin) floorFor(groupName string) uint32 {
	plug.mu.RLock()
	cfg := plug.config
	plug.mu.RUnlock()
	if g, ok := cfg.groupByName(groupName); ok {
		return g.TTLFloor
	}
	return ttlFloorDefault
}

// refreshGroupNow resolves every name of a group at once and programs what it
// learned. It is what `update firewall domain-group` runs, and it is the only
// path back from a cold cache: nothing is registered at startup for a group
// that has never resolved, so the rules naming it stay held back until this
// runs.
//
// It reports the first failure and keeps going, because an operator asking for
// a group wants every name they can get rather than a stop at the first one.
func (plug *domainPlugin) refreshGroupNow(g group) (changed int, err error) {
	var firstErr error
	for _, name := range g.Names {
		for _, fam := range families {
			key := nameKey{group: g.Name, name: name, family: fam}
			didChange, resolveErr := plug.resolveAndRecord(key)
			if resolveErr != nil && firstErr == nil {
				firstErr = resolveErr
			}
			if didChange {
				changed++
			}
		}
	}
	// resolveAndRecord rearmed each unit from this goroutine, so wake the
	// worker: it may be asleep until a due time that has just moved.
	plug.nudge()

	if applyErr := plug.applyTables(); applyErr != nil {
		return changed, applyErr
	}
	return changed, firstErr
}

func incRefreshOutcome(result string) {
	if m := domainMetricsPtr.Load(); m != nil {
		m.refreshOutcomes.With(result).Inc()
	}
}

// markRefreshChanged stamps the last-refresh gauge. Only a refresh that
// CHANGED the addresses calls it: a gauge that also moved on every confirming
// refresh would answer "when did we last query", which nobody asks.
func markRefreshChanged() {
	if m := domainMetricsPtr.Load(); m != nil {
		m.lastRefresh.Set(float64(time.Now().Unix()))
	}
}

// updateMetricsGauges publishes how many addresses are enforced and how old
// the oldest of them is.
func (plug *domainPlugin) updateMetricsGauges(cfg *domainConfig) {
	m := domainMetricsPtr.Load()
	if m == nil || cfg == nil {
		return
	}
	total := 0
	var oldest time.Duration
	now := time.Now()
	for _, g := range cfg.groups {
		for key, entry := range plug.cache.entriesFor(g) {
			_ = key
			total += len(entry.Addresses)
			if entry.ResolvedAt.IsZero() {
				continue
			}
			if age := now.Sub(entry.ResolvedAt); age > oldest {
				oldest = age
			}
		}
	}
	m.addressesCached.Set(float64(total))
	m.dataAge.Set(oldest.Seconds())
}
