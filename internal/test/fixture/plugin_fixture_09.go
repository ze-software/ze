package fixture

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	Register("plugin/interface-type-show", interfaceTypeShow09)
	Register("plugin/ipv4-announce-withdraw", announceWithdraw09("ipv4/unicast", "origin igp local-preference 200 nhop 10.0.1.254 nlri ipv4/unicast add 10.0.1.0/24", "nlri ipv4/unicast del 10.0.1.0/24"))
	Register("plugin/ipv6-announce-withdraw", announceWithdraw09("ipv6/unicast", "origin igp local-preference 200 nhop 2001::11 nlri ipv6/unicast add fc00:1::/64", "nlri ipv6/unicast del fc00:1::/64"))
	Register("plugin/l2tp-config-show", l2tpConfigShow09)
	Register("plugin/l2tp-empty-show", l2tpEmptyShow09)
	Register("plugin/l2tp-health-show", l2tpHealthShow09)
	Register("plugin/l2tp-history-show", l2tpHistoryShow09)
	Register("plugin/l2tp-session-detail-show", l2tpSessionDetailShow09)
	Register("plugin/l2tp-sessions-show", l2tpSessionsShow09)
	Register("plugin/l2tp-statistics-show", l2tpStatisticsShow09)
	Register("plugin/l2tp-tunnel-detail-show", l2tpTunnelDetailShow09)
	Register("plugin/l2tp-tunnels-show", l2tpTunnelsShow09)
	Register("plugin/l2tp-history-show-peer", l2tpPeer09(true))
	Register("plugin/l2tp-session-detail-show-peer", l2tpPeer09(true))
	Register("plugin/l2tp-sessions-show-peer", l2tpPeer09(true))
	Register("plugin/l2tp-statistics-show-peer", l2tpPeer09(false))
	Register("plugin/l2tp-tunnel-detail-show-peer", l2tpPeer09(false))
	Register("plugin/l2tp-tunnels-show-peer", l2tpPeer09(false))
	Register("plugin/lg-csv-download", lgCSVDownload09)
	Register("plugin/lg-paginate", lgInjectRoutes09("lg-paginate"))
	Register("plugin/lg-ui-pages", lgInjectRoutes09("lg-ui-pages"))
	Register("plugin/lg-tls-default-on", lgTLSDefaultOn09)
	Register("plugin/lg-token-gate", lgTokenGate09)
	Register("plugin/llgr-egress-state-unloaded", llgrReadvertise09("llgr-egress-state-unloaded"))
	Register("plugin/llgr-readvertise-multipeer", llgrReadvertise09("llgr-readvertise-multipeer"))
	Register("plugin/local-pref-strip-ebgp", localPrefStrip09)
	Register("plugin/mcp-tools-list-hidden-command", mcpHiddenCommand09)
	Register("plugin/med-ibgp-post-selection-removal", medRawDrop09)
	Register("plugin/med-locally-set-reaches-peer", medLocallySet09)
	Register("plugin/med-not-propagated-across-as", rsObserver09("med-not-propagated-across-as", 3, "10.0.0.0/24"))
	Register("plugin/med-removal-before-decision", medRemovalBeforeDecision09)
	Register("plugin/med-removal-configured", rsObserver09("med-removal-configured", 2, "10.0.0.0/24"))
	Register("plugin/med-removal-export-refused", rsObserver09("med-removal-export-refused", 2, "10.0.0.0/24"))
	Register("plugin/metrics-flap-notification-duration", metricsFlap09)
	Register("plugin/metrics-name-show", metricsNameShow09)
	Register("plugin/metrics-session-lifecycle", metricsLifecycle09)
	Register("plugin/modify-accumulator-per-peer-isolation", accumulatorIsolation09)
}

type commandResult09 struct {
	status string
	data   any
	text   string
	err    error
}

func dispatch09(ctx context.Context, p *sdk.Plugin, command string) commandResult09 {
	status, raw, err := p.DispatchCommand(ctx, command)
	r := commandResult09{status: status, err: err}
	if err != nil {
		r.text = err.Error()
		return r
	}
	if len(raw) == 0 {
		return r
	}
	for range 4 {
		var decoded any
		if err := json.Unmarshal(raw, &decoded); err != nil {
			r.err = fmt.Errorf("decode %q: %w", command, err)
			r.text = string(raw)
			return r
		}
		text, encoded := decoded.(string)
		if !encoded {
			r.data = decoded
			encodedData, _ := json.Marshal(decoded)
			r.text = string(encodedData)
			return r
		}
		r.data = text
		r.text = text
		if !json.Valid([]byte(text)) {
			return r
		}
		raw = json.RawMessage(text)
	}
	r.err = fmt.Errorf("decode %q: result remained JSON text after 4 layers", command)
	return r
}

