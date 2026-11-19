# Spec: RFC dependency graph and release layers

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | tooling |
| Priority | P2 |
| Release reason | Release planning aid: distinguish the obligations of a small enabled capability set from defects and advisory work in additional capabilities. The page does not change the shipped binary or certify a release. |
| RFC gaps | - |
| Depends | - |
| Phase | 4/4 -- presentation approved; scoped verification and publication |
| Handoff | - |
| Updated | 2026-11-15 |

## Task

Add a new website page showing the RFC dependency graph, release-layer filters,
derived proof figures, requirement levels and the specifications associated with
each RFC. Identify a small BGP capability boundary and the work needed to expand
it. Preserve unclassified work and uncertainty rather than treating missing
metadata as evidence that a spec is unrelated or belongs to the core.

This is a new page. The initial prototype preserved existing pages. Thomas
subsequently requested menu and related-page links so the map is connected to
the rest of the website. That navigation work and publication are now included;
existing page content and layouts remain outside scope.

## Owner Requirements

Thomas, 2026-11-14:

> a new page if you do it. Do not edit any page. Make it a new one.

> For every spec we probably want to tag the spec to an RFC so we know at which layer we need to be associated and therefore the level at which should be fixed.

> Also, a "may" or a "should" is not the same thing as a "must", so we may have effectively some fixes for "may" feature, and that would be interesting to know.

Presentation choice, Thomas, 2026-11-14:

> Dependency graph, with release-layer filters

Research decision, Thomas, 2026-11-15:

> Build the graph first; audit mappings in batches

Presentation approval and navigation follow-up, Thomas, 2026-11-15:

> Great

> where is /quality/rfc-map/ linked from ? menu ? pages ?

> I think it's an easier patch to see than the others, it should be well meshed.

The map's owning session handed its six source files and this record to the
homepage/navigation session for scoped verification and landing. The owner's
instruction to regenerate, commit and push the website authorizes publication
here. Ongoing RFC proof and product changes remain with the original session.

## Scope Decisions

| Subject | Current decision |
|---------|------------------|
| Opening view | Dependency graph; release layers are filters |
| Existing pages | Preserve content and layouts; add the approved menu and related-page connections |
| RFC membership | A citation is not an obligation to implement another feature |
| Requirement level | Separate MUST-family, SHOULD-family and MAY-family requirements; retain the exact keyword on each row |
| Optional capability | Mandatory requirements within a capability apply when that capability is enabled; optionality of the capability is a separate fact |
| Missing spec mapping | Visible as unclassified; no inference that it is platform work |
| Layer meaning | A capability boundary, not the graph's citation depth |
| Status figures | Proof, recorded unmet work and fresh source verification are distinct |
| Research gate | Approved 2026-11-15: build from the existing evidence first, then audit mappings in batches |

## Required Reading

| Source | Constraint |
|--------|------------|
| `docs/contributing/rfc-conformance-gates.md`, priority tree and coverage figures | Reuse the canonical figures and declared required/optional profile membership; no transitive enrollment from a citation |
| `docs/contributing/gh-pages.md`, RFC profiles and requirement ledger | Extend the registered page-producer mechanism and existing ledger; do not run conformance checks to render a page |
| `docs/contributing/spec-workflow.md`, metadata | `RFC gaps` already carries requirement IDs or summary stems; distinguish a whole-RFC relation from an exact requirement relation |
| `website/AI.md`, facts and roadmap | Figures remain source-bound; roadmap snapshots describe committed revisions, not arbitrary working-tree text |
| `ai/doors/system.md` and `ai/doors/le-tooling.md` | New tooling registers without adding a central dispatch branch; tests and builds run only in the main thread |

## Current Behavior

| Mechanism | Source and producer | Observed boundary |
|-----------|---------------------|-------------------|
| RFC graph and figures | `internal/le/site/rfcledger.go`, `requirementLedgerFrom` | Publishes typed graph edges, graph rows, profile trees, per-RFC figures and exact requirement levels |
| Website producers | `internal/le/site/producer.go`, `registerProducer` and `allProducers` | A new producer can register in a new file; ordinary producers run in registration order |
| Spec roadmap | `internal/le/site/roadmap.go`, `renderRoadmap` | Collects a pinned committed snapshot with explicit RFC gaps and spec dependencies |
| Public profile pages | `internal/le/site/rfcprofiles.go` | Existing dependency-depth groups are not a complete release-layer classification |

