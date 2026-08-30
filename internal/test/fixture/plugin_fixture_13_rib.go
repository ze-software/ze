package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("plugin/rib-advertised-user-path", ribAdvertisedUserPath13)
	Register("plugin/rib-best-selection", ribBestSelection13)
	Register("plugin/rib-clear-out-family", ribClearOutFamily13)
	Register("plugin/rib-filter-show", ribFilterShow13)
	Register("plugin/rib-forward-handle-observed", ribForwardObserved13)
	Register("plugin/rib-graph-best", ribGraphBest13)
	Register("plugin/rib-graph-filtered", ribGraphFiltered13)
	Register("plugin/rib-graph", ribGraph13)
	Register("plugin/rib-inject-rfc5549", ribInjectRFC5549)
	Register("plugin/rib-pipe-filter", ribPipeFilter13)
}

func ribReady13(ctx context.Context, plugin *sdk.Plugin) error {
	result := pollCommand13(ctx, plugin, "show bgp rib status", 20, 250*time.Millisecond, done13)
	return requireStatus13("rib plugin readiness", result, "done")
}

func countFrom13(result commandResult13) (int, bool) {
	value, ok := result.object()["count"]
	if !ok {
		return 0, false
	}
	return number13(value), true
}

func ribAdvertisedUserPath13(ctx context.Context, args []string) error {
	return observe13(ctx, "rib-advertised-user-path", func(ctx context.Context, plugin *sdk.Plugin) error {
		update := command13(ctx, plugin, "peer * update text origin igp local-preference 100 nhop 101.1.101.1 nlri ipv4/unicast add 1.1.0.0/24")
		if err := requireStatus13("user update", update, "done"); err != nil {
			return err
		}
		if err := requireStatus13("wait for update acknowledgement", command13(ctx, plugin, "request quiesce"), "done"); err != nil {
			return err
		}
		result := pollCommand13(ctx, plugin, "show bgp rib advertised prefix 1.1.0.0/24 count", 20, 250*time.Millisecond, func(result commandResult13) bool {
			count, ok := countFrom13(result)
			return done13(result) && ok && count == 1
		})
		if err := requireStatus13("show advertised count", result, "done"); err != nil {
			return err
		}
		if count, ok := countFrom13(result); !ok || count != 1 {
			return fmt.Errorf("advertised count did not reach 1: %s", result.raw)
		}
		result = command13(ctx, plugin, "show bgp rib received prefix 1.1.0.0/24 count")
		if err := requireStatus13("show received count", result, "done"); err != nil {
			return err
		}
		if count, ok := countFrom13(result); !ok || count != 0 {
			return fmt.Errorf("expected received count 0, got %s", result.raw)
		}
		fmt.Fprintln(os.Stderr, "OK: show bgp rib advertised selects Adj-RIB-Out from user update")
		return waitEORSent13(ctx, plugin, "*")
	})
}

type assertions13 struct{ failures []error }

func (a *assertions13) status(label string, result commandResult13) {
	if err := requireStatus13(label, result, "done"); err != nil {
		a.failures = append(a.failures, err)
		fmt.Fprintln(os.Stderr, "FAIL", err)
		return
	}
	fmt.Fprintf(os.Stderr, "OK %s: status=%s\n", label, result.status)
}

func (a *assertions13) contains(label string, result commandResult13, substring string) {
	if !strings.Contains(result.text(), substring) {
		err := fmt.Errorf("%s: output does not contain %q: %.200s", label, substring, result.raw)
		a.failures = append(a.failures, err)
		fmt.Fprintln(os.Stderr, "FAIL", err)
		return
	}
	fmt.Fprintf(os.Stderr, "OK %s: contains %q\n", label, substring)
}

func (a *assertions13) notContains(label string, result commandResult13, substring string) {
	if strings.Contains(result.text(), substring) {
		err := fmt.Errorf("%s: output contains forbidden %q", label, substring)
		a.failures = append(a.failures, err)
		fmt.Fprintln(os.Stderr, "FAIL", err)
		return
	}
	fmt.Fprintf(os.Stderr, "OK %s: does not contain %q\n", label, substring)
}

func (a *assertions13) finish(label string) error {
	if len(a.failures) == 0 {
		return nil
	}
	return fmt.Errorf("%d %s assertions failed: %w", len(a.failures), label, errors.Join(a.failures...))
}

func bestPathFor13(result commandResult13, prefix string) map[string]any {
	rows, _ := result.object()["best-path"].([]any)
	for _, value := range rows {
		row, _ := value.(map[string]any)
		if row["prefix"] == prefix {
			return row
		}
	}
	return nil
}

