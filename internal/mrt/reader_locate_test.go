// VALIDATES: a decode failure names WHICH record failed -- its 1-based ordinal
// in the stream, plus type, subtype and timestamp.
// PREVENTS: the bare propagated error. A decode error carries an offset inside
// the record's own fields ("peer entry 3 ip: mrt: short data"), which on a
// multi-GB RIB dump locates nothing: the user cannot find the record, cannot
// trim the file, and cannot tell the collector operator what to look at
// (ai/rules/cli.md, leg 3 "what to do next").
package mrt

import (
	"bytes"
	"strings"
	"testing"
)

// recordWithBody frames a payload in the MRT common header.
func recordWithBody(ts uint32, typ, subtype uint16, body []byte) []byte {
	rec := make([]byte, CommonHeaderLen+len(body))
	WriteCommonHeader(rec, 0, ts, typ, subtype, uint32(len(body)))
	copy(rec[CommonHeaderLen:], body)
	return rec
}

func TestReadFrom_DecodeErrorNamesTheRecord(t *testing.T) {
	// Three records: two that decode, then one whose PEER_INDEX_TABLE body is
	// too short. The error must point at record 3, not merely say "short data".
	// A record type the handler does not subscribe to: dispatch skips it, so it
	// contributes an ordinal and nothing else.
	good := recordWithBody(1000, TypeTableDump, TableDumpAFIIPv4, nil)
	bad := recordWithBody(4242, TypeTableDumpV2, TDV2PeerIndexTable, []byte{0, 1, 2})

	stream := make([]byte, 0, len(good)*2+len(bad))
	stream = append(stream, good...)
	stream = append(stream, good...)
	stream = append(stream, bad...)

	handler := &Handler{
		OnPeerIndex: func(Header, *PeerIndexTable) error { return nil },
	}

	err := ReadFrom(bytes.NewReader(stream), handler)
	if err == nil {
		t.Fatal("a malformed PEER_INDEX_TABLE must be reported")
	}

	msg := err.Error()
	for _, want := range []string{"record 3", "4242"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q must locate the failing record: missing %q", msg, want)
		}
	}
	// The underlying cause must survive the wrapping.
	if !strings.Contains(msg, "short data") {
		t.Errorf("error %q dropped its cause", msg)
	}
}

func TestReadFrom_TruncatedRecordNamesTheRecord(t *testing.T) {
	// A record whose declared length runs past the end of the stream.
	good := recordWithBody(1000, TypeTableDump, TableDumpAFIIPv4, nil)
	truncated := make([]byte, CommonHeaderLen)
	WriteCommonHeader(truncated, 0, 7777, TypeTableDumpV2, TDV2RIBIPv4Unicast, 64)

	stream := append(append([]byte{}, good...), truncated...)

	err := ReadFrom(bytes.NewReader(stream), &Handler{})
	if err == nil {
		t.Fatal("a truncated record must be reported")
	}
	msg := err.Error()
	if !strings.Contains(msg, "record 2") {
		t.Errorf("error %q must name the truncated record's ordinal", msg)
	}
	if !strings.Contains(msg, "7777") {
		t.Errorf("error %q must carry the record timestamp", msg)
	}
}
