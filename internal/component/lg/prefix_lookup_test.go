// Design: docs/architecture/api/birdwatcher-compat.md -- the prefix lookup, an address or a prefix
// Related: prefix_lookup.go -- parsePrefixLookup, the rule the three writers share
// Related: ribfake_test.go -- the fake answers `covering` with the plugin's meaning

package lg

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
)

// TestParsePrefixLookup proves the one rule every prefix field follows.
//
// VALIDATES: a prefix with its length becomes the `exact` selector, its host
// bits cleared, and a bare address the `covering` selector, each carrying the
// text net/netip writes; the length bounds hold at 32 for IPv4 and 128 for
// IPv6; every other value is refused with an operator error that repeats it
// and ends on a valid form.
// PREVENTS: a host address sent as a text match, which finds only a /32 route
// and never the prefix covering it; a prefix sent as the text filter
// `prefix`, which lists 192.0.0.0/24 for 192.0.0.0/2; a prefix with host bits
// set, which bgp-rib's `exact` refuses; and user text reaching the command
// line.
func TestParsePrefixLookup(t *testing.T) {
	accepted := []struct {
		name, input, keyword, value string
	}{
		{"ipv4 address", "10.1.2.3", filterCovering, "10.1.2.3"},
		{"ipv4 prefix", "10.0.0.0/8", filterExact, "10.0.0.0/8"},
		{"ipv4 prefix with one digit of length", "192.0.0.0/2", filterExact, "192.0.0.0/2"},
		{"ipv4 host bits cleared", "10.0.0.1/8", filterExact, "10.0.0.0/8"},
		{"ipv4 last valid length", "10.0.0.0/32", filterExact, "10.0.0.0/32"},
		{"ipv4 default route", "0.0.0.0/0", filterExact, "0.0.0.0/0"},
		{"ipv6 address written in capitals", "2001:DB8::1", filterCovering, "2001:db8::1"},
		{"ipv6 prefix", "2001:db8::/32", filterExact, "2001:db8::/32"},
		{"ipv6 host bits cleared", "2001:db8::1/32", filterExact, "2001:db8::/32"},
		{"ipv6 last valid length", "2001:db8::1/128", filterExact, "2001:db8::1/128"},
	}
	for _, tt := range accepted {
		t.Run(tt.name, func(t *testing.T) {
			lookup, err := parsePrefixLookup(tt.input)
			if err != nil {
				t.Fatalf("parsePrefixLookup(%q) refused: %v", tt.input, err)
			}
			if lookup.keyword != tt.keyword || lookup.value != tt.value {
				t.Errorf("parsePrefixLookup(%q) = %s %s, want %s %s",
					tt.input, lookup.keyword, lookup.value, tt.keyword, tt.value)
			}
		})
	}

	refused := []struct {
		name, input, why, fix string
	}{
		{"ipv4 length past 32", "10.0.0.0/33", "an IPv4 prefix length is 0 to 32", "10.0.0.0/32"},
		{"ipv6 length past 128", "2001:db8::/129", "an IPv6 prefix length is 0 to 128", "2001:db8::/128"},
		{"zone", "fe80::1%eth0", "a route prefix has no zone", "fe80::1"},
		{"not an address", "not-a-prefix", "the field takes an address", "192.0.2.0/24"},
		{"negative length", "10.0.0.0/-1", "the field takes an address", "192.0.2.0/24"},
		{"space injection after a prefix", "10.0.0.0/24 rm", "the field takes an address", "192.0.2.0/24"},
		{"keyword injection after an address", "10.0.0.1 prefix 1", "the field takes an address", "192.0.2.0/24"},
		{"pipe", "10.0.0.1|count", "the field takes an address", "192.0.2.0/24"},
	}
	for _, tt := range refused {
		t.Run(tt.name, func(t *testing.T) {
			lookup, err := parsePrefixLookup(tt.input)
			if err == nil {
				t.Fatalf("parsePrefixLookup(%q) = %s %s, want a refusal", tt.input, lookup.keyword, lookup.value)
			}
			message := err.Error()
			lines := strings.Split(message, "\n")
			if !strings.Contains(lines[0], tt.input) {
				t.Errorf("first line %q does not name the value %q", lines[0], tt.input)
			}
			if !strings.Contains(message, tt.why) {
				t.Errorf("refusal %q does not say %q", message, tt.why)
			}
			if last := lines[len(lines)-1]; last != "fix: "+tt.fix {
				t.Errorf("last line = %q, want %q", last, "fix: "+tt.fix)
			}
		})
	}
}

