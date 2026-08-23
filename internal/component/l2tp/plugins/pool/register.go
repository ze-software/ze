// Design: docs/research/l2tpv2-ze-integration.md -- l2tp-pool plugin lifecycle
// Related: l2tppool.go -- atomic logger, Name constant
// Related: pool.go -- ipv4Pool bitmap allocation

package l2tppool

import (
	"encoding/json"
	"fmt"
	"net"
	"net/netip"
	"os"
	"sync"

	"github.com/ze-software/ze/internal/component/l2tp"
	l2tpevents "github.com/ze-software/ze/internal/component/l2tp/events"
	"github.com/ze-software/ze/internal/component/l2tp/plugins/pool/yang"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/configvalue"
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
	unsub func()

	sessionAddrs    sync.Map // sessionKey -> sessionAddr (IPv4)
	sessionPrefixes sync.Map // sessionKey -> sessionPrefix (IPv6)
}

type sessionPrefix struct {
	prefix   netip.Prefix
	fromPool bool
	poolName string
}

type sessionKey struct {
	tunnelID  uint16
	sessionID uint16
}

type sessionAddr struct {
	addr     netip.Addr
	fromPool bool   // false = RADIUS-assigned, skip pool.release
	poolName string // "" = default pool, non-empty = named pool
}

func init() {
	l2tp.RegisterPoolHandler(poolInstance.handle)
	l2tp.RegisterPrefixHandler(poolInstance.handlePrefix)
	l2tp.RegisterPrefixReleaser(poolInstance.releasePrefix)

	reg := registry.Registration{
		Name:                    Name,
		Description:             "IPv4 address and IPv6 prefix pool for L2TP PPP sessions",
		Features:                "yang",
		YANG:                    yang.ZeL2TPPoolConfYANG,
		ConfigRoots:             []string{"l2tp"},
		InProcessConfigVerifier: verifyPoolConfig,
		RunEngine:               runPlugin,
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
	for _, sec := range sections {
		if sec.Root != "l2tp" {
			continue
		}
		if _, _, err := parsePoolConfig(sec.Data); err != nil {
			return err
		}
	}
	return nil
}

func (p *poolPlugin) setEventBus(eb ze.EventBus) {
	p.busMu.Lock()
	defer p.busMu.Unlock()
	if p.unsub != nil {
		p.unsub()
	}
	p.bus = eb
	p.unsub = l2tpevents.SessionDown.Subscribe(eb, func(payload *l2tpevents.SessionDownPayload) {
		p.onSessionDown(payload)
	})
}

func (p *poolPlugin) onSessionDown(payload *l2tpevents.SessionDownPayload) {
	key := sessionKey{tunnelID: payload.TunnelID, sessionID: payload.SessionID}
	val, ok := p.sessionAddrs.LoadAndDelete(key)
	if ok {
		sa, ok2 := val.(sessionAddr)
		if ok2 {
			if !sa.fromPool {
				logger().Info("l2tp-pool: RADIUS-assigned address cleared on session-down",
					"tunnel", payload.TunnelID, "session", payload.SessionID, "address", sa.addr)
			} else {
				p.mu.RLock()
				var pool *ipv4Pool
				if sa.poolName != "" {
					if p.namedPools != nil {
						pool = p.namedPools[sa.poolName]
					}
				} else {
					pool = p.pool
				}
				p.mu.RUnlock()
				if pool != nil {
					pool.release(sa.addr)
					logger().Info("l2tp-pool: released address on session-down",
						"tunnel", payload.TunnelID, "session", payload.SessionID,
						"address", sa.addr, "pool", sa.poolName)
				}
			}
		}
	}

	p.releasePrefix(payload.TunnelID, payload.SessionID)
}

func (p *poolPlugin) handlePrefix(req l2tp.PrefixRequest) l2tp.PrefixResult {
	key := sessionKey{tunnelID: req.TunnelID, sessionID: req.SessionID}
	if _, already := p.sessionPrefixes.Load(key); already {
		return l2tp.PrefixResult{OK: false, Reason: "prefix already allocated for this session"}
	}

	meta := l2tp.LoadSessionMetadata(req.TunnelID, req.SessionID)
	if meta != nil && meta.DelegatedIPv6Prefix.IsValid() {
		p.sessionPrefixes.Store(
			sessionKey{tunnelID: req.TunnelID, sessionID: req.SessionID},
			sessionPrefix{prefix: meta.DelegatedIPv6Prefix, fromPool: false},
		)
		logger().Info("l2tp-pool: using RADIUS-assigned IPv6 prefix",
			"tunnel", req.TunnelID, "session", req.SessionID, "prefix", meta.DelegatedIPv6Prefix)
		return l2tp.PrefixResult{OK: true, Prefix: meta.DelegatedIPv6Prefix}
	}

	poolName := req.PoolName
	if poolName == "" && meta != nil && meta.FramedIPv6Pool != "" {
		poolName = meta.FramedIPv6Pool
	}

	p.v6mu.RLock()
	var pool *ipv6PrefixPool
	if poolName != "" {
		if p.v6namedPools != nil {
			pool = p.v6namedPools[poolName]
		}
	} else {
		pool = p.v6pool
	}
	p.v6mu.RUnlock()

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
		sessionKey{tunnelID: req.TunnelID, sessionID: req.SessionID},
		sessionPrefix{prefix: prefix, fromPool: true, poolName: poolName},
	)

	logger().Info("l2tp-pool: allocated IPv6 prefix",
		"tunnel", req.TunnelID, "session", req.SessionID, "prefix", prefix)
	return l2tp.PrefixResult{OK: true, Prefix: prefix}
}

