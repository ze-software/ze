// Design: docs/architecture/l2tp/bng-5-pppoe.md -- VPP control-map ownership
// Related: api.go -- dump, dumpPing; access_map.go -- readControlBinding;
// binapi/pppoe/pppoe_rpc.ba.go -- the generated dump helpers
// VALIDATES: a dump decodes its details through GoVPP's real stream, which
// resolves every reply in the binapi tree of the message sent last.
package vpp

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"testing"

	"go.fd.io/govpp/adapter"
	"go.fd.io/govpp/api"
	interfaces "go.fd.io/govpp/binapi/interface"
	"go.fd.io/govpp/binapi/memclnt"
	"go.fd.io/govpp/codec"
	"go.fd.io/govpp/core"

	pppoeapi "github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane/vpp/binapi/pppoe"
)

// TestVPPDumpDecodesInTheRequestBinapiTree drives GoVPP's real core
// connection and Stream over a fake VPP endpoint, so Stream.RecvMsg performs
// its own Connection.LookupByID. The targeted control-mapping read sends a
// PPPoE request from Ze's binapi tree and must decode its details and complete;
// the interface readback sends a request from GoVPP's tree and must too. With
// GoVPP's control ping after the PPPoE request, the stream looks the details ID
// up in GoVPP's tree and the read fails with "unknown message ID", as VPP
// startup did in the lab.
func TestVPPDumpDecodesInTheRequestBinapiTree(t *testing.T) {
	endpoint := &treeEndpoint{
		ids:   make(map[string]uint16),
		names: make(map[uint16]string),
		answers: map[string]api.Message{
			messageKey(&pppoeapi.PppoeCpBindingDump{}): &pppoeapi.PppoeCpBindingDetails{DpSwIfIndex: 17, CpSwIfIndex: 18},
			messageKey(&interfaces.SwInterfaceDump{}):  &interfaces.SwInterfaceDetails{SwIfIndex: 8, InterfaceName: "pppoe_session4"},
		},
	}
	connection, err := core.Connect(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(connection.Disconnect)
	client := &binaryAPI{open: func(ctx context.Context) (api.Stream, error) { return connection.NewStream(ctx) }}

	pair, present, err := client.readControlBinding(t.Context(), 17)
	if err != nil {
		t.Fatalf("control mapping read through GoVPP's stream: %v", err)
	}
	if !present {
		t.Fatal("control mapping details were not visited")
	}
	if pair != (controlPair{dataplane: 17, control: 18}) {
		t.Fatalf("control mapping = %+v", pair)
	}
	name, err := client.interfaceName(t.Context(), 8)
	if err != nil {
		t.Fatalf("interface readback through GoVPP's stream: %v", err)
	}
	if name != "pppoe_session4" {
		t.Fatalf("interface name = %q", name)
	}
	if endpoint.answered != 2 {
		t.Fatalf("endpoint answered %d dumps, want 2", endpoint.answered)
	}
	if endpoint.err != nil {
		t.Fatal(endpoint.err)
	}
}

// TestVPPGeneratedDumpHelperDecodesInTheRequestBinapiTree drives the
// generated PPPoE RPC helper over the same real GoVPP stream. The helper ends
// its dump with the control ping of the memclnt package it imports. Generated
// under GoVPP's import prefix alone, that was GoVPP's ping, so the stream
// looked the details ID up in GoVPP's tree and the first Recv failed with
// "unknown message ID". The details must arrive and the stream must end at the
// ping reply.
func TestVPPGeneratedDumpHelperDecodesInTheRequestBinapiTree(t *testing.T) {
	endpoint := &treeEndpoint{
		ids:   make(map[string]uint16),
		names: make(map[uint16]string),
		answers: map[string]api.Message{
			messageKey(&pppoeapi.PppoeCpBindingDump{}): &pppoeapi.PppoeCpBindingDetails{DpSwIfIndex: 17, CpSwIfIndex: 18},
		},
	}
	connection, err := core.Connect(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(connection.Disconnect)

	stream, err := pppoeapi.NewServiceClient(connection).PppoeCpBindingDump(t.Context(), &pppoeapi.PppoeCpBindingDump{SwIfIndex: 17})
	if err != nil {
		t.Fatalf("generated binding dump did not start: %v", err)
	}
	details, err := stream.Recv()
	if err != nil {
		t.Fatalf("binding details through the generated helper: %v", err)
	}
	if details.DpSwIfIndex != 17 {
		t.Fatalf("details DP interface = %d, want 17", details.DpSwIfIndex)
	}
	if details.CpSwIfIndex != 18 {
		t.Fatalf("details CP interface = %d, want 18", details.CpSwIfIndex)
	}
	if _, err := stream.Recv(); !errors.Is(err, io.EOF) {
		t.Fatalf("generated dump did not end at its control ping reply: %v", err)
	}
	if endpoint.answered != 1 {
		t.Fatalf("endpoint answered %d dumps, want 1", endpoint.answered)
	}
	if endpoint.err != nil {
		t.Fatal(endpoint.err)
	}
}

// A request whose binapi tree has no control ping is refused before a stream
// opens, so a dump can never end with a ping that strands its replies.
func TestVPPDumpRefusesRequestWithoutTreePing(t *testing.T) {
	client := &binaryAPI{open: func(context.Context) (api.Stream, error) {
		t.Error("dump opened a stream for a request with no control ping")
		return nil, errors.New("unexpected stream")
	}}
	if err := client.dump(t.Context(), foreignTreeRequest{}, 1, func(api.Message) error { return nil }); err == nil {
		t.Fatal("dump accepted a request from a tree with no control ping")
	}
}

// foreignTreeRequest is a value message: it belongs to no binapi tree.
type foreignTreeRequest struct{}

func (foreignTreeRequest) GetMessageName() string          { return "foreign_dump" }
func (foreignTreeRequest) GetCrcString() string            { return "00000000" }
func (foreignTreeRequest) GetMessageType() api.MessageType { return api.RequestMessage }

func messageKey(message api.Message) string {
	return message.GetMessageName() + "_" + message.GetCrcString()
}

// treeEndpoint is the VPP side of GoVPP's adapter.VppAPI. It numbers each
// message by name and CRC, as VPP's message table does, so the control ping
// of GoVPP's and Ze's memclnt share one ID. A dump request it has an answer
// for is answered when its ping arrives: the details, then the ping reply,
// each carrying its own request's context. GoVPP's synchronous core.Connect
// starts no worker, and the callback runs inside SendMsg, so every method runs
// on the test goroutine: not safe for concurrent use.
type treeEndpoint struct {
	ids            map[string]uint16
	names          map[uint16]string
	answers        map[string]api.Message
	callback       adapter.MsgCallback
	pending        api.Message
	pendingContext uint32
	answered       int
	err            error
}

func (e *treeEndpoint) Connect() error                              { return nil }
func (e *treeEndpoint) Disconnect() error                           { return nil }
func (e *treeEndpoint) WaitReady() error                            { return nil }
func (e *treeEndpoint) SetMsgCallback(callback adapter.MsgCallback) { e.callback = callback }

func (e *treeEndpoint) GetMsgID(name, crc string) (uint16, error) {
	key := name + "_" + crc
	if id, known := e.ids[key]; known {
		return id, nil
	}
	id := uint16(len(e.ids) + 1)
	e.ids[key] = id
	e.names[id] = key
	return id, nil
}

func (e *treeEndpoint) SendMsg(requestContext uint32, data []byte) error {
	if len(data) < 2 {
		return errors.New("tree endpoint: request shorter than its message ID")
	}
	key := e.names[binary.BigEndian.Uint16(data)]
	if answer, known := e.answers[key]; known {
		e.pending, e.pendingContext = answer, requestContext
		return nil
	}
	if key != messageKey(&memclnt.ControlPing{}) {
		return fmt.Errorf("tree endpoint: unexpected request %q", key)
	}
	if e.pending != nil {
		e.reply(e.pendingContext, e.pending)
		e.pending = nil
		e.answered++
	}
	// The reply bytes are the same whichever memclnt binding encodes them.
	e.reply(requestContext, &memclnt.ControlPingReply{})
	return nil
}

func (e *treeEndpoint) reply(requestContext uint32, message api.Message) {
	id, err := e.GetMsgID(message.GetMessageName(), message.GetCrcString())
	if err != nil {
		e.err = errors.Join(e.err, err)
		return
	}
	data, err := codec.DefaultCodec.EncodeMsg(message, id)
	if err != nil {
		e.err = errors.Join(e.err, err)
		return
	}
	// A reply's context follows its two-byte message ID (codec getOffset).
	binary.BigEndian.PutUint32(data[2:6], requestContext)
	e.callback(id, data)
}
