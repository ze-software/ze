# Learned: a mirror asserts sameness, so every sender field in it is wrong

`ze-peer` built its OPEN by copying ze's OPEN and writing over three fixed
offsets. 637 `.ci` files drive a BGP session through that one function.

A mirror is an assertion that the two speakers are the same. Every field in a
BGP OPEN that describes the SENDER is therefore wrong in a mirror by
construction. The only question is whether anything reads it.

| Field | Does ze check it? | How the defect appeared |
|-------|-------------------|-------------------------|
| AS (My AS field, capability 65) | Yes, `validateOpenPeerAS` | 132 red tests |
| Role (capability 9) | Yes, `validateOpenRolePair` | 16 `.ci` files carry a hand-written drop-and-add |
| ADD-PATH direction (capability 69) | No. `Negotiate` intersects to `AddPathNone` | No red. A coverage hole: no `.ci` in the tree configures a one-directional session |
| FQDN (capability 73) | No consumer | Silent |
| Graceful Restart time (capability 64) | Yes, `startEORTimer` reads it | Silent, and ze acts on its own restart time believing it is the peer's |

Ranked by how loudly they failed, these read as five unrelated bugs. Ranked by
what they are, they are one. **Rank a symptom list by mechanism before you size
the work, because the loudest instance is rarely the whole defect.**

## The narrow fix re-arms the defect

The first repair patched capability 65 beside the My AS field. It closed every
`.ci` that sets `option=asn`, which is why two suites went green.

It could not close the other 132, and the reason is structural rather than an
oversight. Those files set no `option=asn` at all, so there was no value to
patch. A fix that writes a value into one more carrier still leaves the code
writing facts into a buffer it did not build, and the next capability ze learns
to send re-opens it.

**A fix that adds a second write to a copied buffer has not addressed the
defect. It has moved the boundary.** The shape that ends it is the one ze's own
producer already used: resolve the fact once, then let every carrier read that
resolution.

## The fact was already declared, once

The absent-option case looked like an edit to 128 `.ci` files. It was not.

Each of those files embeds ze's own configuration, which already declares
`asn { local N; remote M }`. The AS ze expects from its peer is `M`, written
down in the same file. Asking the author to repeat it in the peer block would
have created the second declaration that later disagrees.

`declarePeerAS` (`internal/test/runner/peer_asn.go`) derives it instead. Zero
`.ci` files changed, and the derivation is identity-preserving for the 397 iBGP
files where `local == remote`.

**Before writing a value into a second place, look for where it is already
written.** The remedy for a fact that exists in one half of a file is usually to
read that half, not to make the author state it twice.

The derivation fails closed. Two peers ze dials at one address, expecting
different ASNs, is refused rather than guessed, because nothing on the wire
tells those sessions apart.

## Re-measure the premise at implementation time

This spec was written against 224 failures across four suites. By the time it
was implemented the count was 132 in one suite, because a narrow patch had
landed in between and turned `encode` and `vrrp` green.

The measurement was four hours old and already wrong about which files were
red, which suites were affected, and what the remaining mechanism was. A spec
that had been implemented against its own opening paragraph would have written
tests for a population that no longer existed.

**A spec's measured premise is evidence with a timestamp. Re-run the
measurement before the first line of code, not to validate the spec, but
because the answer decides what the tests are for.**

## A quoted sentence is the cheapest thing to verify

The narrow patch justified itself with a quotation attributed to RFC 6793
Section 4.1: "if the value of the AS number field is not the same as the value
of the AS number encoded in the AS4 capability, then the BGP speaker MUST send
a NOTIFICATION". That sentence is not in RFC 6793. `MUST send a NOTIFICATION`
occurs nowhere in the document.

Section 4.1 states a precedence rule and no error behavior at all. The real
mechanism is RFC 4271 Section 6.2, reached because `openAdvertisedAS` applies
6793's precedence rule and then hands `validateOpenPeerAS` an AS the session is
not configured for. The observed failure was real. The reason given for it was
invented.

The same fabricated quotation had reached `docs/architecture/testing/ci-format.md`,
where it was published.

No gate reads a citation in a comment, and `./le rfc check` binds requirement
ids to tests without reading a quoted claim back against `rfc/full/`. So an
invented quotation is green everywhere.

**Quotation marks read as evidence already checked, which is why they are the
least checked thing in the repository.** Rows in
`plan/journal/reference-checked-claim-unchecked.md` record the weaker version of
this, a wrong section number attached to a true claim. This one is a sentence
the document does not contain.

## The repair kept rebuilding the defect it was repairing

Six review rounds ran. Four of them found the same class, one layer further out
each time, and every instance was in code written to FIX the previous instance.

| Round | Where the class was found |
|-------|---------------------------|
| 1 | The new AS derivation: "the config declares no peer AS" and "I could not read it" both returned nil, and the peer mirrored ze's AS |
| 2 | The guard added in round 1: its own eBGP classifier was blind to a router-level `local` AS, and it dropped any peer whose AS it could not parse |
| 3 | The reader under the round-2 fix: `leaf` and `namedBlocks` returned `""` for input they could not match, so a three-state parse sat on a two-state find |
| 5 | The AS_TRANS guard: its precondition asked "was capability 65 dropped" when the question is "will ze-peer's own capability 65 stay off the wire", which a stated capability answers too |

**A guard cannot be trusted when its own population is computed by the reader it
guards.** That is what rounds 1, 2 and 3 kept producing. The guard looked
correct in isolation every time, and every time it was blind in exactly the way
the thing it guarded was blind, because both asked the same reader.

Rounds 1 and 2 fixed instances and the class moved. Round 3 diagnosed the shape
rather than the instance, and round 4 rewrote the reader: a tokenizer whose
separator set is read from ze's own `readWord`, and seven text matchers deleted.
That ended it. **The rewrite was SMALLER than the matchers it replaced.** When a
repair keeps failing one call site further out, the next repair is not another
guard.

Round 5's instance is the miniature version, and it is the one to remember,
because it is the cheapest to write by accident: two spellings of one question,
`statedAS` for "does this line carry the AS" used as the precondition when the
precondition needed "will ze's own capability be emitted". The fix is two named
predicates and a comment saying they must not be collapsed. Round 6 confirmed
each is independently load-bearing by breaking each alone.

**The transferable test: when you add a guard, ask what computes the set it
walks. If that is the same code the guard exists to check, the guard is
decoration.**

## Related

- `plan/journal/mirrored-field-asserts-the-wrong-sender.md` holds the five
  sender fields still mirrored.
- `plan/journal/reference-checked-claim-unchecked.md` holds the citation class.
- `plan/learned/005-runner-drops-what-it-cannot-honor.md` reached the rule this
  spec applied to `option=asn`: refuse where the `.ci` is read, before a socket
  exists.
