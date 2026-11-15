package domain

import (
	"errors"
	"testing"
	"time"

	mdns "github.com/miekg/dns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/diagnostic"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestFirstAnswerIsRecordedEvenWhenEmpty proves an authoritative answer
// carrying no address is written once.
//
// "Never asked" and "asked, and the name holds nothing" are different facts.
// An IPv4-only name answers NOERROR-empty for AAAA every time, so without this
// write its IPv6 family would read as unqueried for the life of the box, and
// an operator debugging a rule that filters no IPv6 would have nothing to go
// on. This is the zero-that-cannot-be-told-from-an-absence that
// ai/rules/principles.md names first.
func TestFirstAnswerIsRecordedEvenWhenEmpty(t *testing.T) {
	plug, _ := newTestPlugin(t, oneGroupConfig(), map[string]answer{
		stubKey("a.invalid", mdns.TypeAAAA): {ttl: 300, status: "NOERROR"},
	})
	key := nameKey{group: "cdn", name: "a.invalid", family: familyV6}

	changed, err := plug.resolveAndRecord(key)
	require.NoError(t, err)
	assert.False(t, changed, "an empty first answer changes no address, so nothing is reprogrammed")

	entry, seen := plug.cache.get(nameKey{group: "cdn", name: "a.invalid", family: familyV6})
	require.True(t, seen, "the answer must be recorded, or it reads as never asked")
	assert.Empty(t, entry.Addresses)
	assert.Equal(t, "NOERROR", entry.Status)
	assert.False(t, entry.ResolvedAt.IsZero())
	assert.False(t, entry.failing())
}

// TestSteadyStateWritesNothingAfterTheFirstAnswer proves an answer that
// confirms the same addresses leaves the time they were recorded alone.
//
// Its name predates spec-fetched-data-survives-restart, which replaced AC-2's
// "no zefs write occurs" with a write on every answer that renews the stored
// expiry (TestEveryAnswerRenewsExpiry). ResolvedAt is not part of the renewal:
// it dates the addresses, and `show firewall domain-group` and the data-age
// metric read it as that.
// PREVENTS: a confirming answer re-dating addresses that did not move, which
// would make the oldest data the firewall enforces look one TTL old forever.
func TestSteadyStateWritesNothingAfterTheFirstAnswer(t *testing.T) {
	plug, _ := newTestPlugin(t, oneGroupConfig(), map[string]answer{
		stubKey("a.invalid", mdns.TypeA): {records: []string{"192.0.2.1"}, ttl: 300, status: "NOERROR"},
	})

	_, err := plug.resolveAndRecord(v4Key())
	require.NoError(t, err)
	first, _ := plug.cache.get(nameKey{group: "cdn", name: "a.invalid", family: familyV4})

	for range 3 {
		_, err = plug.resolveAndRecord(v4Key())
		require.NoError(t, err)
	}

	after, _ := plug.cache.get(nameKey{group: "cdn", name: "a.invalid", family: familyV4})
	assert.Equal(t, first.ResolvedAt, after.ResolvedAt,
		"a confirming refresh must not re-date the addresses")
}

// TestFailureIsMarkedOnceAndClearedOnRecovery proves an outage is recorded at
// its start and cleared at its end, and that a continuing outage does not move
// the marker.
//
// The addresses stay exactly as they were throughout: keeping the last good
// answer is the point. What the marker adds is that an operator can see the
// firewall is enforcing addresses whose name stopped answering, which the
// addresses alone cannot say.
func TestFailureIsMarkedOnceAndClearedOnRecovery(t *testing.T) {
	script := map[string]answer{
		stubKey("a.invalid", mdns.TypeA): {records: []string{"192.0.2.1"}, ttl: 300, status: "NOERROR"},
	}
	plug, stub := newTestPlugin(t, oneGroupConfig(), script)
	entryKey := nameKey{group: "cdn", name: "a.invalid", family: familyV4}

	_, err := plug.resolveAndRecord(v4Key())
	require.NoError(t, err)
	healthy, _ := plug.cache.get(entryKey)
	require.False(t, healthy.failing())

	// The outage starts.
	stub.byName[stubKey("a.invalid", mdns.TypeA)] = answer{ttl: 300, status: "SERVFAIL"}
	_, err = plug.resolveAndRecord(v4Key())
	require.Error(t, err)

	marked, _ := plug.cache.get(entryKey)
	require.True(t, marked.failing(), "the outage must be recorded when it starts")
	assert.Equal(t, "SERVFAIL", marked.Status)
	assert.Equal(t, []string{"192.0.2.1"}, marked.Addresses, "the last good addresses stay enforced")

	// It continues. The marker is not moved, so an outage costs one write
	// however long it lasts.
	_, err = plug.resolveAndRecord(v4Key())
	require.Error(t, err)
	still, _ := plug.cache.get(entryKey)
	assert.Equal(t, marked.FailingSince, still.FailingSince,
		"a continuing outage must not rewrite the marker on every refresh")

	// It ends. The marker is cleared, or a healthy name would keep reporting
	// as failing forever.
	stub.byName[stubKey("a.invalid", mdns.TypeA)] = answer{records: []string{"192.0.2.1"}, ttl: 300, status: "NOERROR"}
	_, err = plug.resolveAndRecord(v4Key())
	require.NoError(t, err)

	recovered, _ := plug.cache.get(entryKey)
	assert.False(t, recovered.failing(), "recovery must clear the marker")
	assert.Equal(t, "NOERROR", recovered.Status)
	assert.Equal(t, []string{"192.0.2.1"}, recovered.Addresses)
}

// TestTransportErrorIsMarkedTooProves a query that never reached a server is
// recorded as a failure. Only the resolver's answer distinguishes the causes;
// to the firewall both mean "this name is not answering".
func TestTransportErrorIsMarkedToo(t *testing.T) {
	script := map[string]answer{
		stubKey("a.invalid", mdns.TypeA): {records: []string{"192.0.2.1"}, ttl: 300, status: "NOERROR"},
	}
	plug, stub := newTestPlugin(t, oneGroupConfig(), script)
	_, err := plug.resolveAndRecord(v4Key())
	require.NoError(t, err)

	stub.byName[stubKey("a.invalid", mdns.TypeA)] = answer{err: assert.AnError}
	_, err = plug.resolveAndRecord(v4Key())
	require.Error(t, err)

	entry, _ := plug.cache.get(nameKey{group: "cdn", name: "a.invalid", family: familyV4})
	assert.True(t, entry.failing())
	assert.Equal(t, "unspecified", entry.Status, "a transport error carries no response code to report")
}

// TestFailureOnANameWithNoEntryWritesNothing proves a name that has never
// answered does not gain an entry from failing. There are no addresses to
// describe, and the no-data check already reports the group.
func TestFailureOnANameWithNoEntryWritesNothing(t *testing.T) {
	plug, _ := newTestPlugin(t, oneGroupConfig(), map[string]answer{
		stubKey("a.invalid", mdns.TypeA): {err: assert.AnError},
	})
	_, err := plug.resolveAndRecord(v4Key())
	require.Error(t, err)

	_, seen := plug.cache.get(nameKey{group: "cdn", name: "a.invalid", family: familyV4})
	assert.False(t, seen, "a failure invents no entry")
}

// TestDoctorReportsAGroupWithNoAddresses proves `ze doctor` names a referenced
// group that filters nothing.
//
// The rule naming it holds its whole table back, so this is the check that
// tells an operator why a filter they wrote is not in the kernel.
func TestDoctorReportsAGroupWithNoAddresses(t *testing.T) {
	cfg := oneGroupConfig()
	dir := t.TempDir()
	cache := newStore(domainTestStorage(t, dir))
	require.NoError(t, cache.open(cfg.groups))

	diags := domainGroupDiagnostics(cfg, cache, time.Now())
	require.Len(t, diags, 1)
	assert.Equal(t, codeDomainGroupNoData, diags[0].Code)
	assert.Equal(t, diagnostic.SeverityError, diags[0].Severity)
	assert.Contains(t, diags[0].Message, "cdn")
	assert.Contains(t, diags[0].Help, "update firewall domain-group cdn")
}

// TestDoctorReportsALongFailureAsStale proves a name that stopped resolving is
// reported once the outage passes the bound, and not before.
//
// The measurement is FailingSince, not ResolvedAt. A name that resolves
// correctly every five minutes to the same addresses has an ancient
// ResolvedAt, so measuring from it would report every stable group in the box
// as broken, which is the check that gets ignored.
func TestDoctorReportsALongFailureAsStale(t *testing.T) {
	cfg := oneGroupConfig()
	dir := t.TempDir()
	cache := newStore(domainTestStorage(t, dir))
	require.NoError(t, cache.open(cfg.groups))

	now := time.Now()
	key := nameKey{group: "cdn", name: "a.invalid", family: familyV4}

	// Resolving normally, with addresses learned long ago. Not stale: nothing
	// is failing.
	require.NoError(t, cache.put(key, resolvedName{
		Addresses: []string{"192.0.2.1"}, ResolvedAt: now.Add(-90 * 24 * time.Hour), Status: "NOERROR",
	}))
	assert.Empty(t, domainGroupDiagnostics(cfg, cache, now),
		"a stable name is not stale, however long its addresses have held")

	// Failing, but inside the bound. Still not reported: a short outage is
	// ordinary and recovering from one is what last-known-good is for.
	require.NoError(t, cache.put(key, resolvedName{
		Addresses: []string{"192.0.2.1"}, ResolvedAt: now.Add(-90 * 24 * time.Hour),
		Status: "SERVFAIL", FailingSince: now.Add(-staleAfter + time.Minute),
	}))
	assert.Empty(t, domainGroupDiagnostics(cfg, cache, now), "a short outage is not reported")

	// Failing past the bound.
	since := now.Add(-staleAfter - time.Minute)
	require.NoError(t, cache.put(key, resolvedName{
		Addresses: []string{"192.0.2.1"}, ResolvedAt: now.Add(-90 * 24 * time.Hour),
		Status: "SERVFAIL", FailingSince: since,
	}))
	diags := domainGroupDiagnostics(cfg, cache, now)
	require.Len(t, diags, 1)
	assert.Equal(t, codeDomainGroupStaleData, diags[0].Code)
	assert.Equal(t, diagnostic.SeverityWarning, diags[0].Severity)
	assert.Contains(t, diags[0].Message, "a.invalid", "the message names the name that stopped answering")
	assert.Positive(t, diags[0].Actual)
}

// TestDoctorIgnoresAGroupNoRuleNames proves an unreferenced group raises
// nothing. It enforces nothing, so reporting it would be noise.
func TestDoctorIgnoresAGroupNoRuleNames(t *testing.T) {
	cfg := &domainConfig{groups: []group{{Name: "cdn", Names: []string{"a.invalid"}}}}
	dir := t.TempDir()
	cache := newStore(domainTestStorage(t, dir))
	require.NoError(t, cache.open(cfg.groups))

	assert.Empty(t, domainGroupDiagnostics(cfg, cache, time.Now()))
}

// TestDoctorCodesAreRegistered proves `ze show diagnostic <code>` can describe both
// codes. A diagnostic with no metadata prints a bare identifier.
func TestDoctorCodesAreRegistered(t *testing.T) {
	codes := map[string]bool{}
	for _, meta := range domainDiagnosticCodes {
		codes[meta.Code] = true
		assert.NotEmpty(t, meta.Title, meta.Code)
		assert.NotEmpty(t, meta.Description, meta.Code)
		assert.NotEmpty(t, meta.Examples, meta.Code)
	}
	assert.True(t, codes[codeDomainGroupStaleData])
	assert.True(t, codes[codeDomainGroupNoData])
}

// TestDoctorReportsAnUnreadableStoredAnswer proves `ze doctor` names a stored
// answer it cannot decode, and only that one.
//
// The name reads as unresolved, so a start asks for it at once and the next
// answer rewrites it; until then the group enforces less than the box learned,
// and the WARN at load is a line an operator may never read.
// VALIDATES: spec-fetched-data-survives-restart, Integration Checklist row
// "Doctor code per problem class" (unreadable DNS answer).
// PREVENTS: a corrupt value that silently shrinks a permit list across a
// restart.
func TestDoctorReportsAnUnreadableStoredAnswer(t *testing.T) {
	cfg := &domainConfig{
		groups: []group{{Name: "cdn", Names: []string{"a.invalid", "b.invalid"}, TTLFloor: 60}},
		refs:   []termRef{{Group: "cdn", TableName: "ze_filter"}},
	}
	dir := t.TempDir()
	persistence := domainTestStorage(t, dir)
	readable := nameKey{group: "cdn", name: "a.invalid", family: familyV4}
	corrupt := nameKey{group: "cdn", name: "b.invalid", family: familyV4}

	seed := newStore(persistence)
	require.NoError(t, seed.put(readable, resolvedName{
		Addresses: []string{"192.0.2.1"}, ResolvedAt: time.Now(), Status: "NOERROR",
	}))
	require.NoError(t, persistence.WriteKey(storedAnswerKey(corrupt), []byte("{not json")))

	cache := newStore(persistence)
	require.NoError(t, cache.open(cfg.groups))

	diags := domainGroupDiagnostics(cfg, cache, time.Now())
	require.Len(t, diags, 1, "the readable answer raises nothing, the group has addresses")
	assert.Equal(t, codeDomainGroupUnreadableData, diags[0].Code)
	assert.Equal(t, diagnostic.SeverityWarning, diags[0].Severity)
	assert.Contains(t, diags[0].Message, "b.invalid")
	assert.Contains(t, diags[0].Message, "ipv4")
	assert.Contains(t, diags[0].Help, "update firewall domain-group cdn")
	assert.Equal(t, "update firewall domain-group cdn", diags[0].Fix, "the fix is the command alone")
}

// TestDoctorUnreadableCodeIsRegistered proves `ze show diagnostic` can describe
// the unreadable-answer code, and that the check init() registered declares
// every code the post-config check emits.
//
// It reads the registered check rather than a list built beside it, so a code
// dropped from the registration's literal Codes goes red here.
func TestDoctorUnreadableCodeIsRegistered(t *testing.T) {
	found := false
	for _, meta := range domainDiagnosticCodes {
		if meta.Code != codeDomainGroupUnreadableData {
			continue
		}
		found = true
		assert.NotEmpty(t, meta.Title)
		assert.NotEmpty(t, meta.Description)
		assert.NotEmpty(t, meta.Examples)
	}
	assert.True(t, found)

	var registered []string
	checks := 0
	for _, check := range diagnostic.DoctorChecksForPhase(diagnostic.DoctorPhasePostConfig) {
		if check.Name != "firewall-domain-group-data" {
			continue
		}
		checks++
		registered = check.Codes
	}
	require.Equal(t, 1, checks, "init registers the domain-group data check once")
	assert.Contains(t, registered, codeDomainGroupUnreadableData)
	assert.Contains(t, registered, codeDomainGroupStaleData)
	assert.Contains(t, registered, codeDomainGroupNoData)
}

// failingWrites wraps a store's persistence and refuses every write while
// refuse is set, the way a full or read-only store does.
type failingWrites struct {
	cacheKeys
	refuse bool
}

var errStoreRefused = errors.New("store refused the write")

func (f *failingWrites) WriteKey(key string, data []byte) error {
	if f.refuse {
		return errStoreRefused
	}
	return f.cacheKeys.WriteKey(key, data)
}

// TestUnsavedAnswerIsReportedUntilAWriteSucceeds proves an answer the store
// refuses is named by the runtime doctor check, with its cause and the command
// that asks again, and that the next write of the unit clears it.
//
// A refused write leaves the name enforcing what it held, and a restart does
// not have the answer; the WARN is a line an operator may never read.
// VALIDATES: spec-fetched-data-survives-restart, Integration Checklist row
// "Doctor code per problem class" (unsaved DNS answer).
// PREVENTS: a store that stopped persisting answers with nothing in ze doctor.
func TestUnsavedAnswerIsReportedUntilAWriteSucceeds(t *testing.T) {
	plug, _ := newTestPlugin(t, oneGroupConfig(), map[string]answer{
		stubKey("a.invalid", mdns.TypeA): {records: []string{"192.0.2.1"}, ttl: 300, status: "NOERROR"},
	})
	keys := &failingWrites{cacheKeys: plug.cache.keys, refuse: true}
	plug.cache.keys = keys

	changed, err := plug.resolveAndRecord(v4Key())
	require.NoError(t, err)
	assert.False(t, changed, "a change the store refused is not programmed")
	_, seen := plug.cache.get(v4Key())
	assert.False(t, seen, "memory stays equal to the store")

	diags, err := plug.answerDoctorCheck(unsavedAnswerCheckName)
	require.NoError(t, err)
	require.Len(t, diags, 1)
	assert.Equal(t, codeDomainGroupUnsavedAnswer, diags[0].Code)
	assert.Equal(t, string(diagnostic.SeverityWarning), diags[0].Severity)
	assert.Contains(t, diags[0].Message, "a.invalid")
	assert.Contains(t, diags[0].Message, "ipv4")
	assert.Contains(t, diags[0].Message, errStoreRefused.Error(), "the finding names the cause")
	assert.Equal(t, "update firewall domain-group cdn", diags[0].Fix)

	keys.refuse = false
	changed, err = plug.resolveAndRecord(v4Key())
	require.NoError(t, err)
	assert.True(t, changed, "the saved answer is programmed")

	diags, err = plug.answerDoctorCheck(unsavedAnswerCheckName)
	require.NoError(t, err)
	assert.Empty(t, diags, "a write that succeeds clears the finding")
}

// TestUnsavedAnswerCheckIsDeclaredWithItsCode proves the runtime check
// declares the code it answers, that `ze show diagnostic` can describe it, and
// that the callback refuses a check name it never declared.
func TestUnsavedAnswerCheckIsDeclaredWithItsCode(t *testing.T) {
	assert.Equal(t, sdk.DoctorPhaseRuntime, unsavedAnswerCheckDecl.Phase)
	assert.Equal(t, []string{codeDomainGroupUnsavedAnswer}, unsavedAnswerCheckDecl.Codes)

	found := false
	for _, meta := range domainDiagnosticCodes {
		if meta.Code != codeDomainGroupUnsavedAnswer {
			continue
		}
		found = true
		assert.NotEmpty(t, meta.Title)
		assert.NotEmpty(t, meta.Description)
		assert.NotEmpty(t, meta.Examples)
	}
	assert.True(t, found)

	plug, _ := newTestPlugin(t, oneGroupConfig(), nil)
	_, err := plug.answerDoctorCheck("firewall-domain-group-data")
	assert.Error(t, err)
}
