"""Embed locally generated Ze terminal recordings in rendered documentation pages."""

import hashlib
import html
import json
import pathlib
import re
import subprocess
from typing import NamedTuple

import sitepaths


# The site source tree, named once in sitepaths rather than recomputed from
# this file's position. A file that moves takes a positional answer with it
# and the answer stops being true.
GH_PAGES = sitepaths.SOURCE_ROOT
MAIN_REPO = sitepaths.MAIN_REPO
DEMO_ROOT = MAIN_REPO / "demos" / "terminal"
SOURCE_MANIFEST = DEMO_ROOT / "manifest.json"
SITE_ASSET_ROOT = GH_PAGES / "assets" / "demos"
ARTIFACT_MANIFEST = SITE_ASSET_ROOT / "manifest.json"
MARKER_RE = re.compile(r"<!--\s*terminal-demo:\s*([a-z0-9-]+)\s*-->")
ASSET_EXTENSIONS = {
    "cast": ".cast",
    "poster": ".png",
    "transcript": ".txt",
    "video": ".webm",
}
# A demo publishes the asset set its kind names, and nothing else. A terminal
# session is a byte stream, so it records an asciicast the player replays; a
# browser recording has no byte stream and stays a video with a poster frame.
KIND_ASSETS = {
    "terminal": ("cast", "transcript"),
    "browser": ("poster", "transcript", "video"),
}
# The player is served from this site: no CDN, and nothing fetched from
# asciinema.org. Both files are pinned, committed and reviewed under
# website/assets/vendor/, and `assets/vendor` is not a source-only directory,
# so build-site stages them verbatim.
PLAYER_JS = "assets/vendor/asciinema-player.min.js"
PLAYER_CSS = "assets/vendor/asciinema-player.css"
# The nominal monospace cell, used only to reserve the demo box before the
# player loads. They are JetBrains Mono's own metrics, the same pair
# demos/terminal/pty-session.py derives the recorded grid with, so the reserved
# box has the shape of the recording. The player measures the visitor's real
# font at run time and scales the terminal into whatever box it is given, so a
# small disagreement costs letterboxing rather than a reflow.
CELL_ADVANCE_RATIO = 0.6
CELL_LINE_RATIO = 1.32


def _load_json(path):
    with path.open(encoding="utf-8") as stream:
        value = json.load(stream)
    if not isinstance(value, dict):
        raise ValueError("terminal demo manifest must contain a JSON object: %s" % path)
    return value


def _sha256(path):
    digest = hashlib.sha256()
    with path.open("rb") as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def _assert_generated_assets_untracked():
    checks = (
        (GH_PAGES, "assets/demos"),
        (MAIN_REPO, "demos/terminal/artifacts"),
    )
    tracked = []
    for repository, path in checks:
        worktree = subprocess.run(
            ["git", "-C", str(repository), "rev-parse", "--is-inside-work-tree"],
            capture_output=True,
            text=True,
        )
        if worktree.returncode != 0:
            continue
        result = subprocess.run(
            ["git", "-C", str(repository), "ls-files", "--", path],
            check=True,
            capture_output=True,
            text=True,
        )
        tracked.extend(line for line in result.stdout.splitlines() if line)
    if tracked:
        raise ValueError(
            "generated terminal media must stay out of Git history: "
            + ", ".join(tracked)
        )


_assert_generated_assets_untracked()


def _load_catalog():
    source = _load_json(SOURCE_MANIFEST)
    generated = _load_json(ARTIFACT_MANIFEST)
    if source.get("schema") != 2:
        raise ValueError("Ze demo source manifest must use schema 2")
    if generated.get("schema") != 2:
        raise ValueError("Ze demo artifact manifest must use schema 2")

    source_demos = source.get("demos")
    generated_demos = generated.get("demos")
    renderer = generated.get("renderer")
    if not isinstance(source_demos, list):
        raise ValueError("terminal demo source manifest demos must be a list")
    if not isinstance(generated_demos, dict):
        raise ValueError("terminal demo artifact manifest demos must be an object")
    if not isinstance(renderer, dict):
        raise ValueError("terminal demo artifact manifest renderer must be an object")
    gallery_page = source.get("gallery-page")
    if not isinstance(gallery_page, str) or not gallery_page:
        raise ValueError("terminal demo source manifest gallery-page is required")

    catalog = {}
    for demo in source_demos:
        if not isinstance(demo, dict):
            raise ValueError("terminal demo source entries must be objects")
        demo_id = demo.get("id")
        if not isinstance(demo_id, str) or not demo_id:
            raise ValueError("terminal demo source entry is missing an id")
        fields = ["title", "description", "page", "platform", "kind", "engine"]
        if demo.get("kind") not in KIND_ASSETS:
            raise ValueError(
                "terminal demo %s has an unknown kind: %r" % (demo_id, demo.get("kind"))
            )
        if "cast" not in KIND_ASSETS[demo["kind"]]:
            # A cast states its own length, so only a kind without one has to
            # be told. Restating a fact the artifact carries is what let four
            # published durations drift from their recordings.
            fields.append("duration")
        for field in fields:
            if not isinstance(demo.get(field), str) or not demo[field]:
                raise ValueError(
                    "terminal demo %s source field %s is required" % (demo_id, field)
                )
        if demo_id in catalog:
            raise ValueError("duplicate terminal demo id: %s" % demo_id)
        artifact = generated_demos.get(demo_id)
        if not isinstance(artifact, dict):
            raise ValueError("terminal demo %s has no generated artifacts" % demo_id)
        catalog[demo_id] = (demo, artifact)
    return catalog, renderer, gallery_page


