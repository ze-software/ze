# Spec: web-2 -- Config View (Read-Only Navigation)

| Field | Value |
|-------|-------|
| Status | in-progress |
| Depends | spec-web-1-foundation |
| Phase | 1/10 |
| Updated | 2026-03-27 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` -- workflow rules
3. `plan/spec-web-0-umbrella.md` -- umbrella design decisions (D-7 through D-9, D-15 through D-17, D-19)
4. `internal/component/config/schema.go` -- Node types, ValueType, schemaGetter
5. `internal/component/config/tree.go` -- runtime config tree accessors
6. `internal/component/cli/editor.go` -- schemaGetter interface, walkSchema
7. `internal/component/cli/editor_walk.go` -- walkPath, list key consumption
8. `internal/component/web/handler.go` -- route registration (from web-1)
9. `internal/component/web/render.go` -- template rendering (from web-1)

## Task

Implement YANG-to-HTML template rendering for config tree navigation. The web UI walks the YANG schema (via `schemaGetter`) and the runtime config tree to render read-only views of the configuration. Each YANG node kind gets its own HTML template. Breadcrumb navigation with a back button enables traversal up and down the config hierarchy. URL path segments map 1:1 to the YANG context path.

This is Phase 2 of the web interface. It builds on the foundation from web-1 (HTTP server, TLS, auth, layout frame, asset embedding) and provides the read-only config display that web-3 (config editing) will extend with write operations.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` -- overall architecture, component model
  -> Constraint: web UI is a component (`internal/component/web/`), not a plugin
- [ ] `docs/architecture/zefs-format.md` -- config tree storage
  -> Constraint: Tree accessors provide runtime config values for form field population
- [ ] `plan/spec-web-0-umbrella.md` -- umbrella design decisions
  -> Decision: D-7 URL scheme, D-8 template structure, D-9 schema source, D-15 list layout, D-16 navigation, D-17 container layout, D-19 HTMX partials

### RFC Summaries (MUST for protocol work)
- Not applicable (no protocol work in this spec)

### Source Files
- [ ] `internal/component/config/schema.go` -- Node types (ContainerNode, ListNode, LeafNode, FlexNode, FreeformNode, InlineListNode, MultiLeafNode, BracketLeafListNode, ValueOrArrayNode), ValueType enum (10 types), validation
  -> Constraint: Node.Kind() determines which template to render. ValueType determines input field type. FreeformNode is terminal (no schemaGetter). MultiLeafNode, BracketLeafListNode, and ValueOrArrayNode all return NodeLeaf from Kind() -- distinguished by concrete type assertion.
- [ ] `internal/component/config/tree.go` -- Tree with Get(), GetList(), GetContainer(), ordered iteration
  -> Constraint: Tree provides configured values. Missing keys mean unconfigured leaves.
- [ ] `internal/component/config/yang_schema.go` -- yangTypeToValueType, YANG entry to config node mapping
  -> Constraint: ValueType comes from YANG type mapping. Range constraints from YANG entry.
- [ ] `internal/component/cli/editor.go` -- schemaGetter interface definition, walkSchema
  -> Decision: web handler reuses the same schemaGetter interface and walkSchema logic
- [ ] `internal/component/cli/editor_walk.go` -- walkPath implementation, list key path consumption
  -> Constraint: list keys consume 2 path segments (list name + key value)
- [ ] `internal/component/web/handler.go` -- route registration from web-1
  -> Constraint: config routes registered on the existing mux
- [ ] `internal/component/web/render.go` -- template rendering from web-1
  -> Decision: extend with node-kind-specific template data types

