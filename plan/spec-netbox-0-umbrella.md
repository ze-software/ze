# Spec: netbox-0-umbrella -- Ze builds its configuration from NetBox

| Field | Value |
|-------|-------|
| Status | design |
| Scope | config |
| Depends | spec-irr-apply-policy (the apply policy this set reuses, see "Apply policy: one mechanism"); spec-fleet-2-config-templates (overlap on hub-side composition, see R-9) |
| Phase | - |
| Handoff | - |
| Updated | 2026-10-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

This is the umbrella of a spec set (`docs/contributing/spec-workflow.md`, "Spec sets"). It
carries the whole design, every owner decision, the NetBox survey and the cut into children.
The children are written after the owner approves the cut; each child copies its rows from here
and never restates a decision differently. Status stays `design` until the owner approves.

## Task

Owner request (Thomas, 2026-10-08): Ze builds its configuration from NetBox data. Not scheduled
for implementation; the spec is written now so that it can be completed, once Thomas gives read
access to his NetBox, by running the survey in "NetBox Survey" and revisiting only the sections
marked with a survey ID.

The principle is `docs/architecture/core-design.md` section 22 "Sources and Read Views": NetBox
is a normalized, write-optimized source; a builder pulls from it and writes a read view (Ze
config); the router pulls the read view. Nothing pushes data; a notification means "pull now".
Nobody edits a read view. Fetching and applying are separate decisions. Section 22 already names
the shape: "the hub pulls from NetBox and builds the configuration of each router, and the router
pulls from the hub. Ze has no NetBox builder today."

### Owner decisions (binding, 2026-10-08)

