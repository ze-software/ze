package fixture

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func init() {
	observers := map[string]ObserverScenario{
		"plugin/fib-rib-event":                             fibRIBEvent07,
		"plugin/fib-srv6-kernel":                           fibSRv6Kernel07,
		"plugin/fib-sysrib":                                fibSysRIB07,
		"plugin/fib-table":                                 fibTable07,
		"plugin/filter-family-export-flowspec":             filterFamilyExportFlowSpec07,
		"plugin/filter-family-import-remove":               waitForPeerUpdate07(),
		"plugin/filter-family-import-teardown":             waitForPeerUpdate07(),
		"plugin/filter-irr-fail":                           filterIRRFail07,
		"plugin/filter-irr-update":                         filterIRRUpdate07,
		"plugin/filter-irr":                                waitForPeerUpdate07(),
		"plugin/firewall-global-options":                   firewallGlobalOptions07,
		"plugin/firewall-irr-cold-cache-recovers":          firewallIRRColdCache07,
		"plugin/firewall-irr-commit":                       firewallIRRCommit07,
		"plugin/firewall-irr-empty-answer-keeps-last-good": firewallIRREmptyAnswer07,
		"plugin/firewall-irr-iface-commit":                 firewallIRRIfaceCommit07,
		"plugin/firewall-irr-iface-no-blackhole":           firewallIRRIfaceNoBlackhole07,
		"plugin/firewall-irr-iface-reject":                 observerMessage07("OK: reached observer (verify rejection tested via stderr pattern)"),
		"plugin/firewall-irr-refresh":                      firewallIRRRefresh07,
		"plugin/firewall-irr-reject":                       observerMessage07("OK: reached observer (verify rejection tested via stderr pattern)"),
		"plugin/firewall-irr-show":                         firewallIRRShow07,
		"plugin/firewall-irr-table-term-commit":            firewallIRRTableTermCommit07,
		"plugin/firewall-irr-table-term-uncached-reject":   firewallIRRTableTermUncached07,
		"plugin/firewall-irr-update":                       firewallIRRUpdate07,
		"plugin/flowspec-announce":                         flowSpecAnnounce07,
		"plugin/flowspec-fw-withdraw":                      flowSpecWithdraw07,
	}
	for name, scenario := range observers {
		n, s := name, scenario
		Register(n, func(ctx context.Context, _ []string) error {
			return Observe(ctx, n, sdk.Registration{}, s)
		})
	}
	Register("plugin/fib-vpp-coexist-with-fib-kernel", fibVPPCoexist07)
	Register("plugin/fib-vpp-plugin-load", fibVPPLoad07)
	Register("plugin/firewall-metrics-registered", firewallMetrics07)
	Register("plugin/flowspec-fw-legacy-table-removed", flowSpecLegacyTable07)
	Register("plugin/flowspec-fw-protocol-sctp", flowSpecSCTP07)
	Register("plugin/flowspec-fw-untranslatable-keeps-others", flowSpecUntranslatable07)
	Register("plugin/flowspec-fw-withdraw-removes-table", flowSpecWithdrawTable07)
	Register("plugin/firewall-irr-table-term-commit/reload", func(ctx context.Context, _ []string) error {
		return reloadFirewallIRR07(ctx, "observer.fetched")
	})
	Register("plugin/firewall-irr-table-term-uncached-reject/reload", func(ctx context.Context, _ []string) error {
		return reloadFirewallIRR07(ctx, "observer.ready")
	})
	Register("plugin/flowspec-fw-legacy-table-removed/seed", flowSpecLegacySeed07)
}

type commandResult07 struct {
	status string
	data   any
	err    error
}

func command07(ctx context.Context, p *sdk.Plugin, command string) commandResult07 {
	var data any
	status, err := Dispatch(ctx, p, command, &data)
	return commandResult07{status: status, data: data, err: err}
}

func until07(ctx context.Context, p *sdk.Plugin, command string, attempts int, delay time.Duration, accept func(commandResult07) bool) commandResult07 {
	var result commandResult07
	Poll(ctx, attempts, delay, func() bool {
		result = command07(ctx, p, command)
		return accept(result)
	})
	return result
}

func done07(ctx context.Context, p *sdk.Plugin, command string) commandResult07 {
	return until07(ctx, p, command, 20, 250*time.Millisecond, func(r commandResult07) bool { return r.status == "done" })
}

func object07(value any) map[string]any {
	if value == nil {
		return nil
	}
	if object, ok := value.(map[string]any); ok {
		return object
	}
	return nil
}

