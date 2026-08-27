#!/usr/bin/env python3

import importlib.util
import pathlib
import unittest


HERE = pathlib.Path(__file__).resolve().parent
SPEC = importlib.util.spec_from_file_location(
    "gen_wiki_commands", HERE / "gen_wiki_commands.py"
)
gen_wiki_commands = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(gen_wiki_commands)


class CommandWikiGeneratorTest(unittest.TestCase):
    def test_render_detail_preserves_operator_availability_and_surface(self):
        lines = gen_wiki_commands.render_detail(
            {
                "path": "monitor test",
                "mode": "read-only",
                "answer-shape": "tab",
                "address-fields": ["address"],
                "operators": [
                    {"name": "json", "available": "always"},
                    {"name": "match", "available": "with-rows"},
                    {"name": "log", "available": "when-streaming"},
                    {
                        "name": "save",
                        "available": "always",
                        "local-only": True,
                    },
                ],
            }
        )
        self.assertEqual(
            [
                "",
                "Mode: read-only",
                "Answer shape: `tab`",
                "Address fields: `address`",
                "",
                "**Pipes:**",
                "Always: `json`, `save`",
                "",
                "When the answer has rows: `match` -- this command has not declared its answer shape, so each of these applies to an answer that carries rows and is refused by name over one that does not.",
                "",
                "While the command keeps answering: `log`",
                "",
                "Local process only: `save` -- daemon-expanded SSH and web chains refuse these operators.",
            ],
            lines,
        )


if __name__ == "__main__":
    unittest.main()
