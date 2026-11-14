---
spec: terminal-demo-showcase
topic: Config Editor
---

## Before

The site showed one short recording per feature, and most of them started Ze
with a prepared configuration, so none showed a feature being switched on in a
router that was already running. The home page and the README opened on the
BGP dashboard recording.

A change to IRR filtering made on a running router (another as-set, another
IRR server, a new import filter on a peer) was accepted by the commit and then
not applied: the filter kept the configuration it had started with.

For contributors: each recording has a check that runs the same steps against
a live lab. A few of those checks could pass when Ze had not done what the
recording shows, and from 2026-11-08 the checks of the recordings that share
one lab could not run at all, because the lab started its second router in a
way a newer IPsec safeguard refuses.

## After

Eight topic recordings replace seven older ones: RPKI, IRR filtering, BFD, OSPF,
IPsec, VRRP interface tracking, traffic usage and commit confirmed. In each one
the configuration is loaded section by section into a running Ze through the
SSH editor and committed, then the recording shows the proof, a realistic
failure, and its diagnosis with Ze's own commands before the fix. Each sits on
its feature's guide page and in the demonstration gallery.

One continuous recording of 21 minutes 47 seconds builds all eight topics on a
single running router without restarting it. It is the recording on the home
page, and the README shows a screenshot that links to it.

A live change to IRR filtering is now applied by the commit.

These nine recordings were last made on 2026-10-28 and have not been made
again since. The scripts behind them changed a little afterwards: the
published continuous recording types `commit confirmed 8` where its script now
says `commit confirmed 15`. What was rerun on 2026-11-14 is the check behind
every recording on the site, twenty in all, against the current software, and
all twenty passed.

For contributors: those checks now fail when Ze does not do what the recording
shows. A link counts as up only when it has carrier, the commit-confirmed
check reads the committed configuration and not the preview, and a lab command
that hangs ends on a deadline that names it. Operators see no change in Ze
from these repairs.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
