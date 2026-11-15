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

// storeSet writes asSet's dataset and a pointer naming it, in the owner's
// order, and returns the pointer.
func storeSet(t *testing.T, s *consumerState, asSet string, generation uint32, prefixes ...string) Pointer {
	t.Helper()
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
	pointer := storeSet(t, s, "AS-TEST", 3, "10.0.0.0/24", "10.0.1.0/24")
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
	pointer := storeSet(t, s, "AS-TEST", 3, "10.0.0.0/24")
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
	if len(s.calls) != 2 || s.calls[0] != "put "+InterestKey("bgp-filter-irr") {
		t.Fatalf("calls = %q, want the put first", s.calls)
	}
	if s.calls[1] != `emit irr/interest-changed {"consumer":"bgp-filter-irr"}` {
		t.Fatalf("notice = %q", s.calls[1])
	}
	in, err := DecodeInterest(s.values[InterestKey("bgp-filter-irr")])
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
