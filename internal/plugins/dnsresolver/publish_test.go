package dnsresolver

import (
	"encoding/json"
	"net/netip"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/dnssource"
)

// TestResolverPublishesSelectionOnceStarted proves the LAN resolver publishes
// its forward selection with the addresses it listens on, holds a selection
// built before the plugin could emit until setAnnouncer, and sends a body
// only when it changed. Method: apply a config before and after the sender
// is set, and count the bodies the sender received.
func TestResolverPublishesSelectionOnceStarted(t *testing.T) {
	res := newResolver(time.Now)
	cfg := defaultConfig()
	cfg.Forward = []upstream{{IP: netip.MustParseAddr("198.51.100.53"), Port: defaultPort}}
	cfg.Listeners = []listenerEndpoint{{Name: "lan", IP: netip.MustParseAddr("192.0.2.1"), Port: defaultPort}}
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
