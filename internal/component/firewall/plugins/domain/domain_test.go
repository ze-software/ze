package domain

import (
	"context"
	"net/netip"
	"path/filepath"
	"sync"
	"testing"
	"time"

	mdns "github.com/miekg/dns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/component/resolve/dns"
)

// answer is one canned DNS reply for the stub resolver.
type answer struct {
	records []string
	ttl     uint32
	status  string
	err     error
}

// stubResolver answers from a script and counts what it was asked, so a test
// can assert both what the plugin did with an answer and that it asked at all.
type stubResolver struct {
	byName map[string]answer
	calls  []string
}

func (s *stubResolver) resolve(_ context.Context, name string, qtype uint16) ([]string, uint32, string, error) {
	key := stubKey(name, qtype)
	s.calls = append(s.calls, key)
	a, ok := s.byName[key]
	if !ok {
		// A name the script does not mention exists and holds no record of
		// this type: NOERROR with nothing, which is what a real server answers
		// for an IPv4-only name asked for AAAA.
		return nil, 300, dns.StatusSuccess.String(), nil
	}
	return a.records, a.ttl, a.status, a.err
}

func stubKey(name string, qtype uint16) string {
	if qtype == mdns.TypeAAAA {
		return name + "/AAAA"
	}
	return name + "/A"
}

// newTestPlugin builds a plugin over a temp-directory cache and change log and
// a stub resolver, with no SDK connection: every path this file exercises runs
// below the plugin's engine callbacks.
func newTestPlugin(t *testing.T, cfg *domainConfig, script map[string]answer) (*domainPlugin, *stubResolver) {
	t.Helper()
	dir := t.TempDir()
	stub := &stubResolver{byName: script}

	plug := &domainPlugin{
		resolve:   stub.resolve,
		cache:     newStore(filepath.Join(dir, "database.zefs")),
		changeLog: newChangeLog(filepath.Join(dir, changeLogFileName)),
		sched:     newSchedule(),
		wake:      make(chan struct{}, 1),
		stopCh:    make(chan struct{}),
		done:      make(chan struct{}),
		config:    cfg,
	}
	require.NoError(t, plug.cache.open(cfg.groups))
	require.NoError(t, plug.changeLog.open())
	t.Cleanup(plug.cache.close)
	return plug, stub
}

func oneGroupConfig() *domainConfig {
	return &domainConfig{
		groups: []group{{Name: "cdn", Names: []string{"a.invalid"}, TTLFloor: 60}},
		refs:   []termRef{{Group: "cdn", TableName: "ze_filter"}},
	}
}

func v4Key() nameKey {
	return nameKey{group: "cdn", name: "a.invalid", family: familyV4}
}

// TestRefreshChangedAddressesReprogramsAndLogsOnce proves a name that moved is
// recorded exactly once, with the addresses it held before and the ones it
// holds now.
//
// VALIDATES: AC-3 -- the cache is replaced and exactly ONE record is appended
// naming the old set, the new set and the time.
// PREVENTS: a second record for the same move, which would make the log say a
// name moved twice.
func TestRefreshChangedAddressesReprogramsAndLogsOnce(t *testing.T) {
	plug, _ := newTestPlugin(t, oneGroupConfig(), map[string]answer{
		stubKey("a.invalid", mdns.TypeA): {records: []string{"192.0.2.1"}, ttl: 300, status: "NOERROR"},
	})

	changed, err := plug.resolveAndRecord(v4Key())
	require.NoError(t, err)
	assert.True(t, changed, "a name resolving for the first time is a change")

	entry, ok := plug.cache.get(nameKey{group: "cdn", name: "a.invalid", family: familyV4})
	require.True(t, ok, "the answer must reach the cache")
	assert.Equal(t, []string{"192.0.2.1"}, entry.Addresses)

	records, err := plug.changeLog.records(0)
	require.NoError(t, err)
	require.Len(t, records, 1, "exactly one record for one change")
	assert.Equal(t, "cdn", records[0].Group)
	assert.Equal(t, "a.invalid", records[0].Name)
	assert.Equal(t, "ipv4", records[0].Family)
	assert.Empty(t, records[0].Old, "nothing was cached before")
	assert.Equal(t, []string{"192.0.2.1"}, records[0].New)
	assert.False(t, records[0].Time.IsZero(), "the record must say when the name moved")
}

// TestRefreshSameAddressesSkipsReprogramAndLog proves a re-resolution that
// returned the same addresses writes nothing at all.
//
// VALIDATES: AC-2 -- no set is reprogrammed, no cache write happens, and no
// change-log record is written.
// PREVENTS: a name with a 60-second TTL rewriting the kernel's sets 1440 times
// a day and filling the change log with a record of nothing happening.
func TestRefreshSameAddressesSkipsReprogramAndLog(t *testing.T) {
	plug, _ := newTestPlugin(t, oneGroupConfig(), map[string]answer{
		stubKey("a.invalid", mdns.TypeA): {records: []string{"192.0.2.1"}, ttl: 300, status: "NOERROR"},
	})

	changed, err := plug.resolveAndRecord(v4Key())
	require.NoError(t, err)
	require.True(t, changed)

	changed, err = plug.resolveAndRecord(v4Key())
	require.NoError(t, err)
	assert.False(t, changed, "the same addresses are not a change")

	records, err := plug.changeLog.records(0)
	require.NoError(t, err)
	assert.Len(t, records, 1, "the confirming refresh must add no record")
}

