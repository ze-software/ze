# Spec: release-documentation-data

| Field | Value |
|-------|-------|
| Status | ready |
| Scope | tooling |
| Priority | P2 |
| Release reason | Each release needs recoverable documentation and YANG data that later website builds cannot replace with current facts. |
| Depends | plan/release/delivery/spec-release-build.md |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-24 |

## Task

Create one immutable data bundle for each published release. Generate its documentation with the website renderer available at publication or regeneration time. Keep rolling documentation live, with no rolling bundle or retained snapshot.

This specification authorizes planning only. Implementation, publication, release creation, and website regeneration are not authorized by writing this file.

### Owner requirements

Thomas, 2026-10-24, verbatim:

> we should capture the data in a format which is not including any presentation or formatting, so that it can be formatted to look like whatever the website needs or requires at the time of the generation.

> Write a spec for this design, with immutable release data, no presentation in the bundle, and no snapshots for rolling documentation. Don't change the code yet.

These quotes govern the data contract below. Tests and implementation documentation must cite this section until closure moves the quotes to the owning architecture page.

### Scope

| Included | Boundary |
|----------|----------|
| Public documentation | Published guides and documentation pages, command and equivalent-command references, plugin references, configuration references, and the feature/RFC reference facts they use. Derive membership from the owning documentation contributors. |
| Source YANG | Every module and submodule in the documented release schema, including bootstrap modules and the resolved import/include dependency set. |
| Semantic dependencies | Cross-references, literal examples, data tables, citations, content assets and transcripts required to understand the included documents. |
| Website integration | Release selection, scoped search, links, Markdown exports, raw YANG downloads, clean rebuilds, and partial builds. |
| Release integration | Capture from the pinned release inputs and include the bundle in the existing release asset and checksum flow. |
| Excluded | Site templates, styling, scripts, page markup, marketing pages, news, presentation decks, source repositories, binaries, build caches, and rolling or nightly archives. A documentation link to a project-wide page stays an explicitly external-to-version link. |

## Required Reading

### Architecture Docs

- [ ] `docs/contributing/gh-pages.md` — current extraction and producer contracts.
  → Constraint: release replay must bypass `refreshNativeSurfaces` and report every generated route to producer coverage.
- [ ] `website/AI.md` — rendering, mirrors, source links and deployment boundaries.
  → Decision: both live collection and bundle decoding feed the same semantic data model before any presentation pass.
- [ ] `ai/doors/system.md` and `ai/doors/le-tooling.md` — ownership and compilation boundaries.
  → Constraint: product schema exports cannot import `internal/le`; development commands remain in the existing launcher registry.
- [ ] `plan/release/delivery/spec-release-build.md` — release identity, pinned inputs and signed checksums.
  → Constraint: reuse its release version and signing flow. Do not create a documentation version scheme or signing authority.
- [ ] `ai/rules/architecture.md`, `ai/rules/cli.md`, `ai/rules/repo-maintenance.md` and `ai/patterns/cli-command.md` — registration, command grammar and discovery.
  → Constraint: registered contributors own their semantic data. Rendering cannot discover missing release facts from current registries.

**Key insights:** a rendered page is not a data archive. A configuration tree is not the original YANG. A checksum inside an archive cannot authenticate the archive itself.

## Current Behavior

| Source read | Producing symbol | Constraint on this change |
|-------------|------------------|---------------------------|
| `internal/le/site/build.go` | `Build`, `refreshNativeSurfaces`, `publishCommandCatalog`, `publishPluginRegistry` | The build collects current facts before rendering. Commands and configuration come from a compiled daemon, while plugin inventory comes from the launcher process. Release capture must bind all three to the same commit and feature profile. |
| `internal/le/site/docs.go` | `docsSourceText`, `docsProducerPages`, `docsRenderer.renderSource` | Sources can be authored or generated. Rendering mixes token substitution, Markdown conversion, links, demos and layout. Split semantic collection before presentation. |
| `internal/le/site/docsmanifest.go` | `docsDestination`, `docsLinkManifest` | Current source-to-route mappings are website decisions. Preserve logical document and section identities without archiving these mappings as layout instructions. |
| `internal/le/site/config.go` | `renderConfiguration`, `configOwnershipData` | Reference data is already separate from page generation. YANG links currently use `main` and need release context. |
| `internal/le/site/search.go` | `pageSearchRecords`, `configSearchRecords` | Search reads page mirrors and the root configuration tree. Release indexes must read the selected release dataset. |
| `internal/le/site/producer.go` | `renderProducers`, `coverageOf` | A preserved output folder is not sufficient. Every release route needs a producer claim. |
| `internal/component/config/yang/cli/tree.go` | `walkYANGEntry` | The tree omits RPCs, notifications and list-key leaves. Do not reconstruct source modules from this projection. |
| `internal/component/config/yang/loader.go` | `sourcedModules.Parse`, `sourcedModules.source`, `Loader.LoadEmbedded`, `Loader.LoadRegistered` | The loader retains parsed source text and loads bootstrap modules separately. Imports read directly from disk can lack retained text and must be captured or refused. |

Preserve existing rolling destinations, complete prose and examples, command qualifiers, configuration ownership, Markdown exports, and useful behavior without browser scripts. Change only how documentation data is collected, retained and selected for rendering.

