# Dependencies

Ze uses 66 direct Go dependencies. The list and versions come from `go.mod`, and each entry explains how Ze uses that dependency, with references to the code that imports it.

## Terminal, Web & SSH Interfaces (11)

| Module | Version | Licence | Copyright | Why we use it |
| --- | --- | --- | --- | --- |
| `charm.land/bubbletea/v2` | `v2.1.1` | `MIT` | Charmbracelet, Inc. | The Elm-style TUI framework driving Ze's interactive CLI: config editor, dashboard, monitor, ping and traceroute views, run per SSH session. |
| `charm.land/bubbles/v2` | `v2.2.2` | `MIT` | Charmbracelet, Inc. | Pre-built TUI widgets (text input, viewport) for the CLI's interactive screens. |
| `charm.land/lipgloss/v2` | `v2.1.1` | `MIT` | Charmbracelet, Inc. | Styling and layout for the CLI's colors, borders, and widths. |
| `charm.land/wish/v2` | `v2.0.5` | `MIT` | Charmbracelet, Inc | SSH server middleware that wires each incoming session to run the CLI's Bubbletea program. |
| `charm.land/ssh` | `v0.4.3` | `BSD-3-Clause` | Glider Labs | SSH session and public-key types that the wish server and the CLI's key-based auth build on. |
| `github.com/a-h/templ` | `v0.3.1118` | `MIT` | Adrian Hesketh | Compiles the typed server-rendered components used by Ze's web configuration interface and BGP looking glass into Go. |
| `github.com/charmbracelet/colorprofile` | `v0.4.3` | `MIT` | Charmbracelet, Inc | Forces a deterministic color profile in render tests, so CLI screenshot/layout tests don't depend on the terminal running them. |
| `github.com/charmbracelet/x/ansi` | `v0.11.9` | `MIT` | Charmbracelet, Inc. | Strips terminal escape codes in the CLI editor tests, so a pattern matches the text the operator sees rather than its color codes. |
| `github.com/muesli/reflow` | `v0.3.0` | `MIT` | Christian Muehlhaeuser | ANSI-aware text width calculation, so the CLI's prompt and status bar line up correctly despite embedded color codes. |
| `github.com/creack/pty` | `v1.1.24` | `MIT` | Keith Rarick | Opens a pseudo-terminal in a Linux integration test that exercises serial console handling. |
| `github.com/chromedp/chromedp` | `v0.23.1` | `MIT` | Kenneth Shaw | Drives headless Chrome in a test that proves the website fills its published figures from the facts snapshot. |

## Networking & Protocols (7)

| Module | Version | Licence | Copyright | Why we use it |
| --- | --- | --- | --- | --- |
| `github.com/miekg/dns` | `v1.1.73` | `BSD-3-Clause` | The Go Authors, Miek Gieben | DNS message parsing and serving underneath Ze's DNS server engine, the AS112 blackhole plugin, and GeoDNS. It's the library underpinning CoreDNS. |
| `github.com/insomniacslk/dhcp` | `v0.0.0-20261014062924-c6bb46ad3f3a` | `BSD-3-Clause` | Andrea Barberio | DHCPv4/DHCPv6 client used for interface lease handling and the installer's disk-provisioning DHCP client. |
| `github.com/beevik/ntp` | `v1.6.0` | `BSD-2-Clause` | Brett Vickers | NTP client queries used by the ntp plugin. |
| `golang.zx2c4.com/wireguard/wgctrl` | `v0.0.0-20241231184526-a9ab2273dd10` | `MIT` | Matt Layher | Creates and configures WireGuard interfaces and peers from the interface netlink backend. |
| `github.com/mdlayher/packet` | `v1.2.0` | `MIT` | Matt Layher | Raw AF_PACKET sockets behind the `diag capture` CLI command. |
| `github.com/packetcap/go-pcap` | `v0.0.0-20260731105150-c86974bbfbcd` | `Apache-2.0` | No copyright holder stated | Only its filter subpackage: parses tcpdump/BPF-style filter expressions for `diag capture`. Packet capture itself goes through mdlayher/packet instead. |
| `github.com/cloudflare/circl` | `v1.6.5` | `BSD-3-Clause` | Cloudflare, The Go Authors | Only its Ed448 package: Ed448 signatures for IKEv2 authentication (RFC 8420). |

