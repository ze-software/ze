// Design: docs/research/l2tpv2-ze-integration.md -- l2tp-pool plugin lifecycle
// Related: l2tppool.go -- atomic logger, Name constant
// Related: pool.go -- ipv4Pool bitmap allocation

package l2tppool

import (
	"fmt"
	"net"
	"net/netip"
	"os"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/component/l2tp"
	"github.com/ze-software/ze/internal/component/l2tp/plugins/pool/yang"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/l2tp/subscriber"
	subevents "github.com/ze-software/ze/internal/component/l2tp/subscriber/events"
	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/dnssource"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/internal/core/slogutil"
	"github.com/ze-software/ze/internal/core/textbuf"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
	"github.com/ze-software/ze/pkg/ze"
)

var poolInstance = &poolPlugin{}

type poolPlugin struct {
	mu         sync.RWMutex
	pool       *ipv4Pool
	namedPools map[string]*ipv4Pool

	v6mu         sync.RWMutex
	v6pool       *ipv6PrefixPool
	v6namedPools map[string]*ipv6PrefixPool

	busMu sync.Mutex
	bus   ze.EventBus
	unsub []func()

	// learned is the newest system-dns snapshot, which the received source
	// of every subscriber list reads when a session asks for its address.
	learned dnssource.Follower
	// published is the body of the last selection publication sent
	// (publishPools), so an unchanged set is not sent again.
	published poolPublished

	// Both maps are private: storeAddressLocked and carryIPv4 write only
	// string/sessionAddr pairs; handlePrefix and carryIPv6 write only
	// string/sessionPrefix pairs. Peer data supplies fields, never Go types.
	// A failed assertion while reading one is therefore an internal defect.
	sessionAddrs    sync.Map // canonical subscriber ID -> sessionAddr (IPv4)
	sessionPrefixes sync.Map // canonical L2TP subscriber ID -> sessionPrefix (IPv6)
	// mu serializes address ownership with pool replacement. Tokens name
	// individual answers, so a late failed answer cannot free a successor.
	nextAllocationID   uint64
	duplicateAddresses map[netip.Addr]uint32
}

type sessionPrefix struct {
	prefix    netip.Prefix
	fromPool  bool
	poolName  string
	tunnelID  uint16
	sessionID uint16
}

type sessionAddr struct {
	tunnelID     uint16
	sessionID    uint16
	addr         netip.Addr
	fromPool     bool   // true when a current pool reserves this address
	poolName     string // "" = default pool, non-empty = named pool
	allocationID uint64
	local        netip.Addr
	dnsPrimary   netip.Addr
	dnsSecondary netip.Addr
}

const invalidIPv4RecordPanic = "BUG: pool IPv4 record is not a sessionAddr"

// configRootL2TP is the YANG container this plugin reads.
const configRootL2TP = "l2tp"

func init() {
	l2tp.RegisterPoolHandler(poolInstance.handle)
	l2tp.RegisterPrefixHandler(poolInstance.handlePrefix)
	l2tp.RegisterPrefixReleaser(poolInstance.releasePrefix)

	reg := registry.Registration{
		Name:                    Name,
		Description:             "IPv4 address and IPv6 prefix pool for L2TP PPP sessions",
		Features:                "yang",
		YANG:                    yang.ZeL2TPPoolConfYANG,
		ConfigRoots:             []string{configRootL2TP},
		InProcessConfigVerifier: verifyPoolConfig,
		RunEngine:               runPlugin,
		Commands:                commandDecls(),
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
		},
		ConfigureEventBus: func(eb ze.EventBus) {
			poolInstance.setEventBus(eb)
		},
	}
	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		cfg.ConfigLogger = func(level string) {
			setLogger(slogutil.PluginLogger(reg.Name, level))
		}
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		fmt.Fprintf(os.Stderr, "%s: registration failed: %v\n", Name, err)
		os.Exit(1)
	}
}

func verifyPoolConfig(sections []sdk.ConfigSection) error {
	_, _, err := parsePoolSections(sections)
	return err
}

