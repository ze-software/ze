# Spec: generated-command-usage

| Field | Value |
|-------|-------|
| Status | in-progress |
| Scope | cli |
| Depends | - |
| Phase | - |
| Deferral shard | `plan/deferrals/generated-command-usage.md` |
| Handoff | - |
| Updated | 2026-08-29 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Eighty hand-written `Usage:` sentences live inside YANG `description` strings,
spread over 24 `-cmd` modules. Each one states the argument grammar of one
command in prose. Nothing checks them, nothing generates them, and nothing
reads them: the published catalog once got its syntax column from a retired
Python post-processing step (`usage_syntax` in the deleted
`website/tools/render-cli-catalog.py`, removed in `eae282592`), never from
`ze help command --json`. `commandEntry` in `cmd/ze/help_command.go` has never
carried a usage or syntax field.

Prose is the wrong home for this twice over. `ai/rules/cli.md` already forbids
it: "A `description` states what the leaf MEANS. It MUST NOT prescribe a CLI
spelling." That rule has no feeder for the `Usage:` case, so 80 violations
accumulated. And prose drifts silently: `create interface dummy name <name>
unit <vid>` is what the description claims, while the model declares no leaf
for `<vid>` at all.

The goal is that every command's usage line is GENERATED from the command
model, that the prose is deleted, and that a gate keeps the two honest while
the deletion happens command by command. A command the model cannot express
keeps its authored sentence and stays a reported difference, and the way to
close it is to make the model able to express it.

Five failure classes stand between the model and a generated line. Four were
found by survey, the fifth by reading the announce module. They are enumerated
under Current Behavior and each one owns an implementation phase.

## Required Reading

### Architecture Docs
- [ ] `ai/rules/cli.md` - the grammar the generated line must render, and the
      rule the prose already breaks
  → Constraint: every command places a closed keyword before any user-supplied
    value, so an optional value is rendered as `[keyword <value>]` and never as
    a bare optional positional.
  → Constraint: a `description` MUST NOT prescribe a CLI spelling. Deleting the
    80 `Usage:` sentences is compliance with an existing rule, not a new policy.
  → Constraint: `selector` MUST NOT leak into operator syntax. The announce
    module declares a mandatory `selector` leaf, so the generated line for
    `announce` must not render the word `selector` as a keyword.
  → Decision: the static grammar gate is `./le cli-grammar`, and its R9 check
    refuses a sibling collision. Any new sibling container this spec adds passes
    that gate before it is considered modelled.
- [ ] `docs/architecture/api/commands.md` - the published command contract
  → Constraint: handler error text already spells usage strings by hand, such as
    the `show traffic usage` row's `usage: show traffic usage [name <interface>]`.
    Those are handler-side and out of scope here, but the generated renderer is
    the obvious later source for them.
- [ ] `ai/patterns/cli-command.md` - the structural template for a command
  → Constraint: a new command's YANG node is where its grammar is declared, so
    the pattern doc gains the rule that the grammar is declared, never described.
- [ ] `ai/rules/simplicity.md` - the shape of the fix
  → Decision: alternation between whole argument forms is already expressible as
    sibling containers. A new alternation mechanism would be machinery the model
    does not need, so the fifth failure class is modelled, not annotated.

### RFC Summaries (Scope: protocol)
- N-A. This spec changes no wire-visible behavior and implements no RFC.

**Key insights:** (minimal context to resume after compaction)
- Position comes from the PATH, not from the leaf list. A leaf whose name equals
  a container name on the path belongs immediately after that container's
  keyword. This one rule explains both the `create interface dummy name <name>
  unit <vid>` position and the `remote <name> called <number>` order.
- The order used for POSITIONAL BINDING and the order used for DISPLAY are
  different concepts. Binding must stop depending on slice order before display
  order is allowed to change, or `show system sockets 8080` silently binds to
  the wrong leaf.
- `ze:syntax` is TAKEN and means the config parsing mode
  (`internal/component/config/yang/modules/ze-extensions.yang`), so it is not a
  free name for anything about command usage.

## Current Behavior (MANDATORY)

**Source files read:** (must read BEFORE you write this spec)
- [ ] `cmd/ze/help_command.go` - builds the published catalog. `commandEntry`
      has fields for path, description, mode, wire method, backend, task
      support, args, pipes, operators, answer shape, address fields, aliases and
      subcommands, and no usage field. `extractArgs` copies `node.ArgDefs` into
      a flat `commandArg` list with name, type, values and mandatory, and no
      position.
- [ ] `internal/component/config/yang/command.go` - `extractArgDefs` iterates
      `entry.Dir`, which is a map, then ends with a sort on `Name`. That sort is
      what makes the result deterministic today, and it is alphabetical rather
      than declared. `mergeYANGEntry` extracts argument definitions only for a
      node that carries a wire method, so an intermediate container that
      declares leaves contributes none.
- [ ] `internal/component/command/node.go` - `ArgDef` carries Name, Kind,
      EnumValues, UintBits, Ranges, Pattern, UnionDefs, Mandatory. No position,
      no anchor. `Node.Children` is a map, so child order is not carried either.
- [ ] `internal/component/plugin/server/command.go` - `positionalDef` walks the
      definition slice in slice order in two passes, mandatory tier then
      optional, and returns the first definition whose `ValidateArgString`
      succeeds. Its own comment records that the author already met this hazard
      once and fixed the mandatory half of it. Within a tier, slice order still
      decides.
- [ ] `internal/component/command/help.go` - `writeHelp` prints the full
      description ONLY when the node has no children. A command node that also
      has subcommands, such as `create interface dummy name`, never shows its
      description at all: the operator gets a child listing. `writeHelpEntry`
      renders listings through `helpfmt.Summary`.
- [ ] `internal/core/helpfmt/helpfmt.go` - `Summary` returns the first sentence
      or first line, so a listing already drops the `Usage:` sentence that
      follows it.
- [ ] `internal/component/iface/yang/ze-iface-cmd.yang` - the `unit` container
      under `create/interface/dummy/name` carries
      `ze:command "ze-iface:interface-unit-add"`, one `name` leaf described as
      "Parent interface name", and the prose
      `create interface dummy name <name> unit <vid>`. There is no leaf for
      `<vid>`, and the one leaf present belongs to the parent's keyword.
- [ ] `internal/component/l2tp/cmd/yang/ze-l2tp-cmd.yang` - the `called`
      container under `request/l2tp/outgoing-call/remote` declares a `remote`
      leaf then a `called` leaf, both mandatory, both matching a container name
      on the path. Alphabetical ordering inverts them against the declared and
      documented form.
- [ ] `internal/component/cmd/show/yang/ze-cli-show-cmd.yang` - the `sockets`
      container declares optional `protocol` (enumeration tcp/udp), `state`
      (string, no pattern) and `port` (uint32, no range). The pattern-less
      string accepts anything.
- [ ] `internal/component/bgp/plugins/cmd/announce/yang/ze-cli-announce-cmd.yang`
      - the `withdraw` container has a `ze:command`, zero leaves, and the prose
      `withdraw tag <key> <value|*> | withdraw tag * | withdraw id <N> |
      withdraw all`. The `announce` container declares one mandatory `selector`
      leaf while its prose names a family word and a whole route specification.
- [ ] `internal/component/config/yang/modules/ze-extensions.yang` - declares
      `listener`, `syntax`, `flatten`, `key-type`, `route-attributes`, `ordered`,
      `allow-unknown-fields`, `sensitive`, `validate`, `command`, `task-support`,
      `edit-shortcut`, `display-key`, `backend`, `os`, `cumulative`, `required`,
      `suggest`, `hidden`, `decorate`, `filter`, `ephemeral`, `related`,
      `ui-resource`, `ui-permissions`, `ui-csp`, `ensure-exists`, `bcrypt`. No
      `usage` and no `modifier`.
- [ ] `internal/le/docvalid/actions.go` - the action table holds
      `command-contract`, `doc-drift` and `pipe-operators-update`. A new verb is
      one row plus its answer function.
- [ ] `internal/le/docvalid/contract.go` - `Validate` already walks the YANG
      command tree and maps a YANG path to a CLI path with `yangPathToCLIPath`,
      which is the walk the usage gate needs.

**Measured facts** (each derived by a repository-wide count, not by memory):

| Fact | Value |
|------|-------|
| Authored `Usage:` sentences in `.yang` descriptions | 80, and 82 once the gate also reads `Syntax:` and `Filters:` (AC-6, corrected 2026-08-29) |
| Modules carrying at least one | 24 |
| `ze:command` declarations | 384 |
| Distinct `ze:command` identifiers | 377 |
| Command NODES the product tree carries | 377 |

**Corrected 2026-08-29: 384 is the DECLARATION count, and 377 is the node count.
The two are not the same measurement and the difference is not the identifier
collision.** Seven declarations live in test-only plugin modules under
`internal/test/plugins/` (`ze-fakeas112-cmd`, `ze-fakel2tp-cmd`,
`ze-fakeredist-cmd`), which the product binary never registers. The existing
`./le docvalid command-contract` has always reported 377, so a phase's Verify
line must expect 377 command nodes rather than 384.

**The five failure classes:**

| Class | Count | What blocks generation | Verified instance |
|-------|-------|------------------------|-------------------|
| (a) Position | 24 lines | `extractArgs` flattens every leaf onto the leaf-most node as a trailing list, so a generated line appends values at the end | `create interface dummy name unit <name>` instead of `create interface dummy name <name> unit <vid>` |
| (b) Order | every line with two or more leaves | `extractArgDefs` sorts alphabetically, so declared order is lost | `request l2tp outgoing-call remote <name> called <number>` inverts to `called ... remote ...` |
| (c) Missing | 18 exclusive, 31 commands with zero leaves | the value is declared nowhere in the model | the `unit` container declares no leaf for `<vid>` |
| (d) Modifier groups | 13 lines carrying 25 groups | an optional, sometimes repeatable, keyword-introduced trailing group has no modelled form. None of the 13 has its modifier keyword as a child container | `announce ... [tag <key> <value>] [for <duration>]`; `resolve ping <target> [source <ip>] [count <n>] [size <bytes>]`; `show metrics name <name> [label=value ...]`; `[withdraw]` as a presence-only flag; `show bgp peer <selector> rib [scope\|filters\|terminal]` |
| (e) Whole-form alternation | 1 command | four mutually exclusive tail grammars behind one `ze:command` and zero leaves | `withdraw tag <key> <value\|*> \| withdraw tag * \| withdraw id <N> \| withdraw all` |

**Behavior to preserve:**
- `ze help command` listing lines. `helpfmt.Summary` already truncates at the
  first sentence, so removing the `Usage:` sentence must not change a listing.
