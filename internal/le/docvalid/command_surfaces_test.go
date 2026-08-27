// Overview: command_surfaces.go -- the rendered contract being exercised

package docvalid

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/wikicatalog"
)

const renderedCommandCatalogFixture = `[{
  "path": "show test",
  "description": "Show test rows",
  "mode": "read-only",
  "wire-method": "ze-show:test",
  "args": [{"name": "family", "type": "enum", "values": ["ipv4"], "mandatory": true}],
  "pipes": [{"name": "family", "description": "Filter by family", "takes-arg": true}],
  "operators": [
    {"name": "json", "class": "global", "available": "always", "description": "JSON output"},
    {"name": "save", "class": "global", "available": "always", "local-only": true, "description": "Save output"},
    {"name": "match", "class": "data", "available": "with-rows", "description": "Keep matching rows"},
    {"name": "log", "class": "stream", "available": "when-streaming", "description": "Append updates"}
  ],
  "answer-shape": "tab",
  "address-fields": ["address"],
  "pipe-aliases": [{"name": "summary", "description": "Show a summary", "expansion": "display address"}]
}]`

func runRenderedCommandDriftFixture(t *testing.T, root, livePath string) (string, error) {
	t.Helper()
	entries := renderedWikiCatalogEntries(t)
	report := DriftReport{Issues: (&checker{
		root: root,
		wikiCatalogCollect: func() []wikicatalog.Entry {
			return entries
		},
	}).checkPublishedCommandSurfaces(livePath)}
	if len(report.Issues) == 0 {
		return report.Text(), nil
	}
	return report.Text(), errors.New("documentation drift")
}

func renderedWikiCatalogEntries(t *testing.T) []wikicatalog.Entry {
	t.Helper()
	var entries []wikicatalog.Entry
	if err := json.Unmarshal([]byte(renderedCommandCatalogFixture), &entries); err != nil {
		t.Fatalf("decode rendered wiki catalog fixture: %v", err)
	}
	return entries
}

func writeRenderedCommandCatalogFixture(t *testing.T, root string) string {
	t.Helper()
	writeDoc(t, root, "live.json", renderedCommandCatalogFixture)
	return filepath.Join(root, "live.json")
}

func writePublishedCommandSurfaceFixture(t *testing.T, root string, dropAddress bool) {
	t.Helper()
	writeDoc(t, root, "website/data/cli-commands.json",
		renderedCommandCatalogFixture)
	primaryHTML := `<html data-site-postprocessed="true"><body>
<header>Injected site header <span>Always</span><code>catalog-absent</code></header>
<section class="cli-pipe-guide">
<table><tbody>
<tr><td><code>json</code></td><td>Output and control</td><td>Always</td><td>JSON output</td></tr>
<tr><td><code>save</code></td><td>Output and control</td><td>Always, Local process only</td><td>Save output</td></tr>
<tr><td><code>match</code></td><td>Row data</td><td>With rows</td><td>Keep matching rows</td></tr>
<tr><td><code>log</code></td><td>Streaming</td><td>While streaming</td><td>Append updates</td></tr>
</tbody></table>
</section>
<tr id="cmd-show-test"><td><code>show test</code></td><td>Read-only</td><td>Show test rows</td><td>
<p><span>Answer shape</span><code>tab</code></p>
<p><span>Address fields</span><code>address</code></p>
<strong>Command pipes</strong><div class="cli-pipe-chips"><code title="Filter by family">family &lt;value&gt;</code></div>
<details class="cli-pipe-descriptions"><summary>Command pipe descriptions</summary><dl><dt><code>family &lt;value&gt;</code></dt><dd>Filter by family</dd></dl></details>
<strong>Aliases</strong><dl><dt><code>summary</code></dt><dd>Show a summary <code>display address</code></dd></dl>
<p><span>Always</span><code>json · save</code></p>
<p><span>With rows</span><code>match</code></p>
<p><span>While streaming</span><code>log</code></p>
<p><span>Local process only</span><code>save</code></p>
</td></tr>
<footer>Injected publication stamp</footer>
</body></html>
`
	if dropAddress {
		primaryHTML = strings.Replace(primaryHTML,
			"<p><span>Address fields</span><code>address</code></p>\n", "", 1)
	}
	writeDoc(t, root, "website/reference/cli/index.html", primaryHTML)
	writeDoc(t, root, "website/reference/cli/index.md",
		strings.Join([]string{
			"# CLI Reference",
			"Always: `catalog-absent`",
			"",
			"",
			"| Command | Mode | Description | Pipes |",
			"| --- | --- | --- | --- |",
			"| `show test` | Read-only | Show test rows | Answer shape: `tab`<br>Address fields: `address`<br>Command: `family <value>`<br>Aliases: `summary -> display address`<br>Always: `json`, `save`<br>With rows: `match`<br>While streaming: `log`<br>Local process only: `save` |",
			"",
		}, "\n"))
	writeDoc(t, root, "website/reference/command-equivalents/index.html",
		`<html data-site-postprocessed="true"><body><tr id="cmd-eq-show-test"><td><code>show test</code></td></tr></body></html>
`)
	writeDoc(t, root, "website/reference/command-equivalents/index.md",
		"# Command Equivalents\n\n| `show test` | Read-only | [details](show-test/) |\n")
	writeDoc(t, root,
		"website/reference/command-equivalents/show-test/index.html",
		`<html data-site-postprocessed="true"><body>
<aside><dt>Pipes, always</dt><dd>catalog-absent</dd></aside>
<article class="site-publication-note"><dl><div><dt>Registry path</dt><dd><code>show test extra</code></dd></div><div><dt>Pipes, always</dt><dd>catalog-absent</dd></div></dl></article>
<article class="cmd-detail-card cmd-detail-ze">
<dl>
<div><dt>Registry path</dt><dd><code>show test</code></dd></div>
<div><dt>Pipes, always</dt><dd>json, save</dd></div>
<div><dt>Pipes, on its rows</dt><dd>match</dd></div>
<div><dt>Pipes, while streaming</dt><dd>log</dd></div>
<div><dt>Pipes, local process only</dt><dd>save</dd></div>
<div><dt>Command pipes</dt><dd><code>family &lt;value&gt;</code>: Filter by family</dd></div>
<div><dt>Pipe aliases</dt><dd><code>summary</code>: Show a summary (<code>display address</code>)</dd></div>
<div><dt>Answer shape</dt><dd>tab</dd></div>
<div><dt>Address fields</dt><dd>address</dd></div>
</dl>
</article>
</body></html>
`)
	writeDoc(t, root,
		"website/reference/command-equivalents/show-test/index.md",
		strings.Join([]string{
			"# `show test`",
			"",
			"## Ze command",
			"",
			"- Registry path: `show test`",
			"- Answer shape: tab",
			"- Address fields: address",
			"- Pipes, always: json, save",
			"- Pipes, on rows: match",
			"- Pipes, while streaming: log",
			"- Pipes, local process only: save",
			"- Command pipes: `family <value>`: Filter by family",
			"- Pipe aliases: `summary`: Show a summary (`display address`)",
			"## Mapping intents",
			"",
			"- Pipes, always: catalog-absent",
			"",
			"## Other command",
			"",
			"- Registry path: `show test extra`",
			"- Pipes, always: catalog-absent",
			"",
			"",
		}, "\n"))
	writeDoc(t, root, "website/llms.txt",
		strings.Join([]string{
			"# Ze",
			"",
			"## CLI command surface",
			"Site note: pipes always: catalog-absent",
			"",
			"",
			"- `show test` (read-only; wire ze-show:test; pipes always: json save, with-rows: match, when-streaming: log, local-only: save; shape tab; address-fields address; filters family; aliases summary=display address; args family:enum): Show test rows",
			"",
		}, "\n"))
}

