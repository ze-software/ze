# 711: spec-cpe-3-console-serial

**Status:** Done
**Commit:** 17a7b607a feat(system): add serial console configuration (spec-cpe-3)
**Docs commit:** b508e6ecb docs(testing): add QEMU integration testing guide

## What Was Built

Serial console configuration under `system { console { device <name> { speed <baud> } } }`.
Single termios code path on all Linux hosts. On systemd hosts, detects active
`serial-getty@<device>.service` and skips with a warning. On gokrazy (no systemctl),
the check is skipped and termios is applied directly.

YANG schema with speed enum (9600-115200, default 115200). Config extraction via
both `*config.Tree` (startup) and `map[string]any` (reload) paths. Wired at startup
and on config reload in `cmd/ze/hub/main.go`.

## Lessons Learned

### QEMU integration tests are mandatory for linux-only code

**Problem:** Initial implementation skipped the functional test with the
rationalization "requires a real serial device." This was wrong. The project
has a QEMU Alpine VM (`make ze-qemu-integration-test`) that provides full
kernel capabilities, and PTY pairs (`creack/pty`, already vendored) are the
standard virtual substitute for serial ports.

**Fix:** Created `console_integration_linux_test.go` with 7 QEMU integration
tests using PTY pairs. Added `config/system` to the QEMU Makefile target.

**Rule created:** `ai/rules/platform-linux.md` with a virtual substitute table
(PTY for serial, veth for networking, netns for isolation), build tag guidance,
and Makefile wiring instructions. Added to CLAUDE.md "Before You..." table and
`ai/INDEX.md`. Human-facing guide at `docs/architecture/testing/qemu-integration.md`.

**Root cause:** The ai/rules/testing.md QEMU section was too brief. It said
"add your package to the Makefile" but didn't explain virtual substitutes for
hardware or emphasize that "needs real hardware" is never valid. The expanded
rule closes this gap for future sessions.

### Review findings drive real improvements

The `/ze-review` pass caught four issues that all got fixed:
1. `ValidConsoleDeviceName` didn't reject null bytes/control characters
2. `gettyActive` called `exec.LookPath` redundantly per device (cached now)
3. `applyTermios` only set Cflag, leaving Lflag/Iflag/Oflag in whatever state
   the terminal had before (now sets full raw mode)
4. `ExtractConsoleFromMap` (reload path) had no unit tests

## Files

| File | Purpose |
|------|---------|
| `internal/component/config/system/console.go` | Types, extraction, validation |
| `internal/component/config/system/console_linux.go` | Termios apply + getty check |
| `internal/component/config/system/console_other.go` | Non-linux stub |
| `internal/component/config/system/console_test.go` | 14 unit tests |
| `internal/component/config/system/console_integration_linux_test.go` | 7 QEMU integration tests |
| `internal/component/config/system/system.go` | ConsoleDevices field + extractConsole call |
| `internal/component/config/system/yang/ze-system-conf.yang` | Console container |
| `cmd/ze/hub/main.go` | applyConsole + applyConsoleFromMap wiring |
