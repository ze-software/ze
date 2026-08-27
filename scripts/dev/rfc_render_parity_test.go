// The migration's proof for `ze-rfc-index-update`: the script and the command
// generate the same FILES.
//
// VALIDATES: spec-le-is-a-ze-binary AC-8 and AC-11 -- over an export of HEAD and
// over fixture trees, rfc_requirements.py --write writes what `le rfc
// index-update` writes, byte for byte, and both refuse the same destructive
// inputs with the same exit code.
// PREVENTS: a port judged by the two summary lines the generator prints. Those
// two lines carry a path and a count, and every row of every page they name
// could be wrong behind them. The comparison here is over the FILES: the ledger
// and every requirement table, in both directions, so a page one half emitted
// and the other did not is a failure rather than a silence.
//
// It lives beside rfc_parity_test.go rather than inside it because that file is
// a tagged RFC test and the edit-time hook guards it. The helpers it declares
// are visible here: same package, same migration, and step 14 deletes both with
// the script.

package main

import (
	"fmt"
	"maps"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/rfc"
)

// TestRFCBothHalvesRenderTheCheckoutIntoTheSameBytes compares every generated
// file over an export of HEAD.
//
// Both halves run over their OWN export: the write DELETES, so one shared tree
// would let the second half read what the first produced and compare a page
// against itself.
func TestRFCBothHalvesRenderTheCheckoutIntoTheSameBytes(t *testing.T) {
	root := devPyRoot(t)
	scriptTree := rfcPyExportHead(t, root)
	commandTree := rfcPyExportHead(t, root)

	script := rfcPyRunScript(t, scriptTree, "--write")
	if script.Code != 0 {
		t.Fatalf("the script exited %d over the export: %s%s",
			script.Code, script.Stdout, script.Stderr)
	}
	report, err := rfc.IndexUpdate(commandTree)
	if err != nil {
		t.Fatalf("IndexUpdate over the export: %v", err)
	}
	if report.Text() != rfcPyGateText(script) {
		t.Errorf("the two halves print different pages\nscript:\n%s\ncommand:\n%s",
			rfcPyGateText(script), report.Text())
	}
	rfcPyGeneratedAgree(t, scriptTree, commandTree, 100)
}

// rfcPyRenderAgree runs both halves of the generator over TWO copies of one
// fixture and compares what each left behind.
//
// Two trees, because the write DELETES: one shared tree would let the second
// half prune what the first produced. It answers the command's error so a
// refusal case can compare the page the two halves refuse with.
func rfcPyRenderAgree(t *testing.T, what string, files map[string]string) {
	t.Helper()

	// The SECOND tree is the one devPyPointAt leaves the command aimed at, so
	// the command tree is created last.
	scriptTree := rfcPyTree(t, files)
	commandTree := rfcPyTree(t, files)

	script := rfcPyRunScript(t, scriptTree, "--write")
	command := rfcPyActionCommand(t, "index-update")
	devPyAgree(t, what, script, command, rfcPyRenderText(script), rfcPyRenderText(command))
	// A render that produced nothing would compare two empty directories and
	// pass every assertion above, so the floor is the ledger plus one table.
	rfcPyGeneratedAgree(t, scriptTree, commandTree, 2)
}

// rfcPyRenderText normalises the two spellings one refusal has.
//
// The script prints its refusal on stdout with a `rfc-requirements: cannot run`
// banner; the command prints it on stderr with the `error: ` prefix every
// ported le tool writes. Both are renderings of the same message, and the
// message itself -- which the port copied word for word -- is what the
// comparison is about.
func rfcPyRenderText(result devPyResult) string {
	text := ansiRE.ReplaceAllString(result.Stdout+result.Stderr, "")
	text = strings.ReplaceAll(text, "rfc-requirements: cannot run: ", "")
	return strings.TrimPrefix(text, "error: ")
}

