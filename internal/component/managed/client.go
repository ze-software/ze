// Design: docs/architecture/fleet-config.md — managed client connection lifecycle
// Related: handler.go — processes config responses from hub
// Related: reconnect.go — backoff for connection retries
// Related: heartbeat.go — liveness detection
// Related: baseline.go — BaselineAccess and Diverged, which decide a hold
// Related: report.go — per-session fleet inventory and health reporter
// Related: sample.go — per-session fleet metric sample sender

package managed

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/ze-software/ze/internal/component/config"
	pluginipc "github.com/ze-software/ze/internal/component/plugin/ipc"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/metrics"
	"github.com/ze-software/ze/internal/core/network"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/report"
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
	errHubClientRemoved = errors.New("the applied config has no bus hub client block")
)

// ErrNoHubClient is what ClientConfig.Endpoint answers for a config that
// declares no hub client block.
var ErrNoHubClient = errors.New("no bus hub client block")

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
	Name        string // Client identity (from hub client block name)
	Server      string // Hub address (host:port)
	Token       string // Auth token
	TLSInsecure bool   // Skip TLS certificate verification (INSECURE: only for testing or self-signed hubs with explicit opt-in)
	// CA names the pki ca entry holding the hub's certificate authority root
	// (bus/hub/client/ca). It authenticates a hub whose certificate no
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
	// CommitOver transactionally commits data, the validated configuration
	// the hub returned, by an ordinary push or a revert its operator decided
	// (applyFetched). Before it stages anything it MUST hand check the
	// configuration active then, read under the lock its stage takes, and
	// refuse with check's error, as is, when check refuses it: check refuses a
	// configuration other than the one the client read before its fetch
	// (activeCheck), so a local commit that landed during the fetch is never
	// overwritten. nil refuses every configuration as not ready.
	CommitOver   func(data []byte, check func(active []byte) error) error
	CheckManaged func() bool // Returns false when meta/instance/managed is disabled; nil = always managed
	// Baseline reads the hash of the configuration the device runs and the
	// baseline (the hash of the last configuration its hub gave it), and
	// stores the baseline. With it the client holds: it applies nothing the
	// hub sends while the two differ (fetchAgainstBaseline). It is required:
	// RunManagedClient refuses to run a client without it, because such a
	// client could compare nothing and would apply every configuration the hub
	// returns over whatever the device runs.
	Baseline *BaselineAccess
	// Chain reads and writes the persisted SHA-1 configuration hash chain state
	// under meta/managed/chain-state. Optional: nil keeps chain state in memory
	// only for unit tests that do not wire persistence.
	Chain *ChainAccess
	// Metrics is the optional Prometheus metrics registry used for client chain
	// mismatch and resync counters. Nil disables metrics cleanly.
	Metrics metrics.Registry
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
	// could not be compared with it. OnHubAnswer is told, after each fetch
	// the hub answered, what that answer said of the hub's copy of a hold and
	// of a revert (HubAnswer). OnChainState is told whenever the runtime SHA-1
	// hash chain status changes. The daemon's owner of this client reports
	// these in `show fleet status` and runtime doctor checks. The client calls
	// them from its own goroutine and MUST NOT be blocked by them. nil reports
	// nothing.
	OnConnection     func(connected bool)
	OnSessionFailure func(failure HubSessionFailure)
	OnHold           func(holding bool)
	OnHubAnswer      func(answer HubAnswer)
	OnChainState     func(status ChainStatus)
	// SampleCollector collects device-local metric readings for periodic
	// VerbSampleReport pushes when the hub advertises a positive sample
	// interval. Nil leaves sampling idle.
	SampleCollector SampleCollectorFunc

	// held is what this run of the client remembers of the copies it sent
	// its hub (heldUploads). It is state, not configuration: a caller leaves
	// it unset, and RunManagedClient's own copy of the configuration starts
	// with none.
	held heldUploads
	// fetchTimeout bounds each config-fetch of this client (fetchBound). Zero
	// means configFetchTimeout, which every production client uses; a test
	// shortens it, as ManagedServer.pushTimeout, so no test waits 30 seconds.
	fetchTimeout time.Duration
	// pingTimeout bounds each client-initiated VerbPing round-trip (pingBound).
	// Zero means pingTimeout (10s); a test shortens it.
	pingTimeout time.Duration
	// pingInterval overrides heartbeatInterval for the ping ticker in tests
	// when positive; zero uses heartbeatInterval (30s).
	pingInterval time.Duration
	// chainState holds the concurrency-safe runtime hash-chain snapshot, latch,
	// and metrics handles for this client run.
	chainState *chainRuntime
	// reportTimeout bounds each fleet report collection and RPC round-trip
	// (reportCallTimeout). Zero means defaultReportCallTimeout; a test
	// shortens it so no test waits 30 seconds.
	reportTimeout time.Duration
	// reportProducersFn overrides fleet.ReportProducers for this client when
	// non-nil. Tests set it to supply deterministic short-interval producers
	// without mutating the process-wide producer registry.
	reportProducersFn func() []fleet.ReportProducer
	// sampleIntervalUnit scales SampleIntervalSeconds into a time.Duration.
	// Zero means time.Second; tests shorten it so interval tests run quickly.
	sampleIntervalUnit time.Duration
	// sampleTimeout bounds one sample collection and RPC round-trip when
	// positive; zero derives the bound from the negotiated interval.
	sampleTimeout time.Duration
	// sampleNow overrides time.Now for sample collection timestamps when non-nil.
	sampleNow func() time.Time
}

// heldUploadTimeout bounds one config-held round-trip, the write of the copy
// included: the context deadline is also the write deadline
// (pkg/plugin/rpc writeFrame). The upload runs on the notification loop, which
// answers no config-changed while it runs, so it MUST end before the hub gives
// up on a config-changed it sent meanwhile (pluginserver.ManagedPushTimeout)
// and closes the connection. Every bound of a loop round-trip stays below that
// push bound except the fetch's (configFetchTimeout), which a large answer
// needs.
const heldUploadTimeout = 5 * time.Second

// configAckTimeout bounds one config-ack round-trip, the write included (the
// context deadline is also the write deadline). The hub answers an ack at
// once, so only a hub that stalls reaches it. Without it a hub that keeps the
// connection and its pings alive but never answers an ack would hold the
// notification loop for ever. The loop sends the ack of a hold after its
// upload, so the two bounds together MUST stay below the hub's push bound
// (pluginserver.ManagedPushTimeout), for the reason heldUploadTimeout gives.
// The fetch before them is the one exception (configFetchTimeout).
const configAckTimeout = 3 * time.Second

// configFetchTimeout bounds one config-fetch round-trip, the write included
// (the context deadline is also the write deadline), at connect, on the
// notification loop, on the fetch made again over a local commit, and at
// first boot. Without it a hub that keeps the connection open but never
// answers a fetch would hold the device there for ever. It is the one loop
// bound NOT below the hub's push bound (pluginserver.ManagedPushTimeout, 10
// seconds), by design: the answer can carry a configuration of up to
// rpc.MaxMessageSize (16 MiB), and 30 seconds carry that on a link of about
// 4.5 Mbit/s, where any bound below the push bound would cut every large
// configuration on an ordinary link. A fetch on the loop that runs past the
// push bound costs the hub's config-changed sent meanwhile: the hub closes the
// connection, and the device fetches again when it reconnects.
const configFetchTimeout = 30 * time.Second

// heldUploads is what one run of a client remembers of the copies it sent its
// hub. Only the connection's goroutine reads and writes it (fetchAndProcess
// runs there alone), so it holds no lock. Both of its bounds are one entry.
type heldUploads struct {
	// failed is the version whose last upload failed in this run, and
	// failure says why, for the hold ack. No copy of that version is sent
	// again in the run: the same bytes would fail the same way, and each try
	// holds the notification loop. failed is not recorded when no upload
	// failed, and a newer version replaces it.
	failed  ConfigVersion
	failure string
	// refusedOn is the connection whose hub answered config-held as a verb
	// it does not take, a hub built before it. No copy is sent on that
	// connection again; the next connection tries once more. nil when no hub
	// refused the verb.
	refusedOn *rpc.MuxConn
}

// reportConnection tells the owner of this client whether it holds a hub
// connection (ClientConfig.OnConnection).
func (cfg *ClientConfig) reportConnection(connected bool) {
	if cfg.OnConnection != nil {
		cfg.OnConnection(connected)
	}
}

