"""Filesystem contracts for website sources and the generated Pages artifact."""

import os
import pathlib

def _repo_root() -> pathlib.Path:
    """The checkout, found by walking up rather than by counting directories.

    `SOURCE_ROOT` used to be this file's own parent's parent, which is true
    only while this file sits in `website/tools`. Every one of the 45 tools
    here derives its paths from it, so a move would have pointed the whole
    site build at the wrong tree, silently: nothing raises when a path is
    merely wrong.

    `ZE_REPO_ROOT` wins when set, which is how a container or a worktree names
    a tree it mounted elsewhere. Both markers are required because `go.mod`
    alone is not a checkout: a vendored module has one.
    """
    named = os.environ.get("ZE_REPO_ROOT")
    if named:
        return pathlib.Path(named).resolve()
    here = pathlib.Path(__file__).resolve()
    for parent in here.parents:
        if (parent / "go.mod").is_file() and (parent / "feature-gates.txt").is_file():
            return parent
    raise RuntimeError("cannot locate the Ze checkout from %s" % here)


HERE = pathlib.Path(__file__).resolve().parent
MAIN_REPO = pathlib.Path(os.environ.get("ZE_MAIN_REPO", _repo_root())).resolve()
# The site SOURCE tree, named from the checkout rather than from this file's
# position, so the tooling can move and still build the same website.
SOURCE_ROOT = (MAIN_REPO / "website").resolve()
OUTPUT_ROOT = pathlib.Path(
    os.environ.get("ZE_SITE_OUTPUT", MAIN_REPO.parent / "gh-pages")
).resolve()

if os.environ.get("ZE_SITE_ALLOW_IN_TREE") != "1" and (
    OUTPUT_ROOT == SOURCE_ROOT or SOURCE_ROOT in OUTPUT_ROOT.parents
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
    "assets/vendor/README.md",
    "assets/vendor/fonts/README.md",
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
