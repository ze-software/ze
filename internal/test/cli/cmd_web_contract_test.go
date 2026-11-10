// Design: docs/architecture/testing/runner-architecture.md -- web discovery admission.
// Related: cmd_web_discovery.go -- webAdmissionRecord, the record the scheduler admits from.
//go:build linux

package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/ze-software/ze/internal/test/runner"
)

// VALIDATES: AC-40 for the web adapter. A `.wb` case declaring a capability the
// host lacks reaches the scheduler as SKIPPED with a reason naming the
// capability, and its execution callback never runs. Method: discover a real
// case file, build the scheduler record the way runWebTests does
// (AddTestWithNick, then webAdmissionRecord), run the production parallel
// runner, and read the runner's own verdict from the record.
// PREVENTS: webAdmissionRecord dropping the skip on its way to the scheduler.
// zeTestRunWebTest then answers (true, nil) on test.admission.SkipReason, so
// the case is reported as an ordinary PASS: a result that reads test.admission
// instead of the scheduler's verdict cannot see that.
func TestWebAdapterCapabilityPlacement(t *testing.T) {
	var host runner.Record
	if _, err := runner.ParseResourceOption(&host, "needs-linux:caps=net-admin"); err != nil {
		t.Fatal(err)
	}
	if host.SkipReason == "" {
		t.Skip("the incapable-target case needs a host without effective net-admin, and this process holds it")
	}
	dir := t.TempDir()
	body := "option=server:kind=lg-no-engine\noption=needs-linux:caps=net-admin\naction=wait:ms=1\n"
	if err := os.WriteFile(filepath.Join(dir, "capability.wb"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	tests := runner.NewTestSet[*zeTestWebTest]()
	if err := discoverWebTests(tests, dir, dir, 1); err != nil {
		t.Fatal(err)
	}
	tests.EnableAll()
	if tests.Count() != 1 {
		t.Fatalf("expected one capability case, found %d", tests.Count())
	}
	test := tests.Selected()[0]
	if !strings.Contains(test.admission.SkipReason, "net-admin") {
		t.Fatalf("discovery lost the capability: skip reason %q does not name net-admin", test.admission.SkipReason)
	}

	pr := runner.NewParallelRunner[*zeTestWebTest](runner.NewColors())
	pr.SetQuiet(true)
	var executed atomic.Bool
	record := pr.AddTestWithNick(test.Name, test.Nick, test, func(ctx context.Context, c *zeTestWebTest) (bool, error) {
		executed.Store(true)
		return zeTestRunWebTest(ctx, c, zeTestWebBinaries{}, 1)
	})
	webAdmissionRecord(record, test)
	if !pr.Run(context.Background()) {
		t.Fatalf("a capability-skipped case failed the run: %v", record.Error)
	}

	if executed.Load() {
		t.Errorf("the scheduler ran the execution callback of a case whose capability the host lacks")
	}
	if record.State != runner.StateSkip {
		t.Errorf("scheduler verdict = %v, want StateSkip: the case was reported as an ordinary result", record.State)
	}
	if !strings.Contains(record.SkipReason, "net-admin") {
		t.Errorf("scheduler skip reason %q does not name net-admin", record.SkipReason)
	}
}
