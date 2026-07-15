# Spec: vrrp-6 -- keepalived interop + live failover validation

| Field | Value |
|-------|-------|
| Status | ready |
| Depends | spec-vrrp-5 |
| Phase | - |
| Updated | 2026-07-14 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `plan/spec-vrrp-0-umbrella.md` -- decisions, AC-1..AC-4, A-5, R-7
3. `.claude/rules/planning.md` -- workflow rules
4. `ai/rules/qemu-testing.md` -- the interop-lab four-step pattern (netns script, Alpine peer, runtime kernel, make target)
5. `rfc/short/rfc9568.md` + `rfc/short/rfc3768.md` -- wire-assertable behaviors
6. `scripts/evidence/effective-l2tp-ppp.py` -- the netns evidence-script blueprint
7. `test/interop/interop.py` + `test/interop/run.py` -- container harness extension points

## Task

Prove ze's VRRP implementation (children 1-5) interoperates with keepalived, the
dominant independent implementation, and that live failover behaves per RFC 9568
(v3) and RFC 3768 (v2 opt-in) on a real Linux kernel. Three deliverable groups:

1. **QEMU netns evidence script** `scripts/evidence/effective-vrrp-keepalived.py`
   (blueprint: `effective-l2tp-ppp.py`): ze and keepalived (Alpine package) in
   separate network namespaces joined through a bridge, plus an observer
   namespace running tcpdump and dataplane probes. Eight scenarios with explicit
   wire-level assertions (advert fields, TTL/hop-limit, GARP/NA frames, timing
   deltas from capture timestamps, ARP/ND cache contents, VIP reachability).
2. **Container interop scenarios** in `test/interop/scenarios/` against a new
   keepalived container (`Dockerfile.keepalived` + harness hook), mirroring the
   config-file-presence launch pattern the harness already uses for FRR, BIRD,
   and GoBGP. Three scenarios: v3 election, v2 election, graceful failover.
3. **needs-linux `.ci` tests** in `test/vrrp/` driving the single ze daemon with
   a raw-socket advert injector: backup hold, master-down failover, preempt-false.

This child validates umbrella AC-1..AC-4 and risk R-7 end to end. It adds no Go
feature code; its feature surface is the make targets, the harness extension,
and the evidence script (test infrastructure per `ai/rules/discovery-updates.md`).

## Required Reading

### Architecture Docs
<!-- NEVER tick [ ] to [x] -- checkboxes are template markers, not progress trackers. -->
- [ ] `ai/rules/qemu-testing.md` - interop labs and Docker-based tests need a QEMU runner
  → Constraint: the four-step pattern is mandatory and shipped in one change: netns evidence script mirroring effective-l2tp-ppp.py (LineCollector, marker waits, kernel probe, cleanup), peer from Alpine packages via `--packages`, runtime kernel via `--kernel tmp/kernel/vmlinuz` when a CONFIG_* is needed, `ze-qemu-<feature>-test` target in mk/test-integration.mk added to .PHONY and the Makefile help block, plus a row in the rule's interop-lab table (`ai/rules/qemu-testing.md:172`)
  → Decision: CONFIG_MACVLAN=y, CONFIG_BRIDGE=y, CONFIG_VETH=y are already in `gokrazy/kernel/runtime.config:45-47`, so NO kernel-config addition is needed; the target uses `--kernel tmp/kernel/vmlinuz` (like `ze-qemu-l2tp-ppp-test`, `mk/test-integration.mk:409-416`) to guarantee macvlan is built in rather than depending on the stock Alpine ISO kernel's module set
- [ ] `ai/rules/interop-and-goal-validation.md` - interop is mandatory for wire protocols
  → Constraint: check.py contract: (1) wait for convergence (master/backup state = the wait_session equivalent), (2) assert the specific protocol behavior, (3) re-verify stability afterward, (4) log_pass/log_fail, (5) raise on failure; Goal Validation table maps each umbrella goal to concrete scenario evidence
- [ ] `ai/rules/testing.md` (sleep ratchet + payload-predicate waits) - deterministic waits
  → Constraint: the `time.sleep(` count across `test/**/*.ci` may only go down (`test/.ci-sleep-baseline`); .ci observers use `wait_for_event`/`wait_until`/`dispatch_until` and `runtime_fail`, never `sys.exit(1)`; the injector paces advert transmission with select() timeouts on its raw socket so the new .ci files contain zero `time.sleep(` occurrences
  → Constraint: evidence-script waits are predicate polls with deadlines (LineCollector.wait_for model, `scripts/evidence/effective-l2tp-ppp.py:283`); the only permitted fixed durations are protocol-defined observation windows (master-down interval, skew), asserted from tcpdump wire timestamps, not wall-clock sleeps
- [ ] `ai/rules/spec-no-code.md` + `ai/rules/planning.md` (Spec Rules, Risks & Assumptions)
  → Constraint: tables and prose only; plain fences for topology/config text; every assumption carries a Basis and a validation method
- [ ] `plan/spec-vrrp-0-umbrella.md` - scope row child 6, AC-1..AC-4, A-5, R-7
  → Decision: umbrella commits child 6 to BOTH interop paths: container scenarios `test/interop/scenarios/vrrp-*-keepalived/` (umbrella Interop Tests table + Files to Create) AND the QEMU netns evidence script; dropping either is a scope reduction requiring user approval, so this spec extends the container harness rather than deferring it

### RFC Summaries (MUST for protocol work)
- [ ] `rfc/short/rfc9568.md` - VRRPv3: what is assertable on the wire
  → Constraint: adverts: version/type nibbles 3/1, TTL/hop-limit exactly 255, dst 224.0.0.18 / ff02::12, source MAC = 00:00:5e:00:01:{vrid} (v4) / 00:00:5e:00:02:{vrid} (v6), Max Advertise Interval in centiseconds (12-bit, default 100), IPv6 first listed address MUST be link-local; prio-0 advert on graceful shutdown; Backup promotes after Skew_Time on prio-0 (Skew = ((256-prio)*Active_Adver_Interval)/256 cs) vs full Active_Down_Interval (3*interval + skew) on silence; on promotion: gratuitous ARP per IPv4 VIP (sender IP == target IP == VIP, sender MAC = virtual MAC; errata 7947/7949) or unsolicited NA per IPv6 VIP (type 136, R=1 S=0 O=1, TLL option = virtual MAC); Active tie-break: equal priority resolved by greater primary IPvX address; non-owner Active with Accept_Mode False MUST NOT accept packets to the VIP (ping needs accept-mode true)
- [ ] `rfc/short/rfc3768.md` - VRRPv2: opt-in wire format keepalived speaks by default
  → Constraint: version nibble 2, Auth Type byte (0 = none is the only ze-supported value), Adver Int in whole seconds (8-bit, default 1), checksum WITHOUT pseudo-header, mandatory 8-byte Authentication Data trailer (total length = 8 + 4*count + 8), receiver discards on interval mismatch (unlike v3), IPv4 only

