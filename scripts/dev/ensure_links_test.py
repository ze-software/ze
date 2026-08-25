#!/usr/bin/env python3
"""Unit tests for scripts/dev/ensure-links.py symlink handling.

VALIDATES: ensure_symlink never silently repoints cache/, in every shape the
QEMU VM actually presents.
PREVENTS: a foreign environment hijacking the shared cache/ symlink. `make
ze-qemu-*-test` runs `make ze-unit-test-cached` INSIDE the VM with HOME=/root
(scripts/evidence/qemu-run.py) over a read-write 9p mount, so ensure-links runs
there against the host's checkout and repointed cache/ to /root/.cache/ze. That
was invisible until GOCACHE moved under cache/ (Makefile:17), after which the
next host build died with "failed to initialize build cache ... mkdir cache:
file exists".

The two tests that matter are test_foreign_target_missing_is_not_repointed and
test_unreadable_current_target_does_not_raise: a first attempt at this fix
gated on "is the current target a live directory?", which FAILS from inside the
VM (the host's target does not exist there, so the probe says "dangling" and
repoints anyway) and CRASHES when the current target is unreadable
(Path.is_dir() raises PermissionError on Python 3.12 rather than returning
False). Both shapes are pinned below.
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
    # Register before exec: decorators resolve the module out of sys.modules
    # while the module body is still executing.
    sys.modules[spec.name] = module
    spec.loader.exec_module(module)
    return module


ensure_links = _load()


class MigrateRefusesAnUndeletableTreeTest(unittest.TestCase):
    """migrate() has no undo, so what it cannot finish it must not start."""

    def test_a_read_only_directory_is_refused_before_anything_moves(self):
        # The measured case: `go` writes gomodcache directories mode 0555 on
        # purpose. Across devices shutil.move is copytree-then-rmtree, and
        # rmtree cannot unlink a child of a 0555 directory, so the move dies
        # partway with entries on both sides and no record of how far it got.
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            link = root / "tmp"
            (link / "keep").mkdir(parents=True)
            (link / "keep" / "session-id").write_text("do not lose me")
            cache = link / "gomodcache" / "pkg"
            cache.mkdir(parents=True)
            (cache / "locked").write_text("x")
            cache.chmod(0o555)
            target = root / "target"
            try:
                result = ensure_links.migrate(link, target, False)

                self.assertTrue(result.startswith("REFUSE"), result)
                self.assertIn("gomodcache", result)
                self.assertIn("modcacherw", result)
                # Nothing moved: the refusal is the whole point.
                self.assertFalse(target.exists(), "the target was created anyway")
                self.assertTrue((link / "keep" / "session-id").exists())
                self.assertFalse(link.is_symlink())
            finally:
                cache.chmod(0o755)

    def test_a_deletable_tree_still_migrates(self):
        # The refusal must not fire on an ordinary tree, or it replaces one
        # failure mode with another.
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            link = root / "tmp"
            (link / "sub").mkdir(parents=True)
            (link / "sub" / "file").write_text("x")
            target = root / "target"

            result = ensure_links.migrate(link, target, False)

            self.assertTrue(result.startswith("migrated"), result)
            self.assertTrue(link.is_symlink())
            self.assertEqual((target / "sub" / "file").read_text(), "x")


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

    def test_foreign_live_target_is_not_repointed(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            mine = root / "mine"
            mine.mkdir()
            foreign = root / "foreign"
            link = root / "cache"
            link.symlink_to(mine)
            result = ensure_links.ensure_symlink(link, foreign, False)
            self.assertTrue(result.startswith("MISMATCH"), result)
            self.assertEqual(Path(link).resolve(), mine.resolve())
            self.assertFalse(foreign.exists())

    def test_foreign_target_missing_is_not_repointed(self):
        # THE VM SHAPE. Inside the QEMU guest the host's cache target does not
        # exist, so any "is the current target alive?" heuristic concludes
        # "dangling" and hijacks the link. Absence must not authorise a repoint.
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            host_target = root / "does-not-exist-in-the-guest"
            guest_target = root / "root-cache-ze"
            link = root / "cache"
            link.symlink_to(host_target)
            result = ensure_links.ensure_symlink(link, guest_target, False)
            self.assertTrue(result.startswith("MISMATCH"), result)
            self.assertEqual(Path(link).readlink(), host_target)
            self.assertFalse(guest_target.exists())

    def test_unreadable_current_target_does_not_raise(self):
        # Path.is_dir() raises PermissionError (not False) on Python 3.12 for an
        # unreadable path, which crashed ensure-links against /root/.cache/ze.
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            secret = root / "secret"
            secret.mkdir()
            (secret / "inner").mkdir()
            secret.chmod(0o000)
            link = root / "cache"
            link.symlink_to(secret / "inner")
            try:
                result = ensure_links.ensure_symlink(link, root / "target", False)
            finally:
                secret.chmod(0o755)
            self.assertTrue(result.startswith("MISMATCH"), result)

    def test_explicit_repoint_is_honoured(self):
        # --repoint-cache passes auto_repoint=True; that is the ONLY way cache/
        # moves, and it is also how tmp/ follows a moved checkout.
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            old = root / "old"
            old.mkdir()
            new = root / "new"
            link = root / "cache"
            link.symlink_to(old)
            result = ensure_links.ensure_symlink(link, new, True)
            self.assertTrue(result.startswith("repointed"), result)
            self.assertEqual(Path(link).resolve(), new.resolve())

    def test_absent_link_is_created_even_without_auto_repoint(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            target = root / "target"
            link = root / "cache"
            result = ensure_links.ensure_symlink(link, target, False)
            self.assertTrue(result.startswith("created"), result)
            self.assertEqual(Path(link).resolve(), target.resolve())

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
