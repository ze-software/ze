// Goal: prove the reader accepts every classic pcap a colleague's tcpdump can
// produce -- either byte order, either timestamp resolution -- and refuses a
// crafted record before it sizes a buffer.
// Method: build files octet by octet, so the test states the format rather than
// depending on the writer to agree with itself.

package pcap

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"testing"
	"time"
)

// buildFile writes a pcap file with the given magic, byte order and records.
// The magic decides both the endianness a reader must detect and whether the
// second timestamp field counts microseconds or nanoseconds.
func buildFile(order binary.ByteOrder, magic, linkType uint32, records [][]byte, stamps []time.Time) []byte {
	var buf bytes.Buffer
	hdr := make([]byte, FileHeaderLen)
	order.PutUint32(hdr[0:4], magic)
	order.PutUint16(hdr[4:6], versionMajor)
	order.PutUint16(hdr[6:8], versionMinor)
	order.PutUint32(hdr[16:20], 65535)
	order.PutUint32(hdr[20:24], linkType)
	buf.Write(hdr)

	for i, data := range records {
		rec := make([]byte, RecordHeaderLen)
		fraction := uint32(stamps[i].Nanosecond() / 1000)
		if magic == magicNano {
			fraction = uint32(stamps[i].Nanosecond())
		}
		order.PutUint32(rec[0:4], uint32(stamps[i].Unix()))
		order.PutUint32(rec[4:8], fraction)
		order.PutUint32(rec[8:12], uint32(len(data)))
		order.PutUint32(rec[12:16], uint32(len(data)))
		buf.Write(rec)
		buf.Write(data)
	}
	return buf.Bytes()
}

func TestReadPcapRecords(t *testing.T) {
	records := [][]byte{{0x01, 0x02}, {0x03, 0x04, 0x05}}
	stamps := []time.Time{
		time.Unix(1_700_000_000, 250_000_000).UTC(),
		time.Unix(1_700_000_001, 750_000_000).UTC(),
	}

	cases := []struct {
		name  string
		order binary.ByteOrder
		magic uint32
	}{
		{"little-endian microseconds", binary.LittleEndian, magicMicro},
		{"big-endian microseconds", binary.BigEndian, magicMicro},
		{"little-endian nanoseconds", binary.LittleEndian, magicNano},
		{"big-endian nanoseconds", binary.BigEndian, magicNano},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			file := buildFile(tc.order, tc.magic, LinkTypeRaw, records, stamps)
			reader, err := NewReader(bytes.NewReader(file))
			if err != nil {
				t.Fatalf("NewReader: %v", err)
			}
			if reader.LinkType() != LinkTypeRaw {
				t.Errorf("link type = %d, want %d", reader.LinkType(), LinkTypeRaw)
			}
			if reader.SnapLen() != 65535 {
				t.Errorf("snaplen = %d, want 65535", reader.SnapLen())
			}

			var record Record
			for i := range records {
				if err := reader.Next(&record); err != nil {
					t.Fatalf("record %d: %v", i, err)
				}
				if !bytes.Equal(record.Data, records[i]) {
					t.Errorf("record %d bytes = % x, want % x", i, record.Data, records[i])
				}
				if !record.Timestamp.Equal(stamps[i]) {
					t.Errorf("record %d timestamp = %s, want %s", i, record.Timestamp, stamps[i])
				}
			}
			if err := reader.Next(&record); !errors.Is(err, io.EOF) {
				t.Errorf("after the last record: %v, want io.EOF", err)
			}
		})
	}
}

func TestReadPcapRejectsBadMagic(t *testing.T) {
	file := make([]byte, FileHeaderLen)
	binary.LittleEndian.PutUint32(file[0:4], 0xDEADBEEF)
	if _, err := NewReader(bytes.NewReader(file)); !errors.Is(err, ErrBadMagic) {
		t.Errorf("NewReader over a non-pcap file: %v, want ErrBadMagic", err)
	}
}

// TestReadPcapRejectsOversizeRecord proves the reader refuses an
// attacker-controlled record length before it allocates a buffer for it (R-7).
func TestReadPcapRejectsOversizeRecord(t *testing.T) {
	var buf bytes.Buffer
	hdr := make([]byte, FileHeaderLen)
	binary.LittleEndian.PutUint32(hdr[0:4], magicMicro)
	binary.LittleEndian.PutUint32(hdr[20:24], LinkTypeRaw)
	buf.Write(hdr)

	rec := make([]byte, RecordHeaderLen)
	binary.LittleEndian.PutUint32(rec[8:12], RecordBytesMax+1)
	binary.LittleEndian.PutUint32(rec[12:16], RecordBytesMax+1)
	buf.Write(rec)

	reader, err := NewReader(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	var record Record
	if err := reader.Next(&record); err == nil {
		t.Fatal("Next accepted a record declaring more than RecordBytesMax")
	}
}

// TestReadPcapRejectsShortOriginal proves a record claiming fewer on-network
// bytes than it carries is refused rather than read as truncation in reverse.
func TestReadPcapRejectsShortOriginal(t *testing.T) {
	var buf bytes.Buffer
	hdr := make([]byte, FileHeaderLen)
	binary.LittleEndian.PutUint32(hdr[0:4], magicMicro)
	binary.LittleEndian.PutUint32(hdr[20:24], LinkTypeRaw)
	buf.Write(hdr)

	rec := make([]byte, RecordHeaderLen)
	binary.LittleEndian.PutUint32(rec[8:12], 10)
	binary.LittleEndian.PutUint32(rec[12:16], 4)
	buf.Write(rec)
	buf.Write(make([]byte, 10))

	reader, err := NewReader(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	var record Record
	if err := reader.Next(&record); err == nil {
		t.Fatal("Next accepted a record whose original length is below its captured length")
	}
}

// TestReadWriteRoundTrip proves the writer and the reader agree, which is the
// contract the capture round trip rests on.
func TestReadWriteRoundTrip(t *testing.T) {
	payloads := [][]byte{{0xAA}, {0xBB, 0xCC}, bytes.Repeat([]byte{0xDD}, 1000)}

	var file bytes.Buffer
	if err := WriteFileHeader(&file, 65535, LinkTypeRaw); err != nil {
		t.Fatalf("WriteFileHeader: %v", err)
	}
	for _, payload := range payloads {
		if err := WriteRecord(&file, testStamp, payload, len(payload)+7); err != nil {
			t.Fatalf("WriteRecord: %v", err)
		}
	}

	reader, err := NewReader(bytes.NewReader(file.Bytes()))
	if err != nil {
		t.Fatalf("NewReader: %v", err)
	}
	var record Record
	for i, payload := range payloads {
		if err := reader.Next(&record); err != nil {
			t.Fatalf("record %d: %v", i, err)
		}
		if !bytes.Equal(record.Data, payload) {
			t.Errorf("record %d bytes = % x, want % x", i, record.Data, payload)
		}
		if record.OriginalLen != len(payload)+7 {
			t.Errorf("record %d original length = %d, want %d", i, record.OriginalLen, len(payload)+7)
		}
	}
	if err := reader.Next(&record); !errors.Is(err, io.EOF) {
		t.Errorf("after the last record: %v, want io.EOF", err)
	}
}
