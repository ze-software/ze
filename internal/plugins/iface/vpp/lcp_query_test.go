// Design: docs/research/vpp-deployment-reference.md -- actual LCP pair readback

package ifacevpp

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"go.fd.io/govpp/api"
	"go.fd.io/govpp/binapi/interface_types"
	"go.fd.io/govpp/binapi/lcp"

	"github.com/ze-software/ze/internal/component/iface"
	vppcomp "github.com/ze-software/ze/internal/component/vpp"
)

type lcpQueryStream struct {
	ctx      context.Context
	request  api.Message
	messages []api.Message
	received int
	closed   int
	sendErr  error
	recvErr  error
	closeErr error
}

func (s *lcpQueryStream) Context() context.Context    { return s.ctx }
func (s *lcpQueryStream) SendMsg(m api.Message) error { s.request = m; return s.sendErr }
func (s *lcpQueryStream) RecvMsg() (api.Message, error) {
	if s.recvErr != nil {
		return nil, s.recvErr
	}
	if s.received == len(s.messages) {
		return nil, io.EOF
	}
	m := s.messages[s.received]
	s.received++
	return m, nil
}
func (s *lcpQueryStream) Close() error { s.closed++; return s.closeErr }

func validLCPDetails() *lcp.LcpItfPairDetails {
	return &lcp.LcpItfPairDetails{PhySwIfIndex: 5, HostSwIfIndex: 17, VifIndex: 42,
		HostIfName: "actualcp", HostIfType: lcp.LCP_API_ITF_HOST_TAP, Netns: "dataplane"}
}

func validLCPReply() *lcp.LcpItfPairGetV2Reply {
	return &lcp.LcpItfPairGetV2Reply{Cursor: ^uint32(0)}
}

// streamLCPDetails returns the details message a valid readback carries
// first, so a case can corrupt one of its fields.
func streamLCPDetails(t *testing.T, s *lcpQueryStream) *lcp.LcpItfPairDetails {
	t.Helper()
	details, ok := s.messages[0].(*lcp.LcpItfPairDetails)
	if !ok {
		t.Fatalf("message 0 is %T, not the details a valid readback starts with", s.messages[0])
	}
	return details
}

// streamLCPReply returns the final reply a valid readback carries second, so
// a case can corrupt one of its fields.
func streamLCPReply(t *testing.T, s *lcpQueryStream) *lcp.LcpItfPairGetV2Reply {
	t.Helper()
	reply, ok := s.messages[1].(*lcp.LcpItfPairGetV2Reply)
	if !ok {
		t.Fatalf("message 1 is %T, not the final reply a valid readback ends with", s.messages[1])
	}
	return reply
}

// TestGetLCPPairUsesActualIdentity proves the name resolver's DP index selects
// exactly one actual pair, without reconstructing the Linux name from it.
func TestGetLCPPairUsesActualIdentity(t *testing.T) {
	withLCPSettings(t, vppcomp.LCPSettings{Enabled: true, Netns: "dataplane"})
	stream := &lcpQueryStream{messages: []api.Message{validLCPDetails(), validLCPReply()}}
	previous := newLCPQueryStream
	newLCPQueryStream = func(ctx context.Context) (api.Stream, error) {
		deadline, ok := ctx.Deadline()
		if !ok {
			t.Fatal("query has no deadline")
		}
		if time.Until(deadline) > lcpPairReadTimeout {
			t.Fatal("query deadline exceeds bound")
		}
		stream.ctx = ctx
		return stream, nil
	}
	t.Cleanup(func() { newLCPQueryStream = previous })
	b := newLCPBackend(&progChannel{})
	pair, err := b.GetLCPPair(context.Background(), "loop0")
	if err != nil {
		t.Fatal(err)
	}
	want := iface.LCPPair{DataplaneIndex: 5, ControlPlaneIndex: 17, LinuxIndex: 42,
		ControlDevice: "actualcp", Namespace: "dataplane"}
	if pair != want {
		t.Fatalf("pair=%+v want=%+v", pair, want)
	}
	request, ok := stream.request.(*lcp.LcpItfPairGetV2)
	if !ok {
		t.Fatalf("request=%T", stream.request)
	}
	if request.SwIfIndex != 5 {
		t.Fatalf("selected index=%d", request.SwIfIndex)
	}
	if stream.received != 2 {
		t.Fatalf("received=%d", stream.received)
	}
	if stream.closed != 1 {
		t.Fatalf("stream closes=%d", stream.closed)
	}
}

