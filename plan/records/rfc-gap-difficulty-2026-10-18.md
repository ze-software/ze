# RFC `unimplemented` verdicts: difficulty and implementation order

Date: 2026-10-18. A planning document, not a spec: it rates every RFC requirement
whose audit verdict is `unimplemented` and orders them easiest to hardest, so the
owner's "implement from easiest to hardest" has a queue. Each implementation
still needs its own spec or commit under `ai/rules/planning.md`.

## Source and method

- The set is every verdict `"verdict": "unimplemented"` in `rfc/audit/*.json`
  on the committed tree at `99228a5b3d`: **59 rows across 32 summaries**, every
  one carrying `{gap}` in `rfc/short/<stem>.md`. The handover said 57; the two
  extra rows are RFC3209-4.6.1-2 and RFC4301-4.4.2.1-3, both re-judged to
  `unimplemented` on 2026-10-18 when their wrong tags were moved off by owner
  ruling (their verdict notes say so). RFC7752-3.3.2.7-1 is `{superseded}` by
  RFC9552-5.3.2.7-1 and shares its fix.
- The requirement text below is the row quote from `rfc/short/<stem>.md`. Every
  quote was checked against `rfc/full/<stem>.txt` (or `rfc/drafts/`) by a
  whitespace- and punctuation-normalized match, and `./le rfc check` (row-quote
  gate) passed on the same tree.
- "Today" names the producing component and function, taken from the verdict's
  `code` map and re-read at the producer for the level 1 and level 2 rows
  (L2TP `ReliableConfig`, BFD `parseAuthConfig` and the `secret` leaf, DHCP
  option readers, sFlow `validateSFlowCollectors`, VRRP `FillChecksum`, IPsec
  `unmatched` default, RADIUS `exchange` and `aaa.AuthRequest`, SR Policy BSID
  encoders, the RFC 7606 validator signature, `OrderAttributes`). Level 3 to 5
  rows rest on the audit notes plus a targeted read of the named function.
- Difficulty: **1** trivial (YANG leaf and plumbing), **2** small (one function
  plus tests), **3** medium (new behavior across a component), **4** large (new
  subsystem or protocol state), **5** very large or needs external
  infrastructure. Effort is engineering days for one agent including tests and
  docs: L1 0.5 d, L2 1 d, L3 2 to 5 d, L4 1 to 3 weeks, L5 more than 4 weeks or
  blocked outside Ze.
- **Gate** marks a row that cannot be implemented on difficulty alone:
  `owner-ruling` means a recorded owner decision keeps the current behavior as
  a disclosed deviation and must be reversed first; `scope` means the row's
  annotation records an absent feature that no agreed scope covers
  (`ai/rules/rfc-compliance.md`), so the owner commissioning it is the gate.

## Summary (easiest first)