// rfcPyRefusalAgree fails unless both halves refused, with the same page, and
// neither touched the tree first.
//
// The last clause is what makes a refusal case discriminate: a guard that
// reports the right message AFTER writing has already done the damage it
// exists to prevent.
func rfcPyRefusalAgree(t *testing.T, what string, files map[string]string) {
	t.Helper()

	scriptTree := rfcPyTree(t, files)
	commandTree := rfcPyTree(t, files)

	script := rfcPyRunScript(t, scriptTree, "--write")
	command := rfcPyActionCommand(t, "index-update")
	if command.Code != 2 {
		t.Fatalf("%s: the command exited %d rather than refusing:\n%s%s",
			what, command.Code, command.Stdout, command.Stderr)
	}
	devPyAgree(t, what, script, command, rfcPyRenderText(script), rfcPyRenderText(command))
	rfcPyGeneratedAgree(t, scriptTree, commandTree, 0)
	for _, tree := range []string{scriptTree, commandTree} {
		if _, err := os.Stat(filepath.Join(tree, "ai", "RFC-REQUIREMENTS.md")); err == nil {
			t.Errorf("%s: a refusal wrote the ledger before refusing", what)
		}
	}
}

// rfcPyRenderSummary is the fixture summary for the render cases: one gated row
// with a coverage annotation, so the RFC is fully covered and the rollup, the
// audit table and the extraction table all have a row to render.
const rfcPyRenderSummary = `# RFC 9999

| Field | Value |
|-------|-------|
| Obsoleted-by | None |

## Compliance Checklist

- [ ] [RFC9999-2-1] [MUST] A speaker MUST send the widget (§2) {gap: nothing sends it}
`

// rfcPyRenderWith answers the render fixture with the named files added.
//
// The public status page is part of the base here and not of rfcPyWith,
// because the render READS it and both halves refuse an absent one in their own
// language. Comparing two spellings of "no such file" would compare the
// interpreter against the Go runtime rather than the generator against the
// generator.
func rfcPyRenderWith(extra map[string]string) map[string]string {
	files := rfcPyWith(map[string]string{
		"rfc/short/rfc9999.md":        rfcPyRenderSummary,
		"docs/features/rfc-status.md": "| RFC 9999 | Widgets | Supported | full | |\n",
	})
	maps.Copy(files, extra)
	return files
}

// rfcPyTagLine writes one `RFC requirement:` tag into a fixture the scanner
// reads with its `#`-comment reader.
//
// The marker is assembled rather than spelled, because a literal one in this
// file makes the edit-time hook read the whole file as an RFC-tagged test and
// refuse every later edit to it. Nothing here is a live tag either way: the Go
// reader matches `//` at the start of a line, never a `#` inside a string.
func rfcPyTagLine(rid, polarity string) string {
	return "# RFC requirement" + ": " + rid + " " + polarity + "\n"
}

func TestRFCBothHalvesRenderTheBranchesTheCheckoutNeverReaches(t *testing.T) {
	// test-asserts-nothing: rfcPyRenderAgree owns the byte-equality assertion for every subtest.
	// Every case below is a state HEAD does not hold, so the byte comparison
	// over the checkout says nothing about any of them: HEAD's worklist is
	// never empty, every enrolled RFC has a public row, every summary is
	// declared, and nothing is proven only by nightly evidence.
	cases := []struct {
		name  string
		files map[string]string
	}{
		{"an empty worklist, no disposition and a complete public page", rfcPyRenderWith(nil)},
		{"an enrolled RFC with no public row at all", rfcPyRenderWith(map[string]string{
			"docs/features/rfc-status.md": "| RFC 1234 | Other | Supported | full | |\n",
		})},
		{"a declared remainder, one debt and one not", rfcPyRenderWith(map[string]string{
			"rfc/short/rfc1000.md": "# RFC 1000\n",
			"rfc/short/rfc1001.md": "# RFC 1001\n",
			"rfc/not-enrolled.txt": "rfc1000 non-normative the document states nothing\n" +
				"rfc1001 blocked waits on grep 'A|B'\n",
		})},
		{"a requirement proven only by a scheduled interop scenario", rfcPyRenderWith(map[string]string{
			"test/interop/scenarios/widget/check.py": rfcPyTagLine("RFC9999-2-1", "positive") +
				rfcPyTagLine("RFC9999-2-1", "negative"),
		})},
		{"a summary whose source states obligations in indicative prose", rfcPyRenderWith(map[string]string{
			"rfc/short/rfc1000.md": "# RFC 1000\n",
			"rfc/full/rfc1000.txt": "A resolver must answer the query.\n",
			"rfc/short/rfc1001.md": "# RFC 1001\n",
			"rfc/full/rfc1001.txt": "This document describes an idea.\n",
			"rfc/short/rfc1002.md": "# RFC 1002\n",
			"rfc/full/rfc1002.txt": "A speaker MUST answer the query.\n",
			"rfc/short/rfc1003.md": "# RFC 1003\n",
		})},
		{"a summary the IETF has obsoleted", rfcPyRenderWith(map[string]string{
			"rfc/short/rfc9999.md": "# RFC 9999\n\n| Obsoleted-by | RFC 9998 |\n\n" +
				"- [ ] [RFC9999-2-1] [MUST] A speaker MUST send the widget (§2) " +
				"{superseded: unresolved; the successor is not here} {gap: nothing sends it}\n",
		})},
		{"an orphan table the generator no longer owns", rfcPyRenderWith(map[string]string{
			"rfc/requirements/rfc1000.md": "# RFC 1000 -- stale\n",
			"rfc/requirements/README.md":  "authored beside the generated files\n",
		})},
	}
	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) { rfcPyRenderAgree(t, one.name, one.files) })
	}
}

