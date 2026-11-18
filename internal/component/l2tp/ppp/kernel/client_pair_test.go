// Design: docs/research/l2tpv2-ze-integration.md -- delayed client ownership controls.
// Related: client.go -- private logical operations never grant native authority.

package kernel

import (
	"errors"
	"io"
	"reflect"
	"testing"
)

// The logical constructor consumes the same sealed claims as the server. Its
// injected operations observe phase and cleanup without wrapping fake fds.
func clientPairFixture(t *testing.T) (*ClientSession, *claimedSessionPair, *[]int) {
	t.Helper()
	cfg := sessionTestConfig()
	released := new([]int)
	unit := sessionTestUnit(cfg)
	unit.closeFile = func(fd int) error { *released = append(*released, fd); return nil }
	ops := clientPairOps{
		prepare: func(*claimedSessionPair) error { return nil },
		connect: func(*claimedSessionPair) error { return nil },
		read: func(_ *claimedSessionPair, unit bool, b []byte) (int, error) {
			if unit {
				b[0] = 2
			} else {
				b[0] = 1
			}
			return 1, nil
		},
		write: func(_ *claimedSessionPair, b []byte) (int, error) { return len(b), nil },
		unit:  func(*claimedSessionPair, clientUnitRequest) error { return nil },
	}
	closers := sessionPairClosers{channel: func(lease sessionChannelLease, _ io.ReadWriteCloser) error {
		*released = append(*released, lease.descriptor())
		return nil
	}}
	client, err := newClientSession(cfg, ops, closers)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Error(err)
		}
	})
	return client, client.pair, released
}

// Constructor, failed Connect and failed MRU cannot admit unit packets. The
// original client opens unit input only after both attachment and MRU succeed.
func TestClientPairDelayedUnitAdmission(t *testing.T) {
	client, pair, _ := clientPairFixture(t)
	buffer := make([]byte, 8)
	if _, err := client.ReadPacket(buffer); err != nil {
		t.Fatal(err)
	}
	if buffer[0] != 1 {
		t.Fatal("constructor admitted unit input")
	}
	want := errors.New("original attachment refused")
	client.ops.connect = func(*claimedSessionPair) error { return want }
	if err := client.Connect(); !errors.Is(err, want) {
		t.Fatal(err)
	}
	if _, err := client.ReadPacket(buffer); err != nil {
		t.Fatal(err)
	}
	if buffer[0] != 1 {
		t.Fatal("failed attachment admitted unit input")
	}
	client.ops.connect = func(got *claimedSessionPair) error {
		if got != pair {
			t.Fatal("attachment lost original pair")
		}
		return nil
	}
	if err := client.Connect(); err != nil {
		t.Fatal(err)
	}
	if _, err := client.ReadPacket(buffer); err != nil {
		t.Fatal(err)
	}
	if buffer[0] != 1 {
		t.Fatal("attachment admitted unit input before MRU")
	}
	client.ops.unit = func(*claimedSessionPair, clientUnitRequest) error { return want }
	if err := client.SetReceiveMRU(1500); !errors.Is(err, want) {
		t.Fatal(err)
	}
	if _, err := client.ReadPacket(buffer); err != nil {
		t.Fatal(err)
	}
	if buffer[0] != 1 {
		t.Fatal("failed MRU admitted unit input")
	}
	client.ops.unit = func(*claimedSessionPair, clientUnitRequest) error { return nil }
	if err := client.SetReceiveMRU(1500); err != nil {
		t.Fatal(err)
	}
	if _, err := client.ReadPacket(buffer); err != nil {
		t.Fatal(err)
	}
	if buffer[0] != 2 {
		t.Fatal("successful attachment and MRU did not admit unit input")
	}
}

