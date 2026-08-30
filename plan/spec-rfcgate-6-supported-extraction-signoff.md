# Spec: rfcgate-6-supported-extraction-signoff

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | protocol |
| Depends | `plan/deferrals/rfcgate-0-umbrella.md` (row D4, "the drain itself"); `plan/spec-followup-rfc-enrollment.md` (owns `rfc/enrolled.txt` and the coverage rollup) |
| Phase | - |
| Deferral shard | `plan/deferrals/rfcgate-6-supported-extraction-signoff.md` |
| Handoff | verify |
| Updated | 2026-08-30 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`./le rfc check` verifies that every requirement **listed** in `rfc/short/<stem>.md`
has a test. It never verifies the checklist is **complete**. An obligation nobody wrote
down is owed no test, so the gate is green for it and stays green forever.
`rfc/extraction/<stem>.json` is the artifact that closes this: a recorded walk of the
RFC's own text where every requirement-stating site is either mapped to a requirement
id or excluded with a reason from a closed set.

Almost nothing carries that artifact. Measured 2026-08-30, all figures derived from the
tree rather than retyped from a previous run:

| Measurement | Value | Derived from |
|---|---|---|
| Enrolled RFC stems | 171 | data rows of `rfc/enrolled.txt` |
| Stems with a valid sign-off | 6 | `./le rfc extraction-status`, field `signed` |
| Sign-off files on disk | 7 | `rfc/extraction/*.json`; `rfc1035` is signed but not enrolled, so it earns no drain credit |
| Backlog | 165 | `./le rfc extraction-status`, field `backlog` |
| Drain rate forcing a sign-off | 0 per month since 2026-07-29 | `rfc/drain-budget.txt`, owner decision D5 |

3.5% of the enrolled corpus has a bounded checklist, and `check_drain_floor` is inert by
owner decision, so nothing pushes that number. The gate's coverage claim is therefore
bounded by what somebody happened to write down, for 165 of 171 RFCs.

**Why this spec cuts at `Supported`.** `docs/features/rfc-status.md` is Ze's public
standards claim. Its own vocabulary paragraph says `Partial` means a named subset is
missing, `Experimental` means implemented but not yet proven in deployment,
`Unsupported` and `Future` mean it is not there. Every one of those already tells a
reader the RFC is not fully met, so an obligation missing from such a row's checklist is
a disclosure that is incomplete rather than a promise that is false. `Supported` is the
only status that promises the RFC **is** met. An unextracted MUST behind a `Supported`
row is a false public claim, made in Ze's own words, on the page a prospective operator
reads. That makes this set, and not the whole backlog, the release-critical one.

**The evidence that a walk finds real defects.** On 2026-08-30 five defects were found
and fixed whose broken requirement was **not on its RFC's checklist**:

| RFC | Section | Defect |
|---|---|---|
| RFC 8907 | 10.5.2 | a TACACS+ reply carrying `TAC_PLUS_UNENCRYPTED_FLAG` authenticated a user with no proof of the shared secret |
| RFC 7427 | 3 and 4 | the signature algorithm was not chosen from the peer's advertised list, and method 14 was used without the permitting notify |
| RFC 7854 | 4.5 | the monitoring station never closed the TCP session after a termination message |
| RFC 8955 | 4.2 | duplicate component types made every such NLRI malformed; GoBGP 3.31 reset the session |
| RFC 4724 | 4 | the End-of-RIB marker was sent up to 2s after the initial update completed |

Every one of the five sits in an RFC with **no** extraction sign-off. None of the seven
signed-off RFCs produced a defect that day. That correlation is the premise this spec
rests on: the artifact is not bookkeeping, it is the only mechanism in the repository
that can surface an obligation nobody extracted.

**The correlation is between signed and unsigned, not between `Supported` and
`Partial`.** None of those five RFCs is in this spec's scope: RFC 8907, RFC 7854, RFC
8955 and RFC 4724 are `Partial` on the ledger and RFC 7427 has no ledger row at all. The
premise is that a walk finds what a checklist missed, and it holds wherever a checklist
is unbounded. The scope cut is about which unbounded checklist costs Ze most when it is
wrong, and that is the one sitting under a public promise.

## Required Reading

### Architecture Docs
- [ ] `rfc/extraction/README.md` - the artifact contract, the three registers, the three
      denominators, the exclusion kinds, and the ratchets
  → Constraint: only dispositions, reasons and the two relocation fields are authored.
    `sites`, `sections`, `quote`, `register`, `source-path`, `source-sha` and every
    published count are DERIVED at check time. Editing a derived field to turn a red
    green fails the check naming the field and the locator
  → Constraint: the writer emits `"disposition": null` for every site and section, and
    an unclassified site FAILS the check. There is no `--sign-off` mode, no default
    disposition and no bulk classifier, so generating skeletons en masse makes the gate
    REDDER
  → Decision: quote the SITE figure (`_sites_for` / `sitesFor`) for sign-off arithmetic,
    the raw occurrence figure only for keyword presence, and the obligation figure only
    for what `check_new_summaries` decided. They are three denominators and all three are
    correct; do not reconcile them
  → Constraint: `relocated-to-spec` is the one exclusion kind that does not dismiss its
    sentence. It requires `relocated-to` naming an existing `plan/spec-<name>.md` and
    `reserved-id` naming an id of THIS RFC that the spec still holds and the summary no
    longer declares
- [ ] `ai/rules/rfc-compliance.md` - "Extraction Completeness" and "Implement Full
      Compliance. Ask Thomas Only Before Doing LESS"
  → Constraint: a found MUST that Ze does not meet MUST NOT be classified `{gap}`,
    `{not-applicable}` or `partial` without asking Thomas. The ask is owed the same
    session the walk finds it, quoting the requirement id and the RFC sentence, naming
    the producing function, and asking WHICH WAY to fix it
  → Constraint: a FIRST sign-off is reviewed, not ratcheted. `check_extraction_ratchet`
    compares a stem against its own HEAD row, so a stem signing off for the first time
    has no baseline and could exclude every site. The published per-RFC exclusion ratio
    is the only control; read it before approving one
- [ ] `ai/rules/evidence.md` - guards, producers, and derived-versus-claimed
  → Constraint: before stating what a check does, read the function that produces the
    verdict. `checkUnprovenSupport` in `internal/le/rfc/check_status.go` is the producer
    for every claim this spec makes about what the ledger gate can and cannot see
