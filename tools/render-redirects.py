#!/usr/bin/env -S uv run python3
"""Render static fallback redirects for legacy public URLs."""

import html
import json
import pathlib

import page_registry
import sitelib

HERE = pathlib.Path(__file__).resolve().parent
GH_PAGES = HERE.parent


def redirect_html(target):
    suffix = "" if pathlib.PurePosixPath(target).suffix else "/"
    target_path = "/" + target.strip("/") + suffix
    target_url = sitelib.SITE_BASE + target.strip("/") + suffix
    escaped_path = html.escape(target_path, quote=True)
    escaped_url = html.escape(target_url, quote=True)
    script_target = json.dumps(target_path)
    return """<!doctype html>
<html lang="en">
<head>
    <meta charset="utf-8">
    <meta name="robots" content="noindex">
    <meta http-equiv="refresh" content="0; url={target_path}">
    <link rel="canonical" href="{target_url}">
    <title>Page moved - Ze</title>
    <script>location.replace({script_target} + location.search + location.hash);</script>
</head>
<body>
    <p>This page moved to <a href="{target_url}">{target_url}</a>.</p>
</body>
</html>
""".format(
        target_path=escaped_path,
        target_url=escaped_url,
        script_target=script_target,
    )


def main():
    directory_redirects = page_registry.url_redirects()
    file_redirects = page_registry.file_redirects()
    for source, target in sorted(directory_redirects.items()):
        destination = GH_PAGES / source / "index.html"
        destination.parent.mkdir(parents=True, exist_ok=True)
        destination.write_text(redirect_html(target))
        destination.with_name("index.md").unlink(missing_ok=True)
    for source, target in sorted(file_redirects.items()):
        destination = GH_PAGES / source
        destination.parent.mkdir(parents=True, exist_ok=True)
        destination.write_text(redirect_html(target))
    print(
        "rendered %d legacy redirects"
        % (len(directory_redirects) + len(file_redirects))
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
