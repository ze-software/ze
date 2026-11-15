// Design: docs/architecture/firewall/firewall-irr.md -- the firewall reads what the irr plugin stored
// Related: irr_test.go -- recordingBackend, useRecordingBackend, setsByName
package irr

// VALIDATES: AC-2, AC-6, AC-8, AC-11, AC-16, AC-21, AC-22, AC-23 for the
// firewall consumer: the plugin programs the stored copies the irr plugin
// wrote, re-reads on a notice, recovers a lost notice, never applies or reads
// in the handshake, and refuses a commit naming a reference with nothing
// stored.
// PREVENTS: the firewall fetching for itself, warning at every start, or
// enforcing an empty list where a stored one exists.

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/netip"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/resolve/irr/store"
	"github.com/ze-software/ze/internal/core/diagnostic"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
	"github.com/ze-software/ze/pkg/zefs"
)

// fakeConsumerState is the daemon store as the plugin reaches it, holding what
// the irr plugin would have written. It counts reads, so a test can tell a
// pass that read nothing from one that read everything.
type fakeConsumerState struct {
	mu         sync.Mutex
	values     map[string][]byte
	generation uint32
	reads      int
	emits      []string
	getErr     map[string]error
	putErr     map[string]error
}

func newFakeConsumerState() *fakeConsumerState {
	return &fakeConsumerState{values: make(map[string][]byte), getErr: make(map[string]error), putErr: make(map[string]error)}
}

// failWrite makes every write of key fail with a store error.
func (s *fakeConsumerState) failWrite(key, reason string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.putErr[key] = errors.New(reason)
}

func (s *fakeConsumerState) StateGet(_ context.Context, key string) ([]byte, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reads++
	if err := s.getErr[key]; err != nil {
		return nil, false, err
	}
	value, ok := s.values[key]
	return value, ok, nil
}

func (s *fakeConsumerState) StatePut(_ context.Context, key string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.putErr[key]; err != nil {
		return err
	}
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

// failRead makes every read of key fail with a store error.
func (s *fakeConsumerState) failRead(key, reason string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.getErr[key] = errors.New(reason)
}

// raise raises the generation as StateIncrement does and returns it. The
// caller holds s.mu.
func (s *fakeConsumerState) raise() uint32 {
	s.generation++
	var counter [4]byte
	binary.BigEndian.PutUint32(counter[:], s.generation)
	s.values[store.GenerationKey()] = counter[:]
	return s.generation
}

// storeList writes asSet's dataset holding prefixes, raises the generation and
// writes the pointer, in the irr plugin's order, and returns the notice the
// irr plugin would send.
func (s *fakeConsumerState) storeList(t *testing.T, asSet string, prefixes ...netip.Prefix) string {
	t.Helper()
	in := store.DatasetInput{
		ASSet: asSet, Server: "whois.radb.net",
		RefreshedAt: time.Unix(1_700_000_000, 0), CheckedAt: time.Unix(1_700_000_600, 0),
	}
	for _, prefix := range prefixes {
		if prefix.Addr().Is4() {
			in.IPv4 = append(in.IPv4, prefix)
		} else {
			in.IPv6 = append(in.IPv6, prefix)
		}
	}
	return s.storeInput(t, in)
}

// storeInput writes the dataset in describes, raises the generation and
// writes the pointer, in the irr plugin's order, and returns the notice.
func (s *fakeConsumerState) storeInput(t *testing.T, in store.DatasetInput) string {
	t.Helper()
	asSet := in.ASSet
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

// clearSet removes asSet's pointer as `clear irr` does and returns the
// removal notice.
func (s *fakeConsumerState) clearSet(t *testing.T, asSet string) string {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.values, store.PointerKey(asSet))
	return noticeOf(t, store.DatasetNotice{ASSet: asSet, Server: "whois.radb.net", Generation: s.raise(), Removed: true})
}

// bind stores the binding of asn to asSet, as the irr plugin does after
// PeeringDB answers.
func (s *fakeConsumerState) bind(t *testing.T, asn uint32, asSet string) {
	t.Helper()
	encoded, err := store.EncodeBinding(store.Binding{ASSet: asSet, PeeringDBURL: "https://www.peeringdb.com", DiscoveredAt: time.Unix(1_700_000_000, 0)})
	if err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.values[store.BindingKey(asn)] = encoded
}

func noticeOf(t *testing.T, notice store.DatasetNotice) string {
	t.Helper()
	payload, err := json.Marshal(notice)
	if err != nil {
		t.Fatal(err)
	}
	return string(payload)
}

var (
	prefixV4 = netip.MustParsePrefix("10.0.0.0/24")
	prefixV6 = netip.MustParsePrefix("2001:db8::/32")
)

func (b *recordingBackend) count() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.applied)
}

