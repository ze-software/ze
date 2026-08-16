#!/usr/bin/env python3
"""Unit tests for code_to_docs.py (ai/CODE-TO-DOCS.md generator)."""

from __future__ import annotations

import os
import shutil
import subprocess
import sys
import tempfile
import textwrap
import unittest
from pathlib import Path

sys.path.insert(0, os.path.dirname(__file__))
from code_to_docs import (
    anchor_symbol_tokens,
    check_anchor_symbols,
    extract_anchor_segments,
    extract_paths,
    filter_gitignored,
    go_declarations,
)


class TestExtractPaths(unittest.TestCase):
    def test_semicolon_separated(self):
        self.assertEqual(
            extract_paths("internal/a.go -- one; cmd/b.go -- two"),
            ["internal/a.go", "cmd/b.go"],
        )

    def test_comma_relative_paths(self):
        # A bare filename after a full path inherits that path's directory.
        self.assertEqual(
            extract_paths("internal/component/x/a.go, b.go"),
            ["internal/component/x/a.go", "internal/component/x/b.go"],
        )


class TestFilterGitignored(unittest.TestCase):
    def _git(self, root: Path, *args: str) -> None:
        subprocess.run(
            ["git", "-C", str(root), *args],
            check=True,
            capture_output=True,
            text=True,
        )

    def test_skips_gitignored_docs(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            self._git(root, "init")
            (root / ".gitignore").write_text("docs/research/comparison/\n")
            (root / "docs").mkdir()
            (root / "docs" / "keep.md").write_text("# keep\n")
            research = root / "docs" / "research" / "comparison" / "freertr"
            research.mkdir(parents=True)
            (research / "23-concurrent-editing.md").write_text("# ignored\n")

            paths = sorted((root / "docs").rglob("*.md"))
            kept = [str(p.relative_to(root)) for p in filter_gitignored(root, paths)]

            self.assertIn("docs/keep.md", kept)
            self.assertNotIn(
                "docs/research/comparison/freertr/23-concurrent-editing.md", kept
            )

    def test_no_git_repo_falls_back(self):
        # Outside a git repository, check-ignore errors (128); index everything.
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "docs").mkdir()
            (root / "docs" / "a.md").write_text("# a\n")
            paths = sorted((root / "docs").rglob("*.md"))
            self.assertEqual(filter_gitignored(root, paths), paths)

    def test_empty(self):
        with tempfile.TemporaryDirectory() as tmp:
            self.assertEqual(filter_gitignored(Path(tmp), []), [])


class TestExtractAnchorSegments(unittest.TestCase):
    def test_description_is_kept_beside_its_paths(self):
        self.assertEqual(
            extract_anchor_segments("internal/a.go -- Run, Stop; cmd/b.go -- Main"),
            [(["internal/a.go"], "Run, Stop"), (["cmd/b.go"], "Main")],
        )

    def test_one_description_covers_every_path_of_its_segment(self):
        self.assertEqual(
            extract_anchor_segments("internal/component/x/a.go, b.go -- Run"),
            [(["internal/component/x/a.go", "internal/component/x/b.go"], "Run")],
        )

    def test_segment_without_a_description(self):
        self.assertEqual(
            extract_anchor_segments("internal/a.go"), [(["internal/a.go"], "")]
        )


class TestAnchorSymbolTokens(unittest.TestCase):
    """The classifier: which description tokens are declaration claims."""

    def test_bare_identifier_is_a_claim(self):
        self.assertEqual(anchor_symbol_tokens("Run"), ["Run"])

    def test_call_parentheses_are_stripped(self):
        self.assertEqual(anchor_symbol_tokens("Run()"), ["Run"])

    def test_dotted_form_is_a_claim(self):
        self.assertEqual(anchor_symbol_tokens("MuxConn.readLoop"), ["MuxConn.readLoop"])

    def test_commas_split_the_description(self):
        self.assertEqual(
            anchor_symbol_tokens("Check, Repair, CheckReport"),
            ["Check", "Repair", "CheckReport"],
        )

    def test_free_text_is_not_a_claim(self):
        for description in (
            "Foo package",
            "non-Linux stub",
            "receive path",
            "WireUpdate struct",
            "cli domain Run()",
            "ze-perf",
            "StateIdle..StateEstablished",
            "",
        ):
            self.assertEqual(
                anchor_symbol_tokens(description), [], f"description: {description!r}"
            )

    def test_mixed_description_keeps_only_the_claims(self):
        self.assertEqual(
            anchor_symbol_tokens("event structs, events.Register"), ["events.Register"]
        )

    def test_single_lowercase_word_is_prose_not_a_claim(self):
        """Severity rule 1: 105 of 372 unresolved claims were this shape."""
        self.assertEqual(anchor_symbol_tokens("link, address, and route"), [])

    def test_rule_1_costs_the_all_lowercase_go_declaration(self):
        """DELIBERATE LIMITATION of severity rule 1, priced by phase 2.

        `run` and `main` are real Go declarations, and an anchor can no longer
        claim either: nothing distinguishes them from the English words. The
        105 prose nouns rule 1 removes are worth more than the claims on
        lowercase declarations it gives up. Change this test only with a
        measurement that says the trade has flipped.
        """
        self.assertEqual(anchor_symbol_tokens("run, main"), [])

    def test_a_lowercase_token_with_a_separator_survives_rule_1(self):
        """Rule 1 stays narrow: a separator means identifier, not English.

        `sa_count` (a metric) and `ze.storage.blob` (a config key) carry no
        capital, and phase 2 measured both as claims a doc really makes. A
        rule keyed on capitals alone would have dropped 4 true findings.
        """
        self.assertEqual(
            anchor_symbol_tokens("sa_count, ze.storage.blob"),
            ["sa_count", "ze.storage.blob"],
        )

    def test_mixed_case_token_is_still_a_claim(self):
        """Rule 1's negative control: one capital is enough to be a claim."""
        self.assertEqual(anchor_symbol_tokens("readLoop"), ["readLoop"])


