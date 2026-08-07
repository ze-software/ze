---
kind: directive
level:
stage:
---
**Prefer `run_rs_observer` for any route-server `.ci`.** The old copy-pasted
`all_peers_eor_sent` poll drove synchronous `show bgp summary` dispatch RPCs whose
30s TLS read could stall under load while the engine forwarded fine, stranding the
shutdown until the outer timeout killed ze. `run_rs_observer` waits on pushed events
instead (no request/response to stall on) and shuts down fire-and-forget.
