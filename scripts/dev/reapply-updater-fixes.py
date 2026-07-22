#!/usr/bin/env python3
"""Re-apply local fixes to the vendored gokrazy updater after `go mod vendor`.

Run from the repo root after EVERY `go mod vendor`, not only after bumping
gokrazy/updater. `go mod vendor` rewrites the whole vendor tree from the module
cache, so bumping any unrelated dependency (a Dependabot fix on grpc, for
example) silently reverts these fixes too:

    go get <any-module>@<version>
    go mod vendor
    python3 scripts/dev/reapply-updater-fixes.py
    go test ./internal/appliance/...

TestUpdaterHardeningMarkersPresent (internal/appliance) is the backstop that
catches a forgotten run.

The upstream PR (scripts/dev/gokrazy-updater-upstream.patch) tracks the same
fixes. Once merged upstream, bump the dependency and delete both this script
and the patch file.
"""

from __future__ import annotations

import re
import shutil
import subprocess
import sys
from pathlib import Path

VENDOR = Path("vendor/github.com/gokrazy/updater/updater.go")
LOCAL = Path("internal/appliance/updater/updater.go")


def check_fix_applied(src: str, label: str, pattern: str) -> bool:
    """Return True if the fix is already present."""
    if re.search(pattern, src):
        print(f"  [ok] {label} -- already applied")
        return True
    return False


def apply_resp_body_close(src: str) -> str:
    """Add defer resp.Body.Close() after every resp, err := t.doer.Do(req)."""
    close_line = "\tdefer resp.Body.Close()\n"

    lines = src.splitlines()
    out: list[str] = []
    i = 0
    fixed = 0
    while i < len(lines):
        out.append(lines[i])
        # Match: resp, err := t.doer.Do(req)
        if "resp, err := t.doer.Do(req)" in lines[i]:
            # Check if next few lines already have resp.Body.Close()
            lookahead = "\n".join(lines[i + 1 : i + 5])
            if "resp.Body.Close()" not in lookahead:
                # Find the `if err != nil { return err }` block, insert after it
                j = i + 1
                while j < len(lines) and "}" not in lines[j]:
                    out.append(lines[j])
                    j += 1
                if j < len(lines):
                    out.append(lines[j])  # the closing }
                    out.append(close_line.rstrip())
                    i = j + 1
                    fixed += 1
                    continue
        i += 1

    if fixed:
        print(f"  [fix] Added {fixed} defer resp.Body.Close() calls")
    else:
        print("  [ok] All resp.Body.Close() calls present")
    # Rejoining split lines drops the terminator, so re-add a trailing newline
    # whenever the source had one. A Go file without it is not gofmt-clean.
    joined = "\n".join(out)
    return joined + "\n" if src.endswith("\n") else joined


def apply_http_nobody(src: str) -> str:
    """Replace nil body in requests with http.NoBody."""
    count = 0
    # Match NewRequestWithContext(..., nil) but not NewRequestWithContext(..., http.NoBody)
    pattern = r"(http\.NewRequestWithContext\([^)]+,\s*)nil\)"

    def replacer(m: re.Match) -> str:
        nonlocal count
        count += 1
        return f"{m.group(1)}http.NoBody)"

    result = re.sub(pattern, replacer, src)
    if count:
        print(f"  [fix] Replaced {count} nil -> http.NoBody")
    else:
        print("  [ok] All requests use http.NoBody")
    return result


def apply_slices_contains(src: str) -> str:
    """Replace manual loop in Supports() with slices.Contains."""
    old_pattern = (
        r"func \(t \*Target\) Supports\(feature ProtocolFeature\) bool \{\n"
        r"\tfor _, f := range t\.supports \{\n"
        r"\t\tif f == string\(feature\) \{\n"
        r"\t\t\treturn true\n"
        r"\t\t\}\n"
        r"\t\}\n"
        r"\treturn false\n"
        r"\}"
    )
    new_code = (
        "func (t *Target) Supports(feature ProtocolFeature) bool {\n"
        "\treturn slices.Contains(t.supports, string(feature))\n"
        "}"
    )
    if re.search(old_pattern, src):
        src = re.sub(old_pattern, new_code, src)
        # slices.Contains needs the "slices" import. Insert it after the
        # "net/url" anchor; the final gofmt pass (see main) only re-sorts
        # imports, it CANNOT add a missing one, so we must add it here and
        # fail loud if the anchor is gone -- otherwise the package would emit
        # slices.Contains with no import and fail to compile.
        if '"slices"' not in src:
            before = src
            src = src.replace('"net/url"\n', '"net/url"\n\t"slices"\n', 1)
            if src == before:
                raise RuntimeError(
                    'could not insert "slices" import: anchor \'"net/url"\' not '
                    "found in the import block. The vendored import block changed "
                    "upstream; update the anchor in apply_slices_contains()."
                )
        print("  [fix] Replaced Supports() loop with slices.Contains")
    else:
        print("  [ok] Supports() already uses slices.Contains")
    return src


def apply_limitreader(src: str) -> str:
    """Cap unbounded io.ReadAll from remote device responses."""
    fixes = 0

    # StreamTo: remoteHash read
    old = "remoteHash, err := io.ReadAll(resp.Body)"
    new = "remoteHash, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))"
    if old in src:
        src = src.replace(old, new, 1)
        fixes += 1

    # requestFeatures: body read (after the status check)
    # Only replace the one inside requestFeatures, not all occurrences
    feat_idx = src.find("func (t *Target) requestFeatures")
    if feat_idx >= 0:
        tail = src[feat_idx:]
        old_feat = "\tbody, err := io.ReadAll(resp.Body)\n"
        new_feat = "\tbody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))\n"
        if old_feat in tail and new_feat not in tail:
            src = src[:feat_idx] + tail.replace(old_feat, new_feat, 1)
            fixes += 1

    if fixes:
        print(f"  [fix] Added {fixes} io.LimitReader caps")
    else:
        print("  [ok] All reads are capped")
    return src


def gofmt(path: Path) -> None:
    """Normalize the rewritten file with gofmt so import order and the trailing
    newline stay canonical regardless of how the text transforms above mangle
    formatting. No-op (with a warning) if gofmt is not on PATH."""
    tool = shutil.which("gofmt")
    if tool is None:
        print("  [warn] gofmt not found on PATH -- skipping format normalization")
        return
    subprocess.run([tool, "-w", str(path)], check=True)
    print("  [ok] gofmt normalized")


def main() -> int:
    if not VENDOR.exists():
        print(f"error: {VENDOR} not found (run go mod vendor first)", file=sys.stderr)
        return 1

    print(f"Reading {VENDOR}")
    src = VENDOR.read_text()
    original = src

    print("Applying fixes:")
    src = apply_resp_body_close(src)
    src = apply_http_nobody(src)
    src = apply_slices_contains(src)
    src = apply_limitreader(src)

    if src == original:
        print("\nNo changes needed (all fixes already upstream or applied).")
        return 0

    VENDOR.write_text(src)
    gofmt(VENDOR)
    print(f"\nWrote {VENDOR}")
    print("Run: go test ./internal/appliance/...")
    return 0


if __name__ == "__main__":
    sys.exit(main())
