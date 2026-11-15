// Design: docs/architecture/bgp/filter-irr.md -- IRR prefix-list filter plugin entry point
// Related: stored.go -- the reader that feeds publishLocked from the irr plugin's store
// Related: doctor.go -- the runtime doctor check over what the reader could not read

package filter_irr

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/component/resolve/irr"
	"github.com/ze-software/ze/internal/component/resolve/irr/store"
	"github.com/ze-software/ze/internal/core/slogutil"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

var errFilterIrrInvalidBgpConfigJson = errors.New("filter-irr: invalid bgp config JSON")

const configRootBGP = "bgp"

var logger = slogutil.LazyLogger("bgp.filter.irr")

// asnState is one enrolled ASN: the set it is filtered by and the stored copy
// of that set the filter enforces. The irr plugin fetches and stores the
// copy; the reader (stored.go) reads it in.
type asnState struct {
	asn uint32
	// configured is the as-set the operator wrote for this ASN, in the one
	// spelling every store key uses (store.ASSetName), empty when the config
	// names none and PeeringDB chooses it through the irr plugin's binding.
	configured string
	// keyErr is why the configured as-set cannot name a stored copy; the ASN
	// then enforces no list. Empty for every name the YANG admits.
	keyErr string
	// asSet is the set whose stored copy is enforced: configured, or the one
	// the binding names, empty before a binding is read. It is what
	// `show bgp irr` reports.
	asSet string
	// list is the enforced copy, nil while nothing is stored. pointer names
	// it; checkedAt and refreshedAt are the dataset's times.
	list        *irrPrefixList
	pointer     store.Pointer
	checkedAt   time.Time
	refreshedAt time.Time
	// readErr is why the last read could not use a stored value: the list,
	// if any, is the copy read before. Empty when the list is current.
	readErr   string
	v4Count   int
	v6Count   int
	peerAddrs []string
}

// newASNState builds an enrolled ASN's state, with no list yet.
func newASNState(asn uint32, configured string) *asnState {
	st := &asnState{asn: asn}
	st.configure(configured)
	return st
}

// configure records the configured as-set in its key spelling.
func (st *asnState) configure(configured string) {
	if configured == "" {
		st.configured, st.asSet, st.keyErr = "", "", ""
		return
	}
	name, err := store.ASSetName(configured)
	if err != nil {
		st.configured, st.asSet, st.keyErr = configured, configured, err.Error()
		return
	}
	st.configured, st.asSet, st.keyErr = name, name, ""
}

// carriedOverBy reports whether next, the state a reload built for this ASN,
// keeps this state's list: it does only while the configured as-set is the
// same, including both being empty. A changed or deleted set answers a
// question the operator no longer asks, so next starts from no list and the
// reader reads the new set's stored copy.
func (st *asnState) carriedOverBy(next *asnState) bool {
	if next == nil {
		return false
	}
	return next.configured == st.configured
}

// irrSnapshot is what the filter evaluator reads: one immutable value per
// publish, swapped whole, so a route is judged wholly by one config and one
// set of lists (AC-34). It holds every enrolled ASN, mapped to its list, nil
// while nothing is stored. The writers of byASN publish it under mu
// (publishLocked); the evaluator takes no lock and never waits.
type irrSnapshot struct {
	asns map[uint32]irrSnapshotEntry
}

// irrSnapshotEntry is one enrolled ASN in a snapshot.
type irrSnapshotEntry struct {
	list   *irrPrefixList // nil while the ASN has no stored copy
	failed bool           // the last read could not use a stored value (the list, if any, is the one read before)
}

// lookup answers asn's entry and whether this config enrolls it. Safe on a
// nil snapshot, which enrolls nothing: the plugin before its first configure.
func (snap *irrSnapshot) lookup(asn uint32) (irrSnapshotEntry, bool) {
	if snap == nil {
		return irrSnapshotEntry{}, false
	}
	entry, ok := snap.asns[asn]
	return entry, ok
}

// interestFailure is the last refused write of the interest key.
type interestFailure struct {
	err   string
	since time.Time
}

