// Design: docs/features/dns-resolver.md -- each consumer publishes its selection
// Related: state.go -- the policy builds that call forwardPolicy
// Related: register.go -- setAnnouncer, wired to the plugin's EmitEvent

package dnsresolver

import (
	"encoding/json"
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
}

// forwardPolicy builds cfg's policy over the current learned records and
// publishes the selection its forward list made. The caller MUST hold
// r.publish, because the announcement is guarded by it.
func (r *resolver) forwardPolicy(cfg *resolverConfig) *policy {
	forward, selection := forwardUpstreams(cfg, r.learned.Records(), r.now())
	r.announce(newResolverPublication(cfg, selection))
	return newPolicy(cfg, forward)
}

// newResolverPublication is the resolver's one selection, with the listener
// addresses it serves DNS on, so ze doctor can tell whether a self entry
// another consumer hands on is answered.
func newResolverPublication(cfg *resolverConfig, selection dnssource.Selection) *dnssource.Publication {
	current := dnssource.NewConsumerSelection(cfg.Sources, "", selection)
	for _, listener := range cfg.Listeners {
		if !slices.Contains(current.Serves, listener.IP) {
			current.Serves = append(current.Serves, listener.IP)
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