func array07(value any) []any {
	if rows, ok := value.([]any); ok {
		return rows
	}
	return nil
}

func number07(value any) int {
	switch value := value.(type) {
	case float64:
		return int(value)
	case int:
		return value
	case json.Number:
		n, _ := value.Int64()
		return int(n)
	default:
		return 0
	}
}

func text07(value any) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func waitEOR07(ctx context.Context, p *sdk.Plugin, expected int) error {
	result := until07(ctx, p, "show bgp peer * detail", 40, 250*time.Millisecond, func(r commandResult07) bool {
		peers := object07(object07(r.data)["peers"])
		ready := 0
		for _, value := range peers {
			if number07(object07(value)["eor-sent"]) >= 1 {
				ready++
			}
		}
		return r.status == "done" && ready >= expected
	})
	if result.status != "done" {
		if result.err != nil {
			return fmt.Errorf("initial-sync End-of-RIB did not reach %d peer(s): status=%s error=%w", expected, result.status, result.err)
		}
		return fmt.Errorf("initial-sync End-of-RIB did not reach %d peer(s): status=%s error=<nil>", expected, result.status)
	}
	peers := object07(object07(result.data)["peers"])
	ready := 0
	for _, value := range peers {
		if number07(object07(value)["eor-sent"]) >= 1 {
			ready++
		}
	}
	if ready < expected {
		return fmt.Errorf("initial-sync End-of-RIB reached %d peer(s), want %d", ready, expected)
	}
	return nil
}

func peerRow07(ctx context.Context, p *sdk.Plugin, address string) (map[string]any, error) {
	result := command07(ctx, p, "show bgp peer "+address+" detail")
	if result.status != "done" {
		if result.err != nil {
			return nil, fmt.Errorf("show peer %s: status=%s error=%w", address, result.status, result.err)
		}
		return nil, fmt.Errorf("show peer %s: status=%s error=<nil>", address, result.status)
	}
	row := object07(object07(result.data)["peers"])[address]
	if object07(row) == nil {
		return nil, fmt.Errorf("show peer %s returned no peer row", address)
	}
	return object07(row), nil
}

func quiesce07(ctx context.Context, p *sdk.Plugin) error {
	result := command07(ctx, p, "request quiesce")
	if result.status != "done" {
		if result.err != nil {
			return fmt.Errorf("quiesce: status=%s error=%w", result.status, result.err)
		}
		return fmt.Errorf("quiesce: status=%s error=<nil>", result.status)
	}
	return nil
}

func observerMessage07(message string) ObserverScenario {
	return func(_ context.Context, _ *sdk.Plugin) error {
		fmt.Fprintln(os.Stderr, message)
		return nil
	}
}

func waitForPeerEOR07(expected int) ObserverScenario {
	return func(ctx context.Context, p *sdk.Plugin) error { return waitEOR07(ctx, p, expected) }
}
func waitForPeerUpdate07() ObserverScenario {
	return func(ctx context.Context, p *sdk.Plugin) error {
		processed := func(value any) bool {
			row := object07(value)
			return number07(row["connections-established"]) >= 1 &&
				(number07(row["updates-received"]) >= 1 || number07(row["connections-dropped"]) >= 1)
		}
		result := until07(ctx, p, "show bgp peer * detail", 40, 125*time.Millisecond, func(result commandResult07) bool {
			for _, value := range object07(object07(result.data)["peers"]) {
				if processed(value) {
					return result.status == "done"
				}
			}
			return false
		})
		for _, value := range object07(object07(result.data)["peers"]) {
			if processed(value) {
				return nil
			}
		}
		return fmt.Errorf("peer UPDATE was not processed: status=%s data=%s error=%v", result.status, text07(result.data), result.err)
	}
}

func waitDaemon07(ctx context.Context) (int, error) {
	var pid int
	if !Poll(ctx, 400, 50*time.Millisecond, func() bool {
		if _, err := os.Stat("daemon.ready"); err != nil {
			return false
		}
		raw, err := os.ReadFile("daemon.pid")
		if err != nil {
			return false
		}
		pid, err = strconv.Atoi(strings.TrimSpace(string(raw)))
		return err == nil
	}) {
		return 0, errors.New("daemon.pid/ready never appeared")
	}
	return pid, nil
}

func terminate07(pid int) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return process.Signal(syscall.SIGTERM)
}

func fetch07(ctx context.Context, port string) (string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:"+port+"/metrics", nil)
	if err != nil {
		return "", err
	}
	client := &http.Client{Timeout: 2 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	return string(body), err
}
