# 633 -- op-0-umbrella

## Context

`spec-op-0-umbrella` was written from a VyOS gap analysis to add ten
operational show/clear commands to ze. The spec's "ten new commands"
framing was wrong: five were already implemented on main (`show
interfaces type`, `show bgp <family> summary`, `ping`, `traceroute`,
`show system uptime`) and the remaining five mixed simple additions
with a structural gap (firewall component was a library, not a running
plugin). The spec was also inaccurate on two backend claims -- VPP
`GetStats` was described as "already used" but returned
`errNotSupported`, and nft `GetCounters` was described as returning
populated counters but filled `ChainCounters.Terms` with an empty
slice. This session landed four of the five remaining commands end-to-
end (ARP, IP route, firewall ruleset, firewall group) and wired the
firewall component properly so the ruleset/group commands have a real
backend to talk to. `clear interfaces counters` stayed deferred
because the top-level `clear` verb does not exist in ze's YANG verb
set and Linux has no generic counter-reset syscall anyway -- both
structural changes, not one-session work.

## Decisions

- Split work into two `iface.Backend` additions (`ListNeighbors`,
  `ListKernelRoutes`) rather than a new `fib` component, because the
  kernel neighbor + routing tables are OS-wide read surfaces best
  dispatched through the existing iface backend (netlink on Linux,
  reject on VPP). A new component would duplicate `RegisterBackend`
  plumbing for no gain.
- Rejected adding `show ip route` to `fib-kernel` over extending iface
  because fib-kernel owns only ze-programmed routes (RTPROT_ZE=250);
  the operator wants the full kernel FIB regardless of who programmed
  each route.
- Wired the firewall component as a full SDK 5-stage plugin modeled on
  `internal/component/traffic` (register.go + engine.go +
  default_linux/other.go). Chose this over a lighter "backend loaded
  by someone, somehow" arrangement because the traffic plugin's
  OnConfigVerify/OnConfigApply/OnConfigRollback + Journal pattern is
  the standard ze shape for a config-driven backend and copying it
  kept the shape consistent.
- Counter-matching in nft uses `Rule.UserData = []byte(term.Name)` on
  AddRule plus an auto-prepended `&expr.Counter{}` on every rule,
  decoded on readback in `GetCounters -> readRuleCounter`. Chose this
  over index-zipping rules against the desired-state term list
  because UserData is explicit and survives partial Apply failures;
  operators never see off-by-one names.
- Every rule auto-carries a counter rather than only when the
  operator declares an explicit `Counter` action. Chose this over
  opt-in because VyOS-parity expectation is "counters always shown"
  and anonymous counters are free in nftables; the Counter action
  stays in the model for named-counter use cases.
- `firewall.StoreLastApplied(tables)` + `firewall.LastApplied()`
  snapshot chosen over inverse-lowering kernel rules back to ze's
  abstract model. Inverse lowering would require decoding every
  nftables expression kind back into 18 Match and 24 Action types,
  an entire second implementation. The applied-desired-state snapshot
  is a few atomic.Pointer operations.
- Added `clear` as a new top-level YANG verb rather than shoehorning
  counter-reset into `set` or `update`. `clear` is semantically
  distinct (reset operational state; config untouched). New package
  `internal/component/cmd/clear/` mirrors the existing del/set/update
  verb packages.
- `iface.Backend.ResetCounters` uses a sentinel
  (`ErrCountersNotResettable`) so the dispatch layer can tell "this
  backend CAN reset" (returns nil) from "this backend cannot, fall
  back to baseline-delta" (returns the sentinel). Netlink always
  returns the sentinel because the Linux kernel exposes no generic
  counter-reset syscall; VPP will return nil once
  sw_interface_clear_stats is wired.
- Baseline-delta when the backend cannot physically clear: capture
  current counters per interface in `iface.counters.go`; GetStats /
  ListInterfaces / GetInterface transparently subtract the baseline
  on read so the operator sees "since last clear" deltas. Wrap
  detection (any raw field < baseline) drops the baseline so
  subsequent reads count from the kernel's new zero --
  information-preserving equivalent of rebase-to-zero.
- Kept wrap detection on the read path rather than also hooking
  every `Set*` mutation to re-check. Reads are cheap; the window
  between kernel-side reset and first read is tiny in practice.
  Proactive per-mutation rebase is a future refinement if a workload
  shows it mattering.

## Consequences

- Firewall config (`firewall { table ... }`) now actually reaches the
  kernel. Before this session, any firewall block in config was
  parsed, validated, and silently discarded -- no one called
  `firewall.LoadBackend` anywhere.
- `show firewall ruleset <name>` and `show firewall group` work
  whenever the firewall section is present in config; under
  exact-or-reject they fail cleanly with "no backend configured"
  otherwise.
- `iface.Backend` grew three methods (ListNeighbors,
  ListKernelRoutes, ResetCounters); every mock/stub/fake in the
  repo had to gain a no-op implementation. This ripples to future
  backend additions (keep the mock zoo small or wrap in a base stub).
- `ze clear interface counters [name]` works on any backend that
  implements real reset (VPP once wired) and on netlink via
  baseline-delta. From the operator's viewpoint semantics are
  identical: counters read zero immediately after clear, grow with
  subsequent traffic.
- On non-Linux platforms `firewall.defaultBackendName` is "", so any
  firewall section in config rejects at verify with "no backend
  configured and no OS default available". Darwin dev workflows that
  happen to include a firewall block will break; not caught by CI
  because CI functional tests run on Linux.