// TestLCPPairReadRefusesIncompleteReadback changes every actual pair identity
// or completion property independently; no refusal publishes a guessed pair.
func TestLCPPairReadRefusesIncompleteReadback(t *testing.T) {
	cases := map[string]func(*testing.T, *lcpQueryStream){
		"missing details":   func(_ *testing.T, s *lcpQueryStream) { s.messages = []api.Message{validLCPReply()} },
		"missing final":     func(_ *testing.T, s *lcpQueryStream) { s.messages = s.messages[:1] },
		"duplicate details": func(_ *testing.T, s *lcpQueryStream) { s.messages[1] = validLCPDetails() },
		"wrong DP":          func(t *testing.T, s *lcpQueryStream) { streamLCPDetails(t, s).PhySwIfIndex = 6 },
		"invalid CP": func(t *testing.T, s *lcpQueryStream) {
			streamLCPDetails(t, s).HostSwIfIndex = ^interface_types.InterfaceIndex(0)
		},
		"zero CP":             func(t *testing.T, s *lcpQueryStream) { streamLCPDetails(t, s).HostSwIfIndex = 0 },
		"same indices":        func(t *testing.T, s *lcpQueryStream) { streamLCPDetails(t, s).HostSwIfIndex = 5 },
		"zero Linux index":    func(t *testing.T, s *lcpQueryStream) { streamLCPDetails(t, s).VifIndex = 0 },
		"invalid Linux index": func(t *testing.T, s *lcpQueryStream) { streamLCPDetails(t, s).VifIndex = ^uint32(0) },
		"wrong namespace":     func(t *testing.T, s *lcpQueryStream) { streamLCPDetails(t, s).Netns = "other" },
		"no device":           func(t *testing.T, s *lcpQueryStream) { streamLCPDetails(t, s).HostIfName = "" },
		"long device":         func(t *testing.T, s *lcpQueryStream) { streamLCPDetails(t, s).HostIfName = "sixteencharacter" },
		"TUN":                 func(t *testing.T, s *lcpQueryStream) { streamLCPDetails(t, s).HostIfType = lcp.LCP_API_ITF_HOST_TUN },
		"retval":              func(t *testing.T, s *lcpQueryStream) { streamLCPReply(t, s).Retval = -1 },
		"cursor":              func(t *testing.T, s *lcpQueryStream) { streamLCPReply(t, s).Cursor = 1 },
		"unexpected reply":    func(_ *testing.T, s *lcpQueryStream) { s.messages[1] = &lcp.LcpDefaultNsGetReply{} },
		"typed nil details":   func(_ *testing.T, s *lcpQueryStream) { var d *lcp.LcpItfPairDetails; s.messages[0] = d },
		"typed nil final":     func(_ *testing.T, s *lcpQueryStream) { var r *lcp.LcpItfPairGetV2Reply; s.messages[1] = r },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			stream := &lcpQueryStream{messages: []api.Message{validLCPDetails(), validLCPReply()}}
			change(t, stream)
			pair, err := readLCPPair(context.Background(), 5, "dataplane", func(context.Context) (api.Stream, error) { return stream, nil })
			if err == nil {
				t.Fatal("accepted invalid pair readback")
			}
			if pair != (iface.LCPPair{}) {
				t.Fatalf("failure published pair=%+v", pair)
			}
			if stream.closed != 1 {
				t.Fatalf("stream closes=%d", stream.closed)
			}
			if stream.received > 2 {
				t.Fatalf("unbounded stream read=%d", stream.received)
			}
		})
	}
}

