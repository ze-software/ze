# Spec: feature maturity declared

| Field | Value |
|-------|-------|
| Status | design |
| Scope | tooling |
| Depends | - |
| Phase | 5/5 |
| Handoff | - |
| Updated | 2026-10-09 |

Set aside 2026-10-09 (owner: "Set aside the 7 specs waiting on something else"): back to `design`, because the owner's 2026-10-08/09 direction on Doc review (Decisions for Owner, last owner decision) is not yet designed into this spec. Next: rewrite the spec around it, then the verify stage.

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Every public statement of how mature a Ze feature is comes from a hand-written
label. `docs/features.md` carries 131 rows whose Status cell an author typed, and
the website carries a second, independent set of 52 cards in
`website/data/features.json` whose `status` field and core/experimental section
an author also typed. Nothing checks either one against the evidence the
repository holds.

A read-only audit on 2026-10-07 (four batches, seed data at the end of this spec)
scored all 131 rows against a draft generic bar. Of the 90 rows claiming
`Supported`, 24 earned it. Two rows under-claim (Backup and restore, MOBIKE).
About a dozen rows carry prose that is factually false at the producer.

The goal: a feature's maturity becomes a declared fact with an evidence pointer
for every criterion, a check refuses a level whose evidence is missing, and every
report that states maturity derives from that one declaration:

| Surface | Today | After |
|---------|-------|-------|
| `docs/features.md` | hand-written table, tracked | derived artifact rendered from the declarations, untracked |
| Site feature cards and the published count | hand-set `status` and section in `website/data/features.json` | card maturity derived from the declarations the card names |
| Status reports (spec work, weekly update, `/ze-status`) | read the hand table | `./le feature report`, with the gap to the next level per feature |
| Vocabulary | the status sentence at the top of `docs/features.md`, plus copies in `featureStatuses` (`internal/le/doc/yangcontract/drift.go`) and `featureStatusLabels` (`internal/le/site/datapages.go`) | declared once in the new `internal/le/feature` package |

`ai/rules/rfc-compliance.md` requires a false public support claim to be
corrected immediately on the surface that carries it. The owner is deciding the
order: whether the over-claims and false prose are corrected in
`docs/features.md` now, ahead of this tooling, or land as this spec's migration
phase (Decisions for Owner, D-5). This spec writes both routes so either can run.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/principles.md` - registration and derive-once
  → Constraint: "Every fact MUST be declared once, and every other surface MUST derive from that declaration." Today maturity is declared twice (features.md row, features.json card) and the vocabulary three times (features.md header sentence, `featureStatuses`, `featureStatusLabels`).
  → Constraint: a new feature MUST register itself and be discovered; adding a feature MUST NOT edit a central list. A declaration directory discovered by listing meets this; a hand table does not.
  → Constraint: "A rule, a document or a comment MUST NOT carry a copy of what a command already prints." The contributing page explains the model and points at `./le feature report`; it does not restate the level list.
- [ ] `ai/rules/architecture.md` - grep for the existing pattern first
  → Decision: the existing pattern for "authored public support claim, checked, rendered into a public page" is the RFC ledger: `## Meta` table in each `rfc/short/<stem>.md`, parsed once by `ParseMeta` (`internal/le/rfc/meta.go`), rendered by `./le rfc index-update` into `docs/features/rfc-status.md`, gated by `./le rfc check` in the verify stages. This spec copies that shape for features.
- [ ] `internal/le/derived/derived.go` - package doc
  → Constraint: a file the checkout derives from its own tree is registered with `derived.Register` from its generator's own `init()`, is not committed (`docs/features/rfc-status.md` is listed in `.gitignore`), is deleted when an input is written and rebuilt when read. Generated `docs/features.md` follows the same rule: removed from git, ignored, registered.
- [ ] `docs/architecture/testing/verify-freshness-scope.md` - verify stage population
  → Constraint: `internal/le/verify/engine/stages.go` declares the ordered stage list by hand ("declared here rather than discovered"), with `stage("rfc", "check")` among the gates. The feature check is added as one stage beside it.
- [ ] `website/AI.md` - site data files and weekly procedure
  → Constraint: the weekly procedure's step "Check Features for drift" tells the writer to "move a feature from Experimental to shipped" by editing `data/features.json`. That instruction becomes wrong and is rewritten in the same change.
- [ ] `docs/architecture/testing/interop.md` - interop suites and scenario identity
  → Constraint: a scenario directory name is its identity and `interoplab.Discover` (`internal/le/interoplab/discover.go`) matches it exactly; S2 evidence is a scenario name resolved through that function, never a hand list.
- [ ] `docs/contributing/rfc-conformance-gates.md` - RFC ledger gates
  → Constraint: S3 reads the ledger through `internal/le/rfc`'s own parse (Meta, requirement rows, extraction sign-off in `rfc/extraction/<stem>.json`). A second parser of `rfc/short/*.md` is banned by derive-once.
- [ ] `docs/architecture/core-design.md` - le areas and the doc drift gate
  → Constraint: `checkFeaturesMD` in `internal/le/doc/yangcontract/drift.go` (Design: core-design.md) validates the hand table's Status vocabulary. With a generated table there is nothing hand-written to validate; the check is deleted, not kept beside the generator (`ai/rules/no-layering.md`).

**Key insights:**
- The site count does NOT read `docs/features.md`. `factsFromSiteData` (`internal/le/site/facts.go`) counts the cards in the `core` and `experimental` sections of `website/data/features.json` (33 + 19 = 52 today). The two surfaces have different granularity (52 cards, 131 rows) and nothing arbitrates between them.
- The audit's own counts: Supported claimed 90, earned 24 (23 clean, plus Chaos MCP whose bar did not fit; Tech-Support Bundle earns it after one doc fix). Supported claimed but earned Experimental 45, Partial 18, Stub-backed 2.
- The audit ran no test. Every "ok" cell means "the file exists", not "it passes".
- `tmp/` is ignored by git, so the audit files under `tmp/feature-maturity/` can disappear. The seed table at the end of this spec carries the per-row result so the spec stands alone.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `docs/features.md` (139 lines) - a status sentence defines six values in prose; the rest is one table, Feature, Status, Description, one row per feature, descriptions with inline `<!-- source: -->` anchors and links into `docs/features/` and `docs/guide/`.
- [ ] `internal/le/doc/yangcontract/drift.go` - `featureStatuses` repeats the six values lowercase; `checkFeaturesMD` refuses a row whose Status cell is not one of them, and its detail string repeats the list a third time.
- [ ] `internal/le/site/datapages.go` - `featureData`, `featureSection`, `featureCard` model `website/data/features.json`. `featureStatusLabels` admits one status, `experimental`; an empty status means shipped. `shippedCards` concatenates the `core` and `experimental` sections; `featureCard.validate` refuses an unknown status, category or empty href.
- [ ] `internal/le/site/facts.go` - `factsFromSiteData` sums the card counts of the `core` and `experimental` sections into `Features.CoreExperimental` and refuses zero. `derived.go` (llms.txt) and `home.go` publish that number.
- [ ] `internal/le/site/docsmanifest.go` - publishes `docs/features.md` at `reference/feature-status`.
- [ ] `internal/le/rfc/meta.go` - `Meta` holds the authored public row cells verbatim ("a generator that synthesized it would be inventing the claim it exists to publish"); `Enrolled()` answers gating.
- [ ] `internal/le/rfc/render_ledger.go` - `renderStatusPage` writes a source anchor naming `docs/features.md` as the "feature status vocabulary" source.
- [ ] `internal/le/rfc/register.go` - registers the area with `leroot.Register` (GroupGate) and its generated files with `derived.Register`.
- [ ] `internal/le/verify/engine/stages.go` - hand-declared stage list, including `rfc check` and the `doc check` stages.
- [ ] `tmp/feature-maturity/BRIEF.md` - the draft generic bar S1-S6 the audit applied.
- [ ] `tmp/feature-maturity/bgp-routing.md`, `config-cli-platform.md`, `diagnostics-api-security.md`, `ipsec-services-access.md` - the per-row audit, its over-claim lists and its "Bar did not fit" sections.

**Behavior to preserve:**
- The published page path `reference/feature-status` and its table columns Feature, Status, Description, and every row's description text except the rows this spec corrects.
- The site's features page: card order as the data file states it, the seven categories, chips, bullets, solid versus dashed rendering, and the refusal of a zero count.
- `./le rfc check`, `./le rfc index-update` and the RFC ledger are read, never changed.

**Behavior to change:**
- `docs/features.md` stops being authored; it is rendered from per-feature declarations.
- A card's shipped or experimental state stops being authored in `website/data/features.json`; it is derived from the declarations the card names.
- A level a feature cannot evidence is refused by a gate in `./le verify`.
- The rows the audit found false or under-claimed are corrected after re-verification at the producer.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- One Markdown declaration per feature, `features/<id>.md`, carrying a `## Meta` table (fields below) and the public description prose. Discovered by directory listing; no index file.
- Commands: `./le feature check`, `./le feature report`, `./le feature index-update`; the site build; the verify stage.

