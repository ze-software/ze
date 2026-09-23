// Design: docs/architecture/flowexport/flow-export-2-flow-records.md -- sFlow v5 exporter obligations
// Related: flow_adapter_test.go -- loopback target helpers reused here
//
// Tagged proofs for the sFlow v5 transport and sample-format sentences the
// 2026-09-21 extraction walk added (SFLOW-V5-x-25 and following). Each test
// decodes the datagram the exporter really sent, or the datagrams
// writeCounterDatagrams really produced, and asserts the on-wire fields.

package sflow

import (
	"bytes"
	"encoding/binary"
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/plugins/flowexport"
)

// sampleRecord is one decoded sample_record of a datagram.
type sampleRecord struct {
	format uint32
	data   []byte
}

// decodedDatagram is the sFlow v5 datagram header plus its sample records.
type decodedDatagram struct {
	subAgentID uint32
	numSamples uint32
	samples    []sampleRecord
}

// decodeDatagram splits an IPv4-agent datagram into its header fields and
// sample records. It fails the test on a truncated datagram.
func decodeDatagram(t *testing.T, dg []byte) decodedDatagram {
	t.Helper()
	if len(dg) < HeaderSizeIPv4 {
		t.Fatalf("datagram too short: %d bytes", len(dg))
	}
	if got := binary.BigEndian.Uint32(dg[4:]); got != AddressTypeIPv4 {
		t.Fatalf("address type = %d, want IPv4 (%d)", got, AddressTypeIPv4)
	}
	d := decodedDatagram{
		subAgentID: binary.BigEndian.Uint32(dg[12:]),
		numSamples: binary.BigEndian.Uint32(dg[24:]),
	}
	off := HeaderSizeIPv4
	for off < len(dg) {
		if off+8 > len(dg) {
			t.Fatalf("sample record header truncated at offset %d", off)
		}
		format := binary.BigEndian.Uint32(dg[off:])
		length := int(binary.BigEndian.Uint32(dg[off+4:]))
		off += 8
		if off+length > len(dg) {
			t.Fatalf("sample record of %d bytes overruns the datagram at offset %d", length, off)
		}
		d.samples = append(d.samples, sampleRecord{format: format, data: dg[off : off+length]})
		off += length
	}
	return d
}

// flowSampleFields are the fixed fields of an expanded flow sample plus its
// first flow record.
type flowSampleFields struct {
	seq          uint32
	sourceID     uint32
	pool         uint32
	numRecords   uint32
	recordFormat uint32
	header       []byte
}

// decodeFlowSample reads the expanded flow sample fields and, when a record
// follows, the sampled_header record's header bytes.
func decodeFlowSample(t *testing.T, data []byte) flowSampleFields {
	t.Helper()
	if len(data) < 44 {
		t.Fatalf("flow_sample too short: %d bytes", len(data))
	}
	if got := binary.BigEndian.Uint32(data[4:]); got != 0 {
		t.Fatalf("source type = %d, want ifIndex 0", got)
	}
	f := flowSampleFields{
		seq:        binary.BigEndian.Uint32(data[0:]),
		sourceID:   binary.BigEndian.Uint32(data[8:]),
		pool:       binary.BigEndian.Uint32(data[16:]),
		numRecords: binary.BigEndian.Uint32(data[40:]),
	}
	if f.numRecords == 0 {
		return f
	}
	rec := data[44:]
	if len(rec) < 24 {
		t.Fatalf("flow record too short: %d bytes", len(rec))
	}
	f.recordFormat = binary.BigEndian.Uint32(rec[0:])
	// record_length(4), header_protocol(4), frame_length(4), stripped(4),
	// then the XDR opaque header: count(4) + bytes.
	hdrLen := int(binary.BigEndian.Uint32(rec[20:]))
	if 24+hdrLen > len(rec) {
		t.Fatalf("sampled_header claims %d bytes, record holds %d", hdrLen, len(rec)-24)
	}
	f.header = rec[24 : 24+hdrLen]
	return f
}

// counterSourceID reads the source index of an expanded counter sample.
func counterSourceID(t *testing.T, data []byte) uint32 {
	t.Helper()
	if len(data) < 16 {
		t.Fatalf("counters_sample too short: %d bytes", len(data))
	}
	if got := binary.BigEndian.Uint32(data[4:]); got != 0 {
		t.Fatalf("source type = %d, want ifIndex 0", got)
	}
	return binary.BigEndian.Uint32(data[8:])
}

