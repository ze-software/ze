# Spec: yang-feature-opt-ins-owned-by-feature

| Field | Value |
|-------|-------|
| Status | design |
| Priority | P3 |
| Release reason | Release-independent yang feature opt ins owned by feature work; historical findings do not establish a current product defect. |
| Scope | config |
| Depends | `spec-bgp-live-receive-binding-and-bfd-keep-session`, closed 2026-10-13 (it edits the BGP `connection bfd` readers and the BFD client commit refusal; this spec moves the schema those readers read, so it lands after that spec closes) |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-13 |

## Task

Move placement-only feature settings to their owners and maintain the schema-authoring rule. V-5, V-6 and V-7 do not disappear from the existing build and are architecture work. V-8 remains subject to original Q-3 rather than being silently commissioned. BFD and LDP missing-build refusal is owned by the release child, not deferred with this cleanup.

This ownership split preserves the original obligations and evidence. Migration alone neither admits implementation nor closes any criterion; original owner pauses and dependency ordering still apply.

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|---|---|---|
| AC-8 | `ze-route-refresh.yang` after the change | declares the `route-refresh` capability leaf at the three `capability` paths; `ze-bgp-conf.yang` no longer declares it |
| AC-9 | BGP `update` block `watchdog` after the change | declared by a module of `bgp-watchdog`; `ze-bgp-conf.yang` no longer declares it |
| AC-11 | `iface` unit `sysctl-profile` after the change (only if Q-2 says move) | declared by a sysctl module that augments the iface unit paths |
| AC-12 | Build without `ze_ospf` (only if Q-3 says move) | `rib distance ospf` does not exist; with `ze_ospf` it exists, declared by the OSPF module, same default |
| AC-13 | `docs/architecture/config/yang-config-design.md` section 12 and `ai/rules/config.md` | state the rule: a client's setting for feature X is declared by X's module through `augment`, in a package gated on both tags |

AC-1 and AC-10 apply here to V-5..V-8 only: preserve full-build config parity and node help for every moved placement-only node. Retain all original owner questions and audit-reader obligations.

## Implementation Ownership

| Scope | Current owner |
|---|---|
| V-5..V-8 placement; AC-8,9,11,12,13 and AC-1/10 for those nodes; durable ownership rule. | `plan/post-release/engineering/spec-yang-feature-opt-ins-owned-by-feature.md` |
| V-1..V-4; AC-2..7 and AC-1/10 for BFD/LDP nodes; repair-specific ownership documentation. | `plan/release/fixes/spec-yang-feature-opt-in-missing-build-refusal.md` |

## Work Not Done

| Scope | Current owner |
|---|---|
| V-1..V-4; AC-2..7 and AC-1/10 for BFD/LDP nodes; repair-specific ownership documentation. | `plan/release/fixes/spec-yang-feature-opt-in-missing-build-refusal.md` |

## Historical planning record

The following is the complete pre-migration planning body. Its old Task, criteria, implementation steps and bucket statements are retained as history, not a second current implementation contract. The current sections above assign every obligation; original owner decisions, constraints, evidence and pauses remain authoritative. Heading levels alone are demoted for that distinction.

Recovery after compaction: `.claude/rules/post-compaction.md`.

#### Task

Owner request (Thomas, 2026-10-12): the settings a client gives feature X (the
BFD timers or the BFD profile a BGP peer uses) are X's configuration. The YANG
node that holds them MUST be defined by X's module, attached to the client's
tree by `augment`, and MUST NOT exist in a binary that does not carry X. The
client module MUST NOT define X's settings itself.

An audit of every `.yang` file under `internal/` and `pkg/` (2026-10-12) found
the violations below. This spec moves each one into its owner and adds the
check that keeps the rule true.

##### Audit coverage

| Population | Count | How it was read |
|------------|-------|-----------------|
| `.yang` files under `internal/` and `pkg/` | 242 (0 under `pkg/`) | `find internal pkg -name '*.yang'` |
| Test fixtures (`testdata/`), not product schema | 21 | excluded |
| Product modules | 221: 98 `-cmd`, 26 `-api`/`-callback`/`-engine`, 97 config, types, extensions and test-plugin modules | every module mapped to its owning directory and registration (`internal/component/plugin/all/all*.go`, `feature-gates.txt`) |
| Modules that carry an `augment` | 47, 92 augment statements | every augment target tallied: `/bgp:bgp` 54, `/clishowcmd:show` 10, `/iface:interface` 8, `/dd:ddos` 5, `/fw:firewall` 4, `/fib:fib` 2, `/cliclearcmd:clear` 2, `/ad:anomaly` 2, `/tc:traffic` 1, `/hub:environment` 1, `/cliannouncecmd:send` 1, `/authz:system` 1. Every one is a feature module attaching to its host, the direction the rule wants |
| Node-name sweep | 40 domain keywords (bfd, rpki, firewall, sysctl, vrrp, mpls, ldp, rsvp, isis, ospf, dhcp, ntp, flowspec, bmp, graceful, route-refresh, mtu, tacacs, radius, certificate, wireguard, ipsec, l2tp, pppoe, vpp, static, redistribute, shaper, qos, telemetry, authz, rib, fib, policy-route, vrf, ddos, anomaly, geodns, as112, dns) | every `container`, `list`, `leaf`, `leaf-list`, `grouping`, `uses` and `augment` whose name carries a keyword, in a module outside that keyword's owning directory; each hit read in place, and its Go reader located |
| BGP core module `ze-bgp-conf.yang` | every node that a BGP plugin directory also names (watchdog, aigp, rr, route-refresh, adj, rpki, filter) | Go reader located for each |