**Key insights:**
- schemaGetter is the polymorphic interface for walking the YANG tree -- ContainerNode, ListNode, FlexNode, InlineListNode all implement `Get(name) Node`. FreeformNode does NOT implement schemaGetter
- ValueType determines HTML input type: TypeString -> text, TypeBool -> checkbox, TypeUint16 -> number (0-65535), TypeUint32 -> number (0-4294967295), TypeIPv4 -> text with dotted-quad pattern, TypeIPv6 -> text with colon-hex pattern, TypeIP -> text (both formats), TypePrefix -> text with CIDR pattern, TypeDuration -> text with duration placeholder, TypeInt -> number (signed)
- Enum detection uses goyang.Entry.Type (restricted values), not ValueType. When goyang.Entry has enum values, render as dropdown regardless of ValueType
- List keys consume 2 path segments during schema walk (the list name and the key value)
- The config Tree provides actual configured values; the schema provides the structure and metadata
- `goyang.Entry` provides YANG descriptions and type information (ranges, enums) for help text and input constraints
- URL path segments map 1:1 to contextPath per D-7

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/config/schema.go` -- defines Node interface, ContainerNode, ListNode, LeafNode, FlexNode, FreeformNode, InlineListNode, MultiLeafNode, BracketLeafListNode, ValueOrArrayNode, ValueType (TypeString, TypeBool, TypeUint16, TypeUint32, TypeIPv4, TypeIPv6, TypeIP, TypePrefix, TypeDuration, TypeInt)
- [ ] `internal/component/config/tree.go` -- Tree with Get/Set methods, ordered children, list iteration
- [ ] `internal/component/cli/editor.go` -- schemaGetter interface: `Get(name string) Node`. walkSchema walks `[]string` path against schema.
- [ ] `internal/component/cli/editor_walk.go` -- walkPath walks both schema and tree in parallel. For lists, the second path segment is the key value.
- [ ] `internal/component/web/handler.go` -- (from web-1) route registration, session cookie auth middleware, layout rendering
- [ ] `internal/component/web/render.go` -- (from web-1) template parsing, template execution, content negotiation

**Behavior to preserve:**
- schemaGetter interface and walkSchema logic unchanged (consumed, not modified)
- config.Node type hierarchy unchanged
- ValueType enum values and string representations unchanged
- Tree accessor patterns unchanged
- Web-1 layout frame, session cookie auth middleware, and content negotiation unchanged

**Behavior to change:**
- None -- all existing behavior preserved. This spec adds new handlers and templates for config navigation.

## Data Flow (MANDATORY)

### Entry Point
- Browser sends GET request to `/show/...` URL (e.g., `/show/bgp/peer/192.168.1.1/`)
- Format: HTTPS URL with path segments mapping to YANG schema path
- List key values containing `/` (e.g., TypePrefix `10.0.0.0/8`) must be URL-encoded as `%2F` in the URL path. The web handler URL-decodes path segments before schema walking

### Transformation Path
1. Auth middleware validates session cookie (from web-1)
2. Config handler extracts URL path, strips `/show/` prefix, splits into `[]string` path segments. Each segment is URL-decoded to handle encoded characters (e.g., `%2F` back to `/` for prefix keys)
3. Schema walk: path segments validated against YANG schema via schemaGetter, producing the target Node
4. Tree walk: same path segments used to traverse the runtime config Tree, producing configured values
5. Node kind dispatch: Node.Kind() selects the template (container.html, list.html, flex.html, freeform.html, inline_list.html). FreeformNode does NOT implement schemaGetter (no Get() method), so the web handler treats it as terminal and renders its entries without drill-down
6. Template data assembly: schema node (structure), tree data (values), YANG entry (metadata), breadcrumb path
7. For leaf inputs: ValueType mapped to HTML input type, YANG range/enum constraints extracted
8. Template execution: Go html/template renders HTML with assembled data
9. HTMX partial response: if `HX-Request` header present, return content fragment only (no layout wrapper)

### Boundaries Crossed

| Boundary | How | Verified |
|----------|-----|----------|
| Browser <-> Web server | HTTPS GET with session cookie, HTMX partial swap headers | [ ] |
| Web handler <-> Schema | Direct in-process via schemaGetter.Get() chain | [ ] |
| Web handler <-> Tree | Direct in-process via Tree accessor methods | [ ] |
| Web handler <-> YANG metadata | Direct in-process via goyang.Entry fields (Description, Type) | [ ] |

### Integration Points
- `schemaGetter` interface -- web handler walks schema to determine node kind and available children
- `config.Tree` -- web handler reads tree at the resolved path for configured values
- `goyang.Entry` -- web handler reads Description for tooltips, Type for input constraints (range, enum values)
- Web-1 layout template -- config view templates are rendered inside the layout frame's content area
- Web-1 session cookie auth middleware -- every config request is authenticated via session cookie before reaching the config handler
- Web-1 content negotiation -- JSON response returned when `Accept: application/json` or `?format=json`. JSON response uses the same serializer as CLI `show | format json`. Envelope structure follows json-format.md CLI convention

### Architectural Verification
- [ ] No bypassed layers (web handler uses schemaGetter and Tree accessors, not raw data)
- [ ] No unintended coupling (config handler depends on schema and tree interfaces, not CLI editor internals)
- [ ] No duplicated functionality (reuses schemaGetter walk pattern from CLI editor, does not recreate)
- [ ] Zero-copy preserved where applicable (reads tree values by reference, no unnecessary copies)

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| GET `/show/bgp/` with session cookie | -> | `handler_config.go` schema walk + container template render | `test/plugin/web-config-view.ci` |
| GET `/show/bgp/peer/` with session cookie | -> | `handler_config.go` schema walk + list template render | `test/plugin/web-config-list.ci` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | GET `/show/` with valid session cookie | Root view renders with all top-level YANG containers as navigable links |
| AC-2 | GET `/show/bgp/` with valid session cookie | Container view: leaves as read-only form fields, sub-containers and lists as clickable links |
| AC-3 | GET `/show/bgp/peer/` with valid session cookie | List view: left panel shows peer key names, right panel shows instructions or is empty |
| AC-4 | GET `/show/bgp/peer/192.168.1.1/` with valid session cookie | List view: left panel with all keys (192.168.1.1 highlighted), right panel with peer's leaves and sub-containers |
| AC-5 | LeafNode with TypeString | Renders as text input |
| AC-6 | LeafNode with TypeBool | Renders as checkbox (web handler converts "on"/absent to "true"/"false" for Editor) |
| AC-7 | LeafNode with enum type (goyang.Entry.Type has restricted values) | Renders as dropdown select with all valid values as options. Enum detection uses goyang.Entry.Type, not ValueType. When goyang.Entry has enum values, render as dropdown regardless of ValueType |
| AC-8 | LeafNode with TypeUint16 | Renders as number input with min=0, max=65535 |
| AC-9 | LeafNode with TypeUint32 | Renders as number input with min=0, max=4294967295, further constrained by YANG range if present |
| AC-10 | LeafNode with TypeIPv4 | Renders as text input with dotted-quad pattern |
| AC-11 | LeafNode with TypeIPv6 | Renders as text input with colon-hex pattern |
| AC-12 | LeafNode with TypeIP | Renders as text input accepting both IPv4 and IPv6 |
| AC-13 | LeafNode with TypePrefix | Renders as text input with CIDR pattern (e.g., "10.0.0.0/8") |
| AC-14 | LeafNode with TypeDuration | Renders as text input with placeholder "e.g., 5s, 100ms" |
| AC-15 | LeafNode with TypeInt | Renders as number input with signed range |
| AC-16 | Breadcrumb at `/show/bgp/peer/192.168.1.1/` | Shows `[back] / > bgp > peer > 192.168.1.1` with each segment as a clickable link to its path |
| AC-17 | Back button at `/show/bgp/peer/192.168.1.1/` | Navigates to `/show/bgp/peer/` |
| AC-18 | Back button at `/show/bgp/` | Navigates to `/show/` |
| AC-19 | Leaf that has no configured value but has a YANG default | Input field shows the default value as placeholder text, visually distinct from configured values |
| AC-20 | LeafNode with YANG description in goyang.Entry | Description text rendered as tooltip or help text on the form field |
| AC-21 | FlexNode in config tree | Renders correctly based on flex mode: flag (no value), value (single value), or block (sub-elements) |
| AC-22 | FreeformNode in config tree | Renders as a list of its entries, not navigable deeper |
| AC-23 | InlineListNode in config tree | Renders with key panel like ListNode |
| AC-24 | Click a breadcrumb segment (HTMX request with HX-Request header) | Content area swaps via HTMX partial, not a full page reload |
| AC-25 | Click a list key in the left panel (HTMX request) | Only the right panel (detail area) swaps, left panel remains |
| AC-26 | List key `10.0.0.0/8` | Accessible at `/show/bgp/route/10.0.0.0%2F8/` -- URL-encoded slash in key value |
| AC-27 | GET `/show/bgp/peer/?format=json` with valid session cookie | Returns JSON matching the CLI `show \| format json` output structure with kebab-case keys per json-format.md |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestNodeKindToTemplate` | `internal/component/web/handler_config_test.go` | ContainerNode maps to container.html, ListNode to list.html, FlexNode to flex.html, FreeformNode to freeform.html, InlineListNode to inline_list.html | |
| `TestContainerTemplateData` | `internal/component/web/handler_config_test.go` | Container node produces template data with leaf fields, container links, and list links separated | |
| `TestListTemplateData` | `internal/component/web/handler_config_test.go` | List node produces template data with key names from Tree and selected key detail when key path present | |
| `TestLeafInputType` | `internal/component/web/handler_config_test.go` | All 10 ValueTypes mapped: TypeString to text, TypeBool to checkbox, TypeUint16 to number (0-65535), TypeUint32 to number (0-4294967295), TypeIPv4 to text with dotted-quad pattern, TypeIPv6 to text with colon-hex pattern, TypeIP to text (both formats), TypePrefix to text with CIDR pattern, TypeDuration to text with placeholder, TypeInt to number (signed). Enum from goyang.Entry.Type renders as select with options | |
| `TestBreadcrumbFromPath` | `internal/component/web/handler_config_test.go` | Path segments ["bgp", "peer", "192.168.1.1"] produce 3 breadcrumb segments plus back link to parent | |
| `TestBreadcrumbRoot` | `internal/component/web/handler_config_test.go` | Empty path produces only root element with no back button | |
| `TestSchemaWalkFromURL` | `internal/component/web/handler_config_test.go` | URL path validated against schema, returns correct Node for valid path | |
| `TestSchemaWalkListKey` | `internal/component/web/handler_config_test.go` | List key consumes 2 path segments (list name + key value), returns correct node | |
| `TestSchemaWalkInvalidPath` | `internal/component/web/handler_config_test.go` | Invalid path (nonexistent schema element) returns error suitable for 404 response | |
| `TestDefaultValuePlaceholder` | `internal/component/web/handler_config_test.go` | Unconfigured leaf with YANG default produces template data with default as placeholder and flag indicating unconfigured | |
| `TestYANGDescription` | `internal/component/web/handler_config_test.go` | goyang.Entry description field available in leaf template data for tooltip rendering | |

