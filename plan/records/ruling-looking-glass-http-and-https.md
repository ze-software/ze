# Ruling: The looking glass may serve HTTP, and serves HTTP and HTTPS at the same time

- **Date:** 2026-11-23
- **Owner:** Thomas Mangin
- **Context:** The looking glass served TLS only, with one block-wide `tls` leaf on `environment looking-glass` and the `ze.looking-glass.tls` environment variable choosing the protocol of every listener at once.
## Decision

Thomas's words, as voice-transcribed:

> "We should allow the looking glass to be HTTP. The information is public."
> "We should be able to serve on both HTTP and HTTPS, not either."

He was then asked two questions and answered "1a 2a":

- **Q1: "Where should the HTTP-or-HTTPS choice live?"** (a) On each server entry, for example `environment { looking-glass { enabled true; server public { ip 0.0.0.0; port 80; tls false; } server secure { ip 0.0.0.0; port 8443; } } }`, where `tls` defaults to true. The block-wide looking-glass `tls` leaf is removed, and every config in the repository is converted in the same work.
- **Q2: "What happens to `ze.looking-glass.tls`?"** (a) Removed: no environment variable switches the looking glass's protocol. A deployment that sets it must write `tls` in the config.

Stated with the questions and not contested, so binding: today's no-store rule applies per entry. On a daemon with no `database/` store, an entry that left `tls` at its default serves HTTP with a warning, and an entry whose operator wrote `tls true` is refused, at start and at reload alike. A reload does what a restart with the same config does.

## What this requires

1. Each `server` entry of `environment looking-glass` carries its own `tls` leaf, default true. One looking glass serves HTTP on some entries and HTTPS on others at the same time.
2. The block-wide `tls` leaf of `environment looking-glass` is removed, and every config in the repository is converted to the per-entry leaf in the same work.
3. The `ze.looking-glass.tls` environment variable is removed, and nothing reads it as a fallback.
4. With no named certificate and no `database/` store, an entry that left `tls` unset serves plain HTTP with a warning that names the entry and the fix, and an entry that says `tls true` is refused at start and at reload.
5. A reload gives each listener the protocol a restart with the same config would give it.
