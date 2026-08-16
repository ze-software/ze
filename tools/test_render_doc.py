#!/usr/bin/env -S uv run --with markdown python3

import hashlib
import json
import importlib.util
import pathlib
import sys
import tempfile
import unittest
from unittest import mock


HERE = pathlib.Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
SPEC = importlib.util.spec_from_file_location("render_doc", HERE / "render-doc.py")
render_doc = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(render_doc)
import sitelib  # noqa: E402
import sitefacts  # noqa: E402


def terminal_demo_fixture(root):
    demo_root = root / "terminal"
    demo_root.mkdir(parents=True)
    site_assets = root / "site-assets"
    site_assets.mkdir(parents=True)
    payloads = {
        "poster": ("demo.png", b"png-data"),
        "transcript": ("demo.txt", b"$ ze show version\n<safe output>\n"),
        "video": ("demo.webm", b"webm-data"),
    }
    assets = {}
    paths = {}
    for kind, (name, payload) in payloads.items():
        path = site_assets / name
        path.write_bytes(payload)
        paths[kind] = path
        assets[kind] = {
            "path": name,
            "bytes": len(payload),
            "sha256": hashlib.sha256(payload).hexdigest(),
        }

    (demo_root / "manifest.json").write_text(
        json.dumps(
            {
                "gallery_page": "guide/terminal-demonstrations.md",
                "schema": 2,
                "demos": [
                    {
                        "id": "demo",
                        "title": "Inspect live state",
                        "description": "Run a checked terminal workflow.",
                        "page": "guide/example.md",
                        "platform": "portable",
                        "kind": "terminal",
                        "engine": "VHS 0.11.0",
                        "duration": "12 seconds",
                    }
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
                    "demo": {
                        "release": "26.07.15",
                        "assets": assets,
                    }
                },
            }
        )
    )
    return demo_root, site_assets / "manifest.json", site_assets, paths


def patch_terminal_demo_paths(demo_root, artifact_manifest, site_assets):
    return mock.patch.multiple(
        render_doc.terminal_demos,
        DEMO_ROOT=demo_root,
        SOURCE_MANIFEST=demo_root / "manifest.json",
        ARTIFACT_MANIFEST=artifact_manifest,
        SITE_ASSET_ROOT=site_assets,
    )


class FrontMatterTest(unittest.TestCase):
    def test_scalar_front_matter_is_removed_from_body(self):
        metadata, body = render_doc.parse_front_matter(
            "---\ndescription: A page: with detail.\ntable-columns: false\n---\n# Body\n"
        )

        self.assertEqual(metadata["description"], "A page: with detail.")
        self.assertEqual(metadata["table-columns"], "false")
        self.assertEqual(body, "# Body\n")

    def test_invalid_boolean_is_rejected(self):
        with self.assertRaisesRegex(ValueError, "table-columns"):
            render_doc.metadata_bool(
                {"table-columns": "sometimes"}, "table-columns", True
            )


class RenderTest(unittest.TestCase):
    def test_front_matter_drives_shared_page_shell(self):
        source_text = """---
title: Metadata title
description: Metadata description.
category: services
journey: Feature
table-columns: false
---
# Visible heading

A rendered page.

| One | Two |
| --- | --- |
| 1 | 2 |
"""
        with tempfile.TemporaryDirectory() as tmp:
            tmp_path = pathlib.Path(tmp)
            source = tmp_path / "source.md"
            destination = tmp_path / "index.html"
            source.write_text(source_text)

            render_doc.render(source, destination, "../")

            html = destination.read_text()
            markdown = (tmp_path / "index.md").read_text()

        self.assertIn("<title>Metadata title - Ze</title>", html)
        self.assertIn('name="description" content="Metadata description."', html)
        self.assertIn('class="md-content reveal cat-services"', html)
        self.assertIn('data-table-columns="off"', html)
        self.assertIn('<span class="journey-eyebrow">Feature</span>', html)
        self.assertIn("<table>", html)
        self.assertEqual(markdown, source_text.split("---\n", 2)[2])


