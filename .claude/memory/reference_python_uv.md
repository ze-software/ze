---
name: Python deps via uv, not pip3
description: Project installs Python dependencies with `uv` (run/pip), not `pip3 --break-system-packages`. User confirmed this after hitting friction with the stress-test setup script.
type: reference
originSessionId: e2f0855a-d034-476f-81b6-5a035dc15c6e
---
Python dependencies in ze are installed via `uv`. The Makefile uses
`uv run --with <pkg>` for ExaBGP-compat tests (see `Makefile:244,286`).

`pip3 install --break-system-packages <pkg>` fails on the user's system
(PEP 668 externally-managed environment). User had to install scapy
manually via `uv` to work around it.

**Outlier to watch:** `test/stress/setup.py:146-149` still calls
`pip3 install --break-system-packages scapy`. It is inconsistent with
the rest of the project and will fail on modern systems. If asked to
touch stress-test setup, propose migrating that call to `uv` (but verify
first whether `bgpupdate` invokes scapy from system python or can be
wrapped in `uv run` -- the constraint was not confirmed).
