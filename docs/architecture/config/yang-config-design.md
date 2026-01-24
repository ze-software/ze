# YANG-Based Configuration System Design

A design for adopting VyOS's configuration architecture using YANG instead of custom XML.

> **Note:** The project is named "ze" (formerly "ze").

> **See also:** [Hub Architecture](../hub-architecture.md) - The evolved design where Config Reader is a separate process and plugins register their own YANG schemas. This document describes the YANG validation layer; the Hub Architecture document describes how it integrates with the multi-process architecture.

---

## 1. Architecture Overview

### VyOS Pattern (XML)

```
XML Schema → RelaxNG Validation → Python Handler → System
     │              │                   │
     │              │                   ├─ verify()
     │              │                   ├─ generate()
     │              │                   └─ apply()
     │              │
     │              └─ Inline validators (shell scripts)
     │
     └─ <completionHelp><script>
```

### Proposed Pattern (YANG)

```
YANG Schema → libyang Validation → Go Handler → System
     │              │                   │
     │              │                   ├─ Validate()
     │              │                   ├─ Generate()
     │              │                   └─ Apply()
     │              │
     │              └─ zx:validator extension (programs)
     │
     └─ zx:completion extension (programs)
```

### Key Principle

**YANG defines format. Extensions declare behavior. Implementation executes behavior.**

Other YANG tools see valid schemas. ZeBGP additionally executes the extensions.

---

## 2. YANG Extensions Module

Define extensions that mirror VyOS capabilities:

```yang
module ze-extensions {
  namespace "urn:ze:yang:extensions";
  prefix zx;

  // Handler binding (like VyOS "owner" attribute)
  extension handler {
    argument "path";
    description
      "Path to handler that manages this subtree.
       Handler must implement Validate/Generate/Apply interface.";
  }

  // Priority for commit ordering (like VyOS "priority")
  extension priority {
    argument "value";
    description
      "Numeric priority for commit ordering. Lower = earlier.
       Ensures interfaces configured before routing protocols.";
  }

  // External validator program
  extension validator {
    argument "command";
    description
      "External program to validate value.
       Receives value as $1, exits 0 for valid, 1 for invalid.
       Stderr becomes error message.";
  }

  // Validator with arguments
  extension validator-arg {
    argument "args";
    description
      "Arguments to pass to validator before the value.
       Example: zx:validator-arg '--range 1-100';";
  }

  // Completion provider
  extension completion {
    argument "command";
    description
      "External program that outputs valid values, one per line.
       Used for CLI autocompletion.";
  }

  // Completion from static list (common case)
  extension completion-list {
    argument "values";
    description
      "Space-separated list of valid completions.
       Example: zx:completion-list 'auto half full';";
  }

  // Error message for constraint failures
  extension constraint-error {
    argument "message";
    description
      "Custom error message when validation fails.";
  }
}
```

---

## 3. Schema Example: Ethernet Interface

### VyOS XML (Current)

```xml
<tagNode name="ethernet" owner="${vyos_conf_scripts_dir}/interfaces_ethernet.py">
  <properties>
    <help>Ethernet Interface</help>
    <priority>318</priority>
    <constraint>
      <regex>((eth|lan)[0-9]+|(eno|ens|enp|enx).+)</regex>
    </constraint>
  </properties>
  <children>
    <leafNode name="address">
      <properties>
        <help>IP address</help>
        <completionHelp>
          <list>dhcp dhcpv6</list>
        </completionHelp>
        <constraint>
          <validator name="interface-address"/>
          <regex>(dhcp|dhcpv6)</regex>
        </constraint>
        <multi/>
      </properties>
    </leafNode>
    <leafNode name="duplex">
      <properties>
        <help>Duplex mode</help>
        <completionHelp>
          <list>auto half full</list>
        </completionHelp>
        <constraint>
          <regex>(auto|half|full)</regex>
        </constraint>
        <constraintErrorMessage>duplex must be auto, half or full</constraintErrorMessage>
      </properties>
      <defaultValue>auto</defaultValue>
    </leafNode>
    <leafNode name="mtu">
      <properties>
        <help>MTU</help>
        <constraint>
          <validator name="numeric" argument="--range 68-16000"/>
        </constraint>
      </properties>
    </leafNode>
  </children>
</tagNode>
```

### YANG Equivalent

