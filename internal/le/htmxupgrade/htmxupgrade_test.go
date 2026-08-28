package htmxupgrade

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
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

// The fixture corpus asks the native scanner about every upstream table entry.
// Each expected row includes the source location and complete message, so a
// value that remains present but routes to the wrong category is still caught.
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

// The expected rows were captured from the byte-identified upstream scanner.
// This one fixture locks the phase ordering and complete human-facing output.
func TestEndToEndFixtureMatchesUpstreamOutput(t *testing.T) {
	path := writeFixtureFile(t, ".html", `<div hx-vars hx-disable sse-connect hx-swap="show:#box:top" hx-target="#box">
  <button hx-get="/save"></button>
</div>
<div hx-on::afteronload="run()"></div>
<div hx-on:htmx:validation:failed="run()"></div>
<script>listen("htmx:afterRequest")</script>
<script>listen("htmx:xhr:abort")</script>
<script>htmx.takeClass (node, "active")</script>
<script>htmx.config.defaultSwapStyle = "outerHTML"</script>
<script>cfg["historyCacheSize"] = 0</script>
<script>headers["HX-Trigger-After-Settle"] = value</script>
`)
	issues, err := checkFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := []Issue{
		{Path: path, Line: 1, Category: "removed-attr", Message: "hx-vars is removed → use hx-vals with js: prefix"},
		{Path: path, Line: 1, Category: "renamed-attr", Message: "hx-disable → rename to hx-ignore (hx-disable now means 'disable during request')"},
		{Path: path, Line: 1, Category: "ext", Message: "sse-connect → rename to hx-sse:connect"},
		{Path: path, Line: 1, Category: "swap-syntax", Message: "old show:#box:top syntax → use show:top showTarget:#box"},
		{Path: path, Line: 1, Category: "inheritance", Message: "hx-swap needs :inherited suffix (descendant on line 2 has hx-get)"},
		{Path: path, Line: 1, Category: "inheritance", Message: "hx-target needs :inherited suffix (descendant on line 2 has hx-get)"},
		{Path: path, Line: 4, Category: "renamed-event", Message: `hx-on::afteronload uses old event name "htmx:afterOnLoad" → use hx-on:htmx:after:init`},
		{Path: path, Line: 5, Category: "removed-event", Message: "hx-on:htmx:validation:failed uses removed event htmx:validation:failed"},
		{Path: path, Line: 6, Category: "old-event", Message: `old event name "htmx:afterRequest" → "htmx:after:request"`},
		{Path: path, Line: 7, Category: "removed-event", Message: `removed event "htmx:xhr:abort"`},
		{Path: path, Line: 8, Category: "old-api", Message: "htmx.takeClass() is removed → removed from core. To restore the htmx 2 behavior verbatim, paste:\n    htmx.takeClass = (elt, cls) => {\n        elt = typeof elt === 'string' ? document.querySelector(elt) : elt;\n        for (let c of elt.parentElement.children) c.classList.remove(cls);\n        elt.classList.add(cls);\n    };\n  For a fully featured replacement (polymorphic targets/sources, selector strings, iterable inputs, q-proxy chaining), load the hx-live extension and use htmx.live.take()."},
		{Path: path, Line: 9, Category: "renamed-config", Message: `config "defaultSwapStyle" → "defaultSwap"`},
		{Path: path, Line: 10, Category: "removed-config", Message: `config "historyCacheSize" is removed`},
		{Path: path, Line: 11, Category: "removed-header", Message: `"HX-Trigger-After-Settle" is removed → use HX-Trigger or JavaScript instead`},
	}
	if !reflect.DeepEqual(issues, want) {
		t.Fatalf("fixture output:\n got: %#v\nwant: %#v", issues, want)
	}
}

