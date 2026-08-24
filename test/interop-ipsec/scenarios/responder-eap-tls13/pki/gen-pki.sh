#!/bin/sh
# Scenario-local PKI for responder-eap-tls13, where ZE is the EAP-TLS SERVER
# and strongSwan is the EAP-TLS CLIENT. That role swap is why this scenario
# cannot share the lab PKI, and it drives every name below.
#
# lab.py mounts exactly three paths into the strongSwan container:
#
#     pki/server.pem      -> /etc/swanctl/x509/server.pem
#     pki/server-key.pem  -> /etc/swanctl/private/server-key.pem
#     pki/ca.pem          -> /etc/swanctl/x509ca/ca.pem
#
# The mount list is fixed, so "server.pem" here is the certificate strongSwan
# PRESENTS, not the certificate of a server. In every other scenario strongSwan
# is the responder and that file really is a server certificate; here it is the
# EAP-TLS client certificate. Ze takes its own key material through
# %%PKI_B64%% placeholders in ze.conf and needs no mount, so its files carry
# their own names.
#
# Unlike eap-tls13 the CA is prime256v1 and not RSA. That RSA constraint is a
# property of strongSwan as the TLS SERVER: write_certificate_authorities()
# (src/libtls/tls_server.c) enumerates trust anchors with a hardcoded KEY_RSA
# filter when it builds the TLS 1.3 CertificateRequest. Here Go is the TLS
# server and Go's crypto/tls sends no certificate_authorities extension at all,
# so the anchor's key type never reaches the wire and the lab's usual EC CA is
# the right choice.
#
# The client certificate's subjectAltName is load-bearing. strongSwan asserts
# the EAP identity "swan-eap-client" (swanctl.conf, eap_id) and selects the
# client certificate that matches it. It parses a bare string identity as an
# FQDN, and an FQDN never matches a certificate whose only identity is the DN
# "CN=swan-eap-client".

set -eu

DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$DIR"

if [ -f ca.pem ] && [ -f server.pem ] && [ -f ze.pem ]; then
    exit 0
fi

# CA
openssl ecparam -genkey -name prime256v1 -noout -out ca-key.pem 2>/dev/null
openssl req -new -x509 -key ca-key.pem -out ca.pem -days 3650 \
    -subj "/CN=ze-interop-ca" -batch 2>/dev/null

# strongSwan's EAP-TLS CLIENT certificate, at the mounted "server.pem" name.
openssl ecparam -genkey -name prime256v1 -noout -out server-key.pem 2>/dev/null
openssl req -new -key server-key.pem -out client.csr \
    -subj "/CN=swan-eap-client" -batch 2>/dev/null
printf "subjectAltName=DNS:swan-eap-client\nextendedKeyUsage=clientAuth\n" > client-ext.cnf
openssl x509 -req -in client.csr -CA ca.pem -CAkey ca-key.pem \
    -CAcreateserial -out server.pem -days 3650 \
    -extfile client-ext.cnf 2>/dev/null
rm -f client.csr client-ext.cnf

# Ze's certificate. One certificate does two jobs: the IKEv2 AUTH signature the
# responder owes the initiator (RFC 7296 Section 2.16), and the TLS server
# certificate the EAP-TLS method presents. eapTLSServerConfig
# (internal/component/ike/engine/responder_eap.go) reads both from the same
# `authentication certificate` leaf, so they cannot differ.
openssl ecparam -genkey -name prime256v1 -noout -out ze-key.pem 2>/dev/null
openssl req -new -key ze-key.pem -out ze.csr \
    -subj "/CN=172.28.0.2" -batch 2>/dev/null
printf "subjectAltName=IP:172.28.0.2\nextendedKeyUsage=serverAuth\n" > ze-ext.cnf
openssl x509 -req -in ze.csr -CA ca.pem -CAkey ca-key.pem \
    -CAcreateserial -out ze.pem -days 3650 \
    -extfile ze-ext.cnf 2>/dev/null
rm -f ze.csr ze-ext.cnf ca.srl
