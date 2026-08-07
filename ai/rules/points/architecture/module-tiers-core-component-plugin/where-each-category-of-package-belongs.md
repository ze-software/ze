---
kind: directive
level:
stage:
---
- **core library** -> `internal/core/`. It has no config-driven lifecycle, no registry side effect, and no reason to live with a component domain.
- **framework** -> usually `internal/component/`. It provides Ze's wiring substrate rather than a runnable feature: config, plugin, command, cli, doctor, hub, lifecycle, and setup-feature integration.
- **host-service** -> `internal/component/`. It is a daemon or appliance service boundary such as web, ssh, gNMI, MCP, looking-glass, host APIs, or gokrazy support. These packages are not pure core libraries because startup, doctor, listener, or platform registration pins them to composition.
- **domain-library** -> lives with the component domain it serves until that domain is split. In this spec only BNG (`l2tp`, `ppp`, `pppoe`, `pppoeclient`, `subscriber`) and VPN (`ike`, `ipsec`) are clustered. PKI stays top-level because it is shared certificate infrastructure for IPsec and future TLS users. AAA, traffic, firewall, and CoS stay flat unless a later spec proves a clean isolated cluster.
- **engine + a feature depends on it** -> **component** (`internal/component/`). It is a platform other plugins build on. BGP is the archetype: its sub-plugins and other code plug into it.
- **engine + nothing depends on it** -> **edge plugin** (`internal/plugins/`). IS-IS, OSPF, LDP, RSVP-TE are edge protocols: they consume services (iface, the RIB) but nothing consumes them. A *gated* edge engine's blank import in the generated `all_<tag>.go`, a `cmd/ze` dispatch companion, or `cmd/ze/setup_features_*.go` is a registration import, NOT a dependency, so it does not promote the engine to a component.
- The RIB stays **component** because edge protocols install routes through it.
