package htmxupgrade

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/le/lepath"
)

// The fixture corpus asks the Go scanner about every entry copied from the
// vendored scanner. Each expected row includes the source location and complete
// message, so a table that remains present but routes to the wrong category is
// still caught.
func TestFixtureCorpusDrivesEveryScannerRule(t *testing.T) {
	for _, rule := range removedAttrs {
		t.Run("removed-attr/"+rule.old, func(t *testing.T) {
			assertFixtureIssue(t, ".html", "safe\n<div "+rule.old+"></div>\n", Issue{
				Line: 2, Category: "removed-attr",
				Message: rule.old + " is removed → " + rule.new,
			})
		})
	}
	for _, rule := range renamedAttrs {
		t.Run("renamed-attr/"+rule.old, func(t *testing.T) {
			assertFixtureIssue(t, ".html", "safe\n<div "+rule.old+"></div>\n", Issue{
				Line: 2, Category: "renamed-attr", Message: rule.old + " → " + rule.new,
			})
		})
	}
	for _, rule := range extensionAttrRenames {
		t.Run("extension/"+rule.old, func(t *testing.T) {
			assertFixtureIssue(t, ".html", "safe\n<div "+rule.old+"></div>\n", Issue{
				Line: 2, Category: "ext", Message: rule.old + " → " + rule.new,
			})
		})
	}
	for _, rule := range allEventRenames {
		t.Run("attribute-event/"+rule.old, func(t *testing.T) {
			name := "hx-on:" + strings.ToLower(rule.old)
			if event, ok := strings.CutPrefix(rule.old, "htmx:"); ok {
				name = "hx-on::" + strings.ToLower(event)
			}
			assertFixtureIssue(t, ".html", "safe\n<div "+name+"=\"run()\"></div>\n", Issue{
				Line: 2, Category: "renamed-event",
				Message: name + ` uses old event name "` + rule.old + `" → use hx-on:` + rule.new,
			})
		})
		t.Run("text-event/"+rule.old, func(t *testing.T) {
			assertFixtureIssue(t, ".js", "safe\nlisten(\""+rule.old+"\")\n", Issue{
				Line: 2, Category: "old-event",
				Message: `old event name "` + rule.old + `" → "` + rule.new + `"`,
			})
		})
	}
	for _, event := range removedEvents {
		t.Run("attribute-removed-event/"+event, func(t *testing.T) {
			name := "hx-on:" + strings.ToLower(event)
			assertFixtureIssue(t, ".html", "safe\n<div "+name+"=\"run()\"></div>\n", Issue{
				Line: 2, Category: "removed-event", Message: name + " uses removed event " + event,
			})
		})
		t.Run("text-removed-event/"+event, func(t *testing.T) {
			assertFixtureIssue(t, ".js", "safe\nlisten(\""+event+"\")\n", Issue{
				Line: 2, Category: "removed-event", Message: `removed event "` + event + `"`,
			})
		})
	}
	for _, rule := range removedJSAPI {
		t.Run("javascript-api/"+rule.old, func(t *testing.T) {
			assertFixtureIssue(t, ".js", "safe\n"+rule.old+" (node)\n", Issue{
				Line: 2, Category: "old-api", Message: rule.old + "() is removed → " + rule.new,
			})
		})
	}
	for _, rule := range configRenames {
		t.Run("renamed-config/"+rule.old, func(t *testing.T) {
			assertFixtureIssue(t, ".js", "safe\nhtmx.config."+rule.old+" = true\n", Issue{
				Line: 2, Category: "renamed-config",
				Message: `config "` + rule.old + `" → "` + rule.new + `"`,
			})
		})
	}
	for _, key := range removedConfig {
		t.Run("removed-config/"+key, func(t *testing.T) {
			assertFixtureIssue(t, ".js", "safe\nhtmx.config."+key+" = true\n", Issue{
				Line: 2, Category: "removed-config", Message: `config "` + key + `" is removed`,
			})
		})
	}
	for _, rule := range removedResponseHeaders {
		t.Run("removed-header/"+rule.old, func(t *testing.T) {
			assertFixtureIssue(t, ".js", "safe\nheaders[\""+rule.old+"\"] = value\n", Issue{
				Line: 2, Category: "removed-header",
				Message: `"` + rule.old + `" is removed → ` + rule.new,
			})
		})
	}

	assertFixtureIssue(t, ".html", "safe\n<div hx-swap=\"show:#box:top\"></div>\n", Issue{
		Line: 2, Category: "swap-syntax",
		Message: "old show:#box:top syntax → use show:top showTarget:#box",
	})
}

