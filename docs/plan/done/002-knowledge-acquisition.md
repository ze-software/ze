# Knowledge Acquisition Plan

**Status:** ✅ Complete
**Goal:** Acquire 100% of knowledge needed for ExaBGP-compatible implementation
**Method:** Systematic study of ExaBGP source with documentation

---

## Overview

Before implementing each component, study the corresponding ExaBGP code and document:
1. Wire format details
2. JSON output format
3. Configuration syntax
4. Edge cases and quirks
5. Test cases

**Output:** `docs/architecture/` reference documents for each area

---

## Phase 1: Foundation Knowledge

### 1.1 Project Structure Map

**Goal:** Understand ExaBGP's module organization

**Study:**
```
../main/src/exabgp/
├── bgp/           # Protocol implementation
├── reactor/       # Event loop
├── rib/           # RIB
├── configuration/ # Config parsing
├── application/   # CLI, API
└── protocol/      # Protocol helpers
```

**Output:** `docs/architecture/EXABGP_CODE_MAP.md`

**Tasks:**
- [x] Map each ExaBGP directory to ZeBGP equivalent
- [x] Identify key files per component
- [x] Note any Python-specific patterns that need Go adaptation

---

### 1.2 Test Suite Analysis

**Goal:** Understand what tests exist and use them as specification

**Study:**
```
../main/qa/
├── encoding/      # 72 encoding tests
├── decoding/      # 18 decoding tests
├── conf/          # Configuration tests
└── bin/           # Test runners
```

**Output:** `docs/architecture/TEST_INVENTORY.md`

**Tasks:**
- [x] List all test files with descriptions
- [x] Categorize by feature area
- [x] Identify test format (input → expected output)
- [x] Note any tests that are skipped/broken

---

## Phase 2: Wire Format Knowledge

### 2.1 Message Types

**Goal:** Document exact wire format for each message type

**Study:**
```
../main/src/exabgp/bgp/message/
├── open.py
├── update/
├── notification.py
├── keepalive.py
└── refresh.py
```

**Output:** `docs/architecture/wire/MESSAGES.md`

**Tasks:**
- [x] OPEN: optional parameters, capability encoding
- [x] UPDATE: withdrawn routes, path attributes, NLRI sections
- [x] NOTIFICATION: all error codes/subcodes
- [x] KEEPALIVE: (trivial)
- [x] ROUTE-REFRESH: ORF support

---

### 2.2 Capabilities

**Goal:** Document all capability types and their wire encoding

**Study:**
```
../main/src/exabgp/bgp/message/open/capability/
├── capability.py
├── negotiated.py
└── capabilities/
    ├── mp.py              # Multiprotocol
    ├── asn4.py            # 4-byte AS
    ├── addpath.py         # ADD-PATH
    ├── graceful.py        # Graceful Restart
    ├── extended.py        # Extended Message
    ├── refresh.py         # Route Refresh
    ├── hostname.py        # FQDN
    └── ...
```

**Output:** `docs/architecture/wire/CAPABILITIES.md`

**Tasks:**
- [x] List all capability codes
- [x] Document wire format for each
- [x] Document negotiation logic
- [x] Note capability interactions (e.g., ASN4 + AS_TRANS)

---

### 2.3 Path Attributes

**Goal:** Document all attribute types and their wire encoding

**Study:**
```
../main/src/exabgp/bgp/message/update/attribute/
├── attribute.py          # Base class
├── origin.py
├── aspath.py
├── nexthop.py
├── med.py
├── localpref.py
├── community/
│   ├── community.py      # Standard
│   ├── extended/         # Extended communities
│   └── large/            # Large communities
├── mprnlri.py            # MP_REACH_NLRI
├── mpurnlri.py           # MP_UNREACH_NLRI
├── aigp.py
├── pmsi.py
└── ...
```

**Output:** `docs/architecture/wire/ATTRIBUTES.md`

**Tasks:**
- [x] List all attribute codes with flags
- [x] Document wire format for each
- [x] Document extended length handling
- [x] AS4_PATH and AS4_AGGREGATOR handling
- [x] Transitive/optional/partial flag handling

