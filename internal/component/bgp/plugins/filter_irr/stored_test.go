// Design: docs/architecture/bgp/filter-irr.md -- the filter reads what the irr plugin stored
package filter_irr

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/netip"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/resolve/irr/store"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
	"github.com/ze-software/ze/pkg/zefs"
)

// fakeConsumerState is the daemon store as the filter reaches it, holding what
// the irr plugin would have written. It counts the reads, so a test can tell a
// pass that read nothing from one that read everything.
type fakeConsumerState struct {
	mu         sync.Mutex
	values     map[string][]byte
	generation uint32
	reads      int
	puts       []string
	emits      []string
	putErr     error
	// onGet, when set, runs after each read with the key read, outside mu: a
	// test reconfigures the plugin there while the reader is reading.
	onGet func(key string)
	// busy, when set, holds every read until it is closed: the store is busy
	// with another write.
	busy chan struct{}
}

func newFakeConsumerState() *fakeConsumerState {
	return &fakeConsumerState{values: make(map[string][]byte)}
}

func (s *fakeConsumerState) StateGet(_ context.Context, key string) ([]byte, bool, error) {
	s.mu.Lock()
	busy := s.busy
	s.mu.Unlock()
	if busy != nil {
		<-busy
	}
	s.mu.Lock()
	s.reads++
	value, ok := s.values[key]
	hook := s.onGet
	s.mu.Unlock()
	if hook != nil {
		hook(key)
	}
	return value, ok, nil
}

func (s *fakeConsumerState) StatePut(_ context.Context, key string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.putErr != nil {
		return s.putErr
	}
	s.puts = append(s.puts, key)
	s.values[key] = data
	return nil
}

func (s *fakeConsumerState) EmitEvent(_ context.Context, namespace, eventType, _, _, event string) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.emits = append(s.emits, namespace+"/"+eventType+" "+event)
	return 1, nil
}

func (s *fakeConsumerState) readCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.reads
}

// raise raises the generation as StateIncrement does and returns it.
func (s *fakeConsumerState) raise() uint32 {
	s.generation++
	var counter [4]byte
	binary.BigEndian.PutUint32(counter[:], s.generation)
	s.values[store.GenerationKey()] = counter[:]
	return s.generation
}

// storeList writes asSet's dataset holding prefixes (space separated), raises
// the generation and writes the pointer, in the irr plugin's order, and
// returns the notice the plugin would send.
func (s *fakeConsumerState) storeList(t *testing.T, asSet, prefixes string) string {
	t.Helper()
	in := store.DatasetInput{
		ASSet: asSet, Server: "whois.radb.net",
		RefreshedAt: time.Unix(1_700_000_000, 0), CheckedAt: time.Unix(1_700_000_600, 0),
	}
	for text := range strings.FieldsSeq(prefixes) {
		prefix := netip.MustParsePrefix(text)
		if prefix.Addr().Is4() {
			in.IPv4 = append(in.IPv4, prefix)
		} else {
			in.IPv6 = append(in.IPv6, prefix)
		}
	}
	dataset, err := store.NewDataset(in)
	if err != nil {
		t.Fatalf("NewDataset: %v", err)
	}
	value := make([]byte, dataset.EncodedLen())
	dataset.WriteTo(value, 0)
	digest := store.DatasetDigest(value)
	s.mu.Lock()
	defer s.mu.Unlock()
	pointer := store.Pointer{Server: dataset.Server(), Generation: s.raise(), Digest: hex.EncodeToString(digest[:])}
	s.values[pointer.DatasetKey(asSet)] = value
	encoded, err := store.EncodePointer(pointer)
	if err != nil {
		t.Fatal(err)
	}
	s.values[store.PointerKey(asSet)] = encoded
	return noticeOf(t, store.DatasetNotice{ASSet: asSet, Server: pointer.Server, Generation: pointer.Generation, Digest: pointer.Digest})
}

// clearSet removes asSet's pointer and dataset as `clear irr` does and returns
// the removal notice.
func (s *fakeConsumerState) clearSet(t *testing.T, asSet string) string {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.values, store.PointerKey(asSet))
	return noticeOf(t, store.DatasetNotice{ASSet: asSet, Server: "whois.radb.net", Generation: s.raise(), Removed: true})
}