**Key insights:**
- The QEMU netns lab is the mandatory-runnable interop path (macOS dev machines cannot run the Docker lab's kernel features, `ai/rules/qemu-testing.md:137-145`); the container path adds cross-environment coverage on Linux hosts with Docker, exactly like `test/interop/scenarios/isis-p2p-frr/` (its check.py:10-12 documents "Linux Docker/QEMU harness ONLY").
- Everything asserted here is observable without touching ze internals: tcpdump line output (fields + timestamps), keepalived notify markers, `ip -j neigh` / `ip -j addr` JSON, and ping exit codes.
- keepalived speaks v2 by default for IPv4 and v3 for IPv6 or with `vrrp_version 3`; a v3-only ze silently ignores v2 adverts, so scenario pairing must set versions explicitly on both sides.

## Current Behavior (MANDATORY)

**Source files read:** (producers read directly this session)
- [ ] `scripts/evidence/effective-l2tp-ppp.py` - the netns evidence blueprint: PID-suffixed netns/veth names :29-34, setup_netns with underlay ping proof :144-190, kernel capability probe ensure_kernel_support :193-221, host ze build ensure_ze (tags ze_core,ze_distro into tmp/evidence/bin) :232-258, LineCollector thread + predicate wait_for(predicate, timeout, proc, fatal) :261-304, fatal-needle early abort :307-333, marker-wait orchestration :596-654, dataplane ping verify :422-431, teardown-to-initial-state wait :435-473, diagnostics dump on failure :695-726, cleanup in finally :727-733
- [ ] `test/interop/interop.py` - Scenario.setup() launches peer containers by config-file presence: `frr.conf` :1299-1313, `bird.conf` :1316-1326, `gobgp.toml` :1329-1339; ze container always started with NET_ADMIN :1288-1296; docker_run helper :248-257; network create/retry :139-175; teardown :1344-1353. NO keepalived support today; the extension point is mechanical (one more config-file hook + container constants + teardown row)
- [ ] `test/interop/run.py` - build_images builds ze-interop/bird-interop/gobgp-interop and pulls FRR :31-93; scenario discovery = every `scenarios/<dir>` containing check.py :133-145; a keepalived image build slots in beside the BIRD build :53-67
- [ ] `test/interop/scenarios/isis-p2p-frr/check.py` - precedent for a raw-L2 (non-TCP) protocol in the container lab; docstring pins it to the Linux Docker/QEMU harness, cannot run on darwin :10-12
- [ ] `test/interop/daemons` - FRR daemons file ships `vrrpd=no` :23 (the FRR-vrrpd fallback would need this flipped; keepalived is the chosen peer, so it stays `no`)
- [ ] `test/interop/Dockerfile.ze` - ze container entrypoint `tini -- ze` with the scenario config mounted at /etc/ze/bgp.conf :23-31; NET_ADMIN (interop.py:1293) permits macvlan creation and raw proto-112 sockets inside the container
- [ ] `scripts/evidence/qemu-run.py` - CLI surface: `--run`, `--keep-alive`, `--packages` (Alpine apk), `--timeout`, `--kernel` (custom kernel, e.g. tmp/kernel/vmlinuz) :480-510
- [ ] `mk/test-integration.mk` - target patterns to mirror: `ze-qemu-l2tp-ppp-test` :409-416 (tmp/kernel/vmlinuz guard + qemu-run.py --kernel --packages --run --timeout), `ze-qemu-isis-frr-test` :360-364 (tcpdump in --packages), `ze-qemu-pppoe-accel-test` :423-430 (runtime-kernel interop sibling); .PHONY qemu block :28; `ze-interop-test` drives test/interop/run.py with optional INTEROP_SCENARIO filter :32-36
- [ ] `internal/test/runner/record_parse.go` - `needs-linux` option handling :383-395 (on GOOS != linux the runner sets SkipReason, test reports SKIP; inert on Linux so QEMU runs it); ZE_QEMU_LINUX_ONLY tight loop :222-228
- [ ] `test/isis/isis-adjacency.ci` - the suite-split model :15-23: config-validate surface runs natively; live behavior is explicitly delegated to QEMU integration + the interop scenario; header comments name the owning specs
- [ ] `gokrazy/kernel/runtime.config` - CONFIG_MACVLAN=y :45, CONFIG_BRIDGE=y :46, CONFIG_VETH=y :47 already present (added by earlier appliance work); `make ze-kernel` stages tmp/kernel/vmlinuz and rebuilds when runtime.config changes

**Behavior to preserve:** (unless user explicitly said to change)
- Existing interop scenarios, harness launch order, and container/network names untouched; the keepalived hook is additive (new config-file key `keepalived.conf`)
- `test/interop/daemons` keeps `vrrpd=no`
- Existing qemu targets and their package lists unchanged
- `test/.ci-sleep-baseline` count does not increase (new .ci files contain zero `time.sleep(`)
- `ze-verify` scope unchanged: all new targets are integration-tier, never part of the default gate (mk/test-integration.mk:1-5)

**Behavior to change:**
- None removed. New: `ze-qemu-vrrp-keepalived-test` target; keepalived container hook + image build in the interop harness; `scripts/evidence/effective-vrrp-keepalived.py`; three container scenarios; three needs-linux .ci tests; docs/functional-tests.md + qemu-testing lab-table registration.

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- Operator/CI: `make ze-qemu-vrrp-keepalived-test` (QEMU lab), `make ze-interop-test INTEROP_SCENARIO=vrrp-v3-keepalived` (container lab), `make ze-qemu-needs-linux-test` / `make ze-qemu-all-test` (.ci tests inside the VM)
- Wire: proto-112 multicast adverts, GARP broadcasts, unsolicited NA multicasts between the ze and keepalived namespaces/containers over a shared L2 segment

### Transformation Path
1. Make target invokes `scripts/evidence/qemu-run.py --kernel tmp/kernel/vmlinuz --packages "keepalived tcpdump iproute2 iputils-ping kmod python3" --run 'python3 scripts/evidence/effective-vrrp-keepalived.py'`
2. Inside the VM the evidence script builds ze from /workspace (ensure_ze pattern), creates the lab: netns `lan` holding bridge br0; veth pairs ze0/ka0/ob0 with peer ends enslaved to br0; addresses per scenario
3. Per scenario: write ze.conf + keepalived.conf into a work dir; start tcpdump (line-buffered, -e -vv) in the observer netns feeding a LineCollector; start ze in its netns; start `keepalived -n -l -f <conf>` in its netns with notify markers appended to a state file
4. Predicate waits: capture-ready marker ("listening on"), keepalived state markers (MASTER/BACKUP lines), advert lines matching version/vrid/prio/interval, GARP/NA lines, `ip -j neigh` / ping probes from the observer netns
5. Assertions compare captured payload fields and tcpdump timestamps against RFC-derived expectations; any failure dumps diagnostics and exits non-zero; cleanup restores namespaces
6. Container path: run.py builds images -> interop.py Scenario.setup() starts ze + keepalived containers from scenario configs -> check.py converges on keepalived state markers, then asserts VIP reachability, MAC ownership, and stability
7. .ci path: the runner (needs-linux) boots one ze daemon whose config creates a veth pair + vrrp group; an embedded python plugin injects crafted adverts through a raw proto-112 socket on the peer veth end and asserts ze's state transitions via production log expectations + `runtime_fail`

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Host make ↔ QEMU VM | qemu-run.py boots Alpine VM, repo mounted at /workspace (virtio-9p), command via --run | [ ] |
| Evidence script ↔ namespaces | `ip netns exec` wrappers (ns_run/ns_popen blueprint) | [ ] |
| ze ↔ keepalived | proto-112 multicast + GARP/NA over the br0 L2 segment (the actual interop surface) | [ ] |
| Script ↔ tcpdump | line-buffered stdout into LineCollector; predicates over parsed lines + timestamps | [ ] |
| Script ↔ keepalived state | notify_master/notify_backup/notify_fault shell markers appended to a file the script polls | [ ] |
| run.py ↔ docker | image build + container lifecycle (existing harness machinery) | [ ] |
| check.py ↔ containers | docker exec probes (state file, ip -j neigh, ping) via the interop module helpers | [ ] |
| .ci plugin ↔ kernel | SOCK_RAW proto 112 on the veth peer end, inside the daemon's netns (QEMU root) | [ ] |

### Integration Points
- `scripts/evidence/qemu-run.py` CLI (:480-510) - unchanged, consumed by the new target
- `test/interop/interop.py` Scenario.setup() (:1299-1339) - gains the keepalived config-file hook following the FRR/BIRD/GoBGP pattern
- `test/interop/run.py` build_images (:31-93) - gains the Dockerfile.keepalived build
- `internal/test/runner` needs-linux option (record_parse.go:383-395) - discovers the new .ci tests automatically in `ze-qemu-needs-linux-test` / `ze-qemu-all-test`; no per-test wiring
- `test/scripts/ze_api.py` - `runtime_fail`, `wait_for_event`, `wait_until` used by the .ci injector/observer plugins
- spec-vrrp-5 deliverables consumed: vrrp YANG config shape, production state-transition log lines, `show vrrp` payloads (exact markers finalized against child 5's naming at implementation)

### Architectural Verification
- [ ] No bypassed layers (assertions observe the wire and kernel state; nothing reaches into ze internals)
- [ ] No unintended coupling (harness keepalived hook is generic "peer container from config file", no vrrp-specific logic in interop.py beyond the launch hook; scenarios carry all protocol knowledge)
- [ ] No duplicated functionality (reuses qemu-run.py, LineCollector blueprint, ze_api waits, existing image-build machinery)
- [ ] Zero-copy preserved where applicable (N/A -- no Go code in this child)
- [ ] Registration over hardcoding -- the .ci tests are discovered by the runner's suite scan (no per-test wiring, record_parse.go needs-linux contract); container scenarios are discovered by run.py's scenarios/-directory scan (:133-145); the peer container launches via the established config-file-presence registration pattern; no vrrp spelling is added to any central Go package (`ai/rules/plugin-self-containment.md`)

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | keepalived from the Alpine package runs foreground (`-n -l`) inside a netns and inside a container (umbrella A-5) | qemu-testing.md:150-155 Alpine-peer pattern (xl2tpd/accel-ppp/frr precedent); keepalived is in Alpine community | Fall back to FRR vrrpd (flip `test/interop/daemons:23` vrrpd=yes) and translate scenario configs | QS-1 boots keepalived in the netns lab; CS-1 in the container | unvalidated |
| A-2 | tcpdump's VRRP/ARP/ICMPv6 printers expose every asserted field: version, vrid, prio, advert interval, authtype, addr list (-vv), TTL/hop-limit (-v), GARP sender/target, NA flags + TLL option, ether src (-e) | tcpdump already used for wire assertions in `ze-qemu-isis-frr-test` (mk/test-integration.mk:363) | Parse pcap (`tcpdump -w` + stdlib-only Python parser, bgpgen.py precedent) instead of line output | QS-1 implementation (first scenario exercises every printer) | unvalidated |
| A-3 | The Docker bridge network floods proto-112 multicast, ARP broadcast, and ff02::/16 multicast between containers, and the Docker VM kernel provides macvlan for ze's virtual-MAC device | isis-p2p-frr runs raw-L2 multicast (IS-IS) over the same bridge (check.py:10-12); docker's own macvlan network driver implies kernel support | Container scenarios documented Linux-host-only (already the isis precedent); QEMU lab remains the mandatory-runnable path | CS-1 on a Linux Docker host | unvalidated |
| A-4 | keepalived notify scripts (notify_master/notify_backup/notify_fault) fire deterministically on every state change, giving a payload-predicate state source | keepalived documented notify contract; markers are file-append shell one-liners | Use `vrrp_notify_fifo` or SIGUSR1 /tmp/keepalived.data dumps as the state source | QS-1/QS-2 (BACKUP then MASTER markers observed) | unvalidated |
| A-5 | The runtime kernel provides every lab kernel feature: CONFIG_MACVLAN=y, CONFIG_BRIDGE=y, CONFIG_VETH=y | read directly, `gokrazy/kernel/runtime.config:45-47` | Add the missing CONFIG_* + runtime.require symbol per qemu-testing step 3 | `make ze-kernel` + QS-1 boot (script kernel probe asserts macvlan/bridge/veth creatable) | **VALIDATED 2026-07-15, and superseded: the custom kernel is not needed at all.** Probed the STOCK Alpine kernel (6.12.13-0-virt) directly in the VM: `dummy`, `macvlan mode bridge`, `bridge`, `veth` and `ip netns add` all create cleanly. The lab therefore drops `--kernel tmp/kernel/vmlinuz` (which this spec's "Files to Modify" had specified), matching the isis-frr/ldp-frr precedent. Rationale for the change: the l2tp/pppoe labs need `--kernel` because the stock kernel lacks CONFIG_PPPOL2TP/CONFIG_PPPOE; VRRP needs no such symbol, and keeping `--kernel` would force a ~30-minute `make ze-kernel` build on anyone running the target for zero benefit. `ai/rules/qemu-testing.md` step 3 updated to say the custom kernel is conditional on a genuinely missing CONFIG_*. |
| A-9 | keepalived in Alpine speaks VRRPv3 and can build the virtual MAC | added 2026-07-15 during implementation | Fall back to FRR vrrpd per A-1 | **VALIDATED**: probed in the VM, Alpine ships keepalived v2.3.1 (05/24,2024) built with `VRRP`, `VRRP_AUTH`, `VRRP_VMAC`, `VRRP_IPVLAN`; tcpdump 4.99.5 present. Both install from stock Alpine repos with no custom kernel. |
| A-6 | A single-daemon .ci with a raw-socket python injector plugin can emulate the VRRP peer (two-full-daemon .ci is not feasible: the runner boots exactly one DUT) | .ci tmpfs python plugin pattern + ze-created veth pair (iface YANG veth :586 per umbrella); QEMU runs .ci as root so SOCK_RAW works | Drop the three .ci tests; the evidence script alone carries live coverage and the Functional Tests table is re-justified | `test/vrrp/vrrp-backup-hold.ci` under `make ze-qemu-needs-linux-test` | unvalidated |
| A-7 | spec-vrrp-5 ships an accept-mode leaf (v3) so a non-owner Active answers ping to the VIP; scenario ping assertions depend on it | RFC 9568 §6.1/§6.4.3 (Accept_Mode False forbids accepting VIP-addressed packets); umbrella child-2 scope lists accept-mode | Ping-the-VIP assertions become route-through-the-VIP assertions (observer routes a probe prefix via the VIP as gateway), which Accept_Mode does not gate | Cross-check spec-vrrp-5 YANG during /ze-implement audit; QS-1 ping | unvalidated |
| A-8 | ze's state transitions are observable from outside via production log lines (backup/master transitions) and `show vrrp`, stable enough for expect=stderr patterns | umbrella children 2/5 define FSM + show/telemetry surface | Fall back to `dispatch_until` on `show vrrp` JSON payloads only | .ci implementation against the landed child-5 log naming | **VALIDATED 2026-07-15**: `emitStateChange` (`internal/plugins/vrrp/engine.go:318-327`) logs `vrrp: state change` at Info with structured fields `interface`, `unit`, `family`, `group`, `vrid`, `from`, `to`, `reason`, `virtual-addresses`. `viewState` (`internal/plugins/vrrp/vrrp.go:61-72`) renders the state as exactly `initialize` / `backup` / `master`, so `to=master` is a stable payload predicate for both the netns lab's LineCollector and `.ci` expect=stderr. It is a production log line (not test-only), so asserting on it does not create a test-only surface. |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | (umbrella R-7) Duplicate-VRID equal-priority convergence flaps under QEMU TCG timing jitter instead of settling on the IP tie-break | QS-7 sees alternating advert sources in the final observation window | Deterministic winner (ze gets the numerically greater primary IP); convergence window scaled to advert interval; stability asserted over the last 3 master-down intervals of capture, not instantaneously |
| R-2 | Tight timing assertions (skew < promotion < master-down) flake on loaded hosts / TCG | Intermittent QS-2/QS-3 failures with measured deltas near bounds | All deltas measured from tcpdump wire timestamps (not wall clock); acceptance bands = protocol bound + fixed margin (Boundary Tests table); no lower bounds tighter than the protocol requires |
| R-3 | keepalived version drift changes config keywords or log formats across Alpine releases | keepalived config parse error at scenario start | Assertions pinned to notify markers + wire behavior, never keepalived log text; script runs `keepalived -t -f <conf>` as a config gate and fails with the version string in the diagnostics |
| R-4 | Dead-master simulation: after SIGKILL, ze's kernel netns still answers ARP for the VIP (macvlan + address survive the process), masking failover | QS-2 observer neigh cache never repoints; keepalived promotes but ping keeps resolving to the old MAC | Node death = SIGKILL + `ip link set <ze-veth> down` (carrier loss, the cable-pull model); assert the neigh repoint only after keepalived's GARP is observed |
| R-5 | tcpdump misses the first frames of a short GARP/NA burst (capture startup race) | QS-2/QS-6 promotion observed but no GARP/NA line captured | Capture starts before the trigger and the script waits for tcpdump's "listening on" marker before acting; failover triggers are script-driven so ordering is controlled |
| R-6 | Orphaned lab state (netns, veth, bridge, stopped processes) pollutes later runs or the host VM | Second scenario fails on name collisions | PID-suffixed names + cleanup-first setup + finally-cleanup, exactly the blueprint (:29-34, :124-141); per-scenario teardown returns to a probed-clean baseline before the next scenario |
| R-7 | v2 scenario false-positive: keepalived silently discarding malformed ze v2 adverts looks identical to "backup by choice" | CS-2/QS-5 pass while keepalived logs checksum/auth drops | Positive assertion added: after ze stops advertising, keepalived MUST promote within its v2 master-down interval -- it only does that if it accepted and timed against ze's adverts; plus wire-format field assertions (authtype 0, intvl 1s, length includes the 8-byte trailer) |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| `make ze-qemu-vrrp-keepalived-test` | → | qemu-run.py boots runtime kernel, runs effective-vrrp-keepalived.py scenarios QS-1..QS-8 | evidence script exits 0 printing one `PASS: QS-N ...` marker per scenario (target run recorded in Pre-Commit Verification) |
| `make ze-interop-test INTEROP_SCENARIO=vrrp-v3-keepalived` | → | run.py builds keepalived image; interop.py keepalived.conf hook starts the peer container | `test/interop/scenarios/vrrp-v3-keepalived/check.py` |
| Proto-112 advert (higher prio) received by ze in Backup | → | transport rx -> packet decode -> FSM Backup rearm (children 1/2/4/5 chain) | `test/vrrp/vrrp-backup-hold.ci` (needs-linux) |
| Master-down expiry after injector silence | → | FSM promotion -> VIP install + GARP + advert tx | `test/vrrp/vrrp-failover.ci` (needs-linux) |
| Lower-prio advert with ze preempt false | → | FSM rearm without preemption (no Master transition) | `test/vrrp/vrrp-preempt.ci` (needs-linux) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | QS-1: ze prio 200 (v3, IPv4, advert 1000 ms) vs keepalived prio 100 (`vrrp_version 3`) on one L2 segment | keepalived notify marker settles BACKUP; tcpdump shows only ze-sourced adverts: VRRPv3/type 1, vrid 10, prio 200, interval 100cs, IPv4 TTL 255, ether src 00:00:5e:00:01:0a; observer ARP for the VIP resolves to 00:00:5e:00:01:0a and the VIP answers ping (accept-mode true, A-7) |
| AC-2 | QS-2: node death (SIGKILL ze + ze-side veth link down), later fresh ze restart (link up + start) | keepalived marker MASTER within the [3.0 s, 6.0 s] acceptance band measured from the last ze advert wire timestamp, sends its own GARPs + prio-100 adverts; after restart ze preempts: promotion after its own master-down (about 3.22 s at prio 200), GARP burst captured with sender IP == target IP == VIP and sender MAC 00:00:5e:00:01:0a, keepalived marker returns BACKUP, observer neigh cache repoints to the virtual MAC after the GARP, VIP pings again |
| AC-3 | QS-3: graceful stop (SIGTERM) of Master ze | tcpdump captures ze's prio-0 advert; keepalived promotes within skew: wire delta from prio-0 advert to keepalived's first advert/GARP <= 3.0 s (distinguishing: the no-prio-0 path is >= 3.61 s), proving the Skew_Time path per RFC 9568 §6.4.2 |
| AC-4 | QS-4: keepalived prio 200 Master first; ze joins prio 100 preempt false; then ze reconfigured to prio 250 preempt false | ze never sends an advert in either phase (observation window = 3 master-down intervals per phase, zero ze-sourced adverts in capture); keepalived marker stays MASTER throughout; proves both reverse mastership and Preempt_Mode False rearm per RFC 9568 §6.4.2 |
| AC-5 | QS-5: ze group `version 2` prio 200 vs keepalived default (v2) prio 100, both advert 1 s | tcpdump shows VRRPv2 adverts from ze: version 2, authtype 0 (none), intvl 1 s, VRRP length 8 + 4*count + 8 (trailer present); keepalived stays BACKUP (accepted format); after ze SIGTERM keepalived promotes within its v2 master-down (proves it timed against ze's adverts, R-7 mitigation) |
| AC-6 | QS-6: ze IPv6 group (link-local VIP fe80-range + global VIP) prio 200 vs keepalived IPv6 instance prio 100 | ze adverts: src = ze link-local, dst ff02::12, hop limit 255, ether src 00:00:5e:00:02:0a, first listed address link-local; on promotion an unsolicited NA per VIP: ICMPv6 type 136, flags R=1 S=0 O=1, dst ff02::1, TLL option = 00:00:5e:00:02:0a; global VIP answers ping6 from the observer |
| AC-7 | QS-7: duplicate VRID, both prio 150, ze holds the numerically greater primary IP | Both may claim Master transiently; within 3 master-down intervals the IP tie-break converges: capture's final window contains adverts from exactly one source (ze), keepalived marker BACKUP, no alternation (umbrella R-7) |
| AC-8 | QS-8: ze vs ze (second ze instance in the keepalived slot), prio 200 vs 100, v3 IPv4 | Higher-priority ze elected Master within 3x advert + skew; VIP pingable; backup ze silent on the wire (maps umbrella AC-1 two-ze-box election) |
| AC-9 | Container path: `make ze-interop-test` with the three vrrp scenarios | CS-1 (v3 election), CS-2 (v2 election), CS-3 (graceful failover via docker stop/start) pass through the extended harness; every check.py follows the interop contract (converge, assert, stability recheck, raise on failure) |
| AC-10 | `make ze-qemu-needs-linux-test` | `vrrp-backup-hold.ci`, `vrrp-failover.ci`, `vrrp-preempt.ci` run and pass inside the VM; on darwin the same tests report SKIP with the needs-linux reason (record_parse.go:383-395) |
| AC-11 | Discovery surfaces | `ze-qemu-vrrp-keepalived-test` exists in mk/test-integration.mk with the tmp/kernel/vmlinuz guard, is in .PHONY, has a Makefile help row; docs/functional-tests.md documents the lab; ai/rules/qemu-testing.md interop-lab table gains the VRRP row |

## End-to-End User Stories (MANDATORY for new features)

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Runs `make ze-qemu-vrrp-keepalived-test` on macOS or Linux | make -> qemu-run.py (runtime kernel) -> VM -> evidence script -> 8 scenarios -> PASS/FAIL exit code | evidence script QS-1..QS-8 (AC-1..AC-8) |
| 2 | Runs `make ze-interop-test` on a Linux Docker host | run.py builds keepalived image -> interop.py starts ze + keepalived containers -> check.py asserts | CS-1/CS-2/CS-3 check.py (AC-9) |
| 3 | Runs `make ze-qemu-needs-linux-test` while iterating on VRRP | runner discovers needs-linux .ci -> boots ze + injector plugin in VM | `test/vrrp/vrrp-backup-hold.ci`, `vrrp-failover.ci`, `vrrp-preempt.ci` (AC-10) |
| 4 | Reads docs/functional-tests.md to find how VRRP is validated | doc row -> make target -> script | AC-11 doc assertions (grep in Pre-Commit Verification) |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| (none -- this child adds no Go feature code) | - | Go unit coverage is owned by spec-vrrp-1..5; this child's tests are the evidence script, container scenarios, and .ci files below | |

### Boundary Tests (MANDATORY for numeric inputs)

Timing acceptance bands, all measured from tcpdump wire timestamps (defaults:
advert 100 cs; keepalived prio 100: skew 61 cs, master-down 361 cs; ze prio 200:
skew 22 cs, master-down 322 cs):

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| QS-3 prio-0 -> keepalived promotion delta | 0-3.0 s (skew path) | 3.0 s | N/A (faster is fine) | 3.61 s would mean the skew path was not taken (full master-down) |
| QS-2 silence -> keepalived promotion delta (from last ze advert) | 3.0-6.0 s | 6.0 s | 3.0 s (promotion before 3x advert = peer ignored ze's adverts) | 6.0 s (master-down 3.61 s + TCG margin exceeded) |
| QS-2 ze preempt-return promotion delta (from ze restart advert-rx start) | 2.8-6.0 s | 6.0 s | 2.8 s (before its own master-down 3.22 s minus margin) | 6.0 s |
| QS-4 ze advert count during no-preempt windows | 0 | 0 | N/A | 1 (any ze advert = preempt-false violated) |
| QS-5 v2 on-wire advert interval | 1 s exact (8-bit seconds) | 1 | 0 (field cannot encode) | 2 (would be an interval mismatch; RFC 3768 receivers discard) |
| QS-5 v2 VRRP message length (1 VIP) | 20 bytes (8 + 4 + 8 trailer) | 20 | 12 (missing trailer) | 24 (spurious extra trailer, the uvrrpd v3/v4 bug class) |
| QS-1 advert interval field | 100 cs | 100 | 99 | 101 (config maps 1000 ms -> 100 cs exactly) |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| vrrp-backup-hold | `test/vrrp/vrrp-backup-hold.ci` (needs-linux) | ze group prio 100 on a config-created veth; injector plugin sends v3 adverts prio 200 (paced by select() timeouts, no time.sleep); ze holds Backup: expect production backup-state log, reject any master-transition log for the window; injector `runtime_fail`s if its adverts stop being consumed (socket errors) | |
| vrrp-failover | `test/vrrp/vrrp-failover.ci` (needs-linux) | injector sends prio-200 adverts, then goes silent; ze promotes after master-down: expect master-transition production log; injector receive-asserts ze's advert and GARP on its raw sockets via wait_until, `runtime_fail` on absence | |
| vrrp-preempt | `test/vrrp/vrrp-preempt.ci` (needs-linux) | ze prio 200 preempt false; injector master prio 100; ze never promotes: expect sustained backup log, reject master-transition pattern across 3 injector master-down windows | |

Justification for the split (isis-adjacency.ci model, `test/isis/isis-adjacency.ci:15-23`):
the .ci runner boots exactly one DUT daemon, so a two-daemon (ze + keepalived or
ze + ze) .ci is not feasible; the raw-socket injector substitutes for the peer in
single-daemon .ci form, and full two-implementation behavior is owned by the
evidence script (QS-1..QS-8) and container scenarios (CS-1..CS-3). The rest of
the `test/vrrp/` suite is owned by spec-vrrp-5: its native (darwin)
config-validate files are `vrrp-config.ci` and `vrrp-config-invalid.ci`, while
its daemon-booting tests (`vrrp-instance-up.ci`, and any others its suite list
marks so) are `option=needs-linux` like this child's rows; this child adds only
the three live injector rows above.

### Interop Tests (MANDATORY for protocol features)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| QS-1..QS-7 (netns lab) | `scripts/evidence/effective-vrrp-keepalived.py` | keepalived (Alpine pkg) | RFC 9568 v3 IPv4/IPv6 + RFC 3768 v2 election, failover, prio-0, preempt-false, tie-break, GARP/NA -- the mandatory-runnable interop path | |
| QS-8 (netns lab) | same script | ze (second instance) | umbrella AC-1 two-ze-box election baseline | |
| vrrp-v3-keepalived | `test/interop/scenarios/vrrp-v3-keepalived/` | keepalived container | v3 election + VIP reachability + virtual MAC in peer neigh table, cross-environment | |
| vrrp-v2-keepalived | `test/interop/scenarios/vrrp-v2-keepalived/` | keepalived container | v2 opt-in wire format accepted by default keepalived | |
| vrrp-failover-keepalived | `test/interop/scenarios/vrrp-failover-keepalived/` | keepalived container | graceful failover + preempt return via docker stop/start | |

### Future (if deferring any tests)
- None deferred. (FRR-vrrpd as a second independent peer is explicitly out of scope, recorded in Known Limitations, not a deferral of committed work.)

## Files to Modify
- `mk/test-integration.mk` - add `ze-qemu-vrrp-keepalived-test` (qemu-run.py `--packages "keepalived tcpdump iproute2 iputils-ping kmod python3" --run 'python3 scripts/evidence/effective-vrrp-keepalived.py' --timeout 900`); add to the .PHONY qemu block (:28); Quick-reference header row
  - **Superseded 2026-07-15 (see A-5):** no `--kernel tmp/kernel/vmlinuz` and no vmlinuz guard. The stock Alpine kernel was probed and creates macvlan/bridge/veth/netns fine, so requiring the custom kernel would only impose a ~30-minute `make ze-kernel` build. DONE: target added, in `.PHONY`, `Makefile` help-test row present, `ai/rules/qemu-testing.md` lab table row added.
- `Makefile` - help block row beside the other qemu labs (:559-563 region)
- `docs/functional-tests.md` - register the VRRP keepalived lab, the make target, and the needs-linux .ci trio (discovery-updates rule)
- `ai/rules/qemu-testing.md` - add the VRRP row to the interop-lab table (:169-172; the rule itself demands the row)
- `test/interop/run.py` - build `keepalived-interop` image from Dockerfile.keepalived (mirror the BIRD build :53-67)
- `test/interop/interop.py` - KEEPALIVED container constants; Scenario.setup() hook: `keepalived.conf` presence starts the keepalived container with NET_ADMIN (mirror bird :1316-1326); teardown row; small Keepalived probe helper (docker exec: state-marker file, `ip -j neigh`, ping) for check.py use

## Files to Create
- `scripts/evidence/effective-vrrp-keepalived.py` - the QEMU netns lab: topology setup, keepalived boot + notify markers, tcpdump LineCollector, scenarios QS-1..QS-8, diagnostics, cleanup
- `test/interop/Dockerfile.keepalived` - Alpine + `apk add keepalived`, foreground entrypoint (`keepalived -n -l -f /etc/keepalived/keepalived.conf`), config mounted by the harness
- `test/interop/scenarios/vrrp-v3-keepalived/{ze.conf,keepalived.conf,check.py}` - CS-1
- `test/interop/scenarios/vrrp-v2-keepalived/{ze.conf,keepalived.conf,check.py}` - CS-2
- `test/interop/scenarios/vrrp-failover-keepalived/{ze.conf,keepalived.conf,check.py}` - CS-3
- `test/vrrp/vrrp-backup-hold.ci`, `test/vrrp/vrrp-failover.ci`, `test/vrrp/vrrp-preempt.ci` - needs-linux injector tests

### Scenario reference (QEMU netns lab)

Topology (PID-suffixed names per the blueprint; three leaf namespaces plus a
bridge namespace so the observer sees flooded multicast/broadcast):

```
  [ze netns]          [peer netns]         [observer netns]
    ze0 ---.            ka0 ---.             ob0 ---.
           |                   |                    |
        (veth)              (veth)               (veth)
           |                   |                    |
    [lan netns:  br0  <- all three peer ends enslaved, no snooping filter]
```

| # | Name | ze config (shape per umbrella; exact leaves per spec-vrrp-5) | keepalived config | Wire/state assertions (summary; details in AC table) |
|---|------|-------------------------------------------------------------|-------------------|------------------------------------------------------|
| QS-1 | v3 IPv4 election | vrid 10, prio 200, virtual-address 192.0.2.1, advertise-interval-milliseconds 1000, accept-mode true | v3, prio 100, same VIP | AC-1 |
| QS-2 | node-death failover + preempt return | as QS-1 | as QS-1 | AC-2 |
| QS-3 | graceful stop (prio-0) | as QS-1 | as QS-1 | AC-3 |
| QS-4 | reverse mastership + preempt false | prio 100 then 250, preempt false | prio 200 | AC-4 |
| QS-5 | v2 opt-in | version 2, prio 200, advert 1000 ms (= 1 s on wire) | default v2, prio 100 | AC-5 |
| QS-6 | IPv6 v3 | ipv6 group: link-local VIP + global VIP, prio 200, accept-mode true | IPv6 instance, prio 100 | AC-6 |
| QS-7 | duplicate-VRID tie-break | prio 150, greater primary IP | prio 150, lesser primary IP | AC-7 |
| QS-8 | ze vs ze | prio 200 | (second ze instance, prio 100) | AC-8 |

Example keepalived peer config, QS-1 (plain text; notify markers are the
machine-readable state source; the file is validated with `keepalived -t`):

```
global_defs {
    vrrp_version 3
}
vrrp_instance lab {
    state BACKUP
    interface ka0
    virtual_router_id 10
    priority 100
    advert_int 1
    virtual_ipaddress {
        192.0.2.1/24
    }
    notify_master "/bin/sh -c 'echo MASTER >> /work/ka-state.log'"
    notify_backup "/bin/sh -c 'echo BACKUP >> /work/ka-state.log'"
    notify_fault  "/bin/sh -c 'echo FAULT >> /work/ka-state.log'"
}
```

QS-5 variant: drop `vrrp_version 3` (keepalived defaults to v2 for IPv4).
QS-6 variant: `virtual_ipaddress { fe80::1 2001:db8::1/64 }` in an IPv6
instance (keepalived uses v3 for IPv6 unconditionally).

Example ze config shape, QS-1 (umbrella-agreed surface; exact type/leaf names
follow spec-vrrp-5's landed YANG):

```
interface {
    ethernet ze0 {
        unit 0 {
            ipv4 {
                address 192.0.2.251/24;
                vrrp {
                    group 10 {
                        virtual-address 192.0.2.1;
                        priority 200;
                        advertise-interval-milliseconds 1000;
                    }
                }
            }
        }
    }
}
```

### Integration Checklist
| Integration Point | Needed? | File |
|-------------------|---------|------|
| YANG schema (new RPCs/config) | N/A | vrrp YANG is spec-vrrp-5's; this child only consumes it |
| YANG validation constraints | N/A | spec-vrrp-5 |
| YANG custom validators | N/A | spec-vrrp-5 |
| CLI commands/flags | N/A | no CLI changes |
| CLI grammar (action before identifier) | N/A | no CLI changes |
| Editor autocomplete | N/A | no config surface added |
| Functional test for new RPC/API | Yes | the three needs-linux .ci tests + evidence script + container scenarios ARE the tests this child ships |
| Pipe completeness | N/A | no command output added |
| Env var registration | N/A | no environment leaves; scenario parameters are script-internal constants (evidence scripts may honor ZE_EVIDENCE_ZE_BINARY like the blueprint :233) |
| Doctor check for runtime dependencies | N/A | doctor-vrrp-* checks are children 3/4/5; test infra needs none |
| Prometheus counters/metrics | N/A | no observable Go state added |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | validation infra only; the feature rows are owned by spec-vrrp-5 (verify by grep of docs/features.md for vrrp after child 5 lands) |
| 2 | Config syntax changed? | No | consumes child-5 syntax |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | No | none |
| 6 | Has a user guide page? | No | docs/guide/vrrp.md is child 5's; interop evidence may be cited there by child 5 |
| 7 | Wire format changed? | No | VRRP wire formats are the RFCs'; nothing ze-internal changes |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | Yes | `docs/features/rfc-status.md`: append interop-proven evidence (keepalived scenarios + lab target) to the RFC 9568 and RFC 3768 rows child 5 creates, with source anchors to the script and scenarios |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md`: VRRP keepalived lab section (make target, script, scenarios, .ci trio); `ai/rules/qemu-testing.md` interop-lab table row |
| 11 | Affects daemon comparison? | No | comparison row is child 5's |
| 12 | Internal architecture changed? | No | no architecture change; grep docs/ for source anchors on interop.py/run.py before commit |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin/event/command inventory changed? | No | none |
| 16 | Changed source files referenced by doc source anchors? | Yes | grep `docs/` for `source:` anchors on `test/interop/interop.py`, `test/interop/run.py`, `mk/test-integration.mk`; update any stale claims |
| 17 | Existing docs show config/CLI/API examples for this area? | No | examples land with child 5 |

## Implementation Steps

### /implement Stage Mapping
| /implement Stage | Spec Section |
|------------------|--------------|
| 1. Read spec | This file + spec-vrrp-0-umbrella.md + landed spec-vrrp-5 deliverables |
| 2. Audit | Files to Modify/Create + TDD Test Plan; validate A-1..A-8 cheaply (grep child-5 YANG for accept-mode, check Alpine keepalived availability) |
| 3. Wiring phase | Wiring Test table -- create the make target + script skeleton + failing scenario stubs |
| 4. Implement (TDD) | Implementation Phases below |
| 5. Full verification | `make ze-lint && make ze-unit-test && make ze-functional-test` + `make ze-qemu-vrrp-keepalived-test` + `make ze-qemu-needs-linux-test` (+ `make ze-interop-test` on a Linux Docker host) |
| 6. Critical review | Critical Review Checklist below |
| 7-9. Fix/re-verify loop | Until clean |
| 10. Deliverables review | Deliverables Checklist below |
| 11. Security review | Security Review Checklist below |
| 12. Documentation review | Documentation Update Checklist above |
| 13. /ze-review gate | Review Gate section; loop to 0 BLOCKER / 0 ISSUE |
| 14. Present summary + close | Two-commit closure per `ai/rules/planning.md` |

### Implementation Phases

Each phase ends with a **Self-Critical Review**. Fix issues before proceeding.

1. **Phase: Wiring (MANDATORY FIRST)** -- make target + script skeleton reachable end to end
   - Tests: `make ze-qemu-vrrp-keepalived-test` reaches the script inside the VM and fails on the unimplemented-scenario stub (the failing wiring test)
   - Files: `mk/test-integration.mk` (+.PHONY + Quick-reference), `Makefile` help row, `scripts/evidence/effective-vrrp-keepalived.py` (topology setup + kernel probe + keepalived -t gate + scenario runner scaffold)
   - Verify: VM boots the runtime kernel, keepalived installs from Alpine, macvlan/bridge/veth probes pass (validates A-1, A-5)
2. **Phase: QS-1 election + capture infrastructure** -- LineCollector-over-tcpdump, notify markers, neigh/ping probes
   - Tests: QS-1 assertions (AC-1); this phase settles A-2 (printer fields), A-4 (notify markers), A-7 (accept-mode/ping)
   - Files: evidence script
   - Verify: QS-1 PASS marker; every later scenario reuses this machinery
3. **Phase: QS-2 + QS-3 failover timing** -- node death, preempt return, prio-0 skew path
   - Tests: AC-2, AC-3 with the Boundary Tests timing bands
   - Files: evidence script
   - Verify: wire-timestamp deltas within bands on 3 consecutive runs (R-2 confidence)
4. **Phase: QS-4 + QS-5** -- no-preempt/reverse mastership + v2 opt-in wire format
   - Tests: AC-4, AC-5 (including the v2 promote-after-stop positive check, R-7 mitigation)
   - Files: evidence script
5. **Phase: QS-6 + QS-7 + QS-8** -- IPv6 NA, duplicate-VRID tie-break, ze-vs-ze
   - Tests: AC-6, AC-7, AC-8
   - Files: evidence script
6. **Phase: needs-linux .ci trio** -- raw-socket injector plugin pattern
   - Tests: `vrrp-backup-hold.ci`, `vrrp-failover.ci`, `vrrp-preempt.ci` under `make ze-qemu-needs-linux-test`; SKIP-with-reason verified natively on darwin (AC-10; validates A-6, A-8)
   - Files: the three .ci files in `test/vrrp/`
   - Verify: zero `time.sleep(` in the new files; `test/.ci-sleep-baseline` untouched
7. **Phase: container harness + CS-1..CS-3** -- Dockerfile.keepalived, run.py build, interop.py hook, scenarios
   - Tests: `make ze-interop-test INTEROP_SCENARIO=vrrp-v3-keepalived` (then v2, failover) on a Linux Docker host (AC-9; validates A-3)
   - Files: `test/interop/Dockerfile.keepalived`, `test/interop/run.py`, `test/interop/interop.py`, three scenario directories
8. **Functional tests** -- covered by phases 2-7 (this child's deliverables ARE the functional tests)
9. **RFC refs** -- evidence script and check.py comments cite the RFC section behind each wire assertion (see RFC Documentation)
10. **Full verification** -- `make ze-verify` (native surface) + the three integration targets above
11. **Complete spec** -- audit tables, Goal Validation evidence, learned summary `plan/learned/NNN-vrrp-6-interop.md`, two-commit closure

### Critical Review Checklist (/implement stage 6)
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | Every AC-1..AC-11 demonstrated with named scenario/test + captured evidence |
| Feature completeness | Every End-to-End User Story path runs; both interop paths (QEMU + container) work; umbrella AC-1..AC-4 + R-7 all mapped in Goal Validation |
| Correctness | Assertions encode RFC 9568/3768 semantics (skew vs master-down distinction, GARP sip==tip, NA flags 1/0/1 + TLL, v2 trailer length, tie-break by greater IP), NOT reference-implementation behavior (umbrella R-1) |
| Naming | Scenario dirs `vrrp-*-keepalived` (isis-p2p-frr convention); target `ze-qemu-vrrp-keepalived-test` (qemu-lab convention); script `effective-vrrp-keepalived.py` |
| Data flow | Assertions observe wire/kernel/notify state only; no test reaches into ze internals or private files |
| CLI grammar | N/A -- no CLI added |
| Registration over hardcoding | .ci tests discovered by the runner suite scan (needs-linux contract, no per-test wiring); container scenarios discovered by the scenarios/-dir scan; keepalived container launches via the config-file-presence hook pattern; no vrrp spelling added to central Go packages |
| Doctor checks | N/A -- no runtime dependency added to ze |
| YANG validation | N/A -- no YANG added |
| Prometheus counters | N/A -- none added |
| Rule: qemu-testing | All four interop-lab steps shipped in this change, including the rule's own lab-table row |
| Rule: testing (sleep ratchet) | Zero `time.sleep(` in new .ci files; evidence-script waits are deadline-bounded predicate polls; timing asserted from wire timestamps |
| Rule: interop-and-goal-validation | Every check.py follows the 5-point contract; Goal Validation table filled with concrete evidence |

### Deliverables Checklist (/implement stage 10)
| Deliverable | Verification method |
|-------------|---------------------|
| Evidence script with 8 scenarios | `python3 scripts/evidence/effective-vrrp-keepalived.py --list` (or grep for QS-1..QS-8 markers); full run via the make target |
| `ze-qemu-vrrp-keepalived-test` wired | `make -n ze-qemu-vrrp-keepalived-test` shows the qemu-run.py invocation; grep .PHONY line in mk/test-integration.mk; grep Makefile help |
| keepalived harness hook | grep `keepalived` in test/interop/interop.py + run.py; `ls test/interop/Dockerfile.keepalived` |
| Three container scenarios | `ls test/interop/scenarios/vrrp-*-keepalived/` shows ze.conf + keepalived.conf + check.py each |
| Three needs-linux .ci tests | `ls test/vrrp/vrrp-{backup-hold,failover,preempt}.ci`; grep `option=needs-linux` in each; native run reports SKIP |
| Sleep ratchet held | `grep -c 'time.sleep(' test/vrrp/vrrp-*.ci` returns 0; baseline file unchanged |
| Docs registered | grep vrrp in docs/functional-tests.md; grep VRRP row in ai/rules/qemu-testing.md lab table |
| Goal Validation filled | spec section has evidence per umbrella AC-1..AC-4 + R-7 |

### Security Review Checklist (/implement stage 11)
| Check | What to look for |
|-------|-----------------|
| Input validation | keepalived configs validated with `keepalived -t` before boot; injector fixture bytes are constants with correct checksums (a malformed fixture would test nothing) |
| Host isolation | All netns/veth/bridge names PID-suffixed; cleanup-first setup + finally-cleanup; script refuses to run without CAP_NET_ADMIN and outside Linux (blueprint :193-221); nothing touches the host default netns except namespace creation |
| Privilege scope | Raw sockets and SIGKILL/SIGSTOP confined to lab namespaces/processes the script created; container caps limited to NET_ADMIN (matching existing peers) |
| Secrets | No authentication material anywhere (v2 auth type 0 only, per umbrella decision); keepalived configs contain no passwords |
| Resource cleanup | Orphaned keepalived/tcpdump/ze processes killed via netns pid sweep (blueprint :111-129); work dir removed on success, kept for diagnosis on failure |
| Error leakage | Failure diagnostics dump lab state (netns links, neigh tables, capture tail, keepalived marker file), never host-wide state |

### Failure Routing
| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails wrong reason | Fix test assertion or setup |
| Test fails behavior mismatch | Re-read rfc/short section -> if ze is wrong, file the defect against the owning child (1-5); this child never patches ze behavior silently |
| Lint failure | Fix inline; if architectural -> DESIGN phase |
| Scenario flaky under TCG | Widen only the margin side of the Boundary band with evidence; never weaken the distinguishing bound; payload-predicate waits, never sleeps |
| keepalived config rejected | R-3 route: pin against `keepalived -t` output, adjust keywords for the Alpine version, record in Mistake Log |
| 3 fix attempts fail | STOP. Report all 3 approaches. Ask user. |

## Mistake Log

### Wrong Assumptions
| What was assumed | What was true | How discovered | Impact |
|------------------|---------------|----------------|--------|
| That VRRP worked end-to-end and this lab would prove interop | **VRRP is completely non-functional on a real system.** Every instance fails to create with `vrrp: instance create failed ... open transport for zv4-<ifindex>-<vrid>: vrrp/transport: resolve macvlan zv4-<ifindex>-<vrid>: route ip+net: no such network interface`. No advert is ever sent. | First run of this lab, 2026-07-15. All three scenarios (QS-1/2/3) failed identically before any interop assertion was reached. | **BLOCKER (FIXED).** Root cause: `engine.create` (`internal/plugins/vrrp/engine.go:116-126`) calls `createMacvlan` and then `openInstance` on the next line, treating creation as synchronous. It is not: `createMacvlan` is `iface.RegisterOwnedMacvlan` (`internal/plugins/vrrp/register.go:289-295`), which only records desired state and pokes a coalescing channel (`internal/component/iface/device_owner.go:59-89`, `trigger()` -> `registryReconcileCh` at `internal/component/iface/register.go:353`); the device is created later by `reconcileOwnedDevices` (`internal/component/iface/config_apply.go:1006`). `openInstance` resolves the macvlan BY NAME and always loses that race. Why no test caught it: `engine_test.go:41` stubs `createMacvlan`, and the transport integration tests create their own macvlan, so `engine.create` was never exercised against the real registry plus the real transport. **Fix:** the live `createMacvlan` wrapper now calls `waitDevicePresent` (register.go) after registering, blocking on `net.InterfaceByName` until the reconcile pass creates the device (bounded, `macvlanCreateTimeout`); unit-tested in `register_test.go`. Confirmed on the wire: after the fix ze reaches `state change from=backup to=master` in the lab. |
| That transmitting the RFC 9568 message-only IPv4 checksum was correct (a prior session made TX "strict-9568") | **keepalived and the RFC 5798 deployed base require the IPv4 pseudo-header checksum form; ze's message-only adverts were rejected as "Invalid VRRPv3 checksum".** | Second lab run, 2026-07-15: keepalived logged `(lab) Invalid VRRPv3 checksum` for every ze advert. Confirmed by capturing keepalived's OWN advert (checksum 0xa102) and computing both forms offline: pseudo-header 0xa102 MATCHED, message-only 0x448e did not. So keepalived computes AND requires the pseudo-header form. RFC 9568 Section 5.2.8 (rfc/full/rfc9568.txt:880-887) + change note 4 confirm message-only is the newest-RFC behavior, but the deployed base predates it. | **BLOCKER (FIXED).** ze's checksum math was correct (both golden forms exist and are hand-verified); the defect was the TX policy. Fix: `FillChecksum` (`internal/plugins/vrrp/packet/checksum.go`) now transmits the RFC 5798 pseudo-header form for v3/IPv4. RX still dual-accepts both, with the primary/fallback ORDER flipped so the pseudo-header form is canonical (unflagged) and message-only is the counted `checksum-rfc9568-message-only` fallback (was `checksum-rfc5798-compat`). Field `LegacyChecksum` -> `MsgOnlyChecksum`. This is a deliberate divergence from RFC 9568's clarification, documented in code and `docs/features/rfc-status.md`; revisit when strict-RFC-9568 receivers dominate. |

| That putting the VIP on the virtual-MAC macvlan was enough for L2 ownership of the VIP | **The macvlan does not serve the VIP's ARP at all.** Only the parent answers ARP for the VIP, and it uses its own real MAC (`arp_ignore=0` default). Silencing the parent (`arp_ignore=1`) does NOT make the macvlan answer -- an isolated netns probe (2026-07-15) showed the macvlan never even receives the broadcast ARP request, so with the parent quiet the VIP becomes 100% unreachable. | Fifth lab run: QS-1 observer resolved 192.0.2.1 to the parent MAC `66:2e:...`, not `00:00:5e:00:01:0a`. Sixth run (after adding parent `arp_ignore=1`): observer got NO ARP reply and could not ping the VIP. Minimal netns repro confirmed: macvlan bridge-mode + `/32` VIP + parent same-subnet IP -> only the parent answers; parent `arp_ignore=1` -> nothing answers. | **RESOLVED 2026-07-15 (was OPEN).** Original diagnosis (kept for history): adverts, election, failover timing, GARP were all correct, but the VIP was not L2-owned by the virtual MAC; the `arp_ignore`-alone tuning was tried and REVERTED because alone it turns a reachable-but-wrong-MAC VIP into an unreachable one. **Resolution:** the recipe was reverse-engineered from keepalived's `use_vmac` and proven byte-identical to its live kernel state (exhaustive sysctl+route diff), with a SIGSTOP freeze test proving the KERNEL (not keepalived userspace) answers -- so a pure-kernel recipe exists. It has FOUR parts, each proven necessary by isolating it: (1) macvlan **private** mode, not bridge (`register.go` `iface.MacvlanModePrivate`); (2) install the VIP with the **parent subnet prefix**, not `/32` (`register.go` `vipCIDRs`), so the macvlan owns the subnet's connected route; (3) parent sysctls `arp_ignore=1 arp_filter=1 rp_filter=1` and macvlan sysctls `arp_ignore=1 rp_filter=0 disable_ipv6=1`; (4) global `all.rp_filter=0` (effective rp_filter is max(all,iface), so the macvlan cannot reach 0 unless all is 0). Applied at macvlan create and restored on teardown, refcounted per-parent and globally (`internal/plugins/vrrp/dataplane_linux.go`). The `arp_ignore`-alone attempt failed for lack of arp_filter, all.rp_filter=0, disable_ipv6, private mode, and the subnet prefix. Cold-start note: the FIRST resolution after a neighbour flush can lose an ARP-flux race and cache the parent's real MAC once, then every resolution after is the virtual MAC; keepalived's `use_vmac` shows the IDENTICAL first-resolution race (proven in the same QEMU), so the interop assertion flushes+re-resolves in its poll. **Proven end-to-end:** interop lab QS-1/QS-2/QS-3 all PASS against keepalived 2.3.1 -- the observer resolves 192.0.2.1 to `00:00:5e:00:01:0a`, and the VIP re-points to the virtual MAC after failover. `docs/guide/vrrp.md` updated from "known limitation" to the working description. |

| That the dataplane recipe as first landed was complete and correct | **A critical self-review found six real issues, one confirmed on the wire.** | Critical review 2026-07-15 after the RESOLVED entry above; the owner case was reproduced in QEMU (`tmp/mv-owner.sh`). | **ALL RESOLVED 2026-07-15.** (1) **Address-owner regression (confirmed in QEMU):** the owner's VIP equals a parent real address, so installing it on the macvlan at the subnet prefix added a duplicate connected route and both parent and macvlan answered ARP; QEMU showed the observer resolving 5/5 to the parent's real MAC. Fix: `vipMaskBits` (`register.go`) installs a VIP equal to a real address as a host route (`/32`), verified no-duplicate-route in QEMU. (2) **`vipMaskBits` fallback bug:** a VIP outside every parent subnet was installed with a non-containing parent prefix's length (bogus connected route); now falls back to a host route. (3) **`disable_ipv6` was cargo-culted** from keepalived -- QEMU proved the IPv4 recipe reaches the virtual MAC without it (bridge topology, 5/5 after cold start), so it was removed. (4) **IPv6 L2 ownership was untested** -- now validated in QEMU (bridge topology, 6/6 to the virtual MAC with zero sysctls, no cold race, because ND uses solicited-node multicast so the parent never competes); the no-op IPv6 path is correct. (5) **iface/sysctl coordination:** iface re-emits the parent's ARP sysctls from unit config on every apply, which could silently clobber VRRP's; the engine now re-asserts the recipe for every running instance on each apply (`reassertDataplaneSysctls`, no refcount change). (6) **Macvlan mode drift** was undetectable (`InterfaceInfo` carried no mode); added `MacvlanMode` readback (`show_linux.go`) and a drift comparison in `ownedMacvlanMatchesSpec`, so a bridge-mode device is re-created as private. Cold-start race, `all.rp_filter=0` host-global side effect (not restored on SIGKILL), and the owner's real-MAC behavior are documented in `docs/guide/vrrp.md`. Unit tests added for owner/fallback masks, mode drift, and per-apply re-assert. |

### Failed Approaches
| Approach | Why abandoned | Replacement |
|----------|---------------|-------------|
| Lab QS-2 phase B checked for the restarted ze's advert with a one-shot read right after `wait_ze_state("master")` | `wait_ze_state` returns on the LOG marker; the first advert reaches the tcpdump LineCollector a moment later, so the one-shot read raced the gap and reported "no advert" though ze had sent one (visible in the capture at the master-transition timestamp). | Poll with `wait_until` for an advert with `ts > restart` (bounded by `WIRE_EVENT_TIMEOUT_S`) before reading the list. This was a LAB race, not a ze bug: ze's restart-then-preempt path advertised correctly. |

### Escalation Candidates
| Mistake | Frequency | Proposed rule | Action |
|---------|-----------|---------------|--------|

## Design Insights
<!-- LIVE -- write IMMEDIATELY when you learn something -->
- A dead process does not release its kernel network state: after SIGKILL the ze netns still answers ARP for installed VIPs, so "node death" in a netns lab must include carrier loss (link down) to be a faithful failover trigger (R-4).
- The v2 interop scenario needs a positive keepalived reaction (promotion after ze stops) because "peer stays backup" is indistinguishable from "peer silently discards our malformed packets" (R-7).

## Core Insight

Interop assertions must be phrased as wire evidence (tcpdump fields and
timestamps) plus peer-visible reactions (notify markers, promotions), never as
ze-internal state: that is what makes the same scenario meaningful against any
peer implementation, and what makes a pass mean "keepalived agreed", not "ze
agreed with itself".

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Extend the container harness with a keepalived peer (Dockerfile.keepalived + config-file hook) AND ship the QEMU netns lab | (b) defer containers, QEMU-only | Umbrella commits child 6 to both paths (Interop Tests table + Files to Create); the harness extension is mechanical -- one more launch hook mirroring bird.conf (interop.py:1316-1326) and one image build (run.py:53-67); the QEMU lab remains the mandatory-runnable path per qemu-testing.md:137-145 |
| QEMU lab uses `--kernel tmp/kernel/vmlinuz` | stock Alpine ISO kernel + modprobe | CONFIG_MACVLAN=y/BRIDGE=y/VETH=y are already in gokrazy/kernel/runtime.config:45-47 (built in, deterministic); the stock ISO kernel's module set is unverified across Alpine releases; l2tp/pppoe labs set the precedent (mk/test-integration.mk:409-430) |
| keepalived as the peer daemon | FRR vrrpd (daemons:23 is vrrpd=no) | keepalived is the umbrella-named dominant implementation and the v2-default interop case; FRR vrrpd noted as fallback only (A-1) |
| Three-leaf-plus-bridge topology with a dedicated observer netns | two netns on a bare veth | Observer sees flooded multicast/broadcast on the bridge for capture AND is an independent third party for ARP/ND-cache and VIP-ping assertions -- a veth-only pair has no neutral vantage point |
| keepalived state via notify script markers | parsing keepalived logs; SIGUSR1 data dumps | Markers are a documented, version-stable contract and a clean payload-predicate wait source; log formats drift (R-3) |
| Node death = SIGKILL + link down; graceful stop = SIGTERM | SIGSTOP/SIGCONT | SIGSTOP leaves the kernel answering ARP for the VIP (unfaithful, R-4) and resumes as Master without a promotion (no GARP burst to assert); kill+restart exercises Backup -> preempt -> promotion -> GARP, the path AC-2 must prove |
| Single-daemon .ci with raw-socket injector; no two-daemon .ci | two-daemon .ci topology | The runner boots exactly one DUT; the injector reproduces peer stimulus deterministically; two-implementation truth lives in the evidence script (isis-adjacency.ci split model) |
| Added QS-8 (ze vs ze) beyond the task's 7 keepalived scenarios | keepalived-only | Umbrella AC-1 is a two-ze-box election; the lab already has everything needed (second ze process in the peer slot), and Goal Validation cannot map AC-1 honestly without it |
| Injector paces sends with select() socket timeouts | time.sleep between sends | Keeps the .ci sleep-ratchet count at zero and doubles as the receive wait |

## Known Limitations
- keepalived is the only external implementation tested; FRR vrrpd / BIRD (no VRRP) are out of scope (fallback path documented in A-1).
- Container scenarios run only on Linux hosts with Docker (isis-p2p-frr precedent); macOS coverage is via the QEMU lab.
- No keepalived `use_vmac` (peer-side virtual-MAC) pairing: keepalived runs in its default real-MAC mode; ze's virtual-MAC behavior is asserted from ze's side of the wire.
- No unicast-peer, sync-group, or tracking interop (out of umbrella scope).
- Timing bands are tuned for QEMU (HVF/TCG) margins; bare-metal runs will pass with wide headroom but the bands are not tightness proofs.

## RFC Documentation

This child adds no Go code, so `// RFC ...` code comments do not apply. Instead:
every wire assertion in `effective-vrrp-keepalived.py`, the check.py files, and
the .ci injector fixtures carries a comment citing the RFC section it enforces
(RFC 9568 §5.1.x TTL/hop-limit, §5.2.x field encodings, §6.4.x transitions and
GARP/NA content incl. errata 7947/7949, §7.1 receive checks, §8.4.2 v2 interop;
RFC 3768 §5.3.x v2 fields, §7.1 interval-mismatch discard). The fixture bytes in
the .ci injector document their field layout byte by byte.

## Implementation Summary

### What Was Implemented
- (fill during implementation)

### Bugs Found/Fixed
- (fill during implementation)

### Documentation Updates
- (fill during implementation)

### Deviations from Plan
- (fill during implementation)

## Implementation Audit

### Requirements from Task
| Requirement | Status | Location | Notes |
|-------------|--------|----------|-------|

### Acceptance Criteria
| AC ID | Status | Demonstrated By | Notes |
|-------|--------|-----------------|-------|

### Tests from TDD Plan
| Test | Status | Location | Notes |
|------|--------|----------|-------|

### Files from Plan
| File | Status | Notes |
|------|--------|-------|

### Audit Summary
- **Total items:**
- **Done:**
- **Partial:** (all require user approval)
- **Skipped:** (all require user approval)
- **Changed:** (documented in Deviations)

## Goal Validation (BLOCKING)
| Goal (from Task / umbrella) | Evidence Type | Concrete Evidence |
|--------------------------|---------------|-------------------|
| Umbrella AC-1: two ze boxes, VRID election, VIP pingable, virtual MAC on wire | QEMU interop scenario | QS-8 (ze vs ze) + QS-1 wire MAC/ARP assertions (fill: run output) |
| Umbrella AC-2: master stopped, backup promotes within skew, GARP, VIP reachable | QEMU interop scenario | QS-3 (prio-0 skew path) + QS-2 (node death + GARP + neigh repoint) (fill: measured deltas) |
| Umbrella AC-3: ze vs keepalived, v2 and v3, both directions, preempt on/off | QEMU + container interop | QS-1/QS-5 (ze master), QS-4 (keepalived master, preempt off), QS-2 (preempt on return); CS-1/CS-2/CS-3 (fill: pass output) |
| Umbrella AC-4: IPv6 link-local VIP, NA on promotion, VIP reachable | QEMU interop scenario | QS-6 (NA type 136 flags 1/0/1 + TLL, ping6) (fill: capture lines) |
| Umbrella R-7: duplicate VRID converges by IP tie-break | QEMU interop scenario | QS-7 single-master final window (fill: capture summary) |
| Live kernel behavior of the ze FSM entry points | needs-linux functional tests | vrrp-backup-hold.ci / vrrp-failover.ci / vrrp-preempt.ci under ze-qemu-needs-linux-test (fill: run output) |

## Review Gate

### Run 1 (initial)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Fixes applied
- (fill during /ze-review)

### Run 2+ (re-runs until clean)
| # | Severity | Finding | Location | Action |
|---|----------|---------|----------|--------|

### Final status
- [ ] `/ze-review` re-run shows 0 BLOCKER, 0 ISSUE
- [ ] All NOTEs recorded above (or explicitly "none")

## Pre-Commit Verification

### Files Exist (ls)
| File | Exists | Evidence |
|------|--------|----------|

### AC Verified (grep/test)
| AC ID | Claim | Fresh Evidence |
|-------|-------|----------------|

### Wiring Verified (end-to-end)
| Entry Point | .ci File | Verified |
|-------------|----------|----------|

### Assumptions Resolved
| ID | Final Status | Evidence |
|----|--------------|----------|

### Documentation Verified
| Documentation claim or category | Source evidence | Verified |
|---------------------------------|-----------------|----------|

## Checklist

### Goal Gates (MUST pass)
- [ ] AC-1..AC-11 all demonstrated
- [ ] End-to-End User Stories: every story has a working path and a passing test
- [ ] Wiring Test table complete -- every row has a concrete test name, none deferred
- [ ] `/ze-review` gate clean (Review Gate section filled -- 0 BLOCKER, 0 ISSUE)
- [ ] `make ze-test` passes (lint + all ze tests)
- [ ] Feature code integrated (make targets, harness extension, evidence script)
- [ ] Integration completeness proven end-to-end (all three entry points run)
- [ ] Documentation Update Checklist answered Yes/No with source evidence
- [ ] Architecture docs and guides updated where changed behavior is documented
- [ ] Critical Review passes (all 6 checks in `ai/rules/quality.md` -- no failures)
- [ ] Risks & Assumptions: every A-N confirmed or broken (none `unvalidated`); broken ones in Mistake Log; surviving risks copied to Executive Summary

### Quality Gates (SHOULD pass -- defer with user approval)
- [ ] RFC constraint comments added (script/scenario/fixture comments per RFC Documentation)
- [ ] Implementation Audit complete
- [ ] Mistake Log escalation reviewed

### Design
- [ ] Abstract when you can (2+ use cases?)
- [ ] No speculative features (needed NOW?)
- [ ] Single responsibility per component
- [ ] Explicit > implicit behavior
- [ ] Minimal coupling

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (timing bands table)
- [ ] Functional tests for end-to-end behavior
- [ ] Interop tests for protocol features (this child IS the interop suite)
- [ ] Goal Validation table filled with concrete evidence

### Completion (BLOCKING -- before ANY commit)
- [ ] Critical Review passes -- all 6 checks in `ai/rules/quality.md` documented pass in spec. A single failure = work is not complete.
- [ ] Partial/Skipped items have user approval
- [ ] Implementation Summary filled
- [ ] Implementation Audit filled (every requirement, AC, test, file has status + location)
- [ ] Write learned summary to `plan/learned/NNN-<name>.md`
- [ ] **Commit A:** code + tests + docs + spec (with all edits) + learned summary + counter bump
- [ ] **Commit B:** `git rm plan/<spec>` only (preserves edited spec in git history from commit A)
