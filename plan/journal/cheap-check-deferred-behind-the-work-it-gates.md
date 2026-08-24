# The cheap check that decides the work, run after the work

A spec rests on one claim it cannot see around. The claim has a validation, the
validation is cheap, and the spec says so: the row is written, the method is
named, the status reads `unvalidated`. Then the implementation phases run,
because each of them fixes something real on its own merits, and every one of
those fixes is correct. Nobody is wrong about any of the code. What nobody does
is the ten minutes that would say whether the claim is true.

When it is finally run, at closure, the claim is false, and the work has to be
described in a way its plan never anticipated: real defects fixed, and the
problem the spec exists for still open.

This is not the same failure as never recording the assumption. The record was
correct. The two things that make the class are ORDER and COST: a validation
that is cheaper than any phase it gates was scheduled after all of them, and the
phases were shippable enough that nothing ever forced the question. A spec is
most exposed to this when its findings stand alone, because then the keystone
never blocks anything and its falsity costs nothing until the closing summary
has to say what was achieved.

The habit it asks for: when an assumption is the reason the spec EXISTS, its
validation is phase one and it is a gate, not a task. Cheapness is the argument
for running it first, never the reason it can wait.

| Date | Spec | Surface | Symptom | Fix |
|------|------|---------|---------|-----|
| 2026-08-24 | fixit-firewall-concurrency-deadlock | spec phase order, ddos and firewall lock discipline | A-6, the spec's keystone, said the command that stalled dispatch for 255s on 2026-07-12 was one taking the ddos-local responder mutex. It was reasoned from code and it was the only chain the tree supported, so five design decisions and four implementation phases were built on it. Its stated validation was to read the `.ci` that observed the stall and see which command it dispatched. That took ten minutes and had been available for 39 days. Run at last in phase 1, it came back FALSE: the probe in `test/plugin/ddos-direction.ci` dispatched `show ddos incidents` and nothing else, and `handleShowDdosIncidents` (`internal/plugins/ddos/observe/show.go`) loads `activeStore` atomically then calls `(*store).list`, which holds `store.mu` over an in-memory ring copy. Every `store.mu` holder in that package is in-memory. No firewall lock is on that path, so the proposed chain was never the one that stalled | Nothing to fix in the code, and that is the point. D-1 through D-5 are six real defects read from the producing functions, each proven by a test that reds when it is reverted, and all of them stay. What changed is the CLAIM: the spec closes on AC-1's cannot-reproduce branch, the audit row for root-causing the hang reads NOT MET rather than Partial, and Known Limitations opens by saying the 255s stall was never explained. The 2026-07-12 mechanism is still unidentified and no dump can be taken now |
