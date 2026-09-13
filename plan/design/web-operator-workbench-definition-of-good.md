# Ze Web Operator Workbench: Definition of Good

## What This Document Is

This document defines the high-level quality bar for the alternative Ze web UI.
It does not specify every page. It defines what the UI must make possible, how
it should feel to a network engineer, and how future designs should be judged.

Implementers and AI agents must treat this as a decision contract. When there
are multiple plausible UI designs, choose the one that satisfies these rules in
priority order. If a design violates a MUST below, do not implement it without a
new explicit design decision.

The page-level blueprint lives in
[web-interface-design.md](web-interface-design.md). The implementation spec for
the modular registry lives in
[`plan/spec-web-plugin-extension-registry.md`](../spec-web-plugin-extension-registry.md).

## The Short Version

A good Ze web UI lets an engineer understand the current device, make a precise
configuration change, commit it safely, and verify the result without losing
context.

It should feel like an operational control surface, not like a schema browser
with decoration. The YANG interface remains available because it is complete,
accurate, and valuable. The web CLI remains available because exact CLI flow is
sometimes the fastest or safest operator path. The workbench earns its existence
by reducing friction for common engineering workflows.

## Primary User

The primary user is a network engineer operating a real device:

- they know protocols and router behavior;
- they often arrive with a specific task, fault, peer, interface, route, or
  policy in mind;
- they need to inspect live state before and after making a change;
- they need confidence that the UI did not hide important configuration;
- they may be working under incident pressure.

The UI is not a marketing site, not a simplified consumer control panel, and not
an attempt to hide networking concepts from network operators.

## Normative Language

- **MUST** means the implementation is not acceptable without it.
- **SHOULD** means the default choice; deviating requires a documented reason.
- **MAY** means acceptable when it does not weaken a MUST or SHOULD.
- **DO NOT** marks a known bad direction.

## Decision Hierarchy

When design goals conflict, resolve them in this order:

1. Preserve correctness, authorization, audit, and the YANG-backed config model.
2. Preserve the operator loop: observe, change, commit, verify.
3. Preserve context: avoid unnecessary page changes and manual command rebuilds.
4. Preserve feature ownership: the owning feature registers its UI contribution.
5. Preserve performance under real router data sizes.
6. Preserve visual consistency with the shared workbench shell and components.

A visually nicer design that breaks a higher-priority rule is the wrong design.

## Product Principles / Decision Rules

### 0. Keep the Three Web Modes Clear

**Decision rule:** CLI over HTTPS, the YANG editor, and the Operator Workbench
MUST be visible as three peer web modes.

The workbench is the new interface, but it is not the only interface. A good
implementation lets an engineer intentionally choose:

- CLI over HTTPS for exact CLI flow from a browser;
- the YANG editor for complete schema-backed coverage and recovery;
- the workbench for common operational workflows with tables, details, and
  contextual tools.

The mode boundary matters because it prevents the workbench from becoming both a
partial schema editor and an awkward terminal. Each mode should be excellent at
its job and share the same auth, audit, commit, and dispatcher foundations.

### 1. Optimize the Operator Loop

**Decision rule:** every page and interaction MUST support the observe ->
change -> commit -> verify loop with the fewest reasonable context changes.

The core loop is:

1. Find the object or fault.
2. Inspect configuration and live state.
3. Make the smallest intended change.
4. Review and commit.
5. Verify immediately from the same screen.
6. Keep the evidence or output if further action is needed.

Every major interaction should reduce the number of page changes in that loop.
An edit that requires moving to a different page and then manually navigating
back to run `show` commands is not good enough when the same context can carry
the related operational tools.

### 2. Keep the YANG Interface as the Bedrock

**Decision rule:** the workbench MUST remain an optimized alternative over the
same YANG-backed configuration truth, never a replacement for it.

The workbench is an alternative optimized view. The YANG-based UI remains the
canonical complete editor and fallback for anything the workbench does not yet
model.

Good behavior:

- a workbench page can fall back to generic YANG detail for uncommon branches;
- hand-written forms still write through the same editor/session/commit path;
- the workbench never accepts configuration that the YANG parser would reject;
- a missing purpose-built page is a graceful fallback, not a dead end.

