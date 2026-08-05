// VALIDATES: that checkIPsecHealth reports degraded, and NAMES the peer, when the
// kernel SAD does not hold a Child SA the engine counts as installed (AC-5).
// PREVENTS: the false green this spec exists to remove -- every other arm of the
// health check reads engine belief, so a tunnel the kernel dropped stays healthy.
//
// Design: plan/spec-ipsec-dataplane-inspection.md -- kernel dataplane read surface
// Related: health_drift.go -- driftingPeers, driftDetail

package engine

import (
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/core/health"
)

// useDriftSAD scripts the kernel half of the comparison.
func useDriftSAD(t *testing.T, sas []dataplane.SAInfo, err error) {
	t.Helper()
	old := driftSAD
	driftSAD = func() ([]dataplane.SAInfo, error) { return sas, err }
	t.Cleanup(func() { driftSAD = old })
}

// TestDriftingPeersEmptyWithNoSessions pins the boundary: no peer session means
// nothing can be missing, and the answer is a KNOWN empty set rather than an
// unknown one.
func TestDriftingPeersEmptyWithNoSessions(t *testing.T) {
	useDriftSAD(t, []dataplane.SAInfo{{SPI: 100}}, nil)
	SetActivePeersForTest(nil)

	peers, known := driftingPeers()
	if !known {
		t.Fatal("known = false with a readable SAD")
	}
	if len(peers) != 0 {
		t.Fatalf("peers = %v, want none when no peer session exists", peers)
	}
}

// TestDriftingPeersUnknownWhenSADUnreadable is the fail-closed half: an
// unreadable dataplane must not read as "no drift".
func TestDriftingPeersUnknownWhenSADUnreadable(t *testing.T) {
	useDriftSAD(t, nil, errors.New("operation not permitted"))

	peers, known := driftingPeers()
	if known {
		t.Error("known = true after the SAD read failed")
	}
	if peers != nil {
		t.Errorf("peers = %v, want nil beside known=false", peers)
	}
}

func TestDriftingPeersUnknownWithNoBackend(t *testing.T) {
	// The real driftSAD, with no backend loaded, must report not-known rather
	// than an empty, confident answer.
	if err := dataplane.CloseBackend(); err != nil {
		t.Fatalf("CloseBackend: %v", err)
	}
	if _, known := driftSAD(); known == nil {
		t.Error("driftSAD returned no error with no backend loaded")
	}
}

func TestDriftDetailNamesEveryPeer(t *testing.T) {
	msg := driftDetail([]string{"peer-alpha", "peer-beta"})
	for _, want := range []string{"peer-alpha", "peer-beta", "drift"} {
		if !strings.Contains(msg, want) {
			t.Errorf("driftDetail = %q, missing %q", msg, want)
		}
	}
}

// TestCheckIPsecHealthDegradedOnDrift is AC-5 through the entry point. Driving
// driftingPeers alone would leave checkIPsecHealth's own arm unproven, and that
// arm is the one an operator reads (ai/rules/evidence.md).
func TestCheckIPsecHealthDegradedOnDrift(t *testing.T) {
	table := NewSATable()
	sa := &SA{PeerName: "peer-alpha", State: StateEstablished}
	sa.InitiatorSPI = [8]byte{1, 2, 3, 4, 5, 6, 7, 8}
	table.Insert(sa)
	SetActiveTableForTest(table)
	t.Cleanup(func() { SetActiveTableForTest(nil) })

	ps := &PeerSession{peerName: "peer-alpha"}
	ps.childSA = &ChildSA{InboundSPI: 100, OutboundSPI: 200}
	SetActivePeersForTest(map[string]*PeerSession{"peer-alpha": ps})
	t.Cleanup(func() { SetActivePeersForTest(nil) })

	t.Run("kernel holds both SPIs", func(t *testing.T) {
		useDriftSAD(t, []dataplane.SAInfo{{SPI: 100}, {SPI: 200}}, nil)
		status, detail := checkIPsecHealth()
		if status != health.StatusHealthy {
			t.Errorf("status = %v (%q), want healthy when belief and kernel agree", status, detail)
		}
	})

	t.Run("kernel is missing the outbound SPI", func(t *testing.T) {
		useDriftSAD(t, []dataplane.SAInfo{{SPI: 100}}, nil)
		status, detail := checkIPsecHealth()
		if status == health.StatusHealthy {
			t.Error("status = healthy while the kernel does not hold the outbound child SA")
		}
		if !strings.Contains(detail, "peer-alpha") {
			t.Errorf("detail = %q, want the drifting peer named", detail)
		}
	})

	t.Run("SAD unreadable is not drift", func(t *testing.T) {
		useDriftSAD(t, nil, errors.New("operation not permitted"))
		status, detail := checkIPsecHealth()
		if status != health.StatusHealthy {
			t.Errorf("status = %v (%q): an unreadable dataplane is not evidence of drift", status, detail)
		}
	})
}
