// Related: check.go -- the drift gate these tests call as a function
// Related: sync.go -- the writing half, driven the same way
//
// Every test here CALLS the tool. The scripts these replace could only be
// driven as subprocesses, because the ignore build constraint left their
// package with no non-test file in it (spec-le-is-a-ze-binary, AC-5). What that
// tag cost is visible
// in the registry tests below: a fake transport can now prove the drift
// comparison makes no HTTP request, which the subprocess test could only
// approximate by making the network unreachable and reading the output.

package vendorweb

import (
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/registry"
)

// vendorFiles is every source the sync's tables name, with the content the
// fixture gives it. A fixture carrying all of them syncs with no warning, which
// is what makes a warning in a test mean something.
var vendorFiles = map[string]string{
	"third_party/web/htmx/htmx.min.js":                "// htmx core\n",
	"third_party/web/htmx/hx-sse.min.js":              "// htmx sse\n",
	"third_party/web/ze/ze.svg":                       "<svg/>\n",
	"third_party/web/uplot/uPlot.min.js":              "// uplot\n",
	"third_party/web/uplot/uPlot.min.css":             ".uplot{}\n",
	"third_party/web/swagger-ui/swagger-ui.css":       ".swagger{}\n",
	"third_party/web/swagger-ui/swagger-ui-bundle.js": "// swagger\n",
}

// broadAssets are the files every consumer in consumers holds a copy of.
var broadAssets = []string{"htmx.min.js", "hx-sse.min.js", "ze.svg", "uPlot.min.js", "uPlot.min.css"}

// targetedConsumer is the one consumer the swagger-ui package reaches.
const targetedConsumer = "internal/component/api/rest/assets"

// versionlessManifest names both htmx files and carries no version, so
// checkVersion returns before it queries the registry.
const versionlessManifest = "| File | Package |\n|------|---------|\n| htmx.min.js | htmx.org |\n"

// versionedManifest CARRIES a version, so a query WOULD be made if the code
// reached one. It is what makes the no-network test discriminate.
const versionedManifest = "| Asset | Version |\n|-------|---------|\n| htmx.min.js | 4.0.0 |\n"

// fixture builds a tree shaped like this repository's vendoring: one source
// copy per vendored file and one consumer copy per subscriber, all matching.
func fixture(t *testing.T) string {
	t.Helper()

	root := t.TempDir()

	writeFile(t, root, "go.mod", "module example.test/vendorweb\n\ngo 1.26\n")
	writeFile(t, root, "third_party/web/MANIFEST.md", versionlessManifest)

	for rel, body := range vendorFiles {
		writeFile(t, root, rel, body)
	}

	for _, consumer := range consumers {
		for _, name := range broadAssets {
			writeFile(t, root, filepath.Join(consumer, name), vendorFiles["third_party/web/"+vendorDirOf(name)+"/"+name])
		}
	}

	for _, a := range targetedAssets {
		writeFile(t, root, filepath.Join(a.dest, a.name), vendorFiles[filepath.Join(a.srcDir, a.name)])
	}

	return root
}

// vendorDirOf answers the vendor package directory a broad asset comes from,
// read from the sync's own table so the fixture cannot disagree with it.
func vendorDirOf(name string) string {
	for _, a := range assets {
		if a.name == name {
			return filepath.Base(a.srcDir)
		}
	}
	return ""
}

func writeFile(t *testing.T, root, rel, body string) {
	t.Helper()

	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("create the fixture directory for %s: %v", rel, err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write the fixture file %s: %v", rel, err)
	}
}

func readFile(t *testing.T, root, rel string) string {
	t.Helper()

	body, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(body)
}

func removePath(t *testing.T, root, rel string) {
	t.Helper()

	if err := os.RemoveAll(filepath.Join(root, rel)); err != nil {
		t.Fatalf("remove %s: %v", rel, err)
	}
}

// problemsOf answers the problems of one kind, so an assertion names what it is
// about instead of indexing into a list.
func problemsOf(report CheckReport, kind string) []Problem {
	var out []Problem
	for _, problem := range report.Problems {
		if problem.Kind == kind {
			out = append(out, problem)
		}
	}
	return out
}