func (p *poolPlugin) setEventBus(eb ze.EventBus) {
	p.busMu.Lock()
	defer p.busMu.Unlock()
	for _, unsub := range p.unsub {
		unsub()
	}
	p.bus = eb
	p.unsub = []func(){
		subevents.SessionDown.Subscribe(eb, func(payload *subevents.SessionDownPayload) {
			p.onSessionDown(payload)
		}),
		subevents.PoolAnswerUndelivered.Subscribe(eb, p.onPoolAnswerUndelivered),
		// The selection is evaluated when a session asks; a fresh snapshot
		// only changes what is published for show system dns.
		dnssource.SnapshotEvent.Subscribe(eb, func(snapshot *dnssource.Snapshot) {
			if p.learned.AcceptSnapshot(snapshot) {
				p.publishPools(eb)
			}
		}),
	}
	// A new bus has no publication from this plugin: send the whole set.
	p.published.reset()
	p.publishPools(eb)
	// A snapshot emitted before the subscription is not delivered, so ask for
	// the current one: sessions that start now see the leases already held.
	if _, err := dnssource.SnapshotRequest.Emit(eb); err != nil {
		logger().Warn("l2tp-pool: learned resolvers not requested; the received source is empty until the next lease change",
			"error", err)
	}
}

// onSessionDown returns whatever the session held to the pool it came from.
//
// It reads the transport-generic subscriber topic rather than the L2TP one:
// PPPoE emits only subevents.SessionDown (pppoe.Subsystem.onSessionDown), and
// subscriberBridge.onSessionDown re-emits every L2TP teardown on the same
// topic, so one subscription covers both transports. Subscribing to
// l2tpevents.SessionDown as well would deliver each L2TP teardown twice.
//
// Safe against a repeated delivery either way: sessionAddrs.LoadAndDelete and
// sessionPrefixes.LoadAndDelete hand the entry to exactly one caller, so no
// second call reaches pool.release and no reassigned address is freed.
func (p *poolPlugin) onSessionDown(payload *subevents.SessionDownPayload) {
	if err := subscriber.RequireID(logger(), payload.Session.ID); err != nil {
		return
	}
	p.mu.Lock()
	p.releaseAddressLocked(payload.Session.ID)
	p.mu.Unlock()
	p.releasePrefixBySubscriberID(payload.Session.ID)
}

func (p *poolPlugin) handlePrefix(req l2tp.PrefixRequest) l2tp.PrefixResult {
	p.v6mu.Lock()
	defer p.v6mu.Unlock()
	key := subscriber.SessionID(subscriber.AccessL2TP, uint64(req.TunnelID), req.SessionID)
	if _, already := p.sessionPrefixes.Load(key); already {
		return l2tp.PrefixResult{OK: false, Reason: "prefix already allocated for this session"}
	}

	meta := l2tp.LoadSessionMetadata(key)
	if meta != nil && meta.DelegatedIPv6Prefix.IsValid() {
		poolName, inRange, err := p.reserveRADIUSPrefixLocked(meta.DelegatedIPv6Prefix)
		if err != nil {
			return l2tp.PrefixResult{OK: false, Reason: err.Error()}
		}
		p.sessionPrefixes.Store(
			key,
			sessionPrefix{prefix: meta.DelegatedIPv6Prefix, fromPool: inRange, poolName: poolName,
				tunnelID: req.TunnelID, sessionID: req.SessionID},
		)
		logger().Info("l2tp-pool: using RADIUS-assigned IPv6 prefix",
			"tunnel", req.TunnelID, "session", req.SessionID, "prefix", meta.DelegatedIPv6Prefix)
		return l2tp.PrefixResult{OK: true, Prefix: meta.DelegatedIPv6Prefix}
	}

	poolName := req.PoolName
	if poolName == "" && meta != nil && meta.FramedIPv6Pool != "" {
		poolName = meta.FramedIPv6Pool
	}

	var pool *ipv6PrefixPool
	if poolName == "" {
		pool = p.v6pool
	} else if p.v6namedPools != nil {
		pool = p.v6namedPools[poolName]
	}

	if pool == nil {
		return l2tp.PrefixResult{OK: false, Reason: "no IPv6 prefix pool configured"}
	}

	prefix, ok := pool.allocate()
	if !ok {
		logger().Warn("l2tp-pool: IPv6 prefix pool exhausted",
			"tunnel", req.TunnelID, "session", req.SessionID)
		return l2tp.PrefixResult{OK: false, Reason: "IPv6 prefix pool exhausted"}
	}

	p.sessionPrefixes.Store(
		key,
		sessionPrefix{prefix: prefix, fromPool: true, poolName: poolName,
			tunnelID: req.TunnelID, sessionID: req.SessionID},
	)

	logger().Info("l2tp-pool: allocated IPv6 prefix",
		"tunnel", req.TunnelID, "session", req.SessionID, "prefix", prefix)
	return l2tp.PrefixResult{OK: true, Prefix: prefix}
}