```yang
module ze-interfaces-ethernet {
  namespace "urn:ze:interfaces:ethernet";
  prefix eth;

  import ietf-inet-types { prefix inet; }
  import ze-extensions { prefix zx; }

  // Handler for this subtree
  zx:handler "/usr/lib/ze/handlers/interfaces-ethernet";
  zx:priority "318";

  list ethernet {
    key "name";
    description "Ethernet Interface";

    leaf name {
      type string {
        pattern "((eth|lan)[0-9]+|(eno|ens|enp|enx).+)";
      }
      description "Interface name";
    }

    leaf-list address {
      type union {
        type inet:ipv4-prefix;
        type inet:ipv6-prefix;
        type enumeration {
          enum dhcp;
          enum dhcpv6;
        }
      }
      description "IP address";

      // Dynamic validation: is this address available?
      zx:validator "/usr/lib/ze/validators/interface-address";

      // Static completion + dynamic
      zx:completion-list "dhcp dhcpv6";
    }

    leaf duplex {
      type enumeration {
        enum auto { description "Auto negotiation"; }
        enum half { description "Half duplex"; }
        enum full { description "Full duplex"; }
      }
      default "auto";
      description "Duplex mode";
      zx:constraint-error "duplex must be auto, half or full";
    }

    leaf speed {
      type enumeration {
        enum auto;
        enum 10;
        enum 100;
        enum 1000;
        enum 2500;
        enum 10000;
      }
      default "auto";
      description "Link speed";

      // Validate against hardware capability
      zx:validator "/usr/lib/ze/validators/ethtool-speed";
    }

    leaf mtu {
      type uint16 {
        range "68..16000";
      }
      description "Maximum transmission unit";

      // Validate against hardware limits
      zx:validator "/usr/lib/ze/validators/mtu-check";
    }

    // Cross-field validation via must
    must "speed = 'auto' or duplex != 'auto'" {
      error-message "Manual speed requires manual duplex";
    }

    must "duplex = 'auto' or speed != 'auto'" {
      error-message "Manual duplex requires manual speed";
    }
  }
}
```

---

## 4. Schema Example: BGP

```yang
module ze-bgp {
  namespace "urn:ze:bgp";
  prefix bgp;

  import ietf-inet-types { prefix inet; }
  import ze-extensions { prefix zx; }

  zx:handler "/usr/lib/ze/handlers/bgp";
  zx:priority "820";  // After interfaces

  container bgp {
    description "BGP configuration";

    leaf local-as {
      type inet:as-number;
      mandatory true;
      description "Local autonomous system number";
    }

    leaf router-id {
      type inet:ipv4-address;
      description "BGP router identifier";

      // If not set, derive from interfaces
      zx:completion "/usr/lib/ze/completers/router-ids";
    }

    list neighbor {
      key "address";
      description "BGP neighbor";

      leaf address {
        type inet:ip-address;
        description "Neighbor IP address";

        // Complete from discovered peers or config
        zx:completion "/usr/lib/ze/completers/bgp-neighbors";
      }

      leaf remote-as {
        type inet:as-number;
        mandatory true;
        description "Neighbor autonomous system";
      }

      leaf description {
        type string {
          length "1..80";
        }
      }

      leaf update-source {
        type leafref {
          path "/interfaces/interface/name";
        }
        description "Source interface for BGP session";

        // Also allow IP address
        zx:validator "/usr/lib/ze/validators/update-source";
      }

      container timers {
        leaf hold-time {
          type uint16 {
            range "0 | 3..65535";
          }
          default "180";
          description "Hold time in seconds";
        }

        leaf keepalive {
          type uint16 {
            range "1..65535";
          }
          default "60";
        }

        // Cross-validation
        must "../keepalive < ../hold-time or ../hold-time = 0" {
          error-message "Keepalive must be less than hold-time";
        }
      }

      container address-family {
        container ipv4-unicast {
          presence "Enable IPv4 unicast";

          leaf-list network {
            type inet:ipv4-prefix;
            description "Networks to announce";
          }

          leaf-list redistribute {
            type enumeration {
              enum connected;
              enum static;
              enum ospf;
            }
          }
        }

        container ipv6-unicast {
          presence "Enable IPv6 unicast";

          leaf-list network {
            type inet:ipv6-prefix;
          }
        }
      }
    }

    // Peer groups
    list peer-group {
      key "name";

      leaf name {
        type string {
          length "1..63";
          pattern "[a-zA-Z][a-zA-Z0-9_-]*";
        }
      }

      // Same structure as neighbor, uses grouping in real impl
      leaf remote-as {
        type inet:as-number;
      }
    }
  }

  // Operational state (read-only, populated by daemon)
  container bgp-state {
    config false;

    list neighbor {
      key "address";

      leaf address {
        type inet:ip-address;
      }

      leaf session-state {
        type enumeration {
          enum idle;
          enum connect;
          enum active;
          enum opensent;
          enum openconfirm;
          enum established;
        }
      }

      leaf uptime {
        type uint32;
        units "seconds";
      }

      leaf prefixes-received {
        type uint32;
      }

      leaf prefixes-sent {
        type uint32;
      }
    }
  }
}
```

