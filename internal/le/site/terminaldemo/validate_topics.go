// Design: docs/contributing/terminal-demos.md -- one check shared by each topic and the continuous showcase
// Related: registry.go -- topics enrich the existing scenario registry

package siteterminaldemo

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
)

// liveTopic holds the authored fragment and its check against an already-running
// lab. The check MUST NOT prepare, start or stop the shown daemon.
type liveTopic struct {
	fragment string
	check    func() error
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
// are relative to demos/terminal. This uses the same editor as the recording,
// never an offline import into the daemon's store.
func loadShowcaseSections(paths ...string) error {
	if len(paths) == 0 {
		return errors.New("showcase load needs at least one configuration section")
	}
	args := []string{flagTimeout, "60", "--delay", "0.1"}
	for _, path := range paths {
		args = append(args,
			"--command", "load file absolute merge "+filepath.Join(demoTree(), path),
			"--command", `@wait merged from \S*`+regexp.QuoteMeta(filepath.Base(path)))
	}
	args = append(args,
		"--command", "show | compare",
		"--command", "commit now", "--command", "@wait Session committed",
		"--command", "exit", "--command", `@wait operational\]`,
		"--command", "exit", "--", "sshpass", "-e", "ssh", "ze-demo")
	output, err := runPTYFixture(scenarioEnv(showcaseLabID, demoPassword), args...)
	if err != nil {
		return fmt.Errorf("load showcase sections over SSH: %w\n%s", err, output)
	}
	return contains(output, "Session committed")
}