func (p *poolPlugin) releasePrefix(tunnelID, sessionID uint16) {
	key := sessionKey{tunnelID: tunnelID, sessionID: sessionID}
	val, ok := p.sessionPrefixes.LoadAndDelete(key)
	if !ok {
		return
	}
	sp, ok := val.(sessionPrefix)
	if !ok {
		return
	}
	if !sp.fromPool {
		logger().Info("l2tp-pool: RADIUS-assigned IPv6 prefix cleared",
			"tunnel", tunnelID, "session", sessionID, "prefix", sp.prefix)
		return
	}
	p.v6mu.RLock()
	var pool *ipv6PrefixPool
	if sp.poolName != "" {
		if p.v6namedPools != nil {
			pool = p.v6namedPools[sp.poolName]
		}
	} else {
		pool = p.v6pool
	}
	p.v6mu.RUnlock()
	if pool != nil {
		pool.release(sp.prefix)
		logger().Info("l2tp-pool: released IPv6 prefix",
			"tunnel", tunnelID, "session", sessionID, "prefix", sp.prefix)
	}
}

func (p *poolPlugin) handle(req ppp.EventIPRequest) ppp.IPResponseArgs {
	if req.Family != ppp.AddressFamilyIPv4 {
		return ppp.IPResponseArgs{Accept: false, Family: req.Family, Reason: "IPv6 not supported by static pool"}
	}

	p.mu.RLock()
	pool := p.pool
	named := p.namedPools
	p.mu.RUnlock()

	if pool == nil {
		return ppp.IPResponseArgs{Accept: false, Family: req.Family, Reason: "no pool configured"}
	}

	meta := l2tp.LoadSessionMetadata(req.TunnelID, req.SessionID)

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

	// RFC 2865 Section 5.8: Framed-IP-Address bypasses pool allocation.
	// Uses the selected pool's gateway/DNS (named pool if Framed-Pool
	// was also set, default pool otherwise).
	if meta != nil && meta.FramedIP.IsValid() {
		p.sessionAddrs.Store(
			sessionKey{tunnelID: req.TunnelID, sessionID: req.SessionID},
			sessionAddr{addr: meta.FramedIP, fromPool: false},
		)
		logger().Info("l2tp-pool: using RADIUS-assigned address",
			"tunnel", req.TunnelID, "session", req.SessionID, "address", meta.FramedIP)
		return ppp.IPResponseArgs{
			Accept:       true,
			Family:       ppp.AddressFamilyIPv4,
			Local:        pool.gateway,
			Peer:         meta.FramedIP,
			DNSPrimary:   pool.dnsPrimary,
			DNSSecondary: pool.dnsSecondary,
		}
	}

	addr, ok := pool.allocate()
	if !ok {
		logger().Warn("l2tp-pool: pool exhausted",
			"tunnel", req.TunnelID, "session", req.SessionID)
		return ppp.IPResponseArgs{Accept: false, Family: req.Family, Reason: "pool exhausted"}
	}

	p.sessionAddrs.Store(
		sessionKey{tunnelID: req.TunnelID, sessionID: req.SessionID},
		sessionAddr{addr: addr, fromPool: true, poolName: poolName},
	)

	logger().Info("l2tp-pool: allocated address",
		"tunnel", req.TunnelID, "session", req.SessionID, "address", addr)

	return ppp.IPResponseArgs{
		Accept:       true,
		Family:       ppp.AddressFamilyIPv4,
		Local:        pool.gateway,
		Peer:         addr,
		DNSPrimary:   pool.dnsPrimary,
		DNSSecondary: pool.dnsSecondary,
	}
}