func ribBestSelection13(ctx context.Context, args []string) error {
	return observe13(ctx, "best-selection-test", func(ctx context.Context, plugin *sdk.Plugin) error {
		if err := ribReady13(ctx, plugin); err != nil {
			return err
		}
		a := &assertions13{}
		injections := []struct{ label, command string }{
			{"inject-lp100", "request bgp rib inject 10.0.0.1 ipv4/unicast 10.10.1.0/24 localpref 100"},
			{"inject-lp200", "request bgp rib inject 10.0.0.2 ipv4/unicast 10.10.1.0/24 localpref 200"},
		}
		for _, injection := range injections {
			a.status(injection.label, command13(ctx, plugin, injection.command))
		}
		best := command13(ctx, plugin, "show bgp rib best")
		a.status("best-localpref", best)
		if row := bestPathFor13(best, "10.10.1.0/24"); row == nil || row["best-peer"] != addrPeerTwo {
			a.failures = append(a.failures, fmt.Errorf("localpref winner=%v want 10.0.0.2", row))
		}
		for _, injection := range []struct{ label, command string }{
			{"inject-short-path", "request bgp rib inject 10.0.0.3 ipv4/unicast 10.20.1.0/24 aspath 64501"},
			{"inject-long-path", "request bgp rib inject 10.0.0.4 ipv4/unicast 10.20.1.0/24 aspath 64501,64502,64503"},
		} {
			a.status(injection.label, command13(ctx, plugin, injection.command))
		}
		best = command13(ctx, plugin, "show bgp rib best")
		a.status("best-aspath", best)
		if row := bestPathFor13(best, "10.20.1.0/24"); row == nil || row["best-peer"] != "10.0.0.3" {
			a.failures = append(a.failures, fmt.Errorf("aspath winner=%v want 10.0.0.3", row))
		}
		for _, injection := range []struct{ label, command string }{
			{"inject-peer6", "request bgp rib inject 10.0.0.6 ipv4/unicast 10.30.1.0/24 origin igp"},
			{"inject-peer5", "request bgp rib inject 10.0.0.5 ipv4/unicast 10.30.1.0/24 origin igp"},
		} {
			a.status(injection.label, command13(ctx, plugin, injection.command))
		}
		best = command13(ctx, plugin, "show bgp rib best")
		a.status("best-tiebreak", best)
		if row := bestPathFor13(best, "10.30.1.0/24"); row == nil || row["best-peer"] != "10.0.0.5" {
			a.failures = append(a.failures, fmt.Errorf("tiebreak winner=%v want 10.0.0.5", row))
		}
		count := command13(ctx, plugin, "show bgp rib best count")
		a.status("best-count", count)
		if value, ok := countFrom13(count); !ok || value != 3 {
			a.failures = append(a.failures, fmt.Errorf("best count=%d valid=%v want 3", value, ok))
		}
		return a.finish("best-path")
	})
}

func ribClearOutFamily13(ctx context.Context, args []string) error {
	return observe13(ctx, "rib-clear-out-family-test", func(ctx context.Context, plugin *sdk.Plugin) error {
		if err := requireStatus13("clear out family", command13(ctx, plugin, "clear bgp rib out * ipv4/unicast"), "done"); err != nil {
			return err
		}
		if err := requireStatus13("clear out missing selector", command13(ctx, plugin, "clear bgp rib out"), "error"); err != nil {
			return err
		}
		return waitEORSent13(ctx, plugin, "peer1")
	})
}

func ribFilterShow13(ctx context.Context, args []string) error {
	return observe13(ctx, "filter-test", func(ctx context.Context, plugin *sdk.Plugin) error {
		pollCommand13(ctx, plugin, "show bgp rib count", 20, 250*time.Millisecond, done13)
		checks := []struct {
			label, command string
			errorData      bool
		}{
			{fieldCommunity, "show bgp rib received community 65000:100", false},
			{fieldPath, "show bgp rib received path 64501", false},
			{pipeCount, "show bgp rib count", false},
			{"family+community", "show bgp rib received family ipv4/unicast community 65000:100", false},
			{"unknown-keyword", "show bgp rib bogus", true},
		}
		for _, check := range checks {
			result := command13(ctx, plugin, check.command)
			if err := requireStatus13(check.label, result, "done"); err != nil {
				return err
			}
			if check.errorData && !strings.Contains(result.text(), `"error"`) {
				return fmt.Errorf("%s: expected error JSON, got %.200s", check.label, result.raw)
			}
		}
		return waitEORSent13(ctx, plugin, "peer1")
	})
}

func ribForwardObserved13(ctx context.Context, args []string) error {
	return observe13(ctx, "shutdown-after-update", func(ctx context.Context, plugin *sdk.Plugin) error {
		if err := waitEORSent13(ctx, plugin, "peer1"); err != nil {
			return err
		}
		result := pollCommand13(ctx, plugin, "show bgp rib best", 40, 100*time.Millisecond, func(result commandResult13) bool { return strings.Contains(result.text(), "192.168.1.0/24") })
		if !strings.Contains(result.text(), "192.168.1.0/24") {
			return errors.New("192.168.1.0/24 never appeared in bgp rib show best")
		}
		return waitEORSent13(ctx, plugin, "peer1")
	})
}

