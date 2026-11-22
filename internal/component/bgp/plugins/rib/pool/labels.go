// Design: docs/architecture/pool-architecture.md -- per-attribute pool instances

package pool

import (
	"encoding/binary"
	"slices"

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
	return AppendLabels(nil, h)
}

// AppendLabels appends to dst the label stack h names, top of stack first, and
// returns the extended slice. It appends nothing for InvalidHandle and for a
// handle the pool cannot read, so a caller reads "no label stack" from an
// unchanged length. A dst with room for the stack makes the call allocate
// nothing, which is what a walk over a whole table needs (the MRT snapshot,
// storage.PeerRIB.IterateFamilyWire); ResolveLabels is the owned-copy form.
func AppendLabels(dst []uint32, h attrpool.Handle) []uint32 {
	if !h.IsValid() {
		return dst
	}
	data, err := Labels.Get(h)
	if err != nil {
		return dst
	}
	entries := len(data) / 4
	dst = slices.Grow(dst, entries)
	for i := range entries {
		dst = append(dst, binary.LittleEndian.Uint32(data[i*4:]))
	}
	return dst
}