// Split shutdown keeps the unit until the owner's final cleanup; stopped
// guards win even over invalid mode/MRU input and no operation runs afterward.
func TestClientPairSplitCloseRetainsUnit(t *testing.T) {
	client, _, released := clientPairFixture(t)
	for range 2 {
		if err := client.CloseChannel(); err != nil {
			t.Fatal(err)
		}
	}
	if !reflect.DeepEqual(*released, []int{31}) {
		t.Fatalf("channel close released %v", *released)
	}
	if _, err := client.Write([]byte{1}); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
	if err := client.SetReceiveMRU(0); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
	if err := client.SetIPv4Mode(NPModeUnspecified); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
	if err := client.SetIPv6Mode(NPModeUnspecified); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatal(err)
	}
	for range 2 {
		if err := client.CloseUnit(); err != nil {
			t.Fatal(err)
		}
	}
	if !reflect.DeepEqual(*released, []int{31, 32}) {
		t.Fatalf("final close released %v", *released)
	}
}

// An earlier CloseUnit is refused and retains custody until channel stop.
func TestClientPairCloseUnitRequiresChannelStop(t *testing.T) {
	client, _, released := clientPairFixture(t)
	if err := client.CloseUnit(); err == nil {
		t.Fatal("unit released before channel stop")
	}
	if len(*released) != 0 {
		t.Fatal("early refusal changed custody")
	}
}

// Copying a public client cannot create a second operational lifecycle mutex.
func TestClientPairCopiedClientRefusesOperations(t *testing.T) {
	client, _, released := clientPairFixture(t)
	copied := *client
	if err := copied.Connect(); err == nil {
		t.Fatal("copied client operated")
	}
	if err := copied.Close(); err == nil {
		t.Fatal("copied client released original")
	}
	if len(*released) != 0 {
		t.Fatal("copied client changed custody")
	}
}

// Every newly claimed sibling transfers even when the other source refuses;
// cleanup panics become retained causes and cannot bypass the second close.
func TestClientPairConstructorRollbackPreservesAllCauses(t *testing.T) {
	cfg := sessionTestConfig()
	primary := errors.New("preparation refused")
	channelCause := errors.New("channel close panic")
	unitCause := errors.New("unit close failure")
	var order []int
	sessionTestUnit(cfg).closeFile = func(fd int) error { order = append(order, fd); return unitCause }
	ops := clientPairOps{prepare: func(*claimedSessionPair) error { return primary }}
	closers := sessionPairClosers{channel: func(lease sessionChannelLease, _ io.ReadWriteCloser) error {
		order = append(order, lease.descriptor())
		panic(channelCause)
	}}
	client, err := newClientSession(cfg, ops, closers)
	if client != nil {
		t.Fatal("failed constructor published owner")
	}
	for _, cause := range []error{primary, channelCause, unitCause} {
		if !errors.Is(err, cause) {
			t.Fatalf("lost original cause %v: %v", cause, err)
		}
	}
	if !reflect.DeepEqual(order, []int{31, 32}) {
		t.Fatalf("partial cleanup order %v", order)
	}
}

type clientClaimChannel struct {
	source  sessionChannelSource
	order   *[]string
	err     error
	failure any
}

func (source *clientClaimChannel) claimSessionChannel() (sessionChannelLease, error) {
	*source.order = append(*source.order, "channel")
	if source.failure != nil {
		panic(source.failure)
	}
	if source.err != nil {
		return nil, source.err
	}
	return source.source.claimSessionChannel()
}

type clientClaimUnit struct {
	source  sessionUnitSource
	order   *[]string
	err     error
	failure any
}

func (source *clientClaimUnit) claimSession() (sessionUnitLease, error) {
	*source.order = append(*source.order, "unit")
	if source.failure != nil {
		panic(source.failure)
	}
	if source.err != nil {
		return nil, source.err
	}
	return source.source.claimSession()
}

