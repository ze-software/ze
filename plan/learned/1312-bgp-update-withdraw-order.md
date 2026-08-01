# 1312 -- UPDATE withdrawals run before announces

## Context

RFC 4271 Section 4.3 makes three statements about one message shape. An UPDATE SHOULD
NOT carry the same prefix in WITHDRAWN ROUTES and NLRI. A speaker MUST be able to
process one that does. And it SHOULD treat that message as though WITHDRAWN did not
name the prefix.

Ze applied every insert before every remove. The withdrawal therefore deleted the route
the same message had just announced, and a reachable prefix became unreachable.

The defect surfaced during work on a public wiki page about how Ze proves RFC compliance
(`plan/learned/1311-rfc-compliance-docs.md`). The page quoted this gap as a worked
example, a reviewer checked the quotation against the RFC, and the quotation was wrong.

## Decisions

- **Fixed the order rather than reclassifying the annotation.** Ze diverged from the RFC
  either way, so relabelling would have left the behaviour wrong and moved only the
  paperwork.
- **All withdrawals precede all announces, rather than pairing each section with its
  own.** The guarantee then holds whichever section carried the prefix, including a
  prefix withdrawn in the legacy field and announced through MP_REACH.
- **Annotated the MUST `{single-polarity: positive}`.** The obligation is to ACCEPT a
  message shape, so no input exists that must be rejected, and a negative test would
  have to assert the absence of an error, which proves nothing (`ai/rules/tdd.md`).

## Consequences

- RFC 4271 drops from twenty-one gap annotations to twenty. `check_gap_count_agreement`
  reads the spelled number in `docs/features/rfc-status.md`, so that prose had to change
  in the same commit.
- **Nine consumers shared the ordering, in five plugins, and the first commit fixed
  one.** Announce-then-withdraw was the house style for reading an UPDATE, so every
  consumer written against that pattern inherited the defect:

  | Consumer | What the same-prefix UPDATE did |
  |---|---|
  | `rib` `handleReceivedStructured` | route absent from the RIB |
  | `rib` `handleReceivedPool` (JSON) | same, on the external-plugin path |
  | `rib` `handleInjectWireRoute` | same, on injection |
  | `adj_rib_in` `handleReceivedStructured` | route absent from `show bgp peer ... rib` and BMP |
  | `persist` | add queued before del, so the stored route was deleted |
  | `rpki` | prefix dropped from the ASPA tracker, so re-validation never revisits it |
  | `rr` and `rs` withdrawal maps | route left unregistered, so it is never withdrawn when the peer goes down |
  | `adj_rib_in` `handleReceived` (legacy JSON) | Del applied after Add in producer order, so the route vanished |
  | `rib` `handleSentStructured` | the record of what Ze SENT said withdrawn for a prefix the receiver installs |

  The first three were found by grepping `internal/component/bgp/plugins/rib/`. That
  scope was the mistake: the same behaviour lives in four other plugins, and an
  independent review found the `adj_rib_in` one, which was the worst because it made two
  views of the same event disagree once `rib` alone was fixed.

- **The last two were found by the second review, after the first sweep looked
  complete.** Each round of looking found more: the first grep found three, an
  independent review found a fourth, a behaviour-scoped sweep found three more, and a
  second review found the final two. The lesson is not that any one search was lazy. It
  is that a defect which is a *house style* does not have a natural edge, so "I have
  found them all" needs a mechanical basis rather than a feeling.
- **Three of the nine are proven by tagged tests; six are reasoned reorders.** The three
  in `rib` (structured, pool, injection) have tagged tests, each mutation-checked against
  the original ordering. The other six rest on reading the producing code and on their
  package tests staying green. That is honest coverage, not complete coverage.

## Gotchas

- **Scope the sibling audit by BEHAVIOUR, not by directory.** I grepped the plugin I was
  editing and found three call sites, which felt thorough and was not. The question that
  finds all seven is "what else reads an UPDATE and applies both sections", and the grep
  that answers it is for `wu.Withdrawn()` or `MPUnreach()` across `internal/`, then a
  read of each hit to see whether it mutates something prefix-keyed. Two hits were
  genuinely inert (family-set builders) and had to be read to know that.
- **A `{gap}` can sit on the wrong obligation inside one paragraph, and no gate can see
  it.** Section 4.3 carries a MUST and a SHOULD one sentence apart, captured as
  `RFC4271-4.3-5` and `RFC4271-4.3-7`. The gap was written against the MUST, which Ze
  met the whole time. What failed was the SHOULD. The checker verifies that an
  annotation exists and carries a reason, never that the reason describes THAT
  requirement's obligation. Only reading the RFC beside the annotation finds this.
- **The obvious mutation did not discriminate.** To prove the new test catches the bug I
  first disabled the withdrawal block. The test passed, because deleting withdrawals
  also lets the announce win. Only restoring the original ORDER made it fail. A mutation
  that makes the test pass for a second reason proves nothing about the test.
- **Reordering blocks can silently rebind a shared `err`.** In `handleInjectWireRoute`
  the announce block tests the error from `wu.NLRI()`, fetched much earlier for RFC 7606
  validation. Moving a `wdData, err := wu.Withdrawn()` above it would have made that
  block read the withdrawal's error instead. The withdrawal blocks there now carry their
  own `wdErr` and `unreachErr`.
- **Prove a reordering is a reordering.** Comparing the multiset of non-comment lines
  before and after showed the structured-path change added only comments. That is
  stronger evidence than reading the diff.

## Files

- `internal/component/bgp/plugins/rib/rib_structured.go` -- `handleReceivedStructured`
- `internal/component/bgp/plugins/rib/rib.go` -- `handleReceivedPool`
- `internal/component/bgp/plugins/rib/rib_inject.go` -- `handleInjectWireRoute`
- `internal/component/bgp/plugins/adj_rib_in/rib.go` -- `handleReceivedStructured`
- `internal/component/bgp/plugins/persist/server.go` -- the op queue
- `internal/component/bgp/plugins/rpki/rpki.go` -- `removeWithdrawnFromTracker` ordering
- `internal/component/bgp/plugins/rr/withdrawal.go` -- `updateWithdrawalMapWire`
- `internal/component/bgp/plugins/rs/server_inventory.go` -- `extractWireNLRIRecords`
- `internal/component/bgp/plugins/rib/rib_rfc4271_mixed_update_test.go` -- the proof, and
  it covers the `rib` structured path only. The other six carry no test for this shape
- `rfc/short/rfc4271.md`, `docs/features/rfc-status.md`, `ai/RFC-REQUIREMENTS.md`