## Existing Mechanisms

| Mechanism | Where | Current purpose | Disposition |
|-----------|-------|-----------------|-------------|
| Requirement ledger | `internal/le/site/rfcledger.go`, `requirementLedgerFrom` | Single projection of the existing conformance model for the website | Extend by reading it; do not reconstruct proof from test tags in browser code |
| Page registration | `internal/le/site/producer.go`, `registerProducer` | Registers independent page producers | Extend with one new producer and new assets |
| Immutable roadmap collector | `internal/le/spec/roadmap`, `Collect` | Reads current specs from one committed revision | Reuse; stale seeded roadmap output is not a fresh input |
| Exact RFC relation | `internal/le/spec/status/specstatus.go`, `ResolveRFCGaps` | Resolves authored requirement IDs and summary stems | Reuse; incidental body mentions do not become confirmed associations |

## Research Findings to Preserve

| Finding | Consequence |
|---------|-------------|
| Most current specs have no declared RFC relation | A full inventory must include a visible unclassified queue; a mapping audit is work of its own |
| `Implementation: foundation` identifies informational guidance, not a product release layer | Never use that field as an onion-core assignment |
| Existing SHOULD proof sections combine SHOULD and MAY | Exact-keyword counts and requirement lists are available; separate proof percentages require a canonical model projection, not a browser approximation |
| RFC 4271 has recorded unmet and proof-pending requirements, with at least one stale gap description | Do not advertise base BGP as fully supported or interpret every recorded gap as a freshly verified defect |
| Advanced BGP capabilities share `ze_bgp` | A visual filter does not disable their code or configuration |
| Shared graph dependencies can have several parents and cycles | Count an RFC and a spec once; show all applicable relationships |

## Initial Association and Requirement-Level Census

The read-only inventory at committed `92a375243505126eee13cc6c3d1721a17e642543`
contains 678 specs. Of those, 25 declare nonempty `RFC gaps`; requirement gap
and partial annotations name 63 spec identities, with six also present in the
header set. The combined 82 associations leave 596 specs needing classification.
These counts describe that snapshot, not a permanent population or a release
verdict.

| Association | Presentation constraint |
|-------------|-------------------------|
| Exact requirement ID in `RFC gaps` | Show the exact normative keyword from the requirement |
| Whole RFC stem in `RFC gaps` | Normative level remains unassigned until the owned requirement is identified |
| Spec path in a requirement annotation | Label "Named in requirement gap"; association alone does not establish sole implementation ownership |
| Incidental prose mention | Candidate only, never a confirmed blocker |
| No reviewed association | Needs mapping; never silently assign platform/core |

Among the 25 explicitly tagged specs, 19 list mandatory IDs only, two list
SHOULD IDs only, two mix mandatory and SHOULD IDs, and two name whole RFCs.
The exact-ID headers contain no MAY reference. This characterizes the declared
relationships, not the whole scope or severity of each spec.

| Example | Declared relationship |
|---------|-----------------------|
| `spec-bmp-late-attach-and-forked.md` | Two SHOULD rows; the Task also describes enabled-feature defects |
| `spec-ospf-rfc1583-compatibility.md` | One SHOULD row |
| `spec-ipsec-spd-final-discard-vpp-ospf.md` | Two MUST rows and one SHOULD row |
| `spec-tftp-server-uploads.md` | Six MUST rows and one SHOULD row, on an additional capability |

These examples show why requirement level, feature inclusion and observed
correctness must remain independent fields.

## Risks and Assumptions

| ID | Risk | Required response |
|----|------|-------------------|
| R-1 | Percentages are mistaken for implementation correctness or release approval | Name the proof population and show recorded gaps and source-review limits separately |
| R-2 | RFC and roadmap inputs describe different revisions | Bind the input revisions or disclose and refuse an authoritative joined release verdict |
| R-3 | Missing metadata silently removes blockers | Preserve unmapped specs and unresolved associations in the page |
| R-4 | A provisional layer becomes a support promise | Label proposed boundaries; admission still requires an enforced capability boundary and candidate evidence |

## Design Alternatives

| Approach | Benefit | Cost and disposition |
|----------|---------|----------------------|
| Browser reads the full existing requirement ledger and roadmap | Smallest renderer and no new relationship artifact | The current requirement ledger is about 23 MB and contains evidence the opening graph does not need; rejected as the initial interaction path |
| New producer projects the recorded inputs into a compact page model; visible nodes read canonical RFC figure files | Keeps the opening graph small, preserves existing proof rules and can render an accessible fallback | Requires a projection and its provenance contract; recommended |

