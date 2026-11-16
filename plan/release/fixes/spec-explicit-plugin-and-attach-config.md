# Spec: explicit-plugin-and-attach-config

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | config |
| Priority | P1 |
| Release reason | Configuration correctness of shipped behaviour: a config that asks for a feature and grants its process too little loads and silently does nothing, and Ze adds process bindings and loads plugins the operator never wrote |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Design tier | strongest tier |
| Implementation tier | development tier |
| Updated | 2026-11-16 |

Bucket: `plan/release/fixes/`. `plan/README.md`, "Split fixes from features":
"Accepted but ineffective configuration is not an absent feature: its incorrect
behavior still needs a release owner."

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Three owner decisions, 2026-11-16, made while reviewing
`plan/release/fixes/spec-bgp-best-route-advertised-to-peers.md`:

| # | Decision | Recorded in |
|---|----------|-------------|
| D-1 | What Ze does for an operator is what the operator configured. Explicit configuration prevents surprises, its friction is the accepted tradeoff, and tooling carries the help | `ai/META-DECISIONS.md`, "Code and dependencies" |
| D-2 | A feature whose process is granted less than it needs is refused at commit. The check detects a missing block and a narrower one alike, and the refusal gives the line that fixes it | `docs/architecture/api/architecture.md`, "Owner ruling: explicit, or refused at commit" |
| D-3 | The same holds for the content of the `plugin { }` block: a plugin the config needs is declared there, and a commit that needs one it does not declare is refused with the line that declares it | the same section |

Goal: Ze adds no `attach process` binding and loads no plugin the operator did
not write. Where the configuration asks for a feature and lacks what the
feature needs, `ze config validate`, the editor's commit and the daemon's start
refuse it with one message that names the feature, the peer or plugin, and the
exact line to add.

What asks for a feature is what the operator wrote, and only that. A
capability that is on by default asks for nothing. AIGP is the case that
matters: RFC 7311 Section 3.3 enables it on every internal session
(`internal/component/bgp/reactor/config_aigp.go` `AIGPEnabled`), so today any
config with one internal peer derives the Loc-RIB's binding on every peer and
is then judged by the AIGP completeness check. Under D-2 that would refuse
every internal peer that attaches no RIB, including a peer that only receives
API announcements. So a default is never a trigger:

| The config holds | Is a RIB grant needed on that peer? |
|------------------|-------------------------------------|
| AIGP at its default, and no `bgp-rib` block on the peer | No. AIGP selection has no RIB to act in, and the config loads |
| AIGP set explicitly on the peer, and no `bgp-rib` block | Yes: the operator asked for AIGP selection, and the commit is refused with the line |
| A `bgp-rib` block on the peer that grants less than the plugin's declared set, whatever AIGP is | Yes: a partial block is refused with the line that completes it |
| A FlowSpec family the operator configured on the peer | Yes: the family is written by the operator, and RFC 8955 Section 6 authorization needs the RIB |

This spec is a skeleton. It records the decisions, the behaviour read at the
producing code, and the questions the design must answer. No design is
proposed yet.

### Owner question the design cannot answer

