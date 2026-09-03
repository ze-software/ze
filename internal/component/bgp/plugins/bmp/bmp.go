// RFC: rfc/short/rfc9069.md
// Design: docs/architecture/core-design.md -- BMP plugin lifecycle
//
// Related: header.go -- wire format encode/decode
// Related: tlv.go -- TLV encode/decode
// Related: msg.go -- message type encode/decode
// Detail: bmp_events.go -- reactor events turned into BMP sender messages
// Detail: sender_config.go -- the sender configuration the callbacks below install

package bmp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"hash/fnv"
	"io"
	"log/slog"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

var errPluginNotInitialized = errors.New("plugin not initialized")

// Route-monitoring policy values (the YANG leaf's enum). Named constants rather
// than repeated literals so a typo in one arm of the dispatch cannot silently
// disable a direction.
const (
	policyPrePolicy  = "pre-policy"  // Route Monitoring for received UPDATEs only
	policyPostPolicy = "post-policy" // Route Monitoring for sent UPDATEs only
	policyAll        = "all"         // both directions
)

// maxBMPMsgSize is the upper bound on a single BMP message.
// BGP max (4096) + BMP framing (48) with generous headroom for TLVs.
const maxBMPMsgSize = 65535

// sessionReadDeadline is the read deadline for receiver sessions.
// Ensures sessions are interruptible on shutdown.
const sessionReadDeadline = 30 * time.Second

// maxDedupPerPeer caps the dedup hash set per peer to bound memory.
// A full Internet table is ~1M prefixes; 100k covers realistic churn.
const maxDedupPerPeer = 100_000

// yangTrue is the string form the YANG config tree delivers for a boolean
// leaf set to true (all config values arrive as strings).
const yangTrue = "true"

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

// receiverConfig holds parsed receiver configuration from environment { bmp { ... } }.
// YANG config tree delivers all values as strings (including booleans and numbers).
// YANG list with key is delivered as a map keyed by the key value.
type receiverConfig struct {
	Enabled     string                    `json:"enabled"`
	Servers     map[string]listenerConfig `json:"server"`
	MaxSessions string                    `json:"max-sessions"`
	RouteAction string                    `json:"route-action"`
}

type listenerConfig struct {
	IP   string `json:"ip"`
	Port string `json:"port"`
}

// The two configuration roots BMP reads, and the protocol name it injects
// routes and withdrawals under.
const (
	configRootBGP         = "bgp"
	configRootEnvironment = "environment"
	protocolBMP           = "bmp"
)

// environmentSection wraps the full environment config section.
// ExtractConfigSubtree returns {"environment": {"bmp": {...}}}, so we need
// two levels of wrapping.
type environmentSection struct {
	Environment *struct {
		BMP *receiverConfig `json:"bmp"`
	} `json:"environment"`
}

// bgpSenderSection wraps the full bgp config section.
// ExtractConfigSubtree returns {"bgp": {"bmp": {"sender": {...}}}}.
//
// The two leaves outside the BMP subtree are the router's own BGP identity, and
// RFC 9069 Section 5.1 requires the Loc-RIB emulated peer to describe itself
// with them: "Peer Autonomous System (AS): Set to the primary router BGP
// autonomous system number (ASN)", and "Peer BGP ID: Set the ID to the
// router-id of the VRF instance if VRF is used; otherwise, set to the global
// instance router-id." Both are `mandatory true` under `container bgp`
// (internal/component/bgp/yang/ze-bgp-conf.yang), and the plugin is handed that
// whole container already (configRootBGP), so they arrive with the sender
// subtree rather than needing a second config source.
//
// Every scalar the config tree delivers is a JSON string, numbers included,
// which is why the ASN is a string here beside the dotted-quad router-id.
type bgpSenderSection struct {
	BGP *struct {
		RouterID string `json:"router-id"`
		Session  *struct {
			ASN *struct {
				Local string `json:"local"`
			} `json:"asn"`
		} `json:"session"`
		BMP *struct {
			Sender *senderConfig `json:"sender"`
		} `json:"bmp"`
	} `json:"bgp"`
}

// openPair caches the actual BGP OPEN PDU bytes for a peer.
// Populated by OPEN message events, consumed by Peer Up on state change.
// RFC 7854 Section 4.10: Peer Up MUST include sent and received OPEN PDUs.
type openPair struct {
	sent     []byte // complete BGP OPEN (marker + length + type + body)
	received []byte // complete BGP OPEN (marker + length + type + body)
}

