// Design: docs/architecture/ospf/ospf-2-wire.md -- Link State Update packet body codec
// RFC 2328 Appendix A.3.5: Link State Update packet.

package packet

const lsUpdateFixedLen = 4

// LSUpdate is the Link State Update packet body.
type LSUpdate struct {
	LSAs []LSA
}

// DecodeLSUpdate parses a Link State Update body. Iteration is driven by each
// LSA Length field and must consume exactly the body after the 4-byte count.
func DecodeLSUpdate(body []byte) (LSUpdate, error) {
	if len(body) < lsUpdateFixedLen {
		return LSUpdate{}, ErrTruncated
	}
	count := readUint32(body, 0)
	maxCount := (len(body) - lsUpdateFixedLen) / lsaHeaderMinLen
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
func (u LSUpdate) EncodedLen() int {
	n := lsUpdateFixedLen
	for _, lsa := range u.LSAs {
		n += lsa.EncodedLen()
	}
	return n
}

// WriteTo serializes the LS Update body into buf at off.
func (u LSUpdate) WriteTo(buf []byte, off int) int {
	off += writeUint32(buf, off, uint32(len(u.LSAs)))
	for i := range u.LSAs {
		off = (&u.LSAs[i]).WriteTo(buf, off)
	}
	return off
}