- The published `args` list on `commandEntry`.
  `internal/le/docvalid/command_surfaces.go` reads it in
  `duplicatePublishedCommandIdentity` to check published command identity, so it
  stays.
- Tab completion at every existing path. The completer reads `Children` and
  `ArgDefs` as sets.
- Which value a positional token binds to on every existing command. The two
  named regressions are `show system sockets 8080` binding to `port` and
  `show system sockets ESTABLISHED` binding to `state`.

**Behavior to change:**
- Argument definition ordering becomes declared rather than alphabetical.
- Positional binding stops depending on slice order.
- `ze help <command>` prints a generated usage line, including for a command
  node that has children, where today it prints nothing.
- `ze help command --json` publishes `usage` and `grammar`.
- Every `Usage:` sentence leaves the YANG descriptions.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- A `-cmd` YANG module on disk, parsed by goyang into an entry tree.
  Format at entry: YANG text, containers carrying `ze:command`, leaves carrying
  a type and an optional `mandatory true`.

### Transformation Path
1. goyang parses the module. Each entry keeps its parsed statement in its `Node`
   field, and `nextStatement` appends substatements in lexer order, so
   declaration order survives the parse and nothing sorts it.
2. `mergeYANGEntry` in `internal/component/config/yang/command.go` builds the
   command tree and calls `extractArgDefs` for every node with a wire method.
3. A usage renderer in `internal/component/command` walks the path from the root
   to the command node and produces an ordered token list, then renders that
   list to a string.
4. Three sinks read the result: `writeHelp` in
   `internal/component/command/help.go` for operator help, `renderHelpCommand`
   in `cmd/ze/help_command.go` for the published catalog, and the new
   `./le docvalid usage-contract` verb for the gate.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| YANG text → command tree | goyang parse, `mergeYANGEntry` | No |
| Command tree → operator help | `writeHelp` writes to stderr | No |
| Command tree → published catalog | `commandEntry` JSON, kebab-case keys | No |
| Command tree → repository gate | `./le docvalid usage-contract` compares the generated string against the authored sentence and against HEAD | No |
| Command tree → positional binding | `positionalDef` in `internal/component/plugin/server/command.go` | No |

### Integration Points
- `internal/component/command` gains the renderer. It is the package both the
  CLI and `cmd/ze` already read the tree from, so no new dependency direction
  appears.
- `internal/le/docvalid` gains one action row and one answer function, reusing
  the YANG walk `Validate` already performs.
- `internal/component/config/yang/modules/ze-extensions.yang` gains `modifier`
  and `usage`.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | No | |
| No unintended coupling (components stay isolated) | No | |
| No duplicated functionality (extends existing, does not recreate) | No | |
| Zero-copy preserved where applicable (refs, not copies) | No | |
| Registration over hardcoding: new commands, views, families, and handlers register, and the core discovers them. No per-feature field, switch case, or factory is added to a core/shared package (`ai/rules/plugins.md`) | No | |

## Rendering Rules

The renderer is defined by this table. Every acceptance criterion about output
reduces to one of these rows, and a case not covered here is a spec gap rather
than an implementer's choice.

| Model shape | Rendered as |
|-------------|-------------|
| A container on the path from the root to the command node | its own name, as a literal keyword, in path order |
| A leaf whose name equals a container name on the path | `<leaf-name>` placed immediately after that container's keyword |
| A leaf a container on the path DECLARES, where that container runs no command | `<leaf-name>` placed immediately after that container's keyword, for every command beneath it. `request interface <name> up` and `request peer <selector> flush` are that shape, and the leaf sits on `interface` and on `peer` |
| A command under such a container that carries `ze:inherit "none"` | no inherited value. It acts on no single member of the set the container names: `show bgp peer list` reads every peer, `request interface migrate` names two interfaces of its own |
| A mandatory leaf whose name matches no container on the path | `<leaf-name>`, appended after the last path keyword, in declaration order |
| An optional leaf whose name matches no container on the path | `[leaf-name <leaf-name>]`, appended after every mandatory token, in declaration order |
| A leaf of type enumeration | its value set joined by a vertical bar replaces `<leaf-name>`, giving `<tcp\|udp>` |
| A leaf of type union | the union's member forms joined by a vertical bar |
| A child container carrying `ze:modifier "once"` | `[keyword <its leaves>]`, appended after everything above |
| A child container carrying `ze:modifier "once"` and NO leaf | `[keyword]`, a presence-only flag |
| A child container carrying `ze:modifier "repeat"` | `[keyword <its leaves> ...]` |
| A child container carrying `ze:modifier "required"` | `keyword <its leaves>`, with no brackets, before every group the command runs without |
| A child container carrying `ze:modifier "choice"` | `[word\|word\|word]`, the closed set its one leaf declares. The container's own name is never typed |
| A leaf of type enumeration, anywhere above | its values in the order the MODULE declares them, which is the order of the integers YANG assigns |
| A container with no `ze:command` | no usage line. It is a grouping node |

The last five rows were added on 2026-08-29, after the Known Limitations entry
below recorded that four argument shapes had no rule. All four are now stated.
The fourth was a repeating token carrying its own internal syntax
(`[label=value ...]`), and it needed no new rendering rule: the owner replaced
the packed spelling with two tokens, so `ze:modifier "repeat"` renders it.

