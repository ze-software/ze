// Design: docs/architecture/flowexport/flow-export-2-flow-records.md -- Prefix-to-AS radix tree

package enrich

import "net/netip"

// ASEntry holds BGP-derived data for a prefix: origin AS, next-hop,
// local-pref, and the full AS path. Value type, safe to copy.
type ASEntry struct {
	AS        uint32
	NextHop   netip.Addr
	LocalPref uint32
	ASPath    []uint32
}

// trieNode is one node in the binary trie. Children are indexed by
// the next bit (0 or 1). A non-nil entry marks this node as a valid
// prefix endpoint for longest-prefix-match.
type trieNode struct {
	children [2]*trieNode
	entry    *ASEntry
}

// RadixTree stores prefix-to-ASEntry mappings in a binary trie for
// longest-prefix-match lookups. Separate roots for IPv4 and IPv6.
// The tree is mutable during construction; callers should build a
// complete tree and then swap it atomically into the Enricher.
type RadixTree struct {
	v4 trieNode
	v6 trieNode
}

// NewRadixTree returns an empty radix tree.
func NewRadixTree() *RadixTree {
	return &RadixTree{}
}

// Insert adds a prefix-to-ASEntry mapping. If the prefix already
// exists, its entry is replaced.
func (t *RadixTree) Insert(prefix netip.Prefix, entry ASEntry) {
	prefix = prefix.Masked()
	addr := prefix.Addr()
	bits := prefix.Bits()

	node := t.root(addr)
	ab := addrBits(addr)

	for i := range bits {
		byteIdx := i / 8
		bitIdx := uint(7 - i%8)
		bit := (ab[byteIdx] >> bitIdx) & 1
		if node.children[bit] == nil {
			node.children[bit] = &trieNode{}
		}
		node = node.children[bit]
	}
	e := entry
	node.entry = &e
}

func (t *RadixTree) root(addr netip.Addr) *trieNode {
	if addr.Is6() {
		return &t.v6
	}
	return &t.v4
}

// addrBits returns the address bytes as a 16-byte array (IPv4 in first 4).
func addrBits(addr netip.Addr) [16]byte {
	if addr.Is6() {
		return addr.As16()
	}
	a4 := addr.As4()
	var out [16]byte
	copy(out[:4], a4[:])
	return out
}

// Delete removes a prefix mapping. No-op if the prefix is not present.
func (t *RadixTree) Delete(prefix netip.Prefix) {
	prefix = prefix.Masked()
	addr := prefix.Addr()
	bits := prefix.Bits()

	node := t.root(addr)
	ab := addrBits(addr)

	for i := range bits {
		byteIdx := i / 8
		bitIdx := uint(7 - i%8)
		bit := (ab[byteIdx] >> bitIdx) & 1
		if node.children[bit] == nil {
			return
		}
		node = node.children[bit]
	}
	node.entry = nil
}

// Lookup performs a longest-prefix-match for addr. Returns the entry
// from the most specific prefix containing addr, or false if no
// prefix matches. Hot path: no allocations, no fmt calls.
func (t *RadixTree) Lookup(addr netip.Addr) (ASEntry, bool) {
	if addr.Is6() {
		return lookup6(t, addr)
	}
	return lookup4(t, addr)
}

func lookup4(t *RadixTree, addr netip.Addr) (ASEntry, bool) {
	a4 := addr.As4()
	node := &t.v4
	var best *ASEntry

	if node.entry != nil {
		best = node.entry
	}

	for i := range 32 {
		byteIdx := i / 8
		bitIdx := uint(7 - i%8)
		bit := (a4[byteIdx] >> bitIdx) & 1
		child := node.children[bit]
		if child == nil {
			break
		}
		node = child
		if node.entry != nil {
			best = node.entry
		}
	}

	if best == nil {
		return ASEntry{}, false
	}
	return *best, true
}

func lookup6(t *RadixTree, addr netip.Addr) (ASEntry, bool) {
	a16 := addr.As16()
	node := &t.v6
	var best *ASEntry

	if node.entry != nil {
		best = node.entry
	}

	for i := range 128 {
		byteIdx := i / 8
		bitIdx := uint(7 - i%8)
		bit := (a16[byteIdx] >> bitIdx) & 1
		child := node.children[bit]
		if child == nil {
			break
		}
		node = child
		if node.entry != nil {
			best = node.entry
		}
	}

	if best == nil {
		return ASEntry{}, false
	}
	return *best, true
}
