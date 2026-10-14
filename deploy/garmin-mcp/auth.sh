#!/bin/sh
# One-off Garmin sign-in on the laptop (email, password, MFA), writing the
# token files and the base64 bundle into the named volume. Run from anywhere.
set -e
cd "$(dirname "$0")"
docker compose --profile auth run --rm garmin-mcp-auth
echo
echo "Now copy the bundle to the clipboard with: $(dirname "$0")/bundle.sh"
