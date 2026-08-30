package fixture

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func plugin15EORSent(r plugin15Result) bool {
	if !plugin15Done(r) {
		return false
	}
	m, err := plugin15Map(r)
	if err != nil {
		return false
	}
	peers, _ := m["peers"].(map[string]any)
	for _, value := range peers {
		row, _ := value.(map[string]any)
		if plugin15Number(row["eor-sent"]) >= 1 {
			return true
		}
	}
	return false
}

func plugin15WaitEOR(ctx context.Context, p *sdk.Plugin, selector string) error {
	r := plugin15PollCommand(ctx, p, "show bgp peer "+selector+" detail", 40, 250*time.Millisecond, plugin15EORSent)
	if !plugin15EORSent(r) {
		return fmt.Errorf("ze never sent its End-of-RIB to %s", selector)
	}
	return nil
}

func plugin15RRBasic(ctx context.Context, p *sdk.Plugin) error {
	if err := plugin15WaitEOR(ctx, p, "peer-src"); err != nil {
		return err
	}
	detail := plugin15Dispatch(ctx, p, "show bgp peer peer-src detail")
	if !plugin15Done(detail) {
		return fmt.Errorf("peer detail failed: status=%s data=%s", detail.status, detail.text())
	}
	if !strings.Contains(detail.text(), "route-reflector-client") {
		return fmt.Errorf("missing route-reflector-client: %.300s", detail.text())
	}
	if !strings.Contains(detail.text(), "cluster-id") {
		return fmt.Errorf("missing cluster-id: %.300s", detail.text())
	}
	rib := plugin15PollCommand(ctx, p, "show bgp rib best", 40, 250*time.Millisecond, func(r plugin15Result) bool {
		m, err := plugin15Map(r)
		if err != nil {
			return false
		}
		entries, _ := m["best-path"].([]any)
		for _, value := range entries {
			row, _ := value.(map[string]any)
			if row["prefix"] == prefixTenOne {
				return true
			}
		}
		return false
	})
	if !plugin15Done(rib) {
		return fmt.Errorf("rib show best failed: status=%s data=%s", rib.status, rib.text())
	}
	m, err := plugin15Map(rib)
	if err != nil {
		return err
	}
	found := false
	entries, _ := m["best-path"].([]any)
	for _, value := range entries {
		row, _ := value.(map[string]any)
		found = found || row["prefix"] == prefixTenOne
	}
	if !found {
		return fmt.Errorf("route 10.1.0.0/24 not in RIB")
	}
	fmt.Fprintln(os.Stderr, "OK: RR client and cluster-id visible in peer detail, route in RIB")
	return plugin15WaitEOR(ctx, p, "peer-src")
}

func plugin15RRIPv6(ctx context.Context, p *sdk.Plugin) error {
	if err := plugin15WaitEOR(ctx, p, "peer-src"); err != nil {
		return err
	}
	detail := plugin15Dispatch(ctx, p, "show bgp peer peer-src detail")
	if !plugin15Done(detail) {
		return fmt.Errorf("peer detail failed: status=%s data=%s", detail.status, detail.text())
	}
	if !strings.Contains(detail.text(), "route-reflector-client") {
		return fmt.Errorf("missing route-reflector-client: %.300s", detail.text())
	}
	fmt.Fprintln(os.Stderr, "OK: IPv6 RR client config accepted, peer detail shows RR fields")
	return plugin15WaitEOR(ctx, p, "peer-src")
}

func plugin15RRStatus(ctx context.Context, p *sdk.Plugin) error {
	r := plugin15PollCommand(ctx, p, "show rr status", 40, 250*time.Millisecond, func(r plugin15Result) bool {
		m, _ := plugin15Map(r)
		running, _ := m["running"].(bool)
		return plugin15Done(r) && running
	})
	m, err := plugin15Map(r)
	if err != nil || !plugin15Done(r) || m["running"] != true {
		return fmt.Errorf("expected {running:true}, status=%s data=%s", r.status, r.text())
	}
	fmt.Fprintln(os.Stderr, "PASS: show rr status returned running=true")
	peersResult := plugin15Dispatch(ctx, p, "show rr peers")
	if !plugin15Done(peersResult) {
		return fmt.Errorf("show rr peers status=%s data=%s", peersResult.status, peersResult.text())
	}
	var decoded any
	if err := peersResult.value(&decoded); err != nil {
		return err
	}
	if object, ok := decoded.(map[string]any); ok {
		decoded = object["peers"]
	}
	peers, ok := decoded.([]any)
	if !ok {
		return fmt.Errorf("expected peer list, got: %s", peersResult.text())
	}
	fmt.Fprintf(os.Stderr, "PASS: show rr peers returned %d peer(s)\n", len(peers))
	return nil
}

