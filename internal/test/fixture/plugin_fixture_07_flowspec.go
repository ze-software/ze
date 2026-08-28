package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func filterFamilyExportFlowSpec07(ctx context.Context, p *sdk.Plugin) error {
	if err := waitEOR07(ctx, p, 2); err != nil {
		return err
	}
	command := "update text extended-community [copy-to-nexthop] nhop 1.2.3.4 nlri ipv4/flow add destination-ipv4 10.1.0.0/24 protocol tcp destination-port 80"
	announced, withdrawn, err := p.UpdateRoute(ctx, "*", command)
	if err != nil {
		return fmt.Errorf("announce FlowSpec route: %w", err)
	}
	if announced != 1 || withdrawn != 0 {
		return fmt.Errorf("announce FlowSpec route: announced=%d withdrawn=%d, want 1/0", announced, withdrawn)
	}
	if err := quiesce07(ctx, p); err != nil {
		return err
	}
	unfiltered, err := peerRow07(ctx, p, "127.0.0.2")
	if err != nil {
		return err
	}
	if number07(unfiltered["updates-sent"])-number07(unfiltered["eor-sent"]) < 1 {
		return fmt.Errorf("unfiltered peer received no UPDATE beyond EOR: %s", text07(unfiltered))
	}
	filtered, err := peerRow07(ctx, p, "127.0.0.1")
	if err != nil {
		return err
	}
	if leaked := number07(filtered["updates-sent"]) - number07(filtered["eor-sent"]); leaked != 0 {
		return fmt.Errorf("filtered peer received %d UPDATE(s) beyond EOR: %s", leaked, text07(filtered))
	}
	return nil
}

func flowSpecAnnounce07(ctx context.Context, p *sdk.Plugin) error {
	if err := waitEOR07(ctx, p, 1); err != nil {
		return err
	}
	command := "update text extended-community [copy-to-nexthop] nhop 1.2.3.4 nlri ipv4/flow add source-ipv4 10.0.0.2/32"
	announced, withdrawn, err := p.UpdateRoute(ctx, "*", command)
	if err != nil {
		return err
	}
	if announced != 1 || withdrawn != 0 {
		return fmt.Errorf("announce FlowSpec route: announced=%d withdrawn=%d, want 1/0", announced, withdrawn)
	}
	return quiesce07(ctx, p)
}

func flowSpecWithdraw07(ctx context.Context, p *sdk.Plugin) error {
	if err := waitEOR07(ctx, p, 1); err != nil {
		return err
	}
	commands := []struct {
		command              string
		announced, withdrawn uint32
	}{
		{"update text extended-community [copy-to-nexthop] nhop 1.2.3.4 nlri ipv4/flow add destination-ipv4 10.1.0.0/24", 1, 0},
		{"update text nlri ipv4/flow del destination-ipv4 10.1.0.0/24", 0, 1},
	}
	for _, step := range commands {
		announced, withdrawn, err := p.UpdateRoute(ctx, "*", step.command)
		if err != nil {
			return err
		}
		if announced != step.announced || withdrawn != step.withdrawn {
			return fmt.Errorf("%q: announced=%d withdrawn=%d, want %d/%d", step.command, announced, withdrawn, step.announced, step.withdrawn)
		}
		if err := quiesce07(ctx, p); err != nil {
			return err
		}
	}
	return nil
}

func flowSpecLegacySeed07(ctx context.Context, _ []string) error {
	commands := [][]string{
		{"add", "table", "inet", "flowspec"},
		{"add", "chain", "inet", "flowspec", "flowspec-fwd", "{ type filter hook forward priority -1 ; policy accept ; }"},
		{"add", "rule", "inet", "flowspec", "flowspec-fwd", "ip", "daddr", "198.51.100.0/24", "drop"},
	}
	for _, args := range commands {
		if output, err := exec.CommandContext(ctx, "nft", args...).CombinedOutput(); err != nil {
			return fmt.Errorf("nft %s: %w: %s", strings.Join(args, " "), err, output)
		}
	}
	output, err := exec.CommandContext(ctx, "nft", "list", "table", "inet", "flowspec").Output()
	if err != nil {
		return fmt.Errorf("list seeded FlowSpec table: %w", err)
	}
	_, err = os.Stdout.Write(output)
	return err
}

func nftRuleset07(ctx context.Context) (string, error) {
	output, err := exec.CommandContext(ctx, "nft", "list", "ruleset").Output()
	return string(output), err
}

func tableBlock07(ruleset, header string) string {
	start := strings.Index(ruleset, header)
	if start < 0 {
		return ""
	}
	depth := 0
	for index := start; index < len(ruleset); index++ {
		switch ruleset[index] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return ruleset[start : index+1]
			}
		}
	}
	return ruleset[start:]
}

