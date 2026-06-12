# 889 -- reactor-tuning-yang

## Context

17+ env vars controlled reactor behavior but only 4 had YANG leaves. The rest were invisible in `show configuration`, excluded from commit/rollback, unvalidated, undiscoverable, and required a restart. The goal was to promote 7 operator-facing env vars to YANG config leaves under `environment { reactor { } }`.

## Decisions

- Promoted via `envPlumbingTable` plumbing over direct config struct, because the consumer code already reads `env.Get*()` and the plumbing preserves that interface with zero consumer-side changes.
- Marked env vars as deprecated over removing them, preserving OS env override as an emergency escape hatch while directing operators to the YANG surface.
- Chose `uint32` for pool byte budgets over `int64` (matching the env type), because >4GB buffer pools are unrealistic and `uint32` is the standard YANG numeric type in this codebase.
- Excluded `ze.rs.chan.size` from this spec over bundling it, because the RS plugin owns its own YANG and plugin-self-containment requires a separate spec.
- No backward compatibility: env vars deprecated, YANG is the canonical surface.

## Consequences

- Operators can tune forward pool and buffer sizes via `set protocols bgp reactor forward-queue-size 128` with validation, commit/rollback, and config backup.
- Seven env vars print deprecation warnings on first use, pointing to the YANG alternative.
- The `envPlumbingTable` pattern is now the established way to promote env vars to YANG: add YANG leaf, add plumbing entry, mark env var deprecated. No consumer code changes needed.

## Gotchas

- The spec listed `ze.fwd.chan.size` default as 64, but the actual env registration default is 256. The YANG default must match the env registration, not the fwdPool internal fallback (which only fires when chanSize <= 0).
- Assumption A-1 ("reactor config struct carries YANG leaves") was wrong. There is no config struct. Values flow through string-based env plumbing: YANG -> `ExtractEnvironment` -> `ApplyEnvConfig` -> `env.Set` -> consumer `env.Get*()`.

## Files

- `internal/component/bgp/yang/ze-bgp-conf.yang` -- 7 new leaves in reactor container
- `internal/component/config/apply_env.go` -- 7 envPlumbingTable entries
- `internal/component/config/environment.go` -- Deprecated markers on 7 env vars
- `internal/component/config/apply_env_test.go` -- plumbing + OS-override tests
- `test/parse/reactor-tuning.ci` -- valid config acceptance
- `test/parse/reactor-tuning-reject.ci` -- forward-queue-size range rejection
- `test/parse/reactor-tuning-reject-buffer.ci` -- read-buffer-size range rejection
- `docs/guide/configuration.md` -- reactor settings table expanded
- `docs/architecture/config/environment-block.md` -- reactor leaf list updated
- `docs/architecture/config/environment.md` -- forward/buffer table with YANG leaf column
