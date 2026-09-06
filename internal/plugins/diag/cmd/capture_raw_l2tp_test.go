// Goal: prove the L2TP capture is byte-identical to what it was before
// internal/core/pcap took over the file format (AC-7 of spec-bgp-pcap-decode).
// Method: build the expected octets by hand, so the test states the old format
// rather than asking the new code whether it agrees with itself.

//go:build ze_l2tp

package cmd

import (
	"bytes"
	"encoding/binary"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp"
)

func TestExportL2TPPcapUnchanged(t *testing.T) {
	stamp := time.Date(2026, 1, 1, 12, 0, 0, 250_000_000, time.UTC)
	first := []byte{0xC8, 0x02, 0x00, 0x14}
	second := []byte{0xC8, 0x02, 0x00, 0x18, 0x01}

	// The ring hands entries newest-first and the export walks them backwards.
	entries := []l2tp.RawCaptureEntry{
		{Timestamp: stamp.Add(time.Millisecond), Direction: 1, Data: second},
		{Timestamp: stamp, Direction: 0, Data: first},
	}

	got, err := exportL2TPPcap(entries)
	if err != nil {
		t.Fatalf("exportL2TPPcap: %v", err)
	}

	var want bytes.Buffer
	hdr := make([]byte, 24)
	binary.LittleEndian.PutUint32(hdr[0:4], 0xa1b2c3d4)
	binary.LittleEndian.PutUint16(hdr[4:6], 2)
	binary.LittleEndian.PutUint16(hdr[6:8], 4)
	binary.LittleEndian.PutUint32(hdr[16:20], 1500)
	binary.LittleEndian.PutUint32(hdr[20:24], 101)
	want.Write(hdr)
	for i, data := range [][]byte{first, second} {
		ts := stamp.Add(time.Duration(i) * time.Millisecond)
		rec := make([]byte, 16)
		binary.LittleEndian.PutUint32(rec[0:4], uint32(ts.Unix()))            //nolint:gosec // a fixed test timestamp
		binary.LittleEndian.PutUint32(rec[4:8], uint32(ts.Nanosecond()/1000)) //nolint:gosec // below one million
		binary.LittleEndian.PutUint32(rec[8:12], uint32(len(data)))           //nolint:gosec // a slice length
		binary.LittleEndian.PutUint32(rec[12:16], uint32(len(data)))          //nolint:gosec // a slice length
		want.Write(rec)
		want.Write(data)
	}

	if !bytes.Equal(got, want.Bytes()) {
		t.Errorf("the L2TP capture changed.\ngot:  % x\nwant: % x", got, want.Bytes())
	}
}
