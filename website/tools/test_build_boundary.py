#!/usr/bin/env -S uv run --with pytest python3
"""Contracts separating tracked website sources from the Pages artifact."""

import importlib.util
import os
import pathlib
import subprocess
import sys

HERE = pathlib.Path(__file__).resolve().parent
SOURCE_ROOT = HERE.parent
sys.path.insert(0, str(HERE))

import page_registry  # noqa: E402
import sitepaths  # noqa: E402


def load_build_site():
    spec = importlib.util.spec_from_file_location("build_site", HERE / "build-site.py")
    module = importlib.util.module_from_spec(spec)
    assert spec.loader is not None
    spec.loader.exec_module(module)
    return module


GENERATED_FILES = {
    "assets/header.html",
    "assets/site.css",
    "assets/site.js",
    "blog/feed.xml",
    "project/changes/feed.xml",
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
    output = subprocess.check_output(
        ["git", "ls-files", "--cached", "--others", "--exclude-standard"],
        cwd=SOURCE_ROOT,
        text=True,
    )
    return {path for path in output.splitlines() if (SOURCE_ROOT / path).is_file()}


def test_default_roots_keep_generated_output_outside_sources():
    assert sitepaths.SOURCE_ROOT == SOURCE_ROOT.resolve()
    assert sitepaths.MAIN_REPO == SOURCE_ROOT.parent.resolve()
    assert sitepaths.OUTPUT_ROOT == (SOURCE_ROOT.parent.parent / "gh-pages").resolve()
    assert sitepaths.OUTPUT_ROOT != sitepaths.SOURCE_ROOT


def test_environment_can_select_output_root(tmp_path):
    output = tmp_path / "pages"
    main = tmp_path / "source"
    env = os.environ | {
        "ZE_SITE_OUTPUT": str(output),
        "ZE_MAIN_REPO": str(main),
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


def test_publish_replaces_old_pages_content_but_keeps_git(tmp_path):
    build_site = load_build_site()
    build = tmp_path / "build"
    publish = tmp_path / "publish"
    (publish / ".git").mkdir(parents=True)
    (publish / "tools").mkdir()
    (publish / "tools" / "build.py").write_text("old source\n")
    (publish / "old.html").write_text("old page\n")
    (build / "assets").mkdir(parents=True)
    (build / ".nojekyll").write_text("")
    (build / "CNAME").write_text("ze-software.net\n")
    (build / "index.html").write_text("new page\n")
    (build / "assets" / "site.css").write_text("css\n")

    original_output = build_site.OUTPUT_ROOT
    original_publish = build_site.PUBLISH_ROOT
    try:
        build_site.OUTPUT_ROOT = build
        build_site.PUBLISH_ROOT = publish
        build_site.publish_artifact()
    finally:
        build_site.OUTPUT_ROOT = original_output
        build_site.PUBLISH_ROOT = original_publish

    assert (publish / ".git").is_dir()
    assert not (publish / "tools").exists()
    assert not (publish / "old.html").exists()
    assert (publish / "index.html").read_text() == "new page\n"
    assert (publish / "assets" / "site.css").read_text() == "css\n"


def _page(stamp, body):
    return (
        "<html><body>%s<footer>"
        '<span class="footer-published">Published %s</span>'
        "</footer></body></html>\n" % (body, stamp)
    )


def _publish(build_site, build, publish):
    original_output = build_site.OUTPUT_ROOT
    original_publish = build_site.PUBLISH_ROOT
    try:
        build_site.OUTPUT_ROOT = build
        build_site.PUBLISH_ROOT = publish
        build_site.publish_artifact()
    finally:
        build_site.OUTPUT_ROOT = original_output
        build_site.PUBLISH_ROOT = original_publish


def _stamped_trees(tmp_path, old_page, new_page):
    build = tmp_path / "build"
    publish = tmp_path / "publish"
    (publish / ".git").mkdir(parents=True)
    build.mkdir()
    (publish / "page.html").write_text(old_page)
    (build / "page.html").write_text(new_page)
    return build, publish


def test_publish_keeps_the_old_stamp_on_an_unchanged_page(tmp_path):
    """An unchanged page must not be rewritten just because a build ran.

    Every page carries the build's publication time, so without this one build
    rewrote 673 of 754 published files with nothing but a new stamp.
    """
    build_site = load_build_site()
    build, publish = _stamped_trees(
        tmp_path,
        _page("17 August 2026 20:32 UTC", "<p>same</p>"),
        _page("18 August 2026 11:28 UTC", "<p>same</p>"),
    )

    _publish(build_site, build, publish)

    assert (publish / "page.html").read_text() == _page(
        "17 August 2026 20:32 UTC", "<p>same</p>"
    )


def test_publish_stamps_a_page_whose_content_changed(tmp_path):
    """The carry-over must not freeze the stamp on a page that did change."""
    build_site = load_build_site()
    build, publish = _stamped_trees(
        tmp_path,
        _page("17 August 2026 20:32 UTC", "<p>old</p>"),
        _page("18 August 2026 11:28 UTC", "<p>new</p>"),
    )

    _publish(build_site, build, publish)

    assert (publish / "page.html").read_text() == _page(
        "18 August 2026 11:28 UTC", "<p>new</p>"
    )


def test_publish_stamps_a_page_that_is_new(tmp_path):
    """A page with no previous publication has no stamp to carry."""
    build_site = load_build_site()
    build = tmp_path / "build"
    publish = tmp_path / "publish"
    (publish / ".git").mkdir(parents=True)
    build.mkdir()
    (build / "page.html").write_text(_page("18 August 2026 11:28 UTC", "<p>new</p>"))

    _publish(build_site, build, publish)

    assert (publish / "page.html").read_text() == _page(
        "18 August 2026 11:28 UTC", "<p>new</p>"
    )


def test_publish_keeps_the_facts_stamp_when_no_other_fact_changed(tmp_path):
    """The facts snapshot records the same build time and must carry it too.

    It is the one file left that a no-op rebuild would rewrite, and a status
    that is never clean cannot say whether anything changed.
    """
    build_site = load_build_site()
    build = tmp_path / "build"
    publish = tmp_path / "publish"
    (publish / ".git").mkdir(parents=True)
    (publish / "data").mkdir()
    (build / "data").mkdir(parents=True)
    (publish / "data" / "site-facts.json").write_text(
        '{\n  "published_at": "2026-08-17T20:32:00+00:00",\n  "stars": 12\n}\n'
    )
    (build / "data" / "site-facts.json").write_text(
        '{\n  "published_at": "2026-08-18T11:54:21+00:00",\n  "stars": 12\n}\n'
    )

    _publish(build_site, build, publish)

    assert "2026-08-17T20:32:00+00:00" in (
        publish / "data" / "site-facts.json"
    ).read_text()


def test_publish_stamps_the_facts_when_another_fact_changed(tmp_path):
    """A real fact change republishes the snapshot with this build's time."""
    build_site = load_build_site()
    build = tmp_path / "build"
    publish = tmp_path / "publish"
    (publish / ".git").mkdir(parents=True)
    (publish / "data").mkdir()
    (build / "data").mkdir(parents=True)
    (publish / "data" / "site-facts.json").write_text(
        '{\n  "published_at": "2026-08-17T20:32:00+00:00",\n  "stars": 12\n}\n'
    )
    (build / "data" / "site-facts.json").write_text(
        '{\n  "published_at": "2026-08-18T11:54:21+00:00",\n  "stars": 13\n}\n'
    )

    _publish(build_site, build, publish)

    assert "2026-08-18T11:54:21+00:00" in (
        publish / "data" / "site-facts.json"
    ).read_text()
