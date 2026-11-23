// Design: docs/architecture/plugin/rib-storage-design.md -- RFC 7947 Section 2.1 pre-policy Adj-RIB-In retention fixtures
// RFC: rfc/short/rfc7947.md -- Section 2.1
// Related: register_adj_rib_in_filtered.go -- fixture registration
package fixture

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func findAdjRIBInRow(rows []any, nlriHex string) map[string]any {
	want := strings.ToLower(nlriHex)
	for _, entry := range rows {
		row := map09(entry)
		if strings.ToLower(fmt.Sprint(row["nlri-hex"])) == want {
			return row
		}
	}
	return nil
}

func writeStderrOK(line string) {
	_, _ = os.Stderr.WriteString(line) //nolint:errcheck // best-effort diagnostic line for runner stderr assertion
}

func requireAdjRIBInStatus(status commandResult09, wantTotal, wantFiltered string) error {
	if !done09(status) {
		return fmt.Errorf("show bgp adj-rib-in status failed: %s", status.text)
	}
	if !strings.Contains(status.text, wantTotal) {
		return fmt.Errorf("adj-rib-in status missing %s: %s", wantTotal, status.text)
	}
	if !strings.Contains(status.text, wantFiltered) {
		return fmt.Errorf("adj-rib-in status missing %s: %s", wantFiltered, status.text)
	}
	return nil
}

// adjRIBInFilteredVisible verifies that a loop-denied UPDATE from an rs-client
// peer is retained in Adj-RIB-In marked filtered with filter-step "loop", is
// rendered on json/yaml/table pipes, updates ze_adjribin_routes_filtered, and
// stays absent from bgp-rib while an accepted control is installed.
func adjRIBInFilteredVisible(ctx context.Context, _ []string) error {
	return observe09(ctx, "adj-rib-in-filtered-visible", func(ctx context.Context, p *sdk.Plugin) error {
		if err := p12WaitPeersEOR(ctx, p, "rs-client-peer"); err != nil {
			return err
		}
		status := pollCommand09(ctx, p, "show bgp adj-rib-in status", 80, func(r commandResult09) bool {
			return done09(r) && strings.Contains(r.text, `"total-routes":2`) && strings.Contains(r.text, `"filtered-routes":1`)
		})
		if err := requireAdjRIBInStatus(status, `"total-routes":2`, `"filtered-routes":1`); err != nil {
			return err
		}

		// Wait for the accepted control route to land in bgp-rib before judging absence of the denied route.
		ribResp := pollCommand09(ctx, p, "show bgp rib", 80, func(r commandResult09) bool {
			return done09(r) && strings.Contains(r.text, "10.0.0.0/24")
		})
		if !done09(ribResp) {
			return fmt.Errorf("show bgp rib failed: %s", ribResp.text)
		}
		if !strings.Contains(ribResp.text, "10.0.0.0/24") {
			return fmt.Errorf("bgp-rib never received accepted control 10.0.0.0/24: %s", ribResp.text)
		}
		if strings.Contains(ribResp.text, "10.0.1.0/24") {
			return fmt.Errorf("bgp-rib leaked loop-denied prefix 10.0.1.0/24: %s", ribResp.text)
		}

		show := dispatch09(ctx, p, "show bgp adj-rib-in")
		if !done09(show) {
			return fmt.Errorf("show bgp adj-rib-in failed: %s", show.text)
		}
		peerRows := list09(map09(map09(show.data)["adj-rib-in"])["127.0.0.1"])
		if len(peerRows) != 2 {
			return fmt.Errorf("127.0.0.1 stored %d rows, expected 2: %s", len(peerRows), show.text)
		}
		deniedRow := findAdjRIBInRow(peerRows, "180a0001")
		if deniedRow == nil {
			return fmt.Errorf("denied 10.0.1.0/24 missing from adj-rib-in: %s", show.text)
		}
		if deniedRow["filtered"] != true {
			return fmt.Errorf("denied 10.0.1.0/24 filtered=%v, want true: %s", deniedRow["filtered"], show.text)
		}
		if fmt.Sprint(deniedRow["filter-step"]) != "loop" {
			return fmt.Errorf("denied 10.0.1.0/24 filter-step=%q, want loop: %s", deniedRow["filter-step"], show.text)
		}
		acceptedRow := findAdjRIBInRow(peerRows, "180a0000")
		if acceptedRow == nil {
			return fmt.Errorf("accepted 10.0.0.0/24 missing from adj-rib-in: %s", show.text)
		}
		if acceptedRow["filtered"] != false {
			return fmt.Errorf("accepted 10.0.0.0/24 filtered=%v, want false: %s", acceptedRow["filtered"], show.text)
		}

		// Verify json, yaml, and table pipes each expose the filtered population.
		for _, pipe := range []string{"| json", "| yaml", "| table"} {
			rendered := dispatch09(ctx, p, "show bgp adj-rib-in "+pipe)
			if !done09(rendered) {
				return fmt.Errorf("show bgp adj-rib-in %s failed: %s", pipe, rendered.text)
			}
			if !strings.Contains(rendered.text, "filtered") {
				return fmt.Errorf("show bgp adj-rib-in %s omitted filtered: %s", pipe, rendered.text)
			}
			if !strings.Contains(rendered.text, "loop") {
				return fmt.Errorf("show bgp adj-rib-in %s omitted loop: %s", pipe, rendered.text)
			}
		}

		// Verify the Prometheus filtered-route gauge reflects the stored filtered entry.
		const filteredMetric = `ze_adjribin_routes_filtered{peer="127.0.0.1"}`
		metricText := metricText09(ctx, p)
		val, ok := metricValue09(metricText, filteredMetric)
		if !ok {
			return fmt.Errorf("metric %s missing from show metrics values", filteredMetric)
		}
		if val != 1 {
			return fmt.Errorf("metric %s = %v, want 1", filteredMetric, val)
		}

		var tb textbuf.Buffer
		tb.Str("OK: adj-rib-in holds loop-denied 10.0.1.0/24 (filtered=true, step=loop), rib omits it, metric=").Int(int64(val)).Byte('\n')
		writeStderrOK(tb.String())
		return nil
	})
}

