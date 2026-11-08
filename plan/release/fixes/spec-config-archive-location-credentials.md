# Spec: config archive credentials and plain-HTTP upload

| Field | Value |
|-------|-------|
| Status | skeleton |
| Scope | config |
| Priority | P0 |
| Release reason | Security: the archive destination's credentials are stored and displayed in clear, and the whole config, with its reversible `$9$` secrets, is uploaded over plain HTTP to any remote host the operator names. |
| RFC gaps | - |
| Depends | - |
| Phase | - |
| Handoff | - |
| Updated | 2026-11-08 |

Recovery after compaction: `.claude/rules/post-compaction.md`.

## Task

`system archive <name> location` is a plain string. The only way to give the
uploader credentials is userinfo in the URL (`https://user:pass@host/path`):
Go's HTTP client sends it as Basic auth. The leaf carries no `ze:sensitive`, so
the password is stored in the config file, printed by `show config`, carried
in every archived copy, and published on every surface that shows config.
Separately, `ValidateArchiveLocation` accepts `http://` to a remote host, while
`ValidateFetchURL` refuses plain HTTP to anything but loopback. An `http://`
archive sends the whole config, its `$9$` secrets (reversible by
`ze secret decode`) and the Basic auth header, in clear, on every trigger.

Goal: archive credentials live in their own leaves, the password one
`ze:sensitive`; a `location` carrying userinfo is refused at verify with the
fix; and an `http://` location to a non-loopback host is refused at verify, the
same rule `ValidateFetchURL` enforces for reads.

VyOS fixed the same class in T6304 (`575f265da` in
`~/Code/github.com/vyos/vyos-1x`): a structured archive location with separate
credentials.

## Required Reading

### Architecture Docs
- [ ] `docs/guide/config-archive.md` - the location forms an operator writes, and the redaction it promises for refusals
- [ ] `docs/architecture/config/yang-config-design.md` - `ze:sensitive`, how a sensitive leaf is printed and decoded
- [ ] `ai/doors/config-yang.md` - validators, `ze:validate`, refusal at verify

**Key insights:**
- `SensitiveKeys` collects sensitive leaves by NAME, so marking `location` itself sensitive would mask every leaf named `location` in the schema; separate credential leaves avoid that.
- `ValidateArchiveLocation` is the one check behind both the commit-time validator and the manual `ze config archive` command.

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/config/system/yang/ze-system-conf.yang` - `list archive`,
  `leaf location`: `type string`, `ze:validate "archive-location"`, no
  `ze:sensitive`; no username or password leaf exists.
- [ ] `internal/component/config/validators_archive.go` - `ValidateArchiveLocation`
  accepts any `file`, `http` or `https` URL, with no host check and no userinfo
  check; its refusals redact the URL.
- [ ] `internal/component/config/validators.go` - `ValidateFetchURL` accepts
  `https` with a host, accepts `http` only to `loopbackFetchHosts`, and refuses
  every other form.
- [ ] `internal/component/config/archive/archive.go` - `ToHTTP` POSTs the config
  to the location URL with `http.DefaultClient`, so userinfo becomes Basic
  auth; `archiveToLocation` dispatches `http` and `https` alike. Upload errors
  redact the URL.
- [ ] `internal/component/config/schema.go` - `SensitiveKeys` keys sensitive
  leaves by leaf name.
- `docs/guide/config-archive.md` documents `http://host/path` as a valid form
  and says a refusal "never echoes the password of a URL with credentials in
  it", so credentials in the URL are the documented route.

**Behavior to preserve:**
- `file://` locations, and `https://` locations without userinfo, keep working.
- `http://` to a loopback host keeps working, as for fetch URLs.
- Refusals and upload errors keep redacting.

**Behavior to change:**
- Credentials move to separate leaves, the password `ze:sensitive`.
- A `location` with userinfo is refused at verify.
- An `http://` location to a non-loopback host is refused at verify.
- The repository's configs, `.ci` bodies and docs that use either form are converted in the same work.

## Existing Mechanisms (MANDATORY)

| Mechanism | Where (file + symbol, or page + section) | What it does today | This spec (extends, replaces, or does not apply, with the reason) |
|-----------|------------------------------------------|--------------------|-------------------------------------------------------------------|
| Fetch URL policy | `internal/component/config/validators.go` `ValidateFetchURL`, `loopbackFetchHosts` | Refuses plain HTTP to a non-loopback host for reads | Extends: the archive validator applies the same host rule, from the same loopback list, so the two cannot drift |
| Archive location check | `internal/component/config/validators_archive.go` `ValidateArchiveLocation` | Scheme check for commit and the manual command | Extends: adds the userinfo refusal and the plain-HTTP host rule |
| Sensitive leaves | `ze:sensitive`, `internal/component/config/schema.go` `SensitiveKeys`, `secret_text.go` `EncodeSecrets` | Stores and prints a value as `$9$`, masks it on every surface | Extends: the new password leaf uses it |
| URL redaction | `internal/core/redact`, `archive.go` `RedactURL` | Hides userinfo in messages | Does not apply as the fix: redaction covers messages only, not storage or display |
| HTTP uploader | `internal/component/config/archive/archive.go` `ToHTTP` | POSTs to the location | Extends: sets Basic auth from the new leaves |