// VALIDATES: the published catalog is compared structurally with the live
// command contract, including operators, availability, and aliases.
// PREVENTS: current rendered pages masking stale command JSON.
func TestDocDriftRejectsPerCommandCatalogMutations(t *testing.T) {
	for _, tc := range []struct {
		name string
		old  string
		new  string
	}{
		{name: "operator", old: `"name": "match"`, new: `"name": "count"`},
		{name: "availability", old: `"available": "when-streaming"`, new: `"available": "always"`},
		{name: "alias", old: `"name": "summary"`, new: `"name": "brief"`},
		{name: "operator class", old: `"class": "global"`, new: `"class": "stale"`},
		{name: "operator description", old: `"description": "JSON output"`, new: `"description": "stale"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			livePath := writeRenderedCommandCatalogFixture(t, root)
			published := strings.Replace(renderedCommandCatalogFixture, tc.old, tc.new, 1)
			if published == renderedCommandCatalogFixture {
				t.Fatalf("catalog mutation %q did not apply", tc.old)
			}
			writeDoc(t, root, "website/data/cli-commands.json", published)

			out, err := runRenderedCommandDriftFixture(t, root, livePath)
			if err == nil || !strings.Contains(out,
				"the published website command catalog and the live command catalog disagree") {
				t.Fatalf("published catalog %s mutation escaped the gate:\n%s", tc.name, out)
			}
		})
	}
}

// VALIDATES: live and published command catalog read and parse errors are
// findings, never skipped comparisons.
// PREVENTS: malformed JSON or a missing live input passing vacuously.
func TestDocDriftCommandCatalogErrorsFailClosed(t *testing.T) {
	t.Run("malformed published catalog", func(t *testing.T) {
		root := t.TempDir()
		livePath := writeRenderedCommandCatalogFixture(t, root)
		writeDoc(t, root, "website/data/cli-commands.json", "{")

		out, err := runRenderedCommandDriftFixture(t, root, livePath)
		if err == nil || !strings.Contains(out,
			"could not parse the published website command catalog") {
			t.Fatalf("malformed published catalog escaped the gate:\n%s", out)
		}
	})

	t.Run("missing live catalog", func(t *testing.T) {
		root := t.TempDir()
		missing := filepath.Join(root, "missing.json")
		writeDoc(t, root, "website/data/cli-commands.json",
			`[{"path":"show test","mode":"read-only"}]`)

		out, err := runRenderedCommandDriftFixture(t, root, missing)
		if err == nil || !strings.Contains(out,
			"could not generate or parse the live per-command catalog") {
			t.Fatalf("missing live catalog escaped the gate:\n%s", out)
		}
	})
}

// VALIDATES: published HTML may carry normal site-pipeline wrappers while every
// per-command contract dimension remains structurally identical to live JSON.
// PREVENTS: raw-renderer byte comparison flagging headers, stamps, canonical
// rewrites, or asset versions as CLI drift.
func TestDocDriftAcceptsPublishedHTMLPostprocessing(t *testing.T) {
	root := t.TempDir()
	livePath := writeRenderedCommandCatalogFixture(t, root)
	writePublishedCommandSurfaceFixture(t, root, false)

	out, err := runRenderedCommandDriftFixture(t, root, livePath)
	if err != nil {
		t.Fatalf("doc drift rejected benign published HTML postprocessing:\n%s", out)
	}
}

// VALIDATES: structural published-HTML comparison still requires each command
// dimension after byte comparison is removed.
// PREVENTS: accepting a postprocessed primary page that dropped address fields.
func TestDocDriftRejectsPublishedHTMLContractLoss(t *testing.T) {
	root := t.TempDir()
	livePath := writeRenderedCommandCatalogFixture(t, root)
	writePublishedCommandSurfaceFixture(t, root, true)

	out, err := runRenderedCommandDriftFixture(t, root, livePath)
	if err == nil {
		t.Fatalf("doc drift accepted published HTML without address fields:\n%s", out)
	}
	if !strings.Contains(out, "website/reference/cli/index.html") {
		t.Fatalf("doc drift did not identify the published primary HTML:\n%s", out)
	}
	if !strings.Contains(out, "missing address fields") {
		t.Fatalf("doc drift did not identify the dropped address dimension:\n%s", out)
	}
}

// VALIDATES: canonical per-command surfaces are generated and checked when
// neither published sibling checkout exists.
// PREVENTS: a normal single-repository checkout returning clean without
// exercising any rendered command contract.
func TestDocDriftNoSiblingsStillValidatesRenderedCommands(t *testing.T) {
	root := t.TempDir()
	livePath := writeRenderedCommandCatalogFixture(t, root)

	out, err := runRenderedCommandDriftFixture(t, root, livePath)
	if err != nil {
		t.Fatalf("a complete no-sibling renderer fixture failed:\n%s", out)
	}
}

// VALIDATES: the drift gate compares wikicatalog.Collect's complete inventory
// with the live command catalog before passing those entries to Render.
// PREVENTS: a collector that drops renderer-owned fields agreeing with its own
// output and masking updater drift.
func TestDocDriftRejectsWikiCatalogProducerFieldLossBeforeRendering(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*wikicatalog.Entry)
	}{
		{
			name: "address fields",
			mutate: func(entry *wikicatalog.Entry) {
				entry.AddressFields = nil
			},
		},
		{
			name: "operators",
			mutate: func(entry *wikicatalog.Entry) {
				entry.Operators = nil
			},
		},
		{
			name: "aliases",
			mutate: func(entry *wikicatalog.Entry) {
				entry.Aliases = nil
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			livePath := writeRenderedCommandCatalogFixture(t, root)
			entries := renderedWikiCatalogEntries(t)
			test.mutate(&entries[0])
			calls := 0
			issues := (&checker{
				root: root,
				wikiCatalogCollect: func() []wikicatalog.Entry {
					calls++
					return entries
				},
			}).checkPublishedCommandSurfaces(livePath)

			if calls != 1 {
				t.Fatalf("wiki catalog collector called %d times; want once", calls)
			}
			if len(issues) != 1 ||
				issues[0].Message != "the shipping wiki catalog producer and the live command catalog disagree" {
				t.Fatalf("producer %s loss did not fail before rendering: %+v", test.name, issues)
			}
		})
	}
}

// VALIDATES: the no-sibling path checks independent command dimensions in the
// canonical renderer output rather than treating successful process exit as proof.
// PREVENTS: a renderer silently dropping one command's address contract while
// ze-doc-verify has no published sibling to compare.
func TestDocDriftNoSiblingsRejectsMutatedRendererContract(t *testing.T) {
	installCommandRendererMutation(
		t,
		"llms.txt",
		"address-fields address",
		"address-fields",
	)
	root := t.TempDir()
	livePath := writeRenderedCommandCatalogFixture(t, root)

	out, err := runRenderedCommandDriftFixture(t, root, livePath)
	if err == nil {
		t.Fatalf("doc drift accepted a renderer that dropped address fields:\n%s", out)
	}
	if !strings.Contains(out, "generated per-command surface dropped part of the live command contract") {
		t.Fatalf("doc drift did not report the mutated renderer contract:\n%s", out)
	}
	if !strings.Contains(out, `address field "address"`) {
		t.Fatalf("doc drift did not identify the dropped address dimension:\n%s", out)
	}
}

// VALIDATES: the primary Markdown parser keeps the final local-only group
// bounded to the Pipes table cell and requires save under that exact qualifier.
// PREVENTS: a row-level name search passing when save remains under always but
// disappears from the independent local-process-only contract.
func TestDocDriftNoSiblingsRejectsPrimaryMarkdownQualifierMutation(t *testing.T) {
	installCommandRendererMutation(
		t,
		"reference/cli/index.md",
		"<br>Local process only: `save`",
		"",
	)
	root := t.TempDir()
	livePath := writeRenderedCommandCatalogFixture(t, root)

	out, err := runRenderedCommandDriftFixture(t, root, livePath)
	if err == nil {
		t.Fatalf("doc drift accepted primary Markdown without local-only save:\n%s", out)
	}
	if !strings.Contains(out, "reference/cli/index.md") {
		t.Fatalf("doc drift did not identify the primary Markdown row:\n%s", out)
	}
	if !strings.Contains(out, `local-only surface qualifier for operator "save"`) {
		t.Fatalf("doc drift did not identify the exact missing qualifier:\n%s", out)
	}
}

// VALIDATES: canonical renderer execution errors are documentation findings.
// PREVENTS: a broken in-repo renderer degrading into a skipped surface check.
func TestDocDriftRendererErrorsFailClosed(t *testing.T) {
	previous := renderCommandSurfaces
	t.Cleanup(func() {
		renderCommandSurfaces = previous
	})
	renderCommandSurfaces = func(string, []publishedCommand) error {
		return errors.New("fixture renderer failure")
	}
	root := t.TempDir()
	livePath := writeRenderedCommandCatalogFixture(t, root)

	out, err := runRenderedCommandDriftFixture(t, root, livePath)
	if err == nil {
		t.Fatalf("doc drift accepted a failing canonical renderer:\n%s", out)
	}
	if !strings.Contains(out, "could not generate the expected per-command surfaces") {
		t.Fatalf("doc drift did not report expected-surface generation failure:\n%s", out)
	}
	if !strings.Contains(out, "internal/le/sitebuild") {
		t.Fatalf("doc drift did not identify the native site producer:\n%s", out)
	}
	if !strings.Contains(out, "fixture renderer failure") {
		t.Fatalf("doc drift hid the renderer's corrective detail:\n%s", out)
	}
}

// VALIDATES: every published HTML, Markdown, and llms command surface is
// structurally checked against live JSON, and every generated page path exists.
// PREVENTS: current cli-commands.json masking stale human or agent-facing pages.
func TestDocDriftRejectsStaleRenderedCommandSurfaces(t *testing.T) {
	root := t.TempDir()
	livePath := writeRenderedCommandCatalogFixture(t, root)
	writeDoc(t, root, "website/data/cli-commands.json",
		renderedCommandCatalogFixture)
	surfaces := []struct {
		name string
		path string
	}{
		{name: "primary CLI HTML", path: "reference/cli/index.html"},
		{name: "primary CLI Markdown", path: "reference/cli/index.md"},
		{name: "command equivalents HTML", path: "reference/command-equivalents/index.html"},
		{name: "command equivalents Markdown", path: "reference/command-equivalents/index.md"},
		{name: "llms", path: "llms.txt"},
	}
	for _, surface := range surfaces {
		writeDoc(t, root, filepath.Join("website", surface.path),
			"stale rendered command surface\n")
	}

	out, err := runRenderedCommandDriftFixture(t, root, livePath)
	if err == nil {
		t.Fatalf("doc drift accepted stale rendered surfaces with current JSON:\n%s", out)
	}
	for _, surface := range surfaces {
		if !strings.Contains(out, filepath.ToSlash(surface.path)) {
			t.Fatalf("doc drift did not identify stale %s:\n%s", surface.name, out)
		}
	}
	if !strings.Contains(out,
		"the generated per-command surface dropped part of the live command contract") {
		t.Fatalf("doc drift did not structurally reject stale surfaces:\n%s", out)
	}
	if !strings.Contains(out, "reference/command-equivalents/show-test/index.html") {
		t.Fatalf("doc drift did not identify a missing generated detail page:\n%s", out)
	}
	if !strings.Contains(out, "the published per-command surface is missing or unreadable") {
		t.Fatalf("doc drift did not fail closed on missing published surfaces:\n%s", out)
	}
}

// VALIDATES: in-process wiki Markdown is checked structurally even when no
// sibling wiki checkout exists.
// PREVENTS: a missing sibling hiding omitted, extra, or malformed rendered data.
func TestDocDriftNoSiblingWikiFailsClosed(t *testing.T) {
	for _, tc := range []struct {
		name        string
		old         string
		replacement string
		want        string
	}{
		{
			name:        "catalog-absent operator",
			old:         "Always: `json`, `save`",
			replacement: "Always: `json`, `save`, `catalog-absent`",
			want:        "catalog-absent operator",
		},
		{
			name:        "dropped address field",
			old:         "Address fields: `address`\n",
			replacement: "",
			want:        "wiki address fields",
		},
		{
			name:        "malformed summary row",
			old:         "| `show test` | read-only | Show test rows |",
			replacement: "| ``show test`` | read-only | Show test rows |",
			want:        "wiki command summary row",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := runRenderedWikiMutationFixture(
				t, tc.old, tc.replacement,
			)
			if err == nil || !strings.Contains(out, tc.want) {
				t.Fatalf("rendered wiki %s mutation escaped the gate:\n%s", tc.name, out)
			}
		})
	}
}

// VALIDATES: primary HTML and Markdown publish the exact command-owned filter
// and alias sets from the live catalog.
// PREVENTS: the global operator checks masking a dropped command-specific pipe.
func TestDocDriftRejectsDroppedPrimaryFiltersAndAliases(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		old         string
		replacement string
	}{
		{
			name: "HTML filter",
			path: "reference/cli/index.html",
			old:  `<code title="Filter by family">family &lt;value&gt;</code>`,
		},
		{
			name: "HTML alias",
			path: "reference/cli/index.html",
			old:  `<strong>Aliases</strong><dl><dt><code>summary</code></dt><dd>Show a summary <code>display address</code></dd></dl>`,
		},
		{
			name: "Markdown filter",
			path: "reference/cli/index.md",
			old:  "Command: `family <value>`<br>",
		},
		{
			name: "Markdown alias",
			path: "reference/cli/index.md",
			old:  "Aliases: `summary -> display address`<br>",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			livePath := writeRenderedCommandCatalogFixture(t, root)
			writePublishedCommandSurfaceFixture(t, root, false)
			mutatePublishedCommandSurface(t, root, tc.path, tc.old, tc.replacement)

			out, err := runRenderedCommandDriftFixture(t, root, livePath)
			if err == nil {
				t.Fatalf("doc drift accepted a dropped primary %s:\n%s", tc.name, out)
			}
			if !strings.Contains(out, filepath.ToSlash(tc.path)) {
				t.Fatalf("doc drift did not identify the mutated primary surface:\n%s", out)
			}
		})
	}
}

// VALIDATES: every rendered command operator group is an exact set projection,
// not a one-way expected-name search.
// PREVENTS: a stale hard-coded operator surviving after catalog removal.
func TestDocDriftRejectsExtraOperatorsOnEveryRenderedSurface(t *testing.T) {
	tests := []struct {
		name string
		path string
		old  string
		new  string
	}{
		{
			name: "primary HTML",
			path: "reference/cli/index.html",
			old:  "<span>Always</span><code>json · save</code>",
			new:  "<span>Always</span><code>json · save · catalog-absent</code>",
		},
		{
			name: "primary Markdown",
			path: "reference/cli/index.md",
			old:  "Always: `json`, `save`",
			new:  "Always: `json`, `save`, `catalog-absent`",
		},
		{
			name: "command-equivalent HTML",
			path: "reference/command-equivalents/show-test/index.html",
			old:  "<div><dt>Pipes, always</dt><dd>json, save</dd></div>",
			new:  "<div><dt>Pipes, always</dt><dd>json, save, catalog-absent</dd></div>",
		},
		{
			name: "command-equivalent Markdown",
			path: "reference/command-equivalents/show-test/index.md",
			old:  "- Pipes, always: json, save",
			new:  "- Pipes, always: json, save, catalog-absent",
		},
		{
			name: "llms",
			path: "llms.txt",
			old:  "pipes always: json save,",
			new:  "pipes always: json save catalog-absent,",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			livePath := writeRenderedCommandCatalogFixture(t, root)
			writePublishedCommandSurfaceFixture(t, root, false)
			mutatePublishedCommandSurface(t, root, tc.path, tc.old, tc.new)

			out, err := runRenderedCommandDriftFixture(t, root, livePath)
			if err == nil {
				t.Fatalf("doc drift accepted an extra operator on %s:\n%s", tc.name, out)
			}
			if !strings.Contains(out, "catalog-absent operator") ||
				!strings.Contains(out, filepath.ToSlash(tc.path)) {
				t.Fatalf("doc drift did not identify the extra %s operator:\n%s", tc.name, out)
			}
		})
	}
}

// VALIDATES: each command owns at most one correctly labeled operator group
// for an availability on every rendered surface.
// PREVENTS: a renderer hiding a catalog-absent operator in a second group after
// the first group has already satisfied the live contract.
func TestDocDriftRejectsDuplicateOperatorGroupsOnEveryRenderedSurface(t *testing.T) {
	tests := []struct {
		name string
		path string
		old  string
		new  string
	}{
		{
			name: "primary HTML",
			path: "reference/cli/index.html",
			old:  "<p><span>Always</span><code>json · save</code></p>",
			new:  "<p><span>Always</span><code>json · save</code></p><p><span>Always</span><code>catalog-absent</code></p>",
		},
		{
			name: "primary Markdown",
			path: "reference/cli/index.md",
			old:  "Always: `json`, `save`",
			new:  "Always: `json`, `save`<br>Always: `catalog-absent`",
		},
		{
			name: "command-equivalent HTML",
			path: "reference/command-equivalents/show-test/index.html",
			old:  "<div><dt>Pipes, always</dt><dd>json, save</dd></div>",
			new:  "<div><dt>Pipes, always</dt><dd>json, save</dd></div><div><dt>Pipes, always</dt><dd>catalog-absent</dd></div>",
		},
		{
			name: "command-equivalent Markdown",
			path: "reference/command-equivalents/show-test/index.md",
			old:  "- Pipes, always: json, save",
			new:  "- Pipes, always: json, save\n- Pipes, always: catalog-absent",
		},
		{
			name: "llms",
			path: "llms.txt",
			old:  "pipes always: json save,",
			new:  "pipes always: json save, always: catalog-absent,",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			livePath := writeRenderedCommandCatalogFixture(t, root)
			writePublishedCommandSurfaceFixture(t, root, false)
			mutatePublishedCommandSurface(t, root, tc.path, tc.old, tc.new)

			out, err := runRenderedCommandDriftFixture(t, root, livePath)
			if err == nil {
				t.Fatalf("doc drift accepted a duplicate operator group on %s:\n%s", tc.name, out)
			}
			if !strings.Contains(out, "duplicate operator availability group") ||
				!strings.Contains(out, filepath.ToSlash(tc.path)) {
				t.Fatalf("doc drift did not identify the duplicate %s group:\n%s", tc.name, out)
			}
		})
	}

	t.Run("wiki", func(t *testing.T) {
		out, err := runRenderedWikiMutationFixture(
			t,
			"Always: `json`, `save`",
			"Always: `json`, `save`\nAlways: `catalog-absent`",
		)
		if err == nil ||
			!strings.Contains(out, "duplicate operator availability group") ||
			!strings.Contains(out, "internal/le/wikicatalog/render.go") {
			t.Fatalf("doc drift did not identify the duplicate wiki group:\n%s", out)
		}
	})
}

// VALIDATES: every direct Pipes,* term in the command-owned definition list is
// one of the command renderer's exact availability labels.
// PREVENTS: an obsolete pipe group surviving beside all current groups.
func TestDocDriftRejectsUnknownEquivalentHTMLPipeTerm(t *testing.T) {
	root := t.TempDir()
	livePath := writeRenderedCommandCatalogFixture(t, root)
	writePublishedCommandSurfaceFixture(t, root, false)
	mutatePublishedCommandSurface(
		t, root, "reference/command-equivalents/show-test/index.html",
		"<div><dt>Address fields</dt><dd>address</dd></div>",
		"<div><dt>Address fields</dt><dd>address</dd></div>"+
			"<div><dt>Pipes, legacy</dt><dd>nosuchop</dd></div>",
	)

	out, err := runRenderedCommandDriftFixture(t, root, livePath)
	if err == nil ||
		!strings.Contains(out, "malformed operator availability group") ||
		!strings.Contains(out, "Pipes, legacy") {
		t.Fatalf("unknown equivalent HTML pipe term escaped validation:\n%s", out)
	}
}

// VALIDATES: every active Pipes,* list label in the command-equivalent Ze
// section is classified before the four catalog groups are compared.
// PREVENTS: an obsolete Markdown pipe group surviving beside all current groups.
func TestDocDriftRejectsUnknownEquivalentMarkdownPipeLabel(t *testing.T) {
	root := t.TempDir()
	livePath := writeRenderedCommandCatalogFixture(t, root)
	writePublishedCommandSurfaceFixture(t, root, false)
	mutatePublishedCommandSurface(
		t, root, "reference/command-equivalents/show-test/index.md",
		"- Pipe aliases: `summary`: Show a summary (`display address`)",
		"- Pipes, legacy: nosuchop\n"+
			"- Pipe aliases: `summary`: Show a summary (`display address`)",
	)

	out, err := runRenderedCommandDriftFixture(t, root, livePath)
	if err == nil ||
		!strings.Contains(out, "malformed operator availability group") ||
		!strings.Contains(out, "Pipes, legacy") {
		t.Fatalf("unknown equivalent Markdown pipe label escaped validation:\n%s", out)
	}
}

// VALIDATES: primary HTML and Markdown enumerate operator-like labels inside
// the command-owned contract cell rather than querying only expected labels.
// PREVENTS: an obsolete Pipes,* segment surviving beside all current groups.
func TestDocDriftRejectsUnknownPrimaryOperatorLabels(t *testing.T) {
	tests := []struct {
		name string
		path string
		old  string
		new  string
	}{
		{
			name: "HTML",
			path: "reference/cli/index.html",
			old:  "<p><span>Local process only</span><code>save</code></p>",
			new: "<p><span>Local process only</span><code>save</code></p>" +
				"<p><span>Pipes, legacy</span><code>nosuchop</code></p>",
		},
		{
			name: "Markdown",
			path: "reference/cli/index.md",
			old:  "Local process only: `save`",
			new:  "Local process only: `save`<br>Pipes, legacy: `nosuchop`",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			livePath := writeRenderedCommandCatalogFixture(t, root)
			writePublishedCommandSurfaceFixture(t, root, false)
			mutatePublishedCommandSurface(t, root, tc.path, tc.old, tc.new)

			out, err := runRenderedCommandDriftFixture(t, root, livePath)
			if err == nil ||
				!strings.Contains(out, "malformed operator availability group") ||
				!strings.Contains(out, "Pipes, legacy") ||
				!strings.Contains(out, filepath.ToSlash(tc.path)) {
				t.Fatalf("unknown primary %s operator label escaped validation:\n%s", tc.name, out)
			}
		})
	}
}

// VALIDATES: direct non-pipe definition terms remain publication metadata and
// are not mistaken for operator availability groups.
func TestDocDriftAllowsEquivalentHTMLNonPipeTerm(t *testing.T) {
	root := t.TempDir()
	livePath := writeRenderedCommandCatalogFixture(t, root)
	writePublishedCommandSurfaceFixture(t, root, false)
	mutatePublishedCommandSurface(
		t, root, "reference/command-equivalents/show-test/index.html",
		"<div><dt>Address fields</dt><dd>address</dd></div>",
		"<div><dt>Address fields</dt><dd>address</dd></div>"+
			"<div><dt>Publication note</dt><dd>postprocessed</dd></div>",
	)

	if out, err := runRenderedCommandDriftFixture(t, root, livePath); err != nil {
		t.Fatalf("non-pipe equivalent HTML term was rejected:\n%s", out)
	}
}

// VALIDATES: every command is parsed from exactly one command-owned container
// before any operator groups are inspected.
// PREVENTS: a valid first container hiding a catalog-absent operator in a
// duplicate row, article, section, or metadata line.
func TestDocDriftRejectsDuplicateCommandContainersOnEverySurface(t *testing.T) {
	tests := []struct {
		name string
		path string
		old  string
		new  string
	}{
		{
			name: "primary HTML row",
			path: "reference/cli/index.html",
			old:  "</td></tr>\n<footer>",
			new: "</td></tr>\n" +
				`<tr id="cmd-show-test"><td><p><span>Always</span><code>catalog-absent</code></p></td></tr>` +
				"\n<footer>",
		},
		{
			name: "primary Markdown row",
			path: "reference/cli/index.md",
			old:  "| `show test` | Read-only | Show test rows |",
			new: "| `show test` | Read-only | Duplicate | Always: `catalog-absent` |\n" +
				"| `show test` | Read-only | Show test rows |",
		},
		{
			name: "command-equivalent HTML article",
			path: "reference/command-equivalents/show-test/index.html",
			old:  "</article>\n</body>",
			new: "</article>\n" +
				`<article class="cmd-detail-card cmd-detail-ze"><dt>Registry path</dt><dd><code>show test</code></dd><dt>Pipes, always</dt><dd>catalog-absent</dd></article>` +
				"\n</body>",
		},
		{
			name: "command-equivalent Markdown section",
			path: "reference/command-equivalents/show-test/index.md",
			old:  "## Mapping intents",
			new: "## Ze command\n\n- Registry path: `show test`\n" +
				"- Pipes, always: catalog-absent\n\n## Mapping intents",
		},
		{
			name: "llms metadata row",
			path: "llms.txt",
			old:  "): Show test rows\n",
			new: "): Show test rows\n" +
				"- `show test` (read-only; pipes always: catalog-absent): Duplicate\n",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			livePath := writeRenderedCommandCatalogFixture(t, root)
			writePublishedCommandSurfaceFixture(t, root, false)
			mutatePublishedCommandSurface(t, root, tc.path, tc.old, tc.new)

			out, err := runRenderedCommandDriftFixture(t, root, livePath)
			if err == nil {
				t.Fatalf("doc drift accepted duplicate %s container:\n%s", tc.name, out)
			}
			wantMessage := "does not have exactly one command container"
			wantCommand := `command "show test"`
			if tc.name == "primary HTML row" {
				wantMessage = "malformed command container"
				wantCommand = ""
			}
			if !strings.Contains(out, wantMessage) ||
				!strings.Contains(out, filepath.ToSlash(tc.path)) ||
				wantCommand != "" && !strings.Contains(out, wantCommand) {
				t.Fatalf("doc drift did not identify the duplicate %s container:\n%s", tc.name, out)
			}
		})
	}

	t.Run("wiki command detail section", func(t *testing.T) {
		out, err := runRenderedWikiMutationFixture(
			t,
			"### `show test`",
			"### `show test`\nAlways: `catalog-absent`\n\n### `show test`",
		)
		if err == nil ||
			!strings.Contains(out, "does not have exactly one command container") ||
			!strings.Contains(out, "internal/le/wikicatalog/render.go") ||
			!strings.Contains(out, `command "show test"`) {
			t.Fatalf("doc drift did not identify the duplicate wiki command detail:\n%s", out)
		}
	})
}

// VALIDATES: HTML group scanners report a matched opener without its closing
// delimiter even after a valid group has already satisfied the live catalog.
// PREVENTS: an unterminated trailing duplicate being treated as absence or EOF.
func TestDocDriftRejectsMalformedTrailingHTMLOperatorGroups(t *testing.T) {
	tests := []struct {
		name string
		path string
		old  string
		new  string
	}{
		{
			name: "primary HTML",
			path: "reference/cli/index.html",
			old:  "<p><span>Local process only</span><code>save</code></p>\n</td></tr>",
			new: "<p><span>Local process only</span><code>save</code></p>\n" +
				"<p><span>Always</span><code>catalog-absent\n</td></tr>",
		},
		{
			name: "command-equivalent HTML",
			path: "reference/command-equivalents/show-test/index.html",
			old: "<div><dt>Address fields</dt><dd>address</dd></div>\n" +
				"</dl>\n</article>",
			new: "<div><dt>Address fields</dt><dd>address</dd></div>\n" +
				"<div><dt>Pipes, always</dt><dd>catalog-absent\n</dl>\n</article>",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			livePath := writeRenderedCommandCatalogFixture(t, root)
			writePublishedCommandSurfaceFixture(t, root, false)
			mutatePublishedCommandSurface(t, root, tc.path, tc.old, tc.new)

			out, err := runRenderedCommandDriftFixture(t, root, livePath)
			if err == nil {
				t.Fatalf("doc drift accepted a malformed trailing %s group:\n%s", tc.name, out)
			}
			if !strings.Contains(out, "malformed operator availability group") ||
				!strings.Contains(out, filepath.ToSlash(tc.path)) ||
				!strings.Contains(out, `command "show test"`) {
				t.Fatalf("doc drift did not identify the malformed trailing %s group:\n%s", tc.name, out)
			}
		})
	}
}

// VALIDATES: a unique operator group preserves the catalog's ordered names.
// PREVENTS: set-only comparison accepting reordered generated documentation.
func TestDocDriftRejectsReorderedOperatorGroup(t *testing.T) {
	root := t.TempDir()
	livePath := writeRenderedCommandCatalogFixture(t, root)
	writePublishedCommandSurfaceFixture(t, root, false)
	mutatePublishedCommandSurface(
		t,
		root,
		"reference/cli/index.html",
		"<p><span>Always</span><code>json · save</code></p>",
		"<p><span>Always</span><code>save · json</code></p>",
	)

	out, err := runRenderedCommandDriftFixture(t, root, livePath)
	if err == nil {
		t.Fatalf("doc drift accepted a reordered operator group:\n%s", out)
	}
	if !strings.Contains(out, "always operator order") ||
		!strings.Contains(out, "reference/cli/index.html") {
		t.Fatalf("doc drift did not identify the reordered operator group:\n%s", out)
	}
}

// VALIDATES: primary command rows remain identity candidates when their id is
// missing or noncanonical, and their owned contract is still inspected.
// PREVENTS: removing cmd-* from a stale row hiding its identity or operators.
func TestDocDriftRejectsPrimaryRowsWithoutCanonicalIDs(t *testing.T) {
	for _, replacement := range []string{
		"<tr",
		`<tr id="show-test"`,
	} {
		t.Run(replacement, func(t *testing.T) {
			root := t.TempDir()
			livePath := writeRenderedCommandCatalogFixture(t, root)
			writePublishedCommandSurfaceFixture(t, root, false)
			mutatePublishedCommandSurface(
				t, root, "reference/cli/index.html",
				`<tr id="cmd-show-test"`, replacement,
			)
			mutatePublishedCommandSurface(
				t, root, "reference/cli/index.html",
				"<p><span>Always</span><code>json · save</code></p>",
				"<p><span>Always</span><code>json · save</code><code>catalog-absent</code></p>",
			)

			out, err := runRenderedCommandDriftFixture(t, root, livePath)
			if err == nil ||
				!strings.Contains(out, "does not have its canonical cmd-* row ID") ||
				!strings.Contains(out, "catalog-absent operator") {
				t.Fatalf("noncanonical primary row escaped structural validation:\n%s", out)
			}
		})
	}
}

// VALIDATES: every command-like row contributes its visible identity even when
// no canonical id associates it with a live command.
// PREVENTS: catalog-absent command rows disappearing by dropping their id.
func TestDocDriftRejectsCatalogAbsentPrimaryRowWithoutID(t *testing.T) {
	root := t.TempDir()
	livePath := writeRenderedCommandCatalogFixture(t, root)
	writePublishedCommandSurfaceFixture(t, root, false)
	mutatePublishedCommandSurface(
		t, root, "reference/cli/index.html",
		`<tr id="cmd-show-test"><td><code>show test</code>`,
		`<tr><td><code>show removed</code></td><td>Read-only</td>`+
			`<td>Removed</td><td><span>Always</span><code>nosuchop</code></td></tr>`+
			`<tr id="cmd-show-test"><td><code>show test</code>`,
	)

	out, err := runRenderedCommandDriftFixture(t, root, livePath)
	if err == nil ||
		!strings.Contains(out, "show removed") ||
		!strings.Contains(out, "absent from the live command catalog") {
		t.Fatalf("catalog-absent no-id primary row escaped identity validation:\n%s", out)
	}
}

// VALIDATES: a no-ID row with a visible command identity remains a candidate
// even when an extra cell makes its table shape noncanonical.
// PREVENTS: a stale command escaping both malformed-container and extra-identity
// accounting by changing its cell count.
func TestDocDriftRejectsFiveCellPrimaryRowWithoutID(t *testing.T) {
	root := t.TempDir()
	livePath := writeRenderedCommandCatalogFixture(t, root)
	writePublishedCommandSurfaceFixture(t, root, false)
	mutatePublishedCommandSurface(
		t, root, "reference/cli/index.html",
		`<tr id="cmd-show-test"><td><code>show test</code>`,
		`<tr><td><code>show removed</code></td><td>Read-only</td>`+
			`<td>Removed</td><td></td><td>unexpected</td></tr>`+
			`<tr id="cmd-show-test"><td><code>show test</code>`,
	)

	out, err := runRenderedCommandDriftFixture(t, root, livePath)
	if err == nil ||
		!strings.Contains(out, "malformed command container") ||
		!strings.Contains(out, "show removed") ||
		!strings.Contains(out, "absent from the live command catalog") {
		t.Fatalf("five-cell no-ID primary row escaped closed validation:\n%s", out)
	}
}

// VALIDATES: a labeled primary operator segment is consumed through its next
// label boundary rather than ending at the first code element.
// PREVENTS: a trailing sibling code element hiding a catalog-absent operator.
func TestDocDriftRejectsTrailingPrimaryOperatorSegmentContent(t *testing.T) {
	for _, tc := range []struct {
		name, suffix, want string
	}{
		{"catalog-absent code", "<code>nosuchop</code>", "catalog-absent operator"},
		{"text", "trailing text", "malformed operator availability group"},
		{"element", "<em>trailing element</em>", "malformed operator availability group"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			livePath := writeRenderedCommandCatalogFixture(t, root)
			writePublishedCommandSurfaceFixture(t, root, false)
			mutatePublishedCommandSurface(
				t, root, "reference/cli/index.html",
				"<p><span>Always</span><code>json · save</code></p>",
				"<p><span>Always</span><code>json · save</code>"+tc.suffix+"</p>",
			)

			out, err := runRenderedCommandDriftFixture(t, root, livePath)
			if err == nil || !strings.Contains(out, tc.want) {
				t.Fatalf("trailing %s escaped validation:\n%s", tc.name, out)
			}
		})
	}
}

// VALIDATES: every equivalent article's term groups are owned by one dl.
// PREVENTS: valid-looking dt/dd siblings reverting to invalid standalone terms.
func TestDocDriftRejectsEquivalentTermsWithoutDefinitionList(t *testing.T) {
	root := t.TempDir()
	livePath := writeRenderedCommandCatalogFixture(t, root)
	writePublishedCommandSurfaceFixture(t, root, false)
	mutatePublishedCommandSurface(
		t, root, "reference/command-equivalents/show-test/index.html",
		"<article class=\"cmd-detail-card cmd-detail-ze\">\n<dl>\n",
		"<article class=\"cmd-detail-card cmd-detail-ze\">\n",
	)
	mutatePublishedCommandSurface(
		t, root, "reference/command-equivalents/show-test/index.html",
		"\n</dl>\n</article>",
		"\n</article>",
	)

	out, err := runRenderedCommandDriftFixture(t, root, livePath)
	if err == nil || !strings.Contains(out, "definition list structure") {
		t.Fatalf("standalone equivalent terms escaped structural validation:\n%s", out)
	}
}

// VALIDATES: the primary HTML operator reference preserves catalog-owned class
// and description, with unrelated site postprocessing still allowed.
// PREVENTS: operator names remaining current while their explanatory metadata drifts.
func TestDocDriftRejectsStaleRenderedOperatorMetadata(t *testing.T) {
	for _, tc := range []struct {
		name string
		old  string
		new  string
	}{
		{name: "class", old: "Output and control", new: "Stale class"},
		{name: "description", old: "JSON output", new: "Stale description"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			livePath := writeRenderedCommandCatalogFixture(t, root)
			writePublishedCommandSurfaceFixture(t, root, false)
			mutatePublishedCommandSurface(
				t, root, "reference/cli/index.html", tc.old, tc.new,
			)

			out, err := runRenderedCommandDriftFixture(t, root, livePath)
			if err == nil {
				t.Fatalf("doc drift accepted a stale operator %s:\n%s", tc.name, out)
			}
			if !strings.Contains(out, "generated operator metadata disagrees") ||
				!strings.Contains(out, tc.name) {
				t.Fatalf("doc drift did not identify stale operator %s:\n%s", tc.name, out)
			}
		})
	}
}

// VALIDATES: HTML containers are tokenizer-built trees bound by exact row id or
// Ze article class plus exact Registry path, and close before the next peer.
// PREVENTS: a later row/article close repairing the expected command's opener.
func TestDocDriftRejectsMalformedStructuredHTMLContainers(t *testing.T) {
	tests := []struct {
		name string
		path string
		old  string
		new  string
	}{
		{
			name: "primary row missing close before next row",
			path: "reference/cli/index.html",
			old:  "</td></tr>\n<footer>",
			new: "</td>\n" +
				`<tr id="cmd-next"><td>next</td></tr>` + "\n<footer>",
		},
		{
			name: "Ze article missing close before next article",
			path: "reference/command-equivalents/show-test/index.html",
			old: "<div><dt>Address fields</dt><dd>address</dd></div>\n" +
				"</dl>\n</article>",
			new: "<div><dt>Address fields</dt><dd>address</dd></div>\n</dl>\n" +
				`<article class="cmd-detail-card"><p>next</p></article>`,
		},
		{
			name: "partial primary row opener",
			path: "reference/cli/index.html",
			old:  `<tr id="cmd-show-test"><td><code>show test</code>`,
			new:  `<tr id="cmd-show-test"<td><code>show test</code>`,
		},
		{
			name: "partial Ze article opener",
			path: "reference/command-equivalents/show-test/index.html",
			old: "<article class=\"cmd-detail-card cmd-detail-ze\">\n<dl>\n" +
				"<div><dt>Registry path</dt><dd><code>show test</code></dd></div>",
			new: "<article class=\"cmd-detail-card cmd-detail-ze\"\n<dl>\n" +
				"<div><dt>Registry path</dt><dd><code>show test</code></dd></div>",
		},
		{
			name: "wrong Registry path",
			path: "reference/command-equivalents/show-test/index.html",
			old:  "<dt>Registry path</dt><dd><code>show test</code></dd>",
			new:  "<dt>Registry path</dt><dd><code>show test wrong</code></dd>",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			livePath := writeRenderedCommandCatalogFixture(t, root)
			writePublishedCommandSurfaceFixture(t, root, false)
			mutatePublishedCommandSurface(t, root, tc.path, tc.old, tc.new)

			out, err := runRenderedCommandDriftFixture(t, root, livePath)
			if err == nil {
				t.Fatalf("doc drift accepted malformed %s:\n%s", tc.name, out)
			}
			if !strings.Contains(out, filepath.ToSlash(tc.path)) ||
				!strings.Contains(out, `command "show test"`) {
				t.Fatalf("doc drift did not bind malformed %s to show test:\n%s", tc.name, out)
			}
		})
	}
}

// VALIDATES: an article's visible Registry path is recovered independently
// from whether its definition markup is canonical.
// PREVENTS: malformed same-command articles escaping duplicate detection by
// inserting elements between the term and definition or around the code.
func TestDocDriftRejectsMalformedSamePathHTMLIdentity(t *testing.T) {
	root := t.TempDir()
	livePath := writeRenderedCommandCatalogFixture(t, root)
	writePublishedCommandSurfaceFixture(t, root, false)
	mutatePublishedCommandSurface(
		t,
		root,
		"reference/command-equivalents/show-test/index.html",
		"</article>\n</body>",
		"</article>\n"+
			`<article class="cmd-detail-card cmd-detail-ze">`+
			`<dt>Registry path</dt><span>intervening</span>`+
			`<dd><span><code>show test</code></span></dd></article>`+
			"\n</body>",
	)

	out, err := runRenderedCommandDriftFixture(t, root, livePath)
	if err == nil {
		t.Fatalf("doc drift accepted a malformed same-path article:\n%s", out)
	}
	if !strings.Contains(out, "does not have exactly one command container") ||
		!strings.Contains(out, `command "show test"`) {
		t.Fatalf("doc drift did not attribute the malformed article to show test:\n%s", out)
	}
}

// VALIDATES: Markdown heading identity follows rendered inline HTML text while
// canonicality remains byte-exact.
// PREVENTS: tags, comments, and entities hiding a malformed Ze command heading.
func TestDocDriftRejectsVisibleNoncanonicalZeHeadings(t *testing.T) {
	for _, heading := range []string{
		"## Ze <span>command</span>",
		"## Ze <!--publication note-->command",
		"## Ze&#32;command",
	} {
		t.Run(heading, func(t *testing.T) {
			_, source := markdownATXHeading(heading)
			if got := markdownRenderedHeadingIdentity(source); got != "Ze command" {
				t.Fatalf("heading %q renders as %q; want %q", heading, got, "Ze command")
			}
			root := t.TempDir()
			livePath := writeRenderedCommandCatalogFixture(t, root)
			writePublishedCommandSurfaceFixture(t, root, false)
			mutatePublishedCommandSurface(
				t,
				root,
				"reference/command-equivalents/show-test/index.md",
				"## Ze command",
				heading,
			)

			out, err := runRenderedCommandDriftFixture(t, root, livePath)
			if err == nil {
				t.Fatalf("doc drift accepted visible noncanonical heading %q:\n%s", heading, out)
			}
			if !strings.Contains(out, `command "show test"`) {
				t.Fatalf("doc drift did not bind heading %q to the Ze container:\n%s", heading, out)
			}
		})
	}
}

// VALIDATES: command-like row and article elements nested directly inside a
// captured container are also independent identity candidates.
// PREVENTS: nesting a catalog-absent or malformed command root inside a live
// command container to hide it from identity validation.
func TestDocDriftRejectsDirectChildHTMLContainers(t *testing.T) {
	tests := []struct {
		name string
		path string
		old  string
		new  string
	}{
		{
			name: "row",
			path: "reference/cli/index.html",
			old:  "</td></tr>\n<footer>",
			new:  `</td><tr id="cmd-nested"><td>nested</td></tr></tr>` + "\n<footer>",
		},
		{
			name: "command-like article",
			path: "reference/command-equivalents/show-test/index.html",
			old: "<div><dt>Address fields</dt><dd>address</dd></div>\n" +
				"</dl>\n</article>",
			new: "<div><dt>Address fields</dt><dd>address</dd></div>\n</dl>\n" +
				`<article class="cmd-detail-card cmd-detail-ze"><p>nested</p></article>` +
				"\n</article>",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			livePath := writeRenderedCommandCatalogFixture(t, root)
			writePublishedCommandSurfaceFixture(t, root, false)
			mutatePublishedCommandSurface(t, root, tc.path, tc.old, tc.new)

			out, err := runRenderedCommandDriftFixture(t, root, livePath)
			if err == nil || !strings.Contains(out, "absent from the live command catalog") {
				t.Fatalf("doc drift accepted a direct child %s:\n%s", tc.name, out)
			}
		})
	}
}

// VALIDATES: closing a captured container's ancestor exposes the missing close
// before a later candidate starts at the captured container's parent level.
// PREVENTS: a true peer's close repairing a malformed command article.
func TestDocDriftRejectsMissingCloseBeforeTruePeer(t *testing.T) {
	root := t.TempDir()
	livePath := writeRenderedCommandCatalogFixture(t, root)
	writePublishedCommandSurfaceFixture(t, root, false)
	const path = "reference/command-equivalents/show-test/index.html"
	mutatePublishedCommandSurface(
		t,
		root,
		path,
		"<article class=\"cmd-detail-card cmd-detail-ze\">\n<dl>\n"+
			"<div><dt>Registry path</dt><dd><code>show test</code></dd></div>",
		"<section><article class=\"cmd-detail-card cmd-detail-ze\">\n<dl>\n"+
			"<div><dt>Registry path</dt><dd><code>show test</code></dd></div>",
	)
	mutatePublishedCommandSurface(
		t,
		root,
		path,
		"<div><dt>Address fields</dt><dd>address</dd></div>\n</dl>\n</article>\n</body>",
		"<div><dt>Address fields</dt><dd>address</dd></div>\n</dl>\n</section>\n"+
			`<article class="cmd-detail-card cmd-detail-ze">`+
			`<dt>Registry path</dt><dd><code>show test extra</code></dd>`+
			"</article>\n</body>",
	)

	out, err := runRenderedCommandDriftFixture(t, root, livePath)
	if err == nil {
		t.Fatalf("doc drift accepted a missing close before a true peer:\n%s", out)
	}
	if !strings.Contains(out, filepath.ToSlash(path)) ||
		!strings.Contains(out, `command "show test"`) {
		t.Fatalf("doc drift did not bind the missing close to show test:\n%s", out)
	}
}

// VALIDATES: CommonMark code spans replace line endings with spaces and trim
// exactly one boundary space only when the content is not all spaces.
// PREVENTS: Fields-style collapsing from changing command identity.
func TestNormalizeMarkdownCodeSpanCommonMarkWhitespace(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "plain", input: "show test", want: "show test"},
		{name: "one boundary space", input: " show test ", want: "show test"},
		{name: "two boundary spaces", input: "  show test  ", want: " show test "},
		{name: "internal spaces", input: "show  test", want: "show  test"},
		{name: "tabs", input: " \tshow test\t ", want: "\tshow test\t"},
		{name: "all spaces", input: "   ", want: "   "},
		{name: "line feed", input: "show\ntest", want: "show test"},
		{name: "CRLF", input: " show\r\ntest ", want: "show test"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := normalizeMarkdownCodeSpan(tc.input); got != tc.want {
				t.Fatalf("normalizeMarkdownCodeSpan(%q) = %q; want %q", tc.input, got, tc.want)
			}
		})
	}
}

// VALIDATES: complete matching-run code values are decoded before inline HTML
// rendering, preserving literal angle-bracket argument notation.
// PREVENTS: `<value>` being mistaken for an HTML element and disappearing.
func TestCommandMarkdownGroupsPreserveLiteralCodeAngles(t *testing.T) {
	row := "| `show test` | read-only | Show test rows | Command: `family <value>` |"
	groups := commandMarkdownGroups(row, "Command")
	if len(groups) != 1 || len(groups[0]) != 1 || groups[0][0] != "family <value>" {
		t.Fatalf("command Markdown groups = %#v; want [[family <value>]]", groups)
	}
}

// VALIDATES: CommonMark normalization determines the exact parsed identity and
// therefore whether an added row is unrelated or a second live-command row.
// PREVENTS: an error from a zero-count misclassification satisfying a mutation
// which is specifically meant to prove duplicate attribution.
func TestDocDriftUsesExactCommonMarkCodeSpanIdentity(t *testing.T) {
	tests := []struct {
		name         string
		identity     string
		wantIdentity string
		wantCount    int
	}{
		{
			name: "two boundary spaces", identity: "  show test  ",
			wantIdentity: " show test ", wantCount: 1,
		},
		{
			name: "internal double space", identity: "show  test",
			wantIdentity: "show  test", wantCount: 1,
		},
		{
			name: "one boundary space", identity: " show test ",
			wantIdentity: "show test", wantCount: 2,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			content := "| `show test` | Read-only | Show test rows |\n" +
				"| `" + tc.identity + "` | Read-only | Extra command |"
			identities := primaryMarkdownCommandIdentities(content)
			if len(identities) != 2 || identities[1] != tc.wantIdentity {
				t.Fatalf("primary identities = %#v; want second identity %q",
					identities, tc.wantIdentity)
			}
			count := 0
			for _, identity := range identities {
				if identity == "show test" {
					count++
				}
			}
			if count != tc.wantCount {
				t.Fatalf("show test attribution count = %d; want %d", count, tc.wantCount)
			}

			root := t.TempDir()
			livePath := writeRenderedCommandCatalogFixture(t, root)
			writePublishedCommandSurfaceFixture(t, root, false)
			mutatePublishedCommandSurface(
				t,
				root,
				"reference/cli/index.md",
				"| `show test` | Read-only | Show test rows |",
				content,
			)
			out, err := runRenderedCommandDriftFixture(t, root, livePath)
			if err == nil {
				t.Fatalf("doc drift accepted identity %q:\n%s", tc.identity, out)
			}
			if tc.wantCount == 2 &&
				!strings.Contains(out, "has 2 primary CLI Markdown command row identity containers") {
				t.Fatalf("duplicate was not attributed exactly twice:\n%s", out)
			}
		})
	}
}

// VALIDATES: primary rows, Ze sections, and llms rows accept only their exact
// opening grammar, even when a canonical container also exists.
// PREVENTS: malformed same-command prefixes being ignored before or after the
// valid container.
func TestDocDriftRejectsMalformedMarkdownCommandOpeners(t *testing.T) {
	tests := []struct {
		name string
		path string
		old  string
		new  string
	}{
		{
			name: "primary row before canonical",
			path: "reference/cli/index.md",
			old:  "| `show test` | Read-only | Show test rows |",
			new:  "  | `show test` malformed row\n| `show test` | Read-only | Show test rows |",
		},
		{
			name: "primary row after canonical",
			path: "reference/cli/index.md",
			old:  "| `show test` | Read-only | Show test rows |",
			new:  "| `show test` | Read-only | Show test rows |\n| `show test | malformed row",
		},
		{
			name: "Ze heading before canonical",
			path: "reference/command-equivalents/show-test/index.md",
			old:  "## Ze command",
			new:  "## Ze command malformed\n\n## Ze command",
		},
		{
			name: "Ze heading after canonical",
			path: "reference/command-equivalents/show-test/index.md",
			old:  "## Mapping intents",
			new:  "## Ze command malformed\n\n## Mapping intents",
		},
		{
			name: "llms malformed row",
			path: "llms.txt",
			old:  "- `show test` (read-only;",
			new:  "  - `show test` read-only; malformed\n- `show test` (read-only;",
		},
		{
			name: "llms unclosed code span",
			path: "llms.txt",
			old:  "- `show test` (read-only;",
			new:  "- `show test (read-only; malformed\n- `show test` (read-only;",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			livePath := writeRenderedCommandCatalogFixture(t, root)
			writePublishedCommandSurfaceFixture(t, root, false)
			mutatePublishedCommandSurface(t, root, tc.path, tc.old, tc.new)

			out, err := runRenderedCommandDriftFixture(t, root, livePath)
			if err == nil {
				t.Fatalf("doc drift accepted malformed %s:\n%s", tc.name, out)
			}
			if !strings.Contains(out, filepath.ToSlash(tc.path)) {
				t.Fatalf("doc drift did not identify malformed %s:\n%s", tc.name, out)
			}
		})
	}
}

