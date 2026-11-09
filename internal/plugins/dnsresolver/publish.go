// Design: docs/features/dns-resolver.md -- each consumer publishes its selection
// Related: state.go -- the policy builds that call forwardPolicy
// Related: register.go -- setAnnouncer, wired to the plugin's EmitEvent
// Related: listener_state.go -- serving, the bound listener addresses a publication lists

package dnsresolver

import (
	"encoding/json"
	"net/netip"
	"slices"

	"github.com/ze-software/ze/internal/core/dnssource"
)

// resolverConsumer is the name the LAN resolver publishes its selection
// under, its plugin name.
const resolverConsumer = "dns-resolver"

// announcement is the resolver's publication state: the body it last built
// and the body it last handed to send. send is nil until the plugin has
// started and can emit; a body built before then is sent when it is set.
type announcement struct {
	send    func(body string)
	pending string
	sent    string
	// built and selection are the config and the forward selection the last
	// publication was built from, so a listener edge publishes them again
	// with the sockets bound at that moment. built is nil before the first
	// policy is built.
	built     *resolverConfig
	selection dnssource.Selection
}

// forwardPolicy builds cfg's policy over the current learned records and
// publishes the selection its forward list made. The caller MUST hold
// r.publish, because the announcement is guarded by it.
func (r *resolver) forwardPolicy(cfg *resolverConfig) *policy {
	forward, selection := forwardUpstreams(cfg, r.learned.Records(), r.now())
	built := *cfg
	r.announcement.built = &built
	r.announcement.selection = selection
	r.announce(newResolverPublication(cfg, selection, r.listeners.serving(cfg.Listeners)))
	return newPolicy(cfg, forward)
}

// onListenerChange is the harness's OnListenerChange: it records the edge
// for show and the gauge, then publishes the selection again with the
// sockets bound now, so Serves follows a bind, an unbind and a crash. The
// harness calls it with none of its locks held, and never while r.publish
// is held: apply and narrow release it before the transaction reconciles.
func (r *resolver) onListenerChange(protocol string, listen netip.AddrPort, up bool) {
	r.listeners.change(protocol, listen, up)
	r.publish.Lock()
	defer r.publish.Unlock()
	built := r.announcement.built
	// No policy was built yet, so there is no selection to publish: the
	// first apply publishes one with the sockets bound by then.
	if built == nil {
		return
	}
	r.announce(newResolverPublication(built, r.announcement.selection, r.listeners.serving(built.Listeners)))
}

// newResolverPublication is the resolver's one selection, with serving, the
// listener addresses whose UDP socket is bound, so ze doctor can tell whether
// a self entry another consumer hands on is answered. A configured listener
// that is not bound is not served, so it is not listed: listing it would hide
// the doctor-dns-source-self-unserved warning while clients time out.
//
// A disabled resolver binds nothing and answers nobody, so it publishes no
// selection at all: the consumer stays listed with an empty set, the form a
// consumer with no list publishes, and neither show system dns nor ze doctor
// reports a forward list nothing uses.
func newResolverPublication(cfg *resolverConfig, selection dnssource.Selection, serving []netip.Addr) *dnssource.Publication {
	if !cfg.Enabled {
		return &dnssource.Publication{Consumer: resolverConsumer, Selections: []dnssource.ConsumerSelection{}}
	}
	current := dnssource.NewConsumerSelection(cfg.Sources, "", selection)
	for _, ip := range serving {
		if !slices.Contains(current.Serves, ip) {
			current.Serves = append(current.Serves, ip)
		}
	}
	return &dnssource.Publication{Consumer: resolverConsumer, Selections: []dnssource.ConsumerSelection{current}}
}

// announce records publication and sends it when it differs from the last
// one sent. The caller MUST hold r.publish.
func (r *resolver) announce(publication *dnssource.Publication) {
	body, err := json.Marshal(publication)
	if err != nil {
		loggerPtr.Load().Warn("dns-resolver: selection not published", "path", forwardDNSSourcePath, "error", err)
		return
	}
	r.announcement.pending = string(body)
	r.flushAnnouncement()
}

// setAnnouncer hands the resolver the function that emits a publication and
// sends the one built before the plugin could emit. It MUST be called once
// the plugin has started, before which an emit has no engine to reach.
func (r *resolver) setAnnouncer(send func(body string)) {
	r.publish.Lock()
	defer r.publish.Unlock()
	r.announcement.send = send
	r.flushAnnouncement()
}

// flushAnnouncement sends the pending body when one can be sent and it is
// not the body last sent. The caller MUST hold r.publish.
func (r *resolver) flushAnnouncement() {
	// refuse-by-default: guards setAnnouncer -- no sender yet keeps the body pending until the plugin has started
	if r.announcement.send == nil {
		return
	}
	if r.announcement.pending == "" {
		return
	}
	if r.announcement.pending == r.announcement.sent {
		return
	}
	r.announcement.send(r.announcement.pending)
	r.announcement.sent = r.announcement.pending
}
