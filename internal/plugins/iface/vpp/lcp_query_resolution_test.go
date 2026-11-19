// Design: docs/research/vpp-deployment-reference.md -- resolved LCP readback

package ifacevpp

import (
	"context"
	"errors"
	"testing"

	"go.fd.io/govpp/api"
	interfaces "go.fd.io/govpp/binapi/interface"
	"go.fd.io/govpp/binapi/lcp"

	vppcomp "github.com/ze-software/ze/internal/component/vpp"
)

// TestGetLCPPairResolvedNameAvoidsSharedChannel supplies an already resolved
// identity on a cold backend. Pair readback must use only its owned stream,
// without lazy shared-channel acquisition or an unrelated interface dump.
func TestGetLCPPairResolvedNameAvoidsSharedChannel(t *testing.T) {
	withLCPSettings(t, vppcomp.LCPSettings{Enabled: true, Netns: "dataplane"})
	previousConnector := getActiveConnector
	getActiveConnector = func() vppConnector {
		t.Fatal("resolved pair query reached the context-free shared channel")
		return nil
	}
	t.Cleanup(func() { getActiveConnector = previousConnector })
	stream := &lcpQueryStream{messages: []api.Message{validLCPDetails(), validLCPReply()}}
	previousStream := newLCPQueryStream
	newLCPQueryStream = func(ctx context.Context) (api.Stream, error) { stream.ctx = ctx; return stream, nil }
	t.Cleanup(func() { newLCPQueryStream = previousStream })
	b := &vppBackendImpl{names: newNameMap()}
	b.names.Add("logical", 5, "actual0")
	pair, err := b.GetLCPPair(context.Background(), "actual0")
	if err != nil {
		t.Fatal(err)
	}
	if pair.DataplaneIndex != 5 {
		t.Fatalf("DP index=%d", pair.DataplaneIndex)
	}
	if stream.closed != 1 {
		t.Fatalf("owned stream closes=%d", stream.closed)
	}
	if b.chReady {
		t.Fatal("pair read acquired the shared channel")
	}
	logical, ok := b.names.lookupName(5)
	if !ok {
		t.Fatal("pair read removed logical reverse identity")
	}
	if logical != "logical" {
		t.Fatalf("pair read changed logical reverse identity: %q", logical)
	}
}

type cancelLCPQueryStream struct {
	*lcpQueryStream
	cancel context.CancelFunc
}

func (s *cancelLCPQueryStream) SendMsg(message api.Message) error {
	s.cancel()
	return s.lcpQueryStream.SendMsg(message)
}

func (s *cancelLCPQueryStream) RecvMsg() (api.Message, error) {
	<-s.ctx.Done()
	return nil, s.ctx.Err()
}

// TestGetLCPPairCancellationClosesOwnedStream cancels at the actual send/receive
// boundary and requires the public query to return that cancellation and close
// its stream. The cold backend has no shared channel to hide an extra read.
func TestGetLCPPairCancellationClosesOwnedStream(t *testing.T) {
	withLCPSettings(t, vppcomp.LCPSettings{Enabled: true, Netns: "dataplane"})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream := &cancelLCPQueryStream{lcpQueryStream: &lcpQueryStream{}, cancel: cancel}
	previous := newLCPQueryStream
	newLCPQueryStream = func(bounded context.Context) (api.Stream, error) { stream.ctx = bounded; return stream, nil }
	t.Cleanup(func() { newLCPQueryStream = previous })
	b := &vppBackendImpl{names: newNameMap()}
	b.names.Add("actual0", 5, "actual0")
	pair, err := b.GetLCPPair(ctx, "actual0")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
	if pair.DataplaneIndex != 0 {
		t.Fatalf("canceled read published %+v", pair)
	}
	if stream.closed != 1 {
		t.Fatalf("owned stream closes=%d", stream.closed)
	}
}