def _artifact_source(demo_id, artifact, kind):
    assets = artifact.get("assets")
    if not isinstance(assets, dict) or not isinstance(assets.get(kind), dict):
        raise ValueError(
            "terminal demo %s is missing its %s artifact" % (demo_id, kind)
        )
    metadata = assets[kind]
    rel_value = metadata.get("path")
    expected_hash = metadata.get("sha256")
    expected_bytes = metadata.get("bytes")
    if not isinstance(rel_value, str) or not rel_value:
        raise ValueError("terminal demo %s %s path is invalid" % (demo_id, kind))
    rel = pathlib.PurePosixPath(rel_value)
    if rel.is_absolute() or ".." in rel.parts:
        raise ValueError(
            "terminal demo %s %s path leaves the demo root" % (demo_id, kind)
        )
    source = SITE_ASSET_ROOT.joinpath(*rel.parts)
    if not source.is_file():
        raise ValueError(
            "terminal demo %s %s artifact is missing: %s" % (demo_id, kind, source)
        )
    if not isinstance(expected_bytes, int) or source.stat().st_size != expected_bytes:
        raise ValueError(
            "terminal demo %s %s artifact size does not match its manifest"
            % (demo_id, kind)
        )
    if not isinstance(expected_hash, str) or _sha256(source) != expected_hash:
        raise ValueError(
            "terminal demo %s %s artifact digest does not match its manifest"
            % (demo_id, kind)
        )
    return source, expected_hash, expected_bytes


def _publish_assets(demo_id, artifact, kind):
    """Verify and return exactly the assets this demo kind publishes.

    A half-converted demo carrying both a cast and a video fails here, which is
    what keeps a page from ever showing a player and a video at once.
    """
    published = {}
    for asset in KIND_ASSETS[kind]:
        source, _, _ = _artifact_source(demo_id, artifact, asset)
        published[asset] = source
    extra = sorted(set(artifact.get("assets", {})) - set(KIND_ASSETS[kind]))
    if extra:
        raise ValueError(
            "terminal demo %s is a %s demo and must not publish %s"
            % (demo_id, kind, ", ".join(extra))
        )
    return published


class CastFacts(NamedTuple):
    """What a recording says about itself: its grid and how long it runs."""

    columns: int
    rows: int
    seconds: float


def _cast_facts(path):
    """Read the grid and the running time out of an asciicast v2 file."""
    lines = path.read_text(encoding="utf-8").splitlines()
    header = json.loads(lines[0]) if lines else None
    if not isinstance(header, dict) or header.get("version") != 2:
        raise ValueError("not an asciicast v2 recording: %s" % path)
    columns = header.get("width")
    rows = header.get("height")
    if not isinstance(columns, int) or not isinstance(rows, int):
        raise ValueError("asciicast %s does not record its grid" % path)
    if columns < 1 or rows < 1:
        raise ValueError("asciicast %s records a %dx%d grid" % (path, columns, rows))
    seconds = 0.0
    for line in reversed(lines[1:]):
        if not line.strip():
            continue
        event = json.loads(line)
        seconds = float(event[0])
        break
    if seconds < 0:
        raise ValueError("asciicast %s ends at %r seconds" % (path, seconds))
    return CastFacts(columns, rows, seconds)


def _duration_phrase(seconds):
    """Spell a running time the way the demo catalog has always spelled one."""
    total = int(round(seconds))
    if total < 60:
        return "%d second%s" % (total, "" if total == 1 else "s")
    minutes, rest = divmod(total, 60)
    phrase = "%d minute%s" % (minutes, "" if minutes == 1 else "s")
    if rest:
        phrase += " %d second%s" % (rest, "" if rest == 1 else "s")
    return phrase