// VALIDATES: fenced examples and path-prefix collisions are unrelated content,
// while the canonical containers outside each fence remain uniquely selected.
// PREVENTS: examples or a longer command path becoming a duplicate for show test.
func TestDocDriftIgnoresFencedAndPrefixCollisionExamples(t *testing.T) {
	root := t.TempDir()
	livePath := writeRenderedCommandCatalogFixture(t, root)
	writePublishedCommandSurfaceFixture(t, root, false)
	mutatePublishedCommandSurface(
		t, root, "reference/cli/index.md",
		"| Command | Mode | Description | Pipes |",
		"```markdown\n| `show test` | fenced duplicate |\n```\n"+
			"| Command | Mode | Description | Pipes |",
	)
	mutatePublishedCommandSurface(
		t, root, "reference/command-equivalents/show-test/index.md",
		"## Ze command",
		"~~~markdown\n## Ze command\n- Registry path: `show test`\n~~~\n## Ze command",
	)
	mutatePublishedCommandSurface(
		t, root, "llms.txt",
		"## CLI command surface",
		"## CLI command surface\n\n```\n"+
			"- `show test` (read-only; pipes always: catalog-absent): fenced\n```",
	)

	out, err := runRenderedCommandDriftFixture(t, root, livePath)
	if err != nil {
		t.Fatalf("doc drift counted fenced or prefix-collision examples:\n%s", out)
	}
}