func done09(r commandResult09) bool { return r.status == "done" && r.err == nil }

func map09(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func list09(v any) []any {
	rows, _ := v.([]any)
	return rows
}

func number09(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case json.Number:
		i, _ := n.Int64()
		return int(i)
	}
	return 0
}

func observe09(ctx context.Context, name string, scenario ObserverScenario) error {
	return Observe(ctx, name, sdk.Registration{}, scenario)
}
func passiveObserve09(ctx context.Context, name string, registration sdk.Registration, scenario ObserverScenario) error {
	plugin, err := newObserver(name)
	if err != nil {
		return fmt.Errorf("connect observer %s: %w", name, err)
	}
	defer plugin.Close()
	result := make(chan error, 1)
	plugin.OnAllPluginsReady(func() error {
		go func() {
			scenarioErr := invokeScenario(ctx, plugin, scenario)
			result <- scenarioErr
			if scenarioErr != nil {
				_ = plugin.Close()
			}
		}()
		return nil
	})
	runErr := plugin.Run(ctx, registration)
	select {
	case scenarioErr := <-result:
		return errors.Join(scenarioErr, runErr)
	default:
		return runErr
	}
}

func announceWithdraw09(family, add, del string) Driver {
	return func(ctx context.Context, _ []string) error {
		return passiveObserve09(ctx, "announce-routes", sdk.Registration{}, func(ctx context.Context, p *sdk.Plugin) error {
			for _, command := range []string{add, del} {
				announced, withdrawn, err := p.UpdateRoute(ctx, "*", "update text "+command)
				if err != nil {
					return fmt.Errorf("%s update %q: %w", family, command, err)
				}
				if announced+withdrawn == 0 {
					return fmt.Errorf("%s update %q acknowledged no routes", family, command)
				}
				if quiesced := dispatch09(ctx, p, "request quiesce"); !done09(quiesced) {
					return fmt.Errorf("%s update %q quiesce: status=%s data=%s", family, command, quiesced.status, quiesced.text)
				}
			}
			return nil
		})
	}
}

func interfaceTypeShow09(ctx context.Context, _ []string) error {
	return observe09(ctx, "interface-type-show-test", func(ctx context.Context, p *sdk.Plugin) error {
		all := dispatch09(ctx, p, "show interface")
		rows := list09(all.data)
		if !done09(all) || len(rows) == 0 {
			return fmt.Errorf("interface-all: expected a non-empty list, status=%s data=%s", all.status, all.text)
		}
		wanted := ""
		expected := make([]string, 0)
		for _, value := range rows {
			row := map09(value)
			typeName, _ := row["type"].(string)
			if wanted == "" && typeName != "" {
				wanted = typeName
			}
		}
		if wanted == "" {
			return fmt.Errorf("interface-all: no interface carries a type: %s", all.text)
		}
		for _, value := range rows {
			row := map09(value)
			if typeName, _ := row["type"].(string); strings.EqualFold(typeName, wanted) {
				if name, ok := row["name"].(string); ok {
					expected = append(expected, name)
				}
			}
		}
		sort.Strings(expected)
		filtered := dispatch09(ctx, p, "show interface type "+wanted)
		wrapper := map09(filtered.data)
		filteredRows, ok := wrapper["interfaces"].([]any)
		if !done09(filtered) || !ok {
			return fmt.Errorf("interface-type: expected interfaces wrapper, status=%s data=%s", filtered.status, filtered.text)
		}
		got := make([]string, 0, len(filteredRows))
		for _, value := range filteredRows {
			row := map09(value)
			name, _ := row["name"].(string)
			typeName, _ := row["type"].(string)
			got = append(got, name)
			if !strings.EqualFold(typeName, wanted) {
				return fmt.Errorf("interface-type %s: row %s has type=%q", wanted, name, typeName)
			}
		}
		sort.Strings(got)
		if strings.Join(got, "\x00") != strings.Join(expected, "\x00") {
			return fmt.Errorf("interface-type %s: filtered set %v != expected %v", wanted, got, expected)
		}
		unknown := "zz-not-an-interface-type"
		rejected := dispatch09(ctx, p, "show interface type "+unknown)
		if rejected.status != "error" || rejected.err == nil {
			return fmt.Errorf("interface-type %s: expected error, status=%s data=%s", unknown, rejected.status, rejected.text)
		}
		if !strings.Contains(rejected.text, "unknown interface type") || !strings.Contains(strings.ToLower(rejected.text), strings.ToLower(wanted)) {
			return fmt.Errorf("interface-type %s: unexpected rejection: %s", unknown, rejected.text)
		}
		fmt.Fprintf(os.Stderr, "OK: show interface type %s -> %v; %s rejected\n", wanted, got, unknown)
		return nil
	})
}

