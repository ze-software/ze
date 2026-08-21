---
kind: directive
level: SHOULD
stage:
---
**SHOULD prefer `run_rs_observer` for any route-server `.ci`.** A synchronous
`show bgp` dispatch can stall on a TLS read while the engine forwards.
`run_rs_observer` waits on pushed events and shuts down fire-and-forget.