// TestPrefixLookupReceivedCommand proves the command the API and the graph
// send: the received scope, then the one filter the field became.
func TestPrefixLookupReceivedCommand(t *testing.T) {
	for input, want := range map[string]string{
		"10.1.2.3":   "show bgp rib | received | covering 10.1.2.3",
		"10.0.0.0/8": "show bgp rib | received | exact 10.0.0.0/8",
		"10.0.0.1/8": "show bgp rib | received | exact 10.0.0.0/8",
	} {
		lookup, err := parsePrefixLookup(input)
		if err != nil {
			t.Fatalf("parsePrefixLookup(%q): %v", input, err)
		}
		if got := lookup.receivedCommand(); got != want {
			t.Errorf("receivedCommand for %q = %q, want %q", input, got, want)
		}
	}
}

// TestParseExactPrefixLookup proves the route detail's rule: one prefix,
// asked for exactly, and a bare address refused.
//
// VALIDATES: a prefix with its length becomes `exact` with its host bits
// cleared; a bare address is refused with its host prefix as the valid form;
// a value that is neither gets parsePrefixLookup's refusal.
// PREVENTS: a detail for an address answered by whichever covering route
// came first.
func TestParseExactPrefixLookup(t *testing.T) {
	lookup, err := parseExactPrefixLookup("192.0.0.1/2")
	if err != nil {
		t.Fatalf("parseExactPrefixLookup(192.0.0.1/2) refused: %v", err)
	}
	if lookup.keyword != filterExact || lookup.value != "192.0.0.0/2" {
		t.Errorf("parseExactPrefixLookup(192.0.0.1/2) = %s %s, want exact 192.0.0.0/2", lookup.keyword, lookup.value)
	}

	for input, fix := range map[string]string{
		"10.0.0.1":     "fix: 10.0.0.1/32",
		"2001:db8::1":  "fix: 2001:db8::1/128",
		"not-a-prefix": "fix: 192.0.2.0/24",
		"10.0.0.0/33":  "fix: 10.0.0.0/32",
	} {
		_, err := parseExactPrefixLookup(input)
		if err == nil {
			t.Fatalf("parseExactPrefixLookup(%q) accepted, want a refusal", input)
		}
		lines := strings.Split(err.Error(), "\n")
		if !strings.Contains(lines[0], input) {
			t.Errorf("first line %q does not name the value %q", lines[0], input)
		}
		if last := lines[len(lines)-1]; last != fix {
			t.Errorf("refusal of %q ends %q, want %q", input, last, fix)
		}
	}
}

// lookupRequest drives one request through the server's own handler chain,
// answered by mockDispatch, and returns the response with every command the
// handler dispatched. form, when not empty, is sent as a POST body.
func lookupRequest(t *testing.T, target, form string) (*httptest.ResponseRecorder, []string) {
	t.Helper()

	var mu sync.Mutex
	var commands []string
	answer := mockDispatch()
	recording := func(ctx context.Context, caller plugin.CallerIdentity, cmd string) (*plugin.Response, error) {
		mu.Lock()
		commands = append(commands, cmd)
		mu.Unlock()
		return answer(ctx, caller, cmd)
	}

	srv, err := NewLGServer(LGConfig{
		ListenAddrs: []ListenAddr{{Addr: "127.0.0.1:0"}},
		Dispatch:    recording,
	})
	if err != nil {
		t.Fatalf("new lg server: %v", err)
	}

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, http.NoBody)
	if form != "" {
		req = httptest.NewRequestWithContext(t.Context(), http.MethodPost, target, strings.NewReader(form))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	rec := httptest.NewRecorder()
	srv.server.Handler.ServeHTTP(rec, req)

	mu.Lock()
	defer mu.Unlock()
	return rec, commands
}

// ribCommands keeps the `show bgp rib` commands of a recorded request, the
// ones a prefix lookup is carried by.
func ribCommands(commands []string) []string {
	var rib []string
	for _, cmd := range commands {
		if strings.HasPrefix(cmd, "show bgp rib ") {
			rib = append(rib, cmd)
		}
	}
	return rib
}