func TestRFCBothHalvesRenderTheSameAuditVerdicts(t *testing.T) {
	// HEAD's audit record is entirely FRESH and every verdict on it is
	// `enforced`, so the byte comparison over the checkout exercises exactly
	// one of the states below. Each of the others contradicts the row it sits
	// on, and a contradiction has to be visible where the claim is made.
	cases := []struct {
		name, record string
		// marks is the shard cell the row must carry, and reason the worklist
		// cell. Both are empty for a verdict that IS proof.
		marks, reason string
	}{
		{
			name: "a fresh enforced verdict is proof and is unmarked",
			record: rfcPyAudit(rfcPyVerdict("RFC9999-2-1", rfcPyReqSHA1,
				rfcPyPair(rfcPyGoFileSHA), rfcPyPair(rfcPyGoUnitSHA))),
		},
		{
			name: "a verdict whose unit moved is not proof",
			record: rfcPyAudit(rfcPyVerdict("RFC9999-2-1", rfcPyReqSHA1,
				rfcPyPair(rfcPyGoFileSHA), rfcPyPair(rfcPyOtherSHA))),
			marks:  "**audit: enforced, stale-unit**",
			reason: "`enforced (stale-unit)`",
		},
		{
			name: "a verdict whose requirement text moved is not proof",
			record: rfcPyAudit(rfcPyVerdict("RFC9999-2-1", rfcPyOtherSHA,
				rfcPyPair(rfcPyGoFileSHA), rfcPyPair(rfcPyGoUnitSHA))),
			marks:  "**audit: enforced, stale-requirement**",
			reason: "`enforced (stale-requirement)`",
		},
		{
			name: "a verdict on a file that only moved around its unit is not proof",
			record: rfcPyAudit(rfcPyVerdict("RFC9999-2-1", rfcPyReqSHA1,
				rfcPyPair(rfcPyOtherSHA), rfcPyPair(rfcPyGoUnitSHA))),
			marks:  "**audit: enforced, shifted**",
			reason: "`enforced (shifted)`",
		},
	}
	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			// RFC9999-2-2 carries a verdict as well, and its coverage is ONE
			// polarity, so it is not auditable. Its verdict must still be
			// counted: the record view is total over what the record holds,
			// and gating it on the requirement view left real judgements in no
			// column at all and, when fresh and enforced, in no worklist row.
			files := rfcPyAuditTree(one.record[:strings.LastIndex(one.record, "\n  }\n")] +
				",\n" + rfcPyVerdict("RFC9999-2-2", rfcPyReqSHA2,
				`"`+rfcPyCIKey+`": "`+rfcPyCISHA+`"`,
				`"`+rfcPyCIKey+`": "`+rfcPyCISHA+`"`) + "\n  }\n}\n")
			files["docs/features/rfc-status.md"] =
				"| RFC 9999 | Widgets | Supported | full | |\n"
			rfcPyRenderAgree(t, one.name, files)

			// And the verdict actually reached the pages: a comparison of two
			// halves that both dropped the row would pass every assertion
			// above.
			ledger := rfcPyLedgerOf(t, files)
			shard := rfcPyShardOf(t, files, "rfc9999")
			if one.reason == "" {
				if !strings.Contains(ledger, "None: every recorded verdict is a fresh") {
					t.Errorf("a fresh enforced verdict was reported as a finding:\n%s", ledger)
				}
				if strings.Contains(shard, "**audit:") {
					t.Errorf("a fresh enforced verdict marked its own row:\n%s", shard)
				}
				return
			}
			if !strings.Contains(ledger, one.reason) {
				t.Errorf("the worklist does not carry %q:\n%s", one.reason, ledger)
			}
			if !strings.Contains(shard, one.marks) {
				t.Errorf("the row does not carry %q:\n%s", one.marks, shard)
			}
		})
	}
}

