package irr

import (
	"context"
	"encoding/binary"
	"errors"
	"net/netip"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/resolve/irr/store"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// fakeState is the daemon store as the SDK state RPCs show it, with a log of
// every write in order and per-key write failures: failPut refuses one key,
// failPutPrefix every key under a prefix (a dataset key ends in a digest the
// test cannot name in advance), and failRemove one removal. Safe for
// concurrent use.
type fakeState struct {
	mu            sync.Mutex
	values        map[string][]byte
	ops           []string
	failPut       map[string]error
	failPutPrefix map[string]error
	failRemove    map[string]error
}

func newFakeState() *fakeState {
	return &fakeState{
		values: make(map[string][]byte), failPut: make(map[string]error),
		failPutPrefix: make(map[string]error), failRemove: make(map[string]error),
	}
}

func (s *fakeState) StateGet(_ context.Context, key string) ([]byte, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, ok := s.values[key]
	return slices.Clone(value), ok, nil
}

func (s *fakeState) StatePut(_ context.Context, key string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.failPut[key]; err != nil {
		s.ops = append(s.ops, "put-refused "+key)
		return err
	}
	for prefix, err := range s.failPutPrefix {
		if strings.HasPrefix(key, prefix) {
			s.ops = append(s.ops, "put-refused "+key)
			return err
		}
	}
	s.ops = append(s.ops, "put "+key)
	s.values[key] = slices.Clone(data)
	return nil
}

func (s *fakeState) StateRemove(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.failRemove[key]; err != nil {
		s.ops = append(s.ops, "remove-refused "+key)
		return err
	}
	s.ops = append(s.ops, "remove "+key)
	delete(s.values, key)
	return nil
}

func (s *fakeState) StateList(_ context.Context, prefix string) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var keys []string
	for key := range s.values {
		if strings.HasPrefix(key, prefix) {
			keys = append(keys, key)
		}
	}
	slices.Sort(keys)
	return keys, nil
}

func (s *fakeState) StateIncrement(_ context.Context, key string) (uint32, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var value uint32
	if stored, ok := s.values[key]; ok && len(stored) == 4 {
		value = binary.BigEndian.Uint32(stored)
	}
	value++
	s.values[key] = binary.BigEndian.AppendUint32(nil, value)
	s.ops = append(s.ops, "increment "+key)
	return value, nil
}

func (s *fakeState) has(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.values[key]
	return ok
}

// keysUnder lists the stored keys under prefix, in order.
func (s *fakeState) keysUnder(prefix string) []string {
	keys, _ := s.StateList(context.Background(), prefix)
	return keys
}

func (s *fakeState) value(key string) []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.values[key])
}

func (s *fakeState) log() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.ops)
}

// fakeNotices records every notice the owner emits.
type fakeNotices struct {
	mu   sync.Mutex
	sent []string
}

func (n *fakeNotices) EmitEvent(_ context.Context, namespace, eventType, _, _, event string) (int, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	n.sent = append(n.sent, namespace+"/"+eventType+" "+event)
	return 1, nil
}

func (n *fakeNotices) count() int {
	n.mu.Lock()
	defer n.mu.Unlock()
	return len(n.sent)
}

// fakeAnswer is what the fake server answers for one AS-SET.
type fakeAnswer struct {
	ipv4 []netip.Prefix
	err  error
}

// fakeResolver answers from a table instead of a socket and records every
// query. Its refresh keeps the seeded copy on an error or an empty answer, as
// PrefixStore does. Called from the worker only.
type fakeResolver struct {
	clock   func() time.Time
	answers map[string]fakeAnswer
	sets    map[uint32]string
	pdbErr  error
	seeded  map[string]store.CachedEntry
	queries []string
	asked   []uint32
	servers []string
	// panicOn is an AS-SET whose fetch panics, as a parser defect on a
	// malformed answer would.
	panicOn string
}

func newFakeResolver(clock func() time.Time) *fakeResolver {
	return &fakeResolver{
		clock: clock, answers: make(map[string]fakeAnswer), sets: make(map[uint32]string),
		seeded: make(map[string]store.CachedEntry),
	}
}

