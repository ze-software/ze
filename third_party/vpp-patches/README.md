# Patched VPP for the PPPoE mechanism lab

This directory owns the retained diffs and reproducible build recipe for the
PPPoE VPP mechanism proof. No copy of the VPP source tree is kept in Ze.

The base is FD.io VPP **v25.10**, commit
`cbba0451bb0af02a3ab8e163f6f99062782258e6`. The Dockerfile verifies the
SHA256 of that commit's codeload archive before extraction. The five VyOS
patches are unmodified originals from
[`vyos/vyos-vpp-patches`](https://github.com/vyos/vyos-vpp-patches/tree/9c48583794af21d9aca5406e5473a805d7904bd1/patches/vpp),
commit `9c48583794af21d9aca5406e5473a805d7904bd1`, and retain their upstream
copyright and attribution.
`LICENSE` is copied verbatim from that VPP source commit and installed in
the image at `/usr/share/doc/vpp/LICENSE`.

| Patch | Purpose |
|-------|---------|
| 0013 | Per-access-interface control mapping and address-independent session creation |
| 0014 | Punt IPv6 router solicitations, router advertisements and DHCPv6 to the control plane |
| 0018 | Make the ingress policer available on IPv4/IPv6 unicast and multicast arcs |
| 0025 | Remove the incorrect blanket PPPoE feature from the control interface |
| 0031 | Dump the control/data interface bindings |
| 0098 (Ze) | Provide seeded scalar CRC32C helpers for native targets without a CRC ISA, with a standalone C regression |
| 0099 (Ze) | Validate logical Ethernet/VLAN/PPPoE and IPv6 control lengths before classification or tracing |
| ze/0100 (Ze) | Add a lab-only regression of control binding validation, leaving the binding producer unchanged |
| ze/0101 (Ze) | Validate control binding software indices and refuse deletion unless both mappings identify the exact pair |
| ze/0102 (Ze) | Exercise the three receive lanes and control dispatch through the native graph |
| ze/0103 (Ze) | Add native process/resource tokens and guard session, CP, interface, route, gate and policer mutations |

The Dockerfile verifies and extracts the pinned source, then applies the five
original patches, `0098` and `0099` in lexical order with zero fuzz. It compiles
that source and runs the scalar CRC regression before applying the binding
regressions and producer repairs under `ze/` in lexical order in a separate
layer. CMake then reuses the original configured options and incrementally
builds those changes, including `0103`'s ownership API. Only after those shared layers can the
optional metadata assignment proof apply; installation follows its incremental build. A rejected
patch stops the build. Auto-discovery is left disabled:
Ze owns session creation and teardown, so the lab configuration must not set
`pppoe { enable-auto-discovery }`.

Run `./le --name session build vpp run`. The registered image name is
`ze-vpp-pppoe-lab`; the answer identifies the immutable built image. The
builder releases its temporary retention tag and preserves that reusable
cache tag. The narrow build context contains this directory alone.
The native action handles Ctrl-C and SIGTERM through its build context;
retention cleanup uses an uncancelled context so cancellation cannot interrupt
removal of the build's temporary tag.

Run `./le --name session build vpp update` to regenerate Ze's patched PPPoE
and Feature Go bindings. `Dockerfile.api` reads the commit and SHA256 pins from the main
Dockerfile through the native action. It applies the same complete baseline
catalog as the runtime recipe: root `*.patch` files in lexical order, followed
by `ze/*.patch` files in lexical order, all with zero fuzz. It then runs upstream
`vppapigen` with Python to emit JSON.
This recipe does not compile VPP and does not depend on the build CPU.
The action sends only that recipe and the baseline patch inputs to Docker.
It excludes the `proof/` mutations. The `0103` ownership messages come from this
same baseline. On 2026-11-16, two runs against the complete catalog generated
byte-identical PPPoE and Feature API and RPC files. The source commit and
archive digest in both generation reports match the pins above. This proves
repeatable binding generation; runtime compatibility still needs the provider's
integration tests.
GoVPP's `binapi-generator@v0.13.0` then generates the API and RPC files.
Feature comes from `src/vnet/feature/feature.api` in that same source tree.
The input includes the pinned `memclnt` API because RPC generation resolves
the control-ping request and reply even when only PPPoE and Feature are published.
The generator runs under Ze's Go toolchain with `CGO_ENABLED=0`.
<!-- source: internal/le/build/vpp/api_recipe.go -- apiSourcePins and apiBuildRecipe -->

The action validates the patched session field, CP pair fields and binding dump,
plus Feature enable/disable and readback field types. It parses and checks all
four files before publishing `pppoe.ba.go` and `pppoe_rpc.ba.go` in
`internal/component/l2tp/pppoe/dataplane/vpp/binapi/pppoe/`, and `feature.ba.go`
and `feature_rpc.ba.go` in the sibling `binapi/feature/` directory.
Imports of unchanged VPP types use GoVPP's existing `binapi` packages.
The action refuses output files without GoVPP's generated header and refuses
symbolic links in the output path. Other package files stay unchanged.
Private build inputs, JSON, extra generated type packages and temporary image
retention tags are removed after success or failure. Docker retains the
reusable `ze-vpp-pppoe-api` cache image. Its contents are API JSON only.
<!-- source: internal/le/build/vpp/api.go -- generateAPI -->
<!-- source: internal/le/build/vpp/api_publish.go -- apiCheckOutput and apiPublish -->

`./le --name session build vpp run variant missing-frame-metadata` builds the
counterfactual as `ze-vpp-pppoe-lab-missing-frame-metadata`; `variant baseline`
and omission both select the normal image. These are the only accepted values.
The structured answer records `proof-variant`, the immutable image ID and the
recipe. The image also contains `/usr/share/doc/vpp/proof-variant` so the proof
can check the artifact's identity rather than trust its cache tag.

Both variants share the pinned source, the seven-patch compile and the late
`ze/` regression and producer layer. Only after those cached layers
does the counterfactual apply `proof/missing-frame-metadata.diff` and
incrementally rebuild. That test-owned
diff removes only `pppoe_frame_parse`'s successful output assignment; the
canonical `0099` diff is unchanged. The baseline must pass all 31 runtime
cases, and the counterfactual must fail the explicit metadata oracle. Neither
the counterfactual image nor its source mutation is a product configuration.
The applied replacement carries `// MUTATION-APPLIED` only in the deliberately
broken image's source. The recipe checks the source is nonempty and verifies
the exact replacement line before compiling it. Keeping this test fixture in
Ze does not apply that cut to the baseline or to Ze's live product source.

This Phase 0 image builds only `af_packet`, `linux-cp` and `pppoe`, using
upstream's `VPP_PLUGINS` CMake option. `VPP_BUILD_NATIVE_ONLY=ON` compiles only
for the build host's CPU, avoiding unused multiarch variants. It uses one
compile worker to fit the lab's two-core, two-GiB Docker host. Rebuild this
CPU-specific image inside the target Ze-kernel guest before that guest's lab
run; an image built on another CPU is not a portable artifact.
It is **not** an appliance or DPDK
package. Building it does not prove a PPPoE session works; packet captures,
session readback and the remaining Phase 0 assumptions require the lab run.

`0098-crc32-portable-scalar-helpers.patch` completes the software CRC32C path
used when the native compiler target lacks CRC instructions. The upstream
connectionless session hash calls scalar helpers unconditionally, but the
pinned header provided them only for hardware CRC targets. The retained diff
preserves seeded CRC state and least-significant-byte-first word processing,
leaves the intrinsic marker unset on the software path, and keeps native CPU
flags unchanged.

The standalone `test_crc32c_scalar` executable checks 27 literal Castagnoli
raw-state vectors across 16-, 32- and 64-bit words, three seeds and high-bit
values. Two `123456789` checks cover byte and mixed-width chaining against
the standard finalized CRC32C value `0xe3069283`. The Docker build preserves
the executable's nonzero failure status and requires its complete answer to be
`VPP CRC32C scalar helpers: 29 cases passed`. This executable is not installed.
The native ARM build first stopped at the missing helpers. With this repair,
the source build completed and the executable passed all 29 cases on
2026-11-15 in the Ze-kernel Docker guest.

`0099-pppoe-bound-frame-parsing.patch` is Ze's retained hardening diff after
the five upstream patches. The adopted `pppoe_input_node` previously read the
PPP protocol before checking the logical frame length, and
`is_control_plane_packet` read IPv6 and transport fields beyond a truncated
payload. The control dispatcher's trace also read a PPP protocol from discovery
frames, where that field does not exist. No crash has been demonstrated.

All three receive lanes and the control dispatcher now share
`pppoe_frame_parse`. It checks the complete buffer chain against the declared
PPPoE payload, consolidates split headers only when necessary, distinguishes
the six-byte discovery header from a session's PPP protocol, and validates
IPv6, ICMPv6 and UDP header bounds before their fields are read. A rejected
frame increments the plugin's `malformed PPPoE frame length` error; its trace
uses only the initialized parser result.

The lab recipe enables `VPP_PPPOE_FRAME_TESTS`; upstream builds with this diff
leave it off by default. On the running lab VPP,
`vppctl test pppoe frame-bounds` exercises the actual parser with real VPP
buffers, including truncated declarations, poisoned padding, valid discovery,
VLAN, split headers, IPv6 and transport boundaries. Every accepted case also
checks explicit session ID, PPP protocol, Ethernet offset, inner EtherType and
code values, including tagged and split discovery, LCP and IPv6 frames. It must
print `PPPoE frame bounds: 31 cases passed`. A mutant that removes the parser's
output assignment must fail these metadata assertions; that discrimination
has not run yet. This runtime proof and malformed-frame
receive assertions must pass before the image serves as Phase 0 evidence.

The optional `ze/0102` command, `vppctl test pppoe receive-bounds`, separately
submits exact one-buffer and four-buffer frames to the real input node, then
one directly to control dispatch. Four vectors force both paired input lanes;
the singleton and remainder exercise scalar input. It retains inspection
references while the normal graph drops the buffers, then checks all six
BAD_LENGTH errors, exact native drop-counter deltas, initialized formatted
trace metadata and unchanged session allocation. Success requires
`PPPoE receive bounds: 4 paths passed`. It runs only in the single-thread lab
with automatic session discovery and trace filters off. This proof is authored;
zero-fuzz application, compilation and execution remain owed. The existing 31
parser cases and their metadata counterfactual remain unchanged.

The same lab test option enables `vppctl test pppoe cp-binding`. It creates real
loopback and VLAN software interfaces and calls the actual binding API. Its
first case deletes an absent pair whose two indices lie inside allocated
mapping vectors; the adopted producer returned success in the native runtime
proof, reporting `unmapped deletion was accepted` while VPP stayed running.
The later cases require invalid CP/DP refusal, preserved bindings after invalid
or mismatched deletion, safe
deletion outside the current mapping vector, valid differing software/hardware
indices, and exact creation/deletion. After the normal producer repair the
unchanged test must print `PPPoE cp binding: 10 cases passed`. The command removes
only its own interfaces and mapping state. The unchanged regression passed
with `0101` on 2026-11-15 in the same Ze-kernel Docker guest that observed the
pre-repair failure. The full packet mechanism proof remains separate.

`ze/0101-pppoe-validate-cp-bindings.patch` repairs `pppoe_add_del_cp` in the
normal image. The API and CLI supply software interface indices. The adopted
producer used the DP software index as a hardware-pool index and deleted map
cells without checking their bounds or reciprocal association. The repair
validates both software interfaces before lookup, resolves the hardware from
the DP software interface for the existing add-time Ethernet check, and
returns `INVALID_SW_IF_INDEX` for invalid interfaces. Deletion returns
`NO_SUCH_ENTRY` unless both map cells identify the requested pair, before any
write. An exact deletion clears both cells and leaves other bindings intact.
The binary API already returns the producer's status; the CLI now reports both
new refusals and names the command to inspect interfaces or current bindings.

`ze/0103-pppoe-owned-reconciliation.patch` adds Ze API messages without changing
the adopted message layouts or CRCs. `pppoe_ze_server` returns a 128-bit process
nonce from Linux `getrandom`. Every other Ze operation requires that nonce and
a nonzero 128-bit caller token. A matching MAC, session ID, or interface index
alone does not give a caller ownership after a lost reply.
The legacy VAT plugin also supplies adapters for these nine new services;
its token arguments contain exactly 32 hexadecimal digits.

Owned session and reciprocal control-map operations support query, add, and
delete. A repeated add returns the same owned object. A repeated delete returns
explicit absence. Original session allocation or reconfiguration cannot keep
an old token. Original map writes and interface deletion or reuse clear the
affected map tokens. The control-map token table has 4096 reusable slots.
An unowned stock session can use an IPv6 FIB, whose index is not an IPv4 FIB
index. Query still reports its presence, interface and zero owner token, while
its table ID stays explicitly unknown (`~0`). Only the nonzero token installed
by the Ze IPv4 creator permits resolving that session's IPv4 table.

The policer query returns the actual index from the native name hash and reads
the allocated directional slot. It never uses dump order. Names use an independent
policer lifetime token: `ze-pppoe/<32 lowercase hexadecimal digits>/rx` or `/tx`.
That token changes when the policer owner is replaced. Cold native metadata
binds it to the owning session token without enlarging the 64-byte packet-path
policer structure. Stock add, update and delete invalidate the metadata; an
object recreated at the same index and name cannot inherit it.
This is Ze's exclusive object namespace, not an authorization boundary against
another privileged VPP API client. The binding operation checks the live
session token, expected slot, and exact owned name in one API request.
It does not repeat an already-applied feature enable, and it refuses removal
while any corresponding policer arc remains enabled. Unallocated slots and
missing policers have explicit absent answers. Policer pool index zero is valid.

Policer create, update and delete also run under these native receipts. Updates
use the same native object and preserve buckets and counters on an identical
retry. Deletion refuses an object referenced by any directional slot. An exact
unbound orphan can be deleted after its old session is gone without touching
the replacement interface. The algorithm is explicitly 1R2C, KBPS, round-down,
transmit/drop/drop with caller-supplied positive rate and burst.

Route, feature and interface writes carry the session token in the same
serialized API request that changes native state. Route mutations own the
`ze-pppoe` source and preserve other paths; native initialization allocates that
source once. Typed feature selectors cover the four family gates and the seven
policer arcs. Typed interface operations cover L3 MTU, admin state and each
family's table. Querying a token and then sending an unguarded stock mutation
does not provide this guarantee. A deleted CP interface can confirm map absence
only after both reciprocal cells and the retained owner record have disappeared.
Native sessions retain at most1024 exact path receipts. Manual hiding/reuse,
original reconfiguration and actual interface deletion remove those paths;
VPP's permanent-DROP path state after interface deletion is not treated as
route removal. Stock and owned session deletion also fence receive/policer
arcs and unlink directional slots before a hidden index can be reused. They
leave the independent policer objects available for exact orphan cleanup.

Route receipts preserve the requested next hop before VPP normalizes the path
in place. In particular, a zero-next-hop host route becomes an attached path.
Retry matching uses the original request, and withdrawal applies the native
normalization once, so a successful retry cannot make the route unremovable.

The patch initializes untouched policer slots to `~0`, and manual session
deletion uses the existing session record after its learned link disappears.
The lab command `vppctl test pppoe ownership` requires a single-thread instance
without other sessions or policers. Its expected answer is
`PPPoE ownership: 55 cases passed`. The command exercises the actual producers
including an unowned IPv6 session with no corresponding IPv4 FIB index. It checks
that an owned zero-peer session leaves no stock `pppoe` source at `0.0.0.0/32`,
while a stock creation with FIB enabled installs that exact source/path. Two
owned subscribers then share one prefix: its actual `ze-pppoe` path list goes
from two paths to the surviving owner's one path, then to absence, while a
foreign API-source path survives both withdrawals. The same owner must query absent for a wrong MAC or unused SID without changing
the original session. Each of the eleven numeric selectors is exercised against
an independent literal native arc/node, with actual RX/TX bindings and native
reference counts across enable, disable and retries. Native config-hash readback
checks both policers use 1R2C, KBPS, round-down and transmit/drop/drop. These checks
strengthen the existing case groups; the declared total remains 55. The command removes its own
objects. On 2026-11-16, the pinned source and complete patch catalog compiled
inside the Ze-kernel Docker guest. The native binding and ownership commands
passed their exact 10-case and 55-case output checks on image
`sha256:a017bfa233b228b5e486a2ec74fc7a2c5d52c7cf078af66dead01e36139d4d06`.
The preceding ownership run failed because an IPv4 path remained after
withdrawal; preserving the requested path before native normalization repaired
that failure without removing its assertion. This proves the native command's
cases. Lost-reply/restart integration through Ze and ownership discrimination
runs remain owed.
