#!/usr/bin/env bash
# Release build inside arthik-builder, on gtlbrunner: the GitLab release job and
# `./build.sh arthik` there run this from the source root.
#
#   BUILD_CACHE  Gradle, Go and Android caches, kept between builds
#   KEY_DIR      arthik.jks + keystore.env (read-only): ARTHIK_KEYSTORE_PASSWORD,
#                optional ARTHIK_KEY_ALIAS and ARTHIK_KEY_PASSWORD
#
# Leaves dist/release/arthik-vX.Y.Z.apk and SHA256SUMS. Never uploads (scripts/upload-release.sh does, in CI).
# Any argument (e.g. "ci") is accepted and ignored: every build is the full, tested release.
set -euo pipefail
cd "$(dirname "$0")/.."
: "${BUILD_CACHE:?BUILD_CACHE is not set (gtlbrunner sets it)}"
: "${KEY_DIR:?KEY_DIR is not set (gtlbrunner sets it)}"
[ -f "$KEY_DIR/arthik.jks" ] || { echo "$KEY_DIR/arthik.jks is missing" >&2; exit 1; }
[ -f "$KEY_DIR/keystore.env" ] || { echo "$KEY_DIR/keystore.env is missing" >&2; exit 1; }
set -a; . "$KEY_DIR/keystore.env"; set +a
[ -n "${ARTHIK_KEYSTORE_PASSWORD:-}" ] || { echo "ARTHIK_KEYSTORE_PASSWORD is not set in $KEY_DIR/keystore.env" >&2; exit 1; }
export ARTHIK_KEYSTORE="$KEY_DIR/arthik.jks"

# every cache under BUILD_CACHE, so nothing lands in the checkout or the image
export HOME="$BUILD_CACHE/home" GRADLE_USER_HOME="$BUILD_CACHE/gradle" ANDROID_USER_HOME="$BUILD_CACHE/android" \
  GOPATH="$BUILD_CACHE/go" GOCACHE="$BUILD_CACHE/go-build" JAVA_TOOL_OPTIONS="-Duser.home=$BUILD_CACHE/home"
mkdir -p "$HOME" "$GRADLE_USER_HOME" "$ANDROID_USER_HOME" "$GOPATH" "$GOCACHE"

VERSION=$(cat VERSION)
bash android/scripts/build-in-container.sh release   # Go checks, engine for every CPU, Android tests, signed APK

rm -rf dist/release && mkdir -p dist/release
cp "android/dist/arthik-$VERSION.apk" "dist/release/arthik-v$VERSION.apk"
(cd dist/release && sha256sum arthik-v*.apk > SHA256SUMS)
echo "== dist/release: arthik-v$VERSION.apk, SHA256SUMS =="