type irrPlugin struct {
	plugin *sdk.Plugin
	// started is set by OnStarted, after the startup handshake. Configure runs
	// inside that handshake, where no state RPC is served, so the first
	// configure leaves the store read to OnStarted, and only a reload
	// configure, which arrives after it, asks the reader itself. Guarded by mu.
	started bool

	mu    sync.RWMutex
	byASN map[uint32]*asnState
	// snapshot is byASN as the evaluator reads it, republished by every
	// writer of byASN or of a state's list (publishLocked).
	snapshot atomic.Pointer[irrSnapshot]
	config   *irrConfig

	// reader reads the irr plugin's stored lists and writes the filter's
	// interest key (stored.go). Nil in tests that build the plugin as a
	// literal and publish lists themselves.
	reader *storeReader
	// interestErr is the last refused interest write, nil once one succeeds.
	interestErr atomic.Pointer[interestFailure]

	// readmit holds the prefixes the filter rejected because the list did not
	// authorize them yet (readmit.go). Nil in tests that build the plugin as a
	// literal, where nothing is held.
	readmit *readmitter
}

func runFilterIRR(conn net.Conn) int {
	p := sdk.NewWithConn(pluginName, conn)
	defer func() { _ = p.Close() }()

	plug := &irrPlugin{
		plugin: p,
		byASN:  make(map[uint32]*asnState),
	}
	plug.reader = newStoreReader(plug, p)
	plug.readmit = newReadmitter(dispatchEngine{plugin: p}, plug, readmitInterval)

	// A plugin holding OnConfigure alone answers every config-apply OK and
	// changes nothing, so a live IRR edit was accepted and ignored. The reload
	// pair is what makes a commit reach the filter (config_tx.go).
	newIRRConfigTx(plug).Register(p)

	// The one evaluator registration (FR-8): the typed bridge slot when the
	// plugin runs internal, the filter-update batch when it runs forked.
	p.OnFilter(plug.evaluate)

	p.OnExecuteCommand(func(serial, command string, args []string, peer string) (string, any, error) {
		return plug.handleCommand(command, args)
	})
	p.OnDoctorCheck(plug.answerDoctorCheck)

	// The irr plugin's notice of a stored or removed dataset. Subscribed at
	// stage 5, before any event flows, so a notice sent between the
	// subscription and the OnStarted read is delivered and queued.
	p.AddStartupSubscriptionsIn(store.NoticeNamespace, []string{store.EventDatasetStored})
	p.OnEvent(func(event string) error {
		plug.reader.notice(event)
		return nil
	})

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	stop := make(chan struct{})
	defer close(stop)
	p.OnStarted(func(_ context.Context) error {
		plug.mu.Lock()
		plug.started = true
		plug.mu.Unlock()
		// Both workers live as long as the plugin, across every configure
		// cycle: the readmitter stops with stop, the reader with ctx.
		go plug.readmit.run(stop)
		go plug.reader.run(ctx)
		plug.reader.readAll()
		return nil
	})

	if err := p.Run(ctx, &sdk.Registration{
		Commands: commandDecls(),
		// A forked program has no registry row to supply ConfigReads, so its
		// Stage 1 declaration must request the policy settings as well as peers.
		WantsConfig:  []string{configRootBGP, policy.RootName},
		Filters:      filterDecls(),
		DoctorChecks: []sdk.DoctorCheckDecl{storedCopyCheckDecl},
	}); err != nil {
		logger().Error("filter-irr plugin failed", "error", err)
		return 1
	}
	return 0
}

