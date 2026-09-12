// VALIDATES: that checkIPsecHealth reports degraded, and NAMES the peer, when the
// kernel SAD does not hold a Child SA the engine counts as installed (AC-5).
// PREVENTS: the false green this spec exists to remove -- every other arm of the
// health check reads engine belief, so a tunnel the kernel dropped stays healthy.
//
// Design: docs/architecture/ike/ipsec-dataplane-inspection.md -- kernel dataplane read surface
// Related: health_drift.go -- driftingPeers, driftDetail

package engine

import (
	"errors"
	"net"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/core/health"
	"github.com/ze-software/ze/internal/core/slogutil"
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
		useDriftSAD(t, []dataplane.SAInfo{{SPI: 100, Proto: dataplane.ProtoESP}, {SPI: 200, Proto: dataplane.ProtoESP}}, nil)
		status, detail := checkIPsecHealth()
		if status != health.StatusHealthy {
			t.Errorf("status = %v (%q), want healthy when belief and kernel agree", status, detail)
		}
	})

	t.Run("kernel is missing the outbound SPI", func(t *testing.T) {
		useDriftSAD(t, []dataplane.SAInfo{{SPI: 100, Proto: dataplane.ProtoESP}}, nil)
		status, detail := checkIPsecHealth()
		if status == health.StatusHealthy {
			t.Error("status = healthy while the kernel does not hold the outbound child SA")
		}
		if !strings.Contains(detail, "peer-alpha") {
			t.Errorf("detail = %q, want the drifting peer named", detail)
		}
	})

	t.Run("SAD unreadable is unknown health", func(t *testing.T) {
		useDriftSAD(t, nil, errors.New("operation not permitted"))
		status, detail := checkIPsecHealth()
		if status == health.StatusHealthy || detail == "" {
			t.Errorf("unreadable dataplane returned status %v and detail %q", status, detail)
		}
	})
}

// VALIDATES: driftingPeersFrom compares a SUPPLIED SA list in one direction, so
// an SPI the kernel holds and the engine does not name is not drift.
// PREVENTS: a rekey window reading as drift. RFC 7296 Section 2.8 keeps the old
// and the new Child SA alive together, so the kernel legitimately holds an SPI
// the engine has already replaced.
func TestDriftingPeersFromSAsComparesOneDirection(t *testing.T) {
	alpha := &PeerSession{peerName: "peer-alpha"}
	alpha.childSA = &ChildSA{InboundSPI: 100, OutboundSPI: 200}
	beta := &PeerSession{peerName: "peer-beta"}
	beta.childSA = &ChildSA{InboundSPI: 300, OutboundSPI: 400}
	SetActivePeersForTest(map[string]*PeerSession{"peer-alpha": alpha, "peer-beta": beta})
	t.Cleanup(func() { SetActivePeersForTest(nil) })

	tests := []struct {
		name string
		sas  []dataplane.SAInfo
		want []string
	}{
		{
			name: "kernel holds every believed SPI",
			sas:  []dataplane.SAInfo{{SPI: 100}, {SPI: 200}, {SPI: 300}, {SPI: 400}},
		},
		{
			name: "kernel holds an extra SPI from a rekey window",
			sas:  []dataplane.SAInfo{{SPI: 100}, {SPI: 200}, {SPI: 300}, {SPI: 400}, {SPI: 999}},
		},
		{
			name: "kernel is missing one SPI of one peer",
			sas:  []dataplane.SAInfo{{SPI: 100}, {SPI: 300}, {SPI: 400}},
			want: []string{"peer-alpha"},
		},
		{
			name: "kernel holds nothing",
			want: []string{"peer-alpha", "peer-beta"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for i := range tt.sas {
				tt.sas[i].Proto = dataplane.ProtoESP
			}
			got := driftingPeersFrom(DataplaneSnapshot{Peers: PeerInfoMap(), SAs: tt.sas})
			if !slices.Equal(got, tt.want) {
				t.Errorf("driftingPeersFrom = %v, want %v", got, tt.want)
			}
		})
	}
}

// Equal SPIs can name different destinations, interfaces, and protocols. A foreign
// state must neither conceal missing ESP state nor make the peer healthy.
func TestHealthDriftUsesQualifiedSAIdentity(t *testing.T) {
	establishedPeer(t, "peer-alpha", 100, 200)
	ps := ActivePeers()["peer-alpha"]
	local := net.ParseIP("192.0.2.1")
	remote := net.ParseIP("192.0.2.2")
	ps.setChildSA(&ChildSA{InboundSPI: 100, OutboundSPI: 200, LocalAddr: local, RemoteAddr: remote, IfID: 7})
	inbound := dataplane.SAInfo{SPI: 100, Dst: local.To4(), Proto: dataplane.ProtoESP, IfID: 7}
	outbound := dataplane.SAInfo{SPI: 200, Dst: remote.To4(), Proto: dataplane.ProtoESP, IfID: 7}
	for _, field := range []string{"destination", "interface", "protocol"} {
		t.Run(field, func(t *testing.T) {
			other := outbound
			switch field {
			case "destination":
				other.Dst = net.ParseIP("192.0.2.3")
			case "interface":
				other.IfID = 8
			case "protocol":
				other.Proto = dataplane.ProtoAH
			}
			useDriftSAD(t, []dataplane.SAInfo{inbound, outbound, other}, nil)
			if status, detail := checkIPsecHealth(); status != health.StatusHealthy {
				t.Fatalf("coexisting SAs returned %v (%s)", status, detail)
			}
			useDriftSAD(t, []dataplane.SAInfo{inbound, other}, nil)
			status, detail := checkIPsecHealth()
			if status == health.StatusHealthy || !strings.Contains(detail, "peer-alpha") {
				t.Fatalf("same SPI concealed missing %s-qualified SA: %v (%s)", field, status, detail)
			}
		})
	}
}

