# Spec: container-4-operator-surface

| Field | Value |
|-------|-------|
| Status | design |
| Scope | config |
| Priority | P3 |
| Release reason | Post-release feature, by owner ruling (umbrella O-3): Ze runs OCI images itself after the first release. This child is the operator's side of that feature. |
| RFC gaps | - |
| Depends | spec-container-1-runtime, spec-container-2-image-store, spec-container-3-network |
| Phase | - |
| Handoff | - |
| Design tier | development tier |
| Implementation tier | strongest tier |
| Updated | 2026-11-16 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The umbrella (`plan/post-release/features/spec-container-0-umbrella.md`) decides how Ze runs a container. It does not yet say what an operator writes, types and reads. Without that, the three sibling children (runtime, image store, network) each invent words for their own nodes and commands, and the feature reaches the operator as three vocabularies.

The owner ruled that the configuration tree and the commands follow the container model operators already know from an established router operating system: a named container with its image, networks, published ports, volumes, environment, resource limits and restart policy, and image management as operational commands. The operations and the tree are kept. The words follow Ze's grammar.

This child is the one table of those words, and the code that spans the three siblings:

- The complete schema of the `container` config root, node by node, with the sibling that realizes each node.
- The complete command set, with typed arguments, structured answers, pipes and behavior on every surface.
- Container state as the operator sees it, and what each listing shows.
- What `ze config validate` refuses with no daemon, and what a build without the gate answers.
- The doctor checks that span the siblings, every operator message with its fix, metrics, and the pages.

The interactive shell inside a container is not here. It belongs to the shell child, `spec-container-5-shell`.

### How the work divides between this child and its siblings

| Part | Designed here | Lands with |
|------|---------------|-----------|
| A config node | Its name, type, constraints, default and help (tables S-1 to S-3) | The sibling named in the node's row, when it implements the behavior. This child adds the nodes marked "operator surface" and proves the whole module against the tables |
| A command | Its path, leaves, answer shape and pipes (table C-1) | The sibling named in the command's row. This child lands the container listing, the detail, the log and its stream |
| The config verifier | Which rule is judged offline (table V-1) | The runtime child lands the registration and its verifier entry point. Each child adds the rules in its rows |
| State, doctor, metrics, pages | All of it | This child |
| The closing test of the whole container work (umbrella AC-14) | All of it (section X-1) | This child, as its last phase |

Order: this spec holds the one table of config words and command words. Its design is settled, and this spec reaches `ready`, before any sibling starts its wiring phase. Its own implementation phases come after the three siblings' phases, which is what the Depends row states. No sibling lists this spec in its own Depends row: a sibling reads these tables as a settled design, and waits for none of this child's code.

## Required Reading

### Architecture Docs
- [ ] `plan/post-release/features/spec-container-0-umbrella.md` - the approved design and the owner's rulings
  → Decision: limits are mandatory with defaults (R-2), so no node of this schema has a value that means "no limit"
  → Decision: a missing image is a state, never a commit refusal, so no verify rule may ask whether an image is in the store
  → Constraint: a container's link is an ordinary Ze interface and its translation is state Ze owns and shows, so the listings name Ze interfaces and never a second runtime's objects
- [ ] `ai/doors/system.md` - what every change respects
  → Constraint: every plugin also runs as a separate process, so a command that reads container state is answered inside the plugin's process, never by a daemon-side Go call into it
  → Constraint: a compile tag is a boundary. No always-on list (a validator section list, a code table, a help string) may learn the word `container`
- [ ] `ai/doors/config-yang.md`, `docs/architecture/config/yang-config-design.md` - schema and validation layers
  → Constraint: `ze config validate` never calls a live `OnConfigVerify`, and the in-process verifier does not run at daemon start. A cross-leaf rule is therefore one judging function called from both the in-process verifier and the plugin's configure and verify callbacks
  → Constraint: a `ze:validate` annotation runs only under a section named in the central `validatedSections` list. This schema uses native types, `leafref` and mandatory nodes only, so it needs no entry in that list
  → Decision: the mandatory check and the reference check walk every top-level section present, so `mandatory true` and `leafref` work under a new root with no central edit
  → Constraint: an on/off setting is a positive boolean. A flag with no value is a `presence` container or a boolean, never `type empty`, outside the two named exceptions
  → Constraint: a dimensioned value is an unsigned integer with a `units` statement and a unit-free name
- [ ] `ai/rules/config.md`, `ai/patterns/config-option.md` - node obligations
  → Constraint: every node carries a `ze:help` of at most 96 characters and 25 words, and a different `description`
  → Constraint: a leaf that names another config entry is a `leafref`, with no hand-written existence check
- [ ] `ai/doors/commands-cli.md`, `ai/rules/cli.md`, `docs/architecture/cli/command-verbs.md`, `docs/architecture/cli/root-namespace-grammar.md` - command grammar
  → Decision: the verb is chosen by the ordered test in "Choosing the verb", which asks `clear`, `create`, `update` and `delete` before `request`. An image fetch makes something exist, so it is `create`, and only the restart reaches `request`
  → Constraint: a command addressing one member of a set types it with a closed keyword. A mandatory leaf is its guard, as `request plugin restart` has it
  → Constraint: enumeration is spelled `list`, rendering and narrowing are pipes, and a set action has an explicit `all`
  → Constraint: a handler answers data, never text, and every argument it reads is a declared leaf
- [ ] `docs/architecture/api/commands.md` - sections "One-shot answer of a streaming command", "Which surfaces offer a command", "A plugin declares its own answer shape", "Per-command declarations", "MCP Task Methods" (the page is 3678 lines: these sections and lines 1 to 599 were read)
  → Decision: the log stream registers with `RegisterPluginStreamingHandler` and declares `OneShotSample`, so the one-shot surfaces answer the recent lines instead of refusing
  → Decision: each command declares its shape, its column order and its address fields, so the pipe layer knows each answer before the command runs
  → Constraint: a streaming prefix is matched as literal leading words, so the words of a stream command up to its last fixed word hold no operator value
- [ ] `docs/architecture/command-ownership.md`, `ai/doors/system-plugins.md`, `ai/rules/plugins.md` - ownership
  → Constraint: the plugin folder owns its commands, schema and doctor checks. Removing it removes every one of them
  → Constraint: a plugin's check over its live state runs in the `runtime` phase and is answered by its `OnDoctorCheck` handler
- [ ] `docs/architecture/doctor-and-health-checks.md`, `ai/rules/user-facing-errors.md` - doctor and messages
  → Constraint: every finding carries `fix`, the exact command alone. Every `Warn` or `Error` line names a doctor code a registered check declares
  → Constraint: a refusal is an operator error: the problem, why it matters, and the fix on the last line
- [ ] `docs/architecture/plugin/feature-gates.md` - section "Refusing a root whose plugin is compiled out"
  → Decision: the refusal in a build without the gate is produced by `config.KeywordHint` from the generated root table. This child writes no code for it and proves its text
- [ ] `ai/doors/api-surfaces.md`, `ai/doors/forwarding.md`, `ai/doors/plugin-infra.md`, `ai/doors/le-tooling.md`, `ai/doors/hub-engine.md`, `ai/doors/core.md`, `ai/doors/appliance.md` - surfaces, ownership of kernel state, the SDK, the gates, the boot promise, state and codes, the device
  → Constraint: no surface adapter gains a command. REST, gRPC, MCP and the web derive from the registry and the schema
  → Constraint: the report bus is process-local, so the plugin raises issues through the SDK's `RaiseIssue`
  → Constraint: what `ze config validate` accepts is what boots. A rule judged at one door only breaks that promise
- [ ] The operator-model research and the mechanisms research, in `tmp/session/2026-11-15-223847/scratch/` - the reference model's tree, semantics and commands, and its mapping onto Ze's rules
  → Decision: the reference model's main trap is a container that silently does not run when its image is absent. Ze names the image in the state and gives the fetch command in doctor

**Key insights:** (minimal context to resume after compaction)
- One plugin owns the `container` root. The siblings are parts of that one plugin, so they share one schema module, one verifier and one command schema.
- Every command is a YANG command node bound to a wire method that the plugin's package registers as a proxy to the plugin, as VRRP's commands are. The typed selector is the last words of the path (`name` then the value), the shape `show vrrp interface name` has. A stream's fixed prefix then holds no operator value.
- The firmware commands answer a long operation once, at its end, under the request's context. The image fetch does the same, adds `ze:task-support required` for MCP, and shows an in-flight fetch in the image listing.
- Cross-leaf rules live in one judging function with two callers: the in-process verifier (validate, editor commit, API commit) and the plugin's own configure and verify callbacks (start, reload).
- No value means "no limit", no commit writes into the operator's config, and no keyword shares a slot with a number.
- Existing leaf-lists and keyed lists in Ze's schema are named in the singular (`address`, `name-server`, `member`, `veth`, `table`). This schema follows the tree.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `internal/component/config/plugin_verify.go` - `VerifyPluginConfigMapTransition` calls each registration's `InProcessConfigVerifier` with the sections of the roots it owns, a deleted root as an empty object. Its comment states it calls no live `OnConfigVerify`
- [ ] `internal/component/config/compiled_out_root.go` - `KeywordHint` answers, for a keyword a gated plugin's config root holds, that the plugin is compiled out, with the build tags and the fix line
- [ ] `internal/component/plugin/server/handler.go` - `RegisterStreamingHandler` and `RegisterPluginStreamingHandler` store a handler under a literal prefix with its `OneShot` declaration. `matchStreamingCommand` matches the longest literal prefix, and `GetStreamingHandlerForCommand` judges the words after it against the model. `RPCRegistration.PluginCommand` makes a builtin a proxy to a plugin command
- [ ] `internal/component/command/grammar/checker.go` - `selectorKinds` holds `name`, `id`, `index`, `address`, `type`, `key`. `CheckNode` flags a mandatory free-form value on a non-selector node that still has keyword children (R6), and `CheckName` refuses a first word that is not a verb
- [ ] `internal/plugins/update-cmd/yang/ze-update-firmware-cmd.yang` - the firmware verbs sit under `update system firmware` as one command node each, with no argument leaf and no task annotation
- [ ] `internal/plugins/update-cmd/cmd/firmware.go` - `handleFirmwareDownload` calls the backend under the request's context and answers one result map when the download ends. No progress is reported during it
- [ ] `internal/plugins/vrrp/yang/ze-vrrp-cmd.yang` - `show vrrp interface name` is a command node named `name` that holds one leaf for the value. Its module description says its wire methods are registered as plugin proxies. `clear vrrp statistics` alone is refused, and `all` is its explicit form

**Behavior to preserve:**
- Every existing config root, command and doctor code. A config with no `container` root parses, validates and runs as today.
- The streaming registry's literal prefix match, the verifier doors and the gated-root hint are used as they are.

