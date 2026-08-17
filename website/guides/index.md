# Guides

Use these pages to install Ze, configure a network role, operate the system, and diagnose problems. Feature pages explain what Ze can do. Guides explain how to do it.

## Start and install

- [Quickstart](quickstart/): bring up two BGP peers quickly.
- [Install Ze](ze-install/): install the daemon or provision an appliance.
- [Build and install on Ubuntu](ubuntu-build-install/): compile, install, create zefs, and start SSH.
- [CLI tour](cli/): use interactive, one-shot, pipe, and runtime commands.
- [Configuration](configuration-model/): understand the hierarchical configuration format.
- [Lifecycle and rollback](lifecycle/): reload, restart, archive, update, and recover.

## Routing

- [BGP peering](bgp-peering/): configure peers, groups, families, and capabilities.
- [BGP policy](bgp-policy/): build import, export, validation, and redistribution policy.
- [BGP resilience](bgp-resilience/): configure refresh, graceful restart, persistence, reflection, and multipath.
- [OSPF](ospf/), [IS-IS](isis/), and [static routes](static-routes/): bring routing protocols online.
- [BFD](bfd/): configure fast failure detection.
- [MPLS](mpls/) and [RSVP-TE](rsvp-te/): configure label switching and traffic engineering.

## Services and security

- [Firewall and policy routing](firewall/): protect and steer traffic.
- [FlowSpec protected router](flowspec-protected-router/): translate FlowSpec into local protection.
- [DDoS mitigation](ddos-mitigation/): detect floods and respond locally or upstream.
- [PPPoE](pppoe/), [L2TP](l2tp/), and [IPsec](ipsec/): configure access and tunnel services.
- [Authentication](authentication/), [authorization](authorization/), [RADIUS](radius/), and [TACACS+](tacacs/): secure operator access.

## Interfaces and automation

- [REST and gRPC](api/): use authenticated management APIs.
- [gNMI](gnmi/): use Capabilities, Get, Set, and Subscribe.
- [Web interface](web-interface/): edit configuration and run commands in the browser.
- [MCP](mcp/overview/): connect tools, resources, and remote clients.
- [Fleet configuration](fleet-config/): distribute configuration to managed nodes.

## Diagnose and recover

- [System readiness](system-readiness/): run offline doctor checks and inspect runtime health.
- [Production diagnostics](production-diagnostics/): start from symptoms and debug safely.
- [Monitoring](monitoring/): configure flow export, MRT, and operational checks.
- [Debugging tools](debugging-tools/): trace netlink, capture packets, and inspect state.
- [Logging](logging/) and [audit](audit/): inspect operational and security history.

Use [site search](../search/) for a specific protocol, command, or configuration leaf.