### Transformation Path
1. Parse: `internal/le/feature` reads every `features/*.md` through one parser into one declaration value per feature (the `ParseMeta` precedent). A malformed or unknown field is refused by name.
2. Resolve evidence: each pointer is resolved by the producer that owns the fact: file existence for tests and docs; `interoplab.Discover` for scenarios; `internal/le/rfc` for RFC stems, enrolment, gap rows and extraction sign-off; the spec metadata reader for open `plan/immediate/` specs; the journal reader (`internal/le/spec/journal`) for row dates.
3. Compute the ceiling: the highest level whose mechanical criteria pass and whose attested criteria carry a current attestation. Umbrella features take the worst of their parts.
4. Judge: the check refuses a declared level above the ceiling; the report lists every feature with its declared level, ceiling, and the unmet criteria between declared and next level.
5. Render: the derived `docs/features.md` (header vocabulary from the one vocabulary declaration, one row per declaration); the site renderer and facts read card-to-feature mappings and derive each card's state and the count.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Declaration files -> `internal/le/feature` | Markdown `## Meta` table, one parser | No |
| `internal/le/feature` -> `internal/le/rfc` | exported read of Meta and requirement rows; no re-parse | No |
| `internal/le/feature` -> `internal/le/interoplab` | `Discover` by scenario selector | No |
| `internal/le/feature` -> `internal/le/site` | site renderer and facts read the parsed declarations through an exported function; `website/data/features.json` names declaration ids | No |
| `internal/le/feature` -> `internal/le/derived` | `derived.Register` for `docs/features.md` | No |
| `internal/le/verify/engine` -> `./le feature check` | one stage entry | No |

### Integration Points
- `leroot.Register` - the `feature` area registers its three verbs (GroupGate), as `internal/le/rfc/register.go` does.
- `derived.Register` - `docs/features.md` registers as a derived artifact fed by the declaration directory.
- `internal/le/site` - `featureCard` gains the list of declaration ids it represents and loses `status`; `shippedCards` and `factsFromSiteData` derive membership from the declarations.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | to prove: only `internal/le/feature` parses `features/*.md`; grep finds no other reader |
| No unintended coupling (components stay isolated) | No | to prove: `internal/le/feature` imports `internal/le/rfc`, `interoplab`, `derived`, `spec/journal`, never `internal/component/*` |
| No duplicated functionality (extends existing, does not recreate) | No | to prove: RFC facts come from `internal/le/rfc`, scenario names from `interoplab.Discover`; no second parser |
| Zero-copy preserved where applicable (refs, not copies) | No | N-A in substance: host tooling, no wire path |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | to prove: a new feature is one new `features/<id>.md`; no Go edit |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | No | to prove: `featureStatuses` (drift.go) and `featureStatusLabels` (datapages.go) are deleted and their callers derive from the vocabulary in `internal/le/feature` |

## Design

### The declaration: where it lives and why

**Choice: one Markdown file per feature, `features/<id>.md`, a top-level directory beside `rfc/`, each carrying a `## Meta` table and the public description. Discovered by listing the directory.**

| Option | For | Against | Verdict |
|--------|-----|---------|---------|
| Go registration from the feature's own package (`init()` in `register.go`) | literal reading of "registered by the feature's own package" | maturity is a claim about repository evidence (tests, scenarios, docs, specs, journal), never read by the shipped binary; it would put release metadata in `ze`, and the set would vary by build tag (a `ze_core` build drops every BGP row, so the rendered table would depend on tags); about 20 rows have no runtime package at all (Docker, netlab, `le perf`, interop suites, Chaos MCP, umbrella rows); 131 long prose descriptions would become Go string literals | rejected |
| One data file holding all 131 features (JSON or one Markdown table) | one place to read | it is the hand table again under another name; every concurrent session edits the same file, so conflicts across the sessions sharing this checkout become routine; adding a feature edits a central list, which `ai/rules/principles.md` bans | rejected |
| Meta table inside the existing `docs/features/<topic>.md` pages | prose and claim side by side | many rows have no page or share one (`docs/guide/configuration.md` backs several), so the declaration count would not equal the feature count; docs pages are published, so the Meta table would leak into the site | rejected |
| **One file per feature, `features/<id>.md`** | the feature's own record: adding a feature adds one file and edits nothing else (registration by discovery); the same shape as `rfc/short/<stem>.md`, which the tooling already parses, renders and gates (uniformity); concurrent sessions touch different files; prose stays prose | a new top-level directory; Go packages do not point at their declaration (the `Components` field points the other way and is checked) | **chosen** |

The derive-once direction: the declaration names its producing packages
(`Components`), and the check proves those paths exist. Whether every registered
plugin must be named by some declaration is D-10.

### Declaration fields

| Field | Required when | Type | Meaning | Checked mechanically |
|-------|---------------|------|---------|----------------------|
| Name | always | text | public row name | non-empty, unique across declarations |
| Page | optional | repo path | link target of the row name | path exists |
| Kind | always | one of the kinds below | decides what S1, S2 and S3 mean | member of the kind vocabulary |
| Scope | always | `complete`, `partial`, `future`, `rejected` (if D-1 picks two axes) | how much of the stated feature exists | member of the vocabulary |
| Scope gaps | Scope is `partial` | list of named subsets | the unimplemented or unproven subsets the public row must disclose | non-empty when partial, empty otherwise |
| Level | Scope is `complete` or `partial` | `supported`, `experimental`, `stub-backed` | the declared evidence level of the implemented scope | at or below the computed ceiling |
| Parts | Kind is `umbrella` | list of declaration ids | the features an umbrella row summarises | every id exists, no cycle |
| Components | Kind is not `umbrella` | list of repo paths | the packages or directories that produce the behavior | every path exists |
| Real-path tests | S1 | list of test paths, a Go test as file plus function name | S1 evidence | path exists; the functional runner's own discovery includes it; a named Go test function exists |
| Interop | S2 | list of suite plus scenario name | S2 evidence | resolves through `interoplab.Discover` |
| RFCs | S3 | list of `rfc/short` stems | S3 evidence | stem exists; enrolment, implementation kind and open gap rows read from `internal/le/rfc` |
| Docs | S4 | list of doc paths | the user-facing pages | path exists; each carries at least one source anchor that resolves |
| Doc review | S4 attestation | date plus the claims checked at the producer | a reader compared the page and the row prose to the producing function | date present and not older than the newest change to a listed Docs path or the description (staleness, not truth) |
| Defect review | S5 attestation | date plus the journal rows and specs judged, each with its verdict at the producer | a reader judged open defects at the producer, never from a journal Fix cell | date present and not older than the newest journal row or `plan/immediate/` spec that names a Components path |
| Stub evidence | S6 | subset of the listed tests or scenarios marked stub | which evidence comes only from a stub harness | marked items are members of the listed evidence |
| Extra criteria | optional | list of criterion, the level it gates, evidence pointer or attestation | per-feature completion criteria (seeded from the audit) | pointer resolves as for S1 and S2; attestation dated |
| Description | always | prose section, not a Meta row | the public row text, moved verbatim | non-empty; no unescaped table pipe |

The id is the file stem. Row order in the rendered table: by Kind, then Name,
unless D-11 keeps the current hand order (then a `Rank` field is added, as
`Meta.Rank` does for the RFC ledger).

### Levels (recommended model: two axes; D-1 decides)

The audit found `Experimental` and `Partial` are not ordered: most protocol rows
qualify for both. `Stub-backed` is a fact about the evidence source, decided by
S6. The recommended model separates them:

| Axis | Values | Question it answers |
|------|--------|---------------------|
| Scope | complete, partial, future, rejected | how much of the stated feature exists |
| Evidence level (implemented scope only) | supported, experimental, stub-backed | how well what exists is proven |

Public label rendered from the pair:

| Scope | Level | Rendered Status |
|-------|-------|-----------------|
| complete | supported | Supported |
| complete | experimental | Experimental |
| complete | stub-backed | Stub-backed |
| partial | supported | Supported (partial) |
| partial | experimental | Experimental (partial) |
| partial | stub-backed | Stub-backed (partial) |
| future | - | Future |
| rejected | - | Rejected |

If D-1 keeps one ladder, the Scope axis collapses and `partial` becomes a Level
value; AC-6 and AC-16 change accordingly. Everything else in this spec holds.

### Completion criteria: the generic bar

| Criterion | Supported requires | Experimental requires |
|-----------|--------------------|-----------------------|
| S1 real path | at least one real-path test, meaning per Kind (table below) | Components exist and the feature is reachable through its real entry point (attested if no S1 test) |
| S2 interop | per Kind; at least one non-stub scenario where the Kind requires one | none |
| S3 RFC | every listed stem that counts (D-3) shows no open gap on a MUST row Ze implements | none |
| S4 docs | a Docs page exists, anchors resolve, Doc review current | none |
| S5 defects | no `plan/immediate/` spec names a Components path; Defect review current with every judged item verdict "not a defect in this feature" or "fixed at producer" | none |
| S6 not stub-only | at least one S1 or S2 item not marked stub | level `stub-backed` requires every S1 and S2 item for the external dependency to be stub |
| Extra criteria | every extra criterion gating `supported` met | every extra criterion gating `experimental` met |

### Completion criteria: per-Kind meaning

