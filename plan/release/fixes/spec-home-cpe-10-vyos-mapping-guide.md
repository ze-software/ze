# Spec: home-cpe-10-vyos-mapping-guide

| Field | Value |
|-------|-------|
| Status | design |
| Scope | docs |
| Priority | P2 |
| Release reason | Documentation for the home-CPE migration: the conversion is done by a model, so a written mapping is the only thing that makes it reproducible and reviewable (umbrella goal 2). It also records every place an accepted VyOS construct has no Ze equivalent today, which is the list of defects children 1 to 22 close. |
| RFC gaps | - |
| Depends | - (rows that name a child carry that child's state; they change when the child lands) |
| Phase | 0/2 |
| Handoff | - |
| Updated | 2026-11-02 |

Parent: `plan/release/fixes/spec-home-cpe-0-umbrella.md` (child 10).

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

Write `docs/guide/migrating-from-vyos.md`, a guide page a model (Claude Opus
5.5) follows to convert a VyOS home-CPE configuration to Ze, and that a human
reads to review the result. No converter program is written.

The page holds:

1. **How the model is fed.** The VyOS configuration in `set` command form,
   secrets redacted before the model sees it; the list of VyOS paths that hold
   secrets; the placeholder form the model writes for each; the rule that the
   model never receives a password, a key or a PSK.
2. **The mapping.** For each VyOS construct a home CPE uses: the Ze form, the
   YANG location it was verified against, and any rewrite the conversion must
   make. Ze has no single-command equivalent of VyOS's tree, so the guide maps
   by construct, not by line.
3. **Dropped constructs**, each with its reason, so "every source line mapped or
   listed as dropped" is checkable.
4. **The conversion record.** The model writes a table beside its output: every
   VyOS source line, the Ze lines it became or "dropped: reason". The record is
   what a reviewer reads and what child 11's `conversion` scenario checks.
5. **The check.** `ze config validate` on the output with lab values
   substituted for placeholders, then a Ze start with it (commit-time verifiers
   do not run under `ze config validate`), then the lab (child 11).
6. **Interim rewrites**, each tied to the child that removes it, so the guide
   shrinks as children land.

## Required Reading

### Architecture Docs
- [ ] `plan/release/fixes/spec-home-cpe-0-umbrella.md` "Conversion by the model", "Capabilities a home FTTP CPE needs"
  → Decision: the model reads the redacted VyOS configuration and writes Ze configuration with placeholders; the owner fills secrets on the router; passwords are re-entered as bcrypt.
- [ ] `docs/guide/configuration.md`, the `ze config validate` paragraph on plugin verifiers
  → Constraint: "It does not run under `ze config validate`, which reads the schema and does not run a plugin's config verifier." The guide's check therefore has a second step that starts Ze with the file.
- [ ] `ai/rules/writing.md`, `ai/rules/documentation.md`
  → Constraint: every factual statement carries a `<!-- source: -->` anchor to the YANG or the producer.
- [ ] `docs/guide/static-routes.md`, the two examples that name `interface pppoe0`
  → Constraint: its `interface pppoe0` example does not resolve today (umbrella doc defect, owned by child 1); the guide must not repeat it.

**Key insights:**
- A placeholder cannot pass validation in a leaf with a format (`ze:bcrypt` password, WireGuard keys), so validation runs on the file with lab values substituted, never on the placeholder file.
- VyOS lets an ESP group stay unattached; Ze needs the peer's group named, so the guide writes an explicit group with `pfs disable` where VyOS's peer had no `default-esp-group`.

## Current Behavior (MANDATORY)

**Source files read (each row of the mapping cites one):**
- [ ] `internal/component/iface/yang/ze-iface-conf.yang` - `list pppoe-client` (`source-interface`, `authentication` `username` `password` `service-name` `ac-name`, `no-default-route`, `route-priority`; `uses interface-common`, which gives `mtu` and `mac`); the description says the OS name is `pppN`; `list ethernet` with `offload` `gro` `gso` `tso` and `mac` `address`/`match`; `list bridge` (`member`, `uses interface-unit`); unit `ipv4 address`, `ipv6 address`; `list wireguard` (`listen-port`, `private-key`, `peer` with `public-key`, `preshared-key`, `allowed-ips`, `persistent-keepalive`).
- [ ] `internal/component/firewall/yang/ze-firewall-conf.yang` - `table` with `family` (`inet`, `ip`, `ip6`, ...), `chain` `type` (`filter`, `nat`, `route`), `hook` (`input`, `postrouting`, ...), `policy` (`accept`, `drop` only); `term` `from` (`protocol` enum with `icmpv6`, `source-address`/`destination-address` accepting `@set`, ports accepting `@set`, `input-interface`/`output-interface` with a trailing `*` prefix match, `tcp-flags`, `connection-state` new/established/related/invalid); `then` (`accept`, `drop`, `reject`, `masquerade`, `tcp-mss-set`, `log`, `counter`); `set` (types `ipv4`, `ipv6`, `inet-service`, ...); `global-options` (`all-ping`, `broadcast-ping`, `syn-cookies`, `receive-redirects`, `send-redirects`, `source-validation`, `log-martians`, `ipv6-receive-redirects`, `ipv6-src-route`; no IPv4 source-route leaf).
- [ ] `internal/component/sysctl/yang/ze-sysctl-conf.yang` - `sysctl setting <name> value`.
- [ ] `internal/plugins/dhcpserver/yang/ze-dhcp-server-conf.yang` - `service dhcp-server` `listen-interface`, `shared-network` `subnet` `prefix`, `range` `start` `stop`, `lease-time`, `default-router`, `dns-server`, `domain-name`, `static-mapping` `mac-address` `ip-address`; no server-identifier, no subnet-id.
- [ ] `internal/component/pki/yang/ze-pki-conf.yang` - `pki ca <n> certificate`, `crl`; `pki certificate <n> certificate`, `intermediate`, `private key`.
- [ ] `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` - `vpn ipsec interface` (fills `local-address` of every peer that names none), `esp-group` `pfs` `enable`/`disable` (default enable), `proposal` `encryption` (enum `aes128gcm`), `ike-group`, `site-to-site peer` (`ike-group`, `esp-group`, `local-address`, `remote-address`, `authentication` `mode`, `x509`, `vti` `bind`).
- [ ] `internal/component/ssh/yang/ze-ssh-conf.yang` - `environment ssh server <n>` with `zt:listener` (`ip`, `port`); `public-keys` augmenting `system authentication user` (`type`, `key`).
- [ ] `internal/component/authz/yang/ze-authz-conf.yang` - `system authentication user <n> password` (`ze:bcrypt`), `plaintext-password`, `profile`.
- [ ] `internal/component/config/system/yang/ze-system-conf.yang` - `system host`, `domain`, `name-server`, `conntrack module`, `update-check` ("On gokrazy, gokrazy manages the image updates."), `commit-revisions`, `archive`.
- [ ] `internal/plugins/ntp/yang/ze-ntp-conf.yang` - `environment ntp server <n> address`.
- [ ] `internal/component/api/grpc/yang/ze-grpc-conf.yang` - `environment api-server grpc server <n>` (`zt:listener`, `tls-cert`, `tls-key`).
- [ ] `internal/component/telemetry/exporter/yang/ze-telemetry-conf.yang` - `telemetry prometheus server <n>`, `netdata` `enabled`, collectors.
- [ ] `internal/plugins/static/yang/ze-static-conf.yang` - `static table route prefix next hop address|interface`.

**Behavior to preserve:** every guide page; this adds one.

**Behavior to change:** `docs/guide/README.md` index gains the page.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| ExaBGP configuration migration | `internal/exabgp/migration/migrate.go` `MigrateFromExaBGP`; `docs/exabgp/exabgp-migration.md` | converts an ExaBGP configuration tree to Ze configuration in code (`ze exabgp migrate`) | does not apply: the umbrella decided no converter program for VyOS; the guide is the VyOS counterpart of the ExaBGP migration page, and the README index lists it the same way |
| Offline validation of a configuration file | `internal/component/config/cli/cmd_validate.go` `cmdValidate`, `internal/component/config/plugin_verify.go` `VerifyPluginConfigContent`; `ai/doors/config-yang.md` Traps | `ze config validate` runs the YANG checks and each registration's in-process verifier, never a live `OnConfigVerify` | extends: the guide's check step uses it first, then a Ze start so the live verifiers run too |
| Parse functional suite | `test/parse/` (`./le test functional parse`) | `.ci` files that feed a configuration to the parser and assert acceptance or refusal | extends: `vyos-guide-example.ci` is one more file in that suite |
| Documentation source anchors | `./le doc check all`; `ai/rules/documentation.md` | checks pages and their `<!-- source: -->` anchors | extends: every mapping row carries an anchor to its YANG file and the gate judges them |
| Guide index | `docs/guide/README.md` task table and page table | lists every guide page, the ExaBGP migration included | extends: one row for the VyOS page |
| Logical interface name resolution | `docs/architecture/iface/logical-name-resolution.md` | how a configured interface name becomes a kernel device | does not apply here: child 1 changes it; the guide only records the interim `ppp*` rewrite until then |

## The mapping the guide carries

Each row is verified against the YANG named in Current Behavior. "Now" means
today's tree; the child column names the spec that changes the row.

| VyOS construct | Ze form | Rewrite or note | Child |
|----------------|---------|-----------------|-------|
| `interfaces pppoe pppoeN source-interface`, `authentication username/password`, `mtu`, `no-default-route` | `interface pppoe-client <name>` with `source-interface`, `authentication username`, `password` (placeholder), `mtu`, `no-default-route` | the name is a config key; the kernel device is `pppN` | 1 |
| references to `pppoe0` in firewall and NAT | `input-interface ppp*` / `output-interface ppp*` | trailing `*` prefix match until child 1 resolves the configured name | 1 |
| static default route `interface pppoe0` | not expressible until child 1; interim: drop `no-default-route` and set `route-priority` to the VyOS distance | meaning preserved (default via the session), flagged in the record | 1 |
| `ip adjust-mss <N>` on the PPPoE interface | a filter chain at hook postrouting, term `from output-interface ppp*` and `tcp-flags` SYN set and RST clear, `then tcp-mss-set <N>` | VyOS writes the rule in `raw` chain `VYOS_TCP_MSS` at postrouting priority -300 with an explicit `size N+1-65535` match (`vyos.ifconfig.Interface.set_tcp_ipv4_mss`); Ze writes no comparison and relies on the kernel guard, which skips a two-byte MSS write that would raise the value, so both clamp and neither raises (child 21 proves it on the installed rule). Postrouting, not forward, keeps locally originated SYNs and SYN-ACKs clamped as VyOS does. `clamp-mss-to-pmtu` has no form until child 9 | 1, 9 |
| `interfaces bridge brN member interface`, `address` | `interface bridge <n>` `member`, unit `ipv4 address` / `ipv6 address` | | - |
| `interfaces ethernet ethN hw-id`, `offload gro/gso/tso` | `interface ethernet <n>` `mac match`, `offload gro/gso/tso` | | - |
| `firewall ipv4/ipv6 input filter` rules | `firewall table <n> family ip/ip6/inet` `chain <n> type filter hook input` with ordered `term`s | VyOS rule numbers become term names in order | - |
| `protocol ipv6-icmp` | `protocol icmpv6` | | - |
| `protocol all` | no `protocol` match | | - |
| `default-action accept/drop` | chain `policy accept/drop` | | - |
| `default-action reject` | final term `then reject`, chain `policy drop` | `policy` is accept or drop only | - |
| `default-log` | final term `then log` and the default action | | - |
| `state established/related` | `connection-state` | | - |
| `firewall group address-group/network-group/port-group` | `set <n> type ipv4/ipv6/inet-service` in the same table (interval flag for networks), referenced as `@<n>` | a set is per table: declare it in every table that uses it | - |
| `firewall global-options` leaves | same names under `firewall global-options` | `ip-src-route` has no leaf: `sysctl setting net.ipv4.conf.all.accept_source_route value 0` | - |
| `nat source rule outbound-interface name pppoe0 translation address masquerade` | table `family ip`, chain `type nat hook postrouting`, term `from output-interface ppp*` `then masquerade` | | 1 |
| `system conntrack modules` | `system conntrack module` | helpers are not built into Ze's kernel until child 3 lands (owner decision Option A: built in, assigned only when listed); `nfs` and `sqlnet` are dropped | 3 |
| `service dhcp-server shared-network-name subnet range start/stop`, `static-mapping mac ip-address`, `option default-router/name-server/domain-name`, `lease` | `service dhcp-server shared-network <n> subnet <p>` `range` `start` `stop`, `static-mapping` `mac-address` `ip-address`, `default-router`, `dns-server`, `domain-name`, `lease-time`, `listen-interface` | dropped: `subnet-id`, `server-identifier`; reservations in a later subnet misbehave until child 2 | 2 |
| `interfaces wireguard wgN address, private-key, port, peer public-key/preshared-key/allowed-ips/persistent-keepalive` | `interface wireguard <n>` `listen-port`, `private-key`, `peer` `public-key`, `preshared-key`, `allowed-ips`, `persistent-keepalive`; unit address | keys are placeholders | - |
| `pki ca <n> certificate`, `pki certificate <n> certificate/private key` | `pki ca <n> certificate`, `pki certificate <n> certificate`, `private key` | VyOS stores base64 bodies; the Ze leaf takes PEM inline, so the model adds the BEGIN/END lines; the private key is a placeholder | 5 for Ed25519 |
| `vpn ipsec interface` | `vpn ipsec interface` | | - |
| `vpn ipsec esp-group/ike-group proposal encryption aes128gcm128` | `esp-group`/`ike-group` `proposal encryption aes128gcm` | | - |
| site-to-site peer with `local-address` | peer with no `local-address` | Ze fills it from `vpn ipsec interface` at apply; a dynamic WAN address follows only after child 4 | 4 |
| peer without `default-esp-group` (ESP group left unattached) | an explicit `esp-group` with `pfs disable`, named on the peer | VyOS rendered an empty ESP proposal; Ze's `pfs` default is enable | - |
| peer `vti bind` | peer `vti bind` to an `xfrm` interface | IPv6 inside an IPv4 SA proven only after children 20 and 6 | 6, 20 |
| `service ssh listen-address 0.0.0.0` and `::` | one `environment ssh server <n> ip :: port 22` | both on one port fail one bind; validate refuses after child 7 | 7 |
| `system login user <n> authentication public-keys <k> type key` | `system authentication user <n> public-keys <k> type key` | | - |
| `system login user <n> authentication encrypted-password` (sha512-crypt) | `system authentication user <n> password` placeholder | re-entered as bcrypt on the router; sha512-crypt is refused at login | - |
| `system host-name`, `domain-name`, `name-server` | `system host`, `domain`, `name-server` | kernel hostname on the appliance after child 7 | 7 |
| `service ntp server` | `environment ntp server <n> address` | | - |
| `system config-management commit-revisions`, `commit-archive location` | `system commit-revisions`, `system archive` | | - |
| `service https api keys` (REST on loopback, plain HTTP, one token) | `environment api-server grpc server <n>` with `tls-cert`, `tls-key` | automation moves to gRPC over TLS | - |
| `container` (monitoring) | `telemetry prometheus server <n>` with `netdata enabled` | Netdata-named metrics replace the container | - |
| `system update-check` | dropped | on gokrazy, gokrazy manages image updates | - |
| `policy prefix-list/route-map` not referenced | dropped | unused BGP policy | - |

### VyOS source inspected

YANG validation proves a Ze form is accepted; it cannot prove the form does
what the VyOS construct did. The VyOS side of each row is therefore read in
VyOS's own source, not inferred from its CLI. Read for this spec: vyos-1x,
branch `rolling`, `git describe` `vyos/1.5dev0-5851-gdb29bdc89` (commit of
2026-10-17), local checkout `github.com/vyos/vyos-1x`. Files read so far:
`python/vyos/ifconfig/interface.py` (`set_tcp_ipv4_mss`, `set_tcp_ipv6_mss`),
`data/vyos-firewall-init.conf` (chain `VYOS_TCP_MSS`),
`src/conf_mode/system_conntrack.py` (`module_map`),
`data/templates/conntrack/nftables-helpers.j2`. Implementation reads the
producer of every other row the same way and names it in the guide, with the
same version line; a row whose VyOS producer was not read carries no class.

### Rewrite classification

| Class | Meaning | What the guide does |
|-------|---------|---------------------|
| equivalent | the Ze form produces the behavior the VyOS producer does, read at both producers | states the mapping |
| conditional | equivalent only while a stated condition holds (a child not yet landed, a value in a range) | states the condition and the child that removes it |
| behavior change | the Ze form differs in an operator-visible way | states what changes and what the operator loses or must do |

Classes of the rows whose VyOS producer is read above, and of the rows whose
difference the table already states; implementation classifies the rest:

| Row | Class | Reason |
|-----|-------|--------|
| `ip adjust-mss <N>` | equivalent | both clamp to N on SYN and SYN-ACK leaving the interface, in postrouting, and neither raises (VyOS by its range match, Ze by the kernel's two-byte guard) |
| `adjust-mss clamp-mss-to-pmtu` | conditional | no Ze form until child 9 |
| `system conntrack modules` | conditional | equivalent for ftp, h323, pptp, sip, tftp once child 3 lands (same ports as VyOS `module_map`); `nfs`, `sqlnet` dropped, a behavior change (no user-space helper in Ze) |
| references to `pppoe0` (`ppp*` prefix match) | conditional | matches every PPP device, not only the named one, until child 1 |
| static default route `interface pppoe0` | behavior change until child 1 | the default route comes from the session (`route-priority`) instead of a static route |
| `system login ... encrypted-password` | behavior change | the hash is not carried; the owner re-enters the password as bcrypt |
| `service ssh listen-address 0.0.0.0` and `::` | behavior change | one listener on `::` instead of two |
| `service https api keys` | behavior change | REST automation moves to gRPC over TLS |
| `container` (monitoring) | behavior change | Netdata-named metrics replace the container |
| site-to-site peer `local-address` | conditional | follows a dynamic WAN address only after child 4 |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- The owner's VyOS configuration in `set` form, redacted.

### Transformation Path
1. Redaction: secrets replaced on the VyOS side before the model sees the file.
2. The model reads the guide and the redacted file, writes the Ze configuration with placeholders and the conversion record.
3. Lab values substituted for placeholders; `ze config validate`; Ze started with the file; child 11's scenarios.
4. On the router, the owner fills the placeholders and re-enters passwords as bcrypt.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| owner ↔ model | redacted text only | No |
| model ↔ Ze | the Ze configuration file | No |

### Integration Points
- Child 11 `conversion` scenario reads the record and the output.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers | Yes | validation through `ze config validate` and a Ze start |
| No unintended coupling | N-A | docs |
| No duplicated functionality | Yes | the guide cites YANG; it restates no leaf description |
| Zero-copy preserved where applicable | N-A | docs |
| Registration over hardcoding, outbound | N-A | docs |
| Registration over hardcoding, inbound | N-A | docs |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | VyOS's `strip-private` output pipe redacts every secret path the guide lists, and leaves addresses the conversion needs readable or replaced consistently | VyOS operational pipe | the guide lists a manual redaction per secret path instead | read vyos-1x's strip-private implementation before writing the section | unvalidated |
| A-2 | A home CPE uses only the constructs in the mapping table | umbrella capability table | a line maps to nothing | the record's "dropped" rows are reviewed; a new construct adds a row | unvalidated |
| A-3 | Placeholders are distinguishable from real values and never valid in any leaf | a fixed `<<SECRET:...>>` form | an unfilled placeholder reaches production | the guide states it; the lab substitution refuses an unknown placeholder (child 11) | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A row goes stale when a child lands | child changes a leaf | each row names its child; the child's Documentation checklist names this page |
| R-2 | The model invents a leaf | validate refuses | validation is the second step of the check, and the record is reviewed |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | a converted configuration behaves differently from the VyOS one |
| How is it reverted? | single commit revert |
| Who else touches this path? | children 1 to 22 edit rows |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| the guide's complete worked example (invented VyOS, its Ze form) | → | `ze config validate` | `test/parse/vyos-guide-example.ci` |
| the converted CPE configuration | → | Ze start and the lab | child 11 scenario `conversion` |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | `docs/guide/migrating-from-vyos.md` | holds the feeding procedure, the secret path list, the placeholder form, the mapping table, the dropped list, the record format, the check and the interim rewrites |
| AC-2 | each mapping row | carries a `<!-- source: -->` anchor to the YANG file that holds its Ze form; `./le doc check all` passes |
| AC-7 | each mapping row | carries its rewrite class (equivalent, conditional, behavior change) from the classification table, and every behavior change names what the operator loses; the page names the VyOS version and the vyos-1x files the rows were read from |
| AC-3 | the worked example | its Ze side passes `ze config validate` in `vyos-guide-example.ci` |
| AC-4 | a row with an interim rewrite | names the child that removes it |
| AC-5 | `docs/guide/README.md` | lists the page |
| AC-6 | `docs/guide/static-routes.md` | the guide links no example that does not resolve |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| N-A | - | documentation: no Go changes; the worked example is proven by the functional test below | |

### Boundary Tests (numeric inputs)
N-A: no numeric input.

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `vyos-guide-example` | `test/parse/vyos-guide-example.ci` | the guide's example configuration is valid Ze | |

### Interop Tests (Scope: protocol)
N-A: docs; child 11 carries the lab proof.

## Files to Modify
- `docs/guide/README.md` - index entry

## Files to Create
- `docs/guide/migrating-from-vyos.md` - the guide
- `test/parse/vyos-guide-example.ci` - the worked example validated

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema | N-A | docs |
| YANG validation constraints | N-A | docs |
| YANG custom validators | N-A | docs |
| CLI commands/flags | N-A | docs |
| CLI grammar | N-A | docs |
| Editor autocomplete | N-A | docs |
| Functional test for new RPC/API | N-A | docs |
| Pipe completeness | N-A | docs |
| Env var registration | N-A | docs |
| Doctor check | N-A | docs |
| Prometheus counters/metrics | N-A | docs |
| BGP family surface | N-A | docs |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | No | |
| 2 | Config syntax changed? | No | |
| 3 | CLI command added/changed? | No | |
| 4 | API/RPC added/changed? | No | |
| 5 | Plugin added/changed? | No | |
| 6 | Has a user guide page? | Yes | `docs/guide/migrating-from-vyos.md` |
| 7 | Wire format changed? | No | |
| 8 | Plugin SDK/protocol changed? | No | |
| 9 | RFC behavior? | No | |
| 10 | Test infrastructure changed? | No | |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` if it names VyOS |
| 12 | Internal architecture changed? | No | |
| 13 | Route metadata keys? | No | |
| 14 | Prometheus counters? | No | |
| 15 | Registered inventory changed? | No | |
| 16 | Changed source referenced by doc anchors? | No | no source changes |
| 17 | Existing docs show examples for this area? | Yes | the page's examples verified against the YANG |

## Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- page skeleton, worked example and its `.ci`
2. **Phase: Mapping** -- every row with its anchor, the dropped list, feeding and checking sections; strip-private read (A-1)

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Correctness | every Ze form re-read in its YANG at writing time |
| Rule: writing | plain words, no em dashes in prose written for Thomas's name |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| Guide page | file exists; anchors resolve |
| Example | `vyos-guide-example.ci` passes |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Secrets | the guide never asks for a secret to be pasted to the model; the example holds invented values |

## Owned journal rows

None: `grep -rn -i 'vyos' plan/journal/` finds no open row.

| Class file | Date | Surface (short) |
|------------|------|-----------------|

## Key Design Decisions

| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Map by construct, with a per-line record | a line-by-line translation table | VyOS and Ze trees differ in shape; the record gives the line coverage |
| Validate with lab values, then start Ze | validate the placeholder file | placeholders fail format leaves; `ze config validate` skips plugin verifiers |
| Interim rewrites live in the guide, tied to children | wait for children before writing | the migration can start now and the rows shrink as children land |

## Known Limitations

- The guide covers the home-CPE construct set of the umbrella; other VyOS features (BGP, VRRP, ...) are not mapped.

## Checklist

### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] AC-N rows carry testable assertions
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints

### Goal Gates (MUST pass)
- [ ] AC-1..AC-7 all demonstrated
- [ ] Every user story has a working path (N-A: docs)
- [ ] Wiring Test table complete
- [ ] The targeted tests for what changed pass: `./le test functional parse` for `vyos-guide-example.ci`, `./le doc check all` for the page's anchors, and `./le verify changed` over the closing commits

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (N-A: no numeric input)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (N-A: docs; child 11 carries the lab proof)

### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean
- [ ] **Commit A** and **Commit B** per `ai/rules/planning.md`
