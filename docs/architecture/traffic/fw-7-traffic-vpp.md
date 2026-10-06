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

**The ingress policer is rejected under this rule.** `InterfaceQoS.Ingress`
carries a per-interface upload rate that the tc backend installs as a
`matchall` filter with a `police` action on the `clsact` ingress hook. VPP binds
policers to the egress output arc, and to the ingress classify pipeline for
classes that carry a steering filter, so there is no faithful translation of an
interface-wide upload rate here. `Verify` refuses it at commit and
`applyInterface` refuses it again for a caller that reached `Apply` without
passing through `OnConfigVerify`. Programming the egress half and reporting
success would reproduce, one backend over, the exact defect the ingress policer
was built to close.

<!-- source: internal/plugins/traffic/vpp/verify.go -- errIngressPolicerNotSupportedByBackend -->

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

Each `Apply` opens and closes its own GoVPP channel. The backend holds the
connector accessor and its last successful binding trackers, not a channel.
This matches fibvpp's per-call channel pattern.
<!-- source: internal/plugins/traffic/vpp/backend_linux.go -- backend, Apply -->

### A Ze restart with external VPP

Startup discovers live policers through `PolicerDump`. Its `PolicerDetails`
reply has a name and configuration but **no policer index**. `PolicerDumpV2`
returns the same reply type, so switching dump versions does not supply one.
The backend cannot adopt an index from dump order.
<!-- source: internal/plugins/traffic/vpp/ops_linux.go -- dumpPolicers -->

Before the first apply, Ze unbinds and deletes discovered names in its
`ze/<interface>/<class>` namespace. This includes names still in the config:
`PolicerAddDel(IsAdd=true)` creates, it does not update an existing name.
The apply then recreates desired policers and records the returned indices.
Foreign names are not deleted. An unbind or delete failure aborts startup
rather than declaring cleanup successful.
<!-- source: internal/plugins/traffic/vpp/backend_linux.go -- cleanupStartupOrphans, applyInterface -->

This retains the existing exclusive-interface ownership assumption: a Ze-owned
name identifies an interface whose policing Ze manages. VPP 26.06's output
unbind clears that interface's slot without checking which policer occupies it.
Unrelated names and interfaces remain untouched, but another writer replacing
policing on the same managed interface is not protected by this API.
<!-- source: internal/plugins/traffic/vpp/backend_linux.go -- cleanupStartupOrphans, ifaceNameFromPolicerName -->

This reconciliation has an unpoliced interval between unbind and rebind.
It is not index-preserving adoption or an atomic replacement. Startup cleanup
precedes the apply undo list, so a later failure does not restore the removed
startup objects. Same-process updates use `PolicerUpdate` after confirming that
the cached index still names the intended policer.
<!-- source: internal/plugins/traffic/vpp/backend_linux.go -- applyWithOps, applyInterface -->

The corrected daemon-restart proof exposed duplicate creation on pinned amd64
VPP 26.06. The earlier v25.10 apply evidence did not prove real Ze replacement:
stopping a `docker exec` client had left its daemon and VPP state alive.
The source correction needs a fresh full `./le test deployment vpp-test` run.
<!-- source: internal/le/test/deployment/vppevidencerun.go -- runTrafficInterface -->

### Undo list for a partial failure

New policers, output bindings, classify tables and sessions append undo closures
to a per-Apply list. Before updating an existing policer, its indexed ownership
readback also captures the actual live configuration. On failure, undo runs in
reverse and restores that configuration in place, without deleting the existing
policer. Prior tracked classify bindings and migrated or renamed output bindings
are also restored. A successful output-class rename retains the new binding
while deleting the old policer: VPP has one output slot per interface, so
unbinding the old name would clear the replacement too. Every recovery error
is returned alongside the original apply error; recovery is not an atomic
dataplane transaction and may itself fail.

Startup cleanup is outside this list. The component journal records its undo
only after `Apply` succeeds, so it cannot recover a failed `Apply`; recovery
belongs to the backend.
<!-- source: internal/plugins/traffic/vpp/backend_linux.go -- applyWithOps, applyInterface -->

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
