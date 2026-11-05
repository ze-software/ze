package l2tp

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
)

// TestMRUProtocolFloors exercises both config entry points after resolving
// effective IPv6CP defaults. The original-tree load also reaches the registered
// validator rather than relying only on subsystem extraction.
func TestMRUProtocolFloors(t *testing.T) {
	for _, mode := range []string{"default", "true", "false", "both-disabled"} {
		for _, mru := range []uint16{64, 67, 68, 1279, 1280, 1492, 1500, 1501} {
			t.Run(fmt.Sprintf("%s/%d", mode, mru), func(t *testing.T) {
				root := map[string]any{"mru": fmt.Sprint(mru)}
				tree := config.NewTree()
				l2tp := tree.GetOrCreateContainer("l2tp")
				l2tp.Set("mru", fmt.Sprint(mru))
				body := fmt.Sprintf("mru %d;", mru)
				if mode != "default" {
					ipv6 := mode
					if mode == "both-disabled" {
						ipv6 = "false"
					}
					ncp := map[string]any{"enable-ipv6cp": ipv6}
					ncpTree := l2tp.GetOrCreateContainer("ncp")
					ncpTree.Set("enable-ipv6cp", ipv6)
					body += " ncp { enable-ipv6cp " + ipv6 + ";"
					if mode == "both-disabled" {
						ncp["enable-ipcp"] = "false"
						ncpTree.Set("enable-ipcp", "false")
						body += " enable-ipcp false;"
					}
					body += " }"
					root["ncp"] = ncp
				}
				valid := mru >= 68 && mru <= 1500 &&
					((mode == "false" || mode == "both-disabled") || mru >= 1280)
				fromTree, treeErr := ExtractParameters(tree)
				fromProvider, providerErr := extractFromProvider(&fakeConfigProvider{
					trees: map[string]map[string]any{"l2tp": root},
				})
				_, loadErr := config.LoadConfig("l2tp { "+body+" }", "mru-floor.conf", nil)
				for name, err := range map[string]error{"tree": treeErr, "provider": providerErr, "load": loadErr} {
					if (err == nil) != valid {
						t.Errorf("%s valid=%v, error=%v", name, valid, err)
					}
					if err != nil && !strings.Contains(err.Error(), "mru") && !strings.Contains(err.Error(), "MRU") {
						t.Errorf("%s does not name the offending MRU: %v", name, err)
					}
				}
				if valid && (fromTree.MRU != mru || fromProvider.MRU != mru ||
					fromTree.EnableIPv6CP != fromProvider.EnableIPv6CP) {
					t.Errorf("effective settings differ: tree=%+v provider=%+v", fromTree, fromProvider)
				}
			})
		}
	}
}

func TestMRUFloorReloadIsolation(t *testing.T) {
	s := newStartedSubsystem(t, Parameters{Enabled: true, MRU: 1492, EnableIPCP: true, EnableIPv6CP: true})
	bad := &fakeConfigProvider{trees: map[string]map[string]any{
		"l2tp": {"mru": "1279"},
	}}
	if err := s.Reload(context.Background(), bad); err == nil {
		t.Fatal("default-enabled IPv6 accepted MRU 1279")
	}
	if s.params.MRU != 1492 || !s.params.EnableIPCP || !s.params.EnableIPv6CP {
		t.Fatalf("rejected reload changed live policy: %+v", s.params)
	}
	good := &fakeConfigProvider{trees: map[string]map[string]any{
		"l2tp": {"mru": "68", "ncp": map[string]any{"enable-ipv6cp": "false"}},
	}}
	if err := s.Reload(context.Background(), good); err != nil {
		t.Fatal(err)
	}
	if s.params.MRU != 68 || !s.params.EnableIPCP || s.params.EnableIPv6CP {
		t.Fatalf("valid IPv4-only reload not applied: %+v", s.params)
	}
}

func TestSubsystemStartRejectsMRUFloor(t *testing.T) {
	for _, tc := range []Parameters{
		{Enabled: true, MRU: 67, EnableIPCP: true},
		{Enabled: true, MRU: 1279, EnableIPv6CP: true},
	} {
		s := NewSubsystem(tc)
		if err := s.Start(context.Background(), nil, nil); err == nil {
			t.Fatalf("accepted incompatible direct Parameters: %+v", tc)
		}
		if s.started {
			t.Fatal("refused subsystem was marked started")
		}
	}
}
