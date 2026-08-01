# 677: appliance-2-remote

Remote operations for gokrazy-based Ze appliance fleet: OTA push, config preview, batch init, config-push, and parallel operations.

## Context

The bastion manages a fleet of gokrazy devices. After building images (spec 1) and backing them up (spec 3), the operator needs to push images and config changes to running devices without physical access. This spec adds the bastion-side operations; device-side behavior (config loading priority, auto-revert) is in spec 4.

## Decisions

| Decision | Rationale |
|----------|-----------|
| Push TLS verifies against stored cert.pem only | Devices use self-signed certs; system CA pool won't help |
| Push uses HTTP basic auth (empty user, token as password) | Matches gokrazy's update API scheme |
| --all requires passphrase agent (refuses interactive) | Interactive prompt per-device is error-prone for batch |
| Config preview reuses resolveSeedConfig() directly | No divergence between preview and what assemble builds |
| Config-push SSH uses operator's ssh-agent, not Ze's passphrase agent | SSH key management is the operator's concern |
| --parallel N bounded 1-64, default 1 | Sequential is safe default; pool pattern with WaitGroup.Go |
| Error messages differentiate unreachable from protocol errors | AC-41 vs AC-54 require distinct messages |
| sshExecFunc is a replaceable function variable | Testability without real SSH; real impl uses x/crypto/ssh |
| Batch init password=generate prints once to stdout | Never stored plaintext; each device gets unique random |
| listAddressedAppliances() shared by push --all and config-push --all | DRY; linter caught duplication |

## Patterns

- **Testable network operations**: `httptest.NewTLSServer` for push (real TLS), function variable for SSH mock
- **Fleet iteration helper**: `listAddressedAppliances()` filters by device.address presence, reused by all --all commands
- **Parallel execution**: `runParallel(names, N, op)` with bounded worker pool; sequential fallback when N=1
- **Error type differentiation**: `protocolError` type vs raw network errors for user-facing messages

## Mistakes

| Mistake | Impact | Fix |
|---------|--------|-----|
| Initial error message used "unreachable" for all failures | 401 auth errors confusingly labeled "unreachable" | Introduced protocolError type to differentiate |
| goconst lint triggered by "amd64" reaching 3 occurrences | Blocked lint pass | Extracted archAMD64/archARM64 constants (pre-existing issue) |

## Files

- `cmd/ze/appliance/cmd_push.go` (250L): OTA push via gokrazy HTTPS update API
- `cmd/ze/appliance/cmd_push_test.go` (350L): Push tests with TLS mock server
- `cmd/ze/appliance/cmd_config.go` (70L): Config preview (--merged)
- `cmd/ze/appliance/cmd_config_test.go` (90L): Config merge output tests
- `cmd/ze/appliance/cmd_config_push.go` (130L): Config push via SSH (mocked)
- `cmd/ze/appliance/cmd_config_push_test.go` (240L): Config push tests with SSH mock
- `cmd/ze/appliance/parallel.go` (90L): Bounded worker pool for --parallel N

## Files Modified

- `cmd/ze/appliance/main.go`: Added push, config, config-push to handlers + stubs + usage
- `cmd/ze/appliance/cmd_init.go`: Added --batch flag, runBatchInit, initOneFromBatch
- `cmd/ze/appliance/register.go`: Updated Subs string
- `cmd/ze/appliance/config.go`: Extracted archAMD64/archARM64 constants
- `docs/guide/appliance.md`: Added remote operations and batch init sections