// VALIDATES: the drift gate reports a consumer copy that differs from its
// source, names it, and answers a non-zero verdict.
// PREVENTS: a vendoring guard that prints DRIFT and returns success. ze then
// serves an asset that is in no consumer's source of truth, and every gate
// reading this verdict stays green while it happens.
//
// This is what TestDriftCheckExitsNonZeroOnMismatch (scripts/vendor) proved by
// reading an exit status. The proof is the same and the seam is closer: the
// verdict is the error Check answers, and AnswerCheck turns it into the status.
func TestCheckReportsDriftAndNamesTheCopy(t *testing.T) {
	const drifted = "internal/component/web/assets/htmx.min.js"

	root := fixture(t)
	writeFile(t, root, drifted, "// an edited consumer copy\n")

	report, err := Check(root, false)
	if err == nil {
		t.Fatalf("Check accepted a drifted consumer copy; a drift gate must fail closed\n%s", report.Text())
	}

	found := problemsOf(report, ProblemDrift)
	if len(found) != 1 {
		t.Fatalf("Check reported %d DRIFT problems, want 1: %v", len(found), report.Problems)
	}
	if found[0].File != drifted {
		t.Errorf("the DRIFT problem names %q, want %q", found[0].File, drifted)
	}
	if !strings.Contains(report.Text(), "DRIFT: "+drifted) {
		t.Errorf("the rendering does not name the drifted copy:\n%s", report.Text())
	}
}

// VALIDATES: a consumer that subscribes to a package and holds no copy of one
// of its files is a problem, not an absence nobody looks at.
// PREVENTS: an asset the sync was never told to write staying invisible.
func TestCheckReportsAMissingConsumerCopy(t *testing.T) {
	const missing = "internal/component/lg/assets/hx-sse.min.js"

	root := fixture(t)
	removePath(t, root, missing)

	report, err := Check(root, false)
	if err == nil {
		t.Fatal("Check accepted a consumer missing one file of a package it subscribes to")
	}

	found := problemsOf(report, ProblemMissing)
	if len(found) != 1 || found[0].File != missing {
		t.Errorf("Check reported %v, want one MISSING for %s", report.Problems, missing)
	}
}

// VALIDATES: a vendored package that reaches no consumer is reported.
// PREVENTS: a new third_party/web/ directory nobody wired into the sync sitting
// there unread. The subscription rule alone would say nothing about it.
func TestCheckReportsAVendorPackageNoConsumerHolds(t *testing.T) {
	root := fixture(t)
	writeFile(t, root, "third_party/web/orphan/orphan.js", "// nobody embeds this\n")

	report, err := Check(root, false)
	if err == nil {
		t.Fatal("Check accepted a vendored package that reaches no consumer")
	}

	found := problemsOf(report, ProblemUnsynced)
	if len(found) != 1 || found[0].File != filepath.Join(vendorDir, "orphan") {
		t.Errorf("Check reported %v, want one UNSYNCED for third_party/web/orphan", report.Problems)
	}
}

// VALIDATES: one file name vendored twice is refused, because a consumer copy is
// matched by NAME and an ambiguous name has no single source.
// PREVENTS: a comparison that silently picks whichever package the walk reached
// first, so a copy is gated against a source nobody chose.
func TestCheckRefusesAnAmbiguousVendorFileName(t *testing.T) {
	root := fixture(t)
	writeFile(t, root, "third_party/web/other/htmx.min.js", "// a second source of one name\n")

	_, err := Check(root, false)
	if err == nil {
		t.Fatal("Check accepted one file name vendored in two packages")
	}
	if !strings.Contains(err.Error(), "vendored twice") {
		t.Errorf("Check refused with %q, which does not say the name is ambiguous", err)
	}
}

// VALIDATES: the two populations are FAIL-CLOSED. A tree with no vendored
// package, and a tree with no consumer directory, are each refused rather than
// reported as clean.
// PREVENTS: the gate passing because it read nothing. That is the shape this
// whole check exists to catch, applied to itself.
func TestCheckRefusesAnEmptyPopulation(t *testing.T) {
	cases := []struct {
		name string
		// empty leaves the directory in place and takes its content away, so
		// the run reaches the population guard rather than the read error one
		// directory above it.
		empty func(t *testing.T, root string)
		want  string
	}{
		{
			name: "no vendored package",
			empty: func(t *testing.T, root string) {
				for dir := range vendorFiles {
					removePath(t, root, filepath.Dir(dir))
				}
			},
			want: "holds no vendored package",
		},
		{
			name: "no consumer directory",
			empty: func(t *testing.T, root string) {
				for _, consumer := range consumers {
					removePath(t, root, consumer)
				}
				removePath(t, root, targetedConsumer)
			},
			want: "holds no assets/ directory",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := fixture(t)
			tc.empty(t, root)

			_, err := Check(root, false)
			if err == nil {
				t.Fatalf("Check accepted a tree with no %s", tc.name)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("Check refused with %q, want a message saying %q", err, tc.want)
			}
		})
	}
}