func TestIssuesOnOneLineKeepScannerPhaseAndAttributeOrder(t *testing.T) {
	path := writeFixtureFile(t, ".html",
		`<div hx-vars hx-disable sse-connect hx-swap="show:#x:top" hx-target><button hx-get></button></div> htmx.addClass() HX-Trigger-After-Swap`)
	issues, err := checkFile(path)
	if err != nil {
		t.Fatal(err)
	}
	categories := make([]string, 0, len(issues))
	for _, issue := range issues {
		categories = append(categories, issue.Category)
	}
	want := []string{
		"removed-attr",
		"renamed-attr",
		"ext",
		"swap-syntax",
		"inheritance",
		"inheritance",
		"old-api",
		"removed-header",
	}
	if !reflect.DeepEqual(categories, want) {
		t.Fatalf("same-line issue order: got %v want %v", categories, want)
	}
}

func TestFixtureCorpusCoversEveryInheritedAttribute(t *testing.T) {
	for name := range inheritableAttrs {
		t.Run(name, func(t *testing.T) {
			child := "<button hx-post=\"/save\"></button>"
			message := name + " needs :inherited suffix (descendant on line 3 has hx-post)"
			if name == "hx-boost" {
				child = "<a href=\"/next\"></a>"
				message = "hx-boost needs :inherited suffix (descendant <a> on line 3 will no longer inherit it)"
			}
			assertFixtureIssue(t, ".html", "safe\n<div "+name+"=\"yes\">\n"+child+"\n</div>\n", Issue{
				Line: 2, Category: "inheritance", Message: message,
			})
		})
	}
}

func TestInheritanceUsesDepthFirstSourceOrderAndExplicitSuffixes(t *testing.T) {
	path := writeFixtureFile(t, ".html", `<section hx-target="#one" hx-swap:inherited="outerHTML">
  <div><button hx-put="/first"></button></div>
  <button hx-delete="/second"></button>
</section>
<div hx-boost><span><form action="/go"></form></span><a href="/later"></a></div>
<button hx-target="#self" hx-get="/self"></button>
<div hx-swap="outerHTML"></div><button hx-post="/sibling"></button>
`)
	issues, err := checkFile(path)
	if err != nil {
		t.Fatal(err)
	}
	inheritance := issuesOfCategory(issues, "inheritance")
	want := []Issue{
		{Path: path, Line: 1, Category: "inheritance", Message: "hx-target needs :inherited suffix (descendant on line 2 has hx-put)"},
		{Path: path, Line: 5, Category: "inheritance", Message: "hx-boost needs :inherited suffix (descendant <form> on line 5 will no longer inherit it)"},
	}
	if !reflect.DeepEqual(inheritance, want) {
		t.Fatalf("inheritance rows:\n got: %#v\nwant: %#v", inheritance, want)
	}
}

func TestEveryRequestAttributeParticipatesInInheritance(t *testing.T) {
	for name := range requestAttrs {
		t.Run(name, func(t *testing.T) {
			assertFixtureIssue(t, ".html",
				"<div hx-target=\"#result\">\n<button "+name+"=\"/request\"></button>\n</div>\n",
				Issue{
					Line: 1, Category: "inheritance",
					Message: "hx-target needs :inherited suffix (descendant on line 2 has " + name + ")",
				})
		})
	}
}

func TestTemplateStrippingAndMalformedMarkupKeepSourceLocations(t *testing.T) {
	body := `{% if shown
and permitted %}
{{ value
}}
<?php echo "<div hx-vars>";
?>
<% ignored = "<div hx-vars>"
%>
<div><span hx-vars></div>
`
	assertFixtureIssue(t, ".templ", body, Issue{
		Line: 9, Category: "removed-attr", Message: "hx-vars is removed → use hx-vals with js: prefix",
	})
}

