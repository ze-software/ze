# Spec: Hub - ZeBGP YANG Modules

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `docs/architecture/hub-architecture.md` - Hub design
4. `docs/architecture/config/yang-config-design.md` - YANG config design
5. `docs/architecture/config/syntax.md` - current config syntax

## Task

Define the YANG modules for ZeBGP's built-in configuration. These modules will be declared by the BGP engine and other internal subsystems during Stage 1.

### Goals

1. Define `ze-bgp.yang` - BGP configuration schema
2. Define `ze-rib.yang` - RIB configuration schema (if needed)
3. Define `ze-types.yang` - Common types (inet:ip-address, etc.)
4. Map existing config syntax to YANG
5. Support leafref for cross-references (peer-group, route-map, etc.)

### Non-Goals

- Full RFC-compliant BGP YANG (use simplified version)
- OpenConfig compatibility (maybe future)
- Runtime state modeling (config only)

### Dependencies

- None (can be done in parallel with Phase 1-4)
- Used by Phase 3 for testing YANG validation

## Required Reading

### Architecture Docs
- [ ] `docs/architecture/hub-architecture.md` - [YANG and leafref section]
- [ ] `docs/architecture/config/yang-config-design.md` - [design decisions]
- [ ] `docs/architecture/config/syntax.md` - [current config format]

### External Resources
- [ ] RFC 7950 - YANG 1.1 (key sections)
- [ ] OpenConfig BGP model (for reference, not compliance)

**Key insights:**
- YANG modules declare types, containers, lists, leaves
- leafref enables cross-references with validation
- Keep it simple - not trying to match OpenConfig complexity
- Modules are embedded in binaries via `//go:embed` for distribution
- Plugins send embedded YANG content during Stage 1 declaration

## Design

### Module Structure

```
yang/
├── ze-types.yang      # Common types (ip-address, asn, etc.)
├── ze-bgp.yang        # BGP configuration
├── ze-rib.yang        # RIB configuration (optional)
└── ze-plugin.yang     # Plugin configuration
```

### ze-types.yang

```yang
module ze-types {
    namespace "urn:ze:types";
    prefix zt;

    typedef ip-address {
        type union {
            type ipv4-address;
            type ipv6-address;
        }
    }

    typedef ipv4-address {
        type string {
            pattern '(([0-9]|[1-9][0-9]|1[0-9][0-9]|2[0-4][0-9]|25[0-5])\.){3}'
                  + '([0-9]|[1-9][0-9]|1[0-9][0-9]|2[0-4][0-9]|25[0-5])';
        }
    }

    typedef ipv6-address {
        type string {
            // Simplified pattern
            pattern '([0-9a-fA-F]{0,4}:){2,7}[0-9a-fA-F]{0,4}';
        }
    }

    typedef asn {
        type uint32 {
            range "1..4294967295";
        }
        description "Autonomous System Number (4-byte)";
    }

    typedef asn2 {
        type uint16 {
            range "1..65535";
        }
        description "Autonomous System Number (2-byte legacy)";
    }

    typedef port {
        type uint16 {
            range "1..65535";
        }
    }

    typedef prefix-ipv4 {
        type string {
            pattern '(([0-9]|[1-9][0-9]|1[0-9][0-9]|2[0-4][0-9]|25[0-5])\.){3}'
                  + '([0-9]|[1-9][0-9]|1[0-9][0-9]|2[0-4][0-9]|25[0-5])'
                  + '/(([0-9])|([1-2][0-9])|(3[0-2]))';
        }
    }

    typedef community {
        type string {
            pattern '[0-9]+:[0-9]+';
        }
    }
}
```

### ze-bgp.yang

