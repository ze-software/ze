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

1. **Remote SSH client does not verify host keys.** The `ze` remote CLI
   (`ze ssh <target>`) uses `ssh.InsecureIgnoreHostKey()`. Connections to
   remote ze instances are vulnerable to man-in-the-middle attacks.
   Mitigation: use only over trusted networks until host-key verification
   is implemented.

2. **Managed TLS transport does not verify server certificates.** The managed
   client uses `tls.Config{InsecureSkipVerify: true}` when connecting to the
   hub. A network attacker can impersonate the hub and inject configuration.
   Mitigation: use only over trusted networks, or deploy a reverse proxy
   with proper certificate validation.

3. **Non-SSH command surfaces drop caller identity.** Web admin, web L2TP,
   and MCP dispatcher paths do not propagate the authenticated username and
   remote address to the authorization layer. Authenticated but restricted
   operators may be able to execute commands outside their profile on these
   surfaces. Mitigation: restrict access to these surfaces to trusted
   operators until authz context propagation is fixed.

4. **SSH lifecycle commands bypass authorization.** Any authenticated SSH
   user can invoke `stop`, `restart`, or `reboot` before the normal
   authorization check runs. Mitigation: restrict SSH access to trusted
   operators.
