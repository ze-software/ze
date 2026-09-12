# Learned: a paraphrase of a requirement is a second declaration of it

From `spec-config-apply-ordering-covers-every-root`, closed 2026-09-12. Sixteen
commits, seven review rounds, six of which found a product defect.

The config transaction orders what a reload applies. The order is the owner's. He
gave it in May 2026, a spec kept a summary of it, and the words themselves were
never written down. The summary said make-before-break: add the new address,
update the services that bind it, remove the old address last. What he asked for
is the opposite, and he had said why:

> if an IP is deleted to move, the move require BGP for the peer to be stopped,
> then the move then readded

The subsystem was built from the summary. So were its tests, its two `.ci` files,
its comments and its design page. Four months later the code, the tests and the
documentation all agreed with each other and all disagreed with the requirement.

## Nothing inside the tree could find it

This is the part worth carrying. The defect was not hidden by complexity, and no
amount of reading would have surfaced it. Every artifact that could have
contradicted the paraphrase was derived from the paraphrase:

- The unit tests asserted the applied order against a model the same author
  wrote, so they were green under the wrong policy and would have been green
  under the right one.
- The two functional tests rotated BGP router-ids and asserted that peers
  reconnect. Their own comments admitted the address operations they stand in for
  are not the ones they emit, so nothing ever read the order a commit APPLIES.
- The design page described the code, accurately, which is what made it useless
  as an arbiter.

The arbitrating text was one sentence long and was not in the repository. The
repair is that sentence, quoted verbatim, under "The requirement" in
`docs/architecture/config/apply-ordering.md`, with the page declared to outrank
the spec wherever they disagree.

**Routing.** The page carries the owner's words, which is where this lesson
lives for anyone who touches the subsystem again. `ai/rules/principles.md`
already carries the general form, "every fact MUST be declared once, and every
other surface MUST derive from that declaration". What it does not say is that a
requirement a person STATES is such a fact, and that a spec's restatement of it
is the second copy. A rule making that explicit is proposed to the owner rather
than written here, because it changes what every spec owes:

> **An owner requirement MUST be recorded verbatim on a durable page, and every
> spec, comment and test that depends on it MUST point at that page rather than
> restate it.** A summary of a requirement is a second declaration of it and
> drifts like any other copy, and the code agreeing with the summary is not
> evidence, because the code was built from it. Where the requirement arrived in
> conversation, the quote carries its date and the words are not edited.

## Coverage without ordering is not ordering

The spec's own fix reproduced its own defect one level down.

The defect was that a transaction with one undecomposed root dropped the WHOLE
transaction to an unordered apply. The fix made every root a node in the graph.
But a node that declares neither what it produces nor what it consumes can earn
no edge, so an undecomposed root was now IN the graph and still ordered by
nothing: its position was the slice tie-break. Review round 1 found that. Round 2
found the repair was shape-dependent. Round 4 found the same hole on the
decomposed side, where an address arriving through an opaque operation left the
binder that consumes it ordered against nothing.

Three rounds, three instances, one sentence: **a node that declares nothing is
ordered by nothing, and being present in the graph is not being ordered by it.**
The answer that finally held is not a better insertion point. It is
`operationPhase`: every operation lands on one of three rungs, a stop, an
addressing change and a start, and the sort drains the lowest ready rung. A rung
is a property of the operation, so an opaque one still has a place.

**Routing.** `docs/architecture/config/apply-ordering.md` under "The fail-safe
default", which states which way an unreadable operation falls and why.

## A count is not a proof

`test/reload/config-apply-ordering-address-swap.ci` asserted
`option=tcp_connections:value=2` to show the daemon stopped and restarted a peer
across an address move.

A daemon that emits no peer operation at all reaches two connections on its own
retry timer. Measured under the revert the test exists to catch: the peer
reported success after two connections with the defect present. The test passed
under exactly the build it was written to fail on.

What made it discriminate is a signal only the daemon can produce: the check peer
now holds its session open (`option=linger`), so a second connection can only
follow a daemon-side drop, and the driver waits for a marker the returned session
writes after it re-announces its route.

**Routing.** `docs/architecture/testing/ci-format.md`, on the `tcp_connections`
row, where the next author reaches for it. The `linger` row already carries the
mechanism. A row in `plan/journal/green-that-could-not-have-been-red.md` records
the instance.

## A mechanism that has never run reports success

`TxCoordinator.Execute` took the ordered path only when operations existed AND no
participant was uncovered. Twenty-two non-test `WantsConfig` declarations name
the `bgp` root, across 21 plugins, and not one of those plugins registers a
decomposer, so every `bgp` reload was uncovered and fell to the unordered apply,
at `Info`, reporting success.

Two of the nine ordering rules were dead in the same way. One never fired in its
life: it compared an address operation's interface against a field an interface
operation does not carry, so its right-hand side was empty for every pair it ever
saw. A rule that matches nothing and a rule that is correct look identical from
outside the graph.

The tell is the same in all three cases: the thing that would have reported the
failure was the thing that was off. When a mechanism has a fallback, ask how
often the fallback runs before asking whether the mechanism is correct.

**Routing.** A row in `plan/journal/unwired-feature.md` already collects this
class, and `ai/rules/completion.md` already requires a non-test caller for a new
exported symbol. What this instance adds is the fallback shape, which is recorded
in the spec's Design Insights rather than as a new rule.

## What made the later review rounds productive

Seven rounds ran. Rounds 1 to 6 each found a product defect, and rounds 3 and 4
found defects in code earlier rounds had passed over.

The difference was not diligence. Each later round was given a NAMED scope: the
previous round's repair and what it touched, plus a fixed list of re-verifications
(the five phases at their producers, the fail-safe direction at every decision
point, the QEMU evidence against the code as it now stands). A round told to
review generally re-reads what the last round already read. A round told to
attack the repair reads the code the repair created, which nobody has read yet.

Both extra rounds past the cap were authorized by the owner, and round 6 earned
round 7 by finding a defect in round 5's own fix.

**Routing.** `ai/rules/planning.md` already requires each round's scope to be
written into the Review Gate before it runs. This is evidence that the rule pays,
so nothing new is owed.
