#!/bin/bash
#
# setup-codesign-identity.sh — create a STABLE, self-signed code-signing
# identity for KubePilot on macOS, so rebuilt binaries keep a constant code
# identity across builds.
#
# WHY THIS EXISTS
# ---------------
# macOS gates access to the local network (System Settings > Privacy &
# Security > Local Network). The grant is keyed to a binary's *code identity*.
# An ad-hoc signature's identity is the binary's content hash, so every rebuild
# looks like a brand-new app that was never granted access — and because the
# server runs as a launchd background agent, it can't show the permission
# prompt and is silently denied. The symptom is the launchd server failing with
#   dial tcp <cluster-ip>:6443: connect: no route to host
# while `kubectl`/`curl` from Terminal reach the same cluster fine.
#
# Signing every build with ONE fixed identity + identifier keeps the code
# signature's "designated requirement" constant, so the Local Network grant
# sticks across rebuilds. You grant it once; it stays granted.
#
# USAGE (run once, in your own Terminal so the keychain can be unlocked):
#   ./scripts/setup-codesign-identity.sh
#
# Then rebuild (scripts/build-kubepilot.sh signs automatically) and grant
# "kubepilot" in System Settings > Privacy & Security > Local Network.
#
set -euo pipefail

IDENTITY_NAME="${KP_SIGN_IDENTITY:-KubePilot Local Signing}"
KEYCHAIN="${HOME}/Library/Keychains/login.keychain-db"
P12_PASS="kubepilot"

if [[ "$(uname -s)" != "Darwin" ]]; then
  echo "This script is only needed on macOS." >&2
  exit 0
fi

if security find-identity -v -p codesigning 2>/dev/null | grep -qF "$IDENTITY_NAME"; then
  echo "Code-signing identity '$IDENTITY_NAME' already exists — nothing to do."
  security find-identity -v -p codesigning | grep -F "$IDENTITY_NAME"
  exit 0
fi

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
cd "$WORK"

echo "Generating self-signed code-signing certificate '$IDENTITY_NAME'…"
cat > csr.cnf <<EOF
[req]
distinguished_name = dn
x509_extensions = v3
prompt = no
[dn]
CN = ${IDENTITY_NAME}
[v3]
basicConstraints = critical, CA:false
keyUsage = critical, digitalSignature
extendedKeyUsage = critical, codeSigning
EOF

openssl req -x509 -newkey rsa:2048 -keyout key.pem -out cert.pem \
  -days 3650 -nodes -config csr.cnf >/dev/null 2>&1

# macOS `security` needs a legacy-MAC PKCS#12 (OpenSSL 3 defaults are unreadable).
openssl pkcs12 -export -legacy -inkey key.pem -in cert.pem -out kp.p12 \
  -name "$IDENTITY_NAME" -passout "pass:${P12_PASS}" >/dev/null 2>&1

echo "Importing into the login keychain (may prompt for your login password)…"
security import kp.p12 -k "$KEYCHAIN" -P "$P12_PASS" \
  -T /usr/bin/codesign -A

echo "Trusting the certificate for code signing…"
# User-domain trust for the codeSign policy; may prompt for your login password.
security add-trusted-cert -p codeSign -k "$KEYCHAIN" cert.pem 2>/dev/null || \
  echo "note: add-trusted-cert skipped/failed; codesign can still use the identity." >&2

echo
if security find-identity -v -p codesigning | grep -qF "$IDENTITY_NAME"; then
  echo "✅ Identity ready:"
  security find-identity -v -p codesigning | grep -F "$IDENTITY_NAME"
  echo
  echo "Next:"
  echo "  1. Rebuild:  ./scripts/build-kubepilot.sh   (it now signs automatically)"
  echo "  2. In System Settings > Privacy & Security > Local Network, enable 'kubepilot'."
else
  echo "⚠️  Identity not visible to codesign yet. Open Keychain Access, find" >&2
  echo "    '$IDENTITY_NAME' under 'login', and set its trust to 'Code Signing: Always Trust'." >&2
  exit 1
fi
