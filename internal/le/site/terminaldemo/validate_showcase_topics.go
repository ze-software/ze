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
		if word != tapeSourceCommand {
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
	if err := awaitShowcasePeers(environ, showcaseTransit, showcaseFRR); err != nil {
		return err
	}
	answer, err := runPTYFixture(environ, showcaseDashboardSession()...)
	if err != nil {
		return err
	}
	return requireAll(answer, "established", "ASN ^", "operational]")
}

// dashboardReturnWait is the wait that follows the `q` that leaves
// `monitor bgp`. It names the status line stopDashboard sets
// (internal/component/cli/model_dashboard.go), which only the CLI's own view
// renders, so it proves the dashboard gave the screen back. The `exit` after
// it must then close the session, which proves the prompt took a command.
//
// It must not wait for `operational]` or the prompt. A command session's wait
// reads only the rows whose text changed since the last keystroke
// (terminalScreen.show), and the mode line and the prompt come back with the
// text they had before the dashboard opened. They count as changed only when
// the dashboard painted over their rows, which the peer table does not.
// operationalModeLine is the pattern of the mode line the CLI shows once
// `exit` has left configuration mode (idleInfoLine, internal/component/cli).
const operationalModeLine = `operational\]`

const dashboardReturnWait = tapeWaitDirective + " dashboard stopped"

// showcaseDashboardSession answers the recorder arguments of the showcase's
// dashboard check: leave configuration mode, open the dashboard on an
// established peer, sort by ASN, quit, and close the session.
func showcaseDashboardSession() []string {
	return []string{flagTimeout, "30", flagDelay, liveTopicKeyDelay,
		flagCommand, cliExitCommand, flagCommand, ptyWait(operationalModeLine),
		flagCommand, "monitor bgp", flagCommand, ptyWait("established"),
		flagCommand, ptyType("s"), flagCommand, ptyWait(`ASN \^`),
		flagCommand, ptyType("q"), flagCommand, dashboardReturnWait,
		flagCommand, cliExitCommand, "--", "sshpass", "-e", "ssh", "ze-demo"}
}
