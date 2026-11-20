# Dependencies

Ze uses 42 direct Go dependencies. The list and versions come from `go.mod`, and each entry explains how Ze uses that dependency, with references to the code that imports it.

## Terminal, Web & SSH Interfaces (9)

| Module | Version | Licence | Copyright | Why we use it |
| --- | --- | --- | --- | --- |
| `charm.land/bubbletea/v2` | `v2.0.8` | `MIT` | Charmbracelet, Inc. | The Elm-style TUI framework driving Ze's interactive CLI: config editor, dashboard, monitor, ping and traceroute views, run per SSH session. |
| `charm.land/bubbles/v2` | `v2.1.1` | `MIT` | Charmbracelet, Inc. | Pre-built TUI widgets (text input, viewport) for the CLI's interactive screens. |
| `charm.land/lipgloss/v2` | `v2.0.6` | `MIT` | Charmbracelet, Inc. | Styling and layout for the CLI's colors, borders, and widths. |
| `charm.land/wish/v2` | `v2.0.3` | `MIT` | Charmbracelet, Inc | SSH server middleware that wires each incoming session to run the CLI's Bubbletea program. |
| `charm.land/ssh` | `v0.4.3` | `BSD-3-Clause` | Glider Labs | SSH session and public-key types that the wish server and the CLI's key-based auth build on. |
| `github.com/a-h/templ` | `v0.3.1020` | `MIT` | Adrian Hesketh | Compiles the typed server-rendered components used by Ze's web configuration interface and BGP looking glass into Go. |
| `github.com/charmbracelet/colorprofile` | `v0.4.3` | `MIT` | Charmbracelet, Inc | Forces a deterministic color profile in render tests, so CLI screenshot/layout tests don't depend on the terminal running them. |
| `github.com/muesli/reflow` | `v0.3.0` | `MIT` | Christian Muehlhaeuser | ANSI-aware text width calculation, so the CLI's prompt and status bar line up correctly despite embedded color codes. |
| `github.com/creack/pty` | `v1.1.24` | `MIT` | Keith Rarick | Opens a pseudo-terminal in a Linux integration test that exercises serial console handling. |

## Networking & Protocols (6)

| Module | Version | Licence | Copyright | Why we use it |
| --- | --- | --- | --- | --- |
| `github.com/miekg/dns` | `v1.1.72` | `BSD-3-Clause` | The Go Authors, Miek Gieben | DNS message parsing and serving underneath Ze's DNS server engine, the AS112 blackhole plugin, and GeoDNS. It's the library underpinning CoreDNS. |
| `github.com/insomniacslk/dhcp` | `v0.0.0-20260719225207-c76316d4aa82` | `BSD-3-Clause` | Andrea Barberio | DHCPv4/DHCPv6 client used for interface lease handling and the installer's disk-provisioning DHCP client. |
| `github.com/beevik/ntp` | `v1.5.0` | `BSD-2-Clause` | Brett Vickers | NTP client queries used by the ntp plugin. |
| `golang.zx2c4.com/wireguard/wgctrl` | `v0.0.0-20241231184526-a9ab2273dd10` | `MIT` | Matt Layher | Creates and configures WireGuard interfaces and peers from the interface netlink backend. |
| `github.com/mdlayher/packet` | `v1.1.2` | `MIT` | Matt Layher | Raw AF_PACKET sockets behind the `diag capture` CLI command. |
| `github.com/packetcap/go-pcap` | `v0.0.0-20251215121130-f2cf9f991e7c` | `Apache-2.0` | No copyright holder stated | Only its filter subpackage: parses tcpdump/BPF-style filter expressions for `diag capture`. Packet capture itself goes through mdlayher/packet instead. |

## Linux Kernel Interfaces (6)

