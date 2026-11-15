# VyOS Configuration System Architecture Research

Comprehensive analysis of how XML is used to define configuration schema, how validation works, and how configuration transforms into tool-specific configurations.

Source: the `vyos-1x` repository at commit `db29bdc899e4` (2026-10-17). Every
path on this page is relative to that repository. The code blocks are
shortened from the source to show its shape; open the named file for the exact
text.

---

## 1. XML-Based Configuration Schema Definition

### Overall Structure

VyOS uses XML schema files to define the entire configuration tree hierarchy. These are stored in:
- Primary schema: `/interface-definitions/` - Contains 130 `.xml.in` files (template preprocessed XML)
- Include files: `/interface-definitions/include/` - Reusable schema snippets organized by subsystem (BGP, firewall, interfaces, etc.)
- Grammar of the definitions: `/schema/` - Contains the RelaxNG (RNG) grammar that the build checks the definition files against. It never sees a value an operator types

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

          <!-- A node with numeric leaves, each with a range -->
          <node name="ring-buffer">
            <properties>
              <help>Shared buffer between the device driver and NIC</help>
            </properties>
            <children>
              <leafNode name="rx">
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
              <!-- leafNode "tx" has the same shape -->
            </children>
          </node>
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

## 2. Validation System

VyOS checks three different things at three different times. Only the last two
judge what an operator types.

| When | What is checked | By what |
|------|-----------------|---------|
| Package build | The definition XML files themselves | The RelaxNG grammar (`scripts/build-command-templates`) |
| `set`, as the operator types | One value, against its node's constraints | The `syntax:expression` of the compiled node template, which runs `validate-value` with the node's regexes and validators |
| `commit` | The configuration as a whole | The `verify()` function of each owner script |

### Build time: the grammar of the definitions

**When:** At package build, never on the router
**What validates:** The definition XML against the RelaxNG grammar. A value an
operator types is never checked against this grammar

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

The build turns each definition into a node template. A `<constraint>` becomes
that template's `syntax:expression`, which is what judges a value at `set`.

### On the router: values at `set`, semantics at `commit`

#### At `set` - Inline Validators (simple, non-interactive)

**When:** As the operator types a value
**What validates:** One value against the regexes and validators of its node

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

A definition names the validator, and `validate-value` runs it when a value is
set:

```xml
<constraint>
  <validator name="numeric" argument="--range 80-16384"/>
</constraint>
```

#### At `commit` - Python Verification Functions (complex, context-aware)

**When:** At commit, before anything is applied
**What validates:** Semantic correctness and hardware constraints
**Location:** The `verify()` function of each script in `/src/conf_mode/`

Shared checks are in `/python/vyos/configverify.py` - Called from Python config handlers

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
│ 2. VALUE VALIDATION AT SET (compiled node templates)             │
│                                                                  │
│  - The path must be a node the definitions declare              │
│  - Run the node's validators (numeric range, regex, etc.)       │
│  - A refused value never enters the session config              │
└──────────────────────────────────────────────────────────────────┘
                                │
                                ▼
┌──────────────────────────────────────────────────────────────────┐
│ 3. CONFIG TREE (libvyosconfig, in OCaml, loaded from Python)     │
│                                                                  │
│  - Holds the session (proposed) config as a ConfigTree          │
│  - Holds the running (effective) config as a second tree        │
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
│ 5. PYTHON CONFIG SCRIPT EXECUTION (4 phases)                     │
│                                                                  │
│  Phase 1: get_config()                                          │
│    - Load config from session into Python dict                  │
│    - Merge defaults, when the script asks for them              │
│    - Apply key name mangling (hyphens → underscores)            │
│                                                                  │
│  Phase 2: verify(config)                                        │
│    - Semantic validation (hardware checks, dependencies)        │
│    - Check for conflicts with other interfaces                  │
│    - Raise ConfigError if invalid                               │
│                                                                  │
│  Phase 3: generate(config)                                      │
│    - Render the config files of the tools the script drives     │
│                                                                  │
│  Phase 4: apply(config)                                         │
│    - Actually apply configuration to system                     │
│    - Reload or restart the tools whose files were rendered      │
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

Every value in that dict is a string, `mtu` included. A script converts a
number by hand, as `int(ethernet['mtu'])` below.