// TestGetLinkLearnsActualNameForLCP uses actual link readback for a registered
// logical alias, then reads its pair by the native name Resolve returns. The
// monitor's reverse logical identity must survive that native-name learning.
func TestGetLinkLearnsActualNameForLCP(t *testing.T) {
	withLCPSettings(t, vppcomp.LCPSettings{Enabled: true, Netns: "dataplane"})
	channel := addressTestChannel()
	channel.details[0].InterfaceName = asciiName("loop1.100")
	b := &vppBackendImpl{ch: channel, names: newNameMap()}
	b.populate.Do(func() {})
	b.names.Add("logical.100", 10, "logical.100")
	info, err := b.GetLink("logical.100")
	if err != nil {
		t.Fatal(err)
	}
	if info.OsName != "loop1.100" {
		t.Fatalf("actual name=%q", info.OsName)
	}
	details := validLCPDetails()
	details.PhySwIfIndex = 10
	stream := &lcpQueryStream{messages: []api.Message{details, validLCPReply()}}
	previous := newLCPQueryStream
	newLCPQueryStream = func(context.Context) (api.Stream, error) { return stream, nil }
	t.Cleanup(func() { newLCPQueryStream = previous })
	pair, err := b.GetLCPPair(context.Background(), info.OsName)
	if err != nil {
		t.Fatal(err)
	}
	if pair.DataplaneIndex != 10 {
		t.Fatalf("DP index=%d", pair.DataplaneIndex)
	}
	logical, ok := b.names.lookupName(10)
	if !ok {
		t.Fatal("native learning removed logical reverse identity")
	}
	if logical != "logical.100" {
		t.Fatalf("native learning changed reverse logical identity: %q", logical)
	}
}

// TestGetLinkLearnsExternalNameForLCP reads a device added after initial map
// population. Its exact readback establishes the native index for pair lookup;
// a substring neighbor cannot establish such an identity.
func TestGetLinkLearnsExternalNameForLCP(t *testing.T) {
	withLCPSettings(t, vppcomp.LCPSettings{Enabled: true, Netns: "dataplane"})
	channel := &dumpChannel{details: []interfaces.SwInterfaceDetails{{SwIfIndex: 5, InterfaceName: asciiName("external0")}}}
	b := &vppBackendImpl{ch: channel, names: newNameMap()}
	b.populate.Do(func() {})
	if _, err := b.GetLink("external"); err == nil {
		t.Fatal("substring read selected its neighbor")
	}
	info, err := b.GetLink("external0")
	if err != nil {
		t.Fatal(err)
	}
	stream := &lcpQueryStream{messages: []api.Message{validLCPDetails(), validLCPReply()}}
	previous := newLCPQueryStream
	newLCPQueryStream = func(context.Context) (api.Stream, error) { return stream, nil }
	t.Cleanup(func() { newLCPQueryStream = previous })
	pair, err := b.GetLCPPair(context.Background(), info.OsName)
	if err != nil {
		t.Fatal(err)
	}
	if pair.DataplaneIndex != 5 {
		t.Fatalf("DP index=%d", pair.DataplaneIndex)
	}
	request, ok := stream.request.(*lcp.LcpItfPairGetV2)
	if !ok {
		t.Fatalf("pair request=%T", stream.request)
	}
	if request.SwIfIndex != 5 {
		t.Fatalf("requested DP index=%d", request.SwIfIndex)
	}
}

// TestGetLCPPairRefusesReservedNativeIndex gives each reserved index its exact
// native name; neither local0 nor the invalid-index sentinel can open a query.
func TestGetLCPPairRefusesReservedNativeIndex(t *testing.T) {
	withLCPSettings(t, vppcomp.LCPSettings{Enabled: true, Netns: "dataplane"})
	previous := newLCPQueryStream
	newLCPQueryStream = func(context.Context) (api.Stream, error) {
		t.Fatal("reserved native index reached pair query")
		return nil, errors.New("unexpected pair query")
	}
	t.Cleanup(func() { newLCPQueryStream = previous })
	for _, index := range []uint32{0, ^uint32(0)} {
		b := &vppBackendImpl{names: newNameMap()}
		b.names.Add("logical", index, "actual0")
		if _, err := b.GetLCPPair(context.Background(), "actual0"); err == nil {
			t.Fatalf("accepted reserved native index %d", index)
		}
	}
}
