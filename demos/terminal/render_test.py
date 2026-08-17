#!/usr/bin/env -S uv run --with pytest python3
"""Terminal demo render freshness tests."""

import importlib.util
import pathlib
import sys


HERE = pathlib.Path(__file__).resolve().parent


def load_render():
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
        "Output artifacts/sample.webm\nSource common.tape\nType \"show\"\n",
        encoding="utf-8",
    )
    (demo_dir / "validate.sh").write_text("echo validate\n", encoding="utf-8")
    render.ROOT = root
    render.DEMO_ROOT = demo_root
    render.SHARED_SOURCE_PATHS = (demo_root / "common.tape", demo_root / "render.py")
    return render, {"id": "sample", "kind": "terminal", "source": "sample/demo.tape", "validate": "sample/validate.sh"}, demo_dir


def test_definition_digest_changes_only_for_vhs_definition(tmp_path):
    render, demo, demo_dir = write_demo_tree(tmp_path)

    before = render.definition_digest(demo)
    (demo_dir / "validate.sh").write_text("echo validate changed\n", encoding="utf-8")
    assert render.definition_digest(demo) == before

    (demo_dir / "demo.tape").write_text(
        "Output artifacts/sample.webm\nSource common.tape\nType \"show bgp\"\n",
        encoding="utf-8",
    )
    assert render.definition_digest(demo) != before


def test_definition_check_ignores_non_vhs_source_digest(tmp_path):
    render, demo, _ = write_demo_tree(tmp_path)
    artifact_root = tmp_path / "gh-pages" / "assets" / "demos"
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

    render.verify_assets(manifest, {"sample": demo}, ["sample"], None, definition_only=True)