##### Violations

| # | Node path | Defined in | Belongs to | Survives without the owner? | Evidence |
|---|-----------|-----------|-----------|-----------------------------|----------|
| V-1 | `bgp peer <p> connection bfd` with `enabled`, `mode`, `profile`, `min-ttl`, `interface`, `strict`, `hold-down`, `hold-time`; the same container under `bgp group <g> connection` and `bgp group <g> peer <p> connection` (grouping `peer-fields`, used three times) | `internal/component/bgp/yang/ze-bgp-conf.yang`, grouping `peer-fields`, container `bfd` (`ze_bgp`) | BFD, `internal/component/bfd` (`ze_bfd`) | Yes. A `ze_bgp` build without `ze_bfd` accepts the container; the node's own description says "When the plugin is absent, the BGP peer starts without BFD and Ze logs a warning" | `internal/component/plugin/all/all_ze_bfd.go` (`//go:build ze_bfd`), the `ze_bfd` block of `feature-gates.txt`; readers `internal/component/bgp/config/peers.go` (`GetContainer("bfd")`) and `internal/component/bgp/reactor/config.go` (`mapMap(connMap, "bfd")`) |
| V-2 | OSPFv2 and OSPFv3 interface `bfd`, with the BFD timers inline: `enabled`, `min-tx`, `min-rx`, `multiplier` | `internal/plugins/ospf/yang/ze-ospf-conf.yang`, both interface `bfd` containers (`ze_ospf`) | BFD (`ze_bfd`) | Yes, in a `ze_ospf` build without `ze_bfd` | reader `internal/plugins/ospf/config.go` `parseInterfaceBFD` |
| V-3 | `static route ... next-hop ... bfd-profile` | `internal/plugins/static/yang/ze-static-conf.yang`, leaf `bfd-profile` (always linked, `all.go`) | BFD (`ze_bfd`) | Yes, in every build without `ze_bfd` | reader `internal/plugins/static/config.go` (`entry["bfd-profile"]`) |
| V-4 | OSPFv2 and OSPFv3 interface `ldp-sync` (`enable`, `holddown`) | `internal/plugins/ospf/yang/ze-ospf-conf.yang`, both `ldp-sync` containers (`ze_ospf`) | LDP, `internal/plugins/ldp` (`ze_ldp`): the opt-in only has meaning when LDP runs (RFC 5443 Section 2: the IGP holds the link cost out while LDP is not operational) | Yes, in a `ze_ospf` build without `ze_ldp` | reader `internal/plugins/ospf/config.go` (`m["ldp-sync"]`), state machine `internal/plugins/ospf/ldp_sync.go` |
| V-5 | `update` block `watchdog` (grouping `update-block`, used twice) | `internal/component/bgp/yang/ze-bgp-conf.yang`, grouping `update-block`, container `watchdog` | `bgp-watchdog`, `internal/component/bgp/plugins/watchdog`. The node's own description: "The bgp-watchdog plugin owns every route of the block" | No: both carry `ze_bgp`. Placement only | reader `internal/component/bgp/plugins/watchdog/config.go` `parseConfig` |
| V-6 | `session capability route-refresh` | `internal/component/bgp/yang/ze-bgp-conf.yang`, container `capability`, leaf `route-refresh` | `bgp-route-refresh`, `internal/component/bgp/plugins/route_refresh`: its registration claims capability codes 2 and 70 (`CapabilityCodes` in `register.go`) and registers `ze-route-refresh.yang`, whose body is only a NOTE that the node "is defined in peer-fields grouping (ze-bgp-conf.yang)" | No: both carry `ze_bgp`. Placement only, and the plugin ships an empty module | readers `internal/component/bgp/reactor/config_capabilities.go`, `internal/component/bgp/plugins/route_refresh/route_refresh.go` |
| V-7 | interface unit `sysctl-profile` (reference by `zt:node-name` to a sysctl profile) | `internal/component/iface/yang/ze-iface-conf.yang`, leaf-list `sysctl-profile` | sysctl, `internal/component/sysctl` | No: both always linked. Placement only: it is sysctl's opt-in on the client tree, the same shape as V-3 | reader `internal/component/iface/config.go` |
| V-8 | `rib distance` leaves `ospf`, `isis`, `ebgp`, `ibgp` (and `static`) | `internal/component/sysrib/yang/ze-rib-conf.yang`, container `distance` (always linked) | each protocol: `ospf` (`ze_ospf`), `isis` (`ze_isis`), `ebgp`/`ibgp` (`ze_bgp`); `static` is always linked | Yes for `ospf`, `isis`, `ebgp`, `ibgp`. The module states the choice: "This container is the only place any protocol's distance is declared" | the `distance` container description |

##### Cross-references judged legitimate

