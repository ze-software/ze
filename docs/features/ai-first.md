# AI-First Design

<!-- source: internal/component/mcp/tools.go -- MCP tool dispatch primitives -->
<!-- source: internal/plugins/help/help_ai.go -- ze help ai machine-readable reference -->
<!-- source: internal/test/cli/cmd_mcp.go -- MCP test client -->
<!-- source: ai/rules/repo-maintenance.md -- Current Discovery Surfaces -->
<!-- source: internal/le/repo/inventory/register.go -- inventory command registration -->

Ze uses shared registrations to describe commands and configuration. The CLI,
MCP and other operator interfaces read this metadata for the surfaces they
support. AI tools can discover the command catalog, call daemon commands and
read structured output while helping an operator use or debug the system.

## Shared Registrations

The core design point is broader than AI. A feature added to Ze should avoid
separate hand-written glue for every surface. The same registration path can
feed the CLI, SSH sessions, the web workbench, REST/gRPC, MCP, generated
references, completion, authorization, audit, and diagnostics.

## CLI commands as the API surface

The CLI and MCP use the same command catalog and daemon dispatcher, with
availability determined by each command's registration. Offline commands may
have a different daemon path, interactive views require a terminal, and
streaming-only commands are excluded from one-shot surfaces. A shared catalog
therefore describes which commands each interface offers; it does not promise
that every CLI interaction has an equivalent MCP call.
<!-- source: internal/component/command/surface.go -- AnswerSurface -->

## Self-Describing Command Reference

```
ze help command              # full command catalog, filterable
ze help command bgp          # filter to BGP-related commands
ze help command --json       # machine-readable JSON (for tooling, wiki generation)
ze help ai                   # AI-oriented summary with recipes and context
ze help ai --json            # machine-readable JSON reference
ze help ai api               # daemon API endpoints (ze-bgp:*, ze-iface:*, ...)
```

The command and RPC listings are assembled from the binary's plugin registry,
YANG schemas and command registrations. Generation removes a separately
maintained list of those definitions. Descriptions can still be wrong, and the
hand-written recipes need updating when command behavior changes.

`ze help command` is the human-facing catalog. The native
`./le cli catalog update file <destination>` action renders the corresponding
Markdown from the live registries.

Both commands are served by the daemon too, so `ze cli -c`, SSH, REST, gRPC,
MCP and the web reach them, under `show command catalog` (`help command`) and
`show command reference` (`help ai`): `help` is an offline root, not a verb,
so no daemon command path starts with it. `--json`, `--verbose` and `-v` are
flags of the local `ze help` command only. On every other surface `show
command catalog` answers its catalog rows as data: it prints the table, and
`show command catalog bgp | json` gives the same rows as JSON. `show command
reference` answers its Markdown page as text, and the structured reference is
the MCP `ze_reference` tool. A bare word means the
same on every surface: `help command json` filters on the word `json`, and
`help ai json` prints the summary.
<!-- source: internal/plugins/help/register.go -- render, renderTo -->
<!-- source: internal/plugins/help/help_command.go -- catalogAnswer, helpCommandAnswer, drawCatalog -->
<!-- source: internal/plugins/help/help_ai.go -- aiHelpAnswer, referenceAnswer -->

## Structured Diagnostics

```
ze cli -c "show config validation <file> | json"
ze show diagnostic [--json] <diagnostic-code>
ze config fix --plan <file>
```

The validation verdict is a record the pipe layer renders, so `| json`,
`| yaml` and `| table` are three views of one payload and no command carries a
rendering flag of its own. The fix plan prints for a reader.

Config validation emits diagnostic records with a code, severity and message.
Source spans, expected/actual facts, help and repair metadata are included when
the producing check supplies them. Agents must handle their absence.
<!-- source: internal/core/diagnostic/types.go -- Diagnostic -->
<!-- source: internal/component/config/cli/cmd_validate.go -- runValidation, validationResult.addError, validationResult.addErrorWithRepair -->

Codes such as `config-parse`, `config-yang-type` and
`config-listener-conflict` identify the kind of failure.
`ze show diagnostic <code>` explains a registered code.
The readiness checks `ze doctor` runs report codes from the same table, which the
package owning each dependency declares: the DNS resolver's
`doctor-dns-resolver-port-unavailable` (a listener port the system will not let ze
bind) and `doctor-dns-resolver-upstream-unreachable` (no forward server answers a
probe), for example.
<!-- source: internal/core/diagnostic/codes.go -- CodeDoctorDNSResolverPortUnavailable, CodeDoctorDNSResolverUpstreamUnreachable -->
<!-- source: internal/plugins/dnsresolver/doctor.go -- doctorChecks -->


`ze config fix --plan <file>` reports candidate repairs without editing files.
Where a repair is available, its diagnostic can carry a stable repair ID and a
safety label such as `format-only`, `section-local`, `behavior-preserving` or
`requires-human-review`. A diagnostic does not always have an automatic repair.
<!-- source: internal/component/config/cli/cmd_fix.go -- resolveFixPlan -->

