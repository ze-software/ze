# Spec: bus-2-implied-display-and-grant-refusals

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | config |
| Priority | P1 |
| Release reason | Configuration correctness and transparency of shipped behavior: what a plugin registered and Ze runs on the operator's behalf is invisible on the CLI, editor, web UI and API; a config whose allow-list gives a feature less than it needs loads and moves nothing; and AIGP at its default makes validation demand a Loc-RIB feed the operator never asked for |
| RFC gaps | - |
| Depends | `spec-explicit-plugin-and-attach-config` (closed; superseded this skeleton) |
| Phase | Design drafted 2026-11-23 on rulings R1 to R7. Owed before the status moves to design: the full Design documents verdicts, the changed-wire census, and the read of `rfc/short/rfc7311.md` |
| Handoff | - |
| Design tier | strongest tier |
| Implementation tier | development tier |
| Updated | 2026-11-26 |

Child 2 of 3, split from `spec-explicit-plugin-and-attach-config` (Child 1, closed).

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Child 1 makes every plugin register what it publishes, receives and sends, and runs those registrations as the default. This spec makes that default visible and makes a feature's needs enforceable.

### What moved to Child 1 on 2026-11-23

Rulings R1 and R7 turned "implied flows" from something this spec computed into something a plugin registers. The ownership is now:

| Mechanism | Owner |
|-----------|-------|
| The three declarations on the registration (publishes, receives, sends) | Child 1 |
| Resolving the edges that run: registered defaults, replaced or narrowed by the operator's flows | Child 1 |
| The delivery index (`DeliveryGraph`) | Child 1 |
| Configuration paths: a flow names its receiver (`to`) and its subjects (`match`) by path; receiver nodes and subject nodes are registered; the eight commit checks on a path (owner words W1 to W6, 2026-11-23) | Child 1 |
| The `origin` of each edge and every display of it | This spec |
| The `\| implied` view of the configuration | This spec |
| The requirement a feature's config places on a plugin, declared by that plugin, and the refusal of an allow-list that removes it | This spec |
| The AIGP trigger | This spec |

No resolver is written here. The display reads the edges Child 1 resolved, so what is shown and what runs cannot differ by construction.

### 1. The implied view (`| implied`, owner decisions D-10, Q4, Q5)

Nothing Ze does on an operator's behalf is hidden. One pipe operation shows every implied item beside what the operator wrote, marked as implied, with what implies it.

Every implied kind comes from a registration. None is computed from a name in engine or config code.

| Implied kind | Registered by | Implied by (shown) |
|--------------|---------------|--------------------|
| Endpoint: a plugin Ze loads with no `endpoint` written | The plugin's `ConfigRoots`, `Families`, `Dependencies`, `ConfigNeeds`, `DataPlane`, or a publication a written flow names | The config path, family, dependent plugin or flow |
| Reception: an event delivery with no flow written | The receiving plugin's `Receives` declaration, one per receiver instance: an endpoint, or an entry of a receiver node the plugin registered (an ExaBGP bridge script) | The receiver's configuration path, written or implied. The item is printed as the flow that would state it: `to` the receiver's path, `match` the publisher's root path, `event` the registered list, in the stored path form of Child 1, so an operator can copy it and narrow it |
| Send: a command a plugin may issue with no allow-list written | The plugin's `Sends` declaration | The endpoint |
| Filter member inserted in a peer's filter list (plugin `FilterInsertions` and built-in engine steps such as `loop-detection` on import and trailing `next-hop:auto` on export) | The plugin's `FilterInsertions` or the engine's registered implied-filter seam (direction, position, ref, reason) | The config path, session fact, or RFC default that implies the insertion; when added to an existing `filters [ ... ];` line, rendered on the same line with a trailing comment naming what was added and why (`filters [ my-filter next-hop:auto ]; # implied: next-hop:auto (<reason>)`) |
| The `state` delivery to the owner of a filter member, where the operator's flow omits it | The filter owner's registration | The filter member and the peer |
| Dormant reception: a registered reception with no publisher present | The receiving plugin | Named as dormant, with the missing publisher |

