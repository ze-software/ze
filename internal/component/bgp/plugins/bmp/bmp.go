// Design: docs/architecture/core-design.md -- BMP plugin lifecycle
//
// Related: header.go -- wire format encode/decode
// Related: tlv.go -- TLV encode/decode
// Related: msg.go -- message type encode/decode

package bmp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
)

// maxBMPMsgSize is the upper bound on a single BMP message.
// BGP max (4096) + BMP framing (48) with generous headroom for TLVs.
const maxBMPMsgSize = 65535

// sessionReadDeadline is the read deadline for receiver sessions.
// Ensures sessions are interruptible on shutdown.
const sessionReadDeadline = 30 * time.Second

// loggerPtr is the package-level logger, disabled by default.
var loggerPtr atomic.Pointer[slog.Logger]

func init() {
	d := slogutil.DiscardLogger()
	loggerPtr.Store(d)
}

func logger() *slog.Logger { return loggerPtr.Load() }

func setLogger(l *slog.Logger) {
	if l != nil {
		loggerPtr.Store(l)
	}
}

// receiverConfig holds parsed receiver configuration.
type receiverConfig struct {
	Servers     []listenerConfig `json:"server"`
	MaxSessions uint16           `json:"max-sessions"`
}

type listenerConfig struct {
	Name string `json:"name"`
	IP   string `json:"ip"`
	Port uint16 `json:"port"`
}

// senderConfig holds parsed sender configuration.
type senderConfig struct {
	Collectors            []collectorConfig `json:"collector"`
	RouteMonitoringPolicy string            `json:"route-monitoring-policy"`
	StatisticsTimeout     uint16            `json:"statistics-timeout"`
}

type collectorConfig struct {
	Name    string `json:"name"`
	Address string `json:"address"`
	Port    uint16 `json:"port"`
}

// bmpConfig is the top-level BMP config parsed from the bgp section.
type bmpConfig struct {
	Receiver *receiverConfig `json:"receiver"`
	Sender   *senderConfig   `json:"sender"`
}

// bgpSection wraps the bmp key from the bgp config section.
type bgpSection struct {
	BMP *bmpConfig `json:"bmp"`
}

// BMPPlugin implements the bgp-bmp plugin.
// It manages both receiver (TCP listener for inbound BMP) and
// sender (outbound TCP to collectors) functionality.
//
// Caller MUST close stopCh and call stopListeners when done.
type BMPPlugin struct {
	plugin *sdk.Plugin
	mu     sync.RWMutex

	// Receiver state.
	listeners []net.Listener
	sessions  sync.WaitGroup

	// Sender state.
	senders []*senderSession

	// stopCh signals all background goroutines to stop.
	stopCh chan struct{}
}

// RunBMPPlugin is the in-process entry point for the bgp-bmp plugin.
func RunBMPPlugin(conn net.Conn) int {
	logger().Debug("bgp-bmp plugin starting")

	p := sdk.NewWithConn("bgp-bmp", conn)
	defer closeLog(p, "plugin")

	bp := &BMPPlugin{
		plugin: p,
		stopCh: make(chan struct{}),
	}

	defer func() {
		close(bp.stopCh)
		bp.stopSenders()
		bp.stopListeners()
		bp.sessions.Wait()
	}()

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		for _, section := range sections {
			if section.Root != "bgp" {
				continue
			}
			cfg, err := parseBMPConfig(section.Data)
			if err != nil {
				logger().Error("bmp: config parse failed", "error", err)
				return err
			}
			if cfg == nil {
				continue
			}
			if cfg.Receiver != nil {
				bp.startReceiver(cfg.Receiver)
			}
			if cfg.Sender != nil {
				bp.startSender(cfg.Sender)
			}
		}
		return nil
	})

	ctx := context.Background()
	err := p.Run(ctx, sdk.Registration{
		Commands: []sdk.CommandDecl{
			{Name: "bmp sessions"},
			{Name: "bmp peers"},
			{Name: "bmp collectors"},
		},
		WantsConfig: []string{"bgp"},
	})
	if err != nil {
		logger().Error("bgp-bmp plugin failed", "error", err)
		return 1
	}

	return 0
}