func TestDuplicateAttributeUsesTheLastValueAtTheFirstPosition(t *testing.T) {
	assertFixtureIssue(t, ".html", `<div hx-swap="innerHTML" hx-swap="scroll:#results:bottom"></div>`, Issue{
		Line: 1, Category: "swap-syntax",
		Message: "old scroll:#results:bottom syntax → use scroll:bottom scrollTarget:#results",
	})
}

func TestEveryJavaScriptExtensionSkipsDOMChecks(t *testing.T) {
	for extension := range jsExtensions {
		t.Run(extension, func(t *testing.T) {
			path := writeFixtureFile(t, extension, `const template = "<div hx-vars></div>";`)
			issues, err := checkFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if len(issues) != 0 {
				t.Fatalf("%s built a DOM: %#v", extension, issues)
			}
		})
	}
}

func TestPackageDiscoveryAndFileCollectionFollowTheProducer(t *testing.T) {
	root := t.TempDir()
	for _, path := range []string{
		"internal/zeta/assets/htmx.min.js",
		"internal/alpha/assets/htmx-custom.js",
		"internal/testdata/fixture/assets/htmx.min.js",
		"internal/.hidden/assets/htmx.min.js",
		"internal/node_modules/dependency/assets/htmx.min.js",
		"internal/not-htmx/assets/alpine.js",
	} {
		writeTreeFile(t, root, path, "safe\n")
	}
	for _, path := range []string{
		"internal/alpha/page.html",
		"internal/alpha/component.templ",
		"internal/alpha/render.go",
		"internal/alpha/testdata/golden/page.jinja2",
		"internal/alpha/ignored.HTML",
		"internal/zeta/app.ts",
	} {
		writeTreeFile(t, root, path, "safe\n")
	}

	packages, err := htmxPackages(root)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"internal/alpha", "internal/zeta"}; !reflect.DeepEqual(packages, want) {
		t.Fatalf("packages: got %v want %v", packages, want)
	}

	files, err := collectFiles(root, packages)
	if err != nil {
		t.Fatal(err)
	}
	relative := make([]string, 0, len(files))
	for _, path := range files {
		rel, err := filepath.Rel(root, path)
		if err != nil {
			t.Fatal(err)
		}
		relative = append(relative, filepath.ToSlash(rel))
	}
	wantFiles := []string{
		"internal/alpha/assets/htmx-custom.js",
		"internal/alpha/component.templ",
		"internal/alpha/page.html",
		"internal/alpha/render.go",
		"internal/alpha/testdata/golden/page.jinja2",
		"internal/zeta/app.ts",
		"internal/zeta/assets/htmx.min.js",
	}
	if !reflect.DeepEqual(relative, wantFiles) {
		t.Fatalf("files:\n got %v\nwant %v", relative, wantFiles)
	}
}

func TestExplainedParserTrimsRowsAndLastDuplicateWins(t *testing.T) {
	root := t.TempDir()
	writeTreeFile(t, root, explained, strings.Join([]string{
		" # comment",
		"",
		" internal/app/page.html | inheritance | first reason ",
		"internal/app/page.html | inheritance | final reason",
		"",
	}, "\n"))
	rows, err := readExplained(root)
	if err != nil {
		t.Fatal(err)
	}
	key := explainedKey{path: "internal/app/page.html", category: "inheritance"}
	if len(rows) != 1 || rows[key] != "final reason" {
		t.Fatalf("explained rows: %#v", rows)
	}
}