func TestScannerPatternBoundariesMatchUpstream(t *testing.T) {
	cases := []struct {
		name string
		body string
		want []Issue
	}{
		{
			name: "case-folding-and-character-reference",
			body: `<DIV HX-SWAP="show:&#35;box:bottom"></DIV>`,
			want: []Issue{{Line: 1, Category: "swap-syntax", Message: "old show:#box:bottom syntax → use show:bottom showTarget:#box"}},
		},
		{
			name: "hx-on-suppresses-whole-text-line",
			body: `<div hx-on::afterrequest="run()">htmx:afterSwap htmx:xhr:abort</div>`,
			want: []Issue{{Line: 1, Category: "renamed-event", Message: `hx-on::afterrequest uses old event name "htmx:afterRequest" → use hx-on:htmx:after:request`}},
		},
		{
			name: "first-swap-match-only",
			body: `<div hx-swap="scroll:#first:bottom show:#second:top"></div>`,
			want: []Issue{{Line: 1, Category: "swap-syntax", Message: "old scroll:#first:bottom syntax → use scroll:bottom scrollTarget:#first"}},
		},
		{
			name: "unicode-api-whitespace",
			body: "htmx.addClass\u00a0(node)",
			want: []Issue{{Line: 1, Category: "old-api", Message: "htmx.addClass() is removed → use element.classList.add()"}},
		},
		{
			name: "quoted-config-key",
			body: `cfg['historyEnabled']: true`,
			want: []Issue{{Line: 1, Category: "renamed-config", Message: `config "historyEnabled" → "history"`}},
		},
		{
			name: "near-misses",
			body: "<div hx-swap=\"show::top scroll:#x:left show:#x: bottom\"></div>\nhtmx.addClass/**/()\nobject.config.defaultSwapStyleName = true\n",
			want: []Issue{},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			path := writeFixtureFile(t, ".html", test.body)
			issues, err := checkFile(path)
			if err != nil {
				t.Fatal(err)
			}
			for index := range test.want {
				test.want[index].Path = path
			}
			if !reflect.DeepEqual(issues, test.want) {
				t.Fatalf("issues:\n got: %#v\nwant: %#v", issues, test.want)
			}
		})
	}
}

