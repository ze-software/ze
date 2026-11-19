// Design: docs/guide/vpp.md -- narrow typed native startup CLI messages
// Related: startup_cli_api.go -- bounds precede every native length read
package vpp

import (
	"bytes"
	"encoding/binary"
	"testing"

	"go.fd.io/govpp/api"
	"go.fd.io/govpp/codec"
)

// Actual GoVPP codec framing must use the declared request/reply types, CRCs,
// signed Retval and native length-prefixed strings, without a fixture codec.
func TestVPPStartupCLITypedCodecMatchesNativeLayout(t *testing.T) {
	request := &startupCLIRequest{Cmd: "exec /a.cli"}
	if request.GetMessageName() != "cli_inband" {
		t.Fatal("request name")
	}
	if request.GetCrcString() != "f8377302" {
		t.Fatal("request CRC")
	}
	if request.GetMessageType() != api.RequestMessage {
		t.Fatal("request kind")
	}
	body, err := request.Marshal(nil)
	if err != nil {
		t.Fatal(err)
	}
	want := append([]byte{0, 0, 0, 11}, []byte("exec /a.cli")...)
	if !bytes.Equal(body, want) {
		t.Fatalf("request body %x, want %x", body, want)
	}
	frame, err := codec.DefaultCodec.EncodeMsg(request, 731)
	if err != nil {
		t.Fatal(err)
	}
	if binary.BigEndian.Uint16(frame[:2]) != 731 {
		t.Fatal("request ID")
	}
	if !bytes.Equal(frame[10:], want) {
		t.Fatal("GoVPP request header/body changed")
	}
	var decoded startupCLIRequest
	if err := codec.DefaultCodec.DecodeMsg(frame, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Cmd != request.Cmd {
		t.Fatal("request round trip")
	}
	reply := &startupCLIReply{Retval: -7, Reply: "failed"}
	if reply.GetMessageName() != "cli_inband_reply" {
		t.Fatal("reply name")
	}
	if reply.GetCrcString() != "05879051" {
		t.Fatal("reply CRC")
	}
	if reply.GetMessageType() != api.ReplyMessage {
		t.Fatal("reply kind")
	}
	frame, err = codec.DefaultCodec.EncodeMsg(reply, 732)
	if err != nil {
		t.Fatal(err)
	}
	want = []byte{0xff, 0xff, 0xff, 0xf9, 0, 0, 0, 6, 'f', 'a', 'i', 'l', 'e', 'd'}
	if !bytes.Equal(frame[6:], want) {
		t.Fatalf("native reply body %x, want %x", frame[6:], want)
	}
	var received startupCLIReply
	if err := codec.DefaultCodec.DecodeMsg(frame, &received); err != nil {
		t.Fatal(err)
	}
	if received != *reply {
		t.Fatal("signed native error or output lost", received)
	}
	request.Reset()
	reply.Reset()
	if request.Cmd != "" {
		t.Fatal("request reset")
	}
	if reply.Retval != 0 {
		t.Fatal("reply reset retval")
	}
	if reply.Reply != "" {
		t.Fatal("reply reset text")
	}
}

// Malformed native lengths refuse before a slice or decode can panic, and
// leave the prior decoded value unchanged. Truncation is never a success.
func TestVPPStartupCLITypedCodecRefusesMalformedBodies(t *testing.T) {
	for _, body := range [][]byte{nil, {0}, {0, 0, 0}, {0, 0, 0, 4}, {0, 0, 0, 2, 'x'}, {0, 0, 0, 0, 'x'}, {0xff, 0xff, 0xff, 0xff}} {
		request := startupCLIRequest{Cmd: "old"}
		if err := request.Unmarshal(body); err == nil {
			t.Fatal("malformed request admitted", body)
		}
		if request.Cmd != "old" {
			t.Fatal("refusal damaged prior request")
		}
	}
	for _, body := range [][]byte{nil, {0}, {0, 0, 0, 0}, {0, 0, 0, 0, 0, 0, 0}, {0, 0, 0, 0, 0, 0, 0, 4}, {0, 0, 0, 0, 0, 0, 0, 0, 'x'}, {0, 0, 0, 0, 0xff, 0xff, 0xff, 0xff}} {
		reply := startupCLIReply{Retval: -1, Reply: "old"}
		if err := reply.Unmarshal(body); err == nil {
			t.Fatal("malformed reply admitted", body)
		}
		if reply != (startupCLIReply{Retval: -1, Reply: "old"}) {
			t.Fatal("refusal damaged prior reply")
		}
	}
	request := startupCLIRequest{Cmd: "exec /a.cli"}
	if _, err := request.Marshal(make([]byte, request.Size()-1)); err == nil {
		t.Fatal("short request output accepted")
	}
	reply := startupCLIReply{Reply: "done"}
	if _, err := reply.Marshal(make([]byte, reply.Size()-1)); err == nil {
		t.Fatal("short reply output accepted")
	}
	// Empty strings are valid raw wire DTOs even though ExecuteStartupCLI
	// requires one nonempty configured path before constructing a request.
	if err := request.Unmarshal([]byte{0, 0, 0, 0}); err != nil {
		t.Fatal(err)
	}
	if err := reply.Unmarshal([]byte{0, 0, 0, 0, 0, 0, 0, 0}); err != nil {
		t.Fatal(err)
	}
}

// GoVPP's actual registry must see both concrete typed messages. A method
// whose codec exists but was never registered cannot receive a typed reply.
func TestVPPStartupCLITypedMessagesRegistered(t *testing.T) {
	foundRequest, foundReply := false, false
	for _, messages := range api.GetRegisteredMessages() {
		for key, message := range messages {
			if key == "cli_inband_f8377302" {
				if _, ok := message.(*startupCLIRequest); ok {
					foundRequest = true
				}
			}
			if key == "cli_inband_reply_05879051" {
				if _, ok := message.(*startupCLIReply); ok {
					foundReply = true
				}
			}
		}
	}
	if !foundRequest {
		t.Fatal("typed request is absent from actual GoVPP registry")
	}
	if !foundReply {
		t.Fatal("typed reply is absent from actual GoVPP registry")
	}
}