// TestRefreshSameAddressesInADifferentOrderIsNotAChange proves a server that
// rotates its answer for load balancing does not read as a move. This is the
// common case for a CDN name and it would otherwise reprogram the kernel and
// write a record on every single refresh.
func TestRefreshSameAddressesInADifferentOrderIsNotAChange(t *testing.T) {
	script := map[string]answer{
		stubKey("a.invalid", mdns.TypeA): {records: []string{"192.0.2.1", "192.0.2.2"}, ttl: 300, status: "NOERROR"},
	}
	plug, stub := newTestPlugin(t, oneGroupConfig(), script)

	changed, err := plug.resolveAndRecord(v4Key())
	require.NoError(t, err)
	require.True(t, changed)

	stub.byName[stubKey("a.invalid", mdns.TypeA)] = answer{
		records: []string{"192.0.2.2", "192.0.2.1"}, ttl: 300, status: "NOERROR",
	}
	changed, err = plug.resolveAndRecord(v4Key())
	require.NoError(t, err)
	assert.False(t, changed, "a rotated answer holds the same addresses")
}

// TestRefreshTransportErrorKeepsLastGood proves a query that never reached a
// server leaves the addresses alone.
//
// VALIDATES: AC-4 -- the set keeps the last-good addresses and the change log
// gets no record.
// PREVENTS: an unreachable resolver emptying a live firewall set, which for a
// permit rule blocks every source the rule was written to allow.
func TestRefreshTransportErrorKeepsLastGood(t *testing.T) {
	script := map[string]answer{
		stubKey("a.invalid", mdns.TypeA): {records: []string{"192.0.2.1"}, ttl: 300, status: "NOERROR"},
	}
	plug, stub := newTestPlugin(t, oneGroupConfig(), script)

	_, err := plug.resolveAndRecord(v4Key())
	require.NoError(t, err)

	stub.byName[stubKey("a.invalid", mdns.TypeA)] = answer{err: assert.AnError}
	changed, err := plug.resolveAndRecord(v4Key())
	require.Error(t, err, "a transport failure is reported")
	assert.False(t, changed)

	entry, ok := plug.cache.get(nameKey{group: "cdn", name: "a.invalid", family: familyV4})
	require.True(t, ok)
	assert.Equal(t, []string{"192.0.2.1"}, entry.Addresses, "the last good addresses stay")

	records, err := plug.changeLog.records(0)
	require.NoError(t, err)
	assert.Len(t, records, 1, "a failure is not a change")
}

// TestRefreshErrorStillRearmsTheSchedule proves a failed unit is asked for
// again. A unit left unscheduled after a failure would never be retried, so a
// single blip would freeze that name for the life of the process.
func TestRefreshErrorStillRearmsTheSchedule(t *testing.T) {
	plug, _ := newTestPlugin(t, oneGroupConfig(), map[string]answer{
		stubKey("a.invalid", mdns.TypeA): {err: assert.AnError},
	})

	before := time.Now()
	_, err := plug.resolveAndRecord(v4Key())
	require.Error(t, err)

	key, due, found := plug.sched.nextDue()
	require.True(t, found, "a failed unit must stay scheduled")
	assert.Equal(t, v4Key(), key)
	assert.True(t, due.After(before), "and must be due in the future, not at once")
}

// TestClassifyRcodeNXDOMAINEmptiesSet proves an authoritative absence removes
// the addresses.
//
// VALIDATES: AC-5, the NXDOMAIN branch -- the set is emptied and a record is
// written.
// PREVENTS: a deleted name enforcing its old addresses forever, which for a
// permit rule keeps trusting a name somebody else can now register.
func TestClassifyRcodeNXDOMAINEmptiesSet(t *testing.T) {
	script := map[string]answer{
		stubKey("a.invalid", mdns.TypeA): {records: []string{"192.0.2.1"}, ttl: 300, status: "NOERROR"},
	}
	plug, stub := newTestPlugin(t, oneGroupConfig(), script)

	_, err := plug.resolveAndRecord(v4Key())
	require.NoError(t, err)

	stub.byName[stubKey("a.invalid", mdns.TypeA)] = answer{ttl: 300, status: "NXDOMAIN"}
	changed, err := plug.resolveAndRecord(v4Key())
	require.NoError(t, err, "NXDOMAIN is an answer, not a failure")
	assert.True(t, changed)

	entry, ok := plug.cache.get(nameKey{group: "cdn", name: "a.invalid", family: familyV4})
	require.True(t, ok)
	assert.Empty(t, entry.Addresses, "an authoritative absence empties the set")
	assert.Equal(t, "NXDOMAIN", entry.Status)

	records, err := plug.changeLog.records(0)
	require.NoError(t, err)
	require.Len(t, records, 2)
	assert.Equal(t, []string{"192.0.2.1"}, records[1].Old)
	assert.Empty(t, records[1].New)
	assert.Equal(t, "NXDOMAIN", records[1].Status)
}

