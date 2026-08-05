#!/usr/bin/env python3
"""Render and verify Ze terminal demonstrations from the checked-in VHS tapes."""

from __future__ import annotations

import argparse
import contextlib
import fcntl
import hashlib
import json
import os
import pathlib
import re
import shutil
import subprocess
import sys
import time
from typing import Any

ROOT = pathlib.Path(__file__).resolve().parents[2]
DEMO_ROOT = ROOT / "demos" / "terminal"
DEFAULT_ARTIFACT_ROOT = ROOT.parent / "gh-pages" / "assets" / "demos"
ARTIFACT_ROOT = pathlib.Path(
    os.environ.get("ZE_TERMINAL_DEMO_OUTPUT", DEFAULT_ARTIFACT_ROOT)
).resolve()
MANIFEST_PATH = DEMO_ROOT / "manifest.json"
ARTIFACT_MANIFEST_PATH = ARTIFACT_ROOT / "manifest.json"
BINARY_PATH = ROOT / "tmp" / "terminal-demos" / "bin" / "ze"
SHARED_SOURCE_PATHS = (
    DEMO_ROOT / "common.tape",
    DEMO_ROOT / "cards.sh",
    DEMO_ROOT / "Dockerfile",
    DEMO_ROOT / "container-entrypoint.sh",
    DEMO_ROOT / "demo-lock.sh",
    DEMO_ROOT / "validate-common.sh",
    DEMO_ROOT / "pty-session.py",
    DEMO_ROOT / "render.py",
)
ID_RE = re.compile(r"^[a-z0-9]+(?:-[a-z0-9]+)*$")
SLEEP_RE = re.compile(r"^Sleep (\d+(?:\.\d+)?)(ms|s|m)$")
LOCK_PATH = ROOT / "tmp" / "terminal-demos" / "demo-run.lock"
# Longer than the shell's 1800, because this side holds the lock for a whole
# `--all` render rather than for one demo.
LOCK_WAIT_SECONDS = 7200
LOCK_DEPTH = 0
RENDER_SPEEDUP = 5
RENDER_TYPING_SPEED_MS = 25
OUTPUT_WIDTH = 1680
OUTPUT_HEIGHT = 1008