| # | Requirement | RFC section | Lvl | Effort | Gate | Depends on | Owning spec |
|---|-------------|-------------|-----|--------|------|-----------|-------------|
| 1 | RFC2661-5.8-8 | RFC 2661 5.8 | 1 | 0.5 d | | | none |
| 2 | RFC4301-4.4.1-3 | RFC 4301 4.4.1 | 1 | 0.5 d | owner-ruling (R27) | | none |
| 3 | RFC5880-6.7.2-2 | RFC 5880 6.7.2 | 2 | 1 d (shared 3-5) | | | spec-rfc-verdict-test-fix-pass |
| 4 | RFC5880-6.7.3-14 | RFC 5880 6.7.3 | 2 | with #3 | | #3 | none |
| 5 | RFC5880-6.7.4-8 | RFC 5880 6.7.4 | 2 | with #3 | | #3 | none |
| 6 | RFC3579-3-2 | RFC 3579 3 | 2 | 1 d | | | none |
| 7 | RFC9830-2.4.2-12 | RFC 9830 2.4.2 | 2 | 1 d | | | spec-bgp-unowned-rfc-rows |
| 8 | SFLOW-V5-4.3-2 | sFlow v5 4.3 | 2 | 1 d | | | none |
| 9 | RFC2131-4.1-7 | RFC 2131 4.1 | 2 | 1.5 d (shared 9-10) | | | spec-rfc-verdict-fix-services |
| 10 | RFC2131-4.1-15 | RFC 2131 4.1 | 2 | with #9 | | #9 | none |
| 11 | RFC7950-7.2.2-1 | RFC 7950 7.2.2 | 2 | 1 d | | | pre-release/spec-config-yang-loader-structural-checks |
| 12 | RFC7432-8.2.1-4 | RFC 7432 8.2.1 | 2 | 1.5 d | | | none needed: re-judged `enforced` 2026-10-18 after `0411b53e91` (`rfc/audit/rfc7432.json`); the BGP verdict fix pass (spec-rfc-verdict-fix-bgp) owned it until then |
| 13 | DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-6-1 | draft-ietf-idr-linklocal-capability 6 | 2 | 1.5 d | | | spec-bgp-unowned-rfc-rows |
| 14 | DRAFT-IETF-BESS-MUP-SAFI-3.3.6-5 | draft-ietf-bess-mup-safi 3.3.6 | 2 | 1.5 d | | | spec-bgp-unowned-rfc-rows |
| 15 | RFC5216-5.3-5 | RFC 5216 5.3 | 2 | 1.5 d | | | none |
| 16 | RFC9568-7.2-5 | RFC 9568 7.2 | 2 | 1.5 d (shared 16-17) | owner-ruling (keepalived interop) | | none |
| 17 | RFC9568-5.2.8-2 | RFC 9568 5.2.8 | 2 | with #16 | owner-ruling | #16 | none |
| 18 | RFC4271-8.2.2-20 | RFC 4271 8.2.2 | 3 | 2 d | | | spec-bgp-fsm-delayopen-and-automatic-start |
| 19 | RFC7606-5.1-1 | RFC 7606 5.1 | 3 | 3 d | | | spec-bgp-unowned-rfc-rows |
| 20 | DRAFT-IETF-BESS-MUP-SAFI-3.3.6-4 | draft-ietf-bess-mup-safi 3.3.6 | 3 | 3 d | | #14 | spec-bgp-unowned-rfc-rows |
| 21 | RFC9136-3.1-4 | RFC 9136 3.1 | 3 | 3 d | | | none needed: re-judged `enforced` 2026-10-19 after `4283f8d2e3` (`rfc/audit/rfc9136.json`); the BGP verdict fix pass (spec-rfc-verdict-fix-bgp) owned it until then |
| 22 | RFC4659-3.2.1.1-3 | RFC 4659 3.2.1.1 | 3 | 4 d | | | none |
| 23 | RFC9552-5.3.2.7-1 | RFC 9552 5.3.2.7 | 3 | 3 d | scope (optional TLV policy) | | none |
| 24 | RFC7752-3.3.2.7-1 | RFC 7752 3.3.2.7 | 3 | with #23 | scope | #23 | none |
| 25 | RFC9256-4-7 | RFC 9256 4 | 3 | 4 d | scope (MAY, types I/J/K) | | none |
| 26 | RFC4303-1-2 | RFC 4303 1 | 3 | 3 d | | | none |
| 27 | RFC4301-4.4.2-2 | RFC 4301 4.4.2 | 3 | 4 d | | | none |
| 28 | RFC7296-2.21.4-8 | RFC 7296 2.21.4 | 3 | 4 d | | | none |
| 29 | RFC4301-4.4.2.1-3 | RFC 4301 4.4.2.1 | 3 | 4 d | | RFC4301-4.4.2.1-2 (byte lifetimes) | spec-ipsec-lifetime-volume |
| 30 | RFC3101-3.1-5 | RFC 3101 3.1 | 3 | 3 d | | | none |
| 31 | RFC3101-3.1-4 | RFC 3101 3.1 | 3 | 3 d | owner-ruling (disclosed deviation) | #30 | none |
| 32 | RFC7950-7.6.5-2 | RFC 7950 7.6.5 | 3 | 3 d | | | pre-release/spec-config-yang-when-unique-choice |
| 33 | RFC7950-7.6.1-2 | RFC 7950 7.6.1 | 3 | 3 d | | #32 (case model) | pre-release/spec-config-yang-when-unique-choice |
| 34 | RFC7950-9.1-2 | RFC 7950 9.1 | 3 | 5 d | | | spec-config-yang-type-identityref |
| 35 | RFC7950-11-3 | RFC 7950 11 | 3 | 4 d | | | spec-config-yang-authoring-checks |
| 36 | RFC7950-8.1-4 | RFC 7950 8.1 | 4 | 2 w | | #32, #33 | pre-release/spec-config-yang-when-unique-choice |
| 37 | RFC7950-8.1-1 | RFC 7950 8.1 | 4 | with #36 | | #36 | pre-release/spec-config-yang-when-unique-choice |
| 38 | RFC7950-8.3.3-2 | RFC 7950 8.3.3 | 4 | 1 d after #36 | | #36, #37 | pre-release/spec-config-yang-when-unique-choice |
| 39 | RFC7950-7.21.5-2 | RFC 7950 7.21.5 | 4 | with #36 (when engine) | | #36 | none |
| 40 | RFC7950-7.6.1-4 | RFC 7950 7.6.1 | 4 | 2 d after #39 | | #33, #39 | pre-release/spec-config-yang-when-unique-choice |
| 41 | RFC4271-8.2.2-13 | RFC 4271 8.2.2 | 4 | 1.5 w | scope (DelayOpen optional) | #18 | spec-bgp-fsm-delayopen-and-automatic-start |
| 42 | RFC9252-5-2 | RFC 9252 5 | 4 | 2 w | | | immediate/spec-srv6-bestpath-resolvability |
| 43 | DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-1-2 | draft-ietf-idr-linklocal-capability 1 | 4 | 2 w | | | spec-bgp-unowned-rfc-rows |
| 44 | RFC2545-3-6 | RFC 2545 3 | 4 | 2 w | | #43 helps | spec-bgp-unowned-rfc-rows (row 5, AC-5) |
| 45 | RFC8277-3.2.2-3 | RFC 8277 3.2.2 | 4 | 2 w | scope (Multiple Labels capability) | RFC8277-3.2.1-4 | none |
| 46 | RFC9086-7-2 | RFC 9086 7 | 4 | 3 w | scope (owner R3) | | spec-bgp-epe-peer-segments-and-srv6-peernode |
| 47 | RFC9012-13-16 | RFC 9012 13 | 4 | 3 w (consumer) | scope | | spec-bgp-tunnel-encapsulation-consumption |
| 48 | RFC9012-3.7-2 | RFC 9012 3.7 | 4 | 2 d after #47 | scope | #47 | spec-bgp-tunnel-encapsulation-consumption |
| 49 | RFC9012-4.2-1 | RFC 9012 4.2 | 4 | 2 d after #47 | scope | #47 | spec-bgp-tunnel-encapsulation-consumption |
| 50 | RFC4301-5.2-1 | RFC 4301 4.4.1 | 4 | 2 w (shared 50-51) | | #2 interacts | immediate/spec-rfc4301-architecture-gaps |
| 51 | RFC4301-5.2-9 | RFC 4301 5.2 | 4 | with #50 | | #50 | none |
| 52 | RFC4301-5.1-7 | RFC 4301 5.1 | 4 | 3 w | scope | #2 interacts | none |
| 53 | RFC3948-5.1-1 | RFC 3948 5.1 | 4 | 2 w | | | immediate/spec-ike-virtual-ip-assignment |
| 54 | RFC3209-4.6.1-2 | RFC 3209 4.6.1.2 | 4 | 3 w | scope | | spec-rsvpte-ipv6-lsp-tunnel |
| 55 | RFC2328-8.1-1 | RFC 2328 8.1 | 4 | 2 w | owner-ruling 8 (g) | | none |
| 56 | RFC5392-3.2.1-5 | RFC 5392 3.2.1 | 4 | 3 w | scope | | none |
| 57 | SFLOW-V5-4.3-1 | sFlow v5 4.3 | 5 | 6 w+ | scope (SNMP agent) | | none |
| 58 | RFC4659-8-5 | RFC 4659 8 | 5 | 6 w+ | scope (no-ASBR-feature) | #22 | none |
| 59 | RFC4303-3.4.4.1-2 | RFC 4303 3.4.4.1 | 5 | external | external (kernel/VPP) | | spec-rfc-verdict-fix-ike-eap |

Count per level: **L1 2, L2 15, L3 18, L4 21, L5 3** (59).

