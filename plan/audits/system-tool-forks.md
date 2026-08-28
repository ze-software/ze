# External command forks in Ze code -- audit 2026-08-21

Audit of every site in Ze's shipped source that runs an external binary or a
shell, against `ai/rules/go-standards.md` "External Commands" and the (empty)
register `ai/allowed-system-commands.md`. Read-only: no source file was edited,
no test, lint or verify gate was run.

## Method, so this can be re-run

Searched `cmd/`, `internal/`, `pkg/` and the retired `scripts/evidence/` (current producer: `internal/le/`), excluding
`_test.go` anywhere and `internal/test/`, with two greps from the repo root:

```
grep -rn --include='*.go' -E '"os/exec"|exec\.Command|exec\.CommandContext|exec\.LookPath|exec\.Cmd|syscall\.Exec|unix\.Exec|sh -c|bash -c' \
    cmd/ internal/ pkg/ scripts/evidence/ | grep -v '_test\.go' | grep -v 'internal/test/'

grep -rn --include='*.go' -E 'os\.StartProcess|syscall\.ForkExec|unix\.Exec|ForkExec|pty\.Start|/bin/sh|/bin/bash|/bin/ash|"sh"|"bash"' \
    cmd/ internal/ pkg/ scripts/evidence/ | grep -v '_test\.go' | grep -v 'internal/test/'
```

These are historical commands over the retired tree. The current first-party
evidence producers live under `internal/le/deployment/` and `internal/le/qemu/`.

The second grep is what caught `internal/component/config/cli/cmd_edit.go`,
which forks through `os.StartProcess` and never touches `os/exec`. Every hit
was then read in place and traced to its enclosing function and, where a
wrapper was found, to its call sites: `internal/appliance/cmd_build.go`
`runExternal`, `internal/plugins/systemd/main.go` `realServiceOps.run` /
`.output` / `.lookPath`, and `internal/component/vpp/dpdk.go` `loadModule` are
each ONE finding with several call sites, not one finding per caller.

Excluded by the audit's scope and by the rule itself: `_test.go` files,
`test/`, `internal/test/`, `.ci` / `.et` / `.wb` fixtures, the retired `Makefile` (current producers: `internal/le/` native action tables), the retired `mk/` (current producer: `internal/le/`),
the retired `scripts/dev/` (current producer: `internal/le/`), the retired `scripts/checks/` (current producer: `internal/le/`), the retired `scripts/codegen/` (current producer: `internal/le/`), the retired `scripts/status/` (current producer: `internal/le/`),
`vendor/`, `third_party/`, `docs/` and `plan/`. The Python and shell drivers
under the retired `scripts/evidence/` (current producer: `internal/le/`) (`qemu-run.py`, `effective-*.py`,
`effective-verify.sh`) are also excluded: they are the harness that produces
evidence on a developer machine, not Go code Ze ships.

The two Go diagnostics under the retired `scripts/evidence/` (current producer: `internal/le/`) are already clean.
the retired `scripts/evidence/l2tp-tunnel-diag/main.go` (current producer: `internal/le/deployment/l2tpdiag_linux.go`) and
the retired `scripts/evidence/l2tp-pppox-diag/main.go` (current producer: `internal/le/deployment/l2tpdiag_linux.go`) both build their netlink and
PPPoL2TP requests by hand through `vishvananda/netlink`, `netlink/nl` and
`golang.org/x/sys/unix`; neither imports `os/exec`. Nothing to do there.

## Counts

| Measure | Count |
|---------|-------|
| Fork sites (`exec.Command*`, `syscall.Exec`, `os.StartProcess`) | 39 |
| `exec.LookPath`-only sites (resolve PATH, no fork) | 13 |
| Files in scope importing `os/exec` | 31 |
| Distinct named external binaries forked | 34 |
| Classes of arbitrary operator-supplied command forked | 4 |
| Findings (after collapsing wrappers) | 18 |
| Fork sites on a daemon runtime path | 12 |
| Fork sites on an operator CLI / installer path | 21 |
| Fork sites on a `ze doctor` path | 2 |
| Fork sites in harness code that lives under `internal/` | 4 |
| Fork sites in the retired `scripts/evidence/` (current producer: `internal/le/`) Go | 0 |
| Findings with no native Go path | 5 |

