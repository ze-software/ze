// Design: docs/architecture/pool-architecture.md -- per-attribute pool instances

package pool

import (
	"encoding/binary"

	"github.com/ze-software/ze/internal/component/bgp/attrpool"
)

// Labels stores MPLS label stacks as pooled byte slices. Each label is
// 4 bytes (uint32, little-endian). The pool deduplicates identical label
// stacks across routes (common for PE-originated prefixes sharing a label).
var Labels *attrpool.Pool

func init() {
	Labels = mustPool(15, 1<<12, shardsHot) // label stacks diverse across prefixes
}

// InternLabels stores a label stack in the pool and returns a handle.
// Returns InvalidHandle if labels is empty.
func InternLabels(labels []uint32) attrpool.Handle {
	if len(labels) == 0 {
		return attrpool.InvalidHandle
	}
	buf := make([]byte, len(labels)*4)
	for i, l := range labels {
		binary.LittleEndian.PutUint32(buf[i*4:], l)
	}
	h, _ := Labels.Intern(buf)
	return h
}

// ResolveLabels retrieves the label stack for a handle.
// Returns nil for InvalidHandle.
func ResolveLabels(h attrpool.Handle) []uint32 {
	if !h.IsValid() {
		return nil
	}
	data, err := Labels.Get(h)
	if err != nil || len(data) == 0 {
		return nil
	}
	n := len(data) / 4
	labels := make([]uint32, n)
	for i := range n {
		labels[i] = binary.LittleEndian.Uint32(data[i*4:])
	}
	return labels
}