class TestCheckAnchorSymbols(unittest.TestCase):
    """AC-1..AC-3: an anchor's symbol list is checked against the anchored file."""

    def _root(self, tmp, go_src, go_name="bar.go"):
        root = Path(tmp)
        (root / "internal" / "foo").mkdir(parents=True)
        if go_src is not None:
            (root / "internal" / "foo" / go_name).write_text(go_src)
        return root

    def _anchor(self, description, paths=("internal/foo/bar.go",)):
        return [("docs/test.md", 7, list(paths), description)]

    def test_anchor_naming_a_declared_symbol_passes(self):
        """AC-2: a symbol declared in the anchored file produces no finding."""
        with tempfile.TemporaryDirectory() as tmp:
            root = self._root(tmp, "package foo\n\nfunc Run() {}\n")
            self.assertEqual(check_anchor_symbols(root, self._anchor("Run")), [])

    def test_absent_symbol_is_reported(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._root(tmp, "package foo\n\nfunc Run() {}\n")
            problems = check_anchor_symbols(root, self._anchor("Vanished"))
            self.assertEqual(len(problems), 1)
            self.assertIn("docs/test.md:7", problems[0])
            self.assertIn("internal/foo/bar.go", problems[0])
            self.assertIn("Vanished", problems[0])

    def test_each_comma_token_is_checked_separately(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._root(tmp, "package foo\n\nfunc Run() {}\n")
            problems = check_anchor_symbols(root, self._anchor("Run, Vanished"))
            self.assertEqual(len(problems), 1)
            self.assertIn("Vanished", problems[0])

    def test_free_text_description_is_not_flagged(self):
        """AC-3: a description that describes rather than names is never a claim."""
        with tempfile.TemporaryDirectory() as tmp:
            root = self._root(tmp, "package foo\n\nfunc Run() {}\n")
            self.assertEqual(
                check_anchor_symbols(root, self._anchor("Foo package")), []
            )

    def test_declarations_of_every_kind_resolve(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._root(
                tmp,
                textwrap.dedent("""\
                    package foo

                    type MuxConn struct {
                    \tSeq int
                    }

                    type Handler interface {
                    \tHandle(x int) error
                    }

                    const (
                    \tStateIdle = iota
                    \tStateOpen
                    )

                    var Registry = map[string]int{}

                    func (m *MuxConn) readLoop() {}

                    func Run() {}
                    """),
            )
            anchors = self._anchor(
                "MuxConn, Handler, Registry, StateIdle, StateOpen, "
                "MuxConn.readLoop, Run(), Seq, Handler.Handle"
            )
            self.assertEqual(check_anchor_symbols(root, anchors), [])

    def test_blank_line_does_not_end_a_declaration_body(self):
        """A blank line separates members; it does not close the declaration."""
        with tempfile.TemporaryDirectory() as tmp:
            root = self._root(
                tmp,
                textwrap.dedent("""\
                    package foo

                    type vppOps interface {
                    \tdumpInterfaces() error

                    \t// A comment block, then more methods.
                    \tclassifyAddDelTable(idx uint32) error
                    }

                    const (
                    \tFirst = 1

                    \tSecond = 2
                    )
                    """),
            )
            anchors = self._anchor(
                "dumpInterfaces, classifyAddDelTable, vppOps.classifyAddDelTable, "
                "First, Second"
            )
            self.assertEqual(check_anchor_symbols(root, anchors), [])

    def test_function_body_locals_are_not_declarations(self):
        """The scan reads declarations, never a function body.

        Asserted on go_declarations, the function that produces the answer.
        The check no longer REPORTS a body local, because rule 2 demotes it
        (see test_rule_2_costs_the_claim_a_file_only_mentions), so the
        property has to be pinned where it still holds.
        """
        names, dotted = go_declarations(
            "package foo\n\nfunc Run() {\n\tlocalOnly := 1\n\t_ = localOnly\n}\n"
        )
        self.assertIn("Run", names)
        self.assertNotIn("localOnly", names)
        self.assertEqual(dotted, set())

    def test_rule_2_costs_the_claim_a_file_only_mentions(self):
        """DELIBERATE LIMITATION of severity rule 2, priced by phase 2.

        A rename that leaves the old name behind in a comment of the same file
        now PASSES: the file no longer declares `oldName`, but its text still
        holds it, so the check cannot tell a stale doc claim from an accurate
        one about a call or a comment. This is the price of demoting 230 of
        372 findings that were true-but-unhelpful. Change this test only with
        a measurement that says the tree can carry the stricter rule.
        """
        with tempfile.TemporaryDirectory() as tmp:
            root = self._root(
                tmp,
                textwrap.dedent("""\
                    package foo

                    // NewName was called oldName until the rename.
                    func NewName() {}
                    """),
            )
            self.assertEqual(check_anchor_symbols(root, self._anchor("oldName")), [])

    def test_rule_2_demotes_a_call_the_anchored_file_makes(self):
        """AC-3: a call is not a declaration, and the reader still finds it.

        Phase 2 measured 26 of these. Until rule 2 they were reported.
        """
        with tempfile.TemporaryDirectory() as tmp:
            root = self._root(tmp, "package foo\n\nfunc Run() { events.Register() }\n")
            self.assertEqual(
                check_anchor_symbols(root, self._anchor("events.Register")),
                [],
            )

    def test_rule_2_demotes_a_member_reached_through_a_receiver(self):
        """AC-3: 24 of the 372 were this shape. `\\b` sees past the dot."""
        with tempfile.TemporaryDirectory() as tmp:
            root = self._root(tmp, "package foo\n\nfunc Run(p *Peer) { p.Start() }\n")
            self.assertEqual(check_anchor_symbols(root, self._anchor("Start")), [])

    def test_rule_2_demotes_a_string_key(self):
        """AC-3: 63 of the 372 were an env, config, CLI or metric key."""
        with tempfile.TemporaryDirectory() as tmp:
            root = self._root(
                tmp, 'package foo\n\nvar dir = os.Getenv("ZE_STORAGE_DIR")\n'
            )
            self.assertEqual(
                check_anchor_symbols(root, self._anchor("ZE_STORAGE_DIR")),
                [],
            )

    def test_uppercase_claim_absent_from_the_file_text_still_fails(self):
        """Rule 2's negative control: absent from the text is still a finding.

        The dotted shape phase 2 kept (`Peer.Run`, `Exporter.Status`): the file
        declares neither the member nor the dotted name, and never writes it.
        """
        with tempfile.TemporaryDirectory() as tmp:
            root = self._root(tmp, "package foo\n\nfunc Run() { events.Register() }\n")
            problems = check_anchor_symbols(root, self._anchor("Exporter.Status"))
            self.assertEqual(len(problems), 1)
            self.assertIn("Exporter.Status", problems[0])

    def test_rule_2_matches_a_whole_word_not_a_substring(self):
        """A longer name that CONTAINS the claim does not satisfy rule 2.

        Without the word boundary, `Run` would pass on any file mentioning
        `Runner`, and the check would resolve on coincidence.
        """
        with tempfile.TemporaryDirectory() as tmp:
            root = self._root(tmp, "package foo\n\ntype Runner struct{}\n")
            problems = check_anchor_symbols(root, self._anchor("Run"))
            self.assertEqual(len(problems), 1)
            self.assertIn("'Run'", problems[0])

    def test_claim_resolves_against_any_path_of_the_segment(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._root(tmp, "package foo\n\nfunc Run() {}\n")
            (root / "internal" / "foo" / "other.go").write_text(
                "package foo\n\nfunc Stop() {}\n"
            )
            anchors = self._anchor(
                "Run, Stop", ("internal/foo/bar.go", "internal/foo/other.go")
            )
            self.assertEqual(check_anchor_symbols(root, anchors), [])

    def test_unreadable_go_file_is_a_finding(self):
        """Security review: fail closed, never a silent skip."""
        with tempfile.TemporaryDirectory() as tmp:
            root = self._root(tmp, None)
            (root / "internal" / "foo" / "bar.go").write_bytes(
                b"package foo\n\xff\xfe\n"
            )
            problems = check_anchor_symbols(root, self._anchor("Run"))
            self.assertEqual(len(problems), 1)
            self.assertIn("internal/foo/bar.go", problems[0])
            self.assertIn("cannot", problems[0])

    def test_unreadable_file_does_not_turn_an_unknown_claim_into_an_absent_one(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._root(tmp, "package foo\n\nfunc Run() {}\n")
            (root / "internal" / "foo" / "broken.go").write_bytes(
                b"package foo\n\xff\xfe\n"
            )
            anchors = self._anchor(
                "Run, Unknowable", ("internal/foo/bar.go", "internal/foo/broken.go")
            )
            problems = check_anchor_symbols(root, anchors)
            self.assertEqual(len(problems), 1)
            self.assertIn("cannot", problems[0])

    def test_non_go_anchor_is_skipped(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            (root / "scripts").mkdir()
            (root / "scripts" / "tool.py").write_text("def helper():\n    pass\n")
            anchors = self._anchor("Vanished", ("scripts/tool.py",))
            self.assertEqual(check_anchor_symbols(root, anchors), [])

    def test_missing_file_is_left_to_the_stale_reference_check(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            anchors = self._anchor("Vanished", ("internal/gone/bar.go",))
            self.assertEqual(check_anchor_symbols(root, anchors), [])

    def test_line_number_suffix_is_stripped_before_reading(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._root(tmp, "package foo\n\nfunc Run() {}\n")
            anchors = self._anchor("Run", ("internal/foo/bar.go:47",))
            self.assertEqual(check_anchor_symbols(root, anchors), [])


class TestArmedGate(unittest.TestCase):
    """AC-1 end to end: `--check` itself fails, not only check_anchor_symbols.

    Every test above calls the function directly, so none of them sees the
    wiring in main(). These run the script over a tree of their own: the
    script takes its root from its own location (`parents[2]`), so a copy at
    <tmp>/scripts/dev/ indexes <tmp>.
    """

    def _tree(self, tmp: str, description: str) -> Path:
        root = Path(tmp)
        (root / "scripts" / "dev").mkdir(parents=True)
        shutil.copy(
            Path(__file__).with_name("code_to_docs.py"),
            root / "scripts" / "dev" / "code_to_docs.py",
        )
        (root / "ai").mkdir()
        (root / "internal" / "foo").mkdir(parents=True)
        (root / "internal" / "foo" / "bar.go").write_text(
            "package foo\n\nfunc Run() {}\n"
        )
        (root / "docs").mkdir()
        (root / "docs" / "page.md").write_text(
            f"# page\n\n<!-- source: internal/foo/bar.go -- {description} -->\n"
        )
        return root

    def _run(self, root: Path, *args: str) -> subprocess.CompletedProcess:
        return subprocess.run(
            [sys.executable, str(root / "scripts" / "dev" / "code_to_docs.py"), *args],
            capture_output=True,
            text=True,
        )

    def test_check_fails_on_an_anchor_naming_an_absent_symbol(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = self._tree(tmp, "Vanished")
            self.assertEqual(self._run(root).returncode, 0)  # generate the index
            proc = self._run(root, "--check")
            self.assertEqual(proc.returncode, 1)
            self.assertIn("CLAIM:", proc.stdout)
            self.assertIn("Vanished", proc.stdout)
            self.assertIn("docs/page.md:3", proc.stdout)

    def test_check_passes_when_the_named_symbol_is_declared(self):
        """The negative control: the same tree, one word changed."""
        with tempfile.TemporaryDirectory() as tmp:
            root = self._tree(tmp, "Run")
            self.assertEqual(self._run(root).returncode, 0)
            proc = self._run(root, "--check")
            self.assertEqual(proc.returncode, 0, proc.stdout + proc.stderr)
            self.assertIn("all references valid", proc.stdout)

    def test_a_finding_never_enters_the_generated_content(self):
        """A claim finding must not make the index it generated look stale.

        The findings go to stdout. If they reached `content`, check mode would
        build a different file from generate mode and report "stale -- run:
        make ze-doc-index-update" forever, however many times you ran it.
        """
        with tempfile.TemporaryDirectory() as tmp:
            root = self._tree(tmp, "Vanished")
            self.assertEqual(self._run(root).returncode, 0)
            index = (root / "ai" / "CODE-TO-DOCS.md").read_text()
            self.assertNotIn("Vanished", index)
            proc = self._run(root, "--check")
            self.assertNotIn("stale", proc.stderr)
            self.assertIn("CLAIM:", proc.stdout)
            # Generate mode run again on the same tree writes the same bytes,
            # so the two modes agree on `content` with a finding outstanding.
            self.assertEqual(self._run(root).returncode, 0)
            self.assertEqual((root / "ai" / "CODE-TO-DOCS.md").read_text(), index)


if __name__ == "__main__":
    unittest.main()
