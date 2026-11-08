package system

import (
	"errors"
	"net/netip"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/clock"
	"github.com/ze-software/ze/internal/core/dnssource"
)

// manualClock is a clock.Clock whose time moves only when a test advances
// it, and whose one AfterFunc timer fires inside advance. The registry arms
// a single expiry timer, so one slot is all it needs.
type manualClock struct {
	mu      sync.Mutex
	now     time.Time
	due     time.Time
	fn      func()
	armed   bool
	created int
}

func (c *manualClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *manualClock) Sleep(time.Duration) { panic("BUG: manualClock.Sleep not used by the registry") }
func (c *manualClock) After(time.Duration) <-chan time.Time {
	panic("BUG: manualClock.After not used by the registry")
}
func (c *manualClock) NewTimer(time.Duration) clock.Timer {
	panic("BUG: manualClock.NewTimer not used by the registry")
}
func (c *manualClock) NewTicker(time.Duration) clock.Ticker {
	panic("BUG: manualClock.NewTicker not used by the registry")
}
func (c *manualClock) AfterFunc(d time.Duration, f func()) clock.Timer {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.created++
	c.fn = f
	c.due = c.now.Add(d)
	c.armed = true
	return manualTimer{c}
}

// advance moves the time by d and fires the timer when it is due. The
// callback runs without c.mu held, as a real timer's does.
func (c *manualClock) advance(d time.Duration) {
	c.mu.Lock()
	c.now = c.now.Add(d)
	fire := c.armed && !c.now.Before(c.due)
	if fire {
		c.armed = false
	}
	fn := c.fn
	c.mu.Unlock()
	if fire {
		fn()
	}
}

type manualTimer struct{ c *manualClock }

func (t manualTimer) Stop() bool {
	t.c.mu.Lock()
	defer t.c.mu.Unlock()
	was := t.c.armed
	t.c.armed = false
	return was
}

func (t manualTimer) Reset(d time.Duration) bool {
	t.c.mu.Lock()
	defer t.c.mu.Unlock()
	was := t.c.armed
	t.c.due = t.c.now.Add(d)
	t.c.armed = true
	return was
}

func (t manualTimer) C() <-chan time.Time { panic("BUG: manualTimer.C not used by the registry") }

var (
	routerTestDHCPv4 = ResolverOrigin{rank: 100, name: "dhcpv4"}
	routerTestPPP    = ResolverOrigin{rank: 300, name: "ppp"}
)

func routerList(t *testing.T, written dnssource.Written) dnssource.List {
	t.Helper()
	list, err := dnssource.NewWrittenList(routerDNSSourcePath, dnssource.FamilyAll, &written, routerDNSSourceRefusals...)
	if err != nil {
		t.Fatalf("NewWrittenList: %v", err)
	}
	return list
}

func routerServers(t *testing.T, r *Resolvers) []netip.Addr {
	t.Helper()
	return r.Snapshot().Router.Servers
}

func addrList(texts ...string) []netip.Addr {
	out := make([]netip.Addr, len(texts))
	for i, text := range texts {
		out[i] = netip.MustParseAddr(text)
	}
	return out
}

