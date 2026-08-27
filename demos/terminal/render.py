#!/usr/bin/env python3
"""Render and verify Ze terminal demonstrations from the checked-in tapes."""

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

# The sibling screen model. Both of this directory's programs are run by path,
# and both are also LOADED by path from the tests, where the directory is not on
# the import path. Putting it there is what makes the sibling reachable either
# way.
sys.path.insert(0, str(pathlib.Path(__file__).resolve().parent))

import screen  # noqa: E402  reached by the line above

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
    DEMO_ROOT / "screen.py",
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
# The artifacts a demo of each kind produces, keyed by asset name. A terminal
# demo records the terminal byte stream, so one asciicast replaces both the
# video and the poster the video needed; a browser demo has no byte stream and
# keeps its recording. This map is the only place the two sets are written, so
# a demo cannot be asked for one asset here and checked for another elsewhere.
ASSET_EXTENSIONS = {
    "terminal": {"cast": ".cast", "transcript": ".txt"},
    "browser": {"video": ".webm", "poster": ".png", "transcript": ".txt"},
}


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

    gallery_page = manifest.get("gallery-page")
    if not isinstance(gallery_page, str) or not gallery_page:
        raise ValueError("manifest.json: gallery-page is required")
    if not (ROOT / "docs" / gallery_page).is_file():
        raise ValueError(f"manifest.json: gallery page does not exist: {gallery_page}")
    indexed = demo_by_id(manifest)
    required = (
        "title",
        "description",
        "page",
        "anchor",
        "platform",
        "kind",
        "engine",
        "source",
        "validate",
    )
    for demo_id, demo in indexed.items():
        for field in required:
            if not isinstance(demo.get(field), str) or not demo[field]:
                raise ValueError(f"manifest.json: {demo_id}.{field} is required")
        if demo["kind"] not in ASSET_EXTENSIONS:
            raise ValueError(f"manifest.json: {demo_id}.kind is unsupported")
        # A cast states its own running time, so a demo that publishes one is
        # never told how long it runs: the page derives the phrase from the
        # artifact (website/tools/terminal_demos._load_catalog). Stating it here
        # as well is what let four published durations drift from their
        # recordings, so the field is refused where a cast exists and required
        # where none does.
        if "cast" in ASSET_EXTENSIONS[demo["kind"]]:
            if "duration" in demo:
                raise ValueError(
                    f"manifest.json: {demo_id}.duration is read from the cast, "
                    "so the manifest must not state it"
                )
        elif not isinstance(demo.get("duration"), str) or not demo["duration"]:
            raise ValueError(f"manifest.json: {demo_id}.duration is required")
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


def asset_paths(demo: dict[str, Any]) -> dict[str, pathlib.Path]:
    """Every artifact this demo's kind produces, keyed by asset name.

    `validate_contract` already refuses an unknown kind, so the raise below is
    for a caller that reached here around it. It fails closed rather than
    returning an empty set, which every caller would read as "this demo owes no
    artifact" and would report as a clean render.
    """
    extensions = ASSET_EXTENSIONS.get(demo["kind"])
    if extensions is None:
        raise ValueError(f"{demo['id']}: unsupported kind {demo['kind']!r}")
    return {
        name: ARTIFACT_ROOT / f"{demo['id']}{suffix}"
        for name, suffix in extensions.items()
    }


def remove_superseded_assets(demo: dict[str, Any]) -> list[pathlib.Path]:
    """Delete the artifacts this demo's kind no longer produces.

    Nothing else in the pipeline would: `verify_assets` reads the set the
    manifest NAMES, and the site stages the whole directory as it stands. So an
    artifact that stops being produced is published for as long as the tree
    survives, and the asciicast conversion produces seventeen `.webm` and
    seventeen `.png` of exactly that kind.

    Only a file named `<this demo's id><a suffix some kind produces>` is a
    candidate, so this can never reach the artifact manifest, another demo's
    recording, or a file this pipeline did not write.
    """
    keep = {path.suffix for path in asset_paths(demo).values()}
    superseded = {
        suffix
        for extensions in ASSET_EXTENSIONS.values()
        for suffix in extensions.values()
    } - keep
    removed = []
    for suffix in sorted(superseded):
        path = ARTIFACT_ROOT / f"{demo['id']}{suffix}"
        if path.is_file():
            path.unlink()
            removed.append(path)
    return removed