- [ ] `ai/rules/planning.md` - phase boundaries and the deferral shard contract
  → Constraint: an agent whose package turns out too big reports the size to the main
    thread, which re-cuts. It MUST NOT trim an AC or weaken a test to fit. With 49 stems
    this is the expected failure mode, so the package boundary is the TIER, not the spec

### RFC Summaries (Scope: protocol)
- [ ] `rfc/short/rfc2545.md`, `rfc/extraction/rfc2545.json` - a `prose`-register sign-off
      whose sections 2 and 4 bind a network administrator and an operator
  → Constraint: `binds-another-role` is the kind for an obligation binding an operator, a
    CA, a registry or the peer. Its reason must name the role, not merely assert it
- [ ] `rfc/short/rfc4486.md`, `rfc/extraction/rfc4486.json` - the smallest `rfc2119`
      sign-off in the tree, one gated MUST across ten advisory statements
  → Decision: read this one first as the worked shape of a small walk before starting
    RFC 3748, which carries 115 uppercase keyword occurrences
- [ ] `rfc/short/rfc7296.md`, `rfc/extraction/rfc7296.json` - the largest sign-off, and
      the only one carrying `relocated-to-spec` sites
  → Constraint: twelve sites relocated by owner ruling D-1 (2026-07-31) to
    `plan/spec-ipsec-remote-access.md` and `plan/spec-ipsec-ipcomp.md`. A relocation
    counts in `Extraction.excluded`, so it costs the same resign-reason as any other
    re-classification and is published apart

**Key insights:** (minimal context to resume after compaction)
- The deliverable per stem is a VALID sign-off the check accepts, never a skeleton.
  `./le rfc extraction-create STEM=<stem>` writes only unclassified dispositions.
- 49 stems are in scope. 39 have a summary and a source and need a walk. 10 have neither
  and need enrolment first.
