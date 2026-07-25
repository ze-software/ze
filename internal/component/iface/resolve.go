// Design: plan/learned/950-iface-resolve-2-resolver.md -- shared logical-name resolver
// Overview: iface.go -- Binding / LinkEvent value types
// Related: dispatch.go -- GetInterface backend dispatch this builds on
// Related: events/events.go -- monitor event namespace + constants

package iface

import (
	"encoding/json"
	"fmt"
	"net"
	"net/netip"
	"strings"
	"sync"

	ifaceevents "github.com/ze-software/ze/internal/core/iface/events"
	"github.com/ze-software/ze/pkg/ze"
)

// resolver maps a logical interface name to a kernel device and serves value
// snapshots (Resolve), a scope-tagged address list (Addresses), and per-name
// link events (Subscribe). It is the single owner of logical-name -> device
// resolution for external consumers; the iface component's own mutation ops
// keep using netlink.Link directly (they need the full link object).
//
// Translation: a logical name maps to its OS device via the os-name selector
// (config os-name leaf, defaulting to the name itself). The forward map
// (logical -> os) drives Resolve/Addresses; the reverse map (os -> logical)
// lets a kernel-name monitor event reach the logical name(s) bound to it.
//
// Caching: Resolve results are cached by logical name. The cached ifindex is a
// hint -- any create/up/down monitor event for the device drops the entry, so
// the next Resolve re-reads the kernel and a deleted device never returns a
// stale ifindex (RTM_DELLINK arrives as a down event, monitor_linux.go).
type resolver struct {
	mu        sync.Mutex
	cache     map[string]Binding
	subs      map[string]map[int]chan LinkEvent
	nextSubID int
	osNames   map[string]string   // logical -> os (only entries that override)
	logicalOf map[string][]string // os -> logical names bound to it
	// permMACs maps a logical name to the normalized MAC its mac/match selector
	// binds to; permMACOf is the reverse (MAC -> logical names) so a device
	// event can reach the names that select it. Both empty when no mac/match
	// selector is configured (the common case).
	permMACs  map[string]string
	permMACOf map[string][]string
	// boundOS records the kernel device each logical name last resolved to. It
	// outlives a cache invalidation so a down/delete event for a device whose
	// MAC can no longer be read still reaches the mac/match binding it backed.
	boundOS map[string]string
	bound   bool
	unsub   []func()
}

var globalResolver = &resolver{
	cache: make(map[string]Binding),
	subs:  make(map[string]map[int]chan LinkEvent),
}

// Resolve returns a value Binding for the logical interface name. The name is
// translated to its OS device via the os-name selector, then resolved against
// the active backend. Returns an error if no backend is loaded or the device
// is absent. The result is cached until a monitor link event invalidates it.
func Resolve(name string) (Binding, error) { return globalResolver.resolve(name) }

// Addresses returns the IP addresses of the logical interface, each tagged with
// Family and LinkLocal so a consumer can split v4 / v6-link-local / v6-global
// without re-parsing. The logical name is translated via the os-name selector
// first. Returns an error if no backend is loaded or the device is absent.
func Addresses(name string) ([]AddrInfo, error) { return globalResolver.addresses(name) }

// Subscribe registers for link events on the logical name and returns a
// buffered channel plus a cancel func. The channel receives appeared/up/down
// events even for an interface that does not exist yet (it fires when the
// device appears). On a full channel events are dropped; the consumer should
// re-Resolve on the next event it does receive. Cancel removes the
// subscription and closes the channel; it leaks no goroutine because event
// fan-out is synchronous in the monitor handler.
func Subscribe(name string) (<-chan LinkEvent, func()) { return globalResolver.subscribe(name) }