// TestClassifyRcodeServfailKeepsLastGood proves a server-side failure does not
// empty the set.
//
// VALIDATES: AC-5, the SERVFAIL branch.
// PREVENTS: a broken upstream resolver, or a broken DNSSEC chain, blackholing
// every rule keyed on a domain group. Before the resolver surfaced the response
// code this was indistinguishable from NXDOMAIN.
func TestClassifyRcodeServfailKeepsLastGood(t *testing.T) {
	assertTransientKeepsLastGood(t, "SERVFAIL")
}

// TestClassifyRcodeRefusedKeepsLastGood proves a refusal does not empty the set.
//
// VALIDATES: AC-5, the REFUSED branch.
func TestClassifyRcodeRefusedKeepsLastGood(t *testing.T) {
	assertTransientKeepsLastGood(t, "REFUSED")
}

func assertTransientKeepsLastGood(t *testing.T, status string) {
	t.Helper()
	script := map[string]answer{
		stubKey("a.invalid", mdns.TypeA): {records: []string{"192.0.2.1"}, ttl: 300, status: "NOERROR"},
	}
	plug, stub := newTestPlugin(t, oneGroupConfig(), script)

	_, err := plug.resolveAndRecord(v4Key())
	require.NoError(t, err)

	stub.byName[stubKey("a.invalid", mdns.TypeA)] = answer{ttl: 300, status: status}
	changed, err := plug.resolveAndRecord(v4Key())
	require.Error(t, err, status+" must be reported, not treated as an empty answer")
	assert.False(t, changed)

	entry, ok := plug.cache.get(nameKey{group: "cdn", name: "a.invalid", family: familyV4})
	require.True(t, ok)
	assert.Equal(t, []string{"192.0.2.1"}, entry.Addresses, status+" says nothing about the name")

	records, err := plug.changeLog.records(0)
	require.NoError(t, err)
	assert.Len(t, records, 1, status+" is not a change")
}

// TestClassifyUnknownStatusKeepsLastGood proves a status spelling the resolver
// does not publish is treated as transient rather than as an absence.
//
// This is the fail-closed direction. A status that parses to
// StatusUnspecified must never reach the emptying branch, because the one
// thing certain about an unrecognized answer is that it does not authorize
// removing a live filter.
func TestClassifyUnknownStatusKeepsLastGood(t *testing.T) {
	assertTransientKeepsLastGood(t, "SOMETHING-NEW")
}

// TestRefreshNOERROREmptyIsAuthoritative proves an IPv4-only name yields an
// empty IPv6 set rather than a failure.
//
// A name with no AAAA record answers NOERROR with no records, every time. Read
// as a failure it would leave the IPv6 set permanently unresolved, and the
// table naming it held back; read as authoritative it is the correct content of
// that set.
func TestRefreshNOERROREmptyIsAuthoritative(t *testing.T) {
	plug, _ := newTestPlugin(t, oneGroupConfig(), map[string]answer{
		stubKey("a.invalid", mdns.TypeA):    {records: []string{"192.0.2.1"}, ttl: 300, status: "NOERROR"},
		stubKey("a.invalid", mdns.TypeAAAA): {ttl: 300, status: "NOERROR"},
	})

	v6 := nameKey{group: "cdn", name: "a.invalid", family: familyV6}
	changed, err := plug.resolveAndRecord(v6)
	require.NoError(t, err, "NOERROR with no records is an answer")
	assert.False(t, changed, "empty then empty is not a change")

	// And it does not block the IPv4 family.
	changed, err = plug.resolveAndRecord(v4Key())
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, []string{"192.0.2.1"}, plug.cache.groupAddresses("cdn", []string{"a.invalid"}, familyV4))
	assert.Empty(t, plug.cache.groupAddresses("cdn", []string{"a.invalid"}, familyV6))
}

// TestRefreshCapsAddressesPerName proves one name cannot insert an unbounded
// number of addresses into a group.
//
// DNS is attacker-influenced input and a permit rule keyed on a name trusts
// whoever controls it, so a hijacked answer carrying ten thousand addresses
// must not become ten thousand permitted sources (R-5).
func TestRefreshCapsAddressesPerName(t *testing.T) {
	records := make([]string, 0, maxAddressesPerName*2)
	for i := range maxAddressesPerName * 2 {
		records = append(records, netAddrForIndex(i))
	}
	plug, _ := newTestPlugin(t, oneGroupConfig(), map[string]answer{
		stubKey("a.invalid", mdns.TypeA): {records: records, ttl: 300, status: "NOERROR"},
	})

	_, err := plug.resolveAndRecord(v4Key())
	require.NoError(t, err)

	entry, ok := plug.cache.get(nameKey{group: "cdn", name: "a.invalid", family: familyV4})
	require.True(t, ok)
	assert.Len(t, entry.Addresses, maxAddressesPerName, "the cap bounds what one name contributes")
}

