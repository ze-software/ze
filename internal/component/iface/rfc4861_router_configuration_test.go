// RFC 4861 Section 6.2.1 and 6.2.2 obligations on the router configuration
// variables and on which interfaces advertise.
//
// VALIDATES: each bounded router configuration variable is refused at config
// verify one step outside its range and accepted at its edge; advertising is
// off unless the operator turns it on; only an advertising unit gets a sender.
// PREVENTS: an advertisement a conforming host rejects, and Router
// Advertisements on an interface nobody configured to send them.

package iface

import (
	"log/slog"
	"strings"
	"testing"
)

// rfc4861RefusedNaming parses body, requires a refusal, and requires the
// refusal to name leaf, so a refusal for another leaf cannot pass the case.
func rfc4861RefusedNaming(t *testing.T, body, leaf string) {
	t.Helper()
	if msg := mustRejectRA(t, body); !strings.Contains(msg, leaf) {
		t.Errorf("refusal %q does not name %s", msg, leaf)
	}
}

// RFC requirement: RFC4861-6.2.1-3 negative -- maximum-interval 3 and 1801 are
// refused at config verify, naming maximum-interval, while 4 and 1800 are
// accepted.
// RFC requirement: RFC4861-6.2.1-3 positive -- maximum-interval 4 and 1800, the edges of the range, are accepted.
func TestRFC4861MaxRtrAdvIntervalBounds(t *testing.T) {
	rfc4861RefusedNaming(t, `{"enabled": "true", "maximum-interval": "3", "minimum-interval": "3", "router-lifetime": "0"}`, "maximum-interval")
	rfc4861RefusedNaming(t, `{"enabled": "true", "maximum-interval": "1801", "router-lifetime": "0"}`, "maximum-interval")
	mustParseRA(t, `{"enabled": "true", "maximum-interval": "4", "minimum-interval": "3", "router-lifetime": "0"}`)
	mustParseRA(t, `{"enabled": "true", "maximum-interval": "1800", "router-lifetime": "0"}`)
}

// RFC requirement: RFC4861-6.2.1-4 negative -- with maximum-interval 600,
// minimum-interval 2 and 451 (above 0.75 * 600) are refused naming
// minimum-interval, while 3 and 450 are accepted.
// RFC requirement: RFC4861-6.2.1-4 positive -- with maximum-interval 600, minimum-interval 3 and 450, the edges of the range, are accepted.
func TestRFC4861MinRtrAdvIntervalBounds(t *testing.T) {
	rfc4861RefusedNaming(t, `{"enabled": "true", "maximum-interval": "600", "minimum-interval": "2"}`, "minimum-interval")
	rfc4861RefusedNaming(t, `{"enabled": "true", "maximum-interval": "600", "minimum-interval": "451"}`, "minimum-interval")
	mustParseRA(t, `{"enabled": "true", "maximum-interval": "600", "minimum-interval": "3"}`)
	mustParseRA(t, `{"enabled": "true", "maximum-interval": "600", "minimum-interval": "450"}`)
}

// RFC requirement: RFC4861-6.2.1-5 negative -- reachable-time 3600001
// milliseconds is refused naming reachable-time, and 3600000 is accepted.
// RFC requirement: RFC4861-6.2.1-5 positive -- reachable-time 3600000 milliseconds, the ceiling, is accepted.
func TestRFC4861AdvReachableTimeBound(t *testing.T) {
	rfc4861RefusedNaming(t, `{"enabled": "true", "reachable-time": "3600001"}`, "reachable-time")
	mustParseRA(t, `{"enabled": "true", "reachable-time": "3600000"}`)
}

