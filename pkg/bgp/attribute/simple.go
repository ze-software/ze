package attribute

import (
	"encoding/binary"
	"net/netip"
)

// NextHop represents the NEXT_HOP attribute (RFC 4271 Section 5.1.3).
type NextHop struct {
	Addr netip.Addr
}

func (n *NextHop) Code() AttributeCode   { return AttrNextHop }
func (n *NextHop) Flags() AttributeFlags { return FlagTransitive }
func (n *NextHop) Len() int {
	if n.Addr.Is6() {
		return 16
	}
	return 4
}
func (n *NextHop) Pack() []byte { return n.Addr.AsSlice() }

// ParseNextHop parses a NEXT_HOP attribute.
func ParseNextHop(data []byte) (*NextHop, error) {
	if len(data) != 4 && len(data) != 16 {
		return nil, ErrInvalidLength
	}
	addr, ok := netip.AddrFromSlice(data)
	if !ok {
		return nil, ErrMalformedValue
	}
	return &NextHop{Addr: addr}, nil
}

// MED represents the MULTI_EXIT_DISC attribute (RFC 4271 Section 5.1.4).
type MED uint32

func (m MED) Code() AttributeCode   { return AttrMED }
func (m MED) Flags() AttributeFlags { return FlagOptional }
func (m MED) Len() int              { return 4 }
func (m MED) Pack() []byte {
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, uint32(m))
	return buf
}

// ParseMED parses a MULTI_EXIT_DISC attribute.
func ParseMED(data []byte) (MED, error) {
	if len(data) != 4 {
		return 0, ErrInvalidLength
	}
	return MED(binary.BigEndian.Uint32(data)), nil
}

// LocalPref represents the LOCAL_PREF attribute (RFC 4271 Section 5.1.5).
type LocalPref uint32

func (l LocalPref) Code() AttributeCode   { return AttrLocalPref }
func (l LocalPref) Flags() AttributeFlags { return FlagTransitive }
func (l LocalPref) Len() int              { return 4 }
func (l LocalPref) Pack() []byte {
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, uint32(l))
	return buf
}

// ParseLocalPref parses a LOCAL_PREF attribute.
func ParseLocalPref(data []byte) (LocalPref, error) {
	if len(data) != 4 {
		return 0, ErrInvalidLength
	}
	return LocalPref(binary.BigEndian.Uint32(data)), nil
}

// AtomicAggregate represents the ATOMIC_AGGREGATE attribute (RFC 4271 Section 5.1.6).
// It has no value - its presence indicates aggregation occurred.
type AtomicAggregate struct{}

func (AtomicAggregate) Code() AttributeCode   { return AttrAtomicAggregate }
func (AtomicAggregate) Flags() AttributeFlags { return FlagTransitive }
func (AtomicAggregate) Len() int              { return 0 }
func (AtomicAggregate) Pack() []byte          { return nil }

// Aggregator represents the AGGREGATOR attribute (RFC 4271 Section 5.1.7).
type Aggregator struct {
	ASN     uint32
	Address netip.Addr
}

func (a *Aggregator) Code() AttributeCode   { return AttrAggregator }
func (a *Aggregator) Flags() AttributeFlags { return FlagOptional | FlagTransitive }
func (a *Aggregator) Len() int              { return 8 } // 4-byte AS + 4-byte IP
func (a *Aggregator) Pack() []byte {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint32(buf[0:4], a.ASN)
	copy(buf[4:8], a.Address.AsSlice())
	return buf
}

// ParseAggregator parses an AGGREGATOR attribute.
func ParseAggregator(data []byte, fourByteAS bool) (*Aggregator, error) {
	if fourByteAS {
		if len(data) != 8 {
			return nil, ErrInvalidLength
		}
		addr, _ := netip.AddrFromSlice(data[4:8])
		return &Aggregator{
			ASN:     binary.BigEndian.Uint32(data[0:4]),
			Address: addr,
		}, nil
	}
	// 2-byte AS format
	if len(data) != 6 {
		return nil, ErrInvalidLength
	}
	addr, _ := netip.AddrFromSlice(data[2:6])
	return &Aggregator{
		ASN:     uint32(binary.BigEndian.Uint16(data[0:2])),
		Address: addr,
	}, nil
}

// OriginatorID represents the ORIGINATOR_ID attribute (RFC 4456).
type OriginatorID netip.Addr

func (o OriginatorID) Code() AttributeCode   { return AttrOriginatorID }
func (o OriginatorID) Flags() AttributeFlags { return FlagOptional }
func (o OriginatorID) Len() int              { return 4 }
func (o OriginatorID) Pack() []byte          { return netip.Addr(o).AsSlice() }

// ClusterList represents the CLUSTER_LIST attribute (RFC 4456).
type ClusterList []uint32

func (c ClusterList) Code() AttributeCode   { return AttrClusterList }
func (c ClusterList) Flags() AttributeFlags { return FlagOptional }
func (c ClusterList) Len() int              { return len(c) * 4 }
func (c ClusterList) Pack() []byte {
	buf := make([]byte, len(c)*4)
	for i, id := range c {
		binary.BigEndian.PutUint32(buf[i*4:], id)
	}
	return buf
}

// ParseClusterList parses a CLUSTER_LIST attribute.
func ParseClusterList(data []byte) (ClusterList, error) {
	if len(data)%4 != 0 {
		return nil, ErrInvalidLength
	}
	list := make(ClusterList, len(data)/4)
	for i := range list {
		list[i] = binary.BigEndian.Uint32(data[i*4:])
	}
	return list, nil
}