// boundASN is the AS number every bind test binds: the one peer these tests
// enroll.
const boundASN uint32 = 65001

// bind stores the binding of boundASN to asSet, as the irr plugin does after
// PeeringDB answers.
func (s *fakeConsumerState) bind(t *testing.T, asSet string) {
	t.Helper()
	encoded, err := store.EncodeBinding(store.Binding{ASSet: asSet, PeeringDBURL: "https://www.peeringdb.com", DiscoveredAt: time.Unix(1_700_000_000, 0)})
	if err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values[store.BindingKey(boundASN)] = encoded
}

func noticeOf(t *testing.T, notice store.DatasetNotice) string {
	t.Helper()
	payload, err := json.Marshal(notice)
	if err != nil {
		t.Fatal(err)
	}
	return string(payload)
}

// storedPlugin builds a plugin enrolled for each ASN in byConfigured (ASN ->
// configured as-set, "" for a PeeringDB-chosen one), its reader over state.
func storedPlugin(state *fakeConsumerState, byConfigured map[uint32]string) *irrPlugin {
	plug := &irrPlugin{byASN: make(map[uint32]*asnState)}
	for asn, configured := range byConfigured {
		plug.byASN[asn] = newASNState(asn, configured)
	}
	plug.mu.Lock()
	plug.publishLocked()
	plug.mu.Unlock()
	plug.reader = newStoreReader(plug, state)
	return plug
}

// readStored runs the reader pass OnStarted asks for: every ASN, then the
// interest key.
func readStored(plug *irrPlugin) {
	plug.reader.readAll()
	plug.reader.pass(context.Background())
}

// deliver hands the reader a notice and runs the pass it wakes.
func deliver(plug *irrPlugin, notice string) {
	plug.reader.notice(notice)
	plug.reader.pass(context.Background())
}

// testState is the fake store the reader of a test plugin reads.
func testState(plug *irrPlugin) *fakeConsumerState {
	state, ok := plug.reader.state.(*fakeConsumerState)
	if !ok {
		panic("BUG: test plugin reader is not over a fakeConsumerState")
	}
	return state
}

func announceFor(plug *irrPlugin, asn, prefix string) sdk.FilterAction {
	return plug.handleFilterUpdate(&sdk.FilterUpdateInput{
		Filter: "bgp-filter-irr:" + asn, Peer: readmitTestPeer, Fields: announceFields(prefix),
	}).Action
}

// VALIDATES: TestConsumerReadsNoFetch -- the start read publishes what the irr
// plugin stored, so a listed prefix is accepted with no fetch, and the filter
// holds no IRR or PeeringDB client to fetch with (AC-1, AC-14).
// METHOD: store AS-TEST, run the OnStarted pass, judge a listed and an
// unlisted prefix; walk the plugin's fields for a resolver type.
func TestConsumerReadsNoFetch(t *testing.T) {
	state := newFakeConsumerState()
	state.storeList(t, "AS-TEST", "10.0.0.0/24")
	plug := storedPlugin(state, map[uint32]string{65001: "AS-TEST"})

	if got := announceFor(plug, "65001", "10.0.0.0/24"); got != sdk.FilterReject {
		t.Fatalf("before the start read: action = %v, want reject (no list)", got)
	}
	readStored(plug)
	if got := announceFor(plug, "65001", "10.0.0.0/24"); got != sdk.FilterAccept {
		t.Fatalf("listed prefix after the start read: action = %v, want accept", got)
	}
	if got := announceFor(plug, "65001", "192.0.2.0/24"); got != sdk.FilterReject {
		t.Fatalf("unlisted prefix: action = %v, want reject", got)
	}
	st := plug.byASN[65001]
	if st.v4Count != 1 || st.pointer.Generation != 1 || st.checkedAt.Unix() != 1_700_000_600 {
		t.Fatalf("state = %+v, want the stored copy's counts, generation and checked-at", st)
	}

	resolverType := reflect.TypeFor[store.PrefixStore]()
	fields := reflect.TypeFor[irrPlugin]()
	for i := range fields.NumField() {
		field := fields.Field(i).Type
		for field.Kind() == reflect.Pointer {
			field = field.Elem()
		}
		if field == resolverType || strings.Contains(field.PkgPath(), "resolve/irr") && !strings.HasSuffix(field.PkgPath(), "/store") ||
			strings.Contains(field.PkgPath(), "resolve/peeringdb") {
			t.Fatalf("irrPlugin.%s holds a resolver (%s): the filter must fetch nothing", fields.Field(i).Name, field)
		}
	}
}

