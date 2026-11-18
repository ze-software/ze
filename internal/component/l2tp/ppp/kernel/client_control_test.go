// Design: docs/research/l2tpv2-ze-integration.md -- delayed client controls.
// Related: client.go -- actual client state transitions, with private logical I/O.

package kernel

import (
	"errors"
	"io"
	"reflect"
	"testing"
)

// Fresh unconsumed logical inputs have a complete valid operation set. A
// missing-operation refusal must never mask the validation question under test.
func clientConfigurationInputs(t *testing.T) (SessionConfig, clientPairOps,
	sessionPairClosers, *[]int, *[]string) {
	t.Helper()
	cfg := sessionTestConfig()
	released := new([]int)
	reached := new([]string)
	sessionTestUnit(cfg).closeFile = func(fd int) error { *released = append(*released, fd); return nil }
	ops := clientPairOps{
		prepare: func(*claimedSessionPair) error { *reached = append(*reached, "prepare"); return nil },
		connect: func(*claimedSessionPair) error { *reached = append(*reached, "connect"); return nil },
		read: func(_ *claimedSessionPair, _ bool, buffer []byte) (int, error) {
			*reached = append(*reached, "read")
			buffer[0] = 0x80
			return 1, nil
		},
		write: func(_ *claimedSessionPair, buffer []byte) (int, error) {
			*reached = append(*reached, "write")
			return len(buffer), nil
		},
		unit: func(*claimedSessionPair, clientUnitRequest) error {
			*reached = append(*reached, "unit")
			return nil
		},
	}
	closers := sessionPairClosers{channel: func(lease sessionChannelLease, _ io.ReadWriteCloser) error {
		*released = append(*released, lease.descriptor())
		return nil
	}}
	return cfg, ops, closers, released, reached
}

// The valid opposite proves the malformed-input tests start from a fully
// usable fresh operation set rather than an already consumed owner.
func TestClientPairValidConfigurationReachesPreparation(t *testing.T) {
	cfg, ops, closers, released, reached := clientConfigurationInputs(t)
	client, err := newClientSession(cfg, ops, closers)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Error(err)
		}
	})
	if !reflect.DeepEqual(*reached, []string{"prepare"}) {
		t.Fatalf("constructor operations %v", *reached)
	}
	if len(*released) != 0 {
		t.Fatalf("valid constructor released %v", *released)
	}
	if err := client.Connect(); err != nil {
		t.Fatal(err)
	}
	if err := client.SetReceiveMRU(1500); err != nil {
		t.Fatal(err)
	}
	if _, err := client.ReadPacket(make([]byte, 8)); err != nil {
		t.Fatal(err)
	}
	if _, err := client.WritePacket([]byte{0xc0, 0x21}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(*reached, []string{"prepare", "connect", "unit", "read", "write"}) {
		t.Fatalf("valid pair did not reach its operations: %v", *reached)
	}
}

// After channel stop, ReadPacket and Connect must refuse before touching any
// dependency. Retained unit custody is independent of operation admission.
func TestClientPairStoppedReadAndConnectReachNoOperation(t *testing.T) {
	client, _, released := clientPairFixture(t)
	var reads, connects int
	client.ops.read = func(*claimedSessionPair, bool, []byte) (int, error) { reads++; return 0, nil }
	client.ops.connect = func(*claimedSessionPair) error { connects++; return nil }
	if err := client.CloseChannel(); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if n, err := client.ReadPacket(make([]byte, 8)); !errors.Is(err, io.ErrClosedPipe) {
			t.Fatalf("stopped read %d %v", n, err)
		}
		if err := client.Connect(); !errors.Is(err, io.ErrClosedPipe) {
			t.Fatalf("stopped connect %v", err)
		}
	}
	if reads != 0 {
		t.Fatalf("stopped reader reached dependency %d times", reads)
	}
	if connects != 0 {
		t.Fatalf("stopped connector reached dependency %d times", connects)
	}
	if !reflect.DeepEqual(*released, []int{31}) {
		t.Fatalf("stopped operations released unit: %v", *released)
	}
}

