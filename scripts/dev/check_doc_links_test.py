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


def run_design(repo: Path) -> subprocess.CompletedProcess:
    return subprocess.run(
        [sys.executable, str(SCRIPT), "--design-only"],
        cwd=str(repo),
        capture_output=True,
        text=True,
        check=False,
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

    def test_digest_line_number_run_is_stripped(self) -> None:
        """AC-8: `path,62,152,169` names the file, and the file is checked.

        `ai/digests/*.md` writes several line numbers as a comma run. The run
        is not part of the path, so a live file must pass. The second half is
        the negative control: the strip must not turn the check off, so a dead
        file written the same way is still reported, under its stripped name.
        """
        repo = self._repo(
            "Live `ai/rules/sample.md,62,152,169`, dead `ai/rules/absent.md,7`.\n"
            "Range `ai/rules/sample.md,875-886`, mixed "
            "`ai/rules/sample.md,12,40-44`.\n"
        )
        res = run(repo)
        self.assertEqual(res.returncode, 1, res.stdout + res.stderr)
        self.assertIn("broken path reference: ai/rules/absent.md", res.stdout)
        self.assertNotIn("reference: ai/rules/sample.md", res.stdout)


class InRepo:
    """Run a block with the process cwd at the repository root.

    Both new checks read the tree (git ls-files, .claude/hooks/), so they are
    only meaningful from the root. unittest may be started from anywhere.
    """

    def __enter__(self):
        self.prev = os.getcwd()
        os.chdir(REPO)
        return self

    def __exit__(self, *_exc):
        os.chdir(self.prev)
        return False


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

    def test_check_source_filename_resolves(self) -> None:
        """A check's OWN file name is a live reference, not a dead one.

        VALIDATES: `check_doc_links.py` resolves. `SH_TOKEN` never sees it (it
                   is not a `.sh`), and `CHECK_TOKEN` matches the stem inside
                   it, so the stem must be in the set `CHECK_TOKEN` resolves
                   against.
        PREVENTS:  the gate row in
                   `ai/rules/points/repo-maintenance/discovery-updates/`
                   having to omit the file that holds `check_ignore_reasons`
                   to keep the lint green.
        """
        with InRepo():
            _, checks, errors = cdl.known_names()
            self.assertEqual(errors, [])
            self.assertIn("check_doc_links", checks)
            files = self._doc(
                "`check_ignore_reasons` in `check_doc_links.py` gates it.\n"
            )
            self.assertEqual(cdl.check_hook_names(files), [])

    def test_unknown_check_name_still_dead(self) -> None:
        """The negative control: widening the set resolved nothing else."""
        with InRepo():
            files = self._doc("The gate `check_nonexistent_thing` reads it.\n")
            found = cdl.check_hook_names(files)
        self.assertEqual(len(found), 1, found)
        self.assertIn("check_nonexistent_thing", found[0])

    def test_check_py_outside_the_check_sources_still_dead(self) -> None:
        """The accepted population is check SOURCES, not every tracked file.

        `test/interop/testdata/check_except_probe.py` is tracked and its name
        matches `CHECK_TOKEN`, but `python_check_sources` does not read it. A
        rule naming it is naming an unrelated script, and must still fail.
        """
        with InRepo():
            sources = cdl.python_check_sources()
            self.assertNotIn("test/interop/testdata/check_except_probe.py", sources)
            files = self._doc("The gate `check_except_probe.py` reads it.\n")
            found = cdl.check_hook_names(files)
        self.assertEqual(len(found), 1, found)
        self.assertIn("check_except_probe", found[0])

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


class IgnoreMarkerTest(unittest.TestCase):
    """The `doc-links: ignore` marker: its grammar and its mandatory reason.

    VALIDATES: AC-4 of spec doc-claims-are-checked-not-just-resolved -- a
               marker with no reason fails the gate and names its line, over
               every TRACKED file rather than over the walked corpus.
    PREVENTS:  the silent allowlist. 98 dead citations sat behind markers no
               gate read, and three more survived inside `ai/rules/` itself,
               each hiding a path deleted by the problem-journal migration.
    """

    def _repo(self, corpus: str, path: str = "ai/rules/sample.md") -> Path:
        d = tempfile.mkdtemp(prefix="ignore-marker-")
        self.addCleanup(lambda: shutil.rmtree(d, ignore_errors=True))
        repo = Path(d)
        subprocess.run(
            ["git", "init", "-q"], cwd=str(repo), check=True, capture_output=True
        )
        (repo / ".gitignore").write_text(IGNORED_PATHS, encoding="utf-8")
        doc = repo / path
        doc.parent.mkdir(parents=True, exist_ok=True)
        doc.write_text(corpus, encoding="utf-8")
        # The sweep reads TRACKED files, so the fixture has to be tracked.
        # A throwaway repo under tempfile: no shared index is touched.
        subprocess.run(
            ["git", "add", "-A"], cwd=str(repo), check=True, capture_output=True
        )
        return repo

    def test_ignore_marker_without_a_reason_fails(self) -> None:
        """AC-4: the gate fails and names the line."""
        repo = self._repo("Gone: `ai/rules/absent.md`. <!-- doc-links: ignore -->\n")
        res = run(repo)
        self.assertEqual(res.returncode, 1, res.stdout + res.stderr)
        self.assertIn("ai/rules/sample.md:1", res.stdout)
        self.assertIn("states no reason", res.stdout)
        # It excuses nothing meanwhile: the reference it hid is reported too.
        self.assertIn("ai/rules/absent.md", res.stdout)

    def test_reasoned_marker_still_suppresses(self) -> None:
        """The negative control: the marker keeps working when it says why."""
        repo = self._repo(
            "Gone: `ai/rules/absent.md`. "
            "<!-- doc-links: ignore (negative example, deliberately absent) -->\n"
        )
        res = run(repo)
        self.assertEqual(res.returncode, 0, res.stdout + res.stderr)

    def test_empty_parentheses_are_not_a_reason(self) -> None:
        """`()` is the shape that would make the requirement free to satisfy."""
        repo = self._repo("Gone: `ai/rules/absent.md`. <!-- doc-links: ignore () -->\n")
        res = run(repo)
        self.assertEqual(res.returncode, 1, res.stdout + res.stderr)
        self.assertIn("states no reason", res.stdout)

    def test_prose_mention_does_not_suppress(self) -> None:
        """The marker is HTML-comment grammar, never a bare substring.

        VALIDATES: a document that MENTIONS the marker in prose no longer
                   silences its own line.
        PREVENTS:  the one document that could never be checked being the one
                   that describes the marker.
        """
        repo = self._repo(
            "A line carrying `doc-links: ignore` used to be skipped, so "
            "`ai/rules/absent.md` went unseen.\n"
        )
        res = run(repo)
        self.assertEqual(res.returncode, 1, res.stdout + res.stderr)
        self.assertIn("ai/rules/absent.md", res.stdout)
        # A mention is not a marker, so it is not an unreasoned one either.
        self.assertNotIn("states no reason", res.stdout)

    def test_marker_outside_the_walked_corpus_is_still_swept(self) -> None:
        """`docs/` is in no `MD_GLOBS` pattern, and its markers are audited.

        VALIDATES: the sweep enumerates tracked files, not `MD_GLOBS`.
        PREVENTS:  the hole that made this worth writing -- a marker nobody
                   reads suppresses nothing and rots unseen, so scoping the
                   audit to the walked corpus would leave it open.
        """
        repo = self._repo(
            "Gone: `ai/rules/absent.md`. <!-- doc-links: ignore -->\n",
            path="docs/architecture/sample.md",
        )
        res = run(repo)
        self.assertEqual(res.returncode, 1, res.stdout + res.stderr)
        self.assertIn("docs/architecture/sample.md:1", res.stdout)
        self.assertIn("states no reason", res.stdout)
        # `check_markdown` still never walks docs/, so check 1 reports nothing
        # here. Check 5 reads the same file and reports the reference the
        # unreasoned marker failed to excuse.
        self.assertNotIn("broken path reference", res.stdout)
        self.assertIn("dead path reference: ai/rules/absent.md", res.stdout)

    def test_backticked_marker_is_an_example_not_a_marker(self) -> None:
        """A code span showing the syntax must not suppress its own line.

        VALIDATES: `ai/INDEX.md` can document the marker on the same line as
                   the paths it names, and those paths stay checked.
        PREVENTS:  rebuilding the prose-mention hole by writing the
                   documentation for the grammar that closed it.
        """
        repo = self._repo(
            "Write it as `<!-- doc-links: ignore (why) -->`; "
            "`ai/rules/absent.md` is still checked.\n"
        )
        res = run(repo)
        self.assertEqual(res.returncode, 1, res.stdout + res.stderr)
        self.assertIn("ai/rules/absent.md", res.stdout)
        # An example excuses nothing, so there is no exemption to audit either.
        self.assertNotIn("states no reason", res.stdout)

    def test_unreadable_file_is_a_finding(self) -> None:
        """Fail closed: a file that EXISTS and cannot be read is never a silent pass."""
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "locked.md"
            path.write_text("<!-- doc-links: ignore -->\n", encoding="utf-8")
            path.chmod(0o000)
            try:
                found = cdl.check_ignore_reasons(files=(str(path),))
            finally:
                path.chmod(0o600)
        self.assertEqual(len(found), 1, found)
        self.assertIn("cannot read", found[0])

    def test_a_file_deleted_mid_sweep_is_not_a_finding(self) -> None:
        """A path that is GONE carries no marker to audit.

        Several sessions share this checkout, so a spec closure deletes its
        spec between `git ls-files` and the read. That window produced a false
        red twice on 2026-08-10; a vanished file is skipped, an unreadable one
        above still fails closed.
        """
        self.assertEqual(cdl.check_ignore_reasons(files=("no/such/file.md",)), [])


BASELINE_REL = "scripts/dev/doc_citation_baseline.txt"


class TrackedCitationTest(unittest.TestCase):
    """Check 5: a path reference in ANY tracked file, behind a baseline.

    VALIDATES: AC-1, AC-2, AC-5, AC-6, AC-7 and AC-9 of spec
               dead-learned-citations-outside-the-walked-corpus.
    PREVENTS:  the hole that spec measured. `MD_GLOBS` names fourteen globs,
               so a dead citation in `docs/`, in a `plan/spec-*.md`, in the
               `Makefile` or in a hook was read by no gate at all, and 2704
               of them had collected.
    """

    def _repo(
        self,
        files: dict[str, str],
        baseline: str | None = None,
        commit: bool = False,
    ) -> Path:
        """A throwaway git repo holding `files`, all tracked.

        `baseline` is written before the commit, so a test can commit one
        baseline and then grow the working-tree copy against it.
        """
        d = tempfile.mkdtemp(prefix="tracked-citation-")
        self.addCleanup(lambda: shutil.rmtree(d, ignore_errors=True))
        repo = Path(d)
        subprocess.run(
            ["git", "init", "-q"], cwd=str(repo), check=True, capture_output=True
        )
        (repo / ".gitignore").write_text(IGNORED_PATHS, encoding="utf-8")
        for rel, body in files.items():
            doc = repo / rel
            doc.parent.mkdir(parents=True, exist_ok=True)
            doc.write_text(body, encoding="utf-8")
        if baseline is not None:
            self._write_baseline(repo, baseline)
        # The sweep reads TRACKED files, so the fixture has to be tracked.
        # A throwaway repo under tempfile: no shared index is touched.
        subprocess.run(
            ["git", "add", "-A"], cwd=str(repo), check=True, capture_output=True
        )
        if commit:
            subprocess.run(
                [
                    "git",
                    "-c",
                    "commit.gpgsign=false",
                    "-c",
                    "user.email=fixture@example.invalid",
                    "-c",
                    "user.name=fixture",
                    "commit",
                    "-q",
                    "-m",
                    "fixture",
                ],
                cwd=str(repo),
                check=True,
                capture_output=True,
            )
        return repo

    @staticmethod
    def _write_baseline(repo: Path, body: str) -> None:
        path = repo / BASELINE_REL
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(body, encoding="utf-8")

    def test_dead_citation_outside_md_globs_is_reported(self) -> None:
        """AC-1: the gate names the file, the line and the path, and exits 1."""
        repo = self._repo(
            {"docs/architecture/sample.md": "Read `internal/absent/thing.go`.\n"}
        )
        res = run(repo)
        self.assertEqual(res.returncode, 1, res.stdout + res.stderr)
        self.assertIn(
            "docs/architecture/sample.md:1: dead path reference: "
            "internal/absent/thing.go",
            res.stdout,
        )
        # The two ways out are on the line the author reads.
        self.assertIn("repair the reference", res.stdout)
        self.assertIn("doc-links: ignore", res.stdout)

    def test_a_dead_citation_in_a_non_markdown_file_is_reported(self) -> None:
        """The sweep is over TRACKED files, not over `*.md`.

        VALIDATES: a `.py`, `.sh` or `.go` file is swept like a document.
        PREVENTS:  the mutation every other fixture survives. Narrowing
                   `tracked_files` to `f.endswith(".md")` leaves this suite
                   green while 38 markers in `scripts/dev/hook-fixture-check.py`
                   and the citations in `.claude/hooks/*.py` go unread, which
                   is the spec's central claim deleted by one line.
        """
        repo = self._repo(
            {
                "scripts/dev/sample.py": '"""Read `internal/absent/thing.go`."""\n',
                "scripts/dev/sample.sh": "# Read `internal/absent/other.go`\n",
            }
        )
        res = run(repo)
        self.assertEqual(res.returncode, 1, res.stdout + res.stderr)
        self.assertIn("scripts/dev/sample.py:1", res.stdout)
        self.assertIn("scripts/dev/sample.sh:1", res.stdout)

    def test_live_citation_outside_md_globs_passes(self) -> None:
        """AC-2: the same file, citing a path that resolves, is no finding."""
        repo = self._repo(
            {
                "docs/architecture/sample.md": "Read `internal/x/thing.go`.\n",
                "internal/x/thing.go": "package x\n",
            }
        )
        res = run(repo)
        self.assertEqual(res.returncode, 0, res.stdout + res.stderr)

    def test_baselined_pair_is_not_reported(self) -> None:
        """AC-5: the grandfathered population passes."""
        repo = self._repo(
            {"docs/architecture/sample.md": "Read `internal/absent/thing.go`.\n"},
            baseline="docs/architecture/sample.md\tinternal/absent/thing.go\n",
        )
        res = run(repo)
        self.assertEqual(res.returncode, 0, res.stdout + res.stderr)
        self.assertIn("1 citation pair(s) baselined", res.stdout)

    def test_stale_baseline_entry_is_reported_as_drift(self) -> None:
        """AC-6: a pair the tree no longer holds is a WARN, never a failure.

        Drift is what makes the baseline shrinkable: the gate says which entry
        to delete, and it does not fail the run of the author who repaired it.
        """
        repo = self._repo(
            {"docs/architecture/sample.md": "Nothing is cited here.\n"},
            baseline="docs/architecture/sample.md\tinternal/absent/thing.go\n",
        )
        res = run(repo)
        self.assertEqual(res.returncode, 0, res.stdout + res.stderr)
        self.assertIn(
            "WARN scripts/dev/doc_citation_baseline.txt: "
            "docs/architecture/sample.md no longer cites internal/absent/thing.go",
            res.stdout,
        )
        self.assertIn("1 stale baseline entry(s)", res.stdout)

    def test_new_citer_of_baselined_target_is_reported(self) -> None:
        """AC-7: the baseline holds PAIRS, so rot cannot spread to a new file.

        The discriminating fixture: one file's pair is baselined and passes,
        and a second file citing the SAME dead target fails. A baseline keyed
        on the bare target would pass both.
        """
        repo = self._repo(
            {
                "docs/architecture/sample.md": "Read `internal/absent/thing.go`.\n",
                "docs/architecture/second.md": "Also `internal/absent/thing.go`.\n",
            },
            baseline="docs/architecture/sample.md\tinternal/absent/thing.go\n",
        )
        res = run(repo)
        self.assertEqual(res.returncode, 1, res.stdout + res.stderr)
        self.assertIn(
            "docs/architecture/second.md:1: dead path reference: "
            "internal/absent/thing.go",
            res.stdout,
        )
        self.assertNotIn("docs/architecture/sample.md:1", res.stdout)

    def test_baseline_that_grew_against_head_is_refused(self) -> None:
        """AC-9: shrink-only, enforced against HEAD.

        Both directions are asserted. Growing the committed baseline fails,
        and shrinking it passes -- without the second half the check could be
        refusing every baseline and still look correct.
        """
        repo = self._repo(
            {
                "docs/architecture/sample.md": "Read `internal/absent/thing.go`.\n",
                "docs/architecture/second.md": "Also `internal/absent/other.go`.\n",
            },
            baseline="docs/architecture/sample.md\tinternal/absent/thing.go\n",
            commit=True,
        )
        self._write_baseline(
            repo,
            "docs/architecture/sample.md\tinternal/absent/thing.go\n"
            "docs/architecture/second.md\tinternal/absent/other.go\n",
        )
        res = run(repo)
        self.assertEqual(res.returncode, 1, res.stdout + res.stderr)
        self.assertIn("1 baseline pair(s) are new against HEAD", res.stdout)
        self.assertIn("shrink-only", res.stdout)
        # The failure names the pair, so an author acts on it without a rerun.
        self.assertIn(
            "docs/architecture/second.md -> internal/absent/other.go", res.stdout
        )

        # The negative control: a baseline that shrank is accepted, and the
        # citation it stopped covering is what fails instead.
        self._write_baseline(repo, "")
        res = run(repo)
        self.assertEqual(res.returncode, 1, res.stdout + res.stderr)
        self.assertNotIn("shrink-only", res.stdout)
        self.assertIn("dead path reference: internal/absent/thing.go", res.stdout)

    def test_baseline_pair_absent_from_head_is_refused_at_equal_size(self) -> None:
        """AC-9 is over the PAIRS, so a same-size baseline is not exempt.

        The discriminating fixture: one baselined citation is repaired and its
        pair is dropped, and a new dead citation's pair takes its place. The
        total is unchanged, so a check comparing sizes passes the swap and
        grandfathers a pair the sweep had just failed on.
        """
        repo = self._repo(
            {
                "docs/architecture/sample.md": "Read `internal/absent/thing.go`.\n",
                "docs/architecture/second.md": "Plain prose, no reference.\n",
            },
            baseline="docs/architecture/sample.md\tinternal/absent/thing.go\n",
            commit=True,
        )
        # The repair: the old citation is gone. The new dead one replaces it.
        (repo / "docs/architecture/sample.md").write_text(
            "Plain prose, no reference.\n", encoding="utf-8"
        )
        (repo / "docs/architecture/second.md").write_text(
            "Also `internal/absent/other.go`.\n", encoding="utf-8"
        )
        self._write_baseline(
            repo, "docs/architecture/second.md\tinternal/absent/other.go\n"
        )
        res = run(repo)
        self.assertEqual(res.returncode, 1, res.stdout + res.stderr)
        self.assertIn("1 baseline pair(s) are new against HEAD", res.stdout)
        self.assertIn(
            "docs/architecture/second.md -> internal/absent/other.go", res.stdout
        )

    def test_no_head_baseline_yet_passes(self) -> None:
        """The first commit of the baseline has nothing to compare against.

        `git show HEAD:<baseline>` fails in a repo whose HEAD never held the
        file, and in a repo with no commit at all. Both must pass rather than
        crash: the gate would be unrunnable on the commit that arms it.
        """
        repo = self._repo(
            {"docs/architecture/sample.md": "Read `internal/absent/thing.go`.\n"},
            baseline="docs/architecture/sample.md\tinternal/absent/thing.go\n",
        )
        res = run(repo)
        self.assertEqual(res.returncode, 0, res.stdout + res.stderr)

    def test_vendored_and_handover_citations_are_not_swept(self) -> None:
        """The two excluded roots, each for the reason stated in the source.

        A vendored file's references point into its upstream repo, so nobody
        here can repair one. `plan/handover/` describes the tree as it was.
        """
        repo = self._repo(
            {
                "vendor/x/README.md": "Read `internal/absent/thing.go`.\n",
                "third_party/y/README.md": "Read `internal/absent/thing.go`.\n",
                "plan/handover/old.md": "Read `internal/absent/thing.go`.\n",
            }
        )
        res = run(repo)
        self.assertEqual(res.returncode, 0, res.stdout + res.stderr)

    def test_unreadable_file_is_a_finding(self) -> None:
        """Fail closed, as the marker sweep does: unreadable is never a pass."""
        with tempfile.TemporaryDirectory() as tmp:
            path = Path(tmp) / "locked.md"
            path.write_text("Read `internal/absent/thing.go`.\n", encoding="utf-8")
            path.chmod(0o000)
            try:
                found = cdl.check_tracked_citations(files=(str(path),), baseline=set())
            finally:
                path.chmod(0o600)
        self.assertEqual(len(found), 1, found)
        self.assertIn("cannot read", found[0])


class DesignRefTest(unittest.TestCase):
    """`// Design:` targets in Go, test files included.

    VALIDATES: AC-1, AC-3, AC-4 of spec fixit-dead-design-pointers-in-tests --
               a `_test.go` is read like any other Go file, and inside one a
               `plan/spec-` target is refused even when the spec exists.
    PREVENTS:  the class regrowing. Spec closure deletes the spec
               (ai/rules/planning.md, "Spec Closure"), so a live spec pointer
               in a test dies on an unrelated author's closure commit. 133 dead
               pointers accumulated behind the `_test.go` exclusion in
               `go_files()`, which no gate ever read.
    """

    def _repo(self, files: dict[str, str]) -> Path:
        d = tempfile.mkdtemp(prefix="design-ref-")
        self.addCleanup(lambda: shutil.rmtree(d, ignore_errors=True))
        repo = Path(d)
        subprocess.run(
            ["git", "init", "-q"], cwd=str(repo), check=True, capture_output=True
        )
        (repo / ".gitignore").write_text(IGNORED_PATHS, encoding="utf-8")
        for rel, body in files.items():
            p = repo / rel
            p.parent.mkdir(parents=True, exist_ok=True)
            p.write_text(body, encoding="utf-8")
        # `go_files()` reads `git ls-files`, so the fixture has to be tracked.
        subprocess.run(
            ["git", "add", "-A"], cwd=str(repo), check=True, capture_output=True
        )
        return repo

    def test_design_ref_in_a_test_file_is_checked(self) -> None:
        """AC-1: a dead target in a `_test.go` is reported."""
        repo = self._repo(
            {
                "internal/x/foo_test.go": (
                    "// Design: docs/architecture/absent.md\npackage x\n"
                )
            }
        )
        res = run_design(repo)
        self.assertEqual(res.returncode, 1, res.stdout + res.stderr)
        self.assertIn("internal/x/foo_test.go:1", res.stdout)
        self.assertIn("docs/architecture/absent.md", res.stdout)

    def test_design_ref_to_a_live_spec_is_refused_in_a_test_file(self) -> None:
        """AC-3: existence is not enough. The message names the rule."""
        repo = self._repo(
            {
                "plan/spec-live.md": "# Spec: live\n",
                "internal/x/foo_test.go": "// Design: plan/spec-live.md\npackage x\n",
            }
        )
        self.assertTrue((repo / "plan" / "spec-live.md").exists())
        res = run_design(repo)
        self.assertEqual(res.returncode, 1, res.stdout + res.stderr)
        self.assertIn("internal/x/foo_test.go:1", res.stdout)
        self.assertIn("plan/spec-live.md", res.stdout)
        # The finding must teach the rule, not just print the path.
        self.assertIn("durable", res.stdout)
        self.assertIn("ai/rules/planning.md", res.stdout)
        self.assertNotIn("broken Design reference", res.stdout)

    def test_design_ref_to_a_live_spec_is_allowed_outside_a_test_file(self) -> None:
        """AC-4: the ban is scoped to test files.

        The negative control for the check above: 21 live `plan/` pointers sit
        in non-test Go today, and refusing those would be a false red.
        """
        repo = self._repo(
            {
                "plan/spec-live.md": "# Spec: live\n",
                "internal/x/foo.go": "// Design: plan/spec-live.md\npackage x\n",
            }
        )
        res = run_design(repo)
        self.assertEqual(res.returncode, 0, res.stdout + res.stderr)

    def test_design_ref_outside_a_test_file_still_reports_a_dead_target(self) -> None:
        """The gate keeps the job it already did.

        Widening the file list and adding the spec refusal must not cost the
        original behavior: a non-test `.go` naming a target that does not
        exist is still a finding. Nothing else in this file pins it, so a
        later change that scoped the whole check to test files would pass.
        """
        repo = self._repo(
            {"internal/x/foo.go": "// Design: docs/architecture/absent.md\npackage x\n"}
        )
        res = run_design(repo)
        self.assertEqual(res.returncode, 1, res.stdout + res.stderr)
        self.assertIn("internal/x/foo.go:1", res.stdout)
        self.assertIn("broken Design reference", res.stdout)


class KnownRootTest(unittest.TestCase):
    """Every top-level directory this repo cites has to be a checked root.

    VALIDATES: `candidate_paths` returns a path to verify for each top-level
               directory the tree's prose cites, so a dead citation into it is
               reported rather than skipped.
    PREVENTS:  a new tree arriving with no citation checking. `contrib/` was
               added for the netlab daemon integration
               (spec-netlab-integration), and until it joined KNOWN_ROOTS every
               `contrib/...` reference in `ai/INDEX.md` was DISCARDED by the
               root filter: the discovery rows pointed into a tree the gate
               could not see, so a rename would have gone unnoticed.
    """

    def test_contrib_is_a_checked_root(self) -> None:
        """A `contrib/...` span reaches path_resolves instead of being discarded."""
        self.assertEqual(
            cdl.candidate_paths("contrib/netlab/ze.yml"),
            ["contrib/netlab/ze.yml"],
        )
        self.assertEqual(
            cdl.candidate_paths("contrib/netlab/does-not-exist.yml"),
            ["contrib/netlab/does-not-exist.yml"],
        )

    def test_the_contrib_citations_in_the_index_resolve(self) -> None:
        """The rows ai/INDEX.md carries point at files that exist.

        The assertion above only proves the grammar READS the span. This proves
        the tree it reads is really there, so neither can pass over an empty
        set.
        """
        with InRepo():
            for path in (
                "contrib/netlab/README.md",
                "contrib/netlab/ze.yml",
                "contrib/netlab/ze/ze.j2",
                "contrib/netlab/topology.yml",
                "contrib/netlab/golden/",
            ):
                self.assertTrue(
                    cdl.path_resolves(path), f"{path} is cited and does not resolve"
                )


class PytestNodeIdTest(unittest.TestCase):
    """A pytest node id names a file, and the file is what the gate checks.

    VALIDATES: `candidate_paths` strips `::test_name` the way it strips
               `:Symbol`, so the target it returns is the file itself.
    PREVENTS:  a live file reported as a dead citation. The single-colon rule
               removed the test name and left the second colon on the path, so
               `test/ipsec-interop/lab_test.py::test_x` became
               `test/ipsec-interop/lab_test.py:`, which resolves nowhere. The
               cheapest way out of that finding is a suppression over a
               reference that is not broken, which is how a gate stops meaning
               anything.
    """

    def test_a_node_id_reduces_to_its_file(self) -> None:
        self.assertEqual(
            cdl.candidate_paths("test/ipsec-interop/lab_test.py::test_x"),
            ["test/ipsec-interop/lab_test.py"],
        )

    def test_a_single_colon_symbol_still_reduces_to_its_file(self) -> None:
        self.assertEqual(
            cdl.candidate_paths("internal/x/y.go:Symbol"),
            ["internal/x/y.go"],
        )


class RealCorpusTest(unittest.TestCase):
    """AC-4: after the fixes, the lint exits 0 against the real tree."""

    def test_real_corpus_is_present_and_clean(self) -> None:
        for rel in cdl.NAME_LINT_FILES:
            self.assertTrue(
                (REPO / rel).exists(),
                f"{rel} is missing; the check would go quiet without it",
            )
        with InRepo():
            self.assertEqual(cdl.check_hook_names(), [])

    def test_real_corpus_has_no_dead_learned_citation(self) -> None:
        """AC-3: no citation of the retired numbered corpus survives anywhere.

        VALIDATES: `plan/learned/` holds three aggregate documents, and every
                   reference to it in the tree names one of them.
        PREVENTS:  this spec's own subject returning. 451 citations of
                   `plan/learned/<id>-*.md` outlived commit 2cff2050a, which
                   retired the numbered corpus, and no gate read one.

        Three assertions, because the first two are about an ABSENCE. The
        third counts the `plan/learned/` citations the grammar DOES read, so
        a sweep that saw nothing at all cannot pass this test.
        """
        learned = "plan/learned/"
        with InRepo():
            _unreadable, _markers, dead = cdl.sweep_tracked(markers=False)
            corpus_dead = [
                f"{where}: {path}"
                for where, path in cdl.check_markdown(False)
                if path.startswith(learned)
            ]
            baselined = sorted(
                pair for pair in cdl.load_baseline() if pair[1].startswith(learned)
            )
            read = 0
            named = subprocess.run(
                ["git", "grep", "-l", learned],
                capture_output=True,
                text=True,
                check=False,
            ).stdout.split()
            for rel in named:
                try:
                    text = Path(rel).read_text(encoding="utf-8", errors="replace")
                except OSError:
                    continue
                for line in text.splitlines():
                    read += sum(
                        1
                        for target in cdl.line_citations(line)
                        if target.startswith(learned) and cdl.path_resolves(target)
                    )
        surviving = [
            f"{citer}:{lineno}: {target}"
            for citer, lineno, target in dead
            if target.startswith(learned)
        ]
        self.assertEqual(surviving + corpus_dead, [], surviving + corpus_dead)
        self.assertEqual(
            baselined,
            [],
            "a plan/learned/ citation is repaired, never grandfathered",
        )
        self.assertGreater(
            read, 0, "no plan/learned/ citation was read: the sweep proves nothing"
        )

    def test_baseline_header_is_what_the_generator_writes(self) -> None:
        """The committed baseline's header is `write_baseline`'s own text.

        VALIDATES: the artifact and its generator carry ONE header.
        PREVENTS:  the divergence that shipped on 2026-08-10. The shrink-only
                   rule changed from a count to a set comparison, the header on
                   disk was corrected BY HAND, and the generator kept the old
                   text. Every gate stayed green: none of them reads a comment.
                   The next `--write-baseline` would have reverted the
                   correction and restored a description that is now false.

        The second assertion is what makes the first discriminate: comparing a
        header against itself passes over an empty file too.
        """
        disk = (REPO / cdl.BASELINE_REL).read_text(encoding="utf-8")
        self.assertTrue(
            disk.startswith(cdl.BASELINE_HEADER),
            "the baseline header was hand-edited: regenerate it with "
            "`python3 scripts/dev/check_doc_links.py --write-baseline`, or "
            "correct BASELINE_HEADER, whichever text is right",
        )
        self.assertIn("SHRINK-ONLY", cdl.BASELINE_HEADER)

        # Drive the GENERATOR, not the constant. Comparing the file against
        # BASELINE_HEADER alone stays green when `write_baseline` stops
        # emitting it, because both sides read the same constant. The sweep is
        # stubbed because this asserts what is WRITTEN, not what is found.
        with tempfile.TemporaryDirectory() as tmp:
            out = Path(tmp) / "generated.txt"
            real = cdl.sweep_tracked
            cdl.sweep_tracked = lambda *a, **k: ([], [], [])
            try:
                cdl.write_baseline(str(out))
            finally:
                cdl.sweep_tracked = real
            self.assertEqual(out.read_text(encoding="utf-8"), cdl.BASELINE_HEADER)

    def test_a_baseline_line_with_no_tab_is_reported(self) -> None:
        """A malformed baseline entry names itself.

        VALIDATES: `baseline_format_problems` reports a line with no TAB.
        PREVENTS:  a silent typo. Such a line grandfathers nothing, so the
                   citation it was written for is reported anyway, and the
                   author reads that as a gate ignoring its own baseline.

        The live file is asserted clean in the same test, so the reporter
        cannot be one that fires on everything.
        """
        with tempfile.TemporaryDirectory() as tmp:
            bad = Path(tmp) / "baseline.txt"
            bad.write_text(
                "# comment\n\ndocs/a.md\tinternal/gone.go\ndocs/b.md missing tab\n",
                encoding="utf-8",
            )
            problems = cdl.baseline_format_problems(str(bad))
        self.assertEqual(len(problems), 1, problems)
        self.assertIn("docs/b.md missing tab", problems[0])
        self.assertIn("no TAB", problems[0])
        with InRepo():
            self.assertEqual(cdl.baseline_format_problems(), [])

    def test_real_corpus_has_no_unreasoned_marker(self) -> None:
        """Every surviving marker in the tree states why (spec A-3).

        The second assertion is what makes the first mean anything: a sweep
        over a tree with no markers at all would be green and prove nothing.
        It counts reasoned markers rather than pinning a number, so removing
        one never reds this test.
        """
        with InRepo():
            self.assertEqual(cdl.check_ignore_reasons(), [])
            reasoned = 0
            for path in cdl.tracked_files():
                raw = Path(path).read_bytes()
                if cdl.MARKER_BYTES not in raw:
                    continue
                for line in raw.decode("utf-8", errors="replace").splitlines():
                    reasoned += sum(
                        1
                        for tail in cdl.ignore_markers(line)
                        if cdl.marker_reason(tail)
                    )
        self.assertGreater(reasoned, 0, "no marker was read: the sweep proves nothing")

    def test_the_sweep_excludes_its_own_implementation(self) -> None:
        """The checker and its fixtures own the marker; they are not corpus."""
        with InRepo():
            tracked = set(cdl.tracked_files())
        self.assertNotIn("scripts/dev/check_doc_links.py", tracked)
        self.assertNotIn("scripts/dev/check_doc_links_test.py", tracked)
        self.assertIn("ai/rules/repo-maintenance.md", tracked)

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