// TestRecordExpiryWithdraws: a lease whose validity ends leaves the router's
// selection at that instant with no producer call, the next class in the
// list takes over, the watcher (Ze's internal resolver) is told, and the
// record leaves the snapshot. One timer serves every lease.
//
// VALIDATES: AC-3 (spec-home-cpe-23), the expiry sweeper.
// PREVENTS: a stale ISP resolver staying in resolv.conf and Ze's lookups
// after its lease ran out because its producer stopped without withdrawing.
func TestRecordExpiryWithdraws(t *testing.T) {
	clk := &manualClock{now: time.Unix(1_000_000, 0)}
	r := &Resolvers{clock: clk}
	list := routerList(t, dnssource.Written{
		Sources: []dnssource.Class{dnssource.ClassReceived, dnssource.ClassConfigured},
		Mode:    dnssource.ModeFirstUsable,
	})
	if err := r.Configure("", []string{"198.51.100.53"}, list); err != nil {
		t.Fatalf("Configure: %v", err)
	}
	var seen [][]netip.Addr
	stop := r.Watch(func(snapshot *dnssource.Snapshot) { seen = append(seen, snapshot.Router.Servers) })
	defer stop()

	ends := clk.Now().Add(10 * time.Second)
	if err := r.SetLease(routerTestDHCPv4.Source("eth0", "0"), []string{"192.0.2.53"}, ValidFor(10*time.Second)); err != nil {
		t.Fatalf("SetLease: %v", err)
	}
	if err := r.SetLease(routerTestPPP.Source("wan0", ""), []string{"192.0.2.54"}, ValidFor(10*time.Second+time.Minute)); err != nil {
		t.Fatalf("SetLease: %v", err)
	}
	if got, want := routerServers(t, r), addrList("192.0.2.53", "192.0.2.54"); !slices.Equal(got, want) {
		t.Fatalf("before expiry: servers = %v, want %v", got, want)
	}
	if got := r.Snapshot().Records[0].Expires; !got.Equal(ends) {
		t.Fatalf("record expires = %v, want %v", got, ends)
	}

	clk.advance(9 * time.Second)
	if got, want := routerServers(t, r), addrList("192.0.2.53", "192.0.2.54"); !slices.Equal(got, want) {
		t.Fatalf("one second before expiry: servers = %v, want %v", got, want)
	}
	clk.advance(time.Second)
	if got, want := routerServers(t, r), addrList("192.0.2.54"); !slices.Equal(got, want) {
		t.Fatalf("at the first expiry: servers = %v, want %v", got, want)
	}
	clk.advance(time.Minute)
	if got, want := routerServers(t, r), addrList("198.51.100.53"); !slices.Equal(got, want) {
		t.Fatalf("after both expired: servers = %v, want %v (configured, the next class)", got, want)
	}
	if records := r.Snapshot().Records; len(records) != 0 {
		t.Fatalf("expired records still published: %+v", records)
	}
	if last := seen[len(seen)-1]; !slices.Equal(last, addrList("198.51.100.53")) {
		t.Fatalf("watcher last saw %v, want the configured server", last)
	}
	if clk.created != 1 {
		t.Fatalf("expiry timers created = %d, want 1 re-armed timer", clk.created)
	}

	// A validity that already ended withdraws at once.
	if err := r.SetLease(routerTestDHCPv4.Source("eth0", "0"), []string{"192.0.2.53"}, ValidFor(0)); err != nil {
		t.Fatalf("SetLease: %v", err)
	}
	if got, want := routerServers(t, r), addrList("198.51.100.53"); !slices.Equal(got, want) {
		t.Fatalf("ended validity: servers = %v, want %v", got, want)
	}
}

// TestLeaseValidityCountsOnTheRegistryClock: a lease's lifetime starts when
// the registry records it, on the registry's clock, whatever the producer's
// clock reads. Method: a registry whose clock is far from the producer's
// (a DHCP client on a test clock, a host clock stepped by NTP) takes a 30s
// lease, and the record must end 30s after the registry's receipt.
//
// VALIDATES: the cross-clock contract of SetLease (spec-home-cpe-23).
// PREVENTS: the regression where a producer handed an instant read on its
// own clock, which the registry compared with its own: a DHCPv6 lease from
// a client whose clock lagged was dropped at once, so its server never
// reached resolv.conf (TestRFC9915Section182105RenewWithoutDNSStopsUsingIt).
func TestLeaseValidityCountsOnTheRegistryClock(t *testing.T) {
	clk := &manualClock{now: time.Unix(1_000_000, 0)}
	r := &Resolvers{clock: clk}
	if err := r.Configure("", nil, routerDefault); err != nil {
		t.Fatalf("Configure: %v", err)
	}
	received := clk.Now()
	if err := r.SetLease(routerTestDHCPv4.Source("eth0", "0"), []string{"192.0.2.53"}, ValidFor(30*time.Second)); err != nil {
		t.Fatalf("SetLease: %v", err)
	}
	records := r.Snapshot().Records
	if len(records) != 1 {
		t.Fatalf("records = %+v, want the one lease", records)
	}
	if got := records[0].Received; !got.Equal(received) {
		t.Fatalf("record received = %v, want the registry's %v", got, received)
	}
	if got, want := records[0].Expires, received.Add(30*time.Second); !got.Equal(want) {
		t.Fatalf("record expires = %v, want %v", got, want)
	}
	clk.advance(29 * time.Second)
	if got, want := routerServers(t, r), addrList("192.0.2.53"); !slices.Equal(got, want) {
		t.Fatalf("one second before the lifetime ends: servers = %v, want %v", got, want)
	}
	clk.advance(time.Second)
	if got := routerServers(t, r); len(got) != 0 {
		t.Fatalf("at the end of the lifetime: servers = %v, want none", got)
	}
	if err := r.SetLease(routerTestDHCPv4.Source("eth0", "0"), []string{"192.0.2.53"}, ValidFor(-time.Second)); err != nil {
		t.Fatalf("SetLease: %v", err)
	}
	if got := routerServers(t, r); len(got) != 0 {
		t.Fatalf("negative lifetime: servers = %v, want none", got)
	}
}