def _ratio(value):
    return ("%.4f" % value).rstrip("0").rstrip(".")


def _reserved_box(facts):
    """The aspect ratio a page reserves for a recording of this grid (R-2)."""
    return "%s / %s" % (
        _ratio(facts.columns * CELL_ADVANCE_RATIO),
        _ratio(facts.rows * CELL_LINE_RATIO),
    )


def player_head(root):
    """The player's own stylesheet and script, for `page_head(extra_head=...)`.

    Only a page carrying a terminal demo asks for these, so the other ~700
    published pages download neither.
    """
    return (
        '        <link rel="stylesheet" href="%s%s" />\n'
        '        <script src="%s%s" defer></script>\n'
    ) % (root, PLAYER_CSS, root, PLAYER_JS)


def _player_mount(cast_url, transcript_url, facts, label):
    """The element site.js turns into a player, sized before the script runs."""
    return """<div
      class="terminal-demo__player"
      data-terminal-demo-player
      data-cast-src="%s"
      data-cols="%d"
      data-rows="%d"
      style="--demo-aspect: %s"
      aria-label="%s"
    ></div>
    <noscript>
      <p class="terminal-demo__noscript">This recording is replayed by the
      page's own player. <a href="%s">Download the asciicast</a> or
      <a href="%s">read the transcript</a>.</p>
    </noscript>""" % (
        cast_url,
        facts.columns,
        facts.rows,
        _reserved_box(facts),
        label,
        cast_url,
        transcript_url,
    )


def hero_mount(demo_id, root, label):
    """The homepage hero's player, bound to the same manifest every page reads.

    The hero used to spell the demo's asset paths and digests by hand, which
    made it a fourth place the asset set was written down and the only one no
    render could correct.
    """
    catalog, _, _ = _load_catalog()
    if demo_id not in catalog:
        raise ValueError("unknown terminal demo: %s" % demo_id)
    demo, artifact = catalog[demo_id]
    if demo["kind"] != "terminal":
        raise ValueError("hero demo %s is not a terminal demo" % demo_id)
    published = _publish_assets(demo_id, artifact, demo["kind"])
    facts = _cast_facts(published["cast"])
    return _player_mount(
        html.escape(_asset_url(root, demo_id, artifact, "cast"), quote=True),
        html.escape(_asset_url(root, demo_id, artifact, "transcript"), quote=True),
        facts,
        html.escape(label, quote=True),
    )


def _asset_url(root, demo_id, artifact, kind):
    metadata = artifact["assets"][kind]
    return "%sassets/demos/%s%s?v=%s" % (
        root,
        demo_id,
        ASSET_EXTENSIONS[kind],
        metadata["sha256"][:10],
    )


def _platform_label(platform):
    if platform == "linux":
        return "Linux namespace lab"
    if platform == "portable":
        return "macOS and Linux"
    raise ValueError("unknown terminal demo platform: %r" % platform)


def _platform_sentence(platform):
    if platform == "linux":
        return "in a Linux namespace lab"
    if platform == "portable":
        return "on macOS and Linux"
    raise ValueError("unknown terminal demo platform: %r" % platform)


