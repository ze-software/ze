// Design: docs/architecture/l2tp/bng-5-pppoe.md -- subscriber auth policy.

package pppoe

import (
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// TestPPPoEAuthAllowDefaults proves method-dependent defaults and ordered-set
// preservation through the real config parser rather than a synthetic map.
func TestPPPoEAuthAllowDefaults(t *testing.T) {
	for _, tc := range []struct {
		method string
		list   string
		want   []ppp.AuthMethod
	}{
		{"chap-md5", "", []ppp.AuthMethod{ppp.AuthMethodCHAPMD5}},
		{"ms-chap-v2", "", []ppp.AuthMethod{ppp.AuthMethodMSCHAPv2}},
		{"pap", "", []ppp.AuthMethod{ppp.AuthMethodCHAPMD5, ppp.AuthMethodPAP}},
		{"none", "", nil},
		{"pap", "authentication-allow [pap ms-chap-v2 chap-md5 pap];", []ppp.AuthMethod{ppp.AuthMethodPAP, ppp.AuthMethodMSCHAPv2, ppp.AuthMethodCHAPMD5}},
	} {
		t.Run(tc.method+tc.list, func(t *testing.T) {
			p := extractFromConfigText(t, "pppoe { auth-method "+tc.method+"; allow-no-auth true; "+tc.list+" }")
			if !slices.Equal(p.AuthAllow, tc.want) {
				t.Fatalf("effective policy = %v, want %v", p.AuthAllow, tc.want)
			}
		})
	}
}

// TestPPPoEAuthAllowProviderShapes proves all supported provider encodings use
// the same name parser and malformed encodings cannot widen permissions.
func TestPPPoEAuthAllowProviderShapes(t *testing.T) {
	for _, value := range []any{"chap-md5", []string{"chap-md5"}, []any{"chap-md5"}, []string{}, []any{}} {
		p, err := ExtractParameters(map[string]any{"pppoe": map[string]any{"authentication-allow": value}})
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(p.AuthAllow, []ppp.AuthMethod{ppp.AuthMethodCHAPMD5}) {
			t.Fatalf("%#v resolved to %v", value, p.AuthAllow)
		}
	}
	for _, value := range []any{"", "unknown", []any{"chap-md5", false}} {
		if _, err := ExtractParameters(map[string]any{"pppoe": map[string]any{"authentication-allow": value}}); err == nil {
			t.Errorf("accepted %#v", value)
		}
	}
}

// TestPPPoEAuthAllowOriginalTreeValidation proves cross-leaf validation reaches
// the original tree boundary, including the implicit CHAP-MD5 method.
func TestPPPoEAuthAllowOriginalTreeValidation(t *testing.T) {
	for _, text := range []string{
		"pppoe { authentication-allow [pap]; }",
		"pppoe { auth-method none; allow-no-auth true; authentication-allow [chap-md5]; }",
	} {
		tree, err := config.ParseTreeForValidation(text)
		if err == nil {
			err = validateAuthPolicy(tree)
		}
		if err == nil {
			t.Fatalf("accepted %s", text)
		}
	}
}

// TestPPPoEAuthAllowReloadIsolation proves startup and reload share extraction
// and mapping, and replacing policy never mutates an older server generation.
func TestPPPoEAuthAllowReloadIsolation(t *testing.T) {
	f := newReloadFixture(t)
	before := extractFromConfigText(t, "pppoe { authentication-allow [chap-md5 pap]; interface eth0 {} }")
	old := f.sub.servers[reloadEth0Index]
	applyInterfaceConfig(old, &before.Interfaces[0], &before)
	before.AuthAllow[0] = ppp.AuthMethodMSCHAPv2
	if old.authAllow[0] != ppp.AuthMethodCHAPMD5 {
		t.Fatal("server aliases mutable parameter policy")
	}
	if err := f.reload(t, map[string]any{"enabled": "true", "auth-method": "ms-chap-v2", "authentication-allow": []any{"ms-chap-v2"}, "interface": map[string]any{"eth0": map[string]any{}}}); err != nil {
		t.Fatal(err)
	}
	next := f.sub.servers[reloadEth0Index]
	if !slices.Equal(next.authAllow, []ppp.AuthMethod{ppp.AuthMethodMSCHAPv2}) {
		t.Fatalf("reload policy = %v", next.authAllow)
	}
	if !slices.Equal(old.authAllow, []ppp.AuthMethod{ppp.AuthMethodCHAPMD5, ppp.AuthMethodPAP}) {
		t.Fatalf("old generation changed to %v", old.authAllow)
	}
	if err := f.reload(t, map[string]any{"authentication-allow": []any{"chap-md5", 42}}); err == nil {
		t.Fatal("reload accepted malformed explicit policy")
	}
	if f.sub.servers[reloadEth0Index] != next {
		t.Fatal("refused reload published a new generation")
	}
}
