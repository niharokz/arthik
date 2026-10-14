#!/usr/bin/env bash
# Builds the arthik Android app with Docker — nothing else to install.
#
#   ./android/build.sh keystore   create your signing key (once; keep it backed up)
#   ./android/build.sh            signed release APK  → android/dist/arthik-<version>.apk
#   ./android/build.sh debug      debug APK (no key)  → android/dist/arthik-<version>-debug.apk
#   ./android/build.sh quick      signed release APK, no tests, 64-bit phones only (fastest)
#
# Settings come from android/.env (copy android/.env.example).
set -euo pipefail
cd "$(dirname "$0")/.."                       # repo root

ENV_FILE=android/.env
if [ -f "$ENV_FILE" ]; then set -a; . "$ENV_FILE"; set +a; fi
KEYS_DIR="${ARTHIK_KEYSTORE_DIR:-$HOME/.arthik-keys}"
KEY_FILE="$KEYS_DIR/arthik.jks"
IMAGE=arthik-android-builder

mkdir -p android/.cache/home android/dist "$KEYS_DIR"
echo "== builder image =="
# The Dockerfile copies nothing, so it is sent alone (no build context): sending
# android/ would upload the multi-GB caches in android/.cache on every run.
docker build -t "$IMAGE" - < android/Dockerfile

# Runs a command in the builder as you (files stay yours), with caches kept in android/.cache.
run() {
  docker run --rm -t --user "$(id -u):$(id -g)" \
    -v "$PWD":/src -v "$KEYS_DIR":/keys -w /src \
    -e HOME=/src/android/.cache/home \
    -e JAVA_TOOL_OPTIONS=-Duser.home=/src/android/.cache/home \
    -e GRADLE_USER_HOME=/src/android/.cache/gradle \
    -e ANDROID_USER_HOME=/src/android/.cache/android \
    -e GOPATH=/src/android/.cache/go -e GOCACHE=/src/android/.cache/go-build \
    -e ARTHIK_KEYSTORE_PASSWORD -e ARTHIK_KEY_ALIAS -e ARTHIK_KEY_PASSWORD \
    "$@"
}

case "${1:-release}" in
  keystore)
    [ -f "$KEY_FILE" ] && { echo "A key already exists at $KEY_FILE — keep using it (updates must be signed with the same key)."; exit 1; }
    [ -n "${ARTHIK_KEYSTORE_PASSWORD:-}" ] || { echo "Set ARTHIK_KEYSTORE_PASSWORD in $ENV_FILE first."; exit 1; }
    run "$IMAGE" keytool -genkeypair -v -keystore /keys/arthik.jks -alias "${ARTHIK_KEY_ALIAS:-arthik}" \
      -keyalg RSA -keysize 4096 -validity 36500 -dname "CN=arthik" \
      -storepass:env ARTHIK_KEYSTORE_PASSWORD -keypass:env ARTHIK_KEYSTORE_PASSWORD
    echo "Key created: $KEY_FILE — back it up (e.g. into your password manager)."
    ;;
  release)
    [ -f "$KEY_FILE" ] || { echo "No signing key yet. Run: ./android/build.sh keystore"; exit 1; }
    run -e ARTHIK_KEYSTORE=/keys/arthik.jks -e ARTHIK_QUICK -e ARTHIK_ABIS "$IMAGE" bash android/scripts/build-in-container.sh release
    ;;
  debug)
    run -e ARTHIK_QUICK -e ARTHIK_ABIS "$IMAGE" bash android/scripts/build-in-container.sh debug
    ;;
  quick)
    [ -f "$KEY_FILE" ] || { echo "No signing key yet. Run: ./android/build.sh keystore"; exit 1; }
    run -e ARTHIK_KEYSTORE=/keys/arthik.jks -e ARTHIK_QUICK=1 -e ARTHIK_ABIS="${ARTHIK_ABIS:-arm64-v8a}" "$IMAGE" bash android/scripts/build-in-container.sh release
    ;;
  *)
    echo "usage: $0 [keystore|release|debug|quick]"; exit 2 ;;
esac
