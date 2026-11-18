// Design: docs/architecture/l2tp/subscriber-session-model.md -- pool configuration and live allocation ownership

package l2tppool

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/netip"
	"strings"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
	subevents "github.com/ze-software/ze/internal/component/l2tp/subscriber/events"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// reloadPoolSection makes the same root-wrapped section the engine delivers.
func reloadPoolSection(t *testing.T, pool map[string]any) []sdk.ConfigSection {
	t.Helper()
	data, err := json.Marshal(map[string]any{"l2tp": map[string]any{"pool": pool}})
	if err != nil {
		t.Fatal(err)
	}
	return []sdk.ConfigSection{{Root: configRootL2TP, Data: string(data)}}
}

func reloadIPv4(start, end string) map[string]any {
	return map[string]any{"gateway": "10.0.0.1", "start": start, "end": end}
}

func reloadIPv6(block string, length int) map[string]any {
	return map[string]any{"block": block, "delegation-length": length}
}

func reloadSessionDown(p *poolPlugin, tunnel, session uint16) {
	p.onSessionDown(&subevents.SessionDownPayload{Session: subscriber.Session{
		ID:         subscriber.SessionID(subscriber.AccessL2TP, uint64(tunnel), session),
		AccessType: subscriber.AccessL2TP, TunnelID: tunnel, SessionID: session,
	}})
}

// TestPoolConfigTxAppliesReloadedRanges proves verify stages and apply installs
// widened, named and IPv6 pools by allocating through their request handlers.
func TestPoolConfigTxAppliesReloadedRanges(t *testing.T) {
	p := &poolPlugin{}
	tx := p.configTransaction()
	if err := tx.Configure(reloadPoolSection(t, map[string]any{
		"ipv4": reloadIPv4("10.0.0.10", "10.0.0.10"),
	})); err != nil {
		t.Fatal(err)
	}
	old := p.pool
	if err := tx.Verify(reloadPoolSection(t, map[string]any{
		"ipv4":       reloadIPv4("10.0.0.10", "10.0.0.11"),
		"named-pool": map[string]any{"gold": reloadIPv4("10.0.0.20", "10.0.0.20")},
		"ipv6-pd":    reloadIPv6("2001:db8:100::/56", 60),
	})); err != nil {
		t.Fatal(err)
	}
	if p.pool != old {
		t.Fatal("verify changed running pool")
	}
	if err := tx.Apply(); err != nil {
		t.Fatal(err)
	}
	allocate(t, p, 1, 1, subscriber.AccessL2TP)
	if got := allocate(t, p, 1, 2, subscriber.AccessL2TP); got != netip.MustParseAddr("10.0.0.11") {
		t.Fatalf("added range returned %s", got)
	}
	if got := p.namedPools["gold"]; got == nil {
		t.Fatal("named pool not installed")
	}
	if got := p.handlePrefix(l2tp.PrefixRequest{SubscriberID: subscriber.SessionID(subscriber.AccessL2TP, 1, 3), TunnelID: 1, SessionID: 3}); !got.OK {
		t.Fatalf("new IPv6 block refused: %s", got.Reason)
	}
}