// TestRefreshRejectsUnparseableRecords proves a record that is not an address
// never reaches the set. It would otherwise be handed to nftables as a
// literal, which is untrusted input reaching a kernel interface.
func TestRefreshRejectsUnparseableRecords(t *testing.T) {
	plug, _ := newTestPlugin(t, oneGroupConfig(), map[string]answer{
		stubKey("a.invalid", mdns.TypeA): {
			records: []string{"192.0.2.1", "not-an-address", "192.0.2.0/24", ""},
			ttl:     300, status: "NOERROR",
		},
	})

	_, err := plug.resolveAndRecord(v4Key())
	require.NoError(t, err)

	entry, ok := plug.cache.get(nameKey{group: "cdn", name: "a.invalid", family: familyV4})
	require.True(t, ok)
	assert.Equal(t, []string{"192.0.2.1"}, entry.Addresses)
}

// TestRefreshRejectsWrongFamilyRecords proves an AAAA answer cannot land in
// the IPv4 set. nftables refuses a set element of the wrong type, so the whole
// table would fail to program.
func TestRefreshRejectsWrongFamilyRecords(t *testing.T) {
	plug, _ := newTestPlugin(t, oneGroupConfig(), map[string]answer{
		stubKey("a.invalid", mdns.TypeA): {records: []string{"2001:db8::1", "192.0.2.1"}, ttl: 300, status: "NOERROR"},
	})

	_, err := plug.resolveAndRecord(v4Key())
	require.NoError(t, err)
	entry, _ := plug.cache.get(nameKey{group: "cdn", name: "a.invalid", family: familyV4})
	assert.Equal(t, []string{"192.0.2.1"}, entry.Addresses)
}

// TestDomainGroupColdStartProgramsFromCache proves a restart with DNS
// unreachable programs the sets from what was learned last time.
//
// VALIDATES: AC-7 -- every domain-group set is programmed from the cached
// addresses before any resolution is attempted, so filtering resumes without
// waiting for DNS.
// PREVENTS: a box rebooting into an unfiltered state whenever its upstream
// resolver is slower to come back than Ze is.
func TestDomainGroupColdStartProgramsFromCache(t *testing.T) {
	dir := t.TempDir()
	cfg := oneGroupConfig()
	zefsPath := filepath.Join(dir, "database.zefs")

	// First run: resolve and persist.
	first := &domainPlugin{
		resolve: (&stubResolver{byName: map[string]answer{
			stubKey("a.invalid", mdns.TypeA): {records: []string{"192.0.2.1"}, ttl: 300, status: "NOERROR"},
		}}).resolve,
		cache:     newStore(zefsPath),
		changeLog: newChangeLog(filepath.Join(dir, changeLogFileName)),
		sched:     newSchedule(),
		wake:      make(chan struct{}, 1),
		stopCh:    make(chan struct{}),
		done:      make(chan struct{}),
		config:    cfg,
	}
	require.NoError(t, first.cache.open(cfg.groups))
	require.NoError(t, first.changeLog.open())
	_, err := first.resolveAndRecord(v4Key())
	require.NoError(t, err)
	first.cache.close()

	// Second run: a resolver that fails every query, standing for DNS being
	// unreachable at boot.
	second := &domainPlugin{
		resolve: func(context.Context, string, uint16) ([]string, uint32, string, error) {
			return nil, 0, "", assert.AnError
		},
		cache:     newStore(zefsPath),
		changeLog: newChangeLog(filepath.Join(dir, changeLogFileName)),
		sched:     newSchedule(),
		wake:      make(chan struct{}, 1),
		stopCh:    make(chan struct{}),
		done:      make(chan struct{}),
		config:    cfg,
	}
	require.NoError(t, second.cache.open(cfg.groups))
	t.Cleanup(second.cache.close)

	// The addresses are there before anything is resolved, and the tables the
	// registry would be handed carry them.
	assert.Equal(t, []string{"192.0.2.1"}, second.cache.groupAddresses("cdn", []string{"a.invalid"}, familyV4))
	tables := buildTables(cfg, func(g group, fam family) []string {
		return second.cache.groupAddresses(g.Name, g.Names, fam)
	})
	require.Len(t, tables, 1)
	require.Len(t, tables[0].Sets, 2, "both families are declared so the v6 twin term has a set")
	assert.Equal(t, "domain_v4_cdn", tables[0].Sets[0].Name)
	require.Len(t, tables[0].Sets[0].Elements, 1)
	assert.Equal(t, "192.0.2.1", tables[0].Sets[0].Elements[0].Value)
}