## Linux Kernel Interfaces (12)

| Module | Version | Licence | Copyright | Why we use it |
| --- | --- | --- | --- | --- |
| `github.com/vishvananda/netlink` | `v1.3.1` | `Apache-2.0` | Vishvananda Ishaya, Docker, Inc. | The main library for configuring the Linux network stack: links, addresses, routes, neighbors, bridges, VLANs, tunnels, IPsec, QoS. Used across interfaces, traffic, FIB, OSPF, ISIS, and MPLS. |
| `github.com/vishvananda/netns` | `v0.0.5` | `Apache-2.0` | Vishvananda Ishaya, Docker, Inc. | Linux network-namespace handles, used alongside netlink for namespace-scoped route watching. |
| `github.com/google/nftables` | `v0.3.0` | `Apache-2.0` | Google LLC | Programs the kernel's nftables firewall (tables, chains, rules, sets) over netlink. |
| `github.com/mdlayher/netlink` | `v1.11.2` | `MIT` | Matt Layher | Low-level generic netlink sockets, used to flush conntrack flow entries for flow export. |
| `github.com/mdlayher/genetlink` | `v1.4.0` | `MIT` | Matt Layher | Generic netlink family dialing, used to attach to the kernel's psample subsystem for sampled flow export. |
| `github.com/cilium/ebpf` | `v0.22.0` | `MIT` | Nathan Sweet, Cloudflare, Authors of Cilium | Assembles and attaches eBPF programs in pure Go, no clang or libbpf needed, to count traffic for the trafficusage plugin. |
| `filippo.io/mlockexe` | `v1.0.0` | `CC0-1.0 OR 0BSD` | No copyright holder stated | Locks the running ze executable's own file-backed pages in memory with mlock2(2) and MLOCK_ONFAULT, so the kernel cannot evict the daemon's text under memory pressure. A router short of memory reclaims page cache, and the running daemon IS page cache: without the lock it waits on the disk holding its own binary at the moment it must keep sessions alive. |
| `github.com/opencontainers/runc` | `v1.5.3` | `Apache-2.0` | Docker, Inc. | Provides the libcontainer runtime library used by the Linux container plugin, including its container init sequence. |
| `github.com/opencontainers/cgroups` | `v0.1.0` | `Apache-2.0` | The containerd Authors | Describes and manages the Linux control groups used by the container plugin for resource limits and cleanup. |
| `github.com/opencontainers/runtime-spec` | `v1.3.0` | `Apache-2.0` | The Linux Foundation | Provides OCI seccomp action definitions used by the container plugin's system-call filter tests. |
| `github.com/cyphar/filepath-securejoin` | `v0.7.0` | `BSD-3-Clause AND MPL-2.0` | Docker Inc & Go Authors, SUSE LLC | Resolves each path inside a container image layer without letting a symbolic link or '..' escape the image's root, when the container image store unpacks a layer on Linux. |
| `github.com/klauspost/compress` | `v1.20.4` | `BSD-3-Clause AND Apache-2.0 AND MIT` | The Go Authors, Klaus Post, The New York Times Company, The Snappy-Go Authors, The filepathx Authors | Decodes zstd-compressed container image layers in the container image store, with one decoder thread and a bounded window. |

## Routing & Dataplane (2)

| Module | Version | Licence | Copyright | Why we use it |
| --- | --- | --- | --- | --- |
| `github.com/gaissmai/bart` | `v0.30.0` | `MIT` | Karl Gaissmaier | Balanced Routing Table, built on Knuth's Allotment Routing Table (ART): the longest-prefix-match structure underneath Ze's RIB store. |
| `go.fd.io/govpp` | `v0.13.0` | `Apache-2.0` | Cisco and/or its affiliates | Binary-API client for FD.io VPP, used by the optional VPP dataplane backends for interfaces, firewall, traffic, FIB, and IKE. |

