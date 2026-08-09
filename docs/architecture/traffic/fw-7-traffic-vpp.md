# VPP Traffic-Control Backend

A second traffic-control backend beside netlink, registered as
`traffic.RegisterBackend("vpp", ...)`, programming VPP policers from the same
`traffic-control { }` config the netlink backend consumes.

The test seam and the context plumbing are in
[backend hardening](fw-7b-backend-hardening.md). DSCP policing and multi-class
steering, which this design rejected, landed later in
[the VPP traffic follow-up](followup-vpp-traffic.md).

## Exact or reject

<!-- source: internal/plugins/traffic/vpp/verify.go -- backend verifier -->
<!-- source: internal/component/traffic/backend.go -- Verifier, RegisterVerifier, RunVerifier -->

If a backend cannot apply EXACTLY what the operator's config asks for, the
verifier fails at commit with a clear error. No silent approximation, no
truncation, no best-effort mapping. The rule is `ai/rules/protocol.md`, and it
was codified from this work's review findings.

Five silent approximations were found in review here:

| What shipped in a draft | Why it was silently wrong |
|--------------------------|---------------------------|
| `egressMapFromPrioClasses` | discarded classes beyond 256 |
| a DSCP-filter path | each filter issued its own `QosEgressMapUpdate`, so only the last entry survived |
| `filter protocol` | the classify table was never attached to an interface |
| `filter dscp` | QoS mark with no ingress `QosRecordEnableDisable` |
| multiple policers on the output feature arc | they run IN SERIES per packet, so "fast class 10 Mbps, slow class 1 Mbps" becomes "everything at 1 Mbps" |

The last one is the most insidious shape of this bug: tests pass, the verifier
accepts, the backend programs VPP successfully, the operator sees "commit
applied", and the runtime behavior is wrong. Only reasoning about VPP's
feature-arc semantics exposed it.

**"Tests compile and pass" is not proof a backend feature works.** The unit tests
for `egressMapFromPrioClasses` and `protocolMatchBytes` passed on a translation
that was structurally wrong at the VPP API layer. A unit test validates the
translator's internal consistency, never that its output is what the external
system acts on. For a backend talking to an external system, the test must
exercise that system or the reviewer must read its semantics.

### A per-backend verifier, not a YANG gate

The YANG `ze:backend` gate annotates LEAVES, not enum values. "Reject
`qdisc hfsc` and accept `qdisc htb`" needs per-value logic.
`traffic.RegisterVerifier` and `RunVerifier` are called from `OnConfigVerify`
after the schema gate passes. Any future backend that accepts a subset of what
the schema permits uses the same hook.

**`ze config validate` (offline) does not invoke plugin `OnConfigVerify`
callbacks.** A `.ci` test for a verifier-driven rejection must run the daemon,
not the offline CLI.

## Hard-fail on a missing connection

<!-- source: internal/component/vpp/conn.go -- Connector.WaitConnected -->

`Apply` calls `Connector.WaitConnected(ctx, 5s)` and returns
`vpp not connected after 5s` on timeout. Soft-accept-with-warning and
stash-and-retry were both rejected: they create the failure mode where the
operator believes QoS is active and nothing happened.

`WaitConnected` is public, so any future VPP-dependent synchronous operation uses
it instead of another polling loop.

## State ownership

<!-- source: internal/plugins/traffic/vpp/backend_linux.go -- applyAll, applyInterface, reconcileRemovals -->

The traffic component's reactor holds `previousCfg` and calls `Apply(desired)`
with the full new state. The backend tracks which policer names it bound to which
interface, so it can diff and remove what the new state no longer references.
Neither layer duplicates the other's state.

Each `Apply` opens and closes its own GoVPP channel. The backend struct holds
only a `*vpp.Connector` accessor. This matches fibvpp's per-call channel pattern
and has no pool-draining risk.

### Undo list for a partial failure

Every successful `PolicerAddDel`, `PolicerOutput`, `ClassifyAddDelSession` and
`QosMarkEnableDisable` appends an undo closure to a per-Apply list. On any error
before commit the undos run in reverse, so VPP is back to its pre-Apply state
before the component's journal rollback re-applies the previous config. Without
it, orphaned policers accumulate on a flaky apply path.

### Tolerant reconcile after a VPP restart

Deleting a stale policer index or classify session logs a warning and continues,
rather than failing the whole Apply. After a VPP restart the first Apply programs
the new state and replaces the stale cache. No reconnect subscription is needed.

## Traps

<!-- source: internal/plugins/traffic/vpp/binapi_imports.go -- blank-import anchor -->

- **`PolicerAddDel` returns a new `PolicerIndex`, and `PolicerDel` takes that
  index, not the policer name.** The backend tracks `(name, index)` pairs.
- **`QosEgressMapUpdate` replaces the whole map.** Two DSCP filters on one
  interface, each pushing its own single-entry update, leave only the last one.
  Aggregate at the interface level and push once per interface. An isolated unit
  test does not see this.
- **`fmt.Sscanf` does not support `%[...]` character classes.** A composite
  string key parsed with `fmt.Sscanf(key, "%[^|]|%d", ...)` fails at runtime with
  `bad verb '%['`. Use a typed struct key.
- **Vendored GoVPP does not include every binapi package.** `policer`,
  `policer_types`, `qos` and `classify` were absent at v0.13.0. The fix is a
  blank-import anchor file (`binapi_imports.go`) and then `go mod vendor`. The
  anchor file is permanent, because a non-Linux build does not reference those
  packages through `backend_linux.go`.
