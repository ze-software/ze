// Design: docs/architecture/fleet-config.md — managed client connection lifecycle
// Related: handler.go — processes config responses from hub
// Related: reconnect.go — backoff for connection retries
// Related: heartbeat.go — liveness detection

package managed

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net"
	"time"

	pluginipc "codeberg.org/thomas-mangin/ze/internal/component/plugin/ipc"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	"codeberg.org/thomas-mangin/ze/pkg/fleet"
	"codeberg.org/thomas-mangin/ze/pkg/plugin/rpc"
)

const (
	heartbeatInterval = 30 * time.Second
	heartbeatMissed   = 3
	connectTimeout    = 5 * time.Second
	maxAuthResponse   = 512
)

var logger = slogutil.LazyLogger("hub.managed")

// ClientConfig holds the configuration for a managed client connection.
type ClientConfig struct {
	Name         string // Client identity (from hub client block name)
	Server       string // Hub address (host:port)
	Token        string // Auth token
	Version      string // Current config version hash (empty on first boot)
	Handler      *Handler
	OnReload     func()      // Called after new config is cached and applied
	CheckManaged func() bool // Returns false when meta/instance/managed is disabled; nil = always managed
}

// RunManagedClient connects to the hub and maintains the connection with
// reconnect and heartbeat. Blocks until ctx is canceled. This is a
// long-lived goroutine -- one per managed client instance.
func RunManagedClient(ctx context.Context, cfg ClientConfig) {
	backoff := NewBackoff(1*time.Second, 60*time.Second, 0.1)

	for {
		// Check managed flag before each connection attempt (AC-17).
		if cfg.CheckManaged != nil && !cfg.CheckManaged() {
			logger().Info("meta/instance/managed is false, stopping hub connection")
			return
		}

		err := runConnection(ctx, &cfg, backoff)
		if ctx.Err() != nil {
			return // shutdown
		}

		delay := backoff.Next()
		logger().Warn("connection lost, reconnecting",
			"delay", delay.Round(time.Millisecond),
			"error", err)

		timer := time.NewTimer(delay)
		select {
		case <-timer.C:
			// Continue reconnect loop.
		case <-ctx.Done():
			timer.Stop()
			return
		}
	}
}

// runConnection handles a single connection to the hub: connect, auth,
// fetch config, run heartbeat + notification loop. Returns on any error
// (caller retries with backoff). Resets backoff on successful auth.
func runConnection(ctx context.Context, cfg *ClientConfig, backoff *Backoff) error {
	// TLS connect.
	tlsConf := &tls.Config{
		InsecureSkipVerify: true, //nolint:gosec // hub uses self-signed certs; cert pinning planned
		MinVersion:         tls.VersionTLS13,
	}

	connectCtx, connectCancel := context.WithTimeout(ctx, connectTimeout)
	defer connectCancel()

	conn, err := (&tls.Dialer{Config: tlsConf}).DialContext(connectCtx, "tcp", cfg.Server)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer conn.Close() //nolint:errcheck // cleanup

	// Auth.
	if err := pluginipc.SendAuth(ctx, conn, cfg.Token, cfg.Name); err != nil {
		return fmt.Errorf("auth send: %w", err)
	}

	// Read auth response line (newline-terminated).
	if err := setAuthDeadline(ctx, conn); err != nil {
		return err
	}
	authLine, err := readLine(conn, maxAuthResponse)
	if err != nil {
		return fmt.Errorf("read auth response: %w", err)
	}
	_ = conn.SetReadDeadline(time.Time{}) // clear deadline

	// Parse response: #<id> <verb> [payload]
	_, verb, _, parseErr := rpc.ParseLine(authLine)
	if parseErr != nil || verb != "ok" {
		return fmt.Errorf("auth rejected")
	}

	// Auth succeeded -- reset backoff for fresh retry delays on next disconnect.
	backoff.Reset()

	// Wrap in MuxConn for multiplexed RPCs.
	rc := rpc.NewConn(conn, conn)
	mc := rpc.NewMuxConn(rc)
	defer mc.Close() //nolint:errcheck // cleanup

	logger().Info("connected to hub", "server", cfg.Server, "name", cfg.Name)

	// Fetch config.
	if err := fetchAndProcess(ctx, mc, cfg); err != nil {
		return err
	}

	// Start heartbeat.
	hbDone := make(chan struct{})
	hb := NewHeartbeat(heartbeatInterval, heartbeatMissed, func() {
		close(hbDone)
	})
	hb.Start()
	defer hb.Stop()

	// Ping sender: sends ping every heartbeatInterval.
	go func() {
		ticker := time.NewTicker(heartbeatInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if pingErr := SendPing(ctx, mc); pingErr != nil {
					return
				}
				hb.RecordPong() // Successful ping response = hub is alive.
			case <-ctx.Done():
				return
			case <-hbDone:
				return
			case <-mc.Done():
				return
			}
		}
	}()

	// Notification loop: handle hub-initiated RPCs.
	return notificationLoop(ctx, mc, cfg, hbDone)
}

