### `reload` suite -- 6 iface tunnel/wireguard tests time out without CAP_NET_ADMIN (unprivileged sandbox)

Observed 2026-07-09 (rootless Linux sandbox; `unshare --net` returns
"Operation not permitted", no CAP_NET_ADMIN). Six `.ci` tests in the `reload`
suite time out (20s each), blocking a native `make ze-verify` /
`ze-functional-test` from completing:

- `test-tx-iface-tunnel-modify-key` (id 26)
- `test-tx-iface-tunnel-remove` (id 27)
- `test-tx-iface-wireguard-invalid-bad-public-key` (id 29)
- `test-tx-iface-wireguard-invalid-no-private-key` (id 30)
- `test-tx-iface-wireguard-modify` (id 31)
- `test-tx-iface-wireguard-remove` (id 32)

Root cause is environmental, not a product regression. All six put a real
Linux interface (gre tunnel or wireguard) in the **boot** config. At startup
the iface plugin's `OnConfigure` handler
(`internal/component/iface/register.go:395`, error return at `:416`) calls
`applyConfig`, which invokes the netlink create
(`internal/plugins/iface/netlink/tunnel_linux.go:42-51`,
`internal/plugins/iface/netlink/wireguard_linux.go:33-41`). Without
CAP_NET_ADMIN that returns EPERM ("operation not permitted"). Unlike the
reload path, a startup create failure is FATAL: `OnConfigure` returns the
error, which fails the interface plugin's Config-stage handshake in
`deliverConfigRPC` (`internal/component/plugin/server/startup.go:691,713`),
cascading to "config-path plugin startup failed". The daemon never serves
BGP, so the peer never connects and the test's route+EOR expectation times
out.

The four sibling tests that apply the interface on **reload** rather than at
boot (`test-tx-iface-apply` id 23, `test-tx-iface-bgp-chain` id 24,
`test-tx-iface-tunnel-create` id 25, `test-tx-iface-wireguard-apply` id 28)
PASS natively: the daemon boots with no interface, the BGP peer receives
route+EOR (assertion satisfied) before the failing reload, and a reload
create failure rolls back without killing the already-running daemon.

These are legitimately privilege-dependent (create-then-modify/remove a real
netdev), so they pass under QEMU-root / a privileged host, not in this
sandbox. Per `ai/rules/qemu-testing.md` the six boot-config tests arguably
belong on `option=needs-linux` rather than the current
`option=skip-os:value=darwin`, BUT (a) `needs-linux` only skips on non-Linux
GOOS, so it would NOT change native-Linux behavior here (still run, still time
out), and (b) reclassifying adds them to `ze-qemu-needs-linux-test`, whose
Alpine VM kernel must carry the gre/wireguard modules -- `runtime.config` has
`CONFIG_WIREGUARD`/`CONFIG_NET_UDP_TUNNEL`/`CONFIG_TUN` but no explicit GRE
symbol, so this needs QEMU verification before flipping. Classification change
deferred pending that verification; the native timeout is documented here so
`ze-verify` in an unprivileged sandbox is not treated as a structural red.
Owner: whichever session runs these under QEMU-root and confirms the kernel
modules.