| ID | Decision |
|----|----------|
| D-1 | NetBox is the source for instances; Ze config holds templates. A Ze BGP group is the template (existing group -> peer inheritance). NetBox supplies the members and their per-instance values |
| D-2 | The YANG config describes how the data is named in NetBox, with sane defaults when nothing is configured, overridable. Default: the NetBox object names the Ze group by the group's own name (the `peer_group` choice value == Ze group name, D-19) |
| D-3 | NetBox settings sit on the group (template), never on an individual peer |
| D-4 | SUPERSEDED by D-13, D-17 and D-19: ~~netbox-bgp will be installed on the owner's NetBox; design for it~~. netbox-bgp is not used (D-19) |
| D-5 | VPN profile bodies (IKE/IPsec policies and proposals) come from NetBox, the one exception to "bodies live in Ze" |
| D-6 | Restated by D-17, then by D-19: a NetBox value Ze cannot apply exactly is refused, never silently ignored. Ze reads only the fields its own BGP model defines, so no foreign body field (policies, prefix lists, max-prefix) exists to refuse; a remote address carrying a NetBox `vrf` is refused (v1 maps no VRF). Prefix limits, policies and max-prefix are template body in the Ze group |
| D-7 | A customer service spanning interface + VLAN + IP + BGP is tied together with a tag or a custom field; no Custom Objects plugin (may come later) |
| D-8 | No pre-merge validation of NetBox Branching branches (Branching is not installed); future work |
| D-9 | A router never modifies NetBox and only reaches its own configuration: the builder runs on the fleet hub with one read-only NetBox token; routers hold no NetBox credential and fetch their generated config from the hub. Ze pulls from NetBox and never writes to it; anything flowing back (session state) is published by Ze for others to pull |
| D-10 | Generated entries are keyed by NetBox object ID, never by name, so a rename never deletes and re-adds (flaps) a session. Generated entries are owned by NetBox and read-only on the router (an edit is refused with a pointer to the NetBox object). Each generated entry records its provenance (object type, id, changelog id) so `show` explains where it came from |
| D-11 | One NetBox change set becomes one Ze config transaction (verify/apply/rollback across components), so a customer service is provisioned completely or not at all. A reference to a Ze template that does not exist fails verify; the last good config stays; an alert is raised |
| D-12 | (coordinator addition) The spec is written so a later agent, given a URL and a read-only v2 token, completes it mechanically: survey rows with exact GET queries, design sections marked with the survey ID they depend on, YANG defaults the survey can only confirm or override |
| D-13 | (2026-10-08, second round) No netbox-bgp. Thomas: "let's document how netbox should be setup part of the work to be done and not require an extension which can get out of sync". Ze depends on core NetBox only. BGP sessions are modelled with Ze-defined custom fields in core NetBox (see "BGP in core NetBox") |
| D-14 | NetBox setup documentation is part of the work: a page states exactly how to set up NetBox for Ze (custom fields, choice set, service marker, service user, read-only v2 token, object permissions, event rule and signed webhook). Ze never writes NetBox; the operator applies the setup |
| D-15 | Thomas: "hold large removal should be a per system option value". One system-level shrink threshold applies to every source (IRR consumers and NetBox alike), not per source and not per group |
| D-16 | Approved as recommended: Q-1 (one shared apply mechanism, IRR spec amended before implementation), Q-2 (six children, child 3 now custom fields), Q-4 (no IKE pre-shared keys in v1), Q-8 (token in config only), Q-9 (webhook on the hub web server), Q-10 (fleet-2 redesigned as tree composition, recorded only), Q-11 (add a NetBox field, never parse names), Q-12 (no Branching / Custom Objects skeletons now), Q-13 (standalone router out of scope). VPN stays on NetBox's core VPN models |
| D-17 | SUPERSEDED by D-19 (2026-10-08, fifth round). (2026-10-08, third round) Thomas: "we should have default which are netbox-bgp compatible". Reading taken (main thread, recorded here): Ze does NOT require netbox-bgp and does not depend on it, but the DEFAULT BGP mapping matches netbox-bgp's model, so a NetBox with netbox-bgp installed works with no Ze mapping config. A plain NetBox follows the documented custom-field setup, selected by overriding the mapping in Ze config. One reader, never two implementations (`ai/rules/no-layering.md`): the builder reads a session record of logical fields; the per-group mapping names the endpoint and the field path of each logical field. The custom-field names mirror netbox-bgp's field names, so moving between the two is a rename, not a remodel. This supersedes D-13 where they differ |
| D-18 | (2026-10-08, fourth round) Thomas "agree" to Q-14..Q-18: shrink threshold absent = off, a warning when a NetBox source has none, the guide recommends 20; one IP address = one session in the custom-field model for v1; `ze_service` custom field (renamed `service` by D-20) as the service marker (tag as the alternative); a documented maintenance procedure for switching a group between models, no id translation; free dotted field paths checked by pattern. Partly moot after D-19: with one model there is nothing to switch (Q-17, R-14 withdrawn), and the one-router-per-address limit lost its netbox-bgp answer and is reopened as Q-21 |
| D-19 | (2026-10-08, fifth round) Thomas: "We are not going to use netbox-bgp, so we are not bound by any of its conventions and can decide how to label things." Supersedes D-17 and every "netbox-bgp-compatible default". Reading taken (main thread): one BGP model, Ze's own, on core NetBox only (the D-13 direction). No preset registry, no `model` leaf, no group endpoint, no netbox-bgp refusal lists. Ze chooses the labels (D-20 sets the rule for them). The per-group `field <logical-field>` overrides stay (D-2, Q-18). Q-19 closes with this decision |
| D-20 | (2026-10-08, fifth round, owner correction) NetBox-side labels are vendor-neutral: custom fields and choice sets carry plain descriptive names with no `ze_` prefix (`peer_group`, `remote_as`, `local_as`, `devices`, choice set `bgp_peer_groups`, `interface_template`, `service`), so the same NetBox data serves another export mechanism later (another config generator, Ansible). Ze-specific names exist only on the Ze side (YANG, `nb-<id>` keys). This renames the earlier `ze_service` (D-18, Q-16) and `ze_template` defaults. An operator whose NetBox already uses other names maps them with the per-group `field` leaves and the selector `custom-field` leaves |
| D-21 | (2026-10-08, fifth round) Thomas: the local address is derived, never stored on the peer's record. An eBGP session MUST use the router's own address on the local interface of the point-to-point link to the peer (the device's interface address in the same prefix as the remote address). An iBGP session MUST use a dedicated BGP loopback of the router as local address (update-source) for every outgoing connection: one loopback per router, shared by every iBGP session. Reason (paraphrased): with the point-to-point interface passive in the IGP its prefix is known network-wide even without next-hop-self on iBGP; for iBGP, one group with one source address is impossible with a point-to-point address, and tying iBGP to one link would drop every iBGP session when that link goes down. Settles Q-21. Reading taken here: iBGP when the remote AS equals the group's `session asn local`; the loopback is the device's address with NetBox's built-in IPAddress role `loopback`, exactly one per family. That selection is superseded by D-22: a per-family tag on an interface selects the iBGP source, never the `loopback` role |
| D-22 | (2026-10-08, sixth round, closes Q-22) Thomas: "we need to tag the loopback or IP to use; I gave you best practice so you can ensure examples, documentation, etc. use it." Refined the same day: the tag goes on the INTERFACE, not on the IP address, and there is one tag PER ADDRESS FAMILY. Rule: two Ze-side settings name the tags, with vendor-neutral defaults `bgp-source-ipv4` and `bgp-source-ipv6` (no `ze_` prefix, D-20), declared once in the BGP mapping-defaults record and overridable per group like the other record values. For an iBGP peer of family F, exactly one interface of the device carries F's tag, and the local address is that interface's single address in F. The two tags may sit on one interface or on two. A family with no iBGP record needs no tag. No tagged interface, several, or a tagged interface with zero or several addresses in F is a build error naming the device and the family, raised only when the client has an iBGP record in F. The `loopback` role selects nothing. Best practice, which every example, the setup page and `show netbox setup` show (D-21 is the rule, D-22 makes it the documented norm): eBGP on the point-to-point interface address, that interface passive in the IGP; iBGP from a dedicated loopback interface that carries the tags. Thomas's reason: the point-to-point prefix is known through the IGP without next-hop-self; one iBGP source is shared by every session; an L2 failure must not drop every iBGP session. `netbox-setup-drift` warns, never errors, when a tagged interface is not of NetBox type `virtual` (NetBox has no loopback interface type, External sources) |

## Split (approved, Q-2 in D-16)

One umbrella plus six children. The reason for splitting: the six parts have different
dependencies, different files and different test infrastructure, and the BGP mapping is useful
on its own long before interfaces and VPN are.

| Child | Path (planned, not yet written) | Owns | Depends on |
|-------|-------------------------------|------|------------|
| 1 | `plan/spec-netbox-1-source.md` | `netbox source` YANG (URL, token, TLS, refresh, notification secret), the commands `show netbox` and `update netbox refresh`, the REST client (GET only, pagination, v2 bearer), the mock NetBox server, the "pull now" endpoint with HMAC check, changelog catch-up, unreachable handling and its warning, reachability doctor check; the setup framework (D-14): the setup-definition registry each mapper adds to, `show netbox setup`, the `netbox-setup-drift` doctor check, and the first version of `docs/guide/netbox.md` (service user, token, permissions, event rule, webhook) | - |
| 2 | `plan/spec-netbox-2-builder.md` | the hub-side builder: the `netbox client <c>` binding, device lookup and per-client fetch, mapper registry, composition of operator source + generated contribution into the served config, the served-config provider the hub's `ReadConfig` reaches by registration ("Served-config provider"), the one strip point that removes template-side selectors from every client config the hub serves, bound or not, hub-side verify (the only verify template-side config meets on the hub, so its refusals are build refusals), last good, the generic `generated` provenance grouping (`ze-generated.yang` and its `LoadEmbedded` entry) with its `ze:generated` extension, the generated-key prefix registry and the schema-driven refusals, editor refusal on the router, the router reload backstop (Provenance, "Backstop"), apply-policy integration (held, apply, dismiss; `netbox source <s> apply` verified by the shared validation of `internal/core/applypolicy`; shrink read from the system-level threshold, D-15), `config-ack` failure warning, the generic derived-state sweep at start and at reload acceptance with its settle-and-sweep ordering (AC-61 in unit tests; its `.ci`, `netbox-root-removed-sweeps-state.ci`, needs child 3's mapper), the commands `show netbox client`, `show netbox held`, `update netbox apply` and `clear netbox held`, and metrics | 1, `internal/core/applypolicy` (from `spec-irr-apply-policy`) |
| 3 | `plan/spec-netbox-3-bgp.md` | the session-record reader over core NetBox IP addresses (one model, Ze's own, D-19) and its one record of mapping defaults, the per-device local-address derivation (D-21), the `bgp group <g> netbox` augment, `uses generated` on the bgp `peer`, `bgp` in the plugin's `ConfigReads` and the refusal of a selector in the hub's own config, the refusal rules, its setup definitions and its section of the setup page; the first end-to-end proof of child 2's builder through a real mapper | 2 |
| 4 | `plan/spec-netbox-4-interface.md` | a unit template in `ze-iface-conf.yang` (Ze has none today), interface/sub-interface/VLAN/IP mapping, refusal of NetBox modes Ze cannot express, `uses generated` on the unit, `interface` in `ConfigReads` and the hub-own-config selector refusal | 2, 3 (AC-18 generates a BGP session beside a unit) |
| 5 | `plan/spec-netbox-5-vpn.md` | a site-to-site template in `ze-ipsec-conf.yang` (Ze has none today), Tunnel -> site-to-site peer, IPSecProfile -> generated ike-group and esp-group bodies, `uses generated` on the site-to-site peer, ike-group and esp-group, `vpn` in `ConfigReads` and the hub-own-config selector refusal | 2 |
| 6 | `plan/spec-netbox-6-service.md` | customer-service grouping by tag or custom field, completeness rule, service-level hold | 3, 4 (5 optional) |

Future work the owner already placed out of scope (D-7, D-8): Branching pre-merge validation,
Custom Objects support. Not specs yet; the owner decides whether they become skeletons (Q-12).

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/core-design.md` section 22 - sources, builders, read views
  → Decision: the hub is the builder, the served per-client config is the read view, the router is the consumer that pulls. The builder is the only component that knows NetBox's data model, so every NetBox type name lives in the netbox plugin and its mappers, never in bgp/iface/ipsec code.
  → Constraint: "A pull that fails keeps the last good read view" applies twice: a failed NetBox pull keeps the last applied generated contribution, and a failed composition or hub verify keeps the last served config.
  → Constraint: the sentence "Ze has no NetBox builder today." must be replaced in the same piece of work that lands the builder (child 2), per `ai/rules/documentation.md`. This spec does not edit core-design.md.
- [ ] `docs/architecture/core-design.md` section 18 and `docs/architecture/config/transaction-protocol.md` - verify/apply/rollback
  → Decision: D-11 is met on the router by the existing path: a fetched config is one reload, which is one transaction (verify all, apply all, rollback all). The builder never splits one change set over two served versions.
  → Constraint: verify on the router runs plugin verify handlers the hub cannot run, so the hub check is necessary but not sufficient; a router-side refusal arrives as `config-ack ok:false` and must raise a hub warning (today it is only logged, `managed_serve.go` handleRequest `VerbConfigAck`).
- [ ] `docs/architecture/fleet-config.md` - hub, managed client, config-fetch, config-changed
  → Constraint: the hub stores each client's config as one blob at `file/active/client-<name>.conf` (`ClientConfigKey`); `HandleConfigFetch` hashes whatever `ReadConfig` returns (`fleet.VersionHash`), and `SetWriteObserver` maps a written client key to a `config-changed` notify. The served config changes only through what `ReadConfig` returns and a notify.
  → Constraint: the client name is taken only from the authenticated session (`handleConn` -> `pluginipc.AuthenticateWithLookup`; comment "a client can only fetch its own config"), one connection per name, and `config-fetch` serves `HandleConfigFetch(name, ...)`. Verified 2026-10-08 in `internal/component/plugin/server/managed_serve.go`. Per-router isolation (D-9) therefore needs no new mechanism: the builder writes a generated config per client name and the existing fetch serves only that name's.
  → Constraint: Non-Goals "Incremental config updates": full config on change. The builder publishes a whole served config per client, never a delta.
- [ ] `docs/architecture/config/syntax.md`, `docs/guide/config-editor.md` - authorship metadata, blame, inactive nodes
  → Decision: provenance is NOT carried in `MetaEntry` (`internal/component/config/meta.go`): its fields are an edit-session shape (`User`, `Source`, `Time`, `Previous`) and an object type, id and changelog id would be packed into strings (`ai/rules/go-standards.md` typed-vs-string). Provenance is real config data in a generic `generated` container on each generated entry (see "Provenance and ownership").
  → Decision: NetBox `offline` maps to the existing inactive mechanism (`Tree.SetInactive`), so a deactivated generated peer stays in config, keyed and explained, with no new leaf.
- [ ] `docs/guide/bgp-peering.md` "Groups and inheritance", `internal/component/bgp/config/resolve.go`
  → Decision: generated peers are written as `peer` entries under the Ze group named by the mapping; inheritance (`deepMergeMaps`, group then peer) does the templating. No new BGP template concept.
  → Constraint: `validatePeerName` refuses a name starting `dyn-` ("prefix is reserved for dynamic peers", a literal in `resolve.go`). The generated key prefix `nb-` MUST NOT be added there: a NetBox name in bgp code breaks plugin ownership (`ai/rules/plugins.md`). It is reserved by generic config validation driven by the `ze:generated` extension, with the prefix registered by the netbox plugin (see "Provenance and ownership").
  → Constraint: the provenance container and `description` stay out of `PeerSettings` (`internal/component/bgp/reactor/peer_settings.go`, which has no description field today): `peerSettingsRestartReason` (`peer_settings_apply.go`) restarts the session for any field outside `hotSwappableSettings`, so a provenance field there would flap the session on every change id and break AC-10.
  → Constraint: a dynamic group is a group whose `connection remote ip` is `dynamic` (`isDynamicGroup`); it has no static members, so a group whose remote is `dynamic` cannot carry a `netbox` selector (verify refuses the pair).
  → Decision: prefix limits, policies and max-prefix are template body in the Ze group (`family <f> prefix maximum`, filters, policy); NetBox carries no field for them (D-6, D-13).
- [ ] `internal/component/iface/yang/ze-iface-conf.yang`
  → Constraint: Ze has no interface template or group. A unit (`grouping interface-unit`, `list unit`) carries `vlan-id`, `description`, `disable`, `vrf` (refused: "Not implemented"), `sysctl-profile`, `ipv4 address` and `ipv6 address` leaf-lists. A unit has no `mtu` leaf; `mtu` exists at interface level only. A bridge has a `member` leaf-list; there is no VLAN filtering, access/trunk mode or 802.1ad (QinQ) model. D-3 for interfaces therefore needs child 4 to add a unit template.
- [ ] `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang`
  → Constraint: `vpn ipsec` has `ike-group` (key-exchange ikev1/ikev2, lifetime, dead-peer-detection, `proposal` with encryption, hash, dh-group), `esp-group` (lifetime, pfs enable/disable, `proposal` with encryption, hash), and `site-to-site peer` (ike-group, esp-group, connection-type, local-address, remote-address, authentication mode pre-shared-secret/x509/eap-*, traffic-selector, vti). Encryption enum: aes128, aes256, aes128gcm, aes256gcm, aes*ccm*, chacha20poly1305, 3des. Hash enum: sha1, sha256, sha384, sha512. No site-to-site template exists; D-3 for VPN needs child 5 to add one.
  → Constraint: `esp-group pfs` is enable/disable only. On a PFS rekey Ze uses the Diffie-Hellman group the IKE SA negotiated (`childRekeyDHGroup` in `internal/component/ike/engine/rekey.go`: "The group is the one the IKE SA negotiated. An ESP proposal carries no group of its own in Ze's data model"). A NetBox `pfs_group` can therefore be honoured only when it equals that group.
- [ ] `plan/spec-irr-apply-policy.md` (status ready) - immediate/delay/scheduled/operator, shrink threshold, held/apply/dismiss commands
  → Decision: NetBox reuses the same policy, not a copy (`ai/rules/principles.md`, every fact once). See "Apply policy: one mechanism".
  → Constraint: that spec names the shared locations, and this spec only points to them: the decision and the window check in `internal/core/applypolicy`, the `apply` grouping in `internal/component/config/yang/modules/ze-apply-policy.yang`, and `system apply-policy shrink-threshold` in `ze-system-conf.yang`, all added by that spec. The verbs are `update ... apply`, `clear ... held`, `show ... held`; the alert is `report.RaiseWarning` / `ClearWarning` plus a Prometheus gauge.
- [ ] `plan/spec-fleet-2-config-templates.md` (skeleton), `plan/spec-fleet-5-staged-rollout.md` (skeleton), `plan/spec-fleet-7-config-reconnect-resolution.md` (skeleton)
  → Constraint: fleet-2 plans hub-side rendering inside `HandleConfigFetch` with Go `text/template` and "No external data sources". Two hub-side producers of the served config would be two declarations of one fact; the served config must have one composition point that both feed (R-9, Q-10): the served-config provider ("Served-config provider").
  → Constraint: fleet-7 "Adopt (router wins)" copies a router's config into the hub's authoritative copy. Adopting must never turn a NetBox-owned entry into an operator-authored one (R-10).
- [ ] `ai/rules/config.md`, `ai/patterns/config-option.md`
  → Constraint: config is manipulated only as a parsed YANG tree or as `set` lines; the builder composes trees, never text. Every new node has `ze:help` (96 chars, 25 words) and a different `description`. Leaf names spelled in full.
  → Constraint: secrets are `ze:sensitive` leaves (precedent: `ze-ddos-flowtriq-conf.yang` `api-key`, a Bearer token). `ze:sensitive` only replaces the value with a placeholder in display output (`ze-extensions.yang`); Ze does not encode it, so a token typed into the editor is written to the hub's config file as typed, and only a `$9$` value the operator wrote by hand is decoded on read (the flowtriq `api-key` description). The token is therefore in the clear at rest on the hub (Security Review Checklist, Q-20). YANG constraints judge a different form on each input path: the block parser decodes a `$9$` value on a sensitive leaf before `ValidateLeafValue` (`parser.go`), while a set line passes the raw token to `ValidateLeafValue` with no decode (`setparser.go`, `setparser_meta.go`), and a value stored encoded reaches plugin verify still encoded. So no sensitive leaf of this set carries a YANG `pattern` or `length`: the netbox plugin's config verify decodes the value itself (`secret.IsEncoded` then `secret.Decode`, as `parsePreSharedSecret` in `internal/component/ike/ipsec/config.go` does) and then checks the token's `nbt_` prefix and the notification secret's length 16..256.
- [ ] `docs/architecture/config/yang-config-design.md` - YANG layering: format in YANG, behavior declared by extensions in `ze-extensions.yang`, executed in Go
  → Decision: the ownership refusal is declared by a new extension `ze:generated` on the generic provenance container, so the editor and commit validation find owned entries from the schema. The extension is added to `internal/component/config/yang/modules/ze-extensions.yang` and to the page's extension table in the same change ("An extension that is absent here is a defect of this page"). Precedent for a schema walk driven by an extension: `CheckRequired` in `internal/component/config/required.go` (`ze:required`).
  → Constraint: the provenance grouping is generic (it names no NetBox type) and reaches routers, so it lives in a grouping-only module of the config component, `internal/component/config/yang/modules/ze-generated.yang` (beside `ze-types.yang`), `uses`d by the BGP peer (child 3), the iface unit (child 4) and the ipsec site-to-site peer, ike-group and esp-group (child 5). `LoadEmbedded` in `internal/component/config/yang/loader.go` lists the embedded modules by path (today `ze-extensions.yang` and `ze-types.yang`), so child 2 adds `ze-generated.yang` there, and the page's "`LoadEmbedded()` loads the two foundation modules" sentence with it.
- [ ] `internal/component/bgp/plugins/role/yang/ze-role.yang` - the augment pattern: a plugin module imports `ze-bgp-conf` and augments `/bgp:bgp/bgp:group` and `/bgp:bgp/bgp:group/bgp:peer`
  → Decision: the template-side `netbox` selector containers are augments declared in the netbox plugin's own modules (`internal/plugins/netbox/yang/`), never added to `ze-bgp-conf.yang`, `ze-iface-conf.yang` or `ze-ipsec-conf.yang`. With the gate off the augments do not exist and a config naming them fails to parse (an unknown key), rather than being accepted and inert (AC-49).
  → Decision: with the gate on, the augments also exist in the hub's own schema, where a selector would be accepted and inert (the builder reads client configs, never the hub's own tree). The netbox plugin's verify refuses a `netbox` selector in the hub's own config naming the template (AC-50); to see it, each mapper child adds its root to the plugin's `ConfigReads` (`bgp` child 3, `interface` child 4, `vpn` child 5).
- [ ] `ai/rules/plugins.md` - plugin owns its whole surface; registration over switch dispatch
  → Constraint: removing the netbox plugin (feature gate off) MUST make every NetBox feature disappear while bgp/iface/ipsec keep working: the selector augments vanish with the plugin, the provenance grouping is generic data, the editor refusal is driven by the extension, and the reserved key prefix is registered by the plugin, never written in bgp, iface or ipsec code.
- [ ] `internal/component/plugin/registry/registry.go` `ConfigReads` - "roots the plugin reads but does not own" (`bgp-rpki` reads `pki`)
  → Decision: the netbox plugin lists `system` in `ConfigReads` to read `system apply-policy shrink-threshold` (A-14), and the mapper children add `bgp`, `interface` and `vpn` (AC-50). The client binding sits under the plugin's own `netbox` root (`netbox client <c>`), never as an augment of `plugin hub server <s> client <c>`: `ConfigRoots` plus `ConfigReads` are all the verifier and schema see (`registry.go`, the `ConfigReads` comment), and reading `plugin` would hand the netbox plugin every plugin's config.
  → Constraint: `show/update/clear netbox ...` dispatch registers handlers; no switch over subcommands; mappers register with the builder.
- [ ] `internal/component/web/webroute.go`
  → Constraint: `WebRoute.Wrap` has two kinds, `WrapAuth` (session) and `WrapMutation` (session plus same-origin). A NetBox webhook carries no session; it authenticates by HMAC. Child 1 adds a third kind whose handler authenticates the request itself (Q-9 names the alternative, a dedicated listener).
- [ ] `internal/test/mock/peeringdb/` - deterministic HTTP mock server for tests
  → Decision: child 1 adds `internal/test/mock/netbox` in the same shape, serving canned REST answers and the changelog, and sending signed webhooks on demand.
- [ ] `test/managed/*.ci` (15 files, for example `config-push-transactional.ci`) - hub plus managed client driven by `le test fixture managed/...`
  → Decision: the end-to-end tests extend that fixture: hub + managed client + mock NetBox.

### External sources (re-verified 2026-10-08 at pinned tags)

NetBox paths are relative to `https://raw.githubusercontent.com/netbox-community/netbox/v4.6.10/`
(the latest 4.6 tag on 2026-10-08). The two netbox-bgp rows this table carried were removed on
2026-10-08 with D-19: Ze no longer reads that plugin's model.

| Fact | Source | Used for |
|------|--------|----------|
| Custom field types include Object, Multiple object, Selection and Multiple selection; a Related Object Filter takes a `query_params` JSON dict. The page says "Each custom selection field must designate a choice set containing at least two choices", but the model's `CustomFieldChoiceSet.clean` refuses only an empty set ("Must define base or extra choices."), so a one-choice set is accepted by the code; the page does not document REST filtering by custom field (`cf_` prefix) | `docs/customization/custom-fields.md` line 79; `netbox/extras/models/customfields.py` line 1019 | the BGP model (D-19); one Ze group gives a one-choice set (A-16); the `cf_` filter is the next row, and S-15 confirms it on the owner's NetBox |
| Every custom field gets a REST filter named `cf_<name>` unless its `filter_logic` is `disabled` (default `loose`). A Multiple object field is stored as a list of primary keys, and its filter is a number filter with lookup `contains`, so `cf_<name>=<id>` selects the objects whose list holds that id. Verified in the source; the REST answer shape of a multi-object value (a list of nested objects) is not, and stays S-15 | `netbox/netbox/filtersets.py` lines 332-339; `netbox/extras/models/customfields.py` lines 176-182 (`filter_logic`), 524-525 (`serialize`), 784-786 (`to_filter`) | the `cf_devices` device filter; the setup page requires `filter_logic` not `disabled` |
| IKEProposal (authentication_method, encryption_algorithm, authentication_algorithm, group, sa_lifetime), IKEPolicy (version, mode, proposals, preshared_key; no device), IPSecProposal (encryption_algorithm, authentication_algorithm, sa_lifetime_seconds, sa_lifetime_data), IPSecPolicy (proposals, pfs_group), IPSecProfile (mode, ike_policy, ipsec_policy) | `netbox/vpn/models/crypto.py` | VPN mapping table; A-15 |
| Encryption aes-128-cbc, aes-128-gcm, aes-192-cbc, aes-192-gcm, aes-256-cbc, aes-256-gcm, 3des-cbc, des-cbc; authentication hmac-sha1/256/384/512, hmac-md5; tunnel encapsulation gre, ipsec-transport, ipsec-tunnel, ip-ip, l2tp, openvpn, pptp, wireguard; termination role peer, hub, spoke; tunnel status planned, active, disabled | `netbox/vpn/choices.py` | VPN mapping and refusals; S-7 compares the installed version |
| IP address roles are loopback, secondary, anycast, vip, vrrp, hsrp, glbp, carp; `IPAddressFilterSet` filters `role` (a multiple-choice filter over those values) and `family` | `netbox/ipam/choices.py` lines 77-97 (`IPAddressRoleChoices`); `netbox/ipam/filtersets.py` lines 602-604, 711-714 | not used for selection since D-22: the iBGP source is chosen by interface tag, not by role |
| Every `NetBoxModelFilterSet` has `tag` (`TagFilter`, matched on the tag's `slug`; several `tag=` values are ANDed); `InterfaceFilterSet` inherits it through `DeviceComponentFilterSet`, which also declares `device_id`. A slug that names no tag is not a valid choice of the filter | `netbox/netbox/filtersets.py` lines 318, 326-327; `netbox/extras/filters.py` lines 109-117; `netbox/dcim/filtersets.py` lines 1757, 1827, 1907, 2214 | the per-family tagged-interface GET `/api/dcim/interfaces/?device_id=<id>&tag=<tag>` (D-22); the setup page requires both tags to exist |
| `IPAddressFilterSet.interface_id` selects the addresses assigned to the given interfaces | `netbox/ipam/filtersets.py` lines 677-681 | the tagged interface's addresses, `/api/ipam/ip-addresses/?interface_id=<id>` (D-22) |
| Interface types: NetBox has no loopback type; a loopback is modeled as type `virtual` (group "Virtual interfaces", with `bridge` and `lag`); `InterfaceFilterSet` filters `type` and `kind` (`virtual` = every virtual type) | `netbox/dcim/choices.py` lines 889-892, 1172-1178; `netbox/dcim/filtersets.py` lines 2240-2241, 2293, 2364-2370 | the drift check's best-practice warning: a tagged interface whose `type` is not `virtual` (D-22); S-16 confirms the owner's loopbacks are typed `virtual` |
| IP address statuses include active, reserved, deprecated | `netbox/ipam/choices.py` | the default `status-active` / `status-inactive` values |
| `IPAddressFilterSet.device_id` (`method='filter_device'`) selects the addresses assigned to the interfaces of the device, its virtual-chassis interfaces included (`device.vc_interfaces()`) | `netbox/ipam/filtersets.py` lines 656-660, 794-803 | the per-device local-address derivation ("Fetch per client"); S-15 |
| Every object serializer carries `url` (REST detail) and `display_url` (the UI detail view) | `netbox/netbox/api/serializers/base.py` line 20, `netbox/netbox/api/serializers/fields.py` lines 45-54 | provenance `url` (F-3); S-15 confirms it on IP address records |
| v2 tokens: prefix `nbt_`, sent as `Authorization: Bearer nbt_<key>.<token>`; fields `version` (default 2), `write_enabled`, `expires`, `allowed_ips` | `netbox/users/constants.py` line 19, `netbox/netbox/api/authentication.py`, `netbox/users/models/tokens.py` | token leaf, A-2 |
| Webhook body keys event, timestamp, object_type, data, request, snapshots; `secret` adds `X-Hook-Signature`, the hex HMAC-SHA512 of the body | `docs/integrations/webhooks.md`; `docs/models/extras/webhook.md` line 73 | notification endpoint |
| Rendering a device's config template is "a POST request" | `docs/features/configuration-rendering.md` line 52 | Key Design Decisions (render-config not used) |
| Object-permission constraints are Django query filters over the object's own fields | `docs/administration/permissions.md` lines 30-32 | A-15 |
| GraphQL is read-only; lists are `<object>_list` with `pagination: {limit: N}`; the documented call sends the query as a request body | `docs/integrations/graphql-api.md` lines 3, 46, 134 | S-11 |
| Research brief (coordinator, 2026-10-08), not verified at the tag: interface modes access/tagged/tagged-all/q-in-q; v1 tokens deprecated in 4.6; changelog filterable by time_after, request_id, changed_object_type, related_object | coordinator brief | claims only: S-6 (modes), S-2 (token), S-10 and A-4 (changelog) |

**Key insights:** (minimal context to resume)
- The hub already isolates routers (name from auth session). The builder is a hub-side producer of each client's served config; routers never see NetBox.
- Templates are Ze config: BGP groups exist; interface and VPN templates do not and are added by children 4 and 5.
- Generated entries carry key `nb-<netbox-id>` and a generic `generated` provenance container; the editor refuses edits under such entries.
- The served config is composed from the operator source (`client-<name>.conf`) and the applied generated contribution, verified on the hub, kept last-good, and published under the apply policy shared with the IRR spec.
- REST GET with a read-only v2 token is the only access; GraphQL is a later optimisation once S-11 confirms a read-only token can query it.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/plugin/server/managed_serve.go` - managed listener; `handleConn` authenticates and binds the name; `handleRequest` serves `config-fetch` (`HandleConfigFetch`), logs `config-ack`; `NotifyConfigChanged` queues a notify; `ClientConfigKey` / `ClientNameFromConfigKey`
- [ ] `internal/component/plugin/server/managed.go` - `HandleConfigFetch` reads, hashes, answers `current` or the base64 config
- [ ] `cmd/ze/hub/managed_server.go` - wires `ReadConfig` to `store.ReadFile(ClientConfigKey(name))` and `SetWriteObserver` to `NotifyConfigChanged`
- [ ] `internal/component/managed/handler.go` - client side: `OnFetch` on `config-changed`, `ValidateConfig` decodes and validates; commit happens through `ClientConfig.OnCommit`
- [ ] `internal/component/bgp/config/resolve.go` - `ResolveBGPTree`, group/peer deep merge, dynamic groups, `validatePeerName` (`dyn-` reserved), `validateGroupName`
- [ ] `internal/component/bgp/yang/ze-bgp-conf.yang` - `list group` with `uses peer-fields` and `list peer` keyed by name
- [ ] `internal/component/iface/yang/ze-iface-conf.yang`, `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` - see Required Reading
- [ ] `internal/component/config/meta.go`, `serialize_blame.go`, `tree.go` - authorship metadata and inactive nodes
- [ ] `internal/component/cli/editor_commands.go` (`Editor.SetValue`, `Editor.DeleteValue`), `internal/component/web/editor.go` (`EditorManager.SetValue` delegates to the user's `cli.Editor`)
- [ ] `internal/core/report/report.go` - `RaiseWarning`, `ClearWarning`
- [ ] `internal/core/diagnostic/doctor_registry.go` - `RegisterDoctorCheck`

**Behavior to preserve:**
- A managed client with no `netbox client <c>` entry and no `netbox` selector in its `client-<name>.conf` is served exactly `client-<name>.conf`, byte for byte, as today; its version hash is unchanged. A hub built without the netbox gate serves every client exactly as today.
- The managed protocol (four verbs, `pkg/fleet/envelope.go`) is unchanged; no new verb.
- BGP group inheritance semantics are unchanged; a generated peer is an ordinary peer to the reactor.
- A client can only fetch its own config.

**Behavior to change:**
- A NetBox-enabled client is served the builder's last good composed config.
- A client that is not bound but whose `client-<name>.conf` carries `netbox` selectors is served that config with the selectors stripped, and the hub raises warning `netbox-unbound-selectors` naming the client.
- `config-ack ok:false` raises a hub warning in addition to the log line.
- The router's editor refuses edits under generated entries.
- Operator-authored keys starting with a registered generated-key prefix (`nb-`, registered by the netbox plugin) are refused, by generic config validation, in every list whose entries use the `generated` grouping.

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- NetBox REST API answers (JSON over HTTPS), pulled by the hub with a read-only v2 token.
- A NetBox webhook POST to the hub (signed), meaning only "pull now".
- A write of `client-<name>.conf` into the hub store (changes the template side). Its writers today are `ze data import` (offline) and `request data restore ... client` (`restoreClientConfig` in `internal/component/config/storage/cli/data_rpc.go`); neither runs YANG or plugin verify on the client config, and this set adds no validating write path. The write observer reaches the builder, so every refusal of template-side config is a BUILD refusal: warning `netbox-build-failed`, served config unchanged, last good kept (AC-12, AC-34, AC-43).

### Transformation Path
1. Trigger: periodic catch-up timer, a verified webhook, an operator `update netbox refresh`, a write of a NetBox-enabled `client-<name>.conf` (seen by `SetWriteObserver`), or hub start.
2. Change discovery (child 1): read the changelog since the stored cursor; map changed objects to the affected clients; with no cursor, an expired cursor or at start, every NetBox-enabled client is affected.
3. Fetch (child 1): for each affected client, GET the objects of its NetBox device (S-4, S-14) per registered mapper, paginated.
4. Map (children 3-5): each registered mapper turns its objects into `set`-line equivalent tree nodes under the Ze template the client's config names, keyed `nb-<id>`, each with a `generated` provenance container; a refused object fails the whole build for that client with a message naming the object.
5. Compose (child 2): parse `client-<name>.conf` into a YANG tree, merge the generated tree into it (tree merge, never text), remove every template-side `netbox` selector container (hub builder input that routers never receive), serialize.
6. Hub verify (child 2): schema validation (YANG patterns and ranges, the generic generated-entry validation), each mapper's selector verify (a dynamic group with a selector), plus the offline validators (`ResolveBGPTree` and the iface/ipsec equivalents); a reference to a missing Ze template fails here (D-11). Any refusal fails the client's build.
7. Apply policy (child 2, shared mechanism): compare the candidate generated contribution with the applied one; immediate, delayed, window or operator; shrink threshold. A held candidate is stored, not served.
8. Publish: store the composed config as the client's served config (key `meta/netbox/served/<client>`) and notify (`NotifyConfigChanged`, through the notify function the provider registration receives).
9. Router: on `config-changed`, fetches when ready, verifies and applies as one transaction, acks. A refusal keeps its running config and raises a hub warning.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| NetBox -> hub | HTTPS GET, `Authorization: Bearer nbt_...`, JSON | No (S-2, S-11 settle it against the owner's NetBox) |
| NetBox -> hub (notify) | HTTPS POST, `X-Hook-Signature` HMAC-SHA512 | No |
| netbox plugin -> mappers | mapper registry (registration in each mapper's `register.go`) | No |
| builder -> hub storage | served config blob and generated state through `statestore` / storage under registered keys | No |
| hub -> router | existing `config-changed` / `config-fetch` | Yes (`managed_serve.go`) |

### Integration Points
- `ManagedServerConfig.ReadConfig` (`cmd/ze/hub/managed_server.go`, today `store.ReadFile(pluginserver.ClientConfigKey(name))`) - asks the registered served-config provider first ("Served-config provider"); with none registered, or when the provider declines the client, it reads `client-<name>.conf` as today.
- `Storage.SetWriteObserver` - unchanged: a write of `client-<name>.conf` notifies the client, and the provider sees the write through the builder's trigger (Transformation Path step 1). A publish, a bind and an unbind notify through the provider's notify function, never through a second key mapping.
- `cli.Editor.SetValue` / `DeleteValue` - the ownership refusal (shared by CLI and web; MCP to be confirmed, A-9).
- `report.RaiseWarning` / `ClearWarning`, the plugin metrics registry, `diagnostic.RegisterDoctorCheck`.
- The apply-policy mechanism shared with `spec-irr-apply-policy`.

### Architectural Verification
| Check | Holds? | Evidence |
|-------|--------|----------|
| No bypassed layers (data flows through the intended path) | Yes (design) | the router receives NetBox data only through the existing fetch and applies it through the existing reload transaction |
| No unintended coupling (components stay isolated) | Yes (design) | NetBox types live only in `internal/plugins/netbox` and its mapper sub-packages; bgp/iface/ipsec gain only YANG templates and `uses generated`; the selectors are augments from the plugin and the `nb-` prefix is registered by the plugin |
| No duplicated functionality (extends existing, does not recreate) | Yes (design) | group inheritance, managed fetch, transaction protocol, report bus, apply policy are reused; R-9 names the fleet-2 overlap to resolve |
| Zero-copy preserved where applicable (refs, not copies) | N-A | control-plane config build, not a wire path |
| Registration over hardcoding, outbound | Yes (design) | each mapper registers the NetBox object kinds it reads and the Ze list it writes; the builder iterates the registry |
| Registration over hardcoding, inbound | Yes (design) | the editor refusal and the reserved-prefix refusal are schema-driven: generic config validation finds every list whose entries carry the `ze:generated` container and refuses an operator-authored key starting with a prefix registered by a builder (`nb-` from `internal/plugins/netbox/register.go`); no bgp, iface or ipsec file names NetBox. Lists to check at implementation, none to be edited for NetBox: `reservedPeerNames`, `validatePeerName`, unit-name validators, `./le config claims` exceptions |

## Design

### Placement

| Package | Kind | Why |
|---------|------|-----|
| `internal/plugins/netbox` | system plugin, feature-gated (`feature-gates.txt` row, tag to be chosen) | an engine nothing else depends on (`ai/rules/architecture.md`); runs on the hub |
| `internal/plugins/netbox/bgp`, `/iface`, `/vpn`, `/service` | mapper sub-packages, each with `register.go` | sub-plugins under the owner's namespace; each registers with the builder |
| `internal/plugins/netbox/yang/ze-netbox-conf.yang` | hub config under the plugin's own root: `netbox source <s>` and the client binding `netbox client <c>` | owned by the plugin; no hub module is augmented, and the plugin reads no `plugin` root |
| `internal/plugins/netbox/yang/ze-netbox-bgp-group.yang`, `ze-netbox-iface.yang`, `ze-netbox-vpn.yang` | template-side selectors: augments of `/bgp:bgp/bgp:group`, the iface unit template and the ipsec site-to-site template with a `netbox` container (`ze-role.yang` pattern) | vanish with the gate; stripped from every served config, bound or not, by the provider's one strip point |
| `internal/component/config/yang/modules/ze-generated.yang` | generic grouping-only module: the `generated` provenance container carrying the `ze:generated` extension | reaches routers; names no NetBox type |
| `internal/component/config` generated-entry validation (beside `required.go`) | the schema walk that refuses operator-authored provenance and registered key prefixes, and the prefix registry builders register into | generic, `CheckRequired` precedent |
| `internal/component/plugin/server` (beside `ClientConfigKey`) | the one-slot served-config provider registry ("Served-config provider") | always-on and already imported by `cmd/ze/hub`, so the hub reaches the gated plugin through registration, never an import |
| `internal/test/mock/netbox` | test mock | peeringdb mock shape |
| `internal/core/applypolicy`, `ze-apply-policy.yang` | named and created by `spec-irr-apply-policy` | one mechanism for IRR and NetBox |

### YANG: hub side (builder)

Defaults are chosen so the survey can only confirm or override them (D-12).

| Path | Type | Default | Meaning | Depends on |
|------|------|---------|---------|------------|
| `netbox source <name>` | list | - | one NetBox instance | |
| `netbox source <name> url` | string, `https://` URI | required | API base, for example `https://netbox.example.net` | S-12 |
| `netbox source <name> token` | string, `ze:sensitive` (display placeholder only; stored as typed, or as a hand-written `$9$` value, so no YANG pattern, Required Reading `ai/rules/config.md`) | required | v2 bearer token; must be read-only (D-9). Plugin verify decodes a `$9$` value, then refuses a value not starting `nbt_` (AC-1). Config only, no env var override (Q-8). In the clear at rest on the hub (Security Review Checklist, Q-20) | S-2 |
| `netbox source <name> tls ca` | leafref to `pki ca` | system CA pool | trust anchor for NetBox's certificate | S-12 |
| `netbox source <name> refresh-interval` | uint32 seconds, range 30..86400 | 300 | changelog catch-up period | S-10 |
| `netbox source <name> settle-time` | uint16 seconds, range 0..600 | 10 | coalescing window after the first trigger before a build starts (multi-request edits, R-4) | |
| `netbox source <name> notification secret` | string, `ze:sensitive`, no YANG `length`: plugin verify decodes a `$9$` value, then refuses a length outside 16..256 (AC-1) | absent: the endpoint refuses every notification for this source with the same 401 as a bad signature, so the answer reveals no configured source name | HMAC key matching the NetBox webhook `secret` | S-10 |
| `netbox source <name> apply` | `uses` the shared apply-policy grouping (mode, delay, window) | mode immediate, no delay | when a built change is published. Its cross-leaf refusals (`scheduled` needs both window leaves, `start` != `end`, `delay maximum` not below `minimum`) are the one validation function of `internal/core/applypolicy` that every consumer's verify calls (AC-58) | |
| `system apply-policy shrink-threshold` | uint8 percent, range 1..100, in `ze-system-conf.yang` | absent = off (Q-14) | ONE system-level value (D-15) read by every source: the IRR consumers and every NetBox client build. Not per source, not per group | |
| `netbox client <c>` | list keyed by the managed client name (the name of a `plugin hub server <s> client <c>` entry) | absent: client not NetBox-enabled | opt a managed client in. The plugin does not read the `plugin` root, so the name is matched at build time against the client configs in the hub store: a name with no `client-<c>.conf` fails that build with `netbox-build-failed` naming it | |
| `netbox client <c> source` | leafref to `netbox source` | required | the NetBox instance the client is built from | |
| `netbox client <c> device` | string | the client name | NetBox device (or VM) name | S-4, S-14 |
| `netbox client <c> kind` | enum device, virtual-machine | device | which NetBox model the name refers to | S-4 |

### YANG: template side (in `client-<name>.conf`, edited on the hub)

The `netbox` container exists only on templates (D-3) and its presence opts the template in. It
is an augment from the netbox plugin's modules (Placement), so with the gate off it does not
parse. The hub strips it when composing, so routers never receive it.

The BGP mapping defaults are DATA, not YANG defaults: YANG cannot configure a leaf-list as empty,
so a `status-inactive` default carried in YANG could never be overridden to "no status makes a
peer inactive". The BGP mapper (`internal/plugins/netbox/bgp`) holds ONE record of defaults:
every field path, the status values and the two iBGP source tags (D-22). The reader, `show netbox setup` and the
drift check all read that record, so each default is declared once. A per-group leaf, when
configured, replaces the record's value for that group; absent, the record's value applies. There
is one BGP model (D-19), so the record is a value: no registry holds it and nothing selects it by
name.

| Path | YANG default | Meaning | Depends on |
|------|--------------|---------|------------|
| `bgp group <g> netbox value` | none: absent means the group name | records whose `group` field equals this value. This effective value, never the group name, is the group's entry in the `bgp_peer_groups` choice set that `show netbox setup` prints and `netbox-setup-drift` checks | S-15 |
| `bgp group <g> netbox field <logical-field>` (group, remote-address, remote-as, local-as, devices, status, description) | none: the record's path (`custom_fields.peer_group`, `address`, `custom_fields.remote_as.asn`, `custom_fields.local_as.asn`, `custom_fields.devices`, `status.value`, `description`) | path of the logical field in a record (dotted JSON path, lowercase names, digits and underscores, checked by YANG pattern, D-18). The device filter is not declared separately: it is DERIVED as `cf_<name>` from the effective `devices` path `custom_fields.<name>`, so overriding `field devices` moves the filter with it. A `devices` path not of that form has no REST filter and fails the build naming the leaf (selector verify) | S-15 |
| `bgp group <g> netbox status-active`, `status-inactive` | none: the record's (`active` / `deprecated`) | status values that make a peer present, or present and inactive; any other status = absent. A configured leaf-list replaces the record's | S-15 |
| `bgp group <g> netbox source-tag-ipv4`, `source-tag-ipv6` | none: the record's (`bgp-source-ipv4` / `bgp-source-ipv6`) | slug of the NetBox tag that marks, per family, the one device interface whose address in that family is the local address of the group's iBGP peers (D-22). A configured leaf replaces the record's value for that group | S-16 |

Removed with D-19 (2026-10-08): the `model`, `endpoint`, `virtual-machine-filter`,
`refuse-when-set`, `group-refuse-when-set` and `group-endpoint` leaves, and the preset registry.
The endpoint is fixed at `ipam/ip-addresses`, because the peer list's key space rests on one record
type (Provenance, Key). Ze reads only the fields its own model defines, so no foreign body field is
left to refuse. A client of `kind virtual-machine` bound to a BGP group fails the build naming the
kind (AC-45): `devices` targets `dcim.Device`, and v1 defines no virtual-machine field.

| Path | Default | Selects | Depends on |
|------|---------|---------|------------|
| `interface unit-template <t> netbox custom-field` | `interface_template` (a selection custom field backed by a Choice Set), value = template name; one unit template gives a one-choice set (A-16) | sub-interfaces of the device whose field equals the template name | S-5, S-8 |
| `interface unit-template <t> netbox tag` | absent | alternative selector by tag slug | S-8 |
| `vpn ipsec site-to-site template <t> netbox tunnel-group` | the template name | tunnels whose `group` has this name | S-7 |
| `vpn ipsec site-to-site template <t> netbox tag` | absent | alternative selector | S-8 |
| `netbox service custom-field` (top level, child 6) | `service` | the field whose value ties a customer service together | S-3, S-8 |

Every selector also requires the object to belong to the client's device (device FK, `devices` for a BGP
remote address, or the interface's device for interface addresses and tunnel terminations). An object matching two templates
fails the build naming both (R-6).

### Provenance and ownership (D-10)

| Element | Design |
|---------|--------|
| Key | every generated list entry is keyed `nb-<netbox-object-id>` of the record it is built from: a BGP peer by its remote IPAddress, a unit by its Interface, a site-to-site peer by its Tunnel, an ike-group by its IKEPolicy, an esp-group by its IPSecPolicy. A rename in NetBox changes only `description`, never the key. Each generated list draws its keys from ONE NetBox object type, so two keys in one list cannot collide: Ze peer names are unique across groups (`resolve.go`, "duplicate peer name"), and every peer is an IPAddress id; units sit in the `unit` list of their parent interface; `site-to-site peer`, `ike-group` and `esp-group` are three separate lists in `ze-ipsec-conf.yang`. One IPAddress selected by two groups is R-6 (AC-44), not a key collision. The same key on two routers is no collision either: each router has its own served config (Q-19 closed by D-19) |
| Reserved prefix | the netbox plugin registers `nb-` with the generated-key prefix registry of `internal/component/config`; generic validation refuses an operator-authored key starting with a registered prefix in every list whose entries use the `generated` grouping. No bgp, iface or ipsec code names it (unlike the `dyn-` literal in `resolve.go`) |
| Provenance | a generic `generated` container on the entry (`ze-generated.yang`, extension `ze:generated`): `builder` (`netbox`), `source` (the `netbox source` name), `object-type` (the endpoint the record came from, for example `ipam/ip-addresses` or `vpn/tunnels`), `object-id`, `change-id` (last changelog id seen for it), `contributor` leaf-list (`<object-type>:<id>` of the other objects the entry read: IP address, ASN, VLAN), `url` (the record's `display_url`, the NetBox UI page). Written only by the builder. It stays out of `PeerSettings` and its equivalents, so a provenance change never restarts a session |
| Display | config `show` prints the container with the entry; the router answers "where did this come from" with no new command. The `url` is stored, because only the hub knows the source URL; NetBox supplies it in every record (External sources) |
| Refusal on the router | `cli.Editor.SetValue` / `DeleteValue` (and the deactivate/activate/insert paths) refuse a change at or below an entry carrying a `ze:generated` container, with the message: entry is owned by `<builder>` `<object-type>` `<object-id>`; change it at `<url>`. Schema-driven: the editor looks for the extension, so no path list exists |
| Refusal on the hub | an operator-authored `generated` container or registered-prefix key in a bound client's `client-<name>.conf` fails hub verify at build (the same generic validation), so the build is refused with `netbox-build-failed` and the last good stays served (AC-12); only the builder writes it. No commit-time refusal exists for client configs (Data Flow, Entry Point) |
| Backstop (child 2) | the router's reload refuses a diff that adds, removes or changes an entry carrying a `ze:generated` container unless the reload is a managed commit. The check sits in `Server.reloadConfig` (`internal/component/plugin/server/reload.go`), over the `config.DiffMaps` diff it already computes, because every reload passes there: SIGHUP and `daemon reload` after a file edit (`runReloadContext` -> `Server.ReloadConfig` -> `reloadConfig`; `ReloadFromDisk` only for a daemon with no reload source), and the managed commit, which writes the candidate and calls the same reload (`wireManagedCommit` in `cmd/ze/hub/managed.go`). The mark is a context value: `cmd/ze/hub/main.go` hands `wireManagedCommit` `reloadAfterCommitContext`, not the shared `reloadAfterCommit` closure that SSH, web, gNMI, the data RPC and the runtime commit also call, and `wireManagedCommit` calls it with a marked context. `runReloadContext` derives `reloadCtx` from that context, and `rollbackReload` reuses `reloadCtx`, so the rollback of a managed commit is marked too. Every unmarked reload is refused naming the entry, its builder and its `url`, and the running config stays (AC-59). A file edit followed by a restart (not a reload) is not refused, and it heals on reconnect: the client's version is the hash of the file it loaded (`fleet.VersionHash` in `cmd/ze/ze_core_start.go`), so it differs from the served config's hash and the hub serves the full config as a managed commit. Covers file mode and any writer that bypasses the editor (A-9). Generic: it reads the extension, never the `nb-` prefix |

### Composition, verify, last good (D-11)

| Step | Rule |
|------|------|
| Inputs | operator source `client-<name>.conf`; applied generated contribution for the client |
| Merge | parsed YANG trees; generated entries are added under their template; an entry key collision is impossible by the reserved prefix, and is a build error if it happens anyway |
| Hub verify | YANG validation plus the offline validators of each touched component; a template named by a generated entry that does not exist fails here (for example the operator deleted `bgp group transit` while NetBox still has addresses whose `peer_group` is `transit`) |
| On failure | the served config stays the last good one; warning `netbox-build-failed` (source `netbox`, subject client name, detail: object or template at fault); metric `ze_netbox_build_total{result="failed"}` |
| Atomicity | one build per client covers every change the trigger discovered; the router applies the served config as one transaction. Several NetBox change sets coalesced in `settle-time` become one transaction, never one change set split over two |
| Router refusal | `config-ack ok:false` raises warning `managed-config-rejected` on the hub (generic, not NetBox-specific; also useful without NetBox) |

### Served-config provider (child 2, the seam R-9 names)

Today `ReadConfig` is `store.ReadFile(pluginserver.ClientConfigKey(name))`. The netbox plugin is
feature-gated, so always-on `cmd/ze/hub` MUST NOT import it (`feature-gates.txt`). The seam is a
registration:

| Element | Design |
|---------|--------|
| Registry | a one-slot served-config provider registry in `internal/component/plugin/server`. The netbox plugin registers its provider from its gated registration; with the gate off nothing registers. A second registration fails at startup naming both, so the hub has ONE composition point: fleet-2's redesign (R-9, Q-10) feeds this provider and never registers a second one |
| Call | `ReadConfig(name)` asks the provider first. The provider answers with bytes or declines the client; on decline, and with no provider, `ReadConfig` reads `client-<name>.conf` as today |
| Notify | the registration hands the provider a notify function (`NotifyConfigChanged`), called when a client's answer changes: a publish, or a bind or unbind taking effect (next row) |
| Settle | the registration also gives the server a settle function. The server calls it each time a reload is accepted (`reloadAcceptance.complete` in `internal/component/plugin/server/reload_tx.go`, beside `finishRemovalScope`, only when accepted), and once at start. The settle takes the binding set from the `netbox client` list of the tree it is given, never from the plugin's last apply, so a bind or unbind changes the provider's answer only here; the plugin's config apply changes no answer, and `OnConfigRollback` has no binding to undo. The commit is final only at acceptance: `config-committed` fires inside the transaction, and `runReloadContext` (`cmd/ze/hub/main_reload.go`) can still call `rollbackReload` after it, inside the same acceptance scope (`DeferReloadAcceptance`), so neither the apply nor `config-committed` is the point of no return. A refused commit therefore never notifies a client and never serves it a stripped template |
| Overlapping acceptances | the server releases the transaction lock before the acceptance (`reloadConfig` in `reload.go`), and the hub's later steps run outside it, so reload B can commit while reload A is still pending (the `previous` chain in `reload_tx.go` exists for this). Three rules make the order safe. (1) Each acceptance scope records, when it claims ownership under the transaction lock (`claimReloadOwnership`), a sequence number from one server counter that only increases, and the candidate tree it commits; the chain orders scopes but gives no number to compare, and the scope keeps no tree after acceptance today. (2) Settle and the sweep run as one step under one server mutex, with the tree of the scope being accepted, never the running tree, which can hold a newer scope's tentative change. (3) A completion whose sequence is not above the last settled one is a no-op: no settle, no sweep. So an A accepted after B settled cannot revert the binding to A's set or sweep a client B re-bound, and A's completion never applies B's tentative unbind. The start settle has sequence 0 |
| Settle never waits for a fetch | settle runs in the committing caller (`runReloadContext` and `reloadConfig`'s deferred accept), so it MUST NOT wait for a build. Every write of a client's state (the three keys below) is checked against the last settled binding set under the plugin's settle lock, and a write for a client not bound there is dropped. The lock is held only for the check and the local store write, never across a NetBox fetch. Under that lock the settle replaces the binding set, cancels the in-flight builds of each unbound client (their context), and notifies the changed clients, then returns. A write that ran before the settle took the lock is finished and the sweep deletes it; a write after it is dropped. Settle is bounded by one local store write. The simpler wait-for-builds design fails because a build blocked in a NetBox HTTP fetch would stall the commit and every acceptance behind it |
| Bound client | the last good composed config, stored under key `meta/netbox/served/<client>` (registered by the plugin). Bound with no stored build yet: the stripped template (next row), with no warning. A write of `client-<name>.conf` while bound does not change the answer: the stored last good stays served until the rebuild the write triggers publishes, and a rebuild that fails verify keeps it (D-11, AC-12) |
| Unbind discards the client's state | the plugin keeps a client's state under three keys, `meta/netbox/served/<c>`, `meta/netbox/applied/<c>` and `meta/netbox/held/<c>`, declared as derived state bound to `netbox client` ("Derived-state sweep" below). The plugin itself deletes nothing. At settle, an unbound client's builds stop, its answer becomes the unbound one (rows below) and the client is notified; the server's sweep then deletes the three keys. A rollback before acceptance (`OnConfigRollback`, `rollbackReload`, a rejected live removal) leaves the binding and its state as they were. A rebind after an accepted unbind is a first build: the stripped template is served until it publishes, and the bootstrap rule ("Apply policy") applies it at once in every mode. A held change from before the unbind is gone, so it is never applied. When that first build fails verify, the client stays on the stripped template, with no generated entry and no stored last good, until a build passes; `netbox-build-failed` names the client and the fault. This is the intended safe state: the router never runs an entry built for the old binding, and the operator is alerted |
| Not bound, config carries `netbox` selectors | `client-<name>.conf` parsed, the selectors removed by the same strip function the compose step uses (one strip point, every mapper's selectors), serialized; warning `netbox-unbound-selectors` naming the client, cleared once the conf carries no selector. The plugin evaluates the warning on unbind, on each write of `client-<name>.conf`, at hub start and on each fetch, so an offline client is warned before it fetches. This is the revert path: deleting `netbox client <c>` serves the template without selectors and without generated entries (AC-56) |
| Not bound, no selector | declined, so the bytes and version hash are those of `client-<name>.conf` (AC-8) |
| Gate off, stored served blob left behind | no provider exists, so the blob is never read and never served; `client-<name>.conf` is served as today. A gate-off hub config cannot hold the `netbox` root, so the start sweep deletes the blob and the applied and held records through the persisted declaration ("Derived-state sweep"). Its selectors are unknown keys to the router's schema (AC-49), so the router refuses at parse, keeps its running config and acks `ok:false`, and the hub raises `managed-config-rejected` naming the client (AC-14, AC-57). Stripping is not possible here: only the gated plugin's schema knows the selectors, and naming `netbox` in always-on code would break plugin ownership. The operator removes the selectors to recover |

### Derived-state sweep (child 2, generic)

A plugin's stored state that only means something while a config entry exists must go when the
entry goes, also when the plugin does not run. Autoload starts a plugin only for a present config
root (`Server.getConfigPathPlugins`, `internal/component/plugin/server/startup_autoload.go`), so a
`netbox` root removed while the hub was down, or removed live, leaves no plugin to clean up. A live
removal's `bye removed` callback is not the place either: the removal stays provisional until the
reload is accepted, and a rejected one restarts the plugin (`finishRemovalScope`,
`internal/component/plugin/server/startup_removal.go`). The server sweeps instead, from what each
plugin declares. Always-on code names no plugin.

| Element | Design |
|---------|--------|
| Declaration | a new `Registration` field, `DerivedState` (`internal/component/plugin/registry/registry.go`): a list of pairs, a storage key prefix and the config list that binds it. Netbox declares `meta/netbox/served/`, `meta/netbox/applied/` and `meta/netbox/held/`, each bound to `netbox client`. A key is the prefix followed by one list entry's key. A prefix outside `meta/<plugin name>/` fails registration at startup naming the plugin |
| Persistence | at start the server writes each registration's declaration under `meta/plugin-derived/<plugin>`, and the sweep reads every persisted declaration. A persisted declaration is store data, so the sweep checks each prefix again when it reads it, with the registration rule: strictly under `meta/<plugin>/`, ending in `/`, never `meta/<plugin>/` itself, never under `meta/plugin-derived/`. A record that fails the check is skipped with an error naming the plugin and the prefix, so a stale or damaged record cannot sweep another plugin's keys or the plugin's other keys. A hub built without a plugin's gate still sweeps that plugin's state, so a gate-off start followed by a gate-on start with the binding re-added serves nothing stale |
| Sweep | for each declaration, list the keys under the prefix (`ListKeys`) and remove (`RemoveKey`) each whose entry is not in the config list of the accepted config. An absent root has no entries, so all its keys go |
| When | at hub start, after the config is loaded and before the plugin phase (`Server.runPluginStartup`, ahead of `getConfigPathPlugins`), then the provider's start settle. Both read the full loaded tree from the coordinator, `s.reactor.GetConfigTree()` (`Coordinator.GetConfigTree`, built from the loaded tree by `NewCoordinator` in `cmd/ze/hub/main.go`, and the tree `deliverConfigRPC` in `startup.go` hands plugins), never `ServerConfig`, which carries only `ConfiguredPaths`. Both finish inside `runPluginStartup`, so before `WaitForStartupComplete` returns and before `startManagedServer` (`cmd/ze/hub/main.go`) serves a fetch: no fetch meets a provider with an empty binding set. After every accepted reload (`reloadAcceptance.complete`), in the one settle-and-sweep step under the server mutex ("Overlapping acceptances"), with the accepted scope's tree, after the settle; the write check ("Settle never waits for a fetch") means no state write for an unbound client lands after the sweep. Never on a rejected reload, so every rollback window keeps the state |
| Store | the plugin server holds no store today; `cmd/ze/hub/main.go` hands it the hub store |

### Removal

| Event | Result |
|-------|--------|
| Object deleted in NetBox, or no longer matching a selector | entry absent from the next build, removed through the apply policy |
| Shrink: the candidate removes more than `system apply-policy shrink-threshold` percent of the client's generated entries | held with reason `shrink` in every mode, warning `netbox-shrink-held`, gauge `ze_netbox_shrink_held_clients` (shared mechanism). One system-level value for every source (D-15); absent = no shrink hold |
| The selector's target is gone (peer group, tunnel group, choice value deleted or renamed) | build error, not an empty member set; last good stays (R-5) |
| Device not found in NetBox | build error; last good stays |
| NetBox unreachable, 401, 403, 5xx, malformed JSON | no build; last applied stays; client marked stale with `stale-since`; warning `netbox-unreachable` or `netbox-auth-failed` after the first failure; cleared on the next success |

### Apply policy: one mechanism

The IRR spec defines when a fetched change takes effect. The NetBox builder needs exactly the
same decision over a different payload (a generated config contribution instead of a prefix
list). Decided (Q-1, D-16): one shared mechanism used by both.

| Part | Shared | Per consumer |
|------|--------|--------------|
| YANG `apply` grouping (its contents are the IRR spec's; the shrink threshold is not in it, see the system-level row below) | yes, `ze-apply-policy.yang` | where it is `uses`d: `bgp policy irr`, `firewall irr`, `netbox source` |
| Decision: given mode, delay, window, threshold, clock, removal ratio -> apply now, hold until T, hold for operator, hold for shrink | yes, `internal/core/applypolicy`, with an injected clock and random source | - |
| Window evaluation (today private to `config/system`) | yes, moved once (the IRR spec already plans the move) | - |
| Validation of the grouping's cross-leaf rules (`scheduled` needs both window leaves, `start` != `end`, `delay maximum` not below `minimum`) | yes, one function in `internal/core/applypolicy`, called by every consumer's verify (`spec-irr-apply-policy`, AC-58 here) | - |
| Held record (held-since, apply-due, reason, dismissed digest) and its persistence | no: the IRR spec keeps its held record and persistence in the IRR store | each consumer owns its own (IRR store `meta/irr-applied/...`; the netbox plugin per client). The decision's outcome (reason, due time) is the shared input both records carry |
| Shrink threshold | one leaf, `system apply-policy shrink-threshold`, read by every consumer (D-15) | the removal ratio each consumer computes (IRR: per family of a prefix list; NetBox: generated entries of one client) |
| Commands | same verbs (`update ... apply`, `clear ... held`, `show ... held`) | same argument shape, `all` or one object, with the object per consumer: IRR `all|asn <n>|as-set <name>`, NetBox `update netbox apply all|client <c>`, `clear netbox held all|client <c>`, `show netbox held [client <c>]` |
| Alert | same warning contract and gauge pattern | codes `irr-shrink-held`, `netbox-shrink-held` |

Effect on the IRR spec: Q-1 is approved (D-16), and `plan/spec-irr-apply-policy.md` was edited in
place on 2026-10-08 (a dated note at its top names the change): the policy decision, the `apply` grouping and the window evaluator
go to the shared package, the prefix-list storage stays in the IRR store, and the shrink
threshold leaves the per-consumer `apply` container for the one system-level leaf (D-15). Child 2
depends on that spec and uses what it creates.

Assumption A-14: a plugin consumer (the BGP IRR filter, the firewall IRR plugin, the netbox
builder) reads `system apply-policy shrink-threshold` by listing `system` in its registration's
`ConfigReads` (`internal/component/plugin/registry/registry.go`, the way `bgp-rpki` reads `pki`);
what stays to validate is that a commit changing only that leaf re-delivers config to the
consumer (the IRR spec's A-10 owns the same check).

Bootstrap: a client with no applied generated contribution applies its first successful build at
once in every mode (same rule as the IRR spec Q-1). An accepted unbind deletes the client's applied and held
records ("Derived-state sweep"), so a rebind after it is a bootstrap; an unbind rolled back before
acceptance deletes nothing and is not; a template edit while bound is not,
and its rebuild follows the policy against the applied contribution.

### BGP sessions: Ze's own model on core NetBox (child 3, D-13, D-19)

A BGP peer is the customer's (remote) `ipam.IPAddress`. Custom fields that Ze defines on that
address name the Ze group, the remote AS and the Ze routers that peer with it. The builder reads,
per enabled group, the remote addresses of the client's device and extracts logical fields by
path (defaults: the one record above). There is one model and one code path.

| Logical field | Ze | Rule |
|---------------|----|------|
| record `id` | peer key `nb-<id>`; provenance `object-type` `ipam/ip-addresses` | D-10 |
| group | the Ze group the peer is generated under | must equal `value` |
| remote-address | `connection remote ip` | without prefix length |
| (no field: derived) | `connection local ip` | "Local address per device" below |
| remote-as | `session asn remote` | |
| local-as | checked only | when set, equal to the group's `session asn local`, else refused naming both |
| devices | client binding | must list the client's device. A record the `cf_<name>` filter returned whose `devices` does not list it is REFUSED (build error naming the record and the device), never skipped: the filter selected it, so the filter and the field disagree, and skipping would hide a filter that may also be dropping records |
| status | presence | `status-active` present, `status-inactive` present and inactive, otherwise absent |
| description | `description` | a rename changes only this; no flap |
| IPAddress `vrf` (core field, no mapping leaf) | none | non-null refuses the build naming the record and the VRF: v1 maps no VRF, and a peer generated in the default table would be silently wrong (D-6). Prefix limits, policies and max-prefix are template body in the Ze group |
| a group whose `connection remote ip` is `dynamic` with a `netbox` container | verify refuses | dynamic groups have no static members |

The custom fields, on the remote `ipam.IPAddress`. The names are vendor-neutral defaults (D-20),
each overridable per group by a `field` leaf:

| Custom field | Type | Points to | Ze requires it when `peer_group` is set |
|--------------|------|-----------|---------------------------------------------|
| `peer_group` | Selection, choice set `bgp_peer_groups` (values = each bound group's effective `value`, by default its name) | - | it is the selector |
| `remote_as` | Object | `ipam.ASN` | yes |
| `devices` | Multiple object, filter logic not `disabled` | `dcim.Device` | yes, at least one device; the client's device must be among them |
| `local_as` | Object | `ipam.ASN` | no (checked when set) |

`devices` is a deliberate exception to section 22 "Duplication: Not permitted": the remote
address is not assigned to any router, so no core NetBox relation names the routers, and the field
is the binding the `cf_devices` filter selects on. It is the only stored binding: the local
address is derived from the device's own addresses, so no stored value can disagree with NetBox's
own interface assignment.

The `bgp_peer_groups` choice set holds the effective `value` of each Ze group bound to NetBox
(the group name unless the group sets `value`). One group
gives a one-choice set, which NetBox 4.6.10's model accepts although its page asks for two
(External sources, A-16). The nested shape of object and multi-object custom fields in REST
answers is S-15.

#### Local address per device (D-21, D-22)

The local address is DERIVED, never stored on the remote address's record. One remote address
may list several devices (a customer of two route servers, or of two edge routers); each listed
router builds its own peer, keyed `nb-<id>` in its own served config, with a local address derived
from that router's own addresses. The eBGP input is the per-client
`/api/ipam/ip-addresses/?device_id=<id>` answer: the addresses assigned to the device's
interfaces, each with its prefix length (External sources). The iBGP input, per family F that
has at least one iBGP record, is the device's interfaces carrying F's effective source tag
(`/api/dcim/interfaces/?device_id=<id>&tag=<tag-F>`) and, when exactly one does, its addresses
(`/api/ipam/ip-addresses/?interface_id=<interface id>`) (D-22, External sources). The local
address always has the remote address's family.

The session kind is decided by one comparison: the record's remote AS against the peer's
effective `session asn local`, which is Ze config (the group, through inheritance). It is iBGP
when they are equal, eBGP otherwise. The NetBox `local_as` field is only checked against that
value (logical field table above), so the local AS has one declaration.

| Kind | Case | Result |
|------|------|--------|
| eBGP | exactly one device address whose own prefix (address and prefix length as NetBox stores it) contains the remote address | that address, without prefix length, is the peer's `connection local ip`: the router's address on the point-to-point link to the peer |
| eBGP | none (a multihop peer, or a link not recorded in NetBox) | build refused naming the record, the remote address and the device. eBGP multihop is not expressible in v1 (Known Limitations) |
| eBGP | two or more | build refused naming the record and every candidate, never a pick |
| iBGP | exactly one device interface carries the tag of the remote address's family F, and it holds exactly one address in F | that address, without prefix length, is the peer's `connection local ip`: by best practice the router's BGP loopback, shared by every iBGP session in F. The IPv4 and IPv6 tags may sit on one interface or on two |
| iBGP | no device interface carries F's tag (for example an IPv6 iBGP peer and only the IPv4 tag set) | build refused naming the record, the device, the family and the tag |
| iBGP | two or more device interfaces carry F's tag | build refused naming the record, the device, the family and every tagged interface, never a pick |
| iBGP | the one tagged interface holds no address in F, or two or more | build refused naming the record, the device, the family, the interface and every candidate address, never a pick |
| both | the remote address is itself one of the device's addresses | build refused naming the record and the device |
| both | the peer's group sets `connection local ip` while carrying a `netbox` container | hub verify refuses the group: the local address of a generated peer has one source, the derivation |

With several devices, `local_as` is checked against each router's own group, so routers of
different local AS leave it empty.

A refused derivation refuses the client's WHOLE build, eBGP sessions included (Data Flow step 4).
The iBGP refusals are raised per iBGP record, so a device with zero or several interfaces tagged
for one family, or a tagged interface with zero or several addresses in it, fails its build only
once the client has at least one iBGP record in that family; a client with only eBGP records in
that family builds normally, and a family with no iBGP record needs no tag (D-22).

Fetch per client: one lookup of the device id by name, then one GET of
`/api/ipam/ip-addresses/?cf_<name>=<id>` per distinct derived device filter of the bound groups,
paginated, one GET of `/api/ipam/ip-addresses/?device_id=<id>`, paginated, for the eBGP
derivation, and per family with an iBGP record and per distinct effective tag of that family one
GET of `/api/dcim/interfaces/?device_id=<id>&tag=<tag>` plus, when it answers one interface, one
GET of `/api/ipam/ip-addresses/?interface_id=<interface id>`.

Caps per build, declared here and enforced by child 1's client: a single REST response larger
than 16 MiB, or more than 1000 pages for one endpoint, fails the build with `netbox-build-failed`
naming the endpoint and the cap; served configs stay unchanged (AC-38).

### NetBox setup (D-14, children 1 and 3-6)

| Element | Design |
|---------|--------|
| Setup page | `docs/guide/netbox.md` states every step the operator applies in NetBox: the BGP custom fields on IP addresses and their choice set (with filter logic left enabled on `devices`, External sources), how to record a customer who peers with several Ze routers (one address, several devices), the best practice as the norm every example follows (D-22): eBGP on the point-to-point interface address, recorded with its real link prefix length, that interface passive in the IGP; iBGP from a dedicated loopback interface (NetBox type `virtual`) carrying the IPv4 source tag and the IPv6 source tag (one interface may carry both, or each family its own), with exactly one address per family on it; Thomas's reason (the link prefix is known through the IGP without next-hop-self, one iBGP source serves every session, an L2 failure must not drop every iBGP session); the two tags must exist in NetBox and a family with no iBGP session needs no tag. The page names `show netbox setup` as the place these exact values come from and never copies a field name or a choice value. Also: the interface and VPN selectors, the service marker, a service user with view-only object permissions on the object types the mappers read, a v2 token with write disabled, expiry and `allowed_ips` set to the hub, the event rule (object types, create/update/delete) and its webhook (URL `https://<hub>/netbox/notify/<source>`, secret = `notification secret`, SSL verification on). Ze never writes NetBox |
| Single declaration | each mapper registers its setup definitions (custom fields with type, object type, target, choice set; the object types it reads) in a setup registry. The page describes the shape and points to the command for exact values; it never lists values that the config decides |
| `show netbox setup [source <s>]` (recommended) | prints the definitions derived from the registry and the hub's config: the custom fields with type, object type, target and filter logic, named from each bound group's effective field paths (the defaults record, or a group's `field` override), and the choice set whose values are the bound groups' effective `value`s (the group name unless `value` is set; one value allowed); the iBGP source requirement of D-22 with each bound group's effective tag slugs (per family, on each bound device, exactly one interface carries that family's tag and holds exactly one address in that family; best practice: a dedicated loopback interface of type `virtual`, the IGP-passive point-to-point address for eBGP); the webhook URL from the hub web address, the object types for the permissions and the event rule. `| json` emits each definition shaped as the NetBox REST body of that object, for the operator's own tooling. Ze prints, the operator writes |
| `netbox-setup-drift` doctor check (recommended) | GETs `/api/extras/custom-fields/`, `/api/extras/custom-field-choice-sets/`, `/api/extras/event-rules/`; compares with the same definitions: missing field, wrong type or object type or target, `devices` with filter logic `disabled` (error: the derived `cf_<name>` filter would not exist, External sources), a bound group whose effective `value` is missing from the choice set (error: the operator cannot select it), a choice value that is no bound group's effective `value` (warning: selecting it fails the build), a source tag missing from `/api/extras/tags/?slug=<tag>` (warning: an iBGP build in that family fails), per bound device and per family with an iBGP record (the family is read from the device's records through the same `cf_<name>=<id>` GET the build uses) from `/api/dcim/interfaces/?device_id=<id>&tag=<tag>` and `/api/ipam/ip-addresses/?interface_id=<id>`: no interface carries the family's tag, two or more do, or the tagged interface holds zero or several addresses in the family (warning for each, naming the device and the family: the build failure is the error, `netbox-build-failed` naming the record), and the tagged interface's `type` is not `virtual` (best-practice warning, D-22: the iBGP source is not on a loopback), no event rule pointing at the hub (warning: catch-up only). Same producer as the command, so the two never disagree |
### Mapping: interface / VLAN / IP -> Ze unit (child 4)

Ze-authored physical interfaces (for example `ethernet eth1`) stay Ze-owned; NetBox supplies
sub-interfaces under them. Child 4 adds `interface unit-template <t>` (unit settings without name,
vlan-id and addresses) and a unit leaf naming its template, inherited like a BGP group.

| NetBox | Ze | Rule | Depends on |
|--------|----|------|------------|
| Interface `id` | unit key `nb-<id>` under the parent | D-10 | |
| `parent` | the Ze interface whose name equals the parent's name | parent missing in Ze config fails the build | S-5 |
| `name`, `description` | unit `description` | | |
| `enabled` false | unit `disable` | | |
| `mtu` | none | set: refused naming the interface. Decision (v1): Ze has no unit `mtu` leaf, and adding one is a new Ze feature outside this set, so a value Ze cannot apply is refused (D-6); the operator can ask for a unit MTU leaf later | S-5 |
| `mode` absent, with a VLAN in the sub-interface's 802.1Q tag (S-6: where NetBox records it) | unit `vlan-id` | | S-6 |
| `mode` access, tagged, tagged-all, q-in-q; `qinq_svlan`; `vlan_translation_policy` | refused | Ze has no bridge VLAN filtering or QinQ model | S-6 |
| `vrf` | refused | Ze refuses unit `vrf` today | S-5 |
| `lag`, `bridge` membership | not generated in v1 | physical topology stays Ze-owned | S-5 |
| IPAddress assigned to the interface, status active | `ipv4 address` / `ipv6 address` member | other statuses absent | |

### Mapping: VPN -> Ze IPsec (child 5, bodies from NetBox per D-5)

Tunnel selected by the template's selector, with a TunnelTermination on the client's device.

| NetBox | Ze | Rule |
|--------|----|------|
| Tunnel `id` | `site-to-site peer nb-<id>` | |
| Tunnel `status` active / disabled / planned | present / inactive / absent | |
| Tunnel `encapsulation` ipsec-tunnel, ipsec-transport | peer `mode` tunnel / transport | gre, ip-ip, l2tp, openvpn, pptp, wireguard refused in v1 |
| local termination `outside_ip` | `local-address` | |
| the other termination's `outside_ip` | `remote-address` | exactly one other `peer` termination; hub/spoke refused in v1 |
| IPSecProfile `id` | generated `ike-group nb-<ike-policy-id>` and `esp-group nb-<ipsec-policy-id>`, referenced by the peer | bodies generated (D-5); several tunnels sharing a policy share one group |
| IPSecProfile `mode` esp / ah | esp accepted; ah refused | Ze IPsec is ESP |
| IKEPolicy `version` 1 / 2 | `key-exchange` ikev1 / ikev2 | `mode` aggressive refused |
| IKEProposal encryption aes-128-cbc, aes-256-cbc, aes-128-gcm, aes-256-gcm, 3des-cbc | aes128, aes256, aes128gcm, aes256gcm, 3des | aes-192-*, des-cbc refused (no Ze enum) |
| IKEProposal / IPSecProposal authentication hmac-sha1/256/384/512 | sha1/256/384/512 | hmac-md5 refused |
| IKEProposal `group` | proposal `dh-group` | values outside Ze's accepted set refused |
| IKEProposal `sa_lifetime`, IPSecProposal `sa_lifetime_seconds` | group `lifetime` | differing lifetimes across proposals of one policy refused |
| IPSecProposal `sa_lifetime_data` | refused when set | no Ze leaf |
| IPSecPolicy `pfs_group` | null: `pfs disable`; set: `pfs enable` | Ze rekeys with the DH group the IKE SA negotiated (`childRekeyDHGroup`), so a set `pfs_group` must equal the `group` of every IKEProposal of the profile's IKEPolicy (whichever one is negotiated); any other value is refused naming the policy, `pfs_group` and the IKE groups (D-6) |
| IKEProposal `authentication_method` preshared-keys / certificates | authentication `mode` pre-shared-secret / x509 | rsa/dsa-signatures refused |
| IKEPolicy `preshared_key` | Q-4 | secret handling decision |
| template (Ze) | dead-peer-detection, close-action, connection-type, traffic-selector, vti, x509 identity | Ze-owned |

### Customer service (child 6, D-7)

| Rule | Design | Depends on |
|------|--------|------------|
| Tie | objects (Interface, IPAddress, including a remote address that carries a BGP peer, VLAN, Tunnel) carrying the same value of the `service` custom field (default) or a configured tag prefix | S-3, S-8 |
| Completeness | a service is built only when every object it references resolves on the client's device (a session's local address is assigned to a generated or Ze-owned interface; its VLAN exists) | |
| Incomplete service | not built; the previous build of that service stays; warning `netbox-service-incomplete` naming the service and the missing reference; the rest of the client builds normally | |
| Objects with no service value | built individually, as children 3-5 | |

### Commands (hub)

| Command | Answer |
|---------|--------|
| `show netbox` | per source: URL, last success, stale-since, cursor, clients |
| `show netbox client <c>` | applied build time and change ids, held state, last error, entry counts per mapper |
| `show netbox held [client <c>]` | added, removed and changed entries with their NetBox objects, held-since, apply-due, reason; every held client without `client` |
| `update netbox refresh [source <s>]` | pull now (same as a webhook) |
| `update netbox apply all|client <c>` | publish the held build |
| `clear netbox held all|client <c>` | dismiss the held build |

### Notification endpoint

| Item | Design |
|------|--------|
| Route | `POST /netbox/notify/<source>` on the hub web server, a third `WrapKind` whose handler authenticates itself (Q-9) |
| Authentication | `X-Hook-Signature` must equal the hex HMAC-SHA512 of the raw body keyed by `notification secret`, compared in constant time; otherwise 401 and a counter |
| Order | the size check (64 KiB) runs first, before the `<source>` path element is looked up and before any HMAC, so a 413 is the same for every source name and reveals none (AC-37); then the source lookup and the signature, both answering the uniform 401 |
| Body | not trusted, not parsed beyond the size check; it only triggers a coalesced pull. Replay is harmless because the effect is idempotent |
| Rate | triggers coalesce into one pending build per source (`settle-time`) |

## NetBox Survey (to complete with access to the owner's NetBox)

Run by a later agent once Thomas provides a URL and a read-only v2 token.

| Rule | |
|------|---|
| Access | GET requests only. A read-only token (`write_enabled` false). No POST, PUT, PATCH, DELETE, ever, including GraphQL until S-11 says a read-only token may query it (S-11 itself is a GET) |
| Secrets | The token is passed at run time as environment variables `ZE_NETBOX_URL` and `ZE_NETBOX_TOKEN`, never written to the repository, the spec, a scratch file or a log. Requests send `Authorization: Bearer $ZE_NETBOX_TOKEN` and `Accept: application/json` |
| Sensitive values | never print `preshared_key`, custom-field values that look like secrets, or config-context bodies; record only presence and counts |
| Recording | fill the Result and Verdict cells below; then revisit only the design rows whose "Depends on" cell names that survey ID, and update the matching A-row status |

| ID | Question | Decision that depends on it | Query (GET, relative to `ZE_NETBOX_URL`) | Confirms | Refutes | Result | Verdict |
|----|----------|-----------------------------|------------------------------------------|----------|---------|--------|---------|
| S-1 | Withdrawn (2026-10-08, netbox-bgp dropped, D-19): whether netbox-bgp is installed no longer decides anything | - | - | - | - | | |
| S-2 | NetBox version, token kind | v2 only (A-2) | `/api/status/` (`netbox-version`); the token string starts `nbt_` (Thomas confirms the token is read-only and has `allowed_ips` set to the hub) | 4.6.x, v2 token | v1 token only | | |
| S-3 | How customers are represented | child 6 tie (custom field vs tag vs tenant) | `/api/tenancy/tenants/?limit=0`, `/api/tenancy/tenant-groups/?limit=0`, `/api/circuits/circuits/?limit=1` (count), `/api/extras/tags/?limit=0`, `/api/ipam/ip-addresses/?tenant_id=<one tenant id>&limit=1` (count; the `tenant_id` filter is in `netbox/tenancy/filtersets.py` line 268 at v4.6.10) | a consistent marker per customer exists or can be added | no marker; descriptions only (then the tie must be created in NetBox first) | | |
| S-4 | Candidate routers: device or VM, role, naming | hub binding defaults `netbox device` = client name, `kind device` | `/api/dcim/device-roles/?limit=0`, `/api/dcim/devices/?role=<router role slug>&limit=0`, `/api/virtualization/virtual-machines/?limit=0` | routers are devices named like the hub client names | VMs, or a naming scheme that differs (then set `netbox device` per client) | | |
| S-5 | Sub-interfaces and parents populated on candidate routers | child 4 parent rule, refusals | `/api/dcim/interfaces/?device=<name>&limit=0` per candidate; inspect `parent`, `mode`, `vrf`, `lag`, `bridge`, `mtu`, `enabled` | sub-interfaces have `parent` set and no `mode` | sub-interfaces modelled as virtual interfaces without parent, or with modes | | |
| S-6 | VLAN modes and where the 802.1Q tag of a sub-interface is recorded | child 4 `vlan-id` source | counts `/api/dcim/interfaces/?mode=access&limit=1`, `mode=tagged`, `mode=tagged-all`, `mode=q-in-q`; for a sub-interface, its `untagged_vlan` / name suffix | L3 sub-interfaces carry their VLAN in a field the mapper can read | VLAN only in the interface name (then the mapper parses nothing and the operator must set a field: Q-11) | | |
| S-7 | VPN tunnels and IPsec profiles recorded | child 5 scope | `/api/vpn/tunnels/?limit=0`, `/api/vpn/tunnel-groups/?limit=0`, `/api/vpn/tunnel-terminations/?limit=0`, `/api/vpn/ipsec-profiles/?limit=0`, `/api/vpn/ike-proposals/?limit=0`, `/api/vpn/ipsec-proposals/?limit=0`; for `/api/vpn/ike-policies/` record only whether `preshared_key` is non-empty; `/api/schema/?format=json`: the enum values of `encryption_algorithm`, `authentication_algorithm`, `group`, `pfs_group` and `encapsulation` on the installed version, compared with the External sources row for `netbox/vpn/choices.py` | tunnels exist with groups and profiles; the installed enums equal the pinned ones | none recorded (child 5 is then the lowest priority) | | |
| S-8 | Custom fields, choice sets, tags, config contexts in use | default selector names `interface_template`, `service`; collisions | `/api/extras/custom-fields/?limit=0` (name, object_types, type, choice_set), `/api/extras/custom-field-choice-sets/?limit=0`, `/api/extras/tags/?limit=0`, `/api/extras/config-contexts/?limit=0` (names only) | no field of those names, or one whose meaning matches (reuse it) | a field of that name exists with another meaning (the operator maps the selector's `custom-field` leaf to another name; the default changes only if the clash is likely everywhere) | | |
| S-9 | Withdrawn (2026-10-08, netbox-bgp dropped, D-19): Ze defines the `bgp_peer_groups` choice values itself, so no existing peer-group naming is to be compared | - | - | - | - | | |
| S-10 | Event rules and webhooks already configured; changelog endpoint and retention | notification design; catch-up path | `/api/extras/event-rules/?limit=0`, `/api/extras/webhooks/?limit=0`; `/api/core/object-changes/?limit=1` and `/api/extras/object-changes/?limit=1` (whichever answers) | one changelog path answers; retention covers `refresh-interval` many times over | changelog disabled or very short retention (catch-up then rebuilds fully each period) | | |
| S-11 | GraphQL usable with a read-only token by GET | later optimisation only | `/graphql/?query=%7Bsite_list(pagination%3A%7Blimit%3A1%7D)%7Bid%7D%7D` (the URL-encoded form of `{site_list(pagination:{limit:1}){id}}`) | answers with a read-only token | refused (stay on REST, which is the design) | | |
| S-12 | Reachability hub <-> NetBox, both directions, and TLS | URL, CA, webhook delivery | Thomas answers: hub can reach NetBox HTTPS; NetBox's `rqworker` can reach the hub's web port; which CA signs each side | both directions open | one direction closed (without the webhook, catch-up alone works, slower) | | |
| S-13 | ASN objects used by sessions | `local_as` / `remote_as` mapping | `/api/ipam/asns/?limit=0` | ASN objects exist | - | | |
| S-14 | Device names vs hub client names | default `netbox device` | `/api/dcim/devices/?name=<client>` for each hub client name | each answers one device | mismatch (set `netbox device`) | | |
| S-15 | Custom-field filtering and shape on 4.6, name clashes, and the local-address derivation on real data | the defaults record's paths and the device filter derived from the `devices` path; the derivation (D-21) | `/api/extras/custom-fields/?name=peer_group`, `?name=remote_as`, `?name=devices`, `?name=local_as` (clash check); `/api/ipam/ip-addresses/?device_id=<candidate router id>&limit=0` (the device's addresses with their prefix lengths; answers without any setup). CONDITIONAL, only once Thomas has applied the setup (Ze never writes NetBox; until then these cells stay empty and A-1 and A-16 stay unvalidated): `/api/ipam/ip-addresses/?cf_peer_group=<value>&limit=1` and `/api/ipam/ip-addresses/?cf_devices=<device id>&limit=1` (counts narrower than unfiltered; the multi-object `contains` filter is verified in the source only, External sources); one record's `custom_fields.remote_as` carries `asn`, `devices` is a list of nested objects each carrying `id`, the record carries `display_url` and `vrf`; for each such record, exactly one address in the `device_id` answer has a prefix containing the remote address (the derivation finds its local address); with one Ze group bound, `/api/extras/custom-field-choice-sets/?name=bgp_peer_groups` shows a set holding exactly one choice (A-16) | filters narrow by value and by object id; nested objects carry those keys; derivation finds one address per record; the one-choice set exists | a name is already defined in NetBox: when its type, object type and meaning match, reuse it (the drift check then reports ok); otherwise map the logical field to another name with the group's `field` leaf, and `show netbox setup` prints that name. Neutral names make such a clash more likely than prefixed ones did; the multi-object filter needs another syntax (change the `cf_<name>` derivation in child 3, one place); remote addresses with no device address in their prefix (the eBGP derivation refuses them, D-21; multihop is a Known Limitation) | | |
| S-16 | Loopbacks and point-to-point links on candidate routers (D-21, D-22) | the iBGP source-tag rule (per family, exactly one interface carrying the family's tag, with exactly one address in it), the default tag slugs, the drift check's `virtual` test, and the eBGP derivation | `/api/extras/tags/?slug=bgp-source-ipv4` and `?slug=bgp-source-ipv6` (clash check); `/api/dcim/interfaces/?device_id=<candidate router id>&type=virtual&limit=0` (how the owner models loopbacks: type, name, and the `type` answer shape `{value, label}`); `/api/ipam/ip-addresses/?device_id=<id>&limit=0` (are link addresses recorded with their real prefix length, for example /30, /31, /127). CONDITIONAL, once Thomas has tagged the interfaces: `/api/dcim/interfaces/?device_id=<id>&tag=bgp-source-ipv4` and `&tag=bgp-source-ipv6` answer one interface each, and `/api/ipam/ip-addresses/?interface_id=<that id>` holds one address per family | each router's loopback is one interface of type `virtual` with one address per family it runs iBGP in; no existing tag has those slugs; link addresses carry their link prefix length | loopbacks modeled with another type (the drift check's best-practice test changes, one place in child 3); a tag slug already used with another meaning (the operator sets `source-tag-ipv4` / `source-tag-ipv6`); link addresses stored as /32 (no prefix contains the remote, so every eBGP build fails) | | |

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | The defaults record's paths and the `cf_devices` filter match core NetBox 4.6's REST answers | D-19; `docs/customization/custom-fields.md`, `netbox/netbox/filtersets.py` and `netbox/extras/models/customfields.py` at v4.6.10 (External sources) | a default path or the filter changes before implementation | S-15; `netbox-real-api` interop | unvalidated |
| A-2 | A v2 read-only token is available; v1 is not supported | coordinator brief (v2 since 4.5, v1 deprecated 4.6) | a v1 token needs the `Token` scheme; refused by design | S-2 | unvalidated |
| A-3 | REST GET answers everything the mappers need, filtered by device | NetBox REST filters by `device` / `device_id` | more requests per client; still correct | S-5, S-7 against the real API; mock tests | unvalidated |
| A-4 | The changelog is available and filterable by `time_after` | coordinator brief; path moved between apps (`extras` vs `core`) across 4.x | catch-up falls back to a full rebuild each period | S-10 | unvalidated |
| A-5 | Routers are NetBox devices whose names equal the hub client names | D-2 default | `netbox device` set per client | S-4, S-14 | unvalidated |
| A-6 | Sub-interfaces are recorded with `parent` and their VLAN in a readable field | NetBox models interface `parent` | child 4 design changes (Q-11) | S-5, S-6 | unvalidated |
| A-7 | A webhook can reach the hub | one RQ worker running (owner fact) | catch-up only; slower convergence | S-12 | unvalidated |
| A-8 | The managed router applies a fetched config as one reload transaction, all or nothing | `fleet-config.md` "Config Change (Two-Phase)"; `test/managed/config-push-transactional.ci` | D-11 not met on the router; must be fixed before child 2 closes | run that test; read `ClientConfig.OnCommit` | unvalidated |
| A-9 | Every interactive edit path (CLI, web, MCP) reaches `cli.Editor.SetValue` / `DeleteValue` | web `EditorManager.SetValue` delegates to `cli.Editor` | MCP or another path bypasses the refusal; the reload backstop still holds (AC-59) | grep MCP config write path | unvalidated |
| A-10 | Hub-side offline validation (`ResolveBGPTree` and iface/ipsec equivalents) catches a missing template | `ResolveBGPTree` resolves groups offline | a missing template is caught only by the router's verify (still safe, slower, router-side alert) | unit test in child 2 | unvalidated |
| A-11 | Interface and VPN template concepts can be added to iface and ipsec YANG without changing existing configs | additive lists | migration needed | child 4/5 design | unvalidated |
| A-12 | `fleet-6` divergence detection exists and fleet-7 adopt is still unbuilt | fleet-7 skeleton text | R-10 timing differs | grep `diverged` in `internal/component/managed` | unvalidated |
| A-13 | A customer service is created in NetBox over several API requests, not one | NetBox UI creates one object per request | the completeness rule is unneeded but harmless | S-10 changelog request ids for an existing service | unvalidated |
| A-14 | Every consumer (BGP IRR filter, firewall IRR plugin, netbox builder) reads the system-level `system apply-policy shrink-threshold` by listing `system` in its registration's `ConfigReads`, and a commit that changes only that leaf re-delivers config to it | D-15; `registry.Registration.ConfigReads` (`internal/component/plugin/registry/registry.go`: "roots the plugin reads but does not own"; `bgp-rpki` lists `pki` in `internal/component/bgp/plugins/rpki/register.go`, verified 2026-10-08) | the builder keeps the old threshold until its own subtree changes; then the system component publishes the value to consumers instead (still one declaration) | the same check as the IRR spec's A-10: the reconfigure path for a `ConfigReads` root, and a test that changes only the system leaf | unvalidated |
| A-15 | NetBox object permissions cannot scope a per-router token to that router's objects, so per-router NetBox tokens are not a design option | `docs/administration/permissions.md` lines 30-32 (constraints are query filters over the object's own fields); `netbox/vpn/models/crypto.py` (IKEPolicy has no device); an ASN carries no device, and the remote IP address of a session is not assigned to the router | per-router tokens become possible, but D-9 (routers hold no NetBox credential) still stands | read the permission docs at the pinned tag (done 2026-10-08); `netbox-real-api` interop | validated (docs, 2026-10-08) |
| A-16 | NetBox 4.6 accepts a choice set with one choice, so a single Ze group bound to NetBox (`peer_group`, choice set `bgp_peer_groups`) or a single unit template (`interface_template`) can be set up | `netbox/extras/models/customfields.py` line 1019 at v4.6.10 refuses only an empty set; the page (`docs/customization/custom-fields.md` line 79) asks for two | `show netbox setup` and the guide tell a one-group operator to add a placeholder choice, and the drift check treats that choice as expected | S-15 on the owner's NetBox; `netbox-real-api` interop applies a one-choice set | unvalidated |
### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | A NetBox typo or permission change removes most of a router's sessions | shrink warning | the system-level shrink threshold (Q-14); a selector target that is missing is an error, not an empty set |
| R-2 | The token is not read-only | Thomas's confirmation in S-2 | design uses GET only; doctor check reports the configured URL and token kind; documentation states the token must be read-only with `allowed_ips` |
| R-3 | A forged webhook | 401 counter | HMAC-SHA512 required; body ignored; effect idempotent |
| R-4 | A service is half built because NetBox objects arrive in several requests | `netbox-service-incomplete` warning | settle-time coalescing, completeness rule, apply delay |
| R-5 | Renaming a `peer_group` choice or a tunnel group in NetBox silently empties a Ze group | build error | target-missing is an error; last good stays |
| R-6 | One object matches two templates | build error naming both | refuse, never pick |
| R-7 | NetBox outage | `netbox-unreachable` warning, stale-since | last applied stays; routers keep running |
| R-8 | Hub verify passes but router verify fails | `managed-config-rejected` warning | router keeps running config; the build stays served but flagged; operator fixes NetBox or template |
| R-9 | fleet-2 renders client configs in `HandleConfigFetch` with `text/template`, a second producer of the served config, and text templating breaks `ai/rules/config.md` | both specs implemented independently | one composition point on the hub fed by both: the one-slot served-config provider, whose registry refuses a second provider at startup (AC-57); fleet-2 redesigned as tree composition (Q-10) |
| R-10 | fleet-7 "adopt" turns a NetBox-owned entry into an operator entry, or a router-side emergency edit changes one | divergence report lists an `nb-` entry | router editor refuses; adopt strips or refuses entries carrying a `generated` container (recorded for fleet-7) |
| R-11 | Large NetBox, many routers: build cost | build duration metric | per-client builds only for affected clients (changelog), coalesced |
| R-12 | NetBox enum values change between releases (new algorithms) | unknown value | unknown value refused naming the field, never mapped to a default |
| R-13 | Pre-shared keys transit to the hub | PSK in NetBox (S-7) | Q-4 |
| R-14 | Withdrawn (2026-10-08, netbox-bgp dropped, D-19): there is one BGP model, so no model switch re-keys a group's peers | - | - |
| R-15 | The derived local address is missing or ambiguous: an eBGP remote address's prefix holds no address of a listed router (multihop) or two of them; for the iBGP peer's family, no router interface carries the family's source tag, two do, or the tagged interface holds zero or several addresses in that family | `netbox-build-failed` naming the record, the device and the family; the `netbox-setup-drift` source-tag items | refused, never a pick (D-21, D-22); eBGP multihop is a Known Limitation of v1; S-16 checks the loopbacks and tags on the owner's routers |

## Blast Radius

| Question | Answer |
|----------|--------|
| What breaks if this is wrong? | generated BGP sessions, sub-interfaces or tunnels on managed routers are added or removed wrongly; mitigated by hub verify, router transaction, shrink hold and last good |
| How is it reverted? | delete the client's `netbox client <c>` entry: the plugin notifies the client and the hub serves `client-<name>.conf` with its selectors stripped, so the generated entries disappear in one transaction (warning `netbox-unbound-selectors` until the operator removes the selectors, after which the served bytes are `client-<name>.conf` exactly; "Served-config provider", AC-56); the code is behind a feature gate, and a hub built without it serves `client-<name>.conf` raw (AC-57) |
| Who else touches this path? | fleet-2/5/7 specs (hub-side served config), IRR apply policy (shared mechanism), managed client commit path |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | → | Feature Code | Test |
|-------------|---|--------------|------|
| hub config `netbox source` + `netbox client <c>` + `bgp group g netbox` in `client-<c>.conf` | → | builder fetch, bgp mapper, compose, publish | `test/managed/netbox-session-generated.ci` |
| signed webhook POST | → | notification handler -> coalesced build | `test/managed/netbox-webhook-pull-now.ci` |
| unsigned or wrongly signed POST | → | handler refusal | `test/managed/netbox-webhook-bad-signature.ci` |
| router CLI `set` under an `nb-` peer | → | editor refusal | `test/managed/netbox-generated-entry-read-only.ci` |
| router config file edit of an `nb-` peer, then reload | → | reload backstop in `Server.reloadConfig` | `test/managed/netbox-generated-entry-file-edit-refused.ci` |
| hub config: delete `netbox client <c>` | → | provider unbound path, selector strip, notify | `test/managed/netbox-unbind-strips-selectors.ci` |
| hub config: remove the `netbox` root offline (hub restart, as `managed-hub-ca-trust.ci` restarts it) and live, then restore it | → | derived-state sweep at start and at reload acceptance | `test/managed/netbox-root-removed-sweeps-state.ci` |
| NetBox deletes most sessions | → | shrink hold, warning | `test/managed/netbox-shrink-held.ci` |
| operator deletes the Ze group still used | → | hub verify failure, last good | `test/managed/netbox-missing-template-keeps-last-good.ci` |
| mock NetBox down | → | stale, warning, last good | `test/managed/netbox-unreachable-keeps-last-good.ci` |

## Acceptance Criteria

Grouped by the child that will own them; each child copies its rows. An AC whose expected
behavior names a generated BGP peer is owned by child 3, the first child with a real mapper, even
when it proves child 2's builder; child 2 proves the same mechanisms in unit tests with a mapper
the test registers.

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 (1) | `netbox source` with a token not starting `nbt_`, or a `notification secret` shorter than 16 or longer than 256 characters, each typed plain and as a `$9$` value, through a set line and through a config file; or a non-https URL | commit refused naming the leaf, judged on the decoded value by plugin verify in all four input forms; a valid token or a 16- and a 256-character secret given as `$9$` is accepted |
| AC-2 (1) | any build | the mock NetBox records only GET requests carrying `Authorization: Bearer nbt_...` |
| AC-3 (1) | webhook with a correct `X-Hook-Signature` | one build starts within `settle-time` + 1 s; ten webhooks within `settle-time` start one build |
| AC-4 (1) | webhook with a missing or wrong signature, for an unknown source, or for a source without `notification secret` | 401 in every case, identical bodies; no build; counter `ze_netbox_notification_total{result="refused"}` +1 |
| AC-5 (1) | mock changelog lists a change for device A only | only client A is rebuilt |
| AC-6 (1) | mock NetBox returns 503, then recovers | `show netbox` reports stale-since; warning `netbox-unreachable` raised; served configs unchanged; on recovery the warning clears and builds resume |
| AC-7 (1) | mock returns 403 | warning `netbox-auth-failed`; served configs unchanged |
| AC-8 (2) | client with no `netbox client <c>` entry and no `netbox` selector in its `client-<name>.conf` | served bytes and version hash identical to `client-<name>.conf` |
| AC-9 (3) | client bound; bgp group `transit` with `session asn local 65000` and an empty `netbox` container (no mapping config); mock IP address 42 `192.0.2.2/30` with `peer_group transit`, `remote_as` AS 64500 and `devices` listing the client's device, which holds `192.0.2.1/30` | router runs peer `nb-42` under group `transit` with remote `192.0.2.2`, local `192.0.2.1` and remote AS 64500, and a `generated` container with `builder netbox`, `object-type ipam/ip-addresses`, `object-id 42` and the record's `display_url` as `url`; the served config carries no `netbox` selector container |
| AC-10 (3) | session 42 renamed in mock NetBox | only `description` changes; the BGP session is not reset |
| AC-11 (3) | router `set bgp group transit peer nb-42 ...` or `delete` it | refused naming `netbox` `ipam/ip-addresses` 42 and the stored `url`; config unchanged |
| AC-12 (2) | a bound client's `client-<name>.conf` is written (through `ze data import` or `request data restore ... client`) with an operator-authored `nb-7` entry or `generated` container in a list using the `generated` grouping (a test-registered prefix and schema in child 2's unit tests) | the build is refused: warning `netbox-build-failed` naming the reserved prefix or the container; served config bytes and version hash unchanged (last good kept). With the netbox gate off, no prefix is registered and the same key is accepted by config validation |
| AC-13 (3) | operator deletes group `transit` while sessions still select it | build fails; served config unchanged; warning `netbox-build-failed` names group `transit` |
| AC-14 (2) | router refuses the served config (verify failure) | hub warning `managed-config-rejected` with the client name and the router's error |
| AC-15 (3) | `system apply-policy shrink-threshold 20`, 10 sessions applied, mock deletes 3 | held with reason `shrink`; warning `netbox-shrink-held`; router unchanged; `update netbox apply client <c>` publishes it and clears the warning. With the leaf absent, the same deletion is applied with no hold |
| AC-16 (3) | `apply mode operator`; a session added | held; `show netbox held client <c>` lists it with its NetBox object; `update netbox apply` publishes; `clear netbox held` dismisses and the same build does not re-raise it |
| AC-17 (3) | first successful build of a client in operator mode | applied at once (bootstrap) |
| AC-18 (4) | one webhook-coalesced change adds an interface unit and a BGP session using it | the router applies both in one transaction (one reload generation) |
| AC-19 (3) | (rewritten 2026-10-08, D-19) a selected remote address carries a non-null `vrf` | build refused naming the record and the VRF; nothing from that client published |
| AC-20 (3) | Withdrawn (2026-10-08, netbox-bgp dropped): `remote_prefix` was a netbox-bgp field; Ze's model has none | - |
| AC-21 (3) | status `deprecated`; status `reserved` (defaults); then the group sets `status-inactive reserved` | peer inactive; peer absent; then the `reserved` peer is inactive and the `deprecated` one absent |
| AC-22 (3) | `bgp group transit netbox value edge-transit` | records whose group is `edge-transit` are generated under `transit` |
| AC-22b (3) | a session whose `local_as` differs from the group's `session asn local` | refused naming both values |
| AC-22c (3) | (rewritten 2026-10-08, D-21) eBGP derivation failures for remote `198.51.100.2` (remote AS differs from the group's local AS): the device holds no address whose prefix contains it; then the device holds `198.51.100.1/24` and `198.51.100.3/24`; then the remote address is itself one of the device's addresses; then group `transit` also sets `connection local ip` | refused naming the record, the remote address and the device; refused naming both candidates; refused naming the record and the device; refused naming the group (one source for a generated peer's local address). The mock records one GET of `/api/ipam/ip-addresses/?device_id=<id>` per build |
| AC-22d (3) | Withdrawn (2026-10-08, netbox-bgp dropped): `group-endpoint` existed only for netbox-bgp peer-group objects. A `peer_group` value with no Ze group is the drift warning of AC-32 | - |
| AC-23 (4) | sub-interface of `eth1` with VLAN 100, IP 192.0.2.1/30, `interface_template` = `customer` | unit `nb-<id>` under `ethernet eth1` with `vlan-id 100`, the address, template `customer` |
| AC-24 (4) | interface with mode tagged, a VRF, or a q-in-q svlan | build refused naming the interface and the field |
| AC-25 (4) | parent interface absent in Ze config | build refused naming the parent |
| AC-26 (5) | tunnel ipsec-tunnel in group `site`, profile with IKEv2 aes-256-gcm DH 19 | peer `nb-<tunnel>` under template `site`, generated `ike-group` and `esp-group` with the mapped values |
| AC-27 (5) | aes-192-cbc, des-cbc, hmac-md5, AH, IKEv1 aggressive, or gre encapsulation | build refused naming object, field and value |
| AC-28 (6) | service `cust-17` whose BGP session's local address is on a sub-interface not yet in NetBox | service not built; warning `netbox-service-incomplete` names `cust-17`; other services build |
| AC-29 (6) | the missing sub-interface is added | the whole service is applied in one transaction |
| AC-30 (1) | `refresh-interval` 29 or 86401, `settle-time` 601 | refused by YANG range |
| AC-31 (3) | (rewritten 2026-10-08, D-19) `show netbox setup` with groups `transit` and `customer` bound; then `customer` also sets `field remote-as custom_fields.peer_asn.asn` | prints `peer_group`, `remote_as`, `devices` (Multiple object, `dcim.Device`, filter logic enabled) and `local_as` with type and target, choice set `bgp_peer_groups` with exactly `customer` and `transit`, the webhook URL and the permission object types; then `peer_asn` is printed as well, for `customer`; `| json` emits NetBox REST bodies; the mock records no request. With `transit` alone the choice set holds the one value `transit`. When `transit` also sets `netbox value edge-transit`, the choice set holds `customer` and `edge-transit`, never `transit` |
| AC-32 (3) | doctor `netbox-setup-drift` against the mock: a bound group whose effective `value` is missing from the choice set; an extra choice; a field with a wrong object type; `devices` defined as a single Object, or with filter logic `disabled`; then everything matching; then group `transit` sets `netbox value edge-transit` and the choice set holds `edge-transit` and `customer` | error, warning, error, error, then ok, each naming the item; with `value edge-transit`: ok, and a choice set holding `transit` instead reports `edge-transit` missing (error) and `transit` extra (warning); the mock records only GET |
| AC-33 (1) | a `netbox source` configured and `system apply-policy shrink-threshold` absent | `netbox-setup-drift` reports a warning naming the leaf and the recommended value 20; with the leaf set, no such warning |
| AC-34 (3) | in a bound client's `client-<name>.conf`: a `field` leaf whose value is not a dotted path of lowercase names, digits and underscores | the build is refused: warning `netbox-build-failed` naming the leaf (the YANG pattern at hub verify); served config unchanged. (The `model` half of this row was withdrawn 2026-10-08 with the `model` leaf, D-19) |
| AC-35 (1) | `show netbox setup` with a source and no mapper-specific group bound | prints the webhook URL, the event-rule object types, and the token requirements (v2, write disabled, `allowed_ips`); the mock records no request |
| AC-36 (1) | `tls ca` naming a `pki ca` that did not sign the mock's certificate; then the right one | first: no build, `netbox-unreachable` naming the TLS failure; second: builds resume |
| AC-37 (1) | webhook body larger than 64 KiB with a valid signature | refused (413) before the source is looked up and before the HMAC is computed, with the same answer for a configured and an unknown source name; no build; counter `refused` +1 |
| AC-38 (1) | a NetBox REST response larger than the per-response cap (16 MiB), or more than 1000 pages for one endpoint in one build | no build; warning `netbox-build-failed` naming the endpoint and the cap; served configs unchanged |
| AC-39 (1) | `update netbox refresh source <s>` | one build of every bound client of that source starts within `settle-time` + 1 s; an unknown source is refused naming it |
| AC-40 (1) | doctor reachability check against the mock up, down, and answering 401 | ok, error naming the URL, error naming the token (never its value) |
| AC-41 (2) | client bound with `netbox device edge-99`, absent from the mock | build fails; warning `netbox-build-failed` names device `edge-99`; served config unchanged |
| AC-42 (2) | client bound with `kind virtual-machine` | the lookup GETs `/api/virtualization/virtual-machines/?name=<device>` and never `/api/dcim/devices/` |
| AC-43 (3) | in a bound client's `client-<name>.conf`, a group whose `connection remote ip` is `dynamic` carries a `netbox` container | the build is refused: warning `netbox-build-failed` naming the group; served config unchanged |
| AC-44 (3) | two groups whose `value` selects the same record (R-6) | build refused naming both groups and the record |
| AC-45 (3) | client of `kind virtual-machine` bound, with a BGP group carrying a `netbox` container | build refused naming the group and the kind (`devices` targets devices only); no unfiltered GET recorded |
| AC-46 (4) | a sub-interface matching two unit templates (one by tag, one by custom field) | build refused naming both templates and the interface |
| AC-47 (4) | sub-interface with `mtu` set | build refused naming the interface and the field |
| AC-48 (5) | IPSecPolicy `pfs_group` 19 with IKE proposal group 19; then group 14 with `pfs_group` 19; then `pfs_group` null | `pfs enable`; refused naming the policy and both groups; `pfs disable` |
| AC-49 (3) | netbox gate off; a config holding `bgp group <g> netbox` | refused at parse as an unknown key (the augment does not exist), never accepted and inert |
| AC-50 (3) | gate on; the hub's OWN config (not a client config) holds `bgp group <g> netbox` | commit refused by the netbox plugin's verify naming the group: a selector has meaning only in a client config the builder reads. Children 4 and 5 add the same row for their selectors |
| AC-51 (3) | Withdrawn (2026-10-08, netbox-bgp dropped): every peer key is an IPAddress id, so two record types can no longer meet on one key (Q-19 closed, Provenance "Key"). One record selected by two groups stays AC-44 | - |
| AC-52 (3) | group `transit` with `session asn local 65000` on every router; mock IP address 77 `203.0.113.50/24` with `peer_group transit`, `remote_as` AS 64500 (eBGP), whose `devices` lists `edge-01` (holding `203.0.113.1/24`) and `edge-02` (holding `203.0.113.2/24`), both bound clients | `edge-01` runs peer `nb-77` with local `203.0.113.1`, `edge-02` runs peer `nb-77` with local `203.0.113.2`; each build GETs `/api/ipam/ip-addresses/?cf_devices=<own device id>`; a third bound client not listed gets no `nb-77` |
| AC-53 (3) | (rewritten 2026-10-08, D-22) iBGP, best-practice layout: group `core` with `session asn local 65000` and the default tags; mock remote addresses 88 `10.255.0.9/32` and 89 `2001:db8:ff::9/128`, each with `remote_as` AS 65000 and `devices` listing the client's device. (a) Both tags on one interface: interface `lo0` (type `virtual`) carries `bgp-source-ipv4` and `bgp-source-ipv6` and holds `10.255.0.1/32` and `2001:db8:ff::1/128`; `eth1` (IGP-passive point-to-point link) holds `192.0.2.1/30`. (b) Tags on two interfaces: `lo0` carries `bgp-source-ipv4` and holds `10.255.0.1/32`, `lo1` (type `virtual`) carries `bgp-source-ipv6` and holds `2001:db8:ff::1/128` | in both layouts peer `nb-88` with local `10.255.0.1` and peer `nb-89` with local `2001:db8:ff::1` (the tagged loopback, never the link address); a second iBGP record per family on the same router gets the same local address; the mock records the GETs `/api/dcim/interfaces/?device_id=<id>&tag=bgp-source-ipv4` and `&tag=bgp-source-ipv6` and `/api/ipam/ip-addresses/?interface_id=<id>` |
| AC-54 (3) | (rewritten 2026-10-08, D-22) iBGP failures, one at a time: an IPv6 iBGP record and no interface carries `bgp-source-ipv6`; two interfaces carry `bgp-source-ipv4` and the record is IPv4; the one `bgp-source-ipv4` interface holds no IPv4 address; it holds two IPv4 addresses. Then: no interface carries `bgp-source-ipv6` and the client has only IPv4 iBGP records and IPv6 eBGP records | each of the first four refuses the client's whole build (its eBGP sessions included) naming the record, the device and the family, plus the tag, every tagged interface, or every candidate address, never a pick; the last builds normally (a family with no iBGP record needs no tag) |
| AC-55 (3) | (rewritten 2026-10-08, D-22) `show netbox setup` and `netbox-setup-drift` with an iBGP-capable group bound (any group: the kind is decided per record) | setup prints the iBGP source requirement with the effective tag slugs per family and the best practice (a dedicated loopback interface of type `virtual` for iBGP, the point-to-point address for eBGP); drift reports a missing tag object, a device with an iBGP record in a family and no interface tagged for it, a device with two tagged interfaces in one family, and a tagged interface with zero or two addresses in the family, each a warning naming the device and the family (each fails a build only once the client has an iBGP record in that family); the mock records only GET |
| AC-56 (3) | a bound client runs peer `nb-42` (AC-9); the operator deletes `netbox client <c>` and leaves the `bgp group transit netbox` selector in `client-<c>.conf`; then removes the selector | the client is notified; the served config carries no `netbox` container and no `nb-` entry, and the router removes `nb-42` in one transaction; warning `netbox-unbound-selectors` names the client. After the selector is removed: served bytes and version hash identical to `client-<c>.conf`, warning cleared. Rebind (the selector restored, `netbox client <c>` added again, a session deleted in NetBox while unbound): `meta/netbox/served/<c>`, `meta/netbox/applied/<c>` and `meta/netbox/held/<c>` were deleted by the sweep once the unbind was accepted; the client is served the stripped template, with no `nb-` entry, until the first build, which applies at once in `operator` mode (bootstrap) and carries no entry for the deleted session. Rebind whose first build fails verify (the bound group's template deleted): the client stays on the stripped template with no `nb-` entry, `netbox-build-failed` names the client and the missing template, and nothing is applied until a build passes. Rolled-back unbind, with an `operator`-mode change held for the client: the commit deleting `netbox client <c>` is refused, once by another participant's apply (`OnConfigRollback`) and once by a step after `ReloadConfig` (`rollbackReload`); each time the client is not notified, its served bytes and version hash are unchanged, the three keys remain, and the held change is still held. Template edit while bound: the last good stays served until the rebuild publishes, and the router never receives the stripped template. Child 2 proves the same with a test-registered mapper and selector |
| AC-57 (2) | hub built without the netbox gate; the store holds `meta/netbox/served/<c>` from an earlier build and `client-<c>.conf` carries a `netbox` selector | `ReadConfig` returns `client-<c>.conf` bytes, never the stored blob; the router refuses it and keeps its running config (at parse when the router is also built without the gate; with the gate on at the router, by the netbox plugin's verify, AC-50); hub warning `managed-config-rejected` names the client (AC-14); the start sweep has removed the stored blob through the persisted declaration (AC-61). Separately: a second served-config provider registration fails at startup naming both |
| AC-58 (2) | `netbox source <s> apply`: `mode scheduled` without both window leaves; `start` equal to `end`; `delay { minimum 5; maximum 2; }` | commit refused by the netbox plugin's verify, which calls the one validation function of `internal/core/applypolicy`; each message is the one the IRR consumers give for the same input (IRR AC-16, AC-19c) |
| AC-59 (2) | managed router running a config with a `generated` entry (child 2 unit tests: a test-registered schema; child 3 `.ci`: peer `nb-42`); the operator edits the config file to change or delete that entry, then reloads (SIGHUP or `daemon reload`) | reload refused naming the entry, its builder and its `url`; running config unchanged. The same change arriving as a managed commit applies |
| AC-60 (3) | the mock answers the `cf_devices=<id>` GET with a record whose `devices` does not list the client's device; then the group sets `field devices custom_fields.routers`; then `field devices status.value` | build refused naming the record and the device; then the GET uses `cf_routers=<id>` and `show netbox setup` prints `routers`; then the build is refused naming the `field devices` leaf (no `cf_` filter exists for that path) |
| AC-61 (3) | a bound client with a published build and an `operator`-mode change held. (a) Offline: the hub is stopped, the `netbox` root is removed from its config, the hub starts, is stopped again, the root and `netbox client <c>` are restored with a session deleted in NetBox meanwhile, and the hub starts. (b) Live: a commit removes the whole `netbox` root, then a later commit restores it and the binding. (c) Live, refused: the root-removing commit is rejected after the plugin's `bye removed` | (a) and (b): after the removal is accepted, no key under `meta/netbox/served/`, `meta/netbox/applied/` or `meta/netbox/held/` remains; on re-add the client is served the stripped template with no `nb-` entry until its first build, which applies at once (bootstrap), never holds the old change, and carries no entry for the deleted session. (c): the plugin is restored, every key remains, and the client's served bytes and the held change are unchanged. Child 2 proves (a) to (c) in unit tests with a test-registered declaration, and a gate-off start removes a gated plugin's keys through its persisted declaration |
| AC-62 (3) | (added 2026-10-08, D-22) `netbox-setup-drift` best-practice check: device A's `bgp-source-ipv4` interface is `lo0` of type `virtual`; device B's is `eth1`, type `1000base-t`, holding one IPv4 address; both have an IPv4 iBGP record | A reports no source item; B reports a warning, never an error, naming the device, the family `ipv4`, the interface and its type, saying the iBGP source is not on a loopback (best practice, D-22); B's build still succeeds with local address `eth1`'s address |

## End-to-End User Stories

| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | adds a BGP session in NetBox for router edge-01 | webhook -> hub build -> served config -> config-changed -> router reload -> session up | `test/managed/netbox-session-generated.ci` |
| 2 | renames the session in NetBox | build -> description change only, no session reset | `test/managed/netbox-rename-no-flap.ci` |
| 3 | tries to edit a generated peer on the router | editor refusal with pointer | `test/managed/netbox-generated-entry-read-only.ci` |
| 4 | deletes a peer group's sessions by mistake | shrink hold, warning, operator dismisses | `test/managed/netbox-shrink-held.ci` |
| 5 | provisions a customer (sub-interface, IP, session) in three NetBox edits | settle, completeness, one transaction | `test/managed/netbox-service-atomic.ci` |
| 6 | asks where a router value came from | `show` prints the `generated` container with its `url` | `test/managed/netbox-session-generated.ci` |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestClientGETOnlyBearer` | `internal/plugins/netbox/client_test.go` | GET only, bearer header, pagination via `next` | |
| `TestNotifySignature` | `internal/plugins/netbox/notify_test.go` | HMAC-SHA512 accept, wrong, missing, constant-time compare | |
| `TestChangelogAffectedClients` | `internal/plugins/netbox/changes_test.go` | changed objects -> affected clients; expired cursor -> all | |
| `TestComposeMergesUnderTemplate` | `internal/plugins/netbox/compose_test.go` | tree merge, reserved prefix collision is an error | |
| `TestComposeMissingTemplateFails` | same | D-11 | |
| `TestReservedPrefixRefused` | `internal/component/config/generated_test.go` | a prefix a test registers is refused for operator-authored keys in every list whose entries use the `generated` grouping; an operator-authored `generated` container is refused; no bgp code names the prefix | |
| `TestGeneratedURLStored` | same | the `url` leaf holds the record's `display_url` as delivered and is shown with the entry | |
| `TestMappingDefaults` | `internal/plugins/netbox/bgp/defaults_test.go` | the one defaults record carries every path, both status lists and both source tags (`bgp-source-ipv4`, `bgp-source-ipv6`); a group leaf replaces exactly its own value and nothing else; the device filter is `cf_<name>` derived from the effective `devices` path, and a `devices` path outside `custom_fields.<name>` is refused | |
| `TestServedConfigProvider` | `internal/component/plugin/server/served_provider_test.go` | no provider: `client-<name>.conf` bytes; provider declines: same; provider answers: its bytes and their hash; a second registration fails naming both; settle is called once at start and after each accepted reload, before the sweep, and never for a rejected one | |
| `TestProviderUnboundStripsSelectors` | `internal/plugins/netbox/provider_test.go` | bound: the stored last good; bound with none stored: the stripped template, no warning; unbound with selectors: the same strip function as compose, warning `netbox-unbound-selectors`; unbound without selectors: declined; bind and unbind change the answer and call the notify function only at settle; an unbind apply followed by `OnConfigRollback`, and one followed by a re-apply of the prior binding (the `rollbackReload` shape), each leave the answer, the three keys and a held change unchanged with no notify; the plugin removes no key itself; a rebind after an accepted unbind and sweep serves the stripped template and its first build is a bootstrap; a rebind whose first build fails verify stays on the stripped template and raises `netbox-build-failed`; a `client-<c>.conf` write while bound keeps the last good served; overlap: B (rebind) settled before A (unbind) completes, then A's completion is a no-op and the client stays bound with its keys; B (unbind) still tentative when A completes, then A's settle uses A's tree and the client stays bound; a build blocked in a mock NetBox fetch does not delay settle (settle returns while the fetch is still blocked), its context is cancelled, and the state write it makes after the unbind settle is dropped, so no key exists after the sweep | |
| `TestDerivedStateSweep` | `internal/component/plugin/server/derived_state_test.go` | with a test-registered declaration: keys of entries absent from the accepted config are removed, present ones kept; an absent root removes all; a rejected reload, and a rejected live removal, remove nothing; the start sweep runs before the plugin phase and removes keys of a root absent at start; a persisted declaration whose registration is absent still sweeps; a prefix outside `meta/<plugin>/` fails registration naming the plugin; a persisted record whose prefix is another plugin's, `meta/<plugin>/` itself, under `meta/plugin-derived/`, or missing the final `/` is skipped with an error and removes nothing; overlap: two scopes A then B with B accepted first, then A's acceptance sweeps nothing and keeps the keys of an entry B re-added; A accepted while B is tentative sweeps with A's tree, not the running tree; two acceptances racing run settle and sweep one at a time, and the older one, when it runs second, does nothing | |
| `TestReloadRefusesGeneratedEntryChange` | `internal/component/plugin/server/reload_generated_test.go` | a non-managed reload that adds, removes or changes an entry carrying `ze:generated` is refused naming entry, builder and `url`; a managed reload of the same diff passes; a diff touching no generated entry passes | |
| `TestSourceVerify` | `internal/plugins/netbox/config_test.go` | token `nbt_` prefix and secret length 16..256 on the decoded value, plain and `$9$`; `apply` refusals come from the `internal/core/applypolicy` validation function | |
| `TestStripSelectors` | `internal/plugins/netbox/compose_test.go` | the composed served config carries no template-side `netbox` container, for bgp, iface and ipsec selectors | |
| `TestEditorRefusesNetBoxOwned` | `internal/component/cli/editor_provenance_test.go` | set, delete, deactivate, insert refused with pointer | |
| `TestSessionReaderDefaults` | `internal/plugins/netbox/bgp/reader_test.go` | every logical field from an IP-address record with no mapping config | |
| `TestSessionReaderFieldOverride` | same | a `field` leaf moves one logical field to another path and the same peer results | |
| `TestSessionReaderRefusesVRF` | same | a remote address with a non-null `vrf` is refused naming the record and the VRF | |
| `TestSessionReaderLocalASAndDeviceChecks` | same | local AS mismatch; a record whose `devices` does not list the client's device | |
| `TestLocalAddressDerivation` | `internal/plugins/netbox/bgp/localaddr_test.go` | the kind decided by remote AS against the effective `session asn local`; eBGP: one device address whose prefix contains the remote (used), none (refused naming record, remote and device), two (refused naming both, never a pick); iBGP (D-22): exactly one interface carrying the family's tag with exactly one address in the family (used), with both tags on one interface and with each tag on its own interface; no tagged interface (refused naming record, device, family and tag), two tagged interfaces (refused naming both, never a pick), a tagged interface with zero or two addresses in the family (refused naming the interface and the candidates, never a pick); a family with no iBGP record and no tag (builds); a group's `source-tag-ipv4` / `source-tag-ipv6` override selects by its slug; an address of role `loopback` on an untagged interface is never used; both kinds: remote equal to a device address (refused), and a group setting `connection local ip` beside its `netbox` container (refused at selector verify, so the derivation is the one source); IPv4 and IPv6; two devices on one record each get their own address (D-21) | |
| `TestSetupDefinitionsDerivedFromConfig` | `internal/plugins/netbox/setup_test.go` | choice set values equal the bound groups' effective `value`s (the name unless `value` is set); a group's `field` override changes the printed field name for that group only; the iBGP source requirement prints the effective tag slugs per family and the best practice, and a group's `source-tag-ipv4` / `source-tag-ipv6` changes them for that group only (D-22) | |
| `TestSetupDriftCheck` | same | each drift class, GET only; the D-22 source items (missing tag object; per family with an iBGP record: no tagged interface, two, a tagged interface with zero or two addresses in the family) as warnings, and the best-practice warning for a tagged interface whose `type` is not `virtual` (AC-62), never an error | |
| `TestIfaceMapper*` | `internal/plugins/netbox/iface/mapper_test.go` | interface mapping and refusals | |
| `TestVPNMapper*` | `internal/plugins/netbox/vpn/mapper_test.go` | every algorithm row, refusals | |
| `TestServiceCompleteness` | `internal/plugins/netbox/service/service_test.go` | incomplete service held, others built | |
| `TestApplyPolicyNetBox*` | shared policy package tests plus `internal/plugins/netbox/apply_test.go` | modes, shrink, bootstrap with injected clock | |

### Boundary Tests (numeric inputs)
| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| `refresh-interval` | 30..86400 | 30, 86400 | 29 | 86401 |
| `settle-time` | 0..600 | 0, 600 | N/A | 601 |
| `notification secret` length (plugin verify on the decoded value; plain and `$9$`, set line and config file) | 16..256 | 16, 256 | 15 | 257 |
| `system apply-policy shrink-threshold` | 1..100 | 1, 100 | 0 | 101 |
| NetBox object id in a key | 1..2^63-1 | 1 | 0 refused | N/A |

### Functional Tests
| Test | Location | End-User Scenario | Status |
|------|----------|-------------------|--------|
| the ten Wiring Test rows plus `netbox-rename-no-flap.ci`, `netbox-service-atomic.ci`, `netbox-iface-unit-generated.ci`, `netbox-vpn-tunnel-generated.ci`, `netbox-operator-mode-held.ci` | `test/managed/` | hub + managed client + mock NetBox via `le test fixture managed/...` | |

### Interop Tests (Scope: protocol)
| Scenario | Directory | Peer Daemon | What It Proves | Status |
|----------|-----------|-------------|----------------|--------|
| `netbox-real-api` | `test/interop/scenarios/` | one plain NetBox 4.6 container owned by the test, with the documented setup applied by the harness through its API (never the owner's NetBox) | the default paths and the `cf_devices` multi-object filter match core NetBox's real answers; one eBGP session on a remote address listing two devices, one iBGP session whose local address is the address of the loopback interface (type `virtual`) tagged `bgp-source-ipv4` (the `tag` and `interface_id` filters answered by a real NetBox, D-22), one sub-interface and one tunnel generate as in the mock tests; `netbox-setup-drift` reports ok | |
| `netbox-session-frr` | same | FRR | a NetBox-generated peer establishes with FRR; the BGP wire behavior is the existing one, so this is goal validation for story 1, not a protocol change | |

## Files to Modify
- `cmd/ze/hub/managed_server.go` (child 2) - `ReadConfig` asks the registered served-config provider first, then reads `client-<name>.conf`; the provider registration receives `NotifyConfigChanged`
- `internal/component/plugin/server` (child 2, new file beside `managed.go`) - the one-slot served-config provider registry, with its settle function
- `internal/component/plugin/server` (child 2, new file `derived_state.go`) - the derived-state sweep and the persisted declarations; `reload_tx.go` `reloadAcceptance.complete` calls settle then the sweep on an accepted reload, as one step under one server mutex, skipped for a scope older than the last settled one; `reload_compensation.go` `claimReloadOwnership` gives each scope its sequence number and keeps its candidate tree; `startup.go` `runPluginStartup` runs the start sweep before `getConfigPathPlugins`
- `internal/component/plugin/registry/registry.go` (child 2) - the `DerivedState` registration field and its `meta/<plugin>/` prefix check
- `internal/component/plugin/server/reload.go` (child 2) - `Server.reloadConfig` refuses a non-managed reload whose diff touches an entry carrying `ze:generated` (Provenance, "Backstop")
- `cmd/ze/hub/managed.go` (child 2) - `wireManagedCommit` takes a context reload and calls it with the managed-commit mark
- `cmd/ze/hub/main.go` (child 2) - pass `wireManagedCommit` `reloadAfterCommitContext` in place of the shared `reloadAfterCommit` closure; hand the plugin server the hub store for the derived-state sweep
- `internal/component/plugin/server/managed_serve.go` - `config-ack ok:false` raises `managed-config-rejected`
- `internal/component/cli/editor_commands.go` (and deactivate/insert paths) - ownership refusal
- `internal/component/bgp/yang/ze-bgp-conf.yang` (child 3) - `uses generated` on `peer` only; no `netbox` container (the selector is an augment from the netbox plugin)
- `internal/component/iface/yang/ze-iface-conf.yang` (child 4) - `unit-template`, template leaf on unit, `uses generated` on unit
- `internal/component/ike/ipsec/yang/ze-ipsec-conf.yang` (child 5) - site-to-site template, `uses generated` on peer, ike-group, esp-group
- `internal/component/config/yang/modules/ze-extensions.yang` (child 2) - the `ze:generated` extension
- `internal/component/config/yang/loader.go` (child 2) - `LoadEmbedded` gains `modules/ze-generated.yang` (its list holds `ze-extensions.yang` and `ze-types.yang` today; `ze-apply-policy.yang` is added by `spec-irr-apply-policy`); `docs/architecture/config/yang-config-design.md` "`LoadEmbedded()` loads the two foundation modules" and its module table updated in the same change
- `internal/component/config` (beside `required.go`) - generic generated-entry validation and the generated-key prefix registry
- `internal/component/web/webroute.go` - third wrap kind
- `feature-gates.txt` - netbox plugin gate
- the hub client list (`plugin hub server <s> client <c>`) - no file edit and no augment: the binding is `netbox client <c>` under the plugin's own root in `internal/plugins/netbox/yang/ze-netbox-conf.yang`
- shared apply policy - none here: `internal/core/applypolicy`, `ze-apply-policy.yang` and `system apply-policy shrink-threshold` are created by `spec-irr-apply-policy`
- `bgp` config code - none: no bgp file names the `nb-` prefix; `resolve.go` stays untouched (Required Reading, `validatePeerName` constraint)
- `docs/architecture/core-design.md` section 22 (child 2, replace "Ze has no NetBox builder today."), `docs/architecture/fleet-config.md`

## Files to Create
- `internal/plugins/netbox/` (register, client, notify, changes, builder, compose, apply, commands, metrics, doctor, `yang/ze-netbox-conf.yang`)
- `internal/plugins/netbox/bgp`, `/iface`, `/vpn`, `/service` mappers, with `internal/plugins/netbox/bgp/defaults.go` (the one record of BGP mapping defaults) and `localaddr.go` (the per-device local-address derivation)
- `internal/plugins/netbox/yang/ze-netbox-bgp-group.yang`, `ze-netbox-iface.yang`, `ze-netbox-vpn.yang` (template-side selector augments)
- `internal/component/config/yang/modules/ze-generated.yang` (the generic `generated` provenance grouping)
- `internal/test/mock/netbox/`
- `test/managed/netbox-*.ci`, the fixture scenarios, `test/interop/scenarios/netbox-real-api`, `netbox-session-frr`
- `docs/guide/netbox.md`, `docs/architecture/netbox.md`

### Integration Checklist
| Integration Point | Applies? | File / reason |
|-------------------|----------|---------------|
| YANG schema (new RPCs/config) | Yes | `internal/plugins/netbox/yang/ze-netbox-conf.yang`, `ze-netbox-bgp-group.yang`, `ze-netbox-iface.yang`, `ze-netbox-vpn.yang`, `ze-generated.yang`, `ze:generated` in `ze-extensions.yang`, bgp/iface/ipsec YANG |
| YANG validation constraints | Yes | URL pattern, field-path pattern, ranges in Boundary Tests, `choice` for selectors; the `nbt_` prefix and the notification secret's length are checked by the plugin's config verify on the decoded value, not by a YANG `pattern` or `length` (Required Reading, `ai/rules/config.md`). Constraints on template-side leaves (in client configs) bite at hub verify during the build, not at a commit |
| YANG custom validators | Yes | generic generated-entry validation (registered key prefixes, operator-authored `generated` containers refused); the refusal of a group `connection local ip` beside a `netbox` container (AC-22c) is the netbox plugin's selector verify, not a central validator |
| CLI commands/flags | Yes | `show netbox ...`, `update netbox refresh|apply`, `clear netbox held` (YANG command modules) |
| CLI grammar (keyword before value) | Yes | `client <c>`, `source <s>` |
| Editor autocomplete | Yes | `client` completes bound client names; `source` completes sources |
| Functional test for new RPC/API | Yes | `test/managed/netbox-*.ci` |
| Pipe completeness | Yes | `show netbox*` through `ApplyPipes` |
| Env var registration | No | no leaf under `environment/`; the token is config only, no env var override (Q-8) |
| Doctor check for runtime dependencies | Yes | outbound HTTPS to NetBox (URL resolves, TLS verifies, `/api/status/` GET answers, token accepted) plus a diagnostic code in `internal/core/diagnostic/codes.go` |
| Prometheus counters/metrics | Yes | `ze_netbox_build_total{result}`, `ze_netbox_fetch_total{result}`, `ze_netbox_notification_total{result}`, `ze_netbox_stale_sources`, `ze_netbox_held_clients`, `ze_netbox_shrink_held_clients`, `ze_netbox_generated_entries{client,mapper}`, `ze_managed_config_rejected_total` |
| BGP family surface (new SAFI / capability / attribute) | N-A | no wire change |

### Documentation Update Checklist (BLOCKING)
| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 1 | New user-facing feature? | Yes | `features/netbox.md` (new) |
| 2 | Config syntax changed? | Yes | `docs/guide/configuration.md`, `docs/architecture/config/syntax.md` (the `generated` container, reserved key prefixes) |
| 3 | CLI command added/changed? | Yes | `docs/guide/command-reference.md` |
| 4 | API/RPC added/changed? | Yes | `docs/architecture/api/commands.md` |
| 5 | Plugin added/changed? | Yes | `docs/guide/plugins.md` |
| 6 | Has a user guide page? | Yes | `docs/guide/netbox.md` (new): the NetBox setup for BGP (the custom fields, and the best practice of D-22 as the norm every example follows: eBGP on the IGP-passive point-to-point link address with its real prefix length, iBGP from a dedicated loopback interface carrying the per-family source tags, with Thomas's reason; D-21), the service user, token, permissions, event rule and webhook (D-14); it names `show netbox setup` as the source of the exact custom fields, choice values and `set` lines and never copies them; the recommended shrink threshold (20), with the sentence that setting `system apply-policy shrink-threshold` also holds large removals for both IRR consumers (one system value, D-15); `docs/guide/bgp-peering.md` groups section gains NetBox members |
| 7 | Wire format changed? | No | no BGP/IKE wire change |
| 8 | Plugin SDK/protocol changed? | Yes | managed protocol unchanged; the registration gains `DerivedState`: `docs/architecture/api/architecture.md` (plugin registry, the field and its prefix rule) and `ai/patterns/plugin.md` (new-plugin checklist: declare derived state instead of deleting it in a callback) |
| 9 | RFC behavior implemented, changed, or newly proven? | No | none |
| 10 | Test infrastructure changed? | Yes | `docs/functional-tests.md` (mock NetBox) |
| 11 | Affects daemon comparison? | Yes | `docs/comparison.md` (config built from NetBox) |
| 12 | Internal architecture changed? | Yes | `docs/architecture/core-design.md` section 22, `docs/architecture/fleet-config.md` (the served-config provider seam), `docs/architecture/netbox.md` (new), `docs/architecture/api/process-protocol.md` and `docs/guide/config-reload.md` (both anchor `reload.go`: a reload refuses a non-managed change to a `generated` entry; `process-protocol.md` also anchors `startup.go`: the start sweep before the plugin phase, and a live removal's state goes through the sweep at acceptance, never through `bye removed`), `docs/architecture/config/transaction-protocol.md` (anchors `reload_tx.go`: an accepted reload calls the served-config provider's settle, then the derived-state sweep), `docs/architecture/hub-architecture.md` (anchors `cmd/ze/hub/main.go`: the managed commit's reload carries the managed-commit mark) |
| 13 | Route metadata keys added/changed? | No | none |
| 14 | Prometheus counters added/changed? | Yes | `docs/plugin-development/metrics.md` |
| 15 | Registered plugin, event type, command, inventory changed? | Yes | `docs/plugin-overview.md`, `docs/features/plugins.md`, `docs/guide/status.md` |
| 16 | Changed source files referenced by doc anchors? | Yes | derived per child with `./le spec citation anchors spec plan/<child>.md`; known now: `fleet-config.md` anchors `managed_serve.go`, `cmd/ze/hub/managed_server.go` |
| 17 | Existing docs show config/CLI examples for this area? | Yes | `docs/guide/bgp-peering.md`, `fleet-config.md` examples checked against YANG |

### Discovery (`ai/rules/repo-maintenance.md`)
| Question | Answer |
|----------|--------|
| Where an agent looks first | new `ai/INDEX.md` row "NetBox, source of truth, generated config" -> `docs/architecture/netbox.md`, `docs/guide/netbox.md` |
| Rule preventing regression | the reserved prefix validator and editor refusal tests; `./le config claims` covers the new subtrees |
| Registry preventing drift | mapper registry; `feature-gates.txt` row |
| Verification | `test/managed/netbox-*.ci`, `netbox-real-api` interop |

## Implementation Steps

Order: apply-policy decision (Q-1) -> child 1 -> child 2 -> child 3 -> children 4, 5 -> child 6.
Each child starts with its wiring phase and its failing `.ci`.

1. **Child 1: source** -- YANG, client, mock, notification, changelog, unreachable, doctor, setup framework. Tests AC-1..AC-7, AC-30, AC-33, AC-35..AC-40.
2. **Child 2: builder** -- `netbox client` binding, device lookup, served-config provider and its registry, compose and the one selector strip point (bound and unbound clients), verify, last good, generic provenance (`ze-generated.yang`, `LoadEmbedded`) and `ze:generated`, prefix registry, editor refusal, reload backstop, apply policy, the client and held commands, ack warning. The generic derived-state sweep, its persisted declarations, the settle-and-sweep step and the acceptance sequence (`derived_state.go`). Tests AC-8, AC-12, AC-14, AC-41, AC-42, AC-57..AC-59, AC-61 (a) to (c) in unit tests (`TestDerivedStateSweep`, `TestProviderUnboundStripsSelectors`), plus unit tests with a test-registered mapper for every mechanism AC-9..AC-17 prove end to end. Replaces the section 22 sentence.
3. **Child 3: BGP** -- the mapping-defaults record, session-record reader, per-device local-address derivation (D-21, D-22), group augment, `uses generated` on the bgp peer, `bgp` in `ConfigReads`, setup definitions. AC-9..AC-11, AC-13, AC-15..AC-17, AC-19, AC-21, AC-22, AC-22b, AC-22c, AC-31, AC-32, AC-34, AC-43..AC-45, AC-49, AC-50, AC-52..AC-56, AC-60, AC-62, and AC-61 end to end (`test/managed/netbox-root-removed-sweeps-state.ci`: its held change and its deleted session need the BGP mapper, so the `.ci` runs here, with the sweep child 2 built).
4. **Child 4: interfaces** -- unit template, `uses generated` on the unit, `interface` in `ConfigReads`, mapper. AC-18, AC-23..AC-25, AC-46, AC-47.
5. **Child 5: VPN** -- site-to-site template, `uses generated` on the peer, ike-group and esp-group, `vpn` in `ConfigReads`, mapper. AC-26, AC-27, AC-48.
6. **Child 6: service** -- tie and completeness. AC-28, AC-29.

### Critical Review Checklist
| Check | What to verify for this spec |
|-------|------------------------------|
| Completeness | every AC-N at file:line in its child |
| Correctness | a refused object fails the client's whole build; nothing NetBox-shaped is silently dropped; no write request ever leaves the hub |
| Naming | JSON keys kebab-case; YANG leaves spelled in full; warning codes `netbox-*` |
| Data flow | NetBox types only under `internal/plugins/netbox`; routers never import the plugin |
| Rule: config.md | composition by YANG tree, never text |
| Rule: no-layering | the apply policy moved, not copied |

### Deliverables Checklist
| Deliverable | Verification method |
|-------------|---------------------|
| mock NetBox records only GET | AC-2 assertion |
| generated peer on router with provenance | `netbox-session-generated.ci` |
| rename does not flap | `netbox-rename-no-flap.ci` checks session uptime |

### Security Review Checklist
| Check | What to look for |
|-------|-----------------|
| Write isolation | the HTTP client exposes no method other than GET; test asserts it |
| Token handling | `ze:sensitive` (display placeholder); never logged; never in `show` output, warnings or doctor output |
| Token at rest | the token is stored in the hub's config as typed (`ze:sensitive` only obfuscates display; a hand-written `$9$` value is an encoding decoded on read, not encryption). The protection that exists today is the store's: the tree root and its directories are exactly 0700 and frame files exactly 0600, owned by the daemon's effective user, with symlinks refused (`docs/architecture/storage-backends.md`; `0o600` writes in `internal/component/config/storage/store.go`). Ze adds no encryption. A compromise of the hub's store, or of any copy of its files, exposes a read-only NetBox token; its reach is bounded by D-9 and the setup page (write disabled, `allowed_ips` set to the hub, expiry set, view-only object permissions). Q-20 |
| Webhook | HMAC verified before any work; body size capped; constant-time compare |
| Per-router isolation | served config keyed by authenticated name (existing) |
| Untrusted input | every NetBox string validated against the Ze leaf it fills (names, addresses, ranges); unknown enum values refused |
| Resource exhaustion | 1000 pages per endpoint per build; 16 MiB per response; 64 KiB webhook body checked first; build concurrency one per source ("Fetch per client", AC-37, AC-38) |

### Failure Routing

| Failure | Route To |
|---------|----------|
| Compilation error | Fix in the phase that introduced it |
| Test fails for the wrong reason | Fix the test assertion or setup |
| Test fails on behavior mismatch | Re-read Current Behavior; if misunderstood -> RESEARCH |
| Lint failure | Fix inline; if architectural -> DESIGN |
| Functional test fails | Wrong AC -> DESIGN, correct AC -> IMPLEMENT |
| Audit finds a missing AC | Back to the relevant phase |
| 3 fix attempts failed | STOP, report, ask |

## Design Insights

- Per-router isolation already exists in the hub; the builder adds none, it only produces per-client served configs.
- `MetaEntry` looks like a provenance carrier but is an edit-session shape; real config data is simpler and survives every path the config travels.

## Key Design Decisions
| Decision | Alternatives Considered | Rationale |
|----------|------------------------|-----------|
| Builder on the hub, routers never talk to NetBox | builder on each router | D-9; NetBox object permissions cannot device-scope shared objects (A-15); one token |
| REST GET only | GraphQL | a read-only token is defined for GET; GraphQL is POST by default and unverified with read-only tokens (S-11) |
| NetBox config rendering not used | `render-config` | rendering is a POST (verified, External sources), which a GET-only client cannot send; its output is text, and Ze composes trees, never text (`ai/rules/config.md`) |
| BGP mapping defaults as ONE data record in the BGP mapper (revised 2026-10-08, D-19: was a preset registry) | YANG defaults; a preset registry (D-17) | YANG cannot configure a leaf-list as empty, so a `status-inactive` default in YANG could never be overridden to "none"; the record is the one declaration the reader, `show netbox setup` and the drift check read. With one model a registry is machinery with one entry (`ai/rules/simplicity.md`), so the record is a plain value |
| Template-side selectors as augments from the netbox plugin | `uses` of a grouping inside bgp, iface and ipsec YANG | the `ze-role.yang` pattern; with the gate off the selectors do not exist, so a config naming them fails to parse rather than being accepted and inert |
| Uniform 401 on the notification endpoint | 404 for an unknown source | the answer reveals no configured source name |
| Template-side refusals are build refusals (`netbox-build-failed`, last good kept) | a validating commit path for client configs on the hub | today's writers of `client-<name>.conf` (`ze data import`, `request data restore ... client`) run no verify, and the write observer already reaches the builder; a new write path is out of this set |
| Client binding `netbox client <c>` under the plugin's own root | augment of `plugin hub server <s> client <c>` | the verifier sees only `ConfigRoots` plus `ConfigReads`; reading `plugin` would hand the plugin every plugin's config, and no hub module is augmented |
| NetBox `mtu` on a sub-interface refused in v1 | map it to the interface MTU; add a unit `mtu` leaf | Ze's unit has no `mtu` leaf; a value Ze cannot apply exactly is refused (D-6); a unit MTU leaf is a separate Ze feature |
| `pfs_group` accepted only when equal to every IKE proposal group | map it to a DH group on the ESP proposal | Ze's ESP proposal has no group; a PFS rekey uses the group the IKE SA negotiated (`childRekeyDHGroup`), so any other value is refused (D-6) |
| Provenance `url` stored in the entry | derive it on the router from the source URL | only the hub knows the source URL; NetBox gives `display_url` in every record |
| Provenance as a config container | `MetaEntry`; a provenance index beside the config; a new fleet field | survives restart, fetch, backup and `show` with no protocol change; typed fields |
| Key `nb-<id>` | name-based keys | D-10, no flap on rename |
| Served config stored as last good, composed on input change | compose at each fetch | compose at fetch cannot keep last good (D-11) |
| Whole build per client fails on one refused object | skip the bad object | a skipped object looks applied in NetBox (D-6 reasoning) |
| Selector target missing is an error | empty member set | R-1, R-5 |
| Apply policy shared with IRR | a NetBox copy | every fact once |
| One BGP model, Ze's own, on core NetBox (D-19, 2026-10-08) | netbox-bgp-compatible defaults with a `custom-field` preset (D-17, superseded); require netbox-bgp (first design, D-4) | Thomas does not use netbox-bgp, so Ze is bound by none of its conventions; one model removes the preset registry, the `model` leaf, the group endpoint, the refusal lists and the cross-type key collision (Q-19) |
| One session-record reader with field paths | a reader with fixed names | D-2 and Q-18: the operator names fields in config, so a NetBox that already uses other names needs config, not code |
| Vendor-neutral NetBox labels (D-20, 2026-10-08) | names mirroring netbox-bgp (D-17); `ze_`-prefixed names | the NetBox data stays usable by another export mechanism; Ze-specific names live only on the Ze side. A clash with an existing field is reused when it means the same, else mapped by the `field` leaf (S-15) |
| Local address derived per device: eBGP from the point-to-point interface address, iBGP from the address in the peer's family of the one interface carrying that family's source tag (D-21, D-22) | a stored `local_address` object field (the earlier custom-field model); a multi-object `local_addresses` field paired by device; the group's `connection local ip`; the IPAddress role `loopback` (the D-21 reading, replaced by D-22); the device's `primary_ip4` / `primary_ip6`; a tag on the IP address (replaced by the interface tag the same day) | D-21 and D-22 (Thomas's reasons recorded there); one remote address can list several routers, a stored single address serves one router only, and a stored list duplicates NetBox's interface assignment (section 22 "Duplication"). The tag is an explicit designation the operator sets, with a vendor-neutral slug (D-20), so several loopbacks never force a pick |
| `show netbox setup` and `netbox-setup-drift` from one producer | a hand-written list in the guide | every fact declared once; the guide cannot disagree with the config |
| Shrink threshold at system level | per source, per group, per consumer | D-15 |
| Served config reached through a registered one-slot provider | `cmd/ze/hub` reads a netbox key itself; the builder overwrites `client-<name>.conf` | an always-on import of a gated package is refused by the gate (`feature-gates.txt`); overwriting the operator's template destroys the source the builder composes from; one slot is R-9's single composition point |
| A bind or unbind takes effect at reload acceptance (provider settle), and the state is deleted only after it | delete in the plugin's apply, restored by `OnConfigRollback`; delete at `config-committed` and treat a later `rollbackReload` re-apply as a rebind (bootstrap) | an apply is undone by `OnConfigRollback`, and `config-committed` fires inside the transaction while `runReloadContext` can still call `rollbackReload` within the same `DeferReloadAcceptance` scope. Deleting at either point loses the served blob and the applied and held records of a commit that is then refused: the client was notified and its router dropped every generated entry, and the next build is a bootstrap that applies a pending operator or shrink hold at once. Acceptance is the one point every rollback precedes, and the server already finalizes live plugin removal there (`finishRemovalScope`) |
| Settle is a registered function the server calls in `reloadAcceptance.complete`, not a subscriber to `txevents.EventAccepted` | subscribe to `EventAccepted`, as `l2tp` `authradius` does for its own acceptance wait | `reloadAcceptance.finish` emits `EventAccepted` after `complete(true)` returns, so the sweep inside `complete` would run before a settle driven by it. The event carries only a transaction id, with no tree and no sequence, so it cannot order overlapping acceptances; it never fires at start; and a plugin outside the hub process receives it on its stream after the commit returns. The sweep needs the settle done first, with the same tree, under one lock, at start and at every acceptance: one call at one point gives that, and the event would need the same hook added beside it |
| Derived state swept by the plugin server from a `DerivedState` declaration, at start and at each accepted reload | the plugin deletes an unbound client's records at its own start; a binding epoch stored in the binding and in each record, a mismatch discarding the record | the plugin does not run when its root is absent (autoload starts it only for a present root), so a root removed offline and later re-added served stale state. The epoch has to be written into the hub config's `netbox client <c>` entry by the hub, or by hand by the operator: the first makes the hub a config writer, the second puts a value no operator can know in their config; it also leaves the rollback windows open. The sweep is registration-based, names no plugin, and closes both cases with one rule; the persisted declaration extends it to a start without the plugin's gate |
| Unbind removes the client's served blob and its applied and held records (through the sweep); start removes them for every unbound client | serve the stored blob only when it records the hash of the current `client-<name>.conf`, and treat a mismatch as a bootstrap | the hash rule misses a rebind with no template edit (hashes match, so the blob with entries for objects NetBox has since deleted is served at once). On a template edit while bound it contradicts D-11: a mismatch serves the stripped template, so a rebuild that fails verify drops every generated entry where AC-12 and `netbox-missing-template-keeps-last-good.ci` keep the last good. Every edit also removes and re-adds every generated entry, and the bootstrap applies a held shrink at once. One rule covers both cases: the state goes with the binding |
| Selectors stripped for every served client, bound or not, with a warning when unbound | strip only for bound clients | unbinding is the documented revert, and an unbound conf still carries its selectors, which the router cannot parse (AC-49) |
| Device filter derived as `cf_<name>` from the `devices` path | a separate `device-filter` leaf | the custom-field name is one fact; a second leaf goes stale when only one is overridden |
| Webhook body ignored | apply the body's data | nothing pushes data; the body is a hint only |

## Open Questions for Thomas

| ID | Question | Status / recommendation |
|----|----------|------------------------|
| Q-1 | Shared apply mechanism, IRR spec amended first | Approved (D-16); IRR spec amended 2026-10-08 |
| Q-2 | Six-child split | Approved; child 3 is the session-record reader (D-17, then D-19) |
| Q-3 | Shrink default per NetBox source | Replaced by D-15 and Q-14 |
| Q-4 | IKE pre-shared keys from NetBox | Approved: none in v1; a non-empty NetBox PSK is refused |
| Q-5, Q-6, Q-7 | netbox-bgp body fields, `remote_prefix`, `max_prefixes` | Settled by D-17, then moot by D-19 (2026-10-08): Ze's own model defines no such field, so nothing is left to refuse; limits and policies are Ze template body |
| Q-8 | Token source | Approved: config only |
| Q-9 | Webhook endpoint | Approved: hub web server |
| Q-10 | fleet-2 as tree composition | Approved, recorded here only |
| Q-11 | VLAN in names | Approved: add a NetBox field |
| Q-12 | Branching / Custom Objects skeletons | Approved: not now |
| Q-13 | Standalone router | Approved: out of scope |
| Q-14 | Default of `system apply-policy shrink-threshold` | Approved (D-18): absent = off, so IRR behavior stays identical to today (IRR AC-1); the `netbox-setup-drift` doctor check warns when a NetBox source is configured and the leaf is absent (AC-33); the guide recommends 20 |
| Q-15 | Custom-field model: one record per remote address, so one remote address cannot peer with two routers | Approved (D-18): accepted for v1; the netbox-bgp model has one record per session and handles it. Reopened 2026-10-08 as Q-21: with netbox-bgp dropped (D-19) that answer is gone. Lifted 2026-10-08 by D-21: the multi-object `devices` field lets one remote address peer with several routers |
| Q-16 | Service marker | Approved (D-18): the `ze_service` custom field is the default, tag selectable as the alternative. Renamed `service` by D-20 (2026-10-08) |
| Q-17 | Switching a group between models re-keys every peer (R-14) | Approved (D-18): a documented maintenance procedure in `docs/guide/netbox.md`; no id translation. Withdrawn 2026-10-08: one model, nothing to switch (D-19) |
| Q-18 | Field paths | Approved (D-18): free dotted paths validated by a YANG pattern, so a new NetBox release needs config, not code |
| Q-19 | Ze peer names are unique across groups, so one client whose groups use the two presets can meet a BGPSession and an IPAddress with the same id, both keyed `nb-<id>` | Resolved 2026-10-08 by D-19: one BGP model, so every peer key is an IPAddress id and no two record types share the peer list; the other generated lists each draw from one type (Provenance, Key). AC-51 and R-14 withdrawn |
| Q-20 | The NetBox token is in the clear at rest in the hub's store, protected only by the store's 0700/0600 owner-only permissions | Decided 2026-10-08 (Thomas): accepted for v1, because the token is read-only, limited by `allowed_ips` to the hub and set to expire. Any at-rest protection is a separate, generic spec for every `ze:sensitive` leaf, never a NetBox-only mechanism |
| Q-21 | One remote address, several Ze routers (two route servers, two edge routers): how is the local address of each router's session recorded? Options: (a) a multi-object `devices` field, the local address derived per device as that device's one address whose prefix contains the remote address, the group's `connection local ip` for multihop; (b) a stored `local_address` (one router only, the earlier model); (c) a multi-object `local_addresses` field paired with devices by interface assignment | Closed 2026-10-08 by D-21 (Thomas): derived, never stored; eBGP from the point-to-point interface address, iBGP from the router's one BGP loopback. The closure adopted (a) WITHOUT its multihop clause: a group `connection local ip` beside a `netbox` container is refused (AC-22c), so eBGP multihop is a Known Limitation. With (a)'s multi-object `devices` field, one remote address peers with several routers, which lifts the Q-15 limit ("Local address per device", AC-22c, AC-52..AC-55) |
| Q-22 | A router with several `loopback`-role addresses in one family (S-16): which one is the iBGP local address? Options were (a) the device's `primary_ip4` / `primary_ip6`; (b) a vendor-neutral tag (D-20) | Decided 2026-10-08 by D-22 (Thomas): "we need to tag the loopback or IP to use". Option (b), refined: one tag per family on the interface (`bgp-source-ipv4`, `bgp-source-ipv6` by default), and the `loopback` role selects nothing. Zero or several tagged interfaces, or a tagged interface with zero or several addresses in the family, fails the build only when the client has an iBGP record in that family ("Local address per device", AC-53..AC-55, AC-62) |

## Known Limitations
- Bridge VLAN filtering, QinQ, VRF, LAG membership from NetBox: Ze has no model for them; refused, not generated.
- eBGP multihop from NetBox: an eBGP local address is the point-to-point interface address (D-21), so a remote address outside every interface prefix of the router fails the build. A Ze-authored peer can still be multihop.
- BGP on a router recorded as a NetBox virtual machine: `devices` targets `dcim.Device` only (AC-45).
- An iBGP source that is not exactly one tagged interface with exactly one address in the family (D-22): once the client has at least one iBGP record in that family, its whole build fails, eBGP sessions included. A client with only eBGP records in that family builds normally, and needs no tag for it.
- Session state does not flow back into NetBox; Ze publishes it for others to pull (D-9), through existing `show` and metrics.
- NetBox Branching pre-merge validation and Custom Objects: future (D-7, D-8, Q-12).

## RFC Documentation (Scope: protocol)

N-A: no protocol implementation changes. Generated BGP and IPsec config feeds existing code.

## Checklist

### Pre-Spec Verification (before the design is presented)
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

### Goal Gates (MUST pass)
- [ ] AC-1..AC-N all demonstrated
- [ ] Every user story has a working path and a passing test
- [ ] Wiring Test table complete: every row a concrete test name, none deferred
- [ ] `./le verify worktree` passes
- [ ] Feature code integrated (`internal/*`, `cmd/*`), not library-only
- [ ] Integration and Documentation checklists answered Yes/No/N-A with evidence
- [ ] Architectural Verification table filled, including registration over hardcoding
- [ ] Critical Review passes, and `ai/rules/quality.md` is satisfied
- [ ] Every A-N confirmed or broken, none `unvalidated`
- [ ] Every item this spec did not do is a spec of its own, named here, in its own bucket

### TDD
- [ ] Tests written
- [ ] Tests FAIL (paste output)
- [ ] Tests PASS (paste output)
- [ ] Boundary tests for all numeric inputs
- [ ] Functional `.ci` tests for end-to-end behavior
- [ ] Interop tests (`netbox-real-api`, `netbox-session-frr`)

### Closure
- [ ] Append `plan/TEMPLATE-CLOSURE.md` and complete every section in it
- [ ] `/ze-review` gate clean, recorded via `internal/le/spec/review.go`
- [ ] Any lesson routed to its governing surface under `ai/rules/planning.md`
- [ ] **Commit A:** code + tests + docs + edited spec + any journal rows owed by the work
- [ ] **Commit B:** `remove plan/spec-netbox-0-umbrella.md` only, in the same `./le commit create` script