The projection adds a reader of existing published data, not another RFC
collector or conformance check. No third-party graph library is introduced.

## Proposed Page

| Area | Observable behavior |
|------|---------------------|
| Route | New `/quality/rfc-map/`, with a Markdown mirror |
| Opening view | Dependency graph focused on RFC 4271 and the proposed Base IPv4 BGP layer; nearby context is distinct from the selected capability set |
| Layer selector | Proposed capability groups, an All RFCs view and a visible unclassified group |
| Existing profile selector | Reads current required and optional membership from the recorded ledger; never changes profile declarations |
| Corpus navigation | Search reaches every recorded graph node, including documents without summaries; large branches expand on demand instead of drawing every label initially |
| Graph | One node per RFC identity, typed citation/update/obsolescence edges, shared dependencies and expandable cycles; selection never silently adds an RFC to scope |
| RFC inspector | Canonical mandatory proof percentage and segments, exact-keyword requirement filters, linked specs and links to existing evidence pages |
| Spec relations | Distinguish exact header requirements, whole-RFC header associations and citations inside requirement gap/partial annotations |
| Mapping queue | Every recorded spec appears once in the complete inventory; unmapped and explicitly no-gap-declared specs stay distinguishable and visible |
| Provenance | Show the roadmap revision and any actual evidence timestamp; the RFC ledger's absent revision and age are explicitly unknown, never replaced with current HEAD, projection time or filesystem mtime |
| Access | Keyboard-accessible selection, a readable list alternative, responsive layout and a static fallback when JavaScript is unavailable |

### Initial Proposed Layer Declaration

One new `website/data/rfc-release-layers.json` owns the provisional group
membership. It contains a schema version, the proposed flag, default group and
groups with stable IDs, names and RFC stems. It contains no proof counts,
percentages, graph edges or copies of existing profile declarations. Validate
stems against the recorded corpus and keep unassigned documents accessible.

| Group | Added RFCs |
|-------|------------|
| Base IPv4 BGP | rfc4271 |
| Modern IPv4 interoperability | rfc5492, rfc6286, rfc6608, rfc6793, rfc7606, rfc7607, rfc8212, rfc9687, rfc9774 |
| IPv6 unicast | rfc4760, rfc2545 |
| Common policy communities | rfc1997, rfc8092, rfc7999 |
| IXP route-server role | rfc7947, rfc7948, rfc7454, rfc9234 |
| RPKI validation | rfc6811, rfc8210, rfc8893, rfc9582 |
| Additional session operations | rfc2385, rfc5082, rfc2918, rfc7313, rfc4486, rfc8654, rfc9072, rfc8950, rfc8203, rfc9003, rfc7911 |
| Reflection and restart | rfc4456, rfc5065, rfc4724, rfc8538, rfc9494 |
| Extended policy attributes | rfc4360, rfc5701 |
| Monitoring and archival | rfc7854, rfc8671, rfc9069, rfc6396, rfc6397, rfc8050 |

These groups are planning proposals, not an independent statement of supported
features or an automatic cumulative release profile. All other technologies
remain available through the recorded technology aggregates and All RFCs.
Existing required and optional profile membership stays authoritative when
viewing an existing profile. Header-level update edges are context; exact
rewritten-row applicability remains linked to the existing requirement page
until its mapping is reviewed.

### Proof and Obligation Presentation

| Input | Display contract |
|-------|------------------|
| Walked RFC with mandatory obligations owed | Mandatory proof uses canonical Ze proven plus OS verified over mandatory kept; show the numerator and denominator |
| No summary or no valid extraction | Unknown, with the specific reason; no invented percentage |
| No mandatory rows | No MUST rows, not 100 percent |
| Every mandatory row validly excluded | No MUST obligations owed; keep the exclusion explanation |
| All mandatory rows unassessed | Zero proven with assessment unknown; never claim the implementation is absent |
| Keyword filter | Exact observed keywords and a Mandatory preset using the recorded gated flag; show listed-statement counts, not independently computed proof figures |
| SHOULD or MAY selected | Filter requirement lists and associated exact-RID relations; the full-scope mandatory proof card remains explicitly unchanged |
| Partial, lower-layer, stale, escaped or excluded statement | Preserve the canonical annotation and evidence link; no browser proof reclassification |
| Whole-RFC spec association | Normative scope unassigned; do not assign every MUST in the RFC to the spec |
| Multi-RFC spec | Show every association, with deduplicated overall spec accounting; no automatic fix-order verdict for unreviewed citations |

