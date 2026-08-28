package fixture

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func policyRuleLines(dump string) []string {
	var rules []string
	for _, line := range strings.Split(dump, "\n") {
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

func policyBootApply(ctx context.Context, _ []string) error {
	pid, err := waitDaemon(ctx, 200, 50*time.Millisecond)
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
	pid, err := waitDaemon(ctx, 200, 50*time.Millisecond)
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
	pid, err := waitDaemon(ctx, 200, 50*time.Millisecond)
	if err != nil {
		return err
	}
	var rules string
	if !Poll(ctx, 100, 50*time.Millisecond, func() bool {
		rules, _ = netfilterCommandOutput(ctx, "ip", "rule", "show")
		for _, line := range strings.Split(rules, "\n") {
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
	for _, line := range strings.Split(rules, "\n") {
		if strings.Contains(line, "lookup 100") && strings.Contains(line, "fwmark") {
			fmt.Printf("IP_RULE: %s\n", line)
		}
	}
	return signalProcess(pid, syscall.SIGTERM)
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
	pid, err := waitDaemon(ctx, 200, 50*time.Millisecond)
	if err != nil {
		return err
	}
	var rules string
	if !Poll(ctx, 100, 50*time.Millisecond, func() bool {
		rules, _ = netfilterCommandOutput(ctx, "ip", "rule", "show")
		for _, line := range strings.Split(rules, "\n") {
			if autoTableRule(line) {
				return true
			}
		}
		return false
	}) {
		return fmt.Errorf("auto table ip rule was not programmed")
	}
	nftOut, err := netfilterCommandOutput(ctx, "nft", "list", "table", "inet", "ze_pr")
	if err != nil {
		return err
	}
	fmt.Print(nftOut)
	for _, line := range strings.Split(rules, "\n") {
		if autoTableRule(line) {
			fmt.Printf("IP_RULE_AUTO: %s\n", line)
		}
	}
	routes, err := netfilterCommandOutput(ctx, "ip", "route", "show", "table", "all")
	if err != nil {
		return err
	}
	for _, line := range strings.Split(routes, "\n") {
		if strings.Contains(line, "10.0.0.1") && strings.Contains(line, "proto") {
			fmt.Printf("AUTO_ROUTE: %s\n", line)
		}
	}
	return signalProcess(pid, syscall.SIGTERM)
}

func policyReload(ctx context.Context, _ []string) error {
	deadline := time.Now().Add(12 * time.Second)
	pid, err := waitDaemon(ctx, 200, 50*time.Millisecond)
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
		return fmt.Errorf("Phase 1 FAIL: ze_pr table missing after boot")
	}
	beforeRules := policyRuleLines(before)
	if len(beforeRules) == 0 {
		return fmt.Errorf("Phase 1 FAIL: ze_pr has no rules at all; got:\n%s", before)
	}
	if !containsRule(beforeRules, "accept") {
		return fmt.Errorf("Phase 1 FAIL: accept rule missing; got:\n%s", before)
	}
	fmt.Println("PHASE1_OK")
	if err := copyFile("config2.conf", "ze-bgp.conf"); err != nil {
		return err
	}
	if err := signalProcess(pid, syscall.SIGHUP); err != nil {
		return err
	}
	waitRules(func(rules []string) bool { return containsRule(rules, "drop") && !containsRule(rules, "accept") })
	after, err := netfilterCommandOutput(ctx, "nft", "list", "table", "inet", "ze_pr")
	if err != nil {
		return fmt.Errorf("Phase 2 FAIL: ze_pr table missing after reload")
	}
	afterRules := policyRuleLines(after)
	if len(afterRules) == 0 {
		return fmt.Errorf("Phase 2 FAIL: ze_pr has no rules after reload; got:\n%s", after)
	}
	if containsRule(afterRules, "accept") {
		return fmt.Errorf("Phase 2 FAIL: old accept rule still present after reload; got:\n%s", after)
	}
	if !containsRule(afterRules, "drop") {
		return fmt.Errorf("Phase 2 FAIL: new drop rule missing after reload; got:\n%s", after)
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