**Step 3: Value Validation (done at each `set`, before the commit)**

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
├── firewall/              # Firewall templates (nftables)
├── dhcp-server/           # DHCP server templates
└── ...
```

### Concrete Example: BGP Configuration Generation

**Input: Configuration Dict (from get_frrender_dict())**

Values are strings. The template receives the `bgp` sub-dict, so its keys are
the template's top-level names.

```python
{
    'bgp': {
        'system_as': '65001',
        'neighbor': {
            '10.0.0.1': {
                'remote_as': '65002',
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

**Template File:** `/data/templates/frr/bgpd.frr.j2` (Jinja2), reduced here to
the lines this input reaches

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

router bgp {{ system_as }}
{%   if neighbor is vyos_defined %}
{%     for peer, peer_config in neighbor.items() %}
 {{ bgp_neighbor(peer, peer_config) }}
{%     endfor %}
{%   endif %}
{%   if address_family is vyos_defined %}
{%     if address_family.ipv4_unicast is vyos_defined %}
 address-family ipv4 unicast
{%       if address_family.ipv4_unicast.network is vyos_defined %}
{%         for network in address_family.ipv4_unicast.network %}
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
    cached_config_dict = {}

    def generate(self, config_dict) -> bool:
        """Generate FRR configuration file; False when nothing changed"""

        # The whole dict is compared with the one rendered last time.
        # (The source also compares the DHCP gateways the file embeds.)
        if self.cached_config_dict == config_dict:
            return False
        self.cached_config_dict = config_dict

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
        return True

    def apply(self, count_max=5):
        """Test the rendered file, then reload it, retrying the reload"""
        rc, emsg = rc_cmd(f'/usr/lib/frr/frr-reload.py --test {self._frr_conf}')
        if rc != 0:
            raise ConfigError(ERROR_RELOAD_TEST)

        count = 0
        while count < count_max:
            count += 1
            rc, emsg = rc_cmd(f'/usr/lib/frr/frr-reload.py --reload {self._frr_conf}')
            if rc != 0:
                sleep(2)
                continue
            break
```

The rendering process:
1. Collect config via `get_frrender_dict()` - gathers all FRR-relevant config
2. Render templates using Jinja2 - produces FRR config syntax. Skipped when the
   collected dict equals the one rendered last time
3. Validate syntax - `frr-reload.py --test <file>`; a failure aborts the commit
4. Apply - `frr-reload.py --reload <file>`, tried up to five times

The config daemon (section 7) holds one long-lived `FRRender` and calls it once
per commit, on the last script of the commit, not once per script.

---

## 5. Key Architectural Insights

### Which Config a Script Reads, and Where Defaults Come From

There is no resolution order across trees. A read uses exactly one tree, and
the two are never merged:

| `effective` | Tree read |
|-------------|-----------|
| `False` (the default) | The session config: what the commit proposes |
| `True` | The running config: what was last committed |

Defaults are not part of either tree. A script gets them only when it asks:

| Flag | What is merged under the explicit values |
|------|------------------------------------------|
| none | Nothing. A leaf the operator did not set is absent from the dict |
| `with_defaults` | The defaults of the leaves directly under paths present in the config |
| `with_recursive_defaults` | Also the defaults under nodes that are absent from the config |

A default is never produced for a tag node, in either mode: with no instance
name there is nothing to attach it to.

```python
# From config.py
def get_config_dict(self, path=[], effective=False, key_mangling=None,
                    get_first_key=False, no_multi_convert=False,
                    no_tag_node_value_mangle=False,
                    with_defaults=False,
                    with_recursive_defaults=False,
                    with_pki=False):
    # One tree, chosen by `effective`
    root_dict = self.get_cached_root_dict(effective)
    conf_dict = get_sub_dict(root_dict, path, get_first_key=get_first_key)

    if with_defaults or with_recursive_defaults:
        defaults = self.get_config_defaults(...,
                                            recursive=with_recursive_defaults)
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
| Definition grammar (build time, not operator input) | RelaxNG grammar | Structure of the definition files | Node types, allowed children |
| Inline validators (at `set`) | `/src/validators/` | Syntax | IP addresses, numeric ranges, regex |
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
[Value Validation at set]
    ├─ The path is a declared node
    └─ Inline validators
    ↓
[Python Handler Invoked at commit]
    ├─ get_config() → Parse to dict
    ├─ verify() → Semantic validation
    ├─ generate() → Render the tool's config files
    └─ apply() → Actual system changes
    ↓
[Template Rendering for Delegation, once per commit]
    ├─ get_frrender_dict() → Collect routing protocol config
    ├─ Render Jinja2 templates → FRR syntax
    └─ Test, then apply, via frr-reload.py
    ↓
[Running Configuration Updated]
    ├─ System state changed (ifconfig, ethtool, etc.)
    └─ Session config → Running config
```

This architecture provides:
- **Single source of truth:** XML schema defines all valid configuration
- **Checked input, untyped data:** Constraints refuse an invalid value at `set`, but a script receives every value as a string and converts it by hand
- **Modularity:** Each subsystem has its own handler script
- **Delegation:** Complex tools (FRR) get intermediate config via templates
- **Testability:** Each layer validates independently

---

## 7. What a Script Author Also Relies On

The sections above describe the schema and the four functions. An author who
wraps a daemon also depends on the mechanisms below.

### Registration is build-time

A subtree joins the system when its package is built and installed: the
definition XML is compiled into node templates and into a Python reference
cache. An add-on package ships a cache of its own, which
`python/vyos/xml_ref/update_cache.py` merges at install. Nothing registers at
runtime, and no protocol lets a separate process declare a subtree.

### The config daemon

Each owner script was first a new Python interpreter for each commit.
`src/services/vyos-configd` removes that cost: it loads every script named in
`configd-include.json` once, as a module (`python/vyos/configmanager.py`,
`load_as_module`), and calls the four functions of the script that owns each
changed node. A script therefore lives across commits, and what it keeps in a
module global survives from one commit to the next.

### Knowing what changed

| Call (`python/vyos/configdict.py`) | Answer |
|-------------------------------------|--------|
| `is_node_changed(conf, path)` | Whether anything under the path differs between the session and running config |
| `leaf_node_changed(conf, path)` | The previous values of a leaf that changed |
| `node_changed(conf, path)` | The child keys added or deleted under a path |

A script chooses how hard to act from these. `service_ntp.py` sets a
`restart_required` key when the VRF changed, and its `apply` then restarts the
daemon; otherwise it runs `systemctl reload-or-restart`.

There is no single way to learn that the subtree was deleted. A script tests
`conf.exists(base)` and then signals it to its own later phases by returning
`None` (`service_ntp.py`), by returning `{}` (`service_lldp.py`), or by setting
a `deleted` key in the dict (40 of the 130 scripts).

### Ordering and effects across subtrees

- Each owner declares a `<priority>`, and a commit runs the changed owners in
  priority order (`python/vyos/configdiff.py`).
- A script whose change must re-run another script calls `set_dependents` in
  `get_config` and `call_dependents` in `apply` (`python/vyos/configdep.py`).
  The pairs are listed by hand in `data/config-mode-dependencies/*.json`.
- A script reads outside its own subtree freely: its `Config` object is the
  whole tree.

### Failure

A `ConfigError` raised in `get_config` or `verify` refuses the commit before
anything is written. Raised in `generate` or `apply`, it is reported as an
apply failure, and what the script already wrote stays: `render()` in
`python/vyos/template.py` renders fully, then overwrites the destination in
place, and nothing restores the previous file.

A few scripts ask the tool itself to judge the rendered file: `nft --check`
for the firewall and NAT scripts, `frr-reload.py --test` for FRR, and
`sshd -t` for SSH. The SSH test runs in `apply`, after the file is already
replaced.

### Operational commands

A script under `src/op_mode/` that follows the `python/vyos/opmode.py`
convention exposes functions named after a verb (`show_`, `restart_`,
`reset_`, `clear_` and others). `opmode.run` turns the typed arguments of each
function into command-line flags. A `show` function takes a `raw` argument:
with it the function returns a dict or a list, which is printed as JSON, and
without it the function returns formatted text. Failures are typed
(`UnconfiguredSubsystem`, `DataUnavailable`, `PermissionDenied` and others).
The CLI tree for these commands is still separate hand-written XML under
`op-mode-definitions/`.

---

## 8. Relevance to Ze

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