| Node | Where | Why it is not a violation |
|------|-------|---------------------------|
| Every `augment` in the 47 augmenting modules | see Audit coverage | each is a feature module attaching to its host (BGP plugins to `bgp`, firewall plugins to `firewall`, ddos/anomaly/fib sub-plugins to their parent, VRRP to `interface`, SSH user keys to `authz`, traffic-usage to `traffic`, BGP environment to `hub`) |
| `ze-policyroute-conf.yang` imports `ze-firewall-conf` | `internal/plugins/policyroute/yang/` | a declared dependency (`TestPolicyRoutesDependsOnFirewall`); it reuses firewall's definitions, it defines none of firewall's settings |
| OSPF interface `ipsec` | `ze-ospf-conf.yang` | RFC 4552 manual keying for OSPFv3 is OSPF's own feature; OSPF installs the XFRM state itself (`internal/plugins/ospf/ipsec_install.go`; `feature-gates.txt` records it as independent of IKE) |
| interface unit `mpls` (`enable`) | `ze-iface-conf.yang` | iface writes `net.mpls.conf.<iface>.input` itself (`internal/component/iface/config.go`, `um["mpls"]`); `ze_mpls` carries only the show-forwarding component and owns no config module. Owner question Q-4 |
| interface `dhcp`, `dhcpv6`, `rdnss`, `wireguard`, `pppoe-client` | `ze-iface-conf.yang` | implemented by iface and its always-linked backends (`internal/plugins/iface/dhcp`, `internal/plugins/iface/ra`, `internal/component/iface/wireguard.go`); the DHCP server plugin is a different feature |
| VPP `plugins wireguard` | `ze-vpp-conf.yang` | selects which VPP shared object VPP loads, a VPP setting |
| `certificate`, `ca-certificate`, `host-certificate` in web, lg, geodns, as112, plugin hub, ssh, ipsec, rpki | each service's module | each names the identity that service presents, a reference to an object of the always-linked PKI component; the reference node is the service's own setting. They are `type string`, not `leafref` |
| BFD session `vrf` | `ze-bfd-conf.yang` | a reference to a VRF name inside BFD's own module |
| `aigp`, `route-reflector-client`, `cluster-id` | `ze-bgp-conf.yang` | read by the BGP reactor core (`reactor/config_aigp.go`, `reactor/config.go`, `reactor/filter/loop.go`); the `aigp` and `rr` plugin directories hold an attribute formatter and commands, not the behavior these leaves configure |
| `exabgp.yang` | `internal/exabgp/migration/` | the input schema of the ExaBGP syntax migration, not Ze's runtime configuration |
| OSPF `redistribute source` enumeration (`connected`, `static`, `kernel`, `bgp`, `isis`) | `ze-ospf-conf.yang` | names other protocols, defines none of their settings. It is a hand-written enumeration of other plugins' names, which `ai/rules/principles.md` refuses; owner question Q-5 |

Not judged: `bgp peer rib adj in|out` (`ze-bgp-conf.yang`, container `rib`). The
reader was not located in the audit; the implementer locates it before Phase 1 (A-6).

#### Required Reading

##### Architecture Docs
- [ ] `docs/architecture/config/yang-config-design.md` section 12, "Structure, Toggles, Defaults and Layout"
  → Constraint: `augment` is used "only when a plugin extends another component's YANG"; the rule this spec enforces is the converse, and the page does not state it yet.
  → Constraint: "A binary links every module a linked module imports, because the generated `register.go` of each `yang/` package blank-imports the packages registering its modules' imports". A BFD module that imports `ze-bgp-conf` therefore links BGP's schema into a BFD-only build; the augment module MUST be a separate package gated on both tags.
- [ ] `docs/architecture/config/yang-config-design.md` "Several modules can declare one node"
  → Decision: container merge (same-named node, no import) is the other attachment shape; it is not used here, because an augment is checked against its target at load and a container merge of `bgp` in a build without BGP would create a phantom root.
- [ ] `docs/architecture/command-ownership.md` "Container merge" and "Augment"
  → Constraint: an augment names its target module, so the target module must be linked wherever the augmenting module is.
- [ ] `feature-gates.txt` (header, the `ze_bfd` block, the `ze_radius` block)
  → Constraint: a package is gated by one manifest row; a package that lives under another gated path gets the compound constraint from `parentTagOf` path nesting (`authradius` under `component/l2tp`, listed as `ze_radius`, emits `ze_l2tp && ze_radius`; `all_ze_radius_ze_l2tp.go` is the generated file). The client augment packages reuse that shape.
  → Constraint: `internal/component/bfd/api` and `bfd/packet` are the always-on client contract; BGP, OSPF and static nil-check `GetService`. The Go readers stay as they are.
- [ ] `ai/rules/config.md`
  → Constraint: an unknown key MUST fail at every level with the closest valid key suggested. After the move, `connection bfd` in a build without `ze_bfd` is an unknown key and is refused, which replaces today's "starts without BFD and logs a warning".
- [ ] `ai/rules/plugins.md`
  → Constraint: a plugin owns its entire feature surface.
- [ ] `spec-plugin-ownership-imports`
  → Constraint: that spec removes the 35 import edges `./le arch tier check` reports, and nothing else. It moves a command schema only where the schema's location is itself one of those edges (the `mtu`, `ping`, `traceroute` `-cmd` modules and `ze-rib-poolstats-cmd`); it moves no other command schema and no config node. Shared files: `feature-gates.txt` and the generated `all*.go` only. (Corrected 2026-10-12 after that spec's design review round 4: this line had said it moves command schemas to their owners in general.)

##### RFC Summaries (references only, no protocol change)
- [ ] `rfc/short/rfc5443.md` - LDP-IGP synchronization
  → Constraint: Section 2: the IGP advertises maximum cost while LDP is not operational. Without LDP in the binary the opt-in can never be satisfied (A-3).
- [ ] `rfc/short/rfc5880.md`, `rfc/short/rfc5881.md`, `rfc/short/rfc5883.md` - BFD timers and hop modes
  → Constraint: no wire change; the move changes only which module declares the leaves.