### Boundary Tests (MANDATORY for numeric inputs)

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| URL path depth | 0 to max YANG depth | Deepest valid YANG path | N/A | Path beyond schema depth returns 404 |
| List key | Depends on key type | Valid key value | N/A (missing key shows list view) | N/A (nonexistent key shows list view with no selection) |

### Functional Tests

| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `web-config-view` | `test/plugin/web-config-view.ci` | User navigates to `/show/bgp/`, sees container view with leaves and sub-container links | |
| `web-config-list` | `test/plugin/web-config-list.ci` | User navigates to `/show/bgp/peer/`, sees list view with peer keys in left panel | |

### Future (if deferring any tests)
- None -- all tests required for this spec are listed above

## Files to Modify

- `internal/component/web/handler.go` -- register `/show/` route prefix on mux, dispatch to config handler
- `internal/component/web/render.go` -- add template data types for container, list, leaf input, flex, freeform, inline list, and breadcrumb views; add ValueType-to-HTML-input-type mapping function covering all 10 ValueTypes plus goyang enum detection

### Integration Checklist

| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs) | No | N/A -- no new RPCs, reads existing YANG schemas |
| CLI commands/flags | No | N/A -- no CLI changes |
| Editor autocomplete | No | N/A -- autocomplete is web-5 scope |
| Functional test for new RPC/API | Yes | `test/plugin/web-config-view.ci`, `test/plugin/web-config-list.ci` |

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md` -- add web config navigation |
| 2 | Config syntax changed? | No | N/A |
| 3 | CLI command added/changed? | No | N/A |
| 4 | API/RPC added/changed? | No | N/A |
| 5 | Plugin added/changed? | No | N/A |
| 6 | Has a user guide page? | Yes | `docs/guide/web-interface.md` -- add config navigation section |
| 7 | Wire format changed? | No | N/A |
| 8 | Plugin SDK/protocol changed? | No | N/A |
| 9 | RFC behavior implemented? | No | N/A |
| 10 | Test infrastructure changed? | No | N/A |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` -- web-based config viewing |
| 12 | Internal architecture changed? | No | N/A |

