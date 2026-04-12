# 566 -- iface-wireguard

## Context

Ze had no WireGuard interface support; the iface component only modelled
ethernet/dummy/veth/bridge/loopback/VLAN/tunnel. This spec added a new
top-level `wireguard` list under `container interface`, modelled on the
`spec-iface-tunnel` pattern but with three substantial departures: keys
are stored inline in the config via the existing `ze:sensitive` + `$9$`
pattern (not file paths or env vars), peer state is reconciled in-place
per-peer via wgctrl's `ReplacePeers: true` so handshake counters are
preserved across reload, and the configuration surface exposes a full
declarative shape for WireGuard (listen-port, fwmark, private-key,
nested peer list with public-key, preshared-key, endpoint, allowed-ips,
persistent-keepalive) matching VyOS's form. The `wireguard` Linux kernel
module is configured via the `wg` genetlink family, not rtnetlink, which
required pulling in `golang.zx2c4.com/wireguard/wgctrl` and its
transitive `mdlayher/genetlink` + `mdlayher/netlink` dependencies.

## Decisions

- **`ze:sensitive` + `$9$` encoding for keys**, not file paths or env
  vars. The parser already auto-decodes `$9$` on load
  (`internal/component/config/parser.go:127`) and dumps auto-encode on
  output (`cmd/ze/config/cmd_dump.go:132`); wireguard joins this path
  for free. JunOS-compatible obfuscation, same posture as BGP MD5
  passwords and other sensitive leaves -- config file permissions are
  the actual protection. File-path keys were rejected as a cross-cutting
  split from the existing sensitive-leaf idiom; env vars were rejected
  because `env.MustRegister` wants static names at package init time and
  does not fit per-interface dynamic secrets.
- **Split `interface-physical` into `interface-common` + `interface-l2`**.
  WireGuard is L3 with no MAC, so it uses `interface-common` (description,
  mtu, os-name, disable) while ethernet/veth/bridge/tunnel continue using
  `interface-l2` (common + mac-address). The YANG walker sorts children
  alphabetically so the split is cosmetically invisible in config dumps.
  Tunnel stays on `interface-l2` for now even though L3 tunnel cases also
  lack a MAC; per-case tunnel MAC handling is a deferred follow-up.
- **Spec-level reconcile via `wireguardSpecEqual`**, not per-peer diff.
  The original spec text called for `diffWireguardPeers(previous,
  current) []wgtypes.PeerConfig`, but once Phase 7 implemented
  `ConfigureWireguardDevice` with `wgtypes.Config{ReplacePeers: true}`
  the per-peer diff became redundant: the kernel matches unchanged
  peers by public-key and preserves their handshake state, so
  "apply entire spec on every change" produces the same kernel outcome
  as a narrower diff at a fraction of the code. `applyWireguards`
  skips no-op reloads via a spec-equality check that treats peers as
  an unordered set keyed by public-key.
- **`ze:listener` extension gains a Protocol field (TCP/UDP)**.
  `ListenerEndpoint` in `internal/component/config/listener.go` grew a
  `Protocol string` field (`ProtocolTCP` / `ProtocolUDP`) so wireguard's
  UDP ports do not clash with TCP services on the same port. All eight
  existing services (web, ssh, mcp, looking-glass, prometheus,
  plugin-hub, api-rest, api-grpc) are annotated as TCP. A new
  `collectWireguardListeners` helper walks `interface.wireguard` list
  entries because wireguard uses a flat `leaf listen-port` rather than
  the `server` sub-list shape the other services use. Replacing the
  hardcoded `knownListenerServices` list with a YANG-walk discovering
  every `ze:listener`-marked node in the schema was deferred to a
  follow-up spec.
- **Full `wgctrl` vendor drop**, not hand-rolled genetlink. wgctrl is
  authored by the WireGuard authors (Donenfeld, Layher), pure Go,
  ~500 KB total vendor footprint including transitive
  `mdlayher/genetlink` + `mdlayher/netlink`, and already used by
  Tailscale / Flannel / Cilium. Hand-rolling would have been ~800 LOC
  of genetlink marshaling under ze's maintenance burden for zero
  upside. D4 approved the new dependency up front.

## Consequences

