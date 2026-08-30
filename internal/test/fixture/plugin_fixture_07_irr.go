package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func filterIRRFail07(ctx context.Context, p *sdk.Plugin) error {
	until07(ctx, p, "show bgp irr", 20, 500*time.Millisecond, func(result commandResult07) bool {
		for _, row := range array07(object07(result.data)["entries"]) {
			entry := object07(row)
			if number07(entry["asn"]) == 65100 && entry["status"] != "pending" {
				return true
			}
		}
		return false
	})
	result := command07(ctx, p, "update bgp irr asn 65100")
	if result.status != statusError {
		return fmt.Errorf("update bgp irr asn 65100 status=%s, want error", result.status)
	}
	result = command07(ctx, p, "show bgp irr")
	if result.status != statusDone {
		return fmt.Errorf("show bgp irr status=%s, want done", result.status)
	}
	for _, row := range array07(object07(result.data)["entries"]) {
		entry := object07(row)
		if number07(entry["asn"]) == 65100 && entry["status"] == statusError {
			return nil
		}
	}
	return errors.New("show bgp irr does not show error status for ASN 65100")
}

func filterIRRUpdate07(ctx context.Context, p *sdk.Plugin) error {
	if result := done07(ctx, p, "update bgp irr all"); result.status != statusDone {
		return fmt.Errorf("update bgp irr all status=%s error=%w", result.status, result.err)
	}
	if result := command07(ctx, p, "show bgp irr"); result.status != statusDone {
		return fmt.Errorf("show bgp irr status=%s error=%w", result.status, result.err)
	}
	result := command07(ctx, p, "show bgp irr")
	if result.status != statusDone {
		return fmt.Errorf("show bgp irr via dispatch status=%s error=%w", result.status, result.err)
	}
	if _, ok := object07(result.data)["entries"]; !ok {
		return errors.New("show bgp irr missing entries after refresh")
	}
	return nil
}

func updateASSet07(ctx context.Context, p *sdk.Plugin, name string) (map[string]any, error) {
	result := done07(ctx, p, "update firewall irr as-set "+name)
	if result.status != statusDone {
		return nil, fmt.Errorf("update firewall irr as-set %s: status=%s error=%w", name, result.status, result.err)
	}
	return object07(result.data), nil
}

func firewallIRRColdCache07(ctx context.Context, p *sdk.Plugin) error {
	lan := until07(ctx, p, "show firewall ruleset lan", 50, 100*time.Millisecond, func(r commandResult07) bool { return r.status == statusDone })
	if lan.status != statusDone {
		return fmt.Errorf("lan was not programmed while wan waited: status=%s", lan.status)
	}
	if wan := done07(ctx, p, "show firewall ruleset wan"); wan.status == statusDone {
		return errors.New("wan was programmed with an unregistered IRR set")
	}
	data, err := updateASSet07(ctx, p, "AS-TEST")
	if err != nil || number07(data["ipv4-count"]) < 1 {
		return fmt.Errorf("no prefixes cached for AS-TEST: data=%s error=%w", text07(data), err)
	}
	wan := until07(ctx, p, "show firewall ruleset wan", 50, 100*time.Millisecond, func(r commandResult07) bool {
		return r.status == statusDone && strings.Contains(text07(r.data), "from-as-test")
	})
	dumped := text07(wan.data)
	if wan.status != statusDone || !strings.Contains(dumped, "from-as-test") || !strings.Contains(dumped, "from-as-test_v6") {
		return fmt.Errorf("wan ruleset incomplete after IRR update: %s", dumped)
	}
	return nil
}

func firewallIRRCommit07(ctx context.Context, p *sdk.Plugin) error {
	data, err := updateASSet07(ctx, p, "AS-TEST")
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "OK: cached AS-TEST: v4=%d v6=%d\n", number07(data["ipv4-count"]), number07(data["ipv6-count"]))
	if result := done07(ctx, p, "update firewall irr all"); result.status != statusDone {
		return fmt.Errorf("update firewall irr all: status=%s error=%w", result.status, result.err)
	}
	return nil
}

func firewallIRREmptyAnswer07(ctx context.Context, p *sdk.Plugin) error {
	data, err := updateASSet07(ctx, p, "AS-TEST")
	if err != nil {
		return err
	}
	v4, v6 := number07(data["ipv4-count"]), number07(data["ipv6-count"])
	if v4 < 1 {
		return fmt.Errorf("expected v4 prefixes on first refresh, got %d", v4)
	}
	if result := done07(ctx, p, "update firewall irr as-set AS-TEST"); result.status == statusDone {
		return errors.New("refresh that learned nothing reported success")
	}
	shown := done07(ctx, p, "show firewall irr")
	if shown.status != statusDone {
		return fmt.Errorf("show firewall irr: status=%s error=%w", shown.status, shown.err)
	}
	for _, row := range array07(object07(shown.data)["entries"]) {
		entry := object07(row)
		if entry["name"] != "AS-TEST" {
			continue
		}
		if number07(entry["ipv4-count"]) != v4 || number07(entry["ipv6-count"]) != v6 || entry["status"] != statusStale || entry["stale-since"] == nil {
			return fmt.Errorf("last-good entry not preserved as stale: %s", text07(entry))
		}
		return nil
	}
	return fmt.Errorf("AS-TEST missing from show output: %s", text07(shown.data))
}