## Data Contract

### Bundle contents

The release asset is `ze-docs-<release>.tar.gz`. It contains UTF-8 JSON plus exact original YANG and necessary passive content assets. Archive member paths identify data, not website destinations.

| Member | Type | Content |
|--------|------|---------|
| `manifest.json` | Typed metadata | Format version, release version, full commit, commit timestamp, collector identity, documented feature/profile inventory, and ordered payload inventory. |
| `documents.json` | Semantic documents | Stable document IDs, titles, summaries, section IDs, content nodes, semantic relationships and source provenance. |
| `references.json` | Typed reference records | Commands, arguments, usage tokens, output contracts, pipes, aliases, equivalents, plugins, dependencies, configuration nodes and resolved reference facts. |
| `yang/index.json` | Module inventory | Module/submodule identity, namespace where applicable, revision or explicit absence, source digest, member path, profile membership, and resolved import/include edges. |
| `yang/objects/<digest>.yang` | Original bytes | Source text bound to the resolved module or submodule. Preserve comments, descriptions, whitespace and revision statements exactly. |
| `assets/<digest>` | Original content bytes | Only referenced content assets, with media type, semantic role, caption/alternative text and provenance in the document records. |

The manifest lists each payload member's path, byte length and SHA-256. It does not contain its own digest. The release checksum envelope binds the entire archive, including the manifest.

JSON keys use kebab-case. Each supported format has an explicit typed schema. Duplicate keys, undeclared fields, unknown node kinds and ambiguous identities are errors. No untyped extension map or raw-markup escape field is permitted.

### Semantic document nodes

| Node or value | Preserved meaning | Excluded presentation |
|---------------|-------------------|-----------------------|
| Section | Stable ID, title, parent and ordered children | Heading level tags, font size, page position |
| Paragraph | Ordered text and semantic references | HTML, Markdown delimiters, CSS classes |
| Sequence or list | Ordered/unordered meaning, items and explicit start value when meaningful | Bullet glyph, indentation, column layout |
| Table | Column identities and labels, typed cell values, row associations and meaningful order | Widths, alignment, colors, responsive rules |
| Example or literal | Exact text, language/content type and explanation | Syntax-highlighting markup or editor settings |
| Definition or notice | Term/meaning association or semantic notice kind | Card, badge, icon or color selection |
| Reference | Target document/section/entity ID, label, or an explicit external citation | Rendered anchor, site root, relative stylesheet or navigation URL |
| Asset | Content identity, caption, alternative text, transcript and provenance | Player implementation, poster layout, autoplay or display dimensions |

Semantic structure is data. It preserves the relationship between an instruction and its example without fixing a visual arrangement. Literal examples and raw YANG can contain markup characters as quoted data. Renderers must escape them, never execute them.

Parse authored Markdown into this model. Resolve generated facts into typed values, retaining units, qualifiers and provenance. Register semantic collectors for generated references instead of scraping HTML. A current source construct without a meaning-preserving conversion stops capture with its source and location. It must not be silently stripped, flattened, or stored as opaque HTML/Markdown. The same refusal applies to rolling collection after the common model replaces its old path.

The bundle contains no generated HTML, rendered Markdown, templates, scripts, stylesheets, CSS selectors, DOM attributes, colors, spacing, page categories used as colors, layout hints, navigation menus, search indexes, route destinations, or executable asset content. Current renderers own these outputs. Content media is allowed only as passive evidence or explanation, with all required bytes included and no mutable dependency.

### Identities and source completeness

Document and section IDs are permanent semantic identities. They do not depend on heading slug generation, presentation order or current source paths.

| Content owner | Persistent declaration | Initial migration |
|---------------|------------------------|-------------------|
| Authored document | A scalar `document-id` in the source's front matter | Assign the current public documentation identity once, then retain the value when the source or title changes. |
| Authored section | A `doc-section` comment directive immediately before its heading, carrying one section ID | Seed the ID from the current published anchor, including duplicate-heading suffixes. The collector consumes the directive as metadata and never stores its markup. |
| Generated document or section | An explicit document ID on the owning semantic contributor and its registered entity keys | Reuse current reference identities. A changed entity key means a different entity unless its owner explicitly declares identity continuity. |
| Existing rolling route or fragment | The current website route mapping and alias records | Preserve existing destinations and anchors during migration. Subsequent source moves retain redirects/aliases in website routing, outside the bundle. |

Migrate every included authored source and generated contributor in the same implementation. A source without its required declaration, a duplicate ID, or an unresolved reference refuses collection. The migration inventory is derived from contributors, not a separate hand-maintained page list. Tests rename and reorder source files, titles and sections while checking identity, old rolling links and release selector behavior.

Internal links store document, section or entity IDs. Source citations store repository identity, release commit and source path. External URLs remain citation data. The renderer builds URLs from these records and the selected version context.

Capture original YANG from the resolved loader's actual module set, including `ze-extensions`, `ze-types`, submodules and every import/include target. Identify distinct revisions separately. Do not use the plugin directory listing or the bare-name `sourceModules` enumeration as proof of completeness. If a resolved module has no exact source bytes, capture refuses the release.

