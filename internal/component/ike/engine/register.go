// Design: plan/spec-ipsec-7-ikev2-engine.md -- IKE engine component registration
package engine

import (
	"fmt"
	"log/slog"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/component/ike/transport"
	"codeberg.org/thomas-mangin/ze/internal/component/ipsec"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

var (
	loggerPtr   atomic.Pointer[slog.Logger]
	eventBusPtr atomic.Pointer[ze.EventBus]
)

func setLogger(l *slog.Logger)   { loggerPtr.Store(l) }
func getLogger() *slog.Logger    { return loggerPtr.Load() }
func setEventBus(eb ze.EventBus) { eventBusPtr.Store(&eb) }

func getEventBus() ze.EventBus {
	p := eventBusPtr.Load()
	if p == nil {
		return nil
	}
	return *p
}

func init() {
	d := slogutil.DiscardLogger()
	loggerPtr.Store(d)

	reg := registry.Registration{
		Name:        "ike",
		Description: "IKEv2 engine for native IPsec VPN",
		ConfigRoots: []string{"vpn"},
		RunEngine:   runEngine,
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
		},
		ConfigureEventBus: func(eb any) {
			if e, ok := eb.(ze.EventBus); ok {
				setEventBus(e)
			}
		},
	}
	reg.CLIHandler = func(_ []string) int { return 1 }
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "ike: registration failed: %v\n", err)
		os.Exit(1)
	}
}

func runEngine(conn net.Conn) int {
	log := getLogger()
	log.Debug("ike engine starting")

	p := sdk.NewWithConn("ike", conn)
	defer closeSDK(p)

	table := NewSATable()
	var tr *transport.UDPTransport
	var activeCfg *ipsec.IPsecConfig
	activePeers := make(map[string]*PeerSession)

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		cfg, err := parseIPsecSections(sections)
		if err != nil {
			return fmt.Errorf("ike config: %w", err)
		}

		if tr == nil && len(cfg.Peers) > 0 {
			listenAddr := "0.0.0.0:500"
			if cfg.Interface != "" {
				listenAddr = cfg.Interface + ":500"
			}
			var tErr error
			tr, tErr = transport.NewUDPTransport(listenAddr, log)
			if tErr != nil {
				log.Warn("ike: failed to start UDP transport", "error", tErr)
			} else {
				go tr.Run()
				go dispatchInbound(tr, table, log)
			}
		}

		eb := getEventBus()
		reconcilePeers(cfg, activeCfg, activePeers, table, tr, eb, log)
		activeCfg = cfg

		log.Info("ike engine configured", "peers", len(cfg.Peers))
		return nil
	})

	ctx, cancel := sdk.SignalContext()
	defer cancel()

	if err := p.Run(ctx, sdk.Registration{
		WantsConfig: []string{"vpn"},
	}); err != nil {
		log.Error("ike engine failed", "error", err)
		return 1
	}

	// Cleanup after Run returns (shutdown).
	for name, ps := range activePeers {
		ps.Stop()
		delete(activePeers, name)
	}
	if tr != nil {
		if err := tr.Close(); err != nil {
			log.Warn("ike: transport close error", "error", err)
		}
	}

	return 0
}

func closeSDK(p *sdk.Plugin) {
	if err := p.Close(); err != nil {
		getLogger().Debug("ike: sdk close", "error", err)
	}
}

// inboundRateLimiter is a simple token-bucket rate limiter for inbound IKE packets.
type inboundRateLimiter struct {
	mu       sync.Mutex
	tokens   int
	max      int
	lastFill time.Time
	rate     int // tokens per second
}

func newInboundRateLimiter(perSecond, burst int) *inboundRateLimiter {
	return &inboundRateLimiter{
		tokens:   burst,
		max:      burst,
		rate:     perSecond,
		lastFill: time.Now(),
	}
}

func (l *inboundRateLimiter) allow() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	elapsed := now.Sub(l.lastFill)
	if elapsed >= time.Second {
		l.tokens = l.max
		l.lastFill = now
	} else {
		fill := int(elapsed.Seconds() * float64(l.rate))
		l.tokens = min(l.tokens+fill, l.max)
		if fill > 0 {
			l.lastFill = now
		}
	}
	if l.tokens <= 0 {
		return false
	}
	l.tokens--
	return true
}

// dispatchInbound reads packets from the transport and dispatches to the
// correct SA by SPI pair.
func dispatchInbound(tr *transport.UDPTransport, table *SATable, log *slog.Logger) {
	limiter := newInboundRateLimiter(100, 200)

	for pkt := range tr.Recv() {
		if len(pkt.Data) < 28 {
			continue
		}
		// RFC 7296 Section 3.1: major version in upper nibble of byte 17.
		if pkt.Data[17]>>4 != 2 {
			continue
		}
		if !limiter.allow() {
			continue
		}

		var iSPI, rSPI [8]byte
		copy(iSPI[:], pkt.Data[0:8])
		copy(rSPI[:], pkt.Data[8:16])

		// RFC 7296 Section 2.6: initiator SPI MUST NOT be zero.
		if iSPI == [8]byte{} {
			continue
		}

		sa := table.Lookup(iSPI, rSPI)
		if sa == nil && rSPI == [8]byte{} {
			sa = table.LookupByInitiatorSPI(iSPI)
		}
		if sa == nil {
			log.Debug("ike: no SA for packet", "ispi", SPIHex(iSPI), "rspi", SPIHex(rSPI))
			continue
		}

		handleInbound(sa, pkt, table, tr, log)
	}
}