// dumpScope marks a full-table Loc-RIB replay this plugin requested. session
// names the one collector session the dump is for; nil session means every
// connected session (the dump requested when Loc-RIB monitoring starts, which
// is addressed to whoever is already connected).
type dumpScope struct {
	session *senderSession

	// replayID is the correlation token this dump put on its replay-request.
	// The BGP RIB echoes it verbatim onto every batch it produces in answer
	// (rib_bestchange.go:1202), so `batch.ReplayID == scope.replayID` answers
	// "is this batch MINE" exactly, rather than the weaker "is a dump of mine
	// in flight" -- which claimed a replay somebody else asked for.
	replayID uint64

	// closed records the families whose batch this dump closed with an
	// End-of-RIB marker, so closeDumpFamilies can send markers for exactly the
	// families the RIB stayed silent about. Guarded by mu: the dump is
	// delivered synchronously on the emitting goroutine, but a batch can reach
	// handleBestChange from ANOTHER producer's goroutine while this scope is
	// published, so the map must not be written unsynchronized. (That batch no
	// longer WRITES here -- the replayID check rejects it first -- but the lock
	// is what makes that ordering safe rather than lucky.)
	mu     sync.Mutex
	closed map[family.Family]bool
}

// noteClosed records that fam's dump was closed with an End-of-RIB marker.
func (d *dumpScope) noteClosed(fam family.Family) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed == nil {
		d.closed = make(map[family.Family]bool, 2)
	}
	d.closed[fam] = true
}

// unclosed returns the subset of want this dump did NOT close with an
// End-of-RIB marker. RFC 4724 Section 4 requires the marker "once it completes
// the initial routing update (including the case when there is no update to
// send) for an address family", so a family the RIB stayed silent about is
// exactly a family that still owes a marker -- not one to skip.
func (d *dumpScope) unclosed(want []family.Family) []family.Family {
	d.mu.Lock()
	defer d.mu.Unlock()
	missing := make([]family.Family, 0, len(want))
	for _, fam := range want {
		if !d.closed[fam] {
			missing = append(missing, fam)
		}
	}
	return missing
}

// peerUpState is everything the Peer Up message for one established BGP peer
// needs. It is recorded when the peer comes up and dropped when it goes down,
// so a collector that connects (or reconnects) later can be told about every
// peer that is up right now -- a BMP session carries no state across TCP
// connections, and RFC 7854 Section 4.10 Route Monitoring only makes sense to a
// collector that has seen the peer's Peer Up first.
//
// The timestamp inside peer is the moment the peer came up, not the moment the
// Peer Up is (re)sent: it describes the event, per RFC 7854 Section 4.2.
type peerUpState struct {
	peer       PeerHeader
	localAddr  [16]byte
	localPort  uint16
	remotePort uint16
	sentOpen   []byte
	recvOpen   []byte
}

