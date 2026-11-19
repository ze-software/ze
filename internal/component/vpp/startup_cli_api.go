// Design: docs/guide/vpp.md -- narrow typed startup CLI binary API
// Related: register_startup_cli_api.go -- GoVPP typed-message registration
// API: https://github.com/FDio/vpp/blob/cbba0451bb0af02a3ab8e163f6f99062782258e6/src/vlibmemory/vlib.api
// cli_inband and cli_inband_reply have the same fields as the GoVPP v0.13.0
// definitions and CRCs: https://github.com/FDio/govpp/blob/v0.13.0/binapi/vlib/vlib.ba.go
// This is a hand-maintained narrow binding, not claimed generator output.
package vpp

import (
	"encoding/binary"
	"errors"
	"io"

	"go.fd.io/govpp/api"
)

// These are raw native DTOs; zero means an empty string and zero native
// result, not a validated startup request or completion. GoVPP constructs
// non-nil instances before decoding. Bounds precede every slice and a failed
// decode preserves the prior DTO. They are not safe for concurrent mutation.
type startupCLIRequest struct {
	Cmd string `binapi:"string[],name=cmd" json:"cmd,omitempty"`
}
type startupCLIReply struct {
	Retval int32  `binapi:"i32,name=retval" json:"retval,omitempty"`
	Reply  string `binapi:"string[],name=reply" json:"reply,omitempty"`
}

func (m *startupCLIRequest) Reset()                        { *m = startupCLIRequest{} }
func (*startupCLIRequest) GetMessageName() string          { return "cli_inband" }
func (*startupCLIRequest) GetCrcString() string            { return "f8377302" }
func (*startupCLIRequest) GetMessageType() api.MessageType { return api.RequestMessage }
func (m *startupCLIRequest) Size() int                     { return 4 + len(m.Cmd) }
func (m *startupCLIRequest) Marshal(out []byte) ([]byte, error) {
	if uint64(len(m.Cmd)) > uint64(^uint32(0)) {
		return nil, errors.New("VPP CLI command exceeds native string length")
	}
	if out == nil {
		out = make([]byte, m.Size())
	}
	if len(out) < m.Size() {
		return nil, io.ErrShortBuffer
	}
	binary.BigEndian.PutUint32(out[:4], uint32(len(m.Cmd)))
	copy(out[4:m.Size()], m.Cmd)
	return out[:m.Size()], nil
}
func (m *startupCLIRequest) Unmarshal(data []byte) error {
	if len(data) < 4 {
		return io.ErrUnexpectedEOF
	}
	length := binary.BigEndian.Uint32(data[:4])
	if uint64(length) != uint64(len(data)-4) {
		return errors.New("VPP CLI command length differs from its native body")
	}
	m.Cmd = string(data[4:])
	return nil
}

func (m *startupCLIReply) Reset()                        { *m = startupCLIReply{} }
func (*startupCLIReply) GetMessageName() string          { return "cli_inband_reply" }
func (*startupCLIReply) GetCrcString() string            { return "05879051" }
func (*startupCLIReply) GetMessageType() api.MessageType { return api.ReplyMessage }
func (m *startupCLIReply) Size() int                     { return 8 + len(m.Reply) }
func (m *startupCLIReply) Marshal(out []byte) ([]byte, error) {
	if uint64(len(m.Reply)) > uint64(^uint32(0)) {
		return nil, errors.New("VPP CLI output exceeds native string length")
	}
	if out == nil {
		out = make([]byte, m.Size())
	}
	if len(out) < m.Size() {
		return nil, io.ErrShortBuffer
	}
	binary.BigEndian.PutUint32(out[:4], uint32(m.Retval))
	binary.BigEndian.PutUint32(out[4:8], uint32(len(m.Reply)))
	copy(out[8:m.Size()], m.Reply)
	return out[:m.Size()], nil
}
func (m *startupCLIReply) Unmarshal(data []byte) error {
	if len(data) < 8 {
		return io.ErrUnexpectedEOF
	}
	length := binary.BigEndian.Uint32(data[4:8])
	if uint64(length) != uint64(len(data)-8) {
		return errors.New("VPP CLI output length differs from its native body")
	}
	result := int32(binary.BigEndian.Uint32(data[:4]))
	output := string(data[8:])
	m.Retval, m.Reply = result, output
	return nil
}