| # | Question | Recommended | What the other answer changes |
|---|----------|-------------|-------------------------------|
| Q-1 | Does D-3 reach a plugin Ze loads because the operator wrote that plugin's own config root (`ntp { }` loads `ntp`), or only the loads the operator did not ask for by name? | Only the second. Writing a plugin's own root is the explicit request to run it. The loads to make explicit are the ones a root shared by several plugins causes (`bgp { }` loads every plugin that declares the `bgp` root), and the loads by family, event type, send type, dependency and data-plane choice | Reaching the first too means every feature needs two blocks, its own and a `plugin { }` entry, in every config the repository carries |
| Q-2 | Does D-1 reach the filter-state grant? | No. It grants only `state`, to a plugin whose filter the operator already named in that peer's policy, so the plugin can free per-peer memory. Naming the filter is the explicit decision | Removing it makes the operator repeat the plugin in an `attach process` block on every peer whose policy names its filter |

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/api/architecture.md` - "Attaching a Process to a Peer", "The Two Bindings Ze Derives", "The Implicit State Grant"
  → Constraint: "A peer that attaches no block for a program is fed to it never, and refuses every message the program aims at that peer. Silence is the default, and the config is what breaks it. This holds for ze's own plugins as it holds for an operator's program"
  → Constraint: the page lists two derived bindings; the code derives four (FlowSpec and AIGP are missing from the page), a page defect this spec repairs when it removes them
- [ ] `docs/architecture/core-design.md` - "The Two Deliveries a Redistribution Rule Derives", protocol-agnostic plugin loading
- [ ] `docs/architecture/api/process-protocol.md` - plugin auto-loading
- [ ] `docs/architecture/config/yang-config-design.md` - "Where a validator actually runs"
  → Constraint: `ze config validate` never calls a live `OnConfigVerify` (`ai/doors/config-yang.md`, Traps). A refusal that must hold at validate, commit and start lives where all three run it

**Key insights:**
- Ze never widens a block the operator wrote. The defect is uneven telling: two features refuse a narrow block, one loads and moves nothing.
- The four derived bindings name plugins from config code, which `ai/META-DECISIONS.md` forbids for a new design.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/config/redistribute_binding.go` - `wireRedistributeDelivery` derives, on every peer, a `bgp-rib` receive binding for a redistribution rule that imports from BGP, for a FlowSpec family and for AIGP, and a receive-state and send-update binding for the redistribution orchestrator when BGP is a destination. `grantEveryPeer` adds them; `processNameFor` resolves the operator's alias
- [ ] `internal/component/bgp/reactor/config.go` - `EnsureProcessBinding` adds a derived binding only when the peer names no block for that process: "A binding an operator typed is the operator's decision about what that process is fed. A derived binding never widens it"
- [ ] `internal/component/bgp/config/flowspec_binding.go` - `requireRIBDelivery` refuses a config whose `bgp-rib` block on any peer lacks `update-received` and `state`, for FlowSpec authorization and AIGP selection only: "remove the restrictive attach binding or grant those events"
- [ ] `internal/component/bgp/config/filter_state_binding.go` - `grantFilterOwnersState` grants a peer's `state` to the plugin whose filter the peer's policy names, widening the operator's block for that one type, marked implicit
- [ ] `internal/component/plugin/server/delivery_reconcile.go` - `reconcileDelivery` logs one warning per peer, process and event type the plugin declared and the peer does not grant, and one for a process no peer attaches. It names no feature and gives no line for the narrow case
- [ ] `internal/component/plugin/server/startup_autoload.go` - `getConfigPathPlugins`, `getUnclaimedFamilyPlugins`, `getUnclaimedEventTypePlugins`, `getUnclaimedSendTypePlugins`, `appendDataPlaneWriter`: a plugin is loaded for a present config root it declares, for a configured family, event type or send type nobody claims, as a dependency, and as the writer of the configured data plane
- [ ] `internal/component/plugin/registry/registry.go` - `Registration.ConfigRoots`: "A present root auto-loads the plugin"

**Behavior to preserve:**
- A block the operator wrote is never widened.
- An operator's own process, and the ExaBGP bridge, are fed and may send only where a peer's `attach process` block grants it.
- FlowSpec authorization and AIGP selection keep refusing a `bgp-rib` block that grants too little.