## Data Flow

### Entry Point

The named `rfc-map` site producer reads existing recorded data from the
canonical site artifact and the new layer declaration from source. The new
producer itself renders the new route and its own assets only, with no RFC
checks or test scans. The surrounding native action currently calls
`derived.EnsureAll`; missing derived artifacts can trigger source/test-tag
scans and repository artifact regeneration. Before invoking it, establish that
its derived prerequisites are already present. If they are missing, report
that preparation dependency rather than silently regenerating existing pages
or claiming the render performs no scans. The initial presentation render was
limited to this producer. The later approved navigation/publication follow-up
includes the normal complete website regeneration and check.

### Transformation Path

1. Read the requirement ledger and roadmap bytes once, retaining their hashes,
   recorded metadata and available revision fields. Validate required shapes
   before writing a page. Missing evidence age/revision stays unknown. A file
   timestamp, if shown, is labeled as a file timestamp; projection/render time
   never stands for proof freshness.
2. Index RFC identities and exact requirement IDs from the typed ledger.
   Preserve graph edge kinds, canonical cycle components and profile tiers.
3. Resolve each roadmap `RFC gaps` token as an exact known RID or RFC stem.
   Resolve spec filenames in typed gap/partial annotation reasons against the
   recorded roadmap population, preserving historical paths as provenance.
   Never infer a RID from spelling or promote incidental prose mentions.
4. Build deduplicated relations, visible diagnostics and the complete mapping
   queue. A citation is an association, not proof of sole ownership.
5. Validate proposed layer members and derive their graph slices. Shared RFCs
   and specs count once; related context does not enlarge the selected set.
6. Write the new HTML, Markdown mirror, compact relationship/topology model
   and scoped assets. Do not copy proof measurements into the page model.
7. The browser reads canonical `data/rfc/<stem>.json` for visible nodes and
   selected inspectors. Existing figure data remains the only measurement
   source. The page identifies graph/mapping snapshot provenance separately
   and never certifies a single verified release candidate from mixed inputs.

### New Page-Model Contract

Publish the compact projection at `data/rfc-map.json`. The browser also reads
the existing roadmap JSON for spec display fields, rather than publishing a
second copy of that inventory. A changed roadmap revision after projection is
reported as a stale relationship snapshot; missing/new specs remain visible,
with no claim that their relationship audit is current.

| Field | Type | Meaning |
|-------|------|---------|
| Schema version | Integer | Version one of the new page-only projection |
| Inputs | List of records | Input role, artifact path, hash of bytes read, available source revision and actual evidence timestamp; absent values stay unknown |
| Nodes | List of records | RFC stem, title, aggregate, summary/extraction availability and cycle component; no measurement counts or percentages |
| Edges | List of records | Existing graph from, kind and to, preserving direction and kind |
| Requirements | Map from stem to statement records | RID, exact keyword, gated flag, section, statement text and existing annotation needed for filtering/provenance; no locally computed proof verdict |
| Relations | List of records | Spec identity, RFC stem, optional RID, relation kind and source evidence, deduplicated by identity and provenance |
| Diagnostics | List of records | Unknown IDs, unresolved/ambiguous spec references and invalid associations, with source and actionable explanation |

The provisional layer file stays a separate authored source and is projected
as group IDs, names and member identities. Counts are derived from unique
identities in the browser, never independently authored. Existing roadmap and
RFC figure files keep their existing schemas.

### Changed-Wire Census

**Search evidence:** `website/AI.md` documents the existing ledger, roadmap and
per-RFC files. `rfcledger.go` `requirementLedgerFrom`, `roadmap.go`
`renderRoadmap`, `rfcdata.go` and the existing browser figure reader are the
positive writer/reader controls. This work adds consumers and a new page-only
model; it changes no existing data schema or protocol wire.

