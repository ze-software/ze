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

**Stress test generator:** `test/stress/bgpgen.py` is a pure-stdlib BGP raw
UPDATE file generator (RFC 4271 / 4760 wire format) that replaced the
upstream scapy-based `bgpupdate` tool. It is ~500x faster (1M /24 prefixes
in ~1 s vs. ~500 s under scapy). There is no remaining scapy dependency in
the stress test path.

`test/stress/setup.py` no longer installs scapy; `test/stress/run.py` no
longer checks for it. If a new scenario needs features bgpgen does not
cover (MPLS labels, withdraw, ADD_PATH, multi-ASN paths), extend
`bgpgen.py` rather than reintroducing scapy.
