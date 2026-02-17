# Ze — A Modern BGP Implementation

> **Status: Early Development** — Ze is under heavy active development and is not yet ready for production use. APIs, configuration syntax, and plugin interfaces may change without notice. Feedback and contributions are welcome.

Ze is a BGP daemon written in Go, built by the creator of [ExaBGP](https://github.com/Exa-Networks/exabgp). It uses a **plugin-based architecture** where the engine handles the protocol and plugins implement everything else — RIB storage, route policy, route reflection, and any custom logic you need.

## Why Ze

**From the author of ExaBGP** — ExaBGP has been trusted in production networks worldwide for over a decade: traffic engineering, DDoS mitigation, route injection, SDN integration, and network monitoring. Ze applies those lessons to a ground-up redesign.

| Design goal | Ze's approach |
|-------------|---------------|
| Decouple RIB from FSM | RIB lives in a plugin — swap, extend, or skip it entirely |
| Flexible policy | Plugins in any language (Go, Python, Rust) implement your logic |
| Efficient parsing | Wire-first: lazy parsing via iterators, zero-copy forwarding |
| Low memory overhead | Attribute deduplication pools with reference counting |
| Testable BGP behavior | Built-in chaos testing with deterministic virtual clock |

## Features

### Protocol Support

Ze aims for comprehensive RFC 4271 compliance with broad multiprotocol support:

**Address Families:**
IPv4/IPv6 Unicast and Multicast, VPNv4/VPNv6 (RFC 4364, 4659), EVPN (RFC 7432, 9136), FlowSpec (RFC 8955/8956), Labeled Unicast (RFC 8277), BGP-LS (RFC 7752, 9085, 9514), VPLS (RFC 4761/4762), MVPN (RFC 6514), Route Target Constraint (RFC 4684), Mobile User Plane (draft-ietf-bess-mup-safi)

**Capabilities:**
4-byte ASN (RFC 6793), ADD-PATH (RFC 7911), Extended Messages (RFC 8654), Extended Next-Hop (RFC 8950), Graceful Restart (RFC 4724), Route Refresh (RFC 2918/7313), Role-based filtering (RFC 9234)

**Error Handling:**
Revised error handling with treat-as-withdraw (RFC 7606), shutdown communication (RFC 9003)

### Plugin Architecture

Ze ships with 15 plugins covering core BGP features and every supported address family:

**Behavioural plugins:**

| Plugin | Purpose |
|--------|---------|
| `bgp-rib` | Route Information Base — storage, best-path selection |
| `bgp-rr` | Route Reflector (RFC 4456) |
| `bgp-gr` | Graceful Restart (RFC 4724, RFC 9494) |
| `bgp-role` | Role negotiation and OTC filtering (RFC 9234) |
| `bgp-hostname` | Hostname capability (RFC 9018) |
| `bgp-llnh` | Link-local next-hop handling |

**NLRI family plugins:**

| Plugin | Purpose |
|--------|---------|
| `bgp-nlri-evpn` | EVPN types 1-5 — MAC-IP, Ethernet Segment, etc. (RFC 7432, 9136) |
| `bgp-nlri-vpn` | VPNv4/VPNv6 with Route Distinguisher and label stack (RFC 4364, 4659, 4798) |
| `bgp-nlri-flowspec` | FlowSpec traffic filter rules for IPv4/IPv6/VPN (RFC 8955, 8956) |
| `bgp-nlri-ls` | BGP-LS link-state topology including SRv6 (RFC 7752, 9085, 9514) |
| `bgp-nlri-labeled` | MPLS labeled unicast with label stacks (RFC 8277, 3032) |
| `bgp-nlri-vpls` | L2VPN VPLS pseudowires (RFC 4761, 4762) |
| `bgp-nlri-mvpn` | Multicast VPN (RFC 6514) |
| `bgp-nlri-mup` | Mobile User Plane (draft-ietf-bess-mup-safi) |
| `bgp-nlri-rtc` | Route Target Constraint (RFC 4684) |

**Write your own plugins** in any language. Plugins communicate with the engine over JSON-based IPC — no Go dependency required. The SDK handles the 5-stage startup protocol, event subscriptions, and command dispatch.

```
Engine → Plugin:  JSON events with base64-encoded wire bytes
Plugin → Engine:  text commands (update, forward, withdraw)
```

Go plugins compiled into the binary get a **performance fast path** using `ze.name` syntax in config instead of a command string:

```
run "ze.bgp-rib";          # fast: goroutine + Unix socket pair, no fork
run "ze plugin bgp-rib";   # slow: fork/exec subprocess + pipes
run "./my-plugin.py";      # external: any language, fork/exec
```

All three use the same IPC protocol — switching between modes is a one-line config change.

### Performance by Design

- **Buffer-first encoding** — all wire serialisation writes into pre-allocated buffers, never `make([]byte)` + return
- **Zero-copy forwarding** — when peers share the same negotiated capabilities, UPDATE bytes are forwarded unchanged
- **Lazy parsing** — NLRI and attributes are parsed on demand via iterators, not eagerly on receipt
- **Attribute deduplication** — per-type pools (ORIGIN, AS_PATH, MED, communities, etc.) with reference counting eliminate redundant storage

### ExaBGP Compatibility

Ze includes a compatibility bridge for ExaBGP plugins:

```
process exabgp-compat {
    run "ze exabgp plugin /path/to/your-exabgp-plugin.py";
}
```

Bidirectional JSON/command translation lets existing ExaBGP plugins work with Ze. A config migration tool converts ExaBGP configurations to Ze format:

```bash
ze config migrate exabgp.conf > ze.conf
```

### Testing

- **3700+ test functions** with race detector (`make unit-test`)
- **26 linters** via golangci-lint (`make lint`)
- **Functional tests** — encoding, decoding, plugin communication, config parsing, dynamic reload (`make functional-test`)
- **ExaBGP compatibility tests** — wire format validation against ExaBGP 5.0 (`make exabgp-test`)
- **Fuzz testing** — message parsing, attribute handling, NLRI decoding, config tokenisation (`make fuzz-test`)
- **Chaos testing** — in-process BGP simulation (`make chaos-test`)

### ze-test — Functional Test Runner

`ze-test` drives all functional testing with a built-in BGP test peer:

```bash
ze-test bgp encode --all        # run all encoding tests
ze-test bgp decode --list       # list available decode tests
ze-test bgp plugin 0 1 2        # run specific plugin tests by index
ze-test editor --all            # run interactive editor tests
ze-test peer --mode sink        # accept any BGP session, reply keepalive
ze-test peer --mode echo        # echo received messages back
ze-test peer --mode check test.ci  # validate wire output against expected
```

### ze-bgp-chaos — Chaos Testing

`ze-bgp-chaos` is a chaos monkey that simulates multiple BGP peers against a Ze route server, validates route propagation correctness, and injects faults:

```bash
# Basic run: 4 peers, random seed, run until Ctrl-C
ze-bgp-chaos

# Deterministic with specific parameters
ze-bgp-chaos --seed 42 --peers 8 --duration 60s --routes 200

# Multi-family with chaos control
ze-bgp-chaos --families ipv4/unicast,ipv6/unicast --chaos-rate 0.2

# In-process mode: mock network + virtual clock (fully deterministic)
ze-bgp-chaos --in-process --seed 42 --duration 30s

# Record event log, then replay or shrink a failure
ze-bgp-chaos --event-log run.ndjson --seed 42
ze-bgp-chaos --replay run.ndjson
ze-bgp-chaos --shrink run.ndjson

# Property-based validation
ze-bgp-chaos --properties all --convergence-deadline 5s
ze-bgp-chaos --properties list   # show available properties
```

Features include configurable iBGP/eBGP ratios, heavy-peer route flooding, route churn, replayable NDJSON event logs, automatic failure shrinking to minimal reproduction, property-based validation, and Prometheus metrics export.

## Quick Start

### Build

```bash
git clone https://codeberg.org/thomas-mangin/ze.git
cd ze
make build    # produces bin/ze
```

Requires **Go 1.25+**.

### Run

```bash
# Start the BGP daemon
bin/ze config.conf

# Validate a configuration
bin/ze config check config.conf

# Decode a BGP message
bin/ze bgp decode --update FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF...

# Run all tests
make test-all
```

### Configuration

Ze uses a hierarchical configuration syntax with two styles of template inheritance:

**Named template** — define once, inherit by name:

```
plugin {
    external rib {
        run "ze plugin bgp-rib";
    }
}

template {
    bgp {
        peer * {
            inherit-name rr-client;
            local-as 65533;
            capability {
                graceful-restart 120;
            }
            process rib {
                send {
                    state;
                    update;
                }
                receive {
                    update;
                }
            }
        }
    }
}

bgp {
    peer 10.0.0.1 {
        inherit rr-client;
        router-id 10.0.0.2;
        local-address 10.0.0.2;
        peer-as 65533;
        hold-time 180;
        update {
            attribute {
                origin igp;
                next-hop 10.0.0.2;
                community 30740:30740;
            }
            nlri {
                ipv4/unicast add 10.0.1.0/24;
                ipv4/unicast add 10.0.2.0/24;
            }
        }
        update {
            attribute {
                origin igp;
                next-hop 2A02:B80:0:2::1;
                local-preference 200;
            }
            nlri {
                ipv6/unicast add 2A02:B80:0:1::/64;
            }
        }
    }
}
```

**Glob template** — applies automatically to matching peers:

```
template {
    bgp {
        peer 10.0.0.* {
            local-as 65000;
            peer-as 65001;
        }
    }
}

bgp {
    peer 10.0.0.5 { }    # inherits config from pattern match
    peer 10.0.0.6 { }    # same
}
```

Multiple `update` blocks allow announcing routes in different address families with distinct attributes. YANG schema validation catches typos and unknown keys at load time — no silent misconfiguration.

### Interactive Configuration Editor

Ze includes a terminal-based configuration editor (`ze config edit`) with NOS-like `set`/`delete` commands:

```bash
ze config edit config.conf
```

Features include YANG-driven tab completion, hierarchical navigation with `edit`/`top`/`up`, live validation, diff preview before committing, and session recovery if the editor is interrupted. Changes can be applied to the running daemon via automatic reload notification.

## Architecture Overview

```
                         ZeBGP Engine
    ┌──────────┐  ┌──────────┐  ┌──────────┐
    │  Peer 1  │  │  Peer 2  │  │  Peer N  │    BGP sessions
    │   FSM    │  │   FSM    │  │   FSM    │    (per-peer goroutine)
    └────┬─────┘  └────┬─────┘  └────┬─────┘
         └─────────────┼─────────────┘
                       ▼
                ┌─────────────┐
                │   Reactor   │     Event loop, BGP cache
                └──────┬──────┘
                       │
    ═══════════════════╪═══════════════════════
        JSON events    │    commands            Process boundary
    ═══════════════════╪═══════════════════════
                       │
         ┌─────────────┼─────────────┐
         ▼             ▼             ▼
    ┌────────┐   ┌────────┐   ┌────────┐
    │  RIB   │   │  RR    │   │ Custom │    Plugins
    │ Plugin │   │ Plugin │   │ Plugin │    (any language)
    └────────┘   └────────┘   └────────┘
```

The engine handles TCP, FSM state transitions, message framing, capability negotiation, and keepalive timers. It passes wire bytes and structured events to plugins over Unix sockets. Plugins own all routing decisions.

This separation means you can:
- Run without a RIB at all (pure route injection / monitoring)
- Implement custom best-path selection
- Build a route reflector with non-standard policy
- Add application-specific logic in your preferred language

## Project Status

Ze is in **early active development** with no release yet. The protocol implementation is broad, the plugin architecture is functional, and the test suite is extensive — but interfaces are still evolving and breaking changes should be expected. Ze is not ready for production use.

That said, it already establishes BGP sessions, exchanges routes across all listed address families, and passes thousands of unit, functional, fuzz, and chaos tests. If you're interested in the design or want to contribute, now is a great time to get involved.

### An AI-Assisted Project

Ze exists because large-language-model coding assistants made it feasible. Writing a full BGP implementation from scratch — with comprehensive RFC compliance, a plugin architecture, and broad address family support — would be an enormous undertaking for a solo developer. AI tooling (Claude Code) made it realistic to attempt, handling the volume of boilerplate, protocol encoding, and test generation while the author focused on architecture and design decisions informed by over a decade of ExaBGP experience. The project's extensive rule system and spec-driven workflow were developed specifically to keep AI-generated code aligned with production-quality standards.

Contributions, feedback, and bug reports are welcome on the [issue tracker](https://codeberg.org/thomas-mangin/ze/issues).

## License

[GNU Affero General Public License v3.0](LICENSE)

## Links

- **Repository:** [codeberg.org/thomas-mangin/ze](https://codeberg.org/thomas-mangin/ze)
- **ExaBGP:** [github.com/Exa-Networks/exabgp](https://github.com/Exa-Networks/exabgp)
