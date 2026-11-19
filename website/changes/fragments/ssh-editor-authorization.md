---
spec: ssh-editor-authorization
topic: RBAC
found-by: review
---

## Before

On a router with `system authorization` configured, the interactive SSH
configuration editor (`ze cli`, `ze config edit`) never asked the user's
profile before a change. A user whose profile denied the edit section could
enter configuration mode, `set` and `commit`, on the router's own
configuration and on any stored configuration the session named, a hub's
client configurations included. The web editor, the web terminal, the REST
and gRPC configuration sessions and the SSH configuration upload already
checked.

## After

Each configuration-mode change in the SSH editor is authorized against the
edit section of the user's profile with the same permission strings the web
and the API use (`config set`, `config delete`, `config commit`,
`config discard`, `config rollback`, `config rename`, `config add`,
`config save`). A refused line prints `command restricted by access control:`
with the permission, then how to get it, and changes nothing. Browsing
(`show`, `edit`, `up`, `top`) stays open to a read-only user. A session on a
named stored configuration asks the string followed by the name
(`config commit client-edge.conf`), so an entry can single out one file.
A profile entry naming one of these strings no longer logs a warning that it
matches no known command. The sample `operator` profile in the RBAC guide
now allows `config`; its old `configure` and `commit` entries matched
nothing.

## Corrects

none

## Exposure

Every daemon with `system authorization` profiles and SSH enabled: a user
with a profile that denied editing could change and commit the configuration
over the SSH editor. A daemon with no `system authorization` is permissive by
design and is unchanged. The attached console of `ze start --cli` and
`ze config edit -f` or `--backup` authenticate nobody and are unchanged.

## Upstream

none

## Asked by

none