// VALIDATES: the start pass tells the irr plugin what the filter enforces,
// after the publish: configured sets by name, PeeringDB-chosen ones by ASN.
func TestStartWritesInterestAfterPublish(t *testing.T) {
	state := newFakeConsumerState()
	plug := storedPlugin(state, map[uint32]string{65001: "as-test", 65002: ""})
	readStored(plug)
	value, found, _ := state.StateGet(context.Background(), zefs.KeyIRRInterest.Key(pluginName))
	if !found {
		t.Fatalf("no interest key written; puts = %q", state.puts)
	}
	in, err := store.DecodeInterest(value)
	if err != nil || len(in.ASSets) != 1 || in.ASSets[0] != "AS-TEST" || len(in.ASNs) != 1 || in.ASNs[0] != 65002 {
		t.Fatalf("interest = %+v, %v; want AS-TEST and AS65002", in, err)
	}
	if len(state.emits) != 1 || !strings.HasPrefix(state.emits[0], "irr/interest-changed ") {
		t.Fatalf("emits = %q, want one interest notice", state.emits)
	}
	// An unchanged interest is not written again.
	readStored(plug)
	if len(state.puts) != 1 {
		t.Fatalf("puts = %q, want the unchanged interest written once", state.puts)
	}
}

// VALIDATES: TestNoticeGapRereads -- a notice one generation ahead reads only
// the set it names; a notice two generations ahead reads every pointer,
// because the one in between was lost (AC-8, R-3).
// METHOD: two enrolled sets; store a new copy of each, deliver only the second
// notice; both must be enforced.
func TestNoticeGapRereads(t *testing.T) {
	state := newFakeConsumerState()
	state.storeList(t, "AS-ONE", "10.1.0.0/24")
	state.storeList(t, "AS-TWO", "10.2.0.0/24")
	plug := storedPlugin(state, map[uint32]string{65001: "AS-ONE", 65002: "AS-TWO"})
	readStored(plug)

	// In sequence: a notice for AS-ONE reads AS-ONE alone.
	deliver(plug, state.storeList(t, "AS-ONE", "10.1.0.0/24 10.1.1.0/24"))
	if got := announceFor(plug, "65001", "10.1.1.0/24"); got != sdk.FilterAccept {
		t.Fatalf("in-sequence notice not applied: action = %v", got)
	}

	// The AS-TWO notice is lost; the next AS-ONE notice skips a generation.
	state.storeList(t, "AS-TWO", "10.2.0.0/24 10.2.1.0/24")
	deliver(plug, state.storeList(t, "AS-ONE", "10.1.0.0/24 10.1.1.0/24 10.1.2.0/24"))
	if got := announceFor(plug, "65002", "10.2.1.0/24"); got != sdk.FilterAccept {
		t.Fatalf("the gap did not re-read AS-TWO: action = %v, want accept", got)
	}
	if seen := plug.reader.seen.Load(); seen != 5 {
		t.Fatalf("seen = %d, want 5", seen)
	}
}

// VALIDATES: a notice in sequence reads only what it names: an ASN on another
// set is not read at all.
func TestNoticeInSequenceReadsOnlyItsSet(t *testing.T) {
	state := newFakeConsumerState()
	state.storeList(t, "AS-ONE", "10.1.0.0/24")
	state.storeList(t, "AS-TWO", "10.2.0.0/24")
	plug := storedPlugin(state, map[uint32]string{65001: "AS-ONE", 65002: "AS-TWO"})
	readStored(plug)
	notice := state.storeList(t, "AS-ONE", "10.1.0.0/24 10.1.1.0/24")
	before := state.readCount()
	deliver(plug, notice)
	// One pointer and one dataset: AS-TWO's pointer is not read.
	if reads := state.readCount() - before; reads != 2 {
		t.Fatalf("an in-sequence notice made %d reads, want 2", reads)
	}
}