// reportSessionFailure tells the owner of this client why an attempt failed
// before the hub session opened (ClientConfig.OnSessionFailure).
func (cfg *ClientConfig) reportSessionFailure(step HubSessionStep, err error) error {
	failure := classifySessionFailure(cfg, step, err, time.Now())
	if cfg.OnSessionFailure != nil {
		cfg.OnSessionFailure(failure.clone())
	}
	return sessionAttemptError{failure: failure, err: err}
}

// reportHold tells the owner of this client whether its last fetch was held
// (ClientConfig.OnHold).
func (cfg *ClientConfig) reportHold(holding bool) {
	if cfg.OnHold != nil {
		cfg.OnHold(holding)
	}
}

// reportHubAnswer tells the owner of this client what the hub's last fetch
// answer said of its copy of a hold (ClientConfig.OnHubAnswer).
func (cfg *ClientConfig) reportHubAnswer(answer HubAnswer) {
	if cfg.OnHubAnswer != nil {
		cfg.OnHubAnswer(answer)
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

// fetchBound returns the bound on one config-fetch round-trip.
func (cfg *ClientConfig) fetchBound() time.Duration {
	if cfg.fetchTimeout > 0 {
		return cfg.fetchTimeout
	}
	return configFetchTimeout
}

// reportCallTimeout returns the bound on one fleet report collection and RPC
// round-trip.
func (cfg *ClientConfig) reportCallTimeout() time.Duration {
	if cfg.reportTimeout > 0 {
		return cfg.reportTimeout
	}
	return defaultReportCallTimeout
}

// configuredReportProducers returns the fleet report producers scheduled for
// each authenticated session of this client.
func (cfg *ClientConfig) configuredReportProducers() []fleet.ReportProducer {
	if cfg.reportProducersFn != nil {
		return cfg.reportProducersFn()
	}
	return fleet.ReportProducers()
}

// noBaselineDefect is the message of RunManagedClient's refusal of a cfg with
// no Baseline: what the client does not do, why, and that the reader reports
// it, since nothing an operator configures causes it.
const noBaselineDefect = "this hub client was built without access to the device's baseline, " +
	"so it cannot tell a local change from the hub's configuration and does not connect to its hub. " +
	"This is a defect in ze, not in the configuration: report it with this message"

// RunManagedClient connects to the hub and maintains the connection with
// reconnect and heartbeat. Blocks until ctx is canceled. This is a
// long-lived goroutine -- one per managed client instance.
// A caller that starts a goroutine MUST cancel ctx and wait for this function
// to return before releasing resources borrowed by CheckManaged, CommitOver or
// Baseline. A cfg without Baseline MUST NOT run: it is refused at once with
// one Error line and one report-bus error under doctor-internal-defect, before
// any connection. Only a defect in the code that built cfg leaves Baseline
// nil, so no configuration change and no doctor fix clears it.
func RunManagedClient(ctx context.Context, cfg ClientConfig) {
	if cfg.Baseline == nil {
		logger().Error(noBaselineDefect, "code", diagnostic.CodeDoctorInternalDefect, "name", cfg.Name)
		report.RaiseErrorFix(doctorComponent, diagnostic.CodeDoctorInternalDefect, "hub client "+cfg.Name,
			noBaselineDefect, "ze show errors")
		return
	}
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
			logger().Warn("the config the hub sent has no bus hub client block, so this node no longer follows its hub",
				"code", codeDoctorHubClientMissing, "name", cfg.Name, "fix", hubClientMissingFix)
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
		if errors.Is(err, errActiveChangedTwice) {
			// Logged at Info where it was decided (logLocalCommitTwice): the
			// connection was not lost, and the reconnect is quiet.
			if !waitBackoff(ctx, delay) {
				return
			}
			continue
		}
		logReconnectWarning(delay, err)

		if !waitBackoff(ctx, delay) {
			return
		}
	}
}

// logReconnectWarning logs the reconnect backoff warning with a static doctor
// code constant matching the failed session attempt's class.
func logReconnectWarning(delay time.Duration, err error) {
	rounded := delay.Round(time.Millisecond)
	attempt, ok := errors.AsType[sessionAttemptError](err)
	if !ok {
		logger().Warn("connection lost, reconnecting", "code", codeHubUnreachable, "delay", rounded, "error", err)
		return
	}
	code, fix := attempt.failure.ReconnectMeta(time.Now())
	switch code {
	case CodeDoctorHubCertificateUnverified:
		logger().Warn("connection lost, reconnecting", "code", CodeDoctorHubCertificateUnverified, "fix", fix, "delay", rounded, "error", err)
	case CodeDoctorHubHandshakeFailed:
		logger().Warn("connection lost, reconnecting", "code", CodeDoctorHubHandshakeFailed, "fix", fix, "delay", rounded, "error", err)
	case CodeDoctorHubAuthRejected:
		logger().Warn("connection lost, reconnecting", "code", CodeDoctorHubAuthRejected, "fix", fix, "delay", rounded, "error", err)
	default:
		logger().Warn("connection lost, reconnecting", "code", codeHubUnreachable, "fix", fix, "delay", rounded, "error", err)
	}
}

// waitBackoff waits delay before RunManagedClient dials again, and answers
// false when ctx ended first.
func waitBackoff(ctx context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
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
		return cfg.reportSessionFailure(HubSessionStepAnchor, err)
	}

	connectCtx, connectCancel := context.WithTimeout(ctx, cfg.connectTimeout())
	defer connectCancel()

	dialer := &network.RealDialer{}
	if err := dialer.SetSourceAddress(cfg.SourceAddress); err != nil {
		return cfg.reportSessionFailure(HubSessionStepConnect, err)
	}
	rawConn, err := dialer.DialContext(connectCtx, "tcp", cfg.Server)
	if err != nil {
		return cfg.reportSessionFailure(HubSessionStepConnect, fmt.Errorf("connect: %w", err))
	}

	tlsConn := tls.Client(rawConn, tlsConf)
	if err := tlsConn.HandshakeContext(connectCtx); err != nil {
		rawConn.Close() //nolint:errcheck // cleanup on handshake failure
		return cfg.reportSessionFailure(HubSessionStepHandshake, fmt.Errorf("tls handshake: %w", err))
	}
	conn := net.Conn(tlsConn)
	defer conn.Close() //nolint:errcheck // cleanup

	// Auth.
	if err := pluginipc.SendAuth(ctx, conn, cfg.Token, cfg.Name); err != nil {
		return cfg.reportSessionFailure(HubSessionStepAuth, fmt.Errorf("auth send: %w", err))
	}

	// Read auth response line (newline-terminated).
	if err := setAuthDeadline(ctx, conn); err != nil {
		return cfg.reportSessionFailure(HubSessionStepAuth, err)
	}
	authLine, err := readLine(conn, maxAuthResponse)
	if err != nil {
		return cfg.reportSessionFailure(HubSessionStepAuth, fmt.Errorf("read auth response: %w", err))
	}
	_ = conn.SetReadDeadline(time.Time{}) // clear deadline

	// Parse response: #<id> <verb> [payload]
	_, verb, _, parseErr := rpc.ParseLine(authLine)
	if parseErr != nil || verb != "ok" {
		return cfg.reportSessionFailure(HubSessionStepAuth, errAuthRejected)
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

	// Load persisted chain state and reset the per-session failed-sync latch
	// before any fetch or worker starts on this connection (AC-11, R-5).
	cfg.initChainRuntime()
	driftCh := make(chan string, 1)

	// Fetch config. A local commit that landed during both fetches
	// (errActiveChangedTwice) is logged here, once, and ends the connection:
	// the reconnect's fetch decides again over what the device then runs, so
	// a device that still differs holds, uploads its copy and acks holding.
	outcome, err := fetchAndProcessOutcomeWithDrift(ctx, mc, cfg, driftCh)
	if err != nil {
		if errors.Is(err, errActiveChangedTwice) {
			logLocalCommitTwice(cfg, err)
		}
		return err
	}

	// Start periodic inventory and health reporting for this authenticated
	// session. The defer runs before mc.Close so the worker is canceled and
	// joined before the connection ends or the next session starts.
	stopReporter := startSessionReporter(ctx, mc, cfg)
	defer stopReporter()

	// Create the per-session metric sampler and activate it only when the hub
	// advertised a positive sample interval. The defer runs before mc.Close so
	// the sampler worker is canceled and joined before the connection ends.
	sampler := newSessionSampler(ctx, mc, cfg)
	defer sampler.Stop()
	if outcome.hasHubReply {
		sampler.UpdateInterval(outcome.sampleIntervalSeconds)
	}

	// Start heartbeat.
	hbDone := make(chan struct{})
	hb := NewHeartbeat(heartbeatInterval, heartbeatMissed, func() {
		close(hbDone)
	})
	hb.Start()
	defer hb.Stop()

	// Joined ping sender: sends VerbPing every heartbeatInterval carrying our
	// current chain head with pingTimeout and signals driftCh on mismatch.
	// Canceled and joined BEFORE hb.Stop() and mc.Close() so no ping outlives
	// runConnection or touches borrowed state after shutdown (R-5, R-6).
	pingCtx, pingCancel := context.WithCancel(ctx)
	pingDone := make(chan struct{})
	go func() {
		defer close(pingDone)
		runPingLoop(pingCtx, mc, cfg, hb, hbDone, driftCh)
	}()
	defer func() {
		pingCancel()
		<-pingDone
	}()

	// Notification loop: handle hub-initiated RPCs and ping-triggered drift.
	return notificationLoopWithDrift(ctx, mc, cfg, hbDone, sampler, driftCh)
}

// runPingLoop sends periodic VerbPing heartbeats carrying the client's current
// SHA-1 chain head with a bounded timeout and signals driftCh on mismatch.
func runPingLoop(ctx context.Context, mc *rpc.MuxConn, cfg *ClientConfig, hb *Heartbeat, hbDone <-chan struct{}, driftCh chan<- string) {
	ticker := time.NewTicker(cfg.pingTickInterval())
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			reply, pingErr := sendPing(ctx, mc, cfg.currentChainHash(), cfg.pingBound())
			if pingErr != nil {
				if errors.Is(pingErr, context.DeadlineExceeded) && ctx.Err() == nil {
					continue
				}
				return
			}
			hb.recordPong() // Successful ping response = hub is alive.
			cfg.checkReplyChain(reply, driftCh)
		case <-ctx.Done():
			return
		case <-hbDone:
			return
		case <-mc.Done():
			return
		}
	}
}

