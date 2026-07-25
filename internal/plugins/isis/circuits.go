// Design: plan/learned/931-isis-5-adjacency.md -- engine <-> adjacency-circuit wiring.
// Related: server.go -- the engine struct, dispatcher, and lifecycle this extends
// Related: events.go -- the eventSink the circuits emit session events through
//
// RFC: rfc/short/rfc5308.md sec 2/3 -- IPv6 interface-address scope (TLV 232
//   link-local in the IIH, non-link-local in the LSP) and the no-link-local-in-
//   TLV-236 rule applied by the connected/interface helpers below (isis-12).
//
// This file holds the engine's adjacency-circuit management: building a circuit
// per opened interface, running its Hello-send + hold-timer-sweep goroutine,
// tearing it down on a circuit-down event, publishing the adjacency metrics, and
// rendering the `show isis neighbor` snapshot. It is split out of server.go so
// the dispatcher/lifecycle file stays focused.

package isis

import (
	"net/netip"
	"slices"
	"time"

	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/plugins/isis/adjacency"
	"github.com/ze-software/ze/internal/plugins/isis/circuit"
	"github.com/ze-software/ze/internal/plugins/isis/types"
)

// launchCircuitGoroutine builds the adjacency circuit for an opened interface and
// runs its Hello-send and hold-timer-sweep loop. The circuit owns the adjacency
// table; the receive fan routes IIHs into it by ifindex. The goroutine stops on
// ctx cancellation (engine shutdown) OR on the per-circuit stop channel, which
// onCircuitDown/closeCircuit close when the circuit is removed (link-down,
// disable, or reconcile-remove). This bounds the goroutine to the circuit's
// lifetime (the goroutine-lifecycle rule: long-lived workers, no per-event leak):
// before the fix the loop only watched e.ctx.Done() and kept ticking after the
// circuit was torn down, and a reopen of the same interface stacked a second
// goroutine on it.
func (e *engine) launchCircuitGoroutine(ic InterfaceConfig) {
	c := e.buildCircuit(ic)
	if c == nil {
		// The transport reported no open circuit (e.g. a fake without a real
		// socket on a path that did not open). Nothing to run.
		return
	}

	name := ic.Name
	// Stop any goroutine still bound to this interface name before launching the
	// new one, so a reopen (link-up after a down, or a reconcile re-add) can never
	// run two hello+sweep goroutines for a single circuit. Defensive: a clean
	// down already closed and cleared the channel via onCircuitDown.
	stop := make(chan struct{})
	e.circuitsMu.Lock()
	if prev, ok := e.circuitStop[name]; ok {
		close(prev)
	}
	e.circuits[c.IfIndex()] = c
	e.circuitByName[name] = c
	e.circuitStop[name] = stop
	e.circuitsMu.Unlock()

	helloEvery := time.Duration(ic.HelloInterval) * time.Second
	if helloEvery <= 0 {
		helloEvery = time.Duration(DefaultHelloInterval) * time.Second
	}

	e.wg.Go(func() {
		hello := time.NewTicker(helloEvery)
		sweep := time.NewTicker(sweepInterval)
		defer hello.Stop()
		defer sweep.Stop()
		// Send an initial Hello immediately so an adjacency can form before the
		// first tick (ISO/IEC 10589 section 8.2: Hellos are sent on circuit up).
		if err := c.SendHello(); err != nil {
			e.log.Debug("isis: initial hello send", "interface", name, "err", err)
		}
		for {
			select {
			case <-e.ctx.Done():
				return
			case <-stop:
				// The circuit was removed (link-down/disable/reconcile-remove); exit
				// so the worker does not keep sending Hellos and sweeping an empty
				// table on a gone circuit.
				return
			case <-hello.C:
				if err := c.SendHello(); err != nil {
					e.log.Debug("isis: hello send", "interface", name, "err", err)
				}
			case <-sweep.C:
				c.Sweep()
				e.publishAdjMetrics()
			}
		}
	})
}