// countersFor builds n interface counter sets with ifIndex 1..n.
func countersFor(n int) []flowexport.InterfaceCounters {
	ifaces := make([]flowexport.InterfaceCounters, n)
	for i := range ifaces {
		ifaces[i].IfIndex = uint32(i + 1) //nolint:gosec // small test range
		ifaces[i].IfInOctets = uint64(i) * 1000
	}
	return ifaces
}

// countersPerDatagram is how many 120-byte counter samples fit behind a
// 28-byte IPv4 header inside MaxDatagramSize: 11, since 12 would need 1468.
const countersPerDatagram = (flowexport.MaxDatagramSize - HeaderSizeIPv4) / (counterSampleHeaderSize + ifCountersRecordHeaderSize + flowexport.IfCountersSize)

var testAgent = netip.MustParseAddr("10.0.0.1")

func encodeOne(t *testing.T, enc *FlowEncoder, s *flowexport.Sender, pc net.PacketConn, sample flowexport.FlowSample) decodedDatagram {
	t.Helper()
	if err := enc.EncodeFlowSample(sample, s); err != nil {
		t.Fatal(err)
	}
	return decodeDatagram(t, recvFlowDatagram(t, pc))
}

// RFC requirement: SFLOW-V5-x-25 positive -- a single sampled packet leaves the agent at once: after one EncodeFlowSample call the sender has transmitted exactly one datagram, and that datagram carries the sample (num_samples == 1, the flow_sample's source_id is the sampled ifIndex).
func TestSFlowV5SampleSentWithoutWaitingForBuffer(t *testing.T) {
	pc, s := newLoopbackFlowTarget(t)
	defer func() { _ = pc.Close() }()
	defer func() { _ = s.Close() }()

	enc := NewFlowEncoder(testAgent, 1, time.Unix(1716000000, 0))
	if err := enc.EncodeFlowSample(flowexport.FlowSample{IfIndex: 5, Rate: 256, OrigSize: 64, Header: []byte{1, 2, 3, 4}}, s); err != nil {
		t.Fatal(err)
	}
	datagrams, _, _ := s.Stats()
	if datagrams != 1 {
		t.Fatalf("datagrams sent after one sample = %d, want 1", datagrams)
	}
	dg := decodeDatagram(t, recvFlowDatagram(t, pc))
	if dg.numSamples != 1 || len(dg.samples) != 1 {
		t.Fatalf("num_samples = %d (%d records), want 1", dg.numSamples, len(dg.samples))
	}
	if got := decodeFlowSample(t, dg.samples[0].data).sourceID; got != 5 {
		t.Errorf("flow_sample source_id = %d, want 5", got)
	}
}

// RFC requirement: SFLOW-V5-x-25 negative -- no datagram is withheld to collect more samples: three consecutive samples produce three datagrams, the sender's datagram count advances by exactly one per sample, and a datagram carrying more than one flow_sample never appears.
func TestSFlowV5NoDatagramHeldForMoreSamples(t *testing.T) {
	pc, s := newLoopbackFlowTarget(t)
	defer func() { _ = pc.Close() }()
	defer func() { _ = s.Close() }()

	enc := NewFlowEncoder(testAgent, 1, time.Unix(1716000000, 0))
	for i := uint64(1); i <= 3; i++ {
		if err := enc.EncodeFlowSample(flowexport.FlowSample{IfIndex: 5, Rate: 256, OrigSize: 64, Header: []byte{9}}, s); err != nil {
			t.Fatal(err)
		}
		datagrams, _, _ := s.Stats()
		if datagrams != i {
			t.Fatalf("after sample %d: datagrams sent = %d, want %d", i, datagrams, i)
		}
		dg := decodeDatagram(t, recvFlowDatagram(t, pc))
		if dg.numSamples != 1 || len(dg.samples) != 1 {
			t.Errorf("datagram %d: num_samples = %d (%d records), want exactly 1", i, dg.numSamples, len(dg.samples))
		}
	}
}

// RFC requirement: SFLOW-V5-x-26 positive -- every outstanding counter of a poll is sent: 30 interfaces, more than two datagrams' worth, come out with each ifIndex present in exactly one counters_sample across the emitted datagrams.
func TestSFlowV5OutstandingCountersAllSent(t *testing.T) {
	const n = 30
	buf := make([]byte, flowexport.MaxDatagramSize)
	datagrams, _ := writeCounterDatagrams(buf, testAgent, 1, 0, 0, countersFor(n), map[uint32]uint32{})

	seen := make(map[uint32]int)
	for _, raw := range datagrams {
		dg := decodeDatagram(t, raw)
		for _, rec := range dg.samples {
			seen[counterSourceID(t, rec.data)]++
		}
	}
	for ifIndex := uint32(1); ifIndex <= n; ifIndex++ {
		if seen[ifIndex] != 1 {
			t.Errorf("ifIndex %d exported %d times, want exactly 1", ifIndex, seen[ifIndex])
		}
	}
	if len(seen) != n {
		t.Errorf("%d distinct sources exported, want %d", len(seen), n)
	}
}

