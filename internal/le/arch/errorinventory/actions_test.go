// VALIDATES: the wiring of le arch error-inventory (AC-1, AC-5 first case,
// AC-6): the registered report action, run the way the binary runs it
// (leroot.Run), lists the fixture tree's one site per surface, among them
// the plain web refusal as a row with its surface, file and line, owner
// package, door, status and pre-rank, and one count line per surface, and
// `| json` answers the same rows through the shared renderer; and the
// change-set judge refuses a change adding a plain
// http.Error in a web package, naming its file and line, its status and the
// fix.
// PREVENTS: an error-inventory command that exists and answers nothing, a
// private JSON mode beside the shared renderer, and a check that admits a
// new plain web refusal.
package archerrorinventory

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/commit/changecheck"
	leroot "github.com/ze-software/ze/internal/le/le/root"
)

// fixtureTree is the testdata checkout the action tests point the command at.
const fixtureTree = "testdata/tree"

// fixtureWebSite is the fixture's one web refusal, at its file and line.
const fixtureWebSite = "internal/component/web/refuse.go:7"

// useFixtureTree points both actions at the testdata tree for one test.
// Tests that call it MUST NOT run in parallel: checkoutRoot is package state.
func useFixtureTree(t *testing.T) string {
	t.Helper()
	return useTree(t, fixtureTree)
}

// useTree points both actions at the testdata tree dir for one test, with no
// floor: a fixture holds a handful of sites, not the checkout's thousands.
// Tests that call it MUST NOT run in parallel: checkoutRoot and
// checkoutFloors are package state.
func useTree(t *testing.T, dir string) string {
	t.Helper()
	tree, err := filepath.Abs(dir)
	if err != nil {
		t.Fatalf("fixture tree: %v", err)
	}
	savedRoot, savedFloors := checkoutRoot, checkoutFloors
	checkoutRoot = func() (string, error) { return tree, nil }
	checkoutFloors = Floors{}
	t.Cleanup(func() { checkoutRoot, checkoutFloors = savedRoot, savedFloors })
	return tree
}

// runRegistered runs the registered command with args the way the le binary
// does, and answers its stdout and exit code.
func runRegistered(t *testing.T, args ...string) (string, int) {
	t.Helper()
	handler := leroot.LookupCommand(area)
	if handler == nil {
		t.Fatalf("le %s is not registered", area)
	}
	var out, errOut strings.Builder
	code := leroot.Run(area, handler, args, &out, &errOut)
	if errOut.Len() > 0 {
		t.Logf("le %s %s stderr:\n%s", area, strings.Join(args, " "), errOut.String())
	}
	return out.String(), code
}

// fixtureCounts are the count lines the fixture tree answers: one site per
// surface. The daemon-command site is also a gRPC status writer call, so the
// grpc count holds only the tree's other gRPC site (AC-2).
var fixtureCounts = []string{
	"surface=daemon-command sites=1 typed=0 plain-names-command=1 plain=0",
	"surface=config-verify sites=1 typed=0 plain-names-command=0 plain=1",
	"surface=log sites=1 typed=0 plain-names-command=0 plain=1",
	"surface=doctor sites=1 typed=1 plain-names-command=0 plain=0",
	"surface=local-cli sites=1 typed=0 plain-names-command=0 plain=1",
	"surface=mcp sites=1 typed=0 plain-names-command=0 plain=1",
	"surface=rest sites=1 typed=0 plain-names-command=0 plain=1",
	"surface=web sites=1 typed=0 plain-names-command=0 plain=1",
	"surface=gnmi sites=1 typed=0 plain-names-command=0 plain=1",
	"surface=grpc sites=1 typed=0 plain-names-command=0 plain=1",
	"surface=le sites=1 typed=0 plain-names-command=0 plain=1",
}