// handleConfigure makes cfg the filter's config, at boot and on every
// applied or rolled-back reload. The new per-ASN state is published under
// plug.mu in one swap, so a concurrent filter UPDATE reads the old state or
// the new one, never a mix. It reads and writes no store: the reader does,
// on its own worker, once the plugin has started.
func (plug *irrPlugin) handleConfigure(delivery irrDelivery) {
	cfg := parseIRRConfig(delivery.policy, delivery.bgp)

	newByASN := make(map[uint32]*asnState)
	for _, peer := range cfg.Peers {
		if peer.Disabled {
			continue
		}
		// Enroll ONLY peers that asked for IRR filtering: a peer with no
		// "import { filters [ bgp-filter-irr:<asn> ]; final accept; }"
		// reference has no `show bgp irr` entry (docs/guide/irr-filtering.md),
		// and its AS number is no interest the irr plugin fetches for. An
		// unenrolled peer's filter chain never reaches handleFilterUpdate.
		if !peer.UsesIRR {
			continue
		}
		st, exists := newByASN[peer.RemoteASN]
		if !exists {
			st = newASNState(peer.RemoteASN, peer.ASSet)
			newByASN[peer.RemoteASN] = st
		}
		st.peerAddrs = append(st.peerAddrs, peer.PeerAddr)
		if peer.ASSet != "" && st.configured == "" {
			st.configure(peer.ASSet)
		}
	}

	// All mutable plugin fields are published under plug.mu so a concurrent
	// `show bgp irr` or reader pass never reads a half-updated configure.
	plug.mu.Lock()
	for asn, oldSt := range plug.byASN {
		newSt := newByASN[asn]
		// A list stored for another AS-SET answers a question the operator no
		// longer asks: carrying it over would filter against the old set until
		// the reader read the new one, so the edit would be accepted and not
		// enforced. Deleting the configured set is a change too, because the
		// binding chooses the set again.
		if !oldSt.carriedOverBy(newSt) {
			continue
		}
		newSt.asSet = oldSt.asSet
		newSt.list = oldSt.list
		newSt.pointer = oldSt.pointer
		newSt.checkedAt = oldSt.checkedAt
		newSt.refreshedAt = oldSt.refreshedAt
		newSt.readErr = oldSt.readErr
		newSt.v4Count = oldSt.v4Count
		newSt.v6Count = oldSt.v6Count
	}
	plug.byASN = newByASN
	plug.publishLocked() // a changed as-set publishes its ASN with no list
	plug.config = cfg
	started := plug.started
	plug.mu.Unlock()

	// A prefix held for a peer this config no longer filters under that ASN
	// has no list to be re-evaluated against, and no peer to ask.
	if plug.readmit != nil {
		owners := make(map[heldOwner]struct{}, len(cfg.Peers))
		for i := range cfg.Peers {
			peer := &cfg.Peers[i]
			if peer.enrolled() && peer.RemoteIP.IsValid() {
				owners[heldOwner{peer: peer.RemoteIP, asn: peer.RemoteASN}] = struct{}{}
			}
		}
		plug.readmit.retain(owners)
	}
	for _, name := range peersMissingRefreshGrant(cfg) {
		logger().Warn("irr: a route this peer sends before its IRR list authorizes it stays rejected until the peer sends it again: the peer does not let bgp-filter-irr ask it for its routes",
			"peer", name,
			"action", "add `attach process bgp-filter-irr { send [ refresh ] }` to this peer, or to its group")
	}
	for _, name := range peersMissingRouteRefresh(cfg) {
		logger().Warn("irr: a route this peer sends before its IRR list authorizes it stays rejected until the peer sends it again: ze does not advertise Route Refresh to this peer, so the capability is never negotiated and bgp-filter-irr cannot ask the peer for its routes",
			"peer", name,
			"action", "add `session { capability { route-refresh enable } }` to this peer, or to its group, and enable Route Refresh on the peer's side too; the capability is negotiated when the session next comes up")
	}

	// A reload reads the stored copies of the sets it newly names and tells
	// the irr plugin, which fetches at once a set it holds nothing for. An
	// UPDATE arriving for such a peer meanwhile is judged at once against no
	// list: its announced prefixes take the filter's on-error reject, are
	// held, and the readmitter asks the peer for them again once the list is
	// published (AC-5, AC-32). The first configure leaves this to OnStarted.
	if started && plug.reader != nil {
		plug.reader.readAll()
	}

	// "peers" counts the peers ENROLLED for IRR filtering, not every BGP peer:
	// an operator reading this line is asking how much IRR work was set up.
	enrolledPeers := 0
	for _, st := range newByASN {
		enrolledPeers += len(st.peerAddrs)
	}
	logger().Debug("configured", "peers", enrolledPeers, "asns", len(newByASN))
}

