package as112

import (
	"fmt"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/redistevents"
	as112yang "github.com/ze-software/ze/internal/plugins/as112/yang"
	"github.com/ze-software/ze/pkg/ze"
)

// apply and setServing are test-only producer entry points that pin servingFn to
// a constant reporting the SAME state for every family, preserving the single
// serving-bool semantics these tests were written against. They live in _test.go
// (not the production file) so the production reconcile surface stays minimal --
// production wires a live PER-FAMILY servingFn (mgr.servingFor).

// apply reconciles with an explicit (config, serving) pair.
func (p *as112Producer) apply(cfg as112Config, serving bool) {
	p.setServingFn(func(family.Family) bool { return serving })
	p.reconcile(&cfg)
}

// setServing pins the serving state (all families) and reconciles, mirroring a
// runtime serving transition.
func (p *as112Producer) setServing(serving bool) {
	p.setServingFn(func(family.Family) bool { return serving })
	p.reconcile(nil)
}

// ---- config-leaf parsing (Phase 3) ----

// TestParseConfig_ASNDefault112 validates AC-3: asn unset defaults to 112.
func TestParseConfig_ASNDefault112(t *testing.T) {
	cfg, err := parseConfig(`{"service":{"as112":{"enabled":"true"}}}`)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	if cfg.ASN != 112 {
		t.Fatalf("ASN = %d, want 112 (default)", cfg.ASN)
	}
}

// TestParseConfig_ASNExplicit validates AC-4: an explicit asn is parsed.
func TestParseConfig_ASNExplicit(t *testing.T) {
	cfg, err := parseConfig(`{"service":{"as112":{"enabled":"true","asn":"65001"}}}`)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	if cfg.ASN != 65001 {
		t.Fatalf("ASN = %d, want 65001", cfg.ASN)
	}
}

// TestParseConfig_InvalidASN validates AC-12: asn 0 and non-numeric are rejected.
func TestParseConfig_InvalidASN(t *testing.T) {
	if _, err := parseConfig(`{"service":{"as112":{"asn":"0"}}}`); err == nil {
		t.Fatal("asn 0 accepted, want error (0 reserved)")
	}
	if _, err := parseConfig(`{"service":{"as112":{"asn":"nope"}}}`); err == nil {
		t.Fatal("asn non-numeric accepted, want error")
	}
}

// TestParseConfig_WatchdogDefaultTrue validates AC-7: watchdog unset -> true.
func TestParseConfig_WatchdogDefaultTrue(t *testing.T) {
	cfg, err := parseConfig(`{"service":{"as112":{"enabled":"true"}}}`)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	if !cfg.Watchdog {
		t.Fatal("Watchdog = false, want true (default)")
	}
}

// TestParseConfig_WatchdogFalse validates AC-8: watchdog false is parsed.
func TestParseConfig_WatchdogFalse(t *testing.T) {
	cfg, err := parseConfig(`{"service":{"as112":{"enabled":"true","watchdog":"false"}}}`)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	if cfg.Watchdog {
		t.Fatal("Watchdog = true, want false")
	}
}

// TestParseConfig_CommunityWellKnown validates AC-5: well-known names and AA:NN
// parse to the correct uint32 values.
func TestParseConfig_CommunityWellKnown(t *testing.T) {
	cfg, err := parseConfig(`{"service":{"as112":{"enabled":"true","community":["nopeer","no-export","65000:100"]}}}`)
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	want := []uint32{0xFFFFFF04, 0xFFFFFF01, 65000<<16 | 100} // NOPEER, NO_EXPORT, 65000:100
	if len(cfg.Community) != len(want) {
		t.Fatalf("Community len = %d, want %d", len(cfg.Community), len(want))
	}
	for i := range want {
		if cfg.Community[i] != want[i] {
			t.Fatalf("Community[%d] = %#x, want %#x", i, cfg.Community[i], want[i])
		}
	}
}

// TestParseConfig_InvalidCommunity validates AC-12: a malformed community is
// rejected at config time, not emit time.
func TestParseConfig_InvalidCommunity(t *testing.T) {
	if _, err := parseConfig(`{"service":{"as112":{"community":["not-a-community"]}}}`); err == nil {
		t.Fatal("invalid community accepted, want error")
	}
}

