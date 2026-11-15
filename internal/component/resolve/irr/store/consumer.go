// Design: docs/architecture/irr-owner.md -- what a consumer reads and writes: the reader contract
// Related: pointer.go -- the pointer, binding, interest and generation values
// Related: dataset.go -- the dataset value a pointer names
// Related: notice.go -- the notices that tell a consumer to read again
//
// Every consumer of the irr plugin's data (bgp-filter-irr, firewall-irr) reads
// the same three values in the same order: the binding of a bare AS number,
// the pointer of an AS-SET, then the dataset the pointer names. It holds a
// read-only grant on them, so these functions take only the read half of the
// SDK's state RPCs. The one value a consumer writes, its interest key, and the
// notice that follows it, go through PublishInterest.

package store

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
)

// StateReader is the read half of the SDK's state RPCs (sdk.Plugin.StateGet),
// all a consumer may do with the owner's keys.
type StateReader interface {
	StateGet(ctx context.Context, key string) ([]byte, bool, error)
}

// InterestPublisher is what PublishInterest needs of the SDK: a write of the
// consumer's own interest key and the emit-event that tells the owner.
type InterestPublisher interface {
	StatePut(ctx context.Context, key string, data []byte) error
	EmitEvent(ctx context.Context, namespace, eventType, direction, peerAddress, event string) (int, error)
}

// Named answers of the consumer reads. ErrNotStored is the only one that means
// "the owner holds nothing for this name": no binding yet, no pointer yet, or
// `clear irr` removed it. A consumer then enforces no list. Every other error
// means a stored value exists and cannot be used: the consumer keeps what it
// enforces, logs the error and reports it (AC-22).
var (
	ErrNotStored           = errors.New("irr/store: nothing is stored")
	ErrDatasetNotPointed   = errors.New("irr/store: the stored dataset is not the one its pointer names")
	ErrGenerationMalformed = errors.New("irr/store: the stored generation is malformed")
)

// generationLen is the size of the counter StateIncrement keeps: a big-endian
// uint32 (internal/core/statestore Increment).
const generationLen = 4

// StoredSet is the copy of one AS-SET a consumer enforces: the pointer it was
// read under and the dataset that pointer names. Only ReadSet and ReadDataset
// produce one, so its dataset always passed its digest and its pointer's.
type StoredSet struct {
	Pointer Pointer
	Dataset Dataset
}

// ReadGeneration answers the generation the owner last raised. A counter never
// raised answers 0 with no error: no pointer was ever written, and the first
// notice will carry 1.
func ReadGeneration(ctx context.Context, r StateReader) (uint32, error) {
	value, found, err := r.StateGet(ctx, GenerationKey())
	if err != nil {
		return 0, err
	}
	if !found {
		return 0, nil
	}
	if len(value) != generationLen {
		return 0, fmt.Errorf("%w: %d bytes, want %d", ErrGenerationMalformed, len(value), generationLen)
	}
	return binary.BigEndian.Uint32(value), nil
}

// ReadBinding answers the AS-SET the owner queries for the bare AS number asn.
// No binding answers ErrNotStored: the owner has not asked PeeringDB yet. A
// consumer re-reads the binding on every dataset notice, because a binding
// that moved is announced only by the notice of the new set.
func ReadBinding(ctx context.Context, r StateReader, asn uint32) (Binding, error) {
	value, found, err := r.StateGet(ctx, BindingKey(asn))
	if err != nil {
		return Binding{}, err
	}
	if !found {
		return Binding{}, ErrNotStored
	}
	return DecodeBinding(value)
}

// ReadPointer answers the pointer of asSet, which MUST be an ASSetName answer.
// No pointer answers ErrNotStored. A consumer that already enforces the
// dataset this pointer names skips ReadDataset.
func ReadPointer(ctx context.Context, r StateReader, asSet string) (Pointer, error) {
	value, found, err := r.StateGet(ctx, PointerKey(asSet))
	if err != nil {
		return Pointer{}, err
	}
	if !found {
		return Pointer{}, ErrNotStored
	}
	return DecodePointer(value)
}

// ReadDataset reads the dataset pointer names for asSet and refuses one that
// is missing, fails its own digest or decode, or is not the dataset the
// pointer names. None of these is ErrNotStored: a pointer exists, so the
// consumer keeps what it enforces. The key ends in the pointer's digest and
// DecodeDataset refuses a value whose own digest is another, so a value that
// is not the pointed one answers ErrDatasetNotPointed.
func ReadDataset(ctx context.Context, r StateReader, asSet string, pointer Pointer) (StoredSet, error) {
	key := pointer.DatasetKey(asSet)
	value, found, err := r.StateGet(ctx, key)
	if err != nil {
		return StoredSet{}, err
	}
	if !found {
		return StoredSet{}, fmt.Errorf("%w: %s is named by its pointer and not stored", ErrDatasetNotPointed, key)
	}
	dataset, err := DecodeDataset(key, value)
	if errors.Is(err, ErrDatasetIdentity) {
		return StoredSet{}, fmt.Errorf("%w: %w", ErrDatasetNotPointed, err)
	}
	if err != nil {
		return StoredSet{}, err
	}
	return StoredSet{Pointer: pointer, Dataset: dataset}, nil
}

// ReadSet reads the pointer of asSet, then the dataset it names.
func ReadSet(ctx context.Context, r StateReader, asSet string) (StoredSet, error) {
	pointer, err := ReadPointer(ctx, r, asSet)
	if err != nil {
		return StoredSet{}, err
	}
	return ReadDataset(ctx, r, asSet, pointer)
}

// PublishInterest writes consumer's interest key, then tells the owner to read
// it again. The key is written first: the owner reads the key, never the
// notice, so a notice that is lost is recovered at the owner's next read, and
// a key written without its notice is read at the owner's next start. The
// caller MUST NOT hold a lock the filter evaluator or a publish takes: the
// write can take the SDK's whole state timeout under disk load.
func PublishInterest(ctx context.Context, w InterestPublisher, consumer string, in Interest) error {
	value, err := EncodeInterest(in)
	if err != nil {
		return err
	}
	if err := w.StatePut(ctx, InterestKey(consumer), value); err != nil {
		return err
	}
	payload, err := json.Marshal(InterestNotice{Consumer: consumer})
	if err != nil {
		return err
	}
	_, err = w.EmitEvent(ctx, NoticeNamespace, EventInterestChanged, "", "", string(payload))
	return err
}

// DecodeDatasetNotice reads the payload of irr/dataset-stored as a consumer
// subscribed without an envelope receives it. A notice that names no set the
// store can key, or generation 0, is refused.
func DecodeDatasetNotice(event string) (DatasetNotice, error) {
	var notice DatasetNotice
	if err := json.Unmarshal([]byte(event), &notice); err != nil {
		return DatasetNotice{}, fmt.Errorf("irr/store: dataset notice: %w", err)
	}
	set, err := ASSetName(notice.ASSet)
	if err != nil {
		return DatasetNotice{}, fmt.Errorf("irr/store: dataset notice: %w", err)
	}
	if notice.Generation == 0 {
		return DatasetNotice{}, errors.New("irr/store: dataset notice: generation 0")
	}
	notice.ASSet = set
	return notice, nil
}