---

## 5. Validation Layers

### Layer 1: YANG Type Validation (libyang)

Handled automatically by YANG parser:

| Constraint | YANG Syntax | Validated By |
|------------|-------------|--------------|
| Enumeration | `type enumeration` | libyang |
| Range | `type uint16 { range "1..100" }` | libyang |
| Pattern | `type string { pattern "..." }` | libyang |
| Length | `type string { length "1..255" }` | libyang |
| Union | `type union { ... }` | libyang |
| Leafref exists | `type leafref { path "..." }` | libyang + state |

### Layer 2: Cross-Field Validation (must/when)

YANG XPath constraints, evaluated by libyang:

```yang
must "../keepalive < ../hold-time" {
  error-message "Keepalive must be less than hold-time";
}

when "../type = 'ethernet'" {
  // This leaf only valid for ethernet
}
```

### Layer 3: External Validators (zx:validator)

Custom programs for validation that can't be expressed in YANG:

```
┌─────────────────────────────────────────────────────────────────────────┐
│ Validator Protocol                                                       │
│                                                                         │
│ Input:                                                                  │
│   $1 = value to validate                                                │
│   $2... = extra args from zx:validator-arg                              │
│   stdin = JSON context (optional, for complex validators)               │
│                                                                         │
│ Output:                                                                 │
│   exit 0 = valid                                                        │
│   exit 1 = invalid, stderr = error message                              │
│   exit 2 = invalid, stdout = JSON with structured error                 │
│                                                                         │
│ Example validators:                                                     │
│   /usr/lib/ze/validators/                                            │
│     ├── interface-exists      # Check interface exists in system        │
│     ├── ip-available          # Check IP not already assigned           │
│     ├── mtu-check             # Check MTU against hardware limits       │
│     ├── ethtool-speed         # Check NIC supports speed                │
│     ├── numeric               # Generic numeric range checker           │
│     └── bgp-neighbor-valid    # Complex BGP neighbor validation         │
└─────────────────────────────────────────────────────────────────────────┘
```

### Layer 4: Semantic Validation (Handler.Validate)

Handler code for complex validation requiring full context:

```go
// Handler interface
type Handler interface {
    // Validate checks semantic correctness
    // Called after YANG + external validators pass
    Validate(ctx context.Context, config ConfigNode) error

    // Generate produces intermediate configs (optional)
    Generate(ctx context.Context, config ConfigNode) error

    // Apply makes changes to the system
    Apply(ctx context.Context, config ConfigNode) error
}

// Example: interfaces-ethernet handler
func (h *EthernetHandler) Validate(ctx context.Context, cfg ConfigNode) error {
    for _, iface := range cfg.List("ethernet") {
        name := iface.Leaf("name")

        // Check interface exists in system
        if !h.system.InterfaceExists(name) {
            return fmt.Errorf("interface %s does not exist", name)
        }

        // Check speed/duplex against hardware
        speed := iface.Leaf("speed")
        duplex := iface.Leaf("duplex")
        if err := h.validateSpeedDuplex(name, speed, duplex); err != nil {
            return err
        }

        // Check MTU against hardware limits
        if mtu := iface.Leaf("mtu"); mtu != "" {
            min, max := h.system.MTULimits(name)
            if mtuVal < min || mtuVal > max {
                return fmt.Errorf("MTU %d outside hardware limits [%d-%d]",
                                  mtuVal, min, max)
            }
        }
    }
    return nil
}
```

---

## 6. Completion System

### Static Completion

From YANG schema directly:

```yang
leaf duplex {
  type enumeration {
    enum auto;
    enum half;
    enum full;
  }
}
// Completion: [auto, half, full] - derived from enum
```