Distinct binaries: `modprobe`, `sh`, `wg`, `vppctl`, `systemctl`, `getent`,
`groupadd`, `addgroup`, `useradd`, `adduser`, `userdel`, `deluser`, `groupdel`,
`delgroup`, `chown`, `vpp`, `mkfs.ext4`, `debugfs`, `e2fsck`, `mount`,
`umount`, `grub-mkstandalone`, `grub2-mkstandalone`, `xorriso`, `python3`,
`go`, `qemu-system-x86_64`, `qemu-system-aarch64`, `ps`, `grep`, `sysctl`,
`agent-browser`, `bgpd`, `bird`, plus `brew` and `ze` resolved by LookPath.
The four arbitrary-command classes are the plugin `run` string, the healthcheck
`probe` and `on-change` strings, the ExaBGP bridge script, and the external
plugin spec passed to the schema CLI.

## Findings

Ordered most severe first. "Ships as" says which path the fork is on.

| # | Site (file:symbol) | Command(s) | Ships as | Native replacement | Severity |
|---|--------------------|------------|----------|--------------------|----------|
| 1 | `internal/component/l2tp/kernel_linux.go:514` `probeKernelModules` | `modprobe l2tp_ppp`, `modprobe pppol2tp` | daemon runtime, L2TP subsystem `Start()` | `os.Stat("/sys/module/<n>")` (already present as `moduleBuiltIn`); real load via `unix.FinitModule` | Critical |
| 2 | `internal/component/plugin/process/process.go:622` `(*Process).startExternal` | `/bin/sh -c <plugin run>` | daemon runtime, plugin supervisor | `exec` of argv without a shell is still a fork; the plugin contract itself needs a decision | High |
| 3 | `internal/component/config/system/conntrack_linux.go:41` `LoadConntrackModules` | `modprobe nf_conntrack_*` | daemon runtime, system config apply (`cmd/ze/hub/main_system.go:229`) | `unix.FinitModule`; presence already answered by `/proc/modules` in the same file | High |
| 4 | `internal/plugins/diag/diag.go:54,60` `RunWgKeypair` | `wg genkey`, `wg pubkey` | operator CLI, `ze generate wireguard keypair` | `crypto/ecdh` X25519 (`ecdh.X25519().GenerateKey`), base64 std encoding | High |
| 5 | `internal/component/bgp/plugins/healthcheck/probe.go:22` `runProbeCommand` and `hooks.go:63` `runSingleHook` | `/bin/sh -c <probe>`, `/bin/sh -c <hook>` | daemon runtime, per probe interval and per state change | none for the arbitrary command; native probes (TCP connect, HTTP, ICMP) would cover the common cases | High |
| 6 | `internal/plugins/flowexport/conntrack_setup_appliance_linux.go:54` `loadConntrackModules` | `modprobe nf_conntrack`, `modprobe nf_conntrack_netlink` | daemon runtime, gokrazy appliance ONLY (`ze_appliance` tag) | delete it: on gokrazy the modules are built in and `modprobe` is absent by construction | High |
| 7 | `internal/component/doctor/checks_linux.go:458` `checkVPPVersion` | `vppctl show version` | `ze doctor` | GoVPP binary API over the socket Ze already dials (`internal/component/vpp/conn.go`): `vpe.ShowVersion` | High |
| 8 | `internal/plugins/iface/vpp/doctor.go:382` `vppctlShowPlugins` | `vppctl show plugins` | `ze doctor` | GoVPP `vlib.CliInband` / plugin-info over the same connector | High |
| 9 | `internal/component/vpp/dpdk_linux.go:18` `loadModuleLinux` (wrapper `internal/component/vpp/dpdk.go:207` `loadModule`, call site `dpdk.go:198`) | `modprobe vfio-pci`, `modprobe vfio_iommu_type1` | daemon runtime, DPDK NIC bind | `unix.FinitModule`; `/sys/module` for the already-loaded case | Medium |
| 10 | `internal/component/config/system/console_linux.go:130` `gettyActive` (resolution at `:51` `ApplyConsole`) | `systemctl is-active --quiet serial-getty@<dev>.service` | daemon runtime, system config apply (`cmd/ze/hub/main_system.go:116,185`) | `os.Stat("/run/systemd/system")` to detect systemd, then D-Bus, or drop the check: the same file already owns the tty through `unix.IoctlSetTermios` | Medium |
| 11 | `internal/core/hostload/hostload.go:67` `processCount` and `hostload_darwin.go:16` `readLoadAvg1` | `sh -c "ps -eo comm \| grep -c ..."`, `sysctl -n vm.loadavg` | `internal/core/`, but the only consumers are `internal/test/runner` and the retired `scripts/status` (current producer: `internal/le/`) | walk `/proc/*/comm` with `os.ReadFile` on Linux; `unix.SysctlRaw("vm.loadavg")` on darwin. `hostload_linux.go` already reads `/proc/loadavg` this way | Medium |
| 12 | `internal/component/bgp/cli/decode_plugin.go:86` `invokePluginDecodeRequest`, `:285` `invokePluginSubprocess`, `:208` `invokePluginPath` | `$0 plugin <name> --decode`, `<plugin path> --decode` | operator CLI, `ze decode` | `invokePluginInProcess` at `decode_plugin.go:341` already does this in-process and is the test-mode fallback | Medium |
| 13 | `internal/plugins/systemd/main.go:139` `realServiceOps.run`, `:149` `.output`, `:128` `.lookPath` (17 call sites in `cmd_install.go`, `cmd_uninstall.go`) | `systemctl daemon-reload/enable/start/stop/disable`, `getent passwd\|group`, `groupadd`, `addgroup`, `useradd`, `adduser`, `userdel`, `deluser`, `groupdel`, `delgroup`, `chown` | installer CLI, `ze systemd install/uninstall` on a systemd host | `getent` -> `os/user.Lookup`/`LookupGroup`; `chown` -> `os.Chown`; `systemctl` -> D-Bus. User and group creation has no clean native path | Medium |
| 14 | `internal/appliance/cmd_build.go:108` `runExternal` (call sites `cmd_build.go:396,419,425`, `diskverify.go:75,96,100`), `cmd_iso.go:787,802` `runISOBuilder`, `cmd_kernel.go:550` `defaultKernelBuild`, `cmd_initrd.go:150` `defaultInitrdMakeBuild`, `cmd_run.go:111` `launchQEMU` | `mkfs.ext4`, `debugfs`, `e2fsck`, `mount`, `umount`, `grub-mkstandalone`, `xorriso`, `python3`, `go build`, `qemu-system-*` | build-host CLI, `ze appliance build/iso/kernel/initrd/run` | mostly none, see "no native path" below. `mkfs.ext4` + `debugfs` are replaceable by a Go ext4 writer; the rest are not | Medium |
| 15 | `internal/exabgp/bridge/bridge.go:387` `(*Bridge).Start`, `internal/plugins/exabgp/bridgeplugin/internal.go:199` `(*bridgeRunner).startScript`, `internal/plugins/exabgp/main_sdk.go:76` `runSDKMode` | operator-supplied ExaBGP script argv | daemon runtime (bridgeplugin) and CLI (`ze exabgp plugin`) | none: running the operator's ExaBGP process IS the feature | Low |
| 16 | `internal/component/vpp/vpp.go:257` `(*VPPManager).runOnce` | `vpp -c <startup.conf>` | daemon runtime, VPP supervisor | none: VPP is a separate daemon by design. `External: true` already skips the fork | Low |
| 17 | `internal/chaos/orchestrator/fork.go:36` `forkZe`, `:174` `forkDaemon` | `ze -`, `bgpd`, `bird` | interop CLI, `ze chaos` | none for FRR and BIRD: they are the peers under test | Low |
| 18 | `internal/component/web/testing/runner.go:466,476` `(*Browser).runAgentCore`, `.runAgentOutput` | `agent-browser` | harness under `internal/component/`, imported only by `internal/test/cli/cmd_web.go` | none: driving a real browser is the point. Belongs under `internal/test/` | Low |