// TestParseConfig_TooManyCommunities validates the community count is bounded so
// the COMMUNITIES attribute cannot overflow the BGP UPDATE size limit.
func TestParseConfig_TooManyCommunities(t *testing.T) {
	comms := make([]string, maxCommunities+1)
	for i := range comms {
		comms[i] = fmt.Sprintf(`"%d:1"`, i)
	}
	data := `{"service":{"as112":{"community":[` + strings.Join(comms, ",") + `]}}}`
	if _, err := parseConfig(data); err == nil {
		t.Fatalf("%d communities accepted, want error (max %d)", maxCommunities+1, maxCommunities)
	}
}

// TestMaxCommunitiesMatchesYANG guards against drift between the Go bound
// (maxCommunities) and the YANG `max-elements` on the community leaf-list: both
// cap the same list, so a change to one without the other would let ze-repository-check
// and the engine disagree on acceptance.
func TestMaxCommunitiesMatchesYANG(t *testing.T) {
	yang := as112yang.ZeAs112ConfYANG
	idx := strings.Index(yang, "leaf-list community")
	if idx < 0 {
		t.Fatal("community leaf-list not found in ze-as112-conf.yang")
	}
	after := yang[idx:]
	mi := strings.Index(after, "max-elements")
	if mi < 0 {
		t.Fatal("max-elements not found under the community leaf-list")
	}
	fields := strings.Fields(after[mi:])
	if len(fields) < 2 {
		t.Fatalf("malformed max-elements statement: %q", after[mi:mi+40])
	}
	n, err := strconv.Atoi(strings.TrimRight(fields[1], ";"))
	if err != nil {
		t.Fatalf("parse max-elements value %q: %v", fields[1], err)
	}
	if n != maxCommunities {
		t.Fatalf("YANG max-elements %d != maxCommunities %d -- keep them in sync", n, maxCommunities)
	}
}

// ---- producer reconcile (Phase 4) ----

// capturedBatch is a deep copy of an emitted RouteChangeBatch. The pool releases
// the batch after Emit, so a test fake MUST copy the fields it inspects rather
// than retain the pointer (redistevents ReleaseBatch contract).
type capturedBatch struct {
	afi       uint16
	safi      uint8
	originASN uint32
	community []uint32
	replayID  uint64
	action    redistevents.RouteAction
	prefixes  []netip.Prefix
}

type captureBus struct {
	mu      sync.Mutex
	batches []capturedBatch
}

func (b *captureBus) Emit(_, _ string, payload any) (int, error) {
	rb, ok := payload.(*redistevents.RouteChangeBatch)
	if !ok {
		return 0, nil
	}
	cb := capturedBatch{afi: rb.AFI, safi: rb.SAFI, originASN: rb.OriginASN, replayID: rb.ReplayID}
	cb.community = append([]uint32(nil), rb.Community...)
	for _, e := range rb.Entries {
		cb.prefixes = append(cb.prefixes, e.Prefix)
		cb.action = e.Action // as112 batches are single-action (all add or all remove)
	}
	b.mu.Lock()
	b.batches = append(b.batches, cb)
	b.mu.Unlock()
	return 0, nil
}

func (b *captureBus) Subscribe(_, _ string, _ func(any)) func() { return func() {} }

func (b *captureBus) snapshot() []capturedBatch {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]capturedBatch, len(b.batches))
	copy(out, b.batches)
	return out
}

func (b *captureBus) reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.batches = nil
}

// setBusForTest installs eb as the package EventBus and restores the previous
// value on cleanup, so producer tests do not leak the bus into each other.
func setBusForTest(t *testing.T, eb ze.EventBus) {
	t.Helper()
	prev := eventBusPtr.Load()
	if eb == nil {
		eventBusPtr.Store(nil)
	} else {
		eventBusPtr.Store(&eb)
	}
	t.Cleanup(func() { eventBusPtr.Store(prev) })
}

func findByAFI(batches []capturedBatch, afi uint16) (capturedBatch, bool) {
	for _, b := range batches {
		if b.afi == afi {
			return b, true
		}
	}
	return capturedBatch{}, false
}

