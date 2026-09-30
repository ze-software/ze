# Author brief (Procedure P, author half) -- spec-rfc-verdict-test-fix-pass children

Repo: /home/thomas/Code/github.com/ze-software/ze/main (shared checkout; other agents and sessions work in it).
You are the TEST AUTHOR for one package of one child spec. Your child spec path and package are in your prompt.

READ FIRST: RULINGS.md in this directory (binding main-thread rulings), then (in this order):
1. Your child spec: Task, Owned stems, Blocked by, Method, Required Reading constraints, every row table (R-7, split-needed, missing, row-quality, mistagged, narrowing rows) -- note which rows touch your package's stems.
2. Parent plan/pre-release/spec-rfc-verdict-test-fix-pass.md: Split section (P-1..P-3), Method table, Required Reading constraints (gates facts).
3. ai/skills/ze-rfc-audit.md (four questions, STRICTNESS), docs/contributing/rfc-conformance-gates.md, ai/rules/rfc-compliance.md, ai/rules/testing.md.
4. docs/contributing/ze-go-style.md IN FULL before the first Go edit (owner directive).
5. The docs page for your package (look it up in ai/CODE-TO-DOCS.md) before reading wide code.

YOUR LIST: run the child spec's "Derived listing" jq, then keep only verdicts that tag a test in YOUR package. Drop ids listed under "Blocked by". Verdicts that also tag tests in another package of the same child: handle them (you own the verdict if your package is listed first); do not edit test files of another child.

PER VERDICT: read its audit note (rfc/audit/<stem>.json), the row quote (rfc/short/<stem>.md), the RFC text (rfc/full/<stem>.txt or rfc/drafts/), every tagged unit (grep "RFC requirement: <ID>"), and the producer. Then exactly one of:
 a) TESTS: make the tagged tests assert EVERY clause of the quoted sentence in BOTH polarities (compliant -> RFC outcome; non-compliant -> refusal), buffers isolated, the negative violating THIS requirement. Prefer new test functions. A valid {single-polarity: ...} only where no refusal path can exist AND no negative tag existed at HEAD (the coverage ratchet refuses replacing held proof).
 b) ROW CORRECTION: the row claims more than its sentence, is a fragment/list-pointer/bare pronoun, carries a wrong level, or its obligation belongs to another document -> correct/split/retire under D-2/D-3/D-7/D-10 with the exact paragraph formats (Correction <date>: / Retired <date>:) in rfc/corrections/<stem>.md; new split rows need both polarity tags + records + an extraction site (rfc/extraction/<stem>.json mapped-to or unsourced-ids). Watch D-7's 15% stop.
 c) CODE DEFECT (D-8): verified at the producer -> failing test first, then the fix with the RFC quote comment above it (ai/rules/rfc-compliance.md 2026-09-24 rule), docs page updated in the same change. If the fix is large, crosses into another child's packages, or needs a design choice, write the failing test (untagged), STOP on that verdict and report it with a recommendation.
 d) BLOCKED by a named spec not listed -> report it.
RULES:
- Before editing any EXISTING RFC-tagged test unit: `./le rfc approve unit <pkg>.<TestName> reason "D-15: <why>"` (standing approval P-1). New test functions need none.
- Use the Edit/Write tools for every file (hooks check them); never shell heredocs into the repo.
- Tag prose states only what the body asserts. For EVERY tag you add, move or change: `./le rfc discriminate-record ...` (candidates: `./le rfc discriminate stem <stem>`) and confirm it OBSERVED red. Never hand-write gomu reports to fake a mutant; use a real gomu report or the revert route.
- Run the package tests with the registered ./le action (`./le job run ...` per ai/rules/commands.md), logs under `$(./le session scratch ensure)`. gofmt clean. If the package has Linux-only/integration tests they run natively here; QEMU-guest units need `ze appliance kernel`.
- Never re-run a check to reconfirm a result you already read.
- Known unrelated reds: 18 rfc-check producer-changed records in rfc4271/7611/7705/7947/8907 (other sessions); the wait-file driver change breaking two BGP plugin tests (path-asn-filter-export-reject, redistribute-export-modify); ui 127/192/196; numberparse allowlist; the bgp/reactor link-local tests need fd00::2 on loopback. Check plan/journal/ before treating a red as new.
- A defect you walk into that does NOT block your verdicts: one row in plan/journal/<class>.md (grep first; row <= 600 chars), nothing else.
- Do NOT stamp any verdict. Do NOT commit.

HANDOFF: write <scratch>/children/<child>/<package-slug>-author.md with a table: id | resolution (tests/row/defect/blocked/unresolved) | what now proves each clause (test names, +/-) | records written | expected verdict | notes. Then a list of EVERY file you changed (exact paths). Report back <= 15 lines: counts per resolution, defects found, anything needing the main thread.

## ADDENDUM 2026-09-29
- Ledger writes (discriminate-record writes rfc/discrimination) are safe; but never run `./le rfc reseal`, `audit-stamp` or `index-update` (judges do, under a lock).
- Budget: you have about 100 tool calls. Plan for it: finish verdicts completely one at a time, write the handoff early and update it, never leave a verdict half-edited.
- Continuation: if a previous author's handoff exists for your package, read it first and continue from its "unresolved" rows; append to it.
- KEEP THE TREE COMPILING (2026-09-29, binding): judges' commits build the whole tree, so a half-done edit in your package blocks every other agent's commit. A failing-first test must COMPILE and fail at run time (stub the new function first with the old behaviour, then change it). Never leave a renamed/removed symbol with callers pending across tool calls longer than necessary; finish a signature change and all its callers in one step.
- NEW ROW IDS (binding): a new id is <Prefix>-<the exact section it cites>-<n> with n above that section's high-water mark (checkIDAllocation). Cite §3.1.6 -> id RFC2205-3.1.6-1, never RFC2205-3-46. Judges have had to rename many ids; get it right first time.

ADDENDUM (2026-09-30, owner: work is being lost at cutoffs). APPEND to your handoff after EACH id you finish (one row, files touched), not at the end. Before your 80th tool call, stop starting new ids: write the handoff, list what is half-done and exactly where. A cutoff must never lose a finished verdict's record.
NOTE: the budget is enforced by a hook (100 Bash+Write+Edit calls per agent, resumes included). Past it, only a write naming the session-state file passes, so a handoff append to scratch/ is REFUSED after the cap. Hence: append per id, and finish your handoff before call 90.

ADDENDUM (2026-09-30, lock scope). `./le rfc discriminate-record` writes only rfc/discrimination/<stem>.json, so take a PER-STEM lock for it: `flock scratch/children/ledger-<stem>.lock ./le rfc discriminate-record ...` (e.g. ledger-rfc4271.lock). Keep the GLOBAL `scratch/children/ledger.lock` for corpus-wide steps only: `./le rfc reseal`, `./le rfc audit-stamp`, `./le rfc index-update`. A long record run under the global lock stalls every other agent.
