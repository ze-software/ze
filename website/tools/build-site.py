#!/usr/bin/env -S uv run --with markdown --with rcssmin --with rjsmin python3
"""Build the complete website in an isolated Pages artifact directory."""

import hashlib
import os
import pathlib
import shutil
import subprocess
import sys
import tempfile

import sitepaths

SOURCE_ROOT = sitepaths.SOURCE_ROOT
PUBLISH_ROOT = sitepaths.OUTPUT_ROOT
OUTPUT_ROOT = PUBLISH_ROOT
MAIN_REPO = sitepaths.MAIN_REPO


def source_files():
    result = subprocess.run(
        ["git", "ls-files", "-z", "--cached", "--others", "--exclude-standard"],
        cwd=SOURCE_ROOT,
        check=True,
        capture_output=True,
    )
    paths = []
    for raw in result.stdout.split(b"\0"):
        if not raw:
            continue
        path = SOURCE_ROOT / raw.decode()
        if path.is_file() or path.is_symlink():
            paths.append(path)
    return paths


def source_digest(paths):
    digest = hashlib.sha256()
    for path in sorted(paths):
        rel = path.relative_to(SOURCE_ROOT).as_posix()
        digest.update(rel.encode())
        digest.update(b"\0")
        if path.is_symlink():
            digest.update(path.readlink().as_posix().encode())
        else:
            digest.update(path.read_bytes())
        digest.update(b"\0")
    return digest.hexdigest()


def clean_tree_except_git(root):
    root.mkdir(parents=True, exist_ok=True)
    for path in root.iterdir():
        if path.name == ".git":
            continue
        if path.is_dir() and not path.is_symlink():
            shutil.rmtree(path)
        else:
            path.unlink()


def replace_destination(source, destination):
    if destination.is_dir() and not destination.is_symlink():
        shutil.rmtree(destination)
    elif destination.exists() or destination.is_symlink():
        destination.unlink()
    if source.is_dir() and not source.is_symlink():
        shutil.copytree(source, destination, symlinks=True)
    elif source.is_symlink():
        destination.symlink_to(source.readlink())
    else:
        shutil.copy2(source, destination)


def copy_tree_except_git(source, destination):
    destination.mkdir(parents=True, exist_ok=True)
    for path in source.iterdir():
        if path.name == ".git":
            continue
        replace_destination(path, destination / path.name)


def clean_output_root():
    clean_tree_except_git(OUTPUT_ROOT)


def stage_sources(paths, clean):
    if clean:
        clean_output_root()
    elif not OUTPUT_ROOT.exists():
        raise RuntimeError(
            "partial build requires an existing full artifact: %s" % OUTPUT_ROOT
        )
    OUTPUT_ROOT.mkdir(parents=True, exist_ok=True)

    for source in paths:
        rel = source.relative_to(SOURCE_ROOT)
        destination = OUTPUT_ROOT / rel
        destination.parent.mkdir(parents=True, exist_ok=True)
        replace_destination(source, destination)


def stage_terminal_media():
    # Demo media is intentionally ignored by Git and produced before this build,
    # so the tracked-source staging pass cannot discover it.
    source = pathlib.Path(
        os.environ.get("ZE_TERMINAL_DEMO_SOURCE", SOURCE_ROOT / "assets" / "demos")
    ).resolve()
    if not source.is_dir():
        return
    shutil.copytree(source, OUTPUT_ROOT / "assets" / "demos", dirs_exist_ok=True)


def run(command):
    env = os.environ.copy()
    env["ZE_MAIN_REPO"] = str(MAIN_REPO)
    env["ZE_SITE_OUTPUT"] = str(OUTPUT_ROOT)
    env["ZE_SITE_ALLOW_IN_TREE"] = "1"
    result = subprocess.run(command, cwd=OUTPUT_ROOT, env=env)
    if result.returncode:
        raise RuntimeError(
            "command failed with exit status %d: %s"
            % (result.returncode, " ".join(map(str, command)))
        )