Or from extension:

```yang
leaf address {
  type inet:ip-address;
  zx:completion-list "dhcp dhcpv6";  // Static additions
}
```

### Dynamic Completion

External programs that query system state:

```yang
leaf interface {
  type string;
  zx:completion "/usr/lib/ze/completers/list-interfaces";
}
```

Completer protocol:

```
┌─────────────────────────────────────────────────────────────────────────┐
│ Completer Protocol                                                       │
│                                                                         │
│ Input:                                                                  │
│   $1 = partial value (what user typed so far)                           │
│   stdin = JSON context (current config path, for context-aware)         │
│                                                                         │
│ Output:                                                                 │
│   stdout = one completion per line                                      │
│   Format: "value<TAB>description" (description optional)                │
│                                                                         │
│ Example: list-interfaces                                                │
│   #!/bin/sh                                                             │
│   ls /sys/class/net | while read iface; do                              │
│     state=$(cat /sys/class/net/$iface/operstate 2>/dev/null)            │
│     echo "$iface	$state"                                               │
│   done                                                                  │
│                                                                         │
│ Output:                                                                 │
│   eth0    up                                                            │
│   eth1    down                                                          │
│   lo      unknown                                                       │
└─────────────────────────────────────────────────────────────────────────┘
```

### Completion Sources (Priority Order)

1. YANG enumeration values
2. `zx:completion-list` static values
3. `zx:completion` program output
4. `leafref` target values (from operational state)

---

## 7. Handler System

### Handler Discovery

Handlers declared in YANG via `zx:handler`:

```yang
module ze-interfaces-ethernet {
  zx:handler "/usr/lib/ze/handlers/interfaces-ethernet";

  list ethernet { ... }
}
```

### Handler Interface (Go)

```go
package handler

import "context"

// ConfigNode represents a node in the config tree
type ConfigNode interface {
    Path() string
    Leaf(name string) string
    LeafList(name string) []string
    Container(name string) ConfigNode
    List(name string) []ConfigNode
    Exists() bool
}

// Diff represents changes between configs
type Diff struct {
    Added   []ConfigNode
    Removed []ConfigNode
    Changed []struct {
        Old ConfigNode
        New ConfigNode
    }
}

// Handler processes configuration for a subtree
type Handler interface {
    // Priority returns commit ordering priority
    Priority() int

    // Validate checks semantic correctness
    Validate(ctx context.Context, proposed ConfigNode) error

    // Generate creates intermediate configs if needed
    Generate(ctx context.Context, proposed ConfigNode) error

    // Apply makes system changes
    // Receives diff for incremental application
    Apply(ctx context.Context, diff Diff) error

    // Rollback reverts failed changes
    Rollback(ctx context.Context, diff Diff) error
}
```

### Handler Execution Flow

```
User: commit
         │
         ▼
┌─────────────────────────────────────────────────────────────────────────┐
│ 1. YANG Validation (libyang)                                            │
│    - Type constraints                                                   │
│    - Pattern/range                                                      │
│    - must/when expressions                                              │
│    - leafref (against current state)                                    │
└─────────────────────────────────────────────────────────────────────────┘
         │ pass
         ▼
┌─────────────────────────────────────────────────────────────────────────┐
│ 2. External Validators (zx:validator)                                   │
│    - Run for each leaf with validator                                   │
│    - Parallel execution where possible                                  │
│    - Collect all errors before failing                                  │
└─────────────────────────────────────────────────────────────────────────┘
         │ pass
         ▼
┌─────────────────────────────────────────────────────────────────────────┐
│ 3. Handler Validation (sorted by priority)                              │
│    for handler in sorted(handlers, key=priority):                       │
│        if err := handler.Validate(ctx, config); err != nil:             │
│            return err  // Stop on first failure                         │
└─────────────────────────────────────────────────────────────────────────┘
         │ pass
         ▼
┌─────────────────────────────────────────────────────────────────────────┐
│ 4. Generate Phase (sorted by priority)                                  │
│    for handler in sorted(handlers, key=priority):                       │
│        handler.Generate(ctx, config)                                    │
│    // Produces intermediate configs (FRR, etc.)                         │
└─────────────────────────────────────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────────────────────────────────────┐
│ 5. Apply Phase (sorted by priority)                                     │
│    applied := []Handler{}                                               │
│    for handler in sorted(handlers, key=priority):                       │
│        if err := handler.Apply(ctx, diff); err != nil:                  │
│            // Rollback in reverse order                                 │
│            for h in reverse(applied):                                   │
│                h.Rollback(ctx, diff)                                    │
│            return err                                                   │
│        applied = append(applied, handler)                               │
└─────────────────────────────────────────────────────────────────────────┘
         │ success
         ▼
┌─────────────────────────────────────────────────────────────────────────┐
│ 6. Update Running Config                                                │
│    - Proposed config becomes running config                             │
│    - Update operational state                                           │
└─────────────────────────────────────────────────────────────────────────┘
```