A doctor diagnostic can also carry `fix`: the command or config statement the
operator runs to clear the problem, held alone with no label of its own. It is
not a repair ID and carries no safety label. The `ze doctor` text report draws
it on a line of its own under the finding, opened by `fix:`, and JSON carries
it under `fix` when the check names one.
<!-- source: internal/core/diagnostic/types.go -- Diagnostic -->
<!-- source: internal/component/doctor/doctor.go -- outputText, outputJSON -->

## Bundled Skills

```
ze show skills
ze show skill ze-diagnostics
ze show skill ze --full
```

The installed binary serves the workflow guides embedded when it was built.
They cover diagnostics, config, commands and agent edit loops, so an agent can
load the guide relevant to its task without fetching a separate version.
Embedding fixes which text the binary serves. It does not check that every
example describes the implemented behavior.
<!-- source: internal/plugins/skills/main.go -- skillFS, showSkill -->

Over the daemon (MCP, REST, gRPC, SSH, web) the expanded form is the model's
flag leaf `full`, as in `show skill ze full`; the local `--full` means the same.
<!-- source: internal/plugins/skills/yang/ze-skills-cmd.yang -- leaf full -->
<!-- source: internal/plugins/skills/main.go -- showSkill -->

## Development-Time Discovery

Feature, tooling, self-check, verification and test-infrastructure changes
must update their discovery path in the same work. The standard path is
`ai/rules/repo-maintenance.md` for policy, `ai/INDEX.md` for keyword lookup,
and the relevant native action or docs page for verification and usage.

<!-- source: ai/rules/repo-maintenance.md -- discovery path directive -->

Agents should use the existing inventory and verification surfaces:
`./le repo inventory`, `./le cli list`, `./le doc check all`,
`./le doc index update`, and `./le doc wiring`.

`./le commit create` prepares a commit: agents supply the vetted subject, body
and explicit file list, and the helper validates the selected content and writes
a message file and executable script. Preparation leaves the shared Git index
untouched. Running the generated script creates and publishes the commit using
a private index.

The script checks whether another commit changed a selected path since
preparation. It builds on a parent read for that attempt and publishes with a
compare-and-swap against that parent. If HEAD moved elsewhere, it rechecks the
selected paths and rebuilds on the new parent, for at most five attempts.
<!-- source: internal/le/commit/prepare.go -- Create -->
<!-- source: internal/le/commit/script.go -- renderPublish, publishAttemptsMax -->

<!-- source: internal/le/doc/check/actions.go -- Actions -->

## MCP Transport

The MCP (Model Context Protocol) server exposes daemon command calls through
the shared dispatcher:

| Tool | Description |
|------|-------------|
| `ze_execute` | Dispatch a daemon command string, subject to the caller's permissions and the command's supported surface |
| `ze_reference` | Reference assembled from this binary's registries and schemas; the same data as `ze help ai --json` |
| Generated tools | Command groups named from their registered paths, with an `action` enum where the group has named actions |
<!-- source: internal/component/mcp/tools.go -- toolHandlers, groupCommands, buildToolDef -->

The server builds generated tools from the live command list at `tools/list` <!-- doc-links: ignore (MCP method name, not a path) -->
time. YANG RPC input metadata supplies typed parameters when it is available;
groups without typed parameters expose a generic `arguments` string. Tool
names and grouping depend on the registered command population, so clients
should read the live tool inventory rather than assume a route-specific tool
such as `ze_announce` or `ze_withdraw` exists.
<!-- source: internal/component/mcp/tools.go -- buildToolDef -->

`ze_execute` accepts the same daemon command grammar as other command callers.
It does not turn offline file operations, interactive terminal views or event
subscriptions into equivalent MCP operations. The presence of notification
metadata in `ze_reference` describes the schema, and does not by itself provide
an event subscription.
<!-- source: internal/component/command/surface.go -- AnswerSurface -->
<!-- source: internal/component/aihelp/aihelp.go -- Build -->

Start with `ze start --mcp <port>` or configure via YANG (`environment/mcp`).

## What Makes This Different

Other software adds an API endpoint and expects operators to build separate
wrappers, UI, documentation, and automation around it. Ze exposes its command
surface through a self-describing interface that tools can discover at runtime.
`ze help command` lists the commands this binary carries with their descriptions.
`ze help ai` adds context such as recipes, families and update syntax. Generated
MCP tools use typed parameters where the registrations provide them and generic
arguments otherwise. `system command list` reads the running daemon's builtin
and plugin-declared command registries; `system command help name "<path>"`
reads a command's help. The separate `show command list` lists builtins;
`show command help <path>` can resolve both registries.
<!-- source: internal/component/plugin/server/system.go -- commandRows, handleSystemCommandHelp -->
<!-- source: internal/plugins/meta/cmd/help.go -- handleBgpCommandList, handleBgpCommandHelp -->

The useful property is the shared surface: when a plugin or subsystem registers
commands, YANG, RPCs, or events, the same metadata can feed CLI, web, generated
references, automation, authorization, audit, and diagnostics.

See [MCP Guide](../guide/mcp/overview.md) for configuration and
[MCP Remote Access](../guide/mcp/remote-access.md) for tunneling.
