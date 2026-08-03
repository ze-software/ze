#!/usr/bin/env python3
"""Unit tests for check_doc_links.py, the corpus path-reference gate.

Driven end-to-end through the real entry point (subprocess) per the guard test
corollary in ai/rules/evidence.md: the exit code IS the gate. Each
fixture builds a throwaway git repo so a test never depends on the real tree.

VALIDATES: a reference to a gitignored (generated) path is not reported missing
           when the generator has not run, while a reference to any other
           missing path still fails the gate.
PREVENTS:  the CI-only red where a fresh checkout has no CLAUDE.md /
           AGENTS.md / .claude/skills (all gitignored, all produced by
           `make ze-ai-sync`) and every rule file citing them is called broken.
"""

from __future__ import annotations

import ast
import os
import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

HERE = Path(__file__).resolve().parent
SCRIPT = HERE / "check_doc_links.py"

sys.path.insert(0, str(HERE))
import check_doc_links as cdl

REPO = Path(
    subprocess.run(
        ["git", "rev-parse", "--show-toplevel"],
        cwd=str(HERE),
        capture_output=True,
        text=True,
        check=True,
    ).stdout.strip()
)

IGNORED_PATHS = "CLAUDE.md\nAGENTS.md\n.claude/skills/\n"


def run(repo: Path) -> subprocess.CompletedProcess:
    return subprocess.run(
        [sys.executable, str(SCRIPT), "--md-only"],
        cwd=str(repo),
        capture_output=True,
        text=True,
    )


class DocLinksGateTest(unittest.TestCase):
    def _repo(self, corpus: str, gitignore: str = IGNORED_PATHS) -> Path:
        d = tempfile.mkdtemp(prefix="doc-links-")
        self.addCleanup(lambda: shutil.rmtree(d, ignore_errors=True))
        repo = Path(d)
        subprocess.run(
            ["git", "init", "-q"], cwd=str(repo), check=True, capture_output=True
        )
        (repo / ".gitignore").write_text(gitignore, encoding="utf-8")
        rules = repo / "ai" / "rules"
        rules.mkdir(parents=True)
        (rules / "sample.md").write_text(corpus, encoding="utf-8")
        return repo

    def test_generated_target_absent_is_not_broken(self) -> None:
        """A rule citing the generated CLAUDE.md passes on a fresh checkout."""
        repo = self._repo("Never edit `CLAUDE.md`; edit `ai/rules/sample.md`.\n")
        self.assertFalse((repo / "CLAUDE.md").exists())
        res = run(repo)
        self.assertEqual(res.returncode, 0, res.stdout + res.stderr)
        self.assertNotIn("CLAUDE.md", res.stdout)

    def test_generated_directory_absent_is_not_broken(self) -> None:
        """The ignored-directory form (`.claude/skills/`) is covered too."""
        repo = self._repo("Skills are synced into `.claude/skills/`.\n")
        res = run(repo)
        self.assertEqual(res.returncode, 0, res.stdout + res.stderr)

    def test_untracked_missing_path_still_fails(self) -> None:
        """The negative control: a plain missing path is still a hard error."""
        repo = self._repo("See `ai/rules/absent.md` for details.\n")
        res = run(repo)
        self.assertEqual(res.returncode, 1, res.stdout + res.stderr)
        self.assertIn("ai/rules/absent.md", res.stdout)

    def test_generated_and_missing_together(self) -> None:
        """One line, both kinds: only the non-ignored path is reported."""
        repo = self._repo("`CLAUDE.md` is generated, `ai/rules/absent.md` is not.\n")
        res = run(repo)
        self.assertEqual(res.returncode, 1, res.stdout + res.stderr)
        self.assertIn("ai/rules/absent.md", res.stdout)
        self.assertNotIn("broken path reference: CLAUDE.md", res.stdout)

    def test_present_path_resolves(self) -> None:
        """A reference to a file that exists stays green."""
        repo = self._repo("See `ai/rules/sample.md`.\n")
        res = run(repo)
        self.assertEqual(res.returncode, 0, res.stdout + res.stderr)


class InRepo:
    """Run a block with the process cwd at the repository root.

    Both new checks read the tree (git ls-files, .claude/hooks/), so they are
    only meaningful from the root. unittest may be started from anywhere.
    """

    def __enter__(self):
        self.prev = os.getcwd()
        os.chdir(REPO)
        return self

    def __exit__(self, *exc):
        os.chdir(self.prev)
        return False


