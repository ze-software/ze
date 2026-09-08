# Doctor and Runtime Health Checks

Ze has three tiers of self-diagnosis, and they differ by WHEN they answer.

| Tier | Command | When the fact is produced |
|------|---------|---------------------------|
| Offline readiness | `ze doctor` | At read time, in the operator's own process, by re-probing config and the environment |
| Live health | `show health` | At read time, inside the daemon, by running a registered probe now |
| Recorded setup outcome | `show plugin list` | Once, in a plugin's own `init()`, before `main()`. The `outcome` and `reason` columns replay it |

The third tier is the only one that remembers. A plugin that failed to set
itself up is often a plugin that never reached the line where it would have
registered a probe, so the first two tiers answer for it with silence, and
absence reads as "not built into this binary". The setup registry answers
because the record is written by the plugin and keyed by its name, and the row
set is derived from the plugin registry, so a plugin that recorded nothing is
listed as `unknown` rather than dropped.

The daemon consults that registry once, at the first statement of `hub.run`,
and refuses to start when a plugin recorded a HARD failure. A CLI verb never
reaches `run`, so no `ze` invocation is refused by it: the command that tells
the operator what is wrong keeps working on a host where the daemon will not
boot.

## Kernel capabilities: one enrolment, three callers

A subsystem needs a kernel feature the host can lack. MPLS forwarding needs an
AF_MPLS table. IPsec needs the XFRM dataplane. Ze does not discover the absence
when the first labeled route or the first Child SA fails: it asks before it
starts, and it refuses.

The enrolment is `internal/component/kernelcap`. A subsystem registers from its
OWN package, and the registration carries five things: the subsystem name, the
kernel feature, the configuration that requires it, the two diagnostic codes,
and two functions. `InUse` reads the parsed config tree. `Probe` reads the host.

<!-- source: internal/component/kernelcap/kernelcap.go -- Capability, MustRegister, Evaluate, Refuse -->
<!-- source: internal/component/ike/engine/kernelcap_linux.go -- the ipsec enrolment -->
<!-- source: internal/plugins/fib/kernel/kernelcap_linux.go -- the mpls enrolment -->

This is the `ze doctor` tier of the table above, not a fourth one. The verdict is
produced at read time, in the reader's own process, and it keeps no memory of a
start. It cannot move into the setup registry, whose records are written before
`main()` and therefore cannot see the configuration this verdict depends on.

`MustRegister` registers the doctor check too, so one call from the owner puts
the capability on all four surfaces. A second registration would be a second
declaration of the same fact.

### Three states, and the third one is why this is safe

| Probe answer | Diagnostic | What the daemon does |
|--------------|-----------|----------------------|
| Present | none | starts |
| Absent | `SeverityError`, the subsystem's absent code | refuses, exit 1 |
| Cannot determine | `SeverityWarning`, the subsystem's unknown code | starts |

A capability that could not be DETERMINED never refuses. The gate runs on every
`ze` start on Linux, so refusing on a probe ze could not read would turn a
working deployment into a dead one. A probe that returns no verdict at all is
reported the same way: the zero `State` is `Unspecified`, and it is never read as
a pass.

The probes read netlink or procfs. Neither executes an external binary: a second
dependency that can be absent for its own reasons is the fault this enrolment
removes, not a way to detect it. `TestCapabilityProbeExecsNoBinary` holds the
rule over the package's source.

### Four callers, one verdict

| Caller | Where | What it does |
|--------|-------|--------------|
| `ze doctor` | the registered check, run by `runChecks` | renders the diagnostic; `Run` already exits 1 on any `SeverityError` |
| daemon start | `runYANGConfig`, after `applyEvolutions` and before `EnsureActiveVersion` | one stderr line, one `logStartupFailure`, `return 1` |
| config reload | `runReloadContext`, before `ReloadConfig` | refuses with the running configuration still serving |
| `ze config validate` | `runValidation` | `config-kernel-capability` at error severity |

<!-- source: cmd/ze/hub/main.go -- runYANGConfig, the kernel capability refusal -->
<!-- source: cmd/ze/hub/main_reload.go -- runReloadContext, the same gate before ReloadConfig -->
<!-- source: internal/component/config/cli/cmd_validate.go -- runValidation -->

The start refusal keeps the idiom the plugin setup refusal uses, and for the same
reason it names EVERY failing subsystem rather than the first: an operator who
repairs one fault and restarts to meet the next pays a whole boot for each fault
after the first.

