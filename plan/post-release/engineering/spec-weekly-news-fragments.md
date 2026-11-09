# Spec: weekly-news-fragments

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | tooling |
| Priority | P3 |
| Release reason | Development tooling for the weekly public update and the release notes; no shipped behaviour changes |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-09 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The weekly update is written from about a thousand commits a week. Commits say
what changed, but five facts the update needs never reach them, so the weekly
writer reconstructs them after the fact by grepping and reading code: what an
operator would have hit before the change, whether it corrects an earlier
ze-news claim, which configurations a security fix exposed, bugs reported
upstream, and who outside the project asked for it. The agent that did the work
knew each of them.

Thomas agreed (2026-11-07 and 2026-11-09 sessions):

| Decision | Choice |
|----------|--------|
| Who writes it | The agent closing a change writes one news fragment carrying those facts, at the moment it knows them |
| Required | Every spec closure owes a fragment, or a `no-news "<reason>"` statement; no path classifier decides it |
| Fixes outside a spec | Written by the fixing agent when operator-visible, by skill instruction only; no check |
| Storage | One file per change under `website/changes/fragments/`, kept permanently; the site does not publish it |
| Shape | Front matter (spec or journal row, topic, optional feature id, optional `found-by`) and six fixed headings; "none" allowed except under Before and After |
| Found by | Optional `found-by` names what surfaced the defect a fix fragment describes, from a closed list declared once in `newsfragment.go`: `rfc-check`, `interop`, `test-judge`, `review`, `operator-report`, `upstream`. `spec-weekly-event-report` lists fix fragments by this value; a test-judge refusal gets no record of its own |
| Consumer | The weekly-update skill starts from fragments, verifies every claim at the producer, and selects. A fragment is a claim, never evidence |
| Release notes | Derived from the fragments added since the previous release, not from commit subjects (`spec-release-build` AC-25 changes) |
| Measure | Over the next two weekly updates, per published item: which of the five facts came from a fragment, from research, or failed verification, recorded in `plan/records/weekly-fragment-measure.md` |

The companion spec `spec-weekly-event-report` covers the derived report of
tooling events (recorded breaks, judge verdicts, interop red/green).

## Required Reading

### Architecture Docs
- [ ] `ai/doors/system.md`, `ai/doors/le-tooling.md` - the design every le change keeps
  → Constraint: a commit gate that refuses wrongly blocks every concurrent session at once (`plan/learned/HOOK-FRICTION.md`); each refusal names the exact file to write or the exact keyword that admits the commit
  → Constraint: le never links into shipped `ze`; nothing here touches product code
- [ ] `docs/contributing/committing.md` - the commit flow, keywords and refusals
  → Constraint: a new keyword is one row in the closed `createParameters` table plus a `createPolicy` row and an `Options` field (`internal/le/commit/actions.go`); a reason keyword becomes a trailer through `admissionTrailer` (`prepare.go`)
- [ ] `ai/skills/ze-close.md` - canonical closure skill; `.claude/skills/` is a generated copy (`internal/le/ai/sync/ai.go`)
  → Decision: the fragment is written in step 6 and carried by commit A; `no-news "<reason>"` rides on commit B
  → Constraint: edit `ai/skills/*.md` only, then `./le ai sync update`
- [ ] `ai/skills/ze-weekly-update.md` - the consumer; Phase 1 step 7 lists the facts researched by hand today
  → Decision: Phase 1 gains a step that lists the fragments added in the range; step 7 becomes verification of fragment claims plus research only for what no fragment carries
- [ ] `plan/learned/HOOK-FRICTION.md` F12 - a slug-keyed learned summary was treated as proof of closure and misfired
  → Constraint: a fragment's presence or name is never read as proof that a spec closed; closure stays the removed spec (`closedSpecStem`)
- [ ] `ai/rules/planning.md` - lessons update the system; no learned summary is written any more
  → Constraint: a fragment is operator-facing and carries no agent lessons
- [ ] `plan/release/delivery/spec-release-build.md` AC-25, AC-26 - release notes planned from commit subjects
  → Decision: AC-25 is restated to derive the release notes from fragments added since the previous release tag