// VALIDATES: wiki detail headings are exact, fence-aware structural headings.
// PREVENTS: malformed same-command headings being ignored or fenced examples
// being counted as a second command detail.
func TestDocDriftWikiHeadingGrammarIsExactAndFenceAware(t *testing.T) {
	const canonical = "### `show test`"
	for _, tc := range []struct {
		name        string
		replacement string
		accept      bool
	}{
		{
			name:        "malformed before",
			replacement: "### `show test` malformed\n" + canonical,
		},
		{
			name:        "malformed after",
			replacement: canonical + "\n### `show test` malformed",
		},
		{
			name: "fenced heading",
			replacement: "~~~markdown\n### `show test`\n~~~\n" +
				canonical,
			accept: true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := runRenderedWikiMutationFixture(
				t, canonical, tc.replacement,
			)
			if tc.accept {
				if err != nil {
					t.Fatalf("doc drift counted a fenced wiki heading:\n%s", out)
				}
				return
			}
			if err == nil || !strings.Contains(out, "wiki command detail section") {
				t.Fatalf("doc drift did not identify %s wiki heading:\n%s", tc.name, out)
			}
		})
	}
}

// VALIDATES: a malformed Registry path definition which still renders the
// expected identity is counted as a same-command article.
// PREVENTS: one valid article hiding a malformed duplicate with the same path.
func TestCommandSurfacesRejectMalformedSamePathRegistryIdentity(t *testing.T) {
	root := t.TempDir()
	livePath := writeRenderedCommandCatalogFixture(t, root)
	writePublishedCommandSurfaceFixture(t, root, false)
	mutatePublishedCommandSurface(t, root,
		"reference/command-equivalents/show-test/index.html",
		"</article>\n</body>",
		"</article>\n"+
			`<article class="cmd-detail-card cmd-detail-ze"><dt>Registry path</dt><dd><code>other command</code></dd><dt>Registry path</dt><dd><code>show test</code></dd></article>`+
			"\n</body>",
	)

	out, err := runRenderedCommandDriftFixture(t, root, livePath)
	if err == nil || !strings.Contains(out, "does not have exactly one command container") {
		t.Fatalf("malformed same-path Registry identity escaped the gate:\n%s", out)
	}
}

