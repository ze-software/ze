#!/usr/bin/env python3
"""Shared kernel-source URL/series construction.

The cdn.kernel.org base URL and the `vN.x` series string are built in exactly
one place (AC-11). Both build.py (which downloads inside the builder) and
qemu-build.py (which pre-downloads on the host because the VM may lack network)
import these helpers instead of re-deriving the URL.
"""

from __future__ import annotations

KERNEL_URL = "https://cdn.kernel.org/pub/linux/kernel"


def series(version: str) -> str:
    """Major-version series directory, e.g. "7.1.1" -> "v7.x"."""
    return f"v{version.split('.', 1)[0]}.x"


def tarball_name(version: str) -> str:
    return f"linux-{version}.tar.xz"


def tarball_url(version: str) -> str:
    return f"{KERNEL_URL}/{series(version)}/{tarball_name(version)}"