// BMPPlugin implements the bgp-bmp plugin.
// It manages both receiver (TCP listener for inbound BMP) and
// sender (outbound TCP to collectors) functionality.
//
// Caller MUST close stopCh and call stopListeners when done.
type BMPPlugin struct {
	plugin *sdk.Plugin
	mu     sync.RWMutex
	state  *bmpState

	// Receiver state.
	listeners []net.Listener
	sessions  sync.WaitGroup

	// Sender state. All three are protected by mu: the sender set and the two
	// config leaves are written on the configure path and read on the plugin's
	// event-delivery goroutine, which snapshots them together (see
	// handleStructuredEvent).
	senders            []*senderSession
	routeMonitorPolicy string // one of policyPrePolicy, policyPostPolicy, policyAll
	routeMirroring     bool

	// Loc-RIB monitoring state (RFC 9069, PeerType=3). locRIBUnsub is the
	// best-change EventBus unsubscribe (nil when not subscribed). locRIBUp
	// records that Loc-RIB monitoring has been announced to at least one
	// collector, which is what gates the Loc-RIB Peer Down on shutdown; the
	// once-per-session guard for the Peer Up itself lives on the session
	// (senderSession.locRIBUpSent), because each BMP session needs its own.
	// Protected by mu.
	locRIBUnsub func()
	locRIBUp    bool

	// identity is the router's own ASN and router-id, read from the `bgp`
	// config section (parseSenderConfig, installed by applySenderConfig). It is
	// what RFC 9069 Section 5.1 requires the Loc-RIB emulated peer's per-peer
	// header to carry, and what the fabricated OPEN of its Peer Up describes
	// itself with. Protected by mu.
	//
	// A POINTER, and nil until a `bgp` section has been parsed: RFC 9069
	// Section 1.1 names the case a router monitors a Loc-RIB with no BGP peer
	// up at all ("VRF tables with no peers"), so "ze does not know its own
	// identity yet" has to be distinguishable from an identity whose fields
	// read zero. A zero PeerAS and a zero Peer BGP ID are what a collector
	// would take for the router's real answer (ai/rules/principles.md), and
	// RFC 6286 Section 2.1 makes a zero BGP Identifier invalid outright.
	identity *localIdentity

	// openCache stores real OPEN PDUs per peer for Peer Up messages.
	// Key is peer address string. Populated by OPEN message events,
	// consumed by state events. Protected by mu.
	openCache map[string]*openPair

	// dumpScope describes the full-table Loc-RIB replay THIS plugin currently
	// has in flight, published only for the duration of its replay-request Emit.
	// nil means the plugin did not ask for the replay being delivered -- another
	// subscriber did (internal/component/sysrib/sysrib.go emits on the same
	// broadcast handle), and such a batch must not be mistaken for this
	// plugin's own dump.
	//
	// One pointer rather than a target plus a flag: the two questions ("is this
	// ours" and "who is it for") are answered from a single atomic load, so a
	// dump that ends mid-batch cannot leave a reader holding half of each.
	// dumpMu serializes the whole publish/emit/retract window so two collectors
	// reconnecting together cannot overwrite each other's scope.
	dumpMu    sync.Mutex
	dumpScope atomic.Pointer[dumpScope]

	// peerUps holds the Peer Up state of every currently established BGP peer,
	// keyed by peer address. Written on peer up/down, read when a collector
	// connects so the new BMP session is told about the peers that came up
	// before it. Protected by mu.
	peerUps map[string]*peerUpState

	// dedupState tracks per-peer UPDATE body hashes for Route Monitoring dedup.
	// Key: peer address. Value: set of FNV-64 hashes of RawBytes.
	// Cleared per-peer on peer-down. Protected by mu.
	// Capped at maxDedupPerPeer entries per peer to bound memory.
	dedupState map[string]map[uint64]struct{}

	// dedupHasher is pre-allocated FNV-64a hasher, reused via Reset().
	// Safe without locking because it is touched only from handleSenderUpdate,
	// and structured events reach a plugin on ONE delivery goroutine
	// (internal/component/plugin/process/delivery.go startDeliveryLocked starts
	// a single deliveryLoop per process). Nothing else may use it.
	dedupHasher hash.Hash64

	// stopCh signals all background goroutines to stop.
	stopCh chan struct{}
}

// runBMPPlugin is the in-process entry point for the bgp-bmp plugin.
func runBMPPlugin(conn net.Conn) int {
	logger().Debug("bgp-bmp plugin starting")

	p := sdk.NewWithConn("bgp-bmp", conn)
	defer closeLog(p, "plugin")

	bp := &BMPPlugin{
		plugin:      p,
		state:       newBMPState(),
		openCache:   make(map[string]*openPair),
		peerUps:     make(map[string]*peerUpState),
		dedupState:  make(map[string]map[uint64]struct{}),
		dedupHasher: fnv.New64a(),
		stopCh:      make(chan struct{}),
	}

	defer func() {
		close(bp.stopCh)
		bp.stopLocRIB()         // unsubscribe from best-change
		bp.sendLocRIBPeerDown() // RFC 9069 Peer Down, before senders close
		bp.stopSenders()
		bp.stopListeners()
		bp.sessions.Wait()
	}()

	bp.registerCallbacks()

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	err := p.Run(ctx, sdk.Registration{
		Commands: []sdk.CommandDecl{
			{Name: "show bmp sessions", Description: "Show BMP receiver sessions"},
			{Name: "show bmp peers", Description: "Show monitored BGP peers"},
			{Name: "show bmp collectors", Description: "Show BMP sender collector status"},
			{Name: "show bmp rib", Description: "Show BMP-monitored routes"},
		},
		WantsConfig: []string{configRootBGP, configRootEnvironment},
	})
	if err != nil {
		logger().Error("bgp-bmp plugin failed", "error", err)
		return 1
	}

	return 0
}