// TestDomainGroupOnConfigureAfterRespawnPreservesChangeLogAndCache proves a
// respawned plugin keeps both stores.
//
// VALIDATES: AC-11 -- the plugin is respawned, programs its sets from the zefs
// cache, and loses no previously recorded change-log record. Process
// supervision itself is ProcessManager.Respawn and is proven generically; what
// is proven here is that this plugin's own state survives it.
func TestDomainGroupOnConfigureAfterRespawnPreservesChangeLogAndCache(t *testing.T) {
	dir := t.TempDir()
	cfg := oneGroupConfig()
	zefsPath := filepath.Join(dir, "database.zefs")
	logPath := filepath.Join(dir, changeLogFileName)

	build := func(script map[string]answer) *domainPlugin {
		p := &domainPlugin{
			resolve:   (&stubResolver{byName: script}).resolve,
			cache:     newStore(zefsPath),
			changeLog: newChangeLog(logPath),
			sched:     newSchedule(),
			wake:      make(chan struct{}, 1),
			stopCh:    make(chan struct{}),
			done:      make(chan struct{}),
			config:    cfg,
		}
		require.NoError(t, p.cache.open(cfg.groups))
		require.NoError(t, p.changeLog.open())
		return p
	}

	first := build(map[string]answer{
		stubKey("a.invalid", mdns.TypeA): {records: []string{"192.0.2.1"}, ttl: 300, status: "NOERROR"},
	})
	_, err := first.resolveAndRecord(v4Key())
	require.NoError(t, err)
	first.cache.close()

	// The process dies here and the manager respawns it.
	second := build(map[string]answer{
		stubKey("a.invalid", mdns.TypeA): {records: []string{"192.0.2.9"}, ttl: 300, status: "NOERROR"},
	})
	t.Cleanup(second.cache.close)

	assert.Equal(t, []string{"192.0.2.1"}, second.cache.groupAddresses("cdn", []string{"a.invalid"}, familyV4),
		"the cache survives the respawn")

	_, err = second.resolveAndRecord(v4Key())
	require.NoError(t, err)

	records, err := second.changeLog.records(0)
	require.NoError(t, err)
	require.Len(t, records, 2, "the record written before the crash is still there")
	assert.Empty(t, records[0].Old)
	assert.Equal(t, []string{"192.0.2.1"}, records[0].New)
	assert.Equal(t, []string{"192.0.2.1"}, records[1].Old)
	assert.Equal(t, []string{"192.0.2.9"}, records[1].New)
}

// TestDomainGroupVerifyRefusesUncachedName proves a commit naming a group that
// has never resolved is refused, with the group and the fetch command named.
//
// VALIDATES: AC-6 -- the commit is REFUSED at verify. No table is programmed
// and no table is silently held back.
// PREVENTS: the registry's default for this shape, which is to hold the whole
// table back with a WARN. That is right when the alternative is a blackhole and
// wrong here: the operator learns their filter left the kernel from a log line,
// after the traffic it was written for has passed.
func TestDomainGroupVerifyRefusesUncachedName(t *testing.T) {
	plug, _ := newTestPlugin(t, oneGroupConfig(), nil)

	err := plug.verify(oneGroupConfig())
	require.Error(t, err, "a group with no addresses must not commit")
	assert.Contains(t, err.Error(), "cdn", "the message names the group")
	assert.Contains(t, err.Error(), "update firewall domain-group cdn",
		"the message names the command that fixes it")
}

// TestDomainGroupVerifyAcceptsAResolvedGroup proves the refusal is conditional
// on the data rather than unconditional. A verify that refused every commit
// would pass the test above and make the feature unusable.
func TestDomainGroupVerifyAcceptsAResolvedGroup(t *testing.T) {
	plug, _ := newTestPlugin(t, oneGroupConfig(), map[string]answer{
		stubKey("a.invalid", mdns.TypeA): {records: []string{"192.0.2.1"}, ttl: 300, status: "NOERROR"},
	})
	_, err := plug.resolveAndRecord(v4Key())
	require.NoError(t, err)

	assert.NoError(t, plug.verify(oneGroupConfig()), "a group with addresses commits")
}

// TestDomainGroupVerifyIgnoresAnUnreferencedGroup proves a group no rule names
// does not refuse a commit. It enforces nothing, so refusing for it would
// refuse a config that harms nobody.
func TestDomainGroupVerifyIgnoresAnUnreferencedGroup(t *testing.T) {
	cfg := &domainConfig{groups: []group{{Name: "cdn", Names: []string{"a.invalid"}, TTLFloor: 60}}}
	plug, _ := newTestPlugin(t, cfg, nil)
	assert.NoError(t, plug.verify(cfg))
}

// TestDomainGroupVerifyRefusesAGroupWithNoNames proves a group holding no
// domain-names is refused rather than accepted as an empty filter. An empty
// permit set blocks everything, so an operator must not reach it by omission.
func TestDomainGroupVerifyRefusesAGroupWithNoNames(t *testing.T) {
	cfg := &domainConfig{
		groups: []group{{Name: "cdn", TTLFloor: 60}},
		refs:   []termRef{{Group: "cdn", TableName: "ze_filter"}},
	}
	plug, _ := newTestPlugin(t, cfg, nil)
	err := plug.verify(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "holds no domain-names")
}