func plugin15RuntimeMemory(ctx context.Context, p *sdk.Plugin) error {
	if err := plugin15WaitEOR(ctx, p, "*"); err != nil {
		return err
	}
	r := plugin15Dispatch(ctx, p, "show runtime memory")
	if !plugin15Done(r) {
		return fmt.Errorf("runtime-memory: status=%s data=%s", r.status, r.text())
	}
	m, err := plugin15Map(r)
	if err != nil {
		return fmt.Errorf("runtime-memory: expected dict: %w", err)
	}
	for _, key := range []string{"alloc", "heap-in-use", "num-gc"} {
		if _, ok := m[key]; !ok {
			return fmt.Errorf("runtime-memory: missing key %q", key)
		}
	}
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	fmt.Fprintf(os.Stderr, "OK: runtime memory returned %v\n", keys)
	return nil
}

func plugin15PeerGotRoute(r plugin15Result) bool {
	m, err := plugin15Map(r)
	if err != nil || !plugin15Done(r) {
		return false
	}
	peers, _ := m["peers"].(map[string]any)
	for _, value := range peers {
		row, _ := value.(map[string]any)
		if plugin15Number(row["updates-sent"])-plugin15Number(row["eor-sent"]) >= 1 {
			return true
		}
	}
	return false
}

func plugin15SendCommunity(ctx context.Context, p *sdk.Plugin) error {
	if r := plugin15Dispatch(ctx, p, "request quiesce"); !plugin15Done(r) {
		return fmt.Errorf("quiesce failed: %s", r.text())
	}
	for _, name := range []string{"dest-none-peer", "dest-standard-peer"} {
		r := plugin15PollCommand(ctx, p, "show bgp peer "+name+" detail", 24, 250*time.Millisecond, plugin15PeerGotRoute)
		if !plugin15PeerGotRoute(r) {
			return fmt.Errorf("%s was never sent the forwarded route: %s", name, r.text())
		}
		fmt.Fprintf(os.Stderr, "OK: %s was sent the forwarded route\n", name)
	}
	return nil
}

func plugin15ShowBGPBare(ctx context.Context, p *sdk.Plugin) error {
	bare := plugin15Dispatch(ctx, p, "show bgp")
	if !plugin15Done(bare) {
		return fmt.Errorf("show bgp status=%s data=%s", bare.status, bare.text())
	}
	data, err := plugin15Map(bare)
	if err != nil {
		return err
	}
	for _, key := range []string{fieldRouterID, fieldLocalAS, columnUptime, fieldPeersConfigured, fieldPeersEstablished, fieldPeers} {
		if _, ok := data[key]; !ok {
			return fmt.Errorf("show bgp is not the summary, %s is missing", key)
		}
	}
	unknown := plugin15Dispatch(ctx, p, "show bgp nonsense")
	message := unknown.text()
	if unknown.status != statusError || !strings.Contains(message, "unknown command") || !strings.Contains(message, "show bgp nonsense") {
		return fmt.Errorf("show bgp nonsense must report an unknown command; status=%s data=%q", unknown.status, message)
	}
	if strings.Contains(message, "invalid family") || strings.Contains(message, "afi/safi") {
		return fmt.Errorf("an unregistered subcommand is not a family typo; got %q", message)
	}
	fmt.Fprintf(os.Stderr, "OK: show bgp answers the summary for %v peer(s)\n", data["peers-configured"])
	return nil
}

func plugin15ShowBGPChildren(ctx context.Context, p *sdk.Plugin) error {
	var failures []string
	for _, command := range []string{
		"show bgp rpki status",
		"show bgp rs peers",
		"show bgp adj-rib-in",
		"show bgp healthcheck",
	} {
		r := plugin15Dispatch(ctx, p, command)
		if !plugin15Done(r) {
			failures = append(failures, fmt.Sprintf("%s status=%s data=%s", command, r.status, r.text()))
			continue
		}
		data, _ := plugin15Map(r)
		if _, swallowed := data["peers-configured"]; swallowed {
			failures = append(failures, command+" reached the summary handler")
			continue
		}
		fmt.Fprintf(os.Stderr, "OK: %s reached its own handler\n", command)
	}
	if r := plugin15Dispatch(ctx, p, "show bgp"); !plugin15Done(r) {
		failures = append(failures, "show bgp status="+r.status)
	}
	if len(failures) != 0 {
		return fmt.Errorf("%s", strings.Join(failures, "; "))
	}
	return nil
}

