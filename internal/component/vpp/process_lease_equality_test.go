// Design: docs/research/vpp-deployment-reference.md -- physical cleanup equality
// Related: process_lease_equality.go -- comparisons carry no logical epoch authority
package vpp

import (
	"errors"
	"io"
	"testing"
	"testing/synctest"
)

type equalityDescriptor struct {
	processDescriptor
	observe func()
}

func (d *equalityDescriptor) Exited() (bool, error) {
	if d.observe != nil {
		d.observe()
	}
	return d.processDescriptor.Exited()
}

func equalityLease(t *testing.T, descriptor processDescriptor, connector, generation uint64) *ProcessLease {
	t.Helper()
	witness, err := newProcessWitness(descriptor)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := witness.release(); err != nil {
			t.Error(err)
		}
	})
	if err := witness.bind(connector, generation, make(chan struct{}, 1)); err != nil {
		t.Fatal(err)
	}
	lease, err := witness.retain()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := lease.Close(); err != nil {
			t.Error(err)
		}
	})
	return lease
}

// Distinct transport epochs can name the same physical process. Matching
// connector/generation cannot compensate for different pidfs device or inode.
func TestVPPPhysicalLeaseEqualityUsesKernelIdentity(t *testing.T) {
	for _, test := range []struct {
		name  string
		right processIdentity
		same  bool
	}{
		{"same physical peer", processIdentity{device: 3, inode: 701}, true},
		{"different inode", processIdentity{device: 3, inode: 702}, false},
		{"different pidfs device", processIdentity{device: 4, inode: 701}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			left := equalityLease(t, &peerTestDescriptor{identity: processIdentity{device: 3, inode: 701}}, 11, 7)
			right := equalityLease(t, &peerTestDescriptor{identity: test.right}, 22, 8)
			same, err := left.SameProcess(right)
			if err != nil {
				t.Fatal(err)
			}
			if same != test.same {
				t.Fatalf("physical equality=%v, want %v", same, test.same)
			}
			reverse, err := right.SameProcess(left)
			if err != nil {
				t.Fatal(err)
			}
			if reverse != test.same {
				t.Fatal("opposite-direction comparison changed equality")
			}
		})
	}
}

// Invalid, unobservable, exited and closed leases are unknown/refused, never
// a successful equality or a known different live process.
func TestVPPPhysicalLeaseEqualityRefusesUnusable(t *testing.T) {
	for _, kind := range []string{"nil first", "nil second", "zero first", "zero second", "closed first", "closed second", "dead first", "dead second", "unknown first", "unknown second"} {
		t.Run(kind, func(t *testing.T) {
			leftDescriptor := &peerTestDescriptor{identity: processIdentity{device: 3, inode: 711}}
			rightDescriptor := &peerTestDescriptor{identity: processIdentity{device: 3, inode: 711}}
			left := equalityLease(t, leftDescriptor, 11, 7)
			right := equalityLease(t, rightDescriptor, 22, 8)
			switch kind {
			case "nil first":
				left = nil
			case "nil second":
				right = nil
			case "zero first":
				left = &ProcessLease{}
			case "zero second":
				right = &ProcessLease{}
			case "closed first":
				if err := left.Close(); err != nil {
					t.Fatal(err)
				}
			case "closed second":
				if err := right.Close(); err != nil {
					t.Fatal(err)
				}
			case "dead first":
				leftDescriptor.mu.Lock()
				leftDescriptor.dead = true
				leftDescriptor.mu.Unlock()
			case "dead second":
				rightDescriptor.mu.Lock()
				rightDescriptor.dead = true
				rightDescriptor.mu.Unlock()
			case "unknown first":
				leftDescriptor.mu.Lock()
				leftDescriptor.failure = errors.New("pidfd poll refused")
				leftDescriptor.mu.Unlock()
			case "unknown second":
				rightDescriptor.mu.Lock()
				rightDescriptor.failure = errors.New("pidfd poll refused")
				rightDescriptor.mu.Unlock()
			}
			same, err := left.SameProcess(right)
			if err == nil {
				t.Fatal("unusable process lease returned a known comparison")
			}
			if same {
				t.Fatal("unusable process lease authorized physical equality")
			}
		})
	}
}

// A real Close can revoke either lease while another owner keeps its physical
// witness alive. Virtual time prevents the unrelated observation ticker from
// running the scripted Close before comparison reaches the descriptor read.
func TestVPPPhysicalLeaseEqualityRechecksCloseDuringObservation(t *testing.T) {
	for _, first := range []bool{false, true} {
		name := "second lease closes during first observation"
		if first {
			name = "first lease closes during second observation"
		}
		t.Run(name, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				leftDescriptor := &equalityDescriptor{processDescriptor: &peerTestDescriptor{identity: processIdentity{device: 3, inode: 721}}}
				rightDescriptor := &equalityDescriptor{processDescriptor: &peerTestDescriptor{identity: processIdentity{device: 3, inode: 721}}}
				left := equalityLease(t, leftDescriptor, 11, 7)
				right := equalityLease(t, rightDescriptor, 22, 8)
				closeOther := func() {
					if err := right.Close(); err != nil {
						t.Error(err)
					}
				}
				leftDescriptor.observe = closeOther
				if first {
					leftDescriptor.observe = nil
					rightDescriptor.observe = func() {
						if err := left.Close(); err != nil {
							t.Error(err)
						}
					}
				}
				same, err := left.SameProcess(right)
				if !errors.Is(err, io.ErrClosedPipe) {
					t.Fatal("concurrent lease Close was not retained", err)
				}
				if same {
					t.Fatal("a closed lease authorized cleanup after its physical poll")
				}
			})
		})
	}
}
