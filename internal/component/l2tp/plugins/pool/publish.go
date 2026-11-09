// Design: docs/features/dns-resolver.md -- each consumer publishes its selection
// Related: dns_source.go -- poolDNSCandidates, the classes every session shares
// Related: register.go -- setEventBus and the config callbacks call publishPools

package l2tppool

import (
	"encoding/json"
	"net/netip"
	"slices"
	"sync"
	"time"

	"github.com/ze-software/ze/internal/core/dnssource"
	"github.com/ze-software/ze/pkg/ze"
)

// poolConsumer is the name the pool plugin publishes its selections under.
const poolConsumer = Name

// poolPublished remembers the body of the last publication sent. Safe for
// concurrent use.
type poolPublished struct {
	mu   sync.Mutex
	body string
	// withdrawn is set once the withdrawal went out (withdraw): no later
	// set is sent, so a config callback or snapshot that arrives during the
	// stop cannot list the pools again.
	withdrawn bool
}

// reset forgets the last body and any withdrawal, so the next publication is
// sent whatever it holds: a new bus has not seen it, and the plugin run that
// installed the bus publishes again.
func (pp *poolPublished) reset() {
	pp.mu.Lock()
	defer pp.mu.Unlock()
	pp.body = ""
	pp.withdrawn = false
}

// currentBus answers the bus setEventBus installed, nil before it.
func (p *poolPlugin) currentBus() ze.EventBus {
	p.busMu.Lock()
	defer p.busMu.Unlock()
	return p.bus
}

// publishPools publishes the subscriber list of every running IPv4 pool for
// show system dns and ze doctor: the default pool with no instance, each
// named pool under its name. RADIUS names a session's servers when the
// session asks, so the published selection is what a session RADIUS gave no
// MS DNS server gets: the radius class contributes nothing here. It sends
// only a set that differs from the last one sent, nothing while no bus is
// installed, and nothing after withdraw until a new run installs a bus.
func (p *poolPlugin) publishPools(bus ze.EventBus) {
	// refuse-by-default: guards setEventBus -- before the bus is installed nobody can read a publication, and setEventBus publishes the whole set
	if bus == nil {
		return
	}
	publication := p.poolPublication(p.learned.Records(), time.Now())
	body, err := json.Marshal(publication)
	if err != nil {
		logger().Warn("l2tp-pool: subscriber DNS selections not published; show system dns shows the previous ones", "error", err)
		return
	}
	p.published.mu.Lock()
	defer p.published.mu.Unlock()
	if p.published.withdrawn {
		return
	}
	if string(body) == p.published.body {
		return
	}
	if _, err := dnssource.PublicationEvent.Emit(bus, publication); err != nil {
		logger().Warn("l2tp-pool: subscriber DNS selections not published; show system dns shows the previous ones", "error", err)
		return
	}
	p.published.body = string(body)
}

// withdraw sends the pool plugin's withdrawal on the bus setEventBus
// installed, so the registry forgets every pool's selection, and stops every
// later publication until a new run installs a bus. It MUST be called from the
// plugin's bye handler, while the registry still reads the bus. Safe for
// concurrent use.
func (p *poolPlugin) withdraw() {
	bus := p.currentBus()
	p.published.mu.Lock()
	defer p.published.mu.Unlock()
	p.published.withdrawn = true
	// Nothing was ever sent, so the registry holds nothing to forget. A body
	// is recorded only after an emit on an installed bus, so bus is set here.
	if p.published.body == "" {
		return
	}
	if _, err := dnssource.PublicationEvent.Emit(bus, dnssource.NewWithdrawal(poolConsumer)); err != nil {
		logger().Warn("l2tp-pool: subscriber DNS selections not withdrawn; show system dns keeps reporting them until the daemon restarts",
			"error", err)
		return
	}
	p.published.body = ""
}

// poolPublication builds the publication of the running pools over records
// at now, the named pools sorted by name.
func (p *poolPlugin) poolPublication(records []dnssource.Record, now time.Time) *dnssource.Publication {
	p.mu.RLock()
	defer p.mu.RUnlock()
	publication := &dnssource.Publication{Consumer: poolConsumer, Selections: []dnssource.ConsumerSelection{}}
	if p.pool != nil {
		publication.Selections = append(publication.Selections, poolSelection(p.pool, "", records, now))
	}
	names := make([]string, 0, len(p.namedPools))
	for name := range p.namedPools {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		publication.Selections = append(publication.Selections, poolSelection(p.namedPools[name], name, records, now))
	}
	return publication
}

// poolSelection is pool's list as published under instance, its servers cut
// at what IPCP carries.
func poolSelection(pool *ipv4Pool, instance string, records []dnssource.Record, now time.Time) dnssource.ConsumerSelection {
	selection := dnssource.Select(pool.sources, poolDNSCandidates(pool, records, now))
	current := dnssource.NewConsumerSelection(pool.sources, instance, selection)
	if len(current.Servers) > ipcpDNSSlots {
		current.Servers = current.Servers[:ipcpDNSSlots]
	}
	current.Self = slices.DeleteFunc(current.Self, func(addr netip.Addr) bool { return !slices.Contains(current.Servers, addr) })
	return current
}
