#!/bin/sh
# Run Arthik locally on a throwaway folder (the demo data is generated on start):
#   sh scripts/dev.sh      then open http://localhost:8080  (sign in: me / dev, or the demo)
set -eu
cd "$(dirname "$0")/.."
tmp="${TMPDIR:-/tmp}/arthik-dev"
mkdir -p "$tmp/data" "$tmp/state"
ARTHIK_DATA="$tmp/data" ARTHIK_STATE_DIR="$tmp/state" ARTHIK_LISTEN=127.0.0.1:8080 \
ARTHIK_USER=me ARTHIK_PASSWORD=dev ARTHIK_SECRET=dev-only-secret-not-for-real-use-0000 \
ARTHIK_DEMO_USER=demouser ARTHIK_DEMO_PASSWORD=demo \
exec go run -ldflags "-X main.version=$(cat VERSION)-dev" ./cmd/arthik serve