// VALIDATES: a removal notice drops the list, so the peer's announcements are
// rejected and held (AC-16); a stored value that cannot be used keeps the list
// enforced before (AC-22).
func TestRemovedSetDropsAndUnreadableKeeps(t *testing.T) {
	state := newFakeConsumerState()
	state.storeList(t, "AS-TEST", "10.0.0.0/24")
	plug := storedPlugin(state, map[uint32]string{65001: "AS-TEST"})
	readStored(plug)

	// A corrupt dataset under a new pointer: the old list stays.
	notice := state.storeList(t, "AS-TEST", "10.0.0.0/24 10.0.1.0/24")
	state.mu.Lock()
	for key, value := range state.values {
		if strings.HasPrefix(key, "meta/irr-data/") {
			value[len(value)-1] ^= 0xff
		}
	}
	state.mu.Unlock()
	deliver(plug, notice)
	if got := announceFor(plug, "65001", "10.0.0.0/24"); got != sdk.FilterAccept {
		t.Fatalf("an unreadable copy dropped the enforced list: action = %v", got)
	}
	if plug.byASN[65001].readErr == "" {
		t.Fatal("an unreadable copy left no read error for show and doctor")
	}

	deliver(plug, state.clearSet(t, "AS-TEST"))
	if got := announceFor(plug, "65001", "10.0.0.0/24"); got != sdk.FilterReject {
		t.Fatalf("a cleared set is still enforced: action = %v, want reject", got)
	}
}

// VALIDATES: a PeeringDB-chosen set follows its binding on every notice: the
// owner announces a moved binding only by the notice of the new set.
func TestBindingFollowedOnNotice(t *testing.T) {
	state := newFakeConsumerState()
	state.bind(t, "AS-OLD")
	state.storeList(t, "AS-OLD", "10.0.0.0/24")
	plug := storedPlugin(state, map[uint32]string{65001: ""})
	readStored(plug)
	if got := announceFor(plug, "65001", "10.0.0.0/24"); got != sdk.FilterAccept {
		t.Fatalf("bound set not enforced: action = %v", got)
	}
	state.bind(t, "AS-NEW")
	deliver(plug, state.storeList(t, "AS-NEW", "192.0.2.0/24"))
	if got := announceFor(plug, "65001", "192.0.2.0/24"); got != sdk.FilterAccept {
		t.Fatalf("moved binding not followed: action = %v", got)
	}
	if got := announceFor(plug, "65001", "10.0.0.0/24"); got != sdk.FilterReject {
		t.Fatalf("old bound set still enforced: action = %v", got)
	}
}

// VALIDATES: a configure reads and writes no store, and a reload that changes
// a peer's set publishes it with no list until the new set's copy is read: the
// old set's list is never enforced under the new set.
func TestReloadToAnotherSetStartsFromNoList(t *testing.T) {
	state := newFakeConsumerState()
	state.storeList(t, "AS-OLD", "10.0.0.0/24")
	plug := storedPlugin(state, map[uint32]string{65001: "AS-OLD"})
	readStored(plug)
	before := state.readCount()
	plug.configureOf(irrConfigFor("AS-NEW"))
	if state.readCount() != before {
		t.Fatal("configure read the store; the reader alone reads it")
	}
	if got := announceFor(plug, "65001", "10.0.0.0/24"); got != sdk.FilterReject {
		t.Fatalf("the old set's list is enforced under the new set: action = %v", got)
	}
}

// VALIDATES: an interest write the store refuses is reported and tried again.
func TestInterestWriteRefusedIsReported(t *testing.T) {
	state := newFakeConsumerState()
	state.putErr = errors.New("store refused")
	plug := storedPlugin(state, map[uint32]string{65001: "AS-TEST"})
	plug.reader.readAll()
	if plug.reader.pass(context.Background()) {
		t.Fatal("a refused interest write reported success")
	}
	findings, err := plug.answerDoctorCheck(storedCopyCheckName)
	if err != nil || len(findings) != 1 || findings[0].Code != codeIRRInterestUnsaved || findings[0].Fix == "" {
		t.Fatalf("findings = %+v, %v; want the interest-unsaved code with a fix", findings, err)
	}
}