// TestPoolReloadCarriesLiveAllocations proves rebuilt bitmaps retain both
// address families, and allocation after apply skips each live assignment.
func TestPoolReloadCarriesLiveAllocations(t *testing.T) {
	p := &poolPlugin{}
	tx := p.configTransaction()
	first := reloadPoolSection(t, map[string]any{
		"ipv4":    reloadIPv4("10.0.0.10", "10.0.0.11"),
		"ipv6-pd": reloadIPv6("2001:db8:100::/56", 60),
	})
	if err := tx.Configure(first); err != nil {
		t.Fatal(err)
	}
	ip := allocate(t, p, 1, 1, subscriber.AccessL2TP)
	prefix := p.handlePrefix(l2tp.PrefixRequest{SubscriberID: subscriber.SessionID(subscriber.AccessL2TP, 1, 1), TunnelID: 1, SessionID: 1})
	if !prefix.OK {
		t.Fatal(prefix.Reason)
	}
	if err := tx.Verify(reloadPoolSection(t, map[string]any{
		"ipv4":    reloadIPv4("10.0.0.10", "10.0.0.12"),
		"ipv6-pd": reloadIPv6("2001:db8:100::/55", 60),
	})); err != nil {
		t.Fatal(err)
	}
	if err := tx.Apply(); err != nil {
		t.Fatal(err)
	}
	if _, used, _ := p.pool.stats(); used != 1 {
		t.Fatalf("carried IPv4 count %d, want 1", used)
	}
	if got := allocate(t, p, 1, 2, subscriber.AccessL2TP); got == ip {
		t.Fatalf("live address %s allocated twice", got)
	}
	if _, used, _ := p.v6pool.stats(); used != 1 {
		t.Fatalf("carried IPv6 count %d, want 1", used)
	}
	if got := p.handlePrefix(l2tp.PrefixRequest{SubscriberID: subscriber.SessionID(subscriber.AccessL2TP, 1, 2), TunnelID: 1, SessionID: 2}); !got.OK || got.Prefix == prefix.Prefix {
		t.Fatalf("live prefix not excluded: %+v", got)
	}
}

// TestPoolReloadOrphanKeepsAddress proves a removed range does not revoke the
// live session record, and its later release does not change an unrelated pool.
func TestPoolReloadOrphanKeepsAddress(t *testing.T) {
	p := &poolPlugin{}
	tx := p.configTransaction()
	if err := tx.Configure(reloadPoolSection(t, map[string]any{"ipv4": reloadIPv4("10.0.0.10", "10.0.0.10")})); err != nil {
		t.Fatal(err)
	}
	ip := allocate(t, p, 1, 1, subscriber.AccessL2TP)
	if err := tx.Verify(reloadPoolSection(t, map[string]any{"ipv4": reloadIPv4("10.0.0.20", "10.0.0.20")})); err != nil {
		t.Fatal(err)
	}
	if err := tx.Apply(); err != nil {
		t.Fatal(err)
	}
	stored, ok := p.sessionAddrs.Load(subscriber.SessionID(subscriber.AccessL2TP, uint64(1), 1))
	if !ok {
		t.Fatal("orphan session record removed")
	}
	allocation, ok := stored.(sessionAddr)
	if !ok {
		t.Fatalf("orphan record has type %T, want sessionAddr", stored)
	}
	if got := allocation.addr; got != ip {
		t.Fatalf("orphan address changed to %s", got)
	}
	allocate(t, p, 1, 2, subscriber.AccessL2TP)
	reloadSessionDown(p, 1, 1)
	if _, used, _ := p.pool.stats(); used != 1 {
		t.Fatalf("orphan release changed new pool: %d", used)
	}
}

// TestPoolReloadCarriedAddressReleasedOnSessionDown proves the reservation's
// new pool owns release after an address moves from default to named.
func TestPoolReloadCarriedAddressReleasedOnSessionDown(t *testing.T) {
	p := &poolPlugin{}
	tx := p.configTransaction()
	if err := tx.Configure(reloadPoolSection(t, map[string]any{"ipv4": reloadIPv4("10.0.0.10", "10.0.0.10")})); err != nil {
		t.Fatal(err)
	}
	ip := allocate(t, p, 1, 1, subscriber.AccessL2TP)
	if err := tx.Verify(reloadPoolSection(t, map[string]any{
		"ipv4":       reloadIPv4("10.0.0.20", "10.0.0.20"),
		"named-pool": map[string]any{"gold": reloadIPv4("10.0.0.10", "10.0.0.10")},
	})); err != nil {
		t.Fatal(err)
	}
	if err := tx.Apply(); err != nil {
		t.Fatal(err)
	}
	if _, used, _ := p.namedPools["gold"].stats(); used != 1 {
		t.Fatalf("named carry count %d", used)
	}
	reloadSessionDown(p, 1, 1)
	if got, ok := p.namedPools["gold"].allocate(); !ok || got != ip {
		t.Fatalf("carried address not released into new owner: %s, %v", got, ok)
	}
}

