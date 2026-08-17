"""Embed locally generated Ze terminal recordings in rendered documentation pages."""

import hashlib
import html
import json
import pathlib
import re
import subprocess
import sitepaths


GH_PAGES = pathlib.Path(__file__).resolve().parent.parent
MAIN_REPO = sitepaths.MAIN_REPO
DEMO_ROOT = MAIN_REPO / "demos" / "terminal"
SOURCE_MANIFEST = DEMO_ROOT / "manifest.json"
SITE_ASSET_ROOT = GH_PAGES / "assets" / "demos"
ARTIFACT_MANIFEST = SITE_ASSET_ROOT / "manifest.json"
MARKER_RE = re.compile(r"<!--\s*terminal-demo:\s*([a-z0-9-]+)\s*-->")
ASSET_EXTENSIONS = {
    "poster": ".png",
    "transcript": ".txt",
    "video": ".webm",
}


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
    gallery_page = source.get("gallery_page")
    if not isinstance(gallery_page, str) or not gallery_page:
        raise ValueError("terminal demo source manifest gallery_page is required")

    catalog = {}
    for demo in source_demos:
        if not isinstance(demo, dict):
            raise ValueError("terminal demo source entries must be objects")
        demo_id = demo.get("id")
        if not isinstance(demo_id, str) or not demo_id:
            raise ValueError("terminal demo source entry is missing an id")
        for field in (
            "title",
            "description",
            "page",
            "platform",
            "duration",
            "kind",
            "engine",
        ):
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


def _publish_assets(demo_id, artifact):
    published = {}
    for kind in ASSET_EXTENSIONS:
        source, _, _ = _artifact_source(demo_id, artifact, kind)
        published[kind] = source
    return published


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


def _render_html(demo_id, demo, artifact, renderer, root, transcript):
    title = html.escape(demo["title"])
    description = html.escape(demo["description"])
    release = html.escape(str(artifact.get("release", "unknown")))
    duration = html.escape(demo["duration"])
    platform = html.escape(_platform_label(demo["platform"]))
    engine = html.escape(demo["engine"])
    kind = demo["kind"]
    kind_label = "Browser" if kind == "browser" else "Terminal"
    eyebrow = html.escape("Replayable Ze %s lab" % kind.lower())
    label = html.escape(demo["title"] + " demonstration", quote=True)
    poster_url = html.escape(_asset_url(root, demo_id, artifact, "poster"), quote=True)
    transcript_url = html.escape(
        _asset_url(root, demo_id, artifact, "transcript"), quote=True
    )
    video_url = html.escape(_asset_url(root, demo_id, artifact, "video"), quote=True)
    transcript_html = html.escape(transcript.rstrip())
    recording_name = html.escape(demo_id + "." + kind.lower())

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
      <span>WEBM</span>
    </div>
    <video controls playsinline preload="metadata" poster="%s" aria-label="%s">
      <source src="%s" type="video/webm">
      Your browser cannot play WebM video. <a href="%s">Download the recording</a>.
    </video>
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
        poster_url,
        label,
        video_url,
        video_url,
        release,
        duration,
        platform,
        kind_label,
        engine,
        transcript_url,
        transcript_html,
    )


def _render_markdown(demo_id, demo, artifact, root, transcript):
    video_url = _asset_url(root, demo_id, artifact, "video")
    poster_url = _asset_url(root, demo_id, artifact, "poster")
    transcript_url = _asset_url(root, demo_id, artifact, "transcript")
    return """### Demo: %s

%s

[Play the WebM recording](%s) · [View the poster](%s) · [Plain-text transcript](%s)

Recorded with Ze %s %s using %s. Duration: %s.

```console
%s
```
""" % (
        demo["title"],
        demo["description"],
        video_url,
        poster_url,
        transcript_url,
        artifact.get("release", "unknown"),
        _platform_sentence(demo["platform"]),
        demo["engine"],
        demo["duration"],
        transcript.rstrip(),
    )


def expand(body_html, markdown_text, root, doc_rel):
    """Replace terminal-demo markers using verified local assets."""
    html_ids = MARKER_RE.findall(body_html)
    markdown_ids = MARKER_RE.findall(markdown_text)
    if not html_ids and not markdown_ids:
        return body_html, markdown_text
    if html_ids != markdown_ids:
        raise ValueError("terminal demo markers changed during Markdown rendering")

    catalog, renderer, gallery_page = _load_catalog()
    html_replacements = {}
    markdown_replacements = {}
    for demo_id in dict.fromkeys(markdown_ids):
        if demo_id not in catalog:
            raise ValueError("unknown terminal demo marker: %s" % demo_id)
        demo, artifact = catalog[demo_id]
        if doc_rel and doc_rel not in (demo.get("page"), gallery_page):
            raise ValueError(
                "terminal demo %s belongs on %s, not %s"
                % (demo_id, demo.get("page"), doc_rel)
            )
        published = _publish_assets(demo_id, artifact)
        transcript = published["transcript"].read_text(encoding="utf-8")
        html_replacements[demo_id] = _render_html(
            demo_id, demo, artifact, renderer, root, transcript
        )
        markdown_replacements[demo_id] = _render_markdown(
            demo_id, demo, artifact, root, transcript
        )

    return (
        MARKER_RE.sub(lambda match: html_replacements[match.group(1)], body_html),
        MARKER_RE.sub(
            lambda match: markdown_replacements[match.group(1)], markdown_text
        ),
    )
