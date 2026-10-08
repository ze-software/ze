// Design: reference/README.md -- the non-normative IETF reference tree
// Related: refresh.go -- the refresh these tests call as a function
//
// VALIDATES: the refresh selects BCP and Informational RFCs and active drafts
// for every working-group directory, writes a sorted index, downloads only
// texts the reference tree holds, and reports what changed.
// PREVENTS: a write into rfc/full or rfc/drafts, a partial index after a
// failed fetch, a deleted file, and a category name invented for an unknown code.

package dataietfreference

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// fixtureDay is the date every test passes, so the header is deterministic.
var fixtureDay = time.Date(2026, 10, 8, 12, 0, 0, 0, time.UTC)

// datatracker serves canned answers keyed by the request URI the refresh
// emits, counts every hit, and answers 404 to anything it does not hold, so
// an unexpected URL fails the refresh by name instead of passing in silence.
type datatracker struct {
	mu      sync.Mutex
	answers map[string]string
	failing map[string]int
	hits    map[string]int
}

func newDatatracker() *datatracker {
	return &datatracker{answers: map[string]string{}, failing: map[string]int{}, hits: map[string]int{}}
}

func (d *datatracker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	d.mu.Lock()
	defer d.mu.Unlock()
	uri := r.URL.RequestURI()
	d.hits[uri]++
	if code, ok := d.failing[uri]; ok {
		http.Error(w, "injected failure", code)
		return
	}
	body, ok := d.answers[uri]
	if !ok {
		http.NotFound(w, r)
		return
	}
	if _, err := w.Write([]byte(body)); err != nil {
		panic("BUG: fixture server write: " + err.Error())
	}
}

func (d *datatracker) hitsFor(uri string) int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.hits[uri]
}

func groupURI(wg string) string {
	return "/api/v1/group/group/?acronym=" + wg + "&format=json"
}

func docURI(id, kind string) string {
	return "/api/v1/doc/document/?group=" + id + "&type=" + kind + "&limit=200&format=json"
}

func obsURI(rfc string) string {
	return "/api/v1/doc/relateddocument/?target__name=" + rfc + "&relationship=obs&format=json"
}

func page(next, objects string) string {
	nextJSON := "null"
	if next != "" {
		nextJSON = `"` + next + `"`
	}
	return `{"meta":{"next":` + nextJSON + `},"objects":[` + objects + `]}`
}

func level(code string) string {
	if code == "" {
		return "null"
	}
	return `"/api/v1/name/stdlevelname/` + code + `/"`
}

func doc(name, std, intended, rev, title string, states ...int) string {
	var quoted []string
	for _, s := range states {
		quoted = append(quoted, `"/api/v1/doc/state/`+itoa(s)+`/"`)
	}
	return `{"name":"` + name + `","std_level":` + level(std) + `,"intended_std_level":` +
		level(intended) + `,"rev":"` + rev + `","states":[` + strings.Join(quoted, ",") +
		`],"title":"` + title + `"}`
}

func itoa(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	return itoa(n/10) + string(rune('0'+n%10))
}

func related(sources ...string) string {
	var objects []string
	for _, s := range sources {
		objects = append(objects, `{"source":"/api/v1/doc/document/`+s+`/","target":"/api/v1/doc/document/x/"}`)
	}
	return page("", strings.Join(objects, ","))
}

