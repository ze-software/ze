// Design: docs/features/interfaces.md -- ordered creation and counter continuity
// Related: counter_create_linux_test.go -- actual backend capability wiring

//go:build linux

package ifacenetlink

import (
	"errors"
	"syscall"
	"testing"

	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"
)

// TestCounterCreateReplyOrder requires both matching replies, in either order,
// through the same completion predicate the native receive loop uses.
func TestCounterCreateReplyOrder(t *testing.T) {
	cases := []struct {
		name     string
		messages [2]syscall.NetlinkMessage
	}{
		{"echo-first", [2]syscall.NetlinkMessage{counterCreateEcho(), counterCreateACK(0)}},
		{"ack-first", [2]syscall.NetlinkMessage{counterCreateACK(0), counterCreateEcho()}},
	}
	for index := range cases {
		test := &cases[index]
		t.Run(test.name, func(t *testing.T) {
			var source counterSource
			dump := counterCreateDump()
			if err := counterCreateDecode(t, &source, &dump, test.messages[:1], nl.PidKernel); err != nil {
				t.Fatal(err)
			}
			if counterCreateComplete(t, &dump) {
				t.Fatal("creation completed after only one of echo and ACK")
			}
			if err := counterCreateDecode(t, &source, &dump, test.messages[1:], nl.PidKernel); err != nil {
				t.Fatal(err)
			}
			if !counterCreateComplete(t, &dump) {
				t.Fatal("matching echo and ACK did not complete creation")
			}
			counterCreateOriginalEcho(t, &dump)
		})
	}
}

// TestCounterCreateMulticastOrdering keeps the creator's generation when later
// notifications delete and replace that index before the matching ACK.
func TestCounterCreateMulticastOrdering(t *testing.T) {
	var source counterSource
	dump := counterCreateDump()
	before := counterLinkMessage(0, unix.RTM_NEWLINK, 50)
	if err := counterCreateDecode(t, &source, &dump, []syscall.NetlinkMessage{before}, nl.PidKernel); err != nil {
		t.Fatal(err)
	}
	if len(dump.links) != 0 {
		t.Fatal("multicast became a creator echo")
	}
	if counterCreateComplete(t, &dump) {
		t.Fatal("multicast completed a creation request")
	}
	if err := counterCreateDecode(t, &source, &dump, []syscall.NetlinkMessage{counterCreateEcho()}, nl.PidKernel); err != nil {
		t.Fatal(err)
	}
	counterCreateOriginalEcho(t, &dump)
	original := dump.links[0].generation
	messages := []syscall.NetlinkMessage{
		counterLinkMessage(0, unix.RTM_DELLINK, 101),
		counterLinkMessage(0, unix.RTM_NEWLINK, 303),
		counterCreateACK(0),
	}
	if err := counterCreateDecode(t, &source, &dump, messages, nl.PidKernel); err != nil {
		t.Fatal(err)
	}
	if !counterCreateComplete(t, &dump) {
		t.Fatal("matching echo and ACK did not finish after multicast")
	}
	counterCreateOriginalEcho(t, &dump)
	if dump.links[0].generation != original {
		t.Fatal("multicast rebound the original creator receipt")
	}
	if source.states[7].generation == original {
		t.Fatal("delete and replacement retained the original generation")
	}
}

// TestCounterCreateMissingEcho refuses to complete on a successful ACK followed
// only by link notifications, while still consuming those notifications.
func TestCounterCreateMissingEcho(t *testing.T) {
	var source counterSource
	dump := counterCreateDump()
	messages := []syscall.NetlinkMessage{
		counterCreateACK(0),
		counterLinkMessage(0, unix.RTM_NEWLINK, 202),
	}
	if err := counterCreateDecode(t, &source, &dump, messages, nl.PidKernel); err != nil {
		t.Fatal(err)
	}
	if counterCreateComplete(t, &dump) {
		t.Fatal("ACK and multicast completed creation without an echo")
	}
	if len(dump.links) != 0 {
		t.Fatal("notification supplied an unconfirmed creator identity")
	}
	if source.states[7].generation == 0 {
		t.Fatal("creator decoder discarded the real link notification")
	}
}

