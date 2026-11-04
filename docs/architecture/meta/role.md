# Role Plugin -- Meta Keys

<!-- source: internal/component/bgp/plugins/role/otc.go -- decideOTCIngress, OTCEgressFilter -->
<!-- source: internal/component/bgp/plugins/role/otc_filter.go -- evaluateOTCIngress, evaluateOTCEgress, otcInsertions -->

## Keys Set

| Key | Type | Stage | When Set | Description |
|-----|------|-------|----------|-------------|
| `src-role` | `string` | Ingress filter | Source peer has role config | Set to our configured role for the source peer (e.g., `"provider"`, `"customer"`, `"peer"`, `"rs"`, `"rs-client"`). Derived from `peerRoleConfig.role` (our config), not from the peer's OPEN capability. |
| `src-peer-role` | `string` | Ingress filter | `resolvePeerRole` returns a non-empty role | What the source peer IS to us: the Role capability it announced, or the complement of our configured role, from RFC 9234 Table 2. It is written BEFORE the OTC early return, so a peer this plugin has no OTC work for still publishes its role. |

## Keys Read

| Key | Type | Stage | How Used | Description |
|-----|------|-------|----------|-------------|
| `src-role` | `string` | Egress filter | `resolveSrcRole(meta, srcCfg)` | Suppression frame: the value is OUR role toward the source, so `customer`/`peer`/`rs-client`/`rs` mean the source peer IS a Provider/Peer/RS/RS-Client. A route from one of those is suppressed toward a destination that IS a Provider/Peer/RS (RFC 9234 Section 3.1, "All other routes MUST NOT be propagated"). `provider` is the only accepted case (the source is our Customer, whose routes may transit). If we don't configure a role, we don't filter. |

## Absence

`src-role` being absent from `meta` does **not** disable OTC suppression. The
egress filter reads it through `resolveSrcRole`, which falls back to the
source peer's configured role when the key is missing. Takes that same
fallback when the key is present but not a usable string (a malformed input
must never be more permissive than a missing one). Only a source peer with
**no role config at all** yields the empty role, and that is what disables
filtering: if we don't configure a role for a peer, we choose not to filter
its routes.

The fallback exists because not every egress caller has been through the
ingress filter. `RelayStoredRoute` replays out of the Adj-RIB-In with no
ingress metadata. Treating the missing key as "no restriction" silently
skipped an RFC 9234 Section 5 leak guard on that path.

The destination side has the same shape. The destination role comes from the
peer's OPEN Role capability, which a peer may legitimately not send (accepted
when `strict` is unset). So `resolvePeerRole` recovers what the peer IS from
the complement of our configured role toward it. That recovery feeds Section 5
gates only: export-set matching keeps using the capability value, because
`unknown` there is an operator-selected target for peers that announced no
role, not an unanswered question.

Absent and NEVER RECORDED are different states, and only the export set can
tell them apart. `applyValidateOpen` records what each OPEN declared,
INCLUDING declaring none, so a present-and-empty entry means the peer was
validated and sent no role. A MISSING entry means no OPEN was ever recorded.
Both resolve to the configured complement for Section 5 gates, so the RFC
procedures cannot tell them apart and do not need to. The export set can, and
reports a suppression it could not evaluate under its own reason
(`role-unrecorded`) rather than as an export-set decision against role
`unknown`.

A role-configured peer no longer reaches that missing state through an
unanswered validator. `broadcastStampedValidateOpen` still skips a plugin it cannot
ask -- a nil plugin conn or a validate-open RPC error -- but it now carries the
skip forward as `OpenValidationUnavailableError` and the session is refused with
NOTIFICATION 2/11 (`unansweredOpenValidation`, `askOpenValidators`). Skipping
used to read as consent, so the session established with the Section 5 leak
guard absent.