// fixtureWorld is two working groups, alpha and beta, that between them hit
// every selection branch, a paged answer, a title with embedded newlines, and
// each of the three locations.
func fixtureWorld() *datatracker {
	d := newDatatracker()
	d.answers[groupURI("alpha")] = page("", `{"id":10,"acronym":"alpha"}`)
	d.answers[groupURI("beta")] = page("", `{"id":20,"acronym":"beta"}`)

	d.answers[docURI("10", "rfc")] = page("", strings.Join([]string{
		doc("rfc9001", "bcp", "", "", "Held In rfc/full", 177),
		doc("rfc9002", "inf", "", "", `Title\n   Over\tTwo  Lines`, 177),
		doc("rfc9003", "ps", "", "", "Standards Track Is Excluded", 177),
		doc("rfc9004", "exp", "", "", "Experimental Is Excluded", 177),
	}, ","))
	d.answers[docURI("10", "draft")] = page("", strings.Join([]string{
		doc("draft-ietf-alpha-one", "", "ps", "03", "Alpha One", 1, 150),
		doc("draft-ietf-alpha-two", "", "", "07", "Held In rfc/drafts", 1),
		doc("draft-ietf-alpha-expired", "", "inf", "04", "Expired Is Excluded", 2),
		doc("draft-ietf-alpha-published", "", "ps", "12", "Published Is Excluded", 3),
	}, ","))
	d.answers[docURI("20", "rfc")] = page("/api/v1/doc/document/?format=json&group=20&type=rfc&limit=200&offset=200",
		doc("rfc8001", "inf", "", "", "Beta First Page", 177))
	d.answers["/api/v1/doc/document/?format=json&group=20&type=rfc&limit=200&offset=200"] = page("",
		doc("rfc8002", "bcp", "", "", "Beta Second Page", 177))
	d.answers[docURI("20", "draft")] = page("", "")

	d.answers[obsURI("rfc9001")] = related()
	d.answers[obsURI("rfc9002")] = related()
	d.answers[obsURI("rfc8001")] = related()
	d.answers[obsURI("rfc8002")] = related("rfc9999", "rfc9998")

	d.answers["/rfc/rfc9002.txt"] = "text of rfc9002\n"
	d.answers["/rfc/rfc8001.txt"] = "text of rfc8001\n"
	d.answers["/rfc/rfc8002.txt"] = "text of rfc8002\n"
	d.answers["/archive/id/draft-ietf-alpha-one-03.txt"] = "text of alpha-one 03\n"
	return d
}

// fixtureTree builds a checkout with the two group directories, one RFC the
// owner holds in rfc/full and one draft held in rfc/drafts.
func fixtureTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, root, "reference/ietf/alpha/.keep", "")
	writeFile(t, root, "reference/ietf/beta/.keep", "")
	writeFile(t, root, "rfc/full/rfc9001.txt", "owner copy of rfc9001\n")
	writeFile(t, root, "rfc/drafts/draft-ietf-alpha-two.txt", "owner copy of alpha-two\n")
	return root
}