Of the 59, 5 carry an `owner-ruling` gate (#2, #16, #17, #31, #55; #50 and #51
interact with #2's default), 14 a `scope` gate, and 1 sits outside Ze (#59). The
39 with no gate can start as soon as their dependency is in.

### Owner decisions, 2026-10-18

| Row | Decision |
|-----|----------|
| #2 RFC4301-4.4.1-3 | Follow the RFC: R27 reversed, `unmatched` defaults to discard |
| #16, #17 RFC9568-7.2-5, 5.2.8-2 | Do the work (RFC 9568 checksum; keepalived compatibility as an opt-in) |
| #31 RFC3101-3.1-4 | Fix it: the wedge-avoidance deviation is reversed |
| #55 RFC2328-8.1-1 | Unnumbered P2P, approved shape: `interface <x> unit <u> ipv4 unnumbered <donor-iface>` in iface (Cisco `ip unnumbered`, Junos `unnumbered-address`), with an optional source-address choice when the donor holds several IPv4 addresses (Junos `preferred-source-address`), else its first. Ze installs the donor address as a /32 on the link (the FRR wire shape). Commit refuses `unnumbered` with `address`, a missing, unnumbered or address-less donor, a source address the donor does not hold, and OSPF on an unnumbered non-point-to-point interface; doctor warns when the donor is not in OSPF. Donor down or address lost takes the link out of OSPF. OSPF sends a 0.0.0.0 hello mask, ifIndex link data, no stub link (RFC 2328 9.5, 12.4.1.1) |
| #7 RFC9830-2.4.2-12 | Bring up to date: the 2026-10-11 decline is reversed, implement the type-13 SRv6 form |
| #2 follow-up | Config migration writes `unmatched bypass` into configs saved before the discard default; boot with the leaf unset discards with an Error log; the RFC-tagged IPsec test changes are approved |
| #15 RFC5216-5.3-5 | No AIA caIssuers retrieval (the fetch URL comes from an untrusted client); the configured intermediate pool is the implementation, and the row stays a disclosed gap |
| #20 DRAFT-IETF-BESS-MUP-SAFI-3.3.6-4 | 2026-10-19: resolve the locator's originator from the IGP link-state snapshots; a locator in no snapshot keeps the route with a warning (that case recorded as partial); scheduled after the other medium rows |
| #22 RFC4659-3.2.1.1-3 | 2026-10-19: follow the RFC literally: include the link-local whenever the peer shares a subnet, even when the speaker's own global (e.g. a loopback) is outside that subnet |
| All other rows | Implement from easiest to hardest |

## Bundles that close several rows with one change

| Bundle | Rows | Why one change |
|--------|------|----------------|
| BFD binary secret | #3, #4, #5 | one `secret` leaf, one parser, three auth types |
| DHCP option overload | #9, #10 | one option iterator that walks options, then file, then sname |
| VRRPv3 IPv4 checksum | #16, #17 | one transmit form and one receive acceptance rule |
| BGP-LS Link Name | #23, #24 | RFC 9552 restates RFC 7752; one originator |
| YANG case model | #32, #33 | both need walkTree and ApplyDefaults to see case nodes |
| YANG when/unique/choice | #36 to #40 | one validator pass; #38 closes once #36/#37 run on every commit surface |
| Tunnel Encapsulation consumer | #47, #48, #49 | all three are rules of the absent attribute-23 consumer |
| SPD-I discard paths | #50, #51 | same three no-entry paths |

## Rows in detail

### 1. RFC2661-5.8-8 (L1, 0.5 d)
- **Requirement** (RFC 2661 Section 5.8, SHOULD): "If no peer response is detected after several retransmissions, (a recommended default is 5, but SHOULD be configurable)".
- **Today**: the L2TP component's reliable delivery engine has `ReliableConfig.MaxRetransmit` (zero means `DefaultMaxRetransmit`, 5), but both tunnel constructors (`reactor.go::locateTunnelLocked`, `reactor_dial.go::handleDial`) pass only `RecvWindow`; `ze-l2tp-conf.yang` has no leaf.
- **Missing**: a `max-retransmit` leaf (default 5, range 1..N) in `ze-l2tp-conf.yang`, parsed into the reactor defaults and passed into both `ReliableConfig` literals.
- **Proof**: unit test that a configured value reaches the tunnel's engine and that teardown fires after N retransmits on the fake clock; a `.ci` config parse test for the leaf.

### 2. RFC4301-4.4.1-3 (L1, 0.5 d, owner-ruling)
- **Requirement** (RFC 4301 Section 4.4.1, SHOULD): "Every SPD SHOULD have a nominal, final entry that matches anything that is otherwise unmatched, and discards it."
- **Today**: the IPsec component installs the final entry (`engine/unmatched.go::installUnmatched`), and `unmatched discard` makes it discard; the `vpn ipsec unmatched` leaf defaults to `bypass` (`ze-ipsec-conf.yang`, `ipsec/config.go::parseUnmatched`), kept by owner ruling R27 so the router's own control plane is not cut off on upgrade.
- **Missing**: only the default. The code change is `default discard`, but it reverses R27 and needs a control-plane bypass story (automatic BYPASS entries for BGP/SSH/DNS) or an accepted upgrade break.
- **Gate**: owner must reverse R27.
- **Proof**: `TestRFC4301DefaultFinalSPDEntryDiscards` already exists as the gap demonstration and turns green; interop-ipsec scenario confirming strongSwan traffic still flows with the discard default.

### 3 to 5. RFC5880-6.7.2-2, RFC5880-6.7.3-14, RFC5880-6.7.4-8 (L2, 1 d together)
- **Requirement** (RFC 5880 Sections 6.7.2, 6.7.3, 6.7.4, SHOULD): "the management interface by which the password is configured MUST accept ASCII strings, and SHOULD also allow for the configuration of any arbitrary binary string in hexadecimal form." (6.7.3 and 6.7.4 repeat the hexadecimal sentence for Keyed MD5 and Keyed SHA1.)
- **Today**: the BFD component's `config.go::parseAuthConfig` stores `[]byte(secret)` of the `secret` leaf (`type string`, `ze:sensitive`) in `ze-bfd-conf.yang`; ASCII is met, hex is not.
- **Missing**: a hex form, either a sibling `secret-hex` leaf in a choice with `secret` or a `0x` prefix rule documented in the leaf; parser decodes it, length limits (16/16/20 bytes) apply to the decoded bytes; redaction must cover the new leaf.
- **Proof**: unit tests per auth type (binary key with a zero byte and a high byte reaches the session key); `.ci` parse test; existing `bfd-simple-password-bird` interop extended with a hex secret.

### 6. RFC3579-3-2 (L2, 1 d)
- **Requirement** (RFC 3579 Section 3, SHOULD): "The NAS-Port or NAS-Port-Id attributes SHOULD be included by the NAS in Access-Request packets".
- **Today**: the RADIUS component's admin-login `(*radiusAuthenticator).exchange` adds Service-Type, NAS-Identifier, User-Name, NAS-IP-Address; no NAS-Port/NAS-Port-Id. `aaa.AuthRequest` already carries `Service` and `RemoteAddr`, but `exchange` receives only the username and credential.
- **Missing**: pass the request into `exchange` and add NAS-Port-Id (RFC 2869 text) naming the access channel (for example `ssh`, `web`, `api`, plus the session's tty or connection id where one exists).
- **Proof**: unit test decoding the Access-Request for each service; interop-radius EAP scenario asserting FreeRADIUS logs NAS-Port-Id.

### 7. RFC9830-2.4.2-12 (L2, 1 d)
- **Requirement** (RFC 9830 Section 2.4.2, MUST): "The value MUST be 18 when a SRv6 BSID is present".
- **Today**: the SR Policy NLRI plugin's `config.go::buildBindingSIDSubTLV` emits type 13 length 6 (MPLS), `buildBindingSIDNullSubTLV` length 2, and a configured `srv6-binding-sid` goes out as type 20 (`buildSRv6BindingSIDSubTLV`); type 13 never carries an SRv6 BSID.
- **Missing**: the backward-compatible type-13 SRv6 form (length 18: flags, reserved, 16-octet SID), selected by a config choice; decoder check that a type-13 length-18 value is accepted and any other length with an SRv6 BSID refused.
- **Proof**: encode/decode unit tests with hex; extend `bgp-sr-policy-gobgp` interop if GoBGP parses the form.

### 8. SFLOW-V5-4.3-2 (L2, 1 d)
- **Requirement** (sFlow v5 Section 4.3, SHOULD): "In the case of a multi-homed agent, this should be the loopback address of the agent."
- **Today**: the flowexport plugin's `sflow/register.go::validateSFlowCollectors` checks the agent address is specified and identical on all collectors; nothing checks it is on a loopback.
- **Missing**: at verify, resolve the address to an interface through the iface component and warn (SHOULD, so a warning naming the fix, not a refusal) when the router is multi-homed and the address is not on a loopback.
- **Proof**: unit test of the check with a fake interface table (loopback, non-loopback, absent); `.ci` showing the warning text.

### 9 and 10. RFC2131-4.1-7, RFC2131-4.1-15 (L2, 1.5 d together)
- **Requirement** (RFC 2131 Section 4.1, MUST): "The options in the 'options' field MUST be interpreted first, so that any 'option overload' options may be interpreted." and "The 'file' field MUST be interpreted next (if the 'option overload' option indicates that the 'file' field contains DHCP options), followed by the 'sname' field."
- **Today**: the DHCP server plugin's readers (`parseMsgType`, `parseOptionAddr`, `parseOptionBytes`, `parseOptionString` in `handler.go`) each walk `pkt[240:]` only; option 52 is never read.
- **Missing**: one option iterator over options, then file (if 52 says 1 or 3), then sname (if 2 or 3); all four readers use it.
- **Proof**: unit tests with hand-built packets carrying option 53 / 50 in file and sname under each overload value, and a negative test where file holds bytes but 52 is absent.

### 11. RFC7950-7.2.2-1 (L2, 1 d)
- **Requirement** (RFC 7950 Section 7.2.2, MUST): "A submodule MUST only be included by either the module to which it belongs or another submodule that belongs to that module."
- **Today**: the config component's YANG loader (`loader.go::Resolve`, `loader_structure.go::checkStructure`) never compares an include with the submodule's belongs-to; goyang accepts the violation.
- **Missing**: a `checkStructure` rule over every include.
- **Proof**: loader unit test with an embedded fixture module including a foreign submodule (refused, error names both modules) and a valid one (accepted).

### 12. RFC7432-8.2.1-4 (L2, 1.5 d)
- **Requirement** (RFC 7432 Section 8.2.1, MUST): "If All-Active redundancy mode is desired, then the "Single-Active" bit in the flags of the ESI Label extended community MUST be set to 0 and the MPLS label in that Extended Community MUST be set to a valid MPLS label value."
- **Today**: the EVPN plugin carries configured ESI Label bytes (`config.go::parseConfigRoute`, `encode.go::l2vpnRouteToEVPNParams`); `message/update_build_evpn.go::ValidateEVPNOrigination` checks the community is present, not its flag or label. No redundancy-mode setting exists.
- **Missing**: a redundancy-mode leaf (all-active / single-active) on the originated Ethernet A-D route, and origination validation of the flag and label against it.
- **Proof**: unit tests on `ValidateEVPNOrigination` with both modes and wrong flag/label; `.ci` refusal message.

### 13. DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-6-1 (L2, 1.5 d)
- **Requirement** (draft-ietf-idr-linklocal-capability Section 6, MUST): "If the Next Hop field is malformed, the implementation MUST handle the malformed UPDATE message using the approach of "treat-as- withdraw", as described in section 7.3 of [RFC7606]."
- **Today**: the BGP message package's `rfc7606.go::validateMPReachNextHop` checks the length (session reset on bad length, correct) but not the contents of a 32-octet field; the capability lives in the `llnh` plugin and the validator takes only `isIBGP`, `asn4` and an ADD-PATH callback.
- **Missing**: pass "capability 77 negotiated" into the validator (like `addPathFor`) and treat-as-withdraw a 32-octet next hop that is not Global then Link-Local (fe80::/10).
- **Proof**: validator unit tests with hex (global+LL accepted; LL+global, global+global withdrawn); `.ci` on a session with the capability.

### 14. DRAFT-IETF-BESS-MUP-SAFI-3.3.6-5 (L2, 1.5 d)
- **Requirement** (draft-ietf-bess-mup-safi Section 3.3.6, MUST): "When a BGP speaker receives a MP_REACH_NLRI attribute update message with a Direct Segment Discovery route without a prefix SID attribute, than it MUST be treated as if it contained a malformed prefix SID attribute and the "Treat-as-withdraw procedure of [RFC7606] is applied."
- **Today**: the MUP plugin's `rfc7606.go::recognizerFor` runs `RecognizeNLRI` and `ValidateNLRI` on one NLRI with no path attributes; no UPDATE-level rule for MUP exists in session validation.
- **Missing**: an UPDATE-level rule (attributes plus MP_REACH family and route type) that withdraws DSD routes when attribute 40 is absent.
- **Proof**: unit test with hex UPDATE; extend `bgp-mup-malformed-t1st-withdraw-gobgp` style scenario with a DSD route without Prefix-SID.

### 15. RFC5216-5.3-5 (L2, 1.5 d)
- **Requirement** (RFC 5216 Section 5.3, SHOULD): "including the ability to retrieve intermediate certificates that may be necessary to validate the peer certificate."
- **Today**: the EAP core's `eap_tls.go::newTLSMethod` validates against `ClientCAs` with only the intermediates the peer sends; no configured pool, no AIA fetch.
- **Missing**: a configured intermediate pool (PKI certificate references) used in verification; AIA fetching is optional and not proposed.
- **Proof**: unit test with a three-level chain where the peer omits the intermediate (fails today, passes with the pool); interop-ipsec EAP-TLS scenario with a leaf-only client.

### 16 and 17. RFC9568-7.2-5, RFC9568-5.2.8-2 (L2, 1.5 d together, owner-ruling)
- **Requirement** (RFC 9568 Section 7.2, MUST): "* Compute the VRRP checksum"; (Section 5.2.8, MUST): "For the IPv4 address family, the checksum calculation only includes the VRRP message starting with the Version field and ending after the last IPv4 address (refer to Section 5.2)."
- **Today**: the VRRP plugin's `packet/checksum.go::FillChecksum` uses the RFC 5798 pseudo-header for IPv4 (`pseudoSumV4Legacy`) and `verifyReceived` accepts both forms, a disclosed deviation for keepalived interop.
- **Missing**: a per-group checksum-mode leaf (rfc9568 default, rfc5798-compatible opt-in) or the owner accepting that keepalived peers break; receive must then accept only the configured form.
- **Gate**: owner ruling on the keepalived deviation.
- **Proof**: golden IPv4 advert test for each mode; `vrrp-mastership-keepalived` interop in compatible mode stays green.

### 18. RFC4271-8.2.2-20 (L3, 2 d)
- **Requirement** (RFC 4271 Section 8.2.2, MUST): "or an AutomaticStart event (Event 3), the local system: - initializes all BGP resources for the peer connection, - sets ConnectRetryCounter to zero".
- **Today**: the BGP FSM (`fsm/state.go`) declares no Event 3; a peer that restarts itself fires Event 6 (`reactor/session.go::startDamped`), which keeps the counter.
- **Missing**: Event 3 in the FSM, a DampPeerOscillations attribute (config) that chooses Event 3 vs Event 6 on automatic restart, and the Idle action list for Event 3.
- **Proof**: FSM unit test (Event 3 resets the counter, Event 6 keeps it); functional `.ci` restart scenario.

### 19. RFC7606-5.1-1 (L3, 3 d)
- **Requirement** (RFC 7606 Section 5.1, MUST): "o The MP_REACH_NLRI or MP_UNREACH_NLRI attribute (if present) SHALL be encoded as the very first path attribute in an UPDATE message."
- **Today**: the BGP wire code puts MP_UNREACH first and MP_REACH at its type-code position (`core/bgp/attribute/origin.go::OrderAttributes`) or last (`wireu/split.go::buildUpdatePayload`); `docs/architecture/wire/mp-nlri-ordering.md` documents the divergence.
- **Missing**: MP_REACH first in every builder (split, ordered writer, forward path that rewrites), keeping the three builders byte-identical; golden hex in many `.ci` files changes.
- **Proof**: `TestRFC7606Section51MPAttributeEncodedFirst` turns green; `.ci` golden updates; `bgp-rfc7606-relay-shape-frr` interop still passes.

### 20. DRAFT-IETF-BESS-MUP-SAFI-3.3.6-4 (L3, 3 d, after #14)
- **Requirement** (draft-ietf-bess-mup-safi Section 3.3.6, MUST): "If the result of the match is not identical then the receiving BGP speaker MUST consider it as a malformed NLRI and the "Treat-as-withdraw procedure of [RFC7606] is applied."
- **Today**: as #14; no BGP code reads the Prefix-SID SRv6 locator or compares it with the DSD next hop.
- **Missing**: extract the locator from the SRv6 Service TLV of attribute 40, compare with the MP_REACH next hop, withdraw on mismatch.
- **Proof**: unit tests with hex (match kept, mismatch withdrawn); GoBGP scenario if GoBGP can originate DSD.

### 21. RFC9136-3.1-4 (L3, 3 d)
- **Requirement** (RFC 9136 Section 3.1, MUST): "* The Ethernet Segment Identifier MUST be a non-zero 10-octet identifier if the ESI is used as an Overlay Index (see the definition of "Overlay Index" in Section 3.2). It MUST be all bytes zero otherwise."
- **Today**: the EVPN plugin copies the ESI octets (`types.go::parseEVPNType5`, `EVPNType5.WriteTo`, `encode.go::l2vpnRouteToEVPNParams`); no overlay-index concept.
- **Missing**: an overlay-index selector on Type 5 origination (none / esi / gateway-ip) with origination validation (non-zero ESI only when esi is the overlay index, zero otherwise).
- **Proof**: origination unit tests per selector; `bgp-evpn-frr` interop with an ESI overlay index route.

### 22. RFC4659-3.2.1.1-3 (L3, 4 d)
- **Requirement** (RFC 4659 Section 3.2.1.1, SHALL): "The link-local address shall be included in the Next Hop field if and only if the advertising BGP speaker shares a common subnet with the peer the route is being advertised to [BGP-IPv6]."
- **Today**: the BGP message builder `update_build_vpn.go::buildMPReachVPN` emits one RD plus one address (length 24); `reactor/peer_static_routes.go::buildStaticRouteUpdateNew` has a link-local argument the VPN branch drops.
- **Missing**: the 48-octet form (RD+global, RD+link-local) and the common-subnet decision, reusing the RFC 2545 egress logic in `reactor/link_scope.go`.
- **Proof**: encoder unit tests with hex; `bgp-vpn-frr` interop with a VPNv6 route over a shared subnet asserting FRR sees the link-local.

### 23 and 24. RFC9552-5.3.2.7-1, RFC7752-3.3.2.7-1 (L3, 3 d together, scope)
- **Requirement** (RFC 9552 Section 5.3.2.7, RECOMMENDED): "This symbolic name can be the FQDN for the link, a substring of the FQDN, or any string that an operator wants to use for the link. The use of the FQDN or a substring of it is strongly RECOMMENDED." (RFC 7752 Section 3.3.2.7 says the same and is superseded.)
- **Today**: the BGP-LS plugin decodes a received Link Name (`nlri/ls/attr_link.go::decodeLinkName`) and the exporter copies supplied TLVs (`ls_export/export_encode.go::originateAttributes`); no native name is derived.
- **Missing**: originate Link Name for local links as `<hostname>.<domain>` plus interface name (operator override per link).
- **Proof**: exporter unit test asserting the TLV bytes; `ospf-debug-te-frr` or a BGP-LS collector scenario showing the name.

### 25. RFC9256-4-7 (L3, 4 d, scope)
- **Requirement** (RFC 9256 Section 4, MAY): "The SR Algorithm (refer to Section 3.1.1 of [RFC8402]), the SRv6 SID behavior (as defined in [RFC8986] or other SRv6 specifications), and structure (as defined in [RFC8986]) MAY also be provided."
- **Today**: the SR Policy plugin's `config.go::parseSegmentList` accepts type-a and type-b only and `buildSegmentSubSubTLV` emits only A and B.
- **Missing**: segment types I, J and K (RFC 9830 encodings) with their optional algorithm, behavior and structure fields.
- **Proof**: encoder unit tests with hex per type; `bgp-sr-policy-gobgp` interop if GoBGP decodes them.

### 26. RFC4303-1-2 (L3, 3 d)
- **Requirement** (RFC 4303 Section 1, MUST): "Integrity-only ESP MUST be offered as a service selection option, e.g., it must be negotiable in SA management protocols".
- **Today**: the IKE component's ESP vocabulary (`ipsec/types.go::ParseEncryptionAlgo`) has no null cipher, `engine/initiator.go::resolveESPTransforms` resolves registered ciphers only, and the matcher refuses ENCR_NULL; the XFRM dataplane already knows `ecb(cipher_null)` (used for OSPFv3 manual SAs).
- **Missing**: `null` in the ESP vocabulary, the proposal matcher accepting ENCR_NULL only with a non-null integrity transform (RFC 7296 still refuses null+null), installation through XFRM and VPP.
- **Proof**: matcher unit tests (null+sha256 accepted, null+none refused); interop-ipsec scenario negotiating ENCR_NULL with strongSwan.

### 27. RFC4301-4.4.2-2 (L3, 4 d)
- **Requirement** (RFC 4301 Section 4.4.2, MUST): "the entry for an inbound SA in the SAD MUST be initially populated with the value or values negotiated at the time the SA was created."
- **Today**: `engine/child.go::installChildSA` installs one inbound policy and one transport-mode state selector from `Selectors[0]` (`selectorProto`, `selectorPort`); `ts_narrow.go` records every accepted pair in `NegotiatedPairs`.
- **Missing**: one inbound and outbound policy per negotiated pair, all referencing the SA (XFRM allows several policies per SA; a transport-mode state selector may need to widen to the union or be cleared).
- **Proof**: unit test with two TS pairs asserting two policies; interop-ipsec scenario with strongSwan proposing two subnets.

### 28. RFC7296-2.21.4-8 (L3, 4 d)
- **Requirement** (RFC 7296 Section 2.21.4, SHOULD): "A node receiving a suspicious message from an IP address (and port, if NAT traversal is used) with which it has an IKE SA SHOULD send an IKE Notify payload in an IKE INFORMATIONAL exchange over that SA."
- **Today**: the IKE engine's `inbound.go::handleOwnedInbound` counts and drops suspicious messages; `notify_error.go::answerOutOfSA` sends only the unprotected INVALID_IKE_SPI.
- **Missing**: look up the IKE SA by source address (and port), start a rate-limited protected INFORMATIONAL with the matching Notify.
- **Proof**: engine unit test with a fake peer; interop-ipsec scenario injecting a bad-SPI packet from the peer address.

### 29. RFC4301-4.4.2.1-3 (L3, 4 d, after byte lifetimes)
- **Requirement** (RFC 4301 Section 4.4.2.1, MUST): "Note that implementations MUST be able to handle having the counters at the ends of an SA get out of synch, e.g., because of packet loss or because the implementations at each end of the SA aren't doing things the same way."
- **Today**: `engine/rekey.go::newLifetimeState` sets time lifetimes only; `dataplane/xfrm_linux.go::xfrmStateFromParams` installs no byte limit.
- **Missing**: byte-count lifetimes (RFC4301-4.4.2.1-2, `plan/spec-ipsec-lifetime-volume.md`), then rekey that tolerates the peer expiring first or later (accept peer DELETE/rekey at any byte count, never require agreement).
- **Proof**: unit test with soft/hard byte expiry from the kernel and a peer rekey arriving early; interop-ipsec scenario with a small byte lifetime on one side only.

### 30. RFC3101-3.1-5 (L3, 3 d)
- **Requirement** (RFC 3101 Section 3.1, MUST): "An NSSA border router whose NSSA's NSSATranslatorRole is set to Candidate must maintain a list of the NSSA's border routers that are reachable both over the NSSA and as ASBRs over the AS's transit topology."
- **Today**: the OSPF plugin's `nssa.go::electNSSATranslator` / `nssaABRs` list B-bit routers the NSSA SPF reaches; transit ASBR reachability is never checked (gap demo `TestRFC3101BorderRouterNotReachedAsASBROverTransitIsNotListed`).
- **Missing**: intersect with routers reachable as ASBRs in the backbone/transit SPF result.
- **Proof**: the gap demo turns green; `ospf-stub-nssa-frr` interop with two NSSA ABRs.

### 31. RFC3101-3.1-4 (L3, 3 d, owner-ruling, after #30)
- **Requirement** (RFC 3101 Section 3.1, MUST): "If there exists another border router in this list whose router-LSA has bit Nt set or who has a higher router ID, then its NSSATranslatorState is disabled."
- **Today**: `instance.go::setConfig` sets Nt on every non-`never` NSSA border router; `nssa.go` disables a candidate only for Nt AND higher router ID; `TestOSPFNSSANonCandidateDoesNotWedge` pins the chosen behavior.
- **Missing**: Nt only for the Always role; list all B-bit routers; disable on Nt OR higher router ID. Reverses the wedge-avoidance deviation.
- **Gate**: owner ruling on the disclosed deviation.
- **Proof**: `TestRFC3101HigherRouterIDWithoutNtDisablesCandidate` turns green; `ospf-stub-nssa-frr` interop with FRR as Always translator.

### 32. RFC7950-7.6.5-2 (L3, 3 d)
- **Requirement** (RFC 7950 Section 7.6.5, MUST): "Otherwise, if this ancestor is a case node, the leaf MUST exist if any node from the case exists in the data tree."
- **Today**: the config component's `yang/validator.go::walkTree` looks children up only in the container's `Dir`, and `validator_mandatory.go::requireMandatoryUnderAbsent` stops at a choice; live instance: a GRE tunnel without `remote ip` passes.
- **Missing**: walkTree descends through choice and case entries; mandatory check per present case.
- **Proof**: validator unit tests; `.ci` refusing the GRE tunnel without remote ip.

### 33. RFC7950-7.6.1-2 (L3, 3 d, after #32)
- **Requirement** (RFC 7950 Section 7.6.1, MUST): "Otherwise, if this ancestor is a case node, the default value MUST be used if any node from the case exists in the data tree or the case node is the choice's default case, and if no nodes from any other case exist in the data tree."
- **Today**: `yang_schema.go::flattenChoiceCases` drops case nodes, so `schema_defaults.go::ApplyDefaults` fills every case's defaults.
- **Missing**: keep the case in the schema model and apply defaults only for the active or default case.
- **Proof**: unit tests on ApplyDefaults with two cases; `.ci` showing `show` output with only the active case's defaults.

### 34. RFC7950-9.1-2 (L3, 5 d)
- **Requirement** (RFC 7950 Section 9.1, MUST): "If the data type does not have a canonical form, the format of the value MUST match the data type's lexical representation, but the exact format is implementation dependent."
- **Today**: identityref and instance-identifier are absent from the config validator.
- **Missing**: identityref support (lexical `prefix:name`, base derivation, completion and CLI); instance-identifier at least lexically validated (`plan/spec-config-yang-type-identityref.md`).
- **Proof**: validator unit tests per type; `.ci` with a module using identityref.

### 35. RFC7950-11-3 (L3, 4 d)
- **Requirement** (RFC 7950 Section 11, MUST NOT): "In statements that have any data definition statements as substatements, those data definition substatements MUST NOT be reordered."
- **Today**: `yang/loader.go::LoadEmbedded` parses embedded modules with no comparison to a previous revision.
- **Missing**: a repo check (`./le` gate) that compares each embedded module against its last committed revision and refuses reordered data definitions (`plan/spec-config-yang-authoring-checks.md`).
- **Proof**: unit tests of the comparator over fixture pairs; the gate wired into verify.

### 36 to 38. RFC7950-8.1-4, RFC7950-8.1-1, RFC7950-8.3.3-2 (L4, 2 w together)
- **Requirement** (RFC 7950 Section 8.1, MUST): "All key leafs MUST be present for all list entries. o Nodes MUST be present for at most one case branch in all choices. ... o All "unique" constraints on lists MUST be satisfied. ..." (row quote in full in `rfc/short/rfc7950.md`); 8.1-1: "If the constraint is defined on configuration data, it MUST be true in a valid configuration data tree. ..."; 8.3.3-2: "If the datastore is "running" or "startup", these constraints MUST be enforced at the end of the <edit-config> or <copy-config> operation."
- **Today**: `walkTree` evaluates no when, unique or choice exclusivity; `validator_mandatory.go::mandatoryPresent` accepts a choice at the first case; `ze-iface-conf` declares `unique`. All commit surfaces (LoadConfig, `ze config validate`, web `validateStagedTree`, SSH `sectionErrors`) already reach `ValidateCustomSections`.
- **Missing**: unique evaluation and choice exclusivity (L3 parts), plus `when` (needs an XPath evaluator, the L4 part); 8.3.3-2 closes once these run inside `ValidateCustomSections`.
- **Proof**: validator unit tests per constraint; `.ci` refusals through `ze config validate` and an editor commit `.et` test.

### 39. RFC7950-7.21.5-2 (L4, with #36)
- **Requirement** (RFC 7950 Section 7.21.5, MUST): "If the XPath expression references any node that also has associated "when" statements, those "when" expressions MUST be evaluated first. There MUST NOT be any circular dependencies among "when" expressions."
- **Today**: no when evaluation; Ze modules declare no `when` today.
- **Missing**: dependency-ordered when evaluation and a load-time cycle refusal.
- **Proof**: unit tests with fixture modules (ordered dependency, cycle refused at load).

### 40. RFC7950-7.6.1-4 (L4, 2 d after #39)
- **Requirement** (RFC 7950 Section 7.6.1, MUST): "... Note that if the leaf or any of its ancestors has a "when" condition or "if-feature" expression that evaluates to "false", then the default value is not in use. When the default value is in use, the server MUST operationally behave as if the leaf was present in the data tree with the default value as its value."
- **Today**: `ApplyDefaults` evaluates no when and Ze has no if-feature.
- **Missing**: ApplyDefaults consults the when result (and if-feature once features exist).
- **Proof**: unit test on ApplyDefaults with a false when.

### 41. RFC4271-8.2.2-13 (L4, 1.5 w, scope, after #18)
- **Requirement** (RFC 4271 Section 8.2.2, MUST): "If the local system receives a TcpConnectionFails event (Event 18), the local system: - restarts the ConnectRetryTimer (with the initial value), - stops and clears the DelayOpenTimer (sets the value to zero), - releases all BGP resource, - increments the ConnectRetryCounter by 1".
- **Today**: `fsm/fsm.go::handleActive` goes straight to OpenSent; no DelayOpen, so a connection is never held in Active.
- **Missing**: the optional DelayOpen feature: timer, attribute, Active/Connect holding, Events 12/16/17/18 paths (`plan/spec-bgp-fsm-delayopen-and-automatic-start.md`, status ready).
- **Proof**: FSM unit tests per event; `.ci`; FRR interop with `delay-open` on one side.

### 42. RFC9252-5-2 (L4, 2 w)
- **Requirement** (RFC 9252 Section 5, MUST): "Therefore, the ingress PE MUST perform a resolvability check for the SRv6 Service SID before considering the received prefix for the BGP best path computation."
- **Today**: the RIB plugin's `rib_bestchange.go::isSRv6Ineligible` gates on SID validity only; the sysrib's `fibEntry` checks resolvability after selection and only declines the FIB write.
- **Missing**: locator reachability fed into best-path eligibility, with re-evaluation when reachability changes (cross-component: sysrib to RIB) (`plan/immediate/spec-srv6-bestpath-resolvability.md`).
- **Proof**: RIB unit test (unresolvable SID loses, second path wins); `bgp-srv6-frr` interop with an unreachable locator.

### 43. DRAFT-IETF-IDR-LINKLOCAL-CAPABILITY-1-2 (L4, 2 w)
- **Requirement** (draft-ietf-idr-linklocal-capability Section 1, MUST): "Since IPv6 Link-Local addresses are not required to be globally unique, implementations must ensure that they are strictly associated with a specific interface."
- **Today**: `rib_bestchange.go::mpNextHopAddr` produces a zoneless address; best-change events name no interface; `config_nexthop_form.go::applyLinkLocal` parses `session link-local` with no tie to the session interface.
- **Missing**: zone/interface carried from the session into received link-local next hops, through the best-change event into sysrib and the FIB; send side derives or validates the link-local against the session interface.
- **Proof**: RIB and sysrib unit tests asserting the interface; `bgp-linklocal-only-multihop-withdraw-frr` style interop with two links sharing one fe80 address.

### 44. RFC2545-3-6 (L4, 2 w)
- **Requirement** (RFC 2545 Section 3, SHALL): "The link-local address shall be included in the Next Hop field if and only if the BGP speaker shares a common subnet with the entity identified by the global IPv6 address carried in the Network Address of Next Hop field and the peer the route is being advertised to."
- **Today**: `reactor/link_scope.go::linkLocalNextHop` returns no address for a third-party next hop (`classify` says ThirdParty) when only its global address is known.
- **Missing**: a source for the third-party router's link-local (neighbor table lookup through the iface component), with the liveness question decided.
- **Proof**: unit test with a fake neighbor table; `bgp-rfc2545-linklocal-nexthop-frr` interop with a third-party next hop.

### 45. RFC8277-3.2.2-3 (L4, 2 w, scope)
- **Requirement** (RFC 8277 Section 3.2.2, MUST): "It can decide not to propagate the route to the given peer. In that case, if a previous route with the same AFI, SAFI, and prefix (but with fewer labels) has already been propagated to that peer, that route MUST be withdrawn from that peer using the procedure of Section 2.4."
- **Today**: `filter_delta_handlers.go::mpReachNextHopHandler` keeps the whole NLRI; `update_build_labeled.go::BuildLabeledUnicastNLRIBytes` writes the supplied stack; no Multiple Labels Capability.
- **Missing**: the Multiple Labels Capability (count per family), a per-recipient decision on label count, and the paired withdrawal; depends on RFC8277-3.2.1-4.
- **Proof**: reactor unit test of the withdraw; `bgp-labeled-withdraw-compatibility-frr` interop.

### 46. RFC9086-7-2 (L4, 3 w, scope)
- **Requirement** (RFC 9086 Section 7, MUST): "PeerAdj SID, and PeerSet SID as well as control of which information is advertised to which internal or external peer."
- **Today**: `epe/epe_config.go::ParseConfig` supports PeerNode only; `ls_export/export_config.go::parseExportConfig` has no per-recipient control.
- **Missing**: PeerAdj and PeerSet SID allocation and advertisement, and per-collector filters (`plan/spec-bgp-epe-peer-segments-and-srv6-peernode.md`, status ready).
- **Proof**: exporter unit tests with hex TLVs; BGP-LS collector interop.

### 47 to 49. RFC9012-13-16, RFC9012-3.7-2, RFC9012-4.2-1 (L4, 3 w plus 2 d each, scope)
- **Requirement** (RFC 9012 Section 13, MUST): "... Sub-TLVs of this sort MUST be disregarded."; (Section 3.7): "If included in a BGP UPDATE for any other address family, it MUST be ignored."; (Section 4.2): "In case of such a conflict, the information in the Router's MAC Extended Community MUST be used."
- **Today**: attribute 23 is carried opaquely (`core/bgp/attribute/tunnel_encap.go::SubTLVs`, `reactor/session_tunnel_encap.go::tunnelTLVLayout`); `rib_bestchange.go::checkRouteBestChange` exports no tunnel parameters.
- **Missing**: the attribute-driven encapsulation consumer (RIB to sysrib to FIB/VPP encap), then the three rules inside it (`plan/spec-bgp-tunnel-encapsulation-consumption.md`).
- **Proof**: consumer unit tests for each rule; FRR or GoBGP interop with VXLAN encapsulation attribute and installed encap route.

### 50 and 51. RFC4301-5.2-1, RFC4301-5.2-9 (L4, 2 w together)
- **Requirement** (RFC 4301 Section 4.4.1, MUST): "Since the SPD-I is just a part of the SPD, if a packet that is looked up in the SPD-I cannot be matched to an entry there, then the packet MUST be discarded."; (Section 5.2): "(No SAs are created in response to receipt of a packet that requires IPsec protection; only BYPASS or DISCARD cache entries can be created this way.) If there is no match, discard the traffic."
- **Today**: where `installUnmatched` installed the catch-all, every packet matches; three paths have no entry: VPP (`dataplane/vpp_policy.go::vppPolicyInterface` refuses IfIndex 0, so no catch-all under the default bypass), before first apply and after exit (`engine/apply.go`, `removeUnmatched`), and tolerated bypass install failures or no XFRM.
- **Missing**: a node-wide VPP SPD binding, a catch-all installed at engine start before the first apply and kept on exit until removal is safe, and install failure that fails the apply in both modes (`plan/immediate/spec-rfc4301-architecture-gaps.md`).
- **Proof**: engine unit tests per path; VPP integration test; interop-ipsec scenario with a clear packet before first apply.

### 52. RFC4301-5.1-7 (L4, 3 w, scope)
- **Requirement** (RFC 4301 Section 5.1, MUST): "If the SPD entry calls for PROTECT, i.e., creation of an SA, the key management mechanism (e.g., IKEv2) is invoked to create the SA. If SA creation succeeds, a new outbound (SPD-S) cache entry is created, along with outbound and inbound SAD entries, otherwise the packet is discarded."
- **Today**: PROTECT is installed only beside a negotiated Child SA (`engine/child.go::installChildSA`); `ipsec/spd_policy.go::parseSPDPolicy` refuses operator protect entries; nothing subscribes to XFRM acquire.
- **Missing**: operator PROTECT entries installed as trap policies, an XFRM acquire listener that starts IKE/Child SA creation, discard while pending; VPP equivalent.
- **Proof**: engine unit test with a fake acquire; interop-ipsec scenario where traffic triggers the SA with strongSwan.

### 53. RFC3948-5.1-1 (L4, 2 w)
- **Requirement** (RFC 3948 Section 5.1, MUST): "Because SGW will now see two possible SAs that lead to 10.1.2.3, it can become confused about where to send packets coming from Suzy's server. Implementors MUST devise ways of preventing this from occurring."
- **Today**: `core/eap/pool.go::Allocate` exists with no non-test caller; `engine/apply.go::reloadPool` stores a pool nothing reads; no CP payload is built.
- **Missing**: CFG_REQUEST/CFG_REPLY handling assigning a unique inner address, release on SA deletion, and a refusal for overlapping self-chosen addresses (`plan/immediate/spec-ike-virtual-ip-assignment.md`).
- **Proof**: engine unit tests; interop-ipsec scenario with two strongSwan clients behind one NAT.

### 54. RFC3209-4.6.1-2 (L4, 3 w, scope)
- **Requirement** (RFC 3209 Section 4.6.1.2, MUST): the 16 bits before the Tunnel ID in the LSP_TUNNEL_IPv6 SESSION object "MUST be zero" (row quote is the packet diagram).
- **Today**: the RSVP-TE plugin's `wire.go::encodeSessionIPv4` is the only SESSION encoder; `CTypeLSPTunnelIPv6` is unused.
- **Missing**: IPv6 LSP tunnels: SESSION, SENDER_TEMPLATE, FILTER_SPEC, RRO/ERO IPv6 forms and the transit path (`plan/spec-rsvpte-ipv6-lsp-tunnel.md`).
- **Proof**: encoder unit test with hex; `interop-rsvpte` scenario with freeRouter over IPv6.

### 55. RFC2328-8.1-1 (L4, 2 w, owner-ruling 8 (g))
- **Requirement** (RFC 2328 Section 8.1, MUST): "On these interfaces, the IP source should be set to any of the other IP addresses belonging to the router. For this reason, there must be at least one IP address assigned to the router."
- **Today**: the OSPF plugin's `transport/backend_linux.go::interfaceIPv4` requires an interface IPv4 and refuses one without; unnumbered point-to-point is not implemented (owner ruling 8 (g)).
- **Missing**: unnumbered P2P: config, source selection from another router address, router-LSA link data as ifIndex, SPF next hop via interface, FIB with interface-only next hop.
- **Proof**: unit tests on LSA origination; `ospf-p2p-frr` variant with `ip unnumbered`.

### 56. RFC5392-3.2.1-5 (L4, 3 w, scope)
- **Requirement** (RFC 5392 Section 3.2.1, MUST): "The Remote-AS-Number sub-TLV MUST be included in the Link TLV of both the Inter-AS-TE-v2 LSA and Inter-AS-TE-v3 LSA."
- **Today**: the OSPF plugin registers inter-AS TE only as an OSPFv2 opaque type-6 consumer (`te.go::registerTEConsumer`); no OSPFv3 Inter-AS-TE-v3 LSA.
- **Missing**: OSPFv3 Inter-AS-TE-v3 LSA (function code 13) origination and flooding with its Link TLV and sub-TLVs.
- **Proof**: encoder unit tests with hex; `ospf-te-interas-frr` extended to OSPFv3 if FRR supports it.

### 57. SFLOW-V5-4.3-1 (L5, 6 w+, scope)
- **Requirement** (sFlow v5 Section 4.3, MUST): "The sFlowAgent address must provide SNMP connectivity to the agent."
- **Today**: `sflow/register.go::parseAgentAddress` checks the address; no SNMP agent exists anywhere in Ze.
- **Missing**: an SNMP agent (at least SNMPv2c/v3 with the sFlow MIB and ifTable) answering at the agent address.
- **Proof**: unit tests of the agent; interop with net-snmp `snmpwalk` and an sFlow collector.

### 58. RFC4659-8-5 (L5, 6 w+, scope, after #22)
- **Requirement** (RFC 4659 Section 8, SHALL): "When the VPN-IPv6 traffic is to be transported using IPv6 tunneling, the BGP Next Hop Field SHALL contain an IPv6 address."
- **Today**: generic VPNv6 origination and forwarding exist (`buildMPReachVPN`, `mpReachNextHopHandler`); no inter-AS option (b) ASBR: no local label binding, VPN routes not mirrored to Loc-RIB (`checkRouteBestChange`) nor installed by sysrib.
- **Missing**: option (b) ASBR: label allocation, MPLS swap entries, VPN route installation, IPv6 tunnel transport.
- **Proof**: two-AS lab with FRR ASBRs and an MPLS dataplane check.

### 59. RFC4303-3.4.4.1-2 (L5, external)
- **Requirement** (RFC 4303 Section 3.4.4.1, SHOULD): "If the default padding scheme (see Section 2.4) has been employed, the receiver SHOULD inspect the Padding field before removing the padding prior to passing the decrypted data to the next layer."
- **Today**: ESP runs in Linux XFRM (`dataplane/xfrm_linux.go::xfrmStateFromParams`) or VPP (`dataplane/vpp.go::InstallSA`, `vppSAFlags`); neither exposes a padding-inspection knob.
- **Missing**: padding inspection in the kernel or VPP ESP decap path; Ze cannot do it from Go without moving ESP processing into Ze.
- **Proof**: a kernel or VPP patch with a test sending bad padding and observing the drop counter; Ze side only an integration test reading that counter.

## Suggested waves

1. Wave 1 (L1/L2, no gate, about 12 d): #1, #3 to #15. Each is one commit.
2. Wave 2 (owner rulings needed first): #2, #16/#17, #31, #55. Ask the owner in one message.
3. Wave 3 (L3, no gate): #18 to #22, #26 to #30, #32 to #35.
4. Wave 4 (YANG when/unique/choice bundle #36 to #40), then the L4 rows with existing ready specs (#41, #42, #46, #47 to #49, #50/#51, #53, #54).
5. Wave 5 (scope decisions): #23/#24, #25, #45, #52, #56, #57, #58; #59 is outside Ze.