Two consequences are deliberate. An optional leaf always gains its keyword, so
`show system sockets` renders `[protocol <tcp|udp>] [state <state>] [port
<port>]` where the prose wrote `[tcp|udp] [state <STATE>] [port <N>]`. The
generated form is the one `ai/rules/cli.md` requires, so the prose is what
changes. And a command node that also has children renders its own usage and
then its children are listed, which is new output where today there is none.

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | goyang preserves declaration order in each entry's parsed statement and nothing sorts it | the vendored goyang `Entry` keeps its statement in the `Node` field, and `nextStatement` in the vendored parser appends substatements in lexer order | declaration order is unrecoverable without a new extension, and phase 4 needs a redesign | `TestArgDefsFollowDeclarationOrder` over a fixture module declaring leaves in non-alphabetical order | confirmed 2026-08-29. `declaredLeafNames` (`internal/component/config/yang/command.go`) reads `entry.Node.Statement().SubStatements()` and a fixture declaring `remote, called, zone, attempts` extracts in that order; the same fixture extracted `attempts, called, remote, zone` before the change. `TestArgDefsAreDeterministic` repeats the build 12 times for one answer. A leaf arriving from a grouping or an augment is named by no substatement of the container, so those follow in name order |
| A-2 | Every other reader of the argument definitions treats them as a set, so changing their order changes nothing for them | `implicitSelectorDef` and `matchCommandTokens` in `internal/component/plugin/server/command.go`, the completer in `internal/component/command/completer.go`, the grammar checker in `internal/component/command/grammar/checker.go`, `hasImplicitSelectorArg` in `cmd/ze/internal/cmdutil/cmdutil.go` | phase 4 silently changes completion or grammar verdicts | the completer and grammar unit suites green after phase 4, plus a search for index-based access into the definition slice | confirmed 2026-08-29, with one residue. No reader indexes the slice by POSITION: every `ArgDefs[i]` in the tree is a `for i := range` cursor. `implicitSelectorDef` returns nil when a second candidate exists, `hasImplicitSelectorArg` is an any-match, and `matchCommandTokens` looks up by name, so all three are order-independent by construction. The completer, grammar, cli, cli/client, cmdutil, wikicatalog, cidispatch and docvalid suites are green after the change. THE RESIDUE: `positionalError` (`internal/component/plugin/server/command.go`) builds its message from the FIRST enum or union definition, so the wording of a rejection can change for a command whose declared order differs from the alphabetical one. It is message text, not a verdict |
| A-3 | `helpfmt.Summary` already drops the `Usage:` sentence, so listings do not change when the prose goes | `Summary` in `internal/core/helpfmt/helpfmt.go` returns the first sentence | listing output changes for up to 80 commands and every `.ci` asserting a listing goes red | `TestHelpListingUnchangedWithoutUsageProse` comparing listings before and after prose removal on a fixture | confirmed 2026-08-29. The test compares two listings byte for byte and they are equal. It holds at the OPERATOR surface too: `Page.WriteTo` (`internal/core/helpfmt/helpfmt.go`) applies `Summary(e.Desc)` to every section entry. The per-command HEADER is a different matter and the spec does not claim otherwise: `WriteTo` prints `p.Summary` whole, so a command's own page shows the authored sentence today and will lose it in phase 8 |
| A-5 | The announce plugin's withdraw handler can serve three wire methods without changing how it parses its tail | `handleWithdraw` (`internal/component/bgp/plugins/cmd/announce/announce.go`) | phase 7 grows to include the handler's argument parsing | read the handler that answers `ze-bgp:withdraw` before phase 7 starts, and record what it accepts | confirmed 2026-08-29, and the split is cheaper than the spec assumed. `handleWithdraw` is ALREADY a keyword switch over `args[0]` that dispatches to `handlewithdrawTag`, `handleWithdrawID` and `handlewithdrawAll`, each taking `args[1:]`. So the three siblings register three functions that exist, and the hand-rolled switch is DELETED rather than kept beside them (`ai/rules/no-layering.md`). No argument parsing changes. TWO FACTS THE PROSE GETS WRONG, so do not model from it: `handlewithdrawTag` defaults `value` to `"*"` when it is absent, so `withdraw tag <key>` alone is valid and the prose's `<value\|*>` is not mandatory; and `handlewithdrawAll` accepts a `selector <pattern>` filter (`kwSelector`) that is documented NOWHERE, which the model must declare rather than drop |
| A-6 | 18 of the missing values are exclusive to class (c), and 31 commands declare zero leaves | research survey handed to this spec | phase 6 is larger than planned and the phase is re-cut | the gate's own first-run report, which counts them from source | BROKEN 2026-08-29, twice, and the phase is re-cut. Class (c) is 26 commands over 10 modules, not 18 and not the 23 an earlier re-derivation gave. The 23 was itself wrong: it counted seven `create`/`delete interface` rows where the gate shows eight, and it put `resolve ping` and `resolve traceroute` in class (d) when AC-15 already rules that a single optional value is a plain optional leaf. The 26 are the eleven interface commands in `ze-iface-cmd.yang` (`create interface address`, `create interface unit`, `create interface dummy name address`, `create interface dummy name unit`, `create interface bridge name address`, `create interface bridge name unit`, `create interface veth name`, `delete interface name address`, `delete interface name unit`, `request interface mtu`, `request interface mac`), the seven lookups in `ze-resolve-cmd.yang`, `resolve ping`, `resolve traceroute`, and `show config cat`, `show data cat`, `show env get`, `show firewall ruleset`, `show interface type` and `show route lookup`. `delete interface name unit` is the one that carries no prose for its value: its authored line reads `delete interface name <name> unit` while `handleUnitDel` (`internal/component/iface/cmd/manage.go`) requires the VLAN id in `args[0]`, so declaring the leaf is a correction the prose never asked for and it ADDS one difference rather than closing one. The "31 commands with zero leaves" half is not re-derivable: the gate walks the 81 prose-carrying nodes, not the 379, so it never counts a leafless command that never had prose. |
| A-7 | No `.ci` or `.et` fixture asserts the authored `Usage:` text of a command | not yet counted | removing the prose reddens functional tests that were pinning documentation | a repository-wide count of `Usage:` inside `test/`, run in phase 1 | confirmed 2026-08-29. Five hits under `test/`, none over a YANG description. `test/ui/tui-noargs-nontty-fallback.ci` and `test/parse/help-no-color.ci` assert the binary's own top-level banner; `test/parse/cli-generate-wireguard-keypair.ci` asserts a handler-side message written in Go; the last two are Python fixtures under `test/exabgp-compat/`. Every `.et` file lives under `test/`, so the same count covers them. Deleting the 80 sentences reddens no functional test |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The order trap. Changing the definition order to declaration order changes which leaf a bare positional token BINDS to, and the result is a validation verdict rather than an error the operator can read. `show system sockets 8080` would bind to `state`, a pattern-less string that accepts anything, instead of `port`. Corrected 2026-08-29: the binding is a validation binding, not a rewrite, so on this command it changes no answer. It changes an answer where a MANDATORY typed leaf is filled positionally, which is the `show tcp-check <host> <port>` case `positionalDef`'s own comment records | `TestPositionalBindingIsOrderIndependent` fails when it shuffles the definition slice | Phase 3 lands BEFORE phase 4. Binding ranks by constraint strength (enumeration, then bounded uint, then unbounded uint, then patterned string, then union ranked by its most permissive member, then pattern-less string) and breaks a tie by name, so no slice order can change the answer |
| R-2 | The gate's cheapest route from red to green is deleting the authored sentence, which hides the gap instead of closing it | the authored-line count falls while no model change lands in the same commit | The gate compares against HEAD, never against a checked-in baseline. Deleting an authored line whose generated usage differed at HEAD is refused unless the generated usage now equals that line |
| R-4 | Splitting `withdraw` into sibling commands changes the wire methods the announce plugin must serve, and a missed registration makes a documented command unreachable | `./le docvalid command-contract` reports a YANG node with no handler | Phase 7 registers the handler side first and the YANG second, so the contract check is green at every commit |
| R-5 | Declaring the 18 missing leaves gives typed arguments to commands that previously accepted a free-form tail, so a previously accepted invocation is now rejected | the `.ci` suite for the affected command goes red | Each missing leaf is declared with the widest type that still names the value, and the phase adds a `.ci` asserting the previously accepted invocation still works |
| R-6 | A large enumeration renders an unreadable line, since an enum leaf renders its whole value set | a generated line exceeds terminal width in `ze help` | Accepted for this spec and recorded under Known Limitations. Truncation is a rendering change with no model consequence |
| R-7 | New sibling containers under `withdraw` trip the R9 sibling-collision check in the static grammar gate | `./le cli-grammar` reports a collision | Run the static gate in phase 7 before the modelling is called done |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | An operator's command binds a value to the wrong argument and the daemon acts on it without an error. That is the positional-binding half. The usage half is documentation only: a wrong generated line misleads but breaks nothing |
| How is it reverted? | Phases 1 to 5 revert as single commits. From phase 6 onward the YANG modules carry declared leaves that handlers rely on, so a revert must take the YANG and the handler edits together |
| Who else touches this path? | `plan/spec-cli-root-namespace-grammar-deferred-gate-reach.md` works the same static grammar gate. `plan/spec-le-command-namespaces.md` works the `./le` action tables this spec adds a verb to |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `./le docvalid usage-contract` | → | the usage gate answer function in `internal/le/docvalid` | `TestUsageContractVerbRegistered` |
| `ze help create interface dummy name unit` | → | `writeHelp` in `internal/component/command/help.go` calls the renderer | `test-help-usage-generated.ci` |
| `ze help command --json` | → | `renderHelpCommand` in `cmd/ze/help_command.go` populates `usage` and `grammar` | `TestHelpCommandJSONPublishesUsage` |
| `show system sockets port 8080` over the command socket | → | `handleShowSystemSockets` in `internal/component/cmd/show/sockets_linux.go` | `test-show-system-sockets-keyword-filters.ci` |
| A YANG container carrying `ze:modifier` | → | the renderer's modifier branch | `TestUsageRendersModifierGroup` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `ze help command --json` over the whole catalog | Every entry that carries a wire method carries a non-empty `usage` string. No entry with a wire method has an absent or empty `usage` |
| AC-2 | `ze create interface dummy name unit help` | The first line reads `create interface dummy name <name> unit <vid>`, where `vid` is the leaf the `unit` container declares. The value belongs to the keyword of the container that declares it, not to the end of the line |
| AC-3 | `ze request l2tp outgoing-call remote called help` | The first line reads `request l2tp outgoing-call remote <remote> called <called>`. The order follows the declaration and the path, not the alphabet |
| AC-4 | `show system sockets port 8080` and `show system sockets state ESTABLISHED`, run against a Linux daemon | The first filters by port, the second by socket state. A bare `8080` is NOT a filter and never was: `handleShowSystemSockets` (`internal/component/cmd/show/sockets_linux.go`) matches the literal keywords `tcp`, `udp`, `state` and `port` and ignores every other token, so the keyword form is the only filtering form and it is the one `ai/rules/cli.md` requires. The separate order-independence property is about which leaf `validateCommandArgs` BINDS a positional token to, which is `TestPositionalBindingIsOrderIndependent` |
| AC-5 | `ze create interface dummy name help`, a command node that also has children | The generated usage line is printed, followed by the child listing. Today this node prints no description at all |
| AC-6 | Any YANG `description` reached by the command tree | No description prescribes a CLI spelling under any of `Usage:`, `Syntax:` or `Filters:`. `./le docvalid usage-contract` exits non-zero when one does. `Example:` is NOT a marker: `ze-fib-p4-conf.yang` writes `Example: 127.0.0.1:9559` to say what a listener address looks like, which prescribes no CLI spelling |
| AC-7 | A commit that removes an authored `Usage:` sentence whose generated usage differed from it at HEAD, without changing the model | `./le docvalid usage-contract` exits non-zero and names the command, the authored line and the generated line. One difference is exempt, by owner ruling of 2026-08-29: placeholder wording alone. `usageShape` (`internal/le/docvalid/usage.go`) folds every `<...>` group to `<>` and the two lines are compared folded, so `[count <n>]` against `[count <count>]` is a deletion the gate allows and `request interface <name> down` against `request interface down <name>` is one it still refuses |
| AC-8 | A commit that adds a `ze:command` container whose description contains `Usage:` | The gate exits non-zero and names the container |
| AC-10 | `ze announce help` | The line ends with `[tag <key> <value>] [for <duration>]`, rendered from a child container carrying `ze:modifier` for the two-value `tag` group and from an optional leaf for `for`, and `ze help command --json` marks those tokens optional |
| AC-11 | `ze withdraw help`, and `ze help command withdraw` | `withdraw tag`, `withdraw id` and `withdraw all` each appear as their own command with their own usage line, and `withdraw` alone lists them as subcommands |
| AC-12 | `ze help command --json` for any entry | The entry carries an ordered `grammar` token list, and rendering that list produces the entry's `usage` string byte for byte |
| AC-13 | Tab completion after `create interface dummy name eth0 ` | `unit` and `address` are still offered. The modelling changes remove no completion that exists today |
| AC-14 | `ze help command` listing output, before and after the prose is removed | Every listing line is byte-identical. `helpfmt.Summary` already stops at the first sentence |
| AC-15 | `ze resolve ping help` | The line reads `resolve ping <target> [source <source>] [count <count>] [size <size>]`. Each optional value is a modelled optional LEAF, so the renderer's `[leaf-name <leaf-name>]` rule produces it and no extension is needed: a group needs `ze:modifier` only when it carries more than one value or repeats. The prose placeholders `<ip>`, `<n>` and `<bytes>` name a type, and the renderer reads leaf names |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | asks how to add a VLAN unit: `ze help create interface dummy name unit` | YANG → command tree → renderer → `writeHelp` → stderr | `test-help-usage-generated.ci` |
| 2 | builds a valid invocation from the catalog without parsing a string: reads `grammar` from `ze help command --json` | YANG → command tree → renderer → `commandEntry` JSON | `TestHelpCommandJSONPublishesUsage` |
| 3 | filters open sockets by port: `show system sockets port 8080` | CLI → command socket → `validateCommandArgs` → handler | `test-show-system-sockets-keyword-filters.ci` |
| 4 | withdraws one on-demand announcement by id: `withdraw id 3` | CLI → command tree path `withdraw/id` → announce handler | `test-withdraw-forms-are-separate-commands.ci` |
| 5 | discovers a modifier without reading source: `ze help announce` | YANG modifier containers → renderer → stderr | `test-help-usage-modifiers.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestArgDefsFollowDeclarationOrder` | `internal/component/config/yang/command_test.go` | leaves declared `remote` then `called` produce that order, not the alphabetical one | |
| `TestArgDefsAreDeterministic` | `internal/component/config/yang/command_test.go` | repeated builds of the same module produce the same order, since the entry directory is a map | |
| `TestPositionalDefPrefersConstrainedDef` | `internal/component/plugin/server/command_test.go` | an enumeration is offered a token before a pattern-less string, whatever the slice order | |
| `TestPositionalBindingIsOrderIndependent` | `internal/component/plugin/server/command_test.go` | every permutation of the `show system sockets` definitions binds `8080` to `port` and `ESTABLISHED` to `state` | |
| `TestUsagePlacesValueAfterDeclaringKeyword` | `internal/component/command/usage_test.go` | the class (a) rendering rule | |
| `TestUsageRendersOptionalLeafWithKeyword` | `internal/component/command/usage_test.go` | an optional leaf renders `[keyword <value>]`, never a bare optional positional | |
| `TestUsageRendersEnumValueSet` | `internal/component/command/usage_test.go` | an enumeration renders its whole value set | |
| `TestUsageRendersModifierGroup` | `internal/component/command/usage_test.go` | `ze:modifier "once"` and `"repeat"` render a bracketed group, with a trailing ellipsis for the repeat form | |
| `TestUsageGrammarRendersToUsageString` | `internal/component/command/usage_test.go` | the token list and the string cannot disagree | |
| `TestUsageContractVerbRegistered` | `internal/le/docvalid/actions_test.go` | the verb is in the action table and reachable | |
| `TestUsageContractRefusesAuthoredProse` | `internal/le/docvalid/usage_test.go` | a description containing `Usage:` fails the gate | |
| `TestUsageContractRefusesHiddenGap` | `internal/le/docvalid/usage_test.go` | deleting an authored line whose generated usage differed at HEAD fails the gate | |
| `TestHelpCommandJSONPublishesUsage` | `cmd/ze/help_command_test.go` | `usage` and `grammar` appear, and `args` is unchanged | |
| `TestHelpPrintsUsageForNodeWithChildren` | `internal/component/command/help_test.go` | AC-5 | |
| `TestHelpListingUnchangedWithoutUsageProse` | `internal/component/command/help_test.go` | A-3, listings are byte-identical | |
| `TestCLIGrammarGateStatic` | `internal/le/cligrammar/cligrammar_test.go` | existing test, must stay green after every modelling phase | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `show system sockets` port leaf | 0-4294967295 (uint32, no declared range) | 4294967295 | N/A | 4294967296, which binds to no definition and falls through to the pattern-less string |
| `create interface dummy name unit` vid leaf, once declared | 1-4094 | 4094 | 0 | 4095 |
| Authored CLI-grammar sentence count, over all three markers | 0 to the count recorded at HEAD, currently 82 | the HEAD count | N/A | HEAD count plus one is refused |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `test-help-usage-generated.ci` | `test/ui/` | an operator asks for help on a nested command and reads a correct invocation form | |
| `test-help-usage-node-with-children.ci` | `test/ui/` | an operator asks for help on a command that also has subcommands and gets both | |
| `test-help-usage-modifiers.ci` | `test/ui/` | an operator discovers the optional trailing groups of `announce` and `resolve ping` | |
| `test-show-system-sockets-keyword-filters.ci` | `test/ui/` | an operator filters sockets by port and by state, each behind its keyword. Linux only, because the handler is | |
| `test-withdraw-forms-are-separate-commands.ci` | `test/ui/` | an operator withdraws by tag, by id, and all, each as its own command | written 2026-08-29; asserts the help page of each form, not a live withdraw, because a withdraw needs a peer session and the grammar is what the split changed |
| `test-help-command-json-usage.ci` | `test/ui/` | an agent reads `usage` and `grammar` from the catalog | |