// buildCircuit constructs an adjacency circuit for an opened interface, pulling
// the resolved ifindex / source MAC / MTU from the transport and the System ID /
// area addresses from the active config. Returns nil when the transport has no
// open circuit on the interface (so the engine does not track a phantom circuit).
func (e *engine) buildCircuit(ic InterfaceConfig) *circuit.Circuit {
	ifindex, hwaddr, _, ok := e.transport.CircuitInfo(ic.Name)
	if !ok {
		return nil
	}

	e.mu.Lock()
	sysID := e.cfg.SystemID
	areas := areaAddresses(e.cfg.NETs)
	e.mu.Unlock()

	kind := adjacency.KindBroadcast
	if ic.CircuitType == CircuitPointToPoint {
		kind = adjacency.KindP2P
	}

	cfg := circuit.Config{
		Name:           ic.Name,
		IfIndex:        ifindex,
		SystemID:       sysID,
		SNPA:           adjacency.SNPA(hwaddr),
		Areas:          areas,
		IPv4:           interfaceIPv4(ic),
		AdvertiseIPv6:  advertisesIPv6(ic),
		IPv6LinkLocal:  interfaceIPv6LinkLocal(ic),
		Kind:           kind,
		Levels:         circuitLevels(ic.Level),
		HelloInterval:  ic.HelloInterval,
		HoldMult:       ic.HoldMult,
		Priority:       ic.Priority,
		LocalCircuitID: localCircuitID(ifindex),
	}
	c := circuit.New(cfg, e.transport, time.Now)
	if e.sink != nil {
		c.SetEventSink(e.sink)
	}
	// Install the per-interface (IIH) authentication signer (isis-10). A no-op
	// when no auth key chain is configured on this circuit's level.
	e.installCircuitSigner(c)
	// On an adjacency transition, refresh the metrics AND re-originate the node's
	// own LSP set: an Up neighbor must appear in TLV 22, a lost one must be
	// withdrawn (the Wiring Test: adjacency Up -> origination -> store -> SRM).
	// On a P2P circuit reaching Up, also send the initial CSNP to synchronize the
	// two LSDBs fast (isis-7, AC-11): the onUp closure captures the interface name
	// so the flooder can source a CSNP on exactly that circuit at the Up level.
	name := ic.Name
	c.SetTransitionHooks(
		func(level adjacency.Level) {
			e.publishAdjMetrics()
			// Run the per-level DIS election BEFORE re-originating the own LSP so a
			// new pseudo-node (and the star encoding) is in place when the own LSP is
			// rebuilt (isis-8). On a P2P circuit runElection is a no-op.
			e.runElection(c)
			e.originate()
			e.onAdjacencyUpFlood(name, level)
		},
		func(adjacency.Level) {
			e.publishAdjMetrics()
			e.runElection(c)
			e.originate()
		},
	)
	return c
}

// onCircuitDown is the transport OnCircuitDown callback: a link went down or the
// interface was disabled (closeCircuit -> DisableInterface routes here too). It
// stops the per-circuit hello+sweep goroutine, tears down every adjacency on the
// circuit (emitting session-down), and removes the circuit from the maps. Closing
// the stop channel here is what bounds the goroutine's lifetime to the circuit's:
// without it the worker leaked, kept sending Hellos on a gone circuit, and a
// reopen would stack a second goroutine on the same interface.
func (e *engine) onCircuitDown(ifindex int, name string) {
	e.circuitsMu.Lock()
	c := e.circuits[ifindex]
	if stop, ok := e.circuitStop[name]; ok {
		close(stop)
		delete(e.circuitStop, name)
	}
	delete(e.circuits, ifindex)
	delete(e.circuitByName, name)
	e.circuitsMu.Unlock()
	if c != nil {
		c.Teardown()
		e.publishAdjMetrics()
		// Drop the circuit's SRM/SSN flags from every LSP (isis-6: flags cleared
		// on circuit removal). Purge any pseudo-node LSP this node originated as the
		// DIS on the gone circuit and clear its recorded pseudo-node (isis-8, R-2) so
		// no phantom node lingers, then re-originate so the lost adjacency / star
		// entry leaves TLV 22.
		e.clearCircuitFlags(name)
		e.clearCircuitDIS(name)
		e.originate()
	}
}