func TestExplainedRowsFailClosed(t *testing.T) {
	root := fixtureTree(t, "")
	if err := os.Remove(filepath.Join(root, filepath.FromSlash(explained))); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	payload, code := runAt(root, false, &stderr)
	wantMissing := "htmx-upgrade-check: scripts/dev/htmx-upgrade-explained.txt is missing; it is this gate's record of what does not apply, and its absence is not an empty list\n"
	if payload != nil || code != 1 || stderr.String() != wantMissing {
		t.Fatalf("missing explained file: payload=%#v code=%d stderr=%q", payload, code, stderr.String())
	}

	cases := []string{
		"internal/app/page.html | removed-attr\n",
		"internal/app/page.html || reason\n",
		"| removed-attr | reason\n",
		"internal/app/page.html | removed-attr | reason | extra\n",
	}
	for _, body := range cases {
		if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(explained)), []byte("# comment\n\n"+body), 0o600); err != nil {
			t.Fatal(err)
		}
		_, code, err := checkTree(root, io.Discard)
		want := "htmx-upgrade-check: scripts/dev/htmx-upgrade-explained.txt:3: a row is `<path> | <category> | <reason>`, and every field must carry text"
		if code != 1 || err == nil || err.Error() != want {
			t.Errorf("row %q: code=%d err=%v", body, code, err)
		}
	}
	if err := os.WriteFile(
		filepath.Join(root, filepath.FromSlash(explained)), []byte{0xff}, 0o600,
	); err != nil {
		t.Fatal(err)
	}
	_, code, err := checkTree(root, io.Discard)
	wantUTF8 := "htmx-upgrade-check: scripts/dev/htmx-upgrade-explained.txt is not valid UTF-8"
	if code != 1 || err == nil || err.Error() != wantUTF8 {
		t.Errorf("invalid UTF-8: code=%d err=%v", code, err)
	}
}

func TestCheckHidesExplainedIssuesAndReportPrintsAll(t *testing.T) {
	root := fixtureTree(t,
		"internal/app/page.html | removed-attr | the fixture explains it\n")

	payload, code := runAt(root, false, io.Discard)
	if code != 0 {
		t.Fatalf("check exited %d", code)
	}
	check := reportPayload(t, payload)
	if len(check.Issues) != 0 {
		t.Fatalf("check exposed explained rows: %#v", check.Issues)
	}
	wantCheck := "htmx-upgrade-check: 2 file(s) in internal/app carry no unexplained htmx 4 upgrade issue (1 explained)\n"
	if got := check.Text(); got != wantCheck {
		t.Fatalf("check output:\n got %q\nwant %q", got, wantCheck)
	}

	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(explained)), []byte("malformed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	payload, code = runAt(root, true, io.Discard)
	if code != 0 {
		t.Fatalf("report exited %d over a malformed explained file", code)
	}
	report := reportPayload(t, payload)
	wantReport := "internal/app/page.html:1: [removed-attr] hx-vars is removed → use hx-vals with js: prefix\n\n1 issue(s) over 2 file(s) in internal/app\n"
	if got := report.Text(); got != wantReport {
		t.Fatalf("report output:\n got %q\nwant %q", got, wantReport)
	}
}

func TestCheckReportsUnexplainedAndStaleRowsInProducerOrder(t *testing.T) {
	root := fixtureTree(t, strings.Join([]string{
		"z.html | ext | stale z",
		"a.html | old-api | stale a",
		"",
	}, "\n"))
	payload, code := runAt(root, false, io.Discard)
	if code != 1 {
		t.Fatalf("check exited %d, want 1", code)
	}
	report := reportPayload(t, payload)
	want := strings.Join([]string{
		"internal/app/page.html:1: [removed-attr] hx-vars is removed → use hx-vals with js: prefix",
		"STALE: scripts/dev/htmx-upgrade-explained.txt explains old-api in a.html, and the scan reports none there",
		"STALE: scripts/dev/htmx-upgrade-explained.txt explains ext in z.html, and the scan reports none there",
		"",
		"1 unexplained issue(s) and 2 stale row(s) over 2 file(s) in internal/app",
		"Fix the site, or add its row to scripts/dev/htmx-upgrade-explained.txt",
		"",
	}, "\n")
	if got := report.Text(); got != want {
		t.Fatalf("failure output:\n got %q\nwant %q", got, want)
	}
}

func TestEmptyDiscoveryFailsClosedButReportStillExitsZero(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, consumerRoot), 0o750); err != nil {
		t.Fatal(err)
	}
	payload, code := runAt(root, false, io.Discard)
	if payload != nil || code != 1 {
		t.Fatalf("empty check: payload=%#v code=%d", payload, code)
	}

	payload, code = runAt(root, true, io.Discard)
	if code != 0 {
		t.Fatalf("empty report exited %d", code)
	}
	if got := reportPayload(t, payload).Text(); got != "\n0 issue(s) over 0 file(s) in \n" {
		t.Fatalf("empty report output is %q", got)
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(encoded, []byte(`"issues":[]`)) {
		t.Fatalf("empty issue rows are not pipeable: %s", encoded)
	}
}