// fetchStatusCurrent is the status of a config-fetch answer that says the hub
// holds the configuration whose hash the client sent (pkg/fleet
// ConfigFetchResponse).
const fetchStatusCurrent = "current"

// holdRefusalPrefix opens the config-ack error of a device that holds. The
// hub stores the text as that client's last refusal and parses nothing in it.
const holdRefusalPrefix = "holding: the configuration this device runs differs from the last one this hub sent"

// fetchOutcome carries hub-advertised session settings returned by a
// successful config-fetch reply. hasHubReply distinguishes an explicit zero
// (or omitted) interval from a fetch that never reached the hub (such as a
// local baseline read failure).
type fetchOutcome struct {
	sampleIntervalSeconds int
	hasHubReply           bool
}

// fetchAndProcess fetches config from hub, validates, commits, and ACKs the result.
// Each fetch it makes, the one made again over a local commit included, is
// bounded by cfg's fetchBound (configFetchTimeout). It returns errEndpointChanged or errHubClientRemoved when the config it
// applied changes or removes the hub connection: the ACK is already sent, and
// the caller MUST end the connection. A device that holds, or that cannot
// compare its configuration with its baseline, applies nothing and answers
// nil: the connection is kept. cfg.Baseline MUST NOT be nil, which
// RunManagedClient guarantees.
func fetchAndProcess(ctx context.Context, mc *rpc.MuxConn, cfg *ClientConfig) error {
	_, err := fetchAndProcessOutcome(ctx, mc, cfg)
	return err
}

// fetchAndProcessOutcome is fetchAndProcess returning the hub's advertised
// sample interval alongside the fetch result.
func fetchAndProcessOutcome(ctx context.Context, mc *rpc.MuxConn, cfg *ClientConfig) (fetchOutcome, error) {
	return fetchAndProcessOutcomeWithDrift(ctx, mc, cfg, nil)
}

func fetchAndProcessOutcomeWithDrift(ctx context.Context, mc *rpc.MuxConn, cfg *ClientConfig, driftCh chan<- string) (fetchOutcome, error) {
	return fetchAgainstBaselineWithMode(ctx, mc, cfg, cfg.Baseline, refetchOnce, "", "", driftCh)
}

// fetchAgainstBaselineWithMode is the fetch fetchAndProcess makes: it sends
// the hash of the configuration the device runs, read now, and applies what
// the hub returns only while that configuration is the last one the hub gave
// (Diverged). The hub's "current" answer ends a hold: it proves hub and
// device hold the same configuration, so that hash becomes the baseline, and
// the device acknowledges it so the hub drops the refusal it recorded while
// the device held. The one other end is a revert: an answer whose
// discard-version is exactly the active hash is applied (applyRevertedWithDrift). A
// discard-version that names any other hash is a decision taken on an older
// configuration of this device, and the device holds. refetch says whether a
// push or a revert that meets a local commit landed during this fetch may
// fetch once more over that newer edit (refetchOverLocalCommitWithDrift).
func fetchAgainstBaselineWithMode(ctx context.Context, mc *rpc.MuxConn, cfg *ClientConfig, access *BaselineAccess, refetch revertRefetch, resync, triggerChain string, driftCh chan<- string) (fetchOutcome, error) {
	active, baseline, err := access.read()
	// refuse-by-default: guards fetchAndProcessOutcome -- refuseUncompared logged and acked the store read error; keep the session open with hasHubReply false.
	if err != nil {
		refuseUncompared(ctx, mc, cfg, err)
		cfg.reportHold(true)
		return fetchOutcome{}, nil
	}
	if resync == fleet.ResyncModeDiff && Diverged(active, baseline) {
		// Hold precedence (fleet-6/fleet-7, AC-9): never auto-resync over a local edit.
		cfg.reportHold(true)
		return fetchOutcome{}, nil
	}
	resp, err := fetchConfigWithin(ctx, mc, active.Hash(), cfg.currentChainHash(), resync, cfg.fetchBound())
	if err != nil {
		return fetchOutcome{}, fmt.Errorf("fetch: %w", err)
	}
	outcome := fetchOutcome{
		sampleIntervalSeconds: resp.SampleIntervalSeconds,
		hasHubReply:           true,
	}
	cfg.reportHubAnswer(hubAnswerOf(resp, active))
	if resp.Status == fetchStatusCurrent {
		return handleCurrentFetchResponse(ctx, mc, cfg, access, resp, active, baseline, refetch, resync, triggerChain, driftCh)
	}
	if resp.Mode == fleet.FetchModeDiff {
		if Diverged(active, baseline) {
			holdFetched(ctx, mc, cfg, access, resp, active, baseline)
			cfg.reportHold(true)
			return outcome, nil
		}
		return handleDiffFetchResponse(ctx, mc, cfg, access, resp, active, baseline, refetch, triggerChain, driftCh)
	}
	if resp.Config == "" {
		if resync == fleet.ResyncModeFull {
			failFetchAndAck(ctx, mc, cfg, resp.Version, resp.ChainHash, triggerChain, chainMetricMethodFull, fleet.RejectKindResync, "hub returned empty config on full replay")
			return outcome, nil
		}
		logger().Debug("the hub sent no config", "version", active.Hash())
		return outcome, nil
	}
	// Diverged holds only when both hashes are recorded, so an answer with no
	// discard-version never matches the active hash here.
	if Diverged(active, baseline) && resp.DiscardVersion == active.Hash() {
		return applyRevertedWithDrift(ctx, mc, cfg, access, resp, active, refetch, driftCh)
	}
	return holdOrApplyWithDrift(ctx, mc, cfg, access, resp, active, baseline, refetch, triggerChain, driftCh)
}