// RFC requirement: SFLOW-V5-x-26 negative -- the partially filled tail datagram is not held back for a later poll: 30 counters fill two datagrams of 11 and a third of 8, so the third goes out with its 8 remaining samples and no counter stays outstanding after the call.
func TestSFlowV5TailDatagramNotWithheld(t *testing.T) {
	const n = 30
	buf := make([]byte, flowexport.MaxDatagramSize)
	datagrams, nextSeq := writeCounterDatagrams(buf, testAgent, 1, 0, 0, countersFor(n), map[uint32]uint32{})

	if len(datagrams) != 3 {
		t.Fatalf("datagrams = %d, want 3", len(datagrams))
	}
	want := []uint32{countersPerDatagram, countersPerDatagram, n - 2*countersPerDatagram}
	total := uint32(0)
	for i, raw := range datagrams {
		dg := decodeDatagram(t, raw)
		if dg.numSamples != want[i] {
			t.Errorf("datagram %d: num_samples = %d, want %d", i, dg.numSamples, want[i])
		}
		total += dg.numSamples
	}
	if total != n {
		t.Errorf("samples sent = %d, want %d (none outstanding)", total, n)
	}
	if nextSeq != 3 {
		t.Errorf("next datagram sequence = %d, want 3 (one per emitted datagram)", nextSeq)
	}
}

// RFC requirement: SFLOW-V5-x-27 positive -- one expanded encoding covers all interfaces: alternating low and full-width ifIndex values decode unchanged from counter format 4 and flow format 3.
// RFC requirement: SFLOW-V5-x-12 positive -- source indexes above 24 bits decode unchanged from expanded counter format 4 and expanded flow format 3, including 0xFFFFFFFF.
func TestSFlowV5ExpandedEncodingForEveryInterface(t *testing.T) {
	ifaces := []flowexport.InterfaceCounters{{IfIndex: 1}, {IfIndex: 0xFFFFFFFF}, {IfIndex: 200}, {IfIndex: 0x01000000}, {IfIndex: 70000}}
	buf := make([]byte, flowexport.MaxDatagramSize)
	datagrams, _ := writeCounterDatagrams(buf, testAgent, 1, 0, 0, ifaces, map[uint32]uint32{})
	if len(datagrams) != 1 {
		t.Fatalf("datagrams = %d, want 1", len(datagrams))
	}
	dg := decodeDatagram(t, datagrams[0])
	if len(dg.samples) != len(ifaces) {
		t.Fatalf("samples = %d, want %d", len(dg.samples), len(ifaces))
	}
	for i, rec := range dg.samples {
		if rec.format != 4 {
			t.Errorf("ifIndex %d: counters data_format = %d, want expanded 4", ifaces[i].IfIndex, rec.format)
		}
		if got := counterSourceID(t, rec.data); got != ifaces[i].IfIndex {
			t.Errorf("counter source index = %#x, want %#x", got, ifaces[i].IfIndex)
		}
	}

	pc, s := newLoopbackFlowTarget(t)
	defer func() { _ = pc.Close() }()
	defer func() { _ = s.Close() }()
	enc := NewFlowEncoder(testAgent, 1, time.Unix(1716000000, 0))
	for _, ifc := range ifaces {
		fdg := encodeOne(t, enc, s, pc, flowexport.FlowSample{IfIndex: ifc.IfIndex, Rate: 64, OrigSize: 60, Header: []byte{1}})
		if fdg.samples[0].format != 3 {
			t.Errorf("ifIndex %d: flow data_format = %d, want expanded 3", ifc.IfIndex, fdg.samples[0].format)
		}
		if got := decodeFlowSample(t, fdg.samples[0].data).sourceID; got != ifc.IfIndex {
			t.Errorf("flow source index = %#x, want %#x", got, ifc.IfIndex)
		}
	}
}

