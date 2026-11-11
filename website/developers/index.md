# Developers

The public plugin SDK lets you extend Ze. These pages explain how plugins communicate with the daemon and how to contribute a change to the project.

## Build extensions

- [Plugin development](plugins/): start with the SDK and plugin startup sequence.
- [Protocol](plugins/protocol/): connect a plugin to the daemon and manage its startup and shutdown.
- [Configuration schemas](plugins/schema/): define plugin configuration with YANG.
- [Handlers](plugins/handlers/): receive events and implement plugin behavior.
- [Commands](plugins/commands/): add commands to CLI, web, API, gRPC, and MCP.
- [Metrics](plugins/metrics/): publish operational measurements.
- [Plugin testing](plugins/testing/): test plugin registration and behavior while Ze runs.

## Contribute to Ze

- [Developer setup](../contribute/developer-setup/): build, test, and debug the source tree.
- [Contributor guide](../contribute/guide/): prepare and submit a change.
- [Contributor testing](../contribute/testing/): choose tests that exercise the behavior you changed.
- [RFC implementation guide](../contribute/rfc-implementation-guide/): implement and verify a standard.
- [Documentation testing](../contribute/documentation-testing/): keep examples tied to executable evidence.
- [Style guide](../style-guide/): follow the website's visual and content conventions.
