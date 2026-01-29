# VyOS Configuration System Architecture Research

Comprehensive analysis of how XML is used to define configuration schema, how validation works, and how configuration transforms into tool-specific configurations.

---

## 1. XML-Based Configuration Schema Definition

### Overall Structure

VyOS uses XML schema files to define the entire configuration tree hierarchy. These are stored in:
- Primary schema: `/interface-definitions/` - Contains ~124 `.xml.in` files (template preprocessed XML)
- Include files: `/interface-definitions/include/` - Reusable schema snippets organized by subsystem (BGP, firewall, interfaces, etc.)
- Schema validation: `/schema/` - Contains RelaxNG (RNG) grammar files for XML validation

### XML Element Types

The schema defines four node types:

```xml
<!-- Regular node: fixed-name intermediate node, no values, can have children -->
<node name="system">
  <properties>
    <help>System parameters</help>
  </properties>
  <children>
    <!-- child nodes here -->
  </children>
</node>

<!-- Tag node: variable-name container (like interface names eth0, eth1) -->
<tagNode name="ethernet" owner="${vyos_conf_scripts_dir}/interfaces_ethernet.py">
  <properties>
    <help>Ethernet Interface</help>
    <priority>318</priority>
    <valueHelp>
      <format>ethN</format>
      <description>Ethernet interface name</description>
    </valueHelp>
    <constraint>
      <regex>((eth|lan)[0-9]+|(eno|ens|enp|enx).+)</regex>
    </constraint>
  </properties>
  <children>
    <!-- child leaf nodes -->
  </children>
</tagNode>

<!-- Leaf node: terminal node with values, no children -->
<leafNode name="duplex">
  <properties>
    <help>Duplex mode</help>
    <completionHelp>
      <list>auto half full</list>
    </completionHelp>
    <valueHelp>
      <format>auto</format>
      <description>Auto negotiation</description>
    </valueHelp>
    <constraint>
      <regex>(auto|half|full)</regex>
    </constraint>
    <constraintErrorMessage>duplex must be auto, half or full</constraintErrorMessage>
  </properties>
  <defaultValue>auto</defaultValue>
</leafNode>

<!-- Valueless leaf node: Boolean-style config option -->
<leafNode name="disable-flow-control">
  <properties>
    <help>Disable Ethernet flow control</help>
    <valueless/>
  </properties>
</leafNode>
```

### Concrete Example: Ethernet Interface Schema

File: `/interface-definitions/interfaces_ethernet.xml.in`

```xml
<?xml version="1.0"?>
<interfaceDefinition>
  <node name="interfaces">
    <properties>
      <help>Network interfaces</help>
    </properties>
    <children>
      <!-- Key: "owner" attribute points to Python handler -->
      <tagNode name="ethernet" owner="${vyos_conf_scripts_dir}/interfaces_ethernet.py">
        <properties>
          <help>Ethernet Interface</help>
          <priority>318</priority>
          <constraint>
            <regex>((eth|lan)[0-9]+|(eno|ens|enp|enx).+)</regex>
          </constraint>
        </properties>
        <children>
          <!-- Includes reusable schema fragments -->
          #include <include/interface/address-ipv4-ipv6-dhcp.xml.i>
          #include <include/generic-description.xml.i>

          <!-- Direct leaf nodes -->
          <leafNode name="duplex">
            <properties>
              <help>Duplex mode</help>
              <constraint>
                <regex>(auto|half|full)</regex>
              </constraint>
            </properties>
            <defaultValue>auto</defaultValue>
          </leafNode>

          <!-- Numeric with range -->
          <leafNode name="ring-buffer rx">
            <properties>
              <help>RX ring buffer</help>
              <valueHelp>
                <format>u32:80-16384</format>
                <description>ring buffer size</description>
              </valueHelp>
              <constraint>
                <validator name="numeric" argument="--range 80-16384"/>
              </constraint>
            </properties>
          </leafNode>
        </children>
      </tagNode>
    </children>
  </node>
</interfaceDefinition>
```

Include file example: `/interface-definitions/include/interface/address-ipv4-ipv6-dhcp.xml.i`