// fakeRegistry answers every request without a socket, and counts them.
type fakeRegistry struct {
	calls  int
	status int
	body   string
	err    error
}

func (f *fakeRegistry) RoundTrip(*http.Request) (*http.Response, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return &http.Response{
		StatusCode: f.status,
		Status:     http.StatusText(f.status),
		Body:       io.NopCloser(strings.NewReader(f.body)),
		Header:     http.Header{},
	}, nil
}

// useRegistry installs a transport for one test and takes it out again.
func useRegistry(t *testing.T, rt http.RoundTripper) {
	t.Helper()

	registryClient.Transport = rt
	t.Cleanup(func() { registryClient.Transport = nil })
}

// VALIDATES: the drift comparison queries no registry. With --updates absent,
// NO HTTP request is made at all, over a manifest that carries a version and so
// would produce one if the code reached it.
// PREVENTS: a commit gate whose verdict depends on npm being up and reachable.
// A gate that cannot run offline turns every airgapped checkout, every CI
// sandbox with no egress and every train journey into a red verify that no edit
// in the tree explains.
//
// TestDriftCheckNeedsNoNetwork (scripts/vendor) proved this by pointing the
// child process at a closed port and reading its output for registry lines. The
// transport counts the requests instead, so an absent line and an absent
// request are no longer the same evidence.
func TestCheckMakesNoRegistryCallWithoutUpdates(t *testing.T) {
	fake := &fakeRegistry{status: http.StatusOK, body: `{"version":"9.9.9"}`}
	useRegistry(t, fake)

	root := fixture(t)
	writeFile(t, root, "third_party/web/MANIFEST.md", versionedManifest)

	report, err := Check(root, false)
	if err != nil {
		t.Fatalf("Check failed over a clean tree: %v\n%s", err, report.Text())
	}
	if fake.calls != 0 {
		t.Errorf("the drift comparison made %d registry requests, want 0", fake.calls)
	}
	if report.Updates != nil {
		t.Errorf("the drift comparison answered a registry report: %v", report.Updates)
	}

	// The control: the same tree WITH the flag does reach the registry, so the
	// zero above is a property of the code rather than of the fixture.
	report, err = Check(root, true)
	if err != nil {
		t.Fatalf("Check --updates failed over a clean tree: %v", err)
	}
	if fake.calls != 1 {
		t.Errorf("the registry query made %d requests, want 1", fake.calls)
	}
	if report.Updates == nil {
		t.Fatal("Check --updates answered no registry report")
	}
}

// VALIDATES: each of the four outcomes the registry query can reach renders the
// line the script rendered for it.
// PREVENTS: an upgrade nobody sees. The report is read by a person deciding
// whether to vendor a new release, so "up to date" printed over a stale version
// is the whole failure.
func TestUpdateReportRendersEachOutcome(t *testing.T) {
	cases := []struct {
		name     string
		manifest string
		fake     *fakeRegistry
		want     string
	}{
		{
			name:     "no version in the manifest",
			manifest: versionlessManifest,
			fake:     &fakeRegistry{status: http.StatusOK, body: `{"version":"4.0.0"}`},
			want:     "  htmx.org: version not found in MANIFEST.md\n",
		},
		{
			name:     "the registry did not answer",
			manifest: versionedManifest,
			fake:     &fakeRegistry{err: errors.New("dial tcp: refused")},
			want:     "  htmx.org: could not fetch latest version (",
		},
		{
			name:     "up to date",
			manifest: versionedManifest,
			fake:     &fakeRegistry{status: http.StatusOK, body: `{"version":"4.0.0"}`},
			want:     "  htmx.org: 4.0.0 (up to date)\n",
		},
		{
			name:     "an upgrade is available",
			manifest: versionedManifest,
			fake:     &fakeRegistry{status: http.StatusOK, body: `{"version":"4.1.0"}`},
			want:     "  htmx.org: 4.0.0 -> 4.1.0 available\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			useRegistry(t, tc.fake)

			root := fixture(t)
			writeFile(t, root, "third_party/web/MANIFEST.md", tc.manifest)

			report, err := Check(root, true)
			if err != nil {
				t.Fatalf("Check --updates failed: %v", err)
			}
			if !strings.Contains(report.Text(), tc.want) {
				t.Errorf("the rendering does not hold %q:\n%s", tc.want, report.Text())
			}
		})
	}
}

