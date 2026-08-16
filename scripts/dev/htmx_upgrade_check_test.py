#!/usr/bin/env python3
"""Unit tests for htmx_upgrade_check.

Collected by TestPythonUnitTests (scripts/dev/python_tests_test.go).

The gate is driven as a SUBPROCESS against synthetic trees, because its exit
code is what a make target reads and what a reviewer trusts
(`ai/rules/evidence.md`). Its derivation helper is driven in-process, one
property per test.

The live tree is exercised too: the repository's own scan must reach every
package that embeds htmx, so a fourth interface cannot be added outside the
gate's sight without a red test.
"""

import os
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path

import htmx_upgrade_check as gate

TOOL = Path(gate.__file__).resolve()
REPO = TOOL.parent.parent.parent


def write(path: Path, text: str) -> None:
    """Create a file and every directory above it."""
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(text, encoding="utf-8")


def synthetic(
    root: Path, *, markup: str, explained: str, htmx_name: str = "htmx.min.js"
) -> None:
    """Build a minimal tree the gate can scan: a go.mod, a vendored scanner,
    one consumer package that embeds htmx, and an explained list."""
    write(root / "go.mod", "module synthetic\n")
    write(
        root / "third_party" / "web" / "htmx-upgrade-check.py",
        (REPO / "third_party" / "web" / "htmx-upgrade-check.py").read_text(
            encoding="utf-8"
        ),
    )
    write(root / "scripts" / "dev" / "htmx-upgrade-explained.txt", explained)
    write(
        root / "internal" / "component" / "demo" / "assets" / htmx_name,
        "// library bytes\n",
    )
    write(root / "internal" / "component" / "demo" / "page.html", markup)


def run(root: Path, *args: str) -> subprocess.CompletedProcess:
    """Run the gate against a tree and return the finished process."""
    return subprocess.run(
        [sys.executable, str(TOOL), "--root", str(root), *args],
        capture_output=True,
        text=True,
        check=False,
    )


# Markup with one issue the scanner reports: hx-ext is removed in htmx 4.
MARKUP_ONE_ISSUE = '<div hx-ext="sse" sse-connect="/events"></div>\n'

# Markup with no htmx attribute at all.
MARKUP_CLEAN = '<div class="panel">text</div>\n'


class TestPackageDerivation(unittest.TestCase):
    """htmx_packages names the package holding each htmx-bearing assets dir."""

    def test_finds_the_package_that_embeds_htmx(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            synthetic(root, markup=MARKUP_CLEAN, explained="")
            self.assertEqual(
                gate.htmx_packages(root),
                [os.path.join("internal", "component", "demo")],
            )

    def test_skips_an_assets_dir_with_no_htmx(self):
        """A consumer of another vendored library is not an htmx consumer.

        internal/component/api/rest/assets holds swagger-ui and no htmx, and it
        must stay outside the scan: its files would be judged against rules
        that govern markup it does not write.
        """
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            synthetic(root, markup=MARKUP_CLEAN, explained="")
            write(
                root / "internal" / "component" / "other" / "assets" / "swagger.js",
                "//\n",
            )
            self.assertEqual(
                gate.htmx_packages(root),
                [os.path.join("internal", "component", "demo")],
            )

    def test_the_live_tree_names_every_htmx_interface(self):
        """The real repository's three htmx interfaces are all in scope."""
        found = gate.htmx_packages(REPO)
        self.assertEqual(
            sorted(found),
            [
                os.path.join("internal", "chaos", "web"),
                os.path.join("internal", "component", "lg"),
                os.path.join("internal", "component", "web"),
            ],
        )


class TestGateVerdict(unittest.TestCase):
    """The exit code, driven as a subprocess."""

    def test_clean_tree_passes(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            synthetic(root, markup=MARKUP_CLEAN, explained="")
            done = run(root)
            self.assertEqual(done.returncode, 0, done.stdout + done.stderr)

    def test_an_unexplained_issue_fails(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            synthetic(root, markup=MARKUP_ONE_ISSUE, explained="")
            done = run(root)
            self.assertEqual(done.returncode, 1, done.stdout + done.stderr)
            self.assertIn("hx-ext", done.stdout)

    def test_an_explained_issue_passes(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            synthetic(
                root,
                markup=MARKUP_ONE_ISSUE,
                explained=(
                    "internal/component/demo/page.html | ext | fixture, never served\n"
                    "internal/component/demo/page.html | removed-attr | fixture, never served\n"
                ),
            )
            done = run(root)
            self.assertEqual(done.returncode, 0, done.stdout + done.stderr)

    def test_a_stale_row_fails(self):
        """An exemption that explains nothing is red, not silently ignored."""
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            synthetic(
                root,
                markup=MARKUP_CLEAN,
                explained="internal/component/demo/page.html | ext | fixture, never served\n",
            )
            done = run(root)
            self.assertEqual(done.returncode, 1, done.stdout + done.stderr)
            self.assertIn("STALE", done.stdout)

    def test_a_malformed_row_fails(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            synthetic(
                root,
                markup=MARKUP_CLEAN,
                explained="internal/component/demo/page.html\n",
            )
            done = run(root)
            self.assertNotEqual(done.returncode, 0)
            self.assertIn("every field must carry text", done.stdout + done.stderr)

    def test_no_htmx_consumer_fails_closed(self):
        """A run that scanned nothing must not report what a full run reports."""
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            synthetic(root, markup=MARKUP_CLEAN, explained="", htmx_name="uPlot.min.js")
            done = run(root)
            self.assertEqual(done.returncode, 1, done.stdout + done.stderr)
            self.assertIn("proved nothing", done.stderr)

    def test_a_missing_explained_list_fails_closed(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            synthetic(root, markup=MARKUP_CLEAN, explained="")
            (root / "scripts" / "dev" / "htmx-upgrade-explained.txt").unlink()
            done = run(root)
            self.assertNotEqual(done.returncode, 0)
            self.assertIn("is not an empty list", done.stdout + done.stderr)

    def test_a_missing_scanner_fails_closed(self):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            synthetic(root, markup=MARKUP_CLEAN, explained="")
            (root / "third_party" / "web" / "htmx-upgrade-check.py").unlink()
            done = run(root)
            self.assertNotEqual(done.returncode, 0)
            self.assertIn("re-vendor it", done.stdout + done.stderr)


class TestVendoredScanner(unittest.TestCase):
    """The vendored scanner keeps the interface this gate calls."""

    def test_the_scanner_exposes_what_the_gate_imports(self):
        module = gate.load_scanner(REPO)
        self.assertTrue(callable(module.check_file))
        self.assertTrue(callable(module.collect_files))
        self.assertIn(".js", module.DEFAULT_EXTENSIONS)

    def test_the_scanner_reads_the_inheritance_carriers(self):
        """The category a text search cannot produce is the reason to vendor it.

        `inheritance` is reported only when the scanner has built a DOM and
        found that a DESCENDANT issues a request. A grep for attribute names
        cannot reach that verdict, so its absence would mean the gate had
        quietly become a text search.
        """
        module = gate.load_scanner(REPO)
        with tempfile.TemporaryDirectory() as tmp:
            page = Path(tmp) / "page.html"
            page.write_text(
                '<div hx-target="#out"><button hx-post="/a">go</button></div>\n',
                encoding="utf-8",
            )
            categories = {issue.category for issue in module.check_file(str(page))}
        self.assertIn("inheritance", categories)


if __name__ == "__main__":
    unittest.main()