### Self-exec sites, listed for completeness

These re-run Ze's own binary rather than a system tool. They do not depend on
an absent binary and are not what the rule is aimed at, but they are forks and
a spec should say so explicitly rather than leave them unclassified.

| Site | Call | Ships as |
|------|------|----------|
| `internal/component/config/system/selfupdate.go:116` `defaultRestart` | `syscall.Exec(binPath, os.Args, os.Environ())` | daemon runtime, self-update restart |
| `internal/plugins/provision/main.go:452` `forkAndServe` | `exec.CommandContext(self, "-")` | provisioning CLI |
| `internal/component/config/cli/cmd_edit.go:88` `startEphemeralDaemon`, `:141` `startWebOnlyDaemon` | `os.StartProcess(exe, ...)` | operator CLI, `ze config edit` |
| `cmd/ze/login.go:125` `defaultExecShell` | `syscall.Exec("/usr/local/bin/ze-recovery-shell", []string{"ash"}, ...)` | appliance serial console login |
| `internal/component/config/schema/cli/main.go:722` `getExternalPluginYANG` | `exec.CommandContext(args[0], ..., "--yang")` | schema CLI (external plugin, not self) |

### `exec.LookPath`-only sites

`exec.LookPath` walks `$PATH` and stats; it forks nothing. Listing them so the
next reader does not re-find them and mistake them for forks. The ones that
guard a fork are already covered by the finding above them.

