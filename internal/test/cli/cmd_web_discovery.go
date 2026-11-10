// Design: docs/architecture/testing/runner-architecture.md -- immutable web discovery.
package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	webtesting "github.com/ze-software/ze/internal/component/web/testing"
	"github.com/ze-software/ze/internal/test/runner"
)

// discoverWebTests retains every case, including failures, before any dependency
// check or build. The parsed snapshot belongs to this suite for its whole run.
func discoverWebTests(tests *runner.TestSet[*zeTestWebTest], directory, baseDir string, concurrency int) error {
	return filepath.WalkDir(directory, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".wb") {
			return nil
		}
		name, err := filepath.Rel(baseDir, path)
		if err != nil {
			return err
		}
		test := discoverWebCase(path, concurrency)
		test.Name = name
		test.Nick = runner.GenerateNick(name)
		tests.Add(test)
		return nil
	})
}

// discoverWebCase publishes either the complete parsed snapshot or a parse
// failure. A failed read can never supply an insecure default server case.
func discoverWebCase(path string, concurrency int) *zeTestWebTest {
	test := &zeTestWebTest{Path: path}
	content, err := os.ReadFile(path) //nolint:gosec // Path is an authored test selected by suite discovery.
	if err == nil {
		test.parsed, err = webtesting.ParseWBFile(string(content))
	}
	if err == nil {
		// Validate headroom overflow before dependencies or startup. The actual
		// execution deadline is created by the admitted callback, not here.
		var cancel context.CancelFunc
		_, cancel, err = webtesting.CaseContext(context.Background(), test.parsed.Timeout, concurrency)
		if err == nil {
			cancel()
		}
	}
	if err == nil {
		if len(test.parsed.Plugins) > 0 {
			if test.parsed.ServerKind() != webtesting.WBServerDaemon {
				err = errors.New("option=plugin needs option=server:kind=daemon, the only kind that loads a config")
			}
		}
	}
	if err != nil {
		test.parsed = nil
		test.admission.ParseFailed = true
		test.admission.Error = fmt.Errorf("parse %s: %w", path, err)
		test.SetError(test.admission.Error)
		return test
	}
	webResourceRecord(&test.admission, &test.parsed.Admission)
	if test.parsed.SkipReason != "" {
		test.admission.SkipReason = test.parsed.SkipReason
	}
	return test
}

// webDiscoveryErrors MUST run before dependency checks: a missing browser must
// not hide a malformed selected case behind a casual-run skip.
func webDiscoveryErrors(tests []*zeTestWebTest) error {
	var failures []error
	for _, test := range tests {
		if test.admission.ParseFailed {
			failures = append(failures, test.admission.Error)
		}
	}
	return errors.Join(failures...)
}

func webRunnableTests(tests []*zeTestWebTest) []*zeTestWebTest {
	var runnable []*zeTestWebTest
	for _, test := range tests {
		if test.admission.ParseFailed {
			continue
		}
		if test.admission.SkipReason != "" {
			continue
		}
		runnable = append(runnable, test)
	}
	return runnable
}

func webAdmissionRecord(record *runner.Record, test *zeTestWebTest) {
	webResourceRecord(record, &test.admission)
	record.ParseFailed = test.admission.ParseFailed
	record.Error = test.admission.Error
}

func webResourceRecord(record, admission *runner.Record) {
	record.NeedsLinux = admission.NeedsLinux
	record.ExclusiveGroup = admission.ExclusiveGroup
	record.SkipReason = admission.SkipReason
}
