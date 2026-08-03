# 1152 -- rib-arch-8: General NLRI-Byte Rewrite via ModAccumulator

## Context

The egress-filter `ModAccumulator` (`internal/component/bgp/filterapi/filterapi.go`)
accumulated two per-peer modification kinds: attribute ops (`Op`/`OpCopy`) and
announce→withdraw conversion (`SetWithdraw`). There was no way for a filter to rewrite
the NLRI prefixes themselves. rib-arch-8 adds that third modification kind so an egress
filter can substitute the announced (and withdrawn) NLRI bytes for a destination peer
(e.g. prefix translation).

## Decisions

- **Build the primitive + unit tests only; no `.ci`, no driving surface** (user-approved
  scope, 2026-07-14). Chosen over building a full config/filter surface because no
  in-process filter needs NLRI rewrite today, and external filters already rewrite NLRI via
  the raw wire override (`runEgressPolicyChain` → `exportWireOverride`,
  `reactor_api_forward.go,515`). The ModAccumulator primitive is the cleaner path for a
  future in-process consumer; it is inert until one calls it.
- **Carry the rewrites on the existing `mods` argument, not a new `buildModifiedPayload`
  parameter.** Chosen to avoid churning the ~24 callers: `buildModifiedPayload` already
  receives `*ModAccumulator`, so it reads `mods.NLRIRewrite()`/`mods.WithdrawnRewrite()`
  directly. The legacy explicit `nlriOverride` argument still takes precedence.
- **Rewrite both announce and withdrawn sections (AC-2).** The announce NLRI reuses the
  existing `nlriOverride` substitution slot; the withdrawn NLRI required a new step-1
  substitution (the withdrawn section was previously copied verbatim). Rewriting both keeps
  adj-rib-out consistent: a prefix rewritten on announce is withdrawn under the same prefix.

## Consequences

- A future in-process egress filter can do per-peer prefix translation via
  `mods.SetNLRIRewrite`/`SetWithdrawnRewrite` without rebuilding the whole payload.
- The forward-path gate changed from `mods.Len() > 0` to `mods.HasModifications()` so a
  rewrite-only modification (no attribute ops) still triggers the payload rebuild.
- All changes are inert unless a filter sets a rewrite; no forward-path behaviour changes for
  existing filters (full `filterapi` + `reactor` suites pass unchanged).

## Gotchas

- `SetWithdraw` (announce→withdraw conversion) routes to `buildWithdrawalPayload`, a
  DIFFERENT branch from `buildModifiedPayload`; the NLRI rewrites apply only on the latter.
  A filter that both converts to withdrawal and sets a rewrite gets the withdrawal conversion.
- The rewrite is raw wire NLRI bytes: the filter owns validity (add-path path-ids are part of
  those bytes). An oversized (> 65535) withdrawn rewrite abandons the modification rather than
  truncating.
- `Len()` still counts only attribute ops; use `HasModifications()` to detect a rewrite.

## Files

- `internal/component/bgp/filterapi/filterapi.go` -- ModAccumulator rewrite fields + methods
- `internal/component/bgp/reactor/forward_build.go` -- rewrite application (announce + withdrawn)
- `internal/component/bgp/reactor/reactor_api_forward.go`, `forward_rs.go` -- HasModifications gate
- `internal/component/bgp/filterapi/filterapi_test.go`, `reactor/forward_build_test.go` -- unit tests