Bad behavior:

- deleting or hiding the YANG interface as part of the workbench experiment;
- duplicating validation rules in ad hoc web-only code;
- treating YANG as an implementation detail rather than the source of truth.

### 3. Tables Are the Default Shape for Named Data

**Decision rule:** named data MUST default to a table unless it is a singleton
form or a visual monitor where a table would hide the operational meaning.

Named network objects should be visible as tables: peers, interfaces, addresses,
routes, sessions, policies, rules, lists, users, and services where applicable.

A good table has:

- stable columns with meaningful labels;
- useful empty states with headers still visible;
- sorting and filtering for large or operational lists;
- row actions for inspect, edit, enable/disable, clone, delete, and related
  diagnostics where appropriate;
- inline status for the facts an engineer scans first;
- server-side caps or pagination for large datasets.

The table is not only a visual preference. It is how an engineer compares many
objects quickly.

### 4. Configuration and State Belong Together

**Decision rule:** pages SHOULD show the configuration and live state that
determine whether the same object works.

The workbench should not force a split between configure and monitor when the
same object is being operated.

Good BGP peer page:

- configured remote address, AS, families, group, filters;
- live session state, uptime, negotiated capabilities, counters, last error;
- related tools: detail, capabilities, statistics, route refresh, soft reset,
  teardown with confirmation.

Good interface page:

- configured name, type, MTU, addresses, VLAN/unit data;
- live link state, counters, errors, drops, speed, carrier;
- related tools: counters, clear counters, packet capture, ping from source.

### 5. Operational Commands Stay in Context

**Decision rule:** related operational commands MUST be available from the object
or page context and MUST return output without replacing the workspace.

Operational commands should be attached to the object or page that makes them
relevant. The output should appear as an overlay, drawer, panel, or job dock so
the engineer does not lose the current table or edit form.

Good behavior:

- row and detail actions use `ze:related` descriptors where a schema node owns
  the context;
- the browser submits a tool ID and context path, never raw command text;
- output can be searched, copied, downloaded, rerun, pinned, and closed;
- multiple outputs can stay open at once;
- long-running jobs can be minimized and continue in the background.

Bad behavior:

- every check pushes the user into a separate terminal page;
- command output replaces the current workspace;
- rerun requires reconstructing command arguments manually.

### 6. The Workbench Must Be Modular

**Decision rule:** a feature-owned page, table, widget, monitor, or tool MUST be
registered by the owning feature or its web module, not added to a central
feature switch.

Good workbench code is contributed by the feature that owns the domain.

The central web package owns:

- shell layout;
- top navigation;
- left navigation composition;
- command palette shell;
- common table/detail/form/overlay components;
- auth/session/editor/commit integration;
- registry validation and deterministic ordering.

Feature packages own:

- their nav entries;
- their pages;
- their table definitions;
- their dashboard widgets;
- their monitor panels;
- their related tool metadata;
- their command-palette entries.

Both shapes are acceptable:

- a small feature can register from the feature package itself;
- a larger feature can register from a `<feature>-web` or `<feature>/web`
  package when that keeps dependencies and build tags cleaner.

A new feature page that requires editing a large central switch is a design
failure unless the edit is to a shared registry primitive.

### 7. Progressive Disclosure Beats Hidden Complexity

**Decision rule:** the default view SHOULD expose the facts needed for first
action, with deeper detail one click away.

The UI should expose the facts needed for first decision-making immediately and
put deeper detail one click away.

Good pattern:

- table columns show health, identity, state, and common configuration;
- a drawer/detail pane shows grouped configuration and live state;
- advanced fields remain accessible but do not dominate the scanning view;
- dangerous actions are present when useful but clearly separated and confirmed.

Bad pattern:

- a page shows every YANG leaf at once and calls that user-friendly;
- a table hides the operational state that determines whether the object works;
- advanced and destructive controls compete visually with routine actions.

### 8. The UI Must Be Fast Under Real Data

**Decision rule:** a page MUST bound expensive data and degrade independently
when live data is slow, missing, or unsupported.

