// Design: docs/features/interfaces.md -- creation-bound raw counter ownership
// Related: counter_create_protocol_linux_test.go -- ordered creator replies

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

// TestCounterCreateInvalidRequest refuses malformed creation envelopes before
// any native attempt, rather than treating an unattempted request as ownership.
func TestCounterCreateInvalidRequest(t *testing.T) {
	cases := []struct {
		name    string
		request *nl.NetlinkRequest
	}{
		{"nil", nil},
		{"get", counterCreateRequest(unix.RTM_GETLINK, unix.NLM_F_CREATE|unix.NLM_F_EXCL)},
		{"delete", counterCreateRequest(unix.RTM_DELLINK, unix.NLM_F_CREATE|unix.NLM_F_EXCL)},
		{"set", counterCreateRequest(unix.RTM_SETLINK, unix.NLM_F_CREATE|unix.NLM_F_EXCL)},
		{"missing-exclusive", counterCreateRequest(unix.RTM_NEWLINK, unix.NLM_F_CREATE)},
		{"missing-create", counterCreateRequest(unix.RTM_NEWLINK, unix.NLM_F_EXCL)},
		{"replace", counterCreateRequest(unix.RTM_NEWLINK, unix.NLM_F_CREATE|unix.NLM_F_EXCL|unix.NLM_F_REPLACE)},
		{"append", counterCreateRequest(unix.RTM_NEWLINK, unix.NLM_F_CREATE|unix.NLM_F_EXCL|unix.NLM_F_APPEND)},
		{"unknown-flags", counterCreateRequest(unix.RTM_NEWLINK, unix.NLM_F_CREATE|unix.NLM_F_EXCL|0x8000)},
		{"missing-link-header", nl.NewNetlinkRequest(unix.RTM_NEWLINK, unix.NLM_F_CREATE|unix.NLM_F_EXCL|unix.NLM_F_ACK)},
		{"short-link-header", counterCreateShortHeaderRequest()},
		{"foreign-link-family", counterCreateHeaderRequest(unix.AF_BRIDGE, 0)},
		{"negative-link-index", counterCreateHeaderRequest(unix.AF_UNSPEC, -1)},
		{"namespace-fd-override", counterCreateNamespaceRequest(unix.IFLA_NET_NS_FD)},
		{"namespace-pid-override", counterCreateNamespaceRequest(unix.IFLA_NET_NS_PID)},
		{"namespace-id-override", counterCreateNamespaceRequest(unix.IFLA_IF_NETNSID)},
		{"flagged-namespace-fd", counterCreateNamespaceRequest(unix.IFLA_NET_NS_FD | unix.NLA_F_NESTED)},
		{"flagged-namespace-id", counterCreateNamespaceRequest(unix.IFLA_IF_NETNSID | unix.NLA_F_NET_BYTEORDER)},
		{"attribute-trailer", counterCreateMalformedAttribute([]byte{0})},
		{"short-attribute-length", counterCreateAttributeLength(3, 4)},
		{"attribute-length-overrun", counterCreateAttributeLength(8, 4)},
		{"attribute-alignment-overrun", counterCreateAttributeLength(5, 5)},
		{"oversized", counterCreateOversizedRequest()},
	}
	for index := range cases {
		test := &cases[index]
		t.Run(test.name, func(t *testing.T) {
			backend := &netlinkBackend{}
			t.Cleanup(func() {
				if err := backend.Close(); err != nil {
					t.Error(err)
				}
			})
			created, read, attempted, err := counterCreateCall(t, backend, test.request)
			if err == nil {
				t.Fatal("invalid creation request reported success")
			}
			if attempted {
				t.Fatal("invalid creation request reached the native attempt")
			}
			if created != nil {
				t.Fatal("invalid creation request returned a link receipt")
			}
			if read != nil {
				t.Fatal("invalid creation request returned a counter reader")
			}
			if backend.counters.socket != nil {
				t.Fatal("invalid creation request opened a counter socket")
			}
		})
	}
}

