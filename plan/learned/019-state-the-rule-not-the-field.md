# Learned: a repair that names a field leaves the next field

The BFD first-packet lookup (`firstPacketKey`, `internal/component/bfd/engine/engine.go`)
selects the session for a Control packet whose Your Discriminator is zero
(RFC 5880 Section 6.8.6). It was repaired five times in one spec. Each repair was
correct in the dimension it examined, and blind in the next one:

| Round | What was repaired | What it did not see |
|-------|-------------------|---------------------|
| 8 | the packet's LOCAL address came from the wildcard bind, so no key could match it | the interface |
| 10 | a multi-hop packet carried an ingress interface the session's key never holds | single-hop |
| 11 | `ingressInterface` answered for multi-hop as well, which the mode invariant forbids | a single-hop session whose key names no link |
| 12 | such a session could never be selected at all, so a strict BGP peer sat in OpenSent for the life of the daemon | a fourth field would repeat the walk |
| 13 | the rule stated once as a property, with the relaxation set derived from the flags | nothing, and round 14 confirmed it |

The shape is one sentence, and it was available from round 8: **a key field the
session left UNSET does not participate in the match.** A session's key is what
its client could say about it; a received packet carries a real value in every
field the kernel reports; an exact struct match between the two cannot hit. Every
one of the first four repairs is an instance of that sentence applied to one
field.

Rounds 8 to 12 cost four review rounds, four fixes and four tests. Round 13's
general form cost one comment, one `optionalKeyFields` constant and one loop.

## The test for a rule is structural, not behavioral

A behavioral test for round 12's fix proves that a session with no interface is
selected. It says nothing about the fifth field, because that field does not
exist yet, and a test cannot exercise what nobody has written.

So the round-13 rule is held by a test that reads the TYPES:
`TestFirstPacketKeyMirrorsEveryKeyField` fails when a field is added to `api.Key`
and not classified, and `TestRelaxationsCoverEveryRelaxableField` derives the
relaxable set from the code by zeroing a populated key with `optionalKeyFields`
and reading back which fields moved. Neither one can be satisfied by a comment.

## A structural test can enforce the NAMES and miss the VALUES

That pair still had the spec's own defect one layer down. Both tests work over
field NAMES: they force a new `api.Key` field to appear on `firstPacketKey` and
in the placement table. Neither forces `firstPacketIndex` to COPY the value. A
field declared on both structs and left out of the builder makes every session
index with that field's zero value, and `l.byKey[firstPacketIndex(key)] = entry`
then replaces one session's index entry with another's, in silence, with both
sessions still answering by discriminator.

`TestFirstPacketIndexCarriesEveryFieldValue` is the missing half: it asserts no
indexed field is left at its zero value while the key's field is set, and that
varying exactly one `api.Key` field changes the index. When you write a parity
test, ask which of the three it holds: the NAME, the VALUE, or the ORDER.

## A comment promoted to specification owes a test

Inside one popcount tier the relaxation walk is ordered by the declaration order
of `relaxLocal` and `relaxIface`, and that order decides a real case: two
sessions, one naming only the link and one naming only the local address, are
each one relaxation away from a packet carrying both. The comment above
`keyRelaxations` states that the LINK wins and gives the reason.

Nothing asserted it. Swapping the two constants inverted the documented behavior
with every test in the package still green, which is the definition of a claim
with no evidence behind it (`plan/journal/green-that-could-not-have-been-red.md`).
`TestFirstPacketTieBreakPrefersTheLink` now reds on that swap, and it is the only
test in the package that does.

The general shape: a comment that explains a decision costs nothing. A comment
that STATES a behavior a reader will rely on is a specification, and it owes the
same proof as any other claim: break the code it describes, and watch a test go
red for the sentence the comment writes.
