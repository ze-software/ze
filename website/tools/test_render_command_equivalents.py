#!/usr/bin/env -S uv run python3

import importlib.util
import pathlib
import sys
import unittest


HERE = pathlib.Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
SPEC = importlib.util.spec_from_file_location(
    "render_command_equivalents", HERE / "render-command-equivalents.py"
)
render_command_equivalents = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(render_command_equivalents)


class CommandEquivalentPipeContractTest(unittest.TestCase):
    def test_split_operators_preserves_all_availability_classes_and_surface(self):
        command = {
            "operators": [
                {"name": "json", "available": "always"},
                {"name": "match", "available": "with-rows"},
                {"name": "log", "available": "when-streaming"},
                {"name": "save", "available": "always", "local-only": True},
            ]
        }

        always, with_rows, streaming, local_only = (
            render_command_equivalents.split_operators(command)
        )

        self.assertEqual(["json", "save"], always)
        self.assertEqual(["match"], with_rows)
        self.assertEqual(["log"], streaming)
        self.assertEqual(["save"], local_only)

    def test_detail_labels_streaming_and_local_only_operators(self):
        rendered = render_command_equivalents.render_ze_detail(
            {
                "path": "monitor test",
                "mode": "read-only",
                "description": "Monitor test rows.",
                "answer-shape": "tab",
                "address-fields": ["address"],
                "operators": [
                    {
                        "name": "log",
                        "available": "when-streaming",
                    },
                    {
                        "name": "save",
                        "available": "always",
                        "local-only": True,
                    },
                ],
            }
        )

        self.assertIn("<div><dt>Pipes, always</dt><dd>save</dd></div>", rendered)
        self.assertIn("<div><dt>Pipes, while streaming</dt><dd>log</dd></div>", rendered)
        self.assertIn("<div><dt>Pipes, local process only</dt><dd>save</dd></div>", rendered)
        self.assertIn("<div><dt>Answer shape</dt><dd>tab</dd></div>", rendered)
        self.assertIn("<div><dt>Address fields</dt><dd>address</dd></div>", rendered)
        row = {
            "command": {
                "path": "monitor test",
                "mode": "read-only",
                "description": "Monitor test rows.",
                "operators": [
                    {"name": "save", "available": "always", "local-only": True}
                ],
            },
            "entries": [],
        }
        markdown = render_command_equivalents.render_detail_markdown(row, {}, [], {})
        self.assertIn("- Pipes, always: save\n", markdown)
        self.assertIn("- Pipes, local process only: save\n", markdown)

    def test_html_and_markdown_details_render_command_pipes_and_aliases_exactly(self):
        command = {
            "path": "show bgp rib",
            "mode": "read-only",
            "description": "Show routes.",
            "pipes": [
                {
                    "name": "family",
                    "takes-arg": True,
                    "description": "Filter by family",
                }
            ],
            "pipe-aliases": [
                {
                    "name": "summary",
                    "description": "Aggregate fields",
                    "expansion": "display address",
                }
            ],
        }

        self.assertEqual(
            "\n".join(
                [
                    '<article class="cmd-detail-card cmd-detail-ze"><h2>Ze command</h2>',
                    '<dl class="cmd-meta">',
                    '<div><dt>Syntax</dt><dd><code>show bgp rib</code></dd></div>',
                    '<div><dt>Registry path</dt><dd><code>show bgp rib</code></dd></div>',
                    '<div><dt>Mode</dt><dd>Read-only</dd></div>',
                    '<div><dt>Wire method</dt><dd><code>not listed</code></dd></div>',
                    '<div><dt>Command pipes</dt><dd><code>family &lt;value&gt;</code>: Filter by family</dd></div>',
                    '<div><dt>Pipe aliases</dt><dd><code>summary</code>: Aggregate fields (<code>display address</code>)</dd></div>',
                    "</dl>",
                    "<h3>Description</h3><p>Show routes.</p>",
                    "<h3>Arguments</h3>",
                    "<p>No command-specific arguments listed.</p>",
                    "</article>",
                ]
            ),
            render_command_equivalents.render_ze_detail(command),
        )
        row = {"command": command, "entries": []}
        self.assertEqual(
            "\n".join(
                [
                    "# `show bgp rib`",
                    "",
                    "## Ze command",
                    "",
                    "- Syntax: `show bgp rib`",
                    "- Registry path: `show bgp rib`",
                    "- Mode: Read-only",
                    "- Wire method: `not listed`",
                    "- Answer shape: not declared",
                    "- Address fields: none",
                    "- Pipes, always: none",
                    "- Pipes, on rows: none",
                    "- Pipes, while streaming: none",
                    "- Pipes, local process only: none",
                    "- Command pipes: `family <value>`: Filter by family",
                    "- Pipe aliases: `summary`: Aggregate fields (`display address`)",
                    "",
                    "Show routes.",
                    "",
                    "## Mapping intents",
                    "",
                    "No vendor equivalent has been curated yet for this Ze command.",
                    "## Vendor equivalents",
                    "",
                ]
            ),
            render_command_equivalents.render_detail_markdown(row, {}, [], {}),
        )


if __name__ == "__main__":
    unittest.main()
