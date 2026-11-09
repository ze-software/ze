package dnsresolver

import (
	"encoding/json"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/dnssource"
)

// decodePublication decodes one body the announcer sent.
func decodePublication(t *testing.T, body string) dnssource.Publication {
	t.Helper()
	var publication dnssource.Publication
	if err := json.Unmarshal([]byte(body), &publication); err != nil {
		t.Fatalf("body is not a Publication: %v\n%s", err, body)
	}
	return publication
}

// TestResolverPublishesSelectionOnceStarted proves the LAN resolver publishes
// its forward selection with the addresses it listens on, holds a selection
// built before the plugin could emit until setAnnouncer, and sends a body
// only when it changed. Method: report the listener's UDP socket bound, as
// the harness does, apply a config before and after the sender is set, and
// count the bodies the sender received.
func TestResolverPublishesSelectionOnceStarted(t *testing.T) {
	res := newResolver(time.Now)
	cfg := defaultConfig()
	cfg.Enabled = true
	cfg.Forward = []upstream{{IP: netip.MustParseAddr("198.51.100.53"), Port: defaultPort}}
	cfg.Listeners = []listenerEndpoint{{Name: "lan", IP: netip.MustParseAddr("192.0.2.1"), Port: defaultPort}}
	res.onListenerChange(networkUDP, netip.AddrPortFrom(cfg.Listeners[0].IP, cfg.Listeners[0].Port), true)
	res.apply(&cfg)

	var bodies []string
	res.setAnnouncer(func(body string) { bodies = append(bodies, body) })
	if len(bodies) != 1 {
		t.Fatalf("setAnnouncer sent %d bodies, want the 1 built before it", len(bodies))
	}
	res.apply(&cfg)
	if len(bodies) != 1 {
		t.Fatalf("an unchanged selection was sent again: %d bodies", len(bodies))
	}

	var publication dnssource.Publication
	if err := json.Unmarshal([]byte(bodies[0]), &publication); err != nil {
		t.Fatalf("body is not a Publication: %v\n%s", err, bodies[0])
	}
	if publication.Consumer != resolverConsumer || len(publication.Selections) != 1 {
		t.Fatalf("publication = %+v, want one selection under %q", publication, resolverConsumer)
	}
	selection := publication.Selections[0]
	if selection.Path != forwardDNSSourcePath || !selection.Default {
		t.Errorf("selection path %q default %v, want %q under the built-in default", selection.Path, selection.Default, forwardDNSSourcePath)
	}
	if len(selection.Servers) != 1 || selection.Servers[0] != cfg.Forward[0].IP {
		t.Errorf("servers = %v, want the forward server", selection.Servers)
	}
	if len(selection.Serves) != 1 || selection.Serves[0] != cfg.Listeners[0].IP {
		t.Errorf("serves = %v, want the listener address", selection.Serves)
	}

	cfg.Forward = []upstream{{IP: netip.MustParseAddr("198.51.100.54"), Port: defaultPort}}
	res.apply(&cfg)
	if len(bodies) != 2 {
		t.Fatalf("a changed selection was not sent: %d bodies", len(bodies))
	}
}

// TestResolverServesOnlyBoundListeners proves the publication lists a
// listener address as served only while its UDP socket is bound, so ze doctor
// warns doctor-dns-source-self-unserved for a self address nothing answers.
// Method: apply a config naming one listener, then feed the bind and unbind
// edges the harness reports through OnListenerChange, and read Serves in the
// body each edge sends.
//
// VALIDATES: spec-home-cpe-23b D-6, the resolver's Serves are the addresses
// it answers on. PREVENTS: a configured but unbound listener read as served.
func TestResolverServesOnlyBoundListeners(t *testing.T) {
	res := newResolver(time.Now)
	cfg := defaultConfig()
	cfg.Enabled = true
	listener := listenerEndpoint{Name: "lan", IP: netip.MustParseAddr("192.0.2.1"), Port: defaultPort}
	listen := netip.AddrPortFrom(listener.IP, listener.Port)
	cfg.Listeners = []listenerEndpoint{listener}
	res.apply(&cfg)

	var bodies []string
	res.setAnnouncer(func(body string) { bodies = append(bodies, body) })
	serves := func() []netip.Addr {
		t.Helper()
		publication := decodePublication(t, bodies[len(bodies)-1])
		if len(publication.Selections) != 1 {
			t.Fatalf("publication = %+v, want one selection", publication)
		}
		return publication.Selections[0].Serves
	}

	if got := serves(); len(got) != 0 {
		t.Fatalf("serves = %v before any socket is bound, want none", got)
	}
	res.onListenerChange(networkUDP, listen, true)
	if got := serves(); len(got) != 1 || got[0] != listener.IP {
		t.Fatalf("serves = %v once UDP is bound, want [%v]", got, listener.IP)
	}
	res.onListenerChange(networkUDP, listen, false)
	if got := serves(); len(got) != 0 {
		t.Fatalf("serves = %v once UDP is unbound, want none", got)
	}
}