- New code paths: `parseWireguardEntry` -> `applyWireguards` ->
  `Backend.CreateWireguardDevice` (netlink.LinkAdd) +
  `Backend.ConfigureWireguardDevice` (wgctrl.Client.ConfigureDevice).
  Reconciliation is in-place per peer via ReplacePeers: unchanged peers
  preserve handshake state, added peers join, removed peers are evicted.
  New wireguard entries get a CreateWireguardDevice before the
  Configure call.
- `zeManageable` returns true for `wireguard` so Phase 4 reconciliation
  deletes wg netdevs not in config on reload (same as tunnels).
- `ze init` discovery: `DiscoverInterfaces` walks the wireguard netdevs,
  calls `GetWireguardDevice` to populate `DiscoveredInterface.Wireguard`,
  and `generateInterfaceConfig` emits a complete wireguard block with
  `secret.Encode`-passed private-key and preshared-key. Peer names are
  synthesized as `peer0`, `peer1`, ... because the kernel only tracks
  peers by public-key; operators rename via `ze config edit`.
- The `wgtypes` subpackage was vendored in Phase 1 as a scaffolding
  anchor (`type WireguardKey = wgtypes.Key`) so `go mod tidy` would
  not prune the dependency before Phase 4 had real usage. Phase 7
  extended the vendor drop to the full wgctrl client plus transitive
  deps (genetlink, netlink). Vendor growth across the two drops was
  ~540 KB.
- Future WireGuard enhancements (dynamic YANG-walk listener collector,
  per-case tunnel MAC handling, `ze init` loading the ifacenetlink
  backend so discovery actually works in production) are recorded as
  deferrals.

## Gotchas

- **`ze init` does not load the ifacenetlink backend** in production,
  so `DiscoverInterfaces()` returns "no backend loaded" silently. Phase
  9's wireguard discovery emission code is correct but never fires in
  practice until that pre-existing issue is fixed. Noted in the
  Phase 9 commit message body.
- **YANG walker alphabetizes children** (`flattenChildren` via
  `sortedKeys` in `yang_schema.go`). My `/ze-review` initially raised
  an ISSUE that splitting `interface-physical` would shuffle leaf
  order for every ethernet/dummy/veth/bridge/tunnel entry in config
  dumps, then discovered that YANG-derived schemas always alphabetize
  regardless of grouping structure. The split turned out to be
  cosmetically invisible. `TestSerializeSetSchemaOrder` passes only
  because it uses a hand-built Go schema (`testSchema()`), not a
  YANG-derived one.
- **Go module tidy prunes unused vendor deps.** Phase 1's first attempt
  did `go get wgctrl@latest && go mod tidy && go mod vendor` without an
  actual import in source, which pruned the dep before it reached
  `vendor/`. Fix: always add a real import in source BEFORE tidying.
  Phase 1 landed a minimal scaffold file (`type WireguardKey =
  wgtypes.Key`) specifically to anchor the vendor drop.
- **`go mod vendor` only vendors packages reachable from source imports.**
  Phase 7's commit script enumerated `vendor/golang.zx2c4.com/wireguard/wgctrl`
  but missed `vendor/golang.zx2c4.com/wireguard/ipc/namedpipe` and the
  parent `wireguard/LICENSE` file, both of which were listed in
  `vendor/modules.txt` but untracked. The user caught this in a
  follow-up `vendor(wireguard): restore ipc/namedpipe files` commit.
  Lesson: when committing a vendor drop, `git add vendor/` as a
  directory (after verifying via `go mod verify`) rather than
  enumerating subdirectories piecemeal.
- **Formatter-induced import pruning during large edits.** Several
  Phase 7 Write/Edit operations removed newly-added imports
  (`wgctrl`, `wgtypes`, `net`, `time`) because no code in the edited
  file used them yet. Fix: add imports AND usages in the same edit,
  never in separate passes, and when the formatter strips an import,
  add the usage immediately rather than re-adding the import.
- **Unused-type linter flags exported types only leniently.** Phase 4's
  first attempt put `type wireguardEntry struct` + parser in
  `wireguard.go`, triggering `unused` because nothing in the package
  referenced the unexported type yet. Fix: move unexported type
  definitions to the file that contains the caller (`config.go`)
  while keeping exported types in the domain file (`wireguard.go`),
  matching the tunnel pattern. Reverse: exported types with no local
  references are fine because external packages might consume them.
- **JunOS `$9$` is obfuscation only.** Operators must be explicitly
  told: anyone with read access to the config file can trivially
  recover the plaintext key via `secret.Decode`. Documented in
  `docs/features/interfaces.md` "Key material and $9$ encoding",
  same posture as BGP MD5.