func writeFile(t *testing.T, root, rel, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, root, rel string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func exists(root, rel string) bool {
	_, err := os.Stat(filepath.Join(root, filepath.FromSlash(rel)))
	return err == nil
}

func serve(t *testing.T, d *datatracker) Sources {
	t.Helper()
	server := httptest.NewServer(d)
	t.Cleanup(server.Close)
	return Sources{Datatracker: server.URL, RFCEditor: server.URL, IDArchive: server.URL, Client: server.Client()}
}

const wantIndex = "# Snapshot of the IETF datatracker taken 2026-10-08: every published ALPHA and BETA RFC\n" +
	"# whose IETF category is BCP or Informational, and every active ALPHA and BETA draft.\n" +
	"# Non-normative. See reference/README.md before citing any of these as a requirement.\n" +
	"# columns: wg\tkind\tstem\tcategory (drafts: intended status)\trevision\tlocation\tobsoleted-by\ttitle\n" +
	"alpha\tdraft\tdraft-ietf-alpha-one\tProposed Standard\t03\treference/ietf/alpha\t\tAlpha One\n" +
	"alpha\tdraft\tdraft-ietf-alpha-two\tunset\t07\trfc/drafts\t\tHeld In rfc/drafts\n" +
	"alpha\trfc\trfc9001\tBCP\t\trfc/full\t\tHeld In rfc/full\n" +
	"alpha\trfc\trfc9002\tInformational\t\treference/ietf/alpha\t\tTitle Over Two Lines\n" +
	"beta\trfc\trfc8001\tInformational\t\treference/ietf/beta\t\tBeta First Page\n" +
	"beta\trfc\trfc8002\tBCP\t\treference/ietf/beta\trfc9998,rfc9999\tBeta Second Page\n"

// TestRefreshWritesIndex proves the whole refresh over a fixture world: the
// group list comes from the directories, only BCP and Informational RFCs and
// active drafts are selected, a paged answer is followed, a title's embedded
// whitespace collapses, location prefers rfc/full then rfc/drafts, and only a
// document held in the reference tree has its text downloaded.
func TestRefreshWritesIndex(t *testing.T) {
	root := fixtureTree(t)
	d := fixtureWorld()

	report, err := Refresh(context.Background(), root, fixtureDay, serve(t, d))
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if got := readFile(t, root, indexRel); got != wantIndex {
		t.Fatalf("INDEX.tsv:\n%s\nwant:\n%s", got, wantIndex)
	}
	for rel, want := range map[string]string{
		"reference/ietf/alpha/rfc9002.txt":              "text of rfc9002\n",
		"reference/ietf/alpha/draft-ietf-alpha-one.txt": "text of alpha-one 03\n",
		"reference/ietf/beta/rfc8001.txt":               "text of rfc8001\n",
		"reference/ietf/beta/rfc8002.txt":               "text of rfc8002\n",
	} {
		if got := readFile(t, root, rel); got != want {
			t.Errorf("%s = %q, want %q", rel, got, want)
		}
	}
	if got := len(report.Added); got != 6 {
		t.Errorf("added = %d (%v), want 6", got, report.Added)
	}
	if len(report.Updated) != 0 || report.Unchanged != 0 || len(report.NoLongerListed) != 0 {
		t.Errorf("report = %+v, want only additions", report)
	}
	if got := len(report.Downloaded); got != 4 {
		t.Errorf("downloaded = %v, want 4 texts", report.Downloaded)
	}
}

// TestRefreshNeverWritesRFCTrees proves a document already held in rfc/full or
// rfc/drafts is neither downloaded nor copied: the owner's file is untouched
// and the reference tree holds no second copy.
func TestRefreshNeverWritesRFCTrees(t *testing.T) {
	root := fixtureTree(t)
	d := fixtureWorld()

	if _, err := Refresh(context.Background(), root, fixtureDay, serve(t, d)); err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	if got := readFile(t, root, "rfc/full/rfc9001.txt"); got != "owner copy of rfc9001\n" {
		t.Errorf("rfc/full/rfc9001.txt rewritten: %q", got)
	}
	if got := readFile(t, root, "rfc/drafts/draft-ietf-alpha-two.txt"); got != "owner copy of alpha-two\n" {
		t.Errorf("rfc/drafts/draft-ietf-alpha-two.txt rewritten: %q", got)
	}
	for _, rel := range []string{"reference/ietf/alpha/rfc9001.txt", "reference/ietf/alpha/draft-ietf-alpha-two.txt"} {
		if exists(root, rel) {
			t.Errorf("%s written beside the owner's copy", rel)
		}
	}
	for _, uri := range []string{"/rfc/rfc9001.txt", "/archive/id/draft-ietf-alpha-two-07.txt"} {
		if n := d.hitsFor(uri); n != 0 {
			t.Errorf("%s fetched %d times, want 0", uri, n)
		}
	}
	entries, err := os.ReadDir(filepath.Join(root, "rfc", "full"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Errorf("rfc/full holds %d files, want the owner's one", len(entries))
	}
}

// TestRefreshSecondRun proves the counts against the previous index: an
// unchanged row is not re-downloaded, a draft whose revision moved is, a file
// no longer selected is reported and kept, never deleted.
func TestRefreshSecondRun(t *testing.T) {
	root := fixtureTree(t)
	d := fixtureWorld()
	sources := serve(t, d)
	if _, err := Refresh(context.Background(), root, fixtureDay, sources); err != nil {
		t.Fatalf("first Refresh: %v", err)
	}
	writeFile(t, root, "reference/ietf/beta/rfc7000.txt", "dropped upstream\n")
	d.mu.Lock()
	d.answers[docURI("10", "draft")] = page("",
		doc("draft-ietf-alpha-one", "", "ps", "04", "Alpha One", 1)+","+
			doc("draft-ietf-alpha-two", "", "", "07", "Held In rfc/drafts", 1))
	d.answers["/archive/id/draft-ietf-alpha-one-04.txt"] = "text of alpha-one 04\n"
	d.mu.Unlock()

	report, err := Refresh(context.Background(), root, fixtureDay, sources)
	if err != nil {
		t.Fatalf("second Refresh: %v", err)
	}
	if got := readFile(t, root, "reference/ietf/alpha/draft-ietf-alpha-one.txt"); got != "text of alpha-one 04\n" {
		t.Errorf("revised draft text = %q, want revision 04", got)
	}
	if n := d.hitsFor("/rfc/rfc8001.txt"); n != 1 {
		t.Errorf("unchanged RFC fetched %d times, want once (first run only)", n)
	}
	if !slices.Equal(report.Updated, []string{"alpha/draft-ietf-alpha-one"}) {
		t.Errorf("updated = %v, want the revised draft", report.Updated)
	}
	if report.Unchanged != 5 || len(report.Added) != 0 {
		t.Errorf("report = %+v, want 5 unchanged, 0 added", report)
	}
	if !slices.Equal(report.NoLongerListed, []string{"reference/ietf/beta/rfc7000.txt"}) {
		t.Errorf("no-longer-listed = %v", report.NoLongerListed)
	}
	if !exists(root, "reference/ietf/beta/rfc7000.txt") {
		t.Error("an unlisted file was deleted")
	}
}

// previousIndex is a well-formed index from an earlier run, so a refresh
// reads past it and reaches the fetch that fails.
const previousIndex = "# an earlier run\n" +
	"alpha\trfc\trfc9002\tInformational\t\treference/ietf/alpha\t\tOld Title\n"

// TestRefreshRefusesMalformedIndex proves a previous index line with the
// wrong column count stops the refresh before any fetch, rather than being
// read as a row with no revision.
func TestRefreshRefusesMalformedIndex(t *testing.T) {
	root := fixtureTree(t)
	writeFile(t, root, indexRel, "not\ta\trow\n")
	d := fixtureWorld()
	if _, err := Refresh(context.Background(), root, fixtureDay, serve(t, d)); err == nil {
		t.Fatal("Refresh accepted a malformed previous index")
	}
	if n := d.hitsFor(groupURI("alpha")); n != 0 {
		t.Errorf("datatracker reached %d times past a malformed index", n)
	}
}

// TestRefreshFailureLeavesIndex proves a failed fetch, of metadata or of a
// text, fails the refresh and leaves the previous index and the tree as they
// were: no partial index, no text from the half that did answer.
func TestRefreshFailureLeavesIndex(t *testing.T) {
	for _, failing := range []string{obsURI("rfc8002"), "/rfc/rfc8002.txt", docURI("20", "draft")} {
		t.Run(failing, func(t *testing.T) {
			root := fixtureTree(t)
			writeFile(t, root, indexRel, previousIndex)
			d := fixtureWorld()
			d.failing[failing] = http.StatusInternalServerError

			if _, err := Refresh(context.Background(), root, fixtureDay, serve(t, d)); err == nil {
				t.Fatal("Refresh succeeded over a failed fetch")
			}
			if got := readFile(t, root, indexRel); got != previousIndex {
				t.Errorf("index rewritten after a failure: %q", got)
			}
			for _, rel := range []string{"reference/ietf/alpha/rfc9002.txt", "reference/ietf/beta/rfc8001.txt"} {
				if exists(root, rel) {
					t.Errorf("%s written by a failed refresh", rel)
				}
			}
		})
	}
}

// TestRefreshRefusesEmptyTree proves a tree with no working-group directory
// is an error, not an empty index.
func TestRefreshRefusesEmptyTree(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, indexRel, "previous index\n")
	if _, err := Refresh(context.Background(), root, fixtureDay, serve(t, newDatatracker())); err == nil {
		t.Fatal("Refresh succeeded with no working group")
	}
}

// TestCategoryName pins every datatracker standard-level code to its name and
// refuses a code it does not know rather than inventing one.
func TestCategoryName(t *testing.T) {
	for code, want := range map[string]string{
		"bcp": "BCP", "inf": "Informational", "ps": "Proposed Standard", "exp": "Experimental",
		"ds": "Draft Standard", "hist": "Historic", "std": "Internet Standard", "unkn": "Unknown", "": "unset",
	} {
		got, err := categoryName(code)
		if err != nil {
			t.Errorf("categoryName(%q): %v", code, err)
			continue
		}
		if got != want {
			t.Errorf("categoryName(%q) = %q, want %q", code, got, want)
		}
	}
	if _, err := categoryName("zz"); err == nil {
		t.Error("categoryName accepted an unknown code")
	}
}

// TestTitleCollapse pins the whitespace rule: every run of spaces, tabs and
// newlines becomes one space, so a title can never break a TSV row.
func TestTitleCollapse(t *testing.T) {
	if got := collapse("  A\n\tB   C \r\n"); got != "A B C" {
		t.Errorf("collapse = %q", got)
	}
}
