# Command Reference

Ze commands fall into two categories: **shell commands** that run locally
and **runtime commands** sent to the running daemon via SSH.
<!-- source: cmd/ze/main.go -- main dispatch -->

This page explains the command model. For a live, searchable list of every
command with its description, run `ze help command` (or `ze help command --json`
for machine-readable output). The wiki's auto-generated
[command-catalog](https://github.com/ze-software/ze/wiki/command-catalog)
is produced from this JSON.

For the generated cross-vendor migration view (Junos MX, Cisco IOS XR,
Nokia SR OS, and VyOS), use the website's
[Command Equivalents](https://ze-software.net/command-equivalents/) page.
It joins `ze help command --json` with the curated vendor mapping in the
website branch, so Ze command additions appear as unmapped rows until a vendor
equivalent is added. For code-tree readers, the maintained data and generator
are in the repository on the `gh-pages` branch:
[`data/command-equivalents.json`](https://github.com/ze-software/ze/blob/gh-pages/data/command-equivalents.json)
and
[`tools/render-command-equivalents.py`](https://github.com/ze-software/ze/blob/gh-pages/tools/render-command-equivalents.py).
<!-- source: ../gh-pages/tools/render-command-equivalents.py -- load_inputs, build_rows -->
<!-- source: ../gh-pages/data/command-equivalents.json -- vendor mapping -->

## Conventions

Every command that takes a **filename argument** accepts `-` as an alias for
**stdin** when reading and **stdout** when writing, so configs and data stream
through pipes without temp files:

```
generate | ze config show -                 # read a config from stdin
ssh host cat rib.mrt | ze analyze show -    # analyse an MRT streamed in
ze config show - | ze config set - bgp session asn local 65001 | ze config validate -
ze config migrate -o - old.conf             # write the migrated config to stdout
```

stdin can be consumed once: a second `-` in a single command fails with a clear
error rather than reading empty. A file literally named `-` is addressed as
`./-`. Interactive/history-dependent editor commands (`ze config edit`,
`ze config rollback`, `ze config history`) reject `-` because they need a TTY or
on-disk revision history that a pipe does not have.
<!-- source: internal/core/cliio/cliio.go -- ReadFile/OpenReader/Create/WriteFile, ErrStdinClaimed -->

## Shell Commands

Run directly from the terminal. No daemon required (except `ze signal`, `ze status`,
`ze cli`, and daemon-targeted `ze show` subcommands).
Some `ze show` subcommands run locally: `version`, `bgp decode`, `bgp encode`,
`env`, `schema`, `yang`, `completion`.

### ze

Start the daemon or access subcommands. When invoked with no arguments in an
interactive terminal, shows a navigable menu of all commands grouped by section.
When piped or scripted (stdin is not a TTY), prints static help and exits 1.
<!-- source: cmd/ze/tui_menu.go -- runTUILauncher, buildTopLevel -->

```
ze                               # Interactive command menu (TTY only)
ze start <config-file>           # Start daemon from a config file (keyword-first)
ze start                         # Start daemon from database
ze -                             # Start daemon reading config from stdin
```
<!-- source: cmd/ze/ze_core_start.go -- cmdStart, startConfigPath (ze start <config-file>) -->
<!-- source: cmd/ze/ze_core_dispatch.go -- zeDispatch (- stdin sentinel; no free-form config-path sink) -->

The bare `ze <config-file>` form was **removed**: a free-form path in the first
position can collide with a command name (a config file named `bgp` or `signal`
would dispatch as that command). Use `ze start <config-file>`. Reading config
from stdin (`ze -`) is unaffected.

### Demo: Discover Ze commands interactively

Use type-ahead filtering and drill-down navigation in Ze's interactive command launcher.

[Play the WebM recording](../../../assets/demos/launcher.webm?v=46c97f8572) · [View the poster](../../../assets/demos/launcher.png?v=cae872cf66) · [Plain-text transcript](../../../assets/demos/launcher.txt?v=0399dbc59f)

Recorded with Ze 26.07.18 on macOS and Linux using VHS 0.11.0. Duration: 1 minute 5 seconds.

```console
$ ze

Type "show" to filter the command launcher, then press Enter to open the show command tree.
Type "traceroute" to find the path diagnostic command.
Press Escape and Left to return, then type "doctor" to find the readiness checker.
Press Escape to move back through the menu and return to the shell.
```


| Flag | Purpose |
|------|---------|
| `-d`, `--debug` | Enable debug logging |
| `-f <file>` | Use filesystem directly, bypass blob store |
| `--plugin <name>` | Load plugin before starting a YANG/native config (repeatable). Hub/orchestrator configs reject this; use `plugin { internal ... }` or `plugin { external ... }` in the config instead. |
| `--plugins` | List available internal plugins |
| `--pprof <addr:port>` | Start pprof HTTP server |
| `-V`, `--version` | Show version (also available as `ze show version`) |
| `--chaos-seed <N>` | Enable chaos self-test mode |
| `--chaos-rate <0-1>` | Fault probability per operation |
| `--server <host:port>` | Override hub address for managed mode |
| `--name <name>` | Override client name for managed mode |
| `--token <token>` | Override auth token for managed mode |
| `--color` | Force colored output (even when not a TTY) |
| `--no-color` | Disable colored output (also: `NO_COLOR` env var, `TERM=dumb`) |
<!-- source: cmd/ze/main.go -- global flag parsing -->

### ze config validate

Validate a configuration file without starting the daemon.

```
ze config validate <config-file>
ze config validate -q <config-file>     # Quiet: exit code only
ze config validate --json <config-file> # JSON output
```

| Flag | Purpose |
|------|---------|
| `-v` | Verbose output |
| `-q` | Quiet mode (exit code only) |
| `--json` | JSON output |

Exit codes: 0 = valid, 1 = invalid, 2 = file not found.

Validation includes the commit-time backend capability gate: a config whose
active `backend` leaf does not implement a feature it uses (e.g. `backend vpp`
with a `bridge`, `tunnel`, `wireguard`, `veth`, or `mirror` entry) is
rejected with one error per offending YANG path, matching the diagnostic
the running daemon produces on reload. See
[Backend Capability Errors](../configuration/index.md#backend-capability-errors).
<!-- source: internal/component/config/cli/cmd_validate.go -- cmdValidate, backend-gate loop -->
<!-- source: internal/component/config/backend_gate.go -- ValidateBackendFeatures -->

### ze config

Configuration management.

**Editing:**

```
ze config edit [file]            # Interactive editor
ze config set <file> <path> <value>
ze config deactivate <file> <path>  # Mark a node inactive (kept in file, skipped at apply)
ze config activate <file> <path>    # Clear the inactive flag on a node
```

`deactivate` and `activate` accept any node type: leaf, container, list entry,
or leaf-list value. The deactivated node round-trips through save/load and is
skipped at apply time. See `docs/guide/config-deactivate.md`.

**Storage:**

```
ze config import <file>...       # Import files into the database
ze config import --name <n> <file>  # Import under a different name
ze config rename <old> <new>     # Rename a config in the database
ze config ls [prefix]            # List files in database
ze config cat <key>              # Print database entry
```

**Inspection:**

```
ze config validate <file>        # Validate configuration file
ze config show <file> [path...]  # Show the config tree at a path (one-shot; --json)
ze config dump <file>            # Dump parsed configuration
ze config diff <f1> <f2>         # Compare two configs
ze config diff <N> <file>        # Compare with rollback revision
ze config fmt <file>             # Format and normalize
```

`show` is the one-shot, non-interactive way to inspect a config subtree:
`ze config show ze.conf bgp peer edge1` prints only that subtree (list entries
are addressed by key). With no path it prints the whole parsed tree; `--json`
emits the subtree as a JSON object. Shell completion for the path tokens is
served by the `ze config completion` engine (`ze completion bash|zsh|fish`).
<!-- source: internal/component/config/cli/cmd_show.go -- cmdShow, showConfig, openShowEditor -->

**Shell completion (`--family`, config sections):**

```
ze completion flags <command path>   # Flags a subcommand accepts (registry inventory)
ze completion families               # Address families (completes --family <TAB>)
```

The generated bash/zsh/fish scripts (`ze completion <shell>`) complete
subcommand flag names from the registry inventory and `--family` values from the
address-family registry.
<!-- source: internal/component/command/registry/flags.go -- RegisterCommandFlags, CommandFlags -->
<!-- source: internal/plugins/completion/flags.go -- writeFlags, writeFamilies -->

**History:**

```
ze config history <file>         # List rollback revisions
ze config rollback <N> <file>    # Restore revision N
ze config archive <name> <file>  # Archive config (see config-archive.md)
```

**Migration:**

```
ze config migrate <file>         # Convert old format to current
```
<!-- source: internal/component/config/cli/main.go -- subcommandHandlers, storageHandlers -->

| Flag | Purpose |
|------|---------|
| `-f` | Bypass database, use filesystem directly |
| `-o <output>` | Output file (migrate) |
| `--dry-run` | Show what would be migrated without changes (migrate) |
| `--list` | List available transformations (migrate) |
| `--format <fmt>` | Output format: `set` (default) or `hierarchical` (migrate) |

### ze signal

Send commands to the running daemon via SSH.

```
ze signal reload                 # Reload configuration
ze signal stop                   # Graceful shutdown (no GR marker)
ze signal restart                # Graceful restart (with GR marker)
ze signal reboot                 # Graceful shutdown + OS reboot (with GR marker, requires root on Linux)
ze signal status                 # Dump process status
ze signal quit                   # Immediate exit + goroutine dump (halt)
```

| Flag | Purpose |
|------|---------|
| `--host` | SSH host (default: 127.0.0.1 or `ze_ssh_host`) |
| `--port` | SSH port (default: 2222 or `ze_ssh_port`) |

Exit codes: 0 = ok, 1 = not running, 4 = command failed.
Reload is transactional: the daemon stages the new config as a candidate version,
runs verification and apply, then promotes the candidate to active only after the
runtime accepts it.
<!-- source: internal/plugins/signal/main.go -- Commands registry, ExitSuccess/ExitNotRunning/ExitNoCredentials/ExitSignalFailed -->

### ze status

Check if the daemon is running.

```
ze status
```

| Flag | Purpose |
|------|---------|
| `--host` | SSH host |
| `--port` | SSH port |

Exit codes: 0 = running, 1 = not running.
<!-- source: internal/plugins/signal/main.go -- RunStatus -->

### ze bgp

BGP protocol tools (offline, no daemon needed).

```
ze bgp decode <hex>              # Decode BGP message hex to JSON
ze bgp encode <route-command>    # Encode route command to BGP hex
ze bgp plugin cli                # Plugin debug shell (5-stage handshake + interactive)
ze bgp plugin cli --name <name>  # Debug shell with custom plugin name

# Also available via YANG verb dispatch (same behavior, no daemon needed):
ze show bgp decode <hex>
ze show bgp encode <route-command>
```

**decode flags:**

| Flag | Purpose |
|------|---------|
| `--open` | Decode as OPEN message |
| `--update` | Decode as UPDATE message |
| `--nlri <family>` | Decode as NLRI for family |
| `-f <family>` | Address family |
| `--json` | JSON output |
| `--plugin <name>` | Load plugin (repeatable) |

**encode flags:**

| Flag | Purpose |
|------|---------|
| `-f <family>` | Address family (default: ipv4/unicast) |
| `-a <asn>` | Local ASN (default: 65533) |
| `-z <asn>` | Peer ASN (default: 65533) |
| `-i` | Enable ADD-PATH (include path-id) |
| `-n` | Output only NLRI bytes |
| `--no-header` | Exclude BGP header |
| `--asn4` | 4-byte ASN (default: true) |
<!-- source: internal/component/bgp/cli/main.go -- Run; internal/component/bgp/cli/decode.go -- cmdDecode; internal/component/bgp/cli/encode.go -- cmdEncode -->

### ze show warnings / ze show errors

Operational report bus. A single place for Ze subsystems to surface
operator-visible issues. Warnings are state-based (a condition is currently
problematic and may resolve). Errors are event-based (something already
happened; no clear API). Both commands query the same in-process report
bus and return newest-first JSON snapshots.

```
ze show warnings                       # JSON: {"warnings": [...], "count": N}
ze show warnings source bgp            # only warnings from the bgp subsystem
ze show errors                         # JSON: {"errors":   [...], "count": N}
ze show errors source l2tp             # only errors from the l2tp subsystem
ze show errors source l2tp count 5     # last 5 errors from l2tp
```

**Issue shape** (every entry in both responses):

| Field | Type | Description |
|-------|------|-------------|
| `source` | string | Subsystem that raised the issue (`bgp`, `config`, `iface`, ...) |
| `code` | string | Stable kebab-case identifier of the condition or event |
| `severity` | string | `warning` or `error` |
| `subject` | string | What the issue is about: peer address, transaction id, file path |
| `message` | string | Human-readable one-liner |
| `detail` | object | Optional structured context (family, code/subcode, reason, ...) |
| `raised` | RFC 3339 time | When the issue first appeared on the bus |
| `updated` | RFC 3339 time | Most recent raise time (warnings advance; errors equal raised) |

<!-- source: internal/core/report/report.go -- Issue struct -->

**Day-one BGP vocabulary** (raised by the BGP reactor):

| Severity | Source/Code | When raised | When cleared |
|----------|-------------|-------------|--------------|
| warning | `bgp / prefix-threshold` | Per-family prefix count crosses the configured warning threshold upward | Per-family count drops below threshold |
| warning | `bgp / prefix-stale` | `peer { prefix { updated ... } }` date is older than 180 days | Peer re-added with a fresher date, or peer removed |
| error | `bgp / notification-sent` | This ze instance sends a NOTIFICATION to a peer (code/subcode in `detail`) | Never (errors are events) |
| error | `bgp / notification-received` | A peer sends a NOTIFICATION to this ze instance | Never |
| error | `bgp / session-dropped` | An Established session ends without a NOTIFICATION exchange (TCP loss, hold-timer with no notification, peer FIN) | Never |

<!-- source: internal/component/bgp/reactor/session_prefix.go -- report code constants and helper functions -->
<!-- source: internal/component/bgp/reactor/peer_stats.go -- IncrNotificationSent, IncrNotificationReceived -->
<!-- source: internal/component/bgp/reactor/peer_run.go -- raiseSessionDropped at FSM Established->Idle transition -->

**Capacity limits** (configurable via env vars):

| Env var | Default | Maximum | Purpose |
|---------|---------|---------|---------|
| `ze.report.warnings.max` | 1024 | 10000 | Cap on active warning set, oldest-by-Updated evicted at cap |
| `ze.report.errors.max` | 256 | 10000 | Ring buffer size for recent error events |

Over-limit raise calls are silently rejected and logged at debug level.
Field length limits (Source 64, Code 64, Subject 256, Message 1024, Detail 16 keys)
prevent any producer from pushing multi-megabyte entries.

<!-- source: internal/core/report/report.go -- validFields, maxWarningCap, maxErrorCap -->

**Login banner integration**: the Ze CLI login banner reads from the same bus,
filtered by source `bgp`. One active warning shows the detail line; multiple
warnings collapse to a count line pointing at `show warnings`.

<!-- source: internal/component/bgp/config/loader.go -- collectPrefixWarnings -->

### ze show audit / ze show aaa accounting

Audit and accounting visibility for operator actions.

```
ze show audit
ze show audit action config-commit
ze show audit actor alice surface web count 20
ze show audit since 2026-05-24T10:00:00Z until 2026-05-24T11:00:00Z
ze show aaa accounting
```

`show audit` returns `entries` and `count`. Each entry includes `timestamp`,
`actor`, `remote-addr`, `surface`, `action`, `detail`, and `outcome`.
Filters are optional and can be combined. Time filters use RFC 3339.

`show aaa accounting` currently reports TACACS+ accounting queue drops:
`dropped-records` is the number of START/STOP accounting records that could not
be queued because the worker was stopped or the queue was full.

<!-- source: internal/component/cmd/show/audit.go -- handleShowAudit -->
<!-- source: internal/component/aaa/cmd/show.go -- handleShowAAAAccounting -->

### ze show system

Daemon process introspection for the running ze process. `show runtime
memory` is the Go allocator view; `show system memory` (below) is the
OS view. Available via daemon SSH (online RPC); YANG registration only.

```
ze show runtime memory             # runtime.MemStats (alloc, heap, GC) + hardware enrichment
ze show system cpu                 # goroutine count, logical CPUs, GOMAXPROCS + hardware
ze show system date                # wall-clock time: RFC3339, Unix, timezone
ze show system platform            # runtime platform type and capabilities
```

Each response is a flat JSON map with kebab-case keys:

| Command | Top-level keys |
|---------|----------------|
| `show runtime memory` | `alloc`, `total-alloc`, `sys`, `heap-alloc`, `heap-sys`, `heap-in-use`, `heap-objects`, `stack-in-use`, `num-gc`, `gc-cpu-pct`, `hardware` (optional: physical memory + ECC from `host.DetectMemory()`) |
| `show system cpu` | `num-cpu`, `num-goroutines`, `max-procs`, `go-version`, `hardware` (optional: `host.DetectCPU()`) |
| `show system date` | `time` (RFC3339), `unix`, `unix-nano`, `timezone`, `utc-offset-secs` |
| `show system platform` | `type` (gokrazy, systemd, container, plain-linux, darwin), `read-only-root`, `perm-available`, `systemd-available`, `gokrazy-update-socket`, `gokrazy-ui-available`, `reboot-allowed`, `persistent-storage-writable`, `fd-limit-soft-current`, `fd-limit-hard-max`, `fd-limit-raisable` |
| `show system conntrack` | `count`, `max`, `buckets`, `expect-max`, `accounting`, `timestamp`, `checksum`, `log-invalid`, `modules` (loaded nf_conntrack_* list), `timeouts` (per-protocol), `tcp-behavior` (be-liberal, loose, max-retrans, ignore-invalid-rst) |

The `hardware` subobject under `memory` and `cpu` mirrors the data
returned by `show host memory` / `show host cpu`. Both paths are
correct; pick `show system *` when you want runtime-first with hardware
as context, `show host *` when you want hardware-first.

<!-- source: internal/component/cmd/show/system.go -- handleShowSystemMemory/CPU/Date -->

### show host

Host hardware inventory. Read-only. Walks sysfs/procfs (and issues best-effort
`ethtool` ioctls for NIC firmware/rings) to produce a structured JSON description
of the machine. Served by the daemon when reachable; when no daemon is running it
falls back to the same in-process detection library, so `show host` works whether
or not the daemon is up. JSON output for pipeline consumption (`jq`, Prometheus
scrapers, SNMP shims).

```
ze show host all                   # Full inventory (all sections), JSON
ze show host cpu                   # CPU only
ze show host nic                   # Physical NICs (virtual interfaces filtered)
ze show host dmi                   # DMI/SMBIOS board identity
ze show host memory                # /proc/meminfo (incl. hugepages) + ECC counters (edac)
ze show host thermal               # hwmon sensors + per-CPU throttle counts
ze show host storage               # Block devices + NVMe firmware
ze show host kernel                # Kernel release, cmdline, microcode, arch flags
```

### Demo: Inspect a Linux host before Ze starts

Use Ze's offline command fallback to read the complete kernel, CPU, and memory inventory in human-readable structured output.

[Play the WebM recording](../../../assets/demos/host-inventory.webm?v=8c89c5019c) · [View the poster](../../../assets/demos/host-inventory.png?v=01c12c6314) · [Plain-text transcript](../../../assets/demos/host-inventory.txt?v=5b221c4c0f)

Recorded with Ze 26.07.18 in a Linux namespace lab using VHS 0.11.0. Duration: 51 seconds.

```console
An operator needs to inspect an unfamiliar Linux host before starting Ze.

$ ze show host kernel | ze pipe yaml
The complete live kernel inventory is displayed.

$ ze show host cpu | ze pipe yaml
The complete CPU topology, model, core, and thread inventory is displayed.

$ ze show host memory | ze pipe yaml
The complete memory capacity, availability, cache, swap, and ECC inventory is displayed.

The commands work without a running Ze daemon. Every field returned by `ze show host` remains visible and machine-readable.
```


<!-- source: internal/plugins/host-cmd/cmd/show_host.go -- online `show host *` RPCs -->
<!-- source: internal/plugins/host/host.go -- RunShow offline fallback (registry.RegisterOfflineFallback) -->
<!-- source: internal/component/host/inventory.go -- Inventory struct and types -->

Online (daemon) and offline (fallback) paths share the same detection library, so
the JSON shapes are identical either way.

**Sections:**

| Section | Fields (kebab-case keys) |
|---------|--------------------------|
| `cpu` | `vendor`, `model-name`, `family`, `model`, `stepping`, `logical-cpus`, `physical-cores`, `threads-per-core`, `hybrid`, `scaling-driver`, `hwp-available`, `base-freq-mhz`, `max-freq-mhz`, `microcode`, `cores[]` with per-core `role` (`performance`/`efficient`/`uniform`), `current-freq-mhz`, `core-throttle-count`, `package-throttle-count` |
| `nic` | Per physical interface: `name`, `driver`, `pci-vendor`, `pci-device`, `mac`, `link-speed-mbps`, `duplex`, `carrier`, `rx-queues`, `tx-queues`, `ring-rx`, `ring-tx`, `firmware-version` |
| `dmi` | `system-vendor`, `system-product`, `board-*`, `bios-*`, `chassis-*` |
| `memory` | `total-bytes`, `free-bytes`, `available-bytes`, `buffers-bytes`, `cached-bytes`, `swap-total-bytes`, `swap-free-bytes`, `hugepages-total`, `hugepages-free`, `hugepage-size-bytes`, `ecc-correctable-errors`, `ecc-uncorrectable-errors`, `ecc-present` |
| `thermal` | `sensors[]` (hwmon: `name`, `device`, `temp-mc`, `alarm`), `throttle[]` (per-CPU `core-throttle-count`, `package-throttle-count`) |
| `storage` | `devices[]` with `name`, `size-bytes`, `model`, `serial`, `transport` (`nvme`/`sata`/`mmc`/`virtio`/`unknown`), `rotational`, `nvme-firmware-version` (NVMe only), `smart` (via direct ioctl, no smartctl binary: `healthy`, `temp-celsius`, `power-on-hours`, `error-count`, `percent-used` (NVMe), `available-spare` (NVMe); `unavailable` + `unavailable-note` when device lacks SMART or insufficient privileges) |
| `kernel` | `release`, `version`, `architecture`, `cmdline`, `boot-time` (RFC3339), `boot-time-unix`, `microcode-revision`, `arch-flags[]` (security-relevant subset: `smep`, `smap`, `ibt`, `user_shstk`, `ibrs`, `ibrs_enhanced`, `ssbd`) |

All temperatures are reported in **millicelsius** (kernel hwmon convention).
All sizes in **bytes**. All frequencies in **MHz**. Unreadable sysfs files
are omitted from the JSON rather than returning `null` or an empty string.
Permission errors are recorded in the inventory's `errors[]` array.

**Virtual interface filtering**: `ze show host nic` only reports physical
interfaces. The filter is structural (presence of
`/sys/class/net/<n>/device/`) rather than a driver-name allowlist, so
new virtual drivers (wireguard, ipvlan, etc.) are filtered uniformly.

**Platform**: Linux only. Darwin and other platforms return
`ErrUnsupported` per section; `ze show host` reports "unsupported on this
platform" so scripts can probe gracefully.

### show storage smart

Online RPC (requires running daemon with `storage { smart { enabled true } }` config):

```
show storage smart                 # Per-device SMART health status
```

<!-- source: internal/component/storage/show.go -- handleShowStorageSmart -->
<!-- source: internal/component/storage/manager.go -- Manager.Status -->

Returns a JSON array of per-device objects:

| Field | Description |
|-------|-------------|
| `name` | Block device name (e.g. `sda`, `nvme0n1`) |
| `transport` | `nvme`, `sata`, or `unknown` |
| `healthy` | SMART overall health assessment |
| `temp-celsius` | Current temperature (0 = not reported) |
| `power-on-hours` | Cumulative powered-on hours |
| `error-count` | Reallocated sector count (ATA) or media errors (NVMe) |
| `percent-used` | NVMe endurance estimate (0-255, NVMe only) |
| `available-spare` | NVMe spare capacity percentage (NVMe only) |
| `smart-enabled` | Whether SMART enable command succeeded |
| `last-checked` | Timestamp of last health poll |
| `last-short-test` | Timestamp of last short self-test (if scheduled) |
| `last-long-test` | Timestamp of last extended self-test (if scheduled) |

Temperature alerts are emitted to the report bus (`show warnings` / `show errors`):
`temp-high` (informational threshold), `temp-rising` (rate-of-change), `temp-critical` (critical threshold), `smart-failing` (health status failed).

### ze support

Generate a tech-support archive for troubleshooting. Collects system state,
health checks, configuration, logs, and diagnostics into a compressed tar.gz
with one JSON file per module. No shell-outs; gokrazy-safe.

```
ze support                             # Full bundle, all 20 modules
ze support --module version,doctor     # Only named modules
ze support --exclude dmesg             # All except named modules
ze support --json                      # Output manifest JSON to stdout
ze support --list-modules              # List available modules
ze support --reason "BGP flap"         # Embed reason in manifest
ze support --sensitive                 # Include passwords (default: redacted)
ze support --since 2h                  # Time scope for log collection
ze support --output /var/support/      # Output directory (default: cwd)
```

<!-- source: internal/component/support/support.go -- Run, collect, SupportManifest -->
<!-- source: internal/component/support/modules.go -- moduleRegistry, ModuleNames, ModuleList -->

### ze interface

OS network interface management (standalone, no daemon needed for most commands).
Show uses the verb syntax: `ze show interface`.

```
ze show interface                  # List all interfaces (also via daemon SSH)
ze show interface brief            # One-line-per-interface summary
ze show interface scan             # Discover and classify every OS interface
ze show interface name <n> detail  # Show details for one interface
ze show interface name <n> counters  # Counters only for named interface
ze show interface type <type>      # Filter by type (ethernet, bridge, vxlan, wireguard, ...)
ze show interface errors           # Interfaces with non-zero Rx/Tx error or drop counters
ze show interface rate             # Per-second rate data for all interfaces
ze show interface rate <name>      # Per-second rate data for one interface
ze show interface --json           # JSON output
ze interface create dummy <name>   # Create a dummy interface
ze interface create veth <n> <p>   # Create a veth pair
ze interface delete <name>         # Delete an interface
ze interface unit add <name> <id> [vlan-id <vid>]  # Add a logical unit
ze interface unit del <name> <id>  # Delete a logical unit
ze interface addr add <name> unit <id> <cidr>      # Add IP address
ze interface addr del <name> unit <id> <cidr>      # Remove IP address
ze interface migrate ...           # Make-before-break migration (requires daemon)
```

**`show interface name <name> detail`** (and the standalone
`ze interface show <name>`) shows the OS device name (`OS Name`) alongside the
configured name, and the NIC's permanent/factory MAC (`Perm MAC`) alongside the
operational MAC, so overriding an interface's MAC does not hide the device's
stable hardware identity.
<!-- source: internal/component/iface/cli/show.go -- showOne / formatInterfaceDetail -->

**Every subcommand above goes to the daemon; only the bare `ze show interface`
and `ze show interface <name>` are served in-process.** `show interface` is a
local handler registered at two words
(internal/component/iface/cli/register.go), and its own arguments are an
interface name, not a keyword. registry.LookupLocal refuses it for any argv that
reaches a declared command below it, so `brief`, `scan`, `type`, `errors`,
`rate` and the two `name ...` forms are answered by the daemon. Before that rule
existed, all seven reached the in-process handler and were read as interface
names: `ze show interface brief` looked for an interface called "brief".

**`show interface type <type>`** is case-insensitive; unknown types reject
with the sorted list of types actually present on the system. Empty-Type
interfaces are hidden from both the response and the valid-types list.

**`show interface errors`** skips interfaces without stats and interfaces
whose `RxErrors`, `RxDropped`, `TxErrors`, `TxDropped` counters are all
zero. The response includes only the four counter fields per interface
for compact diffing across snapshots.

### show traffic control

Traffic control (TC) state. Queries the active TC backend for qdisc, class,
and filter state per interface. Returns "traffic control not available on
this platform" when no TC backend is loaded (e.g. on macOS).

```
ze show traffic control            # Summary of all interfaces with qdiscs
ze show traffic control <ifname>   # Detail for one interface
```

### show flow export

Per-collector flow export statistics for the `flowexport` component (sFlow v5,
NetFlow v9, IPFIX). Returns `{"status": "not-configured"}` when no
`flow-export { }` section is present.

```
ze show flow export               # All configured collectors
ze show flow export <collector>   # One collector by name (error if not found)
```

Each entry reports `name`, `address`, `port`, `protocol`, `datagrams-sent`,
`bytes-sent`, `errors`, `sequence`, and `last-export-time` (Unix seconds,
omitted before the first poll). JSON by default; full pipe operators supported.
See the [Flow Export guide](../flow-export/index.md).

<!-- source: internal/plugins/flowexport/cmd_show.go -- handleShowFlowExport, ze-show:flow-export -->
<!-- source: internal/plugins/flowexport/exporter.go -- newExporter, exporter.status -->

### show traffic stat
<!-- source: internal/component/trafficstat/cmd/traffic.go -- handleShowTraffic -->

Aggregated traffic snapshot from the trafficstat service: per-interface rates,
top-N source/dest IPs, top ports (with service names from the portname table),
protocol mix (TCP/UDP/...), severity tier, and 60s history. Returns a degraded
snapshot (interface rates only) when no collector (traffic-usage / flow-export)
is active.

```
show traffic stat                     # All interfaces
show traffic stat name <interface>    # One interface by name
```

### monitor traffic stat
<!-- source: internal/component/trafficstat/cmd/render.go -- createTrafficMonitorSession -->

Full-screen live traffic monitor. Renders per-interface rates, top talkers,
top ports with service names and amplification labels, protocol mix percentages,
severity badge, and a sparkline history. Esc/q to quit.

```
monitor traffic stat                  # All interfaces
monitor traffic stat name <interface> # One interface by name
```

### show traffic feature
<!-- source: internal/component/trafficfeature/cmd/traffic_feature.go -- handleShowTrafficFeature, ze-show:traffic-feature -->

Neutral per-source traffic feature signals (facts, not verdicts): fan-out
(distinct destinations), out/in byte ratio (exfiltration), destination-port
entropy, new-peer, rare-port/proto, and coarse beaconing. Returns
`{"degraded": bool, "top-source-ips": [{address, fan-out, out-in-ratio,
port-entropy, new-peer, rare-port, beaconing}]}`; `out-in-ratio` is the string
`"inf"` when a source has no inbound bytes. Optional `name <address>` filters to
one source.

```
show traffic feature                  # Top source entities
show traffic feature name <address>   # One source by address
```

### show anomaly detect
<!-- source: internal/plugins/anomaly/detect/show.go -- handleShowAnomaly, ze-show:anomaly -->

Recent behavioral anomaly incidents from the report-only `anomaly-detect` plugin.
Returns `{"enabled": bool, "incidents": [{entity, cohort, score, severity,
at, fired-features: [{name, z}]}]}`; the ring is empty until an entity's
correlated deviation confirms. The detector reports only; the `anomaly/shape`
responder acts on these incidents.

```
show anomaly detect                   # Recent anomaly incidents
```

### show anomaly shape
<!-- source: internal/plugins/anomaly/shape/show.go -- handleShowAnomalyShape, ze-show:anomaly-shape -->

Status of the shadow-first anomaly responder. Returns `{"enabled": bool, "mode":
"shadow"|"armed", "action": "limit"|"drop", "kill-switch": bool, "armed-count": N,
"armed": [source, ...]}`. In shadow mode (default) nothing is installed; armed
sources carry a live rate-limit with a timed auto-revert.

```
show anomaly shape                    # Responder mode + armed sources
```

### show traffic usage

Per-interface eBPF byte accounting for the `traffic-usage` plugin (TCX
per-port/protocol, and per-IP when `track-ip` is enabled). Returns
`{"status": "not-configured"}` when no `traffic { usage { } }` section is present.

```
ze show traffic usage                 # All monitored interfaces
ze show traffic usage name <interface># One interface by name
```

Each entry reports `ingress-ports`, `egress-ports`, `map-entries`, and (only
when `track-ip` is enabled) `ingress-ips` and `egress-ips`. JSON by default;
full pipe operators supported. See the [Traffic Usage guide](../traffic-usage/index.md).

<!-- source: internal/plugins/trafficusage/show.go -- handleShowTrafficUsage, ze-show:traffic-usage -->

### show route / show neighbor

Kernel routing and neighbor tables. These commands are object-rooted (no
`ip` namespace); they dispatch through the iface backend; on the netlink
backend they read the live kernel state, on VPP they reject under
exact-or-reject since the kernel FIB/ARP table is not the authoritative
forwarding source there.

```
ze show neighbor                       # Kernel neighbor table (IPv4 ARP + IPv6 ND)
ze show neighbor ipv4                  # IPv4 only (alias: ze show arp)
ze show neighbor ipv6                  # IPv6 only (ND)
ze show route                          # Full kernel routing table (all protocols)
ze show route <cidr>                   # Filter to an exact CIDR match
ze show route default                  # Default route(s) (0.0.0.0/0, ::/0)
```

**`show neighbor`** (IPv4-only shortcut: **`show arp`**) returns per-entry
`address`, `mac-address`, `device`, `family`, and `state` (reachable,
stale, delay, probe, failed, permanent, noarp, incomplete). Unresolved
entries (no IP) are skipped. FAILED and INCOMPLETE entries are kept with
an empty MAC so operators can diagnose neighbor discovery problems.

**`show route`** renders the `protocol` field by name for well-known
values (kernel, static, bgp, ra, dhcp, zebra, ze for RTPROT_ZE=250, plus
ospf/isis/rip/eigrp/babel) and as a decimal string otherwise. Connected
routes have an empty `nexthop`; the `source` field carries the
preferred-source IP when the kernel reports one.

**`show ospf route`** returns the OSPF SPF route table: area, prefix,
metric, path type, advertising router, and equal-cost next-hop set. **`show ospf
route fast-reroute`** returns each prefix's primary next-hops with their RFC 5286
LFA / TI-LFA backup next-hop, protection class (node/link/downstream), and
TI-LFA repair label stack; unprotected primaries are shown as unprotected. **`show ospf
spf`** returns per-area SPF run state: last run time, duration, node count,
pending state, and current throttle delay. **`show ospf border-routers`**
returns reachable ABRs and ASBRs with area, metric, and next-hop set. **`show
ospf virtual-links`** returns each configured OSPF virtual link (RFC 2328 §15)
with its transit area, remote router-id, adjacency state, transit-area-computed
cost, and transit next hop.
**`show ospf database opaque-link`**, **`show ospf database opaque-area`**, and
**`show ospf database opaque-as`** filter the LSDB to the RFC 5250 opaque LSAs of
Type 9 (link-local), Type 10 (area), and Type 11 (AS) respectively; the opaque-area
and opaque-as views decode any RFC 3630 / RFC 5392 Traffic Engineering body inline.
**`show ospf te-database`** renders the Traffic Engineering Database: router addresses
plus TE links with their Link ID, local/remote address, link type, TE metric,
bandwidths, admin group, and (for inter-AS links) remote AS and remote ASBR.
**`show ospf database router-information`** decodes the RFC 7770 Router Information
LSAs for both address families (OSPFv2 Opaque type 4 and OSPFv3 function code 12) into
the advertised informational capability bits and the TLV list.

**`show ospf segment-routing`** and **`show ospf ipv6 segment-routing`** render the
RFC 8665 / RFC 8666 Segment Routing state for each address family: the configured
SRGB/SRLB label ranges, the advertised SR-Algorithm, this node's node Prefix-SIDs, and
the Adjacency-SIDs allocated per adjacency.

<!-- source: internal/plugins/ospf/register.go -- OnExecuteCommand show ospf route/spf/border-routers -->
<!-- source: internal/plugins/ospf/te_show.go -- teDatabaseSnapshot, teDecodeOpaqueLSA -->
<!-- source: internal/plugins/ospf/ri_show.go -- riDatabaseSnapshot, decodeRIBody -->
<!-- source: internal/plugins/ospf/show_database.go -- dbSubviewType opaque-link/opaque-area/opaque-as -->
<!-- source: internal/plugins/ospf/spf/route.go -- RouteSnapshotEntry -->
<!-- source: internal/plugins/ospf/spf/interarea.go -- BorderRouterSnapshotEntry -->
<!-- source: internal/plugins/ospf/spf/computer.go -- spfSnapshotEntry and BorderRouterSnapshot -->

<!-- source: internal/component/iface/cmd/show_neighbor.go -- handleShowNeighbor, handleShowArp -->
<!-- source: internal/component/iface/cmd/show_route.go -- handleShowRoute -->
<!-- source: internal/plugins/iface/netlink/neighbor_linux.go -- ListNeighbors -->
<!-- source: internal/plugins/iface/netlink/route_linux.go -- ListKernelRoutes -->

### show mpls

MPLS label-switching forwarding table, read directly from the kernel
AF_MPLS routing table (the authoritative dataplane state, like `show ip
route` for IP).

```
ze show mpls forwarding                 # All installed MPLS forwarding entries
ze show mpls forwarding limit 500       # Cap the response size
```

Each entry reports the incoming label (`in-label`), the `operation`
applied (`swap` when an outgoing label stack is programmed, `pop` for
disposition / implicit-null), any `out-labels`, the `next-hop`, and the
egress `device`. On non-Linux platforms the table is empty (the kernel
MPLS FIB is Linux-only).

<!-- source: internal/component/mpls/show_forwarding.go -- handleShowMPLSForwarding -->
<!-- source: internal/component/mpls/forwarding_linux.go -- dumpMPLSRoutes -->

### show rsvp-te

RSVP-TE (RFC 3209) LSP, admission, and RFC 4090 fast-reroute state, reported as
JSON by the rsvp-te component.

```
ze show rsvp-te session         # All LSPs: state, role, bandwidth, in/out labels, ERO/RRO
ze show rsvp-te interface       # Per-interface reserved / available / max bandwidth
ze show rsvp-te tunnel          # Configured ingress tunnels and their state
ze show rsvp-te fast-reroute    # RFC 4090 protection: bypass LSPs and protected LSPs
```

`show rsvp-te fast-reroute` lists each configured `bypass` LSP and each protected
transit LSP with its armed bypass, the protection `mode` (`facility`), whether
`node-protection` is requested, and whether local protection is `available` and
`in-use`. `show rsvp-te lsp` is an alias for `show rsvp-te session`.

<!-- source: internal/plugins/rsvpte/cmd_show.go -- show rsvp-te RPC proxies -->
<!-- source: internal/plugins/rsvpte/show_data.go -- show rsvp-te data builders -->

### show isis

IS-IS link-state IGP state, read from the running IS-IS engine. Each command
is a thin proxy to a fixed engine command; the output routes through the pipe
machinery (`| json`, `| table`, `| text`, `| count`, `| match`, `| resolve`,
`| origin`).

```
ze show isis neighbor                   # Adjacencies: system-id, interface, level, state, hold time
ze show isis database                   # LSDB summary: LSPID, sequence, lifetime, checksum, overload, own
ze show isis database detail            # LSDB with each LSP expanded into its decoded TLVs (same columns plus tlvs)
ze show isis route                      # IS-IS-computed IPv4 routes: prefix, metric, level, next-hops
ze show isis route ipv6                 # IS-IS-computed IPv6 routes (RFC 5308)
ze show isis interface                  # Circuits: level, type, metric, hello/hold, passive, DIS, adjacency count
ze show isis hostname                   # System ID -> dynamic hostname mapping (TLV 137, RFC 5301)
ze show isis spf-log                    # Recent SPF runs: timestamp, level, trigger, duration, node count
```

The neighbour and database views are also available in the web UI at `/isis`
and `/isis/database`, with live updates over SSE.

<!-- source: internal/plugins/isis/cmd_show.go -- ze-show:isis-* RPC proxies -->
<!-- source: internal/plugins/isis/show.go -- hostname/interface/spf-log render -->
<!-- source: internal/plugins/isis/yang/ze-isis-cmd.yang -- show/clear command tree -->

### clear isis

Runtime actions that reset IS-IS operational state without changing the
configuration.

```
ze clear isis adjacency                 # Tear down every adjacency; neighbors re-form from the next Hello
ze clear isis counters                  # Reset the SPF-run log (observational state)
```

`clear isis adjacency` drops the adjacency records so each neighbour re-learns
from the next Hello, triggering LSP re-origination and an SPF re-run; the
circuit is not closed. `clear isis counters` clears the SPF-run history surfaced
by `show isis spf-log`; the monotonic Prometheus series are process counters and
are not reset (resetting them mid-process breaks `rate()`).

<!-- source: internal/plugins/isis/cmd_show.go -- ze-clear:isis-* RPC proxies -->
<!-- source: internal/plugins/isis/show.go -- clearAdjacencies/clearCounters -->

### IS-IS (Offline Tools)

IS-IS protocol tools that run without a daemon, mirroring `ze bgp decode`.

| Command | Access | Purpose |
|---------|--------|---------|
| `isis decode [--pretty]` | offline | Decode a hex IS-IS PDU on stdin and emit JSON on stdout |

`ze isis decode` runs without a daemon. Input is ASCII hex (whitespace and
newlines allowed); raw PDU bytes piped straight from a capture also work (an
IS-IS PDU starts with the protocol discriminator `0x83`, which is not an ASCII
hex digit, so the two encodings are never confused). Output is the JSON view of
the decoded PDU (common header, body, and decoded TLVs). Use `--pretty` for
indented output. The verb is `isis decode` (a dedicated offline tool), distinct
from the `show isis` / `clear isis` command tree above.

Example:

```
echo 831b0100... | ze isis decode --pretty
```

Exit code is 0 on a successful parse, 1 on unreadable input, oversized input, or
a malformed PDU; stderr carries the reason.

<!-- source: internal/plugins/isis/cli/decode.go -- cmdDecode, toWire -->
<!-- source: internal/plugins/isis/cli/register.go -- isis root namespace, decode member -->

### show pki

PKI certificate store introspection. Shows certificates loaded from
the `pki {}` config section.

```
ze show pki certificates                           # List all loaded certs (CA + device)
ze show pki certificate <name>                     # Full details for a named certificate
ze show pki certificate <name> pem                 # PEM-encoded certificate (+ intermediate)
ze show pki certificate <name> bundle pem          # Certificate + private key in one PEM
ze show pki certificate <name> fingerprint         # SHA-256 fingerprint (colon-separated hex)
ze show pki certificate <name> fingerprint sha512  # SHA-512 fingerprint
```

**`show pki certificates`** returns a sorted list of all loaded
certificates with name, type (ca/device), subject CN, issuer CN,
expiry date, key algorithm, and validity status.

**`show pki certificate <name>`** returns full details: subject,
issuer, serial, validity period, key algorithm, key size, SANs,
key usage, private key presence, and chain validation status.

**`show pki certificate <name> pem`** returns the certificate in PEM
format. Includes the intermediate certificate if one is stored.

**`show pki certificate <name> bundle pem`** returns the certificate
and its private key concatenated in PEM format (device certificates
only). Useful for clients that need a single PEM file (e.g. OpenConnect).

**`show pki certificate <name> fingerprint [sha256|sha384|sha512]`**
returns the DER fingerprint as colon-separated hex. Defaults to SHA-256.

<!-- source: internal/component/pki/show.go -- handleShowPKICertificates, handleShowPKICertificate -->

### show firewall

Firewall (nftables) introspection. Requires the `firewall { ... }`
section in config so the firewall plugin loads and applies a backend;
without it the handlers reject under exact-or-reject.

```
ze show firewall ruleset <name>         # Rules + per-term counters for table <name>
ze show firewall group                  # List all known group names (applied sets)
ze show firewall group <name>           # Elements of a named group
```

**`show firewall ruleset`** joins the applied desired state (chains +
terms) with kernel counters read back via the nft backend's `GetCounters`
call. Every rule is auto-instrumented with an anonymous counter
expression when applied; the term name is stored in nftables'
`Rule.UserData` and recovered on readback so the join is explicit (not
index-based). Rejects when no firewall backend is loaded or when the
active backend is not `nft`.

**`show firewall group`** reads from the applied-state snapshot, not
the kernel -- groups (nftables named sets) are part of the desired
state the operator typed into config. Calling with no argument returns
`{ name, tables[], members }` per group; a positional name returns the
raw elements.

<!-- source: internal/plugins/firewall/nft/cmd_show.go -- handleShowFirewallRuleset, handleShowFirewallGroup -->
<!-- source: internal/component/firewall/engine.go -- runEngine; OnConfigApply stores LastApplied -->
<!-- source: internal/plugins/firewall/nft/backend_linux.go -- applyChain (UserData + auto Counter), readRuleCounter -->

### show system uptime

```
ze show system uptime        # Daemon start time and uptime duration
```

Returns `start-time` (RFC3339) and `uptime` (truncated to seconds).
Returns an error when the daemon is not running (context is nil or the
reactor is absent).

<!-- source: internal/component/cmd/show/show.go -- handleShowUptime -->

### show system sockets

```
ze show system sockets                          # All TCP and UDP sockets
ze show system sockets tcp                      # TCP only
ze show system sockets tcp state ESTABLISHED    # Filter by state
ze show system sockets tcp port 179             # Filter by port
```

Returns JSON array of sockets with protocol, local-addr, local-port,
remote-addr, remote-port, state, tx-queue, rx-queue. Linux only.

<!-- source: internal/component/cmd/show/sockets_linux.go -- handleShowSystemSockets -->

### show system kernel-log

```
ze show system kernel-log                       # Last 50 entries
ze show system kernel-log count 20              # Last 20 entries
ze show system kernel-log level err             # Errors and above
ze show system kernel-log level err count 10    # Combined
```

Reads /dev/kmsg. Returns entries with level, sequence, timestamp-us, message.
Levels: emerg, alert, crit, err, warning, notice, info, debug. Linux only.

<!-- source: internal/plugins/host-cmd/cmd/show_kernel_log_linux.go -- handleShowSystemKernelLog -->

### show system goroutines

```
ze show system goroutines summary    # Count by state
ze show system goroutines blocked    # Only waiting goroutines
ze show system goroutines full       # Full stack dump
```

The `full` mode uses singleflight deduplication: concurrent requests share
a single 16 MB allocation.

<!-- source: internal/component/cmd/show/goroutines.go -- handleShowSystemGoroutines -->

### show tcp-check

```
ze show tcp-check <host> <port>                    # Basic connectivity test
ze show tcp-check <host> <port> timeout 3s         # Custom timeout (1s-30s)
ze show tcp-check <host> <port> source 10.0.0.1    # Bind source IP
```

Returns result (connected/refused/timeout) and latency-ms.

<!-- source: internal/plugins/diag/cmd/tcp_check.go -- HandleTCPCheck -->

### show traceroute

```
ze show traceroute 8.8.8.8                        # Trace path to target
ze show traceroute 8.8.8.8 max-hops 10            # Limit to 10 hops (1-64)
ze show traceroute 8.8.8.8 timeout 2s             # Per-probe timeout (1s-30s)
ze show traceroute 8.8.8.8 probes 1               # 1 probe per hop (1-10)
ze show traceroute 2001:db8::1                     # IPv6 target
ze show traceroute example.com                     # Hostname (resolved to IP)
```

Returns JSON with target and per-hop array. Each hop has: hop (int), addr
(string or "*" for timeout), rtt-ms (float or null), ttl (int). Requires
CAP_NET_RAW (root privilege enforced at startup).

<!-- source: internal/component/traceroute/cmd/traceroute.go -- handleTraceroute -->

### monitor traceroute

```
monitor traceroute 8.8.8.8                          # Live mtr-style path trace (alt screen)
monitor traceroute 8.8.8.8 max-hops 10              # Limit to 10 hops (1-64, default 16)
monitor traceroute 8.8.8.8 | log                    # Appending scrollback, one line per round
monitor traceroute 8.8.8.8 | log | resolve          # Log with reverse DNS in hop legend
monitor traceroute 8.8.8.8 | log | origin           # Log with ASN/network in hop legend
monitor traceroute 8.8.8.8 | table                  # Alt screen with formatted output
monitor traceroute 8.8.8.8 | json                   # Alt screen with JSON per round
```

Continuous mtr-style traceroute. Plain mode uses the alt screen with columns:
Hop, Address, Loss%, Snt, Last, Avg, Best, Wrst, StDev. Each round is a
complete trace. Esc/q/Ctrl-C to stop; last snapshot copied to scrollback.

In `| log` mode, the hop legend (printed every 25 rounds) is enriched by
`| resolve` (adds reverse DNS hostnames) or `| origin` (adds ASN name
or AS number from Team Cymru).

Requires CAP_NET_RAW.

<!-- source: internal/component/cli/model_traceroute.go -- traceroute monitor model -->

### monitor ping

```
monitor ping 8.8.8.8                                # Live ping (alt screen, 1s interval)
monitor ping 8.8.8.8 interval 500ms                 # Custom interval (100ms-30s)
monitor ping 8.8.8.8 timeout 3s                     # Custom timeout (1s-30s)
monitor ping 8.8.8.8 | log                          # Appending scrollback, one line per reply
monitor ping 8.8.8.8 | table                        # Alt screen with formatted stats
monitor ping 8.8.8.8 | json                         # Alt screen with JSON per reply
```

Continuous ICMP ping. Plain mode uses the alt screen showing: Sent, Recv,
Loss%, Last, Min, Avg, Max, StDev. Esc/q/Ctrl-C to stop.

Default interval: 1s. Default timeout: 5s.

Requires CAP_NET_RAW.

<!-- source: internal/component/cli/model_ping.go -- ping monitor model -->

### show capture interface

```
ze show capture interface eth0                                  # Capture 100 packets, pcap output
ze show capture interface eth0 count 10                         # Capture 10 packets
ze show capture interface eth0 duration 5s                      # Capture for 5 seconds
ze show capture interface eth0 tcp port 179 count 10            # BPF filter: TCP port 179
ze show capture interface eth0 format text                      # Human-readable one-line-per-packet
ze show capture interface eth0 snap-len 128 format text         # Truncate packets to 128 bytes
ze show capture interface eth0 udp port 53 count 5 format text  # DNS traffic, text output
```

Live packet capture using AF_PACKET raw sockets with BPF filters. Replaces
`tcpdump` on gokrazy appliances. Default output is base64-encoded pcap (pipe to
`base64 -d > capture.pcap` for Wireshark). `format text` produces one line per
packet: `TIMESTAMP PROTO SRC:PORT -> DST:PORT FLAGS LEN HEX`. Limits: count
1-10000, duration 1s-60s, snap-len 64-65535. One active capture per interface.
Linux only (requires CAP_NET_RAW). Pure Go, no libpcap/cgo dependency.

<!-- source: internal/plugins/diag/cmd/capture_interface_linux.go -- HandleCaptureInterface -->

### show system file-descriptors

```
ze show system file-descriptors summary    # Counts by type + limits
ze show system file-descriptors detail     # Full FD list with targets
```

Returns total, by-type (socket/pipe/file/anon_inode), soft-limit, hard-limit.
Linux only.

<!-- source: internal/component/cmd/show/fd_linux.go -- handleShowSystemFD -->

### show dns lookup

```
ze show dns lookup example.com                 # A record (default)
ze show dns lookup example.com type AAAA       # AAAA record
ze show dns lookup example.com type MX         # MX record
```

Returns structured JSON with name, type, records, count, query-time-ms.
Supported types: A, AAAA, MX, NS, TXT, CNAME, PTR.

<!-- source: internal/component/resolve/cmd/show_dns.go -- handleDNSLookup -->

### show dns cache stats / list / record

```
ze show dns cache stats               # Cache hit/miss/eviction counters + hit-rate/miss-rate
ze show dns cache list                # List all non-expired cached entries (sorted by TTL ascending)
ze show dns cache record example.com  # Show cached entries for a specific name
```

`stats` returns entries, capacity, hits, misses, evictions, expired, hit-rate, miss-rate.
`list` returns each entry with name, type, records, and ttl-seconds.
`record <name>` filters cached entries by name (all types for that name).

<!-- source: internal/component/resolve/cmd/show_dns.go -- handleDNSCacheStats, handleDNSCacheList, handleDNSCacheRecord -->

### clear dns cache / stats / record

```
clear dns cache                                     # Flush all entries and reset all counters
clear dns cache stats                               # Zero counters without removing entries
clear dns cache record example.com                  # Delete all entries matching name (all types)
clear dns cache record example.com type AAAA        # Delete a single entry by name and type
```

<!-- source: internal/component/resolve/cmd/dns.go -- handleClearDNSCache, handleClearDNSCacheStats, handleClearDNSCacheRecord -->

### clear vpn ipsec sa

```
clear vpn ipsec sa                          # Terminate all IPsec SAs
clear vpn ipsec sa peer <name>              # Terminate SAs for one peer (1-255 chars)
```

Without arguments, terminates every active Security Association and
returns the count. With `peer <name>`, terminates only the named
peer's SAs and returns the peer name. Peers configured with `connection-type initiate` re-establish
automatically; `respond`-only peers wait for the remote side.

Peer name must be 1-255 characters. A missing name after `peer`
returns an error. An unknown peer name returns "peer not found".

<!-- source: internal/component/ike/cmd/ipsec.go -- handleClearIPsecSA -->

### show system profile

```
ze show system profile heap                     # Heap profile
ze show system profile cpu duration 10s         # CPU profile (1s-60s)
ze show system profile goroutine                # Goroutine profile
ze show system profile allocs                   # Allocation profile
```

Returns base64-encoded pprof data. CPU profiling is mutex-protected:
concurrent requests return an error.

<!-- source: internal/component/cmd/show/profile.go -- handleShowSystemProfile -->

### show system memory

```
ze show system memory        # OS process memory (VmRSS/VmSize) from /proc/self/status
```

Returns vm-rss-kb, vm-size-kb, vm-swap-kb, vm-peak-kb, vm-data-kb,
vm-stack-kb, threads. Linux only.

<!-- source: internal/component/cmd/show/memory_map_linux.go -- handleShowSystemMemoryMap -->

### show system update

```
ze show system update           # Firmware update status
ze show system update | json    # Machine-readable output
```

Returns: running-version, remote-version, update-available, status, last-check, last-error,
download-status, download-sha256, staged-version, staged-path, restart, server-paused.

Status values: "up to date", "update available", "downloading", "verifying", "staged",
"paused by server", "waiting for maintenance window", "waiting for spread",
"check failed", "not configured", "error: ...".

<!-- source: internal/plugins/update-cmd/cmd/show.go -- handleShowSystemUpdate -->

### show system update history

```
ze show system update history           # Last 20 update events
ze show system update history | table   # Tabular view
```

Returns an array of events with: timestamp, from (version), to (version), result.
Result values: "success", "failed-download", "failed-checksum", "failed-stage",
"blocked-minimum-version", "paused".

History is persisted to `ze-update-history.json` in the binary's directory and
survives restarts.

<!-- source: internal/plugins/update-cmd/cmd/show.go -- handleShowSystemUpdateHistory -->

### update system firmware

```
ze update system firmware check       # Immediate version check (bypass interval timer)
ze update system firmware download    # Download now (bypass spread, maintenance window)
ze update system firmware apply       # Full cycle: download+verify+stage+restart
ze update system firmware restart     # Restart into staged version now
ze update system firmware rollback    # Restore .prev binary and restart
```

All firmware commands are RPC-only (no config state change). They override the
automated schedule for one-shot operation.

`apply` and `download` bypass server-side pause (pause is for automated fleet
rollout, not manual intervention). Both check minimum-version and warn when
sha256 is absent from the manifest.

`rollback` renames the `.prev` backup to the target binary and restarts. After
rollback, `.prev` no longer exists and the new version is gone from disk.

<!-- source: internal/plugins/update-cmd/cmd/firmware.go -- firmware CLI handlers -->

### show bgp summary

```
ze show bgp summary                  # Every configured peer
ze show bgp summary ipv4             # Expanded to ipv4/unicast
ze show bgp summary ipv6             # Expanded to ipv6/unicast
ze show bgp summary l2vpn            # Expanded to l2vpn/evpn
ze show bgp summary <afi>/<safi>     # Full AFI/SAFI form (e.g. ipv4/vpn)
```

The family argument is validated against the families any peer has
actually negotiated; unknown or un-negotiated families reject with the
sorted set of currently-negotiated families so the operator sees
exactly what is reachable on the running daemon.

<!-- source: internal/component/bgp/plugins/cmd/peer/yang/ze-peer-cmd.yang -- module ze-peer-cmd; internal/component/bgp/plugins/cmd/peer/summary.go -- handleBgpSummary -->

### ping / traceroute

`show ping` and `show traceroute` use Ze's internal ICMP engine and work without
the daemon running (registered as local handlers). `monitor ping` and `monitor
traceroute` also work offline, streaming results until Ctrl-C.

```
ze show ping 8.8.8.8                          # 5 probes, 5s timeout
ze show ping 8.8.8.8 count 10 timeout 3s      # count 1-100, timeout 1s-30s
ze show ping 8.8.8.8 size 1400                # ICMP payload bytes (1-65507)
ze monitor ping 8.8.8.8 interval 500ms        # stream until Ctrl-C (100ms-30s)
ze monitor ping 8.8.8.8 count 5               # stop after 5 probes
ze monitor ping 8.8.8.8 size 1400 count 20    # 20 probes carrying a 1400-byte payload
```

`size` is the ICMP **payload** length, not the total packet: `size 1400` sends
1400 bytes of payload plus the 8-byte ICMP and 20-byte IPv4 headers. This differs
from `ping(8)`, whose familiar "64 bytes" counts the whole ICMP message (56
payload + 8 header). Omit `size` and the engine sends its small default payload.
The 65507 ceiling is what still fits a 65535-byte IP datagram after both headers.

Both commands take `count` (1-100) and `size` (1-65507). They mean the same thing
in each: `show ping` runs a bounded batch and prints a summary, while `monitor
ping` streams live statistics. Omitting `count` on `monitor ping` streams until
Ctrl-C, which is the difference between them; `monitor ping` additionally takes
`interval` (100ms-30s). The two accept identical arguments whether or not a
daemon is running.

<!-- source: internal/component/ping/cmd/register.go -- showPingLocal -->
<!-- source: internal/component/ping/cmd/ping.go -- parsePingArgs -->
<!-- source: internal/plugins/ping-cmd/yang/ze-ping-cmd.yang -- show/ping size leaf -->
<!-- source: internal/component/traceroute/cmd/register.go -- showTracerouteLocal -->

### show crashes

```
ze show crashes              # List crash files with timestamp and size (JSON)
ze show crashes latest       # Display the most recent crash report
ze show crashes name <file>  # Display a specific crash report
```

`show crashes` is served by the daemon when reachable; when no daemon is running
it falls back to reading the crash files in-process. That fallback matters most
here: you inspect a crash precisely when the daemon has died, so the command must
work with no daemon.

Crash reports contain the panic stack trace, ring buffer context (last 64
log entries before the crash), version, build date, and uptime. Crash files
are stored in the autodetected crash directory (see `ze.crash.dir` env var).
<!-- source: internal/plugins/crashes/cmd/register.go -- online show crashes RPC -->
<!-- source: internal/plugins/crashes/register.go -- offline fallback (registry.RegisterOfflineFallback) -->

<!-- source: internal/plugins/crashes/cmd/show.go -- show crashes command -->
<!-- source: internal/plugins/crashes/crashes.go -- crash storage commands -->

### clear interface counters

```
ze clear interface counters                # Reset counters on every managed interface
ze clear interface counters <name>         # Reset counters on one interface
```

Grammar uses action-before-identifier: `counters` keyword first, then the
optional interface name. Bare `ze clear interface counters` (no name) clears
all interfaces. The old forms `clear interface <name> counters` and
`clear interface <name>` are still accepted with a deprecation warning.
Errors in argument shape (unknown trailing keyword, three or more tokens)
reject with the usage line rather than silently defaulting to "all".

The `clear` verb resets runtime/operational state without touching
configuration. Backends that expose a real counter-reset syscall
(VPP's `sw_interface_clear_stats`, once wired) zero the kernel
counters directly. Linux netlink has no generic counter-reset, so ze
falls back to a per-interface baseline: the current raw counter
values are captured, and every subsequent `show interface [counters]`
read subtracts the baseline before returning so the operator sees
"since last clear" deltas.

Wrap detection: if a subsequent read observes a raw counter lower than
its baseline (interface bounce, driver reload, delete+recreate), ze
treats it as a kernel-level reset, drops the baseline, and returns
the raw value. Subsequent reads resume from the kernel's new zero
without underflow.

<!-- source: internal/component/iface/cmd/clear.go -- handleClearInterfaceCounters -->
<!-- source: internal/component/iface/counters.go -- baselineStore, applyBaseline (wrap rebases) -->
<!-- source: internal/component/iface/dispatch.go -- ResetCounters, GetStats/ListInterfaces/GetInterface apply baseline -->

**migrate flags (dispatched to running daemon via SSH):**

| Flag | Purpose |
|------|---------|
| `--from <iface>.<unit>` | Source interface and unit (required) |
| `--to <iface>.<unit>` | Destination interface and unit (required) |
| `--address <cidr>` | IP address to migrate (required) |
| `--create <type>` | Create new interface: dummy, veth, bridge |
| `--timeout <duration>` | BGP readiness timeout (default: 30s) |
<!-- source: internal/component/iface/cli/main.go -- Run; internal/component/iface/cli/show.go -- cmdShow; internal/component/iface/cli/migrate.go -- cmdMigrate -->

### ze exabgp

ExaBGP compatibility tools.

```
ze exabgp plugin <cmd> [args]    # Run ExaBGP plugin with ze
ze exabgp migrate <file>         # Convert ExaBGP config to ze
ze exabgp migrate --env <file>   # Convert ExaBGP env file to ze config
```

**migrate flags:**

| Flag | Purpose |
|------|---------|
| `--dry-run` | Show what would be done without output |
| `--env <file>` | Migrate ExaBGP INI environment file |

**plugin flags:**

| Flag | Purpose |
|------|---------|
| `--family <family>` | Address family (repeatable) |
| `--route-refresh` | Enable route-refresh |
| `--add-path <mode>` | ADD-PATH mode: receive, send, both |

When launched by ze's process manager (as an external plugin), the bridge detects
`ZE_PLUGIN_HUB_TOKEN` and automatically uses TLS connect-back with the SDK.
In standalone mode (no env var), it uses stdin/stdout with inline MuxConn framing.

<!-- source: internal/plugins/exabgp/main.go -- Run, cmdPlugin, cmdMigrate -->
<!-- source: internal/plugins/exabgp/main_sdk.go -- runSDKMode TLS connect-back -->

### ze schema

Schema discovery.

```
ze schema list                   # List registered schemas
ze schema show <module>          # Show YANG module content
ze schema handlers               # List handler-to-module mapping
ze schema methods [module]       # List RPCs from YANG
ze schema events [module]        # List notifications
ze schema protocol               # Show protocol version
```

All subcommands accept `--json`.
<!-- source: internal/component/config/yang/cli/main.go -- Run -->

### ze yang

YANG analysis.

```
ze yang completion               # Detect prefix collisions
ze yang tree                     # Print unified tree
ze yang doc [command]            # Command documentation
```

| Flag | Purpose |
|------|---------|
| `--json` | JSON output |
| `--commands` | Show command tree (tree) |
| `--config` | Show config tree (tree) |
| `--min-prefix <N>` | Minimum prefix length (completion, default: 1) |
| `--list` | List commands (doc) |
<!-- source: internal/component/config/yang/cli/main.go -- Run -->

### ze init

Bootstrap the database (interactive or piped).

```
ze init                          # Interactive setup
ze init -managed                 # Fleet mode
ze init -force                   # Replace existing database
```

Prompts for: username, password, host (127.0.0.1), port (2222), name (hostname).
After credentials are stored, ze init discovers OS network interfaces via netlink
and writes initial interface configuration (ethernet, bridge, veth, dummy, loopback)
to the database as `ze.conf`.
<!-- source: internal/plugins/init/main.go -- Run, runInit, defaultHost, defaultPort -->
<!-- source: internal/component/iface/discover.go -- DiscoverInterfaces -->
<!-- source: internal/component/iface/emit.go -- EmitConfig -->

### ze install

Install ze binary, systemd service, or provision remote devices.

```
ze install local                     # copy binary and create config directory
ze install local --prefix /usr/local # non-interactive prefix selection
ze install local --dry-run           # preview what would be done
sudo ze install systemd              # write and enable ze.service
sudo ze install systemd --start      # install, enable, and start
ze install systemd --dry-run         # print the unit file, no writes
ze install remote --interface eth0 --network 10.0.0.0/24 \
  --image /path/to/gokrazy.img \
  --ssh-username admin --ssh-password secret
```

#### ze install local

| Flag | Purpose |
|------|---------|
| `--prefix` | Installation prefix (default: interactive selection) |
| `--dry-run` | Print what would be done without making changes |

#### ze install systemd

| Flag | Purpose |
|------|---------|
| `--config <dir>` | Override the config directory used in the unit file |
| `--start` | Start the service after enabling it |
| `--force` | Overwrite an existing `/etc/systemd/system/ze.service` |
| `--dry-run` | Print the generated unit file to stdout without root, systemctl, or filesystem writes |

`ze install systemd` requires Linux, `systemctl`, root, and an existing
`<config-dir>/database.zefs`. Run `sudo ze init` first. The generated unit runs
as user/group `ze`, sets `XDG_RUNTIME_DIR=/run/ze`, creates `/run/ze` through
`RuntimeDirectory=ze`, and grants `CAP_NET_ADMIN`, `CAP_NET_RAW`, and
`CAP_NET_BIND_SERVICE` through systemd capabilities.

The daemon socket is `/run/ze/ze.socket` under this unit. Configure
`daemon { socket "/run/ze/ze.socket"; }` or run operator commands with
`XDG_RUNTIME_DIR=/run/ze` so local CLI commands connect to the same socket.

#### ze install remote

| Flag | Purpose |
|------|---------|
| `--interface` | Network interface for provisioning (required) |
| `--network` | Provisioning network CIDR, /8../30 (required) |
| `--image` | Path to gokrazy disk image (required) |
| `--ssh-username` | Admin username for installed target (required) |
| `--ssh-password` | Admin password, bcrypt-hashed before use (required) |
| `--address` | Override server IP (default: first IPv4 on interface, or from `--network` if none) |

Zero-touch provisioning server. Generates a ze config from CLI flags and
forks `ze -` to start DHCP+PXE, TFTP, and HTTP servers for PXE-booting
target machines with a gokrazy image. The DHCP pool range scales with
subnet size. Requires root on Linux.
<!-- source: cmd/ze/install/dispatch.go -- dispatch -->
<!-- source: internal/plugins/local/ -- binary copy -->
<!-- source: internal/plugins/systemd/ -- unit file -->
<!-- source: internal/plugins/provision/ -- PXE provisioning -->
<!-- source: internal/plugins/systemd/ -- shared service runtime -->

### ze uninstall

Remove ze binary or systemd service.

```
sudo ze uninstall systemd            # stop, disable, and remove the unit
sudo ze uninstall systemd --purge    # also remove the ze user and group
ze uninstall local                   # remove binary
ze uninstall local --purge           # also remove config directory and database
```
<!-- source: cmd/ze/uninstall/dispatch.go -- dispatch -->
<!-- source: internal/plugins/local/ -- binary removal -->
<!-- source: internal/plugins/systemd/ -- unit removal -->


### ze passwd

Bcrypt-hash a plaintext password for use in `system.authentication.user.password`.
Reads from stdin (piped, single line) or interactive TTY (prompts twice for
confirmation). Uses `bcrypt.DefaultCost` (10), the same cost as `ze init`.

```
echo "secret" | ze passwd                                # one-shot pipe
ze passwd                                                # interactive
```

The output is suitable for direct paste into a YANG `password` leaf, or as a
shell substitution into `ze config set ... password "$(echo s | ze passwd)"`.
<!-- source: internal/plugins/passwd/main.go -- runImpl -->

### --user / -u flag (all client CLIs)

`ze cli`, `ze bgp plugin cli`, `ze signal`, `ze config set`, `ze config edit`,
and `ze interface migrate` accept `--user <name>` (long) and `-u <name>`
(short) to override the bootstrap super-admin username. Without the flag,
the CLI uses the username stored for the selected zefs SSH target. By default it
follows `meta/ssh/default`, then reads `meta/ssh/<host>/<port>/username`.

| Source | Wins over |
|--------|-----------|
| `--user`/`-u` flag | env, zefs |
| `ze.ssh.username` env var | zefs |
| zefs `meta/ssh/<host>/<port>/username` | (default) |

The password for a non-super-admin user must come from `ze.ssh.password`
(env) or an interactive prompt. There is intentionally no `--password`
flag (passwords in argv leak into shell history and `ps`).

The zefs store is one source among these, not a prerequisite. It is created
`0600` and owned by whoever installed ze, so an operator who cannot read it can
still log in by naming themselves with `--user` and supplying `ze.ssh.password`:
resolution falls back to the built-in `127.0.0.1:2222` target. Set `ze.ssh.host`
and `ze.ssh.port` (or pass `--remote`) if the daemon listens elsewhere, because
the `meta/ssh/default` pointer lives in the store and cannot be read either.
With no username from flag or env and no readable store, the CLI fails and names
`--user` and `ze.ssh.password` rather than guessing an identity.
<!-- source: internal/core/ssh/client/client.go -- readCredentials, openStoreIfReadable -->

See [authentication.md](../authentication/index.md) for the full multi-user workflow.
<!-- source: internal/core/ssh/client/client.go -- ReadCredentialsWithFlags -->
<!-- source: docs/guide/authentication.md -- Logging in as a YANG user -->

### ze start --web

Add the HTTPS web interface alongside the BGP daemon. The web server runs on a
separate port and provides configuration viewing, editing, and admin commands.

```
ze start --web 8443                              # Start daemon + web on port 8443
ze start --web 8443 --insecure-web               # No authentication (forces 127.0.0.1)
ze start --mcp 9718                              # Start daemon + MCP server
ze start --web 8443 --mcp 9718                   # Both web and MCP
```

| Flag | Purpose |
|------|---------|
| `--web <port>` | Start web interface on `0.0.0.0:<port>` (requires config) |
| `--web-only` | Start web UI only, no daemon (config editing only, default port 3443) |
| `--insecure-web` | Disable authentication (forces `127.0.0.1`, requires `--web` or `--web-only`) |
| `--mcp <port>` | Start MCP server on `127.0.0.1:<port>` (AI control interface) |

The web server uses a self-signed ECDSA P-256 certificate (persisted in zefs) with SANs
for localhost, 127.0.0.1, ::1, and the listen address.

See [Web Interface Guide](../web-interface/index.md) for full usage documentation.
<!-- source: cmd/ze/ze_core_start.go -- cmdStart, flagStartWeb, flagStartWebOnly, flagStartInsecureWeb, flagStartMCP -->
<!-- source: cmd/ze/hub/main.go -- RunWebOnly, resolveWebListeners -->

### debug (set / delete / show / clear)

Granular debug edited as persistent state in `debug.zefs`. Verb-first grammar
(`set`/`delete`/`show`/`clear`), matching VyOS syslog-level configuration: enabling
debug for a subsystem is `set`, disabling is `delete`.

```
ze set debug module <name>                       # Enable debug for a subsystem
ze set debug module <name> level <level>         # Set level (debug/info/warn/error)
ze set debug module <name> flag <flag>           # Add a debug flag
ze set debug module <name> scope <kind> <value>  # Add a scope filter (e.g. scope direction receive)
ze delete debug module <name>                    # Disable debug for a subsystem
ze delete debug module <name> flag <flag>        # Remove a flag
ze delete debug module <name> scope <kind> <value>  # Remove a scope filter
ze show debug profile                            # List stored profiles
ze show debug profile name <name>                # Show a stored profile (e.g. name default)
ze show debug profile name <name> module <prefix>  # Filter the table to one subsystem subtree
ze set debug profile name <name>                 # Save current state as a named profile
ze set debug active name <name>                  # Load and apply a named profile
ze delete debug profile name <name>              # Delete a named profile
ze clear debug                                   # Clear the default profile
ze set debug timeout <duration>                  # Auto-disable timer (e.g. 30m, 1h, 90s; 0=off)
```

Hierarchical prefixes work: `ze set debug module bgp` covers all bgp.* subsystems.
Not auto-applied on reboot (safety). Use `ze set debug active name <name>` after restart.
Each plugin declares valid flags via the debug YANG registry; invalid flags are rejected.

`show debug profile name <name>` reads a stored profile (offline). To see the daemon's
actual live state, use `show debug` (YANG-dispatched RPC, requires a running daemon).
<!-- source: internal/plugins/debug/debug.go -- runSetModule, runDeleteModule, applyProfile -->
<!-- source: internal/plugins/debug/register.go -- set/delete/show/clear debug registrations -->
<!-- source: internal/plugins/debug/cmd/handlers.go -- show debug live state RPC -->
<!-- source: internal/component/debug/yang/register.go -- RegisterModule, ValidateFlag -->

### ze pipe

Apply pipe formatting to stdin. Offline commands (like `ze show debug profile`) do not pass
through the YANG-dispatched pipe infrastructure. `ze pipe` provides the same
operators as a standalone filter.

```
<command> | ze pipe json [compact]     # Format as JSON (pretty or compact)
<command> | ze pipe yaml               # YAML output
<command> | ze pipe table              # Box-drawing table
<command> | ze pipe text               # Space-aligned columns
<command> | ze pipe ndjson             # One compact JSON object per line
<command> | ze pipe match <pattern>    # Grep lines (case-insensitive)
<command> | ze pipe count              # Count items (JSON-aware)
<command> | ze pipe first <n>          # Take first N items
<command> | ze pipe last <n>           # Take last N items
<command> | ze pipe resolve            # Add reverse DNS for IP values
```

Format operators (json, yaml, table, text, ndjson) expect JSON input. Filter
operators (match, count, first, last) work on both JSON and plain text.
`resolve` adds a `<key>-name` sibling field for each IP address value in JSON.
<!-- source: cmd/ze/ze_core_pipe.go -- runPipe -->

### ze data

Low-level blob store management.

```
ze data import <file>...           # Import files into blob
ze data rm <key>...                # Remove entries
ze data ls [prefix]                # List entries
ze data cat <key>                  # Print entry content
ze data registered                 # List all registered key patterns
ze data registered <pattern>       # Show details for a key pattern
```

| Flag | Purpose |
|------|---------|
| `--path <store>` | Blob store path |
<!-- source: internal/component/config/storage/cli/main.go -- Run, subcommandHandlers -->

### ze plugin

Plugin management.

```
ze plugin <name> [args]          # Run plugin CLI handler
ze plugin test                   # Test plugin schema/config
```
<!-- source: internal/component/plugin/cli/main.go -- Run -->

### ze completion

Generate shell completion scripts for bash, zsh, fish, and nushell. The scripts provide tab completion for subcommands, flags, plugin names, YANG schema modules, `show`/`run` command trees, and argument values (address families, log levels).

```
ze completion bash
ze completion zsh
ze completion fish
ze completion nushell
```

#### Installation

| Shell | Quick (current session) | Persistent |
|-------|------------------------|------------|
| Bash | `eval "$(ze completion bash)"` | `ze completion bash > /etc/bash_completion.d/ze` |
| Zsh | `eval "$(ze completion zsh)"` | `ze completion zsh > ~/.zsh/completions/_ze && autoload -Uz compinit && compinit` |
| Fish | `ze completion fish \| source` | `ze completion fish > ~/.config/fish/completions/ze.fish` |
| Nushell | `ze completion nushell \| save -f ($nu.default-config-dir \| path join "completions" "ze.nu")` | Add `source completions/ze.nu` to `config.nu` |

Shell completion is registered by command name (`ze`), not by path. If using a local build (`./bin/ze`), ensure the binary is reachable as `ze` in your PATH for completion to activate.
<!-- source: internal/plugins/completion/main.go -- Run -->

### ze env

Environment variable management.

```
ze env registered                # List all registered env vars + log subsystems
ze env registered <key>          # Show details for a specific env var
ze env list -v                   # List with current effective values
ze env get <key>                 # Show single env var details
ze env get ze.log.bgp.reactor    # Inspect a concrete log-subsystem key
```

| Flag | Purpose |
|------|---------|
| `-v`, `--verbose` | Show current effective values (list) |

Shell completion offers env keys after `ze env get` and `ze env registered` via `ze completion words env`. Concrete `ze.log.<subsystem>` keys that appear in `ze env list` are completable and inspectable.
<!-- source: internal/plugins/env/env.go -- Run -->
<!-- source: internal/core/envcatalog/catalog.go -- VisibleEntries -->

### ze resolve

Query DNS, Team Cymru, PeeringDB, and IRR resolution services. Offline tool -- no running daemon required.

```
ze resolve dns a example.com                           # IPv4 address records
ze resolve dns aaaa example.com                        # IPv6 address records
ze resolve dns txt example.com                         # TXT records
ze resolve dns ptr 8.8.8.8                             # Reverse DNS
ze resolve cymru asn-name 13335                        # ASN to org name
ze resolve peeringdb max-prefix 13335                  # IPv4/IPv6 prefix counts
ze resolve peeringdb as-set 13335                      # Registered IRR AS-SETs
ze resolve irr as-set AS-CLOUDFLARE                    # Expand AS-SET to member ASNs
ze resolve irr prefix AS-CLOUDFLARE                    # Lookup announced prefixes
```

| Flag | Subcommand | Purpose |
|------|------------|---------|
| `--server <host>` | dns, irr | Override DNS/whois server |
| `--dns-server <host>` | cymru | Override DNS server for TXT queries |
| `--url <url>` | peeringdb | Override PeeringDB API base URL |
<!-- source: internal/component/resolve/cli/main.go -- Run -->

### ze-perf

BGP propagation latency benchmark tool. Separate binary from `ze`.

<!-- source: internal/perf/cli/register.go -- ze-perf CLI entry point -->

```
ze-perf <command> [flags]
```

| Command | Purpose |
|---------|---------|
| `run` | Run benchmark against a BGP DUT |
| `report` | Generate comparison report from result files |
| `track` | Track performance history and detect regressions |

#### ze-perf run

Run a BGP propagation benchmark against a device under test (DUT). Establishes
sender and receiver sessions with the DUT, injects routes from the sender, and
measures how quickly they propagate through to the receiver.

<!-- source: internal/perf/cli/cmd_run.go -- run subcommand -->

```
ze-perf run --dut-addr 172.31.0.2 --dut-asn 65000
ze-perf run --dut-addr 172.31.0.5 --dut-asn 65000 --dut-name gobgp --routes 10000 --json
ze-perf run --dut-addr 172.31.0.2 --dut-asn 65000 --family ipv6/unicast
ze-perf run --dut-addr 172.31.0.2 --dut-asn 65000 --force-mp --repeat 10
```

**DUT flags:**

| Flag | Type | Default | Purpose |
|------|------|---------|---------|
| `--dut-addr` | string | (required) | DUT BGP address |
| `--dut-port` | int | 179 | DUT BGP port |
| `--dut-asn` | int | (required) | DUT autonomous system number |
| `--dut-name` | string | `unknown` | DUT implementation name (appears in results) |
| `--dut-version` | string | | DUT version string |

**Sender/receiver flags:**

| Flag | Type | Default | Purpose |
|------|------|---------|---------|
| `--sender-addr` | string | `127.0.0.1` | Sender local address |
| `--sender-asn` | int | `65001` | Sender autonomous system number |
| `--sender-port` | int | `0` | DUT port for sender (0 = use `--dut-port`) |
| `--receiver-addr` | string | `127.0.0.2` | Receiver local address |
| `--receiver-asn` | int | `65002` | Receiver autonomous system number |
| `--receiver-port` | int | `0` | DUT port for receiver (0 = use `--dut-port`) |

**Benchmark flags:**

| Flag | Type | Default | Purpose |
|------|------|---------|---------|
| `--routes` | int | `1000` | Number of routes to inject |
| `--family` | string | `ipv4/unicast` | Address family (`ipv4/unicast` or `ipv6/unicast`) |
| `--force-mp` | bool | `false` | Force MP_REACH_NLRI for IPv4 unicast |
| `--seed` | uint64 | `0` | Deterministic seed (0 = random) |
| `--warmup` | duration | `2s` | Warmup delay after session establishment |
| `--connect-timeout` | duration | `10s` | TCP connection timeout |
| `--duration` | duration | `60s` | Maximum time to wait for convergence per iteration |

**Iteration flags:**

| Flag | Type | Default | Purpose |
|------|------|---------|---------|
| `--repeat` | int | `5` | Number of benchmark iterations |
| `--warmup-runs` | int | `1` | Warmup iterations (discarded from results) |
| `--iter-delay` | duration | `3s` | Delay between iterations |
| `--batch-size` | int | `0` | UPDATE batch size (0 = single UPDATE per prefix) |

**Output flags:**

| Flag | Type | Default | Purpose |
|------|------|---------|---------|
| `--json` | bool | `false` | JSON output |
| `--output` | string | | Output file path (implies `--json`) |

Exit codes: 0 = success, 1 = error (missing flags, validation failure, benchmark failure).

#### ze-perf report

Generate a comparison report from one or more result JSON files.

<!-- source: internal/perf/cli/cmd_report.go -- report subcommand -->

```
ze-perf report result-ze.json result-gobgp.json
ze-perf report --html result-ze.json result-gobgp.json > report.html
```

| Flag | Type | Default | Purpose |
|------|------|---------|---------|
| `--md` | bool | `true` | Markdown output |
| `--html` | bool | `false` | HTML output (overrides `--md`) |

Reads result JSON files produced by `ze-perf run --json` and generates a
side-by-side comparison table.

#### ze-perf track

Track performance history and detect regressions from an NDJSON file.

<!-- source: internal/perf/cli/cmd_track.go -- track subcommand -->

```
ze-perf track history.ndjson
ze-perf track --check history.ndjson
ze-perf track --html history.ndjson > trend.html
ze-perf track --check --threshold-convergence 15 history.ndjson
```

| Flag | Type | Default | Purpose |
|------|------|---------|---------|
| `--md` | bool | `true` | Markdown output |
| `--html` | bool | `false` | HTML output (overrides `--md`) |
| `--check` | bool | `false` | Check for regressions (exit 1 on regression) |
| `--last` | int | `0` | Only consider last N entries (0 = all) |
| `--threshold-convergence` | int | `20` | Convergence regression threshold (%) |
| `--threshold-throughput` | int | `20` | Throughput regression threshold (%) |
| `--threshold-p99` | int | `30` | P99 latency regression threshold (%) |

<!-- source: internal/perf/regression.go -- regression detection thresholds -->

Exit codes: 0 = no regression (or report mode), 1 = regression detected or error.

---

## Runtime Commands

Commands sent to the running daemon. Access through three entry points:

| Entry | Access | Usage |
|-------|--------|-------|
| `ze cli` | Full (interactive) | Exploration, monitoring |
| `ze show <cmd>` | Read-only | Scripting, dashboards |

**Note:** Some `ze show` subcommands run locally without a daemon (version,
bgp decode/encode, env, schema, yang, completion). These are dispatched
via local handlers before attempting SSH connection.

`ze cli` accepts `-c <command>` for single-shot execution and
`--format <format>` (default: yaml).
<!-- source: internal/component/cli/client/main.go -- Run -->

### Peer Selector

Many commands take a `peer <selector>` argument:

| Selector | Example | Description |
|----------|---------|-------------|
| `*` | `peer *` | All peers |
| Name | `peer upstream1` | By configured peer name |
| IP address | `peer 10.0.0.1` | By peer IP |
| ASN | `peer as65001` | By remote ASN, case-insensitive (matches all peers with that ASN) |
| Glob | `peer 192.168.*.*` | Pattern match |
| Exclusion | `peer !10.0.0.1` | All except this peer |
| ASN exclusion | `peer !as65001` | All except peers with this ASN |
<!-- source: internal/component/bgp/reactor/reactor_api.go -- getMatchingPeersSel; internal/component/bgp/plugins/cmd/peer/peer.go -- peer command handler -->

### Peer Commands

| Command | Access | Purpose |
|---------|--------|---------|
| `show bgp peer list` | read-only | List all peers (IP, ASN, state, uptime) |
| `show bgp peer <sel> detail` | read-only | Detailed peer info (config, state, counters, `prefix-updated` date, `prefix-stale` warning) |
| `show bgp peer <sel> capabilities` | read-only | Negotiated capabilities |
| `show bgp peer <sel> statistics` | read-only | Per-peer update statistics with rates |
| `show bgp peer <sel> history` | read-only | FSM transition history |
| `show bgp summary` | read-only | BGP summary table (all peers) |
| `show bgp summary <afi/safi>` | read-only | Per-family summary: filter to peers that negotiated this AFI/SAFI. Shorthands `ipv4`, `ipv6`, `l2vpn` expand to `ipv4/unicast`, `ipv6/unicast`, `l2vpn/evpn`. Unknown or un-negotiated families reject with the list of families currently negotiated on this daemon. Response adds `family` + `peers-in-family`; `peers-established` is the filtered count |
| `request peer <sel> pause` | write | Pause read loop (flow control) |
| `request peer <sel> resume` | write | Resume read loop |
| `request peer <sel> teardown [<code>] [<msg>]` | write | Graceful close with NOTIFICATION |
| `request peer <sel> flush` | write | Block until all queued updates for peer are on the wire |
<!-- source: internal/component/bgp/plugins/cmd/peer/peer.go -- peer command handlers; internal/component/bgp/plugins/cmd/peer/yang/ze-peer-cmd.yang -->

### Policy Test (Dry-Run)

| Command | Access | Purpose |
|---------|--------|---------|
| `show policy test peer <sel> export update <HEX>` | read-only | Dry-run the peer's configured export chain against a BGP UPDATE |
| `show policy test peer <sel> import update <HEX>` | read-only | Dry-run the peer's configured import chain against a BGP UPDATE |
| `show policy test peer <sel> export filter <NAME> update <HEX>` | read-only | Dry-run a single named filter against a BGP UPDATE |

The peer selector comes first (`peer <sel>`, matching `show bgp peer <sel> ...`), then the direction (`import`/`export`), then optional `filter <NAME>`, then `update <HEX>`.

`<HEX>` is a hex-encoded full BGP UPDATE message (including the 19-byte header). The `0x` prefix is optional.

Optional: `source-asn4 false` to test with ASN2 encoding context (default: ASN4). This is what makes AS4_PATH (RFC 6793) the active path carrier.

Output is structured JSON with fields: `direction`, `peer`, `action` (accept/reject/modify), `trace` (per-filter decisions), `text-before`, `text-after`, `changed-attrs`, and `wire-changes` (wire-level attribute ops such as `AS4_PATH suppressed` that the flat filter text cannot express).

This command does not forward routes, update the RIB, populate cache, or mutate peer state.
<!-- source: internal/component/bgp/plugins/cmd/policy/handler.go -- handleShowPolicyTest -->

### Set Commands

| Command | Access | Purpose |
|---------|--------|---------|
| `set system file-descriptors <N\|max>` | write | Raise process FD soft limit (Linux only; `max` sets to hard limit) |

#### Peer Config Keys

Config keys are parsed from the YANG `peer-fields` schema via `ParseInlineArgs`. Container prefixes (`remote`, `local`) scope sub-keys. The parser walks the YANG tree to determine how many tokens each field consumes (leaf = name + value, container = name + recurse into children).

| Key | Value | Required | Description |
|-----|-------|----------|-------------|
| `remote ip` | IP address | Yes | Peer remote IP address |
| `remote as` | ASN (uint32) | Yes | Peer AS number |
| `local as` | ASN (uint32) | No | Local AS override |
| `local ip` | IP address | No | Local IP for this session |
| `router-id` | IPv4 address | No | Router ID override |
| `timer hold-time` | seconds (0-86400) | No | Hold time (default: 90) |
| `timer connect-retry` | seconds | No | Connect retry interval (default: 120) |
| `remote connect` | true/false | No | Initiate outbound connections (default: true) |
| `local accept` | true/false | No | Accept inbound connections (default: true) |
| `description` | text | No | Peer description |
| `link-local` | IPv6 address | No | Link-local next-hop |
| `port` | 1-65535 | No | Per-peer listen port |
| `group-updates` | enable/disable | No | UPDATE grouping |

<!-- source: internal/component/config/setparser_inline.go -- ParseInlineArgs YANG-driven parser -->
<!-- source: internal/component/config/setparser.go -- parseSet structural-only commands -->
<!-- source: internal/component/bgp/yang/ze-bgp-conf.yang -- grouping peer-fields, the source of every key in this table -->
<!-- source: internal/component/plugin/types_bgp.go -- AddDynamicPeer, which takes the parsed peer-fields tree -->

### Del Commands

| Command | Access | Purpose |
|---------|--------|---------|
| `delete bgp peer <sel>` | write | Remove peer |
<!-- source: internal/component/bgp/plugins/cmd/peer/peer.go -- del peer handler -->

### Update Commands

| Command | Access | Purpose |
|---------|--------|---------|
<!-- source: internal/component/cmd/update/update.go -- update verb RPC registration; internal/component/cmd/update/yang/ze-cli-update-cmd.yang -->

### Route Injection

```
peer <sel> update text <attrs> nlri <family> <op> <prefixes>
peer <sel> update hex <hex-data>
peer <sel> update b64 <b64-data>
peer <sel> raw [<type>] <encoding> <data>
```

Text format attributes:

| Attribute | Syntax |
|-----------|--------|
| `origin` | `origin set igp` / `egp` / `incomplete` |
| `nhop` | `nhop set 192.168.1.1` or `nhop set self` |
| `med` | `med set 100` |
| `local-preference` | `local-preference set 200` |
| `as-path` | `as-path set [ 65001 65002 ]` |
| `community` | `community set [ 65000:100 no-export ]` |
| `large-community` | `large-community set [ 65000:1:1 ]` |
| `extended-community` | `extended-community set [ rt:65000:100 ]` |

NLRI operations: `nlri <family> add <prefixes>`, `nlri <family> del <prefixes>`,
`nlri <family> eor`.
<!-- source: internal/component/bgp/plugins/cmd/update/ -- update text/hex/b64 parsing; internal/component/bgp/plugins/cmd/raw/ -- raw message injection -->

### RIB Commands

| Command | Access | Purpose |
|---------|--------|---------|
| `show bgp rib status` | read-only | RIB summary (peer count, routes, families) |
| `show bgp rib` | read-only | Stream Adj-RIB-In and Adj-RIB-Out routes |
| `show bgp rib \| received` | read-only | Stream received routes only |
| `show bgp rib \| advertised` | read-only | Stream advertised routes only |
| `show bgp rib \| peer <selector>` | read-only | Stream routes for one peer selector |
| `show bgp rib best` | read-only | Best-path per prefix |
| `show bgp rib best status` | read-only | Best-path computation status |
| `clear bgp rib in <selector>` | write | Clear Adj-RIB-In (`*` for all peers) |
| `clear bgp rib out <selector> [family]` | write | Regenerate and re-advertise Adj-RIB-Out (`*` for all peers, optional family filter) |
| `request bgp rib inject <peer> <family> <prefix> [attrs...]` | write | Insert route into Adj-RIB-In as if received from peer |
| `request bgp rib withdraw <peer> <family> <prefix>` | write | Remove route from Adj-RIB-In |
| `show bgp rib rpf <family> <source-addr>` | read | RPF lookup: longest-prefix-match against Loc-RIB for CIDR families |
<!-- source: internal/component/bgp/plugins/cmd/rib/ -- RIB proxy RPCs; internal/component/bgp/plugins/rib/ -- RIB plugin -->

### IRR Filter Commands

| Command | Access | Purpose |
|---------|--------|---------|
| `show bgp irr` | read-only | Per-ASN IRR filter status: AS-SET, prefix count, server, last/next refresh |
| `show bgp irr prefix <peer>` | read-only | List all IRR-resolved prefixes for a peer |
| `show bgp irr check <peer> <prefix>` | read-only | Test whether a prefix would be accepted by the IRR filter |
| `update bgp irr all` | write | Refresh all IRR prefix-lists immediately |
| `update bgp irr asn <asn>` | write | Refresh IRR prefix-list for a specific ASN |
| `update bgp irr as-set <as-set>` | write | Refresh IRR prefix-list for a specific AS-SET |
<!-- source: internal/component/bgp/plugins/filter_irr/command.go -- handleCommand, showIRR, showIRRPrefix, showIRRCheck, updateASN, updateASSet -->
| `update bgp peer <sel> prefix` | write | Refresh max-prefix limits from PeeringDB (saves to draft; run `config commit` to apply) |
<!-- source: internal/component/bgp/plugins/cmd/peer/prefix_update.go -- handleBgpPeerPrefixUpdate -->

### Healthcheck Commands

| Command | Access | Purpose |
|---------|--------|---------|
| `show bgp healthcheck` | read-only | JSON summary of all healthcheck probes |
| `show bgp healthcheck <name>` | read-only | Detailed status of a single probe |
| `clear bgp healthcheck <name>` | write | Withdraw route, reset FSM to INIT, immediate re-check. Error if DISABLED. |
<!-- source: internal/component/bgp/plugins/healthcheck/healthcheck.go -- handleCommand -->

### BMP (RFC 7854)

| Command | Access | Purpose |
|---------|--------|---------|
| `show bmp sessions` | read-only | Show active BMP receiver sessions (router address, sysName, uptime) |
| `show bmp peers` | read-only | Show monitored BGP peers (AS, BGP ID, up/down status) |
| `show bmp collectors` | read-only | Show BMP sender collector connection status |
| `show bmp rib` | read-only | Show BMP-monitored routes |
<!-- source: internal/component/bgp/plugins/bmp/cmd_show.go -- ForwardToPlugin proxy -->

### Commit (Atomic Updates)

| Command | Access | Purpose |
|---------|--------|---------|
| `request commit start <name>` | write | Begin named update window |
| `request commit end <name>` | write | Flush queued updates |
| `request commit eor <name>` | write | Flush updates and send End-of-RIB |
| `request commit show <name>` | read-only | Show queue status |
| `request commit rollback <name>` | write | Discard queued updates |
| `request commit withdraw <name> route <prefix>` | write | Withdraw prefix from window |
| `request commit list` | read-only | List active commits |

Commit names must not collide with action keywords (`list`, `start`, `end`,
`eor`, `rollback`, `show`, `withdraw`). The old grammar `commit <name> <action>`
is accepted with a deprecation warning but does not work when the name equals
a keyword.
<!-- source: internal/component/bgp/plugins/cmd/commit/ -- commit command RPCs -->

### Cache Commands

| Command | Access | Purpose |
|---------|--------|---------|
| `show cache` | read-only | List cached message IDs |
| `request cache retain <id>` | write | Pin in cache (prevent eviction) |
| `request cache release <id>` | write | Release from cache |
| `request cache expire <id>` | write | Remove immediately |
| `request cache forward <id> <peer-sel>` | write | Re-inject UPDATE to peer(s) |

Batch operations: `request cache forward <id1>,<id2> <selector>`.
<!-- source: internal/component/bgp/plugins/cmd/cache/yang/ze-cli-cache-cmd.yang -- module ze-cli-cache-cmd -->

### Static Routes

| Command | Access | Purpose |
|---------|--------|---------|
| `show static` | read-only | Show all configured static routes in JSON: prefix, action, next-hops with address/weight/BFD status, metric, tag |

The static route plugin programs routes directly to the kernel (netlink multipath) or VPP. It auto-loads when the config contains a `static { }` section. See [Static Routes Guide](../static-routes/index.md).
<!-- source: internal/plugins/static/cmd_show.go -- ForwardToPlugin proxy -->

### Sysctl (Kernel Tunables)

| Command | Access | Purpose |
|---------|--------|---------|
| `show sysctl` | read-only | Show all active sysctl keys with value, source (config/transient/default), and persistence |
| `show sysctl keys` | read-only | List all known sysctl keys with descriptions and types |
| `show sysctl key <key>` | read-only | Show detail for one key: description, type, range, current value, source |
| `set sysctl <key> <value>` | write | Set a transient sysctl value (overrides defaults, blocked by config) |
| `show sysctl profiles` | read-only | List all registered sysctl profiles (built-in and user-defined) with key counts |
| `show sysctl profile <name>` | read-only | Show detail for one profile: description, all key/value pairs |

The sysctl plugin manages kernel tunables with three-layer precedence: config (persistent, from YANG) wins over transient (CLI `set sysctl`), which wins over defaults (plugin-declared via EventBus). Original values are restored on clean daemon stop.

Config example: `sysctl { setting net.ipv4.conf.all.forwarding { value 1; } }`

Named profiles group co-dependent tunables for common use cases. Apply them per interface unit: `sysctl-profile [ dsr hardened ]`. Built-in profiles: `dsr`, `router`, `hardened`, `multihomed`, `proxy`. User-defined profiles declared in `sysctl { profile <name> { ... } }`.

When fib-kernel is loaded, it automatically enables IPv4 and IPv6 forwarding as defaults.
<!-- source: internal/component/sysctl/register.go -- OnExecuteCommand, CommandDecl -->
<!-- source: internal/core/sysctl/profiles.go -- ProfileDef, builtinProfiles -->

### L2TPv2 (Operational)

| Command | Access | Purpose |
|---------|--------|---------|
| `show l2tp` | run | L2TP subsystem summary (tunnel/session counts) |
| `show l2tp tunnels` | run | List all active L2TP tunnels |
| `show l2tp tunnel <tid>` | run | Show one tunnel by local tunnel ID |
| `show l2tp sessions` | run | List all active L2TP sessions |
| `show l2tp session <sid>` | run | Show one session by local session ID |
| `show l2tp statistics` | run | Protocol counters |
| `show l2tp listeners` | run | Bound UDP listener endpoints |
| `show l2tp config` | run | Effective runtime configuration |
| `show l2tp observer <sid>` | run | Per-session event ring snapshot (timestamps, types, RTT, reasons) |
| `show l2tp observer all` | run | Summary of all active session event rings |
| `show l2tp cqm <login>` | run | Per-login CQM bucket history (100s echo RTT/loss aggregates) |
| `show l2tp cqm summary` | run | Aggregate CQM state across all tracked logins |
| `show l2tp echo <login>` | run | Current echo state for a login (RTT, loss ratio, interval) |
| `show l2tp reliable <tid>` | run | Reliable transport window state (Ns, Nr, cwnd, retransmits) |
| `clear l2tp tunnel id <tid>` | run | Send StopCCN for one tunnel |
| `clear l2tp tunnel all` | run | Send StopCCN for every tunnel |
| `clear l2tp session id <sid> [reason <text...>] [cause <code>]` | run | Send CDN for one session with optional audit reason and disconnect cause |
| `clear l2tp session all` | run | Send CDN for every session |

The `clear l2tp session id` command accepts optional keyword arguments:
- `reason <text...>`: free-text audit reason, recorded in the per-session event ring
- `cause <code>`: RADIUS Disconnect-Cause value (uint16), recorded alongside the reason

<!-- source: internal/component/l2tp/cmd/yang/ze-l2tp-cmd.yang -->
<!-- source: internal/component/l2tp/cmd/l2tp.go -- handleSessionTeardown, parseKeywordArgs -->

### PPPoE Commands

| Command | Mode | Description |
|---------|------|-------------|
| `show pppoe` | run | PPPoE subsystem summary (session/interface counts) |
| `show pppoe sessions` | run | List all active PPPoE sessions |
| `show pppoe session <sid>` | run | Show one session by session ID |
| `show pppoe statistics` | run | Per-interface session counts and limits |
| `show pppoe interfaces` | run | Configured PPPoE access interfaces |

<!-- source: internal/component/l2tp/pppoe/cmd/yang/ze-pppoe-cmd.yang -->
<!-- source: internal/component/l2tp/pppoe/cmd/pppoe.go -- RPC handlers -->

### L2TPv2 Web UI

The web interface at `/l2tp` provides session management and CQM graphing.

| URL | Method | Purpose |
|-----|--------|---------|
| `/l2tp` | GET | Session list with sortable columns |
| `/l2tp/<sid>` | GET | Session detail: state, PPP options, CQM chart, event timeline, disconnect |
| `/l2tp/<login>/samples` | GET | CQM buckets as columnar JSON (uPlot data shape) |
| `/l2tp/<login>/samples.csv` | GET | CQM buckets as CSV download |
| `/l2tp/<login>/samples/stream` | GET | SSE stream pushing new CQM buckets every 100s |
| `/l2tp/<sid>/disconnect` | POST | Disconnect session (requires `reason` form field; optional `cause`) |

Disconnect is gated by authz: the `clear` prefix is denied in the built-in read-only profile.
The CQM chart uses vendored uPlot rendered client-side with CSS color variables
(`--color-l2tp-established`, `--color-l2tp-negotiating`, `--color-l2tp-down`).

<!-- source: internal/component/web/handler_l2tp.go -->

### L2TPv2 (Offline Tools)

| Command | Access | Purpose |
|---------|--------|---------|
| `l2tp decode [--pretty]` | offline | Decode a hex L2TPv2 control message on stdin and emit JSON on stdout |

`ze l2tp decode` runs without a daemon. Input is ASCII hex (whitespace
allowed); output is a JSON object with a parsed `header` and an `avps`
array. Each AVP carries its vendor, numeric type, RFC 2661 catalog name
(when vendor 0), flag booleans, and the raw value as lowercase hex. Use
`--pretty` for indented output.

Example:

```
echo c8020044... | ze l2tp decode --pretty
```

Exit code is 0 on successful parse, 1 on invalid hex, truncated header, or
malformed AVPs; stderr carries the reason.
<!-- source: internal/component/l2tp/cli/decode.go -- cmdDecode, avpName -->

### Event Monitoring

```
monitor event [include|exclude <types>] [peer <sel>] [direction <dir>]
```

| Filter | Values |
|--------|--------|
| `peer` | IP address, `*` |
| `include` | Comma-separated event types to include: update, open, notification, keepalive, refresh, state, negotiated |
| `exclude` | Comma-separated event types to exclude: update, open, notification, keepalive, refresh, state, negotiated |
| `direction` | sent, received |

The `include` and `exclude` filters are mutually exclusive.

Streaming command: use in interactive `ze cli` or via SSH. `monitor bgp` (no `event`) is a separate command: the live peer dashboard, documented in the [Monitoring guide](../monitoring/index.md).
<!-- source: internal/plugins/meta/yang/ze-command-monitor-cmd.yang -- module ze-command-monitor-cmd; internal/component/bgp/plugins/cmd/monitor/yang/ze-monitor-cmd.yang -- module ze-monitor-cmd; internal/component/plugin/server/event_monitor.go -- ParseEventMonitorArgs -->

### Netlink Monitoring

```
monitor system netlink [route|link|address|all]
```

Stream kernel netlink events as one JSON line per event. Replaces `ip monitor` on gokrazy appliances. Linux only.

| Group | Events |
|-------|--------|
| `route` | Route add/delete with prefix, gateway, table, protocol |
| `link` | Interface up/down/create/delete with name, state, MTU, MAC |
| `address` | Address add/remove with CIDR, interface |
| `all` | All of the above (default when no group specified) |

Streaming command: use in interactive `ze cli` or via SSH. Press Esc to stop.
<!-- source: internal/component/iface/cmd/monitor_netlink_linux.go -- streamNetlinkMonitor -->

### Interface Rate Monitoring

```
monitor interface rate [<name>]
```

Stream per-second interface rate data as JSON lines (one line per tick, 1s interval). Without a name, streams all interfaces sorted by name. With a name, streams only that interface.

Each JSON line contains: `name`, `rx-bps`, `tx-bps`, `rx-pps`, `tx-pps`, and the raw kernel `stats` snapshot (8 counters). Rate values are computed from raw kernel counter deltas; counter wraps produce 0 rather than negative spikes.

Streaming command: use in interactive `ze cli` or via SSH. Press Esc to stop.
<!-- source: internal/component/iface/cmd/interface_rate.go -- streamInterfaceRate -->

### Metrics

| Command | Access | Purpose |
|---------|--------|---------|
| `show metrics values` | read-only | Prometheus text format metrics |
| `show metrics list` | read-only | List metric names |
| `show metrics pool` | read-only | Per-attribute-pool occupancy, dedup rates, and aggregate totals (13 BGP pools) |
<!-- source: internal/component/cmd/metrics/ -- show metrics values/list/pool RPCs -->

### Logging

| Command | Access | Purpose |
|---------|--------|---------|
| `show log levels` | read-only | List subsystems with current log levels |
| `show log recent` | read-only | Show recent log entries from in-memory ring |
| `request log level <subsystem> <level>` | write | Set log level at runtime |

Levels: debug, info, warn, err, disabled.
<!-- source: internal/plugins/log/cmd/handlers.go -- show log levels/recent, request log level RPCs; internal/core/slogutil/slogutil.go -- level definitions -->

### Debug (live state)

| Command | Access | Purpose |
|---------|--------|---------|
| `show debug` | read-only | Show live debug state from the running daemon (levels, flags, scopes) |

Unlike `show debug profile name <name>` (offline, reads a stored profile from debug.zefs),
`show debug` queries the daemon's actual runtime slogutil state. Use this to verify what is
active after enabling debug or applying a profile.
<!-- source: internal/plugins/debug/cmd/handlers.go -- show debug RPC -->

### Plugin Configuration (from plugin context)

| Command | Access | Purpose |
|---------|--------|---------|
| `bgp plugin encoding <json\|text>` | write | Set event encoding |
| `bgp plugin format <hex\|base64\|parsed\|full>` | write | Set wire format display |
| `bgp plugin ack <sync\|async>` | write | Set ACK timing |
<!-- source: internal/component/cmd/subscribe/ -- subscribe/unsubscribe RPCs -->

### Discovery

| Command | Access | Purpose |
|---------|--------|---------|
| `help` | read-only | List available subcommands |
| `show command list` | read-only | List all commands with descriptions |
| `show command help <name>` | read-only | Detailed help for a command |
| `show event list` | read-only | List available event types |
<!-- source: internal/plugins/meta/yang/ze-command-meta-cmd.yang -- module ze-command-meta-cmd -->

---

## Interactive CLI Features

Inside `ze cli`:

| Feature | Syntax |
|---------|--------|
| Pipe: filter lines | `show bgp peer list \| match established` |
| Pipe: count | `show bgp peer list \| count` |
| Pipe: table format | `show bgp rib \| table` |
| Pipe: text format | `show bgp peer list \| text` |
| Pipe: JSON pretty | `show bgp peer list \| json` |
| Pipe: JSON compact | `show bgp peer list \| json compact` |
| Pipe: NDJSON | `show bgp peer list \| ndjson` |
| Pipe: YAML | `show bgp peer list \| yaml` |
| Pipe: reverse DNS | `show traceroute 8.8.8.8 \| resolve` |
| Pipe: ASN lookup | `show traceroute 8.8.8.8 \| origin` |
| Pipe: streaming log | `monitor traceroute 8.8.8.8 \| log` |
| Pipe: first N items | `show bgp rib \| first 100` |
| Pipe: last N items | `show bgp rib \| last 10` |
| Pipe: disable paging | `show bgp peer list \| no-more` |
| Set default format | `set cli format json` (session override) |
| Show current format | `set cli format` (no argument) |
| Tab completion | Contextual command/argument completion |
<!-- source: internal/component/cli/client/main.go -- pipe operators, interactive model -->
<!-- source: internal/component/command/pipe.go -- pipe operator definitions -->
<!-- source: internal/component/cli/model_keys.go -- handleSetCLIFormat -->

---

## Signal Handling

The daemon handles these Unix signals directly:

| Signal | Effect |
|--------|--------|
| `SIGHUP` | Reload configuration |
| `SIGTERM` / `SIGINT` | Graceful shutdown |
| `SIGUSR1` | Dump status to stderr |
<!-- source: internal/component/bgp/reactor/signal.go -- SignalHandler, SIGTERM/SIGINT/SIGHUP/SIGUSR1 -->

## ze-chaos

Chaos monkey for testing Ze BGP route server propagation.

### AI Integration Flags

| Flag | Description |
|------|-------------|
| `--mcp <addr:port>` | Start chaos MCP server for AI queries (e.g. `:8001`) |
| `--ze-mcp <port>` | Inject Ze MCP server port into generated config |
| `--ai-help` | Print chaos MCP tool definitions as JSON and exit |

```bash
ze-chaos --mcp :8001 --web :8000 --peers 4  # MCP + web dashboard
ze-chaos --ze-mcp 9718 --peers 4             # Inject MCP into Ze config
ze-chaos --ai-help                           # Print tool schemas
```
<!-- source: internal/chaos/orchestrator/cli.go -- CLI flags -->
