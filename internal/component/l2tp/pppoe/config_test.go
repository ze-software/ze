// Related: config.go -- ExtractParameters, fed by Tree.ToMap

package pppoe

import (
	"errors"
	"testing"
	"time"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// extractFromConfigText drives ExtractParameters from its REAL producer: the
// config parser builds a Tree, ToMap renders it, and the hub hands that map
// over unchanged (cmd/ze/hub/main.go, cmd/ze/hub/register_l2tp.go).
//
// Building the map by hand is what let the interface-shape bug live. Two tests
// wrote `"interface": []any{...}`, a shape ToMap cannot emit for a keyed YANG
// list, and both passed while every real config produced zero interfaces.
func extractFromConfigText(t *testing.T, text string) Parameters {
	t.Helper()
	tree, err := zeconfig.ParseTreeForValidation(text)
	if err != nil {
		t.Fatalf("parse config: %v", err)
	}
	p, err := ExtractParameters(tree.ToMap())
	if err != nil {
		t.Fatalf("extract parameters: %v", err)
	}
	return p
}

// extractErrFromConfigText is extractFromConfigText's negative twin: it returns
// the error instead of failing the test, for the configurations the AC must
// refuse to start on.
func extractErrFromConfigText(t *testing.T, text string) error {
	t.Helper()
	tree, err := zeconfig.ParseTreeForValidation(text)
	if err != nil {
		t.Fatalf("parse config: %v", err)
	}
	_, extractErr := ExtractParameters(tree.ToMap())
	return extractErr
}

// VALIDATES: a configured access interface reaches Parameters.Interfaces, which
// is the condition registerBNGSubsystems requires before it registers the PPPoE
// subsystem at all.
// PREVENTS: the subsystem never starting. With Interfaces empty the hub skips
// registration, no discovery socket is opened, and a configured AC answers no
// PADI. The daemon logs nothing about PPPoE, so the symptom is silence.
func TestExtractParametersFindsInterfaceFromRealTree(t *testing.T) {
	p := extractFromConfigText(t, `
pppoe {
    enabled true
    ac-name my-ac
    interface eth0 {
    }
}
`)
	if !p.Enabled {
		t.Error("enabled true must reach Parameters.Enabled")
	}
	if p.ACName != "my-ac" {
		t.Errorf("ac-name = %q, want my-ac", p.ACName)
	}
	if len(p.Interfaces) != 1 {
		t.Fatalf("got %d interfaces, want 1 -- the hub registers the subsystem only when this is non-empty", len(p.Interfaces))
	}
	if p.Interfaces[0].Name != "eth0" {
		t.Errorf("interface name = %q, want eth0 (the list key IS the name)", p.Interfaces[0].Name)
	}
	if p.Interfaces[0].MaxSessions != DefaultMaxSessions {
		t.Errorf("interface max-sessions = %d, want the global default %d", p.Interfaces[0].MaxSessions, DefaultMaxSessions)
	}
}

// VALIDATES: every scalar leaf survives the tree, which delivers each one as a
// string, and a per-interface override beats the global value.
// PREVENTS: the earlier .(bool)/.(float64) assertions, under which `enabled`
// stayed false and every numeric leaf silently fell back to its default.
func TestExtractParametersReadsScalarLeavesAndOverride(t *testing.T) {
	p := extractFromConfigText(t, `
pppoe {
    enabled true
    cookie-timeout 10
    max-sessions 1000
    padi-rate-limit 50
    interface eth0 {
        max-sessions 200
    }
    interface eth1 {
    }
}
`)
	if p.CookieTimeout != 10*time.Second {
		t.Errorf("cookie-timeout = %v, want 10s", p.CookieTimeout)
	}
	if p.MaxSessions != 1000 {
		t.Errorf("max-sessions = %d, want 1000", p.MaxSessions)
	}
	if p.PADIRateLimit != 50 {
		t.Errorf("padi-rate-limit = %d, want 50", p.PADIRateLimit)
	}
	if len(p.Interfaces) != 2 {
		t.Fatalf("got %d interfaces, want 2", len(p.Interfaces))
	}
	// Sorted by name, so the order is the config's and not the map's.
	if p.Interfaces[0].Name != "eth0" || p.Interfaces[1].Name != "eth1" {
		t.Fatalf("interface order = %q,%q, want eth0,eth1", p.Interfaces[0].Name, p.Interfaces[1].Name)
	}
	if p.Interfaces[0].MaxSessions != 200 {
		t.Errorf("eth0 max-sessions = %d, want its own 200", p.Interfaces[0].MaxSessions)
	}
	if p.Interfaces[1].MaxSessions != 1000 {
		t.Errorf("eth1 max-sessions = %d, want the global 1000", p.Interfaces[1].MaxSessions)
	}
}

// VALIDATES: a leaf-list reaches Parameters whether it has one member or
// several, because ToMap collapses a single member to a bare string.
// PREVENTS: Service-Name filtering being configured and never applied.
// MatchServiceName (discovery.go) treats an empty list as "accept any", so a
// dropped one-member filter turns a restrictive AC into an open one, which no
// error and no log distinguishes from a working filter.
func TestExtractParametersReadsServiceNameLeafList(t *testing.T) {
	one := extractFromConfigText(t, `
pppoe {
    enabled true
    service-name only-this
    interface eth0 {
        service-name iface-only
    }
}
`)
	if len(one.ServiceNames) != 1 || one.ServiceNames[0] != "only-this" {
		t.Errorf("global service-name = %v, want [only-this] (a single member arrives as a bare string)", one.ServiceNames)
	}
	if len(one.Interfaces) != 1 {
		t.Fatalf("got %d interfaces, want 1", len(one.Interfaces))
	}
	if len(one.Interfaces[0].ServiceNames) != 1 || one.Interfaces[0].ServiceNames[0] != "iface-only" {
		t.Errorf("interface service-name = %v, want [iface-only]", one.Interfaces[0].ServiceNames)
	}

	many := extractFromConfigText(t, `
pppoe {
    enabled true
    service-name first
    service-name second
    interface eth0 {
    }
}
`)
	if len(many.ServiceNames) != 2 {
		t.Fatalf("global service-name = %v, want two members", many.ServiceNames)
	}
}

// VALIDATES: a pppoe block with no interface list yields no interfaces, and an
// absent block yields the documented defaults.
// PREVENTS: a nil map or a missing key panicking the hub during startup.
func TestExtractParametersWithoutInterfaces(t *testing.T) {
	bare := extractFromConfigText(t, "pppoe {\n    enabled true\n}\n")
	if len(bare.Interfaces) != 0 {
		t.Errorf("interfaces = %v, want none", bare.Interfaces)
	}

	absent, err := ExtractParameters(map[string]any{})
	if err != nil {
		t.Fatalf("absent pppoe block must not error: %v", err)
	}
	if absent.Enabled {
		t.Error("no pppoe block must not enable the subsystem")
	}
	if absent.ACName != DefaultACName || absent.MaxSessions != DefaultMaxSessions {
		t.Errorf("defaults = %q/%d, want %q/%d", absent.ACName, absent.MaxSessions, DefaultACName, DefaultMaxSessions)
	}
}

// VALIDATES: `auth-method` reaches Parameters.AuthMethod, and the default when
// the leaf is absent is CHAP-MD5 rather than "no authentication at all".
// PREVENTS: an access concentrator that never advertises an Auth-Protocol. Ze
// then runs every subscriber through the accounting-only no-auth phase, where
// EventAuthRequest carries an empty Username, so an auth handler holding the
// operator's credentials answers "unknown user" and no session ever comes up.
// The AC looked healthy through PADS and failed on the first frame that
// mattered.
func TestExtractParametersAuthMethod(t *testing.T) {
	def := extractFromConfigText(t, "pppoe {\n    enabled true\n    interface eth0 {\n    }\n}\n")
	if def.AuthMethod != ppp.AuthMethodCHAPMD5 {
		t.Errorf("default auth-method = %v, want chap-md5", def.AuthMethod)
	}
	if def.AllowNoAuth {
		t.Error("allow-no-auth must default to false, so a client that refuses the method is disconnected")
	}

	for _, tc := range []struct {
		spelling string
		want     ppp.AuthMethod
	}{
		{"pap", ppp.AuthMethodPAP},
		{"chap-md5", ppp.AuthMethodCHAPMD5},
		{"ms-chap-v2", ppp.AuthMethodMSCHAPv2},
	} {
		p := extractFromConfigText(t, "pppoe {\n    enabled true\n    auth-method "+tc.spelling+"\n    interface eth0 {\n    }\n}\n")
		if p.AuthMethod != tc.want {
			t.Errorf("auth-method %s = %v, want %v", tc.spelling, p.AuthMethod, tc.want)
		}
	}
}

// VALIDATES: `auth-method none` is refused unless `allow-no-auth true` says so,
// and accepted when it does.
// PREVENTS: a one-word edit turning a BNG into an open access concentrator. The
// hub propagates the error and refuses to start (cmd/ze/hub/register_l2tp.go),
// which is louder than admitting every subscriber unauthenticated.
func TestExtractParametersAuthMethodNoneNeedsAllowNoAuth(t *testing.T) {
	err := extractErrFromConfigText(t, "pppoe {\n    enabled true\n    auth-method none\n    interface eth0 {\n    }\n}\n")
	if !errors.Is(err, ErrAuthMethodNoneRequiresAllow) {
		t.Fatalf("auth-method none alone: err = %v, want ErrAuthMethodNoneRequiresAllow", err)
	}

	ok := extractFromConfigText(t, "pppoe {\n    enabled true\n    auth-method none\n    allow-no-auth true\n    interface eth0 {\n    }\n}\n")
	if ok.AuthMethod != ppp.AuthMethodNone || !ok.AllowNoAuth {
		t.Errorf("auth-method none + allow-no-auth true = %v/%v, want none/true", ok.AuthMethod, ok.AllowNoAuth)
	}
}