- On VPP, both `show ip route` and `show ip arp` reject with
  explanatory messages (VPP's FIB/ND tables aren't the kernel's).
  Implementing the VPP path later means adding `ip_route_v2_dump`
  and `ip_neighbor_dump` callers; the rejection text already names
  the missing calls.
- Test drift: adding a new plugin to the registry means
  `TestAllPluginsRegistered` and `TestAvailablePlugins` need their
  expected-lists updated. Both took the addition of `"firewall"`.

## Gotchas

- `check-existing-patterns.sh` blocks `Write` on a new `.go` file
  when a struct-or-function name matches one elsewhere; my first
  attempt at `firewall/engine.go` carried a public `DefaultBackendName()`
  that collided with iface/traffic. Removed the export -- readers use
  `ActiveBackendName()` anyway.
- `block-temp-debug.sh` blocks `fmt.Fprintf(os.Stderr, ...)` in any
  `.go` file that is NOT named `register.go`. The init()-time fatal
  "firewall: registration failed" had to move from a single
  `engine.go` into a separate `register.go` + `engine.go` split,
  matching the traffic component's existing split.
- `require-related-refs.sh` blocks `Write` when a referenced sibling
  file does not yet exist. Ordering matters -- create engine.go
  before accessor.go that mentions it, or drop the forward reference
  from the `// Related:` line and add it after.
- The `block-pipe-tail.sh` hook rejects `| tail`; commands piped to
  `tail` must be captured to a file first and read via Read. Same
  for `| head` followed by `| tail` chains.
- `TestDetectSection_DispatchesEachSection` fails on Darwin because
  host inventory is Linux-only; pre-existing, not caused by this
  session's changes. Safe to ignore until op-1's Darwin-aware fix
  lands.

## Files

- `internal/component/iface/iface.go` -- added `NeighborInfo`,
  `KernelRoute`, `NeighborFamily*` constants.
- `internal/component/iface/backend.go` -- added `ListNeighbors` and
  `ListKernelRoutes` to the Backend interface.
- `internal/component/iface/dispatch.go` -- added package-level
  `ListNeighbors` and `ListKernelRoutes` dispatchers.
- `internal/plugins/iface/netlink/neighbor_linux.go` (new) -- netlink
  NeighList impl + NUD state decoder.
- `internal/plugins/iface/netlink/route_linux.go` (new) -- netlink
  RouteList impl + rtm_protocol name decoder.
- `internal/plugins/iface/netlink/backend_other.go`,
  `internal/plugins/iface/vpp/ifacevpp.go`,
  `internal/component/iface/config_test.go`,
  `internal/component/iface/migrate_linux_test.go` -- stub
  implementations for ListNeighbors/ListKernelRoutes.
- `internal/component/firewall/register.go` (new) -- plugin
  registration.
- `internal/component/firewall/engine.go` (new) -- 5-stage lifecycle.
- `internal/component/firewall/accessor.go` (new) -- LastApplied,
  ActiveBackendName, StripZeTablePrefix.
- `internal/component/firewall/default_{linux,other}.go` (new) --
  build-tagged default backend name.
- `internal/component/firewall/backend.go` -- LoadBackend /
  CloseBackend now update active-name + applied snapshot.
- `internal/plugins/firewall/nft/backend_linux.go` -- applyChain adds
  auto-counter + UserData; GetCounters decodes via readRuleCounter.
- `internal/component/plugin/all/all.go` -- blank import firewall
  component.
- `internal/component/cmd/show/ip.go` (new) -- handleShowArp,
  handleShowIPRoute.
- `internal/component/cmd/show/firewall.go` (new) --
  handleShowFirewallRuleset, handleShowFirewallGroup.
- `internal/component/cmd/show/{ip,firewall}_test.go` (new) -- wiring
  + behaviour tests.
- `internal/component/cmd/show/schema/ze-cli-show-cmd.yang` -- added
  `ip` and `firewall` containers with ze:command leaves.
- `cmd/ze/main_test.go`,
  `internal/component/plugin/all/all_test.go` -- added
  `"firewall"` to expected plugin lists.
- `docs/guide/command-reference.md` -- added `show ip`, `show
  firewall`, `show system uptime`, `show bgp summary`, `ping /
  traceroute`, and `clear interface counters` sections with source
  anchors.

### Clear verb (new)
- `cmd/ze/main.go` -- added `"clear"` to `yangVerbs`.
- `internal/component/cmd/clear/{doc,clear}.go` (new) -- verb
  package; placeholder body, YANG lives in the schema subdir.
- `internal/component/cmd/clear/schema/{embed,register}.go`,
  `ze-cli-clear-{api,cmd}.yang` (new) -- YANG schema + `init()`
  registration (crucially blank-imported by `plugin/all/all.go` so
  the yang loader actually sees it; missed the first time).
- `internal/component/iface/backend.go` -- `ErrCountersNotResettable`
  sentinel + `ResetCounters` method.
- `internal/component/iface/counters.go` (new) -- `baselineStore`,
  `applyBaseline`, `resetCountersViaBackend` with wrap detection.
- `internal/component/iface/dispatch.go` -- GetStats,
  ListInterfaces, GetInterface now apply baselines;
  `ResetCounters` package-level dispatcher.
- `internal/plugins/iface/netlink/neighbor_linux.go` --
  `ResetCounters` returns `iface.ErrCountersNotResettable` so the
  dispatch layer falls through to baseline-delta.
- `internal/plugins/iface/vpp/ifacevpp.go` --
  `errNotSupported("ResetCounters ...")` pending GoVPP
  sw_interface_clear_stats wiring.
- `internal/component/iface/cmd/clear.go` (new) --
  `ze-clear:interface-counters` handler.
- `internal/component/iface/counters_test.go`,
  `internal/component/iface/cmd/clear_test.go` (new) --
  baseline-behaviour tests + WireMethod-registered wiring test.