## Config, RPC & Telemetry (5)

| Module | Version | Licence | Copyright | Why we use it |
| --- | --- | --- | --- | --- |
| `github.com/openconfig/goyang` | `v1.6.4-0.20251113201521-a80f2799d7b9` | `Apache-2.0` | Google Inc. | Parses and validates the YANG module definitions behind Ze's config schema, CLI completion, and validation engine. |
| `github.com/openconfig/gnmi` | `v0.14.1` | `Apache-2.0` | Google Inc. | Generated gNMI protobuf/gRPC types that Ze's gNMI server implements for Get/Set/Subscribe/Capabilities. |
| `google.golang.org/grpc` | `v1.85.0` | `Apache-2.0` | gRPC authors | Backs Ze's gRPC servers: gNMI and Ze's own management API. |
| `google.golang.org/protobuf` | `v1.36.12` | `BSD-3-Clause` | The Go Authors | Runtime support for the generated protobuf message types behind Ze's gRPC API. |
| `google.golang.org/grpc/cmd/protoc-gen-go-grpc` | `v1.6.2` | `Apache-2.0` | gRPC authors | Build-time only: the protoc plugin used to regenerate the gRPC API's Go bindings from ze.proto. |

## Observability (5)

| Module | Version | Licence | Copyright | Why we use it |
| --- | --- | --- | --- | --- |
| `github.com/prometheus/client_golang` | `v1.25.0` | `Apache-2.0` | The Prometheus Authors | Ze's internal metrics backend: counters, gauges, and histograms against a private registry. |
| `github.com/prometheus/client_model` | `v0.6.3` | `Apache-2.0` | The Prometheus Authors | The metric sample types the L2TP subscriber tests and the BGP interop lab read a gauge's current value through. |
| `github.com/prometheus/common` | `v0.74.0` | `Apache-2.0` | The Prometheus Authors | Reads the Prometheus text exposition format back: the IPsec interop test parses Ze's own metrics answer and compares the gauges against the kernel. |
| `github.com/prometheus/procfs` | `v0.23.0` | `Apache-2.0` | The Prometheus Authors | Parses Linux /proc for the telemetry collector: CPU, memory, network device and socket stats, conntrack, softnet. |
| `github.com/sirupsen/logrus` | `v1.10.2` | `MIT` | Simon Eskildsen | Only to satisfy GoVPP's logging interface, bridged into Ze's own slog logger via a hook. Not used as Ze's own logger. |

## Standard Library Extensions (golang.org/x) (9)

| Module | Version | Licence | Copyright | Why we use it |
| --- | --- | --- | --- | --- |
| `golang.org/x/crypto` | `v0.59.0` | `BSD-3-Clause` | The Go Authors | SSH protocol primitives and certificate signing for the SSH server, plus password hashing. |
| `golang.org/x/mod` | `v0.43.0` | `BSD-3-Clause` | The Go Authors | Parses and rewrites go.mod files when the appliance build prepares an isolated instance of the tree, so an image is never built from the tracked working copy. |
| `golang.org/x/net` | `v0.62.0` | `BSD-3-Clause` | The Go Authors | Raw ICMP packet connections for the traceroute plugin, and hostname normalization for MCP's auth. |
| `golang.org/x/sync` | `v0.25.0` | `BSD-3-Clause` | The Go Authors | errgroup, which the plugin manager uses to drain every plugin's event queue concurrently and return the first failure. |
| `golang.org/x/sys` | `v0.50.0` | `BSD-3-Clause` | The Go Authors | Low-level Linux syscalls: disk sync and reboot in the installer, used throughout the Linux-specific components. |
| `golang.org/x/term` | `v0.48.0` | `BSD-3-Clause` | The Go Authors | Reads passwords without echo and detects an interactive terminal during CLI login. |
| `golang.org/x/text` | `v0.44.0` | `BSD-3-Clause` | The Go Authors | Prepares TACACS+ usernames with the PRECIS UsernameCasePreserved profile that RFC 8907 Section 3.7 requires. |
| `golang.org/x/tools` | `v0.52.0` | `BSD-3-Clause` | The Go Authors | Build-time only: goimports, pinned via a tools.go tracking file, never compiled into Ze's binaries. |
| `golang.org/x/vuln` | `v1.9.0` | `BSD-3-Clause` | The Go Authors | Build-time only: govulncheck, pinned through the tools.go tracking file, which reports known vulnerabilities in the modules Ze vendors. |

