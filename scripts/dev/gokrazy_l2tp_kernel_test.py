#!/usr/bin/env python3
"""Unit tests for the L2TP boot proof's kernel-package validation.

VALIDATES: the gokrazy L2TP evidence refuses to boot a kernel that cannot
pass ze's fail-closed PPPoL2TP probe (the pinned rtr7 kernel has no l2tp
support, so booting it crash-loops the appliance daemon at first boot), and
rejects wrong-architecture or stub kernel trees before wasting an image build.
PREVENTS: regressing to the pre-2026-07-23 behavior where the proof silently
built the pinned kernel and every run died waiting for "web server listening".
"""

import importlib.util
import tempfile
import unittest
from pathlib import Path

SCRIPT = (
    Path(__file__).resolve().parents[1] / "evidence" / "effective-gokrazy-l2tp-ppp.py"
)


def _load():
    spec = importlib.util.spec_from_file_location("gokrazy_l2tp_proof", SCRIPT)
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


PROOF = _load()

# Minimal headers carrying each architecture's kernel magic at its offset.
AMD64_VMLINUZ = bytes(0x202) + b"HdrS" + bytes(64)
ARM64_VMLINUZ = bytes(0x38) + b"ARMd" + bytes(64)


def _write_pkg(
    root: Path,
    vmlinuz: bytes | None = AMD64_VMLINUZ,
    builtin: str | None = "kernel/net/l2tp/l2tp_ppp.ko\n",
    release: str = "7.1.1",
) -> Path:
    pkg = root / "pkg"
    if vmlinuz is not None:
        pkg.mkdir(parents=True, exist_ok=True)
        (pkg / "vmlinuz").write_bytes(vmlinuz)
    if builtin is not None:
        moddir = pkg / "lib" / "modules" / release
        moddir.mkdir(parents=True, exist_ok=True)
        (moddir / "modules.builtin").write_text(builtin, encoding="utf-8")
    return pkg


class KernelPkgProblemsTest(unittest.TestCase):
    def test_valid_amd64_builtin_l2tp_accepted(self):
        with tempfile.TemporaryDirectory() as tmp:
            pkg = _write_pkg(Path(tmp))
            self.assertEqual(PROOF.kernel_pkg_problems(pkg, "amd64"), [])

    def test_valid_arm64_builtin_l2tp_accepted(self):
        with tempfile.TemporaryDirectory() as tmp:
            pkg = _write_pkg(Path(tmp), vmlinuz=ARM64_VMLINUZ)
            self.assertEqual(PROOF.kernel_pkg_problems(pkg, "arm64"), [])

    def test_missing_vmlinuz_rejected(self):
        with tempfile.TemporaryDirectory() as tmp:
            pkg = _write_pkg(Path(tmp), vmlinuz=None)
            problems = PROOF.kernel_pkg_problems(pkg, "amd64")
            self.assertTrue(any("no vmlinuz" in p for p in problems), problems)

    def test_wrong_architecture_rejected(self):
        # The polluted-modcache case: an arm64 kernel under the amd64 pin.
        with tempfile.TemporaryDirectory() as tmp:
            pkg = _write_pkg(Path(tmp), vmlinuz=ARM64_VMLINUZ)
            problems = PROOF.kernel_pkg_problems(pkg, "amd64")
            self.assertTrue(any("not a amd64 kernel" in p for p in problems), problems)

    def test_pinned_kernel_without_l2tp_rejected(self):
        # The crash-loop case: a genuine kernel whose modules.builtin has no
        # l2tp (the pinned rtr7 kernel ships zero loadable modules too).
        with tempfile.TemporaryDirectory() as tmp:
            pkg = _write_pkg(Path(tmp), builtin="kernel/drivers/net/e1000.ko\n")
            problems = PROOF.kernel_pkg_problems(pkg, "amd64")
            self.assertTrue(any("no PPPoL2TP support" in p for p in problems), problems)

    def test_stub_cache_tree_rejected(self):
        # The poisoned-cache case: placeholder files instead of a kernel tree.
        with tempfile.TemporaryDirectory() as tmp:
            pkg = Path(tmp) / "pkg"
            pkg.mkdir()
            (pkg / "vmlinuz").write_text("runtime-kernel\n", encoding="utf-8")
            problems = PROOF.kernel_pkg_problems(pkg, "amd64")
            self.assertTrue(any("not a amd64 kernel" in p for p in problems), problems)
            self.assertTrue(any("modules.builtin" in p for p in problems), problems)

    def test_loadable_module_accepted_without_builtin_entry(self):
        with tempfile.TemporaryDirectory() as tmp:
            pkg = _write_pkg(Path(tmp), builtin="kernel/drivers/net/e1000.ko\n")
            ko = (
                pkg
                / "lib"
                / "modules"
                / "7.1.1"
                / "kernel"
                / "net"
                / "l2tp"
                / "l2tp_ppp.ko"
            )
            ko.parent.mkdir(parents=True, exist_ok=True)
            ko.write_bytes(b"\x7fELF")
            self.assertEqual(PROOF.kernel_pkg_problems(pkg, "amd64"), [])