// TestRouterSourceOrderReceivedFirst and TestRouterSourceOrderConfiguredFirst:
// under first-usable the first class of the written list that holds a
// usable server is the router's answer alone, whichever order is written.
//
// VALIDATES: AC-4 (spec-home-cpe-23) and its mirror, user story 2.
// PREVENTS: the written order being ignored in favour of the default's.
func TestRouterSourceOrderReceivedFirst(t *testing.T) {
	r := &Resolvers{}
	list := routerList(t, dnssource.Written{
		Sources: []dnssource.Class{dnssource.ClassReceived, dnssource.ClassConfigured},
		Mode:    dnssource.ModeFirstUsable,
	})
	if err := r.Configure("", []string{"198.51.100.53"}, list); err != nil {
		t.Fatalf("Configure: %v", err)
	}
	if got, want := routerServers(t, r), addrList("198.51.100.53"); !slices.Equal(got, want) {
		t.Fatalf("no lease: servers = %v, want %v (configured is the backup)", got, want)
	}
	if err := r.SetLease(routerTestDHCPv4.Source("eth0", "0"), []string{"192.0.2.53"}, UntilWithdrawn); err != nil {
		t.Fatalf("SetLease: %v", err)
	}
	if got, want := routerServers(t, r), addrList("192.0.2.53"); !slices.Equal(got, want) {
		t.Fatalf("lease present: servers = %v, want %v (received wins alone)", got, want)
	}
	if got := r.Snapshot().Router; got.Default || got.Mode != "first-usable" || !slices.Equal(got.Used, []dnssource.Class{dnssource.ClassReceived}) {
		t.Fatalf("selection = %+v, want written first-usable, used [received]", got)
	}
}

func TestRouterSourceOrderConfiguredFirst(t *testing.T) {
	r := &Resolvers{}
	list := routerList(t, dnssource.Written{
		Sources: []dnssource.Class{dnssource.ClassConfigured, dnssource.ClassReceived},
		Mode:    dnssource.ModeFirstUsable,
	})
	if err := r.Configure("", []string{"198.51.100.53"}, list); err != nil {
		t.Fatalf("Configure: %v", err)
	}
	if err := r.SetLease(routerTestDHCPv4.Source("eth0", "0"), []string{"192.0.2.53"}, UntilWithdrawn); err != nil {
		t.Fatalf("SetLease: %v", err)
	}
	if got, want := routerServers(t, r), addrList("198.51.100.53"); !slices.Equal(got, want) {
		t.Fatalf("servers = %v, want %v (configured wins, received is the backup)", got, want)
	}
}

// TestRouterSourceMerge: under merge the router lists every class in the
// written order, each server once, so the internal resolver tries the
// configured backup when the received server does not answer.
//
// VALIDATES: AC-17's selection and order (spec-home-cpe-23).
// PREVENTS: merge collapsing to first-usable, or listing in default order.
func TestRouterSourceMerge(t *testing.T) {
	r := &Resolvers{}
	list := routerList(t, dnssource.Written{
		Sources: []dnssource.Class{dnssource.ClassReceived, dnssource.ClassConfigured},
		Mode:    dnssource.ModeMerge,
	})
	if err := r.Configure("", []string{"198.51.100.53", "192.0.2.53"}, list); err != nil {
		t.Fatalf("Configure: %v", err)
	}
	if err := r.SetLease(routerTestDHCPv4.Source("eth0", "0"), []string{"192.0.2.53"}, UntilWithdrawn); err != nil {
		t.Fatalf("SetLease: %v", err)
	}
	if got, want := r.Servers(), []string{"192.0.2.53", "198.51.100.53"}; !slices.Equal(got, want) {
		t.Fatalf("servers = %v, want %v (received, then configured, 192.0.2.53 once)", got, want)
	}
}

