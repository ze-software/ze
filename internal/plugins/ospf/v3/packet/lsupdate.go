// Design: docs/architecture/ospf/ospfv3-2-wire.md -- OSPFv3 Link State Update packet body codec.
// RFC: rfc/short/rfc5340.md (§A.3.5 Link State Update packet)

package packet

// lsUpdateFixedLen is the 4-octet "# LSAs" count prefix.
const lsUpdateFixedLen = 4

// LSUpdate is the OSPFv3 Link State Update packet body. Each LSA uses the
// 20-octet OSPFv3 LSA header and is self-delimited by its Length field.
type LSUpdate struct {
	LSAs []LSA
}

// DecodeLSUpdate parses a Link State Update body. The 32-bit "# LSAs" count is
// validated against the maximum number of minimum-size LSAs the body could hold
// before any allocation, then iteration is driven by each LSA Length field and
// must consume the body exactly (RFC 5340 §A.3.5).
func DecodeLSUpdate(body []byte) (LSUpdate, error) {
	if len(body) < lsUpdateFixedLen {
		return LSUpdate{}, ErrTruncated
	}
	count := readUint32(body, 0)
	maxCount := (len(body) - lsUpdateFixedLen) / LSAHeaderLen
	if count > uint32(maxCount) {
		return LSUpdate{}, ErrLength
	}
	out := LSUpdate{LSAs: make([]LSA, 0, int(count))}
	it := NewLSAIterator(body[lsUpdateFixedLen:])
	for it.Next() {
		out.LSAs = append(out.LSAs, it.LSA())
	}
	if err := it.Err(); err != nil {
		return LSUpdate{}, err
	}
	if len(out.LSAs) != int(count) {
		return LSUpdate{}, ErrLength
	}
	return out, nil
}

// EncodedLen returns the LS Update body length.
func (u *LSUpdate) EncodedLen() int {
	n := lsUpdateFixedLen
	for i := range u.LSAs {
		n += u.LSAs[i].EncodedLen()
	}
	return n
}

// WriteTo serializes the LS Update body into buf at off.
func (u *LSUpdate) WriteTo(buf []byte, off int) int {
	off += writeUint32(buf, off, uint32(len(u.LSAs)))
	for i := range u.LSAs {
		off = u.LSAs[i].WriteTo(buf, off)
	}
	return off
}