```xml
<!-- include start from interface/address-ipv4-ipv6-dhcp.xml.i -->
<leafNode name="address">
  <properties>
    <help>IP address</help>
    <completionHelp>
      <list>dhcp dhcpv6</list>
    </completionHelp>
    <valueHelp>
      <format>ipv4net</format>
      <description>IPv4 address and prefix length</description>
    </valueHelp>
    <valueHelp>
      <format>ipv6net</format>
      <description>IPv6 address and prefix length</description>
    </valueHelp>
    <constraint>
      <validator name="interface-address"/>
      <regex>(dhcp|dhcpv6)</regex>
    </constraint>
    <multi/>  <!-- Allows multiple values -->
  </properties>
</leafNode>
<!-- include end -->
```

### Constraint Definition

Constraints are reusable and organized:

File: `/interface-definitions/include/constraint/host-name.xml.i`

```xml
<regex>[A-Za-z0-9][-.A-Za-z0-9]*[A-Za-z0-9]</regex>
```

File: `/interface-definitions/include/bgp/afi-allowas-in.xml.i`

```xml
<node name="allowas-in">
  <properties>
    <help>Accept route containing local-as in as-path</help>
  </properties>
  <children>
    <leafNode name="number">
      <properties>
        <help>Number of occurrences of AS number</help>
        <constraint>
          <validator name="numeric" argument="--range 1-10"/>
        </constraint>
      </properties>
    </leafNode>
  </children>
</node>
```

---

## 2. Validation System (Two-Layer)

VyOS implements validation in two layers:

### Layer 1: Schema Validation (Passive)

**When:** Happens immediately as user types in CLI
**What validates:** Structure against RelaxNG grammar

Schema file: `/schema/interface_definition.rng` (RELAX-NG format)

```xml
<!-- Defines allowed structure -->
<define name="leafNode">
  <element name="leafNode">
    <ref name="nodeNameAttr"/>
    <interleave>
      <ref name="properties"/>
      <zeroOrMore>
        <ref name="constraint"/>
      </zeroOrMore>
    </interleave>
  </element>
</define>
```

The schema defines:
- What node types are allowed (node, tagNode, leafNode)
- What properties each can have (help, priority, owner, defaultValue)
- What constraints are allowed (regex, validator)

### Layer 2: Active Validation (Commit-Time)

**When:** Happens at commit/configuration application time
**What validates:** Semantic correctness and hardware constraints
**Location:** Python scripts in `/src/conf_mode/`

Two validation methods:

#### Method A - Inline Validators (simple, non-interactive)

Located in `/src/validators/` - Shell scripts or binaries

Example: `/src/validators/ip-cidr`

```bash
#!/bin/sh
ipaddrcheck --is-any-cidr "$1"
if [ $? -gt 0 ]; then
    echo "Error: $1 is not a valid IP CIDR"
    exit 1
fi
exit 0
```

These are invoked during XML schema validation:

```xml
<constraint>
  <validator name="numeric" argument="--range 80-16384"/>
</constraint>
```

#### Method B - Python Verification Functions (complex, context-aware)

Located in `/python/vyos/configverify.py` - Called from Python config handlers

Example from `interfaces_ethernet.py`:

```python
from vyos.configverify import verify_mtu
from vyos.configverify import verify_address
from vyos.configverify import verify_dhcpv6

def verify(ethernet):
    """Semantic validation of configuration"""

    # Check MTU against hardware limits
    verify_mtu(ethernet)
    verify_mtu_ipv6(ethernet)

    # Check IPv4/IPv6 address validity
    verify_address(ethernet)

    # Check DHCPv6 specific constraints
    verify_dhcpv6(ethernet)

    # Check if interface is valid hardware
    ethtool = Ethtool(ifname)
    verify_speed_duplex(ethernet, ethtool)
    verify_flow_control(ethernet, ethtool)
    verify_ring_buffer(ethernet, ethtool)
```

Example validation function in `configverify.py`:

```python
def verify_mtu(config):
    """Validate MTU against hardware capabilities"""
    from vyos.ifconfig import Interface
    if 'mtu' in config:
        mtu = int(config['mtu'])
        tmp = Interface(config['ifname'])
        try:
            min_mtu = tmp.get_min_mtu()
            max_mtu = tmp.get_max_mtu()
        except:
            min_mtu = 68
            max_mtu = 9000

        if mtu < min_mtu:
            raise ConfigError(f'MTU too low, minimum {min_mtu}!')
        if mtu > max_mtu:
            raise ConfigError(f'MTU too high, maximum {max_mtu}!')
```

---

## 3. Configuration Processing Flow

### The Complete Pipeline