func ribGraphBest13(ctx context.Context, args []string) error {
	return observe13(ctx, "graph-best-test", func(ctx context.Context, plugin *sdk.Plugin) error {
		if err := ribReady13(ctx, plugin); err != nil {
			return err
		}
		a := &assertions13{}
		a.status("inject", command13(ctx, plugin, "request bgp rib inject 10.0.0.1 ipv4/unicast 10.10.1.0/24 aspath 64501,64502"))
		graph := command13(ctx, plugin, "show bgp rib best graph")
		a.status("best-graph", graph)
		for _, want := range []string{asnLabel64501, asnLabel64502, "┌"} {
			a.contains("best-graph", graph, want)
		}
		return a.finish("best-graph")
	})
}

func ribGraphFiltered13(ctx context.Context, args []string) error {
	return observe13(ctx, "graph-filtered-test", func(ctx context.Context, plugin *sdk.Plugin) error {
		if err := ribReady13(ctx, plugin); err != nil {
			return err
		}
		a := &assertions13{}
		a.status("inject-1", command13(ctx, plugin, "request bgp rib inject 10.0.0.1 ipv4/unicast 10.10.1.0/24 aspath 64501,64502"))
		a.status("inject-2", command13(ctx, plugin, "request bgp rib inject 10.0.0.2 ipv4/unicast 10.20.1.0/24 aspath 64601,64602"))
		graph := command13(ctx, plugin, "show bgp rib received prefix 10.10 graph")
		a.status("filtered-graph", graph)
		for _, want := range []string{asnLabel64501, asnLabel64502} {
			a.contains("filtered-graph", graph, want)
		}
		for _, absent := range []string{"AS64601", "AS64602"} {
			a.notContains("filtered-graph", graph, absent)
		}
		return a.finish("filtered-graph")
	})
}

func ribGraph13(ctx context.Context, args []string) error {
	return observe13(ctx, "graph-test", func(ctx context.Context, plugin *sdk.Plugin) error {
		if err := ribReady13(ctx, plugin); err != nil {
			return err
		}
		a := &assertions13{}
		a.status("inject-1", command13(ctx, plugin, "request bgp rib inject 10.0.0.1 ipv4/unicast 10.10.1.0/24 aspath 64501,64502,64503"))
		a.status("inject-2", command13(ctx, plugin, "request bgp rib inject 10.0.0.2 ipv4/unicast 10.10.2.0/24 aspath 64504,64502,64503"))
		graph := command13(ctx, plugin, "show bgp rib received graph")
		a.status("graph", graph)
		for _, want := range []string{asnLabel64501, asnLabel64502, "AS64503", "AS64504", "┌"} {
			a.contains("graph", graph, want)
		}
		a.status("empty-graph", command13(ctx, plugin, "show bgp rib received prefix 99.99.99.0/24 graph"))
		return a.finish("graph")
	})
}

func ribInjectRFC5549(ctx context.Context, args []string) error {
	return observe13(ctx, "rib-inject-rfc5549", func(ctx context.Context, plugin *sdk.Plugin) error {
		inject := command13(ctx, plugin, "request bgp rib inject 10.0.0.99 ipv4/unicast 192.168.1.0/24 origin igp nexthop 2001:db8::1")
		if err := requireStatus13("rib inject", inject, "done"); err != nil {
			return err
		}
		best := command13(ctx, plugin, "show bgp rib best")
		if err := requireStatus13("rib best show", best, "done"); err != nil {
			return err
		}
		if !strings.Contains(best.text(), "192.168.1.0/24") || !strings.Contains(best.text(), "2001:db8::1") {
			return fmt.Errorf("extended next-hop missing from best show: %.300s", best.raw)
		}
		fmt.Fprintln(os.Stderr, "OK: RFC 5549 extended next-hop stored and recovered")
		if err := waitEORSent13(ctx, plugin, "peer1"); err != nil {
			return err
		}
		var state string
		result := pollCommand13(ctx, plugin, "show bgp peer peer1 detail", 60, 250*time.Millisecond, func(result commandResult13) bool {
			value, ok := peerField13(result, "state")
			state, _ = value.(string)
			return ok && state != "" && state != stateEstablished
		})
		value, _ := peerField13(result, "state")
		state, _ = value.(string)
		if state == "" || state == stateEstablished {
			return fmt.Errorf("peer1 never consumed End-of-RIB (state=%q)", state)
		}
		return nil
	})
}