func plugin15ShowBGPFamily(ctx context.Context, p *sdk.Plugin) error {
	if err := plugin15WaitEOR(ctx, p, "*"); err != nil {
		return err
	}
	for _, command := range []string{"show bgp ipv4", "show bgp ipv4/unicast"} {
		r := plugin15Dispatch(ctx, p, command)
		if !plugin15Done(r) {
			return fmt.Errorf("%s status=%s data=%s", command, r.status, r.text())
		}
		data, err := plugin15Map(r)
		if err != nil {
			return err
		}
		if _, ok := data["peers-configured"]; !ok {
			return fmt.Errorf("%s is not a summary", command)
		}
		if data["family"] != familyIPv4Unicast {
			return fmt.Errorf("%s did not scope to ipv4/unicast: %v", command, data["family"])
		}
	}
	unnegotiated := plugin15Dispatch(ctx, p, "show bgp l2vpn")
	message := unnegotiated.text()
	if unnegotiated.status != statusError || !strings.Contains(message, "l2vpn") {
		return fmt.Errorf("show bgp l2vpn must reject and name the family: status=%s data=%q", unnegotiated.status, message)
	}
	if strings.Contains(message, "unknown command") {
		return fmt.Errorf("a known family is not an unknown command: %q", message)
	}
	fmt.Fprintln(os.Stderr, "OK: the family argument reaches the summary handler")
	return nil
}

func plugin15ShowBGPSummaryGone(ctx context.Context, p *sdk.Plugin) error {
	bare := plugin15Dispatch(ctx, p, "show bgp")
	bareMap, _ := plugin15Map(bare)
	if !plugin15Done(bare) {
		return fmt.Errorf("show bgp status=%s data=%s", bare.status, bare.text())
	}
	if _, ok := bareMap["peers-configured"]; !ok {
		return fmt.Errorf("show bgp did not answer the replacement summary")
	}
	var failures []string
	for _, command := range []string{"show bgp summary", "show bgp summary ipv4"} {
		r := plugin15Dispatch(ctx, p, command)
		message := r.text()
		switch {
		case r.status != statusError:
			failures = append(failures, fmt.Sprintf("%s status=%s want error", command, r.status))
		case !strings.Contains(message, "unknown command"):
			failures = append(failures, fmt.Sprintf("%s must report an unknown command; got %q", command, message))
		case !strings.Contains(message, "show bgp summary"):
			failures = append(failures, fmt.Sprintf("%s must name what was typed; got %q", command, message))
		case strings.Contains(message, "invalid family") || strings.Contains(message, "afi/safi") || strings.Contains(message, "negotiated"):
			failures = append(failures, fmt.Sprintf("a retired command is not a family typo; got %q", message))
		default:
			fmt.Fprintf(os.Stderr, "OK: %s answers %q\n", command, message)
		}
	}
	if len(failures) != 0 {
		return fmt.Errorf("%s", strings.Join(failures, "; "))
	}
	return plugin15WaitEOR(ctx, p, "peer1")
}

