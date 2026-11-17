// Design: docs/contributing/terminal-demos.md -- one check shared by each topic and the continuous showcase
// Related: registry.go -- topics enrich the existing scenario registry

package siteterminaldemo

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"slices"

	"github.com/ze-software/ze/internal/component/config"
)

// liveTopic holds the authored fragment and its check against an already-running
// lab. The check MUST NOT prepare, start or stop the shown daemon.
type liveTopic struct {
	fragment string
	check    func() error
	// absent refuses a configuration that already holds what this topic's
	// live commit adds. The base guard runs every registered topic's
	// declaration, so a new topic is guarded with no edit to the guard.
	absent func(*config.Tree) error
}

// liveTopicIDs answers the registered live topics in a stable order, so a
// guard that walks them names the same topic first on every run.
func liveTopicIDs(registry scenarioRegistry) []string {
	ids := make([]string, 0, len(registry))
	for id, sc := range registry {
		if sc.live != nil {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	return ids
}

// requireLiveTopicsAbsent runs the absence declaration of every live topic in
// registry over tree, and names the first topic the tree already configures.
func requireLiveTopicsAbsent(registry scenarioRegistry, tree *config.Tree) error {
	for _, id := range liveTopicIDs(registry) {
		if err := registry[id].live.absent(tree); err != nil {
			return fmt.Errorf("live topic %s: %w", id, err)
		}
	}
	return nil
}

// requireLiveTopicAbsent reads the active store before a topic's live commit
// and runs that topic's own absence declaration over it.
func requireLiveTopicAbsent(id string) error {
	sc, err := scenarios.lookup(id)
	if err != nil {
		return err
	}
	if sc.live == nil {
		return fmt.Errorf("demo %s is not a live topic, so it declares nothing to be absent", id)
	}
	text, err := runZe([]string{commandConfig, commandCat, zeConfigFile}, scenarioEnv(showcaseLabID, demoPassword), nil)
	if err != nil {
		return err
	}
	tree, err := config.ParseTreeForValidation(text)
	if err != nil {
		return fmt.Errorf("parse running configuration: %w", err)
	}
	if err := sc.live.absent(tree); err != nil {
		return fmt.Errorf("%s is configured before its live commit: %w", id, err)
	}
	return nil
}

// validateLiveTopic owns one lab lifetime. Every callback MUST leave that
// lifetime to this wrapper, including when a diagnostic or recovery fails.
func validateLiveTopic(check func() error) (err error) {
	defer func() {
		err = errors.Join(err, runShowcaseLab(commandStop, nil, io.Discard))
	}()
	if err := runShowcaseLab(actionPrepare, nil, io.Discard); err != nil {
		return err
	}
	if err := runShowcaseLab(commandStart, nil, io.Discard); err != nil {
		return err
	}
	pid, err := showcasePID()
	if err != nil {
		return err
	}
	if err := requireShowcaseBase(); err != nil {
		return err
	}
	if err := check(); err != nil {
		return errors.Join(err, requireShowcasePID(pid))
	}
	return requireShowcasePID(pid)
}

func showcaseCLI(command string) (string, error) {
	return cli(scenarioEnv(showcaseLabID, demoPassword), command)
}

// loadShowcaseSections commits all named snippets in one SSH candidate. Paths
// name the staged topics inside the lab's configuration directory. This uses
// the same editor as the recording, never an offline import into its store.
// liveTopicKeyDelay is the recorder's pause, in seconds, after a command that
// no wait follows, in the sessions that drive the shared showcase daemon.
const liveTopicKeyDelay = "0.1"

// sessionCommitted is the text the editor prints once `commit now` landed.
const sessionCommitted = "Session committed"

// keyEnter names the Enter key of ptyKeys.
const keyEnter = "enter"

func loadShowcaseSections(paths ...string) error {
	if len(paths) == 0 {
		return errors.New("showcase load needs at least one configuration section")
	}
	args := []string{flagTimeout, "60", flagDelay, liveTopicKeyDelay}
	for _, path := range paths {
		args = append(args,
			flagCommand, "load file absolute merge "+path,
			flagCommand, ptyWait(`merged from \S*`+regexp.QuoteMeta(filepath.Base(path))))
	}
	args = append(args,
		flagCommand, "show | compare",
		flagCommand, "commit now", flagCommand, ptyWait(sessionCommitted),
		flagCommand, cliExitCommand, flagCommand, ptyWait(operationalModeLine),
		flagCommand, cliExitCommand, "--", "sshpass", "-e", "ssh", "ze-demo")
	output, err := runPTYFixture(scenarioEnv(showcaseLabID, demoPassword), args...)
	if err != nil {
		return fmt.Errorf("load showcase sections over SSH: %w\n%s", err, output)
	}
	return contains(output, "Session committed")
}
