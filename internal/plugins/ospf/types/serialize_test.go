// Design: docs/architecture/ospf/ospf-1-types.md -- WriteTo round-trip tests for all OSPF leaf types

package types

import (
	"bytes"
	"testing"
)

// VALIDATES: AC-12 - every type writes exact big-endian wire bytes at the requested offset.
// PREVENTS: packet codec helpers from allocating or writing shifted fields.
func TestWriteToRoundTrip(t *testing.T) {
	buf := [32]byte{}
	router := RouterID{10, 0, 0, 1}
	if n := router.WriteTo(buf[:], 1); n != RouterIDLen || !bytes.Equal(buf[1:5], []byte{10, 0, 0, 1}) {
		t.Fatalf("RouterID WriteTo n=%d bytes=%v", n, buf[1:5])
	}
	area := AreaID{0, 0, 0, 1}
	if n := area.WriteTo(buf[:], 2); n != AreaIDLen || !bytes.Equal(buf[2:6], []byte{0, 0, 0, 1}) {
		t.Fatalf("AreaID WriteTo n=%d bytes=%v", n, buf[2:6])
	}
	lsid := LinkStateID{192, 0, 2, 7}
	if n := lsid.WriteTo(buf[:], 3); n != LinkStateIDLen || !bytes.Equal(buf[3:7], []byte{192, 0, 2, 7}) {
		t.Fatalf("LinkStateID WriteTo n=%d bytes=%v", n, buf[3:7])
	}
	if n := LSTypeASExternal.WriteTo(buf[:], 0); n != OptionsLen || buf[0] != byte(LSTypeASExternal) {
		t.Fatalf("LSType WriteTo n=%d byte=%d", n, buf[0])
	}
	seq := InitialSequenceNumber
	if n := seq.WriteTo(buf[:], 0); n != LSSequenceNumberLen || !bytes.Equal(buf[:4], []byte{0x80, 0x00, 0x00, 0x01}) {
		t.Fatalf("LSSequenceNumber WriteTo n=%d bytes=%v", n, buf[:4])
	}
	age := LSAge(MaxAge)
	if n := age.WriteTo(buf[:], 0); n != LSAgeLen || !bytes.Equal(buf[:2], []byte{0x0e, 0x10}) {
		t.Fatalf("LSAge WriteTo n=%d bytes=%v", n, buf[:2])
	}
	metric, err := NewMetric(10)
	if err != nil {
		t.Fatalf("NewMetric returned error: %v", err)
	}
	if n := metric.WriteTo(buf[:], 0); n != MetricLen || !bytes.Equal(buf[:2], []byte{0x00, 0x0a}) {
		t.Fatalf("Metric WriteTo n=%d bytes=%v", n, buf[:2])
	}
	opts := OptionE.Set(OptionNP)
	if n := opts.WriteTo(buf[:], 0); n != OptionsLen || buf[0] != byte(OptionE|OptionNP) {
		t.Fatalf("Options WriteTo n=%d byte=%#x", n, buf[0])
	}
}