| Surface | Form |
|---------|------|
| Pipe, bare, no argument | `show config running \| implied`, `show config source <file> \| implied`, `show config dump <file> \| implied`, editor `show \| implied` and `show \| implied \| format config`. Written and implied items together; standard pipes such as `\| match` filter them |
| Text | Standalone implied items carry `# implied by: <reason>` above them. When `\| implied` modifies an existing line (such as inserting extra members into `filters [ ... ];`), it stays on the same line with a trailing comment naming what was added and why (`filters [ my-filter next-hop:auto ]; # implied: next-hop:auto (<reason>)`, owner ruling 2026-11-23). Always a single hash followed by a space, never `##` at column 0, which `config.DetectFormat` (`internal/component/config/serialize_set.go`) reads as a set-meta line. `show config running` gains a hierarchical text renderer |
| JSON and YAML | An envelope with the written config under `config` and the implied items under `implied` (`endpoints`, `flows`, `sends`, `filters`), each with `implied-by`. The written config stays byte-pure |
| A present and empty send allow-list | Shown as written, distinct from an absent one, in text and in the envelope |
| Plain-display notice | When implied items exist, a human display states the count and names `\| implied`: the editor status bar, the `\| text` footer, `stderr` of `ze config show`. Never in `\| json`, `\| yaml`, REST, gRPC or `ze config show` stdout |
| Web | The web CLI and the graphical inspection offer `show \| implied`. Workbench tree pages show implied rows read-only, with a badge |
| gNMI | `Get` returns the written config only, so `Get` then `Set` round-trips |
| No daemon, external endpoint | The view takes the program's declaration the way Child 1's offline validation does: it starts the program in query mode, which writes the declaration and never enters the run loop (ruled by Thomas on 2026-11-23). When the program cannot be asked, the view refuses, names the endpoint, its `run` line and the cause, and gives `ze show plugin conformance run "<run line>"`. It never shows a partial set as complete |

### 2. Declared feature requirements and the refusal of a narrower allow-list

Today BGP config code names plugins to decide what a feature needs. `requireRIBDelivery` (`internal/component/bgp/config/flowspec_binding.go`) holds the literals `bgp-rs`, `bgp-rr` and `bgp-adj-rib-in`, and `wireRedistributeDelivery` and `requireSelectionFeeds` (`redistribute_binding.go`) reach the Loc-RIB and the orchestrator through named constants. Child 1 keeps those checks working in bus syntax. This spec replaces the names with a declaration.

| Step | Design |
|------|--------|
| The declaration | A plugin declares, on its registration, each requirement a feature's config places on it: the condition (a config path or a session fact, in the form `FilterInsertions` conditions already use), and the receptions and sends that condition needs. It generalizes `rpc.DisseminationDecl`, which states two-sided transit needs today |
| The check | One function compares every active requirement with the edges Child 1 resolved. It runs at `ze config validate`, editor commit, reload and `ze start`, with one verdict |
| The refusal | Where an operator's flow (which replaces the registered reception for its pair) or the endpoint's one send allow-list removes something an active requirement needs, the commit is refused. The message names the feature, the endpoint, the flow or the send list, and the line that completes it, as a typed `FixCommand`. Ze never widens what was written and never loads it to do nothing |
| What it replaces | The literals in `requireRIBDelivery`, the constants behind `wireRedistributeDelivery` and `requireSelectionFeeds`, and the per-role check of `ValidateDisseminationBindings` with its `disseminationAttachLine`. One declaration, one check, one message shape |
| Redistribution | Gains the refusal it lacks today: a rule whose Loc-RIB or orchestrator is narrowed below what the rule needs is refused, where today it loads and moves no route with one log line |
| The implicit `state` delivery | Stays as documented on `architecture.md`: the filter owner receives `state` even where the operator's flow omits it, and the edge is marked. The requirement declaration is what states it, in place of `grantFilterOwnersState` naming it in BGP config code |
| Dynamic groups | The collectors in `bgp/config/loader_create.go` that skip dynamic groups for family, event and send auto-load are fixed, so an implied endpoint a dynamic group needs is loaded and shown |

### 3. AIGP