// TestAS112Producer_AnnouncesCoveringPrefixes validates AC-1: serving + enabled
// announces the four COVERING prefixes (two per family), not the host addresses,
// carrying the origin ASN.
func TestAS112Producer_AnnouncesCoveringPrefixes(t *testing.T) {
	bus := &captureBus{}
	setBusForTest(t, bus)
	prod := newAS112Producer()

	prod.apply(as112Config{Enabled: true, Watchdog: true, ASN: 112, AddressFamily: addressFamilyBoth}, true)

	batches := bus.snapshot()
	if len(batches) != 2 {
		t.Fatalf("emitted %d batches, want 2 (v4 + v6)", len(batches))
	}
	v4, ok := findByAFI(batches, 1) // AFI 1 = IPv4
	if !ok {
		t.Fatal("no IPv4 batch")
	}
	if v4.action != redistevents.ActionAdd {
		t.Fatalf("v4 action = %v, want add", v4.action)
	}
	if v4.originASN != 112 {
		t.Fatalf("v4 originASN = %d, want 112", v4.originASN)
	}
	wantV4 := []netip.Prefix{
		netip.MustParsePrefix("192.175.48.0/24"),
		netip.MustParsePrefix("192.31.196.0/24"),
	}
	if !prefixesEqual(v4.prefixes, wantV4) {
		t.Fatalf("v4 prefixes = %v, want %v (covering, not host)", v4.prefixes, wantV4)
	}
	v6, ok := findByAFI(batches, 2) // AFI 2 = IPv6
	if !ok {
		t.Fatal("no IPv6 batch")
	}
	wantV6 := []netip.Prefix{
		netip.MustParsePrefix("2620:4f:8000::/48"),
		netip.MustParsePrefix("2001:4:112::/48"),
	}
	if !prefixesEqual(v6.prefixes, wantV6) {
		t.Fatalf("v6 prefixes = %v, want %v", v6.prefixes, wantV6)
	}
}

// TestAS112Producer_WithheldUntilServing validates AC-7: with the watchdog on
// (default), nothing is announced until serving; then it is.
func TestAS112Producer_WithheldUntilServing(t *testing.T) {
	bus := &captureBus{}
	setBusForTest(t, bus)
	prod := newAS112Producer()

	prod.apply(as112Config{Enabled: true, Watchdog: true, ASN: 112, AddressFamily: addressFamilyBoth}, false)
	if got := len(bus.snapshot()); got != 0 {
		t.Fatalf("announced %d batches while not serving, want 0", got)
	}

	prod.apply(as112Config{Enabled: true, Watchdog: true, ASN: 112, AddressFamily: addressFamilyBoth}, true)
	for _, b := range bus.snapshot() {
		if b.action != redistevents.ActionAdd {
			t.Fatalf("action = %v after serving, want add", b.action)
		}
	}
	if got := len(bus.snapshot()); got != 2 {
		t.Fatalf("announced %d batches after serving, want 2", got)
	}
}

// TestAS112Producer_WithdrawOnServingLoss validates AC-7/AC-9: losing serving
// state withdraws the previously-announced covering prefixes.
func TestAS112Producer_WithdrawOnServingLoss(t *testing.T) {
	bus := &captureBus{}
	setBusForTest(t, bus)
	prod := newAS112Producer()

	cfg := as112Config{Enabled: true, Watchdog: true, ASN: 112, AddressFamily: addressFamilyBoth}
	prod.apply(cfg, true)
	bus.reset()

	prod.apply(cfg, false) // serving lost
	batches := bus.snapshot()
	if len(batches) != 2 {
		t.Fatalf("withdraw emitted %d batches, want 2", len(batches))
	}
	for _, b := range batches {
		if b.action != redistevents.ActionRemove {
			t.Fatalf("action = %v on serving loss, want remove", b.action)
		}
	}
}

// TestAS112Producer_WatchdogFalseAnnouncesWithoutServing validates AC-8:
// watchdog false announces as soon as enabled, without the serving gate.
func TestAS112Producer_WatchdogFalseAnnouncesWithoutServing(t *testing.T) {
	bus := &captureBus{}
	setBusForTest(t, bus)
	prod := newAS112Producer()

	prod.apply(as112Config{Enabled: true, Watchdog: false, ASN: 112, AddressFamily: addressFamilyBoth}, false)
	batches := bus.snapshot()
	if len(batches) != 2 {
		t.Fatalf("watchdog-off announced %d batches while not serving, want 2", len(batches))
	}
	for _, b := range batches {
		if b.action != redistevents.ActionAdd {
			t.Fatalf("action = %v, want add", b.action)
		}
	}
}