// TestErrorInventoryReportThroughAction is AC-1 and AC-6 through the
// registered action: over the fixture tree, which holds one site per
// surface from all three sources, `report` prints the web refusal as a row
// with its file and line, surface, status, pre-rank, owner package, door,
// writer and message, and the exact count line of every surface in order;
// and `report | json` answers the same rows as JSON, one per surface.
func TestErrorInventoryReportThroughAction(t *testing.T) {
	useFixtureTree(t)

	text, code := runRegistered(t, "report")
	if code != 0 {
		t.Fatalf("report exited %d:\n%s", code, text)
	}
	for _, want := range []string{
		fixtureWebSite,
		"surface=web",
		"status=plain",
		"pre-rank=1",
		"package=internal/component/web",
		"door=ai/doors/api-surfaces.md",
		"http.Error",
		`"unauthorized"`,
	} {
		if !strings.Contains(text, want) {
			t.Errorf("report does not print %q:\n%s", want, text)
		}
	}
	countBlock := "\n" + strings.Join(fixtureCounts, "\n") + "\n"
	if !strings.Contains(text, countBlock) {
		t.Errorf("report does not print the count lines in order:\n%s\nwant:%s", text, countBlock)
	}

	raw, code := runRegistered(t, "report", "|", "json")
	if code != 0 {
		t.Fatalf("report | json exited %d:\n%s", code, raw)
	}
	var decoded struct {
		Rows []struct {
			Surface string   `json:"surface"`
			File    string   `json:"file"`
			Line    int      `json:"line"`
			Package string   `json:"package"`
			Doors   []string `json:"doors"`
			Status  string   `json:"status"`
			PreRank int      `json:"pre-rank"`
		} `json:"rows"`
		Surfaces []struct {
			Surface string `json:"surface"`
			Sites   int    `json:"sites"`
		} `json:"surfaces"`
	}
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		t.Fatalf("report | json is not one JSON document: %v\n%s", err, raw)
	}
	if len(decoded.Rows) != len(fixtureCounts) {
		t.Fatalf("report | json rows %d, want %d:\n%s", len(decoded.Rows), len(fixtureCounts), raw)
	}
	webRows := 0
	for _, row := range decoded.Rows {
		if row.Surface != "web" {
			continue
		}
		webRows++
		if row.File != "internal/component/web/refuse.go" || row.Line != 7 {
			t.Errorf("web row is %s:%d, want %s", row.File, row.Line, fixtureWebSite)
		}
		if row.Status != "plain" || row.PreRank != 1 {
			t.Errorf("web row status %s pre-rank %d, want plain 1", row.Status, row.PreRank)
		}
		if row.Package != "internal/component/web" {
			t.Errorf("web row package %s, want internal/component/web", row.Package)
		}
		if len(row.Doors) != 1 || row.Doors[0] != "ai/doors/api-surfaces.md" {
			t.Errorf("web row doors %v, want [ai/doors/api-surfaces.md]", row.Doors)
		}
	}
	if webRows != 1 {
		t.Errorf("report | json web rows %d, want 1:\n%s", webRows, raw)
	}
	if len(decoded.Surfaces) != len(fixtureCounts) {
		t.Fatalf("report | json counts %d surfaces, want %d:\n%s", len(decoded.Surfaces), len(fixtureCounts), raw)
	}
	for i, count := range decoded.Surfaces {
		want := "surface=" + count.Surface + " sites=" + strconv.Itoa(count.Sites) + " "
		if !strings.HasPrefix(fixtureCounts[i], want) {
			t.Errorf("report | json count %d is %s sites=%d, want %s", i, count.Surface, count.Sites, fixtureCounts[i])
		}
	}
}

// addedWebFile is the file the change set adds, in the fixture's web
// package. It is never written to disk: only the After side holds it.
const addedWebFile = "internal/component/web/zz_error_inventory_fixture.go"

// addedWebSource is addedWebFile's content: one plain web refusal, at line 7.
const addedWebSource = "package web\n\n" +
	"import \"net/http\"\n\n" +
	"// refuseAdded is the change set's plain refusal.\n" +
	"func refuseAdded(w http.ResponseWriter) {\n" +
	"\thttp.Error(w, \"unauthorized\", 401)\n" +
	"}\n"

