//go:build linux

// Design: docs/architecture/anomaly/anomaly-2-shape.md -- armed limit action drops excess packets and counts drops
// Related: match.go -- buildActions, buildSourceTerm, buildTables
// Related: responder.go -- responder.onDetected, responder.onCleared, responder.reinstall

package shape

import (
	"errors"
	"math"
	"net"
	"net/netip"
	"os"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/core/anomalyevent"
	_ "github.com/ze-software/ze/internal/plugins/firewall/nft"
	"github.com/ze-software/ze/internal/test/userns"
)

const (
	testLimitRate  = 5
	testLimitBurst = 5
	testLimitUnit  = "second"
	burstPackets   = 60
	burstSlack     = 2
	udpProbeText   = "ze-anomaly-shape-limit-probe"
	udpDrainIdle   = 100 * time.Millisecond
	udpReadBufSize = 1 << 20
)

type udpBurstResult struct {
	received int
	window   time.Duration
}

// TestAnomalyShapeLimitDropsTheExcess proves that arming a source under
// action limit programs a real Linux nftables rule that passes within-rate
// packets, drops excess packets, counts only the dropped excess packets in the
// term counter, leaves unarmed sources untouched, and restores full delivery
// after onCleared withdraws the rule.
//
// VALIDATES: AC-1 and the Wiring Test from responder.onDetected through
// buildActions, firewall.RegisterTables, firewall.ApplyAll, and firewallnft
// against a real Linux kernel in an isolated user and network namespace.
// PREVENTS: an action limit rule with no drop verdict or inverted Over polarity
// reporting a source as armed while dropping nothing.
func TestAnomalyShapeLimitDropsTheExcess(t *testing.T) {
	if !userns.Enter(t) {
		return
	}

	var registerErr error
	var applyErr error
	origRegisterTables := registerTables
	origApplyAll := applyAll
	registerTables = func(own string, tables []firewall.Table) error {
		registerErr = origRegisterTables(own, tables)
		return registerErr
	}
	applyAll = func() error {
		applyErr = origApplyAll()
		return applyErr
	}
	t.Cleanup(func() {
		registerTables = origRegisterTables
		applyAll = origApplyAll
	})

	cfg := DefaultConfig()
	cfg.Mode = ModeArmed
	cfg.Action = ActionLimit
	cfg.LimitRate = testLimitRate
	cfg.LimitUnit = testLimitUnit
	cfg.LimitBurst = testLimitBurst
	if err := cfg.Validate(); err != nil {
		t.Fatalf("validate test config: %v", err)
	}

	r := newResponder(cfg)
	t.Cleanup(func() {
		r.Stop()
		if err := firewall.RegisterTables(owner, nil); err != nil {
			t.Errorf("withdraw anomaly-shape tables: %v", err)
		}
		if err := firewall.ApplyAll(); err != nil {
			t.Errorf("apply anomaly-shape table withdrawal: %v", err)
		}
		if err := firewall.CloseBackend(); err != nil {
			t.Errorf("close firewall backend: %v", err)
		}
	})

	armedSrc := netip.MustParseAddr("127.0.0.1")
	controlSrc := netip.MustParseAddr("127.0.0.2")
	armedPrefix := netip.PrefixFrom(armedSrc, 32)

	listener, dstAddr := listenLoopbackUDP(t, armedSrc)

	r.onDetected(&anomalyevent.AnomalyDetected{
		EntityKind: anomalyevent.EntityKindSource,
		Entity:     armedPrefix,
	})
	if registerErr != nil {
		t.Fatalf("register anomaly-shape tables onDetected: %v", registerErr)
	}
	if applyErr != nil {
		t.Fatalf("apply anomaly-shape tables onDetected in test namespace: %v", applyErr)
	}

	armed := probeUDPBurst(t, listener, armedSrc, dstAddr)
	refill := int(math.Ceil(float64(testLimitRate) * armed.window.Seconds()))
	ceiling := testLimitBurst + refill + burstSlack

	if armed.received <= 0 {
		t.Fatalf("armed source received %d of %d packets, want > 0 conforming packets passed",
			armed.received, burstPackets)
	}
	if armed.received >= burstPackets {
		t.Fatalf("armed source received %d of %d packets in %v, want excess dropped (< %d)",
			armed.received, burstPackets, armed.window, burstPackets)
	}
	if armed.received > ceiling {
		t.Fatalf("armed source received %d of %d packets in %v, want <= %d (burst %d + refill %d + slack %d)",
			armed.received, burstPackets, armed.window, ceiling, testLimitBurst, refill, burstSlack)
	}

	termName := buildSourceTerm(armedPrefix, cfg).Name
	counter := readTermCounter(t, tableNameV4, "ingress", termName)
	wantDropped := uint64(burstPackets - armed.received)
	if counter.Packets == 0 {
		t.Fatalf("term %q counter.Packets = 0, want %d dropped excess packets", termName, wantDropped)
	}
	if counter.Packets >= uint64(burstPackets) {
		t.Fatalf("term %q counter.Packets = %d (>= sent %d): counter counts total traffic instead of dropped excess",
			termName, counter.Packets, burstPackets)
	}
	if counter.Packets != wantDropped {
		t.Fatalf("term %q counter.Packets = %d, want %d (sent %d - received %d)",
			termName, counter.Packets, wantDropped, burstPackets, armed.received)
	}

	control := probeUDPBurst(t, listener, controlSrc, dstAddr)
	if control.received != burstPackets {
		t.Fatalf("unarmed control source %s received %d of %d packets, want all %d delivered",
			controlSrc, control.received, burstPackets, burstPackets)
	}

	r.onCleared(&anomalyevent.AnomalyCleared{
		EntityKind: anomalyevent.EntityKindSource,
		Entity:     armedPrefix,
	})
	if registerErr != nil {
		t.Fatalf("register anomaly-shape withdrawal onCleared: %v", registerErr)
	}
	if applyErr != nil {
		t.Fatalf("apply anomaly-shape withdrawal onCleared in test namespace: %v", applyErr)
	}

	afterClear := probeUDPBurst(t, listener, armedSrc, dstAddr)
	if afterClear.received != burstPackets {
		t.Fatalf("after onCleared source %s received %d of %d packets, want all %d delivered",
			armedSrc, afterClear.received, burstPackets, burstPackets)
	}
}