// TestAS112Producer_DisabledWithdraws validates AC-9: enabled=false withdraws.
func TestAS112Producer_DisabledWithdraws(t *testing.T) {
	bus := &captureBus{}
	setBusForTest(t, bus)
	prod := newAS112Producer()

	prod.apply(as112Config{Enabled: true, Watchdog: true, ASN: 112, AddressFamily: addressFamilyBoth}, true)
	bus.reset()

	prod.apply(as112Config{Enabled: false, Watchdog: true, AddressFamily: addressFamilyBoth}, false)
	batches := bus.snapshot()
	if len(batches) != 2 {
		t.Fatalf("disable emitted %d batches, want 2 withdraws", len(batches))
	}
	for _, b := range batches {
		if b.action != redistevents.ActionRemove {
			t.Fatalf("action = %v on disable, want remove", b.action)
		}
	}
}

// TestAS112Producer_AddressFamilyDropWithdraws validates that narrowing the
// address-family withdraws only the dropped family, leaving the other announced.
func TestAS112Producer_AddressFamilyDropWithdraws(t *testing.T) {
	bus := &captureBus{}
	setBusForTest(t, bus)
	prod := newAS112Producer()

	prod.apply(as112Config{Enabled: true, Watchdog: true, ASN: 112, AddressFamily: addressFamilyBoth}, true)
	bus.reset()

	// both -> ipv4-only: v6 must be withdrawn, v4 left as-is (no re-emit).
	prod.apply(as112Config{Enabled: true, Watchdog: true, ASN: 112, AddressFamily: addressFamilyIPv4Only}, true)
	batches := bus.snapshot()
	if len(batches) != 1 {
		t.Fatalf("family narrowing emitted %d batches, want 1 (v6 withdraw)", len(batches))
	}
	if batches[0].afi != 2 || batches[0].action != redistevents.ActionRemove {
		t.Fatalf("got afi=%d action=%v, want v6 remove", batches[0].afi, batches[0].action)
	}
}

// TestAS112Producer_AttributeChangeReAnnounces validates AC-4: changing the
// origin ASN while announced re-emits the covering prefixes with the new ASN.
func TestAS112Producer_AttributeChangeReAnnounces(t *testing.T) {
	bus := &captureBus{}
	setBusForTest(t, bus)
	prod := newAS112Producer()

	prod.apply(as112Config{Enabled: true, Watchdog: true, ASN: 112, AddressFamily: addressFamilyBoth}, true)
	bus.reset()

	prod.apply(as112Config{Enabled: true, Watchdog: true, ASN: 65001, AddressFamily: addressFamilyBoth}, true)
	batches := bus.snapshot()
	if len(batches) != 2 {
		t.Fatalf("asn change emitted %d batches, want 2 re-announces", len(batches))
	}
	for _, b := range batches {
		if b.action != redistevents.ActionAdd || b.originASN != 65001 {
			t.Fatalf("got action=%v asn=%d, want add 65001", b.action, b.originASN)
		}
	}
}

// TestAS112Producer_NoChurnOnIdenticalApply validates that re-applying an
// unchanged config while announced emits nothing (no route flap).
func TestAS112Producer_NoChurnOnIdenticalApply(t *testing.T) {
	bus := &captureBus{}
	setBusForTest(t, bus)
	prod := newAS112Producer()

	cfg := as112Config{Enabled: true, Watchdog: true, ASN: 112, AddressFamily: addressFamilyBoth}
	prod.apply(cfg, true)
	bus.reset()

	prod.apply(cfg, true) // identical
	if got := len(bus.snapshot()); got != 0 {
		t.Fatalf("identical re-apply emitted %d batches, want 0", got)
	}
}

// TestAS112Producer_ReemitOnReplay validates AC-14: a replay request re-emits
// the current announced set tagged with the ReplayID, carrying the community.
func TestAS112Producer_ReemitOnReplay(t *testing.T) {
	bus := &captureBus{}
	setBusForTest(t, bus)
	prod := newAS112Producer()

	prod.apply(as112Config{Enabled: true, Watchdog: true, ASN: 112, Community: []uint32{0xFFFFFF04}, AddressFamily: addressFamilyBoth}, true)
	bus.reset()

	prod.reemitAll(42)
	batches := bus.snapshot()
	if len(batches) != 2 {
		t.Fatalf("replay emitted %d batches, want 2", len(batches))
	}
	for _, b := range batches {
		if b.action != redistevents.ActionAdd {
			t.Fatalf("replay action = %v, want add", b.action)
		}
		if b.replayID != 42 {
			t.Fatalf("replay batch ReplayID = %d, want 42", b.replayID)
		}
		if len(b.community) != 1 || b.community[0] != 0xFFFFFF04 {
			t.Fatalf("replay community = %v, want [nopeer]", b.community)
		}
	}
}