// readTargets captures, under mu, what the reader needs of every enrolled ASN
// wanted selects. An ASN whose configured as-set cannot name a stored copy is
// left out: it enforces no list.
func (plug *irrPlugin) readTargets(wanted func(readTarget) bool) []readTarget {
	plug.mu.RLock()
	defer plug.mu.RUnlock()
	targets := make([]readTarget, 0, len(plug.byASN))
	for asn, st := range plug.byASN {
		if st.keyErr != "" {
			continue
		}
		target := readTarget{asn: asn, configured: st.configured, pointer: st.pointer, asSet: st.asSet}
		if wanted(target) {
			targets = append(targets, target)
		}
	}
	return targets
}

// apply installs what the reader read and publishes it, in one swap under
// mu. An ASN a reload replaced or re-pointed while the reader read is left
// alone: the reload asked for its own read. It reports whether a list changed,
// so the readmitter checks what it holds.
func (plug *irrPlugin) apply(targets []readTarget, outcomes map[uint32]readOutcome) bool {
	plug.mu.Lock()
	changed := false
	var failures []readFailure
	for _, target := range targets {
		st := plug.byASN[target.asn]
		if st == nil || st.configured != target.configured {
			continue
		}
		outcome := outcomes[target.asn]
		switch {
		case outcome.err != nil:
			// A stored value exists and cannot be used: keep what is enforced
			// (AC-22) and say so.
			st.readErr = outcome.err.Error()
			failures = append(failures, readFailure{asn: target.asn, asSet: outcome.asSet, err: outcome.err})
		case outcome.notStored:
			// Nothing is stored: not fetched yet, or `clear irr` removed it.
			changed = changed || st.list != nil
			st.asSet = outcome.asSet
			if st.configured != "" {
				st.asSet = st.configured
			}
			st.list, st.pointer = nil, store.Pointer{}
			st.checkedAt, st.refreshedAt = time.Time{}, time.Time{}
			st.readErr, st.v4Count, st.v6Count = "", 0, 0
		case outcome.unchanged:
			st.readErr = ""
		default:
			changed = true
			dataset := outcome.stored.Dataset
			st.asSet = outcome.asSet
			st.list = outcome.list
			st.pointer = outcome.stored.Pointer
			st.checkedAt, st.refreshedAt = dataset.CheckedAt(), dataset.RefreshedAt()
			st.readErr = ""
			st.v4Count, st.v6Count = len(dataset.IPv4()), len(dataset.IPv6())
		}
	}
	plug.publishLocked()
	plug.mu.Unlock()
	for _, failure := range failures {
		logger().Warn("irr: a stored IRR list cannot be read, the list read before stays enforced",
			"asn", failure.asn, "as-set", failure.asSet, "error", failure.err,
			"code", codeIRRStoredCopyUnreadable, "fix", fixFetchAgain(failure.asSet))
	}
	return changed
}

// readFailure is one ASN whose stored value apply could not use.
type readFailure struct {
	asn   uint32
	asSet string
	err   error
}

// readmitAfterPublish wakes the readmitter: a list that arrived or changed may
// now authorize a prefix the filter rejected.
func (plug *irrPlugin) readmitAfterPublish() {
	if plug.readmit != nil {
		plug.readmit.notify()
	}
}

// interest is what the filter tells the irr plugin it enforces: every
// configured as-set, and every AS number whose set PeeringDB chooses.
func (plug *irrPlugin) interest() store.Interest {
	plug.mu.RLock()
	defer plug.mu.RUnlock()
	in := store.Interest{ASSets: []string{}, ASNs: []uint32{}}
	sets := make(map[string]struct{}, len(plug.byASN))
	for asn, st := range plug.byASN {
		switch {
		case st.keyErr != "":
		case st.configured == "":
			in.ASNs = append(in.ASNs, asn)
		default:
			if _, dup := sets[st.configured]; !dup {
				sets[st.configured] = struct{}{}
				in.ASSets = append(in.ASSets, st.configured)
			}
		}
	}
	slices.Sort(in.ASSets)
	slices.Sort(in.ASNs)
	return in
}

