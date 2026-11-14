// Design: docs/guide/config-editor.md -- confirmed commits in the running SSH editor.
// Related: register_scenarios.go -- the standalone and continuous topic share this check.

package siteterminaldemo

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"time"

	"github.com/ze-software/ze/internal/component/config"
)

func checkTopicCommitConfirmed() error {
	if err := awaitTopicCommitConfirmed("original"); err != nil {
		return err
	}
	revisionsBefore, err := confirmedHistoryRevisions()
	if err != nil {
		return err
	}
	environ := scenarioEnv(showcaseLabID, demoPassword)
	// The transcript cannot show that the trial was applied: `show | compare`
	// prints edge-trial before the commit. The committed store is read while
	// the session holds the confirm window open.
	output, err := runPTYFixtureObserved(environ, awaitCommitConfirmedTrial,
		flagDelay, "0.2", flagTimeout, "30",
		flagCommand, "show system host",
		flagCommand, "load file absolute merge /src/demos/terminal/topics/commit-confirmed/1-trial-host.conf",
		flagCommand, ptyWait("merged from"),
		flagCommand, "show | compare",
		flagCommand, "commit confirmed 15",
		flagCommand, ptyWait("Confirm within"),
		flagCommand, "show system host",
		flagCommand, ptyWait("automatically rolled back"),
		flagCommand, "show system host",
		flagCommand, "show | compare",
		flagCommand, "show | changes",
		flagCommand, ptyWait("No pending changes"),
		flagCommand, cliExitCommand, flagCommand, ptyWait(operationalModeLine), flagCommand, cliExitCommand,
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
	revisionsAfter, err := confirmedHistoryRevisions()
	if err != nil {
		return err
	}
	if err := requireTrialRecorded(revisionsBefore, revisionsAfter); err != nil {
		return err
	}
	output, err = runPTYFixture(environ,
		flagDelay, "0.2", flagTimeout, "30",
		flagCommand, "load file absolute merge /src/demos/terminal/topics/commit-confirmed/2-confirmed-host.conf",
		flagCommand, ptyWait("merged from"),
		flagCommand, "show | compare",
		flagCommand, "commit confirmed 15",
		flagCommand, ptyWait("Confirm within"),
		flagCommand, "commit accept",
		flagCommand, ptyWait("Commit accepted"),
		// sleep(timer): commit confirmed 15 above sets the deadline the accepted value must survive.
		flagCommand, ptySleep("16"),
		flagCommand, "show system host",
		flagCommand, cliExitCommand, flagCommand, ptyWait(operationalModeLine), flagCommand, cliExitCommand,
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
		expected = commitConfirmedOriginalHost
	case "confirmed":
		expected = "edge-confirmed"
	default:
		return fmt.Errorf("unknown confirmed-commit readiness stage %q", stage)
	}
	return awaitPolicyTopic(func() error { return requireCommittedHostname(expected) })
}

// commitConfirmedOriginalHost is the hostname the lab's base configuration
// holds, and commitConfirmedTrialHost the one 1-trial-host.conf commits for
// the length of the confirm window.
const (
	commitConfirmedOriginalHost = "edge-original"
	commitConfirmedTrialHost    = "edge-trial"
)

// awaitCommitConfirmedTrial waits until the committed store holds the trial
// hostname. It runs while the trial session is open, so it succeeds only
// inside the confirm window of a trial that was applied: before the commit and
// after the rollback the store holds edge-original. Thirty seconds cover the
// session's login and load and the fifteen-second window after them.
func awaitCommitConfirmedTrial() error {
	return awaitNetworkObservation("confirmed trial was never applied to the committed store", 30*time.Second, func() error {
		return requireCommittedHostname(commitConfirmedTrialHost)
	})
}

// requireCommittedHostname reads the committed store once, never the editor's
// candidate.
func requireCommittedHostname(expected string) error {
	text, err := runZe([]string{commandConfig, commandCat, zeConfigFile}, scenarioEnv(showcaseLabID, demoPassword), nil)
	if err != nil {
		return err
	}
	host, err := committedHostname(text)
	if err != nil {
		return err
	}
	if host != expected {
		return fmt.Errorf("committed hostname is %q, expected %q", host, expected)
	}
	return nil
}

// committedHostname answers `system host` of a configuration's text.
func committedHostname(text string) (string, error) {
	tree, err := config.ParseTreeForValidation(text)
	if err != nil {
		return "", err
	}
	return configHostname(tree)
}

// configHostname answers `system host` of a parsed configuration.
func configHostname(tree *config.Tree) (string, error) {
	system := tree.GetContainer("system")
	if system == nil {
		return "", fmt.Errorf("committed configuration has no system section")
	}
	host, present := system.Get("host")
	if !present {
		return "", fmt.Errorf("committed configuration has no hostname")
	}
	return host, nil
}

// confirmedHistoryRevisions counts the rollback revisions stored beside the
// lab's configuration.
func confirmedHistoryRevisions() (int, error) {
	// History resolves the supplied file's folder, not ZE_CONFIG_DIR.
	path := filepath.Join(scenarioConfigDir(showcaseLabID), zeConfigFile)
	output, err := cli(scenarioEnv(showcaseLabID, demoPassword), "show config history "+path+" | json | no-more")
	if err != nil {
		return 0, err
	}
	return countConfigRevisions(output)
}

// countConfigRevisions counts the stored revisions in the JSON answer of
// `show config history`. An open draft is a row of that answer too
// (dataHistory, internal/component/config/cli/config_data.go); it carries no
// timestamp and is not a revision.
func countConfigRevisions(document string) (int, error) {
	var rows []struct {
		Timestamp string `json:"timestamp"`
	}
	if err := json.Unmarshal([]byte(document), &rows); err != nil {
		return 0, fmt.Errorf("decode config history: %w; answer: %s", err, document)
	}
	revisions := 0
	for _, row := range rows {
		if row.Timestamp != "" {
			revisions++
		}
	}
	return revisions, nil
}

// requireTrialRecorded compares the revision counts read before the trial and
// after its rollback. The trial's commit stores one revision
// (storage.WriteCandidateVersionWithGuard, from Editor.commitSessionCandidate)
// and the deadline's revert stores another (confirmRevert,
// cmd/ze/hub/confirm_window.go), so a trial that was applied and rolled back
// adds two. A trial that never reached the store adds none, and one that was
// applied and never reverted adds one. A floor on the total proved nothing:
// init, the import and the topics committed earlier already left revisions.
func requireTrialRecorded(before, after int) error {
	const trialRevisions = 2
	if after-before >= trialRevisions {
		return nil
	}
	return fmt.Errorf("the confirmed trial and its rollback stored %d revisions, expected at least %d (%d before the trial, %d after its rollback): the trial was not applied, or was not reverted",
		after-before, trialRevisions, before, after)
}