// publishAdjMetrics recomputes the adjacency gauges from every live circuit. It
// resets the per-(level,interface) up gauge and the per-level total, then sums
// across circuits. Cheap (circuit count is small) and called on every transition
// and sweep so the gauges track the live state.
func (e *engine) publishAdjMetrics() {
	e.circuitsMu.RLock()
	circuits := make([]*circuit.Circuit, 0, len(e.circuitByName))
	for _, c := range e.circuitByName {
		circuits = append(circuits, c)
	}
	e.circuitsMu.RUnlock()

	upByLevelIface := map[[2]string]int{}
	totalByLevel := map[string]int{}
	for _, c := range circuits {
		for _, row := range c.Table().Snapshot() {
			totalByLevel[row.Level]++
			if row.State == adjacency.StateUp.String() {
				upByLevelIface[[2]string{row.Level, c.Name()}]++
			}
		}
	}
	e.mu.Lock()
	adjUp := e.adjUp
	adjTotal := e.adjTotal
	e.mu.Unlock()
	for key, n := range upByLevelIface {
		adjUp.With(key[0], key[1]).Set(float64(n))
	}
	for level, n := range totalByLevel {
		adjTotal.With(level).Set(float64(n))
	}
}

// neighborSnapshot returns the merged `show isis neighbor` view across every
// live circuit (the snapshot API consumed by spec-isis-13). Each row carries the
// interface plus the per-neighbor fields. It reads each circuit's table under
// the table read lock and never exposes a live pointer.
func (e *engine) neighborSnapshot() []any {
	e.circuitsMu.RLock()
	circuits := make([]*circuit.Circuit, 0, len(e.circuitByName))
	for _, c := range e.circuitByName {
		circuits = append(circuits, c)
	}
	e.circuitsMu.RUnlock()

	type neighborRow struct {
		Interface  string `json:"interface"`
		SystemID   string `json:"system-id"`
		SNPA       string `json:"snpa,omitempty"`
		Level      string `json:"level"`
		State      string `json:"state"`
		IPv4       string `json:"ipv4,omitempty"`
		IPv6       string `json:"ipv6,omitempty"`
		HoldTime   uint16 `json:"hold-time"`
		HoldExpiry int64  `json:"hold-expiry-unix"`
	}

	out := make([]any, 0)
	for _, c := range circuits {
		for _, row := range c.Table().Snapshot() {
			out = append(out, neighborRow{
				Interface:  c.Name(),
				SystemID:   row.SystemID,
				SNPA:       row.SNPA,
				Level:      row.Level,
				State:      row.State,
				IPv4:       row.IPv4,
				IPv6:       row.IPv6,
				HoldTime:   row.HoldTime,
				HoldExpiry: row.HoldExpiry,
			})
		}
	}
	return out
}

// areaAddresses extracts the distinct area addresses from the configured NETs
// (each NET's area portion). These are originated in TLV 1 and used for the L1
// area match (ISO/IEC 10589 section 8.2.2).
func areaAddresses(nets []types.NET) []types.AreaID {
	out := make([]types.AreaID, 0, len(nets))
	for _, n := range nets {
		area := n.AreaID()
		if !containsArea(out, area) {
			out = append(out, area)
		}
	}
	return out
}

// containsArea reports whether areas already holds a, by value equality.
func containsArea(areas []types.AreaID, a types.AreaID) bool {
	return slices.ContainsFunc(areas, a.Equal)
}

// circuitLevels maps a configured interface level to the adjacency levels the
// circuit forms at.
func circuitLevels(l Level) []adjacency.Level {
	switch l {
	case LevelL1:
		return []adjacency.Level{adjacency.Level1}
	case LevelL2:
		return []adjacency.Level{adjacency.Level2}
	default:
		return []adjacency.Level{adjacency.Level1, adjacency.Level2}
	}
}

// advertisesIPv6 reports whether the interface enables the IPv6 address family
// (so TLV 129 advertises NLPID 0x8E, the IIH carries TLV 232, and TLV 236 /
// IPv6 SPF run for the circuit). It gates all IPv6 origination + SPF (isis-12).
func advertisesIPv6(ic InterfaceConfig) bool {
	return slices.Contains(ic.AddressFamily, "ipv6-unicast")
}

// localCircuitID derives the 1-octet local circuit ID from the ifindex (the
// low octet). ISO/IEC 10589 assigns a per-circuit ID locally; the ifindex low
// octet is a stable, unique-enough value within one node for the P2P IIH and
// TLV 240. A future spec may assign IDs explicitly.
func localCircuitID(ifindex int) uint8 { return uint8(ifindex) }