// evaluate is the plugin's filter evaluator, on both transports (OnFilter).
// It honors the fast-path contract: it reads only the published snapshot,
// never waits, and keeps nothing of route past its return.
func (plug *irrPlugin) evaluate(route *sdk.FilterRoute) (*sdk.FilterUpdateOutput, error) {
	in, err := route.Input()
	if err != nil {
		return nil, err
	}
	return plug.handleFilterUpdate(&in), nil
}

// handleFilterUpdate judges one UPDATE against the snapshot entry of the ASN
// its filter name carries, from the prefix and withdrawn fields. It never
// waits: an ASN with no list yet rejects its announced prefixes, the filter's
// on-error verdict, keeps the withdrawals, and holds the prefixes for the
// readmitter, which asks the peer for them again once the reader publishes
// the list (FR-10, AC-32). A rejected prefix beside an admitted one, or beside
// a withdrawal, is dropped alone by a prefix remove, which the engine applies
// to the body NLRI and to MP_REACH_NLRI (spec D-5).
func (plug *irrPlugin) handleFilterUpdate(in *sdk.FilterUpdateInput) *sdk.FilterUpdateOutput {
	asn := extractASNFromFilter(in.Filter)
	if asn == 0 {
		logger().Warn("irr filter: cannot extract ASN from filter name", "filter", in.Filter, "peer", in.Peer)
		return &sdk.FilterUpdateOutput{Action: sdk.FilterReject}
	}

	entry, enrolled := plug.snapshot.Load().lookup(asn)
	list := entry.list
	if list != nil && len(list.entries) == 0 {
		list = nil
	}
	judged, err := judgeNLRI(in.Fields, list)
	if err != nil {
		// Fail closed, and hold nothing: nothing in the route can be judged
		// again.
		logger().Warn("irr filter reject", "filter", in.Filter, "peer", in.Peer, "reason", "fields", "error", err)
		return &sdk.FilterUpdateOutput{Action: sdk.FilterReject}
	}
	var out *sdk.FilterUpdateOutput
	if list == nil {
		out = judgeUnlisted(in, &judged, entry.failed)
	} else {
		out = judgeListed(in, &judged)
	}
	// A whole reject admits no announced prefix (judgeUnlisted admits none,
	// judgeListed rejects whole only when none is in list), so the per-prefix
	// verdicts are the whole account of what the update held.
	plug.trackVerdict(in.Peer, asn, enrolled, &judged)
	return out
}

// judgeUnlisted decides an UPDATE for an ASN that has no list yet: every
// announced prefix is rejected, and the withdrawals still go through.
func judgeUnlisted(in *sdk.FilterUpdateInput, judged *routeNLRI, lookupFailed bool) *sdk.FilterUpdateOutput {
	if out, kept := keepWithdrawals(judged); kept {
		logger().Info("irr filter keeps the withdrawals", "filter", in.Filter, "peer", in.Peer,
			"verdict", out.Action, "rejected", judged.rejected, "reason", "no-prefix-list", "lookup-failed", lookupFailed)
		return out
	}
	logger().Info("irr filter reject", "filter", in.Filter, "peer", in.Peer, "rejected", judged.rejected,
		"reason", "no-prefix-list", "lookup-failed", lookupFailed)
	return &sdk.FilterUpdateOutput{Action: sdk.FilterReject}
}

// judgeListed decides an UPDATE against the ASN's list.
func judgeListed(in *sdk.FilterUpdateInput, judged *routeNLRI) *sdk.FilterUpdateOutput {
	// Every judged prefix in list, or none to judge: accept unmodified.
	if judged.rejected == 0 {
		logger().Info("irr filter accept", "filter", in.Filter, "peer", in.Peer, "accepted", judged.accepted)
		return &sdk.FilterUpdateOutput{Action: sdk.FilterAccept}
	}

	// Every announced prefix out of list: drop the announcements and keep the
	// withdrawals, else reject the whole update.
	if judged.accepted == 0 {
		if out, kept := keepWithdrawals(judged); kept {
			logger().Info("irr filter keeps the withdrawals", "filter", in.Filter, "peer", in.Peer,
				"verdict", out.Action, "rejected", judged.rejected)
			return out
		}
		logger().Info("irr filter reject", "filter", in.Filter, "peer", in.Peer, "rejected", judged.rejected)
		return &sdk.FilterUpdateOutput{Action: sdk.FilterReject}
	}

	// Mixed: keep the in-list prefixes and drop the unauthorized ones.
	logger().Info("irr filter modify", "filter", in.Filter, "peer", in.Peer,
		"accepted", judged.accepted, "rejected", judged.rejected)
	return &sdk.FilterUpdateOutput{Action: sdk.FilterModify, Sets: judged.rejectedRemoves()}
}

