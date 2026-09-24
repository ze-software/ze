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
		{argAdd, nftTable, nftFamilyInet, nftTableFlowspec},
		{argAdd, nftChain, nftFamilyInet, nftTableFlowspec, nftChainFlowspecForward, nftForwardHookSpec},
		{argAdd, nftRule, nftFamilyInet, nftTableFlowspec, nftChainFlowspecForward, "ip", nftMatchDestination, "198.51.100.0/24", nftVerdictDrop},
	}
	for _, args := range commands {
		if output, err := exec.CommandContext(ctx, "nft", args...).CombinedOutput(); err != nil { //nolint:gosec // the fixture chooses the program and its arguments
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

// Each selected rule is installed once in each relevant kernel hook.
func flowSpecBothHooks07(ruleset, prefix string) bool {
	table := tableBlock07(ruleset, "table inet ze_flowspec {")
	for _, hook := range []struct{ chain, hook string }{
		{"flowspec-in", "input"},
		{"flowspec-fwd", "forward"},
	} {
		chain := tableBlock07(table, "chain "+hook.chain+" {")
		if !strings.Contains(chain, "hook "+hook.hook) {
			return false
		}
		if strings.Count(chain, prefix) != 1 {
			return false
		}
	}
	return strings.Count(table, prefix) == 2
}

func flowSpecLegacyTable07(ctx context.Context, plugin *sdk.Plugin) error {
	var err error
	var ruleset string
	original := Poll(ctx, 150, 100*time.Millisecond, func() bool {
		ruleset, err = nftRuleset07(ctx)
		if err != nil {
			return false
		}
		table := tableBlock07(ruleset, "table inet ze_flowspec {")
		return flowSpecBothHooks07(ruleset, "10.1.0.0/24") && !strings.Contains(table, "log prefix")
	})
	if !original {
		return fmt.Errorf("unsampled original never reached both FlowSpec hooks exactly once: %s", ruleset)
	}
	if err := releaseFlowSpecPeer07(ctx, plugin); err != nil {
		return err
	}
	ok := Poll(ctx, 150, 100*time.Millisecond, func() bool {
		ruleset, err = nftRuleset07(ctx)
		if err != nil {
			return false
		}
		table := tableBlock07(ruleset, "table inet ze_flowspec {")
		return flowSpecBothHooks07(ruleset, "10.1.0.0/24") && strings.Contains(table, "log prefix")
	})
	fmt.Fprint(os.Stderr, ruleset) //nolint:errcheck // observed kernel state relayed by the daemon
	if !ok {
		return errors.New("sampled replacement never reached both FlowSpec hooks exactly once")
	}
	if strings.Contains(ruleset, "table inet flowspec {") || strings.Contains(ruleset, "198.51.100.0/24") {
		return errors.New("legacy FlowSpec table or rule survived reconcile")
	}
	return nil
}

func flowSpecSCTP07(ctx context.Context, _ []string) error {
	pid, err := waitDaemon07(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = terminate07(pid) }()
	var ruleset string
	ok := Poll(ctx, 200, 100*time.Millisecond, func() bool {
		ruleset, err = nftRuleset07(ctx)
		if err != nil {
			return false
		}
		return flowSpecBothHooks07(ruleset, "10.1.0.0/24") && (strings.Contains(ruleset, "l4proto 132") || strings.Contains(ruleset, "l4proto sctp"))
	})
	fmt.Fprint(os.Stdout, ruleset) //nolint:errcheck // progress output
	if !ok {
		return errors.New("protocol-only SCTP rule did not reach both kernel hooks")
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
	defer func() { _ = terminate07(pid) }()
	var ruleset, metrics string
	settled := Poll(ctx, 200, 100*time.Millisecond, func() bool {
		ruleset, err = nftRuleset07(ctx)
		if err != nil {
			return false
		}
		metrics, err = fetch07(ctx, args[0])
		if err != nil {
			return false
		}
		for _, reason := range []string{"unknown-protocol", "unsupported-component"} {
			count, exists := counter07(metrics, `ze_flowspec_rules_refused_total{reason="`+reason+`"}`)
			if !exists || count < 1 {
				return false
			}
		}
		table := tableBlock07(ruleset, "table inet ze_flowspec {")
		if strings.Contains(table, "10.2.0.0/24") || strings.Contains(table, "10.3.0.0/24") {
			return false
		}
		if !strings.Contains(ruleset, "table inet ze_fslocal {") {
			return false
		}
		return flowSpecBothHooks07(ruleset, "10.1.0.0/24") && (strings.Contains(table, "l4proto 132") || strings.Contains(table, "l4proto sctp"))
	})
	fmt.Fprint(os.Stdout, ruleset) //nolint:errcheck // progress output
	for line := range strings.SplitSeq(metrics, "\n") {
		if strings.HasPrefix(line, "ze_flowspec") {
			fmt.Fprintln(os.Stdout, line) //nolint:errcheck // progress output
		}
	}
	if !settled {
		return errors.New("refusal isolation did not preserve both hooks and the other owner with both refusals counted")
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
		if len(argv) > 1 && argv[1] == fieldPeer {
			pids = append(pids, pid)
		}
	}
	return pids, nil
}

func flowSpecTables07(ruleset string) []string {
	var tables []string
	for line := range strings.SplitSeq(ruleset, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "table ") && strings.Contains(line, "flowspec") {
			tables = append(tables, line)
		}
	}
	return tables
}

// The peer waits for this unique UPDATE before its observed-state transition.
// It is not an EOR or KEEPALIVE that initial sync could satisfy by accident.
// Keep the expectations in the legacy-table and withdrawal .ci carriers.
const flowSpecTransitionMarker07 = "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF003102000000154001010040020040050400000064400304C000020120C612FFFE"

// Use the attached process's authenticated SDK rail, not the operator SSH CLI.
func releaseFlowSpecPeer07(ctx context.Context, plugin *sdk.Plugin) error {
	_, err := plugin01RequireDone(ctx, plugin, "send bgp 127.0.0.1 raw hex "+flowSpecTransitionMarker07)
	return err
}

func flowSpecWithdrawTable07(ctx context.Context, plugin *sdk.Plugin) error {
	var err error
	var ruleset string
	installed := Poll(ctx, 150, 100*time.Millisecond, func() bool {
		ruleset, err = nftRuleset07(ctx)
		return err == nil && flowSpecBothHooks07(ruleset, "10.1.0.0/24") && flowSpecBothHooks07(ruleset, "10.2.0.0/24")
	})
	if !installed {
		return fmt.Errorf("both selected rules never reached both hooks: %s", ruleset)
	}
	if err := releaseFlowSpecPeer07(ctx, plugin); err != nil {
		return err
	}
	settled := Poll(ctx, 150, 100*time.Millisecond, func() bool {
		ruleset, err = nftRuleset07(ctx)
		return err == nil && flowSpecBothHooks07(ruleset, "10.1.0.0/24") && !strings.Contains(ruleset, "10.2.0.0/24")
	})
	if !settled {
		return fmt.Errorf("MP_UNREACH did not remove only the withdrawn rule: %s", ruleset)
	}
	fmt.Fprint(os.Stderr, ruleset) //nolint:errcheck // observed surviving rule relayed by the daemon
	peers, err := peerPIDs07()
	if err != nil {
		return err
	}
	if len(peers) == 0 {
		return errors.New("no peer process to stop after the observed withdrawal")
	}
	for _, peer := range peers {
		if err := terminate07(peer); err != nil {
			return fmt.Errorf("stop peer: %w", err)
		}
	}
	gone := Poll(ctx, 150, 100*time.Millisecond, func() bool {
		ruleset, err = nftRuleset07(ctx)
		return err == nil && len(flowSpecTables07(ruleset)) == 0
	})
	fmt.Fprint(os.Stderr, ruleset) //nolint:errcheck // observed final kernel state
	if !gone {
		return fmt.Errorf("FlowSpec table survives last withdraw: %v", flowSpecTables07(ruleset))
	}
	_, err = fmt.Fprintf(os.Stderr, "flowspec-tables-after-peer-withdraw=%d\n", len(flowSpecTables07(ruleset)))
	return err
}
