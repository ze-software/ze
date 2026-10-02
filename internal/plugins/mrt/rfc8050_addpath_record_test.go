package mrt

import (
	"bytes"
	"net/netip"
	"os"
	"path/filepath"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	mrtfmt "github.com/ze-software/ze/internal/mrt"
)

// TestRFC8050AddPathRecordAddsNoFieldToTheBaseLayout proves that Ze's MRT
// writer, given an UPDATE whose NLRI carries a Path Identifier, writes an
// add-path BGP4MP record whose fields are exactly the base subtype's fields,
// followed by the whole message.
//
// Method: the input is forced toward the violation: the UPDATE's NLRI holds
// Path Identifier 1 before 10.0.0.0/24, which a writer could be tempted to
// lift into the record. The component is configured for add-path, records one
// received and one sent copy, and the dump file is compared octet by octet,
// after the 4-octet timestamp, with the literal records: type 16, subtype 9
// (received) or 11 (sent), length 65, the BGP4MP_MESSAGE_AS4 fields of RFC
// 6396 Section 4.4.3 and nothing else before the 45-octet UPDATE.
//
// RFC requirement: RFC8050-x-3 negative -- with an UPDATE whose NLRI carries a
// Path Identifier, the written BGP4MP_MESSAGE_AS4_ADDPATH and
// BGP4MP_MESSAGE_AS4_LOCAL_ADDPATH records hold no field beyond the base
// subtype's (record length 65 = 20 field octets + 45), and the BGP message
// field is the entire UPDATE, Path Identifier included.
func TestRFC8050AddPathRecordAddsNoFieldToTheBaseLayout(t *testing.T) {
	update := []byte{
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
		0x00, 0x2d, 0x02,
		0x00, 0x00,
		0x00, 0x0e,
		0x40, 0x01, 0x01, 0x00,
		0x40, 0x02, 0x00,
		0x40, 0x03, 0x04, 0xc0, 0x00, 0x02, 0x01,
		0x00, 0x00, 0x00, 0x01, 0x18, 0x0a, 0x00, 0x00,
	}
	fields := []byte{
		0x00, 0x00, 0xfd, 0xe9,
		0x00, 0x00, 0xfd, 0xe8,
		0x00, 0x00,
		0x00, 0x01,
		0xc0, 0x00, 0x02, 0x01,
		0xc0, 0x00, 0x02, 0x02,
	}

	c := New(Config{AddPath: true}, nil)
	path := filepath.Join(t.TempDir(), "all.mrt")
	c.allMsgs = newAsyncWriter(mrtfmt.NewWriter(path), c.logger)
	peer := &plugin.PeerInfo{
		Address:      netip.MustParseAddr("192.0.2.1"),
		LocalAddress: netip.MustParseAddr("192.0.2.2"),
		PeerAS:       65001,
		LocalAS:      65000,
	}
	c.OnBGPMessage(peer, msgtype.TypeUPDATE, false, update)
	c.OnBGPMessage(peer, msgtype.TypeUPDATE, true, update)
	if err := c.allMsgs.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	dump, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read dump: %v", err)
	}
	recordLen := mrtfmt.CommonHeaderLen + len(fields) + len(update)
	if len(dump) != 2*recordLen {
		t.Fatalf("dump = %d octets, want two records of %d", len(dump), recordLen)
	}
	for i, subtype := range []byte{0x09, 0x0b} {
		record := dump[i*recordLen : (i+1)*recordLen]
		want := []byte{0x00, 0x10, 0x00, subtype, 0x00, 0x00, 0x00, 0x41}
		want = append(want, fields...)
		want = append(want, update...)
		if !bytes.Equal(record[4:], want) {
			t.Errorf("record %d after the timestamp = % x, want % x", i, record[4:], want)
		}
	}
}