func plugin15RIBBestWalk(ctx context.Context, p *sdk.Plugin) error {
	const routes = 12
	var failures []string
	ready := plugin15PollCommand(ctx, p, "show bgp rib status", 40, 250*time.Millisecond, plugin15Done)
	if !plugin15Done(ready) {
		failures = append(failures, "rib plugin did not become ready")
	}
	for i := range routes {
		command := fmt.Sprintf("request bgp rib inject 10.0.0.1 ipv4/unicast 10.10.%d.0/24 aspath 64501,%d nhop 10.0.0.1 localpref 100", i, 64600+i)
		r := plugin15Dispatch(ctx, p, command)
		if !plugin15Done(r) {
			failures = append(failures, fmt.Sprintf("inject-%d: expected status=done, got=%s data=%s", i, r.status, r.text()))
			break
		}
		fmt.Fprintf(os.Stderr, "OK inject-%d: status=done\n", i)
	}
	best := plugin15Dispatch(ctx, p, "show bgp rib best")
	if !plugin15Done(best) {
		failures = append(failures, fmt.Sprintf("best: expected status=done, got=%s data=%s", best.status, best.text()))
	} else {
		fmt.Fprintln(os.Stderr, "OK best: status=done")
		data, err := plugin15Map(best)
		if err != nil {
			failures = append(failures, "best: invalid document: "+err.Error())
		} else {
			entries, ok := data["best-path"].([]any)
			if !ok {
				failures = append(failures, "best: no best-path envelope")
			} else {
				if len(entries) != routes {
					failures = append(failures, fmt.Sprintf("best: expected %d rows, got %d", routes, len(entries)))
				} else {
					fmt.Fprintf(os.Stderr, "OK best: %d rows\n", len(entries))
				}
				gotPrefixes := make([]string, 0, len(entries))
				for _, value := range entries {
					row, _ := value.(map[string]any)
					prefix, _ := row["prefix"].(string)
					gotPrefixes = append(gotPrefixes, prefix)
				}
				wantPrefixes := make([]string, routes)
				for i := range routes {
					wantPrefixes[i] = fmt.Sprintf("10.10.%d.0/24", i)
				}
				sort.Strings(gotPrefixes)
				sort.Strings(wantPrefixes)
				if !reflect.DeepEqual(gotPrefixes, wantPrefixes) {
					failures = append(failures, fmt.Sprintf("best: prefixes %v != %v", gotPrefixes, wantPrefixes))
				} else {
					fmt.Fprintln(os.Stderr, "OK best: every injected prefix is a row")
				}
				if len(entries) != 0 {
					first, _ := entries[0].(map[string]any)
					for _, field := range []string{fieldFamily, columnPrefix, "best-peer", "attributes"} {
						if _, ok := first[field]; !ok {
							failures = append(failures, fmt.Sprintf("best: row is missing %q", field))
						}
					}
					attributes, _ := first["attributes"].(map[string]any)
					if attributes["next-hop"] != addrPeerOne {
						failures = append(failures, fmt.Sprintf("best: row lost its next-hop: %v", attributes))
					} else {
						fmt.Fprintln(os.Stderr, "OK best: the row carries the attributes it was selected with")
					}
				}
			}
		}
	}
	first := plugin15Dispatch(ctx, p, "show bgp rib best first 2")
	if !plugin15Done(first) {
		failures = append(failures, fmt.Sprintf("best-first: expected status=done, got=%s", first.status))
	} else {
		fmt.Fprintln(os.Stderr, "OK best-first: status=done")
		data, _ := plugin15Map(first)
		entries, _ := data["best-path"].([]any)
		if len(entries) != 2 {
			failures = append(failures, fmt.Sprintf("best-first: expected 2 rows, got %d", len(entries)))
		} else {
			fmt.Fprintln(os.Stderr, "OK best-first: the walk stopped at two rows")
		}
	}
	count := plugin15Dispatch(ctx, p, "show bgp rib best count")
	if !plugin15Done(count) {
		failures = append(failures, fmt.Sprintf("best-count: expected status=done, got=%s", count.status))
	} else {
		fmt.Fprintln(os.Stderr, "OK best-count: status=done")
		data, _ := plugin15Map(count)
		if plugin15Number(data["count"]) != routes {
			failures = append(failures, fmt.Sprintf("best-count: expected count=%d, got %v", routes, data))
		} else {
			fmt.Fprintf(os.Stderr, "OK best-count: count=%d\n", routes)
		}
	}
	if len(failures) != 0 {
		return fmt.Errorf("%d best-path walk assertions failed: %s", len(failures), strings.Join(failures, "; "))
	}
	return nil
}

func plugin15RIBDocumentTotal(value any) (int, bool) {
	object, ok := value.(map[string]any)
	if !ok {
		return 0, false
	}
	if routes, ok := object["routes"].([]any); ok {
		return len(routes), true
	}
	rib, ok := object["adj-rib-in"].(map[string]any)
	if !ok {
		return 0, false
	}
	total := 0
	for _, value := range rib {
		routes, ok := value.([]any)
		if !ok {
			return 0, false
		}
		total += len(routes)
	}
	return total, true
}