func firewallIRRIfaceCommit07(ctx context.Context, p *sdk.Plugin) error {
	data, err := updateASSet07(ctx, p, "AS-TEST")
	if err == nil {
		fmt.Fprintf(os.Stderr, "OK: cached AS-TEST: v4=%d v6=%d\n", number07(data["ipv4-count"]), number07(data["ipv6-count"]))
	}
	return err
}

func firewallIRRIfaceNoBlackhole07(ctx context.Context, p *sdk.Plugin) error {
	data, err := updateASSet07(ctx, p, "AS-TEST")
	if err != nil || number07(data["ipv4-count"]) < 1 {
		return fmt.Errorf("initial update did not return v4 prefixes: data=%s error=%w", text07(data), err)
	}
	if result := done07(ctx, p, "update firewall irr as-set AS-TEST"); result.status == statusDone {
		return errors.New("refresh that learned nothing reported success")
	}
	result := done07(ctx, p, "show firewall ruleset irr_iface")
	if result.status != statusDone || !strings.Contains(text07(result.data), "iface_eth1_v4") {
		return fmt.Errorf("eth1 lost its accept rule: status=%s data=%s", result.status, text07(result.data))
	}
	return nil
}

func firewallIRRRefresh07(ctx context.Context, p *sdk.Plugin) error {
	data, err := updateASSet07(ctx, p, "AS-TEST")
	if err != nil || number07(data["ipv4-count"]) < 1 {
		return fmt.Errorf("initial cache missing v4 prefixes: data=%s error=%w", text07(data), err)
	}
	if result := done07(ctx, p, "show firewall irr"); result.status != statusDone {
		return fmt.Errorf("show after refresh: status=%s error=%w", result.status, result.err)
	}
	return nil
}

func firewallIRRShow07(ctx context.Context, p *sdk.Plugin) error {
	if _, err := updateASSet07(ctx, p, "AS-TEST"); err != nil {
		return err
	}
	shown := done07(ctx, p, "show firewall irr")
	if shown.status != statusDone || object07(shown.data)["server"] == nil {
		return fmt.Errorf("show firewall irr missing server: status=%s data=%s", shown.status, text07(shown.data))
	}
	prefixes := done07(ctx, p, "show firewall irr prefix AS-TEST")
	ipv4 := array07(object07(prefixes.data)["ipv4"])
	if prefixes.status != statusDone || len(ipv4) < 1 {
		return fmt.Errorf("show firewall irr prefix missing ipv4: status=%s data=%s", prefixes.status, text07(prefixes.data))
	}
	return nil
}

func firewallIRRTableTermCommit07(ctx context.Context, p *sdk.Plugin) error {
	data, err := updateASSet07(ctx, p, "AS-TEST")
	if err != nil || number07(data["ipv4-count"]) < 1 || number07(data["ipv6-count"]) < 1 {
		return fmt.Errorf("AS-TEST must cache both families: data=%s error=%w", text07(data), err)
	}
	data, err = updateASSet07(ctx, p, "AS-V4ONLY")
	if err != nil || number07(data["ipv4-count"]) < 1 || number07(data["ipv6-count"]) != 0 {
		return fmt.Errorf("AS-V4ONLY family counts invalid: data=%s error=%w", text07(data), err)
	}
	if err := os.WriteFile("observer.fetched", []byte("ok"), 0o600); err != nil {
		return err
	}
	if !Poll(ctx, 200, 100*time.Millisecond, func() bool { _, err := os.Stat("reload.done"); return err == nil }) {
		return errors.New("reload trigger never created reload.done marker")
	}
	result := until07(ctx, p, "show firewall ruleset wan", 100, 100*time.Millisecond, func(r commandResult07) bool {
		return r.status == statusDone && strings.Contains(text07(r.data), "from-v4only_v6")
	})
	dumped := text07(result.data)
	for _, term := range []string{"from-as-test", "from-as-test_v6", "from-v4only", "from-v4only_v6"} {
		if !strings.Contains(dumped, term) {
			return fmt.Errorf("term %s missing from kernel ruleset: %s", term, dumped)
		}
	}
	return nil
}

func firewallIRRTableTermUncached07(ctx context.Context, p *sdk.Plugin) error {
	if err := os.WriteFile("observer.ready", []byte("ok"), 0o600); err != nil {
		return err
	}
	if !Poll(ctx, 200, 100*time.Millisecond, func() bool { _, err := os.Stat("reload.done"); return err == nil }) {
		return errors.New("reload trigger never created reload.done marker")
	}
	if result := done07(ctx, p, "show firewall ruleset wan"); result.status == statusDone {
		return fmt.Errorf("refused config reached kernel: %s", text07(result.data))
	}
	return nil
}

func firewallIRRUpdate07(ctx context.Context, p *sdk.Plugin) error {
	data, err := updateASSet07(ctx, p, "AS-TEST")
	if err != nil || data["ipv4-count"] == nil || number07(data["ipv4-count"]) < 1 {
		return fmt.Errorf("update response missing positive ipv4-count: data=%s error=%w", text07(data), err)
	}
	return nil
}
