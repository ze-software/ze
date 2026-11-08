# Spec: appliance serial console log level

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | config |
| Priority | P1 |
| Release reason | Every appliance image ships a debug console level: every kernel message, debug included, and Ze's whole log stream go to a 115200-baud serial console, and a message burst waits on that line. A quiet production default with a debug option is part of the appliance boot the first release delivers. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

The runtime image boots with `loglevel=8` on its kernel command line, so the
console level admits every printk level, `KERN_DEBUG` included. The console is
`ttyS0` at 115200 baud (`SerialConsole` in `gokrazy/ze/config.json`). Ze also
logs to kmsg (`ze.log.backend=kmsg,stderr` in the same file), so Ze's whole log
stream reaches the serial line as well. A burst of kernel or Ze messages then
waits on a slow serial console.

Goal: a production image boots with a quiet console level, so only errors and
worse reach the serial console, while `dmesg` and the kmsg ring keep every
message. An operator can still raise the level for serial debugging, at build
time and at run time. Ze's own kmsg lines carry the priority of their slog
level, so the console level filters Ze's chatter and still shows Ze's errors.

VyOS fixed the same class in T9283 (`fad6408ed` in
`~/Code/github.com/vyos/vyos-1x`): `kernel.printk = 4 4 1 7` in
`src/etc/sysctl.d/30-vyos-router.conf`, "Stop low-level kernel messages from
reaching the console".

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/appliance/README.md` - which page owns the image kernel command line
- [ ] `docs/architecture/appliance/builder.md` - how `ze appliance init` image options reach the build
- [ ] `docs/guide/appliance.md` - what the operator reads about logging and the console

**Key insights:**
- The console level filters only what reaches the console. The kmsg ring keeps every record.
- A kmsg write with no `<N>` prefix takes `default_message_loglevel`, so console filtering of Ze's lines needs the prefix.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `gokrazy/ze/config.json` - `KernelExtraArgs` holds `loglevel=8` and
  `printk.devkmsg=on`; `SerialConsole` is `ttyS0,115200`; the ze package
  environment sets `ze.log=info` and `ze.log.backend=kmsg,stderr`. Commit
  `3873f45e9b` (2026-06-16) added `loglevel=8`, and its message is the only
  recorded reason: "Gokrazy init and Ze log to kmsg at INFO level (6) but the
  default console loglevel is 4. Messages were silently dropped, making serial
  debugging impossible." No page under `docs/architecture/appliance/` records
  it, and `docs/guide/appliance.md` names only the `ze.log.backend` value.
- [ ] `internal/core/slogutil/slogutil.go` - `createHandler` and `openKmsg`:
  the kmsg backend writes each formatted slog line to `/dev/kmsg` with no `<N>`
  priority prefix, so the kernel files every line at
  `default_message_loglevel`. Ze's 7.3 kernel (the lab Docker host, `uname -r`
  7.3.0-ze, booted from Ze's image) reads `/proc/sys/kernel/printk` as
  `8 4 1 7`: every Ze line, ERROR and DEBUG alike, is filed at level 4
  (warning), and the console shows it because 4 is below 8. The commit
  message's "INFO level (6)" does not match what Ze writes.
- [ ] `internal/appliance/cmd_iso.go` - `isoKernelConsoleArgs`: the installer
  ISO passes only `console=` arguments, so the installer boots with the
  kernel's built-in console level. This spec does not change it.
- [ ] `internal/appliance/kernelargs.go` - `hugepageKernelArgs`,
  `isolatedCPUKernelArgs` and `crashDumpKernelArgs` turn `image.*` options from
  `ze appliance init` into extra kernel arguments; `resolveBuildParentDir`
  passes them to `internal/appliance/instance/prepare.go`, whose
  `deriveConfigJSON` appends them after the instance's `KernelExtraArgs`.

**Behavior to preserve:**
- The kmsg ring and `dmesg` keep every message. Only the console level changes.
- Ze still logs to kmsg and stderr with the configured `ze.log` level.
- An operator who needs serial debugging can get today's behavior back.

**Behavior to change:**
- The default runtime console level becomes 4 (errors and worse reach the
  console).
- Ze's kmsg lines carry the syslog priority of their slog level.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Instance kernel arguments | `gokrazy/ze/config.json` `KernelExtraArgs` | Carries `loglevel=8` for every image | Extends: the default argument becomes the quiet level |
| Image options that become kernel arguments | `internal/appliance/kernelargs.go` (`hugepageKernelArgs` and siblings, `resolveBuildParentDir`), `internal/appliance/instance/prepare.go` `Options.ExtraKernelArgs`, `deriveConfigJSON` | Turns `image.*` options from `ze appliance init` into extra kernel arguments appended after the defaults | Extends: a console log level image option is one more argument builder of the same shape; the design states how its `loglevel=` token relates to the default one |
| The sysctl component | `internal/component/sysctl` (config over transient over plugin defaults), `ai/doors/forwarding.md` | Owns every kernel tunable an operator sets at run time | Extends: `kernel.printk` is a sysctl, so the run-time debug route is `set sysctl kernel.printk ...`; the design checks that the key passes the key registry in `internal/core/sysctl` and adds it if it does not |
| The kmsg log backend | `internal/core/slogutil/slogutil.go` `openKmsg`, `createHandler` | Writes slog text lines to `/dev/kmsg` with no priority | Extends: each line gets the `<N>` prefix of its slog level |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Build time: the `image` block that `ze appliance init` writes, and the checked-in `gokrazy/ze/config.json`.
- Run time: `set sysctl kernel.printk` through the sysctl component, and every slog record Ze writes.

### Transformation Path
1. `resolveBuildParentDir` collects the image kernel arguments, and `deriveConfigJSON` appends them to `KernelExtraArgs` of the prepared instance.
2. gokrazy writes the command line, and the kernel sets `console_loglevel` from the `loglevel=` token.
3. At run time, the slog handler formats a record and the kmsg writer prefixes the priority of its level before the write to `/dev/kmsg`.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Build host to device | kernel command line in the image | No |
| Ze to kernel | `/dev/kmsg` record priority, `/proc/sys/kernel/printk` | No |

### Integration Points
- `internal/appliance/kernelargs.go` image argument builders.
- `internal/core/slogutil/slogutil.go` `openKmsg`.

## Risks & Assumptions

### Assumptions
| Assumption | Validation method | Status |
|------------|-------------------|--------|
| The kernel uses the last `loglevel=` token when the image option appends one after the default | Read `console_loglevel` on a booted image with both tokens | unvalidated |
| gokrazy's own kmsg lines carry a priority, so a quiet console still shows its errors | Read gokrazy's kmsg writer in the pinned module | unvalidated |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| booted appliance image | -> | `KernelExtraArgs` default | QEMU appliance boot test reads `/proc/sys/kernel/printk` (AC-1) |
| `ze appliance init` image option | -> | `internal/appliance/kernelargs.go` | `.ci` under `./le test functional appliance` (AC-2, AC-3) |
| Ze ERROR log line | -> | `openKmsg` priority prefix | Linux test reading `/dev/kmsg` (AC-5) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | An appliance image built with no console option boots | `/proc/sys/kernel/printk` reads 4 as its first field, and the kernel command line carries `loglevel=4`. Test: a QEMU appliance boot test reads both | `printk.devkmsg=on` may stay if the design keeps it, with its reason written here |
| AC-2 | `ze appliance init` with the new image option set to a debug level, then a build | The image's kernel command line carries the requested `loglevel=`, and the booted console level equals it. Test: a `.ci` under the appliance functional suite through `ze appliance init`, beside the siblings of `image.isolated-cpus`, and the QEMU boot test | None |
| AC-3 | The image option set to a value outside 0 to 8 | `ze appliance init` refuses it with the accepted range and the fix. Test: a `.ci` refusal case | None |
| AC-4 | A running appliance, then `set sysctl kernel.printk` with a debug level, committed | The console level changes at once, and deleting the setting restores the boot value. Test: a QEMU or `needs-linux` functional test reading `/proc/sys/kernel/printk` before and after | None |
| AC-5 | Ze logs at ERROR, WARN, INFO and DEBUG with the kmsg backend | Each `/dev/kmsg` record carries priority 3, 4, 6 and 7 respectively. Test: a slogutil unit test over the written bytes, and a Linux test that reads the records back from `/dev/kmsg` | None |
| AC-6 | A booted image at the default level, and Ze logs one ERROR and one INFO line | The ERROR line reaches the serial console and the INFO line does not; both are in `dmesg`. Test: the QEMU boot test reads the serial log | None |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates |
|------|------|-----------|
| kmsg priority per slog level | `internal/core/slogutil/` | AC-5 |
| console level image option renders and refuses | `internal/appliance/kernelargs_test.go` | AC-2, AC-3 |

### Functional Tests
| Test | Location | Validates |
|------|----------|-----------|
| image option through `ze appliance init` | the appliance `.ci` suite | AC-2, AC-3 |
| booted image console level and serial output | QEMU appliance boot test | AC-1, AC-4, AC-6 |

## Files to Modify

- `gokrazy/ze/config.json` - the default console level.
- `internal/appliance/kernelargs.go` - the console level image option.
- `internal/core/slogutil/slogutil.go` - the kmsg priority prefix.
- `docs/guide/appliance.md` - the console level, the image option and the
  run-time sysctl.
- The page under `docs/architecture/appliance/` that owns the image kernel
  command line records the default and why.

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 2 | Config syntax changed? | Yes | the `ze appliance init` image option page |
| 6 | Has a user guide page? | Yes | `docs/guide/appliance.md` |
| 12 | Internal architecture changed? | Yes | the appliance page that owns the kernel command line |

## Implementation Steps

1. Write the failing tests for AC-1 to AC-6.
2. Prefix kmsg lines with the priority of their slog level.
3. Set the default console level and add the image option.
4. Confirm the run-time sysctl route, update the pages, run the tests.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `escape-hatch-scoped-wider-than-its-justification.md` | 2026-11-08 | `loglevel=8` in `KernelExtraArgs` reaches every production serial console |
| `library-default-left-in-place.md` | 2026-11-08 | `openKmsg` writes no priority, every Ze line filed at level 4 |

## Checklist

- [ ] Tests written
- [ ] Tests FAIL before the fix
- [ ] Tests PASS after the fix
- [ ] `./le test functional appliance`
- [ ] `./le test qemu installer`
