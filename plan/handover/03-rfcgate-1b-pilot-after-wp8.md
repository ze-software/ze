# 03 -- RFC 7296 pilot: five packages landed, implementation moves to Opus 4.8

| Field | Value |
|-------|-------|
| Spec | `plan/spec-rfcgate-1b-rfc7296-pilot.md` |
| Landed here | WP-1 defects, WP-5, WP-3, WP-8, plus the IPComp and remote-access splits |
| Gate | 2867 gated MUST-level requirements, 2971 tags resolved |
| Written | 2026-07-31 |

Supersedes `02-rfcgate-1b-pilot-wp1-wp2-wp6-wp12.md`, `02-design-wp3.md` and
`02-design-wp5.md`. All three are removed by the commit that adds this file.

## Why this handover exists

`c_model_phase` (`.claude/hooks/pretool-writeedit.py`, `plan/learned/1310-phase-gates.md`)
landed mid-session and blocks an implementation edit made on Opus 5. The owner chose to
start an Opus 4.8 session rather than write the ack. Everything below is what that session
needs. Nothing here is blocked on anything except the model.

## Landed this session

| Commit | What |
|--------|------|
| `dbf3dbcd0` | the three WP-1 defects handed over by 02, each mutation-verified |
| `61363ef6f` | functional test 370, a missing happens-before edge, not slowness |
| `88c0abe1d` | WP-5, six rows, 17 of 17 mutations killed |
| `413f6ae21` | IPComp and remote-access split out, per two owner decisions |
| `789578e28` | the four IPComp deferral rows |
| `87d749149` | WP-3, fifteen rows, 39 of 39 mutations killed |
| `1ebdb35cc` | WP-8, four rows landed and two held |

Four structural gates were red at HEAD when this session started and are green now:
`ze-rfc-check`, `ze-doc-test`, `ze-regen-check-readonly`, and `go test ./scripts/dev`. The
cause of all four was a stale generated index, not a code defect.

## Owner decisions, 2026-07-31 (BINDING, do not re-open)

1. **IPComp is not implemented in the pilot.** `plan/spec-ipsec-ipcomp.md` covers the whole
   feature. All four `RFC7296-2.22-*` rows left the pilot with deferral rows.
2. **Hash-and-URL lands inside WP-10, behind a YANG leaf defaulting OFF.** Support is a
   MUST, and the obligation's verb is "capable of being configured". The load-bearing
   bound: verify the SHA-1 BEFORE `x509.ParseCertificate`. A fetcher that parses first
   passes every other test row and is still exploitable.
3. **WP-9 is split.** The two live defects and the conformant rows stay in the pilot. The
   nine feature rows moved to `plan/spec-ipsec-remote-access.md`.
4. **`cookie-threshold` defaults to 0, challenge every initiation.** See WP-4 below. This
   overrides `03-design-wp4.md` section 3.3, which is self-contradictory on this point.
5. **Standing approval** to use `// rfc-test-change-approved:` for the rest of this spec,
   but ONLY where the edit makes a tagged test assert MORE than before.

## Next: WP-4, and a design defect to fix before you write any code

`03-design-wp4.md` is otherwise sound. Its audit was completed and is worth not repeating.
The ids were re-verified: §1.2 mark 4, §2.6 mark 2, §2.6.1 none, all six ordinals legal, no
renumbering. All six rows are genuinely absent. `NotifyCookie` (`wire/payload_notify.go`)
has no production referent at all.

**The design defect.** Section 3.3's `cookie-threshold` with `default 1` does NOT close the
half-open-slot defect it exists to close. The first spoofed datagram sees zero half-open
SAs, so no cookie is demanded. The CAS in `tryResponderSAInit` (`engine/register.go`) then
wedges that peer's only slot for the 30s `responderHandshakeTimeout`, exactly as today.
Section 3.2's claim that "the CAS is reached only by a sender that echoed a cookie" holds
only if a cookie is demanded on the FIRST initiation, which section 3.3's own semantics do
not produce. Section 6.3's scenario 18 assumes the opposite.

**The owner's ruling:** `cookie-threshold` is the number of half-open IKE SAs Ze tolerates
before it challenges, and it defaults to 0. Monotone, no magic value. The cookie is demanded
before the first state commitment. Cost: one extra round trip on every inbound handshake,
and four responder unit tests plus four responder-side interop scenarios then traverse the
challenge path.

**Two live defects WP-4 owns**, both confirmed by reading the producing functions:
- Ze can never establish with a peer that prefers a different DH group.
  `handleSAInitResponse` (`engine/fsm.go`) has no `NotifyInvalidKEPayload` case, so the
  response falls to the completeness gate and sets `StateDead`. `runInitiator` loops on
  `sa.State != StateEstablished` only and re-sends `sa.LastSentMsg`, and `newInitiatorSA`
  (`engine/initiator.go`) rebuilds DH from `Proposals[0].DHGroup` each cycle. Same wrong
  group, forever.