// termConfig is a config whose table ze_wan has one term per AS-SET named.
func termConfig(asSets ...string) *irrConfig {
	cfg := &irrConfig{}
	for _, name := range asSets {
		cfg.refs = append(cfg.refs, irrRef{Name: name, IsASSet: true, TableName: "ze_wan"})
	}
	return cfg
}

// startedPlugin builds a plugin over state, configured with cfg and marked
// started, with its applies recorded rather than programmed. No pass has run.
func startedPlugin(t *testing.T, state *fakeConsumerState, cfg *irrConfig) (*irrPlugin, *recordingBackend) {
	t.Helper()
	backend := useRecordingBackend(t)
	plug := &irrPlugin{refs: make(map[string]*refState)}
	plug.reader = newStoreReader(plug, state)
	plug.configure(cfg)
	plug.started = true
	return plug, backend
}

// v4Elements is the element count of the IPv4 set of name in the last apply,
// or -1 when no applied table declares it.
func v4Elements(t *testing.T, backend *recordingBackend, name string) int {
	t.Helper()
	applied, ok := backend.last()
	if !ok {
		t.Fatal("nothing was programmed")
	}
	for _, tbl := range applied {
		if set, found := setsByName(tbl)["irr_v4_"+name]; found {
			return len(set.Elements)
		}
	}
	return -1
}

// VALIDATES: AC-21 -- the handshake configure reads no store and programs
// nothing. Method: deliver a config naming a stored set through the
// OnConfigure callback, then count the store reads and the applies.
// PREVENTS: the handshake applying tables from an empty cache and warning that
// every reference held no data, at every start.
func TestHandshakeConfigureAppliesNothing(t *testing.T) {
	state := newFakeConsumerState()
	state.storeList(t, "AS-TEST", prefixV4)
	backend := useRecordingBackend(t)
	plug := &irrPlugin{refs: make(map[string]*refState)}
	plug.reader = newStoreReader(plug, state)
	before := state.readCount()

	sections := []sdk.ConfigSection{{
		Root: "firewall",
		Data: `{"firewall":{"table":{"wan":{"family":"inet","chain":{"input":{"term":{"t":{"from":{"source-as-set":"AS-TEST"},"then":{"drop":""}}}}}}}}}`,
	}}
	if err := plug.handshakeConfigure(sections); err != nil {
		t.Fatalf("handshakeConfigure: %v", err)
	}
	if got := state.readCount() - before; got != 0 {
		t.Errorf("the handshake read the store %d times, want none", got)
	}
	if got := backend.count(); got != 0 {
		t.Errorf("the handshake programmed the firewall %d times, want none", got)
	}
	if _, ok := plug.refs["AS-TEST"]; !ok {
		t.Error("the handshake did not record the configured reference")
	}
}

// VALIDATES: AC-2 -- the first pass after the start programs the stored copy,
// both family sets of a term included, and writes the interest key after it.
// PREVENTS: a restart that enforces nothing until a fetch.
func TestStartedReadProgramsStoredSets(t *testing.T) {
	state := newFakeConsumerState()
	state.storeList(t, "AS-TEST", prefixV4)
	plug, backend := startedPlugin(t, state, termConfig("AS-TEST"))

	if err := plug.reader.passNow(context.Background()); err != nil {
		t.Fatalf("pass: %v", err)
	}
	applied, ok := backend.last()
	if !ok || len(applied) != 1 || applied[0].Name != "ze_wan" {
		t.Fatalf("applied %+v, want the ze_wan table the term names", applied)
	}
	sets := setsByName(applied[0])
	if len(sets["irr_v4_AS-TEST"].Elements) != 2 {
		t.Errorf("irr_v4_AS-TEST holds %+v, want the stored prefix", sets["irr_v4_AS-TEST"].Elements)
	}
	if _, ok := sets["irr_v6_AS-TEST"]; !ok {
		t.Error("the term's IPv6 twin names irr_v6_AS-TEST, and the table does not declare it")
	}
	interest, err := store.DecodeInterest(state.values[zefs.KeyIRRInterest.Key(pluginName)])
	if err != nil {
		t.Fatalf("interest key: %v", err)
	}
	if len(interest.ASSets) != 1 || interest.ASSets[0] != "AS-TEST" {
		t.Errorf("interest %+v, want AS-TEST", interest)
	}
	if len(state.emits) != 1 || !strings.Contains(state.emits[0], store.EventInterestChanged) {
		t.Errorf("notices sent %v, want one interest notice", state.emits)
	}
}