func (r *resolver) resolve(name string) (Binding, error) {
	r.mu.Lock()
	if b, ok := r.cache[name]; ok {
		r.mu.Unlock()
		return b, nil
	}
	r.mu.Unlock()

	info, err := r.osDeviceFor(name)
	if err != nil {
		return Binding{}, err
	}
	b := bindingFromInfo(info.OsName, info)

	r.mu.Lock()
	r.cache[name] = b
	r.mu.Unlock()
	return b, nil
}

func (r *resolver) addresses(name string) ([]AddrInfo, error) {
	info, err := r.osDeviceFor(name)
	if err != nil {
		return nil, err
	}
	return classifyAddresses(info.Addresses), nil
}

// osDeviceFor resolves a logical name to its kernel device InterfaceInfo. The
// mac/match selector wins when set: the resolver scans every interface and
// binds to the one carrying the configured hardware MAC (matchByMAC). Without a
// MAC selector the os-name selector (or the logical name itself) is looked up
// directly. On success it records the resolved kernel device so a later down
// event for it can reach this logical name. The backend call runs outside the
// lock; only the small map reads/writes are locked.
func (r *resolver) osDeviceFor(name string) (*InterfaceInfo, error) {
	r.mu.Lock()
	wantMAC := r.permMACs[name]
	osn := r.osNames[name]
	r.mu.Unlock()

	if wantMAC != "" {
		info, err := r.matchByMAC(name, wantMAC)
		if err != nil {
			return nil, err
		}
		r.recordBinding(name, info.OsName)
		return info, nil
	}

	osName := osn
	if osName == "" {
		osName = name
	}
	info, err := GetInterface(osName)
	if err != nil {
		return nil, err
	}
	r.recordBinding(name, info.OsName)
	return info, nil
}

// matchByMAC returns the interface whose hardware MAC equals want. It matches
// the permanent (factory) MAC when the device reports one -- so the binding
// survives an operational MAC override on a real NIC -- and falls back to the
// current MAC for the virtual kinds that report no permanent address. When
// several devices match (a kernel anomaly) the lowest ifindex wins, for
// determinism.
func (r *resolver) matchByMAC(name, want string) (*InterfaceInfo, error) {
	target := normalizeMAC(want)
	infos, err := ListInterfaces()
	if err != nil {
		return nil, err
	}
	var best *InterfaceInfo
	for i := range infos {
		if normalizeMAC(deviceMatchMAC(&infos[i])) != target {
			continue
		}
		if best == nil || infos[i].Index < best.Index {
			c := infos[i]
			best = &c
		}
	}
	if best == nil {
		return nil, fmt.Errorf("iface: no device with MAC %s for logical interface %q", target, name)
	}
	return best, nil
}

// deviceMatchMAC returns the MAC the resolver matches a device on: its permanent
// (factory) address when present, else its current address.
func deviceMatchMAC(info *InterfaceInfo) string {
	if info.PermanentMAC != "" {
		return info.PermanentMAC
	}
	return info.MAC
}

// normalizeMAC canonicalizes a MAC string for comparison (lower-case,
// colon-separated) via net.ParseMAC. An empty string stays empty; an
// unparseable value falls back to a trimmed lower-case copy so a garbage
// selector never collides with a real address by coincidence.
func normalizeMAC(s string) string {
	if s == "" {
		return ""
	}
	if hw, err := net.ParseMAC(s); err == nil {
		return hw.String()
	}
	return strings.ToLower(strings.TrimSpace(s))
}

// recordBinding remembers the kernel device a logical name currently resolves
// to. The reverse lookup (boundOS) lets a down/delete event for that device
// reach the logical name even after GetInterface can no longer report its MAC
// (the device is gone) -- the only way a deferred-then-bound mac/match selector
// learns it lost its device.
func (r *resolver) recordBinding(name, osDevice string) {
	if osDevice == "" {
		return
	}
	r.mu.Lock()
	if r.boundOS == nil {
		r.boundOS = make(map[string]string)
	}
	r.boundOS[name] = osDevice
	r.mu.Unlock()
}

