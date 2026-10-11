#!/usr/bin/env bash
# Runs inside the builder container (see build.sh):
# Go checks → engine for each phone CPU → Android unit tests → APK in android/dist/.
set -euo pipefail
MODE="${1:-release}"
cd /src

VERSION=$(sed -n 's/^var version = "\(.*\)"$/\1/p' cmd/arthik/main.go)
[ -n "$VERSION" ] || { echo "can't read the version from cmd/arthik/main.go" >&2; exit 1; }
echo "== arthik $VERSION ($MODE) =="

echo "== Go: vet + tests =="
go vet ./...
go test ./...

echo "== engine (libarthik.so per CPU) =="
LIBS=android/app/src/main/jniLibs
rm -rf "$LIBS"
engine() { # engine <abi> <env…>
  local abi=$1; shift
  mkdir -p "$LIBS/$abi"
  env CGO_ENABLED=0 "$@" go build -trimpath -ldflags "-s -w -X main.version=$VERSION" -o "$LIBS/$abi/libarthik.so" ./cmd/arthik
  echo "  $abi  $(du -h "$LIBS/$abi/libarthik.so" | cut -f1)"
}
engine arm64-v8a   GOOS=android GOARCH=arm64       # almost every phone
engine armeabi-v7a GOOS=linux   GOARCH=arm GOARM=7 # older 32-bit phones (static binary)
engine x86_64      GOOS=linux   GOARCH=amd64       # emulators, Chromebooks

echo "== APK =="
cd android
chmod +x gradlew
if [ "$MODE" = "debug" ]; then
  ./gradlew --no-daemon --console=plain -ParthikVersion="$VERSION" testDebugUnitTest assembleDebug
  APK=app/build/outputs/apk/debug/app-debug.apk
  OUT="dist/arthik-$VERSION-debug.apk"
else
  ./gradlew --no-daemon --console=plain -ParthikVersion="$VERSION" testReleaseUnitTest assembleRelease
  APK=app/build/outputs/apk/release/app-release.apk
  OUT="dist/arthik-$VERSION.apk"
  apksigner verify --print-certs "$APK" | sed -n '1,2p'
fi
mkdir -p dist
cp "$APK" "$OUT"
echo "== done: android/$OUT ($(du -h "$OUT" | cut -f1)) =="
