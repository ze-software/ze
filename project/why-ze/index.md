# Why Ze / Why Not Ze

Ze is a network operating system (NOS) with programmable routing. It combines its own BGP engine
with Linux networking and network services, managed through one configuration
and plugin engine. You can use it as a route controller, or evaluate it as the
software that configures a router and its services.
<!-- source: feature-gates.txt -- BGP, routing, management and network-service feature boundaries -->

> Ze is pre-release software. This assessment was reviewed on November 16, 2026.
> APIs and configuration syntax can change. The
> [feature inventory](https://ze-software.net/reference/feature-status/) records
> implementation scope and maturity, including partial and experimental features.
> Corrections are welcome through the
> [issue tracker](https://github.com/ze-software/ze/issues).
<!-- source: website/data/github-releases.json -- recorded release inventory -->
<!-- source: features/configuration.md -- Scope, Scope gaps and Level -->

## Reasons to consider Ze

### Programmable routing, with control of the device around it

Ze keeps ExaBGP's controller model: a process can receive BGP events and inject
routes. Your controller can remain a separate program, written in a language
that can read and write the process protocol. The Go plugin SDK provides another
way to integrate custom logic.
<!-- source: pkg/plugin/sdk/sdk.go -- plugin SDK entry point -->
<!-- source: pkg/plugin/rpc/mux.go -- multiplexed process protocol -->

That controller can run alongside the device functions Ze provides. The code
includes interface configuration, routing-table installation, firewall rules
and IPsec, plus access services such as L2TP and PPPoE. Each has its own scope
and maturity record. A route-injection deployment can use the BGP functions
without configuring those services.
<!-- source: feature-gates.txt -- ze_bgp, ze_ike, ze_l2tp and system feature boundaries -->
<!-- source: features/route-installation.md -- route installation declaration -->
<!-- source: features/firewall.md -- firewall declaration -->

For an existing ExaBGP deployment, `ze exabgp migrate` converts configuration
and `ze exabgp plugin` bridges process scripts. The
[migration guide](https://ze-software.net/use-cases/exabgp-migration/) explains the differences to check
with your own configuration and scripts before replacing a deployment.
<!-- source: internal/plugins/exabgp/main.go -- cmdMigrate, cmdPlugin -->

### Operator tools that share the configuration model

Ze uses YANG to describe configuration. Its CLI and web editor use that schema,
so operators can inspect and edit the same configuration through either
interface. The editor provides completion, comparisons and commit validation.
The [configuration editor guide](../../guides/config-editor/index.md) shows the workflow.
<!-- source: features/configuration.md -- configuration model and transactional commits -->
<!-- source: features/web-interface.md -- web configuration editor -->

Operational tools include a live BGP dashboard, a looking glass and `ze doctor`
for configuration and runtime diagnostics. These are useful when the task is
to understand the device as well as exchange routes. The
[operations guide](../../guides/operations/index.md) describes the diagnostic commands.
<!-- source: README.md -- Why Ze, Tools for operators -->

### Automation through native APIs and MCP

REST and gRPC are implemented command transports. They share an execution
engine and expose command discovery and execution, with streaming for registered
streaming commands. The gRPC service also provides configuration sessions.
The [API guide](../../guides/api/index.md) documents authentication and listener requirements.
<!-- source: internal/component/api/engine.go -- APIEngine.ListCommands, Execute, Stream -->
<!-- source: internal/component/api/grpc/server.go -- zeServiceImpl.Execute, Stream, zeConfigServiceImpl -->

Ze's Model Context Protocol (MCP) server builds tools from command metadata,
so an AI assistant can discover commands and call the daemon. This gives an
assistant access to the operator's tools. The
[MCP guide](../../guides/mcp/overview/index.md) explains the connection and access controls.
<!-- source: internal/component/mcp/tools.go -- groupCommands, generateTools, buildToolDef -->

API compatibility still matters when you choose an implementation. Ze's gRPC
service uses Ze's command model, so an integration written for another daemon's
protobuf service needs an adapter. API completion is unimplemented, and the
REST/gRPC feature remains experimental. [gNMI](../../guides/gnmi/index.md) is a separate
configuration and telemetry interface.
<!-- source: internal/component/api/grpc/server.go -- zeServiceImpl.Execute, Complete -->
<!-- source: features/rest-grpc-api.md -- Scope gaps and Level -->
<!-- source: features/gnmi.md -- gNMI declaration -->

### Deployment as a service or an appliance

Ze can run as a Linux daemon or in a container. There is also an immutable
appliance image built with Linux and gokrazy, with a read-only root filesystem
and persistent storage under `/perm`. The [appliance guide](../../guides/appliance/index.md)
and [Docker guide](../../guides/docker/index.md) describe those deployment choices.
<!-- source: gokrazy/ze/config.json -- appliance package composition -->
<!-- source: docs/guide/appliance.md -- image contents and filesystem layout -->
<!-- source: docs/guide/docker.md -- container deployment -->

Optional subsystems have build tags, so you can compile a focused service or
include the wider network OS. An omitted subsystem's code and schema are
excluded from the build. The available functions therefore depend on both the
build and its configuration.
<!-- source: feature-gates.txt -- optional subsystem package declarations -->

### Routing policy, monitoring and protocol tools

BGP functions include route-server and route-reflector plugins, routing
information base (RIB) storage, graceful restart and filter plugins. Custom
plugins can extend policy through the same plugin interfaces. The
[plugin guide](../../developers/plugins/index.md) explains those interfaces and their
configuration requirements.
<!-- source: internal/component/plugin/all/all_ze_bgp.go -- BGP plugin composition -->
<!-- source: internal/component/bgp/plugins/rs/register.go -- route-server registration and filters -->

For monitoring and analysis, Ze includes BGP events, a looking glass and MRT
recording. The MRT recorder can write UPDATEs, all BGP messages and periodic
TABLE_DUMP_V2 RIB snapshots. MRT recording is experimental. The
[MRT guide](../../guides/mrt-analysis/index.md) also describes tools to inspect and replay
recorded data.
<!-- source: internal/plugins/mrt/component.go -- Component.Start, OnBGPMessage, ribDumpLoop -->
<!-- source: features/mrt-dump.md -- MRT scope and maturity -->

Standalone tools include `ze bgp decode`, `ze bgp encode` and
`ze config validate`. They are useful for protocol debugging and offline
configuration checks, even when you do not run Ze as the router.
<!-- source: cmd/ze/dispatch_bgp.go -- BGP command registration -->
<!-- source: internal/component/config/cli/cmd_validate.go -- runValidation -->

## Reasons to choose another implementation

### Your deployment requires a released, operationally established NOS

Ze has no published release. Its test suites and interoperability labs provide
evidence about specific behavior, but they do not establish a history of
production operation. A network that requires an established release and support
process needs an implementation that already meets those requirements.
<!-- source: website/data/github-releases.json -- empty release inventory -->

The distinction applies across the network OS. OSPFv2, OSPFv3 and IS-IS exist,
and the code includes BFD, MPLS, IPsec and subscriber access functions. OSPF,
IS-IS and PPPoE remain experimental and partial. Their presence in the binary
is a reason to evaluate the documented behavior in a lab, rather than assume
that the whole routing suite is ready for your network.
<!-- source: feature-gates.txt -- protocol and dataplane composition -->
<!-- source: features/ospf.md -- Scope, Scope gaps and Level -->
<!-- source: features/is-is.md -- Scope, Scope gaps and Level -->
<!-- source: features/pppoe-access.md -- Scope, Scope gaps and Level -->

### You need a feature Ze does not provide

The [feature inventory](https://ze-software.net/reference/feature-status/)
records what each feature implements and the evidence behind its maturity.
The [comparison](https://ze-software.net/compare/bgp/) identifies gaps such as BGP confederation
support and PIM multicast routing. Check the behavior your deployment depends
on, including the selected dataplane, rather than infer support from a protocol
name.
<!-- source: docs/comparison.md -- Key Gaps and protocol comparison tables -->
<!-- source: features/rest-grpc-api.md -- example of partial feature scope -->

The [standards coverage pages](https://ze-software.net/quality/rfc-compliance/)
describe what has been tested and what is missing. They list tests that still
need checking separately from behavior Ze does not implement. A feature label
or a large test count alone does not tell you whether Ze meets your network's
requirements.
<!-- source: docs/contributing/rfc-conformance-gates.md -- requirement evidence and discrimination records -->

### Your project requires a different license or maintenance model

Ze is licensed under AGPL-3.0-only. If your project requires a permissive
license, Ze does not meet that requirement. The [license](https://ze-software.net/license/) and
[contribution policy](https://ze-software.net/contribute/guide/) describe the project terms and contribution requirements.
<!-- source: LICENSE -- GNU Affero General Public License version 3 -->
<!-- source: README.md -- Contributing and License -->

Ze is developed with AI coding assistants, with Thomas Mangin directing the
architecture and development decisions. Specifications, tests and independent
review constrain that process. If your organization requires a larger
maintainer team or a contracted support arrangement, evaluate those requirements
separately from the feature list.
<!-- source: README.md -- AI-Assisted Development -->

## Engineering tradeoffs

### Plugins share a contract, with a choice of process boundary

Ze separates BGP functions such as RIB storage and route-server behavior into
plugins. Internal plugins run inside the daemon and use DirectBridge for typed
calls after startup. External plugins run in separate processes and communicate
through the process protocol. The separation lets a plugin be replaced without
moving its policy into the BGP engine.
<!-- source: internal/component/plugin/process/process.go -- Process.startInternal, startExternal -->
<!-- source: pkg/plugin/rpc/bridge.go -- DirectBridge -->

Internal plugins share the daemon's address space. External processes provide
a process boundary, with transport and serialization costs. Both forms still
need correct lifecycle handling and event ordering.
<!-- source: internal/component/plugin/process/process.go -- Process.startInternal, startExternal -->
<!-- source: pkg/plugin/rpc/bridge.go -- DirectBridge.SendCallback, DeliverEvents -->

### Performance depends on the workload

Ze is written in Go and uses pooled data and lazy wire parsing to reduce
allocation and decoding costs. `WireUpdate` retains the supplied payload and
parses sections when needed. These techniques reduce particular costs, and their
effect on throughput depends on the workload.
<!-- source: internal/component/bgp/wireu/wire_update.go -- NewWireUpdate, InitWireUpdate, ensureParsed -->
<!-- source: internal/component/bgp/attrpool/pool.go -- attribute pool -->

The [performance results](../../performance/bgp/index.md) describe measured scenarios, with
their dates and environments. They cannot establish a general percentage cost
for Go, a plugin boundary or another implementation. For a deployment decision,
use the [benchmarking guide](../../guides/benchmarking/index.md) to test your route counts,
policies and peer topology, and check that all expected routes arrive.
<!-- source: docs/performance.md -- Methodology, Environment and Reading the Results -->
<!-- source: docs/guide/benchmarking.md -- Before you report a number -->

### Process IPC and management APIs serve different callers

External plugins use a multiplexed text protocol, while operators and automation
can use REST, gRPC or MCP. A custom plugin therefore needs to implement the
plugin contract even if its application already uses a management API. The
[process protocol reference](../../developers/plugins/protocol/index.md) describes
that contract, and the [API guide](../../guides/api/index.md) describes daemon management.
<!-- source: pkg/plugin/rpc/framing.go -- multiplexed protocol framing -->
<!-- source: pkg/plugin/rpc/mux.go -- MuxConn -->
<!-- source: internal/component/api/engine.go -- APIEngine -->

## Further reading

- [Feature inventory](https://ze-software.net/reference/feature-status/): scope, maturity and evidence for each feature.
- [Feature comparison](https://ze-software.net/compare/bgp/): differences between routing implementations.
- [Architecture](../../architecture/index.md): the configuration engine, subsystems and plugins.
- [Quick start](../../guides/quickstart/index.md): a first lab deployment.