### Interop Tests (Scope: protocol)
- N-A. No wire-visible behavior changes and no peer daemon is involved.

## Files to Modify
- `internal/component/config/yang/command.go` - `extractArgDefs` takes declaration
  order from the parsed statement instead of sorting on name, and records for
  each definition whether its name matches a container on the path.
- `internal/component/command/node.go` - the argument definition gains the anchor
  that says which path keyword the value follows; the node gains its modifier
  children.
- `internal/component/command/help.go` - `writeHelp` prints the generated usage
  line for any node with a wire method, including one that has children.
- `internal/component/plugin/server/command.go` - `positionalDef` ranks by
  constraint strength and breaks ties by name, so no slice order changes an answer.
- `cmd/ze/help_command.go` - `commandEntry` gains `usage` and `grammar`;
  `extractArgs` and the `args` field are untouched.
- `internal/component/config/yang/modules/ze-extensions.yang` - declares
  `modifier` and `usage`.
- `internal/le/docvalid/actions.go` - one action row for `usage-contract`.
- The 24 modules carrying authored prose, which are the whole of the class (a)
  to (e) work: `internal/component/bgp/plugins/cmd/announce/yang/ze-cli-announce-cmd.yang`,
  `internal/component/bgp/plugins/cmd/cache/yang/ze-cli-cache-cmd.yang`,
  `internal/component/bgp/plugins/cmd/peer/yang/ze-peer-cmd.yang`,
  `internal/component/bgp/plugins/cmd/policy/yang/ze-policy-cmd.yang`,
  `internal/component/bgp/plugins/filter_irr/yang/ze-filter-irr-cmd.yang`,
  `internal/component/cmd/show/yang/ze-cli-show-cmd.yang`,
  `internal/component/firewall/plugins/irr/yang/ze-firewall-irr-cmd.yang`,
  `internal/component/firewall/yang/ze-firewall-cmd.yang`,
  `internal/component/iface/yang/ze-iface-cmd.yang`,
  `internal/component/iface/yang/ze-iface-interface-cmd.yang`,
  `internal/component/iface/yang/ze-iface-show-cmd.yang`,
  `internal/component/ike/yang/ze-ipsec-cmd.yang`,
  `internal/component/l2tp/cmd/yang/ze-l2tp-cmd.yang`,
  `internal/plugins/as112/yang/ze-as112-cmd.yang`,
  `internal/plugins/config-cli/yang/ze-config-cli-cmd.yang`,
  `internal/plugins/config-storage/yang/ze-storage-cli-cmd.yang`,
  `internal/plugins/env/yang/ze-env-cmd.yang`,
  `internal/plugins/isis/yang/ze-isis-cmd.yang`,
  `internal/plugins/log/yang/ze-log-cmd.yang`,
  `internal/plugins/ospf/yang/ze-ospf-cmd.yang`,
  `internal/plugins/ping-cmd/yang/ze-ping-cmd.yang`,
  `internal/plugins/pki-cmd/yang/ze-pki-cmd.yang`,
  `internal/plugins/resolve-cmd/yang/ze-resolve-cmd.yang`,
  `internal/plugins/traceroute-cmd/yang/ze-traceroute-cmd.yang`.
- `docs/guide/command-reference.md` - the usage column becomes generated output.
- `docs/architecture/api/commands.md` - the catalog contract gains `usage` and
  `grammar`.
- `ai/patterns/cli-command.md` - a command declares its grammar, never describes it.
- `ai/rules/cli.md` - the `Usage:` prose ban gains its feeder, named.
- `ai/INDEX.md` - the native command inventory row for `./le docvalid`.

## Files to Create
- `internal/component/command/usage.go` - the renderer that turns a path and a
  node into an ordered token list and a string.
- `internal/component/command/usage_test.go` - the rendering rule table as tests.
- `internal/le/docvalid/usage.go` - the gate: authored versus generated,
  compared against HEAD.
- `internal/le/docvalid/usage_test.go` - the gate's refusals.
- `test/ui/test-help-usage-generated.ci`
- `test/ui/test-help-usage-node-with-children.ci`
- `test/ui/test-help-usage-modifiers.ci`
- `test/ui/test-show-system-sockets-keyword-filters.ci`
- `test/ui/test-withdraw-forms-are-separate-commands.ci`
- `test/ui/test-help-command-json-usage.ci`
- `plan/deferrals/generated-command-usage.md`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/component/config/yang/modules/ze-extensions.yang` declares `modifier` and `usage`; the 24 `-cmd` modules gain the missing leaves and the modifier containers |
| YANG validation constraints | Yes | Each newly declared leaf takes the narrowest native type available. The unit vid is a bounded uint16, not a string |
| YANG custom validators | N-A | No value needs validation the type system cannot state |
| CLI commands/flags | Yes | `cmd/ze/help_command.go` publishes the new fields. No new flag is added |
| CLI grammar (keyword before value) | Yes | The optional-leaf rendering rule exists to satisfy `ai/rules/cli.md`, and `./le cli-grammar` gates every new sibling container |
| Editor autocomplete | Yes | Automatic. New leaves and containers reach completion through the same tree, which is what AC-13 protects |
| Functional test for new RPC/API | Yes | Six `.ci` files under `test/ui/` |
| Pipe completeness | N-A | `ze help command` already routes through the pipe layer, and no new command is added |
| Env var registration | N-A | No environment leaf is added |
| Doctor check for runtime dependencies | N-A | No new file path, socket, port, binary, or kernel facility |
| Prometheus counters/metrics | N-A | Documentation generation is not observable runtime state |
| BGP family surface (new SAFI / capability / attribute) | N-A | No SAFI, capability, or attribute |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `docs/features.md`, one line: command help states the invocation form, generated from the model |
| 2 | Config syntax changed? | N-A | No config leaf changes. The edited YANG is all `config false` command schema |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md`, and the three `withdraw` forms |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md`, the `usage` and `grammar` fields |
| 5 | Plugin added/changed? | Yes | the announce plugin serves three withdraw wire methods; `docs/guide/plugins.md` |
| 6 | Has a user guide page? | Yes | `docs/guide/command-reference.md` |
| 7 | Wire format changed? | N-A | Nothing on the wire changes |
| 8 | Plugin SDK/protocol changed? | N-A | The plugin protocol is unchanged; only which wire methods announce registers |
| 9 | RFC behavior implemented, changed, or newly proven? | N-A | No RFC obligation is touched |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` gains the six `test/ui/` fixtures |
| 11 | Affects daemon comparison? | No | `docs/comparison.md` makes no claim about help output |
| 12 | Internal architecture changed? | Yes | `docs/architecture/api/commands.md` is the subsystem doc `internal/component/command/help.go` already declares in its `// Design:` header |
| 13 | Route metadata keys added/changed? | N-A | No route metadata |
| 14 | Prometheus counters added/changed? | N-A | None added |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | three new command paths under `withdraw`; `docs/plugin-overview.md` and `docs/features/plugins.md` |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | RUN 2026-08-29, exit 0. `./le spec citation anchors spec plan/spec-generated-command-usage.md` names EIGHT further documents that mention this spec's code and that the spec does not list. Three are affected and three of the eight are not yet judged: `docs/features/introspection.md` and `docs/guide/cli.md` (both mention `cmd/ze/help_command.go`) ARE affected, because the catalog gains `usage` and `grammar`; `docs/architecture/cli/command-completion.md` (mentions `internal/component/command/node.go`) needs a read, because argument definitions are now in declared order. The five that mention `internal/component/plugin/server/command.go` (`docs/architecture/aaa-tacacs.md`, `docs/architecture/api/architecture.md`, `docs/features/cli-commands.md`, `docs/guide/authorization.md`, `docs/guide/tacacs.md`) are UNAFFECTED: they describe authorization and dispatch, and the change is confined to which definition a positional token binds to. TWO FURTHER SURFACES appeared once the code was read and neither was in the spec: `internal/le/wikicatalog/catalog.go` is a SECOND producer of the same catalog and `./le docvalid doc-drift` compares the two, and `internal/le/docvalid/command_surfaces.go` parses the live catalog with `DisallowUnknownFields`. Both had to gain the fields in phase 5. The published artifacts they check, `../wiki/command-catalog.md` and `../gh-pages/data/cli-commands.json`, are now stale and need `./le wiki-catalog update` and `./le site build`. Four declared design documents are named here. `docs/architecture/api/commands.md`, declared by `internal/component/command/help.go`, IS affected: help output gains a generated usage line and the catalog gains two fields. `docs/architecture/api/process-protocol.md`, declared by `internal/component/plugin/server/command.go`, is UNAFFECTED: the plugin transport, message shape and dispatch keys do not change, only which argument definition a positional token binds to inside one server-side helper. `docs/architecture/config/yang-config-design.md`, declared by `internal/component/config/yang/command.go`, is UNAFFECTED: the config tree, its resolution and its validation are untouched, and the change is confined to the `config false` command schema. `docs/architecture/core-design.md`, declared by `internal/le/docvalid/actions.go`, is UNAFFECTED: it describes documentation checks as one command, and this spec adds one verb to that command without changing the shape |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | `docs/guide/command-reference.md` shows invocation forms taken from the prose being deleted. Each one is checked against the generated line |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- the gate exists and fails
   - Tests: `TestUsageContractVerbRegistered`, `TestUsageContractRefusesAuthoredProse`
   - Files: `internal/le/docvalid/actions.go`, `internal/le/docvalid/usage.go`,
     `internal/le/docvalid/usage_test.go`
   - Also in this phase: count `Usage:` inside `test/` to settle A-7, and run
     `./le spec citation anchors` to settle documentation row 16
   - Verify: `./le docvalid usage-contract` runs, reports 82 authored lines and
     377 command nodes, and exits non-zero because the generated side is a stub