// closeLog closes c and logs any error. Used in deferred cleanup.
func closeLog(c interface{ Close() error }, what string) {
	if err := c.Close(); err != nil {
		logger().Debug("bmp: close failed", "what", what, "error", err)
	}
}

// parseBMPConfig extracts BMP config from the bgp section JSON.
func parseBMPConfig(data string) (*bmpConfig, error) {
	var sec bgpSection
	if err := json.Unmarshal([]byte(data), &sec); err != nil {
		return nil, fmt.Errorf("bmp config: %w", err)
	}
	return sec.BMP, nil
}

// startReceiver starts TCP listeners for the BMP receiver.
func (bp *BMPPlugin) startReceiver(cfg *receiverConfig) {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	for _, srv := range cfg.Servers {
		addr := net.JoinHostPort(srv.IP, fmt.Sprintf("%d", srv.Port))
		var lc net.ListenConfig
		ln, err := lc.Listen(context.Background(), "tcp", addr)
		if err != nil {
			logger().Error("bmp: listener bind failed", "address", addr, "error", err)
			continue
		}
		bp.listeners = append(bp.listeners, ln)
		logger().Info("bmp: receiver listening", "address", addr)

		maxSess := cfg.MaxSessions
		bp.sessions.Go(func() {
			bp.acceptLoop(ln, maxSess)
		})
	}
}

// stopListeners closes all receiver listeners.
func (bp *BMPPlugin) stopListeners() {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	for _, ln := range bp.listeners {
		if err := ln.Close(); err != nil {
			logger().Debug("bmp: listener close", "error", err)
		}
	}
	bp.listeners = nil
}

// startSender starts outbound TCP connections to BMP collectors.
func (bp *BMPPlugin) startSender(cfg *senderConfig) {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	for _, col := range cfg.Collectors {
		ss := newSenderSession(col)
		bp.senders = append(bp.senders, ss)
		bp.sessions.Go(ss.run)
		logger().Info("bmp: sender started", "collector", col.Name, "address", col.Address, "port", col.Port)
	}
}

// stopSenders stops all sender sessions.
func (bp *BMPPlugin) stopSenders() {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	for _, ss := range bp.senders {
		ss.stop()
	}
	bp.senders = nil
}

// acceptLoop accepts BMP connections on the listener until it is closed.
func (bp *BMPPlugin) acceptLoop(ln net.Listener, maxSessions uint16) {
	var active atomic.Int32

	for {
		conn, err := ln.Accept()
		if err != nil {
			if bp.isStopping() {
				return
			}
			logger().Warn("bmp: accept failed", "error", err)
			return
		}

		// Increment before goroutine spawn to avoid TOCTOU race at the limit.
		if int(active.Add(1)) > int(maxSessions) {
			active.Add(-1)
			logger().Warn("bmp: max sessions reached, rejecting", "remote", conn.RemoteAddr())
			closeLog(conn, "rejected-conn")
			continue
		}

		bp.sessions.Go(func() {
			defer active.Add(-1)
			bp.handleSession(conn)
		})
	}
}

// isStopping returns true if the stop channel has been closed.
func (bp *BMPPlugin) isStopping() bool {
	select {
	case <-bp.stopCh:
		return true
	default: // active
		return false
	}
}