// VALIDATES: a reload that deletes the configured as-set drops that set's list
// at once, tells the irr plugin the AS number is now chosen by PeeringDB, and
// then enforces the set the binding names, never the deleted one. (Replaces
// TestRemovingConfiguredASSetRediscoversThroughPeeringDB: the discovery itself
// is the irr plugin's, TestDiscoveryFollowsPeeringDB there.)
// METHOD: read AS-OLD, reload with no as-set, read again; check the verdicts
// and the stored interest; then bind AS-PDB, store it and deliver its notice.
func TestReloadRemovingTheSetFollowsTheBinding(t *testing.T) {
	state := newFakeConsumerState()
	state.storeList(t, "AS-OLD", "10.0.0.0/24")
	plug := storedPlugin(state, map[uint32]string{65001: "AS-OLD"})
	readStored(plug)
	if got := announceFor(plug, "65001", "10.0.0.0/24"); got != sdk.FilterAccept {
		t.Fatalf("AS-OLD not enforced before the reload: action = %v", got)
	}

	plug.configureOf(irrConfigFor(""))
	if got := announceFor(plug, "65001", "10.0.0.0/24"); got != sdk.FilterReject {
		t.Fatalf("the deleted set's list was carried over: action = %v, want reject", got)
	}
	readStored(plug)
	if got := announceFor(plug, "65001", "10.0.0.0/24"); got != sdk.FilterReject {
		t.Fatalf("with no binding stored the deleted set's list is enforced: action = %v, want reject", got)
	}
	value, found, _ := state.StateGet(context.Background(), zefs.KeyIRRInterest.Key(pluginName))
	if !found {
		t.Fatal("no interest key written after the reload")
	}
	in, err := store.DecodeInterest(value)
	if err != nil || len(in.ASSets) != 0 || len(in.ASNs) != 1 || in.ASNs[0] != 65001 {
		t.Fatalf("interest = %+v, %v; want AS 65001 alone, AS-OLD no longer named", in, err)
	}

	state.bind(t, "AS-PDB")
	deliver(plug, state.storeList(t, "AS-PDB", "192.0.2.0/24"))
	if got := announceFor(plug, "65001", "192.0.2.0/24"); got != sdk.FilterAccept {
		t.Fatalf("the bound AS-PDB list is not enforced: action = %v", got)
	}
	if got := announceFor(plug, "65001", "10.0.0.0/24"); got != sdk.FilterReject {
		t.Fatalf("the deleted AS-OLD list is still enforced: action = %v", got)
	}
	if st := plug.byASN[65001]; st.asSet != "AS-PDB" {
		t.Fatalf("as-set = %q, want AS-PDB", st.asSet)
	}
}

// VALIDATES: a reload that configures no as-set, before and after, keeps the
// list of the set the binding chose, with no read: the configured/discovered
// split does not turn every reload into an empty list.
// METHOD: bind and store AS-PDB, read it, then reload twice with no as-set and
// judge a listed prefix after each, before any further read.
func TestReloadKeepsADiscoveredASSet(t *testing.T) {
	state := newFakeConsumerState()
	state.bind(t, "AS-PDB")
	state.storeList(t, "AS-PDB", "192.0.2.0/24")
	plug := storedPlugin(state, map[uint32]string{65001: ""})
	readStored(plug)
	if got := announceFor(plug, "65001", "192.0.2.0/24"); got != sdk.FilterAccept {
		t.Fatalf("the bound set is not enforced: action = %v", got)
	}
	for reload := range 2 {
		before := state.readCount()
		plug.configureOf(irrConfigFor(""))
		if state.readCount() != before {
			t.Fatalf("reload %d read the store; the reader alone reads it", reload)
		}
		if got := announceFor(plug, "65001", "192.0.2.0/24"); got != sdk.FilterAccept {
			t.Fatalf("reload %d dropped the discovered list: action = %v", reload, got)
		}
		if st := plug.byASN[65001]; st.asSet != "AS-PDB" {
			t.Fatalf("reload %d: as-set = %q, want the discovered AS-PDB", reload, st.asSet)
		}
	}
}

