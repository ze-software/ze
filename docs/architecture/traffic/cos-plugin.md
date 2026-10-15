# Class of Service: Named VLAN QoS Profiles

Interface VLAN QoS maps (`ingress-qos-map`, `egress-qos-map`) needed repeating
inline on every unit. A BNG with hundreds of VLAN subscribers sharing one 802.1p
mapping needs a named profile instead.

The `class-of-service` plugin owns the profile definitions and binds them to
interfaces. The iface component keeps the low-level mechanism, documented in
[`../iface/vlan-qos-map.md`](../iface/vlan-qos-map.md).

The RADIUS-driven half is [cos-dynamic](cos-dynamic.md).

## Decisions

### A shared registry in `internal/core/cos`

<!-- source: internal/core/cos/cos.go -- Replace, Lookup, RegisterResolver, RegisterProfileParser -->

The registry holds the installed profiles, the ones `show class-of-service` and
the RADIUS Filter-Id handler read. It is the same pattern the subscriber handler
registry uses. DirectBridge and a plugin-internal store both cost more and buy
nothing here. Beside the profiles the cos plugin registers two pure functions,
its profile parser and its resolver, so the interface component parses and
resolves profiles without importing the plugin.

### Verify parses, apply installs, rollback restores

<!-- source: internal/plugins/cos/register.go -- verifyCoSConfig, newCoSConfigTx -->
<!-- source: internal/plugins/cos/config.go -- parseCoSSections, installCoSProfiles -->

The cos config is one value, the profile set, and it follows the plugin config
transaction (`sdk.ConfigTx`). Every verify, from a commit, a CLI `validate` or
the editor, parses the candidate and changes nothing, so a refused commit never
reaches the RADIUS handler or `show`. The apply installs the whole set with one
`Replace`, so a dropped profile is gone and a reader never sees half of two
sets. A rollback installs the set the apply replaced. A reload that edits a
profile takes effect without a restart.

### YANG container-merge for the interface binding

<!-- source: internal/plugins/cos/config.go -- parseProfiles -->

Container-merge needs no import of `ze-iface-conf`, and deleting the plugin
cleanly removes both the profile definitions and the interface binding. An
augment would leave the binding leaf behind.

### The profile reference is a string, validated in Go

<!-- source: internal/component/iface/config.go -- parseResolvedIfaceSections, resolveUnitQoS -->

A leafref across container-merged modules creates YANG coupling. Go validation
through `cos.Resolve()` is equivalent and survives the plugin's removal.

### The interface resolves against the candidate's profiles

<!-- source: internal/component/iface/register.go -- ConfigReads -->
<!-- source: internal/component/iface/config_apply.go -- indexUnitQoS -->

The interface component reads the `class-of-service` root (`ConfigReads`), so a
commit that changes a profile reaches it with the candidate's
`class-of-service` section. Its verify and its boot configure resolve every
unit against the profiles parsed from that section, never against the installed
registry. A unit naming a profile the candidate drops is refused at verify with
`class-of-service profile "<name>" not found`. The operation decomposer is
handed only the `interface` root, so its parse keeps the profile name
unresolved: a commit that adds a profile and the unit using it decomposes, and
verify does the resolution.

At apply, a VLAN unit that already exists and whose resolved maps differ from
the previous config's is reprogrammed with `UpdateVLANQoSMap`, and a rollback
writes the previous maps back. `CreateVLAN` programs the maps of a VLAN it
creates only.

### Two explicit direction containers

Ingress is PCP-keyed and egress is priority-keyed, in two containers, matching
the kernel model of two independent maps. A bidirectional shorthand hides that
they are independent.

### Inheritance at the interface with a per-unit override

The BNG case has hundreds of units sharing one profile. The interface carries the
profile, a unit can override it, and `none` opts a unit out.

## Consequences

- No ordering between the cos and interface plugins is load-bearing, at verify
  or at apply: nothing computes an interface map from the installed registry.
- Inline qos maps and a `class-of-service` reference are mutually exclusive on
  one unit. The conflict check is in the cos resolver, which `resolveUnitQoS`
  calls.
- Removing the plugin (delete the directory and the blank import in `all.go`)
  removes the entire class-of-service surface. Inline qos maps keep working.
