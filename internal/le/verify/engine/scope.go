// Design: docs/architecture/testing/verify-freshness-scope.md -- one change set for the whole run
// Overview: run.go -- the run that publishes it
// Detail: ../../changed/selector.go -- the selector that produces both answers
//
// A verify run selects the change set ONCE, before its first stage, and names
// both answers to every stage it starts.
//
// One selection per run is what keeps the stages consistent. Six sessions share
// this checkout, so a stage that ran the selector itself would answer about the
// tree as it stood at that minute, and two stages of one run could then judge
// two different trees with nothing saying which. It also pays the selector's
// 2.6s `go list` once rather than once for each scoped stage.
//
// Both answers go to files inside the run's own log directory. Two runs of one
// checkout therefore never overwrite each other's answer, and the answer a red
// run judged stays beside the logs that recorded the red.
//
// Every route that cannot produce an answer publishes NOTHING, and unset is the
// widest reading of both variables: a stage with no package answer selects its
// own, and a matrix with no tag answer judges every row. A narrow answer is
// published only when the selector produced one, because a guard that cannot
// read its input must not return a valid-looking narrow answer
// (ai/rules/evidence.md).

package verifyengine

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/changed"
	"github.com/ze-software/ze/internal/le/gaterun"
)

const (
	// scopePackagesFile holds the packages a scoped stage must cover, one
	// ./-prefixed word per line.
	scopePackagesFile = "scope-packages.txt"
	// scopeTagsFile holds the feature tags the change set reaches, one per line.
	scopeTagsFile = "scope-tags.txt"
)

// changeScopeAnswer names the two files this run published.
type changeScopeAnswer struct {
	packagesPath string
	tagsPath     string
}

// publishChangeScope selects the change set for root, writes both answers under
// logDir, and names each one to every stage this run starts. It answers the
// restore that puts the previous values back when the run ends.
//
// The returned function is never nil, so the caller defers it unconditionally.
func publishChangeScope(root, logDir string) func() {
	answer, err := selectChangeSet(root, logDir)
	if err != nil {
		gaterun.Note("verify: the change set was not published (" + err.Error() +
			"), so every stage judges the whole tree")
		return func() {}
	}
	restorePackages := nameChangeScope(changed.ScopeFileKey, answer.packagesPath)
	restoreTags := nameChangeScope(changed.ScopeTagsKey, answer.tagsPath)
	return func() {
		restoreTags()
		restorePackages()
	}
}

// selectChangeSet runs the selector once and writes the two answers it gives.
//
// A widened answer is a real answer and is published as it stands: the selector
// says `./...` and every tag, which is what a stage reading it must then judge.
// Only a selector that refused the checkout leaves the run with nothing to
// publish.
func selectChangeSet(root, logDir string) (changeScopeAnswer, error) {
	report, code := (changed.Scope{Root: root}).Resolve([]string{"--print=both"})
	if code != 0 {
		return changeScopeAnswer{}, errors.New("the selector refused this checkout")
	}
	answer := changeScopeAnswer{
		packagesPath: filepath.Join(logDir, scopePackagesFile),
		tagsPath:     filepath.Join(logDir, scopeTagsFile),
	}
	if err := writeChangeScopeAnswer(answer.packagesPath, report.Packages); err != nil {
		return changeScopeAnswer{}, err
	}
	if err := writeChangeScopeAnswer(answer.tagsPath, report.Tags); err != nil {
		return changeScopeAnswer{}, err
	}
	return answer, nil
}

// writeChangeScopeAnswer writes one answer, one line per entry. An empty answer
// writes an empty file rather than no file: the file's existence is what tells a
// stage the run did select, and its emptiness is the selector's own answer that
// no changed path is compiled or read by a Go package.
func writeChangeScopeAnswer(path string, lines []string) error {
	var body textbuf.Buffer
	for _, line := range lines {
		body.Str(line).Byte('\n')
	}
	if err := os.WriteFile(path, body.Bytes(), 0o600); err != nil {
		return fmt.Errorf("write %s: %w", filepath.Base(path), err)
	}
	return nil
}

// nameChangeScope gives one scope variable its value and answers the restore
// that puts the previous one back.
//
// The restore matters because the stages run inside this process: a run that
// left its answer behind would scope whatever ran next, a second run or a test,
// to a change set selected for a tree that has since moved.
func nameChangeScope(key, value string) func() {
	previous := env.Get(key)
	if err := env.Set(key, value); err != nil {
		gaterun.Note("verify: " + key + " could not be named (" + err.Error() +
			"), so every stage reading it judges the whole tree")
		return func() {}
	}
	return func() {
		if err := env.Set(key, previous); err != nil {
			gaterun.Note("verify: " + key + " could not be restored: " + err.Error())
		}
	}
}