## Files to Create

- `internal/component/web/handler_config.go` -- config tree GET handlers: URL path parsing, schema walk via schemaGetter, node kind dispatch to templates, tree value extraction, breadcrumb assembly, HTMX partial detection
- `internal/component/web/handler_config_test.go` -- unit tests for all template data assembly and schema walk logic
- `internal/component/web/templates/container.html` -- container node template: full-width layout, leaves as typed form fields (read-only), sub-containers and lists as navigable links
- `internal/component/web/templates/list.html` -- list node template: left panel with flat key list (single column, click to select), right panel with selected entry detail or empty state
- `internal/component/web/templates/leaf_input.html` -- leaf input partial: typed input field based on ValueType (text, checkbox, number, select), placeholder for defaults, tooltip for YANG description, read-only attribute
- `internal/component/web/templates/flex.html` -- flex node template: renders flag (no value), value (single input), or block (sub-elements) based on flex mode
- `internal/component/web/templates/freeform.html` -- FreeformNode template: terminal node, not navigable. Renders as a list of key entries (word -> true pattern). FreeformNode does NOT implement schemaGetter (no Get() method). Web handler treats it as terminal -- renders entries, no drill-down
- `internal/component/web/templates/inline_list.html` -- InlineListNode template: renders like list with key panel
- `internal/component/web/templates/breadcrumb.html` -- breadcrumb partial: back button (always present except at root) plus path segments as clickable links, HTMX target for content area swap
- `test/plugin/web-config-view.ci` -- functional test: navigate to container view, verify HTML contains expected leaf fields and container links
- `test/plugin/web-config-list.ci` -- functional test: navigate to list view, verify HTML contains peer keys in left panel