```yang
module ze-bgp {
    namespace "urn:ze:bgp";
    prefix bgp;

    import ze-types { prefix zt; }

    container bgp {
        description "BGP configuration";

        leaf local-as {
            type zt:asn;
            mandatory true;
        }

        leaf router-id {
            type zt:ipv4-address;
            mandatory true;
        }

        leaf hold-time {
            type uint16 {
                range "0 | 3..65535";
            }
            default 180;
        }

        // Peer groups
        list peer-group {
            key "name";
            description "BGP peer group template";

            leaf name {
                type string {
                    length "1..64";
                }
            }

            leaf peer-as {
                type zt:asn;
            }

            uses peer-config;
        }

        // Peers
        list peer {
            key "address";
            description "BGP peer configuration";

            leaf address {
                type zt:ip-address;
            }

            leaf peer-as {
                type zt:asn;
            }

            leaf group {
                type leafref {
                    path "../../peer-group/name";
                }
                description "Reference to peer-group (must exist)";
            }

            uses peer-config;
        }

        // Policy
        list route-map {
            key "name";

            leaf name {
                type string;
            }

            list rule {
                key "sequence";

                leaf sequence {
                    type uint32;
                }

                leaf action {
                    type enumeration {
                        enum permit;
                        enum deny;
                    }
                }

                // Match conditions
                container match {
                    leaf prefix-list {
                        type leafref {
                            path "../../../prefix-list/name";
                        }
                    }
                    leaf community {
                        type zt:community;
                    }
                }

                // Set actions
                container set {
                    leaf local-pref {
                        type uint32;
                    }
                    leaf med {
                        type uint32;
                    }
                    leaf community {
                        type zt:community;
                    }
                }
            }
        }

        list prefix-list {
            key "name";

            leaf name {
                type string;
            }

            list entry {
                key "sequence";

                leaf sequence {
                    type uint32;
                }

                leaf action {
                    type enumeration {
                        enum permit;
                        enum deny;
                    }
                }

                leaf prefix {
                    type zt:prefix-ipv4;
                }

                leaf le {
                    type uint8 { range "0..32"; }
                }

                leaf ge {
                    type uint8 { range "0..32"; }
                }
            }
        }
    }

    grouping peer-config {
        description "Common peer/peer-group configuration";

        leaf passive {
            type boolean;
            default false;
        }

        leaf route-reflector-client {
            type boolean;
            default false;
        }

        container timers {
            leaf hold-time {
                type uint16 {
                    range "0 | 3..65535";
                }
            }

            leaf keepalive {
                type uint16;
            }
        }

        container capability {
            leaf route-refresh {
                type boolean;
                default true;
            }

            leaf add-path {
                type enumeration {
                    enum disabled;
                    enum receive;
                    enum send;
                    enum both;
                }
                default disabled;
            }

            leaf extended-message {
                type boolean;
                default false;
            }
        }

        leaf route-map-in {
            type leafref {
                path "../../route-map/name";
            }
        }

        leaf route-map-out {
            type leafref {
                path "../../route-map/name";
            }
        }
    }
}
```

### ze-plugin.yang

```yang
module ze-plugin {
    namespace "urn:ze:plugin";
    prefix plug;

    container plugin {
        list external {
            key "name";

            leaf name {
                type string;
            }

            leaf run {
                type string;
                mandatory true;
                description "Command to execute plugin";
            }

            leaf enabled {
                type boolean;
                default true;
            }
        }
    }
}
```

### Config Syntax Mapping

| Current Config | YANG Path | Notes |
|----------------|-----------|-------|
| `bgp { local-as 65001; }` | `/bgp/local-as` | |
| `peer 192.0.2.1 { ... }` | `/bgp/peer[address=192.0.2.1]` | |
| `peer-group upstream { ... }` | `/bgp/peer-group[name=upstream]` | |
| `group upstream;` (in peer) | `/bgp/peer[...]/group` | leafref |
| `route-map foo { ... }` | `/bgp/route-map[name=foo]` | |
| `route-map-in foo;` | `/bgp/peer[...]/route-map-in` | leafref |

## 🧪 TDD Test Plan

### Unit Tests

