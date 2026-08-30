# The PKI certificate store

Shared certificate infrastructure: CA certificates and device certificates
loaded from config, validated at load time, and served to any consumer that
needs X.509 material. IPsec is the first consumer; TLS, SSH host certificates
and managed-device authentication can use the same store.

A TLS listener that serves one of these certificates with its full chain is
[`tls-listeners.md`](tls-listeners.md).

<!-- source: internal/component/pki/store.go -- Load, Validate, GetCA, GetCertificate, CertCN, CAPool, IntermediatePool, ExportPEM, CleanupPEM -->
<!-- source: internal/component/pki/config.go -- ParseConfig, parseCACert, parseDeviceCert, parsePrivateKey, verifyKeyMatchesCert -->
<!-- source: internal/component/pki/types.go -- CACertEntry, CertificateEntry, PKIConfig, CertSummary -->
<!-- source: internal/component/pki/show.go -- handleShowPKICertificates, handleShowPKICertificate, handleShowPKICertificatePEM, handleShowPKICertificateBundlePEM, handleShowPKICertificateFingerprint -->
<!-- source: internal/component/pki/yang/ze-pki-conf.yang -- the pki config module -->

## Decisions

**Each output form of `show pki certificate name` is its own command.** The
detail, `pem`, `bundle pem` and `fingerprint` forms take structurally different
tails, so one command reading them from its arguments states its grammar in a
handler switch where no operator and no catalog can read it. Four sibling
containers, each with its own `ze:command` and its own handler, put the grammar
in the model instead.

**Certificate values are base64-encoded DER, not PEM.** The PEM header adds
nothing inside a YANG leaf, and raw DER is what the config convention Ze follows
already stores.

**Private key detection tries PKCS8, then SEC1, then PKCS1.** Real keys arrive
in all three encodings depending on the tool that produced them, so requiring
one format rejects valid input.

**The store is an atomic pointer swap, not a mutex.** Readers are the show
commands and the IPsec consumer; the writer is a config reload. Readers never
block the writer.

**The chain and the expiry are validated at load, not lazily.** Failing at
config load produces a better error and stops the daemon from running with a
broken certificate.

**PEM export goes to a temporary directory.** A consumer that expects PEM file
paths rather than inline data gets them from `ExportPEM`, and `CleanupPEM`
removes them.

**PKI is its own component, not part of ipsec.** It is shared infrastructure.
Web TLS, SSH host certificates and managed-device authentication are all
plausible consumers.

## Consequences worth knowing

- The IKE engine calls `Load` after the config parse, then uses `CAPool` and
  `GetCertificate` for X.509 authentication.
- The YANG module registers through a schema `init()` and a blank import. No
  wiring code is needed in the program entry point.
- The show commands are RPC handlers inside the pki package, not in the shared
  show command package. The component stays self-contained.
- The private key leaf is marked sensitive, so the config parser decodes the
  reversible encoding before the PKI parser sees it. The parser receives
  plaintext base64.
- The health check reports degraded when a certificate expires within 30 days
  and down when one has expired. Expiry warnings are raised on the report bus
  after each load and clear when the certificate is renewed.

<!-- source: internal/component/pki/health.go -- the pki health check and expiry warnings -->

## Traps this code exists to avoid

**A YANG list is built with the list-entry API, not with nested containers.** A
test that builds list entries with the container accessor produces a tree whose
list getter returns nil. Always add list entries as list entries.

**Key size needs an explicit type switch.** RSA, ECDSA and Ed25519 public keys
have no common size method. ECDSA reads the curve parameter bit size.

**An Ed25519 private key in Go is a 64-byte seed.** The public key is the second
half of the slice. Do not take the public key through the interface method and
type-assert it for comparison; use the slice directly.