func TestActionsClaimExactGateMetadataAndArgumentCodes(t *testing.T) {
	list := Actions()
	if list.Area != area || len(list.Actions) != 2 {
		t.Fatalf("action list: %#v", list)
	}
	want := []struct {
		verb string
		gate string
		why  string
	}{
		{verb: "check", gate: "ze-htmx-upgrade-check", why: "htmx's OWN scanner, vendored at third_party/web/htmx-upgrade-check.py, reports no htmx 4 issue that scripts/dev/htmx-upgrade-explained.txt does not account for. It builds a DOM, so it reads the inheritance carriers a text search cannot"},
		{verb: "report", gate: "ze-htmx-upgrade-report", why: "print every htmx 4 upgrade issue, explained or not, and exit 0"},
	}
	for index, row := range list.Actions {
		if row.Verb != want[index].verb || row.Gate != want[index].gate || row.Why != want[index].why || row.Writes || len(row.Forks) != 0 {
			t.Errorf("action %d: %#v", index, row)
		}
	}
	payload, code := Answer(nil)
	if code != 0 || !reflect.DeepEqual(payload, list) {
		t.Errorf("bare area: payload=%#v code=%d", payload, code)
	}
	if payload, code := Answer([]string{"unknown"}); payload != nil || code != 2 {
		t.Errorf("unknown verb: payload=%#v code=%d", payload, code)
	}
	if payload, code := Answer([]string{"check", "extra"}); payload != nil || code != 1 {
		t.Errorf("extra value: payload=%#v code=%d", payload, code)
	}
}

func TestActionsUseTheCheckoutNamedByTheEnvironment(t *testing.T) {
	root := fixtureTree(t,
		"internal/app/page.html | removed-attr | fixture\n")
	useHTMXCheckout(t, root)

	payload, code := Answer([]string{"check"})
	if code != 0 {
		t.Fatalf("environment-selected check exited %d", code)
	}
	if got := reportPayload(t, payload).Packages; got != "internal/app" {
		t.Fatalf("check scanned %q, want the fixture package", got)
	}
	payload, code = Answer([]string{"report"})
	if code != 0 || len(reportPayload(t, payload).Issues) != 1 {
		t.Fatalf("environment-selected report: payload=%#v code=%d", payload, code)
	}
}

func TestBothActionsLeaveTheTreeByteForByteUnchanged(t *testing.T) {
	root := fixtureTree(t,
		"internal/app/page.html | removed-attr | fixture\n")
	before := treeState(t, root)
	if _, code := runAt(root, false, io.Discard); code != 0 {
		t.Fatalf("check exited %d", code)
	}
	if _, code := runAt(root, true, io.Discard); code != 0 {
		t.Fatalf("report exited %d", code)
	}
	after := treeState(t, root)
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("actions changed the fixture:\n before=%#v\n after=%#v", before, after)
	}
}