2. **Phase: The renderer** -- the rendering rule table becomes code
   - Tests: `TestUsagePlacesValueAfterDeclaringKeyword`,
     `TestUsageRendersOptionalLeafWithKeyword`, `TestUsageRendersEnumValueSet`,
     `TestUsageGrammarRendersToUsageString`
   - Files: `internal/component/command/usage.go`,
     `internal/component/command/node.go`
   - Verify: the gate now prints a generated line beside each authored one, and
     the difference count is the number this spec must drive to zero
3. **Phase: Decouple positional binding from slice order** -- lands BEFORE any
   ordering change, because R-1 is silent
   - Tests: `TestPositionalDefPrefersConstrainedDef`,
     `TestPositionalBindingIsOrderIndependent`,
     `test-show-system-sockets-keyword-filters.ci`
   - Files: `internal/component/plugin/server/command.go`
   - Verify: shuffling the definition slice changes no binding on any command
4. **Phase: Declaration order and position** -- classes (a) and (b)
   - Tests: `TestArgDefsFollowDeclarationOrder`, `TestArgDefsAreDeterministic`
   - Files: `internal/component/config/yang/command.go`
   - Verify: `request l2tp outgoing-call remote called` renders in declared order,
     and 24 position-class differences close
5. **Phase: The operator and agent surfaces** -- the thing this spec is for
   - Tests: `TestHelpPrintsUsageForNodeWithChildren`,
     `TestHelpListingUnchangedWithoutUsageProse`,
     `TestHelpCommandJSONPublishesUsage`, `test-help-usage-generated.ci`,
     `test-help-usage-node-with-children.ci`, `test-help-command-json-usage.ci`
   - Files: `internal/component/command/help.go`, `cmd/ze/help_command.go`
   - Verify: an operator reads the invocation form from `ze help`, and an agent
     reads `grammar` from the catalog. Every later phase now improves a live surface
0. **Phase: Every declared verb dispatches** -- added 2026-08-29, and it blocks
   phase 7
   - Tests: `TestEveryDeclaredVerbDispatches`,
     `TestUndeclaredVerbFallsThroughToUsage`
   - Files: `cmd/ze/ze_core_dispatch.go`,
     `cmd/ze/ze_core_dispatch_verbs_test.go`
   - Verify: `ze announce help`, `ze withdraw help`, `ze create help`,
     `ze debug help`, `ze system help` and `ze peer help` each exit 0 on a build
     carrying every feature gate
6. **Phase: Declare the missing values** -- class (c)
   - Tests: the `.ci` suite of each affected command, unchanged, plus the
     boundary rows for each new numeric leaf
   - Files: 15 modules and the 23 commands the A-6 row names
   - Verify: 23 more differences close, and no previously accepted invocation is
     rejected
7. **Phase: Modifier groups and whole-form alternation** -- classes (d) and (e).
   The two halves are independent and (e) is the smaller: (e) is `withdraw`
   alone, and (d) is 14 commands and the extension they need
   - Tests: `TestUsageRendersModifierGroup`, `test-help-usage-modifiers.ci`,
     `test-withdraw-forms-are-separate-commands.ci`, `TestCLIGrammarGateStatic`
   - Files: `internal/component/config/yang/modules/ze-extensions.yang`, the 14
     modifier-carrying modules, the announce module and its handler registration
   - Verify: every modifier group renders from the model, `withdraw` becomes
     three commands, `./le docvalid command-contract` and `./le cli-grammar`
     stay green
8. **Phase: Delete the prose and cap the exception** -- the end state
   - Tests: `TestUsageContractRefusesHiddenGap`,
     `TestUsageContractRefusesNewDeclaredTail`
   - Files: the 24 modules lose their last `Usage:` sentences; `ai/rules/cli.md`
     and `ai/patterns/cli-command.md` name the feeder
   - Verify: every authored sentence the model can express is gone, the ones it
     cannot are the only ones left, and the gate is a plain assertion that every
     command node renders a usage line

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-N has an implementation at file and symbol, and every one of the five failure classes has closed to zero differences |
| Feature completeness | Each of the five user stories runs end to end against a daemon, not only in a unit fixture |
| Correctness | A generated line is a line an operator can type. Pick ten commands at random, type the generated line, and check the daemon accepts it |
| Correctness | The binding ranking is total: no two definitions tie on constraint strength without the name tiebreak resolving them |
| Naming | JSON keys are kebab-case: `usage`, `grammar`. The extension names are `modifier` and `usage`, and `syntax` is untouched |
| Data flow | The renderer reads the tree and nothing else. It does not read a description, so a description can never influence a generated line |
| Rule: `ai/rules/cli.md` | No generated line places a value before its keyword, and no `--flag` appears in a description or a generated line |
| Rule: `ai/rules/no-layering.md` | `args` is not replaced by `grammar`. If review finds `grammar` makes `args` dead, `args` is deleted in the same commit, not left beside it |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| No authored usage prose remains | a repository-wide count of `Usage:`, `Syntax:` and `Filters:` in `.yang` returns 0 |
| Every command node renders a usage line | `./le docvalid usage-contract` exits 0 |
| The catalog publishes both projections | `ze help command --json` shows `usage` and `grammar` on every entry with a wire method |
| Operator help shows the form | `ze help create interface dummy name unit` prints the AC-2 line |
| Binding is order independent | `TestPositionalBindingIsOrderIndependent` passes over every permutation |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | Declaring the 18 missing leaves tightens what a handler receives. Check that no handler treated an absent leaf as permission to accept a wider value, and that a newly typed leaf does not reject a value the handler still needs |
| Error leakage | A generated usage line is built from YANG names and types only. Confirm no description text, file path, or handler identifier reaches it |
| Resource exhaustion | A modifier marked `repeat` states no bound. Confirm the handler bounds the repetition, since the usage line advertises it as unbounded |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| A generated line disagrees with the authored one and the authored one is right | The model is missing something. That is class (c), (d) or (e), so route to phase 6 or 7. Never edit the renderer to reproduce a prose accident |
| A generated line disagrees with the authored one and the generated one is right | Delete the authored line in the same commit and say why |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

## Design Insights

- Position is a property of the PATH, not of the leaf list. The l2tp case and
  the iface case looked like two problems, an order bug and a position bug, and
  one rule closes both: a leaf whose name equals a container name on the path
  belongs immediately after that container's keyword. Reading them as one rule
  is what keeps the renderer small.
- The prose was never the source of truth for anything. It reached the published
  catalog through a Python step that no longer exists, so nothing has read these
  80 sentences since `eae282592`. That makes deletion cheap and it makes the
  drift unsurprising.
- `writeHelp` suppresses the description of any node that has children, so for a
  command such as `create interface dummy name` the authored usage line has been
  invisible to operators all along. Generation does not only make the line
  correct, it makes it reachable.
- **The renderer writes the leaf's name TWICE for an optional value, so a
  keyword can never differ from the value it introduces.** `UsageLine`
  (`internal/component/command/usage.go`) writes `[` + Text + ` <` + Text + `>]`,
  and Text is the leaf name. So an optional leaf produces `[for <for>]` and
  never `[for <duration>]`. This is what decides when a group needs
  `ze:modifier` and when a leaf will do: a leaf suffices exactly when the
  keyword IS the value's name, which is true for `[count <count>]` and false for
  `[for <duration>]` and `[tag <key>]`. A one-leaf `ze:modifier` group is the
  only form that states a keyword and a differently-named value.
- **A MANDATORY leaf renders with no keyword at all, so an argument whose
  handler requires its keyword must be declared OPTIONAL or it renders a line
  nobody can type.** `show policy test peer ... update <hex>` is that case:
  `parsePolicyTestArgs` (`internal/component/bgp/plugins/cmd/policy/handler.go`)
  reads the literal `update` token, and a mandatory `update` leaf would render a
  bare `<update>`. The model understates by one notch to keep the line typeable.
- **Declaring a leaf can make a WORKING command unknown, and the mechanism is
  not the type.** `implicitSelectorDef`
  (`internal/component/plugin/server/command.go`) answered nil whenever two
  mandatory string leaves were unmatched, and `matchCommandTokens` then failed
  the whole match rather than the argument. So adding the MAC value to
  `request interface mac` would have turned `request interface zetest0 mac
  02:de:ad:be:ef:01` into an unknown command, silently, with no test naming the
  cause. The fix is in the dispatcher, not in the model: a leaf that states a
  pattern is a typed value and is no longer a candidate for the inline
  identifier slot while a pattern-less leaf is available. Ambiguity is still
  refused, so the change can only resolve a case that used to resolve to
  nothing.

- **A mandatory keyword-introduced value needs no new token kind, because it is
  already two tokens.** `scope <link|area|as>` is a `UsageKeyword` followed by a
  `UsageValue`, exactly as a path container and its anchored leaf are. So
  `ze:modifier "required"` gains a Modifier occurrence and gains no `UsageKind`:
  `appendGroupTokens` (`internal/component/command/usage.go`) flattens the group
  into the keyword and its values, and the round trip is free. Only a group the
  command runs WITHOUT needs a token of its own, because only that group needs a
  bracket to hold it together.
- **A presence-only flag needs neither.** A `ze:modifier "once"` container with
  no leaf already renders `[withdraw]`: `usageGroupToken` builds an empty group
  and `writeUsageToken` writes the bracket pair around the keyword alone. The
  obvious alternative, a leaf of `type empty`, would have been the DEFECT, not
  the model. Phase 1 of `validateCommandArgs`
  (`internal/component/plugin/server/command.go`) demands a value after every
  declared leaf name it meets, so a `withdraw` leaf turns the shipped `debug ip
  ospf inject opaque ... withdraw` into "withdraw requires a value".