// handleCurrentFetchResponse settles baseline and hash-chain state for a
// Status=="current" fetch reply. On a re-anchor reply, a non-holding client
// verifies FoldFull(activeBytes) == resp.ChainHash and persists the anchor
// without sending config-ack (preserving TestFirstBaselineSendsNoAck); a
// holding client whose hold just ended sends config-ack as before. If local
// bytes do not match resp.ChainHash, a non-holding client falls back once to
// resync=="full".
func handleCurrentFetchResponse(ctx context.Context, mc *rpc.MuxConn, cfg *ClientConfig, access *BaselineAccess, resp fleet.ConfigFetchResponse, active, baseline ConfigVersion, refetch revertRefetch, resync, triggerChain string, driftCh chan<- string) (fetchOutcome, error) {
	outcome := fetchOutcome{
		sampleIntervalSeconds: resp.SampleIntervalSeconds,
		hasHubReply:           true,
	}
	diverged := Diverged(active, baseline)
	if resp.ChainHash != "" {
		clientChain := cfg.currentChainHash()
		if !resp.Reanchor && resp.ChainHash == clientChain {
			cfg.persistChainState(active.Hash(), resp.ChainHash, "")
			settleBaseline(cfg, access, active, baseline)
			if diverged {
				sendAckWithDrift(ctx, mc, cfg, fleet.ConfigAck{Version: active.Hash(), OK: true, ChainHash: resp.ChainHash}, driftCh)
			}
			cfg.reportHold(false)
			return outcome, nil
		}
		anchorValid := fleet.ValidChainHash(resp.ChainHash)
		var localAnchor string
		if anchorValid {
			if activeBytes, readErr := access.activeBytes(); readErr == nil {
				localAnchor = fleet.FoldFull(activeBytes)
			}
		}
		if anchorValid && localAnchor == resp.ChainHash {
			cfg.persistChainState(active.Hash(), resp.ChainHash, "")
			settleBaseline(cfg, access, active, baseline)
			if diverged {
				sendAckWithDrift(ctx, mc, cfg, fleet.ConfigAck{Version: active.Hash(), OK: true, ChainHash: resp.ChainHash}, driftCh)
			}
			cfg.reportHold(false)
			return outcome, nil
		}
		if !diverged && resync != fleet.ResyncModeFull {
			reason := "local active configuration anchor did not match hub reanchor chain-hash"
			if !anchorValid {
				reason = "hub returned invalid chain-hash on current reply"
			}
			logger().Info("current reanchor verification failed, falling back to full configuration replay",
				"name", cfg.Name, "version", active.Hash(), "error", reason)
			return fetchFullReplayFallback(ctx, mc, cfg, access, active, baseline, refetch, triggerChain, driftCh, reason)
		}
		if diverged {
			cfg.reportHold(true)
			return outcome, nil
		}
		failFetchAndAck(ctx, mc, cfg, active.Hash(), resp.ChainHash, triggerChain, chainMetricMethodFull, fleet.RejectKindResync, "reanchor chain-hash mismatch after full replay request")
		return outcome, nil
	}
	settleBaseline(cfg, access, active, baseline)
	if diverged {
		// The hub last heard a refusal from this device. Nothing was
		// applied, so no ack would otherwise follow, and the hub would go
		// on listing a device that holds after its operator resolved it.
		sendAckWithDrift(ctx, mc, cfg, fleet.ConfigAck{Version: active.Hash(), OK: true, ChainHash: cfg.currentChainHash()}, driftCh)
	}
	cfg.reportHold(false)
	return outcome, nil
}

// handleDiffFetchResponse verifies, reconstructs, validates, and commits a
// Mode=="diff" response. Any pre-commit failure stages nothing, leaves the
// chain untouched, records a diff fallback, and triggers ONE bounded
// resync=="full" fetch.
func handleDiffFetchResponse(ctx context.Context, mc *rpc.MuxConn, cfg *ClientConfig, access *BaselineAccess, resp fleet.ConfigFetchResponse, active, baseline ConfigVersion, refetch revertRefetch, triggerChain string, driftCh chan<- string) (fetchOutcome, error) {
	outcome := fetchOutcome{
		sampleIntervalSeconds: resp.SampleIntervalSeconds,
		hasHubReply:           true,
	}
	activeBytes, readErr := access.activeBytes()
	var reconstructed []byte
	var diffErr error
	if readErr != nil {
		diffErr = fmt.Errorf("read active config: %w", readErr)
	} else {
		reconstructed, diffErr = reconstructAndVerifyDiff(activeBytes, cfg.currentChainHash(), resp, cfg.Handler)
	}
	if diffErr != nil {
		logger().Info("diff resync verification failed, falling back to full configuration replay",
			"name", cfg.Name, "version", resp.Version, "error", diffErr)
		cfg.chain().recordResync(chainMetricMethodDiff, chainMetricResultFallback)
		return fetchFullReplayFallback(ctx, mc, cfg, access, active, baseline, refetch, triggerChain, driftCh, diffErr.Error())
	}

	if cfg.CommitOver == nil {
		failFetchAndAck(ctx, mc, cfg, resp.Version, resp.ChainHash, triggerChain, chainMetricMethodDiff, fleet.RejectKindCommit, "commit not ready")
		cfg.reportHold(false)
		return outcome, nil
	}
	commitErr := cfg.CommitOver(reconstructed, activeCheck(active))
	if errors.Is(commitErr, errActiveChanged) {
		return refetchOverLocalCommitWithDrift(ctx, mc, cfg, access, active, refetch, driftCh)
	}
	// refuse-by-default: guards fetchAgainstBaselineWithMode -- commit refusal is acked, latched, logged, and reported on the warning bus while keeping the session open.
	if commitErr != nil {
		failFetchAndAck(ctx, mc, cfg, resp.Version, resp.ChainHash, triggerChain, chainMetricMethodDiff, fleet.RejectKindCommit, "commit config: "+commitErr.Error())
		cfg.reportHold(false)
		return outcome, nil //nolint:nilerr // commit refusal is acked and latched without closing the session
	}

	cfg.persistChainState(resp.Version, resp.ChainHash, LastResyncDiff)
	cfg.chain().recordResync(chainMetricMethodDiff, chainMetricResultApplied)
	sendAckWithDrift(ctx, mc, cfg, fleet.ConfigAck{
		Version:   resp.Version,
		OK:        true,
		ChainHash: resp.ChainHash,
	}, driftCh)
	cfg.reportHold(false)
	logger().Info("config updated via diff", "version", resp.Version)
	return outcome, cfg.adoptEndpoint(reconstructed)
}

// fetchFullReplayFallback executes the single bounded resync=="full" fallback
// after a diff or reanchor failure. It re-checks Diverged(active, baseline) so
// a local edit is never overwritten, and refuses any recursive diff reply.
func fetchFullReplayFallback(ctx context.Context, mc *rpc.MuxConn, cfg *ClientConfig, access *BaselineAccess, _, _ ConfigVersion, refetch revertRefetch, triggerChain string, driftCh chan<- string, diffReason string) (fetchOutcome, error) {
	active, baseline, err := access.read()
	// refuse-by-default: guards handleDiffFetchResponse -- refuseUncompared logged and acked the store read error; keep the session open with hasHubReply false.
	if err != nil {
		refuseUncompared(ctx, mc, cfg, err)
		cfg.reportHold(true)
		return fetchOutcome{}, nil
	}
	if Diverged(active, baseline) {
		cfg.reportHold(true)
		return fetchOutcome{}, nil
	}
	resp, err := fetchConfigWithin(ctx, mc, active.Hash(), cfg.currentChainHash(), fleet.ResyncModeFull, cfg.fetchBound())
	if err != nil {
		return fetchOutcome{}, fmt.Errorf("full replay fetch: %w", err)
	}
	outcome := fetchOutcome{
		sampleIntervalSeconds: resp.SampleIntervalSeconds,
		hasHubReply:           true,
	}
	cfg.reportHubAnswer(hubAnswerOf(resp, active))
	if resp.Status == fetchStatusCurrent {
		return handleCurrentFetchResponse(ctx, mc, cfg, access, resp, active, baseline, refetch, fleet.ResyncModeFull, triggerChain, driftCh)
	}
	if resp.Mode == fleet.FetchModeDiff {
		cause := combineDiffAndFullError(diffReason, "hub returned diff mode in response to full replay request")
		failFetchAndAck(ctx, mc, cfg, resp.Version, resp.ChainHash, triggerChain, chainMetricMethodFull, fleet.RejectKindResync, cause)
		cfg.reportHold(false)
		return outcome, nil
	}
	if resp.Config == "" {
		cause := combineDiffAndFullError(diffReason, "hub returned empty config on full replay")
		failFetchAndAck(ctx, mc, cfg, resp.Version, resp.ChainHash, triggerChain, chainMetricMethodFull, fleet.RejectKindResync, cause)
		cfg.reportHold(false)
		return outcome, nil
	}
	applyErr := applyFullFetched(ctx, mc, cfg, resp, active, diffReason, triggerChain, driftCh)
	if errors.Is(applyErr, errActiveChanged) {
		return refetchOverLocalCommitWithDrift(ctx, mc, cfg, access, active, refetch, driftCh)
	}
	cfg.reportHold(false)
	return outcome, applyErr
}