```
┌──────────────────────────────────────────────────────────────────┐
│ 1. USER CONFIGURATION (CLI or config file)                       │
│                                                                  │
│  set interfaces ethernet eth0 address 192.168.1.1/24             │
│  set interfaces ethernet eth0 duplex full                        │
│  set interfaces ethernet eth0 mtu 9000                           │
└──────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌──────────────────────────────────────────────────────────────────┐
│ 2. CONFIG TREE PARSING (libvyosconfig, in C++)                   │
│                                                                  │
│  - Parse into ConfigTree structure                              │
│  - Maintains session/proposed vs running config                 │
└──────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌──────────────────────────────────────────────────────────────────┐
│ 3. XML SCHEMA VALIDATION                                         │
│                                                                  │
│  - Validate against RelaxNG grammar                             │
│  - Run inline validators (numeric range, regex, etc.)           │
│  - Check node types and structure                               │
└──────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌──────────────────────────────────────────────────────────────────┐
│ 4. PYTHON CONFIG HANDLER LOOKUP                                  │
│                                                                  │
│  XML owner attribute:                                            │
│  <tagNode name="ethernet"                                       │
│          owner="${vyos_conf_scripts_dir}/interfaces_ethernet.py"│
│                                                                  │
│  → Invokes: /src/conf_mode/interfaces_ethernet.py               │
└──────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌──────────────────────────────────────────────────────────────────┐
│ 5. PYTHON CONFIG SCRIPT EXECUTION (3 phases)                     │
│                                                                  │
│  Phase 1: get_config()                                          │
│    - Load config from session into Python dict                  │
│    - Merge defaults                                             │
│    - Apply key name mangling (hyphens → underscores)            │
│                                                                  │
│  Phase 2: verify(config)                                        │
│    - Semantic validation (hardware checks, dependencies)        │
│    - Check for conflicts with other interfaces                  │
│    - Raise ConfigError if invalid                               │
│                                                                  │
│  Phase 3: apply(config)                                         │
│    - Actually apply configuration to system                     │
│    - May generate intermediate configs for other tools          │
└──────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌──────────────────────────────────────────────────────────────────┐
│ 6. CONFIG TO SYSTEM TRANSFORMATION                               │
│                                                                  │
│  EthernetIf(ifname).update(config)                              │
│    ├─ Sets MTU                                                   │
│    ├─ Sets MAC address                                           │
│    ├─ Applies IP addresses                                       │
│    ├─ Enables/disables interface                                 │
│    └─ Sets NIC offload features                                  │
└──────────────────────────────────────────────────────────────────┘
```

### Concrete Example: Configuration Flow for eth0

**Step 1: User input (CLI)**

```
configure
set interfaces ethernet eth0 address 192.168.1.1/24
set interfaces ethernet eth0 duplex full
set interfaces ethernet eth0 speed 1000
set interfaces ethernet eth0 mtu 1500
commit
```

**Step 2: ConfigTree Representation (Python dict via libvyosconfig)**

```json
{
    "interfaces": {
        "ethernet": {
            "eth0": {
                "address": ["192.168.1.1/24"],
                "duplex": "full",
                "speed": "1000",
                "mtu": "1500"
            }
        }
    }
}
```

**Step 3: XML Schema Validation**

- ethernet is allowed under interfaces
- eth0 matches regex pattern `((eth|lan)[0-9]+...)`
- address is valid leafNode under ethernet
- duplex matches constraint regex `(auto|half|full)`
- speed matches allowed values `auto|10|100|1000|...`
- mtu numeric validator passes: 1500 in valid range

**Step 4: Handler Invocation**

```python
# interfaces_ethernet.py is invoked as a script
#!/usr/bin/env python3

def get_config(config=None):
    # Load and return configuration
    conf = Config()
    base = ['interfaces', 'ethernet']
    ifname, ethernet = get_interface_dict(conf, base)

    # Result:
    # ethernet = {
    #     'ifname': 'eth0',
    #     'address': ['192.168.1.1/24'],
    #     'duplex': 'full',
    #     'speed': '1000',
    #     'mtu': '1500'
    # }
    return ethernet

def verify(ethernet):
    # Semantic validation
    ethtool = Ethtool('eth0')

    # Check speed/duplex match (both auto or both manual)
    if (ethernet['speed'] == 'auto' and ethernet['duplex'] != 'auto') or \
       (ethernet['speed'] != 'auto' and ethernet['duplex'] == 'auto'):
        raise ConfigError('Speed/Duplex mismatch')

    # Check if NIC supports this speed/duplex
    if not ethtool.check_speed_duplex(ethernet['speed'], ethernet['duplex']):
        raise ConfigError('NIC does not support this speed/duplex!')

    # Validate MTU against hardware
    if not ethtool.check_mtu_capability(int(ethernet['mtu'])):
        raise ConfigError('MTU not supported!')

def apply(ethernet):
    # Apply to system
    e = EthernetIf('eth0')
    e.update(ethernet)
    # This calls ethtool to set speed, duplex, MTU, etc.
```