// TestDomainGroupVerifyPerformsNoNetworkIO proves verify never resolves. The
// verify callback runs while a commit is held open, so a DNS round trip there
// would put every timeout it can hit in the path of an operator pressing
// return.
func TestDomainGroupVerifyPerformsNoNetworkIO(t *testing.T) {
	plug, stub := newTestPlugin(t, oneGroupConfig(), map[string]answer{
		stubKey("a.invalid", mdns.TypeA): {records: []string{"192.0.2.1"}, ttl: 300, status: "NOERROR"},
	})
	_ = plug.verify(oneGroupConfig())
	assert.Empty(t, stub.calls, "verify must not query DNS")
}

// recordingBackend stands in for nftables so the apply path runs end to end in
// a unit test. It records the desired state it was handed, which is what lets
// a test assert that the addresses reached a backend rather than only the
// cache.
type recordingBackend struct {
	mu     sync.Mutex
	tables []firewall.Table
}

func (b *recordingBackend) Apply(desired []firewall.Table) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.tables = append([]firewall.Table(nil), desired...)
	return nil
}

func (b *recordingBackend) ListTables() ([]firewall.Table, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.tables, nil
}

func (b *recordingBackend) GetCounters(string) ([]firewall.ChainCounters, error) { return nil, nil }
func (b *recordingBackend) Close() error                                         { return nil }

func (b *recordingBackend) applied() []firewall.Table {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.tables
}

// sharedBackend is the one recording backend of this test binary. The firewall
// registry is a package-level singleton and RegisterBackend refuses a duplicate
// name, so the factory must hand back the SAME object every time: a helper that
// registered a factory over a fresh object would leave every later test reading
// one nothing ever writes to.
var (
	sharedBackend = &recordingBackend{}
	backendOnce   sync.Once
)

// reset drops what the backend recorded, so each test reads only its own
// applies.
func (b *recordingBackend) reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.tables = nil
}

// useRecordingBackend installs the recording backend as the active one, so
// ApplyAll reaches something rather than failing with "unknown backend". The
// backend is process-wide, so the tests that use it do not run in parallel.
func useRecordingBackend(t *testing.T) *recordingBackend {
	t.Helper()
	var regErr error
	backendOnce.Do(func() {
		regErr = firewall.RegisterBackend("domain-group-test", func() (firewall.Backend, error) {
			return sharedBackend, nil
		})
	})
	require.NoError(t, regErr)
	require.NoError(t, firewall.LoadBackend("domain-group-test"))
	sharedBackend.reset()
	t.Cleanup(func() { _ = firewall.RegisterTables(tableOwner, nil) })
	return sharedBackend
}

// TestRefreshGroupNowAsksBothFamiliesOfEveryName proves the update command
// covers the whole group. It is the only path back from a cold cache, so a
// name it skipped would stay unresolved and keep its table held back.
func TestRefreshGroupNowAsksBothFamiliesOfEveryName(t *testing.T) {
	cfg := &domainConfig{
		groups: []group{{Name: "cdn", Names: []string{"a.invalid", "b.invalid"}, TTLFloor: 60}},
		refs:   []termRef{{Group: "cdn", TableName: "ze_filter"}},
	}
	backend := useRecordingBackend(t)
	plug, stub := newTestPlugin(t, cfg, map[string]answer{
		stubKey("a.invalid", mdns.TypeA):    {records: []string{"192.0.2.1"}, ttl: 300, status: "NOERROR"},
		stubKey("b.invalid", mdns.TypeAAAA): {records: []string{"2001:db8::1"}, ttl: 300, status: "NOERROR"},
	})

	changed, err := plug.refreshGroupNow(cfg.groups[0])
	require.NoError(t, err)
	assert.Equal(t, 2, changed)
	assert.ElementsMatch(t, []string{
		"a.invalid/A", "a.invalid/AAAA", "b.invalid/A", "b.invalid/AAAA",
	}, stub.calls)

	// The addresses reached a backend, which is what AC-1 asserts: the set for
	// the group holds exactly what the names resolved to.
	applied := backend.applied()
	require.NotEmpty(t, applied, "the update command must program the tables, not only the cache")
	var v4, v6 []string
	for i := range applied {
		for j := range applied[i].Sets {
			set := &applied[i].Sets[j]
			for k := range set.Elements {
				if set.Name == "domain_v4_cdn" {
					v4 = append(v4, set.Elements[k].Value)
				}
				if set.Name == "domain_v6_cdn" {
					v6 = append(v6, set.Elements[k].Value)
				}
			}
		}
	}
	assert.Equal(t, []string{"192.0.2.1"}, v4)
	assert.Equal(t, []string{"2001:db8::1"}, v6)
}