## Testing & Build Tooling (15)

| Module | Version | Licence | Copyright | Why we use it |
| --- | --- | --- | --- | --- |
| `github.com/gofrs/flock` | `v0.13.2` | `BSD-3-Clause` | The Gofrs, Tim Heckman | Locks pending RFC approval records while development tools update them, so concurrent writers preserve each other's approvals. |
| `github.com/moby/sys/user` | `v0.5.0` | `Apache-2.0` | No copyright holder stated | Parses container account files through the runtime library; regression tests check valid groups and rejection of oversized group lines. |
| `github.com/moby/patternmatcher` | `v0.6.1` | `Apache-2.0` | Docker, Inc. | Applies .dockerignore patterns when preparing the files sent to Docker for lab image builds. |
| `mvdan.cc/sh/v3` | `v3.14.1` | `BSD-3-Clause` | Daniel Martí | Parses Bash commands so the agent safety checks can inspect the commands a session is about to run. |
| `github.com/stretchr/testify` | `v1.12.1` | `MIT` | Mat Ryer, Tyler Bunnell and contributors | Assertion and require helpers used across the Go unit test suite. |
| `github.com/pelletier/go-toml/v2` | `v2.4.3` | `MIT` | Thomas Pelletier | Parses kl2tpd TOML configuration in the L2TP scale-input tests. |
| `github.com/gokrazy/gokapi` | `v0.0.0-20251205165548-0927bab199d4` | `BSD-3-Clause` | the gokrazy authors | Exercises the gokrazy on-device API's remote connection setup in appliance updater tests, proving reserved password bytes survive URL construction and invalid-port errors omit credentials. |
| `github.com/gokrazy/tools` | `v0.0.0-20261026191944-942a8faaca93` | `BSD-3-Clause` | the gokrazy authors | Drives gokrazy appliance image builds from Ze's appliance build tooling. |
| `github.com/gokrazy/updater` | `v0.0.0-20260620140544-0a84d8ab3878` | `BSD-3-Clause` | the gokrazy authors | Referenced only in a regression test against Ze's own vendored update-push logic, written locally after a bug was found upstream. Not used in production. |
| `github.com/sivchari/gomu` | `v0.3.0` | `MIT` | sivchari | Mutation-testing tool, run as a native Go command to advisory-score how well the test suite exercises the code. Not a build or CI gate. |
| `gopkg.in/yaml.v3` | `v3.0.1` | `MIT AND Apache-2.0` | Kirill Simonov, Canonical Ltd | Parses GitHub Actions workflow YAML in verification tests so aliases, duplicate keys, and malformed trigger structures fail closed. |
| `github.com/golangci/golangci-lint/v2` | `v2.15.0` | `GPL-3.0` | No copyright holder stated | The linter behind `./le go lint run`, pinned through the tools.go tracking file so every machine and every CI job lints with one version. |
| `honnef.co/go/tools` | `v0.8.2` | `MIT` | Dominik Honnef | Staticcheck, run beside golangci-lint from the same tools.go pin, for the analyses golangci-lint does not carry. |
| `github.com/yuin/goldmark` | `v1.8.7` | `MIT` | Yusuke Inuzuka | Renders the Markdown of `docs/` and the website into the published HTML pages, in the Go site build that replaced the retired Python renderers. |
| `github.com/anmitsu/go-shlex` | `v0.0.0-20200514113438-38f4b401e2be` | `MIT` | anmitsu <anmitsu.s@gmail.com> | Splits an operator-typed command line into words for the stress-reproduction runner, so a quoted argument survives. |

## Also in the shipped binary

