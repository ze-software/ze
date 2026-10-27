// Design: docs/architecture/testing/interop.md -- native DelayOpen capture boundaries.
package bgp

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net/netip"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/pcap"
	"github.com/ze-software/ze/internal/le/interoplab"
)

// VALIDATES: complete timestamped TCP history discriminates the two permitted
// OPEN triggers and refuses missing evidence rather than accepting Established.
// PREVENTS: premature OPEN, a late capture, or a truncated capture passing.
func TestDelayOpenWireBoundaries(t *testing.T) {
	for _, test := range []struct {
		name                   string
		zeOpen, frrOpen        time.Duration
		early                  bool
		omitSYN, truncate, gap bool
		want                   string
	}{
		{name: "expiry", zeOpen: 5 * time.Second, frrOpen: 5*time.Second + time.Millisecond},
		{name: "early", zeOpen: time.Second + time.Millisecond, frrOpen: time.Second, early: true},
		{name: "premature", zeOpen: time.Second, frrOpen: time.Second + time.Millisecond, want: "premature Ze OPEN"},
		{name: "early did not release", zeOpen: 20 * time.Second, frrOpen: time.Second, early: true, want: "early OPEN did not release"},
		{name: "missing SYN", zeOpen: 5 * time.Second, frrOpen: 6 * time.Second, omitSYN: true, want: "handshake"},
		{name: "truncated pcap", zeOpen: 5 * time.Second, frrOpen: 6 * time.Second, truncate: true, want: ""},
		{name: "missing first byte", zeOpen: 5 * time.Second, frrOpen: 6 * time.Second, gap: true, want: "initial TCP payload"},
	} {
		t.Run(test.name, func(t *testing.T) {
			capture := delayOpenTestCapture(t, test.zeOpen, test.frrOpen, test.omitSYN, test.gap)
			if test.truncate {
				capture = capture[:len(capture)-1]
			}
			history, err := delayOpenHistory(capture, netip.MustParseAddr("172.30.0.2"))
			if err == nil {
				zeDelay, frrDelay := 5*time.Second, 30*time.Second
				if test.early {
					zeDelay, frrDelay = 20*time.Second, time.Second
				}
				_, err = judgeDelayOpen(history, netip.MustParseAddr("172.30.0.3"), zeDelay, frrDelay, test.early)
			}
			if test.want == "" && !test.truncate {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || (test.want != "" && !strings.Contains(err.Error(), test.want)) {
				t.Fatalf("error = %v, want %q", err, test.want)
			}
		})
	}
}

// VALIDATES: kernel-drop and capture-bound evidence is mandatory.
// PREVENTS: a negative timing verdict over an incomplete capture.
func TestDelayOpenCaptureCompletion(t *testing.T) {
	for _, test := range []struct {
		log   string
		valid bool
	}{
		{"12 packets captured\n12 packets received by filter\n0 packets dropped by kernel\n", true},
		{"12 packets captured\n1 packets dropped by kernel\n", false},
		{"4096 packets captured\n0 packets dropped by kernel\n", false},
		{"listening on eth0\n", false},
		{"0 packets captured\n0 packets dropped by kernel\n", false},
	} {
		if _, err := delayOpenCaptureComplete(test.log); (err == nil) != test.valid {
			t.Errorf("log %q: %v", test.log, err)
		}
	}
}

func delayOpenTestCapture(t *testing.T, zeOpen, frrOpen time.Duration, omitSYN, gap bool) []byte {
	t.Helper()
	var out bytes.Buffer
	if err := pcap.WriteFileHeader(&out, 65535, pcap.LinkTypeEthernet); err != nil {
		t.Fatal(err)
	}
	start := time.Unix(1700000000, 0)
	packet := func(at time.Duration, reverse bool, seq, ack uint32, flags byte, body []byte) {
		raw := make([]byte, 54+len(body))
		binary.BigEndian.PutUint16(raw[12:14], 0x0800)
		ip := raw[14:34]
		ip[0], ip[9] = 0x45, 6
		binary.BigEndian.PutUint16(ip[2:4], uint16(40+len(body)))
		copy(ip[12:16], []byte{172, 30, 0, 2})
		copy(ip[16:20], []byte{172, 30, 0, 3})
		tcp := raw[34:]
		binary.BigEndian.PutUint16(tcp[:2], 40000)
		binary.BigEndian.PutUint16(tcp[2:4], 179)
		if reverse {
			ip[15], ip[19] = 3, 2
			binary.BigEndian.PutUint16(tcp[:2], 179)
			binary.BigEndian.PutUint16(tcp[2:4], 40000)
		}
		binary.BigEndian.PutUint32(tcp[4:8], seq)
		binary.BigEndian.PutUint32(tcp[8:12], ack)
		tcp[12], tcp[13] = 0x50, flags
		copy(tcp[20:], body)
		if err := pcap.WriteRecord(&out, start.Add(at), raw, len(raw)); err != nil {
			t.Fatal(err)
		}
	}
	if !omitSYN {
		packet(0, false, 100, 0, 2, nil)
	}
	packet(time.Millisecond, true, 200, 101, 18, nil)
	packet(2*time.Millisecond, false, 101, 201, 16, nil)
	open := speakerMessage(bgpOpen, []byte{4, 253, 232, 0, 90, 172, 30, 0, 2, 0})
	keepalive := speakerMessage(bgpKeepalive, nil)
	zeSeq := uint32(101)
	if gap {
		zeSeq++
	}
	// File order is capture order, independent of which direction speaks first.
	if zeOpen <= frrOpen {
		packet(zeOpen+2*time.Millisecond, false, zeSeq, 201, 24, open)
		packet(frrOpen+2*time.Millisecond, true, 201, zeSeq+uint32(len(open)), 24, open)
	} else {
		packet(frrOpen+2*time.Millisecond, true, 201, zeSeq, 24, open)
		packet(zeOpen+2*time.Millisecond, false, zeSeq, 201+uint32(len(open)), 24, open)
	}
	at := max(zeOpen, frrOpen) + 3*time.Millisecond
	packet(at, false, zeSeq+uint32(len(open)), 201+uint32(len(open)), 24, keepalive)
	packet(at+time.Millisecond, true, 201+uint32(len(open)), zeSeq+uint32(len(open)+len(keepalive)), 24, keepalive)
	return out.Bytes()
}

// VALIDATES: the oracle reads reassembled BGP, not one message per packet.
// PREVENTS: legitimate segmentation/retransmission becoming a timing failure,
// or a missing segment disappearing from a negative assertion.
func TestDelayOpenTCPReassembly(t *testing.T) {
	for _, test := range []struct {
		name               string
		missing, reordered bool
	}{
		{name: "split and retransmitted"},
		{name: "missing segment", missing: true},
		{name: "out of order", reordered: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			original := delayOpenTestCapture(t, 5*time.Second, 5*time.Second+time.Millisecond, false, false)
			reader, err := pcap.NewReader(bytes.NewReader(original))
			if err != nil {
				t.Fatal(err)
			}
			var out bytes.Buffer
			if err := pcap.WriteFileHeader(&out, 65535, pcap.LinkTypeEthernet); err != nil {
				t.Fatal(err)
			}
			var record pcap.Record
			for {
				err := reader.Next(&record)
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatal(err)
				}
				raw := record.Data
				if len(raw) == 83 && raw[29] == 2 {
					// Split Ze's OPEN in its header and retransmit the first
					// segment; the reordered case receives the tail first.
					first := bytes.Clone(raw[:64])
					binary.BigEndian.PutUint16(first[16:18], 50)
					second := append(bytes.Clone(raw[:54]), raw[64:]...)
					binary.BigEndian.PutUint16(second[16:18], uint16(len(second)-14))
					binary.BigEndian.PutUint32(second[38:42], binary.BigEndian.Uint32(raw[38:42])+10)
					if test.reordered {
						if err := pcap.WriteRecord(&out, record.Timestamp, second, len(second)); err != nil {
							t.Fatal(err)
						}
						record.Timestamp = record.Timestamp.Add(time.Microsecond)
					}
					for _, part := range [][]byte{first, first} {
						if err := pcap.WriteRecord(&out, record.Timestamp, part, len(part)); err != nil {
							t.Fatal(err)
						}
					}
					if !test.missing && !test.reordered {
						if err := pcap.WriteRecord(&out, record.Timestamp, second, len(second)); err != nil {
							t.Fatal(err)
						}
					}
				} else if err := pcap.WriteRecord(&out, record.Timestamp, raw, len(raw)); err != nil {
					t.Fatal(err)
				}
			}
			history, err := delayOpenHistory(out.Bytes(), netip.MustParseAddr("172.30.0.2"))
			if err == nil {
				_, err = judgeDelayOpen(history, netip.MustParseAddr("172.30.0.3"), 5*time.Second, 30*time.Second, false)
			}
			if test.reordered && err == nil {
				open := history[0].Ze[0]
				if open.Complete.Sub(open.First) != time.Microsecond {
					t.Fatalf("out-of-order OPEN boundaries: %+v", open)
				}
			}
			if test.missing {
				if err == nil || !strings.Contains(err.Error(), "reassembly") {
					t.Fatalf("missing segment: %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
		})
	}
}

// VALIDATES: timing covers a collision loser as well as the eventual winner;
// overlapping connections are reported only when both handshakes were seen.
// PREVENTS: selecting the good connection and hiding a premature OPEN elsewhere.
func TestDelayOpenChecksLosingConnections(t *testing.T) {
	ze, peer := netip.MustParseAddr("172.30.0.2"), netip.MustParseAddr("172.30.0.3")
	history, err := delayOpenHistory(delayOpenTestCapture(t, 5*time.Second, 5*time.Second+time.Millisecond, false, false), ze)
	if err != nil {
		t.Fatal(err)
	}
	winner := history[0]
	loser := &delayOpenConnection{
		Flow:      pcap.Flow{SourceAddr: ze, TargetAddr: peer, SourcePort: 179, TargetPort: 40001},
		Initiator: peer,
		Handshake: winner.Handshake.Add(time.Millisecond),
		Closed:    winner.Handshake.Add(time.Second),
	}
	history = append(history, loser)
	observation, err := judgeDelayOpen(history, peer, 5*time.Second, 30*time.Second, false)
	if err != nil || !strings.Contains(observation, "opposite-initiator overlaps=1") {
		t.Fatalf("observed overlap: %q, %v", observation, err)
	}
	loser.Ze = []delayOpenMessage{{Type: bgpOpen, First: loser.Handshake.Add(time.Millisecond), Complete: loser.Handshake.Add(time.Millisecond)}}
	if _, err := judgeDelayOpen(history, peer, 5*time.Second, 30*time.Second, false); err == nil || !strings.Contains(err.Error(), "premature Ze OPEN") {
		t.Fatalf("losing connection's premature OPEN: %v", err)
	}
}

// VALIDATES: message completion uses the first arrival of every byte, including
// when a later retransmission covers a wider sequence range than an earlier one.
// PREVENTS: a legitimate early-OPEN reply being called premature because a full
// retransmission overwrote the original OPEN tail's timestamp.
func TestDelayOpenOverlappingRetransmissionKeepsFirstArrival(t *testing.T) {
	records := delayOpenTestRecords(t, delayOpenTestCapture(t, time.Second+2*time.Millisecond, time.Second, false, false))
	var out bytes.Buffer
	if err := pcap.WriteFileHeader(&out, 65535, pcap.LinkTypeEthernet); err != nil {
		t.Fatal(err)
	}
	var retransmission pcap.Record
	var completed time.Time
	write := func(record pcap.Record) {
		if err := pcap.WriteRecord(&out, record.Timestamp, record.Data, len(record.Data)); err != nil {
			t.Fatal(err)
		}
	}
	for _, record := range records {
		raw := record.Data
		if len(raw) != 83 || raw[29] != 3 {
			write(record)
			continue
		}
		retransmission = record
		retransmission.Timestamp = record.Timestamp.Add(500 * time.Millisecond)
		first := bytes.Clone(raw[:64])
		binary.BigEndian.PutUint16(first[16:18], 50)
		second := append(bytes.Clone(raw[:54]), raw[64:]...)
		binary.BigEndian.PutUint16(second[16:18], uint16(len(second)-14))
		binary.BigEndian.PutUint32(second[38:42], binary.BigEndian.Uint32(raw[38:42])+10)
		write(pcap.Record{Timestamp: record.Timestamp, Data: first})
		completed = record.Timestamp.Add(time.Millisecond)
		write(pcap.Record{Timestamp: completed, Data: second})
	}
	if retransmission.Data == nil {
		t.Fatal("fixture did not contain FRR OPEN")
	}
	write(retransmission)
	history, err := delayOpenHistory(out.Bytes(), netip.MustParseAddr("172.30.0.2"))
	if err != nil {
		t.Fatal(err)
	}
	if got := history[0].FRR[0].Complete; !got.Equal(completed) {
		t.Fatalf("FRR OPEN first-arrival completion = %s, want %s, not the later full retransmission", got, completed)
	}
	if _, err := judgeDelayOpen(history, netip.MustParseAddr("172.30.0.3"), 20*time.Second, time.Second, true); err != nil {
		t.Fatalf("valid early OPEN/reply rejected after overlapping retransmission: %v", err)
	}
}

// VALIDATES: an ACK or FIN can prove payload absent from the pcap even when
// reassembly has no segment from that direction and consequently reports no gap.
// PREVENTS: a missing collision loser's OPEN being hidden by a complete winner.
func TestDelayOpenRejectsAcknowledgedOrFINDelimitedMissingPayload(t *testing.T) {
	original := delayOpenTestRecords(t, delayOpenTestCapture(t, 5*time.Second, 5*time.Second+time.Millisecond, false, false))
	for _, test := range []struct {
		name         string
		capturedOpen bool
		fin          bool
	}{
		{name: "entire loser direction acknowledged"},
		{name: "entire loser direction FIN delimited", fin: true},
		{name: "missing trailing bytes acknowledged", capturedOpen: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			var out bytes.Buffer
			if err := pcap.WriteFileHeader(&out, 65535, pcap.LinkTypeEthernet); err != nil {
				t.Fatal(err)
			}
			write := func(at time.Time, raw []byte) {
				if err := pcap.WriteRecord(&out, at, raw, len(raw)); err != nil {
					t.Fatal(err)
				}
			}
			for _, record := range original {
				write(record.Timestamp, record.Data)
			}
			// A distinct native tuple starts after the valid winner. Preserve
			// its complete handshake but omit payload independently of the
			// ACK/FIN which proves that those bytes existed.
			start := original[0].Timestamp.Add(10 * time.Second)
			loserPacket := func(raw []byte) []byte {
				raw = bytes.Clone(raw)
				if raw[29] == 2 {
					binary.BigEndian.PutUint16(raw[34:36], 40001)
				} else {
					binary.BigEndian.PutUint16(raw[36:38], 40001)
				}
				return raw
			}
			for index := range 3 {
				write(start.Add(time.Duration(index)*time.Millisecond), loserPacket(original[index].Data))
			}
			boundary := uint32(130) // ISN 100 + SYN + an entirely uncaptured 29-byte OPEN.
			if test.capturedOpen {
				write(start.Add(5*time.Second), loserPacket(original[3].Data))
				boundary += 19 // The captured OPEN is followed by an uncaptured KEEPALIVE.
			}
			var close []byte
			if test.fin {
				close = loserPacket(original[2].Data)
				close[47] = 17 // FIN|ACK, with sequence after the missing OPEN.
				binary.BigEndian.PutUint32(close[38:42], boundary)
			} else {
				close = loserPacket(original[1].Data)
				close[47] = 20 // RST|ACK, acknowledging all missing payload bytes.
				binary.BigEndian.PutUint32(close[38:42], 201)
				binary.BigEndian.PutUint32(close[42:46], boundary)
			}
			write(start.Add(6*time.Second), close)
			history, err := delayOpenHistory(out.Bytes(), netip.MustParseAddr("172.30.0.2"))
			if err == nil {
				_, timingErr := judgeDelayOpen(history, netip.MustParseAddr("172.30.0.3"), 5*time.Second, 30*time.Second, false)
				t.Fatalf("incomplete captured history accepted despite TCP payload boundary (timing verdict: %v)", timingErr)
			}
		})
	}
}

// VALIDATES: every additional completed native connection must have captured
// closure, even if it has not yet exchanged OPENs.
// PREVENTS: a valid winner masking an unclosed, OPEN-less collision connection.
func TestDelayOpenRejectsAdditionalLiveConnection(t *testing.T) {
	ze, peer := netip.MustParseAddr("172.30.0.2"), netip.MustParseAddr("172.30.0.3")
	for _, oneOpen := range []bool{false, true} {
		t.Run(map[bool]string{false: "no OPEN", true: "one OPEN"}[oneOpen], func(t *testing.T) {
			history, err := delayOpenHistory(delayOpenTestCapture(t, 5*time.Second, 5*time.Second+time.Millisecond, false, false), ze)
			if err != nil {
				t.Fatal(err)
			}
			additional := &delayOpenConnection{
				Flow:      pcap.Flow{SourceAddr: ze, TargetAddr: peer, SourcePort: 179, TargetPort: 40001},
				Initiator: peer,
				Handshake: history[0].Handshake.Add(time.Millisecond),
			}
			if oneOpen {
				at := additional.Handshake.Add(5 * time.Second)
				additional.Ze = []delayOpenMessage{{Type: bgpOpen, First: at, Complete: at}}
			}
			history = append(history, additional)
			if observation, err := judgeDelayOpen(history, peer, 5*time.Second, 30*time.Second, false); err == nil {
				t.Fatalf("additional unclosed completed TCP connection accepted: %s", observation)
			}
		})
	}
}

func delayOpenTestRecords(t *testing.T, raw []byte) []pcap.Record {
	t.Helper()
	reader, err := pcap.NewReader(bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	var records []pcap.Record
	for {
		var record pcap.Record
		err := reader.Next(&record)
		if err == io.EOF {
			return records
		}
		if err != nil {
			t.Fatal(err)
		}
		record.Data = bytes.Clone(record.Data)
		records = append(records, record)
	}
}

// VALIDATES: sequence wrap preserves coverage offsets and a captured FIN
// contributes exactly one acknowledged sequence number, not one payload byte.
// PREVENTS: the missing-byte guard refusing a valid native close or allowing an
// unseen FIN to explain a missing final payload byte.
func TestDelayOpenSequenceWrapAndFIN(t *testing.T) {
	for _, test := range []struct {
		name          string
		wrap, omitFIN bool
	}{
		{name: "captured FIN acknowledged"},
		{name: "sequence wrap and FIN acknowledged", wrap: true},
		{name: "uncaptured FIN cannot explain ACK", omitFIN: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			records := delayOpenTestRecords(t, delayOpenTestCapture(t, 5*time.Second, 5*time.Second+time.Millisecond, false, false))
			at := records[len(records)-1].Timestamp.Add(time.Second)
			fin := bytes.Clone(records[2].Data)
			fin[47] = 17
			binary.BigEndian.PutUint32(fin[38:42], 149)
			binary.BigEndian.PutUint32(fin[42:46], 249)
			ack := bytes.Clone(records[1].Data)
			ack[47] = 16
			binary.BigEndian.PutUint32(ack[38:42], 249)
			binary.BigEndian.PutUint32(ack[42:46], 150)
			if !test.omitFIN {
				records = append(records, pcap.Record{Timestamp: at, Data: fin})
			}
			records = append(records, pcap.Record{Timestamp: at.Add(time.Millisecond), Data: ack})
			var out bytes.Buffer
			if err := pcap.WriteFileHeader(&out, 65535, pcap.LinkTypeEthernet); err != nil {
				t.Fatal(err)
			}
			for _, record := range records {
				raw := record.Data
				if test.wrap {
					sourceDelta, targetDelta := ^uint32(0)-110, ^uint32(0)-210
					if raw[29] == 3 {
						sourceDelta, targetDelta = targetDelta, sourceDelta
					}
					binary.BigEndian.PutUint32(raw[38:42], binary.BigEndian.Uint32(raw[38:42])+sourceDelta)
					if raw[47]&16 != 0 {
						binary.BigEndian.PutUint32(raw[42:46], binary.BigEndian.Uint32(raw[42:46])+targetDelta)
					}
				}
				if err := pcap.WriteRecord(&out, record.Timestamp, raw, len(raw)); err != nil {
					t.Fatal(err)
				}
			}
			_, err := delayOpenHistory(out.Bytes(), netip.MustParseAddr("172.30.0.2"))
			if test.omitFIN {
				if err == nil || !strings.Contains(err.Error(), "ACK proves missing") {
					t.Fatalf("missing FIN: %v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
		})
	}
}

// VALIDATES: successful silent signalling is Exec, followed by a measured
// capture-exit barrier; nonzero signalling exits never pass that barrier.
// PREVENTS: Query's nonempty-output contract rejecting a successful pkill or
// reading the pcap before the capture process has flushed it.
func TestDelayOpenCaptureStopAcceptsSilentExec(t *testing.T) {
	for _, code := range []int{0, 1} {
		lab := &delayOpenStopLab{exitCode: code}
		err := stopDelayOpenCapture(t.Context(), lab)
		if code == 0 {
			if err != nil || !lab.queriedExit {
				t.Fatalf("silent capture stop: queried=%v err=%v", lab.queriedExit, err)
			}
		} else if err == nil || lab.queriedExit {
			t.Fatalf("failed signal accepted: queried=%v err=%v", lab.queriedExit, err)
		}
	}
}

type delayOpenStopLab struct {
	noEvidenceLab
	exitCode              int
	signaled, queriedExit bool
}

func (lab *delayOpenStopLab) Exec(_ context.Context, peer string, argv []string, _ []interoplab.EnvironmentVariable) (interoplab.CommandResult, error) {
	if peer != "ze" || strings.Join(argv, " ") != "pkill -INT -x tcpdump" {
		return interoplab.CommandResult{}, errors.New("unexpected capture-stop execution")
	}
	lab.signaled = true
	return interoplab.CommandResult{ExitCode: lab.exitCode}, nil
}

func (lab *delayOpenStopLab) Query(_ context.Context, peer string, argv []string, _ []interoplab.EnvironmentVariable) (string, error) {
	if !lab.signaled || peer != "ze" || strings.Join(argv, " ") != "cat "+delayOpenCaptureExit {
		return "", errors.New("capture exit queried before silent Exec")
	}
	lab.queriedExit = true
	return "0\n", nil
}

// VALIDATES: an ACK proves its bytes existed by that packet's timestamp, not
// merely somewhere in the eventual capture.
// PREVENTS: a later retransmission concealing a missing premature loser OPEN.
func TestDelayOpenRejectsLateRetransmissionAfterACK(t *testing.T) {
	records := delayOpenTestRecords(t, delayOpenTestCapture(t, 5*time.Second, 5*time.Second+time.Millisecond, false, false))
	var out bytes.Buffer
	if err := pcap.WriteFileHeader(&out, 65535, pcap.LinkTypeEthernet); err != nil {
		t.Fatal(err)
	}
	write := func(at time.Time, raw []byte) {
		if err := pcap.WriteRecord(&out, at, raw, len(raw)); err != nil {
			t.Fatal(err)
		}
	}
	for _, record := range records {
		write(record.Timestamp, record.Data)
	}
	loserPacket := func(raw []byte) []byte {
		raw = bytes.Clone(raw)
		if raw[29] == 2 {
			binary.BigEndian.PutUint16(raw[34:36], 40001)
		} else {
			binary.BigEndian.PutUint16(raw[36:38], 40001)
		}
		return raw
	}
	start := records[0].Timestamp.Add(10 * time.Second)
	for index := range 3 {
		write(start.Add(time.Duration(index)*time.Millisecond), loserPacket(records[index].Data))
	}
	ack := loserPacket(records[1].Data)
	ack[47] = 16
	binary.BigEndian.PutUint32(ack[38:42], 201)
	binary.BigEndian.PutUint32(ack[42:46], 130) // Received Ze's 29-byte OPEN before one second.
	write(start.Add(time.Second), ack)
	// Only its later retransmission is captured, at the permitted five-second
	// timer boundary. Final byte coverage alone cannot establish timely OPEN.
	write(start.Add(5*time.Second+2*time.Millisecond), loserPacket(records[3].Data))
	close := bytes.Clone(ack)
	close[47] = 20
	write(start.Add(6*time.Second), close)
	history, err := delayOpenHistory(out.Bytes(), netip.MustParseAddr("172.30.0.2"))
	if err == nil {
		_, timingErr := judgeDelayOpen(history, netip.MustParseAddr("172.30.0.3"), 5*time.Second, 30*time.Second, false)
		t.Fatalf("late retransmission concealed payload already proven by ACK (timing verdict: %v)", timingErr)
	}
	if !strings.Contains(err.Error(), "ACK") {
		t.Fatalf("capture rejected for an unrelated reason, want temporal ACK coverage: %v", err)
	}
}

// VALIDATES: a later captured FIN cannot explain an earlier ACK of its sequence
// number, even when the eventual capture has complete payload and FIN coverage.
// PREVENTS: final coverage retroactively legitimizing a missing original FIN.
func TestDelayOpenRejectsFutureFINForEarlierACK(t *testing.T) {
	records := delayOpenTestRecords(t, delayOpenTestCapture(t, 5*time.Second, 5*time.Second+time.Millisecond, false, false))
	var out bytes.Buffer
	if err := pcap.WriteFileHeader(&out, 65535, pcap.LinkTypeEthernet); err != nil {
		t.Fatal(err)
	}
	write := func(at time.Time, raw []byte) {
		if err := pcap.WriteRecord(&out, at, raw, len(raw)); err != nil {
			t.Fatal(err)
		}
	}
	for _, record := range records {
		write(record.Timestamp, record.Data)
	}
	ack := bytes.Clone(records[1].Data)
	ack[47] = 16
	binary.BigEndian.PutUint32(ack[38:42], 249)
	binary.BigEndian.PutUint32(ack[42:46], 150) // 48 bytes and an unseen FIN acknowledged.
	write(records[0].Timestamp.Add(6*time.Second), ack)
	fin := bytes.Clone(records[2].Data)
	fin[47] = 17
	binary.BigEndian.PutUint32(fin[38:42], 149)
	binary.BigEndian.PutUint32(fin[42:46], 249)
	write(records[0].Timestamp.Add(7*time.Second), fin)
	if _, err := delayOpenHistory(out.Bytes(), netip.MustParseAddr("172.30.0.2")); err == nil || !strings.Contains(err.Error(), "ACK precedes captured TCP FIN") {
		t.Fatalf("future FIN explained earlier ACK: %v", err)
	}
}
