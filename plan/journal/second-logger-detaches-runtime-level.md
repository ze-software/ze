| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-10-28 | - (terminal-demo showcase, found, not caused) | panic guards calling `slogutil.Logger(<subsystem>)` a second time: bmp, dhcpserver, ldp, tftpserver, rsvpte, vrrp, vrrp/transport (`panic_guard.go`) | `slogutil.Logger` stores a fresh LevelVar per call, so the plugin's own logger keeps a level `request log level` no longer reaches; for bgp.rpki the demo never saw "rtr: sync complete" | open: `slogutil.Logger` reuses and updates the stored LevelVar; rpki fixed locally by a forwarding handler |