**Behavior to change:**
- No `attach process` binding is derived. A redistribution rule, a FlowSpec family or AIGP whose process lacks a needed grant on a peer is refused, whether the block is missing or narrower.
- Redistribution joins FlowSpec and AIGP: a narrow block is a refusal, not a daemon that loads and redistributes nothing.
- One message shape for every such refusal, naming the feature, the peer, the process and the line to add.
- Plugin loading follows D-3, within the boundary Q-1 settles.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Derived process bindings | `internal/component/bgp/config/redistribute_binding.go` `wireRedistributeDelivery`, `grantEveryPeer` | adds four bindings the operator did not write, and names the Loc-RIB and orchestrator plugins from config code | Replaces: deleted, and the check below takes its place. Nothing derives a binding afterwards |
| Completeness refusal | `internal/component/bgp/config/flowspec_binding.go` `requireRIBDelivery` | refuses a narrow `bgp-rib` block for two features, with a fixed message | Extends: becomes the one check, fed by what each feature's plugin declares it needs, covering a missing block too and every feature |
| Operator precedence | `internal/component/bgp/reactor/config.go` `EnsureProcessBinding` | keeps the operator's block when one exists | Does not apply once nothing is derived; its rule, that a typed block is never widened, is what this spec generalizes |
| Implicit state grant | `internal/component/bgp/config/filter_state_binding.go` `grantFilterOwnersState` | widens a block by one type for a filter owner | Preserved, pending Q-2 |
| Delivery reconcile | `internal/component/plugin/server/delivery_reconcile.go` `reconcileDelivery` | warns in the log when a plugin's declared events and a peer's grant disagree | Extends: stays the runtime voice for an external plugin, which declares after the config is parsed; the commit-time refusal covers what is known at config time |
| Alias resolution | `internal/component/bgp/config/redistribute_binding.go` `processNameFor` | names the process a registry plugin runs under | Extends: the refusal names the operator's alias in the line it gives |
| Plugin auto-load | `internal/component/plugin/server/startup_autoload.go` `getConfigPathPlugins` and its siblings | loads plugins by config root, family, event type, send type, dependency and data plane | Replaces or narrows per Q-1: a load the operator did not write becomes a refusal naming the `plugin { }` entry to add |
| Typed fix of a refusal | `internal/component/cli/fix_frame.go` `FixCommand`, `FrameFix`; `docs/architecture/cli/error-surface.md` | a refusal carries its fix as a typed editor command (verb, position, arguments, and which arguments are placeholders), and every surface prints it as the exact command for the operator's position. Nothing applies it | Extends: the grant and plugin refusals carry their missing lines as these typed fixes, and the editor gains the step that applies a refusal's concrete fixes to the candidate on the operator's request, on the SSH editor and the web editor alike (`ai/doors/commands-cli.md`, Traps: one surface fixed, the other not). No second fix format |
| Absent-policy report | `internal/component/bgp/reactor/rfc8212_warning.go` `RFC8212Absences` | one value with message and editor fix, read by validate and the session log | Does not apply as the carrier: that is a warning for a legal config. Its shape, one value with the fix line read by every surface, is the model for the refusal |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: a commit in the editor, `ze config validate`, or the daemon's start.