One archive serves the release. If published build profiles differ, retain their distinct feature sets and record availability per reference/module. Identical source objects share one digest member. The collector cannot silently label the launcher's compile-time registrations as the shipped profile.

## Data Flow

### Entry Points

| Entry | Input | Result |
|-------|-------|--------|
| Planned `./le build release` | The release build's validated version, full commit and declared build profiles | Calls documentation capture before checksums and publication. A failed capture prevents release completion. |
| Proposed `./le site release-capture version <version> revision <full-commit> output <archive>` | A validated release identity and the release build's pinned source export | Creates the same bundle locally without signing, uploading, publishing or changing a Git branch. Rejects rolling/nightly identities. |
| Proposed `./le site release-check input <archive>` | A local bundle | Checks format, internal integrity, semantic completeness and identity. Reports provenance as unverified and makes no catalog change. |
| Proposed `./le site release-register input <archive> checksums <file> signature <file> asset <locator>` | A bundle, the release workflow's signed checksum envelope, and the published asset locator | Verifies the envelope with the release workflow's trusted public key, binds the archive digest and embedded release identity, then atomically adds an accepted catalog entry. |
| `./le site build` | Current sources plus the accepted release catalog and verified bundles | Renders rolling documentation and every accepted release. |
| `./le site check` | The generated site and the same catalog | Checks version coverage, mirrors, links and per-version data ownership. |

Capture resolves the full commit once. The release build supplies its clean source export and source-tree identity. Standalone capture creates that export through the same release helper. Dirty or unrelated working-tree files never enter it. Collectors and the inspected daemon must be built from the pinned export and declared profiles. All capture dependencies are local, pinned inputs. A missing generated input or recorded evidence is an error, not permission to query current sources or the network.

### Transformation Path

1. Registered documentation contributors collect a validated semantic dataset. Resolve authored content, generated facts, references, source modules and asset dependencies before rendering.
2. Rolling generation passes that dataset directly to the renderer in memory. It neither invokes the bundle writer nor serializes a complete dataset to a temporary file.
3. Release capture validates identity and completeness, then encodes the dataset and original objects into a deterministic bundle.
4. The release workflow signs its checksum envelope and publishes the bundle with the other release assets. It invokes `release-register` only after publication succeeds. The action authenticates the envelope before adding the catalog entry. It uses the trusted key configuration owned by the release workflow, never a key supplied inside the bundle or alongside candidate data. The asset locator must name the expected repository/release asset and return the authenticated bytes. Missing authentication or an unavailable asset refuses registration without a catalog change.
5. The website acquires and verifies every cataloged bundle before touching the published output. A bundle reader returns the same semantic dataset type as live collection.
6. The current renderer produces pages, Markdown exports, navigation, scoped search and downloads from the selected dataset. Release rendering cannot call live collectors.

### Boundaries and Integration

| Boundary | Contract | Verification |
|----------|----------|--------------|
| Product schema to tooling | Exact resolved module source export owned by config/YANG, without importing `internal/le` | `TestDocumentationYANGClosure` |
| Source collection to renderer | Typed semantic dataset, no presentation fields | `TestDocumentationDataHasNoPresentation` |
| Release workflow to storage | Deterministic bytes plus externally authenticated archive digest | `TestReleaseDocumentationCaptureDeterministic` |
| Bundle reader to renderer | Format-aware, validated data only, no live-source access | `TestReleaseDocumentationRebuildWithoutSources` |
| Producer to site checker | Every versioned route and named download accounted for | `TestReleaseDocumentationCoverage` |

### Architectural Verification

| Check | Holds? | Design evidence |
|-------|--------|-----------------|
| No bypassed layers | Yes, required | Release and rolling share semantic rendering. Release decoding never enters `refreshNativeSurfaces`. |
| No unintended coupling | Yes, required | Source export stays in config/YANG. Bundle and presentation logic stay in tooling. |
| No duplicated functionality | Yes, required | Refactor existing documentation/reference producers. Do not keep an alternative legacy renderer for rolling. |
| Zero-copy constraints | N-A | Cold documentation generation does not change packet or runtime configuration paths. |
| Outbound registration | Yes, required | Register capture/check actions and the release producer with existing launcher/site registries. |
| Inbound registration | Yes, required | Derive document membership from contributors and release membership from the catalog. No release-name switches or copied plugin/module lists. |

## Immutability, Compatibility and Storage