// VALIDATES: AC-6 -- a cold start programs no set, still writes the interest
// that makes the irr plugin fetch, and the notice of the stored set programs
// it. PREVENTS: a firewall that waits for a fetch it never asked for.
func TestNoticeProgramsTheNamedSet(t *testing.T) {
	state := newFakeConsumerState()
	plug, backend := startedPlugin(t, state, termConfig("AS-TEST"))
	if err := plug.reader.passNow(context.Background()); err != nil {
		t.Fatalf("cold pass: %v", err)
	}
	if got := v4Elements(t, backend, "AS-TEST"); got != -1 {
		t.Fatalf("a cold start programmed irr_v4_AS-TEST with %d elements, want no set", got)
	}
	if _, ok := state.values[zefs.KeyIRRInterest.Key(pluginName)]; !ok {
		t.Fatal("a cold start wrote no interest key, so the irr plugin is never asked")
	}

	plug.reader.notice(state.storeList(t, "AS-TEST", prefixV4))
	if err := plug.reader.pass(context.Background()); err != nil {
		t.Fatalf("notice pass: %v", err)
	}
	if got := v4Elements(t, backend, "AS-TEST"); got != 2 {
		t.Fatalf("after the notice irr_v4_AS-TEST holds %d elements, want 2", got)
	}
}

// VALIDATES: AC-8 -- a notice two generations past the last one applied means
// a notice was lost, so every reference is read again.
// PREVENTS: a lost notice leaving the firewall on an older copy (R-3).
func TestNoticeGapRereads(t *testing.T) {
	state := newFakeConsumerState()
	state.storeList(t, "AS-ONE", prefixV4)
	state.storeList(t, "AS-TWO", prefixV4)
	plug, backend := startedPlugin(t, state, termConfig("AS-ONE", "AS-TWO"))
	if err := plug.reader.passNow(context.Background()); err != nil {
		t.Fatalf("first pass: %v", err)
	}

	state.storeList(t, "AS-TWO", prefixV4, netip.MustParsePrefix("10.9.0.0/24")) // its notice is lost
	plug.reader.notice(state.storeList(t, "AS-ONE", prefixV4))
	if err := plug.reader.pass(context.Background()); err != nil {
		t.Fatalf("gap pass: %v", err)
	}
	if got := v4Elements(t, backend, "AS-TWO"); got != 4 {
		t.Fatalf("after the gap irr_v4_AS-TWO holds %d elements, want the 4 of the copy whose notice was lost", got)
	}
}

// VALIDATES: AC-16 and AC-22 -- a removed set drops its list, and a stored
// value that cannot be used keeps the list enforced and is reported with its
// fix. PREVENTS: an unreadable value read as an empty list, or a cleared set
// kept.
func TestRemovedSetDropsAndUnreadableKeeps(t *testing.T) {
	state := newFakeConsumerState()
	state.storeList(t, "AS-GONE", prefixV4)
	state.storeList(t, "AS-BAD", prefixV4)
	plug, backend := startedPlugin(t, state, termConfig("AS-GONE", "AS-BAD"))
	if err := plug.reader.passNow(context.Background()); err != nil {
		t.Fatalf("first pass: %v", err)
	}

	state.failRead(store.PointerKey("AS-BAD"), "disk read failed")
	plug.reader.notice(state.clearSet(t, "AS-GONE"))
	plug.reader.notice(state.storeList(t, "AS-BAD", prefixV4))
	if err := plug.reader.pass(context.Background()); err != nil {
		t.Fatalf("pass: %v", err)
	}
	if got := v4Elements(t, backend, "AS-GONE"); got != -1 {
		t.Errorf("the cleared AS-GONE still programs %d elements", got)
	}
	if got := v4Elements(t, backend, "AS-BAD"); got != 2 {
		t.Errorf("the unreadable AS-BAD programs %d elements, want the 2 it enforced", got)
	}
	findings, err := plug.answerDoctorCheck(storedCopyCheckName)
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) == 0 || findings[0].Code != codeIRRStoredCopyUnreadable || findings[0].Fix != "update irr as-set AS-BAD" {
		t.Errorf("doctor findings %+v, want %s for AS-BAD with its fix", findings, codeIRRStoredCopyUnreadable)
	}
}

