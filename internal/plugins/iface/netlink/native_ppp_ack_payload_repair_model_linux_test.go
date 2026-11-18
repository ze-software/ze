// Design: docs/features/interfaces.md -- strict original PPP ACK payload models
// These are non-native models. No descriptor fixture is sent to a native API.

//go:build linux

package ifacenetlink

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"testing"

	"github.com/vishvananda/netlink/nl"
	"golang.org/x/sys/unix"
)

// TestNativePPPModelACKPayloadBoundaries pairs useful exact success/full-NACK
// controls with extra bytes inside NLMSG_ERROR and unsupported flag cases.
// Outer framing remains valid in these corruptions, so complete-datagram
// validation alone cannot detect them. The existing parser must go RED here.
func TestNativePPPModelACKPayloadBoundaries(t *testing.T) {
	request := nativePPPLinkRequest(37, 0, 0, nl.NewRtAttr(unix.IFLA_MTU, nl.Uint32Attr(1431)))
	request.Seq = 41
	request.Pid = 73
	original := request.Serialize()
	// Numeric wire-model value only; no native authority is created.
	nl.NativeEndian().PutUint32(original[len(original)-4:], 19)
	digest := sha256.Sum256(original)
	positive := nativePPPModelACK(original, 0, true)
	negative := nativePPPModelACK(original, -int32(unix.ESRCH), false)
	for _, control := range []struct {
		name   string
		wire   []byte
		wanted error
	}{
		{"exact capped success", positive, nil},
		{"exact uncapped full NACK", negative, unix.ESRCH},
	} {
		t.Run(control.name, func(t *testing.T) {
			nativeErr, parseErr := nativePPPParseACK(request.NlMsghdr, digest, control.wire)
			if parseErr != nil {
				t.Fatalf("useful strict ACK control failed: %v", parseErr)
			}
			if !errors.Is(nativeErr, control.wanted) {
				t.Fatalf("native error=%v, want %v", nativeErr, control.wanted)
			}
		})
	}
	for _, malformed := range []struct {
		name string
		wire []byte
	}{
		{"success inner aligned extra payload", nativePPPModelACKInnerTail(positive, []byte{1, 2, 3, 4})},
		{"NACK inner aligned extra payload", nativePPPModelACKInnerTail(negative, []byte{1, 2, 3, 4})},
		{"success inner aligned zero payload", nativePPPModelACKInnerTail(positive, []byte{0, 0, 0, 0})},
		{"NACK inner aligned zero payload", nativePPPModelACKInnerTail(negative, []byte{0, 0, 0, 0})},
		{"success uncapped", nativePPPModelACKFlags(positive, 0)},
		{"success unexpected ACK TLVs", nativePPPModelACKFlags(positive, unix.NLM_F_CAPPED|unix.NLM_F_ACK_TLVS)},
		{"NACK unexpected ACK TLVs", nativePPPModelACKFlags(negative, unix.NLM_F_ACK_TLVS)},
		{"success unexpected multipart", nativePPPModelACKFlags(positive, unix.NLM_F_CAPPED|unix.NLM_F_MULTI)},
		{"NACK unexpected multipart", nativePPPModelACKFlags(negative, unix.NLM_F_MULTI)},
		{"success unexpected request", nativePPPModelACKFlags(positive, unix.NLM_F_CAPPED|unix.NLM_F_REQUEST)},
		{"NACK unexpected request", nativePPPModelACKFlags(negative, unix.NLM_F_REQUEST)},
		{"success unknown flag", nativePPPModelACKFlags(positive, unix.NLM_F_CAPPED|0x8000)},
		{"NACK unknown flag", nativePPPModelACKFlags(negative, 0x8000)},
	} {
		t.Run(malformed.name, func(t *testing.T) {
			_, parseErr := nativePPPParseACK(request.NlMsghdr, digest, malformed.wire)
			if parseErr == nil {
				t.Fatal("malformed inner ACK payload/flags became a definitive outcome")
			}
		})
	}
	// This first NACK is independently exact and digest-valid. Only the outer
	// trailer is bad, so its original errno must survive the Unknown verdict.
	nativeErr, parseErr := nativePPPParseACK(request.NlMsghdr, digest,
		append(bytes.Clone(negative), 0x7f))
	if parseErr == nil {
		t.Fatal("malformed outer trailer became a definitive outcome")
	}
	if !errors.Is(nativeErr, unix.ESRCH) {
		t.Fatal("bad outer trailer lost the independently valid original NACK errno")
	}
}

func nativePPPModelACKInnerTail(original, tail []byte) []byte {
	wire := append(bytes.Clone(original), tail...)
	// Grow the one message, not just its outer datagram. The byte-count and
	// alignment remain valid so the test specifically judges inner payload.
	nl.NativeEndian().PutUint32(wire[:4], uint32(len(wire)))
	return wire
}

func nativePPPModelACKFlags(original []byte, flags uint16) []byte {
	wire := bytes.Clone(original)
	nl.NativeEndian().PutUint16(wire[6:8], flags)
	return wire
}