**Step 5: System Application**

```bash
ethtool -s eth0 speed 1000 duplex full
ip link set dev eth0 mtu 1500
ip addr add 192.168.1.1/24 dev eth0
ip link set dev eth0 up
```

---

## 4. Configuration Transformation to Tool-Specific Formats

VyOS transforms its configuration into tool-specific configurations through templating:

### Template System Architecture

**Templating Engine:** Jinja2 (Python)

**Template Location:** `/data/templates/` - Contains templates for different subsystems

```
/data/templates/
├── frr/                    # FRR routing daemon templates
│   ├── bgpd.frr.j2        # BGP configuration
│   ├── ospfd.frr.j2       # OSPF configuration
│   ├── isisd.frr.j2       # ISIS configuration
│   └── staticd.frr.j2     # Static routes
├── iptables/              # Firewall templates
├── dhcp/                  # DHCP server templates
└── ...
```

### Concrete Example: BGP Configuration Generation

**Input: Configuration Dict (from get_frrender_dict())**

```python
{
    'bgp': {
        'local_as': 65001,
        'neighbor': {
            '10.0.0.1': {
                'remote_as': 65002,
                'description': 'upstream'
            }
        },
        'address_family': {
            'ipv4_unicast': {
                'network': ['10.0.0.0/24']
            }
        }
    }
}
```

**Template File:** `/data/templates/frr/bgpd.frr.j2` (Jinja2)

```jinja2
{### MACRO for recurring peer pattern ###}
{% macro bgp_neighbor(neighbor, config, peer_group=false) %}
{% if config.remote_as is vyos_defined %}
 neighbor {{ neighbor }} remote-as {{ config.remote_as }}
{%     endif %}
{%     if config.description is vyos_defined %}
 neighbor {{ neighbor }} description {{ config.description }}
{%     endif %}
{% endmacro %}

router bgp {{ bgp.local_as }}
{%   if bgp.neighbor is vyos_defined %}
{%     for neighbor, neighbor_config in bgp.neighbor.items() %}
 {{ bgp_neighbor(neighbor, neighbor_config) }}
{%     endfor %}
{%   endif %}
{%   if bgp.address_family is vyos_defined %}
{%     if bgp.address_family.ipv4_unicast is vyos_defined %}
 address-family ipv4 unicast
{%       if bgp.address_family.ipv4_unicast.network is vyos_defined %}
{%         for network in bgp.address_family.ipv4_unicast.network %}
  network {{ network }}
{%         endfor %}
{%       endif %}
 exit-address-family
{%     endif %}
{%   endif %}
```

**Generated Output (FRR configuration)**

```
router bgp 65001
 neighbor 10.0.0.1 remote-as 65002
 neighbor 10.0.0.1 description upstream
 address-family ipv4 unicast
  network 10.0.0.0/24
 exit-address-family
```

### Template Rendering System (FRRender class)

Located in `/python/vyos/frrender.py`:

```python
class FRRender:
    def generate(self, config_dict) -> None:
        """Generate FRR configuration file from config dict"""

        output = '!\n'

        # Render each protocol's template
        if 'bgp' in config_dict and 'deleted' not in config_dict['bgp']:
            output += render_to_string('frr/bgpd.frr.j2',
                                      config_dict['bgp'])
            output += '\n'

        if 'ospf' in config_dict and 'deleted' not in config_dict['ospf']:
            output += render_to_string('frr/ospfd.frr.j2',
                                      config_dict['ospf'])
            output += '\n'

        # ... more protocols ...

        # Write rendered config
        write_file(self._frr_conf, output)

    def apply(self) -> None:
        """Apply rendered config using frr-reload.py"""
        cmd('frr-reload.py --reload < /run/frr/config/vyos.frr.conf')
```

