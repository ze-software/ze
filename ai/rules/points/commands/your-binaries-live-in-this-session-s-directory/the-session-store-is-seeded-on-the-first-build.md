---
kind: note
level:
stage:
---
That session-local `etc/ze` is SEEDED, once, by the first ze_core binary this
session builds (`scripts/dev/session-seed-store.sh`, called from the `ze`,
`ze-appliance-build` and `ze-stripped-build` recipes -- the three that link
`internal/plugins/init` and the silent `NewBlob` path). An
unseeded store is not red: `NewBlob`
(`internal/component/config/storage/blob.go`) creates the blob and returns a nil
error when it is absent, so `ze` would start with no users and a fresh SSH host
key rather than fail. The credentials are generated per session -- user `admin`,
and a random password at `<session-dir>/etc/ze/.dev-password`, mode 0600 under a
gitignored root -- so nothing is tracked and two sessions never share one. A
second `make ze` reseeds nothing and rotates nothing.