// adjRIBInFilteredStatus verifies that an import-policy-denied UPDATE on an
// rs-client peer is counted separately in show bgp adj-rib-in status alongside
// total-routes and carries filter-step "peer-policy-chain".
func adjRIBInFilteredStatus(ctx context.Context, _ []string) error {
	return observe09(ctx, "adj-rib-in-filtered-status", func(ctx context.Context, p *sdk.Plugin) error {
		if err := p12WaitPeersEOR(ctx, p, "rs-client-peer"); err != nil {
			return err
		}
		status := pollCommand09(ctx, p, "show bgp adj-rib-in status", 80, func(r commandResult09) bool {
			return done09(r) && strings.Contains(r.text, `"total-routes":2`) && strings.Contains(r.text, `"filtered-routes":1`)
		})
		if err := requireAdjRIBInStatus(status, `"total-routes":2`, `"filtered-routes":1`); err != nil {
			return err
		}

		show := dispatch09(ctx, p, "show bgp adj-rib-in")
		if !done09(show) {
			return fmt.Errorf("show bgp adj-rib-in failed: %s", show.text)
		}
		peerRows := list09(map09(map09(show.data)["adj-rib-in"])["127.0.0.1"])
		deniedRow := findAdjRIBInRow(peerRows, "180a0002")
		if deniedRow == nil {
			return fmt.Errorf("policy-denied 10.0.2.0/24 missing from adj-rib-in: %s", show.text)
		}
		if deniedRow["filtered"] != true {
			return fmt.Errorf("policy-denied 10.0.2.0/24 filtered=%v, want true: %s", deniedRow["filtered"], show.text)
		}
		if fmt.Sprint(deniedRow["filter-step"]) != "peer-policy-chain" {
			return fmt.Errorf("policy-denied 10.0.2.0/24 filter-step=%q, want peer-policy-chain: %s", deniedRow["filter-step"], show.text)
		}

		for _, pipe := range []string{"| json", "| yaml", "| table"} {
			rendered := dispatch09(ctx, p, "show bgp adj-rib-in status "+pipe)
			if !done09(rendered) {
				return fmt.Errorf("show bgp adj-rib-in status %s failed: %s", pipe, rendered.text)
			}
			if !strings.Contains(rendered.text, "filtered-routes") {
				return fmt.Errorf("show bgp adj-rib-in status %s omitted filtered-routes: %s", pipe, rendered.text)
			}
		}

		writeStderrOK("OK: adj-rib-in status reports total-routes=2 filtered-routes=1 (step=peer-policy-chain)\n")
		return nil
	})
}

