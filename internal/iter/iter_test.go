package iter

import (
	"errors"
	"fmt"
	"testing"
	"unsafe"
)

// prefixSizeFunc reads a 1-byte length prefix: element = [len:1][payload:len].
func prefixSizeFunc(data []byte) (int, error) {
	if len(data) < 1 {
		return 0, fmt.Errorf("empty data")
	}
	return 1 + int(data[0]), nil
}

// TestElements_Basic verifies correct iteration over multiple elements.
//
// VALIDATES: Next() yields correct subslices for each element.
// PREVENTS: Off-by-one in offset tracking.
func TestElements_Basic(t *testing.T) {
	t.Parallel()

	// Three elements: [2, 0xAA, 0xBB], [1, 0xCC], [0]
	data := []byte{2, 0xAA, 0xBB, 1, 0xCC, 0}

	e := NewElements(data, prefixSizeFunc)
	var got [][]byte
	for elem := e.Next(); elem != nil; elem = e.Next() {
		got = append(got, elem)
	}

	if err := e.Err(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d elements, want 3", len(got))
	}

	// Element 0: [2, 0xAA, 0xBB]
	if len(got[0]) != 3 || got[0][0] != 2 || got[0][1] != 0xAA || got[0][2] != 0xBB {
		t.Errorf("element 0 = %v, want [2 0xAA 0xBB]", got[0])
	}
	// Element 1: [1, 0xCC]
	if len(got[1]) != 2 || got[1][0] != 1 || got[1][1] != 0xCC {
		t.Errorf("element 1 = %v, want [1 0xCC]", got[1])
	}
	// Element 2: [0]
	if len(got[2]) != 1 || got[2][0] != 0 {
		t.Errorf("element 2 = %v, want [0]", got[2])
	}
}

// TestElements_Empty verifies empty data returns nil immediately.
//
// VALIDATES: Next() on empty data returns nil, Err() is nil.
// PREVENTS: Panic on empty input.
func TestElements_Empty(t *testing.T) {
	t.Parallel()

	e := NewElements(nil, prefixSizeFunc)
	if elem := e.Next(); elem != nil {
		t.Errorf("got %v, want nil", elem)
	}
	if err := e.Err(); err != nil {
		t.Errorf("got err %v, want nil", err)
	}

	e2 := NewElements([]byte{}, prefixSizeFunc)
	if elem := e2.Next(); elem != nil {
		t.Errorf("got %v, want nil", elem)
	}
	if err := e2.Err(); err != nil {
		t.Errorf("got err %v, want nil", err)
	}
}