// VALIDATES: AC-11 consumer half -- an ASN reference enforces the set the
// stored binding names, under the ASN's own set name, and its interest names
// the AS number. PREVENTS: an ASN reference keyed as a literal set, which the
// irr plugin never stores for a bound AS number.
func TestASNReferenceFollowsBinding(t *testing.T) {
	state := newFakeConsumerState()
	state.bind(t, 65546, "AS-CUSTOMER")
	state.storeList(t, "AS-CUSTOMER", prefixV4)
	cfg := &irrConfig{refs: []irrRef{{Name: "AS65546", TableName: "ze_wan"}}}
	plug, backend := startedPlugin(t, state, cfg)
	if err := plug.reader.passNow(context.Background()); err != nil {
		t.Fatalf("pass: %v", err)
	}
	if got := v4Elements(t, backend, "AS65546"); got != 2 {
		t.Fatalf("irr_v4_AS65546 holds %d elements, want the 2 of the bound AS-CUSTOMER", got)
	}
	interest, err := store.DecodeInterest(state.values[zefs.KeyIRRInterest.Key(pluginName)])
	if err != nil {
		t.Fatal(err)
	}
	if len(interest.ASNs) != 1 || interest.ASNs[0] != 65546 || len(interest.ASSets) != 0 {
		t.Errorf("interest %+v, want the AS number 65546 alone", interest)
	}
}

// VALIDATES: AC-4 and the reload half of AC-1 -- a reload keeping the
// reference keeps the copy it enforces and programs it again, reading no
// dataset. PREVENTS: a reload that rebuilt the plugin's lists empty and held
// the operator's table back while the commit reported success.
func TestReloadKeepsTheEnforcedCopy(t *testing.T) {
	state := newFakeConsumerState()
	state.storeList(t, "AS-TEST", prefixV4)
	plug, backend := startedPlugin(t, state, termConfig("AS-TEST"))
	if err := plug.reader.passNow(context.Background()); err != nil {
		t.Fatalf("first pass: %v", err)
	}
	pointer, err := store.ReadPointer(context.Background(), state, "AS-TEST")
	if err != nil {
		t.Fatalf("ReadPointer: %v", err)
	}
	datasetKey := pointer.DatasetKey("AS-TEST")
	state.failRead(datasetKey, "the dataset must not be read again")
	applies := backend.count()

	plug.configure(termConfig("AS-TEST"))
	if err := plug.reader.passNow(context.Background()); err != nil {
		t.Fatalf("reload pass: %v", err)
	}
	if backend.count() != applies+1 {
		t.Fatalf("the reload applied %d times, want once: a commit's tables must reach the firewall", backend.count()-applies)
	}
	if got := v4Elements(t, backend, "AS-TEST"); got != 2 {
		t.Fatalf("after the reload irr_v4_AS-TEST holds %d elements, want 2", got)
	}
	if reason := plug.refs["AS-TEST"].readErr; reason != "" {
		t.Errorf("the reload read the dataset its pointer already named: %s", reason)
	}
}

// VALIDATES: AC-12 firewall half -- a stored list longer than a firewall set
// holds is refused whole, the error names the count, and the set already
// programmed stays. PREVENTS: programming part of the operator's rule and
// reporting success.
func TestOversizedStoredListKeepsTheProgrammedSet(t *testing.T) {
	withPrefixBound(t, 2)
	state := newFakeConsumerState()
	state.storeList(t, "AS-TEST", netip.MustParsePrefix("192.0.2.0/24"))
	plug, backend := startedPlugin(t, state, termConfig("AS-TEST"))
	if err := plug.reader.passNow(context.Background()); err != nil {
		t.Fatalf("first pass: %v", err)
	}

	plug.reader.notice(state.storeList(t, "AS-TEST", prefixRun(3)...))
	err := plug.reader.pass(context.Background())
	if err == nil {
		t.Fatal("a list longer than a firewall set holds must fail the apply, not program a part of it")
	}
	if !strings.Contains(err.Error(), "3 IPv4 prefixes") {
		t.Errorf("the error must name the count and the bound, got %q", err)
	}
	applied, _ := backend.last()
	v4 := setsByName(applied[0])["irr_v4_AS-TEST"]
	if len(v4.Elements) != 2 || v4.Elements[0].Value != "192.0.2.0" {
		t.Fatalf("the refused list replaced the programmed set: %+v", v4.Elements)
	}
}