| Wire | Role | Carrier | Source evidence | Disposition |
|------|------|---------|-----------------|-------------|
| Existing requirement-ledger JSON | Writer | `rfcledger.go`, `requirementLedgerFrom` | Publishes Stems, Graph and Profiles, without a revision field | Preserve; disclose the absent revision |
| Existing requirement-ledger JSON | New reader | New `rfcmap_model.go` | Read the existing typed ledger and annotation reason | Add consumer, no alternate summary parser |
| Existing roadmap JSON | Writer | `roadmap.go`, `renderRoadmap` | Publishes committed snapshot revision, digest, items and explicit relations | Preserve |
| Existing roadmap JSON | New reader | New `rfcmap_model.go` | Read the recorded snapshot, not a current-HEAD label applied to old data | Add consumer |
| Canonical per-RFC figure JSON | Writer | `rfcdata.go`, existing RFC data publisher | Owns mandatory and advisory figure fields | Preserve |
| Canonical per-RFC figure JSON | New reader | New `rfc-map.js` | Reads visible-node figures and selected-node detail | Add consumer without independent proof rules |
| New provisional-layer JSON | Author/writer | New `website/data/rfc-release-layers.json`, reviewed source declaration | Owns only proposed group IDs, names, member stems and default | New page-only policy declaration; no copied proof measurements |
| New provisional-layer JSON | Reader | New `rfcmap_model.go`, schema/member validator | Loads proposed groups against the recorded corpus | Reject unknown schema, missing/duplicate group IDs, repeated member within a group, unknown default and unknown stem before publishing; preserve RFCs unassigned to a group |
| New compact graph/mapping JSON | Writer | New RFC-map producer | Topology, requirements, relations and input provenance only | New page-owned artifact |
| New compact graph/mapping JSON | Reader | New `rfc-map.js` | Navigation, list filtering and graph interaction | Validate schema and show actionable load errors |
| Existing roadmap JSON | New browser reader | New `rfc-map.js` | Spec inventory display fields and revision read from the canonical roadmap file | Preserve schema; disclose drift from the relationship projection rather than hiding new specs |

### Existing-Page Preservation

Use the existing named-producer build route over the canonical artifact. That
route is present in the shared working tree; its current installed dispatcher
can lag it, so use the named session build. The new producer publishes its own
assets because producer-only rendering skips normal asset staging. Snapshot
existing HTML, Markdown and shared-asset hashes before rendering and compare
them afterward. Never fall back to a whole-site build to obtain missing input
or producer records under this owner's no-existing-page-edit instruction.

### Integration Prerequisite

On 2026-11-15, `internal/le/site/buildproducer.go` and its action wiring are
uncommitted work from another session; installed HEAD rejects the producer
keyword. Their landing is an explicit prerequisite for any committed code or
workflow that depends on those new APIs. The page renderer should depend only
on the already committed producer/shell contracts where possible. A named
working-tree build can support local review while the helper is in flight,
after the derived-input preflight above. Do not carry another session's helper
implementation in the page commit or present it as delivered by this spec.

### Architectural Verification

| Check | Decision |
|-------|----------|
| Registration | New Go file registers one producer through `registerProducer`; no central dispatch or route list edit |
| Shared machinery | Reuse the page shell, typed ledger/roadmap models, canonical figure files and existing publication helpers |
| Product boundary | All implementation is website/tooling; no shipped daemon, wire behavior, config or SDK changes |
| Source authority | New layer declaration owns proposals only; existing summaries, profiles and spec metadata keep their current authority |
| Performance | Project the recorded ledger once; visible-node figure loading; no full-ledger browser startup payload or graph-library dependency |
| Existing pages | No source-page, navigation or shared-style edits; no regeneration of old page routes during review |

## Assumptions

| ID | Assumption | Basis | If wrong | Validation |
|----|------------|-------|----------|------------|
| A-1 | Canonical artifact has recorded RFC ledger, roadmap and figure files | Read in the 2026-11-15 research | Prototype cannot render useful evidence | Producer preflight names missing input before writing |
| A-2 | Producer-only rendering preserves existing pages when derived prerequisites exist | `BuildProducers` skips staging/native refresh and limits stamps to selected claims, but first calls `derived.EnsureAll` | Owner's no-existing-page-edit requirement is violated | Derived-input preflight, before/after byte comparison and preservation fixture |
| A-3 | Proposed BGP groups are useful first review filters | Owner approved graph first and batch audits; groups separate concrete capabilities | Filters mislead release planning | Clearly proposed labels and owner review of the prototype |
| A-4 | Current input schemas remain available during shared-tree work | Typed ledger and roadmap readers observed in research | Integration needs adaptation | Re-read current source before implementation; no copied schema |

## Additional Risks

