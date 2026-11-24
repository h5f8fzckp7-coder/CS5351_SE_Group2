#!/bin/sh
set -eu
d_repo_root=$(cd "$(dirname "$0")/../../.." && pwd)
cd "$d_repo_root/src"
go test -count=1 ./internal/ai ./internal/adapters/aichat ./internal/app -run Comparison
cd "$d_repo_root/src/web"
npm test -- src/features/ai/ComparisonPanel.test.tsx src/features/ai/comparisonSources.test.tsx