// VALIDATES: each family is bounded on its own, so a full IPv4 list leaves the
// IPv6 set complete. PREVENTS: an IPv6 set with no elements, after which the
// interface whitelist's drop term takes every IPv6 packet on the port.
func TestStoredIPv6KeptWhenIPv4FillsTheBound(t *testing.T) {
	withPrefixBound(t, 2)
	state := newFakeConsumerState()
	state.storeList(t, "AS-TEST", netip.MustParsePrefix("10.0.0.0/24"), netip.MustParsePrefix("10.1.0.0/24"), prefixV6)
	plug, backend := startedPlugin(t, state, termConfig("AS-TEST"))
	if err := plug.reader.passNow(context.Background()); err != nil {
		t.Fatalf("pass: %v", err)
	}
	applied, _ := backend.last()
	sets := setsByName(applied[0])
	if got := len(sets["irr_v4_AS-TEST"].Elements); got != 4 {
		t.Errorf("the IPv4 set holds %d elements, want 4 for its two prefixes", got)
	}
	v6 := sets["irr_v6_AS-TEST"]
	if len(v6.Elements) != 2 || v6.Elements[0].Value != "2001:db8::" {
		t.Fatalf("the IPv6 set holds %+v, want the stored prefix", v6.Elements)
	}
}

// VALIDATES: AC-23 -- a commit naming a reference with nothing stored is
// refused with the update irr command that fetches it; once stored, the same
// reference verifies. PREVENTS: a commit that enforces nothing, or a refusal
// naming the deleted `update firewall irr`.
func TestVerifyRefusesNoPointer(t *testing.T) {
	tests := []struct {
		name string
		ref  irrRef
		fix  string
	}{
		{"as-set", irrRef{Name: "AS-MISSING", IsASSet: true, TableName: "ze_wan"}, "update irr as-set AS-MISSING"},
		{"asn", irrRef{Name: "AS99999", TableName: "ze_wan"}, "update irr asn 99999"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := newFakeConsumerState()
			err := verifyStored(context.Background(), state, []irrRef{tt.ref})
			if err == nil {
				t.Fatal("a reference with nothing stored must be refused")
			}
			if !strings.HasSuffix(err.Error(), "fix: "+tt.fix) {
				t.Errorf("refusal %q does not end with the fix %q", err, tt.fix)
			}
			if !strings.Contains(err.Error(), tt.ref.Name) {
				t.Errorf("refusal %q does not name %s", err, tt.ref.Name)
			}
			if tt.ref.IsASSet {
				state.storeList(t, tt.ref.Name, prefixV4)
			} else {
				state.bind(t, 99999, "AS-BOUND")
				state.storeList(t, "AS-BOUND", prefixV4)
			}
			if err := verifyStored(context.Background(), state, []irrRef{tt.ref}); err != nil {
				t.Errorf("after the update the same reference must verify, got %v", err)
			}
		})
	}
}

// VALIDATES: config verify refuses a reference whose stored list holds no
// prefixes, naming the reference. PREVENTS: a zero-prefix list read as a valid
// answer at commit time.
func TestVerifyRefusesAnEmptyStoredList(t *testing.T) {
	state := newFakeConsumerState()
	state.storeList(t, "AS-EMPTY")
	err := verifyStored(context.Background(), state, []irrRef{{Name: "AS-EMPTY", IsASSet: true}})
	if err == nil {
		t.Fatal("a stored list with no prefixes must not verify")
	}
	if !strings.Contains(err.Error(), "AS-EMPTY") {
		t.Errorf("error %q does not name the AS-SET the operator must fetch", err)
	}
}

// VALIDATES: a commit naming an IRR reference from a table term, not only from
// an interface binding, is refused when nothing is stored for it, with the
// update irr command that fetches it. (Replaces TestVerifyRejectsUncachedTableTerm.)
// METHOD: drive the two functions OnConfigVerify calls, parseIRRConfig then
// verifyStored, over a table-term config and an empty store.
// PREVENTS: the actionable refusal staying unreachable for the table form: the
// firewall component refused such a term with `match references unknown set
// "irr_v4_AS99999"`, which names neither the reference nor the command.
func TestVerifyRefusesAnUnstoredTableTerm(t *testing.T) {
	tests := []struct {
		name  string
		leaf  string
		value string
		ref   string
		fix   string
	}{
		{name: "asn", leaf: "source-asn", value: "99999", ref: "AS99999", fix: "update irr asn 99999"},
		{name: "as-set", leaf: "destination-as-set", value: "AS-MISSING", ref: "AS-MISSING", fix: "update irr as-set AS-MISSING"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sections := []sdk.ConfigSection{{
				Root: "firewall",
				Data: `{"firewall":{"table":{"wan":{"family":"inet","chain":{"input":{"term":{"t":{"from":{"` +
					tt.leaf + `":"` + tt.value + `"},"then":{"drop":""}}}}}}}}}`,
			}}
			refs := parseIRRConfig(sections).allRefs()
			if len(refs) != 1 {
				t.Fatalf("allRefs must find the table term reference, got %v", refs)
			}
			if refs[0].TableName != "ze_wan" {
				t.Errorf("reference table = %q, want ze_wan", refs[0].TableName)
			}
			err := verifyStored(context.Background(), newFakeConsumerState(), refs)
			if err == nil {
				t.Fatal("a table-term reference with nothing stored must be refused")
			}
			if !strings.HasSuffix(err.Error(), "fix: "+tt.fix) {
				t.Errorf("refusal %q does not end with the fix %q", err, tt.fix)
			}
			if !strings.Contains(err.Error(), tt.ref) {
				t.Errorf("refusal %q does not name the reference %s", err, tt.ref)
			}
		})
	}
}

