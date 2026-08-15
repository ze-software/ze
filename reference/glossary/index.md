# Glossary

Terms used in Ze documentation and operator output.

## BGP and routing

**Adj-RIB-In**  
Routes accepted from a peer and retained before or alongside local best-path selection. Ze does not keep every import-rejected route unless a component explicitly records it.

**Adj-RIB-Out**  
Routes selected and prepared for advertisement to one peer.

**AFI/SAFI**  
Address Family Identifier and Subsequent Address Family Identifier. Together they identify a BGP family such as IPv4 unicast, EVPN, or FlowSpec.

**AS path**  
The sequence of autonomous systems a BGP route has traversed. Ze uses it for loop detection, policy, best-path selection, and topology display.

**BGP Role**  
An RFC 9234 relationship such as provider, customer, peer, route server, or route-server client. Roles help detect route leaks and control OTC processing.

**End-of-RIB**  
A BGP marker indicating that the initial or refreshed update stream for a family is complete.

**FIB**  
Forwarding Information Base. The routes programmed into a dataplane such as the Linux kernel, VPP, or P4.

**Loc-RIB**  
The local set of selected routes after protocol and policy processing.

**NLRI**  
Network Layer Reachability Information. The family-specific route payload carried by BGP.

**RIB**  
Routing Information Base. A route store used for candidate, accepted, selected, or advertised routing state, depending on context.

**RPKI**  
Resource Public Key Infrastructure. Ze uses validated ROA data received over RTR to determine whether a route origin is Valid, Invalid, or NotFound.

**RTR**  
RPKI-to-Router protocol used between a validating cache and a router.

## Resilience and policy

**ADD-PATH**  
A BGP capability that allows more than one path for the same prefix to cross a session. Send and receive modes are negotiated per family.

**Graceful Restart**  
RFC 4724 behaviour that retains stale forwarding state while a BGP speaker restarts and refreshes its routes.

**Long-Lived Graceful Restart**  
RFC 9494 extension that retains and deprioritises stale routes beyond the ordinary Graceful Restart window.

**Import filter**  
An ordered policy chain applied to a route received from a peer before it enters eligible local routing state.

**Export filter**  
An ordered policy chain applied for one destination peer before advertisement.

**OTC**  
Only-to-Customer, the RFC 9234 path attribute used with BGP Roles to detect and prevent route leaks.

**Route reflection**  
RFC 4456 distribution of iBGP routes through reflectors and clients, avoiding a full iBGP mesh.

**Route server**  
A BGP speaker that distributes eligible routes between participants while preserving the participant next hop and applying per-member policy.

## Ze configuration and operation

**Active configuration**  
The configuration revision currently applied to the running system.

**Candidate configuration**  
A proposed configuration revision being edited and validated before commit.

**Commit**  
The transaction that verifies, applies, and activates a candidate configuration. In route injection, a named commit window groups route operations atomically.

**Component**  
A Ze subsystem that owns runtime behaviour, configuration, commands, health, metrics, or plugins.

**Doctor**  
Offline readiness checks run with `ze doctor`. Findings have structured diagnostic codes that can be inspected with `ze explain`.

**Handler**  
A callback registered by a component or plugin for configuration, commands, events, codecs, validation, or lifecycle stages.

**Plugin**  
A registered Ze extension. Plugins may run in process or as external programs and can own configuration, commands, events, filters, families, services, or dataplane integration.

**Plugin hub**  
The authenticated transport and dispatch layer used by external plugins and managed components. It is separate from Fleet Management even when configuration shares a surrounding plugin block.

**ZeFS**  
Ze's blob storage format for configuration, credentials, revisions, and other registered data.

## Operator surfaces

**CLI**  
The interactive or one-shot command interface reached over Ze's SSH server.

**gNMI**  
The OpenConfig gRPC Network Management Interface used for YANG-oriented Capabilities, Get, Set, and Subscribe operations.

**Looking Glass**  
The public read-only BGP peer, route, search, and AS-path view. It runs separately from the authenticated web workbench.

**MCP**  
Model Context Protocol. Ze exposes registered commands and structured resources to authorised AI clients through its MCP server.

**Web workbench**  
The authenticated browser interface for configuration, commands, operational tables, and live state.

## Testing

**Functional transcript**  
A `.ci` scenario that drives real processes, commands, files, HTTP requests, peers, and expected outputs.

**Interop test**  
A scenario that runs Ze against another real implementation such as FRR, BIRD, GoBGP, strongSwan, xl2tpd, or accel-ppp.

**Property**  
An invariant checked across a scenario, such as convergence, route isolation, or state consistency.

**Shrink**  
Reduction of a recorded chaos event sequence to the smallest sequence that still reproduces a failure.
