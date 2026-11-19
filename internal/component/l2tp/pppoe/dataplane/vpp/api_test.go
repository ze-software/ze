// Design: docs/architecture/l2tp/bng-5-pppoe.md -- VPP session programming
// VALIDATES: requests target created interfaces and retain API refusals.
package vpp

import (
	"context"
	"errors"
	"testing"

	"go.fd.io/govpp/api"
	interfaces "go.fd.io/govpp/binapi/interface"
	"go.fd.io/govpp/binapi/interface_types"
	"go.fd.io/govpp/binapi/ip"
	"go.fd.io/govpp/binapi/memclnt"

	pppoeapi "github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane/vpp/binapi/pppoe"
)

type testStream struct {
	ctx      context.Context
	sent     []api.Message
	replies  []api.Message
	err      error
	closeErr error
	closed   int
}

func (s *testStream) Context() context.Context    { return s.ctx }
func (s *testStream) SendMsg(m api.Message) error { s.sent = append(s.sent, m); return nil }
func (s *testStream) RecvMsg() (api.Message, error) {
	if s.err != nil {
		return nil, s.err
	}
	if len(s.replies) == 0 {
		return nil, errors.New("unexpected receive")
	}
	m := s.replies[0]
	s.replies = s.replies[1:]
	return m, nil
}
func (s *testStream) Close() error { s.closed++; return s.closeErr }

// A successful create uses the patched field and returns only the actual VPP
// native index. A refused reply must never turn into a synthetic interface.
func TestVPPSessionAPIUsesPatchedCreate(t *testing.T) {
	stream := &testStream{replies: []api.Message{&pppoeapi.PppoeAddDelSessionReply{SwIfIndex: 73}}}
	client := binaryAPI{open: func(ctx context.Context) (api.Stream, error) { stream.ctx = ctx; return stream, nil }}
	receipt, err := client.create(context.Background(), 19, [6]byte{2, 0, 0, 0, 0, 1}, 41)
	if err != nil || receipt.index != 73 || !receipt.confirmed {
		t.Fatalf("create = %+v, %v", receipt, err)
	}
	req, ok := stream.sent[0].(*pppoeapi.PppoeAddDelSession)
	if !ok {
		t.Fatalf("request %T", stream.sent[0])
	}
	if !req.IsAdd || !req.DisableFib || req.SessionID != 19 || req.DecapVrfID != 41 {
		t.Fatalf("request %+v", req)
	}
	if stream.closed != 1 {
		t.Fatalf("stream close count %d", stream.closed)
	}
}

// Both address families read the access interface's configured table, including
// legitimate table zero, instead of treating an unavailable table as default.
func TestVPPAccessTablesUseReadback(t *testing.T) {
	var requests []*interfaces.SwInterfaceGetTable
	client := binaryAPI{open: func(ctx context.Context) (api.Stream, error) {
		stream := &testStream{ctx: ctx, replies: []api.Message{&interfaces.SwInterfaceGetTableReply{VrfID: 0}}}
		return &recordTableStream{testStream: stream, requests: &requests}, nil
	}}
	tables, err := client.tables(context.Background(), 17)
	if err != nil || tables != [2]uint32{} {
		t.Fatalf("tables %v, %v", tables, err)
	}
	if len(requests) != 2 || requests[0].SwIfIndex != 17 || requests[0].IsIPv6 || !requests[1].IsIPv6 {
		t.Fatalf("queries %+v", requests)
	}
}

type recordTableStream struct {
	*testStream
	requests *[]*interfaces.SwInterfaceGetTable
}

func (s *recordTableStream) SendMsg(m api.Message) error {
	if req, ok := m.(*interfaces.SwInterfaceGetTable); ok {
		*s.requests = append(*s.requests, req)
	}
	return s.testStream.SendMsg(m)
}

// Readback must finish its control-ping exchange, and each stream must have a
// caller-derived finite deadline and close on every refusal.
func TestVPPAPIReadbackRequiresCompletion(t *testing.T) {
	want := errors.New("stream close")
	stream := &testStream{replies: []api.Message{&interfaces.SwInterfaceDetails{SwIfIndex: 8, InterfaceName: "pppoe_session4"}}, closeErr: want}
	client := binaryAPI{open: func(ctx context.Context) (api.Stream, error) {
		if _, ok := ctx.Deadline(); !ok {
			t.Fatal("unbounded stream")
		}
		stream.ctx = ctx
		return stream, nil
	}}
	if _, err := client.interfaceName(context.Background(), 8); err == nil || !errors.Is(err, want) {
		t.Fatalf("incomplete readback error %v", err)
	}
	if stream.closed != 1 {
		t.Fatalf("closed %d times", stream.closed)
	}
	if _, ok := stream.sent[1].(*memclnt.ControlPing); !ok {
		t.Fatalf("completion request %T", stream.sent[1])
	}
}

// Native API refusal, unexpected reply type and invalid created indexes all
// remain refusals; none can be published as a usable session.
func TestVPPAPICreateRefusesInvalidReplies(t *testing.T) {
	for _, reply := range []api.Message{&pppoeapi.PppoeAddDelSessionReply{Retval: -1}, &pppoeapi.PppoeAddDelSessionReply{}, &pppoeapi.PppoeAddDelSessionReply{SwIfIndex: interface_types.InterfaceIndex(^uint32(0))}, &ip.IPRouteAddDelV2Reply{}} {
		stream := &testStream{replies: []api.Message{reply}}
		client := binaryAPI{open: func(ctx context.Context) (api.Stream, error) { stream.ctx = ctx; return stream, nil }}
		if _, err := client.create(context.Background(), 1, [6]byte{2}, 0); err == nil {
			t.Fatalf("accepted reply %+v", reply)
		}
	}
}