// TestRefreshGroupNowKeepsGoingAfterAFailure proves one failing name does not
// stop the others. An operator asking for a group wants every name they can
// get, not a stop at the first.
func TestRefreshGroupNowKeepsGoingAfterAFailure(t *testing.T) {
	cfg := &domainConfig{
		groups: []group{{Name: "cdn", Names: []string{"a.invalid", "b.invalid"}, TTLFloor: 60}},
		refs:   []termRef{{Group: "cdn", TableName: "ze_filter"}},
	}
	useRecordingBackend(t)
	plug, _ := newTestPlugin(t, cfg, map[string]answer{
		stubKey("a.invalid", mdns.TypeA): {err: assert.AnError},
		stubKey("b.invalid", mdns.TypeA): {records: []string{"192.0.2.2"}, ttl: 300, status: "NOERROR"},
	})

	changed, err := plug.refreshGroupNow(cfg.groups[0])
	require.Error(t, err, "the failure is reported")
	assert.Positive(t, changed, "and the names that answered were still recorded")
	assert.Equal(t, []string{"192.0.2.2"}, plug.cache.groupAddresses("cdn", cfg.groups[0].Names, familyV4))
}

// TestRefreshWorkerStopsOnStop proves the worker ends when the plugin does.
// A worker that outlived its plugin would keep resolving against a closed
// cache for the life of the process.
func TestRefreshWorkerStopsOnStop(t *testing.T) {
	plug, _ := newTestPlugin(t, oneGroupConfig(), nil)
	plug.startRefreshWorker()

	done := make(chan struct{})
	go func() {
		plug.stop()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("stop did not return: the refresh worker is still running")
	}
}

// netAddrForIndex builds a distinct IPv4 address for index i, so the cap test
// feeds real addresses rather than strings the parser would reject anyway.
func netAddrForIndex(i int) string {
	return netip.AddrFrom4([4]byte{198, 51, byte(i / 256), byte(i % 256)}).String()
}

// TestDomainGroupConfigureRegistersSet proves a committed config reaches the
// firewall registry through the plugin's own configure path.
//
// VALIDATES: wiring row 1, AC-1 -- a committed config carrying a domain group
// reaches OnConfigure, then firewall.RegisterTables.
// PREVENTS: the feature existing in isolation. Every unit above drives
// resolveAndRecord or buildTables directly; this one starts where a commit
// starts, so a configure that parsed the config and never registered anything
// would fail here and nowhere else.
func TestDomainGroupConfigureRegistersSet(t *testing.T) {
	backend := useRecordingBackend(t)
	dir := t.TempDir()

	cfg := oneGroupConfig()
	plug := &domainPlugin{
		resolve: (&stubResolver{byName: map[string]answer{
			stubKey("a.invalid", mdns.TypeA): {records: []string{"192.0.2.1"}, ttl: 300, status: "NOERROR"},
		}}).resolve,
		cache:     newStore(filepath.Join(dir, "database.zefs")),
		changeLog: newChangeLog(filepath.Join(dir, changeLogFileName)),
		sched:     newSchedule(),
		wake:      make(chan struct{}, 1),
		stopCh:    make(chan struct{}),
		done:      make(chan struct{}),
		config:    cfg,
	}
	require.NoError(t, plug.cache.open(cfg.groups))
	require.NoError(t, plug.changeLog.open())

	// Resolve once, as an operator's `update firewall domain-group` would,
	// then configure: this is the order a real commit takes, because verify
	// refuses a group that has never resolved.
	_, err := plug.resolveAndRecord(v4Key())
	require.NoError(t, err)

	require.NoError(t, plug.configure(cfg))
	t.Cleanup(plug.stop)

	applied := backend.applied()
	require.NotEmpty(t, applied, "configure must register and apply, not only parse")
	found := false
	for i := range applied {
		for j := range applied[i].Sets {
			if applied[i].Sets[j].Name != "domain_v4_cdn" {
				continue
			}
			found = true
			require.Len(t, applied[i].Sets[j].Elements, 1)
			assert.Equal(t, "192.0.2.1", applied[i].Sets[j].Elements[0].Value)
		}
	}
	assert.True(t, found, "the group's set must reach the backend")
}

// TestDomainGroupConfigureStartsNoRefreshWorker proves configure makes no
// engine call.
//
// VALIDATES: the SDK's startup contract. A resolve is an engine call, and
// OnConfigure runs while the engine is waiting for its response
// (Plugin.OnStarted, pkg/plugin/sdk/sdk_callbacks.go), so a worker started here
// resolves a never-cached name at once and the startup coordinator reads that
// request where it expects this plugin's `ready`.
// PREVENTS: the daemon refusing to start with "plugin firewall-domain failed
// during startup at stage Ready: stage 5: expected ready, got
// ze-plugin-engine:resolve-dns", which is what
// test/firewall/firewall-cli-domain-group-show.ci met on its first run.
func TestDomainGroupConfigureStartsNoRefreshWorker(t *testing.T) {
	useRecordingBackend(t)
	cfg := oneGroupConfig()
	plug, stub := newTestPlugin(t, cfg, nil)
	t.Cleanup(plug.stop)

	require.NoError(t, plug.configure(cfg))

	plug.mu.RLock()
	started := plug.workerStarted
	plug.mu.RUnlock()
	assert.False(t, started, "configure must leave the refresh worker unstarted")

	// The cache is cold, so reset leaves every unit due now. A worker started
	// by configure would have resolved within this settle, and the read of
	// stub.calls below would race with its append, which the race detector
	// reports as the same defect.
	time.Sleep(100 * time.Millisecond)
	assert.Empty(t, stub.calls, "configure must ask for no name")
}