// RFC requirement: SFLOW-V5-x-27 negative -- compact formats never accompany expanded formats: alternating low and full-width interface IDs produce only expanded samples across counter and flow datagrams.
func TestSFlowV5CompactFormatsNeverMixedIn(t *testing.T) {
	ifaces := []flowexport.InterfaceCounters{{IfIndex: 1}, {IfIndex: 0xFFFFFFFF}, {IfIndex: 200}}
	buf := make([]byte, flowexport.MaxDatagramSize)
	datagrams, _ := writeCounterDatagrams(buf, testAgent, 1, 0, 0, ifaces, map[uint32]uint32{})

	pc, s := newLoopbackFlowTarget(t)
	defer func() { _ = pc.Close() }()
	defer func() { _ = s.Close() }()
	enc := NewFlowEncoder(testAgent, 1, time.Unix(1716000000, 0))
	for _, ifc := range ifaces {
		if err := enc.EncodeFlowSample(flowexport.FlowSample{IfIndex: ifc.IfIndex, Rate: 64, OrigSize: 60, Header: []byte{1}}, s); err != nil {
			t.Fatal(err)
		}
		datagrams = append(datagrams, recvFlowDatagram(t, pc))
	}

	records := 0
	for i, raw := range datagrams {
		for _, rec := range decodeDatagram(t, raw).samples {
			records++
			if rec.format == 1 || rec.format == 2 {
				t.Errorf("datagram %d: compact data_format %d emitted", i, rec.format)
			}
		}
	}
	if records != 6 {
		t.Errorf("sample records inspected = %d, want 6", records)
	}
}

// RFC requirement: SFLOW-V5-x-28 positive -- a reset of the sample_pool and of the sequence_number happen together: a fresh encoder's first flow_sample decodes with sequence_number 1 and sample_pool equal to one rate's worth of packets.
func TestSFlowV5PoolAndSequenceResetTogether(t *testing.T) {
	pc, s := newLoopbackFlowTarget(t)
	defer func() { _ = pc.Close() }()
	defer func() { _ = s.Close() }()

	const rate = 512
	enc := NewFlowEncoder(testAgent, 1, time.Unix(1716000000, 0))
	fs := decodeFlowSample(t, encodeOne(t, enc, s, pc, flowexport.FlowSample{IfIndex: 3, Rate: rate, OrigSize: 60, Header: []byte{1}}).samples[0].data)
	if fs.seq != 1 {
		t.Errorf("sequence_number = %d, want 1 after reset", fs.seq)
	}
	if fs.pool != rate {
		t.Errorf("sample_pool = %d, want %d (one sample at rate %d) after reset", fs.pool, rate, rate)
	}
}

// RFC requirement: SFLOW-V5-x-28 negative -- a sample_pool reset with the sequence_number carrying on never appears: after three samples on one encoder, a second encoder (the only reset path) starts at pool == rate with sequence_number 1 rather than 4, and the first encoder's fourth sample keeps pool == 4*rate with sequence_number 4.
func TestSFlowV5PoolNeverResetsWithoutSequence(t *testing.T) {
	pc, s := newLoopbackFlowTarget(t)
	defer func() { _ = pc.Close() }()
	defer func() { _ = s.Close() }()

	const rate = 512
	sample := flowexport.FlowSample{IfIndex: 3, Rate: rate, OrigSize: 60, Header: []byte{1}}
	first := NewFlowEncoder(testAgent, 1, time.Unix(1716000000, 0))
	for range 3 {
		encodeOne(t, first, s, pc, sample)
	}

	second := NewFlowEncoder(testAgent, 1, time.Unix(1716000000, 0))
	fs := decodeFlowSample(t, encodeOne(t, second, s, pc, sample).samples[0].data)
	if fs.pool != rate || fs.seq != 1 {
		t.Errorf("reset encoder: sample_pool = %d, sequence_number = %d, want %d and 1", fs.pool, fs.seq, rate)
	}

	fs = decodeFlowSample(t, encodeOne(t, first, s, pc, sample).samples[0].data)
	if fs.seq != 4 || fs.pool != 4*rate {
		t.Errorf("continuing encoder: sample_pool = %d, sequence_number = %d, want %d and 4", fs.pool, fs.seq, 4*rate)
	}
}

// RFC requirement: SFLOW-V5-x-31 positive -- two data sources of one sub-agent carry that sub-agent: flow samples from ifIndex 3 and ifIndex 9 through an encoder built with sub_agent_id 7 both decode with sub_agent_id 7 in the datagram header.
func TestSFlowV5DataSourcesCarryTheirSubAgent(t *testing.T) {
	pc, s := newLoopbackFlowTarget(t)
	defer func() { _ = pc.Close() }()
	defer func() { _ = s.Close() }()

	enc := NewFlowEncoder(testAgent, 7, time.Unix(1716000000, 0))
	for _, ifIndex := range []uint32{3, 9} {
		dg := encodeOne(t, enc, s, pc, flowexport.FlowSample{IfIndex: ifIndex, Rate: 64, OrigSize: 60, Header: []byte{1}})
		if dg.subAgentID != 7 {
			t.Errorf("ifIndex %d: sub_agent_id = %d, want 7", ifIndex, dg.subAgentID)
		}
		if got := decodeFlowSample(t, dg.samples[0].data).sourceID; got != ifIndex {
			t.Errorf("source_id = %d, want %d", got, ifIndex)
		}
	}
}

