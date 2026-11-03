// Design: docs/architecture/testing/runner-architecture.md -- web discovery admission.
package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/test/runner"
)

// TestWebDiscoverySnapshot removes the authored file after discovery: admission
// and execution must still use the same parsed skip, without starting a server.
func TestWebDiscoverySnapshot(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "snapshot.wb")
	body := "option=skip:reason=snapshot\noption=needs-linux\noption=exclusive:group=web-snapshot\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	tests := runner.NewTestSet[*zeTestWebTest]()
	if err := discoverWebTests(tests, dir, dir, 1); err != nil {
		t.Fatal(err)
	}
	if tests.Count() != 1 {
		t.Fatalf("discovered %d cases", tests.Count())
	}
	tests.EnableAll()
	test := tests.Selected()[0]
	if !test.admission.NeedsLinux || test.admission.ExclusiveGroup != "web-snapshot" {
		t.Fatalf("resource declarations lost: %+v", &test.admission)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if ok, err := zeTestRunWebTest(context.Background(), test, zeTestWebBinaries{}, 1); !ok || err != nil {
		t.Fatalf("execution reread the authored path: passed=%v err=%v", ok, err)
	}
}

// TestWebDiscoveryBoundaryOutcomes exercises discovery I/O and validation
// refusals before dependency lookup, retaining only executable snapshots.
func TestWebDiscoveryBoundaryOutcomes(t *testing.T) {
	dir := t.TempDir()
	for _, item := range []struct {
		name, body  string
		concurrency int
		failed      bool
	}{
		{"plugin-web", "option=plugin:name=fakeenrich\n", 1, true},
		{"plugin-daemon", "option=server:kind=daemon\noption=plugin:name=fakeenrich\n", 1, false},
		{"overflow", "option=timeout:value=" + time.Duration(1<<63-1).String() + "\n", 4, true},
		{"bad-concurrency", "action=wait:ms=1\n", 0, true},
	} {
		path := filepath.Join(dir, item.name+".wb")
		if err := os.WriteFile(path, []byte(item.body), 0o600); err != nil {
			t.Fatal(err)
		}
		found := discoverWebCase(path, item.concurrency)
		if found.admission.ParseFailed != item.failed {
			t.Fatalf("%s: failed=%t error=%v", item.name, found.admission.ParseFailed, found.GetError())
		}
	}
	for _, root := range []string{filepath.Join(dir, "missing"), dir} {
		tests := runner.NewTestSet[*zeTestWebTest]()
		if err := discoverWebTests(tests, root, "relative-base", 1); err == nil {
			t.Fatalf("invalid discovery root/base succeeded: %s", root)
		}
	}
	failed := discoverWebCase(filepath.Join(dir, "missing.wb"), 1)
	skipped := &zeTestWebTest{admission: runner.Record{SkipReason: "owned"}}
	valid := discoverWebCase(filepath.Join(dir, "plugin-daemon.wb"), 1)
	runnable := webRunnableTests([]*zeTestWebTest{failed, skipped, valid})
	if len(runnable) != 1 || runnable[0] != valid {
		t.Fatalf("runnable selection retained an invalid case: %v", runnable)
	}
	for _, item := range []struct {
		test        *zeTestWebTest
		concurrency int
		cause       string
	}{{&zeTestWebTest{}, 1, "not discovered"}, {valid, 0, "concurrency"}} {
		passed, err := zeTestRunWebTest(context.Background(), item.test, zeTestWebBinaries{}, item.concurrency)
		if passed || err == nil || !strings.Contains(err.Error(), item.cause) {
			t.Fatalf("invalid callback input reached startup: passed=%t err=%v", passed, err)
		}
	}
}

// TestWebDiscoveryParsePrecedence gives discovery both a valid skip and a
// malformed capability declaration. Dependency lookup must never hide the error.
func TestWebDiscoveryParsePrecedence(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "broken.wb")
	if err := os.WriteFile(path, []byte("option=skip:reason=example\noption=exclusive:group=held\noption=needs-linux:caps=net-admn\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	tests := runner.NewTestSet[*zeTestWebTest]()
	if err := discoverWebTests(tests, dir, dir, 1); err != nil {
		t.Fatal(err)
	}
	tests.EnableAll()
	test := tests.Selected()[0]
	if !test.admission.ParseFailed {
		t.Fatal("malformed declaration did not set ParseFailed")
	}
	t.Setenv("PATH", t.TempDir())
	if err := webDiscoveryErrors(tests.Selected()); err == nil || !strings.Contains(err.Error(), "net-admn") {
		t.Fatalf("missing dependency hid parse cause: %v", err)
	}
	pr := runner.NewParallelRunner[*zeTestWebTest](runner.NewColors())
	pr.SetQuiet(true)
	record := pr.AddTestWithNick(test.Name, test.Nick, test, func(context.Context, *zeTestWebTest) (bool, error) {
		t.Error("malformed case reached execution")
		return true, nil
	})
	webAdmissionRecord(record, test)
	if pr.Run(context.Background()) {
		t.Fatal("malformed case passed scheduler")
	}
	if record.State != runner.StateFail || record.Error == nil {
		t.Fatalf("wrong scheduler verdict: %+v", record)
	}
}

// TestWebCommandParseBeforeDependency drives the real command against an owned
// discovery root with no browser on PATH. A malformed skipped file must fail.
func TestWebCommandParseBeforeDependency(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module web-admission-fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	suite := filepath.Join(dir, "test", "web")
	if err := os.MkdirAll(suite, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(suite, "broken.wb"), []byte("option=skip:reason=example\noption=timeout:value=broken\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	t.Setenv("PATH", t.TempDir())
	if err := cmdWebMain([]string{"--all", "--quiet"}); err == nil || !strings.Contains(err.Error(), "timeout") {
		t.Fatalf("command hid malformed file behind missing dependency: %v", err)
	}
}