// registerCallbacks wires every engine callback the BMP plugin serves onto its
// SDK plugin, and declares the startup subscriptions the sender feeds on.
// Called once, before Run enters the startup handshake.
//
// Separated from runBMPPlugin so a test can register the callbacks on a
// BMPPlugin it built itself and then drive one of them through the SDK the way
// the engine does. bp.plugin MUST be set before this is called.
func (bp *BMPPlugin) registerCallbacks() {
	p := bp.plugin

	// Reload state, carried between the config callbacks. Stage-2 configure and
	// all three reload callbacks are served on one goroutine (Plugin.Run, then
	// eventLoop or bridgeEventLoop), so these are captured locals rather than
	// plugin state, as they are for every other plugin that reloads
	// (internal/plugins/ddos/detect/register.go).
	//
	// currentSender is the configuration in force. It starts at the default
	// configuration, which is what the plugin boots with: no collector, no Route
	// Mirroring, no Loc-RIB monitoring, so a rollback always has one to restore.
	// It is also the configuration the first arriving one is COMPARED against,
	// which is why it carries the YANG defaults rather than empty strings.
	//
	// pendingSender is what verify parsed and apply installs. replacedSender is
	// what apply displaced, for a rollback to put back. Verify clears both, and
	// verify runs for every participant before any apply runs for one, so
	// replacedSender is non-nil only when THIS transaction's apply ran.
	currentSender := defaultSenderConfig()
	var pendingSender, replacedSender *senderConfig

	p.OnExecuteCommand(func(serial, command string, args []string, peer string) (string, any, error) {
		return bp.handleCommand(command)
	})

	// Structured event handler: receives peer state changes and UPDATE messages
	// from the reactor via DirectBridge. Used by the sender to stream BMP to collectors.
	p.OnStructuredEvent(func(events []any) error {
		for _, event := range events {
			se, ok := event.(*rpc.StructuredEvent)
			if !ok || se.PeerAddress == "" {
				continue
			}
			bp.handleStructuredEvent(se)
		}
		return nil
	})

	// Subscribe to peer state (up/down), received/sent updates, and OPEN messages.
	// OPEN subscriptions cache real OPEN PDUs for Peer Up (RFC 7854 S4.10).
	// Notification/keepalive/refresh subscriptions support Route Mirroring (RFC 7854 S4.7).
	// All subscribed unconditionally: config loads after subscriptions, and
	// route-mirroring can be toggled via config reload. Cost is one type-check
	// per event when mirroring is disabled.
	p.SetStartupSubscriptions([]string{
		"state",
		"update direction received", "update direction sent",
		"open direction received", "open direction sent",
		"notification direction received", "notification direction sent",
		"keepalive direction received", "keepalive direction sent",
		"refresh direction received", "refresh direction sent",
	}, nil, "full")

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		for _, section := range sections {
			switch section.Root {
			case configRootEnvironment:
				rcv, err := parseReceiverConfig(section.Data)
				if err != nil {
					logger().Error("bmp: receiver config parse failed", "error", err)
					return err
				}
				if rcv.Enabled == yangTrue && len(rcv.Servers) > 0 {
					bp.startReceiver(rcv)
				}
			case configRootBGP:
				snd, err := parseSenderConfig(section.Data)
				if err != nil {
					logger().Error("bmp: sender config parse failed", "error", err)
					return err
				}
				bp.applySenderConfig(currentSender, snd)
				currentSender = snd
			}
		}
		return nil
	})

	// A reload never re-delivers Stage-2 configure: OnConfigure is served once,
	// by serveOne in Plugin.Run, and its handler is never filed in the SDK's
	// dispatch map (OnConfigure, pkg/plugin/sdk/sdk_callbacks.go). The reload
	// rail is config-verify, config-apply and config-rollback, so all three are
	// wired below and both rails end in applySenderConfig.
	//
	// Registering them is what makes a reload reach the sender at all: the SDK
	// answers each of the three with an accept-and-do-nothing default when no
	// handler is filed (initCallbackDefaults, pkg/plugin/sdk/sdk_callbacks.go),
	// so an unregistered plugin reports every reload applied and applies none.
	//
	// Verify carries the whole candidate `bgp` subtree and apply carries only
	// the diff, so the parsed configuration is stashed here for apply to
	// install. Rejecting a subtree that does not parse is what verify is for:
	// the transaction aborts before any BMP session is bounced.
	p.OnConfigVerify(func(sections []sdk.ConfigSection) error {
		pendingSender = nil
		replacedSender = nil
		for _, section := range sections {
			if section.Root != configRootBGP {
				continue
			}
			snd, err := parseSenderConfig(section.Data)
			if err != nil {
				logger().Error("bmp: sender config parse failed", "error", err)
				return err
			}
			pendingSender = snd
		}
		return nil
	})

	// A reload that changed no `bgp` section leaves nothing pending, so the
	// collector sessions are left alone. Anything else MIGHT have changed the
	// configuration those sessions run under: the `bgp` root carries every
	// neighbor and policy the operator edits, so the candidate is handed to
	// applySenderConfig beside the one in force and the two are compared
	// there. RFC 8671 Section 7.2 owes a bounce to a change in behavior, and
	// most of what arrives here is not one.
	p.OnConfigApply(func(_ []sdk.ConfigDiffSection) error {
		if pendingSender == nil {
			return nil
		}
		replacedSender = currentSender
		applied := pendingSender
		pendingSender = nil
		bp.applySenderConfig(currentSender, applied)
		currentSender = applied
		return nil
	})

	// A rolled-back reload leaves the router on the configuration it already
	// had, so the collectors are put back onto that one. Without this the BMP
	// feed would describe a configuration the operator's commit never took, and
	// nothing would correct it until the next reload. A rollback of a
	// transaction whose apply changed nothing restores the same configuration,
	// which applySenderConfig reads as no change and acts on as one.
	p.OnConfigRollback(func(_ string) error {
		if replacedSender == nil {
			return nil
		}
		restored := replacedSender
		replacedSender = nil
		bp.applySenderConfig(currentSender, restored)
		currentSender = restored
		return nil
	})
}