def build_talk_artifacts():
    activity = OUTPUT_ROOT / "talks" / "linx-2026-06" / "activity.html"
    run(
        [
            sys.executable,
            OUTPUT_ROOT / "presentations" / "tools" / "loc_activity.py",
            "--compact",
            "--repo",
            MAIN_REPO,
            "--days",
            "365",
            "--today",
            "2026-06-11",
            "--output",
            activity,
        ]
    )
    run(
        [
            sys.executable,
            OUTPUT_ROOT / "presentations" / "tools" / "bundle-html.py",
            OUTPUT_ROOT / "talks" / "linx-2026-06" / "index.html",
            OUTPUT_ROOT / "talks" / "netmcr-2026-04" / "index.html",
        ]
    )


def prune_source_only_files():
    paths = sorted(
        OUTPUT_ROOT.rglob("*"), key=lambda path: len(path.parts), reverse=True
    )
    for path in paths:
        rel = path.relative_to(OUTPUT_ROOT).as_posix()
        if path.is_file() or path.is_symlink():
            if sitepaths.is_source_only(rel):
                path.unlink()
        elif path.is_dir() and sitepaths.is_source_only(rel):
            shutil.rmtree(path)


def validate_artifact():
    required = (
        ".nojekyll",
        "CNAME",
        "index.html",
        "assets/site.css",
        "assets/site.js",
        "talks/linx-2026-06/index-inlined.html",
        "talks/netmcr-2026-04/index-inlined.html",
    )
    missing = [path for path in required if not (OUTPUT_ROOT / path).is_file()]
    leaked = [
        path.relative_to(OUTPUT_ROOT).as_posix()
        for path in OUTPUT_ROOT.rglob("*")
        if sitepaths.is_source_only(path.relative_to(OUTPUT_ROOT).as_posix())
    ]
    if missing:
        raise RuntimeError(
            "artifact is missing required files: %s" % ", ".join(missing)
        )
    if leaked:
        raise RuntimeError(
            "source-only paths leaked into artifact: %s" % ", ".join(leaked)
        )


def publish_artifact():
    if not (PUBLISH_ROOT / ".git").exists():
        raise RuntimeError("ZE_SITE_OUTPUT must be a git worktree: %s" % PUBLISH_ROOT)
    clean_tree_except_git(PUBLISH_ROOT)
    copy_tree_except_git(OUTPUT_ROOT, PUBLISH_ROOT)


def main():
    global OUTPUT_ROOT

    if not (MAIN_REPO / "go.mod").is_file():
        raise RuntimeError("Ze main checkout not found: %s" % MAIN_REPO)

    partial = "--only" in sys.argv[1:] or any(
        arg.startswith("--only=") for arg in sys.argv[1:]
    )
    inputs = source_files()
    before = source_digest(inputs)
    tmp_parent = MAIN_REPO / "tmp"
    tmp_parent.mkdir(exist_ok=True)
    with tempfile.TemporaryDirectory(prefix="ze-site-build-", dir=tmp_parent) as build:
        OUTPUT_ROOT = pathlib.Path(build).resolve()
        if partial:
            if not PUBLISH_ROOT.exists():
                raise RuntimeError(
                    "partial build requires an existing full artifact: %s" % PUBLISH_ROOT
                )
            copy_tree_except_git(PUBLISH_ROOT, OUTPUT_ROOT)
        stage_sources(inputs, clean=not partial)
        stage_terminal_media()
        build_talk_artifacts()
        run([sys.executable, OUTPUT_ROOT / "tools" / "build.py", *sys.argv[1:]])
        prune_source_only_files()
        validate_artifact()
        publish_artifact()

    after = source_digest(inputs)
    if after != before:
        raise RuntimeError("website source files changed during the isolated build")

    print("published artifact: %s" % PUBLISH_ROOT)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