// Publication in the dump callback deterministically crosses the observation.
// The replacement is already installed; the returned dump predates it.
func TestHealthUnknownWhenChildChangesDuringDump(t *testing.T) {
	establishedPeer(t, "peer-alpha", 100, 200)
	ps := ActivePeers()["peer-alpha"]
	old := driftSAD
	t.Cleanup(func() { driftSAD = old })
	driftSAD = func() ([]dataplane.SAInfo, error) {
		ps.setChildSA(&ChildSA{InboundSPI: 300, OutboundSPI: 400})
		return []dataplane.SAInfo{{SPI: 100, Proto: dataplane.ProtoESP}, {SPI: 200, Proto: dataplane.ProtoESP}}, nil
	}
	status, detail := checkIPsecHealth()
	if status == health.StatusHealthy || strings.Contains(detail, "drift:") || detail == "" {
		t.Fatalf("changing observation returned %v (%s), want unknown health", status, detail)
	}
	useDriftSAD(t, []dataplane.SAInfo{
		{SPI: 100, Proto: dataplane.ProtoESP}, {SPI: 200, Proto: dataplane.ProtoESP},
		{SPI: 300, Proto: dataplane.ProtoESP}, {SPI: 400, Proto: dataplane.ProtoESP},
	}, nil)
	if status, detail := checkIPsecHealth(); status != health.StatusHealthy {
		t.Fatalf("stable rekey overlap returned %v (%s)", status, detail)
	}
}

type observationBlockingDP struct {
	mockDP
	entered chan struct{}
	release chan struct{}
}

func (d *observationBlockingDP) RemoveSA(uint32, net.IP, uint8) error {
	close(d.entered)
	<-d.release
	return nil
}

// Two independent peer actors can change the dataplane at once. Completing one
// removal must not let health call the still-active writer's observation clean.
func TestHealthUnknownUntilEveryDataplaneWriterFinishes(t *testing.T) {
	establishedPeer(t, "peer-alpha", 100, 200)
	useDriftSAD(t, []dataplane.SAInfo{
		{SPI: 100, Proto: dataplane.ProtoESP}, {SPI: 200, Proto: dataplane.ProtoESP},
	}, nil)
	var done [2]chan struct{}
	var backends [2]*observationBlockingDP
	for i := range backends {
		d := &observationBlockingDP{entered: make(chan struct{}), release: make(chan struct{})}
		backends[i] = d
		done[i] = make(chan struct{})
		go func() {
			defer close(done[i])
			removeChildSAOutgoing(&ChildSA{OutboundSPI: uint32(300 + i)}, d, slogutil.DiscardLogger(), false)
		}()
		<-d.entered
	}
	// Both blocked removals are joined even if an assertion fails.
	t.Cleanup(func() {
		for i, d := range backends {
			select {
			case <-d.release:
			default:
				close(d.release)
			}
			<-done[i]
		}
	})
	for i, d := range backends {
		if status, detail := checkIPsecHealth(); status == health.StatusHealthy {
			t.Fatalf("%d writers remained, health returned %v (%s)", 2-i, status, detail)
		}
		close(d.release)
		<-done[i]
	}
	if status, detail := checkIPsecHealth(); status != health.StatusHealthy {
		t.Fatalf("completed removals left health %v (%s)", status, detail)
	}
}

func TestHealthUnknownBetweenRemovalAndChildPublication(t *testing.T) {
	establishedPeer(t, "peer-alpha", 100, 200)
	ps := ActivePeers()["peer-alpha"]
	// The backend removal has returned, but the owner has not cleared belief yet.
	removeChildSAOutgoing(ps.getChildSA(), &mockDP{}, slogutil.DiscardLogger(), false)
	useDriftSAD(t, []dataplane.SAInfo{{SPI: 100, Proto: dataplane.ProtoESP}}, nil)
	status, detail := checkIPsecHealth()
	if status == health.StatusHealthy || strings.Contains(detail, "drift:") || detail == "" {
		t.Fatalf("removal/publication gap returned %v (%s)", status, detail)
	}
	ps.setChildSA(nil)
	if status, detail := checkIPsecHealth(); status != health.StatusHealthy {
		t.Fatalf("published child removal returned %v (%s)", status, detail)
	}
}
