#!/bin/sh

openssl ecparam -genkey -name secp521r1 -noout -out privkey1.pem
openssl pkcs8 -topk8 -nocrypt -in privkey1.pem -outform DER -out trusted_client_priv.der
openssl req -new -x509 -key privkey1.pem -out trusted_client_cert.pem -days 36500
rm privkey1.pem