The rendering process:
1. Collect config via `get_frrender_dict()` - gathers all FRR-relevant config
2. Render templates using Jinja2 - produces FRR config syntax
3. Validate syntax - test with `frr-reload.py --test`
4. Apply - load validated config into FRR daemons

---

## 5. Key Architectural Insights

### Config Resolution Order

1. Session config (proposed changes) takes priority over
2. Running config (current effective config)
3. Defaults from XML schema (only if not explicitly set)

```python
# From config.py
def get_config_dict(self, path=[], effective=False,
                   with_defaults=False, ...):
    """
    Session config > explicit values > defaults
    """
    conf_dict = get_sub_dict(root_dict, path)

    if with_defaults:
        defaults = get_config_defaults(path)
        # Merge defaults under explicit values
        conf_dict = config_dict_merge(defaults, conf_dict)
```

### Owner Binding

Each subtree in XML can have an owner - a Python script responsible for that configuration:

```xml
<tagNode name="ethernet"
         owner="${vyos_conf_scripts_dir}/interfaces_ethernet.py">
```

When that config changes, the corresponding handler script is automatically invoked:
- `get_config()` - Load config
- `verify()` - Validate semantics
- `generate()` - Generate intermediate configs if needed
- `apply()` - Apply to system

### Multi-Layered Validation

| Layer | Location | Type | Examples |
|-------|----------|------|----------|
| Schema | RelaxNG grammar | Structure | Node types, allowed children |
| Inline validators | `/src/validators/` | Syntax | IP addresses, numeric ranges, regex |
| Python verify() | `/src/conf_mode/*.py` | Semantic | Hardware capabilities, dependencies, conflicts |
| External tools | System commands | Runtime | ethtool, ip, FRR validation |

### Key Files to Modify for Changes

| Area | File Pattern | Purpose |
|------|--------------|---------|
| Schema | `interface-definitions/*.xml.in` | Define config structure |
| Includes | `interface-definitions/include/` | Reusable schema fragments |
| Handlers | `src/conf_mode/*.py` | Semantic validation + system application |
| Templates | `data/templates/frr/*.j2` | Generate tool-specific configs |
| Validators | `src/validators/*` | Custom input validation |
| Common verify | `python/vyos/configverify.py` | Shared validation functions |

---

## 6. Summary: The Complete Data Flow

```
User Config
    ↓
[XML Schema Validation]
    ├─ Structure checks
    └─ Inline validators
    ↓
[Python Handler Invoked]
    ├─ get_config() → Parse to dict
    ├─ verify() → Semantic validation
    ├─ generate() → Create intermediate configs (if FRR)
    └─ apply() → Actual system changes
    ↓
[Template Rendering for Delegation]
    ├─ get_frrender_dict() → Collect routing protocol config
    ├─ Render Jinja2 templates → FRR syntax
    └─ Apply via frr-reload.py
    ↓
[Running Configuration Updated]
    ├─ System state changed (ifconfig, ethtool, etc.)
    └─ Session config → Running config
```

This architecture provides:
- **Single source of truth:** XML schema defines all valid configuration
- **Type safety:** Constraints enforce valid input before semantic validation
- **Modularity:** Each subsystem has its own handler script
- **Delegation:** Complex tools (FRR) get intermediate config via templates
- **Testability:** Each layer validates independently

---

## 7. Relevance to Ze

### Core Pattern: XML → Validation → Template

```
┌─────────────────┐     ┌──────────────────┐     ┌─────────────────┐
│  XML Schema     │ ──▶ │  Python Handler  │ ──▶ │ Jinja2 Template │
│ (structure def) │     │ (semantic verify)│     │ (tool config)   │
└─────────────────┘     └──────────────────┘     └─────────────────┘
```

### Key Design Decisions

| Decision | Rationale |
|----------|-----------|
| XML for schema | Machine-readable, tooling support, CLI auto-generation |
| Python handlers | Complex logic, external tool interaction |
| Jinja2 templates | Readable config generation, separation of concerns |
| owner binding | Each subtree has single responsible handler |
| Two-layer validation | Fast syntax check + deep semantic check |

### Interesting Patterns for Ze

- **Schema-driven CLI** - XML defines valid paths, CLI auto-generated
- **Owner pattern** - subtree → handler binding (similar to plugin model?)
- **Jinja2 for tool configs** - clean separation
- **Validator executables** - small programs for complex validation