// adjRIBInFilteredNotReplayed verifies that the filtered entry is actually
// stored in Adj-RIB-In before the second rs-client peer reconnects, so the
// peer's absence of the filtered prefix on replay is non-vacuous.
func adjRIBInFilteredNotReplayed(ctx context.Context, _ []string) error {
	return passiveObserve09(ctx, "adj-rib-in-filtered-not-replayed", &sdk.Registration{}, func(ctx context.Context, p *sdk.Plugin) error {
		status := pollCommand09(ctx, p, "show bgp adj-rib-in status", 120, func(r commandResult09) bool {
			return done09(r) && strings.Contains(r.text, `"total-routes":2`) && strings.Contains(r.text, `"filtered-routes":1`)
		})
		if err := requireAdjRIBInStatus(status, `"total-routes":2`, `"filtered-routes":1`); err != nil {
			return err
		}
		var tb textbuf.Buffer
		tb.Str("OK: adj-rib-in stored filtered route before replay (").Str(status.text).Str(")\n")
		writeStderrOK(tb.String())
		return nil
	})
}

// adjRIBInFilteredAbsentForPlainPeer verifies AC-5: a non-rs-client peer's
// loop-denied UPDATE stays absent from Adj-RIB-In while its accepted control
// route is stored normally.
func adjRIBInFilteredAbsentForPlainPeer(ctx context.Context, _ []string) error {
	return observe09(ctx, "adj-rib-in-filtered-absent-for-plain-peer", func(ctx context.Context, p *sdk.Plugin) error {
		if err := p12WaitPeersEOR(ctx, p, "plain-peer"); err != nil {
			return err
		}
		status := pollCommand09(ctx, p, "show bgp adj-rib-in status", 80, func(r commandResult09) bool {
			return done09(r) && strings.Contains(r.text, `"total-routes":1`)
		})
		if err := requireAdjRIBInStatus(status, `"total-routes":1`, `"filtered-routes":0`); err != nil {
			return err
		}
		show := dispatch09(ctx, p, "show bgp adj-rib-in")
		if !done09(show) {
			return fmt.Errorf("show bgp adj-rib-in failed: %s", show.text)
		}
		peerRows := list09(map09(map09(show.data)["adj-rib-in"])["127.0.0.1"])
		if len(peerRows) != 1 {
			return fmt.Errorf("plain peer stored %d rows, want 1: %s", len(peerRows), show.text)
		}
		if findAdjRIBInRow(peerRows, "180a0000") == nil {
			return fmt.Errorf("plain peer missing accepted 10.0.0.0/24: %s", show.text)
		}
		if findAdjRIBInRow(peerRows, "180a0001") != nil {
			return fmt.Errorf("plain peer leaked loop-denied 10.0.1.0/24 into adj-rib-in: %s", show.text)
		}
		writeStderrOK("OK: plain peer adj-rib-in holds only accepted control (total=1, filtered=0)\n")
		return nil
	})
}