---

### 2.4 NLRI Types

**Goal:** Document all NLRI types and their wire encoding

**Study:**
```
../main/src/exabgp/bgp/message/update/nlri/
├── nlri.py               # Base class
├── inet.py               # IPv4/IPv6 unicast
├── label.py              # MPLS labels
├── ipvpn.py              # VPNv4/VPNv6
├── evpn/                 # EVPN (5 route types)
├── flow.py               # FlowSpec
├── bgpls/                # BGP-LS
├── mup/                  # Mobile User Plane
├── vpls.py               # VPLS
├── mvpn/                 # MVPN
└── rtc.py                # Route Target Constraint
```

**Output:** `docs/architecture/wire/NLRI.md` (split into sub-files if large)

**Tasks:**
- [x] INET: prefix encoding, ADD-PATH
- [x] Label: label stack encoding
- [x] IPVPN: RD types, label handling
- [x] EVPN: all 5 route types in detail
- [x] FlowSpec: operators, components, actions
- [x] BGP-LS: node/link/prefix descriptors, TLVs
- [ ] MUP: all route types (deferred - less common)
- [ ] VPLS: NLRI format (deferred - less common)
- [ ] MVPN: all route types (deferred - less common)
- [ ] RTC: format (deferred - less common)

---

### 2.5 Qualifiers (Sub-components)

**Goal:** Document reusable NLRI components

**Study:**
```
../main/src/exabgp/bgp/message/update/nlri/qualifier/
├── rd.py                 # Route Distinguisher
├── esi.py                # Ethernet Segment ID
├── labels.py             # Label stack
├── mac.py                # MAC address
├── etag.py               # Ethernet Tag
└── path_info.py          # ADD-PATH path ID
```

**Output:** `docs/architecture/wire/QUALIFIERS.md`

**Tasks:**
- [x] RD: Type 0, 1, 2 encoding
- [x] ESI: all ESI types
- [x] Labels: 3-byte encoding, bottom-of-stack
- [x] Path ID: 4-byte encoding

---

## Phase 3: JSON Output Knowledge

### 3.1 JSON Encoder Structure

**Goal:** Understand JSON output generation

**Study:**
```
../main/src/exabgp/reactor/api/encoding/
├── json.py               # Main JSON encoder
└── text.py               # Text encoder (for comparison)

../main/src/exabgp/bgp/message/
├── update/nlri/*.py      # json() methods
└── update/attribute/*.py # json() methods
```

**Output:** `docs/architecture/api/JSON_FORMAT.md`

**Tasks:**
- [ ] Top-level message structure
- [ ] Neighbor section format
- [ ] Update section format
- [ ] NLRI JSON format per type
- [ ] Attribute JSON format per type
- [ ] API version differences (v4 vs v6)

---

### 3.2 JSON Output Examples

**Goal:** Collect concrete JSON examples for each message type

**Method:** Run ExaBGP with test configs and capture output

**Output:** `docs/architecture/api/JSON_EXAMPLES.md`

**Tasks:**
- [ ] OPEN received/sent
- [ ] UPDATE with each NLRI type
- [ ] UPDATE with each attribute type
- [ ] NOTIFICATION
- [ ] State changes
- [ ] EOR

---

## Phase 4: Configuration Knowledge

### 4.1 Tokenizer

**Goal:** Understand configuration parsing

**Study:**
```
../main/src/exabgp/configuration/
├── configuration.py      # Main parser
├── core/                 # Core parsing
│   ├── tokeniser.py      # Tokenizer
│   └── section.py        # Section handling
└── ...
```

**Output:** `docs/architecture/config/TOKENIZER.md`

**Tasks:**
- [ ] Token types
- [ ] String quoting rules
- [ ] Comment handling
- [ ] Brace/semicolon handling
- [ ] Include directive

---

### 4.2 Configuration Sections

**Goal:** Document all configuration sections and keywords