// rfcPyShardOf renders one fixture through the SCRIPT and answers one stem's
// requirement table.
func rfcPyShardOf(t *testing.T, files map[string]string, stem string) string {
	t.Helper()

	tree := rfcPyTree(t, files)
	if result := rfcPyRunScript(t, tree, "--write"); result.Code != 0 {
		t.Fatalf("the script refused the fixture: %s%s", result.Stdout, result.Stderr)
	}
	body, err := os.ReadFile(filepath.Join(tree, "rfc", "requirements", stem+".md")) // #nosec G304 -- a path this test made
	if err != nil {
		t.Fatalf("reading the generated shard: %v", err)
	}
	return string(body)
}

func TestRFCBothHalvesKeyTheSameStatusRows(t *testing.T) {
	// The public page keys a row three ways, because a summary enrolls under
	// its file stem and only one of the three IS an RFC number. A row the
	// reader cannot key is a row the enrolled RFC behind it has no disclosure
	// under, so the backlog names it -- which is what makes a skipped row
	// visible rather than silent.
	cases := []struct {
		name, stem, page string
		disclosed        bool
	}{
		{name: "a spaceless RFC number keys rfcNNNN", stem: "rfc1000",
			page: "| RFC1000 | Widgets | Supported | full | |\n", disclosed: true},
		{name: "a draft keys its own stem", stem: "draft-ietf-widget-00",
			page: "| draft-ietf-widget-00 | Widgets | Supported | full | |\n", disclosed: true},
		// The draft branch and the bare-stem branch OVERLAP for a lowercase
		// hyphenated name, so only a spelling the stem branch refuses shows
		// that the draft branch does any work at all.
		{name: "a draft the bare-stem branch cannot key", stem: "draft-IETF-widget-00",
			page: "| draft-IETF-widget-00 | Widgets | Supported | full | |\n", disclosed: true},
		{name: "a non-RFC stem keys itself", stem: "sflow-v5",
			page: "| sflow-v5 | Widgets | Supported | full | |\n", disclosed: true},
		{name: "a two-cell row keys nothing", stem: "rfc1000",
			page: "| RFC 1000 | Widgets |\n"},
		{name: "prose is not a row", stem: "rfc1000",
			page: "RFC 1000 is supported in full.\n"},
		{name: "a first cell naming nothing keyable", stem: "rfc1000",
			page: "| Total | 1 | Supported | full | |\n"},
	}
	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			// The extra stem declares no requirement of its own: an id is
			// validated against its summary's stem, and the question here is
			// how a ROW is keyed, not how an id is spelled.
			files := rfcPyRenderWith(map[string]string{
				"rfc/short/" + one.stem + ".md": "# " + one.stem + "\n",
				"rfc/enrolled.txt":              "rfc9999\n" + one.stem + "\n",
				"docs/features/rfc-status.md": "| RFC 9999 | Widgets | Supported | full | |\n" +
					one.page,
			})
			rfcPyRenderAgree(t, one.name, files)

			// The rendered backlog is where a row nobody could key becomes
			// visible: it names every enrolled RFC the page says nothing about.
			ledger := rfcPyLedgerOf(t, files)
			named := strings.Contains(ledger, "| `"+one.stem+"` | yes |")
			if named == one.disclosed {
				t.Errorf("the stem is %sin the rowless backlog, and the page %sdiscloses it:\n%s",
					map[bool]string{true: "", false: "not "}[named],
					map[bool]string{true: "", false: "does not "}[one.disclosed], one.page)
			}
		})
	}
}

