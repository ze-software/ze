# 792 -- Runtime Platform Detection

## Context

Ze runs on gokrazy appliances, systemd hosts, containers, and plain Linux, but had no unified way to identify the platform at runtime. Platform-specific code was scattered (reboot, NTP clock, DHCP resolver paths, crash dir) with no single detection point. Operators and doctor checks needed to know what platform they were on to give appropriate guidance.

## Decisions

- Placed detection in `internal/component/host/` (extends Inventory) over a new `internal/core/platform/` package because the host package already owns hardware/environment detection with the Detector/Root testdata pattern
- Used a priority-ordered classification (gokrazy > container > systemd > plain-linux) because gokrazy is a specialization of Linux with a read-only root, and containers may have systemd paths present from the host
- Detect containers via both cgroups v1 (`/proc/1/cgroup` hint strings) and v2 (`/proc/1/mountinfo` hint strings) because pure cgroups v2 writes only `0::/` to `/proc/1/cgroup` with no runtime markers
- Used `syscall.Access(path, 0x2)` with a named `const wOK` instead of importing `golang.org/x/sys/unix` for a single constant, since the host package has no existing `x/sys/unix` dependency
- Exposed FD limits (soft/hard/raisable) on PlatformInfo rather than only on `show system file-descriptors` because the platform capability view is the natural place for process resource limits
- Added `set system file-descriptors` as an online RPC (not offline) because it modifies the running daemon's rlimit and the change only lasts for the process lifetime

## Consequences

- `show system platform`, `show host platform`, `ze doctor`, and `ze support` all consume the same `DetectPlatform()` path
- Doctor checks can now be platform-conditional (gokrazy /perm writability, container read-only root)
- The `sectionDetectors` map auto-exposes `show host platform` and `ze host show platform` with no additional wiring
- Future platform-specific behavior (config path selection, crash dir, resolv.conf path) can key off `PlatformInfo.Type` instead of ad-hoc file probes

## Files

None recorded.