// TestCounterCreatePartialEchoFailure retains the original creation facts and
// permanently refuses its reader after either an ACK failure or history loss.
func TestCounterCreatePartialEchoFailure(t *testing.T) {
	cases := []struct {
		name    string
		message syscall.NetlinkMessage
		cause   error
	}{
		{"negative-ack", counterCreateACK(unix.EPERM), unix.EPERM},
		{"overrun", syscall.NetlinkMessage{Header: syscall.NlMsghdr{Type: unix.NLMSG_OVERRUN}}, unix.ENOBUFS},
	}
	for index := range cases {
		test := &cases[index]
		t.Run(test.name, func(t *testing.T) {
			var source counterSource
			dump := counterCreateDump()
			if err := counterCreateDecode(t, &source, &dump, []syscall.NetlinkMessage{counterCreateEcho()}, nl.PidKernel); err != nil {
				t.Fatal(err)
			}
			cause := counterCreateDecode(t, &source, &dump, []syscall.NetlinkMessage{test.message}, nl.PidKernel)
			if cause == nil {
				cause = dump.failure
			}
			if !errors.Is(cause, test.cause) {
				t.Fatalf("creator failure=%v, want %v", cause, test.cause)
			}
			counterCreateOriginalEcho(t, &dump)
			created, read, attempted, err := counterCreateFinish(t, &source, &dump, cause)
			if !attempted {
				t.Fatal("post-send failure discarded attempted ownership")
			}
			if !errors.Is(err, test.cause) {
				t.Fatalf("creation failure=%v, want original cause %v", err, test.cause)
			}
			if created == nil {
				t.Fatal("post-echo failure discarded the original link receipt")
			}
			if created.Attrs().Index != 7 {
				t.Fatalf("partial creator index=%d, want original 7", created.Attrs().Index)
			}
			if read == nil {
				t.Fatal("post-echo failure discarded the original refused reader")
			}
			for range 2 {
				stats, err := read()
				if !errors.Is(err, test.cause) {
					t.Fatalf("partial reader failure=%v, want original cause %v", err, test.cause)
				}
				if stats != nil {
					t.Fatal("failed creation reader returned a counter sample")
				}
			}
		})
	}
}

// TestCounterCreateSendUncertainty keeps attempted ownership and its cause when
// no matched echo established a creator identity; nil is no absence receipt.
func TestCounterCreateSendUncertainty(t *testing.T) {
	var source counterSource
	dump := counterCreateDump()
	want := errors.New("creation send outcome is unconfirmed")
	created, read, attempted, err := counterCreateFinish(t, &source, &dump, want)
	if !errors.Is(err, want) {
		t.Fatalf("unconfirmed creation failure=%v, want %v", err, want)
	}
	if !attempted {
		t.Fatal("send uncertainty lost the caller's attempted native owner")
	}
	if created != nil {
		t.Fatal("send uncertainty invented a created link")
	}
	if read != nil {
		t.Fatal("send uncertainty invented a counter reader")
	}
}

// TestCounterBoundMissingStatsRecovers rejects a current reply without stats,
// despite cached old stats, and accepts a later useful reply of the same owner.
func TestCounterBoundMissingStatsRecovers(t *testing.T) {
	var source counterSource
	creation := counterCreateDump()
	if err := counterCreateDecode(t, &source, &creation, []syscall.NetlinkMessage{counterCreateEcho()}, nl.PidKernel); err != nil {
		t.Fatal(err)
	}
	counterCreateOriginalEcho(t, &creation)
	generation := creation.links[0].generation
	missing := counterCreateQueryReply(t, &source, 251, counterCreateNoStats(251))
	stats, err := counterBoundSelect(t, &source, &missing, generation)
	if err == nil {
		t.Fatal("current missing-stat reply borrowed cached old statistics")
	}
	if stats != nil {
		t.Fatal("current missing-stat reply returned a counter sample")
	}
	message := counterLinkMessage(252, unix.RTM_NEWLINK, 303)
	message.Header.Pid = 191
	useful := counterCreateQueryReply(t, &source, 252, message)
	if useful.links[0].generation != generation {
		t.Fatal("measurement-only unavailability changed the confirmed generation")
	}
	stats, err = counterBoundSelect(t, &source, &useful, generation)
	if err != nil {
		t.Fatal(err)
	}
	if stats == nil {
		t.Fatal("same-owner useful reply returned no measurements")
	}
	if stats.RxBytes != 303 {
		t.Fatalf("recovered raw rx-bytes=%d, want current 303", stats.RxBytes)
	}
}

