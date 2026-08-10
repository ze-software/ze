#!/usr/bin/env python3
"""Unit tests for spec-citation-check.py (the spec citation freshness gate).

Driven end-to-end through the real entry point (subprocess) per the guard test
corollary in ai/rules/evidence.md: the exit code IS the gate, so the
gate -- not a helper -- is what gets asserted. Each synthetic fixture builds a
throwaway repo under a tempdir with its own plan/ tree and (optionally) a
baseline, so a test never depends on the real repo's rot.

One fixture is different, and it is the wiring test:
test_real_corpus_has_no_dangling_spec_citation runs the gate against THIS
repository. It is the only test in this file that reads the real tree.

Coverage:
  AC-1  test_citation_dangling_spec_fails       spec -> absent spec, no baseline
        test_citation_baselined_passes          same, but the target is baselined
        test_removing_baselined_ref_is_fine     baseline entry no longer cited
        test_learned_reference_not_fatal        learned -> closed spec is expected
        test_real_corpus_has_no_dangling_spec_citation
                                                the REAL repo, the only fixture
                                                that reads it, wiring test for
                                                `make ze-spec-citation-check`
  AC-2  test_citation_line_drift_warns          quoted token no longer on the line
        test_citation_line_token_present_no_warn token still present -> silent
"""

from __future__ import annotations

import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

HERE = Path(__file__).resolve().parent
SCRIPT = HERE / "spec-citation-check.py"


def repo_root() -> Path:
    """The real repository root: the nearest ancestor that holds plan/ and mk/.

    A wrong answer here would hand the gate a tree with no specs in it, and the
    gate is green over an empty corpus. Both markers are checked, and a miss
    raises rather than returns a guess.
    """
    for candidate in (HERE, *HERE.parents):
        if (candidate / "plan").is_dir() and (candidate / "mk").is_dir():
            return candidate
    raise AssertionError(f"no repository root above {HERE}: none holds plan/ and mk/")


SPEC_HEAD = (
    "# Spec: {name}\n\n| Field | Value |\n|-------|-------|\n| Status | design |\n\n"
)


def run(repo: Path) -> subprocess.CompletedProcess:
    return subprocess.run(
        [sys.executable, str(SCRIPT), "--repo", str(repo)],
        capture_output=True,
        text=True,
    )


def write(repo: Path, rel: str, body: str) -> None:
    p = repo / rel
    p.parent.mkdir(parents=True, exist_ok=True)
    p.write_text(body, encoding="utf-8")


class CitationGateTest(unittest.TestCase):
    def _repo(self) -> Path:
        d = tempfile.mkdtemp(prefix="citation-")
        self.addCleanup(lambda: shutil.rmtree(d, ignore_errors=True))
        repo = Path(d)
        (repo / "plan").mkdir()
        return repo

    # --- AC-1: dangling spec reference is fatal unless baselined -------------

    def test_citation_dangling_spec_fails(self):
        repo = self._repo()
        write(
            repo,
            "plan/spec-a.md",
            SPEC_HEAD.format(name="a")
            + "This depends on `plan/spec-gone.md` for the wire format.\n",  # <!-- doc-links: ignore (fixture path, deliberately absent) -->
        )
        r = run(repo)
        out = r.stdout + r.stderr
        self.assertNotEqual(r.returncode, 0, out)
        self.assertIn("plan/spec-a.md", out)  # citing spec named
        self.assertIn("plan/spec-gone.md", out)  # dangling ref named
        self.assertRegex(out, r"plan/spec-a\.md:\d+")  # citing line named

    def test_citation_baselined_passes(self):
        repo = self._repo()
        write(
            repo,
            "plan/spec-a.md",
            SPEC_HEAD.format(name="a") + "Superseded `plan/spec-gone.md`.\n",  # <!-- doc-links: ignore (fixture path, deliberately absent) -->
        )
        write(
            repo,
            "plan/.citation-baseline",
            "# known-dangling spec targets (allow-list)\nplan/spec-gone.md\n",
        )
        r = run(repo)
        self.assertEqual(r.returncode, 0, r.stdout + r.stderr)

    def test_removing_baselined_ref_is_fine(self):
        # A baseline entry that nothing references anymore must not fail the gate:
        # cleaning up a dangling reference is always allowed.
        repo = self._repo()
        write(repo, "plan/spec-a.md", SPEC_HEAD.format(name="a") + "No refs here.\n")
        write(repo, "plan/.citation-baseline", "plan/spec-gone.md\n")
        r = run(repo)
        self.assertEqual(r.returncode, 0, r.stdout + r.stderr)

    def test_learned_reference_not_fatal(self):
        # A learned summary referencing its now-closed spec is the expected result
        # of the two-commit closure model (spec git-rm'd, learned kept), so the
        # FAIL pass must be scoped to spec -> spec and must NOT fire here.
        repo = self._repo()
        (repo / "plan" / "learned").mkdir()
        write(
            repo,
            "plan/learned/1000-thing.md",
            "# 1000 thing\n\nImplements `plan/spec-thing.md` (now closed).\n",  # <!-- doc-links: ignore (fixture path, deliberately absent) -->
        )
        r = run(repo)
        self.assertEqual(r.returncode, 0, r.stdout + r.stderr)

    def test_real_corpus_has_no_dangling_spec_citation(self):
        # Wiring test for `make ze-spec-citation-check` (mk/inventory.mk:34).
        # The make target runs the script over THIS repository, so this fixture
        # runs the same entry point over the same tree. It is the only test in
        # this file that reads the real corpus; every other one builds a
        # throwaway repo. Spec closure removes a spec file and every sibling
        # citation of it survives the removal, so this row goes red the moment a
        # closure lands without clearing its citers.
        root = repo_root()
        specs = sorted((root / "plan").glob("spec-*.md"))
        self.assertTrue(
            specs,
            f"{root}/plan holds no spec-*.md: the gate is green over an empty"
            " corpus, so this test would pass without reading anything",
        )
        r = run(root)
        out = r.stdout + r.stderr
        self.assertEqual(
            r.returncode,
            0,
            f"spec-citation-check is RED over {root} ({len(specs)} specs):\n{out}",
        )

    # --- AC-2: line-token drift is a non-fatal WARN -------------------------

    def test_citation_line_drift_warns(self):
        repo = self._repo()
        write(repo, "src/foo.go", "package foo\nnewToken := 1\n")
        write(
            repo,
            "plan/spec-a.md",
            SPEC_HEAD.format(name="a")
            + "The guard `oldToken` lives at `src/foo.go:2`.\n",
        )
        r = run(repo)
        out = r.stdout + r.stderr
        self.assertEqual(r.returncode, 0, out)  # WARN is non-fatal
        self.assertIn("WARN", out)
        self.assertIn("src/foo.go:2", out)  # citation named
        self.assertIn("oldToken", out)  # missing token named

    def test_citation_line_token_present_no_warn(self):
        repo = self._repo()
        write(repo, "src/foo.go", "package foo\nnewToken := 1\n")
        write(
            repo,
            "plan/spec-a.md",
            SPEC_HEAD.format(name="a")
            + "The guard `newToken` lives at `src/foo.go:2`.\n",
        )
        r = run(repo)
        out = r.stdout + r.stderr
        self.assertEqual(r.returncode, 0, out)
        self.assertNotIn("WARN", out)


if __name__ == "__main__":
    unittest.main()