| Concern | Required behavior |
|---------|-------------------|
| Deterministic encoding | Canonical JSON field/key order, stable record order, sorted archive members, fixed archive metadata and gzip headers. Use the release commit time, never the wall clock. The same pinned inputs and collector/toolchain produce identical bytes. |
| Existing release | Same identity and same digest is an idempotent success. Same identity with different bytes is refused, including concurrent attempts. There is no overwrite or force option. |
| Publication | Validate and write to task-owned temporary output, then install with a no-replace operation. An interruption cannot leave an accepted partial bundle. |
| Trust | Verify the expected digest against the release signing flow before accepting the catalog entry. An archive's self-reported hash is insufficient. Local format checking does not claim authenticated provenance. |
| Catalog | Keep one small catalog under `website/data/documentation-releases.json`. Each entry holds release identity, immutable asset locator and authenticated digest. Only `release-register` creates accepted entries. It contains no document content or routes. Empty is valid before the first release. |
| Retention | Keep one canonical archive per release with the release assets. The repository stores only the catalog. Do not retain unpacked source copies, alternative exports or rendered pages as archive inputs. Generated published pages are normal website output. |
| Rolling | No archive, manifest inventory, full-dataset JSON file, dated copy, historical entry or retained capture. Existing published reference downloads remain generated outputs, not a new rolling archive. |
| Reader compatibility | Dispatch by bundle format version. Retain a reader for every published format. Convert older data in memory if needed, preserving semantics and original hashes. Never rewrite a released archive. Unknown or lossy conversion fails explicitly. |
| Corrections | A released bundle stays unchanged. A correction requires a new release identity through the release workflow. No silent historical edit. |
| Missing input | Missing, corrupt, mismatched or unsupported bundles prevent a complete site build. Never skip a cataloged release, substitute rolling data or rely on seeded output folders. |

Archive readers reject absolute paths, parent traversal, duplicate member names, symbolic/hard links, unexpected members, wrong sizes and digests. Read validated members directly where possible. No archive entry can write outside task-owned temporary storage. Enforce documented bounds on member count, expanded bytes and semantic depth before allocation, including overflow and decompression abuse. Derive sufficient limits from the measured corpus during implementation and pin them with boundary tests before capture can ship.

Task-owned staging of generated website output for safe publication is permitted and is removed when the operation ends. It is not a serialized documentation dataset, retained rolling snapshot or historical source.

## Website Behavior

Keep existing rolling destinations. Publish a release under `/versions/<release>/`, with document/entity URLs derived from permanent IDs. Keep versioned routes and section anchors stable across layout changes. Route changes require redirects in website code, not edits to archives.

The version selector preserves the document ID when it exists in the target dataset. Otherwise it shows that the document is unavailable for that version and links to its version index. It never substitutes rolling content. Navigation, search, mirrors, canonical URLs, downloads and source links all carry the selected release. Source citations use its full commit, and YANG downloads return its original bytes.

A complete or partial site build regenerates every accepted release through a registered producer. A new empty output directory must work without a previous site, old repository checkout or old executable. Live site-wide news and branding can change. Release facts, prose, examples, reference records and schema bytes cannot change with them.

## Risks & Assumptions

### Assumptions

| ID | Assumption | Basis | If wrong | Validated by | Status |
|----|------------|-------|----------|--------------|--------|
| A-1 | Shared semantic data can represent all included documentation without loss. | Owner requirements and current mixed Markdown/reference producers | Some content cannot be archived faithfully | Corpus inventory plus `TestDocumentationSemanticCorpus` before replacing rolling rendering | To validate during implementation |
| A-2 | Release collection can use the same pinned inputs as binary packaging. | `spec-release-build.md` | Reference data could describe a different binary | Capture identity mismatch fixtures and release wiring test | To validate during implementation |
| A-3 | The exact resolved YANG source set can be exported. | Loader source tracking, with missing-text cases identified above | Incomplete source download | Bootstrap/import/include/revision fixtures and a real profile capture | To validate during implementation |

### Risks

| ID | Risk | Early signal | Mitigation |
|----|------|--------------|------------|
| R-1 | Formatting survives as an opaque string or metadata field. | Raw markup node, renderer option or arbitrary map in decoded data | Typed schema, forbidden-field tests and two independent renderers. Literal examples remain escaped data. |
| R-2 | Conversion loses ordering, emphasis with meaning, qualifiers or table relationships. | Semantic corpus comparison differs | Encode the meaning explicitly or refuse the construct. Never silently simplify it. |
| R-3 | A release acquires current facts through a hidden collector. | Source mutation changes rendered release facts | Deny live collection in release rendering and test with current sources absent. |
| R-4 | Schema capture misses imported, bootstrap or older-revision source. | Unresolved edge or missing source digest | Enumerate the actual resolved module objects and verify every dependency edge. |
| R-5 | Cleanup removes old releases or search mixes versions. | Missing producer claims or cross-version search result | Empty-output and partial-build tests with two releases and rolling. |
| R-6 | A retry overwrites a release or exposes partial output. | Identity collision, race or injected write failure | Atomic no-replace commit, idempotent hash comparison and failure tests. |
| R-7 | Archive decoding consumes unbounded resources or writes outside its root. | Oversized member, traversal or arithmetic overflow | Bounded decoding and negative archive fixtures. |
| R-8 | Assets refer to mutable remote bytes or carry active presentation. | Missing content digest or executable content | Collect the dependency set, validate passive content roles and refuse unresolved assets. |
| R-9 | Future readers reinterpret old data. | Old fixture loses semantics under a new renderer/reader | Keep published format fixtures and compare semantic answers, not HTML bytes. |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Operators can read instructions or schemas for the wrong release. Rolling documentation can lose content during collection refactoring. |
| How is it repaired? | Correct the collector/renderer with a forward change. Preserve published bundle bytes and release identity. Restore generated output from verified bundles. |
| Who else touches this path? | Website contributors, config/YANG tooling, and the release-build and release-distribution specifications. No unrelated launcher changes belong here. |