// VALIDATES: a panic while programming the tables costs that apply and nothing
// more: the pass answers an error, the copy it read stays recorded, the doctor
// reports the tables not programmed, and the next notice pass still reads and
// applies. (Replaces TestRefreshAllSurvivesPanic, whose refresh loop is gone;
// the fetch half is internal/plugins/irr TestFetchPanicKeepsTheWorker.)
// METHOD: program through a backend whose Apply panics, run a commit pass and
// a notice pass, and read the plugin's state and doctor answer after each.
// PREVENTS: the reader goroutine dying on the panic, which the SDK's dispatch
// recovery does not cover, and with it the plugin process: the registry then
// holds back every table naming an IRR set.
func TestReaderPassSurvivesABackendPanic(t *testing.T) {
	state := newFakeConsumerState()
	state.storeList(t, "AS-TEST", prefixV4)
	usePanickingBackend(t)
	plug := &irrPlugin{refs: make(map[string]*refState)}
	plug.reader = newStoreReader(plug, state)
	plug.configure(termConfig("AS-TEST"))
	plug.started = true

	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("a panic escaped the reader's pass and would have ended the plugin: %v", recovered)
		}
	}()
	if err := plug.reader.passNow(context.Background()); err == nil {
		t.Fatal("a pass whose apply panicked answered success")
	}
	if st := plug.refs["AS-TEST"]; st.copy == nil || len(st.copy.entry.IPv4) != 1 {
		t.Fatalf("the copy read before the panic was lost: %+v", st.copy)
	}
	if plug.applyErr.Load() == nil {
		t.Fatal("the panicked apply left no apply failure for the doctor")
	}
	findings, err := plug.answerDoctorCheck(storedCopyCheckName)
	if err != nil {
		t.Fatalf("doctor: %v", err)
	}
	if !hasFinding(findings, codeIRRNotProgrammed) {
		t.Errorf("doctor findings %+v, want %s", findings, codeIRRNotProgrammed)
	}

	plug.reader.notice(state.storeList(t, "AS-TEST", prefixV4, netip.MustParsePrefix("10.9.0.0/24")))
	if err := plug.reader.pass(context.Background()); err == nil {
		t.Fatal("the notice pass through the panicking backend answered success")
	}
	if st := plug.refs["AS-TEST"]; st.copy == nil || len(st.copy.entry.IPv4) != 2 {
		t.Fatalf("the notice pass after the panic did not read the new copy: %+v", st.copy)
	}
}

// hasFinding reports whether findings carry code, with a fix.
func hasFinding(findings []sdk.DoctorCheckDiagnostic, code string) bool {
	for _, finding := range findings {
		if finding.Code == code && finding.Fix != "" {
			return true
		}
	}
	return false
}

