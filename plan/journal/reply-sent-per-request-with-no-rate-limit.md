# A reply sent for each request, with no rate limit

A protocol that answers a solicitation puts the send rate under the control of
whoever sends the solicitations. The specification that defines the reply
usually states a minimum delay between two consecutive replies, and a random
delay before a solicited one, for exactly that reason. Code written from the
message layout alone carries the fields and omits both timers, because neither
appears in the packet.

The tell is a select case that answers an inbound channel by sending at once,
with no timestamp of the previous send anywhere in the function. Ask what an
external party can make this loop do if it sends one request per millisecond.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-08-29 | - | l2tp ppp Router Advertisement sender | `raSenderLoop` (`internal/component/l2tp/ppp/ra_send.go`) answers every Router Solicitation coalesced onto `rsCh` with an immediate `sender.send`, and in the initial burst it does so directly after the advertisement at the top of the loop body. RFC 4861 Section 6.2.6 states two MUST-level obligations that neither path meets: "Router Advertisements sent in response to a Router Solicitation MUST be delayed by a random time between 0 and MAX_RA_DELAY_TIME seconds", and "consecutive Router Advertisements sent to the all-nodes multicast address MUST be rate limited to no more than one advertisement every MIN_DELAY_BETWEEN_RAS seconds". MAX_RA_DELAY_TIME is 0.5 seconds and MIN_DELAY_BETWEEN_RAS is 3 seconds, both Section 10. So a subscriber that sends Router Solicitations in a loop gets one multicast advertisement for each one. RFC 4861 Section 6.2.4 also asks the unsolicited interval to be randomized between MinRtrAdvInterval and MaxRtrAdvInterval, and the loop uses a fixed ticker. Found while adding the Section 6.2.5 cease advertisement to the same function | FIXED 2026-08-29 under `plan/spec-ppp-ra-refinements.md` phase 2, AC-5 to AC-9. `raSchedule` (`internal/component/l2tp/ppp/ra_schedule.go`) holds the Section 6.2.6 algorithm on an injected `clock.Clock`: the time of the last multicast send, a random delay in [0, MAX_RA_DELAY_TIME] drawn once per burst and measured from the FIRST solicitation, the MIN_DELAY_BETWEEN_RAS floor, and the Section 6.2.4 randomized interval with the MAX_INITIAL_RTR_ADVERT_INTERVAL cap. `raSenderLoop` now asks the schedule how long to wait instead of answering `rsCh` at once. The arithmetic lives in `internal/core/ndp/schedule.go`, shared with the LAN sender in `internal/plugins/iface/ra`, which carried the same code and was missing the same parenthetical; it got the guard too |
