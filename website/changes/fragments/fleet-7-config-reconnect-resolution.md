---
spec: fleet-7-config-reconnect-resolution
topic: Fleet
feature: fleet-management
---

## Before

A managed device whose configuration was changed locally, while it could not
reach its hub, held: it ran its own configuration and applied nothing the hub
sent. The hub could not see what the device ran. Its `show doctor` reported
the device as having refused a configuration, and the hold ended only by hand:
import the device's configuration on the hub, or leave the fleet on the
device, roll back and rejoin.

`ze config diff` and `show config diff` compared only the BGP part of two
configurations, so two files that differed elsewhere, such as in a hub client
address, answered "No differences". A changed md5 password of a dynamic BGP
group was printed in the diff.

## After

A holding device sends its hub a copy of the configuration it runs, up to
1 MiB. On the hub, `show fleet divergence list` names every diverged client
with its device and hub versions and when the device's configuration was
written, and `show fleet divergence name <client>` shows what differs, with
every secret masked and the number of changes.

The hub's operator ends the hold from the hub:

- `update fleet divergence name <client> adopt version <hash>` keeps the
  device's change: it becomes the hub's configuration for that client.
- `request fleet divergence name <client> revert version <hash> hub-version
  <hash>` discards it: the device applies the hub's configuration. The revert
  is bound to the two versions the operator saw, so a newer local edit, or a
  hub change made after the show, makes it refuse or void rather than apply
  something nobody looked at.

Both run on a running hub, from any command surface, and are recorded in its
audit log. `ze doctor` on the hub reports `doctor-managed-client-diverged`
with the exact commands, and reports a copy left for a client the hub no
longer serves. `show fleet status` on the device gains `held-by-hub` and
`revert-offered`.

`ze config diff` and `show config diff` now compare every part of the
configuration, and mask every secret, including a dynamic group's password.

## Corrects

none

## Exposure

`ze config diff` and `show config diff` printed the md5 password of a
dynamic BGP group when it changed between the two configurations. Other
secrets were masked.

## Upstream

none

## Asked by

none