`internal/component/plugin/doctor/check_plugins.go:46` `CheckPluginBinaries`
(doctor check that an external plugin's binary exists -- legitimate, the
question it asks is exactly "is this binary present"),
`internal/appliance/doctor_checks.go:13,109,121` `doctorLookPathFn`,
`internal/appliance/homebrew.go:36` `brewLookPathFn` (finds the Homebrew
prefix from `brew`'s own location; `os.Stat` on the two documented prefixes
plus `$HOMEBREW_PREFIX` is the native answer),
`internal/appliance/cmd_iso.go:45,180,193,477,514`,
`internal/appliance/cmd_build.go:99` `resolveE2FSTool`,
`internal/appliance/cmd_run.go:94`,
`internal/chaos/orchestrator/fork.go:30,125`,
`internal/component/config/system/conntrack_linux.go:24`,
`internal/component/config/system/console_linux.go:51`,
`internal/plugins/flowexport/conntrack_setup_appliance_linux.go:49`,
`internal/plugins/systemd/main.go:128`.

`internal/appliance/cmd_initrd.go:36` `initrdLookPathFn` is dead in non-test
code: the only references are in `cmd_initrd_test.go`. It can be deleted.

## Detail

### 1. L2TP module probe forks modprobe on the appliance

`internal/component/l2tp/kernel_linux.go:503-524` `probeKernelModules`:

```go
for _, mod := range [...]string{"l2tp_ppp", "pppol2tp"} {
    if moduleBuiltIn(mod) {
        return nil
    }
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    err := exec.CommandContext(ctx, "modprobe", mod).Run()
```

Called from `internal/component/l2tp/subsystem.go:212` through the
`probeKernelModulesFn` seam, at subsystem `Start()`. The doc comment cites
RFC 2661 Section 24.23 and says startup MUST fail if the probe fails, so the
fork's failure is not silent: it stops L2TP from starting.

That is the problem. On the gokrazy appliance there is no `modprobe`. The
function already contains the correct native check one line above the fork
(`moduleBuiltIn` is `os.Stat("/sys/module/" + name)`), which returns early when
the module is built in. If gokrazy's kernel names the built-in module
differently from either string in the array, or exposes it only through
`/proc/modules`, the `os.Stat` misses, `modprobe` is absent, both iterations
fail, and L2TP refuses to start on the one platform Ze ships.

Replacement: keep the `/sys/module` check, add `/proc/modules` as a second
source (`internal/component/config/system/conntrack_linux.go` already has
`readLoadedModules` doing exactly this), and where a genuine load is needed use
`unix.FinitModule(fd, "", 0)` on an fd opened against the `.ko` under
`/lib/modules/<release>/`. `unix.FinitModule` and `unix.DeleteModule` are both
in `golang.org/x/sys/unix`.