The refusal is scoped by the plugin's own per-peer capability declaration. The
reactor lists the plugins that declared a capability for THIS peer rather than
for every peer (`openPolicyPlugins`) and passes that list down, so bgp-role owes
an answer for exactly the peers `extractRoleCapabilities` published a Role
capability for. A peer with no role configuration has no such declaration, holds
no unanswered policy, and establishes as it did before.

RFC 9234 Section 4.2 requires the connection to be rejected when the Roles do
not correspond. An unreachable validator leaves ze unable to say that they do,
so it rejects rather than assume (owner decision, 2026-08-30).

<!-- source: internal/component/bgp/server/validate.go -- broadcastStampedValidateOpen, askOpenValidators, unansweredOpenValidation, OpenValidationUnavailableError -->
<!-- source: internal/component/bgp/reactor/peer.go -- openPolicyPlugins, capabilitySelectors -->

Learned roles survive a config reload for every peer the new config still
names. A learned role is a property of the session, and an established peer
sends no second OPEN. So wiping the map on reconfigure left every live peer
looking unrecorded until its session bounced.

A learned role is stored for a locally configured role peer and for roleless
destinations the configured peer or dynamic-group census owns, even when no
source role currently activates egress. Name-to-address mapping includes
roleless configured peers. Their advertised Role serves the source's export
set only: RFC 9234 Section 4.2 still requires a locally configured role for
role-pair validation and Section 5 procedures.

Each OPEN replaces the observation, including explicit absence. The same
configured-peer census owns commit-time pruning. The OPEN validator's
removal-only state grant delivers removal reports on both transports even
while no source policy is active; generation ordering keeps an old removal
from deleting a newer OPEN. A later source-policy activation therefore reads
the destination's current advertised role without requiring a reconnect.
Unconfigured roleless OPEN identities are not admitted.

<!-- source: internal/component/bgp/plugins/role/otc.go -- OTCEgressFilter -->
<!-- source: rfc/short/rfc9234.md -- Section 5, OTC leak prevention -->

## Ordering

Ordering is enforced by pipeline structure, not convention.
`bgp-role:otc-ingress` is inserted at the head of each role peer's import
list. While any peer or dynamic group sets `role import`,
`bgp-role:otc-egress` is inserted at the head of every peer's export list
(`otcInsertions`, priority 100, required, generic `any-peer` scope).
The destination need not declare a role: egress also enforces the source's
export set. On receive the reactor finishes the import list before egress,
so ingress runs before egress for that UPDATE. With no roles anywhere,
neither OTC member is inserted.

Do **not** read that as "egress always sees ingress metadata". Egress is
reachable on paths that never ran ingress for the advertisement being built:
`RelayStoredRoute` replays out of the Adj-RIB-In and passes `meta` as nil.
That is why the egress filter recovers the source role from config rather than
trusting the key to be present (see Absence above).

<!-- source: internal/component/bgp/reactor/reactor_notify.go -- ingress filter step loop -->
<!-- source: internal/component/bgp/reactor/reactor_api_forward.go -- egress filter step loop -->

## Coupling

`src-role` is self-contained: no other plugin reads or sets it.

`src-peer-role` is read by `filter_community`, which uses it to derive the RFC
8195 Section 3.2 relation parameter
(`docs/architecture/meta/filter-community.md`). The coupling is the meta key
and nothing else. Neither plugin imports the other, so deleting either folder
leaves the other building: without the role plugin the key is never written
and no relation tag is written either, which is the closed state.

The two plugins are ordered by the peer's import list, not by code
position. `bgp-role:otc-ingress` is an inserted member, and the loader places
every inserted member ahead of the operator's members, so the operator's
`bgp-filter-community:relation` member, the reader, runs after the writer.

<!-- source: internal/component/bgp/plugins/role/otc_filter.go -- otcInsertions -->
<!-- source: internal/component/bgp/plugins/filter_community/community_filter.go -- relationFilterName -->
<!-- source: internal/component/bgp/plugins/filter_community/relation.go -- relationPeerRoleFromMeta -->

