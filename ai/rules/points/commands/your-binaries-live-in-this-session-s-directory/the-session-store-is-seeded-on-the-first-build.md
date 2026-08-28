---
kind: note
level:
stage:
---
The session-local `etc/ze` is seeded once by
`./le session seed-store binary <session-bin>/ze`. `SeedStore` in
`internal/le/session/seed.go` validates that the binary belongs to the current
session directory. Credentials are generated per session, with user `admin`
and a random password at `<session-dir>/etc/ze/.dev-password`, mode 0600.
A later seed preserves the existing store and does not rotate the credentials.
