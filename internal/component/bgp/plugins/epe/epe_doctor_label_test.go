package epe

import (
	"errors"
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/linkstateevents"
	"github.com/ze-software/ze/internal/core/mplsfib"
	"github.com/ze-software/ze/internal/core/replay"
)

// refusingEPEWriter acknowledges every batch addressed to the token it holds
// with refusal, which a test sets and clears.
type refusingEPEWriter struct {
	held    uint64
	refusal error
}

func (w *refusingEPEWriter) handle(batch *mplsfib.EntryBatch) {
	if !batch.Addresses(w.held) {
		return
	}
	batch.Acknowledge(w.refusal)
}

// newRefusalSource builds a Source on the production bus with one refusing
// writer and one configured, established peer, as publishLocked finds it
// after the native FIB owner was reclaimed.
func newRefusalSource(t *testing.T, writer *refusingEPEWriter) (*Source, netip.Addr) {
	t.Helper()
	bus := newEPELabelBus(t)
	t.Cleanup(mplsfib.EntryChange.Subscribe(bus, writer.handle))
	source := NewSource(bus)
	peer := netip.MustParseAddr("192.0.2.1")
	source.config = Config{Base: 16000, Peers: map[netip.Addr]PeerConfig{peer: {Index: 1}}}
	source.sessions[peer] = epeSession{
		local:        linkstateevents.NodeID{ASN: 65000, BGPRouterID: netip.MustParseAddr("192.0.2.254")},
		remote:       linkstateevents.NodeID{ASN: 65001, BGPRouterID: peer},
		localAddress: netip.MustParseAddr("192.0.2.254"),
	}
	source.acquired = true
	return source, peer
}

func publish(source *Source) error {
	source.mu.Lock()
	defer source.mu.Unlock()
	return source.publishLocked()
}

// TestEPELabelDoctorReportsRefusedLabels proves the bgp-epe-label-entries
// check reports a PeerNode SID label the writer refused to install and one it
// refused to remove, each with the writer's cause under
// doctor-fib-mpls-no-writer and a fix, and clears each once the data plane
// holds what EPE wants.
//
// VALIDATES: AC-22 -- an EPE label no writer installs, or one still held
// after EPE withdrew it, reaches `show doctor` with its cause.
// METHOD: a writer on the production bus refuses, then accepts, the add of
// peer 192.0.2.1's label 16001; the session then goes down while the writer
// refuses the removal, then accepts it.
func TestEPELabelDoctorReportsRefusedLabels(t *testing.T) {
	writer := &refusingEPEWriter{refusal: errors.New("netlink: in-label 16001 owned by source 2")}
	source, peer := newRefusalSource(t, writer)

	if got := source.labelDoctorFindings(); len(got) != 0 {
		t.Fatalf("nothing refused yet, got %v", got)
	}
	if err := publish(source); err == nil {
		t.Fatal("a refused label is an error of the publish")
	}
	findings := source.labelDoctorFindings()
	if len(findings) != 1 {
		t.Fatalf("want 1 finding, got %v", findings)
	}
	if findings[0].Code != mplsfib.CodeDoctorNoWriter || findings[0].Severity != "error" || findings[0].Fix == "" {
		t.Fatalf("finding shape: %+v", findings[0])
	}
	for _, want := range []string{"bgp-epe", "does not hold", "pop of in-label 16001", "toward 192.0.2.1", "owned by source 2"} {
		if !strings.Contains(findings[0].Message, want) {
			t.Errorf("want %q in %q", want, findings[0].Message)
		}
	}

	writer.refusal = nil
	if err := publish(source); err != nil {
		t.Fatalf("accepted publish: %v", err)
	}
	if got := source.labelDoctorFindings(); len(got) != 0 {
		t.Fatalf("an installed label clears the refusal, got %v", got)
	}

	delete(source.sessions, peer)
	writer.refusal = errors.New("netlink: no such device")
	if err := publish(source); err == nil {
		t.Fatal("a refused removal is an error of the publish")
	}
	findings = source.labelDoctorFindings()
	if len(findings) != 1 || !strings.Contains(findings[0].Message, "may still hold") ||
		!strings.Contains(findings[0].Message, "no such device") {
		t.Fatalf("want the label EPE withdrew and the writer kept, got %v", findings)
	}

	writer.refusal = nil
	if err := publish(source); err != nil {
		t.Fatalf("accepted removal: %v", err)
	}
	if got := source.labelDoctorFindings(); len(got) != 0 {
		t.Fatalf("a completed removal clears the refusal, got %v", got)
	}
}

// TestEPELabelDoctorForgetsUnwantedRefusal proves a refused label stops being
// a finding once EPE no longer wants it: the peer's session went down before
// any writer installed the label, so the data plane already holds what EPE
// wants.
//
// VALIDATES: the check reports only labels the data plane disagrees with EPE
// about, never a stale refusal.
// METHOD: refuse the add, take the session down, publish again.
func TestEPELabelDoctorForgetsUnwantedRefusal(t *testing.T) {
	writer := &refusingEPEWriter{refusal: errors.New("vpp: mpls table 0 missing")}
	source, peer := newRefusalSource(t, writer)
	if err := publish(source); err == nil {
		t.Fatal("a refused label is an error of the publish")
	}
	delete(source.sessions, peer)
	if err := publish(source); err != nil {
		t.Fatalf("nothing to program: %v", err)
	}
	if got := source.labelDoctorFindings(); len(got) != 0 {
		t.Fatalf("a label EPE no longer wants is no finding, got %v", got)
	}
}

// TestEPELabelDoctorReportsUnreplayedLabels proves a label a writer's replay
// did not install is reported, and a later successful replay clears it.
//
// VALIDATES: AC-22, AC-25 -- PeerNode SID labels a VPP writer asked for again
// and did not install reach `show doctor` with the writer's cause.
// METHOD: the writer holds a token, accepts the first install, refuses the
// replay batch it asked for, then accepts the next replay.
func TestEPELabelDoctorReportsUnreplayedLabels(t *testing.T) {
	writer := &refusingEPEWriter{held: mplsfib.NewReplayToken()}
	source, _ := newRefusalSource(t, writer)
	if err := publish(source); err != nil {
		t.Fatalf("accepted publish: %v", err)
	}

	writer.refusal = errors.New("vpp: mpls table 0 missing")
	source.ReplayLabels(&replay.Request{ReplayID: writer.held})
	findings := source.labelDoctorFindings()
	if len(findings) != 1 || !strings.Contains(findings[0].Message, "pop of in-label 16001") ||
		!strings.Contains(findings[0].Message, "table 0 missing") {
		t.Fatalf("want the unreplayed label with its cause, got %v", findings)
	}

	writer.refusal = nil
	source.ReplayLabels(&replay.Request{ReplayID: writer.held})
	if got := source.labelDoctorFindings(); len(got) != 0 {
		t.Fatalf("a successful replay clears the finding, got %v", got)
	}
}
