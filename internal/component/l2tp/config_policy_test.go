package l2tp

import (
	"context"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/l2tp/ppp"
)

// TestProviderPPPPolicyBoundaries rejects malformed security-policy input at
// the actual reload parser rather than silently replacing it with defaults.
func TestProviderPPPPolicyBoundaries(t *testing.T) {
	for _, value := range []any{42, nil, []any{"chap-md5", 42}, []string{"none"}, []string{"pap"}} {
		cfg := &fakeConfigProvider{trees: map[string]map[string]any{"l2tp": {
			"authentication": map[string]any{"allow": value},
		}}}
		if _, err := extractFromProvider(cfg); err == nil {
			t.Errorf("accepted malformed allow %#v", value)
		}
	}
	for _, value := range []any{42, "63", "1501", "-1", "garbage"} {
		cfg := &fakeConfigProvider{trees: map[string]map[string]any{"l2tp": {"mru": value}}}
		if _, err := extractFromProvider(cfg); err == nil {
			t.Errorf("accepted malformed mru %#v", value)
		}
	}
	for _, value := range []string{"64", "1492", "1500"} {
		cfg := &fakeConfigProvider{trees: map[string]map[string]any{"l2tp": {"mru": value}}}
		params, err := extractFromProvider(cfg)
		if err != nil {
			t.Fatal(err)
		}
		if params.MRU == 0 || len(params.AuthAllow) != 1 || params.AuthAllow[0] != ppp.AuthMethodCHAPMD5 {
			t.Fatalf("invalid effective policy: %+v", params)
		}
	}
}

// TestReloadPPPPolicyRefusalKeepsEffectiveConfig proves failed explicit input
// cannot replace the live policy, and successful reload is visible to show config.
func TestReloadPPPPolicyRefusalKeepsEffectiveConfig(t *testing.T) {
	s := newStartedSubsystem(t, Parameters{
		Enabled: true, MRU: 1492, AuthMethod: ppp.AuthMethodCHAPMD5,
		AuthAllow: []ppp.AuthMethod{ppp.AuthMethodCHAPMD5},
	})
	bad := &fakeConfigProvider{trees: map[string]map[string]any{"l2tp": {
		"mru": "64", "authentication": map[string]any{"allow": []any{"chap-md5", 42}},
	}}}
	if err := s.Reload(context.Background(), bad); err == nil {
		t.Fatal("malformed explicit policy accepted")
	}
	before := s.EffectiveConfig()
	if before.MRU != 1492 || !slices.Equal(before.AuthAllow, []string{"chap-md5"}) {
		t.Fatalf("refused reload changed policy: %+v", before)
	}
	good := &fakeConfigProvider{trees: map[string]map[string]any{"l2tp": {
		"mru": "64", "authentication": map[string]any{"allow": []string{"chap-md5", "pap"}},
	}}}
	if err := s.Reload(context.Background(), good); err != nil {
		t.Fatal(err)
	}
	after := s.EffectiveConfig()
	if after.MRU != 64 || !slices.Equal(after.AuthAllow, []string{"chap-md5", "pap"}) {
		t.Fatalf("effective config omitted applied policy: %+v", after)
	}
}
