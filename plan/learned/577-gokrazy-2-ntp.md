# 577 -- gokrazy-2: NTP Client Plugin

## Context

Ze had zero time synchronization code. On gokrazy appliances (Raspberry Pi, VMs),
the system clock starts at epoch without an RTC. BGP OPEN timestamps, TLS certificate
validation, and log ordering all require a sane clock. gokrazy provided its own NTP
client, but the goal is for ze to own all system services so operators have a single
config surface. This plugin syncs the clock, writes the RTC, and persists time to
disk for recovery on devices without battery-backed clocks.

## Decisions

- Used `beevik/ntp` library (v1.5.0) over raw NTP packet crafting -- same library
  gokrazy uses, proven, small, handles NTPv4 SNTP subset.
- Plugin with `ConfigRoots: ["environment"]` over standalone binary -- single config
  surface, no separate process to manage.
- YANG schema registered via `yang.RegisterModule` in schema package `init()` -- this
  is the mechanism that makes `ze config validate` accept the leaves. The `YANG` field
  on Registration is a separate (redundant) storage for runtime schema queries.
- DHCP option 42 integration via event bus subscription over direct import of ifacedhcp --
  loose coupling, NTP plugin subscribes to `interface/dhcp/lease-acquired` events.
- Configured servers take priority over DHCP-discovered ones -- explicit config always wins.
- Time persistence to `/perm/ze/timefile` (configurable) -- gokrazy ext4 partition survives reboots.
- Anti-thundering-herd jitter (0-250ms) before each NTP query -- prevents synchronized
  boot storms in multi-device deployments.
- Absurd timestamp rejection (year < 2020 or > 2100) -- prevents corrupted NTP responses
  from setting the clock to nonsense values.

## Consequences

- `environment { ntp { enabled true; server pool0 { address ... } } }` activates NTP.
- Clock is set on startup (restore from file, then NTP sync).
- RTC written after each sync (if /dev/rtc0 exists).
- Time saved to disk on shutdown (SIGTERM triggers plugin stop).
- DHCP-discovered NTP servers are used as fallback when no servers configured.
- spec-gokrazy-3 can now remove `WaitForClock` from gokrazy config since ze owns the clock.
- New dependency: `github.com/beevik/ntp v1.5.0` (vendored).

## Gotchas

- YANG registration requires TWO things: (1) `YANG` field on Registration for runtime,
  AND (2) `yang.RegisterModule` in a schema/register.go `init()` for the config validator.
  The DNS resolver was the reference pattern (`internal/component/resolve/dns/schema/register.go`).
- The `config.YANGSchema()` function used by `ze config validate` loads registered YANG
  via `LoadRegistered()`. If the schema package isn't imported (blank import via all.go
  or transitively), the YANG won't be visible to the validator.
- `math/rand/v2` triggers gosec G404 -- NTP jitter and server selection don't need crypto
  randomness; nolint comments required.
- `unsafe.Pointer` for RTC ioctl triggers gosec G103 -- unavoidable for kernel ioctls.
- `os.ReadFile` with config path triggers gosec G304 -- path comes from config, not user input.

## Files

- `internal/plugins/ntp/ntp.go` -- sync worker, config parsing, DHCP integration
- `internal/plugins/ntp/clock_linux.go` -- setClock (Settimeofday), setRTC (ioctl)
- `internal/plugins/ntp/clock_other.go` -- stub for non-Linux
- `internal/plugins/ntp/persist.go` -- saveTime, loadTime (RFC3339 to file)
- `internal/plugins/ntp/register.go` -- plugin registration, OnConfigure, lifecycle
- `internal/plugins/ntp/schema/ze-ntp-conf.yang` -- YANG schema
- `internal/plugins/ntp/schema/embed.go` -- go:embed
- `internal/plugins/ntp/schema/register.go` -- yang.RegisterModule init()
- `internal/plugins/ntp/ntp_test.go` -- 14 unit tests
- `test/parse/ntp-config.ci` -- functional parse test
- `test/parse/ntp-disabled.ci` -- functional parse test