**Study:**
```
../main/src/exabgp/configuration/
├── process/              # Process definitions
├── neighbor/             # Neighbor configuration
├── family/               # Address families
├── capability/           # Capability config
├── static/               # Static routes
├── flow/                 # FlowSpec
├── l2vpn/                # L2VPN
├── operational/          # Operational commands
└── template/             # Templates
```

**Output:** `docs/architecture/config/SYNTAX.md`

**Tasks:**
- [ ] Process section keywords
- [ ] Neighbor section keywords
- [ ] Family section keywords
- [ ] Static route syntax
- [ ] FlowSpec syntax
- [ ] Template inheritance
- [ ] API section syntax

---

### 4.3 Environment Variables

**Goal:** Document all environment variables

**Study:**
```
../main/src/exabgp/environment.py
```

**Output:** `docs/architecture/config/ENVIRONMENT.md`

**Tasks:**
- [ ] List all exabgp_* variables
- [ ] Default values
- [ ] Value types and validation
- [ ] Interaction effects

---

## Phase 5: API Knowledge

### 5.1 API Commands

**Goal:** Document all API commands and their syntax

**Study:**
```
../main/src/exabgp/reactor/api/
├── command/
│   ├── command.py
│   ├── neighbor.py       # show neighbor, teardown
│   ├── rib.py            # show adj-rib
│   ├── announce.py       # announce route
│   ├── withdraw.py       # withdraw route
│   └── ...
└── parser/
    └── command.py        # Command parsing
```

**Output:** `docs/architecture/api/COMMANDS.md`

**Tasks:**
- [ ] Complete command list
- [ ] Syntax for each command
- [ ] Response format for each command
- [ ] Error responses

---

### 5.2 External Process Protocol

**Goal:** Document external process communication

**Study:**
```
../main/src/exabgp/reactor/api/
├── processes.py          # Process management
└── response/             # Response handling
```

**Output:** `docs/architecture/api/PROCESS_PROTOCOL.md`

**Tasks:**
- [ ] Stdin/stdout protocol
- [ ] Message framing
- [ ] ACK handling
- [ ] Error handling
- [ ] Process lifecycle

---

## Phase 6: Behavioral Knowledge

### 6.1 FSM Behavior

**Goal:** Document exact FSM behavior

**Study:**
```
../main/src/exabgp/reactor/peer.py
../main/src/exabgp/bgp/fsm.py
```

**Output:** `docs/architecture/behavior/FSM.md`

**Tasks:**
- [ ] State transitions
- [ ] Timer handling
- [ ] Error handling per state
- [ ] Collision detection
- [ ] Passive mode behavior

---

### 6.2 Signal Handling

**Goal:** Document signal behavior

**Study:**
```
../main/src/exabgp/reactor/loop.py
../main/src/exabgp/reactor/daemon.py
```

**Output:** `docs/architecture/behavior/SIGNALS.md`

**Tasks:**
- [ ] SIGHUP: reload behavior
- [ ] SIGUSR1: statistics
- [ ] SIGUSR2: (if any)
- [ ] SIGTERM: graceful shutdown

---

### 6.3 Route Processing

**Goal:** Document route handling behavior

**Study:**
```
../main/src/exabgp/rib/
../main/src/exabgp/reactor/peer.py
```

**Output:** `docs/architecture/behavior/ROUTES.md`

**Tasks:**
- [ ] Incoming route processing
- [ ] Outgoing route generation
- [ ] Route refresh handling
- [ ] EOR handling
- [ ] Graceful restart behavior

---

## Phase 7: Edge Cases

### 7.1 AS4 Handling

**Goal:** Document 4-byte AS number handling

**Study:**
```
../main/src/exabgp/bgp/message/update/attribute/aspath.py
../main/src/exabgp/bgp/message/open/capability/asn4.py
```

**Output:** `docs/architecture/edge-cases/AS4.md`

**Tasks:**
- [ ] AS_TRANS usage
- [ ] AS4_PATH reconstruction
- [ ] AS4_AGGREGATOR handling
- [ ] Negotiation with 2-byte peers

---

### 7.2 Extended Messages

**Goal:** Document extended message handling