**Key insights:**
- Closure is detectable only at commit B, the one that removes the spec; commit A is byte-for-byte an ordinary commit (`internal/le/commit/review.go`, comment above `closureStemSelected`).
- `specWorkPopulation` gathers every commit and path attributed to a spec through `Spec-work:`; the fragment gate uses it to tell this spec's fragment from a stale same-named one.
- `prospectiveTree` already holds HEAD plus the earlier blocks of an append script plus the carried files, so a fragment carried in commit A of the same script is visible when commit B is judged.
- The site's changes producer reads only `changes/posts` (`internal/le/site/changes.go` `changesSourceDirectory`), so fragments beside it are not published.
- The topic vocabulary is `website/data/topics.json`, read by `loadTopicVocabulary`; the fragment gate must use the same reader, exported once.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/le/commit/prepare.go` - `Create` runs the gates as a fixed sequence in its `native` arm: source gates, verification gates, `specMechanismsRefusal`, `dependencyRefusal`, ledger trailers; `Spec-work:` is appended last from the claim
- [ ] `internal/le/commit/review.go` - `closedSpecStem` names the one spec a removal closes; `specWorkPopulation` lists the commits and paths carrying `Spec-work: <stem>`
- [ ] `internal/le/commit/specmechanisms.go` - `specMechanismsRefusal`, the model for a gate over carried content: reads through `workingTreeSides`, `couldNotRun` when it cannot read, plain error to refuse
- [ ] `internal/le/commit/snapshot.go` - `prospectiveTree` and `prospectiveContent`: the tree the commit produces, earlier script blocks included
- [ ] `internal/le/commit/actions.go` - `declaredTrailers` is the one declaration of trailer keys; `OwesNothing` marks a fact trailer such as `Spec-work:`; a hand-written trailer line is refused
- [ ] `internal/le/weekly/weekly.go` - parses a post's front matter and splits the body for Discord; reads no commits, specs or fragments
- [ ] `internal/le/site/changes.go` - `changesSourceDirectory` is `changes/posts`; `loadTopicVocabulary` reads the tag families from `data/topics.json`

**Behavior to preserve:**
- Every existing commit gate and its order; the closure review still answers first that a review is owed
- A commit with no claim carries no `Spec-work:` and is not a closure; a relocation or rename of a spec is not a closure
- The site build publishes the same pages as today
- The weekly publish path, chunking, archive and resume
- The weekly skill's hard gates: verify at the producer, no internal vocabulary in the post, owner approval before posting

**Behavior to change:**
- A spec closure is refused unless a valid fragment naming the spec is in the prospective tree and was written by this spec's work, or the closure carries `no-news "<reason>"`
- Any commit that adds or edits a fragment has it validated
- The closure skill writes the fragment; the weekly skill reads fragments first and records the measure; release notes derive from fragments

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Closure detection | `internal/le/commit/review.go` `closedSpecStem` | Names the spec a commit closes from a removed spec path | Extends: the closure half of the fragment gate runs only when it answers a stem |
| Spec change set | `internal/le/commit/review.go` `specWorkPopulation` | Lists every commit and path attributed to a spec | Extends: proves the fragment was written by this spec's work |
| Prospective tree | `internal/le/commit/snapshot.go` `prospectiveTree` | The tree the commit produces, earlier script blocks included | Extends: the gate reads the fragment from it |
| Carried-content gate | `internal/le/commit/specmechanisms.go` `specMechanismsRefusal` | Refuses an edited spec that lacks required sections | Extends the pattern: the fragment gate is a sibling function called from `Create` after it |
| Reason keyword with trailer | `internal/le/commit/actions.go` `createParameters`, `createPolicy`; `prepare.go` `admissionTrailer` | `review-override`, `rfc-change-ok` admit a refused commit and record the reason | Extends: `no-news "<reason>"` is one more reason keyword |
| Trailer declaration | `internal/le/commit/actions.go` `declaredTrailers` | One list of trailer keys; refuses hand-written ones | Extends: one `No-news:` row, `OwesNothing` like `Spec-work:` |
| Topic vocabulary | `internal/le/site/changes.go` `loadTopicVocabulary`, `website/data/topics.json` | Validates a post's tags | Extends: exported once and reused by the fragment gate; no copy of the list |
| Closure skill | `ai/skills/ze-close.md` step 6 | Commit A lands code and spec, commit B removes the spec | Extends: step 6 writes the fragment, or picks `no-news` |
| Weekly skill | `ai/skills/ze-weekly-update.md` Phase 1 | Researches the five facts from commits and code | Extends: reads fragments first, verifies them, researches only the rest, records the measure |
| Weekly publisher | `internal/le/weekly` | Publishes an approved post and archives it | Does not apply: publishing is unchanged |
| Feature declaration | `features/<id>.md` | Current state and level of a feature | Does not apply as storage: a fragment names the feature id instead of restating level or scope |
| Learned summary | `plan/learned/NNN-*.md` | Agent lessons, no longer written | Does not apply: different audience; F12 forbids slug-keyed files as closure signals |
| Release changelog (planned) | `plan/release/delivery/spec-release-build.md` AC-25 | Would build release notes from commit subjects | Replaced in that spec: release notes derive from fragments |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- The closing agent writes `website/changes/fragments/<spec-stem>.md` in `/ze-close` step 6 and names it in commit A; a fixing agent outside a spec writes `website/changes/fragments/<journal-class>-<slug>.md`
- `./le commit create` judges it; the weekly writer and the release build read it

### Transformation Path
1. Any commit carrying a fragment path: the gate parses it and validates front matter and headings
2. A closure (commit B): `closedSpecStem` answers the stem; the gate requires `website/changes/fragments/<stem>.md` in the prospective tree, naming the stem, with its path among `specWorkPopulation` paths or this script's paths; or the `no-news` keyword, which becomes a `No-news:` trailer
3. Weekly Phase 1: `git log --diff-filter=A` over `website/changes/fragments/` between `release-from` and `release-to` lists the week's fragments; each claim is verified at the producer before use
4. Release build: the fragments added since the previous release tag become the release notes

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Agent → commit tooling | Fragment file plus optional keyword | No |
| Repository → weekly writer | Fragment files listed from git history | No |
| Repository → release build | Fragment files since the previous tag | No |

### Changed-Wire Census

**Search evidence:** scope `internal/le/commit`, `internal/le/weekly`, `internal/le/site`, `internal/le/build`, `ai/skills`, `website/`; terms `fragment`, `no-news`, `No-news`, `changes/fragments`; no existing writer or reader of either format. Control: `Spec-work:` has a known writer (`prepare.go` `Create`) and reader (`review.go` `specWorkPopulation`, `owed_trailers.go` push derivation).

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| News fragment file | writer | agent, by `ai/skills/ze-close.md` step 6 and `ai/skills/` fix guidance | New | New format, no migration |
| News fragment file | reader | new `newsFragmentRefusal` in `internal/le/commit/newsfragment.go` | New | New format |
| News fragment file | reader | `ai/skills/ze-weekly-update.md` Phase 1 (agent) | New | New format |
| News fragment file | reader | release notes in `internal/le/build/release/` (planned by `spec-release-build`) | Not yet written | That spec's AC-25 is restated |
| News fragment file | reader | `internal/le/site/changes.go` `changesSourceDirectory` | Reads `changes/posts` only | Preserved: fragments stay unpublished, proved by a test |
| No-news trailer | writer | `internal/le/commit/prepare.go` `Create` via `admissionTrailer` | Pattern of `review-override` | New trailer, declared in `declaredTrailers` |
| No-news trailer | reader | `internal/le/commit/owed_trailers.go` push derivation | Skips `OwesNothing` rows | Declared `OwesNothing`, nothing owed at push |

### Integration Points
- `Create` in `internal/le/commit/prepare.go` - one call after `specMechanismsRefusal`, native arm only
- `loadTopicVocabulary` in `internal/le/site/changes.go` - exported (or moved to a leaf package if importing `site` from `commit` pulls the renderer) and called by both the site and the gate

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | to be filled at implementation |
| No unintended coupling (components stay isolated) | No | to be filled: `commit` reaches the topic reader only |
| No duplicated functionality (extends existing, does not recreate) | No | to be filled: one topic reader, one closure detector |
| Zero-copy preserved where applicable (refs, not copies) | No | N-A, tooling |
| Registration over hardcoding, outbound | No | to be filled: the gate is one call in `Create`, matching how every gate there is added |
| Registration over hardcoding, inbound | No | to be filled: topics come from `topics.json`, headings from one declaration in `newsfragment.go` that the README cites |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Judging at commit B over the spec's `Spec-work:` population is enough | `review.go`: commit A is indistinguishable | Spec work landed without a claim is missed | `TestNewsFragmentClosureFindsFragmentFromEarlierSpecCommit` | unvalidated |
| A-2 | Agents write specific fragments when the format demands operator-visible Before and After | Thomas's agreement | Fragments become filler | The two-week measure | unvalidated |
| A-3 | Fixes outside a spec are covered well enough by the skill instruction | Thomas's choice; no marker exists | Many user-visible fixes reach the week with no fragment | Count fragment-less fixes in the two measured weeks | unvalidated |
| A-4 | No site check scans every Markdown file under `website/`, so fragments are neither published nor refused | `changes.go` reads `changes/posts` only | `./le site check` refuses or publishes fragments | `TestFragmentsAreNotPublished` plus one `./le site build` | unvalidated |
| A-5 | Importing the topic reader from `internal/le/commit` does not pull the site renderer | Not read | Commit tooling grows a heavy dependency | Implementation reads the import graph; moves the reader to a leaf package if needed | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The gate misfires and blocks closures across sessions | Refusals on tooling-only specs | The refusal names the fragment path to write and `no-news "<reason>"`; every refusal path has a unit test |
| R-2 | Fragment claims are trusted as evidence in the post | A published claim with no producer check | Weekly skill hard gate: each fragment claim is verified at the producer |
| R-3 | Agents write "none" under every optional heading | The measure shows research still needed for Corrects, Exposure, Upstream | Before and After refuse "none"; the measure decides whether stricter rules are needed |
| R-4 | A fragment from an earlier spec of the same name satisfies a later closure | Same stem reused | The fragment path must be in this spec's population or this script |
| R-5 | A closure script carries the fragment in commit A and the gate does not see it at commit B | Refusal on a correct closure | The gate reads the prospective tree, which includes earlier script blocks; covered by a test |
| R-6 | The gate cannot read a fragment or git history | Unreadable file, git failure | `couldNotRun` (exit 3), never a pass |
| R-7 | A hand-written `No-news:` line bypasses the keyword | Trailer in a body | `declaredTrailers` row makes `refuseHandWrittenTrailer` refuse it |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Spec closures refused until a fragment or `no-news` is given; nothing user-visible in Ze |
| How is it reverted? | Single commit revert of the gate; fragments stay as harmless files |
| Who else touches this path? | Every session's `./le commit create`; `spec-release-build` for the release notes; `spec-weekly-event-report` for the weekly skill |

## Wiring Test (MANDATORY -- NOT deferrable)
| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le commit create ... remove <spec>` with no fragment | → | `newsFragmentRefusal` called from `Create` | `TestCreateRefusesClosureWithoutNewsFragment` |
| `./le commit create ... remove <spec> no-news "<reason>"` | → | `admissionTrailer` writes `No-news:` | `TestCreateNoNewsWritesTrailer` |

