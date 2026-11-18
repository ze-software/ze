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
| Updated | 2026-11-17 |

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
not write. Under the bus model below, read "binding" as a flow on a peer's
list and "plugin" as an endpoint; the flows D-6 names are the one exception,
and they are shown. Where the configuration asks for a feature and lacks what the
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

### The bus model (owner decisions, 2026-11-17)

Thomas ruled that `attach process` comes from ExaBGP's process concept and no
longer fits. One block does three jobs: it names a program (and can start one
inline), it grants events, and it grants the right to send. It exists only
under a BGP peer, while the bus carries many namespaces whose flows no config
can state. The configuration states two things apart: who is registered on the
bus, and what moves between two registered parties.

| # | Decision | Alternative set aside |
|---|----------|-----------------------|
| D-4 | The config states an endpoint (who is on the bus) apart from a flow (what moves between two endpoints). `attach process` is replaced whole, its inline `run` included | Keep `attach process` and add namespaces to it: the three jobs stay in one block |
| D-5 | A peer or group switches a flow on by listing its name. A flow carries no peer selector. A group's list reaches its members, and a member that restates the list replaces it for itself, as every leaf-list in a group merges today | A peer selector inside the flow: one place to read, but a second selector language, and a dynamic peer is reachable only by its group |
| D-6 | A flow that leaves the operator no choice (the Loc-RIB to the system RIB, the system RIB to the FIB) is declared by the plugin and shown by a command. The operator does not write it | The operator writes every flow: each config in the repository carries lines that can take one value only |
| D-7 | The `plugin { }` block becomes the registration half of a `bus { }` block, and the hub listener moves under `bus` with it. The repository's configs are converted directly (`ai/META-DECISIONS.md`, "Ze is pre-release") | Keep `plugin { }` and add only the flows: fewer configs change, and two words name one thing |
| D-8 | A flow's command list is the permission for every command an external program sends, not only for a message toward a peer | Keep today's reach: only `update`, `refresh` and `raw` toward a peer are permissions |

The model, in the operator's words:

| Concept | What it states | Where it is written |
|---------|----------------|---------------------|
| Endpoint | One party on the bus, under a name the operator chooses: a built-in plugin run in the daemon, or a program Ze starts | `bus { endpoint <name> { ... } }` |
| Flow | One named relationship between two parties: the events that go from the first to the second, and the commands the second may send to the first | `bus { flow <name> { ... } }` |
| Flow list | Which flows a scoped publisher instance takes part in | `flow [ <name> ... ]` on a BGP group or peer |

Fields of an endpoint:

| Field | Type | Description |
|-------|------|-------------|
| `name` | string, the list key | The name flows refer to, in the form the `external-plugin-name` validator accepts today |
| `internal` | string | The registry name of a built-in plugin to run in the daemon. Exclusive with `external`. It takes the place of the `use` leaf, which both lists carry today |
| `external` | container holding `run`, `respawn`, `timeout` | The program Ze starts, with the settings the `external` list carries today |
| `encoding` | `json` or `text` | The encoding of what this endpoint is sent, the `encoder` leaf of today |

Fields of a flow:

| Field | Type | Description |
|-------|------|-------------|
| `name` | string, the list key | The name a flow list refers to |
| `from` | string | The publishing party: a subsystem that owns an event namespace, or an endpoint |
| `to` | string | The receiving endpoint |
| `event` | list of string | Event types that go from `from` to `to`, each checked against what `from` registered. The direction suffixes and `*` keep their meaning |
| `command` | list of string | Command types `to` may send to `from`, each checked against what `from` registered |

Before, as an operator writes it today:

```
plugin {
    internal rib { use bgp-rib; }
    external collector { run ./collector.py; encoder json; }
}
bgp {
    group transit {
        attach process rib { receive [ update state refresh ]; send [ update ]; }
        attach process collector { receive [ update-received state ]; }
        peer transit-c {
            attach process rib { receive [ update state refresh ]; send [ update ]; }
        }
    }
}
```

After:

```
bus {
    endpoint rib { internal bgp-rib; }
    endpoint collector { external { run ./collector.py; } encoding json; }

    flow peer-routes { from bgp; to rib; event [ update state refresh ]; command [ update ]; }
    flow watch { from bgp; to collector; event [ update-received state ]; }
}
bgp {
    group transit {
        flow [ peer-routes watch ];
        peer transit-c { flow [ peer-routes ]; }
    }
}
```