Blocker on the `FinitModule` half: `modprobe` also resolves `modules.dep`
dependencies and decompresses `.ko.xz`/`.ko.zst`, and `finit_module(2)` does
neither. That matters only on a general Linux distribution. On the appliance,
where the modules are built in, the `/sys/module` and `/proc/modules` checks
are the complete answer and no load is ever needed.

### 3. Conntrack helper modules, same shape, on the config apply path

`internal/component/config/system/conntrack_linux.go:15-48`
`LoadConntrackModules`, reached from `cmd/ze/hub/main_system.go:229` whenever
system config is applied. It resolves `modprobe` with `LookPath` at `:24` and
returns `nil, nil` when absent, so on the appliance the whole function is a
silent no-op: the operator configured conntrack helpers, Ze reports no error,
and no helper is loaded.

The file already reads `/proc/modules` in `readLoadedModules` and skips modules
that are loaded. The gap is only the load itself.

### 4. WireGuard keypair generation shells out to `wg`

`internal/plugins/diag/diag.go:34-72` `RunWgKeypair`, registered as a local
command at `internal/plugins/diag/register.go:21` under
`ze generate wireguard keypair`. Its own usage text says "The system must have
the `wg` binary installed", and it returns 1 when the binary is absent.

`wg genkey` is 32 bytes from the CSPRNG with the X25519 clamp applied, base64
standard encoding. `wg pubkey` is the Curve25519 scalar base multiplication of
that key. Both are stdlib since Go 1.20:

```go
k, err := ecdh.X25519().GenerateKey(rand.Reader)
priv := base64.StdEncoding.EncodeToString(k.Bytes())
pub  := base64.StdEncoding.EncodeToString(k.PublicKey().Bytes())
```

`golang.org/x/crypto/curve25519` is also already reachable (`x/crypto` is a
dependency, used for bcrypt in `cmd/ze/login.go`). This is the clearest case in
the audit: a shipped CLI command that fails on the appliance, replaced by three
lines of stdlib, with no behavioural difference in the output format.

### 6. flowexport's appliance-only conntrack modprobe is dead by its own comment

`internal/plugins/flowexport/conntrack_setup_appliance_linux.go` is built with
`//go:build linux && ze_appliance`, so it compiles ONLY into the gokrazy
appliance. Its own doc comment at `:44-47` says:

> On gokrazy these are built into the kernel and modprobe is absent, so a
> LookPath miss (or a modprobe error on a built-in module) is expected and only
> logged at debug

A fork that the file states can never succeed in the only build that contains
it should be deleted rather than ported. Note that the rest of the same file is
exemplary and is the pattern the other findings should follow: it writes the
sysctl with `os.WriteFile("/proc/sys/net/netfilter/nf_conntrack_acct", ...)`
and installs the tracking hook over netlink with `google/nftables`, with the
comment "no `nft` binary exists on gokrazy".

### 7 and 8. Both VPP doctor checks fork vppctl

`internal/component/doctor/checks_linux.go:441-476` `checkVPPVersion` runs
`vppctl show version` and greps the output for the literal `"vpp v"`. It
reaches this only when `interface backend` is configured to VPP, so it runs
exactly where VPP matters.

`internal/plugins/iface/vpp/doctor.go:378-388` `vppctlShowPlugins` runs
`vppctl show plugins` and its own comment states the defect precisely:

> It reports the same error for an absent vppctl binary, an absent socket and
> a wedged VPP. A caller MUST NOT read that error as evidence about the plugin
> set.

This is the rule's third defect verbatim: a diagnostic that cannot distinguish
"tool missing" from "the thing you asked about is broken".

Both have the same native replacement, and Ze is already holding the
connection. `internal/component/vpp/conn.go` and `vpp.go` maintain a GoVPP
connector against the VPP API socket, with `(*VPPManager).GetConnector` and
`IsConnected` already exported for dependent plugins. `show version` is the
`vpe.ShowVersion` binary API message; `show plugins` is reachable through
`vlib.CliInband` (or plugin-info) on the same channel.