| Point | Statement |
|-------|-----------|
| Today | `PeerSettings.AIGPEnabled` (`internal/component/bgp/reactor/config_aigp.go`) answers true for every iBGP peer with no `aigp` configured, citing RFC 7311 Section 3.3. `selectionFeatures` reads it, so any config with an iBGP peer makes `requireSelectionFeeds` demand that the Loc-RIB receives `update-received` and `state` from every peer |
| Owner decision D-10 | A capability that is merely on by default implies nothing |
| After Child 1 | The Loc-RIB registers the reception of every peer's updates. A config with no flow already meets what AIGP selection needs, with no requirement stated |
| The change here | Only a peer with `aigp` written in its config activates the requirement. AIGP at its default activates none |
| What changes for an operator | One case: the operator narrowed the Loc-RIB's reception and left AIGP at its default. Today the commit is refused. After, it loads, and the narrowed peers contribute no candidate, which is what the operator wrote |
| RFC 7311 | The session default is unchanged: AIGP_SESSION stays enabled on iBGP sessions and disabled on other EBGP sessions (Section 3.3, quoted in `config_aigp.go`). What Ze sends, accepts and compares is unchanged. Only a config-time refusal is removed. The read of `rfc/short/rfc7311.md` confirms this before the design is presented, and names any requirement that binds the default path |
| Companion assertion | A test on the default configuration (iBGP peers, no flow, no `aigp` written) proves that a route carrying AIGP is still compared by AIGP. The default path is what RFC compliance is judged on (`ai/rules/rfc-compliance.md`) |

### 4. Edge origin on `show event delivery`

Child 1 builds the index. This spec adds one field to each edge and shows it.

| Origin | Meaning |
|--------|---------|
| `written` | An operator flow keeps or narrows this edge |
| `registered` | The plugin's registration, with no operator flow |
| `required` | A feature's active requirement, including the implicit `state` delivery. Replaces `ProcessEdges.Implicit` |
| `dormant` | Registered, with no publisher present |