// VALIDATES: an unreadable MANIFEST.md stops the update report before the drift
// comparison, and the answer says the comparison never ran.
// PREVENTS: a report that prints the consumer-copy heading and then nothing,
// which reads as a clean tree.
func TestUpdateReportStopsWhenTheManifestCannotBeRead(t *testing.T) {
	useRegistry(t, &fakeRegistry{status: http.StatusOK, body: `{"version":"4.0.0"}`})

	root := fixture(t)
	removePath(t, root, "third_party/web/MANIFEST.md")

	report, err := Check(root, true)
	if err == nil {
		t.Fatal("Check --updates accepted a tree with no MANIFEST.md")
	}
	if report.DriftChecked {
		t.Error("the report says the consumer-copy comparison ran, and it did not")
	}
	if report.Text() != "" {
		t.Errorf("the report rendered a run that never happened:\n%s", report.Text())
	}
}

// VALIDATES: the sync restores a consumer copy that no longer matches its
// third_party/web/ source, and says which file it wrote.
// PREVENTS: consumer copies staying hand-maintained. //go:embed cannot reach
// outside its own package, so one library is vendored once per consumer, and a
// copy nobody regenerates is a copy that quietly diverges.
//
// This is the-sync-restores-an-edited-copy (scripts/vendor/sync_web_test.go) as
// a function call. The old case read the file back after a `go run`; this one
// reads it back after Sync, and also reads the report, which the subprocess
// could only expose as text.
func TestSyncRestoresAnEditedConsumerCopy(t *testing.T) {
	const edited = "internal/component/web/assets/htmx.min.js"

	root := fixture(t)
	writeFile(t, root, edited, "// an edited consumer copy\n")

	report, err := Sync(root)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	want := vendorFiles["third_party/web/htmx/htmx.min.js"]
	if got := readFile(t, root, edited); got != want {
		t.Errorf("%s was not restored\n got: %q\nwant: %q", edited, got, want)
	}
	if report.Changed != 1 {
		t.Errorf("Sync reported %d files changed, want 1: %v", report.Changed, report.Files)
	}
	if !strings.Contains(report.Text(), "synced: "+filepath.Join(root, edited)) {
		t.Errorf("the rendering does not name the file it wrote:\n%s", report.Text())
	}
}

// VALIDATES: the sync writes a consumer copy that is absent, and the check it
// is twinned with then passes over the same tree.
// PREVENTS: the two halves disagreeing about what a synced tree is. That
// disagreement is invisible until a release: the sync exits 0 and the gate
// exits 1 on the same checkout.
func TestSyncWritesAMissingCopyAndTheCheckThenPasses(t *testing.T) {
	const missing = "internal/chaos/web/assets/uPlot.min.css"

	root := fixture(t)
	removePath(t, root, missing)

	if _, err := Check(root, false); err == nil {
		t.Fatal("the check accepted a tree with a consumer copy missing, so this test proves nothing")
	}

	if _, err := Sync(root); err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	if report, err := Check(root, false); err != nil {
		t.Errorf("the check refused a tree the sync had just written: %v\n%s", err, report.Text())
	}
}

// VALIDATES: a run over a tree already in sync writes nothing and says so.
// PREVENTS: a generator that rewrites every file it visits. `make generate`
// would then dirty the working tree on every run and the commit gate would
// report a change nobody made.
func TestSyncWritesNothingWhenEverythingMatches(t *testing.T) {
	root := fixture(t)

	before := treeDigest(t, root)

	report, err := Sync(root)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	if report.Changed != 0 || len(report.Files) != 0 {
		t.Errorf("Sync acted on %v over a tree already in sync", report.Files)
	}
	if report.Text() != "all consumer copies are up to date\n" {
		t.Errorf("Sync rendered %q", report.Text())
	}
	if got := treeDigest(t, root); got != before {
		t.Error("Sync changed the tree it reported as up to date")
	}
}