- One spoofed datagram bearing a configured peer's source address takes that peer's only
  half-open slot for 30s. `matchResponderPeer` is the only gate before the CAS.

**The biggest implementation risk** the design names: the IKE_SA_INIT retry must re-anchor
`sa.InitiatorSAInitMsg`, because AUTH is computed over it. Every payload-shape test passes
with the stale value, and the failure surfaces two messages later as an opaque AUTH
mismatch.

## The queue after WP-4

| Order | Work | Design |
|-------|------|--------|
| 1 | `RFC7296-2.3-6`, INVALID_MESSAGE_ID, lands at `2.3-9` | `03-design-2.3-6.md` |
| 2 | WP-9 partial: two live CP defects plus the conformant rows | `03-design-wp9.md` |
| 3 | WP-10: 9 rows, hash-and-URL default off | `03-design-wp10.md` |
| 4 | WP-7: 11 rows, needs a selector config surface that does not exist | `03-design-wp7.md` |
| 5 | Residual rows: 3 from WP-6, 3 from WP-12 including `RFC7296-4-1` | none |
| 6 | Closure: audit, Review Gate, learned summary, the two commits | `/ze-close` |

## Id allocation, measured 2026-07-31

`check_id_allocation` (`scripts/dev/rfc_requirements.py`) refuses an ordinal at or below its
section's high-water mark, computed from COMMITTED HEAD. A stranded ordinal is permanent.
**Recompute at the moment of landing. Never trust a number in this file.**

    git show HEAD:rfc/short/rfc7296.md | grep -o 'RFC7296-2\.3-[0-9]*' | sort -V | tail -1

| Section | Mark | Consequence |
|---------|------|-------------|
| §2.3 | 8 | `2.3-6` lands at `2.3-9`. Ordinal 6 can never be allocated |
| §2.5 | 17 | WP-5 took 14 through 17 |
| §2.23 | 9 | WP-8 took 8 and 9. 10 and 11 are RESERVED for its two held rows |
| §3.3.4 | 3 | WP-10's `3.3.4-1` lands at `3.3.4-4` |
| §1.7 | 2 | the remote-access `1.7-1` lands at `1.7-3` |
| §4 | none | THREE claimants and no mark. Whoever lands first sets it |

**§4 is the trap.** The claimants are `4-1` (pilot, WP-12 residual), `4-2` and `4-3`
(remote-access spec), and `4-4` (WP-10). The pilot's phase list schedules WP-10 at item 14
and WP-9 at item 15. That order strands `4-1`, `4-2` and `4-3` permanently. Land §4
ASCENDING, or land all four together.

## Two rows held open, awaiting an owner ruling

WP-8 reserved `2.23-10` and `2.23-11` rather than annotating them. A QEMU probe against a
real XFRM stack disproved the design. An inbound state with an ESP-in-UDP template refuses
bare ESP. A state without one refuses encapsulated ESP. The lookup is not encap-aware, so
two states on one SPI do not help. Receiving both forms at any time is unreachable per SA
on XFRM.

Writing `{gap}` is itself a compliance decision (`ai/rules/rfc-compliance.md`), so the rows
stay open. Raise them with Thomas.

## Things a session will otherwise rediscover

**Verify the agent, do not relay it.** Three reports this session claimed a gate was green
where the command exited non-zero, or claimed a mutation table that a re-run contradicted.
Each was caught by re-running the command in the main thread. Budget for that.

**`make X | tee` reports tee's exit status.** The session's first act was to call a
six-stage-red `ze-verify` green because of this. Use `echo "EXIT=${PIPESTATUS[0]}"`.

**A commit gate reads the literal phrase "out of scope" as an undeclared deferral.** Two
WP-3 comments used it to mean RFC scope and blocked the commit. Reword rather than file a
deferral shard.

**A new production file's `// Design:` must point at something durable.** WP-3's pointed at
`plan/handover/02-design-wp3.md`, which this commit deletes. Every sibling engine file cites
a `plan/learned/NNN-*.md`.

**`make ze-ste-check` reads the whole working tree.** With a sibling session active it will
name files that are not yours. The commit-time gate is per-commit-scoped, so read the paths
before you spend an edit.

**Mutate with `go -overlay` and copies under `tmp/`.** Never edit the tree to mutate: a
sibling session shares this checkout.

## Verification state

`make ze-verify-changed` passed all 19 stages at `11:58:19Z`, before WP-3 and WP-8 landed.
Since then each package was verified with the changed-scope targets and the ike suite, and
each guard was independently mutation-checked in the main thread. **A full `make ze-verify`
has not run since WP-8. Run one first.**