`ze config validate` answers about the host running the command. A config written
for another machine and validated on a workstation is judged against the
workstation. That is why the gate is not inside `LoadConfig`, which also runs
under `ze doctor` and under offline validation.

There is no operator override, by owner decision (2026-08-14). A NOS that
half-works on a kernel missing a required feature is the hazard this removes, and
an override is what an operator reaches for under pressure.

### The predicate decides more than the refusal

`IPsecInUse` has three readers: the gate, the kernel module check and the
listener check. An empty `vpn { ipsec { } }` block installs no Security
Association, so it refuses no start, warns about no module and binds no IKE port.
Over-reporting is the same defect in all three.

`MPLSInUse` checks `fib { kernel { } }` BEFORE it looks at any labeled family. A
VPP or P4 backend does its own MPLS, and the plugin that programs kernel labels
is the one that carries the requirement.

### A dead label space is repaired, not reported

`net.mpls.platform_labels` is the size of the kernel's label table and it
defaults to 0, which disables MPLS. A kernel that builds MPLS in therefore boots
with the table present and the label space empty, which is the state every ze
appliance starts in.

So the MPLS probe reads EXISTENCE only, and the fib kernel plugin writes the
label space once, immediately before it programs its first label. An operator's
own value is never overwritten: only a table reading exactly 0 is repaired.

<!-- source: internal/plugins/fib/kernel/labelspace_linux.go -- ensureLabelSpace, repairLabelSpace -->

## Two tiers on one topic: locking the executable

`memlock` shows why tier one and tier three are not the same fact, and why
neither derives from the other.

| Question | Tier | What answers it |
|----------|------|-----------------|
| Can THIS HOST lock the ze executable at all? | `ze doctor`, before ze runs | The `memlock-rlimit` check compares `RLIMIT_MEMLOCK` against the size of `/proc/self/exe` |
| Did THIS PROCESS lock it? | `show plugin list`, after ze ran | The `memlock` row's `outcome`, recorded by the plugin's own `init()` |

The doctor check warns under `doctor-memlock-rlimit-low` when the limit is
below the executable's size. The file size is a FLOOR for the mapped size, so a
limit below it cannot possibly hold the executable, a limit above it still
might not, and the check claims only the first. It stays silent for a process
holding `CAP_IPC_LOCK`, which locks what it likes whatever the limit says, so
it does not warn on an appliance where ze runs as root. When it cannot read the
host it says so under `doctor-memlock-rlimit-unknown` rather than passing.

The registry cannot answer the first question, because it replays what already
happened on a host where ze already started. The doctor check cannot answer the
second, because it runs in the operator's own process rather than the daemon's.
<!-- source: internal/plugins/memlock/doctor_linux.go -- checkMemlockLimit, memlockLimitDiagnostics -->
<!-- source: internal/plugins/memlock/memlock_linux.go -- init, the outcome it records -->

<!-- source: internal/component/doctor/doctor.go -- offline checks and listener collection -->
<!-- source: internal/core/health/registry.go -- runtime health registry -->
<!-- source: internal/component/plugin/registry/setup.go -- RecordSetup, SetupResults, HardSetupFailures -->
<!-- source: cmd/ze/hub/startup_gate.go -- hardSetupFailure, the refusal run applies -->

## Discovery beats enumeration

Listener checks are derived from the YANG schema, through
`RegisterListenerDefault` and `CollectListenersWithDefaults`, with a hardcoded
list as a fallback. A new `ze:listener` service is then covered by doctor with
no doctor code change. The alternative, extending the YANG compiler to
propagate `refine` defaults, would have touched every consumer of the compiler.

<!-- source: internal/component/config/listener_defaults.go -- builtin listener defaults -->
<!-- source: internal/le/portdefaults/actions.go -- Answer -->

Two constraints hold this together:

- The ze YANG compiler does not process `refine`, so `LeafNode.Default` is
  always empty for a `uses zt:listener` child. Every schema-driven default
  lookup has to allow for that, not only the listener one.
- `config.YANGSchema()` succeeds with an empty schema when no YANG module is
  registered, which is the normal unit-test context. The schema-driven path
  checks that the discovered service list is non-empty and falls back, or a
  hand-built config tree gets no listener checks at all.

`show doctor` reaches the same checks through a provider
(`diagnostic.RegisterDoctorProvider`) rather than by importing `cmd/ze/doctor`
from `internal/component/cmd/show`. The dependency direction stays cmd to
internal.

## Probe the role the service actually plays