// applyRevertedWithDrift applies the configuration the hub returned to a device
// that holds, because the hub's operator decided that it drops the
// configuration it runs: the answer's discard-version names exactly the active
// hash, so the decision was taken on this configuration and on the hub
// configuration the answer carries (pluginserver.RevertDecision). It commits as
// a push does (applyFullFetched): the stage refuses, under the config's lock,
// when the configuration active then is no longer the one the hash was read
// from (activeCheck). A local commit that landed during the fetch is a newer
// local edit, so the device then fetches again, sending the hash of that edit,
// and decides the new answer as any fetch (refetchOverLocalCommitWithDrift):
// the hub's answer to the first fetch names the version it fetched, the copy it
// holds and the version its decision discards for the old configuration, and a
// hold decided on it would send a copy the hub refuses to show. Like every
// push, the revert stages through the machine entry the freeze exempts and
// records the new baseline, and a confirmed-commit window open on the device
// refuses it. The hold is reported from the store read again after the apply: a
// refused apply leaves the device holding.
func applyRevertedWithDrift(ctx context.Context, mc *rpc.MuxConn, cfg *ClientConfig, access *BaselineAccess, resp fleet.ConfigFetchResponse, active ConfigVersion, refetch revertRefetch, driftCh chan<- string) (fetchOutcome, error) {
	logger().Info("this device held, and its hub's operator decided that it drops the configuration it runs and takes the hub's",
		"name", cfg.Name, "discarded", active.Hash(), "offered", resp.Version)
	applyErr := applyFullFetched(ctx, mc, cfg, resp, active, "", "", driftCh)
	if errors.Is(applyErr, errActiveChanged) {
		return refetchOverLocalCommitWithDrift(ctx, mc, cfg, access, active, refetch, driftCh)
	}
	after, baseline, readErr := access.read()
	cfg.reportHold(readErr != nil || Diverged(after, baseline))
	return fetchOutcome{
		sampleIntervalSeconds: resp.SampleIntervalSeconds,
		hasHubReply:           true,
	}, applyErr
}

// revertRefetch says whether a push or a revert refused because a local
// commit landed during its fetch may fetch again (refetchOverLocalCommit). It
// bounds that path to two fetches per fetchAndProcess call.
type revertRefetch bool

const (
	// refetchOnce is the first fetch of a call: a refused commit fetches again.
	refetchOnce revertRefetch = true
	// refetchNone is the fetch made again: a refused commit fetches no more.
	refetchNone revertRefetch = false
)

// errActiveChangedTwice ends a fetch whose push or revert met a local commit
// on both of its fetches. The device applied nothing and has told its hub
// nothing about the newest edit: ending the fetch here, rather than deciding a
// hold on an answer about another version, ends the connection, so the
// reconnect's fetch decides over what the device then runs.
var errActiveChangedTwice = errors.New("the configuration this device runs changed during both fetches of its hub's configuration, " +
	"so it applied nothing and fetches again when it connects again")

// refetchOverLocalCommitWithDrift answers a push or a revert the stage refused
// because a local commit landed during its fetch (activeCheck): the device
// keeps the newer edit, and fetches once more with its hash so what it decides
// (a hold, its copy and its ack, or an apply) answers a fetch of that version.
// decided is the version the refused commit was decided over. On the fetch made
// again (refetchNone) it fetches no more and answers errActiveChangedTwice.
func refetchOverLocalCommitWithDrift(ctx context.Context, mc *rpc.MuxConn, cfg *ClientConfig, access *BaselineAccess, decided ConfigVersion, refetch revertRefetch, driftCh chan<- string) (fetchOutcome, error) {
	if refetch == refetchNone {
		// The hold is reported from the store read again, as applyReverted
		// reports it: a later local commit may have put the device back on
		// its baseline, and a device there does not hold.
		after, baseline, readErr := access.read()
		cfg.reportHold(readErr != nil || Diverged(after, baseline))
		return fetchOutcome{}, fmt.Errorf("%w (the first fetch was decided over %s)", errActiveChangedTwice, decided.Hash())
	}
	logger().Info("this device's configuration changed while its hub's configuration was fetched, so it keeps the newer local edit, "+
		"applies nothing from that fetch, and fetches again over that edit",
		"name", cfg.Name, "decided", decided.Hash())
	return fetchAgainstBaselineWithMode(ctx, mc, cfg, access, refetchNone, "", "", driftCh)
}

// logLocalCommitTwice logs a fetch that a local commit ended on both of its
// fetches (errActiveChangedTwice), at connect and after a config-changed
// alike; the caller then ends the connection, and RunManagedClient
// reconnects without logging it again. It is Info, not Warn: the hub answered both fetches, and the device
// chose to keep a local edit over them, which is a decision and no fault. A
// hold it leaves is reported by the managed-diverged doctor check.
func logLocalCommitTwice(cfg *ClientConfig, err error) {
	logger().Info("this device's configuration changed during both fetches of its hub's configuration, "+
		"so it keeps its local edit, applies nothing from its hub, and connects again so its next fetch decides over that edit",
		"name", cfg.Name, "server", cfg.Server, "error", err)
}

// errActiveChanged is activeCheck's refusal: the configuration active when a
// push or a revert would stage is not the one the client read before its
// fetch. applyFetched answers it without an ack, because the device then
// fetches again (refetchOverLocalCommit) and that fetch's ack says what it
// decided.
var errActiveChanged = errors.New("the configuration this device runs changed after it fetched its hub's configuration")

// activeCheck is the check every stage of a hub configuration runs under the
// config lock (ClientConfig.CommitOver): it refuses, with errActiveChanged, a
// configuration whose version is not decided, the version the client read
// before its fetch and decided to apply over. It compares with neither the
// baseline nor the hub's version: the decision rested on decided alone.
func activeCheck(decided ConfigVersion) func(active []byte) error {
	return func(active []byte) error {
		if now := versionOf(active); now != decided {
			return fmt.Errorf("%w: it runs %s, and the hub's configuration was decided over %s", errActiveChanged, now.Hash(), decided.Hash())
		}
		return nil
	}
}

// holdOrApplyWithDrift decides a fetched configuration that carries no revert
// of the active version: it holds while the device's configuration differs from
// its baseline (holdFetched), and applies the configuration otherwise. A push
// the stage refused because a local commit landed during the fetch fetches
// again over that edit (refetchOverLocalCommitWithDrift, bounded by refetch).
// The hold is reported after the outcome, so a push that turns into a hold
// never reports "not holding" first.
func holdOrApplyWithDrift(ctx context.Context, mc *rpc.MuxConn, cfg *ClientConfig, access *BaselineAccess, resp fleet.ConfigFetchResponse, active, baseline ConfigVersion, refetch revertRefetch, triggerChain string, driftCh chan<- string) (fetchOutcome, error) {
	outcome := fetchOutcome{
		sampleIntervalSeconds: resp.SampleIntervalSeconds,
		hasHubReply:           true,
	}
	if Diverged(active, baseline) {
		holdFetched(ctx, mc, cfg, access, resp, active, baseline)
		cfg.reportHold(true)
		return outcome, nil
	}
	applyErr := applyFullFetched(ctx, mc, cfg, resp, active, "", triggerChain, driftCh)
	if errors.Is(applyErr, errActiveChanged) {
		return refetchOverLocalCommitWithDrift(ctx, mc, cfg, access, active, refetch, driftCh)
	}
	cfg.reportHold(false)
	return outcome, applyErr
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
			"code", CodeDoctorManagedDiverged, "name", cfg.Name, "version", active.Hash(), "error", err)
		return
	}
	logger().Info("the hub holds the configuration this device runs, recorded as the baseline",
		"name", cfg.Name, "version", active.Hash())
}