func flowSpecLegacyTable07(ctx context.Context, _ []string) error {
	pid, err := waitDaemon07(ctx)
	if err != nil {
		return err
	}
	var ruleset string
	ok := Poll(ctx, 200, 100*time.Millisecond, func() bool {
		ruleset, err = nftRuleset07(ctx)
		return err == nil && strings.Contains(tableBlock07(ruleset, "table inet ze_flowspec {"), "10.1.0.0/24")
	})
	fmt.Fprint(os.Stdout, ruleset)
	_ = terminate07(pid)
	if !ok {
		return errors.New("announced route never reached table inet ze_flowspec")
	}
	if strings.Contains(ruleset, "table inet flowspec {") || strings.Contains(ruleset, "198.51.100.0/24") {
		return errors.New("legacy FlowSpec table or rule survived reconcile")
	}
	if copies := strings.Count(tableBlock07(ruleset, "table inet ze_flowspec {"), "10.1.0.0/24"); copies != 1 {
		return fmt.Errorf("table inet ze_flowspec holds %d copies of route, want 1", copies)
	}
	return nil
}

func flowSpecSCTP07(ctx context.Context, _ []string) error {
	pid, err := waitDaemon07(ctx)
	if err != nil {
		return err
	}
	var ruleset string
	ok := Poll(ctx, 200, 100*time.Millisecond, func() bool {
		ruleset, err = nftRuleset07(ctx)
		return err == nil && strings.Contains(ruleset, "table inet ze_flowspec") && (strings.Contains(ruleset, "l4proto 132") || strings.Contains(ruleset, "l4proto sctp"))
	})
	fmt.Fprint(os.Stdout, ruleset)
	_ = terminate07(pid)
	if !ok {
		return errors.New("no SCTP rule in kernel ruleset")
	}
	return nil
}

func flowSpecUntranslatable07(ctx context.Context, args []string) error {
	if len(args) != 1 {
		return errors.New("untranslatable FlowSpec fixture requires telemetry port")
	}
	pid, err := waitDaemon07(ctx)
	if err != nil {
		return err
	}
	var ruleset, metrics string
	installed := Poll(ctx, 200, 100*time.Millisecond, func() bool {
		ruleset, err = nftRuleset07(ctx)
		return err == nil && strings.Contains(ruleset, "ze_fslocal") && (strings.Contains(ruleset, "l4proto 132") || strings.Contains(ruleset, "l4proto sctp"))
	})
	counted := Poll(ctx, 200, 100*time.Millisecond, func() bool {
		metrics, err = fetch07(ctx, args[0])
		return err == nil && strings.Contains(metrics, `ze_flowspec_rules_refused_total{reason="unknown-protocol"}`)
	})
	fmt.Fprint(os.Stdout, ruleset)
	for _, line := range strings.Split(metrics, "\n") {
		if strings.HasPrefix(line, "ze_flowspec") {
			fmt.Fprintln(os.Stdout, line)
		}
	}
	_ = terminate07(pid)
	if !installed {
		return errors.New("reconcile did not survive untranslatable FlowSpec route")
	}
	if !counted {
		return errors.New("refused route never reached ze_flowspec_rules_refused_total")
	}
	return nil
}

func peerPIDs07() ([]int, error) {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil, err
	}
	var pids []int
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		raw, err := os.ReadFile("/proc/" + entry.Name() + "/cmdline")
		if err != nil {
			continue
		}
		argv := strings.Split(string(raw), "\x00")
		if len(argv) > 1 && argv[1] == "peer" {
			pids = append(pids, pid)
		}
	}
	return pids, nil
}

func flowSpecTables07(ruleset string) []string {
	var tables []string
	for _, line := range strings.Split(ruleset, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "table ") && strings.Contains(line, "flowspec") {
			tables = append(tables, line)
		}
	}
	return tables
}

func flowSpecWithdrawTable07(ctx context.Context, _ []string) error {
	pid, err := waitDaemon07(ctx)
	if err != nil {
		return err
	}
	var ruleset string
	settled := Poll(ctx, 300, 100*time.Millisecond, func() bool {
		ruleset, err = nftRuleset07(ctx)
		return err == nil && strings.Contains(ruleset, "table inet ze_flowspec") && strings.Contains(ruleset, "10.1.0.0/24") && !strings.Contains(ruleset, "10.2.0.0/24")
	})
	if !settled {
		fmt.Fprint(os.Stdout, ruleset)
		_ = terminate07(pid)
		return errors.New("kernel never reached one kept route and no withdrawn route")
	}
	installed := ruleset
	if copies := strings.Count(installed, "10.1.0.0/24"); copies != 1 {
		_ = terminate07(pid)
		return fmt.Errorf("kept route installed %d times, want 1", copies)
	}
	peers, err := peerPIDs07()
	if err != nil || len(peers) == 0 {
		_ = terminate07(pid)
		return fmt.Errorf("no peer process to stop: %w", err)
	}
	for _, peer := range peers {
		_ = terminate07(peer)
	}
	gone := Poll(ctx, 300, 100*time.Millisecond, func() bool {
		ruleset, err = nftRuleset07(ctx)
		return err == nil && len(flowSpecTables07(ruleset)) == 0
	})
	fmt.Fprint(os.Stdout, installed)
	fmt.Fprint(os.Stdout, ruleset)
	_ = terminate07(pid)
	if !gone {
		return fmt.Errorf("FlowSpec table survives last withdraw: %v", flowSpecTables07(ruleset))
	}
	return nil
}