func (p *poolPlugin) releasePrefix(tunnelID, sessionID uint16) {
	p.releasePrefixBySubscriberID(subscriber.SessionID(subscriber.AccessL2TP, uint64(tunnelID), sessionID))
}

func (p *poolPlugin) releasePrefixBySubscriberID(subscriberID string) {
	if err := subscriber.RequireID(logger(), subscriberID); err != nil {
		return
	}
	p.v6mu.Lock()
	defer p.v6mu.Unlock()
	val, ok := p.sessionPrefixes.LoadAndDelete(subscriberID)
	if !ok {
		return
	}
	sp, ok := val.(sessionPrefix)
	if !ok {
		return
	}
	if !sp.fromPool {
		logger().Info("l2tp-pool: RADIUS-assigned IPv6 prefix cleared",
			"subscriber", subscriberID, "tunnel", sp.tunnelID, "session", sp.sessionID, "prefix", sp.prefix)
		return
	}
	if p.v6pool != nil {
		p.v6pool.releaseOverlap(sp.prefix)
	}
	for _, pool := range p.v6namedPools {
		pool.releaseOverlap(sp.prefix)
	}
	logger().Info("l2tp-pool: released IPv6 prefix",
		"subscriber", subscriberID, "tunnel", sp.tunnelID, "session", sp.sessionID, "prefix", sp.prefix)
}