**Why the role plugin publishes rather than exporting an accessor.**
`resolvePeerRole` is unexported. It holds the RFC 9234 Table 2 complement and
the capability preference. Three seams were available: export an accessor,
carry the role in `filterapi.PeerFilterInfo`, or publish a meta key. An
exported accessor would make one plugin a compile-time dependency of another.
A `PeerFilterInfo` field would need the reactor to populate it, which puts
role knowledge into a shared package that owns none (`ai/rules/plugins.md`).
The meta key is the seam this pipeline already has for exactly this.

## Performance

Ingress: one `findOTC()` wire scan per UPDATE.

Egress is **not** a single map lookup. `OTCEgressFilter` scans the payload up
to three times per destination peer: `isPayloadUnicast`, then `findOTC` inside
`checkOTCEgress`, then `findOTC` again in the stamping block when the first
two did not return. The `src-role` lookup replaces a role *derivation*, not
the wire scans.

<!-- source: internal/component/bgp/plugins/role/otc.go -- OTCEgressFilter, checkOTCEgress, isPayloadUnicast -->

## Role resolution

Both directions resolve "what is this peer to us" through `resolvePeerRole`:
the OPEN Role capability when the peer sent one, otherwise the complement of
our configured role toward it (`peerRoleComplement`, the value form of RFC
9234 Table 2). RFC 9234 Section 4.2 makes the configured role the prescribed
input for Section 5 procedures. So a peer that sends no capability is still
subject to those procedures rather than skipping them.

A peer with **no local role config** is not subject to RFC procedures as a
source or destination, regardless of an advertised Role. Its destination
egress member still enforces a configured source's export set against that
advertised Role, without deriving an RFC gate role from it.

For a locally configured session, ingress rejection follows from local config
even when the remote sends no Role. A one-sided role typo can therefore
blackhole prefixes. Egress stamping depends only on the destination's local
role configuration and resolved peer role, and still applies when the source
has no role configuration.

One consequence operators should know: that recovery feeds the RFC gates only.
Export-set matching still sees such a peer as `unknown`. So a peer can hold
two roles at once -- its real role for conformance, `unknown` for policy.
Reaching a capability-less customer with `export default` therefore also needs
`unknown` in the export set.

<!-- source: internal/component/bgp/plugins/role/otc.go -- resolvePeerRole, peerRoleComplement -->
<!-- source: internal/component/bgp/plugins/role/config.go -- resolveExport -->

## Config keying

A configured peer's role config is keyed by its remote address, because that is
the key its readers hold: all three `getFilterConfig` callers pass
`PeerFilterInfo.Address.String()` first. `extractPeerRoleConfigs` takes the
address from `connection > remote > ip` (via `configjson.PeerRemoteIP`, which
also covers a peer inheriting the address from its group). A peer the config
document does not name is keyed by its group instead (see Dynamic groups).

When no remote IP resolves, the delivered map key -- the peer **name** -- is
used instead. Only when that name is itself a parseable address. Operators
commonly name a peer by its own address. In that case the name *is* the string
the readers look up, so the fallback resolves correctly.

A name that is not an address can never be reached, and neither can the
group-level `dynamic` placeholder. Both are refused at parse time with a WARN
naming the peer, rather than stored under a key nothing can find: a missing
config sends every RFC 9234 Section 5 gate down its permissive branch. A
silently inert role is indistinguishable from no role at all. Such a peer
never establishes anyway (`reactor/config.go` fails an empty or `dynamic`
remote IP with `ErrIncompleteConfig` and skips the peer). So the role config
was inert either way -- the defect was that it was inert *silently*.
`bgp-rpki` refuses the same two shapes.

Both refusals are about a **named** peer. A dynamic group's own placeholder
never reaches them: the template visit is keyed by the group and returns first.

<!-- source: internal/component/bgp/plugins/role/config.go -- extractPeerRoleConfigs -->
<!-- source: internal/component/bgp/configjson/traverse.go -- PeerRemoteIP -->
<!-- source: internal/component/bgp/reactor/config.go -- parsePeerFromTree remote-ip validation -->