| ID | Risk | Early signal | Mitigation |
|----|------|--------------|------------|
| R-5 | A large graph becomes unreadable | Overlapping labels or hundreds of nodes on initial load | Focused graph, branch expansion and list alternative |
| R-6 | A summary citation is mistaken for implementation ownership | Umbrella appears as a mandatory fix owner | Typed relation labels and visible provenance; audit before promotion |
| R-7 | Historical paths no longer resolve | Ambiguous or missing current spec identity | Visible diagnostics, no guessing; preserve original reference |
| R-8 | Producer-only infrastructure is unavailable or changes unrelated files | Preflight failure or an old-page hash changes | Stop the render and report exact changes; never restore another task's files |
| R-9 | A data-loading failure leaves a blank graph | JSON or asset fetch error | Retained static list, actionable error and useful links to existing evidence |
| R-10 | Per-RFC measurements refresh after topology/mapping inputs | Different data ages or an unresolved RID | Show separate provenance and no combined release-readiness verdict |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | Open the new route | Graph is the opening view, focused on base BGP, with release-layer controls | Existing page layouts remain because they are outside scope |
| AC-2 | Select any recorded RFC, including one without a summary | The node is reachable with useful title/context or explicit unknown status | Unassessed and text-only documents remain visible |
| AC-3 | Shared parents, cycle and typed-edge fixture | Every selected RFC and relationship remains navigable without duplicate accounting | Context neighbors remain distinct from selected scope |
| AC-4 | Choose a proposed layer or existing profile | Graph and list reflect exactly that declared set; optional/context membership is labeled | Unassigned RFCs remain reachable under All RFCs |
| AC-5 | Select walked, unwalked and zero-denominator RFCs | Canonical mandatory proof is shown where defined; unknown/no-obligation states are explicit | Advisory completion is not required for the mandatory card |
| AC-6 | Filter MUST, SHOULD, MAY or another observed keyword | Exact listed statements and associated RID relations change; card scope stays full mandatory scope | Broad RFC associations retain unassigned normative scope |
| AC-7 | Mixed requirement/spec relations and historical paths | Preserve every association and provenance, with each spec counted once overall | Citations remain associations until audit confirms ownership |
| AC-8 | Missing header, explicit no-gap field or unresolved relation | Every spec remains in the inventory with the appropriate mapping state/diagnostic | Unclassified work is permitted and clearly visible by owner decision |
| AC-9 | Recorded inputs have different or absent revisions/timestamps | Display separate input provenance, explicitly unknown evidence age where absent, and qualify the join; file/render times never become proof freshness | Planning navigation remains useful; no single-candidate readiness assertion is made |
| AC-10 | Named producer render | New page, mirror and scoped assets render in the canonical artifact; existing page bytes are unchanged | Existing unrelated shared-checkout edits remain untouched |
| AC-11 | Keyboard-only and narrow-screen navigation | Select a node, apply filters and read details through usable controls and a list alternative | Graph area may pan within its own viewport; the page body does not overflow |
| AC-12 | JavaScript disabled or input unavailable | Static navigation/evidence links or an actionable preflight/load error remain useful | Interactive graph is unavailable without JavaScript, stated plainly |
| AC-13 | Review versus closure | Render only the new page during review; run scoped tests and final checks after Thomas approves presentation | No early commit or test run over a moving presentation target |

## Wiring and Test Plan

| Entry point / question | Concrete test | When |
|------------------------|---------------|------|
| Producer registration to new route/mirror/assets | `TestRFCMapProducerPublishesNewPage` | After presentation approval |
| Cycles, shared parents, selected versus context set | `TestRFCMapGraphPreservesIdentityAndScope` | After presentation approval |
| Canonical proof edge cases and exact-keyword list counts | `TestRFCMapProofAndKeywordScopes` | After presentation approval |
| Header, annotation, mixed and absent associations | `TestRFCMapRelationsPreserveProvenance` | After presentation approval |
| Complete spec inventory and deduplication | `TestRFCMapAccountsForEverySpec` | After presentation approval |
| Input revision/age qualification and malformed-input error | `TestRFCMapInputProvenance` | After presentation approval |
| No existing page/shared asset bytes change | `TestRFCMapPreservesExistingPages` | After presentation approval |
| Actual generated page, filters, keyboard and responsive interaction | `TestRFCMapBrowserInteraction` | After presentation approval |

