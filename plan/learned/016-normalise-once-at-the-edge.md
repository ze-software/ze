# Learned: normalise at the edge, and the rails stop needing to agree

RFC 6793 splits one fact, the AS path, across two attributes. Ze reconciled them
in the RIB, where the answer was read, and nowhere that reached the wire. A route
learned from an OLD speaker was relayed with AS_TRANS widened to four octets and
the real AS numbers thrown away: a well-formed UPDATE carrying a path nobody
announced, accepted by the peer, run through its best-path, re-advertised.

The fix was not a repair to the relay. It was moving the reconciliation to the
edge, once per received UPDATE, so that everything downstream reads one truth.

## The rails did not need fixing, they needed nothing left to disagree about

Ze had two forward rails resolving the AS-path family, chosen by a guard on
whether the destination was eBGP. The obvious reading is that the guard is the
defect and the rails must be unified. The spec was first written that way: lift
the call out of the guard, add the missing reconstruction arm to the byte
transcoder.

The shape that worked instead was to make the question moot. With the pair
collapsed at ingest, no AS4_PATH survives to be mishandled, and phase 5 of the
implementation was defined as **proving a zero-line diff on both rails**. It got
one. The rails were never wrong about what they did; they were handed inputs that
still contained a contradiction.

**When two code paths disagree about a fact, ask whether the fact can be settled
before either of them sees it.** Unifying them is the expensive answer and it
leaves the contradiction alive, one layer up.

## The fast path is free only if it is defined by identity

"Ze stores four-octet AS numbers and regenerates the two-octet form at encode"
sounds like it costs every UPDATE a conversion. It costs a modern fleet nothing,
because a NEW speaker sends neither AS4 attribute: the collapse answers 0, the
caller keeps its own slice, and no AS_PATH is parsed.

That is checkable by identity rather than by allocation count, and the test says
so in a way a benchmark cannot: the fast-path fixture carries a **deliberately
unparseable AS_PATH**. A clean answer proves the collapse never looked at it. An
allocation ceiling of zero would have passed just as well against an
implementation that parsed the path and threw the result away.

**Assert the work that did NOT happen, not the allocations it did not make.**

## An invariant over each half proves nothing about the pair

The first version of the AS4_PATH derivation satisfied every property anyone had
written down: right AS numbers, right order, and the AS number count difference
between the two attributes preserved. An acceptance criterion asserted exactly
that difference.

It was wrong, and the invariant is why. RFC 6793 Section 4.2.3 has the receiver
take as many AS numbers from the leading part of AS_PATH as make the counts
equal. Prepending to both attributes leaves the difference unchanged, so the
receiver spends that budget on the AS_TRANS placeholders ze just wrote. Measured:
AS_PATH `[64500, 23456, 64496]` with AS4_PATH `[4200000002, 64496]`, prepend
4200000001, and the peer reconstructs `[23456, 4200000001, 4200000002, 64496]`.

The test that caught it does not compare bytes to expected bytes. It runs the two
attributes ze emits through `attribute.MergeAS4Path`, ze's declaration of the
receiver's rule, and compares the result to the path ze meant to send.

**When a fact is split across two artifacts, assert on the recombination.** The
AC that asserted the count difference is the shape to distrust: necessary,
cheap to check, and satisfied by the defect.

## What the RFC does not say is a decision, and it is written down as one

A lone AS4_AGGREGATOR, arriving with no AGGREGATOR beside it, is not covered by
RFC 6793. Every rule in Section 4.2.3 opens "When both of the attributes are
received", and Section 6 makes the attribute malformed on its length alone. The
shape cannot come from a conformant sender at all, because Section 4.2.2 obliges
the pair.

FRR and BIRD disagree about it. BIRD unsets the attribute before its own pairing
test. FRR keeps it, fabricates an AGGREGATOR around it under a comment calling
the shape bogus, copies the AS but not the identifier, and advertises that
invented node downstream. Ze drops it, which is BIRD's answer.

That is recorded in three places, each saying it is a decision rather than
conformance: the producer, the architecture page, and `rfc/short/rfc6793.md`
under a heading that says NOT COVERED BY THE RFC so nobody hunts for a
requirement id. **A choice the standard leaves open is a thing to write down at
the point of choosing, with the alternatives that were rejected.**

## Deleting dead code is where the unimplemented obligations surface

Retiring `aspath_rewrite.go`, which had no non-test caller, was meant to be
bookkeeping. It turned out to be the only caller of the tombstone Section 5.3
Transitive-clear, and of the tombstone-on-unreadable-AGGREGATOR path. Both
behaviours had stopped happening when `ASPathEdit.Record` replaced the
whole-payload rewrite, and no test went red, because the tests drove the dead
file rather than the live rail.

Four green tests were reading as coverage for behaviour the product did not
have. That is worse than an absent test: it is an absent behaviour wearing a
test's clothes.

**Before deleting an unreached file, list what it is the last caller of.** The
answer is a list of things the product quietly stopped doing, and each one needs
a ruling rather than a deletion.

## The peer's own source settles a design question the RFC leaves open

Twice in this work the deciding evidence was FRR's and BIRD's code, not the RFC:
where to reconcile, and what to do with the lone attribute. Both implementations
reconcile at ingest and store one canonical path, which is why neither needs a
Section 4.2.3 step on its forward path, which is what made ze's original plan an
answer to a question nobody else has.

Reading them cost two agents and about twenty minutes. It changed the shape of
the work before it was built rather than after.