**Key insights:**
- Augment keeps every config path unchanged, so no Go reader, no config file and no `.ci` config text changes for V-1 to V-7.
- An augment module imports its target, and an import links the target. Each client augment therefore lives in its own package gated on both tags.
- Every shipped build carries every tag (`gokrazy/ze/config.json` GoBuildTags lists all 37 feature tags), so no operator meets V-1 to V-4 today. The defect is in reduced builds and in ownership.

#### Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/bgp/yang/ze-bgp-conf.yang` - imports only `ze-types`, `ze-extensions`, `ze-hub-conf`; defines `connection bfd` in grouping `peer-fields`, `watchdog` in grouping `update-block`, `capability route-refresh`
- [ ] `internal/plugins/ospf/yang/ze-ospf-conf.yang` - imports no BFD or LDP module; defines `bfd` with inline timers and `ldp-sync` for both address families
- [ ] `internal/plugins/static/yang/ze-static-conf.yang` - defines `bfd-profile` as `type string`
- [ ] `internal/component/bfd/yang/ze-bfd-conf.yang` - top-level `bfd` container: `profile`, `single-hop-session`, `multi-hop-session` lists; no augment
- [ ] `internal/component/bgp/plugins/route_refresh/yang/ze-route-refresh.yang` - imports `ze-bgp-conf`, body is a NOTE only
- [ ] `internal/component/bgp/plugins/gr/yang/ze-graceful-restart.yang` - the precedent: augments the three `peer-fields` instantiations (`/bgp:bgp/bgp:peer/...`, `/bgp:bgp/bgp:group/bgp:peer/...`, `/bgp:bgp/bgp:group/...`)
- [ ] `internal/component/sysrib/yang/ze-rib-conf.yang` - `distance` container, one leaf per protocol
- [ ] `internal/component/iface/yang/ze-iface-conf.yang` - `sysctl-profile` leaf-list
- [ ] `internal/component/plugin/all/all_ze_bfd.go`, `all_ze_radius_ze_l2tp.go` - one-tag and compound-tag group files, generated by `./le plugin imports update`

**Behavior to preserve:**
- Every config path. A config that loads today in a full build loads unchanged, and `ze config dump` prints the same text.
- Every Go reader of the moved nodes reads the same JSON keys at the same paths.
- `?` help and completion text of each moved node (the description and `ze:help` move with the node).
- BFD client behavior in a build with `ze_bfd`, including the commit refusal another session is changing now.

**Behavior to change:**
- In a build without `ze_bfd`, `connection bfd`, the OSPF `bfd` containers and `bfd-profile` do not exist, so a config that sets them is refused at parse with the unknown-key error.
- In a build without `ze_ldp`, OSPF `ldp-sync` does not exist.
- In a build without `ze_ospf`, `ze_isis` or `ze_bgp`, the matching `rib distance` leaf does not exist (only if Q-3 is answered "move").
- `ze-route-refresh.yang` stops being an empty module: it declares the capability leaf.

#### Data Flow (MANDATORY - see `ai/rules/architecture.md`)

##### Entry Point
- Config text or `set` commands; the YANG loader builds the schema from every registered module of the binary.

##### Transformation Path
1. Each `yang/` package registers its module text at `init()`; the generated `register.go` blank-imports the packages of the modules it imports.
2. `DefaultLoader()` parses every module and resolves each `augment` against its target, so the augmented node sits at the target path.
3. The parser builds the tree; the resolver hands `map[string]any` to the reader (BGP reactor, OSPF config, static config, sysrib).
4. Readers are unchanged: the key sits at the same path.

##### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| YANG module to YANG module | `import` plus `augment`, linked by the generated `register.go` | No |
| Build tags | `feature-gates.txt` row, compound constraint from path nesting | No |
| Config to reader | unchanged JSON map | No |

##### Integration Points
- `internal/le/plugin/imports` - generates the group files from `feature-gates.txt`; the new packages get their rows there.
- `internal/component/plugin/all/config_claims_test.go` `TestFeatureGatedModulesEnumerated` - lists gated modules; the new modules appear in it.

##### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes | the loader resolves the augment; readers unchanged |
| No unintended coupling (components stay isolated) | Yes | client modules lose their only mention of BFD and LDP; the BFD-owned augment package imports the client module only under the compound tag |
| No duplicated functionality (extends existing, does not recreate) | Yes | the node moves; the client copy is deleted first (`ai/rules/no-layering.md`) |
| Zero-copy preserved where applicable (refs, not copies) | N-A | schema only |
| Registration over hardcoding, outbound | Yes | each augment package registers through the generated `register.go`; no core list learns a name |
| Registration over hardcoding, inbound | Yes for V-1 to V-7; V-8 removes the central `distance` enumeration if Q-3 says move | searched: `ze-rib-conf.yang` distance leaves, OSPF `redistribute source` enumeration (Q-5) |

#### Risks & Assumptions

##### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The YANG loader resolves an augment whose target node sits inside a `uses` of a grouping, at each instantiation path | `ze-graceful-restart.yang` augments `peer-fields` instantiations today | V-1 needs a different shape | load a full-tag schema after Phase 2 and assert the three paths | unvalidated |
| A-2 | `./le plugin imports update` emits a compound-tag group file for a package listed under one tag and nested under another tag's path, for `ze_bfd` under `ze_bgp`, `ze_ospf`, and under the untagged static path | the `ze_radius` block of `feature-gates.txt`, `all_ze_radius_ze_l2tp.go` | the generator needs an explicit compound row form | run the generator, inspect the emitted `all_ze_*.go` | unvalidated |
| A-3 | In a `ze_ospf` build without `ze_ldp`, an interface with `ldp-sync enable` is held at maximum cost for ever, because no LDP ever reports operational | RFC 5443 Section 2; `ldp_sync.go` header | V-4 is placement only, not a live hazard | read `ldp_sync.go` for the no-LDP path; a unit test with no LDP source | unvalidated |
| A-4 | No functional test or unit test is built without `ze_bfd` and sets `connection bfd` | the runner adds every tag (`internal/test/runner.TestBuildTags`) | that test turns red as intended and is rewritten to assert the refusal | grep `test/` and the reduced-tag tests in `cmd/ze/build_tag_*_test.go` | unvalidated |
| A-5 | The reduced-tag build tests in `cmd/ze/build_tag_*_test.go` can load the config schema, so a test can assert a node is absent in a build without `ze_bfd` | the files exist | the absence test needs a new reduced-tag harness | read those tests | unvalidated |
| A-6 | `bgp peer rib adj in|out` is read by core BGP code, not by `bgp-adj-rib-in` | not located in the audit | it is a ninth violation of the V-5 shape | locate the reader | unvalidated |
| A-7 | The owner wants the strict-mode leaves (`strict`, `hold-down`, `hold-time`, draft-ietf-idr-bgp-bfd-strict-mode, a BGP capability) to move with the rest of `connection bfd` | owner rule: a setting that only has meaning with X is X's | they stay in BGP and BGP needs its own BFD-less refusal | Q-1 | unvalidated |

##### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | The augment package imports `ze-bgp-conf`, so a careless gate links BGP into a BFD-only build | `go list -deps -tags ze_bfd ./cmd/ze` shows `bgp/yang` | compound-tag package (A-2); an absence test in a `ze_bfd`-only build |
| R-2 | The moved node loses its `?` help or completion because description order changes under augment merge | `?` on `connection bfd` shows a different text | functional test asserts the help text of `connection bfd` |
| R-3 | Collision with the in-flight BFD keep-session spec, which edits the readers and the commit refusal | both specs list `ze-bgp-conf.yang` | `Depends` row: this spec starts after that one closes |
| R-4 | `./le config unread-leaves` attributes leaves to the module's package; a leaf declared by the BFD package and read by BGP looks "unread" | that report lists `connection bfd` leaves after the move | teach the report that the reader may sit in the augment target's package, or record the leaves; decide in Phase 1 |
| R-5 | `./le doc yang-contract` usage baseline, built from HEAD modules, reports moved nodes as removed | contract diff after the move | regenerate the baseline in the same commit |
| R-6 | Doc pages cite `ze-bgp-conf.yang` anchors for `connection bfd` | `./le spec citation anchors` | update the anchors in the phase that moves the node |

#### Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | A full build refuses a config it accepted before (an augment path wrong), or loses help text. Reduced builds refuse configs that set an absent feature, which is the intent |
| How is it reverted? | single commit revert; no config migration, paths are unchanged |
| Who else touches this path? | `spec-bgp-live-receive-binding-and-bfd-keep-session` (BGP BFD readers, commit refusal), `spec-plugin-ownership-imports` (generated `all*.go`, `feature-gates.txt`) |

#### Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| config text `bgp peer ... connection bfd { profile p }` in a full build | → | BFD-owned augment of `ze-bgp-conf`, BGP reader `reactor/config.go` | `test/parse/bfd-client-augment-bgp.ci` |
| config text with OSPF interface `bfd { min-tx ... }` in a full build | → | BFD-owned augment of `ze-ospf-conf`, `parseInterfaceBFD` | `test/parse/bfd-client-augment-ospf.ci` |
| config text with static next-hop `bfd-profile` in a full build | → | BFD-owned augment of `ze-static-conf`, static `config.go` | `test/parse/bfd-client-augment-static.ci` |
| schema of a build without `ze_bfd` | → | loader, no BFD augment registered | `TestBFDClientNodesAbsentWithoutBFD` |
| schema of a build without `ze_ldp` | → | loader, no LDP augment registered | `TestLDPSyncAbsentWithoutLDP` |

#### Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | Full-tag build; every config under `test/` that sets `connection bfd`, OSPF `bfd`, `bfd-profile`, `ldp-sync`, `watchdog`, `capability route-refresh` or `sysctl-profile` | parses and dumps byte-identical to before the change |
| AC-2 | `ze-bgp-conf.yang`, `ze-ospf-conf.yang`, `ze-static-conf.yang` after the change | contain no node named `bfd` or `bfd-profile`, and no description that names BFD as a setting they define |
| AC-3 | Build with `ze_bgp` and without `ze_bfd`; config sets `bgp peer <p> connection bfd` | parse refuses it with the unknown-key error naming `bfd`, exit non-zero |
| AC-4 | Same for OSPF (`ze_ospf`, no `ze_bfd`) interface `bfd`, and for static `bfd-profile` (no `ze_bfd`) | refused with the unknown-key error |
| AC-5 | Build with `ze_bfd` and without `ze_bgp` | links no BGP schema package (`go list -deps` shows no `internal/component/bgp/yang`) and loads its schema |
| AC-6 | Build with `ze_ospf` and without `ze_ldp`; config sets OSPF `ldp-sync` | refused with the unknown-key error |
| AC-7 | `ze-ospf-conf.yang` after the change | contains no `ldp-sync` node |
| AC-8 | `ze-route-refresh.yang` after the change | declares the `route-refresh` capability leaf at the three `capability` paths; `ze-bgp-conf.yang` no longer declares it |
| AC-9 | BGP `update` block `watchdog` after the change | declared by a module of `bgp-watchdog`; `ze-bgp-conf.yang` no longer declares it |
| AC-10 | `?` on each moved node in the editor | shows the same `ze:help` and description as before |
| AC-11 | `iface` unit `sysctl-profile` after the change (only if Q-2 says move) | declared by a sysctl module that augments the iface unit paths |
| AC-12 | Build without `ze_ospf` (only if Q-3 says move) | `rib distance ospf` does not exist; with `ze_ospf` it exists, declared by the OSPF module, same default |
| AC-13 | `docs/architecture/config/yang-config-design.md` section 12 and `ai/rules/config.md` | state the rule: a client's setting for feature X is declared by X's module through `augment`, in a package gated on both tags |

