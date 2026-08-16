#!/usr/bin/env -S uv run python3

import importlib.util
import pathlib
import sys
import tempfile
import unittest
from unittest import mock


HERE = pathlib.Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
SPEC = importlib.util.spec_from_file_location(
    "render_llms_txt", HERE / "render-llms-txt.py"
)
render_llms = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(render_llms)
import page_registry  # noqa: E402


class PublishedDocumentationTest(unittest.TestCase):
    def summary_for(self, content):
        with tempfile.TemporaryDirectory() as raw_root:
            path = pathlib.Path(raw_root) / "source.md"
            path.write_text(content)
            return render_llms.markdown_title_and_summary(path)

    def test_summary_skips_yaml_frontmatter(self):
        title, summary = self.summary_for(
            "---\n"
            "title: Terminal Demonstrations\n"
            "description: Recorded operator workflows.\n"
            "---\n\n"
            "# Terminal Demonstrations\n\n"
            "Watch reproducible command and configuration workflows in the browser.\n"
        )

        self.assertEqual("Terminal Demonstrations", title)
        self.assertEqual(
            "Watch reproducible command and configuration workflows in the browser.",
            summary,
        )

    def test_summary_keeps_bold_leading_paragraph(self):
        title, summary = self.summary_for(
            "# VPP Data Plane\n\n"
            "**Status:** The component manages startup,\n"
            "crash recovery, and production telemetry collection.\n"
        )

        self.assertEqual("VPP Data Plane", title)
        self.assertEqual(
            "Status: The component manages startup, crash recovery, and production telemetry collection.",
            summary,
        )

    def test_summary_falls_back_to_first_list_item(self):
        title, summary = self.summary_for(
            "# ExaBGP Compatibility\n\n"
            "- Automatic detection and migration of ExaBGP configuration files\n"
            "- Bidirectional command and event translation\n"
        )

        self.assertEqual("ExaBGP Compatibility", title)
        self.assertEqual(
            "Automatic detection and migration of ExaBGP configuration files",
            summary,
        )

    def test_complete_index_includes_registered_docs_and_use_cases(self):
        with tempfile.TemporaryDirectory() as raw_root:
            root = pathlib.Path(raw_root)
            gh_pages = root / "gh-pages"
            main_docs = root / "main" / "docs" / "guide"
            use_case = gh_pages / "use-cases" / "example"
            main_docs.mkdir(parents=True)
            use_case.mkdir(parents=True)
            (main_docs / "new-guide.md").write_text(
                "# New Guide\n\nOperator workflow from the canonical docs source.\n"
            )
            (use_case / "index.md").write_text(
                "# Worked Example\n\nA complete deployment example.\n"
            )

            use_case_pages = [
                page_registry.MarkdownPage(
                    "example/index.md",
                    "use-cases/example/index.html",
                    "registered description",
                    "routing",
                )
            ]
            with (
                mock.patch.object(render_llms, "GH_PAGES", gh_pages),
                mock.patch.object(
                    page_registry,
                    "DOCS_MANIFEST",
                    {"guide/new-guide.md": "routing"},
                ),
                mock.patch.object(page_registry, "USE_CASE_PAGES", use_case_pages),
            ):
                text = render_llms.render_published_documentation()

            self.assertIn(
                "[New Guide](https://ze-software.net/guides/new-guide/index.md)",
                text,
            )
            self.assertIn("Operator workflow from the canonical docs source.", text)
            self.assertIn(
                "[Worked Example](https://ze-software.net/use-cases/example/index.md)",
                text,
            )
            self.assertIn("A complete deployment example.", text)


if __name__ == "__main__":
    unittest.main()
