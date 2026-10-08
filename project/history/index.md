# Project History

Ze grew from the operational lessons of ExaBGP. ExaBGP made BGP programmable by treating routes and protocol events as input and output for ordinary processes. That model proved that a routing tool did not need to imitate a traditional router CLI to be useful.

## From ExaBGP to Ze

ExaBGP remained deliberately focused on BGP and process integration. Over time, operators needed the surrounding system as well: a validated configuration model, route storage, policy, interface and FIB management, health, telemetry, secure operator access, and a consistent way to expose every command to automation.

Ze keeps the programmable event model and builds those surrounding responsibilities into one system. Its native BGP engine, YANG configuration, plugin registry, CLI, web interface, APIs, MCP server, Looking Glass, telemetry, and optional dataplane integrations share the same runtime state and command dispatcher.

## The first development period

The first tracked development week began in December 2025 with the BGP wire engine, capability negotiation, path attributes, address families, RIB work, session state, and ExaBGP interop. The project then expanded through configuration and plugin architecture, route server and reflector paths, policy, operator surfaces, interfaces, FIB backends, routing protocols, access services, security, diagnostics, packaging, and interoperability.

The [weekly changes](https://ze-software.net/project/changes/) retain the detailed chronological record. [Milestones](https://ze-software.net/project/milestones/) selects the larger product changes, while the [roadmap](https://ze-software.net/project/roadmap/) describes what remains before a stable release.

## What remained familiar

The ExaBGP migration path preserves two useful ideas:

- Existing ExaBGP configuration can be converted into Ze's configuration model.
- Existing process scripts can continue through the compatibility bridge before being ported to the native plugin SDK.

This allows a routing-engine migration and an application rewrite to happen separately. The [ExaBGP migration example](https://ze-software.net/use-cases/exabgp-migration/) documents that path.

## Why the architecture changed

Ze needs to support both a small programmable speaker and a more complete Linux network system. Plugins provide that boundary. BGP, policy, RIB storage, interfaces, FIB backends, routing protocols, services, and operator features register what they own. A deployment loads only the components selected by configuration and build tags.

The shared registration model also keeps operator surfaces aligned. A command registered by a component can be discovered and dispatched through the CLI, web interface, REST, gRPC, and MCP without each surface building a separate implementation.

## Current state

Ze remains pre-release. Capability and maturity claims are generated from current source and evidence where possible. Use the [feature inventory](https://ze-software.net/features/), [RFC status](../../reference/rfcs/index.md), [quality evidence](https://ze-software.net/quality/), and [roadmap](https://ze-software.net/project/roadmap/) for current decisions rather than treating this history as a readiness statement.
