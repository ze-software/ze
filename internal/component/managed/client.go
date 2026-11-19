// Design: docs/architecture/fleet-config.md — managed client connection lifecycle
// Related: handler.go — processes config responses from hub
// Related: reconnect.go — backoff for connection retries
// Related: heartbeat.go — liveness detection
// Related: baseline.go — BaselineAccess and Diverged, which decide a hold

package managed

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/ze-software/ze/internal/component/config"
	pluginipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/network"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/pkg/fleet"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

var (
	errAuthRejected     = errors.New("auth rejected")
	errConnectionClosed = errors.New("connection closed")
	errHeartbeatTimeout = errors.New("heartbeat timeout")
	// errEndpointChanged ends a connection whose applied config names other
	// hub settings than the ones it was made with, so the next one uses them.
	errEndpointChanged = errors.New("the applied config changed the hub connection")
	// errHubClientRemoved ends the client: the applied config has no hub
	// client block, which is a config that asks for no hub connection.
	errHubClientRemoved = errors.New("the applied config has no plugin hub client block")
)

// ErrNoHubClient is what ClientConfig.Endpoint answers for a config that
// declares no hub client block.
var ErrNoHubClient = errors.New("no plugin hub client block")

const (
	heartbeatInterval = 30 * time.Second
	heartbeatMissed   = 3
	// connectTimeoutDefault bounds the dial and the TLS handshake when
	// ClientConfig.ConnectTimeout is zero.
	connectTimeoutDefault = 5 * time.Second
	maxAuthResponse       = 512
)

var logger = slogutil.LazyLogger("hub.managed")

// ClientConfig holds the configuration for a managed client connection.
type ClientConfig struct {
	Name   string // Client identity (from hub client block name)
	Server string // Hub address (host:port)
	Token  string // Auth token
	// Version is the config version hash a fetch sends while Baseline is nil
	// (empty on first boot). With Baseline set a fetch sends the hash of the
	// active configuration, read from the store at fetch time.
	Version     string
	TLSInsecure bool // Skip TLS certificate verification (INSECURE: only for testing or self-signed hubs with explicit opt-in)
	// CA names the pki ca entry holding the hub's certificate authority root
	// (plugin/hub/client/ca). It authenticates a hub whose certificate no
	// public CA issued, which is what a private fleet hub presents, and it
	// keeps authenticating it after the hub reissues its leaf. Empty means the
	// trust anchor comes from the system CA pool. See clientTLSConfig.
	CA string
	// HubRoot is the hub's certificate authority root the store holds, from
	// ze init --hub-ca or kept on the first connection. It anchors the hub
	// when CA is empty. nil means the store holds none.
	HubRoot       *x509.Certificate
	SourceAddress string // Optional source IP for outbound connection
	// ConnectTimeout bounds the dial and the TLS handshake of each connection.
	// Zero means connectTimeoutDefault.
	ConnectTimeout time.Duration
	Handler        *Handler
	OnCommit       func([]byte) error // Called to transactionally commit fetched config
	CheckManaged   func() bool        // Returns false when meta/instance/managed is disabled; nil = always managed
	// Baseline reads the hash of the configuration the device runs and the
	// baseline (the hash of the last configuration its hub gave it), and
	// stores the baseline. With it the client holds: it applies nothing the
	// hub sends while the two differ (fetchAgainstBaseline). nil means the
	// client compares nothing and applies every configuration the hub returns.
	Baseline *BaselineAccess
	// Endpoint reads the hub connection a config declares. The client calls it
	// with each config it has applied, so a push that changes the hub address,
	// the secret, the certificate authority, the source address or the client
	// name takes effect on the running client and not only at the next start.
	// It answers ErrNoHubClient for a config with no hub client block. nil
	// means the connection settings are fixed for the life of the client.
	Endpoint func(config []byte) (Endpoint, error)
	// OnConnection is told true once the hub accepted this client's
	// authentication, and false when that connection ended. OnHold is told,
	// after each fetch the hub answered, whether the client applied nothing
	// because the configuration the device runs differs from its baseline or
	// could not be compared with it. The daemon's owner of this client reports
	// both in `show fleet status`. The client calls them from its own goroutine
	// and MUST NOT be blocked by them. nil reports nothing.
	OnConnection func(connected bool)
	OnHold       func(holding bool)
}