// TestCounterCreateInvalidMessages rejects foreign senders and malformed or
// mismatched completion messages without publishing a created-link receipt.
func TestCounterCreateInvalidMessages(t *testing.T) {
	wrongSequence := counterCreateACK(0)
	wrongSequence.Header.Seq++
	wrongPort := counterCreateACK(0)
	wrongPort.Header.Pid++
	shortACK := counterCreateACK(0)
	shortACK.Data = shortACK.Data[:3]
	shortLink := counterCreateEcho()
	shortLink.Data = shortLink.Data[:unix.SizeofIfInfomsg-1]
	done := syscall.NetlinkMessage{
		Header: syscall.NlMsghdr{Type: unix.NLMSG_DONE, Seq: 151, Pid: 191},
	}
	overrun := syscall.NetlinkMessage{Header: syscall.NlMsghdr{Type: unix.NLMSG_OVERRUN}}
	cases := []struct {
		name     string
		sender   uint32
		messages []syscall.NetlinkMessage
		cause    error
	}{
		{"foreign-sender-empty-batch", 17, nil, nil},
		{"foreign-sender-echo", 17, []syscall.NetlinkMessage{counterCreateEcho()}, nil},
		{"wrong-request-sequence", nl.PidKernel, []syscall.NetlinkMessage{wrongSequence}, nil},
		{"wrong-request-port", nl.PidKernel, []syscall.NetlinkMessage{wrongPort}, nil},
		{"short-ack", nl.PidKernel, []syscall.NetlinkMessage{shortACK}, nil},
		{"short-link", nl.PidKernel, []syscall.NetlinkMessage{shortLink}, nil},
		{"unexpected-done", nl.PidKernel, []syscall.NetlinkMessage{done}, nil},
		{"overrun", nl.PidKernel, []syscall.NetlinkMessage{overrun}, unix.ENOBUFS},
	}
	for index := range cases {
		test := &cases[index]
		t.Run(test.name, func(t *testing.T) {
			var source counterSource
			dump := counterCreateDump()
			err := counterCreateDecode(t, &source, &dump, test.messages, test.sender)
			if err == nil {
				t.Fatal("invalid creator message reported success")
			}
			if test.cause != nil {
				if !errors.Is(err, test.cause) {
					t.Fatalf("creator failure=%v, want cause %v", err, test.cause)
				}
			}
			if len(dump.links) != 0 {
				t.Fatal("invalid creator message published a link receipt")
			}
		})
	}
}

func counterCreateDump() counterDump {
	return counterDump{sequence: 151, portID: 191, single: true}
}

func counterCreateEcho() syscall.NetlinkMessage {
	message := counterLinkMessage(151, unix.RTM_NEWLINK, 101)
	message.Header.Pid = 191
	return message
}

func counterCreateACK(code unix.Errno) syscall.NetlinkMessage {
	return counterCreateACKFor(151, unix.RTM_NEWLINK, unix.NLM_F_CREATE|unix.NLM_F_EXCL|unix.NLM_F_ACK, code)
}

func counterCreateACKFor(sequence uint32, kind, flags int, code unix.Errno) syscall.NetlinkMessage {
	request := nl.NewNetlinkRequest(kind, flags)
	request.Seq = sequence
	request.Pid = 191
	request.AddData(nl.NewIfInfomsg(unix.AF_UNSPEC))
	data := make([]byte, 4+unix.NLMSG_HDRLEN)
	nl.NativeEndian().PutUint32(data, uint32(-int32(code)))
	copy(data[4:], request.Serialize()[:unix.NLMSG_HDRLEN])
	return syscall.NetlinkMessage{
		Header: syscall.NlMsghdr{Type: unix.NLMSG_ERROR, Flags: unix.NLM_F_CAPPED, Seq: sequence, Pid: 191},
		Data:   data,
	}
}

func counterCreateDecode(t *testing.T, source *counterSource, dump *counterDump, messages []syscall.NetlinkMessage, sender uint32) error {
	t.Helper()
	decoder, supported := any(source).(interface {
		consumeCreate(*counterDump, []syscall.NetlinkMessage, uint32) error
	})
	if !supported {
		t.Fatal("counter source does not provide the creator reply decoder")
	}
	source.mu.Lock()
	defer source.mu.Unlock()
	return decoder.consumeCreate(dump, messages, sender)
}

func counterCreateComplete(t *testing.T, dump *counterDump) bool {
	t.Helper()
	completion, supported := any(dump).(interface{ createComplete() bool })
	if !supported {
		t.Fatal("counter dump does not provide the creator completion predicate")
	}
	return completion.createComplete()
}

func counterCreateOriginalEcho(t *testing.T, dump *counterDump) {
	t.Helper()
	if len(dump.links) != 1 {
		t.Fatalf("creator receipts=%d, want one matching echo", len(dump.links))
	}
	if dump.links[0].link.Attrs().Index != 7 {
		t.Fatalf("creator index=%d, want actual echo index 7", dump.links[0].link.Attrs().Index)
	}
	if dump.links[0].generation == 0 {
		t.Fatal("creator echo has no original generation")
	}
	stats := dump.links[0].link.Attrs().Statistics
	if stats == nil {
		t.Fatal("fixture creator echo lost its statistics")
	}
	if stats.RxBytes != 101 {
		t.Fatalf("creator echo RxBytes=%d, want original 101", stats.RxBytes)
	}
}
