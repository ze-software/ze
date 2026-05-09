package show

import (
	"bytes"
	"encoding/binary"
	"testing"
	"time"
)

func TestWritePcapHeader(t *testing.T) {
	var buf bytes.Buffer
	if err := writePcapHeader(&buf, 1500, LinkTypeRaw); err != nil {
		t.Fatal(err)
	}
	if buf.Len() != 24 {
		t.Fatalf("header len = %d, want 24", buf.Len())
	}
	magic := binary.LittleEndian.Uint32(buf.Bytes()[0:4])
	if magic != pcapMagic {
		t.Errorf("magic = 0x%x, want 0x%x", magic, pcapMagic)
	}
	maj := binary.LittleEndian.Uint16(buf.Bytes()[4:6])
	min := binary.LittleEndian.Uint16(buf.Bytes()[6:8])
	if maj != 2 || min != 4 {
		t.Errorf("version = %d.%d, want 2.4", maj, min)
	}
	snapLen := binary.LittleEndian.Uint32(buf.Bytes()[16:20])
	if snapLen != 1500 {
		t.Errorf("snaplen = %d, want 1500", snapLen)
	}
	linkType := binary.LittleEndian.Uint32(buf.Bytes()[20:24])
	if linkType != LinkTypeRaw {
		t.Errorf("link type = %d, want %d", linkType, LinkTypeRaw)
	}
}

func TestWritePcapPacket(t *testing.T) {
	var buf bytes.Buffer
	ts := time.Date(2026, 1, 1, 12, 30, 45, 123456000, time.UTC)
	data := []byte{0xDE, 0xAD, 0xBE, 0xEF}

	if err := writePcapPacket(&buf, ts, data); err != nil {
		t.Fatal(err)
	}
	if buf.Len() != 16+4 {
		t.Fatalf("packet len = %d, want 20", buf.Len())
	}
	tsSec := binary.LittleEndian.Uint32(buf.Bytes()[0:4])
	if int64(tsSec) != ts.Unix() {
		t.Errorf("ts_sec = %d, want %d", tsSec, ts.Unix())
	}
	tsUsec := binary.LittleEndian.Uint32(buf.Bytes()[4:8])
	if tsUsec != 123456 {
		t.Errorf("ts_usec = %d, want 123456", tsUsec)
	}
	inclLen := binary.LittleEndian.Uint32(buf.Bytes()[8:12])
	if inclLen != 4 {
		t.Errorf("incl_len = %d, want 4", inclLen)
	}
	if !bytes.Equal(buf.Bytes()[16:20], data) {
		t.Errorf("data mismatch")
	}
}

func TestPcapRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	if err := writePcapHeader(&buf, 65535, LinkTypeRaw); err != nil {
		t.Fatal(err)
	}
	ts := time.Now()
	pkt1 := []byte{1, 2, 3}
	pkt2 := []byte{4, 5, 6, 7, 8}
	if err := writePcapPacket(&buf, ts, pkt1); err != nil {
		t.Fatal(err)
	}
	if err := writePcapPacket(&buf, ts, pkt2); err != nil {
		t.Fatal(err)
	}
	expected := 24 + (16 + 3) + (16 + 5)
	if buf.Len() != expected {
		t.Fatalf("total len = %d, want %d", buf.Len(), expected)
	}
}