// TestLCPPairReadKeepsIOFailures observes each stream failure and verifies
// closing the stream cannot erase the primary error or publish a success.
func TestLCPPairReadKeepsIOFailures(t *testing.T) {
	for _, stage := range []string{"open", "send", "receive", "close"} {
		t.Run(stage, func(t *testing.T) {
			want := errors.New("stream failure")
			stream := &lcpQueryStream{messages: []api.Message{validLCPDetails(), validLCPReply()}}
			switch stage {
			case "open":
			case "send":
				stream.sendErr = want
			case "receive":
				stream.recvErr = want
			case "close":
				stream.closeErr = want
			}
			pair, err := readLCPPair(context.Background(), 5, "dataplane", func(context.Context) (api.Stream, error) {
				if stage == "open" {
					return nil, want
				}
				return stream, nil
			})
			if !errors.Is(err, want) {
				t.Fatalf("failure lost: %v", err)
			}
			if pair != (iface.LCPPair{}) {
				t.Fatalf("failed read published %+v", pair)
			}
			if stage != "open" {
				if stream.closed != 1 {
					t.Fatalf("stream closes=%d", stream.closed)
				}
			}
		})
	}
}

// TestGetLCPPairRefusesUnavailableAccess proves unresolved or disabled access
// state never reaches the stream factory or produces a synthetic pairing.
func TestGetLCPPairRefusesUnavailableAccess(t *testing.T) {
	for _, stage := range []string{"nil context", "canceled", "no settings", "disabled", "missing device", "local0", "invalid index"} {
		t.Run(stage, func(t *testing.T) {
			withLCPSettings(t, vppcomp.LCPSettings{Enabled: true, Netns: "dataplane"})
			previous := newLCPQueryStream
			newLCPQueryStream = func(context.Context) (api.Stream, error) {
				t.Fatal("unavailable access reached stream factory")
				return nil, errors.New("unreachable")
			}
			t.Cleanup(func() { newLCPQueryStream = previous })
			b := newLCPBackend(&progChannel{})
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			queryContext := ctx
			name := "loop0"
			switch stage {
			case "nil context":
				queryContext = nil
			case "canceled":
				cancel()
			case "no settings":
				getActiveLCPSettings = func() (vppcomp.LCPSettings, bool) { return vppcomp.LCPSettings{}, false }
			case "disabled":
				getActiveLCPSettings = func() (vppcomp.LCPSettings, bool) { return vppcomp.LCPSettings{}, true }
			case "missing device":
				name = "missing"
			case "local0":
				b.names.Add(name, 0, "local0")
			case "invalid index":
				b.names.Add(name, ^uint32(0), "invalid")
			}
			pair, err := b.GetLCPPair(queryContext, name)
			if err == nil {
				t.Fatal("accepted unavailable access")
			}
			if pair != (iface.LCPPair{}) {
				t.Fatalf("failure published %+v", pair)
			}
		})
	}
}

// TestLCPPairReadPreservesEmptyNamespace proves empty means the actual pair's
// namespace rather than silently choosing the configured default name.
func TestLCPPairReadPreservesEmptyNamespace(t *testing.T) {
	details := validLCPDetails()
	details.Netns = ""
	stream := &lcpQueryStream{messages: []api.Message{details, validLCPReply()}}
	pair, err := readLCPPair(context.Background(), 5, "", func(context.Context) (api.Stream, error) { return stream, nil })
	if err != nil {
		t.Fatal(err)
	}
	if pair.Namespace != "" {
		t.Fatalf("invented namespace=%q", pair.Namespace)
	}
	if pair.LinuxIndex != 42 {
		t.Fatalf("lost actual Linux index=%d", pair.LinuxIndex)
	}
	if stream.closed != 1 {
		t.Fatalf("stream closes=%d", stream.closed)
	}
}
