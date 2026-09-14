package fixture

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func policyRuleLines(dump string) []string {
	var rules []string
	for line := range strings.SplitSeq(dump, "\n") {
		s := strings.TrimSpace(line)
		if s == "" || s == "{" || s == "}" || strings.HasPrefix(s, "table ") || strings.HasPrefix(s, "chain ") || strings.HasPrefix(s, "type ") {
			continue
		}
		rules = append(rules, s)
	}
	return rules
}

func policyTableOutput(ctx context.Context, predicate func([]string) bool, attempts int) (string, error) {
	var out string
	var err error
	ok := Poll(ctx, attempts, 50*time.Millisecond, func() bool {
		out, err = netfilterCommandOutput(ctx, "nft", "list", "table", "inet", "ze_pr")
		return err == nil && predicate(policyRuleLines(out))
	})
	if !ok {
		return out, fmt.Errorf("policy rules were not programmed")
	}
	return out, nil
}

// policyRuleDump prints the ze_pr table and one RULE line per programmed rule,
// once the table carries at least two rules. The per-rule lines are what lets a
// test assert which matches share a rule, which a whole-table dump cannot show:
// two matches in one rule and the same two matches in two rules produce the
// same set of substrings.
func policyRuleDump(ctx context.Context, _ []string) error {
	pid, err := waitDaemon(ctx, 200)
	if err != nil {
		return err
	}
	out, err := policyTableOutput(ctx, func(rules []string) bool { return len(rules) >= 2 }, 100)
	if err != nil {
		return err
	}
	fmt.Print(out)
	for _, rule := range policyRuleLines(out) {
		fmt.Printf("RULE %s\n", rule)
	}
	return signalProcess(pid, syscall.SIGTERM)
}

func policySingleTable(ctx context.Context, _ []string) error {
	pid, err := waitDaemon(ctx, 200)
	if err != nil {
		return err
	}
	out, err := policyTableOutput(ctx, func(rules []string) bool { return len(rules) >= 1 }, 100)
	if err != nil {
		return err
	}
	fmt.Print(out)
	return signalProcess(pid, syscall.SIGTERM)
}

func policySetTable(ctx context.Context, _ []string) error {
	pid, err := waitDaemon(ctx, 200)
	if err != nil {
		return err
	}
	var rules string
	if !Poll(ctx, 100, 50*time.Millisecond, func() bool {
		rules, _ = netfilterCommandOutput(ctx, "ip", "rule", "show")
		for line := range strings.SplitSeq(rules, "\n") {
			if strings.Contains(line, "lookup 100") && strings.Contains(line, "fwmark") {
				return true
			}
		}
		return false
	}) {
		return fmt.Errorf("policy ip rule was not programmed")
	}
	nftOut, err := netfilterCommandOutput(ctx, "nft", "list", "table", "inet", "ze_pr")
	if err != nil {
		return err
	}
	fmt.Print(nftOut)
	var mark string
	for line := range strings.SplitSeq(rules, "\n") {
		if strings.Contains(line, "lookup 100") && strings.Contains(line, "fwmark") {
			fmt.Printf("IP_RULE: %s\n", line)
			mark = fwmarkOf(line)
		}
	}
	if err := probeMarkedLookup(ctx, mark, "198.51.100.7", "table 100", func() error {
		// The operator owns the contents of a table their `table N` action
		// names, so the probe route is the test standing in for them. It goes
		// in table 100 and nowhere else, which is what makes the rule the only
		// way a lookup can reach it.
		_, addErr := netfilterCommandOutput(ctx, "ip", "route", "add", "198.51.100.0/24", "dev", "lo", "table", "100")
		return addErr
	}); err != nil {
		return err
	}
	return signalProcess(pid, syscall.SIGTERM)
}

// fwmarkOf answers with the fwmark an `ip rule show` line selects on, in the
// hexadecimal form `ip route get` takes back. The mark is allocated at run time
// from 0x50000 upward, so a test cannot name it and has to read it.
func fwmarkOf(line string) string {
	fields := strings.Fields(line)
	for i, field := range fields {
		if field != "fwmark" || i+1 >= len(fields) {
			continue
		}
		// A rule carrying a mask prints `fwmark 0x50000/0xfffff`.
		return strings.SplitN(fields[i+1], "/", 2)[0]
	}
	return ""
}

// probeMarkedLookup asks the KERNEL where a packet carrying the policy's fwmark
// would go, which is the only question that separates a rule the kernel obeys
// from a rule that merely exists. Reading `ip rule show` back proves the second
// and says nothing about the first.
//
// The control is the same lookup without the mark. The probe destination is
// reachable only through the table the rule selects, so an unmarked lookup that
// resolves means something other than the rule answered, and the marked result
// would then prove nothing.
func probeMarkedLookup(ctx context.Context, mark, destination, want string, prepare func() error) error {
	if mark == "" {
		return fmt.Errorf("no fwmark on the installed ip rule, so the kernel lookup cannot be driven")
	}
	if prepare != nil {
		if err := prepare(); err != nil {
			return fmt.Errorf("staging the probe route: %w", err)
		}
	}

	marked, err := netfilterCommandOutput(ctx, "ip", "route", "get", destination, "mark", mark)
	if err != nil {
		return fmt.Errorf("the marked lookup for %s found no route at all: %w", destination, err)
	}
	fmt.Printf("ROUTE_GET_MARKED: %s\n", strings.TrimSpace(strings.SplitN(marked, "\n", 2)[0]))
	if !strings.Contains(marked, want) {
		return fmt.Errorf("the marked lookup for %s answered %q, want %q: the ip rule did not steer it", destination, strings.TrimSpace(marked), want)
	}

	control, err := netfilterCommandOutput(ctx, "ip", "route", "get", destination)
	if err == nil {
		return fmt.Errorf("the unmarked lookup for %s resolved to %q; only the marked lookup reaches %q", destination, strings.TrimSpace(control), want)
	}
	fmt.Printf("ROUTE_GET_CONTROL: unmarked lookup for %s found no route\n", destination)
	return nil
}