// reportConnection tells the owner of this client whether it holds a hub
// connection (ClientConfig.OnConnection).
func (cfg *ClientConfig) reportConnection(connected bool) {
	if cfg.OnConnection != nil {
		cfg.OnConnection(connected)
	}
}

// reportHold tells the owner of this client whether its last fetch was held
// (ClientConfig.OnHold).
func (cfg *ClientConfig) reportHold(holding bool) {
	if cfg.OnHold != nil {
		cfg.OnHold(holding)
	}
}

// Endpoint is the hub connection a config declares: who the client is and
// where and how it reaches its hub.
type Endpoint struct {
	Name          string // Client identity (hub client block name).
	Server        string // Hub address (host:port).
	Token         string // Auth token.
	CA            string // pki ca entry holding the hub's root; empty means the system pool.
	SourceAddress string // Optional source IP for the outbound connection.
}

// DeclaredEndpoint reads the hub connection the config in data declares: its
// first hub-level client block, as written. It answers ErrNoHubClient for a
// config with no such block. The daemon's start path lays its troubleshooting
// overrides on top (cmd/ze hubClientEndpoint); `show fleet status` reports the
// block itself.
func DeclaredEndpoint(data []byte) (Endpoint, error) {
	// The hub client block is under `plugin`, which the first pass reads on
	// its own. A full parse here would refuse a config that holds a section
	// of an external plugin, whose schema the daemon learns only at boot, and
	// would build values that keep the module set before boot publishes it.
	firstPass, err := config.FirstPass(string(data), "", nil)
	if err != nil {
		return Endpoint{}, err
	}
	if len(firstPass.Hub.Clients) == 0 {
		return Endpoint{}, ErrNoHubClient
	}
	declared := firstPass.Hub.Clients[0]
	return Endpoint{
		Name:          declared.Name,
		Server:        declared.Address(),
		Token:         declared.Secret,
		CA:            declared.CA,
		SourceAddress: declared.SourceAddress,
	}, nil
}

// endpoint returns the hub connection cfg holds now.
func (cfg *ClientConfig) endpoint() Endpoint {
	return Endpoint{
		Name:          cfg.Name,
		Server:        cfg.Server,
		Token:         cfg.Token,
		CA:            cfg.CA,
		SourceAddress: cfg.SourceAddress,
	}
}

// adoptEndpoint reads the hub connection the applied config declares and
// takes it for the next connection. It returns errEndpointChanged when that
// differs from the connection in use, errHubClientRemoved when the config
// declares none, and nil when nothing changed or cfg.Endpoint is nil. A
// config whose hub block cannot be read leaves the settings as they are: the
// daemon already applied that config, and the connection in use still works.
func (cfg *ClientConfig) adoptEndpoint(applied []byte) error {
	if cfg.Endpoint == nil {
		return nil
	}
	next, err := cfg.Endpoint(applied)
	if errors.Is(err, ErrNoHubClient) {
		return errHubClientRemoved
	}
	if err != nil {
		logger().Warn("the applied config's hub client block could not be read, "+
			"so this client keeps the hub connection it has; check the block with: ze config validate",
			"code", diagnostic.CodeConfigHubInvalid, "error", err)
		return nil
	}
	if next == cfg.endpoint() {
		return nil
	}
	cfg.Name = next.Name
	cfg.Server = next.Server
	cfg.Token = next.Token
	cfg.CA = next.CA
	cfg.SourceAddress = next.SourceAddress
	return errEndpointChanged
}

// connectTimeout returns the bound on the dial and the TLS handshake.
func (cfg *ClientConfig) connectTimeout() time.Duration {
	if cfg.ConnectTimeout > 0 {
		return cfg.ConnectTimeout
	}
	return connectTimeoutDefault
}