## Acceptance Criteria
| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Closure whose prospective tree holds a valid `website/changes/fragments/<stem>.md` written by this spec's work | The script is written and the commit prepared as today | Every other gate still refuses on its own grounds, because this gate adds a refusal and removes none |
| AC-2 | Closure with no fragment and no `no-news` | Refused; the message names the fragment path to write, the six headings, and `no-news "<reason>"` | None: a closure must say whether operators see a change |
| AC-3 | Closure with `no-news "<reason>"` and no fragment | Prepared; the message ends with `No-news: <reason>` before `Spec-work:`; the push owes nothing for it | A closure that also carries a fragment is refused as contradictory, because it says both |
| AC-4 | A fragment whose topic is not a tag family in `website/data/topics.json` | Refused, naming the topic and the vocabulary file | A topic added to `topics.json` in the same commit is accepted, because the gate reads the prospective tree |
| AC-4a | A fragment whose `found-by` is not in the declared list | Refused, naming the value and listing the accepted ones | A fragment with no `found-by` is accepted, because most changes are not defects found by a check |
| AC-5 | A fragment missing a heading, or with an empty heading, or with Before or After reading only "none" | Refused, naming each heading at fault | "none" under Corrects, Exposure, Upstream or Asked by is accepted, because those facts are often truly absent |
| AC-6 | Closure where `<stem>.md` exists at HEAD but no commit of this spec or this script touched it | Refused as a stale fragment, naming the path | A fragment edited by any commit carrying `Spec-work: <stem>` is accepted |
| AC-7 | A non-closure commit that carries a fragment (a fix outside a spec) | The fragment is validated by AC-4 and AC-5; no closure rule applies | A non-closure commit with no fragment is prepared as today, because fixes outside a spec owe none |
| AC-8 | A message body with a hand-written `No-news:` line | Refused by the existing hand-written trailer check, naming the keyword | None |
| AC-9 | A closure prepared in a tree that is not the Ze checkout (site or wiki) | The commit is prepared through the same path validation and message contract as today, and its message carries no `No-news:` trailer | The fragment rules are skipped there, because that tree holds no specs or fragments |
| AC-10 | `./le site build` with a fragment present | Writes the changes index, the feed and every week page from `changes/posts`, byte-identical to a build without the fragment | No generated path is derived from a fragment, because fragments are unverified claims |
| AC-11 | `/ze-close` step 6 | Instructs writing the fragment with the six headings, or `no-news "<reason>"` on commit B | None |
| AC-12 | `/ze-weekly-update` Phase 1 | Lists the fragments added in the range, verifies each claim at the producer, researches only what none carries, and appends one table per update (its `release-from`..`release-to` range, whatever its length) to `plan/records/weekly-fragment-measure.md` | None |
| AC-13 | `spec-release-build` AC-25 | States that release notes are the fragments added since the previous release tag, grouped by topic | None |
| AC-14 | `website/changes/fragments/README.md` | States the file name rule, front matter, the six headings, and that a fragment is a claim; cites `newsfragment.go` as the one declaration | None |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestCreateRefusesClosureWithoutNewsFragment` | `internal/le/commit/newsfragment_test.go` | AC-2, wiring | |
| `TestCreateNoNewsWritesTrailer` | `internal/le/commit/newsfragment_test.go` | AC-3, wiring | |
| `TestNewsFragmentAndNoNewsTogetherRefused` | `internal/le/commit/newsfragment_test.go` | AC-3 survivor | |
| `TestNewsFragmentClosureAccepted` | `internal/le/commit/newsfragment_test.go` | AC-1 | |
| `TestNewsFragmentClosureFindsFragmentFromEarlierSpecCommit` | `internal/le/commit/newsfragment_test.go` | AC-1, A-1 | |
| `TestNewsFragmentCarriedEarlierInSameScript` | `internal/le/commit/newsfragment_test.go` | AC-1, R-5 | |
| `TestNewsFragmentUnknownTopicRefused` | `internal/le/commit/newsfragment_test.go` | AC-4 | |
| `TestNewsFragmentTopicAddedInSameCommitAccepted` | `internal/le/commit/newsfragment_test.go` | AC-4 survivor | |
| `TestNewsFragmentFoundByJudged` | `internal/le/commit/newsfragment_test.go` | AC-4a, each declared value accepted, an unknown one refused, absence accepted | |
| `TestNewsFragmentHeadingsJudged` | `internal/le/commit/newsfragment_test.go` | AC-5, table over each heading, empty and "none" | |
| `TestNewsFragmentStaleSameNameRefused` | `internal/le/commit/newsfragment_test.go` | AC-6, R-4 | |
| `TestNewsFragmentValidatedOnNonClosure` | `internal/le/commit/newsfragment_test.go` | AC-7 | |
| `TestNewsFragmentSkippedOutsideCheckout` | `internal/le/commit/newsfragment_test.go` | AC-9 | |
| `TestNewsFragmentUnreadableCouldNotRun` | `internal/le/commit/newsfragment_test.go` | R-6 | |
| `TestHandWrittenNoNewsTrailerRefused` | `internal/le/commit/input_test.go` | AC-8 | |
| `TestNoNewsTrailerOwesNothingAtPush` | `internal/le/commit/owed_answers_test.go` | AC-3 | |
| `TestFragmentsAreNotPublished` | `internal/le/site/changes_test.go` | AC-10, A-4 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | the feature takes no numeric input | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| N-A | | The user is an agent running `./le commit create`; the wiring tests drive `Create` in a temporary repository, which is the same entry point | |

## Files to Modify
- `internal/le/commit/prepare.go` - call `newsFragmentRefusal`; carry the `no-news` reason as a trailer
- `internal/le/commit/actions.go` - `no-news` in `createParameters` and `createPolicy`, `Options` field, `No-news:` in `declaredTrailers`
- `internal/le/site/changes.go` - export the topic vocabulary reader (or move it to a leaf package, A-5)
- `ai/skills/ze-close.md` - step 6 writes the fragment or picks `no-news`
- `ai/skills/ze-weekly-update.md` - Phase 1 reads fragments, verifies, records the measure; step 7 rewritten
- `plan/journal/README.md` - where a row's Fix cell turns `fixed: `, the fixing agent also writes `website/changes/fragments/<journal-class>-<slug>.md` when operators see the fix (no skill owns journal fixes; this page is where the marker is documented)
- `plan/release/delivery/spec-release-build.md` - AC-25 and its test row derive release notes from fragments
- `docs/contributing/committing.md` - the new refusal and the `no-news` keyword
- `docs/contributing/spec-workflow.md` - closure owes a fragment or `no-news`
- `docs/architecture/core-design.md` - declared by `internal/le/weekly/weekly.go`; unaffected, because publishing does not change (confirmed at implementation)
- `docs/features/ai-first.md` - declared by the commit tooling; updated if it lists what a closure carries
- `website/AI.md` - declared by `internal/le/site/changes.go`; its Development Update Checklist names `website/changes/fragments/` as the starting material and that the site does not publish it

## Files to Create
- `internal/le/commit/newsfragment.go` - `newsFragmentRefusal`, the heading declaration and the parser
- `internal/le/commit/newsfragment_test.go` - its tests
- `website/changes/fragments/README.md` - the format, for agents

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | tooling only |
| YANG validation constraints | N-A | tooling only |
| YANG custom validators | N-A | tooling only |
| CLI commands/flags | Yes | `internal/le/commit/actions.go` `no-news` keyword |
| CLI grammar (keyword before value) | Yes | `no-news "<reason>"`, checked by `./le cli grammar` |
| Editor autocomplete | N-A | le keywords complete from `createParameters` |
| Functional test for new RPC/API | N-A | no RPC |
| Pipe completeness | N-A | no new output |
| Env var registration | N-A | none |
| Doctor check for runtime dependencies | N-A | no runtime dependency |
| Prometheus counters/metrics | N-A | none |
| BGP family surface (new SAFI / capability / attribute) | N-A | not BGP |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | N-A | tooling |
| 2 | Config syntax changed? | N-A | |
| 3 | CLI command added/changed? | N-A | le keyword, documented in `docs/contributing/committing.md` |
| 4 | API/RPC added/changed? | N-A | |
| 5 | Plugin added/changed? | N-A | |
| 6 | Has a user guide page? | N-A | |
| 7 | Wire format changed? | N-A | |
| 8 | Plugin SDK/protocol changed? | N-A | |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | |
| 10 | Test infrastructure changed? | N-A | |
| 11 | Affects daemon comparison? | N-A | |
| 12 | Internal architecture changed? | Yes | `docs/contributing/committing.md`, `docs/contributing/spec-workflow.md`, `website/changes/fragments/README.md` |
| 13 | Route metadata keys added/changed? | N-A | |
| 14 | Prometheus counters added/changed? | N-A | |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | N-A | |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | derived by `./le spec citation anchors spec plan/post-release/engineering/spec-weekly-news-fragments.md` at implementation |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/contributing/committing.md` keyword examples |