func directionCount13(result commandResult13, direction string) (int, bool, bool) {
	rows, ok := result.object()["routes"].([]any)
	if !ok {
		return 0, false, false
	}
	count, allPeer := 0, true
	for _, value := range rows {
		row, isRow := value.(map[string]any)
		if !isRow || row["direction"] != direction {
			continue
		}
		count++
		if _, ok := row["peer"]; !ok {
			allPeer = false
		}
	}
	return count, true, allPeer
}

func ribPipeFilter13(ctx context.Context, args []string) error {
	return observe13(ctx, "pipe-test", func(ctx context.Context, plugin *sdk.Plugin) error {
		if err := ribReady13(ctx, plugin); err != nil {
			return err
		}
		if err := waitEORSent13(ctx, plugin, "peer1"); err != nil {
			return err
		}
		injected := command13(ctx, plugin, "peer * update text origin igp local-preference 100 nhop 10.0.0.1 nlri ipv4/unicast add 192.168.1.0/24")
		if err := requireStatus13("inject sent RIB route", injected, "done"); err != nil {
			return err
		}
		if err := requireStatus13("settle injected route", command13(ctx, plugin, "request quiesce"), "done"); err != nil {
			return err
		}
		settled := pollCommand13(ctx, plugin, "show bgp rib count", 40, 250*time.Millisecond, func(result commandResult13) bool {
			count, ok := countFrom13(result)
			return done13(result) && ok && count == 2
		})
		if count, ok := countFrom13(settled); !done13(settled) || !ok || count != 2 {
			received := command13(ctx, plugin, "show bgp rib received count")
			sent := command13(ctx, plugin, "show bgp rib sent count")
			receivedCount, receivedOK := countFrom13(received)
			sentCount, sentOK := countFrom13(sent)
			return fmt.Errorf("RIB never contained both received and sent routes: total=%d valid=%v received=%d valid=%v sent=%d valid=%v", count, ok, receivedCount, receivedOK, sentCount, sentOK)
		}
		a := &assertions13{}
		for _, check := range []struct {
			label, command string
			directions     []string
		}{
			{"scope-received", "show bgp rib received", []string{directionReceived}},
			{"scope-sent", "show bgp rib sent", []string{directionSent}},
			{"scope-sent-received", "show bgp rib sent-received", []string{directionReceived, directionSent}},
			{"scope-default", "show bgp rib", []string{directionReceived, directionSent}},
		} {
			result := command13(ctx, plugin, check.command)
			a.status(check.label, result)
			for _, direction := range check.directions {
				count, valid, allPeer := directionCount13(result, direction)
				if !valid || count == 0 || !allPeer {
					a.failures = append(a.failures, fmt.Errorf("%s: direction=%s count=%d valid=%v all-peer=%v", check.label, direction, count, valid, allPeer))
				}
			}
		}
		for _, check := range []struct {
			label, command string
			count          int
		}{
			{"count-total", "show bgp rib count", 2},
			{"count-received", "show bgp rib received count", 1},
			{"count-sent", "show bgp rib sent count", 1},
			{"path-filter", "show bgp rib received path 64501 count", 0},
			{"community-filter", "show bgp rib received community 65000:100 count", 0},
			{"family-filter", "show bgp rib received family ipv4/unicast count", 1},
			{"prefix-filter", "show bgp rib sent prefix 192.168.1.0/24 count", 1},
			{"combined-filter", "show bgp rib received family ipv4/unicast community 65000:100 count", 0},
		} {
			result := command13(ctx, plugin, check.command)
			a.status(check.label, result)
			if count, ok := countFrom13(result); !ok || count != check.count {
				a.failures = append(a.failures, fmt.Errorf("%s count=%d valid=%v want %d", check.label, count, ok, check.count))
			}
		}
		for _, check := range []struct{ label, command string }{{"best-path", "show bgp rib best"}, {"best-prefix", "show bgp rib best prefix 0.0.0.0/0"}} {
			result := command13(ctx, plugin, check.command)
			a.status(check.label, result)
			if _, ok := result.object()["best-path"]; !ok {
				a.failures = append(a.failures, fmt.Errorf("%s missing best-path", check.label))
			}
		}
		for _, check := range []struct{ label, command string }{{"unknown-keyword", "show bgp rib bogus"}, {"old-show-in", "show bgp rib in"}, {"old-show-out", "show bgp rib out"}} {
			result := command13(ctx, plugin, check.command)
			a.status(check.label, result)
			if _, ok := result.object()["error"]; !ok {
				keys := make([]string, 0, len(result.object()))
				for key := range result.object() {
					keys = append(keys, key)
				}
				sort.Strings(keys)
				a.failures = append(a.failures, fmt.Errorf("%s missing error; keys=%v", check.label, keys))
			}
		}
		return a.finish("RIB pipeline")
	})
}
