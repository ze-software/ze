package fixture

// plugin_fixture_show_mtu.go drives `show mtu` from inside a daemon whose
// namespace sits behind a router that clamps the far link to 1400 octets. The
// .ci builds the topology; each scenario asks one question of the payload.
//
// Design: docs/architecture/diagnostics/path-mtu.md -- the run and the payload
// Related: register_show_mtu.go -- the scenario names
// Related: test/plugin/show-mtu-host.ci -- one address, the way it was found
// Related: test/plugin/show-mtu-exhaustive.ci -- a poisoned route MTU
// Related: test/plugin/show-mtu-json.ci -- the payload as one document
// Related: test/plugin/show-mtu-no-ipsec-component.ci -- a registered inventory with no tunnel
// Related: test/plugin/show-mtu-oversized-tunnels.ci -- two bound tunnels over the clamp
// Related: plugin_fixture_13.go -- observe13, command13 and requireStatus13

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	// showMTUFarAddr is the far namespace's address, two hops from the daemon.
	showMTUFarAddr = "10.99.2.1"
	// showMTUSecondPeer is the second far-side IKE responder's address.
	showMTUSecondPeer = "10.99.2.3"
	// showMTUClampMTU is the MTU the router's far link is clamped to.
	showMTUClampMTU = 1400
	// showMTUPoisonMTU is the route MTU the exhaustive test writes on the far
	// address, which the honor-cache search believes and the exhaustive one does
	// not.
	showMTUPoisonMTU = 1300
	// showMTUUnderlay is the sender's link, the underlay `show mtu` reports.
	showMTUUnderlay = "sr0"
	// showMTUGCMCeiling and showMTURecommended are the aes128gcm figures over
	// a 1400 path with IPv4 endpoints and no UDP: overhead 52, ceiling
	// alignDown(1348, 4) - 2, recommended alignDown(1346 - 32 + 2, 4) - 2.
	// They size the host measurement of the far address (ICMP, 1400).
	showMTUGCMCeiling  = 1346
	showMTURecommended = 1314
	// showMTUIKEConfirmed is the largest size the padded IKE exchange proves
	// on a live tunnel here. The IKE SA is aes256-cbc (RFC 7296 Section 3.14
	// grid of 16 octets), so the exchange asked at the 1400 clamp is sent at
	// the grid point below, 1388, and answered whole. A fit below the ask
	// refutes nothing, so the ICMP figure 1400 stays the path MTU and the row
	// carries `ike-confirmed: 1388` (spec ike-padded-path-probe, design point
	// 0 and the 2026-09-16 ruling).
	showMTUIKEConfirmed = 1388
	// showMTUXfrmMTU is the MTU both bound xfrm interfaces are configured at.
	showMTUXfrmMTU = 1500
	// showMTUKeyVerdict is the payload key of a run's and a tunnel's verdict.
	showMTUKeyVerdict = "verdict"
)

// showMTUResult runs one show mtu command and decodes its document.
func showMTUResult(ctx context.Context, plugin *sdk.Plugin, command string) (map[string]any, error) {
	result := command13(ctx, plugin, command)
	if err := requireStatus13(command, result, statusDone); err != nil {
		return nil, err
	}
	var data map[string]any
	if err := decodeJSON13(result.raw, &data); err != nil {
		return nil, fmt.Errorf("%s: decode: %w", command, err)
	}
	return data, nil
}

func showMTURows(doc map[string]any, key string) ([]map[string]any, error) {
	list, ok := doc[key].([]any)
	if !ok {
		return nil, fmt.Errorf("%s is not a list: %v", key, doc[key])
	}
	rows := make([]map[string]any, 0, len(list))
	for i := range list {
		row, ok := list[i].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("%s[%d] is not a row: %v", key, i, list[i])
		}
		rows = append(rows, row)
	}
	return rows, nil
}

// showMTUMeasured checks the first measurement targets addr with the path
// MTU want and the method want.
func showMTUMeasured(doc map[string]any, addr string, want int, method string) error {
	rows, err := showMTURows(doc, "measurements")
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return fmt.Errorf("no measurement: %v", doc)
	}
	if rows[0]["target"] != addr {
		return fmt.Errorf("measurement target %v, want %s", rows[0]["target"], addr)
	}
	if got := number13(rows[0]["path-mtu"]); got != want {
		return fmt.Errorf("path-mtu %d, want %d: %v", got, want, rows[0])
	}
	if rows[0]["method"] != method {
		return fmt.Errorf("method %v, want %q: %v", rows[0]["method"], method, rows[0])
	}
	return nil
}

