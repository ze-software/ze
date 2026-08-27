// Design: docs/architecture/core-design.md -- the RFC engine proved against fixtures
// Detail: selftest_core.go -- parser, carrier, coverage, status, ratchet, and check stages
// Detail: selftest_state.go -- audit, extraction, render, and write stages
//
// selftest.go owns the RFC fixture-suite control flow and the action answer.
// Each stage uses the production RFC engine and returns one row per property.
package rfc

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/leaction"
	"github.com/ze-software/ze/internal/le/leroot"
)

// selftestStage is one independent concern of the in-process fixture suite.
type selftestStage struct {
	name string
	run  func() ([]leroot.SelftestResult, error)
}

// selftestStages declares the complete RFC engine concern population.
func selftestStages() []selftestStage {
	return []selftestStage{
		{name: "summary", run: func() ([]leroot.SelftestResult, error) {
			return runSummarySelftest(summarySelftestFixture())
		}},
		{name: "carriers", run: runCarrierSelftest},
		{name: "coverage", run: runCoverageSelftest},
		{name: "status", run: runStatusSelftest},
		{name: "audit", run: runAuditSelftest},
		{name: "extraction", run: runExtractionSelftest},
		{name: "render", run: runRenderSelftest},
		{name: "baseline", run: runBaselineSelftest},
		{name: "check", run: runCheckSelftest},
		{name: "real-tree", run: runRealTreeSelftest},
	}
}

// Selftest runs every RFC engine fixture stage in-process.
//
// A fixture write or engine error is returned separately. A property that the
// engine gets wrong is a failed report row.
func Selftest() (leroot.SelftestReport, error) {
	var results []leroot.SelftestResult
	for _, stage := range selftestStages() {
		rows, err := stage.run()
		if err != nil {
			return leroot.SelftestReport{}, err
		}
		results = append(results, rows...)
	}
	return leroot.NewSelftestReport(
		"rfc_requirements selftest OK",
		"rfc_requirements selftest FAILED:",
		results...,
	), nil
}

// selftestResult answers one named property row.
func selftestResult(name string, passed bool, detail string) leroot.SelftestResult {
	if passed {
		return leroot.Pass(name)
	}
	var message textbuf.Buffer
	return leroot.Fail(name, message.Str(name).Str(": ").Str(detail).String())
}

// newSelftestTree writes one fixture checkout.
func newSelftestTree(prefix string, files map[string]string) (string, error) {
	root, err := os.MkdirTemp("", prefix)
	if err != nil {
		return "", err
	}
	if err := writeSelftestFiles(root, files); err != nil {
		cleanupErr := os.RemoveAll(root)
		return "", errors.Join(err, cleanupErr)
	}
	return root, nil
}

// writeSelftestFiles writes path-to-content entries under one fixture root.
func writeSelftestFiles(root string, files map[string]string) error {
	for rel, body := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			return err
		}
	}
	return nil
}

// selftestAnswer is the `le rfc selftest` action.
func selftestAnswer() (any, int) {
	report, err := Selftest()
	if err != nil {
		leaction.ReportError(err)
		return nil, 2
	}
	return report, report.Code(1)
}