// closeLog closes c and logs any error. Used in deferred cleanup.
func closeLog(c interface{ Close() error }, what string) {
	if err := c.Close(); err != nil {
		logger().Debug("bmp: close failed", "what", what, "error", err)
	}
}

// parseReceiverConfig extracts BMP receiver config from the environment section JSON.
// The JSON is {"environment": {"bmp": {...}}} (wrapped by ExtractConfigSubtree).
func parseReceiverConfig(data string) (*receiverConfig, error) {
	var sec environmentSection
	if err := json.Unmarshal([]byte(data), &sec); err != nil {
		return nil, fmt.Errorf("bmp receiver config: %w", err)
	}
	if sec.Environment == nil || sec.Environment.BMP == nil {
		return &receiverConfig{}, nil
	}
	return sec.Environment.BMP, nil
}

// startReceiver starts TCP listeners for the BMP receiver.
func (bp *BMPPlugin) startReceiver(cfg *receiverConfig) {
	bp.mu.Lock()
	defer bp.mu.Unlock()

	if cfg.RouteAction == "redistribute" {
		logger().Warn("bmp: route-action redistribute is not yet implemented, using monitor")
	}
	logger().Info("bmp: receiver route-action: monitor (BMP RIB for visibility)")
	maxSess := parseUint16(cfg.MaxSessions, 100)
	for _, srv := range cfg.Servers {
		addr := net.JoinHostPort(srv.IP, srv.Port)
		var lc net.ListenConfig
		ln, err := lc.Listen(context.Background(), "tcp", addr)
		if err != nil {
			logger().Error("bmp: listener bind failed", "address", addr, "error", err)
			continue
		}
		bp.listeners = append(bp.listeners, ln)
		logger().Info("bmp: receiver listening", "address", addr)

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
	terminated := false
	bp.state.addRouter(remote)
	defer bp.state.removeRouter(remote)
	defer func() {
		if bp.plugin == nil || terminated {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, _, err := bp.plugin.DispatchCommandArgs(ctx, "request bgp rib withdraw-router", []string{protocolBMP, remote}, "")
		if err != nil {
			logger().Debug("bmp: withdraw-router failed on session end", "remote", remote, "error", err)
		}
	}()
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

		ch, _, err := decodeCommonHeader(headerBuf, 0)
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

		if _, ok := msg.(*Termination); ok {
			terminated = true
		}
		bp.processMessage(remote, msg)

		// RFC 7854 Section 4.5: "Likewise, the monitoring station MUST close the
		// TCP session after receiving a termination message." The message is
		// processed first, so its reason TLVs are recorded, and the deferred
		// close then runs. Waiting for the router to close instead lets a router
		// that sends a termination and then holds the connection open keep a
		// session slot for as long as it likes.
		if terminated {
			return
		}
	}
}

// handleCommand dispatches BMP CLI commands to the appropriate handler.
func (bp *BMPPlugin) handleCommand(command string) (string, any, error) {
	switch command {
	case "show bmp sessions":
		return bp.state.sessionsCommand()
	case "show bmp peers":
		return bp.state.peersCommand()
	case "show bmp collectors":
		bp.mu.RLock()
		senders := bp.senders
		bp.mu.RUnlock()
		return bp.state.collectorsCommand(senders)
	case "show bmp rib":
		if bp.plugin == nil {
			return statusError, "", errPluginNotInitialized
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		status, data, err := bp.plugin.DispatchCommandArgs(ctx, "show bgp rib protocol", []string{protocolBMP}, "")
		if err != nil {
			return statusError, "", err
		}
		return status, data, nil
	}
	return statusError, "", fmt.Errorf("unknown command: %s", command)
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
	case *statisticsReport:
		bp.processStatisticsReport(remote, m)
	case *routeMirroring:
		bp.processRouteMirroring(remote, m)
	}
}

func (bp *BMPPlugin) processInitiation(remote string, m *Initiation) {
	var sysName, sysDescr string
	for _, tlv := range m.TLVs {
		switch tlv.Type { //nolint:exhaustive // RFC 7854: unknown TLV types are silently ignored
		case InitTLVSysName:
			sysName = string(tlv.Value)
			logger().Info("bmp: initiation", "remote", remote, "sysName", sysName)
		case InitTLVSysDescr:
			sysDescr = string(tlv.Value)
			logger().Info("bmp: initiation", "remote", remote, "sysDescr", sysDescr)
		case InitTLVString:
			logger().Info("bmp: initiation", "remote", remote, "message", string(tlv.Value))
		}
	}
	bp.state.setRouterInfo(remote, sysName, sysDescr)
}

func (bp *BMPPlugin) processTermination(remote string, _ *Termination) {
	logger().Info("bmp: termination received", "remote", remote)

	if bp.plugin != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, _, err := bp.plugin.DispatchCommandArgs(ctx, "request bgp rib withdraw-router", []string{protocolBMP, remote}, "")
		if err != nil {
			logger().Debug("bmp: withdraw-router failed on termination", "remote", remote, "error", err)
		}
	}
}

