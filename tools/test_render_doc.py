#!/usr/bin/env -S uv run --with markdown python3

import importlib.util
import pathlib
import sys
import tempfile
import unittest


HERE = pathlib.Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
SPEC = importlib.util.spec_from_file_location("render_doc", HERE / "render-doc.py")
render_doc = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(render_doc)


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


if __name__ == "__main__":
    unittest.main()