// bindingFromInfo builds a value Binding from an InterfaceInfo resolved for the
// given OS device name. osName is the kernel device the info was read from.
func bindingFromInfo(osName string, info *InterfaceInfo) Binding {
	return Binding{
		Ifindex: info.Index,
		OsName:  osName,
		OperMAC: info.MAC,
		PermMAC: info.PermanentMAC,
		MTU:     info.MTU,
		State:   info.State,
	}
}

// classifyAddresses returns a copy of addrs with LinkLocal set per the IPv6
// fe80::/10 rule. IPv4 is never link-local-classified. Pure and backend-free
// so the scope split is host-testable.
func classifyAddresses(addrs []AddrInfo) []AddrInfo {
	if len(addrs) == 0 {
		return nil
	}
	out := make([]AddrInfo, len(addrs))
	for i := range addrs {
		out[i] = addrs[i]
		if ip, err := netip.ParseAddr(addrs[i].Address); err == nil {
			out[i].LinkLocal = ip.Is6() && ip.IsLinkLocalUnicast()
		}
	}
	return out
}

func (r *resolver) subscribe(name string) (<-chan LinkEvent, func()) {
	ch := make(chan LinkEvent, 8)
	r.mu.Lock()
	if r.subs[name] == nil {
		r.subs[name] = make(map[int]chan LinkEvent)
	}
	id := r.nextSubID
	r.nextSubID++
	r.subs[name][id] = ch
	r.mu.Unlock()

	var once sync.Once
	cancel := func() {
		once.Do(func() {
			r.mu.Lock()
			if m, ok := r.subs[name]; ok {
				if c, ok := m[id]; ok {
					delete(m, id)
					close(c)
				}
				if len(m) == 0 {
					delete(r.subs, name)
				}
			}
			r.mu.Unlock()
		})
	}
	return ch, cancel
}

// onLinkEvent invalidates the cache for every logical name bound to kernelName
// and fans the event out to each one's subscribers. Sends are non-blocking and
// performed under the lock (cancel holds the same lock when it closes a
// channel, so there is no send-on-closed race); a full channel drops the event
// (the consumer recovers on the next event or a re-Resolve).
func (r *resolver) onLinkEvent(kernelName string, kind LinkEventKind, index int) {
	// For an up/appeared event, learn the device's hardware MAC so a deferred
	// mac/match binding can attach to a freshly appeared device. Done outside
	// the lock (a backend call) and only when mac/match selectors exist, so the
	// common path pays nothing. On a down event the device may already be gone
	// (GetInterface would fail); the boundOS reverse map carries the last-known
	// binding for that case instead.
	var devMatchMAC string
	if kind != LinkDown && r.hasPermMACMatches() {
		if info, err := GetInterface(kernelName); err == nil {
			devMatchMAC = deviceMatchMAC(info)
		}
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	for _, logical := range r.logicalsForLocked(kernelName, devMatchMAC) {
		delete(r.cache, logical)
		ev := LinkEvent{Name: logical, Kind: kind, Index: index}
		for _, c := range r.subs[logical] {
			select {
			case c <- ev:
			default:
				loggerPtr.Load().Debug("iface resolver: subscriber channel full, dropping event",
					"interface", logical, "kind", string(kind))
			}
		}
	}
}

// hasPermMACMatches reports whether any mac/match selector is configured, so
// onLinkEvent can skip the per-event backend MAC lookup in the common case.
func (r *resolver) hasPermMACMatches() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.permMACs) > 0
}

