# Evidence and Guards

**When:** stating what code does, acting on recorded claims, writing or reviewing a guard, or writing any string that enumerates data a registry already holds
**Severity:** blocking
**Related:** writing, planning, protocol

## Directives

- **Coherence is not evidence.** A self-consistent story is a hypothesis, and breadth of reading does not stand in for the one function the claim depends on. You MUST name the keystone fact and read its producer before you report a finding.
- **Zero hits is not absence.** Before you report that something is not there, you MUST run your pattern against a case you KNOW exists. When the control also returns nothing, the pattern is wrong and the corpus was never asked.
- **A peer's CORRECTION arrives already feeling checked.** An assertion invites doubt and a correction implies somebody did the work, so it costs nothing to adopt and nothing pushes back. You MUST verify it at the producer before you act on it, and above all before you relay it to the owner.
- **A pending result is not a result.** You MUST NOT record, act on, or report a verdict before the thing that produces it has reported. Being right by luck leaves the same false record, and the pressure is strongest at the last step before a commit.
- **The named example is the worst sample.** The instance a document NAMES is the one its author already looked at, so it is the member most likely to satisfy the claim and the least informative to check. You MUST derive the set the claim covers and count how many members hold.

## No Fabrication

**Before you claim code behaves a certain way, or recommend work premised on it, you MUST read the function that PRODUCES the behavior.** Naming the caller, the config that reaches it, or a comment beside it does not discharge this.

**You MUST read the implementation source this session before you write a spec or a design.** `writeDesignEvidence` (`internal/le/hookruntime/writeedit.go`) refuses the write when the session recorded no source read, and it is a BACKSTOP you MUST NOT treat as evidence: it accepts ANY recorded read rather than the spec's own subject.

**A line number MUST NOT appear in a document unless a GENERATOR maintains it (owner directive, 2026-08-03).** Nothing refreshes a hand-typed one and nothing can tell it has gone wrong, so name the file and the symbol instead. A file earns the exception by declaring `GENERATED ... do not edit` in its first ten lines, which is what `writeLineCitation` reads rather than a list of filenames.
