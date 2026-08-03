#!/usr/bin/env python3
"""Unit tests for fuzz-targets.py, the fuzz-target discovery generator.

The generator replaces the hand-maintained enumeration in mk/test-fuzz.mk: it
walks internal/ for `func Fuzz`, resolves each to its exact package directory,
and emits one anchored `-fuzz=^<Name>$` invocation per target into the committed
mk/test-fuzz-targets.mk fragment. These tests pin the four behaviours the spec
(plan/spec-fixit-fuzz-target-discovery.md) requires: full coverage, name
anchoring, exact package paths, and stale-fragment detection.

The stale check (AC-6) is driven through the real CLI entry point (subprocess),
per the guard-testing corollary in ai/rules/evidence.md: the exit code
IS the gate, so the gate is what gets asserted, not just the render helper.
"""

from __future__ import annotations

import importlib.util
import os
import re
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

_HERE = Path(__file__).resolve().parent
_GEN = _HERE / "fuzz-targets.py"


def _load_module():
    spec = importlib.util.spec_from_file_location("fuzz_targets", _GEN)
    mod = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(mod)
    return mod


ft = _load_module()

# Repo root (scripts/dev/ -> repo root is parents[2]).
REPO_ROOT = _HERE.parents[1]

ISIS_TARGETS = {"FuzzISISDecodePDU", "FuzzISISTLVIterator", "FuzzISISRoundTrip"}
OSPF_TARGETS = {
    "FuzzOSPFDecodePacket",
    "FuzzOSPFLSAIterator",
    "FuzzOSPFRoundTrip",
    "FuzzOSPFTEBody",
    "FuzzOSPFRIBody",
    "FuzzOSPFExtPrefixBody",
    "FuzzOSPFExtLinkBody",
}


def _grep_fuzz_names(root: Path) -> set[str]:
    """Ground truth: every `func Fuzz<Name>` under internal/, by name."""
    names: set[str] = set()
    pat = re.compile(r"^func (Fuzz[A-Za-z0-9_]+)\s*\(", re.MULTILINE)
    for path in (root / "internal").rglob("*_test.go"):
        text = path.read_text(encoding="utf-8", errors="replace")
        names.update(pat.findall(text))
    return names


class TestFuzzDiscoveryCoversAllTargets(unittest.TestCase):
    """AC-1, AC-4: every `func Fuzz` is discovered, incl. all 10 ISIS/OSPF."""

    def test_covers_all_and_isis_ospf(self):
        discovered = {name for name, _ in ft.discover(REPO_ROOT)}
        ground = _grep_fuzz_names(REPO_ROOT)
        self.assertEqual(
            discovered,
            ground,
            "discovered fuzz names must equal the set of `func Fuzz` in internal/",
        )
        missing = (ISIS_TARGETS | OSPF_TARGETS) - discovered
        self.assertEqual(missing, set(), f"ISIS/OSPF targets not discovered: {missing}")

    def test_new_target_auto_included(self):
        # AC-4: a new func FuzzX in any package is picked up with no edit.
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            pkg = root / "internal" / "widget"
            pkg.mkdir(parents=True)
            (pkg / "widget_fuzz_test.go").write_text(
                'package widget\n\nimport "testing"\n\n'
                "func FuzzWidgetParse(f *testing.F) {}\n"
            )
            names = {name for name, _ in ft.discover(root)}
            self.assertIn("FuzzWidgetParse", names)