// holdFetched refuses the configuration the hub returned because the device
// runs one that differs from its baseline. The hub's configuration is neither
// decoded, validated nor committed. The device first sends the hub a copy of
// the configuration it runs, unless the hub holds one already (uploadHeld),
// so the hub knows at the ack whether it can show the hold. The hub is then
// told in a config-ack that says the device holds, carries the two hashes and
// whether the copy was sent, and no configuration content, and the device
// logs one warning, whose fix names the hub commands only while the hub keeps
// a copy of the active version (heldFix). The caller MUST keep the connection: ending it would make
// the device reconnect and hold again about once a second.
func holdFetched(ctx context.Context, mc *rpc.MuxConn, cfg *ClientConfig, access *BaselineAccess, resp fleet.ConfigFetchResponse, active, baseline ConfigVersion) {
	copied, kept := uploadHeld(ctx, mc, cfg, access, active, resp)
	sendAck(ctx, mc, fleet.ConfigAck{
		Version:   resp.Version,
		OK:        false,
		Holding:   true,
		ChainHash: cfg.currentChainHash(),
		Error: holdRefusalPrefix + " (active " + active.Hash() + ", baseline " + baseline.Hash() + "); " +
			"the device applied nothing and stays connected, " + copied +
			", and ze doctor on the device names the two ways to resolve it",
	})
	logger().Warn("this device holds: the configuration it runs differs from the last one its hub sent, so it applied nothing from the hub",
		"code", CodeDoctorManagedDiverged, "name", cfg.Name,
		"active", active.Hash(), "baseline", baseline.Hash(), "offered", resp.Version, "fix", heldFix(cfg.Name, active, kept))
}

// heldFix is the fix a hold's warning names: the hub commands over the active
// version while the hub keeps a copy of exactly that version, and the manual
// ways out otherwise, since a hub that keeps no such copy has no hold to show
// or end (an old hub has not even the commands).
func heldFix(client string, active ConfigVersion, kept bool) string {
	if kept {
		return managedDivergedFix(client, active.Hash())
	}
	return managedDivergedManualFix(client)
}

// uploadHeld sends the hub a copy of the configuration a holding device runs
// (config-held), so the hub's operator can see the hold and end it from the
// hub. It answers the clause the hold ack carries about the copy. It sends at
// most one copy per call and none when:
//
//   - the hub's answer names this version as the copy it holds (held-version)
//   - this connection's hub refused the verb (heldUploads.refusedOn)
//   - an upload of this version already failed in this run (heldUploads.failed)
//   - the configuration is larger than fleet.HeldConfigMax
//
// The bytes and their version come from one read (BaselineAccess.held), so
// the copy is exactly the configuration whose hash the copy names. The call
// is bounded by heldUploadTimeout. Nothing here ends the connection or
// retries: a copy that could not be sent is logged, and the device holds as
// it would with no hub copy.
func uploadHeld(ctx context.Context, mc *rpc.MuxConn, cfg *ClientConfig, access *BaselineAccess, active ConfigVersion, resp fleet.ConfigFetchResponse) (clause string, kept bool) {
	if resp.HeldVersion == active.Hash() {
		return "the hub holds a copy of it", true
	}
	if cfg.held.refusedOn == mc {
		return "no copy was sent: this hub does not take one", false
	}
	if cfg.held.failed == active {
		return "no copy was sent: " + cfg.held.failure, false
	}
	held, err := access.held()
	if err != nil {
		logger().Warn("this device holds and could not read the configuration it runs, so it sent its hub no copy; check the store with: "+fixCheckStore,
			"code", CodeDoctorManagedDiverged, "name", cfg.Name, "error", err)
		return "no copy was sent: the device could not read the configuration it runs", false
	}
	if len(held.content) > fleet.HeldConfigMax {
		refusal := heldTooLarge(len(held.content))
		cfg.held.failed, cfg.held.failure = held.version, refusal.Problem()
		logger().Warn("this device holds a configuration too large to send its hub, so the hub cannot show or resolve the hold",
			"code", CodeDoctorManagedDiverged, "name", cfg.Name, "version", held.version.Hash(),
			"size", len(held.content), "error", refusal, "fix", managedDivergedManualFix(cfg.Name))
		return "no copy was sent: " + cfg.held.failure, false
	}
	uploadCtx, cancel := context.WithTimeout(ctx, heldUploadTimeout)
	defer cancel()
	rawReply, err := mc.CallRPC(uploadCtx, fleet.VerbConfigHeld, fleet.ConfigHeld{
		Version:   held.version.Hash(),
		Config:    base64.StdEncoding.EncodeToString(held.content),
		EditedAt:  held.editedAt,
		ChainHash: cfg.currentChainHash(),
	})
	if err == nil {
		if _, decErr := fleet.DecodeManagedReply(rawReply); decErr != nil {
			logger().Warn("could not decode hub config-held reply", "code", pluginserver.CodeDoctorManagedChainDrift, "name", cfg.Name, "fix", chainDriftFix, "error", decErr)
		}
		logger().Info("this device holds, and sent its hub a copy of the configuration it runs",
			"name", cfg.Name, "version", held.version.Hash(), "size", len(held.content))
		return "a copy was sent to the hub", held.version == active
	}
	if refused, answered := errors.AsType[*rpc.RPCCallError](err); answered {
		if refused.Code != fleet.HeldRefusedCode {
			// A hub that answers the verb with any other error does not take
			// it: a hub built before config-held answers it as an unknown
			// method.
			cfg.held.refusedOn = mc
			logger().Info("this device holds, and its hub does not take a copy of the configuration it runs",
				"name", cfg.Name, "error", err)
			return "no copy was sent: this hub does not take one", false
		}
	}
	cfg.held.failed, cfg.held.failure = held.version, "the copy failed: "+err.Error()
	logger().Warn("this device holds, and the copy of the configuration it runs did not reach its hub, so it sends none of this version again until its client restarts",
		"code", CodeDoctorManagedDiverged, "name", cfg.Name, "version", held.version.Hash(),
		"error", err, "fix", managedDivergedManualFix(cfg.Name))
	return "no copy was sent: " + cfg.held.failure, false
}

// heldTooLarge is the refusal of a held configuration larger than the copy a
// hub takes. The hold is then ended by hand, as before a hub could take a copy.
func heldTooLarge(size int) *opererr.Error {
	return opererr.New(
		"the configuration this device holds is "+strconv.Itoa(size)+" bytes, larger than the "+
			strconv.Itoa(fleet.HeldConfigMax)+" bytes a hub takes as a copy",
		"the hub cannot show this hold or resolve it, so it is resolved by hand",
		"ze doctor")
}

// refuseUncompared is a fetch the client did not make because the store could
// not say what the device runs or what its baseline is. Applying blindly could
// overwrite a local change, so nothing is fetched or applied; holding in
// silence would hide it, so the hub is told in a config-ack and the device
// logs one warning. The caller MUST keep the connection: the next fetch reads
// the store again.
func refuseUncompared(ctx context.Context, mc *rpc.MuxConn, cfg *ClientConfig, cause error) {
	sendAck(ctx, mc, fleet.ConfigAck{
		OK:        false,
		ChainHash: cfg.currentChainHash(),
		Error: "this device could not compare the configuration it runs with the last one this hub sent, " +
			"so it fetched and applied nothing: " + cause.Error(),
	})
	logger().Warn("this device could not compare the configuration it runs with the last one its hub sent, "+
		"so it fetched and applied nothing from the hub; check the store with: "+fixCheckStore,
		"code", CodeDoctorManagedDiverged, "name", cfg.Name, "error", cause)
}

// sendAck sends ack to the hub. An ack that cannot be sent is logged: the
// connection that lost it ends on its own, and the next one fetches again.
func sendAck(ctx context.Context, mc *rpc.MuxConn, ack fleet.ConfigAck) {
	if _, ackErr := sendConfigAckReply(ctx, mc, ack); ackErr != nil {
		logger().Warn("send ack failed", "code", codeHubUnreachable, "error", ackErr)
	}
}