- A walk that finds an unmet MUST produces an ASK to Thomas, not a `{gap}`.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/le/rfc/check_status.go` - `checkStatusCompleteness` fires only for a stem
      enrolled SINCE HEAD, and for a row deleted while its stem stays enrolled.
      `statusIsSupportClaim` treats every status except `Unsupported` and `Future` as a
      support claim. `checkUnprovenSupport` iterates `stems`, the set of `rfc/short/`
      summary stems, so a ledger row naming an RFC with no summary at all never enters
      the loop; its own error text says so
- [ ] `internal/le/rfc/check_extraction.go` - `checkExtractionRatchet` and
      `checkExtractionRatchetAgainst` compare a stem against its own HEAD row;
      `checkDrainFloor` compares the derived signed count against `parseDrainBudget`
- [ ] `internal/le/rfc/signoff.go` - `evaluateExtraction` and `evaluateExtractions`
      produce the per-stem verdict; `relocationErrors` re-reads a relocation claim on
      every run; `credited` and `registerCounts` produce the published split
- [ ] `internal/le/rfc/inventory.go` - `sitesFor` derives the sites a reviewer classifies,
      `DeriveRegister` grades the source, `Deriver.Inventory` is the per-stem entry
- [ ] `internal/le/rfc/extraction_create.go` - `extractionCreateReport` reports
      `UnclassifiedSites` and `UnclassifiedSections`; `extractionDocument` is the schema
- [ ] `internal/le/rfc/check.go` - `Answer` assembles the check list, calling
      `checkStatusCompleteness`, `checkUnprovenSupport` and `checkGapCountAgreement` in
      one block and `checkExtractionRatchet` further down
- [ ] `rfc/extraction/README.md` - the authored-versus-derived contract
- [ ] `rfc/enrolled.txt` - 171 data rows, tab-separated stem and reason
- [ ] `rfc/not-enrolled.txt` - 9 data rows; it PARTITIONS `rfc/short/*.md` with
      `rfc/enrolled.txt`
- [ ] `docs/features/rfc-status.md` - eight RFC tables sharing one header, plus a ninth
      table of drafts and non-RFC standards that is out of scope here
- [ ] `rfc/drain-budget.txt` - `start 2026-07-29`, `rate 0`

**Behavior to preserve:**
- The authored/derived split. No field this spec writes may be one the check derives.
- The partition invariant: every `rfc/short/*.md` stem is in exactly one of
  `rfc/enrolled.txt` and `rfc/not-enrolled.txt`.
- Every requirement id already declared in a summary. `check_retired_requirements`
  refuses a vanishing id, and a walk that renumbers rather than appends trips it.
- `rate 0` in `rfc/drain-budget.txt`. Arming it is Thomas's one-line decision and is out
  of scope here.
- Existing sign-offs for `rfc1035`, `rfc2545`, `rfc3765`, `rfc4486`, `rfc5301`,
  `rfc7296`, `rfc7999`. Their exclusion counts may not rise without a `resign-reason`.

**Behavior to change:**
- 49 stems gain a valid extraction sign-off.
- The 10 stems in Class B gain a summary, an enrolment row and a public-ledger row that
  agrees with the gate.
- One new check refuses a `Supported` ledger row whose stem has no valid sign-off.

## The scope, derived

Derived 2026-08-30 by reading the Status column of the eight RFC tables in
`docs/features/rfc-status.md` (lines 27-276; the drafts table at line 277 is not RFCs and
is out of scope) and testing each stem for `rfc/extraction/<stem>.json`,
`rfc/short/<stem>.md`, `rfc/full/<stem>.txt` and a row in `rfc/enrolled.txt`.

| Bucket | Count |
|---|---|
| Rows whose Status is exactly `Supported` | 39 |
| Rows whose Status begins `Supported ` and names a scope | 13 |
| Rows whose Status cell reads `Yes` (RFC 1997) | 1 |
| **Total rows claiming support** | **53** |
| Already signed off (`rfc2545`, `rfc3765`, `rfc4486`, `rfc5301`) | 4 |
| **In scope** | **49** |

**A scope-qualified `Supported` is in scope.** "Supported on Linux", "Supported for
subscriber access" and "Supported within BMP sender scope" each promise the RFC is met
within a named scope. That is a positive claim about conformance, and an unextracted MUST
inside the named scope falsifies it exactly as it falsifies a bare `Supported`. Excluding
them would make the word after `Supported` an escape hatch.

**RFC 1997's Status cell reads `Yes`, which is not in the page's own vocabulary.** The
paragraph at the top of the page defines `Supported`, `Experimental`, `Partial`,
`Unsupported` and `Future`. `Yes` is none of them. It reads as a support claim and
`statusIsSupportClaim` treats it as one, so the row is in scope; correcting the cell to
`Supported` is part of this spec's ledger work.

### Class A: enrolled, summary and source present, no sign-off (39)

These need a walk and nothing else.

| Tier | Stems |
|---|---|
| 1 - authentication and cryptography | `rfc2865`, `rfc2866`, `rfc2869`, `rfc5176`, `rfc2759`, `rfc3748`, `rfc4301`, `rfc4303`, `rfc3948` |
| 2 - session establishment, identity, liveness | `rfc6286`, `rfc5492`, `rfc9234`, `rfc8203`, `rfc9003`, `rfc7947` |
| 3 - monitoring, export, validation feeds | `rfc8671`, `rfc9069`, `rfc6396`, `rfc6811`, `rfc9582` |
| 4 - BGP wire core, families, attributes | `rfc4760`, `rfc2918`, `rfc7313`, `rfc7911`, `rfc8654`, `rfc8950`, `rfc5549`, `rfc1997`, `rfc4360`, `rfc8092`, `rfc4456`, `rfc4364`, `rfc3032`, `rfc4761` |
| 5 - provisioning, DNS, file transfer | `rfc7534`, `rfc7535`, `rfc4578`, `rfc1350`, `rfc2347` |

### Class B: no summary, no source text, no disposition (10)

| Stem | Ledger area | Ledger status | Tier |
|---|---|---|---|
| `rfc4302` | Authentication Header | Supported in OSPFv3 manual IPsec path | 1 |
| `rfc5282` | IKEv2 AEAD algorithms | Supported | 1 |
| `rfc2385` | TCP MD5 | Supported on supported kernels | 2 |
| `rfc5082` | GTSM and TTL security | Supported on Linux | 2 |
| `rfc9687` | Send Hold Timer | Supported | 2 |
| `rfc9384` | BFD-triggered BGP Cease | Supported within BFD | 2 |
| `rfc5798` | VRRPv3 | Supported | 2 |
| `rfc8516` | FQDN capability | Supported | 4 |
| `rfc7607` | Prefix-limit enforcement context | Supported | 4 |
| `rfc4762` | VPLS architecture | Supported at feature scope | 4 |

Each of these ten has no `rfc/short/<stem>.md`, no `rfc/full/<stem>.txt`, and no row in
either `rfc/enrolled.txt` or `rfc/not-enrolled.txt`, while `docs/features/rfc-status.md`
claims Ze supports it. No check in `internal/le/rfc` can see them: `checkUnprovenSupport`
iterates summary stems and says in its own error text that "Rows naming an RFC with no
summary at all are outside this check", and `checkStatusCompleteness` only judges stems
that are enrolled. Ten public support claims sit outside the whole RFC gate.

Their deliverable is larger than a walk: fetch the source into `rfc/full/`, write the
summary with `/ze-rfc`, enrol the stem, then sign off. Enrolling gates the RFC's MUSTs,
so each of the ten can turn the gate red until its requirements carry tests. That is the
correct outcome and not a reason to stop, but it is the reason Class B is sequenced after
Class A within each tier and budgeted separately.

## Sequencing, argued from the evidence

The order is by what a missed obligation costs, corroborated by a derived measure of how
thinly each summary already covers its source.

**The cost argument.** The five defects of 2026-08-30 cluster: an authentication bypass
(RFC 8907), a downgrade of signature-algorithm negotiation (RFC 7427), a monitoring
session left open (RFC 7854), a wire-validation miss that reset a peer (RFC 8955), and a
timing violation (RFC 4724). Three of the five sit on authentication or monitoring
surfaces. A missed MUST on an authentication surface admits an unauthenticated party. A
missed MUST on a session surface drops a peering. A missed MUST on a monitoring surface
gives an operator a wrong picture of a correct network. A missed MUST on a file-transfer
surface breaks a boot image download and announces itself. That ordering is the tier
numbering.

**The corroborating measurement.** For each Class A stem, the count of distinct
requirement ids declared in `rfc/short/<stem>.md` divided by the count of uppercase
`MUST`, `SHALL` and `REQUIRED` occurrences in `rfc/full/<stem>.txt`. A low ratio means
the summary declares few obligations against a source that states many, which is where an
unextracted obligation is most likely.

| Stem | ids | uppercase keyword occurrences | ratio | Tier |
|---|---|---|---|---|
| `rfc5176` | 5 | 86 | 0.06 | 1 |
| `rfc2869` | 8 | 53 | 0.15 | 1 |
| `rfc2865` | 13 | 81 | 0.16 | 1 |
| `rfc4301` | 18 | 98 | 0.18 | 1 |
| `rfc9069` | 7 | 35 | 0.20 | 3 |
| `rfc3748` | 26 | 115 | 0.23 | 1 |
| `rfc2866` | 8 | 29 | 0.28 | 1 |
| `rfc8671` | 5 | 16 | 0.31 | 3 |
| `rfc4303` | 21 | 66 | 0.32 | 1 |
| `rfc3948` | 12 | 22 | 0.55 | 1 |
| `rfc4760` | 17 | 14 | 1.21 | 4 |
| `rfc4761` | 27 | 24 | 1.13 | 4 |
| `rfc3032` | 23 | 20 | 1.15 | 4 |
| `rfc1997` | 9 | 3 | 3.00 | 4 |

The ten lowest ratios in Class A are eight Tier 1 stems and both Tier 3 BMP stems. Every
Tier 4 BGP-core stem sits at or above 0.68. The independent measure agrees with the cost
argument, so the tiers are ordered 1, 2, 3, 4, 5.

**What this ratio is not.** The denominator is the raw occurrence count that
`rfc/extraction/README.md` names `source_keyword_count`, and the README is explicit that
it is not the sign-off arithmetic: `sitesFor` counts normative sentences with boilerplate
excluded, and it is the number a reviewer classifies. The raw count also charges each
document up to four keywords for its own "Key words" paragraph and its reference entries.
This ratio is a RANKING heuristic over keyword presence and nothing else. It is safe for
ranking here because the Tier 1 sources carry 22 to 115 occurrences, where a four-keyword
boilerplate charge cannot move a stem between tiers. It must never be quoted as coverage.

**Seven Class A stems have zero uppercase MUST-level keywords** (`rfc6286`, `rfc2918`,
`rfc7534`, `rfc7535`, `rfc1350`, `rfc2347`, and any Class B stem after its source lands).
They will derive `prose` or `manual-walk`, and a `manual-walk` sign-off over a
`Supported` row needs a `register-reason` stating why zero MUSTs is a property of the
text, or `checkUnprovenSupport` refuses it.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- `rfc/full/<stem>.txt`, the RFC's own text, read at check time. Nothing about a sign-off
  is trusted from the artifact except the dispositions and reasons a human authored.
- `docs/features/rfc-status.md`, the public claim, parsed into `LedgerRow` values.
- `rfc/extraction/<stem>.json`, the authored half: dispositions, reasons, `relocated-to`,
  `reserved-id`, `signed-off`, `reviewer`, `register`, `register-reason`, `resign-reason`.

### Transformation Path
1. `Deriver.Inventory` (`internal/le/rfc/inventory.go`) splits the source into sites via
   `sitesFor` and grades the register via `DeriveRegister`.
2. `evaluateExtraction` (`internal/le/rfc/signoff.go`) matches each derived site against
   the artifact's authored disposition, runs the forward arithmetic (every site mapped or
   excluded) and the reverse arithmetic (every gated requirement targeted or declared in
   `unsourced-ids`), and re-reads every relocation claim through `relocationErrors`.
3. `Answer` (`internal/le/rfc/check.go`) folds that verdict together with the status
   checks and `checkExtractionRatchet`.
4. `./le rfc index-update` regenerates `ai/RFC-REQUIREMENTS.md` and
   `rfc/requirements/<stem>.md`, publishing the per-RFC exclusion ratio and the relocated
   subset.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| RFC source text ↔ sign-off artifact | derived sites compared against authored dispositions by `evaluateExtraction`; `source-sha` pins the text | No |
| Sign-off artifact ↔ public ledger | `checkUnprovenSupport` and the new `checkSupportedSignoff` join a `LedgerRow` to a stem's summary and sign-off | No |
| Working tree ↔ HEAD | `checkExtractionRatchetAgainst` reads a HEAD blob so a sign-off cannot be deleted or its exclusions inflated | No |
| Summary ↔ spec | `relocated-to-spec` sites tie a summary to a live `plan/spec-*.md` and are re-read every run | No |

### Integration Points
- `internal/le/rfc/check_status.go` - the new check lands beside `checkUnprovenSupport`,
  which already owns the ledger-row-to-summary join.
- `internal/le/rfc/check.go` `Answer` - the new check is appended to the same violation
  list as the existing status checks.
- `rfc/enrolled.txt` and `rfc/not-enrolled.txt` - Class B stems enter the partition.
- `docs/features/rfc-status.md` - Class B rows gain coverage and remaining prose that
  agrees with their new summaries; the RFC 1997 Status cell is corrected.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The in-scope set is 49 stems, from 53 support-claiming rows less 4 signed | derived 2026-08-30 from the Status column of `docs/features/rfc-status.md` lines 27-276 and `rfc/extraction/*.json` | the tier tables and every count-bearing AC shift | re-derive the two lists at the start of phase 1 and record any delta in this section | unvalidated |
| A-2 | The commissioning brief's figures (213 enrolled, 46 Supported, 2 signed, 44 in scope) count differently from this spec's | `rfc/enrolled.txt` has 213 LINES and 171 data rows; `./le rfc extraction-status` reports `signed: 6`; four support-claiming rows carry sign-offs, not two | if the brief's boundary was intended, 5 stems leave scope | ask Thomas which boundary he meant before phase 1 closes; do not silently pick | unvalidated |
| A-3 | Every Class B stem's RFC text is retrievable from the RFC Editor | the other 184 sources in `rfc/full/` were retrieved the same way | a stem cannot be enrolled and its ledger row is a claim nothing can check | fetch all ten in phase 0 before any walk starts | unvalidated |
| A-4 | A walk of the 39 Class A stems finds at least one unextracted MUST Ze does not meet | five such defects were found on 2026-08-30, all in unsigned RFCs | the ask-Thomas path is never exercised and the spec is pure bookkeeping | the phase report records found-and-unmet obligations per tier, zero included | unvalidated |
| A-5 | Adding `checkSupportedSignoff` after the 49 land leaves the gate green | the check's population is exactly the 49 plus the 4 already signed | the gate reds on landing and blocks unrelated commits | land the check in the final phase only, and run `./le verify worktree` on the commit that adds it | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A session generates skeletons in bulk to show progress; every one carries unclassified sites and the gate goes redder | `./le rfc check` violation count rises while the signed count does not | the deliverable is defined as a sign-off the check ACCEPTS. A skeleton with an unclassified site counts as zero in every AC |
| R-2 | A walk finds an unmet MUST and the session classifies it `{gap}` to keep moving | a new `{gap}` annotation appears in a summary touched by this spec | `ai/rules/rfc-compliance.md` forbids it without asking Thomas. Every found-and-unmet obligation is an ASK the same session, quoting the id, the RFC sentence and the producing function |
| R-3 | Enrolling a Class B stem gates MUSTs with no tests and reds the gate | `./le rfc check` reds on `check_coverage_ratchet` right after an enrolment commit | enrol one Class B stem per commit, with its tests, never in a batch. A stem whose tests are not ready stays out of `rfc/enrolled.txt` and its ledger row is corrected downward until they are |
| R-4 | A first sign-off excludes almost everything and no ratchet notices, because a first sign-off has no HEAD baseline | the published per-RFC exclusion ratio in `ai/RFC-REQUIREMENTS.md` is high for a new stem | the review of each tier reads the exclusion ratio for every stem in it, and an outlier is re-walked rather than approved |
| R-5 | 49 stems is too large for one implementation agent, and the package gets trimmed to fit | an agent reports partial tier coverage | the package boundary is the TIER. An agent whose tier is too big reports the size to the main thread, which re-cuts by stem (`ai/rules/planning.md`) |
| R-6 | A walk renumbers requirement ids rather than appending, and `check_retired_requirements` reds | a violation naming a vanished id | new obligations get NEW ids appended; an existing id's TEXT may be corrected under the same id, and nothing else |
| R-7 | A Class B ledger row turns out to overstate what Ze does, and the honest fix is to lower the Status | the summary's MUSTs cannot all be tagged | lowering a public claim is a compliance decision: ask Thomas which way, never assume (`ai/rules/rfc-compliance.md`) |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing at runtime: the artifacts are gate inputs, not product code. A wrong sign-off is worse than none, because it publishes a bound that does not hold. The one runtime-visible half is any defect a walk finds and fixes, which carries its own spec and its own blast radius |
| How is it reverted? | Per stem, by one commit. `checkExtractionRatchet` refuses removing a sign-off, so a revert of a landed stem needs the owner. The new check is one commit and revertible on its own |
| Who else touches this path? | `plan/spec-followup-rfc-enrollment.md` owns `rfc/enrolled.txt` and the coverage rollup and is the destination of both live rows in `plan/deferrals/rfcgate-0-umbrella.md`. Any concurrent `/ze-rfc` run edits `rfc/short/`. `plan/journal/concurrent-rfc-gate-stale.md` records the shared-checkout failure mode |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le rfc check` over a tree where a `Supported` ledger row's stem has no sign-off | → | `checkSupportedSignoff` (`internal/le/rfc/check_status.go`) | `TestCheckSupportedSignoffRefusesUnsignedSupportedRow` |
| `./le rfc check` over a tree where every support-claiming row's stem carries a valid sign-off | → | `checkSupportedSignoff` via `Answer` (`internal/le/rfc/check.go`) | `TestCheckSupportedSignoffPassesWhenEverySupportedRowIsSigned` |
| `./le rfc check` over a tree where a `Supported` row names an RFC with no summary at all | → | `checkSupportedSignoff` (`internal/le/rfc/check_status.go`) | `TestCheckSupportedSignoffRefusesSupportedRowWithNoSummary` |
| `./le rfc extraction-status` after a tier lands | → | `credited`, `registerCounts` (`internal/le/rfc/signoff.go`) | `TestExtractionStatusCountsTierSignoffs` |
| A landed sign-off whose file is later deleted | → | `checkExtractionRatchetAgainst` (`internal/le/rfc/check_extraction.go`) | existing `internal/le/rfc/extraction_test.go` ratchet cases, extended with an in-scope stem |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `./le rfc extraction-status` is run after the last phase | the `signed` field is at least 55: the 6 credited today plus the 49 in scope. `rfc1035` stays uncredited because it is not enrolled |
| AC-2 | `./le rfc check` is run over the closing tree | it reports no violation naming any of the 49 in-scope stems, and no violation naming extraction sign-off, enrolment, or public status for them |
| AC-3 | Each of the 39 Class A stems is inspected | `rfc/extraction/<stem>.json` exists, every entry in `sites[]` carries a `disposition` of `mapped` or `excluded` and no `null`, every entry in `sections[]` carries `walked` or `skipped`, and `signed-off`, `reviewer` and (for `manual-walk`) `register-reason` are present |
| AC-4 | Each of the 10 Class B stems is inspected | `rfc/full/<stem>.txt`, `rfc/short/<stem>.md` and `rfc/extraction/<stem>.json` all exist, the stem appears in exactly one of `rfc/enrolled.txt` and `rfc/not-enrolled.txt`, and its `docs/features/rfc-status.md` row states a coverage note tied to source anchors |
| AC-5 | The RFC 1997 row is read on the public page | its Status cell reads `Supported`, one of the five statuses the page's own vocabulary paragraph defines |
| AC-6 | A test tree carries a `Supported` ledger row whose stem has no valid sign-off | `./le rfc check` reports a violation naming the stem, the row's Status, and the missing artifact path |
| AC-7 | A test tree carries a `Supported` ledger row naming an RFC with no `rfc/short/` summary | `./le rfc check` reports a violation naming the stem, closing the hole `checkUnprovenSupport` discloses in its own error text |
| AC-8 | A walk finds an obligation Ze does not meet | the phase report names the requirement id, quotes the RFC sentence, names the producing function, and records the question put to Thomas. No `{gap}`, `{not-applicable}` or `partial` annotation is written for it without his answer |
| AC-9 | `rfc/drain-budget.txt` is read at closure | it still reads `rate 0`. This spec does not arm the quota |
| AC-10 | `ai/RFC-REQUIREMENTS.md` is read at closure | it is regenerated, and its per-RFC exclusion ratio is present for all 49 new sign-offs |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | An operator reads `docs/features/rfc-status.md` and sees `Supported` for RFC 2865 | ledger row → `rfc/short/rfc2865.md` checklist → `rfc/extraction/rfc2865.json` bounding that checklist against `rfc/full/rfc2865.txt` | `TestCheckSupportedSignoffPassesWhenEverySupportedRowIsSigned` plus `./le rfc check` green |
| 2 | A contributor adds a new `Supported` row for an RFC with no sign-off | `./le rfc check` → `checkSupportedSignoff` → violation naming the stem | `TestCheckSupportedSignoffRefusesUnsignedSupportedRow` |
| 3 | A contributor deletes a landed sign-off to make a red go green | `./le rfc check` → `checkExtractionRatchetAgainst` reads the HEAD blob → violation | existing ratchet cases in `internal/le/rfc/extraction_test.go` |
| 4 | A reviewer asks how much of RFC 5176 the checklist actually bounds | `./le rfc index-update` → `ai/RFC-REQUIREMENTS.md` per-RFC exclusion ratio | `TestExtractionStatusCountsTierSignoffs` and the regenerated ledger |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestCheckSupportedSignoffRefusesUnsignedSupportedRow` | `internal/le/rfc/check_test.go` | a support-claiming row whose stem has no artifact is refused, and the message names the stem, the Status and the artifact path | |
| `TestCheckSupportedSignoffRefusesSupportedRowWithNoSummary` | `internal/le/rfc/check_test.go` | the Class B hole: a row naming an RFC with no summary is refused rather than skipped | |
| `TestCheckSupportedSignoffPassesWhenEverySupportedRowIsSigned` | `internal/le/rfc/check_test.go` | the check is green on the intended end state, so it can be shown to have been red for a reason | |
| `TestCheckSupportedSignoffIgnoresUnsupportedAndFuture` | `internal/le/rfc/check_test.go` | `Unsupported` and `Future` rows are outside the population, matching `statusIsSupportClaim`'s two exclusions | |
| `TestCheckSupportedSignoffRefusesSkeletonArtifact` | `internal/le/rfc/check_test.go` | an artifact present but carrying a `null` disposition does not satisfy the check; this is the R-1 guard in test form | |
| `TestExtractionStatusCountsTierSignoffs` | `internal/le/rfc/extraction_test.go` | `credited` counts an enrolled signed stem and does not count an unenrolled one, pinning the 6-versus-7 discrepancy | |
| `TestSupportedRowsHaveDerivableScope` | `internal/le/rfc/check_test.go` | the scope derivation is reproducible: parsing the eight RFC tables yields the support-claiming stem set, so A-1 can be re-checked mechanically | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `schema-version` in a sign-off | 1 | 1 | 0 | 2 |
| unclassified sites tolerated in an accepted sign-off | 0 | 0 | N/A | 1 |
| `rate` in `rfc/drain-budget.txt` (unchanged by this spec) | 0 and above | 0 | -1 | N/A |
| exclusion count rise without a `resign-reason` | 0 | 0 | N/A | 1 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `rfc-supported-signoff-refuses-unsigned` | `test/plugin/rfc-supported-signoff-refuses-unsigned.ci` | a contributor runs the repository gate over a tree with an unsigned `Supported` row and sees the refusal text, exit non-zero | |
| `rfc-supported-signoff-accepts-signed` | `test/plugin/rfc-supported-signoff-accepts-signed.ci` | the same gate over the intended end state exits zero and prints no extraction violation | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| N-A: this spec changes no wire behavior | - | - | The artifacts and the check are repository tooling. Any RFC defect a walk FINDS is fixed under its own spec, and that spec owes the interop scenario for the behavior it changes (`ai/rules/interop-and-goal-validation.md`) | |

## Files to Modify
- `internal/le/rfc/check_status.go` - add `checkSupportedSignoff` beside
  `checkUnprovenSupport`, reusing `statusIsSupportClaim` and the `LedgerRow` map
- `internal/le/rfc/check.go` - call it from `Answer` alongside the other status checks
- `internal/le/rfc/check_test.go` - the seven unit cases above
- `internal/le/rfc/extraction_test.go` - extend the ratchet cases with an in-scope stem
- `rfc/short/rfc1997.md`, and every Class A summary a walk adds a requirement to
- `rfc/enrolled.txt` - one row per Class B stem that enrols
- `rfc/not-enrolled.txt` - a disposition for any Class B stem that does not
- `docs/features/rfc-status.md` - the RFC 1997 Status cell, and coverage and remaining
  prose for the ten Class B rows
- `ai/RFC-REQUIREMENTS.md`, `rfc/requirements/*.md` - regenerated by
  `./le rfc index-update`
- `rfc/extraction/README.md` - a paragraph on what a `Supported` row now owes
- `docs/architecture/core-design.md` - declared by the `// Design:` header of every file
  in `internal/le/rfc/`. Named here as unaffected: the doc describes the rfc area as one
  command, and one added check beside `checkUnprovenSupport` changes nothing it states
- `ai/rules/rfc-compliance.md` - one line under "Extraction Completeness" naming the new
  check, so a session meeting its red is routed

## Files to Create
- `rfc/extraction/<stem>.json` - 49 files, one per in-scope stem
- `rfc/full/<stem>.txt` - 10 files, one per Class B stem
- `rfc/short/<stem>.md` - 10 files, one per Class B stem
- `test/plugin/rfc-supported-signoff-refuses-unsigned.ci` - the refusal, end to end
- `test/plugin/rfc-supported-signoff-accepts-signed.ci` - the accepted end state
- `plan/deferrals/rfcgate-6-supported-extraction-signoff.md` - the shard named in the
  metadata table

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | The deliverable is repository gate data and one check. No runtime config surface |
| YANG validation constraints | N-A | No new leaf |
| YANG custom validators | N-A | No new leaf |
| CLI commands/flags | No | `./le rfc check`, `extraction-create` and `extraction-status` already exist and gain no flag |
| CLI grammar (keyword before value) | N-A | No new command |
| Editor autocomplete | N-A | No new leaf |
| Functional test for new RPC/API | Yes | `test/plugin/rfc-supported-signoff-refuses-unsigned.ci` and `-accepts-signed.ci` |
| Pipe completeness | N-A | `./le` development actions are not the operator CLI and carry no pipe surface |
| Env var registration | N-A | No new env var |
| Doctor check for runtime dependencies | N-A | No new file path, socket, port, module, binary or certificate at runtime. The new artifacts are read by a development gate, never by `ze` |
| Prometheus counters/metrics | N-A | No runtime state |
| BGP family surface (new SAFI / capability / attribute) | N-A | No family, capability or attribute changes. A defect a walk finds is fixed under its own spec, which answers this row itself |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | The check and the artifacts are contributor-facing |
| 2 | Config syntax changed? | No | No config surface touched |
| 3 | CLI command added/changed? | No | No new verb or flag |
| 4 | API/RPC added/changed? | No | No API surface |
| 5 | Plugin added/changed? | No | No plugin |
| 6 | Has a user guide page? | No | Contributor documentation only |
| 7 | Wire format changed? | No | No wire behavior |
| 8 | Plugin SDK/protocol changed? | No | No SDK surface |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `docs/features/rfc-status.md` for the ten Class B rows and the RFC 1997 cell; every `rfc/short/` summary a walk adds a requirement to |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` gains the two new `.ci` names |
| 11 | Affects daemon comparison? | No | `docs/comparison.md` compares runtime features |
| 12 | Internal architecture changed? | Yes | `rfc/extraction/README.md` gains what a `Supported` row now owes. `docs/architecture/core-design.md` is DECLARED by the `// Design:` header of `internal/le/rfc/check_status.go` and `internal/le/rfc/check.go`: it describes the rfc area as one command, and adding one check beside `checkUnprovenSupport` changes no behavior that doc describes, so it is named here as unaffected |
| 13 | Route metadata keys added/changed? | No | No route metadata |
| 14 | Prometheus counters added/changed? | No | No counters |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | No registry entry changes |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | DERIVED: run `./le spec citation anchors spec plan/spec-rfcgate-6-supported-extraction-signoff.md` at the start of the final phase and name every doc it lists. `rfc/extraction/README.md` already carries a `<!-- source: internal/le/rfc.Answer -->` anchor over the symbols this spec edits |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `rfc/extraction/README.md` and `ai/rules/rfc-compliance.md` both show the sign-off workflow; verify the command names against `internal/le/rfc/actions.go` |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- register the new check and prove it is reachable
   and RED
   - Tests: `TestCheckSupportedSignoffRefusesUnsignedSupportedRow`,
     `TestCheckSupportedSignoffRefusesSupportedRowWithNoSummary`,
     `TestCheckSupportedSignoffPassesWhenEverySupportedRowIsSigned`,
     `TestCheckSupportedSignoffIgnoresUnsupportedAndFuture`,
     `TestCheckSupportedSignoffRefusesSkeletonArtifact`,
     `TestSupportedRowsHaveDerivableScope`
   - Files: `internal/le/rfc/check_status.go`, `internal/le/rfc/check.go`,
     `internal/le/rfc/check_test.go`
   - Verify: the check runs from `Answer` against in-process fixtures and refuses an
     unsigned row. It is NOT yet called over the real tree, so the repository gate stays
     green while the data lands. Re-derive the two lists here and record any delta against
     A-1 and A-2 in Risks & Assumptions
2. **Phase: Tier 1, authentication and cryptography (Class A, 9 stems)** -- `rfc5176`,
   `rfc2869`, `rfc2865`, `rfc4301`, `rfc3748`, `rfc2866`, `rfc4303`, `rfc3948`, `rfc2759`,
   in that order, lowest ratio first
   - Tests: `./le rfc check` clean for each stem; `TestExtractionStatusCountsTierSignoffs`
   - Files: `rfc/extraction/<stem>.json` per stem, plus any summary a walk appends to
   - Verify: one commit per stem. Each carries the artifact, any new requirement rows and
     their tests. Every found-and-unmet MUST produces the AC-8 ask before the commit
3. **Phase: Tier 2, session establishment and identity (Class A, 6 stems)** -- `rfc7947`,
   `rfc8203`, `rfc9234`, `rfc9003`, `rfc5492`, `rfc6286`
   - Tests: as phase 2
   - Files: as phase 2
   - Verify: `rfc6286` has zero uppercase MUST-level keywords and will derive `prose` or
     `manual-walk`; a `manual-walk` here needs a `register-reason` or
     `checkUnprovenSupport` refuses the pairing with its `Supported` row
4. **Phase: Tier 3, monitoring and validation feeds (Class A, 5 stems)** -- `rfc9069`,
   `rfc8671`, `rfc9582`, `rfc6811`, `rfc6396`
   - Tests: as phase 2
   - Files: as phase 2
   - Verify: RFC 7854 Section 4.5 was a BMP defect found on 2026-08-30 outside this scope;
     read whether its sibling obligations are extracted in `rfc8671` and `rfc9069`
5. **Phase: Tier 4, BGP wire core and families (Class A, 14 stems)** -- `rfc7947` excluded
   as done; `rfc8950`, `rfc5549`, `rfc8092`, `rfc7911`, `rfc7313`, `rfc4456`, `rfc8654`,
   `rfc4761`, `rfc3032`, `rfc4760`, `rfc4360`, `rfc4364`, `rfc1997`, `rfc2918`
   - Tests: as phase 2
   - Files: as phase 2, plus the RFC 1997 Status cell in `docs/features/rfc-status.md`
   - Verify: this tier is measured densest, so the expected yield is low. A tier that
     finds nothing is a result, recorded as such, not a reason to stop early
6. **Phase: Tier 5, provisioning, DNS and file transfer (Class A, 5 stems)** -- `rfc4578`,
   `rfc7534`, `rfc7535`, `rfc1350`, `rfc2347`
   - Tests: as phase 2
   - Files: as phase 2
   - Verify: four of the five have zero uppercase MUST-level keywords; expect `prose` or
     `manual-walk` registers and state the register reason per stem
7. **Phase: Class B enrolment (10 stems)** -- in tier order: `rfc4302`, `rfc5282`,
   `rfc2385`, `rfc5082`, `rfc9687`, `rfc9384`, `rfc5798`, `rfc8516`, `rfc7607`, `rfc4762`
   - Tests: `./le rfc check` clean per stem; the ledger checks that fire on a newly
     enrolled stem
   - Files: `rfc/full/<stem>.txt`, `rfc/short/<stem>.md`, `rfc/extraction/<stem>.json`,
     `rfc/enrolled.txt` or `rfc/not-enrolled.txt`, `docs/features/rfc-status.md`
   - Verify: ONE stem per commit, never a batch (R-3). Enrolling gates that stem's MUSTs,
     so the tests land in the same commit or the stem does not enrol. A stem whose ledger
     row overstates what Ze does raises R-7 to Thomas rather than being enrolled at a
     status nobody can defend
8. **Phase: arm the check** -- call `checkSupportedSignoff` over the real tree
   - Tests: the two `.ci` cases; `./le verify worktree` on this commit
   - Files: `internal/le/rfc/check.go`, `test/plugin/rfc-supported-signoff-*.ci`,
     `rfc/extraction/README.md`, `ai/rules/rfc-compliance.md`, `docs/functional-tests.md`
   - Verify: this is the last commit because the check is red until every phase above has
     landed (A-5). Run `./le rfc index-update` here and read the per-RFC exclusion ratio
     for all 49 new sign-offs (R-4)

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | All 49 stems have an artifact the check ACCEPTS, and the count is read from `./le rfc extraction-status`, never from `ls rfc/extraction` |
| Feature completeness | `checkSupportedSignoff` is called from `Answer` and reached by the two `.ci` cases, not only by unit fixtures |
| Correctness | No authored field duplicates a derived one. Every `excluded-kind` is from the closed set and its reason says what the kind's row in `rfc/extraction/README.md` demands |
| Correctness | Every `manual-walk` sign-off over a support-claiming row carries a `register-reason`, and its source does not derive `rfc2119` |
| Naming | Requirement ids follow `<RFCSTEM>-<section>-<n>`; new obligations get new ids and no existing id changes |
| Data flow | The scope set is re-derived from `docs/features/rfc-status.md` at phase 1 and at closure, never carried forward from this spec's table |
| Rule: `ai/rules/rfc-compliance.md` | No `{gap}`, `{not-applicable}` or `partial` was written for a found-and-unmet obligation without a recorded answer from Thomas |
| Rule: `ai/rules/evidence.md` | Every claim about what a check does names the function in `internal/le/rfc/` that produces the verdict |
| Rule: `ai/rules/completion.md` | No stem was skipped, and no tier was closed with a stem carrying an unclassified site |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| 49 accepted sign-offs | `./le rfc extraction-status` reports `signed` at 55 or more |
| No violation for an in-scope stem | `./le rfc check` output contains none of the 49 stems |
| Every artifact fully classified | no `"disposition": null` in any file under `rfc/extraction/` |
| Class B enrolled or dispositioned | each of the ten stems appears exactly once across `rfc/enrolled.txt` and `rfc/not-enrolled.txt` |
| RFC 1997 status corrected | its Status cell reads `Supported` |
| The check is wired | `grep -n checkSupportedSignoff internal/le/rfc/check.go` returns a call site |
| Quota untouched | `rfc/drain-budget.txt` still reads `rate 0` |
| Ledger regenerated | `./le rfc index-update` produces no diff on a second run |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | The Tier 1 walk covers RADIUS, EAP, MS-CHAPv2, IPsec and ESP. Any obligation about verifying a shared secret, an authenticator, an integrity check value or a replay window is the class that produced the RFC 8907 defect. Treat one as found-and-unmet until the producing function is read |
| Guard fails open | An obligation whose Ze implementation returns a zero value on an error path is a guard that fails open (`ai/rules/evidence.md`). Record it as found-and-unmet even when a checklist row already exists |
| Downgrade | RFC 7427's defect was a negotiated algorithm not chosen from the peer's advertised list. Look for the same shape in `rfc5282` and `rfc3748` |
| Error leakage | A sign-off reason must not quote a secret, a key or an operator credential out of a test fixture |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| `./le rfc check` reds on a derived field mismatch | The artifact edited a derived field. Regenerate with `./le rfc extraction-create` and re-apply the authored dispositions only |
| `./le rfc check` reds on `check_coverage_ratchet` after an enrolment | R-3: the stem enrolled without its tests. Land the tests or take the stem back out of `rfc/enrolled.txt` |
| A walk finds an obligation Ze does not meet | AC-8: ask Thomas the same session, quoting the id, the sentence and the producing function. Do not annotate |
| A tier is too large for one agent | R-5: report the size to the main thread and let it re-cut by stem. Do not trim the tier |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- The gate's own predicate for a support claim, `statusIsSupportClaim`, is broader than
  this spec's scope: it treats everything except `Unsupported` and `Future` as a claim,
  `Partial` included. The narrower cut here is a release-critical slice, not a judgement
  that a `Partial` row's unbounded checklist is acceptable.
- `checkUnprovenSupport` documents its own hole in its error string. That is the cheapest
  kind of finding available in this repository and it was free: reading the producer, as
  `ai/rules/evidence.md` requires, surfaced ten public claims outside the whole gate.
- The three denominators in `rfc/extraction/README.md` are the reason the ratio table in
  this spec carries a caveat rather than a coverage claim. A ratio built on the wrong
  denominator would have looked like a measurement and been an assertion.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Deliver the 49 sign-offs AND one new check | Data only, no check | Nothing holds the property otherwise. `checkExtractionRatchet` refuses a stem LOSING a sign-off; it says nothing about a stem that never had one, so a new `Supported` row lands unsigned and the 49 decay one row at a time. The check is one function beside an existing one |
| Land the check LAST | Land it first, TDD style | The check is red for 49 stems until they land, and a red gate blocks every unrelated commit in a shared checkout (`plan/journal/concurrent-rfc-gate-stale.md`). Phase 1 still writes it and proves it red against fixtures, so the TDD property is kept without reddening the tree |
| Package boundary is the TIER | Per stem, or one agent for all 49 | A stem is too small to justify a phase report; 49 is too large for one context and would produce exactly the trimming `ai/rules/planning.md` bans. A tier is 5 to 14 stems with one shared subject matter |
| Order by blast radius, corroborated by extraction ratio | Alphabetical, or by source size | Two independent signals agree: the 2026-08-30 defects clustered on authentication and monitoring, and the ten thinnest Class A summaries are eight Tier 1 stems and both BMP stems. Size and alphabet correlate with neither |
| Include scope-qualified `Supported` rows | Exact `Supported` only | "Supported on Linux" promises conformance within a named scope. Excluding it makes the words after `Supported` an escape hatch from the gate |
| Class B enrols rather than having its ledger row lowered | Lower the ten rows to `Partial` | Lowering a public claim is a compliance decision Thomas owns (`ai/rules/rfc-compliance.md`), and the default answer is full compliance. R-7 routes a stem to him only when the tests cannot be written |

## Known Limitations

- **It does not walk the `Partial` and `Experimental` rows.** 64 rows read `Partial` and
  29 read `Experimental` on `docs/features/rfc-status.md`. Their checklists are equally
  unbounded, and each already discloses to a reader that the RFC is not fully met, so a
  missing checklist line there is incomplete disclosure rather than a false promise. They
  remain covered by row D4 of `plan/deferrals/rfcgate-0-umbrella.md`.
- **It does not arm `rfc/drain-budget.txt`.** The rate stays 0. Arming it is a one-line
  owner edit and the natural follow-on once this spec measures real throughput per tier,
  which is what owner decision D5 said the first batch was for. The measurement each
  phase report carries (stems per session, obligations found per stem) is the input to
  that decision. Row D5 of `plan/deferrals/rfcgate-0-umbrella.md` stays live.
- **It does not fix the defects a walk finds.** A found-and-unmet MUST is asked to Thomas
  and homed in its own spec (`ai/rules/rfc-compliance.md`, `ai/rules/planning.md`). Only a
  defect that blocks this spec's own goal is fixed inside it.
- **A sign-off bounds keyword-visible sites, not obligations.** `rfc/extraction/README.md`
  is explicit: recall can be near zero for indicative prose, and `unsourced-ids` records
  what the extractor cannot see. This raises a floor from zero; it reaches no ceiling.
- **A first sign-off has no ratchet baseline.** `checkExtractionRatchet` compares a stem
  against its own HEAD row, so all 49 sign off unratcheted. The published per-RFC
  exclusion ratio is the only control, and R-4 makes reading it part of each tier review.

## What starts applying once a stem signs

| Ratchet | Producer | Fires when |
|---|---|---|
| Extraction is monotonic | `checkExtractionRatchetAgainst` (`internal/le/rfc/check_extraction.go`) | a stem that carried a sign-off at HEAD carries none now |
| Exclusions are shrink-only | the same function | a signed stem's exclusion count RISES without a `resign-reason` and a bumped `signed-off` date. A `relocated-to-spec` site counts as an exclusion for this purpose |
| Relocation claims are re-read | `relocationErrors` (`internal/le/rfc/signoff.go`) | the named spec is gone, no longer holds `reserved-id`, or the summary declares that id again. A closing spec turns the site red by design |
| The source sha pins the text | `evaluateExtraction` (`internal/le/rfc/signoff.go`) | the RFC source changes under a signed artifact |
| Drain floor | `checkDrainFloor` (`internal/le/rfc/check_extraction.go`) | inert at `rate 0`; it becomes live the day the quota is armed, and these 49 are the credit it counts |

## RFC Documentation (Scope: protocol)

This spec adds no enforcing code, so it adds no `// RFC NNNN Section X.Y:` comment of its
own. Any requirement a walk newly extracts and any defect a walk finds is documented at
its enforcing code under its own spec, with the section quoted verbatim.

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
- [ ] AC-1..AC-10 all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`). An in-place `./le verify current` is void the moment the tree moves under it
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md`)
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Deferral shard resolved: no live row without a destination

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/session/review.go`
- [ ] Learned summary written to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec + learned summary
- [ ] **Commit B:** `git rm plan/<spec>` only (commit A preserves the spec in history)