// VALIDATES: show firewall irr reports, per reference, the stored copy the
// firewall enforces: a fresh copy as ok with its age, server, generation and
// count; a stale copy as stale since when, still counted; an unstored
// reference as missing; an unreadable one with its error, escaped so the
// answer stays JSON. (Replaces TestShowIRRReportsStaleEntry.)
// METHOD: store copies through the fake store, run a pass and a notice pass,
// and decode the show answer after each.
// PREVENTS: an operator reading "status ok" while the filter runs on prefixes
// the IRR has stopped confirming, or a store error that breaks the answer.
func TestShowReportsTheStoredCopy(t *testing.T) {
	const reason = `disk said "no" at C:\store`
	state := newFakeConsumerState()
	state.storeList(t, "AS-TEST", prefixV4)
	state.storeList(t, "AS-BAD", prefixV4)
	state.failRead(store.PointerKey("AS-BAD"), reason)
	plug, _ := startedPlugin(t, state, termConfig("AS-TEST", "AS-NONE", "AS-BAD"))
	if err := plug.reader.passNow(context.Background()); err != nil {
		t.Fatalf("pass: %v", err)
	}

	entries := entriesByName(t, plug)
	fresh := entries["AS-TEST"]
	if fresh["status"] != "ok" {
		t.Errorf("fresh status = %v, want ok", fresh["status"])
	}
	if _, reported := fresh["stale-since"]; reported {
		t.Error("a fresh copy must not report a stale-since time")
	}
	if _, reported := fresh["data-age-seconds"]; !reported {
		t.Error("show must report the age of the data being enforced")
	}
	if count, _ := fresh["ipv4-count"].(float64); count != 1 {
		t.Errorf("ipv4-count = %v, want 1", fresh["ipv4-count"])
	}
	if fresh["server"] != "whois.radb.net" {
		t.Errorf("server = %v, want whois.radb.net", fresh["server"])
	}
	if generation, _ := fresh["generation"].(float64); generation != 1 {
		t.Errorf("generation = %v, want 1, the generation AS-TEST was stored under", fresh["generation"])
	}
	if entries["AS-NONE"]["status"] != "missing" {
		t.Errorf("unstored status = %v, want missing", entries["AS-NONE"]["status"])
	}
	bad := entries["AS-BAD"]
	if bad["status"] != "unreadable" || bad["error"] != reason {
		t.Errorf("unreadable entry %+v, want status unreadable and error %q", bad, reason)
	}

	plug.reader.notice(state.storeInput(t, store.DatasetInput{
		ASSet: "AS-TEST", Server: "whois.radb.net", IPv4: []netip.Prefix{prefixV4},
		RefreshedAt: time.Unix(1_700_000_000, 0), CheckedAt: time.Unix(1_700_003_600, 0),
		StaleSince: time.Unix(1_700_003_600, 0),
	}))
	if err := plug.reader.pass(context.Background()); err != nil {
		t.Fatalf("notice pass: %v", err)
	}
	stale := entriesByName(t, plug)["AS-TEST"]
	if stale["status"] != "stale" {
		t.Errorf("status = %v, want stale", stale["status"])
	}
	if _, reported := stale["stale-since"]; !reported {
		t.Error("a stale copy must report since when")
	}
	if count, _ := stale["ipv4-count"].(float64); count != 1 {
		t.Errorf("ipv4-count = %v, want 1: the stored prefixes stay enforced", stale["ipv4-count"])
	}
}

// entriesByName runs show firewall irr and indexes its entries by name.
func entriesByName(t *testing.T, plug *irrPlugin) map[string]map[string]any {
	t.Helper()
	byName := make(map[string]map[string]any)
	for _, entry := range showEntries(t, plug) {
		name, _ := entry["name"].(string)
		byName[name] = entry
	}
	return byName
}

// VALIDATES: the post-config doctor check judges each reference's stored copy:
// nothing stored and an empty list are no data, a stale copy is a warning, a
// list too long for a set is oversized, and a fresh copy is no finding. Each
// finding names the command that repairs it.
// METHOD: run storedCopyDiagnostics over the fake store for each case.
// PREVENTS: a firewall filter running on no prefixes, or on prefixes nobody
// confirmed, with no surface an operator would look at.
func TestStoredCopyDiagnostics(t *testing.T) {
	ref := irrRef{Name: "AS-TEST", IsASSet: true, TableName: "ze_wan"}
	stale := store.DatasetInput{
		ASSet: "AS-TEST", Server: "whois.radb.net", IPv4: []netip.Prefix{prefixV4},
		RefreshedAt: time.Unix(1_700_000_000, 0), CheckedAt: time.Unix(1_700_003_600, 0),
		StaleSince: time.Unix(1_700_003_600, 0),
	}
	tests := []struct {
		name     string
		store    func(t *testing.T, state *fakeConsumerState)
		bound    int
		code     string
		severity diagnostic.Severity
		fix      string
	}{
		{name: "not stored", store: func(*testing.T, *fakeConsumerState) {},
			code: codeIRRNoData, severity: diagnostic.SeverityError, fix: "update irr as-set AS-TEST"},
		{name: "empty list", store: func(t *testing.T, s *fakeConsumerState) { s.storeList(t, "AS-TEST") },
			code: codeIRRNoData, severity: diagnostic.SeverityError, fix: "update irr as-set AS-TEST"},
		{name: "stale", store: func(t *testing.T, s *fakeConsumerState) { s.storeInput(t, stale) },
			code: codeIRRStaleData, severity: diagnostic.SeverityWarning, fix: "update irr as-set AS-TEST"},
		{name: "oversized", bound: 2, store: func(t *testing.T, s *fakeConsumerState) { s.storeList(t, "AS-TEST", prefixRun(3)...) },
			code: codeIRROversizedData, severity: diagnostic.SeverityError, fix: "clear irr as-set AS-TEST"},
		{name: "fresh", store: func(t *testing.T, s *fakeConsumerState) { s.storeList(t, "AS-TEST", prefixV4) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.bound > 0 {
				withPrefixBound(t, tt.bound)
			}
			state := newFakeConsumerState()
			tt.store(t, state)
			diags := storedCopyDiagnostics(state, []irrRef{ref}, time.Unix(1_700_007_200, 0))
			if tt.code == "" {
				if len(diags) != 0 {
					t.Fatalf("a fresh stored copy reported %+v, want nothing", diags)
				}
				return
			}
			if len(diags) != 1 {
				t.Fatalf("diagnostics %+v, want one %s", diags, tt.code)
			}
			if diags[0].Code != tt.code || diags[0].Severity != tt.severity || diags[0].Fix != tt.fix {
				t.Errorf("diagnostic %+v, want code %s severity %v fix %q", diags[0], tt.code, tt.severity, tt.fix)
			}
		})
	}
}

