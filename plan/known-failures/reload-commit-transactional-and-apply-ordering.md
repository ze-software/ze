### `reload` suite -- `commit-transactional` and `test-config-apply-ordering-delete` fail in an unprivileged sandbox

Observed 2026-07-22 (rootless Linux sandbox, same environment as
`reload-iface-tunnel-wireguard-cap-net-admin.md`). Two `reload` tests fail that
are NOT covered by that entry:

- `commit-transactional` (id 2) -- `file_check_failed`:
  `expect=file:path=meta/config/rollback: read failed: open
  /tmp/ze-tmpfs-<n>/meta/config/rollback: no such file or directory`
- `test-config-apply-ordering-delete` (id 18) -- times out at 60s

**Attribution.** Both were confirmed pre-existing, not caused by the ike
`OnConfigVerify` wiring added in this session. Method: the new callback body in
`internal/component/ike/engine/register.go` was temporarily replaced with a
no-op, `ze` was rebuilt, and both tests were re-run in isolation. Both still
failed identically. The hook was then restored and
`test-tx-ipsec-eap-tls-requires-ca` re-confirmed PASS.

Root cause not yet traced. Test 18's 60s timeout and its "apply ordering with
delete" subject put it in the same family as the six CAP_NET_ADMIN timeouts, so
it is likely the same environmental cause (a real interface delete that cannot
proceed unprivileged), but that has NOT been verified and should not be assumed.
Test 2 is a different shape: the rollback artifact is simply absent, which is a
storage/transaction question rather than a netlink one.

**Effect.** A native `make ze-verify` / `ze-functional-test` cannot reach green
in this sandbox on the `reload` suite. The other 30 tests pass.

**Next step.** Trace test 2 first: it fails in 2.7s with a concrete missing-file
assertion, so it is cheap to diagnose and is the one most likely to be a real
product gap rather than an environment limitation.