// AddrInfo.Family values returned by the iface resolver, used by the
// interface-address helpers below to split v4 / v6.
const (
	familyIPv4 = "ipv4"
	familyIPv6 = "ipv6"
)

// interfaceIPv4 returns the interface's primary IPv4 address for TLV 132
// origination, resolved via the iface resolver (which maps the logical IS-IS
// interface name to its OS device). It returns an invalid address when the
// interface has no IPv4 or cannot be read; the Hello then omits TLV 132. The
// per-interface config carries no address today, so the OS is the source of
// truth for the interface address.
func interfaceIPv4(ic InterfaceConfig) netip.Addr {
	addrs, err := iface.Addresses(ic.Name)
	if err != nil {
		return netip.Addr{}
	}
	for _, a := range addrs {
		if a.Family != familyIPv4 {
			continue
		}
		if addr, perr := netip.ParseAddr(a.Address); perr == nil && addr.Is4() {
			return addr
		}
	}
	return netip.Addr{}
}

// interfaceIPv6LinkLocal returns the interface's IPv6 LINK-LOCAL address (fe80::)
// for TLV 232 origination in the IIH (RFC 5308 sec 3: a Hello carries only
// link-local addresses), resolved via the iface resolver. It returns an invalid
// address when the interface has no IPv6 link-local address or cannot be read; the IIH then
// omits TLV 232 (isis-12). The link-local address is the IPv6 SPF next-hop a
// neighbor advertises to us.
func interfaceIPv6LinkLocal(ic InterfaceConfig) netip.Addr {
	if !advertisesIPv6(ic) {
		return netip.Addr{}
	}
	addrs, err := iface.Addresses(ic.Name)
	if err != nil {
		return netip.Addr{}
	}
	for _, a := range addrs {
		if a.Family != familyIPv6 || !a.LinkLocal {
			continue
		}
		if addr, perr := netip.ParseAddr(a.Address); perr == nil && addr.Is6() {
			return addr.WithZone("") // strip any scope; the ifindex is carried separately
		}
	}
	return netip.Addr{}
}

// interfaceIPv6Prefixes returns the named interface's connected NON-LINK-LOCAL
// IPv6 prefixes (network-masked) for TLV 236 advertisement (isis-12). Link-local
// (fe80::/10) prefixes are excluded (RFC 5308 sec 2: link-local prefixes MUST NOT
// be advertised in TLV 236). A missing interface or read error yields an empty
// slice. Mirrors interfaceIPv4Prefixes for IPv6.
func interfaceIPv6Prefixes(name string) []netip.Prefix {
	addrs, err := iface.Addresses(name)
	if err != nil {
		return nil
	}
	out := make([]netip.Prefix, 0, len(addrs))
	for _, a := range addrs {
		if a.Family != familyIPv6 || a.LinkLocal {
			continue // RFC 5308 sec 2: no link-local prefixes in TLV 236
		}
		addr, perr := netip.ParseAddr(a.Address)
		if perr != nil || !addr.Is6() {
			continue
		}
		p := netip.PrefixFrom(addr.WithZone(""), a.PrefixLength)
		if !p.IsValid() {
			continue
		}
		out = append(out, p.Masked())
	}
	return out
}

// interfaceIPv6NonLinkLocal returns the interface's NON-LINK-LOCAL IPv6 host
// addresses for the LSP TLV 232 (RFC 5308 sec 3: an LSP carries only
// non-link-local addresses). Mirrors interfaceIPv6LinkLocal but for the LSP
// scope. A missing interface or read error yields an empty slice.
func interfaceIPv6NonLinkLocal(name string) []netip.Addr {
	addrs, err := iface.Addresses(name)
	if err != nil {
		return nil
	}
	out := make([]netip.Addr, 0, len(addrs))
	for _, a := range addrs {
		if a.Family != familyIPv6 || a.LinkLocal {
			continue
		}
		addr, perr := netip.ParseAddr(a.Address)
		if perr != nil || !addr.Is6() {
			continue
		}
		out = append(out, addr.WithZone(""))
	}
	return out
}
