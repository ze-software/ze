# The system plugins door

<!-- door-base: internal/plugins internal/exabgp -->

Read `system.md` first. This door covers `internal/plugins/*` except the routing
protocols (`ospf`, `isis`, `ldp`, `rsvpte`, which are `routing-protocols.md`):

- DHCP
- NTP
- static routes
- the `*-cmd` command providers
- the firewall backends
- the TFTP and image servers
- `init`
- `systemd`
- the ExaBGP bridge with its translator and migration under `internal/exabgp/`

Sysctl is not here: it is
`internal/component/sysctl`, in `forwarding.md`.

## Where it sits

- **Edge tier.** A package lands in `internal/plugins/` when it is a
  config-driven engine that nothing else depends on. If a feature starts to
  import it, it is a component and moves (`docs/architecture/module-tiers.md`,
  "The two axes"). `./le arch tier check` judges the placement.
- **Two shapes share the directory.** A command-only plugin (`host-cmd`,
  `pki-cmd`, `traffic-cmd`, the other `*-cmd` folders) owns the command YANG and
  the RPC handlers and delegates the work to a component or core package. A
  full-subsystem plugin (`dhcpserver`, `ntp`, `static`, `exabgp`) also owns its
  runtime, config and state (`docs/architecture/command-ownership.md`, "Three
  Directories, Three Roles").
- **Most of them can be compiled out.** `feature-gates.txt` gates `ntp`,
  `dhcpserver`, `tftpserver` and `imageserver` (`ze_pxe`), `exabgp`, and the vpp
  backends of `firewall` and `static`. `static`, `firewall/nft` and most `*-cmd`
  plugins are always linked. Read the file, it is the only list.
- **Firewall lowering is a backend, not an engine.** `firewall/nft` and
  `firewall/vpp` call `firewall.RegisterBackend` (`internal/component/firewall`
  `RegisterBackend`). The component owns the model, and the plugin turns it into
  kernel or VPP state (`ai/digests/firewall.md`).

## Who depends on it from outside

| Outside party | Contract | Breaks when |
|---------------|----------|-------------|
| An operator's ExaBGP script | ExaBGP's text command grammar going down, ExaBGP JSON (envelope `6.0.0`) or text going up, chosen by the process's `encoder` leaf (`docs/architecture/exabgp-bridge.md`) | A translated form or an event field changes. The script was written against ExaBGP, and Ze's own CLI does not reach it |
| An operator who ran `ze exabgp migrate` | The `exabgp { bridge { ... } }` root the migration writes, and the registry name `exabgp-bridge` (`exabgp-bridge.md`, "Config root") | The migration and the runner disagree about a leaf |
| An ExaBGP script, through the bridge | The bridge sends `send bgp <selector> update text ...` command strings into the daemon (`internal/exabgp/bridge` `TranslateLine`) | A command the bridge sends is renamed. The dispatch key is the YANG path, so a move is a wire break (`docs/architecture/api/commands.md`, "Dispatch keys are YANG paths") |
| DHCP, TFTP and HTTP clients during PXE | RFC 2131, RFC 2132 and the boot flow (`docs/architecture/provisioning/`) | Any wire change. The client is boot firmware, which Ze cannot change |
| NTP servers, chrony included | The NTP exchange in `internal/plugins/ntp` `syncWorker.doSync` | Ze misreads a server's answer (`plan/journal/error-answered-with-another-errors-code.md`, 2026-10-19) |
| An operator | Each plugin's config YANG, its commands, its error text | A leaf or a command changes without its page |

## Boundaries

- **Every plugin here can run in another process.** The bridge has a second
  runner for exactly that: `ze exabgp plugin` runs as an SDK process over TLS
  connect-back, beside the internal runner (`exabgp-bridge.md`, "The internal
  runner cannot read the `run` line"). Reach the engine through the SDK, the
  bus, or a command, never through a Go call into a component's state
  (`docs/architecture/plugin/plugin-system.md`, "The process boundary").
- **A plugin owns its whole surface.** Its commands, YANG, doctor checks and help
  live in its folder. A central verb package such as `internal/component/cmd/show`
  must not register them or spell them (`command-ownership.md`, "What the Removal
  Test Forbids").
- **Always-on code never imports a gated plugin.** Extract a shared helper to an
  `internal/core/` leaf first (`docs/architecture/plugin/feature-gates.md`, "The
  one invariant").
- **A declaration is a pure value.** Hand the declaration and the start
  function to `sdk.RunOrDeclare` (`pkg/plugin/sdk` `RunOrDeclare`) as separate
  arguments (`ai/rules/plugins.md`).
- **Reuse the RPC line format.** The bridge speaks the plugin protocol through
  `pkg/plugin/rpc`, never through its own parser.

## Read before code

| Read | It answers |
|------|-----------|
| `docs/architecture/command-ownership.md` | Where a command, its YANG and its handler go, and the removal test that judges it |
| `docs/architecture/module-tiers.md` | Plugin or component: the two axes and the gate |
| `docs/architecture/plugin/feature-gates.md` | How to gate a plugin, and what always-on code is allowed to import |
| `docs/architecture/plugin/plugin-system.md` | Registration fields, startup stages, the process boundary, `OnConfigApply` |
| `docs/architecture/exabgp-bridge.md` | The two runners, translation both ways, batching, reload refusal |
| `docs/architecture/provisioning/` | DHCP, TFTP, image server and PXE staging |
| `docs/architecture/static-routes.md`, `docs/architecture/firewall/` | Static route install and the firewall backends |
| `ai/digests/fib-programming.md`, `ai/digests/firewall.md`, `ai/digests/dns-services.md` | The traced flows for static, firewall lowering, and the DNS plugins |
| `ai/patterns/plugin.md`, `ai/patterns/cli-command.md` | The file layout for a new plugin and a new command |
| `ai/rules/plugins.md`, `ai/rules/architecture.md` | The ownership, placement and declaration obligations |

## Traps

- **A nil parsed config read as "nothing to apply".** `parseStaticConfig`
  returned nil both for "no static section" and "no route", so `OnConfigApply`
  skipped a real removal. `copp`, `policyroute` and `ntp` carried the same guard
  (`plan/journal/zero-value-as-valid-answer.md`, 2026-08-18).
- **A plugin that owns config and registers no `OnConfigApply` refuses every
  commit.** The SDK binds the missing callback to `refuseConfigApply`
  (`pkg/plugin/sdk/sdk_callbacks.go`). The RECURRING-PATTERNS entry "An
  unregistered plugin callback answers OK" describes the behavior before that
  fix.
- **A second copy of the RPC line format.** `bridge_muxconn.go` open-coded the
  parse and format beside `rpc.ParseLine` and `rpc.AppendRequest`
  (`plan/journal/helper-bypassed-by-an-open-coded-copy.md`, 2026-08-22).
- **Always-on code that imports a gated package.** `internal/exabgp/bridge`
  imported the gated `bgp/types`, and `./le arch tier check` went red
  (`plan/journal/feature-test-missing-build-tag.md`, 2026-09-21).
- **A consumer that reads another envelope.** `flowspec-firewall` unmarshaled a
  flat envelope the daemon does not emit
  (`plan/journal/declared-format-contradicts-payload.md`, 2026-08-19).
- **A leaf-list read as one value.** `imageserver` served the first
  `listen-interface` only and dropped the rest without a log line
  (`plan/journal/multi-valued-leaf-read-as-single-valued.md`, 2026-09-08).
- **One backend fixed, its sibling not.** The vpp firewall verifier refuses the
  ARP and ND matches the nft backend lowers
  (`plan/journal/guard-added-to-one-half-of-a-pair.md`, 2026-09-27).
- **Backend registrations that tests never remove** fail a second
  `go test -count=2` pass (`plan/journal/registry-contamination.md`, 2026-10-17).

## Gates

| Command | It judges |
|---------|-----------|
| `./le arch tier check` | Placement, and no always-on import of a gated package |
| `./le plugin boundary check` | No same-process-only call that fails silently when the plugin runs external |
| `./le plugin imports check` | The composition root names every registering package |
| `./le plugin declarations check` | The runner's Stage 1 commands match the registration |
| `./le cli ownership`, `./le cli dispatch check` | One owner per command, and every command string the tree sends still routes |
| `./le doc yang-contract command-contract` | Every YANG command node has a handler, and every handler a node |
| `./le repo feature-tags check` | The files derived from `feature-gates.txt` are current |
| `./le test functional dhcp`, `static`, `firewall`, `exabgp-test` | The functional suites for this area. `exabgp-test` runs every ExaBGP compatibility case |
