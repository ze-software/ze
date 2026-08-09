# Class of Service: Named VLAN QoS Profiles

Interface VLAN QoS maps (`ingress-qos-map`, `egress-qos-map`) needed repeating
inline on every unit. A BNG with hundreds of VLAN subscribers sharing one 802.1p
mapping needs a named profile instead.

The `class-of-service` plugin owns the profile definitions and binds them to
interfaces. The iface component keeps the low-level mechanism.

The RADIUS-driven half is [cos-dynamic](cos-dynamic.md).

## Decisions

### A shared registry in `internal/core/cos`

<!-- source: internal/core/cos/cos.go -- Register, Lookup, Clear -->

Config resolution is synchronous inside `InProcessConfigVerifier`, so a shared
registry is the simplest thing that works. It is the same pattern the subscriber
handler registry uses. DirectBridge and a plugin-internal store both cost more
and buy nothing here.

### YANG container-merge for the interface binding

<!-- source: internal/plugins/cos/config.go -- parseAndRegisterProfiles -->

Container-merge needs no import of `ze-iface-conf`, and deleting the plugin
cleanly removes both the profile definitions and the interface binding. An
augment would leave the binding leaf behind.

### The profile reference is a string, validated in Go

<!-- source: internal/component/iface/config.go -- parseUnits, cos.Resolve -->

A leafref across container-merged modules creates YANG coupling. Go validation
through `cos.Lookup()` is equivalent and survives the plugin's removal.

### Two explicit direction containers

Ingress is PCP-keyed and egress is priority-keyed, in two containers, matching
the kernel model of two independent maps. A bidirectional shorthand hides that
they are independent.

### Inheritance at the interface with a per-unit override

The BNG case has hundreds of units sharing one profile. The interface carries the
profile, a unit can override it, and `none` opts a unit out.

## Consequences

- The cos plugin's `InProcessConfigVerifier` MUST run before the interface
  plugin's verifier. It does, because `registry.All()` is sorted alphabetically
  and "cos" sorts before "interface". This ordering is load-bearing and nothing
  else enforces it.
- Inline qos maps and a `class-of-service` reference are mutually exclusive on
  one unit. The conflict check is in `parseUnits`.
- Removing the plugin (delete the directory and the blank import in `all.go`)
  removes the entire class-of-service surface. Inline qos maps keep working.
