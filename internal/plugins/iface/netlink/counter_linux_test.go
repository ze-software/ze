//go:build linux

package ifacenetlink

import (
	"errors"
	"os"
	"syscall"
	"testing"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"
)

// TestCounterGenerationReusedIndex proves that deletion and recreation between
// dumps resets continuity even when the replacement has larger counters.
func TestCounterGenerationReusedIndex(t *testing.T) {
	var source counterSource
	first := counterDump{sequence: 11}
	if err := source.consume(&first, counterLinkMessage(11, unix.RTM_NEWLINK, 100)); err != nil {
		t.Fatal(err)
	}
	if len(first.links) != 1 {
		t.Fatalf("first dump has %d links", len(first.links))
	}
	before := first.links[0].generation
	if before == 0 {
		t.Fatal("first incarnation has no generation")
	}

	second := counterDump{sequence: 12}
	for _, message := range []syscall.NetlinkMessage{
		counterLinkMessage(0, unix.RTM_DELLINK, 100),
		counterLinkMessage(0, unix.RTM_NEWLINK, 200),
		counterLinkMessage(12, unix.RTM_NEWLINK, 300),
	} {
		if err := source.consume(&second, message); err != nil {
			t.Fatal(err)
		}
	}
	if len(second.links) != 1 {
		t.Fatalf("second dump has %d links", len(second.links))
	}
	info := linkToInfo(second.links[0].link)
	if info.Stats == nil || info.Stats.RxBytes != 300 {
		t.Fatalf("replacement stats = %+v, want rx-bytes 300", info.Stats)
	}
	if second.links[0].generation == before || second.links[0].generation == 0 {
		t.Fatal("reused ifIndex retained the old counter generation")
	}
}

// TestCounterGenerationPreservesContinuity proves that ordinary link updates,
// including down/up and AF_BRIDGE removal, do not reset a live interface.
func TestCounterGenerationPreservesContinuity(t *testing.T) {
	var source counterSource
	first := counterDump{sequence: 21}
	if err := source.consume(&first, counterLinkMessage(21, unix.RTM_NEWLINK, 100)); err != nil {
		t.Fatal(err)
	}
	before := first.links[0].generation
	second := counterDump{sequence: 22}
	bridge := counterLinkMessage(0, unix.RTM_DELLINK, 150)
	bridge.Data[0] = unix.AF_BRIDGE
	for _, message := range []syscall.NetlinkMessage{
		counterLinkMessage(0, unix.RTM_NEWLINK, 150),
		bridge,
		counterLinkMessage(22, unix.RTM_NEWLINK, 200),
	} {
		if err := source.consume(&second, message); err != nil {
			t.Fatal(err)
		}
	}
	if len(second.links) != 1 || second.links[0].generation != before {
		t.Fatal("ordinary link update reset counter continuity")
	}
}

// TestCounterGenerationLossStartsNewEpoch proves that a lost event stream does
// not assert that an unchanged ifIndex still identifies the same incarnation.
func TestCounterGenerationLossStartsNewEpoch(t *testing.T) {
	var source counterSource
	first := counterDump{sequence: 31}
	if err := source.consume(&first, counterLinkMessage(31, unix.RTM_NEWLINK, 100)); err != nil {
		t.Fatal(err)
	}
	before := first.links[0].generation
	if err := source.consume(&first, syscall.NetlinkMessage{
		Header: syscall.NlMsghdr{Type: unix.NLMSG_OVERRUN},
	}); !errors.Is(err, unix.ENOBUFS) {
		t.Fatalf("overrun error = %v, want ENOBUFS", err)
	}
	source.invalidate()
	second := counterDump{sequence: 32}
	if err := source.consume(&second, counterLinkMessage(32, unix.RTM_NEWLINK, 200)); err != nil {
		t.Fatal(err)
	}
	if second.links[0].generation == before {
		t.Fatal("event loss retained an unproved incarnation")
	}
}

// TestCounterDumpInterrupted preserves the interruption through NLMSG_DONE so
// callers reject the torn snapshot instead of exporting it.
func TestCounterDumpInterrupted(t *testing.T) {
	var source counterSource
	dump := counterDump{sequence: 41}
	message := counterLinkMessage(41, unix.RTM_NEWLINK, 100)
	message.Header.Flags |= unix.NLM_F_DUMP_INTR
	if err := source.consume(&dump, message); err != nil {
		t.Fatal(err)
	}
	err := source.consume(&dump, syscall.NetlinkMessage{
		Header: syscall.NlMsghdr{Type: unix.NLMSG_DONE, Seq: 41},
	})
	if !errors.Is(err, netlink.ErrDumpInterrupted) {
		t.Fatalf("dump completion = %v, want ErrDumpInterrupted", err)
	}
}

// TestCounterDumpRejectsMalformedMessages prevents a truncated kernel response
// from becoming a successful snapshot or reaching the unsafe netlink decoder.
func TestCounterDumpRejectsMalformedMessages(t *testing.T) {
	for _, kind := range []uint16{unix.RTM_NEWLINK, unix.RTM_DELLINK, unix.NLMSG_ERROR, unix.NLMSG_DONE} {
		var source counterSource
		dump := counterDump{sequence: 51}
		if err := source.consume(&dump, syscall.NetlinkMessage{
			Header: syscall.NlMsghdr{Type: kind, Seq: 51},
			Data:   []byte{1},
		}); err == nil {
			t.Errorf("message type %d accepted a truncated payload", kind)
		}
	}
}