D-1 to D-3 hold unchanged under this model, and their refusals get shorter.
Silence stays the default: a flow no peer lists moves nothing. Delivery stays
the overlap of what the config grants and what the receiving party subscribed
to. A feature whose plugin needs a flow on a peer is refused when the peer's
list lacks it, or when the flow grants less than the plugin declared, and the
fix is the one line that lists or completes the flow. A plugin the config
needs is an `endpoint` entry.

This spec stays a skeleton. The config model above is ruled. The questions
under "Design work owed" are not answered, and the design phase owes the
census of every reader and writer of the changed config before the status
moves to `design`.

### Design work owed

| # | Question | What settles it |
|---|----------|-----------------|
| W-1 | What `from` may name, and how an endpoint under an alias resolves to the namespace its plugin registered | The namespace registry (`internal/core/events`) and `processNameFor` |
| W-2 | How a plugin declares the flow it needs, so the commit check reads it without naming the plugin | `Registration.Dissemination` already carries a namespace and the complete receive and send sets; whether it becomes the one declaration, in place of the constants `locRIBReceive`, `orchestratorReceive` and `orchestratorSend` |
| W-3 | Which commands a program sends are gated today, and where D-8's check runs. Today the one gate is `filterPermittedPeers`, on messages toward a peer, and an operator is exempt | `send_permission.go`, and the plugin-to-engine dispatch in `plugin/server` |
| W-4 | How an `event` or `command` name is checked at commit when `from` or `to` is an external program, which declares after the config is parsed | A-2, and the declare-mode publish at `ze start` |
| W-5 | The command that shows declared and written flows, and whether `show event delivery` becomes it | `delivery_graph.go` `DeliveryGraph` |
| W-6 | What the implicit state grant (Q-2) and the OPEN observer's removal grant become under flows | `filter_state_binding.go`, `addOpenRemovalObservers` |
| W-7 | What `ze exabgp migrate` writes: endpoints, flows and per-peer lists for an ExaBGP `api { processes [ ... ] }` | `internal/exabgp/migration` |
| W-8 | How a plugin loaded with `--plugin` is named as an endpoint | `docs/guide/plugins.md`, "Reading the delivery graph" |
| W-9 | Where the hub listener and its client block sit under `bus`, and what the managed-client config server reads | `ze-plugin-conf.yang`, container `hub` |
| W-10 | Whether a per-attachment encoding is honored today. The `encoder` leaf's description says the binding's value wins; `architecture.md`, "Encoding", says the `content` block parses and is not read. One of the two is wrong, and it decides what dropping `content` removes | `parseProcessBindingsFromTree` stores it; read every reader of `ProcessBinding.Encoding` and `Format` |
| W-11 | What a custom event or send type in a flow auto-loads. Today a `receive` or `send` token no declared plugin produces loads the plugin that does (`getUnclaimedEventTypePlugins`, `getUnclaimedSendTypePlugins`); D-3 makes that an `endpoint` entry the refusal names | `startup_autoload.go` |

### Plugin loading (owner decision, 2026-11-18)

| # | Decision | Alternative set aside |
|---|----------|-----------------------|
| D-9 | A feature's config block implies the requirement for the plugin that serves it, and the requirement is never hidden: it is an `endpoint` entry the config holds. The operator does not work it out or type it. A commit that lacks the entry is refused, names it, and offers to add it (AC-6). Nothing the config does not need is ever asked for | The block starts its plugin with no entry, and a command shows what runs: the requirement is then visible only to an operator who asks |

Thomas's words: "if you have a NTP block, it should automatically imply the
requirement for it. They should be written in a way where you are not hiding
the requirements. So it should suggest in commit that you need to add to the
bus automatically." The decision above is the session's reading of them: the
entry is written in the config, and the tool writes it on request.

One rule therefore covers every reason Ze loads a plugin today
(`internal/component/plugin/server/startup_autoload.go`): the plugin's own
config root, a root several plugins share, a configured family, an event or
send type in a flow, a dependency, and the writer of the chosen data plane.
Each becomes an `endpoint` entry the commit names and offers. The operator
writes `ntp { ... }`, commits, and is offered the one line:

