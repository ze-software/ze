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

## Known Limitations (pre-release)

The following trust-model gaps are known and tracked for remediation before
any production deployment:

1. **~~Remote SSH client does not verify host keys.~~** (Fixed) Remote hosts
   now require trust unless `ze.ssh.insecure=true` is explicitly set.

2. **~~Managed TLS transport does not verify server certificates.~~** (Fixed)
   TLS verification is now the default; insecure mode is opt-in via config.

3. **~~Non-SSH command surfaces drop caller identity.~~** (Fixed) Web admin,
   web L2TP, and MCP dispatcher paths now propagate the authenticated
   username and remote address to the authorization layer.

4. **~~SSH lifecycle commands bypass authorization.~~** (Fixed) All SSH
   command paths (including lifecycle and plugin protocol) now go through
   authorization checks.

5. **Authorization fails open for empty/unassigned users when no profiles
   are configured.** When no user assignments are configured, all access is
   allowed. This is intentional for development but should be documented for
   deployment. When assignments are configured, empty/unassigned users are
   now denied (fail closed).

6. **API and gRPC servers refuse non-loopback listeners without authentication.**
   Remote API listeners now require a token or user authentication to be
   configured. Loopback-only listeners are allowed without auth for local
   development.