// VALIDATES: the runtime check firewall-irr-stored-copies reports each problem
// only the running plugin knows, under its own code with a fix: a stored value
// it could not use, a generation it has not applied, an interest key the store
// refused, tables the firewall refused. A healthy plugin reports nothing.
// METHOD: drive one pass per case over the fake store, then ask the plugin's
// doctor callback.
// PREVENTS: a firewall enforcing an older list, or none, with no doctor
// finding to say why.
func TestStoredCopiesRuntimeFindings(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, state *fakeConsumerState)
		after func(t *testing.T, state *fakeConsumerState)
		code  string
	}{
		{name: "healthy", setup: func(t *testing.T, s *fakeConsumerState) { s.storeList(t, "AS-TEST", prefixV4) }},
		{name: "unreadable", code: codeIRRStoredCopyUnreadable, setup: func(t *testing.T, s *fakeConsumerState) {
			s.storeList(t, "AS-TEST", prefixV4)
			s.failRead(store.PointerKey("AS-TEST"), "disk read failed")
		}},
		{name: "behind", code: codeIRRStoredCopyBehind,
			setup: func(t *testing.T, s *fakeConsumerState) { s.storeList(t, "AS-TEST", prefixV4) },
			// Stored after the pass, and its notice never delivered.
			after: func(t *testing.T, s *fakeConsumerState) { s.storeList(t, "AS-OTHER", prefixV4) }},
		{name: "interest unsaved", code: codeIRRInterestUnsaved, setup: func(t *testing.T, s *fakeConsumerState) {
			s.storeList(t, "AS-TEST", prefixV4)
			s.failWrite(zefs.KeyIRRInterest.Key(pluginName), "store full")
		}},
		{name: "not programmed", code: codeIRRNotProgrammed, setup: func(t *testing.T, s *fakeConsumerState) {
			withPrefixBound(t, 2)
			s.storeList(t, "AS-TEST", prefixRun(3)...)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state := newFakeConsumerState()
			tt.setup(t, state)
			plug, _ := startedPlugin(t, state, termConfig("AS-TEST"))
			_ = plug.reader.passNow(context.Background()) //nolint:errcheck // the failing cases fail the pass on purpose; the doctor answer is the assertion
			if tt.after != nil {
				tt.after(t, state)
			}
			findings, err := plug.answerDoctorCheck(storedCopyCheckName)
			if err != nil {
				t.Fatalf("doctor: %v", err)
			}
			if tt.code == "" {
				if len(findings) != 0 {
					t.Fatalf("a healthy plugin reported %+v, want nothing", findings)
				}
				return
			}
			if len(findings) != 1 || !hasFinding(findings, tt.code) {
				t.Fatalf("findings %+v, want exactly one %s with a fix", findings, tt.code)
			}
		})
	}
	plug, _ := startedPlugin(t, newFakeConsumerState(), termConfig())
	if _, err := plug.answerDoctorCheck("firewall-irr-unknown"); err == nil {
		t.Error("the doctor callback answered a check the plugin never declared")
	}
}
