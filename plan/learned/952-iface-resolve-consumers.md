# 952 -- iface-resolve: migrate consumers onto the resolver (routing/protocols/peripheral)

## Context

Fourth unit of the interface logical-name effort (`spec-iface-resolve-0-umbrella`),
covering umbrella sub-specs 4, 6 and 7. The shared resolver (`iface.Resolve` /
`Addresses` / `Subscribe`, 950) plus its os-name and mac-match selectors (951) existed,
but only IS-IS consumed it; every other subsystem still resolved a configured interface
name straight against the kernel (`netlink.LinkByName` / `net.InterfaceByName` / SIOCGIF
ioctls), forcing `name == kernel device`. This unit migrates the consumer groups so an
operator-facing logical name honors the selectors everywhere.

A four-agent parallel audit found the real sites (the umbrella's "54 sites / 19 components"
was stale after the component->plugin move). Migrated this unit: **routing** (static
nexthop, flowexport sampling), **protocols** (pppoe, ike, ldp), **peripheral** (imageserver,
diag, tftpserver). Deferred: **iface-internal** (sub-spec 5, the backend-dispatch
translation) and the **guard** (sub-spec 7's grep check), which both need the final K-site
set to be fixed first.

## Decisions

- **The process model makes `iface.Resolve` legal for every built-in plugin.** Built-in
  plugins run as **goroutines in the one `ze` process** (`plugin/process/process.go`
  `startInternal`, over a `net.Pipe`), so they share the package-global `iface.activeBackend`
  the iface component goroutine loads. There is no per-plugin process for built-ins, so a
  consumer's `iface.Resolve` uses the same backend -- no IPC, no re-load. (External plugins
  with a `Run` command DO fork via `/bin/sh -c`, but none of these consumers are external.)
  This is why the IS-IS migration works in production and why these do too.
- **pppoe's duplicate SIOCGIF wrapper collapses into a shared `resolve.go`.** It was the
  twin of the ioctl wrapper IS-IS dropped, split across `kernel_linux.go` (ioctl) and
  `socket_other.go` (stub). Because `iface.Resolve` is cross-platform, one non-tagged
  `resolve.go` replaces both; the exported `ResolveInterface` (used by the pppoe client
  dialer) now resolves transparently, so the dialer needed no change. Dropped the `unsafe`
  import.
- **LDP gets the first `Subscribe` consumer beyond IS-IS.** `waitForInterface` was a 5s
  poll loop returning `*net.Interface`; it became `iface.Resolve` (translate logical->os) +
  `iface.Subscribe` (wake the instant the device appears) + `net.InterfaceByName(os-name)`
  for the real `*net.Interface` the multicast socket needs. The retry timer stays as a
  bootstrap fallback (the resolver drops events on a full channel).
- **doctor stays on `net.InterfaceByName` (deliberate, flagged).** The audit classed it M,
  but `ze doctor` is a one-shot root CLI command with **no iface backend loaded** -- a
  resolver call would error on every check. Honoring selectors there needs the netlink
  backend loaded inside the doctor command (CAP_NET_ADMIN + lifecycle), a disproportionate
  change for a diagnostic; deferred. This is the one audit M-site intentionally not migrated.
- **Post-resolution `net.InterfaceByName(os-name)` is legitimate.** ldp and diag still call
  `net.InterfaceByName`, but on `binding.OsName` AFTER resolving the logical name -- to get
  the `*net.Interface` the stdlib (multicast / AF_PACKET) needs, which the value `Binding`
  does not carry. That is correct, not a missed migration.

## Consequences

- Sites migrated: static `resolveNexthopIndex`, flowexport `resolveIfaceIndex` +
  `sampling_worker` idx map, pppoe `resolveInterface` (+ exported), ike `resolveInterfaceAddr`,
  ldp `waitForInterface` + `interfacePrefixes`, imageserver `resolveInterfaceIPv4`, diag
  capture, tftpserver listen loop. All now translate logical->os via the resolver.
- Each migrated package's `integration && linux` tests must load the netlink backend
  (`iface.LoadBackend("netlink")` + blank-import `internal/plugins/iface/netlink`) -- same
  coupling IS-IS hit (950). A consumer's host test that needed a *resolvable* interface had
  to move to an integration test, since no non-Linux iface backend exists (netlink/vpp are
  both Linux-only), so `iface.Resolve` always errors on a darwin host.
- The stale LDP comment ("runs as a separate plugin process, reads the OS directly") was
  corrected -- it described a model that never shipped.