def definition_digest(demo: dict[str, Any]) -> str:
    """Hash the tape files that decide whether demo media must be re-rendered."""
    files = [DEMO_ROOT / "common.tape", DEMO_ROOT / demo["source"]]
    digest = hashlib.sha256()
    render_contract = {
        "kind": demo["kind"],
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
    scratch_root = ROOT / "tmp" / "terminal-demos"
    resolved_scratch_root = scratch_root.resolve()
    scratch_volume = []
    if resolved_scratch_root != scratch_root:
        scratch_volume = [
            "--volume",
            f"{resolved_scratch_root}:{resolved_scratch_root}",
        ]
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
        *scratch_volume,
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
    """Rewrite a tape so the capture costs RENDER_SPEEDUP times less time.

    Every `Sleep` is divided and the typing pace is set to the same fraction of
    the tape's own: `common.tape` asks for 125ms a character and this writes
    25ms, which is 125 / RENDER_SPEEDUP. So ONE factor describes the whole
    recording, and multiplying the timeline by it afterwards -- with ffmpeg for
    a video, with `expand_cast_timeline` for a cast -- gives back the timing the
    tape states, for the typing as well as for the pauses.
    """
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

    Ordinary demos capture faster and restore presentation timing afterwards:
    `expand_timeline` gives the time back to a video, `expand_cast_timeline`
    gives it back to a cast. Rendering seventeen demos in Docker is what this
    pipeline spends its wall clock on, so the fifth is worth having on both.
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


def expand_cast_timeline(cast: pathlib.Path, speedup: int) -> None:
    """Give a recorded terminal back the timing its tape states.

    The capture ran on the accelerated copy of the tape, so every pause and
    every keystroke is `speedup` times short. A cast's timestamps are decimal
    numbers in a text file, so multiplying them is exact and reversible; the
    video arm re-encodes the whole recording to do the same thing.

    This sits here rather than in the recorder for the same reason
    `expand_timeline` does: the recorder is a driver, and it knows the tape it
    was handed and nothing about why that tape is fast. It also keeps one place
    to look for what a stored artifact's clock means -- both arms give the time
    back after the container exits, so what is committed is real-time paced.
    """
    if speedup == 1:
        return
    lines = cast.read_text(encoding="utf-8").splitlines()
    if not lines:
        raise RuntimeError(f"{cast}: the recorder wrote no header")
    expanded = [lines[0]]
    for number, line in enumerate(lines[1:], start=2):
        try:
            event = json.loads(line)
            event[0] = round(event[0] * speedup, 6)
        except (ValueError, TypeError, IndexError, KeyError) as exc:
            raise RuntimeError(f"{cast}:{number}: not an asciicast event") from exc
        expanded.append(json.dumps(event))
    cast.write_text("\n".join(expanded) + "\n", encoding="utf-8")


# A transcript line that quotes the session rather than narrating it. The
# transcripts are hand-authored prose with the session's commands set among
# them, each behind the prompt it was typed at: `$ ` for the container shell
# (container-entrypoint.sh exports PS1='$ '), `ze>` and `ze#` for the two Ze
# CLI modes.
TRANSCRIPT_PROMPT_RE = re.compile(r"^(\$|ze[>#])(?:\s+(\S.*?))?\s*$")


def cast_visible_text(cast: pathlib.Path) -> list[str]:
    """Return what a reader of the cast was shown, one entry per painted line.

    Everything the terminal painted is kept, including what later scrolled away
    or was drawn over: a transcript quotes commands from the whole session, and
    a snapshot of any one moment holds only the last screen of it.
    """
    output: list[str] = []
    with cast.open(encoding="utf-8") as handle:
        first = handle.readline()
        if not first.strip():
            raise RuntimeError(f"{cast}: the recorder wrote no header")
        header = json.loads(first)
        for line in handle:
            if not line.strip():
                continue
            event = json.loads(line)
            if len(event) == 3 and event[1] == "o":
                output.append(event[2])

    # The header's own grid, so the reconstruction wraps and scrolls where the
    # recording did. A wider screen would hold on one line what the reader read
    # as two, and a taller one would hold lines they never saw together.
    painted = screen.Screen(header.get("height"), header.get("width"))
    for burst in output:
        painted.settle(burst)
    return painted.painted()


def transcript_commands(transcript: pathlib.Path) -> list[tuple[int, str, str]]:
    """Return the (line number, prompt, command) the transcript claims."""
    claimed: list[tuple[int, str, str]] = []
    lines = transcript.read_text(encoding="utf-8").splitlines()
    for number, line in enumerate(lines, start=1):
        match = TRANSCRIPT_PROMPT_RE.match(line)
        if match:
            claimed.append((number, match.group(1), match.group(2) or ""))
    return claimed


def check_transcript(
    demo_id: str, cast: pathlib.Path, transcript: pathlib.Path
) -> None:
    """Fail the render unless the cast shows the session the transcript claims.

    The transcript is hand-authored and never recorded, so it is the one
    description of a demo that does not come from the engine driving it. That is
    what makes it a gate on the engine: a recorder that drives a tape differently
    -- a command that is never typed, a wait that returns before the answer, a
    session that ends early -- loses one of these lines or reorders them.

    Each claimed command must appear behind its prompt, on ONE painted line,
    and the lines are searched forwards only, so the ORDER is gated as well as
    the presence. What the transcript narrates around them is prose written
    for a reader and is not looked for.
    """
    claimed = transcript_commands(transcript)
    if not claimed:
        raise RuntimeError(
            f"{demo_id}: {transcript} quotes no command line, so it gates nothing"
        )
    visible = cast_visible_text(cast)
    at = 0
    for matched, (number, prompt, command) in enumerate(claimed):
        for index in range(at, len(visible)):
            found = visible[index].find(prompt)
            if found < 0:
                continue
            if not command or visible[index].find(command, found + len(prompt)) >= 0:
                at = index + 1
                break
        else:
            quoted = f"{prompt} {command}".rstrip()
            raise RuntimeError(
                f"{demo_id}: the recording does not show what "
                f"{transcript}:{number} claims: {quoted!r}. "
                f"{matched} earlier lines matched, "
                f"the search reached painted line {at} of {len(visible)}"
            )


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

    expected = asset_paths(demo)
    # None for a terminal demo, which records a byte stream and needs neither
    # of the two ffmpeg passes below.
    capture_path = expected.get("video")
    cast_path = expected.get("cast")
    # ffmpeg runs on the HOST, and only for a kind that records a video. It used
    # to arrive with demos/terminal/install-vhs.sh, which was deleted with VHS,
    # so it is the operator's to install now. Asked for here rather than at the
    # ffmpeg call, which is reached after the container has run the whole demo.
    if capture_path is not None and shutil.which("ffmpeg") is None:
        raise RuntimeError(
            f"{demo_id}: ffmpeg is required on this host to rescale a "
            f"{demo['kind']} demo's video and poster; install it and re-run"
        )
    for name, path in expected.items():
        # The transcript is hand-authored and copied in, never rendered, so
        # removing it here would delete the demo's only text description of
        # itself before anything could rewrite it.
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
        if capture_path is not None:
            expand_timeline(capture_path, speedup)
            resize_poster(expected["poster"])
        if cast_path is not None:
            expand_cast_timeline(cast_path, speedup)
    finally:
        if render_source != source_path:
            render_source.unlink(missing_ok=True)

    transcript_source = source_path.parent / "transcript.txt"
    if cast_path is not None:
        try:
            check_transcript(demo_id, cast_path, transcript_source)
        except RuntimeError:
            # Nothing reaches the artifact manifest and nothing stays in the
            # publish tree, which is what AC-5 asks for. The recording itself
            # is moved rather than removed: it is what the next reader needs
            # to see, and making it again costs a container run.
            rejected = ROOT / "tmp" / "terminal-demos" / "rejected" / cast_path.name
            rejected.parent.mkdir(parents=True, exist_ok=True)
            shutil.move(cast_path, rejected)
            raise
    shutil.copyfile(transcript_source, expected["transcript"])

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
        "binary-sha256": sha256(BINARY_PATH),
        "source-sha256": source_digest(demo),
        "definition-sha256": definition_digest(demo),
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
    *,
    definition_only: bool = False,
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
        digest_key = "definition-sha256" if definition_only else "source-sha256"
        expected_digest = (
            definition_digest(indexed[demo_id])
            if definition_only
            else source_digest(indexed[demo_id])
        )
        if entry.get(digest_key) != expected_digest:
            stale = "definition" if definition_only else "source"
            raise ValueError(f"{demo_id}: {stale} changed since the last render")
        assets = entry.get("assets")
        if not isinstance(assets, dict):
            raise ValueError(f"{demo_id}: assets are missing")
        # `asset_paths` is what `_render_demo` writes from, so asking it again
        # here checks the demo against the set it was rendered as. An asset
        # the kind does not produce is refused before the files are read: a
        # terminal demo carrying a video, or a browser demo carrying a cast,
        # is a half-converted entry that publishes the demo twice (R-5).
        expected = asset_paths(indexed[demo_id])
        foreign = sorted(set(assets) - set(expected))
        if foreign:
            raise ValueError(
                f"{demo_id}: a {indexed[demo_id]['kind']} demo does not produce "
                + ", ".join(foreign)
                + "; its assets are "
                + ", ".join(sorted(expected))
            )
        for name in expected:
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


def stamp_definition_hashes(
    manifest: dict[str, Any],
    indexed: dict[str, dict[str, Any]],
    selected: list[str],
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
        entry["definition-sha256"] = definition_digest(indexed[demo_id])
    write_artifact_manifest(generated)
    verify_assets(manifest, indexed, selected, None, definition_only=True)


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

    The superseded artifacts go AFTER the manifest is written, so the published
    manifest never names a file this run is about to delete.
    """
    with demo_lock():
        generated = load_artifact_manifest(manifest)
        generated["renderer"] = manifest["renderer"]
        entries = generated["demos"]
        for demo_id in selected:
            entries[demo_id] = run_demo(manifest, indexed[demo_id], release)
        write_artifact_manifest(generated)
        for demo_id in selected:
            for path in remove_superseded_assets(indexed[demo_id]):
                print(f"removed superseded artifact: {path.name}")
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
    parser.add_argument(
        "--check-definition",
        action="store_true",
        help="verify existing artifacts against the tape definitions only",
    )
    parser.add_argument(
        "--stamp-definition-hashes",
        action="store_true",
        help="write tape definition hashes into the existing artifact manifest",
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
    if (
        sum(
            bool(flag)
            for flag in (
                args.check,
                args.validate,
                args.check_definition,
                args.stamp_definition_hashes,
            )
        )
        > 1
    ):
        raise ValueError(
            "--check, --validate, --check-definition, and --stamp-definition-hashes cannot be combined"
        )
    if args.check or args.check_definition:
        with demo_lock():
            verify_assets(
                manifest,
                indexed,
                selected,
                args.release if args.check else None,
                definition_only=args.check_definition,
            )
        return 0
    if args.stamp_definition_hashes:
        with demo_lock():
            stamp_definition_hashes(manifest, indexed, selected)
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