The workbench should assume realistic router data sizes.

Good behavior:

- large tables are filtered, capped, paginated, or lazily loaded server-side;
- polling and SSE are chosen per data shape, not by habit;
- a failed operational command affects its overlay, not the whole page;
- slow widgets degrade independently;
- every empty or unavailable state says what is missing.

Bad behavior:

- rendering an unbounded route table as one HTML page;
- coupling the whole dashboard to the slowest widget;
- silently showing stale or placeholder data as if it were live.

### 9. Safety Is Visible and Enforced

**Decision rule:** safety MUST be visible in the UI and enforced on the server;
UI hiding is polish, not authorization.

The UI should make pending state, destructive actions, and authorization clear.

Good behavior:

- pending changes are always visible;
- review/commit/discard are reachable without losing page context;
- read-only users do not see controls that will obviously fail;
- mutation routes still enforce authorization server-side;
- destructive operational commands require confirmation;
- command output and config values are escaped, bounded, and audited.

### 10. Search and Keyboard Navigation Are First-Class

**Decision rule:** important pages, objects, and safe tools SHOULD be reachable
from search or command palette without requiring menu archaeology.

The top navigation should include a command palette that can find pages,
objects, tools, and safe diagnostics.

Good behavior:

- `Cmd/Ctrl+P` jumps to pages;
- `>` runs registered actions/tools;
- `/` runs safe diagnostics;
- search can find configured objects such as peers, interfaces, addresses, and
  policies;
- palette actions use registered IDs and server-side resolution.

Bad behavior:

- search only matches static menu names;
- the palette accepts arbitrary raw commands from browser text;
- keyboard shortcuts exist but are undiscoverable or inconsistent.

## What Good Looks Like in Real Workflows

### Bring Up a BGP Peer

The engineer opens Routing > BGP > Peers, adds a peer, reviews pending changes,
commits, and sees the peer row transition from configured/down to established or
failing. The row exposes capabilities, statistics, route refresh, and teardown
without leaving the table.

If the peer fails, the engineer can open last error, session details, local
address resolution, and route policy context from the same page.

### Change an Interface and Verify It

The engineer opens Interfaces, edits MTU or admin state, commits, and sees live
link state and counters update in place. If traffic is not moving, they can run
counter detail, clear counters, packet capture, or ping from the interface
context without navigating away.

### Debug a Firewall or Policy Issue

The engineer opens the policy/rules table, filters to the affected interface,
prefix, chain, or action, edits or inserts a rule in a drawer, commits, and then
checks counters/logs from the same rule table. The page does not reload into an
unfiltered default view after the change.

### Work During an Incident

The engineer uses the dashboard or command palette to jump to a failing section,
opens several diagnostic overlays, pins the useful output, changes config, and
reruns the same diagnostics. The UI helps preserve context and evidence rather
than forcing a sequence of unrelated pages.

## Visual and Interaction Quality Bar

### Layout

- Top bar: product identity, current device identity, breadcrumbs, pending
  changes, command palette, user/session controls.
- Left navigation: major sections, feature-contributed entries, active state,
  warning badges where real state supports them.
- Workspace: dense, scan-friendly tables and forms with restrained styling.
- Overlay/job area: related command output that can coexist with the workspace.

### Tone

The UI should feel like a router operations tool: compact, direct, readable,
and calm. It should not feel like a marketing dashboard, a decorative landing
page, or a toy control panel.

### Tables

- Headers remain visible in empty states.
- Status uses words plus restrained visual indicators, not color alone.
- Filters are visible and removable.
- Column visibility/order is treated as table state.
- Row actions are predictable and consistently placed.

### Forms

- Common fields are grouped by task and protocol concept.
- Advanced fields are reachable without overwhelming the default view.
- Validation errors appear beside the field and in a page-level error area.
- The commit model is explicit; auto-save to draft is not the same as apply.

### Empty, Error, and Loading States

Every page must answer:

- Is there no data?
- Is the feature unavailable in this build?
- Is the daemon not running?
- Is the command unsupported in web-only mode?
- Is the user unauthorized?
- Is data loading, stale, or failed?