| Module | Version | Licence | Copyright | Why we use it |
| --- | --- | --- | --- | --- |
| `github.com/vishvananda/netlink` | `v1.3.1` | `Apache-2.0` | Vishvananda Ishaya, Docker, Inc. | The main library for configuring the Linux network stack: links, addresses, routes, neighbors, bridges, VLANs, tunnels, IPsec, QoS. Used across interfaces, traffic, FIB, OSPF, ISIS, and MPLS. |
| `github.com/vishvananda/netns` | `v0.0.5` | `Apache-2.0` | Vishvananda Ishaya, Docker, Inc. | Linux network-namespace handles, used alongside netlink for namespace-scoped route watching. |
| `github.com/google/nftables` | `v0.3.0` | `Apache-2.0` (the licence file names no holder; the holder line is in the source files) | Google LLC | Programs the kernel's nftables firewall (tables, chains, rules, sets) over netlink. |
| `github.com/mdlayher/netlink` | `v1.11.2` | `MIT` | Matt Layher | Low-level generic netlink sockets, used to flush conntrack flow entries for flow export. |
| `github.com/mdlayher/genetlink` | `v1.4.0` | `MIT` | Matt Layher | Generic netlink family dialing, used to attach to the kernel's psample subsystem for sampled flow export. |
| `github.com/cilium/ebpf` | `v0.22.0` | `MIT` | Nathan Sweet, Cloudflare, Authors of Cilium | Assembles and attaches eBPF programs in pure Go, no clang or libbpf needed, to count traffic for the trafficusage plugin. |

## Routing & Dataplane (2)

| Module | Version | Licence | Copyright | Why we use it |
| --- | --- | --- | --- | --- |
| `github.com/gaissmai/bart` | `v0.29.0` | `MIT` | Karl Gaissmaier | Balanced Routing Table, built on Knuth's Allotment Routing Table (ART): the longest-prefix-match structure underneath Ze's RIB store. |
| `go.fd.io/govpp` | `v0.13.0` | `Apache-2.0` | Cisco and/or its affiliates | Binary-API client for FD.io VPP, used by the optional VPP dataplane backends for interfaces, firewall, traffic, FIB, and IKE. |

## Config, RPC & Telemetry (5)

| Module | Version | Licence | Copyright | Why we use it |
| --- | --- | --- | --- | --- |
| `github.com/openconfig/goyang` | `v1.6.3` | `Apache-2.0` | Google Inc. | Parses and validates the YANG module definitions behind Ze's config schema, CLI completion, and validation engine. |
| `github.com/openconfig/gnmi` | `v0.14.1` | `Apache-2.0` | Google Inc. | Generated gNMI protobuf/gRPC types that Ze's gNMI server implements for Get/Set/Subscribe/Capabilities. |
| `google.golang.org/grpc` | `v1.83.0` | `Apache-2.0` | gRPC authors | Backs Ze's gRPC servers: gNMI and Ze's own management API. |
| `google.golang.org/protobuf` | `v1.36.12` | `BSD-3-Clause` | The Go Authors | Runtime support for the generated protobuf message types behind Ze's gRPC API. |
| `google.golang.org/grpc/cmd/protoc-gen-go-grpc` | `v1.6.2` | `Apache-2.0` | gRPC authors | Build-time only: the protoc plugin used to regenerate the gRPC API's Go bindings from ze.proto. |

## Observability (3)

| Module | Version | Licence | Copyright | Why we use it |
| --- | --- | --- | --- | --- |
| `github.com/prometheus/client_golang` | `v1.24.1` | `Apache-2.0` | The Prometheus Authors | Ze's internal metrics backend: counters, gauges, and histograms against a private registry. |
| `github.com/prometheus/procfs` | `v0.21.1` | `Apache-2.0` | The Prometheus Authors | Parses Linux /proc for the telemetry collector: CPU, memory, network device and socket stats, conntrack, softnet. |
| `github.com/sirupsen/logrus` | `v1.10.0` | `MIT` | Simon Eskildsen | Only to satisfy GoVPP's logging interface, bridged into Ze's own slog logger via a hook. Not used as Ze's own logger. |

## Standard Library Extensions (golang.org/x) (6)