- **A modifier group is invisible to dispatch, and that is what makes it the
  safe way to model a tail.** Its leaves live on the CHILD node, so the command
  node's `ArgDefs` stay empty and every token still passes through to the
  handler. The same tail declared as LEAVES would break both ospf inject
  commands outright: with one optional `type` leaf unmatched,
  `unmatchedDefCount` is above zero, so the first token no definition accepts
  reaches `positionalError` instead of the handler.
  `TestModifierGroupsLeaveDispatchUntouched` is that proof.
- **Only the third rule needed a new kind, and adding it cost two surfaces
  beyond the renderer.** A `ze:modifier "choice"` container's own name is never
  typed, so completion had to offer the WORDS instead of the container
  (`choiceSuggestions`, `internal/component/command/completer.go`) and the help
  page had to stop listing it as a subcommand (`listedChildNames`,
  `internal/component/command/help.go`). Without the second,
  `show policy chain peer` would have lost its description page: the container
  was its only child, and `writeHelp` prints a description only for a node with
  no children.
- **The static grammar gate said the new shape was a violation, and the gate was
  reading `len(node.Children)`.** R6 asks whether an ACTION keyword must move in
  front of an identifier, and a modifier group states no action. `CheckNode`
  (`internal/component/command/grammar/checker.go`) now counts subcommands
  through `hasSubcommand`, so `show bgp peer <selector> rib [sent|...]` and
  `show policy chain peer <selector> [import|export]` pass while a real
  subcommand beside a group still fails.
- **The declared-tail extension is removed, and the criterion that guarded it
  with it (owner decision, 2026-08-30).** It was reserved for one situation, the
  announce route specification's trailing token stream, and two things killed it.
  AC-9 ratcheted its count monotonically non-increasing from a count of 0, so the
  criterion refused the first use of the mechanism it existed to permit. And the
  four rendering rules the owner authorised reach every argument shape met here
  except two `create interface` commands, whose values want a leaf moved rather
  than an annotation. A mechanism with no user, and a ratchet defending it, is
  machinery rather than a rule. Nothing is deleted from the tree, because the
  extension was never declared: zero occurrences in every `.yang` and `.go` file.
- **A command the model cannot express keeps its sentence, and that is the
  answer rather than a gap.** It stays a reported difference, and the way to
  close it is to make the model able to express it, not to annotate around it.
  The live instances are `create interface address` and `create interface unit`,
  whose `name` would move onto a container whose `dummy`, `bridge` and `veth`
  subtrees each declare a `name` of their own.

- **Enum values render in declaration order, and that was a precondition rather
  than a nicety.** `enumNames` (`internal/component/config/yang/command.go`)
  sorted on the NAME, so the one command whose authored line the model could
  match exactly would have rendered `[export|import]`. The order is now the
  integers YANG assigns, which is declaration order for every enum here. Two
  published lines changed with it, both toward the module's own order:
  `request log level <logger> <disabled|debug|info|warn|err>` and
  `[source-asn4 <true|false>]`.

- **Declaring PART of a command's tail is worse than declaring none of it, and
  the leaf to miss is the one whose values are typed as bare words.**
  `5f5b73261` gave `show policy test peer` a `selector`, `filter`, `update` and
  `source-asn4` and no direction, while the same diff gave the sibling `show
  policy chain peer` a `ze:modifier "choice"` container for its. Before that
  commit the node declared no leaf, so `validateCommandArgs`
  (`internal/component/plugin/server/command.go`) was skipped and every token
  passed through. After it, the bare `export` had to be PLACED. With `filter
  <name>` present its keyword consumes the filter leaf, so only the source-asn4
  enum was left unmatched and the documented form was refused with `invalid
  value "export", expected one of: true, false` before `parsePolicyTestArgs`
  (`internal/component/bgp/plugins/cmd/policy/handler.go`) ran. Without `filter
  <name>` the same token was bound to `filter` as a filter NAME, which is
  harmless only because the handler re-parses the raw tokens: three functional
  tests kept passing on that wrong binding.
- **The handler decides whether a leaf is mandatory, and here it says yes.**
  `parsePolicyTestArgs` answers `errMissingDirection` when no direction token
  arrives, so `leaf direction` is `mandatory true` rather than optional: an
  optional leaf would model a command the handler cannot run. Nothing is lost
  to the "a mandatory leaf renders no keyword" hazard above, because an ENUM
  renders its closed set rather than the leaf name, and the generated line is
  typeable as it stands: `show policy test peer <selector> <import|export>
  [filter <filter>] [update <update>] [source-asn4 <true|false>]`. What moves
  is which layer refuses a direction-less call, from the handler to the
  dispatcher, and the message with it.
- **`positionalDef` needed no change, and its ranking is what makes a declared
  leaf win the token.** It answered correctly for the definitions it was given
  in both states: nil for `export` when nothing accepted it, and the
  pattern-less `filter` when that was the only definition left. With the enum
  declared, `direction` sits in the mandatory tier, which is offered the token
  first, and `ConstraintEnum` outranks the `ConstraintAny` of a pattern-less
  string, so the binding is stable by tier and by rank
  (`internal/component/command/argvalidate.go`).
- **A dispatcher verdict is read off the DAEMON's log, never off a plugin
  fixture's own assertion.** `test/plugin/policy-test-errors.ci` now carries
  `reject=stderr:pattern=invalid value` and `expect=stderr:contains=required
  argument missing`, and each was proven to discriminate: deleting the leaf
  reddens the first, dropping `mandatory true` reddens the second. The fixture's
  Go assertions were strengthened in the same pass and are NOT the pin, because
  a fixture the daemon runs as a plugin cannot fail its own test
  (`plan/journal/green-that-could-not-have-been-red.md`, 2026-08-29).

- **Deleting a placeholder-only sentence loses information for two of the seven,
  and the remedy is renaming the LEAF, not keeping the prose.** Owner ruling,
  2026-08-29: the generated form of the seven placeholder-only commands is
  acceptable and their prose is deletable. For five of them the model's leaf name
  says as much as the type word it replaces. Two are worse off:
  `create interface unit` wrote `<parent>` where the leaf is called `name`, and
  `show firewall irr prefix` wrote `<asn-or-as-set>` where the leaf is also called
  `name`. `UsageLine` (`internal/component/command/usage.go`) writes the leaf's
  name, so the generated lines now read `create interface unit <name> <vid>` and
  `show firewall irr prefix <name>`, and `<name>` says nothing about what the
  operator types. The model already carries the fix: rename the leaf to `parent`
  and to `as-set`, and the generated line states what the prose stated. That is a
  dispatch-visible change to two handlers, so it is a later pass and NOT part of
  this one. Nothing was renamed here.
- **The AC-7 exemption is mechanical, and its narrowness is what makes it safe.**
  `usageShape` (`internal/le/docvalid/usage.go`) folds every `<...>` group to
  `<>`, and `usageContract` compares the folded lines rather than the raw ones.
  The fold is over the WHOLE bracket group, because `request log level` renders
  the enumeration `<disabled|debug|info|warn|err>` where the prose wrote
  `<level>`; a fold over the text inside the brackets would keep that one refused.
  The fifteen value-position commands are untouched by it: `request interface
  <name> down` and `request interface down <name>` fold to different shapes, so
  the gate still refuses that deletion. Both halves were proven rather than
  asserted -- widening the fold to ignore token order reddens
  `TestUsageContractRefusesDeletingAValuePositionLine`, and deleting the real
  `request interface down` sentence made the gate report 1 hidden difference.

### Corrections to this spec, 2026-08-29

Each row says what the spec claimed, what the code says, and why the change.