// TestErrorInventoryCheckRefusesAddedPlainSite is AC-5's first case through
// the commit seam's runner (changecheck.Run): a change adding a plain
// http.Error in a web package is refused, at its file and line, with its
// status and the fix on a line of its own.
func TestErrorInventoryCheckRefusesAddedPlainSite(t *testing.T) {
	tree := useFixtureTree(t)

	after := func(path string) (string, bool, error) {
		if path != addedWebFile {
			t.Errorf("the judge read %s, which the change does not carry", path)
		}
		return addedWebSource, true, nil
	}
	before := func(path string) (string, bool, error) {
		if path != addedWebFile {
			t.Errorf("the judge read %s, which the change does not carry", path)
		}
		return "", false, nil
	}
	change := changecheck.Change{Root: tree, Paths: []string{addedWebFile}, Before: before, After: after}
	check := changecheck.Check{Name: area + " check", Judge: judgeChange}

	refusals, err := changecheck.Run(change, []changecheck.Check{check})
	if err != nil {
		t.Fatalf("the check could not judge: %v", err)
	}
	if len(refusals) != 1 {
		t.Fatalf("refusals %d, want 1: %+v", len(refusals), refusals)
	}
	text := refusals[0].Text
	for _, want := range []string{addedWebFile + ":7", "web", "plain", "http.Error", `"unauthorized"`, "\nFix:", "opererr"} {
		if !strings.Contains(text, want) {
			t.Errorf("refusal does not name %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, fixtureWebSite) {
		t.Errorf("refusal names the site already in the tree:\n%s", text)
	}
}

// changedHandlers is the fixture's handler file as the change set leaves it:
// the registered handler gains a plain fmt.Errorf return and a plain gRPC
// status return. Both are operator-error sites, and the second is also a
// writer call on the handler's return line.
const changedHandlers = "// Fixture: the daemon-command and config-verify sites.\n" +
	"package fixture\n\n" +
	"import (\n" +
	"\t\"errors\"\n" +
	"\t\"fmt\"\n\n" +
	"\t\"github.com/ze-software/ze/internal/component/command\"\n" +
	"\t\"github.com/ze-software/ze/internal/component/plugin\"\n" +
	"\t\"github.com/ze-software/ze/internal/component/plugin/rpc\"\n" +
	"\tpluginserver \"github.com/ze-software/ze/internal/component/plugin/server\"\n" +
	"\t\"google.golang.org/grpc/codes\"\n" +
	"\t\"google.golang.org/grpc/status\"\n" +
	")\n\n" +
	"// handlePeer is the daemon-command site.\n" +
	"func handlePeer(ctx *pluginserver.CommandContext, _ command.ValidatedArgs) (*plugin.Response, error) {\n" +
	"\tif ctx == nil {\n" +
	"\t\treturn nil, fmt.Errorf(\"peer context missing\")\n" +
	"\t}\n" +
	"\tif ctx.Peer == \"\" {\n" +
	"\t\treturn nil, status.Error(codes.PermissionDenied, \"peer refused\")\n" +
	"\t}\n" +
	"\treturn nil, status.Error(codes.NotFound, \"peer missing: run ze show bgp peer\") // site:overlap\n" +
	"}\n\n" +
	"// verifyFixture is the config-verify site.\n" +
	"func verifyFixture(_ []rpc.ConfigSection) error {\n" +
	"\treturn errors.New(\"fixture section invalid\")\n" +
	"}\n"

// changedWatch is the fixture's log file as the change set leaves it: an
// added error log line that carries a plain gRPC status call on its own line,
// so the writer scan names the log site's line too.
const changedWatch = "// Fixture: the log site, a warning that names no doctor code.\n" +
	"package watch\n\n" +
	"import (\n" +
	"\t\"log/slog\"\n\n" +
	"\t\"google.golang.org/grpc/codes\"\n" +
	"\t\"google.golang.org/grpc/status\"\n" +
	")\n\n" +
	"// Watch logs the fixture's warning and its added error.\n" +
	"func Watch(log *slog.Logger) {\n" +
	"\tlog.Warn(\"watch: peer flapping\")\n" +
	"\tlog.Error(\"watch: peer refused\", \"error\", status.Error(codes.Internal, \"refused\"))\n" +
	"}\n"

// The carried fixture files the R-6 change set edits.
const (
	handlersFile = "internal/plugins/fixture/handlers.go"
	watchFile    = "internal/component/watch/watch.go"
	webFile      = "internal/component/web/refuse.go"
)

// treeSides answers a change's Before side, which reads each path from the
// fixture tree on disk, and its After side, which reads the path from
// changed when it holds it and from disk otherwise. A path neither holds is
// a path the change does not carry: the test fails on it.
func treeSides(t *testing.T, tree string, changed map[string]string, carried []string) (changecheck.Side, changecheck.Side) {
	t.Helper()
	isCarried := func(path string) bool { return slices.Contains(carried, path) }
	fromDisk := func(path string) (string, bool, error) {
		data, err := os.ReadFile(filepath.Join(tree, path))
		if errors.Is(err, fs.ErrNotExist) {
			return "", false, nil
		}
		if err != nil {
			return "", false, err
		}
		return string(data), true, nil
	}
	before := func(path string) (string, bool, error) {
		if !isCarried(path) {
			t.Errorf("the judge read %s, which the change does not carry", path)
		}
		return fromDisk(path)
	}
	after := func(path string) (string, bool, error) {
		if !isCarried(path) {
			t.Errorf("the judge read %s, which the change does not carry", path)
		}
		if text, held := changed[path]; held {
			return text, true, nil
		}
		return fromDisk(path)
	}
	return before, after
}

// judgeOnce runs judgeChange through the commit seam's runner on one change.
func judgeOnce(t *testing.T, change changecheck.Change) []changecheck.Refusal {
	t.Helper()
	check := changecheck.Check{Name: area + " check", Judge: judgeChange}
	refusals, err := changecheck.Run(change, []changecheck.Check{check})
	if err != nil {
		t.Fatalf("the check could not judge: %v", err)
	}
	return refusals
}

// VALIDATES: AC-5 second and third cases, and R-6: a change that adds a
// plain fmt.Errorf return and a plain gRPC status return to a registered
// command handler, and an error log line carrying a plain gRPC status call,
// is not refused for any of them, because the operator-error and
// doctor-coverage checks judge those lines; the same change's added plain
// web refusal still is, so the judge looked.
// PREVENTS: a check that refuses a site another gate already judges, and a
// test that passes because the judge found nothing at all.
func TestErrorInventoryCheckLeavesOtherGatesSites(t *testing.T) {
	tree := useFixtureTree(t)

	carried := []string{handlersFile, watchFile, addedWebFile}
	changed := map[string]string{handlersFile: changedHandlers, watchFile: changedWatch, addedWebFile: addedWebSource}
	before, after := treeSides(t, tree, changed, carried)
	refusals := judgeOnce(t, changecheck.Change{Root: tree, Paths: carried, Before: before, After: after})
	if len(refusals) != 1 {
		t.Fatalf("refusals %d, want 1 (the added web refusal): %+v", len(refusals), refusals)
	}
	text := refusals[0].Text
	if !strings.Contains(text, addedWebFile+":7") {
		t.Errorf("refusal does not name the added web refusal %s:7:\n%s", addedWebFile, text)
	}
	for _, owned := range []string{handlersFile, watchFile, `"peer refused"`, `"refused"`, "peer context missing"} {
		if strings.Contains(text, owned) {
			t.Errorf("refusal names %q, a site another gate judges:\n%s", owned, text)
		}
	}
	if !strings.Contains(text, " 1 added message site(s)") {
		t.Errorf("refusal counts more than the one added web site:\n%s", text)
	}
}

// movedWeb is the fixture's web refusal as the change set leaves it: the same
// plain http.Error, two lines lower under a new comment.
const movedWeb = "package web\n\n" +
	"import \"net/http\"\n\n" +
	"// refuseUnauthorized is the fixture's one web surface site: a plain refusal.\n" +
	"//\n" +
	"// It moved under a longer comment.\n" +
	"func refuseUnauthorized(w http.ResponseWriter) {\n" +
	"\thttp.Error(w, \"unauthorized\", 401)\n" +
	"}\n"

// VALIDATES: AC-5's survivor: a plain web refusal already in the tree is
// reported by report and never refused by the check, whether the change
// carries its file unchanged or moves the line.
// PREVENTS: a check that refuses the tree's existing plain messages, which
// would refuse every commit until the whole worklist is fixed.
func TestErrorInventoryCheckIgnoresExistingSites(t *testing.T) {
	tree := useFixtureTree(t)

	original, err := os.ReadFile(filepath.Join(tree, webFile))
	if err != nil {
		t.Fatalf("fixture web file: %v", err)
	}
	if !strings.Contains(string(original), `http.Error(w, "unauthorized", 401)`) {
		t.Fatalf("fixture %s no longer holds the plain refusal this test moves:\n%s", webFile, original)
	}
	carried := []string{webFile}
	for name, text := range map[string]string{"unchanged": string(original), "moved": movedWeb} {
		before, after := treeSides(t, tree, map[string]string{webFile: text}, carried)
		refusals := judgeOnce(t, changecheck.Change{Root: tree, Paths: carried, Before: before, After: after})
		if len(refusals) != 0 {
			t.Errorf("%s: refusals %d, want 0: %+v", name, len(refusals), refusals)
		}
	}

	out, code := runRegistered(t, "report")
	if code != 0 {
		t.Fatalf("report exit %d, want 0:\n%s", code, out)
	}
	if !strings.Contains(out, fixtureWebSite+" surface=web status=plain") {
		t.Errorf("report does not list the existing plain web refusal %s:\n%s", fixtureWebSite, out)
	}
}