class TestFuzzDiscoveryAnchorsNames(unittest.TestCase):
    """AC-2: prefix-colliding names in one package emit `-fuzz=^<Name>$`."""

    def test_anchored_regexp(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            pkg = root / "internal" / "vpn"
            pkg.mkdir(parents=True)
            # FuzzParseVPN is a prefix of FuzzParseVPNAddPath.
            (pkg / "types_fuzz_test.go").write_text(
                'package vpn\n\nimport "testing"\n\n'
                "func FuzzParseVPN(f *testing.F) {}\n"
                "func FuzzParseVPNAddPath(f *testing.F) {}\n"
            )
            content = ft.render(ft.discover(root))
            # Make-escaped anchors: `$$` renders to a literal `$` for `go test`.
            self.assertIn("-fuzz=^FuzzParseVPN$$", content)
            self.assertIn("-fuzz=^FuzzParseVPNAddPath$$", content)
            # No un-anchored form that would match "more than one fuzz target".
            self.assertNotRegex(content, r"-fuzz=FuzzParseVPN\b")

    def test_every_line_anchored(self):
        content = ft.render(ft.discover(REPO_ROOT))
        for line in content.splitlines():
            if "$(GO_TEST)" not in line:
                continue
            self.assertRegex(
                line,
                r"-fuzz=\^Fuzz[A-Za-z0-9_]+\$\$",
                f"emitted -fuzz line is not anchored: {line}",
            )


class TestFuzzDiscoveryExactPackagePath(unittest.TestCase):
    """AC-3: a package with a `yang` sibling emits the exact packet path."""

    def test_exact_path_no_ellipsis(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            packet = root / "internal" / "plugins" / "isis" / "packet"
            yang = root / "internal" / "plugins" / "isis" / "yang"
            packet.mkdir(parents=True)
            yang.mkdir(parents=True)
            (packet / "fuzz_test.go").write_text(
                'package packet\n\nimport "testing"\n\n'
                "func FuzzISISDecodePDU(f *testing.F) {}\n"
            )
            (yang / "embed.go").write_text("package yang\n")
            targets = ft.discover(root)
            self.assertEqual(len(targets), 1)
            name, pkg = targets[0]
            self.assertEqual(name, "FuzzISISDecodePDU")
            self.assertEqual(pkg, "./internal/plugins/isis/packet")

    def test_real_tree_no_ellipsis(self):
        content = ft.render(ft.discover(REPO_ROOT))
        self.assertNotIn("/...", content)
        # The exact ISIS packet package must appear (not a wildcard).
        self.assertIn("./internal/plugins/isis/packet", content)


class TestFuzzDiscoveryCheckDetectsStale(unittest.TestCase):
    """AC-6: `fuzz-targets.py --check` exits non-zero on a stale fragment."""

    def _run_check(self, root: Path) -> subprocess.CompletedProcess:
        return subprocess.run(
            [sys.executable, str(_GEN), "--check", "--root", str(root)],
            capture_output=True,
            text=True,
        )

    def _run_write(self, root: Path) -> subprocess.CompletedProcess:
        return subprocess.run(
            [sys.executable, str(_GEN), "--root", str(root)],
            capture_output=True,
            text=True,
        )

    def test_stale_then_fresh(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            pkg = root / "internal" / "widget"
            pkg.mkdir(parents=True)
            (pkg / "widget_fuzz_test.go").write_text(
                'package widget\n\nimport "testing"\n\n'
                "func FuzzWidgetParse(f *testing.F) {}\n"
            )
            (root / "mk").mkdir()
            frag = root / "mk" / "test-fuzz-targets.mk"

            # Missing fragment -> stale.
            res = self._run_check(root)
            self.assertNotEqual(res.returncode, 0)
            self.assertIn("is stale; run make generate", res.stdout + res.stderr)

            # Write it, then --check passes.
            wres = self._run_write(root)
            self.assertEqual(wres.returncode, 0, wres.stderr)
            res = self._run_check(root)
            self.assertEqual(res.returncode, 0, res.stdout + res.stderr)

            # Tamper -> stale again.
            frag.write_text(frag.read_text(encoding="utf-8") + "\n# tampered\n")
            res = self._run_check(root)
            self.assertNotEqual(res.returncode, 0)

    def test_committed_fragment_is_fresh(self):
        # The checked-in mk/test-fuzz-targets.mk must match discovery, so a
        # forgotten `make generate` is caught. This is the repo self-test.
        res = self._run_check(REPO_ROOT)
        self.assertEqual(
            res.returncode,
            0,
            "committed mk/test-fuzz-targets.mk is stale; run make generate\n"
            + res.stdout
            + res.stderr,
        )


if __name__ == "__main__":
    unittest.main()