// TestRouterReceivedInterface: received-interface narrows the router's
// received class to the named interface's leases.
//
// VALIDATES: AC-24's selection (spec-home-cpe-23, D-11).
// PREVENTS: a lease on an unnamed interface reaching resolv.conf.
func TestRouterReceivedInterface(t *testing.T) {
	r := &Resolvers{}
	list := routerList(t, dnssource.Written{
		Sources:            []dnssource.Class{dnssource.ClassReceived},
		ReceivedInterfaces: []string{"eth1"},
	})
	if err := r.Configure("", nil, list); err != nil {
		t.Fatalf("Configure: %v", err)
	}
	if err := r.SetLease(routerTestDHCPv4.Source("eth0", "0"), []string{"192.0.2.53"}, UntilWithdrawn); err != nil {
		t.Fatalf("SetLease: %v", err)
	}
	if err := r.SetLease(routerTestDHCPv4.Source("eth1", "0"), []string{"198.51.100.53"}, UntilWithdrawn); err != nil {
		t.Fatalf("SetLease: %v", err)
	}
	if got, want := r.Servers(), []string{"198.51.100.53"}; !slices.Equal(got, want) {
		t.Fatalf("servers = %v, want %v (eth1 alone)", got, want)
	}
	if got := r.Snapshot().Router.ReceivedInterfaces; !slices.Equal(got, []string{"eth1"}) {
		t.Fatalf("published received-interfaces = %v, want [eth1]", got)
	}
}

// TestDefaultListsReproduceToday: with no dns-source written, the router
// selects what Ze selected before the list existed: the name-servers alone
// when any are set, otherwise every lease by origin rank, and a reload that
// drops the name-servers hands the router back to the leases.
//
// VALIDATES: AC-5's router half and AC-2's list change through Configure.
// PREVENTS: the default changing an existing router's resolv.conf.
func TestDefaultListsReproduceToday(t *testing.T) {
	r := &Resolvers{}
	if err := r.Configure("", []string{"9.9.9.9"}, DefaultRouterDNSSource()); err != nil {
		t.Fatalf("Configure: %v", err)
	}
	if err := r.SetLease(routerTestPPP.Source("wan0", ""), []string{"192.0.2.54"}, UntilWithdrawn); err != nil {
		t.Fatalf("SetLease: %v", err)
	}
	if err := r.SetLease(routerTestDHCPv4.Source("eth0", "0"), []string{"192.0.2.53"}, UntilWithdrawn); err != nil {
		t.Fatalf("SetLease: %v", err)
	}
	if got, want := r.Servers(), []string{"9.9.9.9"}; !slices.Equal(got, want) {
		t.Fatalf("name-server set: servers = %v, want %v", got, want)
	}
	if !r.Snapshot().Router.Default {
		t.Fatal("selection not marked as the built-in default")
	}
	if err := r.Configure("", nil, DefaultRouterDNSSource()); err != nil {
		t.Fatalf("Configure: %v", err)
	}
	if got, want := r.Servers(), []string{"192.0.2.53", "192.0.2.54"}; !slices.Equal(got, want) {
		t.Fatalf("name-server removed: servers = %v, want %v (dhcpv4 before ppp)", got, want)
	}
}

// TestConfigureRefusesZeroList: a Configure with a list nobody built changes
// nothing and says so, rather than selecting with a guessed list.
//
// VALIDATES: the zero-List guard on Configure.
// PREVENTS: a refused system dns-source silently becoming the default.
func TestConfigureRefusesZeroList(t *testing.T) {
	r := &Resolvers{}
	if err := r.Configure("", []string{"9.9.9.9"}, DefaultRouterDNSSource()); err != nil {
		t.Fatalf("Configure: %v", err)
	}
	if err := r.Configure("", []string{"192.0.2.1"}, dnssource.List{}); !errors.Is(err, errNoRouterList) {
		t.Fatalf("zero list: err = %v, want errNoRouterList", err)
	}
	if got, want := r.Servers(), []string{"9.9.9.9"}; !slices.Equal(got, want) {
		t.Fatalf("after the refusal: servers = %v, want %v (unchanged)", got, want)
	}
}