## Implementation Steps

1. **Phase: Wiring** -- `no-news` keyword and trailer declaration; `newsFragmentRefusal` stub called from `Create` after `specMechanismsRefusal`
   - Tests: `TestCreateRefusesClosureWithoutNewsFragment`, `TestCreateNoNewsWritesTrailer`
   - Files: `actions.go`, `prepare.go`, `newsfragment.go`
   - Verify: both fail against the stub for the expected reason
2. **Phase: Fragment parsing and validation** -- front matter, the six headings, topic through the exported vocabulary reader
   - Tests: `TestNewsFragmentUnknownTopicRefused`, `TestNewsFragmentTopicAddedInSameCommitAccepted`, `TestNewsFragmentHeadingsJudged`, `TestNewsFragmentValidatedOnNonClosure`, `TestNewsFragmentSkippedOutsideCheckout`, `TestNewsFragmentUnreadableCouldNotRun`
   - Files: `newsfragment.go`, `changes.go`
3. **Phase: Closure rule** -- fragment in the prospective tree, in the spec's population or this script; contradiction with `no-news`
   - Tests: `TestNewsFragmentClosureAccepted`, `TestNewsFragmentClosureFindsFragmentFromEarlierSpecCommit`, `TestNewsFragmentCarriedEarlierInSameScript`, `TestNewsFragmentStaleSameNameRefused`, `TestNewsFragmentAndNoNewsTogetherRefused`, `TestHandWrittenNoNewsTrailerRefused`, `TestNoNewsTrailerOwesNothingAtPush`