class PointerBudgetTest(unittest.TestCase):
    """The `ai/rules/writing.md` budget: 120 characters after the link.

    VALIDATES: AC-1 -- an over-long curated-index entry is named with its
               length and its file:line.
    PREVENTS:  the curated index drifting back into 117 KB of description that
               restates the summaries it links.
    """

    def _index(self, body: str) -> str:
        d = tempfile.mkdtemp(prefix="pointer-budget-")
        self.addCleanup(lambda: shutil.rmtree(d, ignore_errors=True))
        p = Path(d) / "LEARNED-INDEX.md"
        p.write_text(body, encoding="utf-8")
        return str(p)

    def test_index_entry_over_budget_fails(self) -> None:
        path = self._index(
            "- [760](plan/learned/760-a.md) -- " + "x" * 121 + "\n",
        )
        found = cdl.check_index_budget(path)
        self.assertEqual(len(found), 1, found)
        self.assertIn("121 characters after the link", found[0])
        self.assertIn(f"{path}:1", found[0])

    def test_index_entry_within_budget_passes(self) -> None:
        """The boundary: 120 is the last valid length, 121 is the first bad."""
        at_limit = self._index("- [760](plan/learned/760-a.md) -- " + "x" * 120 + "\n")
        self.assertEqual(cdl.check_index_budget(at_limit), [])

        over = self._index("- [760](plan/learned/760-a.md) -- " + "x" * 121 + "\n")
        self.assertEqual(len(cdl.check_index_budget(over)), 1)

        # A heading, a prose line and a table row are not index entries.
        other = self._index(
            "## Core Architecture\n\nProse that runs well past the budget "
            + "y" * 200
            + "\n\n| Question | File |\n|---|---|\n| a | `b` "
            + "z" * 200
            + " |\n"
        )
        self.assertEqual(cdl.check_index_budget(other), [])

    def test_absent_index_is_not_an_error(self) -> None:
        """A tree without the curated index is not the Ze corpus."""
        self.assertEqual(cdl.check_index_budget("no/such/index.md"), [])


