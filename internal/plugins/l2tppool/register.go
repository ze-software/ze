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

	"codeberg.org/thomas-mangin/ze/internal/component/l2tp"
	l2tpevents "codeberg.org/thomas-mangin/ze/internal/component/l2tp/events"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/cli"
	"codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
	"codeberg.org/thomas-mangin/ze/internal/component/ppp"
	"codeberg.org/thomas-mangin/ze/internal/core/slogutil"
	schema "codeberg.org/thomas-mangin/ze/internal/plugins/l2tppool/schema"
	sdk "codeberg.org/thomas-mangin/ze/pkg/plugin/sdk"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
)

var poolInstance = &poolPlugin{}

type poolPlugin struct {
	mu         sync.RWMutex
	pool       *ipv4Pool
	namedPools map[string]*ipv4Pool

	busMu sync.Mutex
	bus   ze.EventBus
	unsub func()

	sessionAddrs sync.Map // sessionKey -> netip.Addr
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

	reg := registry.Registration{
		Name:                    Name,
		Description:             "Static IPv4 address pool for L2TP PPP sessions",
		Features:                "yang",
		YANG:                    schema.ZeL2TPPoolConfYANG,
		ConfigRoots:             []string{"l2tp"},
		InProcessConfigVerifier: verifyPoolConfig,
		RunEngine:               runPlugin,
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
		},
		ConfigureEventBus: func(eb any) {
			if e, ok := eb.(ze.EventBus); ok {
				poolInstance.setEventBus(e)
			}
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
	if !ok {
		return
	}
	sa, ok := val.(sessionAddr)
	if !ok {
		return
	}
	if !sa.fromPool {
		logger().Info("l2tp-pool: RADIUS-assigned address cleared on session-down",
			"tunnel", payload.TunnelID, "session", payload.SessionID, "address", sa.addr)
		return
	}
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
		if named == nil {
			return ppp.IPResponseArgs{Accept: false, Family: req.Family,
				Reason: "named pool " + meta.FramedPool + " not configured"}
		}
		namedPool, ok := named[meta.FramedPool]
		if !ok {
			logger().Warn("l2tp-pool: named pool not found",
				"tunnel", req.TunnelID, "session", req.SessionID, "pool", meta.FramedPool)
			return ppp.IPResponseArgs{Accept: false, Family: req.Family,
				Reason: "named pool " + meta.FramedPool + " not found"}
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
	logger().Debug(Name + " plugin starting (RPC)")

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
			if pendingResult.defaultPool != nil {
				total, _, _ := pendingResult.defaultPool.stats()
				logger().Info("l2tp-pool: configured", "total", total, "named-pools", len(pendingResult.namedPools))
			}
			pendingResult = nil
		}
		return nil
	})

	p.OnConfigApply(func(_ []sdk.ConfigDiffSection) error {
		if pendingResult != nil {
			poolInstance.mu.Lock()
			var liveAllocs uint32
			if old := poolInstance.pool; old != nil {
				_, a, _ := old.stats()
				liveAllocs += a
			}
			for _, np := range poolInstance.namedPools {
				_, a, _ := np.stats()
				liveAllocs += a
			}
			if liveAllocs > 0 {
				poolInstance.mu.Unlock()
				pendingResult = nil
				return fmt.Errorf("l2tp-pool: cannot replace pools with %d live allocations; tear down sessions first", liveAllocs)
			}
			poolInstance.pool = pendingResult.defaultPool
			poolInstance.namedPools = pendingResult.namedPools
			poolInstance.mu.Unlock()
			if pendingResult.defaultPool != nil {
				total, _, _ := pendingResult.defaultPool.stats()
				logger().Info("l2tp-pool: configured", "total", total, "named-pools", len(pendingResult.namedPools))
			}
			pendingResult = nil
		}
		return nil
	})

	p.OnConfigRollback(func(_ string) error {
		pendingResult = nil
		return nil
	})

	p.OnExecuteCommand(func(_, command string, _ []string, _ string) (string, string, error) {
		if command == "l2tp pool show" {
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
			{Name: "l2tp pool show"},
		},
	}); err != nil {
		logger().Error(Name+" plugin failed", "error", err)
		return 1
	}
	return 0
}

func (p *poolPlugin) showPool() string {
	p.mu.RLock()
	pool := p.pool
	p.mu.RUnlock()

	if pool == nil {
		return `{"status":"no pool configured"}`
	}
	total, allocated, available := pool.stats()
	end := uint32ToAddr(addrToUint32(pool.start) + pool.size - 1)

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

	result := poolShowResult{
		Gateway:      pool.gateway.String(),
		RangeStart:   pool.start.String(),
		RangeEnd:     end.String(),
		DNSPrimary:   pool.dnsPrimary.String(),
		DNSSecondary: pool.dnsSecondary.String(),
		Total:        total,
		Allocated:    allocated,
		Available:    available,
		Sessions:     sessions,
	}
	b, err := json.Marshal(result)
	if err != nil {
		return fmt.Sprintf(`{"error":%q}`, err.Error())
	}
	return string(b)
}

type poolShowResult struct {
	Gateway      string         `json:"gateway"`
	RangeStart   string         `json:"range-start"`
	RangeEnd     string         `json:"range-end"`
	DNSPrimary   string         `json:"dns-primary"`
	DNSSecondary string         `json:"dns-secondary"`
	Total        uint32         `json:"total"`
	Allocated    uint32         `json:"allocated"`
	Available    uint32         `json:"available"`
	Sessions     []sessionAlloc `json:"sessions"`
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
	defaultPool *ipv4Pool
	namedPools  map[string]*ipv4Pool
	found       bool
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

	if namedList, ok := poolBlock["named-pool"].([]any); ok {
		named, err := parseNamedPools(namedList)
		if err != nil {
			return poolConfigResult{}, err
		}
		if len(named) > 0 {
			result.namedPools = named
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

func parseNamedPools(list []any) (map[string]*ipv4Pool, error) {
	pools := make(map[string]*ipv4Pool, len(list))
	for _, item := range list {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name, _ := entry["name"].(string)
		if name == "" {
			return nil, fmt.Errorf("%s: named-pool requires a name", Name)
		}
		pool, err := parseIPv4Pool(entry)
		if err != nil {
			return nil, fmt.Errorf("%s: named-pool %q: %w", Name, name, err)
		}
		pools[name] = pool
	}
	return pools, nil
}