// TestCounterBoundMeasurements preserves every supplied raw measurement at its
// full width, rather than manufacturing zero error/drop totals after binding.
func TestCounterBoundMeasurements(t *testing.T) {
	var source counterSource
	creation := counterCreateDump()
	if err := counterCreateDecode(t, &source, &creation, []syscall.NetlinkMessage{counterCreateEcho(), counterCreateACK(0)}, nl.PidKernel); err != nil {
		t.Fatal(err)
	}
	generation := creation.links[0].generation
	want := netlink.LinkStatistics{
		RxBytes: 1<<40 + 11, TxBytes: 1<<41 + 13,
		RxPackets: 1<<42 + 17, TxPackets: 1<<43 + 19,
		RxErrors: 1<<44 + 23, TxErrors: 1<<45 + 29,
		RxDropped: 1<<46 + 31, TxDropped: 1<<47 + 37,
		Multicast: 1<<48 + 41,
	}
	raw := make([]byte, 24*8)
	for index, value := range []uint64{
		want.RxPackets, want.TxPackets, want.RxBytes, want.TxBytes,
		want.RxErrors, want.TxErrors, want.RxDropped, want.TxDropped, want.Multicast,
	} {
		nl.NativeEndian().PutUint64(raw[index*8:(index+1)*8], value)
	}
	message := counterCreateNoStats(551)
	message.Data = append(message.Data, nl.NewRtAttr(unix.IFLA_STATS64, raw).Serialize()...)
	reply := counterCreateQueryReply(t, &source, 551, message)
	stats, err := counterBoundSelect(t, &source, &reply, generation)
	if err != nil {
		t.Fatal(err)
	}
	if stats == nil {
		t.Fatal("supplied current native measurements returned no sample")
	}
	for _, field := range []struct {
		name string
		got  uint64
		want uint64
	}{
		{"rx-bytes", stats.RxBytes, want.RxBytes},
		{"tx-bytes", stats.TxBytes, want.TxBytes},
		{"rx-packets", stats.RxPackets, want.RxPackets},
		{"tx-packets", stats.TxPackets, want.TxPackets},
		{"rx-errors", stats.RxErrors, want.RxErrors},
		{"tx-errors", stats.TxErrors, want.TxErrors},
		{"rx-dropped", stats.RxDropped, want.RxDropped},
		{"tx-dropped", stats.TxDropped, want.TxDropped},
		{"multicast", stats.Multicast, want.Multicast},
	} {
		if field.got != field.want {
			t.Errorf("bound %s=%d, want supplied native total %d", field.name, field.got, field.want)
		}
	}
}

// TestCounterBoundCopiesCurrentReply keeps caller mutation out of the source's
// parsed reply and later readings, while preserving the actual current totals.
func TestCounterBoundCopiesCurrentReply(t *testing.T) {
	var source counterSource
	dump := counterCreateDump()
	if err := counterCreateDecode(t, &source, &dump, []syscall.NetlinkMessage{counterCreateEcho(), counterCreateACK(0)}, nl.PidKernel); err != nil {
		t.Fatal(err)
	}
	generation := dump.links[0].generation
	first, err := counterBoundSelect(t, &source, &dump, generation)
	if err != nil {
		t.Fatal(err)
	}
	if first == nil {
		t.Fatal("useful original reply returned no sample")
	}
	first.RxBytes = 999
	second, err := counterBoundSelect(t, &source, &dump, generation)
	if err != nil {
		t.Fatal(err)
	}
	if second == nil {
		t.Fatal("later useful original reply returned no sample")
	}
	if second.RxBytes != 101 {
		t.Fatalf("caller mutation changed later raw rx-bytes to %d", second.RxBytes)
	}
}

// TestCounterBoundRejectsWrongReply requires a current reply of the original
// index; cached original statistics cannot answer an absent or different link.
func TestCounterBoundRejectsWrongReply(t *testing.T) {
	for _, replyPresent := range []bool{false, true} {
		name := "missing-reply"
		if replyPresent {
			name = "different-index"
		}
		t.Run(name, func(t *testing.T) {
			var source counterSource
			original := counterCreateDump()
			if err := counterCreateDecode(t, &source, &original, []syscall.NetlinkMessage{counterCreateEcho(), counterCreateACK(0)}, nl.PidKernel); err != nil {
				t.Fatal(err)
			}
			generation := original.links[0].generation
			reply := counterDump{sequence: 451, portID: 191, single: true}
			if replyPresent {
				message := counterLinkMessage(451, unix.RTM_NEWLINK, 505)
				message.Header.Pid = 191
				nl.DeserializeIfInfomsg(message.Data).Index = 8
				attributes, err := nl.ParseRouteAttr(message.Data[unix.SizeofIfInfomsg:])
				if err != nil {
					t.Fatal(err)
				}
				for index := range attributes {
					if attributes[index].Attr.Type == unix.IFLA_IFNAME {
						copy(attributes[index].Value, nl.ZeroTerminated("counter1"))
					}
				}
				if err := source.consume(&reply, message); err != nil {
					t.Fatal(err)
				}
			}
			if err := source.consume(&reply, counterCreateACKFor(451, unix.RTM_GETLINK, unix.NLM_F_ACK, 0)); err != nil {
				t.Fatal(err)
			}
			stats, err := counterBoundSelect(t, &source, &reply, generation)
			if err == nil {
				t.Fatal("bound selector accepted no current reply of the original")
			}
			if stats != nil {
				t.Fatal("bound selector borrowed counters from outside its reply")
			}
		})
	}
}