## Wiring Test

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `le site release-capture` | → | Registered capture action and semantic collector | `release-documentation-capture.ci` |
| `le site release-check` | → | Registered validation action and bundle reader | `release-documentation-invalid.ci` |
| `le site release-register` | → | Authenticated release metadata and atomic catalog update | `release-documentation-registration.ci` |
| `le build release` | → | Capture before checksum/signing completion | `release-documentation-release-wiring.ci` |
| `le site build` and `le site check` | → | Live dataset plus registered release producer and coverage | `release-documentation-rebuild.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Capture a pinned release with documented profiles | One bundle identifies the exact release, commit and profiles. Commands, plugins and YANG agree with those inputs. |
| AC-2 | Inspect every typed field and archive member | Only semantic content, provenance, integrity metadata, original YANG and passive content assets exist. No presentation payload or escape field exists. |
| AC-3 | Capture the current included documentation corpus | All contributors, prose, examples, relationships, semantic order and qualifiers survive. Unsupported conversion refuses capture by source location. |
| AC-4 | Build rolling documentation repeatedly, including failure paths | Bundle writer is never called. No rolling manifest, complete-dataset serialization, archive, catalog entry or retained snapshot is created. |
| AC-5 | Decode one bundle into two independently implemented layout/export renderers | Both expose the same semantic documents, reference records and YANG, with different presentation and without changing bundle bytes. |
| AC-6 | Remove original sources/executables and build into an empty destination | Every cataloged release, mirror, search index and YANG download is regenerated from verified bundles plus current renderer assets. |
| AC-7 | Change current prose, command registrations, schema, facts, renderer and CSS | Rolling reflects source changes. Release semantic content and raw YANG hashes remain unchanged. Layout changes are permitted. |
| AC-8 | Traverse internal links, fragments, selector, search and source citations across two releases and rolling | Each resolves within its selected version or explicitly reports absence. Source links pin the release commit. |
| AC-9 | Capture modules with bootstrap, import/include, submodule and multiple-revision dependencies | Every resolved source is present byte-exactly with unambiguous identity. Missing source or dependency refuses capture. |
| AC-10 | Repeat capture with fixed inputs but different directory, clock, file order and host metadata | Archive bytes and digest are identical with the pinned collector/toolchain. |
| AC-11 | Retry, race, interrupt, or attempt to replace an accepted release | Identical retry succeeds. Conflicting data refuses without mutation. No accepted partial output survives. |
| AC-12 | Supply wrong identity, signature binding, digest, format, field, node, size or archive path | Validation refuses with a named cause. No live fallback, skipped release or output escape occurs. |
| AC-13 | Full and partial site builds with two accepted releases | Both retain complete release coverage and version-scoped indexes, independently of previous output. |
| AC-14 | Build and publish through the release workflow | Capture is required before checksum/signing completion. The signed envelope covers the bundle. No catalog entry becomes accepted before verification. |
| AC-15 | Read an older supported format or an unknown format | Supported data retains its semantics through in-memory adaptation. Unknown/lossy conversion refuses. Original archives remain byte-identical. |
| AC-16 | Complete or fail capture/rebuild | Remove task-owned temporary exports and unpacked data. Retain only release assets, the catalog and normal generated website output. |
| AC-17 | Register with missing/invalid signatures, an untrusted key, wrong digest, wrong release locator, duplicate identity or a concurrent writer | Refuse without changing accepted entries. Authenticated identical retry succeeds. Local `release-check` alone can never register a release. |
| AC-18 | Rename/reorder authored source files, document titles and section headings after ID migration | Persistent document/section IDs remain unchanged. Existing rolling links and fragments resolve, and release selection uses the same identity. Missing or duplicate declarations refuse collection. |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestDocumentationDataHasNoPresentation` | `internal/le/site/docdata_test.go` | AC-2, typed nodes and forbidden payloads, literal escaping | Planned |
| `TestDocumentationSemanticCorpus` | `internal/le/site/doccollect_test.go` | AC-3, stable IDs, complete contribution coverage and refusal on unsupported content | Planned |
| `TestDocumentationPersistentIDs` | `internal/le/site/doccollect_test.go` | AC-18, declaration migration, rename/reorder preservation and rolling aliases | Planned |
| `TestRollingDocumentationDoesNotSnapshot` | `internal/le/site/doccollect_test.go` | AC-4, instrument archive/serialization calls and filesystem writes on success/failure | Planned |
| `TestDocumentationIndependentRenderers` | `internal/le/site/docrender_test.go` | AC-5, independent semantic oracle across website and plain export | Planned |
| `TestDocumentationYANGClosure` | `internal/component/config/yang/source_export_test.go` | AC-9, all resolved source identities and exact bytes | Planned |
| `TestReleaseDocumentationCaptureDeterministic` | `internal/le/site/docbundle_test.go` | AC-1 and AC-10 | Planned |
| `TestReleaseDocumentationImmutablePublication` | `internal/le/site/docbundle_test.go` | AC-11, concurrent collision and interruption | Planned |
| `TestReleaseDocumentationRejectsInvalidBundle` | `internal/le/site/docbundle_test.go` | AC-12 and bounded decoding | Planned |
| `TestReleaseDocumentationFormatCompatibility` | `internal/le/site/docbundle_test.go` | AC-15 with retained published-format fixtures | Planned |
| `TestReleaseDocumentationRebuildWithoutSources` | `internal/le/site/docrelease_test.go` | AC-6 and AC-7 with source/daemon access made fatal | Planned |
| `TestReleaseDocumentationVersionLinks` | `internal/le/site/docrelease_test.go` | AC-8, missing-page behavior and original YANG downloads | Planned |
| `TestReleaseDocumentationCoverage` | `internal/le/site/docrelease_test.go` | AC-13, empty and partial output | Planned |
| `TestReleaseDocumentationCleanup` | `internal/le/site/docbundle_test.go` | AC-16, success and injected failures | Planned |
| `TestReleaseDocumentationCatalogAuthentication` | `internal/le/site/docrelease_test.go` | AC-17, trusted envelope, locator/digest binding and atomic catalog admission | Planned |