// TestDisabledResolverPublishesNoSelection proves a disabled resolver, which
// binds nothing, publishes an empty selection set: neither its forward list
// nor its configured listener addresses. Method: apply a disabled config that
// names a forward server and a listener, then decode the body sent.
//
// VALIDATES: spec-home-cpe-23b, a consumer that answers nobody hands on no
// selection. PREVENTS: ze doctor reading a disabled resolver's listener as
// served, or warning doctor-dns-source-empty for it.
func TestDisabledResolverPublishesNoSelection(t *testing.T) {
	res := newResolver(time.Now)
	cfg := defaultConfig()
	cfg.Enabled = false
	cfg.Forward = []upstream{{IP: netip.MustParseAddr("198.51.100.53"), Port: defaultPort}}
	cfg.Listeners = []listenerEndpoint{{Name: "lan", IP: netip.MustParseAddr("192.0.2.1"), Port: defaultPort}}
	res.apply(&cfg)

	var bodies []string
	res.setAnnouncer(func(body string) { bodies = append(bodies, body) })
	if len(bodies) != 1 {
		t.Fatalf("setAnnouncer sent %d bodies, want 1", len(bodies))
	}
	publication := decodePublication(t, bodies[0])
	if publication.Consumer != resolverConsumer {
		t.Fatalf("consumer = %q, want %q", publication.Consumer, resolverConsumer)
	}
	if publication.Selections == nil || len(publication.Selections) != 0 {
		t.Fatalf("selections = %#v, want an empty set", publication.Selections)
	}
}

// TestResolverWithdrawPublishesEmptySelection proves withdraw, called
// directly, sends one withdrawn publication with no selection once the
// resolver has published, and that nothing is published after it, so the
// listener down edges of the stop that follows do not list the consumer
// again. Method: apply and start the announcer, call withdraw, then feed a
// listener edge and apply again, and decode the bodies sent. It does not
// drive the registered OnBye handler; the daemon-level proof that a removed
// resolver leaves show system dns is
// test/draft/plugin/show-system-dns-withdraw-on-removal.ci.
//
// VALIDATES: spec-home-cpe-23b, the withdrawal body and the silence after it.
// PREVENTS: a withdrawn resolver publishing its selection again.
func TestResolverWithdrawPublishesEmptySelection(t *testing.T) {
	res := newResolver(time.Now)
	cfg := defaultConfig()
	cfg.Enabled = true
	cfg.Listeners = []listenerEndpoint{{Name: "lan", IP: netip.MustParseAddr("192.0.2.1"), Port: defaultPort}}
	res.apply(&cfg)
	var bodies []string
	res.setAnnouncer(func(body string) { bodies = append(bodies, body) })

	res.withdraw()
	res.onListenerChange(networkUDP, netip.AddrPortFrom(cfg.Listeners[0].IP, cfg.Listeners[0].Port), true)
	res.apply(&cfg)
	if len(bodies) != 2 {
		t.Fatalf("sent %d bodies, want the publication then only the withdrawal: %q", len(bodies), bodies)
	}
	publication := decodePublication(t, bodies[1])
	if publication.Consumer != resolverConsumer || !publication.Withdrawn || len(publication.Selections) != 0 {
		t.Fatalf("withdrawal = %+v, want a withdrawn %q with no selection", publication, resolverConsumer)
	}
}
