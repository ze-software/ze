# Learned: a value that cannot be told from "never set" needs a companion fact, not a better sentinel

`pppSession.peerInterfaceID` was a bare `[8]byte`. IPv6CP could reach Opened
with it all-zero, and every reader downstream treated that zero as the
subscriber's identifier: `peerLinkLocal` turned it into `fe80::`, `installRoute`
made that the next hop of the delegated prefix, and `onNCPOpened` published it.

The tempting fix is a sentinel: all-zero already means "invalid" on receive, so
read it as "not negotiated" everywhere else. That is the same defect in a new
spelling. It works only while nothing legitimately produces a zero, it puts the
same reasoning in every reader, and the first reader that forgets it is silent
rather than red. `pppSession.peerInterfaceIDNegotiated` is one bool beside the
value, written on exactly the two paths that can produce a trustworthy
identifier, and every reader asks it rather than inspecting the bytes.

**The test that says which shape you have: can a caller tell this value from a
failure that produced it?** Where it cannot, the fix is a second field, not a
cleverer reading of the first (`ai/rules/principles.md`).

## The second origin is the one that makes a sentinel look sufficient

The identifier has two legitimate producers, and only one of them is
negotiation. `evalIPv6CPRequest` sets it when the peer's Configure-Request
carries an acceptable option; `requestIPv6CPInterfaceID` sets it when the
address handler answers with `HasPeerInterface`, which is the operator
asserting the value rather than IPv6CP agreeing it. A sentinel cannot tell
those apart from each other either, and it did not have to, which is why
nothing was red. The companion fact is written on both, and its doc comment
says why the second one counts.

## A validator applied on one side lets an implementation send what it refuses

`isValidIPv6CPInterfaceID` existed and was applied on receive. The Configure-Nak
path read `peerInterfaceID` raw, so Ze proposed values its own validator would
have refused: the stale zero of a request it had just rejected. The repair is
not a new validator but the same one on the transmit path
(`suggestIPv6CPInterfaceID`, `internal/component/l2tp/ppp/ipv6cp.go`), which is
the "pair the check" habit in `docs/contributing/ze-go-style.md` read in the
direction people skip.

Two orderings inside that generator are load-bearing and neither is obvious.
RFC 5072 Section 4.1 requires the "u" bit clear on a suggested identifier;
clearing it can turn an otherwise-valid draw into the all-zero value, because
that bit may be the only one set. So validity is checked AFTER the clear, on the
value actually transmitted. And when no draw survives, the answer is a
Configure-Reject carrying zero, which is the RFC's own terminal state for "a
unique interface identifier cannot be negotiated", rather than a Nak carrying
whatever was in the field.

## An RFC section number is a path, and copying one is owning it

Eight comments and a design page cited "RFC 5072 Section 3.2" as the authority
for rejecting an all-zero identifier. RFC 5072 has no Section 3.2. A ninth site
quoted a sentence -- the identifier `"MUST NOT be all zeros or all ones"` --
that appears nowhere in the document. Five of those sites were written by this
spec, by copying the citation that was already there.

`plan/journal/claim-outlives-the-evidence-it-cites.md` already held two rows of
exactly this shape, for RFC 2866 Section 5.18 and for a quotation attributed to
RFC 4760 Section 6. This is the third. `ai/rules/rfc-compliance.md` requires
reading the RFC's own text, and every one of these sessions did read it -- for
the requirement being implemented. What none of them did was open the section
they were CITING beside it, because the citation arrived as inherited text
rather than as a claim. Two commands settle it, and they are cheap enough to run
on every comment a change touches:

```
grep -n "^[0-9]\+\.[0-9]*\.\? \+[A-Z]" rfc/full/<stem>.txt   # does the section exist
grep -in "<four words of the quote>" rfc/full/<stem>.txt      # does the sentence exist
```

## The proof and its latency

The unit level is proven with observed reds: seven records in
`rfc/discrimination/rfc5072.json`, each taken by breaking the producing function
and watching the tagged unit go red. The interop level is written, registered
and cannot pass, and that is correct rather than a defect. `poolPlugin.handle`
(`internal/component/l2tp/plugins/pool/register.go`) declines every non-IPv4
address request and `runNCPPhase` reads that decline before the session reads a
client frame, so IPv6CP is unreachable in a shipped daemon and both scenarios
fail fast with a citing error rather than passing vacuously.

**A scenario written against behavior the product cannot yet reach is worth
landing, and it is worth landing only if its blocked state is named in three
places a reader will hit:** the checker's own file header, the lab page
(`docs/labs/pppoe-interop.md`), and the spec that unblocks it
(`plan/spec-l2tp-ipv6-subscriber.md`, whose owed work now names running them).
Without the third, the scenario is a test nobody is scheduled to switch on.
