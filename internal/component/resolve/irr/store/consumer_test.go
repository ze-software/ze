// Design: docs/architecture/irr-owner.md -- the consumer reader contract
package store

import (
	"context"
	"encoding/hex"
	"errors"
	"net/netip"
	"testing"
	"time"
)

// consumerState is an in-memory state store recording the order of writes and
// notices, so a test can tell "nothing stored" from "stored and unusable".
type consumerState struct {
	values map[string][]byte
	calls  []string
}

func newConsumerState() *consumerState {
	return &consumerState{values: make(map[string][]byte)}
}

func (s *consumerState) StateGet(_ context.Context, key string) ([]byte, bool, error) {
	value, ok := s.values[key]
	return value, ok, nil
}

func (s *consumerState) StatePut(_ context.Context, key string, data []byte) error {
	s.calls = append(s.calls, "put "+key)
	s.values[key] = data
	return nil
}

func (s *consumerState) EmitEvent(_ context.Context, namespace, eventType, _, _, event string) (int, error) {
	s.calls = append(s.calls, "emit "+namespace+"/"+eventType+" "+event)
	return 1, nil
}

// storeSet writes AS-TEST's dataset and a pointer naming it, in the owner's
// order, and returns the pointer.
func storeSet(t *testing.T, s *consumerState, generation uint32, prefixes ...string) Pointer {
	t.Helper()
	const asSet = "AS-TEST"
	in := DatasetInput{ASSet: asSet, Server: "whois.radb.net", RefreshedAt: time.Unix(1_700_000_000, 0), CheckedAt: time.Unix(1_700_000_600, 0)}
	for _, text := range prefixes {
		in.IPv4 = append(in.IPv4, netip.MustParsePrefix(text))
	}
	dataset := mustDataset(t, in)
	value := encodeDataset(t, dataset)
	digest := DatasetDigest(value)
	pointer := Pointer{Server: dataset.Server(), Generation: generation, Digest: hex.EncodeToString(digest[:])}
	s.values[pointer.DatasetKey(asSet)] = value
	encoded, err := EncodePointer(pointer)
	if err != nil {
		t.Fatal(err)
	}
	s.values[PointerKey(asSet)] = encoded
	return pointer
}

// VALIDATES: a consumer reads the pointer, then the dataset it names, and gets
// the stored prefixes and times back.
// METHOD: store a set as the owner does, ReadSet it.
func TestReadSetReadsThePointedDataset(t *testing.T) {
	s := newConsumerState()
	pointer := storeSet(t, s, 3, "10.0.0.0/24", "10.0.1.0/24")
	got, err := ReadSet(context.Background(), s, "AS-TEST")
	if err != nil {
		t.Fatalf("ReadSet: %v", err)
	}
	if got.Pointer != pointer {
		t.Fatalf("pointer = %+v, want %+v", got.Pointer, pointer)
	}
	if len(got.Dataset.IPv4()) != 2 || got.Dataset.CheckedAt().Unix() != 1_700_000_600 {
		t.Fatalf("dataset = %v checked-at %v", got.Dataset.IPv4(), got.Dataset.CheckedAt())
	}
}

// VALIDATES: no pointer is "nothing stored", the one answer that makes a
// consumer drop its list.
func TestReadSetNothingStored(t *testing.T) {
	_, err := ReadSet(context.Background(), newConsumerState(), "AS-TEST")
	if !errors.Is(err, ErrNotStored) {
		t.Fatalf("err = %v, want ErrNotStored", err)
	}
}