func injectRoute09(ctx context.Context, p *sdk.Plugin, peer, prefix string) error {
	command := fmt.Sprintf("request bgp rib inject %s ipv4/unicast %s origin igp nhop %s aspath 2914,65100", peer, prefix, peer)
	r := dispatch09(ctx, p, command)
	if !done09(r) {
		return fmt.Errorf("%s: status=%s data=%s", command, r.status, r.text)
	}
	return nil
}

func lgInjectRoutes09(name string) Driver {
	return func(ctx context.Context, _ []string) error {
		return passiveObserve09(ctx, name, sdk.Registration{}, func(ctx context.Context, p *sdk.Plugin) error {
			for i := range 15 {
				if err := injectRoute09(ctx, p, "10.0.1.1", fmt.Sprintf("10.20.%d.0/24", i)); err != nil {
					return err
				}
			}
			fmt.Fprintln(os.Stderr, "OK: 15 routes injected")
			return nil
		})
	}
}

func lgCSVDownload09(ctx context.Context, _ []string) error {
	return observe09(ctx, "lg-csv", func(ctx context.Context, p *sdk.Plugin) error {
		const peer = "10.0.1.1"
		if err := injectRoute09(ctx, p, peer, "10.20.0.0/24"); err != nil {
			return err
		}
		port := os.Getenv("ZE_LG_PORT")
		if port == "" {
			return fmt.Errorf("csv: ZE_LG_PORT not set in plugin environment")
		}
		url := "http://127.0.0.1:" + port + "/lg/peer/" + peer + "/download"
		client := &http.Client{Timeout: 3 * time.Second, Transport: &http.Transport{Proxy: nil}}
		var resp *http.Response
		var lastErr error
		lastStatus := 0
		if !Poll(ctx, 75, 200*time.Millisecond, func() bool {
			candidate, err := client.Get(url)
			lastErr = err
			if err != nil {
				return false
			}
			lastStatus = candidate.StatusCode
			if candidate.StatusCode != http.StatusOK {
				_ = candidate.Body.Close()
				return false
			}
			resp = candidate
			return true
		}) {
			if lastStatus != 0 {
				return fmt.Errorf("csv: expected HTTP 200, got %d", lastStatus)
			}
			return fmt.Errorf("csv: LG download never responded at %s: %w", url, lastErr)
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("csv: read body: %w", err)
		}
		cd, ct := resp.Header.Get("Content-Disposition"), resp.Header.Get("Content-Type")
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("csv: expected HTTP 200, got %d", resp.StatusCode)
		}
		if !strings.Contains(cd, "routes-"+peer+".csv.gz") {
			return fmt.Errorf("csv: Content-Disposition filename mismatch: %q", cd)
		}
		if ct != "application/gzip" {
			return fmt.Errorf("csv: Content-Type expected application/gzip, got %q", ct)
		}
		zr, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			return fmt.Errorf("csv: body is not valid gzip: %w", err)
		}
		textBytes, err := io.ReadAll(zr)
		_ = zr.Close()
		if err != nil {
			return fmt.Errorf("csv: decompress body: %w", err)
		}
		text := string(textBytes)
		if !strings.Contains(text, "prefix,next-hop,as-path,origin,local-pref,med") || !strings.Contains(text, "10.20.0.0/24") {
			return fmt.Errorf("csv: gzip body missing header or injected route: %.200q", text)
		}
		fmt.Fprintf(os.Stderr, "OK: csv download status=%d cd=%q ct=%q\n", resp.StatusCode, cd, ct)
		return nil
	})
}

func uint16Arg09(args []string, index int, fallback uint16) (uint16, error) {
	if len(args) <= index {
		return fallback, nil
	}
	n, err := strconv.ParseUint(args[index], 0, 16)
	return uint16(n), err
}
