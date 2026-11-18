//go:build linux

// Design: docs/architecture/appliance/kernel-profiles.md -- shared bridge claims.
// Related: session_unit_helpers_test.go -- explicitly logical one-shot owners.

package kernel

import (
	"errors"
	"io"
	"reflect"
	"testing"
)

// This private model delegates one-shot ownership to the SAME existing model
// source, adding only a close observer. It cannot pass native lease inspection
// or construct a public CreatedChannel/positive native capability receipt.
type originalBridgeModelChannel struct {
	source *sessionTestChannelSource
	close  func() error
}

type originalBridgeModelLease struct {
	sessionChannelLease
	close func() error
}

func (source *originalBridgeModelChannel) claimSessionChannel() (sessionChannelLease, error) {
	lease, err := source.source.claimSessionChannel()
	if err != nil {
		return nil, err
	}
	return &originalBridgeModelLease{sessionChannelLease: lease, close: source.close}, nil
}

func (lease *originalBridgeModelLease) closeStream(io.ReadWriteCloser) error { return lease.close() }

// Both independent claims must close once, in order, after a preparation
// refusal. Each close cause must survive with the actual primary failure.
func TestOriginalBridgeConstructorRollbackRetainsBothCauses(t *testing.T) {
	primary := errors.New("bridge preparation refused")
	firstCause := errors.New("first original close failed")
	secondCause := errors.New("second original close failed")
	var closed []int
	first := &originalBridgeModelChannel{source: &sessionTestChannelSource{fd: 31},
		close: func() error { closed = append(closed, 31); return firstCause }}
	second := &originalBridgeModelChannel{source: &sessionTestChannelSource{fd: 33},
		close: func() error { closed = append(closed, 33); return secondCause }}
	bridge, err := newOriginalChannelBridge(first, second, func(*OriginalChannelBridge) error { return primary })
	if bridge != nil {
		t.Fatal("refused constructor published a bridge")
	}
	for _, cause := range []error{primary, firstCause, secondCause} {
		if !errors.Is(err, cause) {
			t.Fatalf("lost original bridge cause %v: %v", cause, err)
		}
	}
	if !reflect.DeepEqual(closed, []int{31, 33}) {
		t.Fatalf("bridge close order/once %v", closed)
	}
}

// A first-lane claim panic cannot prevent the independent fresh second
// original from being claimed and closed before the same panic is reported.
func TestOriginalBridgeClaimPanicConsumesIndependentSibling(t *testing.T) {
	primary := errors.New("first original claim panicked")
	var claims []string
	var closed []int
	first := &clientClaimChannel{order: &claims, failure: primary}
	second := &clientClaimChannel{order: &claims, source: &originalBridgeModelChannel{
		source: &sessionTestChannelSource{fd: 33},
		close:  func() error { closed = append(closed, 33); return nil },
	}}
	defer func() {
		if failure := recover(); failure != primary {
			t.Fatalf("lost original claim panic: %v", failure)
		}
		if !reflect.DeepEqual(claims, []string{"channel", "channel"}) {
			t.Fatalf("independent claims %v", claims)
		}
		if !reflect.DeepEqual(closed, []int{33}) {
			t.Fatalf("fresh sibling close order/once %v", closed)
		}
	}()
	if _, err := newOriginalChannelBridge(first, second, func(*OriginalChannelBridge) error { t.Fatal("prepared after claim panic"); return nil }); err != nil {
		t.Fatalf("claim panic became ordinary error: %v", err)
	}
	t.Fatal("claim panic disappeared")
}
