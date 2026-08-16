#!/usr/bin/env -S uv run --with pytest python3
"""Contracts separating tracked website sources from the Pages artifact."""

import os
import pathlib
import subprocess
import sys

HERE = pathlib.Path(__file__).resolve().parent
SOURCE_ROOT = HERE.parent
sys.path.insert(0, str(HERE))

import page_registry  # noqa: E402
import sitepaths  # noqa: E402


GENERATED_FILES = {
    "assets/header.html",
    "assets/site.css",
    "assets/site.js",
    "blog/feed.xml",
    "changes/feed.xml",
    "data/changes.json",
    "data/cli-commands.json",
    "data/plugin-registry.json",
    "data/rfc-compliance.json",
    "data/search-index.json",
    "data/site-facts.json",
    "data/yang-config-tree.json",
    "docs/guide/chaos-testing/img/ze-chaos-dashboard.png",
    "guides/chaos-testing/img/ze-chaos-dashboard.png",
    "index.html",
    "llms.txt",
    "robots.txt",
    "sitemap.xml",
    "talks/linx-2026-06/activity.html",
    "talks/linx-2026-06/index-inlined.html",
    "talks/netmcr-2026-04/index-inlined.html",
}

AUTHORED_HTML = set(page_registry.NAV_PATCH_TARGETS) | {
    "talks/linx-2026-06/index.html",
    "talks/netmcr-2026-04/index.html",
}

AUTHORED_INDEX_MARKDOWN = {page.source for page in page_registry.HUB_PAGES} | {
    "use-cases/%s" % page.source for page in page_registry.USE_CASE_PAGES
}


def tracked_files():
    output = subprocess.check_output(["git", "ls-files"], cwd=SOURCE_ROOT, text=True)
    return {path for path in output.splitlines() if (SOURCE_ROOT / path).is_file()}


def test_default_roots_keep_generated_output_outside_sources():
    assert sitepaths.SOURCE_ROOT == SOURCE_ROOT.resolve()
    assert sitepaths.OUTPUT_ROOT == (SOURCE_ROOT.parent / "_site").resolve()
    assert sitepaths.MAIN_REPO == (SOURCE_ROOT.parent / "main").resolve()
    assert sitepaths.OUTPUT_ROOT != sitepaths.SOURCE_ROOT


def test_environment_can_select_output_root(tmp_path):
    output = tmp_path / "_site"
    main = tmp_path / "main"
    env = os.environ | {
        "ZE_SITE_OUTPUT": str(output),
    }
    command = [
        sys.executable,
        "-c",
        "import sitepaths; print(sitepaths.OUTPUT_ROOT); print(sitepaths.MAIN_REPO)",
    ]
    result = subprocess.run(
        command,
        cwd=HERE,
        env=env,
        check=True,
        text=True,
        capture_output=True,
    )
    assert result.stdout.splitlines() == [str(output.resolve()), str(main.resolve())]


def test_generated_website_files_are_not_tracked():
    tracked = tracked_files()
    assert not (GENERATED_FILES & tracked)
    assert {path for path in tracked if path.endswith(".html")} == AUTHORED_HTML
    assert {
        path for path in tracked if path.endswith("index.md")
    } == AUTHORED_INDEX_MARKDOWN


def test_build_control_files_are_not_publishable():
    for path in (
        ".github",
        ".claude",
        ".gitignore",
        "AI.md",
        "CACHEDIR.TAG",
        "tools",
        "update-website.sh",
    ):
        assert sitepaths.is_source_only(path)

    for path in (
        ".nojekyll",
        "CNAME",
        "assets/ze.svg",
        "talks/linx-2026-06/slides.md",
    ):
        assert not sitepaths.is_source_only(path)