func showMTUHasNote(doc map[string]any, fragment string) bool {
	rows, err := showMTURows(doc, "notes")
	if err != nil {
		return false
	}
	for _, row := range rows {
		if text, ok := row["text"].(string); ok && strings.Contains(text, fragment) {
			return true
		}
	}
	return false
}

func showMTUHost(ctx context.Context, _ []string) error {
	return observe13(ctx, "show-mtu-test", func(ctx context.Context, plugin *sdk.Plugin) error {
		command := "show mtu host " + showMTUFarAddr
		doc, err := showMTUResult(ctx, plugin, command)
		if err != nil {
			return err
		}
		if doc["status"] != "ok" {
			return fmt.Errorf("%s: status %v, want ok: %v", command, doc["status"], doc)
		}
		if err := showMTUMeasured(doc, showMTUFarAddr, showMTUClampMTU, "via ICMP"); err != nil {
			return fmt.Errorf("%s: %w", command, err)
		}
		for _, absent := range []string{"inventory", showMTUKeyVerdict, "reference"} {
			if _, present := doc[absent]; present {
				return fmt.Errorf("%s: a host run carries %s: %v", command, absent, doc[absent])
			}
		}
		underlay, ok := doc["underlay"].(map[string]any)
		if !ok {
			return fmt.Errorf("%s: no underlay row: %v", command, doc)
		}
		if underlay["interface"] != showMTUUnderlay {
			return fmt.Errorf("%s: underlay %v, want %s", command, underlay["interface"], showMTUUnderlay)
		}
		detail, err := showMTUResult(ctx, plugin, command+" detail")
		if err != nil {
			return err
		}
		rows, err := showMTURows(detail, "measurements")
		if err != nil {
			return err
		}
		sent, err := showMTURows(rows[0], "probes-sent")
		if err != nil {
			return fmt.Errorf("%s detail: %w", command, err)
		}
		if len(sent) != number13(rows[0]["probes"]) {
			return fmt.Errorf("%s detail: %d probe rows for %v probes", command, len(sent), rows[0]["probes"])
		}
		fmt.Fprintln(os.Stderr, "OK: show mtu host measured the clamp at 1400 via ICMP over", underlay["interface"])
		return nil
	})
}

func showMTUExhaustive(ctx context.Context, _ []string) error {
	return observe13(ctx, "show-mtu-test", func(ctx context.Context, plugin *sdk.Plugin) error {
		honor := "show mtu host " + showMTUFarAddr
		doc, err := showMTUResult(ctx, plugin, honor)
		if err != nil {
			return err
		}
		if err := showMTUMeasured(doc, showMTUFarAddr, showMTUPoisonMTU, "via local iface MTU"); err != nil {
			return fmt.Errorf("%s: %w", honor, err)
		}
		exhaustive := honor + " exhaustive"
		doc, err = showMTUResult(ctx, plugin, exhaustive)
		if err != nil {
			return err
		}
		if err := showMTUMeasured(doc, showMTUFarAddr, showMTUClampMTU, "forced full search"); err != nil {
			return fmt.Errorf("%s: %w", exhaustive, err)
		}
		rows, err := showMTURows(doc, "measurements")
		if err != nil {
			return err
		}
		if got := number13(rows[0]["cached-path-mtu"]); got != showMTUPoisonMTU {
			return fmt.Errorf("%s: cached-path-mtu %d, want the poisoned %d", exhaustive, got, showMTUPoisonMTU)
		}
		if !showMTUHasNote(doc, "stale PMTU of 1300") {
			return fmt.Errorf("%s: no note names the stale cache: %v", exhaustive, doc["notes"])
		}
		fmt.Fprintln(os.Stderr, "OK: the poisoned 1300 held the honor-cache answer and exhaustive measured 1400 on the wire")
		return nil
	})
}