## Learned role lifecycle

For a peer whose role is observed, the learned value is re-established on
every OPEN, including when the OPEN carries no Role capability or an
unassigned value from the 5-255 range. Then the previous learned value is
dropped and a real transition is logged. Without that, a peer that once
advertised a role and reconnected without one kept the stale value
indefinitely. `resolvePeerRole` prefers the learned capability over the config
complement -- so the peer stayed gated against a relationship it had stopped
claiming.

The OPEN is the ordering-safe place to do this. A session-down clear races it:
for an in-process plugin the state event is delivered on the process delivery
loop while validate-open arrives on the bridge callback loop. Clearing on the
OPEN is driven by the same event that sets it, for the same session, so the
last OPEN always decides. The one down that clears, the removal below, orders
itself against the OPEN by generation.

The clear is symmetric with the set on the reject path: the role is recorded
even when `validateOpenRolePair` refuses the session, so it is dropped on
refusal too.

`recordLearnedRole` writes an empty role rather than deleting the entry. So
"this peer's OPEN declared no role" stays distinct from "no OPEN was ever
recorded for this peer". `remoteRoleRecorded` reads that distinction for the
drop reason. Both resolve to the configured complement for RFC 9234 gates.

The reactor's removal of a peer removes its learned role. `bgp-role`
subscribes to `state`. Every registered OPEN validator receives the peer's
removal independently of filter membership, intersected with its state
subscription and peer selector. This cleanup lifetime remains active while
source policies are inactive. The
state `down` with reason `removed`, which the reactor emits for every removal,
established or not, reaches `forgetRemovedPeer` (`removal.go`), which deletes
the role under the key the peer's name maps to and under its address, and logs
`learned role dropped: peer removed` with the role it held.

