"""Single source for the Alpine virt ISO: version pin + durable, verified download.

Both QEMU producers import this so they cannot drift (bumping the version in one place used
to leave the other downloading the superseded ISO):
  - scripts/evidence/qemu-run.py  (host-arch VM for functional tests)
  - tools/kernel-builder/qemu-build.py  (target-arch VM that compiles the kernel)

The ISO lives in the durable cache (~/.cache/ze/alpine-iso), OUTSIDE tmp/, so it survives
`rm -rf tmp` and is shared across worktrees (AC-2). It is verified against Alpine's published
.sha256 on download and re-verified against a stored sidecar on cache hit, so a truncated or
substituted image is never served (AC-5/R-5). Spec: plan/learned/1173-relocate-scratch-and-cache.md.
"""

from __future__ import annotations

import hashlib
import os
import subprocess
import sys
from pathlib import Path

# One version pin, shared by both producers.
ALPINE_VERSION = "3.21"
ALPINE_MINOR = "3"


def durable_cache_dir() -> Path:
    """Match resolveCacheDir() in internal/appliance/cache.go: $XDG_CACHE_HOME/ze or ~/.cache/ze."""
    xdg = os.environ.get("XDG_CACHE_HOME")
    base = Path(xdg) if xdg else Path.home() / ".cache"
    return base / "ze"


def _sha256_file(path: Path) -> str:
    h = hashlib.sha256()
    with path.open("rb") as f:
        for chunk in iter(lambda: f.read(1 << 20), b""):
            h.update(chunk)
    return h.hexdigest()


def _published_sha256(url: str) -> str:
    """Fetch Alpine's published .sha256 and return the 64-hex digest, or fail loudly.

    Alpine publishes `<hex>  <name>` next to each release (verified 2026-07-16); the digest
    is field[0], exactly as internal/appliance/cache.go:downloadChecksum parses it.
    """
    result = subprocess.run(
        ["curl", "-fSL", url + ".sha256"],
        text=True,
        check=False,
        stdout=subprocess.PIPE,
        stderr=subprocess.DEVNULL,
    )
    if result.returncode != 0 or not result.stdout.strip():
        raise SystemExit(f"cannot fetch checksum: {url}.sha256")
    field = result.stdout.split()[0].strip().lower()
    if len(field) != 64 or any(c not in "0123456789abcdef" for c in field):
        raise SystemExit(f"malformed checksum for {url}.sha256: {result.stdout!r}")
    return field


def iso_name(arch: str) -> str:
    return f"alpine-virt-{ALPINE_VERSION}.{ALPINE_MINOR}-{arch}.iso"


def ensure_iso(arch: str) -> Path:
    """Return the verified Alpine virt ISO for `arch` from the durable cache.

    A cache hit re-verifies the ISO against a stored sidecar digest (offline, no network) so
    local corruption is caught; a fresh download is verified against Alpine's published
    .sha256 and only then atomically renamed into place. Fails loudly on any mismatch.
    """
    name = iso_name(arch)
    iso_dir = durable_cache_dir() / "alpine-iso"
    iso_dir.mkdir(parents=True, exist_ok=True)
    iso = iso_dir / name
    sidecar = iso.with_suffix(iso.suffix + ".sha256")
    url = (
        f"https://dl-cdn.alpinelinux.org/alpine/v{ALPINE_VERSION}/releases"
        f"/{arch}/{name}"
    )

    if iso.is_file() and sidecar.is_file():
        want = sidecar.read_text().split()[0].strip().lower()
        if len(want) == 64 and _sha256_file(iso) == want:
            return iso
        print(f"cached ISO {iso} failed integrity check; re-downloading", file=sys.stderr)
        iso.unlink(missing_ok=True)
        sidecar.unlink(missing_ok=True)

    expected = _published_sha256(url)
    print("Downloading Alpine virt ISO...", file=sys.stderr)
    part = iso.with_suffix(iso.suffix + ".part")
    result = subprocess.run(
        ["curl", "-fSL", "--progress-bar", "-o", str(part), url],
        text=True,
        check=False,
    )
    if result.returncode != 0:
        part.unlink(missing_ok=True)
        raise SystemExit(f"download failed: {url}")
    got = _sha256_file(part)
    if got != expected:
        part.unlink(missing_ok=True)
        raise SystemExit(f"checksum mismatch for {name}: got {got}, want {expected}")
    part.rename(iso)
    sidecar.write_text(f"{expected}  {name}\n")
    return iso
