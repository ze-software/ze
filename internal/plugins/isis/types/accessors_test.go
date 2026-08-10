package types

import (
	"bytes"
	"testing"
)

// VALIDATES: Equal methods on every identifier type agree with byte identity,
// and Bytes() accessors return faithful copies (consumed by isis-2 wire codec).
// PREVENTS: an exported accessor or Equal silently diverging from the value.
func TestEqualAndBytesAccessors(t *testing.T) {
	t.Run("SystemID", func(t *testing.T) {
		a := SystemID{0, 1, 0, 2, 0, 3}
		b := a
		c := SystemID{0, 1, 0, 2, 0, 4}
		if !a.Equal(b) || a.Equal(c) {
			t.Error("SystemID.Equal mismatch")
		}
		if !bytes.Equal(a.Bytes(), []byte{0, 1, 0, 2, 0, 3}) {
			t.Errorf("SystemID.Bytes() = %x", a.Bytes())
		}
		// Bytes() must be a copy: mutating it must not affect the value.
		bs := a.Bytes()
		bs[0] = 0xff
		if a[0] != 0 {
			t.Error("SystemID.Bytes() must return an independent copy")
		}
	})

	t.Run("SourceID", func(t *testing.T) {
		a := NewSourceID(SystemID{0, 1, 0, 2, 0, 3}, 5)
		b := a
		c := NewSourceID(SystemID{0, 1, 0, 2, 0, 3}, 6)
		if !a.Equal(b) || a.Equal(c) {
			t.Error("SourceID.Equal mismatch")
		}
	})

	t.Run("LSPID", func(t *testing.T) {
		a := NewLSPID(NewSourceID(SystemID{0, 1, 0, 2, 0, 3}, 0), 1)
		b := a
		c := NewLSPID(NewSourceID(SystemID{0, 1, 0, 2, 0, 3}, 0), 2)
		if !a.Equal(b) || a.Equal(c) {
			t.Error("LSPID.Equal mismatch")
		}
		if a.SourceID() != NewSourceID(SystemID{0, 1, 0, 2, 0, 3}, 0) {
			t.Errorf("LSPID.SourceID() = %v", a.SourceID())
		}
	})
}

// VALIDATES: AreaID WriteTo/AppendTo/String and NET Len/Bytes/Equal behave
// consistently (consumed by isis-2 TLV codec and CLI display).
// PREVENTS: a variable-length accessor returning the wrong slice or length.
func TestAreaIDAndNETAccessors(t *testing.T) {
	area := areaIDFromBytesUnchecked([]byte{0x49, 0x00, 0x01})
	var buf [16]byte
	n := area.WriteTo(buf[:], 0)
	if n != 3 || !bytes.Equal(buf[:n], []byte{0x49, 0x00, 0x01}) {
		t.Errorf("AreaID.WriteTo = %x (n=%d)", buf[:n], n)
	}
	if got := area.String(); got != "49.0001" {
		t.Errorf("AreaID.String() = %q, want %q", got, "49.0001")
	}
	if got := string(area.AppendTo(nil)); got != "49.0001" {
		t.Errorf("AreaID.AppendTo = %q", got)
	}

	raw := []byte{0x49, 0x00, 0x01, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff, 0x00}
	netA, err := nETFromBytes(raw)
	if err != nil {
		t.Fatalf("NETFromBytes: %v", err)
	}
	if netA.Len() != len(raw) {
		t.Errorf("NET.Len() = %d, want %d", netA.Len(), len(raw))
	}
	if !bytes.Equal(netA.Bytes(), raw) {
		t.Errorf("NET.Bytes() = %x, want %x", netA.Bytes(), raw)
	}
	netB, _ := nETFromBytes(raw)
	if !netA.Equal(netB) {
		t.Error("identical NETs must be Equal")
	}
	diff := append([]byte(nil), raw...)
	diff[2] = 0x02
	netC, _ := nETFromBytes(diff)
	if netA.Equal(netC) {
		t.Error("different NETs must not be Equal")
	}
	// NET.Bytes() must be a copy.
	bs := netA.Bytes()
	bs[0] = 0xff
	if netA.Bytes()[0] != 0x49 {
		t.Error("NET.Bytes() must return an independent copy")
	}
}
