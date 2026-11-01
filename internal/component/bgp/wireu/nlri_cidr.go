// Design: docs/architecture/core-design.md — the in-process BGP chain, NLRI drops
// Related: mpwire.go — MP_REACH_NLRI value whose NLRI the walk frames
// RFC: rfc/short/rfc4760.md — MP_REACH_NLRI / MP_UNREACH_NLRI NLRI encoding
// RFC: rfc/short/rfc7911.md — ADD-PATH path identifier

package wireu

// CIDREntry frames the NLRI at off of a section of a CIDR family
// (filtertext.CIDRFamily), returning the whole NLRI, its part after any path
// identifier, and the offset past it. ok == false when the section is
// truncated at off.
//
// RFC 7911 Section 3: "the NLRI encoding MUST be extended by prepending the
// Path Identifier field, which is of four octets."
// RFC 4760 Section 5: "The Length field indicates the length, in bits, of the
// address prefix."
// RFC 8277 Section 2.2: "It specifies the length in bits of the remainder of the
// NLRI field." A label stack, or a withdrawal's Compatibility field, sits
// inside the length, so every CIDR family frames the same way.
//
//	+0 [Path Identifier(4), ADD-PATH only] Length(1) Prefix(ceil(Length/8))
func CIDREntry(section []byte, off int, addPath bool) (entry, cidr []byte, next int, ok bool) {
	pos := off
	if addPath {
		pos += 4
	}
	if pos >= len(section) {
		return nil, nil, 0, false
	}
	end := pos + 1 + (int(section[pos])+7)/8
	if end > len(section) {
		return nil, nil, 0, false
	}
	return section[off:end], section[pos:end], end, true
}

// CIDRCursor walks the entries of one NLRI section of a CIDR family in wire
// order. It is the one walk every NLRI subset uses: the reactor's text answer
// (an ordered prefix list) and dropped offsets, and the policy chain's prefix
// overlay. The zero value walks nothing; NewCIDRCursor starts one.
type CIDRCursor struct {
	section []byte
	off     int
	addPath bool
	bad     bool // the section did not frame
}

// NewCIDRCursor starts a walk of section, whose entries carry a path
// identifier when addPath is true.
func NewCIDRCursor(section []byte, addPath bool) CIDRCursor {
	return CIDRCursor{section: section, addPath: addPath}
}

// Next answers the next entry, its part after any path identifier, and its
// offset in the section. ok is false at the end of the section, and when it
// does not frame, which Bad then reports.
func (c *CIDRCursor) Next() (entry, cidr []byte, start int, ok bool) {
	if c.off >= len(c.section) {
		return nil, nil, 0, false
	}
	entry, cidr, next, framed := CIDREntry(c.section, c.off, c.addPath)
	if !framed {
		c.bad = true
		return nil, nil, 0, false
	}
	start, c.off = c.off, next
	return entry, cidr, start, true
}

// Bad reports whether the walk stopped on an entry that did not frame.
func (c *CIDRCursor) Bad() bool { return c.bad }

// DropCIDREntries appends to out the NLRI of section, in section order, less
// the entries that start at an offset in drops, which MUST be ascending and
// each once. Each kept NLRI is copied whole, path identifier and label stack
// included. The caller sizes out: with a capacity of len(section) the append
// never grows it, so the walk allocates nothing. ok == false when an offset
// starts no entry of the section, or the section does not frame.
func DropCIDREntries(out, section []byte, addPath bool, drops []uint16) ([]byte, bool) {
	cursor := NewCIDRCursor(section, addPath)
	for {
		entry, _, start, more := cursor.Next()
		if !more {
			break
		}
		if len(drops) > 0 && int(drops[0]) < start {
			return nil, false // the offset fell inside the entry before this one
		}
		if len(drops) > 0 && int(drops[0]) == start {
			drops = drops[1:]
			continue
		}
		out = append(out, entry...)
	}
	if cursor.Bad() {
		return nil, false
	}
	if len(drops) > 0 {
		return nil, false // an offset past the last entry
	}
	return out, true
}
