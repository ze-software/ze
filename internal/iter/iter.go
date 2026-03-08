// Design: (none — new generic utility, predates documentation)

// Package iter provides generic zero-allocation iterators for walking
// variable-length records in byte slices.
package iter

import "fmt"

// SizeFunc returns the wire size of the first element in data.
// Returns an error if data is too short or malformed.
type SizeFunc func(data []byte) (int, error)

// Elements iterates over variable-length elements in a byte slice.
// Zero-allocation: returns subslices of the original data.
//
// Example usage:
//
//	e := iter.NewElements(data, sizeFunc)
//	for elem := e.Next(); elem != nil; elem = e.Next() {
//	    // elem is a subslice of data — do not modify
//	}
//	if err := e.Err(); err != nil { ... }
type Elements struct {
	data     []byte
	sizeFunc SizeFunc
	offset   int
	err      error
}

// NewElements creates an iterator over variable-length elements.
// The sizeFunc determines the byte size of each element from its leading bytes.
// The iterator yields subslices of data — caller must not modify data while iterating.
func NewElements(data []byte, sizeFunc SizeFunc) Elements {
	return Elements{data: data, sizeFunc: sizeFunc}
}

// Next returns the next element as a subslice, or nil when done.
// After nil, call Err() to distinguish completion from error.
func (e *Elements) Next() []byte {
	if e.err != nil || e.offset >= len(e.data) {
		return nil
	}

	size, err := e.sizeFunc(e.data[e.offset:])
	if err != nil {
		e.err = err
		return nil
	}

	if e.offset+size > len(e.data) {
		e.err = fmt.Errorf("truncated: element at offset %d claims %d bytes, %d available",
			e.offset, size, len(e.data)-e.offset)
		return nil
	}

	elem := e.data[e.offset : e.offset+size]
	e.offset += size
	return elem
}

// Err returns the first error encountered during iteration.
// Returns nil if iteration completed successfully.
func (e *Elements) Err() error {
	return e.err
}

// Offset returns the current byte position in the data.
func (e *Elements) Offset() int {
	return e.offset
}

// Reset restarts iteration from the beginning, clearing any error.
func (e *Elements) Reset() {
	e.offset = 0
	e.err = nil
}
