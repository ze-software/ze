---
spec: doctor-coverage-1-audit-and-gate
topic: Diagnostics
feature: system-readiness
---

## Before

`ze doctor` run on the box answered only from its own process, so it never saw
the running daemon's live state: a check of that state either did not run or
read an empty copy as healthy, and nothing in the answer said so. A finding
told the operator what was wrong, with any fixing command buried in the
message text. A problem that came and went, such as a peer reset that
recovered, showed in `show errors` or `show warnings` but never in
`show doctor`.

## After

Each `ze doctor` and `show doctor` finding can carry the command that fixes
it, printed on its own `fix:` line under the message and carried as the
`fix` key in JSON. Checks of live daemon state run inside the daemon, through
`show doctor`. A local `ze doctor` first asks the running daemon for its
answer over SSH and prints that. When no daemon answers, it runs its own
checks and adds a `doctor-runtime-not-checked` warning that says why (no
daemon listening, no credentials to log in, a config file the daemon does not
read) and names the command that gets the daemon's answer. `show doctor` also
lists the report bus's recent error events and active warnings as warning
rows, one per source, code and subject, without making the box read not
ready.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
