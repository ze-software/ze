# Pre-Release, and the Product Is the Deliverable

**When:** when a test or a gate goes red, when you are about to re-run a check, or when you are deciding what a commit owes
**Severity:** blocking
**Related:** completion, precommit-verify, testing, git-safety

## Directives

**Ze is PRE-RELEASE. There is no release, no version, no tag, and nobody consumes `main` (owner directive, 2026-08-30).** A red gate on `main` reaches no user, breaks no deployment, and costs no user anything. The only cost a red carries today is the cost of the session that stops to clear it.

**The deliverable is the SOFTWARE. Test code, gate plumbing and verification bookkeeping are INSTRUMENTS, and they MUST NOT be treated as the work.** An instrument earns its cost only by telling you whether the product works.
**A session whose diff is mostly instrument repair delivered nothing, and it MUST report that outcome in those words.** The measure is the diff, not the effort: fixture edits, runner flags, golden files, debt rows and gate bookkeeping are all instrument.

**A red test while the work is in progress is NORMAL, and it MUST NOT hold a commit.** Commit the product change, and name the red test and its cause in one line of the commit body.
**A green tree is owed at NO commit.** An agent that batches finished product work until the tree goes green has chosen the most expensive failure this repository has, which is work that was finished and never landed.

**Every red gets ONE question before anything else: does this red say the PRODUCT is wrong?** Answer it by reading the failing assertion and the function that produces the value it names (`ai/rules/evidence.md`). The answer takes one read, and it decides everything that follows.
**A red that says the product is wrong IS the work. It MUST be fixed at the source, and never by weakening the assertion** (`ai/rules/completion.md` governs it from there).
**A red that says the SCAFFOLDING is wrong MUST be left red, named in one line, and stepped over.** Fixture drift, a stale golden file, a runner flag, a harness path, a gate's own bookkeeping: each costs product time and buys the product nothing.
**A scaffolding red is repaired only when it BLINDS you.** The test you cannot read the product's behavior through is worth the repair. The one that merely prints red is not.

**A check that has run MUST NOT be run again to reconfirm what its output already said.** One run, read the whole output, act on it.
**Three re-runs are forbidden by name: a gate that passed, a log you have already read, and a tree unchanged since the last run.** A re-run is earned by one thing only, which is an EDIT to the code under test.
**Re-reading is the same waste as re-running, and it MUST NOT be spent either.** Opening the same failure summary a second time to look for something new is the tell that the session is avoiding the product.

**Test and gate repair MUST NOT become the session. Before starting a THIRD repair to test or gate scaffolding in one session, stop, return to the product code, and report what stays red.** Two is the budget because the third repair is where a session stops noticing it has changed subject.
**The count is per session, and a new spec, a compaction, or a hand-off MUST NOT reset it.** The budget exists to bound the session's tokens, and none of those three gives the session more.

**A commit MUST NOT be held for a green gate.** A local commit reaches nobody, so it can cost nobody anything, and it protects finished work from the next crash or checkout.
**The gate is owed before a PUSH, which is the act that reaches a reader** (`ai/rules/git-safety.md` carries the verification-debt route). Verification debt is a record of what a push still owes, and it MUST NOT be read as a reason to stop committing.

## What This Never Relaxes

This rule is about the INSTRUMENTS. Every rule below is about the SOFTWARE, and none of them moves.

| Unchanged | Why it is not instrument work |
|-----------|-------------------------------|
| Product correctness, RFC conformance, interop (`ai/rules/rfc-compliance.md`) | They are properties of the thing being shipped, and a peer that rejects ze has met a product defect |
| The ban on deleting or weakening a test to clear a red (`ai/rules/testing.md`) | Weakening hides a product defect. Leaving the test red hides nothing, so it is the permitted move |
| The ban on calling half-written product code finished (`ai/rules/completion.md`) | A completion claim is about the product, and a red test never earns one either way |
| A structural gate charged to your own commit: lint, generated artifacts, tier | It costs seconds rather than the gate's half hour, and it says the tree is BROKEN rather than merely unverified |
| Reading the producer before claiming what code does (`ai/rules/evidence.md`) | Routing a red needs the same one read this rule already asks for |

## Rationale

The gate machinery in this repository was built for a product that ships. It was then applied to a product that has never shipped, and the cost landed entirely on the sessions that write the product.

Two rules pulled in the same direction and compounded. One made every red a defect its finder owns. The other made a green gate a precondition of a commit. Together they turned a fixture drift into a session: find red, own red, repair red, re-run gate, find the next red. The product code that the session was opened to write is what got dropped.

The owner's ruling on 2026-08-30 ends that compounding by naming which of the two things is the deliverable. Nothing about product quality changes, because product quality was never what the sessions were spending on.
