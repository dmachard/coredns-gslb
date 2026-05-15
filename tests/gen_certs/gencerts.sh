#!/bin/sh

# mTLS verification requires a common trusted root CA, the rootca.cnf is from
#  my local machine and then modified for easy cert creation
#
# See the following links for more information:
# https://docs.openssl.org/3.0/man1/openssl-ca/#examples
# https://docs.openssl.org/3.0/man5/x509v3_config/#key-usage
#
# Create self-signed certificate w/ root CA extensions
#

if [ -f /app/certs/ca/cert.pem ]; then
exit 0
fi

mkdir -p certs/client certs/ca
cp gen_certs/rootca.cnf certs/

openssl req -x509 -new -newkey ED25519 -noenc -config gen_certs/rootca.cnf \
 -subj "/CN=coredns-gslb-ca" -keyout certs/ca/key.pem -out certs/ca/cert.pem \
 -extensions default_ca_exts -quiet

# Create client certificate for presenting to mTLS validating server
# and sign with previously created root CA 
# 
openssl req -x509 -new -newkey ED25519 -noenc -config gen_certs/rootca.cnf \
 -subj "/CN=cdns-gslb-client" -keyout certs/client/key.pem \
 -out certs/client/cert.pem -CA certs/ca/cert.pem -CAkey certs/ca/key.pem \
 -extensions client_req_exts -quiet

# openssl verify -x509_strict -CAfile certs/ca/cert.pem certs/client/cert.pem

# Create server certificate w/ extensions for validating
# presented client identity certificate and sign w/ root CA (mTLS)
#
#openssl req -x509 -new -newkey ED25519 -noenc -config gen_certs/rootca.cnf \
# -subj "/CN=cdns-gslb-server" -keyout certs/tls/key.pem \
# -out certs/tls/cert.pem -CA certs/ca/cert.pem -CAkey certs/ca/key.pem \
# -extensions server_req_exts -quiet

# openssl verify -x509_strict -CAfile certs/ca/cert.pem certs/tls/cert.pem