// bmpCompositeKey builds the composite peer identity used as the key in
// ribInPool[bmpProtocolID].
//
// For a monitored BGP peer (RFC 7854 peer types 0 to 2) the identity is
// "<router>:<peer-address>". A Loc-RIB Instance Peer has no peer address to be
// identified by -- RFC 9069 Section 5.1 says "Peer Address: Zero-filled. The
// remote peer address is not applicable" -- so every Peer Type 3 peer of one
// router produced the same key "<router>:0.0.0.0", and two Loc-RIB instances
// shared one pool: a Peer Down for either withdrew the routes of both.
//
// RFC 9069 Section 6.1.1 names the fields that DO identify one: "The BMP
// receiver identifies the Loc-RIB by the peer header distinguisher and BGP ID."
// So a Loc-RIB peer keys on those two.
func bmpCompositeKey(router string, ph PeerHeader) string {
	if ph.PeerType == PeerTypeLocRIB {
		var b textbuf.Buffer
		return b.Str(router).Str(":loc-rib:").Uint(ph.Distinguisher).Byte(':').
			Uint(uint64(ph.PeerBGPID)).String()
	}
	return router + ":" + peerAddressString(ph)
}

// peerAddressString formats the PeerHeader address as a string.
func peerAddressString(ph PeerHeader) string {
	if ph.IsIPv6() {
		return net.IP(ph.Address[:]).String()
	}
	return net.IP(ph.Address[12:16]).String()
}

