// Design: docs/features/interfaces.md -- deterministic creator refusal boundaries
// Related: counter_create_request_linux_test.go -- creator result helpers
// Upstream: github.com/vishvananda/netlink v1.3.1, request payload serialization:
// https://github.com/vishvananda/netlink/blob/v1.3.1/nl/nl_linux.go

//go:build linux

package ifacenetlink

import (
	"errors"
	"os"
	"strings"
	"syscall"
	"testing"

	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"
)

// TestCounterCreateOversizedRawDataRefuses checks the raw-byte preflight before
// any payload callback, using a complete link body rather than malformed bytes.
func TestCounterCreateOversizedRawDataRefuses(t *testing.T) {
	request := nl.NewNetlinkRequest(unix.RTM_NEWLINK, unix.NLM_F_CREATE|unix.NLM_F_EXCL|unix.NLM_F_ACK)
	payload := &counterCreatePayload{}
	request.AddData(payload)
	request.RawData = counterCreateBudgetBody(t)
	request.RawData = append(request.RawData, nl.NewRtAttr(unix.IFLA_IFALIAS, nil).Serialize()...)
	counterCreatePayloadRefusal(t, request)
	if payload.lengths != 0 {
		t.Fatal("oversized raw data reached a payload length callback")
	}
	if payload.serializations != 0 {
		t.Fatal("oversized raw data reached a payload serialization callback")
	}
}

// TestCounterCreateAggregateRawDataRefuses fills the serialized payload budget
// exactly, leaving a valid raw attribute for the final aggregate guard to refuse.
func TestCounterCreateAggregateRawDataRefuses(t *testing.T) {
	request := nl.NewNetlinkRequest(unix.RTM_NEWLINK, unix.NLM_F_CREATE|unix.NLM_F_EXCL|unix.NLM_F_ACK)
	request.AddData(&counterCreateRejectedPayload{
		length: unix.SizeofIfInfomsg,
		wire:   counterCreateBudgetBody(t),
	})
	request.RawData = nl.NewRtAttr(unix.IFLA_IFALIAS, nil).Serialize()
	counterCreatePayloadRefusal(t, request)
}

// TestCounterCreateClosedBackendRefuses sends a valid envelope to a closed
// owner and requires refusal without a native attempt or a reopened socket.
func TestCounterCreateClosedBackendRefuses(t *testing.T) {
	backend := &netlinkBackend{}
	t.Cleanup(func() {
		if err := backend.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := backend.Close(); err != nil {
		t.Fatal(err)
	}
	request := counterCreateRequest(unix.RTM_NEWLINK, unix.NLM_F_CREATE|unix.NLM_F_EXCL|unix.NLM_F_ACK)
	created, read, attempted, err := counterCreatePayloadCall(t, backend, request)
	if !errors.Is(err, os.ErrClosed) {
		t.Fatalf("closed creator failure=%v, want %v", err, os.ErrClosed)
	}
	if attempted {
		t.Fatal("closed creator reached a native attempt")
	}
	if created != nil {
		t.Fatal("closed creator returned a link receipt")
	}
	if read != nil {
		t.Fatal("closed creator returned a counter reader")
	}
	if backend.counters.socket != nil {
		t.Fatal("closed creator reopened its socket")
	}
}

// TestCounterCreateIncompleteReceiptRefuses preserves a partial echo while
// refusing incomplete, invalid or ended identities through the result boundary.
func TestCounterCreateIncompleteReceiptRefuses(t *testing.T) {
	cases := []struct {
		scenario     string
		index        int32
		linkName     string
		acknowledged bool
		ended        bool
		reason       string
	}{
		{"missing-ack", 7, "counter0", false, false, "acknowledgment is incomplete"},
		{"missing-index", 0, "counter0", true, false, "no native index"},
		{"missing-name", 7, "", true, false, "no native name"},
		{"ended-generation", 7, "counter0", true, true, "original generation has ended"},
	}
	for _, test := range cases {
		t.Run(test.scenario, func(t *testing.T) {
			var source counterSource
			dump := counterCreateDump()
			messages := []syscall.NetlinkMessage{counterCreateAdditionalEcho(test.index, test.linkName)}
			if test.acknowledged {
				messages = append(messages, counterCreateACK(0))
			}
			if err := counterCreateDecode(t, &source, &dump, messages, nl.PidKernel); err != nil {
				t.Fatal(err)
			}
			if len(dump.links) != 1 {
				t.Fatalf("partial receipts=%d, want one decoded echo", len(dump.links))
			}
			if test.ended {
				source.mu.Lock()
				source.invalidate()
				source.mu.Unlock()
			}
			created, read, attempted, err := counterCreateFinish(t, &source, &dump, nil)
			if err == nil {
				t.Fatal("unusable creation receipt reported success")
			}
			if !strings.Contains(err.Error(), test.reason) {
				t.Fatalf("creation refusal=%v, want %q", err, test.reason)
			}
			if !attempted {
				t.Fatal("partial creation discarded its attempted ownership")
			}
			if created == nil {
				t.Fatal("partial creation discarded the original echo")
			}
			if created.Attrs().Index != int(test.index) {
				t.Fatal("partial creation changed the original native index")
			}
			if created.Attrs().Name != test.linkName {
				t.Fatal("partial creation changed the original native name")
			}
			if read == nil {
				t.Fatal("partial creation discarded its refused reader")
			}
			// Later source closure must not replace this receipt's original cause.
			source.close()
			for range 2 {
				stats, readErr := read()
				if !errors.Is(readErr, err) {
					t.Fatalf("partial reader failure=%v, want original cause %v", readErr, err)
				}
				if stats != nil {
					t.Fatal("unusable creation receipt published a counter sample")
				}
			}
		})
	}
}

func counterCreateBudgetBody(t *testing.T) []byte {
	t.Helper()
	budget := monitorReceiveBufferBytes - unix.NLMSG_HDRLEN
	wire := make([]byte, 0, budget)
	wire = append(wire, nl.NewIfInfomsg(unix.AF_UNSPEC).Serialize()...)
	attribute := nl.NewRtAttr(unix.IFLA_IFALIAS, make([]byte, 65528)).Serialize()
	// Sixteen 65532-byte attributes and one 32-byte attribute fill the budget.
	for range 16 {
		wire = append(wire, attribute...)
	}
	wire = append(wire, nl.NewRtAttr(unix.IFLA_IFALIAS, make([]byte, 28)).Serialize()...)
	if len(wire) != budget {
		t.Fatalf("framed payload bytes=%d, want exact budget %d", len(wire), budget)
	}
	return wire
}

func counterCreateAdditionalEcho(index int32, name string) syscall.NetlinkMessage {
	header := nl.NewIfInfomsg(unix.AF_UNSPEC)
	header.Index = index
	data := append([]byte(nil), header.Serialize()...)
	if name != "" {
		data = append(data, nl.NewRtAttr(unix.IFLA_IFNAME, nl.ZeroTerminated(name)).Serialize()...)
	}
	return syscall.NetlinkMessage{
		Header: syscall.NlMsghdr{Type: unix.RTM_NEWLINK, Seq: 151, Pid: 191},
		Data:   data,
	}
}
