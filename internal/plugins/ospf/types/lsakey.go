// Design: docs/architecture/ospf/ospf-1-types.md -- LSAKey LSDB identity tuple
// Related: lstype.go -- LS type discriminator
// Related: linkstateid.go -- Link State ID component
// Related: routerid.go -- Advertising Router component

package types

// LSAHeaderLen is the OSPFv2 common LSA header length.
const LSAHeaderLen = 20

// LSAKey is the LSDB identity tuple: LS type, Link State ID, Advertising Router.
//
// RFC 2328 Appendix A.4.1: these three fields uniquely identify an LSA. RFC
// 2328 Section 13.1 uses sequence, age, and checksum only to determine which
// instance is newer; they are deliberately excluded from this key.
type LSAKey struct {
	Type              LSType
	LinkStateID       LinkStateID
	AdvertisingRouter RouterID
}

// lsaKeyFromHeader extracts the LSDB key from an exact 20-byte LSA header.
func lsaKeyFromHeader(header []byte) (LSAKey, error) {
	if len(header) != LSAHeaderLen {
		return LSAKey{}, ErrWrongLength
	}
	lsid, err := LinkStateIDFromBytes(header[4:8])
	if err != nil {
		return LSAKey{}, err
	}
	adv, err := RouterIDFromBytes(header[8:12])
	if err != nil {
		return LSAKey{}, err
	}
	return LSAKey{
		Type:              LSTypeFromByte(header[3]),
		LinkStateID:       lsid,
		AdvertisingRouter: adv,
	}, nil
}

// Compare orders LSA keys by type, Link State ID, then Advertising Router.
func (k LSAKey) Compare(other LSAKey) int {
	if k.Type < other.Type {
		return -1
	}
	if k.Type > other.Type {
		return 1
	}
	if c := compare4([4]byte(k.LinkStateID), [4]byte(other.LinkStateID)); c != 0 {
		return c
	}
	return compare4([4]byte(k.AdvertisingRouter), [4]byte(other.AdvertisingRouter))
}

// Less reports whether k sorts before other.
func (k LSAKey) Less(other LSAKey) bool { return k.Compare(other) < 0 }
