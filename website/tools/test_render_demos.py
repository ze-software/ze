#!/usr/bin/env -S uv run --with markdown python3
"""Site-side tests for the terminal demo embed.

A `kind: terminal` demo publishes an asciicast, so its page carries a player
mount rather than a `<video>`; the `kind: browser` demo keeps the video it has
always had. Both arms are driven through `render-doc.render`, the one call site
of `terminal_demos.expand`, so what these tests read is the published page and
its `index.md` sibling rather than a helper's return value.
"""

import hashlib
import importlib.util
import json
import pathlib
import re
import sys
import tempfile
import unittest
from unittest import mock


HERE = pathlib.Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))

import sitepaths  # noqa: E402  (needs HERE on sys.path)

SPEC = importlib.util.spec_from_file_location("render_doc", HERE / "render-doc.py")
render_doc = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(render_doc)

# 80 columns by 24 rows, with a last event at 95 seconds. The source manifest
# below states "12 seconds" for the same demo, so a page that prints 95 seconds
# can only have read the artifact.
CAST = (
    '{"version": 2, "width": 80, "height": 24}\n'
    '[0.5, "o", "$ ze show version\\r\\n"]\n'
    '[95.0, "o", "ze 26.07.15\\r\\n"]\n'
).encode()
TRANSCRIPT = b"$ ze show version\n<safe output>\n"
POSTER = b"png-data"
VIDEO = b"webm-data"


def version(payload):
    return hashlib.sha256(payload).hexdigest()[:10]


def demo_fixture(root):
    """Two demos: one terminal (cast plus transcript), one browser (the triple)."""
    demo_root = root / "terminal"
    demo_root.mkdir(parents=True)
    site_assets = root / "site-assets"
    site_assets.mkdir(parents=True)

    payloads = {
        "terminal.cast": CAST,
        "terminal.txt": TRANSCRIPT,
        "browser.png": POSTER,
        "browser.txt": TRANSCRIPT,
        "browser.webm": VIDEO,
    }
    for name, payload in payloads.items():
        (site_assets / name).write_bytes(payload)

    def asset(name):
        payload = payloads[name]
        return {
            "path": name,
            "bytes": len(payload),
            "sha256": hashlib.sha256(payload).hexdigest(),
        }

    (demo_root / "manifest.json").write_text(
        json.dumps(
            {
                "gallery-page": "guide/terminal-demonstrations.md",
                "schema": 2,
                "demos": [
                    {
                        "id": "terminal",
                        "title": "Inspect live state",
                        "description": "Run a checked terminal workflow.",
                        "page": "guide/example.md",
                        "platform": "portable",
                        "kind": "terminal",
                        "engine": "pty-session",
                        "duration": "12 seconds",
                    },
                    {
                        "id": "browser",
                        "title": "Commit from the web",
                        "description": "Run a checked browser workflow.",
                        "page": "guide/browser.md",
                        "platform": "portable",
                        "kind": "browser",
                        "engine": "Playwright 1.55.0",
                        "duration": "58 seconds",
                    },
                ],
            }
        )
    )
    (site_assets / "manifest.json").write_text(
        json.dumps(
            {
                "schema": 2,
                "renderer": {
                    "image": "test/ze-demo:latest",
                    "name": "ze-demo",
                    "platform": "linux/native",
                    "version": "2",
                },
                "demos": {
                    "terminal": {
                        "release": "26.07.15",
                        "assets": {
                            "cast": asset("terminal.cast"),
                            "transcript": asset("terminal.txt"),
                        },
                    },
                    "browser": {
                        "release": "26.07.15",
                        "assets": {
                            "poster": asset("browser.png"),
                            "transcript": asset("browser.txt"),
                            "video": asset("browser.webm"),
                        },
                    },
                },
            }
        )
    )
    return demo_root, site_assets / "manifest.json", site_assets


def patched_paths(demo_root, artifact_manifest, site_assets):
    return mock.patch.multiple(
        render_doc.terminal_demos,
        DEMO_ROOT=demo_root,
        SOURCE_MANIFEST=demo_root / "manifest.json",
        ARTIFACT_MANIFEST=artifact_manifest,
        SITE_ASSET_ROOT=site_assets,
    )


def rendered(demo_id, doc_rel):
    """Render a one-marker page; return its HTML and its Markdown sibling."""
    with tempfile.TemporaryDirectory() as tmp:
        tmp_path = pathlib.Path(tmp)
        demo_root, artifact_manifest, site_assets = demo_fixture(tmp_path)
        source = tmp_path / "source.md"
        destination = tmp_path / "public" / "index.html"
        source.write_text("# Example\n\n<!-- terminal-demo: %s -->\n" % demo_id)

        with patched_paths(demo_root, artifact_manifest, site_assets):
            render_doc.render(source, destination, "../", doc_rel=doc_rel)

        return destination.read_text(), destination.with_suffix(".md").read_text()


