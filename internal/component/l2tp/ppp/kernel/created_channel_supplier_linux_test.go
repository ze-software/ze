// Design: docs/research/l2tpv2-ze-integration.md -- original capture wire guards

//go:build linux

package kernel

import (
	"errors"
	"os"
	"testing"
	"unsafe"

	"golang.org/x/sys/unix"
)

// Fixed native layouts keep ioctl numbers, field alignment and zero inputs
// independent of the Go architecture. This creates no channel owner.
func TestOriginalPPPChannelSupplierNativeABI(t *testing.T) {
	var owner pppTransportOwner
	var capture pppTransportCapture
	if unsafe.Sizeof(owner) != 20 {
		t.Fatalf("query size=%d", unsafe.Sizeof(owner))
	}
	if unsafe.Alignof(owner) != 4 {
		t.Fatalf("query align=%d", unsafe.Alignof(owner))
	}
	if unsafe.Offsetof(owner.channel) != 8 {
		t.Fatal("query channel offset changed")
	}
	if unsafe.Offsetof(owner.unit) != 12 {
		t.Fatal("query unit offset changed")
	}
	if unsafe.Sizeof(capture) != 16 {
		t.Fatalf("capture size=%d", unsafe.Sizeof(capture))
	}
	if unsafe.Alignof(capture) != 4 {
		t.Fatalf("capture align=%d", unsafe.Alignof(capture))
	}
	if unsafe.Offsetof(capture.fd) != 4 {
		t.Fatal("capture fd offset changed")
	}
	if pppiocGTransportOwner != 0xc014742c {
		t.Fatal("wrong original query ioctl")
	}
	if pppiocAttChanOwner != 0x4010742d {
		t.Fatal("wrong original capture ioctl")
	}
	if pppTransportOwnerFeatures != 448 {
		t.Fatal("capture/connect/bridge feature mask changed")
	}
}

// Different channel labels have no capture authority; capture receives only
// the exact original socket and held file descriptor. Target fd0 is valid.
func TestOriginalPPPChannelSupplierCapturesFileZeroWithoutNumericLookup(t *testing.T) {
	for _, label := range []int32{0, 7, 2147483647} {
		queries, captures := 0, 0
		err := captureOriginalPPPChannel(73, 0,
			func(fd int, request *pppTransportOwner) error {
				queries++
				if fd != 73 {
					t.Fatal("query changed original socket")
				}
				if *request != (pppTransportOwner{abi: 1}) {
					t.Fatalf("query input=%+v", request)
				}
				*request = pppTransportOwner{abi: 1, features: 448, channel: label, unit: -1}
				return nil
			},
			func(fd int, request *pppTransportCapture) error {
				captures++
				if fd != 73 {
					t.Fatal("capture changed original socket")
				}
				if *request != (pppTransportCapture{abi: 1, fd: 0}) {
					t.Fatalf("capture input=%+v", request)
				}
				return nil
			})
		if err != nil {
			t.Fatal(err)
		}
		if queries != 1 {
			t.Fatalf("queries=%d, want1", queries)
		}
		if captures != 1 {
			t.Fatalf("captures=%d, want1", captures)
		}
	}
}

// Unknown permission/lifetime/contention and unsupported ioctl replies retain
// the original cause. Every one refuses before capture, with no fallback.
func TestOriginalPPPChannelSupplierQueryRefusalRetainsCause(t *testing.T) {
	for _, cause := range []error{unix.ENOTTY, unix.EOPNOTSUPP, unix.EPERM, unix.EACCES, unix.EBUSY, unix.ENOTCONN} {
		captures := 0
		err := captureOriginalPPPChannel(73, 9,
			func(int, *pppTransportOwner) error { return cause },
			func(int, *pppTransportCapture) error { captures++; return nil })
		if !errors.Is(err, cause) {
			t.Fatalf("query lost %v: %v", cause, err)
		}
		if captures != 0 {
			t.Fatalf("refused query captured %d times", captures)
		}
	}
}

// A malformed or incomplete positive query cannot turn its diagnostic labels
// into authority. A complete positive query is the opposite control above.
func TestOriginalPPPChannelSupplierMalformedReplyRefuses(t *testing.T) {
	for _, owner := range []pppTransportOwner{
		{abi: 2, features: 448}, {abi: 1, features: 448, reserved: 1},
		{abi: 1, features: 192}, {abi: 1, features: 256},
		{abi: 1, features: 448, channel: -1}, {abi: 1, features: 448, unit: -2},
	} {
		captures := 0
		err := captureOriginalPPPChannel(73, 9,
			func(_ int, request *pppTransportOwner) error { *request = owner; return nil },
			func(int, *pppTransportCapture) error { captures++; return nil })
		if err == nil {
			t.Fatalf("malformed reply admitted: %+v", owner)
		}
		if captures != 0 {
			t.Fatalf("malformed reply captured %d times", captures)
		}
	}
}

// Once capture was attempted, even EINTR is ambiguous: retain its exact cause
// without retrying, querying a replacement or falling back to numeric attach.
func TestOriginalPPPChannelSupplierCaptureRefusalNeverRetries(t *testing.T) {
	for _, cause := range []error{unix.EINTR, unix.ENOTTY, unix.EBUSY, unix.EPERM, unix.ENOTCONN} {
		queries, captures := 0, 0
		err := captureOriginalPPPChannel(73, 9,
			func(_ int, request *pppTransportOwner) error {
				queries++
				*request = pppTransportOwner{abi: 1, features: 448, channel: 14, unit: -1}
				return nil
			},
			func(int, *pppTransportCapture) error { captures++; return cause })
		if !errors.Is(err, cause) {
			t.Fatalf("capture lost %v: %v", cause, err)
		}
		if queries != 1 {
			t.Fatalf("capture refusal queried %d times", queries)
		}
		if captures != 1 {
			t.Fatalf("capture refusal retried %d times", captures)
		}
	}
}

// A nil or closed original os.File refuses before opening /dev/ppp. A new file
// with the same descriptor label is never manufactured by this public route.
func TestOriginalPPPChannelSupplierRequiresLiveOriginalFile(t *testing.T) {
	channel, err := CreatePPPChannel(nil)
	if err == nil {
		t.Fatal("nil source admitted")
	}
	if channel != nil {
		t.Fatal("nil source allocated an owner")
	}
	file, err := os.CreateTemp(t.TempDir(), "closed-original")
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	channel, err = CreatePPPChannel(file)
	if !errors.Is(err, os.ErrClosed) {
		t.Fatalf("closed source cause=%v", err)
	}
	if channel != nil {
		t.Fatal("closed source allocated an owner")
	}
}
