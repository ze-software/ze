# A setting is changed in one step, past the rate its consumer allows

A value ze publishes tells another party how to behave: a refresh period, a hold
time, a keepalive interval, an advertised window. The other party derives a
deadline from the last value it received. When an operator commits a new value,
ze adopts it at once, because a config commit reads as immediate and the code
that applies it sees only the new number.

The protocol usually limits how fast such a value may move, and the limit exists
for the message that announces the move. That single message is the only warning
the consumer gets. When it is lost, the consumer still holds a deadline computed
from the old value while ze has already moved to the new one, and the state dies
between them.

The tell is an adopt-on-commit path with a bound on the VALUE and no bound on the
RATE OF CHANGE. Look for a validator that checks a range, a ticker that is reset
to the new value, and no memory of the previous one.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-09-05 | mpls-10-rsvp-te-reload-completeness | `adoptedRefreshPeriod` (`internal/plugins/rsvpte/register.go`), the RSVP-TE refresh cadence a config commit installs | RFC 2205 Section 3.7 item 5 says "When R is changed dynamically, there is a limit on how fast it may increase. Specifically, the ratio of two successive values R2/R1 must not exceed 1 + Slew.Max", and fixes the value: "Currently, Slew.Max is 0.30". Ze adopts any period the YANG range allows in ONE step, so a commit from 10 to 300 seconds multiplies R by 30. The tick refreshes before it re-periods, so one message does carry the new period to the neighbor at the old cadence, and a neighbor that receives it recomputes its lifetime in time. The slew limit exists for the case where that message is LOST: the same section pairs it with "With K = 3, one packet may be lost without state timeout while R is increasing 30 percent per refresh cycle". After a 30x jump the neighbor times out at 3 times 10 seconds while ze's next refresh is 300 seconds away, and it deletes a reservation ze still holds. Found at the closure review of the spec that made R changeable at runtime at all; before it, a period change needed a restart | NOT FIXED, and the shape is an owner decision under `ai/rules/rfc-compliance.md`. The repair is not local to this function. Conformance needs the ADVERTISED period and the CADENCE to move together, and they are read from different places: the cadence is the loop's own variable, while the advertised value comes from `e.cfg().RefreshPeriod` at `sendResv` (`engine.go`), at `setupTunnel` and at `reroute`, plus the PSB stamp in `refreshPaths`. So the engine has to hold a RUNNING period beside its configured TARGET period, and every advertisement site has to read the running one. That also changes what a commit means to an operator: a raised `refresh-period` would reach its configured value over about 13 refresh cycles rather than at the next tick, which is a user-visible semantic and the second half of the decision. The sibling row in `plan/journal/bound-too-small-for-its-own-burst.md` holds the same subsystem's other open RFC 2205 timing rule, the item 2 lifetime floor |