// TestPoolConfigTxRollbackCarriesAllocations proves rollback reconstructs the
// previous ranges from current session records, including post-apply sessions.
func TestPoolConfigTxRollbackCarriesAllocations(t *testing.T) {
	p := &poolPlugin{}
	tx := p.configTransaction()
	if err := tx.Configure(reloadPoolSection(t, map[string]any{"ipv4": reloadIPv4("10.0.0.10", "10.0.0.11")})); err != nil {
		t.Fatal(err)
	}
	allocate(t, p, 1, 1, subscriber.AccessL2TP)
	if err := tx.Verify(reloadPoolSection(t, map[string]any{"ipv4": reloadIPv4("10.0.0.10", "10.0.0.12")})); err != nil {
		t.Fatal(err)
	}
	if err := tx.Apply(); err != nil {
		t.Fatal(err)
	}
	allocate(t, p, 1, 2, subscriber.AccessL2TP)
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if total, used, _ := p.pool.stats(); total != 2 || used != 2 {
		t.Fatalf("rollback total=%d allocated=%d, want 2/2", total, used)
	}
	if got := p.handle(ppp.EventIPRequest{SubscriberID: subscriber.SessionID(subscriber.AccessL2TP, uint64(1), 3), TunnelID: 1, SessionID: 3, Family: ppp.AddressFamilyIPv4}); got.Accept {
		t.Fatalf("rollback forgot a live reservation: %+v", got)
	}
}

// TestPoolVerifyRefusesBadPool proves a parse failure cannot change running
// state or leave a candidate which a following apply can install.
func TestPoolVerifyRefusesBadPool(t *testing.T) {
	p := &poolPlugin{}
	tx := p.configTransaction()
	if err := tx.Configure(reloadPoolSection(t, map[string]any{"ipv4": reloadIPv4("10.0.0.10", "10.0.0.11")})); err != nil {
		t.Fatal(err)
	}
	old := p.pool
	if err := tx.Verify(reloadPoolSection(t, map[string]any{"ipv4": reloadIPv4("10.0.0.20", "10.0.0.10")})); err == nil {
		t.Fatal("unordered range accepted")
	}
	if p.pool != old {
		t.Fatal("refused verify changed pools")
	}
	if err := tx.Apply(); err == nil {
		t.Fatal("refused candidate applied")
	}
	allocate(t, p, 1, 1, subscriber.AccessL2TP)
}

// TestPoolUnrelatedL2TPEditInstallsNothing proves declarative equality keeps
// the existing pool object and its live bitmap on another l2tp leaf edit.
func TestPoolUnrelatedL2TPEditInstallsNothing(t *testing.T) {
	p := &poolPlugin{}
	tx := p.configTransaction()
	sections := reloadPoolSection(t, map[string]any{"ipv4": reloadIPv4("10.0.0.10", "10.0.0.11")})
	if err := tx.Configure(sections); err != nil {
		t.Fatal(err)
	}
	old := p.pool
	allocate(t, p, 1, 1, subscriber.AccessL2TP)
	var root map[string]map[string]any
	if err := json.Unmarshal([]byte(sections[0].Data), &root); err != nil {
		t.Fatal(err)
	}
	root["l2tp"]["hello-interval"] = "30"
	data, err := json.Marshal(root)
	if err != nil {
		t.Fatal(err)
	}
	sections[0].Data = string(data)
	if err := tx.Verify(sections); err != nil {
		t.Fatal(err)
	}
	if err := tx.Apply(); err != nil {
		t.Fatal(err)
	}
	if p.pool != old {
		t.Fatal("unrelated l2tp edit rebuilt pools")
	}
	if _, used, _ := p.pool.stats(); used != 1 {
		t.Fatalf("live bitmap changed: %d", used)
	}
}

