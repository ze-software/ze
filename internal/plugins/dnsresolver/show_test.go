package dnsresolver

import (
	"testing"

	"github.com/miekg/dns"

	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/plugin"
)

// TestShowAndClear proves AC-31 at the handlers: show reports the policy,
// the cache entries and the counters, and clear dns resolver cache all
// empties both caches so the next query goes upstream.
func TestShowAndClear(t *testing.T) {
	up := aUpstream(t, 0)
	r := newTestResolver(t, testConfig(up), newTestClock())
	activeResolver.Store(r)
	t.Cleanup(func() { activeResolver.Store(nil) })

	ask(r, query("a.test", dns.TypeA), "127.0.0.1", false)
	ask(r, query("a.test", dns.TypeA), "127.0.0.1", false)
	ask(r, query("a.test", dns.TypeA), "10.0.0.1", false)

	resp, err := handleShowDNSResolver(nil, commandtest.Args())
	if err != nil {
		t.Fatal(err)
	}
	data, ok := resp.Data.(plugin.Map)
	if !ok {
		t.Fatalf("show data %T", resp.Data)
	}
	for key, want := range map[string]any{
		"enabled":              true,
		"mode":                 "forward",
		"allow-from":           1,
		"cache-answer-entries": 1,
		"cache-hits":           uint64(1),
		"cache-misses":         uint64(1),
		"cache-hit-rate":       0.5,
		"refused-acl":          uint64(1),
		"dropped":              uint64(0),
		"rate-limited":         uint64(0),
	} {
		if data[key] != want {
			t.Errorf("show %s = %v (%T), want %v (%T)", key, data[key], data[key], want, want)
		}
	}
	byResult, ok := data["upstream-queries"].(plugin.Map)
	if !ok || byResult["answered"] != uint64(1) {
		t.Errorf("upstream-queries %v, want answered 1", data["upstream-queries"])
	}
	if ups, ok := data["upstreams"].([]string); !ok || len(ups) != 1 || ups[0] != up.upstream().addrPort().String() {
		t.Errorf("upstreams %v", data["upstreams"])
	}

	if _, err := handleClearDNSResolverCache(nil, commandtest.Args()); err != nil {
		t.Fatal(err)
	}
	if n := r.answers.size(); n != 0 {
		t.Fatalf("answer cache holds %d after clear", n)
	}
	ask(r, query("a.test", dns.TypeA), "127.0.0.1", false)
	if n := up.queries.Load(); n != 2 {
		t.Fatalf("upstream saw %d queries, want 2: clear left the entry", n)
	}

	activeResolver.Store(nil)
	if _, err := handleClearDNSResolverCache(nil, commandtest.Args()); err == nil {
		t.Fatal("clear with no resolver running answered no error")
	}
}