// readWhileReloading stores AS-OLD, builds the plugin with AS 65001 configured
// for it, and runs the start read with a reload to reloadTo installed while
// the AS-OLD dataset is being read. It returns the plugin and the state AS
// 65001 had before the read.
func readWhileReloading(t *testing.T, reloadTo string) (*irrPlugin, *asnState) {
	t.Helper()
	state := newFakeConsumerState()
	state.storeList(t, "AS-OLD", "10.0.0.0/24")
	plug := storedPlugin(state, map[uint32]string{65001: "AS-OLD"})
	captured := plug.byASN[65001]
	var once sync.Once
	state.onGet = func(key string) {
		if !strings.HasPrefix(key, "meta/irr-data/") || !strings.Contains(key, "/AS-OLD/") {
			return
		}
		once.Do(func() { plug.configureOf(irrConfigFor(reloadTo)) })
	}
	readStored(plug)
	if plug.byASN[65001] == captured {
		t.Fatal("the reload did not run while the dataset was read, so the test cannot tell")
	}
	return plug, captured
}

// VALIDATES: a read whose ASN a reload moved to another set while it read
// applies nothing: the new set has no list until its own copy is read.
// (Replaces TestRefreshDropsLookupAReloadSuperseded.)
// METHOD: reload to AS-NEW while the AS-OLD dataset is read; AS-NEW is not
// stored, so the next read leaves it with no list too.
func TestReadDropsAReadAReloadSuperseded(t *testing.T) {
	plug, _ := readWhileReloading(t, "AS-NEW")
	for pass := range 2 {
		st := plug.byASN[65001]
		if st.asSet != "AS-NEW" {
			t.Fatalf("pass %d: as-set = %q, want AS-NEW: the superseded read overwrote it", pass, st.asSet)
		}
		if st.list != nil {
			t.Fatalf("pass %d: list = %v, want none: the AS-OLD copy was applied to AS-NEW", pass, st.list.entries)
		}
		if got := announceFor(plug, "65001", "10.0.0.0/24"); got != sdk.FilterReject {
			t.Fatalf("pass %d: the AS-OLD list is enforced under AS-NEW: action = %v", pass, got)
		}
		readStored(plug)
	}
}

// VALIDATES: a read whose ASN a reload replaced while keeping its set is
// applied, although the state it read for is gone. (Replaces
// TestRefreshAppliesLookupAReloadKeptItsASSet.)
// METHOD: reload keeping AS-OLD while the AS-OLD dataset is read.
// PREVENTS: a drop keyed on the state pointer, which every reload replaces,
// discarding every read that races a reload.
func TestReadAppliesAReadAReloadKeptItsSet(t *testing.T) {
	plug, captured := readWhileReloading(t, "AS-OLD")
	st := plug.byASN[65001]
	if st == captured {
		t.Fatal("the reload did not replace the state")
	}
	if st.list == nil || !admitsAll(t, st.list, "10.0.0.0/24") {
		t.Fatal("the AS-OLD copy was not applied: a reload that kept the set dropped the read")
	}
	if got := announceFor(plug, "65001", "10.0.0.0/24"); got != sdk.FilterAccept {
		t.Fatalf("listed prefix after the read: action = %v, want accept", got)
	}
}

// VALIDATES: a read for a configured set a reload deleted while it read
// applies nothing, and the ASN never falls back to the deleted set. (Replaces
// TestInFlightLookupForADeletedSetCommitsNothing.)
// METHOD: reload with no as-set while the AS-OLD dataset is read; no binding is
// stored, so the next read leaves no list either.
func TestReadForADeletedSetAppliesNothing(t *testing.T) {
	plug, _ := readWhileReloading(t, "")
	for pass := range 2 {
		st := plug.byASN[65001]
		if st.list != nil {
			t.Fatalf("pass %d: list = %v, want none: the deleted AS-OLD copy was applied", pass, st.list.entries)
		}
		if st.asSet == "AS-OLD" {
			t.Fatalf("pass %d: as-set is the deleted AS-OLD", pass)
		}
		if got := announceFor(plug, "65001", "10.0.0.0/24"); got != sdk.FilterReject {
			t.Fatalf("pass %d: the deleted set's list is enforced: action = %v", pass, got)
		}
		readStored(plug)
	}
}

