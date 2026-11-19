---
spec: l2tp-session-identity-and-publish-stall
topic: L2TP
feature: l2tpv2-bng
found-by: review
---

## Before

On an LNS with more than one tunnel, a session ID identified a session in one
tunnel only, but several paths treated it as unique. A subscriber's route and
event history could be withdrawn or merged when a session on another tunnel or
listener held the same ID. `clear l2tp session id <id>`, a RADIUS
Disconnect-Request, a session timeout or an idle timeout could end that other
subscriber. A late PPP event of an ended session could act on the new session
that drew its ID again. While a RADIUS accounting server did not answer, every
tunnel stopped handling control packets for up to 10 seconds per session that
ended, so healthy tunnels could miss their hellos and time out.

## After

Each session has its own route record and event history, keyed by an identity
minted once when the session is admitted. A PPP event reaches only the session
it was made for. `clear l2tp session id <id>` and the session `show` commands
accept `tunnel-id`, `listener` and `expected-id`. An ID held by more than one
session is refused, with nothing done, and the refusal lists the qualified
command for each session. A Disconnect-Request and both session timers end only
the session they name. The Accounting-Stop is sent by the session's own
accounting task, after its Start and Interim-Updates, so a silent RADIUS server
delays only that record. A removed session whose cleanup is not confirmed keeps
its tunnel's admission slot, and `ze doctor` reports the slots held this way.

## Corrects

none

## Exposure

none

## Upstream

none

## Asked by

none
