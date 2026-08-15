#!/usr/bin/env -S uv run --with markdown python3
"""Batch-render main repo docs/*.md files into site-shell-wrapped pages.

Usage:
    tools/render-docs.py

Reads tools/page_registry.py DOCS_MANIFEST, renders each registered
../main/docs/<path>.md to the destination selected by
page_registry.docs_dest_rel_for(), and computes the relative --root depth from
that destination automatically. Most pages mirror the source below docs/;
lookup pages can use DOCS_DEST_OVERRIDES to live under reference/<type>/.
Re-run this batch when a registered source changes upstream, or use
render-doc.py to render one file.

Add a new doc to page_registry.DOCS_MANIFEST (path relative to the main repo's
docs/, no extension logic, just the .md path) and re-run to publish it.
"""

import json
import pathlib
import subprocess
import sys
import tempfile

import page_registry

HERE = pathlib.Path(__file__).resolve().parent
GH_PAGES = page_registry.GH_PAGES
MAIN_DOCS = (GH_PAGES.parent / "main" / "docs").resolve()


def main():
    render_doc = HERE / "render-doc.py"
    link_manifest = page_registry.docs_link_manifest()

    # The link manifest is written to a system temp file (not the tools/ source
    # dir) and cleaned up by the context manager on any exit, so a crash never
    # leaves a stray tools/tmp*.json behind. The child render-doc.py processes
    # read it by path while this `with` block keeps it on disk.
    with tempfile.NamedTemporaryFile("w", suffix=".json") as f:
        json.dump(link_manifest, f)
        f.flush()
        manifest_path = pathlib.Path(f.name)

        failures = []
        for doc_path, cat in page_registry.DOCS_MANIFEST.items():
            source = MAIN_DOCS / doc_path
            if not source.exists():
                failures.append("missing source: %s" % source)
                continue
            dest_rel = page_registry.docs_dest_rel_for(doc_path)
            dest = GH_PAGES / dest_rel
            root = page_registry.page_root_for_dest(dest_rel)
            cmd = [
                sys.executable,
                str(render_doc),
                str(source),
                str(dest),
                "--root",
                root,
                "--desc",
                "Ze documentation: %s" % doc_path,
                "--manifest",
                str(manifest_path),
                "--doc-rel",
                doc_path,
                "--dest-rel-dir",
                page_registry.docs_dest_rel_dir_for(doc_path),
            ]
            if cat:
                cmd += ["--cat", cat]
            result = subprocess.run(cmd, capture_output=True, text=True)
            if result.returncode != 0:
                failures.append("%s: %s" % (doc_path, result.stderr.strip()))
            else:
                print(result.stdout.strip())

    if failures:
        print("\n%d failure(s):" % len(failures), file=sys.stderr)
        for f in failures:
            print("  %s" % f, file=sys.stderr)
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