func (r *fakeResolver) refresh(_ context.Context, asSet string) (*store.CachedEntry, error) {
	r.queries = append(r.queries, asSet)
	if asSet == r.panicOn {
		panic("malformed answer for " + asSet)
	}
	answer, ok := r.answers[asSet]
	kept, had := r.seeded[asSet]
	if !ok {
		answer.err = errors.New("connection refused")
	}
	if answer.err != nil || len(answer.ipv4) == 0 {
		err := answer.err
		if err == nil {
			err = store.ErrNoPrefixes
		}
		if !had {
			return &store.CachedEntry{Name: asSet, ASSet: asSet}, err
		}
		kept.StaleSince = r.clock()
		r.seeded[asSet] = kept
		return &kept, err
	}
	entry := store.CachedEntry{Name: asSet, ASSet: asSet, IPv4: answer.ipv4, RefreshedAt: r.clock()}
	r.seeded[asSet] = entry
	return &entry, nil
}

func (r *fakeResolver) seed(entry store.CachedEntry) error {
	r.seeded[entry.Name] = entry
	return nil
}

func (r *fakeResolver) forget(asSet string) { delete(r.seeded, asSet) }

func (r *fakeResolver) discover(_ context.Context, asn uint32) (string, error) {
	r.asked = append(r.asked, asn)
	if r.pdbErr != nil {
		return "", r.pdbErr
	}
	if set, ok := r.sets[asn]; ok {
		return set, nil
	}
	return literalASSet(asn), nil
}

func (r *fakeResolver) useClients(cfg ownerConfig) { r.servers = append(r.servers, cfg.server) }

func (r *fakeResolver) clearCache(rpc.ResolverCacheClearInput) (int, error) { return 0, nil }

// testClock is a settable clock.
type testClock struct{ at time.Time }

func (c *testClock) now() time.Time { return c.at }

// ownerHarness drives an owner's worker steps synchronously: no goroutine
// runs, so every assertion reads a settled state.
type ownerHarness struct {
	t        *testing.T
	state    *fakeState
	notices  *fakeNotices
	clock    *testClock
	resolver *fakeResolver
	owner    *owner
}

func newHarness(t *testing.T, state *fakeState, cfg ownerConfig) *ownerHarness {
	t.Helper()
	clock := &testClock{at: time.Date(2026, 11, 15, 10, 0, 0, 0, time.UTC)}
	h := &ownerHarness{t: t, state: state, notices: &fakeNotices{}, clock: clock, resolver: newFakeResolver(clock.now)}
	h.owner = newOwner(state, h.notices, func(ownerConfig) resolver { return h.resolver })
	h.owner.clock = clock.now
	if err := h.owner.configure(cfg); err != nil {
		t.Fatal(err)
	}
	return h
}

// boot runs what the worker runs before its loop, then one pass.
func (h *ownerHarness) boot() {
	ctx := context.Background()
	h.owner.absorb(ctx)
	h.owner.load(ctx)
	h.owner.publishView()
	h.pass()
}

// pass runs one worker pass: hand-offs, then every due fetch.
func (h *ownerHarness) pass() {
	ctx := context.Background()
	h.owner.absorb(ctx)
	h.owner.fetchDue(ctx)
	h.owner.publishView()
}

// writeInterest stores consumer's interest as the consumer would.
func (h *ownerHarness) writeInterest(consumer string, in store.Interest) {
	h.t.Helper()
	value, err := store.EncodeInterest(in)
	if err != nil {
		h.t.Fatal(err)
	}
	if err := h.state.StatePut(context.Background(), store.InterestKey(consumer), value); err != nil {
		h.t.Fatal(err)
	}
}

// pointer reads the stored pointer of asSet.
func (h *ownerHarness) pointer(asSet string) (store.Pointer, bool) {
	h.t.Helper()
	value, found, _ := h.state.StateGet(context.Background(), store.PointerKey(asSet))
	if !found {
		return store.Pointer{}, false
	}
	pointer, err := store.DecodePointer(value)
	if err != nil {
		h.t.Fatalf("stored pointer of %s: %v", asSet, err)
	}
	return pointer, true
}

// dataset reads and decodes the dataset the pointer of asSet names.
func (h *ownerHarness) dataset(asSet string) store.Dataset {
	h.t.Helper()
	pointer, ok := h.pointer(asSet)
	if !ok {
		h.t.Fatalf("no pointer for %s", asSet)
	}
	key := pointer.DatasetKey(asSet)
	dataset, err := store.DecodeDataset(key, h.state.value(key))
	if err != nil {
		h.t.Fatalf("dataset %s: %v", key, err)
	}
	return dataset
}

func prefixes(texts ...string) []netip.Prefix {
	out := make([]netip.Prefix, 0, len(texts))
	for _, text := range texts {
		out = append(out, netip.MustParsePrefix(text))
	}
	return out
}
