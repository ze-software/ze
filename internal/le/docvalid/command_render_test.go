// Overview: command_render.go -- native command-surface rendering contracts

package docvalid

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// VALIDATES: punctuation-only command paths may not normalize to an empty
// detail-page slug, and validation runs before the renderer creates output
// files.
// PREVENTS: an empty slug overwriting the aggregate command-equivalents index.
func TestRenderNativeCommandSurfacesRejectsEmptySlug(t *testing.T) {
	root := t.TempDir()
	const path = "!!!"

	err := renderNativeCommandSurfaces(root, []publishedCommand{{Path: path, Mode: "read-only"}})
	if err == nil || !strings.Contains(err.Error(), "slug is empty") ||
		!strings.Contains(err.Error(), path) {
		t.Fatalf("renderNativeCommandSurfaces() error = %v; want empty slug error naming %q", err, path)
	}
	entries, readErr := os.ReadDir(root)
	if readErr != nil {
		t.Fatalf("read output root: %v", readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("renderer wrote %d output entries before rejecting empty slug: %v", len(entries), entries)
	}
}

// VALIDATES: distinct command identities may not map to the same detail-page
// slug, and collision detection runs before the renderer creates output files.
// PREVENTS: the later command silently overwriting the earlier command's page.
func TestRenderNativeCommandSurfacesRejectsSlugCollision(t *testing.T) {
	root := t.TempDir()
	commands := []publishedCommand{
		{Path: "show foo-bar", Mode: "read-only"},
		{Path: "show foo bar", Mode: "read-only"},
	}

	err := renderNativeCommandSurfaces(root, commands)
	if err == nil || !strings.Contains(err.Error(), "slug") ||
		!strings.Contains(err.Error(), "show foo-bar") ||
		!strings.Contains(err.Error(), "show foo bar") {
		t.Fatalf("renderNativeCommandSurfaces() error = %v; want identified slug collision", err)
	}
	entries, readErr := os.ReadDir(root)
	if readErr != nil {
		t.Fatalf("read output root: %v", readErr)
	}
	if len(entries) != 0 {
		t.Fatalf("renderer wrote %d output entries before rejecting collision: %v", len(entries), entries)
	}
}

// VALIDATES: native rendering writes the shared and per-command surfaces for
// every command when their slugs are distinct.
func TestRenderNativeCommandSurfacesRendersMultipleCommands(t *testing.T) {
	root := t.TempDir()
	commands := []publishedCommand{
		{Path: "show bgp summary", Description: "BGP summary", Mode: "read-only"},
		{Path: "show route", Description: "Route table", Mode: "read-only"},
	}

	if err := renderNativeCommandSurfaces(root, commands); err != nil {
		t.Fatalf("renderNativeCommandSurfaces() error = %v", err)
	}

	for _, name := range []string{
		"reference/cli/index.html",
		"reference/cli/index.md",
		"reference/command-equivalents/index.html",
		"reference/command-equivalents/index.md",
		"llms.txt",
	} {
		content, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatalf("read shared surface %s: %v", name, err)
		}
		for _, command := range commands {
			if !strings.Contains(string(content), command.Path) {
				t.Errorf("shared surface %s does not contain command %q", name, command.Path)
			}
		}
	}
	visibleFields := map[string][]string{
		"reference/cli/index.html": {
			`<tr id="cmd-show-bgp-summary"><td><code>show bgp summary</code></td><td>read-only</td><td>BGP summary</td><td>`,
			`<tr id="cmd-show-route"><td><code>show route</code></td><td>read-only</td><td>Route table</td><td>`,
		},
		"reference/cli/index.md": {
			"| `show bgp summary` | read-only | BGP summary |",
			"| `show route` | read-only | Route table |",
		},
		"llms.txt": {
			"- `show bgp summary` (read-only): BGP summary",
			"- `show route` (read-only): Route table",
		},
	}
	for name, expected := range visibleFields {
		content, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatalf("read visible command surface %s: %v", name, err)
		}
		for _, value := range expected {
			if !strings.Contains(string(content), value) {
				t.Errorf("command surface %s does not contain %q", name, value)
			}
		}
	}

	for _, command := range commands {
		for _, name := range []string{"index.html", "index.md"} {
			path := filepath.Join(
				root,
				"reference",
				"command-equivalents",
				commandSurfaceSlug(command.Path),
				name,
			)
			content, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read command surface %s: %v", path, err)
			}
			if !strings.Contains(string(content), command.Path) {
				t.Errorf("command surface %s does not contain command %q", path, command.Path)
			}
		}
	}
}