---

## 8. CLI Generation from YANG

### Mapping YANG to CLI Commands

| YANG Construct | CLI Syntax |
|----------------|------------|
| `container foo` | `foo` (enters context) |
| `list foo { key "name" }` | `foo <name>` |
| `leaf bar` | `bar <value>` |
| `leaf-list baz` | `baz <value>` (repeatable) |
| `presence container` | `foo` (no value, enables) |
| `leaf { type empty }` | `foo` (flag, no value) |

### Example CLI Session

```
# Enter configuration mode
ze# configure

# Set ethernet interface (list with key)
ze(config)# interfaces ethernet eth0

# Set leaf values
ze(config-ethernet-eth0)# address 192.168.1.1/24
ze(config-ethernet-eth0)# mtu 9000

# Completion shows valid options
ze(config-ethernet-eth0)# duplex <TAB>
  auto    Auto negotiation
  half    Half duplex
  full    Full duplex

ze(config-ethernet-eth0)# duplex full
ze(config-ethernet-eth0)# exit

# Configure BGP
ze(config)# bgp
ze(config-bgp)# local-as 65001
ze(config-bgp)# neighbor 10.0.0.1
ze(config-bgp-neighbor-10.0.0.1)# remote-as 65002
ze(config-bgp-neighbor-10.0.0.1)# address-family ipv4-unicast
ze(config-...-ipv4-unicast)# network 10.0.0.0/24
ze(config-...-ipv4-unicast)# exit
ze(config-bgp-neighbor-10.0.0.1)# exit
ze(config-bgp)# exit

# Commit changes
ze(config)# commit
Validating...
  [YANG] Structure valid
  [YANG] Constraints valid
  [validator] interface-exists eth0: ok
  [validator] mtu-check eth0 9000: ok
  [handler] interfaces-ethernet: valid
  [handler] bgp: valid
Applying...
  [interfaces-ethernet] Configuring eth0
  [bgp] Generating FRR config
  [bgp] Reloading FRR
Commit complete.
```

### CLI Help from YANG

```yang
leaf mtu {
  type uint16 {
    range "68..16000";
  }
  description "Maximum transmission unit";
  zx:constraint-error "MTU must be between 68 and 16000";
}
```

Generates:

```
ze(config-ethernet-eth0)# mtu ?
  <68-16000>    Maximum transmission unit

ze(config-ethernet-eth0)# mtu 50
Error: MTU must be between 68 and 16000
```

---

## 9. State Population

### Operational State in YANG

```yang
container interfaces-state {
  config false;  // Read-only

  list interface {
    key "name";

    leaf name { type string; }
    leaf oper-status {
      type enumeration {
        enum up;
        enum down;
        enum testing;
      }
    }
    leaf speed { type uint32; units "Mbps"; }
    leaf mtu { type uint16; }

    container statistics {
      leaf in-octets { type uint64; }
      leaf out-octets { type uint64; }
      leaf in-errors { type uint32; }
      leaf out-errors { type uint32; }
    }
  }
}
```

### State Population Daemon

```go
// Runs periodically or on events
func (d *StateDaemon) PopulateInterfaceState(ctx context.Context) error {
    interfaces, err := net.Interfaces()
    if err != nil {
        return err
    }

    for _, iface := range interfaces {
        path := fmt.Sprintf("/interfaces-state/interface[name='%s']", iface.Name)

        d.state.Set(path+"/name", iface.Name)
        d.state.Set(path+"/oper-status", operStatus(iface))
        d.state.Set(path+"/mtu", iface.MTU)

        // Statistics from /sys/class/net/*/statistics/
        stats := readStats(iface.Name)
        d.state.Set(path+"/statistics/in-octets", stats.RxBytes)
        d.state.Set(path+"/statistics/out-octets", stats.TxBytes)
    }

    return nil
}
```