// VALIDATES: a pointer whose dataset is missing or not the one it names is
// unusable, never "nothing stored", so the consumer keeps what it enforces
// (AC-22).
// METHOD: point at another digest and store the old value under the key that
// pointer names; then remove the dataset.
func TestReadSetRefusesADatasetThePointerDoesNotName(t *testing.T) {
	s := newConsumerState()
	pointer := storeSet(t, s, 3, "10.0.0.0/24")
	value := s.values[pointer.DatasetKey("AS-TEST")]
	pointer.Digest = hex.EncodeToString(make([]byte, 32))
	s.values[pointer.DatasetKey("AS-TEST")] = value
	encoded, err := EncodePointer(pointer)
	if err != nil {
		t.Fatal(err)
	}
	s.values[PointerKey("AS-TEST")] = encoded
	_, err = ReadSet(context.Background(), s, "AS-TEST")
	if !errors.Is(err, ErrDatasetNotPointed) || errors.Is(err, ErrNotStored) {
		t.Fatalf("digest mismatch: err = %v, want ErrDatasetNotPointed", err)
	}
	delete(s.values, pointer.DatasetKey("AS-TEST"))
	_, err = ReadSet(context.Background(), s, "AS-TEST")
	if !errors.Is(err, ErrDatasetNotPointed) || errors.Is(err, ErrNotStored) {
		t.Fatalf("missing dataset: err = %v, want ErrDatasetNotPointed", err)
	}
}

// flipOnRead is a consumerState in which the owner flips one set's pointer
// at the moment the reader asks for the dataset the old pointer names: the new
// dataset and its pointer are stored and the old dataset is removed, the
// owner's order (datasets.go writeDataset), before the read is answered.
type flipOnRead struct {
	*consumerState
	oldKey string
	flip   func()
}

func (s *flipOnRead) StateGet(ctx context.Context, key string) ([]byte, bool, error) {
	if key == s.oldKey && s.flip != nil {
		s.flip()
		s.flip = nil
	}
	return s.consumerState.StateGet(ctx, key)
}

// VALIDATES: a reader that read a pointer just before the owner flipped it,
// and asks for the replaced dataset just after the owner removed it, reads the
// new pointer and its dataset; it is not refused (ErrDatasetNotPointed), so a
// firewall commit is not refused and no false stored-copy-unreadable WARN is
// raised at a refresh. A pointer that did not move still refuses a missing
// dataset (TestReadSetRefusesADatasetThePointerDoesNotName).
// METHOD: store generation 3, flip to generation 4 inside the read of the
// generation-3 dataset, ReadSet.
func TestReadSetFollowsAPointerThatMovedUnderTheRead(t *testing.T) {
	s := &flipOnRead{consumerState: newConsumerState()}
	old := storeSet(t, s.consumerState, 3, "10.0.0.0/24")
	s.oldKey = old.DatasetKey("AS-TEST")
	var current Pointer
	s.flip = func() {
		current = storeSet(t, s.consumerState, 4, "10.0.0.0/24", "10.9.0.0/16")
		delete(s.values, old.DatasetKey("AS-TEST"))
	}
	got, err := ReadSet(context.Background(), s, "AS-TEST")
	if err != nil {
		t.Fatalf("ReadSet across a flip: %v", err)
	}
	if got.Pointer != current {
		t.Fatalf("pointer = %+v, want the moved one %+v", got.Pointer, current)
	}
	if len(got.Dataset.IPv4()) != 2 {
		t.Fatalf("dataset = %v, want the two prefixes of generation 4", got.Dataset.IPv4())
	}
}

// VALIDATES: a stored pointer whose server no dataset key can carry is
// refused as malformed when read, so a value written by `ze data write` or
// left by a damaged store never reaches the zefs.KeyEntry.Key panic on ".."
// when a reader forms the dataset key. A server key form, an IPv6 literal with
// its "_port" included, still reads.
// METHOD: store pointers by hand and ReadSet each; a panic fails the test.
func TestReadSetRefusesAPointerServerNoKeyCarries(t *testing.T) {
	digest := hex.EncodeToString(make([]byte, 32))
	for _, server := range []string{"whois..radb.net", "..", "whois.radb.net/x"} {
		s := newConsumerState()
		encoded, err := EncodePointer(Pointer{Server: server, Generation: 1, Digest: digest})
		if err != nil {
			t.Fatal(err)
		}
		s.values[PointerKey("AS-TEST")] = encoded
		_, err = ReadSet(context.Background(), s, "AS-TEST")
		if !errors.Is(err, ErrPointerMalformed) {
			t.Fatalf("server %q: err = %v, want ErrPointerMalformed", server, err)
		}
	}
	encoded, err := EncodePointer(Pointer{Server: "2001:db8::1_4343", Generation: 1, Digest: digest})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecodePointer(encoded); err != nil {
		t.Fatalf("an IPv6 server key form is refused: %v", err)
	}
}