class DeadNameLintTest(unittest.TestCase):
    """Hook and check names cited in the hook-describing documents.

    VALIDATES: AC-3, AC-4, AC-5 -- a `*.sh` filename or `c_*`/`check_*`
               function that names nothing in the tree is reported with its
               file, line and token.
    PREVENTS:  the 16 consolidated shell hooks and the never-existing
               `c_rfc_tagged_test` reading as live checks for another year.
    """

    def _doc(self, body: str) -> tuple[str, ...]:
        d = tempfile.mkdtemp(prefix="dead-name-")
        self.addCleanup(lambda: shutil.rmtree(d, ignore_errors=True))
        p = Path(d) / "HOOK-FRICTION.md"
        p.write_text(body, encoding="utf-8")
        return (str(p),)

    def test_dead_hook_name_fails(self) -> None:
        with InRepo():
            files = self._doc(
                "## `block-nonexistent-thing.sh`\n\nIt blocks things.\n"
                "See also `c_never_existed`.\n"
            )
            found = cdl.check_hook_names(files)
        self.assertEqual(len(found), 2, found)
        joined = "\n".join(found)
        self.assertIn("block-nonexistent-thing.sh", joined)
        self.assertIn("c_never_existed", joined)
        self.assertIn(":1:", found[0])

    def test_live_names_resolve(self) -> None:
        """The negative control: real names never report."""
        with InRepo():
            files = self._doc(
                "`c_test_weakening`, `check_pipe_tail`, `c_auto_lint`, "
                "`spec-session.sh`, `verify-status.sh`, `check_frozen_verbs`.\n"
            )
            self.assertEqual(cdl.check_hook_names(files), [])

    def test_retired_section_name_allowed(self) -> None:
        """A dead name under `## Retired` is history, not a broken reference."""
        with InRepo():
            excused = self._doc(
                "## Retired\n\n### `block-gone.sh` -- retired 2026-04-19\n\n"
                "It used to fire on everything.\n"
            )
            self.assertEqual(cdl.check_hook_names(excused), [])

            # The escape ENDS at the next same-level heading, so a live
            # section after it cannot inherit the exemption.
            leaks = self._doc(
                "## Retired\n\n### `block-gone.sh` -- retired\n\n"
                "## Active\n\n### `block-still-cited.sh`\n"
            )
            found = cdl.check_hook_names(leaks)
        self.assertEqual(len(found), 1, found)
        self.assertIn("block-still-cited.sh", found[0])

    def test_retired_table_cell_must_start_with_the_marker(self) -> None:
        """The row escape is a cell, not a substring anywhere in the row."""
        with InRepo():
            excused = self._doc(
                "| Hook | Status |\n|---|---|\n"
                "| `block-gone.sh` | Retired 2026-04-19 |\n"
            )
            self.assertEqual(cdl.check_hook_names(excused), [])

            # "retired" buried mid-prose in a cell does not excuse the row.
            leaks = self._doc(
                "| Hook | Status |\n|---|---|\n"
                "| `block-gone.sh` | Active, unlike the retired ones |\n"
            )
            found = cdl.check_hook_names(leaks)
        self.assertEqual(len(found), 1, found)
        self.assertIn("block-gone.sh", found[0])

    def test_retired_marker_only_counts_in_a_cell_entitled_to_say_it(self) -> None:
        """The marker must sit in the first cell or the Status column.

        VALIDATES: a row whose STATUS says Active still reports its dead name,
        even when a later prose cell opens with the word "Retired".
        PREVENTS: the `any(cell)` match returning. Anchoring the pattern to the
        start of a cell was not enough on its own -- "Retired elsewhere, but this
        is live" starts with the marker, and one such description excused every
        dead name in its row.
        """
        with InRepo():
            leaks = self._doc(
                "| Hook | Status | Notes |\n|---|---|---|\n"
                "| `block-gone.sh` | Active | Retired elsewhere, but this is "
                "live |\n"
            )
            found = cdl.check_hook_names(leaks)
            self.assertEqual(len(found), 1, found)
            self.assertIn("block-gone.sh", found[0])

            # Both shapes the corpus actually uses keep their excuse: the marker
            # in the header's Status column (plan/learned/HOOK-FRICTION.md's
            # frequency table) and in the row's first cell (its rename table).
            status_col = self._doc(
                "| Hook | Appearances | Status | Entry |\n|--|--|--|--|\n"
                "| `block-gone.sh` | 4 | Retired 2026-04-19 | [Retired](#r) |\n"
            )
            self.assertEqual(cdl.check_hook_names(status_col), [])
            first_cell = self._doc(
                "| Retired name | Live check | Dispatcher |\n|--|--|--|\n"
                "| Retired: `block-gone.sh` | `c_panic` | `x.py` |\n"
            )
            self.assertEqual(cdl.check_hook_names(first_cell), [])

            # A header applies to ITS table only. The same marker position in a
            # later table with no Status column must not stay excused.
            spill = self._doc(
                "| Hook | Status |\n|---|---|\n"
                "| `block-a.sh` | Retired 2026-04-19 |\n"
                "\n"
                "| Hook | Notes |\n|---|---|\n"
                "| `block-b.sh` | Retired somewhere else |\n"
            )
            found = cdl.check_hook_names(spill)
        self.assertEqual(len(found), 1, found)
        self.assertIn("block-b.sh", found[0])

    def test_check_resolved_via_def_not_registry(self) -> None:
        """The `c_rfc_tagged_test` trap: resolve against `def`, not `CHECKS`.

        `rfc-tagged-test` is a registry label. Its guard is
        `_rfc_tagged_change_err`, called from `c_test_weakening`, and it
        appears in no `CHECKS` tuple. A lint resolving against the tuples
        would call live checks dead and be switched off (spec risk R-1).
        """
        with InRepo():
            _, defs, errors = cdl.known_names()
            registry = registry_names(REPO / ".claude" / "hooks")
        self.assertEqual(errors, [])

        # The real guard is a def, and is absent from every CHECKS tuple.
        self.assertIn("_rfc_tagged_change_err", defs)
        self.assertNotIn("_rfc_tagged_change_err", registry)

        # A def that no tuple names still resolves.
        self.assertIn("check_wiring", defs)
        self.assertNotIn("check_wiring", registry)

        # The cited-but-never-existing name resolves nowhere.
        self.assertNotIn("c_rfc_tagged_test", defs)
        self.assertNotIn("c_rfc_tagged_test", registry)
        with InRepo():
            files = self._doc("The hook `c_rfc_tagged_test` blocks edits.\n")
            found = cdl.check_hook_names(files)
        self.assertEqual(len(found), 1, found)
        self.assertIn("c_rfc_tagged_test", found[0])

    def test_verify_wiring_docs_checks_resolved(self) -> None:
        """The lint reads a second tree: four checks live outside the hooks."""
        with InRepo():
            _, defs, errors = cdl.known_names()
        self.assertEqual(errors, [])
        for name in (
            "check_ci_sleep_ratchet",
            "check_ci_sleep_justification",
            "check_ci_log_subsystem_keys",
            "check_known_failure_load_excuses",
        ):
            self.assertIn(name, defs, f"{name} must resolve from the lint set")

    def test_sources_are_parsed_never_executed(self) -> None:
        """A doc check must not run hook code (spec Security Review)."""
        src = SCRIPT.read_text(encoding="utf-8")
        self.assertIn("ast.parse", src)
        for banned in ("importlib", "exec(", "__import__", "runpy"):
            self.assertNotIn(banned, src, f"{banned} would execute hook code")

    def test_missing_lint_source_fails_closed(self) -> None:
        """Losing a source would turn its live checks dead, so it reports."""
        with InRepo():
            original = cdl.NAME_LINT_SOURCES
            cdl.NAME_LINT_SOURCES = original + ("scripts/dev/no_such_check.py",)
            try:
                _, _, errors = cdl.known_names()
            finally:
                cdl.NAME_LINT_SOURCES = original
        self.assertEqual(len(errors), 1, errors)
        self.assertIn("no_such_check.py", errors[0])