// processPeerUp records a peer a monitored router reports as up, and the
// address families that peer carries.
//
// RFC 9069 Section 6.1.1: "Each emulated peer instance MUST send a Peer Up with
// the OPEN message indicating the address family capabilities. A BMP receiver
// MUST process these capabilities to know which peer belongs to which address
// family." The families are read from the sent OPEN, which RFC 9069 Section 5.2
// makes the authoritative one for a Loc-RIB instance peer ("Received OPEN
// Message: Repeat of the same sent OPEN message"), and from the received OPEN
// for a monitored BGP peer, whose sent OPEN says what the MONITORED router
// offered rather than what the peer announces.
//
// An OPEN that does not parse, or that advertises no Multiprotocol capability,
// leaves the set EMPTY. It is never defaulted to IPv4 unicast: a guessed family
// is indistinguishable from an advertised one at every reader
// (ai/rules/principles.md), and the association this requirement exists to
// record would then be an invention.
func (bp *BMPPlugin) processPeerUp(remote string, m *PeerUp) {
	open := m.SentOpenMsg
	if m.Peer.PeerType != PeerTypeLocRIB {
		open = m.ReceivedOpenMsg
	}
	families := openMultiprotocolFamilies(open)
	bp.state.peerUp(remote, m.Peer, families)
	logger().Info("bmp: peer up",
		"remote", remote,
		"peer-as", m.Peer.PeerAS,
		"peer-bgp-id", fmt.Sprintf("%08x", m.Peer.PeerBGPID),
		"local-port", m.LocalPort,
		"remote-port", m.RemotePort,
		"families", families,
	)
}

// openMultiprotocolFamilies answers the address families a BGP OPEN PDU
// advertises, in the order the capabilities appear.
//
// RFC 4760 Section 8 defines the Multiprotocol Extensions capability as the
// declaration of an <AFI, SAFI> pair, so those capabilities ARE the peer's
// family list. A PDU that does not parse as an OPEN, and one carrying no such
// capability, both answer an empty list rather than a default.
func openMultiprotocolFamilies(open []byte) []family.Family {
	if len(open) < message.HeaderLen {
		return nil
	}
	parsed, err := message.UnpackOpen(open[message.HeaderLen:])
	if err != nil {
		return nil
	}
	caps, err := capability.ParseFromOptionalParams(parsed.OptionalParams, parsed.ExtendedParams)
	if err != nil {
		return nil
	}
	var out []family.Family
	for _, capa := range caps {
		multi, ok := capa.(*capability.Multiprotocol)
		if !ok {
			continue
		}
		out = append(out, family.Family{AFI: multi.AFI, SAFI: multi.SAFI})
	}
	return out
}

func (bp *BMPPlugin) processPeerDown(remote string, m *PeerDown) {
	bp.state.peerDown(remote, m.Peer, m.Reason)
	logger().Info("bmp: peer down",
		"remote", remote,
		"peer-as", m.Peer.PeerAS,
		"reason", m.Reason,
	)

	if bp.plugin != nil {
		peerKey := bmpCompositeKey(remote, m.Peer)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_, _, err := bp.plugin.DispatchCommandArgs(ctx, "request bgp rib withdraw-protocol", []string{protocolBMP, peerKey}, "")
		if err != nil {
			logger().Debug("bmp: withdraw-protocol failed on peer down", "peer", peerKey, "error", err)
		}
	}
}

func (bp *BMPPlugin) processRouteMonitoring(remote string, m *RouteMonitoring) {
	if bp.plugin == nil {
		return
	}
	if len(m.BGPUpdate) < bgpHeaderSize {
		logger().Debug("bmp: route monitoring UPDATE too short", "remote", remote, "len", len(m.BGPUpdate))
		return
	}

	updateBody := m.BGPUpdate[bgpHeaderSize:]
	peerKey := bmpCompositeKey(remote, m.Peer)

	if err := bp.plugin.InjectWireRoute(protocolBMP, peerKey, updateBody); err != nil {
		logger().Debug("bmp: inject-wire-route failed",
			"remote", remote,
			"peer", peerKey,
			"error", err,
		)
	}
}

func (bp *BMPPlugin) processStatisticsReport(remote string, m *statisticsReport) {
	logger().Debug("bmp: statistics report",
		"remote", remote,
		"peer-as", m.Peer.PeerAS,
		"stats-count", len(m.Stats),
	)
}

func (bp *BMPPlugin) processRouteMirroring(remote string, m *routeMirroring) {
	logger().Debug("bmp: route mirroring",
		"remote", remote,
		"peer-as", m.Peer.PeerAS,
		"tlv-count", len(m.TLVs),
	)
}

// parseUint16 parses a string to uint16, returning def on error or empty input.
func parseUint16(s string, def uint16) uint16 {
	if s == "" {
		return def
	}
	v, err := strconv.ParseUint(s, 10, 16)
	if err != nil {
		return def
	}
	return uint16(v)
}