| Module | Version | Licence | Copyright | Why we use it |
| --- | --- | --- | --- | --- |
| `golang.org/x/crypto` | `v0.55.0` | `BSD-3-Clause` | The Go Authors | SSH protocol primitives and certificate signing for the SSH server, plus password hashing. |
| `golang.org/x/mod` | `v0.40.0` | `BSD-3-Clause` | The Go Authors | Parses and rewrites go.mod files when the appliance build prepares an isolated instance of the tree, so an image is never built from the tracked working copy. |
| `golang.org/x/net` | `v0.58.0` | `BSD-3-Clause` | The Go Authors | Raw ICMP packet connections for the traceroute plugin, and hostname normalization for MCP's auth. |
| `golang.org/x/sys` | `v0.47.0` | `BSD-3-Clause` | The Go Authors | Low-level Linux syscalls: disk sync and reboot in the installer, used throughout the Linux-specific components. |
| `golang.org/x/term` | `v0.45.0` | `BSD-3-Clause` | The Go Authors | Reads passwords without echo and detects an interactive terminal during CLI login. |
| `golang.org/x/tools` | `v0.49.0` | `BSD-3-Clause` | The Go Authors | Build-time only: goimports, pinned via a tools.go tracking file, never compiled into Ze's binaries. |

## Testing & Build Tooling (5)

| Module | Version | Licence | Copyright | Why we use it |
| --- | --- | --- | --- | --- |
| `github.com/stretchr/testify` | `v1.12.1` | `MIT` | Mat Ryer, Tyler Bunnell and contributors | Assertion and require helpers used across the Go unit test suite. |
| `github.com/gokrazy/tools` | `v0.0.0-20260703063348-3fe400c13246` | `BSD-3-Clause` | the gokrazy authors | Drives gokrazy appliance image builds from Ze's appliance build tooling. |
| `github.com/gokrazy/updater` | `v0.0.0-20260620140544-0a84d8ab3878` | `BSD-3-Clause` | the gokrazy authors | Referenced only in a regression test against Ze's own vendored update-push logic, written locally after a bug was found upstream. Not used in production. |
| `github.com/sivchari/gomu` | `v0.2.1` | `MIT` | sivchari | Mutation-testing tool, run via the Makefile to advisory-score how well the test suite actually exercises the code. Not a build or CI gate. |
| `gopkg.in/yaml.v3` | `v3.0.1` | `MIT AND Apache-2.0` | Kirill Simonov, Canonical Ltd | Parses GitHub Actions workflow YAML in verification tests so aliases, duplicate keys, and malformed trigger structures fail closed. |

## Also in the shipped binary

The shipped `ze` binary also carries these works: the Go modules the direct dependencies bring in, Go itself, the web interface assets, and code copied or ported into Ze. None of them is a direct dependency, so the count above leaves them out. Module versions come from `go.mod`; the others are the versions of the copies Ze carries. `ze thanks` lists the works one binary carries, and `ze thanks licence` prints their licence texts.

### Indirect Go Modules (1)

| Name | Version | Licence | Copyright |
| --- | --- | --- | --- |
| [`github.com/antihax/optional`](https://github.com/antihax/optional) | `v1.0.0` | `MIT` | Adam Hintz |

### Go Toolchain (1)

| Name | Version | Licence | Copyright |
| --- | --- | --- | --- |
| [`Go standard library and runtime`](https://github.com/golang/go) | `go1.27` | `BSD-3-Clause` | The Go Authors |

### Web Interface Assets (1)

| Name | Version | Licence | Copyright |
| --- | --- | --- | --- |
| [`htmx`](https://unpkg.com/htmx.org@4.0.0/) | `4.0.0` | `0BSD` | No copyright holder stated |

### Copied and Ported Source (1)

| Name | Version | Licence | Copyright |
| --- | --- | --- | --- |
| [`gokrazy FAT writer`](https://github.com/gokrazy/internal) | not versioned | `BSD-3-Clause` | the gokrazy authors |