class TerminalDemoPlayerTest(unittest.TestCase):
    def test_render_demo_page_embeds_player(self):
        """AC-8: a player mount bound to the cast, served from this site."""
        page, _ = rendered("terminal", "guide/example.md")

        self.assertIn('data-terminal-demo="terminal"', page)
        self.assertIn(
            'data-cast-src="../assets/demos/terminal.cast?v=%s"' % version(CAST),
            page,
        )
        self.assertIn('class="terminal-demo__player"', page)
        self.assertNotIn("<video", page)
        self.assertNotIn(".webm", page)
        self.assertIn("../assets/vendor/asciinema-player.min.js", page)
        self.assertIn("../assets/vendor/asciinema-player.css", page)
        self.assertNotIn("asciinema.org", page)
        self.assertNotIn("cdn.jsdelivr.net", page)
        self.assertNotIn("unpkg.com", page)

    def test_render_demo_page_reserves_player_box(self):
        """AC-4, R-2: the box comes from the cast's own grid, before load."""
        page, _ = rendered("terminal", "guide/example.md")

        self.assertIn('data-cols="80"', page)
        self.assertIn('data-rows="24"', page)
        # 80 columns of 0.6 em advance by 24 rows of 1.32 em line height.
        self.assertIn("--demo-aspect: 48 / 31.68", page)

    def test_markdown_sibling_links_the_cast(self):
        """AC-9, R-4: the indexed Markdown offers the cast, not a WebM."""
        page, markdown = rendered("terminal", "guide/example.md")

        self.assertIn("### Demo: Inspect live state", markdown)
        self.assertIn("../assets/demos/terminal.cast?v=%s" % version(CAST), markdown)
        self.assertIn(
            "../assets/demos/terminal.txt?v=%s" % version(TRANSCRIPT), markdown
        )
        self.assertNotIn("WebM", markdown)
        self.assertNotIn(".webm", markdown)
        self.assertNotIn(".png", markdown)
        self.assertIn("$ ze show version", markdown)
        self.assertIn("$ ze show version", page)

    def test_duration_is_read_from_the_cast(self):
        """The published length is the artifact's, so an engine swap cannot
        leave the page stating a duration the recording does not have."""
        page, markdown = rendered("terminal", "guide/example.md")

        self.assertIn("1 minute 35 seconds", page)
        self.assertIn("Duration: 1 minute 35 seconds.", markdown)
        self.assertNotIn("12 seconds", page)
        self.assertNotIn("12 seconds", markdown)

    def test_browser_demo_keeps_its_video(self):
        """D-3, AC-7: the browser demo is untouched by the conversion."""
        page, markdown = rendered("browser", "guide/browser.md")

        self.assertIn("<video controls", page)
        self.assertIn('src="../assets/demos/browser.webm?v=%s"' % version(VIDEO), page)
        self.assertIn(
            'poster="../assets/demos/browser.png?v=%s"' % version(POSTER), page
        )
        self.assertIn("WEBM", page)
        self.assertNotIn("terminal-demo__player", page)
        self.assertIn("[Play the WebM recording]", markdown)
        # A video carries its own length; only a cast states one we can read.
        self.assertIn("58 seconds", page)


class HeroMountTest(unittest.TestCase):
    def test_hero_reads_the_manifest_rather_than_hardcoded_digests(self):
        """The homepage hero is bound to the same artifact every page reads."""
        with tempfile.TemporaryDirectory() as tmp:
            tmp_path = pathlib.Path(tmp)
            demo_root, artifact_manifest, site_assets = demo_fixture(tmp_path)
            with patched_paths(demo_root, artifact_manifest, site_assets):
                mount = render_doc.terminal_demos.hero_mount(
                    "terminal", "", "Live dashboard demonstration"
                )

        self.assertIn(
            'data-cast-src="assets/demos/terminal.cast?v=%s"' % version(CAST), mount
        )
        self.assertIn("--demo-aspect: 48 / 31.68", mount)
        self.assertNotIn(".webm", mount)

    def test_hero_refuses_a_demo_that_is_not_a_terminal_recording(self):
        with tempfile.TemporaryDirectory() as tmp:
            tmp_path = pathlib.Path(tmp)
            demo_root, artifact_manifest, site_assets = demo_fixture(tmp_path)
            with (
                patched_paths(demo_root, artifact_manifest, site_assets),
                self.assertRaisesRegex(ValueError, "not a terminal demo"),
            ):
                render_doc.terminal_demos.hero_mount("browser", "", "label")


class SelfHostedFontTest(unittest.TestCase):
    """D-2: the faces come from this site, so a reader who blocks Google, or
    reads the site offline, sees the same page as everyone else."""

    def test_no_external_font_reference(self):
        """AC-10: a published page requests no font from Google."""
        page, _ = rendered("terminal", "guide/example.md")

        self.assertNotIn("fonts.googleapis.com", page)
        self.assertNotIn("fonts.gstatic.com", page)
        self.assertIn('href="../assets/vendor/fonts/fonts.css"', page)

    def test_font_faces_resolve_to_published_files(self):
        """A `@font-face` naming a pruned or missing path renders a page that
        falls back to a system font in silence, and the markup test above still
        passes. So read the stylesheet the page links, and check both halves:
        every file it names exists, and its directory reaches the published
        tree."""
        stylesheet = HERE.parent / "assets" / "vendor" / "fonts" / "fonts.css"
        text = stylesheet.read_text()
        faces = re.findall(r"@font-face\s*\{(.*?)\}", text, re.S)
        self.assertEqual(len(faces), 10)

        weights = set()
        for face in faces:
            family = re.search(r'font-family:\s*"([^"]+)"', face).group(1)
            weight = re.search(r"font-weight:\s*(\d+)", face).group(1)
            weights.add((family, weight))
            # display=swap was in the Google URL; without it first paint blocks
            # on the download instead of showing the fallback face.
            self.assertIn("font-display: swap", face)
            name = re.search(r'url\("([^"]+)"\)', face).group(1)
            self.assertTrue(
                (stylesheet.parent / name).is_file(),
                "%s names a file that does not exist: %s" % (stylesheet, name),
            )

        self.assertEqual(
            weights,
            {
                ("Poppins", "400"),
                ("Poppins", "700"),
                ("Poppins", "800"),
                ("Lato", "400"),
                ("Lato", "700"),
            },
        )

        rel = stylesheet.parent.relative_to(HERE.parent).as_posix()
        self.assertFalse(
            sitepaths.is_source_only(rel),
            "%s is pruned from the published tree, so the faces never ship" % rel,
        )


if __name__ == "__main__":
    unittest.main()