class TerminalDemoRenderTest(unittest.TestCase):
    def test_marker_embeds_local_video_and_accessible_transcript(self):
        with tempfile.TemporaryDirectory() as tmp:
            tmp_path = pathlib.Path(tmp)
            demo_root, artifact_manifest, site_assets, paths = terminal_demo_fixture(
                tmp_path
            )
            source = tmp_path / "source.md"
            destination = tmp_path / "public" / "index.html"
            source.write_text("# Example\n\n<!-- terminal-demo: demo -->\n")

            with patch_terminal_demo_paths(demo_root, artifact_manifest, site_assets):
                render_doc.render(
                    source,
                    destination,
                    "../",
                    doc_rel="guide/example.md",
                )

            rendered_html = destination.read_text()
            rendered_markdown = destination.with_suffix(".md").read_text()
            published_video = (site_assets / "demo.webm").read_bytes()
            source_video = paths["video"].read_bytes()

        video_version = hashlib.sha256(b"webm-data").hexdigest()[:10]
        poster_version = hashlib.sha256(b"png-data").hexdigest()[:10]
        transcript_version = hashlib.sha256(
            b"$ ze show version\n<safe output>\n"
        ).hexdigest()[:10]
        self.assertIn('data-terminal-demo="demo"', rendered_html)
        self.assertIn("<video controls", rendered_html)
        self.assertIn(
            'poster="../assets/demos/demo.png?v=%s"' % poster_version,
            rendered_html,
        )
        self.assertIn(
            'src="../assets/demos/demo.webm?v=%s"' % video_version,
            rendered_html,
        )
        self.assertIn("&lt;safe output&gt;", rendered_html)
        self.assertIn(
            'href="../assets/demos/demo.txt?v=%s"' % transcript_version,
            rendered_html,
        )
        self.assertIn("### Demo: Inspect live state", rendered_markdown)
        self.assertIn(
            "../assets/demos/demo.webm?v=%s" % video_version,
            rendered_markdown,
        )
        self.assertEqual(published_video, source_video)

    def test_gallery_page_can_embed_every_demo(self):
        with tempfile.TemporaryDirectory() as tmp:
            tmp_path = pathlib.Path(tmp)
            demo_root, artifact_manifest, site_assets, _ = terminal_demo_fixture(
                tmp_path
            )
            source = tmp_path / "source.md"
            destination = tmp_path / "public" / "index.html"
            source.write_text("# Demos\n\n<!-- terminal-demo: demo -->\n")

            with patch_terminal_demo_paths(demo_root, artifact_manifest, site_assets):
                render_doc.render(
                    source,
                    destination,
                    "../",
                    doc_rel="guide/terminal-demonstrations.md",
                )

            rendered_html = destination.read_text()

        self.assertIn('data-terminal-demo="demo"', rendered_html)

    def test_tampered_recording_is_rejected_before_publish(self):
        with tempfile.TemporaryDirectory() as tmp:
            tmp_path = pathlib.Path(tmp)
            demo_root, artifact_manifest, site_assets, paths = terminal_demo_fixture(
                tmp_path
            )
            paths["video"].write_bytes(b"not-video")
            source = tmp_path / "source.md"
            destination = tmp_path / "public" / "index.html"
            source.write_text("# Example\n\n<!-- terminal-demo: demo -->\n")

            with (
                patch_terminal_demo_paths(demo_root, artifact_manifest, site_assets),
                self.assertRaisesRegex(ValueError, "digest does not match"),
            ):
                render_doc.render(
                    source,
                    destination,
                    "../",
                    doc_rel="guide/example.md",
                )


class SiteChromeTest(unittest.TestCase):
    def test_github_badge_embeds_shared_build_fact(self):
        original_path = sitefacts.FACTS_PATH
        try:
            with tempfile.TemporaryDirectory() as tmp:
                sitefacts.FACTS_PATH = pathlib.Path(tmp) / "site-facts.json"
                sitefacts.FACTS_PATH.write_text(json.dumps({"github_stars": 46}))
                sitelib.reset_nav_data_cache()

                badge = sitelib.build_nav_badges("../../../")

            self.assertIn('aria-label="Ze on GitHub, 46 stars"', badge)
            self.assertNotIn("data-github-stars", badge)
            self.assertNotIn("data-stars-url", badge)
        finally:
            sitefacts.FACTS_PATH = original_path
            sitelib.reset_nav_data_cache()

    def test_page_shell_mounts_external_shared_header(self):
        page = sitelib.page_head("Title", "Description", "../../../")
        sitelib.page_foot("../../../")

        self.assertIn('id="site-header-mount"', page)
        self.assertIn(
            'data-header-src="../../../assets/header.html"',
            page,
        )
        self.assertNotIn('class="nav-dropdown"', page)

    def test_embedded_header_migrates_to_idempotent_mount(self):
        legacy = (
            "<body>\n"
            '        <header class="site-header"><nav>old menu</nav></header>\n'
            '        <main id="top"></main>\n'
            "</body>\n"
        )

        migrated = sitelib.patch_shared_header(legacy, "../")
        repeated = sitelib.patch_shared_header(migrated, "../")

        self.assertEqual(migrated, repeated)
        self.assertNotIn("old menu", migrated)
        self.assertIn(
            'data-header-src="../assets/header.html" data-site-root="../"',
            migrated,
        )

    def test_shared_header_fragment_contains_navigation(self):
        fragment = sitelib.build_shared_header(sitelib.SHARED_HEADER_ROOT_TOKEN)

        self.assertIn('id="site-nav-links"', fragment)
        self.assertIn("Docs", fragment)
        self.assertIn(sitelib.SHARED_HEADER_ROOT_TOKEN + "docs/", fragment)

    def test_navigation_stays_scannable_with_one_owner_per_destination(self):
        data = json.loads((HERE.parent / "data" / "nav.json").read_text())
        owners = {}

        for dropdown in data["dropdowns"]:
            columns = dropdown["columns"]
            self.assertLessEqual(len(columns), 2, dropdown["label"])
            links = [entry for column in columns for entry in column if "href" in entry]
            self.assertLessEqual(len(links), 12, dropdown["label"])
            for entry in links:
                href = entry["href"]
                self.assertNotIn(
                    href,
                    owners,
                    "%s appears in both %s and %s"
                    % (href, owners.get(href), dropdown["label"]),
                )
                owners[href] = dropdown["label"]

    def test_star_fetch_failure_keeps_last_published_value(self):
        original_path = sitefacts.FACTS_PATH
        original_cache = sitefacts._github_stars_cache
        try:
            with tempfile.TemporaryDirectory() as tmp:
                sitefacts.FACTS_PATH = pathlib.Path(tmp) / "site-facts.json"
                sitefacts.FACTS_PATH.write_text(json.dumps({"github_stars": 46}))
                sitefacts._github_stars_cache = None
                with mock.patch.object(
                    sitefacts.urllib.request,
                    "urlopen",
                    side_effect=OSError("offline"),
                ):
                    stars = sitefacts.github_stars()

            self.assertEqual(stars, 46)
        finally:
            sitefacts.FACTS_PATH = original_path
            sitefacts._github_stars_cache = original_cache


if __name__ == "__main__":
    unittest.main()