// handleSession processes a single BMP session from a remote router.
// RFC 7854: unidirectional, router -> receiver.
func (bp *BMPPlugin) handleSession(conn net.Conn) {
	defer closeLog(conn, "session")

	remote := conn.RemoteAddr().String()
	logger().Info("bmp: session started", "remote", remote)
	defer logger().Info("bmp: session ended", "remote", remote)

	headerBuf := make([]byte, CommonHeaderSize)
	for {
		// Set read deadline so the loop is interruptible on shutdown.
		if err := conn.SetReadDeadline(time.Now().Add(sessionReadDeadline)); err != nil {
			return
		}

		// Read 6-byte common header.
		if _, err := io.ReadFull(conn, headerBuf); err != nil {
			if bp.isStopping() {
				return
			}
			logger().Debug("bmp: read header failed", "remote", remote, "error", err)
			return
		}

		ch, _, err := DecodeCommonHeader(headerBuf, 0)
		if err != nil {
			logger().Warn("bmp: bad header", "remote", remote, "error", err)
			return
		}

		msgLen := int(ch.Length)
		if msgLen < CommonHeaderSize {
			logger().Warn("bmp: invalid length", "remote", remote, "length", msgLen)
			return
		}
		if msgLen > maxBMPMsgSize {
			logger().Warn("bmp: message too large", "remote", remote, "length", msgLen, "max", maxBMPMsgSize)
			return
		}

		msgBuf := make([]byte, msgLen)
		copy(msgBuf, headerBuf)
		remaining := msgLen - CommonHeaderSize
		if remaining > 0 {
			if _, err := io.ReadFull(conn, msgBuf[CommonHeaderSize:]); err != nil {
				logger().Debug("bmp: read body failed", "remote", remote, "error", err)
				return
			}
		}

		msg, err := DecodeMsg(msgBuf)
		if err != nil {
			logger().Warn("bmp: decode failed", "remote", remote, "error", err)
			return
		}

		bp.processMessage(remote, msg)
	}
}

// processMessage dispatches a decoded BMP message to the appropriate handler.
func (bp *BMPPlugin) processMessage(remote string, msg any) {
	switch m := msg.(type) {
	case *Initiation:
		bp.processInitiation(remote, m)
	case *Termination:
		bp.processTermination(remote, m)
	case *PeerUp:
		bp.processPeerUp(remote, m)
	case *PeerDown:
		bp.processPeerDown(remote, m)
	case *RouteMonitoring:
		bp.processRouteMonitoring(remote, m)
	case *StatisticsReport:
		bp.processStatisticsReport(remote, m)
	case *RouteMirroring:
		bp.processRouteMirroring(remote, m)
	}
}

func (bp *BMPPlugin) processInitiation(remote string, m *Initiation) {
	for _, tlv := range m.TLVs {
		switch tlv.Type { //nolint:exhaustive // RFC 7854: unknown TLV types are silently ignored
		case InitTLVSysName:
			logger().Info("bmp: initiation", "remote", remote, "sysName", string(tlv.Value))
		case InitTLVSysDescr:
			logger().Info("bmp: initiation", "remote", remote, "sysDescr", string(tlv.Value))
		case InitTLVString:
			logger().Info("bmp: initiation", "remote", remote, "message", string(tlv.Value))
		}
	}
}

func (bp *BMPPlugin) processTermination(remote string, _ *Termination) {
	logger().Info("bmp: termination received", "remote", remote)
}

func (bp *BMPPlugin) processPeerUp(remote string, m *PeerUp) {
	logger().Info("bmp: peer up",
		"remote", remote,
		"peer-as", m.Peer.PeerAS,
		"peer-bgp-id", fmt.Sprintf("%08x", m.Peer.PeerBGPID),
		"local-port", m.LocalPort,
		"remote-port", m.RemotePort,
	)
}

func (bp *BMPPlugin) processPeerDown(remote string, m *PeerDown) {
	logger().Info("bmp: peer down",
		"remote", remote,
		"peer-as", m.Peer.PeerAS,
		"reason", m.Reason,
	)
}

func (bp *BMPPlugin) processRouteMonitoring(remote string, m *RouteMonitoring) {
	logger().Debug("bmp: route monitoring",
		"remote", remote,
		"peer-as", m.Peer.PeerAS,
		"update-len", len(m.BGPUpdate),
	)
}

func (bp *BMPPlugin) processStatisticsReport(remote string, m *StatisticsReport) {
	logger().Debug("bmp: statistics report",
		"remote", remote,
		"peer-as", m.Peer.PeerAS,
		"stats-count", len(m.Stats),
	)
}

func (bp *BMPPlugin) processRouteMirroring(remote string, m *RouteMirroring) {
	logger().Debug("bmp: route mirroring",
		"remote", remote,
		"peer-as", m.Peer.PeerAS,
		"tlv-count", len(m.TLVs),
	)
}