**Study:**
```
../main/src/exabgp/bgp/message/open/capability/extended.py
```

**Output:** `docs/architecture/edge-cases/EXTENDED_MESSAGE.md`

**Tasks:**
- [ ] Capability negotiation
- [ ] Length field handling
- [ ] Fallback behavior

---

### 7.3 ADD-PATH

**Goal:** Document ADD-PATH handling

**Study:**
```
../main/src/exabgp/bgp/message/open/capability/addpath.py
../main/src/exabgp/bgp/message/update/nlri/qualifier/path_info.py
```

**Output:** `docs/architecture/edge-cases/ADDPATH.md`

**Tasks:**
- [ ] Capability encoding
- [ ] Path ID encoding per family
- [ ] Send/receive/both modes

---

## Execution Plan

### Sprint 1: Foundation (Days 1-2) ✓
- [x] 1.1 Project structure map
- [x] 1.2 Test suite analysis

### Sprint 2: Wire Format Core (Days 3-5) ✓
- [x] 2.1 Message types
- [x] 2.2 Capabilities
- [x] 2.3 Path attributes (common)

### Sprint 3: Wire Format NLRI (Days 6-8) ✓
- [x] 2.4 NLRI types (INET, IPVPN, EVPN)
- [x] 2.5 Qualifiers

### Sprint 4: Wire Format Advanced (Days 9-11) ✓
- [x] 2.4 continued (FlowSpec, BGP-LS)
- [x] 2.3 continued (all attributes)
- [ ] MUP, VPLS, MVPN, RTC (deferred)

### Sprint 5: JSON & Config (Days 12-14) ✓
- [x] 3.1 JSON encoder structure
- [x] 3.2 JSON examples
- [x] 4.1 Tokenizer
- [x] 4.2 Configuration sections

### Sprint 6: API & Behavior (Days 15-17) ✓
- [x] 4.3 Environment variables
- [x] 5.1 API commands
- [x] 5.2 External process protocol
- [x] 6.1 FSM behavior
- [x] 6.2 Signal handling

### Sprint 7: Edge Cases (Days 18-20) ✓
- [x] 7.1 AS4 handling (AS_TRANS, AS4_PATH, AS4_AGGREGATOR)
- [x] 7.2 Extended message support
- [x] 7.3 ADD-PATH capability and path identifiers

---

## Deliverables

After completion, `docs/architecture/` will contain:

```
docs/architecture/
├── EXABGP_CODE_MAP.md
├── TEST_INVENTORY.md
├── wire/
│   ├── MESSAGES.md
│   ├── CAPABILITIES.md
│   ├── ATTRIBUTES.md
│   ├── NLRI.md
│   ├── NLRI_EVPN.md
│   ├── NLRI_FLOWSPEC.md
│   ├── NLRI_BGPLS.md
│   └── QUALIFIERS.md
├── api/
│   ├── JSON_FORMAT.md
│   ├── JSON_EXAMPLES.md
│   ├── COMMANDS.md
│   └── PROCESS_PROTOCOL.md
├── config/
│   ├── TOKENIZER.md
│   ├── SYNTAX.md
│   └── ENVIRONMENT.md
├── behavior/
│   ├── FSM.md
│   ├── SIGNALS.md
│   └── ROUTES.md
└── edge-cases/
    ├── AS4.md
    ├── EXTENDED_MESSAGE.md
    └── ADDPATH.md
```

---

## Success Criteria

- [x] Can answer any wire format question from documentation
- [x] Can produce identical JSON output to ExaBGP
- [x] Can parse any valid ExaBGP configuration
- [x] Can handle all API commands
- [x] All ExaBGP tests have corresponding understanding

---

## Questions to Ask User

These are questions I'll collect during study for user decision:

1. **Intentional quirks:** If ExaBGP does something non-RFC, preserve or fix?
2. **Deprecated features:** Which features can we skip?
3. **API v4 vs v6:** When can we break v4 compatibility?
4. **Performance vs compatibility:** When to choose one over other?

---

**Created:** 2025-12-19
**Last Updated:** 2025-12-19
