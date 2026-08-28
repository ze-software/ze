package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func init() {
	registerTableSnapshot("firewall/copp-bgp", "inet", "ze_copp")
	Register("firewall/copp-trusted", coppTrusted)
	Register("firewall/copp-withdraw", coppWithdraw)
	Register("firewall/ddos-local-withdraw", ddosLocalWithdraw)
	registerTableSnapshot("firewall/firewall-boot-apply", "inet", "ze_fw10_001")
	registerTableSnapshot("firewall/firewall-byte-rate-limit", "inet", "ze_fw10_010")
	Register("firewall/firewall-cli-show", firewallCLIShow)
	Register("firewall/firewall-coexistence-setup", firewallCoexistenceSetup)
	Register("firewall/firewall-coexistence", firewallCoexistence)
	registerTableSnapshot("firewall/firewall-icmp-type", "inet", "ze_fw8_012")
	registerTableSnapshot("firewall/firewall-iface-wildcard", "inet", "ze_fw8_013")
	Register("firewall/firewall-legacy-table-removed-with-no-config-seed", firewallLegacySeed)
	Register("firewall/firewall-legacy-table-removed-with-no-config", firewallLegacySweep)
	registerTableSnapshot("firewall/firewall-masquerade-flags", "inet", "ze_masqf")
	registerTableSnapshot("firewall/firewall-masquerade-ports", "inet", "ze_masqp")
	registerTableSnapshot("firewall/firewall-match-in-set-addr", "inet", "ze_fw10_005")
	registerTableSnapshot("firewall/firewall-match-in-set-port", "inet", "ze_fw10_008")
	registerTableSnapshot("firewall/firewall-nat-exclude", "ip", "ze_fw8_014")
	Register("firewall/firewall-reload", firewallReload)
	Register("firewall/firewall-set-element-timeout", firewallSetElementTimeout)
	registerTableSnapshot("firewall/firewall-setdscp-inet", "inet", "ze_fw10_007")
	registerTableSnapshot("firewall/firewall-snat-addr-range", "ip", "ze_fw10_011")
	Register("firewall/flush-crash", func(ctx context.Context, _ []string) error {
		return firewallPersist(ctx, "ze_fwcrash", syscall.SIGKILL, "CRASH")
	})
	Register("firewall/flush-persist", func(ctx context.Context, _ []string) error {
		return firewallPersist(ctx, "ze_fwpersist", syscall.SIGTERM, "SHUTDOWN")
	})

	Register("flow-export/collector-reload", func(ctx context.Context, _ []string) error {
		return reloadAndStop(ctx, 2500*time.Millisecond)
	})
	Register("flow-export/conntrack-config", waitForDaemonFixture)
	Register("flow-export/flow-export-show", flowExportShow)
	Register("flow-export/ipfix-export", ipfixReceiver)
	Register("flow-export/multi-collector-export", multiCollectorReceiver)
	Register("flow-export/netflow9-export", netflow9Receiver)
	Register("flow-export/sampling-config", waitForDaemonFixture)
	Register("flow-export/sflow-export", sflowReceiver)

	Register("policy/policy-boot-apply", policyBootApply)
	Register("policy/policy-next-hop", policyNextHop)
	Register("policy/policy-reload", policyReload)
	Register("policy/policy-set-table", policySetTable)
	Register("policy/policy-tcp-flags", policySingleTable)
	Register("policy/policy-tcp-mss", policySingleTable)

	Register("static/static-boot-apply-setup", staticSetup("zens0", "192.168.1.2/24", "10.0.0.100/24"))
	Register("static/static-boot-apply", staticBootApply)
	Register("static/static-interface-nexthop-no-backend", staticInterfaceNoBackend)
	Register("static/static-per-route-isolation-setup", staticSetup("zeiso0", "192.168.1.2/24"))
	Register("static/static-per-route-isolation", staticPerRouteIsolation)
	Register("static/static-reload-add-setup", staticSetup("zens0", "192.168.1.2/24", "10.0.0.100/24"))
	Register("static/static-reload-add", staticReloadAdd)
	Register("static/static-reload-empty-section-withdraws-setup", staticSetup("zens1", "192.168.1.3/24", "10.0.0.101/24"))
	Register("static/static-reload-empty-section-withdraws", staticReloadEmpty)
	Register("static/static-reload-remove-setup", staticSetup("zens0", "192.168.1.2/24", "10.0.0.100/24"))
	Register("static/static-reload-remove", staticReloadRemove)
	Register("static/static-show-setup", staticSetup("zens4", "10.0.0.100/24"))
	Register("static/static-show", staticShow)
	Register("static/static-table-interface-setup", staticTableInterfaceSetup)
	Register("static/static-table-interface", staticTableInterface)

	Register("traffic/traffic-boot-apply", sleepFixture(2500*time.Millisecond))
	Register("traffic/traffic-boot-qdisc-tc", sleepFixture(2500*time.Millisecond))
	Register("traffic/traffic-cs6-priority-config", sleepFixture(2500*time.Millisecond))
	Register("traffic/traffic-reload-apply", func(ctx context.Context, _ []string) error {
		return reloadAndStop(ctx, 2*time.Second)
	})
	Register("traffic/traffic-reload-qdisc-tc", trafficReloadQdisc)
	Register("traffic/traffic-vpp-accept-dscp-filter", sleepFixture(6*time.Second))
	Register("traffic/traffic-vpp-accept-multiclass", sleepFixture(6*time.Second))
	Register("traffic/traffic-vpp-not-connected", sleepFixture(6*time.Second))
}

func netfilterCommandOutput(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.Output()
	return string(out), err
}

func commandIgnore(ctx context.Context, name string, args ...string) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	_ = cmd.Run()
}

func waitDaemon(ctx context.Context, attempts int, delay time.Duration) (int, error) {
	var pid int
	ok := Poll(ctx, attempts, delay, func() bool {
		if _, err := os.Stat("daemon.ready"); err != nil {
			return false
		}
		raw, err := os.ReadFile("daemon.pid")
		if err != nil {
			return false
		}
		pid, err = strconv.Atoi(strings.TrimSpace(string(raw)))
		return err == nil && pid > 0
	})
	if !ok {
		return 0, fmt.Errorf("daemon.pid/ready never appeared")
	}
	return pid, nil
}

func signalProcess(pid int, sig syscall.Signal) error {
	if err := syscall.Kill(pid, sig); err != nil && !errors.Is(err, syscall.ESRCH) {
		return fmt.Errorf("signal daemon %d: %w", pid, err)
	}
	return nil
}

func processAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

func waitDead(ctx context.Context, pid int) {
	Poll(ctx, 100, 50*time.Millisecond, func() bool { return !processAlive(pid) })
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o600)
}

func reloadAndStop(ctx context.Context, hold time.Duration) error {
	pid, err := waitDaemon(ctx, 200, 50*time.Millisecond)
	if err != nil {
		return err
	}
	if err := copyFile("config2.conf", "ze-bgp.conf"); err != nil {
		return err
	}
	if err := signalProcess(pid, syscall.SIGHUP); err != nil {
		return err
	}
	if !sleepContext(ctx, hold) {
		return ctx.Err()
	}
	return signalProcess(pid, syscall.SIGTERM)
}

func sleepContext(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