| What changed | Why |
|--------------|-----|
| **A verb dispatch gate was added ahead of every phase.** `isYANGVerb` (`cmd/ze/ze_core_dispatch.go`) gated on a hardcoded eight-entry map while the command tree declared sixteen top-level verbs. `announce`, `withdraw`, `create`, `debug`, `system` and `peer` reached no dispatcher, so four of this spec's acceptance criteria named invocations the binary could not run. The set is now derived from the tree, the local registry and the root registry | Splitting `withdraw` into three siblings without this produces three unreachable commands instead of one. `ai/rules/plugins.md` refuses a per-feature list in a core package where registration can discover it |
| AC-2, AC-3, AC-5, AC-10, AC-11, AC-15 spelled the invocation `ze help <path>` | No such form exists. `extractHelpPath` (`cmd/ze/ze_core_dispatch.go`) takes the help word from the END of the argument list, so the real form is `ze <path> help` |
| AC-3 and AC-15 quoted the PROSE placeholders `<name>`, `<number>`, `<ip>`, `<n>`, `<bytes>` | The renderer reads leaf NAMES and never a description (`internal/component/command/usage.go`, `usageToken`), so it cannot produce a word the model does not carry. The l2tp leaves are `remote` and `called`, and the ping leaves will be `target`, `source`, `count` and `size` |
| AC-4 asserted that a bare `show system sockets 8080` filters by port | It does not, and never did. `handleShowSystemSockets` (`internal/component/cmd/show/sockets_linux.go`) matches the literal keywords `tcp`, `udp`, `state` and `port`, and `validateCommandArgs` (`internal/component/plugin/server/command.go`) passes an unconsumed positional through to the handler unchanged rather than rewriting it into a keyword pair. So the binding decides validation, not filtering. AC-4 now asserts the keyword form, which is what `ai/rules/cli.md` requires anyway, and names `TestPositionalBindingIsOrderIndependent` for the binding half |
| AC-6 keyed the gate on `Usage:` alone | The word in front of a grammar is the cheapest thing to change, so one keyword is not a gate. `show system sockets` writes `Filters: [tcp\|udp] [state <STATE>] [port <N>]` and `show capture` writes another, both prescribing a CLI spelling. `usageMarkers` is now `Usage:`, `Syntax:`, `Filters:`, and the authored count rose from 80 to 82 with the differences from 51 to 53. `Example:` is deliberately not a marker: `ze-fib-p4-conf.yang` uses it for a value, not a grammar. The four `Syntax:`/`Filters:` prescriptions in `ze-rib-api.yang` are outside the gate's population, because `BuildCommandTree` reads `-cmd` modules only and those live on `rpc` statements in an `-api` module. That is a real hole and it is NOT this spec's: widening the population from the command tree to every YANG module is a different change |
| A-6 sized class (c) at 18, then at 23 | 26, over 10 modules, derived by declaring the leaf for each one and reading what the gate then reported. Both earlier numbers were counted by eye off the difference list. See the A-6 row |
| The announce prose was going to be the model for `withdraw` | It is wrong twice. `handlewithdrawTag` defaults the value to `"*"`, so `withdraw tag <key>` alone is valid, and `handlewithdrawAll` accepts an undocumented `selector <pattern>` filter. See A-5 |
| `resolve ping`'s optional values were to be modelled as `ze:modifier` containers | An optional LEAF already renders `[leaf-name <leaf-name>]`, which is the whole of what those three need. `ze:modifier` earns its place only where a group carries more than one value (`[tag <key> <value>]`) or repeats (`[label=value ...]`), which is `ai/rules/simplicity.md` cutting machinery the model does not need |
| The `withdraw id` leaf was going to be `uint64` | R8 refuses a numeric identifier (`internal/component/command/grammar/checker.go`), and the type also decides the error an operator reads. A leaf whose name equals its container is lifted out of args by `matchCommandTokens`, which REFUSES THE WHOLE MATCH when the value fails its type. So `withdraw id abc` would have answered "unknown command" instead of "invalid id". It is `type string { length "1..20"; }`, and `withdrawByID`'s `ParseUint` is the second half of the pair (`docs/contributing/ze-go-style.md`) |
| Closing the inline-value residue was going to mean renaming the leaf to match its container | The owner ruled otherwise on 2026-08-29: the leaf MOVES to the container instead, because the model should state the structure rather than annotate it. Renaming `selector` would also break `applyExtractedSelectors` (`internal/component/plugin/server/command.go`), which bridges that one leaf name onto `CommandContext.Peer`. Moving needs two things the spec did not plan: extraction carries a non-command container's leaves down to every command beneath it, and `ArgDef.Anchor` says which keyword the value follows. It also needs ONE new statement the spec did not plan, `ze:inherit "none"`, because two commands under such a container act on no single member of the set: `show bgp peer list` reads every peer and `request interface migrate` names two interfaces of its own. Without it the container's mandatory leaf becomes theirs, and Phase 3 of `validateCommandArgs` refuses the bare `show bgp peer list` that every `.ci` under `test/ui` runs |
| `request peer teardown` and `show bgp peer rib` could be closed by deleting their prose | They cannot, in one change. Their authored line was wrong about a second thing: `[cease-subcode]` when `handleTeardown` requires it, and `[scope\|filters\|terminal]` when the model declares `[sent\|advertised\|received\|sent-received]`. `usageShape` folds placeholder WORDING and nothing else, so the deletion is refused against the HEAD line either way. Both descriptions now state the true grammar, which takes the difference count down; the deletion lands in the commit after this one, when HEAD carries the corrected line. That two-step is what R-2 designed |
| The `withdraw` split was going to need a new verb | It needs the exemption that already existed. `bridgeSurface` (`internal/component/command/grammar/checker.go`) is the documented E1 set of line-protocol verbs that are deliberately not verb-first, and `ze-bgp:withdraw` was in it. The three successors replace that one entry; the retired method carries no exemption, which `TestExemptCategory` now asserts. Adding `withdraw` to `command.Verbs` would have been a vocabulary decision the file says is deliberate, taken to route around an exemption that already fits |

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| The catalog publishes BOTH `usage` and `grammar`, from one producer | Publish only the rendered `usage` string; publish only `grammar` and let readers render | A rendered string forces every machine reader to parse angle brackets and square brackets, and `args` cannot help because it carries no position: the flat unordered list is exactly the shape that produced the position bug. Publishing only `grammar` breaks the wiki and the operator-facing surfaces that need a string. Both, derived from one token list, cannot disagree, and AC-12 pins that |
| Positional binding ranks by CONSTRAINT STRENGTH, not by any order | Keep slice order and freeze the alphabetical sort; sort definitions for binding and separately for display | Freezing the sort makes display order a hostage to a binding accident. Two sorts on one slice is two truths about one list. Ranking by how much a definition constrains its value removes order from the question entirely, which is the only version where changing display order is provably safe |
| The gate compares against git HEAD, not a checked-in baseline file | A baseline file listing the known differences | A checked-in baseline can be edited to lie, and the cheapest route from red to green becomes editing it. Comparing against HEAD is the idiom the RFC ratchets already use in this repository, and it cannot be edited without also editing history |
| `args` stays on `commandEntry` beside `grammar` | Delete `args` and let readers derive it from `grammar` | `internal/le/docvalid/command_surfaces.go` reads `args` to check published command identity. The two answer different questions: `args` is the type dictionary, `grammar` is the positioned form. If review finds `grammar` makes `args` dead, `ai/rules/no-layering.md` applies and `args` is deleted rather than left beside it |

## Known Limitations

- **Command identity collides, and this spec does not fix it.** 384 `ze:command`
  declarations resolve to 377 distinct identifiers. `ze-iface:interface-unit-add`
  and `ze-iface:interface-addr-add` are each declared three times,
  `ze-clear:interface-counters`, `ze-show:host-all` and `ze-bfd-api:show-profile`
  twice each, for behaviors that differ. A generated usage line is per PATH, so
  it is correct despite the collision, but the published catalog keys on the
  identifier and therefore cannot distinguish the three unit-add commands. This
  needs its own spec. It is recorded here so it is not lost, and it must not be
  fixed on the way to closing this one.
- A large enumeration renders its whole value set, so a leaf with twenty
  enumerated values produces a long line. Truncation is a rendering change with
  no model consequence and is deliberately out of scope.
- Handler-side usage strings, such as the `show traffic usage` error text, are
  still hand-written. The renderer is the obvious source for them, and wiring it
  there is separable future work.
- `ze:modifier "repeat"` states that a group repeats. It states no upper bound,
  because no command in the current population needs one.

- **The gate's population is the command tree, so four prescriptions sit outside
  it.** `BuildCommandTree` (`internal/component/config/yang/command.go`) reads
  `-cmd` modules only. The four `Syntax:` and `Filters:` sentences in
  `internal/component/bgp/plugins/rib/yang/ze-rib-api.yang` sit on `rpc`
  statements in an `-api` module, so `./le docvalid usage-contract` never sees
  them and the authored count of 81 excludes them. Widening the population from
  the command tree to every YANG module is a different change and is not this
  spec's.
- **RESOLVED 2026-08-29 for nineteen commands by the owner's ruling, and two
  `create interface` commands are left.** The residue was a generated line that
  put the value after the LAST keyword where the operator types it INLINE, which
  `implicitSelectorDef` (`internal/component/plugin/server/command.go`) fills.
  The owner ruled the authored spelling correct and chose to MOVE THE LEAF: the
  value is declared once, on the container whose keyword it follows, and
  `inheritArgDefs` (`internal/component/config/yang/command.go`) carries it down
  to every command beneath. `request interface up/down/mtu/mac`, the nine
  commands under `request peer`, the five under `show bgp peer` and
  `update bgp peer prefix` all render their inline value now. No ArgDef changed
  anywhere in the tree, so no token binds to a different leaf.
  `create interface address` and `create interface unit` carry the same defect
  and were left out of that pass: their `name` leaf would have to move onto
  `create/interface`, whose `dummy`, `bridge` and `veth` subtrees declare a
  `name` of their own, and that is a wider change than the container this one
  touched.
- **Three of the four missing shapes now have a rule; the fourth is refused by
  `ai/rules/cli.md` and the six commands close one difference between them.**
  RESOLVED 2026-08-29 for shapes 1 to 3, by owner decision. The mandatory
  keyword-introduced value is `ze:modifier "required"`, the valueless optional
  keyword is a `ze:modifier "once"` container with no leaf, and the bare optional
  alternation is `ze:modifier "choice"`. All five affected commands now render
  their tail from the model, and `show policy chain peer` matches its authored
  line byte for byte, taking the difference count from 33 to 32. What each of the
  other four still owes is below, and none of it is a rendering rule.
- **`show metrics name` takes two tokens for a label filter, and its prose is
  gone.** RESOLVED 2026-08-29 by owner ruling. The packed `label=value` form was
  unpublishable: `ai/rules/cli.md` reads "Free-form values MUST NOT appear in an
  untyped positional slot", and the keyword half of `label=value` came from an
  open set rather than one known at compile time. The command now states
  `show metrics name <name> [label <key> <value> ...]`, modelled as a
  `ze:modifier "repeat"` container in `ze-cli-show-cmd.yang`, which is the shape
  `announce` already renders for `[tag <key> <value>]`. `metricLabelFilters`
  (`internal/component/cmd/show/show.go`) parses the three tokens and refuses a
  group that stops short of its value. That refusal closed a defect the packing
  hid: the old loop skipped any token it could not split on `=`, so
  `show metrics name ze_bgp_up peer` answered every series and reported nothing.
  The web Metrics Query tool was the one caller of the retired spelling;
  `handleMetricsSubmit` (`internal/component/web/page_tools.go`) now splits its
  one form field into the two tokens the command takes.
- **`show pki certificate name` renders `[pem]` and owes the other two arms of
  an alternation the rules do not cover.** `handleShowPKICertificate`
  (`internal/component/pki/show.go`) switches over three mutually exclusive
  tails: `pem`, `bundle pem`, and `fingerprint [sha256|sha384|sha512]`. That is
  whole-form alternation, class (e), and this spec's Key Design Decisions already
  answer it: sibling containers, one per form, each with its own `ze:command`,
  which is how `withdraw` was split. It needs three handler registrations and is
  not a rendering rule.
- **`show bgp peer rib`'s authored `[scope|filters|terminal]` names three token
  CLASSES, not three keywords.** `parsePipelineArgs`
  (`internal/component/bgp/plugins/rib/rib_pipeline.go`) reads a scope keyword
  from `scopeKeywords` (`advertised`, `sent`, `received`, `sent-received`), then
  filter keywords each taking a value, then at most one terminal. The scope half
  is now declared and renders `[sent|advertised|received|sent-received]`. The
  filter and terminal halves are the pipeline grammar `show bgp rib` shares, whose
  own prose lives on an `rpc` statement in `ze-rib-api.yang` and therefore sits
  outside this gate's population. Modeling it is that command's work, not this
  one's.
- **The two ospf inject commands render their whole grammar and still differ
  from their prose, in three places where the PROSE is what is wrong.** The v4
  line writes `[type <128-255>]`, a type rather than a leaf name, and the
  renderer reads leaf names, so the generated form is `[type <type>]`. It writes
  `[hex <body> | tlv <type> <value-hex> ...]` as one alternation, while
  `parseOpaqueInject` (`internal/plugins/ospf/inject.go`) APPENDS both and
  accepts them mixed, so the model states `[hex <body> ...] [tlv <type>
  <value-hex> ...]`. The v6 line states `scope` as mandatory, while
  `parseV3Inject` (`internal/plugins/ospf/inject_v3.go`) requires only `type` and
  `id` and treats `scope` as a cross-check, so the model states it optional.
  Correcting those three sentences is what closes the two differences, and this
  phase left every authored sentence untouched.