// VALIDATES: table rows, list rows, and wiki headings share exact matching-run
// Markdown code-span parsing for two- and three-backtick delimiters.
// PREVENTS: wider delimiters hiding duplicate same-command containers.
func TestCommandSurfacesRecognizeMatchingRunMarkdownCodeSpans(t *testing.T) {
	t.Run("primary table two ticks", func(t *testing.T) {
		root := t.TempDir()
		livePath := writeRenderedCommandCatalogFixture(t, root)
		writePublishedCommandSurfaceFixture(t, root, false)
		mutatePublishedCommandSurface(t, root, "reference/cli/index.md",
			"| `show test` | Read-only | Show test rows |",
			"| ``show test`` | malformed duplicate |\n| `show test` | Read-only | Show test rows |")
		out, err := runRenderedCommandDriftFixture(t, root, livePath)
		if err == nil || !strings.Contains(out, "primary CLI Markdown command row") {
			t.Fatalf("two-tick table identity escaped the gate:\n%s", out)
		}
	})

	t.Run("llms three ticks", func(t *testing.T) {
		root := t.TempDir()
		livePath := writeRenderedCommandCatalogFixture(t, root)
		writePublishedCommandSurfaceFixture(t, root, false)
		mutatePublishedCommandSurface(t, root, "llms.txt",
			"- `show test` (read-only;",
			"- ```show test``` (read-only; pipes always: catalog-absent): malformed\n- `show test` (read-only;")
		out, err := runRenderedCommandDriftFixture(t, root, livePath)
		if err == nil || !strings.Contains(out, "llms.txt command metadata row") {
			t.Fatalf("three-tick list identity escaped the gate:\n%s", out)
		}
	})

	t.Run("duplicate wiki heading with two ticks", func(t *testing.T) {
		accepted, acceptErr := runRenderedWikiMutationFixture(
			t, "### `show test`", "### ``show test``",
		)
		if acceptErr != nil {
			t.Fatalf("valid two-tick wiki heading was rejected:\n%s", accepted)
		}
		out, err := runRenderedWikiMutationFixture(
			t,
			"### `show test`",
			"### ``show test``\nAlways: `catalog-absent`\n\n### `show test`",
		)
		if err == nil ||
			!strings.Contains(out, "does not have exactly one command container") {
			t.Fatalf("two-tick duplicate wiki identity escaped the gate:\n%s", out)
		}
	})
}

// VALIDATES: a backtick fence opener whose info string contains a backtick is
// ordinary active Markdown, not a fence.
// PREVENTS: a malformed opener hiding a trailing duplicate command row.
func TestCommandSurfacesRejectBacktickInfoPseudoFence(t *testing.T) {
	root := t.TempDir()
	livePath := writeRenderedCommandCatalogFixture(t, root)
	writePublishedCommandSurfaceFixture(t, root, false)
	mutatePublishedCommandSurface(t, root, "llms.txt", "): Show test rows\n",
		"): Show test rows\n```markdown`invalid\n"+
			"- `show test` (read-only; pipes always: catalog-absent): duplicate\n")
	out, err := runRenderedCommandDriftFixture(t, root, livePath)
	if err == nil ||
		!strings.Contains(out, "has 2 llms.txt command metadata row identity containers") {
		t.Fatalf("invalid backtick-info opener did not expose exactly two rows:\n%s", out)
	}
}

// VALIDATES: rendered heading identity ignores inline HTML comments while
// canonical source spelling does not.
// PREVENTS: a postprocessor-style comment creating an uncounted Ze heading.
func TestCommandSurfacesRejectCommentedZeHeadingIdentity(t *testing.T) {
	root := t.TempDir()
	livePath := writeRenderedCommandCatalogFixture(t, root)
	writePublishedCommandSurfaceFixture(t, root, false)
	mutatePublishedCommandSurface(t, root,
		"reference/command-equivalents/show-test/index.md", "## Ze command",
		"## Ze command<!--rendered identity-->\n\n## Ze command")
	out, err := runRenderedCommandDriftFixture(t, root, livePath)
	if err == nil || !strings.Contains(out, "does not have exactly one command container") {
		t.Fatalf("commented Ze heading identity escaped the gate:\n%s", out)
	}
}

// VALIDATES: nested command-shaped rows and articles are collected without
// terminating their parent captures.
// PREVENTS: nested command candidates disappearing from identity validation.
func TestCommandSurfacesRejectNestedRowsAndArticles(t *testing.T) {
	root := t.TempDir()
	livePath := writeRenderedCommandCatalogFixture(t, root)
	writePublishedCommandSurfaceFixture(t, root, false)
	mutatePublishedCommandSurface(t, root, "reference/cli/index.html",
		"<p><span>Always</span><code>json · save</code></p>",
		`<div><table><tr id="cmd-nested"><td>nested site row</td></tr></table></div>`+
			"<p><span>Always</span><code>json · save</code></p>")
	mutatePublishedCommandSurface(t, root,
		"reference/command-equivalents/show-test/index.html",
		"<div><dt>Pipes, always</dt><dd>json, save</dd></div>",
		`<div><article class="cmd-detail-card"><p>nested site article</p></article></div>`+
			"<div><dt>Pipes, always</dt><dd>json, save</dd></div>")
	out, err := runRenderedCommandDriftFixture(t, root, livePath)
	if err == nil || !strings.Contains(out, "absent from the live command catalog") {
		t.Fatalf("nested rows or articles escaped identity validation:\n%s", out)
	}
}

// VALIDATES: aggregate command surfaces publish exactly the live command
// identities, not merely at least one container for each live command.
// PREVENTS: stale removed commands surviving beside every current command.
func TestCommandSurfacesRejectExtraAggregateCommands(t *testing.T) {
	tests := []struct {
		name, path, old, replacement string
	}{
		{
			name: "primary HTML",
			path: "reference/cli/index.html",
			old:  `<tr id="cmd-show-test"><td><code>show test</code>`,
			replacement: `<tr id="cmd-show-removed"><td><code>show removed</code></td></tr>` +
				`<tr id="cmd-show-test"><td><code>show test</code>`,
		},
		{
			name: "primary Markdown",
			path: "reference/cli/index.md",
			old:  "| `show test` | Read-only | Show test rows |",
			replacement: "| `show removed` | Read-only | Removed command |\n" +
				"| `show test` | Read-only | Show test rows |",
		},
		{
			name: "malformed primary Markdown",
			path: "reference/cli/index.md",
			old:  "| `show test` | Read-only | Show test rows |",
			replacement: "| ``show removed`` | malformed removed command |\n" +
				"| `show test` | Read-only | Show test rows |",
		},
		{
			name: "llms",
			path: "llms.txt",
			old:  "- `show test` (read-only;",
			replacement: "- `show removed` (read-only; pipes always: nosuchop): stale\n" +
				"- `show test` (read-only;",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			livePath := writeRenderedCommandCatalogFixture(t, root)
			writePublishedCommandSurfaceFixture(t, root, false)
			mutatePublishedCommandSurface(t, root, tc.path, tc.old, tc.replacement)

			out, err := runRenderedCommandDriftFixture(t, root, livePath)
			if err == nil || !strings.Contains(out, "absent from the live command catalog") {
				t.Fatalf("extra aggregate command escaped %s:\n%s", tc.name, out)
			}
		})
	}

	t.Run("llms unrelated section", func(t *testing.T) {
		root := t.TempDir()
		livePath := writeRenderedCommandCatalogFixture(t, root)
		writePublishedCommandSurfaceFixture(t, root, false)
		mutatePublishedCommandSurface(
			t,
			root,
			"llms.txt",
			"# Ze",
			"# Ze\n\n## Other inventory\n\n- `show unrelated`: not a command container",
		)
		if out, err := runRenderedCommandDriftFixture(t, root, livePath); err != nil {
			t.Fatalf("llms code list outside the command surface was rejected:\n%s", out)
		}
	})
}

// VALIDATES: matching delimiter runs are parsed across CommonMark physical-line
// continuations before table, list, and heading identity classification.
// PREVENTS: a line break hiding a malformed duplicate of a live command.
func TestCommandSurfacesRejectMultilineCodeSpanIdentities(t *testing.T) {
	t.Run("primary table", func(t *testing.T) {
		root := t.TempDir()
		livePath := writeRenderedCommandCatalogFixture(t, root)
		writePublishedCommandSurfaceFixture(t, root, false)
		mutatePublishedCommandSurface(t, root, "reference/cli/index.md",
			"| `show test` | Read-only | Show test rows |",
			"| ``show\n  test`` | malformed duplicate |\n"+
				"| `show test` | Read-only | Show test rows |")
		out, err := runRenderedCommandDriftFixture(t, root, livePath)
		if err == nil || !strings.Contains(out, "primary CLI Markdown command row") {
			t.Fatalf("multiline table identity escaped the gate:\n%s", out)
		}
	})

	t.Run("llms CRLF continuation", func(t *testing.T) {
		root := t.TempDir()
		livePath := writeRenderedCommandCatalogFixture(t, root)
		writePublishedCommandSurfaceFixture(t, root, false)
		mutatePublishedCommandSurface(t, root, "llms.txt",
			"- `show test` (read-only;",
			"- `show\r\n  test` (read-only; pipes always: nosuchop): duplicate\n"+
				"- `show test` (read-only;")
		out, err := runRenderedCommandDriftFixture(t, root, livePath)
		if err == nil || !strings.Contains(out, "llms.txt command metadata row") {
			t.Fatalf("multiline CRLF list identity escaped the gate:\n%s", out)
		}
	})

	t.Run("wiki heading", func(t *testing.T) {
		out, err := runRenderedWikiMutationFixture(
			t,
			"### `show test`",
			"### ``show\n    test``\nAlways: `nosuchop`\n\n### `show test`",
		)
		if err == nil ||
			!strings.Contains(out, "does not have exactly one command container") {
			t.Fatalf("multiline heading identity escaped the gate:\n%s", out)
		}
	})
}

// VALIDATES: only ASCII space and tab may follow a CommonMark fence closer.
// PREVENTS: Unicode or form-feed content suffixes exposing fenced command rows.
func TestCommandSurfacesUseASCIIFenceCloserSuffix(t *testing.T) {
	tests := []struct {
		name, suffix string
		wantErr      bool
	}{
		{name: "space", suffix: " ", wantErr: true},
		{name: "tab", suffix: "\t", wantErr: true},
		{name: "non-breaking space", suffix: "\u00a0"},
		{name: "form feed", suffix: "\f"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			livePath := writeRenderedCommandCatalogFixture(t, root)
			writePublishedCommandSurfaceFixture(t, root, false)
			mutatePublishedCommandSurface(
				t,
				root,
				"llms.txt",
				"## CLI command surface",
				"## CLI command surface\n\n```\n```"+tc.suffix+"\n"+
					"- `show test` (read-only; pipes always: nosuchop): example\n```",
			)
			out, err := runRenderedCommandDriftFixture(t, root, livePath)
			if tc.wantErr && err == nil {
				t.Fatalf("ASCII fence closer did not expose the duplicate:\n%s", out)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("invalid fence closer exposed fenced content:\n%s", out)
			}
		})
	}
}

// VALIDATES: Registry identity sees normalized visible labels and every code
// descendant while still requiring a single canonical definition.
// PREVENTS: a malformed same-path article hiding behind label whitespace or a
// preceding unrelated code element.
func TestCommandSurfacesInspectEveryRegistryIdentityCode(t *testing.T) {
	root := t.TempDir()
	livePath := writeRenderedCommandCatalogFixture(t, root)
	writePublishedCommandSurfaceFixture(t, root, false)
	mutatePublishedCommandSurface(t, root,
		"reference/command-equivalents/show-test/index.html",
		"</article>\n</body>",
		"</article>\n"+
			`<article class="cmd-detail-card cmd-detail-ze">`+
			"<dt> Registry\n path </dt><dd><code>other</code>"+
			"<span><code>show test</code></span></dd></article>\n</body>")

	out, err := runRenderedCommandDriftFixture(t, root, livePath)
	if err == nil || !strings.Contains(out, "does not have exactly one command container") {
		t.Fatalf("later Registry identity code escaped the gate:\n%s", out)
	}
}

// VALIDATES: benign HTML wrappers and physical whitespace preserve labeled
// answer-shape and address values on both primary and equivalent surfaces.
// PREVENTS: structural postprocessing being mistaken for contract drift.
func TestCommandSurfacesAcceptPostprocessedLabeledValues(t *testing.T) {
	root := t.TempDir()
	livePath := writeRenderedCommandCatalogFixture(t, root)
	writePublishedCommandSurfaceFixture(t, root, false)
	mutatePublishedCommandSurface(t, root, "reference/cli/index.html",
		"<p><span>Answer shape</span><code>tab</code></p>",
		"<p><span> Answer\n shape </span><code><b>tab</b></code></p>")
	mutatePublishedCommandSurface(t, root, "reference/cli/index.html",
		"<p><span>Address fields</span><code>address</code></p>",
		"<p><span> Address\n fields </span><code><b>address</b></code></p>")
	mutatePublishedCommandSurface(t, root,
		"reference/command-equivalents/show-test/index.html",
		"<div><dt>Answer shape</dt><dd>tab</dd></div>",
		"<div><dt> Answer\n shape </dt><dd><b>tab</b></dd></div>")
	mutatePublishedCommandSurface(t, root,
		"reference/command-equivalents/show-test/index.html",
		"<div><dt>Address fields</dt><dd>address</dd></div>",
		"<div><dt> Address\n fields </dt><dd><b>address</b></dd></div>")

	if out, err := runRenderedCommandDriftFixture(t, root, livePath); err != nil {
		t.Fatalf("benign labeled-value postprocessing was rejected:\n%s", out)
	}
}