```
bus {
    endpoint ntp { internal ntp; }
}
```

A dependency is offered with the plugin that needs it, in the same offer, so
one acceptance completes the config.

### Owner question the design cannot answer

| # | Question | Recommended | What the other answer changes |
|---|----------|-------------|-------------------------------|
| Q-1 | Does D-3 reach a plugin Ze loads because the operator wrote that plugin's own config root (`ntp { }` loads `ntp`), or only the loads the operator did not ask for by name? | Only the second. Writing a plugin's own root is the explicit request to run it. The loads to make explicit are the ones a root shared by several plugins causes (`bgp { }` loads every plugin that declares the `bgp` root), and the loads by family, event type, send type, dependency and data-plane choice | Reaching the first too means every feature needs two blocks, its own and a `plugin { }` entry, in every config the repository carries |
| Q-1 answer | Owner decision, 2026-11-18 (D-9 below). The recommendation in the row above is set aside: a feature's own block does not start its plugin in silence either | - | - |
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
- Plugin loading follows D-3 and D-9: no plugin is loaded without an `endpoint` entry, and the commit names and offers each missing one.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Derived process bindings | `internal/component/bgp/config/redistribute_binding.go` `wireRedistributeDelivery`, `grantEveryPeer` | adds four bindings the operator did not write, and names the Loc-RIB and orchestrator plugins from config code | Replaces: deleted, and the check below takes its place. Nothing derives a binding afterwards |
| Completeness refusal | `internal/component/bgp/config/flowspec_binding.go` `requireRIBDelivery` | refuses a narrow `bgp-rib` block for two features, with a fixed message | Extends: becomes the one check, fed by what each feature's plugin declares it needs, covering a missing block too and every feature |
| Operator precedence | `internal/component/bgp/reactor/config.go` `EnsureProcessBinding` | keeps the operator's block when one exists | Does not apply once nothing is derived; its rule, that a typed block is never widened, is what this spec generalizes |
| Implicit state grant | `internal/component/bgp/config/filter_state_binding.go` `grantFilterOwnersState` | widens a block by one type for a filter owner | Preserved, pending Q-2 |
| Delivery reconcile | `internal/component/plugin/server/delivery_reconcile.go` `reconcileDelivery` | warns in the log when a plugin's declared events and a peer's grant disagree | Extends: stays the runtime voice for an external plugin, which declares after the config is parsed; the commit-time refusal covers what is known at config time |
| Alias resolution | `internal/component/bgp/config/redistribute_binding.go` `processNameFor` | names the process a registry plugin runs under | Extends: the refusal names the operator's alias in the line it gives |
| Plugin auto-load | `internal/component/plugin/server/startup_autoload.go` `getConfigPathPlugins` and its siblings | loads plugins by config root, family, event type, send type, dependency and data plane | Replaces, per D-9: every load becomes a named and offered `endpoint` entry. What the loader knows today, which plugin each root, family, type, dependency and data plane needs, is what the refusal reads to name the entries: a load the operator did not write becomes a refusal naming the `plugin { }` entry to add |
| Typed fix of a refusal | `internal/component/cli/fix_frame.go` `FixCommand`, `FrameFix`; `docs/architecture/cli/error-surface.md` | a refusal carries its fix as a typed editor command (verb, position, arguments, and which arguments are placeholders), and every surface prints it as the exact command for the operator's position. Nothing applies it | Extends: the grant and plugin refusals carry their missing lines as these typed fixes, and the editor gains the step that applies a refusal's concrete fixes to the candidate on the operator's request, on the SSH editor and the web editor alike (`ai/doors/commands-cli.md`, Traps: one surface fixed, the other not). No second fix format |
| Per-peer attachment | `internal/component/bgp/yang/ze-bgp-conf.yang` container `attach`, list `process`; `internal/component/bgp/reactor/config.go` `parseProcessBindingsFromTree` | one block per program on a peer or group, with `receive`, `send`, an inline `run` and a `content` block | Replaces (D-4): the flow list takes its place, the flow carries the two lists, and `run` and `content` go (W-10) |
| Program declaration | `internal/component/plugin/yang/ze-plugin-conf.yang` lists `internal` and `external`, container `hub`; `internal/component/config/loader.go` `ExtractPluginsFromTree` | declares each program once and its encoding. An `external` block can also run a built-in plugin through `use`, and the BGP extractor adds programs declared inline on a peer | Replaces (D-7): the same facts under `bus { endpoint }`, one list where two existed, and no inline declaration |
| Declared grant needs | `pkg/plugin/rpc/dissemination.go` `DisseminationDecl`; `internal/component/plugin/registry/registry.go` `Registration.Dissemination` | a plugin declares a namespace, a role and the complete receive and send sets it needs, and a declaration grants nothing | Extends (W-2): the candidate carrier for what a flow must hold; the refusal then names a flow |
| Delivery graph | `internal/component/plugin/server/delivery_graph.go` `DeliveryGraph`; `show event delivery` | builds peer-to-process edges from each peer's resolved settings and decides delivery | Extends: built from each peer's resolved flow list, and it gains the declared flows of D-6 (W-5) |
| Group merge of a peer's lists | `internal/component/bgp/config/resolve.go` `ResolveBGPTree` | a member inherits its group's blocks, a restated block replaces them, a dynamic peer carries its group's (`docs/architecture/api/architecture.md`, "Groups and Dynamic Peers") | Extends unchanged (D-5): the flow list is one more inherited list |
| Runtime subscription | `pkg/plugin/sdk/sdk_callbacks.go` `SetStartupSubscriptions` | the receiving party says what it wants, per namespace | Does not change: it stays the receiver's half, and the flow is the operator's half |
| Event namespace registry | `internal/core/events` `RegisterNamespace`, `Register` | each publisher registers its namespace and event types | Extends: the source of truth an `event` name is checked against, for every namespace and not only BGP |
| Send permission | `internal/component/bgp/reactor/send_permission.go` `Peer.maySend`, `filterPermittedPeers` | a process's message toward a peer is refused unless that peer's block grants the type; an operator is exempt, and a command with no named sender is refused | Extends (D-8): the flow's `command` list is the grant, and it reaches every command an external program sends (W-3) |
| ExaBGP migration | `internal/exabgp/migration` | writes the Ze config of a migrated ExaBGP file | Extends (W-7): writes endpoints, flows and flow lists |
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
| R-6 | The syntax change reaches every config, `.ci` body, fixture, lab scenario and doc the repository carries that declares a plugin or attaches one | the count of files that hold `plugin {` or `attach process` | Convert with a tool that rewrites the parsed tree, never by text surgery (`ai/rules/config.md`), in the same work as the parser change (`ai/META-DECISIONS.md`, "Ze is pre-release") |
| R-7 | The operator contract changes: the guide, the plugin-development pages, the website and the refusal texts that give an `attach process` line all name the old form | `./le doc check all` and a search for the old keywords after conversion | Each page is edited in the same work as the code that makes it wrong |
| R-8 | A release fix grows into a syntax redesign, and the fix waits on it | the spec stays open while D-2's refusal is unimplemented | Phase the work: the syntax and its conversion first, the refusals on top; split into child specs if the phases cannot land apart |
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