The feature is website tooling with no new operator RPC or protocol behavior;
browser coverage proves its user path. No `.ci` file or protocol interop test is
needed. Website review timing follows the owner's standing rule: render during
review, then add/run the scoped tests and commit once the presentation is
approved. Run no `./le verify` command.

## Files

| New file | Purpose |
|----------|---------|
| `internal/le/site/rfcmap.go` | Register/render the new page and publish its scoped assets |
| `internal/le/site/rfcmap_model.go` | Typed recorded-input projection, layer validation and spec relations |
| `internal/le/site/rfcmap_test.go` | Accounting, provenance, topology and preservation coverage |
| `internal/le/site/browsertest/rfcmap_test.go` | Actual-page interaction and responsive coverage |
| `website/data/rfc-release-layers.json` | Provisional capability-group declaration |
| `website/assets/rfc-map.css` | Styles scoped to the new page |
| `website/assets/js/rfc-map.js` | Graph, list, filtering, canonical figures and inspector |

No existing page, shared asset, RFC summary or spec metadata is changed as part
of the first page. The owning spec is the only existing file this work edits.

## Integration and Documentation Checklist

| Surface | Applies | Disposition |
|---------|---------|-------------|
| YANG schema and validation | No | Website-only tooling |
| Custom validators | Yes | New layer/input shape checks in `rfcmap_model.go` |
| CLI command and grammar | No | Existing producer selection action |
| Completion | No | Existing producer registry discovery |
| Functional user path | Yes | New browser test over generated route |
| Environment variables | No | Existing site/session controls |
| Doctor/runtime diagnostic code | No | No product runtime dependency; renderer/load errors name the data/asset problem |
| Metrics | No | No product behavior |
| Protocol/plugin/SDK surface | No | No protocol or product plugin change |
| Feature declaration | No | Developer planning page, not a daemon capability |
| Config syntax and guide | No | No config change |
| Command/API reference | No | No new command/API |
| Wire format or RFC implementation | No | New consumers of unchanged recorded formats |
| Test infrastructure | No | Existing unit/browser harnesses |
| Comparison and architecture pages | No | No existing contract changed |
| Route metadata and counters | No | No route processing |
| Website discovery and instructions | Yes | Registered route, static mirror, persistent return links, Evaluate menu entry and related-page connections in the quality, RFC and roadmap sidebars; documented in `website/AI.md` |
| Existing source anchors/examples | No | No existing behavior/page claim is altered |

### Documentation Update Checklist

| # | Category | Applies | File and change |
|---|----------|---------|-----------------|
| 1 | Feature declaration | No | This is a website planning view, not a daemon capability |
| 2 | User guide | No | No product workflow changes |
| 3 | Config syntax | No | No configuration changes |
| 4 | CLI reference | No | Existing producer selection action is reused |
| 5 | API/RPC documentation | No | No operator API change |
| 6 | Plugin guide | No | No plugin behavior changes |
| 7 | Wire format | No | Existing published schemas are read unchanged; the new page-only model is described on the new page and in this owning spec |
| 8 | SDK rules | No | No SDK change |
| 9 | RFC compliance | No | This work does not implement or alter an RFC requirement |
| 10 | Test infrastructure | No | Existing unit and browser harnesses suffice |
| 11 | Daemon comparison | No | No daemon capability changes |
| 12 | Architecture | No | Existing producer/recorded-data design is reused |
| 13 | Route metadata | No | No route processing |
| 14 | Prometheus counters | No | No runtime metric changes |
| 15 | Agent discovery | Yes | The new `rfc-map` producer self-registers, appears in existing producer discovery and publishes the new route; this spec owns implementation scope |
| 16 | Existing anchored source | No | New files only; no existing source anchor changes meaning |
| 17 | Existing examples | No | No existing syntax or page example changes |

## Implementation Steps

| Phase | Work and result |
|-------|-----------------|
| 1 | Verify current input and producer APIs, implement the new recorded-input projection and provisional layer declaration |
| 2 | Register the new route, scoped assets, accessible fallback and interactive graph/inspector |
| 3 | Render only `rfc-map` from the named working-tree build into the canonical artifact; compare all existing page hashes and let Thomas review |
| 4 | Apply presentation feedback by rendering only the new page; once approved, add/run scoped tests, final site checks and commit the owned files |

## Design Review Gate