// logicalsForLocked returns every logical name a monitor event for kernelName
// must reach: kernelName itself (the default identity mapping), names whose
// os-name selector points at it, names whose mac/match binding last resolved to
// it (so a down event invalidates them even though the device's MAC is no
// longer readable), and -- when devMatchMAC is known (an up/appeared event) --
// names whose mac/match selector equals the device's hardware MAC, so a freshly
// appeared device reaches a previously-deferred binding. Caller holds r.mu.
func (r *resolver) logicalsForLocked(kernelName, devMatchMAC string) []string {
	seen := make(map[string]struct{})
	var out []string
	add := func(ln string) {
		if _, ok := seen[ln]; ok {
			return
		}
		seen[ln] = struct{}{}
		out = append(out, ln)
	}
	add(kernelName)
	for _, ln := range r.logicalOf[kernelName] {
		add(ln)
	}
	for ln, dev := range r.boundOS {
		if dev == kernelName {
			add(ln)
		}
	}
	if devMatchMAC != "" {
		for _, ln := range r.permMACOf[normalizeMAC(devMatchMAC)] {
			add(ln)
		}
	}
	return out
}

// setMapping publishes the logical<->os-name and logical<->match-MAC selector
// maps after a config apply and drops the per-name cache and learned bindings
// (a config change can move any binding). The match-MAC values are normalized
// once here so the hot lookups compare canonical forms.
func (r *resolver) setMapping(logicalToOS, logicalToMAC map[string]string) {
	rev := make(map[string][]string, len(logicalToOS))
	for ln, osn := range logicalToOS {
		rev[osn] = append(rev[osn], ln)
	}
	macs := make(map[string]string, len(logicalToMAC))
	macRev := make(map[string][]string, len(logicalToMAC))
	for ln, mac := range logicalToMAC {
		n := normalizeMAC(mac)
		if n == "" {
			continue
		}
		macs[ln] = n
		macRev[n] = append(macRev[n], ln)
	}
	r.mu.Lock()
	r.osNames = logicalToOS
	r.logicalOf = rev
	r.permMACs = macs
	r.permMACOf = macRev
	r.cache = make(map[string]Binding)
	r.boundOS = make(map[string]string)
	r.mu.Unlock()
}

// bindEvents subscribes the resolver to the iface monitor's link events so it
// can invalidate its cache and drive Subscribe consumers. Called once by the
// iface component when the event bus is available; idempotent.
func (r *resolver) bindEvents(eb ze.EventBus) {
	if eb == nil {
		return
	}
	r.mu.Lock()
	if r.bound {
		r.mu.Unlock()
		return
	}
	r.bound = true
	r.mu.Unlock()

	sub := func(eventType string, kind LinkEventKind) func() {
		return eb.Subscribe(ifaceevents.Namespace, eventType, func(p any) {
			if name, idx, ok := decodeLinkEvent(p); ok {
				r.onLinkEvent(name, kind, idx)
			}
		})
	}
	u1 := sub(ifaceevents.EventCreated, LinkAppeared)
	u2 := sub(ifaceevents.EventUp, LinkUp)
	u3 := sub(ifaceevents.EventDown, LinkDown)

	r.mu.Lock()
	r.unsub = append(r.unsub, u1, u2, u3)
	r.mu.Unlock()
}

// decodeLinkEvent extracts the interface name and index from a monitor event
// payload. The monitor emits the payload as a JSON string (monitor_linux.go
// emit); both the link and state payload shapes carry name + index.
func decodeLinkEvent(p any) (string, int, bool) {
	s, ok := p.(string)
	if !ok {
		return "", 0, false
	}
	var sp struct {
		Name  string `json:"name"`
		Index int    `json:"index"`
	}
	if err := json.Unmarshal([]byte(s), &sp); err != nil || sp.Name == "" {
		return "", 0, false
	}
	return sp.Name, sp.Index, true
}

// setResolverConfig publishes the iface config's os-name mapping to the shared
// resolver. Called from the iface component's config-apply path.
func setResolverConfig(cfg *ifaceConfig) {
	globalResolver.setMapping(cfg.osNameMap(), cfg.permMACMap())
}

// bindResolverEvents wires the shared resolver to the event bus. Called from
// the iface component's config-apply path once a bus is available.
func bindResolverEvents(eb ze.EventBus) {
	globalResolver.bindEvents(eb)
}
