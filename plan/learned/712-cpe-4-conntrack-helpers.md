# 712: spec-cpe-4-conntrack-helpers

**Status:** Done
**Commit:** 8918a4640 feat(system): add conntrack management (spec-cpe-4)

## What Was Built

Connection tracking management under `system { conntrack {} }`. Three areas:

1. **Helper modules:** declarative loading of 10 kernel conntrack ALG helpers
   (ftp, sip, h323, pptp, tftp, sane, irc, amanda, netbios-ns, snmp) via
   modprobe on Linux. Load-only (never unload). On gokrazy, `exec.LookPath`
   returns error and loading is silently skipped.

2. **Tuning:** user-friendly config for table sizing (table-size, hash-size,
   expect-max), per-protocol timeouts (TCP 10 states, UDP, ICMP, ICMPv6, GRE,
   SCTP 8 states, DCCP 7 states), TCP behavior flags (be-liberal, loose,
   max-retrans, ignore-invalid-rst), and global flags (accounting, timestamp,
   checksum, log-invalid). All mapped internally to sysctl keys and emitted
   via EventBus `(sysctl, default)` with source `"system-conntrack"`.

3. **Observability:** `show system conntrack` reads live sysctl values and
   `/proc/modules`. Telemetry collector gains a configured-max gauge.

Dual-setting prevention: conntrack registers its managed sysctl keys at startup.
The sysctl plugin verifier rejects any key in `sysctl {}` (including profiles)
that conntrack manages.

## Lessons Learned

### Module name normalization between config and kernel

**Problem:** Config uses hyphens (`netbios-ns`) but the kernel uses underscores
(`nf_conntrack_netbios_ns`). The initial implementation concatenated the config
name directly, producing `nf_conntrack_netbios-ns`. While `modprobe` treats
hyphens and underscores interchangeably, `/proc/modules` always uses underscores.
This caused `isModuleLoaded` to miss already-loaded modules and `LoadedConntrackModules`
to return underscore-based names that didn't match config.

**Fix:** Added `toKernelModName` (hyphens to underscores) and `toConfigModName`
(underscores to hyphens) converters. Applied at every boundary between config
names and kernel names.

**Pattern:** When config names differ from kernel/OS names, normalize at the
boundary. Don't assume external tools will normalize for you.

### CheckManaged must cover all sysctl config entry points

**Problem:** The dual-setting prevention (`CheckManaged`) was initially only
added to the `sysctl { setting {} }` validation loop, not the
`sysctl { profile {} }` loop. A user-defined profile could include a
conntrack-managed key and bypass the check.

**Fix:** Added `CheckManaged` to the profile validation loop in
`verifySysctlConfig`.

**Pattern:** When adding a validation gate, grep for all paths that accept the
same input. The symmetry check from `/ze-review` caught this.

### SystemConfig grows: pointer receivers matter

Adding `ConntrackConfig` (360 bytes) to `SystemConfig` pushed it past the
`gocritic` hugeParam threshold (512 bytes). All functions taking `SystemConfig`
by value needed updating to pointer receivers, including pre-existing ones in
archive.go. Changing a struct's size can cascade through unrelated callers.

## Files

| File | Purpose |
|------|---------|
| `internal/component/config/system/conntrack.go` | Types, extraction (Tree + Map), sysctl key mapping, module validation, managed key registration |
| `internal/component/config/system/conntrack_linux.go` | modprobe backend, /proc/modules reader, name normalization |
| `internal/component/config/system/conntrack_other.go` | Non-Linux stub |
| `internal/component/config/system/conntrack_test.go` | 17 unit tests |
| `internal/component/config/system/system.go` | ConntrackConfig field + extractConntrack call |
| `internal/component/config/system/yang/ze-system-conf.yang` | Conntrack container with enums and ranges |
| `internal/core/sysctl/managed.go` | Managed key registry for dual-setting prevention |
| `internal/component/sysctl/register.go` | CheckManaged in both setting and profile verifiers |
| `internal/component/sysctl/sysctl_test.go` | Dual-setting prevention test |
| `internal/component/firewall/cmd_show_conntrack.go` | show system conntrack handler |
| `internal/component/cmd/show/show.go` | RPC registration |
| `internal/component/telemetry/collector/conntrack_linux.go` | configured-max gauge |
| `cmd/ze/hub/main.go` | Startup + reload wiring, managed key registration |
