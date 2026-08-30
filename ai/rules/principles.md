# Principles

**When:** before any decision about how to build, test, verify, or report work in this repository
**Severity:** blocking
**Related:** completion, evidence, testing, rule-precedence

## Directives

**A value that is silently wrong MUST NOT be reachable: code that cannot answer MUST say so, and MUST NOT return zero, nil, false, empty, or the default in place of an answer.** A caller cannot tell a real zero from a failure that produced one, so the defect surfaces far from its cause and reads as data. This is the single largest source of defects this repository has recorded: a type assertion that fails and disables a feature with no log line, a cross-boundary call that no-ops when the plugin runs external, a search whose zero hits are read as absence, a test whose passing assertion would also pass against a stub.

**Before stating what code does, or acting on a claim about it, you MUST read the function that PRODUCES the behavior.** A document, a comment, a commit message, a report from another agent, a test name and a decision record are each a claim by their author on the day they wrote it. They say where to look. They are never the evidence. A self-consistent story is a hypothesis; a coherent explanation that you did not verify at the producer is the shape a fabricated claim takes.

**Work MUST NOT be called done until a user reaches the behavior through the real entry point and a test proves they do.** A library that compiles, an interface that exists, a unit test that passes over the logic in isolation: none of these is the feature. The feature is the path from what the user types to what the product answers, and the proof is a test that exercises that path and would fail if the path broke.

**Writing a defect down MUST NOT be treated as addressing it.** A journal row, a tracking table, a report paragraph and a comment each change nothing about the product. Recording is a step toward a fix and never a substitute for one. The only failure that MAY be recorded instead of fixed is one you actively tried to reproduce and could not, and that record MUST carry the reproduction attempt and the next step.

**Every fact MUST be declared once, and every other surface MUST derive from that declaration.** A second copy is not a convenience: it is a future disagreement with nothing to arbitrate it. A hand-written list beside a registry, a table beside the generator that could emit it, a rule restating a page: each drifts, and the reader cannot tell which side is wrong. When a copy is unavoidable, the copy names its source and a check compares them.

**A new feature MUST register itself and be discovered; it MUST NOT require an edit to a switch, a case, a factory, a field list, or any other central enumeration.** A central list is a second declaration of what already exists, so adding a feature means editing code that has nothing to do with it, and removing one means finding every place that named it. Registration makes the feature's own package the only thing that has to change.

**The work a change owes MUST be measured by what the change can now REACH, and MUST NOT be measured by the files you edited.** The other call site, the sibling path with the same shape, the test that asserts the behavior you changed, the consumer of the name you renamed: each is inside the change whether or not it appears in the diff. A set derived from `git diff` is the edited set wearing another name.

**Scope MUST NOT be reduced, renamed, deferred, or tabled without the user deciding it, and an author MUST NOT be the reviewer of their own work.** Both failures feel like judgement from the inside. Shrinking scope reads as pragmatism, and reviewing your own change reads as efficiency because the context is already loaded. Independence is a property of the CONTEXT, not of the intention: the reader who did not write it is the only one who can find what the writer could not see.

**A rule, a document or a comment MUST NOT carry a copy of what a command already prints.** The tool answers at the moment the answer is actionable, over the tree in hand, and it cannot be stale. A cached copy is stale from the first change and costs every reader who never runs that command. Run it, read what it says, and act on that.

**Several sessions work this checkout at once, so a red, a hunk, a staged file or an artifact you did not produce MUST NOT be treated as yours to fix, wait for, or carry.** A fully green tree is unreachable by construction. Judge your own change by the evidence your own change produced, name what you could not see, and leave another session's work alone.
