# Ruling: Ze always starts, and a service that cannot be served is not started

- **Date:** 2026-11-26
- **Owner:** Thomas Mangin
- **Context:** A web or looking-glass certificate name that did not resolve made `ze start` exit non-zero, and a looking glass with one `tls true` entry and no certificate to present did not start at all. A daemon that does not start brings up no networking, so the operator has no route to the box to correct the configuration that stopped it.
## Decision

Thomas's words, as voice-transcribed:

> "we should never not start ze as if we can not bring networking we can not fix things like web cert and the like so the only good answer is to disable the service badly configured and not start it"

The main session read it back as:

> "Ze always starts. A service whose configuration cannot be served is not started, and is reported with its fix by the log and ze doctor; it never stops ze from starting, because without networking the operator cannot reach the box to fix it."

He was then asked two questions:

- **Q1: what does a reload do with a config that cannot be served?** His answer: "reload stay refused". A reload whose config cannot be served is refused, and the running service keeps serving its current config.
- **Q2: a looking glass with one bad endpoint: "Should only that endpoint stay down while the others serve"?** His answer: "yes".

## What this requires

1. No service's configuration makes `ze start` exit or refuse to start. A service whose configuration cannot be served is not started, and ze runs with its other services.
2. Each service that is not started is reported with its fix: an error line in the log under a doctor code, and a finding of `ze doctor` under the same code.
3. A web server whose certificate name does not resolve is not started. Ze never serves the self-signed certificate in place of a named one.
4. The looking glass is judged one `server` entry at a time. An entry that cannot be served stays down, and the other entries serve: an entry that says `tls true` with no certificate to present, and every entry that asks for TLS when the named certificate does not resolve. An entry that says `tls false` serves.
5. A reload is unchanged: a reload whose config cannot be served is refused as a whole, and every running service keeps what it serves.

## What this supersedes

The ruling of 2026-11-23 (`plan/records/ruling-looking-glass-http-and-https.md`) says an entry that says `tls true` with no certificate "is refused, at start and at reload alike". The start half is superseded: at start that entry stays down and the others serve. The reload half stands.

## Follow-up answers, 2026-11-26

The main session then asked two questions the ruling left open.

1. A certificate name the configuration does not define (a typo) is refused
   by configuration validation before any service starts, so ze exits at
   "load config". Asked whether ze should (a) start with the block of the
   service that names it left out, that service down and reported, or (b)
   start on the last configuration that loaded.
2. The decision of 2026-11-23 in docs/architecture/core-design.md, "Design
   decisions", says a feature that programs only the Linux kernel on a
   daemon whose interfaces run on VPP "is refused at daemon start". Asked
   whether the new rule replaces it: (a) yes, ze starts, that feature stays
   off and ze doctor reports the mismatch, while commit and reload still
   refuse it; (b) no.

Thomas answered "1a 2a" (voice-transcribed).
