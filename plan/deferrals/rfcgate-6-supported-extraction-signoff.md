# Deferrals -- spec-rfcgate-6-supported-extraction-signoff

Source: `plan/spec-rfcgate-6-supported-extraction-signoff.md`. Format: `ai/rules/planning.md`.

The spec's own findings live in its `## Walk Findings (AC-8)` section, not here. This shard
carries only what leaves the spec: work a walk uncovered that this spec does not do, and the
decisions an owner owes.

| Date | Source | What | Reason | Destination | Status |
|------|--------|------|--------|-------------|--------|
| 2026-08-31 | spec-rfcgate-6, rfc5176 walk | RFC 5176 Section 3.4: the Message-Authenticator and Request Authenticator computations are inverted in both directions, so no conformant Dynamic Authorization Client can authenticate to Ze | NOT deferred. `ai/rules/rfc-compliance.md` makes a reachable conformance fix something to do and report, never to ask about, and rung 2 of `ai/rules/rule-precedence.md` puts that above this spec's scope. Verified at both producers (`VerifyMessageAuthenticator` and `VerifyCoARequestAuth`, `internal/component/radius/packet.go`) and against `rfc/full/rfc5176.txt` before dispatch | fixed in this spec's session | in progress |
| 2026-08-31 | spec-rfcgate-6, rfc5176 walk | The other nine RFC 5176 defect classes: Service-Type never read; attributes not treated as mandatory; only the first matching session acted on; Proxy-State and State not echoed; a stale Event-Timestamp NAK'd where Section 6.3 says silently discard; CoA state changes not atomic; Termination-Action State echo absent | A conformance programme, not a walk. `rfc/extraction/rfc5176.json` cannot sign until they resolve, because 23 of 72 sites state obligations Ze does not meet and no exclusion kind in the closed set honestly covers an unmet obligation | needs a spec; Thomas decides whether it runs | open |
| 2026-08-31 | spec-rfcgate-6, rfc5176 walk | RFC 5176 Section 3.2 "Authorize Only" is an explicit OPTIONAL | A MAY clause. `ai/rules/rfc-compliance.md` is explicit that an agent is not authorized to pick: implement it, decline it, or make it a config option. Declining still owes the Section 2.2 CoA-NAK for an unsupported Service-Type | Thomas | open |
| 2026-08-31 | spec-rfcgate-6, rfc2865 walk | RFC 2865 Section 5.25, Class attribute: "The client MUST NOT interpret the attribute locally", against Ze's shipped `profile-attribute class` feature (`mapProfiles`, `internal/component/radius/authenticator.go`; documented in `docs/guide/radius.md`, "Profile mapping") | Not a slip and not an agent's call. A documented, shipped feature sits directly on a MUST NOT, so the answer is either to drop the option or to authorise the deviation and record it as a `plan/journal/` row carrying the RFC section and the reason, which `ai/rules/rfc-compliance.md` requires of an authorised deviation | Thomas | open |
| 2026-08-31 | spec-rfcgate-6, rfc2865 walk | Seven ordinary RFC 2865 conformance fixes: Request Authenticator not regenerated per Identifier on failover (Section 4.1); Access-Challenge not treated as Access-Reject on the admin path (Section 4.4); empty shared secret accepted (Section 3); Service-Type written but never read (Sections 5.6 and 1.1); Access-Request built with no credential attribute (Section 4.1); zero-length User-Name reaching the wire (Section 5) | NOT deferred, only queued. `internal/component/radius/client.go` and `internal/component/radius/packet.go` are held by two other agents in this checkout, so dispatching now would collide. The failover and empty-secret rows were verified at `SendToServers` and `ExtractConfig` by the main thread | dispatched as one package once those files are released | queued |
| 2026-08-31 | spec-rfcgate-6, rfc2865 walk | Add the 28 new MUST-level ids `rfc/short/rfc2865.md` does not declare, with two tagged tests each, and finish the remaining 35 sites | `rfc/short/rfc2865.md` declares 13 ids for a 76-page base protocol carrying 74 normative sites. The gap is structural, and the ids cannot land before the conformance rows above resolve: a new MUST id without its tests reds the corpus for every session in this shared checkout | follows the conformance package | open |
| 2026-08-31 | spec-rfcgate-6 | Partial walk artifacts preserved at `plan/deferrals/rfcgate-6-partial-walks/rfc2865.json` (39 of 74 sites, 77 of 77 sections) and `rfc5176.json` (26 of 72 sites, 28 of 28 sections) | They were written to `tmp/session/.../scratch/`, which is cleaned on a cadence. That is the decay this session filed a row about in `plan/journal/claim-outlives-the-evidence-it-cites.md` on the same day, so leaving ~65 hand classifications there would have been an instance of the class the row describes. Copied into a tracked path the RFC gate does not read, confirmed by running `./le rfc check` after the copy | resumed by whichever spec finishes the two stems | preserved |

## Note on the two blocked stems

`rfc5176` and `rfc2865` are the first two Tier 1 walks and both are `Supported` on
`docs/features/rfc-status.md`. Neither can sign. That is the spec's premise landing rather
than failing: the artifact exists to bound a checklist against the RFC's own text, and both
checklists turned out to be bounded by what somebody happened to write down.

The consequence for `plan/deferrals/rfcgate-0-umbrella.md` row D5, which wants a measured
drain rate before the quota is armed: Tier 1 throughput is not that measurement. A Tier 1
sign-off is gated on conformance work rather than on walking effort, so timing it would
measure the wrong thing. Tier 4 is the tier to measure.