func (p *poolPlugin) handle(req ppp.EventIPRequest) ppp.IPResponseArgs {
	if err := subscriber.RequireID(logger(), req.SubscriberID); err != nil {
		return ppp.IPResponseArgs{Accept: false, Family: req.Family, Reason: err.Error()}
	}
	if req.Family != ppp.AddressFamilyIPv4 {
		return ppp.IPResponseArgs{Accept: false, Family: req.Family, Reason: "IPv6 not supported by static pool"}
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	key := req.SubscriberID
	if stored, ok := p.sessionAddrs.Load(key); ok {
		allocation, ok := stored.(sessionAddr)
		if !ok {
			panic(invalidIPv4RecordPanic)
		}
		return allocation.answer()
	}
	// The counter never wraps into the identity of an earlier answer.
	if p.nextAllocationID == ^uint64(0) {
		return ppp.IPResponseArgs{Family: req.Family, Reason: opererr.New(
			"Subscriber address allocation identifiers are exhausted",
			"Ze cannot identify another address answer safely before restarting",
			"ze show l2tp pool").Error()}
	}
	pool := p.pool
	named := p.namedPools

	meta := l2tp.LoadSessionMetadata(key)

	var poolName string

	// Framed-Pool selects a named pool (affects gateway/DNS for both
	// pool-allocated and RADIUS-assigned IP paths).
	if meta != nil && meta.FramedPool != "" {
		var tb textbuf.Buffer
		if named == nil {
			return ppp.IPResponseArgs{Accept: false, Family: req.Family,
				Reason: tb.Str("named pool ").Str(meta.FramedPool).Str(" not configured").String()}
		}
		namedPool, ok := named[meta.FramedPool]
		if !ok {
			logger().Warn("l2tp-pool: named pool not found",
				"tunnel", req.TunnelID, "session", req.SessionID, "pool", meta.FramedPool)
			return ppp.IPResponseArgs{Accept: false, Family: req.Family,
				Reason: tb.Reset().Str("named pool ").Str(meta.FramedPool).Str(" not found").String()}
		}
		pool = namedPool
		poolName = meta.FramedPool
	}

	if pool == nil {
		return ppp.IPResponseArgs{Accept: false, Family: req.Family, Reason: "no pool configured"}
	}

	// RFC 2548 Sections 2.7.6 and 2.7.7: the radius source is the
	// Access-Accept's MS DNS servers, offered by the pool's dns-source list.
	dns := selectSubscriberDNS(pool, meta, p.learned.Records(), time.Now(), logger())

	// RFC 2865 Section 5.8: Framed-IP-Address bypasses pool allocation.
	// Uses the selected pool's gateway/DNS (named pool if Framed-Pool
	// was also set, default pool otherwise).
	if meta != nil && meta.FramedIP.IsValid() {
		reservationPool, inRange, err := p.reserveRADIUSAddressLocked(meta.FramedIP)
		if err != nil {
			return ppp.IPResponseArgs{Accept: false, Family: req.Family, Reason: err.Error()}
		}
		allocationID := p.storeAddressLocked(key, sessionAddr{addr: meta.FramedIP,
			tunnelID: req.TunnelID, sessionID: req.SessionID,
			fromPool: inRange, poolName: reservationPool,
			local: pool.gateway, dnsPrimary: dns.primary, dnsSecondary: dns.secondary})
		logger().Info("l2tp-pool: using RADIUS-assigned address",
			"tunnel", req.TunnelID, "session", req.SessionID, "address", meta.FramedIP)
		return ppp.IPResponseArgs{
			AllocationID: allocationID,
			Accept:       true,
			Family:       ppp.AddressFamilyIPv4,
			Local:        pool.gateway,
			Peer:         meta.FramedIP,
			DNSPrimary:   dns.primary,
			DNSSecondary: dns.secondary,
		}
	}

	addr, ok := pool.allocate()
	if !ok {
		logger().Warn("l2tp-pool: pool exhausted",
			"tunnel", req.TunnelID, "session", req.SessionID)
		return ppp.IPResponseArgs{Accept: false, Family: req.Family, Reason: "pool exhausted"}
	}

	allocationID := p.storeAddressLocked(key, sessionAddr{addr: addr, fromPool: true, poolName: poolName,
		tunnelID: req.TunnelID, sessionID: req.SessionID,
		local: pool.gateway, dnsPrimary: dns.primary, dnsSecondary: dns.secondary})

	logger().Info("l2tp-pool: allocated address",
		"tunnel", req.TunnelID, "session", req.SessionID, "address", addr)

	return ppp.IPResponseArgs{
		AllocationID: allocationID,
		Accept:       true,
		Family:       ppp.AddressFamilyIPv4,
		Local:        pool.gateway,
		Peer:         addr,
		DNSPrimary:   dns.primary,
		DNSSecondary: dns.secondary,
	}
}

func runPlugin(conn net.Conn) int {
	var tb textbuf.Buffer
	logger().Debug(tb.Str(Name).Str(" plugin starting (RPC)").Slice())

	p := sdk.NewWithConn(Name, conn)
	defer func() { _ = p.Close() }()

	poolInstance.configTransaction().Register(p)
	// A plugin the engine removes while the daemon runs withdraws its
	// selections, so show system dns and ze doctor stop reporting them. On
	// daemon stop no bye arrives, and the registry stops with the daemon.
	p.OnBye(func(string) error {
		poolInstance.withdraw()
		return nil
	})

	p.OnExecuteCommand(func(_, command string, _ []string, _ string) (string, any, error) {
		if command == "show l2tp pool" {
			return "done", poolInstance.showPool(), nil
		}
		return "error", "", fmt.Errorf("unknown command: %s", command)
	})

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	if err := p.Run(ctx, &sdk.Registration{
		WantsConfig:  []string{configRootL2TP},
		VerifyBudget: 1,
		ApplyBudget:  1,
		Commands:     commandDecls(),
	}); err != nil {
		logger().Error(Name+" plugin failed", "error", err)
		return 1
	}
	return 0
}

func (p *poolPlugin) showPool() any {
	result, configured := p.showPoolSnapshot()
	if !configured {
		// Marshaled once by the SDK: return a Go value, not a JSON string
		// literal (which would double-encode on the wire).
		return map[string]any{"status": "no pool configured"}
	}
	// Subscriber registry lookups MUST run after the allocation locks are released.
	// Session-down callbacks can acquire those same allocation locks.
	for index := range result.Sessions {
		session := &result.Sessions[index]
		session.Username = lookupSessionUsername(session.subscriberID)
	}
	return result
}

// showPoolSnapshot captures one installed pool set and its reservations. It
// MUST take the family locks in installation order and release both before
// the caller performs any subscriber registry lookup. Safe for concurrent use.
func (p *poolPlugin) showPoolSnapshot() (poolShowResult, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	p.v6mu.RLock()
	defer p.v6mu.RUnlock()
	var result poolShowResult
	configured := false
	hasIPv4 := false
	if p.pool != nil {
		result.poolIPv4ShowResult = p.pool.showPoolSnapshot()
		configured = true
		hasIPv4 = true
	}
	if len(p.namedPools) > 0 {
		result.NamedPools = make(map[string]poolIPv4ShowResult, len(p.namedPools))
		for name, pool := range p.namedPools {
			result.NamedPools[name] = pool.showPoolSnapshot()
		}
		configured = true
		hasIPv4 = true
	}
	if p.v6pool != nil {
		snapshot := p.v6pool.showPoolSnapshot()
		result.IPv6PD = &snapshot
		configured = true
	}
	if len(p.v6namedPools) > 0 {
		result.NamedIPv6Pools = make(map[string]poolV6ShowResult, len(p.v6namedPools))
		for name, pool := range p.v6namedPools {
			result.NamedIPv6Pools[name] = pool.showPoolSnapshot()
		}
		configured = true
	}
	if hasIPv4 {
		p.sessionAddrs.Range(func(key, value any) bool {
			id, ok := key.(string)
			if !ok {
				return true
			}
			sa, ok := value.(sessionAddr)
			if !ok {
				return true
			}
			result.Sessions = append(result.Sessions, sessionAlloc{
				subscriberID: id,
				TunnelID:     sa.tunnelID,
				SessionID:    sa.sessionID,
				Address:      sa.addr.String(),
			})
			return true
		})
	}
	return result, configured
}

type poolShowResult struct {
	poolIPv4ShowResult
	Sessions       []sessionAlloc                `json:"sessions"`
	IPv6PD         *poolV6ShowResult             `json:"ipv6-pd,omitempty"`
	NamedPools     map[string]poolIPv4ShowResult `json:"named-pools,omitempty"`
	NamedIPv6Pools map[string]poolV6ShowResult   `json:"named-ipv6-pools,omitempty"`
}

type poolIPv4ShowResult struct {
	Gateway      string `json:"gateway"`
	RangeStart   string `json:"range-start"`
	RangeEnd     string `json:"range-end"`
	DNSPrimary   string `json:"dns-primary"`
	DNSSecondary string `json:"dns-secondary"`
	Total        uint32 `json:"total"`
	Allocated    uint32 `json:"allocated"`
	Available    uint32 `json:"available"`
}

type poolV6ShowResult struct {
	Block            string `json:"block"`
	DelegationLength int    `json:"delegation-length"`
	Total            uint32 `json:"total"`
	Allocated        uint32 `json:"allocated"`
	Available        uint32 `json:"available"`
}

func (p *ipv4Pool) showPoolSnapshot() poolIPv4ShowResult {
	total, allocated, available := p.stats()
	return poolIPv4ShowResult{
		Gateway:    p.gateway.String(),
		RangeStart: p.start.String(),
		RangeEnd:   uint32ToAddr(addrToUint32(p.start) + p.size - 1).String(),
		DNSPrimary: p.dnsPrimary.String(), DNSSecondary: p.dnsSecondary.String(),
		Total: total, Allocated: allocated, Available: available,
	}
}

func (p *ipv6PrefixPool) showPoolSnapshot() poolV6ShowResult {
	total, allocated, available := p.stats()
	return poolV6ShowResult{Block: p.block.String(), DelegationLength: p.delegLen,
		Total: total, Allocated: allocated, Available: available}
}

type sessionAlloc struct {
	subscriberID string
	TunnelID     uint16 `json:"tunnel-id"`
	SessionID    uint16 `json:"session-id"`
	Address      string `json:"address"`
	Username     string `json:"username"`
}

func lookupSessionUsername(subscriberID string) string {
	sess, ok := subscriber.DefaultRegistry.Get(subscriberID)
	// refuse-by-default: guards poolPlugin.showPool -- username is optional display data
	if !ok {
		return ""
	}
	return sess.Username
}

type poolConfigResult struct {
	defaultPool   *ipv4Pool
	namedPools    map[string]*ipv4Pool
	defaultV6Pool *ipv6PrefixPool
	namedV6Pools  map[string]*ipv6PrefixPool
	found         bool
}

func parseFullPoolConfig(data string) (poolConfigResult, error) {
	cfg, err := parsePoolDescription(data)
	if err != nil {
		return poolConfigResult{}, err
	}
	return cfg.build(), nil
}

// commandDecls names the commands this plugin serves and states what each
// answer holds (pkg/plugin/rpc/types.go, CommandDecl).
//
// It has two readers and MUST stay one function. init() puts it on the
// registry.Registration, which anything linking the composition root reads
// without starting an engine, and the runner sends it in the Stage 1
// registration message, which a running daemon reads.
func commandDecls() []sdk.CommandDecl {
	return []sdk.CommandDecl{
		{
			Name:      "show l2tp pool",
			ShortHelp: "Show the IPv4 and IPv6 subscriber address pools and what each has allocated.",
		},
	}
}