// VALIDATES: equivalent HTML owns every definition term inside one article
// definition list.
// PREVENTS: dt/dd groups being emitted under div without a valid dl parent.
func TestRenderEquivalentHTMLWrapsTermGroupsInDefinitionList(t *testing.T) {
	command := publishedCommand{
		Path:          "show test",
		AnswerShape:   "tab",
		AddressFields: []string{"address"},
		Operators: []publishedCommandOperator{{
			Name: "json", Available: "always", Description: "JSON output",
		}},
	}
	document := parseRenderedHTML(string(renderEquivalentHTML(&command)))
	node, count, closed := equivalentHTMLCommandContent(document)
	if count != 1 || !closed {
		t.Fatalf("rendered equivalent article is not a closed owned container")
	}
	if !equivalentHTMLDefinitionList(document, node) {
		t.Fatalf("rendered equivalent article does not own one valid definition list")
	}
}

// VALIDATES: native equivalent Markdown escapes detail separators and literal
// backslashes, and the validator recovers each original description and
// expansion without changing detail cardinality.
// PREVENTS: description semicolons becoming phantom filter or alias entries.
func TestRenderEquivalentMarkdownEscapesAndRoundTripsDetails(t *testing.T) {
	command := publishedCommand{
		Path: "show test",
		Pipes: []publishedCommandPipe{
			{
				Name:        "family",
				Description: `Filter; preserve \ path and \; literal \! punctuation`,
				TakesArg:    true,
			},
			{Name: "active", Description: "Keep active rows"},
		},
		Aliases: []publishedCommandAlias{
			{
				Name:        "summary",
				Description: `Summarize; keep \ markers and \; literal \? punctuation`,
				Expansion:   `display address; match \ value and \; literal \! punctuation`,
			},
			{Name: "brief", Description: "Brief rows", Expansion: "display address"},
		},
	}

	rendered := string(renderEquivalentMarkdown(&command))
	for _, want := range []string{
		"`family <value>`: Filter\\; preserve \\\\ path and \\\\\\; literal \\\\\\! punctuation; `active`: Keep active rows",
		"`summary`: Summarize\\; keep \\\\ markers and \\\\\\; literal \\\\\\? punctuation " +
			"(`display address; match \\ value and \\; literal \\! punctuation`); " +
			"`brief`: Brief rows (`display address`)",
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("renderEquivalentMarkdown() omitted escaped detail %q:\n%s", want, rendered)
		}
	}
	if issues := validateEquivalentMarkdownContract("index.md", rendered, &command); len(issues) != 0 {
		t.Fatalf("native equivalent Markdown did not round-trip: %#v\n%s", issues, rendered)
	}

	drifted := strings.Replace(rendered, `Filter\; preserve \\ path`, `Stale\; preserve \\ path`, 1)
	if issues := validateEquivalentMarkdownContract("index.md", drifted, &command); len(issues) == 0 {
		t.Fatalf("escaped description drift passed validation:\n%s", drifted)
	}
}

// VALIDATES: primary Markdown and llms keep each command in one physical
// container by replacing every CRLF, CR, or LF description break with one
// space, without collapsing unrelated spaces or tabs.
// PREVENTS: multiline catalog prose creating phantom aggregate rows.
func TestRenderOneLineDescriptionsRoundTrip(t *testing.T) {
	command := publishedCommand{
		Path:        "show test",
		Mode:        "read-only",
		Description: "first\r\nsecond\rthird\nfourth  \tkept",
	}
	const visible = "first second third fourth  \tkept"

	primary := string(renderPrimaryCommandMarkdown([]publishedCommand{command}))
	if !strings.Contains(primary, "| "+visible+" |") {
		t.Fatalf("primary Markdown did not normalize only line breaks:\n%s", primary)
	}
	row, count, malformed := commandSurfaceMarkdownRow(primary, command.Path)
	if count != 1 || malformed {
		t.Fatalf("primary Markdown row count = %d, malformed = %t:\n%s",
			count, malformed, primary)
	}
	if issues := validatePrimaryMarkdownContract("index.md", row, &command); len(issues) != 0 {
		t.Fatalf("primary Markdown multiline description did not round-trip: %#v", issues)
	}

	llms := string(renderCommandLLMS([]publishedCommand{command}))
	if !strings.Contains(llms, "): "+visible+"\n") {
		t.Fatalf("llms did not normalize only line breaks:\n%s", llms)
	}
	identity, meta, description, count, valid :=
		llmsCommandMetadata(llmsCommandSurfaceContent(llms), command.Path)
	if count != 1 || !valid {
		t.Fatalf("llms metadata count = %d, valid = %t:\n%s", count, valid, llms)
	}
	if issues := validateLLMSCommandContract(
		"llms.txt", identity, meta, description, &command,
	); len(issues) != 0 {
		t.Fatalf("llms multiline description did not round-trip: %#v", issues)
	}
}