class AssertKernelPkgTest(unittest.TestCase):
    def test_raises_systemexit_naming_the_rebuild_command(self):
        with tempfile.TemporaryDirectory() as tmp:
            pkg = _write_pkg(Path(tmp), builtin="nothing\n")
            with self.assertRaises(SystemExit) as ctx:
                PROOF.assert_kernel_pkg(pkg, "amd64", "unit test")
            msg = str(ctx.exception)
            self.assertIn("make ze-kernel KERNEL_ARCH=amd64", msg)
            self.assertIn("unit test", msg)

    def test_valid_package_passes(self):
        with tempfile.TemporaryDirectory() as tmp:
            pkg = _write_pkg(Path(tmp))
            PROOF.assert_kernel_pkg(pkg, "amd64", "unit test")


class MagicComparisonTest(unittest.TestCase):
    """Fixtures LONGER than the magic offset, so rejection can only come from
    the magic-byte comparison itself, never the short-read guard. Without
    these, deleting the comparison and keeping the length check stays green
    while a real (multi-megabyte) wrong-arch kernel is accepted."""

    def test_long_arm64_image_rejected_as_amd64(self):
        with tempfile.TemporaryDirectory() as tmp:
            long_arm64 = bytes(0x38) + b"ARMd" + bytes(0x400)
            pkg = _write_pkg(Path(tmp), vmlinuz=long_arm64)
            problems = PROOF.kernel_pkg_problems(pkg, "amd64")
            self.assertTrue(any("not a amd64 kernel" in p for p in problems), problems)

    def test_long_amd64_bzimage_rejected_as_arm64(self):
        with tempfile.TemporaryDirectory() as tmp:
            long_amd64 = bytes(0x202) + b"HdrS" + bytes(0x400)
            pkg = _write_pkg(Path(tmp), vmlinuz=long_amd64)
            problems = PROOF.kernel_pkg_problems(pkg, "arm64")
            self.assertTrue(any("not a arm64 kernel" in p for p in problems), problems)

    def test_unsupported_arch_fails_with_message_not_keyerror(self):
        with tempfile.TemporaryDirectory() as tmp:
            pkg = _write_pkg(Path(tmp))
            with self.assertRaises(SystemExit) as ctx:
                PROOF.kernel_pkg_problems(pkg, "riscv64")
            self.assertIn("unsupported", str(ctx.exception))


class VersionPinTest(unittest.TestCase):
    def test_stale_release_rejected_against_pin(self):
        with tempfile.TemporaryDirectory() as tmp:
            pkg = _write_pkg(Path(tmp), release="7.1.1")
            problems = PROOF.kernel_pkg_problems(pkg, "amd64", "7.1.4")
            self.assertTrue(
                any("pinned kernel.version 7.1.4" in p for p in problems), problems
            )

    def test_matching_release_accepted(self):
        with tempfile.TemporaryDirectory() as tmp:
            pkg = _write_pkg(Path(tmp), release="7.1.4")
            self.assertEqual(PROOF.kernel_pkg_problems(pkg, "amd64", "7.1.4"), [])

    def test_suffixed_release_accepted(self):
        with tempfile.TemporaryDirectory() as tmp:
            pkg = _write_pkg(Path(tmp), release="7.1.4-ze")
            self.assertEqual(PROOF.kernel_pkg_problems(pkg, "amd64", "7.1.4"), [])

    def test_prefix_collision_rejected(self):
        # 7.1.40 must not satisfy a 7.1.4 pin.
        with tempfile.TemporaryDirectory() as tmp:
            pkg = _write_pkg(Path(tmp), release="7.1.40")
            problems = PROOF.kernel_pkg_problems(pkg, "amd64", "7.1.4")
            self.assertTrue(problems, "7.1.40 accepted against a 7.1.4 pin")


class _ResolveBase(unittest.TestCase):
    """Shared pinning: resolve_kernel_pkg validates against the module-global
    ARCH (read from the caller's GOKRAZY_ARCH at import time), so tests pin it
    to amd64 the same way KERNEL_PKG is pinned — otherwise an operator with
    GOKRAZY_ARCH=arm64 exported gets spurious, wrong-reason results."""

    def setUp(self):
        self._arch = PROOF.ARCH
        PROOF.ARCH = "amd64"
        self._kernel_pkg = PROOF.os.environ.pop("KERNEL_PKG", None)

    def tearDown(self):
        PROOF.ARCH = self._arch
        if self._kernel_pkg is None:
            PROOF.os.environ.pop("KERNEL_PKG", None)
        else:
            PROOF.os.environ["KERNEL_PKG"] = self._kernel_pkg


