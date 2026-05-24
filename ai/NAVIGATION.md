# Task Navigation

Use this document when you are about to do something and need to know
which context to load. The "Before You..." table in INSTRUCTIONS.md
covers 17 specific actions. This document covers the space between
those actions: how to find context when you don't yet know which
action you're taking.

## By Task Type

### Adding a Feature

| Feature kind | Read first | Then read | Cross-cutting |
|---|---|---|---|
| CLI command | `patterns/cli-command.md` | `ai/rules/cli-grammar.md`, `ai/rules/pipe-completeness.md` | `rules/derive-not-hardcode.md` if it lists things |
| Web page/endpoint | `patterns/web-endpoint.md` | `docs/architecture/web-interface.md`, `docs/architecture/web-components.md` | SSE: `docs/architecture/web-components.md` SSE section |
| Plugin | `patterns/plugin.md` | `rules/plugin-design.md`, `rules/goroutine-lifecycle.md` | `rules/naming.md` for registered names |
| Config option | `patterns/config-option.md` | `rules/config-design.md` (listener pattern if network endpoint) | `rules/go-standards.md` env var section |
| NLRI family | `patterns/plugin.md` (NLRI codec section) | `docs/architecture/wire/nlri.md`, `rules/buffer-first.md` | `rules/plugin-design.md` family registration |
| Capability | `patterns/plugin.md` (capabilities section) | `docs/architecture/wire/capabilities.md` | |
| Attribute | `rules/buffer-first.md` | `docs/architecture/wire/attributes.md` | `docs/architecture/encoding-context.md` |
| Functional test | `patterns/functional-test.md` | `docs/architecture/testing/ci-format.md` | `rules/testing.md` for format selection (.ci vs .et vs Go) |
| Editor test | `rules/testing.md` (Editor Tests section) | `test/editor/` existing examples | |
| Telemetry/metrics | `plan/learned/653-netdata-os-collectors.md` | `plan/learned/736-iface-rate.md` | Registration in loader_create.go |
| Diagnostic command | `plan/learned/727-diag-core.md` | `plan/learned/755-ze-doctor.md` | `rules/doctor-checks.md` |
| EventBus event | `rules/plugin-design.md` (EventBus Typed Payloads) | `pkg/ze/eventbus.go` | Use `events.Register[T]`, not raw `bus.Subscribe` |
| DirectBridge handler | `rules/plugin-design.md` (DirectBridge section) | `pkg/plugin/rpc/bridge.go`, `plan/learned/294-inprocess-direct-transport.md` | |
| New component | `docs/architecture/core-design.md` section 1 | `rules/design-principles.md`, `rules/architecture-summary.md` | Proximity principle in `rules/plugin-design.md` |
| New subsystem | `docs/architecture/hub-architecture.md` | `docs/architecture/subsystem-wiring.md` | |

### Modifying Existing Code

| Area | Read first | Key concerns |
|---|---|---|
| Reactor / session | `docs/architecture/core-design.md` sections 1-5 | `rules/goroutine-lifecycle.md`, `make ze-race-reactor` required |
| Wire encoding/decoding | `rules/buffer-first.md`, `rules/memory-architecture.md` | No `make()`, no `append()`, `WriteTo(buf, off) int`, caller-owned buffers |
| RIB / route storage | `docs/architecture/route-types.md`, `docs/architecture/rib-transition.md` | Pool dedup, lazy iterators |
| Route selection | `docs/architecture/route-selection.md` | `ai/LEARNED-INDEX.md` (RIB/Routing section) |
| Config pipeline | `docs/architecture/config/yang-config-design.md` | File -> Tree -> ResolveBGPTree -> map[string]any |
| Plugin SDK | `rules/plugin-design.md` (SDK Is Generic) | No plugin-specific code in SDK |
| Hub / engine | `docs/architecture/hub-architecture.md` | Protocol-agnostic, Coordinator pattern |
| Forward pool | `docs/architecture/forward-congestion-pool.md` | Two-tier model, per-peer workers |
| YANG schemas | `rules/config-design.md` | Augment vs grouping, listener pattern |
| Registration code | `patterns/registration.md` | `init()` + registry + blank import pattern |

