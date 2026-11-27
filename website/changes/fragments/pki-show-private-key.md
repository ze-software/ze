---
spec: pki-show-private-key
topic: Security
found-by: review
---

## Before

`show pki certificate name <name> bundle pem` ran as a read-only `show`
command. Any caller permitted to run `show` commands retrieved a stored device
certificate's PKCS#8 private key in cleartext, with no confirmation keyword and
no audit record.

## After

`show pki certificate name <name> bundle pem confirm` now requires both an
explicit RBAC `edit` grant (default-deny) and the mandatory `confirm` keyword.
This gate applies across CLI, SSH, REST, gRPC, Web, MCP, wire RPC, and plugin
dispatch. Every allowed export and every refused attempt writes a
`pki-private-key-export` audit entry with zero private-key bytes. CLI session
transcripts redact PEM private-key blocks on output. Certificate-only
`show pki certificate name <name> pem` stays unguarded and unchanged.

## Corrects

none

## Exposure

Deployments with stored PKI device certificates were exposed in five cases:

- RBAC was unconfigured.
- A caller held `read-only` or `admin` (`default-action allow`) profiles.
- A caller used the recovery account.
- A caller used a shared API token.
- An external plugin sent `show` commands.

Deployments that stored only CA certificates or keyless certificates were not
exposed.

## Upstream

none

## Asked by

none