// TestCounterGenerationRawDecrease compares full-width values. A reset changes
// the token, but crossing a 32-bit sFlow packet-counter boundary does not.
func TestCounterGenerationRawDecrease(t *testing.T) {
	var source counterSource
	first := counterDump{sequence: 61}
	message := counterLinkMessage(61, unix.RTM_NEWLINK, 100)
	nl.NativeEndian().PutUint64(message.Data[len(message.Data)-24*8:], (1<<32)-1)
	if err := source.consume(&first, message); err != nil {
		t.Fatal(err)
	}
	before := first.links[0].generation
	second := counterDump{sequence: 62}
	message = counterLinkMessage(62, unix.RTM_NEWLINK, 200)
	nl.NativeEndian().PutUint64(message.Data[len(message.Data)-24*8:], (1<<32)+1)
	if err := source.consume(&second, message); err != nil {
		t.Fatal(err)
	}
	if second.links[0].generation != before {
		t.Fatal("32-bit packet-counter wrap reset raw continuity")
	}
	third := counterDump{sequence: 63}
	message = counterLinkMessage(63, unix.RTM_NEWLINK, 300)
	nl.NativeEndian().PutUint64(message.Data[len(message.Data)-24*8:], 4)
	if err := source.consume(&third, message); err != nil {
		t.Fatal(err)
	}
	if third.links[0].generation == before {
		t.Fatal("raw packet-counter decrease retained the old generation")
	}
}

// TestCounterDumpForeignSequence treats a multicast update whose request
// sequence collides with ours as an event, not a sampled dump response.
func TestCounterDumpForeignSequence(t *testing.T) {
	var source counterSource
	dump := counterDump{sequence: 71, portID: 100}
	message := counterLinkMessage(71, unix.RTM_NEWLINK, 100)
	message.Header.Pid = 200
	if err := source.consume(&dump, message); err != nil {
		t.Fatal(err)
	}
	if len(dump.links) != 0 {
		t.Fatal("foreign multicast became a counter sample")
	}
	message.Header.Pid = 100
	if err := source.consume(&dump, message); err != nil {
		t.Fatal(err)
	}
	if len(dump.links) != 1 {
		t.Fatalf("own dump has %d links, want 1", len(dump.links))
	}
}

// TestCounterGenerationInterleavedDump keeps the old row's generation when a
// deletion and replacement arrive before the multipart dump completes.
func TestCounterGenerationInterleavedDump(t *testing.T) {
	var source counterSource
	dump := counterDump{sequence: 81}
	if err := source.consume(&dump, counterLinkMessage(81, unix.RTM_NEWLINK, 100)); err != nil {
		t.Fatal(err)
	}
	before := dump.links[0].generation
	for _, message := range []syscall.NetlinkMessage{
		counterLinkMessage(0, unix.RTM_DELLINK, 100),
		counterLinkMessage(0, unix.RTM_NEWLINK, 200),
		{Header: syscall.NlMsghdr{Type: unix.NLMSG_DONE, Seq: 81}},
	} {
		if err := source.consume(&dump, message); err != nil {
			t.Fatal(err)
		}
	}
	if !dump.done || len(dump.links) != 1 {
		t.Fatal("interleaved notifications changed the sampled row set")
	}
	if dump.links[0].generation != before || dump.links[0].link.Attrs().Statistics.RxBytes != 100 {
		t.Fatal("old sampled counters were reassigned to the replacement")
	}
	next := counterDump{sequence: 82}
	if err := source.consume(&next, counterLinkMessage(82, unix.RTM_NEWLINK, 300)); err != nil {
		t.Fatal(err)
	}
	if next.links[0].generation == before {
		t.Fatal("the next snapshot did not observe the replacement")
	}
}

// TestCounterSourceClose refuses later sampling instead of reopening a socket
// after the owning backend has released its resources.
func TestCounterSourceClose(t *testing.T) {
	var source counterSource
	source.close()
	if _, err := source.snapshot(""); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("snapshot after close = %v, want os.ErrClosed", err)
	}
	source.close()
}

func counterLinkMessage(sequence uint32, kind uint16, rxBytes uint64) syscall.NetlinkMessage {
	const index int32 = 7
	message := nl.NewIfInfomsg(unix.AF_UNSPEC)
	message.Index = index
	data := append([]byte(nil), message.Serialize()...)
	data = append(data, nl.NewRtAttr(unix.IFLA_IFNAME, nl.ZeroTerminated("counter0")).Serialize()...)
	stats := make([]byte, 24*8)
	nl.NativeEndian().PutUint64(stats[2*8:3*8], rxBytes)
	data = append(data, nl.NewRtAttr(unix.IFLA_STATS64, stats).Serialize()...)
	return syscall.NetlinkMessage{
		Header: syscall.NlMsghdr{Type: kind, Seq: sequence},
		Data:   data,
	}
}
