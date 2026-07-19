---
name: Python deps via uv, not pip3
description: Project installs Python dependencies with `uv run --with`. `pip3 install --break-system-packages` is forbidden (PEP 668). No scapy dependency remains.
type: reference
originSessionId: e2f0855a-d034-476f-81b6-5a035dc15c6e
---
Python dependencies in ze are installed via `uv run --with <pkg>` (see
`Makefile:244,286` for the ExaBGP-compat test harness). `pip3 install
--break-system-packages` fails on modern systems (PEP 668 externally-managed
environment) and must not be introduced.

**Stress test path:** the BGP UPDATE stream for stress scenarios is generated
in memory by the Go injector (`ze-test peer --mode inject`), not a Python tool.
The earlier stdlib byte-level oracle (which had replaced the upstream
scapy-based `bgpupdate` tool) has been removed now that the Go builder is
trusted. `test/stress/` holds the Python harness (`test/stress/harness.py`,
`test/stress/run.py`, `test/stress/setup.py`, `test/stress/scenarios/`); no
scapy dependency remains in the stress path. Extend the Go injector for new
scenarios rather than reintroducing scapy.