### Transformation Path
1. [Design work: where each feature's plugin declares the grants it needs, as a pure value the config check reads without naming the plugin]
2. [Design work: the one check, in the walk that validate, commit and start all run]

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Plugin registration ↔ config check | [declared grant needs, read generically] | No |

### Changed-Wire Census

**Search evidence:** [owed at design: whether the declaration of grant needs travels in the Stage-1 declaration, which is a public wire]

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| [format] | [role] | [source site] | [search finding] | [disposition] |

### Integration Points
- [owed at design]

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
| A-1 | Every grant a feature needs is known at config time for a compiled-in plugin | `locRIBReceive` in `redistribute_binding.go` is a constant | the refusal cannot be complete at commit for that feature | design reads each feature's subscription | unvalidated |
| A-2 | An external plugin's needs are not known at commit, since it declares after the config is parsed | `delivery_reconcile.go`: "Config is parsed before an external program can declare anything" | the commit refusal could cover external plugins too | design reads the declare-mode path | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | Every config and test that relies on a derived binding or an auto-loaded plugin stops loading | a wave of refusals across `test/` | Convert each in the same work, with a generator that prints the missing lines for a config; D-1 names tooling as the help |
| R-2 | The refusal holds at commit and not at `ze config validate`, or the reverse | `test/plugin/config-validate-agrees-with-boot.ci` red | Put the check where all three doors run it |
| R-4 | A default-on capability is read as a request, and every internal peer without a RIB is refused | API-only configs with internal peers stop loading | A default is never a trigger (Task table); AC-7 is the positive case |
| R-5 | A migrated ExaBGP config that carries a FlowSpec family is refused, since ExaBGP has no RIB to attach | `./le test functional exabgp` reds on FlowSpec cases | `ze exabgp migrate` writes the `bgp-rib` blocks the family needs, in the same work; ExaBGP compatibility lives in the migration (`ai/META-DECISIONS.md`) |
| R-3 | A config gets stuck: the running daemon holds a config the new rule refuses | a reload after upgrade refuses | No release has shipped, so the repository's configs are converted directly and no migration path is written |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Configs that load today are refused, or a needed grant is still missed in silence |
| How is it reverted? | Restore the derivation and the auto-load; converted configs still load, since an explicit block always won |
| Who else touches this path? | `plan/release/fixes/spec-bgp-best-route-advertised-to-peers.md`, which uses this spec's refusal for transit |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| Commit of a redistribution rule importing from BGP, with no `bgp-rib` block on a peer | → | the one grant check | `test/parse/attach-missing-grant-refused.ci` (planned name) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A redistribution rule importing from BGP; one peer has no `attach process` block for `bgp-rib` | Validate, commit and start each refuse with a message naming the rule, the peer, the process under its alias, and the `attach process` line that grants what is needed. With the line added, the config loads and the peer's routes are redistributed | A config with no such rule needs no block and loads |
| AC-2 | The same rule; the peer's block grants `receive [ state ]` only | The same refusal, naming the missing types. With them added, the config loads and redistributes | The block is never widened: the operator's `state`-only block stays as written until the operator changes it |
| AC-3 | A redistribution rule with BGP as destination; a peer's block for the orchestrator lacks `send [ update ]`, or is missing | Refused with the line; with it added, a redistributed route reaches that peer | Peers the rule does not feed need nothing |
| AC-4 | A FlowSpec family the operator configured, or AIGP the operator set explicitly, on a peer whose `bgp-rib` block is missing or narrow | Refused in the same message shape as AC-1, where today a missing block is derived. With the line added, the config loads and the feature works on that peer | The existing refusal of a narrow block keeps refusing |
| AC-7 | Internal peers with AIGP left at its RFC 7311 default, an API process attached with `send [ update ]`, and no `bgp-rib` block on any peer | The config loads at validate, commit and start, and the peers receive the API announcements | The config survives because a default-on capability asks for nothing. Setting AIGP explicitly on a peer, or adding a partial `bgp-rib` block, moves that peer to AC-4 |
| AC-5 | A config that needs a plugin its `plugin { }` block does not declare, within the Q-1 boundary | Refused with the `plugin { }` entry to add; with it added, the plugin runs | A plugin loaded by its own config root, if Q-1 keeps that |
| AC-6 | An interactive editor session commits the AC-1 config and is refused | The refusal lists the missing lines and offers to add them. Accepting adds exactly those lines to the candidate, shows them as the pending change, and commits nothing: the operator's next commit applies them and succeeds. Declining leaves the candidate untouched | A fix that holds a placeholder is printed and never offered, since Ze cannot know the value. `ze config validate`, the daemon's start, and the API surfaces print the same lines, in a form a program can read, and change no configuration |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Writes a redistribution rule, commits, reads the refusal, pastes the line it gives, commits again | editor commit → grant check → refusal with fix → commit → redistribution | [owed at design] |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| [owed at design] | | | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| N-A | the feature takes no numeric input | | | |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `attach-missing-grant-refused` (planned) | `test/parse/` | AC-1 and AC-2 | |

## Files to Modify
- `internal/component/bgp/config/redistribute_binding.go`, `internal/component/bgp/config/flowspec_binding.go` - derivation removed, one check
- `internal/component/plugin/server/startup_autoload.go` - per Q-1
- `docs/architecture/api/architecture.md`, `docs/architecture/core-design.md` - the derived-binding sections

## Files to Create
- `test/parse/attach-missing-grant-refused.ci` - functional test

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| [owed at design] | | |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| [owed at design] | | | |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the functional test above, red at HEAD
2. [owed at design]

## Design Insights

- Ze already has the right rule in one place: `requireRIBDelivery` refuses a
  narrow block with the reason. The work is making that the only behaviour.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| The commit offers the missing lines and the operator accepts them (owner question, 2026-11-16; proposed, not yet ruled) | add them without asking; print them only | Adding without asking is the derivation D-1 forbids. Printing only leaves a paste the tool can do. An offer keeps the decision with the operator: the lines enter the candidate, are shown as a pending change, and take a second commit |
| Explicit, or refused at commit with the fix (owner decisions D-1 to D-3) | derive the missing configuration; warn and load | Explicit configuration prevents surprises; a refusal that gives the line costs one paste, where a silent load costs an investigation |

## Known Limitations

- None yet.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: the scoped package tests of `internal/component/bgp/config` and `internal/component/plugin/server`, `./le test functional parse`, `./le test functional plugin` and `./le test functional reload`
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/release/fixes/spec-explicit-plugin-and-attach-config.md` only, in the same `./le commit create` script (commit A preserves the spec in history)
