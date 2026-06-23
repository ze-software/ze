// Design: plan/spec-ospf-2-wire.md -- fuzz targets for packet and LSA decode

package packet

import "testing"

func FuzzOSPFDecodePacket(f *testing.F) {
	f.Add(fuzzSeedPacket())
	f.Add([]byte{0x02})
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = DecodePacket(data)
	})
}

func FuzzOSPFLSAIterator(f *testing.F) {
	f.Add(fuzzSeedLSA())
	f.Add([]byte{0, 1, 2})
	f.Fuzz(func(t *testing.T, data []byte) {
		it := NewLSAIterator(data)
		for it.Next() {
			_ = it.LSA()
		}
		_ = it.Err()
	})
}

func FuzzOSPFRoundTrip(f *testing.F) {
	f.Add(fuzzSeedPacket())
	f.Fuzz(func(t *testing.T, data []byte) {
		p, err := DecodePacket(data)
		if err != nil {
			return
		}
		buf := make([]byte, p.EncodedLen())
		(&p).WriteTo(buf, 0)
		_, _ = DecodePacket(buf)
	})
}

func fuzzSeedPacket() []byte {
	p := Packet{Header: Header{Type: PacketTypeHello}, Hello: &Hello{}}
	buf := make([]byte, p.EncodedLen())
	(&p).WriteTo(buf, 0)
	return buf
}

func fuzzSeedLSA() []byte {
	lsa := LSA{Header: LSAHeader{Type: 1}, Router: &RouterLSA{}}
	buf := make([]byte, lsa.EncodedLen())
	(&lsa).WriteTo(buf, 0)
	return buf
}