// sendAckWithDrift sends ack to the hub and returns the hub's replied chain
// hash (empty on legacy reply or send error). It checks reply drift ONLY on
// successful non-holding acknowledgements so failure acks never spin driftCh
// or inflate client chain mismatch counters.
func sendAckWithDrift(ctx context.Context, mc *rpc.MuxConn, cfg *ClientConfig, ack fleet.ConfigAck, driftCh chan<- string) string {
	reply, ackErr := sendConfigAckReply(ctx, mc, ack)
	// refuse-by-default: guards failFetchAndAck -- ack send error is logged under codeHubUnreachable; empty replyChain means no hub reply chain was received.
	if ackErr != nil {
		logger().Warn("send ack failed", "code", codeHubUnreachable, "error", ackErr)
		return ""
	}
	if ack.OK && !ack.Holding {
		cfg.checkReplyChain(reply, driftCh)
	}
	return reply.ChainHash
}

// failFetchAndAck sends a non-OK config-ack without triggering driftCh or
// bumping mismatch counters, then latches both the hub's response/trigger
// chains and the hub's ack reply chain via recordChainDriftFailure.
func failFetchAndAck(ctx context.Context, mc *rpc.MuxConn, cfg *ClientConfig, version, hubChain, triggerChain, method, rejectKind, cause string) {
	replyChain := sendAckWithDrift(ctx, mc, cfg, fleet.ConfigAck{
		Version:    version,
		OK:         false,
		Error:      cause,
		RejectKind: rejectKind,
		ChainHash:  cfg.currentChainHash(),
	}, nil)
	cfg.recordChainDriftFailure(version, hubChain, triggerChain, replyChain, method, cause)
}

// applyFullFetched validates, commits and acknowledges a full configuration the
// hub returned, a push, revert, or full-replay fallback. It commits through
// CommitOver over activeCheck(decided), decided being the version the client
// read before its fetch; a nil CommitOver refuses the configuration as not
// ready. A commit refused with errActiveChanged is answered as is and acks
// nothing: the caller fetches again, and that fetch's ack tells the hub. Any
// other refusal is acked as a refusal with its cause. It returns
// errEndpointChanged or errHubClientRemoved as fetchAndProcess documents.
func applyFullFetched(ctx context.Context, mc *rpc.MuxConn, cfg *ClientConfig, resp fleet.ConfigFetchResponse, decided ConfigVersion, priorDiffReason, triggerChain string, driftCh chan<- string) error {
	data, err := base64.StdEncoding.DecodeString(resp.Config)
	// refuse-by-default: guards holdOrApplyWithDrift -- decode refusal is acked, latched, logged, and reported while keeping the session open.
	if err != nil {
		cause := combineDiffAndFullError(priorDiffReason, "decode config: "+err.Error())
		failFetchAndAck(ctx, mc, cfg, resp.Version, resp.ChainHash, triggerChain, chainMetricMethodFull, fleet.RejectKindResync, cause)
		return nil //nolint:nilerr // decode refusal is acked and latched without closing the session
	}
	if resp.Version != "" {
		if gotVer := fleet.VersionHash(data); gotVer != resp.Version {
			cause := combineDiffAndFullError(priorDiffReason, "config version mismatch: got "+gotVer+", hub declared "+resp.Version)
			failFetchAndAck(ctx, mc, cfg, resp.Version, resp.ChainHash, triggerChain, chainMetricMethodFull, fleet.RejectKindResync, cause)
			return nil
		}
	}
	anchor := fleet.FoldFull(data)
	if resp.ChainHash != "" {
		if !fleet.ValidChainHash(resp.ChainHash) {
			cause := combineDiffAndFullError(priorDiffReason, "config chain-hash invalid: hub declared "+resp.ChainHash)
			failFetchAndAck(ctx, mc, cfg, resp.Version, resp.ChainHash, triggerChain, chainMetricMethodFull, fleet.RejectKindResync, cause)
			return nil
		}
		if anchor != resp.ChainHash {
			cause := combineDiffAndFullError(priorDiffReason, "config chain-hash mismatch: computed "+anchor+", hub declared "+resp.ChainHash)
			failFetchAndAck(ctx, mc, cfg, resp.Version, resp.ChainHash, triggerChain, chainMetricMethodFull, fleet.RejectKindResync, cause)
			return nil
		}
	}
	// refuse-by-default: guards holdOrApplyWithDrift -- validation refusal is acked with RejectKindVerify using a fixed secret-free cause, latched, logged, and reported while keeping the session open.
	if err := cfg.Handler.ValidateRawConfig(data); err != nil {
		cause := combineDiffAndFullError(priorDiffReason, "config failed validation")
		failFetchAndAck(ctx, mc, cfg, resp.Version, resp.ChainHash, triggerChain, chainMetricMethodFull, fleet.RejectKindVerify, cause)
		return nil //nolint:nilerr // validation refusal is acked and latched without closing the session
	}
	if cfg.CommitOver == nil {
		cause := combineDiffAndFullError(priorDiffReason, "commit not ready")
		failFetchAndAck(ctx, mc, cfg, resp.Version, resp.ChainHash, triggerChain, chainMetricMethodFull, fleet.RejectKindCommit, cause)
		return nil
	}
	commitErr := cfg.CommitOver(data, activeCheck(decided))
	if errors.Is(commitErr, errActiveChanged) {
		return commitErr
	}
	// refuse-by-default: guards holdOrApplyWithDrift -- commit refusal is acked with RejectKindCommit, latched, logged, and reported while keeping the session open.
	if commitErr != nil {
		cause := combineDiffAndFullError(priorDiffReason, "commit config: "+commitErr.Error())
		failFetchAndAck(ctx, mc, cfg, resp.Version, resp.ChainHash, triggerChain, chainMetricMethodFull, fleet.RejectKindCommit, cause)
		return nil //nolint:nilerr // commit refusal is acked and latched without closing the session
	}

	version := resp.Version
	if version == "" {
		version = fleet.VersionHash(data)
	}
	cfg.persistChainState(version, anchor, LastResyncFullReplay)
	cfg.chain().recordResync(chainMetricMethodFull, chainMetricResultApplied)
	sendAckWithDrift(ctx, mc, cfg, fleet.ConfigAck{
		Version:   resp.Version,
		OK:        true,
		ChainHash: anchor,
	}, driftCh)
	logger().Info("config updated", "version", resp.Version)
	return cfg.adoptEndpoint(data)
}

// notificationLoop reads hub-initiated RPCs until disconnect or shutdown.
func notificationLoop(ctx context.Context, mc *rpc.MuxConn, cfg *ClientConfig, hbDone <-chan struct{}) error {
	return notificationLoopWithDrift(ctx, mc, cfg, hbDone, nil, nil)
}