// TestPoolAllocateConcurrentWithApply proves allocation and record insertion
// cannot straddle reconciliation and leave a held address free in the new pool.
func TestPoolAllocateConcurrentWithApply(t *testing.T) {
	p := &poolPlugin{}
	tx := p.configTransaction()
	if err := tx.Configure(reloadPoolSection(t, map[string]any{"ipv4": reloadIPv4("10.0.0.10", "10.0.0.250")})); err != nil {
		t.Fatal(err)
	}
	var workers sync.WaitGroup
	workers.Go(func() {
		for session := uint16(1); session <= 64; session++ {
			p.handle(ppp.EventIPRequest{SubscriberID: subscriber.SessionID(subscriber.AccessL2TP, uint64(1), session), TunnelID: 1, SessionID: session, Family: ppp.AddressFamilyIPv4})
		}
	})
	for end := 251; end <= 254; end++ {
		block := reloadIPv4("10.0.0.10", netip.AddrFrom4([4]byte{10, 0, 0, byte(end)}).String())
		if err := tx.Verify(reloadPoolSection(t, map[string]any{"ipv4": block})); err != nil {
			t.Fatal(err)
		}
		if err := tx.Apply(); err != nil {
			t.Fatal(err)
		}
	}
	workers.Wait()
	seen := make(map[netip.Addr]bool)
	p.sessionAddrs.Range(func(_, value any) bool {
		allocation, ok := value.(sessionAddr)
		if !ok {
			t.Errorf("allocation record has type %T, want sessionAddr", value)
			return false
		}
		addr := allocation.addr
		if seen[addr] {
			t.Errorf("duplicate address %s", addr)
		}
		seen[addr] = true
		return true
	})
	if len(seen) != 64 {
		t.Fatalf("session records=%d, want 64", len(seen))
	}
	if _, used, _ := p.pool.stats(); used != 64 {
		t.Fatalf("new bitmap reservations=%d, want 64", used)
	}
	if next := allocate(t, p, 1, 100, subscriber.AccessL2TP); seen[next] {
		t.Fatalf("carried address %s allocated again", next)
	}
}

// TestPoolReloadSplitsLiveDelegation reserves every new slot covered by a
// live larger prefix, even when the covered slots are in different new pools.
func TestPoolReloadSplitsLiveDelegation(t *testing.T) {
	p := &poolPlugin{}
	tx := p.configTransaction()
	if err := tx.Configure(reloadPoolSection(t, map[string]any{
		"ipv6-pd": reloadIPv6("2001:db8:100::/60", 60),
	})); err != nil {
		t.Fatal(err)
	}
	if got := p.handlePrefix(l2tp.PrefixRequest{SubscriberID: subscriber.SessionID(subscriber.AccessL2TP, 1, 1), TunnelID: 1, SessionID: 1}); !got.OK {
		t.Fatal(got.Reason)
	}
	if err := tx.Verify(reloadPoolSection(t, map[string]any{
		"ipv6-pd":         reloadIPv6("2001:db8:100::/61", 64),
		"named-ipv6-pool": map[string]any{"other": reloadIPv6("2001:db8:100:8::/61", 64)},
	})); err != nil {
		t.Fatal(err)
	}
	if err := tx.Apply(); err != nil {
		t.Fatal(err)
	}
	for name, pool := range map[string]*ipv6PrefixPool{"default": p.v6pool, "other": p.v6namedPools["other"]} {
		if _, used, free := pool.stats(); used != 8 || free != 0 {
			t.Fatalf("%s split reservations=%d free=%d, want 8/0", name, used, free)
		}
	}
	p.releasePrefixBySubscriberID(subscriber.SessionID(subscriber.AccessL2TP, 1, 1))
	for name, pool := range map[string]*ipv6PrefixPool{"default": p.v6pool, "other": p.v6namedPools["other"]} {
		if _, used, free := pool.stats(); used != 0 || free != 8 {
			t.Fatalf("%s split release=%d free=%d, want 0/8", name, used, free)
		}
	}
}