### Boundary Tests

| Field | Last valid | Invalid boundary | Required proof |
|-------|------------|------------------|----------------|
| Format version | Every published supported value | Zero, negative, unknown, fractional or overflowing value | Refuse before rendering |
| Member length/count and expanded total | Each documented limit | Limit plus one, declared/actual mismatch and overflow | Bounded refusal with no partial publication |
| Node depth | Documented maximum | One level deeper | Refuse before recursive allocation |
| Logical IDs and paths | Unique canonical identity | Empty, duplicate, ambiguous revision, traversal or alias collision | No overwrite or cross-version resolution |

Numeric limits are implementation constants justified by the corpus measurement, not user configuration. Record their exact values in the architecture page and fixtures before closing this spec.

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `release-documentation-capture` | `test/parse/release-documentation-capture.ci` | Invoke capture and check through real registered commands on a controlled pinned release fixture. Verify semantic data and raw YANG. | Planned |
| `release-documentation-invalid` | `test/parse/release-documentation-invalid.ci` | Reject a tampered bundle and conflicting release identity without mutation. | Planned |
| `release-documentation-registration` | `test/parse/release-documentation-registration.ci` | Invoke structural checking and authenticated registration separately. Reject each missing/bad trust input, then admit one verified release without replacing existing entries. | Planned |
| `release-documentation-rebuild` | `test/parse/release-documentation-rebuild.ci` | Generate two releases plus rolling, remove prior output, rebuild, change current inputs, and verify version isolation and no rolling snapshots. | Planned |
| `release-documentation-release-wiring` | `test/parse/release-documentation-release-wiring.ci` | Invoke release preparation with transport/signing fixtures. Prove a failed capture blocks completion and successful checksums cover the archive. | Planned |

Interop tests are N-A: no network protocol behavior changes. Browser checks cover version selection, search, direct fragments, keyboard navigation and no-script readability. Run only reached package tests and these named functional cases. Full release verification is owner-run under `ai/META-DECISIONS.md`.

## Files to Modify

- `internal/le/site/actions.go` — register capture, checking and authenticated catalog admission.
- `internal/le/site/build.go` — separate collection/rendering and preflight release bundles before output mutation.
- `internal/le/site/producer.go` — register release production and complete route/data ownership.
- `internal/le/site/paths.go` — carry selected-version context without confusing site and version roots.
- `internal/le/site/docs.go` — collect semantic documents and render the common dataset.
- `internal/le/site/docsmanifest.go` — stable identities and rolling route aliases.
- `internal/le/site/markdown.go` — parse source meaning and explicit section IDs before presentation.
- `internal/le/site/doctransform.go` — resolve semantic references within selected versions.
- `internal/le/site/docpage.go` — separate semantic metadata and assets from display choices.
- `internal/le/site/commands.go` — render selected command records.
- `internal/le/site/equivalents.go` — collect and render semantic command equivalence data.
- `internal/le/site/equivalentdetail.go` — render selected command detail without current-source reads.
- `internal/le/site/plugins.go` — collect and render selected plugin records.
- `internal/le/site/config.go` — selected configuration data and original YANG downloads.
- `internal/le/site/catalog.go` — typed command data shared by live collection and bundle reading.
- `internal/le/site/yang.go` — bind extraction to pinned release profile identities.
- `internal/le/site/derived.go` — machine-reader exports from selected data.
- `internal/le/site/datapages.go` — capture semantic facts required by documentation.
- `internal/le/site/search.go` — scoped search generated from each dataset.
- `internal/le/site/shell.go` — current layout with selected-version navigation.
- `internal/le/site/pages.go` — release routes, mirrors and coverage.
- `internal/le/site/footer.go` — distinguish release identity from website generation time.
- `internal/le/site/demo.go` — content media/transcripts without player presentation in the bundle.
- `website/assets/js/site.js` — version selection, scoped search and commit-pinned source links.
- `website/data/frontend-vocab.json` — separate current and release citation context.
- `internal/component/config/yang/loader.go` — expose resolved source objects without changing schema validation.
- `docs/architecture/config/yang-config-design.md` — document the exact resolved-source export and missing-source refusal.
- `docs/contributing/gh-pages.md` — capture, replay, storage and release publication workflow.
- `website/AI.md` — common semantic rendering, persistent IDs and selected-version behavior.
- `ai/INDEX.md` — discovery for the new actions and architecture contract.

