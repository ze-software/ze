# Spec: pki-full-chain

| Field | Value |
|-------|-------|
| Status | skeleton |
| Depends | - |
| Phase | - |
| Updated | 2026-07-03 |

## Post-Compaction Recovery

**Re-read these after context compaction:**
1. This spec file (you're reading it now)
2. `.claude/rules/planning.md` - workflow rules
3. `internal/component/web/service_web.go` - TLS listener setup
4. `internal/component/pki/` - certificate storage and chain handling

## Task

Ze's web/API server always uses self-signed certificates via `selfcert.LoadOrGenerateCert`
(`service_web.go:268`). There is no path for operators to use PKI-stored certificates
with their intermediate chain for the web/API HTTPS endpoint. The PKI component correctly
stores intermediates (`CertificateEntry.Intermediate`, `types.go:28`) and includes them
in PEM output (`show.go:171-172`, `show.go:197-198`), but the web TLS listener never
consumes PKI-stored certs.

This is a two-part gap:
1. Web server cannot use operator-provided certificates from PKI store
2. When #1 is added, the full chain (leaf + intermediates) must be served

## Required Reading

### Architecture Docs
- [ ] `internal/component/web/` - web server implementation
- [ ] `internal/component/pki/` - PKI certificate management
- [ ] `ai/rules/config-surface.md` - YANG vs env var decision

### RFC Summaries (MUST for protocol work)
N/A - standard TLS behavior, not protocol extension.

**Key insights:**
- `service_web.go:268`: `selfcert.LoadOrGenerateCert` is the only TLS path
- `selfcert.go:176`: `NewTLSConfig` builds `tls.Config` with just leaf cert from `tls.X509KeyPair`
- `pki/types.go:28`: `CertificateEntry` has `Intermediate` field (chain is stored)
- `pki/show.go:171-172,197-198`: PEM output includes intermediates
- No wiring exists between PKI store and web server TLS config

## Current Behavior (MANDATORY)

**Source files read:**
- [ ] `internal/component/web/service_web.go` - web server TLS setup
- [ ] `internal/component/web/selfcert.go` - self-signed cert generation
- [ ] `internal/component/pki/types.go` - certificate storage types
- [ ] `internal/component/pki/show.go` - certificate display with chain

**Behavior to preserve:**
- Self-signed cert generation as fallback when no PKI cert configured
- PKI intermediate storage and PEM display
- Web server functionality

**Behavior to change:**
- Add YANG config option to specify PKI certificate name for web server
- When configured, load cert + intermediates from PKI store
- Build `tls.Config` with full certificate chain
- Fall back to self-signed when not configured

## Data Flow (MANDATORY - see `ai/rules/data-flow-tracing.md`)

### Entry Point
- YANG config: web server certificate reference to PKI store

### Transformation Path
1. Config specifies PKI certificate name for web server
2. Web server queries PKI store for certificate entry
3. `CertificateEntry` provides leaf cert + `Intermediate` chain
4. `tls.Config.Certificates` built with full chain
5. HTTPS listener serves leaf + intermediates in TLS handshake

### Boundaries Crossed
| Boundary | How | Verified |
|----------|-----|----------|
| Config -> web server | YANG tree resolution | [ ] |
| Web server -> PKI store | PKI component query | [ ] |
| Web server -> TLS listener | `tls.Config` with chain | [ ] |

### Integration Points
- PKI component certificate lookup
- Web server TLS configuration
- YANG config for certificate reference

### Architectural Verification
- [ ] No bypassed layers (data flows through intended path)
- [ ] No unintended coupling (components remain isolated)
- [ ] No duplicated functionality (extends existing, doesn't recreate)
- [ ] Zero-copy preserved where applicable (uses refs, not copies)
- [ ] Registration over hardcoding

## Risks & Assumptions

### Assumptions
| ID | Assumption | Basis (file/doc/user statement) | If wrong | Validated by | Status |
|----|-----------|--------------------------------|----------|--------------|--------|
| A-1 | PKI store is available before web server starts | Component startup order | Would need lazy cert loading | Check component dependency graph | unvalidated |
| A-2 | Certificate rotation needs graceful reload | Standard practice | Would need server restart | Check if web server supports TLS cert reload | unvalidated |

### Risks
| ID | Risk | Early signal | Mitigation / fallback |
|----|------|--------------|----------------------|
| R-1 | PKI cert expires without auto-renewal | HTTPS stops working | Fall back to self-signed on cert load failure |
| R-2 | Circular dependency: PKI needs web, web needs PKI | Startup deadlock | Lazy cert loading after both components are up |

## Wiring Test (MANDATORY)
| Entry Point | -> | Feature Code | Test |
|-------------|---|--------------|------|
| YANG config web cert reference | -> | TLS listener uses PKI cert with chain | (fill during design) |
| No cert configured | -> | Self-signed fallback (unchanged) | (fill during design) |

## Acceptance Criteria

| AC ID | Input / Condition | Expected Behavior |
|-------|-------------------|-------------------|
| AC-1 | PKI certificate name configured for web server | HTTPS serves leaf + intermediate chain |
| AC-2 | No PKI certificate configured | Self-signed cert used (backward compatible) |
| AC-3 | Referenced PKI certificate does not exist | Config validation error |
| AC-4 | `openssl s_client` against configured web server | Full chain visible in handshake |

## End-to-End User Stories (MANDATORY for new features)
| # | User does | Path through system | Test proving it works |
|---|-----------|--------------------|-----------------------|
| 1 | Configures web server to use PKI cert | YANG -> web server loads cert + chain from PKI -> HTTPS serves full chain | (fill during design) |
| 2 | Does not configure web cert | Web server auto-generates self-signed cert (existing behavior) | (fill during design) |

## Known Limitations
- No current bug: self-signed certs have no chain to serve
- Gap is architectural: no path exists to use operator certs even if desired
- Certificate rotation / reload not covered in this skeleton
