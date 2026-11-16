# Handover: three container phases designed and not yet written

Written 2026-11-16. Companion to `42-container-support-implementation.md`,
which says why: three agents designed their phase, then the edit hook refused
every source edit in the worktree. Each design below was worked out from the
code by the agent that owned it. The full originals are in the worktree's
scratch (`tmp/handoff-runtime.md`, `tmp/handoff-network.md`,
`tmp/handoff-image-store.md`), which lasts only as long as the worktree. This
file keeps what a new agent needs if the scratch is gone. A design here is a
claim: read the producer before applying it.

## Runtime, phase 4: the system-call filter

The spec orders the filter as phase 4 and the device allowlist as phase 6.

Source of the default profile: Moby's default profile,
`github.com/moby/profiles`, `seccomp/default.json`, commit
`f862455628f78f84c937e967e4a03902fb1dee11`, sha256
`3320527d4568265ac97a077609b1bfd4cfd0d789eeb9239b7e094b1526675406`,
Apache-2.0, no NOTICE file in that repository. A copy of the licence and a
README naming source, commit, digest and what was derived belong under
`third_party/moby-seccomp-profile/`, with a paragraph in the notice file.

Files, all in `internal/plugins/container/seccomp/`, Linux only:

| File | Holds |
|------|-------|
| `table.go` | One table per architecture: the architecture's name, its audit constant, a map from call name to number; `nativeTable()` picks by `runtime.GOARCH` and refuses another |
| `table_x86_64.go`, `table_aarch64.go` | The two maps, generated from the vendored `golang.org/x/sys/unix/zsysnum_linux_{amd64,arm64}.go` (385 and 328 calls). Not named `_amd64.go` / `_arm64.go` as the spec says: that suffix is a build constraint, so a host could compile and test only its own table. The spec's file list is to be corrected |
| `profile.go` | `DefaultProfile(bounding)`: default action errno EPERM, rules from the source profile keeping only names at least one table holds, selected by capability as the source does. The source's rule for kernel 4.8 is applied unconditionally: a container starts only through the clone into a control group, which needs kernel 5.7 |
| `filter.go` | `assembleFilter(profile, table)` with the vendored `golang.org/x/net/bpf`. Refuses: flags, a listener path or notify action, another architecture, an argument index above 5, an unknown operator or action, a jump above 255, more than 4096 instructions. Layout: check the architecture (kill the process otherwise), answer ENOSYS above the largest known number, then one block per rule in profile order, first match decides, then the default. 64-bit compares on two 32-bit words, little-endian |
| `load_linux.go` | The raw `seccomp` call with the flag that applies the filter to every thread; a positive return is a thread that could not sync, an error |
| `register_linux.go` | The seam installs the filter; the "not built" refusal is deleted |

`describe_linux.go` then sets the profile, and
`TestDescriptionHasTheDefaultConfinement` gains it.

Tests: six filter tests running the assembled program in the bpf package's VM
(it reads big-endian words, so the test packs fields accordingly); a table
test comparing both tables with the vendored files in both directions;
`TestDefaultProfileNamesOnlyKnownCalls`; a pin that `unshare` is refused
without the admin capability and that call number 470 lies above the largest
known number on both tables. The fixture program gains checks of `unshare(0)`
and raw call 470, judged against the host. Guest test:
`container-default-confinement.ci`, pinning thirteen capabilities (mask
`00000000a00425fb`) until phase 6 returns the fourteenth (`a80425fb`).

## Network, phase 1: engine requests only

The spec's phase 1 "uses no container word". It adds four engine requests, the
SDK methods and a test plugin. The participant, config nodes, verify rules and
doctor codes are phases 4 and 6. Open question for the session: the Wiring
Test table's rows 5 to 9 need the network package, which the phase order puts
in phase 4.

Facts read at the producer: `pluginserver.RegisterEngineOp` takes its
namespace from the calling package (`internal/component/iface/cmd` gives
`ze-iface`, `internal/component/firewall` gives `ze-firewall`); the precedent
is `internal/component/sysrib/register_route_view.go` with
`pkg/plugin/sdk/sdk_route_view.go`; engine requests are not in the
wire-methods snapshot; `firewall` may import `plugin/server` without a cycle.