func listenLoopbackUDP(t *testing.T, addr netip.Addr) (*net.UDPConn, *net.UDPAddr) {
	t.Helper()

	listener, err := net.ListenUDP("udp4", &net.UDPAddr{IP: addr.AsSlice(), Port: 0})
	if err != nil {
		t.Fatalf("listen UDP on %s: %v", addr, err)
	}
	t.Cleanup(func() {
		if err := listener.Close(); err != nil {
			t.Errorf("close UDP listener: %v", err)
		}
	})
	if err := listener.SetReadBuffer(udpReadBufSize); err != nil {
		t.Fatalf("set UDP listener read buffer: %v", err)
	}
	dstAddr, ok := listener.LocalAddr().(*net.UDPAddr)
	if !ok {
		t.Fatalf("listener local addr has type %T, want *net.UDPAddr", listener.LocalAddr())
	}
	return listener, dstAddr
}

func probeUDPBurst(t *testing.T, listener *net.UDPConn, src netip.Addr, dst *net.UDPAddr) udpBurstResult {
	t.Helper()

	sender, err := net.DialUDP("udp4", &net.UDPAddr{IP: src.AsSlice(), Port: 0}, dst)
	if err != nil {
		t.Fatalf("dial UDP from %s to %s: %v", src, dst, err)
	}
	defer func() {
		if err := sender.Close(); err != nil {
			t.Errorf("close UDP sender %s: %v", src, err)
		}
	}()

	payload := []byte(udpProbeText)
	start := time.Now()
	for range burstPackets {
		if _, err := sender.Write(payload); err != nil {
			t.Fatalf("send UDP packet from %s: %v", src, err)
		}
	}
	window := time.Since(start)

	received := 0
	buf := make([]byte, len(payload)+16)
	for {
		if err := listener.SetReadDeadline(time.Now().Add(udpDrainIdle)); err != nil {
			t.Fatalf("set UDP read deadline: %v", err)
		}
		_, err := listener.Read(buf)
		if err == nil {
			received++
			continue
		}
		if errors.Is(err, os.ErrDeadlineExceeded) {
			break
		}
		t.Fatalf("read UDP packet from listener: %v", err)
	}
	return udpBurstResult{received: received, window: window}
}

func readTermCounter(t *testing.T, tableName, chainName, termName string) firewall.TermCounter {
	t.Helper()

	backend := firewall.GetBackend()
	if backend == nil {
		t.Fatalf("firewall backend is nil after applying table %q", tableName)
	}
	chains, err := backend.GetCounters(tableName)
	if err != nil {
		t.Fatalf("get counters for table %q: %v", tableName, err)
	}
	for _, chain := range chains {
		if chain.Chain != chainName {
			continue
		}
		for _, term := range chain.Terms {
			if term.Name == termName {
				return term
			}
		}
	}
	t.Fatalf("counter for table %q chain %q term %q not found in %+v", tableName, chainName, termName, chains)
	return firewall.TermCounter{}
}