One concrete piece of work: `vendor/go.fd.io/govpp/binapi/` currently vendors
`acl`, `classify`, `interface`, `ip`, `ipsec`, `l2`, `lcp`, `memclnt`, `mpls`,
`nat44_ed`, `policer`, `qos`, `span`, `sr`, `vxlan` and `wireguard`, but not
`vpe` or `vlib`. Those two binapi packages must be generated and vendored
before either check can be converted. The doctor check in
`internal/component/doctor` also has no connector in hand today and would need
one passed through `diagnostic.DoctorCheckContext` or fetched from the VPP
component.

### 11. hostload shells out to `ps | grep` from `internal/core/`

`internal/core/hostload/hostload.go:60-73` `processCount` builds a shell
pipeline and runs it:

```go
shellCmd := tb.Str("ps -eo comm | grep -c '").Str(pattern).Byte('\'').String()
cmd := exec.CommandContext(ctx, "sh", "-c", shellCmd)
```

and `hostload_darwin.go:16` forks `sysctl -n vm.loadavg`.

Its only importers are `internal/test/runner/hostload.go` and
the retired `scripts/status/verify_run.go` (current producer: `internal/le/verifyengine/run.go`), both harness, so the practical risk is nil. But
it lives in `internal/core/`, which is unambiguously governed by the rule, and
it is the only `sh -c` string-concatenation pipeline in `internal/core/`.

The native path is already half-written in the same package:
`hostload_linux.go` reads `/proc/loadavg` with `os.ReadFile` and parses field
zero. For `processCount`, walk `/proc/[0-9]*/comm` and count matches; for the
darwin load average, `unix.SysctlRaw("vm.loadavg")` returns a `loadavg` struct
(three `uint32` values plus `fscale`) with no fork. Either convert it or move
the package under `internal/test/` where its consumers live.

### 12. BGP decode forks Ze to decode a plugin NLRI, and already has an in-process path

`internal/component/bgp/cli/decode_plugin.go` forks in three places:
`invokePluginDecodeRequest` (`:86`) and `invokePluginSubprocess` (`:285`) both
run `os.Args[0] plugin <name> --decode` and pipe the request over stdin, and
`invokePluginPath` (`:208`) runs an operator-named plugin binary the same way.

Two things make this the cheapest fix after finding 4. First,
`invokePluginInProcess` at `:341` already decodes in-process and is used as the
fallback at `:150`, `:173` and `:193`. Second, both self-fork sites already
detect the test binary by name and skip the fork entirely
(`strings.HasSuffix(os.Args[0], ".test")`), which means the in-process path is
what the test suite actually exercises: the forking path is the one with less
coverage, not more.

The remaining question for a spec is `invokePluginPath`: an operator-supplied
external decoder binary cannot be run in-process, and it is the same problem
as finding 2.

### 13. `ze systemd install` is a shell script written in Go

`internal/plugins/systemd/main.go` defines the `serviceOps` interface at `:26`
with `run`, `output` and `lookPath`, implemented at `:138`, `:148` and `:128`
by `realServiceOps` over `exec.CommandContext`. `cmd_install.go` and
`cmd_uninstall.go` call through it 17 times.

Three tiers of replacement:

- `getent passwd <u>` / `getent group <g>` (`cmd_install.go:162,168`,
  `cmd_uninstall.go:72,81`) -> `os/user.Lookup` and `user.LookupGroup`. With
  `CGO_ENABLED=0` these parse `/etc/passwd` and `/etc/group` directly, which
  is what `getent` does on any host without NSS plugins.
- `chown <u>:<g> <path>` (`main.go:154` `realServiceOps.chown`) -> `os.Chown`
  with the uid/gid from the lookup above. This one is a pure formatting fork:
  Ze already has the names.
- `systemctl daemon-reload/enable/start/stop/disable`
  (`cmd_install.go:99,103,108`, `cmd_uninstall.go:46,49,57`) -> systemd's D-Bus
  API (`org.freedesktop.systemd1.Manager` `Reload`, `EnableUnitFiles`,
  `StartUnit`, `StopUnit`, `DisableUnitFiles`). This adds a D-Bus dependency
  (`github.com/godbus/dbus/v5`, or `coreos/go-systemd/dbus`) that Ze does not
  currently carry.