// RFC requirement: RFC4861-6.2.1-6 negative -- with maximum-interval 600,
// router-lifetime 599 and 9001 are refused naming router-lifetime, while 0,
// 600 and 9000 are accepted.
// RFC requirement: RFC4861-6.2.1-6 positive -- with maximum-interval 600, router-lifetime 0, 600 and 9000 are accepted.
func TestRFC4861AdvDefaultLifetimeBounds(t *testing.T) {
	rfc4861RefusedNaming(t, `{"enabled": "true", "maximum-interval": "600", "router-lifetime": "599"}`, "router-lifetime")
	rfc4861RefusedNaming(t, `{"enabled": "true", "maximum-interval": "600", "router-lifetime": "9001"}`, "router-lifetime")
	for _, lifetime := range []string{"0", "600", "9000"} {
		mustParseRA(t, `{"enabled": "true", "maximum-interval": "600", "router-lifetime": "`+lifetime+`"}`)
	}
}

// RFC requirement: RFC4861-6.2.1-7 negative -- a prefix whose preferred-lifetime
// (3601) is larger than its valid-lifetime (3600) is refused naming
// preferred-lifetime, and equal lifetimes are accepted.
// RFC requirement: RFC4861-4.6.2-4 negative -- that refusal is what keeps a
// Prefix Information option's Preferred Lifetime from exceeding its Valid
// Lifetime: the configuration that would encode one never verifies.
// RFC requirement: RFC4861-6.2.1-7 positive -- a prefix whose preferred-lifetime equals its valid-lifetime (3600) is accepted.
// RFC requirement: RFC4861-4.6.2-4 positive -- a prefix whose preferred-lifetime equals its valid-lifetime is accepted, so the refusal is of the excess alone.
func TestRFC4861PreferredLifetimeNotAboveValid(t *testing.T) {
	rfc4861RefusedNaming(t, `{"enabled": "true", "prefix": {"2001:db8:1::/64": {"valid-lifetime": "3600", "preferred-lifetime": "3601"}}}`, "preferred-lifetime")
	mustParseRA(t, `{"enabled": "true", "prefix": {"2001:db8:1::/64": {"valid-lifetime": "3600", "preferred-lifetime": "3600"}}}`)
}

// RFC requirement: RFC4861-6.2.1-2 positive -- a router-advertisement container
// that does not set enabled parses with advertising off, and reconcileRA
// starts no sender for it.
func TestRFC4861AdvSendAdvertisementsIsFalseByDefault(t *testing.T) {
	if ra := mustParseRA(t, `{"prefix": {"2001:db8:1::/64": {}}}`); ra.Enabled {
		t.Fatal("advertising is on although the operator never set enabled")
	}
	rec := newRAFactoryRecorder(t)
	active := make(map[raUnitKey]raEntry)
	reconcileRA(raTestConfig(t, `{"prefix": {"2001:db8:1::/64": {}}}`), active, slog.New(slog.DiscardHandler))
	if len(rec.started) != 0 {
		t.Errorf("%d senders started for a unit that never enabled advertising", len(rec.started))
	}
}

// RFC requirement: RFC4861-6.2.2-1 negative -- of two interfaces that both carry
// a router-advertisement container, only eth0 enables it, and reconcileRA
// starts exactly one sender, on eth0.
// RFC requirement: RFC4861-6.2.2-1 positive -- the interface that enables it, eth0, gets its sender.
// RFC requirement: RFC7084-4.3-8 positive -- eth0, the interface whose router-advertisement the operator enables, gets a sender, which is how an interface becomes an advertising interface.
func TestRFC4861OnlyAnAdvertisingInterfaceSends(t *testing.T) {
	cfg := mustParseIfaceJSON(t, `{"interface": {"ethernet": {
		"eth0": {"unit": {"0": {"ipv6": {"router-advertisement": {"enabled": "true"}}}}},
		"eth1": {"unit": {"0": {"ipv6": {"router-advertisement": {"enabled": "false"}}}}}
	}}}`)
	rec := newRAFactoryRecorder(t)
	reconcileRA(cfg, make(map[raUnitKey]raEntry), slog.New(slog.DiscardHandler))
	if len(rec.started) != 1 {
		t.Fatalf("%d senders started, want 1", len(rec.started))
	}
	if got := rec.started[0].spec.Interface; got != "eth0" {
		t.Errorf("sender started on %q, want eth0: eth1 is not an advertising interface", got)
	}
}