// RFC requirement: SFLOW-V5-x-31 negative -- the data-source-to-sub-agent binding never changes within a session: five successive datagrams for ifIndex 3 all carry sub_agent_id 7, and a second session built with sub_agent_id 8 carries 8, so the value is the session's own and not a constant.
func TestSFlowV5SubAgentBindingConstantForSession(t *testing.T) {
	pc, s := newLoopbackFlowTarget(t)
	defer func() { _ = pc.Close() }()
	defer func() { _ = s.Close() }()

	sample := flowexport.FlowSample{IfIndex: 3, Rate: 64, OrigSize: 60, Header: []byte{1}}
	session := NewFlowEncoder(testAgent, 7, time.Unix(1716000000, 0))
	for i := range 5 {
		if got := encodeOne(t, session, s, pc, sample).subAgentID; got != 7 {
			t.Errorf("datagram %d: sub_agent_id = %d, want 7 for the whole session", i, got)
		}
	}
	other := NewFlowEncoder(testAgent, 8, time.Unix(1716000000, 0))
	if got := encodeOne(t, other, s, pc, sample).subAgentID; got != 8 {
		t.Errorf("second session: sub_agent_id = %d, want 8", got)
	}
}

// RFC requirement: SFLOW-V5-x-33 positive -- the flow_sample carries the packet header: it decodes with flow_records == 1, the record's data_format is sampled_header (1), and the record's header bytes equal the 14 captured bytes.
func TestSFlowV5FlowSampleCarriesPacketHeader(t *testing.T) {
	pc, s := newLoopbackFlowTarget(t)
	defer func() { _ = pc.Close() }()
	defer func() { _ = s.Close() }()

	header := []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x02, 0x00, 0x00, 0x00, 0x00, 0x01, 0x08, 0x00}
	enc := NewFlowEncoder(testAgent, 1, time.Unix(1716000000, 0))
	fs := decodeFlowSample(t, encodeOne(t, enc, s, pc, flowexport.FlowSample{IfIndex: 3, Rate: 64, OrigSize: 60, Header: header}).samples[0].data)
	if fs.numRecords != 1 {
		t.Fatalf("flow_records = %d, want 1", fs.numRecords)
	}
	if fs.recordFormat != DataFormatSampledHeader {
		t.Errorf("record data_format = %d, want sampled_header %d", fs.recordFormat, DataFormatSampledHeader)
	}
	if !bytes.Equal(fs.header, header) {
		t.Errorf("header bytes = % x, want % x", fs.header, header)
	}
}

// RFC requirement: SFLOW-V5-x-33 negative -- a flow_sample without header information never appears: a 1500-byte capture that cannot fit the datagram is truncated to the remaining space, and the flow_sample still decodes with flow_records == 1, a sampled_header record and a non-empty header prefix of the capture.
func TestSFlowV5FlowSampleNeverWithoutHeader(t *testing.T) {
	pc, s := newLoopbackFlowTarget(t)
	defer func() { _ = pc.Close() }()
	defer func() { _ = s.Close() }()

	header := make([]byte, 1500)
	for i := range header {
		header[i] = byte(i)
	}
	enc := NewFlowEncoder(testAgent, 1, time.Unix(1716000000, 0))
	dg := encodeOne(t, enc, s, pc, flowexport.FlowSample{IfIndex: 3, Rate: 64, OrigSize: 1500, Header: header})
	if dg.numSamples != 1 {
		t.Fatalf("num_samples = %d, want 1", dg.numSamples)
	}
	fs := decodeFlowSample(t, dg.samples[0].data)
	if fs.numRecords != 1 || fs.recordFormat != DataFormatSampledHeader {
		t.Fatalf("flow_records = %d, data_format = %d, want 1 and sampled_header %d", fs.numRecords, fs.recordFormat, DataFormatSampledHeader)
	}
	if len(fs.header) == 0 || len(fs.header) >= len(header) {
		t.Fatalf("header carried = %d bytes, want a non-empty truncated prefix of %d", len(fs.header), len(header))
	}
	if !bytes.Equal(fs.header, header[:len(fs.header)]) {
		t.Errorf("carried header is not a prefix of the capture")
	}
}
