#!/usr/bin/env python3
"""Unit tests for check_doc_links.py, the corpus path-reference gate.

Driven end-to-end through the real entry point (subprocess) per the guard test
corollary in ai/rules/fail-closed-guards.md: the exit code IS the gate. Each
fixture builds a throwaway git repo so a test never depends on the real tree.

VALIDATES: a reference to a gitignored (generated) path is not reported missing
           when the generator has not run, while a reference to any other
           missing path still fails the gate.
PREVENTS:  the CI-only red where a fresh checkout has no CLAUDE.md /
           AGENTS.md / .claude/skills (all gitignored, all produced by
           `make ze-ai-sync`) and every rule file citing them is called broken.
"""

from __future__ import annotations

import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

HERE = Path(__file__).resolve().parent
SCRIPT = HERE / "check_doc_links.py"

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


if __name__ == "__main__":
    unittest.main()