// TestAS112Producer_ReemitZeroIsNoop validates that a zero ReplayID is a no-op
// (the orchestrator only allocates nonzero tokens).
func TestAS112Producer_ReemitZeroIsNoop(t *testing.T) {
	bus := &captureBus{}
	setBusForTest(t, bus)
	prod := newAS112Producer()

	prod.apply(as112Config{Enabled: true, Watchdog: true, ASN: 112, AddressFamily: addressFamilyBoth}, true)
	bus.reset()

	prod.reemitAll(0)
	if got := len(bus.snapshot()); got != 0 {
		t.Fatalf("reemitAll(0) emitted %d batches, want 0", got)
	}
}

// TestAS112Producer_ReemitWhenNotAnnouncedIsNoop validates a replay before any
// announcement (or after withdrawal) emits nothing.
func TestAS112Producer_ReemitWhenNotAnnouncedIsNoop(t *testing.T) {
	bus := &captureBus{}
	setBusForTest(t, bus)
	prod := newAS112Producer()

	prod.reemitAll(7) // never announced
	if got := len(bus.snapshot()); got != 0 {
		t.Fatalf("replay before announce emitted %d batches, want 0", got)
	}
}

// TestAS112Producer_WithdrawOnShutdown validates AC-9: the shutdown withdraw
// retracts the current set and is idempotent.
func TestAS112Producer_WithdrawOnShutdown(t *testing.T) {
	bus := &captureBus{}
	setBusForTest(t, bus)
	prod := newAS112Producer()

	prod.apply(as112Config{Enabled: true, Watchdog: true, ASN: 112, AddressFamily: addressFamilyBoth}, true)
	bus.reset()

	prod.withdraw()
	if got := len(bus.snapshot()); got != 2 {
		t.Fatalf("withdraw emitted %d batches, want 2", got)
	}
	bus.reset()
	prod.withdraw() // idempotent
	if got := len(bus.snapshot()); got != 0 {
		t.Fatalf("second withdraw emitted %d batches, want 0 (idempotent)", got)
	}
}

// TestAS112Producer_SetServingWithdrawsAndReannounces validates AC-7: a runtime
// serving-state loss (anycast listener down, no config change) withdraws the
// covering prefixes, and a serving recovery re-announces them.
func TestAS112Producer_SetServingWithdrawsAndReannounces(t *testing.T) {
	bus := &captureBus{}
	setBusForTest(t, bus)
	prod := newAS112Producer()

	prod.apply(as112Config{Enabled: true, Watchdog: true, ASN: 112, AddressFamily: addressFamilyBoth}, true)
	bus.reset()

	prod.setServing(false) // runtime serving loss (e.g. listener crash)
	lost := bus.snapshot()
	if len(lost) != 2 {
		t.Fatalf("serving loss emitted %d batches, want 2 withdraws", len(lost))
	}
	for _, b := range lost {
		if b.action != redistevents.ActionRemove {
			t.Fatalf("action = %v on serving loss, want remove", b.action)
		}
	}
	bus.reset()

	prod.setServing(true) // serving recovers
	back := bus.snapshot()
	if len(back) != 2 {
		t.Fatalf("serving recovery emitted %d batches, want 2 announces", len(back))
	}
	for _, b := range back {
		if b.action != redistevents.ActionAdd {
			t.Fatalf("action = %v on serving recovery, want add", b.action)
		}
	}
}

// TestAS112Producer_ApplyConfigUsesStoredServing validates that applyConfig
// reconciles against the runtime serving state set separately by setServing
// (config and serving are independent inputs to the same reconcile).
func TestAS112Producer_ApplyConfigUsesStoredServing(t *testing.T) {
	bus := &captureBus{}
	setBusForTest(t, bus)
	prod := newAS112Producer()

	prod.setServing(true) // listeners up before any config
	if got := len(bus.snapshot()); got != 0 {
		t.Fatalf("setServing before config emitted %d batches, want 0", got)
	}

	prod.applyConfig(as112Config{Enabled: true, Watchdog: true, ASN: 112, AddressFamily: addressFamilyBoth})
	batches := bus.snapshot()
	if len(batches) != 2 {
		t.Fatalf("applyConfig emitted %d batches, want 2 announces", len(batches))
	}
	for _, b := range batches {
		if b.action != redistevents.ActionAdd {
			t.Fatalf("action = %v, want add", b.action)
		}
	}
}