Blank panels are not acceptable.

## Implementation Spec Contract

Every substantial workbench feature, page family, or cross-cutting objective
MUST have an implementation-ready spec before code changes start. The spec must
be detailed enough that a fresh agent can implement it without relying on
conversation history.

A workbench implementation spec MUST include:

- the operator workflow and acceptance criteria it serves;
- the owning feature or `<feature>-web` package that registers the UI;
- the underlying modules, APIs, command handlers, YANG nodes, state snapshots,
  and build tags it depends on;
- the dependency readiness: already available, partial, missing, or explicitly
  out of scope;
- the data flow from config/session/state/command source to rendered component;
- the fallback behavior when dependencies are absent or compiled out;
- the tests that prove registration, fallback, authz, and large-data behavior;
- the exact files expected to change.

A page spec MAY depend on module work that is not complete yet, but it must name
that dependency and state whether the web work blocks on it, uses a placeholder,
or must create a narrower module-facing API first. Hidden dependencies are not
acceptable.

The spec set should be sliced by feature or objective, not by visual layer. Good
slices are `web-workbench-bgp`, `web-workbench-interfaces`,
`web-workbench-firewall`, `web-workbench-command-palette`, and
`web-workbench-related-job-dock`. Bad slices are `make tables prettier` or
`add some pages`, because an implementer cannot prove them end to end.

## Implementer / AI Contract

Before implementing a page, component, or workflow, write down:

- the operator workflow it improves;
- the implementation spec that authorizes the work;
- the owning feature or web module that will register it;
- the source of configuration truth;
- the source of live state;
- the underlying modules and APIs it depends on;
- the related tools or diagnostics available in context;
- the fallback path when data, permissions, build tags, or daemon state are
  missing.

During implementation:

- reuse shared workbench table, detail, form, overlay, and navigation components;
- keep browser submissions to IDs, context paths, and form fields;
- resolve commands and placeholders server-side;
- keep writes on the existing editor/session/commit path;
- add tests for registry ownership, fallback behavior, and unsafe command/data
  boundaries before adding visual polish.

If a requirement is unclear, prefer the smallest design that preserves the
operator loop and the YANG fallback. Do not invent a parallel config model to
make a page easier to render.

## Rejection Criteria

Reject or redesign a proposal if it:

- removes, hides, or weakens the YANG fallback;
- accepts raw operational command text from browser UI;
- requires central workbench switches for feature-owned pages;
- separates verification from the page where the change was made;
- renders unbounded operational data into one page;
- hides authorization failures only by disabling buttons client-side;
- shows stale or placeholder operational state as if it were live;
- has no clear empty/error/unavailable state;
- has no implementation-ready spec or hides dependencies on unfinished modules;
- makes destructive actions look like routine row actions.

## What Good Is Not

- A prettier YANG tree with the same navigation friction.
- A central web package that knows every feature-specific page.
- A dashboard that hides configuration detail behind vague health cards.
- A workflow where verification requires opening a separate terminal and typing
  commands from memory.
- A UI that works only for small lab configs.
- A workbench that prevents fallback to the complete YANG editor.

## Design Review Checklist

Use this checklist when reviewing a proposed page or implementation. A page
that fails any MUST-level question needs redesign before implementation:

- Is there an implementation-ready spec for this feature or objective?
- Does the spec name the underlying modules, APIs, commands, YANG nodes, state
  snapshots, and build tags it depends on?
- Does it support the observe -> change -> commit -> verify loop?
- Can the engineer run related operational checks without leaving context?
- Does it keep YANG as the validation and fallback source of truth?
- Is named data presented as a table with useful empty state?
- Are live state and config visible together where they affect each other?
- Is the page registered by the owning feature rather than a central switch?
- Does the page degrade clearly when data, permissions, or build tags are
  missing?
- Is large data bounded or lazily loaded server-side?
- Are destructive actions confirmed and audited?
- Can the page be found from navigation and command palette?

## Non-Goals

- A full theming or skinning system in the first implementation.
- A client-side single-page application framework.
- Free-form web command execution outside the existing dispatcher and authz
  path.
- Removing the YANG/Finder interface.
