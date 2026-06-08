# 732 -- Masquerade Port Mapping and Flags

## Context

The firewall model already had `Masquerade{Port, PortEnd, Flags}` fields but nothing populated them. Config parsing treated masquerade as presence-only, nft lowering explicitly rejected non-zero fields, VPP silently ignored them, and CLI show displayed bare "masquerade". This spec filled the gap across all five layers.

## Decisions

- Defined our own `MasqFlag*` constants (uint32 bitmask) over reusing kernel `NF_NAT_RANGE_*` values, because the model is backend-neutral and the vendor library takes individual booleans on `expr.Masq`.
- Enforced port/flag mutual exclusion at parse time over deferring to the backend, because it is a kernel-level constraint and the error is clearer at config validation.
- Used `textbuf.Get()` pool for `formatMasquerade` over string concatenation, following the existing `formatLimit` pattern in the same file.
- VPP backend rejects masquerade ports/flags with explicit errors over silently ignoring them, per `exact-or-reject` rule.

## Consequences

- Masquerade with `port-range` or flags now works end-to-end on the nft backend.
- VPP backend is explicit about what it does not support, preventing silent config loss.
- The `parseMasqPorts` helper is specific to masquerade; if redirect or other NAT actions need similar parsing later, consider extracting to a shared helper.

## Gotchas

- `expr.Masq.ToPorts` gates the marshal path: when true, flags are ignored by the kernel. The mutual exclusion check in the parser prevents an operator from accidentally hitting this.
- The nft lowering tests are build-constrained to linux; on darwin they compile-check only.

## Files

- `internal/component/firewall/model.go` -- added `MasqFlagRandom`, `MasqFlagFullyRandom`, `MasqFlagPersistent` constants
- `internal/component/firewall/config.go` -- added `parseMasquerade`, `parseMasqPorts`
- `internal/component/firewall/config_test.go` -- 5 new test functions
- `internal/component/firewall/yang/ze-firewall-conf.yang` -- added leaves to masquerade container
- `internal/plugins/firewall/nft/lower_linux.go` -- rewrote `lowerMasquerade` with port and flag paths
- `internal/plugins/firewall/nft/lower_linux_test.go` -- 3 new test functions
- `internal/plugins/firewall/vpp/verify.go` -- added port/flag rejection in masquerade case
- `internal/plugins/firewall/vpp/verify_test.go` -- 2 new test functions
- `internal/component/firewall/cmd/show.go` -- added `formatMasquerade`
- `internal/component/firewall/cmd/show_test.go` -- 5 new test cases
- `test/firewall/015-masquerade-ports.ci` -- functional test for port mapping
- `test/firewall/016-masquerade-flags.ci` -- functional test for random flag
- `docs/guide/firewall.md` -- updated masquerade action entry