def _render_html(demo_id, demo, artifact, renderer, root, duration, facts, transcript):
    title = html.escape(demo["title"])
    description = html.escape(demo["description"])
    release = html.escape(str(artifact.get("release", "unknown")))
    platform = html.escape(_platform_label(demo["platform"]))
    engine = html.escape(demo["engine"])
    kind = demo["kind"]
    kind_label = "Browser" if kind == "browser" else "Terminal"
    eyebrow = html.escape("Replayable Ze %s lab" % kind.lower())
    label = html.escape(demo["title"] + " demonstration", quote=True)
    transcript_url = html.escape(
        _asset_url(root, demo_id, artifact, "transcript"), quote=True
    )
    transcript_html = html.escape(transcript.rstrip())
    recording_name = html.escape(demo_id + "." + kind.lower())

    # A demo with no cast facts is a browser recording, and it keeps the video
    # element, the poster and the WEBM label it has always had (D-3).
    if facts is None:
        poster_url = html.escape(
            _asset_url(root, demo_id, artifact, "poster"), quote=True
        )
        video_url = html.escape(
            _asset_url(root, demo_id, artifact, "video"), quote=True
        )
        format_label = "WEBM"
        player = (
            '<video controls playsinline preload="metadata" poster="%s" '
            'aria-label="%s">\n'
            '      <source src="%s" type="video/webm">\n'
            "      Your browser cannot play WebM video. "
            '<a href="%s">Download the recording</a>.\n'
            "    </video>"
        ) % (poster_url, label, video_url, video_url)
    else:
        cast_url = html.escape(_asset_url(root, demo_id, artifact, "cast"), quote=True)
        format_label = "CAST"
        player = _player_mount(cast_url, transcript_url, facts, label)

    return """<figure class="terminal-demo" data-terminal-demo="%s">
  <div class="terminal-demo__intro">
    <div>
      <span class="terminal-demo__eyebrow">%s</span>
      <h3>%s</h3>
      <p>%s</p>
    </div>
    <span class="terminal-demo__status"><i aria-hidden="true"></i> Reproducible</span>
  </div>
  <div class="terminal-demo__frame">
    <div class="terminal-demo__bar" aria-hidden="true">
      <span class="terminal-demo__dots"><i></i><i></i><i></i></span>
      <span>%s</span>
      <span>%s</span>
    </div>
    %s
  </div>
  <figcaption>
    <span>Ze %s</span><span>%s</span><span>%s</span><span>%s</span><span>%s</span>
    <a href="%s">Plain-text transcript</a>
  </figcaption>
  <details class="terminal-demo__transcript">
    <summary>Read the demonstration transcript</summary>
    <pre><code>%s</code></pre>
  </details>
</figure>""" % (
        html.escape(demo_id, quote=True),
        eyebrow,
        title,
        description,
        recording_name,
        format_label,
        player,
        release,
        html.escape(duration),
        platform,
        kind_label,
        engine,
        transcript_url,
        transcript_html,
    )


def _render_markdown(demo_id, demo, artifact, root, duration, facts, transcript):
    transcript_url = _asset_url(root, demo_id, artifact, "transcript")
    if facts is None:
        links = (
            "[Play the WebM recording](%s) · [View the poster](%s) · [Plain-text transcript](%s)"
            % (
                _asset_url(root, demo_id, artifact, "video"),
                _asset_url(root, demo_id, artifact, "poster"),
                transcript_url,
            )
        )
    else:
        links = (
            "[Download the asciicast recording](%s) · [Plain-text transcript](%s)"
            % (
                _asset_url(root, demo_id, artifact, "cast"),
                transcript_url,
            )
        )
    return """### Demo: %s

%s

%s

Recorded with Ze %s %s using %s. Duration: %s.

```console
%s
```
""" % (
        demo["title"],
        demo["description"],
        links,
        artifact.get("release", "unknown"),
        _platform_sentence(demo["platform"]),
        demo["engine"],
        duration,
        transcript.rstrip(),
    )


def expand(body_html, markdown_text, root, doc_rel):
    """Replace terminal-demo markers using verified local assets.

    Returns the two rendered bodies and the head fragment the page needs: a
    page with no terminal demo on it gets an empty string and links neither the
    player's stylesheet nor its script.
    """
    html_ids = MARKER_RE.findall(body_html)
    markdown_ids = MARKER_RE.findall(markdown_text)
    if not html_ids and not markdown_ids:
        return body_html, markdown_text, ""
    if html_ids != markdown_ids:
        raise ValueError("terminal demo markers changed during Markdown rendering")

    catalog, renderer, gallery_page = _load_catalog()
    html_replacements = {}
    markdown_replacements = {}
    needs_player = False
    for demo_id in dict.fromkeys(markdown_ids):
        if demo_id not in catalog:
            raise ValueError("unknown terminal demo marker: %s" % demo_id)
        demo, artifact = catalog[demo_id]
        if doc_rel and doc_rel not in (demo.get("page"), gallery_page):
            raise ValueError(
                "terminal demo %s belongs on %s, not %s"
                % (demo_id, demo.get("page"), doc_rel)
            )
        published = _publish_assets(demo_id, artifact, demo["kind"])
        transcript = published["transcript"].read_text(encoding="utf-8")
        if "cast" in published:
            needs_player = True
            facts = _cast_facts(published["cast"])
            duration = _duration_phrase(facts.seconds)
        else:
            facts = None
            duration = demo["duration"]
        html_replacements[demo_id] = _render_html(
            demo_id, demo, artifact, renderer, root, duration, facts, transcript
        )
        markdown_replacements[demo_id] = _render_markdown(
            demo_id, demo, artifact, root, duration, facts, transcript
        )

    return (
        MARKER_RE.sub(lambda match: html_replacements[match.group(1)], body_html),
        MARKER_RE.sub(
            lambda match: markdown_replacements[match.group(1)], markdown_text
        ),
        player_head(root) if needs_player else "",
    )
