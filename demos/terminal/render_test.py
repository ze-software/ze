#!/usr/bin/env python3
"""Terminal demo render freshness tests.

Design: the two cases below cover what `definition_digest` IGNORES, which is what
decides whether a demo must be re-rendered. One proves the digest does not move for
a file that cannot change the recording. The other proves the freshness check reads
the definition digest rather than the source digest. `test_render.py` covers the
other direction, calling `definition_digest` and `verify_assets(...,
definition_only=True)` over an asset set that matches.
"""

import importlib.util
import pathlib
import tempfile
import unittest


HERE = pathlib.Path(__file__).resolve().parent


def load_render():
    """Load render.py as a fresh module.

    Each case rewrites the module's path globals, so each case gets its own
    module object and no case inherits the tree of another one.
    """
    spec = importlib.util.spec_from_file_location("terminal_render", HERE / "render.py")
    module = importlib.util.module_from_spec(spec)
    assert spec.loader is not None
    spec.loader.exec_module(module)
    return module


def write_demo_tree(tmp_path):
    render = load_render()
    root = tmp_path / "main"
    demo_root = root / "demos" / "terminal"
    demo_dir = demo_root / "sample"
    demo_dir.mkdir(parents=True)
    (demo_root / "common.tape").write_text("Set Shell bash\n", encoding="utf-8")
    (demo_dir / "demo.tape").write_text(
        'Output artifacts/sample.webm\nSource common.tape\nType "show"\n',
        encoding="utf-8",
    )
    (demo_dir / "validate.sh").write_text("echo validate\n", encoding="utf-8")
    render.ROOT = root
    render.DEMO_ROOT = demo_root
    render.SHARED_SOURCE_PATHS = (demo_root / "common.tape", demo_root / "render.py")
    return (
        render,
        {
            "id": "sample",
            "kind": "terminal",
            "source": "sample/demo.tape",
            "validate": "sample/validate.sh",
        },
        demo_dir,
    )


class DefinitionDigestTest(unittest.TestCase):
    """Cover what `definition_digest` includes and what it leaves out."""

    def setUp(self):
        tmp = tempfile.TemporaryDirectory(prefix="ze-terminal-render-")
        self.addCleanup(tmp.cleanup)
        self.tmp_path = pathlib.Path(tmp.name)

    def test_definition_digest_changes_only_for_vhs_definition(self):
        render, demo, demo_dir = write_demo_tree(self.tmp_path)

        before = render.definition_digest(demo)
        (demo_dir / "validate.sh").write_text(
            "echo validate changed\n", encoding="utf-8"
        )
        self.assertEqual(render.definition_digest(demo), before)

        (demo_dir / "demo.tape").write_text(
            'Output artifacts/sample.webm\nSource common.tape\nType "show bgp"\n',
            encoding="utf-8",
        )
        self.assertNotEqual(render.definition_digest(demo), before)

    def test_definition_check_ignores_non_vhs_source_digest(self):
        # Phase 4 of the website-asciinema-terminal-demos spec makes
        # verify_assets kind-aware, and a `kind: terminal` demo that carries a
        # video asset MUST be refused once that lands. The fixture below builds
        # exactly that demo, so this case goes red when phase 4 arrives. That red
        # is the new rule working. Give the demo the terminal asset set instead
        # of video, poster and transcript, and keep the assertion this case
        # makes, which is that a stale source_sha256 is not read under
        # definition_only.
        render, demo, _ = write_demo_tree(self.tmp_path)
        artifact_root = self.tmp_path / "gh-pages" / "assets" / "demos"
        artifact_root.mkdir(parents=True)
        assets = {}
        for name, content in {
            "video": b"webm",
            "poster": b"png",
            "transcript": b"text",
        }.items():
            path = artifact_root / f"sample.{name}"
            path.write_bytes(content)
            assets[name] = {
                "path": path.name,
                "bytes": path.stat().st_size,
                "sha256": render.sha256(path),
            }
        render.ARTIFACT_ROOT = artifact_root
        render.ARTIFACT_MANIFEST_PATH = artifact_root / "manifest.json"
        manifest = {"schema": 2, "renderer": {"name": "test"}}
        render.ARTIFACT_MANIFEST_PATH.write_text(
            render.json.dumps(
                {
                    "schema": 2,
                    "renderer": manifest["renderer"],
                    "demos": {
                        "sample": {
                            "release": "old",
                            "source_sha256": "stale-source",
                            "definition_sha256": render.definition_digest(demo),
                            "assets": assets,
                        }
                    },
                }
            )
            + "\n",
            encoding="utf-8",
        )

        # The assertion is that this call returns rather than raises. A stale
        # source_sha256 must not be read when definition_only is set.
        render.verify_assets(
            manifest, {"sample": demo}, ["sample"], None, definition_only=True
        )


if __name__ == "__main__":
    unittest.main()