Also migrate every authored source returned by the documentation contributor inventory: add `document-id` front matter and `doc-section` directives while preserving prose and rolling anchors. Generate the exact population before migration and verify complete coverage. Extend the planned `internal/le/build/release/` implementation to capture before checksums and update its asset verification tests.

## Files to Create

- `internal/le/site/docdata.go` — concrete semantic data types.
- `internal/le/site/doccollect.go` — owning contributor collection before presentation.
- `internal/le/site/docrender.go` — current presentation over the shared semantic dataset.
- `internal/le/site/docbundle.go` — deterministic bundle encoding and strict bounded reading.
- `internal/le/site/docrelease.go` — authenticated catalog admission and release producer.
- `internal/component/config/yang/source_export.go` — product-owned export of resolved original schema sources.
- `website/data/documentation-releases.json` — empty catalog until a verified release exists.
- `docs/architecture/documentation-data.md` — semantic format, immutability, compatibility, capture/replay and the owner quotes.

Create the unit and functional files named in the TDD tables. Keep small semantic, malformed and supported-format fixtures under `internal/le/site/testdata/documentation/`, without copying a complete website.

### Integration Checklist

| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | No | Source export is a Go tooling boundary. No new operator command or configuration leaf. |
| YANG validation constraints | No | Original modules and existing validation behavior remain unchanged. |
| YANG custom validators | No | No new configuration values. |
| CLI commands/flags | Yes | Site capture/check/register actions use existing launcher actions and typed answers. |
| CLI grammar (keyword before value) | Yes | Grammar in Entry Points. Register keyword completion and standard pipe support. |
| Editor autocomplete | N-A | No configuration editor surface. Launcher completion derives from action parameters. |
| Functional test for new RPC/API | Yes | The five launcher entry-point cases above. No daemon RPC. |
| Pipe completeness | Yes | Structured capture/check reports through the common renderer. |
| Env var registration | No | No new environment settings. |
| Doctor check for runtime dependencies | N-A | No daemon runtime dependency. Site release preflight owns missing/corrupt archive diagnostics. |
| Prometheus counters/metrics | No | Offline build operation. |
| BGP family surface | N-A | No BGP behavior or registration changes. |

### Documentation Update Checklist

| # | Question | Applies? | File to update / reason |
|---|----------|----------|-------------------------|
| 1 | New user-facing feature, scope or level? | No | Website delivery capability, no daemon feature maturity claim. |
| 2 | Config syntax changed? | No | Raw schemas preserved, no parser/config change. |
| 3 | CLI command added/changed? | Yes | `docs/contributing/gh-pages.md`: launcher capture/check/register syntax and failures. Operator command reference remains unchanged. |
| 4 | API/RPC added/changed? | No | No public runtime endpoint. |
| 5 | Plugin added/changed? | No | Inventory consumption only. |
| 6 | Has a user guide page? | Yes | `docs/contributing/gh-pages.md`: release publishing and offline recovery. |
| 7 | Wire format changed? | No | Bundle format is documented in its architecture page, no protocol wire change. |
| 8 | Plugin SDK/protocol changed? | No | Product schema export stays internal. |
| 9 | RFC behavior implemented/changed/proven? | No | Preserve schemas, no RFC compliance claim. |
| 10 | Test infrastructure changed? | No | Use the existing fixture and functional runners. |
| 11 | Affects daemon comparison? | No | Preserve captured comparison facts where referenced, make no new parity claim. |
| 12 | Internal architecture changed? | Yes | `docs/architecture/documentation-data.md`, `website/AI.md`: semantic boundary and current renderers. |
| 13 | Route metadata keys changed? | No | No routing metadata changes. |
| 14 | Prometheus counters changed? | No | No runtime metrics. |
| 15 | Registered command or inventory changed? | Yes | `ai/INDEX.md` site entries and `website/AI.md` contributor/capture discovery. |
| 16 | Existing source anchors affected? | Yes | The website owners and `docs/architecture/config/yang-config-design.md` are named above. Derive the reached set with `./le spec citation anchors spec plan/release/delivery/spec-release-documentation-data.md` before implementation edits. Preserve unrelated anchored behavior. |
| 17 | Existing examples for this area? | Yes | Check site build/check examples in both owning website pages and add capture/check examples with source anchors. |

### Discovery and Mechanical Checks

| Question | Answer |
|----------|--------|
| Where is this found? | Site entries in `ai/INDEX.md`, launcher completion, and the website build guide. |
| What prevents drift? | One semantic contributor inventory, one release catalog, and exact bundle member/semantic-reference checks. |
| What proves it? | Named wiring cases, semantic corpus tests, selected-version site checks and source-free recovery. |

## Implementation Steps

