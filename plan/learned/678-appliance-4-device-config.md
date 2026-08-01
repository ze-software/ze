# 678: appliance-4-device-config

Device-side config management for ze appliances: config loading priority, last-known-good hash, auto-revert on runtime failure.

## Context

Final spec in the 4-part appliance series. Specs 1-3 covered build, backup, and remote operations. This spec covers what happens on the device when it boots or receives a config push: which config takes priority, how invalid configs are handled, and how the device recovers from a bad config change.

## Decisions

| Decision | Rationale |
|----------|-----------|
| Pushed config at /perm/ze/config-pushed.conf | /perm/ is gokrazy's persistent partition; ZeFS is read-only after build |
| Validation via config.LoadConfig before applying | Same parse + YANG validation as normal config; no weaker path |
| checkPushedConfig runs on all boots (not just appliance) | ENOENT fast path on non-appliance is cheaper than adding a guard |
| LKG hash uses ConfigHash (same as manifest) | Single hash function; no divergence between build.json and ZeFS |
| HealthRevert uses function variables for filesystem ops | /perm/ze/ paths don't exist on dev machines; testable without real FS |
| HealthRevert not wired to reactor yet | SSH config-push is still a stub (spec 2); wiring deferred to integration |
| Two-tier revert: previous pushed -> ZeFS seed | Gives one undo before falling back to immutable seed |
| 30s health window | BGP sessions typically establish in 5-10s; 30s catches delayed failures |

## Patterns

- **Function variable mocking**: `readPushedConfig`, `removePushedConfig`, `writeActiveHash` are package-level function vars with test-time replacement. Avoids interface overhead for simple filesystem calls.
- **storage.NewBlob for test stores**: `zefs.BlobStore` doesn't implement `storage.Storage` (missing `AcquireLock`). Tests use `storage.NewBlob(dbPath, dir)` instead of `zefs.Create`.
- **Timer-based health monitor**: `HealthRevert` uses `time.AfterFunc` with mutex protection. `OnPeerClosed` stops the timer and reverts; timer expiry confirms the config.

## Mistakes

| Mistake | Impact | Fix |
|---------|--------|-----|
| Used zefs.BlobStore directly in tests | Compile error (missing AcquireLock) | Switched to storage.NewBlob |
| Two sequential append calls in assembleZeFS | gocritic appendCombine lint failure | Combined into single append with multiple args |
| byte slice comparisons with string() cast | gocritic stringXbytes lint failure | Used bytes.Equal instead |

## Deferred

- HealthRevert wiring to reactor via PeerLifecycleObserver (blocked on real SSH config-push integration)
- Functional .ci tests for boot with/without pushed config (gokrazy environment needed)

## Files

- `cmd/ze/pushed_config.go` (70L): Pushed config loading, validation, active hash
- `cmd/ze/pushed_config_test.go` (175L): 6 tests for boot scenarios
- `cmd/ze/health_revert.go` (140L): Auto-revert health monitor with 30s window
- `cmd/ze/health_revert_test.go` (150L): 3 tests for revert and healthy scenarios
- `cmd/ze/appliance/cmd_assemble_lkg_test.go` (95L): 3 tests for LKG hash write

## Files Modified

- `pkg/zefs/keys.go`: Added KeyConfigLastKnownGood
- `cmd/ze/appliance/cmd_assemble.go`: LKG hash write in assembleZeFS
- `cmd/ze/main.go`: Wired checkPushedConfig + writeConfigActiveHash into cmdStart
- `docs/guide/appliance.md`: Device-side config behavior section
- `docs/architecture/core-design.md`: Section 20 (Appliance Config Loading Priority)