4. **Phase: Site preservation** -- `TestFragmentsAreNotPublished`; one `./le site build` and `./le site check` with a fragment present
5. **Phase: Skills and docs** -- `ze-close` step 6, the fix guidance, `ze-weekly-update` Phase 1 and the measure file rule, `website/changes/fragments/README.md`, `committing.md`, `spec-workflow.md`; then `./le ai sync update`
6. **Phase: Release notes** -- restate `spec-release-build` AC-25 and its test row

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Correctness | A fragment's presence is never read as proof of closure; closure is still `closedSpecStem` |
| Correctness | The gate reads the prospective tree, never the working tree, so a fragment left uncommitted does not satisfy it |
| Naming | Keyword `no-news`, trailer `No-news:`, directory `website/changes/fragments/` used identically in code, skills and docs |
| Data flow | One topic reader serves the site and the gate; one heading declaration serves the gate and the README |
| Rule: user-facing errors | Every refusal names what is wrong, why, and the exact file or keyword that fixes it |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Fragment gate | `./le job run label unit-pkg command go test ./internal/le/commit/` passes, with the AC tests listed |
| Topic reader shared | `grep -rn 'topics.json' internal/le` names one reader |
| Skills updated | `grep -n 'fragments' ai/skills/ze-close.md ai/skills/ze-weekly-update.md` |
| Site unchanged | `./le site build`, `./le site check` with a fragment present |
| Release spec restated | `grep -n 'fragment' plan/release/delivery/spec-release-build.md` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The fragment path is fixed by the stem; front matter values are never used as paths |
| Fail closed | An unreadable fragment or failed git read is `couldNotRun`, never a pass |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| The tooling-event log is a derived report in its own spec | One spec with a new append-only log | Every fact declared once; a shared log collides between sessions |
| Every spec closure owes a fragment or `no-news` | Lexical product-path rule; `go list -deps` over shipped flavours | No cheap accurate classifier exists; one keyword on tooling closures costs less than a wrong rule |
| Fragment files under `website/changes/fragments/`, kept | A `## News` section in the spec, read from history | Fixes outside a spec need a home; release notes read files rather than history |
| Date from git, not front matter | A `date:` field | Every fact declared once |
| Judge at commit B over the spec's population | Judge commit A | Commit A cannot be told from an ordinary commit |
| Fixes outside a spec are instruction only | Refuse a journal row turning fixed without a fragment | Thomas's choice; the measure checks whether it leaks |
| Release notes from fragments | Commit subjects | Fragments carry the operator facts; subjects would be a second, weaker account |

## Known Limitations
- The tooling-event report is `spec-weekly-event-report`.
- Whether fragment text is specific cannot be checked by the gate; the two-week measure judges it.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout: `./le job run label unit-pkg command go test ./internal/le/commit/ ./internal/le/site/`, `./le cli grammar`, and `./le site build` then `./le site check` (no `./le test unit` group covers `internal/le`; `./le test functional runner` is unaffected)
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior (N-A: tooling, wiring tests drive `Create`)
- [ ] Interop tests for protocol features (N-A: no protocol)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] **Commit A:** code + tests + docs + edited spec + this spec's own fragment or none
- [ ] **Commit B:** `remove <the spec's path in its bucket>` with `no-news "tooling: no operator-visible change"` unless a fragment was written