AC-1 to AC-7 were written before the bus model. Where a row says an `attach
process` block or line, read a flow on the peer's flow list; where it says a
`plugin { }` entry, read an `endpoint` entry. The design phase rewrites them.

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A redistribution rule importing from BGP; one peer has no `attach process` block for `bgp-rib` | Validate, commit and start each refuse with a message naming the rule, the peer, the process under its alias, and the `attach process` line that grants what is needed. With the line added, the config loads and the peer's routes are redistributed | A config with no such rule needs no block and loads |
| AC-2 | The same rule; the peer's block grants `receive [ state ]` only | The same refusal, naming the missing types. With them added, the config loads and redistributes | The block is never widened: the operator's `state`-only block stays as written until the operator changes it |
| AC-3 | A redistribution rule with BGP as destination; a peer's block for the orchestrator lacks `send [ update ]`, or is missing | Refused with the line; with it added, a redistributed route reaches that peer | Peers the rule does not feed need nothing |
| AC-4 | A FlowSpec family the operator configured, or AIGP the operator set explicitly, on a peer whose `bgp-rib` block is missing or narrow | Refused in the same message shape as AC-1, where today a missing block is derived. With the line added, the config loads and the feature works on that peer | The existing refusal of a narrow block keeps refusing |
| AC-7 | Internal peers with AIGP left at its RFC 7311 default, an API process attached with `send [ update ]`, and no `bgp-rib` block on any peer | The config loads at validate, commit and start, and the peers receive the API announcements | The config survives because a default-on capability asks for nothing. Setting AIGP explicitly on a peer, or adding a partial `bgp-rib` block, moves that peer to AC-4 |
| AC-5 | A config holding a feature block whose plugin has no `endpoint` entry: the plugin's own root (`ntp { }`), a root several plugins share, a configured family, an event type in a flow, and a plugin whose dependency has no entry | Each is refused at validate, commit and start, naming every missing `endpoint` entry, dependencies included, and the interactive commit offers them in one offer (AC-6). With the entries added, the config loads and each plugin runs | An entry is asked for only when the config holds what needs it: a config with no `ntp` block is never asked for an `ntp` endpoint |
| AC-6 | An interactive editor session commits the AC-1 config and is refused | The refusal lists the missing lines and offers to add them. Accepting adds exactly those lines to the candidate, shows them as the pending change, and commits nothing: the operator's next commit applies them and succeeds. Declining leaves the candidate untouched | A fix that holds a placeholder is printed and never offered, since Ze cannot know the value. `ze config validate`, the daemon's start, and the API surfaces print the same lines, in a form a program can read, and change no configuration |
| AC-8 | A config in the "After" form of the Task section, with two peers in the group | `rib` receives the listed events of both peers and may send `update` to both; `collector` receives the events of the peer that lists `watch` and none of `transit-c` | A flow no peer lists moves nothing |
| AC-9 | A peer created by a dynamic group whose list names a flow | The peer takes part in that flow from its first session | none |
| AC-10 | A flow whose `event` or `command` names a type its `from` party did not register, or whose `from` or `to` names nothing | Validate, commit and start refuse, naming the flow, the word, and the names that are accepted | A type an external program declares, within what W-4 settles |
| AC-11 | A running daemon whose plugins declare flows the operator did not write (D-6) | The command W-5 names lists them, marked as declared, beside the written ones | none |
| AC-12 | An external program sends a command its flows do not list | The command is refused, the program is told which flow line grants it, and a counter records the refusal | An operator at the CLI or an API is not a program and is not gated by a flow |
| AC-13 | An ExaBGP config with a process and a neighbor that names it, through `ze exabgp migrate` | The output holds an endpoint, a flow and the neighbor's flow list, loads, and the script is fed and may announce as before | none |
| AC-14 | Every config, `.ci` body, fixture and lab scenario in the repository | None holds `plugin {` or `attach process`, and each loads in the new form | A quoted old form inside a `plan/` record |

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
- `internal/component/plugin/server/startup_autoload.go` - per D-9: the loads become the source of the refusal's named entries
- `docs/architecture/api/architecture.md`, `docs/architecture/core-design.md` - the derived-binding sections
- `internal/component/plugin/yang/ze-plugin-conf.yang`, `internal/component/bgp/yang/ze-bgp-conf.yang` - `bus { endpoint, flow }` and the flow list replace `plugin { }` and `attach process`
- `internal/component/config/loader.go`, `internal/component/bgp/reactor/config.go` - read endpoints and resolve each peer's flow list
- `internal/component/bgp/reactor/send_permission.go` - the grant and its refusal text name a flow
- `internal/exabgp/migration` - writes the new form
- `docs/guide/plugins.md`, `docs/plugin-development/` - the operator and author pages for both blocks

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
| Endpoints and flows, a peer lists flow names, intrinsic flows declared, `bus` replaces `plugin`, the command list gates every external command (owner decisions D-4 to D-8, 2026-11-17) | each row of the D-4 to D-8 table names the alternative set aside | One block did three jobs and existed only under a BGP peer. Naming a flow once removes the repeated lists, keeps group inheritance, and lets the same statement serve every namespace on the bus |
| Explicit, or refused at commit with the fix (owner decisions D-1 to D-3) | derive the missing configuration; warn and load | Explicit configuration prevents surprises; a refusal that gives the line costs one paste, where a silent load costs an investigation |

## Known Limitations

- The flow list is applied to BGP groups and peers only. The bus carries other
  scoped publishers (an interface, an L2TP session); this spec gives them no
  flow list.
- Encoding stays one value per endpoint, and a flow carries no format of its
  own (W-10).

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