// TestPoolReloadMergesLiveDelegations keeps a coarser slot reserved until the
// last of two smaller live delegations ends.
func TestPoolReloadMergesLiveDelegations(t *testing.T) {
	p := &poolPlugin{}
	tx := p.configTransaction()
	if err := tx.Configure(reloadPoolSection(t, map[string]any{
		"ipv6-pd": reloadIPv6("2001:db8:100::/60", 64),
	})); err != nil {
		t.Fatal(err)
	}
	for session := uint16(1); session <= 2; session++ {
		if got := p.handlePrefix(l2tp.PrefixRequest{SubscriberID: subscriber.SessionID(subscriber.AccessL2TP, 1, session), TunnelID: 1, SessionID: session}); !got.OK {
			t.Fatal(got.Reason)
		}
	}
	if err := tx.Verify(reloadPoolSection(t, map[string]any{
		"ipv6-pd": reloadIPv6("2001:db8:100::/60", 60),
	})); err != nil {
		t.Fatal(err)
	}
	if err := tx.Apply(); err != nil {
		t.Fatal(err)
	}
	p.releasePrefixBySubscriberID(subscriber.SessionID(subscriber.AccessL2TP, 1, 1))
	if got := p.handlePrefix(l2tp.PrefixRequest{SubscriberID: subscriber.SessionID(subscriber.AccessL2TP, 1, 3), TunnelID: 1, SessionID: 3}); got.OK {
		t.Fatalf("coarse slot reused while second delegation lives: %+v", got)
	}
	p.releasePrefixBySubscriberID(subscriber.SessionID(subscriber.AccessL2TP, 1, 2))
	if got := p.handlePrefix(l2tp.PrefixRequest{SubscriberID: subscriber.SessionID(subscriber.AccessL2TP, 1, 3), TunnelID: 1, SessionID: 3}); !got.OK {
		t.Fatalf("coarse slot did not return after final release: %s", got.Reason)
	}
}

// TestPoolUndeliveredAnswerReleasesExactAllocation proves a failed answer
// returns its allocation while a delayed duplicate cannot release a successor.
func TestPoolUndeliveredAnswerReleasesExactAllocation(t *testing.T) {
	bus := newReleaseBus()
	p := newSingleAddressPlugin(t, bus)
	req := ppp.EventIPRequest{SubscriberID: subscriber.SessionID(subscriber.AccessL2TP, uint64(1), 1), TunnelID: 1, SessionID: 1, Family: ppp.AddressFamilyIPv4}
	first := p.handle(req)
	if !first.Accept || first.AllocationID == 0 {
		t.Fatalf("allocation has no ownership token: %+v", first)
	}
	failed := &subevents.PoolAnswerUndeliveredPayload{SubscriberID: subscriber.SessionID(subscriber.AccessL2TP, uint64(1), 1), TunnelID: 1, SessionID: 1, AllocationID: first.AllocationID}
	if _, err := subevents.PoolAnswerUndelivered.Emit(bus, failed); err != nil {
		t.Fatal(err)
	}
	if _, used, _ := p.pool.stats(); used != 0 {
		t.Fatalf("undelivered allocation remains reserved: %d", used)
	}
	second := p.handle(req)
	if !second.Accept || second.AllocationID == first.AllocationID {
		t.Fatalf("successor did not get new ownership: %+v", second)
	}
	if _, err := subevents.PoolAnswerUndelivered.Emit(bus, failed); err != nil {
		t.Fatal(err)
	}
	if _, used, _ := p.pool.stats(); used != 1 {
		t.Fatalf("stale answer freed successor reservation: %d", used)
	}
}

