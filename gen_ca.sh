#!/bin/sh

openssl genrsa -out ca.key 2048

# Generate CA certificate (e.g., valid for 10 years)
openssl req -x509 -new -nodes -key ca.key -sha256 -days 3650 -out ca.crt -subj "/CN=MyProxyCA"