class RealCorpusTest(unittest.TestCase):
    """AC-4: after the fixes, the lint exits 0 against the real tree."""

    def test_real_corpus_is_present_and_clean(self) -> None:
        for rel in cdl.NAME_LINT_FILES + (cdl.INDEX_FILE,):
            self.assertTrue(
                (REPO / rel).exists(),
                f"{rel} is missing; both checks would go quiet without it",
            )
        with InRepo():
            self.assertEqual(cdl.check_hook_names(), [])

    def test_design_history_is_scanned(self) -> None:
        """AC-6: the exemption is gone, so a stale path in it fails the gate.

        VALIDATES: `plan/learned/DESIGN-HISTORY.md` is in the scanned corpus and
                   every path it names resolves today.
        PREVENTS:  the three months of undetected rot the blanket exemption
                   bought, on the reasoning that a history may cite dead paths.
                   Agents read this file to FIND code, so its paths must resolve;
                   a genuinely retired one is marked `doc-links: ignore` in place.
        """
        self.assertIn(
            "plan/learned/DESIGN-HISTORY.md",
            cdl.MD_GLOBS,
            "DESIGN-HISTORY.md must be scanned, never exempt",
        )
        with InRepo():
            saved = cdl.MD_GLOBS
            cdl.MD_GLOBS = ["plan/learned/DESIGN-HISTORY.md"]
            try:
                broken = cdl.drop_generated(cdl.check_markdown(False))
            finally:
                cdl.MD_GLOBS = saved
        self.assertEqual(broken, [], broken)

    def test_index_budget_is_blocking_and_the_real_index_is_clean(self) -> None:
        """AC-2: the trim landed, so the budget gate blocks rather than reports.

        VALIDATES: every curated-index entry is under the pointer budget, and a
                   new over-budget entry fails `make ze-doc-test` instead of
                   printing a line nobody reads.
        PREVENTS:  the gate being left report-only after the corpus it was
                   scheduling came clean, which is how a ratchet stops holding.
        """
        self.assertTrue(
            cdl.INDEX_BUDGET_BLOCKING,
            "the curated index is under budget, so the check must block",
        )
        with InRepo():
            self.assertEqual(cdl.check_index_budget(), [])


def registry_names(hooks: Path) -> set[str]:
    """Function names listed in the dispatchers' `CHECKS` tuples.

    Test-only, and deliberately a SECOND implementation: the lint must not
    consult this set, and a test that shared its code could not prove that.
    """
    names: set[str] = set()
    for src in sorted(hooks.glob("*.py")):
        tree = ast.parse(src.read_text(encoding="utf-8"), filename=str(src))
        for node in tree.body:
            if not isinstance(node, ast.Assign):
                continue
            if not any(
                isinstance(t, ast.Name) and t.id == "CHECKS" for t in node.targets
            ):
                continue
            if isinstance(node.value, (ast.Tuple, ast.List)):
                for el in node.value.elts:
                    if isinstance(el, ast.Name):
                        names.add(el.id)
    return names


if __name__ == "__main__":
    unittest.main()