**Leaf subtype handling:** MultiLeafNode, BracketLeafListNode, and ValueOrArrayNode all return NodeLeaf from Kind(). Template code distinguishes them by concrete type assertion to render appropriate input: multi-value input for MultiLeaf, bracket list input for BracketLeafList, single-or-array for ValueOrArray.

## Implementation Steps

### /implement Stage Mapping

| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file |
| 2. Audit | Files to Modify, Files to Create, TDD Test Plan -- check what exists from web-1 |
| 3. Implement (TDD) | Implementation phases below (write-test-fail-implement-pass per phase) |
| 4. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` |
| 5. Critical review | Critical Review Checklist below |
| 6. Fix issues | Fix every issue from critical review |
| 7. Re-verify | Re-run stage 4 |
| 8. Repeat 5-7 | Max 2 review passes |
| 9. Deliverables review | Deliverables Checklist below |
| 10. Security review | Security Review Checklist below |
| 11. Re-verify | Re-run stage 4 |
| 12. Present summary | Executive Summary Report per `rules/planning.md` |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Schema Walk and Template Data** -- implement URL path to schema node resolution and template data assembly
   - Tests: `TestSchemaWalkFromURL`, `TestSchemaWalkListKey`, `TestSchemaWalkInvalidPath`, `TestNodeKindToTemplate`
   - Files: `internal/component/web/handler_config.go`, `internal/component/web/handler_config_test.go`
   - Verify: tests fail -> implement -> tests pass

2. **Phase: Container View** -- implement container template and container-specific data (leaf fields, sub-container links, list links)
   - Tests: `TestContainerTemplateData`
   - Files: `internal/component/web/handler_config.go`, `internal/component/web/templates/container.html`, `internal/component/web/render.go`
   - Verify: tests fail -> implement -> tests pass

3. **Phase: Leaf Input Types** -- implement ValueType-to-HTML-input mapping, YANG metadata extraction for constraints and descriptions
   - Tests: `TestLeafInputType`, `TestDefaultValuePlaceholder`, `TestYANGDescription`
   - Files: `internal/component/web/render.go`, `internal/component/web/templates/leaf_input.html`
   - Verify: tests fail -> implement -> tests pass

4. **Phase: List View** -- implement list template with split layout, key list, selected entry detail
   - Tests: `TestListTemplateData`
   - Files: `internal/component/web/handler_config.go`, `internal/component/web/templates/list.html`
   - Verify: tests fail -> implement -> tests pass

5. **Phase: Flex Node** -- implement flex template rendering for flag, value, and block modes
   - Tests: (covered by `TestNodeKindToTemplate` for dispatch; flex rendering tested via functional test)
   - Files: `internal/component/web/templates/flex.html`
   - Verify: tests fail -> implement -> tests pass

6. **Phase: Breadcrumb Navigation** -- implement breadcrumb partial with back button and clickable path segments
   - Tests: `TestBreadcrumbFromPath`, `TestBreadcrumbRoot`
   - Files: `internal/component/web/handler_config.go`, `internal/component/web/templates/breadcrumb.html`
   - Verify: tests fail -> implement -> tests pass

7. **Phase: Route Registration and HTMX Partials** -- register config routes on mux, implement HTMX partial response (detect HX-Request header, return fragment vs full page)
   - Tests: (integration verified by functional tests)
   - Files: `internal/component/web/handler.go`, `internal/component/web/handler_config.go`
   - Verify: tests fail -> implement -> tests pass

8. **Functional tests** -- create .ci tests verifying end-to-end config navigation from browser perspective
   - Tests: `test/plugin/web-config-view.ci`, `test/plugin/web-config-list.ci`
   - Files: functional test files
   - Verify: functional tests pass

9. **Full verification** -- `make ze-verify` (lint + all ze tests except fuzz)

10. **Complete spec** -- fill audit tables, write learned summary to `plan/learned/NNN-web-2-config-view.md`, delete spec from `plan/`. Summary is part of the commit.

### Critical Review Checklist (/implement stage 5)

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1 through AC-27 has implementation with file:line |
| Correctness | Schema walk handles list keys consuming 2 path segments correctly |
| Correctness | ValueType-to-input mapping covers all ValueType enum values |
| Correctness | Breadcrumb links point to correct URL paths at every depth level |
| Naming | HTML template names match D-8 from umbrella (container.html, list.html, etc.) |
| Naming | Template data field names are consistent with YANG naming (kebab-case in rendered output) |
| Data flow | Schema walk uses schemaGetter interface, not direct node type access |
| Data flow | Tree values read via Tree accessors, not bypassing to raw storage |
| Rule: no-layering | No duplicate schema walk logic (reuses pattern from CLI editor) |
| Rule: file-modularity | handler_config.go stays under 600 lines; split if multiple concerns emerge |

### Deliverables Checklist (/implement stage 9)

| Deliverable | Verification method |
|-------------|---------------------|
| Config GET handler registered on mux | Grep for `/show/` route registration in handler.go |
| container.html template exists | `ls internal/component/web/templates/container.html` |
| list.html template exists | `ls internal/component/web/templates/list.html` |
| leaf_input.html template exists | `ls internal/component/web/templates/leaf_input.html` |
| flex.html template exists | `ls internal/component/web/templates/flex.html` |
| freeform.html template exists | `ls internal/component/web/templates/freeform.html` |
| inline_list.html template exists | `ls internal/component/web/templates/inline_list.html` |
| breadcrumb.html template exists | `ls internal/component/web/templates/breadcrumb.html` |
| All 11 unit tests pass | `go test -run 'TestNodeKind\|TestContainer\|TestList\|TestLeafInput\|TestBreadcrumb\|TestSchemaWalk\|TestDefault\|TestYANG' ./internal/component/web/...` |
| Functional test web-config-view exists | `ls test/plugin/web-config-view.ci` |
| Functional test web-config-list exists | `ls test/plugin/web-config-list.ci` |
| Both functional tests pass | `ze-test bgp plugin web-config-view && ze-test bgp plugin web-config-list` |

### Security Review Checklist (/implement stage 10)

| Check | What to look for |
|-------|-----------------|
| Input validation | URL path segments validated against schema before use. Reject paths with `..`, null bytes, or characters outside YANG identifier set |
| Path traversal | URL path segments cannot escape the config tree (no `../` or absolute paths) |
| Template injection | All user-visible data rendered through html/template (auto-escapes HTML). No raw string concatenation in HTML output |
| Auth enforcement | Every config route goes through session cookie auth middleware (no unauthenticated access to config data) |
| Information leakage | 404 responses for invalid paths do not reveal schema structure beyond what is visible to authenticated users |
| Denial of service | Schema walk bounded by schema depth. No recursive descent without depth limit |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read source from Current Behavior -> RESEARCH if misunderstood |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Functional test fails | Check AC; if AC wrong -> DESIGN; if AC correct -> IMPLEMENT |
| Audit finds missing AC | Back to relevant phase and implement |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions

| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|

### Failed Approaches

| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|

### Escalation Candidates

| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights

## RFC Documentation

Not applicable -- no RFC protocol work in this spec.

## Implementation Summary

### What Was Implemented
- (to be filled during implementation)

### Bugs Found/Fixed
- (to be filled during implementation)

### Documentation Updates
- (to be filled during implementation)

### Deviations from Plan
- (to be filled during implementation)

## Implementation Audit

### Requirements from Task

| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria

| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan

| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan

| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Pre-Commit Verification

### Files Exist (ls)

| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)

| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)

| Entry Point | .ci File | Verified |
|-------------|----------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1 through AC-27 all demonstrated
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (`internal/component/web/`)
- [ ] Integration completeness proven end-to-end
- [ ] Architecture docs updated
- [ ] Critical Review passes (all 6 checks in `rules/quality.md` -- no failures)

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] RFC constraint comments added (N/A for this spec)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] No premature abstraction (3+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional tests for end-to-end behavior

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-web-2-config-view.md`
- [ ] **Summary included in commit** -- NEVER commit implementation without the completed summary. One commit = code + tests + summary.