def sha256(path: pathlib.Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def load_json(path: pathlib.Path) -> dict[str, Any]:
    with path.open(encoding="utf-8") as stream:
        value = json.load(stream)
    if not isinstance(value, dict):
        raise ValueError(f"{path}: expected a JSON object")
    return value


def demo_by_id(manifest: dict[str, Any]) -> dict[str, dict[str, Any]]:
    demos = manifest.get("demos")
    if not isinstance(demos, list) or not demos:
        raise ValueError("manifest.json: demos must be a non-empty list")

    indexed: dict[str, dict[str, Any]] = {}
    for demo in demos:
        if not isinstance(demo, dict):
            raise ValueError("manifest.json: every demo must be an object")
        demo_id = demo.get("id")
        if not isinstance(demo_id, str) or not ID_RE.fullmatch(demo_id):
            raise ValueError(f"manifest.json: invalid demo id {demo_id!r}")
        if demo_id in indexed:
            raise ValueError(f"manifest.json: duplicate demo id {demo_id}")
        indexed[demo_id] = demo
    return indexed


def validate_contract(manifest: dict[str, Any]) -> dict[str, dict[str, Any]]:
    if manifest.get("schema") != 2:
        raise ValueError("manifest.json: unsupported schema")

    renderer = manifest.get("renderer")
    if not isinstance(renderer, dict):
        raise ValueError("manifest.json: renderer must be an object")
    for field in ("name", "version", "image", "platform"):
        if not isinstance(renderer.get(field), str) or not renderer[field]:
            raise ValueError(f"manifest.json: renderer.{field} is required")

    gallery_page = manifest.get("gallery_page")
    if not isinstance(gallery_page, str) or not gallery_page:
        raise ValueError("manifest.json: gallery_page is required")
    if not (ROOT / "docs" / gallery_page).is_file():
        raise ValueError(f"manifest.json: gallery page does not exist: {gallery_page}")
    indexed = demo_by_id(manifest)
    required = (
        "title",
        "description",
        "page",
        "anchor",
        "platform",
        "duration",
        "kind",
        "engine",
        "source",
        "validate",
    )
    for demo_id, demo in indexed.items():
        for field in required:
            if not isinstance(demo.get(field), str) or not demo[field]:
                raise ValueError(f"manifest.json: {demo_id}.{field} is required")
        if demo["kind"] not in ("terminal", "browser"):
            raise ValueError(f"manifest.json: {demo_id}.kind is unsupported")
        if "privileged" in demo and not isinstance(demo["privileged"], bool):
            raise ValueError(f"manifest.json: {demo_id}.privileged must be a boolean")
        if "realtime" in demo and not isinstance(demo["realtime"], bool):
            raise ValueError(f"manifest.json: {demo_id}.realtime must be a boolean")

        page = ROOT / "docs" / demo["page"]
        source = DEMO_ROOT / demo["source"]
        transcript = source.parent / "transcript.txt"
        validator = DEMO_ROOT / demo["validate"]
        if not page.is_file():
            raise ValueError(
                f"manifest.json: page does not exist: {page.relative_to(ROOT)}"
            )
        if not source.is_file():
            raise ValueError(
                f"manifest.json: source does not exist: {source.relative_to(ROOT)}"
            )
        if not transcript.is_file():
            raise ValueError(
                f"manifest.json: transcript does not exist: {transcript.relative_to(ROOT)}"
            )
        if not validator.is_file():
            raise ValueError(
                f"manifest.json: validator does not exist: {validator.relative_to(ROOT)}"
            )
    return indexed


def source_digest(demo: dict[str, Any]) -> str:
    source = DEMO_ROOT / demo["source"]
    files = [*SHARED_SOURCE_PATHS]
    files.extend(
        path
        for path in source.parent.rglob("*")
        if path.is_file() and not path.name.startswith(".")
    )
    digest = hashlib.sha256()
    render_contract = {
        "privileged": demo.get("privileged", False),
        "realtime": demo.get("realtime", False),
        "source": demo["source"],
    }
    digest.update(json.dumps(render_contract, sort_keys=True).encode())
    digest.update(b"\0")
    for path in sorted(files):
        digest.update(path.relative_to(ROOT).as_posix().encode())
        digest.update(b"\0")
        digest.update(path.read_bytes())
        digest.update(b"\0")
    return digest.hexdigest()


@contextlib.contextmanager
def demo_lock() -> Any:
    """Hold the demo state tree for the work of the caller.

    That is one demo for a validation, and a whole `--all` render for
    `render_selected`, which is why the wait above is generous.

    A demo run owns tmp/terminal-demos/state/<demo-id>, and this process owns
    the render tape, the artifacts and the artifact manifest. Two runs at once
    delete and rewrite each other's files, so the lock covers the container AND
    the host steps around it: the tape this process writes and removes, the
    ffmpeg passes that rewrite the capture, and the manifest.

    demos/terminal/demo-lock.sh takes the same lock inside the container. The
    container is told the lock is already held (ZE_DEMO_LOCK_HELD), because a
    second acquisition from another process would wait for this one forever.
    """
    global LOCK_DEPTH

    if LOCK_DEPTH:
        # Already held by this process. `render_selected` holds it around the
        # `run_demo` calls that take it again, and flock(2) would block on a
        # second descriptor over the same file until the wait ran out.
        LOCK_DEPTH += 1
        try:
            yield
        finally:
            LOCK_DEPTH -= 1
        return

    LOCK_PATH.parent.mkdir(parents=True, exist_ok=True)
    deadline = time.monotonic() + LOCK_WAIT_SECONDS
    # Read-only, because the container runs as root and leaves the lock file
    # owned by root on the shared mount. flock(2) takes any open descriptor,
    # so the host does not need write access to a file it did not create.
    handle = os.fdopen(os.open(LOCK_PATH, os.O_RDONLY | os.O_CREAT, 0o644), "rb")
    with handle:
        while not _took_lock(handle):
            if time.monotonic() >= deadline:
                raise RuntimeError(
                    f"another demo run held {LOCK_PATH} for {LOCK_WAIT_SECONDS} seconds"
                )
            time.sleep(0.5)
        LOCK_DEPTH = 1
        try:
            yield
        finally:
            LOCK_DEPTH = 0
            fcntl.flock(handle, fcntl.LOCK_UN)


def _took_lock(handle: Any) -> bool:
    try:
        fcntl.flock(handle, fcntl.LOCK_EX | fcntl.LOCK_NB)
    except OSError:
        return False
    return True


def container_command(
    renderer: dict[str, Any], entry: pathlib.PurePosixPath, privileged: bool
) -> list[str]:
    uid = str(os.getuid()) if hasattr(os, "getuid") else "0"
    gid = str(os.getgid()) if hasattr(os, "getgid") else "0"
    command = [
        "docker",
        "run",
        "--rm",
        "--network",
        "none",
        "--cap-add",
        "NET_ADMIN",
        "--cap-add",
        "NET_RAW",
        "--cap-add",
        "SYS_ADMIN",
        "--security-opt",
        "seccomp=unconfined",
        "--env",
        f"HOST_UID={uid}",
        "--env",
        f"HOST_GID={gid}",
        "--env",
        "ZE_DEMO_LOCK_HELD=1",
        "--volume",
        f"{ROOT}:/src",
        "--volume",
        f"{ARTIFACT_ROOT}:/src/demos/terminal/artifacts",
        "--workdir",
        "/src/demos/terminal",
        renderer["image"],
        str(entry),
    ]
    if privileged:
        command[3:3] = ["--privileged"]
    if not renderer["platform"].endswith("/native"):
        command[3:3] = ["--platform", renderer["platform"]]
    return command


def run_validation(manifest: dict[str, Any], demo: dict[str, Any]) -> None:
    demo_id = demo["id"]
    validator = pathlib.PurePosixPath("/src/demos/terminal") / demo["validate"]
    command = container_command(
        manifest["renderer"], validator, demo.get("privileged", False)
    )
    print(f"validating {demo_id}...")
    with demo_lock():
        subprocess.run(command, cwd=ROOT, check=True)


def accelerated_terminal_tape(demo: dict[str, Any]) -> pathlib.Path:
    source = DEMO_ROOT / demo["source"]
    lines: list[str] = []
    configured = False
    units = {"ms": 1, "s": 1000, "m": 60_000}
    for line in source.read_text(encoding="utf-8").splitlines():
        lines.append(line)
        if line == "Source common.tape":
            lines.append(f"Set TypingSpeed {RENDER_TYPING_SPEED_MS}ms")
            configured = True
            continue
        match = SLEEP_RE.fullmatch(line)
        if match:
            milliseconds = round(
                float(match.group(1)) * units[match.group(2)] / RENDER_SPEEDUP
            )
            lines[-1] = f"Sleep {max(milliseconds, 1)}ms"
    if not configured:
        raise ValueError(f"{demo['id']}: terminal tape does not source common.tape")
    output_dir = ROOT / "tmp" / "terminal-demos" / "render-tapes"
    output_dir.mkdir(parents=True, exist_ok=True)
    output = output_dir / f"{demo['id']}.tape"
    output.write_text("\n".join(lines) + "\n", encoding="utf-8")
    return output


def capture_speedup(demo: dict[str, Any]) -> int:
    """Timeline compression used while capturing.

    Ordinary demos capture faster and restore presentation timing afterwards.
    Real-time demos capture at wall-clock speed so output that depends on the
    real clock (commit-confirmed countdowns, per-second traceroute probes) keeps
    its true cadence instead of being stretched.
    """
    return 1 if demo.get("realtime", False) else RENDER_SPEEDUP


def render_tape(demo: dict[str, Any]) -> pathlib.Path:
    """Return the tape used to capture a terminal demo.

    Real-time demos render the checked-in tape unchanged (full 125ms typing and
    full scripted sleeps); ordinary demos render an accelerated copy.
    """
    if demo.get("realtime", False):
        return DEMO_ROOT / demo["source"]
    return accelerated_terminal_tape(demo)


def expand_timeline(capture: pathlib.Path, speedup: int) -> None:
    compressed = capture.with_name(f"{capture.stem}.fast{capture.suffix}")
    capture.replace(compressed)
    try:
        subprocess.run(
            [
                "ffmpeg",
                "-y",
                "-loglevel",
                "error",
                "-itsscale",
                str(speedup),
                "-i",
                str(compressed),
                "-map",
                "0:v:0",
                "-an",
                "-vf",
                f"scale={OUTPUT_WIDTH}:{OUTPUT_HEIGHT}:flags=lanczos",
                "-c:v",
                "libvpx-vp9",
                "-deadline",
                "realtime",
                "-cpu-used",
                "8",
                "-crf",
                "30",
                "-b:v",
                "0",
                "-row-mt",
                "1",
                "-tile-columns",
                "2",
                "-fps_mode",
                "passthrough",
                str(capture),
            ],
            check=True,
        )
    except BaseException:
        capture.unlink(missing_ok=True)
        compressed.replace(capture)
        raise
    compressed.unlink()


def resize_poster(poster: pathlib.Path) -> None:
    original = poster.with_name(f"{poster.stem}.original{poster.suffix}")
    poster.replace(original)
    try:
        subprocess.run(
            [
                "ffmpeg",
                "-y",
                "-loglevel",
                "error",
                "-i",
                str(original),
                "-vf",
                f"scale={OUTPUT_WIDTH}:{OUTPUT_HEIGHT}:flags=lanczos",
                "-frames:v",
                "1",
                str(poster),
            ],
            check=True,
        )
    except BaseException:
        poster.unlink(missing_ok=True)
        original.replace(poster)
        raise
    original.unlink()


def run_demo(
    manifest: dict[str, Any], demo: dict[str, Any], release: str
) -> dict[str, Any]:
    """Render one demo while it owns the demo state tree.

    The lock covers the render tape and the artifact rewrites as well as the
    container: `_render_demo` writes tmp/terminal-demos/render-tapes/<id>.tape,
    removes it afterwards, and rewrites the capture with ffmpeg. A second run
    of the same demo would remove the tape this one is about to read.
    """
    with demo_lock():
        return _render_demo(manifest, demo, release)


def _render_demo(
    manifest: dict[str, Any], demo: dict[str, Any], release: str
) -> dict[str, Any]:
    demo_id = demo["id"]
    renderer = manifest["renderer"]
    source_path = DEMO_ROOT / demo["source"]
    speedup = capture_speedup(demo)
    render_source = render_tape(demo) if demo["kind"] == "terminal" else source_path
    source = pathlib.PurePosixPath("/src") / render_source.relative_to(ROOT)
    ARTIFACT_ROOT.mkdir(parents=True, exist_ok=True)

    capture_path = ARTIFACT_ROOT / f"{demo_id}.webm"
    expected = {
        "video": capture_path,
        "poster": ARTIFACT_ROOT / f"{demo_id}.png",
        "transcript": ARTIFACT_ROOT / f"{demo_id}.txt",
    }
    for name, path in expected.items():
        if name != "transcript":
            path.unlink(missing_ok=True)

    command = container_command(renderer, source, demo.get("privileged", False))
    image_index = command.index(renderer["image"])
    command[image_index:image_index] = [
        "--env",
        f"ZE_DEMO_RELEASE={release}",
        "--env",
        f"ZE_DEMO_SPEEDUP={speedup}",
    ]
    print(f"rendering {demo_id}...")
    try:
        subprocess.run(command, cwd=ROOT, check=True)
        expand_timeline(capture_path, speedup)
        resize_poster(expected["poster"])
    finally:
        if render_source != source_path:
            render_source.unlink(missing_ok=True)

    shutil.copyfile(source_path.parent / "transcript.txt", expected["transcript"])

    assets: dict[str, dict[str, Any]] = {}
    for name, path in expected.items():
        if not path.is_file() or path.stat().st_size == 0:
            raise RuntimeError(f"{demo_id}: missing generated {name}: {path}")
        assets[name] = {
            "path": path.relative_to(ARTIFACT_ROOT).as_posix(),
            "bytes": path.stat().st_size,
            "sha256": sha256(path),
        }

    return {
        "release": release,
        "binary_sha256": sha256(BINARY_PATH),
        "source_sha256": source_digest(demo),
        "assets": assets,
    }


def load_artifact_manifest(manifest: dict[str, Any]) -> dict[str, Any]:
    if ARTIFACT_MANIFEST_PATH.is_file():
        generated = load_json(ARTIFACT_MANIFEST_PATH)
        if generated.get("schema") == 2 and isinstance(generated.get("demos"), dict):
            return generated
    return {
        "schema": 2,
        "renderer": manifest["renderer"],
        "demos": {},
    }


def write_artifact_manifest(generated: dict[str, Any]) -> None:
    ARTIFACT_ROOT.mkdir(parents=True, exist_ok=True)
    text = json.dumps(generated, indent=2, sort_keys=True) + "\n"
    ARTIFACT_MANIFEST_PATH.write_text(text, encoding="utf-8")


def verify_assets(
    manifest: dict[str, Any],
    indexed: dict[str, dict[str, Any]],
    selected: list[str],
    release: str | None,
) -> None:
    generated = load_json(ARTIFACT_MANIFEST_PATH)
    if generated.get("schema") != 2:
        raise ValueError("generated manifest: unsupported schema")
    if generated.get("renderer") != manifest.get("renderer"):
        raise ValueError("generated manifest: renderer contract is stale")
    generated_demos = generated.get("demos")
    if not isinstance(generated_demos, dict):
        raise ValueError("generated manifest: demos must be an object")

    for demo_id in selected:
        entry = generated_demos.get(demo_id)
        if not isinstance(entry, dict):
            raise ValueError(f"generated manifest: missing {demo_id}")
        if release is not None and entry.get("release") != release:
            raise ValueError(
                f"{demo_id}: rendered for {entry.get('release')!r}, expected {release!r}"
            )
        if entry.get("source_sha256") != source_digest(indexed[demo_id]):
            raise ValueError(f"{demo_id}: source changed since the last render")
        assets = entry.get("assets")
        if not isinstance(assets, dict):
            raise ValueError(f"{demo_id}: assets are missing")
        for name in ("video", "poster", "transcript"):
            asset = assets.get(name)
            if not isinstance(asset, dict) or not isinstance(asset.get("path"), str):
                raise ValueError(f"{demo_id}: missing {name} metadata")
            path = ARTIFACT_ROOT / asset["path"]
            if not path.is_file():
                raise ValueError(f"{demo_id}: missing generated asset: {path}")
            if path.stat().st_size != asset.get("bytes") or sha256(path) != asset.get(
                "sha256"
            ):
                raise ValueError(f"{demo_id}: {name} digest mismatch")
    print("Ze demo artifacts verified: " + ", ".join(selected))


def render_selected(
    manifest: dict[str, Any],
    indexed: dict[str, dict[str, Any]],
    selected: list[str],
    release: str,
) -> None:
    """Render the selected demos and publish one artifact manifest.

    The lock covers the whole read-modify-write of that manifest, not only the
    renders inside it. Two runs that each read it before either writes both
    publish their own view, and the second write drops the first run's entries.
    """
    with demo_lock():
        generated = load_artifact_manifest(manifest)
        generated["renderer"] = manifest["renderer"]
        entries = generated["demos"]
        for demo_id in selected:
            entries[demo_id] = run_demo(manifest, indexed[demo_id], release)
        write_artifact_manifest(generated)
        verify_assets(manifest, indexed, selected, release)


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    selection = parser.add_mutually_exclusive_group(required=True)
    selection.add_argument(
        "--all", action="store_true", help="render or verify every demo"
    )
    selection.add_argument(
        "--demo", action="append", help="demo id to render or verify"
    )
    parser.add_argument(
        "--release", help="Ze release identity recorded in artifact metadata"
    )
    parser.add_argument(
        "--check", action="store_true", help="verify existing artifacts only"
    )
    parser.add_argument(
        "--validate", action="store_true", help="run scenario output validators only"
    )
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    manifest = load_json(MANIFEST_PATH)
    indexed = validate_contract(manifest)
    selected = list(indexed) if args.all else list(dict.fromkeys(args.demo))
    unknown = [demo_id for demo_id in selected if demo_id not in indexed]
    if unknown:
        raise ValueError("unknown demo id(s): " + ", ".join(unknown))

    if args.check and args.validate:
        raise ValueError("--check and --validate cannot be combined")
    if args.check:
        with demo_lock():
            verify_assets(manifest, indexed, selected, args.release)
        return 0
    if not BINARY_PATH.is_file():
        raise ValueError(f"missing demo binary: {BINARY_PATH.relative_to(ROOT)}")

    for demo_id in selected:
        run_validation(manifest, indexed[demo_id])
    if args.validate:
        return 0
    if not args.release:
        raise ValueError("--release is required when rendering")

    render_selected(manifest, indexed, selected, args.release)
    return 0


if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except (OSError, subprocess.CalledProcessError, RuntimeError, ValueError) as exc:
        print(f"error: {exc}", file=sys.stderr)
        raise SystemExit(1)