// VALIDATES: Markdown prose and code values remain literal while structural
// separators are recognized only outside matching code spans and escapes.
// PREVENTS: catalog punctuation or contract-label prose changing metadata
// cardinality across the primary, detail, and llms surfaces.
func TestRenderMarkdownLiteralValuesRoundTripAcrossSurfaces(t *testing.T) {
	command := publishedCommand{
		Path: "show test",
		Mode: "read-only",
		Description: "Always: literal | *emphasis* [link](target) <tag> \\ path\r\n" +
			"Aliases: prose",
		Pipes: []publishedCommandPipe{{
			Name:        "tick`filter",
			Description: "semi; comma, space | _emphasis_ [link](target) <tag> \\ path\nnext",
			TakesArg:    true,
		}},
		Aliases: []publishedCommandAlias{{
			Name:        "tick``alias",
			Description: "detail; | **strong** <node> \\ tail",
			Expansion:   "display `one`, next; when): done \\ path",
		}},
	}

	primaryMarkdown := string(renderPrimaryCommandMarkdown([]publishedCommand{command}))
	row, count, malformed := commandSurfaceMarkdownRow(primaryMarkdown, command.Path)
	if count != 1 || malformed {
		t.Fatalf("primary Markdown row count = %d, malformed = %t:\n%s",
			count, malformed, primaryMarkdown)
	}
	if issues := validatePrimaryMarkdownContract("index.md", row, &command); len(issues) != 0 {
		t.Fatalf("primary Markdown literal values did not round-trip: %#v\n%s",
			issues, primaryMarkdown)
	}

	postprocessed := strings.NewReplacer(
		`Always\:`, `Always:`,
		`Aliases\:`, `Aliases:`,
	).Replace(primaryMarkdown)
	postprocessedRow, postprocessedCount, postprocessedMalformed :=
		commandSurfaceMarkdownRow(postprocessed, command.Path)
	if postprocessedCount != 1 || postprocessedMalformed {
		t.Fatalf("postprocessed primary Markdown row count = %d, malformed = %t:\n%s",
			postprocessedCount, postprocessedMalformed, postprocessed)
	}
	if issues := validatePrimaryMarkdownContract(
		"index.md", postprocessedRow, &command,
	); len(issues) != 0 {
		t.Fatalf("description contract labels leaked into metadata: %#v\n%s",
			issues, postprocessed)
	}
	equivalentMarkdown := string(renderEquivalentMarkdown(&command))
	if issues := validateEquivalentMarkdownContract(
		"equivalent.md", equivalentMarkdown, &command,
	); len(issues) != 0 {
		t.Fatalf("equivalent Markdown literal values did not round-trip: %#v\n%s",
			issues, equivalentMarkdown)
	}

	llms := string(renderCommandLLMS([]publishedCommand{command}))
	identity, meta, description, count, valid :=
		llmsCommandMetadata(llmsCommandSurfaceContent(llms), command.Path)
	if count != 1 || !valid {
		t.Fatalf("llms metadata count = %d, valid = %t:\n%s", count, valid, llms)
	}
	if issues := validateLLMSCommandContract(
		"llms.txt", identity, meta, description, &command,
	); len(issues) != 0 {
		t.Fatalf("llms literal values did not round-trip: %#v\n%s", issues, llms)
	}

	primaryHTML := parseRenderedHTML(string(renderPrimaryCommandHTML([]publishedCommand{command})))
	primaryRow, primaryCount, primaryClosed := commandSurfaceHTMLRow(
		primaryHTML, command.Path,
	)
	if primaryCount != 1 || !primaryClosed {
		t.Fatalf("primary HTML row count = %d, closed = %t", primaryCount, primaryClosed)
	}
	if issues := validatePrimaryCommandContract(
		"index.html", primaryRow, primaryHTML, &command,
	); len(issues) != 0 {
		t.Fatalf("primary HTML literal values did not round-trip: %#v", issues)
	}
	if issues := validateEquivalentCommandContract(
		"equivalent.html", parseRenderedHTML(string(renderEquivalentHTML(&command))), &command,
	); len(issues) != 0 {
		t.Fatalf("equivalent HTML literal values did not round-trip: %#v", issues)
	}

	drifted := strings.Replace(primaryMarkdown, `\*emphasis\*`, `\*changed\*`, 1)
	if drifted == primaryMarkdown {
		t.Fatalf("primary Markdown visible mutation did not apply:\n%s", primaryMarkdown)
	}
	driftedRow, _, _ := commandSurfaceMarkdownRow(drifted, command.Path)
	if issues := validatePrimaryMarkdownContract(
		"index.md", driftedRow, &command,
	); len(issues) == 0 {
		t.Fatalf("primary Markdown visible mutation passed validation:\n%s", drifted)
	}
}