| Kind | Examples (audit row) | S1 means | S2 means | S3 means |
|------|---------------------|----------|----------|----------|
| protocol | BGP families, IS-IS, OSPF, BFD, IKEv2 engine, DHCP, DNS | `.ci` from config or CLI to the wire | interop scenario against a third-party implementation | the RFC ledger, per D-3 |
| daemon | interfaces, firewall, storage, health, audit, diagnostics, CLI, web, API, MCP | `.ci`, `.et` or `.wb` driving the user's command and asserting the answer | n/a, except where the surface is a standard a third-party client consumes (gNMI, MCP, REST OpenAPI): a run with that client | listed stems only (RFC 9728 for MCP) |
| library | IKEv2 wire format, crypto primitives, XFRM interfaces | reached by a named real-path test of a consuming feature, plus fuzz or known-answer tests where the extra criteria name them | interop of the consuming feature | the ledger |
| dev-tool | `le perf`, Chaos MCP, Chaos MRT recording | a recorded run of the tool producing its artifact | an external consumer reads the output (another daemon for perf, bgpdump for MRT) | n/a |
| packaging | Docker, netlab, installation, self-update | the artifact builds and runs `ze` under a gate | a live run in the target environment (containerlab, VM boot) | n/a |
| test-infra | interoperability testing, IPsec interop testing | the suite runs on a schedule and its last result is recorded | its checkers cannot pass vacuously (an empty baseline is an error) | n/a |
| umbrella | BGP Protocol, MPLS/LDP/RSVP-TE | none of its own | none of its own | none of its own; Level and Scope are the worst of Parts |

A requirement Ze meets by configuring the kernel (DF probes, path MTU) takes S3
from the RFC Meta implementation kind (`third-party` or `mixed`), and S1 asserts
the state Ze installs, per `ai/rules/rfc-compliance.md`. No separate Kind.

### Per-feature extra criteria

Each declaration may carry extra criteria. The seed table at the end lists the
audit's "Feature-specific extra criteria" column for every row; Phase 3 turns each
non-`none` entry into an Extra criteria item with the level it gates and its
evidence pointer, or records it as not adopted with a reason in the declaration.

### What the check can and cannot verify

| Verified mechanically | Not verifiable mechanically (attested, dated, staleness-checked) |
|-----------------------|-------------------------------------------------------------------|
| every declared path exists (Components, tests, docs, Page) | that a test would go red if the path broke (vacuity), unless a discrimination record exists |
| a test path is one the functional runner discovers | that a listed test passes today, unless a recorded run exists (D-6) |
| a named Go test function exists in its file | that a doc page's claims match the producing code |
| a scenario name resolves through `interoplab.Discover` | that a journal row is still open (Fix cells go stale; judged at the producer) |
| an RFC stem exists, is enrolled, its implementation kind, its open gap rows, its extraction sign-off | that an evidence item is a stub, when the harness does not say so |
| no `plan/immediate/` spec names a Components path in its Files to Modify | that the row prose is true |
| umbrella level is not above the worst of its parts | that a hard-coded number in prose (row 139 "190", row 94 "138 metrics", row 58 "11") is current |
| an attestation is not older than the newest input it judged | whether the feature is useful or deployed anywhere |

The report prints the right-hand column's items per feature as "attested on
<date>" or "re-review owed", so a reader sees what rests on a person's word.

### Reporting

`./le feature report` lists every declaration: id, Name, Kind, Scope, declared
Level, ceiling, rendered Status, and for the next level up the unmet criteria,
each with the evidence pointer it lacks. A declared level below the ceiling is
listed as a promotion candidate, not refused. Output follows `ai/rules/cli.md`:
structured payload, `| json` with kebab-case keys. `./le feature report
feature <id>` narrows to one.

### Corrections the migration carries

Under-claims, each re-verified at the producer before the level moves:

| Row (audit line) | Claimed | Audit finding | Correction |
|------------------|---------|---------------|------------|
| Backup and restore (12) | Experimental | 11 `test/plugin/data-backup*.ci`, `data-restore-*.ci` exist, not run | run them; Supported only after a green run is recorded |
| IPsec MOBIKE (85) | Rejected | `engine/mobike.go` exists, interop scenarios `mobike-initiator`, `mobike-responder` exist, the MOBIKE section of `docs/guide/ipsec.md` documents it | Experimental (partial), prose aligned with the guide; RFC 4555 Partial disclosed |