// showMTUJSON checks the document every pipe renders from: every key is
// kebab-case, the lists are lists, and no identifier carries a marker. The
// unit test TestShowMTUEveryPipeRendersOnePayload drives the same document
// through `| json`, `| yaml` and `| table`.
func showMTUJSON(ctx context.Context, _ []string) error {
	return observe13(ctx, "show-mtu-test", func(ctx context.Context, plugin *sdk.Plugin) error {
		command := "show mtu host " + showMTUFarAddr
		doc, err := showMTUResult(ctx, plugin, command)
		if err != nil {
			return err
		}
		for key := range doc {
			if strings.ToLower(key) != key || strings.ContainsAny(key, "_ ") {
				return fmt.Errorf("%s: key %q is not kebab-case", command, key)
			}
		}
		for _, key := range []string{"measurements", "tunnels", "notes"} {
			if _, err := showMTURows(doc, key); err != nil {
				return fmt.Errorf("%s: %w", command, err)
			}
		}
		for _, key := range []string{"commands", "caveats"} {
			if _, ok := doc[key].([]any); !ok {
				return fmt.Errorf("%s: %s is not a list: %v", command, key, doc[key])
			}
		}
		rows, err := showMTURows(doc, "measurements")
		if err != nil {
			return err
		}
		for _, row := range rows {
			target, _ := row["target"].(string)
			if strings.ContainsAny(target, "*! ") {
				return fmt.Errorf("%s: target %q carries a marker", command, target)
			}
		}
		fmt.Fprintln(os.Stderr, "OK: show mtu answers one kebab-case document with its lists as lists")
		return nil
	})
}

// showMTUNoTunnels checks a registered inventory that holds no tunnel is
// named as such: `inventory` registered and the verdict no-tunnels, which
// is a different answer from the not-registered one the unit test
// TestShowMTUNoInventoryIsNotZeroTunnels pins.
func showMTUNoTunnels(ctx context.Context, _ []string) error {
	return observe13(ctx, "show-mtu-test", func(ctx context.Context, plugin *sdk.Plugin) error {
		doc, err := showMTUResult(ctx, plugin, "show mtu")
		if err != nil {
			return err
		}
		if doc["inventory"] != "registered" {
			return fmt.Errorf("show mtu: inventory %v, want registered: %v", doc["inventory"], doc)
		}
		if doc[showMTUKeyVerdict] != "no-tunnels" {
			return fmt.Errorf("show mtu: verdict %v, want no-tunnels: %v", doc[showMTUKeyVerdict], doc)
		}
		rows, err := showMTURows(doc, "tunnels")
		if err != nil {
			return err
		}
		if len(rows) != 0 {
			return fmt.Errorf("show mtu: %d tunnels, want none", len(rows))
		}
		if showMTUHasNote(doc, "no IPsec inventory is registered") {
			return fmt.Errorf("show mtu: a registered inventory was reported unregistered: %v", doc["notes"])
		}
		fmt.Fprintln(os.Stderr, "OK: a registered inventory with no tunnel answers verdict no-tunnels")
		return nil
	})
}

// showMTUOversized waits for both bound tunnels to be sized, then checks each
// is oversized against the 1400 clamp with the aes128gcm figures, that both
// commands are listed, and that the reference on the far side makes the
// circuit-clamped underlay advice (AC-13). The tunnels are measured over
// their live IKE SAs: the padded exchange confirms the 1400 clamp down to
// the 1388 grid point (`ike-confirmed`) and the clamp stays the figure.
func showMTUOversized(ctx context.Context, _ []string) error {
	return observe13(ctx, "show-mtu-test", func(ctx context.Context, plugin *sdk.Plugin) error {
		// The tunnels are measured over their live IKE SAs, so the run waits
		// for both SAs to be established before it sizes; inventory Up (the
		// child SA installed) can lead the IKE SA being probe-ready by a tick,
		// and a probe refused sa-down in that window would leave the ICMP
		// figure and read as a flake.
		var saErr error
		established := Poll(ctx, 20, 3*time.Second, func() bool {
			saErr = showMTUSAsEstablished(ctx, plugin, "site-b", "site-c")
			return saErr == nil
		})
		if !established {
			return fmt.Errorf("both IKE SAs were not established within the poll: %w", saErr)
		}
		var doc map[string]any
		sized := Poll(ctx, 20, 3*time.Second, func() bool {
			d, err := showMTUResult(ctx, plugin, "show mtu")
			if err != nil {
				return false
			}
			doc = d
			rows, err := showMTURows(d, "tunnels")
			if err != nil || len(rows) != 2 {
				return false
			}
			return rows[0]["sized"] == true && rows[1]["sized"] == true
		})
		if !sized {
			return fmt.Errorf("show mtu: both tunnels were not sized within the poll: %v", doc)
		}
		rows, err := showMTURows(doc, "tunnels")
		if err != nil {
			return err
		}
		for _, row := range rows {
			if err := showMTUOversizedRow(row); err != nil {
				return err
			}
		}
		measurements, err := showMTURows(doc, "measurements")
		if err != nil {
			return err
		}
		for _, row := range measurements {
			if row["target"] != showMTUFarAddr && row["target"] != showMTUSecondPeer {
				continue
			}
			if err := showMTUIKEProbeRow(row); err != nil {
				return fmt.Errorf("%w; row %v; notes %v", err, row, doc["notes"])
			}
		}
		commands, ok := doc["commands"].([]any)
		if !ok {
			return fmt.Errorf("commands is not a list: %v", doc["commands"])
		}
		joined := fmt.Sprint(commands)
		for _, want := range []string{"set interface xfrm xa mtu 1314", "set interface xfrm xb mtu 1314", "set interface veth sr0 mtu 1400"} {
			if !strings.Contains(joined, want) {
				return fmt.Errorf("commands %v lack %q", commands, want)
			}
		}
		if doc[showMTUKeyVerdict] != "action-needed" {
			return fmt.Errorf("run verdict %v, want action-needed", doc[showMTUKeyVerdict])
		}
		underlay, ok := doc["underlay"].(map[string]any)
		if !ok {
			return errors.New("no underlay row")
		}
		if underlay["advice"] != "circuit-clamped" {
			return fmt.Errorf("underlay advice %v, want circuit-clamped: %v", underlay["advice"], doc["notes"])
		}
		fmt.Fprintln(os.Stderr, "OK: both bound tunnels are oversized against the 1400 clamp and the circuit is clamped")
		return nil
	})
}

