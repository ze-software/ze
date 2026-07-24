# 1267 -- netns-uid-drop-and-ddos-victim

## Context

Continuing the netlink-suite recovery (`plan/handover/21-netlink-suite-recovery.md`),
two clusters were labeled "needs a full Linux host". That framing was wrong: the
QEMU Alpine VM IS a full Linux host (root, netns, nftables, conntrack, eBPF). Each
"environmental" blocker was a real, fixable defect or a QEMU-setup mistake. This
pass resolved the ddos classifier test and unblocked the OSPF interface-missing
cluster (ospf-nbma green), all validated in QEMU.

## Decisions

- **ze_api must be reachable by a uid-dropped observer.** In netns launch mode ze
  runs as a normal user and forks observer plugins as that user; they import
  `ze_api` via PYTHONPATH=`<repo>/test/scripts`, but a 0700 repo root blocks the
  uid from traversing it. Fix: `copyTestScripts` copies `test/scripts/*.py` into
  the tmpfs workdir, which is the observer script's OWN directory (Python puts it
  on `sys.path[0]`) and is already chowned to the child. Chosen over widening repo
  perms (host-global) or rewriting PYTHONPATH.
- **The iface backend must load for config-less consumers.** OSPF reads an
  interface's IPv4 through the iface backend, but that backend loads only from an
  `interface {}` config block. `iface.EnsureBackend` loads the build-time default
  when none is loaded (strict no-op when an explicit backend already did, so
  `interface { backend vpp }` still wins), called from `resolveOSPFInterface`.
  Confined to OSPF's call site rather than loading a backend for every daemon.
- **The ddos loopback test had a reverse-ICMP victim-selection artifact.** Flooding
  the closed 127.0.0.2:9999 makes the kernel emit ICMP-unreachables to 127.0.0.1;
  those egress toward 127.0.0.1 and, embedding the quoted datagram, are larger per
  packet than the forward flood, so `parseTopDestination`'s by-destination-bytes
  pick chose 127.0.0.1 (the source). Fix: bind a listener on 127.0.0.2:9999 so no
  reverse ICMP is generated -- a loopback-only confound a real transit box lacks.
- **An observer must read the field names the code actually emits.** The OSPF
  `Snapshot` struct is de-facto underscore (`network_type`, `hello_interval`) but
  its NBMA fields are kebab (`poll-interval`, `nbma-neighbors`) -- and the
  json-kebab hook forbids making them underscore. So the observer was fixed to
  read the kebab names, not the struct changed.
- **A repeated leaf-list statement must accumulate, not overwrite.** The
  ospf-instance-demux failure ("each instance must carry eth0: instance 0 has 0
  interfaces") looked like an OSPF multi-instance bug but was a CORE config-parser
  bug: `instance-id 0; instance-id 5;` on one interface silently kept only 5. The
  brace parser's two leaf-list stores (`storeValueOrArray`, `parseBracketLeafList`)
  called `Tree.SetSlice`, which REPLACES the whole leaf-list, so the second `name
  value;` statement dropped the first. YANG models repeated leaf-list statements as
  additive (RFC 7950 sec 7.7). Fix: a new `Tree.AppendSlice` (append, dedup as a
  set, preserve deactivation markers) used by both stores; the scalar mirror is
  rebuilt from the full accumulated list. A single statement or a single `[ a b ]`
  bracket on an empty leaf-list is unchanged. Chosen over rewriting the test config
  to the bracket form: the silent data loss is a real fail-closed defect any
  operator following YANG conventions would hit, and the test-weakening hook
  correctly refused the config edit. The OSPF config parse was already correct
  (`configInstanceIDs` handles `["0","5"]`); the loss was strictly upstream in the
  brace walker, so every leaf-list benefits.

## Consequences

- `make ze-netns-qemu-test` gains an OSPF subset (`OSPF_IDS=["50"]`) gating all
  three infra fixes; more interface-missing ospf tests join as they green.
- The infra fixes are general product improvements: an OSPF-on-OS-configured-
  interface daemon now works (EnsureBackend), and any `ze_api`-using observer runs
  under uid-drop netns (copyTestScripts).
- Remaining interface-missing ospf tests (58/68/29/45-48/vlink 14) are mechanical
  now: `needs-linux:caps=net-admin` + per-interface `netns-link` + per-test
  observer kebab/underscore fixes.

## Gotchas

- "Can't be done in QEMU" was three separate self-inflicted mistakes: wrong
  `ze-test` invocation (`plugin` vs `bgp plugin`), a missing package (`nftables`),
  and a bespoke netns setcap on a non-xattr path. Verify the harness before
  blaming the environment.
- A 0700 repo root (a personal umask) silently breaks uid-dropped test children;
  the fix belongs in the harness, not in file permissions.
- The OSPF `Snapshot` struct mixes underscore and kebab JSON tags -- a latent
  inconsistency; the kebab NBMA fields are the rule-correct ones.

## Files

- `internal/component/iface/backend.go`, `active_backend_name_test.go`
- `internal/plugins/ospf/transport/backend_linux.go`
- `internal/test/runner/runner_exec.go`, `runner_exec_util.go`, `runner_exec_util_test.go`
- `scripts/evidence/netns_qemu.py`
- `test/ospf/ospf-nbma.ci`, `test/plugin/ddos-detect-characterize.ci`