## Gotchas

- **No non-Linux iface backend exists.** On a darwin host `iface.Resolve` returns "no backend
  loaded" for any name. So: (a) a consumer's "interface found" host test must become an
  integration test (LDP `TestWaitForInterfaceFound` -> `...Resolves`, with a `// test-relax:`
  token); (b) host tests that use a *nonexistent* interface still pass unchanged (Resolve
  errors whether or not a backend is loaded), so LDP's cancel/warn-once tests stayed host.
- **One-shot CLI commands lack the backend; in-daemon command handlers have it.** A handler
  taking a `*pluginserver.CommandContext` (diag capture) runs in the daemon -> backend loaded.
  A root `CLIHandler` (doctor) runs standalone -> no backend. Check which before migrating a
  command, or you break the standalone path.
- **A migrated SIOCGIF wrapper can orphan a sibling import.** Removing pppoe's
  `resolveInterface` left `unsafe` unused -- drop it. `probeKernelPPPoE` shows as unused after
  the edit but was already dead at HEAD (gopls `unusedfunc`, not golangci `unused`, so it
  does not gate); pre-existing, left alone.
- **Audit M-classifications need an architecture pass.** The parallel audit was fast and
  mostly right, but missed the doctor backend constraint and (for sub-spec 5) flagged 16
  netlink-backend primitives as M when the translation belongs at the dispatch layer, not
  inside the backend. Treat audit output as a work-list to verify, not gospel.
- **`make ze-validate` goes red on PRE-EXISTING exports in any file you touch.** validate.py
  is changed-file-scoped: editing a file pulls every exported symbol in it into the
  "needs a cross-package non-test caller" check, even symbols you never modified. This
  migration's edits surfaced three such pre-existing findings -- ike `SetActiveTable` /
  `SetActivePeers` (test-only exports called solely from `ike/cmd/show_ipsec_test.go`; they
  want the sanctioned `ForTest` suffix) and diag `HandleCaptureInterface` (genuinely wired,
  but via a same-package `RPCRegistration{Handler: ...}` the validator does not model -- it
  only recognizes `MustRegister("name")` strings, `*ForTest`, and cross-pkg refs). None is a
  migration defect; all three are owned by the ze-verify-cleanup track. Do NOT "fix" the diag
  one by unexporting (breaks symmetry with its 3 sibling handlers) or by `ForTest` (a lie --
  it is production code). The commit hook does not run ze-validate, so these do not block the
  commit.

## Verification (all green)

- QEMU integration (verbose PASS): static `TestResolveNexthopIndexUsesResolver`, ldp
  `TestWaitForInterfaceFoundResolves` (real netlink backend; logical name -> real device).
- Cross-compile (linux + darwin) clean for every migrated package; host unit suites green
  (ldp cancel/warn-once, ike, pppoe, pppoeclient, static, flowexport).
- `make ze-lint-changed` 0 issues per group.
- Resolver remap proven centrally (951 stub + QEMU); the trivial per-consumer wrappers are
  covered by cross-compile + wiring grep + the central proof, not duplicate integration tests.

## Files

- routing: `internal/plugins/static/backend_linux.go` (+ `resolve_integration_linux_test.go`),
  `internal/plugins/flowexport/sampling/tc_linux.go`, `internal/plugins/flowexport/sampling_worker.go`
- protocols: `internal/component/l2tp/pppoe/resolve.go` (new), `kernel_linux.go`, `socket_other.go`;
  `internal/component/ike/engine/register.go`; `internal/plugins/ldp/register.go`, `local.go`,
  `register_test.go` (+ `resolve_integration_linux_test.go`)
- peripheral: `internal/plugins/imageserver/register.go`,
  `internal/plugins/diag/cmd/capture_interface_linux.go`, `internal/plugins/tftpserver/register.go`

## Remaining in the umbrella

- Sub-spec 5 (iface-internal): translate logical->os at the iface **dispatch** layer for the
  by-name mutation functions (SetMTU/AddAddress/.../bridge/mirror/xfrm/vlan), leaving
  `GetInterface`/`ListInterfaces` raw (the resolver's primitives) and the backend untouched;
  plus iface-dhcp v4/v6 and provision. The delicate one -- it touches every interface apply.
- Sub-spec 7 guard: a grep check rejecting new direct kernel resolution in consumer packages,
  with an allowlist for the K sites (iface backend, traffic backend, BFD bind, doctor,
  all-interface scans, post-resolution os-name lookups).