The shipped `ze` binary also carries these works: the Go modules the direct dependencies bring in, Go itself, the web interface assets, and code copied or ported into Ze. None of them is a direct dependency, so the count above leaves them out. Module versions come from `go.mod`; the others are the versions of the copies Ze carries. `ze thanks` lists the works one binary carries, and `ze thanks licence` prints their licence texts.

### Indirect Go Modules (45)

| Name | Version | Licence | Copyright |
| --- | --- | --- | --- |
| [`charm.land/log/v2`](https://github.com/charmbracelet/log) | `v2.0.1` | `MIT` | Charmbracelet, Inc |
| [`github.com/antihax/optional`](https://github.com/antihax/optional) | `v1.0.0` | `MIT` | Adam Hintz |
| [`github.com/atotto/clipboard`](https://github.com/atotto/clipboard) | `v0.1.4` | `BSD-3-Clause` | Ato Araki |
| [`github.com/beorn7/perks`](https://github.com/beorn7/perks) | `v1.0.1` | `MIT` | Blake Mizerany |
| [`github.com/cespare/xxhash/v2`](https://github.com/cespare/xxhash/v2) | `v2.3.0` | `MIT` | Caleb Spare |
| [`github.com/charmbracelet/keygen`](https://github.com/charmbracelet/keygen) | `v0.5.4` | `MIT` | Charmbracelet, Inc |
| [`github.com/charmbracelet/ultraviolet`](https://github.com/charmbracelet/ultraviolet) | `v0.0.0-20261023060817-a45b7c86333f` | `MIT` | Charmbracelet, Inc |
| [`github.com/charmbracelet/x/term`](https://github.com/charmbracelet/x/term) | `v0.2.2` | `MIT` | Charmbracelet, Inc. |
| [`github.com/charmbracelet/x/termios`](https://github.com/charmbracelet/x/termios) | `v0.1.1` | `MIT` | Charmbracelet, Inc. |
| [`github.com/charmbracelet/x/windows`](https://github.com/charmbracelet/x/windows) | `v0.2.2` | `MIT` | Charmbracelet, Inc. |
| [`github.com/clipperhouse/displaywidth`](https://github.com/clipperhouse/displaywidth) | `v0.11.0` | `MIT` | Matt Sherman |
| [`github.com/clipperhouse/uax29/v2`](https://github.com/clipperhouse/uax29/v2) | `v2.7.0` | `MIT` | Matt Sherman |
| [`github.com/containerd/console`](https://github.com/containerd/console) | `v1.0.5` | `Apache-2.0` | The containerd Authors |
| [`github.com/coreos/go-systemd/v22`](https://github.com/coreos/go-systemd/v22) | `v22.7.0` | `Apache-2.0` | CoreOS, Inc |
| [`github.com/donovanhide/eventsource`](https://github.com/donovanhide/eventsource) | `v0.0.0-20210830082556-c59027999da0` | `Apache-2.0` | eventsource authors |
| [`github.com/fsnotify/fsnotify`](https://github.com/fsnotify/fsnotify) | `v1.10.1` | `BSD-3-Clause` | The Go Authors, fsnotify Authors |
| [`github.com/ftrvxmtrx/fd`](https://github.com/ftrvxmtrx/fd) | `v0.0.0-20150925145434-c6d800382fff` | `MIT` | Serge Zirukin |
| [`github.com/go-logfmt/logfmt`](https://github.com/go-logfmt/logfmt) | `v0.6.1` | `MIT` | go-logfmt |
| [`github.com/godbus/dbus/v5`](https://github.com/godbus/dbus/v5) | `v5.2.4` | `BSD-2-Clause` | Georg Reinke (<guelfey at gmail dot com>), Google |
| [`github.com/gokrazy/internal`](https://github.com/gokrazy/internal) | `v0.0.0-20260625065634-6994f9152c44` | `BSD-3-Clause` | the gokrazy authors |
| [`github.com/google/go-cmp`](https://github.com/google/go-cmp) | `v0.7.0` | `BSD-3-Clause` | The Go Authors |
| [`github.com/google/renameio/v2`](https://github.com/google/renameio/v2) | `v2.0.2` | `Apache-2.0` | Google Inc. |
| [`github.com/lucasb-eyer/go-colorful`](https://github.com/lucasb-eyer/go-colorful) | `v1.4.1` | `MIT` | Lucas Beyer |
| [`github.com/lunixbochs/struc`](https://github.com/lunixbochs/struc) | `v0.0.0-20241101090106-8d528fa2c543` | `MIT` | Ryan Hileman |
| [`github.com/mattn/go-isatty`](https://github.com/mattn/go-isatty) | `v0.0.24` | `MIT` | Yasuhiro MATSUMOTO <mattn.jp@gmail.com> |
| [`github.com/mattn/go-runewidth`](https://github.com/mattn/go-runewidth) | `v0.0.31` | `MIT` | Yasuhiro Matsumoto |
| [`github.com/mdlayher/socket`](https://github.com/mdlayher/socket) | `v0.8.0` | `MIT` | Matt Layher |
| [`github.com/moby/sys/capability`](https://github.com/moby/sys/capability) | `v0.5.0` | `BSD-2-Clause` | The Capability Authors, Suryandaru Triandana <syndtr@gmail.com> |
| [`github.com/moby/sys/mountinfo`](https://github.com/moby/sys/mountinfo) | `v0.7.3` | `Apache-2.0` | No copyright holder stated |
| [`github.com/moby/sys/userns`](https://github.com/moby/sys/userns) | `v0.2.1` | `Apache-2.0` | No copyright holder stated |
| [`github.com/mrunalp/fileutils`](https://github.com/mrunalp/fileutils) | `v0.5.1` | `Apache-2.0` | Docker, Inc. |
| [`github.com/muesli/cancelreader`](https://github.com/muesli/cancelreader) | `v0.2.2` | `MIT` | Erik Geiser and Christian Muehlhaeuser |
| [`github.com/munnerz/goautoneg`](https://github.com/munnerz/goautoneg) | `v0.0.0-20191010083416-a7dc8b61c822` | `BSD-3-Clause` | Open Knowledge Foundation Ltd |
| [`github.com/opencontainers/selinux`](https://github.com/opencontainers/selinux) | `v1.15.1` | `Apache-2.0` | No copyright holder stated |
| [`github.com/pandatix/go-abnf`](https://github.com/pandatix/go-abnf) | `v0.5.5` | `MIT` | Lucas TESSON - PandatiX <lucastesson@protonmail.com> |
| [`github.com/pierrec/lz4/v4`](https://github.com/pierrec/lz4/v4) | `v4.1.33` | `BSD-3-Clause` | Pierre Curto |
| [`github.com/pires/go-proxyproto`](https://github.com/pires/go-proxyproto) | `v0.16.0` | `Apache-2.0` | Paulo Pires |
| [`github.com/rivo/uniseg`](https://github.com/rivo/uniseg) | `v0.4.8-0.20261110175422-3a370411ca06` | `MIT` | Oliver Kuederle |
| [`github.com/spf13/cobra`](https://github.com/spf13/cobra) | `v1.10.2` | `Apache-2.0` | The Cobra Authors |
| [`github.com/spf13/pflag`](https://github.com/spf13/pflag) | `v1.1.0` | `BSD-3-Clause` | Alex Ogier, The Go Authors |
| [`github.com/u-root/uio`](https://github.com/u-root/uio) | `v0.0.0-20240224005618-d2acac8f3701` | `BSD-3-Clause` | u-root Authors |
| [`github.com/xo/terminfo`](https://github.com/xo/terminfo) | `v1.3.1` | `MIT` | Anmol Sethi |
| [`golang.org/x/crypto/x509roots/fallback`](https://golang.org/x/crypto/x509roots/fallback) | `v0.0.0-20261020153306-9f2b28d4331a` | `BSD-3-Clause` | The Go Authors |
| [`golang.org/x/oauth2`](https://golang.org/x/oauth2) | `v0.38.0` | `BSD-3-Clause` | The Go Authors |
| [`google.golang.org/genproto/googleapis/rpc`](https://google.golang.org/genproto/googleapis/rpc) | `v0.0.0-20261026165753-2ffab32bce7e` | `Apache-2.0` | Google LLC |

### Go Toolchain (1)

| Name | Version | Licence | Copyright |
| --- | --- | --- | --- |
| [`Go standard library and runtime`](https://github.com/golang/go) | `go1.27.4` | `BSD-3-Clause` | The Go Authors |

### Web Interface Assets (2)

| Name | Version | Licence | Copyright |
| --- | --- | --- | --- |
| [`htmx`](https://unpkg.com/htmx.org@4.0.0/) | `4.0.0` | `0BSD` | No copyright holder stated |
| [`uPlot`](https://unpkg.com/uplot@1.6.32/) | `1.6.32` | `MIT` | Leon Sorokin |

### REST API Explorer (15)

| Name | Version | Licence | Copyright |
| --- | --- | --- | --- |
| [`swagger-ui`](https://unpkg.com/swagger-ui-dist@5.32.2/) | `5.32.2` | `Apache-2.0` | SmartBear Software Inc. |
| [`normalize.css`](https://github.com/necolas/normalize.css) | `7.0.0` | `MIT` | Nicolas Gallagher and Jonathan Neal |
| [`classnames`](https://github.com/JedWatson/classnames) | `2.5.1` | `MIT` | Jed Watson |
| [`deep-extend`](https://github.com/unclechu/node-deep-extend) | `0.6.0` | `MIT` | Viacheslav Lotsmanov |
| [`buffer`](https://github.com/feross/buffer) | `6.0.3` | `MIT` | Feross Aboukhadijeh |
| [`fast-json-patch`](https://github.com/Starcounter-Jack/JSON-Patch) | `3.1.1` | `MIT` | Joachim Wester |
| [`repeat-string`](https://github.com/jonschlinkert/repeat-string) | `1.6.1` | `MIT` | Jon Schlinkert |
| [`dompurify`](https://github.com/cure53/DOMPurify) | `3.3.2` | `Apache-2.0 OR MPL-2.0` | Dr.-Ing. Mario Heiderich, Cure53 |
| [`ieee754`](https://github.com/feross/ieee754) | `1.2.1` | `BSD-3-Clause` | Fair Oaks Labs, Inc. |
| [`js-yaml`](https://github.com/nodeca/js-yaml) | `4.1.1` | `MIT` | Vitaly Puzrin |
| [`safe-buffer`](https://github.com/feross/safe-buffer) | `5.2.1` | `MIT` | Feross Aboukhadijeh |
| [`react`](https://github.com/facebook/react) | `18.3.1` | `MIT` | Facebook, Inc. and its affiliates |
| [`react-dom`](https://github.com/facebook/react) | `18.3.1` | `MIT` | Facebook, Inc. and its affiliates |
| [`scheduler`](https://github.com/facebook/react) | `0.23.2` | `MIT` | Facebook, Inc. and its affiliates |
| [`use-sync-external-store`](https://github.com/facebook/react) | `1.4.0` | `MIT` | Meta Platforms, Inc. and affiliates |

### Copied and Ported Source (2)

| Name | Version | Licence | Copyright |
| --- | --- | --- | --- |
| [`gokrazy FAT writer`](https://github.com/gokrazy/internal) | not versioned | `BSD-3-Clause` | the gokrazy authors |
| [`ExaBGP`](https://github.com/Exa-Networks/exabgp) | `f9bfb853b924e97deba646b0bd50693ab07c4f3e` | `BSD-3-Clause` | Exa Networks Limited, Thomas Mangin, Nikita Shirokov, Orange, Alcatel-Lucent, Job Snijders, Evelio Vila, Hiroki Shirokura |

### Registry and Profile Data (2)

| Name | Version | Licence | Copyright |
| --- | --- | --- | --- |
| [`Moby default seccomp profile`](https://github.com/moby/profiles) | `f862455628f78f84c937e967e4a03902fb1dee11` | `Apache-2.0` | No copyright holder stated |
| [`IANA protocol registries`](https://www.iana.org/protocols) | `2025-10-09` | `CC0-1.0` | No copyright holder stated |