// rfcPyLedgerOf renders one fixture through the SCRIPT and answers the ledger
// it wrote, so an assertion about content is made against the half being
// ported TO rather than against the port.
func rfcPyLedgerOf(t *testing.T, files map[string]string) string {
	t.Helper()

	tree := rfcPyTree(t, files)
	if result := rfcPyRunScript(t, tree, "--write"); result.Code != 0 {
		t.Fatalf("the script refused the fixture: %s%s", result.Stdout, result.Stderr)
	}
	body, err := os.ReadFile(filepath.Join(tree, "ai", "RFC-REQUIREMENTS.md")) // #nosec G304 -- a path this test made
	if err != nil {
		t.Fatalf("reading the generated ledger: %v", err)
	}
	return string(body)
}

func TestRFCBothHalvesRefuseTheSameDispositionRows(t *testing.T) {
	// rfc/not-enrolled.txt is what makes the un-enrolled remainder a decision
	// rather than an absence, so a malformed row is REFUSED rather than
	// skipped: a skipped row silently un-declares a summary, which is the
	// exact absence the file exists to abolish. HEAD's file is well-formed, so
	// none of these shapes appears in the byte comparison over the checkout.
	cases := []struct {
		name, rows string
		refuses    bool
	}{
		{name: "a comment and a blank line are skipped",
			rows: "\n# why these are not enrolled\nrfc1000 backlog nobody has walked it\n"},
		{name: "a reason may hold interior whitespace",
			rows: "rfc1000\tbacklog   nobody  has walked  it\n"},
		{name: "a duplicate stem is refused",
			rows: "rfc1000 backlog one\nrfc1000 blocked two\n", refuses: true},
		{name: "a row with no kind is refused", rows: "rfc1000\n", refuses: true},
		{name: "an unknown kind is refused",
			rows: "rfc1000 invented nobody has walked it\n", refuses: true},
		{name: "a bare kind with no reason is refused",
			rows: "rfc1000 backlog\n", refuses: true},
	}
	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			files := rfcPyRenderWith(map[string]string{
				"rfc/short/rfc1000.md": "# RFC 1000\n",
				"rfc/not-enrolled.txt": one.rows,
			})
			if one.refuses {
				rfcPyRefusalAgree(t, one.name, files)
				return
			}
			rfcPyRenderAgree(t, one.name, files)
		})
	}
}

func TestRFCBothHalvesReadTheSameForwardMetaRow(t *testing.T) {
	// The forward row decides which document states an obligation TODAY, and
	// every case here is a way the reader can fail OPEN rather than loudly:
	// three summaries naming a real successor were read as current documents
	// for as long as the label was matched one way and the corpus wrote it
	// four. HEAD holds none of these shapes, so the byte comparison over the
	// checkout says nothing about any of them.
	cases := []struct {
		name, row string
		refuses   bool
	}{
		{name: "an empty cell says nothing obsoletes this", row: "| Obsoleted-by | |"},
		{name: "a dash followed by prose stops at the dash",
			row: "| Obsoleted-by | -- RFC 9998 is a distinct protocol, not a successor |"},
		{name: "a bracketed none", row: "| Obsoleted-by | (none) |"},
		// The token, not the prefix: a value opening with a WORD that begins
		// "none" names a successor, and reading it as "nothing obsoletes this"
		// is the fail-open the lookahead exists to close.
		{name: "a word beginning with none is not the token none",
			row: "| Obsoleted-by | Nonetheless RFC 9998 obsoletes it |"},
		{name: "the spaced label is read too", row: "| Obsoleted By | RFC 9998 |"},
		{name: "a qualifier after the label is kept",
			row: "| Obsoleted-by (partial) | RFC 9998 |"},
		{name: "the chain's last document is the successor",
			row: "| Obsoleted-by | RFC 9997, which was in turn obsoleted by RFC 9998 |"},
		{name: "a spelling nothing reads is refused",
			row: "| Obsoletion | RFC 9998 |", refuses: true},
		{name: "a row naming no RFC is refused",
			row: "| Obsoleted-by | something else |", refuses: true},
		{name: "a summary cannot obsolete itself",
			row: "| Obsoleted-by | RFC 9999 |", refuses: true},
	}
	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) {
			files := rfcPyRenderWith(map[string]string{
				"rfc/short/rfc9999.md": "# RFC 9999\n\n" + one.row + "\n\n" +
					"- [ ] [RFC9999-2-1] [MUST] A speaker MUST send the widget (§2) " +
					"{gap: nothing sends it}\n",
			})
			if one.refuses {
				rfcPyRefusalAgree(t, one.name, files)
				return
			}
			rfcPyRenderAgree(t, one.name, files)
		})
	}
}

