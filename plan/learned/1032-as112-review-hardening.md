# 1032 - AS112 review-hardening (spec-as112-2/3, two /ze-review passes)

## Context

After spec-as112-1/2/3 landed the AS112 anycast DNS feature (RFC 7534/7535),
a full `/ze-review` pass and two subsequent fresh follow-up passes (each run
via an isolated fork, deliberately re-reading the diff from scratch rather
than trusting the prior pass's fixes) found 2 BLOCKER, 7 ISSUE, and 10 NOTE
findings across three rounds. All were fixed and re-verified. The most
reusable lessons are below; the exhaustive finding list lives in the
conversation history, not repeated here.

## Decisions

- **`dns.Server.NotifyStartedFunc` closes the bind/Stop race, not a sleep or
  a mutex.** `internal/core/dnsserver/manager.go`'s `bind()` spawned listener
  goroutines and returned immediately; `Stop()` called `ShutdownContext`
  synchronously, which returns immediately WITHOUT closing the socket if
  `dns.Server.started` is still false (the goroutine hasn't been scheduled
  yet). This leaked a live, unreferenced listener on a port the Manager
  believed was free -- confirmed via a dedicated 30-iteration regression test
  that failed on iteration 0 before the fix. `NotifyStartedFunc` fires
  unconditionally, early inside `serveUDP`/`serveTCP`, strictly after
  `ActivateAndServe` sets `started=true` (verified from miekg/dns source, not
  assumed) -- `bind()` now waits on both UDP and TCP started-channels before
  returning, closing the window exactly.
- **A plugin that calls another in-process package's plain Go functions
  directly (bypassing DirectBridge/DispatchCommand) must check
  `sdk.Plugin.IsInternal()` and either refuse to start or degrade visibly.**
  as112's `RunEngine` called `iface.RegisterOwnedAddresses` as a bare Go
  function call. That only reaches the engine's real registry when as112
  shares process memory with `iface` -- nothing in config validation
  prevents `plugin { external as112 { ... } }`, and an external as112 would
  silently register against its own copy of `iface`'s package-level state,
  never erroring, never landing an address on any real interface. Added
  `sdk.Plugin.IsInternal()` (`p.bridge != nil`, the same DirectBridge
  discovery `NewWithConn` already computes) as a reusable primitive; as112
  refuses to start when external (its whole purpose depends on the call
  succeeding), while `cos` (same unguarded-`iface.GetBackend()` pattern,
  found by a later review pass explicitly checking whether the fix
  generalizes) only warns, since its static config/show functionality still
  works external and only the EventBus-triggered dynamic path is degraded.
  Severity of the response (refuse vs. warn) should track how much of the
  plugin's value survives running external, not be copy-pasted.
- **A fire-and-forget async trigger needs an observable outcome, not just a
  log line.** `iface`'s registry-reconcile trigger
  (`RegisterOwnedAddresses`/`UnregisterOwnedAddresses` -> a non-blocking
  channel signal -> a separate goroutine's `reconcileOnRegistryChange`) only
  ever reached `log.Warn` on failure. Added `RegistryReconcileStatus()`
  (package-level `atomic.Pointer` outcome, updated after every pass) and
  surfaced it through `show as112`'s existing state-reporting handler, in
  BOTH its early-return (`st == nil`, no config yet) and normal branches --
  the first version only added it to the normal branch, missing that a
  registry failure from a DIFFERENT consumer is visible regardless of
  as112's own configured state (caught by a review pass, not the first
  implementation).
- **A doc-comment block with no blank-line separator attaches to the wrong
  declaration.** Inserting a new `var` + its doc comment directly above an
  existing function's doc comment, without a blank line between the two
  blocks, makes Go/godoc treat them as one contiguous comment attached to
  whatever declaration follows -- the existing function silently loses its
  own doc comment. Caught by a review pass reading `go doc` output, not by
  compilation (this is not a compile error).
- **Test-isolation helpers must be updated when the package gains new
  resettable global state.** `resetAddressOwners` (test helper) reset
  `addressOwners`/`staleIfaces`/`addressOwnerTrigger` but was never updated
  when `lastRegistryReconcile` (backing `RegistryReconcileStatus`) was added
  in the same file -- no test failed immediately, but it was a latent
  ordering-dependent flake waiting for a future test.
- **`go test -tags ... -update` needs `-args` before the custom flag when
  combined with a build-tag list**, or `go test` misresolves the package
  pattern and errors on the module-root `tools.go` (`//go:build tools`) file
  with a misleading "build constraints exclude all Go files". Working form:
  `go test -tags '<tags>' -run '<pattern>' ./pkg -args -update`. This
  cost significant debugging time before being isolated to the missing
  `-args` specifically (not the tag list, not the cache, not `-run`).
- **`-tags linux` on a Darwin host is not `GOOS=linux`.** It just adds an
  arbitrary custom tag while still targeting the host OS, so both
  linux-suffixed and darwin-suffixed stdlib files match simultaneously,
  producing `internal/goos`/`internal/cpu` redeclaration errors that look
  unrelated to the actual mistake. Cross-compile with `GOOS=linux
  GOARCH=<arch> go vet -tags integration ./pkg/...` instead.
- **`make ze-linux-test`'s Docker invocation has no elevated capabilities by
  default** (`--user "$(id -u):$(id -g)"`, no `--cap-add`) -- CAP_NET_ADMIN
  netns-creation tests (`withNetNS`) correctly skip gracefully there, not a
  bug in the test. To actually exercise a `//go:build integration && linux`
  netns test on a macOS host, either use `make ze-qemu-integration-test`
  (real Linux kernel, real root; the intended, already-existing path -- the
  package list is auto-derived from the exact build-tag line, so a new
  package with that tag is picked up with zero Makefile changes) or a
  one-off `docker run --privileged` invocation for a quick manual check.
  `--cap-add=NET_ADMIN --cap-add=SYS_ADMIN` without `--privileged` was NOT
  sufficient (still failed on the actual `unshare`/mount syscalls under
  Docker's default seccomp profile) -- confirmed by direct experiment, not
  assumed.

## Consequences

- `sdk.Plugin.IsInternal()` is now a reusable primitive for any future
  plugin that calls another in-process package's functions directly instead
  of going through DirectBridge -- `cos` is the second consumer; a repo-wide
  audit for the same unguarded pattern (any plugin's `ConfigureEventBus`/
  `RunEngine` calling a package-level function on another component
  directly) was explicitly out of scope for this pass and is a reasonable
  follow-up.
- `iface.RegistryReconcileStatus()` is generic (reflects the whole registry,
  not per-owner) -- any future plugin using the address-ownership registry
  gets the same visibility for free, with the same caveat `show as112`'s doc
  comment states explicitly: a failure from a different registry consumer
  shows here too.
- Every fix in this pass added a regression test reproducing the specific
  failure mode before the fix (the dnsserver race test failed reliably on
  iteration 0 pre-fix; the `IsInternal`/`warnIfExternal` tests exercise both
  branches; `RegistryReconcileStatus` is tested via a genuine kernel-apply
  failure injected through the existing fake-backend `addAddressErr` seam,
  not just the success path).

## Files

- `internal/core/dnsserver/manager.go` -- `NotifyStartedFunc` synchronization in `bind()`
- `internal/core/dnsserver/manager_test.go` -- socket-leak regression test
- `pkg/plugin/sdk/sdk.go` -- `Plugin.IsInternal()`
- `pkg/plugin/sdk/sdk_test.go` -- `TestPlugin_IsInternal`
- `internal/plugins/as112/register.go` -- `IsInternal()` refusal in `runAS112Plugin`
- `internal/plugins/as112/register_test.go` -- `TestRunAS112Plugin_RefusesExternalProcess`
- `internal/plugins/as112/show.go` -- `addRegistryStatus`, called from both `handleShowAS112` branches
- `internal/plugins/as112/show_test.go` -- registry-status surfacing tests
- `internal/component/iface/address_owner.go` -- `RegistryReconcileStatus`, `recordRegistryReconcileOutcome`, doc-comment fix
- `internal/component/iface/address_owner_test.go` -- `resetAddressOwners` now resets `lastRegistryReconcile`
- `internal/component/iface/config_apply.go` -- `reconcileOnRegistryChange` records outcome
- `internal/component/iface/config_apply_test.go` -- `TestReconcileOnRegistryChange_RecordsOutcome`
- `internal/component/iface/registry_integration_linux_test.go` (new) -- real-netlink integration test for the address registry
- `internal/component/plugin/all/testdata/*.snapshot` -- regenerated plugin registry snapshots
- `cmd/ze/hub/service_ssh.go` + `service_ssh_test.go` -- SSH exit-code fix (shared infra, found while designing as112-3's healthcheck probe)
- `internal/component/bgp/plugins/cmd/update/update_text.go` -- `parseCommunityText` well-known-name table fix (found while designing as112-3's community worked example)
- `scripts/docvalid/doc_drift.go` -- interop-scenario counter no longer counts dotfile cache dirs
- `docs/guide/as112.md` -- internal-only requirement, `show as112` field docs, `isOnBox` spoofability caveat
