// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- PPPoE client interface kind
// Related: config.go -- pppoeClientEntry parsed from YANG config

package iface

import (
	"errors"
	"log/slog"
	"net/netip"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// PPPoE client session states.
type pppoeSessionState uint8

const (
	sessStateDisconnected pppoeSessionState = iota
	sessStateDiscovery
	sessStateLCP
	sessStateAuth
	sessStateNCP
	sessStateUp
)

var errStopped = errors.New("pppoe-client: stopped")

// Reconnect backoff constants.
const (
	reconnectBaseDelay = 1 * time.Second
	reconnectMaxDelay  = 60 * time.Second
)

// PPPoEClientConfig holds the configuration for a PPPoE client session.
type PPPoEClientConfig struct {
	Name            string
	SourceInterface string
	Username        string
	AuthSecret      string //nolint:gosec // not a hardcoded credential; parsed from ze:sensitive YANG leaf
	ServiceName     string
	ACName          string
	NoDefaultRoute  bool
	MTU             int
}

// pppoeClientStatus is a snapshot of the client state for show commands.
type pppoeClientStatus struct {
	Name            string
	State           string
	SourceInterface string
	PPPInterface    string
	LocalIP         netip.Addr
	PeerIP          netip.Addr
	SessionID       uint16
	Uptime          time.Duration
}

// PPPoEDialer abstracts the platform-specific PPPoE discovery and kernel
// session setup. Defined here to break the import cycle (pppoe imports
// iface). Production implementation lives in pppoeclient package.
type PPPoEDialer interface {
	Dial(cfg PPPoEClientConfig, stopCh <-chan struct{}, logger *slog.Logger) (PPPoESession, error)
}

// PPPoESession represents a fully negotiated PPPoE+PPP session.
type PPPoESession struct {
	SessionID uint16
	UnitNum   int
	LocalIP   netip.Addr
	PeerIP    netip.Addr
	NegMTU    uint16
	// Done is closed when the session ends (echo timeout, LCP terminate).
	// The PPPoEClient selects on this to detect session loss.
	Done <-chan struct{}
	// Cleanup closes kernel resources and sends PADT. Must be called
	// exactly once, after Done fires or on explicit stop.
	Cleanup func()
}

// PPPoEClient manages a single PPPoE client session lifecycle.
type PPPoEClient struct {
	cfg     PPPoEClientConfig
	backend Backend
	dialer  PPPoEDialer
	logger  *slog.Logger

	mu       sync.Mutex
	state    pppoeSessionState
	sessID   uint16
	pppUnit  int
	localIP  netip.Addr
	peerIP   netip.Addr
	upSince  time.Time
	lastErr  error
	attempts int

	stopCh chan struct{}
	done   chan struct{}
}

// NewPPPoEClient creates a PPPoE client from config. Call Start to begin.
func NewPPPoEClient(cfg PPPoEClientConfig, dialer PPPoEDialer, backend Backend, logger *slog.Logger) *PPPoEClient {
	return &PPPoEClient{
		cfg:     cfg,
		backend: backend,
		dialer:  dialer,
		logger:  logger.With("pppoe-client", cfg.Name, "src-iface", cfg.SourceInterface),
		state:   sessStateDisconnected,
		pppUnit: -1,
		stopCh:  make(chan struct{}),
		done:    make(chan struct{}),
	}
}

// Start launches the PPPoE client session goroutine.
func (c *PPPoEClient) Start() {
	go c.run()
}

// Stop signals the client to shut down and waits for cleanup.
func (c *PPPoEClient) Stop() {
	close(c.stopCh)
	<-c.done
}

// Status returns a snapshot of the client state.
func (c *PPPoEClient) status() pppoeClientStatus {
	c.mu.Lock()
	defer c.mu.Unlock()

	pppIface := ""
	if c.pppUnit >= 0 {
		pppIface = textbuf.StrInt("ppp", int64(c.pppUnit))
	}

	var uptime time.Duration
	if c.state == sessStateUp && !c.upSince.IsZero() {
		uptime = time.Since(c.upSince)
	}

	return pppoeClientStatus{
		Name:            c.cfg.Name,
		State:           c.stateString(),
		SourceInterface: c.cfg.SourceInterface,
		PPPInterface:    pppIface,
		LocalIP:         c.localIP,
		PeerIP:          c.peerIP,
		SessionID:       c.sessID,
		Uptime:          uptime,
	}
}

func (c *PPPoEClient) stateString() string {
	switch c.state {
	case sessStateDisconnected:
		return "disconnected"
	case sessStateDiscovery:
		return "discovery"
	case sessStateLCP:
		return "lcp"
	case sessStateAuth:
		return "authenticating"
	case sessStateNCP:
		return "ncp"
	case sessStateUp:
		return "up"
	}
	return "unknown"
}

func (c *PPPoEClient) stopped() bool {
	select {
	case <-c.stopCh:
		return true
	case <-time.After(0):
		return false
	}
}

// run is the main goroutine loop with reconnection on failure (AC-10).
func (c *PPPoEClient) run() {
	defer close(c.done)

	for {
		err := c.runSession()
		if err == nil || errors.Is(err, errStopped) {
			return
		}

		if c.stopped() {
			return
		}

		c.mu.Lock()
		c.state = sessStateDisconnected
		c.lastErr = err
		c.attempts++
		delay := ReconnectDelay(c.attempts)
		c.mu.Unlock()

		c.logger.Info("pppoe-client: session ended, reconnecting",
			"error", err, "delay", delay, "attempt", c.attempts)

		select {
		case <-c.stopCh:
			return
		case <-time.After(delay):
		}
	}
}

// ReconnectDelay computes exponential backoff clamped to reconnectMaxDelay.
func ReconnectDelay(attempt int) time.Duration {
	if attempt <= 0 {
		return reconnectBaseDelay
	}
	shift := min(attempt-1, 6)
	d := reconnectBaseDelay * time.Duration(1<<uint(shift))
	return min(d, reconnectMaxDelay)
}

// runSession executes one complete PPPoE client session: discovery,
// LCP/auth/NCP negotiation, interface setup, then monitors until teardown.
func (c *PPPoEClient) runSession() error {
	c.mu.Lock()
	c.state = sessStateDiscovery
	c.sessID = 0
	c.pppUnit = -1
	c.localIP = netip.Addr{}
	c.peerIP = netip.Addr{}
	c.mu.Unlock()

	// PPPoE Discovery + LCP + Auth + NCP via the Dialer abstraction.
	sess, err := c.dialer.Dial(c.cfg, c.stopCh, c.logger)
	if err != nil {
		return err
	}
	defer sess.Cleanup()

	ifname := textbuf.StrInt("ppp", int64(sess.UnitNum))

	// AC-7: Set MTU on the ppp interface.
	if c.backend != nil {
		mtu := int(sess.NegMTU)
		if c.cfg.MTU > 0 && c.cfg.MTU < mtu {
			mtu = c.cfg.MTU
		}
		if err := c.backend.SetMTU(ifname, mtu); err != nil {
			return err
		}
		if err := c.backend.SetAdminUp(ifname); err != nil {
			return err
		}
		// AC-5: Apply server-assigned address as P2P link.
		if sess.LocalIP.IsValid() && sess.PeerIP.IsValid() {
			localCIDR := sess.LocalIP.String() + "/32"
			peerCIDR := sess.PeerIP.String() + "/32"
			if err := c.backend.AddAddressP2P(ifname, localCIDR, peerCIDR); err != nil {
				return err
			}
		}
		// AC-8: Install default route unless no-default-route is set.
		if !c.cfg.NoDefaultRoute && sess.PeerIP.IsValid() {
			if err := c.backend.AddRoute(ifname, "0.0.0.0/0", sess.PeerIP.String(), 0); err != nil {
				c.logger.Warn("pppoe-client: failed to add default route", "error", err)
			}
		}
	}

	c.mu.Lock()
	c.sessID = sess.SessionID
	c.pppUnit = sess.UnitNum
	c.localIP = sess.LocalIP
	c.peerIP = sess.PeerIP
	c.state = sessStateUp
	c.upSince = time.Now()
	c.attempts = 0
	c.mu.Unlock()

	c.logger.Info("pppoe-client: session up",
		"session-id", sess.SessionID, "iface", ifname,
		"local-ip", sess.LocalIP, "peer-ip", sess.PeerIP)

	// AC-10: Wait for stop signal or session loss (echo timeout, LCP terminate).
	select {
	case <-c.stopCh:
	case <-sess.Done:
		return errors.New("pppoe-client: session lost")
	}
	return nil
}

// pppoeDialerVar holds the production PPPoEDialer, set by the pppoeclient
// package at init time. Nil on non-Linux or when the package is not linked.
var pppoeDialerVar PPPoEDialer

// SetPPPoEDialer registers the production dialer. Called from pppoeclient's
// init() or from the loader that imports the pppoeclient package.
func SetPPPoEDialer(d PPPoEDialer) { pppoeDialerVar = d }

// reconcilePPPoEClients starts clients for new config entries, stops
// clients whose config was removed or disabled. Caller holds pppoeMu.
func reconcilePPPoEClients(cfg *ifaceConfig, active map[string]*PPPoEClient, b Backend, log *slog.Logger) {
	if pppoeDialerVar == nil {
		return
	}

	desired := make(map[string]pppoeClientEntry, len(cfg.PPPoEClient))
	for _, e := range cfg.PPPoEClient {
		if !e.Disable {
			desired[e.Name] = e
		}
	}

	// Stop clients removed, disabled, or whose config changed.
	for name, client := range active {
		entry, ok := desired[name]
		if !ok {
			log.Info("pppoe-client: stopping removed client", "name", name)
			client.Stop()
			delete(active, name)
			continue
		}
		if pppoeClientConfigChanged(client.cfg, entry) {
			log.Info("pppoe-client: restarting client (config changed)", "name", name)
			client.Stop()
			delete(active, name)
		}
	}

	// Start clients that are new or were restarted above.
	for name, entry := range desired {
		if _, running := active[name]; running {
			continue
		}
		clientCfg := PPPoEClientConfig{
			Name:            entry.Name,
			SourceInterface: entry.SourceInterface,
			Username:        entry.Username,
			AuthSecret:      entry.AuthSecret,
			ServiceName:     entry.ServiceName,
			ACName:          entry.ACName,
			NoDefaultRoute:  entry.NoDefaultRoute,
			MTU:             entry.MTU,
		}
		client := NewPPPoEClient(clientCfg, pppoeDialerVar, b, log)
		client.Start()
		active[name] = client
		log.Info("pppoe-client: started client", "name", name, "src-iface", entry.SourceInterface)
	}
}

func pppoeClientConfigChanged(running PPPoEClientConfig, entry pppoeClientEntry) bool {
	return running.SourceInterface != entry.SourceInterface ||
		running.Username != entry.Username ||
		running.AuthSecret != entry.AuthSecret ||
		running.ServiceName != entry.ServiceName ||
		running.ACName != entry.ACName ||
		running.NoDefaultRoute != entry.NoDefaultRoute ||
		running.MTU != entry.MTU
}