NTP is a client in ze, not a server. The reachability check sends an SNTP
probe over UDP to each configured server, matching the daemon's own clock-skew
probe. A TCP connect would hit a closed port and report a false failure. Doctor
does bind UDP/123 separately, for the NTP listener check.

Injectable function variables (`httpHead`, `probeWritable`,
`ntpServerReachable`) isolate the probes in tests, which matches the existing
doctor test style. `probeWritableDir` removes its temporary file before
checking the `Close()` error, otherwise a failing close leaks the file.

## Health checks produce warnings before reading them

`checkFirewallHealth` calls `AuditTables()` and `checkIfaceHealth` calls
`CheckAllInterfaceErrors()`. Each kernel call runs on a goroutine with a
one-second timeout, so a stuck kernel call cannot stall the health endpoint.

<!-- source: internal/component/firewall/audit.go -- firewall drift audit -->
<!-- source: internal/component/iface/health.go -- interface error counter tracking -->
<!-- source: internal/component/bgp/reactor/session_health.go -- EOR timeout and session anomaly -->

Four bounds keep the checks honest:

| Check | Bound | Reason |
|-------|-------|--------|
| Route-count anomaly | 100-prefix floor | a route-server client or a management peer holds 1 to 5 routes, and any ratio on those is noise |
| Per-family EOR | one timer per peer, expected against received family count | the warning clears only when every negotiated family has sent EOR, not on the first one |
| Firewall drift | skipped while `LastApplied() == nil` | before the first apply there is nothing to compare, so every table reads as drift |
| VPP health | `os.Stat` on the socket before dialing | without it a host with no VPP answers 503 forever |

Prefix counting is unconditional rather than gated on a configured maximum,
because the anomaly detector needs a count on every peer. An early return in
`applyPrefixDelta` skips the family-string allocation when neither metrics nor
warnings are configured.

The FIB pending map is capped at 10000 entries, so a catastrophic backend
failure cannot grow it without bound.

## The dependency inventory is a test

`TestDoctorDependencyInventory` holds an expected total. Adding a runtime
dependency without a doctor check fails it, which forces the decision to be
made rather than forgotten.

## Which check a new dependency needs

| New dependency | Doctor check |
|----------------|--------------|
| Config leaf that references a file path (cert, key, binary) | File existence check |
| Config leaf that names an external service or socket | Reachability probe |
| Kernel feature a configured subsystem cannot work without | A `kernelcap.Capability` registered from the owning package. `/proc/modules` cannot answer it: a built-in feature lists no module |
| Kernel module requirement with no configured subsystem behind it | `/proc/modules` check (Linux) |
| New listen address or port | Port bind probe |
| New UDP listener | UDP `ListenPacket` bind probe |
| New service with TLS | Certificate validity and expiry check |
| Embedded certificate material | Parse the certificate and check its validity window |
| External binary (plugin, helper) | `exec.LookPath` or `os.Stat` check |
| Procfs or sysctl dependency | Read and write probe for the exact `/proc` path |
| Netlink dependency | Open the specific netlink family or handle |

## Where each owner registers its check

<!-- source: internal/core/diagnostic -- RegisterDoctorCheck -->

| Dependency owner | Registration mechanism |
|------------------|------------------------|
| Internal plugin (registered by `registry.Register`) | The `Registration.DoctorChecks` field. The doctor runner bridges these at execution time through `checks_plugin_registry.go`. The check function takes `registry.DoctorCheckContext` and returns `[]rpc.DoctorCheckDiagnostic`, and Component is set from the plugin name. `l2tpauthradius/register.go` is the reference example |
| Web, MCP, looking-glass, or other listener component | `diagnostic.RegisterDoctorCheck()` from the owning component's `init()` |
| SSH host-key dependency | `diagnostic.RegisterDoctorCheck()` from the SSH component |
| Interface backend | `diagnostic.RegisterDoctorCheck()` from the backend owner |
| Kernel module, procfs, sysctl, netlink, VPP, or platform-specific backend | `diagnostic.RegisterDoctorCheck()` from the owning backend or component, with build-tagged files where needed |
| Blob storage, platform detection, generic runner state, or a dependency with no narrower owner | `internal/component/doctor`, with a comment or test name making the absent owner explicit |

## The two tests a new check carries

| Test type | What it proves | Location |
|-----------|----------------|----------|
| Unit test | The check fires only when its config block is present, and emits the registered code | The owning package beside the registration, or `internal/component/doctor` when there is no narrower owner |
| Functional test | `ze doctor --json <config>` exposes the behavior through the user entry point | `internal/component/doctor`, or the existing functional suite for that entry point |