// A pre-attachment MRU and independent family decisions cannot grant input.
// Only a successful receive-MRU after the actual attachment transition can do so.
func TestClientPairMRUFailureKeepsAttachedUnitUnreadable(t *testing.T) {
	client, _, _ := clientPairFixture(t)
	if err := client.SetReceiveMRU(1500); err != nil {
		t.Fatal(err)
	}
	if err := client.Connect(); err != nil {
		t.Fatal(err)
	}
	want := errors.New("native MRU refused after attachment")
	client.ops.unit = func(_ *claimedSessionPair, request clientUnitRequest) error {
		if _, mru := request.(clientMRU); mru {
			return want
		}
		return nil
	}
	if err := client.SetReceiveMRU(1500); !errors.Is(err, want) {
		t.Fatal(err)
	}
	if err := client.SetIPv4Mode(NPModeDrop); err != nil {
		t.Fatal(err)
	}
	if err := client.SetIPv6Mode(NPModeDrop); err != nil {
		t.Fatal(err)
	}
	buffer := make([]byte, 8)
	if _, err := client.ReadPacket(buffer); err != nil {
		t.Fatal(err)
	}
	if buffer[0] != 1 {
		t.Fatal("failed post-attachment MRU exposed unit input")
	}
	client.ops.unit = func(*claimedSessionPair, clientUnitRequest) error { return nil }
	if err := client.SetReceiveMRU(1500); err != nil {
		t.Fatal(err)
	}
	if _, err := client.ReadPacket(buffer); err != nil {
		t.Fatal(err)
	}
	if buffer[0] != 2 {
		t.Fatal("successful post-attachment MRU did not expose useful unit input")
	}
}

// Logical descriptor zero remains an owned original label in either lane;
// valid operation and constructor refusal each close both once, in order.
// These are private logical owners and never wrap a process descriptor.
func TestClientPairDescriptorZeroCustody(t *testing.T) {
	for _, lane := range []string{"channel", "unit"} {
		for _, refuse := range []bool{false, true} {
			name := lane + "-valid"
			if refuse {
				name = lane + "-refused"
			}
			t.Run(name, func(t *testing.T) {
				cfg, ops, closers, released, reached := clientConfigurationInputs(t)
				if lane == "channel" {
					cfg.ChanFD = 0
					cfg.Channel = &sessionTestChannelSource{fd: 0}
				} else {
					cfg.UnitFD = 0
					sessionTestUnit(cfg).info.fd = 0
				}
				ops.connect = func(pair *claimedSessionPair) error {
					if pair.channel.descriptor() != cfg.ChanFD {
						t.Fatal("channel zero ownership changed")
					}
					if pair.unit.descriptor() != cfg.UnitFD {
						t.Fatal("unit zero ownership changed")
					}
					return nil
				}
				if refuse {
					cfg.UnitNum++
				}
				client, err := newClientSession(cfg, ops, closers)
				if refuse {
					if err == nil {
						t.Fatal("invalid fd0 pair admitted")
					}
					if client != nil {
						t.Fatal("invalid fd0 pair published")
					}
					if err.Error() != "ppp: supplied unit labels disagree with original creation" {
						t.Fatal(err)
					}
					if len(*reached) != 0 {
						t.Fatalf("invalid fd0 pair reached operations: %v", *reached)
					}
				} else {
					if err != nil {
						t.Fatal(err)
					}
					t.Cleanup(func() {
						if err := client.Close(); err != nil {
							t.Error(err)
						}
					})
					if err := client.Connect(); err != nil {
						t.Fatal(err)
					}
					for range 2 {
						if err := client.Close(); err != nil {
							t.Fatal(err)
						}
					}
				}
				if !reflect.DeepEqual(*released, []int{cfg.ChanFD, cfg.UnitFD}) {
					t.Fatalf("fd0 custody %v", *released)
				}
			})
		}
	}
}

type clientPartialChannel struct {
	source sessionChannelSource
	cause  error
}

func (source *clientPartialChannel) claimSessionChannel() (sessionChannelLease, error) {
	lease, err := source.source.claimSessionChannel()
	return lease, errors.Join(err, source.cause)
}

type clientPartialUnit struct {
	source sessionUnitSource
	cause  error
}

func (source *clientPartialUnit) claimSession() (sessionUnitLease, error) {
	lease, err := source.source.claimSession()
	return lease, errors.Join(err, source.cause)
}

// A claim can return both a real owned lease and an error. Admission must
// release that partial owner AND its fresh sibling, retaining the first cause.
func TestClientPairPartialClaimClosesBothOwners(t *testing.T) {
	for _, lane := range []string{"channel", "unit"} {
		t.Run(lane, func(t *testing.T) {
			cfg, ops, closers, released, reached := clientConfigurationInputs(t)
			want := errors.New("claim returned its original partial lease and refused")
			if lane == "channel" {
				cfg.Channel = &clientPartialChannel{source: cfg.Channel, cause: want}
			}
			if lane == "unit" {
				cfg.Unit = &clientPartialUnit{source: cfg.Unit, cause: want}
			}
			client, err := newClientSession(cfg, ops, closers)
			if client != nil {
				t.Fatal("partial refusal published client")
			}
			if !errors.Is(err, want) {
				t.Fatalf("partial cause lost: %v", err)
			}
			if len(*reached) != 0 {
				t.Fatalf("partial refusal reached operations: %v", *reached)
			}
			if !reflect.DeepEqual(*released, []int{31, 32}) {
				t.Fatalf("partial ownership cleanup %v", *released)
			}
		})
	}
}