// VALIDATES: the generation counter reads as StateIncrement stores it, 0 when
// never raised, and a value of the wrong size is refused, never read as 0.
func TestReadGeneration(t *testing.T) {
	s := newConsumerState()
	if got, err := ReadGeneration(context.Background(), s); err != nil || got != 0 {
		t.Fatalf("never raised: %d, %v", got, err)
	}
	s.values[GenerationKey()] = []byte{0, 0, 1, 2}
	if got, err := ReadGeneration(context.Background(), s); err != nil || got != 258 {
		t.Fatalf("raised: %d, %v, want 258", got, err)
	}
	s.values[GenerationKey()] = []byte{0, 1, 2}
	if _, err := ReadGeneration(context.Background(), s); !errors.Is(err, ErrGenerationMalformed) {
		t.Fatalf("3 bytes: err = %v, want ErrGenerationMalformed", err)
	}
}

// VALIDATES: an AS number with no binding is "nothing stored"; a stored one
// answers its set in ASSetName form.
func TestReadBinding(t *testing.T) {
	s := newConsumerState()
	if _, err := ReadBinding(context.Background(), s, 65001); !errors.Is(err, ErrNotStored) {
		t.Fatalf("no binding: err = %v, want ErrNotStored", err)
	}
	encoded, err := EncodeBinding(Binding{ASSet: "as-foo", DiscoveredAt: time.Unix(1_700_000_000, 0)})
	if err != nil {
		t.Fatal(err)
	}
	s.values[BindingKey(65001)] = encoded
	got, err := ReadBinding(context.Background(), s, 65001)
	if err != nil || got.ASSet != "AS-FOO" {
		t.Fatalf("binding = %+v, %v, want AS-FOO", got, err)
	}
}

// VALIDATES: the interest key is written before its notice, so the owner,
// which reads the key, never acts on a notice whose key is not there yet.
func TestPublishInterestWritesBeforeItNotifies(t *testing.T) {
	s := newConsumerState()
	err := PublishInterest(context.Background(), s, "bgp-filter-irr", Interest{ASSets: []string{"as-test"}, ASNs: []uint32{65002}})
	if err != nil {
		t.Fatalf("PublishInterest: %v", err)
	}
	if len(s.calls) != 2 || s.calls[0] != "put "+interestKey("bgp-filter-irr") {
		t.Fatalf("calls = %q, want the put first", s.calls)
	}
	if s.calls[1] != `emit irr/interest-changed {"consumer":"bgp-filter-irr"}` {
		t.Fatalf("notice = %q", s.calls[1])
	}
	in, err := DecodeInterest(s.values[interestKey("bgp-filter-irr")])
	if err != nil || len(in.ASSets) != 1 || in.ASSets[0] != "AS-TEST" || in.ASNs[0] != 65002 {
		t.Fatalf("stored interest = %+v, %v", in, err)
	}
}

// VALIDATES: a notice is read in ASSetName form and generation 0 is refused.
func TestDecodeDatasetNotice(t *testing.T) {
	got, err := DecodeDatasetNotice(`{"as-set":"as-test","server":"whois.radb.net","generation":4,"digest":"","removed":true}`)
	if err != nil || got.ASSet != "AS-TEST" || got.Generation != 4 || !got.Removed {
		t.Fatalf("notice = %+v, %v", got, err)
	}
	if _, err := DecodeDatasetNotice(`{"as-set":"AS-TEST","generation":0}`); err == nil {
		t.Fatal("generation 0 accepted")
	}
}