- **Config-parser tree auto-renames duplicate list keys** with `#N`
  suffixes (`internal/component/config/tree.go:227`). Two `peer
  site1 { ... }` blocks in the same wireguard become `site1` and
  `site1#1` in the tree map. The parser no longer sees a duplicate,
  so AC-9 ("reject duplicate peer names") cannot be enforced at
  parseWireguardEntry without a YANG-parser change. Recorded as a
  deferral.
- **`/ze-review` false positives from shell-hook bugs.** The
  `validate-spec.sh` hook's code-block regex (`\`\`\`(go|c|...)`)
  is interpreted by GNU grep as a start-of-line assertion followed
  by word alternation, so it flags any line starting with a word
  beginning with `c`/`go`/`python`/etc. My spec had a line starting
  with "cryptographic" which the hook reported as a "code block".
  Worked around by rewording the line; filed as a spec-validator
  bug for later.
- **Reload `.ci` tests are wiring tests, not kernel-state tests.**
  The test runner does not grant CAP_NET_ADMIN and does not
  guarantee the wireguard kernel module, so `netlink.LinkAdd` /
  `wgctrl.ConfigureDevice` calls fail silently in the test
  environment. Reload tests verify parser + dispatch + daemon
  survival via a BGP-peer heartbeat. Kernel-state round-trips
  (AC-13/14/15 as observable behavior, not just wiring) are
  covered by the integration tests under `internal/plugins/
  ifacenetlink/wireguard_linux_test.go`, gated on `//go:build
  integration && linux` and requiring CAP_NET_ADMIN plus
  `/sys/module/wireguard`.

## Files

- `internal/component/iface/schema/ze-iface-conf.yang` -- split
  groupings, new `list wireguard` with nested `list peer`, `ze:sensitive`
  on private-key and peer preshared-key, `ze:listener` on the list entry
- `internal/component/iface/wireguard.go` -- WireguardKey alias,
  WireguardSpec, WireguardPeerSpec exported types
- `internal/component/iface/config.go` -- wireguardEntry,
  parseWireguardEntry, parseWireguardPeer, wireguardSpecEqual,
  wireguardPeerEqual, indexWireguardSpecs, applyWireguards branch in
  applyConfig, zeManageable extension
- `internal/component/iface/backend.go` -- CreateWireguardDevice,
  ConfigureWireguardDevice, GetWireguardDevice interface methods
- `internal/component/iface/discover.go` -- zeTypeWireguard, extended
  infoToZeType, DiscoverInterfaces populates Wireguard spec via
  GetWireguardDevice
- `internal/component/iface/iface.go` -- DiscoveredInterface.Wireguard
  field
- `internal/plugins/ifacenetlink/wireguard_linux.go` -- netlink.LinkAdd
  for netdev create, wgctrl.Client.ConfigureDevice for peer config,
  deviceToSpec + buildWireguardConfig + buildPeerConfig translation
- `internal/component/config/listener.go` -- Protocol field on
  ListenerEndpoint, listenerService.protocol, collectWireguardListeners,
  protocolLabel helper, cross-protocol conflict check
- `cmd/ze/init/main.go` -- emitWireguardBlock helper in
  generateInterfaceConfig, secret.Encode for private-key and
  preshared-key
- `test/reload/test-tx-iface-wireguard-{apply,modify,remove,invalid-*}.ci`
  -- 5 reload wiring tests
- `internal/plugins/ifacenetlink/wireguard_linux_test.go` -- 5
  integration tests gated on CAP_NET_ADMIN + wireguard module
- `internal/component/iface/wireguard_test.go` -- 9 parser unit tests
- `internal/component/iface/config_test.go` -- 7 applyWireguards
  reconcile tests with fakeBackend
- `internal/component/iface/schema_test.go` -- 2 YANG schema sensitive/
  structural tests
- `internal/component/config/listener_test.go` -- 4 listener protocol
  + wireguard collector tests
- `cmd/ze/init/config_test.go` -- 2 generateInterfaceConfig wireguard
  emitter tests
- `docs/features.md`, `docs/features/interfaces.md` -- capability table,
  new "WireGuard Configuration" section
- `go.mod`, `go.sum`, `vendor/golang.zx2c4.com/wireguard/**`,
  `vendor/github.com/mdlayher/genetlink/**`,
  `vendor/github.com/mdlayher/netlink/**` -- new dependency drop
