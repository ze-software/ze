#!/usr/bin/env -S uv run --with markdown python3

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
import sitelib
import sitefacts


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
