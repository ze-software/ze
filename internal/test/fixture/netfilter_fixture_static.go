package fixture

import (
	"context"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func staticSetup(iface string, addresses ...string) Driver {
	return func(ctx context.Context, _ []string) error {
		commandIgnore(ctx, "ip", "link", "add", iface, "type", "dummy")
		for _, address := range addresses {
			commandIgnore(ctx, "ip", "addr", "add", address, "dev", iface)
		}
		commandIgnore(ctx, "ip", "link", "set", iface, "up")
		return nil
	}
}

func routesOutput(ctx context.Context) string {
	out, _ := netfilterCommandOutput(ctx, "ip", "route", "show", "table", "all")
	return out
}

func staticFinish(ctx context.Context, pid int) error {
	if err := signalProcess(pid, syscall.SIGTERM); err != nil {
		return err
	}
	waitDead(ctx, pid)
	return nil
}

func staticBootApply(ctx context.Context, _ []string) error {
	pid, err := waitDaemon(ctx, 200)
	if err != nil {
		return err
	}
	var routes string
	if !Poll(ctx, 100, 100*time.Millisecond, func() bool {
		routes = routesOutput(ctx)
		return strings.Contains(routes, "10.0.0.0/8") && strings.Contains(routes, "default")
	}) {
		return fmt.Errorf("static routes not programmed into kernel:\n%s", routes)
	}
	fmt.Print("10.0.0.0/8\n0.0.0.0/0\n")
	return staticFinish(ctx, pid)
}

func staticInterfaceNoBackend(ctx context.Context, _ []string) error {
	pid, err := waitDaemon(ctx, 200)
	if err != nil {
		return err
	}
	routes := routesOutput(ctx)
	if strings.Contains(routes, "tun404") {
		return fmt.Errorf("route via unresolvable tun404 was programmed, expected skipped:\n%s", routes)
	}
	fmt.Println("DAEMON-UP-WITH-ROUTE-SKIPPED")
	return staticFinish(ctx, pid)
}

func staticPerRouteIsolation(ctx context.Context, _ []string) error {
	pid, err := waitDaemon(ctx, 200)
	if err != nil {
		return err
	}
	var routes string
	if !Poll(ctx, 100, 100*time.Millisecond, func() bool {
		routes = routesOutput(ctx)
		return strings.Contains(routes, "10.0.0.0/8")
	}) {
		return fmt.Errorf("good static route not programmed into kernel (per-route isolation broken?):\n%s", routes)
	}
	if strings.Contains(routes, "172.16.0.0/12") {
		return fmt.Errorf("unresolvable route 172.16.0.0/12 was programmed, expected skipped:\n%s", routes)
	}
	fmt.Print("GOOD-ROUTE-PROGRAMMED 10.0.0.0/8\nBAD-ROUTE-SKIPPED 172.16.0.0/12\n")
	return staticFinish(ctx, pid)
}

const staticAddConfig = `static {
    table default {
        route 10.0.0.0/8 {
            next {
                hop 192.168.1.1 { }
            }
        }
        route 172.16.0.0/12 {
            next {
                hop 10.0.0.1 { }
            }
        }
    }
}
`

const staticRemoveConfig = `static {
    table default {
        route 10.0.0.0/8 {
            next {
                hop 192.168.1.1 { }
            }
        }
    }
}
`

func staticReloadAdd(ctx context.Context, _ []string) error {
	pid, err := waitDaemon(ctx, 200)
	if err != nil {
		return err
	}
	if !Poll(ctx, 100, 100*time.Millisecond, func() bool { return strings.Contains(routesOutput(ctx), "10.0.0.0/8") }) {
		return fmt.Errorf("initial static route not programmed before reload")
	}
	if err := os.WriteFile("ze-bgp.conf", []byte(staticAddConfig), 0o600); err != nil {
		return err
	}
	if err := signalProcess(pid, syscall.SIGHUP); err != nil {
		return err
	}
	var routes string
	if !Poll(ctx, 100, 100*time.Millisecond, func() bool {
		routes = routesOutput(ctx)
		return strings.Contains(routes, "172.16.0.0/12")
	}) {
		return fmt.Errorf("reload did not add 172.16.0.0/12 to kernel:\n%s", routes)
	}
	fmt.Println("172.16.0.0/12")
	return staticFinish(ctx, pid)
}

func staticReloadEmpty(ctx context.Context, _ []string) error {
	pid, err := waitDaemon(ctx, 200)
	if err != nil {
		return err
	}
	if !Poll(ctx, 100, 100*time.Millisecond, func() bool {
		routes := routesOutput(ctx)
		return strings.Contains(routes, "10.10.0.0/16") && strings.Contains(routes, "172.20.0.0/14")
	}) {
		return fmt.Errorf("initial static routes not programmed before reload")
	}
	if err := os.WriteFile("ze-bgp.conf", []byte("static {\n}\n"), 0o600); err != nil {
		return err
	}
	if err := signalProcess(pid, syscall.SIGHUP); err != nil {
		return err
	}
	var routes string
	if !Poll(ctx, 100, 100*time.Millisecond, func() bool {
		routes = routesOutput(ctx)
		return !strings.Contains(routes, "10.10.0.0/16") && !strings.Contains(routes, "172.20.0.0/14")
	}) {
		return fmt.Errorf("emptying the static section left routes in the kernel:\n%s", routes)
	}
	fmt.Println("withdrawn")
	return staticFinish(ctx, pid)
}

func staticReloadRemove(ctx context.Context, _ []string) error {
	pid, err := waitDaemon(ctx, 200)
	if err != nil {
		return err
	}
	if !Poll(ctx, 100, 100*time.Millisecond, func() bool {
		routes := routesOutput(ctx)
		return strings.Contains(routes, "10.0.0.0/8") && strings.Contains(routes, "172.16.0.0/12")
	}) {
		return fmt.Errorf("initial static routes not programmed before reload")
	}
	if err := os.WriteFile("ze-bgp.conf", []byte(staticRemoveConfig), 0o600); err != nil {
		return err
	}
	if err := signalProcess(pid, syscall.SIGHUP); err != nil {
		return err
	}
	var routes string
	if !Poll(ctx, 100, 100*time.Millisecond, func() bool {
		routes = routesOutput(ctx)
		return strings.Contains(routes, "10.0.0.0/8") && !strings.Contains(routes, "172.16.0.0/12")
	}) {
		return fmt.Errorf("reload did not remove 172.16.0.0/12 from kernel:\n%s", routes)
	}
	fmt.Println("10.0.0.0/8")
	return staticFinish(ctx, pid)
}

type staticNextHop struct {
	Address   string `json:"address"`
	Interface string `json:"interface"`
	Weight    uint32 `json:"weight"`
}

type staticRow struct {
	Prefix   string          `json:"prefix"`
	Table    uint32          `json:"table"`
	Metric   uint32          `json:"metric"`
	Tag      uint32          `json:"tag"`
	Action   string          `json:"action"`
	NextHops []staticNextHop `json:"next-hops"`
}

func staticShow(ctx context.Context, _ []string) error {
	return Observe(ctx, "static-show", sdk.Registration{}, func(ctx context.Context, p *sdk.Plugin) error {
		var rows []staticRow
		status, err := Dispatch(ctx, p, "show static", &rows)
		if err != nil || status != statusDone {
			return fmt.Errorf("show static: status=%s: %w", status, err)
		}
		if len(rows) == 0 {
			return fmt.Errorf("show static: expected a non-empty list")
		}
		byPrefix := make(map[string]staticRow, len(rows))
		for _, row := range rows {
			byPrefix[row.Prefix] = row
		}
		def, ok := byPrefix["0.0.0.0/0"]
		if !ok {
			return fmt.Errorf("show static: 0.0.0.0/0 missing")
		}
		if def.Metric != 100 {
			return fmt.Errorf("0.0.0.0/0 metric = %d, want 100", def.Metric)
		}
		if def.Tag != 42 {
			return fmt.Errorf("0.0.0.0/0 tag = %d, want 42", def.Tag)
		}
		weights := make(map[string]uint32, len(def.NextHops))
		for _, hop := range def.NextHops {
			weights[hop.Address] = hop.Weight
		}
		if len(weights) != 2 || weights["10.0.0.1"] != 3 || weights["10.0.0.2"] != 1 {
			return fmt.Errorf("0.0.0.0/0 next-hop weights = %v, want map[10.0.0.1:3 10.0.0.2:1]", weights)
		}
		blackhole, ok := byPrefix["192.0.2.0/24"]
		if !ok || blackhole.Action != "blackhole" {
			return fmt.Errorf("192.0.2.0/24 action = %q, want blackhole", blackhole.Action)
		}
		fmt.Fprintln(os.Stderr, "OK: show static reports both routes with weights, metric and tag")
		return nil
	})
}

func staticTableInterfaceSetup(ctx context.Context, _ []string) error {
	if err := staticSetup("zens0", "192.168.1.2/24", "10.0.0.100/8")(ctx, nil); err != nil {
		return err
	}
	return staticSetup("tun100")(ctx, nil)
}

func staticTableInterface(ctx context.Context, _ []string) error {
	return Observe(ctx, "static-show", sdk.Registration{}, func(ctx context.Context, p *sdk.Plugin) error {
		var rows []staticRow
		status, err := Dispatch(ctx, p, "show static", &rows)
		if err != nil || status != statusDone {
			return fmt.Errorf("show static: status=%s: %w", status, err)
		}
		if len(rows) < 3 {
			return fmt.Errorf("expected at least 3 routes, got %d", len(rows))
		}
		prefixes := make(map[string]bool)
		var defaultTables []uint32
		var lns *staticRow
		for i := range rows {
			row := &rows[i]
			prefixes[row.Prefix] = true
			if row.Prefix == "0.0.0.0/0" {
				defaultTables = append(defaultTables, row.Table)
				if row.Table == 100 {
					lns = row
				}
			}
		}
		if !prefixes["0.0.0.0/0"] || !prefixes["10.0.0.0/8"] {
			return fmt.Errorf("show static: required prefixes missing")
		}
		if len(defaultTables) < 2 || lns == nil {
			return fmt.Errorf("0.0.0.0/0 should appear in default and table 100, got %v", defaultTables)
		}
		for _, hop := range lns.NextHops {
			if hop.Interface == "tun100" {
				fmt.Fprintln(os.Stderr, "OK: show static reports 3 routes across default and lns, interface-only next-hop resolved")
				return nil
			}
		}
		return fmt.Errorf("lns 0.0.0.0/0 should have an interface-only next-hop on tun100, got %v", lns.NextHops)
	})
}