#### End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Operator of a full build enables BFD on a BGP peer | config text → loader (BFD augment) → BGP reactor → BFD service | existing `test/reload/bgp-bfd-*.ci` stay green, plus `test/parse/bfd-client-augment-bgp.ci` |
| 2 | Builder of a BGP-only image loads a config with `connection bfd` | config text → loader without BFD augment → unknown key | `TestBFDClientNodesAbsentWithoutBFD` plus a reduced-tag parse test |

#### 🧪 TDD Test Plan

##### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestBFDClientNodesAbsentWithoutBFD` | reduced-tag test beside `cmd/ze/build_tag_*_test.go` (A-5) | AC-3, AC-4: no `bfd` under BGP `connection`, OSPF interface, static next-hop | |
| `TestBFDClientNodesPresentWithBFD` | `internal/component/plugin/all/` | AC-1: the three BGP paths, two OSPF paths, static path exist with the BFD module as declarer | |
| `TestLDPSyncAbsentWithoutLDP` | reduced-tag test | AC-6 | |
| `TestBFDBuildLinksNoBGPSchema` | reduced-tag test | AC-5 | |
| `TestRouteRefreshCapabilityDeclaredByPlugin` | `internal/component/bgp/plugins/route_refresh/` | AC-8 | |
| `TestWatchdogDeclaredByPlugin` | `internal/component/bgp/plugins/watchdog/` | AC-9 | |

##### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| OSPF `bfd min-tx` (moved, unchanged) | 1-10000 | 10000 | 0 | 10001 |
| OSPF `bfd min-rx` (moved, unchanged) | 1-10000 | 10000 | 0 | 10001 |
| OSPF `bfd multiplier` (moved, unchanged) | 1-255 | 255 | 0 | 256 |
| BGP `bfd min-ttl` (moved, unchanged) | 0-255 | 255 | N/A | 256 |

##### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| `bfd-client-augment-bgp` | `test/parse/` | BGP peer `connection bfd` with every leaf parses and dumps unchanged | |
| `bfd-client-augment-ospf` | `test/parse/` | OSPFv2 and OSPFv3 interface `bfd` parses and dumps unchanged | |
| `bfd-client-augment-static` | `test/parse/` | static `bfd-profile` parses and dumps unchanged | |
| existing `test/reload/bgp-bfd-*.ci`, `test/reload/static-bfd-*.ci`, OSPF BFD and LDP-sync `.ci` | `test/` | unchanged behavior in a full build | |

##### Interop Tests
N-A: no wire change. Config paths and runtime behavior are unchanged in a full build.

#### Files to Modify
- `internal/component/bgp/yang/ze-bgp-conf.yang` - delete `connection bfd`, `watchdog`, `capability route-refresh` (deleted before the augments are added)
- `internal/plugins/ospf/yang/ze-ospf-conf.yang` - delete both `bfd` containers and both `ldp-sync` containers
- `internal/plugins/static/yang/ze-static-conf.yang` - delete `bfd-profile`
- `internal/component/bgp/plugins/route_refresh/yang/ze-route-refresh.yang` - declare the capability leaf by augment
- `internal/component/iface/yang/ze-iface-conf.yang` - delete `sysctl-profile` (Q-2)
- `internal/component/sysrib/yang/ze-rib-conf.yang` - delete the per-protocol distance leaves of gated protocols (Q-3)
- `feature-gates.txt` - rows for the new augment packages
- `internal/component/plugin/all/all*.go` - regenerated by `./le plugin imports update`
- `docs/architecture/config/yang-config-design.md` - section 12 rule (AC-13)
- `ai/rules/config.md` - the rule, through its point file and `./le ai rules condensed-update`
- `docs/guide/bfd.md`, `docs/guide/static-routes.md`, `docs/architecture/static-routes.md`, `docs/architecture/ospf/ospf-ext-11-ldp-igp-sync.md`, `docs/guide/config-reload.md`, `docs/architecture/behavior/fsm.md`, `docs/architecture/wire/capabilities.md` - where they say which module declares the node, or that a BFD-less build "starts without BFD and logs a warning"

#### Files to Create
- `internal/component/bfd/clients/bgp/yang/` - BFD-owned augment of the three BGP `connection` paths (compound `ze_bgp && ze_bfd`); exact directory decided by A-2
- `internal/component/bfd/clients/ospf/yang/` - BFD-owned augment of the OSPFv2 and OSPFv3 interface paths (`ze_ospf && ze_bfd`)
- `internal/component/bfd/clients/static/yang/` - BFD-owned augment of the static next-hop path (`ze_bfd`)
- `internal/plugins/ldp/clients/ospf/yang/` - LDP-owned augment of the OSPF interface paths (`ze_ospf && ze_ldp`)
- `internal/component/bgp/plugins/watchdog/yang/` - watchdog module augmenting the `update-block` instantiations
- `test/parse/bfd-client-augment-bgp.ci`, `test/parse/bfd-client-augment-ospf.ci`, `test/parse/bfd-client-augment-static.ci`

##### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | the augment modules listed under Files to Create; no new leaf |
| YANG validation constraints | Yes | each moved leaf keeps its range, type and default unchanged |
| YANG custom validators | No | none on the moved leaves; the BFD profile existence check stays in `internal/component/bfd/api/profile_check.go` |
| CLI commands/flags | N-A | no command changes |
| CLI grammar (keyword before value) | N-A | no command changes |
| Editor autocomplete | Yes | automatic from the augmented schema; AC-10 |
| Functional test for new RPC/API | N-A | no RPC |
| Pipe completeness | N-A | no output |
| Env var registration | N-A | no `environment/` leaf moves |
| Doctor check for runtime dependencies | N-A | no runtime dependency added |
| Prometheus counters/metrics | N-A | none |
| BGP family surface (new SAFI / capability / attribute) | N-A | the route-refresh capability leaf moves module, its codes are already registered |

##### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature, or a feature's scope, evidence or level changed? | No | no feature level changes in a full build |
| 2 | Config syntax changed? | No | paths unchanged; `docs/architecture/config/syntax.md` untouched |
| 3 | CLI command added/changed? | No | none |
| 4 | API/RPC added/changed? | No | none |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` if it says which module declares the BFD opt-in; checked in the phase |
| 6 | Has a user guide page? | Yes | `docs/guide/bfd.md`, `docs/guide/static-routes.md`, `docs/guide/config-reload.md` |
| 7 | Wire format changed? | No | none |
| 8 | Plugin SDK/protocol changed? | No | none |
| 9 | RFC behavior implemented, changed, or newly proven? | No | none |
| 10 | Test infrastructure changed? | Yes if A-5 needs a new reduced-tag harness | `docs/functional-tests.md` |
| 11 | Affects daemon comparison? | No | `docs/comparison.md` mentions BFD support, not module placement; checked in the phase |
| 12 | Internal architecture changed? | Yes | `docs/architecture/config/yang-config-design.md` section 12; `feature-gates.txt` comment of the `ze_bfd` block |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | No | none |
| 15 | Registered plugin, event type, send type, command, capability, or inventory changed? | Yes | `./le repo inventory` YANG module count; `docs/plugin-overview.md` if it counts modules |
| 16 | Any changed source file referenced by existing doc source anchors? | Yes | run `./le spec citation anchors spec plan/post-release/engineering/spec-yang-feature-opt-ins-owned-by-feature.md` at Phase 1 and name each |
| 17 | Existing docs show config/CLI/API examples for this area? | Yes | examples stay valid (paths unchanged); prose that says BGP "starts without BFD and logs a warning" changes |

#### Implementation Steps

1. **Phase: Wiring (MANDATORY FIRST)** -- resolve A-1, A-2, A-5, A-6; add the empty BFD client augment package for BGP with its `feature-gates.txt` row, regenerate the group files, and write `TestBFDClientNodesAbsentWithoutBFD` and `TestBFDBuildLinksNoBGPSchema`
   - Tests: `TestBFDClientNodesAbsentWithoutBFD` (red: the node exists), `TestBFDBuildLinksNoBGPSchema`
   - Files: `feature-gates.txt`, the new package, generated `all*.go`
   - Verify: the generated group file carries `ze_bgp && ze_bfd`
2. **Phase: V-1 BGP BFD** -- delete `connection bfd` from `ze-bgp-conf.yang`, then declare it in the BFD augment at the three paths; move descriptions; rewrite the descriptions that promise a BFD-less build "starts without BFD"
   - Tests: AC-1, AC-2, AC-3, AC-5, AC-10, `bfd-client-augment-bgp`
3. **Phase: V-2, V-3 OSPF and static BFD** - same shape; OSPF timers keep their leaves unless Q-6 says profile
   - Tests: AC-4, `bfd-client-augment-ospf`, `bfd-client-augment-static`
4. **Phase: V-4 LDP sync** - LDP-owned augment of OSPF interfaces; resolve A-3 first
   - Tests: AC-6, AC-7, `TestLDPSyncAbsentWithoutLDP`
5. **Phase: V-5, V-6 BGP plugin nodes** - watchdog module, route-refresh module
   - Tests: AC-8, AC-9
6. **Phase: V-7, V-8** - only for the questions the owner answers "move"
   - Tests: AC-11, AC-12
7. **Phase: Rule** - AC-13, and the regression check named in Q-7

##### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every V-N has its node deleted from the client module and declared by its owner |
| Correctness | `ze config dump` of every `test/` config that uses a moved node is byte-identical before and after |
| Data flow | no Go reader changed; a changed reader means a path changed, which is a defect |
| Rule: no-layering | no node exists in both the client and the owner module at any commit |
| Rule: feature gates | no always-on package imports a gated package (`./le arch tier check`) |

##### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| No BFD node in client modules | grep of `ze-bgp-conf.yang`, `ze-ospf-conf.yang`, `ze-static-conf.yang` finds no `bfd` node declaration |
| Compound gates | generated `all_ze_*.go` carries the compound constraints |
| Reduced builds refuse | AC-3, AC-4, AC-6 tests green |

##### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Input validation | moved leaves keep their ranges; an augment that drops a `range` widens accepted input |

##### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read the source in Current Behavior. If misunderstood → RESEARCH |
| Lint failure | Fix inline. If architectural → DESIGN |
| Functional test fails | Check the AC: wrong AC → DESIGN, correct AC → IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase and implement |
| 3 fix attempts failed | STOP. Report all 3 approaches. Ask the user |

#### Design Insights

- The docs state when `augment` is allowed, not when it is required. The rule this spec adds is the converse, and without a check it decays the same way it did here.

#### Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Owner-declared `augment` in a package gated on both tags | (B) container merge without import, gated the same way; (C) keep the node in the client and gate it with YANG `if-feature` (`plan/post-release/features/spec-config-yang-if-feature.md`, not implemented) | an augment is checked against its target at load, so a typo fails loudly; B can create a phantom root and nothing checks its path; C keeps the node in the client module, which the owner's rule forbids |
| Paths unchanged | rename to `bfd-client` or similar | no config migration, no reader change |
| Go readers stay in the clients | move the parse of `connection bfd` into `bfd/api` | the owner's rule is about the schema; the reader is the client's behavior and reads through the always-on `bfd/api` contract |

#### Known Limitations
- Command schemas are outside this spec: `internal/component/cmd/show/yang/ze-cli-show-api.yang` declares `rpc l2tp-health` (handler `internal/component/l2tp/show_health.go`, `ze_l2tp`) and `rpc bgp-health` (handler `internal/component/bgp/plugins/cmd/peer/health.go`, `ze_bgp`) in an always-linked module. Answered by the owner on 2026-10-13 (Q-8): "this is a boundary cross IMHO BGP need to be in the bgp plugin and same for l2tp". The move belongs to `spec-plugin-ownership-imports` (route J, AC-17); out of scope here.

#### Open Questions (which way, with recommendation)

| # | Question | Recommendation |
|---|----------|----------------|
| Q-1 | BGP BFD strict mode (`strict`, `hold-down`, `hold-time`) is a BGP capability that only has meaning with BFD. Move with the rest of `connection bfd` into the BFD augment, or keep in BGP? | Move. It is meaningless without BFD, and keeping it would leave BGP a `bfd`-named node to refuse |
| Q-2 | iface `sysctl-profile`: move to a sysctl-owned augment, or leave as a reference in iface? Both are always linked, so nothing survives | Move, for the same reason as `bfd-profile` (V-3); it is cheap and keeps one rule |
| Q-3 | `rib distance`: each protocol augments its own leaf, or sysrib keeps one central list on purpose? | Move: the gated protocols' leaves survive without them, and the central list is an enumeration `ai/rules/principles.md` refuses |
| Q-4 | interface `mpls enable`: MPLS's setting (move to `ze_mpls`) or iface's kernel setting? | Leave in iface: iface implements it and `ze_mpls` owns no config |
| Q-5 | OSPF `redistribute source` hard-codes `bgp`, `isis` as enum values present without those plugins. Fix here (string plus registry, as `ze-redistribute-conf` does) or separately? | Separately: it is a reference, not a definition |
| Q-6 | OSPF `bfd` carries inline timers where BGP and static name a BFD profile. Keep the leaves (pure move), or replace with `profile`? | Pure move here; a profile change alters config and is its own decision |
| Q-7 | How is the rule kept true: a reduced-tag schema test per gate, or a static check that no module names a node a gated package reads? | Reduced-tag schema test for the gates that have client opt-ins (`ze_bfd`, `ze_ldp`), plus the rule text |
| Q-8 | Recorded as Q-C in `spec-plugin-ownership-imports`. Where do the `l2tp-health` and `bgp-health` RPCs go? `internal/component/cmd/show/yang/ze-cli-show-api.yang` declares them in an always-linked module, while their handlers are gated (`ze_l2tp`, `ze_bgp`). Options: (a) the ownership-imports spec; (b) this spec; (c) a spec of their own | **Decided 2026-10-13: (a).** Thomas: "this is a boundary cross IMHO BGP need to be in the bgp plugin and same for l2tp". Moved to `spec-plugin-ownership-imports` (route J, AC-17, which proves it with a schema test since that spec's gate cannot see it); out of scope here |
| Q-9 | Bucket: `plan/` (no shipped build lacks a tag today) or `plan/pre-release/`? | `plan/` |

#### Checklist

##### Pre-Spec Verification (before the design is presented)
- [ ] Metadata table present, with a valid Status, Depends, Phase and Updated
- [ ] `ai/INDEX.md` keyword table checked
- [ ] An `rfc/short/` summary exists for every RFC referenced
- [ ] Template format followed: the 🧪 emoji, tables rather than prose, `[ ]` never `[x]`
- [ ] No code snippets
- [ ] Files to Modify names feature code, not only tests
- [ ] Current Behavior and Data Flow sections completed
- [ ] AC-N rows carry testable assertions
- [ ] Every assumption has a Basis and a validation method; every failure mode is a risk row
- [ ] Required Reading carries `→ Decision:` / `→ Constraint:` checkpoints
- [ ] Integration Checklist marks "CLI grammar" when a command is added, "Doctor check" when a runtime dependency is

##### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test (N-A when Scope is tooling or docs, which delete that section)
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes. It runs every stage against a COMMIT in a throwaway worktree, which is the pre-commit gate (`ai/rules/git-safety.md`). An in-place `./le verify current` is void the moment the tree moves under it
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied: lint fixed rather than disabled, the focused check for the changed behavior run once with its OUTPUT PASTED, and any red named with the one-line reason it is scaffolding
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

##### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs (or N-A when the feature takes none)
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests for protocol features (or N-A with a reason)

##### Closure
- [ ] Append `plan/templates/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`; no lesson artifact created merely for closure
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove <the spec's path in its bucket>` only, in the same `./le commit create` script (commit A preserves the spec in history)