// TestElements_SingleElement verifies single element yields one subslice.
//
// VALIDATES: Exactly one element returned, then nil.
// PREVENTS: Infinite loop or early termination.
func TestElements_SingleElement(t *testing.T) {
	t.Parallel()

	data := []byte{3, 0x01, 0x02, 0x03}
	e := NewElements(data, prefixSizeFunc)

	elem := e.Next()
	if elem == nil {
		t.Fatal("got nil, want element")
	}
	if len(elem) != 4 {
		t.Errorf("element length = %d, want 4", len(elem))
	}

	if next := e.Next(); next != nil {
		t.Errorf("second Next() = %v, want nil", next)
	}
	if err := e.Err(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestElements_MalformedData verifies truncated data sets Err().
//
// VALIDATES: When element extends beyond data, Err() reports truncation.
// PREVENTS: Index out of range panic.
func TestElements_MalformedData(t *testing.T) {
	t.Parallel()

	// Claims 5 bytes of payload but only 2 available
	data := []byte{5, 0xAA, 0xBB}
	e := NewElements(data, prefixSizeFunc)

	if elem := e.Next(); elem != nil {
		t.Errorf("got %v, want nil", elem)
	}
	if err := e.Err(); err == nil {
		t.Error("expected error for truncated data")
	}
}

// TestElements_SizeFuncError verifies sizeFunc error propagation.
//
// VALIDATES: Error from sizeFunc is available via Err().
// PREVENTS: Swallowed errors causing silent data loss.
func TestElements_SizeFuncError(t *testing.T) {
	t.Parallel()

	errBad := fmt.Errorf("bad element")
	failFunc := func([]byte) (int, error) { return 0, errBad }

	data := []byte{1, 2, 3}
	e := NewElements(data, failFunc)

	if elem := e.Next(); elem != nil {
		t.Errorf("got %v, want nil", elem)
	}
	if !errors.Is(e.Err(), errBad) {
		t.Errorf("Err() = %v, want %v", e.Err(), errBad)
	}
}

// TestElements_ZeroAlloc verifies zero heap allocations during iteration.
//
// VALIDATES: Iterator operates without heap allocation.
// PREVENTS: Performance regression from hidden allocations.
func TestElements_ZeroAlloc(t *testing.T) {
	// AllocsPerRun is incompatible with t.Parallel()
	data := []byte{2, 0xAA, 0xBB, 1, 0xCC, 0}

	allocs := testing.AllocsPerRun(100, func() {
		e := NewElements(data, prefixSizeFunc)
		for elem := e.Next(); elem != nil; elem = e.Next() {
			_ = elem
		}
	})
	if allocs != 0 {
		t.Errorf("allocations = %v, want 0", allocs)
	}
}

// TestElements_SubsliceVerification verifies elements are subslices of original.
//
// VALIDATES: Each element points into the original buffer.
// PREVENTS: Accidental copies.
func TestElements_SubsliceVerification(t *testing.T) {
	t.Parallel()

	data := []byte{2, 0xAA, 0xBB, 1, 0xCC, 0}
	dataStart := uintptr(unsafe.Pointer(&data[0]))
	dataEnd := dataStart + uintptr(len(data))

	e := NewElements(data, prefixSizeFunc)
	for elem := e.Next(); elem != nil; elem = e.Next() {
		elemStart := uintptr(unsafe.Pointer(&elem[0]))
		if elemStart < dataStart || elemStart >= dataEnd {
			t.Errorf("element at %v is outside data range [%v, %v)", elemStart, dataStart, dataEnd)
		}
	}
}

// TestElements_Offset verifies Offset() tracks position correctly.
//
// VALIDATES: Offset() returns cumulative bytes consumed.
// PREVENTS: Incorrect offset calculation breaking caller chunking logic.
func TestElements_Offset(t *testing.T) {
	t.Parallel()

	// [2, AA, BB] size=3, [1, CC] size=2, [0] size=1
	data := []byte{2, 0xAA, 0xBB, 1, 0xCC, 0}

	e := NewElements(data, prefixSizeFunc)

	if e.Offset() != 0 {
		t.Errorf("initial Offset() = %d, want 0", e.Offset())
	}

	e.Next() // consume element 0 (3 bytes)
	if e.Offset() != 3 {
		t.Errorf("after element 0: Offset() = %d, want 3", e.Offset())
	}

	e.Next() // consume element 1 (2 bytes)
	if e.Offset() != 5 {
		t.Errorf("after element 1: Offset() = %d, want 5", e.Offset())
	}

	e.Next() // consume element 2 (1 byte)
	if e.Offset() != 6 {
		t.Errorf("after element 2: Offset() = %d, want 6", e.Offset())
	}
}

// TestElements_Reset verifies Reset() restarts iteration.
//
// VALIDATES: After Reset(), iteration yields same elements again.
// PREVENTS: Stale state after reset.
func TestElements_Reset(t *testing.T) {
	t.Parallel()

	data := []byte{2, 0xAA, 0xBB, 1, 0xCC}
	e := NewElements(data, prefixSizeFunc)

	// Consume all
	count := 0
	for e.Next() != nil {
		count++
	}
	if count != 2 {
		t.Fatalf("first pass: %d elements, want 2", count)
	}

	// Reset and iterate again
	e.Reset()
	count = 0
	for e.Next() != nil {
		count++
	}
	if count != 2 {
		t.Errorf("after Reset: %d elements, want 2", count)
	}
	if e.Offset() != 5 {
		t.Errorf("after Reset iteration: Offset() = %d, want 5", e.Offset())
	}
}

// TestElements_ResetClearsError verifies Reset() clears error state.
//
// VALIDATES: After Reset() on errored iterator, Err() is nil and iteration restarts.
// PREVENTS: Sticky error state after reset.
func TestElements_ResetClearsError(t *testing.T) {
	t.Parallel()

	// First element valid, second truncated
	data := []byte{1, 0xAA, 5, 0xBB}
	e := NewElements(data, prefixSizeFunc)

	e.Next() // OK
	e.Next() // error (truncated)
	if e.Err() == nil {
		t.Fatal("expected error")
	}

	e.Reset()
	if e.Err() != nil {
		t.Errorf("after Reset: Err() = %v, want nil", e.Err())
	}
	if e.Offset() != 0 {
		t.Errorf("after Reset: Offset() = %d, want 0", e.Offset())
	}

	// Should iterate again
	elem := e.Next()
	if elem == nil {
		t.Error("after Reset: Next() = nil, want element")
	}
}