- **The prose deletion stopped at 72 of the 81 sentences, and 9 stand.** The
  count below records the pass that took it to 24; 3b376b4b7 took it to 11 and
  the review commit took it to 9. Each of the 9 is named in Closure status.
- **The prose deletion stopped at 57 of the 81 sentences, and 24 stand.**
  `./le docvalid usage-contract` reported 379 command nodes, 81 authored
  sentences and 32 differences before the deletion phase, then 379, 32 and 32,
  and 379, 25 and 25 after the 7 placeholder-only sentences went. The 57th is
  `show metrics name`, whose grammar changed rather than whose prose was
  reproduced, taking the counts to 379, 24 and 24. The 56 before it are every
  sentence the model reproduces byte for byte and the 7 it reproduces up to the
  word inside the angle brackets. The 24 that stand each differ in token
  structure.
  A grammar change makes the gate's own deletion guard fire in the WORKING TREE
  and only there. `headUsage` (`internal/le/docvalid/usage.go`) reads the
  authored sentence out of git HEAD, so while the change is uncommitted the gate
  compares the new generated line against the retired `[label=value ...]` and
  reports one hidden deletion. The commit carrying both the model and the
  deletion moves HEAD, the path leaves the head map, and the count returns to
  zero. `./le verify worktree` runs against a commit, so it never sees the row.
- **The 7 placeholder-only sentences are gone, and AC-7 gave way.** RESOLVED
  2026-08-29 by owner ruling: the generated form is acceptable and the prose is
  deletable. Their generated line states the same tokens in the same order and
  names the LEAF where the prose named a type: `resolve ping <target> [source
  <source>] [count <count>] [size <size>]` against `[source <ip>] [count <n>]
  [size <bytes>]`, and the same for `create interface unit`, `request as112
  healthcheck`, `request l2tp outgoing-call remote called`, `request log level`,
  `show announcements` and `show firewall irr prefix`. AC-7 compared whole lines
  and so refused the deletion the Failure Routing table asks for; it now
  compares the lines with every `<...>` folded to `<>`, which exempts this
  difference and no other.
- **Fifteen commands render the value in the wrong position, and the RENDERER is
  what is wrong.** Owner ruling, 2026-08-29: the authored spelling is correct
  and the generated spelling is not. `request interface <name> down` is right
  and `request interface down <name>` is wrong; `request peer <selector> flush`
  is right and `request peer flush <selector>` is wrong. The prose therefore
  MUST NOT be deleted until the renderer produces the authored form. Thirteen
  differ in this and nothing else: `request interface up`, `request interface
  down`, `request interface mtu`, `request interface mac`, `request peer flush`,
  `request peer pause`, `request peer resume`, `request peer plugin session
  ready`, `show bgp peer capabilities`, `show bgp peer detail`, `show bgp peer
  history`, `show bgp peer statistics` and `update bgp peer prefix`. Two more
  carry it beside a second difference: `request peer teardown` and `show bgp
  peer rib`.
  The mechanism: `request interface down` declares its `name` leaf on the `down`
  container (`internal/component/iface/yang/ze-iface-cmd.yang`, lines 181 to
  187). The Rendering Rules table anchors a leaf only when its NAME equals a
  container on the path, `name` equals no container in `request interface down`,
  so the leaf falls through to "appended after the last path keyword" and lands
  at the end of the line. The authored form places it after `interface`, which
  means the value belongs to the container ONE ABOVE the one that declares it.
  Whether the model moves the leaf or the renderer learns a second anchor is a
  separate decision and no part of this phase.
- **The other ten standing differences each have their own reason above.**
  `announce`, `debug ip ospf inject opaque`, `debug ipv6 ospf inject lsa`,
  `delete interface name unit`, `resolve traceroute`, `show capture`, `show pki
  certificate name`, `show policy test peer` and `show system sockets`. `show
  metrics name` was the tenth and its entry above records how it closed.
- **A command whose grammar the model cannot state keeps its authored
  sentence.** There is no annotation route: the declared-tail extension and its
  ratchet were removed on 2026-08-30 rather than given a floor, because nothing
  used them. `announce` therefore publishes a generated line naming its peer
  selector and its modifier groups, and its route specification stays in prose
  until the model can state it.

## RFC Documentation (Scope: protocol)

N-A. This spec implements no RFC obligation and changes no wire-visible behavior.

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-15 all demonstrated
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

## Review Gate

Independent review of the eight commits 5f5b73261, e53c244ab, bd25f033e,
2e5e502f1, 5bcb20da2, 0b0c965b9, 3b376b4b7 and eff4c1e38, run by a context that
wrote none of them. Artifact:
`tmp/review/generated-command-usage-0d49d3a4-3753-4eb2-86d9-cd63bdb9cafb.md`,
verdict clean, 9 files pinned, `./le spec session review check` exits 0.

### Run 1
| Severity | Finding | File | Resolution |
|----------|---------|------|------------|
| BLOCKER | `show route lookup not-an-ip` stopped reaching `netip.ParseAddr` once the `ip` leaf declared a pattern, and answered `unexpected argument "not-an-ip", valid keywords: ip`. That names a keyword the published form `show route lookup <ip>` never asks anybody to type and drops the reason the value was refused. `test/ui/cli-verb-daemon-dispatch.ci` check 10 was red at HEAD on it | `internal/component/plugin/server/command.go`, `positionalError` | Fixed. `positionalError` reads the definitions still OPEN and answers the refusal of the one that is open, so the message is `invalid value "not-an-ip", does not match expected pattern` -- the family `show log recent count not-a-number` already answered in. Check 10 now drives a value the pattern ADMITS and `ParseAddr` refuses, check 10b is new and covers the value the pattern refuses, and both were proven to redden |
| ISSUE | `TestDeclaredValuesKeepAcceptedInvocations` keeps `show metrics name ze_bgp_updates_total peer=edge1` and calls the packed token a label filter. `metricLabelFilters` refuses that form now, so the row states a kept invocation the product no longer keeps | `internal/component/plugin/server/usage_model_test.go` | NOT landed, and this is the reason: another session holds that file with an uncommitted `request interface migrate` case whose YANG is also uncommitted, so committing the file would break HEAD the way 5f5b73261 did twice. The correction is one case: input `show metrics name ze_bgp_updates_total label peer edge1`, args `label peer edge1` |
| NOTE | Deleting an authored sentence AND renaming the command path in one commit passes the HEAD comparison: `usageContract` skips a HEAD path the working tree no longer reaches | `internal/le/docvalid/usage.go` | Accepted. Every other attempt to defeat the guard fails closed, including deleting the `ze:command` (the generated line becomes empty and the shapes differ) |
| NOTE | `request interface migrate` at HEAD documents `--from`/`--to` flags, which `ai/rules/cli.md` forbids and `firstFlagToken` refuses | `internal/component/iface/cmd/cmd.go`, `parseMigrateArgs` | Another session is converting it to keyword groups in the working tree. Out of this spec's diff |

### Run 2
| Severity | Finding | File | Resolution |
|----------|---------|------|------------|
| - | No BLOCKER and no ISSUE. The dispatcher package, the command, yang, docvalid, cmd/ze, show, iface, wikicatalog and site packages are green, and `ze-test fixture ui/cli-verb-daemon-dispatch` exits 0 | - | - |

### What the review verified against source

| Claim | How |
|-------|-----|
| Moving a leaf onto a container changed no command's argument set | A dump of every node's `ArgDef` at 2e22aebb1 and at HEAD differs in ONE line, the new `show metrics name label` group. So the `ze:inherit "none"` opt-out is COMPLETE: a command that gained an inherited leaf it did not declare before would appear as a set difference, and none does |
| 19 commands inherit and 2 opt out | A walk of the built tree prints `selector@peer` on nine `request peer`, five `show bgp peer` and `update bgp peer prefix`, `name@interface` on four `request interface`, and the opt-out on `request interface migrate` and `show bgp peer list` |
| Positional binding is order-independent | `positionalDef` ranks by `command.Constraint` and breaks a tie by name, which is the ALPHABETICAL order the slice used to carry, so a same-rank pair binds exactly as it did. A wrong bind fails closed at the selector fence: `Dispatch` adopts a lone positional only when `positional[selectorLeaf]` holds it |
| The generated line reads the model only | `usage.go` imports `textbuf` and reads `Node` and `ArgDef`. No description reaches it |
| HEAD compiles | `go build ./...` and `go build -tags <each of the nine>` `./cmd/ze` both exit 0 in a detached worktree at eff4c1e38 |
| Neither repair reverted another session's work | e53c244ab restores two string literals the consumer never needed; bd25f033e ADDS a const block. Both files are clean in the working tree, so nothing was overwritten |

## Closure status: one command left, and it needs its own spec

Measured on 2026-08-30 with `./le docvalid usage-contract`, over an overlay of
HEAD carrying the working tree: 384 command nodes, 1 authored usage sentence,
1 disagreement, 0 deletions hiding a difference.

Seven sentences were corrected to the generated line and then deleted, and
`show policy test peer` took the same two steps in `eefc62d2e` and `73bd01635`.
That command needed a model change rather than a prose change: `update` was a
mandatory leaf, so the renderer published a bare positional the parser refuses
(`parsePolicyTestArgs`, `internal/component/bgp/plugins/cmd/policy/handler.go`).
`filter` and `source-asn4` had the same shape and rendered correctly only by
accident. All three are now `ze:modifier` containers, and the renderer no longer
hoists a required group ahead of an optional one: declaration order decides,
because nothing else knows which group an operator reads first.

`announce` is the one sentence left, and this spec was wrong about why. It was
declared the permitted residue on the reading that the model cannot state a
route specification. Half of that is false. `handleAnnounce`
(`internal/component/bgp/plugins/cmd/announce/announce.go`) reads `args[0]` as
the FAMILY word and switches on `unicast`, `blackhole` and `flowspec`; the peer
selector arrives out of band through `ctx.PeerSelector()`. So the generated
`announce <selector>` names a first token no operator types, and the family word
IS statable, as the enum leaf the other eight commands now use. Only the three
argument grammars behind it are not.

The module's own 2026-08-29 revision records the fix: `withdraw` was split into
`tag`, `id` and `all`, one `ze:command` per form, each stating its own
arguments. Doing the same to `announce` adds three wire methods and three
handler registrations, which is a command-surface change with `.ci` obligations
of its own (`ai/rules/cli.md`). It needs its own spec, not a branch in this one.

The row is in `plan/journal/command-takes-an-untyped-positional-value.md`.

Two further facts the next session needs. The gate is not wired into
`./le verify`, so nothing runs it: `usageRow` and its siblings stay
package-private until the prose is gone. And `./le` is an existence cache, so a
stale `bin/le` reports stale counts; build a fresh one before reading them, and
only when the tree compiles.
