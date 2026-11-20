#!/usr/bin/env bash
set -euo pipefail
# shellcheck source=common.sh
. "$(dirname "$0")/common.sh"

echo "::group::gofmt"
# Only files in this change set are blocking; main already contains some
# gofmt drift outside E's scope.
mapfile -t go_files < <(changed_files | grep '\.go$' | grep -v '/node_modules/' || true)
dirty=""
if [[ ${#go_files[@]} -gt 0 ]]; then
  dirty="$(gofmt -l "${go_files[@]}")"
fi
if [[ -n "$dirty" ]]; then
  echo "gofmt would rewrite changed files:"
  echo "$dirty"
  exit 1
fi
echo "gofmt clean for changed Go files (${#go_files[@]} checked)"
echo "::endgroup::"

echo "::group::go vet"
(cd src && go vet ./...)
echo "::endgroup::"

echo "::group::go mod verify"
(cd src && go mod verify)
echo "::endgroup::"

echo "::group::TypeScript"
(cd src/web && npm ci --no-audit --no-fund && npx tsc --noEmit)
echo "::endgroup::"

echo "::group::Safety-path checks"
python3 scripts/ci/safety-paths.py
echo "::endgroup::"
