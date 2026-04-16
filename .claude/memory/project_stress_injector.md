---
name: Stress test injector is in-memory Go
description: BGP UPDATE stress data is generated in-memory in Go and streamed through ze-test peer, not via a Python file + bngblaster pipe.
type: project
originSessionId: e2f0855a-d034-476f-81b6-5a035dc15c6e
---
Stress-test architecture decision (2026-04-16): the BGP UPDATE stream
for scenarios 01-05 is generated **in memory in Go** inside `ze-test
peer --mode inject` and streamed directly over the TCP socket after
the OPEN handshake. No file on disk, no external injector.

**Why:** Previous path was Python (scapy, then stdlib `bgpgen.py`)
writing a raw-update file that BNG Blaster replayed. scapy was slow
(~500 s for 1M /24 routes); even the stdlib generator still paid a
filesystem round-trip and the bngblaster orchestration layer. All
the intermediate hops add latency, dependencies, and moving parts
with no value for profiling ze's receive path.

**How to apply:** For any new stress scenario, extend the Go injector
(pool-friendly byte builder + single pre-allocated buffer + single
TCP writer with keepalive goroutine) rather than adding flags to
`bgpgen.py` or re-introducing bngblaster orchestration. bgpgen.py
is kept short-term as a known-good oracle for byte-level diffing;
once the Go builder is trusted across all scenarios, both the
Python script and BNG Blaster are on the chopping block.