// showMTUIKEProbe proves a live tunnel's path is measured by the padded IKE
// exchange: each peer measurement row names `ike` as its prober, keeps the
// 1400 clamp as its figure with `ike-confirmed` at the 1388 grid point the
// exchange proved, spent at least one exchange, carries the different-path
// caveat (the SAs run on UDP/500, not NAT-T) and never the ICMP optimism
// caveat, and both IKE SAs are still established afterwards. A run-level
// caution note names the CBC grid rounding as a confirmation. The ICMP figure
// comes from the router's Fragmentation Needed on the ESP path (the far
// dataplane is noop, so echo never round-trips), and the IKE prober confirms
// it: this is the confirm-first path, prover of AC-1 and AC-7 and design
// point 0.
func showMTUIKEProbe(ctx context.Context, _ []string) error {
	return observe13(ctx, "show-mtu-test", func(ctx context.Context, plugin *sdk.Plugin) error {
		// With the router's errors dropped every ICMP search runs on silence,
		// about 45 s per target, so the SAs are waited for first and `show
		// mtu` runs once against two live tunnels.
		var saErr error
		established := Poll(ctx, 20, 3*time.Second, func() bool {
			saErr = showMTUSAsEstablished(ctx, plugin, "site-b", "site-c")
			return saErr == nil
		})
		if !established {
			return fmt.Errorf("both IKE SAs were not established within the poll: %w", saErr)
		}
		doc, err := showMTUResult(ctx, plugin, "show mtu")
		if err != nil {
			return err
		}
		rows, err := showMTURows(doc, "measurements")
		if err != nil {
			return err
		}
		measured := 0
		for _, row := range rows {
			if row["target"] != showMTUFarAddr && row["target"] != showMTUSecondPeer {
				continue
			}
			if err := showMTUIKEProbeRow(row); err != nil {
				return fmt.Errorf("%w; row %v; notes %v", err, row, doc["notes"])
			}
			measured++
		}
		if measured != 2 {
			return fmt.Errorf("%d peer measurements, want 2: %v", measured, rows)
		}
		tunnels, err := showMTURows(doc, "tunnels")
		if err != nil {
			return err
		}
		if len(tunnels) != 2 {
			return fmt.Errorf("%d tunnel rows, want 2: %v", len(tunnels), tunnels)
		}
		for _, row := range tunnels {
			if row["sized"] != true {
				return fmt.Errorf("tunnel %v is not sized: %v", row["peer"], row)
			}
			if got := number13(row["path-mtu"]); got != showMTUClampMTU {
				return fmt.Errorf("tunnel %v path-mtu %d, want the clamp %d", row["peer"], got, showMTUClampMTU)
			}
		}
		if !showMTUHasNote(doc, "IKE confirmed the path down to 1388 octets") {
			return fmt.Errorf("no caution note names the CBC grid rounding as a confirmation: %v", doc["notes"])
		}
		if err := showMTUSAsEstablished(ctx, plugin, "site-b", "site-c"); err != nil {
			return err
		}
		fmt.Fprintln(os.Stderr, "OK: both tunnels keep the 1400 clamp, confirmed by the padded IKE exchange down to the 1388 grid point, and both SAs are still established")
		return nil
	})
}