// dnsSourceTree builds a config tree whose system dns-source holds sources,
// mode (empty: absent) and received-interface.
func dnsSourceTree(sources []string, mode string, interfaces []string) *config.Tree {
	container := config.NewTree()
	if len(sources) > 0 {
		container.SetSlice("source", sources)
	}
	if mode != "" {
		container.Set("mode", mode)
	}
	if len(interfaces) > 0 {
		container.SetSlice("received-interface", interfaces)
	}
	sys := config.NewTree()
	sys.SetContainer("dns-source", container)
	tree := config.NewTree()
	tree.SetContainer("system", sys)
	return tree
}

// TestModeRequiredEachConsumer (router): the system dns-source refusals run
// through the registered tree validator, which ze config validate, a commit,
// the boot load and a reload all call (config.ValidateCustomSections), and
// each carries the what, why and fix of the spec. The accepted forms pass
// and reach SystemConfig.
//
// VALIDATES: AC-16 (router), AC-24's refusal and AC-11's router half
// through the validation entry point (spec-home-cpe-23).
// PREVENTS: a list Ze cannot evaluate passing validate and failing, or
// silently selecting nothing, at boot.
func TestModeRequiredEachConsumer(t *testing.T) {
	refused := []struct {
		name string
		tree *config.Tree
		want []string
	}{
		{"two sources, no mode", dnsSourceTree([]string{"configured", "received"}, "", nil), []string{
			"system dns-source names 2 sources (configured, received) and no mode",
			"Ze cannot tell whether a later source is a backup or an addition",
			"set system dns-source mode first-usable (a later source is used only when the earlier ones have no usable server), or set system dns-source mode merge (every source, in list order)",
		}},
		{"received-interface without received", dnsSourceTree([]string{"configured"}, "", []string{"eth0"}), []string{
			"system dns-source names received-interface eth0 but its source list does not name received",
			"received-interface only narrows the received source, so without it the filter selects nothing",
			"set system dns-source source received, or delete system dns-source received-interface eth0",
		}},
		{"self", dnsSourceTree([]string{"self"}, "", nil), []string{
			"system dns-source names self",
			"self is Ze's own LAN resolver, and Ze cannot be its own upstream: its lookups would loop back to itself",
			"delete system dns-source source self, and name the upstream servers with set system name-server <address> or take them from the network with set system dns-source source received",
		}},
		{"radius", dnsSourceTree([]string{"radius"}, "", nil), []string{"system dns-source names radius"}},
	}
	for _, tc := range refused {
		_, err := config.ValidateCustomSections(tc.tree)
		if !errors.Is(err, config.ErrCustomValidation) {
			t.Fatalf("%s: err = %v, want a custom validation refusal", tc.name, err)
		}
		for _, want := range tc.want {
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("%s: error %q lacks %q", tc.name, err.Error(), want)
			}
		}
	}

	accepted := []struct {
		name    string
		tree    *config.Tree
		sources []dnssource.Class
	}{
		{"one source, no mode", dnsSourceTree([]string{"received"}, "", []string{"eth1"}), []dnssource.Class{dnssource.ClassReceived}},
		{"two sources, merge", dnsSourceTree([]string{"received", "configured"}, "merge", nil), []dnssource.Class{dnssource.ClassReceived, dnssource.ClassConfigured}},
	}
	for _, tc := range accepted {
		if err := validateRouterDNSSourceTree(tc.tree); err != nil {
			t.Fatalf("%s: refused: %v", tc.name, err)
		}
		list := ExtractSystemConfig(tc.tree).DNSSource
		if list.Stated() || !slices.Equal(list.Sources(), tc.sources) {
			t.Fatalf("%s: extracted %v (stated %v), want the written %v", tc.name, list.Sources(), list.Stated(), tc.sources)
		}
	}
	if list := ExtractSystemConfig(config.NewTree()).DNSSource; !list.Stated() {
		t.Fatal("no system container: extracted list is not the built-in default")
	}
}