// VALIDATES: a code literal uses a delimiter wider than every content run and
// applies CommonMark padding only when it is needed to preserve edge content.
// PREVENTS: backticks or edge spaces truncating an alias/filter value.
func TestMarkdownCodeLiteralRoundTripsMatchingDelimiters(t *testing.T) {
	for _, value := range []string{
		"plain",
		"`edge`",
		"inside `` run",
		" leading and trailing ",
		"   ",
	} {
		literal := markdownCodeLiteral(value)
		got, ok := markdownCodeSpan(literal)
		if !ok || got != value {
			t.Errorf("markdownCodeSpan(markdownCodeLiteral(%q)) = %q, %t; literal %q",
				value, got, ok, literal)
		}
	}
}

// VALIDATES: command identities containing Unicode and backticks retain one
// identity on every Markdown surface and receive a nonempty reserved slug.
// PREVENTS: ASCII-only slug normalization dropping identities or fixed
// backtick delimiters truncating paths.
func TestRenderUnicodeBacktickCommandIdentityRoundTrips(t *testing.T) {
	command := publishedCommand{
		Path:        "表示 `経路`",
		Mode:        "read-only",
		Description: "Unicode route",
	}
	slug := commandSurfaceSlug(command.Path)
	if other := commandSurfaceSlug("表示 `経路2`"); other == "" || other == slug {
		t.Fatalf("distinct Unicode identity slug = %q; first slug %q", other, slug)
	}
	if slug == "" || !strings.HasPrefix(slug, "u--") {
		t.Fatalf("commandSurfaceSlug(%q) = %q; want reserved Unicode slug",
			command.Path, slug)
	}
	if asciiSlug := commandSurfaceSlug(slug); asciiSlug == slug {
		t.Fatalf("Unicode slug %q collides with its ASCII spelling", slug)
	}

	root := t.TempDir()
	if err := renderNativeCommandSurfaces(root, []publishedCommand{command}); err != nil {
		t.Fatalf("render Unicode command surfaces: %v", err)
	}
	if issues := validateGeneratedCommandSurfaces(
		root, root, []publishedCommand{command},
	); len(issues) != 0 {
		t.Fatalf("Unicode/backtick identity did not round-trip: %#v", issues)
	}
}

// VALIDATES: every pipe inside primary Markdown metadata code is escaped before
// the code span enters a GFM table, then decoded back to its catalog value.
// PREVENTS: an alias expansion pipe splitting one command into extra cells.
func TestRenderPrimaryMarkdownEscapesAliasExpansionTablePipe(t *testing.T) {
	command := publishedCommand{
		Path:          "show aliases",
		Mode:          "read-only",
		Description:   "Show aliases",
		AnswerShape:   "tab|map",
		AddressFields: []string{`peer\|address`},
		Pipes: []publishedCommandPipe{{
			Name:        "match|up",
			Description: "Match rows",
		}},
		Operators: []publishedCommandOperator{{
			Name:        "json|lines",
			Class:       "global",
			Available:   "always",
			Description: "JSON lines",
		}},
		Aliases: []publishedCommandAlias{{
			Name:        "quick",
			Description: "Quick view",
			Expansion:   "match up | count",
		}},
	}
	rendered := string(renderPrimaryCommandMarkdown([]publishedCommand{command}))
	const encoded = "Aliases: `quick -> match up \\| count`"
	if !strings.Contains(rendered, encoded) {
		t.Fatalf("primary Markdown omitted encoded alias metadata %q:\n%s",
			encoded, rendered)
	}
	row, count, malformed := commandSurfaceMarkdownRow(rendered, command.Path)
	if count != 1 || malformed {
		t.Fatalf("rendered row count = %d, malformed = %t:\n%s",
			count, malformed, rendered)
	}
	cells, valid := markdownTableCells(row)
	if !valid || len(cells) != 4 {
		t.Fatalf("rendered alias row has %d cells, valid = %t:\n%s",
			len(cells), valid, row)
	}
	if issues := validatePrimaryMarkdownContract(
		"index.md", row, &command,
	); len(issues) != 0 {
		t.Fatalf("encoded alias metadata did not round-trip: %#v", issues)
	}

	unescaped := strings.Replace(
		rendered,
		"quick -> match up \\| count",
		"quick -> match up | count",
		1,
	)
	row, count, malformed = commandSurfaceMarkdownRow(unescaped, command.Path)
	if count != 1 || malformed {
		t.Fatalf("unescaped row identity was not recovered: count = %d, malformed = %t",
			count, malformed)
	}
	if issues := validatePrimaryMarkdownContract(
		"index.md", row, &command,
	); len(issues) == 0 {
		t.Fatalf("unescaped alias table pipe passed validation:\n%s", unescaped)
	}
}
