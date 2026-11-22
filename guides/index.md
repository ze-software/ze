# Guides

Use these pages to install Ze, configure a network role, operate the system, and diagnose problems. Feature pages explain what Ze can do. Guides explain how to do it.

To test Ze with an AI coding agent, start with [Use Ze with an AI agent](ai-agent/): `ze create agent skill` sets up Claude Code or Codex with the guide your installed `ze` serves.

## Start and install

- [Quickstart](quickstart/): run two local BGP peers and verify a received route.
- [Install Ze](ze-install/): install the daemon or provision an appliance.
- [Build and install on Ubuntu](ubuntu-build-install/): compile, install, initialize the store, and start SSH.
- [Docker](docker/): build the image, keep state across recreation, and manage it with Compose.
- [VM and gokrazy appliance](appliance/): build and boot a disk image or installer ISO.
- [containerlab and netlab](netlab/): run the supplied topology with the lab image.
- [CLI tour](cli/) and [Command reference](command-reference/): use interactive, one-shot, pipe, and runtime commands.
- [Configuration](configuration-model/), [Configuration editor](config-editor/), and [Config archive](config-archive/): understand, edit, diff, commit, and archive configuration.
- [Lifecycle and rollback](lifecycle/), [Operations](operations/), and [Self-update](self-update/): reload, restart, update, and recover safely.
- [Environment variables](environment-variables/): runtime paths, logging overrides, and socket settings.

## Routing

- [BGP peering](bgp-peering/): configure peers, groups, families, and capabilities.
- [BGP policy](bgp-policy/), [Route filters and redistribution](redistribution/), and [Route injection](route-injection/): build import, export, validation, and redistribution policy.
- [RPKI origin validation](rpki/) and [IRR prefix filtering](irr-filtering/): validate origins with RTR caches and generate IRR filters from PeeringDB.
- [BGP resilience](bgp-resilience/), [Graceful restart](graceful-restart/), [ADD-PATH](add-path/), [BGP role (RFC 9234)](bgp-role/), and [BGP healthcheck](bgp-healthcheck/): configure refresh, graceful restart, persistence, reflection, multipath, and leak prevention.
- [OSPF](ospf/), [IS-IS](isis/), [static routes](static-routes/), and [policy routing](policy-routing/): bring IGP, static, and policy-based routing online.
- [BFD](bfd/) and [VRRP](vrrp/): configure fast failure detection and gateway redundancy.
- [MPLS](mpls/) and [RSVP-TE](rsvp-te/): configure label switching and traffic engineering.

## Services and security

- [Firewall](firewall/), [Traffic control](traffic-control/), and [VPP data plane](vpp/): protect, shape, and forward traffic.
- [FlowSpec protected router](flowspec-protected-router/) and [FlowSpec route reflector](flowspec-route-reflector/): relay or translate FlowSpec into local protection.
- [DDoS mitigation](ddos-mitigation/) and [Behavioral anomaly detection](anomaly/): detect floods and anomalies and respond locally or upstream.
- [AS112](as112/): run an anycast AS112 DNS sink.
- [PPPoE](pppoe/), [L2TP](l2tp/), and [IPsec](ipsec/): configure access and tunnel services.
- [Authentication](authentication/), [authorization](authorization/), [Operator access with SSH and RBAC](operator-access-rbac/), [RADIUS](radius/), and [TACACS+](tacacs/): secure operator access.

## Interfaces and automation

- [Use Ze with an AI agent](ai-agent/): set up Claude Code, Codex, OpenCode or Pi with `ze create agent skill`.
- [Plugins](plugins/): load, attach, and manage built-in and external plugins.
- [REST and gRPC](api/): use authenticated management APIs.
- [gNMI](gnmi/): use Capabilities, Get, Set, and Subscribe.
- [Web interface](web-interface/), [Looking Glass](looking-glass/), and [Public Looking Glass](public-looking-glass/): edit configuration, inspect routes, and publish a read-only looking glass.
- [MCP](mcp/overview/): connect tools, resources, and remote clients.
- [Fleet configuration](fleet-config/): distribute configuration to managed nodes.

## Diagnose, observe, and test

- [System readiness](system-readiness/): run offline doctor checks and inspect runtime health.
- [Production diagnostics](production-diagnostics/): start from symptoms and debug safely.
- [Monitoring](monitoring/), [BMP](bmp/), [Flow export](flow-export/), [MRT analysis](mrt-analysis/), and [Traffic usage](traffic-usage/): configure monitoring, telemetry, and packet/route exports.
- [Debugging tools](debugging-tools/): trace netlink, capture packets, and inspect state.
- [Logging](logging/) and [audit](audit/): inspect operational and security history.
- [Benchmarking](benchmarking/) and [Chaos testing](chaos-testing/): measure BGP performance and run deterministic failure scenarios.

Use [site search](../search/) for a specific protocol, command, or configuration leaf.