### State Used for leafref Validation

```yang
leaf update-source {
  type leafref {
    path "/interfaces-state/interface/name";
  }
}
```

When validating, libyang checks if value exists in populated state.

---

## 10. File Organization

```
/usr/lib/ze/
├── yang/                              # YANG modules
│   ├── ze-extensions.yang          # Extension definitions
│   ├── ze-interfaces.yang          # Interface container
│   ├── ze-interfaces-ethernet.yang # Ethernet specifics
│   ├── ze-interfaces-loopback.yang
│   ├── ze-bgp.yang                 # BGP configuration
│   ├── ze-policy.yang              # Routing policy
│   └── ietf/                          # Standard IETF modules
│       ├── ietf-inet-types.yang
│       └── ietf-yang-types.yang
│
├── handlers/                          # Configuration handlers
│   ├── interfaces-ethernet            # Ethernet handler binary
│   ├── interfaces-loopback
│   ├── bgp
│   └── policy
│
├── validators/                        # Validation programs
│   ├── interface-exists
│   ├── ip-available
│   ├── mtu-check
│   ├── ethtool-speed
│   ├── numeric
│   └── bgp-neighbor
│
├── completers/                        # Completion programs
│   ├── list-interfaces
│   ├── list-routes
│   ├── bgp-neighbors
│   └── router-ids
│
└── templates/                         # Output templates (if needed)
    └── frr/
        ├── bgpd.conf.tmpl
        └── staticd.conf.tmpl

/var/lib/ze/
├── config/
│   ├── running.json                   # Current running config
│   ├── candidate.json                 # Proposed changes
│   └── rollback/                      # Rollback snapshots
│       ├── 001.json
│       └── 002.json
│
└── state/
    └── operational.json               # Current operational state
```

---

## 11. Comparison Summary

| Aspect | VyOS (XML) | This Design (YANG) |
|--------|-----------|-------------------|
| Schema format | Custom XML | Standard YANG |
| Type validation | RelaxNG + validators | libyang (built-in) |
| Cross-field validation | Python verify() | YANG must/when + handler |
| External validation | `<validator name="..."/>` | `zx:validator` extension |
| Completion | `<completionHelp><script>` | `zx:completion` extension |
| Handler binding | `owner="script.py"` | `zx:handler` extension |
| Priority ordering | `<priority>N</priority>` | `zx:priority` extension |
| Error messages | `<constraintErrorMessage>` | `zx:constraint-error` extension |
| Tooling | VyOS-specific | pyang, libyang, NETCONF/RESTCONF |
| Portability | None | Schema valid for any YANG tool |

---

## 12. Benefits of This Approach

### Standardization

- YANG is RFC 7950, widely supported
- Schema validates with standard tools (pyang)
- Path to NETCONF/RESTCONF/gNMI APIs
- Reuse existing IETF/OpenConfig models

### Extensibility

- Extensions ignored by tools that don't understand them
- Add new validation types without breaking compatibility
- Mix standard and custom features cleanly

### Tooling

- libyang provides fast C validation
- Go bindings available (goyang, ygot)
- Auto-generate Go structs from YANG
- Auto-generate documentation

### Separation of Concerns

- YANG: data model (format, structure, basic constraints)
- Extensions: behavior declaration (what to run)
- Handlers: behavior implementation (how to run)
- State daemon: operational data population

---

## 13. Implementation Path

### Phase 1: Core Infrastructure

1. Define `ze-extensions` YANG module
2. Implement extension parser (read zx:* during schema load)
3. Build handler registration system
4. Build validator/completer execution framework

### Phase 2: Basic Types

1. Port interface configuration
2. Port static routing
3. Implement CLI generator from YANG

### Phase 3: BGP

1. Port BGP configuration schema
2. Implement BGP handler
3. Implement operational state for BGP

### Phase 4: Integration

1. NETCONF server (optional)
2. RESTCONF server (optional)
3. gNMI server (optional)

---

## 14. Open Questions

1. **Handler language**: Go binaries vs scripts vs plugins?
2. **State synchronization**: Push vs pull vs event-driven?
3. **Transaction scope**: Per-handler vs global commit?
4. **Rollback depth**: How many configs to keep?
5. **Startup config**: YANG JSON vs custom format?