`show event delivery` lists every namespace, not BGP alone, and `ze config graph` draws the same edges with the same origins. Each edge names its subject and its receiver by the configuration paths Child 1 resolved it from (the admitting group's path for a dynamic peer), beside the subject key the publisher uses, so an operator can tell which `match` path covered a peer. Lifecycle deliveries that bypass the grant today (`SubscribedProcsNamed`, the removal notice to an OPEN validator declared by `wants-validate-open`) are registered receptions under Child 1 and appear here like any other.

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/architecture.md` - "Owner ruling: implied and shown, or written", "The Two Bindings Ze Derives", "The Implicit State Grant"
  -> Decision: implied items run and are shown; a list narrower than a feature needs is refused at commit
  -> Constraint: Ze never widens an operator-written list, with the one documented exception of the filter owner's `state`
- [ ] `ai/META-DECISIONS.md` - "CLI, config, and daemon surfaces"
  -> Constraint: one display option shows every implied item with what implies it; a design that implies something it cannot show is wrong; a plugin declares, and engine and config code never name a feature, a plugin or a config path
- [ ] `docs/architecture/api/commands.md` - pipe operations, command-declared pipe filters, answer shapes
  -> Constraint: `\| implied` is a command-declared pipe operation with no argument; it keeps the pipe grammar and adds no command argument
- [ ] `docs/architecture/config/yang-config-design.md` - "Where a validator actually runs"
  -> Constraint: a refusal that must hold at validate, commit and start lives where all three run it
- [ ] `docs/architecture/web-interface.md`, `docs/architecture/web-workbench-pages.md`
  -> Constraint: markup is `.templ` with no inline script or style
- [ ] `rfc/short/rfc7311.md`
  -> Constraint: the default configuration path stays compliant; owed read

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/config/flowspec_binding.go` - `requireRIBDelivery` names `bgp-rs`, `bgp-rr` and `bgp-adj-rib-in` as literals and refuses with an `attach process` line
- [ ] `internal/component/bgp/config/redistribute_binding.go` - `wireRedistributeDelivery`, `selectionFeatures`, `requireSelectionFeeds`: FlowSpec and AIGP are refused when narrowed, redistribution is not
- [ ] `internal/component/bgp/config/filter_state_binding.go` - `grantFilterOwnersState` widens the operator's block with `state`, marked implicit
- [ ] `internal/component/bgp/reactor/config_aigp.go` - `AIGPEnabled` defaults to true on iBGP
- [ ] `internal/component/bgp/reactor/config.go` - `EnsureProcessBinding`, `GrantImplicitReceive`
- [ ] `internal/component/plugin/server/delivery_graph.go` - `ProcessEdges.Implicit`, `Inspect`, one namespace
- [ ] `internal/component/plugin/server/delivery_reconcile.go` - `reconcileDelivery` logs each disagreement; `DeliveryDisagreement` feeds doctor
- [ ] `internal/component/plugin/server/dissemination_validate.go` - `ValidateDisseminationBindings` and `disseminationAttachLine`
- [ ] `pkg/plugin/rpc/dissemination.go` - `DisseminationDecl`: role, source receive, destination receive, destination send
- [ ] `internal/component/plugin/server/startup_autoload.go` - plugins loaded for a config path, a family, a dependency, a custom event token, a custom send token
- [ ] `internal/component/command/pipe_filter.go`, `pipe_catalog.go` - `PipeFilter` and its argument kinds, the declaration a command makes for a pipe operation of its own, beside the catalog of generic operators
- [ ] `internal/component/config/cli/running_rpc.go` - `handleShowConfigRunning` answers the running tree with secrets masked
- [ ] `internal/component/cli/model_load.go`, `internal/component/web/graphical_inspection.go` - the editor's show modifiers and the web inspection's admitted words
- [ ] `internal/component/cmd/show/show.go` - `show event delivery`

**Behavior to preserve:**
- An operator-written list is never widened, apart from the filter owner's `state`.
- Machine outputs (`| json`, `| yaml`, REST and gRPC running config, `ze config show` stdout, gNMI `Get`) carry the written config with no prose.
- FlowSpec and explicitly configured AIGP keep refusing a Loc-RIB narrowed below what they need.
- The default configuration path stays RFC 7311 compliant.

**Behavior to change:**
- `| implied` exists on the three config commands, the editor and the web surfaces.
- Feature requirements are declared by plugins; BGP config code names no plugin.
- Redistribution refuses a narrower list.
- AIGP at its default activates no requirement.
- `show event delivery` covers every namespace and shows an origin per edge.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Command-declared pipe filters | `internal/component/command/pipe_filter.go` (`PipeFilter`, `RegisterPipeFilters`) | A command declares a pipe operation its own handler runs, on every surface | Extends: the three config commands declare `implied` with no argument |
| Editor show pipes | `internal/component/cli/model_load.go` (`ClassifyShowPipes`) | Parses the editor's `show \| ...` modifiers | Extends: adds `implied` and the status-bar notice |
| Plugin auto-load | `internal/component/plugin/server/startup_autoload.go` (`getConfigPathPlugins`, `getUnclaimedEventTypePlugins`, `getUnclaimedSendTypePlugins`) | Loads a plugin for a config root, family, dependency, custom event token or custom send token | Extends: each loader reports what it loaded and why, which is the implied endpoint list. Under Child 1 the event-token loader becomes "a flow names a publication this plugin registers". A flow carries no command: a send token such as `bgp:enhanced-refresh` (registered by `bgp-route-refresh`) now appears only in the send list of an endpoint, so what the send-token loader becomes is owed at design |
| Filter insertions | `registry.Registration.FilterInsertions`; `registry.CheckFilterInsertions`; `config.CheckFilterInsertionPaths` | A plugin declares a member and the condition under which the loader inserts it | Extends: the insertion is shown as implied, and its condition form is reused for feature requirements |
| Declared transit needs | `pkg/plugin/rpc/dissemination.go` (`DisseminationDecl`); `registry.Registration.Dissemination` | Declares the receptions and sends a peer-output role needs | Extends: generalized into the feature requirement, with a condition and one-sided needs |
| Role check and its fix line | `internal/component/plugin/server/dissemination_validate.go` (`ValidateDisseminationBindings`, `disseminationAttachLine`) | Refuses a partial transit grant, with an `attach process` line | Replaces: one requirement check and one message shape serve roles and features |
| Loc-RIB and replay requirements | `internal/component/bgp/config/flowspec_binding.go` (`requireRIBDelivery`); `redistribute_binding.go` (`requireSelectionFeeds`, `selectionFeatures`) | Refuses a narrowed Loc-RIB for FlowSpec and AIGP, naming three plugins as literals | Replaces: the plugins declare the requirement; the literals are deleted |
| Derived bindings | `redistribute_binding.go` (`wireRedistributeDelivery`); `internal/component/bgp/reactor/config.go` (`EnsureProcessBinding`) | Adds a binding a rule needs | Does not apply: Child 1 deletes them, because a registered reception covers every peer. This spec owns the refusal that remains |
| Implicit grant and its marking | `filter_state_binding.go` (`grantFilterOwnersState`); `reactor/config.go` (`GrantImplicitReceive`); `delivery_graph.go` (`ProcessEdges.Implicit`) | Widens the operator's block with `state`, marked implicit | Replaces: the filter owner declares the requirement; the `Implicit` list becomes the `required` origin |
| Delivery reconcile | `internal/component/plugin/server/delivery_reconcile.go` (`reconcileDelivery`); `doctor_snapshot.go` (`DeliveryDisagreement`) | Reports where a declaration and a grant disagree | Extends: reads the origin, so a `required` or `dormant` edge is reported for what it is |
| Delivery graph | `internal/component/plugin/server/delivery_graph.go` (`DeliveryGraph`, `Inspect`); `internal/component/cmd/show/show.go` | The index and its inspection, one namespace | Extends: the origin field and the multi-namespace inspection. The index itself is Child 1's |
| `ze config graph` | `internal/component/config/graph.go` | Draws listeners and bindings | Extends: draws the resolved edges with their origins |
| Typed fix of a refusal | `internal/component/cli` (`FixCommand`, `FrameFix`) | Formats a refusal's fix as an exact editor command on every surface | Extends: the requirement refusal carries one |
| Web inspection and workbench | `internal/component/web/graphical_inspection.go` (`graphicalInspectionWords`) and the workbench handlers | Renders `show \| ...` views and tree pages | Extends: `implied` in the inspection, read-only implied rows in the workbench |

## Design documents

The full verdict table is owed before the status moves to design. The rows already judged:

| Document | Section | Derived from | Binding statement | Verdict | How this design stands against it |
|----------|---------|--------------|-------------------|---------|-----------------------------------|
| `docs/architecture/api/architecture.md` | Owner ruling: implied and shown, or written | page of `bgp/config`, `plugin/server` | "A block the operator wrote that grants less than a feature the operator also configured needs is refused at commit, with the line that completes it." | agrees | The requirement check refuses it for every feature, redistribution included |
| `docs/architecture/api/architecture.md` | The Implicit State Grant | same page | "an `attach process` block for that plugin that lacks `state` gets it, because freeing the member's per-peer state is the plugin's obligation, not the operator's" | agrees | Kept; restated as a declared requirement and shown with the `required` origin |
| `docs/architecture/api/architecture.md` | Feature Requirements (Replacing Derived Bindings) | same page | "Derived `ProcessBinding` entries are deleted." | agrees | Landed in closed `spec-explicit-plugin-and-attach-config`, which supersedes this skeleton |
| `ai/doors/plugin-infra.md` | Boundaries | covering door | "The engine never enumerates plugins." | agrees | The three literals in `requireRIBDelivery` are deleted |
| `ai/doors/commands-cli.md` | Boundaries | covering door | "A handler answers data, never text." | agrees | The implied view is data with an `implied-by` field; the text renderer and the envelope are presentations of it |
| `docs/architecture/core-design.md` | Design decisions | decision rows on the page of `bgp/reactor/config.go` | "A configuration that holds such a mismatch is refused at daemon start as well as at commit, reload and `ze config validate`, by one verdict that is the same at each." | agrees | The requirement check gives one verdict at the same four points; it does not touch the data-plane guard |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config validation, commit, reload and daemon start; `show config running|source|dump | implied`; editor `show | implied`; the web inspection and workbench; `show event delivery`; `ze config graph`.

### Transformation Path
1. Child 1 resolves the edges and the loaded endpoints, each with the registration or flow it came from.
2. The requirement check compares each active requirement with those edges and refuses or passes.
3. The implied view reads the same edges and endpoints and marks each with its origin and what implies it.
4. The renderers produce hierarchical text with comments, the JSON and YAML envelope, the editor status line, and the workbench rows.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin registration <-> requirement check | A declared requirement, a pure value; no import from config into BGP | No |
| Daemon <-> offline reader | An offline reader holds compiled registrations only and refuses where an external declaration is needed | No |

### Changed-Wire Census

**Search evidence:** owed at design. The wires known to change: the answer of `show event delivery` (the `implicit` list becomes an `origin` per edge), the answer of `show config running | implied` (a new envelope), and the Stage 1 `dissemination` field, generalized into the requirement.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| owed at design | | | | |

### Integration Points
- The edge resolution of Child 1.
- `registry.Registration` and Stage 1, for the requirement declaration.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding, outbound | No | |
| Registration over hardcoding, inbound | No | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | Every feature's requirement is a function of the config and session facts a `FilterInsertions` condition can already state | The two conditions in use today: a config path, a session fact | A requirement needs a condition form that does not exist | Design pass over redistribution, FlowSpec, AIGP, the filter owner and the transit roles | unvalidated |
| A-2 | Removing the AIGP default trigger changes no behavior RFC 7311 requires on the default path | `config_aigp.go` quote of Section 3.3; R1 gives the Loc-RIB every peer by default | A requirement binds what Ze must do when the Loc-RIB is not fed | Read of `rfc/short/rfc7311.md` and the companion test | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The display and the runtime differ | `show event delivery` holds an edge `\| implied` lacks | Both read the edges Child 1 resolved; a parity test compares them |
| R-2 | The notice breaks a machine output | A `\| json` or `ze config show` test fails | The notice is written to human channels only; a negative test asserts its absence |
| R-3 | `##` at column 0 is read as a set-meta line | A round trip of `\| implied` output fails to parse | A single `# implied by:` everywhere |
| R-4 | An offline view shows a partial set as complete | A config with an external endpoint shows no implied reception offline | The offline view asks the program in query mode and refuses when it cannot |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A valid config is refused, a narrowed feature loads and does nothing, or a script that reads `show config` output breaks |
| How is it reverted? | Revert of this spec's commits; the syntax is Child 1's |
| Who else touches this path? | Child 1 and Child 3 |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| `show config running \| implied` on a daemon with no flow written | -> | The implied view over the resolved edges | `test/plugin/bus-implied-view.ci` (planned) |
| Commit of a redistribution rule with a narrowed Loc-RIB | -> | The requirement check | `test/parse/bus-requirement-refusals.ci` (planned) |
| `show event delivery` on a daemon with written, registered, required and dormant edges | -> | The origin field | `test/plugin/bus-delivery-origins.ci` (planned) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A redistribution rule importing from BGP, no flow written | The config loads at validate, commit and start; routes redistribute; `\| implied` shows the Loc-RIB endpoint and its receptions, each with what implies it | A config with no such rule shows no such endpoint |
| AC-2 | The same rule with a flow that narrows the Loc-RIB below what the rule needs | Validate, commit, reload and start refuse with one message that names the feature, the endpoint, the flow and the completing line as a typed fix. Completing the flow makes it load and redistribute | The operator's list is never widened |
| AC-3 | A rule exporting to BGP, with the orchestrator's send allow-list absent, then present without `update`, then present and empty | Absent loads and redistributes. The other two are refused with the completing line | An unrelated endpoint with an empty allow-list is not refused |
| AC-4 | A FlowSpec family, or `aigp` written on a peer, with the Loc-RIB narrowed | Refused in the same message shape as AC-2 | None |
| AC-5 | iBGP peers with no `aigp` written and the Loc-RIB narrowed | The config loads. On the default configuration with no flow, a route carrying AIGP is compared by AIGP | Only written `aigp` activates the requirement |
| AC-6 | Feature blocks, families, dependencies and a data-plane writer with no `endpoint` written, plus plugins with registered receptions, sends, `FilterInsertions`, and built-in auto-inserted filter members (`loop-detection`, trailing `next-hop:auto`) | Every implied endpoint, reception, send and inserted filter member (plugin and engine-registered, in chain position with its `implied-by` reason) is shown on the CLI (three config commands), the editor, the web inspection, the workbench, REST, gRPC and MCP, and matches what runs. A human display carries the notice; machine outputs do not | A written endpoint or filter member appears once, as written. gNMI `Get` stays written-only |
| AC-7 | A running daemon with edges of every origin: an operator flow, a registered reception, a filter owner's `state`, and a reception whose publisher is absent | `show event delivery` lists each edge once with its origin, across every namespace, and `ze config graph` draws the same set | None: every edge carries an origin |
| AC-8 | `show config source <file> \| implied` with no daemon running, for a file with an external endpoint: with a program that answers in query mode, and with a `run` line that names no program | The first shows the program's registered receptions and sends as implied, and its run loop never starts. The second is refused, naming the endpoint, the `run` line, the cause and the conformance command | None |
| AC-9 | `show config running \| implied \| json`, and the same commands without `\| implied` | With it, the envelope holds `config` and `implied`. Without it, the output is byte-identical to today's and holds no notice and no `implied` key | None |
| AC-10 | A dynamic group whose members need an implied endpoint | The endpoint is loaded and shown | None |
| AC-11 | An endpoint with an empty `send { }`, and a flow with no `event` entry, viewed with `\| implied` in text and JSON | Each is shown as written, distinct from an absent one. No send is shown as implied for that plugin, and no reception for that pair | None |
| AC-12 | A route server with no send list and three connected peers, viewed with `\| implied` and `show event delivery` | Its send toward `bgp` is shown once, as registered, for the plugin, not once per peer, and every connected peer appears as a subject it serves | None |
| AC-13 | A plugin with a registered reception and no flow, and an ExaBGP bridge with two scripts of which one has a flow, viewed with `\| implied` in text and JSON; then the implied flow text pasted back as a written flow | Each implied reception is printed as a flow with `to` and `match` in the stored path form. The script with a flow shows that flow as written and no implied reception; the other script shows its implied reception under its own path. The pasted flow commits, and the item is then shown once, as written. `show event delivery` names the receiver path and the matched path on each edge | A dynamic peer's edge shows its admitting group's path |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Commits a config with `ntp { }` and a redistribution rule, runs `show config running \| implied`, and opens the workbench | Commit -> requirement check -> resolved edges -> text, envelope and workbench rows | `test/plugin/bus-implied-view.ci`, `test/web/bus-implied-workbench.wb` (planned) |
| 2 | Narrows the Loc-RIB below what a rule needs and commits | Commit -> requirement check -> refusal with the typed fix -> the fix applied -> load | `test/parse/bus-requirement-refusals.ci` (planned) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestImpliedViewMatchesResolvedEdges` | `internal/component/plugin/server` | AC-6, R-1: the view and the runtime read one set; the parity assertion | |
| `TestRequirementCheckRefusals` | `internal/component/plugin/server` | AC-2, AC-3, AC-4: one case per feature, each asserting the fix line | |
| `TestAIGPDefaultActivatesNoRequirement` | `internal/component/bgp/config` | AC-5 | |
| `TestAIGPDefaultPathStillSelectsByAIGP` | the Loc-RIB plugin's package | AC-5 companion assertion on the default configuration | |
| `TestNoPluginNameLiteralInBGPConfig` | `internal/component/bgp/config` | The three literals are gone; `./le arch enumeration check` judges the same | |
| `TestDeliveryEdgeOrigins` | `internal/component/plugin/server` | AC-7 | |
| `TestImpliedReceptionPrintsAsPathFlow` | `internal/component/config/cli` | AC-13: an implied reception renders as a flow in the stored path form, per receiver instance, and the rendered text parses and commits as a written flow. Negative: a receiver instance with a written flow shows no implied reception | |
| `TestImpliedEnvelopeKeepsConfigPure` | `internal/component/config/cli` | AC-9, the JSON negative | |
| `TestImpliedTextRoundTrips` | `internal/component/config` | R-3: the text with comments parses back to the written config | |
| `TestOfflineImpliedRefusesUnknownDeclaration` | `internal/component/config/cli` | AC-8 | |
| `TestDynamicGroupImpliedEndpoints` | `internal/component/bgp/config` | AC-10 | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | No numeric inputs introduced | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bus-implied-view` (planned) | `test/plugin/` | AC-1, AC-6, AC-9, AC-11 through the running daemon | |
| `bus-requirement-refusals` (planned) | `test/parse/` | AC-2, AC-3, AC-4, AC-5 | |
| `bus-delivery-origins` (planned) | `test/plugin/` | AC-7 | |
| `bus-implied-offline` (planned) | `test/parse/` | AC-8 | |
| `bus-implied-editor` (planned) | `test/editor/` | AC-6: editor `show \| implied` and the status-bar notice | |
| `bus-implied-workbench` (planned) | `test/web/` | AC-6: the inspection view and the read-only workbench rows | |

## Files to Modify
- `internal/component/plugin/registry/`, `pkg/plugin/rpc/dissemination.go` - the requirement declaration, generalizing `DisseminationDecl`
- `internal/component/plugin/server/delivery_graph.go`, `delivery_reconcile.go`, `dissemination_validate.go`, `startup_autoload.go` - the origin field, the requirement check, the implied endpoint list
- `internal/component/bgp/config/redistribute_binding.go`, `flowspec_binding.go`, `filter_state_binding.go` - delete the plugin-name literals and constants; the plugins declare
- `internal/component/bgp/reactor/config_aigp.go` - the written-only trigger
- `internal/component/command/pipe_filter.go`, `internal/component/config/cli/running_rpc.go`, `internal/component/cli/model_load.go` - the `implied` operation, the text renderer, the notice
- `internal/component/cmd/show/show.go` - `show event delivery`
- `internal/component/web/graphical_inspection.go` and the workbench handlers and `.templ` views - the inspection word and the workbench rows
- `docs/architecture/api/architecture.md`, `docs/architecture/api/commands.md`, `docs/guide/command-reference.md`, `docs/guide/configuration.md` - the implied view, the requirement refusal, the origins

## Files to Create
- `test/plugin/bus-implied-view.ci`, `test/plugin/bus-delivery-origins.ci`, `test/parse/bus-requirement-refusals.ci`, `test/parse/bus-implied-offline.ci`, `test/editor/bus-implied-editor.et`, `test/web/bus-implied-workbench.wb`

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the three failing functional tests of the Wiring Test table.
2. **Phase: Requirement declaration and check** -- declare, check, refuse; delete the literals; AIGP trigger.
3. **Phase: Origin and inspection** -- the origin field, `show event delivery`, `ze config graph`.
4. **Phase: Implied view** -- the pipe operation, the renderers, the notice, the offline refusal.
5. **Phase: Web** -- the inspection and the workbench.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Bare `\| implied` pipe (Q4, 2026-11-23) | `\| display implied`; `\| implied [only]` | `display` selects row fields; a bare operation shows written and implied together and composes with `\| match` |
| A `# implied by:` comment and a JSON and YAML envelope (Q4, 2026-11-23) | `##` marker; implied keys merged into the config | `##` at column 0 breaks format detection; the envelope keeps the written config pure |
| Same-line trailing `# implied:` comment on modified lines (2026-11-23) | Comment above the modified line; splitting `filters [ ... ];` across multiple lines | Keeps the existing line shape intact while naming exactly which tokens were added and why ("No, it should be on the same line and say what was added") |
| Read-only implied rows in the workbench; gNMI `Get` written-only (Q5, 2026-11-23) | Inspection only; implied items in gNMI | Workbench users see everything; `Get` then `Set` round-trips |
| No resolver in this spec; the view reads Child 1's edges (2026-11-23, after R1) | A pure resolver over the config tree shared by runtime and display | R1 made the defaults a registration Child 1 runs; a second computation could differ from it |
| One declared requirement for features and roles | Keep `DisseminationDecl` and add a second declaration | One check and one message shape |

## Known Limitations
- gNMI `Get` returns the written configuration only.
- The offline implied view refuses a config whose external program cannot be asked for its declaration.
- The check on every command a plugin sends was owned by `spec-bus-3-external-command-gating` (superseded by closed `spec-explicit-plugin-and-attach-config`).

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-13 all demonstrated
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: scoped package tests with `./le test unit pkg`, and `./le test functional parse`, `plugin`, `editor`, `web`
- [ ] `./le arch enumeration check` finds no plugin-name literal in `internal/component/bgp/config/`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Functional `.ci` tests for end-user behavior

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean
- [ ] **Commit A:** code + tests + docs + edited spec
- [ ] **Commit B:** `remove plan/release/fixes/spec-bus-2-implied-display-and-grant-refusals.md` only