The down is asynchronous: it is delivered on the event loop while
validate-open arrives on the callback loop, so a down can still be queued when
a peer created again under the same name validates its OPEN. The generation
orders the two. Every OPEN carries the newest forward generation the engine had
issued when it asked (`rpc.ValidateOpenInput.Generation`), and the stored role
keeps it. The reactor issues the removal down's `session-generation` as it
deletes the peer, under the reactor lock (`removalLocked`, which every removal
path calls: `delete bgp peer`, a dynamic member the reactor drops, a reload's
removal, and the undo of a reload's add), so it is above the
stamp of every OPEN the removed peer validated before, and no peer of that name
can validate an OPEN below it. `forgetRemovedPeer` frees a role only when the
down names a generation above the role's stamp, and otherwise logs
`learned role kept: a session newer than the removal stored it`. A down with no
generation frees the role with a warning.

A configured peer a commit removes is also handled by `pruneLearnedRoles` at
config-committed, in whichever process holds the plugin. One entry is left
behind:

| Case | What is left |
|------|--------------|
| An OPEN validated for a peer after its removal down was handled, or after the reactor deleted it (the removal stops the session without waiting for a validation already in flight, and that OPEN's stamp is at or above the removal's) | the role under that peer's key, until an OPEN for the same name replaces it |

It costs one map entry. A peer created again under that name replaces it with
its own OPEN, which `bgp-role` validates before the session can carry a route.

<!-- source: internal/component/bgp/plugins/role/role.go -- applyValidateOpen, recordLearnedRole, remoteRoleRecorded, filterKeyLocked, learnedRolePeerConfiguredLocked, forgetRemovedPeer -->
<!-- source: internal/component/bgp/plugins/role/removal.go -- handleRoleStructuredEvents, handleRoleEvent -->
<!-- source: internal/component/bgp/reactor/peer_removal.go -- removalLocked, completeRemovals -->
<!-- source: internal/component/bgp/server/events.go -- getStructuredStateEvent, deliverPeerState -->
<!-- source: internal/component/bgp/config/filter_state_binding.go -- grantFilterOwnersState -->

The `plugin.PeerInfo` `removalLocked` builds carries the peer's AS as an `rpc.RemoteAS`, read through the guarded accessor; for a dynamic peer removed before its AS was negotiated it is the not-yet-known state, never AS 0.
<!-- source: internal/component/bgp/plugins/role/register.go -- RunEngine, CLIHandler -->
<!-- source: internal/component/bgp/reactor/reactor_dynamic.go -- removeDynamicPeer -->
<!-- source: internal/component/plugin/server/delivery_graph.go -- (*Server).PeerScopedProcs -->

## Observability

Every route the plugin refuses is counted, by reason:

| Metric | Reasons |
|--------|---------|
| `ze_role_route_rejects_total{reason}` | `leak`, `malformed-otc` |
| `ze_role_route_suppressions_total{reason}` | `otc-present`, `source-role`, `export-set`, `role-unrecorded` |

The first drop of each reason also emits one WARN naming the peer, latched per
reason per process so the forward path never pays a per-route log. Per-route
detail stays at debug level. Before this, these paths logged at Debug and
nothing else. So a peer's advertisements could be withheld with no signal at
the default log level -- including because of a role or export-set typo.

`role-unrecorded` and `export-set` are the same suppression with different
causes, and they call for opposite operator actions. `export-set` says the
destination's role is known and your policy excludes it: check the policy.
`role-unrecorded` says no OPEN was ever recorded for that destination, so the
policy was never really evaluated: check whether the role plugin answered
validate-open for that session. They shared the `export-set` counter until the
two states were separated, which sent operators to a policy that had not run.

<!-- source: internal/component/bgp/plugins/role/metrics.go -- recordDrop, buildMetrics -->

## Dynamic groups

Role config stated on a **dynamic group** (`connection > remote > ip dynamic`
plus a `range`) governs every peer the group builds. Such a peer establishes
with a real address and has no `peer` list entry, so no name and no address in
the config document identifies it. Its group does: `buildDynamicPeerSettings`
writes the group name to `PeerSettings.GroupName`, and every
`filterapi.PeerFilterInfo` carries it.

`configjson.ForEachPeer` visits a dynamic group once, with a nil peer map and
`origin.Template` set. `extractPeerRoleConfigs` stores that visit under
`configjson.CapabilitySelector`, which puts the `group:` prefix in front of the
group's name. The prefix separates the two namespaces in one string space.
Nothing at config time compares a group's name against a peer's, so a peer named
`ix` and a group named `ix` can both exist.

Three readers resolve that entry, each from an identity it already holds:

| Reader | Resolves from | Result for a peer of the group |
|--------|---------------|--------------------------------|
| `Peer.getPluginCapabilities` | `PeerSettings.GroupName`, the third link of its name → address → group chain | the OPEN carries capability 9 with the role the group states |
| `getFilterConfig` | `PeerFilterInfo.GroupName` | the RFC 9234 Section 5 OTC gates run, on the group's role |
| `applyValidateOpen` | `rpc.ValidateOpenInput.Group` | the Section 4.2 role pair check runs, `strict` included |

Each reader tries the peer's own key first. A peer named in the group's `peer`
list therefore keeps what it states for itself, and the group's entry answers
only for the peers the template builds.

The capability chain is gated on `PeerSettings.IsDynamic`. A statically
configured peer can never draw a capability declared for a group of the same
name. The name, address and eligible group selectors are queried together,
so a plugin's global capabilities cannot hide its peer-specific declaration
by ending a sequence of separate lookups early.

<!-- source: internal/component/bgp/configjson/traverse.go -- ForEachPeer, CapabilitySelector -->
<!-- source: internal/component/bgp/plugins/role/config.go -- extractPeerRoleConfigs -->
<!-- source: internal/component/bgp/plugins/role/role.go -- getFilterConfig, applyValidateOpen -->
<!-- source: internal/component/bgp/reactor/peer.go -- getPluginCapabilities, capabilitySelectors -->
<!-- source: internal/component/bgp/config/resolve.go -- resolveDynamicGroup -->