// VALIDATES: truncated, suffixed, and prefixed Ze class variants remain
// malformed command-card candidates.
// PREVENTS: a duplicate command article escaping by one class-token mutation.
func TestCommandSurfacesRejectZeClassVariants(t *testing.T) {
	for _, class := range []string{
		"cmd-detail-z",
		"cmd-detail-ze-extra",
		"prefix-cmd-detail-ze",
		"detail-ze",
	} {
		t.Run(class, func(t *testing.T) {
			root := t.TempDir()
			livePath := writeRenderedCommandCatalogFixture(t, root)
			writePublishedCommandSurfaceFixture(t, root, false)
			mutatePublishedCommandSurface(t, root,
				"reference/command-equivalents/show-test/index.html",
				"</article>\n</body>",
				"</article>\n<article class=\"cmd-detail-card "+class+
					"\"><dt>Registry path</dt><dd><code>show test</code></dd></article>\n</body>")
			out, err := runRenderedCommandDriftFixture(t, root, livePath)
			if err == nil || !strings.Contains(out, "does not have exactly one command container") {
				t.Fatalf("Ze class variant %q escaped the gate:\n%s", class, out)
			}
		})
	}
}

// VALIDATES: nested command-container roots are excluded from every parent
// field scan but independently validated as command identities.
// PREVENTS: a child Registry path or operator group contaminating its parent or
// escaping as an uncounted catalog-absent identity.
func TestCommandSurfacesRejectNestedCommandCards(t *testing.T) {
	root := t.TempDir()
	livePath := writeRenderedCommandCatalogFixture(t, root)
	writePublishedCommandSurfaceFixture(t, root, false)
	mutatePublishedCommandSurface(t, root,
		"reference/command-equivalents/show-test/index.html",
		"<div><dt>Pipes, always</dt><dd>json, save</dd></div>",
		`<article class="cmd-detail-card cmd-detail-ze">`+
			`<dt>Registry path</dt><dd><code>show nested</code></dd>`+
			`<dt>Pipes, always</dt><dd>nosuchop</dd></article>`+
			"<div><dt>Pipes, always</dt><dd>json, save</dd></div>")
	mutatePublishedCommandSurface(t, root, "reference/cli/index.html",
		"<p><span>Always</span><code>json · save</code></p>",
		`<table><tr id="cmd-show-nested"><td><code>show nested</code>`+
			`<span>Always</span><code>nosuchop</code></td></tr></table>`+
			"<p><span>Always</span><code>json · save</code></p>")

	out, err := runRenderedCommandDriftFixture(t, root, livePath)
	if err == nil || !strings.Contains(out, "absent from the live command catalog") {
		t.Fatalf("nested command containers escaped identity validation:\n%s", out)
	}
}

// VALIDATES: nested command containers cannot satisfy a parent contract value
// after the parent's own value is removed.
// PREVENTS: bounded tree scans followed by unbounded serialization searches.
func TestCommandSurfacesRejectContractsPresentOnlyInNestedCards(t *testing.T) {
	t.Run("primary filter description", func(t *testing.T) {
		root := t.TempDir()
		livePath := writeRenderedCommandCatalogFixture(t, root)
		writePublishedCommandSurfaceFixture(t, root, false)
		const description = `<details class="cli-pipe-descriptions"><summary>Command pipe descriptions</summary><dl><dt><code>family &lt;value&gt;</code></dt><dd>Filter by family</dd></dl></details>`
		mutatePublishedCommandSurface(
			t,
			root,
			"reference/cli/index.html",
			description,
			`<table><tr id="cmd-nested"><td>`+description+`</td></tr></table>`,
		)
		out, err := runRenderedCommandDriftFixture(t, root, livePath)
		if err == nil || !strings.Contains(out, "command filters") {
			t.Fatalf("nested primary contract satisfied its parent:\n%s", out)
		}
	})

	t.Run("equivalent filter value", func(t *testing.T) {
		root := t.TempDir()
		livePath := writeRenderedCommandCatalogFixture(t, root)
		writePublishedCommandSurfaceFixture(t, root, false)
		mutatePublishedCommandSurface(
			t,
			root,
			"reference/command-equivalents/show-test/index.html",
			`<div><dt>Command pipes</dt><dd><code>family &lt;value&gt;</code>: Filter by family</dd></div>`,
			`<div><dt>Command pipes</dt><dd><article class="cmd-detail-card">`+
				`<code>family &lt;value&gt;</code>: Filter by family`+
				`</article></dd></div>`,
		)
		out, err := runRenderedCommandDriftFixture(t, root, livePath)
		if err == nil || !strings.Contains(out, "command filters") {
			t.Fatalf("nested equivalent contract satisfied its parent:\n%s", out)
		}
	})
}

// VALIDATES: Ze-section identity follows CommonMark emphasis and link rendering,
// in addition to inline HTML rendering.
// PREVENTS: a visibly duplicate heading escaping through non-HTML inline syntax.
func TestCommandSurfacesRejectCommonMarkRenderedZeHeadings(t *testing.T) {
	for _, heading := range []string{
		"## *Ze* command",
		"## [Ze](https://example.invalid/) command",
		"## Ze command\t##",
	} {
		t.Run(heading, func(t *testing.T) {
			root := t.TempDir()
			livePath := writeRenderedCommandCatalogFixture(t, root)
			writePublishedCommandSurfaceFixture(t, root, false)
			mutatePublishedCommandSurface(t, root,
				"reference/command-equivalents/show-test/index.md",
				"## Ze command", heading+"\n\n## Ze command")
			out, err := runRenderedCommandDriftFixture(t, root, livePath)
			if err == nil || !strings.Contains(out, "exactly one command container") {
				t.Fatalf("rendered duplicate Ze heading escaped the gate:\n%s", out)
			}
		})
	}
}

// VALIDATES: every element in a matched visible-value subtree carries its own
// explicit closing token.
// PREVENTS: a closed parent or value root repairing a truncated descendant.
func TestCommandSurfacesRejectUnclosedRegistryAndValueNodes(t *testing.T) {
	tests := []struct {
		name, relative, old, replacement string
	}{
		{
			name:        "primary value",
			relative:    "reference/cli/index.html",
			old:         "<span>Answer shape</span><code>tab</code>",
			replacement: "<span>Answer shape</span><code>tab",
		},
		{
			name:        "Registry value",
			relative:    "reference/command-equivalents/show-test/index.html",
			old:         "<dt>Registry path</dt><dd><code>show test</code></dd>",
			replacement: "<dt>Registry path</dt><dd><code>show test</dd>",
		},
		{
			name:        "primary description descendant",
			relative:    "reference/cli/index.html",
			old:         "<td>Show test rows</td><td>",
			replacement: "<td><span>Show test rows</td><td>",
		},
		{
			name:        "equivalent answer descendant",
			relative:    "reference/command-equivalents/show-test/index.html",
			old:         "<dt>Answer shape</dt><dd>tab</dd>",
			replacement: "<dt>Answer shape</dt><dd><span>tab</dd>",
		},
		{
			name:        "equivalent filter descendant",
			relative:    "reference/command-equivalents/show-test/index.html",
			old:         "Filter by family</dd>",
			replacement: "<em>Filter by family</dd>",
		},
		{
			name:        "equivalent alias descendant",
			relative:    "reference/command-equivalents/show-test/index.html",
			old:         "<code>display address</code>",
			replacement: "<code><b>display address</code>",
		},
		{
			name:        "equivalent index descendant",
			relative:    "reference/command-equivalents/index.html",
			old:         "<code>show test</code>",
			replacement: "<code><span>show test</code>",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			livePath := writeRenderedCommandCatalogFixture(t, root)
			writePublishedCommandSurfaceFixture(t, root, false)
			mutatePublishedCommandSurface(
				t, root, tc.relative, tc.old, tc.replacement,
			)
			if out, err := runRenderedCommandDriftFixture(t, root, livePath); err == nil {
				t.Fatalf("unclosed %s escaped the gate:\n%s", tc.name, out)
			}
		})
	}
}

// VALIDATES: metadata labels are zero-or-one values, including labels absent
// from live, duplicate llms segments, and duplicate Markdown Registry lines.
// PREVENTS: a valid first value hiding stale or contradictory metadata.
func TestCommandSurfacesEnforceMetadataCardinality(t *testing.T) {
	t.Run("absent answer shape", func(t *testing.T) {
		command := publishedCommand{
			Path: "show test", Mode: "read-only", Description: "Show test rows",
		}
		issues := validateLLMSCommandContract(
			"llms.txt", "show test", "read-only; shape tab", "Show test rows", command,
		)
		if len(issues) == 0 {
			t.Fatal("catalog-absent llms answer shape was accepted")
		}
		row := "| `show test` | read-only | Show test rows | Answer shape: `tab` |"
		if issues := validatePrimaryMarkdownContract("index.md", row, command); len(issues) == 0 {
			t.Fatal("catalog-absent Markdown answer shape was accepted")
		}
	})

	t.Run("duplicate llms segment", func(t *testing.T) {
		root := t.TempDir()
		livePath := writeRenderedCommandCatalogFixture(t, root)
		writePublishedCommandSurfaceFixture(t, root, false)
		mutatePublishedCommandSurface(t, root, "llms.txt",
			"; shape tab;", "; shape tab; shape tab;")
		if out, err := runRenderedCommandDriftFixture(t, root, livePath); err == nil {
			t.Fatalf("duplicate llms segment escaped the gate:\n%s", out)
		}
	})

	t.Run("duplicate Markdown Registry line", func(t *testing.T) {
		root := t.TempDir()
		livePath := writeRenderedCommandCatalogFixture(t, root)
		writePublishedCommandSurfaceFixture(t, root, false)
		mutatePublishedCommandSurface(t, root,
			"reference/command-equivalents/show-test/index.md",
			"- Registry path: `show test`",
			"- Registry path: `show test`\n- Registry path: `show test`")
		if out, err := runRenderedCommandDriftFixture(t, root, livePath); err == nil {
			t.Fatalf("duplicate Registry line escaped the gate:\n%s", out)
		}
	})
}

// VALIDATES: primary HTML, Markdown, and llms compare the visible command path,
// mode, and description after renderer-safe inline normalization.
// PREVENTS: current contract metadata hiding a stale visible command summary.
func TestCommandSurfacesCompareVisiblePrimaryValues(t *testing.T) {
	tests := []struct {
		name, relative, old, replacement string
	}{
		{"HTML path", "reference/cli/index.html", "<code>show test</code></td><td>Read-only", "<code>show stale</code></td><td>Read-only"},
		{"HTML mode", "reference/cli/index.html", "<td>Read-only</td><td>Show test rows", "<td>write-only</td><td>Show test rows"},
		{"HTML description", "reference/cli/index.html", "<td>Show test rows</td><td>", "<td>Stale description</td><td>"},
		{"Markdown path", "reference/cli/index.md", "| `show test` | Read-only", "| `show stale` | Read-only"},
		{"Markdown mode", "reference/cli/index.md", "| Read-only | Show test rows |", "| write-only | Show test rows |"},
		{"Markdown description", "reference/cli/index.md", "| Show test rows | Answer shape:", "| Stale description | Answer shape:"},
		{"llms path", "llms.txt", "- `show test` (read-only;", "- `show stale` (read-only;"},
		{"llms mode", "llms.txt", "(read-only; wire", "(write-only; wire"},
		{"llms description", "llms.txt", "): Show test rows", "): Stale description"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			livePath := writeRenderedCommandCatalogFixture(t, root)
			writePublishedCommandSurfaceFixture(t, root, false)
			mutatePublishedCommandSurface(t, root, tc.relative, tc.old, tc.replacement)
			if out, err := runRenderedCommandDriftFixture(t, root, livePath); err == nil {
				t.Fatalf("stale visible %s escaped the gate:\n%s", tc.name, out)
			}
		})
	}
}

// VALIDATES: semantically neutral HTML, emphasis, and link wrappers preserve
// visible primary command values.
// PREVENTS: visible-value checks regressing to serialized-byte comparison.
func TestCommandSurfacesAcceptPostprocessedVisiblePrimaryValues(t *testing.T) {
	root := t.TempDir()
	livePath := writeRenderedCommandCatalogFixture(t, root)
	writePublishedCommandSurfaceFixture(t, root, false)
	mutatePublishedCommandSurface(t, root, "reference/cli/index.html",
		"<code>show test</code></td><td>Read-only</td><td>Show test rows</td>",
		"<code><b>show test</b></code></td><td><em>Read-only</em></td>"+
			"<td><span>Show test rows</span></td>")
	mutatePublishedCommandSurface(t, root, "reference/cli/index.md",
		"| Read-only | Show test rows |",
		"| *Read-only* | [Show test rows](https://example.invalid/) |")
	mutatePublishedCommandSurface(t, root, "llms.txt",
		"): Show test rows", "): <strong>Show test rows</strong>")
	if out, err := runRenderedCommandDriftFixture(t, root, livePath); err != nil {
		t.Fatalf("postprocessed visible values were rejected:\n%s", out)
	}
}

// VALIDATES: both equivalent indexes are exact multisets of visible command
// path plus detail slug, not substring-presence checks.
// PREVENTS: stale paths, slug collisions, duplicates, extras, or missing rows
// surviving while the expected slug marker remains elsewhere.
func TestCommandSurfacesCompareCompleteEquivalentIndexIdentities(t *testing.T) {
	tests := []struct {
		name, relative, old, replacement string
	}{
		{
			"HTML visible path", "reference/command-equivalents/index.html",
			"<code>show test</code>", "<code>show stale</code>",
		},
		{
			"HTML duplicate", "reference/command-equivalents/index.html",
			"</body>", `<tr id="cmd-eq-show-test"><td><code>show test</code></td></tr></body>`,
		},
		{
			"HTML extra", "reference/command-equivalents/index.html",
			"</body>", `<tr id="cmd-eq-show-removed"><td><code>show removed</code></td></tr></body>`,
		},
		{
			"HTML missing", "reference/command-equivalents/index.html",
			`<tr id="cmd-eq-show-test"><td><code>show test</code></td></tr>`, "",
		},
		{
			"HTML slug mismatch", "reference/command-equivalents/index.html",
			"cmd-eq-show-test", "cmd-eq-show-stale",
		},
		{
			"Markdown visible path", "reference/command-equivalents/index.md",
			"`show test`", "`show stale`",
		},
		{
			"Markdown duplicate", "reference/command-equivalents/index.md",
			"| `show test` | Read-only | [details](show-test/) |",
			"| `show test` | Read-only | [details](show-test/) |\n" +
				"| `show test` | Read-only | [details](show-test/) |",
		},
		{
			"Markdown emphasized duplicate", "reference/command-equivalents/index.md",
			"| `show test` | Read-only | [details](show-test/) |",
			"| `show test` | Read-only | [details](show-test/) |\n" +
				"| *`show test`* | Read-only | [details](show-test/) |",
		},
		{
			"Markdown strong extra", "reference/command-equivalents/index.md",
			"| `show test` | Read-only | [details](show-test/) |",
			"| `show test` | Read-only | [details](show-test/) |\n" +
				"| **`show removed`** | Read-only | [details](show-removed/) |",
		},
		{
			"Markdown link-wrapped duplicate", "reference/command-equivalents/index.md",
			"| `show test` | Read-only | [details](show-test/) |",
			"| `show test` | Read-only | [details](show-test/) |\n" +
				"| [`show test`](#identity) | Read-only | [details](show-test/) |",
		},
		{
			"Markdown malformed wrapped duplicate", "reference/command-equivalents/index.md",
			"| `show test` | Read-only | [details](show-test/) |",
			"| `show test` | Read-only | [details](show-test/) |\n" +
				"| *`show test` | Read-only | [details](show-test/) |",
		},
		{
			"Markdown extra", "reference/command-equivalents/index.md",
			"| `show test` | Read-only | [details](show-test/) |",
			"| `show test` | Read-only | [details](show-test/) |\n" +
				"| `show removed` | Read-only | [details](show-removed/) |",
		},
		{
			"Markdown missing", "reference/command-equivalents/index.md",
			"| `show test` | Read-only | [details](show-test/) |", "",
		},
		{
			"Markdown slug mismatch", "reference/command-equivalents/index.md",
			"[details](show-test/)", "[details](show-stale/)",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			livePath := writeRenderedCommandCatalogFixture(t, root)
			writePublishedCommandSurfaceFixture(t, root, false)
			mutatePublishedCommandSurface(
				t, root, tc.relative, tc.old, tc.replacement,
			)
			out, err := runRenderedCommandDriftFixture(t, root, livePath)
			if err == nil || !strings.Contains(out, tc.relative) {
				t.Fatalf("equivalent-index mutation escaped the gate:\n%s", out)
			}
		})
	}
}