// TestAS112Producer_ApplyConfigWatchdogOffIgnoresServing validates AC-8 via the
// config path: watchdog false announces even when serving was never set.
func TestAS112Producer_ApplyConfigWatchdogOffIgnoresServing(t *testing.T) {
	bus := &captureBus{}
	setBusForTest(t, bus)
	prod := newAS112Producer()

	prod.applyConfig(as112Config{Enabled: true, Watchdog: false, ASN: 112, AddressFamily: addressFamilyBoth})
	if got := len(bus.snapshot()); got != 2 {
		t.Fatalf("watchdog-off applyConfig emitted %d batches while not serving, want 2", got)
	}
}

// TestAS112Producer_OnServingChangedReadsLive validates the production path: the
// producer reads the LIVE serving state via servingFn on each reconcile (no
// stored snapshot), so onServingChanged withdraws/re-announces from the true
// current serving state -- the property that makes concurrent listener edges
// converge (the ISSUE-1 fix).
func TestAS112Producer_OnServingChangedReadsLive(t *testing.T) {
	bus := &captureBus{}
	setBusForTest(t, bus)
	prod := newAS112Producer()

	var serving atomic.Bool
	prod.setServingFn(func(family.Family) bool { return serving.Load() }) // live read, like the wired mgr.servingFor

	// enabled + watchdog, not serving yet -> withheld.
	prod.applyConfig(as112Config{Enabled: true, Watchdog: true, ASN: 112, AddressFamily: addressFamilyBoth})
	if got := len(bus.snapshot()); got != 0 {
		t.Fatalf("withheld while not serving: emitted %d, want 0", got)
	}

	// Serving comes up -> announce (onServingChanged re-reads the live value).
	serving.Store(true)
	prod.onServingChanged()
	if got := len(bus.snapshot()); got != 2 {
		t.Fatalf("serving up: emitted %d, want 2 announces", got)
	}
	bus.reset()

	// Serving lost at runtime -> withdraw, no config change.
	serving.Store(false)
	prod.onServingChanged()
	lost := bus.snapshot()
	if len(lost) != 2 {
		t.Fatalf("serving lost: emitted %d, want 2 withdraws", len(lost))
	}
	for _, b := range lost {
		if b.action != redistevents.ActionRemove {
			t.Fatalf("action = %v on serving loss, want remove", b.action)
		}
	}
}

// lastActionByAFI returns the action of the LAST batch emitted for afi -- i.e.
// the net wire state of that family (the covering prefixes are announced or
// withdrawn as a unit). ok is false if the family was never emitted.
func lastActionByAFI(batches []capturedBatch, afi uint16) (redistevents.RouteAction, bool) {
	var action redistevents.RouteAction
	found := false
	for _, b := range batches {
		if b.afi == afi {
			action = b.action
			found = true
		}
	}
	return action, found
}

