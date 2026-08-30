# Documentation First

**When:** before you spawn an agent or open a search to learn how a surface works, and whenever an edit changes behavior a page describes
**Severity:** blocking
**Related:** completion, evidence, context-economy, repo-maintenance, writing

## Directives

**You MUST NOT spawn an agent to establish how a surface works before you READ the page that covers it.** The ban covers a broad search and a wide code read too.
**The lookup costs one command.** `ai/CODE-TO-DOCS.md` names the pages for a file, `ai/DOCS-TO-CODE.md` the files for a page, and `ai/INDEX.md` the page for a keyword. A surface that no index reaches is itself a finding, and the next point governs it.

**An investigation MUST be authorized by a GAP you can name.** There are two: the page is SILENT on the question, or the page DISAGREES with the code.
**You MUST name the page you read, and the sentence that is silent or wrong, before the agent starts.** "I did not check" is not a gap, and neither is "the code is faster to read". An investigation that names no page skipped the cheapest answer.

**A page that disagrees with the code is a DEFECT, and it MUST be repaired in the work that found it.** Read the producing function to learn which side is wrong (`ai/rules/evidence.md`). Correct the page when the code is right, and the code when the page states the contract the code owes.
**Reporting the disagreement is not repairing it (`ai/rules/completion.md`).** A line in a report leaves every later reader believing the page, and the next session pays for the same investigation.

**A change that makes a page wrong MUST carry its page edit in the SAME piece of work.** That edit lands before the next code edit starts. A page is wrong from the moment the behavior it describes changes. The window where nobody notices is the cost this rule removes.
**`ai/rules/repo-maintenance.md` owns WHICH page each change obliges you to update. This point owns WHEN: now, not at review, not at closure, not in a follow-up commit.**

**You MUST NOT defer documentation to a final pass, a closing commit, a review gate, or a follow-up spec.** A batched pass is written from memory, about code that moved three times since. It records what you INTENDED rather than what shipped.
**A skill whose last steps check documentation MUST NOT be read as permission to write none until then.** Those steps CHECK the pages the work already updated. A check that finds nothing updated is a failed step.

## Where The Doc Lives

**You MUST read the surface that answers your question before you search for the answer:**

| Question | Read this first |
|----------|-----------------|
| How is this Go file's surface meant to work | Its `// Design:` header, then every page `ai/CODE-TO-DOCS.md` lists for it |
| Where is this documented contract implemented | `ai/DOCS-TO-CODE.md` |
| Which page owns this topic, keyword, or tool | `ai/INDEX.md` |
| Which rule governs this task | `ai/rules/INDEX.md`, then that rule's file in full |
| What does this command do for a user | `docs/guide/`, then `docs/features/` |
| What is this component's contract or data flow | The owning page under `docs/architecture/` |

## What This Rule Never Licenses

**A page MUST NOT be cited as evidence for what the code does.** It says where to look, and what the surface was meant to be. `ai/rules/evidence.md` is unchanged: a behavioral claim still requires the producing function to be read.
**A page that agrees with your reading is a second opinion, never a proof.** Reading first and believing are different acts, and this rule asks for the first one.