// TestDomainGroupConfigureProgramsBeforeResolving proves the cache is read and
// the tables programmed before any name is asked for.
//
// VALIDATES: AC-7 -- filtering resumes without waiting for DNS.
// PREVENTS: a configure that resolved first, which would leave a box whose
// upstream resolver is slow to come back unfiltered for as long as the query
// takes to time out.
func TestDomainGroupConfigureProgramsBeforeResolving(t *testing.T) {
	backend := useRecordingBackend(t)
	dir := t.TempDir()
	cfg := oneGroupConfig()
	zefsPath := filepath.Join(dir, "database.zefs")

	seed := newStore(zefsPath)
	require.NoError(t, seed.open(cfg.groups))
	require.NoError(t, seed.put(
		nameKey{group: "cdn", name: "a.invalid", family: familyV4},
		resolvedName{Addresses: []string{"192.0.2.1"}, ResolvedAt: time.Now(), Status: "NOERROR"},
	))
	seed.close()

	stub := &stubResolver{byName: map[string]answer{}}
	plug := &domainPlugin{
		resolve:   stub.resolve,
		cache:     newStore(zefsPath),
		changeLog: newChangeLog(filepath.Join(dir, changeLogFileName)),
		sched:     newSchedule(),
		wake:      make(chan struct{}, 1),
		stopCh:    make(chan struct{}),
		done:      make(chan struct{}),
		config:    cfg,
	}
	require.NoError(t, plug.configure(cfg))

	// configure starts no worker, so nothing can resolve behind this read: what
	// the backend holds is what configure itself programmed, from the cache.
	applied := backend.applied()
	plug.stop()

	require.NotEmpty(t, applied, "the cached addresses reach the kernel at configure time")
	found := false
	for i := range applied {
		for j := range applied[i].Sets {
			if applied[i].Sets[j].Name == "domain_v4_cdn" && len(applied[i].Sets[j].Elements) == 1 {
				found = true
			}
		}
	}
	assert.True(t, found, "the set was programmed from the cache, before any query")
}

// TestDomainGroupRefreshFiresAtTTL proves the refresh worker asks for a name
// when its scheduled time arrives.
//
// VALIDATES: wiring row 2 -- a name's answer TTL expiring reaches the refresh
// loop, which calls the resolve path.
// PREVENTS: a schedule that computed due times nobody ever acted on, which
// every timing unit above would still pass.
func TestDomainGroupRefreshFiresAtTTL(t *testing.T) {
	useRecordingBackend(t)
	dir := t.TempDir()
	cfg := oneGroupConfig()

	resolved := make(chan string, 8)
	plug := &domainPlugin{
		resolve: func(_ context.Context, name string, qtype uint16) ([]string, uint32, string, error) {
			select {
			case resolved <- stubKey(name, qtype):
			default:
			}
			if qtype == mdns.TypeA {
				return []string{"192.0.2.1"}, 300, "NOERROR", nil
			}
			return nil, 300, "NOERROR", nil
		},
		cache:     newStore(filepath.Join(dir, "database.zefs")),
		changeLog: newChangeLog(filepath.Join(dir, changeLogFileName)),
		sched:     newSchedule(),
		wake:      make(chan struct{}, 1),
		stopCh:    make(chan struct{}),
		done:      make(chan struct{}),
		config:    cfg,
	}
	require.NoError(t, plug.cache.open(cfg.groups))
	require.NoError(t, plug.changeLog.open())
	t.Cleanup(plug.stop)

	// reset arms every unit as due NOW, which is the state a fresh configure
	// leaves: the worker must pick that up and resolve without waiting a TTL.
	plug.startRefreshWorker()
	plug.rearmSchedule(cfg)

	select {
	case got := <-resolved:
		assert.Equal(t, "a.invalid/A", got, "the earliest-due unit is resolved first")
	case <-time.After(10 * time.Second):
		t.Fatal("the refresh worker never resolved a due name")
	}

	// And the second family follows, on its own schedule entry.
	select {
	case got := <-resolved:
		assert.Equal(t, "a.invalid/AAAA", got)
	case <-time.After(10 * time.Second):
		t.Fatal("the second family was never resolved")
	}

	// The worker rearmed both units into the future rather than spinning.
	_, due, found := plug.sched.nextDue()
	require.True(t, found)
	assert.True(t, due.After(time.Now()), "a refreshed unit is rescheduled ahead, not at once")
}