**Behavior to change:**
- None in existing code. Everything here is new, behind the container gate the runtime child declares.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| In-process config verifier | `internal/component/config/plugin_verify.go` `VerifyPluginConfigMapTransition`; `yang-config-design.md` "Layer 3" | Runs each registration's `InProcessConfigVerifier` over its own roots at validate, editor commit and API commit. Not at daemon start | Extends: the container plugin's verifier calls the one judging function of table V-1. The same function is called from the plugin's configure and verify callbacks, which covers start and reload |
| Mandatory and reference walks | `yang-config-design.md` "Where a validator actually runs" and "Layer 1" | Enforce `mandatory true` and `leafref` in every top-level section present, with the operator refusal built from the schema | Extends by declaration only: the schema states its required nodes and its one reference in YANG |
| Central custom-validator walk | `yang-config-design.md` "Where a validator actually runs" (`validatedSections`) | Runs `ze:validate` annotations only under listed sections | Does not apply: a gated plugin's root must not be named in an always-on list, so this schema declares no `ze:validate`. The hardware address form is a native pattern here |
| Compiled-out root hint | `internal/component/config/compiled_out_root.go` `KeywordHint`; `feature-gates.md` "Refusing a root whose plugin is compiled out" | Names the build tag when an unknown keyword is a gated plugin's root | Extends by registration only: the plugin's `ConfigRoots` entry puts `container` in the generated table. This spec proves the text |
| Streaming registry and one-shot declaration | `internal/component/plugin/server/handler.go` `RegisterPluginStreamingHandler`, `OneShotSample` | Streams on the terminal surfaces, answers one sample on MCP, REST, gRPC and the web console, and names the plugin command the data comes from | Extends: the log stream registers the prefix `monitor container log` and forwards to `show container log` |
| Builtin command as a proxy to a plugin | `internal/component/plugin/server/handler.go` `RPCRegistration.PluginCommand`; `internal/plugins/vrrp/yang/ze-vrrp-cmd.yang` module description | A YANG command node is bound to a wire method whose handler forwards to the plugin, so the answer is produced in the plugin's process | Extends: every command of table C-1 is bound this way |
| Long operation answered once | `internal/plugins/update-cmd/cmd/firmware.go` `handleFirmwareDownload`, `reqCtx` | Runs the download under the request's context and answers one result at its end | Extends: the image fetch and the image update answer the same way. Added: the MCP task annotation, and an in-flight row in the image listing so any surface can read progress |
| MCP task support | `docs/architecture/api/commands.md` "MCP Task Methods"; `ze:task-support` in the extension table of `yang-config-design.md` | A command annotated `required` returns a task at once on MCP, which the client polls | Extends: the fetch and the update carry the annotation |
| Command with a trailing typed selector | `internal/plugins/vrrp/yang/ze-vrrp-cmd.yang`, the `name` node under `show vrrp interface` | A command whose last path word is `name`, holding one leaf for the value | Extends: every per-container command has this shape |
| Selector guard by mandatory leaf, and the explicit `all` | `docs/architecture/cli/command-verbs.md` "Action verbs and their selector" and "Everything is a selector the operator types"; `ze-vrrp-cmd.yang` `clear vrrp statistics all` | A command addressing one member by a typed keyword is guarded by its mandatory leaf, and a bare prefix is refused naming the commands below it | Extends: no container command sets `RequiresSelector`. Its mandatory leaf and the `all` node are the guards |
| Per-command answer declarations | `docs/architecture/api/commands.md` "Per-command declarations" | A command registers its shape, column order and address fields under its path | Extends: each command of table C-1 declares them |
| Runtime doctor checks of a plugin | `docs/architecture/doctor-and-health-checks.md` "The runtime phase" and "Where each owner registers its check" | A plugin declares a `runtime` check at Stage 1 and answers it in its own process | Extends: the six checks of table D-1 |
| Operator error and value refusal | `yang-config-design.md` "Value refusals"; `ai/rules/user-facing-errors.md` | A value outside a range, enum, length or pattern is refused with the problem, the accepted values and a fix line built from the schema | Extends by declaration: every "no unlimited value" refusal is a range refusal the schema produces |
| Sensitive leaves | `yang-config-design.md` extension table, `ze:sensitive` | Masks a leaf on every display and keeps the stored text | Extends: the registry password and a variable's `secret` |
| Interface and system leaf names | The interface config module (`leaf-list address`, `leaf hostname`, `leaf mtu`) and the system config module (`leaf-list name-server`, `leaf host`), read at those nodes by search | The spellings an operator already types for the same concepts | Extends: this schema uses `address`, `hostname`, `mtu` and `name-server` with the same meaning |
| Plugin metrics hook | `ai/patterns/plugin.md` "Metrics Pattern", `Registration.ConfigureMetrics` | Hands a plugin the metrics registry | Extends: the state metrics of table M-1 |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Config: the `container` root, written in a file or through the editor on any surface, as set lines or blocks.
- Commands: the paths of table C-1, typed at the CLI or sent by SSH, the web, REST, gRPC or MCP as a command line with its leaves.
- Doctor: `ze doctor` and `show doctor`. Metrics: a scrape of the exporter.

### Transformation Path
1. The schema judges each value: type, range, pattern, enumeration, mandatory node, reference. A refusal here is built from the schema.
2. The judging function of table V-1 reads the plugin's lowered root and refuses a combination no single node can express. It runs in the in-process verifier and in the plugin's configure and verify callbacks.
3. The siblings apply the accepted config. Each keeps the facts it owns: the runtime child the process state of each container, the image store the digests and stored data, the network child the links and addresses.
4. The state view joins those facts with the config into one record per container (table T-1). The listing, the detail, the doctor checks and the metrics all read that one record.
5. A command handler in the plugin's process answers a structured payload under its declared shape. The dispatcher applies the pipes and each surface renders the result.
6. The log stream forwards to the log command with a cursor and writes each new row.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Operator to schema | Config text through the parser and the editor | No |
| Config component to plugin | The in-process verifier at validate and commit, the config transaction at start and reload | No |
| Any surface to plugin | The dispatcher, the proxy wire method, then the plugin's command, internal and forked | No |
| Daemon stream handler to plugin | A forward to `show container log`, declared with `RegisterPluginStreamingHandler` | No |
| Doctor to plugin | The `OnDoctorCheck` callback | No |
| Plugin to report bus | The SDK's `RaiseIssue` and `ClearIssue` | No |

### Changed-Wire Census

**Search evidence:** searched `internal`, `cmd`, `pkg` and `test` (Go, YANG and `.ci` files) for the wire-method prefix `ze-container:` and for a command subject `container` after each of the seven verbs used here, and every YANG file under `internal` for a top-level config container named `container`. No hit, other than two test messages about the `update` and `delete` verb containers. The positive control is `ze-vrrp:`, which the same search finds in `internal/plugins/vrrp/yang/ze-vrrp-cmd.yang`. The gate manifest holds no container line, the state key registry no container key, and the diagnostic code table one code with the word, `doctor-platform-container-ro`, which is about Ze running inside a container and is not changed. Every wire below is new.

| Wire | Role (writer / reader) | Carrier (file + producer/consumer symbol or script entry) | Source evidence | Migration or preservation disposition |
|------|------------------------|-----------------------------------------------------------|-----------------|----------------------------------------|
| Container config root | writer | The operator's config file, the editor on every surface, gNMI Set | No root of that name in any schema module | New. Nothing to migrate |
| Container config root | reader | The container plugin's verifier and config callbacks, landed by the runtime child | No reader exists | New |
| Container command paths | writer | An operator, a script running `ze cli -c`, REST, gRPC and MCP clients | No command with that subject exists | New. A path is a contract once shipped |
| Container command paths | reader | The proxy wire methods and the plugin's command handlers | No handler exists | New |
| Container answer payloads | writer | The plugin's command handlers | No producer exists | New. Keys are kebab-case and match the schema leaf names |
| Container answer payloads | reader | API clients, the pipe layer, the web result card, the log stream handler (which reads `sequence`) | No consumer exists | New |
| Container doctor codes | writer | The plugin's runtime doctor checks and its `Warn` lines | No code with the `doctor-container-` prefix exists | New |
| Container doctor codes | reader | `ze doctor`, `show doctor`, an agent reading the JSON form | The doctor runner reads any registered code | New rows, existing reader unchanged |
| Container metric names | writer | The plugin through the metrics registry | No `ze_container_` metric exists | New |
| Container metric names | reader | A Prometheus scrape of the exporter, outside the repository | The exporter serves the registry as it is | New names, existing reader unchanged |

### Integration Points
- The `registry.Registration` of the container plugin (runtime child): `ConfigRoots`, `InProcessConfigVerifier`, `ConfigureMetrics`, and the command declarations.
- `pluginserver.RegisterRPCs` with `PluginCommand`, and `pluginserver.RegisterPluginStreamingHandler`: the commands and the log stream.
- The SDK's `DoctorChecks` and `OnDoctorCheck`: the runtime checks.
- The state each sibling keeps inside the plugin: this child reads it through one function per sibling and owns no copy.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding, outbound: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |
| Registration over hardcoding, inbound: no existing switch, seed map, validator, parser, runner, help string, or completion table has to learn this feature's name. Evidence names every list that was searched for the names this feature introduces, and the registry each one now derives from (`ai/rules/principles.md`) | No | |

## The design

### S-1. The `instance` list: one entry per container

The root `container` holds three keyed lists: `instance`, `network` and `registry`. The list of containers is named `instance`, because a list named for its own key word says nothing about what it holds, and `name` is the selector keyword of the commands.

The key of `instance` is `name`: a string of 1 to 63 characters, lowercase letters, digits and interior hyphens. Its help is "A container Ze runs from an image."

"Reference" describes the node the reference model has, in plain words, and how its conflict with Ze's rules is resolved.

| Node | Type and constraints | Default | Help | Reference, and the resolution | Lands with |
|------|----------------------|---------|------|-------------------------------|-----------|
| `image` | string, 1 to 255, the image reference form. Mandatory. Three forms are accepted: a short name (`redis`, `redis:7`), a whole reference with its registry host, and a reference by `sha256` digest | none | Image name; a short name is looked up on Docker Hub, and no tag means latest. | The same node, with the same short names. A name with no registry host resolves against the default registry, Docker Hub, with the `library` prefix for a one-word name, and a name with no tag takes `latest`. Ze adds one thing: the digest the name resolved to is recorded, shown in every listing beside the whole reference, and names the image's folder | image store |
| `enabled` | boolean | true | Run this container; false keeps its config and stops it. | A flag with no value that stops the container. Ze's form is the positive boolean | runtime |
| `description` | string, 1 to 255 | none | Free text shown in the container detail. | Same | operator surface |
| `entrypoint` | leaf-list of strings, each 1 to 4096, ordered, duplicates kept | the image's | Words that replace the image entrypoint. | One string the runtime splits. Ze takes the words, so no hidden shell grammar | runtime |
| `argument` | leaf-list of strings, each 0 to 4096, ordered, duplicates kept | the image's default command | Words passed to the entrypoint, replacing the image default. | Two nodes: a command string and an argument string that is ignored without the first. Folded into one node | runtime |
| `environment` | list, key `name`: 1 to 255, a letter or underscore, then letters, digits and underscores. A mandatory choice of `value` (string, 0 to 4096) or `secret` (string, 1 to 4096, `ze:sensitive`) | none | An environment variable of the workload. | The same keyed list with the same word and its `value` leaf. It sits inside a container entry, so it does not collide with Ze's own `environment` root. Ze adds `secret`, a masked form of the value | runtime |
| `hostname` | string, 1 to 253, a DNS host name | the instance name | Host name inside the container. | The same node. Ze spells it as the interface DHCP client leaf does. System `host` is the router's own name | runtime |
| `restart` | enumeration: `never`, `on-failure`, `always` | `on-failure` | When Ze starts the container again after it exits. | Three values, the first a bare negative word. Ze names it `never` | runtime |
| `stop-timeout` | uint16, 1 to 30, units seconds | 10 | Time between the stop signal and the kill. | Same node. The upper bound is 30, because a daemon stop gives containers 30 seconds (umbrella corrections) | runtime |
| `stop-signal` | enumeration: `term`, `int`, `quit`, `hup`, `usr1`, `usr2` | the image's, else `term` | Signal that asks the container to stop. | No node. Added because the runtime child's finding for a container that ignored its stop signal needs a statement to change | runtime |
| `memory` | uint32, 64 to 1048576, units mebibytes | 512 | Memory limit; the kernel kills inside the container past it. | An integer where zero means no limit. Zero is outside the range. The floor is 64, pending the measurement of the init role's memory, which the runtime child owns (its A-8). This table carries the range, and that child the measurement | runtime |
| `shared-memory` | uint32, 1 to 1048576, units mebibytes | 64 | Size of the shared-memory mount, counted in the memory limit. | An integer where zero means no limit. Zero is outside the range | runtime |
| `cpu` | uint32, 1 to 102400, units percent | 100 | CPU limit in percent of one core; 150 is one and a half cores. | A decimal string of cores, checked by a regular expression, where zero means no limit. Ze's form is an integer with a unit. The kernel side is the runtime child's (A-4) | runtime |
| `process-limit` | uint32, 1 to 32768 | 256 | Most processes and threads the container holds at once. | No node. Added because the umbrella limits processes (AC-6) | runtime |
| `storage` | uint32, 1 to 1048576, units mebibytes | 256 | Size cap of the container's own writable layer. | No node. Added because the umbrella caps the writable layer (O-2) | image store |
| `capability` | leaf-list, enumeration: `chown`, `mknod`, `net-admin`, `net-bind-service`, `net-raw`, `setpcap`, `sys-admin`, `sys-nice`, `sys-rawio`, `sys-time` | none | Linux capability added to the default set. | The same multi-valued node with the same values. The default set is the fourteen capabilities published images expect, so a value already in it is accepted and changes nothing. The value that loads kernel modules is left out (table S-4) | runtime |
| `capability-drop` | leaf-list, enumeration: `all`, and the fourteen of the default set: `audit-write`, `chown`, `dac-override`, `fowner`, `fsetid`, `kill`, `mknod`, `net-bind-service`, `net-raw`, `setfcap`, `setgid`, `setpcap`, `setuid`, `sys-chroot` | none | Linux capability removed from the default set. | No node. Added by owner ruling, so a container runs with less than the default. `all` removes the whole default set, and `capability` then adds back what is named | runtime |
| `host-namespace` | leaf-list, enumeration: `network`, `pid`, `cgroup` | none | Host namespace the container shares in place of its own. | Three flags with no value. One multi-valued node of closed keywords | runtime |
| `user` | uint32, 0 to 65535 | the image's | Numeric user the entrypoint runs as. | Same. The rule that a group needs a user is dropped: each is independent | runtime |
| `group` | uint32, 0 to 65535 | the image's | Numeric group the entrypoint runs as. | Same | runtime |
| `name-server` | leaf-list of `zt:ip-address` | the router's resolver list | Resolver written to the container's resolver file. | A node refused unless a negative flag is set on the network. The flag does not exist in Ze, so the rule is gone. Ze answers no container name over DNS at first | network |
| `log` | enumeration: `buffer`, `system`, `none` | `buffer` | Where the container's output goes. | Three values that name another runtime's log backends. Ze's values name Ze's own: a bounded buffer the log commands read, that buffer and Ze's log, or nothing | runtime |
| `sysctl` | list, key `name`: 1 to 255, a dotted kernel setting name. Leaf `value`: string, 1 to 255, mandatory | none | Kernel setting inside the container's own namespaces. | The same list behind one more fixed word, which is dropped | runtime |
| `device` | list, key `name`. `host-path`: string that starts with the device directory, mandatory. `container-path`: absolute path | `container-path`: the host path | Host device node the container opens. | Two paths named for a direction. Named here for the side they are on | runtime |
| `tmpfs` | list, key `name`. `destination`: absolute path, mandatory. `size`: uint32, 1 to 1048576, units mebibytes, mandatory | none | Memory-backed mount inside the container. | The same list. Its size rule compares with half of the host's memory; here the size counts in `memory` (V-3). The ownership flag is left out | runtime |
| `volume` | list, key `name`. `destination`: absolute path, mandatory. `size`: uint32, 1 to 1048576, units mebibytes. `read-only`: boolean | `size` 64, `read-only` false | Storage that survives image updates. | A host path bound into the container, with a mode word and a mount-propagation word. Ze stores a volume in its own data area with a size cap, so the host path and the propagation are left out, and the mode is a boolean | image store |
| `port` | list, key `name`. `protocol`: enumeration `tcp`, `udp`. `listen-address`: leaf-list of `zt:ip-address`. `listen-port`: one port or a first-last range, mandatory. `container-port`: the same type, mandatory | `protocol` `tcp`; no `listen-address` means every address of the router | Port of the router that reaches a port of the container. | The same list, its two ports named for a packet direction. Named here for where each port is | network |
| `network` | list, key `name`: a `leafref` to the `network` list. `address`: leaf-list of `zt:ip-address`. `mac-address`: string with the six-octet colon pattern | none | Attachment to a container network. | The same list. A keyword in the hardware-address slot makes the system derive an address and write it into the operator's config at commit. In Ze an absent leaf means derived: the address is stable, shown in the interface listing, and never written to the config | network |