// TestPoolRepeatedRequestOwnsNoNewAllocation proves an accepted duplicate
// request neither leaks an address nor hands cleanup ownership to its answer.
func TestPoolRepeatedRequestOwnsNoNewAllocation(t *testing.T) {
	p := &poolPlugin{pool: newIPv4Pool(netip.MustParseAddr("10.0.0.1"),
		netip.MustParseAddr("10.0.0.10"), netip.MustParseAddr("10.0.0.11"), netip.Addr{}, netip.Addr{})}
	req := ppp.EventIPRequest{SubscriberID: subscriber.SessionID(subscriber.AccessL2TP, uint64(1), 1), TunnelID: 1, SessionID: 1, Family: ppp.AddressFamilyIPv4}
	first := p.handle(req)
	second := p.handle(req)
	if !second.Accept || second.Peer != first.Peer {
		t.Fatalf("repeat replaced held address: first=%+v second=%+v", first, second)
	}
	if second.AllocationID != 0 {
		t.Fatalf("repeat owns original answer allocation: %d", second.AllocationID)
	}
	if _, used, _ := p.pool.stats(); used != 1 {
		t.Fatalf("repeat allocated %d addresses, want 1", used)
	}
}

// TestPoolReloadReportsDuplicateOrphanOwners proves a legacy conflict is
// diagnosed from session ownership even after its address leaves every range.
func TestPoolReloadReportsDuplicateOrphanOwners(t *testing.T) {
	p := &poolPlugin{}
	tx := p.configTransaction()
	if err := tx.Configure(reloadPoolSection(t, map[string]any{"ipv4": reloadIPv4("10.0.0.10", "10.0.0.10")})); err != nil {
		t.Fatal(err)
	}
	address := netip.MustParseAddr("203.0.113.10")
	p.sessionAddrs.Store(subscriber.SessionID(subscriber.AccessL2TP, uint64(1), 10), sessionAddr{tunnelID: 1, sessionID: 10, addr: address})
	p.sessionAddrs.Store(subscriber.SessionID(subscriber.AccessL2TP, uint64(2), 11), sessionAddr{tunnelID: 2, sessionID: 11, addr: address})
	defer reloadSessionDown(p, 1, 10)
	defer reloadSessionDown(p, 2, 11)
	var output bytes.Buffer
	previous := logger()
	setLogger(slog.New(slog.NewTextHandler(&output, &slog.HandlerOptions{Level: slog.LevelWarn})))
	t.Cleanup(func() { setLogger(previous) })
	if err := tx.Verify(reloadPoolSection(t, map[string]any{"ipv4": reloadIPv4("10.0.0.20", "10.0.0.20")})); err != nil {
		t.Fatal(err)
	}
	if err := tx.Apply(); err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{"Two live subscribers", "address=203.0.113.10", "session=10", "session=11"} {
		if !strings.Contains(output.String(), fragment) {
			t.Errorf("conflict warning lacks %q: %s", fragment, output.String())
		}
	}
	if count := p.duplicateAddresses[address]; count != 2 {
		t.Fatalf("duplicate orphan owners=%d, want 2", count)
	}
}

// TestPoolReloadDuplicateReservationNeedsFinalRelease proves the first of two
// legacy owners cannot expose its shared address to a third subscriber.
func TestPoolReloadDuplicateReservationNeedsFinalRelease(t *testing.T) {
	p := &poolPlugin{}
	tx := p.configTransaction()
	if err := tx.Configure(reloadPoolSection(t, map[string]any{"ipv4": reloadIPv4("10.0.0.10", "10.0.0.10")})); err != nil {
		t.Fatal(err)
	}
	address := allocate(t, p, 1, 1, subscriber.AccessL2TP)
	p.sessionAddrs.Store(subscriber.SessionID(subscriber.AccessL2TP, uint64(1), 2), sessionAddr{tunnelID: 1, sessionID: 2, addr: address})
	if err := tx.Verify(reloadPoolSection(t, map[string]any{"ipv4": reloadIPv4("10.0.0.10", "10.0.0.11")})); err != nil {
		t.Fatal(err)
	}
	if err := tx.Apply(); err != nil {
		t.Fatal(err)
	}
	reloadSessionDown(p, 1, 1)
	if got := allocate(t, p, 1, 3, subscriber.AccessL2TP); got == address {
		t.Fatalf("shared address %s reused after only one release", address)
	}
	reloadSessionDown(p, 1, 2)
	if got := allocate(t, p, 1, 4, subscriber.AccessL2TP); got != address {
		t.Fatalf("shared address did not return after final release: %s", got)
	}
}
