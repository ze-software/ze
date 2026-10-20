# Owner Meta Decisions

Decisions Thomas made about how agents work and what is in scope. Every main
session loads this file at start: `ai/INSTRUCTIONS.md` includes it, so it
reaches `AGENTS.md`. It is the one home for these decisions. A rule that applies
one points here by id, and never restates the reasoning.

A question one of these rows answers is not put to Thomas. Act on the row and
name it in your report. A new meta decision is appended here in the turn he
states it, and committed then (`ai/rules/rule-precedence.md`).

| Id | Date | Decision | Thomas's words | Applied in |
|----|------|----------|----------------|------------|
| M1 | 2026-10-20 | VMs and test guests run Ze's own system (Ze's kernel and base, as the appliance builds it), not Alpine. A problem that exists only because a guest runs Alpine is out of scope: the fix is moving that guest to Ze's system, never a question and never a test-side workaround for Alpine | "Alpine is not the right kernel and base for the OS. We should use our own system when we build VMs and therefore Alpine questions are out of scope." | `ai/rules/platform-linux.md` |
| M2 | 2026-10-20 | Every RFC Ze implements gets full, correct coverage. The only exception is a basic primitive Linux cannot provide. An absent requirement of an implemented RFC is work to do, not a gap awaiting permission; this supersedes the 2026-10-08 freeze on RFC enrolment. A question the RFC answers is not asked; where the RFC is silent, FRR's and BIRD's source answers it (2026-10-12) | "Unless Linux is unable to provide us the basic primitive ... we should ensure that we have fully correct RFC implementation. So if a question can be answered that way, it shouldn't be asked." | `ai/rules/rfc-compliance.md` |
| M3 | 2026-10-20 | An agreed design answers the questions that follow from it. ExaBGP backward compatibility (6.0, 5.0, 4.2) is agreed, so what it needs is done, not asked. It lives only in the bridge and migration (`ai/rules/go-standards.md`), and the RFC outranks ExaBGP behaviour | "We said we wanted to have exaBGP backward compatibility and therefore if we need to do things we should ensure that we are able to be backward compatible." | `ai/rules/rule-precedence.md` |
| M4 | 2026-10-20 | A logic bug or misbehaviour is fixed, never put to Thomas as a question. The goal is the best possible NOS and plugins | "When you ask me if we should fix a logical bug ... Clearly the software must behave correctly." | `ai/rules/completion.md`, `ai/rules/rule-precedence.md` |
| M5 | 2026-08-14 | The goal behind all work is shipping Ze in production as a NOS. A change to the shipped binary outranks tooling, and "recorded" never counts as closed | Restated 2026-08-08, 08-09 and 08-14 | this row |
| M6 | 2026-09-08 | Before a protocol design is approved, read FRR's and BIRD's source and quote the deciding function | Asks how FRR and BIRD do it before approving a protocol design | this row |
| M7 | 2026-09-26 | Sessions share the one `main` checkout. No worktree per session: fix shared-checkout friction inside that model | Declined after the AI-setup review | this row |
| M8 | 2026-10-15 | Close in-progress specs before opening new ones. The WIP cap (`ZE_SPEC_WIP_CAP`) stays at 22 and is lowered as specs close, never raised | "let's try to close from now on" | this row |
| M9 | 2026-10-17 | A handover lives in the repository (`plan/handover/`), committed the turn it is written, never only in a session directory | "handover must NEVER be in the session" | `.claude/rules/handover-in-repo.md` |