// VALIDATES: a consumer directory that is not there is a warning the run
// carries on past, and the warning goes to the stderr reading rather than into
// the report a reader sees on stdout.
// PREVENTS: one absent package stopping the other consumers being synced.
func TestSyncWarnsAboutAMissingConsumerDirectory(t *testing.T) {
	const gone = "internal/chaos/web/assets"

	root := fixture(t)
	removePath(t, root, gone)

	report, err := Sync(root)
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	warnings := report.Warnings()
	if len(warnings) != 1 {
		t.Fatalf("Sync answered %d warnings, want 1: %v", len(warnings), warnings)
	}
	if !strings.Contains(warnings[0], filepath.Join(root, gone)) {
		t.Errorf("the warning does not name the missing directory: %q", warnings[0])
	}
	if strings.Contains(report.Text(), "consumer directory not found") {
		t.Errorf("the warning reached the stdout rendering:\n%s", report.Text())
	}
}

// VALIDATES: a run that could read NO vendored source is an error, not a tree
// reported as up to date.
// PREVENTS: the one fail-open the script this replaces still has. A sync
// pointed at a tree with no third_party/web/ prints "all consumer copies are up
// to date" and exits 0, which is the same answer a genuinely synced tree gets.
// The check half of this pair has refused that shape since 2026-08-15
// ("a run that read nothing has proven nothing"); the sync half never did.
// scripts/vendor/parity_test.go pins the script still failing open, so that
// test goes red the day somebody fixes the script -- and the answer then is to
// delete the script and the test together.
func TestSyncFailsClosedWhenNoVendoredAssetIsReadable(t *testing.T) {
	root := fixture(t)
	removePath(t, root, "third_party/web")

	report, err := Sync(root)
	if err == nil {
		t.Fatalf("Sync reported success having read nothing:\n%s", report.Text())
	}
	if !strings.Contains(err.Error(), "nothing could be synced") {
		t.Errorf("Sync refused with %q, which does not say it synced nothing", err)
	}
	if strings.Contains(report.Text(), "up to date") {
		t.Errorf("the rendering calls the tree up to date:\n%s", report.Text())
	}
}

// VALIDATES: a destination the run cannot write is an error, because the run
// was told to write it. It is not a warning it carries on past.
// PREVENTS: `make generate` exiting 0 having failed to write a file, which
// leaves the drift gate to report it one commit later.
func TestSyncFailsWhenADestinationCannotBeWritten(t *testing.T) {
	const blocked = "internal/component/web/assets/htmx.min.js"

	root := fixture(t)
	removePath(t, root, blocked)
	// A directory standing where the file goes. os.WriteFile refuses it for
	// every user, which a read-only permission bit would not do under root.
	if err := os.MkdirAll(filepath.Join(root, blocked), 0o750); err != nil {
		t.Fatalf("build the blocked destination: %v", err)
	}

	_, err := Sync(root)
	if err == nil {
		t.Fatal("Sync reported success over a destination it could not write")
	}
	if !strings.Contains(err.Error(), blocked) {
		t.Errorf("Sync refused with %q, which does not name the file it could not write", err)
	}
}

// VALIDATES: every action refuses a value typed after it, because the tree is
// the checkout and the rendering is a pipe operator.
// PREVENTS: a positional path silently naming a tree, which is how the two
// halves of this pair could be pointed at different checkouts.
func TestEveryActionRefusesAValue(t *testing.T) {
	for _, row := range Actions().Actions {
		t.Run(row.Verb, func(t *testing.T) {
			payload, code := Answer([]string{row.Verb, "/some/tree"})
			if code == 0 {
				t.Errorf("%s accepted a value", row.Verb)
			}
			if payload != nil {
				t.Errorf("%s answered a payload for a refused call: %v", row.Verb, payload)
			}
		})
	}
}

// VALIDATES: an action this command does not hold is refused with 2, which the
// Python area answered for the same mistake and which a caller can tell apart
// from a gate that ran and failed.
// PREVENTS: a typo reading as a failed check. `le vendor-web chekc` exiting 1
// says the assets drifted, and they did not.
func TestAnUnknownActionIsRefusedApartFromAFailure(t *testing.T) {
	payload, code := Answer([]string{"chekc"})
	if code != 2 {
		t.Errorf("an unknown action answered %d, want 2", code)
	}
	if payload != nil {
		t.Errorf("an unknown action answered a payload: %v", payload)
	}
}

