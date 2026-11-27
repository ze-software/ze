---
spec: doctor-daemon-route
topic: Diagnostics
---

## Before

Running `show doctor` on a daemon with multiple plugins declaring runtime health checks executed every plugin check serially, each under a 5 s timeout, so one hung plugin delayed every plugin behind it and checks with equal order appeared in non-deterministic order. Running `ze doctor` locally against a daemon whose SSH listener accepted TCP connections but stalled during key exchange, authentication, or command execution could hang indefinitely instead of degrading to local checks.

## After

`show doctor` now runs plugin health checks concurrently across plugins (while keeping checks of the same plugin serial in declared order, each with its own fresh 5 s timeout from pickup) and orders all plugin findings deterministically by `(order, plugin name, check name)`. Concurrent `show doctor` calls queue per plugin up to 16 requests with a 5 s queue-wait limit, reporting `doctor-plugin-unreachable` with `ze request plugin restart <plugin>` if a plugin is busy or stopped while discarding expired requests unrun. Local `ze doctor` now bounds its SSH ask to the running daemon to 22 s across TCP connect, SSH handshake, authentication, and command execution; when the deadline expires, `ze doctor` falls back cleanly to local readiness checks with one `doctor-runtime-not-checked` warning naming the daemon address, command, timeout bound, and `ze cli --user <name> -c "show doctor"` fix.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