// TestCounterBoundGenerationLossPermanent refuses both an old reply and a new
// replacement reply after ordered deletion or a lost notification history.
func TestCounterBoundGenerationLossPermanent(t *testing.T) {
	cases := []struct {
		name        string
		historyLost bool
	}{
		{"delete-and-replace", false},
		{"history-loss", true},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			var source counterSource
			original := counterCreateDump()
			if err := counterCreateDecode(t, &source, &original, []syscall.NetlinkMessage{counterCreateEcho(), counterCreateACK(0)}, nl.PidKernel); err != nil {
				t.Fatal(err)
			}
			generation := original.links[0].generation
			if test.historyLost {
				source.invalidate()
			} else {
				if err := source.consume(&original, counterLinkMessage(0, unix.RTM_DELLINK, 101)); err != nil {
					t.Fatal(err)
				}
			}
			message := counterLinkMessage(351, unix.RTM_NEWLINK, 404)
			message.Header.Pid = 191
			fresh := counterCreateQueryReply(t, &source, 351, message)
			if fresh.links[0].generation == generation {
				t.Fatal("replacement reused the original generation")
			}
			for _, reply := range []*counterDump{&original, &fresh} {
				stats, err := counterBoundSelect(t, &source, reply, generation)
				if err == nil {
					t.Fatal("old owner accepted a reply after its continuity ended")
				}
				if stats != nil {
					t.Fatal("ended owner published a counter sample")
				}
			}
		})
	}
}

// TestCounterBoundClosedSourceRefuses rejects a retained reply after its source
// closes and keeps the actual bound reader from reopening that source.
func TestCounterBoundClosedSourceRefuses(t *testing.T) {
	var source counterSource
	dump := counterCreateDump()
	messages := []syscall.NetlinkMessage{counterCreateEcho(), counterCreateACK(0)}
	if err := counterCreateDecode(t, &source, &dump, messages, nl.PidKernel); err != nil {
		t.Fatal(err)
	}
	_, read, _, err := counterCreateFinish(t, &source, &dump, nil)
	if err != nil {
		t.Fatal(err)
	}
	if read == nil {
		t.Fatal("confirmed creation returned no bound reader")
	}
	generation := dump.links[0].generation
	source.close()
	stats, err := counterBoundSelect(t, &source, &dump, generation)
	if !errors.Is(err, os.ErrClosed) {
		t.Fatalf("closed-source selection=%v, want os.ErrClosed", err)
	}
	if stats != nil {
		t.Fatal("closed-source selection returned a sample")
	}
	stats, err = read()
	if !errors.Is(err, os.ErrClosed) {
		t.Fatalf("closed-source reader=%v, want os.ErrClosed", err)
	}
	if stats != nil {
		t.Fatal("closed-source reader returned a sample")
	}
	if source.socket != nil {
		t.Fatal("retired reader reopened its source")
	}
}

func counterCreateRequest(kind, flags int) *nl.NetlinkRequest {
	request := nl.NewNetlinkRequest(kind, flags)
	request.AddData(nl.NewIfInfomsg(unix.AF_UNSPEC))
	return request
}

func counterCreateOversizedRequest() *nl.NetlinkRequest {
	request := counterCreateRequest(unix.RTM_NEWLINK, unix.NLM_F_CREATE|unix.NLM_F_EXCL)
	// Seventeen individually well-framed attributes exceed the 1MiB request
	// budget without relying on a malformed payload to trigger a refusal.
	block := make([]byte, 65528)
	for range 17 {
		request.AddData(nl.NewRtAttr(unix.IFLA_IFALIAS, block))
	}
	return request
}

func counterCreateShortHeaderRequest() *nl.NetlinkRequest {
	request := nl.NewNetlinkRequest(unix.RTM_NEWLINK, unix.NLM_F_CREATE|unix.NLM_F_EXCL|unix.NLM_F_ACK)
	request.AddData(&counterCreatePayload{wire: make([]byte, unix.SizeofIfInfomsg-1)})
	return request
}

func counterCreateNamespaceRequest(attribute int) *nl.NetlinkRequest {
	request := counterCreateRequest(unix.RTM_NEWLINK, unix.NLM_F_CREATE|unix.NLM_F_EXCL|unix.NLM_F_ACK)
	request.AddData(nl.NewRtAttr(attribute, nl.Uint32Attr(1)))
	return request
}

