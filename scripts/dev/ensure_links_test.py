#!/usr/bin/env python3
"""Unit tests for scripts/dev/ensure-links.py symlink handling.

VALIDATES: ensure_symlink's repoint policy in all three polarities.
PREVENTS: a foreign HOME hijacking the shared cache/ symlink. `make
ze-qemu-*-test` runs `make ze-unit-test-cached` INSIDE the VM with HOME=/root
(scripts/evidence/qemu-run.py), the repo is 9p-mounted read-write, and every
QEMU run silently repointed the host's cache/ to /root/.cache/ze. That was
invisible until GOCACHE moved under cache/ (Makefile:17), at which point the
next host build died with "failed to initialize build cache ... mkdir cache:
file exists" against the dangling link.
"""

from __future__ import annotations

import importlib.util
import sys
import tempfile
import unittest
from pathlib import Path


def _load():
    """Import the hyphenated script as a module."""
    path = Path(__file__).resolve().parent / "ensure-links.py"
    spec = importlib.util.spec_from_file_location("ensure_links", path)
    module = importlib.util.module_from_spec(spec)
    # Register before exec: dataclasses and other decorators resolve the module
    # out of sys.modules while the module body is still executing.
    sys.modules[spec.name] = module
    spec.loader.exec_module(module)
    return module


ensure_links = _load()


class EnsureSymlinkTest(unittest.TestCase):
    def test_already_correct_is_ok(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            target = root / "target"
            target.mkdir()
            link = root / "cache"
            link.symlink_to(target)
            result = ensure_links.ensure_symlink(link, target, False)
            self.assertTrue(result.startswith("ok"), result)
            self.assertEqual(Path(link).resolve(), target.resolve())

    def test_foreign_live_target_is_kept_not_repointed(self):
        # The VM case: HOME differs, the current target is a real directory.
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            mine = root / "mine"
            mine.mkdir()
            foreign = root / "foreign"
            link = root / "cache"
            link.symlink_to(mine)
            result = ensure_links.ensure_symlink(link, foreign, False)
            self.assertTrue(result.startswith("kept"), result)
            self.assertEqual(Path(link).resolve(), mine.resolve())
            # A target we declined to use must not be created as a side effect.
            self.assertFalse(foreign.exists())

    def test_dangling_target_is_repointed_even_when_not_repoint_live(self):
        # repoint_live=False must NOT strand a link whose target is gone.
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            gone = root / "gone"
            link = root / "cache"
            link.symlink_to(gone)
            target = root / "target"
            result = ensure_links.ensure_symlink(link, target, False)
            self.assertTrue(result.startswith("repointed"), result)
            self.assertEqual(Path(link).resolve(), target.resolve())

    def test_repoint_live_true_still_repoints_a_live_target(self):
        # The tmp/ case: keyed on the checkout path, so a mismatch is real drift
        # (the checkout moved) and must be followed even though the old dir lives.
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            old = root / "old"
            old.mkdir()
            new = root / "new"
            link = root / "tmp"
            link.symlink_to(old)
            result = ensure_links.ensure_symlink(link, new, True)
            self.assertTrue(result.startswith("repointed"), result)
            self.assertEqual(Path(link).resolve(), new.resolve())

    def test_real_directory_is_refused_not_clobbered(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            link = root / "cache"
            link.mkdir()
            (link / "keepme").write_text("user work")
            result = ensure_links.ensure_symlink(link, root / "target", False)
            self.assertTrue(result.startswith("SKIP"), result)
            self.assertTrue((link / "keepme").exists())


if __name__ == "__main__":
    unittest.main()