func TestIssueRowsAreStructuredForPipeRendering(t *testing.T) {
	report := Report{
		Issues: []Issue{{Path: "a.html", Category: "removed-attr", Line: 7, Message: "bad"}},
		Files:  3, Packages: "internal/app", stale: []staleRow{{path: "hidden", category: "ext"}},
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"issues":[{"path":"a.html","category":"removed-attr","line":7,"message":"bad"}],"files":3,"packages":"internal/app"}`
	if string(encoded) != want {
		t.Fatalf("structured answer:\n got %s\nwant %s", encoded, want)
	}
}

func TestLiveTreeMatchesItsExplanationLedger(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatal(err)
	}
	report, code, err := checkTree(root, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Fatalf("live tree has unexplained or stale rows:\n%s", report.Text())
	}
	if report.Files == 0 || report.Packages == "" || report.explainedCount == 0 {
		t.Fatalf("live scan proved nothing: %#v", report)
	}
}

func TestCopiedRulesRemainPresentInTheVendoredProducer(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatal(err)
	}
	scannerBody, err := os.ReadFile(filepath.Join(root, "third_party", "web", "htmx-upgrade-check.py"))
	if err != nil {
		t.Fatal(err)
	}
	scannerSource := string(scannerBody)
	for _, rules := range [][]renameRule{
		removedAttrs, renamedAttrs, eventRenames, sseEventRenames, wsEventRenames,
		extensionAttrRenames, removedJSAPI, configRenames, removedResponseHeaders,
	} {
		for _, rule := range rules {
			if !strings.Contains(scannerSource, rule.old) {
				t.Errorf("vendored scanner no longer contains copied key %q", rule.old)
			}
			firstLine, _, _ := strings.Cut(rule.new, "\n")
			if !strings.Contains(scannerSource, firstLine) {
				t.Errorf("vendored scanner no longer contains copied replacement %q", firstLine)
			}
		}
	}
	for _, values := range [][]string{removedEvents, removedConfig, defaultExtensions} {
		for _, value := range values {
			if !strings.Contains(scannerSource, value) {
				t.Errorf("vendored scanner no longer contains copied value %q", value)
			}
		}
	}
	for _, values := range []map[string]bool{
		inheritableAttrs, requestAttrs, jsExtensions, voidElements,
	} {
		for value := range values {
			if !strings.Contains(scannerSource, value) {
				t.Errorf("vendored scanner no longer contains copied value %q", value)
			}
		}
	}

	producerBody, err := os.ReadFile(filepath.Join(root, "scripts", "dev", "htmx_upgrade_check.py"))
	if err != nil {
		t.Fatal(err)
	}
	producerSource := string(producerBody)
	for _, value := range extraExtensions {
		if !strings.Contains(producerSource, value) {
			t.Errorf("Ze producer no longer contains copied extension %q", value)
		}
	}
	for _, value := range []string{
		consumerRoot, consumerDir, htmxPrefix, htmxSuffix, explained,
	} {
		if !strings.Contains(producerSource, value) {
			t.Errorf("Ze producer no longer contains copied path value %q", value)
		}
	}
}

func assertFixtureIssue(t *testing.T, extension, body string, want Issue) {
	t.Helper()
	path := writeFixtureFile(t, extension, body)
	issues, err := checkFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want.Path = path
	if slices.Contains(issues, want) {
		return
	}
	t.Fatalf("missing issue %#v in %#v", want, issues)
}

func writeFixtureFile(t *testing.T, extension, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fixture"+extension)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func reportPayload(t *testing.T, payload any) Report {
	t.Helper()
	report, ok := payload.(Report)
	if !ok {
		t.Fatalf("payload type = %T, want htmxupgrade.Report", payload)
	}
	return report
}

func fixtureTree(t *testing.T, explanation string) string {
	t.Helper()
	root := t.TempDir()
	writeTreeFile(t, root, "internal/app/assets/htmx.min.js", "/* current htmx */\n")
	writeTreeFile(t, root, "internal/app/page.html", `<div hx-vars></div>`)
	writeTreeFile(t, root, explained, explanation)
	return root
}

func writeTreeFile(t *testing.T, root, relative, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func issuesOfCategory(issues []Issue, category string) []Issue {
	found := make([]Issue, 0)
	for _, issue := range issues {
		if issue.Category == category {
			found = append(found, issue)
		}
	}
	return found
}

func useHTMXCheckout(t *testing.T, root string) {
	t.Helper()
	t.Setenv("ZE_REPO_ROOT", root)
	env.ResetCache()
	t.Cleanup(env.ResetCache)
}

func treeState(t *testing.T, root string) map[string]string {
	t.Helper()
	paths := make(map[string]string)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		state := info.Mode().String() + "|" + info.ModTime().String()
		if entry.IsDir() {
			paths[filepath.ToSlash(rel)+"/"] = state
			return nil
		}
		body, err := os.ReadFile(path) //nolint:gosec // a test fixture path
		if err != nil {
			return err
		}
		paths[filepath.ToSlash(rel)] = state + "|" + string(body)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return paths
}