## Data Flow (MANDATORY - see `ai/rules/architecture.md`)

### Entry Point
- Operator config: `system archive <name> location`, and the new credential leaves.

### Transformation Path
1. The `archive-location` validator runs at verify and refuses userinfo and plain HTTP to a remote host.
2. The archive config loader reads location and credentials into `ArchiveConfig`.
3. `ToHTTP` POSTs the config with Basic auth from the credential leaves.

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Ze to archive server | HTTPS POST, Basic auth header | No |
| Config to every display surface | `ze:sensitive` printing | No |

### Integration Points
- `ValidateArchiveLocation`, `ToHTTP`, the archive config loader in `archive.go`.

## Risks & Assumptions

### Assumptions
| Assumption | Validation method | Status |
|------------|-------------------|--------|
| No archive server in the repository's tests needs plain HTTP to a remote host | Grep `test/` and the docs for `archive` `http://` locations | unvalidated |

## Wiring Test (MANDATORY -- NOT deferrable)

| Entry Point | -> | Feature Code | Test |
|-------------|----|--------------|------|
| `system archive` with credential leaves | -> | `ToHTTP` Basic auth | `.ci` against a local HTTPS receiver that checks the header (AC-1) |
| `location` with userinfo | -> | `ValidateArchiveLocation` | `.ci` commit refusal (AC-3) |
| `http://` remote location | -> | `ValidateArchiveLocation` | `.ci` commit refusal (AC-4) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior | Permitted Survivors and Why |
|-------|-------------------|-------------------|-----------------------------|
| AC-1 | An archive entry with an `https://` location and the new username and password leaves, then a commit trigger | The receiver gets the config with the matching Basic auth header. Test: a `.ci` with a local receiver that asserts the header and body | None |
| AC-2 | `show config`, `ze config dump`, the web config view and gNMI Get on that entry | The password prints only in its `$9$` form; the location carries no credential. Test: a `.ci` over `show config` and a config unit test over the printers | None |
| AC-3 | A `location` with userinfo (`https://u:p@host/`) | The commit is refused at verify, the message redacts the password and names the credential leaves as the fix. Test: a validator unit test and a `.ci` refusal | None |
| AC-4 | A `location` of `http://` to a non-loopback host | The commit and the manual `ze config archive` command refuse it, naming https and the loopback hosts. Test: a validator unit test and a `.ci` refusal | `http://` to a loopback host is accepted, as for fetch URLs: the data never leaves the box |
| AC-5 | The repository's configs, `.ci` bodies and docs | None uses userinfo or a remote `http://` archive location. Test: the converted `.ci` files run green under `./le test functional` for their suite | None |

## 🧪 TDD Test Plan

### Unit Tests
| Test | File | Validates |
|------|------|-----------|
| userinfo refused, password redacted | `internal/component/config/validators_archive_test.go` | AC-3 |
| remote http refused, loopback http accepted | `internal/component/config/validators_archive_test.go` | AC-4 |
| password leaf printed as `$9$` | `internal/component/config/` | AC-2 |

### Functional Tests
| Test | Location | Validates |
|------|----------|-----------|
| archive upload with Basic auth | `test/` archive `.ci` | AC-1 |
| `show config` masks the password | `test/` archive `.ci` | AC-2 |
| refusals at commit | `test/` archive `.ci` | AC-3, AC-4 |

## Files to Modify

- `internal/component/config/system/yang/ze-system-conf.yang` - username and sensitive password leaves under `list archive`.
- `internal/component/config/validators_archive.go` - userinfo and plain-HTTP refusals.
- `internal/component/config/archive/archive.go` - Basic auth from the leaves.
- `docs/guide/config-archive.md` - the credential leaves and the HTTPS rule.

### Documentation Update Checklist (BLOCKING)

| # | Question | Applies? | File to update |
|---|----------|----------|---------------|
| 2 | Config syntax changed? | Yes | `docs/guide/config-archive.md`, `docs/guide/configuration.md` |
| 6 | Has a user guide page? | Yes | `docs/guide/config-archive.md` |
| 17 | Existing docs show config examples for this area? | Yes | every `system archive` example |

## Implementation Steps

1. Write the failing tests for AC-1 to AC-5.
2. Add the credential leaves and the uploader's Basic auth.
3. Add the two refusals to `ValidateArchiveLocation`, reusing the fetch loopback list.
4. Convert the repository's users, update the pages, run the tests.

## Owned journal rows

| Class file | Date | Surface (short) |
|------------|------|-----------------|
| `secret-echoed-to-the-client.md` | 2026-11-08 | archive `location` userinfo stored and shown in clear |
| `guard-added-to-one-half-of-a-pair.md` | 2026-11-08 | `ValidateArchiveLocation` accepts remote `http://` that `ValidateFetchURL` refuses |

## Checklist

- [ ] Tests written
- [ ] Tests FAIL before the fix
- [ ] Tests PASS after the fix
- [ ] `./le test unit config`
- [ ] `./le test functional` for the archive suite