### S-2. The `network` list: one entry per container network

The key is `name`, 1 to 11 characters, lowercase letters, digits and interior hyphens. The bound is the network child's: the network's bridge is named with a four-character prefix and the name, and a kernel device name holds 15. A longer name is refused by the schema, never shortened. Help: "A network that containers attach to."

| Node | Type and constraints | Default | Help | Reference, and the resolution | Lands with |
|------|----------------------|---------|------|-------------------------------|-----------|
| `prefix` | leaf-list of `zt:ip-prefix`, at least one | none | Address range of the network, one per address family. | Same | network |
| `gateway` | leaf-list of `zt:ip-address` | the first host address of each prefix | Address of the router on the network. | Same. The derived value is shown in the listing and never written | network |
| `mtu` | uint16, 68 to 16000 | 1500 | Largest packet the network carries. | Same range as an interface | network |
| `masquerade` | boolean | true | Translate outbound traffic to the router's address. | No node: translation is always on. Ze keeps it on by default and adds the node that turns it off, for a network whose prefix the router routes | network |
| `description` | string, 1 to 255 | none | Free text shown in the network listing. | Same | operator surface |

### S-3. The `registry` list: one entry per registry that needs settings

The key is `name`: the registry host, with its port when it has one, exactly as image references write it. A registry with no entry is reached over HTTPS with no credentials. Help: "Settings for one image registry."

| Node | Type and constraints | Default | Help | Reference, and the resolution | Lands with |
|------|----------------------|---------|------|-------------------------------|-----------|
| `authentication` | presence container. `username`: string, 1 to 255, mandatory. `password`: string, 1 to 4096, mandatory, `ze:sensitive` | none | Credentials Ze presents to the registry. | Two leaves with a both-or-neither rule. The presence container with two mandatory leaves states the rule in the schema | image store |
| `transport` | enumeration: `https`, `http` | `https` | Protocol Ze speaks to the registry and to its mirror. | One negative flag that means either plain HTTP or unverified TLS. Ze names the protocol. Unverified TLS does not exist: a private authority is named by `trust-anchor`. The protocol applies to the registry's mirror too, and a mirror's `address` may be a loopback address: the image store child's test registry depends on both | image store |
| `trust-anchor` | `leafref` to the name of a `pki ca` entry | the system roots | Authority that signs the registry's certificate. | No node. Added because a registry signed by a private authority cannot be reached otherwise. A registry that names an anchor is trusted through that anchor only | image store |
| `mirror` | presence container. `address`: an IP address or a host name, mandatory. `port`: `zt:port`. `path`: string that starts with a slash | `port` 443, `path` a single slash | Server Ze asks first; the registry is asked when it fails. | An address leaf and a host-name leaf that exclude each other, a port and a path. One `address` leaf takes either form, as Ze's endpoint rule states. One mirror for each registry, asked first and with no credentials | image store |
| `description` | string, 1 to 255 | none | Free text shown with the registry. | Not in the reference. Added for parity with the other two lists | operator surface |

The default registry, Docker Hub, needs no entry. An entry under its host name gives it credentials, a trust anchor or a mirror like any other.

The root also holds one container, `store`, for the settings of the image store as a whole:

| Node | Type and constraints | Default | Help | Reference, and the resolution | Lands with |
|------|----------------------|---------|------|-------------------------------|-----------|
| `store largest-image` | uint32, 1 to 65536, units mebibytes | 1024 | Largest image the store accepts, measured unpacked. | No node. Added because the cap on one image's unpacked size has to be raisable by the operator | image store |

The root holds one more container, `shell`, for the session limits of the shell child. The limits are that child's design (its O-8). The leaves are rows of this table, and the shell child lands them:

| Node | Type and constraints | Default | Help | Reference, and the resolution | Lands with |
|------|----------------------|---------|------|-------------------------------|-----------|
| `shell idle-timeout` | uint32, 30 to 86400, units seconds | 900 | Time a session with no input and no output stays open. | No node. Added because a shell left alone must end | shell |
| `shell absolute-timeout` | uint32, 60 to 86400, units seconds | 28800 | Longest time one session stays open. | No node. Added for the same reason | shell |
| `shell session-limit` | uint8, 1 to 16 | 4 | Most sessions open in one container at once. | No node. The cap for the whole daemon is 16 and has no leaf | shell |

### S-4. Reference nodes left out

| Reference node, in plain words | Why it is left out |
|--------------------------------|--------------------|
| The search of a short image name across several registries, the second built-in registry, and the flag that removes a registry from the search | A short name resolves against one default registry, by owner ruling. A search across several lets one name resolve to different publishers' images, and Ze records one digest for one name |
| Labels on a container | Nothing in Ze reads a label, so the value would reach no consumer |
| The flag that grants every privilege | It removes the confinement the umbrella requires. Each grant is named: a capability, a host namespace, a device |
| The capability that loads kernel modules | A container that changes the router's kernel ends the property that the running system is the installed image |
| The health check (a command, an interval that is a number or a keyword, a timeout, a retry count) | By owner ruling it is a later child of the container work, with a spec of its own (Known Limitations). The form Ze gives it is decided here so that child takes it: a presence container with a command, an integer interval, a timeout and a retry count. Its absence is the "off" the keyword spelled, so no slot holds a keyword and a number |
| The flag on a network that turns its name service off, and the rule tying a container's resolvers to it | Ze runs no per-network name service, so there is nothing to turn off. A negative boolean is not declared for an absent feature |
| The VRF leaf of a network | Ze refuses a VRF everywhere today. The VRF work adds the leaf to this list when it ships. Until then the keyword is unknown here, which is the refusal the umbrella states |
| The network type node and its second kind, a network placed directly on a parent interface | Ze's interface model has no such interface kind. With one kind left, a type node states nothing |
| The host path, the mount-propagation word and the mode word of a volume | Table S-1, `volume` |
| The ownership flag of a memory-backed mount | The mount is world-writable with the sticky bit, as a temporary directory is, which is what the flag is used for |
| The image build command | The umbrella fetches images by digest and builds none |
| The rule that a container has a network or shares the host's | A container with neither has only its loopback interface. That is the most confined form, so Ze accepts it |

### V-1. What `ze config validate` refuses with no daemon

Every row is judged by the one function. A row marked "host" reads the host it runs on, so its verdict is about that host, as a kernel capability verdict is.

| Rule | Refused when | Lands with |
|------|--------------|-----------|
| V-1 | `host-namespace` holds `network` and the instance has a `network` entry, a `port` entry, or a `sysctl` whose name starts with the network prefix | network |
| V-2 | A `sysctl` name is outside the kernel settings that exist per namespace | runtime |
| V-3 | `shared-memory` plus the `size` of every `tmpfs` exceeds `memory` | runtime |
| V-4 | An attachment `address` is outside every prefix of its network, is a second address of one family, equals the network's gateway, or is held by another instance | network |
| V-5 | A network holds two prefixes of one family, a gateway outside its prefixes, or a prefix that overlaps another container network's | network |
| V-6 | The `listen-port` and `container-port` ranges of a `port` differ in length, a range runs backward, or two entries publish one protocol and port on addresses that overlap | network |
| V-7 | Two of `volume`, `tmpfs` and `device` in one instance name the same path inside the container, or a path holds a parent-directory segment | runtime |
| V-8 | `cpu` exceeds 100 times the processors of the host (host) | runtime |
| V-9 | A published port is one a configured Ze listener binds on an overlapping address. The network child owns the mechanism, this child the message | network |
| V-10 | One capability is in both `capability` and `capability-drop` | runtime |
| V-11 | The config holds a container and the process that judges it does not run as root (host). The appliance and a root daemon are where containers run at first. It is judged the way a kernel requirement is: at `ze config validate`, at start and at reload, on the host where the command runs, and by doctor. A validate run names the user it judged. The runtime child states the same rule | runtime |
| V-12 | The config holds a container and the data directory's filesystem cannot enforce a size cap (host). It is judged at the same four places as V-11, on the host where the command runs | image store |
| V-13 | A network `prefix` has a length outside 1 to 30 for IPv4, or outside 1 to 126 for IPv6 | network |
| V-14 | A network with an IPv6 prefix has an `mtu` below 1280 | network |
| V-15 | A network has more attachments than free addresses in a prefix | network |
| V-16 | The bridge device of a network, `zcn-` plus its name, is declared as an interface in the `interface` config, or a network prefix contains an address the `interface` config sets | network |
| V-17 | Two attachments of one network set or derive the same hardware address | network |
| V-18 | An instance has a `port` entry and no `network` entry | network |
| V-19 | The config needs a container link and the interface backend cannot create one, or needs a translation and the firewall backend cannot carry the tables | network |