// RunManagedClient connects to the hub and maintains the connection with
// reconnect and heartbeat. Blocks until ctx is canceled. This is a
// long-lived goroutine -- one per managed client instance.
// A caller that starts a goroutine MUST cancel ctx and wait for this function
// to return before releasing resources borrowed by CheckManaged, OnCommit or
// Baseline.
func RunManagedClient(ctx context.Context, cfg ClientConfig) {
	backoff := newBackoff(1*time.Second, 60*time.Second, 0.1)

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
		if errors.Is(err, errHubClientRemoved) {
			logger().Warn("the config the hub sent has no plugin hub client block, so this node no longer follows its hub",
				"code", diagnostic.CodeDoctorHubClientMissing, "name", cfg.Name, "fix", hubClientMissingFix)
			return
		}
		if errors.Is(err, errEndpointChanged) {
			// Each turn here follows a config the hub sent and the daemon
			// applied, so the hub's own writes bound this path.
			logger().Info("the applied config changed the hub connection, reconnecting with it",
				"server", cfg.Server, "name", cfg.Name)
			continue
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

// serverNameFromAddr derives the TLS ServerName from a "host:port" address.
// This must be set explicitly: net.Dialer + tls.Client() does NOT infer the
// ServerName from the dial address the way tls.Dialer.DialContext() does. With
// an empty ServerName, certificate verification skips the hostname check
// (crypto/x509 Verify only runs VerifyHostname when DNSName is non-empty), so
// any CA-signed certificate would be silently accepted. Falls back to the raw
// input when it has no port separator.
func serverNameFromAddr(server string) string {
	if host, _, err := net.SplitHostPort(server); err == nil {
		return host
	}
	return server
}

// runConnection handles a single connection to the hub: connect, auth,
// fetch config, run heartbeat + notification loop. Returns on any error
// (caller retries with backoff). Resets backoff on successful auth.
func runConnection(ctx context.Context, cfg *ClientConfig, backoff *Backoff) error {
	tlsConf, err := clientTLSConfig(cfg)
	if err != nil {
		return err
	}

	connectCtx, connectCancel := context.WithTimeout(ctx, cfg.connectTimeout())
	defer connectCancel()

	dialer := &network.RealDialer{}
	if err := dialer.SetSourceAddress(cfg.SourceAddress); err != nil {
		return err
	}
	rawConn, err := dialer.DialContext(connectCtx, "tcp", cfg.Server)
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}

	tlsConn := tls.Client(rawConn, tlsConf)
	if err := tlsConn.HandshakeContext(connectCtx); err != nil {
		rawConn.Close() //nolint:errcheck // cleanup on handshake failure
		return fmt.Errorf("tls handshake: %w", err)
	}
	conn := net.Conn(tlsConn)
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
		return errAuthRejected
	}

	// Auth succeeded -- reset backoff for fresh retry delays on next disconnect.
	backoff.Reset()
	cfg.reportConnection(true)
	defer cfg.reportConnection(false)

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
				if pingErr := sendPing(ctx, mc); pingErr != nil {
					return
				}
				hb.recordPong() // Successful ping response = hub is alive.
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

// fetchStatusCurrent is the status of a config-fetch answer that says the hub
// holds the configuration whose hash the client sent (pkg/fleet
// ConfigFetchResponse).
const fetchStatusCurrent = "current"

// holdRefusalPrefix opens the config-ack error of a device that holds. The
// hub stores the text as that client's last refusal and parses nothing in it.
const holdRefusalPrefix = "holding: the configuration this device runs differs from the last one this hub sent"

// fetchAndProcess fetches config from hub, validates, commits, and ACKs the result.
// It returns errEndpointChanged or errHubClientRemoved when the config it
// applied changes or removes the hub connection: the ACK is already sent, and
// the caller MUST end the connection. A device that holds, or that cannot
// compare its configuration with its baseline, applies nothing and answers
// nil: the connection is kept.
func fetchAndProcess(ctx context.Context, mc *rpc.MuxConn, cfg *ClientConfig) error {
	if cfg.Baseline != nil {
		return fetchAgainstBaseline(ctx, mc, cfg, cfg.Baseline)
	}
	resp, err := FetchConfig(ctx, mc, cfg.Version)
	if err != nil {
		return fmt.Errorf("fetch: %w", err)
	}
	if resp.Status == fetchStatusCurrent {
		logger().Debug("config is current", "version", cfg.Version)
		return nil
	}
	if resp.Config == "" {
		logger().Debug("config is current", "version", cfg.Version)
		return nil
	}
	return applyFetched(ctx, mc, cfg, resp)
}

// fetchAgainstBaseline is fetchAndProcess for a client that holds: it sends
// the hash of the configuration the device runs, read now, and applies what
// the hub returns only while that configuration is the last one the hub gave
// (Diverged). The hub's "current" answer is the one thing that ends a hold:
// it proves hub and device hold the same configuration, so that hash becomes
// the baseline, and the device acknowledges it so the hub drops the refusal
// it recorded while the device held.
func fetchAgainstBaseline(ctx context.Context, mc *rpc.MuxConn, cfg *ClientConfig, access *BaselineAccess) error {
	active, baseline, err := access.read()
	if err != nil {
		refuseUncompared(ctx, mc, cfg, err)
		cfg.reportHold(true)
		return nil
	}
	resp, err := FetchConfig(ctx, mc, active.Hash())
	if err != nil {
		return fmt.Errorf("fetch: %w", err)
	}
	if resp.Status == fetchStatusCurrent {
		settleBaseline(cfg, access, active, baseline)
		if Diverged(active, baseline) {
			// The hub last heard a refusal from this device. Nothing was
			// applied, so no ack would otherwise follow, and the hub would go
			// on listing a device that holds after its operator resolved it.
			sendAck(ctx, mc, fleet.ConfigAck{Version: active.Hash(), OK: true})
		}
		cfg.reportHold(false)
		return nil
	}
	if resp.Config == "" {
		logger().Debug("the hub sent no config", "version", active.Hash())
		return nil
	}
	if Diverged(active, baseline) {
		holdFetched(ctx, mc, cfg, resp, active, baseline)
		cfg.reportHold(true)
		return nil
	}
	cfg.reportHold(false)
	return applyFetched(ctx, mc, cfg, resp)
}

// settleBaseline records active as the baseline after the hub answered
// "current" for it. A baseline that already equals it is left alone, and so is
// a device with no stored configuration, which sent no hash the hub could
// have agreed with. A baseline that cannot be stored is logged and the
// connection is kept: nothing was applied, and the next fetch settles again.
func settleBaseline(cfg *ClientConfig, access *BaselineAccess, active, baseline ConfigVersion) {
	if !active.Recorded() {
		return
	}
	if active == baseline {
		logger().Debug("config is current", "version", active.Hash())
		return
	}
	if err := access.store(active); err != nil {
		logger().Warn("the hub holds the configuration this device runs, and the device could not record that as its baseline, "+
			"so it holds at the next hub change if the two still differ then; check the store with: "+fixCheckStore,
			"code", diagnostic.CodeDoctorManagedDiverged, "name", cfg.Name, "version", active.Hash(), "error", err)
		return
	}
	logger().Info("the hub holds the configuration this device runs, recorded as the baseline",
		"name", cfg.Name, "version", active.Hash())
}

// holdFetched refuses the configuration the hub returned because the device
// runs one that differs from its baseline. The hub's configuration is neither
// decoded, validated nor committed. The hub is told in a config-ack that
// carries the two hashes and no configuration content, and the device logs
// one warning. The caller MUST keep the connection: ending it would make the
// device reconnect and hold again about once a second.
func holdFetched(ctx context.Context, mc *rpc.MuxConn, cfg *ClientConfig, resp fleet.ConfigFetchResponse, active, baseline ConfigVersion) {
	sendAck(ctx, mc, fleet.ConfigAck{
		Version: resp.Version,
		OK:      false,
		Error: holdRefusalPrefix + " (active " + active.Hash() + ", baseline " + baseline.Hash() + "); " +
			"the device applied nothing and stays connected, and ze doctor on the device names the two ways to resolve it",
	})
	logger().Warn("this device holds: the configuration it runs differs from the last one its hub sent, so it applied nothing from the hub",
		"code", diagnostic.CodeDoctorManagedDiverged, "name", cfg.Name,
		"active", active.Hash(), "baseline", baseline.Hash(), "offered", resp.Version, "fix", managedDivergedFix(cfg.Name))
}

// refuseUncompared is a fetch the client did not make because the store could
// not say what the device runs or what its baseline is. Applying blindly could
// overwrite a local change, so nothing is fetched or applied; holding in
// silence would hide it, so the hub is told in a config-ack and the device
// logs one warning. The caller MUST keep the connection: the next fetch reads
// the store again.
func refuseUncompared(ctx context.Context, mc *rpc.MuxConn, cfg *ClientConfig, cause error) {
	sendAck(ctx, mc, fleet.ConfigAck{
		OK: false,
		Error: "this device could not compare the configuration it runs with the last one this hub sent, " +
			"so it fetched and applied nothing: " + cause.Error(),
	})
	logger().Warn("this device could not compare the configuration it runs with the last one its hub sent, "+
		"so it fetched and applied nothing from the hub; check the store with: "+fixCheckStore,
		"code", diagnostic.CodeDoctorManagedDiverged, "name", cfg.Name, "error", cause)
}

// sendAck sends ack to the hub. An ack that cannot be sent is logged: the
// connection that lost it ends on its own, and the next one fetches again.
func sendAck(ctx context.Context, mc *rpc.MuxConn, ack fleet.ConfigAck) {
	if ackErr := SendConfigAck(ctx, mc, ack); ackErr != nil {
		logger().Warn("send ack failed", "code", codeHubUnreachable, "error", ackErr)
	}
}

// applyFetched validates, commits and acknowledges a configuration the hub
// returned. It returns errEndpointChanged or errHubClientRemoved as
// fetchAndProcess documents.
func applyFetched(ctx context.Context, mc *rpc.MuxConn, cfg *ClientConfig, resp fleet.ConfigFetchResponse) error {
	data, ack := cfg.Handler.ValidateConfig(resp)
	if ack.OK {
		if cfg.OnCommit == nil {
			ack.OK = false
			ack.Error = "commit not ready"
		} else if err := cfg.OnCommit(data); err != nil {
			ack.OK = false
			ack.Error = fmt.Sprintf("commit config: %v", err)
		}
	}
	sendAck(ctx, mc, ack)
	if !ack.OK {
		logger().Warn("config rejected", "error", ack.Error)
		return nil
	}
	cfg.Version = resp.Version
	logger().Info("config updated", "version", resp.Version)
	return cfg.adoptEndpoint(data)
}

// notificationLoop reads hub-initiated RPCs until disconnect or shutdown.
func notificationLoop(ctx context.Context, mc *rpc.MuxConn, cfg *ClientConfig, hbDone <-chan struct{}) error {
	for {
		select {
		case req, ok := <-mc.Requests():
			if !ok {
				return errConnectionClosed
			}
			if err := handleHubRequest(ctx, mc, req, cfg); err != nil {
				return err
			}

		case <-hbDone:
			return errHeartbeatTimeout

		case <-mc.Done():
			return errConnectionClosed

		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// handleHubRequest dispatches an inbound request from the hub. A non-nil
// error means the connection MUST end (handleConfigChangedRequest).
func handleHubRequest(ctx context.Context, mc *rpc.MuxConn, req *rpc.Request, cfg *ClientConfig) error {
	switch req.Method {
	case fleet.VerbConfigChanged:
		return handleConfigChangedRequest(ctx, mc, req, cfg)

	case fleet.VerbPing:
		_ = mc.SendOK(ctx, req.ID)
	}
	// Unknown methods silently dropped for forward compatibility.
	return nil
}

// handleConfigChangedRequest processes a config-changed notification. It
// returns errEndpointChanged or errHubClientRemoved when the config it
// applied changes or removes the hub connection, and the caller MUST then end
// the connection. A fetch that fails is logged and the connection is kept: the
// next notification or connection fetches again.
func handleConfigChangedRequest(ctx context.Context, mc *rpc.MuxConn, req *rpc.Request, cfg *ClientConfig) error {
	var n fleet.ConfigChanged
	if err := json.Unmarshal(req.Params, &n); err != nil {
		logger().Warn("bad config-changed payload", "error", err)
		_ = mc.SendError(ctx, req.ID, "bad payload")
		return nil
	}
	_ = mc.SendOK(ctx, req.ID)

	// Fetch the new config.
	err := fetchAndProcess(ctx, mc, cfg)
	if errors.Is(err, errEndpointChanged) {
		return err
	}
	if errors.Is(err, errHubClientRemoved) {
		return err
	}
	if err != nil {
		logger().Warn("fetch after notification failed", "error", err)
	}
	return nil
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

// sendPing sends a ping RPC to the hub for liveness.
func sendPing(ctx context.Context, mc *rpc.MuxConn) error {
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
	return conn.SetReadDeadline(time.Now().Add(connectTimeoutDefault))
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
