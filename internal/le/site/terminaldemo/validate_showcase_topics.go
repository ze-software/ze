// Design: docs/contributing/terminal-demos.md -- independently recorded continuous showcase
// Related: demos/terminal/showcase/demo.tape -- the sole chapter ordering

package siteterminaldemo

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func validateShowcase() error {
	ids, err := showcaseTopicOrder()
	if err != nil {
		return err
	}
	return validateLiveTopic(func() error {
		pid, err := showcasePID()
		if err != nil {
			return err
		}
		if err := validateShowcaseMigration(); err != nil {
			return err
		}
		if err := requireShowcasePID(pid); err != nil {
			return err
		}
		if err := validateShowcaseDashboard(); err != nil {
			return err
		}
		for _, id := range ids {
			if err := requireShowcasePID(pid); err != nil {
				return err
			}
			sc, err := scenarios.lookup(id)
			if err != nil {
				return err
			}
			if err := sc.live.check(); err != nil {
				return fmt.Errorf("showcase chapter %s: %w", id, err)
			}
			if err := requireShowcasePID(pid); err != nil {
				return err
			}
		}
		return nil
	})
}

// showcaseTopicOrder reads the direct Source declarations authored in the
// showcase tape. It does not maintain a second chapter list. Every registered
// live topic must occur once; missing or repeated chapters refuse validation.
func showcaseTopicOrder() ([]string, error) {
	data, err := os.ReadFile(filepath.Join(demoDir("showcase"), "demo.tape"))
	if err != nil {
		return nil, err
	}
	var ordered []string
	seen := make(map[string]bool)
	for line := range strings.SplitSeq(string(data), "\n") {
		word, fragment, found := strings.Cut(strings.TrimSpace(line), " ")
		if !found {
			continue
		}
		if word != "Source" {
			continue
		}
		for id, sc := range scenarios {
			if sc.live == nil {
				continue
			}
			if filepath.Clean(strings.TrimSpace(fragment)) != filepath.Clean(sc.live.fragment) {
				continue
			}
			if seen[id] {
				return nil, fmt.Errorf("showcase sources live topic %s more than once", id)
			}
			seen[id] = true
			ordered = append(ordered, id)
		}
	}
	for id, sc := range scenarios {
		if sc.live != nil {
			if !seen[id] {
				return nil, fmt.Errorf("showcase does not source live topic %s (%s)", id, sc.live.fragment)
			}
		}
	}
	if len(ordered) == 0 {
		return nil, fmt.Errorf("showcase sources no registered live topics")
	}
	return ordered, nil
}

func validateShowcaseMigration() error {
	answer, err := runZe([]string{"exabgp", "migrate", filepath.Join(demoDir("showcase"), "exabgp.conf")},
		scenarioEnv(showcaseLabID, demoPassword), nil)
	if err != nil {
		return err
	}
	return requireAll(answer, "bgp {", "peer", "127.0.0.2", "65000", "65001")
}

func validateShowcaseDashboard() error {
	environ := scenarioEnv(showcaseLabID, demoPassword)
	if err := awaitShowcasePeers(environ, "transit", "frr"); err != nil {
		return err
	}
	answer, err := runPTYFixture(environ, "--timeout", "30", "--delay", "0.1",
		"--command", "exit", "--command", `@wait operational\]`,
		"--command", "monitor bgp", "--command", "@wait established",
		"--command", "@type s", "--command", `@wait ASN \^`,
		"--command", "@type q", "--command", `@wait operational\]`,
		"--command", "exit", "--", "sshpass", "-e", "ssh", "ze-demo")
	if err != nil {
		return err
	}
	return requireAll(answer, "established", "ASN ^", "operational]")
}