| Test | File | Validates | Status |
|------|------|-----------|--------|
| `TestYangTypes_IPAddress` | `internal/yang/modules_test.go` | IP address type validation | |
| `TestYangTypes_ASN` | `internal/yang/modules_test.go` | ASN range validation | |
| `TestYangTypes_Port` | `internal/yang/modules_test.go` | Port range validation | |
| `TestYangBGP_LocalAS` | `internal/yang/modules_test.go` | BGP local-as required | |
| `TestYangBGP_PeerLeafref` | `internal/yang/modules_test.go` | Peer group leafref | |
| `TestYangBGP_RouteMapLeafref` | `internal/yang/modules_test.go` | Route-map leafref | |

### Boundary Tests

| Field | Range | Last Valid | Invalid Below | Invalid Above |
|-------|-------|------------|---------------|---------------|
| ASN | 1-4294967295 | 4294967295 | 0 | 4294967296 |
| ASN2 | 1-65535 | 65535 | 0 | 65536 |
| Port | 1-65535 | 65535 | 0 | 65536 |
| Hold-time | 0, 3-65535 | 65535 | 1, 2 | 65536 |
| Prefix length (v4) | 0-32 | 32 | N/A | 33 |
| Prefix length (v6) | 0-128 | 128 | N/A | 129 |

### Functional Tests

| Test | Location | Scenario | Status |
|------|----------|----------|--------|
| `yang-bgp-valid` | `test/data/yang/bgp-valid.conf` | Valid BGP config | |
| `yang-bgp-leafref` | `test/data/yang/bgp-leafref.conf` | Valid leafref references | |
| `yang-bgp-invalid-asn` | `test/data/yang/bgp-invalid-asn.conf` | Invalid ASN rejected | |
| `yang-bgp-missing-leafref` | `test/data/yang/bgp-missing-leafref.conf` | Missing leafref rejected | |

## Files to Create

- `yang/ze-types.yang` - Common types
- `yang/ze-bgp.yang` - BGP configuration
- `yang/ze-plugin.yang` - Plugin configuration
- `internal/yang/modules_test.go` - Module tests
- `test/data/yang/*.conf` - Test configs

## Files to Modify

- `cmd/ze-subsystem/main.go` - Embed and declare YANG modules
- `docs/architecture/config/syntax.md` - Add YANG mapping table

## Implementation Steps

**Self-Critical Review:** After each step, review for issues and fix before proceeding.

1. **Create ze-types.yang** - Common type definitions
2. **Write type tests** - Test IP, ASN, port validation
3. **Run tests** - Verify with goyang (paste output)
4. **Create ze-bgp.yang** - Full BGP schema
5. **Write BGP tests** - Test mandatory, leafref
6. **Run tests** - Verify PASS
7. **Create ze-plugin.yang** - Plugin configuration
8. **Map existing syntax** - Document mapping table
9. **Embed in subsystem** - Add to ze-subsystem declarations
10. **Functional tests** - Create and run
11. **Verify all** - `make lint && make test && make functional` (paste output)

## Open Questions

| # | Question | Options |
|---|----------|---------|
| 1 | AFI/SAFI configuration | Per-peer vs global families |
| 2 | Policy language | Simple (current) vs full YANG |
| 3 | OpenConfig alignment | Custom (simple) vs OpenConfig-like |

## Checklist

### 🧪 TDD
- [ ] Tests written
- [ ] Tests FAIL (output below)
- [ ] Implementation complete
- [ ] Tests PASS (output below)
- [ ] Boundary tests cover all numeric inputs

### Verification
- [ ] `make lint` passes
- [ ] `make test` passes
- [ ] `make functional` passes

### Documentation
- [ ] Required docs read
- [ ] Config syntax mapping documented
- [ ] YANG modules documented

### Completion
- [ ] Spec updated with Implementation Summary
- [ ] Spec moved to `docs/plan/done/NNN-hub-yang-modules.md`
