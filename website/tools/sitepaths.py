"""Filesystem contracts for website sources and the generated Pages artifact."""

import os
import pathlib

HERE = pathlib.Path(__file__).resolve().parent
SOURCE_ROOT = HERE.parent.resolve()
MAIN_REPO = pathlib.Path(os.environ.get("ZE_MAIN_REPO", SOURCE_ROOT.parent)).resolve()
OUTPUT_ROOT = pathlib.Path(
    os.environ.get("ZE_SITE_OUTPUT", MAIN_REPO.parent / "gh-pages")
).resolve()

if (
    os.environ.get("ZE_SITE_ALLOW_IN_TREE") != "1"
    and (OUTPUT_ROOT == SOURCE_ROOT or SOURCE_ROOT in OUTPUT_ROOT.parents)
):
    raise RuntimeError(
        "ZE_SITE_OUTPUT must be outside the website source tree: %s" % OUTPUT_ROOT
    )

_SOURCE_ONLY_DIRS = (
    ".claude",
    ".github",
    "assets/css",
    "assets/js",
    "blog/posts",
    "changes/posts",
    "presentations/tools",
    "tools",
)
_SOURCE_ONLY_FILES = {
    ".gitignore",
    "AI.md",
    "CACHEDIR.TAG",
    "compare/bgp.md",
    "compare/comparison.md",
    "compare/nos.md",
    "contribute/contribute.md",
    "contribute/guide.md",
    "docs/docs.md",
    "faq/faq.md",
    "license/license.md",
    "quality/browser-editor.md",
    "quality/functional-ci.md",
    "quality/qemu-interop-release.md",
    "quality/quality.md",
    "quality/unit-fuzz-mutation.md",
    "quality/verify-debugging.md",
    "roadmap/roadmap.md",
    "talks/linx-2026-06/update.sh",
    "talks/netmcr-2026-04/update.sh",
    "update-website.sh",
}


def is_source_only(path):
    """Return whether a source-relative path must be absent from deployment."""
    rel = pathlib.PurePosixPath(str(path).replace(os.sep, "/")).as_posix().strip("/")
    if rel in _SOURCE_ONLY_FILES:
        return True
    return any(rel == root or rel.startswith(root + "/") for root in _SOURCE_ONLY_DIRS)
