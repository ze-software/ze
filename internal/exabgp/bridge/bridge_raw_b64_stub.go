// Design: docs/architecture/exabgp-bridge.md -- non-ze_bgp stub for base64 UPDATE decoding.
// Related: bridge_raw_b64.go -- ze_bgp implementation.

//go:build !ze_bgp

package bridge

import (
	"errors"
	"log/slog"
)

// rawWireBuildEnabled records whether this build carries the BGP wire engine.
const rawWireBuildEnabled = false

// Positional direction flag matching bridge_raw_b64.go.
const rawB64FlagSent uint8 = 1 << 0

var errRawB64Unsupported = errors.New("base64 update decoding requires ze_bgp build tag")

// RawUpdate holds the parsed positional fields and decoded BGP UPDATE wire
// body from one `base64` `upd` event line.
type RawUpdate struct {
	Peer         string
	Local        string
	RouterID     string
	ExtraAddPath string
	ConnID       uint64
	LocalAS      uint32
	RemoteAS     uint32
	Flags        uint8
	Body         []byte
}

// Direction reports the ExaBGP event direction ("receive" or "send") derived
// from rawB64FlagSent.
func (r RawUpdate) Direction() string {
	if r.isSent() {
		return modeSend
	}
	return modeReceive
}

// isSent reports whether this UPDATE was sent by Ze rather than received.
func (r RawUpdate) isSent() bool {
	return r.Flags&rawB64FlagSent != 0
}

// SupportsRawWire reports whether this binary was compiled with the BGP engine
// (`ze_bgp`) and can decode `base64` UPDATE lines directly from wire bytes.
func SupportsRawWire() bool {
	return rawWireBuildEnabled
}

// ReadRawB64Update refuses base64 lines in builds compiled without `ze_bgp`.
func ReadRawB64Update(_ string, _ []byte) (RawUpdate, error) {
	return RawUpdate{}, errRawB64Unsupported
}

// RenderRawUpdate returns no output in builds compiled without `ze_bgp`.
func (s *Sessions) RenderRawUpdate(_ RawUpdate, _ Format, _ Forms, _ bool, _ *slog.Logger) []byte {
	return nil
}
