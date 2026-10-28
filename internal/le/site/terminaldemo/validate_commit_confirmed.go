// Design: docs/guide/config-editor.md -- confirmed commits in the running SSH editor.
// Related: register_scenarios.go -- the standalone and continuous topic share this check.

package siteterminaldemo

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/ze-software/ze/internal/component/config"
)

func checkTopicCommitConfirmed() error {
	if err := awaitTopicCommitConfirmed("original"); err != nil {
		return err
	}
	environ := scenarioEnv(showcaseLabID, demoPassword)
	output, err := runPTYFixture(environ,
		"--delay", "0.2", "--timeout", "30",
		"--command", "show system host",
		"--command", "load file absolute merge /src/demos/terminal/topics/commit-confirmed/1-trial-host.conf",
		"--command", "@wait merged from",
		"--command", "show | compare",
		"--command", "commit confirmed 8",
		"--command", "@wait Confirm within",
		"--command", "show system host",
		"--command", "@wait automatically rolled back",
		"--command", "show system host",
		"--command", "show | compare",
		"--command", "show | changes",
		"--command", "@wait No pending changes",
		"--command", "exit", "--command", `@wait operational\]`, "--command", "exit",
		"--", "sshpass", "-e", "ssh", "ze-demo")
	if err != nil {
		return err
	}
	if err := requireAll(output, "edge-original", "edge-trial", "automatically rolled back", "No pending changes"); err != nil {
		return err
	}
	// Read the committed store between transactions, not the editor's retained text.
	if err := awaitTopicCommitConfirmed("original"); err != nil {
		return err
	}
	if err := requireConfirmedHistory(); err != nil {
		return err
	}
	output, err = runPTYFixture(environ,
		"--delay", "0.2", "--timeout", "30",
		"--command", "load file absolute merge /src/demos/terminal/topics/commit-confirmed/2-confirmed-host.conf",
		"--command", "@wait merged from",
		"--command", "show | compare",
		"--command", "commit confirmed 15",
		"--command", "@wait Confirm within",
		"--command", "commit accept",
		"--command", "@wait Commit accepted",
		// sleep(timer): commit confirmed 15 above sets the deadline the accepted value must survive.
		"--command", "@sleep 16",
		"--command", "show system host",
		"--command", "exit", "--command", `@wait operational\]`, "--command", "exit",
		"--", "sshpass", "-e", "ssh", "ze-demo")
	if err != nil {
		return err
	}
	if err := requireAll(output, "Commit accepted", "edge-confirmed"); err != nil {
		return err
	}
	return awaitTopicCommitConfirmed("confirmed")
}

func awaitTopicCommitConfirmed(stage string) error {
	var expected string
	switch stage {
	case "original":
		expected = "edge-original"
	case "confirmed":
		expected = "edge-confirmed"
	default:
		return fmt.Errorf("unknown confirmed-commit readiness stage %q", stage)
	}
	return awaitPolicyTopic(func() error {
		text, err := runZe([]string{commandConfig, commandCat, zeConfigFile}, scenarioEnv(showcaseLabID, demoPassword), nil)
		if err != nil {
			return err
		}
		tree, err := config.ParseTreeForValidation(text)
		if err != nil {
			return err
		}
		system := tree.GetContainer("system")
		if system == nil {
			return fmt.Errorf("committed configuration has no system section")
		}
		host, present := system.Get("host")
		if !present {
			return fmt.Errorf("committed configuration has no hostname")
		}
		if host != expected {
			return fmt.Errorf("committed hostname is %q, expected %q", host, expected)
		}
		return nil
	})
}

func requireConfirmedHistory() error {
	// History resolves the supplied file's folder, not ZE_CONFIG_DIR.
	path := filepath.Join(scenarioConfigDir(showcaseLabID), zeConfigFile)
	output, err := cli(scenarioEnv(showcaseLabID, demoPassword), "show config history "+path+" | json | no-more")
	if err != nil {
		return err
	}
	var history []json.RawMessage
	if err := json.Unmarshal([]byte(output), &history); err != nil {
		return err
	}
	if len(history) < 2 {
		return fmt.Errorf("confirmed commit and rollback left no revision history: %s", output)
	}
	return nil
}
