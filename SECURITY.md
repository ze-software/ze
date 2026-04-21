# Security Policy

## Supported Versions

The project is not yet released. Security fixes will be provided for the latest released version. We intend to provide a smooth upgrade path, allowing users to update to the latest version if/when required.

## Scope

Ze is a Network OS handling BGP sessions, SSH access, and configuration management. The following areas are particularly security-sensitive:

- BGP message parsing and wire protocol handling
- SSH/CLI authentication and authorization
- Plugin process isolation and IPC boundaries
- Configuration injection or unauthorized modification
- Web UI and MCP server access control

Regular bugs (crashes, incorrect route selection, CLI rendering) should be filed as normal issues unless they can be triggered by an unauthenticated remote peer or escalated into unauthorized access.

## Reporting a Vulnerability

To report vulnerabilities, contact Thomas Mangin by email: ze@mangin.com

**What to expect:**

- Acknowledgment within 7 days.
- An initial assessment and next steps within 14 days.
- Coordinated disclosure once a fix is available. We will credit reporters unless they prefer anonymity.

Please include: affected version or commit, steps to reproduce, and potential impact.
