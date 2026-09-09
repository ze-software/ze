# Learned: fix the question, not the site

`rib { fib-withhold [ bgp isis ] }` names the protocols whose selected routes
sysrib declines to write to the FIB. One leaf-list, one permission map, one gate
in front of the publish. The feature is small. It took six review rounds and
thirteen product defects, and all but two of those were one question asked the
wrong way.

## A named site is a sample of the class, never the class

Round 1 named six sites. Each was fixed where the reviewer pointed. Round 2
found the same defect alive on sibling paths, fixed those where round 2 pointed,
and would have run again.

Round 3 broke the pattern by replacing the QUESTION rather than the site. It
declared one predicate, `programmedByZe` (`internal/component/sysrib/sysrib.go`),
and then walked its callers: every Withdraw, Add and Update construction in the
package, and every read of the cache behind it. Rounds 4, 5 and 6 each found
real product defects. None of them was that class again.

`ai/rules/principles.md` already says the work a change owes is measured by what
the change can REACH, not by the files you edited. What this work adds is the
method for a proxy read: the reach set is the callers of a predicate that does
not exist yet, so **write the predicate first and the walk becomes mechanical**.
A reviewer can then check the walk, which they cannot do for a list of sites
somebody remembered to visit.

## A data structure has a producer, and a cache is where a second meaning grows

`resolvedNH` is a map of prefix to last emitted resolved next-hop, written for
the cascade worker to detect resolution changes. This feature was the first code
to ask it a different question: has Ze programmed this prefix?

Three proxies were read for that question across the first two rounds:
`IsValid()` on the stored address, `prev != nil`, and absence from the map. All
three are wrong for one reason. `fibEntry` returns the winner's own target when
there is nothing to resolve, so a route reached over a device alone
(`static { route { forward { interface X } } }`) is programmed with an INVALID
address stored for it. No validity test can tell that from a prefix Ze never
programmed. Presence of the key is the answer; validity of the value is a
different fact.

The struct comment asserted the opposite, and the first code trusted the
comment. `ai/rules/evidence.md` requires reading the producing function before
stating what code does. **A data structure has producers too**, and a cache
carried across a release is the shape most likely to have grown a meaning its
comment never claimed.

## Ask which OPERATION broke, not whether the file is in scope

Two scope calls in this spec went opposite ways, and both were right the second
time.

| Defect | First call | Right call |
|--------|-----------|------------|
| The permission sweep could not put back what it took away | journaled as "outside the problem in hand, which is which protocols reach the FIB" | FIXED. The operation that failed IS "let this protocol reach the FIB again" |
| A promoted ECMP member is programmed with the winner's label stack and SRv6 SID | journaled | journaled. The operation it breaks is MPLS forwarding, and withholding a protocol is untouched by it |

`ai/rules/completion.md` gives one question: does the goal this work exists to
achieve still hold if I leave this? Both calls above applied it. The first
applied it to the feature's NAME, and the name and the operation came apart
because the broken operation was the feature's own inverse.

**A feature's name covers its inverse, its undo and its disable path, and the
scope test has to be read against the operation rather than the noun.** The
correction row is in `plan/journal/guard-added-to-one-half-of-a-pair.md`, which
carries both the wrong call and the reason it was wrong.

## A finding is authoritative about the defect, not about the fix

Round 4 told the author to make the live path obey `fibEntry`'s "not owed"
verdict, so that one function answers for every emitter. Doing that reddened
five tests, two of which `test/ospf/ospf-route-install.ci` runs by name.

The reason is a fact about the scenarios rather than about the code. They load
no `connected` plugin, so nothing inserts a covering route, and every OSPF,
IS-IS and forked next-hop is unresolvable in the Loc-RIB while being on-link all
the same: the kernel resolves it against its own connected routes. Refusing to
program there IS the silent black hole one of those tests exists to prevent.

The author refused the instruction, said why, and replaced the bool with a named
verdict: `fibPathReachable`, `fibPathUnreachable`, `fibPathForbidden`. An
unreachable verdict still carries the producer's target, so the live path
programs it and a cascade reads it as a path lost.

**The subagent was right and the brief was wrong.** A review finding is a claim
about behavior, and you verify it. The fix the finding names is a hypothesis,
and you test it by running the suite before you believe it. The red suite was
the whole evidence here, and it took one run.

## A sentence with a "so" in it is a derivation, and a derivation is untested code

Three times, prose stated more than the code did, and each one read like a proof.

- "A withdraw carries no protocol, so passing one is always safe" was in the
  spec's design section before it was in the code. Passing one makes the kernel
  writer call `RouteDel` on a route that is not there and raise one FIB-sync
  failure per prefix. The protocol was never the question. Whether Ze holds an
  install outstanding is.
- An RFC 9252 Section 5 comment quoted a MUST beside code that declines the FIB
  write AFTER selection, while `rfc/short/rfc9252.md` recorded that same
  requirement as the gap RFC9252-5-2. Two declarations of one fact disagreeing,
  with the comment the wrong one.
- A guide paragraph named `publishChanges` as the filter. It filters nothing.

`ai/rules/documentation.md` puts the page edit in the work that changed the
behavior, and `ai/rules/evidence.md` puts the producer in front of the claim.
What this work adds is where the risk concentrates: **the spec's own design
prose is written before the producer exists, and nothing re-reads it.** A
derivation there is a design decision nobody tested.

## Measure the coverage of a new branch. Do not infer it

Four of this spec's defects were green when they were found: no test covered a
device-only next-hop, an unresolved next-hop, cross-protocol ECMP, or a
withheld-permitted-withheld cycle. The package carried SRv6 branches and, at
HEAD, no SRv6 test at all (`git grep -il srv6 HEAD --
internal/component/sysrib/` answers `sysrib.go` and the events package).
`TestReplaySkipsAPrefixZeDoesNotProgram` was the only test naming `replayBest`
and it is a NEGATIVE, so deleting that function's loop body left the package
green.

`ai/rules/testing.md` already requires a positive and a negative for every gated
requirement. The last gap here was not found by that rule and not by reading the
test list. It was found by running the package under `-covermode=count`, which
showed two of the permission sweep's five outcomes at zero. **Reading the test
names is inference. Run the profile.**

## Rule changes this work proposes

Two, both small, neither edited here.

- `ai/rules/completion.md`, beside the scope question: "Answer it against the
  OPERATION that broke, never against the feature's name. A feature's name
  covers its inverse, its undo and its disable path, so a defect in any of them
  is inside the work in hand even when the noun in the spec title does not
  mention it."
- `ai/rules/testing.md`, beside the positive-and-negative directive: "New
  branches owe a MEASURED coverage figure, not an inferred one. Run the package
  under `-covermode=count` and read the count for each new outcome. A test list
  read by name says which behaviors somebody meant to cover."

## Related

- `plan/journal/guard-added-to-one-half-of-a-pair.md`: the scope correction, and
  the cascade half of the same disagreement, which is still open.
- `plan/journal/helper-bypassed-by-an-open-coded-copy.md`: the promoted ECMP
  member carrying the winner's label stack.
- `plan/journal/comment-describes-superseded-behaviour.md`: the
  `applyRouteInstall` comment, met on the way through the dispatch path.
