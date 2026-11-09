package cmd

import (
	"encoding/json"
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config/system"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/dnssource"
)

// TestShowSystemDNSAnswersRouterAndConsumers proves AC-12's answer shape:
// the router's selection comes first under consumer "system" with its path,
// mode, default marker, the classes used and its servers, followed by each
// published consumer. Method: a fresh registry configured with one
// name-server under a built-in default list, and one published consumer,
// through the RPC handler body.
func TestShowSystemDNSAnswersRouterAndConsumers(t *testing.T) {
	var registry system.Resolvers
	routerList := dnssource.StatedDefault("system dns-source", dnssource.FamilyAll, dnssource.AudienceZe,
		dnssource.ModeFirstUsable, dnssource.ClassConfigured, dnssource.ClassReceived)
	if err := registry.Configure("", []string{"198.51.100.1"}, routerList); err != nil {
		t.Fatalf("Configure: %v", err)
	}
	resolverServes := netip.MustParseAddr("192.0.2.1")
	if err := registry.SetPublication(&dnssource.Publication{
		Consumer: "dns-resolver",
		Selections: []dnssource.ConsumerSelection{{
			Path:    "service dns-resolver forward dns-source",
			Sources: []dnssource.Class{dnssource.ClassReceived},
			Default: true,
			Serves:  []netip.Addr{resolverServes},
		}},
	}); err != nil {
		t.Fatalf("SetPublication: %v", err)
	}

	resp, err := showSystemDNS(&registry)
	if err != nil {
		t.Fatalf("showSystemDNS: %v", err)
	}
	if resp.Status != plugin.StatusDone {
		t.Fatalf("status = %q, error %q", resp.Status, resp.Error)
	}
	raw, ok := resp.Data.(plugin.RawJSON)
	if !ok {
		t.Fatalf("data is %T, want plugin.RawJSON", resp.Data)
	}
	var answer systemDNSAnswer
	if err := json.Unmarshal([]byte(raw), &answer); err != nil {
		t.Fatalf("answer is not JSON: %v\n%s", err, raw)
	}
	if answer.Records == nil {
		t.Fatal("records absent; want an empty list when nothing was learned")
	}
	if len(answer.Consumers) != 2 {
		t.Fatalf("consumers = %d, want router + dns-resolver: %s", len(answer.Consumers), raw)
	}
	router := answer.Consumers[0]
	if router.Consumer != routerConsumer || len(router.Selections) != 1 {
		t.Fatalf("first consumer = %+v, want the router under %q", router, routerConsumer)
	}
	selection := router.Selections[0]
	if !selection.Default {
		t.Errorf("router selection not marked default under a built-in default list: %s", raw)
	}
	if selection.Path != "system dns-source" {
		t.Errorf("router path = %q, want system dns-source", selection.Path)
	}
	if len(selection.Used) != 1 || selection.Used[0] != dnssource.ClassConfigured {
		t.Errorf("router used = %v, want [configured]", selection.Used)
	}
	if len(selection.Servers) != 1 || selection.Servers[0] != netip.MustParseAddr("198.51.100.1") {
		t.Errorf("router servers = %v, want [198.51.100.1]", selection.Servers)
	}
	resolver := answer.Consumers[1]
	if resolver.Consumer != "dns-resolver" || len(resolver.Selections) != 1 || resolver.Selections[0].Serves[0] != resolverServes {
		t.Fatalf("second consumer = %+v, want the published dns-resolver selection", resolver)
	}
	if !strings.Contains(string(raw), `"mode"`) {
		t.Errorf("answer has no mode key: %s", raw)
	}
}