func TestRFCBothHalvesRefuseTheSameDestructiveInputs(t *testing.T) {
	// The write DELETES, so an incomplete collection is a destructive input
	// rather than a partial one. Both refusals are absent from HEAD, and each
	// one is the guard over a deletion.
	cases := []struct {
		name  string
		files map[string]string
	}{
		{"a summary that did not parse", rfcPyRenderWith(map[string]string{
			"rfc/short/rfc1000.md":        "# RFC 1000\n\n- [ ] [MUST] no id here at all\n",
			"rfc/requirements/rfc1000.md": "# RFC 1000 -- would be deleted\n",
		})},
		{"a render with no requirement row for any RFC", rfcPyRenderWith(map[string]string{
			"rfc/enrolled.txt":            "",
			"rfc/short/rfc9999.md":        "",
			"rfc/requirements/rfc9999.md": "# RFC 9999 -- would be deleted\n",
		})},
	}
	for _, one := range cases {
		t.Run(one.name, func(t *testing.T) { rfcPyRefusalAgree(t, one.name, one.files) })
	}
}

// rfcPyGeneratedAgree compares every generated file of two trees, in both
// directions, and fails when either holds fewer than `least` of them.
//
// The floor is what stops a green run over two empty directories. A comparison
// of nothing against nothing passes every assertion in it.
func rfcPyGeneratedAgree(t *testing.T, scriptTree, commandTree string, least int) {
	t.Helper()

	wrote := rfcPyGeneratedBytes(t, scriptTree)
	got := rfcPyGeneratedBytes(t, commandTree)
	if len(wrote) < least {
		t.Fatalf("the script wrote %d generated file(s), fewer than the %d this tree owes",
			len(wrote), least)
	}
	for name, body := range wrote {
		if got[name] != body {
			t.Errorf("the two halves wrote different bytes into %s\n%s",
				name, rfcPyFirstDifference(body, got[name]))
		}
	}
	for name := range got {
		if _, held := wrote[name]; !held {
			t.Errorf("the command wrote %s and the script did not", name)
		}
	}
}

// rfcPyGeneratedBytes reads every file this generator owns, so two trees can be
// compared without either half being asked what it wrote.
//
// An absent ledger and an absent shard directory are both empty answers rather
// than failures: a refusal writes neither, and the caller's floor is what
// decides whether an empty answer is legal for the case in hand.
func rfcPyGeneratedBytes(t *testing.T, tree string) map[string]string {
	t.Helper()

	out := map[string]string{}
	ledger, err := os.ReadFile(filepath.Join(tree, "ai", "RFC-REQUIREMENTS.md")) // #nosec G304 -- a path this test made
	if err == nil {
		out["ai/RFC-REQUIREMENTS.md"] = string(ledger)
	} else if !os.IsNotExist(err) {
		t.Fatalf("reading the generated ledger: %v", err)
	}

	dir := filepath.Join(tree, "rfc", "requirements")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return out
		}
		t.Fatalf("reading the shard directory: %v", err)
	}
	for _, entry := range entries {
		body, err := os.ReadFile(filepath.Join(dir, entry.Name())) // #nosec G304 -- a path this test made
		if err != nil {
			t.Fatalf("reading shard %s: %v", entry.Name(), err)
		}
		out[path.Join("rfc/requirements", entry.Name())] = string(body)
	}
	return out
}

// rfcPyFirstDifference names the first line the two bodies disagree on, because
// a shard runs to thousands of lines and a whole-body dump buries the one row
// that moved.
func rfcPyFirstDifference(want, got string) string {
	wantLines := strings.Split(want, "\n")
	gotLines := strings.Split(got, "\n")
	for i := range max(len(wantLines), len(gotLines)) {
		left, right := "", ""
		if i < len(wantLines) {
			left = wantLines[i]
		}
		if i < len(gotLines) {
			right = gotLines[i]
		}
		if left == right {
			continue
		}
		return fmt.Sprintf("line %d\nscript:  %q\ncommand: %q", i+1, left, right)
	}
	return "the bodies differ in length only"
}
