// Design: docs/features/interfaces.md -- bounded native creation payloads
// Related: counter_create_request_linux_test.go -- actual creator capability

//go:build linux

package ifacenetlink

import (
	"testing"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"
)

// TestCounterCreateTypedNilPayloadRefuses requires real vendor typed-nil values
// to receive a refusal before Len or Serialize can panic or reach native IO.
func TestCounterCreateTypedNilPayloadRefuses(t *testing.T) {
	cases := []struct {
		name    string
		payload nl.NetlinkRequestData
	}{
		{"nil-route-attribute", (*nl.RtAttr)(nil)},
		{"nil-interface-header", (*nl.IfInfomsg)(nil)},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			request := counterCreateRequest(unix.RTM_NEWLINK, unix.NLM_F_CREATE|unix.NLM_F_EXCL|unix.NLM_F_ACK)
			request.AddData(test.payload)
			counterCreatePayloadRefusal(t, request)
		})
	}
}

// TestCounterCreateMalformedPayloadRefuses exercises each bounded preflight
// failure without relying on a kernel refusal or an unrelated malformed attr.
func TestCounterCreateMalformedPayloadRefuses(t *testing.T) {
	cases := []struct {
		name    string
		request func() *nl.NetlinkRequest
	}{
		{"nil-data-entry", func() *nl.NetlinkRequest {
			request := counterCreateRequest(unix.RTM_NEWLINK, unix.NLM_F_CREATE|unix.NLM_F_EXCL|unix.NLM_F_ACK)
			request.AddData(nil)
			return request
		}},
		{"too-many-zero-length-parts", counterCreateEmptyPartsRequest},
		{"negative-length", func() *nl.NetlinkRequest {
			request := counterCreateRequest(unix.RTM_NEWLINK, unix.NLM_F_CREATE|unix.NLM_F_EXCL|unix.NLM_F_ACK)
			request.AddData(&counterCreateRejectedPayload{length: -1})
			return request
		}},
		{"underreported-oversized-serialization", counterCreateUnderreportedRequest},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			counterCreatePayloadRefusal(t, test.request())
		})
	}
}

func counterCreatePayloadRefusal(t *testing.T, request *nl.NetlinkRequest) {
	t.Helper()
	backend := &netlinkBackend{}
	t.Cleanup(func() {
		if err := backend.Close(); err != nil {
			t.Error(err)
		}
	})
	created, read, attempted, err := counterCreatePayloadCall(t, backend, request)
	if err == nil {
		t.Fatal("invalid caller payload reported creation success")
	}
	if attempted {
		t.Fatal("invalid caller payload reached the native attempt")
	}
	if created != nil {
		t.Fatal("invalid caller payload returned a created-link receipt")
	}
	if read != nil {
		t.Fatal("invalid caller payload returned a counter reader")
	}
	if backend.counters.socket != nil {
		t.Fatal("invalid caller payload opened a native counter socket")
	}
}

func counterCreatePayloadCall(t *testing.T, backend *netlinkBackend, request *nl.NetlinkRequest) (netlink.Link, func() (*netlink.LinkStatistics, error), bool, error) {
	t.Helper()
	defer func() {
		if failure := recover(); failure != nil {
			t.Fatalf("invalid caller payload panicked instead of receiving a refusal: %v", failure)
		}
	}()
	return counterCreateCall(t, backend, request)
}

func counterCreateEmptyPartsRequest() *nl.NetlinkRequest {
	request := counterCreateRequest(unix.RTM_NEWLINK, unix.NLM_F_CREATE|unix.NLM_F_EXCL|unix.NLM_F_ACK)
	header := request.Data[0]
	request.Data = make([]nl.NetlinkRequestData, monitorReceiveBufferBytes+1)
	request.Data[0] = header
	empty := &counterCreateRejectedPayload{}
	// The interface slice is bounded at about16MiB; every remaining entry is
	// the same nonnil zero-length payload, so no earlier nil/byte guard wins.
	for index := 1; index < len(request.Data); index++ {
		request.Data[index] = empty
	}
	return request
}

func counterCreateUnderreportedRequest() *nl.NetlinkRequest {
	large := counterCreateOversizedRequest()
	wire := large.Serialize()
	request := nl.NewNetlinkRequest(unix.RTM_NEWLINK, unix.NLM_F_CREATE|unix.NLM_F_EXCL|unix.NLM_F_ACK)
	// Its actual body retains the existing well-framed greater-than1MiB
	// fixture; only the caller's declared length understates that body.
	request.AddData(&counterCreateRejectedPayload{
		length: unix.SizeofIfInfomsg,
		wire:   wire[unix.NLMSG_HDRLEN:],
	})
	return request
}

type counterCreateRejectedPayload struct {
	length int
	wire   []byte
}

func (payload *counterCreateRejectedPayload) Len() int          { return payload.length }
func (payload *counterCreateRejectedPayload) Serialize() []byte { return payload.wire }