func runPlugin(conn net.Conn) int {
	var tb textbuf.Buffer
	logger().Debug(tb.Str(Name).Str(" plugin starting (RPC)").Slice())

	p := sdk.NewWithConn(Name, conn)
	defer func() { _ = p.Close() }()

	p.OnConfigVerify(verifyPoolConfig)

	var pendingResult *poolConfigResult

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		for _, sec := range sections {
			if sec.Root != "l2tp" {
				continue
			}
			result, err := parseFullPoolConfig(sec.Data)
			if err != nil {
				return err
			}
			if result.found {
				pendingResult = &result
			}
		}
		if pendingResult != nil {
			poolInstance.mu.Lock()
			poolInstance.pool = pendingResult.defaultPool
			poolInstance.namedPools = pendingResult.namedPools
			poolInstance.mu.Unlock()
			poolInstance.v6mu.Lock()
			poolInstance.v6pool = pendingResult.defaultV6Pool
			poolInstance.v6namedPools = pendingResult.namedV6Pools
			poolInstance.v6mu.Unlock()
			if pendingResult.defaultPool != nil {
				total, _, _ := pendingResult.defaultPool.stats()
				logger().Info("l2tp-pool: configured", "total", total, "named-pools", len(pendingResult.namedPools))
			}
			if pendingResult.defaultV6Pool != nil {
				total, _, _ := pendingResult.defaultV6Pool.stats()
				logger().Info("l2tp-pool: IPv6 configured", "total", total, "named-v6-pools", len(pendingResult.namedV6Pools))
			}
			pendingResult = nil
		}
		return nil
	})

	p.OnConfigApply(func(_ []sdk.ConfigDiffSection) error {
		if pendingResult != nil {
			poolInstance.mu.Lock()
			poolInstance.v6mu.Lock()
			var liveAllocs uint32
			if old := poolInstance.pool; old != nil {
				_, a, _ := old.stats()
				liveAllocs += a
			}
			for _, np := range poolInstance.namedPools {
				_, a, _ := np.stats()
				liveAllocs += a
			}
			if old := poolInstance.v6pool; old != nil {
				_, a, _ := old.stats()
				liveAllocs += a
			}
			for _, np := range poolInstance.v6namedPools {
				_, a, _ := np.stats()
				liveAllocs += a
			}
			if liveAllocs > 0 {
				poolInstance.v6mu.Unlock()
				poolInstance.mu.Unlock()
				pendingResult = nil
				return fmt.Errorf("l2tp-pool: cannot replace pools with %d live allocations; tear down sessions first", liveAllocs)
			}
			poolInstance.pool = pendingResult.defaultPool
			poolInstance.namedPools = pendingResult.namedPools
			poolInstance.v6pool = pendingResult.defaultV6Pool
			poolInstance.v6namedPools = pendingResult.namedV6Pools
			poolInstance.v6mu.Unlock()
			poolInstance.mu.Unlock()
			if pendingResult.defaultPool != nil {
				total, _, _ := pendingResult.defaultPool.stats()
				logger().Info("l2tp-pool: configured", "total", total, "named-pools", len(pendingResult.namedPools))
			}
			if pendingResult.defaultV6Pool != nil {
				total, _, _ := pendingResult.defaultV6Pool.stats()
				logger().Info("l2tp-pool: IPv6 configured", "total", total, "named-v6-pools", len(pendingResult.namedV6Pools))
			}
			pendingResult = nil
		}
		return nil
	})

	p.OnConfigRollback(func(_ string) error {
		pendingResult = nil
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
	if err := p.Run(ctx, sdk.Registration{
		WantsConfig:  []string{"l2tp"},
		VerifyBudget: 1,
		ApplyBudget:  1,
		Commands: []sdk.CommandDecl{
			{Name: "show l2tp pool"},
		},
	}); err != nil {
		logger().Error(Name+" plugin failed", "error", err)
		return 1
	}
	return 0
}

func (p *poolPlugin) showPool() any {
	p.mu.RLock()
	pool := p.pool
	p.mu.RUnlock()

	p.v6mu.RLock()
	v6pool := p.v6pool
	p.v6mu.RUnlock()

	if pool == nil && v6pool == nil {
		// Marshaled once by the SDK: return a Go value, not a JSON string
		// literal (which would double-encode on the wire).
		return map[string]any{"status": "no pool configured"}
	}

	var result poolShowResult

	if pool != nil {
		total, allocated, available := pool.stats()
		end := uint32ToAddr(addrToUint32(pool.start) + pool.size - 1)
		result.Gateway = pool.gateway.String()
		result.RangeStart = pool.start.String()
		result.RangeEnd = end.String()
		result.DNSPrimary = pool.dnsPrimary.String()
		result.DNSSecondary = pool.dnsSecondary.String()
		result.Total = total
		result.Allocated = allocated
		result.Available = available

		var sessions []sessionAlloc
		p.sessionAddrs.Range(func(key, value any) bool {
			sk, ok := key.(sessionKey)
			if !ok {
				return true
			}
			sa, ok := value.(sessionAddr)
			if !ok {
				return true
			}
			username := lookupSessionUsername(sk.tunnelID, sk.sessionID)
			sessions = append(sessions, sessionAlloc{
				TunnelID:  sk.tunnelID,
				SessionID: sk.sessionID,
				Address:   sa.addr.String(),
				Username:  username,
			})
			return true
		})
		result.Sessions = sessions
	}

	if v6pool != nil {
		v6Total, v6Allocated, v6Available := v6pool.stats()
		result.IPv6PD = &poolV6ShowResult{
			Block:            v6pool.block.String(),
			DelegationLength: v6pool.delegLen,
			Total:            v6Total,
			Allocated:        v6Allocated,
			Available:        v6Available,
		}
	}

	return result
}

type poolShowResult struct {
	Gateway      string            `json:"gateway"`
	RangeStart   string            `json:"range-start"`
	RangeEnd     string            `json:"range-end"`
	DNSPrimary   string            `json:"dns-primary"`
	DNSSecondary string            `json:"dns-secondary"`
	Total        uint32            `json:"total"`
	Allocated    uint32            `json:"allocated"`
	Available    uint32            `json:"available"`
	Sessions     []sessionAlloc    `json:"sessions"`
	IPv6PD       *poolV6ShowResult `json:"ipv6-pd,omitempty"`
}

type poolV6ShowResult struct {
	Block            string `json:"block"`
	DelegationLength int    `json:"delegation-length"`
	Total            uint32 `json:"total"`
	Allocated        uint32 `json:"allocated"`
	Available        uint32 `json:"available"`
}

type sessionAlloc struct {
	TunnelID  uint16 `json:"tunnel-id"`
	SessionID uint16 `json:"session-id"`
	Address   string `json:"address"`
	Username  string `json:"username"`
}

func lookupSessionUsername(tunnelID, sessionID uint16) string {
	svc := l2tp.LookupService()
	if svc == nil {
		return ""
	}
	sess, ok := svc.LookupSession(sessionID)
	if !ok {
		return ""
	}
	if sess.TunnelLocalTID != tunnelID {
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

func parsePoolConfig(data string) (pool *ipv4Pool, found bool, err error) {
	result, err := parseFullPoolConfig(data)
	if err != nil {
		return nil, false, err
	}
	return result.defaultPool, result.found, nil
}

func parseFullPoolConfig(data string) (poolConfigResult, error) {
	if data == "" {
		return poolConfigResult{}, nil
	}
	var tree map[string]any
	if err := json.Unmarshal([]byte(data), &tree); err != nil {
		return poolConfigResult{}, fmt.Errorf("%s: invalid config JSON: %w", Name, err)
	}

	l2tpBlock, ok := tree["l2tp"].(map[string]any)
	if !ok {
		return poolConfigResult{}, nil
	}
	poolBlock, ok := l2tpBlock["pool"].(map[string]any)
	if !ok {
		return poolConfigResult{}, nil
	}

	var result poolConfigResult

	if ipv4Block, ok := poolBlock["ipv4"].(map[string]any); ok {
		p, err := parseIPv4Pool(ipv4Block)
		if err != nil {
			return poolConfigResult{}, err
		}
		result.defaultPool = p
		result.found = true
	}

	// configvalue.ListEntries, not a []any assertion: Tree.ToMap renders a YANG
	// list as a map of list key to entry at every count, so the assertion found
	// nothing whatever the operator wrote and every named pool was lost.
	if entries := configvalue.ListEntries(poolBlock["named-pool"]); len(entries) > 0 {
		named, err := parseNamedPools(entries)
		if err != nil {
			return poolConfigResult{}, err
		}
		if len(named) > 0 {
			result.namedPools = named
			result.found = true
		}
	}

	if v6Block, ok := poolBlock["ipv6-pd"].(map[string]any); ok {
		p, err := parseIPv6PDPool(v6Block)
		if err != nil {
			return poolConfigResult{}, err
		}
		result.defaultV6Pool = p
		result.found = true
	}

	if entries := configvalue.ListEntries(poolBlock["named-ipv6-pool"]); len(entries) > 0 {
		named, err := parseNamedIPv6Pools(entries)
		if err != nil {
			return poolConfigResult{}, err
		}
		if len(named) > 0 {
			result.namedV6Pools = named
			result.found = true
		}
	}

	return result, nil
}

func parseIPv4Pool(ipv4Block map[string]any) (*ipv4Pool, error) {
	gwStr, _ := ipv4Block["gateway"].(string)
	if gwStr == "" {
		return nil, fmt.Errorf("%s: pool ipv4 requires gateway (NAS-side IP)", Name)
	}
	gateway, err := netip.ParseAddr(gwStr)
	if err != nil {
		return nil, fmt.Errorf("%s: invalid gateway address %q: %w", Name, gwStr, err)
	}
	if !gateway.Is4() {
		return nil, fmt.Errorf("%s: gateway must be IPv4", Name)
	}

	startStr, _ := ipv4Block["start"].(string)
	endStr, _ := ipv4Block["end"].(string)
	if startStr == "" || endStr == "" {
		return nil, fmt.Errorf("%s: pool ipv4 requires both start and end", Name)
	}

	start, err := netip.ParseAddr(startStr)
	if err != nil {
		return nil, fmt.Errorf("%s: invalid start address %q: %w", Name, startStr, err)
	}
	end, err := netip.ParseAddr(endStr)
	if err != nil {
		return nil, fmt.Errorf("%s: invalid end address %q: %w", Name, endStr, err)
	}
	if !start.Is4() || !end.Is4() {
		return nil, fmt.Errorf("%s: pool addresses must be IPv4", Name)
	}
	if addrToUint32(end) < addrToUint32(start) {
		return nil, fmt.Errorf("%s: end %s is before start %s", Name, end, start)
	}
	if gateway == start || (addrToUint32(gateway) >= addrToUint32(start) && addrToUint32(gateway) <= addrToUint32(end)) {
		return nil, fmt.Errorf("%s: gateway %s must not overlap pool range %s-%s", Name, gateway, start, end)
	}

	var dnsPrimary, dnsSecondary netip.Addr
	if s, ok := ipv4Block["dns-primary"].(string); ok && s != "" {
		dnsPrimary, err = netip.ParseAddr(s)
		if err != nil {
			return nil, fmt.Errorf("%s: invalid dns-primary %q: %w", Name, s, err)
		}
	}
	if s, ok := ipv4Block["dns-secondary"].(string); ok && s != "" {
		dnsSecondary, err = netip.ParseAddr(s)
		if err != nil {
			return nil, fmt.Errorf("%s: invalid dns-secondary %q: %w", Name, s, err)
		}
	}

	return newIPv4Pool(gateway, start, end, dnsPrimary, dnsSecondary), nil
}

// parseNamedPools builds the Framed-Pool table. The pool name is the list KEY,
// because Tree.ToMap uses it as the map key and does not repeat it as a field
// inside the entry, so reading a "name" leaf out of the entry finds nothing.
func parseNamedPools(entries []configvalue.ListEntry) (map[string]*ipv4Pool, error) {
	pools := make(map[string]*ipv4Pool, len(entries))
	for _, entry := range entries {
		if entry.Key == "" {
			return nil, fmt.Errorf("%s: named-pool requires a name", Name)
		}
		pool, err := parseIPv4Pool(entry.Fields)
		if err != nil {
			return nil, fmt.Errorf("%s: named-pool %q: %w", Name, entry.Key, err)
		}
		pools[entry.Key] = pool
	}
	return pools, nil
}

func parseIPv6PDPool(block map[string]any) (*ipv6PrefixPool, error) {
	blockStr, _ := block["block"].(string)
	if blockStr == "" {
		return nil, fmt.Errorf("%s: ipv6-pd requires block", Name)
	}
	prefix, err := netip.ParsePrefix(blockStr)
	if err != nil {
		return nil, fmt.Errorf("%s: invalid ipv6-pd block %q: %w", Name, blockStr, err)
	}
	if !prefix.Addr().Is6() || prefix.Addr().Is4In6() {
		return nil, fmt.Errorf("%s: ipv6-pd block must be native IPv6", Name)
	}

	delegLen := 56
	if v, ok := block["delegation-length"].(float64); ok {
		delegLen = int(v)
	} else {
		logger().Warn("l2tp-pool: ipv6-pd delegation-length not set, defaulting to /56")
	}

	return newIPv6PrefixPool(prefix, delegLen)
}

// parseNamedIPv6Pools builds the Framed-Pool prefix-delegation table. As with
// parseNamedPools, the pool name is the list KEY rather than a field.
func parseNamedIPv6Pools(entries []configvalue.ListEntry) (map[string]*ipv6PrefixPool, error) {
	pools := make(map[string]*ipv6PrefixPool, len(entries))
	for _, entry := range entries {
		if entry.Key == "" {
			return nil, fmt.Errorf("%s: named-ipv6-pool requires a name", Name)
		}
		pool, err := parseIPv6PDPool(entry.Fields)
		if err != nil {
			return nil, fmt.Errorf("%s: named-ipv6-pool %q: %w", Name, entry.Key, err)
		}
		pools[entry.Key] = pool
	}
	return pools, nil
}