| File | Holds |
|------|-------|
| `pkg/plugin/rpc/owned_devices.go` | Methods `ze-iface:owned-bridges-replace`, `ze-iface:owned-link-create`, `ze-iface:owned-link-remove` and their input and output types. The link is keyed by the owner's own record name, so `iface` carries no container word |
| `pkg/plugin/rpc/firewall_tables.go` | Method `ze-firewall:tables-replace`, input a firewall section, output the table names |
| `pkg/plugin/sdk/sdk_owned_devices.go`, `sdk_firewall_tables.go` | The four SDK methods |
| `internal/component/iface/device_owner.go` | `MustGrantOwnedDeviceNames(owner, prefixes...)` called from a plugin's `init()`, and the refusal for a caller with no grant. Test `TestOwnedBridgeRequestRefusesCallerWithoutGrant` |
| `internal/component/iface/cmd/owned_devices.go` | Registers the three requests, admitted at the configure stage; phase 1 answers a "not implemented" refusal after the grant check |
| `internal/component/iface/yang/ze-iface-owned-api.yang` | A module of its own: `TestAPIRPCInputMatchesCommandTree` requires every request of `ze-iface-api.yang` to be bound to a command path, and these are bound to none |
| `internal/component/firewall/contribution.go`, `register_contribution.go` | A contributed table must be reserved by the caller. Test `TestContributionRefusesUnreservedTable` |
| `internal/component/firewall/yang/ze-firewall-api.yang` | The `tables-replace` request |
| `internal/test/plugins/fakeowner/` | Test plugin granting itself the prefix `zfo` and the table `ze_fakeowner`; it makes the four requests and serves each answer through `show fakeowner outcome` |

Guest test drafts: `container-network-wiring-external.ci`,
`container-network-wiring-internal.ci`, `container-link-wiring-external.ci`,
`container-table-wiring-external.ci`, red on the stub. Pages:
`docs/architecture/api/process-protocol.md`,
`docs/plugin-development/protocol.md`, `docs/functional-tests.md`.

## Image store, phase 1: wiring

No config node lands in phase 1: `image` is mandatory and would refuse the
runtime's six test configs. No new Go module is needed, for this phase or for
the store: the zstd decoder is vendored, archives and gzip are the standard
library.

Findings that change the plan:

1. A plugin command cannot declare its own time bound: the command
   declaration has no such field and every plugin command gets 30 seconds. The
   one-hour bound of fetch and update needs a new Stage 1 field, a change to
   the public plugin protocol with its two pages. Proposed wire key `timeout`,
   whole seconds. To confirm with the owner, and to land as a commit of its
   own in plugin infrastructure before the store's phase 5.
   `TestFetchCommandDeclaresItsBound` stays red until then.
2. The runtime resolves with the instance name alone; the phase that adds the
   `image`, `storage` and `volume` nodes fills the request.
3. The `Store` seam is `Resolve` and `Hold` only, and the runtime's fake
   implements exactly that. The image commands go in a separate interface,
   `ImageOperations`, which a store may also be.
4. The test store registers from `init()`, so a test build linking the real
   store would register twice and panic. The test store is to be chosen at the
   engine entry when `ze.test.container.store` is set, which all six runtime
   tests set.
5. The plugin's command handler refuses every command but restart by a direct
   comparison; it becomes a table lookup.

Ten command nodes, in `internal/plugins/container/yang/ze-container-cmd.yang`:

| Path | Wire method |
|------|-------------|
| `show container image list` | `ze-container:show-image-list` |
| `show container store` | `ze-container:show-store` |
| `show container volume list` | `ze-container:show-volume-list` |
| `create container image reference <image>` | `ze-container:create-image` |
| `update container image name <container>` | `ze-container:update-image` |
| `update container image rollback name <container>` | `ze-container:update-image-rollback` |
| `delete container image digest <digest>` | `ze-container:delete-image` |
| `delete container image all` | `ze-container:delete-image-all` |
| `delete container volume name <volume> instance <container>` | `ze-container:delete-volume` |
| `clear container layer name <container>` | `ze-container:clear-layer` |

Files, in an order that keeps the tree compiling:
`internal/plugins/container/storeseam_operations.go` (the interface and its
answer types), `storeseam_engine.go` and the changed `storeseam_zetest.go`
(the store chosen at the engine entry), `container.go`, `image_cmd.go` (the
ten declarations, proxies and handlers), `register.go`, `doctor_store.go`
(eleven doctor codes this child owns), the new package
`internal/plugins/container/imagestore/` (`register.go`, `store.go` answering
"not implemented in this build", `resolve_test.go` with
`TestResolveAbsentNamesReference`), `image_cmd_test.go`, three state keys in
`pkg/zefs/keys.go`, a data directory in `internal/core/paths/paths.go`
(`ze.data.dir`, `TestDataDirFromBinary`), the gate line and the blank import,
the guest test `container-image-fetch-lists-digest.ci`, and the pages
(`docs/architecture/container/runtime.md`, a new
`docs/architecture/container/image-store.md`, the state key table, the
environment page).
