# A value ze publishes drifts from the behavior it describes

A protocol makes ze announce what it is going to do: a refresh period, a hold
time, a hello interval, a window. The peer sizes its own timers from the
announcement. Two halves therefore have to agree: the number ze puts on the wire,
and the schedule ze actually keeps.

The halves are usually written in different files and at different times, so a
change to one is not a change to the other. Worse, the announced number is
commonly stamped at several sites: once when the state is created, once when it
is refreshed, once when a message is relayed. A fix at the site the bug report
names leaves the others announcing the old value.

The tell is a config leaf whose name is a duration, read in more than one place,
where one of those places drives a ticker and the others fill a wire field. Trace
the leaf to EVERY site before you call the fix complete, and ask at each one
whose schedule the number describes: this node's, or the sender's.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-09-05 | mpls-10-rsvp-te-reload-completeness | the RSVP-TE `refresh-period` leaf, read by `runRefreshLoop` (`internal/plugins/rsvpte/register.go`), by `buildPath` through `psb.RefreshPeriod` (`build.go`), and by `handlePathTransit` (`engine.go`) | The spec was written as one defect, a ticker that kept its launch-time period across a reload. Reading the producers showed three sites, and a fix at the ticker alone would have left the wire lying: `buildPath` advertised the period stamped when the LSP was signaled, and a transit node advertised THIS node's period downstream although it relays a PATH only when one arrives, so the real downstream cadence is the sender's. The receive direction had the same defect from the other end: an egress PSB took its lifetime from the LOCAL period, so shortening `refresh-period` shortened the lifetime of state a NEIGHBOR refreshes and the next cleanup tick deleted a live reservation. RFC 2205 Section 3.7 is what makes all four one defect: "Each Path or Resv message carries a TIME_VALUES object containing the refresh time R used to generate refreshes. The recipient node uses this R to determine the lifetime L of the stored state created or refreshed by the message" | fixed at every site in commit 838416efc: the tick bodies read the live config through `liveConfig`, `refreshPaths` stamps the live period on an ingress PSB, `handlePathTransit` relays the received period, and `receivedRefreshPeriod` (`engine.go`) derives the lifetime of received state from the sender's TIME_VALUES, a field that was decoded and used nowhere. The decision is recorded in `docs/architecture/rsvpte/mpls-rsvp-te.md` under "the lifetime of received state follows the sender's period" |