// VALIDATES: wrapped command identities inside fenced examples are inactive.
// PREVENTS: wrapper-aware index parsing from counting documentation examples.
func TestCommandSurfacesIgnoreFencedWrappedEquivalentIndexRows(t *testing.T) {
	root := t.TempDir()
	livePath := writeRenderedCommandCatalogFixture(t, root)
	writePublishedCommandSurfaceFixture(t, root, false)
	mutatePublishedCommandSurface(
		t,
		root,
		"reference/command-equivalents/index.md",
		"# Command Equivalents",
		"# Command Equivalents\n\n```markdown\n"+
			"| **`show test`** | Read-only | [details](show-test/) |\n```",
	)
	if out, err := runRenderedCommandDriftFixture(t, root, livePath); err != nil {
		t.Fatalf("fenced wrapped equivalent row was counted:\n%s", out)
	}
}

// VALIDATES: a command-equivalent Markdown detail has one active H1 consisting
// of a complete matching-run code span for the live path.
// PREVENTS: a current Registry line masking a stale, malformed, or duplicate
// page title.
func TestCommandSurfacesValidateEquivalentMarkdownTitle(t *testing.T) {
	tests := []struct {
		name, replacement string
	}{
		{name: "wrong path", replacement: "# `show stale`"},
		{name: "unclosed code span", replacement: "# `show test"},
		{name: "second top-level heading", replacement: "# `show test`\n\n# `other`"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			livePath := writeRenderedCommandCatalogFixture(t, root)
			writePublishedCommandSurfaceFixture(t, root, false)
			mutatePublishedCommandSurface(t, root,
				"reference/command-equivalents/show-test/index.md",
				"# `show test`", tc.replacement)
			out, err := runRenderedCommandDriftFixture(t, root, livePath)
			if err == nil || !strings.Contains(out, "top-level heading") {
				t.Fatalf("invalid equivalent title escaped the gate:\n%s", out)
			}
		})
	}
}

// VALIDATES: equivalent filter and alias details compare code identities and
// normalized visible values while ignoring semantically neutral wrappers.
// PREVENTS: serialized-fragment comparison rejecting site postprocessing.
func TestCommandSurfacesAcceptWrappedEquivalentDetails(t *testing.T) {
	root := t.TempDir()
	livePath := writeRenderedCommandCatalogFixture(t, root)
	writePublishedCommandSurfaceFixture(t, root, false)
	mutatePublishedCommandSurface(t, root,
		"reference/command-equivalents/show-test/index.html",
		"<code>family &lt;value&gt;</code>: Filter by family",
		"<span><code><b>family &lt;value&gt;</b></code></span>: "+
			"<em>Filter</em> by <a href=\"#family\">family</a>")
	mutatePublishedCommandSurface(t, root,
		"reference/command-equivalents/show-test/index.html",
		"<code>summary</code>: Show a summary (<code>display address</code>)",
		"<span><code>summary</code></span>: <em>Show a</em> "+
			"<a href=\"#summary\">summary</a> (<code><b>display address</b></code>)")
	mutatePublishedCommandSurface(t, root,
		"reference/command-equivalents/show-test/index.md",
		"`family <value>`: Filter by family",
		"**`family <value>`**: *Filter* by [family](#family)")
	mutatePublishedCommandSurface(t, root,
		"reference/command-equivalents/show-test/index.md",
		"`summary`: Show a summary (`display address`)",
		"**`summary`**: *Show a* [summary](#summary) (**`display address`**)")
	mutatePublishedCommandSurface(t, root,
		"reference/command-equivalents/show-test/index.md",
		"# `show test`", "# **`show test`**")
	if out, err := runRenderedCommandDriftFixture(t, root, livePath); err != nil {
		t.Fatalf("benign equivalent-detail wrappers were rejected:\n%s", out)
	}
}
func TestCommandSurfacesAcceptExplicitEmptyMarkdownDetails(t *testing.T) {
	command := publishedCommand{Path: "show test"}
	content := strings.Join([]string{
		"# `show test`",
		"",
		"## Ze command",
		"",
		"- Registry path: `show test`",
		"- Command pipes: **none**",
		"- Pipe aliases: *none*",
		"",
		"## Mapping intents",
	}, "\n")
	if issues := validateEquivalentMarkdownContract("index.md", content, command); len(issues) != 0 {
		t.Fatalf("explicit empty details were rejected: %#v", issues)
	}
	malformed := strings.Replace(content, "**none**", "`none`", 1)
	if issues := validateEquivalentMarkdownContract("index.md", malformed, command); len(issues) == 0 {
		t.Fatal("code identity disguised as an empty filter projection was accepted")
	}
}

// VALIDATES: equivalent filter and alias projections preserve identity,
// description, expansion, order, and exact cardinality.
// PREVENTS: a valid first detail hiding a stale or duplicate detail.
func TestCommandSurfacesRejectEquivalentDetailMutations(t *testing.T) {
	tests := []struct {
		name, relative, old, replacement string
	}{
		{
			"HTML filter description",
			"reference/command-equivalents/show-test/index.html",
			"<code>family &lt;value&gt;</code>: Filter by family",
			"<code>family &lt;value&gt;</code>: Stale",
		},
		{
			"HTML alias expansion",
			"reference/command-equivalents/show-test/index.html",
			"(<code>display address</code>)", "(<code>display stale</code>)",
		},
		{
			"HTML duplicate filter",
			"reference/command-equivalents/show-test/index.html",
			"<code>family &lt;value&gt;</code>: Filter by family",
			"<code>family &lt;value&gt;</code>: Filter by family<br>" +
				"<code>family &lt;value&gt;</code>: Filter by family",
		},
		{
			"Markdown filter description",
			"reference/command-equivalents/show-test/index.md",
			"`family <value>`: Filter by family", "`family <value>`: Stale",
		},
		{
			"Markdown alias expansion",
			"reference/command-equivalents/show-test/index.md",
			"(`display address`)", "(`display stale`)",
		},
		{
			"Markdown duplicate alias",
			"reference/command-equivalents/show-test/index.md",
			"`summary`: Show a summary (`display address`)",
			"`summary`: Show a summary (`display address`); " +
				"`summary`: Show a summary (`display address`)",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			livePath := writeRenderedCommandCatalogFixture(t, root)
			writePublishedCommandSurfaceFixture(t, root, false)
			mutatePublishedCommandSurface(
				t, root, tc.relative, tc.old, tc.replacement,
			)
			if out, err := runRenderedCommandDriftFixture(t, root, livePath); err == nil {
				t.Fatalf("equivalent-detail mutation escaped the gate:\n%s", out)
			}
		})
	}
}

// VALIDATES: operator guide rows are structural HTML values with normal
// wrappers and attributes, while every extra or malformed row remains visible
// to exact-set validation.
// PREVENTS: postprocessing hiding a stale operator from the catalog projection.
func TestCommandSurfacesParseStructuralOperatorGuideRows(t *testing.T) {
	t.Run("wrapped visible values", func(t *testing.T) {
		root := t.TempDir()
		livePath := writeRenderedCommandCatalogFixture(t, root)
		writePublishedCommandSurfaceFixture(t, root, false)
		mutatePublishedCommandSurface(
			t, root, "reference/cli/index.html",
			"<tr><td><code>json</code></td><td>Output and control</td><td>Always</td><td>JSON output</td></tr>",
			`<tr data-rendered="true"><td><span><code class="name">json</code></span></td>`+
				`<td><strong>Output and control</strong></td><td><span>Always</span></td>`+
				`<td><em>JSON output</em></td></tr>`,
		)
		if out, err := runRenderedCommandDriftFixture(t, root, livePath); err != nil {
			t.Fatalf("wrapped operator values were rejected:\n%s", out)
		}
	})

	for _, tc := range []struct {
		name, old, replacement string
	}{
		{
			name: "wrapped extra",
			old:  "</tbody></table>\n</section>",
			replacement: `<tr><td><span>removed</span></td><td>Row data</td>` +
				`<td>With rows</td><td>Removed operator</td></tr></tbody></table>` +
				"\n</section>",
		},
		{
			name:        "three cells",
			old:         "<tr><td><code>json</code></td><td>Output and control</td><td>Always</td><td>JSON output</td></tr>",
			replacement: "<tr><td><code>json</code></td><td>Output and control</td><td>Always</td></tr>",
		},
		{
			name:        "unclosed descendant",
			old:         "<td><code>json</code></td>",
			replacement: "<td><span><code>json</code></td>",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			livePath := writeRenderedCommandCatalogFixture(t, root)
			writePublishedCommandSurfaceFixture(t, root, false)
			mutatePublishedCommandSurface(
				t, root, "reference/cli/index.html", tc.old, tc.replacement,
			)
			if out, err := runRenderedCommandDriftFixture(t, root, livePath); err == nil {
				t.Fatalf("%s operator row escaped exact validation:\n%s", tc.name, out)
			}
		})
	}
}

// VALIDATES: primary Markdown and llms aggregate scanners recover a sole code
// span through valid emphasis, strong, or link wrappers and ignore fenced rows.
// PREVENTS: wrappers hiding extra identities or making a live identity vanish.
func TestCommandSurfacesParseWrappedAggregateIdentities(t *testing.T) {
	for _, wrapper := range []struct {
		name, value string
	}{
		{name: "emphasis", value: "*`show test`*"},
		{name: "strong", value: "**`show test`**"},
		{name: "link", value: "[`show test`](#identity)"},
	} {
		t.Run(wrapper.name, func(t *testing.T) {
			table := "| " + wrapper.value + " | Read-only | Description |"
			if got := primaryMarkdownCommandIdentities(table); len(got) != 1 ||
				got[0] != "show test" {
				t.Fatalf("primary wrapped identities = %#v", got)
			}
			list := "- " + wrapper.value + " (read-only): Description"
			if got := llmsCommandIdentities(list); len(got) != 1 ||
				got[0] != "show test" {
				t.Fatalf("llms wrapped identities = %#v", got)
			}
			fenced := "```markdown\n" + table + "\n" + list + "\n```"
			if got := primaryMarkdownCommandIdentities(fenced); len(got) != 0 {
				t.Fatalf("fenced primary wrapped identities = %#v", got)
			}
			if got := llmsCommandIdentities(fenced); len(got) != 0 {
				t.Fatalf("fenced llms wrapped identities = %#v", got)
			}
		})
	}

	for _, surface := range []struct {
		name, path, old, replacement string
	}{
		{
			name: "primary Markdown extra",
			path: "reference/cli/index.md",
			old:  "| `show test` | Read-only | Show test rows |",
			replacement: "| `show test` | Read-only | Show test rows |\n" +
				"| **`show removed`** | Read-only | Removed |",
		},
		{
			name: "llms extra",
			path: "llms.txt",
			old:  "): Show test rows\n",
			replacement: "): Show test rows\n" +
				"- [`show removed`](#identity) (read-only): Removed\n",
		},
	} {
		t.Run(surface.name, func(t *testing.T) {
			root := t.TempDir()
			livePath := writeRenderedCommandCatalogFixture(t, root)
			writePublishedCommandSurfaceFixture(t, root, false)
			mutatePublishedCommandSurface(
				t, root, surface.path, surface.old, surface.replacement,
			)
			out, err := runRenderedCommandDriftFixture(t, root, livePath)
			if err == nil || !strings.Contains(out, "absent from the live command catalog") {
				t.Fatalf("wrapped aggregate extra escaped %s:\n%s", surface.name, out)
			}
		})
	}
}

// VALIDATES: CommonMark emphasis and link delimiters disappear only when they
// form valid inline constructs; unmatched markers remain visible literally.
// PREVENTS: punctuation characters being unconditionally deleted from values.
func TestMarkdownInlineVisibleTextUsesMatchedDelimiters(t *testing.T) {
	for _, tc := range []struct {
		source, visible string
	}{
		{source: "*emphasis*", visible: "emphasis"},
		{source: "_emphasis_", visible: "emphasis"},
		{source: "**strong**", visible: "strong"},
		{source: "__strong__", visible: "strong"},
		{source: "[linked](https://example.invalid/)", visible: "linked"},
		{source: "literal * marker_", visible: "literal * marker_"},
		{source: "under_score stays", visible: "under_score stays"},
		{source: "**unclosed", visible: "**unclosed"},
		{source: "**foo*", visible: "*foo"},
		{source: "€_foo_", visible: "€_foo_"},
	} {
		if got := markdownInlineVisibleText(tc.source); got != tc.visible {
			t.Errorf("markdownInlineVisibleText(%q) = %q; want %q",
				tc.source, got, tc.visible)
		}
	}
}

func TestWikiValidatorRejectsDuplicateAndMalformedGroups(t *testing.T) {
	tests := []struct {
		name, old, replacement string
	}{
		{
			name: "duplicate optional metadata",
			old:  "Answer shape: `tab`",
			replacement: "Answer shape: `tab`\n" +
				"Answer shape: `tab`",
		},
		{
			name: "duplicate alias group",
			old: "Named chains:\n" +
				"- `summary` -- Show a summary (`display address`)",
			replacement: "Named chains:\n" +
				"- `summary` -- Show a summary (`display address`)\n\n" +
				"Named chains:\n" +
				"- `summary` -- Show a summary (`display address`)",
		},
		{
			name: "duplicate filter group",
			old: "Command-specific:\n" +
				"- `family` `<value>` -- Filter by family",
			replacement: "Command-specific:\n" +
				"- `family` `<value>` -- Filter by family\n\n" +
				"Command-specific:\n" +
				"- `family` `<value>` -- Filter by family",
		},
		{
			name:        "operator code-span suffix",
			old:         "Always: `json`, `save`",
			replacement: "Always: `json`suffix, `save`",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			out, err := runRenderedWikiMutationFixture(
				t, test.old, test.replacement,
			)
			if err == nil {
				t.Fatalf("wiki mutation passed validation:\n%s", out)
			}
		})
	}
}

