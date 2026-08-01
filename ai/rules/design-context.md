# Design Context

**When:** before any design decision: communication mechanism, naming, package placement, platform backend, or lifecycle
**Severity:** blocking

## Directives

Before any design decision (communication mechanism,
naming, package placement, platform backend, lifecycle), load the
relevant context below. Trained instincts about "how software works"
are wrong here -- ze has opinions.

Incident: session as-router (2026-04-13) made 7 wrong recommendations
by starting to design before loading context.

Incident: session l2tp-8a-auth-pool (2026-04-21) proposed a new
direct-call mechanism between core and plugins, not discovering that
DirectBridge already provides typed function calls. Root cause: no
document in the research path mentioned DirectBridge for request/response.
Fixed by splitting the cross-plugin comm row into broadcast vs
request/response and adding DirectBridge to the anti-pattern table.

## Tier 1: Always Read Before Any Design

| What | Where | Prevents |
|------|-------|----------|
| Design principles | `ai/rules/design-principles.md` | "Good enough" backends, translation layers, implicit behavior, missed abstractions (abstract at 2+ use cases) |
| Plugin architecture | `ai/rules/plugin-design.md` | Wrong package, import violations, wrong comm mechanism |
| Registration pattern | `ai/patterns/registration.md` | Missing init + registry + blank import pattern |
| Existing core packages | `ls internal/core/` | Missing patterns like `internal/core/family/` |

## Tier 2: When Designing a Specific Artifact

| Artifact | Read | Prevents |
|----------|------|----------|
| New plugin | `ai/patterns/plugin.md` | Wrong structure, missing YANG, wrong callback |
| Cross-plugin comm (broadcast) | `pkg/ze/eventbus.go` + `internal/core/events/typed.go` + one consumer (e.g. fibkernel) | EventBus is for async pub/sub notifications, not request/response |
| Cross-plugin comm (request/response) | `pkg/plugin/rpc/bridge.go` (DirectBridge) + `plan/learned/DESIGN-HISTORY.md` "Plugin system: architecture" (294, retired) | DirectBridge for sync typed calls from core to internal plugins. Do not reinvent this. |
| Shared registry | `internal/core/family/` (read the code) | Registry inside a plugin instead of core |
| Config option | `ai/patterns/config-option.md` + `ai/rules/config-design.md` + `ai/rules/config-surface.md` + `ai/rules/config-naming.md` | Missing env var, wrong YANG shape, env-only when should be config, wrong leaf name |
| CLI command | `ai/patterns/cli-command.md` | Wrong dispatch structure |
| TUI / terminal colors | `docs/architecture/cli/color-system.md` | Wrong color roles, inconsistent palette across surfaces |
| Platform-specific | Existing splits (`fibkernel/backend_linux.go`, `ifacenetlink/sysctl_linux.go`) | Wrong build tag, wrong abstraction level |
| New feature with dataplane effect | `internal/plugins/iface/netlink/` + `internal/plugins/iface/vpp/` | Netlink-only feature without VPP support |
| Naming | `ai/rules/naming.md` + `ai/rules/config-naming.md` (config/env) + grep analogous names | Inventing ze-names when kernel/standard names exist, abbreviated YANG leaves, env var path not mirroring YANG |

## Tier 3: When the Design Touches These Areas

| Area | Read | Prevents |
|------|------|----------|
| Plugin startup timing | `internal/component/plugin/server/startup.go` (`TopologicalTiers`, `runPluginPhase`) | Hand-waving instead of tier ordering |
| Wire encoding | `ai/rules/buffer-first.md` | Allocations in encoding |
| Env vars | `ai/rules/go-standards.md` + `ai/rules/config-surface.md` + `ai/rules/config-naming.md` + `internal/core/env/` | `os.Getenv`, missing `MustRegister`, env-only when should be YANG config, wrong naming convention |
| JSON format | `ai/rules/json-format.md` | Wrong key casing |
| Testing | `ai/rules/testing.md` + `ai/patterns/functional-test.md` | Missing .ci tests, wrong structure |
| Daemon lifecycle | `OnStarted`/`OnAllPluginsReady` in a similar plugin | Wrong callback, missing cleanup |

## BGP Domain Facts (Do Not Assume From Training Data)

| Fact | Why it matters |
|------|---------------|
| NEXT_HOP is set at the eBGP border router; all IBGP routes share a small set of next-hops (the L3 device originating the prefix or the eBGP peer) | Attribute byte overlap across IBGP peers is high, not low |
| MED, LOCAL_PREF, communities are policy-set by the sender and tend to be identical across many routes from the same peer | Same-peer attribute reuse is very high |
| AS_PATH is identical for all routes learned from the same external source; IBGP does not prepend | Cross-peer attribute overlap within an AS is significant |
| BGP UPDATE packing groups NLRIs with identical attributes into one message, but convergence events and incremental announcements spread them across multiple UPDATEs | Attribute reuse across UPDATEs from a single peer is common |

## Anti-Patterns

| Anti-pattern | Instead |
|--------------|---------|
| "Docs say X supports Y" | Read the implementation. Docs may be stale or aspirational |
| "Industry standard is X" | Grep ze for how it already does X |
| "Good enough for dev" | "Do it right." Darwin could be prod |
| "Overkill for now" when comparing compile-time vs runtime enforcement | Compile-time enforcement is not overkill, it is correctness. If the compiler can prevent a class of bugs, that is the right option. Implementation cost is not a reason to accept weaker guarantees. |
| "Translation layer for cleaner API" | "Explicit > implicit." Use native names |
| "Put the registry where it's used" | Check `internal/core/` first |
| "DispatchCommand for cross-plugin calls" | EventBus for broadcast; DirectBridge for request/response |
| "New direct-call mechanism for internal plugins" | DirectBridge already exists (`pkg/plugin/rpc/bridge.go`). Read it before proposing. |
| "No cleanup needed on stop" | Ze owns what it touches |
| "VPP support can be added later" | If the feature has a netlink implementation, add the VPP implementation in the same work. Ze targets both dataplanes; netlink-only features create drift. |
| "Defaults are suggestions" | Defaults are requirements; log when overridden |

## Mechanical Check

1. Did I read how ze already handles similar? (grep, not assume)
2. Did I check `internal/core/` for an existing shared pattern?
3. Did I read the relevant `ai/patterns/` file?
4. Does my proposal contradict `ai/rules/design-principles.md`?
5. Am I inventing a name when standard/kernel/existing exists?
6. Am I proposing a new communication mechanism? Read `pkg/plugin/rpc/bridge.go` first. DirectBridge likely already does it.
7. Am I comparing systems or claiming capabilities? Read the implementation for each system being compared. Spawn parallel agents if multiple codepaths need verification. Never answer from docs alone.
