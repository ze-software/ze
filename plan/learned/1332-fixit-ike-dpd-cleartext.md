# 1332 -- Dead Peer Detection sent its probe in cleartext

## Context

`sendDPD` (`internal/component/ike/engine/dpd.go`) built a `wire.Message` carrying a
`Header` and nothing else, wrote those 28 octets, and sent them. RFC 7296 Section 1.4
describes the liveness check as an INFORMATIONAL request with no payloads other than the
empty Encrypted payload the syntax requires, so a bare header is not an INFORMATIONAL
request at all. A conforming peer drops it, every probe went unanswered, and `timedOut`
declared a healthy peer dead at each DPD timeout. Configuring the feature broke the tunnel
the feature exists to protect. It stayed latent only because `newDPDState` returns nil at
`Interval == 0`, so an operator who never configured DPD never met it.

## Decisions

- Built the probe through `buildEncryptedMessageEx(sa, nil, ...)` over hand-building an SK
  payload inside `sendDPD`: one producer serves every encrypted INFORMATIONAL, and
  `handleInformationalOwned` already passes a nil chain the same way. The nil argument IS
  the empty payload list Section 1.4 describes.
- Extracted the missing requirement row in the same spec, over fixing the code alone. The
  gate's blindness is why the defect survived, and a fix with no row leaves the next
  regression equally invisible.
- Kept the request-window reservation where it already sat, over moving it after the build.
  A build failure releases it instead.

## Consequences

- `make ze-rfc-check` verifies that every requirement LISTED in a summary is covered. It
  cannot verify that the summary lists every requirement. A checklist's silence is not
  conformance, and leaving an obligation unextracted is the cheapest way to keep a
  violation invisible.
- The row that looked like it covered DPD, `RFC7296-2.4-1`, governs the ANSWER to an empty
  INFORMATIONAL request. Proximity of subject is not coverage: read what a row's text
  actually obliges before treating it as the guard for a nearby behaviour.
- The receive half needed no change. `handleOwnedInbound` authenticates before `matchesProbe`
  is consulted, so a replayed or out-of-window answer cannot mask a dead peer.
- A bare-header probe arriving FROM a peer is answered with silence, and that is conformant.
  `decryptAndParse` returns "no SK payload", which is not `errInnerParse`, and `errInnerParse`
  is the marker RFC 7296 Section 3.10.1's error-notification precondition rides on.

## Gotchas

- The new row could not take the id the plan reserved for it.
  `plan/learned/1313-rfcgate-1b-rfc7296-pilot.md` had planned `RFC7296-1.4-2`, but
  `check_id_allocation` (`scripts/dev/rfc_requirements.py`) refuses a new row at or below a
  section's high-water mark, because a returning id cannot be told apart from a text
  correction. Section 1.4's mark was 4, so the row is `RFC7296-1.4-5`. Allocate above the
  mark; do not reuse a planned id without checking it.
- The spec's metadata named a deferral shard that did not exist, so its one real deferral was
  invisible to `/ze-close`. The deferred work was stated only in TDD Test Plan prose. A
  metadata row naming a shard is a claim, so check the file is there.
- `sendDPD` with a nil transport still reserves the window, sets `awaitReply`, and stores no
  datagram. `serviceRequestWindow` skips a hold covered by a probe, so nothing frees it and
  only the liveness timeout ends it. Latent: `maintainSA` always passes a real transport.
- Row `RFC7296-2.4-2` (liveness checks are demand-driven, not periodic) is a SHOULD that Ze
  does not meet and does not tag. `handleDPDResponse` clears `awaitReply` but never resets
  `dpd.lastSent`, so probes stay on a fixed period whatever authenticated traffic arrives.
  Untouched by this work and still open.

## Files

- `internal/component/ike/engine/dpd.go`
- `internal/component/ike/engine/rfc7296_dpd_test.go`
- `rfc/short/rfc7296.md`
- `plan/learned/1313-rfcgate-1b-rfc7296-pilot.md`
- `plan/deferrals/fixit-ike-dpd-cleartext.md`