func notificationLoopWithDrift(ctx context.Context, mc *rpc.MuxConn, cfg *ClientConfig, hbDone <-chan struct{}, sampler *sessionSampler, driftCh chan string) error {
	for {
		select {
		case req, ok := <-mc.Requests():
			if !ok {
				return errConnectionClosed
			}
			if err := handleHubRequestWithDrift(ctx, mc, req, cfg, sampler, driftCh); err != nil {
				return err
			}

		case hubChain := <-driftCh:
			if err := handleDriftMismatchWithSampler(ctx, mc, cfg, sampler, hubChain, driftCh); err != nil {
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

// handleDriftMismatchWithSampler handles a chain-hash mismatch signaled by
// sendPing or a reply check, serially on the notification goroutine. Hold
// precedence (Diverged(active, baseline)) and the per-connection failed-sync
// latch are checked before initiating a diff-based resync.
func handleDriftMismatchWithSampler(ctx context.Context, mc *rpc.MuxConn, cfg *ClientConfig, sampler *sessionSampler, hubChain string, driftCh chan<- string) error {
	if hubChain != "" && hubChain == cfg.currentChainHash() {
		return nil
	}
	if cfg.chain().isLatchedChain(hubChain) {
		return nil
	}
	active, baseline, err := cfg.Baseline.read()
	// refuse-by-default: guards notificationLoopWithDrift -- refuseUncompared logged and acked the store read error; keep the session open.
	if err != nil {
		refuseUncompared(ctx, mc, cfg, err)
		cfg.reportHold(true)
		return nil
	}
	if Diverged(active, baseline) {
		cfg.reportHold(true)
		return nil
	}
	outcome, err := fetchAgainstBaselineWithMode(ctx, mc, cfg, cfg.Baseline, refetchOnce, fleet.ResyncModeDiff, hubChain, driftCh)
	if errors.Is(err, errEndpointChanged) {
		return err
	}
	if errors.Is(err, errHubClientRemoved) {
		return err
	}
	if errors.Is(err, errActiveChangedTwice) {
		logLocalCommitTwice(cfg, err)
		return err
	}
	// refuse-by-default: guards notificationLoopWithDrift -- transient resync fetch error is logged and retried on the next heartbeat or reconnect; keep the session open.
	if err != nil {
		logger().Warn("managed client could not resync configuration after hash chain mismatch; retrying on next heartbeat or reconnect",
			"code", codeHubUnreachable, "name", cfg.Name, "server", cfg.Server, "error", err, "fix", "ze doctor")
		return nil
	}
	if outcome.hasHubReply && sampler != nil {
		sampler.UpdateInterval(outcome.sampleIntervalSeconds)
	}
	return nil
}

// handleHubRequestWithDrift dispatches an inbound request from the hub. A
// non-nil error means the connection MUST end (handleConfigChangedRequest).
func handleHubRequestWithDrift(ctx context.Context, mc *rpc.MuxConn, req *rpc.Request, cfg *ClientConfig, sampler *sessionSampler, driftCh chan<- string) error {
	switch req.Method {
	case fleet.VerbConfigChanged:
		return handleConfigChangedRequestWithDrift(ctx, mc, req, cfg, sampler, driftCh)

	case fleet.VerbPing:
		_ = mc.SendResult(ctx, req.ID, fleet.ManagedReply{ChainHash: cfg.currentChainHash()})
	}
	// Unknown methods silently dropped for forward compatibility.
	return nil
}

// handleConfigChangedRequest processes a config-changed notification. It
// returns errEndpointChanged or errHubClientRemoved when the config it
// applied changes or removes the hub connection, and errActiveChangedTwice
// when a local commit ended both of its fetches; the caller MUST then end
// the connection. A fetch that fails, its bound reached included
// (configFetchTimeout), is logged and the connection is kept: the next
// notification or connection fetches again. A fetch a local commit ended on
// both of its fetches (errActiveChangedTwice) is logged at Info, under no hub
// code, because the hub answered both, and is returned like the two above.
func handleConfigChangedRequest(ctx context.Context, mc *rpc.MuxConn, req *rpc.Request, cfg *ClientConfig) error {
	return handleConfigChangedRequestWithDrift(ctx, mc, req, cfg, nil, nil)
}

func handleConfigChangedRequestWithDrift(ctx context.Context, mc *rpc.MuxConn, req *rpc.Request, cfg *ClientConfig, sampler *sessionSampler, driftCh chan<- string) error {
	var n fleet.ConfigChanged
	// refuse-by-default: guards handleHubRequestWithDrift -- malformed notification is rejected over RPC and logged; keep the session open.
	if err := json.Unmarshal(req.Params, &n); err != nil {
		logger().Warn("bad config-changed payload", "code", pluginserver.CodeDoctorManagedChainDrift, "fix", chainDriftFix, "error", err)
		_ = mc.SendError(ctx, req.ID, "bad payload")
		return nil
	}
	_ = mc.SendResult(ctx, req.ID, fleet.ManagedReply{ChainHash: cfg.currentChainHash()})

	// Fetch the new config, preserving the hub's advertised notification chain
	// hash as triggerChain so a failure latches it against subsequent pings.
	outcome, err := fetchAgainstBaselineWithMode(ctx, mc, cfg, cfg.Baseline, refetchOnce, "", n.ChainHash, driftCh)
	if errors.Is(err, errEndpointChanged) {
		return err
	}
	if errors.Is(err, errHubClientRemoved) {
		return err
	}
	if errors.Is(err, errActiveChangedTwice) {
		// Ends the connection, as at connect: nothing else fetches again, so
		// the reconnect's fetch is what decides over the newest edit.
		logLocalCommitTwice(cfg, err)
		return err
	}
	// refuse-by-default: guards handleHubRequestWithDrift -- transient fetch error is logged and retried on the next change or connection; keep the session open.
	if err != nil {
		logger().Warn("this device could not fetch its configuration after its hub announced a change, "+
			"so it keeps the configuration it runs and fetches again at the hub's next change or the next connection; "+
			"check that the hub is reachable and answers with: ze doctor",
			"code", codeHubUnreachable, "name", cfg.Name, "server", cfg.Server, "error", err, "fix", "ze doctor")
		return nil
	}
	if outcome.hasHubReply && sampler != nil {
		sampler.UpdateInterval(outcome.sampleIntervalSeconds)
	}
	return nil
}

// FetchConfig sends a config-fetch RPC to the hub and returns the response.
// version is the client's current config hash (empty on first boot). The
// round-trip is bounded by configFetchTimeout even when ctx carries no
// deadline. A shorter deadline ctx carries still ends it first, so a caller
// MUST NOT derive ctx from a bound it set for another step, as first boot's
// connect bound (cmd/ze fetchInitialConfig).
func FetchConfig(ctx context.Context, mc *rpc.MuxConn, version string) (fleet.ConfigFetchResponse, error) {
	return fetchConfigWithin(ctx, mc, version, "", "", configFetchTimeout)
}

// fetchConfigWithin is the one config-fetch of this package, bounded by
// bound, as SendConfigAck bounds the ack: FetchConfig passes
// configFetchTimeout, and fetchAgainstBaseline the client's fetchBound. An
// answer that arrives after the bound finds no pending call and is dropped
// (rpc MuxConn), so it is never applied.
func fetchConfigWithin(ctx context.Context, mc *rpc.MuxConn, version, chainHash, resync string, bound time.Duration) (fleet.ConfigFetchResponse, error) {
	req := fleet.ConfigFetchRequest{
		Version:   version,
		ChainHash: chainHash,
		Resync:    resync,
	}

	fetchCtx, cancel := context.WithTimeout(ctx, bound)
	defer cancel()
	result, err := mc.CallRPC(fetchCtx, fleet.VerbConfigFetch, req)
	if err != nil {
		return fleet.ConfigFetchResponse{}, fmt.Errorf("config-fetch RPC: %w", err)
	}

	var resp fleet.ConfigFetchResponse
	if err := json.Unmarshal(result, &resp); err != nil {
		return fleet.ConfigFetchResponse{}, fmt.Errorf("unmarshal config-fetch response: %w", err)
	}

	return resp, nil
}

// SendConfigAck sends a config-ack RPC to the hub confirming receipt. The
// round-trip is bounded by configAckTimeout, whatever deadline ctx carries,
// and Error is bounded to fleet.AckErrorMax bytes.
func SendConfigAck(ctx context.Context, mc *rpc.MuxConn, ack fleet.ConfigAck) error {
	_, err := sendConfigAckReply(ctx, mc, ack)
	return err
}

func sendConfigAckReply(ctx context.Context, mc *rpc.MuxConn, ack fleet.ConfigAck) (fleet.ManagedReply, error) {
	ack.Error = fleet.BoundAckError(ack.Error)
	ackCtx, cancel := context.WithTimeout(ctx, configAckTimeout)
	defer cancel()
	raw, err := mc.CallRPC(ackCtx, fleet.VerbConfigAck, ack)
	if err != nil {
		return fleet.ManagedReply{}, fmt.Errorf("config-ack RPC: %w", err)
	}
	reply, err := fleet.DecodeManagedReply(raw)
	if err != nil {
		return fleet.ManagedReply{}, fmt.Errorf("config-ack reply: %w", err)
	}
	return reply, nil
}

// sendPing sends a ping RPC to the hub carrying the client's current SHA-1
// chain head with a bounded per-ping timeout and decodes the hub's reply
// (tolerating an empty legacy ok reply).
func sendPing(ctx context.Context, mc *rpc.MuxConn, chainHash string, bound time.Duration) (fleet.ManagedReply, error) {
	if bound <= 0 {
		bound = pingTimeout
	}
	callCtx, cancel := context.WithTimeout(ctx, bound)
	defer cancel()
	raw, err := mc.CallRPC(callCtx, fleet.VerbPing, fleet.PingRequest{ChainHash: chainHash})
	if err != nil {
		return fleet.ManagedReply{}, fmt.Errorf("ping RPC: %w", err)
	}
	reply, err := fleet.DecodeManagedReply(raw)
	if err != nil {
		return fleet.ManagedReply{}, fmt.Errorf("ping reply: %w", err)
	}
	return reply, nil
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