class ResolveExplicitKernelPkgTest(_ResolveBase):
    def test_explicit_kernel_pkg_is_validated(self):
        # KERNEL_PKG pointing at an l2tp-less tree must fail fast, not build.
        with tempfile.TemporaryDirectory() as tmp:
            pkg = _write_pkg(Path(tmp), builtin="nothing\n")
            PROOF.os.environ["KERNEL_PKG"] = str(pkg)
            with self.assertRaises(SystemExit):
                PROOF.resolve_kernel_pkg(Path(tmp), Path(tmp) / "work")

    def test_explicit_valid_kernel_pkg_copied_per_run(self):
        with tempfile.TemporaryDirectory() as tmp:
            pkg = _write_pkg(Path(tmp))
            work = Path(tmp) / "work"
            work.mkdir()
            PROOF.os.environ["KERNEL_PKG"] = str(pkg)
            got = PROOF.resolve_kernel_pkg(Path(tmp), work)
            # A per-run copy under work, not the shared path itself.
            self.assertEqual(got, work / "kernel-pkg")
            self.assertEqual(PROOF.kernel_pkg_problems(got, "amd64"), [])


def _fixture_root(tmp: Path, pinned: str = "7.1.4") -> Path:
    root = tmp / "root"
    (root / "internal" / "appliance").mkdir(parents=True)
    (root / "internal" / "appliance" / "kernel.version").write_text(
        pinned + "\n", encoding="utf-8"
    )
    (root / "tmp" / "kernel").mkdir(parents=True)
    return root


class ResolveStagedAndCacheTest(_ResolveBase):
    """The non-explicit paths, with the subprocess boundary faked: a valid
    staged pkg short-circuits make entirely; the cache probe's stdout parsing
    fails closed with the remediation, never an IndexError."""

    def test_valid_staged_pkg_reused_without_any_subprocess(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = _fixture_root(Path(tmp))
            staged = root / "tmp" / "kernel" / "pkg"
            staged.mkdir()
            (staged / "vmlinuz").write_bytes(AMD64_VMLINUZ)
            moddir = staged / "lib" / "modules" / "7.1.4"
            moddir.mkdir(parents=True)
            (moddir / "modules.builtin").write_text(
                "kernel/net/l2tp/l2tp_ppp.ko\n", encoding="utf-8"
            )
            work = Path(tmp) / "work"
            work.mkdir()
            calls = []
            orig = PROOF.run_required
            PROOF.run_required = lambda *a, **k: calls.append(a) or self.fail(
                "run_required called on the staged-pkg path"
            )
            try:
                got = PROOF.resolve_kernel_pkg(root, work)
            finally:
                PROOF.run_required = orig
            self.assertEqual(got, work / "kernel-pkg")
            self.assertEqual(calls, [])

    def test_stale_staged_pkg_not_reused_across_version_bump(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = _fixture_root(Path(tmp), pinned="7.1.4")
            staged = root / "tmp" / "kernel" / "pkg"
            staged.mkdir()
            (staged / "vmlinuz").write_bytes(AMD64_VMLINUZ)
            moddir = staged / "lib" / "modules" / "7.1.1"
            moddir.mkdir(parents=True)
            (moddir / "modules.builtin").write_text(
                "kernel/net/l2tp/l2tp_ppp.ko\n", encoding="utf-8"
            )
            work = Path(tmp) / "work"
            work.mkdir()

            class _Probe:
                stdout = ""

            orig = PROOF.run_required
            PROOF.run_required = lambda *a, **k: _Probe()
            try:
                with self.assertRaises(SystemExit) as ctx:
                    PROOF.resolve_kernel_pkg(root, work)
            finally:
                PROOF.run_required = orig
            # It fell through to the cache probe (stale pkg NOT reused) and
            # the empty probe output failed closed with a next step.
            self.assertIn("produced no output", str(ctx.exception))

    def test_bad_cache_content_names_the_build_command(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = _fixture_root(Path(tmp))
            cache = Path(tmp) / "cache-entry"
            cache.mkdir()
            (cache / "vmlinuz").write_text("runtime-kernel\n", encoding="utf-8")
            work = Path(tmp) / "work"
            work.mkdir()

            class _Probe:
                stdout = str(cache) + "\n"

            orig = PROOF.run_required
            PROOF.run_required = lambda *a, **k: _Probe()
            try:
                with self.assertRaises(SystemExit) as ctx:
                    PROOF.resolve_kernel_pkg(root, work)
            finally:
                PROOF.run_required = orig
            msg = str(ctx.exception)
            self.assertIn("make ze-kernel KERNEL_ARCH=amd64", msg)
            self.assertIn("sudo", msg)
            # Existing-but-bad cache: the remediation MUST say to remove the
            # entry first, because make's HIT branch is existence-only and
            # would re-materialize the bad tree as-is.
            self.assertIn(f"rm -rf {cache}", msg)

    def test_missing_cache_does_not_demand_removal(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = _fixture_root(Path(tmp))
            missing = Path(tmp) / "cache-absent"
            work = Path(tmp) / "work"
            work.mkdir()

            class _Probe:
                stdout = str(missing) + "\n"

            orig = PROOF.run_required
            PROOF.run_required = lambda *a, **k: _Probe()
            try:
                with self.assertRaises(SystemExit) as ctx:
                    PROOF.resolve_kernel_pkg(root, work)
            finally:
                PROOF.run_required = orig
            msg = str(ctx.exception)
            self.assertIn("build it once with: make ze-kernel KERNEL_ARCH=amd64", msg)
            self.assertNotIn("rm -rf", msg)


if __name__ == "__main__":
    unittest.main()