// TestAS112Producer_ConcurrentServingConverges drives concurrent serving-state
// transitions (through a lock-taking servingFn, so the real p.mu->s.mu nesting
// is exercised) alongside concurrent config applies, then asserts BOTH that the
// producer's state converges AND that the emitted wire sequence agrees with it.
// The wire assertion is the emit-ordering guard: reconcile emits UNDER p.mu, so
// the last batch per family must reflect the last committed state. If emits ran
// outside the lock (the pre-fix code), a stale ADD could land after a newer
// WITHDRAW and the final wire action would disagree with p.announced -- a route
// leak / blackhole this test would catch under -race.
func TestAS112Producer_ConcurrentServingConverges(t *testing.T) {
	bus := &captureBus{}
	setBusForTest(t, bus)
	prod := newAS112Producer()

	// servingFn takes its own mutex, mirroring mgr.servingFor's s.mu, so
	// reconcile's p.mu -> servingFn -> s.mu nesting is exercised under -race.
	var smu sync.Mutex
	serving := false
	prod.setServingFn(func(family.Family) bool { smu.Lock(); defer smu.Unlock(); return serving })
	flip := func(v bool) {
		smu.Lock()
		serving = v
		smu.Unlock()
		prod.onServingChanged()
	}

	cfg := as112Config{Enabled: true, Watchdog: true, ASN: 112, AddressFamily: addressFamilyBoth}
	prod.applyConfig(cfg)

	var wg sync.WaitGroup
	for i := range 8 {
		wg.Go(func() {
			for j := range 50 {
				flip((i+j)%2 == 0)
			}
		})
	}
	for range 4 {
		wg.Go(func() {
			for range 50 {
				prod.applyConfig(cfg)
			}
		})
	}
	wg.Wait()

	// Deterministic final edge: up -> announced, and the last wire action for
	// BOTH families must be an add (emit order matches commit order).
	flip(true)
	prod.mu.Lock()
	up := prod.announced
	prod.mu.Unlock()
	if !up {
		t.Fatal("did not converge to announced after final serving=true")
	}
	for _, afi := range []uint16{1, 2} {
		act, ok := lastActionByAFI(bus.snapshot(), afi)
		if !ok || act != redistevents.ActionAdd {
			t.Fatalf("afi %d: last wire action = %v (found=%v), want add to match announced state", afi, act, ok)
		}
	}

	// Deterministic final edge: down -> withdrawn, last wire action is remove.
	flip(false)
	prod.mu.Lock()
	down := prod.announced
	prod.mu.Unlock()
	if down {
		t.Fatal("did not converge to withdrawn after final serving=false")
	}
	for _, afi := range []uint16{1, 2} {
		act, ok := lastActionByAFI(bus.snapshot(), afi)
		if !ok || act != redistevents.ActionRemove {
			t.Fatalf("afi %d: last wire action = %v (found=%v), want remove to match withdrawn state", afi, act, ok)
		}
	}
}

// TestAS112Producer_PerFamilyServingGate validates the per-family serving gate:
// with IPv4 anycast up but IPv6 down, only the IPv4 covering prefixes are
// announced; when IPv6 comes up it is announced without re-emitting IPv4; and an
// IPv4-only outage withdraws only IPv4. A partial-outage family must never be
// announced into a dead server (RFC 7534 Section 3.3).
func TestAS112Producer_PerFamilyServingGate(t *testing.T) {
	bus := &captureBus{}
	setBusForTest(t, bus)
	prod := newAS112Producer()

	var mu sync.Mutex
	v4up, v6up := true, false
	prod.setServingFn(func(f family.Family) bool {
		mu.Lock()
		defer mu.Unlock()
		if f.AFI == family.IPv4Unicast.AFI {
			return v4up
		}
		return v6up
	})

	// Enabled + watchdog, v4 up / v6 down -> only v4 announced.
	prod.applyConfig(as112Config{Enabled: true, Watchdog: true, ASN: 112, AddressFamily: addressFamilyBoth})
	batches := bus.snapshot()
	if len(batches) != 1 {
		t.Fatalf("v4-only serving emitted %d batches, want 1 (v4 add only)", len(batches))
	}
	if batches[0].afi != 1 || batches[0].action != redistevents.ActionAdd {
		t.Fatalf("got afi=%d action=%v, want v4 add", batches[0].afi, batches[0].action)
	}
	bus.reset()

	// v6 comes up -> v6 announced, v4 untouched (no re-emit).
	mu.Lock()
	v6up = true
	mu.Unlock()
	prod.onServingChanged()
	batches = bus.snapshot()
	if len(batches) != 1 {
		t.Fatalf("v6 recovery emitted %d batches, want 1 (v6 add only)", len(batches))
	}
	if batches[0].afi != 2 || batches[0].action != redistevents.ActionAdd {
		t.Fatalf("got afi=%d action=%v, want v6 add", batches[0].afi, batches[0].action)
	}
	bus.reset()

	// v4 outage -> only v4 withdrawn, v6 stays announced.
	mu.Lock()
	v4up = false
	mu.Unlock()
	prod.onServingChanged()
	batches = bus.snapshot()
	if len(batches) != 1 {
		t.Fatalf("v4 outage emitted %d batches, want 1 (v4 remove only)", len(batches))
	}
	if batches[0].afi != 1 || batches[0].action != redistevents.ActionRemove {
		t.Fatalf("got afi=%d action=%v, want v4 remove", batches[0].afi, batches[0].action)
	}
}

func prefixesEqual(a, b []netip.Prefix) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