// VALIDATES: the bare command lists its actions, and the listing says which one
// WRITES. The Python area carried that fact as Gate.writes and printed it as
// `writes` or `checks` beside the reason (scripts/le/devtools/gate.py,
// GateSet.render_list).
// PREVENTS: a writing tool that reads like a check. A developer picking an
// action out of a listing has one place to learn that one of the three changes
// the tree, and it must not be a comment.
func TestTheListingSaysWhichActionWrites(t *testing.T) {
	list := Actions()
	if len(list.Actions) != 3 {
		t.Fatalf("the command holds %d actions, want 3: %v", len(list.Actions), list.Actions)
	}

	writers := map[string]bool{}
	for _, row := range list.Actions {
		writers[row.Verb] = row.Writes
		if row.Why == "" {
			t.Errorf("%s carries no reason, so the listing renders it blank", row.Verb)
		}
		if !strings.HasPrefix(row.Gate, "ze-") {
			t.Errorf("%s names gate %q, which is not a Make target", row.Verb, row.Gate)
		}
	}

	want := map[string]bool{"check": false, "sync": true, "update-report": false}
	for verb, writes := range want {
		got, held := writers[verb]
		if !held {
			t.Errorf("the command holds no %q action", verb)
			continue
		}
		if got != writes {
			t.Errorf("%s is marked writes=%v, want %v", verb, got, writes)
		}
	}

	text := list.Text()
	if !strings.Contains(text, "sync") || !strings.Contains(text, "writes") {
		t.Errorf("the listing does not mark the writing action:\n%s", text)
	}
	for line := range strings.SplitSeq(text, "\n") {
		if strings.HasPrefix(line, "  check ") && !strings.Contains(line, "checks") {
			t.Errorf("the listing does not mark the read-only actions:\n%s", text)
		}
	}
}

// VALIDATES: help renders the same writes marker the listing does, because both
// read one table.
// PREVENTS: the two surfaces disagreeing. A Subs line hand-written beside a
// listing is exactly the drift this whole spec exists to remove.
func TestHelpAndTheListingAgreeAboutWhatWrites(t *testing.T) {
	subs := Subs()

	for _, row := range Actions().Actions {
		if !strings.Contains(subs, row.Verb) {
			t.Errorf("the Subs line does not name %q: %q", row.Verb, subs)
		}
		marked := strings.Contains(subs, row.Verb+" (writes)")
		if marked != row.Writes {
			t.Errorf("the Subs line marks %s writes=%v, want %v: %q", row.Verb, marked, row.Writes, subs)
		}
	}

	var registered registry.Meta
	for _, root := range registry.ListRoot() {
		if root.Name == area {
			registered = root.Meta
		}
	}
	if registered.Description == "" {
		t.Fatalf("%s registered no description, so help renders it blank", area)
	}
	if registered.ResolveSubs() != subs {
		t.Errorf("help renders subs %q and the table says %q", registered.ResolveSubs(), subs)
	}
}

// VALIDATES: the command declares the shape of its answer, so the row operators
// act on the rows instead of being refused.
// PREVENTS: `le vendor-web check | match DRIFT` being turned away over an
// answer that does carry rows.
func TestTheCommandDeclaresItsAnswerShape(t *testing.T) {
	shape, declared := command.ShapeForCommand(area)
	if !declared {
		t.Fatalf("%s declares no answer shape", area)
	}
	if shape != command.ShapeMap {
		t.Errorf("%s declares shape %v, want ShapeMap", area, shape)
	}
}

// treeDigest answers a stable description of every file under root, so a test
// can assert a tree did not move without naming each file.
func treeDigest(t *testing.T, root string) string {
	t.Helper()

	var out strings.Builder

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		body, readErr := os.ReadFile(path) //nolint:gosec // a fixture tree this test built
		if readErr != nil {
			return readErr
		}
		out.WriteString(rel)
		out.WriteString("\x00")
		out.Write(body)
		out.WriteString("\x00")
		return nil
	})
	if err != nil {
		t.Fatalf("walk the fixture tree: %v", err)
	}

	return out.String()
}
