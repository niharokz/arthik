#!/bin/sh
# Blocks a push that would publish secrets or personal data.
#
# Install once as a git pre-push hook:
#   ln -s ../../scripts/check-public.sh .git/hooks/pre-push
# Or run by hand:  sh scripts/check-public.sh
#
# Put words that must never appear in the public repo in .private-words
# (one per line, gitignored): your server user name, real card digits, names...
set -eu
cd "$(git rev-parse --show-toplevel)"
fail=0

for f in .env android/.env .private-words; do
  if git ls-files --error-unmatch "$f" >/dev/null 2>&1; then
    echo "✗ $f is tracked by git - remove it: git rm --cached $f"; fail=1
  fi
done
if git ls-files | grep -E '\.(jks|keystore)$|(^|/)patch/' >/dev/null; then
  echo "✗ a signing key or a patch zip is tracked:"; git ls-files | grep -E '\.(jks|keystore)$|(^|/)patch/' | sed 's/^/    /'; fail=1
fi

pat='^(ARTHIK_(PASSWORD|PASSWORD_HASH|SECRET|DEVICE_KEY|KEYSTORE_PASSWORD))=.{12,}'
if git grep -nI -E "$pat" -- . ':!.env.example' ':!android/.env.example' >/dev/null 2>&1; then
  echo "✗ something that looks like a real password or key is committed:"
  git grep -nI -E "$pat" -- . ':!.env.example' ':!android/.env.example'; fail=1
fi

if [ -f .private-words ]; then
  while IFS= read -r word; do
    case "$word" in ''|'#'*) continue ;; esac
    if git grep -nIiF -e "$word" -- . >/dev/null 2>&1; then
      echo "✗ private word \"$word\" found in:"
      git grep -nIiF -l -e "$word" -- . | sed 's/^/    /'; fail=1
    fi
  done < .private-words
fi

if [ "$fail" -ne 0 ]; then
  echo "Push blocked. Fix the files above (or remove the word from .private-words)."
  exit 1
fi
echo "✓ no secrets or private words found"
