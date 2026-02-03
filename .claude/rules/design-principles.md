# Design Principles

## Core Principle

**Design for the long term. Every decision compounds.**

## YAGNI - You Aren't Gonna Need It

**BLOCKING:** Do not build features, abstractions, or flexibility that aren't immediately needed.

| ❌ Wrong | ✅ Right |
|----------|---------|
| "Let's add a plugin system in case we need it" | Build concrete implementation now |
| "This config option might be useful someday" | Add config only when needed |
| "Let's make this generic for future use cases" | Solve the specific problem first |

**Why:** Unused abstractions rot. They become maintenance burden, mislead readers, and often don't fit actual future needs.

## Simplicity Over Cleverness

**Prefer boring code that obviously works over clever code that might work.**

```
❌ Complex: Custom DSL, metaprogramming, reflection magic
✅ Simple: Plain functions, explicit code paths, standard patterns
```

**Test:** Can a new developer understand this in 30 seconds?

## No Identity Wrappers

**A wrapper is justified only if it transforms the interface.**

Valid wrapper reasons:
- Type conversion (e.g., `uint8` → domain type)
- Error wrapping or enrichment
- Default value injection
- Logging/metrics instrumentation

**Identity wrappers add indirection without value:**

```go
// ❌ Bad: Just delegates, adds nothing
func parseOrigin(s string) (uint8, error) {
    return parse.Origin(s)
}

// Call parse.Origin() directly instead
```

```go
// ✅ Good: Transforms the interface (type conversion)
func ParseOrigin(s string) (config.Origin, error) {
    v, err := parse.Origin(s)
    if err != nil {
        return 0, err
    }
    return config.Origin(v), nil  // Type conversion justifies wrapper
}
```

**When you find an identity wrapper:** Delete it, call the underlying function directly.

## Single Responsibility

Each component (function, struct, package) does ONE thing.

**Red flags:**
- Function names with "And" (e.g., `ParseAndValidate`)
- Structs with unrelated fields grouped together
- Packages that are "utils" or "helpers" (where does new code go?)

**Fix:** Split into focused units. Compose them.

## Explicit Over Implicit

**Make behavior obvious. No hidden magic.**

| ❌ Implicit | ✅ Explicit |
|------------|------------|
| Auto-register via init() | Explicit registration call |
| Global state modified by import | Pass dependencies explicitly |
| Convention-based behavior | Documented, visible behavior |
| Silent defaults | Require explicit configuration |

## Minimize Coupling

**Components should know as little as possible about each other.**

### Dependency Direction

```
High-level policy → Low-level details (good)
Low-level details → High-level policy (bad)
```

### Interface Segregation

```go
// ❌ Bad: Forces implementers to provide unused methods
type MessageHandler interface {
    HandleOpen(msg *Open) error
    HandleUpdate(msg *Update) error
    HandleNotification(msg *Notification) error
    HandleKeepAlive(msg *KeepAlive) error
    HandleRouteRefresh(msg *RouteRefresh) error
}

// ✅ Good: Clients depend only on what they need
type UpdateHandler interface {
    HandleUpdate(msg *Update) error
}
```

## Design for Change

**Assume requirements will change. Make changes easy.**

### Isolate Volatility

Put likely-to-change code behind stable interfaces:
- Config formats (parsing isolated from usage)
- Wire formats (encoding/decoding isolated from logic)
- External dependencies (wrapped, not used directly)

### Avoid Stringly-Typed Code

```go
// ❌ Bad: Typos compile, no IDE support
config.Get("peer.remote-as")

// ✅ Good: Compiler catches errors
config.Peer.RemoteAS
```

## Consider Failure Modes

**Every external call can fail. Every input can be malformed.**

### Questions to Ask

1. What if this times out?
2. What if this returns partial data?
3. What if the input is malformed?
4. What if this is called concurrently?
5. What if resources are exhausted?

### Error Handling Design

- Errors should be actionable (what failed, why, how to fix)
- Distinguish recoverable vs fatal errors
- Don't hide errors in logs - propagate them
- Clean up resources on error paths

## Premature Abstraction

**Three concrete implementations before abstracting.**

```
❌ First use case → immediately create interface
✅ First use case → concrete implementation
✅ Second use case → notice patterns, still concrete
✅ Third use case → now abstract with confidence
```

**Why:** Abstractions created too early encode wrong assumptions.

## Package Design

### Package Cohesion

Everything in a package should relate to a single concept:

```
internal/bgp/message/     # BGP message types and parsing
internal/bgp/capability/  # Capability negotiation
internal/bgp/fsm/         # Session state machine
```

### Package Coupling

Packages should form a directed acyclic graph:
- Lower-level packages don't import higher-level
- No circular dependencies
- Clear layering

## Naming

**Names are documentation. Choose them carefully.**

### Be Precise

```go
// ❌ Vague
data, info, result, item, thing

// ✅ Precise
wireBytes, peerConfig, parseResult, nlriEntry, sessionState
```

### Be Consistent

Same concept = same name throughout codebase:
- Don't mix "peer" and "neighbor" for same concept
- Don't mix "message" and "packet" for same concept

### Length Proportional to Scope

```go
i        // loop index (tiny scope)
peer     // local variable (small scope)
peerAddr // struct field (medium scope)
DefaultKeepaliveInterval // package constant (large scope)
```

## Scalability Checklist

Before implementing, verify:

```
[ ] No premature abstraction (do we have 3+ use cases?)
[ ] No speculative features (is this needed NOW?)
[ ] Single responsibility (does each component do one thing?)
[ ] Explicit behavior (no hidden magic or conventions?)
[ ] Clear error handling (all failure modes considered?)
[ ] Minimal coupling (components isolated?)
[ ] Consistent naming (matches codebase conventions?)
[ ] Testable design (can we unit test this in isolation?)
```

## The Next Developer Test

**Would a developer unfamiliar with this code understand it quickly?**

Ask:
- Are the intentions clear from reading the code?
- Are there any "gotchas" that need documentation?
- Would they know where to add a new feature?
- Would they know what tests to add?

If not, simplify until the answer is yes.