This table lists every rule, and table E-1 its message. The child that lands a rule owns its code and its tests, in its own package.

Not judged at verify, by the umbrella's design: whether an image is in the store, whether a registry answers, and whether a `device` host path exists. Each is a state the listing and doctor report.

Required nodes, ranges, enumerations, patterns and the attachment's reference need no rule: the schema refuses them, and its refusal names the accepted values and the fix line.

### V-2. A build without the container gate

| Input | Answer | Produced by |
|-------|--------|-------------|
| A config with a `container` block | Refused at parse as an unknown top-level keyword. The second line says the keyword is configured by a plugin this build was compiled without, and the fix line says to rebuild with the container build tag or remove the block | `config.KeywordHint`, from the generated root table |
| Any command of table C-1 | Refused as an unknown command, with the closest command as the hint | The dispatcher, which holds no such command |
| `ze doctor` | No container check and no container code | The checks are registered by the absent plugin |

### C-1. The command set

Every command is a YANG command node in the plugin's command module, bound to a wire method that the plugin's package registers as a proxy to the plugin. A per-container command ends in the node `name`, whose mandatory leaf holds the container name, completed from the configured instances.

| Path | Leaves | Why this verb | Answer: shape, then fields | Lands with |
|------|--------|---------------|----------------------------|-----------|
| `show container list` | none | Reports only | rows: `name`, `state`, `reason`, `image`, `digest`, `restart-count`, `last-exit-status`, `since` | operator surface |
| `show container detail name` | `value` | Reports only | document: the list fields, then `process-id`, `previous-digest`, the limits in force, `storage-used`, and three row sets: attachments, published ports, volumes | operator surface |
| `show container log name` | `value`; `lines` (uint16, 1 to 10000, default 100); `after` (a sequence number) | Reports only | rows: `sequence`, `time`, `stream`, `line` | operator surface |
| `monitor container log name` | `value` | The same read, streamed | the rows of the log command, as they arrive. One-shot: the newest 100 rows | operator surface |
| `show container image list` | none | Reports only | rows: `reference` (the whole reference a short name resolved to), `digest`, `size`, `fetched`, `state` (`present`, `fetching`, `failed`, `previous`), `received`, `used-by`, and for a failed fetch `error` and `code` | image store |
| `show container store` | none | Reports only | document: `path`, `total`, `free`, `reserve`, `caps-enforced`, and a row set `layers`: `instance`, `digest`, `size`, `used` | image store |
| `show container volume list` | none | Reports only | rows: `instance`, `name`, `size`, `used`, `state` (`in-use`, `orphan`) | image store |
| `show container network list` | none | Reports only | rows: `name`, `prefix`, `gateway`, `interface`, `mtu`, `masquerade`, `attached` | network |
| `show container interface list` | none | Reports only | rows: `instance`, `network`, `interface`, `address`, `mac-address`, `state` | network |
| `create container image reference` | `value`: an image name, the type and the three forms of the `image` node | An image that was not in the store is in it after the command | document: `reference` (whole), `digest`, `size`, `created` (false when the digest was already stored), `started` (the containers that were waiting for it) | image store |
| `update container image name` | `value` | The digest held for a container that exists is rewritten | document: `name`, `previous-digest`, `digest`, `changed`, `restarted` | image store |
| `update container image rollback name` | `value` | The same data is rewritten, to the previous digest | document: `name`, `previous-digest`, `digest`, `restarted` | image store |
| `delete container image digest` | `value`: a digest, or a prefix of at least 12 hexadecimal digits that names one image | A stored image stops existing | document: `deleted` (rows of `digest` and `size`), `kept` (rows of `digest` and `reason`) | image store |
| `delete container image all` | none | The explicit everything form. It removes every image no configured container holds | the same document | image store |
| `delete container volume name` | `value`: the volume; `instance`: the instance, mandatory | Stored data stops existing | document: `instance`, `name`, `size` | image store |
| `clear container layer name` | `value` | Accumulated runtime data returns to empty | document: `name`, `cleared`, `restarted` | image store |
| `request container restart name` | `value` | The system changes, and no narrower verb describes it | document: `name`, `state`, `process-id` | runtime |
| `request container shell name` | `value`; `command`, `input`, `timeout` (the shell child's leaves) | A process starts inside the container | The shell child's: a live session, or the bounded run's document | shell |
| `show container session list` | none | Reports only | rows, one per open session: the shell child's fields | shell |
| `delete container session id` | `value`: the session id, the shell child's leaf, an integer from 1 to 4294967295 | An open session stops existing, which is the `delete` test | The shell child's | shell |

The three shell rows are owned by `spec-container-5-shell`: the commands, their leaves and their answers are that child's, and it lands them. They are listed here so the table is whole, and this table's verb rules apply to them. The first two fit this grammar as they are: a verb, the subject, the action, then the typed selector with the optional leaves after it, and `list` for the enumeration. The session close is `delete container session id`, by the verb test of this table.

What each action does, where the words alone do not say it:

| Command | Behavior |
|---------|----------|
| `create container image reference` | Fetches and verifies the image. It runs under the request's context and answers once, at the end. A caller that goes away stops it, and verified data stays for the next attempt. While it runs, the image listing shows the row as `fetching` with `received`. Every container waiting for that reference then starts, with no further command |
| `update container image name` | Fetches the reference the container's config names. When the digest differs, the container moves to it with an empty writable layer and restarts, and the previous digest stays for rollback. When it is the same, nothing changes and `changed` is false |
| `delete container image digest`, `all` | The image store child states the one delete rule, and this row cites it: an image that any configured container holds as its current or its previous generation is kept, and deleting it is refused. `digest` naming such an image is refused, and the refusal names the container and the generation. `all` skips it and lists it under `kept` |
| `delete container volume name` | Refused while the config still declares that volume, as the image store child states. The refusal names the config statement to remove |
| `update container image rollback name` | The only way back to the previous digest. Ze never rolls back on its own |
| `clear container layer name` | Empties the writable layer. A running container is stopped first and started again on the empty layer. It also accepts the name of a container no longer configured, and then removes the stored layer |
| `request container restart name` | Stops a running container and starts it. Starts a container in the `stopped` or `failed` state. Refused for a `disabled` container, with the config statement that enables it, and for one in `waiting-image`, with the fetch command |

Start and stop: Ze has neither command, as the reference model has neither. Whether a container runs is stated once, by `enabled` in the config. A stop command would create a second statement the config does not hold: a container stopped by hand comes back at the next reboot, and doctor would have to report a class of "stopped by an operator" that no file records. The transient action an operator needs is the restart, which also starts a stopped or failed container.

Pipes: every command answers data and declares its shape and column order, so the global operators apply to all of them. The row operators (`match`, `count`, `first`, `last`, the field display) apply to the commands whose shape holds rows and are refused by name on a document. `show container interface list` and `show container detail name` declare `address` as an address field. The stream takes the operators the catalog marks for streaming. No command owns a pushed-down filter: no listing is large enough to need one.

Authorization: the `show` and `monitor` commands are reads. Every other command lands in the edit section through its verb.

### C-2. Behavior on each surface

| Surface | Listings, detail, log | Actions | `create` and `update container image` | `monitor container log name` |
|---------|-----------------------|---------|----------------------------------------|------------------------------|
| `ze` at a shell, `ze cli -c` | The daemon's answer, rendered by the pipes | The daemon's answer | The call returns when the fetch ends | Streams until interrupted |
| SSH exec and interactive, the `ze cli` shell | The same | The same | The same | Streams until interrupted |
| Web terminal | The same, in the result area | The same | The call returns when the fetch ends | The one-shot sample |
| Web admin | A form with no field or one field per leaf, and the result card | The same | The same. The image listing shows the progress meanwhile | The one-shot sample |
| REST | The JSON envelope with the declared keys | The same | The request stays open until the fetch ends. A client that closes it stops the fetch | The one-shot sample |
| gRPC | The same payload | The same | The same | The one-shot sample |
| MCP | A tool per command with its leaves as parameters | The same | A task is returned at once and polled, by the task annotation | The one-shot sample |

No surface package is edited. Each row follows from the registration and the schema.

### T-1. Container state

One record per configured container, built from the config and the three siblings' facts.

| State | Meaning | Fields that carry the detail | Doctor |
|-------|---------|------------------------------|--------|
| `running` | The entrypoint runs | `digest`, `process-id`, `since`, `restart-count` | none |
| `waiting-image` | The image the config names is not in the store | `reason` names the image reference | D-1 row 1 |
| `restarting` | The entrypoint ended, the policy restarts it, and the delay is running. Also a failed start that will be tried again | `restart-count`, `last-exit-status`, `reason` gives the time of the next start | D-1 row 2 |
| `stopped` | The entrypoint ended and the restart policy keeps it stopped | `last-exit-status`, `reason` names the policy | D-1 row 3, for a non-zero exit only |
| `disabled` | `enabled` is false | none | none |
| `failed` | A start step failed five times in a row, and Ze stopped trying. An entrypoint that keeps exiting never gives this state: it is restarted under its policy with a growing delay and no bound, and the delay returns to one second after a run of one minute | `reason` names the step in plain words: an entrypoint that does not exist, a device that is absent, a limit the kernel refused | D-1 row 4 |

`reason` is one sentence in the operator's terms. `last-exit-status` is the exit code, or the name of the signal that ended the process. These six states are the only names an operator sees. The runtime child defines when each is set (its section "Container state"), and this table states them in the same words. Its internal steps (a start or a stop in progress, an exit not yet judged) each map onto one of the six, and it adds a state only by adding a row here.

When Ze restarts, for its own update included, every container is stopped and started again. The record then shows the new start time, and the last exit status recorded before the restart.

An environment variable's value never appears in a listing, the detail, a log line Ze writes, or a doctor finding. The detail names the variables and nothing more.

### D-1. Doctor checks that span the siblings

Each is a `runtime` check, declared at Stage 1 and answered in the plugin's process. The same code is on the `Warn` line and the report-bus issue raised when the condition starts, and the issue is cleared when it ends.

| # | Code | Severity | Reported when | Message | Fix |
|---|------|----------|---------------|---------|-----|
| 1 | `doctor-container-image-absent` | warning | A container is in `waiting-image` | container NAME is not running: its image REFERENCE is not in the image store, and Ze fetches an image only on request | `ze create container image reference REFERENCE` |
| 2 | `doctor-container-restarting` | warning | A container restarted three or more times since it last ran for a minute | container NAME keeps exiting: N restarts, last exit status S | `ze show container log name NAME` |
| 3 | `doctor-container-stopped` | warning | A container is `stopped` after a non-zero exit | container NAME exited with status S and its restart policy keeps it stopped | `ze request container restart name NAME` |
| 4 | `doctor-container-failed` | error | A container is `failed` | container NAME cannot run: REASON | The command for the reason: the restart, the log, or the config statement to change |
| 5 | `doctor-container-store-orphan` | warning | The store holds a writable layer or a volume of a container or volume the config no longer declares | the image store holds SIZE for NAME, which the configuration no longer declares | `ze clear container layer name NAME`, or `ze delete container volume name VOLUME instance NAME` |
| 6 | `doctor-container-port-blocked` | warning | A container publishes a port and the operator's firewall policy drops the traffic to it | container NAME publishes PROTOCOL port N, and the firewall policy drops connections to it: a published port must also be allowed by the policy | The `set` line of a firewall term that accepts PROTOCOL to the container's port on the network's bridge |

Row 6 is judged by the network child, which reads the firewall policy as its section "Published ports and outbound translation" defines. The code and the message are this child's. The six codes of this table are declared here and nowhere else: a sibling that supplies the facts cites the row. Every container doctor code, in every child, starts with `doctor-container-`.

Checks owned by one sibling stay with it: the kernel capabilities, the cgroup mount and the unprivileged daemon (runtime), free space, the size-cap capability and registry reachability (image store), the link and the translation state (network). A kernel requirement has one pair of codes for each enrolled capability, in every child: `doctor-container-kernel-NAME-unavailable` and `doctor-container-kernel-NAME-unknown`, where NAME is the capability's lower-kebab name. The storage cap of V-12 is such a capability, and its pair is `doctor-container-kernel-quota-unavailable` and `doctor-container-kernel-quota-unknown`.

### E-1. Operator messages of this child

Every refusal is an operator error: the problem, why it matters, the fix alone on the last line.

| Trigger | Problem line | Why | Fix |
|---------|--------------|-----|-----|
| V-1 | container NAME shares the host's network and also has a network, a published port or a network setting | a container in the host's network namespace has no link, address or port of its own | the `delete` line for the statement that conflicts |
| V-3 | container NAME: its memory-backed mounts need M mebibytes and its memory limit is L | those mounts are counted in the memory limit, so the container would be killed when they fill | `set container instance NAME memory M` |
| V-4 | container NAME: address A is not inside network NET (PREFIX) | Ze refuses an address the network cannot route | `set container instance NAME network NET address` with an address of the prefix |
| V-4, held | container NAME: address A is already held by container OTHER | two containers with one address both lose traffic | the `delete` line for one of the two |
| V-6 | container NAME port P: N ports are published to M ports | each published port needs one port of the container | the `set` line with a range of equal length |
| V-9 | container NAME port P: port N is the port Ze's SERVICE listens on | publishing it would take that service's connections | the `set` line with another port |
| V-10 | container NAME: capability C is both added and dropped | the two statements cannot both hold | the `delete` line for one of the two |
| V-11 | this configuration has a container, and Ze runs here as user USER, not as root. A validate run names the user that ran it, which is the user it judged | Ze creates a container's namespaces and limits as root. Containers run on the appliance and under a root daemon | the command that starts Ze as root on this host, or the `delete container` line |
| V-12 | this configuration has a container, and the filesystem of DIRECTORY cannot cap the size of a folder | every container's storage and every volume has a size cap, and Ze refuses to run one it cannot enforce | the statement or command the image store child names for a data directory on a filesystem that can |
| V-2 | container NAME: kernel setting S is not a setting of the container's own namespaces | a setting outside them would change the router itself | the `delete` line for the setting |
| V-5 | network NET: prefix P overlaps network OTHER, or gateway G is outside its prefixes | two networks with one range cannot both be routed | the `set` line with a prefix or a gateway that fits |
| V-7 | container NAME: path P is named by two of its volumes, memory-backed mounts and devices | two mounts on one path hide each other | the `delete` line for one of the two |
| V-8 | container NAME: cpu C is more than the N processors of this host give | a limit above the host's capacity limits nothing | `set container instance NAME cpu` with a value the host has |
| V-13 | network NET: prefix P has a length a container network cannot use | the network needs room for a gateway and at least one container | the `set` line with a length inside the range |
| V-14 | network NET: mtu M is below 1280 and the network has an IPv6 prefix | IPv6 needs a link that carries 1280 bytes | `set container network NET mtu 1280` |
| V-15 | network NET: N containers attach and prefix P has M free addresses | each attachment needs an address | the `set` line with a larger prefix, or the `delete` line for an attachment |
| V-16 | network NET: its bridge device DEVICE, or an address of its prefix, is already declared under `interface` | two owners cannot hold one device name or one address | the `delete` line for the interface statement, or the `set` line with another network name or prefix |
| V-17 | containers NAME and OTHER have the same hardware address on network NET | two links with one hardware address both lose traffic | `set container instance NAME network NET mac-address` with another address |
| V-18 | container NAME publishes a port and has no network | a published port needs an address to reach | `set container instance NAME network NET`, or the `delete` line for the port |
| V-19 | container network NET needs a backend this configuration does not select | the active interface or firewall backend cannot create the link or carry the translation | the `set` line for the backend leaf the message names |
| A value that means no limit | Built by the schema: the value is outside the accepted range, with the range and the `set` line | The schema's own why line | The schema's own fix line |
| `show container log name` with `log none` | container NAME keeps no output: its `log` setting is `none` | nothing was kept to show | `set container instance NAME log buffer` |
| `request container restart name` on a disabled container | container NAME is disabled in the configuration | a restart would start what the configuration says to keep stopped | `set container instance NAME enabled true` |
| An unknown container name in a command | no container named NAME is configured, with the closest name | the command acts on configured containers only | `ze show container list` |

An image that `delete container image all` keeps is not an error: the `kept` row of the answer names the container that holds the digest. `delete container image digest` naming such an image is refused by the image store child's delete rule, and the refusal names the container, the generation, and the config statement that removes the container.

### M-1. Metrics

| Metric | Kind | Labels | Value |
|--------|------|--------|-------|
| `ze_container_instances` | gauge | `state` | Containers in each state of table T-1 |
| `ze_container_instance_running` | gauge | `name` | 1 while the container runs, else 0 |
| `ze_container_instance_restarts_total` | counter | `name` | Starts after the first |
| `ze_container_instance_last_exit_status` | gauge | `name` | The last exit code, 128 plus the signal number for a signal |

Resource use (memory, processes, storage) is the sibling's that reads it. The `name` label is bounded by the config.

### W-1. The web UI

| What appears | From | Extra work |
|--------------|------|-----------|
| The `container` root in the config editor, its three lists, each leaf with its help, its enumeration choices, its default and its range refusal | The schema | None |
| The attachment's network offered from the configured networks | The `leafref` | None |
| The password and a variable's `secret` masked | `ze:sensitive` | None |
| Each command in the admin tree, with a form field per leaf | The command schema | None |
| The log stream | Its one-shot declaration: the newest rows | None |

No container page, template or route is written. The proof is a browser test (AC-13).

### X-1. The closing test of the container work (umbrella AC-14)

One test, in the appliance guest, through the operator's own entry points. It is the last implementation phase of the container work: it runs once the runtime, image store and network children and the other phases of this child are done. The shell child does not gate it.

| Step | What the operator does, in the words of this spec | What the test asserts | Rests on |
|------|---------------------------------------------------|-----------------------|----------|
| 0 | Nothing: this step is the test's own setup, and it stands for a host beyond the router. The test creates a separate network namespace and joins it to the router by a link: a veth pair with an address on each end, the way the repository's Linux integration tests build a peer. The outside namespace routes to nothing but that link | The router's end of the link is up and holds its address, and the outside namespace reaches that address | The test's compiled observer |
| 1 | Fetches the image: `create container image reference redis`. On a router with Internet access the short name resolves to the published Redis image on Docker Hub. In the test the operator types the same short name. The config holds a `container registry` entry for the default registry's host, with `transport http` and a `mirror` on a loopback address: the image store child's test registry, which serves the real published Redis image: the pinned Redis index, under the `latest` tag for the short name `redis`, so the name with no tag resolves there as it does on Docker Hub. The transport applies to the mirror, and the mirror is asked first | The answer holds the whole reference and a digest, and `show container image list` shows the row `present` | Image store AC-1, AC-25 and AC-26 |
| 2 | Declares a network: `set container network lan prefix` with an IPv4 prefix | Accepted | Network AC-1 |
| 3 | Declares the container: `set container instance redis image` with the same reference, `set container instance redis network lan`, `set container instance redis port redis listen-port 6379`, and `set container instance redis port redis container-port 6379`. The protocol is the default, `tcp` | Accepted | This spec, tables S-1 and V-1 |
| 4 | Allows the published port in the firewall policy. In a table of the `inet` family, a `forward` chain of type `filter` with `policy drop` holds two terms. One accepts `connection-state established,related`. The other, for the published port, has `from protocol tcp`, `from destination-port 6379`, `from output-interface` naming the bridge of network `lan`, and `then accept`. The table has no input chain | Accepted. `show doctor` holds no `doctor-container-port-blocked` finding | The network child's policy rule and its local-address match |
| 5 | Commits | `show container list` shows `redis` as `running` with the digest of step 1 | Runtime AC-1, network AC-2, this spec AC-1 |
| 6 | A client connects to the router's address on the link of step 0, port 6379, and speaks the Redis protocol. The client runs beyond the router, in the outside namespace of step 0, as an outside host would. Its connection is translated at the prerouting hook and forwarded to the container, so it crosses the forward hook, where the term of step 4 accepts it | The reply to a ping is the pong reply. A value is stored and read back. A `SAVE` is answered with success | Network AC-6 (umbrella AC-9), and the row "A host beyond the router" of the network child's policy table |
| 7 | Restarts the container: `request container restart name redis` | The listing shows `running` again with `restart-count` unchanged or raised by the runtime child's rule. The same client, on a new connection, reads the stored value back | Runtime AC-22 (the restart command), AC-9 and its declared-volume rule: the config declares no volume, so the image's data path stays in the writable layer. Image store AC-10 and AC-26 (umbrella AC-4) |
| 8 | The client, from the same outside namespace, connects to the same address on port 6380, which no container publishes | The connection is refused | Network AC-6, its permitted survivor |
| 9 | Deletes the firewall term of step 4 that accepts the published port, and commits. The client then opens a new connection from the outside namespace to port 6379 | The connection is not established: the client's bounded attempt ends with no reply. `show doctor` holds `doctor-container-port-blocked`, naming the container, `tcp` and port 6379, with the `set` line of the term as its fix | The network child's policy rule, by owner ruling: a published port must also be allowed by the firewall policy. Table D-1 row 6 |

The client is test code of this repository: a small Redis protocol client in Go, used by the test's compiled observer. No client program is run, because Ze's shipped code and test code run no external program.

How Redis is made to persist: the client sends `SAVE` after it stores the value. By Redis's documentation that command writes the snapshot file in the server's working directory before it answers, and the server loads that file at start. The explicit command is chosen over the append-only argument because it needs no `argument` node in the config, so the container is configured exactly as an operator's first container is, and because it does not depend on the image's default save settings or on what the server does at a stop signal (A-11, A-12).

Where the client is: beyond the router, in the outside namespace of step 0, in every step that connects (6, 7, 8 and 9). A connection opened on the router crosses the output and input hooks and never the forward hook, so a client there would connect with or without the term of step 4, and the test would prove nothing about the policy. From the outside namespace the connection is forwarded, so the term of step 4 is what admits it, and step 9 proves that: with the term deleted, the forward chain's drop policy ends the new connection and doctor reports `doctor-container-port-blocked`. A client on the router itself is the network child's own test, `container-port-from-router`, and is not part of this one.

Step 8 asserts a refusal, not a timeout. The connection to an unpublished port matches no translation term, so it is addressed to the router itself and crosses the input hook, never the forward hook. The firewall policy of step 4 has no input chain, so it reaches no listener and the kernel refuses it.

Step 9 is the one step that waits on an absence. The connection attempt has a fixed bound, and the step's verdict rests on the doctor finding, which is a state the test waits for.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The runtime child lands one plugin named `container` under `internal/plugins/`, with the config root `container`, one build tag, and a verifier entry point the other children add rules to | The runtime child's section "What this child builds": row 5 puts the plugin under `internal/plugins`, and row 7 lands the plugin registration with its config root and verifier entry point, and the gate. Its AC-14 names the one build tag | Paths in Files change. A different root word changes every table | The runtime child's wiring test and its build-tag tests | settled by the sibling's design, not yet validated by a test |
| A-2 | A command may end in a `name` node that holds the selector leaf and further leaves (`lines`, `after`, `instance`), and its proxy reaches the plugin when the plugin runs forked | `ze-vrrp-cmd.yang`, the `name` node under `show vrrp interface`, which holds one leaf | The log and volume commands need another leaf placement | The wiring test of Phase 1, run with the plugin internal and forked | unvalidated |
| A-3 | A list key may be a `leafref`, so the attachment's key is checked and completed from the network list | `yang-config-design.md` states leafref support for a leaf. A key was not read | The key is a string with `ze:reference` and a source the plugin registers | A parse test with an unknown network name | unvalidated |
| A-4 | The runtime child adds and pins the kernel option for a hard CPU limit, so the `cpu` default is enforced | Owner ruling of 2026-11-16 (umbrella, "Accepted defaults": a hard CPU limit, which adds one kernel option) | The `cpu` default cannot be applied exactly, and every container config would be refused | The runtime child's kernel enrolment test | unvalidated |
| A-5 | The web admin tree and its forms are derived from the command schema | `ai/doors/commands-cli.md`, trap "A handler that reads arguments the model never declares". The producing function was not found under the name the web digest gives | W-1 owes web work | The browser test of AC-13 | unvalidated |
| A-6 | A plugin registers its doctor codes from its own package | Precedent, read: the gated VRRP plugin does it. `registerVRRPDiagnosticCodes` in `internal/plugins/vrrp/register.go` calls `diagnostic.Register` for each code of the plugin's own table, and the plugin's `init()` reaches it through `registerVRRP` | The codes need a home that a build without the gate also compiles | `TestDoctorChecksAreRuntimePhase`, which reads the registered codes | settled by precedent, not yet validated by a test |
| A-7 | The stream handler can poll `show container log name` with `after` and receive only newer rows, internal and forked | `handler.go` `RegisterPluginStreamingHandler` and the page's description of `monitor interface rate` | The stream needs a subscription from the plugin | The stream functional test | unvalidated |
| A-8 | The network child names the host end of a link for any instance name of up to 63 characters. A network name is bounded at 11, which table S-2 enforces | The network child's Key Design Decisions row "Device names": the bridge is `zcn-` plus the network name, and a host end is `zc-`, the bridge's interface index and a slot, so no device name is built from an instance name. Its section "Config words this child implements" states that an instance name has no length bound from that child and bounds the network name at 11. Table S-2 of this spec carries that bound | The instance name pattern gets the bound that child states | That child's link naming test, and `TestNetworkNameBound` here | settled by the sibling's design, not yet validated by a test |
| A-10 | A list named `environment` inside a container entry trips no gate that reads the name: the env registration rule and the config claims gate judge Ze's own root | `ai/patterns/config-option.md` states the rule for leaves of the root. The gates were not read | The gate is corrected to judge the root only, since the word is kept by owner ruling | `./le config claims` and the scoped config package test, once the node exists | unvalidated |
| A-11 | The published Redis image keeps its data in its working directory, writes a snapshot there on the `SAVE` command before it answers, and loads it at start | Redis's own documentation of `SAVE` and of its snapshot file. Not in this repository | The closing test uses the append-only argument through the `argument` node | The closing test itself | unvalidated |
| A-12 | A path the image declares as a volume, with no `volume` entry in the config, stays in the container's writable layer | The runtime child's rule, row "Declared volumes" of the merge table in its section "Seams with the other children": such a path is not mounted and stays a directory of the writable layer. Its AC-9 and its Key Design Decisions row state the same. The image store child states it too, row "A declared volume with no configured volume" of its "Three rules of the record" | The closing test's config declares a `volume` at the data path | The runtime child's test of its AC-9, and the closing test | settled by the sibling's design, not yet validated by a test |
| A-9 | Each sibling exposes its facts inside the plugin through one read function | A-1: one plugin, one process | The state view cannot be built without a copy | Phase 4 | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A sibling uses a word for a node or a command that differs from tables S-1 to S-3 and C-1 | A sibling spec, or its schema, names a node or a path absent from the tables | This spec holds the one table of config words and command words. Its design is settled before any sibling's wiring phase, and its own implementation phases come after theirs. A sibling that needs a node or a command adds a row here first. `TestContainerCmdSchemaOwnsItsCommands` and the schema audit of Phase 2 compare what landed with the tables |
| R-2 | A rule is judged at validate and not at start, or the reverse | A config `ze config validate` accepts is refused at boot | One judging function, two callers, and a functional test that runs both doors on the same files |
| R-3 | A secret reaches a listing, a log line or a finding | A variable's value in the JSON of the detail | The detail carries names only. A unit test fails on any value |
| R-4 | The word `container` already means "Ze runs inside a container" in one platform code and its guide page (`docs/guide/docker.md`) | An operator reads a container finding as a platform finding | Codes take the `doctor-container-` prefix, and the new guide page links the other in its first paragraph |
| R-5 | A state read races a transition, so a test sees the state before the one it waits for | A flaky functional test | Tests wait on the state with the runner's wait step, never on time |
| R-6 | The `cpu` default refuses every config on a kernel without the option (A-4) | The first functional test fails at commit | The runtime child pins the option before this child's Phase 1 |
| R-9 | A short image name resolves to an image the operator did not expect, or `latest` moves under a running container | The digest in the listing differs from the one the operator tested | The whole reference and the digest are shown in every listing and in the fetch answer. A container stays on its recorded digest until `update container image name` |
| R-10 | The closing test depends on a public registry at test time | The test fails with no network | The image store child's test registry serves the real Redis image as a mirror on a loopback address, so the test types the short name an operator would |
| R-7 | A long fetch on REST or the web outlives the client's timeout and is stopped half way | A fetch that never ends on a slow link | Verified data stays for the next attempt, the image listing shows progress, and MCP uses a task |
| R-8 | The log buffer grows without bound | Memory use rises with a chatty container | The buffer is bounded by the runtime child. `lines` is capped at 10000 |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | Nothing outside the feature: a config with no `container` root is untouched. Inside it, a wrong word is a contract in the field once shipped: a renamed leaf breaks every saved config, and a renamed command every script |
| How is it reverted? | Before release, by changing the tables and the schema together. After release, a rename needs a conversion of saved configs |
| Who else touches this path? | The three sibling children add nodes and commands to the same schema modules. The shell child adds its three commands. The health check child, when it is written, adds its node |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| A `container` block in a config file given to `ze config validate` | → | The schema module and the judging function, through the in-process verifier | `container-refuses-cross-leaf` |
| The same file at daemon start | → | The same judging function, through the plugin's configure callback | `container-validate-agrees-with-boot` |
| `show container list` over SSH | → | The state view and the list handler in the plugin's process | `container-list-running` |
| `show container list` with the plugin forked | → | The same handler over the plugin protocol | `TestContainerListAnswersWhenForked` |
| `monitor container log name` over SSH exec | → | The stream handler and the log handler | `container-log` |
| `ze doctor` with a container waiting for its image | → | The runtime check and its `OnDoctorCheck` answer | `container-waiting-image` |
| A scrape of the exporter | → | The state metrics | `container-metrics` |
| Every command on REST, gRPC, MCP and the web | → | The registrations of table C-1 | `TestEveryCommandOnEverySurface` |
| A `container` block in a build without the gate | → | `config.KeywordHint` | `TestContainerRootRefusedWithoutTheGate` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | A config names a container whose image is in the store, and is committed (umbrella AC-1, operator half) | `show container list` holds a row for it with `state` `running`, the digest of its image and `restart-count` 0. `show container detail name` adds its process id, the limits in force and its addresses | A container with `enabled false` shows `disabled`, and one whose image is absent shows `waiting-image`, because each is what the operator configured or has not fetched yet |
| AC-2 | A config names a container whose image is not in the store (umbrella AC-2, operator half) | The row shows `waiting-image` and `reason` names the image reference. `show doctor` holds `doctor-container-image-absent` with the fetch command as its fix. After that command the row shows `running` with no further command, and the finding is gone | Other containers of the same commit show their own states, because one missing image does not hold the rest |
| AC-3 | A container's entrypoint exits with a failure (umbrella AC-10, operator half) | Under `on-failure` the row shows `restarting`, then `running`, with `restart-count` raised and `last-exit-status` holding the exit code. After three such exits `show doctor` holds `doctor-container-restarting` with the log command as its fix. Under `never` the row shows `stopped` and doctor holds `doctor-container-stopped` | A container that exits with status 0 under `on-failure` shows `stopped` and raises no finding, because a clean exit is not a failure |
| AC-4 | A container that cannot start, and one with `enabled false` | The first shows `failed` with a `reason` sentence that names the cause, and doctor holds `doctor-container-failed` as an error. The second shows `disabled`. `request container restart name` on the second is refused with the config statement that enables it | A `failed` container starts after `request container restart name` once its cause is gone, because the command is the stated way out |
| AC-5 | Config files that break each rule of table V-1, given to `ze config validate` with no daemon, then to a daemon at start | Both refuse each file, with the message of table E-1 naming the container, the setting and the fix line. A file that breaks no rule is accepted by both | A container with no network and no host namespace is accepted, because an isolated container is the most confined form |
| AC-6 | A config sets `memory`, `shared-memory`, `cpu` or `process-limit` to 0, and another sets only `image` | The first is refused by the schema with the accepted range and the `set` line. The second is accepted, and the detail shows a memory limit of 512 mebibytes and the `on-failure` policy | Any value inside a range is accepted, including the largest, because the bound on the host is the runtime child's rule |
| AC-7 | `ze` built without the container gate reads a config with a `container` block, and is sent `show container list` (umbrella AC-13, schema half) | The config is refused as an unknown keyword, with the line naming the missing plugin and the fix line naming the build tag. The command is refused as unknown | Every other root and command answers as in a build with the gate, because nothing else depends on the plugin |
| AC-8 | The command set of table C-1 in a build with the gate | Every path is in the command catalog with its leaves, its shape and its column order. REST, gRPC, MCP and the web list each one, and the stream is listed with its one-shot sample | A path a sibling has not landed yet is absent until that sibling lands, and its test row stays red until then, because this child does not fake a handler |
| AC-9 | `delete container image` and `request container restart` typed with nothing after them | Each is refused, and the refusal lists the complete forms: `delete container image all`, `delete container image digest`, `request container restart name` | `delete container image all` acts on every unused image, because the operator typed the word |
| AC-10 | `show container log name` and `monitor container log name` for a running container that writes output | The first answers rows with a sequence, a time, the stream and the line, newest last, and `lines 5` answers five. The second writes each new row as it arrives over SSH, and answers the newest rows once on REST and MCP | A container with `log none` is refused with the config statement that keeps output, because nothing was kept |
| AC-11 | A config with an `environment` entry given as `secret` and a registry `password` | `show config` masks both. The container detail lists the variable's name. A search of the detail, the list, the doctor answer and the log rows Ze writes finds the names and the masked form only | An `environment` entry given as `value` is shown in the config, because the operator chose the plain form. It is still absent from the detail |
| AC-12 | A scrape of the exporter with one running and one waiting container | `ze_container_instances` holds 1 for `running` and 1 for `waiting-image`, and `ze_container_instance_running` holds 1 and 0 for the two names | The other states hold 0, because no container is in them |
| AC-13 | A browser opens the config editor and the admin tree | The editor shows the `container` root with its three lists, the `restart` choices, the `memory` default and a refusal for `memory 0`. The admin tree holds `show container list` and a form with one field for `request container restart name` | The log stream shows one sample and no live view, because the web console runs a command once |
| AC-14 | `./le cli grammar`, `./le cli ownership`, `./le doc yang-contract command-contract` and `help-shape` over the tree | Each passes with the container commands and nodes in its population | Nodes a sibling has not landed are not in the population yet, because they are not in the tree |
| AC-15 | The owner's closing test (umbrella AC-14), in the appliance guest: the operator fetches the published Redis image, configures a container for it on a container network with port 6379 published, allows that port in the firewall policy, and commits (section X-1, steps 0 to 5) | The listing shows the container running with the fetched digest. A test client that runs beyond the router, in a separate network namespace the test joins to the router by a link, speaking the Redis protocol itself, connects to the router's address on that link and the published port, gets the reply to a ping, stores a value and reads it back. After `request container restart name` the value is read back again, because Redis wrote it to the container's persistent filesystem. With the policy term for the published port then deleted and committed, a new connection from the same namespace is not established, and `show doctor` holds `doctor-container-port-blocked` with the term's `set` line as its fix: a published port must also be allowed by the firewall policy | A connection from the same outside namespace to port 6380, which the container does not publish, is refused, because only the published port is opened |
| AC-16 | A config with a container is validated, started and reloaded by a daemon that does not run as root, and on a host whose data directory cannot cap a folder's size | Each of the three refuses with the message of table E-1 for V-11 or V-12, which says why and gives the fix, and doctor reports the cause with its fix. Each is judged on the host where the command runs, and a validate run names the user it judged. The same config is accepted on the appliance and under a root daemon whose data directory enforces caps | A config with no container is accepted on those hosts, because nothing needs the privilege or the cap |
| AC-17 | A container publishes a port and the firewall policy drops traffic to it; then the operator adds the term the finding names | `show doctor` holds `doctor-container-port-blocked` naming the container, the protocol and the port, with the `set` line of the term as its fix. After the term is committed the finding is gone and a connection to the published port is answered | A published port the policy allows raises no finding, because nothing blocks it |
| AC-18 | `set container instance web image redis`, then the same with `redis:7`, a whole reference and a digest reference; then the image is fetched | Each form is accepted. The fetch answer, the image listing and the container listing show the whole reference with its registry host and tag, and the digest it resolved to | A reference with a malformed digest or an empty tag is refused by the schema, because no registry could serve it |
| AC-19 | A container with `capability-drop all` and `capability net-bind-service`, one with `capability-drop chown`, and one with `chown` in both lists | The detail of the first shows one capability, of the second the default set less `chown`. The third is refused with the message of V-10 | A container with neither node shows the fourteen default capabilities, because that is the default set |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Writes a container in the config, commits, and reads the listing | editor -> schema -> verifier -> plugin apply -> state view -> `show container list` | `container-list-running` |
| 2 | Commits a container before fetching its image, asks doctor, runs the command doctor gives | commit -> state `waiting-image` -> runtime check -> `create container image reference` -> state `running` | `container-waiting-image` |
| 3 | Validates a config on a laptop before sending it to the router | `ze config validate` -> schema -> in-process verifier | `container-refuses-cross-leaf` |
| 4 | Follows a container's output while it restarts | `monitor container log name` -> stream handler -> log handler -> buffer | `container-log` |
| 5 | Asks why a container is not running, from an AI agent over MCP | MCP tool -> dispatcher -> `show container detail name`, then `show doctor` | `TestEveryCommandOnEverySurface`, `container-restart-state` |
| 6 | Edits a container in the web UI | web editor -> schema -> commit | `container-editor` |
| 7 | Runs a Redis server on the router: fetches the image, configures the container with a published port and its firewall term, commits, uses it from a host beyond the router, restarts it and finds the data | fetch -> store -> config -> verifier -> runtime start -> link and translation -> firewall policy -> client -> restart -> writable layer | `container-redis` |
| 8 | Publishes a port, finds it unreachable, and asks doctor | published port -> firewall policy -> runtime check -> finding with the term to add | `container-port-blocked` |

## 🧪 TDD Test Plan

Every unit test file below is new, under the plugin folder of A-1, and is listed in Files to Create.

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestVerifyRefusesHostNetworkWithAttachment` | `verify_test.go` | V-1 and its message | |
| `TestVerifyRefusesMemoryMountsAboveMemory` | `verify_test.go` | V-3 | |
| `TestVerifyRefusesAddressOutsidePrefix` | `verify_test.go` | V-4, prefix | |
| `TestVerifyRefusesAddressHeldTwice` | `verify_test.go` | V-4, held | |
| `TestVerifyRefusesOverlappingNetworks` | `verify_test.go` | V-5 | |
| `TestVerifyRefusesUnequalPortRanges` | `verify_test.go` | V-6 | |
| `TestVerifyRefusesOnePathTwice` | `verify_test.go` | V-7 | |
| `TestVerifyAcceptsIsolatedContainer` | `verify_test.go` | The permitted survivor of AC-5 | |
| `TestVerifierAndConfigureCallOneJudge` | `verify_test.go` | The two doors reach the same function with the same payload | |
| `TestStateViewNamesTheWaitingImage` | `state_view_test.go` | T-1, `waiting-image` | |
| `TestStateViewRestartingCarriesCountAndExit` | `state_view_test.go` | T-1, `restarting` | |
| `TestStateViewStoppedByPolicy` | `state_view_test.go` | T-1, `stopped`, and no finding for exit 0 | |
| `TestDetailCarriesNoVariableValue` | `command_detail_test.go` | R-3 | |
| `TestLogAfterAnswersOnlyNewerRows` | `command_log_test.go` | The cursor of the stream | |
| `TestDoctorImageAbsentNamesTheFetchCommand` | `doctor_test.go` | D-1 row 1 and its fix | |
| `TestDoctorChecksAreRuntimePhase` | `doctor_test.go` | The phase and the registered codes | |
| `TestMetricsFollowState` | `metrics_test.go` | M-1 | |
| `TestContainerListAnswersWhenForked` | `command_list_test.go` | The list over the plugin protocol | |
| `TestContainerCmdSchemaOwnsItsCommands` | `self_containment_test.go`, in the plugin's schema package | The paths of C-1 are declared by the owner | |
| `TestContainerSchemaHasNoUnlimitedValue` | `schema_test.go`, in the plugin's schema package | Every limit's range starts above zero | |
| `TestContainerRootRefusedWithoutTheGate` | The container absent-gate test file of the hub package | V-2, the hint text and the unknown command | |
| `TestVerifyRefusesCapabilityAddedAndDropped` | `verify_test.go` | V-10 | |
| `TestHostRefusalMessagesNameTheFix` | `verify_test.go` | The V-11 and V-12 messages: the problem, why, and the fix on the last line | |
| `TestPortBlockedFindingNamesTheTerm` | `doctor_test.go` | D-1 row 6: the message and the `set` line of the term | |
| `TestImageNameFormsAccepted` | `schema_test.go`, in the plugin's schema package | The short, whole and digest forms of `image`, and the refused malformed ones | |
| `TestNetworkNameBound` | `schema_test.go`, in the plugin's schema package | A network name of 11 characters is accepted and one of 12 refused | |
| `TestRespClientRoundTrip` | `internal/test/respclient/respclient_test.go` | The test client encodes a command and decodes the simple, error and bulk replies, against a scripted server | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `memory` | 64-1048576 | 1048576 | 63 | 1048577 |
| `shared-memory` | 1-1048576 | 1048576 | 0 | 1048577 |
| `cpu` | 1-102400 | 102400 | 0 | 102401 |
| `process-limit` | 1-32768 | 32768 | 0 | 32769 |
| `storage` | 1-1048576 | 1048576 | 0 | 1048577 |
| `stop-timeout` | 1-30 | 30 | 0 | 31 |
| `store largest-image` | 1-65536 | 65536 | 0 | 65537 |
| network `name` length | 1-11 | 11 | 0 | 12 |
| `user`, `group` | 0-65535 | 65535 | N/A | 65536 |
| `tmpfs size`, `volume size` | 1-1048576 | 1048576 | 0 | 1048577 |
| `mtu` | 68-16000 | 16000 | 67 | 16001 |
| `listen-port`, `container-port`, `mirror port` | 1-65535 | 65535 | 0 | 65536 |
| `lines` | 1-10000 | 10000 | 0 | 10001 |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `container-schema-accepts` | the parse suite | A config using every node validates | |
| `container-refuses-unlimited` | the parse suite | A limit of 0 is refused with the range and the fix line | |
| `container-refuses-cross-leaf` | the parse suite | Each rule of V-1 refuses its file with its message | |
| `container-validate-agrees-with-boot` | the container suite | The same files get the same verdict at start | |
| `container-list-running` | the container suite | A committed container is listed as running with its digest | |
| `container-waiting-image` | the container suite | A missing image is named in the list and in doctor, and the fetch starts the container | |
| `container-restart-state` | the container suite | A failing entrypoint shows its count and exit status, and the policy `never` shows `stopped` | |
| `container-failed-disabled` | the container suite | The `failed` and `disabled` states, and the refused restart | |
| `container-log` | the container suite | The log rows, `lines`, the stream over SSH and the one-shot sample | |
| `container-secrets` | the container suite | A secret is masked in the config and absent from every answer | |
| `container-metrics` | the container suite | The state metrics follow two containers | |
| `container-commands` | the ui suite | Completion offers the paths and the container names, and a bare set action is refused with its forms | |
| `container-editor` | the web suite | The web editor and the admin tree show the root and the commands | |
| `container-host-refused` | the container suite | An unprivileged daemon, and a data directory with no size cap, refuse a container config with the reason and the fix (AC-16) | |
| `container-port-blocked` | the container suite | A published port the policy drops is reported, and the term the finding names clears it (AC-17) | |
| `container-image-names` | the container suite | The short, whole and digest forms are fetched from the test registry and listed with the whole reference and the digest (AC-18) | |
| `container-capability-drop` | the container suite | The capability set of the detail follows the two nodes (AC-19) | |
| `container-redis` | the appliance suite | The closing test of section X-1 (AC-15) | |

Each test is drafted in the draft tree of its suite and promoted. The `container` suite is registered by the runtime child. The tests that start a container carry `option=needs-linux` and run in the QEMU guest, on the fixture image the runtime child provides. `container-redis` runs in the appliance guest and takes its image from the image store child's test registry.

## Files to Modify
- `docs/guide/command-reference.md` - the commands of C-1
- `docs/guide/configuration.md` - the `container` root
- `docs/guide/monitoring.md` - the metrics of M-1
- `docs/guide/docker.md` - one line that points to the new guide page for the other meaning of the word
- `docs/architecture/doctor-and-health-checks.md` - the plugin runtime checks of D-1
- `ai/INDEX.md` - the keyword row for containers Ze runs

No existing feature file is modified: every feature file of this child is new with the plugin, and is listed below. The schema module, the verifier and the registration are created by the runtime child when it lands first, and this child then edits them. The feature declaration is created by the runtime child too, and this child extends it (Documentation checklist row 1).

## Files to Create
- `internal/plugins/container/yang/ze-container-conf.yang` - the nodes of tables S-1 to S-3. Created by the first child to land. This child adds the nodes marked "operator surface" and corrects any node that differs from the tables
- `internal/plugins/container/yang/ze-container-cmd.yang` - the command nodes of C-1, with their leaves and both help texts
- `internal/plugins/container/verify.go` - the judging function, its rules owned here, and its two callers
- `internal/plugins/container/register.go` - the registration, with the command declarations of this child, the doctor checks and the metrics hook
- `internal/plugins/container/register_stream.go` - the log stream registration
- `internal/plugins/container/state_view.go` - the one record per container of T-1
- `internal/plugins/container/command_list.go`, `internal/plugins/container/command_detail.go`, `internal/plugins/container/command_log.go` - the handlers owned here
- `internal/plugins/container/doctor.go` - the checks of D-1
- `internal/plugins/container/metrics.go` - the metrics of M-1
- `internal/plugins/container/verify_test.go`, `internal/plugins/container/state_view_test.go`, `internal/plugins/container/command_list_test.go`, `internal/plugins/container/command_detail_test.go`, `internal/plugins/container/command_log_test.go`, `internal/plugins/container/doctor_test.go`, `internal/plugins/container/metrics_test.go` - the unit tests
- `internal/plugins/container/yang/self_containment_test.go`, `internal/plugins/container/yang/schema_test.go` - the schema tests
- `cmd/ze/hub/build_tag_container_absent_test.go` - the absent-gate test. Created by the runtime child when it lands first, and this child then adds its test
- `test/parse/container-schema-accepts.ci`, `test/parse/container-refuses-unlimited.ci`, `test/parse/container-refuses-cross-leaf.ci` - the parse tests
- `test/container/container-validate-agrees-with-boot.ci`, `test/container/container-list-running.ci`, `test/container/container-waiting-image.ci`, `test/container/container-restart-state.ci`, `test/container/container-failed-disabled.ci`, `test/container/container-log.ci`, `test/container/container-secrets.ci`, `test/container/container-metrics.ci` - the daemon tests
- `test/container/container-host-refused.ci`, `test/container/container-image-names.ci`, `test/container/container-capability-drop.ci` - the tests of AC-16, AC-18 and AC-19. The test of AC-17, `container-port-blocked`, is the network child's file, because that child owns the judgment. This child's part is the message, proven by `TestPortBlockedFindingNamesTheTerm`
- `test/appliance/container-redis.ci` - the closing test of section X-1
- `internal/test/respclient/respclient.go`, `internal/test/respclient/respclient_test.go` - the Redis protocol client the closing test uses, test code only
- `test/ui/container-commands.ci` - completion and refusals
- `test/web/container-editor.wb` - the browser test
- `docs/guide/containers.md` - the guide: the model, a first container, the image commands, the states, the mapping from the model operators know to Ze's words
- `docs/architecture/container/operator-surface.md` - the design page: tables S-1 to S-4, C-1, T-1 and the decisions of this spec

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | The two schema modules in Files to Create |
| YANG validation constraints | Yes | Every leaf of S-1 to S-3 carries a range, length, pattern, enumeration or `zt` type |
| YANG custom validators | No | By design: a `ze:validate` under a gated root needs an always-on list to name the root. Cross-leaf rules are in the verifier |
| CLI commands/flags | Yes | Command nodes in the plugin's command module, with proxy registrations in the plugin's package. No flag |
| CLI grammar (keyword before value) | Yes | Table C-1, proven by `./le cli grammar` |
| Editor autocomplete | Yes | Automatic for enumerations and the reference. Container names in commands complete from the config |
| Functional test for new RPC/API | Yes | The daemon tests in Files to Create |
| Pipe completeness | Yes | Every answer is data with a declared shape and columns |
| Env var registration | N-A | No leaf under Ze's `environment` root. The workload's variables are the `environment` list inside a container entry, which is not that root (A-10) |
| Doctor check for runtime dependencies | N-A | This child adds no file path, socket or kernel dependency. Those belong to the siblings |
| Doctor code per problem class | Yes | Table D-1, in the plugin's doctor file |
| Prometheus counters/metrics | Yes | Table M-1 |
| BGP family surface (new SAFI / capability / attribute) | N-A | No BGP surface |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | Yes | `features/containers.md`: the runtime child creates it, and this child extends it with the operator surface's scope and evidence |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md`: the `container` root. The syntax page: No, the grammar is unchanged |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | No | `docs/architecture/api/commands.md` describes mechanisms, and none changes |
| 5 | Plugin added/changed? | No | The plugin is added by the runtime child, which owns that row |
| 6 | Has a user guide page? | Yes | `docs/guide/containers.md` |
| 7 | Wire format changed? | No | No wire encoding |
| 8 | Plugin SDK/protocol changed? | No | The SDK is used as it is |
| 9 | RFC behavior implemented, changed, or newly proven? | No | No RFC |
| 10 | Test infrastructure changed? | No | The tests use the existing suites |
| 11 | Affects daemon comparison? | Yes | The comparison page: the row for running workloads |
| 12 | Internal architecture changed? | Yes | `docs/architecture/container/operator-surface.md` |
| 13 | Route metadata keys added/changed? | No | None |
| 14 | Prometheus counters added/changed? | Yes | `docs/guide/monitoring.md` |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | The status guide and the generated inventories, for the commands |
| 16 | Any changed source file referenced by existing doc source anchors? | No | Every source file this child changes is new with the feature. Re-derived with `./le spec citation anchors` at implementation |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/docker.md` is about Ze inside a container: one line points to the new page |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- declare the command nodes, their proxies and the stream, with handlers that answer an empty payload
   - Tests: the Wiring Test table
   - Files: the command module, the registration, the stream registration, the list handler
   - Verify: each path is reachable on every surface, internal and forked. The wiring tests fail because the answers are empty. A-2 and A-7 are settled here
2. **Phase: Schema audit** -- compare the module the siblings landed with tables S-1 to S-3, add the nodes owned here, and prove the ranges and help texts
   - Tests: `container-schema-accepts`, `container-refuses-unlimited`, `TestContainerSchemaHasNoUnlimitedValue`, the boundary table
   - Files: the config module
   - Verify: tests fail, then pass. `./le doc yang-contract help-shape` is owed
3. **Phase: Offline verification** -- the rules of V-1 owned here and the two callers
   - Tests: the `TestVerify*` tests, `container-refuses-cross-leaf`, `container-validate-agrees-with-boot`
   - Files: the verifier
   - Verify: validate and boot agree on every file
4. **Phase: State view and listings** -- the record of T-1, the list and the detail
   - Tests: the `TestStateView*` tests, `TestDetailCarriesNoVariableValue`, `container-list-running`, `container-restart-state`, `container-failed-disabled`, `container-secrets`
   - Files: the state view, the list and detail handlers
   - Verify: each state is reached and shown
5. **Phase: Log and stream**
   - Tests: `TestLogAfterAnswersOnlyNewerRows`, `container-log`
   - Files: the log handler, the stream registration
   - Verify: the stream writes new rows, and the one-shot surfaces answer the sample
6. **Phase: Doctor and metrics**
   - Tests: the `TestDoctor*` tests, `TestMetricsFollowState`, `container-waiting-image`, `container-metrics`
   - Files: the doctor and metrics files
   - Verify: each finding appears with its fix and clears, and `./le arch doctor-coverage check` is owed
7. **Phase: Absent gate, surfaces and web**
   - Tests: `TestContainerRootRefusedWithoutTheGate`, `TestEveryCommandOnEverySurface`, `container-commands`, `container-editor`
   - Files: the absent test
   - Verify: A-5 is settled by the browser test
8. **Phase: Pages** -- the guide, this child's part of the feature declaration, the command reference, the config guide, the design page
   - Files: the documentation rows above
   - Verify: `./le doc check all` is owed
9. **Phase: Closing test (LAST, of the whole container work)** -- the Redis protocol test client, then the test of section X-1
   - Tests: `TestRespClientRoundTrip`, `container-redis`
   - Files: the test client package, the appliance test
   - Verify: the test is red with the published port's firewall term removed from step 4 (at the connection of step 6 and at the doctor assertion of step 4), with the term left in place at step 9 (the connection is then established) and with the restart replaced by a writable-layer clear, and green as written. A-11 and A-12 are settled here

### Critical Review Checklist

| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file:line |
| Feature completeness | Every user story has a working path, no broken links |
| Correctness | Every node of the landed module equals its row in S-1 to S-3: name, type, range, default. Every path equals C-1 |
| Naming | JSON keys are kebab-case and equal the leaf names. No node, help text, page or message names the reference system |
| Data flow | One judging function behind both doors. One state record behind the list, the detail, doctor and metrics |
| Operator errors and doctor codes | Every refusal is an `opererr` error with its fix on the last line, every `Warn`/`Error` line and raise names the code the Integration Checklist's doctor row lists, and each doctor check carries `Fix` at the phase that holds its state (`ai/rules/user-facing-errors.md`) |
| Rule: process boundary | No handler, check or metric reads plugin state from the daemon's process. Each test of a command runs forked too |
| Rule: compile tag | No always-on file holds the word `container` for this feature: not the validator section list, not the code table, not a help string |
| Rule: no silent limit | No range admits 0 for a limit, and no default depends on a kernel option the runtime kernel lacks |

### Deliverables Checklist

| Deliverable | Verification method |
|-------------|---------------------|
| The schema matches tables S-1 to S-3 | `./le test unit pkg path` over the plugin's schema package |
| The command set matches C-1 | `./le cli grammar`, then `./le doc yang-contract command-contract` |
| Validate and boot agree | The `container` functional suite, the case `container-validate-agrees-with-boot` |
| The closing test passes | The appliance suite in the appliance guest, the case `container-redis` |
| Every command on every surface | `./le test unit pkg path ./cmd/ze/hub/ run TestEveryCommandOnEverySurface` |
| The absent build refuses the root | The absent test, run without the container tag |
| The pages exist and their anchors hold | `./le doc check all` |

### Security Review Checklist

| Check | What to look for |
|-------|-----------------|
| Input validation | Paths inside the container are absolute with no parent-directory segment. A device path starts with the device directory. An image reference, a registry key and a container name each match a pattern before any code uses them in a path or a URL |
| Secrets | The registry password and a variable's `secret` are `ze:sensitive`. No answer, finding, metric label or Ze log line carries a variable's value or a credential |
| Authorization | Every command that changes the system is under an action verb, so the read-only profile refuses it. The log may hold a workload's secrets: it is a read, and the guide says so |
| Grants | `capability`, `host-namespace` and `device` are the only nodes that widen confinement. Each value is a closed keyword with a help line that says what it grants. No node grants everything |
| Resource exhaustion | `lines` is capped. The listing sizes are bounded by the config. A fetch stops with its caller |
| Error leakage | A registry's error text is shown after the plain sentence, never in place of it, and with credentials removed |
| Set actions | `delete container image all` is the only everything form, and it never removes an image a configured container holds |

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

- The reference model's commit-time rewrite and its "zero means unlimited" values share one cause: a slot that holds a keyword or a sentinel beside a real value. Removing the sentinel and letting absence mean "derived" resolves each case with no new node.
- A gated plugin cannot use `ze:validate`, because the walk that runs it reads an always-on list of section names. Native types, references and the plugin's own verifier are the routes that need no central edit.

## Owned journal rows
| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| The list of containers is named `instance` | Keeping the list named for its key word; `workload` | A list named for its key says nothing about what it holds, and the key word is the commands' selector keyword. `instance` is what an entry is: one running instance of an image |
| The workload's variables keep the word `environment` (owner ruling) | A new word, `variable` | The reference model's words are kept wherever a Ze rule does not forbid them. A list inside a container entry does not collide with Ze's own root. `secret` is added as the masked form |
| Short image names resolve against one default registry, and no tag means `latest` (owner ruling) | A whole name with a mandatory tag; a search across several registries | Operators type short names. Ze records and shows the digest and the whole reference the name resolved to, so the default is never hidden |
| A second leaf-list, `capability-drop`, removes from the default set | A container with `add` and `drop` leaves; negated values in one list | The reference model's `capability` node stays as it is, and a list of closed keywords needs no sign in a value |
| The closing test makes Redis persist with an explicit `SAVE` from the client | The append-only argument in the container's config | The container is configured as a first container is, and the test does not depend on the image's default save settings |
| Every command ends in its typed selector (`name`, `digest`, `reference`) | The selector kind before the action word | The streaming registry matches a literal prefix, and the command precedent read has the selector last. One shape serves reads, the stream and actions. The mandatory leaf is the guard, as the verbs page states for `request plugin restart` |
| The verbs are `create`, `update`, `delete`, `clear` and `request` | `request` for every operation | The verbs page asks the narrower verbs first, and lists commands that took `request` too early as defects |
| No start and no stop command | A start and a stop under `request` | The config states whether a container runs. A second statement outside it would not survive a reboot |
| No limit has an "unlimited" value | Keeping zero as the sentinel | The umbrella makes limits mandatory. A range that starts above zero lets the schema produce the refusal and its fix |
| An absent hardware address means derived, and nothing is written to the config | A keyword in the slot, and a rewrite at commit | Ze changes the operator's config only through the operator's own edits. The derived value is shown in the listing |
| Volumes are stored by Ze with a size cap | Binding a host path | A host path gives a container the router's own files, and has no cap. The umbrella sizes and stores volumes |
| Outbound translation is on by default, with a node that turns it off (owner ruling) | Off by default | The reference model translates, and a container on a private prefix reaches out with no further config. The node serves a network whose prefix the router routes |
| Cross-leaf rules are in one function with two callers | `ze:validate` annotations; rules in the verify callback only | The first needs an always-on list to name a gated root. The second passes offline and fails at start |
| Leaf-lists and keyed lists are named in the singular | The plural the config-option pattern asks for | Every leaf-list and keyed list of the interface, system and firewall schema is singular, and an operator types those daily |

## Known Limitations
- A volume belongs to one instance. Two containers cannot share one.
- The log is a bounded buffer in memory. It does not survive a daemon restart.
- The health check and container labels are not part of this work now, by owner ruling. The health check is work owed to a spec of its own, a later child of the container work (`plan/post-release/features/spec-container-6-health-check.md`, a skeleton, which depends on the shell child and runs its command through that child's bounded run). Table S-4 holds the Ze form that spec takes.
- Ze answers no container name over DNS at first. A container gets the router's resolver list.
- Containers run on the appliance and under a root daemon at first. Elsewhere a container config is refused with the reason.
- A container network has no VRF node until VRF support ships.
- There is no I/O throttle node.
- The web UI has no container page beyond the editor and the admin tree.

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
- [ ] The targeted tests for what changed pass, run in the main checkout by the main thread: the scoped package tests, the component groups and functional suites `./le repo changed scope print both` reaches. No agent runs any `./le verify` command: GitHub CI verifies the pushed commit, and the full verification is Thomas's release check. A red CI reports gets a fix commit
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied: lint fixed rather than disabled, the focused check for the changed behavior run once with its OUTPUT PASTED, and any red named with the one-line reason it is scaffolding
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
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script (commit A preserves the spec in history)