// RFC 4271 Section 9: "If the UPDATE message contains a non-empty WITHDRAWN
// ROUTES field, the previously advertised routes, whose destinations
// (expressed as IP prefixes) are contained in this field, SHALL be removed
// from the Adj-RIB-In."
//
// keepWithdrawals answers an UPDATE that announces no prefix the filter
// accepts, and reports false when it carries no withdrawal to keep. The
// engine drops a rejected UPDATE whole, its withdrawals with it. A withdrawal
// alone is accepted. Beside rejected announcements it is kept by removing
// every rejected prefix: the engine drops those entries, removes an
// MP_REACH_NLRI left empty, and sends the withdrawals on (policychain
// applyPrefixIntent). Callers MUST have ruled out an accepted prefix first.
func keepWithdrawals(judged *routeNLRI) (*sdk.FilterUpdateOutput, bool) {
	if !judged.withdraws {
		return nil, false
	}
	if judged.rejected == 0 {
		return &sdk.FilterUpdateOutput{Action: sdk.FilterAccept}, true
	}
	return &sdk.FilterUpdateOutput{Action: sdk.FilterModify, Sets: judged.rejectedRemoves()}, true
}

// trackVerdict tells the re-admission worker what one UPDATE's verdict did to
// the prefixes it holds. A withdrawal drops them, an accepted prefix is no
// longer held, and a rejected one is held until its ASN's list authorizes it.
// Only an ASN this config enrolls holds anything: no list of any other ASN is
// ever read, so its hold could only leak.
func (plug *irrPlugin) trackVerdict(peerAddr string, asn uint32, enrolled bool, judged *routeNLRI) {
	if plug.readmit == nil {
		return
	}
	// The path almost every UPDATE takes: nothing to hold and nothing held. The
	// count is atomic, so this path takes no lock.
	holds := enrolled && judged.rejected > 0
	if !holds && plug.readmit.heldCount() == 0 {
		return
	}
	peer, err := netip.ParseAddr(peerAddr)
	if err != nil {
		// The engine names the peer by its address; a name that is not one
		// identifies nothing the worker could ask a ROUTE-REFRESH of.
		logger().Debug("irr: filter UPDATE names no peer address, nothing held", "peer", peerAddr)
		return
	}
	for i := range judged.families {
		plug.readmit.track(peer, asn, enrolled, &judged.families[i])
	}
}

// heldList serves the re-admission worker the ASN's current list, nil when it
// has none (readmitSource).
func (plug *irrPlugin) heldList(asn uint32) *irrPrefixList {
	plug.mu.RLock()
	defer plug.mu.RUnlock()
	if st := plug.byASN[asn]; st != nil {
		return st.list
	}
	return nil
}

func extractASNFromFilter(filter string) uint32 {
	for i := len(filter) - 1; i >= 0; i-- {
		if filter[i] < '0' || filter[i] > '9' {
			if i < len(filter)-1 {
				v, ok := readUint(filter[i+1:])
				if ok && v <= 0xFFFFFFFF {
					return uint32(v) //nolint:gosec // range checked
				}
			}
			return 0
		}
	}
	v, ok := readUint(filter)
	if ok && v <= 0xFFFFFFFF {
		return uint32(v) //nolint:gosec // range checked
	}
	return 0
}