func counterCreateHeaderRequest(family uint8, index int32) *nl.NetlinkRequest {
	request := nl.NewNetlinkRequest(unix.RTM_NEWLINK, unix.NLM_F_CREATE|unix.NLM_F_EXCL|unix.NLM_F_ACK)
	header := nl.NewIfInfomsg(unix.AF_UNSPEC)
	header.Family = family
	header.Index = index
	request.AddData(header)
	return request
}

func counterCreateMalformedAttribute(wire []byte) *nl.NetlinkRequest {
	request := counterCreateRequest(unix.RTM_NEWLINK, unix.NLM_F_CREATE|unix.NLM_F_EXCL|unix.NLM_F_ACK)
	request.AddData(&counterCreatePayload{wire: wire})
	return request
}

func counterCreateAttributeLength(declared, actual int) *nl.NetlinkRequest {
	wire := make([]byte, actual)
	nl.NativeEndian().PutUint16(wire[:2], uint16(declared))
	nl.NativeEndian().PutUint16(wire[2:4], unix.IFLA_IFALIAS)
	return counterCreateMalformedAttribute(wire)
}

// The payload is caller-owned data. Its optional source observes only whether
// caller serialization runs outside the source lock; it replaces no native IO.
type counterCreatePayload struct {
	wire           []byte
	source         *counterSource
	lengths        int
	serializations int
	locked         bool
}

func (payload *counterCreatePayload) Len() int {
	payload.observeLock()
	payload.lengths++
	return len(payload.wire)
}

func (payload *counterCreatePayload) Serialize() []byte {
	payload.observeLock()
	payload.serializations++
	return payload.wire
}

func (payload *counterCreatePayload) observeLock() {
	if payload.source == nil {
		return
	}
	if !payload.source.mu.TryLock() {
		payload.locked = true
		return
	}
	payload.source.mu.Unlock()
}

func counterCreateCall(t *testing.T, backend *netlinkBackend, request *nl.NetlinkRequest) (netlink.Link, func() (*netlink.LinkStatistics, error), bool, error) {
	t.Helper()
	creator, supported := any(backend).(interface {
		CreateLinkCounterReader(*nl.NetlinkRequest) (netlink.Link, func() (*netlink.LinkStatistics, error), bool, error)
	})
	if !supported {
		t.Fatal("kernel interface backend does not provide the creation-bound counter capability")
	}
	return creator.CreateLinkCounterReader(request)
}

func counterCreateFinish(t *testing.T, source *counterSource, dump *counterDump, cause error) (netlink.Link, func() (*netlink.LinkStatistics, error), bool, error) {
	t.Helper()
	finisher, supported := any(source).(interface {
		finishCreate(*counterDump, error) (netlink.Link, func() (*netlink.LinkStatistics, error), bool, error)
	})
	if !supported {
		t.Fatal("counter source does not provide the creator result boundary")
	}
	source.mu.Lock()
	defer source.mu.Unlock()
	return finisher.finishCreate(dump, cause)
}

func counterBoundSelect(t *testing.T, source *counterSource, dump *counterDump, generation uint64) (*netlink.LinkStatistics, error) {
	t.Helper()
	selector, supported := any(source).(interface {
		boundCounters(*counterDump, int, uint64) (*netlink.LinkStatistics, error)
	})
	if !supported {
		t.Fatal("counter source does not provide its bound counter selector")
	}
	source.mu.Lock()
	defer source.mu.Unlock()
	return selector.boundCounters(dump, 7, generation)
}

func counterCreateNoStats(sequence uint32) syscall.NetlinkMessage {
	message := nl.NewIfInfomsg(unix.AF_UNSPEC)
	message.Index = 7
	data := append([]byte(nil), message.Serialize()...)
	data = append(data, nl.NewRtAttr(unix.IFLA_IFNAME, nl.ZeroTerminated("counter0")).Serialize()...)
	return syscall.NetlinkMessage{
		Header: syscall.NlMsghdr{Type: unix.RTM_NEWLINK, Seq: sequence, Pid: 191},
		Data:   data,
	}
}

func counterCreateQueryReply(t *testing.T, source *counterSource, sequence uint32, message syscall.NetlinkMessage) counterDump {
	t.Helper()
	dump := counterDump{sequence: sequence, portID: 191, single: true}
	source.mu.Lock()
	defer source.mu.Unlock()
	if err := source.consume(&dump, message); err != nil {
		t.Fatal(err)
	}
	if err := source.consume(&dump, counterCreateACKFor(sequence, unix.RTM_GETLINK, unix.NLM_F_ACK, 0)); err != nil {
		t.Fatal(err)
	}
	return dump
}