func TestHTMLValidatorRejectsNestedAbsentIdentity(t *testing.T) {
	root := t.TempDir()
	livePath := writeRenderedCommandCatalogFixture(t, root)
	writePublishedCommandSurfaceFixture(t, root, false)
	mutatePublishedCommandSurface(
		t,
		root,
		"reference/command-equivalents/show-test/index.html",
		"\n</article>",
		`<article class="cmd-detail-card cmd-detail-ze">`+
			`<div><dt>Registry path</dt><dd><code>show absent</code></dd></div>`+
			`</article>`+"\n</article>",
	)
	out, err := runRenderedCommandDriftFixture(t, root, livePath)
	if err == nil || !strings.Contains(out, "absent from the live command catalog") {
		t.Fatalf("nested absent identity passed validation:\n%s", out)
	}
}

func TestPrimaryHTMLAliasRequiresOneStructuralDefinitionEntry(t *testing.T) {
	root := t.TempDir()
	livePath := writeRenderedCommandCatalogFixture(t, root)
	writePublishedCommandSurfaceFixture(t, root, false)
	mutatePublishedCommandSurface(
		t,
		root,
		"reference/cli/index.html",
		`<dt><code>summary</code></dt><dd>Show a summary <code>display address</code></dd>`,
		`<dt><template><code>summary</code></template></dt>`+
			`<dd>Show a summary <template><code>display address</code></template></dd>`,
	)
	out, err := runRenderedCommandDriftFixture(t, root, livePath)
	if err == nil || !strings.Contains(out, "pipe aliases") {
		t.Fatalf("inert alias fragments passed validation:\n%s", out)
	}
}

func TestNestedCommonMarkEmphasisCannotImpersonateLiteralMarkers(t *testing.T) {
	const mutation = "*foo **bar** baz*"
	if visible := markdownInlineVisibleText(mutation); visible != "foo bar baz" {
		t.Fatalf("markdownInlineVisibleText(%q) = %q; want %q",
			mutation, visible, "foo bar baz")
	}

	command := publishedCommand{
		Path:        "show emphasis",
		Mode:        "read-only",
		Description: mutation,
	}
	rendered := string(renderPrimaryCommandMarkdown([]publishedCommand{command}))
	drifted := strings.Replace(
		rendered,
		markdownLiteralProse(mutation),
		mutation,
		1,
	)
	if drifted == rendered {
		t.Fatalf("nested emphasis mutation did not apply:\n%s", rendered)
	}
	row, count, malformed := commandSurfaceMarkdownRow(drifted, command.Path)
	if count != 1 || malformed {
		t.Fatalf("mutated row count = %d, malformed = %t:\n%s",
			count, malformed, drifted)
	}
	if issues := validatePrimaryMarkdownContract(
		"index.md", row, command,
	); len(issues) == 0 {
		t.Fatalf("nested emphasis impersonated literal marker text:\n%s", drifted)
	}
}

// VALIDATES: resolving an emphasis pair removes every intervening delimiter
// from the CommonMark delimiter stack before a later closer is considered.
// PREVENTS: crossing emphasis delimiters impersonating catalog prose.
func TestCrossingCommonMarkEmphasisCannotImpersonatePlainText(t *testing.T) {
	const mutation = "*foo _bar* baz_"
	const visible = "foo _bar baz_"
	if got := markdownInlineVisibleText(mutation); got != visible {
		t.Fatalf("markdownInlineVisibleText(%q) = %q; want %q",
			mutation, got, visible)
	}

	command := publishedCommand{
		Path:        "show emphasis",
		Mode:        "read-only",
		Description: "foo bar baz",
	}
	row := "| `show emphasis` | read-only | " + mutation + " |  |"
	if issues := validatePrimaryMarkdownContract(
		"index.md", row, command,
	); len(issues) == 0 {
		t.Fatalf("crossing emphasis rendered as %q and passed as plain text", visible)
	}
}

// VALIDATES: every command-owned identity list is unique before any rendered
// surface is generated.
// PREVENTS: duplicate live identities rendering into self-consistent drift.
func TestParseCommandCatalogRejectsDuplicateOwnedIdentities(t *testing.T) {
	const catalog = `[{
  "path": "show unique",
  "description": "show unique values",
  "mode": "read-only",
  "args": [{"name": "family", "type": "enum"}],
  "pipes": [{"name": "family", "description": "family filter"}],
  "operators": [{"name": "json", "class": "global", "available": "always", "description": "JSON"}],
  "address-fields": ["address"],
  "pipe-aliases": [{"name": "quick", "description": "quick view", "expansion": "family ipv4"}],
  "subcommands": ["brief"]
}]`
	tests := []struct {
		list, old, replacement string
	}{
		{
			list: "args",
			old:  `{"name": "family", "type": "enum"}`,
			replacement: `{"name": "family", "type": "enum"}, ` +
				`{"name": "family", "type": "enum"}`,
		},
		{
			list: "pipes",
			old:  `{"name": "family", "description": "family filter"}`,
			replacement: `{"name": "family", "description": "family filter"}, ` +
				`{"name": "family", "description": "family filter"}`,
		},
		{
			list: "operators",
			old: `{"name": "json", "class": "global", ` +
				`"available": "always", "description": "JSON"}`,
			replacement: `{"name": "json", "class": "global", ` +
				`"available": "always", "description": "JSON"}, ` +
				`{"name": "json", "class": "global", ` +
				`"available": "always", "description": "JSON"}`,
		},
		{
			list:        "address-fields",
			old:         `"address-fields": ["address"]`,
			replacement: `"address-fields": ["address", "address"]`,
		},
		{
			list: "pipe-aliases",
			old: `{"name": "quick", "description": "quick view", ` +
				`"expansion": "family ipv4"}`,
			replacement: `{"name": "quick", "description": "quick view", ` +
				`"expansion": "family ipv4"}, ` +
				`{"name": "quick", "description": "quick view", ` +
				`"expansion": "family ipv4"}`,
		},
		{
			list:        "subcommands",
			old:         `"subcommands": ["brief"]`,
			replacement: `"subcommands": ["brief", "brief"]`,
		},
	}
	for _, test := range tests {
		t.Run(test.list, func(t *testing.T) {
			mutated := strings.Replace(catalog, test.old, test.replacement, 1)
			if mutated == catalog {
				t.Fatalf("%s mutation did not apply", test.list)
			}
			_, err := parseCommandCatalog("duplicate fixture", []byte(mutated))
			if err == nil || !strings.Contains(err.Error(), `"show unique"`) ||
				!strings.Contains(err.Error(), test.list+" list") {
				t.Fatalf("%s duplicate error = %v", test.list, err)
			}
		})
	}
}

// VALIDATES: the wiki's Contents block, group headings, and final total are
// exact structural projections of the live command inventory.
// PREVENTS: duplicate, missing, renamed, or stale navigation surviving while
// every per-command row remains current.
func TestWikiValidatorRejectsCatalogStructureMutations(t *testing.T) {
	const entry = "- [show](#show) (1)"
	tests := []struct {
		name, old, replacement string
	}{
		{name: "contents count", old: entry, replacement: "- [show](#show) (2)"},
		{name: "contents anchor", old: entry, replacement: "- [show](#stale) (1)"},
		{name: "contents label", old: entry, replacement: "- [stale](#show) (1)"},
		{name: "contents missing", old: entry, replacement: ""},
		{name: "contents duplicate", old: entry, replacement: entry + "\n" + entry},
		{name: "contents block duplicate", old: "## Contents", replacement: "## Contents\n\n## Contents"},
		{name: "verb heading missing", old: "## show", replacement: ""},
		{name: "verb heading changed", old: "## show", replacement: "## stale"},
		{name: "verb heading duplicate", old: "## show", replacement: "## show\n\n## show"},
		{name: "total changed", old: "*1 commands total.*", replacement: "*2 commands total.*"},
		{name: "total missing", old: "*1 commands total.*", replacement: ""},
		{name: "total duplicate", old: "*1 commands total.*", replacement: "*1 commands total.*\n*1 commands total.*"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			out, err := runRenderedWikiMutationFixture(
				t, test.old, test.replacement,
			)
			if err == nil {
				t.Fatalf("wiki %s mutation passed validation:\n%s", test.name, out)
			}
		})
	}
}

// VALIDATES: wiki navigation and headings preserve literal backtick and Unicode
// verbs while both sides derive the same rendered heading anchor.
// PREVENTS: raw Markdown syntax changing the visible verb or breaking its link.
func TestWikiValidatorAcceptsLiteralVerbAnchors(t *testing.T) {
	const catalog = `[
  {"path": "` + "`show`" + ` route", "description": "backtick", "mode": "read-only"},
  {"path": "contents route", "description": "reserved", "mode": "read-only"},
  {"path": "show! route", "description": "punctuation", "mode": "read-only"},
  {"path": "show-1 route", "description": "slug collision", "mode": "read-only"},
  {"path": "show? route", "description": "collision", "mode": "read-only"},
  {"path": "表示 route", "description": "unicode", "mode": "read-only"}
]`
	live, err := parseCommandCatalog("literal verb fixture", []byte(catalog))
	if err != nil {
		t.Fatal(err)
	}
	rendered, err := renderExpectedWikiCommandSurface("", "", []byte(catalog))
	if err != nil {
		t.Fatal(err)
	}
	if issues := validateGeneratedWikiCommandSurface(rendered, live); len(issues) != 0 {
		t.Fatalf("literal verb wiki did not validate: %#v\n%s", issues, rendered)
	}
	for _, mutation := range []struct{ old, replacement string }{
		{old: `](#show)`, replacement: `](#stale)`},
		{old: `](#表示)`, replacement: `](#stale)`},
		{old: `](#contents-1)`, replacement: `](#stale)`},
		{old: `](#show-1-1)`, replacement: `](#stale)`},
	} {
		changed := []byte(strings.Replace(
			string(rendered), mutation.old, mutation.replacement, 1,
		))
		if issues := validateGeneratedWikiCommandSurface(changed, live); len(issues) == 0 {
			t.Fatalf("stale wiki anchor %q passed validation:\n%s",
				mutation.replacement, changed)
		}
	}
}

func runRenderedWikiMutationFixture(
	t *testing.T,
	old, replacement string,
) (string, error) {
	t.Helper()
	root := t.TempDir()
	livePath := writeRenderedCommandCatalogFixture(t, root)
	liveRaw := []byte(renderedCommandCatalogFixture)
	live, err := parseCommandCatalog("rendered wiki fixture", liveRaw)
	if err != nil {
		t.Fatal(err)
	}
	rendered, err := renderExpectedWikiCommandSurface(root, livePath, liveRaw)
	if err != nil {
		t.Fatal(err)
	}
	mutated := strings.Replace(string(rendered), old, replacement, 1)
	if mutated == string(rendered) {
		t.Fatalf("rendered wiki mutation %q did not apply", old)
	}
	report := DriftReport{
		Issues: validateGeneratedWikiCommandSurface([]byte(mutated), live),
	}
	if len(report.Issues) == 0 {
		return report.Text(), nil
	}
	return report.Text(), errors.New("documentation drift")
}

func mutatePublishedCommandSurface(
	t *testing.T,
	root, relative, old, replacement string,
) {
	t.Helper()
	path := filepath.Join(root, "website", filepath.FromSlash(relative))
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	mutated := strings.Replace(string(content), old, replacement, 1)
	if mutated == string(content) {
		t.Fatalf("surface mutation %q did not apply to %s", old, relative)
	}
	if err := os.WriteFile(path, []byte(mutated), 0o644); err != nil {
		t.Fatal(err)
	}
}

func installCommandRendererMutation(
	t *testing.T,
	relative, old, replacement string,
) {
	t.Helper()
	previous := renderCommandSurfaces
	t.Cleanup(func() {
		renderCommandSurfaces = previous
	})
	renderCommandSurfaces = func(root string, commands []publishedCommand) error {
		if err := previous(root, commands); err != nil {
			return err
		}
		path := filepath.Join(root, filepath.FromSlash(relative))
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		mutated := strings.Replace(string(content), old, replacement, 1)
		if mutated == string(content) {
			return errors.New("renderer mutation did not apply")
		}
		return os.WriteFile(path, []byte(mutated), 0o644)
	}
}

func TestWikiValidatorAcceptsReservedEmptyHeadingAnchors(t *testing.T) {
	live := []publishedCommand{
		{Path: "!!! route", Mode: "read-only", Description: "ASCII punctuation"},
		{Path: "u--212121 route", Mode: "read-only", Description: "Reserved collision"},
		{Path: "！！！ route", Mode: "read-only", Description: "Unicode punctuation"},
	}
	raw, err := json.Marshal(live)
	if err != nil {
		t.Fatalf("marshal command catalog: %v", err)
	}
	rendered, err := renderExpectedWikiCommandSurface("", "", raw)
	if err != nil {
		t.Fatalf("render wiki command catalog: %v", err)
	}
	content := string(rendered)
	for _, want := range []string{
		"- [\\!\\!\\!](#u--212121) (1)",
		"- [u\\-\\-212121](#u--212121-1) (1)",
		"- [！！！](#u--efbc81efbc81efbc81) (1)",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("wiki catalog omitted reserved anchor %q:\n%s", want, content)
		}
	}
	if issues := validateGeneratedWikiCommandSurface(rendered, live); len(issues) != 0 {
		t.Fatalf("validator rejected canonical reserved anchors: %#v", issues)
	}
	drifted := strings.Replace(content, "#u--212121)", "#)", 1)
	if issues := validateGeneratedWikiCommandSurface([]byte(drifted), live); len(issues) == 0 {
		t.Fatal("validator accepted an empty punctuation-only heading link")
	}
}

func TestWikiValidatorRoundTripsNormalizedDescriptionBreaks(t *testing.T) {
	live := []publishedCommand{
		{Path: "show crlf", Mode: "read-only", Description: "first\r\nsecond"},
		{Path: "clear cr", Mode: "offline", Description: "first\rsecond"},
		{Path: "set mixed", Mode: "offline", Description: "first\r\nsecond\rthird\nfourth"},
	}
	raw, err := json.Marshal(live)
	if err != nil {
		t.Fatalf("marshal command catalog: %v", err)
	}
	rendered, err := renderExpectedWikiCommandSurface("", "", raw)
	if err != nil {
		t.Fatalf("render wiki command catalog: %v", err)
	}
	if issues := validateGeneratedWikiCommandSurface(rendered, live); len(issues) != 0 {
		t.Fatalf("validator rejected normalized descriptions: %#v\n%s", issues, rendered)
	}
	content := string(rendered)
	if strings.ContainsRune(content, '\r') {
		t.Fatalf("canonical wiki catalog retained carriage returns: %q", content)
	}
	for _, want := range []string{
		"| `show crlf` | read-only | first |",
		"### `show crlf`\n\nfirst\nsecond\n\nMode: read-only",
		"| `clear cr` | offline | first |",
		"### `clear cr`\n\nfirst\nsecond\n\nMode: offline",
		"### `set mixed`\n\nfirst\nsecond\nthird\nfourth\n\nMode: offline",
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("normalized wiki catalog omitted %q:\n%s", want, content)
		}
	}
}
