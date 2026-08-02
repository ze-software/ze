#!/bin/sh
# Scenario-local PKI for 06-eap-tls13. Identical to the shared
# test/ipsec-interop/pki/gen-pki.sh in every respect but ONE: the CA key is RSA,
# not prime256v1.
#
# That single difference is load-bearing, and it is a property of the PEER, not
# of ze. strongSwan builds the TLS 1.3 CertificateRequest's
# certificate_authorities extension in write_certificate_authorities()
# (src/libtls/tls_server.c), which enumerates trust anchors as:
#
#     lib->credmgr->create_cert_enumerator(lib->credmgr, CERT_X509, KEY_RSA,
#                                          NULL, TRUE)
#
# KEY_RSA is a hardcoded literal. certificate_matches()
# (src/libstrongswan/credentials/certificates/certificate.c) rejects any
# certificate whose public key type is not the requested one, so an ECDSA CA is
# never enumerated however it is configured. With the shared EC CA the list came
# out empty and charon still emitted the extension, as `002f 0002 0000` -- which
# RFC 8446 Section 4.2.4 forbids, because it declares
# `DistinguishedName authorities<3..2^16-1>`. Go's crypto/tls rejects that and
# ze reports "local error: tls: error decoding message".
#
# With an RSA CA the enumerator yields it, charon logs "sending TLS cert request
# for 'CN=ze-interop-ca'", the extension carries a real DistinguishedName, and
# the handshake completes with charon's SHIPPED default
# (send_certreq_authorities = yes).
#
# The leaf keys stay prime256v1 so this scenario keeps exercising the same
# signature algorithms as the rest of the lab. Only the anchor changed.

set -eu

DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$DIR"

if [ -f ca.pem ] && [ -f server.pem ] && [ -f client.pem ]; then
    exit 0
fi

# CA -- RSA, for the reason documented above.
openssl genrsa -out ca-key.pem 2048 2>/dev/null
openssl req -new -x509 -key ca-key.pem -out ca.pem -days 3650 \
    -subj "/CN=ze-interop-ca" -batch 2>/dev/null

# Server cert (for strongSwan)
openssl ecparam -genkey -name prime256v1 -noout -out server-key.pem 2>/dev/null
openssl req -new -key server-key.pem -out server.csr \
    -subj "/CN=172.28.0.3" -batch 2>/dev/null
printf "subjectAltName=IP:172.28.0.3\n" > server-ext.cnf
openssl x509 -req -in server.csr -CA ca.pem -CAkey ca-key.pem \
    -CAcreateserial -out server.pem -days 3650 \
    -extfile server-ext.cnf 2>/dev/null
rm -f server.csr server-ext.cnf

# Client cert (for Ze EAP-TLS)
#
# The subjectAltName is load-bearing, exactly as it is on the server cert above.
# Ze asserts the EAP identity "ze-test-client", and strongSwan binds the TLS peer
# certificate to that identity before it accepts the method. It parses a bare
# string identity as an FQDN, and an FQDN never matches a certificate whose only
# identity is the DN "CN=ze-test-client". Without this SAN strongSwan logs
# "no trusted certificate found for 'ze-test-client' to verify TLS peer" and
# sends a fatal TLS alert, even though the chain itself is valid and trusted.
openssl ecparam -genkey -name prime256v1 -noout -out client-key.pem 2>/dev/null
openssl req -new -key client-key.pem -out client.csr \
    -subj "/CN=ze-test-client" -batch 2>/dev/null
printf "subjectAltName=DNS:ze-test-client\nextendedKeyUsage=clientAuth\n" > client-ext.cnf
openssl x509 -req -in client.csr -CA ca.pem -CAkey ca-key.pem \
    -CAcreateserial -out client.pem -days 3650 \
    -extfile client-ext.cnf 2>/dev/null
rm -f client.csr client-ext.cnf ca.srl
