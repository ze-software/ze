package fixture

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// filterIRRDecision07 observes test/plugin/filter-irr.ci. Its goal is to hold
// the daemon up until the IRR import filter has decided the one UPDATE the
// peer sends, and to check that decision kept 10.0.0.0/24 and dropped
// 192.168.0.0/24. Method: the reactor delivers a received UPDATE to an attached
// process only after the import chain accepted or modified it
// (reactor_notify.go, "Route rejected by filter; don't cache or dispatch"), so
// the update event IS the decision. The peer's updates-received counter is
// not: it counts the UPDATE before the chain runs, and an observer that
// returned on it stopped ze while the filter was still waiting for its first
// IRR resolution, which then failed with "bridge closed".
func filterIRRDecision07(ctx context.Context, _ []string) error {
	const name = "plugin/filter-irr"
	updates := make(chan string, 4)
	setup := func(p *sdk.Plugin) error {
		p.OnEvent(func(event string) error {
			var decoded map[string]any
			if json.Unmarshal([]byte(event), &decoded) != nil {
				return nil //nolint:nilerr // a malformed event is skipped, and failing the handler would end the session
			}
			bgp, _ := decoded["bgp"].(map[string]any)
			message, _ := bgp["message"].(map[string]any)
			if message["direction"] != directionReceived {
				return nil
			}
			update, _ := bgp["update"].(map[string]any)
			if _, hasNLRI := update["nlri"]; !hasNLRI {
				return nil
			}
			select {
			case updates <- text07(update):
			default:
			}
			return nil
		})
		p.SetStartupSubscriptions([]string{eventUpdate}, nil, "parsed")
		return nil
	}
	scenario := func(ctx context.Context, _ *sdk.Plugin) error {
		wait := WaitBudget(50, 10*time.Second)
		var update string
		select {
		case update = <-updates:
		case <-time.After(wait):
			return fmt.Errorf("no filtered UPDATE reached the observer within %s: the IRR import filter rejected it or never decided", wait)
		case <-ctx.Done():
			return ctx.Err()
		}
		if !strings.Contains(update, "10.0.0.0/24") {
			return fmt.Errorf("the IRR filter dropped the in-list prefix 10.0.0.0/24: update=%s", update)
		}
		if strings.Contains(update, "192.168.0.0/24") {
			return fmt.Errorf("the IRR filter kept the out-of-list prefix 192.168.0.0/24: update=%s", update)
		}
		return nil
	}
	return observeConfigured(ctx, name, &sdk.Registration{}, setup, scenario)
}

// filterIRRReadmit07 observes test/plugin/filter-irr-readmit.ci. Its goal is
// to prove that a route rejected because its ASN's IRR list could not be
// resolved is accepted once the list arrives, with no action by the peer's
// operator. Method: the mock IRR fails the first query, so the first
// resolution fails and the peer's UPDATE is rejected as no-prefix-list. Once
// the ASN shows the error and the peer's following End-of-RIB is counted, the
// initial filter verdict and held-route tracking have completed. The observer
// refreshes the ASN the way an operator does; the filter must then send a
// ROUTE-REFRESH (the peer's script checks it before sending the route again).
// Shutdown waits for that route here, delivered only after the import chain
// accepted it (reactor_notify.go).
func filterIRRReadmit07(ctx context.Context, _ []string) error {
	const name = "plugin/filter-irr-readmit"
	updates := make(chan string, 4)
	setup := func(p *sdk.Plugin) error {
		p.OnEvent(func(event string) error {
			var decoded map[string]any
			if json.Unmarshal([]byte(event), &decoded) != nil {
				return nil //nolint:nilerr // a malformed event is skipped, and failing the handler would end the session
			}
			bgp, _ := decoded["bgp"].(map[string]any)
			message, _ := bgp["message"].(map[string]any)
			if message["direction"] != directionReceived {
				return nil
			}
			update, _ := bgp["update"].(map[string]any)
			if _, hasNLRI := update["nlri"]; !hasNLRI {
				return nil
			}
			select {
			case updates <- text07(update):
			default:
			}
			return nil
		})
		p.SetStartupSubscriptions([]string{eventUpdate}, nil, "parsed")
		return nil
	}
	scenario := func(ctx context.Context, p *sdk.Plugin) error {
		var failed commandResult07
		if !Poll(ctx, 40, 250*time.Millisecond, func() bool {
			failed = command07(ctx, p, "show bgp irr")
			if failed.status != statusDone {
				return false
			}
			for _, row := range array07(object07(failed.data)["entries"]) {
				entry := object07(row)
				if number07(entry["asn"]) == 65001 && entry["status"] == statusError {
					return true
				}
			}
			return false
		}) {
			return fmt.Errorf("ASN 65001 never reported its initial IRR failure: status=%s data=%s error=%w", failed.status, text07(failed.data), failed.err)
		}
		// updates-received advances before the import filter runs. The next
		// frame's EOR counter advances only after that verdict and hold finish.
		var received commandResult07
		if !Poll(ctx, 40, 250*time.Millisecond, func() bool {
			received = command07(ctx, p, "show bgp peer 127.0.0.1 detail")
			if received.status != statusDone {
				return false
			}
			peer := object07(object07(object07(received.data)["peers"])["127.0.0.1"])
			return number07(peer["eor-received"]) >= 1
		}) {
			return fmt.Errorf("the End-of-RIB following the initial rejected UPDATE never arrived: status=%s data=%s error=%w", received.status, text07(received.data), received.err)
		}
		select {
		case update := <-updates:
			return fmt.Errorf("the IRR filter accepted a route before its list resolved: update=%s", update)
		default:
		}
		if result := done07(ctx, p, "update bgp irr all"); result.status != statusDone {
			return fmt.Errorf("update bgp irr all status=%s error=%w", result.status, result.err)
		}
		wait := WaitBudget(50, 10*time.Second)
		select {
		case update := <-updates:
			if !strings.Contains(update, "10.0.0.0/24") {
				return fmt.Errorf("the route sent again is not 10.0.0.0/24: update=%s", update)
			}
			return nil
		case <-time.After(wait):
			return fmt.Errorf("the route rejected while the IRR list was missing was not accepted within %s of the list arriving", wait)
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return observeConfigured(ctx, name, &sdk.Registration{}, setup, scenario)
}

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
	// The refresh must fail on the IRR lookup itself (updateASN, command.go).
	// A status alone was also satisfied by the usage error the plugin answered
	// when the forwarder dropped the ASN argument.
	result := command07(ctx, p, "update bgp irr asn 65100")
	if result.status != statusError {
		return fmt.Errorf("update bgp irr asn 65100 status=%s, want error", result.status)
	}
	if result.err == nil {
		return errors.New("update bgp irr asn 65100 returned no error, want the failed refresh of ASN 65100")
	}
	if !strings.Contains(result.err.Error(), "ASN 65100 refresh failed") {
		return fmt.Errorf("update bgp irr asn 65100 error=%w, want the failed refresh of ASN 65100", result.err)
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
	if err := waitForPeerUpdate07()(ctx, p); err != nil {
		return err
	}
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
	return quiesce07(ctx, p)
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
	// The .ci asserts this exact line. Returning nil silently made the test
	// unpassable: every check above can succeed and the run still fails on a
	// pattern nobody writes.
	fmt.Fprintln(os.Stderr, "lan stayed programmed, and the update brought wan back")
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
	// The .ci asserts this exact line; see firewallIRRColdCache07 for why a
	// silent success is a test that cannot pass.
	fmt.Fprintln(os.Stderr, "both table terms and their IPv6 twins are in the kernel ruleset")
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
