---
spec: managed-server-hardening
topic: Fleet
feature: fleet-management
found-by: review
---

## Before

A managed device that ran its hub's configuration could lose a local change.
When its hub pushed a new configuration and a local commit landed on the
device while that configuration was being fetched (the revert of an
unconfirmed `commit confirmed`, for example), the push was applied over the
local commit and the change was gone. Only a revert decided on the hub was
checked against such a commit.

A hub that kept the connection open but never answered a configuration fetch
froze the device's connection to it: the device waited for the answer for
ever and handled no later change from the hub.

At first start (`ze start` after `ze init --managed`), `--connect-timeout`
(`ze.managed.connect.timeout`, 5 seconds by default) bounded the whole first
exchange, the fetch of the configuration included, so a large first
configuration on a slow link could never be fetched.

## After

Every configuration a hub pushes is applied only if the device still runs
the configuration it ran when it asked for the push. When a local commit
landed in between, the device keeps it, fetches once more, and holds: it
sends its hub a copy of what it runs and reports `doctor-managed-diverged`,
as for any other local change. If a local commit lands during that second
fetch too, the device keeps it, logs one line at Info, and reconnects about a
second later to decide again.

Each configuration fetch now waits at most 30 seconds for its answer, at
connect, after a change from the hub, and at first start. A device whose
fetch runs out of time keeps the configuration it runs. After a change from
the hub it logs `doctor-hub-unreachable` and keeps the connection; at connect
it reconnects; at first start it stops with `context deadline exceeded` and
writes no configuration. 30 seconds carry the largest configuration a hub
sends (16 MiB) on a link of about 4.5 Mbit/s. When the hub announces another
change while a fetch has run for more than 10 seconds, the hub drops the
connection, and the device fetches again when it reconnects.

`--connect-timeout` now bounds only reaching the hub: the connection, the
check of the hub's certificate and, at first start, the authentication. The
first fetch has its own 30-second bound, so a first start against a hub that
never answers ends after 30 seconds, not 5.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