### Fixing a Bug

```
1. Read rules/before-writing-code.md (sibling call-site audit)
2. Read rules/anti-rationalization.md (no rationalizing test failures)
3. Grep ALL implementations of the function/protocol step (integration-completeness.md)
4. Check plan/learned/RECURRING-PATTERNS.md for known traps in this area
5. After fixing: rules/testing.md iteration workflow
```

### Writing Documentation

```
1. Read rules/documentation.md (13 categories, source anchors)
2. Read the actual source before any factual claim
3. Add <!-- source: path -- symbol --> anchors
4. Run make ze-doc-test after editing docs/
```

### Working with IPsec / IKE

Read in order: `plan/learned/734` (data model), `plan/learned/739` (crypto),
`plan/learned/740` (engine), `plan/learned/742` (child SA), `plan/learned/744` (EAP/NAT-T).

### Working with CPE / Subscriber

Read: `plan/learned/760` (subscriber session model), `plan/learned/725` (DHCP ranges),
`plan/learned/746` (firewall global options).

### Writing Tests (Which Type?)

| What to test | Test format | Directory | Runner |
|---|---|---|---|
| Config parses correctly | `.ci` | `test/parse/` | `ze-test bgp parse` |
| BGP wire encoding | `.ci` | `test/encode/` | `ze-test bgp encode` |
| BGP wire decoding | `.ci` | `test/decode/` | `ze-test bgp decode` |
| Plugin behavior / API | `.ci` | `test/plugin/` | `ze-test bgp plugin` |
| Config reload via SIGHUP | `.ci` | `test/reload/` | `ze-test bgp reload` |
| CLI show/monitor output | `.ci` | `test/ui/` | `ze-test bgp ui` |
| Web HTTP endpoints | `.ci` | `test/web/` | `ze-test web` |
| Editor TUI interactions | `.et` | `test/editor/` | `ze-test editor` |
| Pure logic (no daemon) | `_test.go` | `internal/<pkg>/` | `go test` |
| Linux-only kernel code | `_test.go` | `internal/<pkg>/` | `make ze-qemu-integration-test` |

Key docs: `patterns/functional-test.md` (directories + runner commands),
`docs/architecture/testing/ci-format.md` (full format reference),
`rules/testing.md` (observer API, iteration workflow).

## When You Don't Know Which Area

Use keyword search in `ai/INDEX.md`. If the keyword isn't there:

```
grep -rn "keyword" docs/architecture/ --include="*.md" -l
grep -rn "keyword" plan/learned/ --include="*.md" -l
grep -rn "keyword" ai/rules/ --include="*.md" -l
```

## Cross-Cutting Rules (Apply Regardless of Area)

These rules are frequently missed because they don't map to a single
artifact type. Check them whenever your work touches the described concern.

| Concern | Rule | When it applies |
|---|---|---|
| Listing/enumerating things | `rules/derive-not-hardcode.md` | Help text, usage strings, error messages, any output that enumerates items |
| Goroutine lifecycle | `rules/goroutine-lifecycle.md` | Any `go func()`, any `OnStarted` callback, any worker pattern |
| File size | `rules/file-modularity.md` | Modified file exceeds 600 lines |
| Pipe operators | `ai/rules/pipe-completeness.md` | Any command producing output |
| Registered names | `rules/plugin-design.md` "Renaming" section | Changing any plugin/subsystem/dispatch/log name |
| Sibling call sites | `rules/before-writing-code.md` "Sibling Call-Site Audit" | Adding a guard/fallback/retry to ANY call site |
| Buffer allocation / memory | `rules/memory-architecture.md`, `rules/buffer-first.md`, `rules/no-sprintf-alloc.md` | Any allocation, pool use, string building, or wire encoding |
| Map keys / dispatch keys | `rules/enum-over-string.md` | Any new `map[string]` or string-based dispatch on a hot path |
| JSON keys | `rules/json-format.md` | Any new JSON output |
| Env vars | `rules/go-standards.md` env section | Any env var access |
| Error handling | `rules/go-standards.md` forbidden section | Any `_` on error return |