func showMTUIKEProbeRow(row map[string]any) error {
	if row["prober"] != "ike" {
		return fmt.Errorf("target %v prober %v, want ike: %v", row["target"], row["prober"], row)
	}
	if declined, present := row["ike-declined"]; present {
		return fmt.Errorf("target %v: the IKE prober declined: %v", row["target"], declined)
	}
	// The IKE SA is aes256-cbc, so the padded exchange asked at the 1400 clamp
	// is sent at the 1388 grid point below it: a fit there confirms the clamp
	// down to 1388 and refutes nothing, so the clamp stays the figure.
	if got := number13(row["path-mtu"]); got != showMTUClampMTU {
		return fmt.Errorf("target %v path-mtu %d, want the ICMP clamp %d", row["target"], got, showMTUClampMTU)
	}
	if got := number13(row["ike-confirmed"]); got != showMTUIKEConfirmed {
		return fmt.Errorf("target %v ike-confirmed %d, want the grid point %d", row["target"], got, showMTUIKEConfirmed)
	}
	if got := number13(row["exchanges"]); got < 1 {
		return fmt.Errorf("target %v exchanges %d, want at least 1", row["target"], got)
	}
	caveats, ok := row["caveats"].([]any)
	if !ok {
		return fmt.Errorf("target %v caveats is not a list: %v", row["target"], row["caveats"])
	}
	joined := fmt.Sprint(caveats)
	if !strings.Contains(joined, "UDP/500") {
		return fmt.Errorf("target %v caveats %v lack the different-path caveat", row["target"], caveats)
	}
	if strings.Contains(joined, "ICMP echo") {
		return fmt.Errorf("target %v caveats %v carry the ICMP optimism caveat on an IKE-measured row", row["target"], caveats)
	}
	return nil
}

// showMTUSAsEstablished refuses unless `show vpn ipsec peer name <peer>` lists
// an established IKE SA for every named peer.
func showMTUSAsEstablished(ctx context.Context, plugin *sdk.Plugin, peers ...string) error {
	for _, peer := range peers {
		command := "show vpn ipsec peer name " + peer
		result := command13(ctx, plugin, command)
		if err := requireStatus13(command, result, statusDone); err != nil {
			return err
		}
		var doc map[string]any
		if err := decodeJSON13(result.raw, &doc); err != nil {
			return fmt.Errorf("%s: decode: %w", command, err)
		}
		records, err := showMTURows(doc, "ike-sas")
		if err != nil {
			return fmt.Errorf("%s: %w", command, err)
		}
		found := false
		for _, record := range records {
			if record["state"] == "established" {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("%s lists no established IKE SA: %v", command, records)
		}
	}
	return nil
}

func showMTUOversizedRow(row map[string]any) error {
	if row[showMTUKeyVerdict] != "oversized" {
		return fmt.Errorf("peer %v verdict %v, want oversized: %v", row["peer"], row[showMTUKeyVerdict], row)
	}
	// The tunnel is measured over its live IKE SA (spec ike-padded-path-probe);
	// the padded exchange confirms the 1400 clamp and the clamp stays the
	// figure the tunnel is sized to.
	if got := number13(row["path-mtu"]); got != showMTUClampMTU {
		return fmt.Errorf("peer %v path-mtu %d, want the clamp %d", row["peer"], got, showMTUClampMTU)
	}
	if row["assumed"] != false {
		return fmt.Errorf("peer %v path is assumed; each peer answered", row["peer"])
	}
	if got := number13(row["ceiling"]); got != showMTUGCMCeiling {
		return fmt.Errorf("peer %v ceiling %d, want %d", row["peer"], got, showMTUGCMCeiling)
	}
	if got := number13(row["recommended"]); got != showMTURecommended {
		return fmt.Errorf("peer %v recommended %d, want %d", row["peer"], got, showMTURecommended)
	}
	if got := number13(row["octets"]); got != showMTUXfrmMTU-showMTUGCMCeiling {
		return fmt.Errorf("peer %v excess %d, want %d", row["peer"], got, showMTUXfrmMTU-showMTUGCMCeiling)
	}
	return nil
}