func autoTableRule(line string) bool {
	if !strings.Contains(line, "fwmark") || !strings.Contains(line, "lookup") {
		return false
	}
	parts := strings.Split(line, "lookup")
	table, err := strconv.Atoi(strings.TrimSpace(parts[len(parts)-1]))
	return err == nil && table >= 2000 && table <= 2999
}

func policyNextHop(ctx context.Context, _ []string) error {
	pid, err := waitDaemon(ctx, 200)
	if err != nil {
		return err
	}
	var rules string
	if !Poll(ctx, 100, 50*time.Millisecond, func() bool {
		rules, _ = netfilterCommandOutput(ctx, "ip", "rule", "show")
		return slices.ContainsFunc(strings.Split(rules, "\n"), autoTableRule)
	}) {
		return fmt.Errorf("auto table ip rule was not programmed")
	}
	nftOut, err := netfilterCommandOutput(ctx, "nft", "list", "table", "inet", "ze_pr")
	if err != nil {
		return err
	}
	fmt.Print(nftOut)
	for line := range strings.SplitSeq(rules, "\n") {
		if autoTableRule(line) {
			fmt.Printf("IP_RULE_AUTO: %s\n", line)
		}
	}
	routes, err := netfilterCommandOutput(ctx, "ip", "route", "show", "table", "all")
	if err != nil {
		return err
	}
	var mark string
	for line := range strings.SplitSeq(routes, "\n") {
		if strings.Contains(line, "10.0.0.1") && strings.Contains(line, "proto") {
			fmt.Printf("AUTO_ROUTE: %s\n", line)
		}
	}
	for line := range strings.SplitSeq(rules, "\n") {
		if autoTableRule(line) {
			mark = fwmarkOf(line)
		}
	}
	// Both producers are on this one answer: the rule has to select the auto
	// table, and the route applyAutoRoutes put in it has to resolve. No probe
	// route is staged, because the auto route IS the route under test.
	if err := probeMarkedLookup(ctx, mark, "203.0.113.9", "via 10.0.0.1", nil); err != nil {
		return err
	}
	return signalProcess(pid, syscall.SIGTERM)
}

func policyReload(ctx context.Context, _ []string) error {
	deadline := time.Now().Add(12 * time.Second)
	pid, err := waitDaemon(ctx, 200)
	if err != nil {
		return err
	}
	waitRules := func(predicate func([]string) bool) {
		remaining := time.Until(deadline)
		attempts := max(1, int(remaining/(50*time.Millisecond)))
		_, _ = policyTableOutput(ctx, predicate, attempts)
	}
	waitRules(func(rules []string) bool {
		for _, rule := range rules {
			if strings.Contains(rule, "accept") {
				return true
			}
		}
		return false
	})
	before, err := netfilterCommandOutput(ctx, "nft", "list", "table", "inet", "ze_pr")
	if err != nil {
		return fmt.Errorf("phase 1 FAIL: ze_pr table missing after boot")
	}
	beforeRules := policyRuleLines(before)
	if len(beforeRules) == 0 {
		return fmt.Errorf("phase 1 FAIL: ze_pr has no rules at all; got:\n%s", before)
	}
	if !containsRule(beforeRules, "accept") {
		return fmt.Errorf("phase 1 FAIL: accept rule missing; got:\n%s", before)
	}
	fmt.Println("PHASE1_OK")
	if err := stageReloadConfig(); err != nil {
		return err
	}
	if err := signalProcess(pid, syscall.SIGHUP); err != nil {
		return err
	}
	waitRules(func(rules []string) bool { return containsRule(rules, "drop") && !containsRule(rules, "accept") })
	after, err := netfilterCommandOutput(ctx, "nft", "list", "table", "inet", "ze_pr")
	if err != nil {
		return fmt.Errorf("phase 2 FAIL: ze_pr table missing after reload")
	}
	afterRules := policyRuleLines(after)
	if len(afterRules) == 0 {
		return fmt.Errorf("phase 2 FAIL: ze_pr has no rules after reload; got:\n%s", after)
	}
	if containsRule(afterRules, "accept") {
		return fmt.Errorf("phase 2 FAIL: old accept rule still present after reload; got:\n%s", after)
	}
	if !containsRule(afterRules, "drop") {
		return fmt.Errorf("phase 2 FAIL: new drop rule missing after reload; got:\n%s", after)
	}
	fmt.Println("PHASE2_OK")
	return signalProcess(pid, syscall.SIGTERM)
}

func containsRule(rules []string, fragment string) bool {
	for _, rule := range rules {
		if strings.Contains(rule, fragment) {
			return true
		}
	}
	return false
}