// publishLocked publishes byASN as the snapshot the evaluator reads. Every
// writer of byASN, or of a state's list or readErr, calls it before releasing
// mu, so the snapshot never lags the state show bgp irr reports. A list is
// never written after it is stored in a state, so the snapshot shares it.
// The caller MUST hold plug.mu for writing.
func (plug *irrPlugin) publishLocked() {
	snap := &irrSnapshot{asns: make(map[uint32]irrSnapshotEntry, len(plug.byASN))}
	for asn, st := range plug.byASN {
		snap.asns[asn] = irrSnapshotEntry{list: st.list, failed: st.readErr != ""}
	}
	plug.snapshot.Store(snap)
}

const maxPrefixEntries = 500_000

func prefixListFromIRR(pl irr.PrefixList) ([]prefixEntry, error) {
	// A list over the cap is refused whole, never cut. A cut fills the budget
	// with IPv4 first, so the IPv6 half is lost and every IPv6 route rejected
	// while the refresh reports success.
	total := len(pl.IPv4) + len(pl.IPv6)
	if total > maxPrefixEntries {
		return nil, fmt.Errorf("irr: prefix list of %d entries (%d IPv4, %d IPv6) exceeds the %d-entry cap, "+
			"so it is refused and the previous list stays in force: narrow the AS-SET",
			total, len(pl.IPv4), len(pl.IPv6), maxPrefixEntries)
	}
	entries := make([]prefixEntry, 0, total)
	for _, p := range pl.IPv4 {
		entries = append(entries, prefixEntry{
			prefix: p,
			ge:     uint8(p.Bits()),
			le:     32,
		})
	}
	for _, p := range pl.IPv6 {
		entries = append(entries, prefixEntry{
			prefix: p,
			ge:     uint8(p.Bits()),
			le:     128,
		})
	}
	return entries, nil
}

// commandDecls names the commands this plugin serves and states what each
// answer holds (pkg/plugin/rpc/types.go, CommandDecl).
//
// It has two readers and MUST stay one function. init() puts it on the
// registry.Registration, which anything linking the composition root reads
// without starting an engine, and the runner sends it in the Stage 1
// registration message, which a running daemon reads.
func commandDecls() []sdk.CommandDecl {
	return []sdk.CommandDecl{
		{Name: "show bgp irr", ShortHelp: "Show IRR filter status per ASN"},
		{Name: "show bgp irr prefix", ShortHelp: "Show IRR-resolved prefixes for a peer", Args: []string{"<peer>"}},
		{Name: "show bgp irr check", ShortHelp: "Check if a prefix is accepted by IRR filter", Args: []string{"<peer>", "<prefix>"}},
	}
}

// asnFilterNamePattern covers the filter names this plugin answers, one per
// AS number: a decimal ASN from 1 to 4294967295 with no leading zero, the
// form extractASNFromFilter reads. AS 0 is refused: it is reserved (RFC 7607)
// and no peer's remote AS, and extractASNFromFilter answers 0 for a name it
// cannot read. The ten-digit alternatives bound the value to 32 bits,
// which a digit count alone cannot, so a reference such as
// bgp-filter-irr:4294967296 fails config load instead of rejecting every
// route at run time (D-10).
const asnFilterNamePattern = `[1-9][0-9]{0,8}` +
	`|[1-3][0-9]{9}` +
	`|4[01][0-9]{8}` +
	`|42[0-8][0-9]{7}` +
	`|429[0-3][0-9]{6}` +
	`|4294[0-8][0-9]{5}` +
	`|42949[0-5][0-9]{4}` +
	`|429496[0-6][0-9]{3}` +
	`|4294967[01][0-9]{2}` +
	`|42949672[0-8][0-9]` +
	`|429496729[0-5]`

// filterDecls is the plugin's Stage 1 filter declaration: one filter per AS
// number, on import, failing closed. It reads the announced and the withdrawn
// NLRI, and answers a rejected prefix as a prefix remove, which needs no set
// declared (docs/plugin-development/protocol.md, "The Sets of a filter-update
// answer").
func filterDecls() []sdk.FilterDecl {
	return []sdk.FilterDecl{{
		NamePattern: asnFilterNamePattern,
		Direction:   sdk.FilterImport,
		Reads:       []string{fieldPrefix, fieldWithdrawn},
		OnError:     sdk.OnErrorReject,
	}}
}
