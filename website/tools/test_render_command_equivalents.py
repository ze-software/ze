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

        self.assertEqual(["json"], always)
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

        self.assertIn("Pipes, while streaming", rendered)
        self.assertIn("Pipes, local process only", rendered)
        self.assertIn("Answer shape", rendered)
        self.assertIn("Address fields", rendered)


if __name__ == "__main__":
    unittest.main()
