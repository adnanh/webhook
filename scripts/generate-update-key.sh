#!/bin/sh
set -eu

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT INT TERM

openssl genpkey -algorithm ED25519 -out "$tmp/private.pem"
openssl pkey -in "$tmp/private.pem" -pubout -outform DER -out "$tmp/public.der"

echo "GitHub Actions secret UPDATE_SIGNING_PRIVATE_KEY:"
cat "$tmp/private.pem"
echo
echo "GitHub Actions variable UPDATE_MANIFEST_PUBLIC_KEY:"
tail -c 32 "$tmp/public.der" | base64 | tr -d '\n'
echo