func TestDOMStackPreservesVoidSelfClosingAndMalformedInheritance(t *testing.T) {
	path := writeFixtureFile(t, ".html", `<input hx-target><button hx-get="/void-sibling"></button>
<section hx-target/><button hx-post="/self-closing-sibling"></button>
<div hx-target><span><button hx-put="/nested"></button></span></div>
<div hx-boost><br><form action="/boost"></form></div>
<div hx-target></section><button hx-delete="/unmatched-end"></button></div>
`)
	issues, err := checkFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := []Issue{
		{Path: path, Line: 3, Category: "inheritance", Message: "hx-target needs :inherited suffix (descendant on line 3 has hx-put)"},
		{Path: path, Line: 4, Category: "inheritance", Message: "hx-boost needs :inherited suffix (descendant <form> on line 4 will no longer inherit it)"},
		{Path: path, Line: 5, Category: "inheritance", Message: "hx-target needs :inherited suffix (descendant on line 5 has hx-delete)"},
	}
	if !reflect.DeepEqual(issues, want) {
		t.Fatalf("DOM issues:\n got: %#v\nwant: %#v", issues, want)
	}
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

func TestTextChecksKeepUpstreamPhaseAndTableOrder(t *testing.T) {
	body := strings.Join([]string{
		"htmx:timeout htmx:afterOnLoad htmx:wsOpen htmx:sseOpen",
		"htmx:xhr:abort htmx:validation:validate",
		"htmx.defineExtension() htmx.addClass()",
		"config.includeIndicatorStyles=true config.defaultSwapStyle=true",
		`config.wsReconnectDelay=1 config.addedClass="x"`,
		"HX-Trigger-After-Settle HX-Trigger-After-Swap",
	}, " ")
	path := writeFixtureFile(t, ".js", body)
	issues, err := checkFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := []Issue{
		{Path: path, Line: 1, Category: "old-event", Message: `old event name "htmx:afterOnLoad" → "htmx:after:init"`},
		{Path: path, Line: 1, Category: "old-event", Message: `old event name "htmx:timeout" → "htmx:error"`},
		{Path: path, Line: 1, Category: "old-event", Message: `old event name "htmx:sseOpen" → "htmx:after:sse:connection"`},
		{Path: path, Line: 1, Category: "old-event", Message: `old event name "htmx:wsOpen" → "htmx:after:ws:connection"`},
		{Path: path, Line: 1, Category: "removed-event", Message: `removed event "htmx:validation:validate"`},
		{Path: path, Line: 1, Category: "removed-event", Message: `removed event "htmx:xhr:abort"`},
		{Path: path, Line: 1, Category: "old-api", Message: "htmx.addClass() is removed → use element.classList.add()"},
		{Path: path, Line: 1, Category: "old-api", Message: "htmx.defineExtension() is removed → use htmx.registerExtension()"},
		{Path: path, Line: 1, Category: "renamed-config", Message: `config "defaultSwapStyle" → "defaultSwap"`},
		{Path: path, Line: 1, Category: "renamed-config", Message: `config "includeIndicatorStyles" → "includeIndicatorCSS"`},
		{Path: path, Line: 1, Category: "removed-config", Message: `config "addedClass" is removed`},
		{Path: path, Line: 1, Category: "removed-config", Message: `config "wsReconnectDelay" is removed`},
		{Path: path, Line: 1, Category: "removed-header", Message: `"HX-Trigger-After-Swap" is removed → use HX-Trigger or JavaScript instead`},
		{Path: path, Line: 1, Category: "removed-header", Message: `"HX-Trigger-After-Settle" is removed → use HX-Trigger or JavaScript instead`},
	}
	if !reflect.DeepEqual(issues, want) {
		t.Fatalf("text phase order:\n got: %#v\nwant: %#v", issues, want)
	}
}

func TestIssueNormalizationDeduplicatesThenStablySortsByLine(t *testing.T) {
	input := []Issue{
		{Path: "fixture", Line: 2, Category: "old-api", Message: "second line"},
		{Path: "fixture", Line: 1, Category: "removed-attr", Message: "first phase"},
		{Path: "fixture", Line: 2, Category: "old-api", Message: "second line"},
		{Path: "fixture", Line: 1, Category: "inheritance", Message: "second phase"},
	}
	want := []Issue{input[1], input[3], input[0]}
	if got := normalizeIssues(input); !reflect.DeepEqual(got, want) {
		t.Fatalf("normalized issues:\n got: %#v\nwant: %#v", got, want)
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

func TestUniversalNewlinesAndInvalidUTF8MatchUpstreamFileReads(t *testing.T) {
	path := filepath.Join(t.TempDir(), "fixture.html")
	body := append([]byte("safe\r\n<div hx-vars></div>\u2028"), 0xff)
	body = append(body, []byte("htmx.addClass()\rHX-Trigger-After-Swap")...)
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatal(err)
	}
	issues, err := checkFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := []Issue{
		{Path: path, Line: 2, Category: "removed-attr", Message: "hx-vars is removed → use hx-vals with js: prefix"},
		{Path: path, Line: 3, Category: "old-api", Message: "htmx.addClass() is removed → use element.classList.add()"},
		{Path: path, Line: 4, Category: "removed-header", Message: `"HX-Trigger-After-Swap" is removed → use HX-Trigger or JavaScript instead`},
	}
	if !reflect.DeepEqual(issues, want) {
		t.Fatalf("line-boundary issues:\n got: %#v\nwant: %#v", issues, want)
	}
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
	wantMissing := "htmx-upgrade-check: test/htmx-upgrade-explained.txt is missing; it is this gate's record of what does not apply, and its absence is not an empty list\n"
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
		want := "htmx-upgrade-check: test/htmx-upgrade-explained.txt:3: a row is `<path> | <category> | <reason>`, and every field must carry text"
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
	wantUTF8 := "htmx-upgrade-check: test/htmx-upgrade-explained.txt is not valid UTF-8"
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
		"STALE: test/htmx-upgrade-explained.txt explains old-api in a.html, and the scan reports none there",
		"STALE: test/htmx-upgrade-explained.txt explains ext in z.html, and the scan reports none there",
		"",
		"1 unexplained issue(s) and 2 stale row(s) over 2 file(s) in internal/app",
		"Fix the site, or add its row to test/htmx-upgrade-explained.txt",
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

func TestActionsClaimExactMetadataAndArgumentCodes(t *testing.T) {
	list := Actions()
	if list.Area != area || len(list.Actions) != 2 {
		t.Fatalf("action list: %#v", list)
	}
	want := []struct {
		verb string
		why  string
	}{
		{verb: "check", why: "the native Go scanner reports no htmx 4 issue that test/htmx-upgrade-explained.txt does not account for. It builds a DOM, so it reads inheritance carriers a text search cannot"},
		{verb: "report", why: "print every htmx 4 upgrade issue, explained or not, and exit 0"},
	}
	for index, row := range list.Actions {
		if row.Verb != want[index].verb || row.Why != want[index].why || row.Writes {
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
	if payload, code := Answer([]string{"check", "extra"}); payload != nil || code != 2 {
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

func TestUpstreamScannerContractDigest(t *testing.T) {
	if upstreamScannerVersion != "4.0.0-beta6" {
		t.Fatalf("upstream version = %q", upstreamScannerVersion)
	}
	if upstreamScannerURL != "https://unpkg.com/htmx.org@4.0.0-beta6/dist/scripts/upgrade-check.py" {
		t.Fatalf("upstream URL = %q", upstreamScannerURL)
	}
	if upstreamSourceSHA256 != "9633ce96b7d16d8ef2c11a6da91a6f0adcea891bec663e005249aea39df7a58b" {
		t.Fatalf("upstream source digest = %q", upstreamSourceSHA256)
	}
	if got := scannerContractDigest(); got != scannerContractSHA256 {
		t.Fatalf("scanner contract digest = %s, want %s", got, scannerContractSHA256)
	}
}

func scannerContractDigest() string {
	digest := sha256.New()
	write := func(value string) {
		digest.Write([]byte(value))
		digest.Write([]byte{0})
	}
	writeRenames := func(section string, rules []renameRule) {
		write(section)
		for _, rule := range rules {
			write(rule.old)
			write(rule.new)
		}
	}
	writeValues := func(section string, values []string) {
		write(section)
		for _, value := range values {
			write(value)
		}
	}
	writeSet := func(section string, values map[string]bool) {
		keys := make([]string, 0, len(values))
		for value := range values {
			keys = append(keys, value)
		}
		slices.Sort(keys)
		writeValues(section, keys)
	}

	writeRenames("removed-attrs", removedAttrs)
	writeRenames("renamed-attrs", renamedAttrs)
	writeSet("inheritable-attrs", inheritableAttrs)
	writeSet("request-attrs", requestAttrs)
	writeRenames("event-renames", eventRenames)
	writeValues("removed-events", removedEvents)
	writeRenames("sse-event-renames", sseEventRenames)
	writeRenames("ws-event-renames", wsEventRenames)
	writeRenames("extension-attr-renames", extensionAttrRenames)
	writeRenames("removed-js-api", removedJSAPI)
	writeRenames("config-renames", configRenames)
	writeValues("removed-config", removedConfig)
	writeRenames("removed-response-headers", removedResponseHeaders)
	writeSet("void-elements", voidElements)
	writeSet("javascript-extensions", jsExtensions)
	writeValues("default-extensions", defaultExtensions)
	writeValues("ze-extensions", extraExtensions)

	return hex.EncodeToString(digest.Sum(nil))
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
