// Overview: command_surfaces.go -- the rendered contract being exercised

package docvalid

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
	report := DriftReport{Issues: (&checker{root: root}).checkPublishedCommandSurfaces(livePath)}
	if len(report.Issues) == 0 {
		return report.Text(), nil
	}
	return report.Text(), errors.New("documentation drift")
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
<tr id="cmd-show-test-extra"><td><p><span>Always</span><code>catalog-absent</code></p></td></tr>
<tr id="cmd-show-test"><td><code>show test</code></td><td>
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
			"| `show test extra` | Read-only | Prefix collision | Always: `catalog-absent` |",
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
<article class="cmd-detail-card cmd-detail-ze"><div><dt>Registry path</dt><dd><code>show test extra</code></dd></div><div><dt>Pipes, always</dt><dd>catalog-absent</dd></div></article>
<article class="cmd-detail-card cmd-detail-ze">
<div><dt>Registry path</dt><dd><code>show test</code></dd></div>
<div><dt>Pipes, always</dt><dd>json, save</dd></div>
<div><dt>Pipes, on its rows</dt><dd>match</dd></div>
<div><dt>Pipes, while streaming</dt><dd>log</dd></div>
<div><dt>Pipes, local process only</dt><dd>save</dd></div>
<div><dt>Command pipes</dt><dd><code>family &lt;value&gt;</code>: Filter by family</dd></div>
<div><dt>Pipe aliases</dt><dd><code>summary</code>: Show a summary (<code>display address</code>)</dd></div>
<div><dt>Answer shape</dt><dd>tab</dd></div>
<div><dt>Address fields</dt><dd>address</dd></div>
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
			"- `show test extra` (read-only; pipes always: catalog-absent): Prefix collision",
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

// VALIDATES: the no-sibling path checks independent command dimensions in the
// canonical renderer output rather than treating successful process exit as proof.
// PREVENTS: a renderer silently dropping one command's address contract while
// ze-doc-verify has no published sibling to compare.
func TestDocDriftNoSiblingsRejectsMutatedRendererContract(t *testing.T) {
	root := t.TempDir()
	livePath := writeRenderedCommandCatalogFixture(t, root)
	sourcePath := filepath.Join(repoRoot(t), "website", "tools", "render-llms-txt.py")
	source, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	mutated := strings.Replace(string(source),
		"if address_fields:",
		"if False and address_fields:",
		1,
	)
	if mutated == string(source) {
		t.Fatal("the llms renderer mutation did not apply")
	}
	writeDoc(t, root, "website/tools/render-llms-txt.py", mutated)

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
	root := t.TempDir()
	livePath := writeRenderedCommandCatalogFixture(t, root)
	sourcePath := filepath.Join(repoRoot(t), "website", "tools", "render-cli-catalog.py")
	source, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	original := `    for availability, names in operators_by_availability(command).items():
        parts.append(`
	replacement := `    for availability, names in operators_by_availability(command).items():
        if availability == "local-only":
            continue
        parts.append(`
	mutationAt := bytes.LastIndex(source, []byte(original))
	if mutationAt == -1 {
		t.Fatal("the primary Markdown qualifier mutation did not apply")
	}
	mutated := make([]byte, 0, len(source)+len(replacement)-len(original))
	mutated = append(mutated, source[:mutationAt]...)
	mutated = append(mutated, replacement...)
	mutated = append(mutated, source[mutationAt+len(original):]...)
	writeDoc(t, root, "website/tools/render-cli-catalog.py", string(mutated))

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
	root := t.TempDir()
	livePath := writeRenderedCommandCatalogFixture(t, root)
	writeDoc(t, root, "website/tools/render-cli-catalog.py",
		`raise SystemExit("fixture renderer failure")`+"\n")

	out, err := runRenderedCommandDriftFixture(t, root, livePath)
	if err == nil {
		t.Fatalf("doc drift accepted a failing canonical renderer:\n%s", out)
	}
	if !strings.Contains(out, "could not generate the expected per-command surfaces") {
		t.Fatalf("doc drift did not report expected-surface generation failure:\n%s", out)
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
			!strings.Contains(out, "scripts/dev/gen_wiki_commands.py") {
			t.Fatalf("doc drift did not identify the duplicate wiki group:\n%s", out)
		}
	})
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
			if !strings.Contains(out, "does not have exactly one command container") ||
				!strings.Contains(out, filepath.ToSlash(tc.path)) ||
				!strings.Contains(out, `command "show test"`) {
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
			!strings.Contains(out, "scripts/dev/gen_wiki_commands.py") ||
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
			old:  "<div><dt>Address fields</dt><dd>address</dd></div>\n</article>",
			new: "<div><dt>Address fields</dt><dd>address</dd></div>\n" +
				"<div><dt>Pipes, always</dt><dd>catalog-absent\n</article>",
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
			old:  "<div><dt>Address fields</dt><dd>address</dd></div>\n</article>",
			new: "<div><dt>Address fields</dt><dd>address</dd></div>\n" +
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
			old: "<article class=\"cmd-detail-card cmd-detail-ze\">\n" +
				"<div><dt>Registry path</dt><dd><code>show test</code></dd></div>",
			new: "<article class=\"cmd-detail-card cmd-detail-ze\"\n" +
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
			old:  "| `show test extra` | Read-only | Prefix collision |",
			new:  "| `show test | malformed row\n| `show test extra` | Read-only | Prefix collision |",
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
// PREVENTS: noncanonical same-command containers hiding behind wider delimiters.
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
	if err == nil || !strings.Contains(out, "does not have exactly one command container") {
		t.Fatalf("invalid backtick-info opener hid a duplicate row:\n%s", out)
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

// VALIDATES: nested command-shaped rows and articles remain descendants, while
// the malformed peer-container mutations above still terminate open captures.
// PREVENTS: valid nested publication markup being mistaken for a peer command.
func TestCommandSurfacesAcceptNestedRowsAndArticles(t *testing.T) {
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
	if err != nil {
		t.Fatalf("nested rows or articles terminated their command capture:\n%s", out)
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