False prose, each re-verified at the producer before the sentence changes (the
audit's statement is a claim until then, `ai/rules/evidence.md`):

| Row (audit line) | Audit says the row claims | Audit says the producer does |
|------------------|---------------------------|------------------------------|
| 35 MPLS/LDP/RSVP-TE | no open-source RSVP-TE peer exists | `test/interop-rsvpte/Dockerfile.freertr` builds freeRtr |
| 54 Tech-Support Bundle | SMART collection via ATA/NVMe ioctls | none of the 20 modules in `moduleRegistry` (`internal/component/support/modules.go`) collects SMART |
| 58 Core Diagnostics | BFD raw capture; root privilege enforcement; 11 commands | the `protocol` enum in `internal/plugins/diag/yang/ze-diag-cmd.yang` lacks bfd; `privilege.CheckPrivileges` only warns; 13 listed |
| 62 Packet Capture | `capture-raw start bgp` grammar | YANG is `show capture raw action start protocol bgp` |
| 69 Named Service Listeners | detects overlapping ip:port across every service | `CollectListeners` applies no defaults in the daemon and validate paths |
| 79 IPsec Data Model | IKE groups with DPD, key-exchange, close-action | close-action and dpd action select nothing; ikev1 runs ikev2 (open immediate specs) |
| 94 Netdata telemetry | matches Netdata "exactly", "138 metrics" | no scrape test, no golden comparison, count hand-written |
| 120 AIGP | capability negotiation; not consumed in best path | `test/parse/dead-capability-rejected.ci` refuses `capability { aigp }`; `BestStepAIGP` in `rib/bestpath.go` |
| 126 IXP route server | (ledger) RFC 7947 Supported | `plan/immediate/spec-rfc7947-adj-rib-in-accepts-filtered-updates.md` records the Section 2.1 MUST unmet |
| 128 Config Schema Stamp | prunes fields on a newer stamp | `RecoverConfig` (`internal/component/config/stamp.go`) loads the newest parseable rollback |
| 133 Archive Pruning | prunes archives | the `commit-revisions` leaf in `ze-system-conf.yang` prunes `file://` archive copies only |
| 139 RFC conformance ledger | "190 pages today" | `rfc/short` holds 202 summaries; a hand copy of tool output |

Row 126's ledger finding is an `rfc/short/rfc7947.md` correction owed under
`ai/rules/rfc-compliance.md`; this spec names it and does not edit the ledger.

## Decisions for Owner

| ID | Question | Options | Recommendation |
|----|----------|---------|----------------|
| D-1 | One ladder or two axes? | (a) one ladder Supported > Experimental > Partial > Stub-backed; (b) Scope axis times Evidence axis | (b): the audit found Experimental and Partial unordered for most protocol rows, and Stub-backed is an evidence-source fact |
| D-2 | Does lab-speaker injection count as S2 for negative cases (malformed input no third-party implementation will send, row 117 typed-family NLRI discard)? | (a) yes, when the scenario names the injected bytes and a third-party peer is on the receiving side; (b) no, S2 needs a third-party sender | (a), recorded as a per-feature extra criterion so it is visible |
| D-3 | Do RFCs that are not enrolled count against S3 (DNS: RFC 1035 not enrolled by owner ruling; RFC 4861/8106 for router advertisements)? | (a) only enrolled stems count; (b) a listed but unenrolled stem caps the feature at Experimental | (a), with the report printing "S3 bounded: <stems> not enrolled" so the bound is public |
| D-4 | Does a large surface get per-subset status (IKEv2 engine, EAP, BGP, CLI)? | (a) no; (b) yes, as child declarations under an umbrella, reusing worst-of | (b) where one open defect demotes a large surface; the mechanism already exists for umbrellas |
| D-5 | Order: correct the false public claims now, or as this spec's migration? | (a) immediately in `docs/features.md`, before any tooling (`ai/rules/rfc-compliance.md`: "corrected immediately"); (b) in Phase 3 | owner is deciding; (a) satisfies the rule today and Phase 3 then moves already-correct prose |
| D-6 | Does Supported require a recorded green run, not just an existing test? | (a) file existence suffices; (b) a recorded run (nightly evidence workflow `.github/workflows/evidence-nightly.yml`) newer than the last change to the test | (b) eventually; (a) for the first landing, with the report printing "exists, not run" |
| D-7 | Declaration location | `features/<id>.md` (chosen) or another directory name | `features/`, beside `rfc/` |
| D-8 | A single false sentence fails S4: drop the level, or a distinct "doc fix owed" state? | (a) drop to Experimental; (b) keep the level, refuse the commit until the prose is fixed | (b): the check refuses a stale Doc review, which forces the fix rather than relabelling |
| D-9 | How does a site card map to the two-state site (solid shipped, dashed experimental)? | (a) solid only when every named feature is complete and supported; (b) solid when supported, partial or not | (a) |
| D-10 | Should every registered plugin be named by some declaration's Components? | (a) no; (b) the report lists unclaimed plugins; (c) the check refuses them | (b) first |
| D-11 | Do dev-tool and test-infra features stay in the product feature table, and in what order? | (a) yes, labelled by Kind, ordered by Kind then Name; (b) a separate tooling table on the same page; (c) keep today's hand order through a Rank field | (a) |

-> Decision (owner, 2026-10-07): implement this spec.
-> Decision (owner, 2026-10-07): D-5 is (a). The false public claims are corrected in `docs/features.md` and `website/data/features.json` before any tooling, as a separate docs commit; Phase 3 then moves already-correct prose.
-> Decision (owner, 2026-10-07): D-6 is (b) from the first landing. Supported requires that the real-path tests exist AND have a recorded green run; "exists, not run" never reaches Supported. The run record is in scope for this spec.
-> Decision (owner, 2026-10-07): D-1, D-2, D-3, D-4, D-7, D-9, D-10 and D-11 take the Recommendation column.
-> Decision (owner, 2026-10-07): D-8 is (b). "We never want to publish lies when we know they are not truthful." Missing evidence lowers the level; a recorded false sentence refuses the commit until the prose is fixed or the doc review is redone.
-> Decision (owner, 2026-10-07): run staleness is option (c). A recorded green run (real-path or interop) is current only while its test file or scenario directory is unchanged AND it is no older than 30 days; an older run is stale and no longer counts toward Supported. The nightly evidence workflow re-records runs. Reason: a run keyed on the test file alone outlived the fixture change in f02d58da88; keying on every imported package would invalidate most runs on every commit.
-> Decision (owner, 2026-10-08, supersedes the 2026-10-07 staleness line above): "a feature once supported needs to remain supported. it should be a warning/option before website regen if the data is getting old". Promotion to supported still requires a current recorded green run. A run that later goes stale (older than 30 days, or its test file or scenario directory changed) never lowers a declared supported level: `./le feature check` accepts it and `./le feature report` lists it as stale. The site build warns before publishing, naming every supported feature with a stale run, and offers to refresh them (re-run and record) first. No CI automation is required. D-8 is unchanged: a known-false sentence still refuses.
-> Decision (implementer, 2026-10-08, the owner decision above): "once supported" is read from HEAD (`internal/le/feature/held.go`, `holdsSupported`). The check before a commit judges the working tree against HEAD, so the commit that raises Level to supported is judged as a promotion and needs every counted run current; once HEAD holds Supported, a stale counted run (aged or changed; S1, S2, or an interop Extra-criteria pointer) is a `Verdict.Warnings` entry instead of unmet (`warnStale`), reported under `warnings` by `./le feature report`. A never-run item still leaves Supported unmet, held or not. `./le site build` calls `feature.StaleSupported` before building, writes one stderr warning per stale run plus the re-record hint, and reports `stale-supported`; `./le site build refresh` first re-records each through `feature.RefreshStale` (RecordRun), reports `refreshed`, and warns only for what is still stale (`internal/le/site/freshness.go`). A keyword, never a prompt, and never the default, since record-run can need Docker or QEMU. The last sentence of the 2026-10-07 Constraint below (a run nobody re-records drops the feature below Supported) no longer holds. Red observed then green: holdsSupported forced false -> TestCheckCountsARealPathRunForThirtyDays, TestCheckCountsAnInteropRunForThirtyDays, TestCheckKeepsAHeldSupportedLevelOverAChangedRun, TestStaleSupportedNamesAHeldLevelWithAStaleRun, TestReportListsAStaleRunAsAWarning RED; forced true -> TestCheckRefusesAPromotionOnAStaleRun, TestCheckRefusesSupportedWithoutGreenRun, TestCheckRefusesInteropWithoutCurrentRun RED; report Warnings dropped -> TestReportListsAStaleRunAsAWarning RED; site refresh branch disabled -> TestFreshnessRefreshReRecordsFirst RED; warning not written -> TestFreshnessWarnsOfEachStaleSupportedRun RED.
-> Constraint (implementer, 2026-10-07, run staleness (c)): NO CI WRITE PATH. The owner decision says the nightly evidence workflow re-records runs, but `.github/workflows/evidence-nightly.yml` holds `permissions: contents: read` and no workflow in `.github/workflows/` commits or pushes anything, so CI cannot land a run record. Not invented. Instead `./le feature record-run due <days>` (`internal/le/feature/due.go`, `RecordDue`) re-records every feature holding a run older than `<days>` days (at most 30), and `docs/contributing/feature-maturity.md` "Recorded runs" tells a maintainer to run it before a release and commit the records. Until the owner grants a CI write path (bot commit or PR), a run nobody re-records ages out and its feature drops below Supported.
-> Decision (implementer, 2026-10-07, run staleness (c)): the bound is `runAgeDaysMax = 30` (`internal/le/feature/runrecord.go`), counted in UTC calendar days from the run's `date`: day 30 current, day 31 stale. "Today" is the check's one date source, `changeDates.today`, now a UTC calendar day passed in by `checkOn(tree, today)` (`Check` passes `calendarDay(time.Now())`), so a dirty path's change date is also UTC. The check says `<item> stale: older than 30 days (recorded <date>)` apart from `<item> stale: test changed since its recorded green run` (`scenario changed` for S2). A run whose date does not parse or falls after today refuses its record (it would never age out). Red observed then green: age branch disabled -> TestCheckCountsARealPathRunForThirtyDays and TestCheckCountsAnInteropRunForThirtyDays RED; `>` changed to `>=` -> both RED on day 30; future-date check disabled -> TestCheckRefusesARunDatedAfterToday RED; due `<=` changed to `<` -> TestRecordDueReRecordsOnlyARunOlderThanTheBound RED.

-> Decision (implementer, 2026-10-07, D-6 mechanism): no existing store records a green run per test. `.github/workflows/evidence-nightly.yml` writes nothing to the tree, `internal/le/test/functional` keeps only `tmp/ze-suite-map.json` (machine-local), and `rfc/discrimination/<stem>.json` records breaks, not green runs. The run record therefore follows the discrimination record's shape: committed `features/runs/<id>.json`, one entry per real-path test item carrying `test-blob`, the git blob id of the test file when it passed (`internal/le/feature/runrecord.go`). A run is current exactly when that id equals the file's blob id now: content, not dates, because a shallow CI checkout holds no history and a reverted edit is not a change. Only passes are recorded. The writer is `./le feature record-run feature <id>` (still owed), which runs each item through the repository's own runners (`./le test <suite> <name>` via `testfunctional.Suites`, `go test -run ^Name$` via `gotoolchain`) and writes the file only when every item passed.
-> Decision (implementer, 2026-10-07): S3 reads the ledger's own verdict. A counting stem must carry a `Support status` starting "Supported" (which `./le rfc check` already refuses over a known unmet MUST) and no gated requirement annotated `{gap}`. RFC 7947 shows why the status matters: its Section 2.1 MUST is unmet with no gap row yet, only prose in `Support remaining`.
-> Decision (implementer, 2026-10-07): the `feature check` verify stage and `TestStagesIncludeFeatureCheck` land with Phase 3's declarations, not Phase 1. The check refuses an empty `features/` (an empty population proves nothing), so a stage added before the migration would redden every verify for every session.
-> Decision (implementer, 2026-10-07, agent 2): S2 resolves through a per-suite catalog, `interoplab.RegisterCatalog` (`internal/le/interoplab/catalog.go`), registered by bgp, ipsec, l2tp, pppoe and radius from the same `Discover` call and checker map each runner uses. One resolution path; no hand list. S2 is required mechanically only for Kind `protocol`; for the other kinds the per-Kind S2 meaning is not an interop-lab scenario, so a feature that needs it carries it as an Extra criterion.
-> Decision (implementer, 2026-10-07, agent 2): RSVP-TE and flow-export interop are Go integration tests (`internal/plugins/rsvpte/freertr_interop_integration_linux_test.go`, `internal/plugins/flowexport/interop_integration_linux_test.go`) with no scenario directory, so a port onto `Discover` is not contained to registration. Not ported; those two features carry no Interop entry until the owner decides. No second resolver was written.
-> Decision (implementer, 2026-10-07, agent 2): D-8(b) staleness compares the Doc review date with the newest change to each Docs path AND the declaration file (a superset of "the description": any declaration edit owes a re-read). The change date is `git log -1 --format=%cs`, today for a dirty path; a shallow checkout is refused rather than answered (verify.yml already fetches full history).
-> Decision (implementer, 2026-10-07, agent 2): AC-8 reads journal rows through a new exported reader, `specjournal.HeadRows` (`internal/le/spec/journal/rows.go`), over the journal's own HEAD parser; a row names a Components path when its Surface, Symptom or Fix cell contains it.
-> Decision (implementer, 2026-10-07, agent 2): Extra criteria syntax is items separated by "; ", each `<level>: <criterion> = <evidence>`, evidence a pointer (Interop entry, test item, repo path) or `YYYY-MM-DD: judged`. An unresolved pointer leaves its gated level and every level above unmet.
-> Decision (implementer, 2026-10-07, agent 2): A-7 is answered by the suite table: a `.ci` must be `test/<suite>/<name>.ci` with `testfunctional.SuiteNamed(<suite>)` held, the mapping `internal/le/rfc` carriers use. `record-run` runs a `.ci` through `testfunctional.Prepare` plus `le test <suite args> <stem>` and a Go test through `go test -run ^Name$ -v`, and requires the item's own PASS line, not only exit 0.
-> Decision (owner reading of D-6, implemented 2026-10-07, agent 5): "tests exist and run green" covers Interop scenarios. Every counted Interop entry (non-stub, plus any Extra criteria pointer naming one) needs a current recorded green run: `features/runs/<id>.json` gains an optional `interop` list of `{scenario, scenario-tree, commit, date, result}`, `scenario-tree` being the scenario directory's git tree id computed over the working tree (`scenarioTreeID`, `internal/le/feature/scenariotree.go`; equals `git rev-parse HEAD:<dir>` when clean). Each lab's `interoplab.Catalog` declares `RunScenario`, which calls the lab's own `RunAt` with the scenario as selector (ipsec and radius `RunAt` gained the selector parameter l2tp already had). `record-run` observes the suite report (exactly one scenario, that name, passed, code 0) and refuses a directory changed mid-run.
-> Decision (implementer, 2026-10-07, Phase 4): AC-14's "non-child" is read under D-11 as "every declaration keeps its own row". A part of an umbrella keeps its row in its Kind's table, and the umbrella's row names each part with that part's status, so no description is dropped from `docs/features.md` (`RenderPage`, `pageRow`).
-> Decision (implementer, 2026-10-07, Phase 4): each site card names the declarations its bullets describe, judged card by card. A review of all 52 cards added `resolution-cli-and-pipes` to Output Formatting (its `ze pipe` bullet) and to DNS Resolver (its `| resolve` and `| origin` bullet). The Output Formatting bullet "Offline via ze format" was false, because `ze format` is no longer a root (`TestRootsRegistered`); it now reads "Offline via ze pipe" (D-8).
-> Decision (implementer, 2026-10-07, Phase 4): `features/development-activity.md` (Kind dev-tool) backs the Development Activity card, because D-9 refuses a card that no declaration backs. Its bullet "Built from git history each time" is true: `renderActivityPage` calls `MeasureActivity` on every site build. The "Live data" chip is open for the owner: the page is a static build, so the data is as of the last publish.
-> Decision (implementer, 2026-10-07, Phase 4): no feature is Supported yet, so every card renders experimental. A section that holds no card is not published (`featuresBody` in `datapages.go`, `writeLLMSFeatures` in `derived.go`), so the page does not show a core heading with nothing under it. The count is still the cards kept.
-> Decision (implementer, 2026-10-07, Phase 5): `docs/features.md` is untracked, so a tracked Markdown link to it 404s on GitHub. Links now point at the published page, `https://ze-software.net/reference/feature-status/`. A code-span mention of the path stays, which is how `docs/features/rfc-status.md` is cited.
-> Constraint (closure audit, 2026-10-09): closure is BLOCKED. (1) `fullStages` in `internal/le/verify/engine/stages.go` holds no `feature check` stage and `TestStagesIncludeFeatureCheck` does not exist, so the gate gates nothing; `docs/architecture/testing/verify-freshness-scope.md` names no such stage either. (2) `./le feature check` on the real tree exits non-zero: 58 Doc review staleness refusals (`staleDocReview`, `internal/le/feature/staleness.go`) because shared pages (`docs/guide/configuration.md`, `docs/guide/command-reference.md`, `docs/guide/config-reload.md`, ...) and 12 declarations changed on 2026-10-08/09 after the 2026-10-07 reviews, plus `backup-and-restore` declared supported above its ceiling (S5: journal `identity-default-hides-a-mapping` row 2026-10-08 names `internal/plugins/init`). Adding the stage as is reds every session's verify; a date-keyed review of a shared page re-fires on every edit to it (R-3 in S4 form). Owner question: which way does stale Doc review behave: refuse (redo ~52 reviews now and after each shared-page edit), or warn like run staleness (owner ruling 2026-10-08) and refuse only a recorded false sentence. (3) `TestMigrationDescriptionsVerbatim` (AC-20) does not exist. AC-24 surfaces now name `./le feature report` (2026-10-09).
-> Decision (owner, 2026-10-08/09, recorded as `plan/handover/2026-10-09-spec-triage-resume.md` gives it, answering the owner question above): each page section declares what it relies on: tests by content hash, the YANG resolved definition per schema path, and CLI commands. The commit that changes a dependency re-stamps or fixes the sections that cite it. A known-false sentence still refuses. The feature Doc review is derived. The spec is rewritten around this before more code.
-> Decision (implementer, 2026-10-07): observed red for the Phase 4 tests, each restored GREEN. CardShipped level check removed: TestASiteCardIsShippedOnlyWhenEveryFeatureItNamesIs RED. Unknown id skipped: TestAFactTheTreeCannotAnswerStopsTheBuild/a_feature_card_no_declaration_backs RED. Scope-gaps label changed: TestFeaturesPageRendersEveryDeclaration RED. feedsPage subdirectory check removed: TestTheDerivedFeaturePageFollowsTheDeclarations RED. place() inverted: TestFeaturesProducerDerivesCardState RED. Count read from the core section: TestFactsFeatureCountFromDeclarations RED.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The site count reads only `website/data/features.json`, never `docs/features.md` | `factsFromSiteData` in `internal/le/site/facts.go`; grep for `features.md` in Go finds only drift.go, render_ledger.go, docsmanifest.go | a third consumer keeps a hand status | grep at Phase 1 start | confirmed 2026-10-07: Go readers of `features.md` are drift.go, render_ledger.go, docsmanifest.go only |
| A-2 | A generated, untracked `docs/features.md` still publishes at `reference/feature-status` | `docs/features/rfc-status.md` is untracked and published the same way through the derived registry | the page vanishes from the site | site build test renders the page | unvalidated |
| A-3 | `interoplab.Discover` resolves scenario names for every suite (BGP, IPsec, L2TP, PPPoE, RADIUS, RSVP-TE) | `internal/le/interoplab/discover.go` | S2 needs a per-suite resolver | unit test per suite | broken 2026-10-07: `Discover` takes each suite's UNEXPORTED checker map and scenario root (callers in `interoplab/{bgp,ipsec,l2tp,pppoe,radius}`); `test/interop-rsvpte` and `test/interop-flowexport` do not go through `Discover` at all |
| A-4 | `internal/le/rfc` exposes enough to answer "open gap on an implemented MUST" without a second parse | `Meta`, coverage rows and gap rows used by `./le rfc check` | an export is added to `internal/le/rfc` | Phase 2 test | confirmed 2026-10-07: `rfc.Collect` answers `Metas` (`Enrolled()`, `Status`) and `Requirements` (`Annotation.Kind == AnnotationGap`, `IsGatedLevel`); no export added |
| A-5 | The audit's per-row results still hold at implementation time | audit dated 2026-10-07, no test run | levels set from stale findings | Phase 3 re-verifies each row at the producer before declaring | unvalidated |
| A-6 | Nothing outside the files listed parses the Status cell of `docs/features.md` | grep over Go, `.claude/`, `ai/`, `scripts/` | another reader breaks on the generated header | grep at Phase 1 | confirmed for parsers 2026-10-07 (no other Go reader). Prose instructing a hand edit also lives in `ai/skills/ze-doc-update.md`, `ai/rules/writing.md`, `ai/rules/points/writing/documentation/which-document-to-update-for-each-change.md`, `ai/INDEX.md`: Phase 5 rewrites them too, beyond AC-24's list |
| A-7 | The functional runner exposes its discovery so the check can ask whether a `.ci` path is discovered | the runner discovers `test/**/*.ci` by suite | the check only tests existence | Phase 2 | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Mass demotion: 66 Supported rows drop at once and the public page reads as a regression | report run in Phase 3 | expected and true; D-5 decides timing; the weekly update explains the method once |
| R-2 | Attestations become rubber stamps: a reader re-dates the review without re-reading | Defect review dates move without verdict changes | each attestation lists the items judged and their verdict; the check refuses a date-only attestation |
| R-3 | Staleness rule over-fires: any journal row naming a broad Components path (`internal/component/bgp`) forces constant re-review | report shows many "re-review owed" | Components name the narrowest package; umbrellas have no Components of their own |
| R-4 | `tmp/feature-maturity/` is deleted before Phase 3 | file missing | the seed table below carries line, name, claimed, earned and extra criteria; the per-criterion evidence cells are re-derived by the check |
| R-5 | Another session edits `docs/features.md` between now and Phase 3, so the migration drops a hand edit | `git log docs/features.md` after the audit date | Phase 3 starts by diffing the table against the seed and carrying any new row or prose |
| R-6 | Description prose moved verbatim carries `<!-- source: -->` anchors whose doc-to-code index (`ai/CODE-TO-DOCS.md`) changes owner from `docs/features.md` to `features/<id>.md` | `./le doc index check` diff | regenerate the index in the same change; anchors stay valid |
| R-7 | The site's 52 cards do not map cleanly to 131 declarations | cards with no matching declaration | a card names one or more ids; a card naming none is refused, so the gap is found at build |
| R-8 | `internal/le/site` features producer is owned by an in-progress spec (spec-site-renderers-in-go, closed 2026-10-08) | overlapping edits to `datapages.go` | Phase 4 starts after that spec closes, or coordinates through its owner |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | nothing in the daemon; the public feature page and site count could publish a wrong level or fail to build |
| How is it reverted? | single commit revert per phase; the hand table returns from history |
| Who else touches this path? | any session adding a feature row, the weekly update procedure, `internal/le/site` work (spec-site-renderers-in-go, closed 2026-10-08; its design lives in `website/AI.md`) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le feature check` | → | `internal/le/feature` check over `features/*.md` | `test/ui/le-feature-answers.ci` |
| `./le feature report` | → | report payload with ceiling and gaps | `test/ui/le-feature-answers.ci` |
| read of `docs/features.md` (derived artifact) | → | the feature renderer registered with `derived.Register` | `TestFeaturesPageRendersEveryDeclaration` in `internal/le/feature/render_test.go` |
| `./le verify` stage list | → | the `feature check` stage | `TestStagesIncludeFeatureCheck` in `internal/le/verify/engine/verifyengine_test.go` |
| site build features page | → | `shippedCards` deriving card state from declarations | `TestFeaturesProducerDerivesCardState` in `internal/le/site/datapages_test.go` |
| site facts | → | `factsFromSiteData` count from derived card state | `TestFactsFeatureCountFromDeclarations` in `internal/le/site/facts_test.go` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | a declaration missing a field its Kind and Scope require | `./le feature check` exits non-zero naming the file and the field |
| AC-2 | a Kind, Scope or Level value outside the vocabulary | refused, naming the value and listing the vocabulary from the one declaration of it |
| AC-3 | a Components, test, Docs or Page path that does not exist, or that escapes the repository | refused, naming the feature, field and path |
| AC-4 | an Interop entry whose name `interoplab.Discover` does not resolve | refused, naming the suite and name |
| AC-5 | an RFCs stem with no `rfc/short/<stem>.md` | refused |
| AC-6 | Level `supported` while a listed, counting stem (D-3) shows an open gap on an implemented MUST | refused; the message names the stem and the gap row ids |
| AC-7 | Level `supported` while a `plan/immediate/` spec names a Components path in its Files to Modify | refused; the message names the spec |
| AC-8 | Level `supported` while the newest journal row or immediate spec naming a Components path is newer than the Defect review date | refused as "re-review owed", naming the row |
| AC-9 | Level `supported` while every S1 and S2 item is marked stub | refused; Level `stub-backed` with no stub item is also refused |
| AC-10 | an umbrella whose Level or Scope is above the worst of its Parts | refused, naming the part that bounds it |
| AC-11 | an Extra criteria item gating the declared level whose pointer does not resolve or whose attestation is missing | refused |
| AC-12 | a declared Level below the computed ceiling | accepted; `./le feature report` lists it as a promotion candidate |
| AC-13 | `./le feature report` | one entry per declaration with id, name, kind, scope, level, ceiling, status label, and the unmet criteria for the next level each with its missing pointer; `| json` emits the same payload with kebab-case keys |
| AC-14 | a read of `docs/features.md` | the page is regenerated from the declarations: the vocabulary sentence comes from the vocabulary declaration, one row per non-child declaration, Status label per the Levels table, description verbatim; the file carries a generated banner |
| AC-15 | `git ls-files docs/features.md` after the change | empty; `.gitignore` names it; `derived.Register` owns it |
| AC-16 | the rendered Status of a partial-scope feature | carries "(partial)" and the row lists the Scope gaps (per the D-1 outcome) |
| AC-17 | `internal/le/doc/yangcontract/drift.go` after the change | holds no feature status vocabulary and no `checkFeaturesMD`; no other Go file holds a copy of the level list |
| AC-18 | a card in `website/data/features.json` | carries the ids it represents and no status; a card naming an unknown id, or only future or rejected features, fails the site build by name |
| AC-19 | site features page and facts | a card renders solid exactly when the D-9 rule holds for its ids; the published count equals the cards the derivation keeps; zero is still refused |
| AC-20 | the migration | one declaration exists for each of the rows present at Phase 3 start (131 on 2026-10-07); every description matches the row's prose byte for byte except the corrected rows |
| AC-21 | the two under-claim rows | Backup and restore declared at the level its recorded run supports; MOBIKE declared implemented with its Scope gaps, no longer Rejected |
| AC-22 | each false-prose row in the corrections table | its description no longer makes the claim the producer contradicts, verified at the producer and recorded in the row's Doc review |
| AC-23 | a new feature added as one `features/<id>.md` | appears in the check, the report and the rendered page with no Go edit |
| AC-24 | `plan/TEMPLATE.md` Documentation Update Checklist row 1, the `website/AI.md` weekly step, `ai/skills/ze-check.md`, `ai/skills/ze-close.md`, `ai/skills/ze-review-deep.md`, `ai/skills/ze-spec.md` | each names the declaration directory and `./le feature report`, not a hand edit to `docs/features.md` or a card status |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestParseDeclarationRefusesMissingField` | `internal/le/feature/declaration_test.go` | AC-1 | |
| `TestParseDeclarationRefusesUnknownVocabulary` | `internal/le/feature/declaration_test.go` | AC-2 | |
| `TestCheckRefusesMissingPath` | `internal/le/feature/check_test.go` | AC-3 | |
| `TestCheckRefusesUnresolvedScenario` | `internal/le/feature/check_test.go` | AC-4 | |
| `TestCheckRefusesUnknownRFCStem` | `internal/le/feature/check_test.go` | AC-5 | |
| `TestCheckRefusesSupportedWithRFCGap` | `internal/le/feature/check_test.go` | AC-6 | |
| `TestCheckRefusesSupportedWithImmediateSpec` | `internal/le/feature/check_test.go` | AC-7 | |
| `TestCheckRefusesStaleDefectReview` | `internal/le/feature/check_test.go` | AC-8 | |
| `TestCheckRefusesStubOnlySupported` | `internal/le/feature/check_test.go` | AC-9 | |
| `TestCheckRefusesUmbrellaAboveWorstPart` | `internal/le/feature/check_test.go` | AC-10 | |
| `TestCheckRefusesUnmetExtraCriterion` | `internal/le/feature/check_test.go` | AC-11 | |
| `TestReportListsUnderClaimAsCandidate` | `internal/le/feature/report_test.go` | AC-12, AC-13 | |
| `TestFeaturesPageRendersEveryDeclaration` | `internal/le/feature/render_test.go` | AC-14, AC-16, AC-23 | |
| `TestFeaturesProducerDerivesCardState` | `internal/le/site/datapages_test.go` | AC-18, AC-19 | |
| `TestFactsFeatureCountFromDeclarations` | `internal/le/site/facts_test.go` | AC-19 | |
| `TestStagesIncludeFeatureCheck` | `internal/le/verify/engine/verifyengine_test.go` | wiring | |
| `TestMigrationDescriptionsVerbatim` | `internal/le/feature/migration_test.go` | AC-20 | |

Each refusal test runs against a fixture declaration that passes, then breaks one
field, so each refusal has an observed red (vacuity guard).

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| attestation date versus newest judged input date | date order | equal dates | N/A | an input one day newer than the attestation is refused |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `le-feature-answers` | `test/ui/le-feature-answers.ci` | a developer runs `./le feature check`, `./le feature report` and `| json` and gets the refusal and report shapes | |

### Interop Tests (Scope: protocol)
N-A: tooling; no wire behavior changes.

## Files to Modify
- `.gitignore` - ignore the derived `docs/features.md`
- `docs/features.md` - removed from git; becomes a derived artifact (removal through `./le commit create ... remove`)
- `website/data/features.json` - each card names its declaration ids; `status` removed; core/experimental membership derived
- `internal/le/site/datapages.go` (Design: `website/AI.md`) - `featureCard` drops `status`, gains ids; `featureStatusLabels` deleted; `shippedCards` derives membership
- `internal/le/site/facts.go` (Design: `website/AI.md`) - count from derived membership
- `internal/le/site/derived.go`, `internal/le/site/home.go` (Design: `website/AI.md`) - read the derived count
- `internal/le/doc/yangcontract/drift.go` (Design: `docs/architecture/core-design.md`) - delete `featureStatuses` and `checkFeaturesMD`
- `internal/le/rfc/render_ledger.go` (Design: `docs/architecture/core-design.md`) - the `renderStatusPage` anchor names the feature vocabulary's new home
- `internal/le/verify/engine/stages.go` (Design: `docs/architecture/testing/verify-freshness-scope.md`) - add the feature check stage
- `website/AI.md` - the weekly "Check Features for drift" step and the `features.json` description
- `plan/TEMPLATE.md` - Documentation Update Checklist row 1
- `ai/skills/ze-check.md`, `ai/skills/ze-close.md`, `ai/skills/ze-review-deep.md`, `ai/skills/ze-spec.md` - name the declaration, not the hand table
- `docs/contributing/documentation-testing.md` - names `docs/features.md` as a checked doc
- `ai/INDEX.md` - a row for "declare or change a feature's maturity"
- `docs/architecture/core-design.md` - the le `feature` area beside `rfc`
- `docs/architecture/testing/verify-freshness-scope.md` - the new stage
- `docs/architecture/site-facts.md` - the feature count fact now derives from the declarations the cards name
- `docs/contributing/gh-pages.md` - card maturity is no longer authored in `features.json`

## Files to Create
- `features/<id>.md` - one declaration per current row (131 on 2026-10-07)
- `internal/le/feature/register.go` - `leroot.Register` and `derived.Register`
- `internal/le/feature/declaration.go`, `check.go`, `report.go`, `render.go` and their tests
- `docs/contributing/feature-maturity.md` - the model, the kinds, the bar, how to add or promote a feature; points at `./le feature report` for the live vocabulary
- `test/ui/le-feature-answers.ci` - functional test of the commands

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | N-A | host tooling; no daemon config or RPC |
| YANG validation constraints | N-A | no YANG |
| YANG custom validators | N-A | no YANG |
| CLI commands/flags | Yes | `./le feature check`, `report`, `index-update` through `leroot.Register` in `internal/le/feature/register.go` |
| CLI grammar (keyword before value) | Yes | `report feature <id>`, per `ai/rules/cli.md` |
| Editor autocomplete | N-A | le command, not the ze editor |
| Functional test for new RPC/API | Yes | `test/ui/le-feature-answers.ci` |
| Pipe completeness | Yes | `| json` on report and check, per `ai/rules/cli.md` |
| Env var registration | N-A | none added |
| Doctor check for runtime dependencies | N-A | no runtime dependency; host tool reading the tree |
| Prometheus counters/metrics | N-A | none |
| BGP family surface (new SAFI / capability / attribute) | N-A | none |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` becomes generated; its content moves to `features/<id>.md` |
| 2 | Config syntax changed? | No | none |
| 3 | CLI command added/changed? | No | ze CLI unchanged; le commands documented in `docs/contributing/feature-maturity.md` |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | none |
| 6 | Has a user guide page? | No | contributor page only: `docs/contributing/feature-maturity.md` |
| 7 | Wire format changed? | No | none |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | No | row 126's `rfc/short/rfc7947.md` correction is named, owed under rfc-compliance, not done here |
| 10 | Test infrastructure changed? | Yes | `docs/architecture/testing/verify-freshness-scope.md` (new stage) |
| 11 | Affects daemon comparison? | No | `docs/comparison.md` checked at Phase 4 for any maturity claim copied from the table |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` (le `feature` area) |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | No | no plugin inventory change |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | run `./le spec citation anchors spec plan/pre-release/spec-feature-maturity-declared.md`; declared Design docs: `website/AI.md`, `docs/architecture/core-design.md`, `docs/architecture/testing/verify-freshness-scope.md`. Mentioning docs: `docs/architecture/site-facts.md` (updated: the feature count's derivation moves to the declarations), `docs/contributing/gh-pages.md` (updated where it describes `features.json` card status); `docs/functional-tests.md` unaffected (it states the stage list lives only in `stagesForMode`); `docs/DESIGN.md` unaffected (mentions the stage file, not the stage population) |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `website/AI.md` weekly step, `plan/TEMPLATE.md` row 1, the four skills |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** - register the `feature` area and the derived artifact; empty declaration directory
   - Tests: `TestStagesIncludeFeatureCheck`, `le-feature-answers.ci` (fails: no declarations)
   - Files: `internal/le/feature/register.go`, `internal/le/verify/engine/stages.go`
   - Verify: the commands answer; the stage exists
2. **Phase: Declaration and check** - parser, vocabulary, evidence resolvers, ceiling, refusals
   - Tests: AC-1 to AC-12 unit tests, each with an observed red
   - Files: `declaration.go`, `check.go`, `report.go`
   - Verify: fixtures pass, each broken field is refused
3. **Phase: Migration** - one declaration per row; re-verify each row's audit finding at the producer; set Level, Scope, Kind; carry the corrections; seed Extra criteria from the table below
   - Tests: `TestMigrationDescriptionsVerbatim`; `./le feature check` passes on the real tree
   - Files: `features/*.md`
   - Verify: the declared levels agree with the seed's Earned column, or each disagreement is explained in the declaration's attestation
4. **Phase: Generation and site** - render `docs/features.md`; remove the hand file; derive card state and the count; delete the vocabulary copies
   - Tests: `TestFeaturesPageRendersEveryDeclaration`, `TestFeaturesProducerDerivesCardState`, `TestFactsFeatureCountFromDeclarations`
   - Files: `render.go`, `.gitignore`, `website/data/features.json`, `internal/le/site/*`, drift.go, render_ledger.go
   - Verify: site build renders `reference/feature-status` and the features page
5. **Phase: Docs and procedure** - contributing page, TEMPLATE row, skills, `website/AI.md`, INDEX, architecture pages
   - Tests: `./le doc index check`, doc link checks
   - Files: the docs and skills listed in Files to Modify
   - Verify: no surface instructs a hand edit of a status

Gates owed by the implementer (the main thread or a fresh agent runs them): `./le verify worktree`, the `test/ui` functional suite, the site build.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | the report, the page and the site all read the same parse |
| Correctness | no second parser of `rfc/short`; scenario names only through `interoplab.Discover` |
| Naming | report JSON keys kebab-case; level and scope values lowercase in declarations, labels rendered once |
| Data flow | `internal/le/site` holds no maturity vocabulary after Phase 4 |
| Rule: no-layering | hand `docs/features.md` and card `status` are gone, not kept beside the derived ones |
| Rule: evidence | every migrated level was re-verified at the producer, not copied from the audit |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| one declaration per row | `ls features/*.md` count equals the row count at Phase 3 start |
| hand table gone | `git ls-files docs/features.md` empty |
| one vocabulary | grep for the level words in `internal/le` finds them only in `internal/le/feature` |
| gate in verify | `TestStagesIncludeFeatureCheck` |
| report | `./le feature report` piped to `json` |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | declaration paths are repository-relative; a `..` component or an absolute path is refused, so the check never reads outside the tree |
| Public claims | no rendered row states a level the check did not accept |

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

- The audit's strongest structural finding: status words mix three facts (scope, evidence strength, evidence source). Declaring them separately removes the ordering argument.
- Journal Fix cells go stale; S5 becomes an attestation with a date that new inputs invalidate, so staleness is mechanical even where truth is not.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| One Markdown declaration per feature in `features/` | Go registration in the feature's package; one data file; Meta in docs pages | see the option table in Design: evidence rather than runtime, build-tag independence, no central list, the RFC ledger precedent |
| Level declared and capped by a computed ceiling | level fully computed from evidence | maturity is a release decision bounded above by evidence; the owner may hold a feature at Experimental for deployment reasons the bar cannot see; a computed level would also rise silently when a test path is added |
| Under-claim is reported, not refused | refuse any level below the ceiling | the ceiling is an upper bound only |
| `docs/features.md` becomes an untracked derived artifact | committed generated copy with a freshness gate | `internal/le/derived` forbids a committed copy of a derived file |
| Site cards keep their copy, lose their status | generate cards from declarations | cards are editorial site copy at a different granularity; only the maturity is a shared fact |

## Known Limitations

- The check proves evidence exists, not that it passes, until D-6 picks recorded runs.
- Prose truth stays a reader's attestation; the check enforces only that the attestation is current.

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
- [ ] Every user story has a working path and a passing test (N-A when Scope is tooling or docs, which delete that section)
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/pre-release/spec-feature-maturity-declared.md` only, in the same `./le commit create` script

## Seed Data

Source: `tmp/feature-maturity/BRIEF.md` (the bar) and the four batch files
`tmp/feature-maturity/bgp-routing.md`, `config-cli-platform.md`,
`diagnostics-api-security.md`, `ipsec-services-access.md` (per-criterion evidence
cells, over-claim lists, "Bar did not fit" sections). Those files are untracked;
this table carries the columns the migration needs. Line is the row's line in
`docs/features.md` on 2026-10-07; all 131 matched the table's name and Status on
that date. The audit ran no test; Earned is a claim to re-verify at the producer
in Phase 3.

| Line | Feature | Claimed | Earned (audit) | Extra criteria (audit) |
|------|---------|---------|----------------|------------------------|
| 9 | BGP Protocol | Supported | Partial | umbrella: worst status of component RFCs |
| 10 | Configuration | Partial | Partial | YANG type coverage |
| 11 | Persistent storage | Experimental | Experimental | .ci crash mid-publication then recovery |
| 12 | Backup and restore | Experimental | Supported (candidate) | none |
| 13 | Deactivate/Activate | Supported | Experimental | .ci deactivate/activate peer on running daemon keeps identity |
| 14 | Environment Variables | Supported | Experimental | none |
| 15 | Outbound source-address | Supported | Experimental | none |
| 16 | CLI Default Output Format | Supported | Supported | none |
| 17 | Interfaces | Experimental | Experimental | Privileged netlink evidence per link type |
| 18 | IPv6 Router Advertisements | Experimental | Experimental | none |
| 19 | Plugins | Partial | Partial | none |
| 20 | BMP Delivery | Supported | Experimental | producer not blocked while collector stalls |
| 21 | BFD | Partial | Partial | none |
| 22 | Kernel Tunable Management | Experimental | Experimental | none |
| 23 | Connection Tracking | Experimental | Experimental | none |
| 24 | Installation | Experimental | Experimental | PXE remote install proof |
| 25 | Modular Deployment | Partial | Partial | none |
| 26 | Static Routes | Supported | Experimental | none |
| 27 | Connected Routes | Supported | Experimental | none |
| 28 | Kernel Routes | Experimental | Experimental | none |
| 29 | Policy Routing | Experimental | Experimental | none |
| 30 | RPF Lookup | Supported | Experimental | none |
| 31 | Route Installation | Experimental | Experimental | kernel/VPP parity |
| 32 | Per-protocol FIB withholding | Experimental | Experimental | none |
| 33 | IS-IS | Experimental | Partial | none |
| 34 | OSPF | Experimental | Partial | none |
| 35 | MPLS/LDP/RSVP-TE | Experimental | Partial | cross-vendor RSVP-TE interop |
| 36 | VRRP | Experimental | Partial | none |
| 37 | Interactive Launcher | Supported | Experimental | none |
| 38 | CLI Commands | Supported | Partial | Gate: every declared command path has a handler |
| 39 | CLI Session Transcript | Supported | Experimental | none |
| 40 | API Commands | Supported | Partial | none |
| 41 | Configuration Reload | Partial | Partial | Privileged dataplane rollback evidence |
| 42 | Fleet Management | Experimental | Experimental | Two-daemon fetch/notify proof |
| 43 | Performance Benchmarking | Supported | Experimental | none |
| 44 | Web Interface | Supported | Supported | Close spec-web-testing-gaps rows |
| 45 | Looking Glass | Supported | Supported | Birdwatcher API consumed by real client |
| 46 | AI-First Design | Supported | Experimental | none |
| 47 | Self-Documenting System | Supported | Experimental | none |
| 48 | Host Inventory | Supported | Supported | none |
| 49 | Self-Update | Supported | Experimental | gokrazy A/B proof |
| 50 | Operational Report Bus | Supported | Partial | none |
| 51 | Local Audit Trail | Supported | Supported | .ci per surface (REST, gRPC, MCP, web) asserting an auth-fail row in show audit; resolve duplicate test/draft/plugin/audit-config-commit.ci |
| 52 | Health Registry | Supported | Partial | each registered check must be able to return non-healthy (a test that drives it red) |
| 53 | Storage SMART Management | Supported | Experimental | per-device-class (ATA, NVMe) proof; reload-updates-interval test at daemon level |
| 54 | Tech-Support Bundle (`ze support`) | Supported | Supported after one doc fix (Experimental strictly: S4) | archive JSON-per-module schema asserted by .ci |
| 55 | System Readiness (`ze doctor`) | Supported | Partial | every diagnostic code in internal/core/diagnostic/codes.go reached by at least one .ci |
| 56 | Granular Debug | Supported | Partial | prove "not auto-applied on reboot" with a restart .ci |
| 57 | Runtime Diagnostics | Supported | Experimental | MCP exposure asserted by a tools/list .ci naming these commands |
| 58 | Core Diagnostics | Supported | Partial | each listed command has a .ci (met except BFD) |
| 59 | Don't Fragment probes | Supported | Partial | IPv6 value <1280 reported as no usable value, proven at .ci |
| 60 | Path MTU diagnostic for IPsec tunnels | Supported | Supported | record a dated interop run of the three mtu scenarios |
| 61 | Protocol Event Capture and Replay | Supported | Supported | .ci for rotation at the cap and for the dropped-events counter |
| 62 | Packet Capture and Decode | Supported | Partial | pcap opened by tshark/tcpdump in an interop step |
| 63 | Memory Lock | Supported | Experimental | systemd unit LimitMEMLOCK checked by internal/plugins/systemd tests (present) |
| 64 | Plugin Setup Results | Supported | Supported | none |
| 65 | Crash Capture | Supported | Partial | full memory image lab on amd64 before claiming it |
| 66 | Interoperability Testing | Supported | Experimental (bar did not fit) | pass-rate published per suite |
| 67 | gNMI | Supported | Experimental | STREAM notification after a CLI commit proven at daemon level |
| 68 | REST/gRPC API | Partial | Partial | OpenAPI document validated against the served routes |
| 69 | Named Service Listeners | Supported | Partial | conflict check also runs in a BGP-less build (the daemon check lives in internal/component/bgp/config) |
| 70 | Management Listener Exposure Guard | Supported | Partial | none |
| 71 | Password Weakness Warning | Supported | Experimental | one .ci per surface (config set, editor commit, REST, passwd) |
| 72 | Live Credential Revocation | Supported | Supported | .ci for SSH password and pubkey revocation and gRPC Bearer |
| 73 | Custom Value Validators at Startup and Reload | Supported | Supported | none |
| 74 | MCP Integration | Supported | Experimental | revision conformance checklist per transport feature |
| 75 | Chaos MCP | Supported | Supported (bar did not fit: dev tool) | .ci for chaos_control and chaos_execute |
| 76 | Chaos MRT Recording | Supported | Experimental | strftime rotation proven |
| 77 | MRT Dump | Supported | Partial | RFC 8050 ADD-PATH snapshot Path ID exposure |
| 78 | PKI Certificate Store | Supported | Supported | intermediate-chain TLS listener check |
| 79 | IPsec Data Model | Supported | Partial | every enum value of an action leaf has a .ci proving distinct behavior |
| 80 | IKEv2 Wire Format | Supported | Supported | fuzz every payload decoder |
| 81 | IKEv2 Cryptographic Primitives | Supported | Supported | KATs for every DH/PRF/AEAD |
| 82 | IKEv2 Engine | Supported | Experimental | rekey collision via interop |
| 83 | IPsec EAP Authentication | Supported | Experimental | revocation proven by interop |
| 84 | IPsec NAT Traversal | Supported | Experimental | appliance-kernel proof |
| 85 | IPsec MOBIKE | Rejected | Experimental | appliance kernel XFRM_MIGRATE |
| 86 | XFRM Interfaces | Supported | Supported | none |
| 87 | IPsec Interop Testing | Supported | Experimental | scheduled workflow runs it |
| 88 | IPsec CLI and Diagnostics | Supported | Partial | none |
| 89 | DNS Resolver | Supported | Experimental | DNSSEC (blocked spec) |
| 90 | Resolution CLI and Pipes | Supported | Experimental | positive .ci for the origin pipe |
| 91 | ASN to RIR | Supported | Supported | none |
| 92 | Pipe Output Limiting | Supported | Supported | none |
| 93 | Plugin-Declared Pipe Aliases | Supported | Supported | none |
| 94 | Netdata-compatible OS Telemetry | Supported | Experimental | Grafana dashboard fixture |
| 95 | Flow Export | Experimental | Experimental | sampled-scale |
| 96 | Traffic Monitor | Experimental | Experimental | none |
| 97 | Traffic Feature Signals | Experimental | Experimental | none |
| 98 | Volumetric DDoS | Experimental | Experimental | flowspec mitigation interop |
| 99 | Behavioral Anomaly Detection | Experimental | Experimental | false-positive rate |
| 100 | Autonomous Anomaly Response | Experimental | Experimental | none |
| 101 | Traffic Usage | Experimental | Experimental | none |
| 102 | L2TPv2 BNG | Partial | Partial | none |
| 103 | TACACS+ AAA | Partial | Partial | PAP-only owner ruling |
| 104 | RADIUS admin AAA | Partial | Partial | none |
| 105 | PPPoE Access | Partial | Partial | none |
| 106 | Firewall | Experimental | Experimental | none |
| 107 | CoPP | Experimental | Experimental | non-TCP CoPP |
| 108 | VPP Firewall Backend | Stub-backed | Stub-backed | none |
| 109 | Commit-Time Backend Capability Check | Supported | Supported | none |
| 110 | Backend-Aware CLI Completion | Supported | Supported | none |
| 111 | Traffic Control Lifecycle | Experimental | Experimental | none |
| 112 | VPP Traffic Control Backend | Stub-backed | Stub-backed | none |
| 113 | RFC 1997 suppression | Supported | Supported | none |
| 114 | RFC 7999 BLACKHOLE | Supported | Supported | VPP discard evidence |
| 115 | Egress attribute conformance | Supported | Partial | per-rail .ci |
| 116 | Path Identifier regeneration | Supported | Experimental | none |
| 117 | Typed-family NLRI discard | Supported | Experimental | no third-party emits unknown types (see D-2) |
| 118 | Prefix-limit counting mode | Supported | Experimental | none |
| 119 | Dynamic peer groups | Supported | Experimental | none |
| 120 | AIGP (audit judged at HEAD) | Supported | Partial | none |
| 121 | PATHS-LIMIT | Supported | Supported | none |
| 122 | RPKI ASPA policy | Supported | Stub-backed | real RTR v2 cache |
| 123 | RPKI per-peer action | Supported | Experimental | none |
| 124 | IRR import filtering | Supported | Stub-backed | none |
| 125 | FlowSpec-to-firewall | Supported | Experimental | nft packet verification |
| 126 | IXP route server dynamic peers (audit judged at HEAD) | Supported | Experimental | per-peer community filter .ci |
| 127 | Subscriber session model | Supported | Experimental | PPPoE/L2TP parity test |
| 128 | Config Schema Stamp | Supported | Experimental | none |
| 129 | Config Dependency Graph | Supported | Supported | none |
| 130 | Graceful Listener Migration | Supported | Experimental | none |
| 131 | Docker Support | Supported | Experimental | Measured sizes |
| 132 | netlab device | Experimental | Experimental | none |
| 133 | Archive Pruning | Supported | Experimental | none |
| 134 | DHCP Server Named Ranges | Supported | Experimental | none |
| 135 | GeoDNS | Experimental | Experimental | none |
| 136 | Authoritative DNS Answer Policy | Supported | Experimental | none |
| 137 | AS112 Anycast DNS | Supported | Experimental | none |
| 138 | ExaBGP compatibility | Supported | Experimental | migrate round-trip on real configs |
| 139 | RFC conformance ledger | Supported | Experimental | tier reflects executed run |
