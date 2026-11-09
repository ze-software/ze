// Design: docs/features/dns-resolver.md -- each consumer publishes its selection
// Related: handler.go -- selectDNS, the evaluation a reply reads too
// Related: config_tx.go -- install hands the publisher each new handler set
// Related: register.go -- followLearnedResolvers republishes on each snapshot

package dhcpserver

import (
	"encoding/json"
	"net/netip"
	"sync"

	"github.com/ze-software/ze/internal/core/dnssource"
)

// dhcpConsumer is the name the DHCP server publishes its selections under,
// its plugin name.
const dhcpConsumer = "dhcpserver"

// dhcpPublisher publishes the option 6 selection of every subnet the server
// runs, one ConsumerSelection per subnet named by its prefix, for show system
// dns and ze doctor. install hands it each new handler set (serve), and each
// new system-dns snapshot makes it evaluate again (refresh), because a
// subnet's answer changes with either. It sends only a body that differs from
// the last one sent. Safe for concurrent use: install and the event callback
// run on different goroutines.
type dhcpPublisher struct {
	mu       sync.Mutex
	handlers []*dhcpHandler
	// devices are the kernel devices the server listens on, whose addresses
	// inside a subnet are what its self class hands a client on that link.
	devices []string
	// send is nil until the plugin has started and can emit; a body built
	// before then waits in pending.
	send    func(body string)
	pending string
	sent    string
}

// serve records the handler set and the listeners install just made the
// running ones, and publishes their selections. A disabled or removed server
// passes no handler, which publishes an empty set, so the registry drops the
// subnets it held.
func (p *dhcpPublisher) serve(handlers []*dhcpHandler, listeners []dhcpListener) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.handlers = handlers
	p.devices = p.devices[:0]
	for _, ln := range listeners {
		p.devices = append(p.devices, ln.device)
	}
	p.publish()
}

// refresh evaluates every subnet again over the newest learned records.
func (p *dhcpPublisher) refresh() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.publish()
}

// setSender hands the publisher the function that emits a publication and
// sends the one built before the plugin could emit. It MUST be called once
// the plugin has started, before which an emit has no engine to reach.
func (p *dhcpPublisher) setSender(send func(body string)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.send = send
	p.flush()
}

// publish builds the publication of the current handler set. The caller MUST
// hold p.mu.
func (p *dhcpPublisher) publish() {
	publication := &dnssource.Publication{Consumer: dhcpConsumer, Selections: []dnssource.ConsumerSelection{}}
	for _, h := range p.handlers {
		self := p.selfAddress(h)
		current := dnssource.NewConsumerSelection(h.subnet.Sources, h.subnet.Prefix.String(), h.selectDNS(self))
		publication.Selections = append(publication.Selections, current)
	}
	body, err := json.Marshal(publication)
	if err != nil {
		loggerPtr.Load().Warn("dhcpserver: option 6 selections not published; show system dns shows the previous ones",
			"path", subnetDNSSourcePath, "error", err)
		return
	}
	p.pending = string(body)
	p.flush()
}

// selfAddress answers Ze's first address inside h's subnet on a listened
// device, the address a client on that link is handed by the self class
// (segment.selfAddress), and the invalid address when no device holds one
// there: a subnet served only through a relay. A device whose addresses
// cannot be read is logged and skipped. The caller MUST hold p.mu.
func (p *dhcpPublisher) selfAddress(h *dhcpHandler) netip.Addr {
	for _, device := range p.devices {
		addrs, err := dhcpDeviceAddrs(device)
		if err != nil {
			loggerPtr.Load().Warn("dhcpserver: published option 6 selection omits the self class of a device whose addresses could not be read",
				"interface", device, "subnet", h.subnet.Prefix, "error", err)
			continue
		}
		for _, addr := range addrs {
			if h.subnet.Prefix.Contains(addr) {
				return addr
			}
		}
	}
	return netip.Addr{}
}

// flush sends the pending body when one can be sent and it is not the body
// last sent. The caller MUST hold p.mu.
func (p *dhcpPublisher) flush() {
	// refuse-by-default: guards setSender -- no sender yet keeps the body pending until the plugin has started
	if p.send == nil {
		return
	}
	if p.pending == "" {
		return
	}
	if p.pending == p.sent {
		return
	}
	p.send(p.pending)
	p.sent = p.pending
}