User and group creation is the part with no answer; see below.

## No native path found

Five findings where I could not identify a Go replacement, with the blocker for
each. These are the rows a register entry would have to justify.

**Appliance image builders (finding 14): `xorriso`, `grub-mkstandalone`,
`python3`, `go`, `qemu-system-*`, `mount`, `umount`.** Blocker: each is a large
third-party program whose job is not a kernel interface Ze could read instead.
`grub-mkstandalone` assembles a GRUB EFI image from GRUB's own module set;
`xorriso` writes an El Torito ISO 9660 filesystem; `python3` runs the kernel
build script at `internal/appliance/cmd_kernel.go:550`; `go build`
cross-compiles `cmd/ze-installer` at `cmd_initrd.go:150`; `qemu-system-*` is a
hypervisor. `mount -o loop,ro` and `umount` in `diskverify.go:96,100` need a
loop device plus a mount syscall, reachable through `unix.Mount` and the
`/dev/loop-control` ioctls, but the tool is the smaller half of that problem.
Mitigating context: all of these run on a build host under `ze appliance ...`,
never on the appliance the image boots, and `resolveE2FSTool`,
`isoLookPathFn` and `launchQEMU` all report a clear "install X" message when
the tool is missing. This is the strongest candidate for register rows rather
than conversion.

**ext4 image authoring: `mkfs.ext4`, `debugfs`, `e2fsck` (finding 14).**
Blocker: no maintained Go library writes an ext4 filesystem. This one is
different from the rest of finding 14 in that a Go implementation is
*conceivable* (`diskfs/go-diskfs` has partial ext4 write support) and the
current code has already been burned by the fork: `cmd_build.go:82-91` carries
a long comment about Alpine splitting `debugfs` into `e2fsprogs-extra`, which
made every tool read as absent, and `diskverify.go:36-37` exists specifically
because `debugfs -R` exits 0 on internal errors, so Ze re-reads the image
itself to confirm the write landed. That verification is the right instinct and
should be noted as evidence for how much the fork costs.

**Operator-supplied commands (findings 2, 5, 15, and `invokePluginPath` in
12).** The plugin `run` string, the healthcheck `probe` and `on-change`
strings, and the ExaBGP bridge script are all "run what the operator wrote".
Blocker: there is no Go replacement for an arbitrary command, only a narrower
contract. Two are separable questions a spec should split. (a) `/bin/sh -c`
versus argv: `internal/component/plugin/process/process.go:622` and the two
healthcheck sites use a shell where the ExaBGP bridge already splits the
command itself (`bridgeplugin/internal.go:194` `splitCommand`) and forks argv
directly. Dropping the shell removes the `/bin/sh` dependency, which the
appliance also lacks, without changing what the operator can express beyond
shell metacharacters. (b) whether healthcheck should offer native TCP-connect,
HTTP and ICMP probe types so that the common cases need no fork at all; Ze
already has the networking to do all three.

**Third-party BGP daemons: `bgpd`, `bird` (finding 17).** Blocker: FRR and BIRD
are the implementations `ze chaos` tests interoperability against. There is no
Go replacement for another vendor's daemon. `internal/chaos/orchestrator` runs
on a developer machine under `ze chaos`, and `forkDaemon` already reports
"`<name>` not found in PATH (use --binary to specify)".

**System user and group creation (finding 13): `groupadd`/`addgroup`,
`useradd`/`adduser`, `userdel`/`deluser`, `groupdel`/`delgroup`.** Blocker:
creating a system user means an atomic edit of `/etc/passwd`, `/etc/shadow`,
`/etc/group` and `/etc/gshadow` under the `/etc/.pwd.lock` protocol, and on a
host that may use LDAP, SSSD or systemd-homed the files are not the source of
truth at all. Rewriting those files from Ze is worse than forking the tool that
owns them. The four-way fallback in `cmd_install.go:176-214` (shadow-utils
first, BusyBox second) is deliberate and correct for the hosts it targets. This
runs under `ze systemd install` on a systemd distribution, never on the
appliance, which has no `/etc/passwd` to edit and no systemd to install into.
