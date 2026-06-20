// Design: plan/spec-iface-resolve-2-resolver.md -- shared logical-name resolver
// Overview: iface.go -- Binding / LinkEvent value types
// Related: dispatch.go -- GetInterface backend dispatch this builds on
// Related: events/events.go -- monitor event namespace + constants

package iface

import (
	"encoding/json"
	"net/netip"
	"sync"

	ifaceevents "codeberg.org/thomas-mangin/ze/internal/component/iface/events"
	"codeberg.org/thomas-mangin/ze/pkg/ze"
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
	bound     bool
	unsub     []func()
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

	osName := r.effectiveOSName(name)
	info, err := GetInterface(osName)
	if err != nil {
		return Binding{}, err
	}
	b := bindingFromInfo(osName, info)

	r.mu.Lock()
	r.cache[name] = b
	r.mu.Unlock()
	return b, nil
}

func (r *resolver) addresses(name string) ([]AddrInfo, error) {
	osName := r.effectiveOSName(name)
	info, err := GetInterface(osName)
	if err != nil {
		return nil, err
	}
	return classifyAddresses(info.Addresses), nil
}

// effectiveOSName translates a logical interface name to its OS device name:
// the configured os-name override when present, otherwise the logical name
// itself (so every name == os-name config resolves unchanged).
func (r *resolver) effectiveOSName(name string) string {
	r.mu.Lock()
	osn := r.osNames[name]
	r.mu.Unlock()
	if osn != "" {
		return osn
	}
	return name
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
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, logical := range r.logicalsForLocked(kernelName) {
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

// logicalsForLocked returns every logical name that resolves to kernelName:
// kernelName itself (the default mapping) plus any names whose os-name selector
// points at it. Caller holds r.mu.
func (r *resolver) logicalsForLocked(kernelName string) []string {
	out := []string{kernelName}
	for _, ln := range r.logicalOf[kernelName] {
		if ln != kernelName {
			out = append(out, ln)
		}
	}
	return out
}

// setMapping publishes the logical<->os-name mapping after a config apply and
// drops the cache (a config change can move a binding).
func (r *resolver) setMapping(logicalToOS map[string]string) {
	rev := make(map[string][]string, len(logicalToOS))
	for ln, osn := range logicalToOS {
		rev[osn] = append(rev[osn], ln)
	}
	r.mu.Lock()
	r.osNames = logicalToOS
	r.logicalOf = rev
	r.cache = make(map[string]Binding)
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
	globalResolver.setMapping(cfg.osNameMap())
}

// bindResolverEvents wires the shared resolver to the event bus. Called from
// the iface component's config-apply path once a bus is available.
func bindResolverEvents(eb ze.EventBus) {
	globalResolver.bindEvents(eb)
}