func plugin15RIBUnderLoad(ctx context.Context, p *sdk.Plugin) error {
	const (
		routes   = 200000
		walksMax = 4000
	)
	var totals []int
	walks := 0
	partial := 0
	var failures []string
	for walks < walksMax {
		walkCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		r := plugin15Dispatch(walkCtx, p, "show bgp rib count")
		timedOut := walkCtx.Err() == context.DeadlineExceeded
		cancel()
		if r.err != nil && r.status == statusError {
			if timedOut {
				return fmt.Errorf("`show bgp rib count` did not answer within 20s at count-%d: the walk is wedged against the UPDATE path", walks)
			}
			continue
		}
		walks++
		if !plugin15Done(r) {
			failures = append(failures, fmt.Sprintf("count-%d: expected status=done, got=%s", walks, r.status))
			break
		}
		data, err := plugin15Map(r)
		if err != nil {
			failures = append(failures, fmt.Sprintf("count-%d: no count in the answer: %s", walks, r.text()))
			break
		}
		totalValue, ok := data["count"].(float64)
		if !ok {
			failures = append(failures, fmt.Sprintf("count-%d: no count in the answer: %v", walks, data))
			break
		}
		total := int(totalValue)
		totals = append(totals, total)
		if total > 0 && total < routes {
			partial++
			if partial == 1 {
				docCtx, docCancel := context.WithTimeout(ctx, 20*time.Second)
				doc := plugin15Dispatch(docCtx, p, "show bgp rib")
				docTimedOut := docCtx.Err() == context.DeadlineExceeded
				docCancel()
				if docTimedOut {
					return fmt.Errorf("`show bgp rib` did not answer within 20s at document: the walk is wedged against the UPDATE path")
				}
				if !plugin15Done(doc) {
					failures = append(failures, fmt.Sprintf("document: expected status=done, got=%s", doc.status))
				} else {
					var value any
					if err := doc.value(&value); err != nil {
						failures = append(failures, "document: no adj-rib-in map in the answer")
					} else if documentTotal, ok := plugin15RIBDocumentTotal(value); !ok {
						failures = append(failures, "document: no adj-rib-in map in the answer")
					} else {
						fmt.Fprintf(os.Stderr, "OK document: dumped %d routes while the peer was still sending\n", documentTotal)
					}
				}
			}
		}
		if total >= routes {
			break
		}
	}
	var first, last any
	if len(totals) != 0 {
		first, last = totals[0], totals[len(totals)-1]
	}
	fmt.Fprintf(os.Stderr, "OK walks: %d dumps answered, first=%v last=%v partial=%d\n", walks, first, last, partial)
	if len(totals) == 0 || totals[len(totals)-1] < routes {
		failures = append(failures, fmt.Sprintf("the table never reached %d routes: last=%v. The peer did not deliver its burst, so this test measured nothing", routes, last))
	}
	if partial == 0 {
		failures = append(failures, "no dump saw a partially filled table, so no walk overlapped the UPDATE path and this test measured nothing")
	}
	if len(failures) != 0 {
		return fmt.Errorf("%d rib-under-load assertions failed: %s", len(failures), strings.Join(failures, "; "))
	}
	return nil
}

func plugin15SmartShow(ctx context.Context, p *sdk.Plugin) error {
	r := plugin15Dispatch(ctx, p, "show storage smart")
	if !plugin15Done(r) {
		return fmt.Errorf("show storage smart (configured): expected done, got status=%s data=%.200s", r.status, r.text())
	}
	data, err := plugin15Map(r)
	if err != nil {
		return fmt.Errorf("show storage smart: invalid JSON: %w", err)
	}
	devices, ok := data["devices"]
	if !ok {
		return fmt.Errorf("show storage smart: missing 'devices' key: %v", data)
	}
	if _, ok := devices.([]any); !ok {
		return fmt.Errorf("show storage smart: 'devices' is not a list")
	}
	return nil
}

func plugin15SmartUnconfigured(ctx context.Context, p *sdk.Plugin) error {
	r := plugin15Dispatch(ctx, p, "show storage smart")
	if r.status != statusError {
		return fmt.Errorf("show storage smart (unconfigured): expected error, got status=%s data=%.200s", r.status, r.text())
	}
	const want = "storage SMART management not configured"
	if !strings.Contains(r.text(), want) {
		return fmt.Errorf("show storage smart (unconfigured): expected error containing %q, got %.200q", want, r.text())
	}
	return nil
}

func plugin15DispatchUntilDone(ctx context.Context, p *sdk.Plugin, command string) plugin15Result {
	return plugin15PollCommand(ctx, p, command, 40, 250*time.Millisecond, plugin15Done)
}