The concern most likely to cause rework is treating graph citations or sparse
spec links as a complete release boundary. The design keeps proposal groups,
actual profile membership, mandatory proof and reviewed ownership separate.
The simplest alternative loads the entire ledger in the browser; the compact
projection avoids that cost while reusing the same canonical data. Native
SVG/DOM interaction avoids a new dependency. The prototype preserves existing
pages; the later approved navigation follow-up connects them to the map.

On 2026-11-15 Thomas requested the expected live local URL after the design
review. That direction authorizes implementation of the agreed graph-first
page. Build and render it for presentation review; tests and commits follow
the owner's website-review rule once he is happy with the page.

## Local Presentation Review

The new route is rendered into the canonical site artifact and served at
`http://127.0.0.1:8000/quality/rfc-map/`. The named producer build completed on
2026-11-15 and published only `rfc-map`. HTTP reads returned 200 for the page,
script, stylesheet, map projection, base-BGP figure file and roadmap input.
The projection carries 339 recorded graph nodes, ten proposed groups and 471
specification relations; those counts describe this recorded input, not a
release verdict.

SHA-256 comparisons covered 2,260 existing HTML, Markdown, CSS and JavaScript
files before and after the render: none changed or disappeared. The published
page-local script and stylesheet match their source bytes. A Node parser-only
syntax check of the new script exited zero. Browser-host access was unavailable,
so visual interaction is still for owner review; no browser-test success is
claimed for that initial review run. Thomas subsequently approved the page;
post-approval verification and publication are recorded below.

## Approved Navigation and Verification

The Evaluate menu links to the map under Evidence. Related-page links connect
the quality overview, testing health, RFC compliance, RFC details and profiles,
RFC status and roadmap pages. Persistent links on the map return to RFC
compliance, completion by use case and the roadmap after the interactive app
replaces its static fallback.

The scoped model tests passed for shared/cyclic graph identities, declared
scope, exact normative keywords, relationship provenance, ambiguous citations,
input hashes and malformed inputs. Producer tests passed for its exact outputs,
independent proof data, preserved existing pages and refusal before writes on
invalid inputs or missing assets.

The initial browser regression exposed loss of All RFCs scope after reload and
loss of keyboard focus after selecting a document. The script now records the
all-documents view in the URL and restores focus to the selected graph/list
control or inspector. The page and its Evaluate menu entry were also inspected
in Chrome through the owner's existing local server; the temporary review tab
was closed afterwards.

`TestRFCMapBrowserInteraction` now passes against the canonical artifact. It
covers reload, keyboard focus, declared scope, exact MUST/SHOULD/MAY filters,
phone layout, persistent navigation, absent summaries, unwalked requirements,
empty/all-excluded mandatory populations and a failed input with static links.
The scoped lint run also identified avoidable structure copies and fixture
path construction in the new map files; these were corrected. Its unrelated
site-package findings in existing rendering files remain outside this change.

Sources landed in `44d388c347` and the approved homepage wording in
`315fcc8f05`; both were pushed. Generated publication landed and was pushed as
Pages commit `1f2a063dfd`. That commit preserves the existing RFC ledger and all
canonical `data/rfc/` proof files. The map was projected with the native producer
from the previous committed Pages ledger (`7aa7fb54f8`, ledger SHA-256
`e553d532c752301c538ae27d921c1f453730f57de934b356602794932a950d5c`)
and the recorded committed roadmap. Draft proof records and test links were not
published. All selected input hashes were checked against the committed Pages
tree after publication.

The temporary input directory and publication selectors under
`tmp/session/2026-11-15-030618/scratch/` and the private `bin/le-home-friendly/`
build were removed after publication. The normal input paths in
`rfcmap_model.go` were restored by reversing only this session's temporary edit;
the source file matches the landed commit. The active preview ledger was never
overwritten.

Retained native evidence is limited to these successful scoped-run logs under
`tmp/session/2026-11-14-002623/scratch/`: `job-unit-pkg-f527b56a.log` (homepage
and navigation), `job-unit-pkg-9566f8a7.log` (all seven map model/publication
tests), and `job-unit-pkg-93c5c562.log` (browser interaction and edge states).
Remove these when formal closure has recorded equivalent CI evidence. This
record remains with the original map session for its formal lifecycle closure.

The working-tree generator initially failed before dispatch because the
existing `ai tokens report` menu summary exceeded its 60-character limit.
Removing the redundant word "the" in `internal/le/ai/tokens/run.go` repaired
that one-line blocker without changing the command's behavior or its other
uncommitted work. The subsequent named build and page render succeeded.
