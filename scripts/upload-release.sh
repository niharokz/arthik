#!/bin/sh
# CI only (release-files job): uploads dist/release/* to the project's package registry
# under the tag, and writes release-notes.md from this version's CHANGELOG.md section.
set -eu
TAG="${CI_COMMIT_TAG:?CI only: no CI_COMMIT_TAG}"
VERSION=$(cat VERSION)
[ "v$VERSION" = "$TAG" ] || { echo "tag $TAG does not match VERSION $VERSION" >&2; exit 1; }
URL="$CI_API_V4_URL/projects/$CI_PROJECT_ID/packages/generic/arthik/$TAG"

for f in dist/release/*; do
  [ -f "$f" ] || continue
  echo "upload $(basename "$f")"
  curl --fail-with-body --silent --show-error --header "JOB-TOKEN: $CI_JOB_TOKEN" \
    --upload-file "$f" "$URL/$(basename "$f")" >/dev/null
done

awk -v v="$VERSION" '/^## /{p = index($0 " ", "## " v " ") == 1; next} p' CHANGELOG.md > release-notes.md
[ -s release-notes.md ] || { echo "CHANGELOG.md has no section for $VERSION" >&2; exit 1; }
echo "release-notes.md ready"