// VALIDATES: a reader pass and a configure share the plugin's state safely:
// run under -race, a data race between the two fails the test. (Replaces
// TestRefreshASNStoreFieldRace.)
// METHOD: reconfigure in a loop on one goroutine, alternating the set, while
// the test goroutine runs reader passes and judges UPDATEs; then settle on
// AS-OLD and check its list is enforced.
func TestReaderPassRacesConfigure(t *testing.T) {
	state := newFakeConsumerState()
	state.storeList(t, "AS-OLD", "10.0.0.0/24")
	state.storeList(t, "AS-NEW", "192.0.2.0/24")
	plug := storedPlugin(state, map[uint32]string{65001: "AS-OLD"})
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Go(func() {
		for i := 0; ; i++ {
			select {
			case <-stop:
				return
			default:
			}
			set := "AS-OLD"
			if i%2 == 1 {
				set = "AS-NEW"
			}
			plug.configureOf(irrConfigFor(set))
		}
	})
	for range 200 {
		readStored(plug)
		announceFor(plug, "65001", "10.0.0.0/24")
	}
	close(stop)
	wg.Wait()

	plug.configureOf(irrConfigFor("AS-OLD"))
	readStored(plug)
	if got := announceFor(plug, "65001", "10.0.0.0/24"); got != sdk.FilterAccept {
		t.Fatalf("after the race settled on AS-OLD: action = %v, want accept", got)
	}
}

// VALIDATES: AC-18 -- while the store is busy (the irr plugin's 5 s write
// holding it) the evaluator answers at once from the list already published,
// and the new list is published within one second of the write completing.
// METHOD: run the reader's worker with every store read held by a gate, ask it
// to read, and judge an UPDATE while the gate is shut; then store the new
// copy, open the gate, deliver the notice and time the publish.
func TestPublishWithinASecondOfTheStoredWrite(t *testing.T) {
	state := newFakeConsumerState()
	state.storeList(t, "AS-TEST", "10.0.0.0/24")
	plug := storedPlugin(state, map[uint32]string{65001: "AS-TEST"})
	readStored(plug)

	busy := make(chan struct{})
	var open sync.Once
	release := func() {
		open.Do(func() {
			state.mu.Lock()
			state.busy = nil
			state.mu.Unlock()
			close(busy)
		})
	}
	state.mu.Lock()
	state.busy = busy
	state.mu.Unlock()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		plug.reader.run(ctx)
	}()
	t.Cleanup(func() {
		release()
		cancel()
		<-done
	})
	before := state.readCount()
	plug.reader.readAll()

	judged := make(chan sdk.FilterAction, 1)
	go func() { judged <- announceFor(plug, "65001", "10.0.0.0/24") }()
	select {
	case got := <-judged:
		if got != sdk.FilterAccept {
			t.Fatalf("while the store is busy: action = %v, want accept from the published list", got)
		}
	case <-time.After(time.Second):
		t.Fatal("the evaluator waited on the busy store")
	}

	// The busy store frees up and the held pass ends: it reads the generation
	// and the unchanged pointer. Only then does the irr plugin's write
	// complete, so the notice alone can publish the new copy.
	release()
	freed := time.Now()
	for state.readCount() < before+2 {
		if time.Since(freed) > time.Second {
			t.Fatal("the held pass did not finish within a second of the store freeing up")
		}
		runtime.Gosched()
	}
	notice := state.storeList(t, "AS-TEST", "10.0.0.0/24 10.0.1.0/24")
	written := time.Now()
	plug.reader.notice(notice)
	for announceFor(plug, "65001", "10.0.1.0/24") != sdk.FilterAccept {
		if time.Since(written) > time.Second {
			t.Fatal("the new list was not published within one second of the write completing")
		}
		runtime.Gosched()
	}
}
