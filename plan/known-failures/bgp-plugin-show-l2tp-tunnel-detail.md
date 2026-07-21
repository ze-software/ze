### `ze-test bgp plugin 458 show-l2tp-tunnel-detail` -- deterministic on darwin, pre-existing, non-BGP

Observed 2026-07-21 on darwin: `bin/ze-test bgp plugin 458`
(show-l2tp-tunnel-detail) fails deterministically in isolation (RC=1) and in full
`ze-verify-changed` runs. This is an L2TP command-surface test.

NOT caused by `spec-fixit-bgp-concurrency-races`: that changeset touches only
`internal/component/bgp/**` and `internal/core/bgp/attribute/**` -- zero L2TP
code -- so it cannot regress an L2TP command test. Pre-existing by construction.

Environmental scope UNVERIFIED and NO root cause asserted (not traced -- outside
this session's area). Deterministic on this darwin host; Linux/CI status not
checked. Triage owner is the L2TP command surface (`internal/plugins/l2tp*`,
`internal/component/l2tp`), not BGP. Confirm darwin-specificity under QEMU/Linux
before attributing.