1. **Phase: Wiring.** Register capture/check/register entry points and release producer. Add failing functional tests and a semantic dataset boundary. No placeholder path may publish success.
2. **Phase: Semantic collection.** Implement typed document/reference records and source conversion. Inventory all included contributors and unsupported constructs. Migrate persistent source IDs and preserve rolling aliases. Replace rolling documentation rendering with the shared semantic path. Prove no snapshot writes.
3. **Phase: Schema and source identity.** Export the complete resolved YANG source set. Bind every reference and module to pinned release profiles. Prove identity mismatches and missing bytes are refused.
4. **Phase: Bundle and integrity.** Implement deterministic encoding, strict bounded reading, format compatibility, atomic no-replace installation and cleanup. Prove corruption, races and failure behavior.
5. **Phase: Website replay.** Implement authenticated catalog registration, version context, routes, source citations, mirrors, downloads and scoped search. Prove source-free clean and partial rebuilds with two releases.
6. **Phase: Release integration and evidence.** Connect the release builder before checksums. Run the named end-to-end cases, update owning docs as their behavior changes, and perform browser checks. Do not publish without the owner's separate release instruction.

Every phase follows test-first development and updates the owning page in the same change. Independent implementation review and closure follow the repository workflow, not this planning turn.

### Critical Review Checklist

| Check | Evidence required |
|-------|-------------------|
| Semantic completeness | Every contributor's meaning survives a presentation-free round trip, including literal text and qualified generated facts. |
| No live release reads | Tests make repository, registry and daemon access fail during replay. |
| No rolling retention | Capture/serialization instrumentation and filesystem assertions cover successful and failed live builds. |
| Schema completeness | Resolved dependency edges and source object hashes agree for every profile. |
| Version isolation | Link, search and download fixtures cover two releases and rolling. |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| Presentation-free versioned format and reader | Semantic schema tests, independent renderer test and supported-format fixtures |
| Immutable release capture | Registered capture/check functional cases plus deterministic/collision tests |
| Versioned website recovery | Rebuild functional case and selected-version site checks |
| No rolling snapshots | `TestRollingDocumentationDoesNotSnapshot` and filesystem assertions in the rebuild case |
| Signed release integration | Release wiring case proving checksum coverage and failure refusal |

### Security Review Checklist

| Check | What to verify |
|-------|----------------|
| Input handling | Strict schema, member paths, bounds and duplicate detection before publication. |
| Trust | Expected archive digest comes from authenticated release metadata, never from the archive alone. |
| Rendering | Text and examples cannot inject HTML, scripts or active links. Only registered passive content types are accepted. |
| Failure atomicity | Existing archives and the accepted catalog remain unchanged after a failed or conflicting operation. |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Unsupported source meaning | Extend the semantic conversion and its fixture before capture. Do not omit the document. |
| Release/profile provenance mismatch | Refuse capture and fix the producing release input. |
| Corrupt or unavailable release archive | Recover the exact authenticated archive. Never recreate its facts from rolling sources. |
| Renderer loses old-format meaning | Repair the format reader/renderer, preserving archived bytes. |
| Reached test fails | Fix the producing behavior. Do not weaken the assertion or reduce the corpus. |

## Design Insights

The archive freezes meaning and original schemas. The renderer owns all presentation, including Markdown exports. Both live and historical generation need this same boundary.

## Owned journal rows

No existing journal row is assigned to this new delivery feature.

| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Semantic JSON plus original source objects | HTML bodies, Markdown snapshots, copied complete site | Owner requires no presentation in the bundle and future rendering freedom. |
| Collect once, render from one data type | Separate historical renderer or live-source fallback | Keeps rolling and release semantics consistent without crossing their input boundaries. |
| Self-contained release archive | Git tag alone or mutable external asset references | Recovery must not need old source checkout, executable or mutable content. |
| Immutable archive, adaptable reader | In-place archive migration or overwrite | Preserve provenance and signed hashes while supporting future renderers. |

## Known Limitations

This design does not preserve historical page appearance or identical rendered HTML. It does not promise to discover corrections to old release content. Signing and publication remain owned by the release workflow dependency.

## Checklist

### Pre-Spec Verification

Planning review on 2026-10-24: an independent reader checked the requirements, current producers, entry points and acceptance coverage. Catalog authentication and persistent identity declarations were clarified. A focused follow-up found no remaining issue in those changes. Implementation and its tests remain unstarted.

- [ ] Metadata, scope, release dependency and source-backed current behavior are complete.
- [ ] Owner requirements are preserved verbatim and the data contract excludes presentation.
- [ ] All acceptance criteria have concrete tests and real entry-point coverage.
- [ ] Integration, documentation and discovery tables are answered.
- [ ] No code snippets, completed implementation claims or checked template markers exist.

### Goal Gates

- [ ] Every acceptance criterion has direct evidence, including source-free recovery and no rolling snapshots.
- [ ] Every assumption is confirmed or the design is corrected.
- [ ] Only reached package/functional checks are run by agents. Full release verification remains owner-run.
- [ ] All temporary capture/build inputs are removed after ownership and active-process checks.

### TDD

- [ ] Record the initial failing result and passing result for each behavior through its owning test action.
- [ ] Prove the real entry points discriminate missing capture, live fallback and version mixing.
- [ ] Cover declared numeric limits, corruption, concurrency and failure cleanup.

### Closure

- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` only when implementation reaches closure.
- [ ] Complete independent implementation review and the goal validation evidence.
- [ ] Move the owner quotes and lasting contracts to the owning architecture page and repoint their consumers.
- [ ] Commit implementation and its evidence before the separate spec-removal commit through `./le commit create`.