// Both claims must run independently even when the first errors or panics.
// The fresh sibling is consumed and closed once with the original cause kept.
func TestClientPairIndependentClaimsAfterRefusal(t *testing.T) {
	for _, lane := range []string{"channel", "unit"} {
		for _, panics := range []bool{false, true} {
			name := lane + "-error"
			if panics {
				name = lane + "-panic"
			}
			t.Run(name, func(t *testing.T) {
				cfg := sessionTestConfig()
				primary := errors.New("original claim refused")
				var claims []string
				var released []int
				sessionTestUnit(cfg).closeFile = func(fd int) error { released = append(released, fd); return nil }
				channel := &clientClaimChannel{source: cfg.Channel, order: &claims}
				unit := &clientClaimUnit{source: cfg.Unit, order: &claims}
				if lane == "channel" {
					if panics {
						channel.failure = primary
					} else {
						channel.err = primary
					}
				} else {
					if panics {
						unit.failure = primary
					} else {
						unit.err = primary
					}
				}
				cfg.Channel, cfg.Unit = channel, unit
				closers := sessionPairClosers{channel: func(lease sessionChannelLease, _ io.ReadWriteCloser) error {
					released = append(released, lease.descriptor())
					return nil
				}}
				var client *ClientSession
				var err error
				var failure any
				func() {
					defer func() { failure = recover() }()
					client, err = newClientSession(cfg, clientPairOps{}, closers)
				}()
				if client != nil {
					t.Fatal("claim refusal published owner")
				}
				if panics {
					if failure != primary {
						t.Fatalf("original panic changed: %v", failure)
					}
				} else {
					if !errors.Is(err, primary) {
						t.Fatalf("original refusal lost: %v", err)
					}
				}
				if !reflect.DeepEqual(claims, []string{"channel", "unit"}) {
					t.Fatalf("claim order %v", claims)
				}
				wantFD := 31
				if lane == "channel" {
					wantFD = 32
				}
				if !reflect.DeepEqual(released, []int{wantFD}) {
					t.Fatalf("fresh sibling cleanup %v", released)
				}
			})
		}
	}
}

// The pair preserves the original writer's short result and error unchanged.
func TestClientPairWritePreservesShortResult(t *testing.T) {
	client, _, _ := clientPairFixture(t)
	want := errors.New("original write refused")
	client.ops.write = func(*claimedSessionPair, []byte) (int, error) { return 1, want }
	n, err := client.WritePacket([]byte{1, 2, 3})
	if n != 1 {
		t.Fatalf("write result %d %v", n, err)
	}
	if !errors.Is(err, want) {
		t.Fatalf("write result %d %v", n, err)
	}
}

// REPLACE ONLY TestClientPairConfigurationRefusalConsumesPair in client_pair_test.go.
// Keep every original subcase and its exact once/order cleanup assertion.
// Independent test judgment is required before Main applies this replacement.
func TestClientPairConfigurationRefusalConsumesPair(t *testing.T) {
	for _, test := range []struct{ name, cause string }{
		{"backend", "ppp: session backend differs from original unit creator"},
		{"channel-label", "ppp: channel label differs from captured original"},
		{"unit-label", "ppp: supplied unit labels disagree with original creation"},
		{"typed-nil", "ppp: no captured channel owner"},
	} {
		t.Run(test.name, func(t *testing.T) {
			cfg, ops, closers, released, reached := clientConfigurationInputs(t)
			switch test.name {
			case "backend":
				cfg.Backend = &sessionTestBackend{}
			case "channel-label":
				cfg.ChanFD = 33 // distinct from BOTH real labels31/32.
			case "unit-label":
				cfg.UnitNum++
			case "typed-nil":
				cfg.Channel = (*sessionTestChannelSource)(nil)
			}
			client, err := newClientSession(cfg, ops, closers)
			if err == nil {
				t.Fatal("invalid pair admitted")
			}
			if client != nil {
				t.Fatal("invalid pair published owner")
			}
			if err.Error() != test.cause {
				t.Fatalf("wrong refusal %q, want %q", err, test.cause)
			}
			if len(*reached) != 0 {
				t.Fatalf("invalid pair reached preparation/operations: %v", *reached)
			}
			want := []int{31, 32}
			if test.name == "typed-nil" {
				want = []int{32}
			}
			if !reflect.DeepEqual(*released, want) {
				t.Fatalf("refused pair cleanup %v", *released)
			}
		})
	}
}