// fetchAndProcess fetches config from hub, validates, caches, and signals reload.
func fetchAndProcess(ctx context.Context, mc *rpc.MuxConn, cfg *ClientConfig) error {
	resp, err := FetchConfig(ctx, mc, cfg.Version)
	if err != nil {
		return fmt.Errorf("fetch: %w", err)
	}

	if resp.Status == "current" || resp.Config == "" {
		logger().Debug("config is current", "version", cfg.Version)
		return nil
	}

	ack := cfg.Handler.ProcessConfig(resp)
	if ackErr := SendConfigAck(ctx, mc, ack); ackErr != nil {
		logger().Warn("send ack failed", "error", ackErr)
	}
	if ack.OK {
		cfg.Version = resp.Version
		logger().Info("config updated", "version", resp.Version)
		if cfg.OnReload != nil {
			cfg.OnReload()
		}
	} else {
		logger().Warn("config rejected", "error", ack.Error)
	}

	return nil
}

// notificationLoop reads hub-initiated RPCs until disconnect or shutdown.
func notificationLoop(ctx context.Context, mc *rpc.MuxConn, cfg *ClientConfig, hbDone <-chan struct{}) error {
	for {
		select {
		case req, ok := <-mc.Requests():
			if !ok {
				return fmt.Errorf("connection closed")
			}
			handleHubRequest(ctx, mc, req, cfg)

		case <-hbDone:
			return fmt.Errorf("heartbeat timeout")

		case <-mc.Done():
			return fmt.Errorf("connection closed")

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// handleHubRequest dispatches an inbound request from the hub.
func handleHubRequest(ctx context.Context, mc *rpc.MuxConn, req *rpc.Request, cfg *ClientConfig) {
	switch req.Method {
	case fleet.VerbConfigChanged:
		handleConfigChangedRequest(ctx, mc, req, cfg)

	case fleet.VerbPing:
		_ = mc.SendOK(ctx, req.ID)
	}
	// Unknown methods silently dropped for forward compatibility.
}

// handleConfigChangedRequest processes a config-changed notification.
func handleConfigChangedRequest(ctx context.Context, mc *rpc.MuxConn, req *rpc.Request, cfg *ClientConfig) {
	var n fleet.ConfigChanged
	if err := json.Unmarshal(req.Params, &n); err != nil {
		logger().Warn("bad config-changed payload", "error", err)
		_ = mc.SendError(ctx, req.ID, "bad payload")
		return
	}
	_ = mc.SendOK(ctx, req.ID)

	// Fetch the new config.
	if err := fetchAndProcess(ctx, mc, cfg); err != nil {
		logger().Warn("fetch after notification failed", "error", err)
	}
}

// FetchConfig sends a config-fetch RPC to the hub and returns the response.
// version is the client's current config hash (empty on first boot).
func FetchConfig(ctx context.Context, mc *rpc.MuxConn, version string) (fleet.ConfigFetchResponse, error) {
	req := fleet.ConfigFetchRequest{Version: version}

	result, err := mc.CallRPC(ctx, fleet.VerbConfigFetch, req)
	if err != nil {
		return fleet.ConfigFetchResponse{}, fmt.Errorf("config-fetch RPC: %w", err)
	}

	var resp fleet.ConfigFetchResponse
	if err := json.Unmarshal(result, &resp); err != nil {
		return fleet.ConfigFetchResponse{}, fmt.Errorf("unmarshal config-fetch response: %w", err)
	}

	return resp, nil
}

// SendConfigAck sends a config-ack RPC to the hub confirming receipt.
func SendConfigAck(ctx context.Context, mc *rpc.MuxConn, ack fleet.ConfigAck) error {
	_, err := mc.CallRPC(ctx, fleet.VerbConfigAck, ack)
	if err != nil {
		return fmt.Errorf("config-ack RPC: %w", err)
	}
	return nil
}

// SendPing sends a ping RPC to the hub for liveness.
func SendPing(ctx context.Context, mc *rpc.MuxConn) error {
	_, err := mc.CallRPC(ctx, fleet.VerbPing, struct{}{})
	if err != nil {
		return fmt.Errorf("ping RPC: %w", err)
	}
	return nil
}

// setAuthDeadline sets a read deadline for the auth response.
func setAuthDeadline(ctx context.Context, conn net.Conn) error {
	if deadline, ok := ctx.Deadline(); ok {
		return conn.SetReadDeadline(deadline)
	}
	return conn.SetReadDeadline(time.Now().Add(connectTimeout))
}

// readLine reads from conn byte-by-byte until newline or maxSize.
// Avoids buffered readers to keep the underlying conn clean for MuxConn.
func readLine(conn net.Conn, maxSize int) ([]byte, error) {
	buf := make([]byte, 0, 128)
	b := make([]byte, 1)
	for {
		n, err := conn.Read(b)
		if err != nil {
			return nil, err
		}
		if n == 0 {
			continue
		}
		if b[0] == '\n' {
			// Strip trailing \r for CRLF compatibility.
			if len(buf) > 0 && buf[len(buf)-1] == '\r' {
				buf = buf[:len(buf)-1]
			}
			return buf, nil
		}
		buf = append(buf, b[0])
		if len(buf) >= maxSize {
			return nil, fmt.Errorf("auth response exceeds %d bytes", maxSize)
		}
	}
}
