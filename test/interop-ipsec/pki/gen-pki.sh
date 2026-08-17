#!/bin/sh
# Generate test PKI for IPsec EAP interop scenarios.
# Output: ca.pem, server.pem, server-key.pem, client.pem, client-key.pem
# All certs are self-signed test-only with 10-year validity.

set -eu

DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$DIR"

if [ -f ca.pem ] && [ -f server.pem ] && [ -f client.pem ]; then
    exit 0
fi

# CA
openssl ecparam -genkey -name prime256v1 -noout -out ca-key.pem 2>/dev/null
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
